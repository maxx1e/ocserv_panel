package app

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/rbac"
)

func TestSettingsServiceGetRetentionDaysDefault(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, s, recorder, rbac.NewDefaultAuthorizer())

	days := service.GetRetentionDays(ctx)

	if days != 365 {
		t.Fatalf("GetRetentionDays = %d, want 365 as default", days)
	}

	// Empty store produces no setting, so default applies.
	value, err := s.GetSetting(ctx, "history.retention_days")
	if err != nil {
		t.Fatalf("store GetSetting: %v", err)
	}
	if value != "" {
		t.Fatalf("store setting = %q, want empty", value)
	}
}

func TestSettingsServiceSetAndGetRetentionDays(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, s, recorder, rbac.NewDefaultAuthorizer())

	// Set to 180.
	if err := service.SetRetentionDays(ctx, owner, 180); err != nil {
		t.Fatalf("SetRetentionDays(180): %v", err)
	}

	// Read back via GetRetentionDays.
	days := service.GetRetentionDays(ctx)
	if days != 180 {
		t.Fatalf("GetRetentionDays = %d, want 180", days)
	}

	// Verify the raw store value.
	stored, err := s.GetSetting(ctx, "history.retention_days")
	if err != nil {
		t.Fatalf("store GetSetting: %v", err)
	}
	if stored != "180" {
		t.Fatalf("stored setting = %q, want %q", stored, "180")
	}

	// Verify audit.
	events := mustRecentAuditEvents(t, s, 1)
	assertAuditOutcome(t, events, "settings.set_retention", string(audit.OutcomeSuccess))
	if events[0].ActorAdminUserID == nil || *events[0].ActorAdminUserID != owner.ID {
		t.Fatalf("actor id = %v, want %d", events[0].ActorAdminUserID, owner.ID)
	}
}

func TestSettingsServiceSetRetentionDaysValidatesAllowed(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, s, recorder, rbac.NewDefaultAuthorizer())

	invalidValues := []int{0, 1, 7, 60, 100, 730}
	for _, days := range invalidValues {
		t.Run(strconv.Itoa(days), func(t *testing.T) {
			if err := service.SetRetentionDays(ctx, owner, days); !errors.Is(err, ErrInvalidRetention) {
				t.Fatalf("SetRetentionDays(%d) err = %v, want ErrInvalidRetention", days, err)
			}
		})
	}

	// Verify no setting was written for the first invalid value tried.
	value, err := s.GetSetting(ctx, "history.retention_days")
	if err != nil {
		t.Fatalf("store GetSetting: %v", err)
	}
	if value != "" {
		t.Fatalf("store setting = %q, want empty after all invalid attempts", value)
	}

	// Verify audit for each rejected value.
	events := mustRecentAuditEvents(t, s, len(invalidValues))
	for i, days := range invalidValues {
		found := false
		for _, event := range events {
			if event.Action == "settings.set_retention" && event.Outcome == string(audit.OutcomeDenied) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no denied audit event for invalid retention %d (event %d)", days, i)
		}
	}
}

func TestSettingsServiceSetRetentionDaysRequiresAuth(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, s, recorder, rbac.NewDefaultAuthorizer())

	err := service.SetRetentionDays(ctx, auth.Principal{}, 90)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("unauthenticated SetRetentionDays err = %v, want ErrUnauthenticated", err)
	}

	events := mustRecentAuditEvents(t, s, 1)
	assertAuditOutcome(t, events, "settings.set_retention", string(audit.OutcomeDenied))
	if events[0].ActorAdminUserID != nil {
		t.Fatalf("actor id = %v, want nil for unauthenticated set", *events[0].ActorAdminUserID)
	}
}

func TestSettingsServiceSetRetentionDaysRequiresOwner(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	viewer := createAdminWithRole(t, ctx, s, "viewer", rbac.RoleViewer)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, s, recorder, rbac.NewDefaultAuthorizer())

	err := service.SetRetentionDays(ctx, viewer, 90)
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("viewer SetRetentionDays err = %v, want ErrPermissionDenied", err)
	}

	events := mustRecentAuditEvents(t, s, 1)
	assertAuditOutcome(t, events, "settings.set_retention", string(audit.OutcomeDenied))
	if events[0].ActorAdminUserID == nil || *events[0].ActorAdminUserID != viewer.ID {
		t.Fatalf("actor id = %v, want %d", events[0].ActorAdminUserID, viewer.ID)
	}
}

func TestSettingsServiceGetAllSettings(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, s, recorder, rbac.NewDefaultAuthorizer())

	// Initially empty.
	settings, err := service.GetAll(ctx, owner)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	// "history.retention_days" should be present (with default value parsed).
	if len(settings) == 0 {
		t.Fatal("GetAll returned empty settings, want at least history.retention_days")
	}
	found := false
	for _, setting := range settings {
		if setting.Key == "history.retention_days" {
			found = true
			if setting.Value != "365" {
				t.Fatalf("history.retention_days = %q, want %q", setting.Value, "365")
			}
		}
	}
	if !found {
		t.Fatal("GetAll missing history.retention_days setting")
	}

	// After setting a value, GetAll should reflect it.
	if err := service.SetRetentionDays(ctx, owner, 180); err != nil {
		t.Fatalf("SetRetentionDays(180): %v", err)
	}

	settings, err = service.GetAll(ctx, owner)
	if err != nil {
		t.Fatalf("GetAll after set: %v", err)
	}
	found = false
	for _, setting := range settings {
		if setting.Key == "history.retention_days" {
			found = true
			if setting.Value != "180" {
				t.Fatalf("history.retention_days = %q, want %q", setting.Value, "180")
			}
		}
	}
	if !found {
		t.Fatal("GetAll missing history.retention_days after set")
	}

	// Verify audit for GetAll.
	events := mustRecentAuditEvents(t, s, 3) // 2 get_all + 1 set_retention
	assertAuditOutcome(t, events, "settings.set_retention", string(audit.OutcomeSuccess))
	assertAuditOutcome(t, events, "settings.get_all", string(audit.OutcomeSuccess))
}
