package server

import (
	"net/http"
	"strconv"
)

// handleSystemHealth returns the current system health snapshot.
func (a *App) handleSystemHealth(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	result, err := a.healthService.SystemHealth(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleLogTail returns the tail of a configured log file.
func (a *App) handleLogTail(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}

	source := r.URL.Query().Get("source")
	if source != "ocserv" && source != "panel" {
		writeError(w, http.StatusBadRequest, "source must be ocserv or panel")
		return
	}

	lines := 25
	if raw := r.URL.Query().Get("lines"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "lines must be a positive integer")
			return
		}
		lines = n
	}

	result, err := a.logTailService.TailLog(r.Context(), principal, source, lines)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
