package app

import (
	"context"
	"fmt"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/ocserv"
	ocservconfig "ocserv_panel/backend/internal/ocserv/config"
	"ocserv_panel/backend/internal/ocserv/service"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/validation"
)

// OcservService orchestrates ocserv readiness, sessions, users, service, and config.
type OcservService struct {
	readiness  *ocserv.ReadinessChecker
	sessions   ocserv.Manager
	users      ocserv.PasswdManager
	svc        service.ServiceManager
	validator  *ocservconfig.ConfigValidator
	recorder   audit.Recorder
	authorizer rbac.Authorizer
}

// NewOcservService returns an OcservService wired with the given dependencies.
func NewOcservService(
	readiness *ocserv.ReadinessChecker,
	sessions ocserv.Manager,
	users ocserv.PasswdManager,
	svc service.ServiceManager,
	validator *ocservconfig.ConfigValidator,
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
func (s *OcservService) ServiceStatus(ctx context.Context, principal auth.Principal) (service.ServiceStatus, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservServiceRead, "ocserv.service.status"); err != nil {
		return service.ServiceNotFound, err
	}
	status, err := s.svc.Status(ctx)
	if err != nil {
		return service.ServiceNotFound, fmt.Errorf("get service status: %w", err)
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

// TerminateSession terminates an active ocserv session by ID.
func (s *OcservService) TerminateSession(ctx context.Context, principal auth.Principal, sessionID string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservSessionsTerminate, "ocserv.sessions.terminate"); err != nil {
		return err
	}
	actorID := principal.ID
	if err := s.sessions.TerminateSession(ctx, sessionID); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.sessions.terminate", "ocserv_session", sessionID, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("terminate ocserv session %s: %w", sessionID, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.sessions.terminate", "ocserv_session", sessionID, audit.OutcomeSuccess, "ocserv session terminated", nil)
	return nil
}

// GetSessionDetail returns detailed information for a single ocserv session.
func (s *OcservService) GetSessionDetail(ctx context.Context, principal auth.Principal, sessionID string) (*ocserv.SessionDetail, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservSessionsRead, "ocserv.sessions.detail"); err != nil {
		return nil, err
	}
	detail, err := s.sessions.GetSessionDetails(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get ocserv session detail %s: %w", sessionID, err)
	}
	return detail, nil
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
// When group is non-empty, the user is assigned to that group.
func (s *OcservService) CreateOcservUser(ctx context.Context, principal auth.Principal, username, password, group string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersCreate, "ocserv.users.create"); err != nil {
		return err
	}
	if err := validation.Username(username); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	if err := validation.Password(password); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	if err := validation.Group(group); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	actorID := principal.ID
	metadata := map[string]string{}
	if group != "" {
		metadata["group"] = group
	}
	if err := s.users.CreateUser(ctx, username, password, group); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.users.create", "ocserv_user", username, audit.OutcomeFailure, err.Error(), metadata)
		return fmt.Errorf("create ocserv user %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.users.create", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user created", metadata)
	return nil
}

// DeleteOcservUser deletes an ocserv password-based user.
func (s *OcservService) DeleteOcservUser(ctx context.Context, principal auth.Principal, username string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersDelete, "ocserv.users.delete"); err != nil {
		return err
	}
	if err := validation.Username(username); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
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
	if err := validation.Username(username); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	if err := validation.Password(newPassword); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	actorID := principal.ID
	if err := s.users.UpdatePassword(ctx, username, newPassword); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.users.update_password", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("update ocserv user password %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.users.update_password", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user password updated", nil)
	return nil
}

// UpdateOcservUserGroup updates an ocserv user's group assignment.
// An empty group removes the user from their current group.
func (s *OcservService) UpdateOcservUserGroup(ctx context.Context, principal auth.Principal, username, group string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersUpdate, "ocserv.users.update_group"); err != nil {
		return err
	}
	if err := validation.Username(username); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	if err := validation.Group(group); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	actorID := principal.ID
	metadata := map[string]string{"group": group}
	if err := s.users.UpdateGroup(ctx, username, group); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.users.update_group", "ocserv_user", username, audit.OutcomeFailure, err.Error(), metadata)
		return fmt.Errorf("update ocserv user group %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.users.update_group", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user group updated", metadata)
	return nil
}

// LockOcservUser locks (disables) a VPN user.
func (s *OcservService) LockOcservUser(ctx context.Context, principal auth.Principal, username string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersUpdate, "ocserv.users.lock"); err != nil {
		return err
	}
	if err := validation.Username(username); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	actorID := principal.ID
	if err := s.users.LockUser(ctx, username); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.users.lock", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("lock ocserv user %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.users.lock", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user locked", nil)
	return nil
}

// UnlockOcservUser unlocks (re-enables) a VPN user.
func (s *OcservService) UnlockOcservUser(ctx context.Context, principal auth.Principal, username string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersUpdate, "ocserv.users.unlock"); err != nil {
		return err
	}
	if err := validation.Username(username); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	actorID := principal.ID
	if err := s.users.UnlockUser(ctx, username); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.users.unlock", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("unlock ocserv user %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.users.unlock", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user unlocked", nil)
	return nil
}

// BulkDeleteOcservUsers deletes multiple ocserv password-based users.
// Each item is processed independently; failures are collected and returned in BulkResult.
func (s *OcservService) BulkDeleteOcservUsers(ctx context.Context, principal auth.Principal, usernames []string) BulkResult {
	result := BulkResult{Total: len(usernames), Errors: []BulkError{}}

	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersDelete, "ocserv.users.bulk_delete"); err != nil {
		result.Failed = result.Total
		for _, u := range usernames {
			result.Errors = append(result.Errors, BulkError{ID: u, Error: "permission denied"})
		}
		return result
	}

	actorID := principal.ID
	for _, username := range usernames {
		if err := validation.Username(username); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BulkError{ID: username, Error: "invalid username"})
			continue
		}

		if err := s.users.DeleteUser(ctx, username); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BulkError{ID: username, Error: err.Error()})
			_ = s.record(ctx, &actorID, "ocserv.users.bulk_delete", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
			continue
		}

		result.Succeeded++
		_ = s.record(ctx, &actorID, "ocserv.users.bulk_delete", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user deleted", nil)
	}

	return result
}

// BulkLockOcservUsers locks (disables) multiple ocserv password-based users.
// Each item is processed independently; failures are collected and returned in BulkResult.
func (s *OcservService) BulkLockOcservUsers(ctx context.Context, principal auth.Principal, usernames []string) BulkResult {
	result := BulkResult{Total: len(usernames), Errors: []BulkError{}}

	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersUpdate, "ocserv.users.bulk_lock"); err != nil {
		result.Failed = result.Total
		for _, u := range usernames {
			result.Errors = append(result.Errors, BulkError{ID: u, Error: "permission denied"})
		}
		return result
	}

	actorID := principal.ID
	for _, username := range usernames {
		if err := validation.Username(username); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BulkError{ID: username, Error: "invalid username"})
			continue
		}

		if err := s.users.LockUser(ctx, username); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BulkError{ID: username, Error: err.Error()})
			_ = s.record(ctx, &actorID, "ocserv.users.bulk_lock", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
			continue
		}

		result.Succeeded++
		_ = s.record(ctx, &actorID, "ocserv.users.bulk_lock", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user locked", nil)
	}

	return result
}

// BulkUnlockOcservUsers unlocks (re-enables) multiple ocserv password-based users.
// Each item is processed independently; failures are collected and returned in BulkResult.
func (s *OcservService) BulkUnlockOcservUsers(ctx context.Context, principal auth.Principal, usernames []string) BulkResult {
	result := BulkResult{Total: len(usernames), Errors: []BulkError{}}

	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservUsersUpdate, "ocserv.users.bulk_unlock"); err != nil {
		result.Failed = result.Total
		for _, u := range usernames {
			result.Errors = append(result.Errors, BulkError{ID: u, Error: "permission denied"})
		}
		return result
	}

	actorID := principal.ID
	for _, username := range usernames {
		if err := validation.Username(username); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BulkError{ID: username, Error: "invalid username"})
			continue
		}

		if err := s.users.UnlockUser(ctx, username); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, BulkError{ID: username, Error: err.Error()})
			_ = s.record(ctx, &actorID, "ocserv.users.bulk_unlock", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
			continue
		}

		result.Succeeded++
		_ = s.record(ctx, &actorID, "ocserv.users.bulk_unlock", "ocserv_user", username, audit.OutcomeSuccess, "ocserv user unlocked", nil)
	}

	return result
}

// GetServerStatus returns the ocserv server status from `occtl show status`.
func (s *OcservService) GetServerStatus(ctx context.Context, principal auth.Principal) (*ocserv.ServerStatus, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservRead, "ocserv.server.status"); err != nil {
		return nil, err
	}
	status, err := s.sessions.GetServerStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("get server status: %w", err)
	}
	return status, nil
}

// ListConnectedUsers returns connected users from `occtl show users`.
func (s *OcservService) ListConnectedUsers(ctx context.Context, principal auth.Principal) ([]ocserv.ConnectedUser, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservSessionsRead, "ocserv.users.connected"); err != nil {
		return nil, err
	}
	users, err := s.sessions.ListConnectedUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list connected users: %w", err)
	}
	return users, nil
}

// GetUserConnections returns detailed connections for a single user.
func (s *OcservService) GetUserConnections(ctx context.Context, principal auth.Principal, username string) ([]ocserv.UserConnection, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservSessionsRead, "ocserv.users.connections"); err != nil {
		return nil, err
	}
	if err := validation.Username(username); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	conns, err := s.sessions.GetUserDetails(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("get user connections %s: %w", username, err)
	}
	return conns, nil
}

// GetConnectionByID returns connection details for a specific connection ID.
func (s *OcservService) GetConnectionByID(ctx context.Context, principal auth.Principal, id int) (*ocserv.UserConnection, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservSessionsRead, "ocserv.connections.detail"); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, fmt.Errorf("%w: connection ID must be positive", ErrInvalidInput)
	}
	conn, err := s.sessions.GetConnectionByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get connection by id %d: %w", id, err)
	}
	return conn, nil
}

// ListEvents returns connection events from `occtl show events`.
func (s *OcservService) ListEvents(ctx context.Context, principal auth.Principal) ([]ocserv.ConnectionEvent, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservSessionsRead, "ocserv.events.list"); err != nil {
		return nil, err
	}
	events, err := s.sessions.ListEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	return events, nil
}

// ListRoutes returns user-provided routes from `occtl show iroutes`.
func (s *OcservService) ListRoutes(ctx context.Context, principal auth.Principal) ([]ocserv.UserRoute, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservSessionsRead, "ocserv.routes.list"); err != nil {
		return nil, err
	}
	routes, err := s.sessions.ListRoutes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	return routes, nil
}

// ListBannedIPs returns banned IPs from `occtl show ip bans`.
func (s *OcservService) ListBannedIPs(ctx context.Context, principal auth.Principal) ([]ocserv.BannedIP, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservSessionsRead, "ocserv.bans.list"); err != nil {
		return nil, err
	}
	bans, err := s.sessions.ListBannedIPs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list banned ips: %w", err)
	}
	return bans, nil
}

// ListBanPoints returns ban point entries from `occtl show ip ban points`.
func (s *OcservService) ListBanPoints(ctx context.Context, principal auth.Principal) ([]ocserv.BanPointEntry, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservSessionsRead, "ocserv.bans.points"); err != nil {
		return nil, err
	}
	points, err := s.sessions.ListBanPoints(ctx)
	if err != nil {
		return nil, fmt.Errorf("list ban points: %w", err)
	}
	return points, nil
}

// DisconnectUser disconnects all sessions for a given username.
func (s *OcservService) DisconnectUser(ctx context.Context, principal auth.Principal, username string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservSessionsDisconnect, "ocserv.user.disconnect"); err != nil {
		return err
	}
	if err := validation.Username(username); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	actorID := principal.ID
	if err := s.sessions.DisconnectUser(ctx, username); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.user.disconnect", "ocserv_user", username, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("disconnect user %s: %w", username, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.user.disconnect", "ocserv_user", username, audit.OutcomeSuccess, "user disconnected", nil)
	return nil
}

// UnbanIP removes an IP from the ban list.
func (s *OcservService) UnbanIP(ctx context.Context, principal auth.Principal, ip string) error {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservBanManage, "ocserv.ban.unban"); err != nil {
		return err
	}
	if err := validation.IPAddress(ip); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	actorID := principal.ID
	if err := s.sessions.UnbanIP(ctx, ip); err != nil {
		_ = s.record(ctx, &actorID, "ocserv.ban.unban", "ocserv_ban", ip, audit.OutcomeFailure, err.Error(), nil)
		return fmt.Errorf("unban ip %s: %w", ip, err)
	}
	_ = s.record(ctx, &actorID, "ocserv.ban.unban", "ocserv_ban", ip, audit.OutcomeSuccess, "IP unbanned", nil)
	return nil
}

// UserSessionCount represents one user's active session count for the donut chart.
type UserSessionCount struct {
	Username     string `json:"username"`
	SessionCount int    `json:"sessionCount"`
}

// ActiveUserSessionsResult is the response for the active sessions donut endpoint.
type ActiveUserSessionsResult struct {
	Users []UserSessionCount `json:"users"`
	Total int                `json:"total"`
}

// ActiveUserSessions returns per-user session counts for the dashboard donut chart.
func (s *OcservService) ActiveUserSessions(ctx context.Context, principal auth.Principal) (ActiveUserSessionsResult, error) {
	if err := s.requirePerm(ctx, principal, rbac.PermissionOcservRead, "ocserv.active_user_sessions"); err != nil {
		return ActiveUserSessionsResult{}, err
	}

	users, err := s.sessions.ListConnectedUsers(ctx)
	if err != nil {
		return ActiveUserSessionsResult{}, fmt.Errorf("list connected users: %w", err)
	}

	counts := make(map[string]int)
	for _, u := range users {
		counts[u.Username]++
	}

	result := make([]UserSessionCount, 0, len(counts))
	total := 0
	for username, count := range counts {
		result = append(result, UserSessionCount{Username: username, SessionCount: count})
		total += count
	}

	return ActiveUserSessionsResult{Users: result, Total: total}, nil
}

// ConfigIssues returns ocserv configuration validation issues.
func (s *OcservService) ConfigIssues(ctx context.Context, principal auth.Principal) ([]ocservconfig.ConfigIssue, error) {
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
