package ocserv

import (
	"ocserv_panel/backend/internal/ocserv/config"
	"ocserv_panel/backend/internal/ocserv/service"
)

// --- config sub-package re-exports ---

// Type aliases for backward compatibility.
// New code should import ocserv_panel/backend/internal/ocserv/config directly.
type ConfigFileManager = config.ConfigFileManager
type ConfigMetadata = config.ConfigMetadata
type BackupInfo = config.BackupInfo
type ConfigIssueSeverity = config.ConfigIssueSeverity
type ConfigIssue = config.ConfigIssue
type ConfigValidator = config.ConfigValidator
type ConfigReloader = config.ConfigReloader
type ReloadResult = config.ReloadResult
type RestartDetector = config.RestartDetector
type RestartAnalysis = config.RestartAnalysis

// Sentinel errors from config package.
var (
	ErrConfigLocked   = config.ErrConfigLocked
	ErrBackupNotFound = config.ErrBackupNotFound
	ErrConfigTooLarge = config.ErrConfigTooLarge
)

// Constants from config package.
const (
	IssueError   = config.IssueError
	IssueWarning = config.IssueWarning
	IssueInfo    = config.IssueInfo
)

// Constructor functions from config package.
var (
	NewConfigFileManager = config.NewConfigFileManager
	NewConfigValidator   = config.NewConfigValidator
	NewConfigReloader    = config.NewConfigReloader
	NewRestartDetector   = config.NewRestartDetector
)

// parseConfigLine delegates to config.ParseConfigLine for internal callers
// (e.g., network_inspector.go) that use the unexported name.
func parseConfigLine(line string) (key, value string, ok bool) {
	return config.ParseConfigLine(line)
}

// --- service sub-package re-exports ---

// Type aliases for service types.
// New code should import ocserv_panel/backend/internal/ocserv/service directly.
type ServiceStatus = service.ServiceStatus
type ServiceManager = service.ServiceManager
type SystemdManager = service.SystemdManager
type FakeServiceManager = service.FakeServiceManager

// Service status constants.
const (
	ServiceActive   = service.ServiceActive
	ServiceInactive = service.ServiceInactive
	ServiceFailed   = service.ServiceFailed
	ServiceNotFound = service.ServiceNotFound
)

// Constructor functions from service package.
var (
	NewSystemdManager     = service.NewSystemdManager
	NewFakeServiceManager = service.NewFakeServiceManager
)
