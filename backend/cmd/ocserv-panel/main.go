package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/auth"
	"ocserv_panel/backend/internal/config"
	"ocserv_panel/backend/internal/db"
	"ocserv_panel/backend/internal/logging"
	"ocserv_panel/backend/internal/logs"
	"ocserv_panel/backend/internal/monitor"
	
	"ocserv_panel/backend/internal/vpn/sessions"
	"ocserv_panel/backend/internal/vpn/users"
	"ocserv_panel/backend/internal/vpn/health"
	"ocserv_panel/backend/internal/vpn/exec"
	vpnconfig "ocserv_panel/backend/internal/vpn/config"
	"ocserv_panel/backend/internal/vpn/service"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/server"
	"ocserv_panel/backend/internal/store"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("ocserv-panel", version)
		os.Exit(0)
	}

	addr := envOrDefault("OCSERV_PANEL_ADDR", "127.0.0.1:8080")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbDSN := envOrDefault("OCSERV_PANEL_DB_DSN", "file:ocserv_panel.db")
	database, err := db.OpenSQLite(ctx, dbDSN)
	if err != nil {
		log.Error().Err(err).Msg("open sqlite database failed")
		os.Exit(1)
	}
	log.Info().Bool("dsn_set", dbDSN != "").Msg("sqlite database ready")
	defer func() {
		if err := database.Close(); err != nil {
			log.Error().Err(err).Msg("close sqlite database failed")
		}
	}()

	if err := db.ApplyMigrations(ctx, database, resolveMigrationsDir()); err != nil {
		log.Error().Err(err).Msg("apply sqlite migrations failed")
		os.Exit(1)
	}
	log.Info().Msg("migrations applied successfully")

	persistence := store.New(database)
	recorder := audit.NewSQLiteRecorder(persistence)
	authService := app.NewAuthService(persistence, recorder)

	configPath := envOrDefault("OCSERV_PANEL_CONFIG_PATH", "/etc/ocserv-panel/config.toml")
	cfg, cfgErr := config.Load(configPath)
	if cfgErr != nil {
		log.Warn().Str("path", configPath).Err(cfgErr).Msg("loading config file failed, continuing with defaults")
	} else {
		log.Info().Str("path", configPath).Msg("config loaded")
	}
	config.MergeEnv(&cfg, os.Getenv)

	// Initialize structured logging.
	logCloser, _ := logging.Setup(logging.Config{
		Level:    "info",
		FilePath: cfg.Installation.PanelLogPath,
	})
	defer logCloser()

	// Handle SIGHUP for log rotation.
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	go func() {
		for range sighup {
			if err := logging.ReopenFile(); err != nil {
				log.Error().Err(err).Msg("failed to reopen log file on SIGHUP")
			} else {
				log.Info().Msg("log file reopened after SIGHUP")
			}
		}
	}()

	log.Info().
		Str("config_path", cfg.Installation.OcservConfigPath).
		Str("socket_path", cfg.Installation.OcservSocketPath).
		Str("unit", cfg.Service.SystemdUnitName).
		Msg("ocserv integration configured")

	settingsService := app.NewSettingsService(&cfg, configPath, persistence, recorder, rbac.NewDefaultAuthorizer())

	// Build ocserv integration components from config.
	runner := exec.NewOSCommandRunner()
	svcManager := service.NewSystemdManager(cfg.Service.SystemdUnitName, runner)
	readinessChecker := health.NewReadinessChecker(
		func() health.ReadinessConfig {
			liveCfg, err := config.Load(configPath)
			if err != nil {
				liveCfg = cfg
			}
			return health.ReadinessConfig{
				OcservBinPath:    liveCfg.Installation.OcservBinPath,
				OcctlBinPath:     liveCfg.Installation.OcctlBinPath,
				OcpasswdBinPath:  liveCfg.Installation.OcpasswdBinPath,
				ConfigPath:       liveCfg.Installation.OcservConfigPath,
				PasswordFilePath: liveCfg.Installation.OcpasswdFilePath,
				SocketPath:       liveCfg.Installation.OcservSocketPath,
				UnitName:         liveCfg.Service.SystemdUnitName,
			}
		},
		runner,
		svcManager,
	)
	sessionManager := sessions.NewOcctlAdapter(cfg.Installation.OcctlBinPath, cfg.Installation.OcservSocketPath, runner)
	passwdManager := users.NewOcpasswdAdapter(cfg.Installation.OcpasswdBinPath, cfg.Installation.OcpasswdFilePath, cfg.Installation.OcservConfigPath, runner)
	configValidator := vpnconfig.NewConfigValidator(cfg.Installation.OcservConfigPath)
	ocservService := app.NewOcservService(readinessChecker, sessionManager, passwdManager, svcManager, configValidator, recorder, rbac.NewDefaultAuthorizer())

	healthCfgFn := func() config.Config {
		liveCfg, err := config.Load(configPath)
		if err != nil {
			return cfg
		}
		return liveCfg
	}

	// Audit log viewing service
	panelFileReader := logs.NewFileLogReader(cfg.Installation.PanelLogPath, logs.SourcePanel)
	ocservFileReader := logs.NewFileLogReader(cfg.Installation.OcservLogPath, logs.SourceOcserv)
	panelJournalReader := logs.NewJournalLogReader(cfg.Installation.PanelSystemdUnit, logs.SourcePanel, runner)
	ocservJournalReader := logs.NewJournalLogReader(cfg.Service.SystemdUnitName, logs.SourceOcserv, runner)
	auditLogService := app.NewAuditLogService(
		healthCfgFn,
		map[string]logs.LogReader{"panel": panelFileReader, "ocserv": ocservFileReader},
		map[string]logs.LogReader{"panel": panelJournalReader, "ocserv": ocservJournalReader},
		recorder,
		rbac.NewDefaultAuthorizer(),
	)

	configFiles := vpnconfig.NewConfigFileManager(cfg.Installation.OcservConfigPath, 10)
	configReloader := vpnconfig.NewConfigReloader(runner, cfg.Installation.OcctlBinPath)
	networkInspector := health.NewNetworkInspectorWithConfig(runner, "/proc/sys", cfg.Installation.OcservConfigPath)
	configMgmtService := app.NewConfigManagementService(
		configFiles,
		configValidator,
		configReloader,
		networkInspector,
		vpnconfig.NewRestartDetector(),
		recorder,
		rbac.NewDefaultAuthorizer(),
	)

	adminUserService := app.NewAdminUserService(persistence, recorder, rbac.NewDefaultAuthorizer(), auth.HashPassword)

	// Create housekeeping scheduler
	housekeepingScheduler := app.NewHousekeepingScheduler(
		persistence,
		recorder,
		func(ctx context.Context, retentionDays int) (int64, error) {
			cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
			return persistence.PurgeHistoricalBefore(ctx, cutoff)
		},
		24*time.Hour,
	)
	go housekeepingScheduler.Run(ctx)

	// Start session ingester: polls ocserv for active sessions and persists to historical_sessions.
	sessionIngester := app.NewSessionIngester(persistence, sessionManager)
	go sessionIngester.Run(ctx)

	metricStore := openMetricStore()
	defer func() {
		if err := metricStore.Close(); err != nil {
			log.Error().Err(err).Msg("close metric store failed")
		}
	}()
	metricCatalog := monitor.DefaultCatalog()
	dashboardService := app.NewDashboardService(persistence, recorder, rbac.NewDefaultAuthorizer(), readinessChecker, metricStore)
	collectors := configuredCollectors(monitor.OSReader{}, cfg.Monitoring)

	// Use config-based collector interval with fallback.
	collectorInterval := time.Duration(cfg.Monitoring.CollectorInterval) * time.Second
	if collectorInterval <= 0 {
		collectorInterval = 15 * time.Second
	}

	collectorService := monitor.NewCollectorService(
		metricStore,
		collectors,
		collectorInterval,
		zerologAdapter{},
		nil,
	)

	vpnWatcher := monitor.NewVPNWatcher(
		collectorService,
		metricCatalog,
		func() ([]string, error) { return monitor.DiscoverAllVPNInterfaces() },
		monitor.OSReader{},
		monitor.MetricVPNInterfaceRXBytes,
		monitor.MetricVPNInterfaceTXBytes,
		zerologAdapter{},
	)

	sessionCollector := monitor.NewSessionCollectorWithNotifier(sessionManager, vpnWatcher)
	collectorService.AddCollector(sessionCollector)
	metricCatalog.SetState(monitor.MetricActiveSessions, monitor.StateCollecting, "")
	metricCatalog.SetState(monitor.MetricConcurrentSessions, monitor.StateCollecting, "")

	go vpnWatcher.Run(ctx)
	go collectorService.Run(ctx)

	loginLimiter := auth.NewRateLimiter(5, time.Minute)

	staticDir := envOrDefault("OCSERV_PANEL_STATIC_DIR", cfg.Installation.StaticDir)

	httpServer := &http.Server{
		Addr: addr,
		Handler: server.NewAppRouter(
			authService,
			dashboardService,
			metricStore,
			metricCatalog,
			settingsService,
			ocservService,
			app.OSLogFileReader{},
			healthCfgFn,
			persistence,
			readinessChecker,
			tsdbPathFromEnv(),
			configMgmtService,
			adminUserService,
			auditLogService,
			rbac.NewDefaultAuthorizer(),
			persistence,
			server.AppOptions{CookieSecure: cookieSecureFromEnv(), LoginLimiter: loginLimiter, RefreshInterval: cfg.Monitoring.DashboardRefreshInterval},
			staticDir,
		),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
	}

	// Resolve TLS cert paths from env (overrides config).
	tlsCert := envOrDefault("OCSERV_PANEL_TLS_CERT", cfg.Security.TLSCertPath)
	tlsKey := envOrDefault("OCSERV_PANEL_TLS_KEY", cfg.Security.TLSKeyPath)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("backend shutdown failed")
		}
	}()

	if tlsCert != "" && tlsKey != "" {
		certReloader, certErr := server.NewCertReloader(tlsCert, tlsKey)
		if certErr != nil {
			log.Error().Err(certErr).Msg("failed to load TLS certificates")
			os.Exit(1)
		}
		defer certReloader.Stop()

		httpServer.TLSConfig = &tls.Config{
			GetCertificate: certReloader.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		}

		log.Info().
			Str("addr", addr).
			Str("tls_cert", tlsCert).
			Str("static_dir", staticDir).
			Msg("starting ocserv panel backend (HTTPS)")
		if err := httpServer.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("backend server failed")
			os.Exit(1)
		}
	} else {
		log.Info().
			Str("addr", addr).
			Str("static_dir", staticDir).
			Msg("starting ocserv panel backend (HTTP)")
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("backend server failed")
			os.Exit(1)
		}
	}
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func tsdbPathFromEnv() string {
	return envOrDefault("OCSERV_PANEL_TSDB_PATH", filepath.Join("data", "metrics"))
}

func openMetricStore() monitor.Store {
	tsdbPath := tsdbPathFromEnv()
	retention := 30 * 24 * time.Hour
	metricStore, err := monitor.OpenTSDBStore(tsdbPath, retention)
	if err == nil {
		log.Info().Str("type", "tsdb").Str("path", tsdbPath).Str("retention", "30d").Msg("metrics store ready")
		return metricStore
	}

	log.Warn().Err(err).Msg("open metrics tsdb failed; using in-memory metrics store")

	return monitor.NewMemoryStore()
}

func configuredCollectors(reader monitor.FileReader, monCfg config.MonitoringConfig) []monitor.Collector {
	collectors := []monitor.Collector{
		monitor.NewCPUCollector(reader),
		monitor.NewMemoryCollector(reader),
	}

	// Network interface: config > env > auto-discovery
	netIface := strings.TrimSpace(monCfg.NetworkInterface)
	if netIface == "" {
		netIface = strings.TrimSpace(os.Getenv("OCSERV_PANEL_NET_IFACE"))
	}
	if netIface == "" {
		if discovered, err := monitor.DiscoverDefaultInterface(); err != nil {
			log.Warn().Err(err).Msg("default interface discovery failed")
		} else if discovered != "" {
			log.Info().Str("interface", discovered).Msg("default interface discovered")
			netIface = discovered
		} else {
			log.Info().Msg("no default interface discovered; network interface metrics will remain unavailable")
		}
	}
	if netIface != "" {
		collectors = append(collectors, monitor.NewInterfaceCollector(
			reader,
			netIface,
			monitor.MetricNetInterfaceRXBytes,
			monitor.MetricNetInterfaceTXBytes,
		))
	}

	return collectors
}

func cookieSecureFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OCSERV_PANEL_COOKIE_SECURE"))) {
	case "1", "t", "true", "yes", "y", "on":
		return true
	case "0", "f", "false", "no", "n", "off":
		return false
	default:
		return true
	}
}

func resolveMigrationsDir() string {
	if value := os.Getenv("OCSERV_PANEL_MIGRATIONS_DIR"); value != "" {
		return value
	}

	for _, candidate := range []string{"migrations", filepath.Join("..", "migrations")} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	return filepath.Join("..", "migrations")
}

// zerologAdapter implements monitor.Logger using the global zerolog logger.
type zerologAdapter struct{}

func (zerologAdapter) Debug(msg string, args ...any) {
	log.Debug().Fields(pairsToMap(args)).Msg(msg)
}

func (zerologAdapter) Warn(msg string, args ...any) {
	log.Warn().Fields(pairsToMap(args)).Msg(msg)
}

func pairsToMap(args []any) map[string]interface{} {
	m := make(map[string]interface{}, len(args)/2)
	for i := 0; i+1 < len(args); i += 2 {
		if k, ok := args[i].(string); ok {
			m[k] = args[i+1]
		}
	}
	return m
}
