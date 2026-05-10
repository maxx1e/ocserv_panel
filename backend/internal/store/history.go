package store

import (
	"context"
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
	rows, err := s.DB.QueryContext(ctx, `SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source FROM historical_sessions ORDER BY disconnected_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query historical sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]HistoricalSession, 0)
	for rows.Next() {
		var session HistoricalSession
		var connectedAt, disconnectedAt string
		if err := rows.Scan(&session.ID, &session.Username, &connectedAt, &disconnectedAt, &session.DurationSeconds, &session.BytesIn, &session.BytesOut, &session.Source); err != nil {
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

// UserSessionSummary aggregates historical session data for a single user.
type UserSessionSummary struct {
	Username             string
	SessionCount         int64
	TotalDurationSeconds int64
	TotalBytesIn         int64
	TotalBytesOut        int64
	AvgBytesIn           int64
	AvgBytesOut          int64
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
			COALESCE(AVG(bytes_in), 0) AS avg_bytes_in,
			COALESCE(AVG(bytes_out), 0) AS avg_bytes_out
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
		if err := rows.Scan(
			&sum.Username,
			&sum.SessionCount,
			&sum.TotalDurationSeconds,
			&sum.TotalBytesIn,
			&sum.TotalBytesOut,
			&sum.AvgBytesIn,
			&sum.AvgBytesOut,
		); err != nil {
			return nil, fmt.Errorf("scan user session summary: %w", err)
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
		`SELECT id, username, connected_at, disconnected_at, duration_seconds, bytes_in, bytes_out, source
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
		if err := rows.Scan(&session.ID, &session.Username, &connectedAt, &disconnectedAt, &session.DurationSeconds, &session.BytesIn, &session.BytesOut, &session.Source); err != nil {
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
