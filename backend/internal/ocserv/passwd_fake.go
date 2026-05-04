package ocserv

import (
	"context"
	"sync"
)

// FakePasswdManager is a thread-safe test double for PasswdManager.
type FakePasswdManager struct {
	mu    sync.Mutex
	users map[string]PasswdUser
	err   error
}

// Compile-time interface satisfaction check.
var _ PasswdManager = (*FakePasswdManager)(nil)

// NewFakePasswdManager returns a FakePasswdManager with an empty user store.
func NewFakePasswdManager() *FakePasswdManager {
	return &FakePasswdManager{users: make(map[string]PasswdUser)}
}

// CreateUser adds a user to the in-memory store.
func (f *FakePasswdManager) CreateUser(ctx context.Context, username, password string) error {
	_ = ctx
	_ = password
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.users[username] = PasswdUser{Username: username, Group: "default", Active: true}
	return nil
}

// DeleteUser removes a user from the in-memory store.
func (f *FakePasswdManager) DeleteUser(ctx context.Context, username string) error {
	_ = ctx
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	delete(f.users, username)
	return nil
}

// UpdatePassword is a no-op in the fake (passwords are not stored).
func (f *FakePasswdManager) UpdatePassword(ctx context.Context, username, newPassword string) error {
	_ = ctx
	_ = newPassword
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.err
}

// ListUsers returns a snapshot of the in-memory user store.
func (f *FakePasswdManager) ListUsers(ctx context.Context) ([]PasswdUser, error) {
	_ = ctx
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	result := make([]PasswdUser, 0, len(f.users))
	for _, u := range f.users {
		result = append(result, u)
	}
	return result, nil
}

// UserExists reports whether the username is in the in-memory store.
func (f *FakePasswdManager) UserExists(ctx context.Context, username string) (bool, error) {
	_ = ctx
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return false, f.err
	}
	_, ok := f.users[username]
	return ok, nil
}

// SetError configures the error returned by all operations.
func (f *FakePasswdManager) SetError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}
