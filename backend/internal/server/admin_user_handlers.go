package server

import (
	"net/http"
	"strconv"
)

type createAdminUserRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}

type updateAdminUserRequest struct {
	DisplayName string  `json:"displayName"`
	Email       string  `json:"email"`
	Role        *string `json:"role"`
}

type resetAdminPasswordRequest struct {
	Password string `json:"password"`
}

func (a *App) handleAdminUsersList(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	users, err := a.adminUserService.ListAdminUsers(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (a *App) handleAdminUserCreate(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	var req createAdminUserRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if req.Role == "" {
		req.Role = "viewer"
	}
	user, err := a.adminUserService.CreateAdminUser(
		r.Context(),
		principal,
		req.Username,
		req.Password,
		req.DisplayName,
		req.Email,
		req.Role,
	)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (a *App) handleAdminUserUpdate(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req updateAdminUserRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if err := a.adminUserService.UpdateAdminUser(
		r.Context(),
		principal,
		id,
		req.DisplayName,
		req.Email,
		req.Role,
	); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleAdminUserDelete(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := a.adminUserService.DeleteAdminUser(r.Context(), principal, id); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleAdminUserDisable(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := a.adminUserService.DisableAdminUser(r.Context(), principal, id); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleAdminUserEnable(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	if err := a.adminUserService.EnableAdminUser(r.Context(), principal, id); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleAdminUserResetPassword(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	var req resetAdminPasswordRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	if err := a.adminUserService.ResetAdminPassword(r.Context(), principal, id, req.Password); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
