package handler

import (
	"time"

	"levelog/backend/internal/levelup"
	"levelog/backend/internal/model"
)

type progressResponse struct {
	Level          int `json:"level"`
	TotalXP        int `json:"totalXp"`
	XPIntoLevel    int `json:"xpIntoLevel"`
	XPForNextLevel int `json:"xpForNextLevel"`
}

func toProgressResponse(p levelup.Progress) progressResponse {
	return progressResponse{
		Level:          p.Level,
		TotalXP:        p.TotalXP,
		XPIntoLevel:    p.XPIntoLevel,
		XPForNextLevel: p.XPForNextLevel,
	}
}

type dailyMissionResponse struct {
	ID          string     `json:"id"`
	TemplateID  string     `json:"missionTemplateId"`
	TargetDate  string     `json:"targetDate"`
	Title       string     `json:"title"`
	XPReward    int        `json:"xpReward"`
	Status      string     `json:"status"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

func toDailyMissionResponse(m model.DailyMission) dailyMissionResponse {
	return dailyMissionResponse{
		ID:          m.ID,
		TemplateID:  m.MissionTemplateID,
		TargetDate:  m.TargetDate,
		Title:       m.TitleSnapshot,
		XPReward:    m.XPRewardSnapshot,
		Status:      string(m.Status),
		CompletedAt: m.CompletedAt,
	}
}

func toDailyMissionResponses(missions []model.DailyMission) []dailyMissionResponse {
	out := make([]dailyMissionResponse, len(missions))
	for i, m := range missions {
		out[i] = toDailyMissionResponse(m)
	}
	return out
}
