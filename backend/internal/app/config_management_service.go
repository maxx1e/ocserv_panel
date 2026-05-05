package app

import (
	"context"
	"errors"
	"fmt"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/ocserv"
	"ocserv_panel/backend/internal/rbac"
)

// Sentinel errors for ConfigManagementService operations.
var (
	ErrConfigValidationFailed = errors.New("config validation failed")
	ErrConfigFileLocked       = errors.New("config file is locked")
	ErrConfigBackupNotFound   = errors.New("config backup not found")
)

// ConfigReadResult carries the current config content, its filesystem metadata,
// and any validation issues found in the content.
type ConfigReadResult struct {
	Content  string
	Metadata ocserv.ConfigMetadata
	Issues   []ocserv.ConfigIssue
}

// ValidationResult carries whether content passed validation and any issues found.
// When restart detection is wired, RequiresRestart indicates whether the proposed
// changes would require a full service restart (vs a simple reload).
type ValidationResult struct {
	Valid           bool
	Issues          []ocserv.ConfigIssue
	RequiresRestart bool
	RestartReasons  []string
}

// ApplyResult carries the outcome of a config write and live reload attempt.
// A non-nil ReloadError indicates the file was written but ocserv rejected the
// new config. The file is not automatically rolled back — the caller decides.
type ApplyResult struct {
	BackupNumber  int
	ReloadSuccess bool
	ReloadError   string
	Issues        []ocserv.ConfigIssue
}

// RollbackResult carries the outcome of restoring a numbered backup and
// reloading ocserv.
type RollbackResult struct {
	ReloadSuccess bool
	ReloadError   string
}

// ConfigManagementService provides RBAC-guarded, audited operations for
// reading, validating, writing, and rolling back the ocserv configuration file.
type ConfigManagementService struct {
	files           *ocserv.ConfigFileManager
	validator       *ocserv.ConfigValidator
	reloader        *ocserv.ConfigReloader
	inspector       *ocserv.NetworkInspector
	restartDetector *ocserv.RestartDetector
	recorder        audit.Recorder
	authorizer      rbac.Authorizer
}

// NewConfigManagementService returns a ConfigManagementService wired with the
// given dependencies. A nil recorder is replaced by a no-op recorder.
func NewConfigManagementService(
	files *ocserv.ConfigFileManager,
	validator *ocserv.ConfigValidator,
	reloader *ocserv.ConfigReloader,
	inspector *ocserv.NetworkInspector,
	restartDetector *ocserv.RestartDetector,
	recorder audit.Recorder,
	authorizer rbac.Authorizer,
) *ConfigManagementService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	return &ConfigManagementService{
		files:           files,
		validator:       validator,
		reloader:        reloader,
		inspector:       inspector,
		restartDetector: restartDetector,
		recorder:        recorder,
		authorizer:      authorizer,
	}
}

// ReadConfig reads the current config file, validates its content, and returns
// the result. Requires PermissionOcservConfigRead.
func (s *ConfigManagementService) ReadConfig(ctx context.Context, principal auth.Principal) (ConfigReadResult, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservConfigRead, "ocserv.config.read"); err != nil {
		return ConfigReadResult{}, err
	}

	content, meta, err := s.files.Read()
	if err != nil {
		return ConfigReadResult{}, fmt.Errorf("read config file: %w", err)
	}

	issues, err := s.validator.ValidateContent(ctx, content)
	if err != nil {
		return ConfigReadResult{}, fmt.Errorf("validate config content: %w", err)
	}

	return ConfigReadResult{
		Content:  content,
		Metadata: meta,
		Issues:   issues,
	}, nil
}

// ListBackups returns metadata for all numbered config backups.
// Requires PermissionOcservConfigRead.
func (s *ConfigManagementService) ListBackups(ctx context.Context, principal auth.Principal) ([]ocserv.BackupInfo, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservConfigRead, "ocserv.config.backups.list"); err != nil {
		return nil, err
	}

	backups, err := s.files.ListBackups()
	if err != nil {
		return nil, fmt.Errorf("list config backups: %w", err)
	}
	return backups, nil
}

// GetBackup returns the content of backup number n.
// Requires PermissionOcservConfigRead. Maps ocserv.ErrBackupNotFound to
// ErrConfigBackupNotFound.
func (s *ConfigManagementService) GetBackup(ctx context.Context, principal auth.Principal, n int) (string, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservConfigRead, "ocserv.config.backup.get"); err != nil {
		return "", err
	}

	content, err := s.files.GetBackup(n)
	if err != nil {
		if errors.Is(err, ocserv.ErrBackupNotFound) {
			return "", ErrConfigBackupNotFound
		}
		return "", fmt.Errorf("get config backup %d: %w", n, err)
	}
	return content, nil
}

// DiffWithContent returns a unified diff between the current config file and the
// proposed content. Requires PermissionOcservConfigRead.
func (s *ConfigManagementService) DiffWithContent(ctx context.Context, principal auth.Principal, proposed string) (string, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservConfigRead, "ocserv.config.diff"); err != nil {
		return "", err
	}

	diff, err := s.files.DiffWithContent(proposed)
	if err != nil {
		return "", fmt.Errorf("diff config with proposed content: %w", err)
	}
	return diff, nil
}

// DiffWithBackup returns a unified diff between backup n and the current live
// config. Requires PermissionOcservConfigRead. Maps ocserv.ErrBackupNotFound to
// ErrConfigBackupNotFound.
func (s *ConfigManagementService) DiffWithBackup(ctx context.Context, principal auth.Principal, n int) (string, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservConfigRead, "ocserv.config.diff.backup"); err != nil {
		return "", err
	}

	diff, err := s.files.DiffWithBackup(n)
	if err != nil {
		if errors.Is(err, ocserv.ErrBackupNotFound) {
			return "", ErrConfigBackupNotFound
		}
		return "", fmt.Errorf("diff config with backup %d: %w", n, err)
	}
	return diff, nil
}

// ValidateContent validates proposed config content without writing to disk.
// Requires PermissionOcservConfigRead. Produces an audit record.
// Returns ErrConfigValidationFailed when error-severity issues are present.
// When a RestartDetector is wired, the result includes restart analysis.
func (s *ConfigManagementService) ValidateContent(ctx context.Context, principal auth.Principal, content string) (ValidationResult, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservConfigRead, "ocserv.config.validate"); err != nil {
		return ValidationResult{}, err
	}

	issues, err := s.validator.ValidateContent(ctx, content)
	if err != nil {
		return ValidationResult{}, fmt.Errorf("validate config content: %w", err)
	}

	valid := !hasErrorSeverity(issues)
	actorID := principal.ID
	if valid {
		_ = s.record(ctx, &actorID, "ocserv.config.validate", "ocserv_config", "", audit.OutcomeSuccess, "config validation passed", nil)
	} else {
		_ = s.record(ctx, &actorID, "ocserv.config.validate", "ocserv_config", "", audit.OutcomeFailure, "config validation failed", nil)
	}

	result := ValidationResult{Valid: valid, Issues: issues}

	// Run restart detection if the detector is wired and current config is readable.
	currentContent, _, readErr := s.files.Read()
	if readErr == nil && s.restartDetector != nil {
		analysis := s.restartDetector.Detect(currentContent, content)
		result.RequiresRestart = analysis.RequiresRestart
		result.RestartReasons = analysis.Reasons
	}

	return result, nil
}

// ApplyConfig validates content, writes it to disk, then reloads ocserv.
// Requires PermissionOcservConfigEdit.
//
// Validation errors block the write — returns ErrConfigValidationFailed when
// error-severity issues are found. ErrConfigTooLarge and ErrConfigLocked from
// the file layer are mapped to ErrConfigValidationFailed and ErrConfigFileLocked
// respectively.
//
// A successful write followed by a reload failure returns a non-error ApplyResult
// with ReloadSuccess=false and a populated ReloadError. This lets callers surface
// partial success to the user (the file changed, but ocserv rejected the config).
func (s *ConfigManagementService) ApplyConfig(ctx context.Context, principal auth.Principal, content string) (ApplyResult, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservConfigEdit, "ocserv.config.apply"); err != nil {
		return ApplyResult{}, err
	}

	actorID := principal.ID

	// Validate before writing — block on any error-severity issue.
	issues, err := s.validator.ValidateContent(ctx, content)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("validate config content: %w", err)
	}
	if hasErrorSeverity(issues) {
		_ = s.record(ctx, &actorID, "ocserv.config.apply", "ocserv_config", "", audit.OutcomeFailure, "config apply blocked by validation errors", nil)
		return ApplyResult{Issues: issues}, ErrConfigValidationFailed
	}

	// Write to disk with backup rotation.
	if err := s.files.Write(content); err != nil {
		switch {
		case errors.Is(err, ocserv.ErrConfigTooLarge):
			_ = s.record(ctx, &actorID, "ocserv.config.apply", "ocserv_config", "", audit.OutcomeFailure, "config too large", nil)
			return ApplyResult{}, ErrConfigValidationFailed
		case errors.Is(err, ocserv.ErrConfigLocked):
			_ = s.record(ctx, &actorID, "ocserv.config.apply", "ocserv_config", "", audit.OutcomeFailure, "config locked", nil)
			return ApplyResult{}, ErrConfigFileLocked
		default:
			_ = s.record(ctx, &actorID, "ocserv.config.apply", "ocserv_config", "", audit.OutcomeFailure, "config write failed", nil)
			return ApplyResult{}, fmt.Errorf("write config file: %w", err)
		}
	}

	// Determine the backup number just created (always .1 after a successful write).
	result := ApplyResult{
		BackupNumber: 1,
		Issues:       issues,
	}

	// Attempt live reload — failure is a partial success, not a hard error.
	reloadResult, reloadErr := s.reloader.Reload(ctx)
	if reloadErr != nil {
		result.ReloadSuccess = false
		result.ReloadError = reloadErr.Error()
		_ = s.record(ctx, &actorID, "ocserv.config.apply", "ocserv_config", "", audit.OutcomeSuccess, "config written; reload command failed", map[string]string{
			"reload_error": reloadErr.Error(),
		})
		return result, nil
	}

	result.ReloadSuccess = reloadResult.Success
	result.ReloadError = reloadResult.Stderr

	if reloadResult.Success {
		_ = s.record(ctx, &actorID, "ocserv.config.apply", "ocserv_config", "", audit.OutcomeSuccess, "config applied and reloaded", nil)
	} else {
		_ = s.record(ctx, &actorID, "ocserv.config.apply", "ocserv_config", "", audit.OutcomeSuccess, "config written; ocserv reload rejected", map[string]string{
			"reload_stderr": reloadResult.Stderr,
		})
	}

	return result, nil
}

// Rollback restores backup number n as the live config and reloads ocserv.
// Requires PermissionOcservConfigRollback. Maps ocserv.ErrBackupNotFound to
// ErrConfigBackupNotFound.
//
// A successful restore followed by a reload failure returns a non-error
// RollbackResult with ReloadSuccess=false.
func (s *ConfigManagementService) Rollback(ctx context.Context, principal auth.Principal, n int) (RollbackResult, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservConfigRollback, "ocserv.config.rollback"); err != nil {
		return RollbackResult{}, err
	}

	actorID := principal.ID

	if err := s.files.Rollback(n); err != nil {
		switch {
		case errors.Is(err, ocserv.ErrBackupNotFound):
			_ = s.record(ctx, &actorID, "ocserv.config.rollback", "ocserv_config", fmt.Sprint(n), audit.OutcomeFailure, "backup not found", nil)
			return RollbackResult{}, ErrConfigBackupNotFound
		case errors.Is(err, ocserv.ErrConfigLocked):
			_ = s.record(ctx, &actorID, "ocserv.config.rollback", "ocserv_config", fmt.Sprint(n), audit.OutcomeFailure, "config locked", nil)
			return RollbackResult{}, ErrConfigFileLocked
		default:
			_ = s.record(ctx, &actorID, "ocserv.config.rollback", "ocserv_config", fmt.Sprint(n), audit.OutcomeFailure, "rollback write failed", nil)
			return RollbackResult{}, fmt.Errorf("rollback to backup %d: %w", n, err)
		}
	}

	result := RollbackResult{}

	reloadResult, reloadErr := s.reloader.Reload(ctx)
	if reloadErr != nil {
		result.ReloadSuccess = false
		result.ReloadError = reloadErr.Error()
		_ = s.record(ctx, &actorID, "ocserv.config.rollback", "ocserv_config", fmt.Sprint(n), audit.OutcomeSuccess, "config rolled back; reload command failed", map[string]string{
			"reload_error": reloadErr.Error(),
		})
		return result, nil
	}

	result.ReloadSuccess = reloadResult.Success
	result.ReloadError = reloadResult.Stderr

	if reloadResult.Success {
		_ = s.record(ctx, &actorID, "ocserv.config.rollback", "ocserv_config", fmt.Sprint(n), audit.OutcomeSuccess, "config rolled back and reloaded", nil)
	} else {
		_ = s.record(ctx, &actorID, "ocserv.config.rollback", "ocserv_config", fmt.Sprint(n), audit.OutcomeSuccess, "config rolled back; ocserv reload rejected", map[string]string{
			"reload_stderr": reloadResult.Stderr,
		})
	}

	return result, nil
}

// InspectNetwork runs host-level network prerequisite checks for ocserv.
// Requires PermissionOcservConfigRead.
func (s *ConfigManagementService) InspectNetwork(ctx context.Context, principal auth.Principal) (ocserv.NetworkPrerequisites, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservConfigRead, "ocserv.config.network.inspect"); err != nil {
		return ocserv.NetworkPrerequisites{}, err
	}

	prereqs, err := s.inspector.Inspect(ctx)
	if err != nil {
		return ocserv.NetworkPrerequisites{}, fmt.Errorf("inspect network prerequisites: %w", err)
	}
	return prereqs, nil
}

// --- private helpers ---

func (s *ConfigManagementService) requirePerm(
	ctx context.Context,
	principal auth.Principal,
	permission rbac.Permission,
	action string,
) error {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, action, "ocserv_config", "", audit.OutcomeDenied, "authentication required", nil)
		return ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), permission) {
		actorID := principal.ID
		_ = s.record(ctx, &actorID, action, "ocserv_config", "", audit.OutcomeDenied, "permission denied", nil)
		return ErrPermissionDenied
	}
	return nil
}

func (s *ConfigManagementService) record(
	ctx context.Context,
	actorID *int64,
	action, resourceType, resourceID string,
	outcome audit.Outcome,
	message string,
	metadata map[string]string,
) error {
	return s.recorder.Record(ctx, audit.Event{
		ActorID:      actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Outcome:      outcome,
		Message:      message,
		Metadata:     metadata,
	})
}

// hasErrorSeverity reports whether any issue has IssueError severity.
func hasErrorSeverity(issues []ocserv.ConfigIssue) bool {
	for _, issue := range issues {
		if issue.Severity == ocserv.IssueError {
			return true
		}
	}
	return false
}
