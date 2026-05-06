package ocserv

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// OcpasswdAdapter implements PasswdManager using the ocpasswd command and
// direct password file parsing.
type OcpasswdAdapter struct {
	binPath    string
	filePath   string
	configPath string
	runner     CommandRunner
}

// Compile-time interface satisfaction check.
var _ PasswdManager = (*OcpasswdAdapter)(nil)

// NewOcpasswdAdapter returns an adapter for ocpasswd user management.
// binPath is the path to the ocpasswd binary, filePath is the ocpasswd
// password file, and configPath is the ocserv config file (required by ocpasswd).
func NewOcpasswdAdapter(binPath, filePath, configPath string, runner CommandRunner) *OcpasswdAdapter {
	return &OcpasswdAdapter{
		binPath:    binPath,
		filePath:   filePath,
		configPath: configPath,
		runner:     runner,
	}
}

// CreateUser creates a new password-based user by running ocpasswd.
// The password is piped via stdin to avoid shell injection.
func (a *OcpasswdAdapter) CreateUser(ctx context.Context, username, password string) error {
	return a.runPasswdCommand(ctx, username, password)
}

// DeleteUser removes a user from the password file using ocpasswd -d.
func (a *OcpasswdAdapter) DeleteUser(ctx context.Context, username string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	args := []string{"-c", a.configPath, "-d", username}
	result, err := a.runner.Run(ctx, a.binPath, args, nil)
	if err != nil {
		return fmt.Errorf("delete user %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("delete user %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

// UpdatePassword changes a user's password by running ocpasswd.
// The new password is piped via stdin.
func (a *OcpasswdAdapter) UpdatePassword(ctx context.Context, username, newPassword string) error {
	return a.runPasswdCommand(ctx, username, newPassword)
}

// ListUsers parses the ocpasswd file and returns all user entries.
// Returns an empty slice when the file does not exist yet.
func (a *OcpasswdAdapter) ListUsers(ctx context.Context) ([]PasswdUser, error) {
	_ = ctx
	data, err := os.ReadFile(a.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []PasswdUser{}, nil
		}
		return nil, fmt.Errorf("read password file %s: %w", a.filePath, err)
	}
	return ParsePasswdFile(data), nil
}

// UserExists checks whether a username appears in the password file.
func (a *OcpasswdAdapter) UserExists(ctx context.Context, username string) (bool, error) {
	users, err := a.ListUsers(ctx)
	if err != nil {
		return false, err
	}
	for _, u := range users {
		if u.Username == username {
			return true, nil
		}
	}
	return false, nil
}

// runPasswdCommand runs ocpasswd for create/update operations, piping the
// password via stdin to avoid it appearing in process arguments.
func (a *OcpasswdAdapter) runPasswdCommand(ctx context.Context, username, password string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	args := []string{"-c", a.configPath, username}
	stdin := strings.NewReader(password + "\n" + password + "\n") // ocpasswd asks twice
	result, err := a.runner.Run(ctx, a.binPath, args, stdin)
	if err != nil {
		return fmt.Errorf("ocpasswd for user %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("ocpasswd for user %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

// ParsePasswdFile parses the ocpasswd file format: username:group:hashed_password
// A hash value of "!" indicates a disabled account.
// Exported for use in tests.
func ParsePasswdFile(data []byte) []PasswdUser {
	var users []PasswdUser
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 2 {
			continue
		}
		username := parts[0]
		group := parts[1]
		// A hash field of exactly "!" means the account is disabled.
		active := len(parts) < 3 || parts[2] != "!"
		users = append(users, PasswdUser{Username: username, Group: group, Active: active})
	}
	return users
}

// LockUser disables a user by setting their hash field to "!" in the password file.
func (a *OcpasswdAdapter) LockUser(ctx context.Context, username string) error {
	_ = ctx
	return a.setUserActive(username, false)
}

// UnlockUser re-enables a locked user by setting their hash field to "*" (requires password reset).
func (a *OcpasswdAdapter) UnlockUser(ctx context.Context, username string) error {
	_ = ctx
	return a.setUserActive(username, true)
}

func (a *OcpasswdAdapter) setUserActive(username string, active bool) error {
	data, err := os.ReadFile(a.filePath)
	if err != nil {
		return fmt.Errorf("read password file: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 3)
		if len(parts) < 2 || parts[0] != username {
			continue
		}
		found = true
		if active {
			if len(parts) == 3 && parts[2] == "!" {
				parts[2] = "*"
				lines[i] = strings.Join(parts, ":")
			}
		} else {
			if len(parts) < 3 {
				lines[i] = trimmed + ":!"
			} else {
				parts[2] = "!"
				lines[i] = strings.Join(parts, ":")
			}
		}
		break
	}
	if !found {
		return fmt.Errorf("user %s not found", username)
	}
	return os.WriteFile(a.filePath, []byte(strings.Join(lines, "\n")), 0600)
}
