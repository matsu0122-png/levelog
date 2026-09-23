package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"levelog/backend/internal/reqid"
)

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	var idSeenByHandler string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idSeenByHandler = reqid.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	respID := rec.Header().Get(RequestIDHeader)
	if respID == "" {
		t.Fatal("expected a generated X-Request-Id header")
	}
	if idSeenByHandler != respID {
		t.Errorf("handler saw request id %q, response header has %q", idSeenByHandler, respID)
	}
}

func TestRequestID_ReusesUpstreamHeader(t *testing.T) {
	const upstreamID = "upstream-provided-id-123"
	var idSeenByHandler string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idSeenByHandler = reqid.FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	req.Header.Set(RequestIDHeader, upstreamID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if idSeenByHandler != upstreamID {
		t.Errorf("handler context id: got %q, want %q", idSeenByHandler, upstreamID)
	}
	if got := rec.Header().Get(RequestIDHeader); got != upstreamID {
		t.Errorf("response header: got %q, want %q", got, upstreamID)
	}
}

func TestRequestID_UniquePerRequest(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		id := rec.Header().Get(RequestIDHeader)
		if seen[id] {
			t.Fatalf("duplicate request id generated: %q", id)
		}
		seen[id] = true
	}
}
