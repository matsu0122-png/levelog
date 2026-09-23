package repository_test

// Real-Postgres test for SessionRepo.DeleteExpired: the service-layer tests
// (internal/service/auth_service_test.go) prove the *business logic* against
// an in-memory fake, but not that the actual SQL is correct. Set
// TEST_DATABASE_URL to run this against a scratch Postgres database (same
// convention as internal/handler/router_integration_test.go); skipped
// entirely otherwise.

import (
	"context"
	"os"
	"testing"
	"time"

	"levelog/backend/internal/db"
	"levelog/backend/internal/model"
	"levelog/backend/internal/repository"
)

func TestSessionRepo_DeleteExpired_RemovesOnlyExpired(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping")
	}

	conn, err := db.Open(dsn, db.PoolOptions{MaxOpenConns: 5, MaxIdleConns: 2, ConnMaxLifetime: 30 * time.Minute})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	ctx := context.Background()
	if err := db.WaitForReady(ctx, conn, 10*time.Second); err != nil {
		t.Fatalf("db not ready: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	userRepo := repository.NewUserRepo(conn)
	sessionRepo := repository.NewSessionRepo(conn)

	user := &model.User{
		Email:        uniqueTestEmail(t),
		PasswordHash: "not-a-real-hash",
		Timezone:     "Asia/Tokyo",
	}
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	now := time.Now()
	expired := &model.Session{UserID: user.ID, TokenHash: "expired-" + user.ID, ExpiresAt: now.Add(-time.Hour)}
	valid := &model.Session{UserID: user.ID, TokenHash: "valid-" + user.ID, ExpiresAt: now.Add(time.Hour)}
	if err := sessionRepo.Create(ctx, expired); err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	if err := sessionRepo.Create(ctx, valid); err != nil {
		t.Fatalf("create valid session: %v", err)
	}

	n, err := sessionRepo.DeleteExpired(ctx, now)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if n < 1 {
		t.Errorf("DeleteExpired: expected at least 1 row deleted, got %d", n)
	}

	if got, err := sessionRepo.GetByTokenHash(ctx, expired.TokenHash); err != nil {
		t.Fatalf("GetByTokenHash(expired): %v", err)
	} else if got != nil {
		t.Error("expired session should have been deleted")
	}

	if got, err := sessionRepo.GetByTokenHash(ctx, valid.TokenHash); err != nil {
		t.Fatalf("GetByTokenHash(valid): %v", err)
	} else if got == nil {
		t.Error("valid session should not have been deleted")
	}
}

func uniqueTestEmail(t *testing.T) string {
	t.Helper()
	return "session-repo-test-" + time.Now().Format("20060102150405.000000000") + "@example.com"
}
