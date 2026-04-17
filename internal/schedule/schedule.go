// Package schedule answers two questions about a config: is blocking active
// right now, and when does state change next.
package schedule

import (
	"time"

	"github.com/sebastien/deepwork/internal/config"
)

// IsActive returns whether any schedule fires at now, and the first matching schedule if so.
// Time matching is start-inclusive, end-exclusive.
func IsActive(c *config.Config, now time.Time) (bool, *config.Schedule) {
	now = now.In(c.Location)
	mod := now.Hour()*60 + now.Minute()
	wd := now.Weekday()
	for i := range c.Schedules {
		s := &c.Schedules[i]
		if _, ok := s.DaysSet[wd]; !ok {
			continue
		}
		if mod >= s.StartMin && mod < s.EndMin {
			return true, s
		}
	}
	return false, nil
}

// NextTransition returns the next moment (truncated to the minute) at which
// IsActive will flip, or ok=false if no flip is scheduled within the next 8 days.
//
// The minute-level brute-force scan is fine at this scale (11520 iterations max,
// O(schedules) per iteration — runs in microseconds even with dozens of schedules).
func NextTransition(c *config.Config, now time.Time) (time.Time, bool) {
	if len(c.Schedules) == 0 {
		return time.Time{}, false
	}
	cur, _ := IsActive(c, now)
	start := now.In(c.Location).Truncate(time.Minute)
	for i := 1; i <= 8*24*60; i++ {
		t := start.Add(time.Duration(i) * time.Minute)
		active, _ := IsActive(c, t)
		if active != cur {
			return t, true
		}
	}
	return time.Time{}, false
}
