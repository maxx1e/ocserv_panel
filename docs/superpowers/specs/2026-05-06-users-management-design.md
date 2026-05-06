# Users Management Page — Design Spec

**Date:** 2026-05-06  
**Status:** Approved  
**Scope:** Full CRUD for Web UI admin users and VPN (ocserv) users via a tabbed page

---

## Overview

Add a Users Management page at `/users` with two tabs:

1. **Web UI Users** — manages admin panel accounts stored in SQLite (`admin_users` table)
2. **VPN Users** — manages ocserv VPN accounts stored in the ocpasswd file

Each tab operates on a completely separate data source. There is no unification or linking between Web UI and VPN user identities.

---

## Architecture Decisions

- **Separate tabs, no unification.** A user needing both WebUI and VPN access is created independently in each tab.
- **RBAC stays owner/viewer.** No new roles. Owner has full CRUD; viewer has read-only.
- **ocpasswd file remains authoritative** for VPN users. No SQLite mirror.
- **SQLite `admin_users` table extended** with `display_name` and `email` columns.
- **Existing VPN user API enhanced** with lock/unlock endpoints.

---

## Data Model

### Migration `0004_admin_user_extensions.sql`

```sql
ALTER TABLE admin_users ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE admin_users ADD COLUMN email TEXT NOT NULL DEFAULT '';
```

No changes to the admin_sessions, admin_user_roles, or audit_log tables.

### Admin User (SQLite)

| Column | Type | Notes |
|--------|------|-------|
| id | INTEGER PK | Existing |
| username | TEXT UNIQUE | Existing |
| password_hash | TEXT | Existing (bcrypt) |
| display_name | TEXT | New, default '' |
| email | TEXT | New, default '' |
| disabled_at | DATETIME | Existing (null = enabled) |
| created_at | DATETIME | Existing |
| updated_at | DATETIME | Existing |

Roles via junction: `admin_user_roles(admin_user_id, role_name)`

### VPN User (ocpasswd file)

| Field | Source | Notes |
|-------|--------|-------|
| username | Parsed from file | Line format: `user:group:hash` |
| group | Parsed from file | May be empty or `*` |
| active | Parsed from hash | `!` prefix or literal `!` = disabled |

---

## Backend API

### New Permissions

Add to `rbac.go`:

```go
PermissionAdminUsersRead   Permission = "admin.users.read"
PermissionAdminUsersCreate Permission = "admin.users.create"
PermissionAdminUsersUpdate Permission = "admin.users.update"
PermissionAdminUsersDelete Permission = "admin.users.delete"
```

Grants:
- Owner: all four
- Viewer: `admin.users.read` only

### Admin User Endpoints

| Method | Path | Permission | Body/Params |
|--------|------|-----------|-------------|
| GET | `/api/admin/users` | admin.users.read | — |
| POST | `/api/admin/users` | admin.users.create | `{username, password, display_name?, email?, role?}` |
| PUT | `/api/admin/users/{id}` | admin.users.update | `{display_name?, email?, role?}` |
| DELETE | `/api/admin/users/{id}` | admin.users.delete | — |
| POST | `/api/admin/users/{id}/disable` | admin.users.update | — |
| POST | `/api/admin/users/{id}/enable` | admin.users.update | — |
| POST | `/api/admin/users/{id}/reset-password` | admin.users.update | `{password}` |

**Response shape (list):**

```json
{
  "users": [
    {
      "id": 1,
      "username": "admin",
      "display_name": "Administrator",
      "email": "admin@example.com",
      "roles": ["owner"],
      "disabled": false,
      "created_at": "2026-05-01T00:00:00Z",
      "updated_at": "2026-05-01T00:00:00Z"
    }
  ]
}
```

Passwords are never included in responses.

### VPN User Endpoints (Enhanced)

Existing endpoints remain. Add lock/unlock:

| Method | Path | Permission | Description |
|--------|------|-----------|-------------|
| GET | `/api/ocserv/users` | ocserv.users.read | List all VPN users |
| POST | `/api/ocserv/users` | ocserv.users.create | Create VPN user |
| DELETE | `/api/ocserv/users/{username}` | ocserv.users.delete | Delete VPN user |
| PUT | `/api/ocserv/users/{username}/password` | ocserv.users.update | Reset VPN password |
| POST | `/api/ocserv/users/{username}/lock` | ocserv.users.update | Disable VPN access |
| POST | `/api/ocserv/users/{username}/unlock` | ocserv.users.update | Re-enable VPN access |

**Lock implementation:** Write `!` as the password hash in ocpasswd file for that user.  
**Unlock implementation:** Cannot restore the original hash; user must reset password after unlock, or unlock restores access with a temporary state (implementation detail for adapter).

---

## Security Guards

1. **Cannot delete self.** API returns 400 if `principal.ID == targetID`.
2. **Cannot delete the last owner.** If only one owner remains, deletion/role-change is blocked.
3. **Cannot disable self.** Same as delete-self guard.
4. **Cannot demote the last owner.** If role change would leave zero owners, block it.
5. **All mutations audited.** Action, actor, target, outcome recorded.
6. **Password never in response.** Only hash stored; never returned.
7. **Validation on all inputs.** Username (alphanum + `._-`, max 64), email (format check), password (min 8, max 128).

---

## Audit Events

| Action | Resource Type | Resource ID |
|--------|--------------|-------------|
| `admin.users.create` | admin_user | username |
| `admin.users.update` | admin_user | username |
| `admin.users.delete` | admin_user | username |
| `admin.users.disable` | admin_user | username |
| `admin.users.enable` | admin_user | username |
| `admin.users.reset_password` | admin_user | username |
| `ocserv.users.lock` | ocserv_user | username |
| `ocserv.users.unlock` | ocserv_user | username |

Existing ocserv audit events (`ocserv.users.create`, `.delete`, `.update_password`) remain unchanged.

---

## Frontend

### Page Structure

```
/users
├── Tab: Web UI Users (default)
│   ├── Search input (filters by username, display_name, email)
│   ├── Role filter dropdown
│   ├── Status filter (all/active/disabled)
│   ├── User count badge
│   ├── Add User button (owner only)
│   ├── Export button
│   ├── Table (sortable columns, pagination, row actions)
│   └── Bulk actions (delete, disable)
└── Tab: VPN Users
    ├── Search input (filters by username)
    ├── Status filter (all/active/locked)
    ├── User count badge
    ├── Add User button (owner only)
    ├── Table (sortable, pagination, row actions)
    └── Bulk actions (delete, lock)
```

### Components

| Component | Purpose |
|-----------|---------|
| `+page.svelte` | Main page with tab state, fetches both user lists |
| `AdminUsersTab.svelte` | Web UI users table with search/filter/sort/pagination |
| `VpnUsersTab.svelte` | VPN users table with search/filter/sort/pagination |
| `AdminUserFormDialog.svelte` | Create/Edit admin user modal |
| `VpnUserFormDialog.svelte` | Create VPN user modal (username + password only) |
| `DeleteConfirmDialog.svelte` | Generic delete confirmation |
| `ResetPasswordDialog.svelte` | Password reset form (new password + confirm) |
| `UserTablePagination.svelte` | Rows-per-page selector + page navigation |

### State Management

Using Svelte 5 runes:
- `$state` for search term, active tab, sort column/direction, current page, page size, selected IDs, dialog open states
- `$derived` for filtered → sorted → paginated user lists
- `$effect` for resetting pagination on search/filter changes

### Dialogs

All dialogs use bindable `open` prop. Form submission calls the API and closes on success. Error states shown inline in the dialog.

### Permission-Based UI

- Owner: sees all action buttons, can open create/edit/delete/disable dialogs
- Viewer: sees the table, search, filters, export — no mutation buttons

### Notifications

Toast notifications for:
- User created successfully
- User updated
- User deleted
- Password reset
- User disabled/enabled
- User locked/unlocked
- Error messages (validation, permission denied, server error)

---

## Testing Strategy

### Backend Tests

1. **AdminUserService unit tests:**
   - CRUD operations
   - Security guards (delete self, last owner, disable self)
   - Validation (empty username, invalid email, short password)
   - RBAC enforcement (viewer cannot create/update/delete)

2. **VPN lock/unlock adapter tests:**
   - Lock writes `!` hash
   - Unlock clears disabled state
   - Lock/unlock non-existent user returns error

3. **API handler tests:**
   - Request parsing
   - Response shapes
   - Error responses (400, 401, 403, 404)

### Frontend Tests

4. **Page interaction tests** (if time permits):
   - Tab switching
   - Search filtering
   - Dialog open/close
   - API integration mocking

---

## Out of Scope

- Linking Web UI and VPN users (no "Both" type)
- New RBAC roles (operator, vpn_user)
- VPN group management UI
- User expiration dates
- OAuth/SSO for admin users
- VPN certificate-based user management
- Last login tracking (can be added later)
- User notes field

---

## Implementation Order

1. Migration (extend admin_users)
2. Backend store layer (admin user repository)
3. Backend service (AdminUserService with security guards)
4. RBAC permissions (new admin.users.* permissions)
5. API handlers (admin user endpoints)
6. VPN lock/unlock (adapter + endpoint)
7. Frontend API client (types + fetch functions)
8. Frontend page + components
9. Tests
10. Verification
