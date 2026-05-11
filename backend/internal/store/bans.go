package store

import (
	"context"
	"fmt"
	"time"
)

// BanEvent represents a single IP ban event recorded from ocserv.
type BanEvent struct {
	ID         int64
	IP         string
	Score      int
	Reason     string
	BannedAt   time.Time
	UnbannedAt *time.Time
	CreatedAt  time.Time
}

// InsertBanEvent records a new ban for ip with the given score, reason, and timestamp.
func (s *Store) InsertBanEvent(ctx context.Context, ip string, score int, reason string, bannedAt time.Time) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO ban_events (ip, score, reason, banned_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		ip, score, reason,
		bannedAt.UTC().Format(time.RFC3339),
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert ban event for %s: %w", ip, err)
	}
	return nil
}

// CloseBanEvent sets unbanned_at on the most recent open (unbanned_at IS NULL)
// ban event for ip. Uses a subquery to select the target row because SQLite does
// not support ORDER BY/LIMIT in UPDATE without SQLITE_ENABLE_UPDATE_DELETE_LIMIT.
func (s *Store) CloseBanEvent(ctx context.Context, ip string, unbanAt time.Time) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE ban_events SET unbanned_at = ?
		 WHERE id = (
		   SELECT id FROM ban_events WHERE ip = ? AND unbanned_at IS NULL
		   ORDER BY banned_at DESC LIMIT 1
		 )`,
		unbanAt.UTC().Format(time.RFC3339), ip,
	)
	if err != nil {
		return fmt.Errorf("close ban event for %s: %w", ip, err)
	}
	return nil
}

// ListBanEvents returns ban events newest-first with total count.
func (s *Store) ListBanEvents(ctx context.Context, limit, offset int) ([]BanEvent, int64, error) {
	var total int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM ban_events`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count ban events: %w", err)
	}

	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, ip, score, reason, banned_at, unbanned_at, created_at
		 FROM ban_events ORDER BY banned_at DESC, id DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query ban events: %w", err)
	}
	defer rows.Close()

	events := make([]BanEvent, 0)
	for rows.Next() {
		var e BanEvent
		var bannedAt, createdAt string
		var unbannedAt *string
		if err := rows.Scan(&e.ID, &e.IP, &e.Score, &e.Reason, &bannedAt, &unbannedAt, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("scan ban event: %w", err)
		}
		t, err := parseTime(bannedAt)
		if err != nil {
			return nil, 0, fmt.Errorf("parse banned_at: %w", err)
		}
		e.BannedAt = t
		if unbannedAt != nil {
			ut, err := parseTime(*unbannedAt)
			if err != nil {
				return nil, 0, fmt.Errorf("parse unbanned_at: %w", err)
			}
			e.UnbannedAt = &ut
		}
		ct, err := parseTime(createdAt)
		if err != nil {
			return nil, 0, fmt.Errorf("parse created_at: %w", err)
		}
		e.CreatedAt = ct
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate ban events: %w", err)
	}
	return events, total, nil
}

// OpenBanEvents returns all ban events where unbanned_at IS NULL.
func (s *Store) OpenBanEvents(ctx context.Context) ([]BanEvent, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, ip, score, reason, banned_at, unbanned_at, created_at
		 FROM ban_events WHERE unbanned_at IS NULL`,
	)
	if err != nil {
		return nil, fmt.Errorf("query open ban events: %w", err)
	}
	defer rows.Close()

	events := make([]BanEvent, 0)
	for rows.Next() {
		var e BanEvent
		var bannedAt, createdAt string
		var unbannedAt *string
		if err := rows.Scan(&e.ID, &e.IP, &e.Score, &e.Reason, &bannedAt, &unbannedAt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan open ban event: %w", err)
		}
		t, err := parseTime(bannedAt)
		if err != nil {
			return nil, fmt.Errorf("parse banned_at: %w", err)
		}
		e.BannedAt = t
		ct, err := parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		e.CreatedAt = ct
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate open ban events: %w", err)
	}
	return events, nil
}
