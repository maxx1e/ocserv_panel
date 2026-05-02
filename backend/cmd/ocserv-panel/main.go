package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ocserv_panel/backend/internal/app"
	"ocserv_panel/backend/internal/audit"
	"ocserv_panel/backend/internal/db"
	"ocserv_panel/backend/internal/metrics"
	"ocserv_panel/backend/internal/monitoring"
	"ocserv_panel/backend/internal/rbac"
	"ocserv_panel/backend/internal/server"
	"ocserv_panel/backend/internal/store"
)

func main() {
	addr := envOrDefault("OCSERV_PANEL_ADDR", "127.0.0.1:8080")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.OpenSQLite(ctx, envOrDefault("OCSERV_PANEL_DB_DSN", "file:ocserv_panel.db"))
	if err != nil {
		slog.Error("open sqlite database failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := database.Close(); err != nil {
			slog.Error("close sqlite database failed", "error", err)
		}
	}()

	if err := db.ApplyMigrations(ctx, database, resolveMigrationsDir()); err != nil {
		slog.Error("apply sqlite migrations failed", "error", err)
		os.Exit(1)
	}

	persistence := store.New(database)
	recorder := audit.NewSQLiteRecorder(persistence)
	authService := app.NewAuthService(persistence, recorder)
	dashboardService := app.NewDashboardService(persistence, recorder, rbac.NewDefaultAuthorizer())
	settingsService := app.NewSettingsService(persistence, recorder, rbac.NewDefaultAuthorizer())

	// Create housekeeping scheduler
	housekeepingScheduler := app.NewHousekeepingScheduler(
		persistence,
		recorder,
		rbac.NewDefaultAuthorizer(),
		func(ctx context.Context, retentionDays int) (int64, error) {
			cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
			return persistence.PurgeHistoricalBefore(ctx, cutoff)
		},
		24*time.Hour,
		app.HousekeepingWithLogger(slog.Default()),
	)
	go housekeepingScheduler.Run(ctx)

	metricStore := openMetricStore()
	defer func() {
		if err := metricStore.Close(); err != nil {
			slog.Error("close metric store failed", "error", err)
		}
	}()
	monitoringService := app.NewMonitoringService(metricStore, rbac.NewDefaultAuthorizer(), nil, recorder)
	collectorService := monitoring.NewCollectorService(
		metricStore,
		configuredCollectors(monitoring.OSReader{}),
		15*time.Second,
		slog.Default(),
		nil,
	)
	go collectorService.Run(ctx)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.NewAppRouter(authService, dashboardService, monitoringService, settingsService, server.AppOptions{CookieSecure: cookieSecureFromEnv()}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("backend shutdown failed", "error", err)
		}
	}()

	slog.Info("starting ocserv panel backend", "addr", addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("backend server failed", "error", err)
		os.Exit(1)
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

func openMetricStore() metrics.Store {
	metricStore, err := metrics.OpenTSDBStore(tsdbPathFromEnv(), 30*24*time.Hour)
	if err == nil {
		return metricStore
	}

	slog.Warn("open metrics tsdb failed; using in-memory metrics store", "error", err)

	return metrics.NewMemoryStore()
}

func configuredCollectors(reader monitoring.FileReader) []monitoring.Collector {
	collectors := []monitoring.Collector{
		monitoring.NewCPUCollector(reader),
		monitoring.NewMemoryCollector(reader),
	}

	if iface := strings.TrimSpace(os.Getenv("OCSERV_PANEL_NET_IFACE")); iface != "" {
		collectors = append(collectors, monitoring.NewInterfaceCollector(
			reader,
			iface,
			metrics.MetricNetInterfaceRXBytes,
			metrics.MetricNetInterfaceTXBytes,
		))
	}
	if iface := strings.TrimSpace(os.Getenv("OCSERV_PANEL_VPN_IFACE")); iface != "" {
		collectors = append(collectors, monitoring.NewInterfaceCollector(
			reader,
			iface,
			metrics.MetricVPNInterfaceRXBytes,
			metrics.MetricVPNInterfaceTXBytes,
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
