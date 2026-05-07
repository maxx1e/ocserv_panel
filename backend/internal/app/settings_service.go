package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

// Config file error sentinels for use in HTTP error mapping.
var (
	ErrConfigFileUnreadable  = errors.New("config file unreadable")
	ErrConfigFileParseFailed = errors.New("config file has syntax errors")
	ErrConfigFileMissing     = errors.New("config file missing")
)

// SettingsService manages application settings stored in app_settings.
type SettingsService struct {
	config        *config.Config
	configPath    string
	startupCfg    config.Config
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
// configPath is the path to the TOML config file for config-source settings.
// Pass an empty string to skip TOML persistence (e.g. in tests without a file).
func NewSettingsService(
	cfg *config.Config,
	configPath string,
	s *store.Store,
	recorder audit.Recorder,
	authorizer rbac.Authorizer,
) *SettingsService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	return &SettingsService{
		config:        cfg,
		configPath:    configPath,
		startupCfg:    *cfg,
		settingsStore: s,
		recorder:      recorder,
		authorizer:    authorizer,
		now:           func() time.Time { return time.Now().UTC() },
	}
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

// SettingsSection represents a group of related settings.
type SettingsSection struct {
	ID             string       `json:"id"`
	Label          string       `json:"label"`
	Title          string       `json:"title"`
	Description    string       `json:"description,omitempty"`
	Settings       []SettingDef `json:"settings"`
	RestartRequired bool        `json:"restart_required"`
	PendingChanges  []string    `json:"pending_changes,omitempty"`
}

// SettingsSectionsResponse is the full API response for GET /api/settings.
type SettingsSectionsResponse struct {
	Sections        []SettingsSection `json:"sections"`
	RestartRequired bool              `json:"restart_required"`
	PendingChanges  []string          `json:"pending_changes"`
	ConfigStatus    string            `json:"config_status"`
	ConfigError     string            `json:"config_error,omitempty"`
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

	// Load live TOML config; fall back to startup snapshot on error.
	liveCfg := s.startupCfg
	configStatus := "ok"
	var configError string
	if s.configPath != "" {
		if loaded, err := config.Load(s.configPath); err == nil {
			liveCfg = loaded
		} else {
			log.Warn().Str("path", s.configPath).Err(err).Msg("settings: failed to load TOML config for GetAllSections, using startup snapshot")
			configStatus = classifyConfigError(err)
			configError = err.Error()
		}
	}

	// Compute which config-source keys have changed since startup.
	pendingKeys := computePendingChanges(s.startupCfg, liveCfg)
	pendingSet := make(map[string]bool, len(pendingKeys))
	for _, k := range pendingKeys {
		pendingSet[k] = true
	}

	sectionDefs := AllSectionDefs()
	defsBySection := SettingDefsBySection()
	sections := make([]SettingsSection, 0, len(sectionDefs))

	for _, sec := range sectionDefs {
		defs := defsBySection[sec.ID]
		settings := make([]SettingDef, 0, len(defs))

		// Collect pending keys that belong to this section.
		var secPending []string
		for _, d := range defs {
			if d.Source == SettingSourceConfig && pendingSet[d.Key] {
				secPending = append(secPending, d.Key)
			}
		}

		for _, d := range defs {
			var value string
			switch d.Source {
			case SettingSourceConfig:
				value = configValueForKey(liveCfg, d.Key)
				if value == "" {
					value = d.Default
				}
			default:
				value = d.Default
				stored, err := s.settingsStore.GetSetting(ctx, d.Key)
				if err == nil && stored != "" {
					value = stored
				}
			}
			// Mask password-type values.
			if d.Type == SettingTypePassword && len(value) > 4 {
				value = "****" + value[len(value)-4:]
			}
			settings = append(settings, SettingDef{
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
			})
		}

		sec := SettingsSection{
			ID:              sec.ID,
			Label:           sec.Label,
			Title:           sec.Title,
			Description:     sec.Description,
			Settings:        settings,
			RestartRequired: len(secPending) > 0,
			PendingChanges:  secPending,
		}
		sections = append(sections, sec)
	}

	if err := s.record(ctx, &actorID, "settings.get_sections", "setting", "", audit.OutcomeSuccess, "settings get_sections loaded", nil); err != nil {
		return nil, fmt.Errorf("audit settings get_sections: %w", err)
	}

	return &SettingsSectionsResponse{
		Sections:        sections,
		RestartRequired: len(pendingKeys) > 0,
		PendingChanges:  pendingKeys,
		ConfigStatus:    configStatus,
		ConfigError:     configError,
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
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeDenied, "settings update validation failed", map[string]string{"value": auditValue(def, value), "error": err.Error()})
		return SettingDef{}, err
	}

	// Security validation: enforce allow-lists for sensitive keys.
	if err := validateSettingValueSecurity(key, value); err != nil {
		_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeDenied, "settings update security validation failed", map[string]string{"value": auditValue(def, value), "error": err.Error()})
		return SettingDef{}, err
	}

	oldValue, _ := s.settingsStore.GetSetting(ctx, key)
	if oldValue == "" {
		oldValue = def.Default
	}

	switch def.Source {
	case SettingSourceConfig:
		// Config-source settings are persisted to the TOML file only.
		if s.configPath == "" {
			_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeFailure, "settings update config path not set", map[string]string{"value": auditValue(def, value)})
			return SettingDef{}, fmt.Errorf("update setting %q: config path is not configured", key)
		}
		current, err := config.LoadStrict(s.configPath)
		if err != nil {
			_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeFailure, "settings update load config failed", map[string]string{"value": auditValue(def, value)})
			return SettingDef{}, fmt.Errorf("update setting %q: %w", key, wrapConfigLoadError(err))
		}
		// For config-source settings, get the real prior value from the TOML, not SQLite.
		oldValue = configValueForKey(current, key)
		if err := applySettingToConfig(&current, key, value); err != nil {
			_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeFailure, "settings update apply failed", map[string]string{"value": auditValue(def, value)})
			return SettingDef{}, err
		}
		if err := config.Validate(current); err != nil {
			_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeDenied, "settings update validation failed", map[string]string{"value": auditValue(def, value), "error": err.Error()})
			return SettingDef{}, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
		}
		if err := config.Save(current, s.configPath); err != nil {
			_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeFailure, "settings update save config failed", map[string]string{"value": auditValue(def, value)})
			return SettingDef{}, fmt.Errorf("update setting %q: save config: %w", key, err)
		}
	default:
		// Database-source settings are persisted to SQLite only.
		if err := s.settingsStore.SetSetting(ctx, key, value); err != nil {
			_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeFailure, "settings update save failed", map[string]string{"value": auditValue(def, value)})
			return SettingDef{}, fmt.Errorf("update setting %q: %w", key, err)
		}
	}

	_ = s.record(ctx, &actorID, "settings.update", "setting", key, audit.OutcomeSuccess, "settings update saved", map[string]string{"old_value": auditValue(def, oldValue), "new_value": auditValue(def, value)})

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

// auditValue returns a safe string for audit logging: redacts values for
// password-type settings so TLS key paths and DSNs are not logged verbatim.
func auditValue(def SettingDef, value string) string {
	if def.Type == SettingTypePassword {
		return "[redacted]"
	}
	return value
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

// RestartStatus reports whether a service restart is needed to apply pending
// config-source changes and returns the list of changed keys.
func (s *SettingsService) RestartStatus(ctx context.Context, principal auth.Principal) (restartRequired bool, pendingChanges []string, err error) {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, "settings.restart_status", "setting", "", audit.OutcomeDenied, "restart status authentication required", nil)
		return false, nil, ErrUnauthenticated
	}
	actorID := principal.ID
	if !s.authorizer.Allowed(principalRoles(principal), rbac.PermissionSettingsRead) {
		_ = s.record(ctx, &actorID, "settings.restart_status", "setting", "", audit.OutcomeDenied, "restart status permission denied", nil)
		return false, nil, ErrPermissionDenied
	}

	liveCfg := s.startupCfg
	if s.configPath != "" {
		if loaded, loadErr := config.Load(s.configPath); loadErr == nil {
			liveCfg = loaded
		} else {
			log.Warn().Str("path", s.configPath).Err(loadErr).Msg("settings: failed to load TOML config for RestartStatus, using startup snapshot")
		}
	}

	pending := computePendingChanges(s.startupCfg, liveCfg)
	restartRequired = len(pending) > 0
	_ = s.record(ctx, &actorID, "settings.restart_status", "setting", "", audit.OutcomeSuccess, "restart status checked", map[string]string{
		"restart_required": strconv.FormatBool(restartRequired),
		"pending_count":    strconv.Itoa(len(pending)),
	})
	return restartRequired, pending, nil
}

// classifyConfigError inspects a config loading error and returns a status
// string suitable for the config_status response field.
func classifyConfigError(err error) string {
	if errors.Is(err, os.ErrNotExist) {
		return "file_missing"
	}
	if errors.Is(err, os.ErrPermission) {
		return "permission_error"
	}
	if strings.Contains(err.Error(), "parse") {
		return "parse_error"
	}
	return "read_error"
}

// wrapConfigLoadError wraps a raw config load error with the appropriate
// sentinel so HTTP handlers can map it to a specific status code.
func wrapConfigLoadError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %s", ErrConfigFileMissing, err.Error())
	}
	if errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("%w: %s", ErrConfigFileUnreadable, err.Error())
	}
	if strings.Contains(err.Error(), "parse") {
		return fmt.Errorf("%w: %s", ErrConfigFileParseFailed, err.Error())
	}
	return fmt.Errorf("%w: %s", ErrConfigFileUnreadable, err.Error())
}
