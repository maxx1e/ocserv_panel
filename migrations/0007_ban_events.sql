-- Persistent log of IP ban events captured from ocserv.
-- unbanned_at is NULL while the ban is active.
CREATE TABLE IF NOT EXISTS ban_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ip TEXT NOT NULL,
  score INTEGER NOT NULL DEFAULT 0,
  reason TEXT NOT NULL DEFAULT '',
  banned_at TEXT NOT NULL,
  unbanned_at TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ban_events_ip ON ban_events(ip);
CREATE INDEX IF NOT EXISTS idx_ban_events_banned_at ON ban_events(banned_at DESC);
