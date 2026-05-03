# Settings Tab UI Redesign

**Date:** 2026-05-03
**Status:** Approved

## Problem

The current Settings tab renders each setting as a vertical stack (label on top, value below) with no descriptions, no tooltips, and no table alignment. This makes the page hard to scan and provides no contextual help for administrators.

## Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Description source | Frontend-only (`descriptions.ts`) | No backend changes needed; descriptions are UI concern |
| Layout | Two-column table (label left, control right) | Scannable, matches reference design pattern |
| Tooltips | Hover-only with ⓘ icon | Desktop-first admin panel; minimal complexity |

## Component Architecture

### New Components

**`SettingTooltip.svelte`** — Shared tooltip component
- Renders an ⓘ icon next to label text
- On hover (150ms delay), shows a floating tooltip card with description text
- Hidden when description is empty (no icon shown)
- Dark background (`slate-800`) with white text, max width ~280px
- CSS `position: absolute` relative to icon container

**`SettingRow.svelte`** — Two-column row layout
- CSS Grid: `grid-template-columns: 240px 1fr`
- Left column: label + ⓘ tooltip + restart indicator, right-aligned, vertically centered
- Right column: control slot, left-aligned, vertically centered
- Responsive: stacks vertically on screens <768px
- Row padding: `12px 0` with subtle bottom border separator
- Shows restart indicator (🔄) when `requiresRestart` is true

**Type-specific components** (each renders inside `SettingRow`):

| Component | Renders |
|---|---|
| `SettingToggle.svelte` | Toggle switch (modify existing) |
| `SettingSelect.svelte` | Dropdown `<select>` |
| `SettingText.svelte` | Text `<input>` with inline edit |
| `SettingNumber.svelte` | Number `<input>` with inline edit |
| `SettingPassword.svelte` | Password `<input>` with show/hide toggle |
| `SettingMultiCheckbox.svelte` | Checkbox group |
| `SettingReadonly.svelte` | Static text display |
| `SettingButton.svelte` | Action button |

**`SettingField.svelte`** — Thin dispatcher:
- Receives full `SettingDef` object
- Switches on `setting.type` to render correct sub-component
- Passes common props (`label`, `description`, `requiresRestart`) to `SettingRow`

### Descriptions Map

**`frontend/src/lib/settings/descriptions.ts`**
- `Record<string, string>` keyed by setting key (e.g., `"general.server_name"`)
- Every setting key gets a description; missing keys simply don't show the ⓘ icon
- Descriptions are concise (1-2 sentences), written for admins managing an ocserv instance

## Tooltip Behavior

- ⓘ icon appears next to label only when a description exists for that setting key
- Hovering over ⓘ shows tooltip card below/to the right of the icon
- 150ms appear delay to prevent flickering
- Tooltip disappears immediately when mouse leaves icon or tooltip card
- Desktop-first (no mobile touch consideration needed)

## Layout & Responsive Behavior

### Desktop (≥768px)
- Two-column grid: 240px label column, 1fr control column
- Label column: right-aligned text, vertically centered
- Control column: left-aligned, vertically centered

### Mobile (<768px)
- Stacks vertically: label on top, control below
- Matches current layout behavior

### Section Cards
- `CardSettings.svelte` continues to wrap each section
- Each card has section title and optional section-level description
- Settings within a card use the two-column row layout
- `SettingsNav.svelte` unchanged

### Edit Flow
- **Toggle/Select/Button:** Always visible and interactive
- **Text/Number/Password:** Click current value to enter inline edit mode. Save on blur or Enter, cancel on Escape.
- **Readonly:** Static display, no interaction

## Files Changed

| File | Change |
|---|---|
| `frontend/src/lib/settings/descriptions.ts` | **New** — descriptions map |
| `frontend/src/lib/components/SettingTooltip.svelte` | **New** — tooltip component |
| `frontend/src/lib/components/SettingRow.svelte` | **New** — two-column row layout |
| `frontend/src/lib/components/SettingSelect.svelte` | **New** — select dropdown |
| `frontend/src/lib/components/SettingText.svelte` | **New** — text input |
| `frontend/src/lib/components/SettingNumber.svelte` | **New** — number input |
| `frontend/src/lib/components/SettingPassword.svelte` | **New** — password input with show/hide |
| `frontend/src/lib/components/SettingMultiCheckbox.svelte` | **New** — checkbox group |
| `frontend/src/lib/components/SettingReadonly.svelte` | **New** — readonly display |
| `frontend/src/lib/components/SettingButton.svelte` | **New** — action button |
| `frontend/src/lib/components/SettingField.svelte` | **Modify** — becomes thin dispatcher |
| `frontend/src/lib/components/SettingToggle.svelte` | **Modify** — wrap in SettingRow |
| `frontend/src/lib/components/CardSettings.svelte` | **Minor modify** — pass section descriptions if needed |

## No Backend Changes

The Go API, `SettingDef` struct, and handlers remain untouched. All changes are frontend-only.