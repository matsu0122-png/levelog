package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"levelog/backend/internal/apperror"
	"levelog/backend/internal/model"
)

func mustTokyoTime(t *testing.T, s string) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", s, loc)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return parsed
}

// setup creates a store, a Monday-scheduled EASY (10 XP) mission template for
// a fresh user, and a service whose clock starts on Monday 2024-01-15 09:00 JST.
func setup(t *testing.T) (ctx context.Context, store *fakeStore, svc *MissionService, clock *fakeClock, userID string) {
	t.Helper()
	ctx = context.Background()
	store = newFakeStore()
	clock = &fakeClock{now: mustTokyoTime(t, "2024-01-15 09:00:00")} // Monday
	svc = newMissionServiceForTest(store, clock)

	user := &model.User{Email: "runner@example.com", PasswordHash: "hash", Timezone: "Asia/Tokyo"}
	if err := store.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	userID = user.ID

	if _, err := svc.CreateTemplate(ctx, userID, TemplateInput{
		Title:      "Run 5km",
		Difficulty: model.DifficultyEasy,
		Days:       []model.Weekday{model.Monday},
		Active:     true,
	}); err != nil {
		t.Fatalf("create template: %v", err)
	}
	return ctx, store, svc, clock, userID
}

func todaysMissionID(t *testing.T, ctx context.Context, svc *MissionService, userID string) string {
	t.Helper()
	today, err := svc.GetToday(ctx, userID)
	if err != nil {
		t.Fatalf("get today: %v", err)
	}
	if len(today.Missions) != 1 {
		t.Fatalf("expected 1 mission generated, got %d", len(today.Missions))
	}
	return today.Missions[0].ID
}

func appErrorStatus(err error) int {
	var aerr *apperror.Error
	if errors.As(err, &aerr) {
		return aerr.Status
	}
	return 0
}

func TestCompleteDailyMission_GrantsCorrectXP(t *testing.T) {
	ctx, _, svc, _, userID := setup(t)
	missionID := todaysMissionID(t, ctx, svc, userID)

	result, err := svc.CompleteDailyMission(ctx, userID, missionID)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if result.XPGained != 10 {
		t.Errorf("XPGained = %d, want 10", result.XPGained)
	}
	if result.Mission.Status != model.DailyMissionCompleted {
		t.Errorf("status = %s, want COMPLETED", result.Mission.Status)
	}
	if result.Progress.TotalXP != 10 {
		t.Errorf("TotalXP = %d, want 10", result.Progress.TotalXP)
	}
}

func TestCompleteDailyMission_DoubleCompleteDoesNotDoubleGrantXP(t *testing.T) {
	ctx, _, svc, _, userID := setup(t)
	missionID := todaysMissionID(t, ctx, svc, userID)

	if _, err := svc.CompleteDailyMission(ctx, userID, missionID); err != nil {
		t.Fatalf("first complete: %v", err)
	}
	second, err := svc.CompleteDailyMission(ctx, userID, missionID)
	if err != nil {
		t.Fatalf("second complete: %v", err)
	}
	if second.XPGained != 0 {
		t.Errorf("second XPGained = %d, want 0", second.XPGained)
	}
	if second.Progress.TotalXP != 10 {
		t.Errorf("TotalXP after double complete = %d, want 10", second.Progress.TotalXP)
	}
}

func TestUncompleteDailyMission_OffsetsXPExactly(t *testing.T) {
	ctx, _, svc, _, userID := setup(t)
	missionID := todaysMissionID(t, ctx, svc, userID)

	if _, err := svc.CompleteDailyMission(ctx, userID, missionID); err != nil {
		t.Fatalf("complete: %v", err)
	}
	result, err := svc.UncompleteDailyMission(ctx, userID, missionID)
	if err != nil {
		t.Fatalf("uncomplete: %v", err)
	}
	if result.Mission.Status != model.DailyMissionPending {
		t.Errorf("status = %s, want PENDING", result.Mission.Status)
	}
	if result.Progress.TotalXP != 0 {
		t.Errorf("TotalXP after uncomplete = %d, want 0", result.Progress.TotalXP)
	}
}

func TestUncompleteDailyMission_RepeatedUncompleteDoesNotChangeXP(t *testing.T) {
	ctx, _, svc, _, userID := setup(t)
	missionID := todaysMissionID(t, ctx, svc, userID)

	if _, err := svc.CompleteDailyMission(ctx, userID, missionID); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if _, err := svc.UncompleteDailyMission(ctx, userID, missionID); err != nil {
		t.Fatalf("first uncomplete: %v", err)
	}
	second, err := svc.UncompleteDailyMission(ctx, userID, missionID)
	if err != nil {
		t.Fatalf("second uncomplete: %v", err)
	}
	if second.Progress.TotalXP != 0 {
		t.Errorf("TotalXP after repeated uncomplete = %d, want 0", second.Progress.TotalXP)
	}

	// A few more toggles should never drift away from a clean 0/10 oscillation.
	for i := 0; i < 3; i++ {
		if _, err := svc.CompleteDailyMission(ctx, userID, missionID); err != nil {
			t.Fatalf("toggle complete %d: %v", i, err)
		}
		if _, err := svc.UncompleteDailyMission(ctx, userID, missionID); err != nil {
			t.Fatalf("toggle uncomplete %d: %v", i, err)
		}
	}
	final, err := svc.GetStats(ctx, userID)
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if final.Progress.TotalXP != 0 {
		t.Errorf("TotalXP after repeated toggling = %d, want 0", final.Progress.TotalXP)
	}
}

func TestCannotModifyPastDateMission(t *testing.T) {
	ctx, _, svc, clock, userID := setup(t)
	missionID := todaysMissionID(t, ctx, svc, userID)

	// Move the clock to the next day; the mission generated for "today"
	// (2024-01-15) is now a past-date mission and must be immutable.
	clock.now = mustTokyoTime(t, "2024-01-16 09:00:00")

	_, err := svc.CompleteDailyMission(ctx, userID, missionID)
	if err == nil {
		t.Fatal("expected error completing a past-date mission, got nil")
	}
	if status := appErrorStatus(err); status != 403 {
		t.Errorf("status = %d, want 403", status)
	}
}

func TestCannotModifyOtherUsersMission(t *testing.T) {
	ctx, store, svc, _, userID := setup(t)
	missionID := todaysMissionID(t, ctx, svc, userID)

	otherUser := &model.User{Email: "other@example.com", PasswordHash: "hash", Timezone: "Asia/Tokyo"}
	if err := store.Create(ctx, otherUser); err != nil {
		t.Fatalf("create other user: %v", err)
	}

	_, err := svc.CompleteDailyMission(ctx, otherUser.ID, missionID)
	if err == nil {
		t.Fatal("expected error completing another user's mission, got nil")
	}
	if status := appErrorStatus(err); status != 404 {
		t.Errorf("status = %d, want 404", status)
	}
}

func TestDailyMissionsAreNotDuplicatedOnRepeatedGeneration(t *testing.T) {
	ctx, _, svc, _, userID := setup(t)

	first, err := svc.GetToday(ctx, userID)
	if err != nil {
		t.Fatalf("get today (1st): %v", err)
	}
	second, err := svc.GetToday(ctx, userID)
	if err != nil {
		t.Fatalf("get today (2nd): %v", err)
	}
	third, err := svc.GetStats(ctx, userID)
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}

	if len(first.Missions) != 1 || len(second.Missions) != 1 {
		t.Fatalf("expected exactly 1 mission each call, got %d and %d", len(first.Missions), len(second.Missions))
	}
	if first.Missions[0].ID != second.Missions[0].ID {
		t.Errorf("expected same mission ID across calls, got %s and %s", first.Missions[0].ID, second.Missions[0].ID)
	}
	if third.TodayTotal != 1 {
		t.Errorf("TodayTotal = %d, want 1", third.TodayTotal)
	}
}

func TestDifficultyXPMapping(t *testing.T) {
	ctx := context.Background()
	store := newFakeStore()
	clock := &fakeClock{now: mustTokyoTime(t, "2024-01-15 09:00:00")}
	svc := newMissionServiceForTest(store, clock)
	user := &model.User{Email: "diff@example.com", PasswordHash: "hash", Timezone: "Asia/Tokyo"}
	if err := store.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	cases := []struct {
		difficulty model.Difficulty
		wantXP     int
	}{
		{model.DifficultyEasy, 10},
		{model.DifficultyNormal, 20},
		{model.DifficultyHard, 40},
		{model.DifficultyExtreme, 80},
	}
	for _, tc := range cases {
		tmpl, err := svc.CreateTemplate(ctx, user.ID, TemplateInput{
			Title:      "Mission " + string(tc.difficulty),
			Difficulty: tc.difficulty,
			Days:       []model.Weekday{model.Monday},
			Active:     true,
		})
		if err != nil {
			t.Fatalf("create template %s: %v", tc.difficulty, err)
		}
		if tmpl.XPReward != tc.wantXP {
			t.Errorf("%s XPReward = %d, want %d", tc.difficulty, tmpl.XPReward, tc.wantXP)
		}
	}
}
