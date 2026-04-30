package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ApplyMigrations applies SQL migration files from dir in lexicographic order.
func ApplyMigrations(ctx context.Context, database *sql.DB, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}

	migrations := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		migrations = append(migrations, entry.Name())
	}

	if len(migrations) == 0 {
		return fmt.Errorf("no sql migrations found in %s", dir)
	}
	if _, err := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    filename TEXT PRIMARY KEY,
    checksum_sha256 TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}

	sort.Strings(migrations)
	for _, migration := range migrations {
		path := filepath.Join(dir, migration)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", migration, err)
		}
		checksum := sha256.Sum256(sqlBytes)
		checksumHex := hex.EncodeToString(checksum[:])

		var appliedChecksum string
		err = database.QueryRowContext(
			ctx,
			`SELECT checksum_sha256 FROM schema_migrations WHERE filename = ?`,
			migration,
		).Scan(&appliedChecksum)
		if err == nil {
			if appliedChecksum != checksumHex {
				return fmt.Errorf("migration %s checksum mismatch", migration)
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("query migration %s status: %w", migration, err)
		}

		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s transaction: %w", migration, err)
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration, err)
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO schema_migrations (filename, checksum_sha256) VALUES (?, ?)`,
			migration,
			checksumHex,
		); err != nil {
			return fmt.Errorf("record migration %s: %w", migration, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", migration, err)
		}
		committed = true
	}

	return nil
}
