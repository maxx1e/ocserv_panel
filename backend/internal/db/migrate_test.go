package db

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyMigrationsAppliesSQLFilesInOrder(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	firstMigration := `CREATE TABLE records (id INTEGER PRIMARY KEY, name TEXT NOT NULL);`
	secondMigration := `INSERT INTO records (id, name) VALUES (1, 'applied in order');`

	if err := os.WriteFile(filepath.Join(dir, "0001_create.sql"), []byte(firstMigration), 0o600); err != nil {
		t.Fatalf("write first migration: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0002_insert.sql"), []byte(secondMigration), 0o600); err != nil {
		t.Fatalf("write second migration: %v", err)
	}

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close sqlite database: %v", err)
		}
	})

	if err := ApplyMigrations(ctx, database, dir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var name string
	if err := database.QueryRowContext(ctx, `SELECT name FROM records WHERE id = ?`, 1).Scan(&name); err != nil {
		t.Fatalf("query inserted row: %v", err)
	}
	if name != "applied in order" {
		t.Fatalf("inserted row name = %q, want %q", name, "applied in order")
	}
}

func TestApplyMigrationsRejectsMissingDirectory(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close sqlite database: %v", err)
		}
	})

	missingDir := filepath.Join(t.TempDir(), "missing")
	if err := ApplyMigrations(ctx, database, missingDir); err == nil {
		t.Fatal("ApplyMigrations returned nil error for missing directory")
	}
}

func TestApplyMigrationsSkipsAlreadyAppliedFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	migration := `
CREATE TABLE records (name TEXT NOT NULL);
INSERT INTO records (name) VALUES ('created once');
`
	if err := os.WriteFile(filepath.Join(dir, "0001_create_and_insert.sql"), []byte(migration), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close sqlite database: %v", err)
		}
	})

	if err := ApplyMigrations(ctx, database, dir); err != nil {
		t.Fatalf("apply migrations first time: %v", err)
	}
	if err := ApplyMigrations(ctx, database, dir); err != nil {
		t.Fatalf("apply migrations second time: %v", err)
	}

	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM records WHERE name = ?`, "created once").Scan(&count); err != nil {
		t.Fatalf("count records: %v", err)
	}
	if count != 1 {
		t.Fatalf("record count = %d, want 1", count)
	}
}

func TestApplyMigrationsRollsBackFailedMigration(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	migration := `
CREATE TABLE partial_records (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
INSERT INTO missing_records (name) VALUES ('should fail');
`
	if err := os.WriteFile(filepath.Join(dir, "0001_failing.sql"), []byte(migration), 0o600); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close sqlite database: %v", err)
		}
	})

	if err := ApplyMigrations(ctx, database, dir); err == nil {
		t.Fatal("ApplyMigrations returned nil error for failing migration")
	}

	var tableName string
	err = database.QueryRowContext(
		ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
		"partial_records",
	).Scan(&tableName)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("partial_records table query error = %v, want sql.ErrNoRows", err)
	}
}

func TestOpenSQLiteEnforcesForeignKeys(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "foreign-keys.db")

	database, err := OpenSQLite(ctx, databasePath)
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close sqlite database: %v", err)
		}
	})

	_, err = database.ExecContext(ctx, `
CREATE TABLE parents (id INTEGER PRIMARY KEY);
CREATE TABLE children (
    id INTEGER PRIMARY KEY,
    parent_id INTEGER NOT NULL,
    FOREIGN KEY (parent_id) REFERENCES parents(id)
);
`)
	if err != nil {
		t.Fatalf("create foreign key tables: %v", err)
	}

	if _, err := database.ExecContext(ctx, `INSERT INTO children (id, parent_id) VALUES (?, ?)`, 1, 999); err == nil {
		t.Fatal("insert child with missing parent succeeded, want foreign key error")
	}
}

func TestOpenSQLiteEnforcesForeignKeysWhenPathContainsForeignKeys(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "foreign_keys.db")

	database, err := OpenSQLite(ctx, databasePath)
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("close sqlite database: %v", err)
		}
	})

	_, err = database.ExecContext(ctx, `
CREATE TABLE parents (id INTEGER PRIMARY KEY);
CREATE TABLE children (
    id INTEGER PRIMARY KEY,
    parent_id INTEGER NOT NULL,
    FOREIGN KEY (parent_id) REFERENCES parents(id)
);
`)
	if err != nil {
		t.Fatalf("create foreign key tables: %v", err)
	}

	if _, err := database.ExecContext(ctx, `INSERT INTO children (id, parent_id) VALUES (?, ?)`, 1, 999); err == nil {
		t.Fatal("insert child with missing parent succeeded, want foreign key error")
	}
}
