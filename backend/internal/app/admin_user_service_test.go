package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

// mockAdminUserStore is a configurable test double for AdminUserStore.
type mockAdminUserStore struct {
	admins      []store.Admin
	ownerCount  int64
	createErr   error
	updateErr   error
	deleteErr   error
	disableErr  error
	enableErr   error
	updatePwErr error
}

func (m *mockAdminUserStore) ListAdmins(_ context.Context) ([]store.Admin, error) {
	return m.admins, nil
}

func (m *mockAdminUserStore) FindAdminByID(_ context.Context, id int64) (store.Admin, bool, error) {
	for _, a := range m.admins {
		if a.ID == id {
			return a, true, nil
		}
	}
	return store.Admin{}, false, nil
}

func (m *mockAdminUserStore) FindAdminByUsername(_ context.Context, username string) (store.Admin, bool, error) {
	for _, a := range m.admins {
		if a.Username == username {
			return a, true, nil
		}
	}
	return store.Admin{}, false, nil
}

func (m *mockAdminUserStore) CreateAdmin(_ context.Context, username, passwordHash, displayName, email string, role rbac.Role) (store.Admin, error) {
	if m.createErr != nil {
		return store.Admin{}, m.createErr
	}
	now := time.Now().UTC()
	return store.Admin{
		ID:           100,
		Username:     username,
		PasswordHash: passwordHash,
		DisplayName:  displayName,
		Email:        email,
		Roles:        []rbac.Role{role},
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (m *mockAdminUserStore) UpdateAdmin(_ context.Context, _ int64, _, _ string, _ *rbac.Role) error {
	return m.updateErr
}

func (m *mockAdminUserStore) DeleteAdmin(_ context.Context, _ int64) error {
	return m.deleteErr
}

func (m *mockAdminUserStore) DisableAdmin(_ context.Context, _ int64) error {
	return m.disableErr
}

func (m *mockAdminUserStore) EnableAdmin(_ context.Context, _ int64) error {
	return m.enableErr
}

func (m *mockAdminUserStore) UpdateAdminPassword(_ context.Context, _ int64, _ string) error {
	return m.updatePwErr
}

func (m *mockAdminUserStore) CountOwners(_ context.Context) (int64, error) {
	return m.ownerCount, nil
}

func adminOwnerPrincipal(id int64) auth.Principal {
	return auth.Principal{
		ID:       id,
		Username: "admin",
		Roles:    []string{string(rbac.RoleOwner)},
	}
}

func adminViewerPrincipal(id int64) auth.Principal {
	return auth.Principal{
		ID:       id,
		Username: "viewer",
		Roles:    []string{string(rbac.RoleViewer)},
	}
}

func fakeHash(password string) (string, error) {
	return "hashed:" + password, nil
}

func newTestAdminUserService(s AdminUserStore) *AdminUserService {
	return NewAdminUserService(s, audit.NoopRecorder{}, rbac.NewDefaultAuthorizer(), fakeHash)
}

func TestAdminUserService_List_OwnerAllowed(t *testing.T) {
	now := time.Now().UTC()
	mockStore := &mockAdminUserStore{
		admins: []store.Admin{
			{ID: 1, Username: "alice", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
			{ID: 2, Username: "bob", Roles: []rbac.Role{rbac.RoleViewer}, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := newTestAdminUserService(mockStore)

	result, err := svc.ListAdminUsers(context.Background(), adminOwnerPrincipal(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 users, got %d", len(result))
	}
	if result[0].Username != "alice" {
		t.Errorf("expected username %q, got %q", "alice", result[0].Username)
	}
}

func TestAdminUserService_List_ViewerAllowed(t *testing.T) {
	now := time.Now().UTC()
	mockStore := &mockAdminUserStore{
		admins: []store.Admin{
			{ID: 1, Username: "alice", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := newTestAdminUserService(mockStore)

	result, err := svc.ListAdminUsers(context.Background(), adminViewerPrincipal(2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 user, got %d", len(result))
	}
}

func TestAdminUserService_Create_Success(t *testing.T) {
	mockStore := &mockAdminUserStore{ownerCount: 1}
	svc := newTestAdminUserService(mockStore)

	result, err := svc.CreateAdminUser(
		context.Background(),
		adminOwnerPrincipal(1),
		"newuser",
		"securepass1",
		"New User",
		"new@example.com",
		"viewer",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Username != "newuser" {
		t.Errorf("expected username %q, got %q", "newuser", result.Username)
	}
	if result.DisplayName != "New User" {
		t.Errorf("expected display name %q, got %q", "New User", result.DisplayName)
	}
	if result.Email != "new@example.com" {
		t.Errorf("expected email %q, got %q", "new@example.com", result.Email)
	}
	if len(result.Roles) != 1 || result.Roles[0] != "viewer" {
		t.Errorf("expected roles [viewer], got %v", result.Roles)
	}
}

func TestAdminUserService_Create_ViewerDenied(t *testing.T) {
	mockStore := &mockAdminUserStore{}
	svc := newTestAdminUserService(mockStore)

	_, err := svc.CreateAdminUser(
		context.Background(),
		adminViewerPrincipal(2),
		"newuser",
		"securepass1",
		"",
		"",
		"viewer",
	)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestAdminUserService_Create_DuplicateUsername(t *testing.T) {
	mockStore := &mockAdminUserStore{
		createErr: store.ErrDuplicateUsername,
	}
	svc := newTestAdminUserService(mockStore)

	_, err := svc.CreateAdminUser(
		context.Background(),
		adminOwnerPrincipal(1),
		"existing",
		"securepass1",
		"",
		"",
		"viewer",
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for duplicate username, got %v", err)
	}
}

func TestAdminUserService_Create_InvalidUsername(t *testing.T) {
	mockStore := &mockAdminUserStore{}
	svc := newTestAdminUserService(mockStore)

	tests := []struct {
		name     string
		username string
	}{
		{"empty", ""},
		{"too long", string(make([]byte, 65))},
		{"invalid chars", "user name!"},
		{"spaces", "user name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateAdminUser(
				context.Background(),
				adminOwnerPrincipal(1),
				tt.username,
				"securepass1",
				"",
				"",
				"viewer",
			)
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput for username %q, got %v", tt.username, err)
			}
		})
	}
}

func TestAdminUserService_Create_ShortPassword(t *testing.T) {
	mockStore := &mockAdminUserStore{}
	svc := newTestAdminUserService(mockStore)

	_, err := svc.CreateAdminUser(
		context.Background(),
		adminOwnerPrincipal(1),
		"validuser",
		"short",
		"",
		"",
		"viewer",
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for short password, got %v", err)
	}
}

func TestAdminUserService_Create_InvalidEmail(t *testing.T) {
	mockStore := &mockAdminUserStore{}
	svc := newTestAdminUserService(mockStore)

	_, err := svc.CreateAdminUser(
		context.Background(),
		adminOwnerPrincipal(1),
		"validuser",
		"securepass1",
		"",
		"not-an-email",
		"viewer",
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for invalid email, got %v", err)
	}
}

func TestAdminUserService_Delete_CannotDeleteSelf(t *testing.T) {
	now := time.Now().UTC()
	mockStore := &mockAdminUserStore{
		admins: []store.Admin{
			{ID: 1, Username: "admin", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
		},
		ownerCount: 2,
	}
	svc := newTestAdminUserService(mockStore)

	err := svc.DeleteAdminUser(context.Background(), adminOwnerPrincipal(1), 1)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for self-deletion, got %v", err)
	}
}

func TestAdminUserService_Delete_CannotDeleteLastOwner(t *testing.T) {
	now := time.Now().UTC()
	mockStore := &mockAdminUserStore{
		admins: []store.Admin{
			{ID: 1, Username: "admin", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
			{ID: 2, Username: "other-owner", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
		},
		ownerCount: 1,
	}
	svc := newTestAdminUserService(mockStore)

	err := svc.DeleteAdminUser(context.Background(), adminOwnerPrincipal(1), 2)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for last owner deletion, got %v", err)
	}
}

func TestAdminUserService_Disable_CannotDisableSelf(t *testing.T) {
	now := time.Now().UTC()
	mockStore := &mockAdminUserStore{
		admins: []store.Admin{
			{ID: 1, Username: "admin", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := newTestAdminUserService(mockStore)

	err := svc.DisableAdminUser(context.Background(), adminOwnerPrincipal(1), 1)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for self-disable, got %v", err)
	}
}

func TestAdminUserService_Update_CannotDemoteLastOwner(t *testing.T) {
	now := time.Now().UTC()
	mockStore := &mockAdminUserStore{
		admins: []store.Admin{
			{ID: 1, Username: "admin", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
			{ID: 2, Username: "only-owner", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
		},
		ownerCount: 1,
	}
	svc := newTestAdminUserService(mockStore)

	viewerRole := "viewer"
	err := svc.UpdateAdminUser(context.Background(), adminOwnerPrincipal(1), 2, "Name", "", &viewerRole)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for last owner demotion, got %v", err)
	}
}

func TestAdminUserService_ResetPassword_ShortPassword(t *testing.T) {
	now := time.Now().UTC()
	mockStore := &mockAdminUserStore{
		admins: []store.Admin{
			{ID: 1, Username: "admin", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
			{ID: 2, Username: "target", Roles: []rbac.Role{rbac.RoleViewer}, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := newTestAdminUserService(mockStore)

	err := svc.ResetAdminPassword(context.Background(), adminOwnerPrincipal(1), 2, "short")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for short password, got %v", err)
	}
}

func TestAdminUserService_ResetPassword_Success(t *testing.T) {
	now := time.Now().UTC()
	mockStore := &mockAdminUserStore{
		admins: []store.Admin{
			{ID: 1, Username: "admin", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
			{ID: 2, Username: "target", Roles: []rbac.Role{rbac.RoleViewer}, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := newTestAdminUserService(mockStore)

	err := svc.ResetAdminPassword(context.Background(), adminOwnerPrincipal(1), 2, "newsecurepassword")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdminUserService_Enable_Success(t *testing.T) {
	now := time.Now().UTC()
	disabledAt := now.Add(-time.Hour)
	mockStore := &mockAdminUserStore{
		admins: []store.Admin{
			{ID: 1, Username: "admin", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
			{ID: 2, Username: "disabled", Roles: []rbac.Role{rbac.RoleViewer}, DisabledAt: &disabledAt, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := newTestAdminUserService(mockStore)

	err := svc.EnableAdminUser(context.Background(), adminOwnerPrincipal(1), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdminUserService_Delete_NotFound(t *testing.T) {
	mockStore := &mockAdminUserStore{
		admins:     []store.Admin{},
		ownerCount: 2,
	}
	svc := newTestAdminUserService(mockStore)

	err := svc.DeleteAdminUser(context.Background(), adminOwnerPrincipal(1), 99)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAdminUserService_Create_InvalidRole(t *testing.T) {
	mockStore := &mockAdminUserStore{}
	svc := newTestAdminUserService(mockStore)

	_, err := svc.CreateAdminUser(
		context.Background(),
		adminOwnerPrincipal(1),
		"validuser",
		"securepass1",
		"",
		"",
		"superadmin",
	)
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for invalid role, got %v", err)
	}
}

func TestAdminUserService_BulkDelete(t *testing.T) {
	now := time.Now().UTC()

	t.Run("deletes multiple users", func(t *testing.T) {
		mockStore := &mockAdminUserStore{
			admins: []store.Admin{
				{ID: 1, Username: "owner1", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
				{ID: 2, Username: "user2", Roles: []rbac.Role{rbac.RoleViewer}, CreatedAt: now, UpdatedAt: now},
				{ID: 3, Username: "user3", Roles: []rbac.Role{rbac.RoleViewer}, CreatedAt: now, UpdatedAt: now},
			},
			ownerCount: 1,
		}
		svc := newTestAdminUserService(mockStore)
		principal := adminOwnerPrincipal(1)

		result := svc.BulkDeleteAdminUsers(context.Background(), principal, []int64{2, 3})

		if result.Total != 2 {
			t.Errorf("expected total 2, got %d", result.Total)
		}
		if result.Succeeded != 2 {
			t.Errorf("expected succeeded 2, got %d", result.Succeeded)
		}
		if result.Failed != 0 {
			t.Errorf("expected failed 0, got %d", result.Failed)
		}
	})

	t.Run("skips self", func(t *testing.T) {
		mockStore := &mockAdminUserStore{
			admins: []store.Admin{
				{ID: 1, Username: "owner1", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
				{ID: 2, Username: "user2", Roles: []rbac.Role{rbac.RoleViewer}, CreatedAt: now, UpdatedAt: now},
			},
			ownerCount: 1,
		}
		svc := newTestAdminUserService(mockStore)
		principal := adminOwnerPrincipal(1)

		result := svc.BulkDeleteAdminUsers(context.Background(), principal, []int64{1, 2})

		if result.Succeeded != 1 {
			t.Errorf("expected succeeded 1, got %d", result.Succeeded)
		}
		if result.Failed != 1 {
			t.Errorf("expected failed 1, got %d", result.Failed)
		}
		if len(result.Errors) != 1 || result.Errors[0].ID != "1" {
			t.Errorf("expected error for ID 1, got %+v", result.Errors)
		}
	})

	t.Run("skips last owner", func(t *testing.T) {
		mockStore := &mockAdminUserStore{
			admins: []store.Admin{
				{ID: 1, Username: "owner1", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
				{ID: 2, Username: "owner2", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
				{ID: 3, Username: "viewer", Roles: []rbac.Role{rbac.RoleViewer}, CreatedAt: now, UpdatedAt: now},
			},
			ownerCount: 1, // Only 1 remaining owner
		}
		svc := newTestAdminUserService(mockStore)
		principal := adminOwnerPrincipal(1)

		result := svc.BulkDeleteAdminUsers(context.Background(), principal, []int64{2, 3})

		if result.Succeeded != 1 {
			t.Errorf("expected succeeded 1, got %d", result.Succeeded)
		}
		if result.Failed != 1 {
			t.Errorf("expected failed 1, got %d", result.Failed)
		}
		if len(result.Errors) == 0 || result.Errors[0].Error != "cannot delete last owner" {
			t.Errorf("expected 'cannot delete last owner' error, got %+v", result.Errors)
		}
	})

	t.Run("viewer denied", func(t *testing.T) {
		mockStore := &mockAdminUserStore{}
		svc := newTestAdminUserService(mockStore)
		principal := adminViewerPrincipal(5)

		result := svc.BulkDeleteAdminUsers(context.Background(), principal, []int64{2, 3})

		if result.Succeeded != 0 {
			t.Errorf("expected succeeded 0, got %d", result.Succeeded)
		}
		if result.Failed != 2 {
			t.Errorf("expected failed 2, got %d", result.Failed)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mockStore := &mockAdminUserStore{
			admins:     []store.Admin{},
			ownerCount: 1,
		}
		svc := newTestAdminUserService(mockStore)
		principal := adminOwnerPrincipal(1)

		result := svc.BulkDeleteAdminUsers(context.Background(), principal, []int64{99})

		if result.Failed != 1 {
			t.Errorf("expected failed 1, got %d", result.Failed)
		}
		if len(result.Errors) == 0 || result.Errors[0].Error != "not found" {
			t.Errorf("expected 'not found' error, got %+v", result.Errors)
		}
	})
}

func TestAdminUserService_BulkDisable(t *testing.T) {
	now := time.Now().UTC()

	t.Run("disables multiple", func(t *testing.T) {
		mockStore := &mockAdminUserStore{
			admins: []store.Admin{
				{ID: 2, Username: "user2", Roles: []rbac.Role{rbac.RoleViewer}, CreatedAt: now, UpdatedAt: now},
				{ID: 3, Username: "user3", Roles: []rbac.Role{rbac.RoleViewer}, CreatedAt: now, UpdatedAt: now},
			},
		}
		svc := newTestAdminUserService(mockStore)
		principal := adminOwnerPrincipal(1)

		result := svc.BulkDisableAdminUsers(context.Background(), principal, []int64{2, 3})

		if result.Succeeded != 2 {
			t.Errorf("expected succeeded 2, got %d", result.Succeeded)
		}
		if result.Failed != 0 {
			t.Errorf("expected failed 0, got %d", result.Failed)
		}
	})

	t.Run("skips self", func(t *testing.T) {
		mockStore := &mockAdminUserStore{
			admins: []store.Admin{
				{ID: 1, Username: "owner1", Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now, UpdatedAt: now},
				{ID: 2, Username: "user2", Roles: []rbac.Role{rbac.RoleViewer}, CreatedAt: now, UpdatedAt: now},
			},
		}
		svc := newTestAdminUserService(mockStore)
		principal := adminOwnerPrincipal(1)

		result := svc.BulkDisableAdminUsers(context.Background(), principal, []int64{1, 2})

		if result.Succeeded != 1 {
			t.Errorf("expected succeeded 1, got %d", result.Succeeded)
		}
		if result.Failed != 1 {
			t.Errorf("expected failed 1, got %d", result.Failed)
		}
	})

	t.Run("store error", func(t *testing.T) {
		mockStore := &mockAdminUserStore{
			admins: []store.Admin{
				{ID: 2, Username: "user2", Roles: []rbac.Role{rbac.RoleViewer}, CreatedAt: now, UpdatedAt: now},
			},
			disableErr: errors.New("db error"),
		}
		svc := newTestAdminUserService(mockStore)
		principal := adminOwnerPrincipal(1)

		result := svc.BulkDisableAdminUsers(context.Background(), principal, []int64{2})

		if result.Failed != 1 {
			t.Errorf("expected failed 1, got %d", result.Failed)
		}
	})
}

func TestAdminUserService_BulkEnable(t *testing.T) {
	now := time.Now().UTC()

	t.Run("enables multiple", func(t *testing.T) {
		disabledAt := now
		mockStore := &mockAdminUserStore{
			admins: []store.Admin{
				{ID: 2, Username: "user2", Roles: []rbac.Role{rbac.RoleViewer}, DisabledAt: &disabledAt, CreatedAt: now, UpdatedAt: now},
				{ID: 3, Username: "user3", Roles: []rbac.Role{rbac.RoleViewer}, DisabledAt: &disabledAt, CreatedAt: now, UpdatedAt: now},
			},
		}
		svc := newTestAdminUserService(mockStore)
		principal := adminOwnerPrincipal(1)

		result := svc.BulkEnableAdminUsers(context.Background(), principal, []int64{2, 3})

		if result.Succeeded != 2 {
			t.Errorf("expected succeeded 2, got %d", result.Succeeded)
		}
	})

	t.Run("handles not found", func(t *testing.T) {
		mockStore := &mockAdminUserStore{
			admins: []store.Admin{
				{ID: 2, Username: "user2", Roles: []rbac.Role{rbac.RoleViewer}, CreatedAt: now, UpdatedAt: now},
			},
		}
		svc := newTestAdminUserService(mockStore)
		principal := adminOwnerPrincipal(1)

		result := svc.BulkEnableAdminUsers(context.Background(), principal, []int64{2, 99})

		if result.Succeeded != 1 {
			t.Errorf("expected succeeded 1, got %d", result.Succeeded)
		}
		if result.Failed != 1 {
			t.Errorf("expected failed 1, got %d", result.Failed)
		}
	})
}
