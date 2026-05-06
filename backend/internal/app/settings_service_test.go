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

func TestSettingsServiceSetRetentionDaysPersists(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	cfg := config.Defaults()
	service := NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())

	// Set to 180.
	if err := service.SetRetentionDays(ctx, owner, 180); err != nil {
		t.Fatalf("SetRetentionDays(180): %v", err)
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
	service := NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())

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
	service := NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())

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
	service := NewSettingsService(&cfg, "", s, recorder, rbac.NewDefaultAuthorizer())

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

