package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"
)

func TestRateLimit_AllowsUpToBurstThenRejects(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// rate.Inf-adjacent: a refill rate slow enough that, within this
	// test's runtime, only the initial burst is available.
	handler := RateLimit(rate.Limit(0.001), 3)(inner)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = "203.0.113.10:12345"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 once burst is exhausted, got %d", rec.Code)
	}
}

func TestRateLimit_TracksClientsIndependently(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := RateLimit(rate.Limit(0.001), 1)(inner)

	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req1.RemoteAddr = "203.0.113.10:1"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("client 1 first request: expected 200, got %d", rec1.Code)
	}

	// Same client again: burst of 1 is exhausted.
	rec1b := httptest.NewRecorder()
	handler.ServeHTTP(rec1b, req1)
	if rec1b.Code != http.StatusTooManyRequests {
		t.Fatalf("client 1 second request: expected 429, got %d", rec1b.Code)
	}

	// A different client's budget is untouched by client 1's requests.
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req2.RemoteAddr = "203.0.113.20:1"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("client 2 first request: expected 200, got %d", rec2.Code)
	}
}

func TestClientIP_PrefersXForwardedForOverRemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "10.0.0.5:54321" // e.g. the nginx container's docker-network IP
	req.Header.Set("X-Forwarded-For", "203.0.113.55")

	if got := clientIP(req); got != "203.0.113.55" {
		t.Errorf("clientIP: got %q, want 203.0.113.55", got)
	}
}

func TestClientIP_TakesLeftmostOfMultipleForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.55, 10.0.0.5")

	if got := clientIP(req); got != "203.0.113.55" {
		t.Errorf("clientIP: got %q, want 203.0.113.55", got)
	}
}

func TestClientIP_FallsBackToRemoteAddrWithoutHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "203.0.113.55:54321"

	if got := clientIP(req); got != "203.0.113.55" {
		t.Errorf("clientIP: got %q, want 203.0.113.55", got)
	}
}
