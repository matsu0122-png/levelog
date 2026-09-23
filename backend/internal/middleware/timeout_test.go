package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeout_SetsDeadlineOnContext(t *testing.T) {
	var hadDeadline bool
	var remaining time.Duration
	handler := Timeout(5 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deadline, ok := r.Context().Deadline()
		hadDeadline = ok
		remaining = time.Until(deadline)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/missions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !hadDeadline {
		t.Fatal("expected request context to carry a deadline")
	}
	if remaining <= 0 || remaining > 5*time.Second {
		t.Errorf("deadline remaining: got %v, want roughly <= 5s and > 0", remaining)
	}
}

func TestTimeout_CancelsContextAfterDeadline(t *testing.T) {
	done := make(chan error, 1)
	handler := Timeout(10 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		done <- r.Context().Err()
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/missions", nil)
	rec := httptest.NewRecorder()

	go handler.ServeHTTP(rec, req)

	select {
	case err := <-done:
		if err != context.DeadlineExceeded {
			t.Errorf("context error: got %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for context cancellation")
	}
}
