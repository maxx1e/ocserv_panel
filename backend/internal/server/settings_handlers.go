package server

import (
	"net/http"

	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/rbac"
)

func (a *App) handleSettingsSectionsGet(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	result, err := a.settingsService.GetAllSections(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPut) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	key := r.URL.Path[len("/api/settings/"):]
	if key == "" {
		writeError(w, http.StatusBadRequest, "setting key is required")
		return
	}
	var req updateSettingRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	setting, err := a.settingsService.UpdateSetting(r.Context(), principal, key, req.Value)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, setting)
}

func (a *App) handleRestartStatus(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	required, pending, err := a.settingsService.RestartStatus(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	if pending == nil {
		pending = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"restart_required": required,
		"pending_changes":  pending,
	})
}

func (a *App) handleRestart(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	if !a.authorizer.Allowed(principalRoles(principal), rbac.PermissionSettingsRestart) {
		writeError(w, http.StatusForbidden, "permission denied")
		return
	}
	// Placeholder: actual restart will be wired in a later task.
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "restart not yet implemented",
	})
}

// principalRoles extracts roles from a principal for RBAC checks.
func principalRoles(p auth.Principal) []rbac.Role {
	roles := make([]rbac.Role, len(p.Roles))
	for i, r := range p.Roles {
		roles[i] = rbac.Role(r)
	}
	return roles
}