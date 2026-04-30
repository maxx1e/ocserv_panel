// Package audit defines audit event boundaries for administrative actions.
package audit

import (
	"context"
	"time"
)

// Outcome records the result of an administrative action.
type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
	OutcomeDenied  Outcome = "denied"
)

// Event describes an auditable administrative action.
type Event struct {
	ID           int64
	ActorID      *int64
	Action       string
	ResourceType string
	ResourceID   string
	Outcome      Outcome
	Message      string
	Metadata     map[string]string
	CreatedAt    time.Time
}

// Recorder persists audit events.
type Recorder interface {
	Record(ctx context.Context, event Event) error
}

// NoopRecorder accepts audit events without persisting them.
type NoopRecorder struct{}

// Record implements Recorder.
func (NoopRecorder) Record(ctx context.Context, event Event) error {
	_ = ctx
	_ = event
	return nil
}
