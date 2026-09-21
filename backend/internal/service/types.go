package service

import (
	"levelog/backend/internal/levelup"
	"levelog/backend/internal/model"
)

// TemplateInput is the validated input for creating/updating a mission
// template. XP is deliberately not part of this struct — it is always
// derived from Difficulty server-side.
type TemplateInput struct {
	Title       string
	Description string
	Difficulty  model.Difficulty
	Days        []model.Weekday
	Active      bool
}

type TodayResult struct {
	Date           string
	Missions       []model.DailyMission
	CompletedCount int
	TotalCount     int
	Progress       levelup.Progress
}

type DayHistory struct {
	Date           string
	Missions       []model.DailyMission
	CompletedCount int
	TotalCount     int
	XPEarned       int
}

type CompleteResult struct {
	Mission   model.DailyMission
	XPGained  int
	LeveledUp bool
	Progress  levelup.Progress
}

type UncompleteResult struct {
	Mission  model.DailyMission
	Progress levelup.Progress
}

type StatsResult struct {
	Progress       levelup.Progress
	TodayCompleted int
	TodayTotal     int
}
