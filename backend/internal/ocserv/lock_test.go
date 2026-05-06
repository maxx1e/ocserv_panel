package ocserv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLockUser_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	passwdFile := filepath.Join(tmpDir, "ocpasswd")
	content := "user1:group1:$hash123\n"
	if err := os.WriteFile(passwdFile, []byte(content), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	adapter := NewOcpasswdAdapter("/usr/bin/ocpasswd", passwdFile, "/etc/ocserv/ocserv.conf", nil)

	err := adapter.LockUser(context.Background(), "user1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	data, err := os.ReadFile(passwdFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	expected := "user1:group1:!"
	got := strings.TrimSpace(string(data))
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLockUser_AlreadyLocked(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	passwdFile := filepath.Join(tmpDir, "ocpasswd")
	content := "user1:group1:!\n"
	if err := os.WriteFile(passwdFile, []byte(content), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	adapter := NewOcpasswdAdapter("/usr/bin/ocpasswd", passwdFile, "/etc/ocserv/ocserv.conf", nil)

	err := adapter.LockUser(context.Background(), "user1")
	if err != nil {
		t.Fatalf("expected no error (idempotent), got: %v", err)
	}

	data, err := os.ReadFile(passwdFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	expected := "user1:group1:!"
	got := strings.TrimSpace(string(data))
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLockUser_NotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	passwdFile := filepath.Join(tmpDir, "ocpasswd")
	content := "other:group:hash\n"
	if err := os.WriteFile(passwdFile, []byte(content), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	adapter := NewOcpasswdAdapter("/usr/bin/ocpasswd", passwdFile, "/etc/ocserv/ocserv.conf", nil)

	err := adapter.LockUser(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for non-existent user, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected error to contain 'not found', got: %v", err)
	}
}

func TestUnlockUser_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	passwdFile := filepath.Join(tmpDir, "ocpasswd")
	content := "user1:group1:!\n"
	if err := os.WriteFile(passwdFile, []byte(content), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	adapter := NewOcpasswdAdapter("/usr/bin/ocpasswd", passwdFile, "/etc/ocserv/ocserv.conf", nil)

	err := adapter.UnlockUser(context.Background(), "user1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	data, err := os.ReadFile(passwdFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	expected := "user1:group1:*"
	got := strings.TrimSpace(string(data))
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestUnlockUser_AlreadyUnlocked(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	passwdFile := filepath.Join(tmpDir, "ocpasswd")
	content := "user1:group1:$hash123\n"
	if err := os.WriteFile(passwdFile, []byte(content), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	adapter := NewOcpasswdAdapter("/usr/bin/ocpasswd", passwdFile, "/etc/ocserv/ocserv.conf", nil)

	err := adapter.UnlockUser(context.Background(), "user1")
	if err != nil {
		t.Fatalf("expected no error (no-op when not locked), got: %v", err)
	}

	data, err := os.ReadFile(passwdFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	// File should be unchanged — hash stays as "$hash123"
	expected := "user1:group1:$hash123"
	got := strings.TrimSpace(string(data))
	if got != expected {
		t.Fatalf("expected %q (unchanged), got %q", expected, got)
	}
}

func TestUnlockUser_NotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	passwdFile := filepath.Join(tmpDir, "ocpasswd")
	content := "other:group:!\n"
	if err := os.WriteFile(passwdFile, []byte(content), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	adapter := NewOcpasswdAdapter("/usr/bin/ocpasswd", passwdFile, "/etc/ocserv/ocserv.conf", nil)

	err := adapter.UnlockUser(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error for non-existent user, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected error to contain 'not found', got: %v", err)
	}
}

func TestLockUser_PreservesOtherUsers(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	passwdFile := filepath.Join(tmpDir, "ocpasswd")
	content := "alice:default:$hash_alice\nbob:vpn:$hash_bob\ncharlie:default:$hash_charlie\n"
	if err := os.WriteFile(passwdFile, []byte(content), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	adapter := NewOcpasswdAdapter("/usr/bin/ocpasswd", passwdFile, "/etc/ocserv/ocserv.conf", nil)

	err := adapter.LockUser(context.Background(), "bob")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	data, err := os.ReadFile(passwdFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}

	// alice unchanged
	if lines[0] != "alice:default:$hash_alice" {
		t.Fatalf("alice line changed: got %q", lines[0])
	}

	// bob locked
	if lines[1] != "bob:vpn:!" {
		t.Fatalf("expected bob to be locked, got %q", lines[1])
	}

	// charlie unchanged
	if lines[2] != "charlie:default:$hash_charlie" {
		t.Fatalf("charlie line changed: got %q", lines[2])
	}
}

func TestParsePasswdFile_LockedUser(t *testing.T) {
	t.Parallel()

	data := []byte("user1:group1:!\nuser2:group2:$hash_active\n")
	users := ParsePasswdFile(data)

	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	// Locked user: Active should be false
	if users[0].Username != "user1" {
		t.Fatalf("expected user1, got %s", users[0].Username)
	}
	if users[0].Active {
		t.Fatal("expected user1 (hash=!) to be inactive, got Active=true")
	}

	// Active user
	if users[1].Username != "user2" {
		t.Fatalf("expected user2, got %s", users[1].Username)
	}
	if !users[1].Active {
		t.Fatal("expected user2 to be active, got Active=false")
	}
}
