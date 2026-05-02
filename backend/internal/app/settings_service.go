package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
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
	config        *config.Config
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
func NewSettingsService(cfg *config.Config, s *store.Store, recorder audit.Recorder, authorizer rbac.Authorizer) *SettingsService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	return &SettingsService{config: cfg, settingsStore: s, recorder: recorder, authorizer: authorizer, now: func() time.Time { return time.Now().UTC() }}
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

// SettingsSection represents a group of related settings.
type SettingsSection struct {
	ID          string       `json:"id"`
	Label       string       `json:"label"`
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Settings    []SettingDef `json:"settings"`
}

// SettingsSectionsResponse is the full API response for GET /api/settings.
type SettingsSectionsResponse struct {
	Sections        []SettingsSection `json:"sections"`
	RestartRequired bool              `json:"restart_required"`
	PendingChanges  []string          `json:"pending_changes"`
}

// GetAllSections returns all settings grouped by section with current values.
func (s *SettingsService) GetAllSections(ctx context.Context, principal auth.Principal) (*SettingsSectionsResponse, error) {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, "settings.get_sections", "setting", "", audit.OutcomeDenied, "settings get_sections authentication required", nil)
		return nil, ErrUnauthenticated
	}
	actorID := principal.ID
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionSettingsRead) {
		_ = s.record(ctx, &actorID, "settings.get_sections", "setting", "", audit.OutcomeDenied, "settings get_sections permission denied", nil)
		return nil, ErrPermissionDenied
	}

	sectionDefs := AllSectionDefs()
	defsBySection := SettingDefsBySection()
	sections := make([]SettingsSection, 0, len(sectionDefs))

	for _, sec := range sectionDefs {
		defs := defsBySection[sec.ID]
		settings := make([]SettingDef, 0, len(defs))
		for _, d := range defs {
			value := d.Default
			if d.Source == SettingSourceDatabase {
				stored, err := s.settingsStore.GetSetting(ctx, d.Key)
				if err == nil && stored != "" {
					value = stored
				}
			}
			// Mask password-type values
			if d.Type == SettingTypePassword && len(value) > 4 {
				value = "****" + value[len(value)-4:]
			}
			setting := SettingDef{
				Key:             d.Key,
				Label:           d.Label,
				Section:         d.Section,
				Type:            d.Type,
				Source:          d.Source,
				RequiresRestart: d.RequiresRestart,
				Default:         d.Default,
				Value:           value,
				Options:         d.Options,
				Validation:      d.Validation,
				SortOrder:       d.SortOrder,
			}
			settings = append(settings, setting)
		}
		sections = append(sections, SettingsSection{
			ID:          sec.ID,
			Label:       sec.Label,
			Title:       sec.Title,
			Description: sec.Description,
			Settings:    settings,
		})
	}

	if err := s.record(ctx, &actorID, "settings.get_sections", "setting", "", audit.OutcomeSuccess, "settings get_sections loaded", nil); err != nil {
		return nil, fmt.Errorf("audit settings get_sections: %w", err)
	}

	return &SettingsSectionsResponse{
		Sections:        sections,
		RestartRequired: false, // will be computed from config state
		PendingChanges:  []string{},
	}, nil
}

// UpdateSetting updates a single setting value.
func (s *SettingsService) UpdateSetting(ctx context.Context, principal auth.Principal, key, value string) (SettingDef, error) {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, "settings.update", "setting", key, audit.OutcomeDenied, "settings update authentication required", map[string]string{"value": value})
		return SettingDef{}, ErrUnauthenticated
	}
	actorID := principal.ID
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionSettingsManage) {
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeDenied, "settings update permission denied", map[string]string{"value": value})
		return SettingDef{}, ErrPermissionDenied
	}

	def, found := FindSettingDef(key)
	if !found {
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeDenied, "settings update unknown key", map[string]string{"value": value})
		return SettingDef{}, ErrInvalidInput
	}

	// Validate the value based on type
	if err := validateSettingValue(def, value); err != nil {
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeDenied, "settings update validation failed", map[string]string{"value": value, "error": err.Error()})
		return SettingDef{}, err
	}

	oldValue, _ := s.settingsStore.GetSetting(ctx, key)
	if oldValue == "" {
		oldValue = def.Default
	}

	// Store the value in app_settings regardless of source.
	// Config-sourced settings will be written to TOML in a later task.
	if err := s.settingsStore.SetSetting(ctx, key, value); err != nil {
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeFailure, "settings update save failed", map[string]string{"value": value})
		return SettingDef{}, fmt.Errorf("update setting %q: %w", key, err)
	}

	_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeSuccess, "settings update saved", map[string]string{"old_value": oldValue, "new_value": value})

	return SettingDef{
		Key:             def.Key,
		Label:           def.Label,
		Section:         def.Section,
		Type:            def.Type,
		Source:          def.Source,
		RequiresRestart: def.RequiresRestart,
		Default:         def.Default,
		Value:           value,
		Options:         def.Options,
		Validation:      def.Validation,
		SortOrder:       def.SortOrder,
	}, nil
}

// validateSettingValue checks a value against its setting definition.
func validateSettingValue(def SettingDef, value string) error {
	switch def.Type {
	case SettingTypeNumber:
		if def.Validation != "" {
			return validateNumberWithConstraints(value, def.Validation)
		}
		return nil
	case SettingTypeToggle:
		if value != "true" && value != "false" {
			return fmt.Errorf("%w: toggle must be true or false", ErrInvalidInput)
		}
		return nil
	case SettingTypeSelect:
		if len(def.Options) > 0 {
			for _, opt := range def.Options {
				if opt == value {
					return nil
				}
			}
			return fmt.Errorf("%w: value must be one of %v", ErrInvalidInput, def.Options)
		}
		return nil
	default:
		return nil
	}
}

// validateNumberWithConstraints parses a validation string like "min:1,max:65535" and checks the value.
func validateNumberWithConstraints(value string, validation string) error {
	num, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("%w: must be a number", ErrInvalidInput)
	}
	// Parse constraints from "min:X,max:Y" or "min:X" or "max:Y"
	parts := strings.Split(validation, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "min:") {
			minVal, err := strconv.Atoi(strings.TrimPrefix(part, "min:"))
			if err == nil && num < minVal {
				return fmt.Errorf("%w: must be at least %d", ErrInvalidInput, minVal)
			}
		} else if strings.HasPrefix(part, "max:") {
			maxVal, err := strconv.Atoi(strings.TrimPrefix(part, "max:"))
			if err == nil && num > maxVal {
				return fmt.Errorf("%w: must be at most %d", ErrInvalidInput, maxVal)
			}
		}
	}
	return nil
}

func (s *SettingsService) record(ctx context.Context, actorID *int64, action string, resourceType string, resourceID string, outcome audit.Outcome, message string, metadata map[string]string) error {
	return s.recorder.Record(ctx, audit.Event{ActorID: actorID, Action: action, ResourceType: resourceType, ResourceID: resourceID, Outcome: outcome, Message: message, Metadata: metadata, CreatedAt: s.now()})
}
