package app

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"ocserv_panel/backend/internal/vpn/sessions"
	"ocserv_panel/backend/internal/store"
)

// sessionIngesterStore is the narrow interface that the ingester needs from the store.
type sessionIngesterStore interface {
	UpsertActiveSession(ctx context.Context, rec store.ActiveSessionRecord) error
}

// sessionIngesterManager is the narrow interface for ocserv session data.
type sessionIngesterManager interface {
	ListConnectedUsers(ctx context.Context) ([]sessions.ConnectedUser, error)
	GetUserDetails(ctx context.Context, username string) ([]sessions.UserConnection, error)
}

// SessionIngester polls ocserv for connected users and upserts their session
// data into the historical_sessions table. This bridges the gap between
// live ocserv state and persisted session history.
type SessionIngester struct {
	manager  sessionIngesterManager
	store    sessionIngesterStore
	interval time.Duration
	logger   zerolog.Logger
	now      func() time.Time
}

// SessionIngesterOption configures a SessionIngester.
type SessionIngesterOption func(*SessionIngester)

// IngesterWithLogger sets the logger for the ingester.
func IngesterWithLogger(logger zerolog.Logger) SessionIngesterOption {
	return func(si *SessionIngester) { si.logger = logger }
}

// IngesterWithInterval sets the poll interval.
func IngesterWithInterval(d time.Duration) SessionIngesterOption {
	return func(si *SessionIngester) {
		if d > 0 {
			si.interval = d
		}
	}
}

// IngesterWithNow sets the time source (useful for testing).
func IngesterWithNow(fn func() time.Time) SessionIngesterOption {
	return func(si *SessionIngester) { si.now = fn }
}

// NewSessionIngester creates a new ingester that polls the ocserv manager and
// persists session data to the store.
func NewSessionIngester(st sessionIngesterStore, mgr sessionIngesterManager, opts ...SessionIngesterOption) *SessionIngester {
	si := &SessionIngester{
		manager:  mgr,
		store:    st,
		interval: 30 * time.Second,
		logger:   log.Logger,
		now:      func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(si)
	}
	return si
}

// Run starts the ingestion loop. It blocks until ctx is cancelled.
func (si *SessionIngester) Run(ctx context.Context) {
	si.logger.Info().Dur("interval", si.interval).Msg("session ingester started")
	ticker := time.NewTicker(si.interval)
	defer ticker.Stop()

	// Run once immediately on start.
	si.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			si.logger.Info().Msg("session ingester stopped")
			return
		case <-ticker.C:
			si.poll(ctx)
		}
	}
}

// poll performs a single ingestion cycle: fetch connected users and upsert their
// session data into the historical sessions table.
func (si *SessionIngester) poll(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}

	users, err := si.manager.ListConnectedUsers(ctx)
	if err != nil {
		si.logger.Warn().Err(err).Msg("session ingester: failed to list connected users")
		return
	}

	if len(users) == 0 {
		si.logger.Debug().Msg("session ingester: no connected users")
		return
	}

	// Deduplicate usernames (a user may have multiple connections shown as
	// separate ConnectedUser rows, but GetUserDetails returns all connections
	// for a given username in one call).
	seen := make(map[string]bool, len(users))
	var uniqueUsernames []string
	for _, u := range users {
		if u.Username != "" && !seen[u.Username] {
			seen[u.Username] = true
			uniqueUsernames = append(uniqueUsernames, u.Username)
		}
	}

	now := si.now()
	var upserted int

	for _, username := range uniqueUsernames {
		conns, err := si.manager.GetUserDetails(ctx, username)
		if err != nil {
			si.logger.Warn().Err(err).Str("username", username).Msg("session ingester: failed to get user details")
			continue
		}

		for _, conn := range conns {
			sessionID := conn.SessionID
			if sessionID == "" {
				// Without a session ID we cannot reliably upsert; skip.
				si.logger.Debug().Str("username", username).Str("device", conn.Device).Msg("session ingester: skipping connection without session ID")
				continue
			}

			connectedAt := conn.ConnectedAt
			if connectedAt.IsZero() {
				// Approximate from now (should not happen with proper parsing).
				connectedAt = now
			}

			durationSeconds := int64(now.Sub(connectedAt).Seconds())
			if durationSeconds < 0 {
				durationSeconds = 0
			}

			rec := store.ActiveSessionRecord{
				OcservSessionID: sessionID,
				Username:        username,
				ConnectedAt:     connectedAt,
				DisconnectedAt:  now,
				DurationSeconds: durationSeconds,
				BytesIn:         conn.RxBytes,
				BytesOut:        conn.TxBytes,
				Source:          "ingested",
			}

			if err := si.store.UpsertActiveSession(ctx, rec); err != nil {
				si.logger.Warn().Err(err).
					Str("username", username).
					Str("session_id", sessionID).
					Msg("session ingester: upsert failed")
				continue
			}
			upserted++
		}
	}

	si.logger.Debug().
		Int("users", len(uniqueUsernames)).
		Int("upserted", upserted).
		Msg("session ingester: poll complete")
}
