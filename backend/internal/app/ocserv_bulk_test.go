package app

import (
	"context"
	"errors"
	"testing"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/ocserv"
)

// --- helper: build an OcservService wired with a custom FakePasswdManager ---

func newBulkTestService(t *testing.T, passwd *ocserv.FakePasswdManager) *OcservService {
	t.Helper()
	svc, _, _, _ := newTestOcservService(t, ocserv.ServiceActive)
	// Replace the fake passwd manager with the one supplied by the test.
	svc.users = passwd
	return svc
}

// --- BulkDeleteOcservUsers ---

func TestOcservServiceBulkDeleteAllSucceed(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()
	_ = passwd.CreateUser(ctx, "alice", "pass")
	_ = passwd.CreateUser(ctx, "bob", "pass")

	svc := newBulkTestService(t, passwd)
	result := svc.BulkDeleteOcservUsers(ctx, ownerPrincipal(), []string{"alice", "bob"})

	if result.Total != 2 {
		t.Errorf("expected Total=2, got %d", result.Total)
	}
	if result.Succeeded != 2 {
		t.Errorf("expected Succeeded=2, got %d", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("expected Failed=0, got %d", result.Failed)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}

	// Verify users were actually removed.
	for _, username := range []string{"alice", "bob"} {
		exists, _ := passwd.UserExists(ctx, username)
		if exists {
			t.Errorf("expected %s to be deleted, but still exists", username)
		}
	}
}

func TestOcservServiceBulkDeletePartialSuccess(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()
	// Only "alice" exists; "ghost" does not — but Delete on FakePasswdManager is
	// idempotent (no error for missing users).  To force a partial failure we use
	// an invalid username that fails validation.
	_ = passwd.CreateUser(ctx, "alice", "pass")

	svc := newBulkTestService(t, passwd)
	// "!invalid!" fails validation.Username.
	result := svc.BulkDeleteOcservUsers(ctx, ownerPrincipal(), []string{"alice", "!invalid!"})

	if result.Total != 2 {
		t.Errorf("expected Total=2, got %d", result.Total)
	}
	if result.Succeeded != 1 {
		t.Errorf("expected Succeeded=1, got %d", result.Succeeded)
	}
	if result.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", result.Failed)
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error entry, got %v", result.Errors)
	}
	if result.Errors[0].ID != "!invalid!" {
		t.Errorf("expected error for !invalid!, got %q", result.Errors[0].ID)
	}
}

func TestOcservServiceBulkDeleteDeniedForViewer(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()
	_ = passwd.CreateUser(ctx, "alice", "pass")
	_ = passwd.CreateUser(ctx, "bob", "pass")

	svc := newBulkTestService(t, passwd)
	viewer := auth.Principal{ID: 2, Username: "viewer", Roles: []string{"viewer"}}
	result := svc.BulkDeleteOcservUsers(ctx, viewer, []string{"alice", "bob"})

	if result.Total != 2 {
		t.Errorf("expected Total=2, got %d", result.Total)
	}
	if result.Succeeded != 0 {
		t.Errorf("expected Succeeded=0, got %d", result.Succeeded)
	}
	if result.Failed != 2 {
		t.Errorf("expected Failed=2 (all denied), got %d", result.Failed)
	}
	for i, e := range result.Errors {
		if e.Error != "permission denied" {
			t.Errorf("result.Errors[%d]: expected %q, got %q", i, "permission denied", e.Error)
		}
	}

	// Users must still exist.
	for _, username := range []string{"alice", "bob"} {
		exists, _ := passwd.UserExists(ctx, username)
		if !exists {
			t.Errorf("expected %s to still exist after denied bulk delete", username)
		}
	}
}

func TestOcservServiceBulkDeleteDeniedForUnauthenticated(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()
	_ = passwd.CreateUser(ctx, "alice", "pass")

	svc := newBulkTestService(t, passwd)
	result := svc.BulkDeleteOcservUsers(ctx, auth.Principal{}, []string{"alice"})

	if result.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", result.Failed)
	}
	if result.Succeeded != 0 {
		t.Errorf("expected Succeeded=0, got %d", result.Succeeded)
	}
}

func TestOcservServiceBulkDeletePasswdManagerError(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()
	_ = passwd.CreateUser(ctx, "alice", "pass")
	passwd.SetError(errors.New("filesystem unavailable"))

	svc := newBulkTestService(t, passwd)
	result := svc.BulkDeleteOcservUsers(ctx, ownerPrincipal(), []string{"alice"})

	if result.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", result.Failed)
	}
	if result.Succeeded != 0 {
		t.Errorf("expected Succeeded=0, got %d", result.Succeeded)
	}
	if len(result.Errors) != 1 || result.Errors[0].Error == "" {
		t.Errorf("expected 1 populated error, got %v", result.Errors)
	}
}

// --- BulkLockOcservUsers ---

func TestOcservServiceBulkLockAllSucceed(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()
	_ = passwd.CreateUser(ctx, "alice", "pass")
	_ = passwd.CreateUser(ctx, "bob", "pass")

	svc := newBulkTestService(t, passwd)
	result := svc.BulkLockOcservUsers(ctx, ownerPrincipal(), []string{"alice", "bob"})

	if result.Succeeded != 2 {
		t.Errorf("expected Succeeded=2, got %d", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("expected Failed=0, got %d", result.Failed)
	}

	// Confirm both users are inactive.
	users, _ := passwd.ListUsers(ctx)
	for _, u := range users {
		if u.Active {
			t.Errorf("expected %s to be locked (inactive), but Active=true", u.Username)
		}
	}
}

func TestOcservServiceBulkLockNonExistentUserReturnsError(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()
	// FakePasswdManager.LockUser returns an error for unknown users.

	svc := newBulkTestService(t, passwd)
	result := svc.BulkLockOcservUsers(ctx, ownerPrincipal(), []string{"ghost"})

	if result.Total != 1 {
		t.Errorf("expected Total=1, got %d", result.Total)
	}
	if result.Failed != 1 {
		t.Errorf("expected Failed=1 (user not found), got %d", result.Failed)
	}
	if result.Succeeded != 0 {
		t.Errorf("expected Succeeded=0, got %d", result.Succeeded)
	}
}

func TestOcservServiceBulkLockDeniedForViewer(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()
	_ = passwd.CreateUser(ctx, "alice", "pass")

	svc := newBulkTestService(t, passwd)
	viewer := auth.Principal{ID: 2, Username: "viewer", Roles: []string{"viewer"}}
	result := svc.BulkLockOcservUsers(ctx, viewer, []string{"alice"})

	if result.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", result.Failed)
	}
	if result.Succeeded != 0 {
		t.Errorf("expected Succeeded=0, got %d", result.Succeeded)
	}
	for i, e := range result.Errors {
		if e.Error != "permission denied" {
			t.Errorf("result.Errors[%d]: expected %q, got %q", i, "permission denied", e.Error)
		}
	}
}

func TestOcservServiceBulkLockInvalidUsernameReturnsError(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()
	_ = passwd.CreateUser(ctx, "alice", "pass")

	svc := newBulkTestService(t, passwd)
	// Mix of valid and invalid username.
	result := svc.BulkLockOcservUsers(ctx, ownerPrincipal(), []string{"alice", "!bad!"})

	if result.Succeeded != 1 {
		t.Errorf("expected Succeeded=1, got %d", result.Succeeded)
	}
	if result.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", result.Failed)
	}
}

// --- BulkUnlockOcservUsers ---

func TestOcservServiceBulkUnlockAllSucceed(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()
	_ = passwd.CreateUser(ctx, "alice", "pass")
	_ = passwd.CreateUser(ctx, "bob", "pass")
	// Lock them first so unlock has something to do.
	_ = passwd.LockUser(ctx, "alice")
	_ = passwd.LockUser(ctx, "bob")

	svc := newBulkTestService(t, passwd)
	result := svc.BulkUnlockOcservUsers(ctx, ownerPrincipal(), []string{"alice", "bob"})

	if result.Succeeded != 2 {
		t.Errorf("expected Succeeded=2, got %d", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("expected Failed=0, got %d", result.Failed)
	}

	// Confirm both users are now active.
	users, _ := passwd.ListUsers(ctx)
	for _, u := range users {
		if !u.Active {
			t.Errorf("expected %s to be unlocked (active), but Active=false", u.Username)
		}
	}
}

func TestOcservServiceBulkUnlockNonExistentUserReturnsError(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()

	svc := newBulkTestService(t, passwd)
	result := svc.BulkUnlockOcservUsers(ctx, ownerPrincipal(), []string{"ghost"})

	if result.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", result.Failed)
	}
	if result.Succeeded != 0 {
		t.Errorf("expected Succeeded=0, got %d", result.Succeeded)
	}
}

func TestOcservServiceBulkUnlockDeniedForViewer(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()
	_ = passwd.CreateUser(ctx, "alice", "pass")
	_ = passwd.LockUser(ctx, "alice")

	svc := newBulkTestService(t, passwd)
	viewer := auth.Principal{ID: 2, Username: "viewer", Roles: []string{"viewer"}}
	result := svc.BulkUnlockOcservUsers(ctx, viewer, []string{"alice"})

	if result.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", result.Failed)
	}
	if result.Succeeded != 0 {
		t.Errorf("expected Succeeded=0, got %d", result.Succeeded)
	}
	for i, e := range result.Errors {
		if e.Error != "permission denied" {
			t.Errorf("result.Errors[%d]: expected %q, got %q", i, "permission denied", e.Error)
		}
	}
}

func TestOcservServiceBulkUnlockDeniedForUnauthenticated(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()
	_ = passwd.CreateUser(ctx, "alice", "pass")
	_ = passwd.LockUser(ctx, "alice")

	svc := newBulkTestService(t, passwd)
	result := svc.BulkUnlockOcservUsers(ctx, auth.Principal{}, []string{"alice"})

	if result.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", result.Failed)
	}
	if result.Succeeded != 0 {
		t.Errorf("expected Succeeded=0, got %d", result.Succeeded)
	}
}

func TestOcservServiceBulkUnlockInvalidUsernameReturnsError(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	ctx := context.Background()
	_ = passwd.CreateUser(ctx, "alice", "pass")
	_ = passwd.LockUser(ctx, "alice")

	svc := newBulkTestService(t, passwd)
	result := svc.BulkUnlockOcservUsers(ctx, ownerPrincipal(), []string{"alice", "!bad!"})

	if result.Succeeded != 1 {
		t.Errorf("expected Succeeded=1, got %d", result.Succeeded)
	}
	if result.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", result.Failed)
	}
}

// --- Empty input edge cases ---

func TestOcservServiceBulkDeleteEmptyList(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	svc := newBulkTestService(t, passwd)
	result := svc.BulkDeleteOcservUsers(context.Background(), ownerPrincipal(), []string{})

	if result.Total != 0 {
		t.Errorf("expected Total=0 for empty input, got %d", result.Total)
	}
	if result.Succeeded != 0 {
		t.Errorf("expected Succeeded=0, got %d", result.Succeeded)
	}
	if result.Failed != 0 {
		t.Errorf("expected Failed=0, got %d", result.Failed)
	}
}

func TestOcservServiceBulkLockEmptyList(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	svc := newBulkTestService(t, passwd)
	result := svc.BulkLockOcservUsers(context.Background(), ownerPrincipal(), []string{})

	if result.Total != 0 {
		t.Errorf("expected Total=0 for empty input, got %d", result.Total)
	}
}

func TestOcservServiceBulkUnlockEmptyList(t *testing.T) {
	t.Parallel()
	passwd := ocserv.NewFakePasswdManager()
	svc := newBulkTestService(t, passwd)
	result := svc.BulkUnlockOcservUsers(context.Background(), ownerPrincipal(), []string{})

	if result.Total != 0 {
		t.Errorf("expected Total=0 for empty input, got %d", result.Total)
	}
}
