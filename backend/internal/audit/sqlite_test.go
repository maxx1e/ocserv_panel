package audit

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ocserv_panel/backend/internal/db"
	"ocserv_panel/backend/internal/store"
)

func newTestRecorder(t *testing.T) (*store.Store, SQLiteRecorder) {
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

	s := store.New(database)
	return s, NewSQLiteRecorder(s)
}

func TestSQLiteRecorderPersistsAuditEvent(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestRecorder(t)
	admin, err := s.CreateFirstOwner(ctx, "owner", "argon2id-hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	actorID := admin.ID
	eventTime := time.Now().UTC().Truncate(time.Second)
	if err := recorder.Record(ctx, Event{
		ActorID:      &actorID,
		Action:       "config.validate",
		ResourceType: "config",
		ResourceID:   "ocserv.conf",
		Outcome:      OutcomeSuccess,
		Message:      "validation completed",
		Metadata:     map[string]string{"field": "redacted", "result": "ok"},
		CreatedAt:    eventTime,
	}); err != nil {
		t.Fatalf("record audit event: %v", err)
	}

	var action, metadataJSON string
	var actor sql.NullInt64
	if err := s.DB.QueryRowContext(ctx, `SELECT actor_admin_user_id, action, metadata_json FROM audit_log ORDER BY id DESC LIMIT 1`).Scan(&actor, &action, &metadataJSON); err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if !actor.Valid || actor.Int64 != actorID {
		t.Fatalf("actor = %#v, want %d", actor, actorID)
	}
	if action != "config.validate" {
		t.Fatalf("action = %q, want config.validate", action)
	}
	if metadataJSON != `{"field":"redacted","result":"ok"}` && metadataJSON != `{"result":"ok","field":"redacted"}` {
		t.Fatalf("metadata json = %s", metadataJSON)
	}
	for _, forbidden := range []string{"plaintext-password", "raw-token"} {
		if strings.Contains(metadataJSON, forbidden) {
			t.Fatalf("metadata contains forbidden plaintext value %q: %s", forbidden, metadataJSON)
		}
	}
}

func TestSQLiteRecorderDoesNotRequireActor(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestRecorder(t)

	if err := recorder.Record(ctx, Event{
		Action:       "system.startup",
		ResourceType: "system",
		Outcome:      OutcomeSuccess,
		Message:      "startup check",
		Metadata:     map[string]string{"field": "redacted"},
		CreatedAt:    time.Now().UTC(),
	}); err != nil {
		t.Fatalf("record audit event without actor: %v", err)
	}

	var actor sql.NullInt64
	var metadataJSON string
	if err := s.DB.QueryRowContext(ctx, `SELECT actor_admin_user_id, metadata_json FROM audit_log ORDER BY id DESC LIMIT 1`).Scan(&actor, &metadataJSON); err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if actor.Valid {
		t.Fatalf("actor valid = true, want null actor")
	}
	if metadataJSON != `{"field":"redacted"}` {
		t.Fatalf("metadata json = %s, want field redacted JSON", metadataJSON)
	}
	if strings.Contains(metadataJSON, "plaintext-password") || strings.Contains(metadataJSON, "raw-token") {
		t.Fatalf("metadata contains forbidden plaintext value: %s", metadataJSON)
	}
}

func TestSQLiteRecorderRedactsSensitiveMetadata(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestRecorder(t)

	if err := recorder.Record(ctx, Event{
		Action:       "security.test",
		ResourceType: "audit",
		Outcome:      OutcomeSuccess,
		Message:      "redaction test",
		Metadata: map[string]string{
			"password":        "plaintext-password",
			"session_token":   "raw-token",
			"private-key":     "hyphen-private-key-value",
			"ssh-private-key": "ssh-private-key-value",
			"description":     "-----BEGIN PRIVATE KEY-----\nprivate-key-material\n-----END PRIVATE KEY-----",
			"rsa_pem":         "-----BEGIN RSA PRIVATE KEY-----\nrsa-key-material\n-----END RSA PRIVATE KEY-----",
			"openssh_pem":     "-----BEGIN OPENSSH PRIVATE KEY-----\nopenssh-key-material\n-----END OPENSSH PRIVATE KEY-----",
			"field":           "safe-value",
		},
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("record audit event: %v", err)
	}

	var metadataJSON string
	if err := s.DB.QueryRowContext(ctx, `SELECT metadata_json FROM audit_log ORDER BY id DESC LIMIT 1`).Scan(&metadataJSON); err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	for _, forbidden := range []string{"plaintext-password", "raw-token", "hyphen-private-key-value", "ssh-private-key-value", "BEGIN PRIVATE KEY", "private-key-material", "BEGIN RSA PRIVATE KEY", "rsa-key-material", "BEGIN OPENSSH PRIVATE KEY", "openssh-key-material"} {
		if strings.Contains(metadataJSON, forbidden) {
			t.Fatalf("metadata contains forbidden plaintext value %q: %s", forbidden, metadataJSON)
		}
	}
	for _, expected := range []string{`"password":"[redacted]"`, `"session_token":"[redacted]"`, `"private-key":"[redacted]"`, `"ssh-private-key":"[redacted]"`, `"description":"[redacted]"`, `"rsa_pem":"[redacted]"`, `"openssh_pem":"[redacted]"`, `"field":"safe-value"`} {
		if !strings.Contains(metadataJSON, expected) {
			t.Fatalf("metadata json = %s, want fragment %s", metadataJSON, expected)
		}
	}
}
