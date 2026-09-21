package service

import "time"

const dateLayout = "2006-01-02"

// TodayFor returns the current calendar date (YYYY-MM-DD) as observed in the
// given IANA timezone, based on now. All "what day is it" decisions in this
// app go through this function so that a user's day boundary always matches
// their own timezone, not the server's.
func TodayFor(timezone string, now time.Time) (string, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return "", err
	}
	return now.In(loc).Format(dateLayout), nil
}

// LastNDates returns the last n calendar dates (YYYY-MM-DD, oldest first)
// ending at and including `today`, in the given timezone.
func LastNDates(timezone string, now time.Time, n int) ([]string, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}
	today := now.In(loc)
	dates := make([]string, n)
	for i := 0; i < n; i++ {
		d := today.AddDate(0, 0, -(n - 1 - i))
		dates[i] = d.Format(dateLayout)
	}
	return dates, nil
}

// WeekdayFor returns the ISO-style weekday (Monday=1..Sunday=7) for `now`
// observed in the given timezone.
func WeekdayFor(timezone string, now time.Time) (time.Weekday, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return 0, err
	}
	return now.In(loc).Weekday(), nil
}
