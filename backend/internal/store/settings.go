package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// GetSetting returns the value for a setting key, or empty string if not set.
func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	return value, nil
}

// SetSetting upserts a setting key with the given value.
func (s *Store) SetSetting(ctx context.Context, key string, value string) error {
	now := time.Now().UTC()
	result, err := s.DB.ExecContext(ctx,
		`INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, formatTime(now))
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("set setting %q: no rows affected", key)
	}
	return nil
}
