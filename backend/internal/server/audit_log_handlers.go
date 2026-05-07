package server

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"ocserv_panel/backend/internal/app"
)

func (a *App) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}

	source := r.URL.Query().Get("source")
	if source == "" {
		writeError(w, http.StatusBadRequest, "source parameter is required")
		return
	}

	query, err := parseAuditLogQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, svcErr := a.auditLogService.ReadLogs(r.Context(), principal, source, query)
	if svcErr != nil {
		a.writeAppError(w, svcErr)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleAuditLogSources(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}

	sources, err := a.auditLogService.GetSources(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"sources": sources})
}

func (a *App) handleAuditLogExport(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}

	source := r.URL.Query().Get("source")
	if source == "" {
		writeError(w, http.StatusBadRequest, "source parameter is required")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	query, err := parseAuditLogQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	data, svcErr := a.auditLogService.ExportLogs(r.Context(), principal, source, query, format)
	if svcErr != nil {
		a.writeAppError(w, svcErr)
		return
	}

	contentType := "application/json"
	filename := "audit-logs-" + source + ".json"
	if format == "csv" {
		contentType = "text/csv"
		filename = "audit-logs-" + source + ".csv"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func parseAuditLogQuery(r *http.Request) (app.AuditLogQuery, error) {
	q := r.URL.Query()
	var query app.AuditLogQuery

	query.Search = q.Get("search")
	query.Level = q.Get("level")
	query.Order = q.Get("order")
	if query.Order == "" {
		query.Order = "desc"
	}

	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return query, fmt.Errorf("invalid limit: %s", v)
		}
		query.Limit = n
	}

	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return query, fmt.Errorf("invalid offset: %s", v)
		}
		query.Offset = n
	}

	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return query, fmt.Errorf("invalid from timestamp: %s", v)
		}
		query.From = t
	}

	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return query, fmt.Errorf("invalid to timestamp: %s", v)
		}
		query.To = t
	}

	return query, nil
}
