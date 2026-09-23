package service

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestRunSessionCleanupLoop_RunsImmediatelyAndStopsOnCancel(t *testing.T) {
	store := newFakeStore()
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	auth := newAuthServiceForTest(store, clock)
	if _, err := auth.Register(context.Background(), "loop@example.com", "password123", "Asia/Tokyo"); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, _, err := auth.Login(context.Background(), "loop@example.com", "password123"); err != nil {
		t.Fatalf("login: %v", err)
	}
	clock.now = clock.now.Add(31 * 24 * time.Hour) // the session is now expired

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		// An interval far longer than this test's timeout: only the
		// immediate run-before-the-first-tick (RunSessionCleanupLoop's
		// documented behavior) should have a chance to fire.
		RunSessionCleanupLoop(ctx, auth, logger, time.Hour)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for {
		store.mu.Lock()
		n := len(store.sessionsByHash)
		store.mu.Unlock()
		if n == 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("expired session was not cleaned up promptly after starting the loop")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunSessionCleanupLoop did not return after ctx was cancelled")
	}
}
