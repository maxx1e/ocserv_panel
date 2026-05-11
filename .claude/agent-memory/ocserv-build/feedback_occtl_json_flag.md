---
name: occtl-json-flag-format
description: occtl uses -j -n global flags for JSON output, NOT --output-type json. Field names have spaces.
type: feedback
---

occtl uses `-j` (or `--json`) as a GLOBAL flag BEFORE the subcommand for JSON output. Also use `-n` for no color/formatting. The flag `--output-type json` does NOT exist.

**Why:** The original implementation incorrectly used `--output-type json` (appended after subcommand) which is not a valid occtl option, causing all session/user/status data to fail silently.

**How to apply:**
- Correct: `occtl -j -n show users`
- Wrong: `occtl show users --output-type json`
- JSON field names use spaces: `"Remote IP"`, `"Connected at"`, `"DTLS cipher"`, `"User-Agent"`
- Numeric values may be strings: `"RX": "31683792"`, `"MTU": "1292"`
- Timestamps are `"2006-01-02 15:04"` format (NOT RFC3339)
- `show sessions all` returns array with `"Session"`, `"Created"`, `"State"`, `"Username"`, `"Remote IP"`, `"User-Agent"`
- `show users` returns rich data with `"ID"`, `"IPv4"`, `"Device"`, `"raw_connected_at"` (unix), `"RX"`, `"TX"`, `"DTLS cipher"`, `"Session"`, etc.
- Some JSON has duplicate keys (`"Routes"` appears twice) — use json.RawMessage for those
- `show session <SID>` returns an ARRAY (even for single session)
- Reference: `/opt/ocserv_panel/.claude/rules/occtl-excamples.md`
