# Settings Page Fixes — Design Spec

**Date:** 2026-05-11  
**Status:** Approved

---

## Objective

Fix six concrete bugs and UX gaps found in the settings page implementation. The May 10 spec (`2026-05-10-settings-page-improvements-design.md`) is substantially complete — this spec addresses the remaining open issues.

---

## Issues & Fixes

### 1. Backend — `handleRestart` Is a No-Op (Critical)

**Problem:** `POST /api/settings/restart` returns `200 OK` with `"restart not yet implemented"`. The frontend calls it, then polls `/api/settings/restart-status` every 3 seconds for up to 2 minutes. Since nothing restarts, `restart_required` stays `true`, the poll never exits, and the UI is stuck in a "Restarting…" state until the 2-minute timeout.

**Fix:** Add `RestartPanelService(ctx context.Context, principal auth.Principal) error` to `OcservService`. The method:
1. Checks `principal.Authenticated()` — returns `ErrUnauthenticated` if not
2. Checks `authorizer.Allowed(roles, rbac.PermissionSettingsRestart)` — returns `ErrPermissionDenied` if not
3. Records audit event `settings.restart` / resource `service` / `OutcomeSuccess`
4. Calls `s.svc.Restart(ctx)` — this invokes `systemctl restart <unit>` via `SystemdManager`

Wire `handleRestart` to call `a.ocservService.RestartPanelService(r.Context(), principal)` and map the result through `writeAppError`.

After `systemctl restart` executes, the process is signalled to stop. The service manager (systemd) starts a new process. During the restart window the frontend's poll calls will get network errors; once the server is back the poll succeeds. The existing 2-minute safety timeout is already in place.

**Files:**
- `backend/internal/app/ocserv_service.go` — add `RestartPanelService`
- `backend/internal/server/settings_handlers.go` — wire `handleRestart`

**Test:** Add a test case to the `OcservService` test harness:
- Unauthenticated principal → `ErrUnauthenticated`
- Viewer principal → `ErrPermissionDenied`
- Owner principal → `FakeServiceManager.RestartCount() == 1`, audit event recorded

---

### 2. Backend — `MonitoringConfig` Bool Fields Should Be String

**Problem:** `MonitoringConfig.HealthEndpoint` and `MetricsEndpoint` are `bool` in `config.go`. The registry registers them as `SettingTypeReadonly` with string values like `"/api/health (always enabled)"` and `"Not implemented"`. The `applySettingToConfig` switch cases for these keys call `parseBool()` — if they were ever invoked this would panic or return an error. The `configValueForKey` function overrides these with hardcoded strings, hiding the struct mismatch.

**Fix:** Change both fields to `string` in `MonitoringConfig`:
```go
HealthEndpoint  string `toml:"health_endpoint"`
MetricsEndpoint string `toml:"metrics_endpoint"`
```

Update `Defaults()` to set:
```go
HealthEndpoint:  "true",
MetricsEndpoint: "false",
```

Update `applySettingToConfig` cases to assign `value` directly (no bool parse). The `configValueForKey` cases already return hardcoded strings for these two keys — keep them as-is since the UI should show the descriptive string, not the raw config value.

Check `MergeEnv` — it does not reference these fields, no change needed there.

**Files:**
- `backend/internal/config/config.go`
- `backend/internal/app/settings_config_apply.go`

---

### 3. Backend — Remove `service.restart_on_failure`

**Problem:** `service.restart_on_failure` is a database-source toggle in the registry. It saves to SQLite successfully but configures nothing. Whether systemd restarts on failure is controlled by the unit file (`Restart=on-failure`), not by this setting. Exposing it as a settable toggle is misleading.

**Fix:** Remove the `service.restart_on_failure` entry from `AllSettingDefs()`. No migration needed — the value may remain in the `app_settings` table as an orphaned row (harmless).

**Files:**
- `backend/internal/app/settings_registry.go`

---

### 4. Frontend — Inline Click-to-Edit for SettingText and SettingSelect

**Problem:** `SettingText.svelte` and `SettingSelect.svelte` show the current value as a `<span>` with a separate "Edit" button. This is inconsistent with `SettingToggle` (direct interaction) and the May 10 spec intent (inline edit on click).

**Fix:** Make the displayed value itself clickable to enter edit mode. The "Edit" button is removed.

**SettingText view mode:**
```html
<span
  class="text-sm text-slate-600 cursor-pointer hover:text-slate-900 hover:underline"
  onclick={startEdit}
  role="button"
  tabindex="0"
  onkeydown={(e) => e.key === 'Enter' && startEdit()}
>
  {setting.value || '—'}
</span>
```

**SettingSelect view mode:** Same pattern — clicking the value displays the `<select>` dropdown.

Edit mode (input/select + Save + Cancel buttons) is unchanged.

**Files:**
- `frontend/src/lib/components/ui/SettingText.svelte`
- `frontend/src/lib/components/ui/SettingSelect.svelte`

---

### 5. Frontend — "Not Yet Active" State for Backup and Notifications Sections

**Problem:** The Backup and Notifications settings sections save to SQLite correctly but none of the logic they configure (backup scheduler, webhook sender) exists. Users who toggle "Enable Notifications" to true have no way to know nothing will happen.

**Fix:** In `+page.svelte`, pass `disabled={section.id === 'backup' || section.id === 'notifications'}` to `CardSettings`, and conditionally render a notice banner inside the card for those sections.

Add a `stub` prop to `CardSettings`:
```svelte
<!-- CardSettings.svelte: when stub === true -->
{#if stub}
  <div class="mb-3 px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg text-xs text-slate-500">
    This feature is not yet implemented. Settings are saved but have no effect.
  </div>
{/if}
```

In `+page.svelte`, when rendering each section:
```svelte
<CardSettings
  id={section.id}
  title={section.title}
  label={section.label}
  description={section.description}
  stub={section.id === 'backup' || section.id === 'notifications'}
>
  {#each section.settings as setting (setting.key)}
    <SettingField
      {setting}
      onUpdate={handleUpdate}
      {savingKey}
      configHealthy={configHealthy && section.id !== 'backup' && section.id !== 'notifications'}
    />
  {/each}
</CardSettings>
```

Passing `configHealthy=false` for stub sections reuses the existing disabled-input code path in all `SettingField` components without new logic.

**Files:**
- `frontend/src/lib/components/ui/CardSettings.svelte`
- `frontend/src/routes/settings/+page.svelte`

---

### 6. Frontend — `SettingTooltip` Accessibility

**Problem:** The `<span>` wrapper has `role="button"` but `tabindex="-1"`, making it mouse-only. The SVG has `aria-hidden="true"` leaving no accessible label.

**Fix:**
```svelte
<span
  class="relative inline-flex items-center ml-1"
  role="tooltip"
  tabindex="0"
  aria-label="Help"
  onmouseenter={handleMouseEnter}
  onmouseleave={handleMouseLeave}
  onfocus={handleMouseEnter}
  onblur={handleMouseLeave}
>
  <svg aria-hidden="true" ...>...</svg>
  {#if showTooltip}
    <div role="tooltip" ...>{description}</div>
  {/if}
</span>
```

Add `onfocus`/`onblur` handlers mirroring `mouseenter`/`mouseleave` so keyboard users see the tooltip on tab focus.

**Files:**
- `frontend/src/lib/components/ui/SettingTooltip.svelte`

---

## Scope Boundaries

**In scope:**
- All six fixes above

**Out of scope:**
- Implementing backup scheduler or webhook sender
- Adding restart button to other pages
- Removing DB rows for `service.restart_on_failure`
- Changing how systemd is configured (unit file)

---

## Files Changed

| File | Change |
|------|--------|
| `backend/internal/app/ocserv_service.go` | Add `RestartPanelService` method |
| `backend/internal/server/settings_handlers.go` | Wire `handleRestart` to `ocservService.RestartPanelService` |
| `backend/internal/config/config.go` | `MonitoringConfig.HealthEndpoint/MetricsEndpoint` bool → string |
| `backend/internal/app/settings_config_apply.go` | Remove `parseBool` for those two keys |
| `backend/internal/app/settings_registry.go` | Remove `service.restart_on_failure` |
| `frontend/src/lib/components/ui/SettingText.svelte` | Inline click-to-edit |
| `frontend/src/lib/components/ui/SettingSelect.svelte` | Inline click-to-edit |
| `frontend/src/lib/components/ui/CardSettings.svelte` | Add `stub` prop + notice banner |
| `frontend/src/routes/settings/+page.svelte` | Pass `stub` for backup/notifications |
| `frontend/src/lib/components/ui/SettingTooltip.svelte` | Fix accessibility |
