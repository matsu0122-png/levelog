package handler

import (
	"net/http"
	"time"

	"levelog/backend/internal/httpx"
	"levelog/backend/internal/middleware"
	"levelog/backend/internal/model"
	"levelog/backend/internal/service"
)

type MissionHandler struct {
	missions *service.MissionService
}

func NewMissionHandler(missions *service.MissionService) *MissionHandler {
	return &MissionHandler{missions: missions}
}

type templateRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Difficulty  string   `json:"difficulty"`
	Days        []string `json:"days"`
	Active      bool     `json:"active"`
}

type templateResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Difficulty  string    `json:"difficulty"`
	XPReward    int       `json:"xpReward"`
	Active      bool      `json:"active"`
	Days        []string  `json:"days"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func toTemplateResponse(t model.MissionTemplate) templateResponse {
	days := make([]string, len(t.ScheduleDays))
	for i, d := range t.ScheduleDays {
		days[i] = d.String()
	}
	return templateResponse{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Difficulty:  string(t.Difficulty),
		XPReward:    t.XPReward,
		Active:      t.Active,
		Days:        days,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func (req templateRequest) toInput() (service.TemplateInput, error) {
	days := make([]model.Weekday, 0, len(req.Days))
	for _, d := range req.Days {
		w, err := model.ParseWeekday(d)
		if err != nil {
			return service.TemplateInput{}, err
		}
		days = append(days, w)
	}
	return service.TemplateInput{
		Title:       req.Title,
		Description: req.Description,
		Difficulty:  model.Difficulty(req.Difficulty),
		Days:        days,
		Active:      req.Active,
	}, nil
}

func (h *MissionHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	templates, err := h.missions.ListTemplates(r.Context(), user.ID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	out := make([]templateResponse, len(templates))
	for i, t := range templates {
		out[i] = toTemplateResponse(t)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *MissionHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	var req templateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	input, err := req.toInput()
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	t, err := h.missions.CreateTemplate(r.Context(), user.ID, input)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	t.ScheduleDays = input.Days
	httpx.WriteJSON(w, http.StatusCreated, toTemplateResponse(*t))
}

func (h *MissionHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id := r.PathValue("id")
	var req templateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	input, err := req.toInput()
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	t, err := h.missions.UpdateTemplate(r.Context(), user.ID, id, input)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	t.ScheduleDays = input.Days
	httpx.WriteJSON(w, http.StatusOK, toTemplateResponse(*t))
}

func (h *MissionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id := r.PathValue("id")
	if err := h.missions.DeleteTemplate(r.Context(), user.ID, id); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type setActiveRequest struct {
	Active bool `json:"active"`
}

func (h *MissionHandler) SetActive(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id := r.PathValue("id")
	var req setActiveRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	if err := h.missions.SetActive(r.Context(), user.ID, id, req.Active); err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
