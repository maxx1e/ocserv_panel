# Bulk Operations for User Management — Design Spec

**Date:** 2026-05-07  
**Status:** Approved  
**Scope:** Add bulk delete/disable/enable/lock/unlock operations to both Admin and VPN user tabs

---

## Overview

Add bulk operations to the existing Users Management page. Both the Admin Users tab and VPN Users tab get checkbox selection with a floating action toolbar for batch operations.

All VPN users from the ocpasswd file are already visible and manageable regardless of how they were created (UI or CLI). No origin tracking is needed.

---

## Backend API

### New Endpoints

| Method | Path | Permission | Body |
|--------|------|-----------|------|
| POST | `/api/admin/users/bulk-delete` | admin:users:delete | `{"ids": [1, 2, 3]}` |
| POST | `/api/admin/users/bulk-disable` | admin:users:update | `{"ids": [1, 2, 3]}` |
| POST | `/api/admin/users/bulk-enable` | admin:users:update | `{"ids": [1, 2, 3]}` |
| POST | `/api/ocserv/users/bulk-delete` | ocserv:users:delete | `{"usernames": ["user1", "user2"]}` |
| POST | `/api/ocserv/users/bulk-lock` | ocserv:users:update | `{"usernames": ["user1", "user2"]}` |
| POST | `/api/ocserv/users/bulk-unlock` | ocserv:users:update | `{"usernames": ["user1", "user2"]}` |

### Response Shape (all bulk endpoints)

```json
{
  "total": 5,
  "succeeded": 4,
  "failed": 1,
  "errors": [
    {"id": "3", "error": "cannot delete last owner"}
  ]
}
```

The `id` field is the user ID (number as string) for admin users, or the username for VPN users.

### Behavior

- Each item is processed individually with its own security guard check
- Partial success is acceptable — some items may fail while others succeed
- Each successful operation is individually audited
- Failed operations are reported in the errors array
- Admin bulk-delete skips: self, last owner
- Admin bulk-disable skips: self
- VPN bulk operations process all items (no self-reference concept for VPN users)
- Maximum batch size: 100 items (return 400 if exceeded)

### Implementation

Add bulk methods to `AdminUserService`:
- `BulkDeleteAdminUsers(ctx, principal, ids []int64) BulkResult`
- `BulkDisableAdminUsers(ctx, principal, ids []int64) BulkResult`
- `BulkEnableAdminUsers(ctx, principal, ids []int64) BulkResult`

Add bulk methods to `OcservService`:
- `BulkDeleteOcservUsers(ctx, principal, usernames []string) BulkResult`
- `BulkLockOcservUsers(ctx, principal, usernames []string) BulkResult`
- `BulkUnlockOcservUsers(ctx, principal, usernames []string) BulkResult`

Define shared `BulkResult` type:
```go
type BulkResult struct {
    Total     int           `json:"total"`
    Succeeded int           `json:"succeeded"`
    Failed    int           `json:"failed"`
    Errors    []BulkError   `json:"errors"`
}

type BulkError struct {
    ID    string `json:"id"`
    Error string `json:"error"`
}
```

---

## Frontend

### Selection Mechanism

Both tabs get:
1. **Header checkbox** — toggles select-all for the current page (not all pages)
2. **Row checkboxes** — toggle individual items
3. Selection state stored as `Set<number>` (admin) or `Set<string>` (VPN)
4. Selection clears when switching tabs, changing filters, or after bulk action completes

### Bulk Action Toolbar

Appears as a sticky bar above the table when ≥1 item selected:

```
┌─────────────────────────────────────────────────────────────┐
│ ✓ 3 users selected    [Delete] [Disable] [Enable]  [✕ Clear]│
└─────────────────────────────────────────────────────────────┘
```

- For Admin tab: Delete (red), Disable, Enable buttons
- For VPN tab: Delete (red), Lock, Unlock buttons
- Clear button (✕) deselects all

### Bulk Confirmation Dialog

Before executing any bulk action, show confirmation:
- Title: "Confirm Bulk [Action]"
- Body: "Are you sure you want to [action] N users? This cannot be undone."
- For delete: extra warning styling (red)
- Cancel and Confirm buttons
- Loading state during execution

### After Completion

- Toast notification: "Bulk [action]: N succeeded, M failed"
- If errors: show expandable error details in toast or inline
- Refresh the user list
- Clear selection

### API Client Functions

Add to `frontend/src/lib/api.ts`:

```typescript
export type BulkResult = {
  total: number;
  succeeded: number;
  failed: number;
  errors: { id: string; error: string }[];
};

export function bulkDeleteAdminUsers(ids: number[]): Promise<BulkResult> { ... }
export function bulkDisableAdminUsers(ids: number[]): Promise<BulkResult> { ... }
export function bulkEnableAdminUsers(ids: number[]): Promise<BulkResult> { ... }
export function bulkDeleteVpnUsers(usernames: string[]): Promise<BulkResult> { ... }
export function bulkLockVpnUsers(usernames: string[]): Promise<BulkResult> { ... }
export function bulkUnlockVpnUsers(usernames: string[]): Promise<BulkResult> { ... }
```

---

## Security

- RBAC checked once at the endpoint level (same permission as single operation)
- Per-item guard checks prevent unsafe operations (delete self, last owner)
- Audit log records each affected user individually
- Maximum 100 items per request (prevents abuse)
- Owner-only: all bulk operations require owner role

---

## Testing

### Backend Tests
- Bulk delete admin: skips self, skips last owner, deletes rest
- Bulk disable admin: skips self, disables rest
- Bulk delete VPN: handles mix of existing and non-existing users
- Bulk lock VPN: handles already-locked users gracefully
- Max batch size enforcement (101 items → 400 error)

### Frontend
- Selection state management (select, deselect, select-all, clear)
- Toolbar visibility tied to selection count
- Confirmation dialog flow

---

## Out of Scope

- Cross-page selection (only current page)
- Undo for bulk operations
- Progress indicator for large batches (batch executes synchronously)
- Bulk create or bulk password reset
- VPN user origin tracking (all users are equal regardless of creation method)
