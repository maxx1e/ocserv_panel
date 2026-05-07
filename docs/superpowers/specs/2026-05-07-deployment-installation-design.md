# Deployment & Installation Design

**Date:** 2026-05-07
**Status:** Draft
**Scope:** Build pipeline, release packaging, interactive installer, TLS, static file serving, systemd, permissions

---

## Context

The ocserv-panel needs to be deployed on a production server already running ocserv. The deployment must:

- Have zero dependencies on HAProxy, Nginx, or Node.js runtime
- Provide good performance and response times
- Use the existing Let's Encrypt certificates for HTTPS
- Run on port 8443 (ocserv owns port 443)
- Be a two-step process: build on dev machine, install on target
- Support both fresh installs and upgrades
- Provide an interactive, human-friendly install experience

## Architecture Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Frontend adapter | `adapter-static` (SPA) | No Node.js runtime needed on server |
| Static file serving | Go serves from filesystem path | Allows independent UI updates without binary rebuild |
| TLS termination | Go `ListenAndServeTLS` directly | Zero-dependency, no reverse proxy |
| Cert reloading | `tls.Config.GetCertificate` with file watcher | Supports certbot auto-renewal |
| Packaging | Release tarball (.tar.gz) | Single artifact to transfer and extract |
| Installation | Interactive shell script | Human-friendly, handles install + upgrade |
| Service account | Dedicated `ocserv-panel` system user | Least privilege, matches systemd hardening |
| Cert access | Verified during install; user grants read access | Script checks and advises, doesn't force a method |

---

## 1. Build Pipeline

### Makefile Targets

```makefile
# Build Go backend (static binary, no CGO)
build-backend:
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=$(VERSION)" \
        -o dist/bin/ocserv-panel ./cmd/ocserv-panel

# Build SvelteKit frontend (adapter-static, SPA mode)
build-frontend:
    cd ../frontend && pnpm install --frozen-lockfile && pnpm build
    cp -r ../frontend/build dist/static

# Package release tarball
release: clean build-backend build-frontend
    mkdir -p dist/migrations dist/sudoers.d
    cp -r ../migrations/*.sql dist/migrations/
    cp ../deploy/sudoers.d/ocserv-panel dist/sudoers.d/
    cp ../deploy/ocserv-panel.service dist/
    cp ../deploy/install.sh dist/
    cp ../deploy/config.toml.example dist/
    echo "$(VERSION)" > dist/VERSION
    tar -czf ocserv-panel-$(VERSION)-linux-amd64.tar.gz -C dist .

clean:
    rm -rf dist/
```

### SvelteKit Adapter Change

Switch `frontend/svelte.config.js` from `adapter-auto` to `adapter-static`:

```javascript
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({
      pages: 'build',
      assets: 'build',
      fallback: 'index.html',  // SPA fallback for client-side routing
      precompress: true         // Generate .gz and .br files
    })
  }
};

export default config;
```

---

## 2. Go Backend Changes

### 2.1 Static File Server

New environment variable / config key:

```
OCSERV_PANEL_STATIC_DIR=/usr/share/ocserv-panel/static
```

Config struct addition:

```go
type InstallationConfig struct {
    // ... existing fields ...
    StaticDir string `toml:"static_dir"`
}
```

Default: `/usr/share/ocserv-panel/static`

**Serving logic:**

```go
// In NewAppRouter, after all /api/* routes:
staticDir := cfg.Installation.StaticDir
if staticDir != "" {
    mux.Handle("/", spaFileServer(staticDir))
}
```

SPA file server behavior:
1. If file exists in `staticDir` at the request path, serve it
2. For `/_app/` paths (Svelte hashed assets): `Cache-Control: public, max-age=31536000, immutable`
3. For `index.html`: `Cache-Control: no-cache`
4. If file doesn't exist and path doesn't start with `/api/`: serve `index.html` (SPA fallback)
5. Pre-compressed: serve `.br` or `.gz` if `Accept-Encoding` matches and file exists

### 2.2 TLS Support

New environment variables / config keys:

```
OCSERV_PANEL_TLS_CERT=/etc/letsencrypt/live/light-mirror.ru/fullchain.pem
OCSERV_PANEL_TLS_KEY=/etc/letsencrypt/live/light-mirror.ru/privkey.pem
```

Config struct additions to `SecurityConfig`:

```go
type SecurityConfig struct {
    // ... existing fields ...
    TLSCertPath string `toml:"tls_cert_path"`  // already exists
    TLSKeyPath  string `toml:"tls_key_path"`   // already exists
}
```

**TLS logic in main.go:**

```go
if cfg.Security.TLSCertPath != "" && cfg.Security.TLSKeyPath != "" {
    tlsConfig := &tls.Config{
        GetCertificate: newCertReloader(cfg.Security.TLSCertPath, cfg.Security.TLSKeyPath),
        MinVersion:     tls.VersionTLS12,
    }
    httpServer.TLSConfig = tlsConfig
    err = httpServer.ListenAndServeTLS("", "")  // certs from GetCertificate
} else {
    err = httpServer.ListenAndServe()
}
```

**Certificate reloader:**

A goroutine-safe cert reloader that:
- Loads cert pair on first call
- Watches file mtime every 60 seconds
- Reloads when files change (certbot renewal)
- Logs reload events

### 2.3 Environment Variable Summary

| Variable | Default | Description |
|----------|---------|-------------|
| `OCSERV_PANEL_ADDR` | `127.0.0.1:8080` | Listen address (change to `0.0.0.0:8443`) |
| `OCSERV_PANEL_STATIC_DIR` | `/usr/share/ocserv-panel/static` | Frontend static files |
| `OCSERV_PANEL_TLS_CERT` | (empty) | TLS certificate path |
| `OCSERV_PANEL_TLS_KEY` | (empty) | TLS private key path |
| `OCSERV_PANEL_DB_DSN` | `file:ocserv_panel.db` | SQLite DSN |
| `OCSERV_PANEL_MIGRATIONS_DIR` | (auto-detect) | Migrations directory |
| `OCSERV_PANEL_CONFIG_PATH` | `/etc/ocserv-panel/config.toml` | Panel config file |
| `OCSERV_PANEL_COOKIE_SECURE` | `true` | Secure cookie flag |
| `OCSERV_PANEL_TSDB_PATH` | `/var/lib/ocserv-panel/metrics` | TSDB metrics path |
| `OCSERV_PANEL_VPN_IFACE` | (auto-detect) | VPN network interface |
| `OCSERV_PANEL_NET_IFACE` | (auto-detect) | System network interface |

---

## 3. Release Tarball Contents

```
ocserv-panel-1.0.0-linux-amd64.tar.gz
├── bin/ocserv-panel                    # Static Go binary
├── static/                             # SvelteKit build (HTML/JS/CSS)
│   ├── index.html
│   ├── _app/                           # Hashed assets
│   └── favicon.png
├── migrations/                         # SQLite migrations
│   ├── 0001_initial.sql
│   ├── 0002_settings_registry.sql
│   ├── 0003_ocserv_binary_settings.sql
│   └── 0004_admin_user_extensions.sql
├── config.toml.example                 # Reference configuration
├── sudoers.d/ocserv-panel              # Sudoers rules
├── ocserv-panel.service                # systemd unit template
├── install.sh                          # Interactive installer
└── VERSION                             # Version string
```

---

## 4. Filesystem Layout on Target

| Path | Owner | Mode | Purpose |
|------|-------|------|---------|
| `/usr/local/bin/ocserv-panel` | root:root | 755 | Go binary |
| `/usr/share/ocserv-panel/static/` | root:root | 755 (dir), 644 (files) | Frontend assets |
| `/usr/share/ocserv-panel/migrations/` | root:root | 755 (dir), 644 (files) | SQL migrations |
| `/etc/ocserv-panel/` | root:ocserv-panel | 750 | Config directory |
| `/etc/ocserv-panel/config.toml` | root:ocserv-panel | 640 | Panel configuration |
| `/var/lib/ocserv-panel/` | ocserv-panel:ocserv-panel | 750 | SQLite DB + TSDB |
| `/var/lib/ocserv-panel/ocserv_panel.db` | ocserv-panel:ocserv-panel | 640 | SQLite database |
| `/var/lib/ocserv-panel/metrics/` | ocserv-panel:ocserv-panel | 750 | TSDB data |
| `/var/log/ocserv-panel/` | ocserv-panel:ocserv-panel | 750 | Log files |
| `/etc/systemd/system/ocserv-panel.service` | root:root | 644 | systemd unit |
| `/etc/sudoers.d/ocserv-panel` | root:root | 440 | Sudo rules |

---

## 5. Service Account & Permissions

### System User

```bash
useradd --system --shell /usr/sbin/nologin --no-create-home \
    --comment "ocserv-panel web UI" ocserv-panel
```

### Required Access

| Resource | Access | Method |
|----------|--------|--------|
| Let's Encrypt certs | Read | ACL or group (verified by installer) |
| `/etc/ocserv/ocserv.conf` | Read | Group `ocserv-panel` added to read ACL |
| `/etc/ocserv/ocpasswd` | Read/Write | Via `ocpasswd` binary through sudoers |
| `/var/run/ocserv-socket` | Read | Socket permissions or ACL |
| systemctl ocserv | Execute | Via sudoers |
| occtl commands | Execute | Via sudoers (if socket not directly accessible) |

### Sudoers Rules

```
# Service management
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl start ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl stop ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl restart ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl reload ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl status ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl is-active ocserv
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/systemctl is-enabled ocserv

# Session management
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/occtl disconnect id *
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/occtl show users
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/occtl show user *
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/occtl show status
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/occtl reload

# Password management
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/ocpasswd -c /etc/ocserv/ocpasswd *
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/ocpasswd -c /etc/ocserv/ocpasswd -d *
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/ocpasswd -c /etc/ocserv/ocpasswd -l *
ocserv-panel ALL=(root) NOPASSWD: /usr/bin/ocpasswd -c /etc/ocserv/ocpasswd -u *
```

---

## 6. Systemd Unit

```ini
[Unit]
Description=ocserv Panel - Web UI for OpenConnect VPN Administration
Documentation=https://github.com/your-org/ocserv_panel
After=network-online.target ocserv.service
Wants=network-online.target ocserv.service

[Service]
Type=simple
User=ocserv-panel
Group=ocserv-panel
WorkingDirectory=/var/lib/ocserv-panel

# Binary
ExecStart=/usr/local/bin/ocserv-panel

# Environment
Environment="OCSERV_PANEL_ADDR=0.0.0.0:8443"
Environment="OCSERV_PANEL_DB_DSN=file:/var/lib/ocserv-panel/ocserv_panel.db?_journal=WAL&_busy_timeout=5000"
Environment="OCSERV_PANEL_CONFIG_PATH=/etc/ocserv-panel/config.toml"
Environment="OCSERV_PANEL_TSDB_PATH=/var/lib/ocserv-panel/metrics"
Environment="OCSERV_PANEL_MIGRATIONS_DIR=/usr/share/ocserv-panel/migrations"
Environment="OCSERV_PANEL_STATIC_DIR=/usr/share/ocserv-panel/static"
Environment="OCSERV_PANEL_TLS_CERT=/etc/letsencrypt/live/light-mirror.ru/fullchain.pem"
Environment="OCSERV_PANEL_TLS_KEY=/etc/letsencrypt/live/light-mirror.ru/privkey.pem"
Environment="OCSERV_PANEL_COOKIE_SECURE=true"

# Restart policy
Restart=on-failure
RestartSec=5s
TimeoutStartSec=30s
TimeoutStopSec=10s

# Filesystem hardening
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/ocserv-panel /var/log/ocserv-panel
ReadOnlyPaths=/etc/ocserv-panel /etc/ocserv /etc/letsencrypt /usr/share/ocserv-panel

# Security hardening
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=true
LockPersonality=true

[Install]
WantedBy=multi-user.target
```

Note: `CAP_NET_BIND_SERVICE` is included in case port 443 is ever needed, but port 8443 doesn't require it. The capability can be removed if not binding to a privileged port.

---

## 7. Interactive Install Script

### Design Principles

- Colored output: green checkmarks, red failures, yellow warnings
- Each step is announced before execution
- Failures stop the script with clear error and remediation advice
- Idempotent: safe to re-run (skips already-done steps)
- Upgrade-aware: detects existing installation via VERSION file
- Non-destructive: backs up config and database before upgrade
- Validates every prerequisite before making changes

### Script Flow

```
install.sh [--upgrade] [--uninstall]
```

#### Phase 1: Preflight Checks

```
╔══════════════════════════════════════════════════════╗
║  ocserv-panel installer v1.0.0                       ║
╚══════════════════════════════════════════════════════╝

Preflight checks:
  [✓] Running as root
  [✓] systemd available
  [✓] Required files present (bin/ocserv-panel, static/, migrations/)
  [✓] ocserv detected: /usr/sbin/ocserv (active, running)
  [✓] occtl: /usr/bin/occtl
  [✓] ocpasswd: /usr/bin/ocpasswd
  [✓] ocserv config: /etc/ocserv/ocserv.conf
  [✓] ocserv password file: /etc/ocserv/ocpasswd
  [✓] ocserv socket: /var/run/ocserv-socket
  [✓] Let's Encrypt certs: /etc/letsencrypt/live/light-mirror.ru/
  [!] Existing installation detected: v0.9.0 → upgrading to v1.0.0
```

#### Phase 2: Confirmation

```
Installation plan:
  1. Stop existing service (upgrade only)
  2. Backup database and config (upgrade only)
  3. Create/verify service account: ocserv-panel
  4. Install binary: /usr/local/bin/ocserv-panel
  5. Install static files: /usr/share/ocserv-panel/static/
  6. Install migrations: /usr/share/ocserv-panel/migrations/
  7. Generate/update config: /etc/ocserv-panel/config.toml
  8. Install sudoers rules: /etc/sudoers.d/ocserv-panel
  9. Verify cert access for ocserv-panel user
  10. Install systemd unit: /etc/systemd/system/ocserv-panel.service
  11. Enable and start service
  12. Verify health endpoint

Proceed? [Y/n]
```

#### Phase 3: Execution

Each step prints status as it executes. On failure, prints remediation steps and exits.

#### Phase 4: Verification

```
Post-install verification:
  [✓] Service active: ocserv-panel.service
  [✓] Listening on 0.0.0.0:8443
  [✓] Health check: https://localhost:8443/api/health → 200 OK
  [✓] TLS handshake successful

════════════════════════════════════════════════════════
  Installation complete!

  Access the panel at: https://light-mirror.ru:8443
  First login: create admin account at setup page

  Useful commands:
    systemctl status ocserv-panel
    journalctl -u ocserv-panel -f
    /usr/local/bin/ocserv-panel --version
════════════════════════════════════════════════════════
```

### Upgrade Behavior

1. Detect existing version from `/usr/share/ocserv-panel/VERSION`
2. Stop service
3. Backup: `tar -czf /var/lib/ocserv-panel/backup-<date>.tar.gz /var/lib/ocserv-panel/ocserv_panel.db /etc/ocserv-panel/config.toml`
4. Replace binary, static files, migrations
5. Preserve config.toml (don't overwrite; show diff if template changed)
6. Run migrations (automatic on service start)
7. Start service
8. Verify health

### Uninstall Behavior (--uninstall)

1. Stop and disable service
2. Remove systemd unit
3. Remove sudoers file
4. Remove binary, static files, migrations
5. Ask: keep or remove data (`/var/lib/ocserv-panel`)
6. Ask: keep or remove config (`/etc/ocserv-panel`)
7. Ask: remove system user

---

## 8. Config.toml Generation

The install script auto-detects system paths and generates config:

```toml
[general]
app_name = "ocserv Panel"
timezone = "UTC"

[installation]
config_path = "/etc/ocserv-panel/config.toml"
data_dir = "/var/lib/ocserv-panel"
log_dir = "/var/log/ocserv-panel"
ocserv_config_path = "/etc/ocserv/ocserv.conf"
ocserv_socket_path = "/var/run/ocserv-socket"
ocserv_bin_path = "/usr/sbin/ocserv"
occtl_bin_path = "/usr/bin/occtl"
ocpasswd_bin_path = "/usr/bin/ocpasswd"
ocpasswd_file_path = "/etc/ocserv/ocpasswd"
cert_dir = "/etc/letsencrypt/live/light-mirror.ru"
panel_log_path = "/var/log/ocserv-panel/panel.log"
static_dir = "/usr/share/ocserv-panel/static"

[service]
name = "ocserv-panel"
user = "ocserv-panel"
group = "ocserv-panel"
start_on_boot = true
systemd_unit_name = "ocserv"

[web]
host = "0.0.0.0"
port = 8443

[security]
tls_cert_path = "/etc/letsencrypt/live/light-mirror.ru/fullchain.pem"
tls_key_path = "/etc/letsencrypt/live/light-mirror.ru/privkey.pem"
csrf_enabled = true
security_headers = true

[database]
dsn = "file:/var/lib/ocserv-panel/ocserv_panel.db?_journal=WAL&_busy_timeout=5000"

[monitoring]
health_endpoint = true
dashboard_refresh_interval = 15
```

---

## 9. Dependencies & Prerequisites

### Build Machine (dev)

| Dependency | Version | Purpose |
|------------|---------|---------|
| Go | 1.22+ | Backend compilation |
| Node.js | 18+ | Frontend build |
| pnpm | 8+ | Frontend package manager |
| make | any | Build orchestration |
| tar, gzip | any | Release packaging |

### Target Server (production)

| Dependency | Required | Purpose |
|------------|----------|---------|
| systemd | Yes | Service management |
| ocserv | Yes | VPN server (already running) |
| Let's Encrypt certs | Yes | TLS for panel |
| curl or wget | Optional | Health check in installer |
| setfacl (acl package) | Optional | Cert access if ACL method used |

**No runtime dependencies:** No Node.js, no reverse proxy, no database server, no external services.

---

## 10. Security Considerations

| Concern | Mitigation |
|---------|-----------|
| TLS cert file access | Installer verifies read access; ACL or group method |
| Private key exposure | Key files read-only by panel user; not stored in config |
| Cert renewal | `GetCertificate` callback reloads on file change |
| SQLite file access | Owned by panel user, mode 640, in protected directory |
| Sudo escalation | Strict allowlist of exact commands in sudoers |
| Network exposure | Panel on 8443 only; firewall rules recommended |
| Service hardening | systemd sandboxing (ProtectSystem, PrivateTmp, etc.) |
| Upgrade safety | Backup DB + config before any upgrade |
| Cookie security | `Secure=true` when TLS is active |

---

## 11. Firewall Recommendation

The installer should advise (not enforce) opening port 8443:

```bash
# For iptables:
iptables -A INPUT -p tcp --dport 8443 -j ACCEPT

# For firewalld:
firewall-cmd --permanent --add-port=8443/tcp && firewall-cmd --reload

# For ufw:
ufw allow 8443/tcp
```

---

## 12. Implementation Scope

### Code Changes Required

1. **Frontend:** Switch to `adapter-static`, add SPA fallback config
2. **Backend:** Add static file server with SPA fallback
3. **Backend:** Add TLS support with cert reloading
4. **Backend:** Add `OCSERV_PANEL_STATIC_DIR` and TLS env var handling
5. **Config:** Add `static_dir` to `InstallationConfig`
6. **Deploy:** Write Makefile for build pipeline
7. **Deploy:** Write interactive install.sh
8. **Deploy:** Update systemd unit template
9. **Deploy:** Update sudoers with full command set
10. **Deploy:** Write config.toml.example

### Files Created/Modified

| File | Action |
|------|--------|
| `frontend/svelte.config.js` | Modify: adapter-static |
| `frontend/package.json` | Add: @sveltejs/adapter-static |
| `backend/internal/server/static.go` | Create: SPA file server |
| `backend/internal/server/tls.go` | Create: cert reloader |
| `backend/cmd/ocserv-panel/main.go` | Modify: TLS + static dir |
| `backend/internal/config/config.go` | Modify: add StaticDir |
| `backend/Makefile` | Create: build targets |
| `deploy/install.sh` | Create: interactive installer |
| `deploy/ocserv-panel.service` | Update: TLS + static env |
| `deploy/sudoers.d/ocserv-panel` | Update: full command set |
| `deploy/config.toml.example` | Create: reference config |
