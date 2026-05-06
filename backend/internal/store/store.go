// Package store contains SQLite-backed persistence repositories.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ocserv_panel/backend/internal/rbac"
)

// ErrFirstOwnerExists is returned when bootstrap owner creation is no longer allowed.
var ErrFirstOwnerExists = errors.New("admin user already exists")

// Store provides SQLite persistence operations.
type Store struct {
	DB *sql.DB
}

// Admin is a persisted administrator account with RBAC roles.
type Admin struct {
	ID           int64
	Username     string
	PasswordHash string
	Roles        []rbac.Role
	DisabledAt   *time.Time
	CreatedAt    time.Time
}

// Session is a persisted admin session. TokenHash must be a hash, never a raw token.
type Session struct {
	ID        int64
	AdminID   int64
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// AuditEvent is a persisted audit event. Outcome is a string to keep store independent of audit.
type AuditEvent struct {
	ID               int64
	ActorAdminUserID *int64
	Action           string
	ResourceType     string
	ResourceID       string
	Outcome          string
	Message          string
	Metadata         map[string]string
	CreatedAt        time.Time
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

// New returns a Store using database.
func New(db *sql.DB) *Store {
	return &Store{DB: db}
}

// AdminCount returns the number of persisted admin users.
func (s *Store) AdminCount(ctx context.Context) (int64, error) {
	var count int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count admin users: %w", err)
	}
	return count, nil
}

// CreateFirstOwner creates the bootstrap owner exactly once.
func (s *Store) CreateFirstOwner(ctx context.Context, username string, passwordHash string) (Admin, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Admin{}, fmt.Errorf("begin create first owner transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Acquire SQLite's write lock before checking the bootstrap invariant.
	if _, err := tx.ExecContext(ctx, `UPDATE app_settings SET value = value WHERE key = ?`, "app.schema_version"); err != nil {
		return Admin{}, fmt.Errorf("lock bootstrap settings: %w", err)
	}

	var count int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users`).Scan(&count); err != nil {
		return Admin{}, fmt.Errorf("count admin users: %w", err)
	}
	if count != 0 {
		return Admin{}, ErrFirstOwnerExists
	}

	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `INSERT INTO admin_users (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)`, username, passwordHash, formatTime(now), formatTime(now))
	if err != nil {
		return Admin{}, fmt.Errorf("insert first owner admin user: %w", err)
	}
	adminID, err := result.LastInsertId()
	if err != nil {
		return Admin{}, fmt.Errorf("read first owner admin id: %w", err)
	}

	var ownerRoleID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = ?`, string(rbac.RoleOwner)).Scan(&ownerRoleID); err != nil {
		return Admin{}, fmt.Errorf("find owner role: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_user_roles (admin_user_id, role_id, created_at) VALUES (?, ?, ?)`, adminID, ownerRoleID, formatTime(now)); err != nil {
		return Admin{}, fmt.Errorf("assign owner role: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Admin{}, fmt.Errorf("commit create first owner transaction: %w", err)
	}
	committed = true

	return Admin{ID: adminID, Username: username, PasswordHash: passwordHash, Roles: []rbac.Role{rbac.RoleOwner}, CreatedAt: now}, nil
}

// FindAdminByUsername returns an admin and its roles by username.
func (s *Store) FindAdminByUsername(ctx context.Context, username string) (Admin, bool, error) {
	admin, found, err := s.findAdmin(ctx, `SELECT id, username, password_hash, disabled_at, created_at FROM admin_users WHERE username = ?`, username)
	if err != nil || !found {
		return admin, found, err
	}
	return s.withRoles(ctx, admin)
}

// FindAdminBySessionTokenHash returns the admin for a current, unrevoked session hash.
func (s *Store) FindAdminBySessionTokenHash(ctx context.Context, tokenHash string, now time.Time) (Admin, bool, error) {
	admin, found, err := s.findAdmin(ctx, `
SELECT au.id, au.username, au.password_hash, au.disabled_at, au.created_at
FROM admin_users au
JOIN admin_sessions s ON s.admin_user_id = au.id
WHERE s.token_hash = ? AND s.revoked_at IS NULL AND s.expires_at > ? AND au.disabled_at IS NULL`, tokenHash, formatTime(now))
	if err != nil || !found {
		return admin, found, err
	}
	return s.withRoles(ctx, admin)
}

// CreateSession stores a hashed session token for an admin.
func (s *Store) CreateSession(ctx context.Context, adminID int64, tokenHash string, expiresAt time.Time) (Session, error) {
	now := time.Now().UTC()
	result, err := s.DB.ExecContext(ctx, `INSERT INTO admin_sessions (admin_user_id, token_hash, expires_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, adminID, tokenHash, formatTime(expiresAt), formatTime(now), formatTime(now))
	if err != nil {
		return Session{}, fmt.Errorf("insert admin session: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Session{}, fmt.Errorf("read admin session id: %w", err)
	}
	return Session{ID: id, AdminID: adminID, TokenHash: tokenHash, ExpiresAt: expiresAt.UTC()}, nil
}

// RevokeSessionByTokenHash revokes a session by token hash.
func (s *Store) RevokeSessionByTokenHash(ctx context.Context, tokenHash string, revokedAt time.Time) error {
	if _, err := s.DB.ExecContext(ctx, `UPDATE admin_sessions SET revoked_at = ?, updated_at = ? WHERE token_hash = ?`, formatTime(revokedAt), formatTime(time.Now().UTC()), tokenHash); err != nil {
		return fmt.Errorf("revoke admin session: %w", err)
	}
	return nil
}

// RecentAuditEvents returns recent audit events in newest-first order.
func (s *Store) RecentAuditEvents(ctx context.Context, limit int) ([]AuditEvent, error) {
	if limit <= 0 {
		return []AuditEvent{}, nil
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, actor_admin_user_id, action, resource_type, resource_id, outcome, message, metadata_json, created_at FROM audit_log ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent audit events: %w", err)
	}
	defer rows.Close()

	events := make([]AuditEvent, 0)
	for rows.Next() {
		event, err := scanAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent audit events: %w", err)
	}
	return events, nil
}

// RecordAudit persists an audit event with JSON metadata.
func (s *Store) RecordAudit(ctx context.Context, event AuditEvent) error {
	metadata := sanitizeAuditMetadata(event.Metadata)
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}
	createdAt := event.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	if _, err := s.DB.ExecContext(ctx, `INSERT INTO audit_log (actor_admin_user_id, action, resource_type, resource_id, outcome, message, metadata_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, event.ActorAdminUserID, event.Action, event.ResourceType, event.ResourceID, event.Outcome, event.Message, string(metadataJSON), formatTime(createdAt)); err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
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

// PurgeHistoricalBefore deletes only historical sessions disconnected before cutoff.
func (s *Store) PurgeHistoricalBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin purge historical sessions transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	rows, err := tx.QueryContext(ctx, `SELECT id, disconnected_at FROM historical_sessions`)
	if err != nil {
		return 0, fmt.Errorf("query historical sessions for purge: %w", err)
	}

	cutoff = cutoff.UTC()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		var disconnectedAt string
		if err := rows.Scan(&id, &disconnectedAt); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scan historical session for purge: %w", err)
		}
		parsedDisconnectedAt, err := parseTime(disconnectedAt)
		if err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("parse purge disconnected_at: %w", err)
		}
		if parsedDisconnectedAt.UTC().Before(cutoff) {
			ids = append(ids, id)
		}
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close historical sessions purge rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate historical sessions for purge: %w", err)
	}

	var deleted int64
	for _, id := range ids {
		result, err := tx.ExecContext(ctx, `DELETE FROM historical_sessions WHERE id = ?`, id)
		if err != nil {
			return 0, fmt.Errorf("delete historical session: %w", err)
		}
		deletedForID, err := result.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("read purged historical session count: %w", err)
		}
		deleted += deletedForID
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit purge historical sessions transaction: %w", err)
	}
	committed = true

	return deleted, nil
}

func (s *Store) findAdmin(ctx context.Context, query string, args ...any) (Admin, bool, error) {
	var admin Admin
	var disabledAt sql.NullString
	var createdAt string
	err := s.DB.QueryRowContext(ctx, query, args...).Scan(&admin.ID, &admin.Username, &admin.PasswordHash, &disabledAt, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Admin{}, false, nil
		}
		return Admin{}, false, fmt.Errorf("query admin user: %w", err)
	}
	parsedCreatedAt, err := parseTime(createdAt)
	if err != nil {
		return Admin{}, false, fmt.Errorf("parse admin created_at: %w", err)
	}
	admin.CreatedAt = parsedCreatedAt
	if disabledAt.Valid {
		parsedDisabledAt, err := parseTime(disabledAt.String)
		if err != nil {
			return Admin{}, false, fmt.Errorf("parse admin disabled_at: %w", err)
		}
		admin.DisabledAt = &parsedDisabledAt
	}
	return admin, true, nil
}

func (s *Store) withRoles(ctx context.Context, admin Admin) (Admin, bool, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT r.name FROM roles r JOIN admin_user_roles aur ON aur.role_id = r.id WHERE aur.admin_user_id = ? ORDER BY r.name`, admin.ID)
	if err != nil {
		return Admin{}, false, fmt.Errorf("query admin roles: %w", err)
	}
	defer rows.Close()

	roles := make([]rbac.Role, 0)
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return Admin{}, false, fmt.Errorf("scan admin role: %w", err)
		}
		roles = append(roles, rbac.Role(role))
	}
	if err := rows.Err(); err != nil {
		return Admin{}, false, fmt.Errorf("iterate admin roles: %w", err)
	}
	admin.Roles = roles
	return admin, true, nil
}

func scanAuditEvent(rows *sql.Rows) (AuditEvent, error) {
	var event AuditEvent
	var actorID sql.NullInt64
	var resourceID sql.NullString
	var metadataJSON string
	var createdAt string
	if err := rows.Scan(&event.ID, &actorID, &event.Action, &event.ResourceType, &resourceID, &event.Outcome, &event.Message, &metadataJSON, &createdAt); err != nil {
		return AuditEvent{}, fmt.Errorf("scan audit event: %w", err)
	}
	if actorID.Valid {
		event.ActorAdminUserID = &actorID.Int64
	}
	if resourceID.Valid {
		event.ResourceID = resourceID.String
	}
	if err := json.Unmarshal([]byte(metadataJSON), &event.Metadata); err != nil {
		return AuditEvent{}, fmt.Errorf("unmarshal audit metadata: %w", err)
	}
	parsedCreatedAt, err := parseTime(createdAt)
	if err != nil {
		return AuditEvent{}, fmt.Errorf("parse audit created_at: %w", err)
	}
	event.CreatedAt = parsedCreatedAt
	return event, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse("2006-01-02 15:04:05", value); err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("parse time %q", value)
}

func sanitizeAuditMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return map[string]string{}
	}
	sanitized := make(map[string]string, len(metadata))
	for key, value := range metadata {
		if isSensitiveMetadataKey(key) || isSensitiveMetadataValue(value) {
			sanitized[key] = "[redacted]"
			continue
		}
		sanitized[key] = value
	}
	return sanitized
}

func isSensitiveMetadataKey(key string) bool {
	normalizedKey := normalizeMetadataKey(key)
	for _, sensitive := range []string{
		"password",
		"passphrase",
		"token",
		"sessiontoken",
		"secret",
		"privatekey",
		"credential",
		"authorization",
		"cookie",
	} {
		if strings.Contains(normalizedKey, sensitive) {
			return true
		}
	}
	return false
}

func isSensitiveMetadataValue(value string) bool {
	upperValue := strings.ToUpper(value)
	return strings.Contains(upperValue, "BEGIN") && strings.Contains(upperValue, "PRIVATE KEY")
}

func normalizeMetadataKey(key string) string {
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return replacer.Replace(strings.ToLower(key))
}
