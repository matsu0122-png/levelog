// Package config reads process configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port            string
	MetricsPort     string
	DatabaseURL     string
	FrontendOrigins []string
	CookieSecure    bool
	CookieDomain    string
	LogLevel        string

	// HTTP server lifecycle.
	HTTPReadHeaderTimeout time.Duration
	HTTPReadTimeout       time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
	RequestTimeout        time.Duration
	ShutdownTimeout       time.Duration

	// Database connection pool / startup.
	DBConnectTimeout  time.Duration
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
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

	// Served on its own port, deliberately separate from Port: the /metrics
	// handler carries no auth of its own, so it's kept off the path Nginx
	// proxies to the internet and reachable only via a loopback-bound
	// docker-compose port mapping (see docker-compose.prod.yml) — the same
	// "not reachable off the box" posture already used for node_exporter
	// (see docs/monitoring-design.md).
	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "9090"
	}

	frontendOrigin := os.Getenv("FRONTEND_ORIGIN")
	if frontendOrigin == "" {
		frontendOrigin = "http://localhost:5173"
	}
	var origins []string
	for _, o := range strings.Split(frontendOrigin, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}

	cookieSecure := false
	if v := os.Getenv("COOKIE_SECURE"); v != "" {
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid COOKIE_SECURE: %w", err)
		}
		cookieSecure = parsed
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	readHeaderTimeout, err := envDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	readTimeout, err := envDuration("HTTP_READ_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	writeTimeout, err := envDuration("HTTP_WRITE_TIMEOUT", 20*time.Second)
	if err != nil {
		return Config{}, err
	}
	idleTimeout, err := envDuration("HTTP_IDLE_TIMEOUT", 60*time.Second)
	if err != nil {
		return Config{}, err
	}
	requestTimeout, err := envDuration("REQUEST_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := envDuration("SHUTDOWN_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	dbConnectTimeout, err := envDuration("DB_CONNECT_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}
	dbConnMaxLifetime, err := envDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	dbMaxOpenConns, err := envInt("DB_MAX_OPEN_CONNS", 20)
	if err != nil {
		return Config{}, err
	}
	dbMaxIdleConns, err := envInt("DB_MAX_IDLE_CONNS", 5)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Port:            port,
		MetricsPort:     metricsPort,
		DatabaseURL:     dbURL,
		FrontendOrigins: origins,
		CookieSecure:    cookieSecure,
		CookieDomain:    os.Getenv("COOKIE_DOMAIN"),
		LogLevel:        logLevel,

		HTTPReadHeaderTimeout: readHeaderTimeout,
		HTTPReadTimeout:       readTimeout,
		HTTPWriteTimeout:      writeTimeout,
		HTTPIdleTimeout:       idleTimeout,
		RequestTimeout:        requestTimeout,
		ShutdownTimeout:       shutdownTimeout,

		DBConnectTimeout:  dbConnectTimeout,
		DBMaxOpenConns:    dbMaxOpenConns,
		DBMaxIdleConns:    dbMaxIdleConns,
		DBConnMaxLifetime: dbConnMaxLifetime,
	}, nil
}

func envDuration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return d, nil
}

func envInt(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return n, nil
}
