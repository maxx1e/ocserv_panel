// Package auth contains authentication types and interfaces.
package auth

import "time"

// Principal represents an authenticated administrator.
type Principal struct {
	ID        int64
	Username  string
	Roles     []string
	CreatedAt time.Time
}

// Authenticated reports whether the principal has the minimum identity fields.
func (p Principal) Authenticated() bool {
	return p.ID > 0 && p.Username != ""
}
