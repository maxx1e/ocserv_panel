package app

import (
	"context"
	"errors"
	"testing"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/rbac"
)

func TestSettingsServiceGetAllSectionsRequiresAuth(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	service := NewSettingsService(s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.GetAllSections(ctx, auth.Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("unauthenticated GetAllSections err = %v, want ErrUnauthenticated", err)
	}
}

func TestSettingsServiceGetAllSectionsRequiresPermission(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	viewer := createAdminWithRole(t, ctx, s, "viewer", rbac.RoleViewer)
	service := NewSettingsService(s, recorder, rbac.NewDefaultAuthorizer())

	// Viewer has settings:read permission, so this should succeed
	_, err := service.GetAllSections(ctx, viewer)
	if err != nil {
		t.Fatalf("viewer GetAllSections should succeed with settings:read, got err = %v", err)
	}
}

func TestSettingsServiceGetAllSectionsReturnsAllSections(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	service := NewSettingsService(s, recorder, rbac.NewDefaultAuthorizer())

	resp, err := service.GetAllSections(ctx, owner)
	if err != nil {
		t.Fatalf("GetAllSections: %v", err)
	}
	if len(resp.Sections) != 12 {
		t.Fatalf("expected 12 sections, got %d", len(resp.Sections))
	}
	// Verify first section is "general"
	if resp.Sections[0].ID != "general" {
		t.Fatalf("first section ID = %q, want %q", resp.Sections[0].ID, "general")
	}
	// Verify general section has settings
	if len(resp.Sections[0].Settings) == 0 {
		t.Fatal("general section should have settings")
	}
}

func TestSettingsServiceUpdateSettingRequiresAuth(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	service := NewSettingsService(s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.UpdateSetting(ctx, auth.Principal{}, "general.theme", "dark")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("unauthenticated UpdateSetting err = %v, want ErrUnauthenticated", err)
	}
}

func TestSettingsServiceUpdateSettingRequiresOwner(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	viewer := createAdminWithRole(t, ctx, s, "viewer", rbac.RoleViewer)
	service := NewSettingsService(s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.UpdateSetting(ctx, viewer, "general.theme", "dark")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("viewer UpdateSetting err = %v, want ErrPermissionDenied", err)
	}
}

func TestSettingsServiceUpdateSettingRejectsUnknownKey(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	service := NewSettingsService(s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.UpdateSetting(ctx, owner, "unknown.key", "value")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown key UpdateSetting err = %v, want ErrInvalidInput", err)
	}
}

func TestSettingsServiceUpdateSettingSavesDatabaseSetting(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	service := NewSettingsService(s, recorder, rbac.NewDefaultAuthorizer())

	result, err := service.UpdateSetting(ctx, owner, "general.theme", "dark")
	if err != nil {
		t.Fatalf("UpdateSetting: %v", err)
	}
	if result.Key != "general.theme" {
		t.Fatalf("result.Key = %q, want %q", result.Key, "general.theme")
	}

	// Verify stored value
	stored, err := s.GetSetting(ctx, "general.theme")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if stored != "dark" {
		t.Fatalf("stored = %q, want %q", stored, "dark")
	}
}

func TestSettingsServiceUpdateSettingValidatesToggle(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	service := NewSettingsService(s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.UpdateSetting(ctx, owner, "security.csrf_enabled", "maybe")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid toggle UpdateSetting err = %v, want ErrInvalidInput", err)
	}
}

func TestSettingsServiceUpdateSettingValidatesSelect(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	service := NewSettingsService(s, recorder, rbac.NewDefaultAuthorizer())

	_, err := service.UpdateSetting(ctx, owner, "general.theme", "invalid_theme")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid select UpdateSetting err = %v, want ErrInvalidInput", err)
	}
}

func TestSettingsServiceUpdateSettingValidatesNumber(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	owner := createAdminWithRole(t, ctx, s, "owner", rbac.RoleOwner)
	service := NewSettingsService(s, recorder, rbac.NewDefaultAuthorizer())

	// Port must be between 1 and 65535
	_, err := service.UpdateSetting(ctx, owner, "web.port", "0")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("port 0 UpdateSetting err = %v, want ErrInvalidInput", err)
	}

	_, err = service.UpdateSetting(ctx, owner, "web.port", "99999")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("port 99999 UpdateSetting err = %v, want ErrInvalidInput", err)
	}

	// Valid port should succeed
	_, err = service.UpdateSetting(ctx, owner, "web.port", "8080")
	if err != nil {
		t.Fatalf("valid port UpdateSetting err = %v", err)
	}
}

func TestValidateNumberWithConstraints(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		validation string
		wantErr    bool
	}{
		{"valid number no constraints", "42", "", false},
		{"invalid number", "abc", "", true},
		{"min constraint pass", "5", "min:1", false},
		{"min constraint fail", "0", "min:1", true},
		{"max constraint pass", "100", "max:65535", false},
		{"max constraint fail", "70000", "max:65535", true},
		{"min and max pass", "8080", "min:1,max:65535", false},
		{"min and max fail low", "0", "min:1,max:65535", true},
		{"min and max fail high", "99999", "min:1,max:65535", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNumberWithConstraints(tt.value, tt.validation)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNumberWithConstraints(%q, %q) error = %v, wantErr %v", tt.value, tt.validation, err, tt.wantErr)
			}
		})
	}
}
