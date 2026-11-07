package learner

// Audit recompute for hebrew audit (INIT-032/SPEC-009 / ADR-0027).
// Recompute walks evidence with StateOf and PlacementSkillBands independently of
// Summary's return path so a headline or estimate regression can be named.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// SkillNumbers is one skill's audit counts.
type SkillNumbers struct {
	KnownSolid     int
	EstimatedKnown float64
	EstimateSource string
	DueReviews     int
	Retention      *float64
}

// Numbers is the learner count set hebrew audit compares.
type Numbers struct {
	KnownAnySkill int
	Skills        map[string]SkillNumbers
	Retention     *float64
	DueReviews    int
	SpellCards    int
	Zone          string // empty when none stored
	ZoneOK        bool
}

// FromSummary maps the page/MCP Summary into Numbers (surface side of the audit).
func FromSummary(sum Summary) Numbers {
	out := Numbers{
		KnownAnySkill: sum.KnownAnySkill,
		Skills:        make(map[string]SkillNumbers, len(sum.Skills)),
		Retention:     sum.Retention,
		DueReviews:    sum.DueReviews,
	}
	for _, sk := range sum.Skills {
		out.Skills[sk.Skill] = SkillNumbers{
			KnownSolid:     sk.Counts[Known] + sk.Counts[Solid],
			EstimatedKnown: sk.EstimatedKnown,
			EstimateSource: sk.EstimateSource,
			DueReviews:     sk.DueReviews,
			Retention:      sk.Retention,
		}
	}
	return out
}

// Recompute builds Numbers from raw evidence via StateOf, PlacementSkillBands,
// DueRetention, and the spelling card count. It does not call Summary.
func (m *Model) Recompute(ctx context.Context, userID int64) (Numbers, error) {
	if err := m.ready(userID); err != nil {
		return Numbers{}, err
	}
	now := m.clock.Now()
	ids, err := m.store.LexemeIDs(ctx)
	if err != nil {
		return Numbers{}, err
	}
	ev, err := m.store.LearnerEvidence(ctx, userID, m.query(now, 0))
	if err != nil {
		return Numbers{}, err
	}
	bands, err := m.bands.LexemeBands(ctx)
	if err != nil {
		return Numbers{}, err
	}
	words := m.evaluate(ev, bands, now)

	skillBands, err := m.store.PlacementSkillBands(ctx, userID, time.Time{})
	if err != nil {
		return Numbers{}, err
	}
	retention, err := m.store.DueRetention(ctx, userID, now.AddDate(0, 0, -RetentionWindowDays), time.Time{})
	if err != nil {
		return Numbers{}, err
	}
	spellCards, err := m.store.CountCardsBySkill(ctx, userID, fsrs.SkillSpell)
	if err != nil {
		return Numbers{}, err
	}
	zone, zoneOK, err := m.store.UserTimeZone(ctx, userID)
	if err != nil {
		return Numbers{}, err
	}

	out := Numbers{
		Skills:     make(map[string]SkillNumbers, len(Skills)),
		SpellCards: spellCards,
		Zone:       zone,
		ZoneOK:     zoneOK,
	}
	retBySkill := map[string]db.RetentionRow{}
	for _, r := range retention {
		retBySkill[r.Skill] = r
	}
	var dueAll, rightAll int
	for _, skill := range Skills {
		known := 0
		evidenced := [5]int{}
		for _, id := range ids {
			w, ok := words[id]
			if !ok {
				continue
			}
			sk := w.skill(skill)
			// StateOf already ran inside evaluate; count known+solid from that state.
			if sk.State == Known || sk.State == Solid {
				known++
			}
			if sk.hasEvidence() && w.Band >= 0 {
				evidenced[w.Band]++
			}
		}
		placed, have := skillBands[skill]
		est := EstimateKnown(placed.Bands, have, evidenced, known)
		src := EstimateSourceEvidence
		if have {
			src = EstimateSourcePlacement
		}
		row := SkillNumbers{
			KnownSolid:     known,
			EstimatedKnown: est,
			EstimateSource: src,
		}
		if r, ok := retBySkill[skill]; ok {
			row.DueReviews = r.Due
			row.Retention = ratio(r.Right, r.Due)
			dueAll += r.Due
			rightAll += r.Right
		}
		out.Skills[skill] = row
	}
	for _, id := range ids {
		w, ok := words[id]
		if !ok {
			continue
		}
		for _, sk := range w.Skills {
			if sk.State == Known || sk.State == Solid {
				out.KnownAnySkill++
				break
			}
		}
	}
	out.DueReviews = dueAll
	out.Retention = ratio(rightAll, dueAll)
	return out, nil
}

// EstimateKnown is the per-skill placement/evidence estimate (ADR-0027 D2).
// Exported for the audit and the cross-surface mutation fences.
func EstimateKnown(placed []db.BandScore, have bool, evidenced [5]int, known int) float64 {
	return estimateKnown(placed, have, evidenced, known)
}

// WeakestSkillHeadline is the old Progress headline: known only when every
// practiced skill is known or solid (Summary.Words[Known]+Words[Solid]).
// The cross-surface fence expects this to differ from KnownAnySkill.
func WeakestSkillHeadline(sum Summary) int {
	return sum.Words[Known] + sum.Words[Solid]
}

// FormatFloat formats an estimate for audit table comparison.
func FormatFloat(v float64) string {
	return fmt.Sprintf("%.4f", v)
}

// FormatRetention formats a nullable retention share.
func FormatRetention(v *float64) string {
	if v == nil {
		return "null"
	}
	return FormatFloat(*v)
}

// FormatAccuracy formats a nullable 14-day accuracy share for audit rows.
func FormatAccuracy(v *float64) string {
	return FormatRetention(v)
}

// DirectionAudit is one direction's verdict and accuracy for hebrew audit.
type DirectionAudit struct {
	Skill      string
	Verdict    string
	Accuracy14 *float64
}

// HistoryStanding is the INIT-033/SPEC-008 audit rows beyond the ADR-0027 set.
type HistoryStanding struct {
	HistoryRule1Days     int
	HistoryLastDay       string // empty when no stored totals day
	HistoryLastKnownAny  int    // daily_totals.known_any_skill for HistoryLastDay
	SnapshotAtKnownAny   int    // SnapshotAt(end of HistoryLastDay).Summary.KnownAnySkill
	ReplayMismatch       int
	Directions           []DirectionAudit // Skills order
	ReviewSkippedNoAudio int
}

// CountRule1Days counts distinct daily_totals days still stamped rule_version 1.
func CountRule1Days(ctx context.Context, store *db.Store, userID int64) (int, error) {
	totals, err := store.ListTotals(ctx, userID, "0001-01-01", "9999-12-31")
	if err != nil {
		return 0, err
	}
	n := 0
	for _, row := range totals {
		if row.RuleVersion == 1 {
			n++
		}
	}
	return n, nil
}

// CountReviewSkippedNoAudio counts due listen cards whose lexeme has no audio_id
// (ADR-0028 D6 / session SkippedNoAudio).
func (m *Model) CountReviewSkippedNoAudio(ctx context.Context, userID int64) (int, error) {
	if err := m.ready(userID); err != nil {
		return 0, err
	}
	due, err := m.store.ListDueCards(ctx, userID, m.clock.Now())
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range due {
		if c.Skill != fsrs.SkillListen {
			continue
		}
		ref, ok, err := m.store.LexemeByID(ctx, c.LexemeID)
		if err != nil {
			return 0, err
		}
		if !ok || ref.AudioID == "" {
			n++
		}
	}
	return n, nil
}

// RecomputeDirections builds per-direction verdict and accuracy14 from AttemptTally
// and Snapshot without calling Standing, so the audit can name a Standing drift.
func (m *Model) RecomputeDirections(ctx context.Context, userID int64) ([]DirectionAudit, error) {
	if err := m.ready(userID); err != nil {
		return nil, err
	}
	now := m.clock.Now()
	snap, err := m.Snapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	since := now.AddDate(0, 0, -StandingWindowDays)
	tallies, err := m.store.AttemptTally(ctx, userID, since, now)
	if err != nil {
		return nil, err
	}
	audioN, err := m.store.CountLexemesWithAudio(ctx)
	if err != nil {
		return nil, err
	}
	bySkill := map[string]struct{ answers, right, weak int }{}
	for _, row := range tallies {
		s := bySkill[row.Skill]
		s.answers += row.Answers
		s.right += row.Answers - row.Misses
		bySkill[row.Skill] = s
	}
	for _, w := range snap.Words {
		for _, skill := range Skills {
			st := w.stateOf(skill)
			if st == Struggling || st == Regressed {
				s := bySkill[skill]
				s.weak++
				bySkill[skill] = s
			}
		}
	}
	out := make([]DirectionAudit, 0, len(Skills))
	for _, skill := range Skills {
		askable := skillAskable(skill, snap, audioN)
		s := bySkill[skill]
		verdict := directionVerdict(askable, s.answers, s.right, s.weak,
			HelpAccuracy, HelpMinAnswers, StrongAccuracy, StrongMinAnswers)
		var acc *float64
		if s.answers > 0 {
			v := float64(s.right) / float64(s.answers)
			acc = &v
		}
		out = append(out, DirectionAudit{Skill: skill, Verdict: verdict, Accuracy14: acc})
	}
	return out, nil
}

// HistoryFacts loads stored totals, SnapshotAt for the last day, live ReplayMismatch,
// Standing directions, and review_skipped_no_audio for the audit surface/recompute pair.
// endOfDay turns a stored day into the instant that ended it, in the zone the daily job
// used for this learner; the caller resolves that zone.
func (m *Model) HistoryFacts(ctx context.Context, userID int64, endOfDay func(day string) (time.Time, error)) (HistoryStanding, error) {
	if endOfDay == nil {
		return HistoryStanding{}, errors.New("learner: HistoryFacts needs endOfDay")
	}
	if err := m.ready(userID); err != nil {
		return HistoryStanding{}, err
	}
	rule1, err := CountRule1Days(ctx, m.store, userID)
	if err != nil {
		return HistoryStanding{}, err
	}
	out := HistoryStanding{HistoryRule1Days: rule1}

	totals, err := m.store.ListTotals(ctx, userID, "0001-01-01", "9999-12-31")
	if err != nil {
		return HistoryStanding{}, err
	}
	if len(totals) > 0 {
		last := totals[len(totals)-1]
		out.HistoryLastDay = last.Day
		out.HistoryLastKnownAny = last.KnownAnySkill
		at, err := endOfDay(last.Day)
		if err != nil {
			return HistoryStanding{}, err
		}
		snap, err := m.SnapshotAt(ctx, userID, at)
		if err != nil {
			return HistoryStanding{}, err
		}
		out.SnapshotAtKnownAny = snap.Summary.KnownAnySkill
	}

	live, err := m.Snapshot(ctx, userID)
	if err != nil {
		return HistoryStanding{}, err
	}
	out.ReplayMismatch = live.ReplayMismatch

	standing, err := m.Standing(ctx, userID)
	if err != nil {
		return HistoryStanding{}, err
	}
	out.Directions = make([]DirectionAudit, 0, len(standing.Directions))
	for _, d := range standing.Directions {
		out.Directions = append(out.Directions, DirectionAudit{
			Skill: d.Skill, Verdict: d.Verdict, Accuracy14: d.Accuracy14,
		})
	}

	skipped, err := m.CountReviewSkippedNoAudio(ctx, userID)
	if err != nil {
		return HistoryStanding{}, err
	}
	out.ReviewSkippedNoAudio = skipped
	return out, nil
}
