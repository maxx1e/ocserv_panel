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

func TestStorePurgeHistoricalComparesParsedTimestamps(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	cutoff := time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"mixed-format-newer", "2025-12-31 12:30:00", "2025-12-31 12:30:00", 1, 1, 60, "ingested", "2025-12-31T12:30:00Z")
	if err != nil {
		t.Fatalf("insert newer mixed-format historical session: %v", err)
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO historical_sessions (username, connected_at, disconnected_at, bytes_in, bytes_out, duration_seconds, source, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"mixed-format-older", "2025-12-31 11:30:00", "2025-12-31 11:30:00", 1, 1, 60, "ingested", "2025-12-31T11:30:00Z")
	if err != nil {
		t.Fatalf("insert older mixed-format historical session: %v", err)
	}

	deleted, err := s.PurgeHistoricalBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("purge historical: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	var newerCount, olderCount int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM historical_sessions WHERE username = ?`, "mixed-format-newer").Scan(&newerCount); err != nil {
		t.Fatalf("count newer row: %v", err)
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM historical_sessions WHERE username = ?`, "mixed-format-older").Scan(&olderCount); err != nil {
		t.Fatalf("count older row: %v", err)
	}
	if newerCount != 1 || olderCount != 0 {
		t.Fatalf("newer count = %d, older count = %d, want 1 and 0", newerCount, olderCount)
	}
}
