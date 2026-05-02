# Dashboard History Monitoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add local TSDB-backed monitoring data, host collectors, authenticated monitoring APIs, and shared Dashboard/History chart UI for ocserv_panel.

**Architecture:** The Go backend embeds Prometheus TSDB behind a narrow internal metrics interface and samples host CPU, memory, and configured interface counters from `/proc` and `/sys`. SQLite remains the source of truth for auth, audit, and historical session metadata. SvelteKit renders short-window Dashboard trends and longer-window History trends with shared lightweight SVG chart components.

**Tech Stack:** Go 1.25, `github.com/prometheus/prometheus/tsdb`, existing Go `net/http` API, Linux `/proc` and `/sys` fixture-readable collectors, Svelte 5/SvelteKit, existing same-origin API client, no frontend chart dependency.

---

## Coordination Rules

- Work in `/opt/git/ocserv_panel` unless the user asks for a worktree.
- Do not commit unless the user explicitly asks for a commit. The plan includes verification checkpoints instead of commit steps because repository rules require explicit commit approval.
- The user approved the embedded Prometheus TSDB dependency exception. If executing this plan in a new conversation, reconfirm before running `go get`.
- Do not run `ocserv`, `occtl`, `ocpasswd`, `systemctl`, `sudo`, shell command collectors, service-control commands, or live config/certificate mutations.
- Use `PATH="$HOME/.local/bin:$PATH"` for backend Go commands.
- Normal tests must not require live `ocserv` or host-specific interface names.
- Keep all TSDB implementation details behind `backend/internal/metrics` and do not import Prometheus TSDB from HTTP handlers or frontend-facing service code.

## File Map

### Backend Metrics Foundation

- Create: `backend/internal/metrics/types.go` — metric names, labels, sample/range/catalog API structs, validation helpers.
- Create: `backend/internal/metrics/memory_store.go` — test-friendly in-memory `MetricsStore` implementation.
- Create: `backend/internal/metrics/tsdb_store.go` — Prometheus TSDB-backed `MetricsStore` implementation.
- Create: `backend/internal/metrics/tsdb_store_test.go` — append/query tests against a temporary TSDB directory.
- Modify: `backend/go.mod` and `backend/go.sum` — add embedded Prometheus TSDB dependency.

### Backend Collectors

- Create: `backend/internal/monitoring/collectors.go` — fixture-readable host collectors for CPU, memory, and interface counters.
- Create: `backend/internal/monitoring/collectors_test.go` — collector parsing and unavailable-state tests.
- Create: `backend/internal/monitoring/collector_service.go` — periodic sampling service with start/stop lifecycle.
- Create: `backend/internal/monitoring/collector_service_test.go` — single collection cycle and no-crash unavailable metric tests.

### Backend Monitoring Service And API

- Create: `backend/internal/app/monitoring_service.go` — RBAC-protected monitoring catalog, overview, and series logic.
- Create: `backend/internal/app/monitoring_service_test.go` — auth, validation, catalog, and unavailable ocserv-only tests.
- Modify: `backend/internal/server/server.go` — monitoring response types and handlers.
- Modify: `backend/internal/server/server_test.go` — monitoring API auth and validation tests.
- Modify: `backend/cmd/ocserv-panel/main.go` — open TSDB, configure collectors, start collector lifecycle, wire monitoring service into router.

### Frontend API And Components

- Modify: `frontend/src/lib/api.ts` — typed monitoring API client methods.
- Create: `frontend/src/lib/monitoring.ts` — frontend metric display helpers.
- Create: `frontend/src/lib/MetricCard.svelte` — metric summary card component.
- Create: `frontend/src/lib/TimeSeriesChart.svelte` — lightweight SVG line/area chart component.
- Create: `frontend/src/lib/UnavailableMetric.svelte` — empty/unavailable metric state component.
- Create: `frontend/src/lib/WindowSelector.svelte` — allowed monitoring window selector.
- Modify: `frontend/src/routes/+page.svelte` — Dashboard short-window monitoring panels.
- Modify: `frontend/src/routes/history/+page.svelte` — History longer-window monitoring section while preserving session table and purge UI.
- Modify: `frontend/src/app.css` — chart, metric grid, and unavailable panel styles.

### Docs And Safety

- Modify: `docs/OPERATIONS.md` — TSDB path, retention, collector interval, interface env vars, no external Prometheus service.
- Modify: `docs/SECURITY.md` — safe TSDB label policy and monitoring non-goals.
- Modify: `docs/ADR-0001-architecture.md` — approved embedded TSDB dependency exception.

---

## Task 1: Metrics Types, Validation, And In-Memory Store

**Owner:** Backend API agent.

**Files:**
- Create: `backend/internal/metrics/types.go`
- Create: `backend/internal/metrics/memory_store.go`
- Create: `backend/internal/metrics/memory_store_test.go`

- [ ] **Step 1: Write metrics type and validation tests**

Create `backend/internal/metrics/memory_store_test.go` with:

```go
package metrics

import (
	"context"
	"testing"
	"time"
)

func TestValidateWindowAndStep(t *testing.T) {
	tests := []struct {
		name      string
		window    string
		step      string
		wantError bool
	}{
		{name: "short dashboard window", window: "1h", step: "60s"},
		{name: "long history window", window: "7d", step: "1h"},
		{name: "invalid window", window: "2h", step: "60s", wantError: true},
		{name: "invalid step", window: "1h", step: "30s", wantError: true},
		{name: "too many points", window: "30d", step: "15s", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ValidateWindowAndStep(tt.window, tt.step)
			if tt.wantError && err == nil {
				t.Fatal("ValidateWindowAndStep succeeded, want error")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("ValidateWindowAndStep returned error: %v", err)
			}
		})
	}
}

func TestMetricCatalogMarksOcservOnlyUnavailable(t *testing.T) {
	catalog := DefaultCatalog()
	for _, name := range []MetricName{MetricUserRXBytes, MetricUserTXBytes, MetricActiveSessions, MetricConnectionRate, MetricConcurrentSessions, MetricConcurrentSessionsLimit} {
		metric, ok := catalog.Find(name)
		if !ok {
			t.Fatalf("catalog missing %s", name)
		}
		if metric.State != StateUnavailable || metric.Reason == "" {
			t.Fatalf("metric %s state = %s reason = %q, want unavailable with reason", name, metric.State, metric.Reason)
		}
	}
}

func TestMemoryStoreAppendAndQueryRange(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)

	err := store.Append(ctx, []Sample{
		{Name: MetricHostCPUUtilization, Labels: map[string]string{"source": "host"}, Timestamp: base, Value: 0.25},
		{Name: MetricHostCPUUtilization, Labels: map[string]string{"source": "host"}, Timestamp: base.Add(time.Minute), Value: 0.50},
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}

	series, err := store.QueryRange(ctx, Query{Metric: MetricHostCPUUtilization, Start: base.Add(-time.Second), End: base.Add(2 * time.Minute), Step: time.Minute})
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if len(series.Points) != 2 {
		t.Fatalf("points = %d, want 2: %#v", len(series.Points), series.Points)
	}
	if series.Points[0].Value != 0.25 || series.Points[1].Value != 0.50 {
		t.Fatalf("points = %#v, want appended values", series.Points)
	}
}
```

- [ ] **Step 2: Run tests and confirm RED**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/metrics
```

Expected: fail because `MetricName`, `ValidateWindowAndStep`, `DefaultCatalog`, `NewMemoryStore`, and related types are not defined.

- [ ] **Step 3: Implement metric types and validation**

Create `backend/internal/metrics/types.go` with:

```go
package metrics

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

type MetricName string

const (
	MetricHostCPUUtilization    MetricName = "host_cpu_utilization"
	MetricHostMemoryUtilization MetricName = "host_memory_utilization"
	MetricNetInterfaceRXBytes   MetricName = "net_interface_rx_bytes"
	MetricNetInterfaceTXBytes   MetricName = "net_interface_tx_bytes"
	MetricVPNInterfaceRXBytes   MetricName = "vpn_interface_rx_bytes"
	MetricVPNInterfaceTXBytes   MetricName = "vpn_interface_tx_bytes"
	MetricUserRXBytes           MetricName = "user_rx_bytes"
	MetricUserTXBytes           MetricName = "user_tx_bytes"
	MetricActiveSessions        MetricName = "active_sessions"
	MetricConnectionRate        MetricName = "connection_rate"
	MetricConcurrentSessions    MetricName = "concurrent_sessions"
	MetricConcurrentSessionsLimit MetricName = "concurrent_sessions_limit"
)

type State string

const (
	StateCollecting  State = "collecting"
	StateUnavailable State = "unavailable"
)

var (
	ErrInvalidMetric = errors.New("invalid metric")
	ErrInvalidWindow = errors.New("invalid monitoring window")
	ErrInvalidStep   = errors.New("invalid monitoring step")
)

type MetricDefinition struct {
	Name        MetricName `json:"name"`
	Title       string     `json:"title"`
	Unit        string     `json:"unit"`
	State       State      `json:"state"`
	Reason      string     `json:"reason,omitempty"`
	Description string     `json:"description"`
}

type Catalog []MetricDefinition

func (c Catalog) Find(name MetricName) (MetricDefinition, bool) {
	for _, definition := range c {
		if definition.Name == name {
			return definition, true
		}
	}
	return MetricDefinition{}, false
}

func DefaultCatalog() Catalog {
	return Catalog{
		{Name: MetricHostCPUUtilization, Title: "CPU", Unit: "percent", State: StateCollecting, Description: "Host CPU utilization"},
		{Name: MetricHostMemoryUtilization, Title: "Memory", Unit: "percent", State: StateCollecting, Description: "Host memory utilization"},
		{Name: MetricNetInterfaceRXBytes, Title: "Network inbound", Unit: "bytes", State: StateUnavailable, Reason: "OCSERV_PANEL_NET_IFACE is not configured", Description: "Configured network interface inbound bytes"},
		{Name: MetricNetInterfaceTXBytes, Title: "Network outbound", Unit: "bytes", State: StateUnavailable, Reason: "OCSERV_PANEL_NET_IFACE is not configured", Description: "Configured network interface outbound bytes"},
		{Name: MetricVPNInterfaceRXBytes, Title: "VPN inbound", Unit: "bytes", State: StateUnavailable, Reason: "OCSERV_PANEL_VPN_IFACE is not configured", Description: "Configured VPN interface inbound bytes"},
		{Name: MetricVPNInterfaceTXBytes, Title: "VPN outbound", Unit: "bytes", State: StateUnavailable, Reason: "OCSERV_PANEL_VPN_IFACE is not configured", Description: "Configured VPN interface outbound bytes"},
		{Name: MetricUserRXBytes, Title: "User inbound", Unit: "bytes", State: StateUnavailable, Reason: "ocserv user ingestion is not implemented", Description: "Per-user inbound bytes"},
		{Name: MetricUserTXBytes, Title: "User outbound", Unit: "bytes", State: StateUnavailable, Reason: "ocserv user ingestion is not implemented", Description: "Per-user outbound bytes"},
		{Name: MetricActiveSessions, Title: "Active sessions", Unit: "sessions", State: StateUnavailable, Reason: "ocserv session ingestion is not implemented", Description: "Active VPN sessions"},
		{Name: MetricConnectionRate, Title: "Connection rate", Unit: "sessions_per_second", State: StateUnavailable, Reason: "ocserv session ingestion is not implemented", Description: "New VPN sessions per second"},
		{Name: MetricConcurrentSessions, Title: "Concurrent sessions", Unit: "sessions", State: StateUnavailable, Reason: "ocserv session ingestion is not implemented", Description: "Current concurrent sessions"},
		{Name: MetricConcurrentSessionsLimit, Title: "Concurrent limit", Unit: "sessions", State: StateUnavailable, Reason: "ocserv config ingestion is not implemented", Description: "Configured concurrent-session limit"},
	}
}

type Sample struct {
	Name      MetricName
	Labels    map[string]string
	Timestamp time.Time
	Value     float64
}

type Point struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type Series struct {
	Metric MetricName         `json:"metric"`
	Labels map[string]string `json:"labels"`
	Points []Point           `json:"points"`
}

type Query struct {
	Metric MetricName
	Start  time.Time
	End    time.Time
	Step   time.Duration
	Labels map[string]string
}

type Store interface {
	Append(context.Context, []Sample) error
	QueryRange(context.Context, Query) (Series, error)
	Close() error
}

var allowedWindows = map[string]time.Duration{
	"15m": 15 * time.Minute,
	"1h":  time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

var allowedSteps = map[string]time.Duration{
	"15s": 15 * time.Second,
	"60s": time.Minute,
	"5m":  5 * time.Minute,
	"1h":  time.Hour,
}

func ValidateWindowAndStep(windowValue string, stepValue string) (time.Duration, time.Duration, error) {
	window, ok := allowedWindows[windowValue]
	if !ok {
		return 0, 0, ErrInvalidWindow
	}
	step, ok := allowedSteps[stepValue]
	if !ok || step <= 0 {
		return 0, 0, ErrInvalidStep
	}
	if int(window/step) > 1500 {
		return 0, 0, fmt.Errorf("%w: too many points", ErrInvalidStep)
	}
	return window, step, nil
}

func ValidateMetric(name MetricName) error {
	if _, ok := DefaultCatalog().Find(name); !ok {
		return ErrInvalidMetric
	}
	return nil
}

func labelKey(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+labels[key])
	}
	return fmt.Sprint(parts)
}
```

- [ ] **Step 4: Implement in-memory store**

Create `backend/internal/metrics/memory_store.go` with:

```go
package metrics

import (
	"context"
	"sort"
	"sync"
)

type MemoryStore struct {
	mu      sync.RWMutex
	samples map[MetricName][]Sample
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{samples: make(map[MetricName][]Sample)}
}

func (s *MemoryStore) Append(_ context.Context, samples []Sample) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sample := range samples {
		if err := ValidateMetric(sample.Name); err != nil {
			return err
		}
		s.samples[sample.Name] = append(s.samples[sample.Name], sample)
	}
	for name := range s.samples {
		sort.Slice(s.samples[name], func(i, j int) bool {
			return s.samples[name][i].Timestamp.Before(s.samples[name][j].Timestamp)
		})
	}
	return nil
}

func (s *MemoryStore) QueryRange(_ context.Context, query Query) (Series, error) {
	if err := ValidateMetric(query.Metric); err != nil {
		return Series{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	series := Series{Metric: query.Metric, Labels: query.Labels, Points: []Point{}}
	for _, sample := range s.samples[query.Metric] {
		if sample.Timestamp.Before(query.Start) || sample.Timestamp.After(query.End) {
			continue
		}
		if !labelsMatch(sample.Labels, query.Labels) {
			continue
		}
		series.Points = append(series.Points, Point{Timestamp: sample.Timestamp, Value: sample.Value})
	}
	return series, nil
}

func (s *MemoryStore) Close() error { return nil }

func labelsMatch(sampleLabels map[string]string, queryLabels map[string]string) bool {
	for key, value := range queryLabels {
		if sampleLabels[key] != value {
			return false
		}
	}
	return true
}
```

- [ ] **Step 5: Run GREEN tests**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/metrics
```

Expected: tests pass.

---

## Task 2: Embedded Prometheus TSDB Store

**Owner:** Backend API agent.

**Files:**
- Modify: `backend/go.mod`
- Modify: `backend/go.sum`
- Create: `backend/internal/metrics/tsdb_store.go`
- Create: `backend/internal/metrics/tsdb_store_test.go`

- [ ] **Step 1: Add the approved TSDB dependency**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go get github.com/prometheus/prometheus@latest
PATH="$HOME/.local/bin:$PATH" go mod tidy
```

Expected: `backend/go.mod` includes `github.com/prometheus/prometheus` and `backend/go.sum` includes checksums. If `go get` selects an incompatible version, pin the newest compatible version that exposes `github.com/prometheus/prometheus/tsdb` for Go 1.25.

- [ ] **Step 2: Write TSDB store tests**

Create `backend/internal/metrics/tsdb_store_test.go` with:

```go
package metrics

import (
	"context"
	"testing"
	"time"
)

func TestTSDBStoreAppendAndQueryRange(t *testing.T) {
	ctx := context.Background()
	store, err := OpenTSDBStore(t.TempDir(), 30*24*time.Hour)
	if err != nil {
		t.Fatalf("OpenTSDBStore: %v", err)
	}
	defer store.Close()

	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	if err := store.Append(ctx, []Sample{
		{Name: MetricHostMemoryUtilization, Labels: map[string]string{"source": "host"}, Timestamp: base, Value: 0.60},
		{Name: MetricHostMemoryUtilization, Labels: map[string]string{"source": "host"}, Timestamp: base.Add(time.Minute), Value: 0.65},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	series, err := store.QueryRange(ctx, Query{Metric: MetricHostMemoryUtilization, Start: base.Add(-time.Second), End: base.Add(2 * time.Minute), Step: time.Minute})
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if len(series.Points) != 2 {
		t.Fatalf("points = %d, want 2: %#v", len(series.Points), series.Points)
	}
	if series.Points[0].Value != 0.60 || series.Points[1].Value != 0.65 {
		t.Fatalf("points = %#v, want appended values", series.Points)
	}
}

func TestTSDBStoreRejectsInvalidMetric(t *testing.T) {
	ctx := context.Background()
	store, err := OpenTSDBStore(t.TempDir(), 30*24*time.Hour)
	if err != nil {
		t.Fatalf("OpenTSDBStore: %v", err)
	}
	defer store.Close()

	err = store.Append(ctx, []Sample{{Name: MetricName("bad_metric"), Timestamp: time.Now().UTC(), Value: 1}})
	if err == nil {
		t.Fatal("Append succeeded, want invalid metric error")
	}
}
```

- [ ] **Step 3: Run tests and confirm RED**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/metrics -run 'TestTSDBStore'
```

Expected: fail because `OpenTSDBStore` is not defined.

- [ ] **Step 4: Implement TSDB store adapter**

Create `backend/internal/metrics/tsdb_store.go` with this structure. If the selected Prometheus module version has small API differences, preserve the public `OpenTSDBStore`, `Append`, `QueryRange`, and `Close` signatures and adapt only the internal calls.

```go
package metrics

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
)

type TSDBStore struct {
	db *tsdb.DB
}

func OpenTSDBStore(path string, retention time.Duration) (*TSDBStore, error) {
	options := tsdb.DefaultOptions()
	options.RetentionDuration = int64(retention / time.Millisecond)
	database, err := tsdb.Open(path, nil, nil, options, nil)
	if err != nil {
		return nil, fmt.Errorf("open metrics tsdb: %w", err)
	}
	return &TSDBStore{db: database}, nil
}

func (s *TSDBStore) Append(ctx context.Context, samples []Sample) error {
	appender := s.db.Appender(ctx)
	for _, sample := range samples {
		if err := ValidateMetric(sample.Name); err != nil {
			_ = appender.Rollback()
			return err
		}
		_, err := appender.Append(0, labelsForSample(sample), sample.Timestamp.UnixMilli(), sample.Value)
		if err != nil {
			_ = appender.Rollback()
			return fmt.Errorf("append metrics sample: %w", err)
		}
	}
	if err := appender.Commit(); err != nil {
		return fmt.Errorf("commit metrics samples: %w", err)
	}
	return nil
}

func (s *TSDBStore) QueryRange(ctx context.Context, query Query) (Series, error) {
	if err := ValidateMetric(query.Metric); err != nil {
		return Series{}, err
	}
	querier, err := s.db.Querier(query.Start.UnixMilli(), query.End.UnixMilli())
	if err != nil {
		return Series{}, fmt.Errorf("open metrics querier: %w", err)
	}
	defer querier.Close()

	matchers := []*labels.Matcher{labels.MustNewMatcher(labels.MatchEqual, "__name__", string(query.Metric))}
	for key, value := range query.Labels {
		matchers = append(matchers, labels.MustNewMatcher(labels.MatchEqual, key, value))
	}
	set := querier.Select(ctx, false, &storage.SelectHints{Start: query.Start.UnixMilli(), End: query.End.UnixMilli(), Step: query.Step.Milliseconds()}, matchers...)
	series := Series{Metric: query.Metric, Labels: query.Labels, Points: []Point{}}
	for set.Next() {
		iterator := set.At().Iterator(nil)
		for iterator.Next() == chunkenc.ValFloat {
			ms, value := iterator.At()
			series.Points = append(series.Points, Point{Timestamp: time.UnixMilli(ms).UTC(), Value: value})
		}
		if err := iterator.Err(); err != nil {
			return Series{}, fmt.Errorf("iterate metrics series: %w", err)
		}
	}
	if err := set.Err(); err != nil {
		return Series{}, fmt.Errorf("select metrics series: %w", err)
	}
	return series, nil
}

func (s *TSDBStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func labelsForSample(sample Sample) labels.Labels {
	builder := labels.NewBuilder(labels.EmptyLabels())
	builder.Set("__name__", string(sample.Name))
	for key, value := range sample.Labels {
		builder.Set(key, value)
	}
	return builder.Labels()
}
```

Also add this import to `tsdb_store.go` if not automatically inserted by the editor:

```go
import "github.com/prometheus/prometheus/tsdb/chunkenc"
```

- [ ] **Step 5: Run dependency and TSDB tests**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/metrics
PATH="$HOME/.local/bin:$PATH" go mod verify
```

Expected: metrics tests pass and module checksums verify.

---

## Task 3: Host Collectors

**Owner:** Backend API agent.

**Files:**
- Create: `backend/internal/monitoring/collectors.go`
- Create: `backend/internal/monitoring/collectors_test.go`

- [ ] **Step 1: Write collector fixture tests**

Create `backend/internal/monitoring/collectors_test.go` with:

```go
package monitoring

import (
	"errors"
	"testing"
	"time"

	"ocserv_panel/backend/internal/metrics"
)

type fixtureReader map[string]string

func (f fixtureReader) ReadFile(path string) ([]byte, error) {
	value, ok := f[path]
	if !ok {
		return nil, errors.New("missing fixture")
	}
	return []byte(value), nil
}

func TestCPUCollectorComputesUtilizationFromDeltas(t *testing.T) {
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	collector := NewCPUCollector(fixtureReader{"/proc/stat": "cpu  100 0 100 800 0 0 0 0 0 0\n"})
	if samples, status := collector.Collect(now); len(samples) != 0 || status.Available {
		t.Fatalf("first collect samples=%#v status=%#v, want no sample warmup unavailable", samples, status)
	}
	collector.reader = fixtureReader{"/proc/stat": "cpu  150 0 150 900 0 0 0 0 0 0\n"}
	samples, status := collector.Collect(now.Add(15 * time.Second))
	if !status.Available {
		t.Fatalf("status = %#v, want available", status)
	}
	if len(samples) != 1 || samples[0].Name != metrics.MetricHostCPUUtilization {
		t.Fatalf("samples = %#v, want CPU sample", samples)
	}
	if samples[0].Value < 0.49 || samples[0].Value > 0.51 {
		t.Fatalf("cpu value = %f, want about 0.50", samples[0].Value)
	}
}

func TestMemoryCollectorComputesUtilization(t *testing.T) {
	collector := NewMemoryCollector(fixtureReader{"/proc/meminfo": "MemTotal:       1000 kB\nMemAvailable:    250 kB\n"})
	samples, status := collector.Collect(time.Now().UTC())
	if !status.Available {
		t.Fatalf("status = %#v, want available", status)
	}
	if len(samples) != 1 || samples[0].Name != metrics.MetricHostMemoryUtilization || samples[0].Value != 0.75 {
		t.Fatalf("samples = %#v, want memory utilization 0.75", samples)
	}
}

func TestInterfaceCollectorReportsMissingInterfaceUnavailable(t *testing.T) {
	collector := NewInterfaceCollector(fixtureReader{}, "eth0", metrics.MetricNetInterfaceRXBytes, metrics.MetricNetInterfaceTXBytes)
	samples, status := collector.Collect(time.Now().UTC())
	if status.Available || status.Reason == "" {
		t.Fatalf("status = %#v, want unavailable with reason", status)
	}
	if len(samples) != 0 {
		t.Fatalf("samples = %#v, want no samples for missing interface", samples)
	}
}

func TestInterfaceCollectorReadsCounters(t *testing.T) {
	reader := fixtureReader{
		"/sys/class/net/eth0/statistics/rx_bytes": "1024\n",
		"/sys/class/net/eth0/statistics/tx_bytes": "2048\n",
	}
	collector := NewInterfaceCollector(reader, "eth0", metrics.MetricNetInterfaceRXBytes, metrics.MetricNetInterfaceTXBytes)
	samples, status := collector.Collect(time.Now().UTC())
	if !status.Available {
		t.Fatalf("status = %#v, want available", status)
	}
	if len(samples) != 2 || samples[0].Value != 1024 || samples[1].Value != 2048 {
		t.Fatalf("samples = %#v, want rx/tx counters", samples)
	}
}
```

- [ ] **Step 2: Run tests and confirm RED**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/monitoring -run 'Test.*Collector'
```

Expected: fail because collectors are not defined.

- [ ] **Step 3: Implement collectors**

Create `backend/internal/monitoring/collectors.go` with:

```go
package monitoring

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ocserv_panel/backend/internal/metrics"
)

type FileReader interface {
	ReadFile(string) ([]byte, error)
}

type OSReader struct{}

func (OSReader) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }

type CollectStatus struct {
	Metric    metrics.MetricName
	Available bool
	Reason    string
}

type Collector interface {
	Collect(time.Time) ([]metrics.Sample, CollectStatus)
}

type CPUCollector struct {
	reader FileReader
	last   cpuTimes
}

type cpuTimes struct {
	Idle  uint64
	Total uint64
	Set   bool
}

func NewCPUCollector(reader FileReader) *CPUCollector {
	return &CPUCollector{reader: reader}
}

func (c *CPUCollector) Collect(now time.Time) ([]metrics.Sample, CollectStatus) {
	data, err := c.reader.ReadFile("/proc/stat")
	if err != nil {
		return nil, CollectStatus{Metric: metrics.MetricHostCPUUtilization, Reason: err.Error()}
	}
	current, err := parseCPUTimes(data)
	if err != nil {
		return nil, CollectStatus{Metric: metrics.MetricHostCPUUtilization, Reason: err.Error()}
	}
	if !c.last.Set {
		c.last = current
		return nil, CollectStatus{Metric: metrics.MetricHostCPUUtilization, Reason: "warming up cpu delta"}
	}
	totalDelta := current.Total - c.last.Total
	idleDelta := current.Idle - c.last.Idle
	c.last = current
	if totalDelta == 0 || idleDelta > totalDelta {
		return nil, CollectStatus{Metric: metrics.MetricHostCPUUtilization, Reason: "invalid cpu delta"}
	}
	value := float64(totalDelta-idleDelta) / float64(totalDelta)
	return []metrics.Sample{{Name: metrics.MetricHostCPUUtilization, Labels: map[string]string{"source": "host"}, Timestamp: now, Value: value}}, CollectStatus{Metric: metrics.MetricHostCPUUtilization, Available: true}
}

func parseCPUTimes(data []byte) (cpuTimes, error) {
	line, _, ok := bytes.Cut(data, []byte("\n"))
	if !ok {
		line = data
	}
	fields := strings.Fields(string(line))
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuTimes{}, fmt.Errorf("invalid /proc/stat cpu line")
	}
	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return cpuTimes{}, fmt.Errorf("parse cpu field: %w", err)
		}
		values = append(values, value)
	}
	total := uint64(0)
	for _, value := range values {
		total += value
	}
	return cpuTimes{Idle: values[3], Total: total, Set: true}, nil
}

type MemoryCollector struct{ reader FileReader }

func NewMemoryCollector(reader FileReader) *MemoryCollector { return &MemoryCollector{reader: reader} }

func (c *MemoryCollector) Collect(now time.Time) ([]metrics.Sample, CollectStatus) {
	data, err := c.reader.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, CollectStatus{Metric: metrics.MetricHostMemoryUtilization, Reason: err.Error()}
	}
	total, available, err := parseMeminfo(data)
	if err != nil {
		return nil, CollectStatus{Metric: metrics.MetricHostMemoryUtilization, Reason: err.Error()}
	}
	value := 1 - (float64(available) / float64(total))
	return []metrics.Sample{{Name: metrics.MetricHostMemoryUtilization, Labels: map[string]string{"source": "host"}, Timestamp: now, Value: value}}, CollectStatus{Metric: metrics.MetricHostMemoryUtilization, Available: true}
}

func parseMeminfo(data []byte) (uint64, uint64, error) {
	values := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("parse meminfo %s: %w", key, err)
		}
		values[key] = value
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	if total == 0 {
		return 0, 0, fmt.Errorf("MemTotal missing")
	}
	return total, available, nil
}

type InterfaceCollector struct {
	reader FileReader
	iface  string
	rx     metrics.MetricName
	tx     metrics.MetricName
}

func NewInterfaceCollector(reader FileReader, iface string, rx metrics.MetricName, tx metrics.MetricName) *InterfaceCollector {
	return &InterfaceCollector{reader: reader, iface: iface, rx: rx, tx: tx}
}

func (c *InterfaceCollector) Collect(now time.Time) ([]metrics.Sample, CollectStatus) {
	if strings.TrimSpace(c.iface) == "" {
		return nil, CollectStatus{Metric: c.rx, Reason: "interface is not configured"}
	}
	rx, err := c.readCounter("rx_bytes")
	if err != nil {
		return nil, CollectStatus{Metric: c.rx, Reason: err.Error()}
	}
	tx, err := c.readCounter("tx_bytes")
	if err != nil {
		return nil, CollectStatus{Metric: c.tx, Reason: err.Error()}
	}
	labels := map[string]string{"interface": c.iface}
	return []metrics.Sample{
		{Name: c.rx, Labels: labels, Timestamp: now, Value: float64(rx)},
		{Name: c.tx, Labels: labels, Timestamp: now, Value: float64(tx)},
	}, CollectStatus{Metric: c.rx, Available: true}
}

func (c *InterfaceCollector) readCounter(name string) (uint64, error) {
	path := filepath.Join("/sys/class/net", c.iface, "statistics", name)
	data, err := c.reader.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", path, err)
	}
	return value, nil
}
```

- [ ] **Step 4: Run GREEN collector tests**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/monitoring -run 'Test.*Collector'
```

Expected: collector tests pass.

---

## Task 4: Collector Service Lifecycle

**Owner:** Backend API agent.

**Files:**
- Create: `backend/internal/monitoring/collector_service.go`
- Create/modify: `backend/internal/monitoring/collector_service_test.go`

- [ ] **Step 1: Write collector service tests**

Create `backend/internal/monitoring/collector_service_test.go` with:

```go
package monitoring

import (
	"context"
	"testing"
	"time"

	"ocserv_panel/backend/internal/metrics"
)

type fixedCollector struct {
	samples []metrics.Sample
	status  CollectStatus
}

func (c fixedCollector) Collect(time.Time) ([]metrics.Sample, CollectStatus) {
	return c.samples, c.status
}

func TestCollectorServiceCollectOnceAppendsAvailableSamples(t *testing.T) {
	ctx := context.Background()
	store := metrics.NewMemoryStore()
	service := NewCollectorService(store, []Collector{fixedCollector{samples: []metrics.Sample{{Name: metrics.MetricHostMemoryUtilization, Timestamp: time.Now().UTC(), Value: 0.5}}, status: CollectStatus{Metric: metrics.MetricHostMemoryUtilization, Available: true}}}, time.Minute)

	service.CollectOnce(ctx, time.Now().UTC())
	series, err := store.QueryRange(ctx, metrics.Query{Metric: metrics.MetricHostMemoryUtilization, Start: time.Now().Add(-time.Hour), End: time.Now().Add(time.Hour), Step: time.Minute})
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if len(series.Points) != 1 {
		t.Fatalf("points = %d, want 1", len(series.Points))
	}
}

func TestCollectorServiceCollectOnceSkipsUnavailableSamples(t *testing.T) {
	ctx := context.Background()
	store := metrics.NewMemoryStore()
	service := NewCollectorService(store, []Collector{fixedCollector{status: CollectStatus{Metric: metrics.MetricVPNInterfaceRXBytes, Reason: "missing"}}}, time.Minute)

	service.CollectOnce(ctx, time.Now().UTC())
	series, err := store.QueryRange(ctx, metrics.Query{Metric: metrics.MetricVPNInterfaceRXBytes, Start: time.Now().Add(-time.Hour), End: time.Now().Add(time.Hour), Step: time.Minute})
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if len(series.Points) != 0 {
		t.Fatalf("points = %#v, want none", series.Points)
	}
}
```

- [ ] **Step 2: Run tests and confirm RED**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/monitoring -run 'TestCollectorService'
```

Expected: fail because `NewCollectorService` is not defined.

- [ ] **Step 3: Implement collector service**

Create `backend/internal/monitoring/collector_service.go` with:

```go
package monitoring

import (
	"context"
	"log/slog"
	"time"

	"ocserv_panel/backend/internal/metrics"
)

type CollectorService struct {
	store      metrics.Store
	collectors []Collector
	interval   time.Duration
}

func NewCollectorService(store metrics.Store, collectors []Collector, interval time.Duration) *CollectorService {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &CollectorService{store: store, collectors: collectors, interval: interval}
}

func (s *CollectorService) CollectOnce(ctx context.Context, now time.Time) {
	for _, collector := range s.collectors {
		samples, status := collector.Collect(now)
		if !status.Available {
			slog.Debug("monitoring metric unavailable", "metric", status.Metric, "reason", status.Reason)
			continue
		}
		if len(samples) == 0 {
			continue
		}
		if err := s.store.Append(ctx, samples); err != nil {
			slog.Warn("append monitoring samples failed", "error", err)
		}
	}
}

func (s *CollectorService) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	s.CollectOnce(ctx, time.Now().UTC())
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.CollectOnce(ctx, now.UTC())
		}
	}
}
```

- [ ] **Step 4: Run monitoring tests**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/monitoring
```

Expected: tests pass.

---

## Task 5: Monitoring Application Service

**Owner:** Backend API agent.

**Files:**
- Create: `backend/internal/app/monitoring_service.go`
- Create: `backend/internal/app/monitoring_service_test.go`

- [ ] **Step 1: Write monitoring service tests**

Create `backend/internal/app/monitoring_service_test.go` with:

```go
package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/metrics"
	"ocserv_panel/backend/internal/rbac"
)

func TestMonitoringServiceRequiresDashboardReadPermission(t *testing.T) {
	service := NewMonitoringService(metrics.NewMemoryStore(), rbac.NewDefaultAuthorizer(), func() time.Time { return time.Now().UTC() })
	if _, err := service.Catalog(context.Background(), auth.Principal{}); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Catalog err = %v, want ErrUnauthenticated", err)
	}
	limited := auth.Principal{ID: 1, Username: "limited", Roles: []string{}}
	if _, err := service.Catalog(context.Background(), limited); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Catalog err = %v, want ErrPermissionDenied", err)
	}
}

func TestMonitoringServiceCatalogIncludesUnavailableOcservMetrics(t *testing.T) {
	principal := auth.Principal{ID: 1, Username: "viewer", Roles: []string{string(rbac.RoleViewer)}}
	service := NewMonitoringService(metrics.NewMemoryStore(), rbac.NewDefaultAuthorizer(), func() time.Time { return time.Now().UTC() })
	catalog, err := service.Catalog(context.Background(), principal)
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	metric, ok := catalog.Find(metrics.MetricActiveSessions)
	if !ok || metric.State != metrics.StateUnavailable || metric.Reason == "" {
		t.Fatalf("active sessions metric = %#v found=%v, want unavailable", metric, ok)
	}
}

func TestMonitoringServiceSeriesValidatesInputs(t *testing.T) {
	principal := auth.Principal{ID: 1, Username: "viewer", Roles: []string{string(rbac.RoleViewer)}}
	service := NewMonitoringService(metrics.NewMemoryStore(), rbac.NewDefaultAuthorizer(), func() time.Time { return time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC) })

	if _, err := service.Series(context.Background(), principal, "bad_metric", "1h", "60s"); err == nil {
		t.Fatal("Series accepted bad metric, want error")
	}
	if _, err := service.Series(context.Background(), principal, string(metrics.MetricHostCPUUtilization), "2h", "60s"); err == nil {
		t.Fatal("Series accepted bad window, want error")
	}
	if _, err := service.Series(context.Background(), principal, string(metrics.MetricHostCPUUtilization), "1h", "30s"); err == nil {
		t.Fatal("Series accepted bad step, want error")
	}
}
```

- [ ] **Step 2: Run tests and confirm RED**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/app -run 'TestMonitoringService'
```

Expected: fail because `NewMonitoringService` is not defined.

- [ ] **Step 3: Implement monitoring service**

Create `backend/internal/app/monitoring_service.go` with:

```go
package app

import (
	"context"
	"fmt"
	"time"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/metrics"
	"ocserv_panel/backend/internal/rbac"
)

type MonitoringService struct {
	store      metrics.Store
	authorizer rbac.Authorizer
	now        func() time.Time
}

type MonitoringOverview struct {
	Window  string           `json:"window"`
	Catalog metrics.Catalog  `json:"catalog"`
	Series  []metrics.Series `json:"series"`
}

func NewMonitoringService(store metrics.Store, authorizer rbac.Authorizer, now func() time.Time) *MonitoringService {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &MonitoringService{store: store, authorizer: authorizer, now: now}
}

func (s *MonitoringService) Catalog(ctx context.Context, principal auth.Principal) (metrics.Catalog, error) {
	if err := s.require(principal); err != nil {
		return nil, err
	}
	return metrics.DefaultCatalog(), nil
}

func (s *MonitoringService) Overview(ctx context.Context, principal auth.Principal, windowValue string) (MonitoringOverview, error) {
	if err := s.require(principal); err != nil {
		return MonitoringOverview{}, err
	}
	window, step, err := metrics.ValidateWindowAndStep(windowValue, "60s")
	if err != nil {
		return MonitoringOverview{}, err
	}
	end := s.now()
	start := end.Add(-window)
	series := make([]metrics.Series, 0)
	for _, metric := range []metrics.MetricName{metrics.MetricHostCPUUtilization, metrics.MetricHostMemoryUtilization, metrics.MetricNetInterfaceRXBytes, metrics.MetricNetInterfaceTXBytes, metrics.MetricVPNInterfaceRXBytes, metrics.MetricVPNInterfaceTXBytes, metrics.MetricActiveSessions, metrics.MetricConcurrentSessions, metrics.MetricConcurrentSessionsLimit} {
		result, err := s.store.QueryRange(ctx, metrics.Query{Metric: metric, Start: start, End: end, Step: step})
		if err != nil {
			return MonitoringOverview{}, fmt.Errorf("query overview metric %s: %w", metric, err)
		}
		series = append(series, result)
	}
	return MonitoringOverview{Window: windowValue, Catalog: metrics.DefaultCatalog(), Series: series}, nil
}

func (s *MonitoringService) Series(ctx context.Context, principal auth.Principal, metricValue string, windowValue string, stepValue string) (metrics.Series, error) {
	if err := s.require(principal); err != nil {
		return metrics.Series{}, err
	}
	metric := metrics.MetricName(metricValue)
	if err := metrics.ValidateMetric(metric); err != nil {
		return metrics.Series{}, err
	}
	window, step, err := metrics.ValidateWindowAndStep(windowValue, stepValue)
	if err != nil {
		return metrics.Series{}, err
	}
	end := s.now()
	start := end.Add(-window)
	return s.store.QueryRange(ctx, metrics.Query{Metric: metric, Start: start, End: end, Step: step})
}

func (s *MonitoringService) require(principal auth.Principal) error {
	if !principal.Authenticated() {
		return ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionReadDashboard) {
		return ErrPermissionDenied
	}
	return nil
}
```

- [ ] **Step 4: Run app tests**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/app -run 'TestMonitoringService'
```

Expected: tests pass.

---

## Task 6: Monitoring HTTP API

**Owner:** Backend API agent.

**Files:**
- Modify: `backend/internal/server/server.go`
- Modify: `backend/internal/server/server_test.go`

- [ ] **Step 1: Extend router dependencies in tests first**

Modify `backend/internal/server/server_test.go` so `testHarness` creates `monitoringService := app.NewMonitoringService(metrics.NewMemoryStore(), rbac.NewDefaultAuthorizer(), time.Now)` and passes it into the router. Add the `metrics` import.

Expected changed harness line:

```go
monitoringService := app.NewMonitoringService(metrics.NewMemoryStore(), rbac.NewDefaultAuthorizer(), func() time.Time { return time.Now().UTC() })
router: NewAppRouter(authService, dashboardService, monitoringService, AppOptions{CookieSecure: cookieSecure}),
```

- [ ] **Step 2: Add monitoring API tests**

Append to `backend/internal/server/server_test.go`:

```go
func TestMonitoringEndpointsRequireAuthentication(t *testing.T) {
	h := newTestHarness(t)
	for _, path := range []string{"/api/monitoring/catalog", "/api/monitoring/overview?window=1h", "/api/monitoring/series?metric=host_cpu_utilization&window=1h&step=60s"} {
		rec := serveAPI(t, h.router, http.MethodGet, path, nil, "")
		assertStatus(t, rec, http.StatusUnauthorized)
	}
}

func TestMonitoringCatalogReturnsMetricStates(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/monitoring/catalog", nil, cookie)
	assertStatus(t, rec, http.StatusOK)

	var body map[string]any
	decodeJSON(t, rec, &body)
	metricsValue, ok := body["metrics"].([]any)
	if !ok || len(metricsValue) == 0 {
		t.Fatalf("metrics missing from catalog response: %v", body)
	}
}

func TestMonitoringSeriesRejectsInvalidQuery(t *testing.T) {
	h := newTestHarness(t)
	h.createOwner(t, "owner", "owner-password")
	cookie := h.login(t, "owner", "owner-password")

	rec := serveAPI(t, h.router, http.MethodGet, "/api/monitoring/series?metric=bad&window=1h&step=60s", nil, cookie)
	assertStatus(t, rec, http.StatusBadRequest)

	rec = serveAPI(t, h.router, http.MethodGet, "/api/monitoring/series?metric=host_cpu_utilization&window=2h&step=60s", nil, cookie)
	assertStatus(t, rec, http.StatusBadRequest)
}
```

- [ ] **Step 3: Run tests and confirm RED**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/server -run 'TestMonitoring'
```

Expected: fail because router signature and monitoring handlers are not implemented.

- [ ] **Step 4: Implement server monitoring wiring**

Modify `backend/internal/server/server.go`:

Add field to `App`:

```go
monitoringService *app.MonitoringService
```

Change constructor signature:

```go
func NewAppRouter(authService *app.AuthService, dashboardService *app.DashboardService, monitoringService *app.MonitoringService, opts AppOptions) http.Handler
```

Set the field in `appHandler` and add routes:

```go
mux.HandleFunc("/api/monitoring/catalog", appHandler.handleMonitoringCatalog)
mux.HandleFunc("/api/monitoring/overview", appHandler.handleMonitoringOverview)
mux.HandleFunc("/api/monitoring/series", appHandler.handleMonitoringSeries)
```

Add response structs near existing response types:

```go
type monitoringCatalogResponse struct {
	Metrics any `json:"metrics"`
}
```

Add handlers near other API handlers:

```go
func (a *App) handleMonitoringCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	principal, ok := a.requirePrincipal(w, r)
	if !ok {
		return
	}
	catalog, err := a.monitoringService.Catalog(r.Context(), principal)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, monitoringCatalogResponse{Metrics: catalog})
}

func (a *App) handleMonitoringOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	principal, ok := a.requirePrincipal(w, r)
	if !ok {
		return
	}
	window := r.URL.Query().Get("window")
	if window == "" {
		window = "1h"
	}
	overview, err := a.monitoringService.Overview(r.Context(), principal, window)
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) handleMonitoringSeries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	principal, ok := a.requirePrincipal(w, r)
	if !ok {
		return
	}
	series, err := a.monitoringService.Series(r.Context(), principal, r.URL.Query().Get("metric"), r.URL.Query().Get("window"), r.URL.Query().Get("step"))
	if err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, series)
}
```

Update all `NewAppRouter` call sites to pass `monitoringService`.

- [ ] **Step 5: Map validation errors to HTTP 400**

In `writeAppError` or the equivalent error mapper in `server.go`, add handling for metrics validation errors:

```go
if errors.Is(err, metrics.ErrInvalidMetric) || errors.Is(err, metrics.ErrInvalidWindow) || errors.Is(err, metrics.ErrInvalidStep) {
	writeError(w, http.StatusBadRequest, "invalid monitoring query")
	return
}
```

Add the `metrics` import to `server.go`.

- [ ] **Step 6: Run server tests**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./internal/server
```

Expected: server tests pass.

---

## Task 7: Backend Main Wiring And Configuration

**Owner:** Deployment/Ops or Backend API agent.

**Files:**
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 1: Add helper functions for monitoring config**

Modify `backend/cmd/ocserv-panel/main.go` to add imports for metrics and monitoring packages:

```go
"ocserv_panel/backend/internal/metrics"
"ocserv_panel/backend/internal/monitoring"
```

Add helper functions near existing env helpers:

```go
func tsdbPathFromEnv() string {
	return envOrDefault("OCSERV_PANEL_TSDB_PATH", filepath.Join("data", "metrics"))
}

func configuredCollectors() []monitoring.Collector {
	reader := monitoring.OSReader{}
	collectors := []monitoring.Collector{
		monitoring.NewCPUCollector(reader),
		monitoring.NewMemoryCollector(reader),
	}
	if iface := os.Getenv("OCSERV_PANEL_NET_IFACE"); iface != "" {
		collectors = append(collectors, monitoring.NewInterfaceCollector(reader, iface, metrics.MetricNetInterfaceRXBytes, metrics.MetricNetInterfaceTXBytes))
	}
	if iface := os.Getenv("OCSERV_PANEL_VPN_IFACE"); iface != "" {
		collectors = append(collectors, monitoring.NewInterfaceCollector(reader, iface, metrics.MetricVPNInterfaceRXBytes, metrics.MetricVPNInterfaceTXBytes))
	}
	return collectors
}
```

- [ ] **Step 2: Open TSDB and wire monitoring service**

In `main`, after creating `dashboardService`, add:

```go
metricsStore, err := metrics.OpenTSDBStore(tsdbPathFromEnv(), 30*24*time.Hour)
if err != nil {
	slog.Warn("open monitoring tsdb failed; monitoring endpoints will be empty", "error", err)
	metricsStore = metrics.NewMemoryStore()
}
defer func() {
	if err := metricsStore.Close(); err != nil {
		slog.Error("close monitoring store failed", "error", err)
	}
}()

monitoringService := app.NewMonitoringService(metricsStore, rbac.NewDefaultAuthorizer(), func() time.Time { return time.Now().UTC() })
collectorService := monitoring.NewCollectorService(metricsStore, configuredCollectors(), 15*time.Second)
go collectorService.Run(ctx)
```

Update router wiring:

```go
Handler: server.NewAppRouter(authService, dashboardService, monitoringService, server.AppOptions{CookieSecure: cookieSecureFromEnv()}),
```

- [ ] **Step 3: Run backend tests**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./...
```

Expected: backend tests pass.

---

## Task 8: Frontend Monitoring API Client And Helpers

**Owner:** Frontend admin UX agent.

**Files:**
- Modify: `frontend/src/lib/api.ts`
- Create: `frontend/src/lib/monitoring.ts`

- [ ] **Step 1: Add monitoring API types and calls**

Modify `frontend/src/lib/api.ts` after existing history types:

```ts
export type MonitoringMetricState = 'collecting' | 'unavailable';

export type MonitoringMetric = {
  name: string;
  title: string;
  unit: string;
  state: MonitoringMetricState;
  reason?: string;
  description: string;
};

export type MonitoringPoint = {
  timestamp: string;
  value: number;
};

export type MonitoringSeries = {
  metric: string;
  labels?: Record<string, string>;
  points: MonitoringPoint[];
};

export type MonitoringCatalog = {
  metrics: MonitoringMetric[];
};

export type MonitoringOverview = {
  window: string;
  catalog: MonitoringMetric[];
  series: MonitoringSeries[];
};

export type MonitoringWindow = '15m' | '1h' | '6h' | '24h' | '7d' | '30d';
export type MonitoringStep = '15s' | '60s' | '5m' | '1h';
```

Add functions near existing API functions:

```ts
export function getMonitoringCatalog(): Promise<MonitoringCatalog> {
  return apiFetch<MonitoringCatalog>('/api/monitoring/catalog');
}

export function getMonitoringOverview(window: MonitoringWindow): Promise<MonitoringOverview> {
  return apiFetch<MonitoringOverview>(`/api/monitoring/overview?window=${encodeURIComponent(window)}`);
}

export function getMonitoringSeries(metric: string, window: MonitoringWindow, step: MonitoringStep): Promise<MonitoringSeries> {
  const params = new URLSearchParams({ metric, window, step });
  return apiFetch<MonitoringSeries>(`/api/monitoring/series?${params.toString()}`);
}
```

- [ ] **Step 2: Add frontend monitoring helpers**

Create `frontend/src/lib/monitoring.ts` with:

```ts
import type { MonitoringMetric, MonitoringPoint, MonitoringSeries } from '$lib/api';

export function metricByName(metrics: MonitoringMetric[], name: string): MonitoringMetric | null {
  return metrics.find((metric) => metric.name === name) ?? null;
}

export function seriesByMetric(series: MonitoringSeries[], name: string): MonitoringSeries | null {
  return series.find((item) => item.metric === name) ?? null;
}

export function latestPoint(series: MonitoringSeries | null | undefined): MonitoringPoint | null {
  const points = series?.points ?? [];
  return points.length > 0 ? points[points.length - 1] : null;
}

export function formatMetricValue(value: number | null | undefined, unit: string): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '—';
  if (unit === 'percent') return `${Math.round(value * 100)}%`;
  if (unit === 'bytes') return formatBytes(value);
  if (unit === 'sessions_per_second') return `${value.toFixed(2)}/s`;
  return `${value}`;
}

export function formatBytes(value: number): string {
  if (value < 1024) return `${value.toFixed(0)} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let scaled = value / 1024;
  let unit = units[0];
  for (let index = 1; scaled >= 1024 && index < units.length; index += 1) {
    scaled /= 1024;
    unit = units[index];
  }
  return `${scaled.toFixed(1)} ${unit}`;
}
```

- [ ] **Step 3: Run frontend check and confirm existing pages still typecheck**

Run from `frontend/`:

```bash
npm run check
```

Expected: check passes or fails only because components/pages are not yet wired in later tasks. Fix any type syntax errors before continuing.

---

## Task 9: Shared Frontend Monitoring Components

**Owner:** Frontend admin UX agent.

**Files:**
- Create: `frontend/src/lib/MetricCard.svelte`
- Create: `frontend/src/lib/TimeSeriesChart.svelte`
- Create: `frontend/src/lib/UnavailableMetric.svelte`
- Create: `frontend/src/lib/WindowSelector.svelte`
- Modify: `frontend/src/app.css`

- [ ] **Step 1: Create `MetricCard.svelte`**

Create `frontend/src/lib/MetricCard.svelte` with:

```svelte
<script lang="ts">
  let { title, value, caption = '' }: { title: string; value: string; caption?: string } = $props();
</script>

<article class="panel stat-card metric-card">
  <span>{title}</span>
  <strong>{value}</strong>
  {#if caption}<small>{caption}</small>{/if}
</article>
```

- [ ] **Step 2: Create `UnavailableMetric.svelte`**

Create `frontend/src/lib/UnavailableMetric.svelte` with:

```svelte
<script lang="ts">
  let { title, reason = 'Metric is not collecting yet.' }: { title: string; reason?: string } = $props();
</script>

<article class="panel unavailable-metric">
  <span class="eyebrow">Not collecting yet</span>
  <h3>{title}</h3>
  <p class="muted">{reason}</p>
</article>
```

- [ ] **Step 3: Create `WindowSelector.svelte`**

Create `frontend/src/lib/WindowSelector.svelte` with:

```svelte
<script lang="ts">
  import type { MonitoringWindow } from '$lib/api';

  let { value = $bindable(), windows }: { value: MonitoringWindow; windows: MonitoringWindow[] } = $props();
</script>

<label class="window-selector">
  <span>Window</span>
  <select bind:value>
    {#each windows as window}
      <option value={window}>{window}</option>
    {/each}
  </select>
</label>
```

- [ ] **Step 4: Create `TimeSeriesChart.svelte`**

Create `frontend/src/lib/TimeSeriesChart.svelte` with:

```svelte
<script lang="ts">
  import type { MonitoringPoint } from '$lib/api';

  let { title, points = [], unit = '' }: { title: string; points?: MonitoringPoint[]; unit?: string } = $props();

  const width = 640;
  const height = 180;
  const padding = 18;

  const path = $derived(buildPath(points, width, height, padding));
  const empty = $derived(points.length === 0);

  function buildPath(values: MonitoringPoint[], chartWidth: number, chartHeight: number, inset: number): string {
    if (values.length === 0) return '';
    if (values.length === 1) {
      const y = chartHeight / 2;
      return `M ${inset} ${y} L ${chartWidth - inset} ${y}`;
    }
    const numeric = values.map((point) => point.value);
    const min = Math.min(...numeric);
    const max = Math.max(...numeric);
    const span = max - min || 1;
    return values
      .map((point, index) => {
        const x = inset + (index / (values.length - 1)) * (chartWidth - inset * 2);
        const y = chartHeight - inset - ((point.value - min) / span) * (chartHeight - inset * 2);
        return `${index === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`;
      })
      .join(' ');
  }
</script>

<article class="panel chart-card">
  <div class="chart-heading">
    <h3>{title}</h3>
    {#if unit}<span>{unit}</span>{/if}
  </div>
  {#if empty}
    <p class="muted">No samples returned for this window.</p>
  {:else}
    <svg viewBox={`0 0 ${width} ${height}`} role="img" aria-label={`${title} time series`}>
      <path class="chart-line" d={path} fill="none" />
    </svg>
  {/if}
</article>
```

- [ ] **Step 5: Add component styles**

Append to `frontend/src/app.css`:

```css
.metric-card strong {
  font-variant-numeric: tabular-nums;
}

.monitoring-toolbar {
  align-items: center;
  display: flex;
  gap: 1rem;
  justify-content: space-between;
}

.window-selector {
  align-items: center;
  display: inline-flex;
  gap: 0.5rem;
}

.window-selector select {
  border: 1px solid #d0d5dd;
  border-radius: 0.55rem;
  padding: 0.45rem 0.7rem;
}

.chart-grid {
  display: grid;
  gap: 1rem;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.chart-heading {
  align-items: baseline;
  display: flex;
  justify-content: space-between;
  gap: 1rem;
}

.chart-heading h3 {
  margin-top: 0;
}

.chart-card svg {
  display: block;
  width: 100%;
}

.chart-line {
  stroke: #175cd3;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 3;
}

.unavailable-metric {
  background: #f9fafb;
}

@media (max-width: 760px) {
  .chart-grid,
  .monitoring-toolbar {
    grid-template-columns: 1fr;
  }

  .monitoring-toolbar {
    align-items: stretch;
    display: grid;
  }
}
```

- [ ] **Step 6: Run frontend check**

Run from `frontend/`:

```bash
npm run check
```

Expected: frontend check passes.

---

## Task 10: Dashboard Monitoring UI

**Owner:** Frontend admin UX agent.

**Files:**
- Modify: `frontend/src/routes/+page.svelte`

- [ ] **Step 1: Wire Dashboard monitoring data**

Modify the `<script>` imports in `frontend/src/routes/+page.svelte` to include:

```ts
import MetricCard from '$lib/MetricCard.svelte';
import TimeSeriesChart from '$lib/TimeSeriesChart.svelte';
import UnavailableMetric from '$lib/UnavailableMetric.svelte';
import WindowSelector from '$lib/WindowSelector.svelte';
import { formatMetricValue, latestPoint, metricByName, seriesByMetric } from '$lib/monitoring';
import { getMonitoringOverview, type MonitoringOverview, type MonitoringWindow } from '$lib/api';
```

Add state:

```ts
let monitoringWindow = $state<MonitoringWindow>('1h');
let monitoring = $state<MonitoringOverview | null>(null);
```

In `loadDashboard`, after `summary = await getDashboardSummary();`, add:

```ts
monitoring = await getMonitoringOverview(monitoringWindow);
```

Add helpers in the script:

```ts
async function reloadMonitoring() {
  monitoring = await getMonitoringOverview(monitoringWindow);
}

function metricValue(name: string): string {
  const metric = metricByName(monitoring?.catalog ?? [], name);
  const point = latestPoint(seriesByMetric(monitoring?.series ?? [], name));
  return formatMetricValue(point?.value, metric?.unit ?? '');
}
```

- [ ] **Step 2: Add Dashboard monitoring markup**

Inside the authenticated `{:else if summary}` block, before `Recent audit`, add:

```svelte
<section class="panel page-stack">
  <div class="monitoring-toolbar">
    <div>
      <h3>Monitoring trends</h3>
      <p class="muted">Short-window host metrics from the local backend TSDB.</p>
    </div>
    <WindowSelector bind:value={monitoringWindow} windows={['15m', '1h', '6h']} />
    <button class="button" type="button" onclick={reloadMonitoring}>Refresh</button>
  </div>

  {#if monitoring}
    <div class="grid cards">
      <MetricCard title="CPU" value={metricValue('host_cpu_utilization')} />
      <MetricCard title="Memory" value={metricValue('host_memory_utilization')} />
      <MetricCard title="Network in" value={metricValue('net_interface_rx_bytes')} />
      <MetricCard title="Network out" value={metricValue('net_interface_tx_bytes')} />
    </div>

    <div class="chart-grid">
      {#each ['host_cpu_utilization', 'host_memory_utilization', 'net_interface_rx_bytes', 'net_interface_tx_bytes', 'vpn_interface_rx_bytes', 'vpn_interface_tx_bytes', 'active_sessions', 'concurrent_sessions'] as metricName}
        {@const metric = metricByName(monitoring.catalog, metricName)}
        {@const series = seriesByMetric(monitoring.series, metricName)}
        {#if metric?.state === 'unavailable'}
          <UnavailableMetric title={metric.title} reason={metric.reason} />
        {:else}
          <TimeSeriesChart title={metric?.title ?? metricName} points={series?.points ?? []} unit={metric?.unit ?? ''} />
        {/if}
      {/each}
    </div>
  {:else}
    <p class="muted">Monitoring overview was not returned yet.</p>
  {/if}
</section>
```

- [ ] **Step 3: Run frontend check**

Run from `frontend/`:

```bash
npm run check
```

Expected: frontend check passes. If Svelte rejects the inline array binding type, define `const dashboardWindows: MonitoringWindow[] = ['15m', '1h', '6h'];` in the script and pass `windows={dashboardWindows}`.

---

## Task 11: History Monitoring UI

**Owner:** Frontend admin UX agent.

**Files:**
- Modify: `frontend/src/routes/history/+page.svelte`

- [ ] **Step 1: Wire History monitoring data**

Modify imports in `frontend/src/routes/history/+page.svelte` to include:

```ts
import TimeSeriesChart from '$lib/TimeSeriesChart.svelte';
import UnavailableMetric from '$lib/UnavailableMetric.svelte';
import WindowSelector from '$lib/WindowSelector.svelte';
import { metricByName, seriesByMetric } from '$lib/monitoring';
import { getMonitoringOverview, type MonitoringOverview, type MonitoringWindow } from '$lib/api';
```

Add state:

```ts
let monitoringWindow = $state<MonitoringWindow>('24h');
let monitoring = $state<MonitoringOverview | null>(null);
const historyWindows: MonitoringWindow[] = ['24h', '7d', '30d'];
```

In `loadHistory`, after `analytics = await getHistoryAnalytics();`, add:

```ts
monitoring = await getMonitoringOverview(monitoringWindow);
```

Add helper:

```ts
async function reloadMonitoring() {
  monitoring = await getMonitoringOverview(monitoringWindow);
}
```

- [ ] **Step 2: Add History monitoring markup**

Inside the authenticated `{:else if analytics}` block, after the total traffic cards and before `Recent historical sessions`, add:

```svelte
<section class="panel page-stack">
  <div class="monitoring-toolbar">
    <div>
      <h3>Monitoring history</h3>
      <p class="muted">Longer-window trends from the local monitoring TSDB.</p>
    </div>
    <WindowSelector bind:value={monitoringWindow} windows={historyWindows} />
    <button class="button" type="button" onclick={reloadMonitoring}>Refresh</button>
  </div>

  {#if monitoring}
    <div class="chart-grid">
      {#each ['host_cpu_utilization', 'host_memory_utilization', 'net_interface_rx_bytes', 'net_interface_tx_bytes', 'vpn_interface_rx_bytes', 'vpn_interface_tx_bytes', 'user_rx_bytes', 'user_tx_bytes', 'connection_rate'] as metricName}
        {@const metric = metricByName(monitoring.catalog, metricName)}
        {@const series = seriesByMetric(monitoring.series, metricName)}
        {#if metric?.state === 'unavailable'}
          <UnavailableMetric title={metric.title} reason={metric.reason} />
        {:else}
          <TimeSeriesChart title={metric?.title ?? metricName} points={series?.points ?? []} unit={metric?.unit ?? ''} />
        {/if}
      {/each}
    </div>
  {:else}
    <p class="muted">Monitoring history was not returned yet.</p>
  {/if}
</section>
```

- [ ] **Step 3: Run frontend check**

Run from `frontend/`:

```bash
npm run check
```

Expected: frontend check passes and existing purge UI remains present.

---

## Task 12: Operations And Security Documentation

**Owner:** Documentation/ADR agent.

**Files:**
- Modify: `docs/OPERATIONS.md`
- Modify: `docs/SECURITY.md`
- Modify: `docs/ADR-0001-architecture.md`

- [ ] **Step 1: Update operations docs**

Add a Monitoring section to `docs/OPERATIONS.md`:

```markdown
## Local Monitoring TSDB

The monitoring slice uses an embedded Prometheus TSDB library inside the Go backend. It does not run a Prometheus server, Grafana, InfluxDB, VictoriaMetrics, or any external monitoring daemon.

Configuration:

- `OCSERV_PANEL_TSDB_PATH`: local TSDB data directory. Default: `./data/metrics` relative to the backend working directory.
- `OCSERV_PANEL_NET_IFACE`: optional primary network interface for host RX/TX counters.
- `OCSERV_PANEL_VPN_IFACE`: optional VPN interface for VPN RX/TX counters.

The collector interval is 15 seconds. Default monitoring retention is 30 days. CPU and memory collectors read `/proc`; interface collectors read `/sys/class/net/<iface>/statistics/rx_bytes` and `tx_bytes`. If an interface is unset, absent, or unreadable, the API reports that metric as unavailable instead of failing the rest of the panel.
```

- [ ] **Step 2: Update security docs**

Add to `docs/SECURITY.md`:

```markdown
## Monitoring Data Safety

Monitoring samples are numeric values stored in the embedded local TSDB. Labels must remain safe metadata such as metric name, interface name, direction, and future usernames from approved ocserv ingestion. Labels must not include passwords, raw session tokens, private keys, credentials, command output, raw request bodies, or certificate material.

The monitoring slice does not run shell commands, `sudo`, `systemctl`, `ocserv`, `occtl`, or `ocpasswd`. It does not access the ocserv management socket. ocserv-specific metrics that require later ingestion are represented as unavailable, not as fake live values.
```

- [ ] **Step 3: Update architecture ADR**

Add to `docs/ADR-0001-architecture.md`:

```markdown
### Embedded Monitoring TSDB Exception

The dashboard/history monitoring slice uses the Prometheus TSDB Go library as an embedded local dependency. This is an approved exception for time-series storage only. The project still does not run a Prometheus server, Grafana, external monitoring service, queue, or cloud control plane. SQLite remains the source of truth for application metadata, auth, audit, and historical session records.
```

- [ ] **Step 4: Review docs for architecture boundaries**

Search docs for contradictions with the new local embedded TSDB exception:

```bash
rg "Prometheus|Grafana|monitoring|TSDB" docs
```

Expected: docs consistently say embedded TSDB dependency is allowed, while external monitoring stacks remain out of scope.

---

## Task 13: Full Verification And Safety Review

**Owner:** Test/verification agent, then code-review agent for final review.

**Files:**
- No planned source edits unless verification discovers defects.

- [ ] **Step 1: Run backend tests**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go test ./...
```

Expected: all backend tests pass.

- [ ] **Step 2: Verify Go modules**

Run from `backend/`:

```bash
PATH="$HOME/.local/bin:$PATH" go mod tidy
PATH="$HOME/.local/bin:$PATH" go mod verify
```

Expected: `go mod tidy` leaves only intended dependency changes and `go mod verify` succeeds.

- [ ] **Step 3: Run frontend checks**

Run from `frontend/`:

```bash
npm run check
npm run build
```

Expected: frontend typecheck and production build pass.

- [ ] **Step 4: Run safety searches**

Run from repository root:

```bash
rg "os/exec|exec\.Command|child_process|sudo|systemctl|occtl|ocpasswd|management socket|/var/run/ocserv|private key|BEGIN PRIVATE KEY" backend frontend docs
```

Expected: no new live command execution, privileged service control, ocserv socket access, or private-key handling introduced by this slice. Existing documentation mentions may appear only as non-goals or warnings.

- [ ] **Step 5: Inspect git diff**

Run from repository root:

```bash
git diff --stat
git diff -- backend/go.mod backend/go.sum backend/internal/metrics backend/internal/monitoring backend/internal/app/monitoring_service.go backend/internal/server/server.go backend/cmd/ocserv-panel/main.go frontend/src/lib frontend/src/routes/+page.svelte frontend/src/routes/history/+page.svelte docs
```

Expected: diff is limited to monitoring TSDB, host collectors, monitoring API/UI, and docs. No unrelated config, certificate, sudo, systemd, or live ocserv integration changes.

---

## Plan Self-Review

- Spec coverage: The plan includes embedded Prometheus TSDB, host collectors, unavailable ocserv-only metric states, authenticated monitoring APIs, Dashboard/History shared components, operations/security docs, and final safety verification.
- Placeholder scan: No `TBD`, `TODO`, or undefined future behavior remains in task steps.
- Type consistency: Backend metric names and frontend string names match the approved metric contract. Monitoring API paths match the spec. Window and step values match the spec.
