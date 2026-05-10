# Settings Page Improvements — Design Spec

**Date:** 2026-05-10  
**Status:** Approved

---

## Objective

Fix functional bugs, wire theme setting to the UI, forward precise validation error messages to users, and deliver a full visual redesign of the settings page.

---

## Issues Found & Fixes

### 1. Backend: Validation errors swallowed (Critical)

**File:** `backend/internal/server/server.go` — `writeAppError`  
**Problem:** `ErrInvalidInput` maps to the generic string `"invalid request"`, discarding the specific message from `validateSettingValue` (e.g. "must be at least 5", "must be at most 65535").  
**Fix:** Preserve and forward the wrapped error message text in the JSON response body when the error `Is(ErrInvalidInput)`. Use `err.Error()` as the message (the sentinel is wrapped so `errors.Is` still identifies the type).

```go
case errors.Is(err, app.ErrInvalidInput):
    writeError(w, http.StatusBadRequest, err.Error())
```

This approach is safe: the error messages are programmer-authored strings in `settings_service.go`, not user-controlled values. No secret data flows through this path.

---

### 2. Backend: `general.language` should be readonly

**File:** `backend/internal/app/settings_registry.go`  
**Problem:** Language is `SettingTypeSelect` with only one option (`"en"`), giving the false impression it can be changed.  
**Fix:** Change `general.language` to `SettingTypeReadonly` with value `"English (en)"`. Remove it from `configValueForKey` switch entirely (it's database-source, defaults handled normally).

---

### 3. Frontend: Theme setting not wired to mode-watcher

**Files:** `frontend/src/routes/settings/+page.svelte`, `frontend/src/routes/+layout.svelte`  
**Problem:** `general.theme` saves to the database but the UI ignores the saved value — `ModeWatcher` uses the OS preference, not the DB setting.  
**Fix:**
- In `+layout.svelte` (or a layout `+layout.server.ts`): after loading auth, fetch the `general.theme` setting value and call `setMode('light' | 'dark' | 'system')` from `mode-watcher`.
- In `settings/+page.svelte`: after `handleUpdate('general.theme', value)` succeeds, call `setMode(value)` immediately without waiting for a page reload.
- Since this is a SPA with `adapter-static`, use `onMount` in `+layout.svelte` to fetch theme on load.

Implementation detail:
```ts
// +layout.svelte onMount
import { setMode } from 'mode-watcher';
const settings = await getSettingsSections();
const theme = settings.sections
  .flatMap(s => s.settings)
  .find(s => s.key === 'general.theme')?.value ?? 'system';
setMode(theme as 'light' | 'dark' | 'system');
```

---

### 4. Frontend: No client-side validation for SettingNumber

**File:** `frontend/src/lib/components/ui/SettingNumber.svelte`  
**Problem:** The backend `Validation` field (e.g. `"min:5"`, `"min:1,max:65535"`) is available in the `SettingDef` but unused client-side. Users see a round-trip error.  
**Fix:** Parse the `validation` string in `SettingNumber` before calling `onUpdate`:
- Extract `min:N` and `max:N` from the comma-separated string
- Show inline error immediately without hitting the API
- Also validate that the value is a valid integer

---

### 5. Frontend: No success feedback for database-source settings

**Problem:** After saving a toggle, select, or text field with `source === 'database'`, nothing confirms success (no toast, no inline indicator). Only config-source settings show "restart pending".  
**Fix:** Show a brief ✓ indicator after successful save. Use a transient state variable that auto-clears after 2 seconds via `setTimeout`. Add `savedSuccess` state alongside `savedPending` in each Setting component.

---

### 6. Frontend: Restart banner shows raw setting keys

**File:** `frontend/src/lib/components/ui/RestartBanner.svelte`  
**Problem:** The pending changes list shows e.g. `monitoring.dashboard_refresh_interval` instead of `Dashboard Refresh Interval (s)`.  
**Fix:** The `SettingsSectionsResponse` contains the full `SettingDef` for each section with labels. Map setting keys to labels in `+page.svelte` before passing to `RestartBanner`. Build a `Map<string, string>` from key → label while loading sections, pass it to `RestartBanner` as `labelMap`.

---

## Visual Redesign

### Layout

Replace the current flat "list of cards" approach with a **two-column layout**:
- **Left sidebar (280px)**: sticky section navigation (the current floating nav tab bar replaced by a proper sidebar nav)
- **Right content area**: the settings cards scroll past

On mobile, the sidebar collapses and the top tab bar remains (keep current mobile behavior).

### Section cards

- Remove the `bg-card` header block at the top of the page — integrate the title into the first card or as a sticky page header
- Use `border-l-2` accent on the active section in the sidebar
- Add a `source` badge next to the section title: `● config` (amber) or `● instant` (green) to indicate whether any settings in the section require a restart
- Card header: add the description text in muted style (currently done), keep the pattern

### Setting rows

- **Text/Number/Select fields**: Adopt inline-edit-on-click (click the value to enter edit mode, not a separate "Edit" button). This matches the toggle's direct-interaction UX.
- **Number field**: show the constraint hint (e.g. "min 5") in muted text below the input during edit mode
- **Success indicator**: a green `✓ Saved` badge that fades out after 2s (replaces silence)
- **Saving spinner**: keep existing "Saving…" label, but make it a small spinner icon
- **SettingRow label column**: remove the `justify-end` + `text-right` alignment — align labels left for readability
- **Tooltip**: improve — use a `title` attribute fallback, fix the `role="img"` to use `role="tooltip"`

### SettingsNav (sidebar variant)

Replace the scrollable tab bar with a sticky vertical nav in the left column:
```
○ General
○ Installation
● Monitoring      ← active, highlighted
○ Service
```
- Active section highlighted with `bg-slate-100 border-l-2 border-slate-600`
- Section labels grouped visually: first 4 (core config) / middle (server/web) / bottom (users/policies/logging) — add thin separators
- On mobile: keep as horizontal scrollable tabs at top

---

## Accessibility Fixes

- `SettingTooltip`: change `role="img"` → `aria-describedby` pattern or remove role; use `aria-label="Help"` on the trigger icon
- `SettingToggle`: already has `role="switch"` and `aria-checked` — good
- Add `aria-invalid` on Number/Text inputs when there's a validation error

---

## Scope Boundaries

**In scope:**
- Backend: fix `writeAppError` for `ErrInvalidInput`, fix `general.language` to readonly
- Frontend: theme wiring, client-side number validation, success feedback, human labels in restart banner, full visual redesign

**Out of scope:**
- Implementing backup/notifications/restore (they're UI stubs that don't work — keep them as-is)
- Implementing `service.restart_on_failure` DB toggle (the flag has no backend effect — it's informational)
- Dark mode CSS overhaul (mode-watcher handles the CSS classes, Tailwind dark: variants are already in use)

---

## Files Changed

| File | Agent | Change |
|------|-------|--------|
| `backend/internal/server/server.go` | backend | Forward ErrInvalidInput message |
| `backend/internal/app/settings_registry.go` | backend | language → readonly |
| `frontend/src/routes/+layout.svelte` | frontend | Load and apply theme on mount |
| `frontend/src/routes/settings/+page.svelte` | frontend | Apply theme on change, human labels for restart banner |
| `frontend/src/lib/components/ui/SettingNumber.svelte` | frontend | Client-side validation |
| `frontend/src/lib/components/ui/SettingRow.svelte` | frontend | Label alignment, redesign |
| `frontend/src/lib/components/ui/SettingTooltip.svelte` | frontend | Fix aria role |
| `frontend/src/lib/components/ui/SettingText.svelte` | frontend | Success feedback, inline edit |
| `frontend/src/lib/components/ui/SettingSelect.svelte` | frontend | Success feedback |
| `frontend/src/lib/components/ui/SettingToggle.svelte` | frontend | Success feedback |
| `frontend/src/lib/components/ui/SettingPassword.svelte` | frontend | Empty-value guard, success feedback |
| `frontend/src/lib/components/ui/RestartBanner.svelte` | frontend | Human labels |
| `frontend/src/lib/components/ui/SettingsNav.svelte` | frontend | Sidebar nav variant |
| `frontend/src/routes/settings/+page.svelte` | frontend | Two-column layout, sidebar nav |
| `backend/internal/app/settings_config_apply_test.go` | backend | Adjust tests if needed |
