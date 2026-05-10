package server

import (
	"context"
	"math"
	"os"
	"runtime"

	"ocserv_panel/backend/internal/metrics"
	"ocserv_panel/backend/internal/monitoring"
	"time"
)

// monitoringOverviewResponse is the response for GET /api/monitoring/overview.
type monitoringOverviewResponse struct {
	Window      string                     `json:"window"`
	GeneratedAt time.Time                  `json:"generatedAt"`
	Catalog     []metrics.MetricDefinition `json:"catalog"`
	Series      []metrics.Series           `json:"series"`
}

// systemInfoResponse is the response for GET /api/monitoring/system-info.
type systemInfoResponse struct {
	CPUCores      int      `json:"cpuCores"`
	MemoryTotalGB float64  `json:"memoryTotalGB"`
	NetInterface  string   `json:"netInterface"`
	VPNInterfaces []string `json:"vpnInterfaces"`
}

func (a *App) buildSystemInfo(ctx context.Context) systemInfoResponse {
	info := systemInfoResponse{
		CPUCores:      runtime.NumCPU(),
		VPNInterfaces: []string{},
	}

	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		if totalKB, _, parseErr := monitoring.ParseMeminfo(string(data)); parseErr == nil && totalKB > 0 {
			gb := float64(totalKB) / (1024 * 1024)
			info.MemoryTotalGB = math.Round(gb*10) / 10
		}
	}

	if lbls, err := a.metricStore.QueryLatestLabels(ctx, metrics.MetricNetInterfaceRXBytes); err == nil {
		info.NetInterface = lbls["interface"]
	}

	if lbls, err := a.metricStore.QueryLatestLabels(ctx, metrics.MetricVPNInterfaceRXBytes); err == nil {
		if iface := lbls["interface"]; iface != "" {
			info.VPNInterfaces = []string{iface}
		}
	}

	return info
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
