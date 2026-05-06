package ocserv

import (
	"errors"
	"fmt"
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
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// ValidateUsername checks that a username is safe for use with ocpasswd.
func ValidateUsername(username string) error {
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

// ValidatePassword checks that a password is safe for the ocpasswd stdin protocol.
func ValidatePassword(password string) error {
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
