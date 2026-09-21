package handler

import (
	"net/http"

	"levelog/backend/internal/config"
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
func NewRouter(svc Services, cfg config.Config) http.Handler {
	mux := http.NewServeMux()

	authHandler := NewAuthHandler(svc.Auth, cfg)
	missionHandler := NewMissionHandler(svc.Missions)
	dailyHandler := NewDailyMissionHandler(svc.Missions)
	statsHandler := NewStatsHandler(svc.Missions, svc.XP)

	requireAuth := middleware.RequireAuth(svc.Auth)

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("POST /api/auth/register", authHandler.Register)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
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
	h = middleware.CORS(cfg.FrontendOrigin)(h)
	h = middleware.Logging(h)
	return h
}
