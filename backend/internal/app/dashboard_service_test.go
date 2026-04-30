package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

func TestDashboardServiceSummaryAndHistoryRequireAuth(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	service := NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())

	if _, err := service.Summary(ctx, auth.Principal{}); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("summary unauthenticated err = %v, want ErrUnauthenticated", err)
	}
	if _, err := service.HistoryAnalytics(ctx, auth.Principal{}); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("history unauthenticated err = %v, want ErrUnauthenticated", err)
	}

	events := mustRecentAuditEvents(t, s, 2)
	assertAuditOutcome(t, events, "dashboard.summary", string(audit.OutcomeDenied))
	assertAuditOutcome(t, events, "history.analytics", string(audit.OutcomeDenied))
	for _, event := range events {
		if event.ActorAdminUserID != nil {
			t.Fatalf("event %#v has actor id, want nil for unauthenticated read denial", event)
		}
	}
}

func TestDashboardServiceUnauthenticatedPurgeIsAuditedAsDenied(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	service := NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())

	if _, err := service.PurgeHistory(ctx, auth.Principal{}, 90, "PURGE"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("unauthenticated purge err = %v, want ErrUnauthenticated", err)
	}

	events := mustRecentAuditEvents(t, s, 1)
	assertAuditOutcome(t, events, "history.purge", string(audit.OutcomeDenied))
	if events[0].ActorAdminUserID != nil {
		t.Fatalf("actor id = %v, want nil for unauthenticated purge", *events[0].ActorAdminUserID)
	}
}

func TestDashboardServiceViewerCanReadButCannotPurge(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	viewer := createAdminWithRole(t, ctx, s, "viewer", rbac.RoleViewer)
	service := NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())
	if err := s.RecordAudit(ctx, store.AuditEvent{Action: "test.seed", ResourceType: "dashboard", Outcome: string(audit.OutcomeSuccess), Message: "seed event", Metadata: map[string]string{"safe": "value"}, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("seed audit event: %v", err)
	}

	summary, err := service.Summary(ctx, viewer)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.BackendStatus != "ok" || summary.Ocserv.Status != "not_configured" || summary.Ocserv.Configured || summary.SetupRequired {
		t.Fatalf("summary = %#v, want ok backend, not_configured ocserv, configured=false, setup=false", summary)
	}
	if summary.AdminCount != 1 || summary.History.TotalSessions == 0 || summary.Analytics.TotalSessions == 0 {
		t.Fatalf("summary counts = %#v, want admin and history counts", summary)
	}
	if summary.ActiveSessionCount != 0 {
		t.Fatalf("active session count = %d, want safe no-op placeholder 0", summary.ActiveSessionCount)
	}
	if !summary.SampleData {
		t.Fatalf("sample data = false, want true when sample sessions exist in analytics")
	}
	if summary.Analytics.SampleSessions == 0 {
		t.Fatalf("analytics = %#v, want sample sessions", summary.Analytics)
	}
	if len(summary.RecentAudit) == 0 {
		t.Fatalf("recent audit = %#v, want recent audit events", summary.RecentAudit)
	}

	analytics, err := service.HistoryAnalytics(ctx, viewer)
	if err != nil {
		t.Fatalf("history analytics: %v", err)
	}
	if analytics.Summary.SampleSessions == 0 || len(analytics.RecentSessions) == 0 {
		t.Fatalf("analytics = %#v, want sample-backed data", analytics)
	}

	if _, err := service.PurgeHistory(ctx, viewer, 90, "PURGE"); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("viewer purge err = %v, want ErrPermissionDenied", err)
	}
	assertAuditOutcome(t, mustRecentAuditEvents(t, s, 1), "history.purge", string(audit.OutcomeDenied))
}

func TestDashboardServiceAuthenticatedPrincipalWithoutDashboardPermissionIsAuditedDenied(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	service := NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())
	principal := createAdminWithoutRoles(t, ctx, s, "limited")

	if _, err := service.Summary(ctx, principal); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("summary err = %v, want ErrPermissionDenied", err)
	}
	if _, err := service.HistoryAnalytics(ctx, principal); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("history err = %v, want ErrPermissionDenied", err)
	}

	events := mustRecentAuditEvents(t, s, 2)
	assertAuditOutcome(t, events, "dashboard.summary", string(audit.OutcomeDenied))
	assertAuditOutcome(t, events, "history.analytics", string(audit.OutcomeDenied))
	for _, event := range events {
		if event.ActorAdminUserID == nil || *event.ActorAdminUserID != principal.ID {
			t.Fatalf("event %#v actor id, want principal id %d", event, principal.ID)
		}
	}
}

func TestDashboardServiceSummaryAuditsSuccessfulRead(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	viewer := createAdminWithRole(t, ctx, s, "viewer", rbac.RoleViewer)
	service := NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())

	if _, err := service.Summary(ctx, viewer); err != nil {
		t.Fatalf("summary: %v", err)
	}

	assertAuditOutcome(
		t,
		mustRecentAuditEvents(t, s, 1),
		"dashboard.summary",
		string(audit.OutcomeSuccess),
	)
}

func TestDashboardServiceHistoryAnalyticsAuditsSuccessfulRead(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	viewer := createAdminWithRole(t, ctx, s, "viewer", rbac.RoleViewer)
	service := NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())

	if _, err := service.HistoryAnalytics(ctx, viewer); err != nil {
		t.Fatalf("history analytics: %v", err)
	}

	assertAuditOutcome(
		t,
		mustRecentAuditEvents(t, s, 1),
		"history.analytics",
		string(audit.OutcomeSuccess),
	)
}

func TestDashboardServiceSummaryAuditsStoreFailure(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	viewer := createAdminWithRole(t, ctx, s, "viewer", rbac.RoleViewer)
	service := NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())
	service.store = failingDashboardStore{adminCountErr: errors.New("forced summary failure")}

	if _, err := service.Summary(ctx, viewer); err == nil {
		t.Fatal("summary succeeded, want failure")
	}

	assertAuditOutcome(
		t,
		mustRecentAuditEvents(t, s, 1),
		"dashboard.summary",
		string(audit.OutcomeFailure),
	)
}

func TestDashboardServiceHistoryAnalyticsAuditsStoreFailure(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	viewer := createAdminWithRole(t, ctx, s, "viewer", rbac.RoleViewer)
	service := NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())
	service.store = failingDashboardStore{analyticsSummaryErr: errors.New("forced history failure")}

	if _, err := service.HistoryAnalytics(ctx, viewer); err == nil {
		t.Fatal("history analytics succeeded, want failure")
	}

	assertAuditOutcome(
		t,
		mustRecentAuditEvents(t, s, 1),
		"history.analytics",
		string(audit.OutcomeFailure),
	)
}

func TestDashboardServiceOwnerPurgeValidationAndDeletion(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	service := NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())
	now := time.Now().UTC()
	oldDisconnected := now.AddDate(0, 0, -120)
	newDisconnected := now.AddDate(0, 0, -10)

	insertHistoricalSession(t, ctx, s, "old-user", oldDisconnected)
	insertHistoricalSession(t, ctx, s, "new-user", newDisconnected)

	if _, err := service.PurgeHistory(ctx, owner, 90, "purge"); !errors.Is(err, ErrInvalidConfirmation) {
		t.Fatalf("wrong confirmation err = %v, want ErrInvalidConfirmation", err)
	}
	if _, err := service.PurgeHistory(ctx, owner, 45, "PURGE"); !errors.Is(err, ErrInvalidRetention) {
		t.Fatalf("wrong retention err = %v, want ErrInvalidRetention", err)
	}

	result, err := service.PurgeHistory(ctx, owner, 90, "PURGE")
	if err != nil {
		t.Fatalf("purge history: %v", err)
	}
	if result.DeletedRows < 1 || result.RetentionDays != 90 || result.Cutoff.IsZero() {
		t.Fatalf("purge result = %#v, want at least one older row deleted", result)
	}
	if oldCount := countRows(t, ctx, s.DB, `SELECT COUNT(*) FROM historical_sessions WHERE username = ?`, "old-user"); oldCount != 0 {
		t.Fatalf("old row count = %d, want 0", oldCount)
	}
	if newCount := countRows(t, ctx, s.DB, `SELECT COUNT(*) FROM historical_sessions WHERE username = ?`, "new-user"); newCount != 1 {
		t.Fatalf("new row count = %d, want 1", newCount)
	}
	assertAuditOutcome(t, mustRecentAuditEvents(t, s, 3), "history.purge", string(audit.OutcomeSuccess))
	assertAuditOutcome(t, mustRecentAuditEvents(t, s, 3), "history.purge", string(audit.OutcomeDenied))
}

func TestDashboardServicePurgeHistoryAllowsDocumentedRetentionDays(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	service := NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())

	for _, retentionDays := range []int{30, 90, 180, 365} {
		t.Run(fmt.Sprintf("%d days", retentionDays), func(t *testing.T) {
			result, err := service.PurgeHistory(ctx, owner, retentionDays, "PURGE")
			if err != nil {
				t.Fatalf("purge history: %v", err)
			}
			if result.RetentionDays != retentionDays || result.Cutoff.IsZero() {
				t.Fatalf("result = %#v, want retention %d and cutoff", result, retentionDays)
			}
		})
	}
}

func TestDashboardServicePurgeHistoryAuditsStoreFailure(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	service := NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer())
	service.store = failingDashboardStore{purgeErr: errors.New("forced purge failure")}

	if _, err := service.PurgeHistory(ctx, owner, 90, "PURGE"); err == nil {
		t.Fatal("purge history succeeded, want failure")
	}
	assertAuditOutcome(t, mustRecentAuditEvents(t, s, 1), "history.purge", string(audit.OutcomeFailure))
}

func insertHistoricalSession(t *testing.T, ctx context.Context, s *store.Store, username string, disconnectedAt time.Time) {
	t.Helper()
	connectedAt := disconnectedAt.Add(-time.Hour)
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		username,
		connectedAt.Format(time.RFC3339Nano),
		disconnectedAt.Format(time.RFC3339Nano),
		1,
		2,
		3600,
		"ingested",
		disconnectedAt.Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("insert historical session: %v", err)
	}
}

func createAdminWithoutRoles(t *testing.T, ctx context.Context, s *store.Store, username string) auth.Principal {
	t.Helper()

	passwordHash, err := auth.HashPassword("password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.DB.ExecContext(ctx, `INSERT INTO admin_users (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)`, username, passwordHash, now, now)
	if err != nil {
		t.Fatalf("insert admin without roles: %v", err)
	}
	adminID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read admin id: %v", err)
	}
	return auth.Principal{ID: adminID, Username: username, CreatedAt: time.Now().UTC()}
}

type failingDashboardStore struct {
	adminCountErr         error
	analyticsSummaryErr   error
	historicalSessionsErr error
	recentAuditEventsErr  error
	purgeErr              error
}

func (f failingDashboardStore) AdminCount(ctx context.Context) (int64, error) {
	if f.adminCountErr != nil {
		return 0, f.adminCountErr
	}
	return 0, nil
}

func (f failingDashboardStore) AnalyticsSummary(ctx context.Context) (store.AnalyticsSummary, error) {
	if f.analyticsSummaryErr != nil {
		return store.AnalyticsSummary{}, f.analyticsSummaryErr
	}
	return store.AnalyticsSummary{}, nil
}

func (f failingDashboardStore) HistoricalSessions(ctx context.Context, limit int) ([]store.HistoricalSession, error) {
	if f.historicalSessionsErr != nil {
		return nil, f.historicalSessionsErr
	}
	return nil, nil
}

func (f failingDashboardStore) RecentAuditEvents(ctx context.Context, limit int) ([]store.AuditEvent, error) {
	if f.recentAuditEventsErr != nil {
		return nil, f.recentAuditEventsErr
	}
	return []store.AuditEvent{}, nil
}

func (f failingDashboardStore) PurgeHistoricalBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, f.purgeErr
}
