# Root Execution Simplification — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Simplify the deployment model by running ocserv-panel as root, eliminating sudoers, dedicated user/group, ACL setup, and systemd hardening.

**Architecture:** The Go application code is unchanged — it calls system binaries directly through the `CommandRunner` interface. Only deployment artifacts (systemd unit, install script, Makefile, config defaults) are modified. The sudoers file is deleted entirely.

**Tech Stack:** Bash (install.sh), systemd, TOML (config), Go (config defaults), Make

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `deploy/ocserv-panel.service` | Modify | Remove User/Group/hardening directives |
| `deploy/sudoers.d/ocserv-panel` | Delete | No longer needed |
| `deploy/install.sh` | Modify | Remove user creation, sudoers, ACL logic |
| `deploy/config.toml.example` | Modify | Update user/group defaults to root |
| `backend/internal/config/config.go` | Modify | Change default User/Group from "ocserv-panel" to "root" |
| `Makefile` | Modify | Remove sudoers from release tarball |

---

### Task 1: Simplify Systemd Unit

**Files:**
- Modify: `deploy/ocserv-panel.service`

- [ ] **Step 1: Replace the systemd unit file**

Replace the entire content of `deploy/ocserv-panel.service` with:

```ini
[Unit]
Description=ocserv Panel - Web UI for OpenConnect VPN Administration
After=network-online.target ocserv.service
Wants=network-online.target ocserv.service

[Service]
Type=simple
WorkingDirectory=/var/lib/ocserv-panel

ExecStart=/usr/local/bin/ocserv-panel

Environment="OCSERV_PANEL_ADDR=0.0.0.0:8443"
Environment="OCSERV_PANEL_DB_DSN=file:/var/lib/ocserv-panel/ocserv_panel.db?_journal=WAL&_busy_timeout=5000"
Environment="OCSERV_PANEL_CONFIG_PATH=/etc/ocserv-panel/config.toml"
Environment="OCSERV_PANEL_TSDB_PATH=/var/lib/ocserv-panel/metrics"
Environment="OCSERV_PANEL_MIGRATIONS_DIR=/usr/share/ocserv-panel/migrations"
Environment="OCSERV_PANEL_STATIC_DIR=/usr/share/ocserv-panel/static"
Environment="OCSERV_PANEL_TLS_CERT=/etc/letsencrypt/live/light-mirror.ru/fullchain.pem"
Environment="OCSERV_PANEL_TLS_KEY=/etc/letsencrypt/live/light-mirror.ru/privkey.pem"
Environment="OCSERV_PANEL_COOKIE_SECURE=true"

Restart=on-failure
RestartSec=5s
TimeoutStartSec=30s
TimeoutStopSec=10s

PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Removed: `User=`, `Group=`, `ProtectSystem=strict`, `ProtectHome=true`, `ReadWritePaths=`, `ReadOnlyPaths=`, `CapabilityBoundingSet=`, `NoNewPrivileges=true`, `PrivateDevices=true`, `MemoryDenyWriteExecute=true`, `ProtectKernelTunables=true`, `ProtectKernelModules=true`, `ProtectControlGroups=true`, `RestrictAddressFamilies=`, `RestrictNamespaces=true`, `LockPersonality=true`.

Kept: `PrivateTmp=true` (zero-cost isolation, no conflict with root).

- [ ] **Step 2: Verify unit file syntax**

Run: `systemd-analyze verify deploy/ocserv-panel.service 2>&1 || true`

Expected: No critical errors (warnings about missing ExecStart binary path are OK in dev).

- [ ] **Step 3: Commit**

```bash
git add deploy/ocserv-panel.service
git commit -m "deploy: simplify systemd unit to run as root

Remove User/Group and all security hardening directives.
Service now runs as root for direct access to ocserv socket,
certificates, and system binaries without sudoers."
```

---

### Task 2: Delete Sudoers File

**Files:**
- Delete: `deploy/sudoers.d/ocserv-panel`

- [ ] **Step 1: Remove the sudoers file**

```bash
git rm deploy/sudoers.d/ocserv-panel
```

- [ ] **Step 2: Remove the sudoers.d directory if empty**

```bash
rmdir deploy/sudoers.d 2>/dev/null && git add deploy/sudoers.d || true
```

- [ ] **Step 3: Commit**

```bash
git commit -m "deploy: remove sudoers file

No longer needed — service runs as root and has direct access
to systemctl, occtl, and ocpasswd without privilege escalation."
```

---

### Task 3: Remove Sudoers from Makefile

**Files:**
- Modify: `Makefile` (project root)

- [ ] **Step 1: Remove sudoers packaging lines**

In the `release` target, remove these two lines:

```makefile
	@mkdir -p $(DIST)/migrations $(DIST)/sudoers.d
	cp deploy/sudoers.d/ocserv-panel $(DIST)/sudoers.d/
```

Replace with:

```makefile
	@mkdir -p $(DIST)/migrations
```

- [ ] **Step 2: Verify make target parses**

Run: `make -n release 2>&1 | head -20`

Expected: No errors about missing sudoers path.

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "build: remove sudoers from release tarball"
```

---

### Task 4: Update Config Defaults

**Files:**
- Modify: `backend/internal/config/config.go` (lines 118-119)
- Modify: `deploy/config.toml.example` (lines 36-37)

- [ ] **Step 1: Update Go config defaults**

In `backend/internal/config/config.go`, change lines 118-119 from:

```go
			User:            "ocserv-panel",
			Group:           "ocserv-panel",
```

To:

```go
			User:            "root",
			Group:           "root",
```

- [ ] **Step 2: Update config.toml.example**

In `deploy/config.toml.example`, change:

```toml
user = "ocserv-panel"
group = "ocserv-panel"
```

To:

```toml
user = "root"
group = "root"
```

- [ ] **Step 3: Verify backend builds**

Run: `cd backend && go build ./...`

Expected: Clean build, exit 0.

- [ ] **Step 4: Verify tests still pass**

Run: `cd backend && go test ./...`

Expected: All packages PASS (no test depends on the user/group default value).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/config/config.go deploy/config.toml.example
git commit -m "config: change default service user/group to root"
```

---

### Task 5: Simplify Install Script

**Files:**
- Modify: `deploy/install.sh`

This is the largest change. The script needs these removals:

- [ ] **Step 1: Remove SERVICE_USER/SERVICE_GROUP variables and SUDOERS_FILE**

At lines 64-66, change:

```bash
SUDOERS_FILE="/etc/sudoers.d/ocserv-panel"
SERVICE_USER="ocserv-panel"
SERVICE_GROUP="ocserv-panel"
```

To:

```bash
# Service runs as root — no dedicated user needed
```

- [ ] **Step 2: Remove `do_create_user()` function**

Delete the entire function at lines 794-812:

```bash
do_create_user() {
    ...
}
```

- [ ] **Step 3: Remove `do_install_sudoers()` function**

Delete the entire function at lines 967-1022 (the function that generates and installs the sudoers file with detected binary paths).

- [ ] **Step 4: Remove `do_verify_cert_access()` function**

Delete the entire function at lines 1026-1200 (the function that checks if SERVICE_USER can read cert files and attempts to set up ACLs/ssl-cert group). Root can read all files.

- [ ] **Step 5: Simplify `do_install_systemd()` — remove hardening directives**

In the `do_install_systemd()` function (starts at line 1210), modify the generated systemd unit content. Remove `User=${SERVICE_USER}`, `Group=${SERVICE_GROUP}`, and all hardening directives from the heredoc. The generated unit should match Task 1's template but with variables for paths:

```bash
do_install_systemd() {
    local ro_paths="/etc/ocserv-panel /etc/ocserv /usr/share/ocserv-panel"
    local cert_dir
    cert_dir="$(dirname "${DETECTED_CERT_PATH}" 2>/dev/null)" || true
    if [[ -n "${cert_dir}" && "${cert_dir}" != "." ]]; then
        ro_paths="${ro_paths} ${cert_dir}"
    fi

    # Build optional TLS env lines
    local tls_env_lines=""
    if [[ -n "${DETECTED_CERT_PATH}" ]]; then
        tls_env_lines="Environment=\"OCSERV_PANEL_TLS_CERT=${DETECTED_CERT_PATH}\"
Environment=\"OCSERV_PANEL_TLS_KEY=${DETECTED_KEY_PATH}\"
Environment=\"OCSERV_PANEL_COOKIE_SECURE=true\""
    fi

    # Create data and log directories
    if ! mkdir -p "${DATA_DIR}" "${LOG_DIR}" "${METRICS_DIR}"; then
        fail "Cannot create data directories"
        return 1
    fi

    cat > "${SYSTEMD_UNIT}" <<EOF
[Unit]
Description=ocserv Panel - Web UI for OpenConnect VPN Administration
After=network-online.target ocserv.service
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${DATA_DIR}

ExecStart=${BINARY_DST}

Environment="OCSERV_PANEL_ADDR=0.0.0.0:${PANEL_PORT}"
Environment="OCSERV_PANEL_DB_DSN=file:${DATA_DIR}/ocserv_panel.db?_journal=WAL&_busy_timeout=5000"
Environment="OCSERV_PANEL_CONFIG_PATH=${CONFIG_FILE}"
Environment="OCSERV_PANEL_TSDB_PATH=${METRICS_DIR}"
Environment="OCSERV_PANEL_MIGRATIONS_DIR=${MIGRATIONS_DST}"
Environment="OCSERV_PANEL_STATIC_DIR=${STATIC_DST}"
${tls_env_lines}

Restart=on-failure
RestartSec=5s
TimeoutStartSec=30s
TimeoutStopSec=10s

PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

    if ! systemctl daemon-reload; then
        fail "systemctl daemon-reload failed"
        printf "       → Check: \${BOLD}systemctl status\${RESET}\n"
        return 1
    fi
    ok "Systemd unit installed: ${SYSTEMD_UNIT}"
}
```

- [ ] **Step 6: Simplify `do_generate_config()` — use root for user/group**

In the config generation heredoc, change:

```bash
user = "${SERVICE_USER}"
group = "${SERVICE_GROUP}"
```

To:

```bash
user = "root"
group = "root"
```

- [ ] **Step 7: Remove ownership/permission fixups**

Remove the line:
```bash
chown "${SERVICE_USER}:${SERVICE_GROUP}" "${DATA_DIR}" "${LOG_DIR}" "${METRICS_DIR}"
chmod 750 "${DATA_DIR}" "${LOG_DIR}" "${METRICS_DIR}"
```

And change config file permissions from:
```bash
chown "root:${SERVICE_GROUP}" "${CONFIG_FILE}"
```
To:
```bash
chown root:root "${CONFIG_FILE}"
chmod 644 "${CONFIG_FILE}"
```

- [ ] **Step 8: Remove sudoers/user steps from main execution flow**

In the `main` function (around lines 2283-2296), remove these steps:

```bash
    run_step "Create system user" do_create_user
    run_step "Install sudoers rules" do_install_sudoers
    run_step "Verify certificate access" do_verify_cert_access
```

The execution flow becomes:

```bash
    run_step "Install binary" do_install_binary
    run_step "Install static assets" do_install_static
    run_step "Install migrations" do_install_migrations
    run_step "Generate configuration" do_generate_config
    run_step "Install systemd unit" do_install_systemd
    run_step "Start service" do_start_service
```

- [ ] **Step 9: Remove visudo preflight check**

In the `preflight()` function, remove the visudo availability check (around lines 607-611).

- [ ] **Step 10: Simplify status checks**

Remove any status check functions that validate:
- Sudoers file syntax
- Sudoers file permissions (440)
- Service user certificate read access via `su -s /bin/sh "${SERVICE_USER}"`
- Database directory ownership checks against `${SERVICE_USER}:${SERVICE_GROUP}`

Replace database ownership checks with simpler directory-exists + writable checks (root always has write).

- [ ] **Step 11: Remove plan output references to sudoers/user**

In `show_plan()`, remove lines like:
```
echo "  ${n}. Create system user '${SERVICE_USER}'"; n=$((n + 1))
echo "  ${n}. Install sudoers → ${SUDOERS_FILE}"; n=$((n + 1))
echo "  ${n}. Verify certificate read access for ${SERVICE_USER}"; n=$((n + 1))
```

- [ ] **Step 12: Simplify troubleshooting messages**

In `do_start_service()`, change:
```bash
printf "         • TLS cert files not readable by '${SERVICE_USER}' user\n"
```
To:
```bash
printf "         • TLS cert files not found at configured path\n"
```

- [ ] **Step 13: Verify install script syntax**

Run: `bash -n deploy/install.sh`

Expected: Exit 0, no syntax errors.

- [ ] **Step 14: Commit**

```bash
git add deploy/install.sh
git commit -m "deploy: simplify install script for root execution

Remove: system user creation, sudoers generation, ACL/ssl-cert
setup, certificate access verification, ownership fixups.
Service runs as root — direct access to all system resources."
```

---

### Task 6: Final Verification

**Files:** None (read-only verification)

- [ ] **Step 1: Verify Go backend builds**

Run: `cd /opt/ocserv_panel/backend && go build ./...`

Expected: Clean build.

- [ ] **Step 2: Verify all Go tests pass**

Run: `cd /opt/ocserv_panel/backend && go test ./...`

Expected: All packages PASS.

- [ ] **Step 3: Verify install script syntax**

Run: `bash -n /opt/ocserv_panel/deploy/install.sh`

Expected: Exit 0.

- [ ] **Step 4: Verify systemd unit syntax**

Run: `systemd-analyze verify /opt/ocserv_panel/deploy/ocserv-panel.service 2>&1 || true`

Expected: No critical errors.

- [ ] **Step 5: Verify no references to old user remain**

Run: `grep -r "ocserv-panel" deploy/ --include="*.service" --include="*.sh" --include="*.toml" | grep -v "ocserv-panel.service\|ocserv-panel\.db\|ocserv-panel\b" | grep "SERVICE_USER\|SERVICE_GROUP\|sudoers" || echo "CLEAN"`

Expected: "CLEAN" — no stale references.

- [ ] **Step 6: Verify sudoers.d directory is gone**

Run: `test ! -d deploy/sudoers.d && echo "GONE" || echo "STILL EXISTS"`

Expected: "GONE".
