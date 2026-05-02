package app

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

// PurgeFunc is called by the housekeeping scheduler to purge old records.
type PurgeFunc func(ctx context.Context, retentionDays int) (int64, error)

// HousekeepingScheduler runs periodic history purge based on configured retention.
type HousekeepingScheduler struct {
	store      *store.Store
	recorder   audit.Recorder
	authorizer rbac.Authorizer
	purge      PurgeFunc
	interval   time.Duration
	logger     *slog.Logger
	now        func() time.Time
}

// NewHousekeepingScheduler creates a scheduler that purges history at the given interval.
func NewHousekeepingScheduler(
	st *store.Store,
	recorder audit.Recorder,
	authorizer rbac.Authorizer,
	purge PurgeFunc,
	interval time.Duration,
	opts ...HousekeepingOption,
) *HousekeepingScheduler {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	h := &HousekeepingScheduler{
		store:      st,
		recorder:   recorder,
		authorizer: authorizer,
		purge:      purge,
		interval:   interval,
		logger:     slog.Default(),
		now:        func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// HousekeepingOption configures a HousekeepingScheduler.
type HousekeepingOption func(*HousekeepingScheduler)

// HousekeepingWithLogger sets the logger.
func HousekeepingWithLogger(logger *slog.Logger) HousekeepingOption {
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
	return days
}

// Run starts the housekeeping scheduler loop. It blocks until ctx is cancelled.
func (h *HousekeepingScheduler) Run(ctx context.Context) {
	h.logger.Info("housekeeping scheduler started", "interval", h.interval)
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// Run once immediately on start
	h.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			h.logger.Info("housekeeping scheduler stopped")
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
	h.logger.Info("housekeeping purge starting", "retention_days", retentionDays)

	deleted, err := h.purge(ctx, retentionDays)
	if err != nil {
		h.logger.Error("housekeeping purge failed", "error", err, "retention_days", retentionDays)
		return
	}
	h.logger.Info("housekeeping purge completed", "deleted_rows", deleted, "retention_days", retentionDays)
}
