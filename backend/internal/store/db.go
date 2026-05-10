// Package store contains SQLite-backed persistence repositories.
package store

import (
	"database/sql"
	"errors"
)

// ErrFirstOwnerExists is returned when bootstrap owner creation is no longer allowed.
var ErrFirstOwnerExists = errors.New("admin user already exists")

// ErrDuplicateUsername is returned when attempting to create an admin with a username that already exists.
var ErrDuplicateUsername = errors.New("username already exists")

// Store provides SQLite persistence operations.
type Store struct {
	DB *sql.DB
}

// New returns a Store using database.
func New(db *sql.DB) *Store {
	return &Store{DB: db}
}
