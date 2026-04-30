package audit

import (
	"context"
	"strings"

	"ocserv_panel/backend/internal/store"
)

// AuditStore is the storage boundary required by SQLiteRecorder.
type AuditStore interface {
	RecordAudit(context.Context, store.AuditEvent) error
}

// SQLiteRecorder persists audit events through the SQLite store.
type SQLiteRecorder struct {
	Store AuditStore
}

// NewSQLiteRecorder returns a recorder backed by s.
func NewSQLiteRecorder(s *store.Store) SQLiteRecorder {
	return SQLiteRecorder{Store: s}
}

// Record implements Recorder.
func (r SQLiteRecorder) Record(ctx context.Context, event Event) error {
	return r.Store.RecordAudit(ctx, store.AuditEvent{
		ID:               event.ID,
		ActorAdminUserID: event.ActorID,
		Action:           event.Action,
		ResourceType:     event.ResourceType,
		ResourceID:       event.ResourceID,
		Outcome:          string(event.Outcome),
		Message:          event.Message,
		Metadata:         sanitizeMetadata(event.Metadata),
		CreatedAt:        event.CreatedAt,
	})
}

func sanitizeMetadata(metadata map[string]string) map[string]string {
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
