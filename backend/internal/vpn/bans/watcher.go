// Package bans provides a background watcher that reconciles the live ocserv
// ban list against a persistent ban_events table in SQLite.
package bans

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"

	"ocserv_panel/backend/internal/vpn/sessions"
)

const defaultTickInterval = 60 * time.Second

// OpenBan is the subset of a ban event needed for reconciliation.
// Using a minimal type here avoids a circular import with the store package.
type OpenBan struct {
	IP string
}

// BanStore is the narrow persistence interface required by BanWatcher.
// Implemented by an adapter in main.go that wraps *store.Store.
type BanStore interface {
	InsertBanEvent(ctx context.Context, ip string, score int, reason string, bannedAt time.Time) error
	CloseBanEvent(ctx context.Context, ip string, unbanAt time.Time) error
	OpenBanEvents(ctx context.Context) ([]OpenBan, error)
}

// banManager is the narrow interface for fetching live bans.
type banManager interface {
	ListBannedIPs(ctx context.Context) ([]sessions.BannedIP, error)
}

// BanWatcher polls ocserv for the live ban list and reconciles it against
// the persistent ban_events table.
type BanWatcher struct {
	manager      banManager
	store        BanStore
	tickInterval time.Duration
}

// NewBanWatcher constructs a BanWatcher. Call Run(ctx) in a goroutine.
func NewBanWatcher(manager banManager, store BanStore) *BanWatcher {
	return &BanWatcher{
		manager:      manager,
		store:        store,
		tickInterval: defaultTickInterval,
	}
}

// Run starts the poll loop. Blocks until ctx is cancelled.
func (w *BanWatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(w.tickInterval)
	defer ticker.Stop()

	if err := w.Sync(ctx); err != nil {
		log.Warn().Err(err).Msg("ban watcher: initial sync failed")
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.Sync(ctx); err != nil {
				log.Warn().Err(err).Msg("ban watcher: sync failed")
			}
		}
	}
}

// Sync performs one reconciliation: fetches the live ban list and the open
// ban events, inserts new bans, and closes bans that have been lifted.
func (w *BanWatcher) Sync(ctx context.Context) error {
	live, err := w.manager.ListBannedIPs(ctx)
	if err != nil {
		return err
	}

	open, err := w.store.OpenBanEvents(ctx)
	if err != nil {
		return err
	}

	liveSet := make(map[string]sessions.BannedIP, len(live))
	for _, b := range live {
		liveSet[b.IP] = b
	}

	openSet := make(map[string]bool, len(open))
	for _, o := range open {
		openSet[o.IP] = true
	}

	now := time.Now().UTC()

	// Insert bans that are live but not yet tracked.
	for _, b := range live {
		if !openSet[b.IP] {
			if err := w.store.InsertBanEvent(ctx, b.IP, b.Score, b.Reason, now); err != nil {
				log.Warn().Err(err).Str("ip", b.IP).Msg("ban watcher: insert failed")
			}
		}
	}

	// Close bans that are tracked but no longer live.
	for _, o := range open {
		if _, stillBanned := liveSet[o.IP]; !stillBanned {
			if err := w.store.CloseBanEvent(ctx, o.IP, now); err != nil {
				log.Warn().Err(err).Str("ip", o.IP).Msg("ban watcher: close failed")
			}
		}
	}

	return nil
}
