package server

import (
	"net/http"

	"ocserv_panel/backend/internal/app"
)

type bulkAdminIDsRequest struct {
	IDs []int64 `json:"ids"`
}

type bulkVpnUsernamesRequest struct {
	Usernames []string `json:"usernames"`
}

func (a *App) handleAdminUsersBulkDelete(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	var req bulkAdminIDsRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids array is required and must not be empty")
		return
	}
	if len(req.IDs) > app.MaxBulkBatchSize {
		writeError(w, http.StatusBadRequest, "batch size exceeds maximum of 100")
		return
	}
	result := a.adminUserService.BulkDeleteAdminUsers(r.Context(), principal, req.IDs)
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleAdminUsersBulkDisable(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	var req bulkAdminIDsRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids array is required and must not be empty")
		return
	}
	if len(req.IDs) > app.MaxBulkBatchSize {
		writeError(w, http.StatusBadRequest, "batch size exceeds maximum of 100")
		return
	}
	result := a.adminUserService.BulkDisableAdminUsers(r.Context(), principal, req.IDs)
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleAdminUsersBulkEnable(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	var req bulkAdminIDsRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids array is required and must not be empty")
		return
	}
	if len(req.IDs) > app.MaxBulkBatchSize {
		writeError(w, http.StatusBadRequest, "batch size exceeds maximum of 100")
		return
	}
	result := a.adminUserService.BulkEnableAdminUsers(r.Context(), principal, req.IDs)
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleOcservUsersBulkDelete(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	var req bulkVpnUsernamesRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if len(req.Usernames) == 0 {
		writeError(w, http.StatusBadRequest, "usernames array is required and must not be empty")
		return
	}
	if len(req.Usernames) > app.MaxBulkBatchSize {
		writeError(w, http.StatusBadRequest, "batch size exceeds maximum of 100")
		return
	}
	result := a.ocservService.BulkDeleteOcservUsers(r.Context(), principal, req.Usernames)
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleOcservUsersBulkLock(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	var req bulkVpnUsernamesRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if len(req.Usernames) == 0 {
		writeError(w, http.StatusBadRequest, "usernames array is required and must not be empty")
		return
	}
	if len(req.Usernames) > app.MaxBulkBatchSize {
		writeError(w, http.StatusBadRequest, "batch size exceeds maximum of 100")
		return
	}
	result := a.ocservService.BulkLockOcservUsers(r.Context(), principal, req.Usernames)
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleOcservUsersBulkUnlock(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	var req bulkVpnUsernamesRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if len(req.Usernames) == 0 {
		writeError(w, http.StatusBadRequest, "usernames array is required and must not be empty")
		return
	}
	if len(req.Usernames) > app.MaxBulkBatchSize {
		writeError(w, http.StatusBadRequest, "batch size exceeds maximum of 100")
		return
	}
	result := a.ocservService.BulkUnlockOcservUsers(r.Context(), principal, req.Usernames)
	writeJSON(w, http.StatusOK, result)
}
