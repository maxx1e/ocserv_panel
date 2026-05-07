# Design: Run ocserv-panel Service as Root

**Date:** 2026-05-07  
**Status:** Draft  
**Goal:** Simplify the privilege model by running the service as root, eliminating sudoers, dedicated user/group, ACL setup, and systemd hardening overhead.

---

## Motivation

The current deployment model creates a dedicated `ocserv-panel` system user with 14 explicit sudoers rules, ACL-based certificate access, and 12+ systemd security directives. For a personal-use, single-node admin panel running on a private VPN host, this adds significant installation complexity and is the primary source of "permission denied" issues during setup and operation.

Running as root eliminates:
- Sudoers file generation and maintenance
- User/group creation and ownership management
- ACL configuration for certificate access
- Permission errors when accessing ocserv socket, config, or password files
- Systemd hardening directives that conflict with root execution

## Scope

### In Scope

1. Systemd service unit — run as root with no hardening
2. Delete sudoers file entirely
3. Simplify install script — remove user creation, sudoers, ACL, ownership logic
4. Update config defaults to reflect root execution
5. Update Makefile packaging to exclude sudoers

### Out of Scope

- Application-level RBAC (unchanged — web dashboard users still have roles)
- Audit logging (unchanged)
- Input validation (unchanged)
- Config backup/diff/apply logic (unchanged)
- Frontend (unchanged)
- API contracts (unchanged)
- Database schema (unchanged)
- Go application code (no changes — CommandRunner calls binaries directly)

## Design

### 1. Systemd Unit (`deploy/ocserv-panel.service`)

**Before:**
```ini
User=ocserv-panel
Group=ocserv-panel
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/ocserv-panel /var/log/ocserv-panel
ReadOnlyPaths=/etc/ocserv-panel /etc/ocserv /etc/letsencrypt /usr/share/ocserv-panel
CapabilityBoundingSet=
NoNewPrivileges=true
PrivateDevices=true
MemoryDenyWriteExecute=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=true
LockPersonality=true
```

**After:**
```ini
# Runs as root — no privilege restrictions needed for personal admin panel
```

Remove all: `User=`, `Group=`, `ProtectSystem=`, `ProtectHome=`, `ReadWritePaths=`, `ReadOnlyPaths=`, `CapabilityBoundingSet=`, `NoNewPrivileges=`, `PrivateDevices=`, `MemoryDenyWriteExecute=`, `ProtectKernelTunables=`, `ProtectKernelModules=`, `ProtectControlGroups=`, `RestrictAddressFamilies=`, `RestrictNamespaces=`, `LockPersonality=`.

Keep: `Type=simple`, `ExecStart=`, `Environment=`, `Restart=`, `RestartSec=`, `TimeoutStartSec=`, `TimeoutStopSec=`, `PrivateTmp=true`.

Rationale for keeping `PrivateTmp=true`: prevents temp file attacks from other processes; zero-cost and doesn't conflict with root.

### 2. Sudoers File (`deploy/sudoers.d/ocserv-panel`)

**Action:** Delete entirely. The file, the install logic that generates it, and the status-check logic that validates it.

### 3. Install Script (`deploy/install.sh`)

Remove these sections/functions:
- `do_create_user()` — no dedicated user needed
- `do_install_sudoers()` — no sudoers needed  
- `do_setup_cert_access()` — root already has read access to all paths
- `do_fix_ownership()` — directories will be owned by root
- Status check for sudoers file syntax
- Status check for certificate read permission via test user

Keep:
- Binary installation
- Config file installation (now owned by `root:root` with 644)
- Directory creation (now owned by `root:root`)
- Systemd unit installation and enablement
- Database migration
- Status reporting (simplified)

### 4. Configuration Defaults (`backend/internal/config/config.go`)

Change defaults:
```go
// Before
User:  "ocserv-panel",
Group: "ocserv-panel",

// After
User:  "root",
Group: "root",
```

### 5. Config Example (`deploy/config.toml.example`)

```toml
[service]
name = "ocserv-panel"
user = "root"
group = "root"
start_on_boot = true
systemd_unit_name = "ocserv"
```

### 6. Makefile (`deploy/Makefile`)

Remove `sudoers.d/ocserv-panel` from the release tarball file list.

## Impact on Go Application Code

**None.** The Go code never invokes `sudo`. It calls `systemctl`, `occtl`, and `ocpasswd` directly through the `CommandRunner` interface. The sudoers file was the external mechanism that allowed these calls to succeed when running as a non-root user. Running as root means they succeed without sudo.

The `CommandRunner` interface, `SystemdManager`, `OcctlAdapter`, `OcpasswdAdapter`, and `ReadinessChecker` all continue to work unchanged.

## Impact on Tests

**None.** All tests use `FakeCommandRunner` and never touch real system binaries or permissions. No test changes needed.

## Migration Path

For existing installations:
1. Stop service
2. Install new binary + service unit
3. Remove sudoers file: `rm /etc/sudoers.d/ocserv-panel`
4. Optionally remove the dedicated user: `userdel ocserv-panel`
5. Start service

The install script's `upgrade` mode should handle steps 2-3 automatically.

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| Panel compromise gives full root | Acceptable for personal-use, single-node. App-level auth + network restriction (VPN host) are the boundaries. |
| Accidental file overwrites | Same as before — config backup/validate/apply flow unchanged |
| Service starts as root on port 8443 | Already runs behind optional HAProxy or direct HTTPS. No privilege ports involved. |

## Verification

- Build succeeds
- All existing tests pass (no Go code changes)
- Systemd unit loads without errors
- Service starts and can reach ocserv socket, read certs, manage users
- Install script runs cleanly on fresh system
