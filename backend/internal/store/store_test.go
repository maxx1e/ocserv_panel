package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ocserv_panel/backend/internal/db"
	"ocserv_panel/backend/internal/rbac"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	ctx := context.Background()
	database, err := db.OpenSQLite(ctx, filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})

	if err := db.ApplyMigrations(ctx, database, "../../../migrations"); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	return New(database)
}

func TestStoreCreateFirstOwnerOnlyOnce(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	admin, err := s.CreateFirstOwner(ctx, "owner", "argon2id-hash")
	if err != nil {
		t.Fatalf("create first owner: %v", err)
	}
	if admin.ID == 0 {
		t.Fatal("created admin has zero id")
	}
	if admin.Username != "owner" {
		t.Fatalf("username = %q, want owner", admin.Username)
	}
	if len(admin.Roles) != 1 || admin.Roles[0] != rbac.RoleOwner {
		t.Fatalf("roles = %#v, want owner", admin.Roles)
	}

	if _, err := s.CreateFirstOwner(ctx, "second", "argon2id-hash-2"); err == nil {
		t.Fatal("second first-owner creation succeeded, want error")
	}

	count, err := s.AdminCount(ctx)
	if err != nil {
		t.Fatalf("admin count: %v", err)
	}
	if count != 1 {
		t.Fatalf("admin count = %d, want 1", count)
	}
}

func TestStoreFindAdminByUsernameIncludesRoles(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	created, err := s.CreateFirstOwner(ctx, "owner", "argon2id-hash")
	if err != nil {
		t.Fatalf("create first owner: %v", err)
	}

	admin, found, err := s.FindAdminByUsername(ctx, "owner")
	if err != nil {
		t.Fatalf("find admin: %v", err)
	}
	if !found {
		t.Fatal("admin not found")
	}
	if admin.ID != created.ID || admin.PasswordHash != "argon2id-hash" {
		t.Fatalf("admin = %#v, want created admin", admin)
	}
	if len(admin.Roles) != 1 || admin.Roles[0] != rbac.RoleOwner {
		t.Fatalf("roles = %#v, want owner", admin.Roles)
	}

	_, found, err = s.FindAdminByUsername(ctx, "missing")
	if err != nil {
		t.Fatalf("find missing admin: %v", err)
	}
	if found {
		t.Fatal("missing admin found")
	}
}

func TestStoreSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	admin, err := s.CreateFirstOwner(ctx, "owner", "argon2id-hash")
	if err != nil {
		t.Fatalf("create first owner: %v", err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	session, err := s.CreateSession(ctx, admin.ID, "sha256-token-hash", expiresAt)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.ID == 0 || session.TokenHash != "sha256-token-hash" {
		t.Fatalf("session = %#v", session)
	}

	bySession, found, err := s.FindAdminBySessionTokenHash(ctx, "sha256-token-hash", time.Now().UTC())
	if err != nil {
		t.Fatalf("find by current session: %v", err)
	}
	if !found || bySession.ID != admin.ID {
		t.Fatalf("session lookup found=%v admin=%#v, want admin id %d", found, bySession, admin.ID)
	}

	if err := s.RevokeSessionByTokenHash(ctx, "sha256-token-hash", time.Now().UTC()); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	_, found, err = s.FindAdminBySessionTokenHash(ctx, "sha256-token-hash", time.Now().UTC())
	if err != nil {
		t.Fatalf("find revoked session: %v", err)
	}
	if found {
		t.Fatal("revoked session found")
	}

	_, err = s.CreateSession(ctx, admin.ID, "expired-token-hash", time.Now().UTC().Add(-time.Minute))
	if err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	_, found, err = s.FindAdminBySessionTokenHash(ctx, "expired-token-hash", time.Now().UTC())
	if err != nil {
		t.Fatalf("find expired session: %v", err)
	}
	if found {
		t.Fatal("expired session found")
	}
}

func TestStoreHistoricalAnalyticsReturnsSampleRows(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	sessions, err := s.HistoricalSessions(ctx, 10)
	if err != nil {
		t.Fatalf("historical sessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("no historical sessions returned")
	}
	sawSample := false
	for _, session := range sessions {
		if session.Source == "sample" {
			sawSample = true
		}
		if session.ConnectedAt.IsZero() || session.DisconnectedAt.IsZero() {
			t.Fatalf("session has zero timestamps: %#v", session)
		}
	}
	if !sawSample {
		t.Fatalf("historical sessions = %#v, want sample source", sessions)
	}

	summary, err := s.AnalyticsSummary(ctx)
	if err != nil {
		t.Fatalf("analytics summary: %v", err)
	}
	if summary.TotalSessions < 3 || summary.SampleSessions < 3 {
		t.Fatalf("summary = %#v, want sample rows counted", summary)
	}
	if summary.TotalBytesIn == 0 || summary.TotalBytesOut == 0 {
		t.Fatalf("summary = %#v, want byte totals", summary)
	}
}

func TestStorePurgeHistoricalDeletesOnlyOlderAnalytics(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	admin, err := s.CreateFirstOwner(ctx, "owner", "argon2id-hash")
	if err != nil {
		t.Fatalf("create first owner: %v", err)
	}
	if _, err := s.CreateSession(ctx, admin.ID, "sha256-token-hash", time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := s.RecordAudit(ctx, AuditEvent{
		ActorAdminUserID: &admin.ID,
		Action:           "test.action",
		ResourceType:     "test",
		ResourceID:       "1",
		Outcome:          "success",
		Message:          "safe test event",
		Metadata:         map[string]string{"field": "redacted"},
		CreatedAt:        time.Now().UTC(),
	}); err != nil {
		t.Fatalf("record audit: %v", err)
	}

	_, err = s.DB.ExecContext(ctx, `INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"old-user", "2025-01-01T00:00:00Z", "2025-01-01T00:05:00Z", 1, 2, 300, "ingested", "2025-01-01T00:05:00Z")
	if err != nil {
		t.Fatalf("insert old historical session: %v", err)
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"new-user", "2026-04-01T00:00:00Z", "2026-04-01T00:05:00Z", 3, 4, 300, "ingested", "2026-04-01T00:05:00Z")
	if err != nil {
		t.Fatalf("insert new historical session: %v", err)
	}

	deleted, err := s.PurgeHistoricalBefore(ctx, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("purge historical: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	var oldCount, newCount, adminCount, sessionCount, auditCount int64
	queries := map[string]*int64{
		`SELECT COUNT(*) FROM historical_sessions WHERE username = 'old-user'`: &oldCount,
		`SELECT COUNT(*) FROM historical_sessions WHERE username = 'new-user'`: &newCount,
		`SELECT COUNT(*) FROM admin_users`:                                     &adminCount,
		`SELECT COUNT(*) FROM admin_sessions`:                                  &sessionCount,
		`SELECT COUNT(*) FROM audit_log`:                                       &auditCount,
	}
	for query, destination := range queries {
		if err := s.DB.QueryRowContext(ctx, query).Scan(destination); err != nil && err != sql.ErrNoRows {
			t.Fatalf("count query %q: %v", query, err)
		}
	}
	if oldCount != 0 || newCount != 1 || adminCount != 1 || sessionCount != 1 || auditCount != 1 {
		t.Fatalf("counts old=%d new=%d admin=%d session=%d audit=%d, want 0/1/1/1/1", oldCount, newCount, adminCount, sessionCount, auditCount)
	}
}

func TestStoreRecordAuditRedactsSensitiveMetadata(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	rawPrivateKey := "-----BEGIN PRIVATE KEY-----\nprivate-key-material\n-----END PRIVATE KEY-----"
	if err := s.RecordAudit(ctx, AuditEvent{
		Action:       "security.test",
		ResourceType: "audit",
		Outcome:      "success",
		Message:      "redaction test",
		Metadata: map[string]string{
			"password":        "plaintext-password",
			"SESSION_TOKEN":   "raw-token",
			"private-key":     "hyphen-private-key-value",
			"ssh-private-key": "ssh-private-key-value",
			"note":            rawPrivateKey,
			"rsa_pem":         "-----BEGIN RSA PRIVATE KEY-----\nrsa-key-material\n-----END RSA PRIVATE KEY-----",
			"openssh_pem":     "-----BEGIN OPENSSH PRIVATE KEY-----\nopenssh-key-material\n-----END OPENSSH PRIVATE KEY-----",
			"field":           "safe-value",
		},
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("record audit: %v", err)
	}

	var metadataJSON string
	if err := s.DB.QueryRowContext(ctx, `SELECT metadata_json FROM audit_log ORDER BY id DESC LIMIT 1`).Scan(&metadataJSON); err != nil {
		t.Fatalf("query metadata json: %v", err)
	}
	for _, forbidden := range []string{"plaintext-password", "raw-token", "hyphen-private-key-value", "ssh-private-key-value", "BEGIN PRIVATE KEY", "private-key-material", "BEGIN RSA PRIVATE KEY", "rsa-key-material", "BEGIN OPENSSH PRIVATE KEY", "openssh-key-material"} {
		if strings.Contains(metadataJSON, forbidden) {
			t.Fatalf("metadata json contains forbidden value %q: %s", forbidden, metadataJSON)
		}
	}
	for _, expected := range []string{`"password":"[redacted]"`, `"SESSION_TOKEN":"[redacted]"`, `"private-key":"[redacted]"`, `"ssh-private-key":"[redacted]"`, `"note":"[redacted]"`, `"rsa_pem":"[redacted]"`, `"openssh_pem":"[redacted]"`, `"field":"safe-value"`} {
		if !strings.Contains(metadataJSON, expected) {
			t.Fatalf("metadata json = %s, want fragment %s", metadataJSON, expected)
		}
	}

	events, err := s.RecentAuditEvents(ctx, 1)
	if err != nil {
		t.Fatalf("recent audit events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("recent event count = %d, want 1", len(events))
	}
	for key, value := range events[0].Metadata {
		if strings.Contains(value, "plaintext-password") || strings.Contains(value, "raw-token") || strings.Contains(value, "hyphen-private-key-value") || strings.Contains(value, "ssh-private-key-value") || strings.Contains(value, "PRIVATE KEY") || strings.Contains(value, "private-key-material") || strings.Contains(value, "rsa-key-material") || strings.Contains(value, "openssh-key-material") {
			t.Fatalf("recent event metadata %q contains forbidden value: %q", key, value)
		}
	}
	if events[0].Metadata["password"] != "[redacted]" || events[0].Metadata["SESSION_TOKEN"] != "[redacted]" || events[0].Metadata["private-key"] != "[redacted]" || events[0].Metadata["ssh-private-key"] != "[redacted]" || events[0].Metadata["note"] != "[redacted]" || events[0].Metadata["rsa_pem"] != "[redacted]" || events[0].Metadata["openssh_pem"] != "[redacted]" {
		t.Fatalf("recent event metadata = %#v, want sensitive values redacted", events[0].Metadata)
	}
}

func TestStorePurgeHistoricalRespectsCutoffBoundary(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	cutoff := time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC)
	// exactly at cutoff — must NOT be deleted (WHERE disconnected_at < cutoff, not <=)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"at-cutoff", "2025-12-31T12:00:00Z", "2025-12-31T12:00:00Z", 1, 1, 60, "ingested", "2025-12-31T12:00:00Z")
	if err != nil {
		t.Fatalf("insert at-cutoff historical session: %v", err)
	}
	// one second before cutoff — must be deleted
	_, err = s.DB.ExecContext(ctx, `INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"before-cutoff", "2025-12-31T11:59:59Z", "2025-12-31T11:59:59Z", 1, 1, 60, "ingested", "2025-12-31T11:59:59Z")
	if err != nil {
		t.Fatalf("insert before-cutoff historical session: %v", err)
	}
	// one second after cutoff — must NOT be deleted
	_, err = s.DB.ExecContext(ctx, `INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"after-cutoff", "2025-12-31T12:00:01Z", "2025-12-31T12:00:01Z", 1, 1, 60, "ingested", "2025-12-31T12:00:01Z")
	if err != nil {
		t.Fatalf("insert after-cutoff historical session: %v", err)
	}

	deleted, err := s.PurgeHistoricalBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("purge historical: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	var atCount, beforeCount, afterCount int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM historical_sessions WHERE username = ?`, "at-cutoff").Scan(&atCount); err != nil {
		t.Fatalf("count at-cutoff row: %v", err)
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM historical_sessions WHERE username = ?`, "before-cutoff").Scan(&beforeCount); err != nil {
		t.Fatalf("count before-cutoff row: %v", err)
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM historical_sessions WHERE username = ?`, "after-cutoff").Scan(&afterCount); err != nil {
		t.Fatalf("count after-cutoff row: %v", err)
	}
	if atCount != 1 || beforeCount != 0 || afterCount != 1 {
		t.Fatalf("at-cutoff=%d before-cutoff=%d after-cutoff=%d, want 1/0/1", atCount, beforeCount, afterCount)
	}
}

func TestUserSessionSummaries(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC()
	insert := func(username string, connectedAt time.Time, bytesIn, bytesOut int64) {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			username, connectedAt.Format(time.RFC3339), connectedAt.Add(time.Hour).Format(time.RFC3339),
			bytesIn, bytesOut, 3600, "ingested", now.Format(time.RFC3339),
		)
		if err != nil {
			t.Fatalf("insert session: %v", err)
		}
	}

	// Two sessions for "alice" within window
	insert("alice", now.Add(-2*time.Hour), 100, 200)
	insert("alice", now.Add(-1*time.Hour), 300, 400)
	// One session for "bob" within window
	insert("bob", now.Add(-3*time.Hour), 50, 150)
	// One old session outside window (should be excluded)
	insert("alice", now.Add(-48*time.Hour), 9999, 9999)

	since := now.Add(-24 * time.Hour)
	summaries, err := s.UserSessionSummaries(ctx, since)
	if err != nil {
		t.Fatalf("UserSessionSummaries: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("got %d summaries, want 2", len(summaries))
	}

	// alice has more total bytes, should be first
	alice := summaries[0]
	if alice.Username != "alice" {
		t.Fatalf("summaries[0].Username = %q, want alice", alice.Username)
	}
	if alice.SessionCount != 2 {
		t.Fatalf("alice.SessionCount = %d, want 2", alice.SessionCount)
	}
	if alice.TotalBytesIn != 400 {
		t.Fatalf("alice.TotalBytesIn = %d, want 400", alice.TotalBytesIn)
	}
	if alice.TotalBytesOut != 600 {
		t.Fatalf("alice.TotalBytesOut = %d, want 600", alice.TotalBytesOut)
	}
	if alice.AvgBytesIn != 200 {
		t.Fatalf("alice.AvgBytesIn = %d, want 200", alice.AvgBytesIn)
	}
	if alice.AvgBytesOut != 300 {
		t.Fatalf("alice.AvgBytesOut = %d, want 300", alice.AvgBytesOut)
	}

	bob := summaries[1]
	if bob.Username != "bob" {
		t.Fatalf("summaries[1].Username = %q, want bob", bob.Username)
	}
	if bob.SessionCount != 1 {
		t.Fatalf("bob.SessionCount = %d, want 1", bob.SessionCount)
	}
}

func TestUserSessions(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC()
	insert := func(username string, connectedAt time.Time, bytesIn, bytesOut int64) {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			username, connectedAt.Format(time.RFC3339), connectedAt.Add(time.Hour).Format(time.RFC3339),
			bytesIn, bytesOut, 3600, "ingested", now.Format(time.RFC3339),
		)
		if err != nil {
			t.Fatalf("insert session: %v", err)
		}
	}

	insert("alice", now.Add(-2*time.Hour), 100, 200)
	insert("alice", now.Add(-1*time.Hour), 300, 400)
	insert("bob", now.Add(-1*time.Hour), 50, 150)
	// Outside window
	insert("alice", now.Add(-48*time.Hour), 9999, 9999)

	since := now.Add(-24 * time.Hour)

	aliceSessions, err := s.UserSessions(ctx, "alice", since)
	if err != nil {
		t.Fatalf("UserSessions alice: %v", err)
	}
	if len(aliceSessions) != 2 {
		t.Fatalf("alice got %d sessions, want 2", len(aliceSessions))
	}
	// Chronological order: oldest first
	if !aliceSessions[0].ConnectedAt.Before(aliceSessions[1].ConnectedAt) {
		t.Fatal("sessions not in chronological order")
	}
	for _, s := range aliceSessions {
		if s.Username != "alice" {
			t.Fatalf("session username = %q, want alice", s.Username)
		}
	}

	bobSessions, err := s.UserSessions(ctx, "bob", since)
	if err != nil {
		t.Fatalf("UserSessions bob: %v", err)
	}
	if len(bobSessions) != 1 {
		t.Fatalf("bob got %d sessions, want 1", len(bobSessions))
	}

	// Unknown user returns empty slice
	unknown, err := s.UserSessions(ctx, "nobody", since)
	if err != nil {
		t.Fatalf("UserSessions unknown: %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("unknown user got %d sessions, want 0", len(unknown))
	}
}

func TestGetSettingReturnsEmptyWhenNotSet(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	value, err := s.GetSetting(ctx, "nonexistent.key")
	if err != nil {
		t.Fatalf("GetSetting unexpected error: %v", err)
	}
	if value != "" {
		t.Fatalf("GetSetting = %q, want empty string for nonexistent key", value)
	}
}

func TestSetAndGetSetting(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if err := s.SetSetting(ctx, "history.retention_days", "90"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	value, err := s.GetSetting(ctx, "history.retention_days")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if value != "90" {
		t.Fatalf("GetSetting = %q, want %q", value, "90")
	}
}

func TestSetSettingUpserts(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	if err := s.SetSetting(ctx, "history.retention_days", "90"); err != nil {
		t.Fatalf("SetSetting first: %v", err)
	}
	if err := s.SetSetting(ctx, "history.retention_days", "180"); err != nil {
		t.Fatalf("SetSetting second: %v", err)
	}

	value, err := s.GetSetting(ctx, "history.retention_days")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if value != "180" {
		t.Fatalf("GetSetting = %q, want %q after upsert", value, "180")
	}
}
