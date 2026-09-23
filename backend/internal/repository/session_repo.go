package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"levelog/backend/internal/model"
)

type SessionRepo struct {
	db *sql.DB
}

func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) Create(ctx context.Context, s *model.Session) error {
	return r.db.QueryRowContext(ctx, `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, s.UserID, s.TokenHash, s.ExpiresAt).Scan(&s.ID, &s.CreatedAt)
}

func (r *SessionRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error) {
	var s model.Session
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM sessions WHERE token_hash = $1
	`, tokenHash).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SessionRepo) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

// DeleteExpired removes every session whose expires_at is before now,
// returning the number of rows removed. Called periodically (see
// service.RunSessionCleanupLoop) so the sessions table doesn't grow
// unbounded — nothing else prunes it (flagged as a gap since phase 2,
// closed in phase 18; see docs/production-roadmap.md).
func (r *SessionRepo) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < $1`, now)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
