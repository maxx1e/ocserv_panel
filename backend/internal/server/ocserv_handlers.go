package server

import (
	"net/http"
	"strconv"
	"time"

	"ocserv_panel/backend/internal/ocserv"
	"ocserv_panel/backend/internal/ocserv/service"
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

type serverStatusResponse struct {
	Status               string `json:"status"`
	ServerPID            int    `json:"serverPid"`
	SecModPID            int    `json:"secModPid"`
	SecModInstances      int    `json:"secModInstances"`
	UpSince              string `json:"upSince,omitempty"`
	ActiveSessions       int    `json:"activeSessions"`
	TotalSessions        int    `json:"totalSessions"`
	TotalAuthFailures    int    `json:"totalAuthFailures"`
	BannedIPs            int    `json:"bannedIps"`
	LastStatsReset       string `json:"lastStatsReset,omitempty"`
	SessionsHandled      int    `json:"sessionsHandled"`
	TimedOutSessions     int    `json:"timedOutSessions"`
	IdleTimedOutSessions int    `json:"idleTimedOutSessions"`
	ErrorClosedSessions  int    `json:"errorClosedSessions"`
	AuthFailures         int    `json:"authFailures"`
	AvgAuthTimeSecs      int64  `json:"avgAuthTimeSeconds"`
	MaxAuthTimeSecs      int64  `json:"maxAuthTimeSeconds"`
	AvgSessionTimeSecs   int64  `json:"avgSessionTimeSeconds"`
	MaxSessionTimeSecs   int64  `json:"maxSessionTimeSeconds"`
	MinMTU               int    `json:"minMtu"`
	MaxMTU               int    `json:"maxMtu"`
	RxBytes              int64  `json:"rxBytes"`
	TxBytes              int64  `json:"txBytes"`
}

type connectedUserResponse struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Vhost        string `json:"vhost"`
	RemoteIP     string `json:"remoteIp"`
	VpnIP        string `json:"vpnIp"`
	Device       string `json:"device"`
	Since        string `json:"since"`
	DurationSecs int64  `json:"durationSeconds"`
	DTLSCipher   string `json:"dtlsCipher"`
	Status       string `json:"status"`
}

type userConnectionResponse struct {
	ID                 string   `json:"id"`
	Username           string   `json:"username"`
	Groupname          string   `json:"groupname"`
	State              string   `json:"state"`
	Vhost              string   `json:"vhost"`
	Device             string   `json:"device"`
	MTU                int      `json:"mtu"`
	RemoteIP           string   `json:"remoteIp"`
	Location           string   `json:"location"`
	LocalDeviceIP      string   `json:"localDeviceIp"`
	IPv4               string   `json:"ipv4"`
	PtPIPv4            string   `json:"ptpIpv4"`
	UserAgent          string   `json:"userAgent"`
	RxBytes            int64    `json:"rxBytes"`
	TxBytes            int64    `json:"txBytes"`
	AvgRxBandwidth     string   `json:"avgRxBandwidth"`
	AvgTxBandwidth     string   `json:"avgTxBandwidth"`
	DPD                int      `json:"dpd"`
	KeepAlive          int      `json:"keepAlive"`
	Hostname           string   `json:"hostname"`
	ConnectedAt        string   `json:"connectedAt,omitempty"`
	SessionID          string   `json:"sessionId"`
	TLSCipher          string   `json:"tlsCipher"`
	DTLSCipher         string   `json:"dtlsCipher"`
	DNS                []string `json:"dns"`
	Routes             []string `json:"routes"`
	RestrictedToRoutes bool     `json:"restrictedToRoutes"`
}

type connectionEventResponse struct {
	Timestamp string `json:"timestamp,omitempty"`
	Username  string `json:"username"`
	RemoteIP  string `json:"remoteIp"`
	EventType string `json:"eventType"`
	Message   string `json:"message"`
}

type userRouteResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Vhost    string `json:"vhost"`
	Device   string `json:"device"`
	VpnIP    string `json:"vpnIp"`
	Route    string `json:"route"`
}

type bannedIPResponse struct {
	IP     string `json:"ip"`
	Score  int    `json:"score"`
	Reason string `json:"reason"`
}

type banPointEntryResponse struct {
	IP     string `json:"ip"`
	Points int    `json:"points"`
	Since  string `json:"since,omitempty"`
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
	Group    string `json:"group"`
}

type updateOcservUserPasswordRequest struct {
	Password string `json:"password"`
}

type updateOcservUserGroupRequest struct {
	Group string `json:"group"`
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
	if err := a.ocservService.CreateOcservUser(r.Context(), principal, req.Username, req.Password, req.Group); err != nil {
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

// handleOcservUserUpdateGroup updates an ocserv user's group assignment.
func (a *App) handleOcservUserUpdateGroup(w http.ResponseWriter, r *http.Request) {
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
	var req updateOcservUserGroupRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if err := a.ocservService.UpdateOcservUserGroup(r.Context(), principal, username, req.Group); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleOcservUserLock locks (disables) an ocserv user account.
func (a *App) handleOcservUserLock(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	username := r.PathValue("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	if err := a.ocservService.LockOcservUser(r.Context(), principal, username); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleOcservUserUnlock unlocks (re-enables) an ocserv user account.
func (a *App) handleOcservUserUnlock(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	username := r.PathValue("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "username is required")
		return
	}
	if err := a.ocservService.UnlockOcservUser(r.Context(), principal, username); err != nil {
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

// handleOcservServerStatus returns the ocserv server status.
func (a *App) handleOcservServerStatus(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	status, err := a.ocservService.GetServerStatus(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, serverStatusToResponse(status))
}

// handleOcservConnectedUsers lists connected users.
func (a *App) handleOcservConnectedUsers(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	users, err := a.ocservService.ListConnectedUsers(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	resp := make([]connectedUserResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, connectedUserToResponse(u))
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleDashboardActiveSessions returns per-user session counts for the donut chart.
func (a *App) handleDashboardActiveSessions(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	result, err := a.ocservService.ActiveUserSessions(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleOcservUserConnections returns connections for a specific user.
func (a *App) handleOcservUserConnections(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
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
	conns, err := a.ocservService.GetUserConnections(r.Context(), principal, username)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	resp := make([]userConnectionResponse, 0, len(conns))
	for _, c := range conns {
		resp = append(resp, userConnectionToResponse(c))
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleOcservConnectionDetail returns full connection detail for a single connection ID.
func (a *App) handleOcservConnectionDetail(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	idStr := r.PathValue("id")
	if idStr == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id must be a positive integer")
		return
	}
	conn, err := a.ocservService.GetConnectionByID(r.Context(), principal, id)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, userConnectionToResponse(*conn))
}

// handleOcservEvents lists connection events.
func (a *App) handleOcservEvents(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	events, err := a.ocservService.ListEvents(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	resp := make([]connectionEventResponse, 0, len(events))
	for _, e := range events {
		resp = append(resp, connectionEventToResponse(e))
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleOcservRoutes lists user-provided routes.
func (a *App) handleOcservRoutes(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	routes, err := a.ocservService.ListRoutes(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	resp := make([]userRouteResponse, 0, len(routes))
	for _, rt := range routes {
		resp = append(resp, userRouteToResponse(rt))
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleOcservBans lists banned IPs.
func (a *App) handleOcservBans(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	bans, err := a.ocservService.ListBannedIPs(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	resp := make([]bannedIPResponse, 0, len(bans))
	for _, b := range bans {
		resp = append(resp, bannedIPToResponse(b))
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleOcservBanPoints lists IP ban points.
func (a *App) handleOcservBanPoints(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	points, err := a.ocservService.ListBanPoints(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	resp := make([]banPointEntryResponse, 0, len(points))
	for _, p := range points {
		resp = append(resp, banPointEntryToResponse(p))
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleOcservUserDisconnect disconnects all sessions for a user.
func (a *App) handleOcservUserDisconnect(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
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
	if err := a.ocservService.DisconnectUser(r.Context(), principal, username); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleOcservUnbanIP unbans an IP address.
func (a *App) handleOcservUnbanIP(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	ip := r.PathValue("ip")
	if ip == "" {
		writeError(w, http.StatusBadRequest, "ip is required")
		return
	}
	if err := a.ocservService.UnbanIP(r.Context(), principal, ip); err != nil {
		a.writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func serviceStatusString(s service.ServiceStatus) string {
	switch s {
	case service.ServiceActive:
		return "active"
	case service.ServiceInactive:
		return "inactive"
	case service.ServiceFailed:
		return "failed"
	default:
		return "not_found"
	}
}

func serviceStatusMessage(s service.ServiceStatus) string {
	switch s {
	case service.ServiceActive:
		return "ocserv service is running"
	case service.ServiceInactive:
		return "ocserv service is stopped"
	case service.ServiceFailed:
		return "ocserv service has failed"
	default:
		return "ocserv service unit not found"
	}
}

func serverStatusToResponse(s *ocserv.ServerStatus) serverStatusResponse {
	if s == nil {
		return serverStatusResponse{}
	}
	upSince := ""
	if !s.UpSince.IsZero() {
		upSince = s.UpSince.UTC().Format(time.RFC3339)
	}
	lastReset := ""
	if !s.LastStatsReset.IsZero() {
		lastReset = s.LastStatsReset.UTC().Format(time.RFC3339)
	}
	return serverStatusResponse{
		Status:               s.Status,
		ServerPID:            s.ServerPID,
		SecModPID:            s.SecModPID,
		SecModInstances:      s.SecModInstances,
		UpSince:              upSince,
		ActiveSessions:       s.ActiveSessions,
		TotalSessions:        s.TotalSessions,
		TotalAuthFailures:    s.TotalAuthFailures,
		BannedIPs:            s.BannedIPs,
		LastStatsReset:       lastReset,
		SessionsHandled:      s.SessionsHandled,
		TimedOutSessions:     s.TimedOutSessions,
		IdleTimedOutSessions: s.IdleTimedOutSessions,
		ErrorClosedSessions:  s.ErrorClosedSessions,
		AuthFailures:         s.AuthFailures,
		AvgAuthTimeSecs:      int64(s.AvgAuthTime.Seconds()),
		MaxAuthTimeSecs:      int64(s.MaxAuthTime.Seconds()),
		AvgSessionTimeSecs:   int64(s.AvgSessionTime.Seconds()),
		MaxSessionTimeSecs:   int64(s.MaxSessionTime.Seconds()),
		MinMTU:               s.MinMTU,
		MaxMTU:               s.MaxMTU,
		RxBytes:              s.RxBytes,
		TxBytes:              s.TxBytes,
	}
}

func connectedUserToResponse(u ocserv.ConnectedUser) connectedUserResponse {
	return connectedUserResponse{
		ID:           u.ID,
		Username:     u.Username,
		Vhost:        u.Vhost,
		RemoteIP:     u.RemoteIP,
		VpnIP:        u.VpnIP,
		Device:       u.Device,
		Since:        u.Since,
		DurationSecs: int64(u.Duration.Seconds()),
		DTLSCipher:   u.DTLSCipher,
		Status:       u.Status,
	}
}

func userConnectionToResponse(c ocserv.UserConnection) userConnectionResponse {
	connectedAt := ""
	if !c.ConnectedAt.IsZero() {
		connectedAt = c.ConnectedAt.UTC().Format(time.RFC3339)
	}
	dns := c.DNS
	if dns == nil {
		dns = []string{}
	}
	routes := c.Routes
	if routes == nil {
		routes = []string{}
	}
	return userConnectionResponse{
		ID:                 c.ID,
		Username:           c.Username,
		Groupname:          c.Groupname,
		State:              c.State,
		Vhost:              c.Vhost,
		Device:             c.Device,
		MTU:                c.MTU,
		RemoteIP:           c.RemoteIP,
		Location:           c.Location,
		LocalDeviceIP:      c.LocalDeviceIP,
		IPv4:               c.IPv4,
		PtPIPv4:            c.PtPIPv4,
		UserAgent:          c.UserAgent,
		RxBytes:            c.RxBytes,
		TxBytes:            c.TxBytes,
		AvgRxBandwidth:     c.AvgRxBandwidth,
		AvgTxBandwidth:     c.AvgTxBandwidth,
		DPD:                c.DPD,
		KeepAlive:          c.KeepAlive,
		Hostname:           c.Hostname,
		ConnectedAt:        connectedAt,
		SessionID:          c.SessionID,
		TLSCipher:          c.TLSCipher,
		DTLSCipher:         c.DTLSCipher,
		DNS:                dns,
		Routes:             routes,
		RestrictedToRoutes: c.RestrictedToRoutes,
	}
}

func connectionEventToResponse(e ocserv.ConnectionEvent) connectionEventResponse {
	ts := ""
	if !e.Timestamp.IsZero() {
		ts = e.Timestamp.UTC().Format(time.RFC3339)
	}
	return connectionEventResponse{
		Timestamp: ts,
		Username:  e.Username,
		RemoteIP:  e.RemoteIP,
		EventType: e.EventType,
		Message:   e.Message,
	}
}

func userRouteToResponse(r ocserv.UserRoute) userRouteResponse {
	return userRouteResponse{
		ID:       r.ID,
		Username: r.Username,
		Vhost:    r.Vhost,
		Device:   r.Device,
		VpnIP:    r.VpnIP,
		Route:    r.Route,
	}
}

func bannedIPToResponse(b ocserv.BannedIP) bannedIPResponse {
	return bannedIPResponse{
		IP:     b.IP,
		Score:  b.Score,
		Reason: b.Reason,
	}
}

func banPointEntryToResponse(p ocserv.BanPointEntry) banPointEntryResponse {
	since := ""
	if !p.Since.IsZero() {
		since = p.Since.UTC().Format(time.RFC3339)
	}
	return banPointEntryResponse{
		IP:     p.IP,
		Points: p.Points,
		Since:  since,
	}
}
