package app

import (
	"context"
	"errors"
	"testing"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/vpn/exec"
	"ocserv_panel/backend/internal/vpn/sessions"
	"ocserv_panel/backend/internal/vpn/health"
	"ocserv_panel/backend/internal/vpn/users"
	"ocserv_panel/backend/internal/vpn/config"
	"ocserv_panel/backend/internal/vpn/service"
	"ocserv_panel/backend/internal/rbac"
)

// newTestOcservService creates an OcservService with all-fake dependencies.
func newTestOcservService(t *testing.T, svcStatus service.ServiceStatus) (
	*OcservService,
	*sessions.FakeManager,
	*users.FakePasswdManager,
	*service.FakeServiceManager,
	*service.FakeServiceManager,
) {
	t.Helper()

	runner := exec.NewFakeCommandRunner()
	fakeSvc := service.NewFakeServiceManager(svcStatus)
	checker := health.NewReadinessChecker(func() health.ReadinessConfig { return health.ReadinessConfig{} }, runner, fakeSvc)

	fakeManager := sessions.NewFakeManager()
	fakeManager.DisconnectErr = nil // allow disconnect by default in service tests
	fakeManager.TerminateErr = nil  // allow terminate by default in service tests

	fakePasswd := users.NewFakePasswdManager()
	validator := config.NewConfigValidator("/nonexistent/ocserv.conf")
	fakePanelSvc := service.NewFakeServiceManager(service.ServiceActive)

	svc := NewOcservService(
		checker,
		fakeManager,
		fakePasswd,
		fakeSvc,
		fakePanelSvc,
		validator,
		audit.NoopRecorder{},
		rbac.NewDefaultAuthorizer(),
	)
	return svc, fakeManager, fakePasswd, fakeSvc, fakePanelSvc
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
	svc, _, _, _, _ := newTestOcservService(t, service.ServiceInactive)
	_, err := svc.Readiness(context.Background(), auth.Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestOcservServiceReadinessAllowedForViewer(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestOcservService(t, service.ServiceInactive)
	_, err := svc.Readiness(context.Background(), viewerPrincipal())
	if err != nil {
		t.Fatalf("expected viewer to read readiness, got %v", err)
	}
}

// Service control tests

func TestOcservServiceStartDeniedForViewer(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestOcservService(t, service.ServiceInactive)
	err := svc.StartService(context.Background(), viewerPrincipal())
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for viewer, got %v", err)
	}
}

func TestOcservServiceStartAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, _, fakeSvc, _ := newTestOcservService(t, service.ServiceInactive)
	if err := svc.StartService(context.Background(), ownerPrincipal()); err != nil {
		t.Fatalf("expected owner to start service, got %v", err)
	}
	if fakeSvc.StartCount() != 1 {
		t.Fatalf("expected 1 start call, got %d", fakeSvc.StartCount())
	}
}

func TestOcservServiceStopAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, _, fakeSvc, _ := newTestOcservService(t, service.ServiceActive)
	if err := svc.StopService(context.Background(), ownerPrincipal()); err != nil {
		t.Fatalf("expected owner to stop service, got %v", err)
	}
	if fakeSvc.StopCount() != 1 {
		t.Fatalf("expected 1 stop call, got %d", fakeSvc.StopCount())
	}
}

func TestOcservServiceRestartAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, _, fakeSvc, _ := newTestOcservService(t, service.ServiceActive)
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
	svc, _, _, _, _ := newTestOcservService(t, service.ServiceInactive)
	_, err := svc.ListSessions(context.Background(), auth.Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestOcservServiceListSessionsAllowedForViewer(t *testing.T) {
	t.Parallel()
	svc, fakeManager, _, _, _ := newTestOcservService(t, service.ServiceInactive)
	fakeManager.Sessions = []sessions.Session{
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
	svc, _, _, _, _ := newTestOcservService(t, service.ServiceInactive)
	err := svc.DisconnectSession(context.Background(), viewerPrincipal(), "s1")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for viewer, got %v", err)
	}
}

func TestOcservServiceDisconnectAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestOcservService(t, service.ServiceActive)
	if err := svc.DisconnectSession(context.Background(), ownerPrincipal(), "s1"); err != nil {
		t.Fatalf("expected owner to disconnect session, got %v", err)
	}
}

// User management tests

func TestOcservServiceCreateUserDeniedForViewer(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestOcservService(t, service.ServiceInactive)
	err := svc.CreateOcservUser(context.Background(), viewerPrincipal(), "alice", "password", "")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for viewer, got %v", err)
	}
}

func TestOcservServiceCreateUserAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, fakePasswd, _, _ := newTestOcservService(t, service.ServiceInactive)
	if err := svc.CreateOcservUser(context.Background(), ownerPrincipal(), "alice", "password", ""); err != nil {
		t.Fatalf("expected owner to create user, got %v", err)
	}
	exists, _ := fakePasswd.UserExists(context.Background(), "alice")
	if !exists {
		t.Fatal("expected alice to exist after creation")
	}
}

func TestOcservServiceDeleteUserAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, fakePasswd, _, _ := newTestOcservService(t, service.ServiceInactive)
	_ = fakePasswd.CreateUser(context.Background(), "alice", "password", "")

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
	svc, _, fakePasswd, _, _ := newTestOcservService(t, service.ServiceInactive)
	_ = fakePasswd.CreateUser(context.Background(), "alice", "password", "")

	users, err := svc.ListOcservUsers(context.Background(), viewerPrincipal())
	if err != nil {
		t.Fatalf("expected viewer to list users, got %v", err)
	}
	if len(users) != 1 || users[0].Username != "alice" {
		t.Fatalf("expected alice, got %v", users)
	}
}

// capturingRecorder records audit events for test assertions.
type capturingRecorder struct {
	events []audit.Event
}

func (r *capturingRecorder) Record(_ context.Context, event audit.Event) error {
	r.events = append(r.events, event)
	return nil
}

// newTestOcservServiceWithRecorder is like newTestOcservService but uses a
// capturing recorder so tests can inspect audit events.
func newTestOcservServiceWithRecorder(t *testing.T, svcStatus service.ServiceStatus) (
	*OcservService,
	*sessions.FakeManager,
	*capturingRecorder,
) {
	t.Helper()

	runner := exec.NewFakeCommandRunner()
	fakeSvc := service.NewFakeServiceManager(svcStatus)
	checker := health.NewReadinessChecker(func() health.ReadinessConfig { return health.ReadinessConfig{} }, runner, fakeSvc)

	fakeManager := sessions.NewFakeManager()
	fakePasswd := users.NewFakePasswdManager()
	validator := config.NewConfigValidator("/nonexistent/ocserv.conf")
	recorder := &capturingRecorder{}
	fakePanelSvc := service.NewFakeServiceManager(service.ServiceActive)

	svc := NewOcservService(
		checker,
		fakeManager,
		fakePasswd,
		fakeSvc,
		fakePanelSvc,
		validator,
		recorder,
		rbac.NewDefaultAuthorizer(),
	)
	return svc, fakeManager, recorder
}

// TerminateSession tests

func TestOcservServiceTerminateSessionDeniedForViewer(t *testing.T) {
	t.Parallel()
	svc, _, rec := newTestOcservServiceWithRecorder(t, service.ServiceActive)
	err := svc.TerminateSession(context.Background(), viewerPrincipal(), "s1")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied for viewer, got %v", err)
	}
	if len(rec.events) != 1 {
		t.Fatalf("expected 1 audit event from requirePerm deny, got %d", len(rec.events))
	}
	if rec.events[0].Outcome != audit.OutcomeDenied {
		t.Fatalf("expected denied outcome, got %q", rec.events[0].Outcome)
	}
}

func TestOcservServiceTerminateSessionSuccess(t *testing.T) {
	t.Parallel()
	svc, fakeManager, rec := newTestOcservServiceWithRecorder(t, service.ServiceActive)
	fakeManager.TerminateErr = nil

	if err := svc.TerminateSession(context.Background(), ownerPrincipal(), "s42"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(rec.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(rec.events))
	}
	ev := rec.events[0]
	if ev.Action != "ocserv.sessions.terminate" {
		t.Errorf("expected action %q, got %q", "ocserv.sessions.terminate", ev.Action)
	}
	if ev.ResourceType != "ocserv_session" {
		t.Errorf("expected resource type %q, got %q", "ocserv_session", ev.ResourceType)
	}
	if ev.ResourceID != "s42" {
		t.Errorf("expected resource id %q, got %q", "s42", ev.ResourceID)
	}
	if ev.Outcome != audit.OutcomeSuccess {
		t.Errorf("expected success outcome, got %q", ev.Outcome)
	}
}

func TestOcservServiceTerminateSessionManagerFailure(t *testing.T) {
	t.Parallel()
	svc, fakeManager, rec := newTestOcservServiceWithRecorder(t, service.ServiceActive)
	fakeManager.TerminateErr = errors.New("socket unavailable")

	err := svc.TerminateSession(context.Background(), ownerPrincipal(), "s9")
	if err == nil {
		t.Fatal("expected error on manager failure, got nil")
	}
	if len(rec.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(rec.events))
	}
	if rec.events[0].Outcome != audit.OutcomeFailure {
		t.Errorf("expected failure outcome, got %q", rec.events[0].Outcome)
	}
}

// GetSessionDetail tests

func TestOcservServiceGetSessionDetailDeniedForUnauthenticated(t *testing.T) {
	t.Parallel()
	// viewer does not have sessions:terminate, but does have sessions:read —
	// GetSessionDetail uses sessions:read, so viewer IS allowed. Test unauthenticated instead.
	svc, _, _ := newTestOcservServiceWithRecorder(t, service.ServiceActive)
	_, err := svc.GetSessionDetail(context.Background(), auth.Principal{}, "s1")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated for anonymous principal, got %v", err)
	}
}

func TestOcservServiceGetSessionDetailSuccess(t *testing.T) {
	t.Parallel()
	svc, fakeManager, _ := newTestOcservServiceWithRecorder(t, service.ServiceActive)
	want := &sessions.SessionDetail{ID: "s5", Username: "bob"}
	fakeManager.SessionDetails["s5"] = want

	got, err := svc.GetSessionDetail(context.Background(), viewerPrincipal(), "s5")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.ID != want.ID || got.Username != want.Username {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

func TestOcservServiceGetSessionDetailManagerError(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestOcservServiceWithRecorder(t, service.ServiceActive)
	// FakeManager returns ErrNotConfigured for unknown ids by default.
	_, err := svc.GetSessionDetail(context.Background(), ownerPrincipal(), "unknown")
	if err == nil {
		t.Fatal("expected error for unknown session id, got nil")
	}
}

// Config issues tests

func TestOcservServiceConfigIssuesAllowedForViewer(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestOcservService(t, service.ServiceInactive)
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
	svc, _, _, _, _ := newTestOcservService(t, service.ServiceInactive)
	_, err := svc.ConfigIssues(context.Background(), auth.Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestRestartPanelServiceDeniedForUnauthenticated(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestOcservService(t, service.ServiceActive)
	err := svc.RestartPanelService(context.Background(), auth.Principal{})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestRestartPanelServiceDeniedForViewer(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := newTestOcservService(t, service.ServiceActive)
	err := svc.RestartPanelService(context.Background(), viewerPrincipal())
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestRestartPanelServiceAllowedForOwner(t *testing.T) {
	t.Parallel()
	svc, _, _, _, fakePanelSvc := newTestOcservService(t, service.ServiceActive)
	if err := svc.RestartPanelService(context.Background(), ownerPrincipal()); err != nil {
		t.Fatalf("expected owner to restart panel, got %v", err)
	}
	if fakePanelSvc.RestartCount() != 1 {
		t.Fatalf("expected 1 restart call on panel svc, got %d", fakePanelSvc.RestartCount())
	}
}
