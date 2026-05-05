package app

import (
	"context"
	"fmt"
	"os"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/ocserv"
	"ocserv_panel/backend/internal/rbac"
)

// adminCounter is a minimal store interface for HealthService.
type adminCounter interface {
	AdminCount(context.Context) (int64, error)
}

// SystemHealthResult is the payload returned by GET /api/health/system.
type SystemHealthResult struct {
	VPNServer VPNServerHealth `json:"vpnServer"`
	Panel     PanelHealth     `json:"panel"`
	Logs      LogsHealth      `json:"logs"`
}

// VPNServerHealth describes the ocserv process and config state.
type VPNServerHealth struct {
	SystemdStatus  string `json:"systemdStatus"`
	ProcessRunning bool   `json:"processRunning"`
	SocketPresent  bool   `json:"socketPresent"`
	ConfigPresent  bool   `json:"configPresent"`
	ConfigValid    bool   `json:"configValid"`
	ConfigErrors   string `json:"configErrors,omitempty"`
	BinaryPresent  bool   `json:"binaryPresent"`
	BinaryVersion  string `json:"binaryVersion,omitempty"`
}

// PanelHealth describes the WebUI panel backend state.
type PanelHealth struct {
	BackendStatus  string `json:"backendStatus"`
	DbReachable    bool   `json:"dbReachable"`
	DbPath         string `json:"dbPath"`
	ConfigPresent  bool   `json:"configPresent"`
	ConfigPath     string `json:"configPath"`
	WebuiUserCount int64  `json:"webuiUserCount"`
}

// LogsHealth describes log file availability.
type LogsHealth struct {
	OcservLogEnabled bool   `json:"ocservLogEnabled"`
	OcservLogPresent bool   `json:"ocservLogPresent"`
	OcservLogPath    string `json:"ocservLogPath"`
	PanelLogEnabled  bool   `json:"panelLogEnabled"`
	PanelLogPresent  bool   `json:"panelLogPresent"`
	PanelLogPath     string `json:"panelLogPath"`
}

// HealthService builds the system health snapshot.
type HealthService struct {
	readiness  *ocserv.ReadinessChecker
	store      adminCounter
	cfgFn      func() config.Config
	authorizer rbac.Authorizer
}

// NewHealthService returns a HealthService.
// readiness may be nil; VPN server fields will be reported as unknown when nil.
func NewHealthService(readiness *ocserv.ReadinessChecker, store adminCounter, cfgFn func() config.Config, authorizer rbac.Authorizer) *HealthService {
	return &HealthService{readiness: readiness, store: store, cfgFn: cfgFn, authorizer: authorizer}
}

// SystemHealth returns the current system health snapshot.
func (s *HealthService) SystemHealth(ctx context.Context, principal auth.Principal) (SystemHealthResult, error) {
	if !principal.Authenticated() {
		return SystemHealthResult{}, ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionReadDashboard) {
		return SystemHealthResult{}, ErrPermissionDenied
	}

	cfg := s.cfgFn()

	vpn := s.buildVPNHealth(ctx)
	panel := s.buildPanelHealth(ctx, cfg)
	logs := s.buildLogsHealth(cfg)

	return SystemHealthResult{VPNServer: vpn, Panel: panel, Logs: logs}, nil
}

func (s *HealthService) buildVPNHealth(ctx context.Context) VPNServerHealth {
	if s.readiness == nil {
		return VPNServerHealth{SystemdStatus: "unknown"}
	}
	r, err := s.readiness.Check(ctx)
	if err != nil {
		return VPNServerHealth{SystemdStatus: "unknown"}
	}
	status := "unknown"
	if r.ServiceRunning {
		status = "active"
	} else if r.BinaryPresent {
		status = "inactive"
	}
	return VPNServerHealth{
		SystemdStatus:  status,
		ProcessRunning: r.ServiceRunning,
		SocketPresent:  r.SocketPresent,
		ConfigPresent:  r.ConfigPresent,
		ConfigValid:    r.ConfigValid,
		ConfigErrors:   r.ConfigErrors,
		BinaryPresent:  r.BinaryPresent,
		BinaryVersion:  r.BinaryVersion,
	}
}

func (s *HealthService) buildPanelHealth(ctx context.Context, cfg config.Config) PanelHealth {
	count, err := s.store.AdminCount(ctx)
	if err != nil {
		count = 0
	}
	configPath := cfg.Installation.ConfigPath
	dbPath := fmt.Sprintf("%s/panel.db", cfg.Installation.DataDir)
	configPresent := fileStatExists(configPath)
	dbReachable := fileStatExists(dbPath)
	return PanelHealth{
		BackendStatus:  "ok",
		DbReachable:    dbReachable,
		DbPath:         dbPath,
		ConfigPresent:  configPresent,
		ConfigPath:     configPath,
		WebuiUserCount: count,
	}
}

func (s *HealthService) buildLogsHealth(cfg config.Config) LogsHealth {
	enabled := cfg.Monitoring.LogFileReadEnabled
	ocservPath := cfg.Installation.OcservLogPath
	panelPath := cfg.Installation.PanelLogPath

	ocservPresent := false
	panelPresent := false
	if enabled {
		ocservPresent = fileStatExists(ocservPath)
		panelPresent = fileStatExists(panelPath)
	}
	return LogsHealth{
		OcservLogEnabled: enabled,
		OcservLogPresent: ocservPresent,
		OcservLogPath:    ocservPath,
		PanelLogEnabled:  enabled,
		PanelLogPresent:  panelPresent,
		PanelLogPath:     panelPath,
	}
}

// fileStatExists returns true if the path exists (file or directory).
func fileStatExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
