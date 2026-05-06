package ocserv

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateUsername(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		username  string
		wantErr   error
		wantNil   bool
	}{
		{name: "valid simple", username: "alice", wantNil: true},
		{name: "valid with digits", username: "bob123", wantNil: true},
		{name: "valid with dot", username: "user.name", wantNil: true},
		{name: "valid with underscore", username: "user_name", wantNil: true},
		{name: "valid with dash", username: "user-name", wantNil: true},
		{name: "valid single char", username: "a", wantNil: true},
		{name: "empty", username: "", wantErr: ErrUsernameEmpty},
		{name: "contains colon", username: "ab:cd", wantErr: ErrUsernameInvalid},
		{name: "contains newline", username: "user\nname", wantErr: ErrUsernameInvalid},
		{name: "contains null byte", username: "user\x00name", wantErr: ErrUsernameInvalid},
		{name: "path traversal", username: "../etc/shadow", wantErr: ErrUsernameInvalid},
		{name: "contains space", username: "a b", wantErr: ErrUsernameInvalid},
		{name: "too long 65 chars", username: strings.Repeat("a", 65), wantErr: ErrUsernameTooLong},
		{name: "exactly 64 chars", username: strings.Repeat("a", 64), wantNil: true},
		{name: "contains at sign", username: "user@host", wantErr: ErrUsernameInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateUsername(tt.username)
			if tt.wantNil {
				if err != nil {
					t.Errorf("ValidateUsername(%q) = %v, want nil", tt.username, err)
				}
				return
			}
			if err == nil {
				t.Errorf("ValidateUsername(%q) = nil, want error wrapping %v", tt.username, tt.wantErr)
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateUsername(%q) = %v, want error wrapping %v", tt.username, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
		wantErr  error
		wantNil  bool
	}{
		{name: "valid simple", password: "password123", wantNil: true},
		{name: "valid single char", password: "a", wantNil: true},
		{name: "valid complex special chars", password: "complex!@#$%^&*()", wantNil: true},
		{name: "valid exactly 128 chars", password: strings.Repeat("x", 128), wantNil: true},
		{name: "empty", password: "", wantErr: ErrPasswordEmpty},
		{name: "contains newline", password: "pass\nword", wantErr: ErrPasswordNewline},
		{name: "contains carriage return", password: "pass\rword", wantErr: ErrPasswordNewline},
		{name: "too long 129 chars", password: strings.Repeat("x", 129), wantErr: ErrPasswordTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePassword(tt.password)
			if tt.wantNil {
				if err != nil {
					t.Errorf("ValidatePassword(%q) = %v, want nil", tt.password, err)
				}
				return
			}
			if err == nil {
				t.Errorf("ValidatePassword(%q) = nil, want error wrapping %v", tt.password, tt.wantErr)
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidatePassword(%q) = %v, want error wrapping %v", tt.password, err, tt.wantErr)
			}
		})
	}
}
