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

	// DefaultZone is the IANA zone for a learner with none, for an unknown
	// zone, and for the job scheduler when TZ is unset. The migration that
	// adds users.tz uses this same literal as its DEFAULT.
	DefaultZone = "America/Denver"
)

var (
	denverOnce sync.Once
	denverLoc  *time.Location

	fallbackOnce sync.Once
)

func denver() *time.Location {
	denverOnce.Do(func() {
		loc, err := time.LoadLocation(DefaultZone)
		if err != nil {
			panic("studyday: " + DefaultZone + ": " + err.Error())
		}
		denverLoc = loc
	})
	return denverLoc
}

// Load returns the IANA location for tz.
// An empty name is DefaultZone. An unknown name is DefaultZone too,
// and the first unknown name in the process is logged.
func Load(tz string) *time.Location {
	tz = strings.TrimSpace(tz)
	if tz == "" || tz == DefaultZone {
		return denver()
	}
	loc, err := time.LoadLocation(tz)
	if err != nil || loc == nil {
		fallbackOnce.Do(func() {
			slog.Warn("studyday: invalid time zone, using "+DefaultZone, "tz", tz, "err", err)
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

// DueCutoff is the next RolloverHour:00 local after now. Today, Progress,
// Review, and the activity day checks all use this instant, in the learner's
// zone. A card counts for the study day when its due time is at or before
// the cutoff, which is the next 4:00 local. A nil loc is DefaultZone.
func DueCutoff(now time.Time, loc *time.Location) time.Time {
	return Next(now, loc)
}
