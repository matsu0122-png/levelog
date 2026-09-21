package model

import "time"

// User is an application account.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	Timezone     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// MissionTemplate is a user-defined recurring mission definition.
type MissionTemplate struct {
	ID          string
	UserID      string
	Title       string
	Description string
	Difficulty  Difficulty
	XPReward    int
	Active      bool
	DeletedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// ScheduleDays is the set of weekdays this mission recurs on.
	ScheduleDays []Weekday
}

// DailyMissionStatus is the lifecycle state of a single day's mission
// instance. There are intentionally only two states: no "failed" or
// "skipped" state exists, because this app never penalizes non-completion.
type DailyMissionStatus string

const (
	DailyMissionPending   DailyMissionStatus = "PENDING"
	DailyMissionCompleted DailyMissionStatus = "COMPLETED"
)

// DailyMission is a snapshot of a mission template generated for one
// specific calendar date (in the owning user's timezone).
type DailyMission struct {
	ID                string
	UserID            string
	MissionTemplateID string
	TargetDate        string // YYYY-MM-DD in the user's timezone
	TitleSnapshot     string
	XPRewardSnapshot  int
	Status            DailyMissionStatus
	CompletedAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// XPTransactionType distinguishes an XP grant from its exact reversal.
type XPTransactionType string

const (
	XPTransactionComplete   XPTransactionType = "COMPLETE"
	XPTransactionUncomplete XPTransactionType = "UNCOMPLETE"
)

// XPTransaction is an immutable ledger entry. A user's total XP is always
// derived by summing these entries — it is never stored/mutated directly.
type XPTransaction struct {
	ID              string
	UserID          string
	DailyMissionID  string
	Amount          int
	TransactionType XPTransactionType
	CreatedAt       time.Time
}

// Session is a server-side login session. Only a SHA-256 hash of the bearer
// token is stored; the raw token exists solely in the client's httpOnly
// cookie and is never persisted.
type Session struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}
