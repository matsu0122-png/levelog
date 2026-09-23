package config

import "testing"

func TestLoad_MissingDatabaseURL(t *testing.T) {
	if _, err := Load(); err == nil {
		t.Fatal("expected error when DATABASE_URL is unset")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port: got %q, want 8080", cfg.Port)
	}
	if cfg.MetricsPort != "9090" {
		t.Errorf("MetricsPort: got %q, want 9090", cfg.MetricsPort)
	}
	if len(cfg.FrontendOrigins) != 1 || cfg.FrontendOrigins[0] != "http://localhost:5173" {
		t.Errorf("FrontendOrigins: got %v, want [http://localhost:5173]", cfg.FrontendOrigins)
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure: expected default false")
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel: got %q, want info", cfg.LogLevel)
	}
	if cfg.HTTPReadTimeout.String() != "15s" {
		t.Errorf("HTTPReadTimeout: got %v, want 15s", cfg.HTTPReadTimeout)
	}
	if cfg.RequestTimeout.String() != "10s" {
		t.Errorf("RequestTimeout: got %v, want 10s", cfg.RequestTimeout)
	}
	if cfg.DBMaxOpenConns != 20 {
		t.Errorf("DBMaxOpenConns: got %d, want 20", cfg.DBMaxOpenConns)
	}
}

func TestLoad_MultipleFrontendOrigins(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("FRONTEND_ORIGIN", " https://staging.example.com ,https://levelog.matsu0122.com ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"https://staging.example.com", "https://levelog.matsu0122.com"}
	if len(cfg.FrontendOrigins) != len(want) {
		t.Fatalf("FrontendOrigins: got %v, want %v", cfg.FrontendOrigins, want)
	}
	for i, o := range want {
		if cfg.FrontendOrigins[i] != o {
			t.Errorf("FrontendOrigins[%d]: got %q, want %q", i, cfg.FrontendOrigins[i], o)
		}
	}
}

func TestLoad_InvalidCookieSecure(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("COOKIE_SECURE", "not-a-bool")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid COOKIE_SECURE")
	}
}

func TestLoad_InvalidDuration(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("REQUEST_TIMEOUT", "not-a-duration")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid REQUEST_TIMEOUT")
	}
}

func TestLoad_InvalidInt(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("DB_MAX_OPEN_CONNS", "not-a-number")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid DB_MAX_OPEN_CONNS")
	}
}

func TestLoad_CustomValuesOverrideDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db")
	t.Setenv("PORT", "9090")
	t.Setenv("METRICS_PORT", "9091")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("COOKIE_DOMAIN", "levelog.matsu0122.com")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("REQUEST_TIMEOUT", "5s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port: got %q, want 9090", cfg.Port)
	}
	if cfg.MetricsPort != "9091" {
		t.Errorf("MetricsPort: got %q, want 9091", cfg.MetricsPort)
	}
	if !cfg.CookieSecure {
		t.Error("CookieSecure: expected true")
	}
	if cfg.CookieDomain != "levelog.matsu0122.com" {
		t.Errorf("CookieDomain: got %q", cfg.CookieDomain)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel: got %q, want debug", cfg.LogLevel)
	}
	if cfg.RequestTimeout.String() != "5s" {
		t.Errorf("RequestTimeout: got %v, want 5s", cfg.RequestTimeout)
	}
}
