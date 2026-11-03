package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// ErrNotDrill means DrillAnswered was given a study card or other non-drill visit.
var ErrNotDrill = errors.New("session: visit is not a drill")

// DrillOutcome reports whether a right answer cleared the pair and the trailing
// non-slow streak length after the answer. INIT-027/SPEC-003.
type DrillOutcome struct {
	Cleared bool
	Streak  int
}

// DrillAnswered grades the served letter or pattern drill step. It writes one
// drill_answers row for the session's learner and clears the pair after
// DrillClearStreak right, non-slow answers. A wrong answer is not asked again
// in that sitting (INIT-028/SPEC-002). Pattern pairs use the same speed rule
// as letter picks (INIT-029/SPEC-005). INIT-027/SPEC-003.
func (s *Session) DrillAnswered(ctx context.Context, cardID int64, correct bool, latency time.Duration) (DrillOutcome, error) {
	if s == nil {
		return DrillOutcome{}, errors.New("session: session is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasLast || cardID != s.last.CardID {
		return DrillOutcome{}, fmt.Errorf("session: card %d: %w", cardID, ErrNotServed)
	}
	if s.last.Kind != KindLetter && s.last.Kind != KindPattern {
		return DrillOutcome{}, fmt.Errorf("session: card %d: %w", cardID, ErrNotDrill)
	}
	// Median before this answer, so Slow matches the clear-rule judgement.
	var medianMS int64
	if s.store != nil {
		latencies, lerr := s.store.CorrectDrillLatencies(ctx, s.userID, DrillMedianWindow)
		if lerr != nil {
			return DrillOutcome{}, lerr
		}
		medianMS = drillMedianMS(latencies)
	}
	s.hasLast = false
	s.peeked = false
	out, err := s.recordDrillAnswerLocked(ctx, correct, latency)
	if err != nil {
		s.hasLast = true
		return DrillOutcome{}, err
	}
	s.tally.Answered++
	if correct {
		s.tally.Right++
	}
	if correct && drillAnswerSlow(latency.Milliseconds(), medianMS) {
		s.tally.Slow++
	}
	// A wrong drill step is not put back into this sitting (INIT-028/SPEC-002).
	return out, nil
}

// recordDrillAnswerLocked writes the drill answer and applies the clear rule.
// The caller holds mu. It does not mark the serve or requeue. INIT-027/SPEC-003.
func (s *Session) recordDrillAnswerLocked(ctx context.Context, correct bool, latency time.Duration) (DrillOutcome, error) {
	if s.store == nil {
		return DrillOutcome{}, errors.New("session: store is not configured")
	}
	if s.clock == nil {
		return DrillOutcome{}, errors.New("session: clock is not configured")
	}
	storeKind, ok := drillStoreKind(s.last.Kind)
	if !ok {
		return DrillOutcome{}, fmt.Errorf("session: kind %q is not a drill", s.last.Kind)
	}
	key := s.last.Key
	ms := latency.Milliseconds()
	if ms < 0 {
		ms = 0
	}

	// Median is taken before this answer so a 10th timed right that is itself
	// slow is judged against the prior nine-or-more baseline (ac-4).
	latencies, err := s.store.CorrectDrillLatencies(ctx, s.userID, DrillMedianWindow)
	if err != nil {
		return DrillOutcome{}, err
	}
	medianMS := drillMedianMS(latencies)

	at := s.clock.Now()
	if err := s.store.InsertDrillAnswer(ctx, db.DrillAnswer{
		LearnerID: s.userID,
		Kind:      storeKind,
		Key:       key,
		Correct:   correct,
		LatencyMS: ms,
		At:        at,
	}); err != nil {
		return DrillOutcome{}, err
	}

	out := DrillOutcome{}
	if !correct {
		return out, nil
	}

	missAt, hasMiss, err := s.store.LastConfusionMiss(ctx, s.userID, storeKind, key)
	if err != nil {
		return DrillOutcome{}, err
	}
	clearAt, hasClear, err := s.store.LatestConfusionClear(ctx, s.userID, storeKind, key)
	if err != nil {
		return DrillOutcome{}, err
	}
	since := drillSince(missAt, hasMiss, clearAt, hasClear)
	answers, err := s.store.DrillAnswersSince(ctx, s.userID, storeKind, key, since)
	if err != nil {
		return DrillOutcome{}, err
	}
	streak := trailingClearStreak(answers, medianMS)
	out.Streak = streak
	if streak < DrillClearStreak {
		return out, nil
	}
	if err := s.store.InsertConfusionClear(ctx, s.userID, storeKind, key, at); err != nil {
		return DrillOutcome{}, err
	}
	s.dropDrillStepsLocked(s.last.Kind, key)
	out.Cleared = true
	s.tally.PairsCleared++
	return out, nil
}

func drillStoreKind(kind string) (string, bool) {
	switch kind {
	case KindLetter:
		return fsrs.MissLetterPair, true
	case KindPattern:
		return fsrs.MissAgreement, true
	default:
		return "", false
	}
}

func drillSince(missAt time.Time, hasMiss bool, clearAt time.Time, hasClear bool) time.Time {
	// The bound is the later of the last miss and the last clear. A clear
	// shares its timestamp with the answer that cleared the pair, and that
	// answer must not start the next streak. DrillAnswersSince is inclusive,
	// so the clear bound moves one nanosecond later.
	switch {
	case hasMiss && hasClear:
		if !missAt.After(clearAt) {
			return clearAt.Add(time.Nanosecond)
		}
		return missAt
	case hasMiss:
		return missAt
	case hasClear:
		return clearAt.Add(time.Nanosecond)
	default:
		return time.Time{}
	}
}

func drillMedianMS(latencies []int64) int64 {
	if len(latencies) < DrillMedianMin {
		return 0
	}
	sorted := append([]int64(nil), latencies...)
	slices.Sort(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

func trailingClearStreak(answers []db.DrillAnswer, medianMS int64) int {
	streak := 0
	for i := len(answers) - 1; i >= 0; i-- {
		a := answers[i]
		if !a.Correct || drillAnswerSlow(a.LatencyMS, medianMS) {
			break
		}
		streak++
	}
	return streak
}

func drillAnswerSlow(latencyMS, medianMS int64) bool {
	if medianMS <= 0 {
		return false
	}
	return latencyMS > int64(DrillSlowFactor)*medianMS
}

// dropDrillStepsLocked removes later visits of this pair after the current
// index. The caller holds mu. INIT-027/SPEC-003.
func (s *Session) dropDrillStepsLocked(kind, key string) {
	if s.i >= len(s.cards) {
		return
	}
	kept := s.cards[:s.i]
	for _, c := range s.cards[s.i:] {
		if c.Kind == kind && c.Key == key {
			continue
		}
		kept = append(kept, c)
	}
	s.cards = kept
}
