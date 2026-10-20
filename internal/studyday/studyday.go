// Package studyday is the learner-local study day.
//
// The day rolls over at RolloverHour in the learner's time zone, the way
// Anki does. Timestamps stay UTC in the database. This package only names
// the boundary.
package studyday

import (
	"log/slog"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // zone data for a distroless, CGO-free image
)

const (
	// RolloverHour is the local hour when a new study day starts.
	RolloverHour = 4

	// DefaultTZ is the zone used when a learner has none, or an unknown one.
	DefaultTZ = "America/Denver"

	// DueRule is the due cutoff for Today and Review.
	// "rollover" (the Anki rule) is the next 4:00 local.
	// "now" is the clock instant. Flip this one line to choose.
	DueRule = "rollover"
)

var (
	denverOnce sync.Once
	denverLoc  *time.Location

	fallbackOnce sync.Once
)

func denver() *time.Location {
	denverOnce.Do(func() {
		loc, err := time.LoadLocation(DefaultTZ)
		if err != nil {
			panic("studyday: " + DefaultTZ + ": " + err.Error())
		}
		denverLoc = loc
	})
	return denverLoc
}

// Load returns the IANA location for tz.
// An empty name is America/Denver. An unknown name is America/Denver too,
// and the first unknown name in the process is logged.
func Load(tz string) *time.Location {
	tz = strings.TrimSpace(tz)
	if tz == "" || tz == DefaultTZ {
		return denver()
	}
	loc, err := time.LoadLocation(tz)
	if err != nil || loc == nil {
		fallbackOnce.Do(func() {
			slog.Warn("studyday: invalid time zone, using America/Denver", "tz", tz, "err", err)
		})
		return denver()
	}
	return loc
}

// Start is the most recent RolloverHour:00 local at or before now.
// A nil loc is America/Denver.
func Start(now time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = denver()
	}
	local := now.In(loc)
	y, m, d := local.Date()
	start := time.Date(y, m, d, RolloverHour, 0, 0, 0, loc)
	if local.Before(start) {
		start = time.Date(y, m, d-1, RolloverHour, 0, 0, 0, loc)
	}
	return start
}

// Key is the YYYY-MM-DD of the study day that contains now.
// It is the local calendar date of Start.
func Key(now time.Time, loc *time.Location) string {
	return Start(now, loc).Format("2006-01-02")
}

// Next is the next rollover strictly after now: RolloverHour:00 local
// on the calendar day after Start.
func Next(now time.Time, loc *time.Location) time.Time {
	start := Start(now, loc)
	y, m, d := start.Date()
	return time.Date(y, m, d+1, RolloverHour, 0, 0, 0, start.Location())
}

// DueCutoff is the latest due time that still counts in the study day of now.
// Today’s due count and the Review queue both pass it to the same comparison
// (due <= cutoff), so they list the same cards.
//
// "rollover" returns the next 4:00 local. Review then serves every card due
// before that rollover, and a card due at 4:00 itself, which is the cutoff.
// "now" returns the clock instant.
func DueCutoff(now time.Time, loc *time.Location) time.Time {
	switch DueRule {
	case "now":
		return now
	default:
		return Next(now, loc)
	}
}
