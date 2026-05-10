package validation

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

var (
	ErrUsernameEmpty   = errors.New("username must not be empty")
	ErrUsernameTooLong = errors.New("username must be at most 64 characters")
	ErrUsernameInvalid = errors.New("username contains invalid characters")
	ErrPasswordEmpty   = errors.New("password must not be empty")
	ErrPasswordTooLong = errors.New("password must be at most 128 characters")
	ErrPasswordNewline = errors.New("password must not contain newline characters")
	ErrGroupTooShort   = errors.New("group must be at least 2 characters")
	ErrGroupTooLong    = errors.New("group must be at most 64 characters")
	ErrGroupInvalid    = errors.New("group contains invalid characters")
)

var (
	ErrIPEmpty   = errors.New("IP address must not be empty")
	ErrIPInvalid = errors.New("IP address is not valid")
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
var groupPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Username checks that a username is safe for use with ocpasswd.
func Username(username string) error {
	if username == "" {
		return ErrUsernameEmpty
	}
	if len(username) > 64 {
		return ErrUsernameTooLong
	}
	if !usernamePattern.MatchString(username) {
		return fmt.Errorf("%w: must match [a-zA-Z0-9._-]", ErrUsernameInvalid)
	}
	return nil
}

// Password checks that a password is safe for the ocpasswd stdin protocol.
func Password(password string) error {
	if password == "" {
		return ErrPasswordEmpty
	}
	if len(password) > 128 {
		return ErrPasswordTooLong
	}
	if strings.ContainsAny(password, "\n\r") {
		return ErrPasswordNewline
	}
	return nil
}

// IPAddress checks that an IP address is valid for use in occtl commands.
// It accepts IPv4 and IPv6 addresses. It rejects:
// - empty strings
// - CIDR notation (contains '/')
// - strings starting with '-' (flag injection)
// - any string that does not parse as a valid IP
func IPAddress(ip string) error {
	if ip == "" {
		return ErrIPEmpty
	}
	if ip[0] == '-' {
		return fmt.Errorf("%w: must not start with a dash", ErrIPInvalid)
	}
	if strings.Contains(ip, "/") {
		return fmt.Errorf("%w: CIDR notation is not allowed", ErrIPInvalid)
	}
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("%w: %q", ErrIPInvalid, ip)
	}
	return nil
}

// Group checks that a group name is valid for use with ocpasswd.
// An empty group is valid (means "no group"). Non-empty groups must be 2-64
// characters matching [a-zA-Z0-9_-]. The minimum of 2 is enforced because
// ocserv ignores single-character groups like "*" and "x".
func Group(group string) error {
	if group == "" {
		return nil
	}
	if len(group) < 2 {
		return ErrGroupTooShort
	}
	if len(group) > 64 {
		return ErrGroupTooLong
	}
	if !groupPattern.MatchString(group) {
		return fmt.Errorf("%w: must match [a-zA-Z0-9_-]", ErrGroupInvalid)
	}
	return nil
}
