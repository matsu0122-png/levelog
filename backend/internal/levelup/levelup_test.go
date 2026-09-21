package levelup

import "testing"

func TestCalculate(t *testing.T) {
	cases := []struct {
		name    string
		totalXP int
		want    Progress
	}{
		{"zero xp starts at level 1", 0, Progress{Level: 1, TotalXP: 0, XPIntoLevel: 0, XPForNextLevel: 100}},
		{"just below level 2 threshold", 99, Progress{Level: 1, TotalXP: 99, XPIntoLevel: 99, XPForNextLevel: 100}},
		{"exactly level 2 threshold", 100, Progress{Level: 2, TotalXP: 100, XPIntoLevel: 0, XPForNextLevel: 200}},
		{"mid level 2", 250, Progress{Level: 2, TotalXP: 250, XPIntoLevel: 150, XPForNextLevel: 200}},
		{"exactly level 3 threshold (100+200)", 300, Progress{Level: 3, TotalXP: 300, XPIntoLevel: 0, XPForNextLevel: 300}},
		{"exactly level 4 threshold (100+200+300)", 600, Progress{Level: 4, TotalXP: 600, XPIntoLevel: 0, XPForNextLevel: 400}},
		{"negative xp clamps to zero", -50, Progress{Level: 1, TotalXP: 0, XPIntoLevel: 0, XPForNextLevel: 100}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Calculate(tc.totalXP)
			if got != tc.want {
				t.Errorf("Calculate(%d) = %+v, want %+v", tc.totalXP, got, tc.want)
			}
		})
	}
}
