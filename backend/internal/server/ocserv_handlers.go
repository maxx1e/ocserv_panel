package server

import (
	"net/http"
	"time"

	"ocserv_panel/backend/internal/ocserv"
)

// ocserv API response types

type ocservReadinessResponse struct {
	BinaryPresent   bool   `json:"binaryPresent"`
	BinaryVersion   string `json:"binaryVersion"`
	ConfigPresent   bool   `json:"configPresent"`
	ConfigValid     bool   `json:"configValid"`
	ServiceRunning  bool   `json:"serviceRunning"`
	SocketPresent   bool   `json:"socketPresent"`
	OcctlPresent    bool   `json:"occtlPresent"`
	OcpasswdPresent bool   `json:"ocpasswdPresent"`
}

type ocservServiceStatusResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ocservSessionResponse struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	RemoteAddress  string `json:"remoteAddress"`
	ConnectedAt    string `json:"connectedAt,omitempty"`
	VPNAddress     string `json:"vpnAddress"`
	RxBytes        int64  `json:"rxBytes"`
	TxBytes        int64  `json:"txBytes"`
	UserAgent      string `json:"userAgent"`
	Device         string `json:"device"`
	ReconnectCount int    `json:"reconnectCount"`
}

type ocservSessionDetailResponse struct {
	ID             string   `json:"id"`
	Username       string   `json:"username"`
	RemoteAddress  string   `json:"remoteAddress"`
	VPNAddress     string   `json:"vpnAddress"`
	ConnectedAt    string   `json:"connectedAt,omitempty"`
	DurationSecs   int64    `json:"durationSeconds"`
	RxBytes        int64    `json:"rxBytes"`
	TxBytes        int64    `json:"txBytes"`
	UserAgent      string   `json:"userAgent"`
	Device         string   `json:"device"`
	IPv4           string   `json:"ipv4"`
	IPv6           string   `json:"ipv6"`
	Routes         []string `json:"routes"`
	DNS            []string `json:"dns"`
	ReconnectCount int      `json:"reconnectCount"`
}

type ocservUserResponse struct {
	Username string `json:"username"`
	Group    string `json:"group"`
	Active   bool   `json:"active"`
}

type ocservConfigIssueResponse struct {
	Severity   string `json:"severity"`
	Key        string `json:"key"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
}

type disconnectSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type terminateSessionRequest struct {
	SessionID string `json:"sessionId"`
}

type createOcservUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type updateOcservUserPasswordRequest struct {
	Password string `json:"password"`
}

// handleOcservReadiness returns the current ocserv installation and runtime readiness.
func (a *App) handleOcservReadiness(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	readiness, err := a.ocservService.Readiness(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ocservReadinessToResponse(readiness))
}

// handleOcservServiceStatus returns the current ocserv service status.
func (a *App) handleOcservServiceStatus(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	status, err := a.ocservService.ServiceStatus(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ocservServiceStatusResponse{
		Status:  serviceStatusString(status),
		Message: serviceStatusMessage(status),
	})
}

// handleOcservServiceStart starts the ocserv service.
func (a *App) handleOcservServiceStart(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	if err := a.ocservService.StartService(r.Context(), principal); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleOcservServiceStop stops the ocserv service.
func (a *App) handleOcservServiceStop(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	if err := a.ocservService.StopService(r.Context(), principal); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleOcservServiceRestart restarts the ocserv service.
func (a *App) handleOcservServiceRestart(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	if err := a.ocservService.RestartService(r.Context(), principal); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleOcservSessions lists active ocserv sessions.
func (a *App) handleOcservSessions(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	sessions, err := a.ocservService.ListSessions(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	resp := make([]ocservSessionResponse, 0, len(sessions))
	for _, s := range sessions {
		resp = append(resp, ocservSessionToResponse(s))
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleOcservSessionDisconnect disconnects a session by ID.
func (a *App) handleOcservSessionDisconnect(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	var req disconnectSessionRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	if err := a.ocservService.DisconnectSession(r.Context(), principal, req.SessionID); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleOcservSessionDetail returns full detail for a single session.
func (a *App) handleOcservSessionDetail(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	detail, err := a.ocservService.GetSessionDetail(r.Context(), principal, id)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ocservSessionDetailToResponse(detail))
}

// handleOcservSessionTerminate terminates a session by ID.
func (a *App) handleOcservSessionTerminate(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	var req terminateSessionRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	if err := a.ocservService.TerminateSession(r.Context(), principal, req.SessionID); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleOcservUsers lists ocserv password-file users.
func (a *App) handleOcservUsers(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	users, err := a.ocservService.ListOcservUsers(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	resp := make([]ocservUserResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, ocservUserToResponse(u))
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleOcservUserCreate creates a new ocserv user.
func (a *App) handleOcservUserCreate(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	var req createOcservUserRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if err := a.ocservService.CreateOcservUser(r.Context(), principal, req.Username, req.Password); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleOcservUserDelete deletes an ocserv user.
func (a *App) handleOcservUserDelete(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodDelete) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	username := r.PathValue("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	if err := a.ocservService.DeleteOcservUser(r.Context(), principal, username); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleOcservUserUpdatePassword updates an ocserv user's password.
func (a *App) handleOcservUserUpdatePassword(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPut) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	username := r.PathValue("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	var req updateOcservUserPasswordRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}
	if err := a.ocservService.UpdateOcservUserPassword(r.Context(), principal, username, req.Password); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleOcservConfigIssues returns ocserv configuration validation issues.
func (a *App) handleOcservConfigIssues(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	issues, err := a.ocservService.ConfigIssues(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	resp := make([]ocservConfigIssueResponse, 0, len(issues))
	for _, issue := range issues {
		resp = append(resp, ocservConfigIssueResponse{
			Severity:   string(issue.Severity),
			Key:        issue.Key,
			Message:    issue.Message,
			Suggestion: issue.Suggestion,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// mapping helpers

func ocservReadinessToResponse(r *ocserv.OcservReadiness) ocservReadinessResponse {
	if r == nil {
		return ocservReadinessResponse{}
	}
	return ocservReadinessResponse{
		BinaryPresent:   r.BinaryPresent,
		BinaryVersion:   r.BinaryVersion,
		ConfigPresent:   r.ConfigPresent,
		ConfigValid:     r.ConfigValid,
		ServiceRunning:  r.ServiceRunning,
		SocketPresent:   r.SocketPresent,
		OcctlPresent:    r.OcctlPresent,
		OcpasswdPresent: r.OcpasswdPresent,
	}
}

func ocservSessionToResponse(s ocserv.Session) ocservSessionResponse {
	connectedAt := ""
	if !s.ConnectedAt.IsZero() {
		connectedAt = s.ConnectedAt.UTC().Format(time.RFC3339)
	}
	return ocservSessionResponse{
		ID:             s.ID,
		Username:       s.Username,
		RemoteAddress:  s.RemoteAddress,
		ConnectedAt:    connectedAt,
		VPNAddress:     s.VPNAddress,
		RxBytes:        s.RxBytes,
		TxBytes:        s.TxBytes,
		UserAgent:      s.UserAgent,
		Device:         s.Device,
		ReconnectCount: s.ReconnectCount,
	}
}

func ocservSessionDetailToResponse(d *ocserv.SessionDetail) ocservSessionDetailResponse {
	if d == nil {
		return ocservSessionDetailResponse{
			Routes: []string{},
			DNS:    []string{},
		}
	}
	connectedAt := ""
	if !d.ConnectedAt.IsZero() {
		connectedAt = d.ConnectedAt.UTC().Format(time.RFC3339)
	}
	routes := d.Routes
	if routes == nil {
		routes = []string{}
	}
	dns := d.DNS
	if dns == nil {
		dns = []string{}
	}
	return ocservSessionDetailResponse{
		ID:             d.ID,
		Username:       d.Username,
		RemoteAddress:  d.RemoteAddress,
		VPNAddress:     d.VPNAddress,
		ConnectedAt:    connectedAt,
		DurationSecs:   int64(d.Duration.Seconds()),
		RxBytes:        d.RxBytes,
		TxBytes:        d.TxBytes,
		UserAgent:      d.UserAgent,
		Device:         d.Device,
		IPv4:           d.IPv4,
		IPv6:           d.IPv6,
		Routes:         routes,
		DNS:            dns,
		ReconnectCount: d.ReconnectCount,
	}
}

func ocservUserToResponse(u ocserv.PasswdUser) ocservUserResponse {
	return ocservUserResponse{
		Username: u.Username,
		Group:    u.Group,
		Active:   u.Active,
	}
}

func serviceStatusString(s ocserv.ServiceStatus) string {
	switch s {
	case ocserv.ServiceActive:
		return "active"
	case ocserv.ServiceInactive:
		return "inactive"
	case ocserv.ServiceFailed:
		return "failed"
	default:
		return "not_found"
	}
}

func serviceStatusMessage(s ocserv.ServiceStatus) string {
	switch s {
	case ocserv.ServiceActive:
		return "ocserv service is running"
	case ocserv.ServiceInactive:
		return "ocserv service is stopped"
	case ocserv.ServiceFailed:
		return "ocserv service has failed"
	default:
		return "ocserv service unit not found"
	}
}
