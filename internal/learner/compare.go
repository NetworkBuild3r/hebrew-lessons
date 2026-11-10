package learner

import (
	"context"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// Rule4Known counts lexemes KnownRule4 calls known in at least one skill.
// includeVoided counts answers an active void covers, which is the old number
// beside the ladder for the 14-day comparison (ADR-0033 D4). The ladder is
// not applied here. A word known in two skills counts once.
func (m *Model) Rule4Known(ctx context.Context, userID int64, includeVoided bool) (int, error) {
	if err := m.ready(userID); err != nil {
		return 0, err
	}
	now := m.clock.Now()
	q := m.query(now, 0)
	q.IncludeVoided = includeVoided
	ev, err := m.store.LearnerEvidence(ctx, userID, q)
	if err != nil {
		return 0, err
	}
	known := map[int64]bool{}
	for _, ls := range ev.Skills {
		if knownRule4Skill(ls, ev.Taught[ls.LexemeID], now) {
			known[ls.LexemeID] = true
		}
	}
	return len(known), nil
}

// knownRule4Skill builds the evidence the model already stores and asks KnownRule4.
// It does not restate the rule. taughtDay is unused by KnownRule4; it is set so
// the evidence matches the snapshot path.
func knownRule4Skill(ls db.LearnerSkill, taughtDay string, now time.Time) bool {
	e := Evidence{
		Outcomes: ls.Outcomes, Window: ls.Window, WindowCorrect: ls.WindowCorrect,
		AgainOf: ls.AgainOf, Agains: ls.Agains, Recent: ls.Recent, RecentRight: ls.RecentRight,
		HasCard: ls.HasCard, Successes: ls.ReviewSuccesses, Stability: ls.Stability,
		SuccessDays: ls.SuccessDays, FirstSuccessDay: ls.FirstSuccessDay, LastSuccessDay: ls.LastSuccessDay,
		LatestRight: ls.LatestRight,
	}
	if ls.HasCard && !ls.LastReview.IsZero() {
		e.Retrievability = fsrs.Retrievability(ls.Stability, now.Sub(ls.LastReview).Hours()/24)
	}
	if taughtDay != "" {
		e.Taught, e.TaughtDay = true, taughtDay
	}
	return KnownRule4(e)
}
