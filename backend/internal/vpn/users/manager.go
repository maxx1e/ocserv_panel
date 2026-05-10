package users

import (
	"context"
	"errors"
)

// ErrPasswordFileMissing is returned when the ocserv password file does not exist.
var ErrPasswordFileMissing = errors.New("ocserv password file does not exist")



// PasswdUser represents a user entry in the ocserv password file.
type PasswdUser struct {
	Username string `json:"username"`
	Group    string `json:"group"`
	Active   bool   `json:"active"`
}

// PasswdManager manages ocserv password-based users.
type PasswdManager interface {
	CreateUser(ctx context.Context, username, password, group string) error
	UpdateGroup(ctx context.Context, username, group string) error
	DeleteUser(ctx context.Context, username string) error
	UpdatePassword(ctx context.Context, username, newPassword string) error
	ListUsers(ctx context.Context) ([]PasswdUser, error)
	UserExists(ctx context.Context, username string) (bool, error)
	LockUser(ctx context.Context, username string) error
	UnlockUser(ctx context.Context, username string) error
}
