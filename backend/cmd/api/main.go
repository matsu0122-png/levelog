// Command api is the levelog backend HTTP server.
package main

import (
	"log"
	"net/http"
	"time"

	"levelog/backend/internal/config"
	"levelog/backend/internal/db"
	"levelog/backend/internal/handler"
	"levelog/backend/internal/repository"
	"levelog/backend/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	conn, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	if err := db.WaitForReady(conn, 30*time.Second); err != nil {
		log.Fatalf("db not ready: %v", err)
	}

	if err := db.Migrate(conn); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("migrations applied")

	userRepo := repository.NewUserRepo(conn)
	sessionRepo := repository.NewSessionRepo(conn)
	templateRepo := repository.NewTemplateRepo(conn)
	dailyRepo := repository.NewDailyMissionRepo(conn)
	xpRepo := repository.NewXPRepo(conn)

	authService := service.NewAuthService(userRepo, sessionRepo, nil)
	missionService := service.NewMissionService(userRepo, templateRepo, dailyRepo, xpRepo, nil)
	xpService := service.NewXPService(xpRepo)

	router := handler.NewRouter(handler.Services{
		Auth:     authService,
		Missions: missionService,
		XP:       xpService,
	}, cfg)

	addr := ":" + cfg.Port
	log.Printf("levelog api listening on %s (frontend origin: %s)", addr, cfg.FrontendOrigin)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}
