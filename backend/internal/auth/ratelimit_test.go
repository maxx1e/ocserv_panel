package auth_test

import (
	"testing"
	"time"

	"ocserv_panel/backend/internal/auth"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := auth.NewRateLimiter(5, time.Minute)

	for i := 0; i < 5; i++ {
		rl.RecordFailure("10.0.0.1")
	}

	// 5 failures recorded; the 5th Allow should still have been fine before recording,
	// but now the 6th call should be blocked. Check that Allow was true before the limit.
	// Re-test with fresh limiter: 5 allows without any failures.
	rl2 := auth.NewRateLimiter(5, time.Minute)
	for i := 0; i < 5; i++ {
		if !rl2.Allow("10.0.0.1") {
			t.Fatalf("expected Allow to return true on attempt %d", i+1)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := auth.NewRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		rl.RecordFailure("10.0.0.1")
	}

	if rl.Allow("10.0.0.1") {
		t.Fatal("expected Allow to return false after 3 failures with limit of 3")
	}
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	rl := auth.NewRateLimiter(2, time.Minute)

	rl.RecordFailure("10.0.0.1")
	rl.RecordFailure("10.0.0.1")

	if rl.Allow("10.0.0.1") {
		t.Fatal("expected 10.0.0.1 to be blocked")
	}

	if !rl.Allow("10.0.0.2") {
		t.Fatal("expected 10.0.0.2 to be allowed (independent IP)")
	}
}

func TestRateLimiter_ResetsAfterWindow(t *testing.T) {
	rl := auth.NewRateLimiter(2, 50*time.Millisecond)

	rl.RecordFailure("10.0.0.1")
	rl.RecordFailure("10.0.0.1")

	if rl.Allow("10.0.0.1") {
		t.Fatal("expected Allow to return false before window expires")
	}

	time.Sleep(60 * time.Millisecond)

	if !rl.Allow("10.0.0.1") {
		t.Fatal("expected Allow to return true after window expires")
	}
}

func TestRateLimiter_RecordFailure(t *testing.T) {
	rl := auth.NewRateLimiter(5, time.Minute)

	for i := 0; i < 5; i++ {
		rl.RecordFailure("192.168.1.1")
	}

	if rl.Allow("192.168.1.1") {
		t.Fatal("expected Allow to return false after 5 recorded failures with limit of 5")
	}
}
