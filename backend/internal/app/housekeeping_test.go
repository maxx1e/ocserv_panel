package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/rbac"
)

func TestHousekeepingSchedulerPurgesOnSchedule(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s, _ := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)

	var purgeCount atomic.Int32
	mockPurger := func(ctx context.Context, retentionDays int) (int64, error) {
		purgeCount.Add(1)
		return 0, nil
	}

	scheduler := NewHousekeepingScheduler(s, audit.NoopRecorder{}, mockPurger, 50*time.Millisecond)

	// Set retention to 365 days
	settingsService := NewSettingsService(s, audit.NoopRecorder{}, rbac.NewDefaultAuthorizer())
	if err := settingsService.SetRetentionDays(ctx, owner, 365); err != nil {
		t.Fatalf("SetRetentionDays: %v", err)
	}

	go scheduler.Run(ctx)

	// Wait for at least one purge cycle
	deadline := time.After(3 * time.Second)
	for purgeCount.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("housekeeping scheduler did not run within timeout")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestHousekeepingSchedulerUsesConfiguredRetention(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestServices(t)

	mockPurger := func(ctx context.Context, retentionDays int) (int64, error) {
		return 0, nil
	}

	scheduler := NewHousekeepingScheduler(s, audit.NoopRecorder{}, mockPurger, time.Hour)

	// Default retention should be 365
	days := scheduler.GetRetentionDays(ctx)
	if days != 365 {
		t.Fatalf("default retention = %d, want 365", days)
	}
}

func TestHousekeepingSchedulerStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	s, _ := newTestServices(t)
	var purgeCount atomic.Int32
	mockPurger := func(ctx context.Context, retentionDays int) (int64, error) {
		purgeCount.Add(1)
		return 0, nil
	}

	scheduler := NewHousekeepingScheduler(s, audit.NoopRecorder{}, mockPurger, 50*time.Millisecond)

	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx)
		close(done)
	}()

	// Wait for at least one purge
	deadline := time.After(3 * time.Second)
	for purgeCount.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("scheduler did not run within timeout")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Cancel context and verify Run returns
	cancel()
	select {
	case <-done:
		// Success: Run returned
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop after context cancellation")
	}
}
