package service

import (
	"context"
	"time"

	"levelog/backend/internal/model"
)

// Clock abstracts "now" so services are deterministic and testable without
// depending on wall-clock time.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// RealClock is the production Clock implementation.
var RealClock Clock = realClock{}

// UserRepository persists application accounts.
type UserRepository interface {
	Create(ctx context.Context, u *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
}

// SessionRepository persists login sessions. Only a hash of the bearer token
// is ever passed in or out.
type SessionRepository interface {
	Create(ctx context.Context, s *model.Session) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error)
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
}

// MissionTemplateRepository persists recurring mission definitions and their
// weekly schedules.
type MissionTemplateRepository interface {
	Create(ctx context.Context, t *model.MissionTemplate) error
	Update(ctx context.Context, t *model.MissionTemplate) error
	GetByID(ctx context.Context, userID, id string) (*model.MissionTemplate, error)
	ListByUser(ctx context.Context, userID string) ([]model.MissionTemplate, error)
	ListActiveByUserAndWeekday(ctx context.Context, userID string, weekday model.Weekday) ([]model.MissionTemplate, error)
	SetActive(ctx context.Context, userID, id string, active bool) error
	SoftDelete(ctx context.Context, userID, id string) error
}

// MissionTransitionResult is returned by DailyMissionRepository.Complete and
// Uncomplete. XPDelta is 0 when the call was an idempotent no-op (the
// mission was already in the target status), so callers can tell a repeat
// tap from an actual state change without granting XP twice.
type MissionTransitionResult struct {
	Mission      model.DailyMission
	XPDelta      int
	TotalXPAfter int
}

// DailyMissionRepository persists per-day mission instances. Complete and
// Uncomplete each perform their status change, completed_at bookkeeping, and
// XP ledger entry atomically (within a single database transaction in the
// production implementation), which is what makes double-tap-safe XP
// granting possible.
type DailyMissionRepository interface {
	// EnsureGenerated creates any daily_missions rows that are missing for
	// targetDate, one per template in templates. Already-existing rows are
	// left untouched (idempotent, race-safe under concurrent calls).
	EnsureGenerated(ctx context.Context, userID, targetDate string, templates []model.MissionTemplate) error

	ListByUserAndDate(ctx context.Context, userID, date string) ([]model.DailyMission, error)
	ListByUserAndDateRange(ctx context.Context, userID string, dates []string) ([]model.DailyMission, error)
	GetByID(ctx context.Context, userID, id string) (*model.DailyMission, error)

	// Complete transitions a PENDING mission to COMPLETED and grants XP.
	// It only succeeds if the mission belongs to userID and its target date
	// equals today; otherwise it returns an *apperror.Error. If the mission
	// is already COMPLETED, it is a no-op (XPDelta will be 0).
	Complete(ctx context.Context, userID, dailyMissionID, today string) (MissionTransitionResult, error)

	// Uncomplete transitions a COMPLETED mission back to PENDING and reverses
	// the XP grant. Same ownership/today rules as Complete. If the mission is
	// already PENDING, it is a no-op (XPDelta will be 0).
	Uncomplete(ctx context.Context, userID, dailyMissionID, today string) (MissionTransitionResult, error)
}

// XPRepository provides read access to the XP ledger. Total XP is always
// derived by summing xp_transactions — never stored as a mutable column.
type XPRepository interface {
	SumByUser(ctx context.Context, userID string) (int, error)
	ListRecentByUser(ctx context.Context, userID string, limit int) ([]model.XPTransaction, error)
}
