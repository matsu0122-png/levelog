package service

import (
	"context"
	"sort"
	"strings"

	"levelog/backend/internal/apperror"
	"levelog/backend/internal/levelup"
	"levelog/backend/internal/model"
)

const maxTitleLength = 100
const maxDescriptionLength = 500

type MissionService struct {
	users     UserRepository
	templates MissionTemplateRepository
	daily     DailyMissionRepository
	xp        XPRepository
	clock     Clock
}

func NewMissionService(users UserRepository, templates MissionTemplateRepository, daily DailyMissionRepository, xp XPRepository, clock Clock) *MissionService {
	if clock == nil {
		clock = RealClock
	}
	return &MissionService{users: users, templates: templates, daily: daily, xp: xp, clock: clock}
}

func validateTemplateInput(in TemplateInput) (TemplateInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return in, apperror.BadRequest("ミッション名を入力してください")
	}
	if len([]rune(in.Title)) > maxTitleLength {
		return in, apperror.BadRequest("ミッション名は100文字以内で入力してください")
	}
	if len([]rune(in.Description)) > maxDescriptionLength {
		return in, apperror.BadRequest("説明は500文字以内で入力してください")
	}
	if !in.Difficulty.Valid() {
		return in, apperror.BadRequest("難易度が不正です")
	}
	if len(in.Days) == 0 {
		return in, apperror.BadRequest("実行する曜日を1つ以上選択してください")
	}
	seen := map[model.Weekday]bool{}
	days := make([]model.Weekday, 0, len(in.Days))
	for _, d := range in.Days {
		if !d.Valid() {
			return in, apperror.BadRequest("曜日の指定が不正です")
		}
		if !seen[d] {
			seen[d] = true
			days = append(days, d)
		}
	}
	sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })
	in.Days = days
	return in, nil
}

func (s *MissionService) CreateTemplate(ctx context.Context, userID string, in TemplateInput) (*model.MissionTemplate, error) {
	in, err := validateTemplateInput(in)
	if err != nil {
		return nil, err
	}
	xpReward, err := in.Difficulty.XPReward()
	if err != nil {
		return nil, apperror.BadRequest("難易度が不正です")
	}

	now := s.clock.Now()
	t := &model.MissionTemplate{
		UserID:       userID,
		Title:        in.Title,
		Description:  in.Description,
		Difficulty:   in.Difficulty,
		XPReward:     xpReward,
		Active:       in.Active,
		ScheduleDays: in.Days,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.templates.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *MissionService) UpdateTemplate(ctx context.Context, userID, id string, in TemplateInput) (*model.MissionTemplate, error) {
	existing, err := s.templates.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, apperror.NotFound("ミッションが見つかりません")
	}

	in, err = validateTemplateInput(in)
	if err != nil {
		return nil, err
	}
	xpReward, err := in.Difficulty.XPReward()
	if err != nil {
		return nil, apperror.BadRequest("難易度が不正です")
	}

	existing.Title = in.Title
	existing.Description = in.Description
	existing.Difficulty = in.Difficulty
	existing.XPReward = xpReward
	existing.Active = in.Active
	existing.ScheduleDays = in.Days
	existing.UpdatedAt = s.clock.Now()

	if err := s.templates.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *MissionService) ListTemplates(ctx context.Context, userID string) ([]model.MissionTemplate, error) {
	return s.templates.ListByUser(ctx, userID)
}

func (s *MissionService) SetActive(ctx context.Context, userID, id string, active bool) error {
	existing, err := s.templates.GetByID(ctx, userID, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return apperror.NotFound("ミッションが見つかりません")
	}
	return s.templates.SetActive(ctx, userID, id, active)
}

func (s *MissionService) DeleteTemplate(ctx context.Context, userID, id string) error {
	existing, err := s.templates.GetByID(ctx, userID, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return apperror.NotFound("ミッションが見つかりません")
	}
	return s.templates.SoftDelete(ctx, userID, id)
}

// today resolves the current calendar date for userID's timezone, along with
// the ISO weekday, and generates any daily missions that are due but not yet
// created for that date.
func (s *MissionService) ensureTodayGenerated(ctx context.Context, userID string) (user *model.User, today string, err error) {
	user, err = s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if user == nil {
		return nil, "", apperror.Unauthorized("認証が必要です")
	}
	now := s.clock.Now()
	today, err = TodayFor(user.Timezone, now)
	if err != nil {
		return nil, "", apperror.Internal("invalid user timezone")
	}
	weekdayTime, err := WeekdayFor(user.Timezone, now)
	if err != nil {
		return nil, "", apperror.Internal("invalid user timezone")
	}
	weekday := model.FromTimeWeekday(weekdayTime)

	templates, err := s.templates.ListActiveByUserAndWeekday(ctx, userID, weekday)
	if err != nil {
		return nil, "", err
	}
	if len(templates) > 0 {
		if err := s.daily.EnsureGenerated(ctx, userID, today, templates); err != nil {
			return nil, "", err
		}
	}
	return user, today, nil
}

func (s *MissionService) GetToday(ctx context.Context, userID string) (TodayResult, error) {
	_, today, err := s.ensureTodayGenerated(ctx, userID)
	if err != nil {
		return TodayResult{}, err
	}
	missions, err := s.daily.ListByUserAndDate(ctx, userID, today)
	if err != nil {
		return TodayResult{}, err
	}
	completed := 0
	for _, m := range missions {
		if m.Status == model.DailyMissionCompleted {
			completed++
		}
	}
	totalXP, err := s.xp.SumByUser(ctx, userID)
	if err != nil {
		return TodayResult{}, err
	}
	return TodayResult{
		Date:           today,
		Missions:       missions,
		CompletedCount: completed,
		TotalCount:     len(missions),
		Progress:       levelup.Calculate(totalXP),
	}, nil
}

func (s *MissionService) GetStats(ctx context.Context, userID string) (StatsResult, error) {
	_, today, err := s.ensureTodayGenerated(ctx, userID)
	if err != nil {
		return StatsResult{}, err
	}
	missions, err := s.daily.ListByUserAndDate(ctx, userID, today)
	if err != nil {
		return StatsResult{}, err
	}
	completed := 0
	for _, m := range missions {
		if m.Status == model.DailyMissionCompleted {
			completed++
		}
	}
	totalXP, err := s.xp.SumByUser(ctx, userID)
	if err != nil {
		return StatsResult{}, err
	}
	return StatsResult{
		Progress:       levelup.Calculate(totalXP),
		TodayCompleted: completed,
		TodayTotal:     len(missions),
	}, nil
}

func (s *MissionService) GetHistory(ctx context.Context, userID string, days int) ([]DayHistory, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, apperror.Unauthorized("認証が必要です")
	}
	dates, err := LastNDates(user.Timezone, s.clock.Now(), days)
	if err != nil {
		return nil, apperror.Internal("invalid user timezone")
	}
	missions, err := s.daily.ListByUserAndDateRange(ctx, userID, dates)
	if err != nil {
		return nil, err
	}

	byDate := map[string][]model.DailyMission{}
	for _, m := range missions {
		byDate[m.TargetDate] = append(byDate[m.TargetDate], m)
	}

	result := make([]DayHistory, 0, len(dates))
	for _, d := range dates {
		dayMissions := byDate[d]
		completed := 0
		xpEarned := 0
		for _, m := range dayMissions {
			if m.Status == model.DailyMissionCompleted {
				completed++
				xpEarned += m.XPRewardSnapshot
			}
		}
		result = append(result, DayHistory{
			Date:           d,
			Missions:       dayMissions,
			CompletedCount: completed,
			TotalCount:     len(dayMissions),
			XPEarned:       xpEarned,
		})
	}
	return result, nil
}

func (s *MissionService) CompleteDailyMission(ctx context.Context, userID, dailyMissionID string) (CompleteResult, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return CompleteResult{}, err
	}
	if user == nil {
		return CompleteResult{}, apperror.Unauthorized("認証が必要です")
	}
	today, err := TodayFor(user.Timezone, s.clock.Now())
	if err != nil {
		return CompleteResult{}, apperror.Internal("invalid user timezone")
	}

	result, err := s.daily.Complete(ctx, userID, dailyMissionID, today)
	if err != nil {
		return CompleteResult{}, err
	}

	progressAfter := levelup.Calculate(result.TotalXPAfter)
	progressBefore := levelup.Calculate(result.TotalXPAfter - result.XPDelta)
	leveledUp := result.XPDelta > 0 && progressAfter.Level > progressBefore.Level

	return CompleteResult{
		Mission:   result.Mission,
		XPGained:  result.XPDelta,
		LeveledUp: leveledUp,
		Progress:  progressAfter,
	}, nil
}

func (s *MissionService) UncompleteDailyMission(ctx context.Context, userID, dailyMissionID string) (UncompleteResult, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return UncompleteResult{}, err
	}
	if user == nil {
		return UncompleteResult{}, apperror.Unauthorized("認証が必要です")
	}
	today, err := TodayFor(user.Timezone, s.clock.Now())
	if err != nil {
		return UncompleteResult{}, apperror.Internal("invalid user timezone")
	}

	result, err := s.daily.Uncomplete(ctx, userID, dailyMissionID, today)
	if err != nil {
		return UncompleteResult{}, err
	}

	return UncompleteResult{
		Mission:  result.Mission,
		Progress: levelup.Calculate(result.TotalXPAfter),
	}, nil
}
