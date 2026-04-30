package app

import (
	"context"
	"fmt"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

// DashboardService returns local dashboard and historical analytics data.
type DashboardService struct {
	store      dashboardStore
	recorder   audit.Recorder
	authorizer rbac.Authorizer
	now        func() time.Time
}

type dashboardStore interface {
	AdminCount(context.Context) (int64, error)
	AnalyticsSummary(context.Context) (store.AnalyticsSummary, error)
	HistoricalSessions(context.Context, int) ([]store.HistoricalSession, error)
	RecentAuditEvents(context.Context, int) ([]store.AuditEvent, error)
	PurgeHistoricalBefore(context.Context, time.Time) (int64, error)
}

// OcservStatusSummary intentionally contains no live ocserv state.
type OcservStatusSummary struct {
	Status     string
	Configured bool
	Message    string
}

// DashboardSummary is the local dashboard summary for HTTP mapping.
type DashboardSummary struct {
	BackendStatus      string
	ActiveSessionCount int
	RecentAudit        []store.AuditEvent
	Analytics          store.AnalyticsSummary
	SampleData         bool
	Ocserv             OcservStatusSummary
	SetupRequired      bool
	AdminCount         int64
	History            store.AnalyticsSummary
}

// HistoryAnalyticsResult contains persisted/sample historical analytics.
type HistoryAnalyticsResult struct {
	Summary        store.AnalyticsSummary
	RecentSessions []store.HistoricalSession
}

// PurgeHistoryResult describes an accepted historical purge.
type PurgeHistoryResult struct {
	RetentionDays int
	Cutoff        time.Time
	DeletedRows   int64
}

// NewDashboardService returns a dashboard service using local SQLite data only.
func NewDashboardService(s *store.Store, recorder audit.Recorder, authorizer rbac.Authorizer) *DashboardService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	return &DashboardService{store: s, recorder: recorder, authorizer: authorizer, now: func() time.Time { return time.Now().UTC() }}
}

// Summary returns local no-op ocserv status and SQLite-backed counts.
func (s *DashboardService) Summary(ctx context.Context, principal auth.Principal) (DashboardSummary, error) {
	if err := s.require(ctx, principal, rbac.PermissionReadDashboard, "dashboard.summary"); err != nil {
		return DashboardSummary{}, err
	}
	actorID := principal.ID
	adminCount, err := s.store.AdminCount(ctx)
	if err != nil {
		_ = s.record(
			ctx,
			&actorID,
			"dashboard.summary",
			"dashboard",
			"",
			audit.OutcomeFailure,
			"dashboard summary load failed",
			map[string]string{"load": "admin_count"},
		)
		return DashboardSummary{}, fmt.Errorf("count admins for dashboard: %w", err)
	}
	history, err := s.store.AnalyticsSummary(ctx)
	if err != nil {
		_ = s.record(
			ctx,
			&actorID,
			"dashboard.summary",
			"dashboard",
			"",
			audit.OutcomeFailure,
			"dashboard summary load failed",
			map[string]string{"load": "analytics_summary"},
		)
		return DashboardSummary{}, fmt.Errorf("load dashboard analytics summary: %w", err)
	}
	recentAudit, err := s.store.RecentAuditEvents(ctx, 10)
	if err != nil {
		_ = s.record(
			ctx,
			&actorID,
			"dashboard.summary",
			"dashboard",
			"",
			audit.OutcomeFailure,
			"dashboard summary load failed",
			map[string]string{"load": "recent_audit_events"},
		)
		return DashboardSummary{}, fmt.Errorf("load dashboard recent audit events: %w", err)
	}
	if err := s.record(
		ctx,
		&actorID,
		"dashboard.summary",
		"dashboard",
		"",
		audit.OutcomeSuccess,
		"dashboard summary loaded",
		map[string]string{"source": "sqlite"},
	); err != nil {
		return DashboardSummary{}, fmt.Errorf("audit dashboard summary: %w", err)
	}
	return DashboardSummary{
		BackendStatus:      "ok",
		ActiveSessionCount: 0,
		RecentAudit:        recentAudit,
		Analytics:          history,
		SampleData:         history.SampleSessions > 0,
		Ocserv:             OcservStatusSummary{Status: "not_configured", Configured: false, Message: "live ocserv integration is not configured"},
		SetupRequired:      adminCount == 0,
		AdminCount:         adminCount,
		History:            history,
	}, nil
}

// HistoryAnalytics returns sample-backed persisted historical analytics.
func (s *DashboardService) HistoryAnalytics(ctx context.Context, principal auth.Principal) (HistoryAnalyticsResult, error) {
	if err := s.require(ctx, principal, rbac.PermissionReadDashboard, "history.analytics"); err != nil {
		return HistoryAnalyticsResult{}, err
	}
	actorID := principal.ID
	summary, err := s.store.AnalyticsSummary(ctx)
	if err != nil {
		_ = s.record(
			ctx,
			&actorID,
			"history.analytics",
			"dashboard",
			"",
			audit.OutcomeFailure,
			"history analytics load failed",
			map[string]string{"load": "analytics_summary"},
		)
		return HistoryAnalyticsResult{}, fmt.Errorf("load history analytics summary: %w", err)
	}
	recent, err := s.store.HistoricalSessions(ctx, 25)
	if err != nil {
		_ = s.record(
			ctx,
			&actorID,
			"history.analytics",
			"dashboard",
			"",
			audit.OutcomeFailure,
			"history analytics load failed",
			map[string]string{"load": "historical_sessions"},
		)
		return HistoryAnalyticsResult{}, fmt.Errorf("load recent historical sessions: %w", err)
	}
	if err := s.record(
		ctx,
		&actorID,
		"history.analytics",
		"dashboard",
		"",
		audit.OutcomeSuccess,
		"history analytics loaded",
		map[string]string{"source": "sqlite"},
	); err != nil {
		return HistoryAnalyticsResult{}, fmt.Errorf("audit history analytics: %w", err)
	}
	return HistoryAnalyticsResult{Summary: summary, RecentSessions: recent}, nil
}

// PurgeHistory deletes historical rows older than an allowed retention window.
func (s *DashboardService) PurgeHistory(ctx context.Context, principal auth.Principal, retentionDays int, confirmation string) (PurgeHistoryResult, error) {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, "history.purge", "historical_session", "", audit.OutcomeDenied, "history purge authentication required", map[string]string{"retention_days": fmt.Sprint(retentionDays)})
		return PurgeHistoryResult{}, ErrUnauthenticated
	}
	actorID := principal.ID
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionHistoryPurge) {
		_ = s.record(ctx, &actorID, "history.purge", "historical_session", "", audit.OutcomeDenied, "history purge permission denied", nil)
		return PurgeHistoryResult{}, ErrPermissionDenied
	}
	if confirmation != "PURGE" {
		_ = s.record(ctx, &actorID, "history.purge", "historical_session", "", audit.OutcomeDenied, "history purge confirmation rejected", map[string]string{"retention_days": fmt.Sprint(retentionDays)})
		return PurgeHistoryResult{}, ErrInvalidConfirmation
	}
	if !allowedRetentionDays(retentionDays) {
		_ = s.record(ctx, &actorID, "history.purge", "historical_session", "", audit.OutcomeDenied, "history purge retention rejected", map[string]string{"retention_days": fmt.Sprint(retentionDays)})
		return PurgeHistoryResult{}, ErrInvalidRetention
	}

	cutoff := s.now().AddDate(0, 0, -retentionDays).UTC()
	deleted, err := s.store.PurgeHistoricalBefore(ctx, cutoff)
	if err != nil {
		_ = s.record(ctx, &actorID, "history.purge", "historical_session", "", audit.OutcomeFailure, "history purge failed", map[string]string{"retention_days": fmt.Sprint(retentionDays)})
		return PurgeHistoryResult{}, fmt.Errorf("purge historical sessions: %w", err)
	}
	if err := s.record(ctx, &actorID, "history.purge", "historical_session", "", audit.OutcomeSuccess, "history purge succeeded", map[string]string{"retention_days": fmt.Sprint(retentionDays), "deleted_rows": fmt.Sprint(deleted)}); err != nil {
		return PurgeHistoryResult{}, fmt.Errorf("audit history purge: %w", err)
	}
	return PurgeHistoryResult{RetentionDays: retentionDays, Cutoff: cutoff, DeletedRows: deleted}, nil
}

func (s *DashboardService) require(ctx context.Context, principal auth.Principal, permission rbac.Permission, action string) error {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, action, "dashboard", "", audit.OutcomeDenied, "dashboard authentication required", nil)
		return ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), permission) {
		actorID := principal.ID
		_ = s.record(ctx, &actorID, action, "dashboard", "", audit.OutcomeDenied, "dashboard permission denied", nil)
		return ErrPermissionDenied
	}
	return nil
}

func (s *DashboardService) record(ctx context.Context, actorID *int64, action string, resourceType string, resourceID string, outcome audit.Outcome, message string, metadata map[string]string) error {
	return s.recorder.Record(ctx, audit.Event{ActorID: actorID, Action: action, ResourceType: resourceType, ResourceID: resourceID, Outcome: outcome, Message: message, Metadata: metadata, CreatedAt: s.now()})
}

func allowedRetentionDays(days int) bool {
	switch days {
	case 30, 90, 180, 365:
		return true
	default:
		return false
	}
}
