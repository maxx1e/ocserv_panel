# Config Management Frontend — Design Spec

**Date:** 2026-05-05
**Route:** `/config`
**Status:** Approved
**Depends on:** `2026-05-05-config-management-backend-design.md` (API contract must be stable)

---

## Overview

A three-tab WebUI page at `/config` that replaces the current placeholder. Lets owners safely view, edit, validate, diff, apply, and roll back the ocserv configuration file, and inspect read-only network prerequisites.

Viewers with `ocserv:config:read` see all tabs in read-only mode. The editor textarea and action buttons are only enabled for owners with `ocserv:config:edit`. Rollback buttons require `ocserv:config:rollback`.

---

## Architecture

Single `+page.svelte` with three tab components. All data flows through the existing `api.ts` client.

### New / Modified Files

```
frontend/src/routes/config/
└── +page.svelte              — replaces current EmptyState placeholder

frontend/src/lib/api.ts       — add config API types and functions
```

No new UI component library additions needed — uses existing `CardTable`, `EmptyState`, lucide-svelte icons, and Tailwind.

---

## API Types (additions to `api.ts`)

```typescript
export type ConfigIssue = {
  severity: 'error' | 'warning' | 'info';
  key: string;
  message: string;
  suggestion?: string;
};

export type ConfigReadResult = {
  content: string;
  path: string;
  lastModified: string;
  size: number;
  issues: ConfigIssue[];
};

export type ConfigValidateResult = {
  valid: boolean;
  issues: ConfigIssue[];
};

export type ConfigDiffResult = {
  diff: string;
};

export type ConfigApplyResult = {
  backupNumber: number;
  reloadSuccess: boolean;
  reloadError: string;
  issues: ConfigIssue[];
};

export type ConfigBackupInfo = {
  number: number;
  timestamp: string;
  size: number;
};

export type ConfigBackupsResult = {
  backups: ConfigBackupInfo[];
  maxBackups: number;
};

export type ConfigBackupContent = {
  content: string;
};

export type ConfigRollbackResult = {
  reloadSuccess: boolean;
  reloadError: string;
};

export type NetworkPrerequisiteCheck = {
  category: 'kernel' | 'firewall' | 'routing';
  name: string;
  status: 'configured' | 'missing' | 'partial' | 'unknown';
  expected: string;
  actual: string;
  suggestion: string;
};

export type NetworkPrerequisites = {
  items: NetworkPrerequisiteCheck[];
};
```

### API Functions

```typescript
export async function getConfig(): Promise<ConfigReadResult>
export async function validateConfig(content: string): Promise<ConfigValidateResult>
export async function diffConfig(content: string): Promise<ConfigDiffResult>
export async function applyConfig(content: string): Promise<ConfigApplyResult>
export async function getConfigBackups(): Promise<ConfigBackupsResult>
export async function getConfigBackup(n: number): Promise<ConfigBackupContent>
export async function diffConfigBackup(n: number): Promise<ConfigDiffResult>
export async function rollbackConfig(n: number): Promise<ConfigRollbackResult>
export async function getNetworkPrerequisites(): Promise<NetworkPrerequisites>
```

---

## Page Layout

**Route:** `/config` — owner-only in sidebar navigation (already configured).

```
┌─────────────────────────────────────────────────┐
│  Config  /etc/ocserv/ocserv.conf  ·  Modified: X │
│  [Editor]  [Backups]  [Network]                  │
├─────────────────────────────────────────────────┤
│  Tab content                                     │
└─────────────────────────────────────────────────┘
```

Top-level state: active tab, principal (for RBAC), loading/error states.

---

## Tab 1: Editor

### State

```typescript
let configContent = $state('');          // current live content from GET /api/config
let editContent = $state('');            // user's draft (starts equal to configContent)
let configMeta = $state<ConfigReadResult | null>(null);
let issues = $state<ConfigIssue[]>([]);  // validation issues for current editContent
let diffText = $state('');               // unified diff string
let showDiffModal = $state(false);
let showApplyConfirm = $state(false);
let applyResult = $state<ConfigApplyResult | null>(null);
let loading = $state(true);
let validating = $state(false);
let applying = $state(false);
let successBanner = $state('');
let errorBanner = $state('');
```

### Layout

```
┌────────────────────────────────────────────────────┐
│ Path: /etc/ocserv/ocserv.conf   Last modified: ...  │
│ Status: ● valid / ⚠ 2 warnings / ✕ 1 error         │
├────────────────────────────────────────────────────┤
│ [textarea — monospace, full-width, ~30 rows]        │
│                                                     │
├────────────────────────────────────────────────────┤
│ Validation results (if any):                        │
│   ✕ error   tcp-port: port 443 already in use       │
│             Suggestion: change tcp-port to 4443     │
│   ⚠ warning server-cert: relative path             │
├────────────────────────────────────────────────────┤
│              [Validate]  [Preview Changes]  [Apply] │
└────────────────────────────────────────────────────┘
```

### RBAC

- Textarea `disabled` and buttons hidden/disabled when user lacks `ocserv:config:edit`
- Read-only view still shows current config and issues

### Validate button

1. Call `validateConfig(editContent)`
2. Show issues inline — severity icon + message + suggestion
3. Show "✓ No issues found" if `valid: true`

### Preview Changes button

1. Call `diffConfig(editContent)`
2. Open diff modal:

```
┌─── Preview Changes ──────────────────────────────┐
│ --- ocserv.conf (current)                         │
│ +++ ocserv.conf (proposed)                        │
│ @@ -12,3 +12,3 @@                                 │
│ -tcp-port = 443                                   │
│ +tcp-port = 4443                                  │
│                                                   │
│            [Cancel]  [Apply Changes]              │
└───────────────────────────────────────────────────┘
```

Diff rendered as a `<pre>` block with line-level coloring:
- `-` lines: `bg-red-50 text-red-800`
- `+` lines: `bg-green-50 text-green-800`
- `@@` context lines: `text-slate-400`

### Apply button (and Apply Changes in diff modal)

1. If no diff was previewed first: call `diffConfig` first, open diff modal
2. After diff modal: show confirmation dialog:
   ```
   ⚠ Apply configuration?
   This will write the new config and reload ocserv.
   Active VPN sessions may be interrupted.
   [Cancel]  [Apply]
   ```
3. Call `applyConfig(editContent)`
4. On success:
   - If `reloadSuccess: true`: green banner "Config applied and ocserv reloaded (backup #N)"
   - If `reloadSuccess: false`: amber banner "Config written (backup #N) but reload failed: {reloadError}. You can retry by clicking Apply again or rollback."
5. On 422: display issues inline (validation failed after server-side re-check)
6. On 409: error banner "Config file is locked by another operation. Try again."

---

## Tab 2: Backups

### State

```typescript
let backups = $state<ConfigBackupInfo[]>([]);
let maxBackups = $state(10);
let selectedBackupN = $state<number | null>(null);
let backupContent = $state('');
let backupDiff = $state('');
let viewMode = $state<'content' | 'diff' | null>(null);
let rollbackPending = $state<number | null>(null);
let rollbackResult = $state<ConfigRollbackResult | null>(null);
let loadingBackups = $state(true);
let rolling = $state(false);
```

### Layout

```
┌──────────────────────────────────────────────────────────┐
│ #  │ Timestamp            │ Size    │ Actions             │
├──────────────────────────────────────────────────────────┤
│ 1  │ 2026-05-05 14:32     │ 2.0 KB  │ [View] [Diff] [↩]  │
│ 2  │ 2026-05-05 13:10     │ 1.9 KB  │ [View] [Diff] [↩]  │
│ ...                                                       │
└──────────────────────────────────────────────────────────┘

[Selected backup content / diff shown below table in a <pre> block]
```

- **View** — loads `getConfigBackup(n)`, shows read-only textarea
- **Diff** — loads `diffConfigBackup(n)`, shows colored unified diff
- **↩ Rollback** button only visible to users with `ocserv:config:rollback`

### Rollback confirmation

```
⚠ Rollback to backup #N?
This will restore the configuration from {timestamp}
and reload ocserv.
[Cancel]  [Rollback]
```

After rollback:
- Success: green banner + refresh backups list + refresh editor tab content
- Reload failed: amber banner like apply partial success
- 404: error banner "Backup #N not found"

---

## Tab 3: Network

### State

```typescript
let network = $state<NetworkPrerequisites | null>(null);
let loadingNetwork = $state(false);
let networkError = $state('');
```

### Layout

```
┌─── Kernel Parameters ─────────────────────────────────┐
│ ● IPv4 Forwarding     configured   expected: 1          │
│ ✕ IPv6 Forwarding     missing      expected: 1          │
│   Fix: sysctl -w net.ipv6.conf.all.forwarding=1        │
│   [Copy]                                               │
├─── Firewall ───────────────────────────────────────────┤
│ ● VPN Masquerade      configured                       │
│ ✕ VPN Forward         missing                          │
│   Fix: iptables -A FORWARD -i vpns+ -j ACCEPT          │
│   [Copy]                                               │
├─── Routing ────────────────────────────────────────────┘
│ ● No subnet conflict  configured                       │
│                              [Refresh]                 │
└────────────────────────────────────────────────────────┘
```

**Status icons:**
- `configured` → filled green circle (`●`)
- `missing` → red circle with X (`✕`)
- `partial` → amber half-filled circle (`◑`)
- `unknown` → grey circle (`○`)

Suggestion rows only appear when `suggestion` is non-empty. The `[Copy]` button uses `navigator.clipboard.writeText()`.

Refresh button re-calls `getNetworkPrerequisites()` and updates the display.

---

## RBAC Summary in UI

| User has | Editor textarea | Validate/Preview | Apply | Rollback buttons |
|----------|----------------|-----------------|-------|-----------------|
| `config:read` only | disabled | hidden | hidden | hidden |
| `config:edit` | enabled | visible | visible | hidden |
| `config:edit` + `config:rollback` | enabled | visible | visible | visible |

RBAC is read from `principal.roles` obtained via `getMe()` on mount.

For this panel, `owner` role maps to all three permissions; `viewer` maps to read-only.

---

## Error Handling

| Scenario | UI response |
|----------|-------------|
| Initial load fails | Error banner, retry button |
| 401 on any action | Redirect to `/login` |
| 403 | Error banner "You do not have permission for this action" |
| 409 (locked) | Error banner "Config is locked by another operation. Try again shortly." |
| 422 (validation) | Inline issues list |
| 503 (ocserv unavailable) | Warning banner "ocserv is not available — config can be edited but reload will not take effect" |
| Network error | Error banner with message |

---

## Security Notes

- Never display or log private key content (backend enforces this; frontend has no special handling needed)
- Confirmation dialogs required before Apply and Rollback
- Diff preview encouraged before Apply (apply without prior preview will open diff first)

---

## Out of Scope

- Syntax highlighting in textarea
- Guided/form-based config editing
- Firewall rule modification (read-only Network tab only)
- Import/export via file upload
