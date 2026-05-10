# Sessions Status Cards Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the 4 generic stat cards on the Sessions page with 4 domain-focused cards (Service, Sessions, Auth, Network) using data already provided by `occtl -j -n show status` via the existing `/api/ocserv/server/status` endpoint.

**Architecture:** Single-file change — `SessionStatusCards.svelte` is rewritten to use the full `ServerStatus` type. Three new pure helper functions (`formatDuration`, `relativeDate`) are added to the component's script block. No backend changes, no new files.

**Tech Stack:** SvelteKit, Svelte 5 runes, Tailwind CSS v4, lucide-svelte, existing `CardStats` component, `formatBytes` from `$lib/monitoring`.

---

## File Map

| Action | File |
|--------|------|
| Modify | `frontend/src/routes/sessions/SessionStatusCards.svelte` |
| Test (unit) | `frontend/src/routes/sessions/sessions-status-cards.test.ts` *(new)* |

---

### Task 1: Write unit tests for the three helper functions

The helpers `formatUptime`, `formatDuration`, and `relativeDate` are pure functions — test them in isolation before touching the component.

**Files:**
- Create: `frontend/src/routes/sessions/sessions-status-cards.test.ts`

- [ ] **Step 1: Create the test file**

```typescript
// frontend/src/routes/sessions/sessions-status-cards.test.ts
import { describe, it, expect, beforeEach, vi } from 'vitest';

// ── helpers under test (copied inline so we can test without rendering) ──────
// These will be extracted/verified against the component after implementation.

function formatUptime(upSince: string | undefined): string {
  if (!upSince) return '—';
  const date = new Date(upSince);
  if (isNaN(date.getTime())) return '—';
  const ms = Date.now() - date.getTime();
  if (ms < 0) return '—';
  const totalMin = Math.floor(ms / 60000);
  const totalHours = Math.floor(totalMin / 60);
  const totalDays = Math.floor(totalHours / 24);
  if (totalDays > 0) return `${totalDays}d ${totalHours % 24}h`;
  if (totalHours > 0) return `${totalHours}h ${totalMin % 60}m`;
  return `${totalMin}m`;
}

function formatDuration(seconds: number | undefined): string {
  if (!seconds) return '—';
  if (seconds >= 3600) {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    return `${h}h ${m}m`;
  }
  if (seconds >= 60) return `${Math.floor(seconds / 60)}m`;
  return `${seconds}s`;
}

function relativeDate(iso: string | undefined): string {
  if (!iso) return '—';
  const date = new Date(iso);
  if (isNaN(date.getTime())) return '—';
  const diffMs = Date.now() - date.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMin / 60);
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays === 0) return 'today';
  if (diffDays < 2) return `${diffDays}d ago`;
  if (diffDays < 30) return `${diffDays}d ago`;
  return `${diffHours}h ago`;
}

// ── formatUptime ──────────────────────────────────────────────────────────────
describe('formatUptime', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-10T12:00:00Z'));
  });

  it('returns — for undefined', () => {
    expect(formatUptime(undefined)).toBe('—');
  });

  it('returns — for invalid date string', () => {
    expect(formatUptime('not-a-date')).toBe('—');
  });

  it('formats minutes only when uptime < 1 hour', () => {
    const upSince = new Date('2026-05-10T11:45:00Z').toISOString();
    expect(formatUptime(upSince)).toBe('15m');
  });

  it('formats hours and minutes when uptime < 1 day', () => {
    const upSince = new Date('2026-05-10T08:30:00Z').toISOString();
    expect(formatUptime(upSince)).toBe('3h 30m');
  });

  it('formats days and hours when uptime >= 1 day', () => {
    const upSince = new Date('2026-05-04T07:29:00Z').toISOString();
    // ~6 days and ~4.5 hours
    const result = formatUptime(upSince);
    expect(result).toMatch(/^\d+d \d+h$/);
    expect(result).toContain('6d');
  });
});

// ── formatDuration ────────────────────────────────────────────────────────────
describe('formatDuration', () => {
  it('returns — for undefined', () => {
    expect(formatDuration(undefined)).toBe('—');
  });

  it('returns — for zero', () => {
    expect(formatDuration(0)).toBe('—');
  });

  it('formats seconds when < 60', () => {
    expect(formatDuration(45)).toBe('45s');
  });

  it('formats minutes when >= 60 and < 3600', () => {
    expect(formatDuration(150)).toBe('2m');
  });

  it('formats hours and minutes when >= 3600', () => {
    expect(formatDuration(30840)).toBe('8h 34m');
  });

  it('formats exactly 1 hour', () => {
    expect(formatDuration(3600)).toBe('1h 0m');
  });
});

// ── relativeDate ──────────────────────────────────────────────────────────────
describe('relativeDate', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-10T12:00:00Z'));
  });

  it('returns — for undefined', () => {
    expect(relativeDate(undefined)).toBe('—');
  });

  it('returns — for invalid date', () => {
    expect(relativeDate('bad')).toBe('—');
  });

  it('returns today when same day', () => {
    expect(relativeDate('2026-05-10T07:29:00Z')).toBe('today');
  });

  it('returns Xd ago for older dates', () => {
    expect(relativeDate('2026-05-04T07:29:00Z')).toBe('6d ago');
  });
});
```

- [ ] **Step 2: Run tests to confirm they pass (pure logic, no rendering needed)**

```bash
cd /opt/ocserv_panel/frontend && npx vitest run src/routes/sessions/sessions-status-cards.test.ts
```

Expected: all tests PASS (the helpers are defined inline in the test file).

- [ ] **Step 3: Commit the test file**

```bash
git add frontend/src/routes/sessions/sessions-status-cards.test.ts
git commit -m "test(sessions): unit tests for status card helper functions"
```

---

### Task 2: Rewrite SessionStatusCards.svelte

Replace the current 4-card layout with the 4 domain-focused cards as specified. The helper functions in the component must match the implementations validated in Task 1.

**Files:**
- Modify: `frontend/src/routes/sessions/SessionStatusCards.svelte`

- [ ] **Step 1: Replace the file contents**

```svelte
<script lang="ts">
  import CardStats from '$lib/components/ui/CardStats.svelte';
  import { Server, Users, ShieldAlert, ArrowUpDown } from 'lucide-svelte';
  import { formatBytes } from '$lib/monitoring';
  import type { ServerStatus } from '$lib/api';

  type Props = {
    status: ServerStatus | null;
    loading: boolean;
    error: string;
  };

  let { status, loading, error }: Props = $props();

  function formatUptime(upSince: string | undefined): string {
    if (!upSince) return '—';
    const date = new Date(upSince);
    if (isNaN(date.getTime())) return '—';
    const ms = Date.now() - date.getTime();
    if (ms < 0) return '—';
    const totalMin = Math.floor(ms / 60000);
    const totalHours = Math.floor(totalMin / 60);
    const totalDays = Math.floor(totalHours / 24);
    if (totalDays > 0) return `${totalDays}d ${totalHours % 24}h`;
    if (totalHours > 0) return `${totalHours}h ${totalMin % 60}m`;
    return `${totalMin}m`;
  }

  function formatDuration(seconds: number | undefined): string {
    if (!seconds) return '—';
    if (seconds >= 3600) {
      const h = Math.floor(seconds / 3600);
      const m = Math.floor((seconds % 3600) / 60);
      return `${h}h ${m}m`;
    }
    if (seconds >= 60) return `${Math.floor(seconds / 60)}m`;
    return `${seconds}s`;
  }

  function relativeDate(iso: string | undefined): string {
    if (!iso) return '—';
    const date = new Date(iso);
    if (isNaN(date.getTime())) return '—';
    const diffMs = Date.now() - date.getTime();
    const diffMin = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMin / 60);
    const diffDays = Math.floor(diffHours / 24);
    if (diffDays === 0) return 'today';
    if (diffDays < 2) return `${diffDays}d ago`;
    if (diffDays < 30) return `${diffDays}d ago`;
    return `${diffHours}h ago`;
  }

  const isOnline = $derived(status?.status === 'online');

  const serviceValue = $derived(
    loading ? '...' : (isOnline ? formatUptime(status?.upSince) : 'Offline')
  );
  const serviceSubtitle = $derived(
    isOnline && status?.serverPid ? `PID: ${status.serverPid}` : undefined
  );

  const sessionsValue = $derived(loading ? '...' : String(status?.activeSessions ?? 0));
  const sessionsSubtitle = $derived(
    status
      ? `${status.sessionsHandled} handled · avg ${formatDuration(status.avgSessionTimeSeconds)}`
      : undefined
  );

  const authValue = $derived(loading ? '...' : String(status?.authFailures ?? 0));
  const authSubtitle = $derived(
    status
      ? `Avg: ${status.avgAuthTimeSeconds}s · Max: ${status.maxAuthTimeSeconds}s`
      : undefined
  );

  const networkValue = $derived(
    loading
      ? '...'
      : status
        ? `${formatBytes(status.rxBytes)} / ${formatBytes(status.txBytes)}`
        : '—'
  );
  const networkSubtitle = $derived(
    status
      ? `MTU ${status.minMtu}–${status.maxMtu} · reset ${relativeDate(status.lastStatsReset)}`
      : undefined
  );
</script>

{#if error}
  <div class="px-4 py-2 rounded-lg text-sm bg-amber-50 text-amber-700 border border-amber-200" role="alert">
    Server status unavailable: {error}
  </div>
{:else}
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
    <CardStats
      title="Service"
      value={serviceValue}
      subtitle={serviceSubtitle}
      icon={Server}
      statusValue={isOnline ? 'online' : 'offline'}
    />
    <CardStats
      title="Sessions"
      value={sessionsValue}
      subtitle={sessionsSubtitle}
      icon={Users}
    />
    <CardStats
      title="Auth"
      value={authValue}
      subtitle={authSubtitle}
      icon={ShieldAlert}
    />
    <CardStats
      title="Network"
      value={networkValue}
      subtitle={networkSubtitle}
      icon={ArrowUpDown}
    />
  </div>
{/if}
```

- [ ] **Step 2: Type-check the frontend**

```bash
cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig tsconfig.json 2>&1 | tail -20
```

Expected: `0 errors` (warnings about unused imports from other files are fine).

- [ ] **Step 3: Run the full test suite**

```bash
cd /opt/ocserv_panel/frontend && npx vitest run
```

Expected: all tests pass including the new helper tests.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/sessions/SessionStatusCards.svelte
git commit -m "feat(sessions): redesign status cards — Service, Sessions, Auth, Network"
```

---

### Task 3: Manual verification

- [ ] **Step 1: Build the frontend to catch any runtime issues**

```bash
cd /opt/ocserv_panel/frontend && npm run build 2>&1 | tail -20
```

Expected: build completes with no errors.

- [ ] **Step 2: Verify the four cards render with correct data**

Start the dev server and open the Sessions page. Confirm:
- **Service card:** shows uptime (e.g. `6d 4h`) as headline, green status dot, `PID: 3108823` subtitle
- **Sessions card:** shows active session count as headline, `32 handled · avg 8h 34m` subtitle
- **Auth card:** shows auth failure count as headline, `Avg: 1s · Max: 10s` subtitle
- **Network card:** shows `165.7 MB / 4.8 GB` headline, `MTU 1284–1292 · reset 6d ago` subtitle

- [ ] **Step 3: Verify offline/loading degradation**

With server offline (or mock): Service card shows `Offline` as headline, no PID subtitle. All other cards show `0` counts and `—` for times.

- [ ] **Step 4: Final commit if any fixes were needed, otherwise done**

```bash
git status
# if clean: nothing to commit — implementation complete
```
