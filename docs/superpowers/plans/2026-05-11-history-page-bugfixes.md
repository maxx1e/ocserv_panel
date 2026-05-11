# History Page Bug Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix six bugs and functional gaps on the History page: active-session display, drawer bar charts, card removal, Users tab 404, Bans time window, and VPN per-interface monitoring lines.

**Architecture:** Backend fixes first (Tasks 1–3), then frontend (Tasks 4–7). Backend and frontend tasks are independent within their tier and can be executed by separate sub-agents. The frontend VPN task (Task 7) depends on the backend having emitted per-interface series (Task 3) — implement Task 3 before Task 7.

**Tech Stack:** Go 1.25 (backend), SvelteKit 5 + Svelte runes + Tailwind CSS v4 + LayerChart (frontend), SQLite via `database/sql`, tstorage via `monitor.TSDBStore` / `monitor.MemoryStore`.

---

## File Map

### Backend (Tasks 1–3)
| File | Change |
|------|--------|
| `backend/internal/server/server.go` | T1: remove 404 guard in `handleHistoryUserSessions`; T2: add `window` param to `handleHistoryBans`; T3: multi-series VPN in `handleMonitoringOverview` |
| `backend/internal/store/bans.go` | T2: add `since *time.Time` to `ListBanEvents` |
| `backend/internal/monitor/types.go` | T3: add `QueryAllLabels` to `Store` interface |
| `backend/internal/monitor/memory.go` | T3: implement `QueryAllLabels` on `MemoryStore` |
| `backend/internal/monitor/tsdb.go` | T3: implement `QueryAllLabels` on `TSDBStore` |
| `backend/internal/server/server_test.go` | T1, T2, T3: add/update tests |
| `backend/internal/store/store_test.go` | T2: add `ListBanEvents` with `since` test |
| `backend/internal/monitor/memory_test.go` | T3: add `QueryAllLabels` test |

### Frontend (Tasks 4–7)
| File | Change |
|------|--------|
| `frontend/src/lib/components/ui/MetricChart.svelte` | T5: add `chartType` prop |
| `frontend/src/lib/components/history/UserSessionDrawer.svelte` | T4: active-session label; T5: bar charts |
| `frontend/src/lib/api/history.ts` | T6: `listHistoricalBans` adds `window` param |
| `frontend/src/lib/monitoring.ts` | T7: add `allSeriesForMetric` helper |
| `frontend/src/routes/history/+page.svelte` | T4: active-session label in sessions table; T6: bans window selector; T7: multi-line VPN; T8: remove summary cards |

---

## Task 1: Fix Users tab 404 — return 200 empty array

**Files:**
- Modify: `backend/internal/server/server.go` (function `handleHistoryUserSessions`, ~line 600)
- Modify: `backend/internal/server/server_test.go`

- [ ] **Step 1: Write the failing test**

Add this test to `backend/internal/server/server_test.go`:

```go
func TestHistoryUserSessionsReturnsEmptyArrayWhenNoSessions(t *testing.T) {
	h := newTestHarness(t)
	cookie := h.createOwnerAndLogin(t)

	req := httptest.NewRequest(http.MethodGet, "/api/history/users/nobody?window=7d", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookie})
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty array, got %d items", len(result))
	}
}
```

> Note: `createOwnerAndLogin` is a helper used in existing tests — check the file for its exact name. If it does not exist, use the pattern: `h.createOwner(t, "admin", "Password1!")` then `cookie := h.login(t, "admin", "Password1!")`.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/server/ -run TestHistoryUserSessionsReturnsEmptyArrayWhenNoSessions -v
```

Expected: FAIL — test gets `404`, not `200`.

- [ ] **Step 3: Remove the 404 guard**

In `backend/internal/server/server.go`, find `handleHistoryUserSessions` and delete the block that returns 404 for empty sessions. The section to remove is:

```go
	if len(sessions) == 0 {
		writeError(w, http.StatusNotFound, "no sessions found for user in this window")
		return
	}
```

The handler should fall through to `writeJSON(w, http.StatusOK, resp)` with an empty `resp` slice.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/server/ -run TestHistoryUserSessionsReturnsEmptyArrayWhenNoSessions -v
```

Expected: PASS.

- [ ] **Step 5: Run full backend test suite**

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1 | tail -20
```

Expected: all packages pass, no regressions.

- [ ] **Step 6: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/server/server.go backend/internal/server/server_test.go
git commit -m "fix(backend): return 200 empty array from history user sessions when no data"
```

---

## Task 2: Bans tab — add time window filter to backend

**Files:**
- Modify: `backend/internal/store/bans.go`
- Modify: `backend/internal/server/server.go` (function `handleHistoryBans`, ~line 748)
- Modify: `backend/internal/store/store_test.go`
- Modify: `backend/internal/server/server_test.go`

- [ ] **Step 1: Write store test**

Add to `backend/internal/store/store_test.go`:

```go
func TestListBanEvents_WithSinceFilter(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour)
	recent := now.Add(-1 * time.Hour)

	if err := s.InsertBanEvent(ctx, "1.1.1.1", 100, "old", old); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := s.InsertBanEvent(ctx, "2.2.2.2", 50, "recent", recent); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// No filter — should return both
	all, total, err := s.ListBanEvents(ctx, nil, 10, 0)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 2 {
		t.Fatalf("want total=2, got %d", total)
	}
	_ = all

	// Filter to last 24h — should return only the recent one
	since := now.Add(-24 * time.Hour)
	filtered, filteredTotal, err := s.ListBanEvents(ctx, &since, 10, 0)
	if err != nil {
		t.Fatalf("list filtered: %v", err)
	}
	if filteredTotal != 1 {
		t.Fatalf("want filtered total=1, got %d", filteredTotal)
	}
	if filtered[0].IP != "2.2.2.2" {
		t.Fatalf("want IP 2.2.2.2, got %s", filtered[0].IP)
	}
}
```

- [ ] **Step 2: Run store test to verify it fails**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/store/ -run TestListBanEvents_WithSinceFilter -v
```

Expected: FAIL — compilation error because `ListBanEvents` does not accept a `since` param yet.

- [ ] **Step 3: Update `ListBanEvents` in store**

In `backend/internal/store/bans.go`, change the function signature and queries:

```go
// ListBanEvents returns ban events newest-first with total count.
// When since is non-nil, only events with banned_at >= since are returned.
func (s *Store) ListBanEvents(ctx context.Context, since *time.Time, limit, offset int) ([]BanEvent, int64, error) {
	var total int64
	var countErr error
	if since != nil {
		sinceStr := since.UTC().Format(time.RFC3339)
		countErr = s.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM ban_events WHERE banned_at >= ?`, sinceStr,
		).Scan(&total)
	} else {
		countErr = s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM ban_events`).Scan(&total)
	}
	if countErr != nil {
		return nil, 0, fmt.Errorf("count ban events: %w", countErr)
	}

	var rows *sql.Rows
	var err error
	if since != nil {
		sinceStr := since.UTC().Format(time.RFC3339)
		rows, err = s.DB.QueryContext(ctx,
			`SELECT id, ip, score, reason, banned_at, unbanned_at
			 FROM ban_events WHERE banned_at >= ?
			 ORDER BY banned_at DESC, id DESC LIMIT ? OFFSET ?`,
			sinceStr, limit, offset,
		)
	} else {
		rows, err = s.DB.QueryContext(ctx,
			`SELECT id, ip, score, reason, banned_at, unbanned_at
			 FROM ban_events ORDER BY banned_at DESC, id DESC LIMIT ? OFFSET ?`,
			limit, offset,
		)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("query ban events: %w", err)
	}
	defer rows.Close()

	events := make([]BanEvent, 0)
	for rows.Next() {
		var e BanEvent
		var bannedAt string
		var unbannedAt *string
		if err := rows.Scan(&e.ID, &e.IP, &e.Score, &e.Reason, &bannedAt, &unbannedAt); err != nil {
			return nil, 0, fmt.Errorf("scan ban event: %w", err)
		}
		parsed, err := parseTime(bannedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("parse banned_at: %w", err)
		}
		e.BannedAt = parsed
		if unbannedAt != nil {
			t, err := parseTime(*unbannedAt)
			if err != nil {
				return nil, 0, fmt.Errorf("parse unbanned_at: %w", err)
			}
			e.UnbannedAt = &t
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate ban events: %w", err)
	}
	return events, total, nil
}
```

- [ ] **Step 4: Fix all callers of `ListBanEvents`**

Search for all uses of `ListBanEvents`:

```bash
grep -rn "ListBanEvents" /opt/ocserv_panel/backend/
```

Update `handleHistoryBans` in `server.go` to pass `since`. Replace the existing function body with:

```go
func (a *App) handleHistoryBans(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	page := 1
	if p, err := strconv.Atoi(queryValueDefault(r, "page", "1")); err == nil && p > 0 {
		page = p
	}
	limit := 25
	if l, err := strconv.Atoi(queryValueDefault(r, "limit", "25")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	offset := (page - 1) * limit

	window := queryValueDefault(r, "window", "all")
	var since *time.Time
	if window != "all" {
		t := a.now().UTC().Add(-historyWindowDuration(window))
		since = &t
	}

	events, total, err := a.store.ListBanEvents(r.Context(), since, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]banEventResponse, 0, len(events))
	for _, e := range events {
		resp = append(resp, banEventResponse{
			ID:         e.ID,
			IP:         e.IP,
			Score:      e.Score,
			Reason:     e.Reason,
			BannedAt:   e.BannedAt,
			UnbannedAt: e.UnbannedAt,
		})
	}
	writeJSON(w, http.StatusOK, listBanEventsResponse{
		Events: resp,
		Total:  total,
		Page:   page,
		Limit:  limit,
	})
}
```

Also update any other call sites found in the grep above (e.g. in tests) to pass `nil` as the `since` argument.

- [ ] **Step 5: Write handler test**

Add to `backend/internal/server/server_test.go`:

```go
func TestHistoryBansWindowFilter(t *testing.T) {
	h := newTestHarness(t)
	ctx := context.Background()
	now := time.Now().UTC()

	// Insert an old ban (48h ago) and a recent ban (1h ago)
	if err := h.store.InsertBanEvent(ctx, "10.0.0.1", 100, "old", now.Add(-48*time.Hour)); err != nil {
		t.Fatalf("insert old ban: %v", err)
	}
	if err := h.store.InsertBanEvent(ctx, "10.0.0.2", 50, "recent", now.Add(-1*time.Hour)); err != nil {
		t.Fatalf("insert recent ban: %v", err)
	}

	cookie := h.createOwnerAndLogin(t)

	// window=24h should return only the recent ban
	req := httptest.NewRequest(http.MethodGet, "/api/history/bans?window=24h", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookie})
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var page struct {
		Events []struct{ IP string } `json:"events"`
		Total  int64                 `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if page.Total != 1 {
		t.Fatalf("want total=1, got %d", page.Total)
	}
	if page.Events[0].IP != "10.0.0.2" {
		t.Fatalf("want IP 10.0.0.2, got %s", page.Events[0].IP)
	}

	// window=all should return both bans
	req2 := httptest.NewRequest(http.MethodGet, "/api/history/bans?window=all", nil)
	req2.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookie})
	w2 := httptest.NewRecorder()
	h.router.ServeHTTP(w2, req2)
	var page2 struct{ Total int64 `json:"total"` }
	json.Unmarshal(w2.Body.Bytes(), &page2)
	if page2.Total != 2 {
		t.Fatalf("want total=2, got %d", page2.Total)
	}
}
```

> Note: replace `createOwnerAndLogin` with the correct helper name from the test file.

- [ ] **Step 6: Run tests**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/store/ -run TestListBanEvents -v && go test ./internal/server/ -run TestHistoryBans -v
```

Expected: all pass.

- [ ] **Step 7: Run full backend test suite**

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1 | tail -20
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
cd /opt/ocserv_panel && git add backend/internal/store/bans.go backend/internal/server/server.go backend/internal/store/store_test.go backend/internal/server/server_test.go
git commit -m "feat(backend): add time window filter to history bans endpoint"
```

---

## Task 3: VPN multi-interface series in monitoring overview

**Files:**
- Modify: `backend/internal/monitor/types.go`
- Modify: `backend/internal/monitor/memory.go`
- Modify: `backend/internal/monitor/tsdb.go`
- Modify: `backend/internal/server/server.go` (function `handleMonitoringOverview`)
- Modify: `backend/internal/monitor/memory_test.go`
- Modify: `backend/internal/server/server_test.go`

- [ ] **Step 1: Write `QueryAllLabels` test for MemoryStore**

Add to `backend/internal/monitor/memory_test.go`:

```go
func TestMemoryStore_QueryAllLabels(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	now := time.Now().UTC()
	err := s.Append(ctx, []Sample{
		{Name: MetricVPNInterfaceRXBytes, Labels: map[string]string{"interface": "vpns0"}, Timestamp: now, Value: 1.0},
		{Name: MetricVPNInterfaceRXBytes, Labels: map[string]string{"interface": "vpns1"}, Timestamp: now, Value: 2.0},
		{Name: MetricVPNInterfaceRXBytes, Labels: map[string]string{"interface": "vpns0"}, Timestamp: now.Add(-time.Minute), Value: 0.5},
		{Name: MetricHostCPUUtilization, Labels: map[string]string{}, Timestamp: now, Value: 50.0},
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}

	labels, err := s.QueryAllLabels(ctx, MetricVPNInterfaceRXBytes)
	if err != nil {
		t.Fatalf("QueryAllLabels: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("want 2 distinct label sets, got %d", len(labels))
	}

	// Collect interface names
	ifaceNames := make(map[string]bool)
	for _, lbl := range labels {
		ifaceNames[lbl["interface"]] = true
	}
	if !ifaceNames["vpns0"] || !ifaceNames["vpns1"] {
		t.Fatalf("expected vpns0 and vpns1, got %v", ifaceNames)
	}
}

func TestMemoryStore_QueryAllLabels_Empty(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	labels, err := s.QueryAllLabels(ctx, MetricVPNInterfaceRXBytes)
	if err != nil {
		t.Fatalf("QueryAllLabels: %v", err)
	}
	if len(labels) != 0 {
		t.Fatalf("want 0 label sets, got %d", len(labels))
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/monitor/ -run TestMemoryStore_QueryAllLabels -v
```

Expected: FAIL — compilation error, `QueryAllLabels` not defined.

- [ ] **Step 3: Add `QueryAllLabels` to `Store` interface**

In `backend/internal/monitor/types.go`, add to the `Store` interface:

```go
type Store interface {
	Append(ctx context.Context, samples []Sample) error
	QueryRange(ctx context.Context, query Query) (Series, error)
	QueryLatestLabels(ctx context.Context, metric MetricName) (map[string]string, error)
	QueryAllLabels(ctx context.Context, metric MetricName) ([]map[string]string, error)
	Close() error
}
```

- [ ] **Step 4: Implement `QueryAllLabels` on `MemoryStore`**

Add to `backend/internal/monitor/memory.go`, after `QueryLatestLabels`:

```go
// QueryAllLabels returns all distinct label sets seen for metric across all stored samples.
func (s *MemoryStore) QueryAllLabels(ctx context.Context, metric MetricName) ([]map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrStoreClosed
	}

	seen := make(map[string]map[string]string)
	for i := range s.samples {
		sample := &s.samples[i]
		if sample.Name != metric {
			continue
		}
		key := labelsKey(sample.Labels)
		if _, ok := seen[key]; !ok {
			seen[key] = cloneLabels(sample.Labels)
		}
	}

	result := make([]map[string]string, 0, len(seen))
	for _, lbls := range seen {
		result = append(result, lbls)
	}
	return result, nil
}
```

Add a helper function at the bottom of `memory.go`:

```go
// labelsKey produces a stable string key for a label map, used for deduplication.
func labelsKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
		b.WriteByte(',')
	}
	return b.String()
}
```

Add required imports at top of `memory.go` if not present: `"sort"` and `"strings"`.

- [ ] **Step 5: Implement `QueryAllLabels` on `TSDBStore`**

Add to `backend/internal/monitor/tsdb.go`, after `QueryLatestLabels`:

```go
// QueryAllLabels returns all distinct label sets seen for metric in the last hour.
func (s *TSDBStore) QueryAllLabels(ctx context.Context, metric MetricName) ([]map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, ErrStoreClosed
	}

	now := time.Now().UTC()
	querier, err := s.db.Querier(now.Add(-time.Hour).UnixMilli(), now.UnixMilli())
	if err != nil {
		return []map[string]string{}, nil
	}
	defer querier.Close()

	matchers := []*labels.Matcher{
		labels.MustNewMatcher(labels.MatchEqual, labels.MetricName, string(metric)),
	}
	sets := querier.Select(ctx, false, nil, matchers...)

	seen := make(map[string]map[string]string)
	for sets.Next() {
		series := sets.At()
		lbls := series.Labels()
		result := make(map[string]string, lbls.Len())
		lbls.Range(func(l labels.Label) {
			if l.Name != labels.MetricName {
				result[l.Name] = l.Value
			}
		})
		key := labelsKey(result)
		if _, ok := seen[key]; !ok {
			seen[key] = result
		}
	}
	if err := sets.Err(); err != nil {
		return nil, fmt.Errorf("select tsdb series for all labels: %w", err)
	}

	out := make([]map[string]string, 0, len(seen))
	for _, lbls := range seen {
		out = append(out, lbls)
	}
	return out, nil
}
```

> Note: `labelsKey` must be accessible from tsdb.go. Add it to `types.go` (shared file) rather than memory.go, or duplicate it in tsdb.go. Simplest: add it to `types.go` since both files are in the `monitor` package.

Move `labelsKey` from memory.go to `types.go` (bottom of the file). Remove it from memory.go.

- [ ] **Step 6: Run MemoryStore tests**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/monitor/ -run TestMemoryStore_QueryAllLabels -v
```

Expected: PASS.

- [ ] **Step 7: Update `handleMonitoringOverview` to emit per-interface VPN series**

In `backend/internal/server/server.go`, replace the `handleMonitoringOverview` function body with:

```go
func (a *App) handleMonitoringOverview(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	window := queryValueDefault(r, "window", "1h")
	step := defaultMonitoringStep(window)
	windowDuration, stepDuration, err := monitor.ValidateWindowAndStep(window, step)
	if err != nil {
		a.writeAppError(w, err)
		return
	}

	now := a.now().UTC()
	series := make([]monitor.Series, 0, len(monitoringOverviewMetrics))

	for _, name := range monitoringOverviewMetrics {
		if name == monitor.MetricVPNInterfaceRXBytes || name == monitor.MetricVPNInterfaceTXBytes {
			// Emit one series per discovered VPN interface.
			allLabels, err := a.metricStore.QueryAllLabels(r.Context(), name)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			if len(allLabels) == 0 {
				// No VPN data yet — emit empty series with empty labels so the
				// frontend can show the unavailable state.
				item, err := a.metricStore.QueryRange(r.Context(), monitor.Query{
					Metric: name,
					Labels: map[string]string{},
					Start:  now.Add(-windowDuration),
					End:    now,
					Step:   stepDuration,
				})
				if err != nil {
					writeError(w, http.StatusInternalServerError, "internal server error")
					return
				}
				series = append(series, item)
			}
			for _, lbls := range allLabels {
				item, err := a.metricStore.QueryRange(r.Context(), monitor.Query{
					Metric: name,
					Labels: lbls,
					Start:  now.Add(-windowDuration),
					End:    now,
					Step:   stepDuration,
				})
				if err != nil {
					writeError(w, http.StatusInternalServerError, "internal server error")
					return
				}
				series = append(series, item)
			}
			continue
		}

		item, err := a.metricStore.QueryRange(r.Context(), monitor.Query{
			Metric: name,
			Labels: map[string]string{},
			Start:  now.Add(-windowDuration),
			End:    now,
			Step:   stepDuration,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		series = append(series, item)
	}

	writeJSON(w, http.StatusOK, monitoringOverviewResponse{
		Window:      window,
		GeneratedAt: now,
		Catalog:     a.catalogList(),
		Series:      series,
	})
}
```

- [ ] **Step 8: Write server test for multi-interface VPN series**

Add to `backend/internal/server/server_test.go`:

```go
func TestMonitoringOverviewReturnsPerInterfaceVPNSeries(t *testing.T) {
	h := newTestHarness(t)
	cookie := h.createOwnerAndLogin(t)

	// Seed the MemoryStore with two VPN interfaces.
	// Access memStore via a type assertion on the router — or seed through a helper.
	// Since testHarness does not expose memStore directly, we need to seed via Append.
	// The testHarness exposes h.router; we need to access memStore.
	// Workaround: use a custom harness that exposes the store.
	// See newTestHarnessWithMemStore below.

	// For now: create a harness with an accessible memStore.
	hh := newTestHarnessWithMemStore(t)
	cookie2 := hh.createOwnerAndLogin(t)

	now := time.Now().UTC()
	ctx := context.Background()
	err := hh.memStore.Append(ctx, []monitor.Sample{
		{Name: monitor.MetricVPNInterfaceRXBytes, Labels: map[string]string{"interface": "vpns0"}, Timestamp: now.Add(-30 * time.Second), Value: 100},
		{Name: monitor.MetricVPNInterfaceRXBytes, Labels: map[string]string{"interface": "vpns1"}, Timestamp: now.Add(-30 * time.Second), Value: 200},
	})
	if err != nil {
		t.Fatalf("append: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/monitoring/overview?window=1h", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookie2})
	w := httptest.NewRecorder()
	hh.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Series []struct {
			Metric string            `json:"metric"`
			Labels map[string]string `json:"labels"`
		} `json:"series"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var vpnRXCount int
	for _, s := range resp.Series {
		if s.Metric == string(monitor.MetricVPNInterfaceRXBytes) {
			vpnRXCount++
		}
	}
	if vpnRXCount != 2 {
		t.Fatalf("expected 2 vpn_interface_rx_bytes series (one per interface), got %d", vpnRXCount)
	}
	_ = cookie // suppress unused
}
```

Add the helper struct and constructor in `server_test.go`:

```go
type testHarnessWithMemStore struct {
	*testHarness
	memStore *monitor.MemoryStore
}

func newTestHarnessWithMemStore(t *testing.T) *testHarnessWithMemStore {
	t.Helper()
	// Build from scratch with the same wiring as newTestHarnessWithCookieSecure
	// but expose the MemoryStore.
	ctx := context.Background()
	database, err := db.OpenSQLite(ctx, filepath.Join(t.TempDir(), "panel.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.ApplyMigrations(ctx, database, migrationsDir(t)); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	s := store.New(database)
	recorder := audit.NewSQLiteRecorder(s)
	authService := app.NewAuthService(s, recorder)
	dashboardService := app.NewDashboardService(s, recorder, rbac.NewDefaultAuthorizer(), nil)
	memStore := monitor.NewMemoryStore()
	catalog := monitor.DefaultCatalog()
	cfg := config.Defaults()
	settingsService := app.NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())
	ocservSvc, fakeManager := newTestOcservServiceWithFake(s, recorder)

	testCfg := config.Defaults()
	testCfgFn := func() config.Config { return testCfg }
	fakeRunner2 := exec.NewFakeCommandRunner()
	fakeSvcMgr2 := service.NewFakeServiceManager(service.ServiceInactive)
	testChecker := health.NewReadinessChecker(func() health.ReadinessConfig { return health.ReadinessConfig{} }, fakeRunner2, fakeSvcMgr2)
	confDir := t.TempDir()
	confPath := filepath.Join(confDir, "ocserv.conf")
	os.WriteFile(confPath, []byte("tcp-port = 443\n"), 0640)
	configFiles := vpnconfig.NewConfigFileManager(confPath, 10)
	configValidator := vpnconfig.NewConfigValidator(confPath)
	configReloader := vpnconfig.NewConfigReloader(fakeRunner2, "occtl")
	networkInspector := health.NewNetworkInspectorWithProcDir(fakeRunner2, confDir)
	configMgmtSvc := app.NewConfigManagementService(configFiles, configValidator, configReloader, networkInspector, vpnconfig.NewRestartDetector(), recorder, rbac.NewDefaultAuthorizer())
	_ = testCfgFn
	_ = testChecker

	base := &testHarness{
		database:   database,
		store:      s,
		ocservFake: fakeManager,
		router: NewAppRouter(
			authService,
			dashboardService,
			memStore,
			catalog,
			settingsService,
			ocservSvc,
			app.FakeLogFileReader{},
			configMgmtSvc,
			rbac.NewDefaultAuthorizer(),
			config.Defaults,
			false,
		),
	}
	return &testHarnessWithMemStore{testHarness: base, memStore: memStore}
}
```

> Note: verify the `NewAppRouter` signature matches what's in the codebase by checking how `newTestHarnessWithCookieSecure` calls it and mirror that exactly.

- [ ] **Step 9: Run tests**

```bash
cd /opt/ocserv_panel/backend && go test ./internal/monitor/ -v -run TestMemoryStore_QueryAllLabels && go test ./internal/server/ -v -run TestMonitoringOverviewReturnsPerInterface
```

Expected: both PASS.

- [ ] **Step 10: Run full backend test suite**

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1 | tail -20
```

Expected: all pass.

- [ ] **Step 11: Commit**

```bash
cd /opt/ocserv_panel && git add \
  backend/internal/monitor/types.go \
  backend/internal/monitor/memory.go \
  backend/internal/monitor/tsdb.go \
  backend/internal/monitor/memory_test.go \
  backend/internal/server/server.go \
  backend/internal/server/server_test.go
git commit -m "feat(backend): emit per-interface VPN series in monitoring overview"
```

---

## Task 4: Sessions tab and drawer — active session label

**Files:**
- Modify: `frontend/src/routes/history/+page.svelte`
- Modify: `frontend/src/lib/components/history/UserSessionDrawer.svelte`

**Context:** Sessions with `state === "authenticated"` or `state === "connected"` are still live. The `disconnectedAt` field is meaningless (set to ingester poll time). Show "Active" badge instead.

- [ ] **Step 1: Update the sessions table in `+page.svelte`**

Find the "Disconnected" column cell (around line 701):
```svelte
<td class="px-4 py-2 text-slate-600 text-xs dark:text-slate-400">{formatAbsoluteTime(session.disconnectedAt ?? session.endedAt)}</td>
```

Replace with:
```svelte
<td class="px-4 py-2 text-xs">
  {#if session.state === 'authenticated' || session.state === 'connected'}
    <span class="px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-700">Active</span>
  {:else}
    <span class="text-slate-600 dark:text-slate-400">{formatAbsoluteTime(session.disconnectedAt ?? session.endedAt)}</span>
  {/if}
</td>
```

- [ ] **Step 2: Update `UserSessionDrawer.svelte` — "Ended" column**

Find the "Ended" cell in the sessions table inside the drawer:
```svelte
<td class="px-4 py-2 text-slate-700" title={session.disconnectedAt ?? session.endedAt ?? ''}>
  {formatRelativeTime(session.disconnectedAt ?? session.endedAt)}
</td>
```

Replace with:
```svelte
<td class="px-4 py-2 text-slate-700">
  {#if (session.state === 'authenticated' || session.state === 'connected')}
    <span class="px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-700">Active</span>
  {:else}
    <span title={session.disconnectedAt ?? session.endedAt ?? ''}>{formatRelativeTime(session.disconnectedAt ?? session.endedAt)}</span>
  {/if}
</td>
```

- [ ] **Step 3: Build the frontend to verify no type errors**

```bash
cd /opt/ocserv_panel/frontend && pnpm check 2>&1 | tail -20
```

Expected: 0 errors.

- [ ] **Step 4: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/routes/history/+page.svelte frontend/src/lib/components/history/UserSessionDrawer.svelte
git commit -m "fix(frontend): show Active badge for live sessions in history sessions tab"
```

---

## Task 5: Drawer charts — switch to bar chart type

**Files:**
- Modify: `frontend/src/lib/components/ui/MetricChart.svelte`
- Modify: `frontend/src/lib/components/history/UserSessionDrawer.svelte`

**Context:** LayerChart exports a `Bars` component. For per-session discrete data, bar/column charts are more readable than lines.

- [ ] **Step 1: Add `chartType` prop to `MetricChart.svelte`**

Replace the `Props` type and destructuring:

```svelte
<script lang="ts">
  import { Area, Axis, Bars, Chart, Highlight, LinearGradient, Svg } from 'layerchart';
  import { Container as ChartContainer, Tooltip as ChartTooltip } from '$lib/components/ui/chart/index';
  import {
    formatMetricValue,
    formatXAxisTime,
    getChartClasses,
    getChartConfig,
    getChartColorVar,
    getXTickCount,
  } from '$lib/monitoring';
  import type { ChartConfig } from '$lib/components/ui/chart/chart-utils';

  type TimeData = { timestamp: Date; value: number };

  type Props = {
    data: TimeData[];
    metric: string;
    height?: number;
    chartType?: 'line' | 'bar';
  };

  let { data, metric, height = 160, chartType = 'line' }: Props = $props();

  const config = $derived(getChartConfig(metric));
  const classes = $derived(getChartClasses(metric));
  const xTickCount = $derived(getXTickCount(data.length));
  const chartColor = $derived(getChartColorVar(metric));

  const chartConfig: ChartConfig = $derived({
    value: { label: metric, color: chartColor },
  });

  function tooltipFormatter(value: number): string {
    return formatMetricValue(metric, value);
  }
</script>
```

Then in the template, replace the `<Svg>` block contents to switch between line and bar:

```svelte
<div style="height: {height}px" class="w-full">
  {#if data.length > 0}
    <ChartContainer config={chartConfig} class="h-full w-full">
      <Chart
        {data}
        x="timestamp"
        y="value"
        yDomain={config.yDomain}
        yNice={config.yNice}
        padding={{ left: 44, bottom: 24, top: 8, right: 8 }}
        tooltip={{ mode: 'bisect-x' }}
      >
        <Svg>
          <Axis placement="left" grid format={config.yAxisFormat} />
          <Axis
            placement="bottom"
            format={(d: Date | number) => formatXAxisTime(d)}
            ticks={xTickCount}
          />
          {#if chartType === 'bar'}
            <Bars class={classes.stroke.replace('stroke-', 'fill-')} radius={2} />
          {:else}
            <LinearGradient stops={[classes.gradientTop, classes.gradientBottom]} vertical>
              {#snippet children({ gradient }: { gradient: string })}
                <Area fill={gradient} line={{ class: classes.stroke }} />
              {/snippet}
            </LinearGradient>
          {/if}
          <Highlight points lines />
        </Svg>
        <ChartTooltip
          label={metric.replace(/_/g, ' ')}
          valueFormatter={tooltipFormatter}
        />
      </Chart>
    </ChartContainer>
  {:else}
    <div class="h-full flex items-center justify-center rounded-lg border border-dashed border-border bg-muted/30">
      <p class="text-sm text-muted-foreground">No data available</p>
    </div>
  {/if}
</div>
```

> Note: `Bars` in LayerChart renders vertical bars. The fill class is derived by replacing `stroke-` with `fill-` in the stroke class string. If `getChartClasses` doesn't use stroke- prefix, inspect the return value and use `chartColor` directly: `<Bars style="fill: {chartColor}" radius={2} />`.

- [ ] **Step 2: Update `UserSessionDrawer.svelte` to use `chartType="bar"`**

In `UserSessionDrawer.svelte`, add `chartType="bar"` to all four `<MetricChart>` usages:

```svelte
<!-- Chart 1: Data usage per session -->
<MetricChart data={rxData} metric="net_interface_rx_bytes" height={120} chartType="bar" />
...
<MetricChart data={txData} metric="net_interface_tx_bytes" height={120} chartType="bar" />

<!-- Chart 2: Session duration -->
<MetricChart data={durationData} metric="active_sessions" height={120} chartType="bar" />

<!-- Chart 3: Cumulative data -->
<MetricChart data={cumulativeData} metric="net_interface_rx_bytes" height={120} chartType="bar" />
```

- [ ] **Step 3: Type-check**

```bash
cd /opt/ocserv_panel/frontend && pnpm check 2>&1 | tail -20
```

Expected: 0 errors.

- [ ] **Step 4: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/lib/components/ui/MetricChart.svelte frontend/src/lib/components/history/UserSessionDrawer.svelte
git commit -m "feat(frontend): use bar charts in session drawer for discrete per-session data"
```

---

## Task 6: Bans tab — add time window selector to frontend

**Files:**
- Modify: `frontend/src/lib/api/history.ts`
- Modify: `frontend/src/routes/history/+page.svelte`

- [ ] **Step 1: Update `listHistoricalBans` to accept window**

In `frontend/src/lib/api/history.ts`, replace:

```ts
export function listHistoricalBans(page = 1, limit = 25): Promise<BanEventPage> {
  return apiFetch<BanEventPage>(`/api/history/bans?page=${page}&limit=${limit}`);
}
```

With:

```ts
export function listHistoricalBans(page = 1, limit = 25, window = 'all'): Promise<BanEventPage> {
  return apiFetch<BanEventPage>(
    `/api/history/bans?page=${page}&limit=${limit}&window=${encodeURIComponent(window)}`
  );
}
```

- [ ] **Step 2: Add `bansWindow` state to `+page.svelte`**

In the `// --- Bans tab state ---` section, add after `bansSearch`:

```svelte
let bansWindow = $state<string>('all');
```

- [ ] **Step 3: Update `loadBans` to pass window**

Replace the `loadBans` call inside:

```svelte
bansData = await listHistoricalBans(bansPage);
```

With:

```svelte
bansData = await listHistoricalBans(bansPage, 25, bansWindow);
```

- [ ] **Step 4: Add window-change handler**

Add a new function after `loadBans`:

```svelte
function handleBansWindowChange(nextWindow: string) {
  if (nextWindow === bansWindow) return;
  bansWindow = nextWindow;
  bansPage = 1;
  void loadBans();
}
```

- [ ] **Step 5: Add `WindowSelector` to Bans tab header**

Find the Bans tab header in the template (around the `IP Ban History` heading):

```svelte
<div class="flex items-center justify-between flex-wrap gap-3">
  <h2 class="text-base font-semibold text-slate-800 dark:text-slate-200">IP Ban History</h2>
  <input
    type="search"
    ...
  />
</div>
```

Replace with:

```svelte
<div class="flex items-center justify-between flex-wrap gap-3">
  <h2 class="text-base font-semibold text-slate-800 dark:text-slate-200">IP Ban History</h2>
  <div class="flex items-center gap-3">
    <WindowSelector
      value={bansWindow}
      windows={['24h', '7d', '30d', 'all']}
      onchange={handleBansWindowChange}
    />
    <input
      type="search"
      placeholder="Filter by IP..."
      bind:value={bansSearch}
      class="px-3 py-1.5 text-sm border border-slate-200 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-200"
    />
  </div>
</div>
```

- [ ] **Step 6: Check `WindowSelector` accepts `"all"` as a value**

Inspect `WindowSelector.svelte`:

```bash
grep -n "windows\|value\|option" /opt/ocserv_panel/frontend/src/lib/components/ui/WindowSelector.svelte | head -20
```

`WindowSelector` renders the provided `windows` array as options. Since `"all"` is a string, it will render as-is. Confirm the display label: if `WindowSelector` formats known values like `"24h"` → `"Last 24h"`, it may display `"all"` as `"all"`. If needed, pass a label map. Inspect the component and adjust labels if necessary.

- [ ] **Step 7: Type-check**

```bash
cd /opt/ocserv_panel/frontend && pnpm check 2>&1 | tail -20
```

Expected: 0 errors.

- [ ] **Step 8: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/lib/api/history.ts frontend/src/routes/history/+page.svelte
git commit -m "feat(frontend): add time window selector to history bans tab"
```

---

## Task 7: Monitoring tab — per-interface VPN lines

**Files:**
- Modify: `frontend/src/lib/monitoring.ts`
- Modify: `frontend/src/routes/history/+page.svelte`

**Context:** After Task 3 (backend), `monitoring.series` now contains multiple entries for `vpn_interface_rx_bytes` and `vpn_interface_tx_bytes`, each with a `labels.interface` field. `MultiLineChart.svelte` already exists and is used on the dashboard — reuse it here. The pattern from `DashboardMetrics.svelte` should be copied.

- [ ] **Step 1: Add `allSeriesForMetric` to `monitoring.ts`**

In `frontend/src/lib/monitoring.ts`, add after `seriesByMetric`:

```ts
export function allSeriesForMetric(
  overview: MonitoringOverview | undefined | null,
  metric: string
): MonitoringSeries[] {
  if (!Array.isArray(overview?.series)) return [];
  return overview.series.filter((s) => s.metric === metric);
}
```

- [ ] **Step 2: Update `+page.svelte` imports**

Add to the imports in the `<script>` section:

```svelte
import MultiLineChart from '$lib/components/ui/MultiLineChart.svelte';
import type { LineData } from '$lib/components/ui/MultiLineChart.svelte';
import { allSeriesForMetric, formatBytes, metricByName, seriesByMetric } from '$lib/monitoring';
```

Replace the existing `import { formatBytes, metricByName, seriesByMetric } from '$lib/monitoring';`.

- [ ] **Step 3: Add VPN derived values to `+page.svelte`**

In the `// --- Derived ---` section, add:

```svelte
const vpnInterfaceNames = $derived(
  [...new Set(
    (monitoring?.series ?? [])
      .filter(s => s.metric === VPN_RX)
      .map(s => s.labels['interface'])
      .filter((v): v is string => Boolean(v))
  )]
);

const vpnRXLines = $derived<LineData[]>(
  vpnInterfaceNames.map(iface => ({
    name: iface,
    data: (monitoring?.series ?? [])
      .filter(s => s.metric === VPN_RX && s.labels['interface'] === iface)
      .flatMap(s => s.points
        .filter(p => Number.isFinite(Date.parse(p.timestamp)) && Number.isFinite(p.value))
        .map(p => ({ timestamp: new Date(p.timestamp), value: p.value }))
      )
      .sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime()),
  }))
);

const vpnTXLines = $derived<LineData[]>(
  vpnInterfaceNames.map(iface => ({
    name: iface,
    data: (monitoring?.series ?? [])
      .filter(s => s.metric === VPN_TX && s.labels['interface'] === iface)
      .flatMap(s => s.points
        .filter(p => Number.isFinite(Date.parse(p.timestamp)) && Number.isFinite(p.value))
        .map(p => ({ timestamp: new Date(p.timestamp), value: p.value }))
      )
      .sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime()),
  }))
);
```

- [ ] **Step 4: Replace VPN chart slots in monitoring tab**

In the monitoring tab template, the current VPN charts use `chartDefinitions` loop. Find where VPN inbound/outbound are rendered and replace them with explicit `MultiLineChart` slots.

The `chartDefinitions` array currently includes `VPN_RX` and `VPN_TX`. Remove them from `chartDefinitions` so they don't render via the generic loop:

```svelte
const chartDefinitions: ChartDefinition[] = [
  { metric: HOST_CPU, title: 'CPU utilization' },
  { metric: HOST_MEMORY, title: 'Memory utilization' },
  { metric: NET_RX, title: 'Network inbound' },
  { metric: NET_TX, title: 'Network outbound' },
  { metric: ACTIVE_SESSIONS, title: 'Active sessions' },
  { metric: CONNECTION_RATE, title: 'Connection rate' },
  { metric: CONCURRENT_SESSIONS, title: 'Concurrent sessions' }
];
```

Then after the `{#each chartDefinitions as chart}` block closes its grid, add the two VPN cards explicitly:

```svelte
<!-- VPN inbound — one line per interface -->
<Card.Root>
  <Card.Header>
    <div>
      <Card.Title>VPN inbound</Card.Title>
      <Card.Description>Last {monitoringWindow}</Card.Description>
    </div>
  </Card.Header>
  <Card.Content>
    <MultiLineChart lines={vpnRXLines} metric={VPN_RX} height={160} />
  </Card.Content>
</Card.Root>
<!-- VPN outbound — one line per interface -->
<Card.Root>
  <Card.Header>
    <div>
      <Card.Title>VPN outbound</Card.Title>
      <Card.Description>Last {monitoringWindow}</Card.Description>
    </div>
  </Card.Header>
  <Card.Content>
    <MultiLineChart lines={vpnTXLines} metric={VPN_TX} height={160} />
  </Card.Content>
</Card.Root>
```

Also update `unavailableChips` derivation — since VPN metrics are removed from `chartDefinitions`, they won't appear in the chips either. Add them explicitly if you want unavailable chips for VPN:

```svelte
const unavailableChips = $derived<Chip[]>([
  ...chartDefinitions
    .filter(chart => !shouldShowChart(chart.metric))
    .map(chart => ({ name: chart.title, reason: unavailableReason(chart.metric) })),
  ...(vpnRXLines.length === 0 ? [{ name: 'VPN inbound', reason: unavailableReason(VPN_RX) }] : []),
  ...(vpnTXLines.length === 0 ? [{ name: 'VPN outbound', reason: unavailableReason(VPN_TX) }] : []),
]);
```

- [ ] **Step 5: Type-check**

```bash
cd /opt/ocserv_panel/frontend && pnpm check 2>&1 | tail -20
```

Expected: 0 errors.

- [ ] **Step 6: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/lib/monitoring.ts frontend/src/routes/history/+page.svelte
git commit -m "feat(frontend): render per-interface VPN lines in history monitoring tab"
```

---

## Task 8: Remove History summary cards

**Files:**
- Modify: `frontend/src/routes/history/+page.svelte`

- [ ] **Step 1: Remove the stat cards block**

In `+page.svelte`, find and remove the entire summary cards grid (4 `CardStats` components and the grid wrapper):

```svelte
  <!-- Summary stat cards -->
  <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-6 mb-8">
    <CardStats title="Sessions" ... />
    <CardStats title="Received" ... />
    <CardStats title="Sent" ... />
    <CardStats title="Total traffic" ... />
  </div>
```

- [ ] **Step 2: Remove unused state and helpers**

Remove the following from the `<script>`:

1. `let analytics = $state<HistoryAnalytics | null>(null);`
2. `const sampleWarning = $derived(sampleDataWarning(analytics));`
3. `const totals = $derived(calculateTotals(analytics?.summary));`
4. The `calculateTotals` function.
5. The `sampleDataWarning` function.
6. The `numberFrom` function.
7. The `analytics = await getHistoryAnalytics();` line inside `loadHistory`.

- [ ] **Step 3: Remove the sample data warning banner**

Find and remove the amber banner block:

```svelte
  {#if sampleWarning}
    <div class="bg-amber-50 border border-amber-200 rounded-lg p-4 mb-6">
      <strong class="text-amber-800 text-sm">Sample data warning:</strong>
      <span class="text-amber-700 text-sm ml-1">{sampleWarning}</span>
    </div>
  {/if}
```

- [ ] **Step 4: Remove unused imports**

Remove from the import block:
- `CardStats` (from `'$lib/components/ui/CardStats.svelte'`)
- `ArrowDown`, `ArrowUp`, `Server` (from `lucide-svelte` — keep `Users`, `BarChart2`, `List`, `Shield`, `Settings`)
- `getHistoryAnalytics` and `HistoryAnalytics` (from `'$lib/api'`)

- [ ] **Step 5: Fix the `{:else if analytics}` guard**

The outer template currently branches on `analytics`. Since analytics is removed, change the guard. The outer check structure is:

```svelte
{:else if analytics}
  <div class="mb-6">
    <h1 ...>History</h1>
    ...
  </div>
  <!-- tabs etc -->
{:else}
  <div class="bg-red-50 ...">
    <p>{error || 'Historical analytics were not returned.'}</p>
  </div>
{/if}
```

Replace `{:else if analytics}` with `{:else if principal}` (we already confirmed the user is authenticated at this point). Remove the final `{:else}` error block (it was only needed when analytics failed to load). Or keep it as a generic error display:

```svelte
{:else if principal}
  <div class="mb-6">
    <h1 class="text-xl font-bold text-slate-700">History</h1>
    <p class="text-sm text-slate-500 mt-1">Analytics are read from the local backend API and persisted SQLite history.</p>
  </div>
  <!-- tabs ... -->
{:else}
  <div class="bg-red-50 border border-red-200 rounded-lg p-6">
    <p class="text-red-700 text-sm">{error || 'Unable to load history page.'}</p>
  </div>
{/if}
```

- [ ] **Step 6: Type-check and build**

```bash
cd /opt/ocserv_panel/frontend && pnpm check 2>&1 | tail -20
```

Expected: 0 errors.

- [ ] **Step 7: Commit**

```bash
cd /opt/ocserv_panel && git add frontend/src/routes/history/+page.svelte
git commit -m "feat(frontend): remove summary cards from history page"
```

---

## Final Verification

- [ ] **Run full backend test suite**

```bash
cd /opt/ocserv_panel/backend && go test ./... 2>&1
```

Expected: all pass.

- [ ] **Run frontend type-check**

```bash
cd /opt/ocserv_panel/frontend && pnpm check 2>&1 | tail -20
```

Expected: 0 errors.

- [ ] **Build frontend**

```bash
cd /opt/ocserv_panel/frontend && pnpm build 2>&1 | tail -10
```

Expected: successful build, no errors.
