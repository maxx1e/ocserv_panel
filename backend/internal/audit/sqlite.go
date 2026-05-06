package audit

import (
	"context"

	"ocserv_panel/backend/internal/sanitize"
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
		Metadata:         sanitize.Metadata(event.Metadata),
		CreatedAt:        event.CreatedAt,
	})
}
