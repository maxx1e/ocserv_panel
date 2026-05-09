# Sessions Tab Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve the Sessions page with configurable refresh timer, click-to-detail for sessions and routes, ban status badges, events pagination, and tab rename.

**Architecture:** Frontend-heavy improvement touching 7 existing Svelte components plus 1 new component (RefreshTimer). Backend adds one new endpoint (`GET /api/ocserv/connections/{id}`) backed by `occtl -j -n show id [ID]`. All changes follow existing patterns exactly.

**Tech Stack:** SvelteKit (Svelte 5 runes), Tailwind CSS, Go HTTP handlers, occtl CLI integration

---

## File Structure

| File | Responsibility |
|------|---------------|
| `frontend/src/routes/sessions/RefreshTimer.svelte` | **NEW** — Circular countdown + interval dropdown + refresh button |
| `frontend/src/routes/sessions/+page.svelte` | Replace inline timer logic with RefreshTimer, rename tab |
| `frontend/src/routes/sessions/ActiveSessionsTab.svelte` | Rename title |
| `frontend/src/routes/sessions/AllSessionsTab.svelte` | Add click-to-detail with SessionDetailDrawer |
| `frontend/src/routes/sessions/EventsTab.svelte` | Add PaginatedTable wrapper |
| `frontend/src/routes/sessions/RoutesTab.svelte` | Add click-to-detail with SessionDetailDrawer |
| `frontend/src/routes/sessions/BansTab.svelte` | Add status badges per IP |
| `frontend/src/lib/api.ts` | Add `getConnectionById` function |
| `backend/internal/ocserv/client.go` | Add `GetConnectionByID` to Manager interface + FakeManager |
| `backend/internal/ocserv/occtl_adapter.go` | Implement `GetConnectionByID` |
| `backend/internal/ocserv/occtl_adapter_test.go` | Tests for new method |
| `backend/internal/app/ocserv_service.go` | Add `GetConnectionByID` service method |
| `backend/internal/server/ocserv_handlers.go` | Add `handleOcservConnectionDetail` handler |
| `backend/internal/server/ocserv_handlers_test.go` | Handler test |
| `backend/internal/server/server.go` | Register new route |

---

### Task 1: Backend — Add `GetConnectionByID` to Manager Interface and FakeManager

**Files:**
- Modify: `backend/internal/ocserv/client.go`

- [ ] **Step 1: Add `GetConnectionByID` to the Manager interface**

In `backend/internal/ocserv/client.go`, add to the `Manager` interface after `GetUserDetails`:

```go
// GetConnectionByID returns full connection details for a numeric connection ID.
// Uses `occtl show id <ID>` under the hood.
GetConnectionByID(ctx context.Context, id int) (*UserConnection, error)
```

- [ ] **Step 2: Add fields to FakeManager**

In the `FakeManager` struct in `backend/internal/ocserv/client.go`, add:

```go
ConnectionByIDResult map[int]*UserConnection
ConnectionByIDErr    error
```

- [ ] **Step 3: Update `NewFakeManager` to initialize the map**

```go
func NewFakeManager() *FakeManager {
	return &FakeManager{
		SessionDetails:        make(map[string]*SessionDetail),
		SessionDetailNotFound: make(map[string]bool),
		ConnectionByIDResult:  make(map[int]*UserConnection),
		UserDetailsResult:     make(map[string][]UserConnection),
		DisconnectErr:         ErrNotConfigured,
		TerminateErr:          ErrNotConfigured,
	}
}
```

- [ ] **Step 4: Add FakeManager method**

```go
// GetConnectionByID returns the configured connection for the given ID.
func (f *FakeManager) GetConnectionByID(ctx context.Context, id int) (*UserConnection, error) {
	_ = ctx
	if conn, ok := f.ConnectionByIDResult[id]; ok {
		return conn, nil
	}
	if f.ConnectionByIDErr != nil {
		return nil, f.ConnectionByIDErr
	}
	return nil, ErrSessionNotFound
}
```

- [ ] **Step 5: Verify compilation**

Run: `cd /opt/ocserv_panel/backend && go build ./...`
Expected: Compilation error — `OcctlAdapter` does not implement `Manager` (missing `GetConnectionByID`). This is expected and will be fixed in Task 2.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/ocserv/client.go
git commit -m "feat(ocserv): add GetConnectionByID to Manager interface and FakeManager"
```

---

### Task 2: Backend — Implement `GetConnectionByID` in OcctlAdapter

**Files:**
- Modify: `backend/internal/ocserv/occtl_adapter.go`
- Modify: `backend/internal/ocserv/occtl_adapter_test.go`

- [ ] **Step 1: Write the test for successful lookup**

Add to `backend/internal/ocserv/occtl_adapter_test.go`:

```go
func TestOcctlAdapterGetConnectionByID(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout: `[{
			"ID": 3549163,
			"Username": "maxxie",
			"Groupname": "(none)",
			"State": "connected",
			"vhost": "default",
			"Device": "vpns0",
			"MTU": "1292",
			"Remote IP": "5.166.153.114",
			"Location": "unknown",
			"Local Device IP": "188.68.41.144",
			"IPv4": "10.10.10.208",
			"P-t-P IPv4": "10.10.10.1",
			"User-Agent": "Open AnyConnect VPN Agent 9.12-3",
			"RX": "304",
			"TX": "304",
			"Average RX": "0 bytes/s",
			"Average TX": "0 bytes/s",
			"DPD": "90",
			"KeepAlive": "32400",
			"Hostname": "LemonGrass",
			"Connected at": "2026-05-09 12:23",
			"raw_connected_at": 1778322239,
			"Full session": "YwDQr54wGdgKCAju+hgsYQcJLc0=",
			"Session": "YwDQr5",
			"TLS ciphersuite": "(TLS1.3)-(ECDHE-SECP256R1)",
			"DTLS cipher": "(DTLS1.2)-(PSK)-(CHACHA20-POLY1305)",
			"DNS": ["1.1.1.1", "9.9.9.9"],
			"Routes": "defaultroute",
			"Restricted to routes": "False"
		}]`,
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	conn, err := adapter.GetConnectionByID(context.Background(), 3549163)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn == nil {
		t.Fatal("expected non-nil connection")
	}
	if conn.Username != "maxxie" {
		t.Fatalf("expected username maxxie, got %s", conn.Username)
	}
	if conn.IPv4 != "10.10.10.208" {
		t.Fatalf("expected IPv4 10.10.10.208, got %s", conn.IPv4)
	}
	if conn.Hostname != "LemonGrass" {
		t.Fatalf("expected hostname LemonGrass, got %s", conn.Hostname)
	}
	if conn.SessionID != "YwDQr5" {
		t.Fatalf("expected session ID YwDQr5, got %s", conn.SessionID)
	}

	// Verify command arguments
	calls := runner.Calls["/usr/local/bin/occtl"]
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	args := calls[0]
	// Should include: -j -n -s /var/run/occtl.socket show id 3549163
	found := false
	for i, a := range args {
		if a == "show" && i+2 < len(args) && args[i+1] == "id" && args[i+2] == "3549163" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'show id 3549163' in args, got %v", args)
	}
}

func TestOcctlAdapterGetConnectionByIDNotFound(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	runner.Results["/usr/local/bin/occtl"] = CommandResult{
		ExitCode: 0,
		Stdout:   "",
	}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	conn, err := adapter.GetConnectionByID(context.Background(), 9999999)
	if err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v (conn=%v)", err, conn)
	}
}

func TestOcctlAdapterGetConnectionByIDInvalidID(t *testing.T) {
	t.Parallel()
	runner := NewFakeCommandRunner()
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/occtl.socket", runner)

	_, err := adapter.GetConnectionByID(context.Background(), -1)
	if err == nil {
		t.Fatal("expected error for negative ID")
	}

	_, err = adapter.GetConnectionByID(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error for zero ID")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/ocserv/ -run TestOcctlAdapterGetConnectionByID -v`
Expected: Compilation failure — `GetConnectionByID` not defined on `OcctlAdapter`.

- [ ] **Step 3: Implement `GetConnectionByID`**

Add to `backend/internal/ocserv/occtl_adapter.go` after the `GetUserDetails` method (around line 357):

```go
// GetConnectionByID calls `occtl -j -n show id <ID>` and parses the connection detail.
// The output format is the same as `show user` (array of connection objects).
// Returns ErrSessionNotFound if the ID has no active connection.
// Timeout: 10 seconds.
func (a *OcctlAdapter) GetConnectionByID(ctx context.Context, id int) (*UserConnection, error) {
	if id <= 0 {
		return nil, fmt.Errorf("connection ID must be positive, got %d", id)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	idStr := strconv.Itoa(id)
	result, err := a.runner.Run(ctx, a.occtlPath, a.withJSONAndSocket([]string{"show", "id", idStr}), nil)
	if err != nil {
		return nil, fmt.Errorf("get connection by id %d: %w", id, err)
	}
	if result.ExitCode != 0 {
		if a.isNotConfiguredErr(result.Stderr) {
			return nil, ErrNotConfigured
		}
		return nil, fmt.Errorf("get connection by id %d: %s", id, strings.TrimSpace(result.Stderr))
	}

	out := strings.TrimSpace(result.Stdout)
	if out == "" {
		return nil, ErrSessionNotFound
	}

	// Parse using the same user detail JSON parser (output format is identical).
	conns, err := parseUserDetailJSON([]byte(out))
	if err != nil {
		// Fall back to text parsing.
		conns, err = parseUserDetailOutput(out)
		if err != nil {
			return nil, fmt.Errorf("parse connection by id %d: %w", id, err)
		}
	}

	if len(conns) == 0 {
		return nil, ErrSessionNotFound
	}
	return &conns[0], nil
}
```

Also add `"strconv"` to the imports if not already present.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/ocserv/ -run TestOcctlAdapterGetConnectionByID -v`
Expected: All 3 tests PASS.

- [ ] **Step 5: Run full package tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/ocserv/ -v`
Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/ocserv/occtl_adapter.go backend/internal/ocserv/occtl_adapter_test.go
git commit -m "feat(ocserv): implement GetConnectionByID in OcctlAdapter"
```

---

### Task 3: Backend — Add Service Method and HTTP Handler

**Files:**
- Modify: `backend/internal/app/ocserv_service.go`
- Modify: `backend/internal/server/ocserv_handlers.go`
- Modify: `backend/internal/server/server.go`
- Modify: `backend/internal/server/ocserv_handlers_test.go`

- [ ] **Step 1: Add service method**

Add to `backend/internal/app/ocserv_service.go` after `GetUserConnections` (around line 441):

```go
// GetConnectionByID returns connection details for a specific connection ID.
func (s *OcservService) GetConnectionByID(ctx context.Context, principal auth.Principal, id int) (*ocserv.UserConnection, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservSessionsRead, "ocserv.connections.detail"); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, fmt.Errorf("%w: connection ID must be positive", ErrInvalidInput)
	}
	conn, err := s.sessions.GetConnectionByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get connection by id %d: %w", id, err)
	}
	return conn, nil
}
```

- [ ] **Step 2: Add HTTP handler**

Add to `backend/internal/server/ocserv_handlers.go` after `handleOcservUserConnections` (around line 612):

```go
// handleOcservConnectionDetail returns full connection detail for a single connection ID.
func (a *App) handleOcservConnectionDetail(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id must be a positive integer")
		return
	}
	conn, err := a.ocservService.GetConnectionByID(r.Context(), principal, id)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userConnectionToResponse(*conn))
}
```

Add `"strconv"` to the imports of `ocserv_handlers.go` if not already present.

- [ ] **Step 3: Register the route**

Add to `backend/internal/server/server.go` after the line registering `GET /api/ocserv/routes` (line ~250):

```go
mux.HandleFunc("GET /api/ocserv/connections/{id}", appHandler.handleOcservConnectionDetail)
```

- [ ] **Step 4: Add handler tests**

Add to `backend/internal/server/ocserv_handlers_test.go`:

```go
// --- handleOcservConnectionDetail tests ---

func TestOcservConnectionDetailRequiresAuth(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/connections/3549163", nil, "")
	assertStatus(t, rec, http.StatusUnauthorized)
}

func TestOcservConnectionDetailInvalidID(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/connections/abc", nil, cookie)
	assertStatus(t, rec, http.StatusBadRequest)
}

func TestOcservConnectionDetailSuccess(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.ocservFake.ConnectionByIDResult[3549163] = &ocserv.UserConnection{
		ID:       "3549163",
		Username: "maxxie",
		IPv4:     "10.10.10.208",
		Device:   "vpns0",
		Hostname: "LemonGrass",
	}
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/connections/3549163", nil, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body map[string]any
	decodeJSON(t, rec, &body)
	assertMapField(t, body, "username", "maxxie")
	assertMapField(t, body, "ipv4", "10.10.10.208")
	assertMapField(t, body, "hostname", "LemonGrass")
}

func TestOcservConnectionDetailNotFound(t *testing.T) {
	t.Parallel()
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-pass")
	cookie := h.login(t, "owner", "owner-pass")

	// FakeManager returns ErrSessionNotFound for unknown IDs
	rec := serveAPI(t, h.router, http.MethodGet, "/api/ocserv/connections/9999999", nil, cookie)
	// ErrSessionNotFound should map to 404 or trigger error handling
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 404 or 500, got %d", rec.Code)
	}
}
```

This follows the exact test pattern used by `TestOcservSessionDetailRequiresAuth`, `TestOcservSessionDetailReturnsDetail`, etc. — using `newTestHarness`, `h.createOwner`, `h.login`, `serveAPI`, `assertStatus`, `decodeJSON`, `assertMapField`.

- [ ] **Step 5: Verify compilation and tests**

Run: `cd /opt/ocserv_panel/backend && go build ./... && go test ./internal/server/ -run TestHandleOcservConnectionDetail -v`
Expected: Build succeeds, tests pass.

- [ ] **Step 6: Run full test suite**

Run: `cd /opt/ocserv_panel/backend && go test ./...`
Expected: All tests pass.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/app/ocserv_service.go backend/internal/server/ocserv_handlers.go backend/internal/server/server.go backend/internal/server/ocserv_handlers_test.go
git commit -m "feat(api): add GET /api/ocserv/connections/{id} endpoint for connection detail by ID"
```

---

### Task 4: Frontend — Add `getConnectionById` API Function

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Add the API function**

Add after the `getUserConnections` function in `frontend/src/lib/api.ts`:

```typescript
export function getConnectionById(id: number): Promise<UserConnection> {
  return apiFetch<UserConnection>(`/api/ocserv/connections/${id}`);
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -5`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat(frontend): add getConnectionById API function"
```

---

### Task 5: Frontend — Create `RefreshTimer.svelte` Component

**Files:**
- Create: `frontend/src/routes/sessions/RefreshTimer.svelte`

- [ ] **Step 1: Create the component**

Create `frontend/src/routes/sessions/RefreshTimer.svelte`:

```svelte
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { RefreshCw, ChevronDown } from 'lucide-svelte';

  type Props = {
    onRefresh: () => void;
    paused?: boolean;
  };

  let { onRefresh, paused = false }: Props = $props();

  const STORAGE_KEY = 'sessions_refresh_interval';
  const INTERVALS = [
    { value: 10000, label: '10s' },
    { value: 15000, label: '15s' },
    { value: 20000, label: '20s' },
    { value: 30000, label: '30s' },
    { value: 60000, label: '1m' },
  ];

  let selectedInterval = $state(30000);
  let timeRemaining = $state(30000);
  let dropdownOpen = $state(false);
  let intervalId: ReturnType<typeof setInterval> | null = null;
  let tickId: ReturnType<typeof setInterval> | null = null;

  // Derived
  const progress = $derived(1 - timeRemaining / selectedInterval);
  const selectedLabel = $derived(INTERVALS.find(i => i.value === selectedInterval)?.label ?? '30s');

  // SVG circle constants
  const RADIUS = 9;
  const CIRCUMFERENCE = 2 * Math.PI * RADIUS;
  const strokeDashoffset = $derived(CIRCUMFERENCE * (1 - progress));

  function loadInterval() {
    if (typeof window === 'undefined') return;
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      const val = parseInt(stored, 10);
      if (INTERVALS.some(i => i.value === val)) {
        selectedInterval = val;
        timeRemaining = val;
      }
    }
  }

  function saveInterval(val: number) {
    localStorage.setItem(STORAGE_KEY, String(val));
  }

  function selectInterval(val: number) {
    selectedInterval = val;
    timeRemaining = val;
    saveInterval(val);
    dropdownOpen = false;
    restartTimer();
  }

  function startTimer() {
    stopTimer();
    tickId = setInterval(() => {
      if (paused) return;
      timeRemaining -= 1000;
      if (timeRemaining <= 0) {
        timeRemaining = selectedInterval;
        onRefresh();
      }
    }, 1000);
  }

  function stopTimer() {
    if (tickId) { clearInterval(tickId); tickId = null; }
    if (intervalId) { clearInterval(intervalId); intervalId = null; }
  }

  function restartTimer() {
    stopTimer();
    startTimer();
  }

  function handleManualRefresh() {
    timeRemaining = selectedInterval;
    onRefresh();
    restartTimer();
  }

  onMount(() => {
    loadInterval();
    timeRemaining = selectedInterval;
    startTimer();
  });

  onDestroy(() => {
    stopTimer();
  });

  // Pause/resume effect
  $effect(() => {
    if (paused) {
      stopTimer();
    } else if (!tickId) {
      startTimer();
    }
  });
</script>

<div class="flex items-center gap-2">
  <!-- Countdown ring + interval selector -->
  <div class="relative">
    <button
      type="button"
      onclick={() => (dropdownOpen = !dropdownOpen)}
      class="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-sm font-medium rounded-md bg-white border border-slate-200 text-slate-600 hover:bg-slate-50 transition-colors"
      aria-label="Change refresh interval"
      aria-expanded={dropdownOpen}
    >
      <!-- SVG Ring -->
      <svg width="20" height="20" viewBox="0 0 22 22" class="flex-shrink-0">
        <!-- Background circle -->
        <circle cx="11" cy="11" r={RADIUS} fill="none" stroke="#e2e8f0" stroke-width="2" />
        <!-- Progress circle -->
        <circle
          cx="11" cy="11" r={RADIUS}
          fill="none"
          stroke="#64748b"
          stroke-width="2"
          stroke-linecap="round"
          stroke-dasharray={CIRCUMFERENCE}
          stroke-dashoffset={strokeDashoffset}
          transform="rotate(-90 11 11)"
          class="transition-[stroke-dashoffset] duration-1000 ease-linear"
        />
      </svg>
      <span class="text-xs text-slate-500">{selectedLabel}</span>
      <ChevronDown class="w-3 h-3 text-slate-400" />
    </button>

    <!-- Dropdown -->
    {#if dropdownOpen}
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="absolute top-full left-0 mt-1 bg-white border border-slate-200 rounded-md shadow-lg z-50 py-1 min-w-[80px]"
        role="menu"
      >
        {#each INTERVALS as option}
          <button
            type="button"
            role="menuitem"
            onclick={() => selectInterval(option.value)}
            class="w-full text-left px-3 py-1.5 text-sm hover:bg-slate-50 transition-colors
              {option.value === selectedInterval ? 'text-slate-800 font-medium bg-slate-50' : 'text-slate-600'}"
          >
            {option.label}
          </button>
        {/each}
      </div>
      <!-- Backdrop to close dropdown -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="fixed inset-0 z-40"
        onclick={() => (dropdownOpen = false)}
        onkeydown={(e) => { if (e.key === 'Escape') dropdownOpen = false; }}
      ></div>
    {/if}
  </div>

  <!-- Refresh button -->
  <button
    type="button"
    onclick={handleManualRefresh}
    class="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium rounded-md bg-white border border-slate-200 text-slate-600 hover:bg-slate-50 transition-colors"
    aria-label="Refresh now"
  >
    <RefreshCw class="w-4 h-4" />
    Refresh
  </button>
</div>
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -5`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/sessions/RefreshTimer.svelte
git commit -m "feat(frontend): add RefreshTimer component with countdown ring and interval selector"
```

---

### Task 6: Frontend — Integrate RefreshTimer into Sessions Page + Rename Tab

**Files:**
- Modify: `frontend/src/routes/sessions/+page.svelte`
- Modify: `frontend/src/routes/sessions/ActiveSessionsTab.svelte`

- [ ] **Step 1: Update `+page.svelte`**

Replace the imports and timer logic. Changes needed:

1. Add import: `import RefreshTimer from './RefreshTimer.svelte';`
2. Remove: `let refreshInterval`, `let tickInterval`, `let nowTick`, `let lastRefreshed`, and the `updatedAgoText` derived.
3. Remove from `onMount`: `refreshInterval = setInterval(...)` and `tickInterval = setInterval(...)`
4. Remove from `onDestroy`: the clearInterval calls for `refreshInterval` and `tickInterval`
5. Rename tab label from `'Active Sessions'` to `'Active Users'`.
6. Add `let timerPaused = $derived(pendingSessionAction !== null || disconnectUserTarget !== null);`
7. Replace the page header refresh area with the RefreshTimer component.

The page header section should become:

```svelte
<!-- Page header -->
<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
  <h1 class="text-2xl font-bold text-slate-800">VPN Sessions</h1>
  <RefreshTimer onRefresh={refreshData} paused={timerPaused} />
</div>
```

The tabs array should become:

```javascript
const tabs = $derived([
  { id: 'active', label: 'Active Users' },
  { id: 'all', label: 'All Sessions' },
  { id: 'events', label: 'Events' },
  { id: 'routes', label: 'Routes' },
  { id: 'bans', label: 'Bans', badge: serverStatus?.bannedIps ?? 0 }
]);
```

The `refreshData` function simplifies (no `lastRefreshed` update needed):

```javascript
async function refreshData() {
  await loadServerStatus();
  refreshSignal++;
}
```

- [ ] **Step 2: Update `ActiveSessionsTab.svelte` title**

Change the CardTable title from `"Active sessions ({users.length})"` to `"Active Users ({users.length})"`.

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -5`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/sessions/+page.svelte frontend/src/routes/sessions/ActiveSessionsTab.svelte
git commit -m "feat(frontend): integrate RefreshTimer and rename Active Sessions to Active Users"
```

---

### Task 7: Frontend — All Sessions Click-to-Detail

**Files:**
- Modify: `frontend/src/routes/sessions/AllSessionsTab.svelte`

- [ ] **Step 1: Add click-to-detail state and logic**

Update `AllSessionsTab.svelte` with:

1. Import `SessionDetailDrawer` and `getOcservSessionDetail` and `type OcservSessionDetail`.
2. Add state for detail panel: `selectedSession`, `sessionDetail`, `detailLoading`, `detailError`.
3. Add click handler on rows.
4. Add `SessionDetailDrawer` rendering.
5. Add `isOwner` prop and action callbacks for disconnect/terminate.

Updated `<script>` block additions after existing state:

```typescript
import SessionDetailDrawer from './SessionDetailDrawer.svelte';
import { getOcservSessionDetail, type OcservSessionDetail } from '$lib/api';

// Add to Props type:
type Props = {
  refreshSignal: number;
  isOwner: boolean;
  onDisconnectSession: (session: { id: string; username: string }) => void;
  onTerminateSession: (session: { id: string; username: string }) => void;
  onDisconnectUser: (username: string) => void;
};

let { refreshSignal, isOwner, onDisconnectSession, onTerminateSession, onDisconnectUser }: Props = $props();

// Detail panel state
let selectedSession = $state<OcservSession | null>(null);
let sessionDetail = $state<OcservSessionDetail | null>(null);
let detailLoading = $state(false);
let detailError = $state('');

async function selectSession(session: OcservSession) {
  selectedSession = session;
  sessionDetail = null;
  detailError = '';
  detailLoading = true;
  try {
    sessionDetail = await getOcservSessionDetail(session.id);
  } catch (caught) {
    if (caught instanceof ApiError && caught.status === 404) {
      detailError = 'Session no longer active.';
    } else {
      detailError = caught instanceof Error ? caught.message : 'Unable to load session detail.';
    }
  } finally {
    detailLoading = false;
  }
}

function closePanel() {
  selectedSession = null;
  sessionDetail = null;
  detailError = '';
}
```

Update the template to wrap in a flex layout when a session is selected, make rows clickable with `onclick={() => selectSession(session)}` and `class:bg-blue-50={selectedSession?.id === session.id}` and `class="hover:bg-slate-50 cursor-pointer transition-colors"`, and render the drawer:

```svelte
{#if selectedSession !== null}
  <SessionDetailDrawer
    session={selectedSession}
    detail={sessionDetail}
    userConnection={null}
    loading={detailLoading}
    error={detailError}
    sessionGone={detailError === 'Session no longer active.'}
    {isOwner}
    onClose={closePanel}
    onDisconnectSession={(s) => onDisconnectSession({ id: s.id, username: s.username })}
    onTerminateSession={(s) => onTerminateSession({ id: s.id, username: s.username })}
    {onDisconnectUser}
  />
{/if}
```

- [ ] **Step 2: Update `+page.svelte` to pass new props to AllSessionsTab**

Change the AllSessionsTab usage in `+page.svelte` from:
```svelte
<AllSessionsTab {refreshSignal} />
```
to:
```svelte
<AllSessionsTab
  {refreshSignal}
  {isOwner}
  onDisconnectSession={confirmDisconnectSession}
  onTerminateSession={confirmTerminateSession}
  onDisconnectUser={confirmDisconnectUser}
/>
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -5`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/sessions/AllSessionsTab.svelte frontend/src/routes/sessions/+page.svelte
git commit -m "feat(frontend): add click-to-detail drawer in All Sessions tab"
```

---

### Task 8: Frontend — Routes Click-to-Detail

**Files:**
- Modify: `frontend/src/routes/sessions/RoutesTab.svelte`

- [ ] **Step 1: Add click-to-detail state and logic**

Update `RoutesTab.svelte`:

1. Import `SessionDetailDrawer`, `getConnectionById`, `ApiError`, `type UserConnection`.
2. Add `isOwner` prop.
3. Add state: `selectedRoute`, `connectionDetail`, `detailLoading`, `detailError`.
4. Add click handler on rows.
5. Render `SessionDetailDrawer` in side-panel mode.

Updated script additions:

```typescript
import SessionDetailDrawer from './SessionDetailDrawer.svelte';
import { getConnectionById, ApiError, type UserConnection } from '$lib/api';

type Props = {
  refreshSignal: number;
  isOwner: boolean;
};

let { refreshSignal, isOwner }: Props = $props();

// Detail panel state
let selectedRoute = $state<UserRoute | null>(null);
let connectionDetail = $state<UserConnection | null>(null);
let detailLoading = $state(false);
let detailError = $state('');

async function selectRoute(route: UserRoute) {
  selectedRoute = route;
  connectionDetail = null;
  detailError = '';
  detailLoading = true;
  try {
    connectionDetail = await getConnectionById(Number(route.id));
  } catch (caught) {
    if (caught instanceof ApiError && caught.status === 404) {
      detailError = 'Connection no longer active.';
    } else {
      detailError = caught instanceof Error ? caught.message : 'Unable to load connection detail.';
    }
  } finally {
    detailLoading = false;
  }
}

function closePanel() {
  selectedRoute = null;
  connectionDetail = null;
  detailError = '';
}
```

Make rows clickable: `onclick={() => selectRoute(route)}` with `cursor-pointer` and `bg-blue-50` for selected.

Render drawer when selected:

```svelte
{#if selectedRoute !== null}
  <SessionDetailDrawer
    session={selectedRoute ? { id: selectedRoute.id, username: selectedRoute.username, remoteAddress: '', vpnAddress: selectedRoute.vpnIp, connectedAt: undefined, rxBytes: 0, txBytes: 0, userAgent: '', device: selectedRoute.device, reconnectCount: 0 } : undefined}
    detail={null}
    userConnection={connectionDetail}
    loading={detailLoading}
    error={detailError}
    sessionGone={detailError === 'Connection no longer active.'}
    {isOwner}
    onClose={closePanel}
    onDisconnectSession={() => {}}
    onTerminateSession={() => {}}
    onDisconnectUser={() => {}}
  />
{/if}
```

- [ ] **Step 2: Update `+page.svelte` to pass `isOwner` to RoutesTab**

Change:
```svelte
<RoutesTab {refreshSignal} />
```
to:
```svelte
<RoutesTab {refreshSignal} {isOwner} />
```

- [ ] **Step 3: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -5`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/sessions/RoutesTab.svelte frontend/src/routes/sessions/+page.svelte
git commit -m "feat(frontend): add click-to-detail drawer in Routes tab via show id"
```

---

### Task 9: Frontend — Events Pagination

**Files:**
- Modify: `frontend/src/routes/sessions/EventsTab.svelte`

- [ ] **Step 1: Add pagination**

Update `EventsTab.svelte`:

1. Import `PaginatedTable` and `paginate` utility.
2. Add `page` state.
3. Add derived `sortedEvents` (newest first) and `pagedEvents`.
4. Wrap table in `PaginatedTable`.

Updated script additions:

```typescript
import PaginatedTable from '$lib/components/ui/PaginatedTable.svelte';
import { paginate } from '$lib/utils/pagination';

const PAGE_SIZE = 20;
let page = $state(1);

const sortedEvents = $derived((() => {
  const copy = [...events];
  copy.sort((a, b) => {
    const ta = a.timestamp ? new Date(a.timestamp).getTime() : 0;
    const tb = b.timestamp ? new Date(b.timestamp).getTime() : 0;
    return tb - ta; // newest first
  });
  return copy;
})());

const pagedEvents = $derived(paginate(sortedEvents, page, PAGE_SIZE));
```

Replace `{#each events as event}` with `{#each pagedEvents.items as event}`.

Wrap the `<table>` in:
```svelte
<PaginatedTable
  totalPages={pagedEvents.totalPages}
  currentPage={pagedEvents.page}
  onPageChange={(p) => (page = p)}
>
  <table>...</table>
</PaginatedTable>
```

Update the CardTable title to include count: `"Connection Events ({events.length})"`.

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -5`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/sessions/EventsTab.svelte
git commit -m "feat(frontend): add pagination and sorting to Events tab"
```

---

### Task 10: Frontend — Bans Status Badges

**Files:**
- Modify: `frontend/src/routes/sessions/BansTab.svelte`

- [ ] **Step 1: Add status badges**

Update `BansTab.svelte`:

1. Add a derived `bannedSet` that cross-references banned IPs.
2. Add a "Status" column to the ban points table.
3. Add unban button inline for banned IPs in the points table.

Add derived:

```typescript
const bannedSet = $derived(new Set(bans.map(b => b.ip)));
```

In the "IP Ban Points" table, add a Status column header:

```svelte
<th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Status</th>
```

For each ban point entry row, add:

```svelte
<td class="px-4 py-3">
  {#if bannedSet.has(entry.ip)}
    <span class="inline-flex px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-700">Banned</span>
  {:else}
    <span class="inline-flex px-2 py-0.5 rounded text-xs font-medium bg-amber-100 text-amber-700">Warning</span>
  {/if}
</td>
```

Add an Action column that shows unban for banned IPs:

```svelte
{#if isOwner}
  <th class="px-4 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider bg-slate-50">Action</th>
{/if}
```

And in the row:

```svelte
{#if isOwner}
  <td class="px-4 py-3">
    {#if bannedSet.has(entry.ip)}
      <button
        type="button"
        onclick={() => confirmUnban(entry.ip)}
        class="px-2 py-1 text-xs font-medium rounded bg-blue-100 text-blue-700 hover:bg-blue-200 transition-colors"
      >
        Unban
      </button>
    {/if}
  </td>
{/if}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -5`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/sessions/BansTab.svelte
git commit -m "feat(frontend): add ban status badges and inline unban in Ban Points table"
```

---

### Task 11: Full Build Verification

**Files:** None (verification only)

- [ ] **Step 1: Run backend tests**

Run: `cd /opt/ocserv_panel/backend && go test ./...`
Expected: All tests pass.

- [ ] **Step 2: Run frontend type check**

Run: `cd /opt/ocserv_panel/frontend && npx svelte-check --tsconfig ./tsconfig.json`
Expected: No errors.

- [ ] **Step 3: Run frontend build**

Run: `cd /opt/ocserv_panel/frontend && npm run build`
Expected: Build succeeds.

- [ ] **Step 4: Run backend build**

Run: `cd /opt/ocserv_panel/backend && go build -o /dev/null ./cmd/server`
Expected: Build succeeds.
