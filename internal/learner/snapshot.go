package learner

import (
	"context"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// BandCount is how many words of one placement band the learner knows.
type BandCount struct {
	// Band is the placement band, 0 to 4.
	Band int
	// Words is how many words of the lexicon are in the band.
	Words int
	// Known is how many of them are known or solid overall.
	Known int
}

// Share is the part of the band's words that are known, 0 to 1. A band with no words is 0.
func (b BandCount) Share() float64 {
	if b.Words == 0 {
		return 0
	}
	return float64(b.Known) / float64(b.Words)
}

// Snapshot is everything the model says about one learner, from one read of the evidence.
// The daily job (INIT-015/SPEC-014) reads it once per learner. Calling Summary and then
// List for every page would read the evidence once per call.
type Snapshot struct {
	// Summary is the counts, estimates, and retention of the learner.
	Summary Summary
	// Words has the state of every word that has any evidence or exposure. A word that is
	// not here is unseen in every skill.
	Words map[int64]WordState
	// Bands has one entry per placement band 0 to 4, in order. A band with no words in it
	// has Words of 0.
	Bands [5]BandCount
	// ReplayMismatch is how many cards disagree with an FSRS replay of their review_log
	// at "now". It is set only on a live snapshot (Snapshot, or SnapshotAt with at not
	// before the clock); past SnapshotAt results leave it 0. See replay.go.
	ReplayMismatch int
}

// Snapshot evaluates the learner once at the model clock, live: stored card state
// and uncapped evidence reads. ReplayMismatch still compares a full replay to the
// stored cards.
func (m *Model) Snapshot(ctx context.Context, userID int64) (Snapshot, error) {
	if err := m.ready(userID); err != nil {
		return Snapshot{}, err
	}
	return m.evaluateSnapshot(ctx, userID, m.clock.Now(), time.Time{}, true)
}

// SnapshotAt evaluates the learner as of moment at under the current rules
// (ADR-0028 D3). Windows (miss, confusion, retention, accuracy) are measured
// back from at. Any at before the clock, even by a moment, is the past: evidence
// and placement reads use AsOf=at, and each card's review_log is replayed up to
// at with the same go-fsrs parameters as Grade (see replay.go). Nothing that
// happened after at is counted. An at that is not before the clock is now, and
// is evaluated live like Snapshot.
func (m *Model) SnapshotAt(ctx context.Context, userID int64, at time.Time) (Snapshot, error) {
	if err := m.ready(userID); err != nil {
		return Snapshot{}, err
	}
	if !at.Before(m.clock.Now()) {
		return m.evaluateSnapshot(ctx, userID, at, time.Time{}, true)
	}
	return m.evaluateSnapshot(ctx, userID, at, at, false)
}

// evaluateSnapshot is the shared body of Snapshot and SnapshotAt.
// at is the evaluation moment (window anchor and retrievability "now").
// asOf bounds evidence reads (zero = live unbound query).
// live selects stored card fields and fills ReplayMismatch.
func (m *Model) evaluateSnapshot(ctx context.Context, userID int64, at, asOf time.Time, live bool) (Snapshot, error) {
	ids, err := m.store.LexemeIDs(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	ev, err := m.store.LearnerEvidence(ctx, userID, m.queryAt(at, 0, asOf))
	if err != nil {
		return Snapshot{}, err
	}
	if !asOf.IsZero() {
		ev.Confusions = confusionsUpTo(ev.Confusions, asOf)
	}

	histories, err := m.store.CardHistories(ctx, userID, asOf)
	if err != nil {
		return Snapshot{}, err
	}

	mismatch := 0
	if live {
		mismatch, err = countReplayMismatches(histories)
		if err != nil {
			return Snapshot{}, err
		}
	} else if err := applyPastCardHistories(&ev, histories); err != nil {
		return Snapshot{}, err
	}

	bands, err := m.bands.LexemeBands(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	words := m.evaluate(ev, bands, at)

	skillBands, err := m.store.PlacementSkillBands(ctx, userID, asOf)
	if err != nil {
		return Snapshot{}, err
	}
	retentionSince := at.AddDate(0, 0, -RetentionWindowDays)
	retention, err := m.store.DueRetention(ctx, userID, retentionSince, asOf)
	if err != nil {
		return Snapshot{}, err
	}

	out := Snapshot{Words: words, ReplayMismatch: mismatch}
	out.Summary.Words = map[State]int{}
	for _, s := range States {
		out.Summary.Words[s] = 0
	}
	for i := range out.Bands {
		out.Bands[i].Band = i
	}
	// Words with no evidence at all are unseen overall and in every skill.
	unseen := len(ids)
	for _, id := range ids {
		if _, ok := words[id]; ok {
			unseen--
		}
		if b := bandOf(bands, id); b >= 0 && b < len(out.Bands) {
			out.Bands[b].Words++
		}
	}
	out.Summary.Words[Unseen] += unseen

	var due, right int
	for _, skill := range Skills {
		ss := SkillSummary{Skill: skill, Counts: map[State]int{}}
		for _, s := range States {
			ss.Counts[s] = 0
		}
		ss.Counts[Unseen] += unseen
		evidenced := [5]int{}
		known := 0
		for _, id := range ids {
			w, ok := words[id]
			if !ok {
				continue
			}
			sk := w.skill(skill)
			ss.Counts[sk.State]++
			if sk.State == Known || sk.State == Solid {
				known++
			}
			if sk.hasEvidence() && w.Band >= 0 {
				evidenced[w.Band]++
			}
		}
		placed, havePlacement := skillBands[skill]
		ss.EstimatedKnown = estimateKnown(placed.Bands, havePlacement, evidenced, known)
		if havePlacement {
			ss.EstimateSource = EstimateSourcePlacement
			ss.PlacementRunAt = placed.RunAt
		} else {
			ss.EstimateSource = EstimateSourceEvidence
		}
		for _, r := range retention {
			if r.Skill == skill {
				ss.DueReviews = r.Due
				ss.Retention = ratio(r.Right, r.Due)
				due += r.Due
				right += r.Right
			}
		}
		out.Summary.Skills = append(out.Summary.Skills, ss)
	}
	for _, id := range ids {
		w, ok := words[id]
		if !ok {
			continue
		}
		out.Summary.Words[w.State]++
		if w.Band >= 0 && w.Band < len(out.Bands) && (w.State == Known || w.State == Solid) {
			out.Bands[w.Band].Known++
		}
		// KnownAnySkill: known or solid in at least one skill, counted once (ADR-0027 D1).
		for _, sk := range w.Skills {
			if sk.State == Known || sk.State == Solid {
				out.Summary.KnownAnySkill++
				break
			}
		}
	}
	out.Summary.DueReviews = due
	out.Summary.Retention = ratio(right, due)
	return out, nil
}

func confusionsUpTo(in []db.LearnerConfusion, asOf time.Time) []db.LearnerConfusion {
	out := in[:0]
	for _, c := range in {
		if !c.At.After(asOf) {
			out = append(out, c)
		}
	}
	return out
}
