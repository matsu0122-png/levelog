package service

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"levelog/backend/internal/apperror"
	"levelog/backend/internal/model"
)

// fakeStore is an in-memory implementation of every repository interface
// used by the service package, so business rules (XP idempotency, ownership,
// same-day-only edits, etc.) can be unit tested without a real Postgres
// instance. It mirrors the atomicity guarantees the Postgres implementation
// provides via database transactions by holding a single mutex around each
// operation.
type fakeStore struct {
	mu sync.Mutex

	seq int

	usersByID    map[string]*model.User
	usersByEmail map[string]string // email -> id

	templates map[string]*model.MissionTemplate

	dailyMissions map[string]*model.DailyMission

	xpTxns []model.XPTransaction
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		usersByID:     map[string]*model.User{},
		usersByEmail:  map[string]string{},
		templates:     map[string]*model.MissionTemplate{},
		dailyMissions: map[string]*model.DailyMission{},
	}
}

func (f *fakeStore) nextID(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%d", prefix, f.seq)
}

// --- UserRepository ---

func (f *fakeStore) Create(ctx context.Context, u *model.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u.ID == "" {
		u.ID = f.nextID("user")
	}
	cp := *u
	f.usersByID[u.ID] = &cp
	f.usersByEmail[u.Email] = u.ID
	return nil
}

func (f *fakeStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.usersByEmail[email]
	if !ok {
		return nil, nil
	}
	cp := *f.usersByID[id]
	return &cp, nil
}

func (f *fakeStore) GetByID(ctx context.Context, id string) (*model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.usersByID[id]
	if !ok {
		return nil, nil
	}
	cp := *u
	return &cp, nil
}

// --- MissionTemplateRepository ---

func (f *fakeStore) CreateTemplate(t *model.MissionTemplate) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t.ID == "" {
		t.ID = f.nextID("tmpl")
	}
	cp := *t
	cp.ScheduleDays = append([]model.Weekday{}, t.ScheduleDays...)
	f.templates[t.ID] = &cp
}

func (f *fakeStore) TCreate(ctx context.Context, t *model.MissionTemplate) error {
	f.CreateTemplate(t)
	return nil
}

func (f *fakeStore) TUpdate(ctx context.Context, t *model.MissionTemplate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.templates[t.ID]; !ok {
		return apperror.NotFound("mission template not found")
	}
	cp := *t
	cp.ScheduleDays = append([]model.Weekday{}, t.ScheduleDays...)
	f.templates[t.ID] = &cp
	return nil
}

func (f *fakeStore) TGetByID(ctx context.Context, userID, id string) (*model.MissionTemplate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.templates[id]
	if !ok || t.UserID != userID || t.DeletedAt != nil {
		return nil, nil
	}
	cp := *t
	cp.ScheduleDays = append([]model.Weekday{}, t.ScheduleDays...)
	return &cp, nil
}

func (f *fakeStore) TListByUser(ctx context.Context, userID string) ([]model.MissionTemplate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.MissionTemplate
	for _, t := range f.templates {
		if t.UserID == userID && t.DeletedAt == nil {
			cp := *t
			cp.ScheduleDays = append([]model.Weekday{}, t.ScheduleDays...)
			out = append(out, cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) TListActiveByUserAndWeekday(ctx context.Context, userID string, weekday model.Weekday) ([]model.MissionTemplate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.MissionTemplate
	for _, t := range f.templates {
		if t.UserID != userID || t.DeletedAt != nil || !t.Active {
			continue
		}
		for _, d := range t.ScheduleDays {
			if d == weekday {
				cp := *t
				cp.ScheduleDays = append([]model.Weekday{}, t.ScheduleDays...)
				out = append(out, cp)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) TSetActive(ctx context.Context, userID, id string, active bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.templates[id]
	if !ok || t.UserID != userID || t.DeletedAt != nil {
		return apperror.NotFound("mission template not found")
	}
	t.Active = active
	return nil
}

func (f *fakeStore) TSoftDelete(ctx context.Context, userID, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.templates[id]
	if !ok || t.UserID != userID || t.DeletedAt != nil {
		return apperror.NotFound("mission template not found")
	}
	now := time.Now()
	t.DeletedAt = &now
	t.Active = false
	return nil
}

// --- DailyMissionRepository ---
// (method names prefixed with D to avoid colliding with UserRepository.GetByID)

func (f *fakeStore) DEnsureGenerated(ctx context.Context, userID, targetDate string, templates []model.MissionTemplate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	existing := map[string]bool{}
	for _, m := range f.dailyMissions {
		if m.UserID == userID && m.TargetDate == targetDate {
			existing[m.MissionTemplateID] = true
		}
	}
	for _, t := range templates {
		if existing[t.ID] {
			continue
		}
		id := f.nextID("daily")
		f.dailyMissions[id] = &model.DailyMission{
			ID:                id,
			UserID:            userID,
			MissionTemplateID: t.ID,
			TargetDate:        targetDate,
			TitleSnapshot:     t.Title,
			XPRewardSnapshot:  t.XPReward,
			Status:            model.DailyMissionPending,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
	}
	return nil
}

func (f *fakeStore) DListByUserAndDate(ctx context.Context, userID, date string) ([]model.DailyMission, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.DailyMission
	for _, m := range f.dailyMissions {
		if m.UserID == userID && m.TargetDate == date {
			out = append(out, *m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) DListByUserAndDateRange(ctx context.Context, userID string, dates []string) ([]model.DailyMission, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	want := map[string]bool{}
	for _, d := range dates {
		want[d] = true
	}
	var out []model.DailyMission
	for _, m := range f.dailyMissions {
		if m.UserID == userID && want[m.TargetDate] {
			out = append(out, *m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) DGetByID(ctx context.Context, userID, id string) (*model.DailyMission, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.dailyMissions[id]
	if !ok || m.UserID != userID {
		return nil, nil
	}
	cp := *m
	return &cp, nil
}

func (f *fakeStore) sumXPLocked(userID string) int {
	total := 0
	for _, tx := range f.xpTxns {
		if tx.UserID == userID {
			total += tx.Amount
		}
	}
	return total
}

func (f *fakeStore) DComplete(ctx context.Context, userID, dailyMissionID, today string) (MissionTransitionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, ok := f.dailyMissions[dailyMissionID]
	if !ok || m.UserID != userID {
		return MissionTransitionResult{}, apperror.NotFound("ミッションが見つかりません")
	}
	if m.TargetDate != today {
		return MissionTransitionResult{}, apperror.Forbidden("過去の日付のミッションは変更できません")
	}
	if m.Status == model.DailyMissionCompleted {
		return MissionTransitionResult{Mission: *m, XPDelta: 0, TotalXPAfter: f.sumXPLocked(userID)}, nil
	}

	now := time.Now()
	m.Status = model.DailyMissionCompleted
	m.CompletedAt = &now
	m.UpdatedAt = now

	f.xpTxns = append(f.xpTxns, model.XPTransaction{
		ID:              f.nextID("xp"),
		UserID:          userID,
		DailyMissionID:  dailyMissionID,
		Amount:          m.XPRewardSnapshot,
		TransactionType: model.XPTransactionComplete,
		CreatedAt:       now,
	})

	return MissionTransitionResult{Mission: *m, XPDelta: m.XPRewardSnapshot, TotalXPAfter: f.sumXPLocked(userID)}, nil
}

func (f *fakeStore) DUncomplete(ctx context.Context, userID, dailyMissionID, today string) (MissionTransitionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, ok := f.dailyMissions[dailyMissionID]
	if !ok || m.UserID != userID {
		return MissionTransitionResult{}, apperror.NotFound("ミッションが見つかりません")
	}
	if m.TargetDate != today {
		return MissionTransitionResult{}, apperror.Forbidden("過去の日付のミッションは変更できません")
	}
	if m.Status == model.DailyMissionPending {
		return MissionTransitionResult{Mission: *m, XPDelta: 0, TotalXPAfter: f.sumXPLocked(userID)}, nil
	}

	now := time.Now()
	reversedAmount := -m.XPRewardSnapshot
	m.Status = model.DailyMissionPending
	m.CompletedAt = nil
	m.UpdatedAt = now

	f.xpTxns = append(f.xpTxns, model.XPTransaction{
		ID:              f.nextID("xp"),
		UserID:          userID,
		DailyMissionID:  dailyMissionID,
		Amount:          reversedAmount,
		TransactionType: model.XPTransactionUncomplete,
		CreatedAt:       now,
	})

	return MissionTransitionResult{Mission: *m, XPDelta: reversedAmount, TotalXPAfter: f.sumXPLocked(userID)}, nil
}

// --- XPRepository ---

func (f *fakeStore) SumByUser(ctx context.Context, userID string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sumXPLocked(userID), nil
}

func (f *fakeStore) ListRecentByUser(ctx context.Context, userID string, limit int) ([]model.XPTransaction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []model.XPTransaction
	for _, tx := range f.xpTxns {
		if tx.UserID == userID {
			out = append(out, tx)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// --- interface adapter wrappers ---
// fakeStore implements UserRepository and the daily/xp interfaces directly.
// MissionTemplateRepository's method names collide with plain CRUD verbs we
// want to keep short above, so templateRepoAdapter exposes the interface
// under the exact interface method names.

type templateRepoAdapter struct{ s *fakeStore }

func (a templateRepoAdapter) Create(ctx context.Context, t *model.MissionTemplate) error {
	return a.s.TCreate(ctx, t)
}
func (a templateRepoAdapter) Update(ctx context.Context, t *model.MissionTemplate) error {
	return a.s.TUpdate(ctx, t)
}
func (a templateRepoAdapter) GetByID(ctx context.Context, userID, id string) (*model.MissionTemplate, error) {
	return a.s.TGetByID(ctx, userID, id)
}
func (a templateRepoAdapter) ListByUser(ctx context.Context, userID string) ([]model.MissionTemplate, error) {
	return a.s.TListByUser(ctx, userID)
}
func (a templateRepoAdapter) ListActiveByUserAndWeekday(ctx context.Context, userID string, weekday model.Weekday) ([]model.MissionTemplate, error) {
	return a.s.TListActiveByUserAndWeekday(ctx, userID, weekday)
}
func (a templateRepoAdapter) SetActive(ctx context.Context, userID, id string, active bool) error {
	return a.s.TSetActive(ctx, userID, id, active)
}
func (a templateRepoAdapter) SoftDelete(ctx context.Context, userID, id string) error {
	return a.s.TSoftDelete(ctx, userID, id)
}

type dailyRepoAdapter struct{ s *fakeStore }

func (a dailyRepoAdapter) EnsureGenerated(ctx context.Context, userID, targetDate string, templates []model.MissionTemplate) error {
	return a.s.DEnsureGenerated(ctx, userID, targetDate, templates)
}
func (a dailyRepoAdapter) ListByUserAndDate(ctx context.Context, userID, date string) ([]model.DailyMission, error) {
	return a.s.DListByUserAndDate(ctx, userID, date)
}
func (a dailyRepoAdapter) ListByUserAndDateRange(ctx context.Context, userID string, dates []string) ([]model.DailyMission, error) {
	return a.s.DListByUserAndDateRange(ctx, userID, dates)
}
func (a dailyRepoAdapter) GetByID(ctx context.Context, userID, id string) (*model.DailyMission, error) {
	return a.s.DGetByID(ctx, userID, id)
}
func (a dailyRepoAdapter) Complete(ctx context.Context, userID, dailyMissionID, today string) (MissionTransitionResult, error) {
	return a.s.DComplete(ctx, userID, dailyMissionID, today)
}
func (a dailyRepoAdapter) Uncomplete(ctx context.Context, userID, dailyMissionID, today string) (MissionTransitionResult, error) {
	return a.s.DUncomplete(ctx, userID, dailyMissionID, today)
}

// fakeClock is a controllable Clock for deterministic tests.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func newMissionServiceForTest(s *fakeStore, clock Clock) *MissionService {
	return NewMissionService(s, templateRepoAdapter{s: s}, dailyRepoAdapter{s: s}, s, clock)
}
