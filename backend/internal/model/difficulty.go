package model

import "fmt"

// Difficulty is a fixed difficulty tier. The XP reward for each tier is fixed
// by the server — clients cannot choose their own XP value.
type Difficulty string

const (
	DifficultyEasy    Difficulty = "EASY"
	DifficultyNormal  Difficulty = "NORMAL"
	DifficultyHard    Difficulty = "HARD"
	DifficultyExtreme Difficulty = "EXTREME"
)

// XPReward returns the fixed XP reward for a difficulty tier.
func (d Difficulty) XPReward() (int, error) {
	switch d {
	case DifficultyEasy:
		return 10, nil
	case DifficultyNormal:
		return 20, nil
	case DifficultyHard:
		return 40, nil
	case DifficultyExtreme:
		return 80, nil
	default:
		return 0, fmt.Errorf("unknown difficulty: %q", d)
	}
}

// Valid reports whether d is one of the known difficulty tiers.
func (d Difficulty) Valid() bool {
	_, err := d.XPReward()
	return err == nil
}
