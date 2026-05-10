package monitor

import (
	"context"
	"sync"
	"testing"
	"time"

	"ocserv_panel/backend/internal/vpn/sessions"
)

func TestSessionCollector_CollectsActiveSessions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	manager := sessions.NewFakeManager()
	manager.ServerStatusResult = &sessions.ServerStatus{
		Status:         "online",
		ActiveSessions: 5,
		TotalSessions:  42,
	}

	collector := NewSessionCollector(manager)
	result := collector.Collect(context.Background(), now)

	if result.Status != CollectStatusAvailable {
		t.Fatalf("status = %q, want %q; message = %q", result.Status, CollectStatusAvailable, result.Message)
	}
	if len(result.Samples) != 2 {
		t.Fatalf("samples length = %d, want 2", len(result.Samples))
	}

	active := result.Samples[0]
	if active.Name != MetricActiveSessions {
		t.Fatalf("sample[0] name = %q, want %q", active.Name, MetricActiveSessions)
	}
	if active.Value != 5 {
		t.Fatalf("sample[0] value = %v, want 5", active.Value)
	}
	if active.Timestamp != now {
		t.Fatalf("sample[0] timestamp = %v, want %v", active.Timestamp, now)
	}

	concurrent := result.Samples[1]
	if concurrent.Name != MetricConcurrentSessions {
		t.Fatalf("sample[1] name = %q, want %q", concurrent.Name, MetricConcurrentSessions)
	}
	if concurrent.Value != 5 {
		t.Fatalf("sample[1] value = %v, want 5", concurrent.Value)
	}
}

func TestSessionCollector_UnavailableWhenError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	manager := sessions.NewFakeManager()
	manager.ServerStatusErr = context.DeadlineExceeded

	collector := NewSessionCollector(manager)
	result := collector.Collect(context.Background(), now)

	if result.Status != CollectStatusUnavailable {
		t.Fatalf("status = %q, want %q", result.Status, CollectStatusUnavailable)
	}
	if len(result.Samples) != 0 {
		t.Fatalf("samples length = %d, want 0", len(result.Samples))
	}
}

func TestSessionCollector_UnavailableWhenNilStatus(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	manager := sessions.NewFakeManager()
	manager.ServerStatusResult = nil

	collector := NewSessionCollector(manager)
	result := collector.Collect(context.Background(), now)

	if result.Status != CollectStatusUnavailable {
		t.Fatalf("status = %q, want %q", result.Status, CollectStatusUnavailable)
	}
}

func TestSessionCollector_ContextCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	manager := sessions.NewFakeManager()
	manager.ServerStatusResult = &sessions.ServerStatus{ActiveSessions: 3}

	collector := NewSessionCollector(manager)
	result := collector.Collect(ctx, time.Now())

	if result.Status != CollectStatusUnavailable {
		t.Fatalf("status = %q, want %q", result.Status, CollectStatusUnavailable)
	}
}

type fakeNotifier struct {
	mu    sync.Mutex
	calls int
}

func (n *fakeNotifier) NotifySessionChange() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.calls++
}

func (n *fakeNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.calls
}

func TestSessionCollector_NotifierCalledOnCountChange(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	manager := sessions.NewFakeManager()
	manager.ServerStatusResult = &sessions.ServerStatus{ActiveSessions: 3}

	notifier := &fakeNotifier{}
	collector := NewSessionCollectorWithNotifier(manager, notifier)

	// First collect: lastCount is 0, active is 3 — count changed.
	collector.Collect(context.Background(), now)
	if notifier.count() != 1 {
		t.Fatalf("notifier calls = %d, want 1 after first collect", notifier.count())
	}

	// Second collect: same count (3) — no notification.
	collector.Collect(context.Background(), now.Add(time.Second))
	if notifier.count() != 1 {
		t.Fatalf("notifier calls = %d, want still 1 after same count", notifier.count())
	}

	// Third collect: count changed to 5.
	manager.ServerStatusResult = &sessions.ServerStatus{ActiveSessions: 5}
	collector.Collect(context.Background(), now.Add(2*time.Second))
	if notifier.count() != 2 {
		t.Fatalf("notifier calls = %d, want 2 after count increase", notifier.count())
	}
}

func TestSessionCollector_NotifierNotCalledOnError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	manager := sessions.NewFakeManager()
	manager.ServerStatusErr = context.DeadlineExceeded

	notifier := &fakeNotifier{}
	collector := NewSessionCollectorWithNotifier(manager, notifier)
	collector.Collect(context.Background(), now)

	if notifier.count() != 0 {
		t.Fatalf("notifier calls = %d, want 0 on error", notifier.count())
	}
}

func TestSessionCollector_NilNotifierIsSafe(t *testing.T) {
	t.Parallel()

	manager := sessions.NewFakeManager()
	manager.ServerStatusResult = &sessions.ServerStatus{ActiveSessions: 2}

	// NewSessionCollectorWithNotifier(provider, nil) must not panic.
	collector := NewSessionCollectorWithNotifier(manager, nil)
	result := collector.Collect(context.Background(), time.Now())
	if result.Status != CollectStatusAvailable {
		t.Fatalf("status = %q, want available", result.Status)
	}
}
