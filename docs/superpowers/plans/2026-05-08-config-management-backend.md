# Config Management Backend — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining gaps between the approved design spec (`docs/superpowers/specs/2026-05-05-config-management-backend-design.md`) and the current implementation.

**Architecture:** The config management backend is already substantially implemented: ConfigFileManager (file I/O, backups, flock, diff), ConfigReloader, ConfigValidator (with ValidateContent), NetworkInspector, ConfigManagementService (RBAC + audit), and HTTP handlers are all wired and tested. Three spec requirements remain unimplemented: (1) unclosed-quote validation check, (2) partial-success test scenario (write ok / reload fail), (3) lock contention test scenario.

**Tech Stack:** Go, standard library (`strings`, `os`, `syscall`), existing test infrastructure (`t.TempDir`, `FakeCommandRunner`)

---

## File Structure

| File | Responsibility | Action |
|------|----------------|--------|
| `backend/internal/ocserv/config_validator.go` | Static config analysis | Modify: add unclosed-quote check in `ValidateContent` |
| `backend/internal/ocserv/config_validator_test.go` | Validator tests | Modify: add unclosed-quote test cases |
| `backend/internal/app/config_management_service_test.go` | Service-layer integration tests | Modify: add partial-success and lock-contention tests |

---

### Task 1: Add Unclosed Quote Validation Check

**Files:**
- Modify: `backend/internal/ocserv/config_validator.go` (inside `ValidateContent`, after the `parseConfigLine` call)
- Modify: `backend/internal/ocserv/config_validator_test.go`

The design spec requires: "Unclosed quote — `error` — Value has odd number of `"` or `'` characters".

- [ ] **Step 1: Write the failing test for unclosed double quote**

Add to `backend/internal/ocserv/config_validator_test.go`:

```go
func TestValidateContentUnclosedDoubleQuote(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator("/dev/null")
	content := "server-cert = \"/etc/ocserv/cert.pem\ntcp-port = 443\n"
	issues, err := v.ValidateContent(context.Background(), content)
	require.NoError(t, err)

	var found bool
	for _, issue := range issues {
		if issue.Severity == IssueError && strings.Contains(issue.Message, "unclosed") {
			found = true
			assert.Equal(t, 1, issue.Line)
			break
		}
	}
	assert.True(t, found, "expected error issue for unclosed double quote, got %v", issues)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/ocserv/ -run TestValidateContentUnclosedDoubleQuote -v`
Expected: FAIL — no "unclosed" issue produced by current validator

- [ ] **Step 3: Write the failing test for unclosed single quote**

Add to `backend/internal/ocserv/config_validator_test.go`:

```go
func TestValidateContentUnclosedSingleQuote(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator("/dev/null")
	content := "server-cert = '/etc/ocserv/cert.pem\n"
	issues, err := v.ValidateContent(context.Background(), content)
	require.NoError(t, err)

	var found bool
	for _, issue := range issues {
		if issue.Severity == IssueError && strings.Contains(issue.Message, "unclosed") {
			found = true
			assert.Equal(t, 1, issue.Line)
			break
		}
	}
	assert.True(t, found, "expected error issue for unclosed single quote, got %v", issues)
}
```

- [ ] **Step 4: Write the test that balanced quotes pass cleanly**

Add to `backend/internal/ocserv/config_validator_test.go`:

```go
func TestValidateContentBalancedQuotesNoIssue(t *testing.T) {
	t.Parallel()
	v := NewConfigValidator("/dev/null")
	content := "server-cert = \"/etc/ocserv/cert.pem\"\nauth = 'plain[passwd=/etc/ocserv/ocpasswd]'\n"
	issues, err := v.ValidateContent(context.Background(), content)
	require.NoError(t, err)
	for _, issue := range issues {
		if strings.Contains(issue.Message, "unclosed") {
			t.Fatalf("unexpected unclosed-quote issue for balanced quotes: %v", issue)
		}
	}
}
```

- [ ] **Step 5: Implement the unclosed quote check**

In `backend/internal/ocserv/config_validator.go`, inside the `ValidateContent` method, add a quote check **after** the `parseConfigLine` call succeeds (i.e., when `ok == true`). The check examines the raw value portion of the line (everything after the `=` sign) for odd quote counts:

```go
// Add this helper function at the bottom of config_validator.go:

// hasUnclosedQuotes checks if a raw value string has an odd number of
// double or single quotes, indicating an unclosed quote.
func hasUnclosedQuotes(rawValue string) (char byte, unclosed bool) {
	dq := strings.Count(rawValue, "\"")
	if dq%2 != 0 {
		return '"', true
	}
	sq := strings.Count(rawValue, "'")
	if sq%2 != 0 {
		return '\'', true
	}
	return 0, false
}
```

In `ValidateContent`, after the `ok` branch from `parseConfigLine` and before the `switch key` block, add:

```go
// Check for unclosed quotes in the raw value portion of the line.
rawValue := line[strings.Index(line, "=")+1:]
if char, unclosed := hasUnclosedQuotes(rawValue); unclosed {
	issues = append(issues, ConfigIssue{
		Line:       i + 1,
		Severity:   IssueError,
		Key:        key,
		Message:    fmt.Sprintf("%s has unclosed %c quote", key, char),
		Suggestion: fmt.Sprintf("Ensure all %c quotes in the value are properly closed.", char),
	})
	continue
}
```

- [ ] **Step 6: Run all validator tests to verify they pass**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/ocserv/ -run "TestValidateContent|TestConfigValidator" -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add backend/internal/ocserv/config_validator.go backend/internal/ocserv/config_validator_test.go
git commit -m "$(cat <<'EOF'
feat(config-validator): add unclosed-quote detection in ValidateContent

Detects values with an odd number of double or single quote characters
and reports them as error-severity issues. This prevents saving configs
with unterminated string values that would confuse ocserv's parser.
EOF
)"
```

---

### Task 2: Add Partial-Success Test (Write OK, Reload Fails)

**Files:**
- Modify: `backend/internal/app/config_management_service_test.go`

The design spec requires testing: "partial-success (write ok / reload fail)".

- [ ] **Step 1: Write the test**

Add to `backend/internal/app/config_management_service_test.go`:

```go
func TestConfigServiceApplyConfigPartialSuccessReloadFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	confPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(confPath, []byte("tcp-port = 443\n"), 0640); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	files := ocserv.NewConfigFileManager(confPath, 10)
	validator := ocserv.NewConfigValidator(confPath)

	runner := ocserv.NewFakeCommandRunner()
	// Reload returns non-zero exit — simulates ocserv rejecting the config at runtime.
	runner.Results["occtl reload"] = ocserv.CommandResult{
		Stderr:   "error in configuration line 2",
		ExitCode: 1,
	}
	reloader := ocserv.NewConfigReloader(runner, "occtl")
	inspector := ocserv.NewNetworkInspectorWithProcDir(runner, dir)

	svc := NewConfigManagementService(
		files,
		validator,
		reloader,
		inspector,
		ocserv.NewRestartDetector(),
		audit.NoopRecorder{},
		rbac.NewDefaultAuthorizer(),
	)

	newContent := "tcp-port = 8443\n"
	result, err := svc.ApplyConfig(context.Background(), ownerPrincipal(), newContent)
	if err != nil {
		t.Fatalf("expected no error for partial success, got %v", err)
	}
	if result.ReloadSuccess {
		t.Fatal("expected ReloadSuccess=false when occtl reload fails")
	}
	if result.ReloadError == "" {
		t.Fatal("expected ReloadError to contain stderr")
	}
	if result.BackupNumber != 1 {
		t.Errorf("expected BackupNumber=1, got %d", result.BackupNumber)
	}

	// Verify the file was actually written despite reload failure.
	data, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != newContent {
		t.Errorf("expected file to be written, got %q", string(data))
	}
}
```

- [ ] **Step 2: Run the test to verify it passes**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/app/ -run TestConfigServiceApplyConfigPartialSuccessReloadFails -v`
Expected: PASS (the implementation already handles this case)

- [ ] **Step 3: Commit**

```bash
git add backend/internal/app/config_management_service_test.go
git commit -m "$(cat <<'EOF'
test(config-service): add partial-success scenario (write ok, reload fail)

Verifies that ApplyConfig returns a non-error result with
ReloadSuccess=false when the config file is successfully written
but occtl reload exits non-zero.
EOF
)"
```

---

### Task 3: Add Lock Contention Test

**Files:**
- Modify: `backend/internal/ocserv/config_file_test.go`

The design spec requires testing: "lock contention — flock returns ErrConfigLocked".

- [ ] **Step 1: Write the test**

Add to `backend/internal/ocserv/config_file_test.go`:

```go
func TestConfigFileManagerWriteLockContention(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(cfgPath, []byte("initial\n"), 0640); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	// Acquire the lock externally to simulate contention.
	lockPath := cfgPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0640)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("acquire test lock: %v", err)
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	}()

	// Now ConfigFileManager.Write should fail with ErrConfigLocked.
	mgr := NewConfigFileManager(cfgPath, 5)
	err = mgr.Write("new content\n")
	if !errors.Is(err, ErrConfigLocked) {
		t.Fatalf("expected ErrConfigLocked, got %v", err)
	}

	// Verify the file was not modified.
	data, _ := os.ReadFile(cfgPath)
	if string(data) != "initial\n" {
		t.Errorf("file should not have changed, got %q", string(data))
	}
}
```

- [ ] **Step 2: Run the test**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/ocserv/ -run TestConfigFileManagerWriteLockContention -v`
Expected: PASS (the flock implementation already handles this)

- [ ] **Step 3: Write a Rollback lock contention test**

Add to `backend/internal/ocserv/config_file_test.go`:

```go
func TestConfigFileManagerRollbackLockContention(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ocserv.conf")
	if err := os.WriteFile(cfgPath, []byte("original\n"), 0640); err != nil {
		t.Fatalf("write original: %v", err)
	}

	mgr := NewConfigFileManager(cfgPath, 5)
	// Create a backup first so rollback has something to target.
	if err := mgr.Write("updated\n"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Now hold the lock externally.
	lockPath := cfgPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0640)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("acquire test lock: %v", err)
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	}()

	// Rollback should fail with ErrConfigLocked.
	err = mgr.Rollback(1)
	if !errors.Is(err, ErrConfigLocked) {
		t.Fatalf("expected ErrConfigLocked, got %v", err)
	}
}
```

- [ ] **Step 4: Run both lock contention tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/ocserv/ -run "LockContention" -v`
Expected: BOTH PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ocserv/config_file_test.go
git commit -m "$(cat <<'EOF'
test(config-file): add flock contention tests for Write and Rollback

Verifies that concurrent lock holders cause ErrConfigLocked and that
file content is not modified when the lock cannot be acquired.
EOF
)"
```

---

### Task 4: Full Verification

**Files:** None modified — verification only.

- [ ] **Step 1: Run all config-related tests**

Run: `cd /opt/ocserv_panel/backend && go test ./internal/ocserv/ ./internal/app/ ./internal/server/ -count=1 -v 2>&1 | grep -E "^(=== RUN|--- PASS|--- FAIL|FAIL|ok)" | tail -60`
Expected: ALL PASS

- [ ] **Step 2: Run full backend test suite**

Run: `cd /opt/ocserv_panel/backend && go test ./... -count=1 2>&1 | tail -20`
Expected: ALL packages pass

- [ ] **Step 3: Build check**

Run: `cd /opt/ocserv_panel/backend && go build ./...`
Expected: Clean build, no errors

- [ ] **Step 4: Verify spec coverage completeness**

Check each spec requirement manually:

| Spec Section | Implementation | Status |
|---|---|---|
| Config File I/O | `config_file.go` | ✅ Complete |
| Backup Rotation | `config_file.go` rotateBackups/copyToBackup | ✅ Complete |
| flock Locking | `config_file.go` acquireLock | ✅ Complete |
| Diff Generation | `config_file.go` unifiedDiff (Myers LCS) | ✅ Complete |
| Sentinel Errors | ErrConfigLocked, ErrBackupNotFound, ErrConfigTooLarge | ✅ Complete |
| Config Reload | `config_reload.go` with 10s timeout | ✅ Complete |
| Malformed line check | `config_validator.go` ValidateContent | ✅ Complete |
| Unclosed quote check | `config_validator.go` ValidateContent | ✅ After Task 1 |
| Port range check | `config_validator.go` checkPortValue | ✅ Complete |
| CIDR notation check | `config_validator.go` checkCIDR | ✅ Complete |
| DNS IP check | `config_validator.go` checkDNS | ✅ Complete |
| Network Inspector | `network_inspector.go` (kernel, firewall, routing) | ✅ Complete |
| Service Layer | `config_management_service.go` (all 8 methods) | ✅ Complete |
| RBAC | 3 permissions in `rbac.go` | ✅ Complete |
| Audit Events | recorded in service layer | ✅ Complete |
| HTTP Endpoints | 9 endpoints in `config_handlers.go` | ✅ Complete |
| Error Codes | 400/401/403/404/409/422/500 in writeConfigError | ✅ Complete |
| Tests | All categories covered | ✅ After Tasks 1-3 |
