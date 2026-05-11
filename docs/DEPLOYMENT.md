# Deployment Guide

How to build the ocserv-panel release package on a development machine and install it on a production server running ocserv.

---

## Build Machine Prerequisites

| Dependency | Minimum Version | Purpose |
|------------|-----------------|---------|
| Go | 1.22+ | Backend compilation |
| Node.js | 18+ | Frontend build toolchain |
| pnpm | 8+ | Frontend package manager |
| GNU Make | any | Build orchestration |
| git | any | Version tagging |
| tar, gzip | any | Release packaging |

Install pnpm if not already available:

```bash
npm install -g pnpm
```

---

## Building the Release Package

### 1. Clone and enter the repository

```bash
git clone <repository-url> ocserv_panel
cd ocserv_panel
```

### 2. Tag a release version (optional)

The build uses `git describe` to derive the version string. If building from an untagged commit, the version will be the short commit hash (e.g., `abc1234-dirty`). For a proper release:

```bash
git tag -a v1.0.0 -m "Release 1.0.0"
```

Or override the version manually:

```bash
make release VERSION=1.0.0
```

### 3. Install frontend dependencies

```bash
make install-deps
```

This runs `pnpm install` in the `frontend/` directory. Only needed on first build or after dependency changes.

### 4. Build the release tarball

```bash
make release
```

This single command:

1. **Cleans** any previous build artifacts (`dist/`, old tarballs)
2. **Builds the Go backend** — static binary with no CGO dependencies, targeting `linux/amd64`
3. **Builds the SvelteKit frontend** — produces optimized HTML/JS/CSS with pre-compressed `.gz` and `.br` variants
4. **Packages everything** into a release tarball

On success, the output is:

```
ocserv-panel-<VERSION>-linux-amd64.tar.gz
```

### 5. Verify the tarball contents

```bash
tar tzf ocserv-panel-*-linux-amd64.tar.gz
```

Expected structure:

```
bin/ocserv-panel              # Static Go binary (~15-20 MB)
static/                       # SvelteKit build output
  index.html
  _app/                       # Hashed JS/CSS assets
  favicon.png
migrations/                   # SQLite schema migrations
  0001_initial.sql
  0002_settings_registry.sql
  0003_ocserv_binary_settings.sql
  0004_admin_user_extensions.sql
config.toml.example           # Reference configuration
sudoers.d/ocserv-panel        # Sudo rules for the service account
ocserv-panel.service          # systemd unit file
install.sh                    # Interactive installer script
VERSION                       # Version string file
```

---

## Individual Build Targets

For development or debugging, each phase can run independently:

```bash
# Backend only (produces dist/bin/ocserv-panel)
make build-backend

# Frontend only (produces dist/static/)
make build-frontend

# Run tests without building
make check

# Clean all artifacts
make clean
```

---

## Transferring to the Target Server

Copy the tarball to your production server using scp, rsync, or any file transfer method:

```bash
scp ocserv-panel-1.0.0-linux-amd64.tar.gz root@your-server:/tmp/
```

---

## Installing on the Target Server

### Prerequisites on the target

- systemd
- ocserv already installed and running
- Let's Encrypt certificates (or other TLS certificates)
- Root access for the installation

### Run the installer

```bash
# Extract the tarball
mkdir -p /tmp/ocserv-panel-install
tar xzf /tmp/ocserv-panel-1.0.0-linux-amd64.tar.gz -C /tmp/ocserv-panel-install

# Run the interactive installer
cd /tmp/ocserv-panel-install
sudo ./install.sh
```

The installer will:

1. Run preflight checks (root, systemd, ocserv detection, cert detection)
2. Show a numbered installation plan and ask for confirmation
3. Create the `ocserv-panel` system user
4. Install the binary, static files, migrations, config, sudoers, and systemd unit
5. Verify TLS certificate access for the service account
6. Enable and start the service
7. Run a health check to confirm the panel is operational

### Upgrading an existing installation

The installer automatically detects an existing installation and enters upgrade mode:

- Backs up the database and config before changes
- Preserves your `config.toml` (does not overwrite)
- Replaces binary, static files, and migrations
- Restarts the service

Simply run the same install procedure with a newer tarball.

### Uninstalling

```bash
cd /tmp/ocserv-panel-install
sudo ./install.sh --uninstall
```

The uninstaller will prompt whether to keep or remove data and configuration.

---

## Post-Installation

After a successful install, the panel is accessible at:

```
https://<your-domain>:8443
```

The first visit presents a setup page to create the initial admin account.

### Useful commands

```bash
# Service status
systemctl status ocserv-panel

# Live logs
journalctl -u ocserv-panel -f

# Version check
/usr/local/bin/ocserv-panel --version
```

---

## Environment Variables

The systemd unit sets these via `Environment=` directives. Override them in a drop-in file (`systemctl edit ocserv-panel`) if needed:

| Variable | Default | Description |
|----------|---------|-------------|
| `OCSERV_PANEL_ADDR` | `0.0.0.0:8443` | Listen address and port |
| `OCSERV_PANEL_DB_DSN` | `file:/var/lib/ocserv-panel/ocserv_panel.db?...` | SQLite database DSN |
| `OCSERV_PANEL_CONFIG_PATH` | `/etc/ocserv-panel/config.toml` | Panel configuration file |
| `OCSERV_PANEL_TSDB_PATH` | `/var/lib/ocserv-panel/metrics` | TSDB metrics storage |
| `OCSERV_PANEL_MIGRATIONS_DIR` | `/usr/share/ocserv-panel/migrations` | SQL migration files |
| `OCSERV_PANEL_STATIC_DIR` | `/usr/share/ocserv-panel/static` | Frontend static files |
| `OCSERV_PANEL_TLS_CERT` | (from config.toml) | TLS certificate path |
| `OCSERV_PANEL_TLS_KEY` | (from config.toml) | TLS private key path |
| `OCSERV_PANEL_COOKIE_SECURE` | `true` | Set `Secure` flag on session cookies |

---

## Filesystem Layout

After installation, files are placed at:

| Path | Owner | Purpose |
|------|-------|---------|
| `/usr/local/bin/ocserv-panel` | root:root | Go binary |
| `/usr/share/ocserv-panel/static/` | root:root | Frontend assets |
| `/usr/share/ocserv-panel/migrations/` | root:root | SQL migrations |
| `/etc/ocserv-panel/config.toml` | root:ocserv-panel | Configuration |
| `/var/lib/ocserv-panel/` | ocserv-panel:ocserv-panel | Database + TSDB |
| `/var/log/ocserv-panel/` | ocserv-panel:ocserv-panel | Log files |
| `/etc/systemd/system/ocserv-panel.service` | root:root | systemd unit |
| `/etc/sudoers.d/ocserv-panel` | root:root | Sudo rules |

---

## Troubleshooting

### Build fails with "pnpm: command not found"

Install pnpm: `npm install -g pnpm`

### Build fails with CGO errors

Ensure `CGO_ENABLED=0` is set (the Makefile handles this). The binary must be statically linked with no C dependencies.

### Frontend build fails with missing dependencies

Run `make install-deps` or `cd frontend && pnpm install --frozen-lockfile`.

### Service fails to start — permission denied on TLS certs

The `ocserv-panel` user needs read access to the certificate files. Grant via ACL:

```bash
setfacl -m u:ocserv-panel:rx /etc/letsencrypt/live
setfacl -m u:ocserv-panel:rx /etc/letsencrypt/archive
setfacl -m u:ocserv-panel:r /etc/letsencrypt/live/your-domain/fullchain.pem
setfacl -m u:ocserv-panel:r /etc/letsencrypt/live/your-domain/privkey.pem
```

### Service fails to start — address already in use

Another process is using port 8443. Check with `ss -tlnp | grep 8443` and adjust `OCSERV_PANEL_ADDR` if needed.

### Health check fails after install

Check logs: `journalctl -u ocserv-panel --no-pager -n 50`

Common causes: wrong cert paths in config, database directory permissions, missing migrations.
