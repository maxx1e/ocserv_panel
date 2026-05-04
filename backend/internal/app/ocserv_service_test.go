package app

import (
	"context"
	"errors"
	"testing"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/ocserv"
	"ocserv_panel/backend/internal/rbac"
)

// newTestOcservService creates an OcservService with all-fake dependencies.
func newTestOcservService(t *testing.T, svcStatus ocserv.ServiceStatus) (
	*OcservService,
	*ocserv.FakeManager,
	*ocserv.FakePasswdManager,
	*ocserv.FakeServiceManager,
) {
	t.Helper()

	runner := ocserv.NewFakeCommandRunner()
	fakeSvc := ocserv.NewFakeServiceManager(svcStatus)
	checker := ocserv.NewReadinessChecker(ocserv.ReadinessConfig{}, runner, fakeSvc)

	fakeManager := ocserv.NewFakeManager()
	fakeManager.DisconnectErr = nil // allow disconnect by default in service tests

	fakePasswd := ocserv.NewFakePasswdManager()
	validator := ocserv.NewConfigValidator("/nonexistent/ocserv.conf")

	svc := NewOcservService(
		checker,
		fakeManager,
		fakePasswd,
		fakeSvc,
		validator,
		audit.NoopRecorder{},
		rbac.NewDefaultAuthorizer(),
	)
	return svc, fakeManager, fakePasswd, fakeSvc
}

func ownerPrincipal() auth.Principal {
	return auth.Principal{ID: 1, Username: "admin", Roles: []string{"owner"}}
}

func viewerPrincipal() auth.Principal {
	return auth.Principal{ID: 2, Username: "viewer", Roles: []string{"viewer"}}
}

// Readiness tests

func TestOcservServiceReadinessRequiresAuth(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newTestOcservService(t, ocserv.ServiceInactive)
	_, err := svc.Readiness(context.Background(), auth.Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestOcservServiceReadinessAllowedForViewer(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newTestOcservService(t, ocserv.ServiceInactive)
	_, err := svc.Readiness(context.Background(), viewerPrincipal())
	if err != nil {
		t.Fatalf("expected viewer to read readiness, got %v", err)
	}
}

// Service control tests

func TestOcservServiceStartDeniedForViewer(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newTestOcservService(t, ocserv.ServiceInactive)
	err := svc.StartService(context.Background(), viewerPrincipal())
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for viewer, got %v", err)
	}
}

func TestOcservServiceStartAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, _, fakeSvc := newTestOcservService(t, ocserv.ServiceInactive)
	if err := svc.StartService(context.Background(), ownerPrincipal()); err != nil {
		t.Fatalf("expected owner to start service, got %v", err)
	}
	if fakeSvc.StartCount() != 1 {
		t.Fatalf("expected 1 start call, got %d", fakeSvc.StartCount())
	}
}

func TestOcservServiceStopAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, _, fakeSvc := newTestOcservService(t, ocserv.ServiceActive)
	if err := svc.StopService(context.Background(), ownerPrincipal()); err != nil {
		t.Fatalf("expected owner to stop service, got %v", err)
	}
	if fakeSvc.StopCount() != 1 {
		t.Fatalf("expected 1 stop call, got %d", fakeSvc.StopCount())
	}
}

func TestOcservServiceRestartAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, _, fakeSvc := newTestOcservService(t, ocserv.ServiceActive)
	if err := svc.RestartService(context.Background(), ownerPrincipal()); err != nil {
		t.Fatalf("expected owner to restart service, got %v", err)
	}
	if fakeSvc.RestartCount() != 1 {
		t.Fatalf("expected 1 restart call, got %d", fakeSvc.RestartCount())
	}
}

// Session tests

func TestOcservServiceListSessionsDeniedForUnauthenticated(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newTestOcservService(t, ocserv.ServiceInactive)
	_, err := svc.ListSessions(context.Background(), auth.Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestOcservServiceListSessionsAllowedForViewer(t *testing.T) {
	t.Parallel()
	svc, fakeManager, _, _ := newTestOcservService(t, ocserv.ServiceInactive)
	fakeManager.Sessions = []ocserv.Session{
		{ID: "s1", Username: "alice"},
	}
	sessions, err := svc.ListSessions(context.Background(), viewerPrincipal())
	if err != nil {
		t.Fatalf("expected viewer to list sessions, got %v", err)
	}
	if len(sessions) != 1 || sessions[0].Username != "alice" {
		t.Fatalf("expected alice session, got %v", sessions)
	}
}

func TestOcservServiceDisconnectDeniedForViewer(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newTestOcservService(t, ocserv.ServiceInactive)
	err := svc.DisconnectSession(context.Background(), viewerPrincipal(), "s1")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for viewer, got %v", err)
	}
}

func TestOcservServiceDisconnectAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newTestOcservService(t, ocserv.ServiceActive)
	if err := svc.DisconnectSession(context.Background(), ownerPrincipal(), "s1"); err != nil {
		t.Fatalf("expected owner to disconnect session, got %v", err)
	}
}

// User management tests

func TestOcservServiceCreateUserDeniedForViewer(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newTestOcservService(t, ocserv.ServiceInactive)
	err := svc.CreateOcservUser(context.Background(), viewerPrincipal(), "alice", "password")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for viewer, got %v", err)
	}
}

func TestOcservServiceCreateUserAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, fakePasswd, _ := newTestOcservService(t, ocserv.ServiceInactive)
	if err := svc.CreateOcservUser(context.Background(), ownerPrincipal(), "alice", "password"); err != nil {
		t.Fatalf("expected owner to create user, got %v", err)
	}
	exists, _ := fakePasswd.UserExists(context.Background(), "alice")
	if !exists {
		t.Fatal("expected alice to exist after creation")
	}
}

func TestOcservServiceDeleteUserAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, fakePasswd, _ := newTestOcservService(t, ocserv.ServiceInactive)
	_ = fakePasswd.CreateUser(context.Background(), "alice", "password")

	if err := svc.DeleteOcservUser(context.Background(), ownerPrincipal(), "alice"); err != nil {
		t.Fatalf("expected owner to delete user, got %v", err)
	}
	exists, _ := fakePasswd.UserExists(context.Background(), "alice")
	if exists {
		t.Fatal("expected alice to be deleted")
	}
}

func TestOcservServiceListUsersAllowedForViewer(t *testing.T) {
	t.Parallel()
	svc, _, fakePasswd, _ := newTestOcservService(t, ocserv.ServiceInactive)
	_ = fakePasswd.CreateUser(context.Background(), "alice", "password")

	users, err := svc.ListOcservUsers(context.Background(), viewerPrincipal())
	if err != nil {
		t.Fatalf("expected viewer to list users, got %v", err)
	}
	if len(users) != 1 || users[0].Username != "alice" {
		t.Fatalf("expected alice, got %v", users)
	}
}

// Config issues tests

func TestOcservServiceConfigIssuesAllowedForViewer(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newTestOcservService(t, ocserv.ServiceInactive)
	// Config path is /nonexistent/ocserv.conf — should return 1 error issue.
	issues, err := svc.ConfigIssues(context.Background(), viewerPrincipal())
	if err != nil {
		t.Fatalf("expected viewer to get config issues, got %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for missing config, got %v", issues)
	}
}

func TestOcservServiceConfigIssuesDeniedForUnauthenticated(t *testing.T) {
	t.Parallel()
	svc, _, _, _ := newTestOcservService(t, ocserv.ServiceInactive)
	_, err := svc.ConfigIssues(context.Background(), auth.Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}
