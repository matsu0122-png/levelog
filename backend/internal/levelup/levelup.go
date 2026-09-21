// Package levelup contains the shared leveling logic used across the backend.
//
// Leveling rule: to go from level N to level N+1 requires N*100 XP.
// So the cumulative XP required to reach level L (starting at level 1 with 0 XP)
// is sum_{i=1}^{L-1} i*100 = 50*L*(L-1).
package levelup

// Progress describes a user's leveling state derived purely from total XP.
type Progress struct {
	Level          int `json:"level"`
	TotalXP        int `json:"totalXp"`
	XPIntoLevel    int `json:"xpIntoLevel"`
	XPForNextLevel int `json:"xpForNextLevel"`
}

// Calculate derives the current level, the XP accumulated within that level,
// and the XP required to reach the next level, from a total XP amount.
//
// Negative totals are treated as zero (defensive; XP should never go negative
// because completions and their exact-offset uncompletions net to zero).
func Calculate(totalXP int) Progress {
	if totalXP < 0 {
		totalXP = 0
	}

	level := 1
	remaining := totalXP
	for {
		needed := level * 100
		if remaining < needed {
			return Progress{
				Level:          level,
				TotalXP:        totalXP,
				XPIntoLevel:    remaining,
				XPForNextLevel: needed,
			}
		}
		remaining -= needed
		level++
	}
}
