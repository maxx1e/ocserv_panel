package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/metrics"
	"ocserv_panel/backend/internal/rbac"
)

func TestMonitoringServiceCatalogRequiresDashboardReadPermission(t *testing.T) {
	ctx := context.Background()
	service := NewMonitoringService(metrics.NewMemoryStore(), rbac.NewDefaultAuthorizer(), fixedMonitoringNow)

	if _, err := service.Catalog(ctx, auth.Principal{}); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("catalog unauthenticated err = %v, want ErrUnauthenticated", err)
	}
	if _, err := service.Catalog(ctx, monitoringPrincipalWithoutRoles()); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("catalog unauthorized err = %v, want ErrPermissionDenied", err)
	}

	catalog, err := service.Catalog(ctx, monitoringViewerPrincipal())
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	assertMetricDefinition(t, catalog, metrics.MetricActiveSessions, metrics.StateCollecting)
	assertMetricDefinition(t, catalog, metrics.MetricConcurrentSessionsLimit, metrics.StateCollecting)
}

func TestMonitoringServiceOverviewRequiresPermissionAndQueriesCoreMetrics(t *testing.T) {
	ctx := context.Background()
	now := fixedMonitoringNow()
	store := metrics.NewMemoryStore()
	service := NewMonitoringService(store, rbac.NewDefaultAuthorizer(), fixedMonitoringNow)

	if err := store.Append(ctx, []metrics.Sample{
		{
			Name:      metrics.MetricHostCPUUtilization,
			Timestamp: now.Add(-30 * time.Minute),
			Value:     40,
		},
		{
			Name:      metrics.MetricHostCPUUtilization,
			Timestamp: now.Add(-30*time.Minute + 20*time.Second),
			Value:     41,
		},
		{
			Name:      metrics.MetricHostMemoryUtilization,
			Timestamp: now.Add(-2 * time.Hour),
			Value:     80,
		},
	}); err != nil {
		t.Fatalf("append samples: %v", err)
	}

	if _, err := service.Overview(ctx, monitoringPrincipalWithoutRoles(), "1h"); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("overview unauthorized err = %v, want ErrPermissionDenied", err)
	}

	overview, err := service.Overview(ctx, monitoringViewerPrincipal(), "1h")
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if overview.Window != "1h" || !overview.GeneratedAt.Equal(now) {
		t.Fatalf("overview metadata = %#v, want fixed 1h overview", overview)
	}
	if len(overview.Catalog) == 0 {
		t.Fatal("overview catalog is empty, want default metric definitions")
	}
	if len(overview.Series) != 10 {
		t.Fatalf("overview series count = %d, want 10 core dashboard metrics", len(overview.Series))
	}

	cpu := findSeries(t, overview.Series, metrics.MetricHostCPUUtilization)
	if len(cpu.Points) != 1 || cpu.Points[0].Value != 41 {
		t.Fatalf("cpu points = %#v, want 1h default 60s bucket with latest value 41", cpu.Points)
	}
	memory := findSeries(t, overview.Series, metrics.MetricHostMemoryUtilization)
	if len(memory.Points) != 0 {
		t.Fatalf("memory points = %#v, want samples outside 1h window excluded", memory.Points)
	}
	findSeries(t, overview.Series, metrics.MetricActiveSessions)
	findSeries(t, overview.Series, metrics.MetricConnectionRate)
	findSeries(t, overview.Series, metrics.MetricConcurrentSessionsLimit)
}

func TestMonitoringServiceOverviewSupportsAllDefaultWindows(t *testing.T) {
	ctx := context.Background()
	service := NewMonitoringService(metrics.NewMemoryStore(), rbac.NewDefaultAuthorizer(), fixedMonitoringNow)
	principal := monitoringViewerPrincipal()

	tests := []struct {
		name   string
		window string
	}{
		{name: "15 minutes", window: "15m"},
		{name: "1 hour", window: "1h"},
		{name: "6 hours", window: "6h"},
		{name: "24 hours", window: "24h"},
		{name: "7 days", window: "7d"},
		{name: "30 days", window: "30d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overview, err := service.Overview(ctx, principal, tt.window)
			if err != nil {
				t.Fatalf("overview window %q: %v", tt.window, err)
			}
			if overview.Window != tt.window {
				t.Fatalf("overview window = %q, want %q", overview.Window, tt.window)
			}
			if len(overview.Series) != len(monitoringOverviewMetrics) {
				t.Fatalf("overview series count = %d, want %d", len(overview.Series), len(monitoringOverviewMetrics))
			}
		})
	}
}

func TestMonitoringServiceSeriesValidationErrors(t *testing.T) {
	ctx := context.Background()
	service := NewMonitoringService(metrics.NewMemoryStore(), rbac.NewDefaultAuthorizer(), fixedMonitoringNow)
	principal := monitoringViewerPrincipal()

	tests := []struct {
		name      string
		metric    metrics.MetricName
		window    string
		step      string
		wantError error
	}{
		{
			name:      "invalid metric",
			metric:    metrics.MetricName("not_a_metric"),
			window:    "1h",
			step:      "60s",
			wantError: metrics.ErrInvalidMetric,
		},
		{
			name:      "invalid window",
			metric:    metrics.MetricHostCPUUtilization,
			window:    "2h",
			step:      "60s",
			wantError: metrics.ErrInvalidWindow,
		},
		{
			name:      "invalid step",
			metric:    metrics.MetricHostCPUUtilization,
			window:    "1h",
			step:      "30s",
			wantError: metrics.ErrInvalidStep,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Series(ctx, principal, tt.metric, tt.window, tt.step)
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("series err = %v, want %v", err, tt.wantError)
			}
		})
	}
}

func TestMonitoringServiceSeriesReturnsPointsInsideWindow(t *testing.T) {
	ctx := context.Background()
	now := fixedMonitoringNow()
	store := metrics.NewMemoryStore()
	service := NewMonitoringService(store, rbac.NewDefaultAuthorizer(), fixedMonitoringNow)

	if err := store.Append(ctx, []metrics.Sample{
		{
			Name:      metrics.MetricNetInterfaceRXBytes,
			Timestamp: now.Add(-61 * time.Minute),
			Value:     1,
		},
		{
			Name:      metrics.MetricNetInterfaceRXBytes,
			Timestamp: now.Add(-30 * time.Minute),
			Value:     2,
		},
		{
			Name:      metrics.MetricNetInterfaceRXBytes,
			Timestamp: now.Add(1 * time.Minute),
			Value:     3,
		},
	}); err != nil {
		t.Fatalf("append samples: %v", err)
	}

	series, err := service.Series(ctx, monitoringViewerPrincipal(), metrics.MetricNetInterfaceRXBytes, "1h", "60s")
	if err != nil {
		t.Fatalf("series: %v", err)
	}
	if series.Metric != metrics.MetricNetInterfaceRXBytes {
		t.Fatalf("series metric = %q, want %q", series.Metric, metrics.MetricNetInterfaceRXBytes)
	}
	if len(series.Points) != 1 || series.Points[0].Value != 2 {
		t.Fatalf("series points = %#v, want only point inside 1h window", series.Points)
	}
}

func fixedMonitoringNow() time.Time {
	return time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
}

func monitoringViewerPrincipal() auth.Principal {
	return auth.Principal{ID: 1, Username: "viewer", Roles: []string{string(rbac.RoleViewer)}}
}

func monitoringPrincipalWithoutRoles() auth.Principal {
	return auth.Principal{ID: 2, Username: "limited"}
}

func assertMetricDefinition(t *testing.T, catalog []metrics.MetricDefinition, name metrics.MetricName, state metrics.State) {
	t.Helper()
	for _, definition := range catalog {
		if definition.Name == name && definition.State == state {
			return
		}
	}
	t.Fatalf("catalog = %#v, want metric %q with state %q", catalog, name, state)
}

func findSeries(t *testing.T, series []metrics.Series, name metrics.MetricName) metrics.Series {
	t.Helper()
	for _, item := range series {
		if item.Metric == name {
			return item
		}
	}
	t.Fatalf("series = %#v, want metric %q", series, name)
	return metrics.Series{}
}
