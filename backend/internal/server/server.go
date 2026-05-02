package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/metrics"
	"ocserv_panel/backend/internal/store"
)

// SessionCookieName is the only cookie consulted for API authentication.
const SessionCookieName = "ocserv_panel_session"

const maxJSONBodyBytes = 1 << 20

// HealthResponse is returned by the health endpoint.
type HealthResponse struct {
	Status string `json:"status"`
}

// NewRouter builds the backend HTTP router.
func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/api/health", handleHealthz)

	return securityHeaders(mux)
}

// App wires HTTP handlers to application services.
type App struct {
	authService       *app.AuthService
	dashboardService  *app.DashboardService
	monitoringService *app.MonitoringService
	settingsService   *app.SettingsService
	cookieSecure      bool
	now               func() time.Time
}

// AppOptions configures NewAppRouter.
type AppOptions struct {
	CookieSecure bool
	Now          func() time.Time
}

type apiError struct {
	Error string `json:"error"`
}

type setupStatusResponse struct {
	Required bool `json:"required"`
}

type credentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type purgeHistoryRequest struct {
	RetentionDays int    `json:"retentionDays"`
	Confirmation  string `json:"confirmation"`
}

type webUISettingsResponse struct {
	CookieSecure     bool   `json:"cookieSecure"`
	HTTPLoginAllowed bool   `json:"httpLoginAllowed"`
	RequestSecure    bool   `json:"requestSecure"`
	Transport        string `json:"transport"`
	Warning          string `json:"warning"`
}

type principalResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"createdAt"`
}

type sessionResponse struct {
	Principal principalResponse `json:"principal"`
	ID        int64             `json:"id"`
	Username  string            `json:"username"`
	Roles     []string          `json:"roles"`
	ExpiresAt time.Time         `json:"expiresAt,omitempty"`
}

type logoutResponse struct {
	OK bool `json:"ok"`
}

type dashboardSummaryResponse struct {
	BackendStatus      string                   `json:"backendStatus"`
	OcservStatus       string                   `json:"ocservStatus"`
	ActiveSessionCount int                      `json:"activeSessionCount"`
	RecentAudit        []auditEventResponse     `json:"recentAudit"`
	Analytics          analyticsSummaryResponse `json:"analytics"`
	SampleData         bool                     `json:"sampleData"`
	Ocserv             ocservStatusResponse     `json:"ocserv"`
	SetupRequired      bool                     `json:"setupRequired"`
	AdminCount         int64                    `json:"adminCount"`
	History            analyticsSummaryResponse `json:"history"`
}

type ocservStatusResponse struct {
	Status     string `json:"status"`
	Configured bool   `json:"configured"`
	Message    string `json:"message"`
}

type analyticsSummaryResponse struct {
	TotalSessions  int64 `json:"totalSessions"`
	SampleSessions int64 `json:"sampleSessions"`
	TotalBytesIn   int64 `json:"totalBytesIn"`
	TotalBytesOut  int64 `json:"totalBytesOut"`
}

type auditEventResponse struct {
	ID               int64             `json:"id"`
	ActorAdminUserID *int64            `json:"actorAdminUserId,omitempty"`
	Action           string            `json:"action"`
	ResourceType     string            `json:"resourceType"`
	ResourceID       string            `json:"resourceId"`
	Outcome          string            `json:"outcome"`
	Message          string            `json:"message"`
	Metadata         map[string]string `json:"metadata"`
	CreatedAt        time.Time         `json:"createdAt"`
}

type historyAnalyticsResponse struct {
	Summary        analyticsSummaryResponse    `json:"summary"`
	RecentSessions []historicalSessionResponse `json:"recentSessions"`
}

type historicalSessionResponse struct {
	ID              int64     `json:"id"`
	Username        string    `json:"username"`
	ConnectedAt     time.Time `json:"connectedAt"`
	DisconnectedAt  time.Time `json:"disconnectedAt"`
	DurationSeconds int64     `json:"durationSeconds"`
	BytesIn         int64     `json:"bytesIn"`
	BytesOut        int64     `json:"bytesOut"`
	Source          string    `json:"source"`
}

type settingsListResponse struct {
	Settings []settingResponse `json:"settings"`
}

type settingResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type updateSettingRequest struct {
	Value string `json:"value"`
}

type purgeHistoryResponse struct {
	RetentionDays int       `json:"retentionDays"`
	Cutoff        time.Time `json:"cutoff"`
	DeletedRows   int64     `json:"deletedRows"`
}

// NewAppRouter builds the backend HTTP API router.
func NewAppRouter(
	authService *app.AuthService,
	dashboardService *app.DashboardService,
	monitoringService *app.MonitoringService,
	settingsService *app.SettingsService,
	opts AppOptions,
) http.Handler {
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	appHandler := &App{
		authService:       authService,
		dashboardService:  dashboardService,
		monitoringService: monitoringService,
		settingsService:   settingsService,
		cookieSecure:      opts.CookieSecure,
		now:               now,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/api/health", handleHealthz)
	mux.HandleFunc("/api/setup/status", appHandler.handleSetupStatus)
	mux.HandleFunc("/api/setup/owner", appHandler.handleSetupOwner)
	mux.HandleFunc("/api/settings/webui", appHandler.handleWebUISettings)
	mux.HandleFunc("/api/auth/login", appHandler.handleLogin)
	mux.HandleFunc("/api/auth/logout", appHandler.handleLogout)
	mux.HandleFunc("/api/auth/me", appHandler.handleMe)
	mux.HandleFunc("/api/dashboard/summary", appHandler.handleDashboardSummary)
	mux.HandleFunc("/api/history/analytics", appHandler.handleHistoryAnalytics)
	mux.HandleFunc("/api/history/purge", appHandler.handleHistoryPurge)
	mux.HandleFunc("/api/monitoring/catalog", appHandler.handleMonitoringCatalog)
	mux.HandleFunc("/api/monitoring/overview", appHandler.handleMonitoringOverview)
	mux.HandleFunc("/api/monitoring/series", appHandler.handleMonitoringSeries)
	mux.HandleFunc("/api/settings", appHandler.handleSettingsGet)
	mux.HandleFunc("/api/settings/", appHandler.handleSettingsPut)

	return securityHeaders(mux)
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

func (a *App) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	required, err := a.authService.SetupRequired(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, setupStatusResponse{Required: required})
}

func (a *App) handleSetupOwner(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	var req credentialsRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required")
		return
	}
	if _, err := a.authService.CreateFirstOwner(r.Context(), req.Username, req.Password); err != nil {
		a.writeAppError(w, err)
		return
	}
	login, err := a.authService.Login(r.Context(), req.Username, req.Password, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	a.setSessionCookie(w, login.Token, login.ExpiresAt)
	writeJSON(w, http.StatusOK, sessionFromLogin(login))
}

func (a *App) handleWebUISettings(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	requestSecure := requestIsSecure(r)
	transport := "http"
	if requestSecure {
		transport = "https"
	}
	httpLoginAllowed := !a.cookieSecure

	writeJSON(w, http.StatusOK, webUISettingsResponse{
		CookieSecure:     a.cookieSecure,
		HTTPLoginAllowed: httpLoginAllowed,
		RequestSecure:    requestSecure,
		Transport:        transport,
		Warning:          webUITransportWarning(httpLoginAllowed, requestSecure),
	})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	var req credentialsRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	login, err := a.authService.Login(r.Context(), req.Username, req.Password, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		if errors.Is(err, app.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid username or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	a.setSessionCookie(w, login.Token, login.ExpiresAt)
	writeJSON(w, http.StatusOK, sessionFromLogin(login))
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	token := sessionTokenFromCookie(r)
	if err := a.authService.Logout(r.Context(), token); err != nil {
		a.clearSessionCookie(w)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	a.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, logoutResponse{OK: true})
}

func (a *App) handleMe(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, principalToSessionResponse(principal, time.Time{}))
}

func (a *App) handleDashboardSummary(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.dashboardService.Summary(r.Context(), auth.Principal{})
		return
	}
	summary, err := a.dashboardService.Summary(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dashboardSummaryToResponse(summary))
}

func (a *App) handleHistoryAnalytics(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.dashboardService.HistoryAnalytics(r.Context(), auth.Principal{})
		return
	}
	analytics, err := a.dashboardService.HistoryAnalytics(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, historyAnalyticsToResponse(analytics))
}

func (a *App) handleHistoryPurge(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.dashboardService.PurgeHistory(r.Context(), auth.Principal{}, 0, "")
		return
	}
	var req purgeHistoryRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	result, err := a.dashboardService.PurgeHistory(r.Context(), principal, req.RetentionDays, req.Confirmation)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, purgeHistoryResponse{RetentionDays: result.RetentionDays, Cutoff: result.Cutoff, DeletedRows: result.DeletedRows})
}

func (a *App) handleMonitoringCatalog(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.monitoringService.Catalog(r.Context(), auth.Principal{})
		return
	}
	catalog, err := a.monitoringService.Catalog(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, catalog)
}

func (a *App) handleMonitoringOverview(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.monitoringService.Overview(r.Context(), auth.Principal{}, queryValueDefault(r, "window", "1h"))
		return
	}
	window := queryValueDefault(r, "window", "1h")
	overview, err := a.monitoringService.Overview(r.Context(), principal, window)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (a *App) handleMonitoringSeries(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		_, _ = a.monitoringService.Series(
			r.Context(),
			auth.Principal{},
			metrics.MetricName(r.URL.Query().Get("metric")),
			queryValueDefault(r, "window", "1h"),
			queryValueDefault(r, "step", "60s"),
		)
		return
	}
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	window := queryValueDefault(r, "window", "1h")
	step := queryValueDefault(r, "step", "60s")
	series, err := a.monitoringService.Series(r.Context(), principal, metrics.MetricName(metric), window, step)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, series)
}

func (a *App) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	settings, err := a.settingsService.GetAll(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	response := make([]settingResponse, 0, len(settings))
	for _, s := range settings {
		response = append(response, settingResponse{Key: s.Key, Value: s.Value})
	}
	writeJSON(w, http.StatusOK, settingsListResponse{Settings: response})
}

func (a *App) handleSettingsPut(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPut) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
		return
	}
	// Extract key from path: /api/settings/{key}
	key := r.URL.Path[len("/api/settings/"):]
	if key == "" {
		writeError(w, http.StatusBadRequest, "setting key is required")
		return
	}
	var req updateSettingRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	switch key {
	case "history.retention_days":
		days, err := strconv.Atoi(req.Value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "retention days must be a number")
			return
		}
		if err := a.settingsService.SetRetentionDays(r.Context(), principal, days); err != nil {
			a.writeAppError(w, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unknown setting key")
		return
	}
	writeJSON(w, http.StatusOK, settingResponse{Key: key, Value: req.Value})
}

func (a *App) authenticate(w http.ResponseWriter, r *http.Request) (auth.Principal, bool) {
	validation, err := a.authService.ValidateSession(r.Context(), sessionTokenFromCookie(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return auth.Principal{}, false
	}
	if !validation.Authenticated {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return auth.Principal{}, false
	}
	return validation.Principal, true
}

func (a *App) writeAppError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, app.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid request")
	case errors.Is(err, app.ErrSetupAlreadyComplete):
		writeError(w, http.StatusConflict, "setup already complete")
	case errors.Is(err, app.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "authentication required")
	case errors.Is(err, app.ErrPermissionDenied):
		writeError(w, http.StatusForbidden, "permission denied")
	case errors.Is(err, app.ErrInvalidConfirmation):
		writeError(w, http.StatusBadRequest, "invalid confirmation")
	case errors.Is(err, app.ErrInvalidRetention):
		writeError(w, http.StatusBadRequest, "invalid retention")
	case isMetricsValidationError(err):
		writeError(w, http.StatusBadRequest, "invalid request")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func isMetricsValidationError(err error) bool {
	return errors.Is(err, metrics.ErrInvalidMetric) ||
		errors.Is(err, metrics.ErrInvalidWindow) ||
		errors.Is(err, metrics.ErrInvalidStep) ||
		errors.Is(err, metrics.ErrInvalidRange)
}

func queryValueDefault(r *http.Request, key string, fallback string) string {
	value := r.URL.Query().Get(key)
	if value == "" {
		return fallback
	}
	return value
}

func (a *App) setSessionCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *App) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  a.now().Add(-time.Hour),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(value); err != nil {
		// The response is already committed. Future structured logging can record this.
		return
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{Error: message})
}

func allowMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	w.Header().Set("Allow", method)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	return false
}

func requestIsSecure(r *http.Request) bool {
	return r.TLS != nil
}

func webUITransportWarning(httpLoginAllowed bool, requestSecure bool) string {
	if httpLoginAllowed {
		return "HTTP login sends credentials/session cookies without transport encryption. Use only on trusted local networks or for testing."
	}
	if !requestSecure {
		return "HTTP login is not enabled, but this request is not encrypted. Use HTTPS before entering credentials on untrusted networks."
	}
	return ""
}

func decodeRequestJSON(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return false
	}
	return true
}

func sessionTokenFromCookie(r *http.Request) string {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func sessionFromLogin(login app.LoginResult) sessionResponse {
	return principalToSessionResponse(login.Principal, login.ExpiresAt)
}

func principalToSessionResponse(principal auth.Principal, expiresAt time.Time) sessionResponse {
	principalBody := principalResponse{
		ID:        principal.ID,
		Username:  principal.Username,
		Roles:     append([]string{}, principal.Roles...),
		CreatedAt: principal.CreatedAt,
	}
	return sessionResponse{
		Principal: principalBody,
		ID:        principalBody.ID,
		Username:  principalBody.Username,
		Roles:     principalBody.Roles,
		ExpiresAt: expiresAt,
	}
}

func dashboardSummaryToResponse(summary app.DashboardSummary) dashboardSummaryResponse {
	return dashboardSummaryResponse{
		BackendStatus:      summary.BackendStatus,
		OcservStatus:       summary.Ocserv.Status,
		ActiveSessionCount: summary.ActiveSessionCount,
		RecentAudit:        auditEventsToResponse(summary.RecentAudit),
		Analytics:          analyticsSummaryToResponse(summary.Analytics),
		SampleData:         summary.SampleData,
		Ocserv: ocservStatusResponse{
			Status:     summary.Ocserv.Status,
			Configured: summary.Ocserv.Configured,
			Message:    summary.Ocserv.Message,
		},
		SetupRequired: summary.SetupRequired,
		AdminCount:    summary.AdminCount,
		History:       analyticsSummaryToResponse(summary.History),
	}
}

func auditEventsToResponse(events []store.AuditEvent) []auditEventResponse {
	recent := make([]auditEventResponse, 0, len(events))
	for _, event := range events {
		metadata := map[string]string{}
		if event.Metadata != nil {
			metadata = event.Metadata
		}
		recent = append(recent, auditEventResponse{
			ID:               event.ID,
			ActorAdminUserID: event.ActorAdminUserID,
			Action:           event.Action,
			ResourceType:     event.ResourceType,
			ResourceID:       event.ResourceID,
			Outcome:          event.Outcome,
			Message:          event.Message,
			Metadata:         metadata,
			CreatedAt:        event.CreatedAt,
		})
	}
	return recent
}

func historyAnalyticsToResponse(result app.HistoryAnalyticsResult) historyAnalyticsResponse {
	recent := make([]historicalSessionResponse, 0, len(result.RecentSessions))
	for _, session := range result.RecentSessions {
		recent = append(recent, historicalSessionResponse{
			ID:              session.ID,
			Username:        session.Username,
			ConnectedAt:     session.ConnectedAt,
			DisconnectedAt:  session.DisconnectedAt,
			DurationSeconds: session.DurationSeconds,
			BytesIn:         session.BytesIn,
			BytesOut:        session.BytesOut,
			Source:          session.Source,
		})
	}
	return historyAnalyticsResponse{Summary: analyticsSummaryToResponse(result.Summary), RecentSessions: recent}
}

func analyticsSummaryToResponse(summary store.AnalyticsSummary) analyticsSummaryResponse {
	return analyticsSummaryResponse{
		TotalSessions:  summary.TotalSessions,
		SampleSessions: summary.SampleSessions,
		TotalBytesIn:   summary.TotalBytesIn,
		TotalBytesOut:  summary.TotalBytesOut,
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
