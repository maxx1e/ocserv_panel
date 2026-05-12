package store

import (
	"context"
	"database/sql"
	"fmt"
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

func TestUpsertActiveSession_InsertAndUpdate(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	connectedAt := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)

	rec := ActiveSessionRecord{
		OcservSessionID: "AbCdEf",
		Username:        "alice",
		ConnectedAt:     connectedAt,
		DisconnectedAt:  now,
		DurationSeconds: 7200,
		BytesIn:         1024,
		BytesOut:        2048,
		Source:          "ingested",
	}

	// First insert
	if err := s.UpsertActiveSession(ctx, rec); err != nil {
		t.Fatalf("upsert first: %v", err)
	}

	// Verify insertion
	sessions, err := s.HistoricalSessions(ctx, 100)
	if err != nil {
		t.Fatalf("query sessions: %v", err)
	}
	var found bool
	for _, sess := range sessions {
		if sess.Username == "alice" && sess.BytesIn == 1024 {
			found = true
			if sess.DurationSeconds != 7200 {
				t.Fatalf("duration = %d, want 7200", sess.DurationSeconds)
			}
			if sess.Source != "ingested" {
				t.Fatalf("source = %q, want ingested", sess.Source)
			}
		}
	}
	if !found {
		t.Fatal("inserted session not found")
	}

	// Update with more bytes and longer duration
	rec.BytesIn = 4096
	rec.BytesOut = 8192
	rec.DurationSeconds = 9000
	rec.DisconnectedAt = now.Add(30 * time.Minute)

	if err := s.UpsertActiveSession(ctx, rec); err != nil {
		t.Fatalf("upsert update: %v", err)
	}

	// Verify update (not duplicate)
	var count int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM historical_sessions WHERE ocserv_session_id = ?`, "AbCdEf").Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row for session AbCdEf, got %d", count)
	}

	var bytesIn, bytesOut, duration int64
	if err := s.DB.QueryRowContext(ctx, `SELECT bytes_in, bytes_out, duration_seconds FROM historical_sessions WHERE ocserv_session_id = ?`, "AbCdEf").Scan(&bytesIn, &bytesOut, &duration); err != nil {
		t.Fatalf("select updated fields: %v", err)
	}
	if bytesIn != 4096 {
		t.Fatalf("bytes_in = %d, want 4096", bytesIn)
	}
	if bytesOut != 8192 {
		t.Fatalf("bytes_out = %d, want 8192", bytesOut)
	}
	if duration != 9000 {
		t.Fatalf("duration = %d, want 9000", duration)
	}
}

func TestUpsertActiveSession_RejectsEmptySessionID(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	rec := ActiveSessionRecord{
		OcservSessionID: "",
		Username:        "alice",
		ConnectedAt:     time.Now().UTC(),
		DisconnectedAt:  time.Now().UTC(),
		DurationSeconds: 100,
		BytesIn:         50,
		BytesOut:        100,
		Source:          "ingested",
	}

	err := s.UpsertActiveSession(ctx, rec)
	if err == nil {
		t.Fatal("expected error for empty session ID, got nil")
	}
}

func TestUpsertActiveSession_RejectsEmptyUsername(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	rec := ActiveSessionRecord{
		OcservSessionID: "SomeID",
		Username:        "",
		ConnectedAt:     time.Now().UTC(),
		DisconnectedAt:  time.Now().UTC(),
		DurationSeconds: 100,
		BytesIn:         50,
		BytesOut:        100,
		Source:          "ingested",
	}

	err := s.UpsertActiveSession(ctx, rec)
	if err == nil {
		t.Fatal("expected error for empty username, got nil")
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

func TestUpsertActiveSession_PersistsState(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	rec := ActiveSessionRecord{
		OcservSessionID: "abc123",
		Username:        "alice",
		ConnectedAt:     time.Now().UTC().Add(-time.Hour),
		DisconnectedAt:  time.Now().UTC(),
		DurationSeconds: 3600,
		BytesIn:         1000,
		BytesOut:        2000,
		Source:          "ingested",
		State:           "authenticated",
	}
	if err := s.UpsertActiveSession(ctx, rec); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	sessions, err := s.UserSessions(ctx, "alice", time.Now().UTC().Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("user sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(sessions))
	}
	if sessions[0].State != "authenticated" {
		t.Errorf("state = %q, want %q", sessions[0].State, "authenticated")
	}
}

func TestListSessions_ReturnsPagedResults(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	base := time.Now().UTC().Add(-10 * time.Hour)
	for i := 0; i < 5; i++ {
		if err := s.UpsertActiveSession(ctx, ActiveSessionRecord{
			OcservSessionID: fmt.Sprintf("sid%d", i),
			Username:        fmt.Sprintf("user%d", i),
			ConnectedAt:     base.Add(time.Duration(i) * time.Hour),
			DisconnectedAt:  base.Add(time.Duration(i)*time.Hour + 30*time.Minute),
			DurationSeconds: 1800,
			BytesIn:         int64(i * 1000),
			BytesOut:        int64(i * 2000),
			Source:          "ingested",
			State:           "disconnected",
		}); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}

	sessions, total, err := s.ListSessions(ctx, base.Add(-time.Hour), 3, 0, "")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(sessions) != 3 {
		t.Errorf("got %d sessions, want 3", len(sessions))
	}

	sessions2, total2, err := s.ListSessions(ctx, base.Add(-time.Hour), 3, 3, "")
	if err != nil {
		t.Fatalf("list sessions page 2: %v", err)
	}
	if total2 != 5 {
		t.Errorf("total2 = %d, want 5", total2)
	}
	if len(sessions2) != 2 {
		t.Errorf("got %d sessions page 2, want 2", len(sessions2))
	}
}

func TestBanEvents_InsertAndList(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC()
	if err := s.InsertBanEvent(ctx, "1.2.3.4", 100, "auth failure", now); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := s.InsertBanEvent(ctx, "5.6.7.8", 50, "too many attempts", now.Add(-time.Hour)); err != nil {
		t.Fatalf("insert 2: %v", err)
	}

	events, total, err := s.ListBanEvents(ctx, 10, 0, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(events) != 2 {
		t.Errorf("len = %d, want 2", len(events))
	}
	// Newest first
	if events[0].IP != "1.2.3.4" {
		t.Errorf("first IP = %q, want 1.2.3.4", events[0].IP)
	}
	if events[0].UnbannedAt != nil {
		t.Error("unbanned_at should be nil for active ban")
	}
}

func TestBanEvents_CloseEvent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC()
	if err := s.InsertBanEvent(ctx, "1.2.3.4", 100, "auth failure", now); err != nil {
		t.Fatalf("insert: %v", err)
	}

	unbanTime := now.Add(time.Hour)
	if err := s.CloseBanEvent(ctx, "1.2.3.4", unbanTime); err != nil {
		t.Fatalf("close: %v", err)
	}

	events, _, err := s.ListBanEvents(ctx, 10, 0, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len = %d, want 1", len(events))
	}
	if events[0].UnbannedAt == nil {
		t.Fatal("unbanned_at should be set")
	}
	if !events[0].UnbannedAt.UTC().Truncate(time.Second).Equal(unbanTime.Truncate(time.Second)) {
		t.Errorf("unbanned_at = %v, want %v", events[0].UnbannedAt, unbanTime)
	}
}

func TestBanEvents_OpenBanEvents(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC()
	if err := s.InsertBanEvent(ctx, "1.2.3.4", 100, "a", now); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := s.InsertBanEvent(ctx, "5.6.7.8", 50, "b", now); err != nil {
		t.Fatalf("insert 2: %v", err)
	}
	if err := s.CloseBanEvent(ctx, "5.6.7.8", now.Add(time.Minute)); err != nil {
		t.Fatalf("close: %v", err)
	}

	open, err := s.OpenBanEvents(ctx)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if len(open) != 1 {
		t.Fatalf("len = %d, want 1", len(open))
	}
	if open[0].IP != "1.2.3.4" {
		t.Errorf("IP = %q, want 1.2.3.4", open[0].IP)
	}
}

func TestBanEvents_ListFiltersBySince(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC()
	// Insert events at varying ages:
	// old: 48h ago, mid: 12h ago, recent: 1h ago
	old := now.Add(-48 * time.Hour)
	mid := now.Add(-12 * time.Hour)
	recent := now.Add(-1 * time.Hour)
	if err := s.InsertBanEvent(ctx, "10.0.0.1", 80, "old", old); err != nil {
		t.Fatalf("insert old: %v", err)
	}
	if err := s.InsertBanEvent(ctx, "10.0.0.2", 90, "mid", mid); err != nil {
		t.Fatalf("insert mid: %v", err)
	}
	if err := s.InsertBanEvent(ctx, "10.0.0.3", 100, "recent", recent); err != nil {
		t.Fatalf("insert recent: %v", err)
	}

	// With nil since, all events returned.
	events, total, err := s.ListBanEvents(ctx, 10, 0, nil)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 3 {
		t.Errorf("total (nil since) = %d, want 3", total)
	}
	if len(events) != 3 {
		t.Errorf("len (nil since) = %d, want 3", len(events))
	}

	// With since = 24h ago, only mid and recent should be returned.
	cutoff := now.Add(-24 * time.Hour)
	events, total, err = s.ListBanEvents(ctx, 10, 0, &cutoff)
	if err != nil {
		t.Fatalf("list with since: %v", err)
	}
	if total != 2 {
		t.Errorf("total (24h since) = %d, want 2", total)
	}
	if len(events) != 2 {
		t.Errorf("len (24h since) = %d, want 2", len(events))
	}
	// Newest first
	if len(events) >= 2 {
		if events[0].IP != "10.0.0.3" {
			t.Errorf("first IP = %q, want 10.0.0.3", events[0].IP)
		}
		if events[1].IP != "10.0.0.2" {
			t.Errorf("second IP = %q, want 10.0.0.2", events[1].IP)
		}
	}

	// With since = 6h ago, only recent should be returned.
	cutoff2 := now.Add(-6 * time.Hour)
	events, total, err = s.ListBanEvents(ctx, 10, 0, &cutoff2)
	if err != nil {
		t.Fatalf("list with 6h since: %v", err)
	}
	if total != 1 {
		t.Errorf("total (6h since) = %d, want 1", total)
	}
	if len(events) != 1 {
		t.Errorf("len (6h since) = %d, want 1", len(events))
	}
	if len(events) == 1 && events[0].IP != "10.0.0.3" {
		t.Errorf("IP = %q, want 10.0.0.3", events[0].IP)
	}
}

func TestListSessions_FiltersBySince(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// base is 30h ago so that:
	//   fid0: base+0h  = now-30h  -> before since (now-25h), excluded
	//   fid1: base+12h = now-18h  -> after  since (now-25h), included
	//   fid2: base+24h = now-6h   -> after  since (now-25h), included
	base := time.Now().UTC().Add(-30 * time.Hour)
	for i := 0; i < 3; i++ {
		if err := s.UpsertActiveSession(ctx, ActiveSessionRecord{
			OcservSessionID: fmt.Sprintf("fid%d", i),
			Username:        "bob",
			ConnectedAt:     base.Add(time.Duration(i*12) * time.Hour),
			DisconnectedAt:  base.Add(time.Duration(i*12)*time.Hour + time.Hour),
			DurationSeconds: 3600,
			BytesIn:         100,
			BytesOut:        200,
			Source:          "ingested",
			State:           "disconnected",
		}); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}

	since := time.Now().UTC().Add(-25 * time.Hour)
	sessions, total, err := s.ListSessions(ctx, since, 10, 0, "")
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(sessions) != 2 {
		t.Errorf("len = %d, want 2", len(sessions))
	}
}

func TestListSessionsUsernameFilter(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	for _, row := range []struct {
		username string
		at       time.Time
	}{
		{"alice", now.Add(-1 * time.Hour)},
		{"alice", now.Add(-2 * time.Hour)},
		{"bob", now.Add(-1 * time.Hour)},
	} {
		_, err := s.DB.ExecContext(ctx,
			`INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at)
			 VALUES (?, ?, ?, 100, 200, 60, 'ingested', ?)`,
			row.username,
			row.at.Format(time.RFC3339),
			row.at.Add(time.Hour).Format(time.RFC3339),
			now.Format(time.RFC3339),
		)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	since := now.Add(-24 * time.Hour)

	// No filter returns all 3.
	all, total, err := s.ListSessions(ctx, since, 10, 0, "")
	if err != nil {
		t.Fatalf("ListSessions no filter: %v", err)
	}
	if total != 3 {
		t.Errorf("total no filter: got %d, want 3", total)
	}
	if len(all) != 3 {
		t.Errorf("rows no filter: got %d, want 3", len(all))
	}

	// Filter by alice returns 2.
	aliceSessions, aliceTotal, err := s.ListSessions(ctx, since, 10, 0, "alice")
	if err != nil {
		t.Fatalf("ListSessions alice: %v", err)
	}
	if aliceTotal != 2 {
		t.Errorf("total alice: got %d, want 2", aliceTotal)
	}
	if len(aliceSessions) != 2 {
		t.Errorf("rows alice: got %d, want 2", len(aliceSessions))
	}
	for _, sess := range aliceSessions {
		if sess.Username != "alice" {
			t.Errorf("expected alice, got %q", sess.Username)
		}
	}
}
