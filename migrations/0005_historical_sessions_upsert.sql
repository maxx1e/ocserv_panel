-- Add ocserv_session_id column for precise deduplication of ingested sessions.
-- A single ocserv session has a unique short ID (e.g. "YwDQr5") that persists
-- across the session's lifetime. This allows exact upsert without relying on
-- the (username, connected_at) pair which could collide if timestamp parsing
-- rounds differently across polls.
ALTER TABLE historical_sessions ADD COLUMN ocserv_session_id TEXT NOT NULL DEFAULT '';

-- Unique index on ocserv_session_id for non-empty values. The empty default
-- allows legacy/sample rows to coexist without violating uniqueness.
CREATE UNIQUE INDEX IF NOT EXISTS idx_historical_sessions_ocserv_sid
    ON historical_sessions(ocserv_session_id) WHERE ocserv_session_id != '';

-- Covering index for the common query pattern: sessions within a time window.
CREATE INDEX IF NOT EXISTS idx_historical_sessions_connected_at
    ON historical_sessions(connected_at);
