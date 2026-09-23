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

// ListByUser fetches every template's schedule days with one follow-up
// query (scheduleDaysForMany) rather than one per template. The obvious
// per-template loop (defer rows.Close(); for rows.Next() { ...
// r.scheduleDaysFor(t.ID) ... }) holds this outer query's connection
// checked out for the whole loop while also needing a second connection
// per iteration for the inner query — under concurrent load approaching
// the pool size (config.DBMaxOpenConns), that pattern starves the pool
// (every in-flight request holds one connection and blocks on a second),
// confirmed with a load test that collapsed throughput ~98% once
// concurrency passed roughly 1.5x the pool size (see
// docs/load-test-results.md). Closing rows before issuing any further
// query avoids that entirely, on top of turning 1+N queries into 2.
func (r *TemplateRepo) ListByUser(ctx context.Context, userID string) ([]model.MissionTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, title, description, difficulty, xp_reward, active, deleted_at, created_at, updated_at
		FROM mission_templates WHERE user_id = $1 AND deleted_at IS NULL ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}

	var out []model.MissionTemplate
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *t)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	if len(out) == 0 {
		return out, nil
	}

	ids := make([]string, len(out))
	for i, t := range out {
		ids[i] = t.ID
	}
	daysByTemplate, err := r.scheduleDaysForMany(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].ScheduleDays = daysByTemplate[out[i].ID]
	}
	return out, nil
}

// scheduleDaysForMany fetches schedule days for every template in
// templateIDs in a single query (Postgres array parameter, natively
// supported by the pgx driver — no pq.Array() needed), returning them
// grouped by template ID. Not called while any other query's *sql.Rows is
// still open — see ListByUser's comment above for why that matters.
func (r *TemplateRepo) scheduleDaysForMany(ctx context.Context, templateIDs []string) (map[string][]model.Weekday, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT mission_template_id, day_of_week FROM mission_schedule_days
		WHERE mission_template_id = ANY($1) ORDER BY mission_template_id, day_of_week
	`, templateIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]model.Weekday, len(templateIDs))
	for rows.Next() {
		var templateID string
		var d int
		if err := rows.Scan(&templateID, &d); err != nil {
			return nil, err
		}
		out[templateID] = append(out[templateID], model.Weekday(d))
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
