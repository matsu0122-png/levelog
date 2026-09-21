package repository

import (
	"context"
	"database/sql"
	"errors"

	"levelog/backend/internal/model"
)

type TemplateRepo struct {
	db *sql.DB
}

func NewTemplateRepo(db *sql.DB) *TemplateRepo {
	return &TemplateRepo{db: db}
}

func (r *TemplateRepo) Create(ctx context.Context, t *model.MissionTemplate) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
		INSERT INTO mission_templates (user_id, title, description, difficulty, xp_reward, active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`, t.UserID, t.Title, t.Description, string(t.Difficulty), t.XPReward, t.Active).
		Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return err
	}

	if err := insertScheduleDays(ctx, tx, t.ID, t.ScheduleDays); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *TemplateRepo) Update(ctx context.Context, t *model.MissionTemplate) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		UPDATE mission_templates
		SET title = $1, description = $2, difficulty = $3, xp_reward = $4, active = $5, updated_at = now()
		WHERE id = $6 AND user_id = $7 AND deleted_at IS NULL
	`, t.Title, t.Description, string(t.Difficulty), t.XPReward, t.Active, t.ID, t.UserID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM mission_schedule_days WHERE mission_template_id = $1`, t.ID); err != nil {
		return err
	}
	if err := insertScheduleDays(ctx, tx, t.ID, t.ScheduleDays); err != nil {
		return err
	}

	return tx.Commit()
}

func insertScheduleDays(ctx context.Context, tx *sql.Tx, templateID string, days []model.Weekday) error {
	for _, d := range days {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mission_schedule_days (mission_template_id, day_of_week) VALUES ($1, $2)
		`, templateID, int(d)); err != nil {
			return err
		}
	}
	return nil
}

func (r *TemplateRepo) scheduleDaysFor(ctx context.Context, templateID string) ([]model.Weekday, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT day_of_week FROM mission_schedule_days WHERE mission_template_id = $1 ORDER BY day_of_week
	`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var days []model.Weekday
	for rows.Next() {
		var d int
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		days = append(days, model.Weekday(d))
	}
	return days, rows.Err()
}

func scanTemplate(row interface {
	Scan(dest ...interface{}) error
}) (*model.MissionTemplate, error) {
	var t model.MissionTemplate
	var difficulty string
	var deletedAt sql.NullTime
	err := row.Scan(&t.ID, &t.UserID, &t.Title, &t.Description, &difficulty, &t.XPReward, &t.Active, &deletedAt, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	t.Difficulty = model.Difficulty(difficulty)
	if deletedAt.Valid {
		dt := deletedAt.Time
		t.DeletedAt = &dt
	}
	return &t, nil
}

func (r *TemplateRepo) GetByID(ctx context.Context, userID, id string) (*model.MissionTemplate, error) {
	t, err := scanTemplate(r.db.QueryRowContext(ctx, `
		SELECT id, user_id, title, description, difficulty, xp_reward, active, deleted_at, created_at, updated_at
		FROM mission_templates WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	days, err := r.scheduleDaysFor(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	t.ScheduleDays = days
	return t, nil
}

func (r *TemplateRepo) ListByUser(ctx context.Context, userID string) ([]model.MissionTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, title, description, difficulty, xp_reward, active, deleted_at, created_at, updated_at
		FROM mission_templates WHERE user_id = $1 AND deleted_at IS NULL ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.MissionTemplate
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		days, err := r.scheduleDaysFor(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		t.ScheduleDays = days
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (r *TemplateRepo) ListActiveByUserAndWeekday(ctx context.Context, userID string, weekday model.Weekday) ([]model.MissionTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.user_id, t.title, t.description, t.difficulty, t.xp_reward, t.active, t.deleted_at, t.created_at, t.updated_at
		FROM mission_templates t
		JOIN mission_schedule_days d ON d.mission_template_id = t.id
		WHERE t.user_id = $1 AND t.active = true AND t.deleted_at IS NULL AND d.day_of_week = $2
		ORDER BY t.created_at
	`, userID, int(weekday))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.MissionTemplate
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (r *TemplateRepo) SetActive(ctx context.Context, userID, id string, active bool) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE mission_templates SET active = $1, updated_at = now()
		WHERE id = $2 AND user_id = $3 AND deleted_at IS NULL
	`, active, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *TemplateRepo) SoftDelete(ctx context.Context, userID, id string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE mission_templates SET deleted_at = now(), active = false, updated_at = now()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
	`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
