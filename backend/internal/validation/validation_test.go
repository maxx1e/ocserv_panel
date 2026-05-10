package validation_test

import (
	"errors"
	"strings"
	"testing"

	"ocserv_panel/backend/internal/validation"
)

func TestUsername(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		username string
		wantErr  error
		wantNil  bool
	}{
		{name: "valid simple", username: "alice", wantNil: true},
		{name: "valid with digits", username: "bob123", wantNil: true},
		{name: "valid with dot", username: "user.name", wantNil: true},
		{name: "valid with underscore", username: "user_name", wantNil: true},
		{name: "valid with dash", username: "user-name", wantNil: true},
		{name: "valid single char", username: "a", wantNil: true},
		{name: "empty", username: "", wantErr: validation.ErrUsernameEmpty},
		{name: "contains colon", username: "ab:cd", wantErr: validation.ErrUsernameInvalid},
		{name: "contains newline", username: "user\nname", wantErr: validation.ErrUsernameInvalid},
		{name: "contains null byte", username: "user\x00name", wantErr: validation.ErrUsernameInvalid},
		{name: "path traversal", username: "../etc/shadow", wantErr: validation.ErrUsernameInvalid},
		{name: "contains space", username: "a b", wantErr: validation.ErrUsernameInvalid},
		{name: "too long 65 chars", username: strings.Repeat("a", 65), wantErr: validation.ErrUsernameTooLong},
		{name: "exactly 64 chars", username: strings.Repeat("a", 64), wantNil: true},
		{name: "contains at sign", username: "user@host", wantErr: validation.ErrUsernameInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validation.Username(tt.username)
			if tt.wantNil {
				if err != nil {
					t.Errorf("Username(%q) = %v, want nil", tt.username, err)
				}
				return
			}
			if err == nil {
				t.Errorf("Username(%q) = nil, want error wrapping %v", tt.username, tt.wantErr)
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Username(%q) = %v, want error wrapping %v", tt.username, err, tt.wantErr)
			}
		})
	}
}

func TestPassword(t *testing.T) {
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
		{name: "empty", password: "", wantErr: validation.ErrPasswordEmpty},
		{name: "contains newline", password: "pass\nword", wantErr: validation.ErrPasswordNewline},
		{name: "contains carriage return", password: "pass\rword", wantErr: validation.ErrPasswordNewline},
		{name: "too long 129 chars", password: strings.Repeat("x", 129), wantErr: validation.ErrPasswordTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validation.Password(tt.password)
			if tt.wantNil {
				if err != nil {
					t.Errorf("Password(%q) = %v, want nil", tt.password, err)
				}
				return
			}
			if err == nil {
				t.Errorf("Password(%q) = nil, want error wrapping %v", tt.password, tt.wantErr)
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Password(%q) = %v, want error wrapping %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestIPAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ip      string
		wantErr error
	}{
		{name: "valid IPv4", ip: "192.168.1.1", wantErr: nil},
		{name: "valid IPv4 loopback", ip: "127.0.0.1", wantErr: nil},
		{name: "valid IPv6", ip: "::1", wantErr: nil},
		{name: "valid IPv6 full", ip: "2001:db8::1", wantErr: nil},
		{name: "valid IPv6 mapped IPv4", ip: "::ffff:192.168.1.1", wantErr: nil},
		{name: "empty string", ip: "", wantErr: validation.ErrIPEmpty},
		{name: "CIDR notation v4", ip: "192.168.1.0/24", wantErr: validation.ErrIPInvalid},
		{name: "CIDR notation v6", ip: "2001:db8::/32", wantErr: validation.ErrIPInvalid},
		{name: "leading dash", ip: "-127.0.0.1", wantErr: validation.ErrIPInvalid},
		{name: "garbage string", ip: "not-an-ip", wantErr: validation.ErrIPInvalid},
		{name: "just dots", ip: "...", wantErr: validation.ErrIPInvalid},
		{name: "spaces", ip: " 1.2.3.4", wantErr: validation.ErrIPInvalid},
		{name: "port number", ip: "192.168.1.1:8080", wantErr: validation.ErrIPInvalid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validation.IPAddress(tc.ip)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("IPAddress(%q) unexpected error: %v", tc.ip, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("IPAddress(%q) expected error wrapping %v, got nil", tc.ip, tc.wantErr)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("IPAddress(%q) error = %v, want %v", tc.ip, err, tc.wantErr)
			}
		})
	}
}

func TestGroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		group   string
		wantErr error
		wantNil bool
	}{
		{name: "empty is valid", group: "", wantNil: true},
		{name: "valid simple", group: "vpn", wantNil: true},
		{name: "valid with digits", group: "group123", wantNil: true},
		{name: "valid with underscore", group: "my_group", wantNil: true},
		{name: "valid with hyphen", group: "my-group", wantNil: true},
		{name: "valid exactly 2 chars", group: "ab", wantNil: true},
		{name: "valid exactly 64 chars", group: strings.Repeat("g", 64), wantNil: true},
		{name: "too short single char", group: "a", wantErr: validation.ErrGroupTooShort},
		{name: "too short asterisk", group: "*", wantErr: validation.ErrGroupTooShort},
		{name: "too short x", group: "x", wantErr: validation.ErrGroupTooShort},
		{name: "too long 65 chars", group: strings.Repeat("g", 65), wantErr: validation.ErrGroupTooLong},
		{name: "contains dot", group: "my.group", wantErr: validation.ErrGroupInvalid},
		{name: "contains space", group: "my group", wantErr: validation.ErrGroupInvalid},
		{name: "contains colon", group: "my:group", wantErr: validation.ErrGroupInvalid},
		{name: "contains slash", group: "my/group", wantErr: validation.ErrGroupInvalid},
		{name: "contains at sign", group: "my@group", wantErr: validation.ErrGroupInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validation.Group(tt.group)
			if tt.wantNil {
				if err != nil {
					t.Errorf("Group(%q) = %v, want nil", tt.group, err)
				}
				return
			}
			if err == nil {
				t.Errorf("Group(%q) = nil, want error wrapping %v", tt.group, tt.wantErr)
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Group(%q) = %v, want error wrapping %v", tt.group, err, tt.wantErr)
			}
		})
	}
}
