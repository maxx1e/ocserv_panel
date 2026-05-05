package app_test

import (
	"context"
	"testing"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/ocserv"
	"ocserv_panel/backend/internal/rbac"
)

func TestHealthServiceSystemHealth_HappyPath(t *testing.T) {
	t.Parallel()

	checker := ocserv.NewReadinessChecker(
		func() ocserv.ReadinessConfig {
			return ocserv.ReadinessConfig{
				OcservBinPath: "/nonexistent/ocserv",
				ConfigPath:    "/nonexistent/ocserv.conf",
				SocketPath:    "/nonexistent/ocserv.sock",
			}
		},
		ocserv.NewFakeCommandRunner(),
		ocserv.NewFakeServiceManager(ocserv.ServiceActive),
	)

	cfg := config.Defaults()
	cfgFn := func() config.Config { return cfg }

	svc := app.NewHealthService(checker, stubAdminCounter(2), cfgFn, rbac.NewDefaultAuthorizer())

	principal := auth.Principal{ID: 1, Username: "owner", Roles: []string{"owner"}}
	result, err := svc.SystemHealth(context.Background(), principal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Panel.BackendStatus != "ok" {
		t.Errorf("expected panel.backendStatus=ok, got %q", result.Panel.BackendStatus)
	}
	if result.Panel.WebuiUserCount != 2 {
		t.Errorf("expected panel.webuiUserCount=2, got %d", result.Panel.WebuiUserCount)
	}
}

func TestHealthServiceSystemHealth_Unauthenticated(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	svc := app.NewHealthService(nil, stubAdminCounter(0), func() config.Config { return cfg }, rbac.NewDefaultAuthorizer())

	_, err := svc.SystemHealth(context.Background(), auth.Principal{})
	if err != app.ErrUnauthenticated {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestHealthServiceSystemHealth_PermissionDenied(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	svc := app.NewHealthService(nil, stubAdminCounter(0), func() config.Config { return cfg }, rbac.NewDefaultAuthorizer())

	// unknown_role has no permissions
	principal2 := auth.Principal{ID: 3, Username: "noperm", Roles: []string{"unknown_role"}}
	_, err := svc.SystemHealth(context.Background(), principal2)
	if err != app.ErrPermissionDenied {
		t.Fatalf("expected ErrPermissionDenied for unknown role, got %v", err)
	}
	// viewer should succeed (has PermissionReadDashboard)
	viewer := auth.Principal{ID: 2, Username: "viewer", Roles: []string{"viewer"}}
	_, err2 := svc.SystemHealth(context.Background(), viewer)
	if err2 != nil {
		t.Fatalf("viewer should have read dashboard perm: %v", err2)
	}
}

// stubAdminCounter is a minimal adminCounter for tests.
type stubAdminCounter int64

func (s stubAdminCounter) AdminCount(_ context.Context) (int64, error) {
	return int64(s), nil
}
