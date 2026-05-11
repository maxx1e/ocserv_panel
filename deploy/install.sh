#!/usr/bin/env bash
# install.sh — ocserv-panel interactive installer
#
# Usage:
#   sudo ./install.sh              # fresh install or upgrade
#   sudo ./install.sh --status     # check current installation health
#   sudo ./install.sh --fix        # check health and auto-fix common issues
#   sudo ./install.sh --healthcheck # quick pass/fail endpoint check (for monitoring)
#   sudo ./install.sh --uninstall  # remove installation
#   sudo ./install.sh --help       # show usage
#
# Exit codes:
#   0 — success / healthy
#   1 — errors found / health check failed
#   2 — warnings only (--status)
#
# Assumptions documented:
#   Binary target   : /usr/local/bin/ocserv-panel
#   Static assets   : /usr/share/ocserv-panel/static/
#   Migrations      : /usr/share/ocserv-panel/migrations/
#   Config file     : /etc/ocserv-panel/config.toml
#   Data / SQLite   : /var/lib/ocserv-panel/
#   Log dir         : /var/log/ocserv-panel/
#   Metrics (tsdb)  : /var/lib/ocserv-panel/metrics/
#   systemd unit    : /etc/systemd/system/ocserv-panel.service
#   Health endpoint : http(s)://127.0.0.1:${PANEL_PORT}/api/health

# ─── Error handling ──────────────────────────────────────────────────────────
# We do NOT use "set -e" because it causes silent exits on benign failures
# (e.g. grep not matching, test returning false, arithmetic returning 0).
# Instead, every critical command is wrapped with explicit error checks.
set -uo pipefail

# Global error trap — catches unexpected failures and reports the line number
trap '__err_handler $LINENO "$BASH_COMMAND"' ERR

__err_handler() {
    local line="$1"
    local cmd="$2"
    # Only report truly unexpected errors — skip intentional failures
    printf '\033[0;31m[error]\033[0m Unexpected failure at line %d: %s\n' "${line}" "${cmd}" >&2
    printf '\033[0;31m[error]\033[0m The installer encountered an unexpected error and cannot continue.\n' >&2
    printf '\033[0;31m[error]\033[0m Re-run with "bash -x %s" for detailed trace output.\n' "${BASH_SOURCE[0]}" >&2
    exit 1
}

# ─── Script location ─────────────────────────────────────────────────────────

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ─── Path constants ───────────────────────────────────────────────────────────

BINARY_SRC="${SCRIPT_DIR}/bin/ocserv-panel"
STATIC_SRC="${SCRIPT_DIR}/static"
MIGRATIONS_SRC="${SCRIPT_DIR}/migrations"
VERSION_FILE="${SCRIPT_DIR}/VERSION"

BINARY_DST="/usr/local/bin/ocserv-panel"
SHARE_DIR="/usr/share/ocserv-panel"
STATIC_DST="${SHARE_DIR}/static"
MIGRATIONS_DST="${SHARE_DIR}/migrations"
CONFIG_DIR="/etc/ocserv-panel"
CONFIG_FILE="${CONFIG_DIR}/config.toml"
DATA_DIR="/var/lib/ocserv-panel"
LOG_DIR="/var/log/ocserv-panel"
METRICS_DIR="${DATA_DIR}/metrics"
SYSTEMD_UNIT="/etc/systemd/system/ocserv-panel.service"
# Service runs as root — no dedicated user or sudoers needed
PANEL_PORT="8443"

# ─── Fix mode flag ───────────────────────────────────────────────────────────

FIX_MODE="false"
FIXES_APPLIED=0
FIXES_FAILED=0

# ─── Port from config helper ─────────────────────────────────────────────────
# Reads the configured port from config.toml (if present) to override the
# default PANEL_PORT. Called before status/healthcheck so we hit the right port.

read_port_from_config() {
    if [[ -f "${CONFIG_FILE}" ]]; then
        local p
        p="$(grep -E '^\s*port\s*=' "${CONFIG_FILE}" | head -1 | sed 's/.*=\s*//' | tr -d ' ')" || true
        if [[ -n "${p}" && "${p}" =~ ^[0-9]+$ ]]; then
            PANEL_PORT="${p}"
        fi
    fi
}

# ─── Detected values (populated by detect_ocserv) ────────────────────────────

DETECTED_OCSERV_BIN=""
DETECTED_OCCTL_BIN=""
DETECTED_OCPASSWD_BIN=""
DETECTED_OCSERV_CONF=""
DETECTED_OCSERV_SOCKET=""
DETECTED_OCPASSWD_FILE=""
DETECTED_CERT_PATH=""
DETECTED_KEY_PATH=""
DETECTED_CERT_DIR=""
EXISTING_VERSION=""

# ─── Colour helpers ───────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
RESET='\033[0m'

ok()   { printf "${GREEN}  [✓]${RESET} %s\n" "$*"; }
fail() { printf "${RED}  [✗]${RESET} %s\n" "$*"; }
warn() { printf "${YELLOW}  [!]${RESET} %s\n" "$*"; }
info() { printf "${BLUE}  [→]${RESET} %s\n" "$*"; }
bold() { printf "${BOLD}%s${RESET}\n" "$*"; }

die() {
    echo ""
    printf "${RED}┌─────────────────────────────────────────────────┐${RESET}\n"
    printf "${RED}│ ERROR: %-40s │${RESET}\n" "$*"
    printf "${RED}└─────────────────────────────────────────────────┘${RESET}\n"
    echo ""
    printf "  Troubleshooting:\n"
    printf "    • Re-run with: ${BOLD}bash -x %s${RESET}\n" "$0"
    printf "    • Check system logs: ${BOLD}journalctl -xe${RESET}\n"
    echo ""
    exit 1
}

# ─── Step execution wrapper ──────────────────────────────────────────────────
# Runs a function and reports success/failure clearly.
# Usage: run_step "Description" function_name [args...]

run_step() {
    local description="$1"
    shift
    info "${description}..."
    if "$@"; then
        ok "${description}"
    else
        local rc=$?
        fail "${description}"
        die "Step failed: ${description} (exit code ${rc})"
    fi
}

# ─── Fix helper ──────────────────────────────────────────────────────────────
# Attempts a fix only when FIX_MODE is enabled. Returns 0 if fix was applied
# successfully, 1 otherwise (including when FIX_MODE is off).
# Usage: try_fix "Description" command [args...]

try_fix() {
    local description="$1"
    shift
    if [[ "${FIX_MODE}" != "true" ]]; then
        return 1
    fi
    info "Fixing: ${description}..."
    if "$@"; then
        ok "Fixed: ${description}"
        FIXES_APPLIED=$((FIXES_APPLIED + 1))
        return 0
    else
        fail "Could not fix: ${description}"
        FIXES_FAILED=$((FIXES_FAILED + 1))
        return 1
    fi
}

# ─── Usage ────────────────────────────────────────────────────────────────────

usage() {
    cat <<EOF
ocserv-panel installer

Usage:
  sudo $0              # fresh install or upgrade
  sudo $0 --status     # check current installation health
  sudo $0 --fix        # check health and auto-fix common issues
  sudo $0 --healthcheck # quick pass/fail endpoint check (for monitoring)
  sudo $0 --uninstall  # remove installation
  sudo $0 --help       # show this help

Exit codes:
  0  success / healthy
  1  errors found / health check failed / fixes failed
  2  warnings only (--status)

The installer:
  - Detects your ocserv configuration automatically
  - Installs binary, static assets, and migrations
  - Generates /etc/ocserv-panel/config.toml (skipped on upgrade if file exists)
  - Installs and starts the systemd service (runs as root)
  - Verifies the health endpoint

Re-running the installer on an existing installation performs an upgrade:
  existing data and configuration are preserved and backed up before changes.

Status check (--status):
  Inspects the current installation and reports any issues:
  binary, service, config, certificates, permissions, health, and ocserv integration.
  Returns exit code 0 (clean), 1 (errors), or 2 (warnings only).

Fix mode (--fix):
  Runs the same checks as --status, but automatically attempts to resolve
  common issues such as:
  - Missing or incorrectly owned data directories
  - SQLite database file permission/ownership problems
  - WAL/SHM journal file ownership mismatches
  - Config file permission modes
  - Service not enabled on boot
  - Relative DSN path in config
  Restarts the service after successful fixes.

Healthcheck (--healthcheck):
  Lightweight check for monitoring/cron. Hits the health endpoint and
  exits 0 (OK) or 1 (FAIL). Reads port from installed config.

Prerequisites:
  - Root access (sudo)
  - systemd
  - ocserv installed and configured
  - TLS certificates (e.g. Let's Encrypt) — optional

EOF
}

# ─── Banner ───────────────────────────────────────────────────────────────────

header() {
    local version="unknown"
    if [[ -f "${VERSION_FILE}" ]]; then
        version="$(cat "${VERSION_FILE}")"
    fi
    echo ""
    bold "╔══════════════════════════════════════════════╗"
    bold "║  ocserv-panel Installer  v${version}"
    bold "╚══════════════════════════════════════════════╝"
    echo ""
}

# ─── Version helper ───────────────────────────────────────────────────────────

read_version() {
    local version="unknown"
    if [[ -f "${VERSION_FILE}" ]]; then
        version="$(cat "${VERSION_FILE}")"
    fi
    echo "${version}"
}

# ─── Detect ocserv binaries and configuration ─────────────────────────────────

detect_ocserv() {
    info "Detecting ocserv installation..."

    # Binary locations — common across distros and manual builds
    local candidate
    for candidate in \
        /usr/sbin/ocserv \
        /usr/local/sbin/ocserv \
        /usr/bin/ocserv \
        /usr/local/bin/ocserv
    do
        if [[ -x "${candidate}" ]]; then
            DETECTED_OCSERV_BIN="${candidate}"
            break
        fi
    done
    if [[ -z "${DETECTED_OCSERV_BIN}" ]]; then
        DETECTED_OCSERV_BIN="$(command -v ocserv 2>/dev/null)" || true
    fi

    for candidate in \
        /usr/bin/occtl \
        /usr/local/bin/occtl \
        /usr/sbin/occtl
    do
        if [[ -x "${candidate}" ]]; then
            DETECTED_OCCTL_BIN="${candidate}"
            break
        fi
    done
    if [[ -z "${DETECTED_OCCTL_BIN}" ]]; then
        DETECTED_OCCTL_BIN="$(command -v occtl 2>/dev/null)" || true
    fi

    for candidate in \
        /usr/bin/ocpasswd \
        /usr/local/bin/ocpasswd \
        /usr/sbin/ocpasswd
    do
        if [[ -x "${candidate}" ]]; then
            DETECTED_OCPASSWD_BIN="${candidate}"
            break
        fi
    done
    if [[ -z "${DETECTED_OCPASSWD_BIN}" ]]; then
        DETECTED_OCPASSWD_BIN="$(command -v ocpasswd 2>/dev/null)" || true
    fi

    # Config file — try to find it from the running unit first
    DETECTED_OCSERV_CONF=""
    if command -v systemctl &>/dev/null; then
        local exec_start=""
        exec_start="$(systemctl show ocserv --property=ExecStart 2>/dev/null \
            | grep -oP '(?<=path=)[^ ;]+' | head -1)" || true
        if [[ -n "${exec_start}" ]]; then
            # Extract -c <configfile> argument
            local unit_conf=""
            unit_conf="$(systemctl show ocserv --property=ExecStart 2>/dev/null \
                | grep -oP '(?<=-c )\S+' | head -1)" || true
            if [[ -n "${unit_conf}" && -f "${unit_conf}" ]]; then
                DETECTED_OCSERV_CONF="${unit_conf}"
            fi
        fi
    fi

    # Fall back to well-known paths
    if [[ -z "${DETECTED_OCSERV_CONF}" ]]; then
        for candidate in \
            /etc/ocserv/ocserv.conf \
            /usr/local/etc/ocserv/ocserv.conf \
            /etc/ocserv.conf
        do
            if [[ -f "${candidate}" ]]; then
                DETECTED_OCSERV_CONF="${candidate}"
                break
            fi
        done
    fi

    # Parse directives from the config file
    if [[ -n "${DETECTED_OCSERV_CONF}" && -r "${DETECTED_OCSERV_CONF}" ]]; then
        local conf="${DETECTED_OCSERV_CONF}"

        # occtl-socket-file = <path> (the management socket used by occtl)
        # NOTE: "socket-file" is internal worker IPC with .PID suffix — NOT for occtl.
        DETECTED_OCSERV_SOCKET="$(grep -E '^\s*occtl-socket-file\s*=' "${conf}" \
            | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'")" || true
        if [[ -z "${DETECTED_OCSERV_SOCKET}" ]]; then
            DETECTED_OCSERV_SOCKET="/var/run/occtl.socket"
        fi

        # plain[passwd=<path>] in auth directive
        DETECTED_OCPASSWD_FILE="$(grep -E '^\s*auth\s*=' "${conf}" \
            | grep -oP '(?<=passwd=)[^]]+' | head -1 | tr -d '"' | tr -d "'")" || true
        if [[ -z "${DETECTED_OCPASSWD_FILE}" ]]; then
            DETECTED_OCPASSWD_FILE="/etc/ocserv/ocpasswd"
        fi

        # server-cert = <path>
        DETECTED_CERT_PATH="$(grep -E '^\s*server-cert\s*=' "${conf}" \
            | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'")" || true
        if [[ -n "${DETECTED_CERT_PATH}" && "${DETECTED_CERT_PATH}" != /* ]]; then
            # Relative path — resolve from config dir
            local conf_dir
            conf_dir="$(dirname "${DETECTED_OCSERV_CONF}")"
            DETECTED_CERT_PATH="${conf_dir}/${DETECTED_CERT_PATH}"
        fi

        # server-key = <path>
        DETECTED_KEY_PATH="$(grep -E '^\s*server-key\s*=' "${conf}" \
            | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'")" || true
        if [[ -n "${DETECTED_KEY_PATH}" && "${DETECTED_KEY_PATH}" != /* ]]; then
            local conf_dir
            conf_dir="$(dirname "${DETECTED_OCSERV_CONF}")"
            DETECTED_KEY_PATH="${conf_dir}/${DETECTED_KEY_PATH}"
        fi

        # Derive cert dir from cert path
        if [[ -n "${DETECTED_CERT_PATH}" ]]; then
            DETECTED_CERT_DIR="$(dirname "${DETECTED_CERT_PATH}")"
        fi
    fi

    # Final defaults for anything still empty
    if [[ -z "${DETECTED_OCSERV_CONF}" ]]; then
        DETECTED_OCSERV_CONF="/etc/ocserv/ocserv.conf"
    fi
    if [[ -z "${DETECTED_OCSERV_SOCKET}" ]]; then
        DETECTED_OCSERV_SOCKET="/var/run/occtl.socket"
    fi
    if [[ -z "${DETECTED_OCPASSWD_FILE}" ]]; then
        DETECTED_OCPASSWD_FILE="/etc/ocserv/ocpasswd"
    fi
    if [[ -z "${DETECTED_CERT_DIR}" ]]; then
        DETECTED_CERT_DIR="/etc/letsencrypt/live"
    fi
}

# ─── Detect existing installation ─────────────────────────────────────────────

detect_existing() {
    EXISTING_VERSION=""
    if [[ -x "${BINARY_DST}" ]]; then
        # The binary may fail (missing config, db, etc.) — that's fine during
        # detection. We use if-construct which is exempt from ERR trap.
        local ver_output=""
        if ver_output="$("${BINARY_DST}" --version 2>/dev/null)"; then
            EXISTING_VERSION="$(echo "${ver_output}" | awk '{print $NF}')" || true
        fi
        if [[ -z "${EXISTING_VERSION}" ]]; then
            EXISTING_VERSION="unknown"
        fi
    fi
}

# ─── Preflight checks ─────────────────────────────────────────────────────────

preflight() {
    bold "Preflight checks"
    echo ""
    local errors=0

    # Running as root
    if [[ "${EUID}" -eq 0 ]]; then
        ok "Running as root"
    else
        fail "Must run as root (use sudo)"
        errors=$((errors + 1))
    fi

    # systemd available
    if command -v systemctl &>/dev/null; then
        if systemctl list-units &>/dev/null 2>&1; then
            ok "systemd is available and running"
        else
            fail "systemctl found but systemd is not running"
            printf "       → Is this system using systemd? Check: ${BOLD}ps 1${RESET}\n"
            errors=$((errors + 1))
        fi
    else
        fail "systemctl not found"
        printf "       → This installer requires systemd. Are you on a supported distro?\n"
        errors=$((errors + 1))
    fi

    # Artifacts present — binary
    if [[ -f "${BINARY_SRC}" ]]; then
        if [[ -x "${BINARY_SRC}" ]]; then
            ok "Binary: ${BINARY_SRC}"
        else
            fail "Binary found but not executable: ${BINARY_SRC}"
            printf "       → Fix with: ${BOLD}chmod +x %s${RESET}\n" "${BINARY_SRC}"
            errors=$((errors + 1))
        fi
    else
        fail "Binary not found: ${BINARY_SRC}"
        printf "       → Expected layout after extracting the tarball:\n"
        printf "         bin/ocserv-panel, static/, migrations/, install.sh\n"
        printf "       → Did you extract the tarball correctly?\n"
        printf "         ${BOLD}tar xzf ocserv-panel-*.tar.gz -C /opt/ocserv-panel${RESET}\n"
        errors=$((errors + 1))
    fi

    # Artifacts present — static directory
    if [[ -d "${STATIC_SRC}" ]]; then
        local file_count
        file_count="$(find "${STATIC_SRC}" -type f | wc -l)" || true
        if [[ "${file_count}" -gt 0 ]]; then
            ok "Static assets: ${STATIC_SRC} (${file_count} files)"
        else
            fail "Static assets directory is empty: ${STATIC_SRC}"
            printf "       → The frontend build may have failed. Rebuild with: ${BOLD}make build-frontend${RESET}\n"
            errors=$((errors + 1))
        fi
    else
        fail "Static assets directory not found: ${STATIC_SRC}"
        printf "       → Expected after tarball extraction. Check archive contents.\n"
        errors=$((errors + 1))
    fi

    # Artifacts present — migrations
    if [[ -d "${MIGRATIONS_SRC}" ]]; then
        local sql_count
        sql_count="$(find "${MIGRATIONS_SRC}" -name '*.sql' -type f | wc -l)" || true
        if [[ "${sql_count}" -gt 0 ]]; then
            ok "Migrations: ${MIGRATIONS_SRC} (${sql_count} SQL files)"
        else
            fail "Migrations directory has no .sql files: ${MIGRATIONS_SRC}"
            errors=$((errors + 1))
        fi
    else
        fail "Migrations directory not found: ${MIGRATIONS_SRC}"
        printf "       → Expected after tarball extraction. Check archive contents.\n"
        errors=$((errors + 1))
    fi

    # VERSION file
    if [[ -f "${VERSION_FILE}" ]]; then
        ok "Version: $(cat "${VERSION_FILE}")"
    else
        warn "VERSION file missing — will use 'unknown' as version identifier"
    fi

    echo ""
    bold "ocserv detection"
    echo ""

    # ocserv binary
    if [[ -n "${DETECTED_OCSERV_BIN}" && -x "${DETECTED_OCSERV_BIN}" ]]; then
        local ocserv_ver=""
        ocserv_ver="$("${DETECTED_OCSERV_BIN}" --version 2>/dev/null | head -1)" || true
        ok "ocserv: ${DETECTED_OCSERV_BIN} ${ocserv_ver:+(${ocserv_ver})}"
    else
        fail "ocserv binary not found"
        printf "       → Is ocserv installed? Check: ${BOLD}which ocserv${RESET}\n"
        printf "       → Install ocserv first, then re-run this installer.\n"
        errors=$((errors + 1))
    fi

    # occtl
    if [[ -n "${DETECTED_OCCTL_BIN}" && -x "${DETECTED_OCCTL_BIN}" ]]; then
        ok "occtl: ${DETECTED_OCCTL_BIN}"
    else
        fail "occtl binary not found"
        printf "       → occtl is required for session management.\n"
        printf "       → Usually installed alongside ocserv. Check: ${BOLD}which occtl${RESET}\n"
        errors=$((errors + 1))
    fi

    # ocpasswd
    if [[ -n "${DETECTED_OCPASSWD_BIN}" && -x "${DETECTED_OCPASSWD_BIN}" ]]; then
        ok "ocpasswd: ${DETECTED_OCPASSWD_BIN}"
    else
        fail "ocpasswd binary not found"
        printf "       → ocpasswd is required for VPN user management.\n"
        printf "       → Usually installed alongside ocserv. Check: ${BOLD}which ocpasswd${RESET}\n"
        errors=$((errors + 1))
    fi

    # ocserv config
    if [[ -n "${DETECTED_OCSERV_CONF}" && -f "${DETECTED_OCSERV_CONF}" ]]; then
        ok "Config: ${DETECTED_OCSERV_CONF}"
    else
        fail "ocserv config not found at: ${DETECTED_OCSERV_CONF:-/etc/ocserv/ocserv.conf}"
        printf "       → The panel needs to read ocserv.conf for user/session management.\n"
        printf "       → Check: ${BOLD}ls /etc/ocserv/ocserv.conf${RESET}\n"
        errors=$((errors + 1))
    fi

    # ocserv running check
    if command -v systemctl &>/dev/null; then
        if systemctl is-active --quiet ocserv 2>/dev/null; then
            ok "ocserv service: running"
        else
            warn "ocserv service is not running"
            printf "       → The panel can still be installed, but some features won't work\n"
            printf "         until ocserv is started: ${BOLD}systemctl start ocserv${RESET}\n"
        fi
    fi

    echo ""
    bold "TLS certificates"
    echo ""

    # TLS cert detection
    if [[ -n "${DETECTED_CERT_PATH}" && -f "${DETECTED_CERT_PATH}" ]]; then
        ok "Certificate: ${DETECTED_CERT_PATH}"
    elif [[ -n "${DETECTED_CERT_PATH}" ]]; then
        fail "Certificate path configured but file not found: ${DETECTED_CERT_PATH}"
        printf "       → The panel needs TLS certs for HTTPS.\n"
        printf "       → Check that Let's Encrypt renewal is working: ${BOLD}certbot certificates${RESET}\n"
        errors=$((errors + 1))
    else
        warn "No TLS certificate detected in ocserv config"
        printf "       → The panel will be installed without HTTPS.\n"
        printf "       → For production, configure TLS certs after install.\n"
    fi

    if [[ -n "${DETECTED_KEY_PATH}" && -f "${DETECTED_KEY_PATH}" ]]; then
        ok "Private key: ${DETECTED_KEY_PATH}"
    elif [[ -n "${DETECTED_KEY_PATH}" ]]; then
        fail "Key path configured but file not found: ${DETECTED_KEY_PATH}"
        errors=$((errors + 1))
    fi

    echo ""
    bold "System tools"
    echo ""

    # curl for health check
    if command -v curl &>/dev/null; then
        ok "curl: available (health check enabled)"
    else
        warn "curl not found — post-install health check will be skipped"
        printf "       → Install curl for automated verification: ${BOLD}apt install curl${RESET}\n"
    fi

    # Disk space check (need at least 100MB free on /usr and /var)
    local usr_free var_free
    usr_free="$(df -m /usr 2>/dev/null | awk 'NR==2{print $4}')" || true
    var_free="$(df -m /var 2>/dev/null | awk 'NR==2{print $4}')" || true
    if [[ -n "${usr_free}" && "${usr_free}" -lt 100 ]]; then
        fail "Less than 100MB free on /usr (${usr_free}MB available)"
        printf "       → The binary and static assets need ~50MB on /usr\n"
        errors=$((errors + 1))
    else
        ok "Disk space /usr: ${usr_free:-unknown}MB available"
    fi
    if [[ -n "${var_free}" && "${var_free}" -lt 100 ]]; then
        fail "Less than 100MB free on /var (${var_free}MB available)"
        printf "       → Database and metrics are stored in /var/lib/ocserv-panel\n"
        errors=$((errors + 1))
    else
        ok "Disk space /var: ${var_free:-unknown}MB available"
    fi

    # Existing installation info
    echo ""
    if [[ -n "${EXISTING_VERSION}" ]]; then
        bold "Existing installation"
        echo ""
        info "Found version: ${EXISTING_VERSION} → will upgrade to $(read_version)"
    fi

    # Final verdict
    echo ""
    if [[ "${errors}" -gt 0 ]]; then
        echo ""
        printf "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}\n"
        printf "${RED}  Preflight failed: %d error(s) found${RESET}\n" "${errors}"
        printf "${RED}  Fix the issues above and re-run the installer.${RESET}\n"
        printf "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}\n"
        echo ""
        exit 1
    fi

    ok "All preflight checks passed"
    echo ""
}

# ─── Installation plan display ────────────────────────────────────────────────

show_plan() {
    local mode="${1:-install}"
    local version
    version="$(read_version)"

    bold "Installation plan"
    echo ""
    if [[ "${mode}" == "upgrade" ]]; then
        info "Upgrading from ${EXISTING_VERSION} → ${version}"
    else
        info "Fresh install of version ${version}"
    fi
    echo ""
    echo "  Detected ocserv paths:"
    echo "    ocserv binary  : ${DETECTED_OCSERV_BIN:-not found}"
    echo "    occtl binary   : ${DETECTED_OCCTL_BIN:-not found}"
    echo "    ocpasswd binary: ${DETECTED_OCPASSWD_BIN:-not found}"
    echo "    config file    : ${DETECTED_OCSERV_CONF}"
    echo "    socket         : ${DETECTED_OCSERV_SOCKET}"
    echo "    password file  : ${DETECTED_OCPASSWD_FILE}"
    if [[ -n "${DETECTED_CERT_PATH}" ]]; then
        echo "    TLS cert       : ${DETECTED_CERT_PATH}"
        echo "    TLS key        : ${DETECTED_KEY_PATH}"
    fi
    echo ""
    echo "  Steps:"
    local n=1
    if [[ "${mode}" == "upgrade" ]]; then
        echo "  ${n}. Stop existing service"; n=$((n + 1))
        echo "  ${n}. Backup data (DB + config)"; n=$((n + 1))
    fi
    echo "  ${n}. Install binary → ${BINARY_DST}"; n=$((n + 1))
    echo "  ${n}. Install static assets → ${STATIC_DST}"; n=$((n + 1))
    echo "  ${n}. Install migrations → ${MIGRATIONS_DST}"; n=$((n + 1))
    if [[ "${mode}" == "upgrade" && -f "${CONFIG_FILE}" ]]; then
        echo "  ${n}. Config preserved (upgrade) → ${CONFIG_FILE}"; n=$((n + 1))
    else
        echo "  ${n}. Generate config → ${CONFIG_FILE}"; n=$((n + 1))
    fi
    echo "  ${n}. Install systemd unit → ${SYSTEMD_UNIT}"; n=$((n + 1))
    echo "  ${n}. Enable and start service"; n=$((n + 1))
    echo "  ${n}. Verify health endpoint"
    echo ""
}

# ─── Confirmation prompt ──────────────────────────────────────────────────────

confirm() {
    local prompt="${1:-Continue?}"
    local default="${2:-y}"
    local yn_hint
    if [[ "${default}" == "y" ]]; then
        yn_hint="[Y/n]"
    else
        yn_hint="[y/N]"
    fi
    while true; do
        printf "${BOLD}%s %s ${RESET}" "${prompt}" "${yn_hint}"
        local answer
        read -r answer
        answer="${answer:-${default}}"
        case "${answer,,}" in
            y|yes) return 0 ;;
            n|no)  return 1 ;;
            *)     echo "  Please answer y or n." ;;
        esac
    done
}

# ─── Stop existing service ────────────────────────────────────────────────────

do_stop_existing() {
    if systemctl is-active --quiet ocserv-panel 2>/dev/null; then
        if ! systemctl stop ocserv-panel; then
            fail "Could not stop ocserv-panel service"
            printf "       → Check: ${BOLD}systemctl status ocserv-panel${RESET}\n"
            printf "       → Force stop: ${BOLD}systemctl kill ocserv-panel${RESET}\n"
            return 1
        fi
        ok "Service stopped"
    else
        ok "Service was not running"
    fi
}

# ─── Backup data ──────────────────────────────────────────────────────────────

do_backup_data() {
    local backup_dir="/var/backups/ocserv-panel"
    local timestamp
    timestamp="$(date +%Y%m%dT%H%M%S)"
    local backup_file="${backup_dir}/backup-${EXISTING_VERSION}-${timestamp}.tar.gz"

    if ! mkdir -p "${backup_dir}"; then
        fail "Cannot create backup directory: ${backup_dir}"
        return 1
    fi

    local items=()
    if [[ -f "${DATA_DIR}/ocserv_panel.db" ]]; then
        items+=("${DATA_DIR}/ocserv_panel.db")
    fi
    if [[ -f "${DATA_DIR}/ocserv_panel.db-wal" ]]; then
        items+=("${DATA_DIR}/ocserv_panel.db-wal")
    fi
    if [[ -f "${CONFIG_FILE}" ]]; then
        items+=("${CONFIG_FILE}")
    fi

    if [[ ${#items[@]} -gt 0 ]]; then
        if tar -czf "${backup_file}" "${items[@]}" 2>/dev/null; then
            ok "Backup: ${backup_file}"
        else
            fail "Backup creation failed"
            printf "       → Check disk space and permissions on ${backup_dir}\n"
            return 1
        fi
    else
        ok "No existing data to back up"
    fi
}

# ─── Install binary ───────────────────────────────────────────────────────────

do_install_binary() {
    if ! install -o root -g root -m 755 "${BINARY_SRC}" "${BINARY_DST}"; then
        fail "Failed to install binary to ${BINARY_DST}"
        printf "       → Check: permissions on $(dirname "${BINARY_DST}"), disk space\n"
        return 1
    fi
    # Verify the installed binary runs
    if "${BINARY_DST}" --version &>/dev/null; then
        ok "Binary installed and verified: ${BINARY_DST}"
    else
        warn "Binary installed but --version check failed (may be fine if config is not yet set up)"
    fi
}

# ─── Install static assets ────────────────────────────────────────────────────

do_install_static() {
    if ! mkdir -p "${STATIC_DST}"; then
        fail "Cannot create directory: ${STATIC_DST}"
        return 1
    fi

    if command -v rsync &>/dev/null; then
        if ! rsync -a --delete "${STATIC_SRC}/" "${STATIC_DST}/"; then
            fail "rsync failed copying static assets"
            return 1
        fi
    else
        rm -rf "${STATIC_DST}"
        if ! cp -r "${STATIC_SRC}" "${STATIC_DST}"; then
            fail "cp failed copying static assets"
            return 1
        fi
    fi
    chown -R "root:root" "${STATIC_DST}"
    ok "Static assets: ${STATIC_DST}"
}

# ─── Install migrations ───────────────────────────────────────────────────────

do_install_migrations() {
    if ! mkdir -p "${MIGRATIONS_DST}"; then
        fail "Cannot create directory: ${MIGRATIONS_DST}"
        return 1
    fi

    if command -v rsync &>/dev/null; then
        if ! rsync -a --delete "${MIGRATIONS_SRC}/" "${MIGRATIONS_DST}/"; then
            fail "rsync failed copying migrations"
            return 1
        fi
    else
        rm -rf "${MIGRATIONS_DST}"
        if ! cp -r "${MIGRATIONS_SRC}" "${MIGRATIONS_DST}"; then
            fail "cp failed copying migrations"
            return 1
        fi
    fi
    chown -R "root:root" "${MIGRATIONS_DST}"
    ok "Migrations: ${MIGRATIONS_DST}"
}

# ─── Generate config.toml ─────────────────────────────────────────────────────

do_generate_config() {
    if [[ -f "${CONFIG_FILE}" ]]; then
        ok "Config preserved (existing): ${CONFIG_FILE}"
        return 0
    fi

    if ! mkdir -p "${CONFIG_DIR}"; then
        fail "Cannot create config directory: ${CONFIG_DIR}"
        return 1
    fi

    local hostname
    hostname="$(hostname -f 2>/dev/null || hostname)"
    local ocserv_bin="${DETECTED_OCSERV_BIN:-/usr/sbin/ocserv}"
    local occtl_bin="${DETECTED_OCCTL_BIN:-/usr/bin/occtl}"
    local ocpasswd_bin="${DETECTED_OCPASSWD_BIN:-/usr/bin/ocpasswd}"
    local ocserv_conf="${DETECTED_OCSERV_CONF:-/etc/ocserv/ocserv.conf}"
    local ocserv_socket="${DETECTED_OCSERV_SOCKET:-/var/run/occtl.socket}"
    local ocpasswd_file="${DETECTED_OCPASSWD_FILE:-/etc/ocserv/ocpasswd}"
    local cert_dir="${DETECTED_CERT_DIR:-/etc/letsencrypt/live}"
    local tls_cert="${DETECTED_CERT_PATH:-}"
    local tls_key="${DETECTED_KEY_PATH:-}"

    cat > "${CONFIG_FILE}" <<EOF
# ocserv-panel configuration
# Generated by installer on $(date -u '+%Y-%m-%dT%H:%M:%SZ')
# Edit to adjust for your environment.

[general]
app_name = "ocserv Panel"
timezone = "UTC"

[installation]
config_path = "${CONFIG_DIR}/config.toml"
data_dir = "${DATA_DIR}"
log_dir = "${LOG_DIR}"
static_dir = "${STATIC_DST}"

ocserv_config_path = "${ocserv_conf}"
ocserv_socket_path = "${ocserv_socket}"
ocserv_bin_path    = "${ocserv_bin}"
occtl_bin_path     = "${occtl_bin}"
ocpasswd_bin_path  = "${ocpasswd_bin}"
ocpasswd_file_path = "${ocpasswd_file}"

cert_dir = "${cert_dir}"
ocserv_log_path = "/var/log/ocserv/ocserv.log"
panel_log_path  = "${LOG_DIR}/panel.log"
panel_systemd_unit = "ocserv-panel"

[service]
name = "ocserv-panel"
user = "root"
group = "root"
start_on_boot = true
systemd_unit_name = "ocserv"

[web]
host    = "0.0.0.0"
port    = ${PANEL_PORT}
base_url = "https://${hostname}:${PANEL_PORT}"

[security]
tls_cert_path  = "${tls_cert}"
tls_key_path   = "${tls_key}"
csrf_enabled   = true
security_headers = true

[database]
dsn = "file:${DATA_DIR}/ocserv_panel.db?_journal=WAL&_busy_timeout=5000"
max_open_conns = 5
max_idle_conns = 2
conn_max_lifetime = 300

[monitoring]
health_endpoint = true
dashboard_refresh_interval = 15
log_file_read_enabled = true
EOF

    chown root:root "${CONFIG_FILE}"
    chmod 644 "${CONFIG_FILE}"
    ok "Config generated: ${CONFIG_FILE}"
}

# ─── Check config keys on upgrade ────────────────────────────────────────────
# Warns if the installed config is missing keys present in the template.
# This helps administrators discover new config options after an upgrade.

do_check_config_keys() {
    # Only relevant when config was preserved (upgrade path)
    if [[ ! -f "${CONFIG_FILE}" ]]; then
        return 0
    fi
    local template="${SCRIPT_DIR}/config.toml.example"
    if [[ ! -f "${template}" ]]; then
        return 0
    fi

    # Extract key names (lines matching: key = value) from both files
    local missing=""
    missing="$(comm -23 \
        <(grep -E '^\s*[a-z_]+\s*=' "${template}" | sed 's/\s*=.*//' | sed 's/^\s*//' | sort -u) \
        <(grep -E '^\s*[a-z_]+\s*=' "${CONFIG_FILE}" | sed 's/\s*=.*//' | sed 's/^\s*//' | sort -u))" || true

    if [[ -n "${missing}" ]]; then
        echo ""
        warn "New config keys available in this version (not in your config):"
        echo "${missing}" | while IFS= read -r key; do
            printf "       • %s\n" "${key}"
        done
        info "Review ${template} and add any needed keys to ${CONFIG_FILE}"
        echo ""
    fi
}

# ─── Install systemd unit ─────────────────────────────────────────────────────

do_install_systemd() {
    local tls_cert="${DETECTED_CERT_PATH:-}"
    local tls_key="${DETECTED_KEY_PATH:-}"

    # Build optional TLS env lines
    local tls_env_lines=""
    if [[ -n "${tls_cert}" ]]; then
        tls_env_lines="Environment=\"OCSERV_PANEL_TLS_CERT=${tls_cert}\"
Environment=\"OCSERV_PANEL_TLS_KEY=${tls_key}\"
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
        printf "       → Check: ${BOLD}systemctl status${RESET}\n"
        return 1
    fi
    ok "Systemd unit installed: ${SYSTEMD_UNIT}"
}

# ─── Start service ────────────────────────────────────────────────────────────

do_start_service() {
    if ! systemctl enable ocserv-panel 2>/dev/null; then
        fail "systemctl enable ocserv-panel failed"
        return 1
    fi

    if ! systemctl start ocserv-panel; then
        fail "Service failed to start"
        echo ""
        printf "       Recent logs:\n"
        journalctl -u ocserv-panel --no-pager -n 15 2>/dev/null | sed 's/^/         /' || true
        echo ""
        printf "       Troubleshooting:\n"
        printf "         ${BOLD}systemctl status ocserv-panel${RESET}\n"
        printf "         ${BOLD}journalctl -u ocserv-panel -f${RESET}\n"
        echo ""
        printf "       Common causes:\n"
        printf "         • TLS cert files not found at configured path\n"
        printf "         • Port ${PANEL_PORT} already in use (check: ss -tlnp | grep ${PANEL_PORT})\n"
        printf "         • Database path not writable\n"
        return 1
    fi

    # Wait for service to stabilise
    local retries=6
    local i=0
    while [[ $i -lt $retries ]]; do
        if systemctl is-active --quiet ocserv-panel; then
            ok "Service is active and running"
            return 0
        fi
        sleep 1
        i=$((i + 1))
    done

    # Service started but isn't stable
    fail "Service started but is not stable after ${retries} seconds"
    printf "       → It may be crash-looping. Check:\n"
    printf "         ${BOLD}journalctl -u ocserv-panel --no-pager -n 30${RESET}\n"
    return 1
}

# ─── Verify health endpoint ───────────────────────────────────────────────────

do_verify_health() {
    if ! command -v curl &>/dev/null; then
        warn "curl not available — skipping health check"
        printf "       → Verify manually: open https://<your-host>:${PANEL_PORT} in a browser\n"
        return 0
    fi

    # Use HTTPS if TLS certs are configured, otherwise HTTP
    local scheme="http"
    if [[ -n "${DETECTED_CERT_PATH}" && -n "${DETECTED_KEY_PATH}" ]]; then
        scheme="https"
    fi
    local health_url="${scheme}://127.0.0.1:${PANEL_PORT}/api/health"
    local retries=5
    local i=0
    local last_code="000"

    while [[ $i -lt $retries ]]; do
        last_code="$(curl -sk -o /dev/null -w '%{http_code}' \
            --connect-timeout 2 --max-time 5 "${health_url}" 2>/dev/null)" || last_code="000"
        if [[ "${last_code}" == "200" ]]; then
            ok "Health check passed: ${health_url} → 200 OK"
            return 0
        fi
        i=$((i + 1))
        sleep 2
    done

    warn "Health endpoint did not respond with 200 (last HTTP code: ${last_code})"
    printf "       → The service may still be initialising.\n"
    printf "       → Manual check: ${BOLD}curl -k ${health_url}${RESET}\n"
    if [[ "${last_code}" == "000" ]]; then
        printf "       → Code 000 means connection refused — service may not be listening yet.\n"
        printf "       → Check: ${BOLD}ss -tlnp | grep ${PANEL_PORT}${RESET}\n"
    fi
    return 0
}

# ─── Success banner ───────────────────────────────────────────────────────────

success_banner() {
    local hostname
    hostname="$(hostname -f 2>/dev/null || hostname)"
    local version
    version="$(read_version)"

    # Determine scheme
    local scheme="http"
    if [[ -n "${DETECTED_CERT_PATH}" && -n "${DETECTED_KEY_PATH}" ]]; then
        scheme="https"
    fi

    echo ""
    printf "${GREEN}╔══════════════════════════════════════════════════╗${RESET}\n"
    printf "${GREEN}║  Installation complete! v%-23s ║${RESET}\n" "${version}"
    printf "${GREEN}╚══════════════════════════════════════════════════╝${RESET}\n"
    echo ""
    info "Access the panel at:"
    echo ""
    printf "    ${BOLD}${scheme}://${hostname}:${PANEL_PORT}${RESET}\n"
    echo ""
    info "First login: create your admin account at the setup page."
    echo ""
    info "Useful commands:"
    echo "    systemctl status ocserv-panel     # service status"
    echo "    journalctl -u ocserv-panel -f     # live logs"
    echo "    systemctl restart ocserv-panel    # restart after config changes"
    echo "    /usr/local/bin/ocserv-panel -v    # show version"
    echo ""
    if [[ -z "${DETECTED_CERT_PATH}" ]]; then
        warn "No TLS certificate detected — the panel is running in HTTP mode."
        printf "       For production use, configure TLS in ${CONFIG_FILE}\n"
        printf "       and set OCSERV_PANEL_TLS_CERT / OCSERV_PANEL_TLS_KEY.\n"
        echo ""
    fi
}

# ─── Status check ────────────────────────────────────────────────────────────

status_check() {
    # Read actual port from config if available
    read_port_from_config

    echo ""
    bold "╔══════════════════════════════════════════════════╗"
    bold "║  ocserv-panel Installation Status               ║"
    bold "╚══════════════════════════════════════════════════╝"
    echo ""

    local errors=0
    local warnings=0

    # ── Binary ──
    bold "Binary"
    echo ""
    if [[ -x "${BINARY_DST}" ]]; then
        local ver=""
        if ver="$("${BINARY_DST}" --version 2>/dev/null)"; then
            ok "Installed: ${BINARY_DST} (${ver})"
        else
            ok "Installed: ${BINARY_DST} (version check failed)"
        fi
    else
        fail "Not installed: ${BINARY_DST}"
        errors=$((errors + 1))
    fi

    # ── Systemd service ──
    echo ""
    bold "Systemd service"
    echo ""
    if [[ -f "${SYSTEMD_UNIT}" ]]; then
        ok "Unit file: ${SYSTEMD_UNIT}"
    else
        fail "Unit file missing: ${SYSTEMD_UNIT}"
        errors=$((errors + 1))
    fi

    if command -v systemctl &>/dev/null; then
        if systemctl is-enabled --quiet ocserv-panel 2>/dev/null; then
            ok "Enabled: yes (starts on boot)"
        else
            warn "Enabled: no (will not start on boot)"
            if ! try_fix "Enable ocserv-panel on boot" systemctl enable ocserv-panel 2>/dev/null; then
                printf "       → Enable with: ${BOLD}systemctl enable ocserv-panel${RESET}\n"
            fi
            warnings=$((warnings + 1))
        fi

        local svc_state
        svc_state="$(systemctl is-active ocserv-panel 2>/dev/null)" || true
        case "${svc_state}" in
            active)
                ok "Status: ${svc_state}"
                # Show uptime
                local since=""
                since="$(systemctl show ocserv-panel --property=ActiveEnterTimestamp 2>/dev/null \
                    | cut -d= -f2)" || true
                if [[ -n "${since}" ]]; then
                    printf "       Running since: ${since}\n"
                fi
                ;;
            failed)
                fail "Status: ${svc_state}"
                printf "       → Last logs:\n"
                journalctl -u ocserv-panel --no-pager -n 5 2>/dev/null | sed 's/^/         /' || true
                printf "       → Full logs: ${BOLD}journalctl -u ocserv-panel -n 50${RESET}\n"
                errors=$((errors + 1))
                ;;
            inactive)
                warn "Status: ${svc_state} (not running)"
                printf "       → Start with: ${BOLD}systemctl start ocserv-panel${RESET}\n"
                warnings=$((warnings + 1))
                ;;
            *)
                warn "Status: ${svc_state:-unknown}"
                warnings=$((warnings + 1))
                ;;
        esac

        # Show listening port
        local listen_info=""
        listen_info="$(ss -tlnp 2>/dev/null | grep ":${PANEL_PORT} ")" || true
        if [[ -n "${listen_info}" ]]; then
            ok "Listening on port ${PANEL_PORT}"
        else
            if [[ "${svc_state}" == "active" ]]; then
                warn "Service active but port ${PANEL_PORT} not found in listener list"
                printf "       → Check: ${BOLD}ss -tlnp | grep ${PANEL_PORT}${RESET}\n"
                warnings=$((warnings + 1))
            fi
        fi
    fi

    # ── Configuration ──
    echo ""
    bold "Configuration"
    echo ""
    if [[ -f "${CONFIG_FILE}" ]]; then
        local perms owner
        perms="$(stat -c '%a' "${CONFIG_FILE}" 2>/dev/null)" || true
        owner="$(stat -c '%U:%G' "${CONFIG_FILE}" 2>/dev/null)" || true
        ok "Config: ${CONFIG_FILE} (${owner}, mode ${perms})"
        if [[ "${perms}" != "644" ]]; then
            warn "Expected mode 644, got ${perms}"
            if ! try_fix "Set config file mode to 644" chmod 644 "${CONFIG_FILE}"; then
                printf "       → Fix: ${BOLD}chmod 644 %s${RESET}\n" "${CONFIG_FILE}"
            fi
            warnings=$((warnings + 1))
        fi
    else
        fail "Config missing: ${CONFIG_FILE}"
        printf "       → Re-run installer or create from template:\n"
        printf "         ${BOLD}cp deploy/config.toml.example %s${RESET}\n" "${CONFIG_FILE}"
        errors=$((errors + 1))
    fi

    # ── Data directories ──
    echo ""
    bold "Data directories"
    echo ""
    for dir_path in "${DATA_DIR}" "${LOG_DIR}" "${METRICS_DIR}"; do
        if [[ -d "${dir_path}" ]]; then
            ok "${dir_path} exists"
        else
            fail "Missing: ${dir_path}"
            if ! try_fix "Create directory ${dir_path}" mkdir -p "${dir_path}"; then
                printf "       → Create: ${BOLD}mkdir -p %s${RESET}\n" "${dir_path}"
            fi
            errors=$((errors + 1))
        fi
    done

    # ── File ownership ──
    # Service runs as root — all installation files must be owned by root:root
    echo ""
    bold "File ownership"
    echo ""

    local ownership_ok=true

    # Helper: check ownership of a file or directory
    __check_ownership() {
        local path="$1"
        local label="$2"
        local expected_owner="root:root"

        if [[ ! -e "${path}" ]]; then
            return 0  # skip non-existent paths (caught by other checks)
        fi

        local actual_owner
        actual_owner="$(stat -c '%U:%G' "${path}" 2>/dev/null)" || true
        if [[ "${actual_owner}" == "${expected_owner}" ]]; then
            return 0
        fi

        ownership_ok=false
        warn "${label}: owned by ${actual_owner} (expected ${expected_owner})"
        if try_fix "Set ownership of ${path} to root:root" chown root:root "${path}"; then
            return 0
        else
            printf "       → Fix: ${BOLD}chown root:root %s${RESET}\n" "${path}"
        fi
        return 1
    }

    # Check binary
    __check_ownership "${BINARY_DST}" "Binary" || warnings=$((warnings + 1))

    # Check config file
    __check_ownership "${CONFIG_FILE}" "Config" || warnings=$((warnings + 1))

    # Check data directories
    for dir_path in "${DATA_DIR}" "${LOG_DIR}" "${METRICS_DIR}"; do
        __check_ownership "${dir_path}" "Directory ${dir_path}" || warnings=$((warnings + 1))
    done

    # Check static assets directory
    __check_ownership "${STATIC_DST}" "Static assets" || warnings=$((warnings + 1))

    # Check migrations directory
    __check_ownership "${MIGRATIONS_DST}" "Migrations" || warnings=$((warnings + 1))

    # Check database file and WAL/SHM journals
    local db_check_file="${DATA_DIR}/ocserv_panel.db"
    if [[ -f "${CONFIG_FILE}" ]]; then
        local dsn_val=""
        dsn_val="$(grep -E '^\s*dsn\s*=' "${CONFIG_FILE}" \
            | head -1 | sed 's/^[^=]*=\s*//' | tr -d '"' | tr -d "'")" || true
        if [[ -n "${dsn_val}" ]]; then
            local dsn_path=""
            dsn_path="$(echo "${dsn_val}" | sed 's|^file:||' | sed 's|?.*||')" || true
            if [[ -n "${dsn_path}" && "${dsn_path}" == /* ]]; then
                db_check_file="${dsn_path}"
            fi
        fi
    fi
    __check_ownership "${db_check_file}" "Database" || warnings=$((warnings + 1))
    __check_ownership "${db_check_file}-wal" "Database WAL" || warnings=$((warnings + 1))
    __check_ownership "${db_check_file}-shm" "Database SHM" || warnings=$((warnings + 1))

    # Check systemd unit file
    __check_ownership "${SYSTEMD_UNIT}" "Systemd unit" || warnings=$((warnings + 1))

    if [[ "${ownership_ok}" == "true" ]]; then
        ok "All installation files owned by root:root"
    fi

    # ── Database health ──
    echo ""
    bold "Database health"
    echo ""

    # Determine the database path from config DSN
    local db_dsn="" db_file="${DATA_DIR}/ocserv_panel.db"
    if [[ -f "${CONFIG_FILE}" ]]; then
        db_dsn="$(grep -E '^\s*dsn\s*=' "${CONFIG_FILE}" \
            | head -1 | sed 's/^[^=]*=\s*//' | tr -d '"' | tr -d "'")" || true
    fi

    # Parse the actual file path from the DSN
    if [[ -n "${db_dsn}" ]]; then
        # DSN format: "file:/path/to/db?params" or just "/path/to/db"
        local parsed_path=""
        parsed_path="$(echo "${db_dsn}" | sed 's|^file:||' | sed 's|?.*||')" || true
        if [[ -n "${parsed_path}" ]]; then
            if [[ "${parsed_path}" == /* ]]; then
                db_file="${parsed_path}"
                ok "DSN uses absolute path: ${db_file}"
            else
                warn "DSN uses relative path: '${parsed_path}'"
                printf "       → Relative paths resolve from the service WorkingDirectory.\n"
                printf "       → If the service starts from a different directory, the DB won't be found.\n"
                printf "       → Recommended: use absolute path in config DSN:\n"
                printf "         ${BOLD}dsn = \"file:${DATA_DIR}/${parsed_path}?_journal=WAL&_busy_timeout=5000\"${RESET}\n"
                # Auto-fix: rewrite DSN to use absolute path
                local abs_dsn="file:${DATA_DIR}/${parsed_path}?_journal=WAL&_busy_timeout=5000"
                local abs_dsn_escaped
                abs_dsn_escaped="$(printf '%s' "${abs_dsn}" | sed 's|[&/]|\\&|g')"
                if try_fix "Rewrite DSN to absolute path in config" \
                    sed -i "s|^\(\s*dsn\s*=\s*\).*|\1\"${abs_dsn_escaped}\"|" "${CONFIG_FILE}"; then
                    db_file="${DATA_DIR}/${parsed_path}"
                else
                    db_file="${DATA_DIR}/${parsed_path}"
                fi
                warnings=$((warnings + 1))
            fi
        fi
    else
        warn "Could not read DSN from config — checking default path"
        warnings=$((warnings + 1))
    fi

    local db_dir
    db_dir="$(dirname "${db_file}")"

    # Check 1: Parent directory exists
    if [[ -d "${db_dir}" ]]; then
        ok "Database directory exists: ${db_dir}"
    else
        fail "Database directory missing: ${db_dir}"
        if ! try_fix "Create database directory" mkdir -p "${db_dir}"; then
            printf "       → Create: ${BOLD}mkdir -p %s${RESET}\n" "${db_dir}"
        fi
        errors=$((errors + 1))
    fi

    # Check 3: Database file itself
    if [[ -f "${db_file}" ]]; then
        local db_size db_owner db_perms
        db_size="$(du -h "${db_file}" 2>/dev/null | cut -f1)" || true
        db_owner="$(stat -c '%U:%G' "${db_file}" 2>/dev/null)" || true
        db_perms="$(stat -c '%a' "${db_file}" 2>/dev/null)" || true
        ok "Database file: ${db_file} (${db_size}, ${db_owner}, mode ${db_perms})"

        # sqlite3 integrity probe (if available)
        if command -v sqlite3 &>/dev/null; then
            local probe_result=""
            probe_result="$(sqlite3 "${db_file}" "SELECT count(*) FROM sqlite_master;" 2>&1)" || true
            if [[ "${probe_result}" =~ ^[0-9]+$ ]]; then
                ok "sqlite3 probe: ${probe_result} tables accessible"
            else
                warn "sqlite3 probe failed: ${probe_result}"
                printf "       → The database file may be corrupted or locked.\n"
                warnings=$((warnings + 1))
            fi
        fi
    else
        info "Database file not yet created: ${db_file}"
        printf "       → Will be created on first successful service start.\n"
    fi

    # Check 4: Disk space for database partition
    local db_free_mb=""
    db_free_mb="$(df -m "${db_dir}" 2>/dev/null | awk 'NR==2{print $4}')" || true
    if [[ -n "${db_free_mb}" ]]; then
        if [[ "${db_free_mb}" -lt 50 ]]; then
            fail "Low disk space on database partition: ${db_free_mb}MB free"
            printf "       → SQLite needs free space for WAL, transactions, and VACUUM.\n"
            errors=$((errors + 1))
        elif [[ "${db_free_mb}" -lt 200 ]]; then
            warn "Database partition has only ${db_free_mb}MB free"
            warnings=$((warnings + 1))
        else
            ok "Database partition: ${db_free_mb}MB free"
        fi
    fi

    # Check 5: Filesystem read-only check
    if [[ -d "${db_dir}" ]]; then
        local mount_info=""
        mount_info="$(findmnt -n -o OPTIONS --target "${db_dir}" 2>/dev/null)" || true
        if [[ -n "${mount_info}" ]] && echo ",${mount_info}," | grep -q ',ro,'; then
            fail "Database directory is on a READ-ONLY filesystem!"
            printf "       → SQLite cannot operate on a read-only filesystem.\n"
            printf "       → Check: ${BOLD}mount | grep '$(df "${db_dir}" 2>/dev/null | awk 'NR==2{print $1}')'${RESET}\n"
            errors=$((errors + 1))
        fi
    fi

    # ── Static assets ──
    echo ""
    bold "Static assets"
    echo ""
    if [[ -d "${STATIC_DST}" ]]; then
        local file_count
        file_count="$(find "${STATIC_DST}" -type f | wc -l)" || true
        ok "${STATIC_DST} (${file_count} files)"
        if [[ -f "${STATIC_DST}/index.html" ]]; then
            ok "SPA entry point: index.html present"
        else
            fail "index.html missing — frontend may not have been built correctly"
            errors=$((errors + 1))
        fi
    else
        fail "Static directory missing: ${STATIC_DST}"
        errors=$((errors + 1))
    fi

    # ── Migrations ──
    if [[ -d "${MIGRATIONS_DST}" ]]; then
        local sql_count
        sql_count="$(find "${MIGRATIONS_DST}" -name '*.sql' | wc -l)" || true
        ok "Migrations: ${MIGRATIONS_DST} (${sql_count} SQL files)"
    else
        fail "Migrations directory missing: ${MIGRATIONS_DST}"
        errors=$((errors + 1))
    fi

    # ── TLS Certificates ──
    echo ""
    bold "TLS certificates"
    echo ""

    # Read cert paths from the installed config if available
    local tls_cert="" tls_key=""
    if [[ -f "${CONFIG_FILE}" ]]; then
        tls_cert="$(grep -E '^\s*tls_cert_path\s*=' "${CONFIG_FILE}" \
            | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'")" || true
        tls_key="$(grep -E '^\s*tls_key_path\s*=' "${CONFIG_FILE}" \
            | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'")" || true
    fi

    if [[ -z "${tls_cert}" && -z "${tls_key}" ]]; then
        info "TLS not configured — running in HTTP mode"
    else
        # Check cert files exist
        if [[ -n "${tls_cert}" && -f "${tls_cert}" ]]; then
            ok "Certificate file: ${tls_cert}"
            # Check expiry
            if command -v openssl &>/dev/null; then
                local expiry
                expiry="$(openssl x509 -in "${tls_cert}" -noout -enddate 2>/dev/null \
                    | sed 's/notAfter=//')" || true
                if [[ -n "${expiry}" ]]; then
                    # Check if expired
                    if openssl x509 -in "${tls_cert}" -noout -checkend 0 &>/dev/null; then
                        ok "Certificate valid until: ${expiry}"
                        # Warn if expiring in 14 days
                        if ! openssl x509 -in "${tls_cert}" -noout -checkend 1209600 &>/dev/null; then
                            warn "Certificate expires within 14 days!"
                            printf "       → Check renewal: ${BOLD}certbot certificates${RESET}\n"
                            warnings=$((warnings + 1))
                        fi
                    else
                        fail "Certificate is EXPIRED (${expiry})"
                        printf "       → Renew: ${BOLD}certbot renew${RESET}\n"
                        errors=$((errors + 1))
                    fi
                fi
            fi
        elif [[ -n "${tls_cert}" ]]; then
            fail "Certificate file not found: ${tls_cert}"
            errors=$((errors + 1))
        fi

        if [[ -n "${tls_key}" && -f "${tls_key}" ]]; then
            ok "Key file: ${tls_key}"
        elif [[ -n "${tls_key}" ]]; then
            fail "Key file not found: ${tls_key}"
            errors=$((errors + 1))
        fi
    fi

    # ── ocserv integration ──
    echo ""
    bold "ocserv integration"
    echo ""

    # Read paths from config
    local ocserv_bin="" occtl_bin="" ocpasswd_bin="" ocserv_conf="" ocserv_socket=""
    if [[ -f "${CONFIG_FILE}" ]]; then
        ocserv_bin="$(grep -E '^\s*ocserv_bin_path\s*=' "${CONFIG_FILE}" \
            | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'")" || true
        occtl_bin="$(grep -E '^\s*occtl_bin_path\s*=' "${CONFIG_FILE}" \
            | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'")" || true
        ocpasswd_bin="$(grep -E '^\s*ocpasswd_bin_path\s*=' "${CONFIG_FILE}" \
            | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'")" || true
        ocserv_conf="$(grep -E '^\s*ocserv_config_path\s*=' "${CONFIG_FILE}" \
            | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'")" || true
        ocserv_socket="$(grep -E '^\s*ocserv_socket_path\s*=' "${CONFIG_FILE}" \
            | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'")" || true
    fi

    # Binaries
    for bin_pair in "ocserv:${ocserv_bin}" "occtl:${occtl_bin}" "ocpasswd:${ocpasswd_bin}"; do
        local bin_name="${bin_pair%%:*}"
        local bin_path="${bin_pair#*:}"
        if [[ -n "${bin_path}" && -x "${bin_path}" ]]; then
            ok "${bin_name}: ${bin_path}"
        elif [[ -n "${bin_path}" ]]; then
            fail "${bin_name}: ${bin_path} (not found or not executable)"
            errors=$((errors + 1))
        fi
    done

    # Config file
    if [[ -n "${ocserv_conf}" && -f "${ocserv_conf}" ]]; then
        ok "ocserv config: ${ocserv_conf}"
    elif [[ -n "${ocserv_conf}" ]]; then
        fail "ocserv config not found: ${ocserv_conf}"
        errors=$((errors + 1))
    fi

    # Socket
    if [[ -n "${ocserv_socket}" ]]; then
        if [[ -S "${ocserv_socket}" ]]; then
            ok "ocserv socket: ${ocserv_socket}"
        else
            warn "ocserv socket not found: ${ocserv_socket}"
            printf "       → Is ocserv running? Check: ${BOLD}systemctl status ocserv${RESET}\n"
            warnings=$((warnings + 1))
        fi
    fi

    # ocserv service status
    if command -v systemctl &>/dev/null; then
        if systemctl is-active --quiet ocserv 2>/dev/null; then
            ok "ocserv service: running"
        else
            warn "ocserv service: not running"
            printf "       → Start: ${BOLD}systemctl start ocserv${RESET}\n"
            warnings=$((warnings + 1))
        fi
    fi

    # ── Health check ──
    echo ""
    bold "Health check"
    echo ""
    if command -v curl &>/dev/null; then
        local scheme="http"
        if [[ -n "${tls_cert}" && -n "${tls_key}" ]]; then
            scheme="https"
        fi
        local health_url="${scheme}://127.0.0.1:${PANEL_PORT}/api/health"
        local http_code
        http_code="$(curl -sk -o /dev/null -w '%{http_code}' \
            --connect-timeout 3 --max-time 5 "${health_url}" 2>/dev/null)" || http_code="000"
        if [[ "${http_code}" == "200" ]]; then
            ok "Health endpoint: ${health_url} → 200 OK"
        elif [[ "${http_code}" == "000" ]]; then
            fail "Health endpoint: connection refused (${health_url})"
            printf "       → Service may not be running or not listening on port ${PANEL_PORT}\n"
            printf "       → Check: ${BOLD}systemctl status ocserv-panel${RESET}\n"
            errors=$((errors + 1))
        else
            warn "Health endpoint: HTTP ${http_code} (${health_url})"
            printf "       → Expected 200 OK. Check service logs.\n"
            warnings=$((warnings + 1))
        fi
    else
        info "curl not available — cannot perform health check"
    fi

    # ── Summary ──
    echo ""
    printf "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
    if [[ "${errors}" -eq 0 && "${warnings}" -eq 0 ]]; then
        printf "${GREEN}  Status: All checks passed ✓${RESET}\n"
    elif [[ "${errors}" -eq 0 ]]; then
        printf "${YELLOW}  Status: OK with %d warning(s)${RESET}\n" "${warnings}"
        if [[ "${FIX_MODE}" != "true" ]]; then
            printf "  Run ${BOLD}--fix${RESET} to attempt automatic repair.\n"
        fi
    else
        printf "${RED}  Status: %d error(s), %d warning(s)${RESET}\n" "${errors}" "${warnings}"
        if [[ "${FIX_MODE}" != "true" ]]; then
            printf "  Run ${BOLD}--fix${RESET} to attempt automatic repair.\n"
        fi
    fi
    printf "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
    echo ""

    # Return meaningful codes: errors → 1, warnings-only → 2, clean → 0
    if [[ "${errors}" -gt 0 ]]; then
        return 1
    elif [[ "${warnings}" -gt 0 ]]; then
        return 2
    fi
    return 0
}

# ─── Uninstall ────────────────────────────────────────────────────────────────

uninstall() {
    header

    bold "Uninstall ocserv-panel"
    echo ""
    warn "This will remove the ocserv-panel service, binary, and shared files."
    echo ""

    if ! confirm "Continue with uninstall?" "n"; then
        echo "Aborted."
        exit 0
    fi

    echo ""

    # Stop and disable service
    info "Stopping and disabling service..."
    if systemctl is-active --quiet ocserv-panel 2>/dev/null; then
        systemctl stop ocserv-panel && ok "Service stopped"
    else
        ok "Service was not running"
    fi
    if systemctl is-enabled --quiet ocserv-panel 2>/dev/null; then
        systemctl disable ocserv-panel 2>/dev/null && ok "Service disabled"
    fi

    # Remove systemd unit
    if [[ -f "${SYSTEMD_UNIT}" ]]; then
        rm -f "${SYSTEMD_UNIT}"
        systemctl daemon-reload
        ok "Systemd unit removed"
    fi

    # Remove binary
    if [[ -f "${BINARY_DST}" ]]; then
        rm -f "${BINARY_DST}"
        ok "Binary removed: ${BINARY_DST}"
    fi

    # Remove shared files
    if [[ -d "${SHARE_DIR}" ]]; then
        rm -rf "${SHARE_DIR}"
        ok "Shared files removed: ${SHARE_DIR}"
    fi

    echo ""
    if confirm "Remove configuration directory ${CONFIG_DIR}?" "n"; then
        rm -rf "${CONFIG_DIR}"
        ok "Config directory removed"
    else
        ok "Config preserved at ${CONFIG_DIR}"
    fi

    echo ""
    if confirm "Remove data directory ${DATA_DIR} (SQLite database and metrics)?" "n"; then
        rm -rf "${DATA_DIR}"
        ok "Data directory removed"
    else
        ok "Data preserved at ${DATA_DIR}"
    fi

    echo ""
    if confirm "Remove log directory ${LOG_DIR}?" "n"; then
        rm -rf "${LOG_DIR}"
        ok "Log directory removed"
    else
        ok "Logs preserved at ${LOG_DIR}"
    fi

    echo ""
    ok "Uninstall complete."
}

# ─── Main install flow ────────────────────────────────────────────────────────

main() {
    local arg="${1:-}"

    case "${arg}" in
        --help|-h)
            usage
            exit 0
            ;;
        --status|--check)
            if [[ "${EUID}" -ne 0 ]]; then
                die "Must run as root (use sudo)"
            fi
            # status_check returns: 0=healthy, 1=errors, 2=warnings-only
            local sc=0
            status_check || sc=$?
            exit "${sc}"
            ;;
        --healthcheck)
            # Lightweight health endpoint check for monitoring/cron
            read_port_from_config
            if ! command -v curl &>/dev/null; then
                echo "curl not available" >&2
                exit 1
            fi
            local scheme="http"
            if [[ -f "${CONFIG_FILE}" ]]; then
                local tls_cert_val=""
                tls_cert_val="$(grep -E '^\s*tls_cert_path\s*=' "${CONFIG_FILE}" \
                    | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'")" || true
                if [[ -n "${tls_cert_val}" ]]; then
                    scheme="https"
                fi
            fi
            local health_url="${scheme}://127.0.0.1:${PANEL_PORT}/api/health"
            local http_code
            http_code="$(curl -sk -o /dev/null -w '%{http_code}' \
                --connect-timeout 3 --max-time 5 "${health_url}" 2>/dev/null)" || http_code="000"
            if [[ "${http_code}" == "200" ]]; then
                echo "OK"
                exit 0
            else
                echo "FAIL (HTTP ${http_code})"
                exit 1
            fi
            ;;
        --fix)
            if [[ "${EUID}" -ne 0 ]]; then
                die "Must run as root (use sudo)"
            fi
            FIX_MODE="true"
            # status_check returns non-zero on errors — we handle our own exit below
            status_check || true  # ignore return; fix mode uses FIXES_* counters
            echo ""
            if [[ "${FIXES_APPLIED}" -gt 0 || "${FIXES_FAILED}" -gt 0 ]]; then
                printf "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
                printf "  Fix summary: ${GREEN}%d applied${RESET}, ${RED}%d failed${RESET}\n" "${FIXES_APPLIED}" "${FIXES_FAILED}"
                printf "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
                echo ""
                if [[ "${FIXES_FAILED}" -gt 0 ]]; then
                    printf "  Some issues could not be fixed automatically.\n"
                    printf "  Review the output above for manual steps.\n"
                    echo ""
                else
                    printf "  All fixable issues have been resolved.\n"
                    printf "  Run ${BOLD}--status${RESET} again to verify.\n"
                    echo ""
                fi
                # Restart service if fixes were applied and service is running
                if [[ "${FIXES_APPLIED}" -gt 0 ]]; then
                    if systemctl is-active --quiet ocserv-panel 2>/dev/null; then
                        info "Restarting service to apply fixes..."
                        if systemctl restart ocserv-panel; then
                            ok "Service restarted successfully"
                        else
                            fail "Service restart failed after fixes"
                        fi
                    fi
                fi
            else
                ok "No fixes were needed — installation looks healthy."
                echo ""
            fi
            if [[ "${FIXES_FAILED}" -gt 0 ]]; then
                exit 1
            fi
            exit 0
            ;;
        --uninstall)
            if [[ "${EUID}" -ne 0 ]]; then
                die "Must run as root (use sudo)"
            fi
            uninstall
            exit 0
            ;;
        "")
            ;;
        *)
            echo "Unknown option: ${arg}" >&2
            usage >&2
            exit 2
            ;;
    esac

    # Must be root for install
    if [[ "${EUID}" -ne 0 ]]; then
        die "Must run as root (use sudo)"
    fi

    header

    # Phase 1: Detect
    detect_ocserv
    detect_existing

    local mode="install"
    if [[ -n "${EXISTING_VERSION}" ]]; then
        mode="upgrade"
    fi

    # Phase 2: Preflight
    preflight

    # Phase 3: Plan and confirm
    show_plan "${mode}"

    if ! confirm "Proceed with ${mode}?" "y"; then
        echo "Aborted."
        exit 0
    fi
    echo ""

    # Phase 4: Execute
    if [[ "${mode}" == "upgrade" ]]; then
        run_step "Stop existing service" do_stop_existing
        run_step "Backup data" do_backup_data
    fi

    run_step "Install binary" do_install_binary
    run_step "Install static assets" do_install_static
    run_step "Install migrations" do_install_migrations
    run_step "Generate configuration" do_generate_config
    do_check_config_keys
    run_step "Install systemd unit" do_install_systemd
    run_step "Start service" do_start_service

    # Phase 5: Verify
    echo ""
    do_verify_health
    success_banner
}

main "$@"
