package server

import (
	"bytes"
	"net/http"
	"os"
	"strconv"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/rbac"
)

const logTailMaxLines = 50
const logTailDefaultLines = 25

// handleSystemHealth returns the current system health snapshot.
func (a *App) handleSystemHealth(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	if !a.authorizer.Allowed(principalRoles(principal), rbac.PermissionReadDashboard) {
		writeError(w, http.StatusForbidden, "permission denied")
		return
	}

	cfg := a.cfgFn()

	vpn := app.BuildVPNHealth(r.Context(), a.readiness)
	panel := app.BuildPanelHealth(r.Context(), a.adminCounter, cfg)
	logs := app.BuildLogsHealth(cfg)
	storage := app.BuildStorageHealth(cfg, a.tsdbPath)

	writeJSON(w, http.StatusOK, app.SystemHealthResult{
		VPNServer: vpn,
		Panel:     panel,
		Logs:      logs,
		Storage:   storage,
	})
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
	if !a.authorizer.Allowed(principalRoles(principal), rbac.PermissionReadDashboard) {
		writeError(w, http.StatusForbidden, "permission denied")
		return
	}

	source := r.URL.Query().Get("source")
	if source != "ocserv" && source != "panel" {
		writeError(w, http.StatusBadRequest, "source must be ocserv or panel")
		return
	}

	lines := logTailDefaultLines
	if raw := r.URL.Query().Get("lines"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "lines must be a positive integer")
			return
		}
		lines = n
	}
	if lines > logTailMaxLines {
		lines = logTailMaxLines
	}

	cfg := a.cfgFn()

	var path string
	switch source {
	case "ocserv":
		path = cfg.Installation.OcservLogPath
	case "panel":
		path = cfg.Installation.PanelLogPath
	}

	enabled := cfg.Monitoring.LogFileReadEnabled
	if !enabled {
		writeJSON(w, http.StatusOK, app.LogTailResult{
			Source:  source,
			Enabled: false,
			Present: false,
			Path:    path,
			Lines:   []string{},
		})
		return
	}

	data, err := a.logFileReader.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, app.LogTailResult{
				Source:  source,
				Enabled: true,
				Present: false,
				Path:    path,
				Lines:   []string{},
			})
			return
		}
		writeJSON(w, http.StatusOK, app.LogTailResult{
			Source:  source,
			Enabled: true,
			Present: false,
			Path:    path,
			Lines:   []string{},
			Error:   "log file could not be read",
		})
		return
	}

	tail := tailLines(data, lines)
	writeJSON(w, http.StatusOK, app.LogTailResult{
		Source:  source,
		Enabled: true,
		Present: true,
		Path:    path,
		Lines:   tail,
	})
}

// tailLines returns the last n lines from data.
func tailLines(data []byte, n int) []string {
	data = bytes.TrimRight(data, "\n\r")
	if len(data) == 0 {
		return []string{}
	}
	all := bytes.Split(data, []byte("\n"))
	if len(all) <= n {
		result := make([]string, 0, len(all))
		for _, l := range all {
			result = append(result, string(l))
		}
		return result
	}
	tail := all[len(all)-n:]
	result := make([]string, 0, len(tail))
	for _, l := range tail {
		result = append(result, string(l))
	}
	return result
}
