package app

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/db"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/store"
)

func newTestServices(t *testing.T) (*store.Store, audit.SQLiteRecorder) {
	t.Helper()

	ctx := context.Background()
	database, err := db.OpenSQLite(ctx, filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Fatalf("close sqlite: %v", err)
		}
	})

	if err := db.ApplyMigrations(ctx, database, "../../../migrations"); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	s := store.New(database)
	return s, audit.NewSQLiteRecorder(s)
}

func TestAuthServiceSetupOnceSucceedsThenRefuses(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	service := NewAuthService(s, recorder)

	required, err := service.SetupRequired(ctx)
	if err != nil {
		t.Fatalf("setup required: %v", err)
	}
	if !required {
		t.Fatal("setup required = false, want true")
	}

	principal, err := service.CreateFirstOwner(ctx, "owner", "correct horse battery staple")
	if err != nil {
		t.Fatalf("create first owner: %v", err)
	}
	if !principal.Authenticated() || principal.Username != "owner" || len(principal.Roles) != 1 || principal.Roles[0] != string(rbac.RoleOwner) {
		t.Fatalf("principal = %#v, want owner principal", principal)
	}

	required, err = service.SetupRequired(ctx)
	if err != nil {
		t.Fatalf("setup required after create: %v", err)
	}
	if required {
		t.Fatal("setup required = true after owner exists")
	}
	if _, err := service.CreateFirstOwner(ctx, "second", "another password"); !errors.Is(err, ErrSetupAlreadyComplete) {
		t.Fatalf("second create err = %v, want ErrSetupAlreadyComplete", err)
	}

	events, err := s.RecentAuditEvents(ctx, 2)
	if err != nil {
		t.Fatalf("recent audit events: %v", err)
	}
	assertAuditOutcome(t, events, "auth.setup", string(audit.OutcomeDenied))
	assertAuditOutcome(t, events, "auth.setup", string(audit.OutcomeSuccess))
}

func TestAuthServiceCreateFirstOwnerRefusesEmptyInput(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	service := NewAuthService(s, recorder)

	tests := []struct {
		name     string
		username string
		password string
	}{
		{name: "empty username", username: "", password: "password"},
		{name: "blank username", username: "   ", password: "password"},
		{name: "empty password", username: "owner", password: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := service.CreateFirstOwner(ctx, tt.username, tt.password); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("create first owner err = %v, want ErrInvalidInput", err)
			}
		})
	}

	count, err := s.AdminCount(ctx)
	if err != nil {
		t.Fatalf("admin count: %v", err)
	}
	if count != 0 {
		t.Fatalf("admin count = %d, want 0", count)
	}
	assertAuditOutcome(t, mustRecentAuditEvents(t, s, 3), "auth.setup", string(audit.OutcomeFailure))
}

func TestAuthServiceLoginStoresOnlyTokenHashAndAuditsGenericFailure(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	service := NewAuthService(s, recorder)
	if _, err := service.CreateFirstOwner(ctx, "owner", "right-password"); err != nil {
		t.Fatalf("create first owner: %v", err)
	}

	login, err := service.Login(ctx, "owner", "right-password", "go-test", "127.0.0.1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.Token == "" || !login.Principal.Authenticated() {
		t.Fatalf("login result = %#v, want raw token and principal", login)
	}
	if time.Until(login.ExpiresAt) < 11*time.Hour || time.Until(login.ExpiresAt) > 13*time.Hour {
		t.Fatalf("expires at = %s, want about 12 hours", login.ExpiresAt)
	}

	var storedTokenHash string
	if err := s.DB.QueryRowContext(ctx, `SELECT token_hash FROM admin_sessions WHERE admin_user_id = ?`, login.Principal.ID).Scan(&storedTokenHash); err != nil {
		t.Fatalf("query token hash: %v", err)
	}
	if storedTokenHash == login.Token || storedTokenHash != auth.HashSessionToken(login.Token) {
		t.Fatalf("stored token hash = %q, raw token = %q", storedTokenHash, login.Token)
	}

	_, err = service.Login(ctx, "owner", "wrong-password", "go-test", "127.0.0.1")
	if !errors.Is(err, ErrInvalidCredentials) || strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), "owner") {
		t.Fatalf("login failure err = %v, want generic ErrInvalidCredentials", err)
	}
	assertAuditOutcome(t, mustRecentAuditEvents(t, s, 3), "auth.login", string(audit.OutcomeFailure))
	assertAuditOutcome(t, mustRecentAuditEvents(t, s, 3), "auth.login", string(audit.OutcomeSuccess))
}

func TestAuthServiceLoginMissingUserExercisesDummyPasswordPath(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	service := NewAuthService(s, recorder)

	called := false
	originalVerifier := verifyDummyPasswordForLogin
	verifyDummyPasswordForLogin = func(password string) error {
		called = true
		if password != "wrong-password" {
			t.Fatalf("dummy verifier password = %q, want attempted password", password)
		}
		return nil
	}
	t.Cleanup(func() { verifyDummyPasswordForLogin = originalVerifier })

	_, err := service.Login(ctx, "missing", "wrong-password", "go-test", "127.0.0.1")
	if !errors.Is(err, ErrInvalidCredentials) || strings.Contains(err.Error(), "missing") || strings.Contains(err.Error(), "password") {
		t.Fatalf("missing-user login err = %v, want generic ErrInvalidCredentials", err)
	}
	if !called {
		t.Fatal("missing-user login did not exercise dummy password verification path")
	}
	assertAuditOutcome(t, mustRecentAuditEvents(t, s, 1), "auth.login", string(audit.OutcomeFailure))
}

func TestAuthServiceLoginDisabledUserUsesGenericFailureAndDummyPath(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	service := NewAuthService(s, recorder)
	principal, err := service.CreateFirstOwner(ctx, "owner", "right-password")
	if err != nil {
		t.Fatalf("create first owner: %v", err)
	}
	if _, err := s.DB.ExecContext(ctx, `UPDATE admin_users SET disabled_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339Nano), principal.ID); err != nil {
		t.Fatalf("disable owner: %v", err)
	}

	called := false
	originalVerifier := verifyDummyPasswordForLogin
	verifyDummyPasswordForLogin = func(password string) error {
		called = true
		if password != "right-password" {
			t.Fatalf("dummy verifier password = %q, want attempted password", password)
		}
		return nil
	}
	t.Cleanup(func() { verifyDummyPasswordForLogin = originalVerifier })

	_, err = service.Login(ctx, "owner", "right-password", "go-test", "127.0.0.1")
	if !errors.Is(err, ErrInvalidCredentials) || strings.Contains(err.Error(), "owner") || strings.Contains(err.Error(), "password") {
		t.Fatalf("disabled-user login err = %v, want generic ErrInvalidCredentials", err)
	}
	if !called {
		t.Fatal("disabled-user login did not exercise dummy password verification path")
	}
	if sessionCount := countRows(t, ctx, s.DB, `SELECT COUNT(*) FROM admin_sessions WHERE admin_user_id = ?`, principal.ID); sessionCount != 0 {
		t.Fatalf("disabled-user session count = %d, want 0", sessionCount)
	}
	assertAuditOutcome(t, mustRecentAuditEvents(t, s, 1), "auth.login", string(audit.OutcomeFailure))
}

func TestAuthServiceValidateSessionAndLogout(t *testing.T) {
	ctx := context.Background()
	s, recorder := newTestServices(t)
	service := NewAuthService(s, recorder)
	if _, err := service.CreateFirstOwner(ctx, "owner", "right-password"); err != nil {
		t.Fatalf("create first owner: %v", err)
	}
	login, err := service.Login(ctx, "owner", "right-password", "go-test", "127.0.0.1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	validated, err := service.ValidateSession(ctx, login.Token)
	if err != nil {
		t.Fatalf("validate session: %v", err)
	}
	if !validated.Authenticated || validated.Principal.Username != "owner" {
		t.Fatalf("validated = %#v, want owner authenticated", validated)
	}

	if err := service.Logout(ctx, login.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	validated, err = service.ValidateSession(ctx, login.Token)
	if err != nil {
		t.Fatalf("validate revoked session: %v", err)
	}
	if validated.Authenticated {
		t.Fatal("revoked session validated as authenticated")
	}

	if malformed, err := service.ValidateSession(ctx, "not a raw token"); err != nil || malformed.Authenticated {
		t.Fatalf("malformed validation = %#v err=%v, want unauthenticated without error", malformed, err)
	}

	admin, found, err := s.FindAdminByUsername(ctx, "owner")
	if err != nil || !found {
		t.Fatalf("find owner found=%v err=%v", found, err)
	}
	expiredRaw, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("new expired session token: %v", err)
	}
	if _, err := s.CreateSession(ctx, admin.ID, auth.HashSessionToken(expiredRaw), time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	validated, err = service.ValidateSession(ctx, expiredRaw)
	if err != nil {
		t.Fatalf("validate expired session: %v", err)
	}
	if validated.Authenticated {
		t.Fatal("expired session validated as authenticated")
	}
	assertAuditOutcome(t, mustRecentAuditEvents(t, s, 2), "auth.logout", string(audit.OutcomeSuccess))
}

func createAdminWithRole(t *testing.T, ctx context.Context, s *store.Store, username string, role rbac.Role) auth.Principal {
	t.Helper()

	passwordHash, err := auth.HashPassword("password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.DB.ExecContext(ctx, `INSERT INTO admin_users (username, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?)`, username, passwordHash, now, now)
	if err != nil {
		t.Fatalf("insert admin: %v", err)
	}
	adminID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("read admin id: %v", err)
	}
	var roleID int64
	if err := s.DB.QueryRowContext(ctx, `SELECT id FROM roles WHERE name = ?`, string(role)).Scan(&roleID); err != nil {
		t.Fatalf("select role: %v", err)
	}
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO admin_user_roles (admin_user_id, role_id, created_at) VALUES (?, ?, ?)`, adminID, roleID, now); err != nil {
		t.Fatalf("insert admin role: %v", err)
	}
	return auth.Principal{ID: adminID, Username: username, Roles: []string{string(role)}, CreatedAt: time.Now().UTC()}
}

func mustRecentAuditEvents(t *testing.T, s *store.Store, limit int) []store.AuditEvent {
	t.Helper()
	events, err := s.RecentAuditEvents(context.Background(), limit)
	if err != nil {
		t.Fatalf("recent audit events: %v", err)
	}
	return events
}

func assertAuditOutcome(t *testing.T, events []store.AuditEvent, action string, outcome string) {
	t.Helper()
	for _, event := range events {
		if event.Action == action && event.Outcome == outcome {
			return
		}
	}
	t.Fatalf("audit events = %#v, want action %q outcome %q", events, action, outcome)
}

func countRows(t *testing.T, ctx context.Context, database *sql.DB, query string, args ...any) int64 {
	t.Helper()
	var count int64
	if err := database.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}
