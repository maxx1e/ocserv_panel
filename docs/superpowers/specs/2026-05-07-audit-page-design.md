# Audit Page Design Specification

**Date:** 2026-05-07  
**Status:** Approved  
**Scope:** Full-stack Audit page for operational log visibility

---

## Overview

An Audit page providing operational log visibility for two sources:
- **Web UI Backend Logs** — panel service logs (zerolog JSON)
- **ocserv VPN Logs** — ocserv service logs (syslog format)

The page replaces the current EmptyState placeholder at `/audit`. It allows admins to inspect logs safely without direct shell access.

---

## Architecture

```
Frontend (Audit Page)
  ├── Tab: "Web UI Backend Logs" → GET /api/audit/logs?source=panel&...
  ├── Tab: "ocserv VPN Logs"    → GET /api/audit/logs?source=ocserv&...
  └── Actions: search, filter, export, refresh
         │
         ▼
Backend Handler (audit_log_handlers.go)
  ├── Authentication + RBAC check
  ├── Input validation (source allowlist, time bounds, line limits)
  └── Delegates to AuditLogService
         │
         ▼
AuditLogService (audit_log_service.go)
  ├── LogReader interface (abstraction)
  │   ├── FileLogReader (reads from PanelLogPath / OcservLogPath)
  │   └── JournalLogReader (fallback: journalctl -u <unit>)
  ├── Filtering (level, text search, time range)
  ├── Pagination (offset/limit with max 500 lines)
  └── Export (JSON/CSV, records audit event)
```

---

## API Endpoints

### GET /api/audit/logs

Read logs from a specified source with filtering and pagination.

**Query Parameters:**

| Parameter | Required | Type | Default | Description |
|-----------|----------|------|---------|-------------|
| `source` | yes | string | — | `"panel"` or `"ocserv"` |
| `search` | no | string | — | Text substring filter (case-insensitive) |
| `level` | no | string | — | Filter by level: `debug`, `info`, `warn`, `error` |
| `from` | no | string | — | ISO 8601 start timestamp |
| `to` | no | string | — | ISO 8601 end timestamp |
| `limit` | no | int | 100 | Max lines returned (1-500) |
| `offset` | no | int | 0 | Pagination offset |
| `order` | no | string | `desc` | Sort order: `asc` or `desc` |

**Response (200):**

```json
{
  "source": "panel",
  "lines": [
    {
      "timestamp": "2026-05-07T10:23:45Z",
      "level": "info",
      "message": "user login succeeded",
      "component": "auth",
      "raw": "full raw log line",
      "metadata": { "requestId": "abc123", "user": "admin" }
    }
  ],
  "total": 1523,
  "offset": 0,
  "limit": 100,
  "hasMore": true
}
```

**Error Responses:**
- 400: Invalid source, invalid time range, invalid limit
- 401: Not authenticated
- 403: Missing PermissionAuditLogsRead
- 503: Log source unavailable (file missing + journal unavailable)

### GET /api/audit/logs/export

Export filtered logs as JSON or CSV download. Same query parameters as `/api/audit/logs` plus:

| Parameter | Required | Type | Default | Description |
|-----------|----------|------|---------|-------------|
| `format` | no | string | `json` | Export format: `json` or `csv` |

Returns file download with `Content-Disposition: attachment` header. Limited to 5000 lines maximum.

Creates an audit event: `action: "audit.logs.export"`.

### GET /api/audit/logs/sources

List available log sources with metadata.

**Response (200):**

```json
{
  "sources": [
    {
      "name": "panel",
      "label": "Web UI Backend Logs",
      "available": true,
      "path": "/var/log/ocserv-panel/panel.log",
      "reader": "file",
      "logLevel": null,
      "logLevelLabel": null
    },
    {
      "name": "ocserv",
      "label": "ocserv VPN Logs",
      "available": true,
      "path": "/var/log/ocserv/ocserv.log",
      "reader": "file",
      "logLevel": 2,
      "logLevelLabel": "info"
    }
  ]
}
```

The `logLevel` and `logLevelLabel` fields are populated for ocserv from the parsed ocserv.conf `log-level` setting.

---

## Log Reading Strategy

### LogReader Interface

```go
type LogReader interface {
    ReadLines(ctx context.Context, opts ReadOpts) (*ReadResult, error)
    Available(ctx context.Context) bool
}

type ReadOpts struct {
    Limit  int
    Offset int
    Order  string // "asc" | "desc"
    Search string
    Level  string
    From   time.Time
    To     time.Time
}

type ReadResult struct {
    Lines []LogLine
    Total int
    HasMore bool
}

type LogLine struct {
    Timestamp time.Time         `json:"timestamp"`
    Level     string            `json:"level"`
    Message   string            `json:"message"`
    Component string            `json:"component"`
    Raw       string            `json:"raw"`
    Metadata  map[string]string `json:"metadata,omitempty"`
}
```

### FileLogReader (Primary)

- Opens configured file path (from config, never from request)
- Reads file from end backwards (for `desc` order) using buffered reverse scan
- Parses lines based on source format (JSON for panel, syslog for ocserv)
- Applies filters (level, search, time range) during scan
- Stops after collecting `limit + offset` matching lines
- File scan capped at 10MB to prevent memory exhaustion on large log files
- Returns `Available() = false` if file doesn't exist or isn't readable

### JournalLogReader (Fallback)

- Used when file reader reports `Available() = false`
- Executes: `journalctl -u <unit> --output=json --since=<from> --until=<to> --no-pager -n <limit>`
- Unit name from config: `SystemdUnitName` for ocserv, `"ocserv-panel"` for panel
- Command built from validated inputs only — no shell interpolation
- Uses existing `CommandRunner` interface for testability
- Returns `Available() = false` if journalctl command fails or unit not found

### Source Resolution

```
1. Check if FileLogReader.Available() for source
2. If yes → use FileLogReader
3. If no → check JournalLogReader.Available()
4. If yes → use JournalLogReader
5. If neither → return 503 with descriptive error
```

---

## Log Parsing

### Panel Logs (zerolog JSON)

Input format:
```json
{"level":"info","time":"2026-05-07T10:23:45Z","message":"request completed","component":"server","request_id":"abc123","caller":"handlers.go:42"}
```

Parsing:
- Direct JSON unmarshal
- Map `time` → `timestamp`, `level` → `level`, `message` → `message`
- Known fields extracted; remaining fields → `metadata` map
- Original JSON stored in `raw`

### ocserv Logs (syslog format)

Input format:
```
May  7 10:23:45 host ocserv[1234]: worker: connected user 'admin' from 192.168.1.1
```

Parsing:
- Regex: `^(\w+\s+\d+\s+\d+:\d+:\d+)\s+\S+\s+ocserv\[(\d+)\]:\s*(.+)$`
- Map matched groups to timestamp (with year inference), component (from prefix before `:`), message
- Level inference: "error" if contains "error"/"failed", "warn" if contains "warning", else "info"
- Original line stored in `raw`

---

## Sensitive Data Masking

Before sending log lines to the client, apply regex-based masking:

1. Password patterns: `password[=:]\s*\S+` → `password=***`
2. Token patterns: `token[=:]\s*[A-Za-z0-9+/=_-]{8,}` → `token=***`
3. Key patterns: `(key|secret|credential)[=:]\s*\S+` → `$1=***`
4. Cookie patterns: `cookie[=:]\s*\S+` → `cookie=***`
5. Authorization headers: `Authorization:\s*\S+` → `Authorization: ***`

Masking is applied to both `message` and `raw` fields.

---

## RBAC

### New Permissions

| Permission | Roles | Purpose |
|------------|-------|---------|
| `PermissionAuditLogsRead` | owner, viewer | View logs via API |
| `PermissionAuditLogsExport` | owner | Export/download logs |

### Authorization Checks

- `/api/audit/logs` — requires `PermissionAuditLogsRead`
- `/api/audit/logs/export` — requires `PermissionAuditLogsExport`
- `/api/audit/logs/sources` — requires `PermissionAuditLogsRead`

---

## Frontend Design

### Page Structure

```
/audit route (audit/+page.svelte)
├── Header
│   ├── Title: "Audit"
│   ├── Subtitle: "Review backend and VPN service logs."
│   └── Export button (owner only, top-right)
├── Toolbar
│   ├── Tabs: "Web UI Backend Logs" | "ocserv VPN Logs"
│   ├── Search input (debounced 300ms)
│   ├── Level filter dropdown (All / Debug / Info / Warn / Error)
│   ├── Time range: Quick select (Last 1h, 6h, 24h, 7d) or custom
│   ├── Auto-refresh toggle (default off, 10s interval when on)
│   └── Manual refresh button (icon)
├── Log Level Banner (ocserv tab only, when level >= 8)
│   └── Warning: "ocserv log-level is set to X (sensitive). Logs may contain credentials."
├── Log Display
│   ├── Table/list with columns: Time | Level | Component | Message
│   ├── Level shown as colored badge (gray=debug, blue=info, yellow=warn, red=error)
│   ├── Each row has copy-to-clipboard button (copies raw line)
│   ├── Click row to expand: shows full raw line + metadata
│   └── Pagination: offset-based with prev/next + page indicator
└── States
    ├── Loading: Spinner centered
    ├── Empty: "No log entries match your filters" with suggestion to adjust
    ├── Error: Dismissible error banner with message
    └── Unavailable: Source not configured or inaccessible
```

### Component Breakdown

| Component | File | Purpose |
|-----------|------|---------|
| Page | `audit/+page.svelte` | Main orchestration |
| LogToolbar | `audit/LogToolbar.svelte` | Search, filters, actions |
| LogTable | `audit/LogTable.svelte` | Log line display + pagination |
| LogRow | `audit/LogRow.svelte` | Single expandable log entry |
| LogLevelBanner | `audit/LogLevelBanner.svelte` | Sensitivity warning for ocserv |

### State Management

Using Svelte 5 runes:
- `$state` for active tab, search query, level filter, time range, auto-refresh toggle
- `$derived` for filtered/paginated display
- `$effect` for auto-refresh polling interval

### API Integration

New functions in `api.ts`:
```typescript
getAuditLogs(params: AuditLogParams): Promise<AuditLogResponse>
getAuditLogSources(): Promise<AuditSourcesResponse>
exportAuditLogs(params: AuditLogParams & { format: string }): Promise<Blob>
```

---

## Configuration

### Existing Fields Used

- `InstallationConfig.OcservLogPath` — path to ocserv log file
- `InstallationConfig.PanelLogPath` — path to panel log file
- `MonitoringConfig.LogFileReadEnabled` — master switch for log reading
- `ServiceConfig.SystemdUnitName` — for journal fallback

### New Config Fields

- `InstallationConfig.PanelSystemdUnit` — systemd unit name for panel service (default: `"ocserv-panel"`)

### ocserv Log Level

Read from ocserv.conf via the existing settings/config parsing:
- Parse `log-level = N` from ocserv config
- Map to labels: 0=default, 1=basic, 2=info, 3=debug, 4=http, 8=sensitive, 9=TLS
- Expose via `/api/audit/logs/sources` response

---

## Security Controls

1. **Source allowlist**: Only `"panel"` and `"ocserv"` values accepted. Mapped server-side to configured paths.
2. **No path in request**: Log file paths come from server config only. Client cannot specify paths.
3. **Sensitive masking**: Applied server-side before response. Covers passwords, tokens, keys, cookies, auth headers.
4. **Line limits**: Max 500 lines per request, max 5000 for export. File scan capped at 10MB.
5. **Time bounds validation**: `from`/`to` must be valid ISO 8601. Max range 7 days for normal reads, 30 days for export.
6. **RBAC enforcement**: All endpoints require authentication + appropriate permission.
7. **Audit trail**: Export action logged as audit event.
8. **Journal safety**: Command arguments built programmatically from validated inputs. Uses `CommandRunner` (no shell).
9. **No shell interpolation**: File paths and journal arguments never pass through shell interpretation.

---

## Testing Strategy

### Backend Unit Tests

| Test | File | Covers |
|------|------|--------|
| Panel log JSON parsing | `log_parser_test.go` | Correct field extraction, malformed JSON handling |
| ocserv syslog parsing | `log_parser_test.go` | Regex matching, level inference, timestamp parsing |
| Sensitive data masking | `log_masker_test.go` | All mask patterns, edge cases, no false positives |
| FileLogReader | `file_reader_test.go` | Read, filter, paginate, missing file, large file cap |
| JournalLogReader | `journal_reader_test.go` | Command building, output parsing, error handling |
| AuditLogService | `audit_log_service_test.go` | Source resolution, RBAC denial, parameter validation |
| API handlers | `audit_log_handlers_test.go` | HTTP status codes, response format, auth checks |

### Frontend Tests

- Component rendering for each tab
- Filter application and search debounce
- Pagination state transitions
- Error/empty/loading state rendering
- Copy-to-clipboard action
- Auto-refresh toggle behavior

### Integration

- Full handler test with fake log reader
- RBAC denial for viewer trying to export
- Invalid parameter rejection (bad source, bad time format)

---

## File Inventory

### Backend (new files)

- `backend/internal/app/audit_log_service.go` — service orchestration
- `backend/internal/app/audit_log_service_test.go` — service tests
- `backend/internal/server/audit_log_handlers.go` — HTTP handlers
- `backend/internal/server/audit_log_handlers_test.go` — handler tests
- `backend/internal/logs/reader.go` — LogReader interface + types
- `backend/internal/logs/file_reader.go` — FileLogReader implementation
- `backend/internal/logs/file_reader_test.go` — file reader tests
- `backend/internal/logs/journal_reader.go` — JournalLogReader implementation
- `backend/internal/logs/journal_reader_test.go` — journal reader tests
- `backend/internal/logs/parser.go` — Log line parsers (JSON + syslog)
- `backend/internal/logs/parser_test.go` — parser tests
- `backend/internal/logs/masker.go` — Sensitive data masking
- `backend/internal/logs/masker_test.go` — masker tests
- `backend/internal/logs/fake.go` — Fake LogReader for testing

### Backend (modified files)

- `backend/internal/rbac/rbac.go` — Add PermissionAuditLogsRead, PermissionAuditLogsExport
- `backend/internal/server/server.go` — Register new routes
- `backend/internal/config/config.go` — Add PanelSystemdUnit field

### Frontend (modified files)

- `frontend/src/routes/audit/+page.svelte` — Replace EmptyState with full implementation
- `frontend/src/lib/api.ts` — Add audit log API functions and types

### Frontend (new files)

- `frontend/src/routes/audit/LogToolbar.svelte` — Toolbar component
- `frontend/src/routes/audit/LogTable.svelte` — Log display table
- `frontend/src/routes/audit/LogRow.svelte` — Expandable log row
- `frontend/src/routes/audit/LogLevelBanner.svelte` — Sensitivity warning

---

## Known Constraints

- Log files with non-UTF8 content will have invalid bytes replaced
- Very large log files (>10MB) will only show the most recent entries within the scan window
- Journal fallback requires journalctl to be available on the system
- Time range filtering for ocserv syslog logs requires year inference (uses current year)
- Auto-refresh at 10s interval may miss lines if logs rotate between polls
