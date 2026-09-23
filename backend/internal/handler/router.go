package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"levelog/backend/internal/config"
	"levelog/backend/internal/metrics"
	"levelog/backend/internal/middleware"
	"levelog/backend/internal/service"
)

type Services struct {
	Auth     *service.AuthService
	Missions *service.MissionService
	XP       *service.XPService
}

// NewRouter wires every route. Auth-protected routes are wrapped with
// middleware.RequireAuth so ownership of every resource is always checked
// against the session's user, never trusted from the request body/path.
// conn is used only for the /health/ready dependency check. The returned
// *metrics.Registry accumulates request counts/latencies as the handler
// serves traffic; the caller mounts its Handler() on a separate,
// non-public-facing server (see cmd/api/main.go) rather than on this
// router, so /metrics is never reachable through the public-facing proxy.
func NewRouter(svc Services, cfg config.Config, conn *sql.DB, logger *slog.Logger) (http.Handler, *metrics.Registry) {
	mux := http.NewServeMux()
	reg := metrics.NewRegistry()

	authHandler := NewAuthHandler(svc.Auth, cfg)
	missionHandler := NewMissionHandler(svc.Missions)
	dailyHandler := NewDailyMissionHandler(svc.Missions)
	statsHandler := NewStatsHandler(svc.Missions, svc.XP)

	requireAuth := middleware.RequireAuth(svc.Auth)

	// /health/live: process is up. Never touches the database — a
	// dependency outage must not make the load balancer kill a healthy
	// instance, only stop routing new traffic to it (that's /health/ready).
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// /health/ready: process is up AND its dependencies (the database) are
	// reachable, so it's safe for the load balancer to route traffic here.
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := conn.PingContext(ctx); err != nil {
			logger.ErrorContext(ctx, "readiness check failed", "err", err)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unavailable"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Kept for backward compatibility with the existing README/docker-compose
	// references; equivalent to /health/live.
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Rate-limited per client IP (independent budgets — see
	// middleware.RateLimit): login is the higher-value brute-force target
	// (a leaked/guessed password grants full account access), so it gets a
	// tighter budget than registration (whose main risk is spam signups,
	// not credential guessing against an existing account).
	loginRateLimit := middleware.RateLimit(rate.Every(6*time.Second), 5)
	registerRateLimit := middleware.RateLimit(rate.Every(10*time.Second), 10)

	mux.Handle("POST /api/auth/register", registerRateLimit(http.HandlerFunc(authHandler.Register)))
	mux.Handle("POST /api/auth/login", loginRateLimit(http.HandlerFunc(authHandler.Login)))
	mux.HandleFunc("POST /api/auth/logout", authHandler.Logout)
	mux.Handle("GET /api/me", requireAuth(http.HandlerFunc(authHandler.Me)))

	mux.Handle("GET /api/missions", requireAuth(http.HandlerFunc(missionHandler.List)))
	mux.Handle("POST /api/missions", requireAuth(http.HandlerFunc(missionHandler.Create)))
	mux.Handle("PUT /api/missions/{id}", requireAuth(http.HandlerFunc(missionHandler.Update)))
	mux.Handle("DELETE /api/missions/{id}", requireAuth(http.HandlerFunc(missionHandler.Delete)))
	mux.Handle("PATCH /api/missions/{id}/active", requireAuth(http.HandlerFunc(missionHandler.SetActive)))

	mux.Handle("GET /api/missions/today", requireAuth(http.HandlerFunc(dailyHandler.Today)))
	mux.Handle("GET /api/missions/history", requireAuth(http.HandlerFunc(dailyHandler.History)))

	mux.Handle("POST /api/daily-missions/{id}/complete", requireAuth(http.HandlerFunc(dailyHandler.Complete)))
	mux.Handle("POST /api/daily-missions/{id}/uncomplete", requireAuth(http.HandlerFunc(dailyHandler.Uncomplete)))

	mux.Handle("GET /api/stats", requireAuth(http.HandlerFunc(statsHandler.Stats)))
	mux.Handle("GET /api/xp/history", requireAuth(http.HandlerFunc(statsHandler.XPHistory)))

	var h http.Handler = mux
	// Wraps mux directly (not another middleware) so mux.Handler(r) inside
	// Metrics resolves the same request mux.ServeHTTP is about to dispatch.
	h = middleware.Metrics(reg, mux)(h)
	h = middleware.CORS(cfg.FrontendOrigins)(h)
	h = middleware.Timeout(cfg.RequestTimeout)(h)
	h = middleware.Logging(logger)(h)
	h = middleware.RequestID(h)
	return h, reg
}
