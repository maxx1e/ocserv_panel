package app

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

// Setting represents a single application setting key-value pair.
type Setting struct {
	Key   string
	Value string
}

// SettingsService manages application settings stored in app_settings.
type SettingsService struct {
	settingsStore settingsStore
	recorder      audit.Recorder
	authorizer    rbac.Authorizer
	now           func() time.Time
}

type settingsStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key string, value string) error
}

// NewSettingsService returns a SettingsService backed by SQLite storage.
func NewSettingsService(s *store.Store, recorder audit.Recorder, authorizer rbac.Authorizer) *SettingsService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	return &SettingsService{settingsStore: s, recorder: recorder, authorizer: authorizer, now: func() time.Time { return time.Now().UTC() }}
}

// GetRetentionDays returns the configured history retention in days, defaulting
// to 365 when no setting is stored or the stored value cannot be parsed.
// This method does not require authentication.
func (s *SettingsService) GetRetentionDays(ctx context.Context) int {
	value, err := s.settingsStore.GetSetting(ctx, "history.retention_days")
	if err != nil || value == "" {
		return 365
	}
	days, err := strconv.Atoi(value)
	if err != nil {
		return 365
	}
	if days <= 0 {
		return 365
	}
	return days
}

// SetRetentionDays sets the history retention window with validation and audit.
func (s *SettingsService) SetRetentionDays(ctx context.Context, principal auth.Principal, days int) error {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, "settings.set_retention", "setting", "history.retention_days", audit.OutcomeDenied, "retention setting authentication required", map[string]string{"retention_days": fmt.Sprint(days)})
		return ErrUnauthenticated
	}
	actorID := principal.ID
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionSettingsManage) {
		_ = s.record(ctx, &actorID, "settings.set_retention", "setting", "history.retention_days", audit.OutcomeDenied, "retention setting permission denied", map[string]string{"retention_days": fmt.Sprint(days)})
		return ErrPermissionDenied
	}
	if !allowedRetentionDays(days) {
		_ = s.record(ctx, &actorID, "settings.set_retention", "setting", "history.retention_days", audit.OutcomeDenied, "retention setting validation rejected", map[string]string{"retention_days": fmt.Sprint(days)})
		return ErrInvalidRetention
	}

	value := strconv.Itoa(days)
	if err := s.settingsStore.SetSetting(ctx, "history.retention_days", value); err != nil {
		_ = s.record(ctx, &actorID, "settings.set_retention", "setting", "history.retention_days", audit.OutcomeFailure, "retention setting save failed", map[string]string{"retention_days": fmt.Sprint(days)})
		return fmt.Errorf("set retention days: %w", err)
	}
	if err := s.record(ctx, &actorID, "settings.set_retention", "setting", "history.retention_days", audit.OutcomeSuccess, "retention setting saved", map[string]string{"retention_days": fmt.Sprint(days)}); err != nil {
		return fmt.Errorf("audit retention setting: %w", err)
	}
	return nil
}

// knownSettingKeys enumerates application settings exposed via GetAll.
var knownSettingKeys = []string{
	"history.retention_days",
}

// GetAll returns all known application settings visible to the principal.
func (s *SettingsService) GetAll(ctx context.Context, principal auth.Principal) ([]Setting, error) {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, "settings.get_all", "setting", "", audit.OutcomeDenied, "settings get_all authentication required", nil)
		return nil, ErrUnauthenticated
	}
	actorID := principal.ID
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionReadDashboard) {
		_ = s.record(ctx, &actorID, "settings.get_all", "setting", "", audit.OutcomeDenied, "settings get_all permission denied", nil)
		return nil, ErrPermissionDenied
	}

	settings := make([]Setting, 0, len(knownSettingKeys))
	for _, key := range knownSettingKeys {
		value, err := s.settingsStore.GetSetting(ctx, key)
		if err != nil {
			_ = s.record(ctx, &actorID, "settings.get_all", "setting", key, audit.OutcomeFailure, "settings get_all read failed", nil)
			return nil, fmt.Errorf("get setting %q: %w", key, err)
		}
		if value == "" {
			value = defaultForSetting(key)
		}
		settings = append(settings, Setting{Key: key, Value: value})
	}

	if err := s.record(ctx, &actorID, "settings.get_all", "setting", "", audit.OutcomeSuccess, "settings get_all loaded", nil); err != nil {
		return nil, fmt.Errorf("audit settings get_all: %w", err)
	}
	return settings, nil
}

// defaultForSetting returns the default value for a known setting key.
func defaultForSetting(key string) string {
	switch key {
	case "history.retention_days":
		return "365"
	default:
		return ""
	}
}

func (s *SettingsService) record(ctx context.Context, actorID *int64, action string, resourceType string, resourceID string, outcome audit.Outcome, message string, metadata map[string]string) error {
	return s.recorder.Record(ctx, audit.Event{ActorID: actorID, Action: action, ResourceType: resourceType, ResourceID: resourceID, Outcome: outcome, Message: message, Metadata: metadata, CreatedAt: s.now()})
}
