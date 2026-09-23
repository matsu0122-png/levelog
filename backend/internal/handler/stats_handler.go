package handler

import (
	"net/http"

	"levelog/backend/internal/httpx"
	"levelog/backend/internal/middleware"
	"levelog/backend/internal/service"
)

type StatsHandler struct {
	missions *service.MissionService
	xp       *service.XPService
}

func NewStatsHandler(missions *service.MissionService, xp *service.XPService) *StatsHandler {
	return &StatsHandler{missions: missions, xp: xp}
}

type statsResponse struct {
	Progress       progressResponse `json:"progress"`
	TodayCompleted int              `json:"todayCompleted"`
	TodayTotal     int              `json:"todayTotal"`
}

func (h *StatsHandler) Stats(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	result, err := h.missions.GetStats(r.Context(), user.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, statsResponse{
		Progress:       toProgressResponse(result.Progress),
		TodayCompleted: result.TodayCompleted,
		TodayTotal:     result.TodayTotal,
	})
}

type xpTransactionResponse struct {
	ID              string `json:"id"`
	DailyMissionID  string `json:"dailyMissionId"`
	Amount          int    `json:"amount"`
	TransactionType string `json:"transactionType"`
	CreatedAt       string `json:"createdAt"`
}

func (h *StatsHandler) XPHistory(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	txns, err := h.xp.ListRecent(r.Context(), user.ID, 50)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]xpTransactionResponse, len(txns))
	for i, tx := range txns {
		out[i] = xpTransactionResponse{
			ID:              tx.ID,
			DailyMissionID:  tx.DailyMissionID,
			Amount:          tx.Amount,
			TransactionType: string(tx.TransactionType),
			CreatedAt:       tx.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}
