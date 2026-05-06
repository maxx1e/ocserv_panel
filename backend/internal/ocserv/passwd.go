package ocserv

import "context"

// PasswdUser represents a user entry in the ocserv password file.
type PasswdUser struct {
	Username string `json:"username"`
	Group    string `json:"group"`
	Active   bool   `json:"active"`
}

// PasswdManager manages ocserv password-based users.
type PasswdManager interface {
	CreateUser(ctx context.Context, username, password string) error
	DeleteUser(ctx context.Context, username string) error
	UpdatePassword(ctx context.Context, username, newPassword string) error
	ListUsers(ctx context.Context) ([]PasswdUser, error)
	UserExists(ctx context.Context, username string) (bool, error)
	LockUser(ctx context.Context, username string) error
	UnlockUser(ctx context.Context, username string) error
}
