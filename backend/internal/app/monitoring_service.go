package app

import (
	"context"
	"fmt"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/metrics"
	"ocserv_panel/backend/internal/rbac"
)

// MonitoringService returns stored dashboard monitoring metrics.
type MonitoringService struct {
	store      metrics.Store
	recorder   audit.Recorder
	authorizer rbac.Authorizer
	now        func() time.Time
	catalog    *metrics.Catalog
}

// MonitoringOverview is a dashboard-ready snapshot of recent metric series.
type MonitoringOverview struct {
	Window      string                     `json:"window"`
	GeneratedAt time.Time                  `json:"generatedAt"`
	Catalog     []metrics.MetricDefinition `json:"catalog"`
	Series      []metrics.Series           `json:"series"`
}

// NewMonitoringService returns a monitoring service backed by a local metric store.
func NewMonitoringService(store metrics.Store, authorizer rbac.Authorizer, now func() time.Time, recorders ...audit.Recorder) *MonitoringService {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	recorder := audit.Recorder(audit.NoopRecorder{})
	if len(recorders) > 0 && recorders[0] != nil {
		recorder = recorders[0]
	}
	catalog := metrics.DefaultCatalog()

	return &MonitoringService{
		store:      store,
		recorder:   recorder,
		authorizer: authorizer,
		now:        now,
		catalog:    &catalog,
	}
}

// SetMetricState updates the state and reason of a metric in the monitoring catalog.
func (s *MonitoringService) SetMetricState(name metrics.MetricName, state metrics.State, reason string) {
	s.catalog.SetState(name, state, reason)
}

// Catalog returns metric definitions visible to dashboard readers.
func (s *MonitoringService) Catalog(ctx context.Context, principal auth.Principal) ([]metrics.MetricDefinition, error) {
	if err := s.require(ctx, principal, "monitoring.catalog"); err != nil {
		return nil, err
	}
	actorID := principal.ID
	if err := s.record(ctx, &actorID, "monitoring.catalog", audit.OutcomeSuccess, "monitoring catalog loaded", nil); err != nil {
		return nil, fmt.Errorf("audit monitoring catalog: %w", err)
	}

	return s.catalogList(), nil
}

// Overview returns core dashboard metric series for a validated window.
func (s *MonitoringService) Overview(ctx context.Context, principal auth.Principal, window string) (MonitoringOverview, error) {
	if err := s.require(ctx, principal, "monitoring.overview"); err != nil {
		return MonitoringOverview{}, err
	}
	actorID := principal.ID

	step := defaultMonitoringStep(window)
	windowDuration, stepDuration, err := metrics.ValidateWindowAndStep(window, step)
	if err != nil {
		return MonitoringOverview{}, err
	}

	now := s.now().UTC()
	series := make([]metrics.Series, 0, len(monitoringOverviewMetrics))
	for _, name := range monitoringOverviewMetrics {
		item, err := s.store.QueryRange(ctx, metrics.Query{
			Metric: name,
			Labels: map[string]string{},
			Start:  now.Add(-windowDuration),
			End:    now,
			Step:   stepDuration,
		})
		if err != nil {
			_ = s.record(ctx, &actorID, "monitoring.overview", audit.OutcomeFailure, "monitoring overview load failed", map[string]string{"metric": string(name)})
			return MonitoringOverview{}, fmt.Errorf("query monitoring overview metric: %w", err)
		}
		series = append(series, item)
	}
	if err := s.record(ctx, &actorID, "monitoring.overview", audit.OutcomeSuccess, "monitoring overview loaded", map[string]string{"window": window}); err != nil {
		return MonitoringOverview{}, fmt.Errorf("audit monitoring overview: %w", err)
	}

	return MonitoringOverview{
		Window:      window,
		GeneratedAt: now,
		Catalog:     s.catalogList(),
		Series:      series,
	}, nil
}

// Series returns one validated metric series for a dashboard reader.
func (s *MonitoringService) Series(ctx context.Context, principal auth.Principal, metric metrics.MetricName, window string, step string) (metrics.Series, error) {
	if err := s.require(ctx, principal, "monitoring.series"); err != nil {
		return metrics.Series{}, err
	}
	actorID := principal.ID
	if err := metrics.ValidateMetric(metric); err != nil {
		return metrics.Series{}, err
	}

	windowDuration, stepDuration, err := metrics.ValidateWindowAndStep(window, step)
	if err != nil {
		return metrics.Series{}, err
	}

	now := s.now().UTC()
	series, err := s.store.QueryRange(ctx, metrics.Query{
		Metric: metric,
		Labels: map[string]string{},
		Start:  now.Add(-windowDuration),
		End:    now,
		Step:   stepDuration,
	})
	if err != nil {
		_ = s.record(ctx, &actorID, "monitoring.series", audit.OutcomeFailure, "monitoring series load failed", map[string]string{"metric": string(metric)})
		return metrics.Series{}, fmt.Errorf("query monitoring series: %w", err)
	}
	if err := s.record(ctx, &actorID, "monitoring.series", audit.OutcomeSuccess, "monitoring series loaded", map[string]string{"metric": string(metric), "window": window, "step": step}); err != nil {
		return metrics.Series{}, fmt.Errorf("audit monitoring series: %w", err)
	}

	return series, nil
}

func (s *MonitoringService) require(ctx context.Context, principal auth.Principal, action string) error {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, action, audit.OutcomeDenied, "monitoring authentication required", nil)
		return ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionReadDashboard) {
		actorID := principal.ID
		_ = s.record(ctx, &actorID, action, audit.OutcomeDenied, "monitoring permission denied", nil)
		return ErrPermissionDenied
	}

	return nil
}

func (s *MonitoringService) record(ctx context.Context, actorID *int64, action string, outcome audit.Outcome, message string, metadata map[string]string) error {
	return s.recorder.Record(ctx, audit.Event{ActorID: actorID, Action: action, ResourceType: "monitoring", Outcome: outcome, Message: message, Metadata: metadata, CreatedAt: s.now()})
}

func defaultMonitoringStep(window string) string {
	switch window {
	case "15m":
		return "15s"
	case "7d", "30d":
		return "1h"
	default:
		return "60s"
	}
}

func (s *MonitoringService) catalogList() []metrics.MetricDefinition {
	definitions := make([]metrics.MetricDefinition, 0, len(monitoringCatalogMetrics))
	for _, name := range monitoringCatalogMetrics {
		definition, ok := s.catalog.Find(name)
		if !ok {
			continue
		}
		definitions = append(definitions, definition)
	}

	return definitions
}

var monitoringCatalogMetrics = []metrics.MetricName{
	metrics.MetricHostCPUUtilization,
	metrics.MetricHostMemoryUtilization,
	metrics.MetricNetInterfaceRXBytes,
	metrics.MetricNetInterfaceTXBytes,
	metrics.MetricVPNInterfaceRXBytes,
	metrics.MetricVPNInterfaceTXBytes,
	metrics.MetricUserRXBytes,
	metrics.MetricUserTXBytes,
	metrics.MetricActiveSessions,
	metrics.MetricConnectionRate,
	metrics.MetricConcurrentSessions,
	metrics.MetricConcurrentSessionsLimit,
}

var monitoringOverviewMetrics = []metrics.MetricName{
	metrics.MetricHostCPUUtilization,
	metrics.MetricHostMemoryUtilization,
	metrics.MetricNetInterfaceRXBytes,
	metrics.MetricNetInterfaceTXBytes,
	metrics.MetricVPNInterfaceRXBytes,
	metrics.MetricVPNInterfaceTXBytes,
	metrics.MetricActiveSessions,
	metrics.MetricConcurrentSessions,
	metrics.MetricConcurrentSessionsLimit,
}
