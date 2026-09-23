package service

import (
	"context"
	"testing"
	"time"
)

// TestCleanupExpiredSessions_RemovesOnlyExpired guards the gap flagged since
// phase 2 (docs/production-roadmap.md): nothing pruned the sessions table,
// so it grew forever. This exercises AuthService.CleanupExpiredSessions
// (the method RunSessionCleanupLoop calls periodically in production, see
// cmd/api/main.go) against real Register/Login-issued sessions, not
// hand-built ones, so it also verifies expires_at is actually set the way
// Register/Login expect.
func TestCleanupExpiredSessions_RemovesOnlyExpired(t *testing.T) {
	store := newFakeStore()
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	auth := newAuthServiceForTest(store, clock)
	ctx := context.Background()

	if _, err := auth.Register(ctx, "expires-soon@example.com", "password123", "Asia/Tokyo"); err != nil {
		t.Fatalf("register (expires-soon): %v", err)
	}
	soonToken, _, err := auth.Login(ctx, "expires-soon@example.com", "password123")
	if err != nil {
		t.Fatalf("login (expires-soon): %v", err)
	}

	// Move the clock forward past the first session's 30-day TTL, then log
	// in as a second user — this session must survive the cleanup below.
	clock.now = clock.now.Add(31 * 24 * time.Hour)

	if _, err := auth.Register(ctx, "still-valid@example.com", "password123", "Asia/Tokyo"); err != nil {
		t.Fatalf("register (still-valid): %v", err)
	}
	validToken, _, err := auth.Login(ctx, "still-valid@example.com", "password123")
	if err != nil {
		t.Fatalf("login (still-valid): %v", err)
	}

	if len(store.sessionsByHash) != 2 {
		t.Fatalf("setup: expected 2 sessions before cleanup, got %d", len(store.sessionsByHash))
	}

	n, err := auth.CleanupExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("CleanupExpiredSessions: %v", err)
	}
	if n != 1 {
		t.Errorf("CleanupExpiredSessions: deleted %d, want 1", n)
	}

	if _, err := auth.ValidateSession(ctx, soonToken); err == nil {
		t.Error("expired session should no longer validate after cleanup")
	}
	if _, err := auth.ValidateSession(ctx, validToken); err != nil {
		t.Errorf("still-valid session should keep validating after cleanup, got err: %v", err)
	}
	if len(store.sessionsByHash) != 1 {
		t.Errorf("expected 1 session left after cleanup, got %d", len(store.sessionsByHash))
	}
}

func TestCleanupExpiredSessions_NoExpiredSessionsIsANoOp(t *testing.T) {
	store := newFakeStore()
	clock := &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	auth := newAuthServiceForTest(store, clock)
	ctx := context.Background()

	if _, err := auth.Register(ctx, "fresh@example.com", "password123", "Asia/Tokyo"); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, _, err := auth.Login(ctx, "fresh@example.com", "password123"); err != nil {
		t.Fatalf("login: %v", err)
	}

	n, err := auth.CleanupExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("CleanupExpiredSessions: %v", err)
	}
	if n != 0 {
		t.Errorf("CleanupExpiredSessions: deleted %d, want 0", n)
	}
	if len(store.sessionsByHash) != 1 {
		t.Errorf("expected the fresh session to survive, got %d sessions", len(store.sessionsByHash))
	}
}
