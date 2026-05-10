package server

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/metrics"
	"ocserv_panel/backend/internal/ocserv"
	"ocserv_panel/backend/internal/rbac"
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
	authService      *app.AuthService
	dashboardService *app.DashboardService
	settingsService  *app.SettingsService
	ocservService    *app.OcservService
	configService    *app.ConfigManagementService
	adminUserService *app.AdminUserService
	auditLogService  *app.AuditLogService
	authorizer       rbac.Authorizer
	loginLimiter     *auth.RateLimiter
	cookieSecure     bool
	refreshInterval  int
	now              func() time.Time

	// Direct dependencies for monitoring/health/log handlers (collapsed thin services).
	metricStore   metrics.Store
	metricCatalog *metrics.Catalog
	logFileReader app.LogFileReader
	cfgFn         func() config.Config
	adminCounter  app.AdminCounter
	readiness     *ocserv.ReadinessChecker
	tsdbPath      string
}

// AppOptions configures NewAppRouter.
type AppOptions struct {
	CookieSecure    bool
	LoginLimiter    *auth.RateLimiter
	Now             func() time.Time
	RefreshInterval int
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
	BackendStatus            string                   `json:"backendStatus"`
	OcservStatus             string                   `json:"ocservStatus"`
	ActiveSessionCount       int                      `json:"activeSessionCount"`
	RecentAudit              []auditEventResponse     `json:"recentAudit"`
	Analytics                analyticsSummaryResponse `json:"analytics"`
	SampleData               bool                     `json:"sampleData"`
	Ocserv                   ocservStatusResponse     `json:"ocserv"`
	SetupRequired            bool                     `json:"setupRequired"`
	AdminCount               int64                    `json:"adminCount"`
	History                  analyticsSummaryResponse `json:"history"`
	DashboardRefreshInterval int                      `json:"dashboardRefreshInterval"`
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
	metricStore metrics.Store,
	metricCatalog *metrics.Catalog,
	settingsService *app.SettingsService,
	ocservService *app.OcservService,
	logFileReader app.LogFileReader,
	cfgFn func() config.Config,
	adminCounter app.AdminCounter,
	readiness *ocserv.ReadinessChecker,
	tsdbPath string,
	configService *app.ConfigManagementService,
	adminUserService *app.AdminUserService,
	auditLogService *app.AuditLogService,
	authorizer rbac.Authorizer,
	opts AppOptions,
	staticDir string,
) http.Handler {
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	appHandler := &App{
		authService:      authService,
		dashboardService: dashboardService,
		settingsService:  settingsService,
		ocservService:    ocservService,
		configService:    configService,
		adminUserService: adminUserService,
		auditLogService:  auditLogService,
		authorizer:       authorizer,
		loginLimiter:     opts.LoginLimiter,
		cookieSecure:     opts.CookieSecure,
		refreshInterval:  opts.RefreshInterval,
		now:              now,
		metricStore:      metricStore,
		metricCatalog:    metricCatalog,
		logFileReader:    logFileReader,
		cfgFn:            cfgFn,
		adminCounter:     adminCounter,
		readiness:        readiness,
		tsdbPath:         tsdbPath,
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
	mux.HandleFunc("GET /api/dashboard/active-sessions", appHandler.handleDashboardActiveSessions)
	mux.HandleFunc("/api/history/analytics", appHandler.handleHistoryAnalytics)
	mux.HandleFunc("/api/history/purge", appHandler.handleHistoryPurge)
	mux.Handle("/api/monitoring/catalog", appHandler.requirePermission(rbac.PermissionReadDashboard)(http.HandlerFunc(appHandler.handleMonitoringCatalog)))
	mux.Handle("/api/monitoring/overview", appHandler.requirePermission(rbac.PermissionReadDashboard)(http.HandlerFunc(appHandler.handleMonitoringOverview)))
	mux.Handle("/api/monitoring/series", appHandler.requirePermission(rbac.PermissionReadDashboard)(http.HandlerFunc(appHandler.handleMonitoringSeries)))
	mux.Handle("GET /api/monitoring/system-info", appHandler.requirePermission(rbac.PermissionReadDashboard)(http.HandlerFunc(appHandler.handleMonitoringSystemInfo)))
	mux.HandleFunc("/api/settings/restart-status", appHandler.handleRestartStatus)
	mux.HandleFunc("/api/settings/restart", appHandler.handleRestart)
	mux.HandleFunc("/api/settings", appHandler.handleSettingsSectionsGet)
	mux.HandleFunc("/api/settings/", appHandler.handleSettingsUpdate)
	// ocserv management API
	mux.HandleFunc("/api/ocserv/readiness", appHandler.handleOcservReadiness)
	mux.HandleFunc("/api/ocserv/service/status", appHandler.handleOcservServiceStatus)
	mux.HandleFunc("/api/ocserv/service/start", appHandler.handleOcservServiceStart)
	mux.HandleFunc("/api/ocserv/service/stop", appHandler.handleOcservServiceStop)
	mux.HandleFunc("/api/ocserv/service/restart", appHandler.handleOcservServiceRestart)
	mux.HandleFunc("/api/ocserv/sessions", appHandler.handleOcservSessions)
	mux.HandleFunc("POST /api/ocserv/sessions/disconnect", appHandler.handleOcservSessionDisconnect)
	mux.HandleFunc("POST /api/ocserv/sessions/terminate", appHandler.handleOcservSessionTerminate)
	mux.HandleFunc("GET /api/ocserv/sessions/{id}", appHandler.handleOcservSessionDetail)
	mux.HandleFunc("/api/ocserv/users", appHandler.handleOcservUsers)
	mux.HandleFunc("/api/ocserv/users/create", appHandler.handleOcservUserCreate)
	mux.HandleFunc("/api/ocserv/users/{username}/password", appHandler.handleOcservUserUpdatePassword)
	mux.HandleFunc("PUT /api/ocserv/users/{username}/group", appHandler.handleOcservUserUpdateGroup)
	mux.HandleFunc("/api/ocserv/users/{username}", appHandler.handleOcservUserDelete)
	mux.HandleFunc("/api/ocserv/config/issues", appHandler.handleOcservConfigIssues)
	// Sessions page extended API
	mux.HandleFunc("GET /api/ocserv/server/status", appHandler.handleOcservServerStatus)
	mux.HandleFunc("GET /api/ocserv/users/connected", appHandler.handleOcservConnectedUsers)
	mux.HandleFunc("GET /api/ocserv/users/{username}/connections", appHandler.handleOcservUserConnections)
	mux.HandleFunc("GET /api/ocserv/events", appHandler.handleOcservEvents)
	mux.HandleFunc("GET /api/ocserv/routes", appHandler.handleOcservRoutes)
	mux.HandleFunc("GET /api/ocserv/connections/{id}", appHandler.handleOcservConnectionDetail)
	mux.HandleFunc("GET /api/ocserv/bans", appHandler.handleOcservBans)
	mux.HandleFunc("GET /api/ocserv/bans/points", appHandler.handleOcservBanPoints)
	mux.HandleFunc("POST /api/ocserv/users/{username}/disconnect", appHandler.handleOcservUserDisconnect)
	mux.HandleFunc("POST /api/ocserv/bans/{ip}/unban", appHandler.handleOcservUnbanIP)
	// health and log endpoints
	mux.HandleFunc("/api/health/system", appHandler.handleSystemHealth)
	mux.HandleFunc("/api/logs/tail", appHandler.handleLogTail)
	// config management API
	mux.HandleFunc("GET /api/config", appHandler.handleConfigRead)
	mux.HandleFunc("POST /api/config/validate", appHandler.handleConfigValidate)
	mux.HandleFunc("POST /api/config/diff", appHandler.handleConfigDiff)
	mux.HandleFunc("POST /api/config/apply", appHandler.handleConfigApply)
	mux.HandleFunc("GET /api/config/backups", appHandler.handleConfigBackups)
	mux.HandleFunc("GET /api/config/backups/{n}", appHandler.handleConfigBackupGet)
	mux.HandleFunc("GET /api/config/backups/{n}/diff", appHandler.handleConfigBackupDiff)
	mux.HandleFunc("POST /api/config/rollback/{n}", appHandler.handleConfigRollback)
	mux.HandleFunc("GET /api/config/network", appHandler.handleConfigNetwork)
	// Admin user management API
	mux.HandleFunc("GET /api/admin/users", appHandler.handleAdminUsersList)
	mux.HandleFunc("POST /api/admin/users", appHandler.handleAdminUserCreate)
	mux.HandleFunc("PUT /api/admin/users/{id}", appHandler.handleAdminUserUpdate)
	mux.HandleFunc("DELETE /api/admin/users/{id}", appHandler.handleAdminUserDelete)
	mux.HandleFunc("POST /api/admin/users/{id}/disable", appHandler.handleAdminUserDisable)
	mux.HandleFunc("POST /api/admin/users/{id}/enable", appHandler.handleAdminUserEnable)
	mux.HandleFunc("POST /api/admin/users/{id}/reset-password", appHandler.handleAdminUserResetPassword)
	// VPN user lock/unlock
	mux.HandleFunc("POST /api/ocserv/users/{username}/lock", appHandler.handleOcservUserLock)
	mux.HandleFunc("POST /api/ocserv/users/{username}/unlock", appHandler.handleOcservUserUnlock)
	// Bulk operations
	mux.HandleFunc("POST /api/admin/users/bulk-delete", appHandler.handleAdminUsersBulkDelete)
	mux.HandleFunc("POST /api/admin/users/bulk-disable", appHandler.handleAdminUsersBulkDisable)
	mux.HandleFunc("POST /api/admin/users/bulk-enable", appHandler.handleAdminUsersBulkEnable)
	mux.HandleFunc("POST /api/ocserv/users/bulk-delete", appHandler.handleOcservUsersBulkDelete)
	mux.HandleFunc("POST /api/ocserv/users/bulk-lock", appHandler.handleOcservUsersBulkLock)
	mux.HandleFunc("POST /api/ocserv/users/bulk-unlock", appHandler.handleOcservUsersBulkUnlock)
	// Audit log viewing
	mux.HandleFunc("GET /api/audit/logs", appHandler.handleAuditLogs)
	mux.HandleFunc("GET /api/audit/logs/sources", appHandler.handleAuditLogSources)
	mux.HandleFunc("GET /api/audit/logs/export", appHandler.handleAuditLogExport)

	// Serve frontend static files with SPA fallback.
	if staticDir != "" {
		mux.Handle("/", NewSPAFileServer(staticDir))
	}

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
	if a.loginLimiter != nil {
		ip := extractIP(r)
		if !a.loginLimiter.Allow(ip) {
			writeError(w, http.StatusTooManyRequests, "too many login attempts, try again later")
			return
		}
	}
	var req credentialsRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	login, err := a.authService.Login(r.Context(), req.Username, req.Password, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		if errors.Is(err, app.ErrInvalidCredentials) {
			if a.loginLimiter != nil {
				a.loginLimiter.RecordFailure(extractIP(r))
			}
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
		return
	}
	summary, err := a.dashboardService.Summary(r.Context(), principal)
	if err != nil {
		a.writeAppError(w, err)
		return
	}
	refreshInterval := a.refreshInterval
	if refreshInterval < 5 {
		refreshInterval = 15
	}
	writeJSON(w, http.StatusOK, dashboardSummaryToResponse(summary, refreshInterval))
}

func (a *App) handleHistoryAnalytics(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	principal, ok := a.authenticate(w, r)
	if !ok {
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
	writeJSON(w, http.StatusOK, a.catalogList())
}

func (a *App) handleMonitoringOverview(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	window := queryValueDefault(r, "window", "1h")
	step := defaultMonitoringStep(window)
	windowDuration, stepDuration, err := metrics.ValidateWindowAndStep(window, step)
	if err != nil {
		a.writeAppError(w, err)
		return
	}

	now := a.now().UTC()
	series := make([]metrics.Series, 0, len(monitoringOverviewMetrics))
	for _, name := range monitoringOverviewMetrics {
		item, err := a.metricStore.QueryRange(r.Context(), metrics.Query{
			Metric: name,
			Labels: map[string]string{},
			Start:  now.Add(-windowDuration),
			End:    now,
			Step:   stepDuration,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		series = append(series, item)
	}

	writeJSON(w, http.StatusOK, monitoringOverviewResponse{
		Window:      window,
		GeneratedAt: now,
		Catalog:     a.catalogList(),
		Series:      series,
	})
}

func (a *App) handleMonitoringSeries(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}

	metric := r.URL.Query().Get("metric")
	if metric == "" {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	window := queryValueDefault(r, "window", "1h")
	step := queryValueDefault(r, "step", "60s")

	metricName := metrics.MetricName(metric)
	if err := metrics.ValidateMetric(metricName); err != nil {
		a.writeAppError(w, err)
		return
	}
	windowDuration, stepDuration, err := metrics.ValidateWindowAndStep(window, step)
	if err != nil {
		a.writeAppError(w, err)
		return
	}

	now := a.now().UTC()
	result, err := a.metricStore.QueryRange(r.Context(), metrics.Query{
		Metric: metricName,
		Labels: map[string]string{},
		Start:  now.Add(-windowDuration),
		End:    now,
		Step:   stepDuration,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleMonitoringSystemInfo(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, a.buildSystemInfo(r.Context()))
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
	case errors.Is(err, app.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
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
	case errors.Is(err, app.ErrConfigFileMissing):
		writeError(w, http.StatusServiceUnavailable, "config file does not exist at the configured path")
	case errors.Is(err, app.ErrConfigFileParseFailed):
		writeError(w, http.StatusUnprocessableEntity, "config file has syntax errors — fix the file manually or restore from backup")
	case errors.Is(err, app.ErrConfigFileUnreadable):
		writeError(w, http.StatusServiceUnavailable, "config file is not readable — check file permissions")
	case errors.Is(err, ocserv.ErrNotConfigured):
		writeError(w, http.StatusServiceUnavailable, "ocserv is not available")
	case errors.Is(err, ocserv.ErrSessionNotFound):
		writeError(w, http.StatusNotFound, "session not found")
	case errors.Is(err, ocserv.ErrPasswordFileMissing):
		writeError(w, http.StatusServiceUnavailable, "the VPN password file does not exist — check server configuration")
	case errors.Is(err, app.ErrLogSourceUnavailable):
		writeError(w, http.StatusServiceUnavailable, "log source is unavailable")
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

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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

func dashboardSummaryToResponse(summary app.DashboardSummary, refreshInterval int) dashboardSummaryResponse {
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
		SetupRequired:            summary.SetupRequired,
		AdminCount:               summary.AdminCount,
		History:                  analyticsSummaryToResponse(summary.History),
		DashboardRefreshInterval: refreshInterval,
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
