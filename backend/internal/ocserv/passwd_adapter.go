package ocserv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
// password file, and configPath is the ocserv config file (retained for
// potential future use but not currently passed to ocpasswd).
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
// When group is non-empty, the -g flag is passed to assign the user to that group.
func (a *OcpasswdAdapter) CreateUser(ctx context.Context, username, password, group string) error {
	return a.runPasswdCommand(ctx, username, password, group)
}

// DeleteUser removes a user from the password file using ocpasswd -d.
func (a *OcpasswdAdapter) DeleteUser(ctx context.Context, username string) error {
	// Verify password file exists before attempting deletion.
	if _, err := os.Stat(a.filePath); os.IsNotExist(err) {
		return fmt.Errorf("password file %s: %w", a.filePath, ErrPasswordFileMissing)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	args := []string{"-c", a.filePath, "-d", username}
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
	return a.runPasswdCommand(ctx, username, newPassword, "")
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
// When group is non-empty, the -g flag is included before the username.
func (a *OcpasswdAdapter) runPasswdCommand(ctx context.Context, username, password, group string) error {
	// Ensure the directory for the password file exists (ocpasswd creates the file but not the directory).
	dir := filepath.Dir(a.filePath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
			return fmt.Errorf("create password file directory %s: %w", dir, mkErr)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	args := []string{"-c", a.filePath}
	if group != "" {
		args = append(args, "-g", group)
	}
	args = append(args, username)
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
// A hash value of "!" or a hash prefixed with "!" (e.g., "!$5$hash...") indicates
// a disabled (locked) account. The "!" prefix is the native ocpasswd -l format.
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
		// A hash field of exactly "!" or prefixed with "!" means the account is locked.
		active := len(parts) < 3 || (parts[2] != "!" && !strings.HasPrefix(parts[2], "!"))
		users = append(users, PasswdUser{Username: username, Group: group, Active: active})
	}
	return users
}

// LockUser disables a user account using ocpasswd -l, which prefixes the
// password hash with "!" without destroying it. The original hash is preserved
// so that UnlockUser can restore access.
func (a *OcpasswdAdapter) LockUser(ctx context.Context, username string) error {
	if _, err := os.Stat(a.filePath); os.IsNotExist(err) {
		return fmt.Errorf("password file %s: %w", a.filePath, ErrPasswordFileMissing)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	args := []string{"-c", a.filePath, "-l", username}
	result, err := a.runner.Run(ctx, a.binPath, args, nil)
	if err != nil {
		return fmt.Errorf("lock user %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("lock user %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

// UnlockUser re-enables a locked user account using ocpasswd -u, which removes
// the "!" prefix from the password hash, restoring the original credentials.
func (a *OcpasswdAdapter) UnlockUser(ctx context.Context, username string) error {
	if _, err := os.Stat(a.filePath); os.IsNotExist(err) {
		return fmt.Errorf("password file %s: %w", a.filePath, ErrPasswordFileMissing)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	args := []string{"-c", a.filePath, "-u", username}
	result, err := a.runner.Run(ctx, a.binPath, args, nil)
	if err != nil {
		return fmt.Errorf("unlock user %s: %w", username, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("unlock user %s: %s", username, strings.TrimSpace(result.Stderr))
	}
	return nil
}

// UpdateGroup changes a user's group in the password file by directly editing
// the second colon-separated field. An empty group sets the field to "*" (the
// ocserv default/no-group marker).
func (a *OcpasswdAdapter) UpdateGroup(ctx context.Context, username, group string) error {
	_ = ctx
	data, err := os.ReadFile(a.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("password file %s: %w", a.filePath, ErrPasswordFileMissing)
		}
		return fmt.Errorf("read password file: %w", err)
	}

	newGroup := group
	if newGroup == "" {
		newGroup = "*"
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
		parts[1] = newGroup
		lines[i] = strings.Join(parts, ":")
		break
	}
	if !found {
		return fmt.Errorf("user %s not found", username)
	}
	return os.WriteFile(a.filePath, []byte(strings.Join(lines, "\n")), 0600)
}

