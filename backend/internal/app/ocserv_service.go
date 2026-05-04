package app

import (
	"context"
	"fmt"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/ocserv"
	"ocserv_panel/backend/internal/rbac"
)

// OcservService orchestrates ocserv readiness, sessions, users, service, and config.
type OcservService struct {
	readiness  *ocserv.ReadinessChecker
	sessions   ocserv.Manager
	users      ocserv.PasswdManager
	svc        ocserv.ServiceManager
	validator  *ocserv.ConfigValidator
	recorder   audit.Recorder
	authorizer rbac.Authorizer
}

// NewOcservService returns an OcservService wired with the given dependencies.
func NewOcservService(
	readiness *ocserv.ReadinessChecker,
	sessions ocserv.Manager,
	users ocserv.PasswdManager,
	svc ocserv.ServiceManager,
	validator *ocserv.ConfigValidator,
	recorder audit.Recorder,
	authorizer rbac.Authorizer,
) *OcservService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	return &OcservService{
		readiness:  readiness,
		sessions:   sessions,
		users:      users,
		svc:        svc,
		validator:  validator,
		recorder:   recorder,
		authorizer: authorizer,
	}
}

// Readiness returns the current ocserv installation and runtime state.
func (s *OcservService) Readiness(ctx context.Context, principal auth.Principal) (*ocserv.OcservReadiness, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservRead, "ocserv.readiness"); err != nil {
		return nil, err
	}
	result, err := s.readiness.Check(ctx)
	if err != nil {
		return nil, fmt.Errorf("check ocserv readiness: %w", err)
	}
	return result, nil
}

// ServiceStatus returns the ocserv service status.
func (s *OcservService) ServiceStatus(ctx context.Context, principal auth.Principal) (ocserv.ServiceStatus, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservServiceRead, "ocserv.service.status"); err != nil {
		return ocserv.ServiceNotFound, err
	}
	status, err := s.svc.Status(ctx)
	if err != nil {
		return ocserv.ServiceNotFound, fmt.Errorf("get service status: %w", err)
	}
	return status, nil
}

// StartService starts the ocserv service.
func (s *OcservService) StartService(ctx context.Context, principal auth.Principal) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservServiceManage, "ocserv.service.start"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.svc.Start(ctx); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.service.start", "ocserv_service", "", audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("start ocserv service: %w", err)
	}
	_ = s.record(ctx, &actorID, "ocserv.service.start", "ocserv_service", "", audit.OutcomeSuccess, "ocserv service started", nil)
	return nil
}

// StopService stops the ocserv service.
func (s *OcservService) StopService(ctx context.Context, principal auth.Principal) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservServiceManage, "ocserv.service.stop"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.svc.Stop(ctx); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.service.stop", "ocserv_service", "", audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("stop ocserv service: %w", err)
	}
	_ = s.record(ctx, &actorID, "ocserv.service.stop", "ocserv_service", "", audit.OutcomeSuccess, "ocserv service stopped", nil)
	return nil
}

// RestartService restarts the ocserv service.
func (s *OcservService) RestartService(ctx context.Context, principal auth.Principal) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservServiceManage, "ocserv.service.restart"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.svc.Restart(ctx); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.service.restart", "ocserv_service", "", audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("restart ocserv service: %w", err)
	}
	_ = s.record(ctx, &actorID, "ocserv.service.restart", "ocserv_service", "", audit.OutcomeSuccess, "ocserv service restarted", nil)
	return nil
}

// ListSessions returns active ocserv sessions.
func (s *OcservService) ListSessions(ctx context.Context, principal auth.Principal) ([]ocserv.Session, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservSessionsRead, "ocserv.sessions.list"); err != nil {
		return nil, err
	}
	sessions, err := s.sessions.ListSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list ocserv sessions: %w", err)
	}
	return sessions, nil
}

// DisconnectSession disconnects an active ocserv session by ID.
func (s *OcservService) DisconnectSession(ctx context.Context, principal auth.Principal, sessionID string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservSessionsDisconnect, "ocserv.sessions.disconnect"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.sessions.DisconnectSession(ctx, sessionID); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.sessions.disconnect", "ocserv_session", sessionID, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("disconnect ocserv session %s: %w", sessionID, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.sessions.disconnect", "ocserv_session", sessionID, audit.OutcomeSuccess, "ocserv session disconnected", nil)
	return nil
}

// ListOcservUsers returns ocserv password-based users.
func (s *OcservService) ListOcservUsers(ctx context.Context, principal auth.Principal) ([]ocserv.PasswdUser, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersRead, "ocserv.users.list"); err != nil {
		return nil, err
	}
	users, err := s.users.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list ocserv users: %w", err)
	}
	return users, nil
}

// CreateOcservUser creates a new ocserv password-based user.
func (s *OcservService) CreateOcservUser(ctx context.Context, principal auth.Principal, username, password string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersCreate, "ocserv.users.create"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.users.CreateUser(ctx, username, password); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.users.create", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("create ocserv user %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.users.create", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user created", nil)
	return nil
}

// DeleteOcservUser deletes an ocserv password-based user.
func (s *OcservService) DeleteOcservUser(ctx context.Context, principal auth.Principal, username string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersDelete, "ocserv.users.delete"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.users.DeleteUser(ctx, username); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.users.delete", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("delete ocserv user %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.users.delete", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user deleted", nil)
	return nil
}

// UpdateOcservUserPassword updates an ocserv user's password.
func (s *OcservService) UpdateOcservUserPassword(ctx context.Context, principal auth.Principal, username, newPassword string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersUpdate, "ocserv.users.update_password"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.users.UpdatePassword(ctx, username, newPassword); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.users.update_password", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("update ocserv user password %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.users.update_password", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user password updated", nil)
	return nil
}

// ConfigIssues returns ocserv configuration validation issues.
func (s *OcservService) ConfigIssues(ctx context.Context, principal auth.Principal) ([]ocserv.ConfigIssue, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservConfigRead, "ocserv.config.issues"); err != nil {
		return nil, err
	}
	issues, err := s.validator.ValidateConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("validate ocserv config: %w", err)
	}
	return issues, nil
}

func (s *OcservService) requirePerm(ctx context.Context, principal auth.Principal, permission rbac.Permission, action string) error {
	if !principal.Authenticated() {
		_ = s.record(ctx, nil, action, "ocserv", "", audit.OutcomeDenied, "ocserv authentication required", nil)
		return ErrUnauthenticated
	}
	if !s.authorizer.Allowed(principalRoles(principal), permission) {
		actorID := principal.ID
		_ = s.record(ctx, &actorID, action, "ocserv", "", audit.OutcomeDenied, "ocserv permission denied", nil)
		return ErrPermissionDenied
	}
	return nil
}

func (s *OcservService) record(ctx context.Context, actorID *int64, action, resourceType, resourceID string, outcome audit.Outcome, message string, metadata map[string]string) error {
	return s.recorder.Record(ctx, audit.Event{
		ActorID:      actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Outcome:      outcome,
		Message:      message,
		Metadata:     metadata,
	})
}
