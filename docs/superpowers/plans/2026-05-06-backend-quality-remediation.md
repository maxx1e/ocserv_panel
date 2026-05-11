# Backend Quality Remediation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all Critical and Important findings from the backend code quality review — security vulnerabilities, dead code removal, implementation gaps, and critical test coverage.

**Architecture:** Four sequential phases: (1) Security hardening with input validation, rate limiting, and allow-lists; (2) Dead code removal; (3) Implementation gap fixes (concurrency, error handling, purge logic); (4) Test coverage for the highest-risk untested areas.

**Tech Stack:** Go 1.22+, SQLite, `net/http`, `crypto/rand`, `sync`, `regexp`

---

## File Structure

### New Files
- `backend/internal/ocserv/validate.go` — Username/password validation functions
- `backend/internal/ocserv/validate_test.go` — Validation tests
- `backend/internal/auth/ratelimit.go` — In-memory per-IP rate limiter
- `backend/internal/auth/ratelimit_test.go` — Rate limiter tests
- `backend/internal/ocserv/passwd_adapter_test.go` — OcpasswdAdapter tests with FakeCommandRunner

### Modified Files
- `backend/internal/server/server.go` — Add ReadTimeout/WriteTimeout, remove double-call pattern, wire rate limiter
- `backend/internal/server/ocserv_handlers.go` — Add input validation calls
- `backend/internal/app/ocserv_service.go` — Add input validation at service layer
- `backend/internal/app/settings_service.go` — Remove dead code (`GetAll`, `Setting`, `knownSettingKeys`, `GetRetentionDays`)
- `backend/internal/app/settings_config_apply.go` — Add binary path allow-list validation
- `backend/internal/store/store.go` — Remove dead code (`SettingDefinition`, `GetSettingDefinitions`), fix `PurgeHistoricalBefore`
- `backend/internal/rbac/rbac.go` — Remove unused permissions
- `backend/internal/ocserv/client.go` — Remove dead `User` struct and `Manager.ListUsers`
- `backend/internal/ocserv/network_inspector.go` — Remove dead `NewNetworkInspector`
- `backend/internal/metrics/types.go` — Add mutex to `Catalog`
- `backend/internal/app/monitoring_service.go` — Use new thread-safe catalog methods
- `backend/cmd/ocserv-panel/main.go` — Wire rate limiter, fix DSN logging, add timeouts
- `backend/internal/audit/sqlite.go` — Export sanitization for reuse
- `backend/internal/store/store.go` — Remove duplicate sanitization, import from audit

---

## Task 1: Input Validation for ocserv Usernames and Passwords

**Files:**
- Create: `backend/internal/ocserv/validate.go`
- Create: `backend/internal/ocserv/validate_test.go`
- Modify: `backend/internal/app/ocserv_service.go:179-218`

- [ ] **Step 1: Write failing tests for username validation**

```go
// backend/internal/ocserv/validate_test.go
package ocserv

import "testing"

func TestValidateUsername(t *testing.T) {
	valid := []string{"alice", "bob123", "user.name", "user_name", "user-name", "a"}
	for _, u := range valid {
		if err := ValidateUsername(u); err != nil {
			t.Errorf("ValidateUsername(%q) = %v, want nil", u, err)
		}
	}

	invalid := []string{
		"",                          // empty
		"ab:cd",                     // colon
		"user\nname",                // newline
		"user\x00name",             // null byte
		"../etc/shadow",            // path traversal
		"a b",                       // space
		string(make([]byte, 65)),   // too long (>64)
		"user@host",                // @ symbol
	}
	for _, u := range invalid {
		if err := ValidateUsername(u); err == nil {
			t.Errorf("ValidateUsername(%q) = nil, want error", u)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	valid := []string{"password123", "a", "complex!@#$%^&*()", string(make([]byte, 128))}
	for _, p := range valid {
		if err := ValidatePassword(p); err != nil {
			t.Errorf("ValidatePassword(%q) = %v, want nil", p[:min(len(p), 10)], err)
		}
	}

	invalid := []string{
		"",                            // empty
		"pass\nword",                  // newline
		"pass\rword",                  // carriage return
		string(make([]byte, 129)),     // too long (>128)
	}
	for _, p := range invalid {
		if err := ValidatePassword(p); err == nil {
			t.Errorf("ValidatePassword(len=%d) = nil, want error", len(p))
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/ocserv/ -run "TestValidateUsername|TestValidatePassword" -v`
Expected: FAIL — `ValidateUsername` and `ValidatePassword` undefined

- [ ] **Step 3: Implement validation functions**

```go
// backend/internal/ocserv/validate.go
package ocserv

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrUsernameEmpty     = errors.New("username must not be empty")
	ErrUsernameTooLong   = errors.New("username must be at most 64 characters")
	ErrUsernameInvalid   = errors.New("username contains invalid characters")
	ErrPasswordEmpty     = errors.New("password must not be empty")
	ErrPasswordTooLong   = errors.New("password must be at most 128 characters")
	ErrPasswordNewline   = errors.New("password must not contain newline characters")
)

// usernamePattern allows alphanumeric, dots, underscores, and hyphens.
var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// ValidateUsername checks that a username is safe for use with ocpasswd.
// Rules: non-empty, max 64 chars, only [a-zA-Z0-9._-].
func ValidateUsername(username string) error {
	if username == "" {
		return ErrUsernameEmpty
	}
	if len(username) > 64 {
		return ErrUsernameTooLong
	}
	if !usernamePattern.MatchString(username) {
		return fmt.Errorf("%w: must match [a-zA-Z0-9._-]", ErrUsernameInvalid)
	}
	return nil
}

// ValidatePassword checks that a password is safe for the ocpasswd stdin protocol.
// Rules: non-empty, max 128 chars, no newlines or carriage returns.
func ValidatePassword(password string) error {
	if password == "" {
		return ErrPasswordEmpty
	}
	if len(password) > 128 {
		return ErrPasswordTooLong
	}
	if strings.ContainsAny(password, "\n\r") {
		return ErrPasswordNewline
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/ocserv/ -run "TestValidateUsername|TestValidatePassword" -v`
Expected: PASS

- [ ] **Step 5: Wire validation into OcservService**

In `backend/internal/app/ocserv_service.go`, add validation calls at the start of `CreateOcservUser` and `UpdateOcservUserPassword`:

```go
// In CreateOcservUser, after the requirePerm call (line 181):
	if err := ocserv.ValidateUsername(username); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}
	if err := ocserv.ValidatePassword(password); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

// In UpdateOcservUserPassword, after the requirePerm call (line 209):
	if err := ocserv.ValidateUsername(username); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}
	if err := ocserv.ValidatePassword(newPassword); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}
```

Also add validation in `DeleteOcservUser` after requirePerm:

```go
	if err := ocserv.ValidateUsername(username); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}
```

- [ ] **Step 6: Run full test suite to ensure no regressions**

Run: `cd /opt/ocserv_panel/backend && go test ./... -count=1`
Expected: All packages PASS

- [ ] **Step 7: Commit**

```bash
git add backend/internal/ocserv/validate.go backend/internal/ocserv/validate_test.go backend/internal/app/ocserv_service.go
git commit -m "$(cat <<'EOF'
fix(security): add username and password input validation for ocserv operations

Prevents ocpasswd file corruption from usernames containing colons,
newlines, or null bytes. Prevents stdin protocol confusion from
passwords containing newline characters. Enforces length limits.
EOF
)"
```

---

## Task 2: Login Rate Limiter

**Files:**
- Create: `backend/internal/auth/ratelimit.go`
- Create: `backend/internal/auth/ratelimit_test.go`
- Modify: `backend/internal/server/server.go`
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 1: Write failing tests for the rate limiter**

```go
// backend/internal/auth/ratelimit_test.go
package auth

import (
	"testing"
	"time"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	for i := 0; i < 5; i++ {
		if !rl.Allow("192.168.1.1") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		rl.Allow("10.0.0.1")
	}
	if rl.Allow("10.0.0.1") {
		t.Fatal("4th attempt should be blocked")
	}
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	rl.Allow("10.0.0.1")
	rl.Allow("10.0.0.1")
	if rl.Allow("10.0.0.1") {
		t.Fatal("3rd attempt for IP1 should be blocked")
	}
	if !rl.Allow("10.0.0.2") {
		t.Fatal("IP2 should still be allowed")
	}
}

func TestRateLimiter_ResetsAfterWindow(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)
	rl.Allow("10.0.0.1")
	rl.Allow("10.0.0.1")
	if rl.Allow("10.0.0.1") {
		t.Fatal("should be blocked before window expires")
	}
	time.Sleep(60 * time.Millisecond)
	if !rl.Allow("10.0.0.1") {
		t.Fatal("should be allowed after window expires")
	}
}

func TestRateLimiter_RecordFailure(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)
	rl.RecordFailure("10.0.0.1")
	rl.RecordFailure("10.0.0.1")
	rl.RecordFailure("10.0.0.1")
	rl.RecordFailure("10.0.0.1")
	rl.RecordFailure("10.0.0.1")
	if rl.Allow("10.0.0.1") {
		t.Fatal("should be blocked after 5 failures")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/auth/ -run "TestRateLimiter" -v`
Expected: FAIL — `NewRateLimiter` undefined

- [ ] **Step 3: Implement the rate limiter**

```go
// backend/internal/auth/ratelimit.go
package auth

import (
	"sync"
	"time"
)

// RateLimiter tracks login attempts per IP using a sliding window.
type RateLimiter struct {
	mu         sync.Mutex
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
// Returns true if under the limit.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	rl.pruneExpired(ip, now)

	if len(rl.attempts[ip]) >= rl.maxAttempts {
		return false
	}
	return true
}

// RecordFailure records a failed login attempt for the IP.
func (rl *RateLimiter) RecordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	rl.pruneExpired(ip, now)
	rl.attempts[ip] = append(rl.attempts[ip], now)
}

// pruneExpired removes attempts older than the window. Must be called with mu held.
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/auth/ -run "TestRateLimiter" -v`
Expected: PASS

- [ ] **Step 5: Wire rate limiter into login handler in server.go**

In `backend/internal/server/server.go`, add a `loginLimiter` field to the `App` struct and check it in `handleLogin`:

Add to `App` struct:
```go
	loginLimiter *auth.RateLimiter
```

Add to `NewAppRouter` (or wherever App is constructed) — pass it as a parameter via `AppOptions`:
```go
type AppOptions struct {
	CookieSecure bool
	LoginLimiter *auth.RateLimiter
}
```

In `handleLogin`, before processing the request body:
```go
	if a.loginLimiter != nil {
		ip := extractIP(r)
		if !a.loginLimiter.Allow(ip) {
			writeError(w, http.StatusTooManyRequests, "too many login attempts, try again later")
			return
		}
	}
```

After a failed login (wrong credentials), record the failure:
```go
	if a.loginLimiter != nil {
		a.loginLimiter.RecordFailure(ip)
	}
```

Add `extractIP` helper:
```go
func extractIP(r *http.Request) string {
	// Use X-Forwarded-For if behind reverse proxy, otherwise RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
```

- [ ] **Step 6: Wire in main.go**

In `backend/cmd/ocserv-panel/main.go`, create the limiter and pass via options:
```go
	loginLimiter := auth.NewRateLimiter(5, time.Minute)
	// ... pass to server.AppOptions{CookieSecure: cookieSecureFromEnv(), LoginLimiter: loginLimiter}
```

- [ ] **Step 7: Run full test suite**

Run: `cd /opt/ocserv_panel/backend && go test ./... -count=1`
Expected: All packages PASS

- [ ] **Step 8: Commit**

```bash
git add backend/internal/auth/ratelimit.go backend/internal/auth/ratelimit_test.go backend/internal/server/server.go backend/cmd/ocserv-panel/main.go
git commit -m "$(cat <<'EOF'
fix(security): add per-IP login rate limiter

Limits login attempts to 5 per minute per IP address using a sliding
window. Prevents brute-force password guessing attacks. Failed attempts
are tracked; successful logins do not consume the budget.
EOF
)"
```

---

## Task 3: Binary Path and Unit Name Allow-List Validation

**Files:**
- Modify: `backend/internal/app/settings_config_apply.go`
- Modify: `backend/internal/app/settings_config_apply_test.go`
- Modify: `backend/internal/app/settings_service.go`

- [ ] **Step 1: Write failing tests for path validation**

```go
// Add to backend/internal/app/settings_config_apply_test.go

func TestValidateSettingValue_BinaryPathAllowList(t *testing.T) {
	binPathKeys := []string{
		"installation.ocserv_bin_path",
		"installation.occtl_bin_path",
		"installation.ocpasswd_bin_path",
	}
	allowedPaths := []string{
		"/usr/bin/ocserv",
		"/usr/local/bin/occtl",
		"/usr/sbin/ocserv",
		"/usr/local/sbin/ocserv",
	}
	blockedPaths := []string{
		"/tmp/evil",
		"../relative",
		"ocserv",
		"/home/user/bin/ocserv",
		"/var/tmp/hack",
	}

	for _, key := range binPathKeys {
		for _, path := range allowedPaths {
			if err := validateSettingValue(key, path); err != nil {
				t.Errorf("validateSettingValue(%q, %q) = %v, want nil", key, path, err)
			}
		}
		for _, path := range blockedPaths {
			if err := validateSettingValue(key, path); err == nil {
				t.Errorf("validateSettingValue(%q, %q) = nil, want error", key, path)
			}
		}
	}
}

func TestValidateSettingValue_SystemdUnitName(t *testing.T) {
	allowed := []string{"ocserv", "ocserv.service", "ocserv-vpn", "ocserv_custom"}
	for _, name := range allowed {
		if err := validateSettingValue("service.systemd_unit_name", name); err != nil {
			t.Errorf("validateSettingValue(unit, %q) = %v, want nil", name, err)
		}
	}
	blocked := []string{"sshd", "nginx", "ssh.service", "ocserv; rm -rf /", ""}
	for _, name := range blocked {
		if err := validateSettingValue("service.systemd_unit_name", name); err == nil {
			t.Errorf("validateSettingValue(unit, %q) = nil, want error", name)
		}
	}
}

func TestValidateSettingValue_LogPathAllowList(t *testing.T) {
	allowed := []string{"/var/log/ocserv.log", "/var/log/ocserv-panel/app.log", "/opt/ocserv_panel/panel.log"}
	for _, p := range allowed {
		if err := validateSettingValue("installation.ocserv_log_path", p); err != nil {
			t.Errorf("validateSettingValue(log_path, %q) = %v, want nil", p, err)
		}
	}
	blocked := []string{"/etc/shadow", "/etc/passwd", "../../../etc/shadow", "relative.log"}
	for _, p := range blocked {
		if err := validateSettingValue("installation.ocserv_log_path", p); err == nil {
			t.Errorf("validateSettingValue(log_path, %q) = nil, want error", p)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/app/ -run "TestValidateSettingValue_BinaryPath|TestValidateSettingValue_Systemd|TestValidateSettingValue_LogPath" -v`
Expected: FAIL — `validateSettingValue` does not exist or doesn't reject bad values

- [ ] **Step 3: Implement validation in settings_config_apply.go**

Add to `backend/internal/app/settings_config_apply.go`:

```go
import (
	"path/filepath"
	"regexp"
)

// allowedBinDirs restricts binary path settings to system directories.
var allowedBinDirs = []string{
	"/usr/bin",
	"/usr/local/bin",
	"/usr/sbin",
	"/usr/local/sbin",
}

// allowedLogDirs restricts log path settings to safe directories.
var allowedLogDirs = []string{
	"/var/log",
	"/opt/ocserv_panel",
	"/var/log/ocserv",
	"/var/log/ocserv-panel",
}

// systemdUnitPattern only allows unit names starting with "ocserv".
var systemdUnitPattern = regexp.MustCompile(`^ocserv[a-zA-Z0-9._-]*$`)

// validateSettingValue performs security-aware validation for sensitive settings.
// Returns nil for settings that need no special validation.
func validateSettingValue(key, value string) error {
	switch key {
	case "installation.ocserv_bin_path", "installation.occtl_bin_path", "installation.ocpasswd_bin_path":
		return validateBinaryPath(value)
	case "service.systemd_unit_name":
		return validateSystemdUnitName(value)
	case "installation.ocserv_log_path", "installation.panel_log_path":
		return validateLogPath(value)
	}
	return nil
}

func validateBinaryPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%w: binary path must be absolute", ErrInvalidInput)
	}
	cleaned := filepath.Clean(path)
	dir := filepath.Dir(cleaned)
	for _, allowed := range allowedBinDirs {
		if dir == allowed {
			return nil
		}
	}
	return fmt.Errorf("%w: binary path must be in /usr/bin, /usr/local/bin, /usr/sbin, or /usr/local/sbin", ErrInvalidInput)
}

func validateSystemdUnitName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: systemd unit name must not be empty", ErrInvalidInput)
	}
	if !systemdUnitPattern.MatchString(name) {
		return fmt.Errorf("%w: systemd unit name must start with 'ocserv' and contain only [a-zA-Z0-9._-]", ErrInvalidInput)
	}
	return nil
}

func validateLogPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%w: log path must be absolute", ErrInvalidInput)
	}
	cleaned := filepath.Clean(path)
	for _, allowed := range allowedLogDirs {
		if strings.HasPrefix(cleaned, allowed+"/") || cleaned == allowed {
			return nil
		}
	}
	return fmt.Errorf("%w: log path must be within /var/log or /opt/ocserv_panel", ErrInvalidInput)
}
```

- [ ] **Step 4: Wire validation into UpdateSetting in settings_service.go**

In `backend/internal/app/settings_service.go`, in the `UpdateSetting` method, add a call to `validateSettingValue` before applying the setting:

```go
	// Add after existing value-format validation and before applySettingToConfig:
	if err := validateSettingValue(key, value); err != nil {
		return err
	}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/app/ -run "TestValidateSettingValue" -v`
Expected: PASS

- [ ] **Step 6: Run full test suite**

Run: `cd /opt/ocserv_panel/backend && go test ./... -count=1`
Expected: All packages PASS

- [ ] **Step 7: Commit**

```bash
git add backend/internal/app/settings_config_apply.go backend/internal/app/settings_config_apply_test.go backend/internal/app/settings_service.go
git commit -m "$(cat <<'EOF'
fix(security): add allow-list validation for binary paths, unit names, and log paths

Binary path settings are restricted to /usr/bin, /usr/local/bin,
/usr/sbin, /usr/local/sbin. Systemd unit name must start with
'ocserv'. Log paths restricted to /var/log and /opt/ocserv_panel.
Prevents arbitrary binary execution and arbitrary file read via
settings modification.
EOF
)"
```

---

## Task 4: HTTP Server Timeout Configuration

**Files:**
- Modify: `backend/cmd/ocserv-panel/main.go:177-181`

- [ ] **Step 1: Add ReadTimeout and WriteTimeout to http.Server**

In `backend/cmd/ocserv-panel/main.go`, update the server declaration:

```go
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.NewAppRouter(/* ... existing args ... */),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
	}
```

- [ ] **Step 2: Redact DSN from startup log**

Replace line 38:
```go
	slog.Info("sqlite database ready", "dsn", dbDSN)
```
With:
```go
	slog.Info("sqlite database ready", "dsn_set", dbDSN != "")
```

- [ ] **Step 3: Run full test suite and verify build**

Run: `cd /opt/ocserv_panel/backend && go build ./... && go test ./... -count=1`
Expected: Build succeeds, all tests pass

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/ocserv-panel/main.go
git commit -m "$(cat <<'EOF'
fix(security): add HTTP server timeouts and redact DSN from logs

Adds ReadTimeout (30s) and WriteTimeout (60s) to prevent slow-body DoS.
Removes DSN value from startup log to avoid accidental credential exposure.
EOF
)"
```

---

## Task 5: Remove Dead Code from store.go

**Files:**
- Modify: `backend/internal/store/store.go:304-348`

- [ ] **Step 1: Remove `SettingDefinition` type and `GetSettingDefinitions` method**

Delete lines defining `SettingDefinition` struct (lines 304-316) and `GetSettingDefinitions` method (lines 318-348) from `backend/internal/store/store.go`.

- [ ] **Step 2: Remove the `database/sql` import if it becomes unused**

Check if `sql.NullString` is still used elsewhere in the file. If not, remove the `"database/sql"` import.

- [ ] **Step 3: Run tests to verify no regressions**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/store/ -v`
Expected: PASS — no test references these symbols

- [ ] **Step 4: Commit**

```bash
git add backend/internal/store/store.go
git commit -m "$(cat <<'EOF'
refactor: remove dead SettingDefinition type and GetSettingDefinitions from store

These were leftover from a database-backed settings design that was
replaced by the in-code registry approach. No code referenced them.
EOF
)"
```

---

## Task 6: Remove Dead Code from settings_service.go

**Files:**
- Modify: `backend/internal/app/settings_service.go:27-160`
- Modify: `backend/internal/app/settings_service_test.go`

- [ ] **Step 1: Remove dead symbols**

Remove from `backend/internal/app/settings_service.go`:
- `Setting` struct (lines 28-31)
- `GetRetentionDays` method (lines 76-89) — duplicated in housekeeping.go
- `knownSettingKeys` var (lines 118-121)
- `GetAll` method (lines 124-152)
- `defaultForSetting` function (lines 155-165)

- [ ] **Step 2: Remove or update tests that reference deleted symbols**

In `backend/internal/app/settings_service_test.go`, remove tests for `GetAll` and `GetRetentionDays` if they exist.

In `backend/internal/app/settings_service_ext_test.go`, check and remove references to these deleted symbols.

- [ ] **Step 3: Verify compilation and tests**

Run: `cd /opt/ocserv_panel/backend && go build ./... && go test ./internal/app/ -count=1`
Expected: Build succeeds, all tests pass

- [ ] **Step 4: Commit**

```bash
git add backend/internal/app/settings_service.go backend/internal/app/settings_service_test.go backend/internal/app/settings_service_ext_test.go
git commit -m "$(cat <<'EOF'
refactor: remove dead GetAll, Setting, GetRetentionDays from SettingsService

GetAll was superseded by GetAllSections. GetRetentionDays was
independently duplicated in HousekeepingScheduler. Neither was wired
to any HTTP handler in production.
EOF
)"
```

---

## Task 7: Remove Dead RBAC Permissions and Dead ocserv Types

**Files:**
- Modify: `backend/internal/rbac/rbac.go`
- Modify: `backend/internal/rbac/rbac_test.go`
- Modify: `backend/internal/ocserv/client.go`
- Modify: `backend/internal/ocserv/client_test.go`
- Modify: `backend/internal/ocserv/network_inspector.go`

- [ ] **Step 1: Remove unused RBAC permissions**

Remove from `backend/internal/rbac/rbac.go`:
- Constants: `PermissionManageUsers`, `PermissionManageConfig`, `PermissionManageService`, `PermissionSettingsExport`, `PermissionSettingsBackup`
- Remove their entries from the `RoleOwner` grants map in `NewDefaultAuthorizer()`

- [ ] **Step 2: Remove dead `User` struct and `Manager.ListUsers` from client.go**

In `backend/internal/ocserv/client.go`:
- Remove the `User` struct (lines 31-35)
- Remove `ListUsers(ctx context.Context) ([]User, error)` from the `Manager` interface
- Remove `ListUsers` from `FakeManager` (the implementation method and the `Users []User` field)
- Remove `ListUsers` from `OcctlAdapter` if it exists

- [ ] **Step 3: Remove dead `NewNetworkInspector` constructor**

In `backend/internal/ocserv/network_inspector.go`, remove the `NewNetworkInspector` function (lines 37-39).

- [ ] **Step 4: Update tests that reference removed symbols**

In `backend/internal/rbac/rbac_test.go`, remove test cases for deleted permissions.
In `backend/internal/ocserv/client_test.go`, remove references to `Users` field or `ListUsers` method on FakeManager.

- [ ] **Step 5: Verify compilation and tests**

Run: `cd /opt/ocserv_panel/backend && go build ./... && go test ./... -count=1`
Expected: All packages PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/rbac/rbac.go backend/internal/rbac/rbac_test.go backend/internal/ocserv/client.go backend/internal/ocserv/client_test.go backend/internal/ocserv/network_inspector.go
git commit -m "$(cat <<'EOF'
refactor: remove dead RBAC permissions, User struct, and unused constructors

Removes PermissionManageUsers/Config/Service (superseded by granular
PermissionOcserv* permissions), PermissionSettingsExport/Backup (never
checked), dead User struct and Manager.ListUsers (unreachable via service
layer), and NewNetworkInspector zero-arg constructor (unused).
EOF
)"
```

---

## Task 8: Add Thread Safety to Metrics Catalog

**Files:**
- Modify: `backend/internal/metrics/types.go:67-108`
- Modify: `backend/internal/app/monitoring_service.go`

- [ ] **Step 1: Add sync.RWMutex to Catalog**

In `backend/internal/metrics/types.go`, update the `Catalog` struct:

```go
import "sync"

type Catalog struct {
	mu      sync.RWMutex
	metrics map[MetricName]MetricDefinition
}
```

Update `Find` to acquire a read lock:
```go
func (c *Catalog) Find(name MetricName) (MetricDefinition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	metric, ok := c.metrics[name]
	return metric, ok
}
```

Update `SetState` to acquire a write lock:
```go
func (c *Catalog) SetState(name MetricName, state State, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if def, ok := c.metrics[name]; ok {
		def.State = state
		def.Reason = reason
		c.metrics[name] = def
	}
}
```

Add a `List` method for safe iteration:
```go
// List returns a snapshot of all metric definitions.
func (c *Catalog) List() []MetricDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	defs := make([]MetricDefinition, 0, len(c.metrics))
	for _, d := range c.metrics {
		defs = append(defs, d)
	}
	return defs
}
```

Note: `DefaultCatalog()` now returns `*Catalog` (pointer) since it has a mutex:
```go
func DefaultCatalog() *Catalog {
	metrics := []MetricDefinition{ /* ... same as before ... */ }
	catalog := &Catalog{metrics: make(map[MetricName]MetricDefinition, len(metrics))}
	for _, metric := range metrics {
		catalog.metrics[metric.Name] = metric
	}
	return catalog
}
```

- [ ] **Step 2: Update MonitoringService to use pointer Catalog and new List method**

In `backend/internal/app/monitoring_service.go`, update:
- The `catalog` field type from `metrics.Catalog` to `*metrics.Catalog`
- `catalogList()` to use `s.catalog.List()`
- `SetMetricState` — it can now call `s.catalog.SetState` directly (already thread-safe)

- [ ] **Step 3: Update main.go if needed**

If `DefaultCatalog` signature changed (returns pointer now), update the caller in `monitoring_service.go` constructor.

- [ ] **Step 4: Run tests including race detector**

Run: `cd /opt/ocserv_panel/backend && go test -race ./internal/metrics/ ./internal/app/ -count=1`
Expected: PASS with no race conditions detected

- [ ] **Step 5: Commit**

```bash
git add backend/internal/metrics/types.go backend/internal/app/monitoring_service.go
git commit -m "$(cat <<'EOF'
fix(concurrency): add RWMutex to metrics Catalog for thread safety

Catalog.SetState and Catalog.Find can now be safely called from
concurrent goroutines. Prevents potential map read/write panic if
SetMetricState is called after the HTTP server starts (e.g. from
dynamic VPN interface discovery).
EOF
)"
```

---

## Task 9: Remove Double-Call Pattern in Auth Failure Handlers

**Files:**
- Modify: `backend/internal/server/server.go`
- Modify: `backend/internal/server/ocserv_handlers.go`
- Modify: `backend/internal/server/config_handlers.go`

- [ ] **Step 1: Remove phantom service calls after auth failure**

In `backend/internal/server/server.go`, find all handlers with this pattern:
```go
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.someService.Method(r.Context(), auth.Principal{})
		return
	}
```

Replace each with simply:
```go
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
```

Apply this to:
- `handleDashboardSummary` (server.go)
- `handleHistoryAnalytics` (server.go)
- `handleHistoryPurge` (server.go)
- `handleMonitoringCatalog` (server.go)
- `handleMonitoringOverview` (server.go)
- `handleMonitoringSeries` (server.go)

In `backend/internal/server/ocserv_handlers.go`:
- `handleOcservUserCreate`
- `handleOcservUserDelete`
- `handleOcservUserUpdatePassword`
- `handleOcservSessions`
- `handleOcservServiceStatus`
- `handleOcservServiceAction`
- Any other handlers with the same pattern

In `backend/internal/server/config_handlers.go`:
- All handlers with the same pattern

- [ ] **Step 2: Verify compilation**

Run: `cd /opt/ocserv_panel/backend && go build ./...`
Expected: Successful

- [ ] **Step 3: Run full test suite**

Run: `cd /opt/ocserv_panel/backend && go test ./... -count=1`
Expected: All packages PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/server/server.go backend/internal/server/ocserv_handlers.go backend/internal/server/config_handlers.go
git commit -m "$(cat <<'EOF'
fix: remove double-call pattern on authentication failure

Handlers were calling service methods with empty Principal{} after
auth failure. This created phantom audit 'denied' records and wasted
computation. The authenticate() method already writes the 401 response,
so handlers now simply return after auth failure.
EOF
)"
```

---

## Task 10: Fix PurgeHistoricalBefore to Use SQL-Level DELETE

**Files:**
- Modify: `backend/internal/store/store.go` (PurgeHistoricalBefore method)

- [ ] **Step 1: Write a test for the optimized purge**

The existing tests should continue to pass. Add a test that verifies large-dataset behavior doesn't load all rows:

```go
// In backend/internal/store/store_test.go, add or verify:
func TestPurgeHistoricalBefore_SQLDirect(t *testing.T) {
	// Insert 100 records with known timestamps, purge half, verify count
	// This test validates the SQL-based approach works correctly
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for i := 0; i < 100; i++ {
		disconnectedAt := now.Add(-time.Duration(i) * time.Hour)
		s.RecordHistoricalSession(ctx, store.HistoricalSession{
			Username:       fmt.Sprintf("user%d", i),
			DisconnectedAt: disconnectedAt,
		})
	}
	cutoff := now.Add(-50 * time.Hour)
	deleted, err := s.PurgeHistoricalBefore(ctx, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if deleted < 49 {
		t.Errorf("expected ~49 deleted, got %d", deleted)
	}
}
```

- [ ] **Step 2: Rewrite PurgeHistoricalBefore to use SQL WHERE clause**

Replace the current implementation in `backend/internal/store/store.go`:

```go
// PurgeHistoricalBefore deletes only historical sessions disconnected before cutoff.
func (s *Store) PurgeHistoricalBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	cutoffStr := cutoff.UTC().Format(time.RFC3339)
	result, err := s.DB.ExecContext(ctx, `DELETE FROM historical_sessions WHERE disconnected_at < ?`, cutoffStr)
	if err != nil {
		return 0, fmt.Errorf("purge historical sessions: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read purged historical session count: %w", err)
	}
	return deleted, nil
}
```

Note: This requires that `disconnected_at` is stored in RFC3339 format for string comparison to work correctly. Verify by checking `RecordHistoricalSession` — if it stores via `time.Format(time.RFC3339)`, this approach is safe. If it uses a different format, match that format.

- [ ] **Step 3: Run tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/store/ -v -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/store/store.go backend/internal/store/store_test.go
git commit -m "$(cat <<'EOF'
fix(perf): replace in-memory purge with SQL-level DELETE WHERE

PurgeHistoricalBefore was loading all rows into memory then filtering
in Go. For large datasets this could OOM. Now uses a single DELETE
with a WHERE clause on disconnected_at, keeping memory constant.
EOF
)"
```

---

## Task 11: OcpasswdAdapter Tests with FakeCommandRunner

**Files:**
- Create: `backend/internal/ocserv/passwd_adapter_test.go`

- [ ] **Step 1: Write comprehensive tests for OcpasswdAdapter**

```go
// backend/internal/ocserv/passwd_adapter_test.go
package ocserv

import (
	"context"
	"strings"
	"testing"
)

func TestOcpasswdAdapter_CreateUser_Success(t *testing.T) {
	fake := &FakeCommandRunner{
		Results: map[string]CommandResult{
			"*": {ExitCode: 0, Stdout: "", Stderr: ""},
		},
	}
	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/etc/ocserv/passwd", "/etc/ocserv/ocserv.conf", fake)

	err := adapter.CreateUser(context.Background(), "testuser", "secret123")
	if err != nil {
		t.Fatalf("CreateUser() = %v, want nil", err)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fake.Calls))
	}
	call := fake.Calls[0]
	if call.Bin != "/usr/local/bin/ocpasswd" {
		t.Errorf("bin = %q, want /usr/local/bin/ocpasswd", call.Bin)
	}
	// Args should be: -c /etc/ocserv/ocserv.conf testuser
	wantArgs := []string{"-c", "/etc/ocserv/ocserv.conf", "testuser"}
	if strings.Join(call.Args, " ") != strings.Join(wantArgs, " ") {
		t.Errorf("args = %v, want %v", call.Args, wantArgs)
	}
	// Stdin should contain password twice (separated by newlines)
	if call.Stdin == nil {
		t.Fatal("stdin should not be nil")
	}
}

func TestOcpasswdAdapter_CreateUser_Failure(t *testing.T) {
	fake := &FakeCommandRunner{
		Results: map[string]CommandResult{
			"*": {ExitCode: 1, Stdout: "", Stderr: "user already exists"},
		},
	}
	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/etc/ocserv/passwd", "/etc/ocserv/ocserv.conf", fake)

	err := adapter.CreateUser(context.Background(), "testuser", "secret123")
	if err == nil {
		t.Fatal("CreateUser() = nil, want error")
	}
	if !strings.Contains(err.Error(), "user already exists") {
		t.Errorf("error = %q, want to contain 'user already exists'", err.Error())
	}
}

func TestOcpasswdAdapter_DeleteUser_Success(t *testing.T) {
	fake := &FakeCommandRunner{
		Results: map[string]CommandResult{
			"*": {ExitCode: 0},
		},
	}
	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/etc/ocserv/passwd", "/etc/ocserv/ocserv.conf", fake)

	err := adapter.DeleteUser(context.Background(), "testuser")
	if err != nil {
		t.Fatalf("DeleteUser() = %v, want nil", err)
	}
	call := fake.Calls[0]
	wantArgs := []string{"-c", "/etc/ocserv/ocserv.conf", "-d", "testuser"}
	if strings.Join(call.Args, " ") != strings.Join(wantArgs, " ") {
		t.Errorf("args = %v, want %v", call.Args, wantArgs)
	}
}

func TestOcpasswdAdapter_DeleteUser_Failure(t *testing.T) {
	fake := &FakeCommandRunner{
		Results: map[string]CommandResult{
			"*": {ExitCode: 1, Stderr: "user not found"},
		},
	}
	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/etc/ocserv/passwd", "/etc/ocserv/ocserv.conf", fake)

	err := adapter.DeleteUser(context.Background(), "nosuchuser")
	if err == nil {
		t.Fatal("DeleteUser() = nil, want error")
	}
}

func TestOcpasswdAdapter_UpdatePassword_Success(t *testing.T) {
	fake := &FakeCommandRunner{
		Results: map[string]CommandResult{
			"*": {ExitCode: 0},
		},
	}
	adapter := NewOcpasswdAdapter("/usr/local/bin/ocpasswd", "/etc/ocserv/passwd", "/etc/ocserv/ocserv.conf", fake)

	err := adapter.UpdatePassword(context.Background(), "testuser", "newpass")
	if err != nil {
		t.Fatalf("UpdatePassword() = %v, want nil", err)
	}
}
```

- [ ] **Step 2: Verify FakeCommandRunner supports these tests**

Check that `FakeCommandRunner` in `runner.go` has a `Calls` field and a wildcard `"*"` key in Results. If not, add the wildcard match or adjust test setup to match the existing fake API.

- [ ] **Step 3: Run the tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/ocserv/ -run "TestOcpasswdAdapter" -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/ocserv/passwd_adapter_test.go
git commit -m "$(cat <<'EOF'
test: add OcpasswdAdapter tests using FakeCommandRunner

Covers CreateUser, DeleteUser, UpdatePassword success and failure
paths. Verifies correct argument construction and stdin handling.
EOF
)"
```

---

## Task 12: Consolidate Duplicate Metadata Sanitization

**Files:**
- Modify: `backend/internal/audit/sqlite.go`
- Modify: `backend/internal/store/store.go`

- [ ] **Step 1: Export sanitization from audit package**

In `backend/internal/audit/sqlite.go`, rename internal sanitization functions to exported versions:
- `sanitizeMetadata` → `SanitizeMetadata`
- `isSensitiveMetadataKey` → keep unexported (internal detail)
- `isSensitiveMetadataValue` → keep unexported (internal detail)

Add a doc comment:
```go
// SanitizeMetadata redacts sensitive values from audit metadata before storage.
// Keys and values matching known sensitive patterns are replaced with "[REDACTED]".
func SanitizeMetadata(meta map[string]string) map[string]string {
	// ... existing implementation ...
}
```

- [ ] **Step 2: Remove duplicate from store.go**

In `backend/internal/store/store.go`:
- Remove `sanitizeAuditMetadata`, `isSensitiveMetadataKey`, `isSensitiveMetadataValue`, `normalizeMetadataKey`
- Replace call sites with `audit.SanitizeMetadata`

- [ ] **Step 3: Verify compilation and tests**

Run: `cd /opt/ocserv_panel/backend && go build ./... && go test ./... -count=1`
Expected: All packages PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/audit/sqlite.go backend/internal/store/store.go
git commit -m "$(cat <<'EOF'
refactor: consolidate duplicate metadata sanitization into audit package

Both audit/sqlite.go and store/store.go had identical sanitization
logic. Now store imports audit.SanitizeMetadata, eliminating the
duplicate and risk of divergence.
EOF
)"
```

---

## Task 13: Session ID Format Validation in occtl Adapter

**Files:**
- Modify: `backend/internal/ocserv/occtl_adapter.go`
- Modify: `backend/internal/ocserv/occtl_adapter_test.go`

- [ ] **Step 1: Write failing tests for session ID validation**

```go
// Add to backend/internal/ocserv/occtl_adapter_test.go

func TestOcctlAdapter_DisconnectSession_InvalidID(t *testing.T) {
	fake := &FakeCommandRunner{}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", fake)

	badIDs := []string{"", "--socket-file=/evil", "id with spaces", "id\nnewline"}
	for _, id := range badIDs {
		err := adapter.DisconnectSession(context.Background(), id)
		if err == nil {
			t.Errorf("DisconnectSession(%q) = nil, want error", id)
		}
	}
}

func TestOcctlAdapter_GetSessionDetails_InvalidID(t *testing.T) {
	fake := &FakeCommandRunner{}
	adapter := NewOcctlAdapter("/usr/local/bin/occtl", "/var/run/ocserv-socket", fake)

	_, err := adapter.GetSessionDetails(context.Background(), "--malicious")
	if err == nil {
		t.Fatal("GetSessionDetails with flag-like ID should fail")
	}
}
```

- [ ] **Step 2: Implement session ID validation**

In `backend/internal/ocserv/occtl_adapter.go`, add:

```go
import "regexp"

// sessionIDPattern matches valid ocserv session IDs (alphanumeric with hyphens/dots).
var sessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func validateSessionID(id string) error {
	if id == "" {
		return errors.New("session ID must not be empty")
	}
	if len(id) > 128 {
		return errors.New("session ID too long")
	}
	if !sessionIDPattern.MatchString(id) {
		return fmt.Errorf("session ID contains invalid characters: %q", id)
	}
	return nil
}
```

Add `validateSessionID(sessionID)` at the start of `DisconnectSession`, `TerminateSession`, and `GetSessionDetails`.

- [ ] **Step 3: Run tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/ocserv/ -run "TestOcctlAdapter" -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add backend/internal/ocserv/occtl_adapter.go backend/internal/ocserv/occtl_adapter_test.go
git commit -m "$(cat <<'EOF'
fix(security): validate session IDs before passing to occtl

Rejects empty, overly long, or character-set-invalid session IDs.
Prevents potential flag injection via IDs starting with '--'.
EOF
)"
```

---

## Verification

After all tasks are complete:

- [ ] **Run full build:**
  ```bash
  cd /opt/ocserv_panel/backend && go build ./...
  ```

- [ ] **Run full test suite with race detector:**
  ```bash
  cd /opt/ocserv_panel/backend && go test -race ./... -count=1
  ```

- [ ] **Verify no dead code remains:**
  ```bash
  cd /opt/ocserv_panel/backend && grep -rn "SettingDefinition\|GetSettingDefinitions\|GetAll\|knownSettingKeys\|PermissionManageUsers\|PermissionManageConfig\|PermissionManageService\|PermissionSettingsExport\|PermissionSettingsBackup\|NewNetworkInspector[^W]" --include="*.go" . | grep -v "_test.go"
  ```

Expected: No matches (all dead code removed)
