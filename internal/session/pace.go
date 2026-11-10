package session

import (
	"context"
	"sort"
)

// sessionBudget is the card count and the planned minutes for one session.
// The count comes from the median seconds per answer over the last
// AnswerPaceWindow timed answers, or from DefaultSecondsPerAnswer when
// fewer exist (ADR-0033 D10, INIT-038/SPEC-006).
func (s *Service) sessionBudget(ctx context.Context, userID int64) (cards, minutes int, err error) {
	lats, err := s.store.RecentAnswerLatencies(ctx, userID, AnswerPaceWindow)
	if err != nil {
		return 0, 0, err
	}
	seconds := float64(DefaultSecondsPerAnswer)
	if len(lats) >= AnswerPaceWindow {
		seconds = medianSeconds(lats)
	}
	cards, minutes = CardsForPace(seconds)
	return cards, minutes, nil
}

// medianSeconds is the median of latencies in milliseconds, as seconds.
// An even count uses the mean of the two middle values.
func medianSeconds(ms []int64) float64 {
	if len(ms) == 0 {
		return DefaultSecondsPerAnswer
	}
	sorted := append([]int64(nil), ms...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	n := len(sorted)
	if n%2 == 1 {
		return float64(sorted[n/2]) / 1000
	}
	return float64(sorted[n/2-1]+sorted[n/2]) / 2 / 1000
}
