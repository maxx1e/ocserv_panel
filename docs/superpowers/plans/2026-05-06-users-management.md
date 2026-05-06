# Users Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a two-tab Users page managing Web UI admin users (SQLite) and VPN users (ocpasswd) with full CRUD, RBAC, audit logging, and security guards.

**Architecture:** Two independent data sources behind a tabbed UI. Admin users extend the existing `admin_users` table with `display_name` and `email`. VPN users use the existing `PasswdManager` interface with new lock/unlock methods. New `AdminUserService` handles business logic and security guards.

**Tech Stack:** Go backend, SQLite, SvelteKit + Tailwind CSS frontend, Svelte 5 runes, lucide-svelte icons

---

## File Structure

### Backend (new files)
- `migrations/0004_admin_user_extensions.sql` — ALTER TABLE for display_name, email
- `backend/internal/app/admin_user_service.go` — AdminUserService with CRUD + security guards
- `backend/internal/app/admin_user_service_test.go` — Unit tests for service
- `backend/internal/server/admin_user_handlers.go` — HTTP handlers for admin user API
- `backend/internal/server/admin_user_handlers_test.go` — Handler tests

### Backend (modified files)
- `backend/internal/store/store.go` — Add Admin CRUD methods (ListAdmins, CreateAdmin, UpdateAdmin, DeleteAdmin, FindAdminByID, CountOwners)
- `backend/internal/rbac/rbac.go` — Add PermissionAdminUsers* permissions
- `backend/internal/server/server.go` — Register new routes
- `backend/internal/ocserv/passwd.go` — Add LockUser/UnlockUser to PasswdManager interface
- `backend/internal/ocserv/passwd_adapter.go` — Implement lock/unlock via file manipulation
- `backend/internal/ocserv/passwd_fake.go` — Add lock/unlock to fake
- `backend/internal/app/ocserv_service.go` — Add LockOcservUser/UnlockOcservUser methods
- `backend/internal/server/ocserv_handlers.go` — Add lock/unlock handler

### Frontend (new files)
- `frontend/src/routes/users/AdminUsersTab.svelte` — Web UI users table
- `frontend/src/routes/users/VpnUsersTab.svelte` — VPN users table
- `frontend/src/routes/users/AdminUserFormDialog.svelte` — Create/Edit admin user dialog
- `frontend/src/routes/users/VpnUserFormDialog.svelte` — Create VPN user dialog
- `frontend/src/routes/users/DeleteConfirmDialog.svelte` — Confirm delete dialog
- `frontend/src/routes/users/ResetPasswordDialog.svelte` — Reset password dialog

### Frontend (modified files)
- `frontend/src/routes/users/+page.svelte` — Replace placeholder with tabbed page
- `frontend/src/lib/api.ts` — Add admin user types and API functions

---

## Task 1: Database Migration

**Files:**
- Create: `migrations/0004_admin_user_extensions.sql`

- [ ] **Step 1: Write the migration file**

```sql
ALTER TABLE admin_users ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE admin_users ADD COLUMN email TEXT NOT NULL DEFAULT '';
```

- [ ] **Step 2: Verify migration applies cleanly**

Run: `cd /opt/ocserv_panel && sqlite3 :memory: < migrations/0001_initial.sql && sqlite3 :memory: ".read migrations/0001_initial.sql" ".read migrations/0002_settings_registry.sql" ".read migrations/0003_ocserv_binary_settings.sql" ".read migrations/0004_admin_user_extensions.sql" ".schema admin_users"`

Expected: Table shows display_name and email columns.

- [ ] **Step 3: Commit**

```bash
git add migrations/0004_admin_user_extensions.sql
git commit -m "feat(db): add display_name and email to admin_users"
```

---

## Task 2: RBAC Permissions for Admin Users

**Files:**
- Modify: `backend/internal/rbac/rbac.go`

- [ ] **Step 1: Add permission constants**

Add after the existing `PermissionOcservConfigRollback` line:

```go
// Admin user management permissions
PermissionAdminUsersRead   Permission = "admin:users:read"
PermissionAdminUsersCreate Permission = "admin:users:create"
PermissionAdminUsersUpdate Permission = "admin:users:update"
PermissionAdminUsersDelete Permission = "admin:users:delete"
```

- [ ] **Step 2: Add grants to NewDefaultAuthorizer**

Add to RoleOwner map:
```go
PermissionAdminUsersRead:   {},
PermissionAdminUsersCreate: {},
PermissionAdminUsersUpdate: {},
PermissionAdminUsersDelete: {},
```

Add to RoleViewer map:
```go
PermissionAdminUsersRead: {},
```

- [ ] **Step 3: Verify build**

Run: `cd /opt/ocserv_panel/backend && go build ./...`
Expected: Clean build.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/rbac/rbac.go
git commit -m "feat(rbac): add admin user management permissions"
```

---

## Task 3: Store Layer — Admin User CRUD

**Files:**
- Modify: `backend/internal/store/store.go`

- [ ] **Step 1: Add new Admin struct fields and methods**

Update the `Admin` struct to include new fields:
```go
type Admin struct {
	ID           int64
	Username     string
	PasswordHash string
	DisplayName  string
	Email        string
	Roles        []rbac.Role
	DisabledAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
```

- [ ] **Step 2: Add ListAdmins method**

```go
// ListAdmins returns all admin users with their roles.
func (s *Store) ListAdmins(ctx context.Context) ([]Admin, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, username, password_hash, display_name, email, disabled_at, created_at, updated_at FROM admin_users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query admin users: %w", err)
	}
	defer rows.Close()

	var admins []Admin
	for rows.Next() {
		var a Admin
		var disabledAt sql.NullString
		var createdAt, updatedAt string
		if err := rows.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.DisplayName, &a.Email, &disabledAt, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan admin user: %w", err)
		}
		parsedCreatedAt, err := parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse admin created_at: %w", err)
		}
		a.CreatedAt = parsedCreatedAt
		parsedUpdatedAt, err := parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse admin updated_at: %w", err)
		}
		a.UpdatedAt = parsedUpdatedAt
		if disabledAt.Valid {
			parsed, err := parseTime(disabledAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse admin disabled_at: %w", err)
			}
			a.DisabledAt = &parsed
		}
		admins = append(admins, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin users: %w", err)
	}

	// Load roles for each admin
	for i := range admins {
		withRoles, _, err := s.withRoles(ctx, admins[i])
		if err != nil {
			return nil, err
		}
		admins[i] = withRoles
	}
	return admins, nil
}
```

- [ ] **Step 3: Add FindAdminByID method**

```go
// FindAdminByID returns an admin by ID with roles.
func (s *Store) FindAdminByID(ctx context.Context, id int64) (Admin, bool, error) {
	admin, found, err := s.findAdmin(ctx, `SELECT id, username, password_hash, disabled_at, created_at FROM admin_users WHERE id = ?`, id)
	if err != nil || !found {
		return admin, found, err
	}
	return s.withRoles(ctx, admin)
}
```

- [ ] **Step 4: Add CreateAdmin method**

```go
// CreateAdmin creates a new admin user with the given role.
func (s *Store) CreateAdmin(ctx context.Context, username, passwordHash, displayName, email string, role rbac.Role) (Admin, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Admin{}, fmt.Errorf("begin create admin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx,
		`INSERT INTO admin_users (username, password_hash, display_name, email, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		username, passwordHash, displayName, email, formatTime(now), formatTime(now))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return Admin{}, fmt.Errorf("username already exists")
		}
		return Admin{}, fmt.Errorf("insert admin user: %w", err)
	}
	adminID, err := result.LastInsertId()
	if err != nil {
		return Admin{}, fmt.Errorf("read admin id: %w", err)
	}

	var roleID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = ?`, string(role)).Scan(&roleID); err != nil {
		return Admin{}, fmt.Errorf("find role %s: %w", role, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_user_roles (admin_user_id, role_id, created_at) VALUES (?, ?, ?)`, adminID, roleID, formatTime(now)); err != nil {
		return Admin{}, fmt.Errorf("assign role: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Admin{}, fmt.Errorf("commit create admin tx: %w", err)
	}
	committed = true

	return Admin{
		ID:          adminID,
		Username:    username,
		DisplayName: displayName,
		Email:       email,
		Roles:       []rbac.Role{role},
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
```

- [ ] **Step 5: Add UpdateAdmin method**

```go
// UpdateAdmin updates display_name, email, and optionally role.
func (s *Store) UpdateAdmin(ctx context.Context, id int64, displayName, email string, role *rbac.Role) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update admin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx,
		`UPDATE admin_users SET display_name = ?, email = ?, updated_at = ? WHERE id = ?`,
		displayName, email, formatTime(now), id); err != nil {
		return fmt.Errorf("update admin user: %w", err)
	}

	if role != nil {
		// Replace all roles with the single new role
		if _, err := tx.ExecContext(ctx, `DELETE FROM admin_user_roles WHERE admin_user_id = ?`, id); err != nil {
			return fmt.Errorf("clear roles: %w", err)
		}
		var roleID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = ?`, string(*role)).Scan(&roleID); err != nil {
			return fmt.Errorf("find role %s: %w", *role, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO admin_user_roles (admin_user_id, role_id, created_at) VALUES (?, ?, ?)`, id, roleID, formatTime(now)); err != nil {
			return fmt.Errorf("assign new role: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update admin tx: %w", err)
	}
	committed = true
	return nil
}
```

- [ ] **Step 6: Add DeleteAdmin, DisableAdmin, EnableAdmin, UpdateAdminPassword, CountOwners methods**

```go
// DeleteAdmin removes an admin user by ID.
func (s *Store) DeleteAdmin(ctx context.Context, id int64) error {
	result, err := s.DB.ExecContext(ctx, `DELETE FROM admin_users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete admin: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("admin user not found")
	}
	return nil
}

// DisableAdmin sets disabled_at on an admin user.
func (s *Store) DisableAdmin(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	_, err := s.DB.ExecContext(ctx, `UPDATE admin_users SET disabled_at = ?, updated_at = ? WHERE id = ?`, formatTime(now), formatTime(now), id)
	if err != nil {
		return fmt.Errorf("disable admin: %w", err)
	}
	return nil
}

// EnableAdmin clears disabled_at on an admin user.
func (s *Store) EnableAdmin(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	_, err := s.DB.ExecContext(ctx, `UPDATE admin_users SET disabled_at = NULL, updated_at = ? WHERE id = ?`, formatTime(now), id)
	if err != nil {
		return fmt.Errorf("enable admin: %w", err)
	}
	return nil
}

// UpdateAdminPassword updates an admin user's password hash.
func (s *Store) UpdateAdminPassword(ctx context.Context, id int64, passwordHash string) error {
	now := time.Now().UTC()
	_, err := s.DB.ExecContext(ctx, `UPDATE admin_users SET password_hash = ?, updated_at = ? WHERE id = ?`, passwordHash, formatTime(now), id)
	if err != nil {
		return fmt.Errorf("update admin password: %w", err)
	}
	return nil
}

// CountOwners returns the number of admin users with the owner role.
func (s *Store) CountOwners(ctx context.Context) (int64, error) {
	var count int64
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(DISTINCT aur.admin_user_id) FROM admin_user_roles aur JOIN roles r ON r.id = aur.role_id WHERE r.name = ?`, string(rbac.RoleOwner)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count owners: %w", err)
	}
	return count, nil
}
```

- [ ] **Step 7: Update findAdmin to include new columns**

Update the `findAdmin` method query and scan to include `display_name`, `email`, `updated_at`:

```go
func (s *Store) findAdmin(ctx context.Context, query string, args ...any) (Admin, bool, error) {
	var admin Admin
	var disabledAt sql.NullString
	var createdAt, updatedAt string
	err := s.DB.QueryRowContext(ctx, query, args...).Scan(&admin.ID, &admin.Username, &admin.PasswordHash, &admin.DisplayName, &admin.Email, &disabledAt, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Admin{}, false, nil
		}
		return Admin{}, false, fmt.Errorf("query admin user: %w", err)
	}
	parsedCreatedAt, err := parseTime(createdAt)
	if err != nil {
		return Admin{}, false, fmt.Errorf("parse admin created_at: %w", err)
	}
	admin.CreatedAt = parsedCreatedAt
	parsedUpdatedAt, err := parseTime(updatedAt)
	if err != nil {
		return Admin{}, false, fmt.Errorf("parse admin updated_at: %w", err)
	}
	admin.UpdatedAt = parsedUpdatedAt
	if disabledAt.Valid {
		parsedDisabledAt, err := parseTime(disabledAt.String)
		if err != nil {
			return Admin{}, false, fmt.Errorf("parse admin disabled_at: %w", err)
		}
		admin.DisabledAt = &parsedDisabledAt
	}
	return admin, true, nil
}
```

**IMPORTANT:** Update ALL existing queries that call `findAdmin` to include the new columns. The existing queries in `FindAdminByUsername` and `FindAdminBySessionTokenHash` must select `display_name, email, updated_at` in addition to existing columns.

- [ ] **Step 8: Update FindAdminByUsername query**

```go
func (s *Store) FindAdminByUsername(ctx context.Context, username string) (Admin, bool, error) {
	admin, found, err := s.findAdmin(ctx, `SELECT id, username, password_hash, display_name, email, disabled_at, created_at, updated_at FROM admin_users WHERE username = ?`, username)
	if err != nil || !found {
		return admin, found, err
	}
	return s.withRoles(ctx, admin)
}
```

- [ ] **Step 9: Update FindAdminBySessionTokenHash query**

```go
func (s *Store) FindAdminBySessionTokenHash(ctx context.Context, tokenHash string, now time.Time) (Admin, bool, error) {
	admin, found, err := s.findAdmin(ctx, `
SELECT au.id, au.username, au.password_hash, au.display_name, au.email, au.disabled_at, au.created_at, au.updated_at
FROM admin_users au
JOIN admin_sessions s ON s.admin_user_id = au.id
WHERE s.token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ? AND au.disabled_at IS NULL`, tokenHash, formatTime(now))
	if err != nil || !found {
		return admin, found, err
	}
	return s.withRoles(ctx, admin)
}
```

- [ ] **Step 10: Add `strings` import if not present**

The `CreateAdmin` method uses `strings.Contains`. Ensure `"strings"` is in the import block.

- [ ] **Step 11: Verify build**

Run: `cd /opt/ocserv_panel/backend && go build ./...`
Expected: Clean build.

- [ ] **Step 12: Commit**

```bash
git add backend/internal/store/store.go
git commit -m "feat(store): add admin user CRUD methods with display_name and email"
```

---

## Task 4: AdminUserService with Security Guards

**Files:**
- Create: `backend/internal/app/admin_user_service.go`
- Create: `backend/internal/app/admin_user_service_test.go`

- [ ] **Step 1: Create AdminUserService**

```go
package app

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

// AdminUserStore defines the persistence interface used by AdminUserService.
type AdminUserStore interface {
	ListAdmins(ctx context.Context) ([]store.Admin, error)
	FindAdminByID(ctx context.Context, id int64) (store.Admin, bool, error)
	FindAdminByUsername(ctx context.Context, username string) (store.Admin, bool, error)
	CreateAdmin(ctx context.Context, username, passwordHash, displayName, email string, role rbac.Role) (store.Admin, error)
	UpdateAdmin(ctx context.Context, id int64, displayName, email string, role *rbac.Role) error
	DeleteAdmin(ctx context.Context, id int64) error
	DisableAdmin(ctx context.Context, id int64) error
	EnableAdmin(ctx context.Context, id int64) error
	UpdateAdminPassword(ctx context.Context, id int64, passwordHash string) error
	CountOwners(ctx context.Context) (int64, error)
}

// AdminUserService manages Web UI admin user lifecycle with RBAC and audit.
type AdminUserService struct {
	store      AdminUserStore
	recorder   audit.Recorder
	authorizer rbac.Authorizer
	hashFn     func(password string) (string, error)
}

// NewAdminUserService creates an AdminUserService.
func NewAdminUserService(store AdminUserStore, recorder audit.Recorder, authorizer rbac.Authorizer, hashFn func(string) (string, error)) *AdminUserService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	return &AdminUserService{
		store:      store,
		recorder:   recorder,
		authorizer: authorizer,
		hashFn:     hashFn,
	}
}

// AdminUserResponse is the API-safe representation (no password hash).
type AdminUserResponse struct {
	ID          int64       `json:"id"`
	Username    string      `json:"username"`
	DisplayName string      `json:"displayName"`
	Email       string      `json:"email"`
	Roles       []string    `json:"roles"`
	Disabled    bool        `json:"disabled"`
	CreatedAt   string      `json:"createdAt"`
	UpdatedAt   string      `json:"updatedAt"`
}

var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// ListAdminUsers returns all admin users (safe for API response).
func (s *AdminUserService) ListAdminUsers(ctx context.Context, principal auth.Principal) ([]AdminUserResponse, error) {
	if err := s.requirePerm(principal, rbac.PermissionAdminUsersRead, "admin.users.list"); err != nil {
		return nil, err
	}
	admins, err := s.store.ListAdmins(ctx)
	if err != nil {
		return nil, fmt.Errorf("list admins: %w", err)
	}
	result := make([]AdminUserResponse, 0, len(admins))
	for _, a := range admins {
		result = append(result, adminToResponse(a))
	}
	return result, nil
}

// CreateAdminUser creates a new admin user.
func (s *AdminUserService) CreateAdminUser(ctx context.Context, principal auth.Principal, username, password, displayName, email, role string) (*AdminUserResponse, error) {
	if err := s.requirePerm(principal, rbac.PermissionAdminUsersCreate, "admin.users.create"); err != nil {
		return nil, err
	}
	// Validate inputs
	if err := s.validateUsername(username); err != nil {
		return nil, err
	}
	if err := s.validatePassword(password); err != nil {
		return nil, err
	}
	if email != "" && !emailRegex.MatchString(email) {
		return nil, ErrInvalidInput
	}
	parsedRole := rbac.Role(role)
	if parsedRole != rbac.RoleOwner && parsedRole != rbac.RoleViewer {
		return nil, ErrInvalidInput
	}

	hash, err := s.hashFn(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	actorID := principal.ID
	admin, err := s.store.CreateAdmin(ctx, username, hash, displayName, email, parsedRole)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			_ = s.record(ctx, &actorID, "admin.users.create", "admin_user", username, audit.OutcomeFailure, "username already exists")
			return nil, ErrInvalidInput
		}
		_ = s.record(ctx, &actorID, "admin.users.create", "admin_user", username, audit.OutcomeFailure, err.Error())
		return nil, fmt.Errorf("create admin: %w", err)
	}
	_ = s.record(ctx, &actorID, "admin.users.create", "admin_user", username, audit.OutcomeSuccess, "admin user created")
	resp := adminToResponse(admin)
	return &resp, nil
}

// UpdateAdminUser updates display_name, email, and role.
func (s *AdminUserService) UpdateAdminUser(ctx context.Context, principal auth.Principal, targetID int64, displayName, email, role string) error {
	if err := s.requirePerm(principal, rbac.PermissionAdminUsersUpdate, "admin.users.update"); err != nil {
		return err
	}
	if email != "" && !emailRegex.MatchString(email) {
		return ErrInvalidInput
	}

	target, found, err := s.store.FindAdminByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("find admin: %w", err)
	}
	if !found {
		return ErrNotFound
	}

	var rolePtr *rbac.Role
	if role != "" {
		parsedRole := rbac.Role(role)
		if parsedRole != rbac.RoleOwner && parsedRole != rbac.RoleViewer {
			return ErrInvalidInput
		}
		// Guard: cannot demote last owner
		if s.hasRole(target, rbac.RoleOwner) && parsedRole != rbac.RoleOwner {
			count, err := s.store.CountOwners(ctx)
			if err != nil {
				return fmt.Errorf("count owners: %w", err)
			}
			if count <= 1 {
				return ErrInvalidInput
			}
		}
		rolePtr = &parsedRole
	}

	actorID := principal.ID
	if err := s.store.UpdateAdmin(ctx, targetID, displayName, email, rolePtr); err != nil {
		_ = s.record(ctx, &actorID, "admin.users.update", "admin_user", target.Username, audit.OutcomeFailure, err.Error())
		return fmt.Errorf("update admin: %w", err)
	}
	_ = s.record(ctx, &actorID, "admin.users.update", "admin_user", target.Username, audit.OutcomeSuccess, "admin user updated")
	return nil
}

// DeleteAdminUser deletes an admin user with safety guards.
func (s *AdminUserService) DeleteAdminUser(ctx context.Context, principal auth.Principal, targetID int64) error {
	if err := s.requirePerm(principal, rbac.PermissionAdminUsersDelete, "admin.users.delete"); err != nil {
		return err
	}
	// Guard: cannot delete self
	if principal.ID == targetID {
		return ErrInvalidInput
	}
	target, found, err := s.store.FindAdminByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("find admin: %w", err)
	}
	if !found {
		return ErrNotFound
	}
	// Guard: cannot delete last owner
	if s.hasRole(target, rbac.RoleOwner) {
		count, err := s.store.CountOwners(ctx)
		if err != nil {
			return fmt.Errorf("count owners: %w", err)
		}
		if count <= 1 {
			return ErrInvalidInput
		}
	}
	actorID := principal.ID
	if err := s.store.DeleteAdmin(ctx, targetID); err != nil {
		_ = s.record(ctx, &actorID, "admin.users.delete", "admin_user", target.Username, audit.OutcomeFailure, err.Error())
		return fmt.Errorf("delete admin: %w", err)
	}
	_ = s.record(ctx, &actorID, "admin.users.delete", "admin_user", target.Username, audit.OutcomeSuccess, "admin user deleted")
	return nil
}

// DisableAdminUser disables an admin user.
func (s *AdminUserService) DisableAdminUser(ctx context.Context, principal auth.Principal, targetID int64) error {
	if err := s.requirePerm(principal, rbac.PermissionAdminUsersUpdate, "admin.users.disable"); err != nil {
		return err
	}
	// Guard: cannot disable self
	if principal.ID == targetID {
		return ErrInvalidInput
	}
	target, found, err := s.store.FindAdminByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("find admin: %w", err)
	}
	if !found {
		return ErrNotFound
	}
	actorID := principal.ID
	if err := s.store.DisableAdmin(ctx, targetID); err != nil {
		_ = s.record(ctx, &actorID, "admin.users.disable", "admin_user", target.Username, audit.OutcomeFailure, err.Error())
		return fmt.Errorf("disable admin: %w", err)
	}
	_ = s.record(ctx, &actorID, "admin.users.disable", "admin_user", target.Username, audit.OutcomeSuccess, "admin user disabled")
	return nil
}

// EnableAdminUser enables an admin user.
func (s *AdminUserService) EnableAdminUser(ctx context.Context, principal auth.Principal, targetID int64) error {
	if err := s.requirePerm(principal, rbac.PermissionAdminUsersUpdate, "admin.users.enable"); err != nil {
		return err
	}
	target, found, err := s.store.FindAdminByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("find admin: %w", err)
	}
	if !found {
		return ErrNotFound
	}
	actorID := principal.ID
	if err := s.store.EnableAdmin(ctx, targetID); err != nil {
		_ = s.record(ctx, &actorID, "admin.users.enable", "admin_user", target.Username, audit.OutcomeFailure, err.Error())
		return fmt.Errorf("enable admin: %w", err)
	}
	_ = s.record(ctx, &actorID, "admin.users.enable", "admin_user", target.Username, audit.OutcomeSuccess, "admin user enabled")
	return nil
}

// ResetAdminPassword resets an admin user's password.
func (s *AdminUserService) ResetAdminPassword(ctx context.Context, principal auth.Principal, targetID int64, newPassword string) error {
	if err := s.requirePerm(principal, rbac.PermissionAdminUsersUpdate, "admin.users.reset_password"); err != nil {
		return err
	}
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}
	target, found, err := s.store.FindAdminByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("find admin: %w", err)
	}
	if !found {
		return ErrNotFound
	}
	hash, err := s.hashFn(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	actorID := principal.ID
	if err := s.store.UpdateAdminPassword(ctx, targetID, hash); err != nil {
		_ = s.record(ctx, &actorID, "admin.users.reset_password", "admin_user", target.Username, audit.OutcomeFailure, err.Error())
		return fmt.Errorf("update password: %w", err)
	}
	_ = s.record(ctx, &actorID, "admin.users.reset_password", "admin_user", target.Username, audit.OutcomeSuccess, "admin user password reset")
	return nil
}

func (s *AdminUserService) requirePerm(principal auth.Principal, permission rbac.Permission, action string) error {
	if !principal.Authenticated() {
		return ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), permission) {
		return ErrPermissionDenied
	}
	return nil
}

func (s *AdminUserService) validateUsername(username string) error {
	if username == "" || len(username) > 64 || !usernameRegex.MatchString(username) {
		return ErrInvalidInput
	}
	return nil
}

func (s *AdminUserService) validatePassword(password string) error {
	if len(password) < 8 || len(password) > 128 || strings.Contains(password, "\n") {
		return ErrInvalidInput
	}
	return nil
}

func (s *AdminUserService) hasRole(admin store.Admin, role rbac.Role) bool {
	for _, r := range admin.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (s *AdminUserService) record(ctx context.Context, actorID *int64, action, resourceType, resourceID string, outcome audit.Outcome, message string) error {
	return s.recorder.Record(ctx, audit.Event{
		ActorID:      actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Outcome:      outcome,
		Message:      message,
	})
}

func adminToResponse(a store.Admin) AdminUserResponse {
	roles := make([]string, 0, len(a.Roles))
	for _, r := range a.Roles {
		roles = append(roles, string(r))
	}
	return AdminUserResponse{
		ID:          a.ID,
		Username:    a.Username,
		DisplayName: a.DisplayName,
		Email:       a.Email,
		Roles:       roles,
		Disabled:    a.DisabledAt != nil,
		CreatedAt:   a.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   a.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
```

- [ ] **Step 2: Add ErrNotFound to errors.go (or app.go where ErrInvalidInput is defined)**

Check existing error definitions in `backend/internal/app/` and add `ErrNotFound` if it doesn't exist:
```go
var ErrNotFound = errors.New("not found")
```

- [ ] **Step 3: Verify build**

Run: `cd /opt/ocserv_panel/backend && go build ./...`

- [ ] **Step 4: Write tests for AdminUserService**

Create `backend/internal/app/admin_user_service_test.go` with tests covering:
- List users (owner and viewer both succeed)
- Create user (owner succeeds, viewer denied)
- Delete self blocked
- Delete last owner blocked
- Disable self blocked
- Demote last owner blocked
- Invalid username rejected
- Short password rejected
- Invalid email rejected

- [ ] **Step 5: Run tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/app/...`
Expected: All pass.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/app/admin_user_service.go backend/internal/app/admin_user_service_test.go
git commit -m "feat(app): add AdminUserService with CRUD and security guards"
```

---

## Task 5: Admin User HTTP Handlers

**Files:**
- Create: `backend/internal/server/admin_user_handlers.go`
- Modify: `backend/internal/server/server.go` (add routes and service field)

- [ ] **Step 1: Add adminUserService to App struct**

In `server.go`, add to the `App` struct:
```go
adminUserService *app.AdminUserService
```

Add to `NewAppRouter` params and wire it.

- [ ] **Step 2: Create admin_user_handlers.go**

```go
package server

import (
	"net/http"
	"strconv"
	"strings"
)

type createAdminUserRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}

type updateAdminUserRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}

type resetAdminPasswordRequest struct {
	Password string `json:"password"`
}

func (a *App) handleAdminUsersList(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	users, err := a.adminUserService.ListAdminUsers(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (a *App) handleAdminUserCreate(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	var req createAdminUserRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if req.Role == "" {
		req.Role = "viewer"
	}
	user, err := a.adminUserService.CreateAdminUser(r.Context(), principal, req.Username, req.Password, req.DisplayName, req.Email, req.Role)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (a *App) handleAdminUserUpdate(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPut) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	id, err := extractAdminUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req updateAdminUserRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if err := a.adminUserService.UpdateAdminUser(r.Context(), principal, id, req.DisplayName, req.Email, req.Role); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleAdminUserDelete(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodDelete) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	id, err := extractAdminUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := a.adminUserService.DeleteAdminUser(r.Context(), principal, id); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleAdminUserDisable(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	id, err := extractAdminUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := a.adminUserService.DisableAdminUser(r.Context(), principal, id); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleAdminUserEnable(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	id, err := extractAdminUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := a.adminUserService.EnableAdminUser(r.Context(), principal, id); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleAdminUserResetPassword(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	id, err := extractAdminUserID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req resetAdminPasswordRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	if err := a.adminUserService.ResetAdminPassword(r.Context(), principal, id, req.Password); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func extractAdminUserID(r *http.Request) (int64, error) {
	idStr := r.PathValue("id")
	if idStr == "" {
		// Fallback: parse from URL path
		parts := strings.Split(r.URL.Path, "/")
		for i, p := range parts {
			if p == "users" && i+1 < len(parts) {
				idStr = parts[i+1]
				break
			}
		}
	}
	return strconv.ParseInt(idStr, 10, 64)
}
```

- [ ] **Step 3: Register routes in server.go**

Add after the existing ocserv routes block:
```go
// Admin user management API
mux.HandleFunc("GET /api/admin/users", appHandler.handleAdminUsersList)
mux.HandleFunc("POST /api/admin/users", appHandler.handleAdminUserCreate)
mux.HandleFunc("PUT /api/admin/users/{id}", appHandler.handleAdminUserUpdate)
mux.HandleFunc("DELETE /api/admin/users/{id}", appHandler.handleAdminUserDelete)
mux.HandleFunc("POST /api/admin/users/{id}/disable", appHandler.handleAdminUserDisable)
mux.HandleFunc("POST /api/admin/users/{id}/enable", appHandler.handleAdminUserEnable)
mux.HandleFunc("POST /api/admin/users/{id}/reset-password", appHandler.handleAdminUserResetPassword)
```

- [ ] **Step 4: Verify build**

Run: `cd /opt/ocserv_panel/backend && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add backend/internal/server/admin_user_handlers.go backend/internal/server/server.go
git commit -m "feat(api): add admin user management HTTP endpoints"
```

---

## Task 6: VPN User Lock/Unlock

**Files:**
- Modify: `backend/internal/ocserv/passwd.go` — Add LockUser/UnlockUser to interface
- Modify: `backend/internal/ocserv/passwd_adapter.go` — Implement lock/unlock
- Modify: `backend/internal/ocserv/passwd_fake.go` — Add to fake
- Modify: `backend/internal/app/ocserv_service.go` — Add service methods
- Modify: `backend/internal/server/ocserv_handlers.go` — Add handlers
- Modify: `backend/internal/server/server.go` — Register routes

- [ ] **Step 1: Extend PasswdManager interface**

Add to `passwd.go`:
```go
type PasswdManager interface {
	CreateUser(ctx context.Context, username, password string) error
	DeleteUser(ctx context.Context, username string) error
	UpdatePassword(ctx context.Context, username, newPassword string) error
	ListUsers(ctx context.Context) ([]PasswdUser, error)
	UserExists(ctx context.Context, username string) (bool, error)
	LockUser(ctx context.Context, username string) error
	UnlockUser(ctx context.Context, username string) error
}
```

- [ ] **Step 2: Implement LockUser/UnlockUser in OcpasswdAdapter**

Add to `passwd_adapter.go`:
```go
// LockUser disables a user by replacing their password hash with "!" in the file.
func (a *OcpasswdAdapter) LockUser(ctx context.Context, username string) error {
	_ = ctx
	return a.setUserActive(username, false)
}

// UnlockUser re-enables a user by removing the "!" lock marker.
// The user will need a password reset after unlock since the hash is destroyed.
func (a *OcpasswdAdapter) UnlockUser(ctx context.Context, username string) error {
	_ = ctx
	return a.setUserActive(username, true)
}

func (a *OcpasswdAdapter) setUserActive(username string, active bool) error {
	data, err := os.ReadFile(a.filePath)
	if err != nil {
		return fmt.Errorf("read password file: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 3)
		if len(parts) < 2 || parts[0] != username {
			continue
		}
		found = true
		if active {
			// If locked (hash is "!"), set to "*" which means needs password reset
			if len(parts) == 3 && parts[2] == "!" {
				parts[2] = "*"
				lines[i] = strings.Join(parts, ":")
			}
		} else {
			// Lock: set hash to "!"
			if len(parts) < 3 {
				lines[i] = trimmed + ":!"
			} else {
				parts[2] = "!"
				lines[i] = strings.Join(parts, ":")
			}
		}
		break
	}
	if !found {
		return fmt.Errorf("user %s not found", username)
	}
	return os.WriteFile(a.filePath, []byte(strings.Join(lines, "\n")), 0600)
}
```

- [ ] **Step 3: Update FakePasswdManager**

Add to `passwd_fake.go`:
```go
func (f *FakePasswdManager) LockUser(ctx context.Context, username string) error {
	// Record call and check for configured error
	return f.maybeError("LockUser", username)
}

func (f *FakePasswdManager) UnlockUser(ctx context.Context, username string) error {
	return f.maybeError("UnlockUser", username)
}
```

(Adapt to existing FakePasswdManager pattern — check the actual file for how errors are stored.)

- [ ] **Step 4: Add service methods to OcservService**

Add to `ocserv_service.go`:
```go
// LockOcservUser locks (disables) a VPN user.
func (s *OcservService) LockOcservUser(ctx context.Context, principal auth.Principal, username string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersUpdate, "ocserv.users.lock"); err != nil {
		return err
	}
	if err := ocserv.ValidateUsername(username); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}
	actorID := principal.ID
	if err := s.users.LockUser(ctx, username); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.users.lock", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("lock ocserv user %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.users.lock", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user locked", nil)
	return nil
}

// UnlockOcservUser unlocks (re-enables) a VPN user.
func (s *OcservService) UnlockOcservUser(ctx context.Context, principal auth.Principal, username string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersUpdate, "ocserv.users.unlock"); err != nil {
		return err
	}
	if err := ocserv.ValidateUsername(username); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}
	actorID := principal.ID
	if err := s.users.UnlockUser(ctx, username); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.users.unlock", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("unlock ocserv user %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.users.unlock", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user unlocked", nil)
	return nil
}
```

- [ ] **Step 5: Add handlers in ocserv_handlers.go**

```go
func (a *App) handleOcservUserLock(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	username := r.PathValue("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	if err := a.ocservService.LockOcservUser(r.Context(), principal, username); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleOcservUserUnlock(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	username := r.PathValue("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	if err := a.ocservService.UnlockOcservUser(r.Context(), principal, username); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 6: Register routes in server.go**

Add after existing ocserv user routes:
```go
mux.HandleFunc("POST /api/ocserv/users/{username}/lock", appHandler.handleOcservUserLock)
mux.HandleFunc("POST /api/ocserv/users/{username}/unlock", appHandler.handleOcservUserUnlock)
```

- [ ] **Step 7: Verify build**

Run: `cd /opt/ocserv_panel/backend && go build ./...`

- [ ] **Step 8: Commit**

```bash
git add backend/internal/ocserv/passwd.go backend/internal/ocserv/passwd_adapter.go backend/internal/ocserv/passwd_fake.go backend/internal/app/ocserv_service.go backend/internal/server/ocserv_handlers.go backend/internal/server/server.go
git commit -m "feat(ocserv): add VPN user lock/unlock support"
```

---

## Task 7: Wire AdminUserService in main.go

**Files:**
- Modify: `backend/cmd/server/main.go` (or wherever services are wired)

- [ ] **Step 1: Find and update the wiring code**

Locate where `NewAppRouter` is called. Add `AdminUserService` creation and pass it to the router.

```go
adminUserService := app.NewAdminUserService(dataStore, auditRecorder, authorizer, auth.HashPassword)
```

Pass it as a parameter to `NewAppRouter`. Update the function signature accordingly.

- [ ] **Step 2: Verify build**

Run: `cd /opt/ocserv_panel/backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/server/main.go backend/internal/server/server.go
git commit -m "feat: wire AdminUserService into application"
```

---

## Task 8: Frontend API Client Types

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add types**

```typescript
export type AdminUser = {
  id: number;
  username: string;
  displayName: string;
  email: string;
  roles: string[];
  disabled: boolean;
  createdAt: string;
  updatedAt: string;
};

export type VpnUser = {
  username: string;
  group: string;
  active: boolean;
};
```

- [ ] **Step 2: Add API functions**

```typescript
export async function getAdminUsers(): Promise<AdminUser[]> {
  const res = await apiFetch('/api/admin/users');
  const data = await res.json();
  return data.users ?? [];
}

export async function createAdminUser(body: { username: string; password: string; displayName?: string; email?: string; role?: string }): Promise<AdminUser> {
  const res = await apiFetch('/api/admin/users', { method: 'POST', body: JSON.stringify(body) });
  return res.json();
}

export async function updateAdminUser(id: number, body: { displayName?: string; email?: string; role?: string }): Promise<void> {
  await apiFetch(`/api/admin/users/${id}`, { method: 'PUT', body: JSON.stringify(body) });
}

export async function deleteAdminUser(id: number): Promise<void> {
  await apiFetch(`/api/admin/users/${id}`, { method: 'DELETE' });
}

export async function disableAdminUser(id: number): Promise<void> {
  await apiFetch(`/api/admin/users/${id}/disable`, { method: 'POST' });
}

export async function enableAdminUser(id: number): Promise<void> {
  await apiFetch(`/api/admin/users/${id}/enable`, { method: 'POST' });
}

export async function resetAdminPassword(id: number, password: string): Promise<void> {
  await apiFetch(`/api/admin/users/${id}/reset-password`, { method: 'POST', body: JSON.stringify({ password }) });
}

export async function getVpnUsers(): Promise<VpnUser[]> {
  const res = await apiFetch('/api/ocserv/users');
  return res.json();
}

export async function createVpnUser(username: string, password: string): Promise<void> {
  await apiFetch('/api/ocserv/users/create', { method: 'POST', body: JSON.stringify({ username, password }) });
}

export async function deleteVpnUser(username: string): Promise<void> {
  await apiFetch(`/api/ocserv/users/${username}`, { method: 'DELETE' });
}

export async function resetVpnPassword(username: string, password: string): Promise<void> {
  await apiFetch(`/api/ocserv/users/${username}/password`, { method: 'PUT', body: JSON.stringify({ password }) });
}

export async function lockVpnUser(username: string): Promise<void> {
  await apiFetch(`/api/ocserv/users/${username}/lock`, { method: 'POST' });
}

export async function unlockVpnUser(username: string): Promise<void> {
  await apiFetch(`/api/ocserv/users/${username}/unlock`, { method: 'POST' });
}
```

- [ ] **Step 3: Verify frontend builds**

Run: `cd /opt/ocserv_panel/frontend && npm run check`

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat(frontend): add admin and VPN user API client types and functions"
```

---

## Task 9: Frontend Users Page (Main + Tabs)

**Files:**
- Rewrite: `frontend/src/routes/users/+page.svelte`
- Create: `frontend/src/routes/users/AdminUsersTab.svelte`
- Create: `frontend/src/routes/users/VpnUsersTab.svelte`

- [ ] **Step 1: Write main page with tab switching**

Replace `frontend/src/routes/users/+page.svelte` with a tabbed page that:
- Fetches admin users and VPN users on mount
- Renders tab buttons (Web UI Users / VPN Users)
- Passes data to the active tab component
- Manages principal state for permission checks
- Shows loading/error states

Use the same patterns as `sessions/+page.svelte`: `$state`, `$derived`, `onMount` with API calls, error handling.

- [ ] **Step 2: Write AdminUsersTab.svelte**

Component that receives `users: AdminUser[]` and `principal` as props. Implements:
- Search input filtering by username, displayName, email
- Role filter dropdown (All / Owner / Viewer)
- Status filter (All / Active / Disabled)
- Sortable table columns (Username, Display Name, Email, Role, Status, Created)
- Pagination with rows-per-page selector
- Checkbox selection for bulk actions
- Row action buttons (edit, disable/enable, reset password, delete) — visible only for owner
- User count badge
- Add User and Export buttons (owner only)
- Opens dialogs for create/edit/delete/reset-password

- [ ] **Step 3: Write VpnUsersTab.svelte**

Similar to AdminUsersTab but for VPN users. Receives `users: VpnUser[]` and `principal`. Implements:
- Search by username
- Status filter (All / Active / Locked)
- Sortable table (Username, Group, Status)
- Pagination
- Row actions (lock/unlock, reset password, delete)
- Add User button (owner only)
- Opens VPN-specific dialogs

- [ ] **Step 4: Verify frontend builds**

Run: `cd /opt/ocserv_panel/frontend && npm run check`

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/users/
git commit -m "feat(frontend): implement users page with admin and VPN tabs"
```

---

## Task 10: Frontend Dialogs

**Files:**
- Create: `frontend/src/routes/users/AdminUserFormDialog.svelte`
- Create: `frontend/src/routes/users/VpnUserFormDialog.svelte`
- Create: `frontend/src/routes/users/DeleteConfirmDialog.svelte`
- Create: `frontend/src/routes/users/ResetPasswordDialog.svelte`

- [ ] **Step 1: AdminUserFormDialog**

Modal with fields: username (create only), password (create only), display name, email, role selector. Validates client-side (required fields, min password length 8). Calls `createAdminUser` or `updateAdminUser`. Closes on success, shows error inline on failure.

- [ ] **Step 2: VpnUserFormDialog**

Modal with fields: username, password, confirm password. Validates (required, passwords match, min 8 chars). Calls `createVpnUser`. Closes on success.

- [ ] **Step 3: DeleteConfirmDialog**

Generic confirmation modal. Receives: `open` (bindable), `itemName`, `onConfirm` callback. Shows "Are you sure you want to delete {itemName}?" with Cancel/Delete buttons. Calls onConfirm and closes.

- [ ] **Step 4: ResetPasswordDialog**

Modal with new password + confirm password fields. Validates match and min length. Calls appropriate reset function. Closes on success.

- [ ] **Step 5: Verify frontend builds**

Run: `cd /opt/ocserv_panel/frontend && npm run check`

- [ ] **Step 6: Commit**

```bash
git add frontend/src/routes/users/
git commit -m "feat(frontend): add user management dialogs (create, edit, delete, reset password)"
```

---

## Task 11: Backend Tests

**Files:**
- Modify/Create: `backend/internal/app/admin_user_service_test.go`
- Create: `backend/internal/ocserv/passwd_adapter_lock_test.go`

- [ ] **Step 1: Write AdminUserService tests**

Test cases:
- `TestListAdminUsers_OwnerAllowed` — owner can list
- `TestListAdminUsers_ViewerAllowed` — viewer can list
- `TestCreateAdminUser_ViewerDenied` — viewer cannot create
- `TestCreateAdminUser_Success` — owner creates user
- `TestCreateAdminUser_DuplicateUsername` — returns error
- `TestDeleteAdminUser_CannotDeleteSelf` — returns error
- `TestDeleteAdminUser_CannotDeleteLastOwner` — returns error
- `TestDisableAdminUser_CannotDisableSelf` — returns error
- `TestUpdateAdminUser_CannotDemoteLastOwner` — returns error
- `TestResetAdminPassword_ShortPasswordRejected` — returns error

- [ ] **Step 2: Write lock/unlock adapter tests**

Test `ParsePasswdFile` with locked entries, and test `setUserActive` with file manipulation.

- [ ] **Step 3: Run all backend tests**

Run: `cd /opt/ocserv_panel/backend && go test ./...`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/app/admin_user_service_test.go backend/internal/ocserv/
git commit -m "test: add admin user service and VPN lock/unlock tests"
```

---

## Task 12: Final Verification

- [ ] **Step 1: Run full backend build and tests**

```bash
cd /opt/ocserv_panel/backend && go build ./... && go test ./...
```

- [ ] **Step 2: Run frontend check**

```bash
cd /opt/ocserv_panel/frontend && npm run check && npm run build
```

- [ ] **Step 3: Verify migration applies**

```bash
cd /opt/ocserv_panel && sqlite3 :memory: < migrations/0001_initial.sql && echo "OK"
```

- [ ] **Step 4: Report results**

Report: files changed, endpoints added, tests passing, any known issues.
