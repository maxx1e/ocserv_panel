package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// HousekeepEvent carries the fields needed to record a housekeeping audit entry.
// It is defined here to avoid an import cycle with the audit package, which already
// imports store via audit/sqlite.go.
type HousekeepEvent struct {
	Action       string
	ResourceType string
	Outcome      string
	Message      string
	Metadata     map[string]string
	CreatedAt    time.Time
}

// HousekeepRecorder is satisfied by any type that can persist a HousekeepEvent.
// It is intentionally narrow to avoid an import cycle: the audit package imports
// store, so store must not import audit.
type HousekeepRecorder interface {
	RecordHousekeep(ctx context.Context, event HousekeepEvent) error
}

// HistoricalSession is an ingested or sample ocserv session history row.
type HistoricalSession struct {
	ID              int64
	Username        string
	ConnectedAt     time.Time
	DisconnectedAt  time.Time
	DurationSeconds int64
	BytesIn         int64
	BytesOut        int64
	Source          string
	State           string
}

// AnalyticsSummary summarizes persisted historical session analytics.
type AnalyticsSummary struct {
	TotalSessions  int64
	SampleSessions int64
	TotalBytesIn   int64
	TotalBytesOut  int64
}

// HistoricalSessions returns historical session rows in newest-first order.
func (s *Store) HistoricalSessions(ctx context.Context, limit int) ([]HistoricalSession, error) {
	if limit <= 0 {
		return []HistoricalSession{}, nil
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source, state FROM historical_sessions ORDER BY disconnected_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query historical sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]HistoricalSession, 0)
	for rows.Next() {
		var session HistoricalSession
		var connectedAt, disconnectedAt string
		if err := rows.Scan(&session.ID, &session.Username, &connectedAt, &disconnectedAt, &session.DurationSeconds, &session.BytesIn, &session.BytesOut, &session.Source, &session.State); err != nil {
			return nil, fmt.Errorf("scan historical session: %w", err)
		}
		parsedConnectedAt, err := parseTime(connectedAt)
		if err != nil {
			return nil, fmt.Errorf("parse historical connected_at: %w", err)
		}
		parsedDisconnectedAt, err := parseTime(disconnectedAt)
		if err != nil {
			return nil, fmt.Errorf("parse historical disconnected_at: %w", err)
		}
		session.ConnectedAt = parsedConnectedAt
		session.DisconnectedAt = parsedDisconnectedAt
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate historical sessions: %w", err)
	}
	return sessions, nil
}

// AnalyticsSummary returns aggregate counters for historical session analytics.
func (s *Store) AnalyticsSummary(ctx context.Context) (AnalyticsSummary, error) {
	var summary AnalyticsSummary
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(CASE WHEN source = 'sample' THEN 1 ELSE 0 END), 0), COALESCE(SUM(bytes_in), 0), COALESCE(SUM(bytes_out), 0) FROM historical_sessions`).Scan(&summary.TotalSessions, &summary.SampleSessions, &summary.TotalBytesIn, &summary.TotalBytesOut); err != nil {
		return AnalyticsSummary{}, fmt.Errorf("query analytics summary: %w", err)
	}
	return summary, nil
}

// PurgeHistoricalBefore deletes historical sessions disconnected before cutoff.
func (s *Store) PurgeHistoricalBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	cutoffStr := cutoff.UTC().Format(time.RFC3339)
	result, err := s.DB.ExecContext(ctx,
		`DELETE FROM historical_sessions WHERE disconnected_at < ?`, cutoffStr)
	if err != nil {
		return 0, fmt.Errorf("purge historical sessions: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read purged historical session count: %w", err)
	}
	return deleted, nil
}

// ActiveSessionRecord holds data for a single active ocserv session to be upserted.
type ActiveSessionRecord struct {
	OcservSessionID string
	Username        string
	ConnectedAt     time.Time
	DisconnectedAt  time.Time
	DurationSeconds int64
	BytesIn         int64
	BytesOut        int64
	Source          string // "ingested"
	State           string // "authenticated", "disconnected", "timed_out", "error"
}

// UpsertActiveSession inserts or updates a historical session row keyed by
// ocserv_session_id. On conflict (same ocserv session), only the mutable fields
// (disconnected_at, duration_seconds, bytes_in, bytes_out, state) are updated.
func (s *Store) UpsertActiveSession(ctx context.Context, rec ActiveSessionRecord) error {
	if rec.OcservSessionID == "" {
		return fmt.Errorf("upsert active session: ocserv_session_id must not be empty")
	}
	if rec.Username == "" {
		return fmt.Errorf("upsert active session: username must not be empty")
	}
	if rec.Source == "" {
		rec.Source = "ingested"
	}
	if rec.State == "" {
		rec.State = "disconnected"
	}

	connectedAt := rec.ConnectedAt.UTC().Format(time.RFC3339)
	disconnectedAt := rec.DisconnectedAt.UTC().Format(time.RFC3339)
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO historical_sessions (
			ocserv_session_id, username, connected_at, disconnected_at,
			duration_seconds, bytes_in, bytes_out, source, state, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(ocserv_session_id) WHERE ocserv_session_id != ''
		DO UPDATE SET
			disconnected_at = excluded.disconnected_at,
			duration_seconds = excluded.duration_seconds,
			bytes_in = excluded.bytes_in,
			bytes_out = excluded.bytes_out,
			state = excluded.state
	`,
		rec.OcservSessionID, rec.Username, connectedAt, disconnectedAt,
		rec.DurationSeconds, rec.BytesIn, rec.BytesOut, rec.Source, rec.State, now,
	)
	if err != nil {
		return fmt.Errorf("upsert active session %s: %w", rec.OcservSessionID, err)
	}
	return nil
}

// UserSessionSummary aggregates historical session data for a single user.
type UserSessionSummary struct {
	Username             string
	SessionCount         int64
	TotalDurationSeconds int64
	TotalBytesIn         int64
	TotalBytesOut        int64
	AvgBytesIn           int64
	AvgBytesOut          int64
	FirstSeenAt          time.Time
}

// UserSessionSummaries returns per-user aggregated session stats for sessions
// where connected_at >= since, ordered by total bytes descending.
func (s *Store) UserSessionSummaries(ctx context.Context, since time.Time) ([]UserSessionSummary, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	rows, err := s.DB.QueryContext(ctx, `
		SELECT
			username,
			COUNT(*) AS session_count,
			COALESCE(SUM(duration_seconds), 0) AS total_duration_seconds,
			COALESCE(SUM(bytes_in), 0) AS total_bytes_in,
			COALESCE(SUM(bytes_out), 0) AS total_bytes_out,
			CAST(COALESCE(AVG(bytes_in), 0) AS INTEGER) AS avg_bytes_in,
			CAST(COALESCE(AVG(bytes_out), 0) AS INTEGER) AS avg_bytes_out,
			MIN(connected_at) AS first_seen_at
		FROM historical_sessions
		WHERE connected_at >= ?
		GROUP BY username
		ORDER BY (SUM(bytes_in) + SUM(bytes_out)) DESC
	`, sinceStr)
	if err != nil {
		return nil, fmt.Errorf("query user session summaries: %w", err)
	}
	defer rows.Close()

	summaries := make([]UserSessionSummary, 0)
	for rows.Next() {
		var sum UserSessionSummary
		var firstSeenStr string
		if err := rows.Scan(
			&sum.Username,
			&sum.SessionCount,
			&sum.TotalDurationSeconds,
			&sum.TotalBytesIn,
			&sum.TotalBytesOut,
			&sum.AvgBytesIn,
			&sum.AvgBytesOut,
			&firstSeenStr,
		); err != nil {
			return nil, fmt.Errorf("scan user session summary: %w", err)
		}
		if t, err := parseTime(firstSeenStr); err == nil {
			sum.FirstSeenAt = t
		}
		summaries = append(summaries, sum)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user session summaries: %w", err)
	}
	return summaries, nil
}

// UserSessions returns individual historical sessions for username where
// connected_at >= since, ordered chronologically (oldest first).
func (s *Store) UserSessions(ctx context.Context, username string, since time.Time) ([]HistoricalSession, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source, state
		 FROM historical_sessions
		 WHERE username = ? AND connected_at >= ?
		 ORDER BY connected_at ASC`,
		username, sinceStr,
	)
	if err != nil {
		return nil, fmt.Errorf("query user sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]HistoricalSession, 0)
	for rows.Next() {
		var session HistoricalSession
		var connectedAt, disconnectedAt string
		if err := rows.Scan(&session.ID, &session.Username, &connectedAt, &disconnectedAt, &session.DurationSeconds, &session.BytesIn, &session.BytesOut, &session.Source, &session.State); err != nil {
			return nil, fmt.Errorf("scan user session: %w", err)
		}
		parsedConnectedAt, err := parseTime(connectedAt)
		if err != nil {
			return nil, fmt.Errorf("parse user session connected_at: %w", err)
		}
		parsedDisconnectedAt, err := parseTime(disconnectedAt)
		if err != nil {
			return nil, fmt.Errorf("parse user session disconnected_at: %w", err)
		}
		session.ConnectedAt = parsedConnectedAt
		session.DisconnectedAt = parsedDisconnectedAt
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user sessions: %w", err)
	}
	return sessions, nil
}

// ListSessions returns historical sessions with connected_at >= since, newest
// first. If username is non-empty, only sessions for that user are returned.
// Returns the total matching count alongside the paged slice.
func (s *Store) ListSessions(ctx context.Context, since time.Time, limit, offset int, username string) ([]HistoricalSession, int64, error) {
	sinceStr := since.UTC().Format(time.RFC3339)

	var (
		total    int64
		countErr error
		rows     *sql.Rows
		queryErr error
	)

	if username != "" {
		countErr = s.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM historical_sessions WHERE connected_at >= ? AND username = ? AND state != 'authenticated'`,
			sinceStr, username,
		).Scan(&total)
	} else {
		countErr = s.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM historical_sessions WHERE connected_at >= ? AND state != 'authenticated'`,
			sinceStr,
		).Scan(&total)
	}
	if countErr != nil {
		return nil, 0, fmt.Errorf("count sessions: %w", countErr)
	}

	if username != "" {
		rows, queryErr = s.DB.QueryContext(ctx,
			`SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source, state
			 FROM historical_sessions
			 WHERE connected_at >= ? AND username = ? AND state != 'authenticated'
			 ORDER BY connected_at DESC, id DESC
			 LIMIT ? OFFSET ?`,
			sinceStr, username, limit, offset,
		)
	} else {
		rows, queryErr = s.DB.QueryContext(ctx,
			`SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source, state
			 FROM historical_sessions
			 WHERE connected_at >= ? AND state != 'authenticated'
			 ORDER BY connected_at DESC, id DESC
			 LIMIT ? OFFSET ?`,
			sinceStr, limit, offset,
		)
	}
	if queryErr != nil {
		return nil, 0, fmt.Errorf("query sessions: %w", queryErr)
	}
	defer rows.Close()

	sessions := make([]HistoricalSession, 0)
	for rows.Next() {
		var session HistoricalSession
		var connectedAt, disconnectedAt string
		if err := rows.Scan(&session.ID, &session.Username, &connectedAt, &disconnectedAt,
			&session.DurationSeconds, &session.BytesIn, &session.BytesOut, &session.Source, &session.State); err != nil {
			return nil, 0, fmt.Errorf("scan session: %w", err)
		}
		parsedConnectedAt, err := parseTime(connectedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("parse connected_at: %w", err)
		}
		parsedDisconnectedAt, err := parseTime(disconnectedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("parse disconnected_at: %w", err)
		}
		session.ConnectedAt = parsedConnectedAt
		session.DisconnectedAt = parsedDisconnectedAt
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate sessions: %w", err)
	}
	return sessions, total, nil
}

// StartHousekeeper starts a background goroutine that purges sessions older than
// the configured retention window. It stops when ctx is cancelled.
//
// recorder must implement HousekeepRecorder; pass an audit.HousekeepAdapter to
// bridge to the full audit.Recorder without introducing an import cycle.
func (s *Store) StartHousekeeper(ctx context.Context, interval time.Duration, recorder HousekeepRecorder, retentionFn func() int) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				retention := retentionFn()
				cutoff := time.Now().UTC().AddDate(0, 0, -retention)
				deleted, err := s.PurgeHistoricalBefore(ctx, cutoff)
				if err != nil {
					log.Error().Err(err).Msg("housekeeping purge failed")
					_ = recorder.RecordHousekeep(ctx, HousekeepEvent{
						Action:       "housekeeping.purge",
						ResourceType: "history",
						Outcome:      "failure",
						Message:      "historical session purge failed",
						Metadata:     map[string]string{"error": err.Error()},
						CreatedAt:    time.Now().UTC(),
					})
					continue
				}
				_ = recorder.RecordHousekeep(ctx, HousekeepEvent{
					Action:       "housekeeping.purge",
					ResourceType: "history",
					Outcome:      "success",
					Message:      "historical sessions purged",
					Metadata:     map[string]string{"deleted_rows": fmt.Sprintf("%d", deleted)},
					CreatedAt:    time.Now().UTC(),
				})
			}
		}
	}()
}
