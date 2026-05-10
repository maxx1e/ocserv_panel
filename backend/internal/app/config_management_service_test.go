package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/ocserv"
	"ocserv_panel/backend/internal/ocserv/cmd"
	ocservconfig "ocserv_panel/backend/internal/ocserv/config"
	"ocserv_panel/backend/internal/rbac"
)

// newTestConfigService builds a ConfigManagementService backed by a temporary
// directory so tests do not touch the real filesystem.
func newTestConfigService(t *testing.T) (*ConfigManagementService, string) {
	t.Helper()

	dir := t.TempDir()
	confPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(confPath, []byte("tcp-port = 443\n"), 0640); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	files := ocservconfig.NewConfigFileManager(confPath, 10)
	validator := ocservconfig.NewConfigValidator(confPath)

	runner := cmd.NewFakeCommandRunner()
	runner.Results["occtl reload"] = cmd.CommandResult{ExitCode: 0}
	reloader := ocservconfig.NewConfigReloader(runner, "occtl")

	inspector := ocserv.NewNetworkInspectorWithProcDir(runner, dir)

	svc := NewConfigManagementService(
		files,
		validator,
		reloader,
		inspector,
		ocservconfig.NewRestartDetector(),
		audit.NoopRecorder{},
		rbac.NewDefaultAuthorizer(),
	)
	return svc, confPath
}

// --- ReadConfig ---

func TestConfigServiceReadConfigRequiresAuth(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	_, err := svc.ReadConfig(context.Background(), auth.Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestConfigServiceReadConfigViewerAllowed(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	result, err := svc.ReadConfig(context.Background(), viewerPrincipal())
	if err != nil {
		t.Fatalf("expected viewer to read config, got %v", err)
	}
	if !strings.Contains(result.Content, "tcp-port") {
		t.Errorf("expected config content to contain 'tcp-port', got %q", result.Content)
	}
}

// --- ApplyConfig ---

func TestConfigServiceApplyConfigRequiresEditPerm(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	_, err := svc.ApplyConfig(context.Background(), viewerPrincipal(), "tcp-port = 443\n")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for viewer, got %v", err)
	}
}

func TestConfigServiceApplyConfigSuccess(t *testing.T) {
	t.Parallel()

	svc, confPath := newTestConfigService(t)
	newContent := "tcp-port = 1194\n"

	result, err := svc.ApplyConfig(context.Background(), ownerPrincipal(), newContent)
	if err != nil {
		t.Fatalf("expected apply to succeed, got %v", err)
	}
	if !result.ReloadSuccess {
		t.Errorf("expected reload success, reload error: %q", result.ReloadError)
	}

	// Verify the file changed on disk.
	data, readErr := os.ReadFile(confPath)
	if readErr != nil {
		t.Fatalf("read config file: %v", readErr)
	}
	if string(data) != newContent {
		t.Errorf("expected file content %q, got %q", newContent, string(data))
	}

	// Verify a backup was created.
	backupPath := confPath + ".1"
	if _, statErr := os.Stat(backupPath); statErr != nil {
		t.Errorf("expected backup .1 to exist: %v", statErr)
	}
}

func TestConfigServiceApplyConfigBlocksOnValidationError(t *testing.T) {
	t.Parallel()

	svc, confPath := newTestConfigService(t)
	invalidContent := "tcp-port = 99999\n"

	_, err := svc.ApplyConfig(context.Background(), ownerPrincipal(), invalidContent)
	if !errors.Is(err, ErrConfigValidationFailed) {
		t.Fatalf("expected ErrConfigValidationFailed, got %v", err)
	}

	// File must not have changed.
	data, readErr := os.ReadFile(confPath)
	if readErr != nil {
		t.Fatalf("read config file: %v", readErr)
	}
	if string(data) != "tcp-port = 443\n" {
		t.Errorf("expected original file content after validation failure, got %q", string(data))
	}
}

func TestConfigServiceApplyConfigRejectsTooLarge(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	// Build a content string just over 64 KB.
	hugeContent := strings.Repeat("# padding\n", 7000) // ~70 KB
	_, err := svc.ApplyConfig(context.Background(), ownerPrincipal(), hugeContent)
	if err == nil {
		t.Fatal("expected error for oversized content, got nil")
	}
}

// --- Rollback ---

func TestConfigServiceRollbackRequiresPermission(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	_, err := svc.Rollback(context.Background(), viewerPrincipal(), 1)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for viewer, got %v", err)
	}
}

func TestConfigServiceRollbackSuccess(t *testing.T) {
	t.Parallel()

	svc, confPath := newTestConfigService(t)

	// Apply new content to create a backup of the original.
	firstApply := "tcp-port = 1194\n"
	if _, err := svc.ApplyConfig(context.Background(), ownerPrincipal(), firstApply); err != nil {
		t.Fatalf("first apply failed: %v", err)
	}

	// Verify current content is the new one.
	data, _ := os.ReadFile(confPath)
	if string(data) != firstApply {
		t.Fatalf("expected first-apply content, got %q", string(data))
	}

	// Roll back to backup 1 (the original content).
	result, err := svc.Rollback(context.Background(), ownerPrincipal(), 1)
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if !result.ReloadSuccess {
		t.Errorf("expected reload success after rollback, reload error: %q", result.ReloadError)
	}

	// After rollback the original content should be restored.
	data, _ = os.ReadFile(confPath)
	if string(data) != "tcp-port = 443\n" {
		t.Errorf("expected original content after rollback, got %q", string(data))
	}
}

func TestConfigServiceRollbackBackupNotFound(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	// No backups exist yet, so backup 1 should not be found.
	_, err := svc.Rollback(context.Background(), ownerPrincipal(), 1)
	if !errors.Is(err, ErrConfigBackupNotFound) {
		t.Fatalf("expected ErrConfigBackupNotFound, got %v", err)
	}
}

// --- ValidateContent ---

func TestConfigServiceValidateContentPassesForValid(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	result, err := svc.ValidateContent(context.Background(), ownerPrincipal(), "tcp-port = 443\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid=true, issues: %v", result.Issues)
	}
}

func TestConfigServiceValidateContentFailsForInvalidPort(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	result, err := svc.ValidateContent(context.Background(), viewerPrincipal(), "tcp-port = 99999\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected valid=false for out-of-range port")
	}
	if len(result.Issues) == 0 {
		t.Error("expected at least one issue")
	}
}

// --- ListBackups and GetBackup ---

func TestConfigServiceListBackupsRequiresAuth(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	_, err := svc.ListBackups(context.Background(), auth.Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestConfigServiceGetBackupNotFound(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	_, err := svc.GetBackup(context.Background(), viewerPrincipal(), 99)
	if !errors.Is(err, ErrConfigBackupNotFound) {
		t.Fatalf("expected ErrConfigBackupNotFound, got %v", err)
	}
}

// --- DiffWithContent ---

func TestConfigServiceDiffWithContentRequiresAuth(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	_, err := svc.DiffWithContent(context.Background(), auth.Principal{}, "tcp-port = 1194\n")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestConfigServiceDiffWithContentViewerAllowed(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	diff, err := svc.DiffWithContent(context.Background(), viewerPrincipal(), "tcp-port = 1194\n")
	if err != nil {
		t.Fatalf("expected viewer to diff, got %v", err)
	}
	if !strings.Contains(diff, "1194") {
		t.Errorf("expected diff to mention new port, got %q", diff)
	}
}

// --- InspectNetwork ---

func TestConfigServiceInspectNetworkRequiresAuth(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	_, err := svc.InspectNetwork(context.Background(), auth.Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestConfigServiceInspectNetworkViewerAllowed(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	prereqs, err := svc.InspectNetwork(context.Background(), viewerPrincipal())
	if err != nil {
		t.Fatalf("expected viewer to inspect network, got %v", err)
	}
	if len(prereqs.Items) == 0 {
		t.Error("expected at least one prerequisite check item")
	}
}

// --- ValidateContent restart detection ---

func TestConfigServiceValidateContentReportsRestart(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	// The test config file contains "tcp-port = 443" — change it to 8443.
	result, err := svc.ValidateContent(context.Background(), ownerPrincipal(), "tcp-port = 8443\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.RequiresRestart {
		t.Error("expected RequiresRestart=true for port change")
	}
	if len(result.RestartReasons) == 0 {
		t.Error("expected at least one restart reason")
	}
}

func TestConfigServiceValidateContentNoRestartForDNS(t *testing.T) {
	t.Parallel()

	svc, _ := newTestConfigService(t)
	// DNS is not a restart-required directive; keep tcp-port unchanged.
	result, err := svc.ValidateContent(context.Background(), ownerPrincipal(), "tcp-port = 443\ndns = 1.1.1.1\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RequiresRestart {
		t.Error("expected RequiresRestart=false for DNS-only change")
	}
}

func TestConfigServiceApplyConfigPartialSuccessReloadFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	confPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(confPath, []byte("tcp-port = 443\n"), 0640); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	files := ocservconfig.NewConfigFileManager(confPath, 10)
	validator := ocservconfig.NewConfigValidator(confPath)

	runner := cmd.NewFakeCommandRunner()
	// Reload returns non-zero exit — simulates ocserv rejecting the config at runtime.
	runner.Results["occtl reload"] = cmd.CommandResult{
		Stderr:   "error in configuration line 2",
		ExitCode: 1,
	}
	reloader := ocservconfig.NewConfigReloader(runner, "occtl")
	inspector := ocserv.NewNetworkInspectorWithProcDir(runner, dir)

	svc := NewConfigManagementService(
		files,
		validator,
		reloader,
		inspector,
		ocservconfig.NewRestartDetector(),
		audit.NoopRecorder{},
		rbac.NewDefaultAuthorizer(),
	)

	newContent := "tcp-port = 8443\n"
	result, err := svc.ApplyConfig(context.Background(), ownerPrincipal(), newContent)
	if err != nil {
		t.Fatalf("expected no error for partial success, got %v", err)
	}
	if result.ReloadSuccess {
		t.Fatal("expected ReloadSuccess=false when occtl reload fails")
	}
	if result.ReloadError == "" {
		t.Fatal("expected ReloadError to contain stderr")
	}
	if result.BackupNumber != 1 {
		t.Errorf("expected BackupNumber=1, got %d", result.BackupNumber)
	}

	// Verify the file was actually written despite reload failure.
	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != newContent {
		t.Errorf("expected file to be written, got %q", string(data))
	}
}
