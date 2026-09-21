// Package config reads process configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port           string
	DatabaseURL    string
	FrontendOrigin string
	CookieSecure   bool
	CookieDomain   string
}

func Load() (Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	frontendOrigin := os.Getenv("FRONTEND_ORIGIN")
	if frontendOrigin == "" {
		frontendOrigin = "http://localhost:5173"
	}

	cookieSecure := false
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid COOKIE_SECURE: %w", err)
		}
		cookieSecure = parsed
	}

	return Config{
		Port:           port,
		DatabaseURL:    dbURL,
		FrontendOrigin: frontendOrigin,
		CookieSecure:   cookieSecure,
		CookieDomain:   os.Getenv("COOKIE_DOMAIN"),
	}, nil
}
