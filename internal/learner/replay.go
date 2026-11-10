package learner

import (
	"math"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// Replay compares stored cards to a fresh FSRS walk of review_log.
//
// Approach (INIT-033/SPEC-003): Snapshot.ReplayMismatch is filled on a live
// snapshot (Snapshot, or SnapshotAt with at not before the clock). The live
// path uses stored card fields for word state and still reports how
// many cards disagree with a full replay. Past days use the replay (or the
// creation state for an unreviewed card) and leave ReplayMismatch at 0. There
// is no separate ReplayCheck helper; the audit reads Snapshot.ReplayMismatch
// on a live Snapshot. CardHistories carries the stored schedule columns so the
// live mismatch check needs no per-card GetCard.

const (
	replayStabilityEps  = 1e-6
	replayDifficultyEps = 1e-6
	replayDueSlack      = time.Second
)

// replayScheduler is the same go-fsrs setup Grade uses (DefaultParam, fuzz off).
// A nil store is fine: ScheduleNext only needs the scheduler.
func replayScheduler() *fsrs.Review {
	return fsrs.New(nil, nil)
}

// replayHistory walks hist.Reviews oldest-first through ScheduleNext and the
// study-day cutoff, matching what Recorder/Grade/Implied store (ADR-0033 D10).
// Implied credit uses the same ScheduleNext call as a normal grade (Good for
// Recorder.Implied, Good or Hard for spelling); the Implied flag does not
// change the math, only the rating and ReviewedAt do.
func replayHistory(sched *fsrs.Review, hist db.CardHistory, zone studyday.Zone) (db.Card, bool, error) {
	if len(hist.Reviews) == 0 {
		return db.Card{}, false, nil
	}
	var prev *db.Card
	for _, r := range hist.Reviews {
		next, _, err := sched.ScheduleNext(prev, r.ReviewedAt, fsrs.Rating(r.Rating))
		if err != nil {
			return db.Card{}, false, err
		}
		next.Due = fsrs.SameDayDue(next.Due, r.ReviewedAt, zone)
		next.ID = hist.CardID
		next.LexemeID = hist.LexemeID
		next.Skill = hist.Skill
		prev = &next
	}
	return *prev, true, nil
}

func cardReplayMismatch(stored, replayed db.Card) bool {
	if stored.State != replayed.State {
		return true
	}
	if math.Abs(stored.Stability-replayed.Stability) > replayStabilityEps {
		return true
	}
	if math.Abs(stored.Difficulty-replayed.Difficulty) > replayDifficultyEps {
		return true
	}
	if stored.Reps != replayed.Reps || stored.Lapses != replayed.Lapses {
		return true
	}
	if timeSlackExceeds(stored.Due, replayed.Due, replayDueSlack) {
		return true
	}
	return timeSlackExceeds(stored.LastReview, replayed.LastReview, replayDueSlack)
}

func timeSlackExceeds(a, b time.Time, slack time.Duration) bool {
	d := a.Sub(b)
	if d < 0 {
		d = -d
	}
	return d > slack
}

type skillKey struct {
	lexeme int64
	skill  string
}

// applyPastCardHistories overlays past-day card fields onto evidence.
// Cards already omitted by CardHistories / LearnerEvidence stay out.
// Unreviewed cards that existed by at keep HasCard with the creation state
// (new, zeros). Cards with reviews use the FSRS replay walk.
func applyPastCardHistories(ev *db.LearnerEvidence, histories []db.CardHistory, zone studyday.Zone) error {
	byKey := make(map[skillKey]db.CardHistory, len(histories))
	for _, h := range histories {
		byKey[skillKey{h.LexemeID, h.Skill}] = h
	}
	sched := replayScheduler()
	for i := range ev.Skills {
		ls := &ev.Skills[i]
		if !ls.HasCard {
			continue
		}
		h, ok := byKey[skillKey{ls.LexemeID, ls.Skill}]
		if !ok {
			// Should not happen when LearnerEvidence and CardHistories share the
			// as-of filter; fail safe by omitting the card.
			clearCardFields(ls)
			continue
		}
		if len(h.Reviews) == 0 {
			ls.CardState = fsrs.StateNew
			ls.Stability = 0
			ls.LastReview = time.Time{}
			ls.Reps = 0
			ls.Lapses = 0
			continue
		}
		replayed, ok, err := replayHistory(sched, h, zone)
		if err != nil {
			return err
		}
		if !ok {
			clearCardFields(ls)
			continue
		}
		ls.CardState = replayed.State
		ls.Stability = replayed.Stability
		ls.LastReview = replayed.LastReview
		ls.Reps = replayed.Reps
		ls.Lapses = replayed.Lapses
	}
	return nil
}

func clearCardFields(ls *db.LearnerSkill) {
	ls.HasCard = false
	ls.CardState = ""
	ls.Stability = 0
	ls.LastReview = time.Time{}
	ls.Reps = 0
	ls.Lapses = 0
}

// countReplayMismatches compares each reviewed card's stored schedule (from
// CardHistories) to a fresh replay. Unreviewed cards are skipped.
func countReplayMismatches(histories []db.CardHistory, zone studyday.Zone) (int, error) {
	sched := replayScheduler()
	n := 0
	for _, h := range histories {
		if len(h.Reviews) == 0 {
			continue
		}
		replayed, ok, err := replayHistory(sched, h, zone)
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		if cardReplayMismatch(h.StoredCard(), replayed) {
			n++
		}
	}
	return n, nil
}
