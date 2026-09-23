package service

import (
	"context"
	"log/slog"
	"time"
)

// RunSessionCleanupLoop calls auth.CleanupExpiredSessions once immediately
// and then on every tick of interval, until ctx is cancelled. Meant to be
// started as its own goroutine (see cmd/api/main.go) and left running for
// the process's lifetime — it returns only when ctx is done.
func RunSessionCleanupLoop(ctx context.Context, auth *AuthService, logger *slog.Logger, interval time.Duration) {
	cleanupOnce(ctx, auth, logger)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanupOnce(ctx, auth, logger)
		}
	}
}

func cleanupOnce(ctx context.Context, auth *AuthService, logger *slog.Logger) {
	n, err := auth.CleanupExpiredSessions(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "session cleanup failed", "err", err)
		return
	}
	if n > 0 {
		logger.InfoContext(ctx, "session cleanup", "deleted", n)
	}
}
