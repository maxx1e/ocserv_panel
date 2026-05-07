package app

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
	Storage   StorageHealth   `json:"storage"`
}

// VPNServerHealth describes the ocserv process and config state.
type VPNServerHealth struct {
	SystemdStatus       string `json:"systemdStatus"`
	ProcessRunning      bool   `json:"processRunning"`
	SocketPresent       bool   `json:"socketPresent"`
	ConfigPresent       bool   `json:"configPresent"`
	ConfigValid         bool   `json:"configValid"`
	ConfigErrors        string `json:"configErrors,omitempty"`
	PasswordFilePresent bool   `json:"passwordFilePresent"`
	BinaryPresent       bool   `json:"binaryPresent"`
	BinaryVersion       string `json:"binaryVersion,omitempty"`
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

// StorageItemHealth describes a single storage backend's health.
type StorageItemHealth struct {
	Engine    string `json:"engine"`
	SizeBytes int64  `json:"sizeBytes"`
	Status    string `json:"status"` // "healthy", "warning", "critical"
	Path      string `json:"path"`
}

// StorageHealth describes storage subsystem health.
type StorageHealth struct {
	SQLite StorageItemHealth `json:"sqlite"`
	TSDB   StorageItemHealth `json:"tsdb"`
}

// Storage size thresholds for health classification.
const (
	sqliteWarnBytes     int64 = 500 * 1024 * 1024      // 500 MiB
	sqliteCriticalBytes int64 = 1024 * 1024 * 1024     // 1 GiB
	tsdbWarnBytes       int64 = 2 * 1024 * 1024 * 1024 // 2 GiB
	tsdbCriticalBytes   int64 = 5 * 1024 * 1024 * 1024 // 5 GiB
)

// HealthService builds the system health snapshot.
type HealthService struct {
	readiness  *ocserv.ReadinessChecker
	store      adminCounter
	cfgFn      func() config.Config
	authorizer rbac.Authorizer
	tsdbPath   string
}

// NewHealthService returns a HealthService.
// readiness may be nil; VPN server fields will be reported as unknown when nil.
func NewHealthService(
	readiness *ocserv.ReadinessChecker,
	store adminCounter,
	cfgFn func() config.Config,
	authorizer rbac.Authorizer,
	tsdbPath string,
) *HealthService {
	return &HealthService{
		readiness:  readiness,
		store:      store,
		cfgFn:      cfgFn,
		authorizer: authorizer,
		tsdbPath:   tsdbPath,
	}
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
	storage := s.buildStorageHealth(cfg)

	return SystemHealthResult{VPNServer: vpn, Panel: panel, Logs: logs, Storage: storage}, nil
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
		SystemdStatus:       status,
		ProcessRunning:      r.ServiceRunning,
		SocketPresent:       r.SocketPresent,
		ConfigPresent:       r.ConfigPresent,
		ConfigValid:         r.ConfigValid,
		ConfigErrors:        r.ConfigErrors,
		PasswordFilePresent: r.PasswordFilePresent,
		BinaryPresent:       r.BinaryPresent,
		BinaryVersion:       r.BinaryVersion,
	}
}

func (s *HealthService) buildPanelHealth(ctx context.Context, cfg config.Config) PanelHealth {
	count, err := s.store.AdminCount(ctx)
	if err != nil {
		count = 0
	}
	configPath := cfg.Installation.ConfigPath
	dbPath := resolveDBPath(cfg)
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

// resolveDBPath derives the SQLite file path from the configured DSN.
// It handles "file:" prefix and query parameters (e.g. "file:ocserv_panel.db?mode=wal").
// If the resulting path is relative, it is resolved relative to DataDir.
func resolveDBPath(cfg config.Config) string {
	dsn := cfg.Database.DSN
	if dsn == "" {
		return fmt.Sprintf("%s/panel.db", cfg.Installation.DataDir)
	}
	// Strip "file:" prefix if present.
	path := strings.TrimPrefix(dsn, "file:")
	// Strip query string if present.
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		path = path[:idx]
	}
	if path == "" || path == ":memory:" {
		return fmt.Sprintf("%s/panel.db", cfg.Installation.DataDir)
	}
	// If the path is relative, resolve against DataDir.
	if !filepath.IsAbs(path) {
		path = filepath.Join(cfg.Installation.DataDir, path)
	}
	return path
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

func (s *HealthService) buildStorageHealth(cfg config.Config) StorageHealth {
	// SQLite
	dbPath := resolveDBPath(cfg)
	sqliteSize := fileSize(dbPath)
	sqliteStatus := storageStatus(sqliteSize, sqliteWarnBytes, sqliteCriticalBytes)

	// TSDB
	tsdbSize := dirSize(s.tsdbPath)
	tsdbStatus := storageStatus(tsdbSize, tsdbWarnBytes, tsdbCriticalBytes)
	tsdbDisplayPath := s.tsdbPath
	if tsdbDisplayPath == "" {
		tsdbDisplayPath = "(in-memory)"
	}

	return StorageHealth{
		SQLite: StorageItemHealth{
			Engine:    "sqlite",
			SizeBytes: sqliteSize,
			Status:    sqliteStatus,
			Path:      maskPath(dbPath, 1),
		},
		TSDB: StorageItemHealth{
			Engine:    "tsdb",
			SizeBytes: tsdbSize,
			Status:    tsdbStatus,
			Path:      maskPath(tsdbDisplayPath, 2),
		},
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

// fileSize returns the size in bytes of a single file. Returns 0 if the file does not exist or on error.
func fileSize(path string) int64 {
	if path == "" {
		return 0
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0
	}
	return info.Size()
}

// dirSizeMaxFiles is the maximum number of files to stat when computing directory size.
// This prevents unbounded latency on very large TSDB directories.
const dirSizeMaxFiles = 100_000

// dirSize returns total size of all regular files under a directory. Returns 0 on error.
// Walks at most dirSizeMaxFiles entries to bound latency.
func dirSize(path string) int64 {
	if path == "" {
		return 0
	}
	var total int64
	var count int
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors, continue walking
		}
		if !d.IsDir() {
			count++
			if count > dirSizeMaxFiles {
				return fs.ErrPermission // sentinel to stop walk
			}
			info, err := d.Info()
			if err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// storageStatus classifies a storage size into health status.
func storageStatus(size, warnThreshold, critThreshold int64) string {
	if size >= critThreshold {
		return "critical"
	}
	if size >= warnThreshold {
		return "warning"
	}
	return "healthy"
}

// maskPath returns only the last N path segments, prefixed with an ellipsis.
// For example, maskPath("/var/lib/ocserv-panel/panel.db", 1) returns ".../panel.db".
func maskPath(path string, segments int) string {
	if path == "" {
		return ""
	}
	if segments <= 0 {
		return "..."
	}
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) <= segments {
		return path
	}
	return "…/" + strings.Join(parts[len(parts)-segments:], "/")
}
