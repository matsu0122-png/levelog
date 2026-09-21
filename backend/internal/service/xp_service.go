package service

import (
	"context"

	"levelog/backend/internal/model"
)

// XPService provides read access to a user's XP ledger history.
type XPService struct {
	xp XPRepository
}

func NewXPService(xp XPRepository) *XPService {
	return &XPService{xp: xp}
}

func (s *XPService) ListRecent(ctx context.Context, userID string, limit int) ([]model.XPTransaction, error) {
	return s.xp.ListRecentByUser(ctx, userID, limit)
}
