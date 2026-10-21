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

	// LearnAhead is how far past now a learning or relearning card may be due
	// and still be served. It is Anki's learn-ahead limit. Review and new
	// cards do not use it; they use the study-day cutoff.
	LearnAhead = 20 * time.Minute
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

// DueCutoff is the next RolloverHour:00 local after now. Progress and the
// activity day checks use this instant. Review-state and new cards are due
// for Today and Review when their due time is at or before it. Learning and
// relearning cards use CardDue instead. A nil loc is DefaultZone.
func DueCutoff(now time.Time, loc *time.Location) time.Time {
	return Next(now, loc)
}

// CardDue is the one predicate Today and Review share.
// A learning or relearning card is due only at or before now plus LearnAhead.
// A review card, a new card, and any other state are due at or before
// reviewLimit, which callers set to DueCutoff. The comparison is inclusive.
// The state strings are fsrs.StateLearning and fsrs.StateRelearning. This
// package cannot import fsrs: that package imports db, and db imports studyday.
func CardDue(state string, due, now, reviewLimit time.Time) bool {
	if state == "learning" || state == "relearning" {
		return !due.After(now.Add(LearnAhead))
	}
	return !due.After(reviewLimit)
}

// DueFetchLimit is the latest due time a due-card query must load so CardDue
// can see every card it might count. It is the later of the study-day cutoff
// and now+LearnAhead. A learning card in the last LearnAhead minutes before
// rollover is due after the cutoff, and a review card is not.
func DueFetchLimit(now time.Time, loc *time.Location) time.Time {
	cutoff := DueCutoff(now, loc)
	ahead := now.Add(LearnAhead)
	if ahead.After(cutoff) {
		return ahead
	}
	return cutoff
}
