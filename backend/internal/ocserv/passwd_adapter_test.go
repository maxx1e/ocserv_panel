package ocserv

import (
	"context"
	"testing"
)

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
