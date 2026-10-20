package learner

import (
	"context"
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
}

// Snapshot evaluates the learner once and returns the summary together with the state of
// every word that has evidence. It reads the same data as Summary, and Summary returns the
// Summary of a Snapshot.
func (m *Model) Snapshot(ctx context.Context, userID int64) (Snapshot, error) {
	if err := m.ready(userID); err != nil {
		return Snapshot{}, err
	}
	now := m.clock.Now()
	ids, err := m.store.LexemeIDs(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	ev, err := m.store.LearnerEvidence(ctx, userID, m.query(now, 0))
	if err != nil {
		return Snapshot{}, err
	}
	bands, err := m.bands.LexemeBands(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	words := m.evaluate(ev, bands, now)
	placed, havePlacement, err := m.store.PlacementBands(ctx, userID)
	if err != nil {
		return Snapshot{}, err
	}
	retention, err := m.store.DueRetention(ctx, userID, now.AddDate(0, 0, -RetentionWindowDays))
	if err != nil {
		return Snapshot{}, err
	}

	out := Snapshot{Words: words}
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
		ss.EstimatedKnown = estimateKnown(placed, havePlacement, evidenced, known)
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
	}
	out.Summary.DueReviews = due
	out.Summary.Retention = ratio(right, due)
	return out, nil
}
