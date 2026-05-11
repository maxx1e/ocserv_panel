# Config Tab Improvements — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform the Config tab from a raw textarea editor into a safer configuration management workflow with clickable validation issues, CIDR auto-fix, CodeMirror 6 editor, restart detection, and extended network checks.

**Architecture:** Backend enhancements add line numbers, auto-fix values, and restart detection to the existing validator/service layer. Frontend replaces the textarea with a CodeMirror 6 component that supports line-aware interactions (jump-to-line, inline error markers, one-click CIDR fixes). The existing 9-endpoint API contract is extended with new fields (backward-compatible).

**Tech Stack:** Go backend (existing patterns), SvelteKit/Svelte 5 frontend, CodeMirror 6 (`codemirror`, `@codemirror/view`, `@codemirror/state`, `@codemirror/language`, `@codemirror/commands`, `@codemirror/search`, `@lezer/highlight`), Tailwind CSS v4.

---

## Context

The Config tab currently works end-to-end: read config, edit in textarea, validate, preview diff, apply with backup rotation, rollback from backups, and view network prerequisites. However, the UX is limited:

- Validation issues don't reference line numbers — the user must manually search for the problem.
- No inline highlighting of invalid lines in the editor.
- CIDR values using netmask notation (e.g., `10.10.10.0/255.255.255.0`) are flagged as invalid but no auto-fix is offered.
- Apply doesn't distinguish between reload-safe vs restart-required changes.
- Network checks don't verify firewall rules for the listener port or VPN subnet overlap.
- The textarea lacks line numbers, making it hard to correlate issues with code.

This plan addresses all six deficiencies incrementally, each task compiling independently.

---

## File Structure

### Backend — Modify

| File | Responsibility |
|------|---------------|
| `backend/internal/ocserv/config_validator.go` | Add `Line`, `FixedValue` fields; CIDR netmask detection |
| `backend/internal/ocserv/config_validator_test.go` | Tests for line numbers and auto-fix |
| `backend/internal/ocserv/network_inspector.go` | Extend with port/subnet/DNS checks |
| `backend/internal/ocserv/network_inspector_test.go` | Tests for new checks |
| `backend/internal/app/config_management_service.go` | Wire restart detector; propagate restart info |
| `backend/internal/app/config_management_service_test.go` | Test restart detection in service |
| `backend/internal/server/config_handlers.go` | Add `line`, `fixedValue`, `requiresRestart`, `restartReasons` to JSON |
| `backend/internal/server/config_handlers_test.go` | Integration tests for new fields |
| `backend/cmd/ocserv-panel/main.go` | Wire `RestartDetector` and updated `NetworkInspector` |

### Backend — Create

| File | Responsibility |
|------|---------------|
| `backend/internal/ocserv/cidr_convert.go` | Netmask-to-CIDR prefix conversion |
| `backend/internal/ocserv/cidr_convert_test.go` | Table-driven conversion tests |
| `backend/internal/ocserv/config_restart_detector.go` | Directive diff → restart requirement analysis |
| `backend/internal/ocserv/config_restart_detector_test.go` | Restart detection tests |

### Frontend — Modify

| File | Responsibility |
|------|---------------|
| `frontend/package.json` | Add CodeMirror 6 dependencies |
| `frontend/src/lib/api.ts` | Add `line`, `fixedValue`, `requiresRestart`, `restartReasons` to types |
| `frontend/src/routes/config/+page.svelte` | Replace textarea, wire interactions, apply flow |

### Frontend — Create

| File | Responsibility |
|------|---------------|
| `frontend/src/lib/components/ui/ConfigEditor.svelte` | CodeMirror 6 Svelte 5 wrapper |
| `frontend/src/lib/components/ui/config-editor-extensions.ts` | INI syntax, gutter markers, line decorations, theme |
| `frontend/src/lib/components/ui/ConfigIssueList.svelte` | Issue list with jump/fix actions |

---

## Phase A: Backend Validator Enhancements

### Task A1: Add `Line` and `FixedValue` fields to ConfigIssue

**Files:**
- Modify: `backend/internal/ocserv/config_validator.go`
- Modify: `backend/internal/ocserv/config_validator_test.go`

- [ ] **Step 1: Write failing test — line number present in ValidateContent result**

```go
func TestValidateContentReportsLineNumbers(t *testing.T) {
    v := NewConfigValidator("/dev/null")
    content := "# comment\ntcp-port = 99999\nroute = invalid\n"
    issues, err := v.ValidateContent(context.Background(), content)
    require.NoError(t, err)
    require.Len(t, issues, 2)
    assert.Equal(t, 2, issues[0].Line) // tcp-port on line 2
    assert.Equal(t, 3, issues[1].Line) // route on line 3
}
```

- [ ] **Step 2: Run test to confirm failure**

Run: `cd backend && go test ./internal/ocserv/ -run TestValidateContentReportsLineNumbers -v`
Expected: FAIL — `ConfigIssue` has no `Line` field.

- [ ] **Step 3: Add Line and FixedValue fields to ConfigIssue struct**

```go
type ConfigIssue struct {
    Line       int                 `json:"line,omitempty"`
    Severity   ConfigIssueSeverity `json:"severity"`
    Key        string              `json:"key"`
    Message    string              `json:"message"`
    Suggestion string              `json:"suggestion,omitempty"`
    FixedValue string              `json:"fixedValue,omitempty"`
}
```

- [ ] **Step 4: Propagate line numbers in ValidateContent**

In the `ValidateContent` loop, set `Line: i+1` on every ConfigIssue created within the loop body (malformed line error, port check results, CIDR check results, DNS check results). Modify the helper methods to accept a `lineNum int` parameter:

```go
func (v *ConfigValidator) checkPortValue(key, value string, lineNum int) []ConfigIssue {
    // ...existing logic...
    return []ConfigIssue{{
        Line:       lineNum,
        Severity:   IssueError,
        // ...rest unchanged
    }}
}
```

Apply the same pattern to `checkCIDR` and `checkDNS`.

- [ ] **Step 5: Run test to confirm it passes**

Run: `cd backend && go test ./internal/ocserv/ -run TestValidateContentReportsLineNumbers -v`
Expected: PASS

- [ ] **Step 6: Run full validator test suite**

Run: `cd backend && go test ./internal/ocserv/ -run TestConfig -v`
Expected: All pass (existing tests don't assert Line, so they remain valid).

- [ ] **Step 7: Commit**

```bash
git add backend/internal/ocserv/config_validator.go backend/internal/ocserv/config_validator_test.go
git commit -m "feat(config): add Line and FixedValue fields to ConfigIssue, propagate line numbers in content validation"
```

---

### Task A2: Add line numbers to file-based ValidateConfig

**Files:**
- Modify: `backend/internal/ocserv/config_validator.go`
- Modify: `backend/internal/ocserv/config_validator_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestValidateConfigFileReportsLineNumbers(t *testing.T) {
    dir := t.TempDir()
    cfgPath := filepath.Join(dir, "ocserv.conf")
    content := "server-cert = ../certs/server-cert.pem\ndevice = vpns\n"
    require.NoError(t, os.WriteFile(cfgPath, []byte(content), 0644))

    v := NewConfigValidator(cfgPath)
    issues, err := v.ValidateConfig(context.Background())
    require.NoError(t, err)
    // server-cert relative path warning should report line 1
    var certIssue *ConfigIssue
    for i := range issues {
        if issues[i].Key == "server-cert" {
            certIssue = &issues[i]
            break
        }
    }
    require.NotNil(t, certIssue)
    assert.Equal(t, 1, certIssue.Line)
}
```

- [ ] **Step 2: Run test to confirm failure**

Run: `cd backend && go test ./internal/ocserv/ -run TestValidateConfigFileReportsLineNumbers -v`
Expected: FAIL — Line is 0.

- [ ] **Step 3: Refactor ValidateConfig to track line index**

Change the loop from `for _, rawLine := range strings.Split(...)` to `for i, rawLine := range strings.Split(...)`. After each `checkCertPath`, `checkAuthDirective`, `checkSocketPath`, `checkDevice` call, set `Line: i+1` on the returned issues:

```go
for i, rawLine := range strings.Split(string(data), "\n") {
    // ...existing logic...
    switch key {
    case "server-cert", "server-key", "ca-cert":
        for _, issue := range v.checkCertPath(key, value, configDir) {
            issue.Line = i + 1
            issues = append(issues, issue)
        }
    // ...similar for other cases
    }
}
```

- [ ] **Step 4: Run tests**

Run: `cd backend && go test ./internal/ocserv/ -run TestConfigValidator -v`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ocserv/config_validator.go backend/internal/ocserv/config_validator_test.go
git commit -m "feat(config): propagate line numbers in file-based config validation"
```

---

### Task A3: Create CIDR netmask-to-prefix conversion helper

**Files:**
- Create: `backend/internal/ocserv/cidr_convert.go`
- Create: `backend/internal/ocserv/cidr_convert_test.go`

- [ ] **Step 1: Write failing tests**

```go
package ocserv

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestIsNetmaskNotation(t *testing.T) {
    tests := []struct {
        input    string
        wantIP   string
        wantMask string
        wantOK   bool
    }{
        {"10.10.10.0/255.255.255.0", "10.10.10.0", "255.255.255.0", true},
        {"192.168.0.0/255.255.0.0", "192.168.0.0", "255.255.0.0", true},
        {"10.0.0.0/24", "", "", false},                    // prefix notation
        {"10.0.0.0/8", "", "", false},                     // prefix notation
        {"invalid", "", "", false},                         // no slash
        {"10.0.0.0/abc", "", "", false},                   // not a dotted quad
    }
    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            ip, mask, ok := isNetmaskNotation(tt.input)
            assert.Equal(t, tt.wantOK, ok)
            if ok {
                assert.Equal(t, tt.wantIP, ip)
                assert.Equal(t, tt.wantMask, mask)
            }
        })
    }
}

func TestNetmaskToCIDR(t *testing.T) {
    tests := []struct {
        ip      string
        mask    string
        want    string
        wantOK  bool
    }{
        {"10.10.10.0", "255.255.255.0", "10.10.10.0/24", true},
        {"192.168.0.0", "255.255.0.0", "192.168.0.0/16", true},
        {"10.0.0.0", "255.0.0.0", "10.0.0.0/8", true},
        {"10.0.0.0", "255.255.128.0", "10.0.0.0/17", true},
        {"172.16.0.0", "255.240.0.0", "172.16.0.0/12", true},
        {"10.0.0.0", "255.255.100.0", "", false},          // non-contiguous
        {"10.0.0.0", "0.255.255.0", "", false},            // non-contiguous
    }
    for _, tt := range tests {
        t.Run(tt.ip+"/"+tt.mask, func(t *testing.T) {
            result, ok := netmaskToCIDR(tt.ip, tt.mask)
            assert.Equal(t, tt.wantOK, ok)
            if ok {
                assert.Equal(t, tt.want, result)
            }
        })
    }
}
```

- [ ] **Step 2: Run tests to confirm failure**

Run: `cd backend && go test ./internal/ocserv/ -run "TestIsNetmaskNotation|TestNetmaskToCIDR" -v`
Expected: FAIL — functions undefined.

- [ ] **Step 3: Implement cidr_convert.go**

```go
package ocserv

import (
    "net"
    "strings"
)

// isNetmaskNotation checks if a value is in "ip/dotted-quad-mask" format.
// Returns the ip and mask parts if detected.
func isNetmaskNotation(value string) (ip, mask string, ok bool) {
    parts := strings.SplitN(value, "/", 2)
    if len(parts) != 2 {
        return "", "", false
    }
    ip = parts[0]
    mask = parts[1]
    // Must have exactly 3 dots to be a dotted-quad mask (not a prefix integer)
    if strings.Count(mask, ".") != 3 {
        return "", "", false
    }
    // Both parts must parse as valid IPs
    if net.ParseIP(ip) == nil || net.ParseIP(mask) == nil {
        return "", "", false
    }
    return ip, mask, true
}

// netmaskToCIDR converts "ip" + "dotted-quad-mask" to CIDR notation.
// Returns the CIDR string and true if the mask is a valid contiguous mask.
// Returns "", false if the mask is non-contiguous.
func netmaskToCIDR(ip, mask string) (string, bool) {
    maskIP := net.ParseIP(mask)
    if maskIP == nil {
        return "", false
    }
    // Convert to 4-byte form
    maskBytes := maskIP.To4()
    if maskBytes == nil {
        return "", false
    }
    ipMask := net.IPMask(maskBytes)
    ones, bits := ipMask.Size()
    if bits == 0 {
        // Size() returns (0,0) for non-contiguous masks
        return "", false
    }
    return ip + "/" + strconv.Itoa(ones), true
}
```

Note: Need to add `"strconv"` to imports.

- [ ] **Step 4: Run tests**

Run: `cd backend && go test ./internal/ocserv/ -run "TestIsNetmaskNotation|TestNetmaskToCIDR" -v`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ocserv/cidr_convert.go backend/internal/ocserv/cidr_convert_test.go
git commit -m "feat(config): add netmask-to-CIDR prefix conversion helper"
```

---

### Task A4: Integrate CIDR auto-fix into validator

**Files:**
- Modify: `backend/internal/ocserv/config_validator.go`
- Modify: `backend/internal/ocserv/config_validator_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestValidateContentCIDRAutoFix(t *testing.T) {
    v := NewConfigValidator("/dev/null")
    content := "route = 10.10.10.0/255.255.255.0\n"
    issues, err := v.ValidateContent(context.Background(), content)
    require.NoError(t, err)
    require.Len(t, issues, 1)
    assert.Equal(t, IssueWarning, issues[0].Severity)
    assert.Equal(t, "route = 10.10.10.0/24", issues[0].FixedValue)
    assert.Equal(t, 1, issues[0].Line)
}

func TestValidateContentNonContiguousMaskNoFix(t *testing.T) {
    v := NewConfigValidator("/dev/null")
    content := "route = 10.0.0.0/255.255.100.0\n"
    issues, err := v.ValidateContent(context.Background(), content)
    require.NoError(t, err)
    require.Len(t, issues, 1)
    assert.Equal(t, IssueError, issues[0].Severity)
    assert.Empty(t, issues[0].FixedValue) // no fix for non-contiguous
}
```

- [ ] **Step 2: Run test to confirm failure**

Run: `cd backend && go test ./internal/ocserv/ -run "TestValidateContentCIDR" -v`
Expected: FAIL.

- [ ] **Step 3: Modify checkCIDR to detect netmask notation**

Update `checkCIDR` signature to accept the raw line and line number:

```go
func (v *ConfigValidator) checkCIDR(key, value string, lineNum int, rawLine string) []ConfigIssue {
    // Check for netmask notation first
    if ip, mask, ok := isNetmaskNotation(value); ok {
        cidr, valid := netmaskToCIDR(ip, mask)
        if valid {
            return []ConfigIssue{{
                Line:       lineNum,
                Severity:   IssueWarning,
                Key:        key,
                Message:    fmt.Sprintf("%s uses netmask notation: %s", key, value),
                Suggestion: fmt.Sprintf("Use CIDR notation: %s", cidr),
                FixedValue: fmt.Sprintf("%s = %s", key, cidr),
            }}
        }
        return []ConfigIssue{{
            Line:     lineNum,
            Severity: IssueError,
            Key:      key,
            Message:  fmt.Sprintf("%s has non-contiguous subnet mask: %s", key, value),
            Suggestion: "Use a valid contiguous mask or CIDR prefix notation.",
        }}
    }

    // Existing net.ParseCIDR check
    _, _, err := net.ParseCIDR(value)
    if err != nil {
        return []ConfigIssue{{
            Line:       lineNum,
            Severity:   IssueError,
            Key:        key,
            Message:    fmt.Sprintf("%s has invalid CIDR value: %s", key, value),
            Suggestion: "Use valid CIDR notation, for example: 10.0.0.0/8",
        }}
    }
    return nil
}
```

Update the call site in `ValidateContent` to pass `i+1` and `rawLine`.

- [ ] **Step 4: Run tests**

Run: `cd backend && go test ./internal/ocserv/ -run TestConfig -v`
Expected: All pass including new tests.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ocserv/config_validator.go backend/internal/ocserv/config_validator_test.go
git commit -m "feat(config): detect netmask notation in route/no-route and provide auto-fix value"
```

---

### Task A5: Create restart detection module

**Files:**
- Create: `backend/internal/ocserv/config_restart_detector.go`
- Create: `backend/internal/ocserv/config_restart_detector_test.go`

- [ ] **Step 1: Write failing tests**

```go
package ocserv

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestRestartDetector(t *testing.T) {
    det := NewRestartDetector()

    tests := []struct {
        name           string
        oldContent     string
        newContent     string
        wantRestart    bool
        wantReasonContains string
    }{
        {
            name:        "port change requires restart",
            oldContent:  "tcp-port = 443\nudp-port = 443\n",
            newContent:  "tcp-port = 8443\nudp-port = 443\n",
            wantRestart: true,
            wantReasonContains: "tcp-port",
        },
        {
            name:        "dns change does not require restart",
            oldContent:  "dns = 8.8.8.8\n",
            newContent:  "dns = 1.1.1.1\n",
            wantRestart: false,
        },
        {
            name:        "cert path change requires restart",
            oldContent:  "server-cert = /etc/ocserv/cert.pem\n",
            newContent:  "server-cert = /etc/ocserv/new-cert.pem\n",
            wantRestart: true,
            wantReasonContains: "server-cert",
        },
        {
            name:        "no changes",
            oldContent:  "tcp-port = 443\n",
            newContent:  "tcp-port = 443\n",
            wantRestart: false,
        },
        {
            name:        "device change requires restart",
            oldContent:  "device = vpns\n",
            newContent:  "device = tun0\n",
            wantRestart: true,
            wantReasonContains: "device",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := det.Detect(tt.oldContent, tt.newContent)
            assert.Equal(t, tt.wantRestart, result.RequiresRestart)
            if tt.wantReasonContains != "" {
                require.NotEmpty(t, result.Reasons)
                found := false
                for _, r := range result.Reasons {
                    if strings.Contains(r, tt.wantReasonContains) {
                        found = true
                        break
                    }
                }
                assert.True(t, found, "expected reason containing %q, got %v", tt.wantReasonContains, result.Reasons)
            }
        })
    }
}
```

- [ ] **Step 2: Run tests to confirm failure**

Run: `cd backend && go test ./internal/ocserv/ -run TestRestartDetector -v`
Expected: FAIL — types undefined.

- [ ] **Step 3: Implement config_restart_detector.go**

```go
package ocserv

import "fmt"

// restartRequiredDirectives are ocserv directives that require a full service
// restart (not just reload) when their value changes.
var restartRequiredDirectives = map[string]bool{
    "tcp-port":    true,
    "udp-port":    true,
    "server-cert": true,
    "server-key":  true,
    "listen-host": true,
    "device":      true,
}

// RestartAnalysis contains the result of comparing two config versions for
// restart-requiring changes.
type RestartAnalysis struct {
    RequiresRestart bool     `json:"requiresRestart"`
    Reasons         []string `json:"reasons,omitempty"`
}

// RestartDetector compares two config versions and determines whether a full
// service restart is needed (vs a simple reload).
type RestartDetector struct{}

// NewRestartDetector creates a new RestartDetector.
func NewRestartDetector() *RestartDetector {
    return &RestartDetector{}
}

// Detect compares oldContent and newContent, returning an analysis of whether
// any restart-required directives changed.
func (d *RestartDetector) Detect(oldContent, newContent string) RestartAnalysis {
    oldValues := parseDirectiveValues(oldContent, restartRequiredDirectives)
    newValues := parseDirectiveValues(newContent, restartRequiredDirectives)

    var reasons []string
    for key := range restartRequiredDirectives {
        oldVal := oldValues[key]
        newVal := newValues[key]
        if oldVal != newVal {
            if oldVal == "" {
                reasons = append(reasons, fmt.Sprintf("%s added: %s", key, newVal))
            } else if newVal == "" {
                reasons = append(reasons, fmt.Sprintf("%s removed (was: %s)", key, oldVal))
            } else {
                reasons = append(reasons, fmt.Sprintf("%s changed: %s -> %s", key, oldVal, newVal))
            }
        }
    }

    return RestartAnalysis{
        RequiresRestart: len(reasons) > 0,
        Reasons:         reasons,
    }
}

// parseDirectiveValues extracts the last value for each directive in the filter set.
func parseDirectiveValues(content string, filter map[string]bool) map[string]string {
    values := make(map[string]string)
    for _, line := range strings.Split(content, "\n") {
        key, value, ok := parseConfigLine(strings.TrimSpace(line))
        if !ok {
            continue
        }
        if filter[key] {
            values[key] = value
        }
    }
    return values
}
```

Note: Need `"strings"` import.

- [ ] **Step 4: Run tests**

Run: `cd backend && go test ./internal/ocserv/ -run TestRestartDetector -v`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ocserv/config_restart_detector.go backend/internal/ocserv/config_restart_detector_test.go
git commit -m "feat(config): add restart detection for directives requiring service restart"
```

---

### Task A6: Wire restart detection into ConfigManagementService

**Files:**
- Modify: `backend/internal/app/config_management_service.go`
- Modify: `backend/internal/app/config_management_service_test.go`
- Modify: `backend/cmd/ocserv-panel/main.go`

- [ ] **Step 1: Write failing test**

```go
func TestValidateContent_ReportsRestartRequired(t *testing.T) {
    // Setup: ConfigFileManager with known current content containing tcp-port = 443
    dir := t.TempDir()
    cfgPath := filepath.Join(dir, "ocserv.conf")
    require.NoError(t, os.WriteFile(cfgPath, []byte("tcp-port = 443\n"), 0644))

    files := ocserv.NewConfigFileManager(cfgPath, 5)
    validator := ocserv.NewConfigValidator(cfgPath)
    reloader := ocserv.NewConfigReloader(&ocserv.FakeCommandRunner{}, "/usr/bin/occtl")
    networkInsp := ocserv.NewNetworkInspector(&ocserv.FakeCommandRunner{})
    restartDet := ocserv.NewRestartDetector()
    svc := app.NewConfigManagementService(files, validator, reloader, networkInsp, restartDet, noopRecorder, rbac.NewDefaultAuthorizer())

    principal := auth.Principal{ID: "u1", Role: "owner"}
    result, err := svc.ValidateContent(ctx, principal, "tcp-port = 8443\n")
    require.NoError(t, err)
    assert.True(t, result.RequiresRestart)
    assert.NotEmpty(t, result.RestartReasons)
}
```

- [ ] **Step 2: Run test to confirm failure**

Expected: FAIL — `NewConfigManagementService` doesn't accept `RestartDetector`, `ValidationResult` doesn't have restart fields.

- [ ] **Step 3: Extend ValidationResult and service**

Add to `ValidationResult`:
```go
type ValidationResult struct {
    Valid           bool
    Issues          []ocserv.ConfigIssue
    RequiresRestart bool
    RestartReasons  []string
}
```

Add `restartDetector *ocserv.RestartDetector` to `ConfigManagementService` struct. Update constructor to accept it. In `ValidateContent`, after validation:
```go
// Read current config for restart comparison
currentContent, _, readErr := s.files.Read()
if readErr == nil {
    analysis := s.restartDetector.Detect(currentContent, content)
    result.RequiresRestart = analysis.RequiresRestart
    result.RestartReasons = analysis.Reasons
}
```

Update `main.go` to pass `ocserv.NewRestartDetector()` to the service constructor.

- [ ] **Step 4: Run tests**

Run: `cd backend && go test ./internal/app/ -v && go test ./internal/ocserv/ -v`
Expected: All pass.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/app/config_management_service.go backend/internal/app/config_management_service_test.go backend/cmd/ocserv-panel/main.go
git commit -m "feat(config): wire restart detection into validate and expose in ValidationResult"
```

---

### Task A7: Update HTTP handler JSON types for new fields

**Files:**
- Modify: `backend/internal/server/config_handlers.go`
- Modify: `backend/internal/server/config_handlers_test.go`

- [ ] **Step 1: Write failing handler test**

```go
func TestHandleConfigValidate_IncludesLineAndRestartFields(t *testing.T) {
    // Setup app router with test config containing tcp-port = 443
    // POST /api/config/validate with body changing port
    // Assert JSON response includes "line" field in issues and "requiresRestart" at top level
}
```

- [ ] **Step 2: Update JSON response types**

Add to `configIssueJSON`:
```go
type configIssueJSON struct {
    Line       int    `json:"line,omitempty"`
    Severity   string `json:"severity"`
    Key        string `json:"key"`
    Message    string `json:"message"`
    Suggestion string `json:"suggestion,omitempty"`
    FixedValue string `json:"fixedValue,omitempty"`
}
```

Add to `configValidateResponse`:
```go
type configValidateResponse struct {
    Valid           bool              `json:"valid"`
    Issues          []configIssueJSON `json:"issues"`
    RequiresRestart bool             `json:"requiresRestart,omitempty"`
    RestartReasons  []string         `json:"restartReasons,omitempty"`
}
```

Update `toConfigIssuesJSON` helper to copy `Line` and `FixedValue`.

Update `handleConfigValidate` to populate `RequiresRestart` and `RestartReasons` from `ValidationResult`.

- [ ] **Step 3: Run tests**

Run: `cd backend && go test ./internal/server/ -v`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/server/config_handlers.go backend/internal/server/config_handlers_test.go
git commit -m "feat(config): expose line, fixedValue, requiresRestart in config API JSON responses"
```

---

## Phase B: Frontend CodeMirror 6 Integration

### Task B1: Install CodeMirror 6 packages

**Files:**
- Modify: `frontend/package.json`

- [ ] **Step 1: Install packages**

```bash
cd frontend && npm install codemirror @codemirror/view @codemirror/state @codemirror/language @codemirror/commands @codemirror/search @lezer/highlight
```

- [ ] **Step 2: Verify build**

Run: `cd frontend && npm run check`
Expected: No type errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/package.json frontend/package-lock.json
git commit -m "chore(frontend): install CodeMirror 6 packages"
```

---

### Task B2: Create CodeMirror extensions (syntax, gutter, decorations, theme)

**Files:**
- Create: `frontend/src/lib/components/ui/config-editor-extensions.ts`

- [ ] **Step 1: Create the extension file**

```typescript
import { EditorView, Decoration, DecorationSet, gutter, GutterMarker } from '@codemirror/view';
import { StateField, StateEffect, RangeSet, Compartment } from '@codemirror/state';
import { StreamLanguage } from '@codemirror/language';
import { tags } from '@lezer/highlight';
import type { ConfigIssue } from '$lib/api';

// ─── INI-like syntax for ocserv config ────────────────────────

export const ocservConfigLanguage = StreamLanguage.define({
  token(stream) {
    if (stream.sol() && stream.match(/^\s*#/)) {
      stream.skipToEnd();
      return 'comment';
    }
    if (stream.sol() && stream.match(/^[a-zA-Z0-9_-]+/)) {
      return 'keyword';
    }
    if (stream.match(/^\s*=\s*/)) {
      return 'operator';
    }
    stream.next();
    return 'string';
  },
});

// ─── Issue line highlights ────────────────────────────────────

export const setIssuesEffect = StateEffect.define<ConfigIssue[]>();

const issueDecorations = StateField.define<DecorationSet>({
  create() { return Decoration.none; },
  update(deco, tr) {
    for (const effect of tr.effects) {
      if (effect.is(setIssuesEffect)) {
        const issues = effect.value;
        const decorations: Range<Decoration>[] = [];
        for (const issue of issues) {
          if (!issue.line || issue.line < 1) continue;
          const lineInfo = tr.state.doc.line(Math.min(issue.line, tr.state.doc.lines));
          const cls = issue.severity === 'error' ? 'cm-line-error' :
                      issue.severity === 'warning' ? 'cm-line-warning' : 'cm-line-info';
          decorations.push(Decoration.line({ class: cls }).range(lineInfo.from));
        }
        return RangeSet.of(decorations, true);
      }
    }
    return deco;
  },
  provide: f => EditorView.decorations.from(f),
});

// ─── Severity gutter markers ─────────────────────────────────

class IssueMark extends GutterMarker {
  constructor(readonly severity: string) { super(); }
  toDOM() {
    const span = document.createElement('span');
    span.className = this.severity === 'error' ? 'cm-gutter-error' :
                     this.severity === 'warning' ? 'cm-gutter-warning' : 'cm-gutter-info';
    span.textContent = this.severity === 'error' ? '●' :
                       this.severity === 'warning' ? '▲' : 'ℹ';
    return span;
  }
}

export const setGutterIssuesEffect = StateEffect.define<ConfigIssue[]>();

const issueGutterState = StateField.define<Map<number, string>>({
  create() { return new Map(); },
  update(map, tr) {
    for (const effect of tr.effects) {
      if (effect.is(setGutterIssuesEffect)) {
        const newMap = new Map<number, string>();
        for (const issue of effect.value) {
          if (issue.line && issue.line >= 1) {
            const existing = newMap.get(issue.line);
            // error takes priority over warning over info
            if (!existing || issue.severity === 'error' || (issue.severity === 'warning' && existing === 'info')) {
              newMap.set(issue.line, issue.severity);
            }
          }
        }
        return newMap;
      }
    }
    return map;
  },
});

export const issueGutter = gutter({
  class: 'cm-issue-gutter',
  markers(view) {
    const map = view.state.field(issueGutterState);
    const markers: Range<GutterMarker>[] = [];
    for (const [lineNum, severity] of map) {
      if (lineNum <= view.state.doc.lines) {
        const line = view.state.doc.line(lineNum);
        markers.push(new IssueMark(severity).range(line.from));
      }
    }
    return RangeSet.of(markers, true);
  },
});

// ─── Read-only compartment ───────────────────────────────────

export const readOnlyCompartment = new Compartment();

// ─── Theme ───────────────────────────────────────────────────

export const ocservTheme = EditorView.theme({
  '&': { fontSize: '13px', fontFamily: 'ui-monospace, monospace' },
  '.cm-content': { padding: '12px 0' },
  '.cm-gutters': { backgroundColor: '#f8fafc', borderRight: '1px solid #e2e8f0', color: '#64748b' },
  '.cm-activeLineGutter': { backgroundColor: '#f1f5f9' },
  '.cm-activeLine': { backgroundColor: '#f8fafc' },
  '.cm-selectionBackground': { backgroundColor: '#e2e8f0 !important' },
  '.cm-line-error': { backgroundColor: '#fef2f2' },
  '.cm-line-warning': { backgroundColor: '#fffbeb' },
  '.cm-line-info': { backgroundColor: '#eff6ff' },
  '.cm-issue-gutter': { width: '16px', paddingLeft: '2px' },
  '.cm-gutter-error': { color: '#ef4444', fontSize: '10px' },
  '.cm-gutter-warning': { color: '#f59e0b', fontSize: '10px' },
  '.cm-gutter-info': { color: '#3b82f6', fontSize: '10px' },
});

// ─── All extensions bundled ──────────────────────────────────

export function createExtensions(readonly: boolean) {
  return [
    ocservConfigLanguage,
    issueDecorations,
    issueGutterState,
    issueGutter,
    ocservTheme,
    readOnlyCompartment.of(EditorView.editable.of(!readonly)),
  ];
}
```

- [ ] **Step 2: Verify types**

Run: `cd frontend && npm run check`
Expected: No type errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/ui/config-editor-extensions.ts
git commit -m "feat(frontend): create CodeMirror 6 extensions for ocserv config syntax and issue annotations"
```

---

### Task B3: Create ConfigEditor Svelte 5 wrapper component

**Files:**
- Create: `frontend/src/lib/components/ui/ConfigEditor.svelte`

- [ ] **Step 1: Create the component**

```svelte
<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter } from '@codemirror/view';
  import { EditorState } from '@codemirror/state';
  import { defaultKeymap, history, historyKeymap } from '@codemirror/commands';
  import { searchKeymap, highlightSelectionMatches } from '@codemirror/search';
  import {
    createExtensions, readOnlyCompartment, setIssuesEffect, setGutterIssuesEffect
  } from './config-editor-extensions';
  import type { ConfigIssue } from '$lib/api';

  let {
    content = $bindable(''),
    readonly = false,
    issues = [] as ConfigIssue[],
    class: className = '',
  }: {
    content: string;
    readonly?: boolean;
    issues?: ConfigIssue[];
    class?: string;
  } = $props();

  let container: HTMLDivElement;
  let view: EditorView | null = $state(null);
  let internalUpdate = false;

  onMount(() => {
    const state = EditorState.create({
      doc: content,
      extensions: [
        lineNumbers(),
        highlightActiveLine(),
        highlightActiveLineGutter(),
        history(),
        highlightSelectionMatches(),
        keymap.of([...defaultKeymap, ...historyKeymap, ...searchKeymap]),
        ...createExtensions(readonly),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            internalUpdate = true;
            content = update.state.doc.toString();
            internalUpdate = false;
          }
        }),
      ],
    });

    view = new EditorView({ state, parent: container });
  });

  onDestroy(() => {
    view?.destroy();
    view = null;
  });

  // Sync external content changes into the editor
  $effect(() => {
    if (!view || internalUpdate) return;
    const currentDoc = view.state.doc.toString();
    if (content !== currentDoc) {
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: content },
      });
    }
  });

  // Sync readonly state
  $effect(() => {
    if (!view) return;
    view.dispatch({
      effects: readOnlyCompartment.reconfigure(EditorView.editable.of(!readonly)),
    });
  });

  // Sync issues into decorations and gutter
  $effect(() => {
    if (!view) return;
    view.dispatch({
      effects: [
        setIssuesEffect.of(issues),
        setGutterIssuesEffect.of(issues),
      ],
    });
  });

  // Public method: scroll to a specific line and highlight it
  export function scrollToLine(line: number) {
    if (!view) return;
    const lineCount = view.state.doc.lines;
    const targetLine = Math.max(1, Math.min(line, lineCount));
    const lineInfo = view.state.doc.line(targetLine);
    view.dispatch({
      selection: { anchor: lineInfo.from },
      effects: EditorView.scrollIntoView(lineInfo.from, { y: 'center' }),
    });
    view.focus();
  }
</script>

<div bind:this={container} class="border border-slate-200 rounded-lg overflow-hidden {className}"></div>
```

- [ ] **Step 2: Verify types**

Run: `cd frontend && npm run check`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/ui/ConfigEditor.svelte
git commit -m "feat(frontend): add ConfigEditor Svelte 5 wrapper for CodeMirror 6"
```

---

### Task B4: Replace textarea with ConfigEditor in config page

**Files:**
- Modify: `frontend/src/routes/config/+page.svelte`

- [ ] **Step 1: Replace the textarea block**

Import the new component:
```typescript
import ConfigEditor from '$lib/components/ui/ConfigEditor.svelte';
```

Add a ref variable:
```typescript
let editorRef: ConfigEditor;
```

Replace the textarea section (the `<div class="bg-white rounded-lg shadow-sm border border-slate-200">` containing the `<textarea>`) with:
```svelte
<ConfigEditor
  bind:this={editorRef}
  bind:content={editContent}
  readonly={!isOwner}
  {issues}
  class="min-h-[400px]"
/>
```

Remove the `<textarea>` and its wrapping div.

- [ ] **Step 2: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/config/+page.svelte
git commit -m "feat(frontend): replace textarea with CodeMirror 6 editor in config page"
```

---

## Phase C: Frontend Issue Interaction

### Task C1: Update frontend API types

**Files:**
- Modify: `frontend/src/lib/api.ts`

- [ ] **Step 1: Update ConfigIssue type**

```typescript
export type ConfigIssue = {
  severity: 'error' | 'warning' | 'info';
  key: string;
  message: string;
  suggestion?: string;
  line?: number;
  fixedValue?: string;
};
```

- [ ] **Step 2: Update ConfigValidateResult type**

```typescript
export type ConfigValidateResult = {
  valid: boolean;
  issues: ConfigIssue[];
  requiresRestart?: boolean;
  restartReasons?: string[];
};
```

- [ ] **Step 3: Update ConfigApplyResult type**

```typescript
export type ConfigApplyResult = {
  backupNumber: number;
  reloadSuccess: boolean;
  reloadError?: string;
  issues: ConfigIssue[];
  requiresRestart?: boolean;
  restartReasons?: string[];
};
```

- [ ] **Step 4: Verify**

Run: `cd frontend && npm run check`
Expected: No errors.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "feat(frontend): update ConfigIssue and validate types for line, fixedValue, restart detection"
```

---

### Task C2: Create ConfigIssueList component

**Files:**
- Create: `frontend/src/lib/components/ui/ConfigIssueList.svelte`

- [ ] **Step 1: Create the component**

```svelte
<script lang="ts">
  import { X, AlertTriangle, Info, Wrench } from 'lucide-svelte';
  import type { ConfigIssue } from '$lib/api';

  let {
    issues = [] as ConfigIssue[],
    onjump,
    onfix,
  }: {
    issues: ConfigIssue[];
    onjump?: (line: number) => void;
    onfix?: (issue: ConfigIssue) => void;
  } = $props();

  function severityIcon(severity: string) {
    switch (severity) {
      case 'error': return { icon: X, color: 'text-red-500' };
      case 'warning': return { icon: AlertTriangle, color: 'text-amber-500' };
      default: return { icon: Info, color: 'text-blue-500' };
    }
  }
</script>

{#if issues.length > 0}
  <div class="bg-white rounded-lg shadow-sm border border-slate-200 p-4">
    <h3 class="text-sm font-semibold text-slate-700 mb-3">Issues ({issues.length})</h3>
    <div class="space-y-2">
      {#each issues as issue}
        {@const sev = severityIcon(issue.severity)}
        <div class="flex items-start gap-2 text-sm">
          <svelte:component this={sev.icon} class="w-4 h-4 mt-0.5 flex-shrink-0 {sev.color}" />
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 flex-wrap">
              {#if issue.line}
                <button
                  type="button"
                  onclick={() => onjump?.(issue.line!)}
                  class="inline-flex items-center px-1.5 py-0.5 text-xs font-mono font-medium text-blue-700 bg-blue-50 rounded hover:bg-blue-100 transition-colors"
                  title="Jump to line {issue.line}"
                >
                  L{issue.line}
                </button>
              {/if}
              <span class="font-medium text-slate-700">{issue.key}:</span>
              <span class="text-slate-600">{issue.message}</span>
            </div>
            {#if issue.suggestion}
              <p class="text-xs text-slate-500 mt-0.5">{issue.suggestion}</p>
            {/if}
            {#if issue.fixedValue}
              <button
                type="button"
                onclick={() => onfix?.(issue)}
                class="inline-flex items-center gap-1 mt-1 px-2 py-1 text-xs font-medium text-emerald-700 bg-emerald-50 border border-emerald-200 rounded hover:bg-emerald-100 transition-colors"
              >
                <Wrench class="w-3 h-3" />
                Fix: {issue.fixedValue}
              </button>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  </div>
{/if}
```

- [ ] **Step 2: Verify**

Run: `cd frontend && npm run check`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/ui/ConfigIssueList.svelte
git commit -m "feat(frontend): add ConfigIssueList component with jump-to-line and auto-fix actions"
```

---

### Task C3: Wire issue list interactions into config page

**Files:**
- Modify: `frontend/src/routes/config/+page.svelte`

- [ ] **Step 1: Import and use ConfigIssueList**

Import:
```typescript
import ConfigIssueList from '$lib/components/ui/ConfigIssueList.svelte';
```

Replace the inline issues rendering block (the `{#if issues.length > 0}` section containing the `{#each issues as issue}` loop) with:
```svelte
<ConfigIssueList
  {issues}
  onjump={(line) => editorRef?.scrollToLine(line)}
  onfix={(issue) => handleAutoFix(issue)}
/>
```

- [ ] **Step 2: Implement handleAutoFix function**

```typescript
function handleAutoFix(issue: ConfigIssue) {
  if (!issue.line || !issue.fixedValue) return;
  const lines = editContent.split('\n');
  const lineIdx = issue.line - 1;
  if (lineIdx < 0 || lineIdx >= lines.length) return;
  lines[lineIdx] = issue.fixedValue;
  editContent = lines.join('\n');
  // Re-validate after fix
  void handleValidate();
}
```

- [ ] **Step 3: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/config/+page.svelte
git commit -m "feat(frontend): wire click-to-jump and auto-fix into config editor page"
```

---

### Task C4: Enhance Apply confirmation with restart warning

**Files:**
- Modify: `frontend/src/routes/config/+page.svelte`

- [ ] **Step 1: Add restart state**

```typescript
let requiresRestart = $state(false);
let restartReasons = $state<string[]>([]);
```

- [ ] **Step 2: Update handleValidate to capture restart info**

After `const result = await validateConfig(editContent)`:
```typescript
requiresRestart = result.requiresRestart ?? false;
restartReasons = result.restartReasons ?? [];
```

- [ ] **Step 3: Update apply confirmation modal**

In the `showApplyConfirm` modal, add between the description paragraph and the buttons:
```svelte
{#if requiresRestart}
  <div class="mt-3 p-3 bg-amber-50 border border-amber-200 rounded-md">
    <p class="text-sm font-medium text-amber-800">This change requires a full service restart.</p>
    <p class="text-xs text-amber-700 mt-1">Active VPN sessions will be disconnected.</p>
    {#if restartReasons.length > 0}
      <ul class="mt-2 text-xs text-amber-700 list-disc list-inside">
        {#each restartReasons as reason}
          <li>{reason}</li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}
```

Change apply button text to be dynamic:
```svelte
{requiresRestart ? 'Restart & Apply' : 'Confirm Apply'}
```

- [ ] **Step 4: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/config/+page.svelte
git commit -m "feat(frontend): show restart warning in apply confirmation when directive changes require restart"
```

---

### Task C5: Auto-validate before Apply

**Files:**
- Modify: `frontend/src/routes/config/+page.svelte`

- [ ] **Step 1: Add validation tracking state**

```typescript
let lastValidatedContent = $state('');
```

- [ ] **Step 2: Update handleValidate to track validated content**

After successful validation, add:
```typescript
lastValidatedContent = editContent;
```

- [ ] **Step 3: Update requestApply to auto-validate**

```typescript
async function requestApply() {
  // Auto-validate if content changed since last validation
  if (editContent !== lastValidatedContent) {
    await handleValidate();
    // If validation found errors, block apply
    if (issues.some(i => i.severity === 'error')) {
      showError('Fix validation errors before applying.');
      return;
    }
  }
  showApplyConfirm = true;
}
```

- [ ] **Step 4: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/routes/config/+page.svelte
git commit -m "feat(frontend): require validation before apply, auto-validate if content changed"
```

---

## Phase D: Backend Network Inspector Extensions

### Task D1: Add firewall listener port check

**Files:**
- Modify: `backend/internal/ocserv/network_inspector.go`
- Modify: `backend/internal/ocserv/network_inspector_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestNetworkInspector_ListenerPortCheck(t *testing.T) {
    dir := t.TempDir()
    cfgPath := filepath.Join(dir, "ocserv.conf")
    require.NoError(t, os.WriteFile(cfgPath, []byte("tcp-port = 443\nudp-port = 443\n"), 0644))

    runner := &FakeCommandRunner{
        Results: map[string]CommandResult{
            "iptables": {Stdout: "-A INPUT -p tcp --dport 443 -j ACCEPT\n-A INPUT -p udp --dport 443 -j ACCEPT\n", ExitCode: 0},
        },
    }

    insp := NewNetworkInspectorWithConfig(runner, dir+"/proc", cfgPath)
    result, err := insp.Inspect(context.Background())
    require.NoError(t, err)

    var portCheck *PrerequisiteCheck
    for i := range result.Items {
        if result.Items[i].Name == "TCP Port 443 Allowed" {
            portCheck = &result.Items[i]
            break
        }
    }
    require.NotNil(t, portCheck)
    assert.Equal(t, "configured", portCheck.Status)
}
```

- [ ] **Step 2: Implement listener port firewall check**

Add `configPath string` to `NetworkInspector` struct. Create a new constructor `NewNetworkInspectorWithConfig` that accepts it (keep the old constructor for backward compat, passing empty string). Add method:

```go
func (i *NetworkInspector) checkListenerPorts(ctx context.Context) []PrerequisiteCheck {
    if i.configPath == "" {
        return nil
    }
    data, err := os.ReadFile(i.configPath)
    if err != nil {
        return nil
    }
    // Parse tcp-port and udp-port
    var tcpPort, udpPort string
    for _, line := range strings.Split(string(data), "\n") {
        key, value, ok := parseConfigLine(strings.TrimSpace(line))
        if !ok { continue }
        if key == "tcp-port" { tcpPort = value }
        if key == "udp-port" { udpPort = value }
    }

    var checks []PrerequisiteCheck
    // Check iptables -S INPUT for each port/protocol
    inputResult, err := i.runner.Run(ctx, "iptables", []string{"-S", "INPUT"}, nil)
    if err != nil {
        // Return unknown checks
        if tcpPort != "" {
            checks = append(checks, PrerequisiteCheck{Category: "firewall", Name: fmt.Sprintf("TCP Port %s Allowed", tcpPort), Status: "unknown", Expected: "ACCEPT rule for port " + tcpPort, Actual: err.Error()})
        }
        return checks
    }

    if tcpPort != "" {
        check := PrerequisiteCheck{Category: "firewall", Name: fmt.Sprintf("TCP Port %s Allowed", tcpPort), Expected: "ACCEPT rule for TCP port " + tcpPort, Suggestion: fmt.Sprintf("iptables -A INPUT -p tcp --dport %s -j ACCEPT", tcpPort)}
        if strings.Contains(inputResult.Stdout, "--dport "+tcpPort) && strings.Contains(inputResult.Stdout, "-p tcp") {
            check.Status = "configured"
            check.Actual = "rule found"
        } else {
            check.Status = "missing"
            check.Actual = "no matching rule"
        }
        checks = append(checks, check)
    }
    if udpPort != "" && udpPort != tcpPort {
        // Similar for UDP
        check := PrerequisiteCheck{Category: "firewall", Name: fmt.Sprintf("UDP Port %s Allowed", udpPort), Expected: "ACCEPT rule for UDP port " + udpPort, Suggestion: fmt.Sprintf("iptables -A INPUT -p udp --dport %s -j ACCEPT", udpPort)}
        if strings.Contains(inputResult.Stdout, "--dport "+udpPort) && strings.Contains(inputResult.Stdout, "-p udp") {
            check.Status = "configured"
            check.Actual = "rule found"
        } else {
            check.Status = "missing"
            check.Actual = "no matching rule"
        }
        checks = append(checks, check)
    }
    return checks
}
```

Call from `Inspect()`.

- [ ] **Step 3: Run tests**

Run: `cd backend && go test ./internal/ocserv/ -run TestNetworkInspector -v`
Expected: All pass.

- [ ] **Step 4: Update main.go to pass config path to NetworkInspector**

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ocserv/network_inspector.go backend/internal/ocserv/network_inspector_test.go backend/cmd/ocserv-panel/main.go
git commit -m "feat(config): add firewall listener port check to network inspector"
```

---

### Task D2: Add VPN subnet overlap detection

**Files:**
- Modify: `backend/internal/ocserv/network_inspector.go`
- Modify: `backend/internal/ocserv/network_inspector_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestNetworkInspector_SubnetOverlap(t *testing.T) {
    dir := t.TempDir()
    cfgPath := filepath.Join(dir, "ocserv.conf")
    require.NoError(t, os.WriteFile(cfgPath, []byte("ipv4-network = 10.10.10.0/24\n"), 0644))

    runner := &FakeCommandRunner{
        Results: map[string]CommandResult{
            "ip": {Stdout: "10.0.0.0/8 dev eth0 proto kernel scope link src 10.0.0.1\n192.168.1.0/24 dev wlan0\n", ExitCode: 0},
        },
    }

    insp := NewNetworkInspectorWithConfig(runner, dir+"/proc", cfgPath)
    result, err := insp.Inspect(context.Background())
    require.NoError(t, err)

    var overlapCheck *PrerequisiteCheck
    for i := range result.Items {
        if result.Items[i].Name == "VPN Subnet Overlap" {
            overlapCheck = &result.Items[i]
            break
        }
    }
    require.NotNil(t, overlapCheck)
    assert.Equal(t, "missing", overlapCheck.Status) // "missing" = overlap detected (problematic)
    assert.Contains(t, overlapCheck.Actual, "10.0.0.0/8")
}
```

- [ ] **Step 2: Implement subnet overlap check**

```go
func (i *NetworkInspector) checkSubnetOverlap(ctx context.Context) []PrerequisiteCheck {
    if i.configPath == "" {
        return nil
    }
    data, err := os.ReadFile(i.configPath)
    if err != nil {
        return nil
    }

    var vpnNetwork string
    for _, line := range strings.Split(string(data), "\n") {
        key, value, ok := parseConfigLine(strings.TrimSpace(line))
        if !ok { continue }
        if key == "ipv4-network" { vpnNetwork = value; break }
    }
    if vpnNetwork == "" {
        return nil
    }

    _, vpnNet, err := net.ParseCIDR(vpnNetwork)
    if err != nil {
        return nil
    }

    // Get host routes
    routeResult, err := i.runner.Run(ctx, "ip", []string{"route", "show"}, nil)
    if err != nil {
        return []PrerequisiteCheck{{
            Category: "routing", Name: "VPN Subnet Overlap", Status: "unknown",
            Expected: "no overlap with host routes", Actual: err.Error(),
        }}
    }

    var overlapping []string
    for _, line := range strings.Split(routeResult.Stdout, "\n") {
        fields := strings.Fields(line)
        if len(fields) < 1 { continue }
        _, hostNet, err := net.ParseCIDR(fields[0])
        if err != nil { continue }
        if vpnNet.Contains(hostNet.IP) || hostNet.Contains(vpnNet.IP) {
            overlapping = append(overlapping, fields[0])
        }
    }

    check := PrerequisiteCheck{
        Category: "routing", Name: "VPN Subnet Overlap",
        Expected: fmt.Sprintf("VPN subnet %s does not overlap host networks", vpnNetwork),
        Suggestion: "Choose a VPN subnet that does not overlap with existing host networks.",
    }
    if len(overlapping) > 0 {
        check.Status = "missing"
        check.Actual = fmt.Sprintf("overlaps with: %s", strings.Join(overlapping, ", "))
    } else {
        check.Status = "configured"
        check.Actual = "no overlap detected"
    }
    return []PrerequisiteCheck{check}
}
```

- [ ] **Step 3: Run tests**

Run: `cd backend && go test ./internal/ocserv/ -run TestNetworkInspector -v`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/ocserv/network_inspector.go backend/internal/ocserv/network_inspector_test.go
git commit -m "feat(config): add VPN subnet overlap detection to network inspector"
```

---

### Task D3: Surface DNS validity in network tab

**Files:**
- Modify: `backend/internal/ocserv/network_inspector.go`
- Modify: `backend/internal/ocserv/network_inspector_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestNetworkInspector_DNSCheck(t *testing.T) {
    dir := t.TempDir()
    cfgPath := filepath.Join(dir, "ocserv.conf")
    require.NoError(t, os.WriteFile(cfgPath, []byte("dns = 8.8.8.8\ndns = invalid\n"), 0644))

    runner := &FakeCommandRunner{}
    insp := NewNetworkInspectorWithConfig(runner, dir+"/proc", cfgPath)
    result, err := insp.Inspect(context.Background())
    require.NoError(t, err)

    var dnsChecks []PrerequisiteCheck
    for _, item := range result.Items {
        if item.Category == "routing" && strings.HasPrefix(item.Name, "DNS") {
            dnsChecks = append(dnsChecks, item)
        }
    }
    require.Len(t, dnsChecks, 2)
    assert.Equal(t, "configured", dnsChecks[0].Status)
    assert.Equal(t, "missing", dnsChecks[1].Status)
}
```

- [ ] **Step 2: Implement DNS check**

```go
func (i *NetworkInspector) checkConfigDNS() []PrerequisiteCheck {
    if i.configPath == "" {
        return nil
    }
    data, err := os.ReadFile(i.configPath)
    if err != nil {
        return nil
    }

    var checks []PrerequisiteCheck
    for _, line := range strings.Split(string(data), "\n") {
        key, value, ok := parseConfigLine(strings.TrimSpace(line))
        if !ok || key != "dns" { continue }
        check := PrerequisiteCheck{
            Category: "routing",
            Name:     fmt.Sprintf("DNS Server: %s", value),
            Expected: "valid IP address",
        }
        if net.ParseIP(value) != nil {
            check.Status = "configured"
            check.Actual = value
        } else {
            check.Status = "missing"
            check.Actual = fmt.Sprintf("invalid IP: %s", value)
            check.Suggestion = "Set dns to a valid IPv4 or IPv6 address, e.g.: dns = 8.8.8.8"
        }
        checks = append(checks, check)
    }
    return checks
}
```

Call from `Inspect()`.

- [ ] **Step 3: Run tests**

Run: `cd backend && go test ./internal/ocserv/ -run TestNetworkInspector -v`
Expected: All pass.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/ocserv/network_inspector.go backend/internal/ocserv/network_inspector_test.go
git commit -m "feat(config): surface DNS IP validation in network prerequisite checks"
```

---

## Phase E: Integration and Polish

### Task E1: Show inline diff in apply confirmation modal

**Files:**
- Modify: `frontend/src/routes/config/+page.svelte`

- [ ] **Step 1: Fetch diff before showing apply confirmation**

Update `requestApply()` to fetch diff after validation:
```typescript
async function requestApply() {
  if (editContent !== lastValidatedContent) {
    await handleValidate();
    if (issues.some(i => i.severity === 'error')) {
      showError('Fix validation errors before applying.');
      return;
    }
  }
  // Fetch diff for display in confirmation
  try {
    const result = await diffConfig(editContent);
    diffText = result.diff;
  } catch {
    diffText = '';
  }
  showApplyConfirm = true;
}
```

- [ ] **Step 2: Show diff in apply confirmation modal**

Add between the description and the restart warning:
```svelte
{#if diffText}
  <div class="mt-3 max-h-[200px] overflow-auto border border-slate-200 rounded-md p-2">
    <div class="font-mono text-xs leading-relaxed">
      {#each diffText.split('\n') as line}
        {@const dl = renderDiffLine(line)}
        <div class="px-1 {dl.cls}">{dl.text}</div>
      {/each}
    </div>
  </div>
{/if}
```

- [ ] **Step 3: Verify build**

Run: `cd frontend && npm run build`
Expected: Build succeeds.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/routes/config/+page.svelte
git commit -m "feat(frontend): show inline diff preview in apply confirmation modal"
```

---

### Task E2: CodeMirror theme polish

**Files:**
- Modify: `frontend/src/lib/components/ui/config-editor-extensions.ts`

- [ ] **Step 1: Refine theme to match Tailwind design system**

Update the `ocservTheme` to ensure consistent visual treatment:
- Ensure proper height inheritance from parent
- Add focused border style matching `focus:ring-2 focus:ring-blue-500`
- Ensure scrollbar styling is consistent

```typescript
export const ocservTheme = EditorView.theme({
  '&': { fontSize: '13px', fontFamily: 'ui-monospace, SFMono-Regular, Consolas, monospace' },
  '&.cm-focused': { outline: '2px solid #3b82f6', outlineOffset: '-1px', borderRadius: '0.5rem' },
  '.cm-scroller': { overflow: 'auto', minHeight: '400px' },
  '.cm-content': { padding: '12px 0' },
  '.cm-gutters': { backgroundColor: '#f8fafc', borderRight: '1px solid #e2e8f0', color: '#64748b' },
  '.cm-activeLineGutter': { backgroundColor: '#f1f5f9' },
  '.cm-activeLine': { backgroundColor: '#f8fafc' },
  '.cm-selectionBackground': { backgroundColor: '#e2e8f0 !important' },
  '.cm-line-error': { backgroundColor: 'rgba(254, 202, 202, 0.3)' },
  '.cm-line-warning': { backgroundColor: 'rgba(254, 240, 138, 0.3)' },
  '.cm-line-info': { backgroundColor: 'rgba(191, 219, 254, 0.2)' },
  '.cm-issue-gutter': { width: '18px', paddingLeft: '3px' },
  '.cm-gutter-error': { color: '#ef4444', fontSize: '11px', lineHeight: '1.5' },
  '.cm-gutter-warning': { color: '#f59e0b', fontSize: '11px', lineHeight: '1.5' },
  '.cm-gutter-info': { color: '#3b82f6', fontSize: '11px', lineHeight: '1.5' },
});
```

- [ ] **Step 2: Verify build**

Run: `cd frontend && npm run check`
Expected: No errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/components/ui/config-editor-extensions.ts
git commit -m "style(frontend): refine CodeMirror theme to match panel's Tailwind design"
```

---

### Task E3: End-to-end handler tests for new API fields

**Files:**
- Modify: `backend/internal/server/config_handlers_test.go`

- [ ] **Step 1: Write integration tests**

```go
func TestConfigValidateHandler_ReturnsLineNumbers(t *testing.T) {
    // Setup full AppRouter with temp config file
    // POST /api/config/validate with {"content": "# ok\ntcp-port = 99999\n"}
    // Assert response includes issue with line=2
}

func TestConfigValidateHandler_ReturnsCIDRFixedValue(t *testing.T) {
    // POST with content containing route = 10.0.0.0/255.255.255.0
    // Assert issue has fixedValue = "route = 10.0.0.0/24"
}

func TestConfigValidateHandler_ReturnsRestartRequired(t *testing.T) {
    // Current config has tcp-port = 443
    // POST with content changing to tcp-port = 8443
    // Assert requiresRestart = true
}
```

- [ ] **Step 2: Run tests**

Run: `cd backend && go test ./internal/server/ -run TestConfigValidate -v`
Expected: All pass.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/server/config_handlers_test.go
git commit -m "test(config): add integration tests for line numbers, fixedValue, and restart detection in API"
```

---

### Task E4: Verify full build and all tests pass

- [ ] **Step 1: Run backend tests**

Run: `cd backend && go test ./...`
Expected: All packages pass.

- [ ] **Step 2: Run frontend build**

Run: `cd frontend && npm run build`
Expected: Build succeeds.

- [ ] **Step 3: Run frontend type check**

Run: `cd frontend && npm run check`
Expected: No errors.

---

## Verification

### Manual Testing Checklist

1. **Open Config tab** — CodeMirror editor renders with line numbers and syntax highlighting
2. **Click Validate** — issues appear with line numbers (blue "L5" badges)
3. **Click line badge** — editor scrolls to and highlights that line
4. **Create a netmask route** (e.g., `route = 10.0.0.0/255.255.255.0`) — validate shows warning with "Fix" button
5. **Click Fix** — line content updates to `route = 10.0.0.0/24`, auto re-validates
6. **Change tcp-port value** — validate reports `requiresRestart: true`
7. **Click Apply** — confirmation modal shows diff inline and restart warning with reasons
8. **Network tab** — shows port firewall checks, subnet overlap check, DNS validity
9. **Read-only user** — editor is non-editable, no action buttons visible
10. **Backups tab** — unchanged behavior, all actions still work

### Automated Tests

```bash
cd backend && go test ./... -v                # All backend tests
cd frontend && npm run check                  # TypeScript/Svelte type check
cd frontend && npm run build                  # Production build
```

---

## Risks and Open Questions

1. **CodeMirror 6 SSR compatibility** — CM6 requires DOM. SvelteKit may attempt SSR. Mitigation: wrap CM6 initialization in `onMount` (client-only). If issues persist, use dynamic import with `ssr: false` in route config.

2. **CM6 bundle size** — CodeMirror adds ~100KB gzipped. Acceptable for an admin panel but worth noting. Tree-shaking helps — only import used extensions.

3. **FakeCommandRunner key matching** — D1-D3 tests need `FakeCommandRunner` to match commands by binary name (`"iptables"`, `"ip"`). Verify the existing fake supports this lookup pattern or extend it.

4. **Restart vs reload boundary** — The hardcoded list covers known restart-required directives. If ocserv adds new listen-affecting directives in future versions, the list needs updating. Document this assumption.

5. **FixedValue line replacement** — The auto-fix replaces an entire line. If the config uses unusual formatting (extra spaces, inline comments after value), the fix may normalize formatting. This is acceptable behavior.

---

## Implementation Sequence (dependency graph)

```
Phase A (backend, sequential): A1 → A2 → A3 → A4 → A5 → A6 → A7
Phase B (frontend, after A7): B1 → B2 → B3 → B4
Phase C (frontend, after B4): C1 → C2 → C3 → C4 → C5
Phase D (backend, parallel with B/C): D1 → D2 → D3
Phase E (after C5 and D3): E1 → E2 → E3 → E4
```

Phases A and D are backend-only with no frontend dependency. Phase B requires A7 (API returns new fields). Phase C requires B4 (editor component exists). Phase E ties everything together.

**Parallelization opportunity:** Phase D (network inspector extensions) can run in parallel with Phases B+C since they touch different files with no shared state.
