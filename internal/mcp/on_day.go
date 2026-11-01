package mcp

import (
	"errors"
	"fmt"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// Same civil-day window as evidence.Taught (INIT-026/SPEC-005 / SPEC-008).
const maxOnAgeDays = 30

// resolveOnDay defaults a missing on to the learner's today and refuses a day
// after today or more than 30 days before it. No UTC invent-today.
func resolveOnDay(on string, now time.Time, z studyday.Zone) (string, error) {
	if z.Loc == nil {
		return "", errors.New("The learner day zone is not connected.")
	}
	if on == "" {
		return studyday.Today(now, z), nil
	}
	if _, err := time.Parse(time.DateOnly, on); err != nil {
		return "", errors.New("on must be YYYY-MM-DD.")
	}
	today := studyday.Today(now, z)
	if on > today {
		return "", fmt.Errorf("on %s is after today %s.", on, today)
	}
	todayT, err := time.ParseInLocation(time.DateOnly, today, z.Loc)
	if err != nil {
		return "", errors.New("on must be YYYY-MM-DD.")
	}
	onT, err := time.ParseInLocation(time.DateOnly, on, z.Loc)
	if err != nil {
		return "", errors.New("on must be YYYY-MM-DD.")
	}
	if onT.Before(todayT.AddDate(0, 0, -maxOnAgeDays)) {
		return "", fmt.Errorf("on %s is more than %d days before today.", on, maxOnAgeDays)
	}
	return on, nil
}
