package server

import (
	"net/http"
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
	_, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	// Placeholder: config diffing not yet implemented.
	writeJSON(w, http.StatusOK, map[string]any{
		"restart_required": false,
		"pending_changes":  []string{},
	})
}

func (a *App) handleRestart(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	_, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	// Placeholder: actual restart will be wired in a later task.
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "restart not yet implemented",
	})
}
