package app

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

// ErrNotFound indicates that the requested resource does not exist.
var ErrNotFound = errors.New("not found")

// AdminUserStore defines persistence operations for admin user management.
type AdminUserStore interface {
	ListAdmins(ctx context.Context) ([]store.Admin, error)
	FindAdminByID(ctx context.Context, id int64) (store.Admin, bool, error)
	FindAdminByUsername(ctx context.Context, username string) (store.Admin, bool, error)
	CreateAdmin(ctx context.Context, username, passwordHash, displayName, email string, role rbac.Role) (store.Admin, error)
	UpdateAdmin(ctx context.Context, id int64, displayName, email string, role *rbac.Role) error
	DeleteAdmin(ctx context.Context, id int64) error
	DisableAdmin(ctx context.Context, id int64) error
	EnableAdmin(ctx context.Context, id int64) error
	UpdateAdminPassword(ctx context.Context, id int64, passwordHash string) error
	CountOwners(ctx context.Context) (int64, error)
}

// AdminUserResponse is the API-safe representation of an admin user (no password hash).
type AdminUserResponse struct {
	ID          int64    `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	Disabled    bool     `json:"disabled"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// AdminUserService manages admin user CRUD with RBAC enforcement and audit logging.
type AdminUserService struct {
	store      AdminUserStore
	recorder   audit.Recorder
	authorizer rbac.Authorizer
	hashFn     func(string) (string, error)
}

// NewAdminUserService constructs an AdminUserService with the given dependencies.
func NewAdminUserService(
	s AdminUserStore,
	recorder audit.Recorder,
	authorizer rbac.Authorizer,
	hashFn func(string) (string, error),
) *AdminUserService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	if hashFn == nil {
		hashFn = auth.HashPassword
	}
	return &AdminUserService{
		store:      s,
		recorder:   recorder,
		authorizer: authorizer,
		hashFn:     hashFn,
	}
}

var (
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	emailRegex    = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// ListAdminUsers returns all admin users in API-safe form.
func (svc *AdminUserService) ListAdminUsers(ctx context.Context, principal auth.Principal) ([]AdminUserResponse, error) {
	if !svc.authorizer.Allowed(principalRoles(principal), rbac.PermissionAdminUsersRead) {
		actorID := principal.ID
		_ = svc.record(ctx, &actorID, "admin.users.list", "admin_user", "", audit.OutcomeDenied, "permission denied")
		return nil, ErrPermissionDenied
	}

	admins, err := svc.store.ListAdmins(ctx)
	if err != nil {
		return nil, fmt.Errorf("list admin users: %w", err)
	}

	responses := make([]AdminUserResponse, 0, len(admins))
	for _, admin := range admins {
		responses = append(responses, adminToResponse(admin))
	}
	return responses, nil
}

// CreateAdminUser creates a new admin user after input validation and authorization.
func (svc *AdminUserService) CreateAdminUser(
	ctx context.Context,
	principal auth.Principal,
	username, password, displayName, email string,
	role string,
) (AdminUserResponse, error) {
	actorID := principal.ID

	if !svc.authorizer.Allowed(principalRoles(principal), rbac.PermissionAdminUsersCreate) {
		_ = svc.record(ctx, &actorID, "admin.users.create", "admin_user", username, audit.OutcomeDenied, "permission denied")
		return AdminUserResponse{}, ErrPermissionDenied
	}

	username = strings.TrimSpace(username)
	displayName = strings.TrimSpace(displayName)
	email = strings.TrimSpace(email)

	if err := validateUsername(username); err != nil {
		_ = svc.record(ctx, &actorID, "admin.users.create", "admin_user", username, audit.OutcomeFailure, err.Error())
		return AdminUserResponse{}, err
	}
	if err := validatePassword(password); err != nil {
		_ = svc.record(ctx, &actorID, "admin.users.create", "admin_user", username, audit.OutcomeFailure, err.Error())
		return AdminUserResponse{}, err
	}
	if err := validateEmail(email); err != nil {
		_ = svc.record(ctx, &actorID, "admin.users.create", "admin_user", username, audit.OutcomeFailure, err.Error())
		return AdminUserResponse{}, err
	}
	rbacRole, err := validateRole(role)
	if err != nil {
		_ = svc.record(ctx, &actorID, "admin.users.create", "admin_user", username, audit.OutcomeFailure, err.Error())
		return AdminUserResponse{}, err
	}

	passwordHash, err := svc.hashFn(password)
	if err != nil {
		_ = svc.record(ctx, &actorID, "admin.users.create", "admin_user", username, audit.OutcomeFailure, "password hashing failed")
		return AdminUserResponse{}, fmt.Errorf("hash password: %w", err)
	}

	admin, err := svc.store.CreateAdmin(ctx, username, passwordHash, displayName, email, rbacRole)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateUsername) {
			_ = svc.record(ctx, &actorID, "admin.users.create", "admin_user", username, audit.OutcomeFailure, "duplicate username")
			return AdminUserResponse{}, fmt.Errorf("%w: username %q already exists", ErrInvalidInput, username)
		}
		_ = svc.record(ctx, &actorID, "admin.users.create", "admin_user", username, audit.OutcomeFailure, "store error")
		return AdminUserResponse{}, fmt.Errorf("create admin user: %w", err)
	}

	_ = svc.record(ctx, &actorID, "admin.users.create", "admin_user", admin.Username, audit.OutcomeSuccess, "admin user created")
	return adminToResponse(admin), nil
}

// UpdateAdminUser updates display name, email, and optionally role for an existing admin user.
func (svc *AdminUserService) UpdateAdminUser(
	ctx context.Context,
	principal auth.Principal,
	targetID int64,
	displayName, email string,
	role *string,
) error {
	actorID := principal.ID
	resourceID := fmt.Sprint(targetID)

	if !svc.authorizer.Allowed(principalRoles(principal), rbac.PermissionAdminUsersUpdate) {
		_ = svc.record(ctx, &actorID, "admin.users.update", "admin_user", resourceID, audit.OutcomeDenied, "permission denied")
		return ErrPermissionDenied
	}

	email = strings.TrimSpace(email)
	displayName = strings.TrimSpace(displayName)

	if err := validateEmail(email); err != nil {
		_ = svc.record(ctx, &actorID, "admin.users.update", "admin_user", resourceID, audit.OutcomeFailure, err.Error())
		return err
	}

	target, found, err := svc.store.FindAdminByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("find admin for update: %w", err)
	}
	if !found {
		_ = svc.record(ctx, &actorID, "admin.users.update", "admin_user", resourceID, audit.OutcomeFailure, "admin not found")
		return ErrNotFound
	}

	var rbacRole *rbac.Role
	if role != nil {
		r, err := validateRole(*role)
		if err != nil {
			_ = svc.record(ctx, &actorID, "admin.users.update", "admin_user", resourceID, audit.OutcomeFailure, err.Error())
			return err
		}
		rbacRole = &r

		// Guard: cannot demote the last owner
		if isOwner(target) && r != rbac.RoleOwner {
			ownerCount, err := svc.store.CountOwners(ctx)
			if err != nil {
				return fmt.Errorf("count owners for demotion guard: %w", err)
			}
			if ownerCount <= 1 {
				_ = svc.record(ctx, &actorID, "admin.users.update", "admin_user", resourceID, audit.OutcomeDenied, "cannot demote last owner")
				return fmt.Errorf("%w: cannot demote the last owner", ErrPermissionDenied)
			}
		}
	}

	if err := svc.store.UpdateAdmin(ctx, targetID, displayName, email, rbacRole); err != nil {
		_ = svc.record(ctx, &actorID, "admin.users.update", "admin_user", resourceID, audit.OutcomeFailure, "store error")
		return fmt.Errorf("update admin user: %w", err)
	}

	_ = svc.record(ctx, &actorID, "admin.users.update", "admin_user", resourceID, audit.OutcomeSuccess, "admin user updated")
	return nil
}

// DeleteAdminUser removes an admin user after authorization and safety checks.
func (svc *AdminUserService) DeleteAdminUser(ctx context.Context, principal auth.Principal, targetID int64) error {
	actorID := principal.ID
	resourceID := fmt.Sprint(targetID)

	if !svc.authorizer.Allowed(principalRoles(principal), rbac.PermissionAdminUsersDelete) {
		_ = svc.record(ctx, &actorID, "admin.users.delete", "admin_user", resourceID, audit.OutcomeDenied, "permission denied")
		return ErrPermissionDenied
	}

	// Guard: cannot delete self
	if principal.ID == targetID {
		_ = svc.record(ctx, &actorID, "admin.users.delete", "admin_user", resourceID, audit.OutcomeDenied, "cannot delete self")
		return fmt.Errorf("%w: cannot delete your own account", ErrPermissionDenied)
	}

	target, found, err := svc.store.FindAdminByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("find admin for delete: %w", err)
	}
	if !found {
		_ = svc.record(ctx, &actorID, "admin.users.delete", "admin_user", resourceID, audit.OutcomeFailure, "admin not found")
		return ErrNotFound
	}

	// Guard: cannot delete last owner
	if isOwner(target) {
		ownerCount, err := svc.store.CountOwners(ctx)
		if err != nil {
			return fmt.Errorf("count owners for delete guard: %w", err)
		}
		if ownerCount <= 1 {
			_ = svc.record(ctx, &actorID, "admin.users.delete", "admin_user", resourceID, audit.OutcomeDenied, "cannot delete last owner")
			return fmt.Errorf("%w: cannot delete the last owner", ErrPermissionDenied)
		}
	}

	if err := svc.store.DeleteAdmin(ctx, targetID); err != nil {
		_ = svc.record(ctx, &actorID, "admin.users.delete", "admin_user", resourceID, audit.OutcomeFailure, "store error")
		return fmt.Errorf("delete admin user: %w", err)
	}

	_ = svc.record(ctx, &actorID, "admin.users.delete", "admin_user", resourceID, audit.OutcomeSuccess, "admin user deleted")
	return nil
}

// DisableAdminUser disables an admin user account.
func (svc *AdminUserService) DisableAdminUser(ctx context.Context, principal auth.Principal, targetID int64) error {
	actorID := principal.ID
	resourceID := fmt.Sprint(targetID)

	if !svc.authorizer.Allowed(principalRoles(principal), rbac.PermissionAdminUsersUpdate) {
		_ = svc.record(ctx, &actorID, "admin.users.disable", "admin_user", resourceID, audit.OutcomeDenied, "permission denied")
		return ErrPermissionDenied
	}

	// Guard: cannot disable self
	if principal.ID == targetID {
		_ = svc.record(ctx, &actorID, "admin.users.disable", "admin_user", resourceID, audit.OutcomeDenied, "cannot disable self")
		return fmt.Errorf("%w: cannot disable your own account", ErrPermissionDenied)
	}

	_, found, err := svc.store.FindAdminByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("find admin for disable: %w", err)
	}
	if !found {
		_ = svc.record(ctx, &actorID, "admin.users.disable", "admin_user", resourceID, audit.OutcomeFailure, "admin not found")
		return ErrNotFound
	}

	if err := svc.store.DisableAdmin(ctx, targetID); err != nil {
		_ = svc.record(ctx, &actorID, "admin.users.disable", "admin_user", resourceID, audit.OutcomeFailure, "store error")
		return fmt.Errorf("disable admin user: %w", err)
	}

	_ = svc.record(ctx, &actorID, "admin.users.disable", "admin_user", resourceID, audit.OutcomeSuccess, "admin user disabled")
	return nil
}

// EnableAdminUser re-enables a disabled admin user account.
func (svc *AdminUserService) EnableAdminUser(ctx context.Context, principal auth.Principal, targetID int64) error {
	actorID := principal.ID
	resourceID := fmt.Sprint(targetID)

	if !svc.authorizer.Allowed(principalRoles(principal), rbac.PermissionAdminUsersUpdate) {
		_ = svc.record(ctx, &actorID, "admin.users.enable", "admin_user", resourceID, audit.OutcomeDenied, "permission denied")
		return ErrPermissionDenied
	}

	_, found, err := svc.store.FindAdminByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("find admin for enable: %w", err)
	}
	if !found {
		_ = svc.record(ctx, &actorID, "admin.users.enable", "admin_user", resourceID, audit.OutcomeFailure, "admin not found")
		return ErrNotFound
	}

	if err := svc.store.EnableAdmin(ctx, targetID); err != nil {
		_ = svc.record(ctx, &actorID, "admin.users.enable", "admin_user", resourceID, audit.OutcomeFailure, "store error")
		return fmt.Errorf("enable admin user: %w", err)
	}

	_ = svc.record(ctx, &actorID, "admin.users.enable", "admin_user", resourceID, audit.OutcomeSuccess, "admin user enabled")
	return nil
}

// ResetAdminPassword changes the password for an admin user.
func (svc *AdminUserService) ResetAdminPassword(ctx context.Context, principal auth.Principal, targetID int64, newPassword string) error {
	actorID := principal.ID
	resourceID := fmt.Sprint(targetID)

	if !svc.authorizer.Allowed(principalRoles(principal), rbac.PermissionAdminUsersUpdate) {
		_ = svc.record(ctx, &actorID, "admin.users.reset_password", "admin_user", resourceID, audit.OutcomeDenied, "permission denied")
		return ErrPermissionDenied
	}

	if err := validatePassword(newPassword); err != nil {
		_ = svc.record(ctx, &actorID, "admin.users.reset_password", "admin_user", resourceID, audit.OutcomeFailure, err.Error())
		return err
	}

	_, found, err := svc.store.FindAdminByID(ctx, targetID)
	if err != nil {
		return fmt.Errorf("find admin for password reset: %w", err)
	}
	if !found {
		_ = svc.record(ctx, &actorID, "admin.users.reset_password", "admin_user", resourceID, audit.OutcomeFailure, "admin not found")
		return ErrNotFound
	}

	passwordHash, err := svc.hashFn(newPassword)
	if err != nil {
		_ = svc.record(ctx, &actorID, "admin.users.reset_password", "admin_user", resourceID, audit.OutcomeFailure, "password hashing failed")
		return fmt.Errorf("hash new password: %w", err)
	}

	if err := svc.store.UpdateAdminPassword(ctx, targetID, passwordHash); err != nil {
		_ = svc.record(ctx, &actorID, "admin.users.reset_password", "admin_user", resourceID, audit.OutcomeFailure, "store error")
		return fmt.Errorf("update admin password: %w", err)
	}

	_ = svc.record(ctx, &actorID, "admin.users.reset_password", "admin_user", resourceID, audit.OutcomeSuccess, "admin password reset")
	return nil
}

func (svc *AdminUserService) record(ctx context.Context, actorID *int64, action, resourceType, resourceID string, outcome audit.Outcome, message string) error {
	return svc.recorder.Record(ctx, audit.Event{
		ActorID:      actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Outcome:      outcome,
		Message:      message,
	})
}

func adminToResponse(admin store.Admin) AdminUserResponse {
	roles := make([]string, 0, len(admin.Roles))
	for _, r := range admin.Roles {
		roles = append(roles, string(r))
	}
	return AdminUserResponse{
		ID:          admin.ID,
		Username:    admin.Username,
		DisplayName: admin.DisplayName,
		Email:       admin.Email,
		Roles:       roles,
		Disabled:    admin.DisabledAt != nil,
		CreatedAt:   admin.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   admin.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func isOwner(admin store.Admin) bool {
	for _, r := range admin.Roles {
		if r == rbac.RoleOwner {
			return true
		}
	}
	return false
}

func validateUsername(username string) error {
	if username == "" {
		return fmt.Errorf("%w: username is required", ErrInvalidInput)
	}
	if len(username) > 64 {
		return fmt.Errorf("%w: username must be at most 64 characters", ErrInvalidInput)
	}
	if !usernameRegex.MatchString(username) {
		return fmt.Errorf("%w: username may only contain letters, digits, dots, underscores, and dashes", ErrInvalidInput)
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", ErrInvalidInput)
	}
	if len(password) > 128 {
		return fmt.Errorf("%w: password must be at most 128 characters", ErrInvalidInput)
	}
	if strings.ContainsAny(password, "\n\r") {
		return fmt.Errorf("%w: password must not contain newlines", ErrInvalidInput)
	}
	return nil
}

func validateEmail(email string) error {
	if email == "" {
		return nil
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("%w: invalid email format", ErrInvalidInput)
	}
	return nil
}

func validateRole(role string) (rbac.Role, error) {
	switch rbac.Role(role) {
	case rbac.RoleOwner, rbac.RoleViewer:
		return rbac.Role(role), nil
	default:
		return "", fmt.Errorf("%w: role must be %q or %q", ErrInvalidInput, rbac.RoleOwner, rbac.RoleViewer)
	}
}
