package ocserv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// OcpasswdAdapter tests using FakeCommandRunner

func TestOcpasswdAdapter_CreateUser_Success(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/ocpasswd"] = CommandResult{ExitCode: 0}

	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/etc/ocserv/ocpasswd", "/etc/ocserv/ocserv.conf", runner)

	err := adapter.CreateUser(context.Background(), "alice", "secret123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.Calls))
	}

	call := runner.Calls[0]
	if call.Name != "/usr/local/bin/ocpasswd" {
		t.Fatalf("expected binary /usr/local/bin/ocpasswd, got %s", call.Name)
	}

	expectedArgs := []string{"-c", "/etc/ocserv/ocserv.conf", "alice"}
	if len(call.Args) != len(expectedArgs) {
		t.Fatalf("expected args %v, got %v", expectedArgs, call.Args)
	}
	for i, arg := range expectedArgs {
		if call.Args[i] != arg {
			t.Fatalf("arg[%d]: expected %q, got %q", i, arg, call.Args[i])
		}
	}
}

func TestOcpasswdAdapter_CreateUser_Failure(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/ocpasswd"] = CommandResult{
		ExitCode: 1,
		Stderr:   "error: unable to write password file\n",
	}

	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/etc/ocserv/ocpasswd", "/etc/ocserv/ocserv.conf", runner)

	err := adapter.CreateUser(context.Background(), "alice", "secret123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "unable to write password file") {
		t.Fatalf("expected error to contain stderr message, got: %v", err)
	}
}

func TestOcpasswdAdapter_DeleteUser_Success(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/ocpasswd"] = CommandResult{ExitCode: 0}

	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/etc/ocserv/ocpasswd", "/etc/ocserv/ocserv.conf", runner)

	err := adapter.DeleteUser(context.Background(), "bob")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.Calls))
	}

	call := runner.Calls[0]
	expectedArgs := []string{"-c", "/etc/ocserv/ocserv.conf", "-d", "bob"}
	if len(call.Args) != len(expectedArgs) {
		t.Fatalf("expected args %v, got %v", expectedArgs, call.Args)
	}
	for i, arg := range expectedArgs {
		if call.Args[i] != arg {
			t.Fatalf("arg[%d]: expected %q, got %q", i, arg, call.Args[i])
		}
	}

	// Verify -d flag is present
	foundD := false
	for _, arg := range call.Args {
		if arg == "-d" {
			foundD = true
			break
		}
	}
	if !foundD {
		t.Fatal("expected -d flag in delete command args")
	}
}

func TestOcpasswdAdapter_DeleteUser_Failure(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/ocpasswd"] = CommandResult{
		ExitCode: 1,
		Stderr:   "error: user does not exist\n",
	}

	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/etc/ocserv/ocpasswd", "/etc/ocserv/ocserv.conf", runner)

	err := adapter.DeleteUser(context.Background(), "bob")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "user does not exist") {
		t.Fatalf("expected error to contain stderr message, got: %v", err)
	}
}

func TestOcpasswdAdapter_UpdatePassword_Success(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/ocpasswd"] = CommandResult{ExitCode: 0}

	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/etc/ocserv/ocpasswd", "/etc/ocserv/ocserv.conf", runner)

	err := adapter.UpdatePassword(context.Background(), "alice", "newpass456")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(runner.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(runner.Calls))
	}

	call := runner.Calls[0]
	if call.Name != "/usr/local/bin/ocpasswd" {
		t.Fatalf("expected binary /usr/local/bin/ocpasswd, got %s", call.Name)
	}

	// UpdatePassword uses the same command structure as CreateUser
	expectedArgs := []string{"-c", "/etc/ocserv/ocserv.conf", "alice"}
	if len(call.Args) != len(expectedArgs) {
		t.Fatalf("expected args %v, got %v", expectedArgs, call.Args)
	}
	for i, arg := range expectedArgs {
		if call.Args[i] != arg {
			t.Fatalf("arg[%d]: expected %q, got %q", i, arg, call.Args[i])
		}
	}
}

func TestOcpasswdAdapter_ListUsers_FileNotExist(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	// Use a non-existent file path — ListUsers reads the file directly
	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/tmp/nonexistent_ocpasswd_file_test_12345", "/etc/ocserv/ocserv.conf", runner)

	users, err := adapter.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected empty slice, got %d users", len(users))
	}
}

func TestOcpasswdAdapter_ListUsers_ParsesFile(t *testing.T) {
	t.Parallel()

	// Create a temp file with ocpasswd format
	tmpDir := t.TempDir()
	passwdFile := filepath.Join(tmpDir, "ocpasswd")
	content := "user1:*:hash1\nuser2:group:hash2\n"
	if err := os.WriteFile(passwdFile, []byte(content), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	runner := NewFakeCommandRunner()
	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", passwdFile, "/etc/ocserv/ocserv.conf", runner)

	users, err := adapter.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	if users[0].Username != "user1" {
		t.Fatalf("expected user1, got %s", users[0].Username)
	}
	if users[0].Group != "*" {
		t.Fatalf("expected group *, got %s", users[0].Group)
	}
	if !users[0].Active {
		t.Fatal("expected user1 to be active")
	}

	if users[1].Username != "user2" {
		t.Fatalf("expected user2, got %s", users[1].Username)
	}
	if users[1].Group != "group" {
		t.Fatalf("expected group 'group', got %s", users[1].Group)
	}
	if !users[1].Active {
		t.Fatal("expected user2 to be active")
	}
}

func TestOcpasswdAdapter_UserExists_Found(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	passwdFile := filepath.Join(tmpDir, "ocpasswd")
	content := "alice:default:hashed\nbob:vpn:hashed2\n"
	if err := os.WriteFile(passwdFile, []byte(content), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	runner := NewFakeCommandRunner()
	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", passwdFile, "/etc/ocserv/ocserv.conf", runner)

	exists, err := adapter.UserExists(context.Background(), "alice")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !exists {
		t.Fatal("expected alice to exist")
	}
}

func TestOcpasswdAdapter_UserExists_NotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	passwdFile := filepath.Join(tmpDir, "ocpasswd")
	content := "alice:default:hashed\nbob:vpn:hashed2\n"
	if err := os.WriteFile(passwdFile, []byte(content), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	runner := NewFakeCommandRunner()
	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", passwdFile, "/etc/ocserv/ocserv.conf", runner)

	exists, err := adapter.UserExists(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if exists {
		t.Fatal("expected nonexistent user to not be found")
	}
}

// ParsePasswdFile tests

func TestParsePasswdFileEmpty(t *testing.T) {
	t.Parallel()
	users := ParsePasswdFile([]byte(""))
	if len(users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(users))
	}
}

func TestParsePasswdFileWithTwoUsers(t *testing.T) {
	t.Parallel()
	data := []byte("alice:default:*hashed*\nbob:vpn:*hashed2*\n")
	users := ParsePasswdFile(data)
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].Username != "alice" {
		t.Fatalf("expected alice, got %s", users[0].Username)
	}
	if users[0].Group != "default" {
		t.Fatalf("expected group=default, got %s", users[0].Group)
	}
	if !users[0].Active {
		t.Fatal("expected alice to be active")
	}
}

func TestParsePasswdFileWithDisabledUser(t *testing.T) {
	t.Parallel()
	data := []byte("alice:default:!\n")
	users := ParsePasswdFile(data)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Active {
		t.Fatal("expected disabled user (!) to be inactive")
	}
}

func TestParsePasswdFileSkipsComments(t *testing.T) {
	t.Parallel()
	data := []byte("# comment\nalice:default:*hashed*\n")
	users := ParsePasswdFile(data)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Username != "alice" {
		t.Fatalf("expected alice, got %s", users[0].Username)
	}
}

func TestParsePasswdFileSkipsBlankLines(t *testing.T) {
	t.Parallel()
	data := []byte("alice:default:*hashed*\n\nbob:default:*hashed2*\n")
	users := ParsePasswdFile(data)
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestParsePasswdFileHandlesMissingHashField(t *testing.T) {
	t.Parallel()
	// Only two fields — valid format without hash.
	data := []byte("alice:default\n")
	users := ParsePasswdFile(data)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if !users[0].Active {
		t.Fatal("expected active when hash field is absent")
	}
}

// FakePasswdManager tests

func TestFakePasswdManagerCreateAndList(t *testing.T) {
	t.Parallel()
	mgr := NewFakePasswdManager()
	if err := mgr.CreateUser(context.Background(), "alice", "password123"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	users, err := mgr.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 1 || users[0].Username != "alice" {
		t.Fatalf("expected alice, got %v", users)
	}
}

func TestFakePasswdManagerDelete(t *testing.T) {
	t.Parallel()
	mgr := NewFakePasswdManager()
	if err := mgr.CreateUser(context.Background(), "alice", "password123"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := mgr.DeleteUser(context.Background(), "alice"); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	exists, err := mgr.UserExists(context.Background(), "alice")
	if err != nil {
		t.Fatalf("user exists: %v", err)
	}
	if exists {
		t.Fatal("expected alice to be deleted")
	}
}

func TestFakePasswdManagerUserExists(t *testing.T) {
	t.Parallel()
	mgr := NewFakePasswdManager()
	if err := mgr.CreateUser(context.Background(), "alice", "password123"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	exists, err := mgr.UserExists(context.Background(), "alice")
	if err != nil {
		t.Fatalf("user exists: %v", err)
	}
	if !exists {
		t.Fatal("expected alice to exist")
	}

	missing, err := mgr.UserExists(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("user exists for nonexistent: %v", err)
	}
	if missing {
		t.Fatal("expected nonexistent user to not exist")
	}
}

func TestFakePasswdManagerSetError(t *testing.T) {
	t.Parallel()
	mgr := NewFakePasswdManager()
	mgr.SetError(errTest)

	if err := mgr.CreateUser(context.Background(), "alice", "password"); err == nil {
		t.Fatal("expected error from CreateUser")
	}
	if _, err := mgr.ListUsers(context.Background()); err == nil {
		t.Fatal("expected error from ListUsers")
	}
	if _, err := mgr.UserExists(context.Background(), "alice"); err == nil {
		t.Fatal("expected error from UserExists")
	}
}

// errTest is a sentinel error used in FakePasswdManager tests.
var errTest = errSentinel("test error")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

// --- Password file missing tests ---

func TestOcpasswdAdapter_DeleteUser_PasswordFileMissing(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/nonexistent/path/ocpasswd", "/etc/ocserv/ocserv.conf", runner)

	err := adapter.DeleteUser(context.Background(), "alice")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "password file") {
		t.Fatalf("expected error mentioning password file, got: %v", err)
	}
	// Verify the sentinel error is in the chain.
	if !errors.Is(err, ErrPasswordFileMissing) {
		t.Fatalf("expected ErrPasswordFileMissing in error chain, got: %v", err)
	}
}

func TestOcpasswdAdapter_LockUser_PasswordFileMissing(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/nonexistent/path/ocpasswd", "/etc/ocserv/ocserv.conf", runner)

	err := adapter.LockUser(context.Background(), "alice")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrPasswordFileMissing) {
		t.Fatalf("expected ErrPasswordFileMissing in error chain, got: %v", err)
	}
}

func TestOcpasswdAdapter_UnlockUser_PasswordFileMissing(t *testing.T) {
	t.Parallel()

	runner := NewFakeCommandRunner()
	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/nonexistent/path/ocpasswd", "/etc/ocserv/ocserv.conf", runner)

	err := adapter.UnlockUser(context.Background(), "alice")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrPasswordFileMissing) {
		t.Fatalf("expected ErrPasswordFileMissing in error chain, got: %v", err)
	}
}

func TestOcpasswdAdapter_CreateUser_CreatesDirIfMissing(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Point to a subdirectory that doesn't exist yet
	nestedDir := filepath.Join(tmpDir, "sub", "dir")
	passwdFile := filepath.Join(nestedDir, "ocpasswd")

	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/ocpasswd"] = CommandResult{ExitCode: 0}

	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", passwdFile, "/etc/ocserv/ocserv.conf", runner)

	err := adapter.CreateUser(context.Background(), "alice", "secret123")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Directory should have been created
	info, err := os.Stat(nestedDir)
	if err != nil {
		t.Fatalf("expected directory to be created, stat error: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory, got file")
	}
}
