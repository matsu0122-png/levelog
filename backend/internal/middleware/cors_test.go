package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_AllowedOrigin(t *testing.T) {
	handler := CORS([]string{"https://levelog.matsu0122.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Origin", "https://levelog.matsu0122.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://levelog.matsu0122.com" {
		t.Errorf("Access-Control-Allow-Origin: got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials: got %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	called := false
	handler := CORS([]string{"https://levelog.matsu0122.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for disallowed origin, got %q", got)
	}
	// The request itself still reaches the handler — CORS is enforced by the
	// browser refusing to expose the response, not by the server blocking it.
	if !called {
		t.Error("expected inner handler to still run for a disallowed origin")
	}
}

func TestCORS_MultipleAllowedOrigins(t *testing.T) {
	handler := CORS([]string{"https://staging.example.com", "https://levelog.matsu0122.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, origin := range []string{"https://staging.example.com", "https://levelog.matsu0122.com"} {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("origin %q: Access-Control-Allow-Origin got %q", origin, got)
		}
	}
}

func TestCORS_PreflightRequest(t *testing.T) {
	called := false
	handler := CORS([]string{"https://levelog.matsu0122.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/missions", nil)
	req.Header.Set("Origin", "https://levelog.matsu0122.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusNoContent)
	}
	if called {
		t.Error("preflight OPTIONS must not reach the inner handler")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("expected Access-Control-Allow-Methods on preflight response")
	}
}
