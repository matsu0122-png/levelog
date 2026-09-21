package handler

import (
	"net/http"
	"strconv"

	"levelog/backend/internal/httpx"
	"levelog/backend/internal/middleware"
	"levelog/backend/internal/service"
)

type DailyMissionHandler struct {
	missions *service.MissionService
}

func NewDailyMissionHandler(missions *service.MissionService) *DailyMissionHandler {
	return &DailyMissionHandler{missions: missions}
}

type todayResponse struct {
	Date           string                 `json:"date"`
	Missions       []dailyMissionResponse `json:"missions"`
	CompletedCount int                    `json:"completedCount"`
	TotalCount     int                    `json:"totalCount"`
	Progress       progressResponse       `json:"progress"`
}

func (h *DailyMissionHandler) Today(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	result, err := h.missions.GetToday(r.Context(), user.ID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, todayResponse{
		Date:           result.Date,
		Missions:       toDailyMissionResponses(result.Missions),
		CompletedCount: result.CompletedCount,
		TotalCount:     result.TotalCount,
		Progress:       toProgressResponse(result.Progress),
	})
}

type historyDayResponse struct {
	Date           string                 `json:"date"`
	Missions       []dailyMissionResponse `json:"missions"`
	CompletedCount int                    `json:"completedCount"`
	TotalCount     int                    `json:"totalCount"`
	XPEarned       int                    `json:"xpEarned"`
}

func (h *DailyMissionHandler) History(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	days := 7
	if v := r.URL.Query().Get("days"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 31 {
			days = parsed
		}
	}
	result, err := h.missions.GetHistory(r.Context(), user.ID, days)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]historyDayResponse, len(result))
	for i, d := range result {
		out[i] = historyDayResponse{
			Date:           d.Date,
			Missions:       toDailyMissionResponses(d.Missions),
			CompletedCount: d.CompletedCount,
			TotalCount:     d.TotalCount,
			XPEarned:       d.XPEarned,
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type completeResponse struct {
	Mission   dailyMissionResponse `json:"mission"`
	XPGained  int                  `json:"xpGained"`
	LeveledUp bool                 `json:"leveledUp"`
	Progress  progressResponse     `json:"progress"`
}

func (h *DailyMissionHandler) Complete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id := r.PathValue("id")
	result, err := h.missions.CompleteDailyMission(r.Context(), user.ID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, completeResponse{
		Mission:   toDailyMissionResponse(result.Mission),
		XPGained:  result.XPGained,
		LeveledUp: result.LeveledUp,
		Progress:  toProgressResponse(result.Progress),
	})
}

type uncompleteResponse struct {
	Mission  dailyMissionResponse `json:"mission"`
	Progress progressResponse     `json:"progress"`
}

func (h *DailyMissionHandler) Uncomplete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id := r.PathValue("id")
	result, err := h.missions.UncompleteDailyMission(r.Context(), user.ID, id)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, uncompleteResponse{
		Mission:  toDailyMissionResponse(result.Mission),
		Progress: toProgressResponse(result.Progress),
	})
}
