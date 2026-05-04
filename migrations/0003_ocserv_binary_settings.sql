-- Migration 0003: Add ocserv binary path settings and rename ocpasswd_path.
--
-- Adds:
--   installation.ocserv_bin_path  - path to ocserv binary
--   installation.occtl_bin_path   - path to occtl binary
--   installation.ocpasswd_bin_path - path to ocpasswd binary
--   installation.ocpasswd_file_path - path to ocpasswd password file (replaces ocpasswd_path)
--   service.systemd_unit_name     - systemd unit name for ocserv
--
-- Renames installation.ocpasswd_path to installation.ocpasswd_file_path.
-- The old key is deleted; any stored runtime value is preserved by migrating it.

-- Step 1: Rename the old key in app_settings (runtime overrides) if it exists.
-- SQLite has no RENAME COLUMN support before 3.25; use INSERT+DELETE.
INSERT OR IGNORE INTO app_settings (key, value)
    SELECT 'installation.ocpasswd_file_path', value
    FROM app_settings
    WHERE key = 'installation.ocpasswd_path';

DELETE FROM app_settings WHERE key = 'installation.ocpasswd_path';

-- Step 2: Rename the definition (UPDATE the primary key via INSERT+DELETE).
-- We insert the new row only if it doesn't already exist, then delete the old one.
INSERT OR IGNORE INTO setting_definitions (key, label, section, type, source, requires_restart, default_value, options, validation, sort_order)
    SELECT 'installation.ocpasswd_file_path', 'ocpasswd Password File', section, type, source, requires_restart, default_value, options, validation, 9
    FROM setting_definitions
    WHERE key = 'installation.ocpasswd_path';

DELETE FROM setting_definitions WHERE key = 'installation.ocpasswd_path';

-- Step 3: Insert new binary path settings (OR IGNORE is safe on re-run).
INSERT OR IGNORE INTO setting_definitions (key, label, section, type, source, requires_restart, default_value, options, validation, sort_order) VALUES
    ('installation.ocserv_bin_path',   'ocserv Binary Path',   'installation', 'text', 'config', 1, '/usr/local/sbin/ocserv', NULL, NULL, 6),
    ('installation.occtl_bin_path',    'occtl Binary Path',    'installation', 'text', 'config', 1, '/usr/local/bin/occtl',   NULL, NULL, 7),
    ('installation.ocpasswd_bin_path', 'ocpasswd Binary Path', 'installation', 'text', 'config', 1, '/usr/local/bin/ocpasswd', NULL, NULL, 8),
    ('service.systemd_unit_name', 'Systemd Unit Name', 'service', 'text', 'config', 1, 'ocserv', NULL, NULL, 7);

-- Step 4: Ensure ocpasswd_file_path exists even if the old key was never present.
INSERT OR IGNORE INTO setting_definitions (key, label, section, type, source, requires_restart, default_value, options, validation, sort_order) VALUES
    ('installation.ocpasswd_file_path', 'ocpasswd Password File', 'installation', 'text', 'config', 1, '/etc/ocserv/ocpasswd', NULL, NULL, 9);
