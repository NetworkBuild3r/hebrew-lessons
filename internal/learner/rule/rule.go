// Package rule is the one copy of the known-word rule and the proved ladder.
// internal/db calls it, and package learner wraps it. Neither restates it.
// The package imports nothing from this module, so db can call it without a cycle.
package rule

import (
	"math"
	"time"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v3"
)

const (
	// LearningSuccesses is how many checked successes a card needs before the
	// same-day half of rule version 4 can apply.
	LearningSuccesses = 3
	// KnownRecentRight is how many of the newest outcomes rule version 4 reads.
	KnownRecentRight = 3
	// KnownRetrievability is the recall chance rule version 4 required.
	// The ladder does not read it.
	KnownRetrievability = 0.85
	// KnownStudyDays is the second-day bar of rule version 4.
	KnownStudyDays = 2
	// ProvedSuccessDays and ProvedSpanDays are the proved rung (ADR-0033 D4).
	ProvedSuccessDays = 3
	ProvedSpanDays    = 7
)

// Rungs of the proved ladder.
const (
	Proved        = "proved"
	RecalledLater = "recalled_later"
	Slipping      = "slipping"
	NotYetAsked   = "not_yet_asked"
)

// Input is one word in one skill. Dates are study days, YYYY-MM-DD.
// A voided answer is already left out by the caller.
type Input struct {
	Outcomes       int
	Recent         int
	RecentRight    int
	Successes      int
	SuccessDays    int
	HasCard        bool
	LatestRight    bool
	Taught         bool
	Stability      float64
	ElapsedDays    float64
	Retrievability float64 // set by the caller, or left 0 to compute from Stability and ElapsedDays
	First          string
	Last           string
	TaughtDay      string
}

// Chance is the FSRS recall chance. Scheduling uses it. The ladder does not.
func Chance(stability, elapsedDays float64) float64 {
	if stability <= 0 || math.IsNaN(stability) || math.IsNaN(elapsedDays) {
		return 0
	}
	if elapsedDays < 0 {
		elapsedDays = 0
	}
	p := gofsrs.DefaultParam()
	return math.Pow(1+p.Factor*elapsedDays/stability, p.Decay)
}

func (in Input) chance() float64 { return in.Retrievability }

// SameDayKnown is rule version 4 without the second study day.
func SameDayKnown(in Input) bool {
	return in.HasCard && in.Successes >= LearningSuccesses &&
		in.Recent >= KnownRecentRight && in.RecentRight >= KnownRecentRight &&
		in.chance() >= KnownRetrievability
}

// Rule4 is rule version 4, including two study days.
func Rule4(in Input) bool {
	return SameDayKnown(in) && in.SuccessDays >= KnownStudyDays
}

// Ladder is the proved rung. ok is false when the word is not taught.
// Recall chance is not read.
func Ladder(in Input) (string, bool) {
	if !in.Taught {
		return "", false
	}
	if in.Outcomes == 0 {
		return NotYetAsked, true
	}
	provedDays := in.SuccessDays >= ProvedSuccessDays && spanDays(in.First, in.Last) >= ProvedSpanDays
	later := in.SuccessDays > 0 && in.TaughtDay != "" && in.Last > in.TaughtDay
	switch {
	case in.LatestRight && provedDays:
		return Proved, true
	case in.LatestRight && later:
		return RecalledLater, true
	case !in.LatestRight && (provedDays || later):
		return Slipping, true
	default:
		return NotYetAsked, true
	}
}

// RecalledOrProved is the looser rung (ADR-0033 D6).
func RecalledOrProved(in Input) bool {
	rung, ok := Ladder(in)
	return ok && (rung == Proved || rung == RecalledLater)
}

func spanDays(first, last string) int {
	if first == "" || last == "" || last < first {
		return 0
	}
	a, err1 := time.Parse(time.DateOnly, first)
	b, err2 := time.Parse(time.DateOnly, last)
	if err1 != nil || err2 != nil {
		return 0
	}
	n := int(b.Sub(a).Hours() / 24)
	if n < 0 {
		return 0
	}
	return n
}
