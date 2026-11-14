package session

import (
	"math"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// A sitting is built once, from the learner's recent accuracy. That is the wrong
// shape for the thing that actually makes someone quit: two misses in a row in the
// next ninety seconds. The mix cannot see that, because the queue is already
// ordered. This file lets the sitting respond while it runs, and lets an easy run
// raise the next sitting.
//
// Two levers, both honest:
//
//   - Frustration trims the rest of the sitting. After FrustrationMisses misses in
//     a row, the visits still ahead that cost the most right now — typed production
//     and words the learner has never met — are taken out of this sitting. They are
//     not lost: they stay due, and the next sitting meets them after these misses
//     have raised the known-card share.
//   - Boredom raises the new-word ceiling on the next sitting. After BoredomRights
//     rights in a row, newAllowance treats the learner as the high new-card tier.
//     The known/struggle split is deliberately NOT touched: that split is ADR-0033's
//     target success rate, and a good run is not evidence that the learner wants a
//     harder mix — it is evidence that they can take more new words.

const (
	// FrustrationMisses is how many misses in a row ease the rest of a sitting.
	// Two, not three: the third miss is the one a learner remembers.
	FrustrationMisses = 2

	// BoredomRights is how many rights in a row mark the recent history as too
	// easy. Five is a comfortable run, long before a learner starts tapping through.
	BoredomRights = 5
)

// Stretched reports whether the newest ratings begin with a right streak at least
// BoredomRights long. It reads the same rows accuracy comes from, so the
// easy-sitting signal needs no extra table and cannot disagree with accuracy.
// Again, Hard, and Good all continue a streak; only Again ends it, because Hard
// and Good are still a correct answer.
func Stretched(ratings []int) bool {
	run := 0
	for _, rating := range ratings {
		if rating <= int(fsrs.Again) {
			return false
		}
		run++
		if run >= BoredomRights {
			return true
		}
	}
	return false
}

// stretchedShare is the accuracy that reaches the high new-card tier: one step
// above HighAccuracy. Only newAllowance reads it.
func stretchedShare() float64 { return math.Nextafter(HighAccuracy, 1) }

// trimForFrustration removes the visits that would cost the most right now from
// the part of the queue not yet served. A typed answer while frustrated is the
// worst of them, then a word the learner has never met. Drills stay. The caller
// holds mu.
func (s *Session) trimForFrustration() int {
	if s == nil || s.i >= len(s.cards) {
		return 0
	}
	kept := s.cards[:s.i:s.i]
	trimmed := 0
	for _, ref := range s.cards[s.i:] {
		if costlyWhileFrustrated(ref) {
			trimmed++
			continue
		}
		kept = append(kept, ref)
	}
	if trimmed == 0 {
		return 0
	}
	s.cards = kept
	return trimmed
}

// costlyWhileFrustrated is a visit worth postponing: production the learner has to
// type, or a word they have never been shown.
func costlyWhileFrustrated(ref CardRef) bool {
	if ref.Kind == KindLetter || ref.Kind == KindPattern {
		return false
	}
	if ref.Intro || ref.New {
		return true
	}
	switch ref.Activity {
	case activity.TypeIt, activity.Dictation:
		return true
	}
	return false
}

// temperMiss is the miss side of the streak, called with mu held.
func (s *Session) temperMiss() {
	s.missStreak++
	s.rightStreak = 0
	if s.missStreak >= FrustrationMisses {
		if n := s.trimForFrustration(); n > 0 {
			s.tally.Eased += n
		}
	}
}

// temperRight is the right side of the streak, called with mu held.
func (s *Session) temperRight() {
	s.rightStreak++
	s.missStreak = 0
	if s.rightStreak >= BoredomRights {
		s.stretched = true
	}
}

// RanStretched reports whether this sitting itself ran a long right streak.
func (s *Session) RanStretched() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stretched
}
