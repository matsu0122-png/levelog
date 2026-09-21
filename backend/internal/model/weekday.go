package model

import (
	"fmt"
	"time"
)

// Weekday represents a day of the week as an ISO-8601 style integer:
// Monday = 1 ... Sunday = 7. This is distinct from Go's time.Weekday, which
// is zero-indexed starting at Sunday.
type Weekday int

const (
	Monday    Weekday = 1
	Tuesday   Weekday = 2
	Wednesday Weekday = 3
	Thursday  Weekday = 4
	Friday    Weekday = 5
	Saturday  Weekday = 6
	Sunday    Weekday = 7
)

// FromTimeWeekday converts a time.Weekday (Sunday=0..Saturday=6) into our
// Monday=1..Sunday=7 representation.
func FromTimeWeekday(w time.Weekday) Weekday {
	if w == time.Sunday {
		return Sunday
	}
	return Weekday(int(w))
}

// Valid reports whether w is one of the seven valid weekday values.
func (w Weekday) Valid() bool {
	return w >= Monday && w <= Sunday
}

var weekdayNames = map[Weekday]string{
	Monday:    "MONDAY",
	Tuesday:   "TUESDAY",
	Wednesday: "WEDNESDAY",
	Thursday:  "THURSDAY",
	Friday:    "FRIDAY",
	Saturday:  "SATURDAY",
	Sunday:    "SUNDAY",
}

func (w Weekday) String() string {
	if name, ok := weekdayNames[w]; ok {
		return name
	}
	return "UNKNOWN"
}

// ParseWeekday converts a weekday name (e.g. "MONDAY") back into a Weekday.
func ParseWeekday(name string) (Weekday, error) {
	for w, n := range weekdayNames {
		if n == name {
			return w, nil
		}
	}
	return 0, fmt.Errorf("unknown weekday: %q", name)
}
