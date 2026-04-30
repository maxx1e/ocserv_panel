// Package app contains application services used by HTTP handlers.
package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

var (
	ErrInvalidInput         = errors.New("invalid input")
	ErrSetupAlreadyComplete = errors.New("setup already complete")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrUnauthenticated      = errors.New("authentication required")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrInvalidConfirmation  = errors.New("invalid confirmation")
	ErrInvalidRetention     = errors.New("invalid retention")
)

// AuthService manages local admin authentication and bootstrap setup.
type AuthService struct {
	store    *store.Store
	recorder audit.Recorder
	now      func() time.Time
}

// LoginResult is returned for successful admin logins.
type LoginResult struct {
	Token     string
	Principal auth.Principal
	ExpiresAt time.Time
}

// SessionValidation is the result of resolving a raw session token.
type SessionValidation struct {
	Authenticated bool
	Principal     auth.Principal
}

// NewAuthService returns an AuthService backed by SQLite storage.
func NewAuthService(s *store.Store, recorder audit.Recorder) *AuthService {
	if recorder == nil {
		recorder = audit.NoopRecorder{}
	}
	return &AuthService{store: s, recorder: recorder, now: func() time.Time { return time.Now().UTC() }}
}

// SetupRequired reports whether the first owner bootstrap is still available.
func (s *AuthService) SetupRequired(ctx context.Context) (bool, error) {
	count, err := s.store.AdminCount(ctx)
	if err != nil {
		return false, fmt.Errorf("check setup requirement: %w", err)
	}
	return count == 0, nil
}

// CreateFirstOwner creates the bootstrap owner exactly once.
func (s *AuthService) CreateFirstOwner(ctx context.Context, username string, password string) (auth.Principal, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		_ = s.record(ctx, nil, "auth.setup", "admin_user", "", audit.OutcomeFailure, "setup input rejected", nil)
		return auth.Principal{}, ErrInvalidInput
	}

	required, err := s.SetupRequired(ctx)
	if err != nil {
		_ = s.record(ctx, nil, "auth.setup", "admin_user", username, audit.OutcomeFailure, "setup availability check failed", nil)
		return auth.Principal{}, err
	}
	if !required {
		_ = s.record(ctx, nil, "auth.setup", "admin_user", username, audit.OutcomeDenied, "setup already complete", nil)
		return auth.Principal{}, ErrSetupAlreadyComplete
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		_ = s.record(ctx, nil, "auth.setup", "admin_user", username, audit.OutcomeFailure, "password hashing failed", nil)
		return auth.Principal{}, fmt.Errorf("hash bootstrap owner password: %w", err)
	}
	admin, err := s.store.CreateFirstOwner(ctx, username, passwordHash)
	if err != nil {
		outcome := audit.OutcomeFailure
		if errors.Is(err, store.ErrFirstOwnerExists) {
			outcome = audit.OutcomeDenied
		}
		_ = s.record(ctx, nil, "auth.setup", "admin_user", username, outcome, "setup failed", nil)
		if errors.Is(err, store.ErrFirstOwnerExists) {
			return auth.Principal{}, ErrSetupAlreadyComplete
		}
		return auth.Principal{}, fmt.Errorf("create bootstrap owner: %w", err)
	}
	principal := principalFromAdmin(admin)
	actorID := admin.ID
	if err := s.record(ctx, &actorID, "auth.setup", "admin_user", admin.Username, audit.OutcomeSuccess, "bootstrap owner created", nil); err != nil {
		return auth.Principal{}, fmt.Errorf("audit bootstrap owner creation: %w", err)
	}
	return principal, nil
}

// Login verifies credentials and creates a 12-hour session.
func (s *AuthService) Login(ctx context.Context, username string, password string, userAgent string, remoteAddr string) (LoginResult, error) {
	admin, found, err := s.store.FindAdminByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		_ = s.record(ctx, nil, "auth.login", "admin_session", "", audit.OutcomeFailure, "login lookup failed", nil)
		return LoginResult{}, fmt.Errorf("lookup admin for login: %w", err)
	}
	if !found || admin.DisabledAt != nil {
		if err := verifyDummyPasswordForLogin(password); err != nil {
			_ = s.record(ctx, nil, "auth.login", "admin_session", "", audit.OutcomeFailure, "login dummy verification failed", nil)
			return LoginResult{}, fmt.Errorf("verify dummy login password: %w", err)
		}
		_ = s.record(ctx, nil, "auth.login", "admin_session", "", audit.OutcomeFailure, "login failed", loginMetadata(userAgent, remoteAddr))
		return LoginResult{}, ErrInvalidCredentials
	}
	valid, err := auth.VerifyPassword(admin.PasswordHash, password)
	if err != nil || !valid {
		actorID := admin.ID
		_ = s.record(ctx, &actorID, "auth.login", "admin_session", "", audit.OutcomeFailure, "login failed", loginMetadata(userAgent, remoteAddr))
		return LoginResult{}, ErrInvalidCredentials
	}

	rawToken, err := auth.NewSessionToken()
	if err != nil {
		actorID := admin.ID
		_ = s.record(ctx, &actorID, "auth.login", "admin_session", "", audit.OutcomeFailure, "session token generation failed", nil)
		return LoginResult{}, fmt.Errorf("create session token: %w", err)
	}
	expiresAt := s.now().Add(12 * time.Hour).UTC()
	session, err := s.store.CreateSession(ctx, admin.ID, auth.HashSessionToken(rawToken), expiresAt)
	if err != nil {
		actorID := admin.ID
		_ = s.record(ctx, &actorID, "auth.login", "admin_session", "", audit.OutcomeFailure, "session creation failed", nil)
		return LoginResult{}, fmt.Errorf("create admin session: %w", err)
	}
	actorID := admin.ID
	if err := s.record(ctx, &actorID, "auth.login", "admin_session", fmt.Sprint(session.ID), audit.OutcomeSuccess, "login succeeded", loginMetadata(userAgent, remoteAddr)); err != nil {
		return LoginResult{}, fmt.Errorf("audit login: %w", err)
	}
	return LoginResult{Token: rawToken, Principal: principalFromAdmin(admin), ExpiresAt: expiresAt}, nil
}

// ValidateSession resolves an active, unexpired, unrevoked session token.
func (s *AuthService) ValidateSession(ctx context.Context, rawToken string) (SessionValidation, error) {
	if !validRawSessionToken(rawToken) {
		return SessionValidation{}, nil
	}
	admin, found, err := s.store.FindAdminBySessionTokenHash(ctx, auth.HashSessionToken(rawToken), s.now())
	if err != nil {
		return SessionValidation{}, fmt.Errorf("validate admin session: %w", err)
	}
	if !found {
		return SessionValidation{}, nil
	}
	return SessionValidation{Authenticated: true, Principal: principalFromAdmin(admin)}, nil
}

// Logout revokes the session matching rawToken when the token is well-formed.
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	if !validRawSessionToken(rawToken) {
		_ = s.record(ctx, nil, "auth.logout", "admin_session", "", audit.OutcomeFailure, "logout token rejected", nil)
		return nil
	}
	validated, err := s.ValidateSession(ctx, rawToken)
	if err != nil {
		_ = s.record(ctx, nil, "auth.logout", "admin_session", "", audit.OutcomeFailure, "logout validation failed", nil)
		return err
	}
	if err := s.store.RevokeSessionByTokenHash(ctx, auth.HashSessionToken(rawToken), s.now()); err != nil {
		_ = s.record(ctx, nil, "auth.logout", "admin_session", "", audit.OutcomeFailure, "logout revoke failed", nil)
		return fmt.Errorf("revoke admin session: %w", err)
	}
	var actorID *int64
	if validated.Authenticated {
		id := validated.Principal.ID
		actorID = &id
	}
	return s.record(ctx, actorID, "auth.logout", "admin_session", "", audit.OutcomeSuccess, "logout succeeded", nil)
}

func (s *AuthService) record(ctx context.Context, actorID *int64, action string, resourceType string, resourceID string, outcome audit.Outcome, message string, metadata map[string]string) error {
	return s.recorder.Record(ctx, audit.Event{ActorID: actorID, Action: action, ResourceType: resourceType, ResourceID: resourceID, Outcome: outcome, Message: message, Metadata: metadata, CreatedAt: s.now()})
}

func principalFromAdmin(admin store.Admin) auth.Principal {
	roles := make([]string, 0, len(admin.Roles))
	for _, role := range admin.Roles {
		roles = append(roles, string(role))
	}
	return auth.Principal{ID: admin.ID, Username: admin.Username, Roles: roles, CreatedAt: admin.CreatedAt}
}

func principalRoles(principal auth.Principal) []rbac.Role {
	roles := make([]rbac.Role, 0, len(principal.Roles))
	for _, role := range principal.Roles {
		roles = append(roles, rbac.Role(role))
	}
	return roles
}

func validRawSessionToken(rawToken string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(rawToken)
	return err == nil && len(decoded) == auth.SessionTokenBytes
}

func loginMetadata(userAgent string, remoteAddr string) map[string]string {
	return map[string]string{"user_agent": userAgent, "remote_addr": remoteAddr}
}

var (
	dummyLoginPasswordHash      string
	dummyLoginPasswordHashErr   error
	dummyLoginPasswordHashOnce  sync.Once
	verifyDummyPasswordForLogin = func(password string) error {
		dummyLoginPasswordHashOnce.Do(func() {
			dummyLoginPasswordHash, dummyLoginPasswordHashErr = auth.HashPassword("dummy login password")
		})
		if dummyLoginPasswordHashErr != nil {
			return fmt.Errorf("prepare dummy login password hash: %w", dummyLoginPasswordHashErr)
		}
		if _, err := auth.VerifyPassword(dummyLoginPasswordHash, password); err != nil {
			return fmt.Errorf("verify dummy login password: %w", err)
		}
		return nil
	}
)
