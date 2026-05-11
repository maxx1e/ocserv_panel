package server

import (
	"context"
	"math"
	"os"
	"runtime"

	"ocserv_panel/backend/internal/monitor"
	"time"
)

// monitoringOverviewResponse is the response for GET /api/monitoring/overview.
type monitoringOverviewResponse struct {
	Window      string                     `json:"window"`
	GeneratedAt time.Time                  `json:"generatedAt"`
	Catalog     []monitor.MetricDefinition `json:"catalog"`
	Series      []monitor.Series           `json:"series"`
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
		if totalKB, _, parseErr := monitor.ParseMeminfo(string(data)); parseErr == nil && totalKB > 0 {
			gb := float64(totalKB) / (1024 * 1024)
			info.MemoryTotalGB = math.Round(gb*10) / 10
		}
	}

	if lbls, err := a.metricStore.QueryLatestLabels(ctx, monitor.MetricNetInterfaceRXBytes); err == nil {
		info.NetInterface = lbls["interface"]
	}

	if labelSets, err := a.metricStore.QueryAllLabels(ctx, monitor.MetricVPNInterfaceRXBytes); err == nil {
		for _, ls := range labelSets {
			if iface := ls["interface"]; iface != "" {
				info.VPNInterfaces = append(info.VPNInterfaces, iface)
			}
		}
	}

	return info
}

// catalogList returns metric definitions for the monitoring catalog.
func (a *App) catalogList() []monitor.MetricDefinition {
	definitions := make([]monitor.MetricDefinition, 0, len(monitoringCatalogMetrics))
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

var monitoringCatalogMetrics = []monitor.MetricName{
	monitor.MetricHostCPUUtilization,
	monitor.MetricHostMemoryUtilization,
	monitor.MetricNetInterfaceRXBytes,
	monitor.MetricNetInterfaceTXBytes,
	monitor.MetricVPNInterfaceRXBytes,
	monitor.MetricVPNInterfaceTXBytes,
	monitor.MetricUserRXBytes,
	monitor.MetricUserTXBytes,
	monitor.MetricActiveSessions,
	monitor.MetricConnectionRate,
	monitor.MetricConcurrentSessions,
	monitor.MetricConcurrentSessionsLimit,
}

var monitoringOverviewMetrics = []monitor.MetricName{
	monitor.MetricHostCPUUtilization,
	monitor.MetricHostMemoryUtilization,
	monitor.MetricNetInterfaceRXBytes,
	monitor.MetricNetInterfaceTXBytes,
	monitor.MetricVPNInterfaceRXBytes,
	monitor.MetricVPNInterfaceTXBytes,
	monitor.MetricActiveSessions,
	monitor.MetricConnectionRate,
	monitor.MetricConcurrentSessions,
	monitor.MetricConcurrentSessionsLimit,
}
