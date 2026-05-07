package app

import (
	"context"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/store"
)

// PurgeFunc is called by the housekeeping scheduler to purge old records.
type PurgeFunc func(ctx context.Context, retentionDays int) (int64, error)

// HousekeepingScheduler runs periodic history purge based on configured retention.
type HousekeepingScheduler struct {
	store    *store.Store
	recorder audit.Recorder
	purge    PurgeFunc
	interval time.Duration
	logger   zerolog.Logger
}

// NewHousekeepingScheduler creates a scheduler that purges history at the given interval.
func NewHousekeepingScheduler(
	st *store.Store,
	recorder audit.Recorder,
	purge PurgeFunc,
	interval time.Duration,
	opts ...HousekeepingOption,
) *HousekeepingScheduler {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	h := &HousekeepingScheduler{
		store:    st,
		recorder: recorder,
		purge:    purge,
		interval: interval,
		logger:   log.Logger,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// HousekeepingOption configures a HousekeepingScheduler.
type HousekeepingOption func(*HousekeepingScheduler)

// HousekeepingWithLogger sets the logger.
func HousekeepingWithLogger(logger zerolog.Logger) HousekeepingOption {
	return func(h *HousekeepingScheduler) { h.logger = logger }
}

// GetRetentionDays returns the configured retention period, defaulting to 365.
func (h *HousekeepingScheduler) GetRetentionDays(ctx context.Context) int {
	value, err := h.store.GetSetting(ctx, "history.retention_days")
	if err != nil || value == "" {
		return 365
	}
	days, atoiErr := strconv.Atoi(value)
	if atoiErr != nil {
		return 365
	}
	if days <= 0 {
		return 365
	}
	return days
}

// Run starts the housekeeping scheduler loop. It blocks until ctx is cancelled.
func (h *HousekeepingScheduler) Run(ctx context.Context) {
	h.logger.Info().Dur("interval", h.interval).Msg("housekeeping scheduler started")
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// Run once immediately on start
	h.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			h.logger.Info().Msg("housekeeping scheduler stopped")
			return
		case <-ticker.C:
			h.runOnce(ctx)
		}
	}
}

func (h *HousekeepingScheduler) runOnce(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	retentionDays := h.GetRetentionDays(ctx)
	h.logger.Info().Int("retention_days", retentionDays).Msg("housekeeping purge starting")

	deleted, err := h.purge(ctx, retentionDays)
	if err != nil {
		h.logger.Error().Err(err).Int("retention_days", retentionDays).Msg("housekeeping purge failed")
		_ = h.recorder.Record(ctx, audit.Event{
			Action:       "housekeeping.purge",
			ResourceType: "historical_session",
			Outcome:      audit.OutcomeFailure,
			Message:      "housekeeping purge failed",
			Metadata:     map[string]string{"retention_days": strconv.Itoa(retentionDays), "error": err.Error()},
			CreatedAt:    time.Now().UTC(),
		})
		return
	}
	h.logger.Info().Int64("deleted_rows", deleted).Int("retention_days", retentionDays).Msg("housekeeping purge completed")
	_ = h.recorder.Record(ctx, audit.Event{
		Action:       "housekeeping.purge",
		ResourceType: "historical_session",
		Outcome:      audit.OutcomeSuccess,
		Message:      "housekeeping purge completed",
		Metadata:     map[string]string{"retention_days": strconv.Itoa(retentionDays), "deleted_rows": strconv.FormatInt(deleted, 10)},
		CreatedAt:    time.Now().UTC(),
	})
}
