package monitoring

import (
	"context"
	"testing"
	"time"

	"ocserv_panel/backend/internal/metrics"
	"ocserv_panel/backend/internal/ocserv"
)

func TestSessionCollector_CollectsActiveSessions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	manager := ocserv.NewFakeManager()
	manager.ServerStatusResult = &ocserv.ServerStatus{
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
	if active.Name != metrics.MetricActiveSessions {
		t.Fatalf("sample[0] name = %q, want %q", active.Name, metrics.MetricActiveSessions)
	}
	if active.Value != 5 {
		t.Fatalf("sample[0] value = %v, want 5", active.Value)
	}
	if active.Timestamp != now {
		t.Fatalf("sample[0] timestamp = %v, want %v", active.Timestamp, now)
	}

	concurrent := result.Samples[1]
	if concurrent.Name != metrics.MetricConcurrentSessions {
		t.Fatalf("sample[1] name = %q, want %q", concurrent.Name, metrics.MetricConcurrentSessions)
	}
	if concurrent.Value != 5 {
		t.Fatalf("sample[1] value = %v, want 5", concurrent.Value)
	}
}

func TestSessionCollector_UnavailableWhenError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	manager := ocserv.NewFakeManager()
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
	manager := ocserv.NewFakeManager()
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

	manager := ocserv.NewFakeManager()
	manager.ServerStatusResult = &ocserv.ServerStatus{ActiveSessions: 3}

	collector := NewSessionCollector(manager)
	result := collector.Collect(ctx, time.Now())

	if result.Status != CollectStatusUnavailable {
		t.Fatalf("status = %q, want %q", result.Status, CollectStatusUnavailable)
	}
}
