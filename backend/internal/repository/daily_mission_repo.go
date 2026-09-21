package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"levelog/backend/internal/apperror"
	"levelog/backend/internal/model"
	"levelog/backend/internal/service"
)

const dateLayout = "2006-01-02"

type DailyMissionRepo struct {
	db *sql.DB
}

func NewDailyMissionRepo(db *sql.DB) *DailyMissionRepo {
	return &DailyMissionRepo{db: db}
}

func parseDate(s string) (time.Time, error) {
	return time.Parse(dateLayout, s)
}

func (r *DailyMissionRepo) EnsureGenerated(ctx context.Context, userID, targetDate string, templates []model.MissionTemplate) error {
	if len(templates) == 0 {
		return nil
	}
	date, err := parseDate(targetDate)
	if err != nil {
		return err
	}
	for _, t := range templates {
		if _, err := r.db.ExecContext(ctx, `
			INSERT INTO daily_missions (user_id, mission_template_id, target_date, title_snapshot, xp_reward_snapshot, status)
			VALUES ($1, $2, $3, $4, $5, 'PENDING')
			ON CONFLICT (user_id, mission_template_id, target_date) DO NOTHING
		`, userID, t.ID, date, t.Title, t.XPReward); err != nil {
			return err
		}
	}
	return nil
}

func scanDailyMission(row interface {
	Scan(dest ...interface{}) error
}) (*model.DailyMission, error) {
	var m model.DailyMission
	var targetDate time.Time
	var status string
	var completedAt sql.NullTime
	err := row.Scan(&m.ID, &m.UserID, &m.MissionTemplateID, &targetDate, &m.TitleSnapshot, &m.XPRewardSnapshot, &status, &completedAt, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, err
	}
	m.TargetDate = targetDate.Format(dateLayout)
	m.Status = model.DailyMissionStatus(status)
	if completedAt.Valid {
		ct := completedAt.Time
		m.CompletedAt = &ct
	}
	return &m, nil
}

const dailyMissionColumns = `id, user_id, mission_template_id, target_date, title_snapshot, xp_reward_snapshot, status, completed_at, created_at, updated_at`

func (r *DailyMissionRepo) ListByUserAndDate(ctx context.Context, userID, date string) ([]model.DailyMission, error) {
	d, err := parseDate(date)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+dailyMissionColumns+`
		FROM daily_missions WHERE user_id = $1 AND target_date = $2
		ORDER BY created_at
	`, userID, d)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.DailyMission
	for rows.Next() {
		m, err := scanDailyMission(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (r *DailyMissionRepo) ListByUserAndDateRange(ctx context.Context, userID string, dates []string) ([]model.DailyMission, error) {
	if len(dates) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(dates))
	args := make([]interface{}, 0, len(dates)+1)
	args = append(args, userID)
	for i, ds := range dates {
		d, err := parseDate(ds)
		if err != nil {
			return nil, err
		}
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, d)
	}
	query := fmt.Sprintf(`
		SELECT %s FROM daily_missions
		WHERE user_id = $1 AND target_date IN (%s)
		ORDER BY target_date, created_at
	`, dailyMissionColumns, joinStrings(placeholders, ", "))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.DailyMission
	for rows.Next() {
		m, err := scanDailyMission(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func joinStrings(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

func (r *DailyMissionRepo) GetByID(ctx context.Context, userID, id string) (*model.DailyMission, error) {
	m, err := scanDailyMission(r.db.QueryRowContext(ctx, `
		SELECT `+dailyMissionColumns+` FROM daily_missions WHERE id = $1 AND user_id = $2
	`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return m, nil
}

// transition implements the shared atomic core of Complete/Uncomplete: lock
// the row, verify ownership and date, and either no-op (already in the
// target status) or flip status + record the XP ledger entry — all inside
// one transaction.
func (r *DailyMissionRepo) transition(
	ctx context.Context,
	userID, dailyMissionID, today string,
	toStatus model.DailyMissionStatus,
	txType model.XPTransactionType,
	amountSign int,
) (service.MissionTransitionResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return service.MissionTransitionResult{}, err
	}
	defer tx.Rollback()

	m, err := scanDailyMission(tx.QueryRowContext(ctx, `
		SELECT `+dailyMissionColumns+` FROM daily_missions WHERE id = $1 FOR UPDATE
	`, dailyMissionID))
	if errors.Is(err, sql.ErrNoRows) {
		return service.MissionTransitionResult{}, apperror.NotFound("ミッションが見つかりません")
	}
	if err != nil {
		return service.MissionTransitionResult{}, err
	}
	if m.UserID != userID {
		return service.MissionTransitionResult{}, apperror.NotFound("ミッションが見つかりません")
	}
	if m.TargetDate != today {
		return service.MissionTransitionResult{}, apperror.Forbidden("過去の日付のミッションは変更できません")
	}

	if m.Status == toStatus {
		total, err := sumXPTx(ctx, tx, userID)
		if err != nil {
			return service.MissionTransitionResult{}, err
		}
		if err := tx.Commit(); err != nil {
			return service.MissionTransitionResult{}, err
		}
		return service.MissionTransitionResult{Mission: *m, XPDelta: 0, TotalXPAfter: total}, nil
	}

	var completedAtSQL interface{}
	if toStatus == model.DailyMissionCompleted {
		completedAtSQL = time.Now()
	} else {
		completedAtSQL = nil
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE daily_missions SET status = $1, completed_at = $2, updated_at = now() WHERE id = $3
	`, string(toStatus), completedAtSQL, dailyMissionID); err != nil {
		return service.MissionTransitionResult{}, err
	}

	amount := amountSign * m.XPRewardSnapshot
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO xp_transactions (user_id, daily_mission_id, amount, transaction_type)
		VALUES ($1, $2, $3, $4)
	`, userID, dailyMissionID, amount, string(txType)); err != nil {
		return service.MissionTransitionResult{}, err
	}

	total, err := sumXPTx(ctx, tx, userID)
	if err != nil {
		return service.MissionTransitionResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return service.MissionTransitionResult{}, err
	}

	m.Status = toStatus
	if toStatus == model.DailyMissionCompleted {
		ct := completedAtSQL.(time.Time)
		m.CompletedAt = &ct
	} else {
		m.CompletedAt = nil
	}

	return service.MissionTransitionResult{Mission: *m, XPDelta: amount, TotalXPAfter: total}, nil
}

func sumXPTx(ctx context.Context, tx *sql.Tx, userID string) (int, error) {
	var total int
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0) FROM xp_transactions WHERE user_id = $1`, userID).Scan(&total)
	return total, err
}

func (r *DailyMissionRepo) Complete(ctx context.Context, userID, dailyMissionID, today string) (service.MissionTransitionResult, error) {
	return r.transition(ctx, userID, dailyMissionID, today, model.DailyMissionCompleted, model.XPTransactionComplete, 1)
}

func (r *DailyMissionRepo) Uncomplete(ctx context.Context, userID, dailyMissionID, today string) (service.MissionTransitionResult, error) {
	return r.transition(ctx, userID, dailyMissionID, today, model.DailyMissionPending, model.XPTransactionUncomplete, -1)
}
