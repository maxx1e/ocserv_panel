PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length(trim(name)) > 0)
);

CREATE TABLE IF NOT EXISTS admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    disabled_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length(trim(username)) > 0),
    CHECK (length(password_hash) > 0)
);

CREATE TABLE IF NOT EXISTS admin_user_roles (
    admin_user_id INTEGER NOT NULL,
    role_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (admin_user_id, role_id),
    FOREIGN KEY (admin_user_id) REFERENCES admin_users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    FOREIGN KEY (role_id) REFERENCES roles(id) ON UPDATE CASCADE ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS admin_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    admin_user_id INTEGER NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    revoked_at TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (admin_user_id) REFERENCES admin_users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    CHECK (length(token_hash) > 0),
    CHECK (length(trim(expires_at)) > 0)
);

CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    actor_admin_user_id INTEGER,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT,
    outcome TEXT NOT NULL CHECK (outcome IN ('success', 'failure', 'denied')),
    message TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(metadata_json)),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (actor_admin_user_id) REFERENCES admin_users(id) ON UPDATE CASCADE ON DELETE SET NULL,
    CHECK (length(trim(action)) > 0),
    CHECK (length(trim(resource_type)) > 0)
);

CREATE TABLE IF NOT EXISTS historical_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    connected_at TEXT NOT NULL,
    disconnected_at TEXT NOT NULL,
    bytes_in INTEGER NOT NULL DEFAULT 0,
    bytes_out INTEGER NOT NULL DEFAULT 0,
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    source TEXT NOT NULL CHECK (source IN ('sample', 'ingested')),
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length(trim(username)) > 0),
    CHECK (length(trim(connected_at)) > 0),
    CHECK (length(trim(disconnected_at)) > 0),
    CHECK (bytes_in >= 0),
    CHECK (bytes_out >= 0),
    CHECK (duration_seconds >= 0)
);

CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length(trim(key)) > 0)
);

INSERT OR IGNORE INTO roles (name, description, created_at)
VALUES
    ('owner', 'Full local administration role for this single-node ocserv panel', '2026-01-01T00:00:00Z'),
    ('viewer', 'Read-only local administration role for dashboard visibility', '2026-01-01T00:00:00Z');

INSERT OR IGNORE INTO historical_sessions (
    username,
    connected_at,
    disconnected_at,
    bytes_in,
    bytes_out,
    duration_seconds,
    source,
    created_at
)
VALUES
    ('sample-owner', '2026-01-01T08:00:00Z', '2026-01-01T09:00:00Z', 1048576, 8388608, 3600, 'sample', '2026-01-01T09:00:00Z'),
    ('sample-viewer', '2026-01-02T10:15:00Z', '2026-01-02T10:45:00Z', 524288, 2097152, 1800, 'sample', '2026-01-02T10:45:00Z'),
    ('sample-ops', '2026-01-03T18:30:00Z', '2026-01-03T20:00:00Z', 2097152, 16777216, 5400, 'sample', '2026-01-03T20:00:00Z');

INSERT OR IGNORE INTO app_settings (key, value, updated_at)
VALUES
    ('app.schema_version', '1', '2026-01-01T00:00:00Z');
