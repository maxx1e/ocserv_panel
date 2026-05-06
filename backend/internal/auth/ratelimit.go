package auth

import (
	"sync"
	"time"
)

// RateLimiter tracks login attempts per IP using a sliding window.
type RateLimiter struct {
	mu          sync.Mutex
	maxAttempts int
	window      time.Duration
	attempts    map[string][]time.Time
}

// NewRateLimiter creates a rate limiter that allows maxAttempts per window per IP.
func NewRateLimiter(maxAttempts int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		maxAttempts: maxAttempts,
		window:      window,
		attempts:    make(map[string][]time.Time),
	}
}

// Allow checks whether the IP is allowed to make another attempt.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	rl.pruneExpired(ip, now)
	return len(rl.attempts[ip]) < rl.maxAttempts
}

// RecordFailure records a failed login attempt for the IP.
func (rl *RateLimiter) RecordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	rl.pruneExpired(ip, now)
	rl.attempts[ip] = append(rl.attempts[ip], now)
}

func (rl *RateLimiter) pruneExpired(ip string, now time.Time) {
	attempts := rl.attempts[ip]
	cutoff := now.Add(-rl.window)
	valid := attempts[:0]
	for _, t := range attempts {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	if len(valid) == 0 {
		delete(rl.attempts, ip)
	} else {
		rl.attempts[ip] = valid
	}
}
