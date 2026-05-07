#!/usr/bin/env bash
# install.sh — ocserv-panel interactive installer
#
# Usage:
#   sudo ./install.sh              # fresh install or upgrade
#   sudo ./install.sh --uninstall  # remove installation
#   sudo ./install.sh --help       # show usage
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
#   Sudoers file    : /etc/sudoers.d/ocserv-panel
#   Service user    : ocserv-panel (system, no login shell)
#   Health endpoint : http://127.0.0.1:${PANEL_PORT}/api/health

set -euo pipefail

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
SUDOERS_FILE="/etc/sudoers.d/ocserv-panel"
SERVICE_USER="ocserv-panel"
SERVICE_GROUP="ocserv-panel"
PANEL_PORT="8080"

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

ok()   { printf "${GREEN}  [ok]${RESET} %s\n" "$*"; }
fail() { printf "${RED}  [fail]${RESET} %s\n" "$*"; }
warn() { printf "${YELLOW}  [warn]${RESET} %s\n" "$*"; }
info() { printf "${BLUE}  [->]${RESET} %s\n" "$*"; }
bold() { printf "${BOLD}%s${RESET}\n" "$*"; }
die()  { printf "${RED}[error]${RESET} %s\n" "$*" >&2; exit 1; }

# ─── Usage ────────────────────────────────────────────────────────────────────

usage() {
    cat <<EOF
ocserv-panel installer

Usage:
  sudo $0              # fresh install or upgrade
  sudo $0 --uninstall  # remove installation
  sudo $0 --help       # show this help

The installer:
  - Detects your ocserv configuration automatically
  - Creates the ocserv-panel system user
  - Installs binary, static assets, and migrations
  - Generates /etc/ocserv-panel/config.toml (skipped on upgrade if file exists)
  - Installs sudoers rules with detected binary paths
  - Installs and starts the systemd service
  - Verifies the health endpoint

Re-running the installer on an existing installation performs an upgrade:
  existing data and configuration are preserved and backed up before changes.
EOF
}

# ─── Banner ───────────────────────────────────────────────────────────────────

header() {
    local version="unknown"
    [[ -f "${VERSION_FILE}" ]] && version="$(cat "${VERSION_FILE}")"
    echo ""
    bold "=========================================="
    bold "  ocserv-panel Installer  v${version}"
    bold "=========================================="
    echo ""
}

# ─── Version helper ───────────────────────────────────────────────────────────

read_version() {
    local version="unknown"
    [[ -f "${VERSION_FILE}" ]] && version="$(cat "${VERSION_FILE}")"
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
    [[ -z "${DETECTED_OCSERV_BIN}" ]] && \
        DETECTED_OCSERV_BIN="$(command -v ocserv 2>/dev/null || true)"

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
    [[ -z "${DETECTED_OCCTL_BIN}" ]] && \
        DETECTED_OCCTL_BIN="$(command -v occtl 2>/dev/null || true)"

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
    [[ -z "${DETECTED_OCPASSWD_BIN}" ]] && \
        DETECTED_OCPASSWD_BIN="$(command -v ocpasswd 2>/dev/null || true)"

    # Config file — try to find it from the running unit first
    DETECTED_OCSERV_CONF=""
    if command -v systemctl &>/dev/null; then
        local exec_start
        exec_start="$(systemctl show ocserv --property=ExecStart 2>/dev/null | grep -oP '(?<=path=)[^ ;]+' | head -1 || true)"
        if [[ -n "${exec_start}" ]]; then
            # Extract -c <configfile> argument
            local unit_conf
            unit_conf="$(systemctl show ocserv --property=ExecStart 2>/dev/null \
                | grep -oP '(?<=-c )\S+' | head -1 || true)"
            [[ -n "${unit_conf}" && -f "${unit_conf}" ]] && \
                DETECTED_OCSERV_CONF="${unit_conf}"
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

        # socket-file = <path>
        DETECTED_OCSERV_SOCKET="$(grep -E '^\s*socket-file\s*=' "${conf}" \
            | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'" || true)"
        [[ -z "${DETECTED_OCSERV_SOCKET}" ]] && \
            DETECTED_OCSERV_SOCKET="/var/run/ocserv-socket"

        # plain[passwd=<path>] in auth directive
        DETECTED_OCPASSWD_FILE="$(grep -E '^\s*auth\s*=' "${conf}" \
            | grep -oP '(?<=passwd=)[^]]+' | head -1 | tr -d '"' | tr -d "'" || true)"
        [[ -z "${DETECTED_OCPASSWD_FILE}" ]] && \
            DETECTED_OCPASSWD_FILE="/etc/ocserv/ocpasswd"

        # server-cert = <path>
        DETECTED_CERT_PATH="$(grep -E '^\s*server-cert\s*=' "${conf}" \
            | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'" || true)"
        if [[ -n "${DETECTED_CERT_PATH}" && ! "${DETECTED_CERT_PATH}" = /* ]]; then
            # Relative path — resolve from config dir
            local conf_dir
            conf_dir="$(dirname "${DETECTED_OCSERV_CONF}")"
            DETECTED_CERT_PATH="${conf_dir}/${DETECTED_CERT_PATH}"
        fi

        # server-key = <path>
        DETECTED_KEY_PATH="$(grep -E '^\s*server-key\s*=' "${conf}" \
            | head -1 | sed 's/.*=\s*//' | tr -d '"' | tr -d "'" || true)"
        if [[ -n "${DETECTED_KEY_PATH}" && ! "${DETECTED_KEY_PATH}" = /* ]]; then
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
    [[ -z "${DETECTED_OCSERV_CONF}" ]]      && DETECTED_OCSERV_CONF="/etc/ocserv/ocserv.conf"
    [[ -z "${DETECTED_OCSERV_SOCKET}" ]]    && DETECTED_OCSERV_SOCKET="/var/run/ocserv-socket"
    [[ -z "${DETECTED_OCPASSWD_FILE}" ]]    && DETECTED_OCPASSWD_FILE="/etc/ocserv/ocpasswd"
    [[ -z "${DETECTED_CERT_PATH}" ]]        && DETECTED_CERT_PATH=""
    [[ -z "${DETECTED_KEY_PATH}" ]]         && DETECTED_KEY_PATH=""
    [[ -z "${DETECTED_CERT_DIR}" ]]         && DETECTED_CERT_DIR="/etc/letsencrypt/live"
}

# ─── Detect existing installation ─────────────────────────────────────────────

detect_existing() {
    EXISTING_VERSION=""
    if [[ -x "${BINARY_DST}" ]]; then
        EXISTING_VERSION="$(${BINARY_DST} --version 2>/dev/null | awk '{print $NF}' || true)"
        [[ -z "${EXISTING_VERSION}" ]] && EXISTING_VERSION="unknown"
    fi
}

# ─── Preflight checks ─────────────────────────────────────────────────────────

preflight() {
    bold "Preflight checks"
    echo ""
    local ok=true

    # Running as root
    if [[ "${EUID}" -eq 0 ]]; then
        ok "Running as root"
    else
        fail "Must run as root (use sudo)"
        ok=false
    fi

    # systemd available
    if command -v systemctl &>/dev/null && systemctl list-units &>/dev/null 2>&1; then
        ok "systemd is available"
    else
        fail "systemd not found or not running"
        ok=false
    fi

    # Artifacts present
    if [[ -f "${BINARY_SRC}" && -x "${BINARY_SRC}" ]]; then
        ok "Binary artifact found: ${BINARY_SRC}"
    else
        fail "Binary not found or not executable: ${BINARY_SRC}"
        ok=false
    fi

    if [[ -d "${STATIC_SRC}" ]]; then
        ok "Static assets found: ${STATIC_SRC}"
    else
        fail "Static assets directory not found: ${STATIC_SRC}"
        ok=false
    fi

    if [[ -d "${MIGRATIONS_SRC}" ]]; then
        ok "Migrations directory found: ${MIGRATIONS_SRC}"
    else
        fail "Migrations directory not found: ${MIGRATIONS_SRC}"
        ok=false
    fi

    if [[ -f "${VERSION_FILE}" ]]; then
        ok "VERSION file found: $(cat "${VERSION_FILE}")"
    else
        warn "VERSION file not found at ${VERSION_FILE}, will use 'unknown'"
    fi

    # ocserv detection summary
    if [[ -n "${DETECTED_OCSERV_BIN}" ]]; then
        ok "ocserv binary: ${DETECTED_OCSERV_BIN}"
    else
        warn "ocserv binary not found — install paths will use defaults"
    fi

    if [[ -n "${DETECTED_OCCTL_BIN}" ]]; then
        ok "occtl binary: ${DETECTED_OCCTL_BIN}"
    else
        warn "occtl binary not found — sudoers rule will use /usr/bin/occtl"
    fi

    if [[ -n "${DETECTED_OCPASSWD_BIN}" ]]; then
        ok "ocpasswd binary: ${DETECTED_OCPASSWD_BIN}"
    else
        warn "ocpasswd binary not found — sudoers rule will use /usr/bin/ocpasswd"
    fi

    if [[ -n "${DETECTED_OCSERV_CONF}" && -f "${DETECTED_OCSERV_CONF}" ]]; then
        ok "ocserv config: ${DETECTED_OCSERV_CONF}"
    else
        warn "ocserv config not found at ${DETECTED_OCSERV_CONF} — using default path"
    fi

    # visudo available for sudoers validation
    if command -v visudo &>/dev/null; then
        ok "visudo available for sudoers validation"
    else
        warn "visudo not found — sudoers file will be installed without syntax check"
    fi

    # curl available for health check
    if command -v curl &>/dev/null; then
        ok "curl available for health check"
    else
        warn "curl not found — post-install health check will be skipped"
    fi

    echo ""
    if [[ "${ok}" != "true" ]]; then
        die "Preflight failed. Correct the errors above and re-run."
    fi
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
    echo "    ocserv binary  : ${DETECTED_OCSERV_BIN:-/usr/sbin/ocserv (default)}"
    echo "    occtl binary   : ${DETECTED_OCCTL_BIN:-/usr/bin/occtl (default)}"
    echo "    ocpasswd binary: ${DETECTED_OCPASSWD_BIN:-/usr/bin/ocpasswd (default)}"
    echo "    config file    : ${DETECTED_OCSERV_CONF}"
    echo "    socket         : ${DETECTED_OCSERV_SOCKET}"
    echo "    password file  : ${DETECTED_OCPASSWD_FILE}"
    [[ -n "${DETECTED_CERT_PATH}" ]] && \
        echo "    server-cert    : ${DETECTED_CERT_PATH}"
    echo ""
    echo "  Steps:"
    local n=1
    [[ "${mode}" == "upgrade" ]] && { echo "  ${n}. Stop existing service"; (( n++ )); }
    [[ "${mode}" == "upgrade" ]] && { echo "  ${n}. Backup data (DB + config)"; (( n++ )); }
    echo "  ${n}. Create system user '${SERVICE_USER}'"; (( n++ ))
    echo "  ${n}. Install binary → ${BINARY_DST}"; (( n++ ))
    echo "  ${n}. Install static assets → ${STATIC_DST}"; (( n++ ))
    echo "  ${n}. Install migrations → ${MIGRATIONS_DST}"; (( n++ ))
    if [[ "${mode}" == "upgrade" && -f "${CONFIG_FILE}" ]]; then
        echo "  ${n}. Config preserved (upgrade) → ${CONFIG_FILE}"; (( n++ ))
    else
        echo "  ${n}. Generate config → ${CONFIG_FILE}"; (( n++ ))
    fi
    echo "  ${n}. Install sudoers → ${SUDOERS_FILE}"; (( n++ ))
    echo "  ${n}. Verify certificate read access for ${SERVICE_USER}"; (( n++ ))
    echo "  ${n}. Install systemd unit → ${SYSTEMD_UNIT}"; (( n++ ))
    echo "  ${n}. Enable and start service"; (( n++ ))
    echo "  ${n}. Verify health endpoint"; (( n++ ))
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
            *)     echo "Please answer y or n." ;;
        esac
    done
}

# ─── Stop existing service ────────────────────────────────────────────────────

stop_existing() {
    info "Stopping existing service..."
    if systemctl is-active --quiet ocserv-panel 2>/dev/null; then
        systemctl stop ocserv-panel
        ok "Service stopped"
    else
        ok "Service was not running"
    fi
}

# ─── Backup data ──────────────────────────────────────────────────────────────

backup_data() {
    local backup_dir="/var/backups/ocserv-panel"
    local timestamp
    timestamp="$(date +%Y%m%dT%H%M%S)"
    local backup_file="${backup_dir}/backup-${EXISTING_VERSION}-${timestamp}.tar.gz"

    info "Backing up data to ${backup_file}..."
    mkdir -p "${backup_dir}"

    local items=()
    [[ -f "${DATA_DIR}/ocserv_panel.db" ]]   && items+=("${DATA_DIR}/ocserv_panel.db")
    [[ -f "${DATA_DIR}/ocserv_panel.db-wal" ]] && items+=("${DATA_DIR}/ocserv_panel.db-wal")
    [[ -f "${CONFIG_FILE}" ]]                 && items+=("${CONFIG_FILE}")

    if [[ ${#items[@]} -gt 0 ]]; then
        tar -czf "${backup_file}" "${items[@]}" 2>/dev/null
        ok "Backup written: ${backup_file}"
    else
        ok "No existing data to back up"
    fi
}

# ─── Create system user ───────────────────────────────────────────────────────

create_user() {
    info "Creating system user '${SERVICE_USER}'..."
    if id "${SERVICE_USER}" &>/dev/null; then
        ok "User '${SERVICE_USER}' already exists"
    else
        useradd \
            --system \
            --no-create-home \
            --shell /usr/sbin/nologin \
            --comment "ocserv Panel service account" \
            "${SERVICE_USER}"
        ok "User '${SERVICE_USER}' created"
    fi
}

# ─── Install binary ───────────────────────────────────────────────────────────

install_binary() {
    info "Installing binary → ${BINARY_DST}..."
    install -o root -g root -m 755 "${BINARY_SRC}" "${BINARY_DST}"
    ok "Binary installed"
}

# ─── Install static assets ────────────────────────────────────────────────────

install_static() {
    info "Installing static assets → ${STATIC_DST}..."
    mkdir -p "${STATIC_DST}"
    # Use rsync if available for efficiency; fall back to cp
    if command -v rsync &>/dev/null; then
        rsync -a --delete "${STATIC_SRC}/" "${STATIC_DST}/"
    else
        rm -rf "${STATIC_DST}"
        cp -r "${STATIC_SRC}" "${STATIC_DST}"
    fi
    chown -R "root:root" "${STATIC_DST}"
    ok "Static assets installed"
}

# ─── Install migrations ───────────────────────────────────────────────────────

install_migrations() {
    info "Installing migrations → ${MIGRATIONS_DST}..."
    mkdir -p "${MIGRATIONS_DST}"
    if command -v rsync &>/dev/null; then
        rsync -a --delete "${MIGRATIONS_SRC}/" "${MIGRATIONS_DST}/"
    else
        rm -rf "${MIGRATIONS_DST}"
        cp -r "${MIGRATIONS_SRC}" "${MIGRATIONS_DST}"
    fi
    chown -R "root:root" "${MIGRATIONS_DST}"
    ok "Migrations installed"
}

# ─── Generate config.toml ─────────────────────────────────────────────────────

generate_config() {
    if [[ -f "${CONFIG_FILE}" ]]; then
        ok "Config file already exists — preserving: ${CONFIG_FILE}"
        return 0
    fi

    info "Generating config → ${CONFIG_FILE}..."
    mkdir -p "${CONFIG_DIR}"

    local hostname
    hostname="$(hostname -f 2>/dev/null || hostname)"
    local ocserv_bin="${DETECTED_OCSERV_BIN:-/usr/sbin/ocserv}"
    local occtl_bin="${DETECTED_OCCTL_BIN:-/usr/bin/occtl}"
    local ocpasswd_bin="${DETECTED_OCPASSWD_BIN:-/usr/bin/ocpasswd}"
    local ocserv_conf="${DETECTED_OCSERV_CONF:-/etc/ocserv/ocserv.conf}"
    local ocserv_socket="${DETECTED_OCSERV_SOCKET:-/var/run/ocserv-socket}"
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
user = "${SERVICE_USER}"
group = "${SERVICE_GROUP}"
start_on_boot = true
systemd_unit_name = "ocserv"

[web]
host    = "0.0.0.0"
port    = ${PANEL_PORT}
base_url = "http://${hostname}:${PANEL_PORT}"

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

    chown "root:${SERVICE_GROUP}" "${CONFIG_FILE}"
    chmod 640 "${CONFIG_FILE}"
    ok "Config written: ${CONFIG_FILE}"
}

# ─── Install sudoers ──────────────────────────────────────────────────────────

install_sudoers() {
    info "Generating sudoers rules → ${SUDOERS_FILE}..."

    # Resolve full paths for every command in sudoers.
    # Using detected paths falls back to command -v fallbacks.
    local systemctl_bin
    systemctl_bin="$(command -v systemctl 2>/dev/null || echo /usr/bin/systemctl)"
    local occtl_bin="${DETECTED_OCCTL_BIN:-$(command -v occtl 2>/dev/null || echo /usr/bin/occtl)}"
    local ocpasswd_bin="${DETECTED_OCPASSWD_BIN:-$(command -v ocpasswd 2>/dev/null || echo /usr/bin/ocpasswd)}"
    local ocpasswd_file="${DETECTED_OCPASSWD_FILE:-/etc/ocserv/ocpasswd}"

    # Temporary file used for visudo validation
    local tmp_sudoers
    tmp_sudoers="$(mktemp)"
    # shellcheck disable=SC2064
    trap "rm -f '${tmp_sudoers}'" EXIT

    cat > "${tmp_sudoers}" <<EOF
# Sudoers rules for the ocserv-panel service account
# Generated by installer on $(date -u '+%Y-%m-%dT%H:%M:%SZ')
# Managed file — do not edit by hand.

# ocserv service lifecycle
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_bin} start ocserv
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_bin} stop ocserv
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_bin} restart ocserv
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_bin} reload ocserv
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_bin} status ocserv
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_bin} is-active ocserv
${SERVICE_USER} ALL=(root) NOPASSWD: ${systemctl_bin} is-enabled ocserv

# Session and status queries via occtl
${SERVICE_USER} ALL=(root) NOPASSWD: ${occtl_bin} disconnect id *
${SERVICE_USER} ALL=(root) NOPASSWD: ${occtl_bin} show users
${SERVICE_USER} ALL=(root) NOPASSWD: ${occtl_bin} show user *
${SERVICE_USER} ALL=(root) NOPASSWD: ${occtl_bin} show status
${SERVICE_USER} ALL=(root) NOPASSWD: ${occtl_bin} reload

# Password file management via ocpasswd
${SERVICE_USER} ALL=(root) NOPASSWD: ${ocpasswd_bin} -c ${ocpasswd_file} *
${SERVICE_USER} ALL=(root) NOPASSWD: ${ocpasswd_bin} -c ${ocpasswd_file} -d *
${SERVICE_USER} ALL=(root) NOPASSWD: ${ocpasswd_bin} -c ${ocpasswd_file} -l *
${SERVICE_USER} ALL=(root) NOPASSWD: ${ocpasswd_bin} -c ${ocpasswd_file} -u *
EOF

    # Validate with visudo if available
    if command -v visudo &>/dev/null; then
        if visudo -c -f "${tmp_sudoers}" &>/dev/null; then
            install -o root -g root -m 440 "${tmp_sudoers}" "${SUDOERS_FILE}"
            ok "Sudoers file installed and validated: ${SUDOERS_FILE}"
        else
            warn "visudo validation failed — sudoers file NOT installed"
            warn "Inspect ${tmp_sudoers} to diagnose"
            trap - EXIT
            return 1
        fi
    else
        install -o root -g root -m 440 "${tmp_sudoers}" "${SUDOERS_FILE}"
        warn "visudo not available — sudoers installed without syntax check"
    fi
    trap - EXIT
    rm -f "${tmp_sudoers}"
}

# ─── Verify certificate access ────────────────────────────────────────────────

verify_cert_access() {
    info "Verifying certificate read access for '${SERVICE_USER}'..."

    local cert_path="${DETECTED_CERT_PATH}"
    local key_path="${DETECTED_KEY_PATH}"

    if [[ -z "${cert_path}" && -z "${key_path}" ]]; then
        ok "No TLS cert paths detected — skipping cert access check"
        return 0
    fi

    local access_ok=true

    for cert_file in "${cert_path}" "${key_path}"; do
        [[ -z "${cert_file}" ]] && continue
        if [[ ! -e "${cert_file}" ]]; then
            warn "Cert/key file not found: ${cert_file}"
            continue
        fi
        # Check if SERVICE_USER can read the file
        if su -s /bin/sh "${SERVICE_USER}" -c "test -r '${cert_file}'" 2>/dev/null; then
            ok "'${SERVICE_USER}' can read: ${cert_file}"
        else
            warn "'${SERVICE_USER}' cannot read: ${cert_file}"
            access_ok=false
        fi
    done

    if [[ "${access_ok}" == "false" ]]; then
        warn "Certificate access issue detected."
        warn "To grant read access without changing ownership, use setfacl:"
        local dir="${DETECTED_CERT_DIR:-$(dirname "${cert_path}")}"
        cat <<EOF

    # Grant recursive read access on the cert directory:
    setfacl -R -m u:${SERVICE_USER}:rX "${dir}"
    setfacl -R -d -m u:${SERVICE_USER}:rX "${dir}"

    # Or for Let's Encrypt, add the user to the certbot group:
    usermod -aG ssl-cert ${SERVICE_USER}

EOF
        warn "The service will start but TLS may not work until cert access is fixed."
    fi
}

# ─── Install systemd unit ─────────────────────────────────────────────────────

install_systemd() {
    info "Installing systemd unit → ${SYSTEMD_UNIT}..."

    local tls_cert="${DETECTED_CERT_PATH:-}"
    local tls_key="${DETECTED_KEY_PATH:-}"
    local cert_dir="${DETECTED_CERT_DIR:-/etc/letsencrypt/live}"
    local ocserv_conf_dir
    ocserv_conf_dir="$(dirname "${DETECTED_OCSERV_CONF:-/etc/ocserv/ocserv.conf}")"

    # Build ReadOnlyPaths list — include cert dir only if it exists
    local ro_paths="${CONFIG_DIR} ${ocserv_conf_dir} ${SHARE_DIR}"
    [[ -d "${cert_dir}" ]] && ro_paths="${ro_paths} ${cert_dir}"

    # Build optional TLS env lines
    local tls_env_lines=""
    if [[ -n "${tls_cert}" ]]; then
        tls_env_lines="Environment=\"OCSERV_PANEL_TLS_CERT=${tls_cert}\"\nEnvironment=\"OCSERV_PANEL_TLS_KEY=${tls_key}\""
    fi

    # Create data and log directories
    mkdir -p "${DATA_DIR}" "${LOG_DIR}" "${METRICS_DIR}"
    chown "${SERVICE_USER}:${SERVICE_GROUP}" "${DATA_DIR}" "${LOG_DIR}" "${METRICS_DIR}"
    chmod 750 "${DATA_DIR}" "${LOG_DIR}" "${METRICS_DIR}"

    cat > "${SYSTEMD_UNIT}" <<EOF
[Unit]
Description=ocserv Panel - Web UI for OpenConnect VPN Administration
After=network-online.target ocserv.service
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
WorkingDirectory=${DATA_DIR}

ExecStart=${BINARY_DST}

Environment="OCSERV_PANEL_ADDR=0.0.0.0:${PANEL_PORT}"
Environment="OCSERV_PANEL_DB_DSN=file:${DATA_DIR}/ocserv_panel.db?_journal=WAL&_busy_timeout=5000"
Environment="OCSERV_PANEL_CONFIG_PATH=${CONFIG_FILE}"
Environment="OCSERV_PANEL_TSDB_PATH=${METRICS_DIR}"
Environment="OCSERV_PANEL_MIGRATIONS_DIR=${MIGRATIONS_DST}"
Environment="OCSERV_PANEL_STATIC_DIR=${STATIC_DST}"
$(printf '%b' "${tls_env_lines}")

Restart=on-failure
RestartSec=5s
TimeoutStartSec=30s
TimeoutStopSec=10s

PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DATA_DIR} ${LOG_DIR}
ReadOnlyPaths=${ro_paths}

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
EOF

    systemctl daemon-reload
    ok "Systemd unit installed and daemon reloaded"
}

# ─── Start service ────────────────────────────────────────────────────────────

start_service() {
    info "Enabling and starting ocserv-panel service..."
    systemctl enable ocserv-panel
    systemctl start ocserv-panel

    # Wait briefly for the service to reach active state
    local retries=6
    local i=0
    while (( i < retries )); do
        if systemctl is-active --quiet ocserv-panel; then
            ok "Service is active"
            return 0
        fi
        sleep 1
        (( i++ ))
    done

    fail "Service did not become active within ${retries} seconds"
    systemctl status ocserv-panel --no-pager || true
    die "Service failed to start. Check 'journalctl -u ocserv-panel' for details."
}

# ─── Verify health endpoint ───────────────────────────────────────────────────

verify_health() {
    if ! command -v curl &>/dev/null; then
        warn "curl not available — skipping health check"
        return 0
    fi

    info "Verifying health endpoint..."
    local health_url="http://127.0.0.1:${PANEL_PORT}/api/health"
    local retries=5
    local i=0

    while (( i < retries )); do
        local http_code
        http_code="$(curl -s -o /dev/null -w '%{http_code}' \
            --connect-timeout 2 --max-time 5 "${health_url}" 2>/dev/null || echo 000)"
        if [[ "${http_code}" == "200" ]]; then
            ok "Health endpoint responded: ${health_url}"
            return 0
        fi
        (( i++ ))
        sleep 2
    done

    warn "Health endpoint did not respond with 200 after ${retries} attempts"
    warn "This may be normal if the service needs extra time to initialise."
    warn "Manual check: curl ${health_url}"
}

# ─── Success banner ───────────────────────────────────────────────────────────

success_banner() {
    local hostname
    hostname="$(hostname -f 2>/dev/null || hostname)"
    local version
    version="$(read_version)"

    echo ""
    bold "=========================================="
    bold "  Installation complete  v${version}"
    bold "=========================================="
    echo ""
    info "Access the panel at:"
    printf "  %s\n" "http://${hostname}:${PANEL_PORT}"
    echo ""
    info "Useful commands:"
    echo "  systemctl status ocserv-panel"
    echo "  journalctl -u ocserv-panel -f"
    echo "  systemctl restart ocserv-panel"
    echo ""
    if [[ -z "${DETECTED_CERT_PATH}" ]]; then
        warn "No TLS certificate detected. The panel is running in HTTP mode."
        warn "For production use, configure a TLS certificate and consider HAProxy"
        warn "for TLS termination. See deploy/haproxy.cfg.snippet for guidance."
    fi
    echo ""
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

    # Stop and disable service
    info "Stopping and disabling service..."
    if systemctl is-active --quiet ocserv-panel 2>/dev/null; then
        systemctl stop ocserv-panel && ok "Service stopped"
    else
        ok "Service was not running"
    fi
    if systemctl is-enabled --quiet ocserv-panel 2>/dev/null; then
        systemctl disable ocserv-panel && ok "Service disabled"
    fi

    # Remove systemd unit
    if [[ -f "${SYSTEMD_UNIT}" ]]; then
        rm -f "${SYSTEMD_UNIT}"
        systemctl daemon-reload
        ok "Systemd unit removed"
    fi

    # Remove sudoers
    if [[ -f "${SUDOERS_FILE}" ]]; then
        rm -f "${SUDOERS_FILE}"
        ok "Sudoers file removed"
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

    if confirm "Remove data directory ${DATA_DIR} (SQLite database and metrics)?" "n"; then
        rm -rf "${DATA_DIR}"
        ok "Data directory removed"
    else
        ok "Data preserved at ${DATA_DIR}"
    fi

    if confirm "Remove log directory ${LOG_DIR}?" "n"; then
        rm -rf "${LOG_DIR}"
        ok "Log directory removed"
    else
        ok "Logs preserved at ${LOG_DIR}"
    fi

    if confirm "Remove service user '${SERVICE_USER}'?" "n"; then
        if id "${SERVICE_USER}" &>/dev/null; then
            userdel "${SERVICE_USER}"
            ok "User '${SERVICE_USER}' removed"
        fi
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

    # Phase 1: Preflight
    preflight

    # Phase 2: Plan and confirm
    show_plan "${mode}"

    if ! confirm "Proceed with ${mode}?" "y"; then
        echo "Aborted."
        exit 0
    fi
    echo ""

    # Phase 3: Execute
    if [[ "${mode}" == "upgrade" ]]; then
        stop_existing
        backup_data
    fi

    create_user
    install_binary
    install_static
    install_migrations
    generate_config
    install_sudoers
    verify_cert_access
    install_systemd
    start_service

    # Phase 4: Verify
    verify_health
    success_banner
}

main "$@"
