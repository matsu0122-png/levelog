package repository

import (
	"context"
	"database/sql"

	"levelog/backend/internal/model"
)

type XPRepo struct {
	db *sql.DB
}

func NewXPRepo(db *sql.DB) *XPRepo {
	return &XPRepo{db: db}
}

func (r *XPRepo) SumByUser(ctx context.Context, userID string) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM xp_transactions WHERE user_id = $1
	`, userID).Scan(&total)
	return total, err
}

func (r *XPRepo) ListRecentByUser(ctx context.Context, userID string, limit int) ([]model.XPTransaction, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, daily_mission_id, amount, transaction_type, created_at
		FROM xp_transactions WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.XPTransaction
	for rows.Next() {
		var tx model.XPTransaction
		var txType string
		if err := rows.Scan(&tx.ID, &tx.UserID, &tx.DailyMissionID, &tx.Amount, &txType, &tx.CreatedAt); err != nil {
			return nil, err
		}
		tx.TransactionType = model.XPTransactionType(txType)
		out = append(out, tx)
	}
	return out, rows.Err()
}
