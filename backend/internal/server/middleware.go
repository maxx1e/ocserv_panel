package server

import (
	"context"
	"net/http"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/rbac"
)

type contextKey string

const principalKey contextKey = "principal"

// requirePermission returns middleware that validates the session token, checks
// the given permission, and injects the principal into context.
// On failure it writes 401/403 and stops the chain.
func (a *App) requirePermission(perm rbac.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := a.authenticate(w, r)
			if !ok {
				return
			}
			if !a.authorizer.Allowed(principalRoles(principal), perm) {
				writeError(w, http.StatusForbidden, "permission denied")
				return
			}
			ctx := context.WithValue(r.Context(), principalKey, principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// principalFromContext retrieves the principal injected by requirePermission.
// Only call this inside a handler wrapped by requirePermission.
func principalFromContext(ctx context.Context) auth.Principal {
	p, _ := ctx.Value(principalKey).(auth.Principal)
	return p
}
