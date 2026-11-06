package learner

// Audit recompute for hebrew audit (INIT-032/SPEC-009 / ADR-0027).
// Recompute walks evidence with StateOf and PlacementSkillBands independently of
// Summary's return path so a headline or estimate regression can be named.

import (
	"context"
	"fmt"

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

	skillBands, err := m.store.PlacementSkillBands(ctx, userID)
	if err != nil {
		return Numbers{}, err
	}
	retention, err := m.store.DueRetention(ctx, userID, now.AddDate(0, 0, -RetentionWindowDays))
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
		est := EstimateKnown(placed, have, evidenced, known)
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
