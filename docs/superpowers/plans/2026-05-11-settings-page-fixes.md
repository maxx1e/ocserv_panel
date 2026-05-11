# Settings Page Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix six concrete bugs in the settings page: implement real panel restart, fix MonitoringConfig bool→string type mismatch, remove misleading `service.restart_on_failure` setting, convert SettingText/SettingSelect to inline click-to-edit, add "not yet implemented" notice to Backup/Notifications sections, and fix SettingTooltip accessibility.

**Architecture:** Backend changes are additive (new `RestartPanelService` method on existing `OcservService`, config struct field type fix, registry entry removal). Frontend changes are surgical edits to five Svelte components plus the settings page. No new files needed.

**Tech Stack:** Go 1.25, SvelteKit 5 + Svelte 5 runes, Tailwind CSS v4. Tests use stdlib `testing` (no testify). Run backend tests with `cd backend && go test ./...`. Run frontend type checks with `cd frontend && npx svelte-check`.

---

## File Map

| File | Change |
|------|--------|
| `backend/internal/app/ocserv_service.go` | Add `RestartPanelService` method |
| `backend/internal/app/ocserv_service_test.go` | Add 3 tests for `RestartPanelService` |
| `backend/internal/server/settings_handlers.go` | Wire `handleRestart` to `ocservService.RestartPanelService` |
| `backend/internal/config/config.go` | `MonitoringConfig.HealthEndpoint/MetricsEndpoint` bool → string |
| `backend/internal/app/settings_config_apply.go` | Remove `parseBool` for those two keys |
| `backend/internal/app/settings_config_apply_test.go` | Remove monitoring.health_endpoint/metrics_endpoint from coverage test table |
| `backend/internal/app/settings_registry.go` | Remove `service.restart_on_failure` entry |
| `frontend/src/lib/components/ui/SettingText.svelte` | Inline click-to-edit (remove Edit button) |
| `frontend/src/lib/components/ui/SettingSelect.svelte` | Inline click-to-edit (remove Edit button) |
| `frontend/src/lib/components/ui/CardSettings.svelte` | Add `stub` prop + notice banner |
| `frontend/src/routes/settings/+page.svelte` | Pass `stub` for backup/notifications sections |
| `frontend/src/lib/components/ui/SettingTooltip.svelte` | Fix role, tabindex, onfocus/onblur |

---

## Task 1: Add `RestartPanelService` to `OcservService` with tests

This wires the "Restart Now" button to actually restart the panel service via systemd. The existing `OcservService` already has a `svc service.ServiceManager` field and a `requirePerm` helper — `RestartPanelService` follows the exact same pattern as `StartService`/`StopService`.

**Files:**
- Modify: `backend/internal/app/ocserv_service.go`
- Modify: `backend/internal/app/ocserv_service_test.go`

- [ ] **Step 1: Write three failing tests**

Add to the end of `backend/internal/app/ocserv_service_test.go`:

```go
func TestRestartPanelServiceDeniedForUnauthenticated(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newTestOcservService(t, service.ServiceActive)
	err := svc.RestartPanelService(context.Background(), auth.Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestRestartPanelServiceDeniedForViewer(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newTestOcservService(t, service.ServiceActive)
	err := svc.RestartPanelService(context.Background(), viewerPrincipal())
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestRestartPanelServiceAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, _, fakeSvc := newTestOcservService(t, service.ServiceActive)
	if err := svc.RestartPanelService(context.Background(), ownerPrincipal()); err != nil {
		t.Fatalf("expected owner to restart panel, got %v", err)
	}
	if fakeSvc.RestartCount() != 1 {
		t.Fatalf("expected 1 restart call, got %d", fakeSvc.RestartCount())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/app/ -run TestRestartPanelService -v
```

Expected: FAIL — `svc.RestartPanelService undefined`

- [ ] **Step 3: Implement `RestartPanelService`**

Add this method to `backend/internal/app/ocserv_service.go` after `RestartService` (around line 117):

```go
// RestartPanelService restarts the panel service itself via systemd.
// After this call the process will be signalled to stop; systemd will restart it.
func (s *OcservService) RestartPanelService(ctx context.Context, principal auth.Principal) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionSettingsRestart, "settings.restart"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.svc.Restart(ctx); err != nil {
		_ = s.record(ctx, &actorID, "settings.restart", "service", "", audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("restart panel service: %w", err)
	}
	_ = s.record(ctx, &actorID, "settings.restart", "service", "", audit.OutcomeSuccess, "panel service restart initiated", nil)
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/app/ -run TestRestartPanelService -v
```

Expected: all three PASS

- [ ] **Step 5: Commit**

```bash
cd /opt/ocserv_panel
git add backend/internal/app/ocserv_service.go backend/internal/app/ocserv_service_test.go
git commit -m "feat(app): add RestartPanelService to OcservService"
```

---

## Task 2: Wire `handleRestart` to `RestartPanelService`

The handler currently returns a 200 OK stub. Replace it with a real call.

**Files:**
- Modify: `backend/internal/server/settings_handlers.go`

- [ ] **Step 1: Replace the stub in `handleRestart`**

In `backend/internal/server/settings_handlers.go`, replace the `handleRestart` function body:

Old content (lines 73–90):
```go
func (a *App) handleRestart(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	if !a.authorizer.Allowed(principalRoles(principal), rbac.PermissionSettingsRestart) {
		writeError(w, http.StatusForbidden, "permission denied")
		return
	}
	// Placeholder: actual restart will be wired in a later task.
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "restart not yet implemented",
	})
}
```

New content:
```go
func (a *App) handleRestart(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	if err := a.ocservService.RestartPanelService(r.Context(), principal); err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "restart initiated",
	})
}
```

Note: The `rbac` import in `settings_handlers.go` was only used for the removed permission check. After this edit, check that `rbac` is still imported elsewhere in the file — if not, remove it from the import block.

- [ ] **Step 2: Build to verify no compile errors**

```bash
cd /opt/ocserv_panel/backend && go build ./...
```

Expected: no errors

- [ ] **Step 3: Run full backend test suite**

```bash
cd /opt/ocserv_panel/backend && go test ./...
```

Expected: all packages PASS

- [ ] **Step 4: Commit**

```bash
cd /opt/ocserv_panel
git add backend/internal/server/settings_handlers.go
git commit -m "feat(server): wire handleRestart to ocservService.RestartPanelService"
```

---

## Task 3: Fix `MonitoringConfig` bool → string type mismatch

`HealthEndpoint` and `MetricsEndpoint` are `bool` in the config struct but are treated as informational readonly strings in the settings registry. Change them to `string`.

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/app/settings_config_apply.go`
- Modify: `backend/internal/app/settings_config_apply_test.go`

- [ ] **Step 1: Update `MonitoringConfig` in `config.go`**

In `backend/internal/config/config.go`, change these two fields in `MonitoringConfig`:

```go
// Before:
HealthEndpoint  bool   `toml:"health_endpoint"`
MetricsEndpoint bool   `toml:"metrics_endpoint"`

// After:
HealthEndpoint  string `toml:"health_endpoint"`
MetricsEndpoint string `toml:"metrics_endpoint"`
```

Update `Defaults()` in the same file — change the `MonitoringConfig` defaults block:

```go
// Before:
Monitoring: MonitoringConfig{
    HealthEndpoint:           true,
    MetricsEndpoint:          false,
    DashboardRefreshInterval: 15,
    LogFileReadEnabled:       true,
    NetworkInterface:         "",
    VPNInterface:             "",
    CollectorInterval:        15,
},

// After:
Monitoring: MonitoringConfig{
    HealthEndpoint:           "true",
    MetricsEndpoint:          "false",
    DashboardRefreshInterval: 15,
    LogFileReadEnabled:       true,
    NetworkInterface:         "",
    VPNInterface:             "",
    CollectorInterval:        15,
},
```

- [ ] **Step 2: Update `applySettingToConfig` in `settings_config_apply.go`**

In `backend/internal/app/settings_config_apply.go`, replace the two cases that call `parseBool()` for monitoring endpoints:

```go
// Before:
case "monitoring.health_endpoint":
    b, err := parseBool(value)
    if err != nil {
        return fmt.Errorf("%w: monitoring.health_endpoint must be true or false", ErrInvalidInput)
    }
    cfg.Monitoring.HealthEndpoint = b
case "monitoring.metrics_endpoint":
    b, err := parseBool(value)
    if err != nil {
        return fmt.Errorf("%w: monitoring.metrics_endpoint must be true or false", ErrInvalidInput)
    }
    cfg.Monitoring.MetricsEndpoint = b

// After:
case "monitoring.health_endpoint":
    cfg.Monitoring.HealthEndpoint = value
case "monitoring.metrics_endpoint":
    cfg.Monitoring.MetricsEndpoint = value
```

- [ ] **Step 3: Update coverage test in `settings_config_apply_test.go`**

The test `TestConfigValueForKeyAllConfigSourceKeys` has a table of keys that are applied then read back. The monitoring endpoint readonly keys are not in that table (they are excluded from the loop by type check), so no change is needed there.

However, `TestConfigValueForKeyCoversAllRegistryConfigSourceKeys` skips `SettingTypeReadonly` keys — `monitoring.health_endpoint` and `monitoring.metrics_endpoint` are readonly, so they are already excluded. No test change is needed.

Build to confirm:

```bash
cd /opt/ocserv_panel/backend && go build ./...
```

Expected: no errors

- [ ] **Step 4: Run full backend test suite**

```bash
cd /opt/ocserv_panel/backend && go test ./...
```

Expected: all packages PASS

- [ ] **Step 5: Commit**

```bash
cd /opt/ocserv_panel
git add backend/internal/config/config.go backend/internal/app/settings_config_apply.go
git commit -m "fix(config): change MonitoringConfig.HealthEndpoint/MetricsEndpoint from bool to string"
```

---

## Task 4: Remove `service.restart_on_failure` from registry

This setting saves to SQLite but has no effect — restart-on-failure is controlled by the systemd unit file. Remove it to avoid misleading users.

**Files:**
- Modify: `backend/internal/app/settings_registry.go`

- [ ] **Step 1: Remove the entry from `AllSettingDefs()`**

In `backend/internal/app/settings_registry.go`, find and delete this line in `AllSettingDefs()`:

```go
{Key: "service.restart_on_failure", Label: "Restart on Failure", Section: "service", Type: SettingTypeToggle, Source: SettingSourceDatabase, RequiresRestart: false, Default: "true", SortOrder: 5},
```

After deleting it, you'll notice that the remaining service settings have SortOrder values 1,2,3,4,6,7. Renumber to 1–6 for cleanliness (edit SortOrder values for `service.maintenance_mode` from 6 → 5 and `service.systemd_unit_name` from 7 → 6):

```go
{Key: "service.maintenance_mode", Label: "Maintenance Mode", Section: "service", Type: SettingTypeToggle, Source: SettingSourceConfig, RequiresRestart: true, Default: "false", SortOrder: 5},
{Key: "service.systemd_unit_name", Label: "Systemd Unit Name", Section: "service", Type: SettingTypeText, Source: SettingSourceConfig, RequiresRestart: true, Default: "ocserv", SortOrder: 6},
```

- [ ] **Step 2: Run backend tests**

```bash
cd /opt/ocserv_panel/backend && go test ./...
```

Expected: all packages PASS

- [ ] **Step 3: Commit**

```bash
cd /opt/ocserv_panel
git add backend/internal/app/settings_registry.go
git commit -m "fix(settings): remove misleading service.restart_on_failure setting"
```

---

## Task 5: Inline click-to-edit for `SettingText`

Replace the "Edit" button pattern with a clickable value span that enters edit mode directly.

**Files:**
- Modify: `frontend/src/lib/components/ui/SettingText.svelte`

- [ ] **Step 1: Replace `SettingText.svelte`**

Replace the entire file content of `frontend/src/lib/components/ui/SettingText.svelte`:

```svelte
<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<SettingDef>;
    savingKey: string | null;
    disabled?: boolean;
  };

  let { setting, onUpdate, savingKey, disabled = false }: Props = $props();

  let editValue = $state('');
  let editing = $state(false);
  let error = $state('');
  let savedPending = $state(false);
  let savedSuccess = $state(false);
  let successTimer: ReturnType<typeof setTimeout> | null = null;
  const isSaving = $derived(savingKey === setting.key);

  function startEdit() {
    if (disabled) return;
    editValue = setting.value;
    editing = true;
    error = '';
  }

  async function save() {
    savedPending = false;
    savedSuccess = false;
    error = '';
    try {
      const result = await onUpdate(setting.key, editValue);
      if (result.requires_restart) {
        savedPending = true;
      } else {
        savedSuccess = true;
        if (successTimer) clearTimeout(successTimer);
        successTimer = setTimeout(() => { savedSuccess = false; }, 2000);
      }
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

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving} {savedPending} {savedSuccess}>
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
    <!-- Clicking the value enters edit mode directly -->
    <span
      class="text-sm {disabled ? 'text-slate-400 cursor-not-allowed' : 'text-slate-600 cursor-pointer hover:text-slate-900 hover:underline'}"
      onclick={startEdit}
      role="button"
      tabindex={disabled ? -1 : 0}
      onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') startEdit(); }}
      aria-label="Edit {setting.label}"
    >{setting.value || '—'}</span>
  {/if}
  {#if error}
    <span class="text-sm text-red-600 max-w-md break-words">{error}</span>
  {/if}
</SettingRow>
```

- [ ] **Step 2: Run frontend type check**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-check 2>&1 | tail -5
```

Expected: `0 ERRORS`

- [ ] **Step 3: Commit**

```bash
cd /opt/ocserv_panel
git add frontend/src/lib/components/ui/SettingText.svelte
git commit -m "feat(frontend): inline click-to-edit for SettingText"
```

---

## Task 6: Inline click-to-edit for `SettingSelect`

Same pattern as SettingText — clicking the displayed value opens the select dropdown directly.

**Files:**
- Modify: `frontend/src/lib/components/ui/SettingSelect.svelte`

- [ ] **Step 1: Replace `SettingSelect.svelte`**

Replace the entire file content of `frontend/src/lib/components/ui/SettingSelect.svelte`:

```svelte
<script lang="ts">
  import SettingRow from './SettingRow.svelte';
  import { settingDescriptions } from '$lib/settings/descriptions';
  import type { SettingDef } from '$lib/api';

  type Props = {
    setting: SettingDef;
    onUpdate: (key: string, value: string) => Promise<SettingDef>;
    savingKey: string | null;
    disabled?: boolean;
  };

  let { setting, onUpdate, savingKey, disabled = false }: Props = $props();

  let editValue = $state('');
  let editing = $state(false);
  let error = $state('');
  let savedPending = $state(false);
  let savedSuccess = $state(false);
  let successTimer: ReturnType<typeof setTimeout> | null = null;
  const isSaving = $derived(savingKey === setting.key);

  function startEdit() {
    if (disabled) return;
    editValue = setting.value;
    editing = true;
    error = '';
  }

  async function save() {
    savedPending = false;
    savedSuccess = false;
    error = '';
    try {
      const result = await onUpdate(setting.key, editValue);
      if (result.requires_restart) {
        savedPending = true;
      } else {
        savedSuccess = true;
        if (successTimer) clearTimeout(successTimer);
        successTimer = setTimeout(() => { savedSuccess = false; }, 2000);
      }
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

<SettingRow label={setting.label} description={settingDescriptions[setting.key]} requiresRestart={setting.requires_restart} isSaving={isSaving} {savedPending} {savedSuccess}>
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
    <!-- Clicking the value enters edit mode directly -->
    <span
      class="text-sm {disabled ? 'text-slate-400 cursor-not-allowed' : 'text-slate-600 cursor-pointer hover:text-slate-900 hover:underline'}"
      onclick={startEdit}
      role="button"
      tabindex={disabled ? -1 : 0}
      onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') startEdit(); }}
      aria-label="Edit {setting.label}"
    >{setting.value || '—'}</span>
  {/if}
  {#if error}
    <span class="text-sm text-red-600 max-w-md break-words">{error}</span>
  {/if}
</SettingRow>
```

- [ ] **Step 2: Run frontend type check**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-check 2>&1 | tail -5
```

Expected: `0 ERRORS`

- [ ] **Step 3: Commit**

```bash
cd /opt/ocserv_panel
git add frontend/src/lib/components/ui/SettingSelect.svelte
git commit -m "feat(frontend): inline click-to-edit for SettingSelect"
```

---

## Task 7: Add "Not Yet Implemented" state to Backup and Notifications sections

Backup and Notifications settings save to the DB but no backend features exist. Add a banner inside those sections and disable their fields.

**Files:**
- Modify: `frontend/src/lib/components/ui/CardSettings.svelte`
- Modify: `frontend/src/routes/settings/+page.svelte`

- [ ] **Step 1: Add `stub` prop to `CardSettings.svelte`**

Replace the entire content of `frontend/src/lib/components/ui/CardSettings.svelte`:

```svelte
<script lang="ts">
  type Props = {
    id?: string;
    title: string;
    label?: string;
    description?: string;
    stub?: boolean;
    children: import('svelte').Snippet;
  };

  let { id, title, label, description, stub = false, children }: Props = $props();
</script>

<section id={id} class="bg-white rounded-lg shadow-sm p-6 scroll-mt-24">
  <p class="text-xs font-bold uppercase tracking-wider text-slate-500">{label ?? title}</p>
  <h3 class="text-lg font-semibold text-slate-800 mt-1">{title}</h3>
  {#if description}
    <p class="text-sm text-slate-500 mt-1">{description}</p>
  {/if}
  {#if stub}
    <div class="mt-3 px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg text-xs text-slate-500">
      This feature is not yet implemented. Settings are saved but have no effect.
    </div>
  {/if}
  <div class="mt-4">
    {@render children()}
  </div>
</section>
```

- [ ] **Step 2: Update `settings/+page.svelte` to pass `stub` and disable stub section fields**

In `frontend/src/routes/settings/+page.svelte`, find the section that renders each `CardSettings` and its settings (around line 291–303). Replace it with:

```svelte
{#each data.sections as section (section.id)}
  {@const isStub = section.id === 'backup' || section.id === 'notifications'}
  <CardSettings
    id={section.id}
    title={section.title}
    label={section.label}
    description={section.description}
    stub={isStub}
  >
    {#each section.settings as setting (setting.key)}
      <SettingField
        {setting}
        onUpdate={handleUpdate}
        {savingKey}
        configHealthy={configHealthy && !isStub}
      />
    {/each}
  </CardSettings>
{/each}
```

- [ ] **Step 3: Run frontend type check**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-check 2>&1 | tail -5
```

Expected: `0 ERRORS`

- [ ] **Step 4: Commit**

```bash
cd /opt/ocserv_panel
git add frontend/src/lib/components/ui/CardSettings.svelte frontend/src/routes/settings/+page.svelte
git commit -m "feat(frontend): add 'not yet implemented' notice to Backup and Notifications sections"
```

---

## Task 8: Fix `SettingTooltip` accessibility

The tooltip span uses `role="button"` with `tabindex="-1"` making it keyboard-inaccessible. Fix with correct role, keyboard handlers, and accessible label.

**Files:**
- Modify: `frontend/src/lib/components/ui/SettingTooltip.svelte`

- [ ] **Step 1: Replace `SettingTooltip.svelte`**

Replace the entire file content:

```svelte
<script lang="ts">
  type Props = {
    description?: string;
  };

  let { description }: Props = $props();

  let showTooltip = $state(false);
  let hoverTimeout: ReturnType<typeof setTimeout> | null = null;

  function show() {
    hoverTimeout = setTimeout(() => {
      showTooltip = true;
    }, 150);
  }

  function hide() {
    if (hoverTimeout) {
      clearTimeout(hoverTimeout);
      hoverTimeout = null;
    }
    showTooltip = false;
  }
</script>

{#if description}
  <span
    class="relative inline-flex items-center ml-1"
    tabindex="0"
    aria-label="Help"
    onmouseenter={show}
    onmouseleave={hide}
    onfocus={show}
    onblur={hide}
  >
    <svg
      class="w-3.5 h-3.5 text-slate-400 hover:text-slate-600 transition-colors cursor-help"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      stroke-width="2"
      aria-hidden="true"
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

- [ ] **Step 2: Run frontend type check**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-check 2>&1 | tail -5
```

Expected: `0 ERRORS`

- [ ] **Step 3: Commit**

```bash
cd /opt/ocserv_panel
git add frontend/src/lib/components/ui/SettingTooltip.svelte
git commit -m "fix(frontend): fix SettingTooltip accessibility — keyboard focus and aria-label"
```

---

## Task 9: Final verification

Run full test suite and type checks to confirm everything is clean.

- [ ] **Step 1: Run all backend tests**

```bash
cd /opt/ocserv_panel/backend && go test ./... -v 2>&1 | tail -30
```

Expected: all packages PASS, no FAIL lines

- [ ] **Step 2: Run frontend type check**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-check 2>&1 | tail -5
```

Expected: `0 ERRORS`

- [ ] **Step 3: Build frontend**

```bash
cd /opt/ocserv_panel/frontend && npx vite build 2>&1 | tail -10
```

Expected: build succeeds with no errors
