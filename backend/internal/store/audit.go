package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"ocserv_panel/backend/internal/sanitize"
)

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

// RecordAudit persists an audit event with JSON metadata.
func (s *Store) RecordAudit(ctx context.Context, event AuditEvent) error {
	metadata := sanitize.Metadata(event.Metadata)
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
