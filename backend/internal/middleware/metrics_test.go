package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeRecorder struct {
	method string
	route  string
	status int
	called bool
}

func (f *fakeRecorder) Observe(method, route string, status int, dur time.Duration) {
	f.method = method
	f.route = route
	f.status = status
	f.called = true
}

func TestMetrics_RecordsRouteMethodAndStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/missions/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	rec := &fakeRecorder{}
	var h http.Handler = mux
	h = Metrics(rec, mux)(h)

	req := httptest.NewRequest(http.MethodGet, "/api/missions/42", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if !rec.called {
		t.Fatal("expected Observe to be called")
	}
	if rec.method != http.MethodGet {
		t.Errorf("method: got %q, want GET", rec.method)
	}
	// The registered pattern (with {id}), not the raw path with 42 — keeps
	// the metric's cardinality bounded regardless of how many distinct IDs
	// are requested.
	if rec.route != "GET /api/missions/{id}" {
		t.Errorf("route: got %q, want \"GET /api/missions/{id}\"", rec.route)
	}
	if rec.status != http.StatusCreated {
		t.Errorf("status: got %d, want 201", rec.status)
	}
}

func TestMetrics_UnmatchedRouteUsesPlaceholder(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/missions", func(w http.ResponseWriter, r *http.Request) {})

	rec := &fakeRecorder{}
	var h http.Handler = mux
	h = Metrics(rec, mux)(h)

	req := httptest.NewRequest(http.MethodGet, "/no-such-route", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if rec.route != "unmatched" {
		t.Errorf("route: got %q, want \"unmatched\"", rec.route)
	}
}
