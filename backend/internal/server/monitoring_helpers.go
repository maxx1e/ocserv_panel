package server

import (
	"ocserv_panel/backend/internal/metrics"
	"time"
)

// monitoringOverviewResponse is the response for GET /api/monitoring/overview.
type monitoringOverviewResponse struct {
	Window      string                     `json:"window"`
	GeneratedAt time.Time                  `json:"generatedAt"`
	Catalog     []metrics.MetricDefinition `json:"catalog"`
	Series      []metrics.Series           `json:"series"`
}

// catalogList returns metric definitions for the monitoring catalog.
func (a *App) catalogList() []metrics.MetricDefinition {
	definitions := make([]metrics.MetricDefinition, 0, len(monitoringCatalogMetrics))
	for _, name := range monitoringCatalogMetrics {
		definition, ok := a.metricCatalog.Find(name)
		if !ok {
			continue
		}
		definitions = append(definitions, definition)
	}
	return definitions
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
	metrics.MetricConnectionRate,
	metrics.MetricConcurrentSessions,
	metrics.MetricConcurrentSessionsLimit,
}
