// Command api is the levelog backend HTTP server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"levelog/backend/internal/config"
	"levelog/backend/internal/db"
	"levelog/backend/internal/handler"
	"levelog/backend/internal/logging"
	"levelog/backend/internal/repository"
	"levelog/backend/internal/service"
)

// How often expired sessions are purged (service.RunSessionCleanupLoop).
// Not exposed as an env var: unlike the DB pool/timeout settings in
// config.Config, there's no deployment-specific "correct" value here — the
// cleanup query is cheap regardless of scale, so a fixed sensible interval
// is enough.
const sessionCleanupInterval = time.Hour

func main() {
	cfg, err := config.Load()
	if err != nil {
		// Logger isn't built yet (it needs cfg.LogLevel), so this one line
		// stays on the stdlib logger.
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.LogLevel)
	slog.SetDefault(logger)

	conn, err := db.Open(cfg.DatabaseURL, db.PoolOptions{
		MaxOpenConns:    cfg.DBMaxOpenConns,
		MaxIdleConns:    cfg.DBMaxIdleConns,
		ConnMaxLifetime: cfg.DBConnMaxLifetime,
	})
	if err != nil {
		logger.Error("open db", "err", err)
		os.Exit(1)
	}
	defer conn.Close()

	startupCtx, stopStartup := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if err := db.WaitForReady(startupCtx, conn, cfg.DBConnectTimeout); err != nil {
		stopStartup()
		logger.Error("db not ready", "err", err)
		os.Exit(1)
	}
	stopStartup()

	if err := db.Migrate(conn); err != nil {
		logger.Error("migrate", "err", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")

	userRepo := repository.NewUserRepo(conn)
	sessionRepo := repository.NewSessionRepo(conn)
	templateRepo := repository.NewTemplateRepo(conn)
	dailyRepo := repository.NewDailyMissionRepo(conn)
	xpRepo := repository.NewXPRepo(conn)

	authService := service.NewAuthService(userRepo, sessionRepo, nil)
	missionService := service.NewMissionService(userRepo, templateRepo, dailyRepo, xpRepo, nil)
	xpService := service.NewXPService(xpRepo)

	router, metricsRegistry := handler.NewRouter(handler.Services{
		Auth:     authService,
		Missions: missionService,
		XP:       xpService,
	}, cfg, conn, logger)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	// Separate server/port from the app's own — see config.MetricsPort's
	// doc comment for why /metrics isn't just another route on router.
	metricsMux := http.NewServeMux()
	metricsMux.Handle("GET /metrics", metricsRegistry.Handler())
	metricsServer := &http.Server{
		Addr:              ":" + cfg.MetricsPort,
		Handler:           metricsMux,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go service.RunSessionCleanupLoop(ctx, authService, logger, sessionCleanupInterval)

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("levelog api listening", "addr", server.Addr, "frontend_origins", cfg.FrontendOrigins)
		serverErr <- server.ListenAndServe()
	}()
	go func() {
		logger.Info("levelog metrics listening", "addr", metricsServer.Addr)
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// Non-fatal: an observability endpoint failing to start
			// shouldn't take down the app it's meant to observe.
			logger.Error("metrics server error", "err", err)
		}
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("metrics server graceful shutdown failed", "err", err)
	}
	logger.Info("server stopped")
}
