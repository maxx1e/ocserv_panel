package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"ocserv_panel/backend/internal/vpn/config"
)

func TestConfigFileManagerReadSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	content := "# ocserv config\nserver-cert = /etc/certs/server.pem\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0640); err != nil {
		t.Fatalf("write config: %v", err)
	}

	mgr := config.NewConfigFileManager(cfgPath, 5)
	got, meta, err := mgr.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != content {
		t.Errorf("Read content = %q, want %q", got, content)
	}
	if meta.Path != cfgPath {
		t.Errorf("meta.Path = %q, want %q", meta.Path, cfgPath)
	}
	if meta.Size != int64(len(content)) {
		t.Errorf("meta.Size = %d, want %d", meta.Size, len(content))
	}
	if meta.LastModified.IsZero() {
		t.Error("meta.LastModified is zero")
	}
}

func TestConfigFileManagerReadNotFound(t *testing.T) {
	t.Parallel()

	mgr := config.NewConfigFileManager("/nonexistent/path/ocserv.conf", 5)
	_, _, err := mgr.Read()
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestConfigFileManagerWriteCreatesBackup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	original := "# original config\n"
	if err := os.WriteFile(cfgPath, []byte(original), 0640); err != nil {
		t.Fatalf("write original: %v", err)
	}

	mgr := config.NewConfigFileManager(cfgPath, 5)
	newContent := "# updated config\n"
	if err := mgr.Write(newContent); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// The original should be in backup .1.
	backupPath := cfgPath + ".1"
	data, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup .1: %v", err)
	}
	if string(data) != original {
		t.Errorf("backup .1 = %q, want %q", string(data), original)
	}

	// The live file should hold new content.
	live, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read live: %v", err)
	}
	if string(live) != newContent {
		t.Errorf("live file = %q, want %q", string(live), newContent)
	}
}

func TestConfigFileManagerWriteRotatesBackups(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(cfgPath, []byte("v0\n"), 0640); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	mgr := config.NewConfigFileManager(cfgPath, 3)

	// Write 4 times: after 4 writes, only 3 backups should exist (.1, .2, .3).
	// .4 should not exist because maxBackups=3 causes deletion on rotation.
	versions := []string{"v1\n", "v2\n", "v3\n", "v4\n"}
	for _, v := range versions {
		if err := mgr.Write(v); err != nil {
			t.Fatalf("Write(%q): %v", v, err)
		}
	}

	// .4 must not exist.
	if _, err := os.Stat(cfgPath + ".4"); !os.IsNotExist(err) {
		t.Error("backup .4 should not exist after maxBackups=3 with 4 writes")
	}

	// .3 must exist.
	if _, err := os.Stat(cfgPath + ".3"); err != nil {
		t.Errorf("backup .3 should exist: %v", err)
	}
}

func TestConfigFileManagerWriteRejectsLargeContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(cfgPath, []byte("initial\n"), 0640); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	mgr := config.NewConfigFileManager(cfgPath, 5)
	// 65 KB exceeds the 64 KB limit.
	large := strings.Repeat("x", 65*1024)
	err := mgr.Write(large)
	if !errors.Is(err, config.ErrConfigTooLarge) {
		t.Errorf("expected ErrConfigTooLarge, got %v", err)
	}
}

func TestConfigFileManagerListBackups(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(cfgPath, []byte("v0\n"), 0640); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	mgr := config.NewConfigFileManager(cfgPath, 5)

	// Two writes produce two backups (.1 and .2).
	if err := mgr.Write("v1\n"); err != nil {
		t.Fatalf("Write v1: %v", err)
	}
	if err := mgr.Write("v2\n"); err != nil {
		t.Fatalf("Write v2: %v", err)
	}

	backups, err := mgr.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("expected 2 backups, got %d: %v", len(backups), backups)
	}

	// Verify both backup numbers are present.
	nums := map[int]bool{}
	for _, b := range backups {
		nums[b.Number] = true
		if b.Size == 0 {
			t.Errorf("backup %d has zero size", b.Number)
		}
	}
	if !nums[1] || !nums[2] {
		t.Errorf("expected backup numbers 1 and 2, got %v", nums)
	}
}

func TestConfigFileManagerGetBackup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	original := "# original\n"
	if err := os.WriteFile(cfgPath, []byte(original), 0640); err != nil {
		t.Fatalf("write original: %v", err)
	}

	mgr := config.NewConfigFileManager(cfgPath, 5)
	if err := mgr.Write("# new\n"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := mgr.GetBackup(1)
	if err != nil {
		t.Fatalf("GetBackup(1): %v", err)
	}
	if got != original {
		t.Errorf("GetBackup(1) = %q, want %q", got, original)
	}
}

func TestConfigFileManagerGetBackupNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(cfgPath, []byte("content\n"), 0640); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	mgr := config.NewConfigFileManager(cfgPath, 5)
	_, err := mgr.GetBackup(99)
	if !errors.Is(err, config.ErrBackupNotFound) {
		t.Errorf("expected ErrBackupNotFound, got %v", err)
	}
}

func TestConfigFileManagerRollbackSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	original := "# original config\n"
	if err := os.WriteFile(cfgPath, []byte(original), 0640); err != nil {
		t.Fatalf("write original: %v", err)
	}

	mgr := config.NewConfigFileManager(cfgPath, 5)
	updated := "# updated config\n"
	if err := mgr.Write(updated); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Verify current state before rollback.
	live, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read live before rollback: %v", err)
	}
	if string(live) != updated {
		t.Fatalf("pre-rollback live = %q, want %q", string(live), updated)
	}

	// Rollback to backup 1 (original content).
	if err := mgr.Rollback(1); err != nil {
		t.Fatalf("Rollback(1): %v", err)
	}

	// Live file should now contain original.
	live, err = os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read live after rollback: %v", err)
	}
	if string(live) != original {
		t.Errorf("post-rollback live = %q, want %q", string(live), original)
	}
}

func TestConfigFileManagerRollbackNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(cfgPath, []byte("content\n"), 0640); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	mgr := config.NewConfigFileManager(cfgPath, 5)
	err := mgr.Rollback(99)
	if !errors.Is(err, config.ErrBackupNotFound) {
		t.Errorf("expected ErrBackupNotFound, got %v", err)
	}
}

func TestConfigFileManagerDiffWithContent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	original := "line-one\nline-two\nline-three\n"
	if err := os.WriteFile(cfgPath, []byte(original), 0640); err != nil {
		t.Fatalf("write original: %v", err)
	}

	mgr := config.NewConfigFileManager(cfgPath, 5)
	proposed := "line-one\nline-two-modified\nline-three\n"
	diff, err := mgr.DiffWithContent(proposed)
	if err != nil {
		t.Fatalf("DiffWithContent: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff for changed content")
	}
	if !strings.Contains(diff, "-line-two") {
		t.Errorf("diff missing removed line, got:\n%s", diff)
	}
	if !strings.Contains(diff, "+line-two-modified") {
		t.Errorf("diff missing added line, got:\n%s", diff)
	}
}

func TestConfigFileManagerDiffWithContentNoDiff(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	content := "# identical content\nkey = value\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0640); err != nil {
		t.Fatalf("write content: %v", err)
	}

	mgr := config.NewConfigFileManager(cfgPath, 5)
	diff, err := mgr.DiffWithContent(content)
	if err != nil {
		t.Fatalf("DiffWithContent: %v", err)
	}
	if diff != "" {
		t.Errorf("expected empty diff for identical content, got:\n%s", diff)
	}
}

func TestConfigFileManagerDiffWithBackup(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	original := "# original line\nkey = old-value\n"
	if err := os.WriteFile(cfgPath, []byte(original), 0640); err != nil {
		t.Fatalf("write original: %v", err)
	}

	mgr := config.NewConfigFileManager(cfgPath, 5)
	// Write new content; original becomes backup .1.
	newContent := "# original line\nkey = new-value\n"
	if err := mgr.Write(newContent); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Diff backup 1 (old) vs current (new).
	diff, err := mgr.DiffWithBackup(1)
	if err != nil {
		t.Fatalf("DiffWithBackup(1): %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff between backup and current")
	}
	if !strings.Contains(diff, "-key = old-value") {
		t.Errorf("diff missing old value, got:\n%s", diff)
	}
	if !strings.Contains(diff, "+key = new-value") {
		t.Errorf("diff missing new value, got:\n%s", diff)
	}
}

func TestConfigFileManagerWriteLockContention(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(cfgPath, []byte("initial\n"), 0640); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	// Acquire the lock externally to simulate contention.
	lockPath := cfgPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0640)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("acquire test lock: %v", err)
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	}()

	// Now ConfigFileManager.Write should fail with ErrConfigLocked.
	mgr := config.NewConfigFileManager(cfgPath, 5)
	err = mgr.Write("new content\n")
	if !errors.Is(err, config.ErrConfigLocked) {
		t.Fatalf("expected ErrConfigLocked, got %v", err)
	}

	// Verify the file was not modified.
	data, _ := os.ReadFile(cfgPath)
	if string(data) != "initial\n" {
		t.Errorf("file should not have changed, got %q", string(data))
	}
}

func TestConfigFileManagerRollbackLockContention(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(cfgPath, []byte("original\n"), 0640); err != nil {
		t.Fatalf("write original: %v", err)
	}

	mgr := config.NewConfigFileManager(cfgPath, 5)
	// Create a backup first so rollback has something to target.
	if err := mgr.Write("updated\n"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Now hold the lock externally.
	lockPath := cfgPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0640)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("acquire test lock: %v", err)
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	}()

	// Rollback should fail with ErrConfigLocked.
	err = mgr.Rollback(1)
	if !errors.Is(err, config.ErrConfigLocked) {
		t.Fatalf("expected ErrConfigLocked, got %v", err)
	}
}
