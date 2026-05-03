# Settings Tab UI Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the Settings tab from a vertical-stack layout to a two-column table layout with per-setting tooltips and decomposed type-specific components.

**Architecture:** Decompose the monolithic `SettingField.svelte` into individual type components (`SettingSelect`, `SettingText`, `SettingNumber`, `SettingPassword`, `SettingMultiCheckbox`, `SettingReadonly`, `SettingButton`), each rendered inside a shared `SettingRow.svelte` two-column layout. Add a `SettingTooltip.svelte` component for hover tooltips and a `descriptions.ts` map for frontend-only setting descriptions. `SettingField.svelte` becomes a thin dispatcher.

**Tech Stack:** SvelteKit (Svelte 5 runes), TypeScript, Tailwind CSS

---

## File Structure

| File | Responsibility |
|---|---|
| `frontend/src/lib/settings/descriptions.ts` | Frontend-only descriptions map keyed by setting key |
| `frontend/src/lib/components/ui/SettingTooltip.svelte` | Hover tooltip component (ⓘ icon + floating card) |
| `frontend/src/lib/components/ui/SettingRow.svelte` | Two-column row layout (label left, control right) |
| `frontend/src/lib/components/ui/SettingSelect.svelte` | Select dropdown setting type |
| `frontend/src/lib/components/ui/SettingText.svelte` | Text input setting type |
| `frontend/src/lib/components/ui/SettingNumber.svelte` | Number input setting type |
| `frontend/src/lib/components/ui/SettingPassword.svelte` | Password input with show/hide toggle |
| `frontend/src/lib/components/ui/SettingMultiCheckbox.svelte` | Multi-checkbox group setting type |
| `frontend/src/lib/components/ui/SettingReadonly.svelte` | Readonly display setting type |
| `frontend/src/lib/components/ui/SettingButton.svelte` | Action button setting type |
| `frontend/src/lib/components/ui/SettingToggle.svelte` | Modify existing — wrap in SettingRow |
| `frontend/src/lib/components/ui/SettingField.svelte` | Modify — becomes thin dispatcher |
| `frontend/src/lib/components/ui/CardSettings.svelte` | Minor — adjust spacing for row layout |

---

### Task 1: Create descriptions map

**Files:**
- Create: `frontend/src/lib/settings/descriptions.ts`

- [ ] **Step 1: Create the descriptions map**

Create `frontend/src/lib/settings/descriptions.ts` with a `Record<string, string>` containing a description for every setting key from the backend registry:

```typescript
export const settingDescriptions: Record<string, string> = {
  // General
  'general.app_name': 'The display name for this ocserv panel instance.',
  'general.timezone': 'Timezone used for timestamps and scheduled tasks.',
  'general.theme': 'UI color theme. "system" follows the operating system preference.',
  'general.language': 'Interface language. Currently only English is available.',

  // Installation
  'installation.config_path': 'Path to the main panel configuration file (TOML format).',
  'installation.data_dir': 'Directory for persistent application data and SQLite database.',
  'installation.log_dir': 'Directory where application log files are written.',
  'installation.ocserv_config_path': 'Path to the ocserv configuration file.',
  'installation.ocserv_socket_path': 'Path to the ocserv management socket for runtime control.',
  'installation.ocpasswd_path': 'Path to the ocpasswd file for password-based user authentication.',
  'installation.cert_dir': 'Directory containing TLS certificates for VPN users.',

  // Service
  'service.name': 'Systemd service name for the panel process.',
  'service.user': 'System user under which the panel service runs.',
  'service.group': 'System group under which the panel service runs.',
  'service.start_on_boot': 'Automatically start the panel service when the system boots.',
  'service.restart_on_failure': 'Automatically restart the service if it crashes.',
  'service.maintenance_mode': 'Enable maintenance mode to block user access during upgrades.',

  // Web Access
  'web.host': 'IP address the HTTP server listens on. Use 0.0.0.0 for all interfaces.',
  'web.port': 'TCP port for the HTTP server. Must be between 1 and 65535.',
  'web.base_url': 'Full base URL used to generate links and redirects.',
  'web.reverse_proxy': 'Enable when running behind HAProxy, nginx, or another reverse proxy.',
  'web.trusted_proxies': 'Comma-separated list of trusted reverse proxy IP addresses.',
  'web.https': 'Enable HTTPS with TLS certificate for the web server.',

  // Security
  'security.allowed_admin_ips': 'Comma-separated list of IP addresses allowed to access the admin panel. Empty allows all.',
  'security.tls_cert_path': 'Path to the TLS certificate file for HTTPS connections.',
  'security.tls_key_path': 'Path to the TLS private key file for HTTPS connections.',
  'security.encryption_key_path': 'Path to the encryption key used for protecting sensitive data at rest.',
  'security.csrf_enabled': 'Enable Cross-Site Request Forgery protection for all state-changing requests.',
  'security.security_headers': 'Add standard security headers (X-Frame-Options, CSP, etc.) to responses.',

  // Database
  'database.dsn': 'SQLite database connection string or file path.',
  'database.max_open_conns': 'Maximum number of simultaneous open database connections.',
  'database.max_idle_conns': 'Maximum number of idle database connections kept in the pool.',
  'database.conn_max_lifetime': 'Maximum time in seconds a database connection may be reused.',

  // Users & Permissions
  'users.default_role': 'Role assigned to newly registered users.',
  'users.allow_self_registration': 'Allow new users to register accounts without an invitation.',
  'users.max_users': 'Maximum number of user accounts. Set to 0 for unlimited.',
  'users.auto_disable_expired_certs': 'Automatically disable user accounts when their certificate expires.',

  // Password Policy
  'password_policy.min_length': 'Minimum number of characters required for a password.',
  'password_policy.require_uppercase': 'Require at least one uppercase letter in passwords.',
  'password_policy.require_lowercase': 'Require at least one lowercase letter in passwords.',
  'password_policy.require_digit': 'Require at least one digit in passwords.',
  'password_policy.require_special': 'Require at least one special character in passwords.',
  'password_policy.expiry_days': 'Number of days before passwords expire. Set to 0 for no expiry.',
  'password_policy.reuse_count': 'Number of previous passwords to prevent from being reused.',

  // Logs & Audit
  'logs.level': 'Minimum log level. Lower levels produce more output.',
  'logs.retention_days': 'Number of days to keep application log files before automatic cleanup.',
  'logs.audit_enabled': 'Record administrative actions to the audit log for traceability.',
  'logs.audit_retention_days': 'Number of days to keep audit log entries before automatic cleanup.',

  // Monitoring
  'monitoring.health_endpoint': 'Enable the /health HTTP endpoint for uptime checks.',
  'monitoring.metrics_endpoint': 'Enable the /metrics HTTP endpoint for monitoring dashboards.',
  'monitoring.dashboard_refresh_interval': 'How often (in seconds) the dashboard auto-refreshes its data.',

  // Backup & Restore
  'backup.schedule': 'How often automatic database backups are created.',
  'backup.destination': 'Directory path where backup files are stored.',
  'backup.retention_days': 'Number of days to keep backup files before automatic cleanup.',
  'backup.encryption': 'Encrypt backup files using the application encryption key.',

  // Notifications
  'notifications.enabled': 'Enable outgoing webhook notifications for system events.',
  'notifications.events': 'Select which system events trigger a notification.',
  'notifications.method': 'Method used to send notifications. Currently only webhooks are supported.',
  'notifications.webhook_url': 'URL to receive webhook POST requests when selected events occur.',
};
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/settings/descriptions.ts
git commit -m "feat(settings): add frontend-only setting descriptions map"
```

---

### Task 2: Create SettingTooltip component

**Files:**
- Create: `frontend/src/lib/components/ui/SettingTooltip.svelte`

- [ ] **Step 1: Create the tooltip component**

Create `frontend/src/lib/components/ui/SettingTooltip.svelte`:

```svelte
<script lang="ts">
  type Props = {
    description?: string;
  };

  let { description }: Props = $props();

  let showTooltip = $state(false);
  let hoverTimeout: ReturnType<typeof setTimeout> | null = null;

  function handleMouseEnter() {
    hoverTimeout = setTimeout(() => {
      showTooltip = true;
    }, 150);
  }

  function handleMouseLeave() {
    if (hoverTimeout) {
      clearTimeout(hoverTimeout);
      hoverTimeout = null;
    }
    showTooltip = false;
  }
</script>

{#if description}
  <span class="relative inline-flex items-center ml-1" onmouseenter={handleMouseEnter} onmouseleave={handleMouseLeave}>
    <svg
      class="w-3.5 h-3.5 text-slate-400 hover:text-slate-600 transition-colors cursor-help"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="2"
    >
      <circle cx="12" cy="12" r="10" />
      <path d="M12 16v-4" />
      <path d="M12 8h.01" />
    </svg>
    {#if showTooltip}
      <div
        class="absolute z-50 bottom-full left-1/2 -translate-x-1/2 mb-2 w-64 px-3 py-2 text-xs text-white bg-slate-800 rounded-lg shadow-lg"
        role="tooltip"
      >
        {description}
        <div class="absolute top-full left-1/2 -translate-x-1/2 -mt-1 w-2 h-2 bg-slate-800 rotate-45"></div>
      </div>
    {/if}
  </span>
{/if}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/components/ui/SettingTooltip.svelte
git commit -m "feat(settings): add SettingTooltip hover component"
```

---

### Task 3: Create SettingRow component

**Files:**
- Create: `frontend/src/lib/components/ui/SettingRow.svelte`

- [ ] **Step 1: Create the two-column row layout component**

Create `frontend/src/lib/components/ui/SettingRow.svelte`:

```svelte
<script lang="ts">
  import SettingTooltip from './SettingTooltip.svelte';

  type Props = {
    label: string;
    description?: string;
    requiresRestart?: boolean;
    isSaving?: boolean;
    children: import('svelte').Snippet;
  };

  let { label, description, requiresRestart = false, isSaving = false, children }: Props = $props();
</script>

<div class="grid grid-cols-1 md:grid-cols-[240px_1fr] gap-1 md:gap-4 py-3 border-b border-slate-100 last:border-0 items-center">
  <div class="flex items-center justify-end md:text-right">
    <span class="font-medium text-slate-700 text-sm">{label}</span>
    <SettingTooltip {description} />
    {#if requiresRestart}
      <span
        class="inline-flex items-center justify-center w-5 h-5 rounded-full bg-amber-100 text-amber-600 text-xs font-bold ml-1"
        title="Requires service restart"
      >&#x21BB;</span>
    {/if}
    {#if isSaving}
      <span class="text-xs text-slate-400 ml-2">Saving…</span>
    {/if}
  </div>
  <div class="flex items-center gap-2">
    {@render children()}
  </div>
</div>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/components/ui/SettingRow.svelte
git commit -m "feat(settings): add SettingRow two-column layout component"
```

---

### Task 4: Create SettingSelect component

**Files:**
- Create: `frontend/src/lib/components/ui/SettingSelect.svelte`

- [ ] **Step 1: Create the select dropdown component**

Create `frontend/src/lib/components/ui/SettingSelect.svelte`:

```svelte
<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<void>;
    savingKey: string | null;
  };

  let { setting, onUpdate, savingKey }: Props = $props();

  let editValue = $state(setting.value);
  let editing = $state(false);
  let error = $state('');
  const isSaving = $derived(savingKey === setting.key);

  function startEdit() {
    editValue = setting.value;
    editing = true;
    error = '';
  }

  async function save() {
    error = '';
    try {
      await onUpdate(setting.key, editValue);
      editing = false;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    }
  }

  function cancel() {
    editValue = setting.value;
    editing = false;
    error = '';
  }
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving}>
  {#if editing}
    <select
      bind:value={editValue}
      class="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 bg-white focus:border-slate-500 focus:ring-1 focus:ring-slate-500"
    >
      {#each (setting.options ?? []) as option}
        <option value={option}>{option}</option>
      {/each}
    </select>
    <button
      class="px-3 py-2 bg-slate-600 text-white text-sm font-medium rounded-lg hover:bg-slate-700 disabled:opacity-50 transition-colors"
      onclick={save}
      disabled={isSaving}
    >Save</button>
    <button
      class="px-3 py-2 border border-slate-300 text-slate-600 text-sm font-medium rounded-lg hover:bg-slate-50 transition-colors"
      onclick={cancel}
    >Cancel</button>
  {:else}
    <span class="text-sm text-slate-600">{setting.value || '—'}</span>
    <button
      class="px-3 py-1.5 text-sm font-medium text-slate-600 border border-slate-300 rounded-lg hover:bg-slate-50 transition-colors"
      onclick={startEdit}
    >Edit</button>
  {/if}
  {#if error}
    <span class="text-sm text-red-600">{error}</span>
  {/if}
</SettingRow>

<script lang="ts">
  import { settingDescriptions } from '$lib/settings/descriptions';
</script>
```

Wait — Svelte 5 doesn't allow two `<script>` blocks. Let me fix this properly. The import of `settingDescriptions` must be in the single script block.

- [ ] **Step 2: Rewrite with correct single script block**

Create `frontend/src/lib/components/ui/SettingSelect.svelte`:

```svelte
<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<void>;
    savingKey: string | null;
  };

  let { setting, onUpdate, savingKey }: Props = $props();

  let editValue = $state(setting.value);
  let editing = $state(false);
  let error = $state('');
  const isSaving = $derived(savingKey === setting.key);

  function startEdit() {
    editValue = setting.value;
    editing = true;
    error = '';
  }

  async function save() {
    error = '';
    try {
      await onUpdate(setting.key, editValue);
      editing = false;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    }
  }

  function cancel() {
    editValue = setting.value;
    editing = false;
    error = '';
  }
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving}>
  {#if editing}
    <select
      bind:value={editValue}
      class="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 bg-white focus:border-slate-500 focus:ring-1 focus:ring-slate-500"
    >
      {#each (setting.options ?? []) as option}
        <option value={option}>{option}</option>
      {/each}
    </select>
    <button
      class="px-3 py-2 bg-slate-600 text-white text-sm font-medium rounded-lg hover:bg-slate-700 disabled:opacity-50 transition-colors"
      onclick={save}
      disabled={isSaving}
    >Save</button>
    <button
      class="px-3 py-2 border border-slate-300 text-slate-600 text-sm font-medium rounded-lg hover:bg-slate-50 transition-colors"
      onclick={cancel}
    >Cancel</button>
  {:else}
    <span class="text-sm text-slate-600">{setting.value || '—'}</span>
    <button
      class="px-3 py-1.5 text-sm font-medium text-slate-600 border border-slate-300 rounded-lg hover:bg-slate-50 transition-colors"
      onclick={startEdit}
    >Edit</button>
  {/if}
  {#if error}
    <span class="text-sm text-red-600">{error}</span>
  {/if}
</SettingRow>
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/ui/SettingSelect.svelte
git commit -m "feat(settings): add SettingSelect component"
```

---

### Task 5: Create SettingText component

**Files:**
- Create: `frontend/src/lib/components/ui/SettingText.svelte`

- [ ] **Step 1: Create the text input component**

Create `frontend/src/lib/components/ui/SettingText.svelte`:

```svelte
<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<void>;
    savingKey: string | null;
  };

  let { setting, onUpdate, savingKey }: Props = $props();

  let editValue = $state(setting.value);
  let editing = $state(false);
  let error = $state('');
  const isSaving = $derived(savingKey === setting.key);

  function startEdit() {
    editValue = setting.value;
    editing = true;
    error = '';
  }

  async function save() {
    error = '';
    try {
      await onUpdate(setting.key, editValue);
      editing = false;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    }
  }

  function cancel() {
    editValue = setting.value;
    editing = false;
    error = '';
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') save();
    if (e.key === 'Escape') cancel();
  }
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving}>
  {#if editing}
    <input
      type="text"
      bind:value={editValue}
      onkeydown={handleKeydown}
      class="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 bg-white focus:border-slate-500 focus:ring-1 focus:ring-slate-500 w-full max-w-md"
    />
    <button
      class="px-3 py-2 bg-slate-600 text-white text-sm font-medium rounded-lg hover:bg-slate-700 disabled:opacity-50 transition-colors"
      onclick={save}
      disabled={isSaving}
    >Save</button>
    <button
      class="px-3 py-2 border border-slate-300 text-slate-600 text-sm font-medium rounded-lg hover:bg-slate-50 transition-colors"
      onclick={cancel}
    >Cancel</button>
  {:else}
    <span class="text-sm text-slate-600">{setting.value || '—'}</span>
    <button
      class="px-3 py-1.5 text-sm font-medium text-slate-600 border border-slate-300 rounded-lg hover:bg-slate-50 transition-colors"
      onclick={startEdit}
    >Edit</button>
  {/if}
  {#if error}
    <span class="text-sm text-red-600">{error}</span>
  {/if}
</SettingRow>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/components/ui/SettingText.svelte
git commit -m "feat(settings): add SettingText component"
```

---

### Task 6: Create SettingNumber component

**Files:**
- Create: `frontend/src/lib/components/ui/SettingNumber.svelte`

- [ ] **Step 1: Create the number input component**

Create `frontend/src/lib/components/ui/SettingNumber.svelte`:

```svelte
<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<void>;
    savingKey: string | null;
  };

  let { setting, onUpdate, savingKey }: Props = $props();

  let editValue = $state(setting.value);
  let editing = $state(false);
  let error = $state('');
  const isSaving = $derived(savingKey === setting.key);

  function startEdit() {
    editValue = setting.value;
    editing = true;
    error = '';
  }

  async function save() {
    error = '';
    try {
      await onUpdate(setting.key, editValue);
      editing = false;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    }
  }

  function cancel() {
    editValue = setting.value;
    editing = false;
    error = '';
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') save();
    if (e.key === 'Escape') cancel();
  }
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving}>
  {#if editing}
    <input
      type="number"
      bind:value={editValue}
      onkeydown={handleKeydown}
      class="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 bg-white focus:border-slate-500 focus:ring-1 focus:ring-slate-500 w-32"
    />
    <button
      class="px-3 py-2 bg-slate-600 text-white text-sm font-medium rounded-lg hover:bg-slate-700 disabled:opacity-50 transition-colors"
      onclick={save}
      disabled={isSaving}
    >Save</button>
    <button
      class="px-3 py-2 border border-slate-300 text-slate-600 text-sm font-medium rounded-lg hover:bg-slate-50 transition-colors"
      onclick={cancel}
    >Cancel</button>
  {:else}
    <span class="text-sm text-slate-600">{setting.value || '—'}</span>
    <button
      class="px-3 py-1.5 text-sm font-medium text-slate-600 border border-slate-300 rounded-lg hover:bg-slate-50 transition-colors"
      onclick={startEdit}
    >Edit</button>
  {/if}
  {#if error}
    <span class="text-sm text-red-600">{error}</span>
  {/if}
</SettingRow>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/components/ui/SettingNumber.svelte
git commit -m "feat(settings): add SettingNumber component"
```

---

### Task 7: Create SettingPassword component

**Files:**
- Create: `frontend/src/lib/components/ui/SettingPassword.svelte`

- [ ] **Step 1: Create the password input component**

Create `frontend/src/lib/components/ui/SettingPassword.svelte`:

```svelte
<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<void>;
    savingKey: string | null;
  };

  let { setting, onUpdate, savingKey }: Props = $props();

  let editValue = $state('');
  let showPassword = $state(false);
  let editing = $state(false);
  let error = $state('');
  const isSaving = $derived(savingKey === setting.key);

  function startEdit() {
    editValue = '';
    editing = true;
    error = '';
  }

  async function save() {
    error = '';
    try {
      await onUpdate(setting.key, editValue);
      editing = false;
      showPassword = false;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    }
  }

  function cancel() {
    editValue = '';
    editing = false;
    showPassword = false;
    error = '';
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') save();
    if (e.key === 'Escape') cancel();
  }
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving}>
  {#if editing}
    <input
      type={showPassword ? 'text' : 'password'}
      bind:value={editValue}
      onkeydown={handleKeydown}
      class="rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 bg-white focus:border-slate-500 focus:ring-1 focus:ring-slate-500 w-full max-w-md"
      placeholder="Enter new value"
    />
    <button
      class="px-3 py-2 text-sm font-medium text-slate-600 border border-slate-300 rounded-lg hover:bg-slate-50 transition-colors"
      onclick={() => (showPassword = !showPassword)}
      type="button"
    >
      {showPassword ? 'Hide' : 'Show'}
    </button>
    <button
      class="px-3 py-2 bg-slate-600 text-white text-sm font-medium rounded-lg hover:bg-slate-700 disabled:opacity-50 transition-colors"
      onclick={save}
      disabled={isSaving}
    >Save</button>
    <button
      class="px-3 py-2 border border-slate-300 text-slate-600 text-sm font-medium rounded-lg hover:bg-slate-50 transition-colors"
      onclick={cancel}
    >Cancel</button>
  {:else}
    <span class="text-sm text-slate-500">••••••••</span>
    <button
      class="px-3 py-1.5 text-sm font-medium text-slate-600 border border-slate-300 rounded-lg hover:bg-slate-50 transition-colors"
      onclick={startEdit}
    >Edit</button>
  {/if}
  {#if error}
    <span class="text-sm text-red-600">{error}</span>
  {/if}
</SettingRow>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/components/ui/SettingPassword.svelte
git commit -m "feat(settings): add SettingPassword component"
```

---

### Task 8: Create SettingMultiCheckbox component

**Files:**
- Create: `frontend/src/lib/components/ui/SettingMultiCheckbox.svelte`

- [ ] **Step 1: Create the multi-checkbox component**

Create `frontend/src/lib/components/ui/SettingMultiCheckbox.svelte`:

```svelte
<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<void>;
    savingKey: string | null;
  };

  let { setting, onUpdate, savingKey }: Props = $props();

  let checkboxValues = $state<string[]>(setting.value ? setting.value.split(',') : []);
  let error = $state('');
  const isSaving = $derived(savingKey === setting.key);
  const checkboxChanged = $derived(checkboxValues.join(',') !== (setting.value || ''));

  function toggleCheckbox(option: string) {
    if (checkboxValues.includes(option)) {
      checkboxValues = checkboxValues.filter((v) => v !== option);
    } else {
      checkboxValues = [...checkboxValues, option];
    }
  }

  async function saveCheckboxes() {
    error = '';
    try {
      await onUpdate(setting.key, checkboxValues.join(','));
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save';
    }
  }

  function cancelCheckboxes() {
    checkboxValues = setting.value ? setting.value.split(',') : [];
    error = '';
  }
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving}>
  <div class="flex flex-col gap-2">
    {#each (setting.options ?? []) as option}
      <label class="flex items-center gap-2 text-sm text-slate-700 cursor-pointer">
        <input
          type="checkbox"
          checked={checkboxValues.includes(option)}
          onchange={() => toggleCheckbox(option)}
          class="rounded border-slate-300 text-slate-600 focus:ring-slate-500"
        />
        {option}
      </label>
    {/each}
    {#if checkboxChanged}
      <div class="flex items-center gap-2 mt-2">
        <button
          class="px-3 py-2 bg-slate-600 text-white text-sm font-medium rounded-lg hover:bg-slate-700 disabled:opacity-50 transition-colors"
          onclick={saveCheckboxes}
          disabled={isSaving}
        >Save</button>
        <button
          class="px-3 py-2 border border-slate-300 text-slate-600 text-sm font-medium rounded-lg hover:bg-slate-50 transition-colors"
          onclick={cancelCheckboxes}
        >Cancel</button>
      </div>
    {/if}
  </div>
  {#if error}
    <span class="text-sm text-red-600">{error}</span>
  {/if}
</SettingRow>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/components/ui/SettingMultiCheckbox.svelte
git commit -m "feat(settings): add SettingMultiCheckbox component"
```

---

### Task 9: Create SettingReadonly component

**Files:**
- Create: `frontend/src/lib/components/ui/SettingReadonly.svelte`

- [ ] **Step 1: Create the readonly display component**

Create `frontend/src/lib/components/ui/SettingReadonly.svelte`:

```svelte
<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
  };

  let { setting }: Props = $props();
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart}>
  <span class="text-sm text-slate-600">{setting.value || '—'}</span>
</SettingRow>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/components/ui/SettingReadonly.svelte
git commit -m "feat(settings): add SettingReadonly component"
```

---

### Task 10: Create SettingButton component

**Files:**
- Create: `frontend/src/lib/components/ui/SettingButton.svelte`

- [ ] **Step 1: Create the action button component**

Create `frontend/src/lib/components/ui/SettingButton.svelte`:

```svelte
<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<void>;
    savingKey: string | null;
  };

  let { setting, onUpdate, savingKey }: Props = $props();
  const isSaving = $derived(savingKey === setting.key);
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving}>
  <button
    class="px-4 py-2 bg-slate-600 text-white text-sm font-medium rounded-lg hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
    onclick={() => onUpdate(setting.key, '')}
    disabled={isSaving}
  >
    {setting.label}
  </button>
</SettingRow>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/components/ui/SettingButton.svelte
git commit -m "feat(settings): add SettingButton component"
```

---

### Task 11: Modify SettingToggle to use SettingRow

**Files:**
- Modify: `frontend/src/lib/components/ui/SettingToggle.svelte`

- [ ] **Step 1: Update SettingToggle to wrap in SettingRow**

Replace the contents of `frontend/src/lib/components/ui/SettingToggle.svelte` with:

```svelte
<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<void>;
    savingKey: string | null;
  };

  let { setting, onUpdate, savingKey }: Props = $props();
  const isSaving = $derived(savingKey === setting.key);

  async function toggleChange(newValue: boolean) {
    try {
      await onUpdate(setting.key, String(newValue));
    } catch {
      // Error handled by parent
    }
  }
</script>

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving}>
  <div class="flex items-center gap-3">
    <button
      class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors {setting.value === 'true' ? 'bg-slate-600' : 'bg-slate-300'} {isSaving ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}"
      onclick={() => toggleChange(setting.value !== 'true')}
      disabled={isSaving}
      role="switch"
      aria-checked={setting.value === 'true'}
      aria-label={setting.label}
      type="button"
    >
      <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform {setting.value === 'true' ? 'translate-x-6' : 'translate-x-1'}"></span>
    </button>
    <span class="text-sm text-slate-500">{setting.value === 'true' ? 'Enabled' : 'Disabled'}</span>
  </div>
</SettingRow>
```

Note: The old `SettingToggle.svelte` accepted `checked`, `onchange`, `disabled`, `label` props. The new version accepts `setting`, `onUpdate`, `savingKey` — matching the interface of all other type components. This means `SettingField.svelte` will no longer need to call `SettingToggle` with the old interface.

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/components/ui/SettingToggle.svelte
git commit -m "feat(settings): update SettingToggle to use SettingRow layout"
```

---

### Task 12: Rewrite SettingField as thin dispatcher

**Files:**
- Modify: `frontend/src/lib/components/ui/SettingField.svelte`

- [ ] **Step 1: Replace SettingField with thin dispatcher**

Replace the entire contents of `frontend/src/lib/components/ui/SettingField.svelte` with:

```svelte
<script lang="ts">
  import type { SettingDef } from '$lib/api';
  import SettingToggle from './SettingToggle.svelte';
  import SettingSelect from './SettingSelect.svelte';
  import SettingText from './SettingText.svelte';
  import SettingNumber from './SettingNumber.svelte';
  import SettingPassword from './SettingPassword.svelte';
  import SettingMultiCheckbox from './SettingMultiCheckbox.svelte';
  import SettingReadonly from './SettingReadonly.svelte';
  import SettingButton from './SettingButton.svelte';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<void>;
    savingKey: string | null;
  };

  let { setting, onUpdate, savingKey }: Props = $props();
</script>

{#if setting.type === 'toggle'}
  <SettingToggle {setting} {onUpdate} {savingKey} />
{:else if setting.type === 'select'}
  <SettingSelect {setting} {onUpdate} {savingKey} />
{:else if setting.type === 'text'}
  <SettingText {setting} {onUpdate} {savingKey} />
{:else if setting.type === 'number'}
  <SettingNumber {setting} {onUpdate} {savingKey} />
{:else if setting.type === 'password'}
  <SettingPassword {setting} {onUpdate} {savingKey} />
{:else if setting.type === 'multi-checkbox'}
  <SettingMultiCheckbox {setting} {onUpdate} {savingKey} />
{:else if setting.type === 'readonly'}
  <SettingReadonly {setting} />
{:else if setting.type === 'button'}
  <SettingButton {setting} {onUpdate} {savingKey} />
{/if}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/components/ui/SettingField.svelte
git commit -m "feat(settings): rewrite SettingField as thin type dispatcher"
```

---

### Task 13: Update CardSettings spacing

**Files:**
- Modify: `frontend/src/lib/components/ui/CardSettings.svelte`

- [ ] **Step 1: Update CardSettings to remove extra spacing**

The current `CardSettings.svelte` uses `space-y-4` for the children slot. Since `SettingRow` now has its own border separators and padding, we should remove the extra spacing. Update the `<div class="mt-4 space-y-4">` to `<div class="mt-4">`:

Replace in `frontend/src/lib/components/ui/CardSettings.svelte`:

```svelte
  <div class="mt-4">
    {@render children()}
  </div>
```

(Only the class changes from `mt-4 space-y-4` to `mt-4`)

- [ ] **Step 2: Commit**

```bash
git add frontend/src/lib/components/ui/CardSettings.svelte
git commit -m "feat(settings): remove redundant spacing from CardSettings for row layout"
```

---

### Task 14: Verify and test

- [ ] **Step 1: Run the frontend dev server and verify the Settings page loads**

```bash
cd frontend && npm run dev
```

Open the Settings page in a browser and verify:
- All 12 sections render correctly
- Two-column layout appears on desktop (label left, control right)
- Layout stacks vertically on mobile (<768px)
- ⓘ tooltip icons appear next to labels on hover
- Tooltips show descriptions with 150ms delay
- Restart indicator (↻) appears on settings that require restart
- All setting types work: toggle, select, text, number, password, multi-checkbox, readonly, button
- Edit/Save/Cancel flows work for text, number, password, and select types
- Saving indicator appears next to the label when saving

- [ ] **Step 2: Run the frontend type check**

```bash
cd frontend && npm run check
```

Expected: No type errors.

- [ ] **Step 3: Run the frontend linter**

```bash
cd frontend && npm run lint
```

Expected: No lint errors.

- [ ] **Step 4: Commit any fixes if needed**

If any issues were found and fixed during verification, commit them:

```bash
git add -A
git commit -m "fix(settings): fix issues found during verification"
```

---

## Self-Review

**Spec coverage check:**
- ✅ Frontend-only descriptions map → Task 1
- ✅ SettingTooltip hover component → Task 2
- ✅ SettingRow two-column layout → Task 3
- ✅ Type-specific components (all 8 types) → Tasks 4-10
- ✅ SettingToggle modified to use SettingRow → Task 11
- ✅ SettingField becomes thin dispatcher → Task 12
- ✅ CardSettings spacing update → Task 13
- ✅ Responsive behavior (grid → stack) → Task 3 (SettingRow)
- ✅ No backend changes → Confirmed, all tasks are frontend-only

**Placeholder scan:** No TBDs, TODOs, or vague steps found.

**Type consistency check:** All type components use the same `Props` interface pattern (`setting: SettingDef`, `onUpdate: (key: string, value: string) => Promise<void>`, `savingKey: string | null`), except `SettingReadonly` which only needs `setting` since it has no save action. All import `settingDescriptions` from `$lib/settings/descriptions`. All use `SettingRow` as the wrapper.