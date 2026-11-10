package learner

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// Standing thresholds (ADR-0028 D5). thresholds_test.go pins them.
const (
	HelpAccuracy       = 0.75
	HelpMinAnswers     = 10
	StrongAccuracy     = 0.85
	StrongMinAnswers   = 20
	StandingWindowDays = 14
	StandingTop        = 5
)

// Standing is where the learner stands across the four directions.
type Standing struct {
	AsOf          time.Time
	KnownAnySkill int
	Directions    []DirectionStanding // listen, read, recall, spell — Skills order
}

// DirectionStanding is one skill's help and strong picture.
type DirectionStanding struct {
	Skill                  string // "read"
	Direction              string // "Reading (Hebrew → English)" — ADR-0028 D4
	Known, Solid, Learning int
	Struggling, Regressed  int
	Due                    int // cards in this skill due for review now (unsuspended, intro done)
	Answers14, Right14     int
	Accuracy14             *float64 // nil below 1 answer
	Verdict                string   // "needs_help" | "strong" | "building" | "not_askable"
	Reason                 string   // one plain sentence
	Help                   []WordNote
	Strong                 []WordNote
	Askable                bool
	// Ladder is this skill's proved rung, counted out of taught words (ADR-0033 D4).
	Ladder LadderCounts
}

// Headline names for the two rows Progress leads with (ADR-0033 D6).
// Listen is Hebrew heard and a meaning picked. Recall is English shown and
// Hebrew typed or picked. Pronunciation is not a row.
const (
	HeadlineListen = "Understand by ear"
	HeadlineRecall = "Hebrew from English"
)

// WordNote is one word on a help or strong list.
type WordNote struct {
	LexemeID            int64
	Hebrew, English     string
	Misses14, Answers14 int
	Cause               string // learner.Diagnose
	Pair                string // DominantPair key, e.g. "ב/כ", or ""
	ConfusedWith        string // the word it was mistaken for, or ""
}

// DirectionLabel is the Progress / MCP name for a skill key (ADR-0028 D4).
func DirectionLabel(skill string) string {
	switch skill {
	case fsrs.SkillListen:
		return "Listening"
	case fsrs.SkillRead:
		return "Reading (Hebrew → English)"
	case fsrs.SkillRecall:
		return "English → Hebrew"
	case fsrs.SkillSpell:
		return "Spelling"
	default:
		return skill
	}
}

// Standing computes the learner's per-direction standing from one Snapshot and one
// 14-day attempt tally (ADR-0028 D5).
func (m *Model) Standing(ctx context.Context, userID int64) (Standing, error) {
	if err := m.ready(userID); err != nil {
		return Standing{}, err
	}
	now := m.clock.Now()
	snap, err := m.Snapshot(ctx, userID)
	if err != nil {
		return Standing{}, err
	}
	since := now.AddDate(0, 0, -StandingWindowDays)
	tallies, err := m.store.AttemptTally(ctx, userID, since, now)
	if err != nil {
		return Standing{}, err
	}
	audioN, err := m.store.CountLexemesWithAudio(ctx)
	if err != nil {
		return Standing{}, err
	}

	bySkill := map[string]map[int64]db.AttemptTallyRow{}
	for _, row := range tallies {
		if bySkill[row.Skill] == nil {
			bySkill[row.Skill] = map[int64]db.AttemptTallyRow{}
		}
		bySkill[row.Skill][row.LexemeID] = row
	}

	// Snapshot words do not carry Hebrew/English; load them once for notes and
	// for resolving ConfusedWith from a DominantPair "other" key.
	textIDs := make([]int64, 0, len(snap.Words))
	for id := range snap.Words {
		textIDs = append(textIDs, id)
	}
	for _, row := range tallies {
		textIDs = append(textIDs, row.LexemeID)
	}
	texts, err := m.store.LexemeTexts(ctx, textIDs)
	if err != nil {
		return Standing{}, err
	}

	dueCards, err := m.store.ListDueCards(ctx, userID, now)
	if err != nil {
		return Standing{}, err
	}
	dueBy := map[string]int{}
	for _, c := range dueCards {
		dueBy[c.Skill]++
	}

	out := Standing{
		AsOf:          now,
		KnownAnySkill: snap.Summary.KnownAnySkill,
		Directions:    make([]DirectionStanding, 0, len(Skills)),
	}
	for _, skill := range Skills {
		d := buildDirection(skill, snap, bySkill[skill], audioN, texts)
		d.Due = dueBy[skill]
		out.Directions = append(out.Directions, d)
	}
	return out, nil
}

func buildDirection(skill string, snap Snapshot, tallies map[int64]db.AttemptTallyRow, audioN int, texts map[int64]db.LexemeText) DirectionStanding {
	d := DirectionStanding{
		Skill:     skill,
		Direction: DirectionLabel(skill),
		Askable:   skillAskable(skill, snap, audioN),
	}
	for _, ss := range snap.Summary.Skills {
		if ss.Skill != skill {
			continue
		}
		d.Known = ss.Counts[Known]
		d.Solid = ss.Counts[Solid]
		d.Learning = ss.Counts[Learning]
		d.Struggling = ss.Counts[Struggling]
		d.Regressed = ss.Counts[Regressed]
		d.Ladder = ss.Ladder
		break
	}
	for _, row := range tallies {
		d.Answers14 += row.Answers
		d.Right14 += row.Answers - row.Misses
	}
	if d.Answers14 > 0 {
		acc := float64(d.Right14) / float64(d.Answers14)
		d.Accuracy14 = &acc
	}
	strugglingOrRegressed := d.Struggling + d.Regressed
	d.Verdict = directionVerdict(
		d.Askable, d.Answers14, d.Right14, strugglingOrRegressed,
		HelpAccuracy, HelpMinAnswers, StrongAccuracy, StrongMinAnswers,
	)
	d.Reason = standingReason(d)
	d.Help = helpNotes(skill, snap, tallies, texts)
	helpIDs := map[int64]bool{}
	for _, n := range d.Help {
		helpIDs[n.LexemeID] = true
	}
	d.Strong = strongNotes(skill, snap, tallies, helpIDs, texts)
	return d
}

// skillAskable reports whether a checked activity can ask any of the learner's words
// for the skill. Listening needs at least one lexeme with audio_id. Spelling is
// askable by type_it without audio (ADR-0028 D6). Read and recall are always askable.
func skillAskable(skill string, _ Snapshot, audioN int) bool {
	switch skill {
	case fsrs.SkillListen:
		return audioN > 0
	default:
		// read, recall, and spell (type_it needs no clip).
		return true
	}
}

// directionVerdict applies ADR-0028 D5. The bars are parameters so a test can
// prove changing a threshold changes the verdict.
func directionVerdict(askable bool, answers, right, strugglingOrRegressed int, helpAcc float64, helpMin int, strongAcc float64, strongMin int) string {
	if !askable {
		return "not_askable"
	}
	var acc float64
	if answers > 0 {
		acc = float64(right) / float64(answers)
	}
	if (answers >= helpMin && acc < helpAcc) || strugglingOrRegressed > 0 {
		return "needs_help"
	}
	if answers >= strongMin && acc >= strongAcc {
		return "strong"
	}
	return "building"
}

func standingReason(d DirectionStanding) string {
	if !d.Askable {
		return "No audio yet — practiced in lessons only"
	}
	if d.Answers14 < 1 {
		return fmt.Sprintf("No checked answers in %d days", StandingWindowDays)
	}
	pct := int(math.Round(100 * float64(d.Right14) / float64(d.Answers14)))
	return fmt.Sprintf("%d%% right over %d answers in %d days", pct, d.Answers14, StandingWindowDays)
}

type helpCand struct {
	id       int64
	weak     bool
	misses   int
	answers  int
	lastMiss time.Time
	word     WordState
}

func helpNotes(skill string, snap Snapshot, tallies map[int64]db.AttemptTallyRow, texts map[int64]db.LexemeText) []WordNote {
	var cands []helpCand
	seen := map[int64]bool{}
	for id, w := range snap.Words {
		st := w.stateOf(skill)
		weak := st == Struggling || st == Regressed
		row := tallies[id]
		if !weak && row.Misses < 2 {
			continue
		}
		cands = append(cands, helpCand{
			id: id, weak: weak, misses: row.Misses, answers: row.Answers,
			lastMiss: row.LastMiss, word: w,
		})
		seen[id] = true
	}
	for id, row := range tallies {
		if seen[id] || row.Misses < 2 {
			continue
		}
		w, ok := snap.Words[id]
		if !ok {
			w = WordState{LexemeID: id}
		}
		cands = append(cands, helpCand{
			id: id, weak: false, misses: row.Misses, answers: row.Answers,
			lastMiss: row.LastMiss, word: w,
		})
	}
	slices.SortFunc(cands, func(a, b helpCand) int {
		if a.weak != b.weak {
			if a.weak {
				return -1
			}
			return 1
		}
		if a.misses != b.misses {
			return b.misses - a.misses
		}
		if !a.lastMiss.Equal(b.lastMiss) {
			if a.lastMiss.After(b.lastMiss) {
				return -1
			}
			return 1
		}
		if a.id < b.id {
			return -1
		}
		if a.id > b.id {
			return 1
		}
		return 0
	})
	if len(cands) > StandingTop {
		cands = cands[:StandingTop]
	}
	out := make([]WordNote, len(cands))
	for i, c := range cands {
		cause := Diagnose(c.word, c.word.Confusions)
		t := texts[c.id]
		pair, with := pairAndConfused(c.word, t.TextNFC)
		out[i] = WordNote{
			LexemeID: c.id, Hebrew: t.TextNFC, English: t.Gloss,
			Misses14: c.misses, Answers14: c.answers,
			Cause: string(cause), Pair: pair, ConfusedWith: with,
		}
	}
	return out
}

func strongNotes(skill string, snap Snapshot, tallies map[int64]db.AttemptTallyRow, skip map[int64]bool, texts map[int64]db.LexemeText) []WordNote {
	type cand struct {
		id        int64
		solid     bool
		stability float64
		misses    int
		answers   int
	}
	var cands []cand
	for id, w := range snap.Words {
		if skip[id] {
			continue
		}
		st := w.stateOf(skill)
		if st != Solid && st != Known {
			continue
		}
		sk := w.skill(skill)
		row := tallies[id]
		cands = append(cands, cand{
			id: id, solid: st == Solid, stability: sk.Stability,
			misses: row.Misses, answers: row.Answers,
		})
	}
	slices.SortFunc(cands, func(a, b cand) int {
		if a.solid != b.solid {
			if a.solid {
				return -1
			}
			return 1
		}
		if a.stability != b.stability {
			if a.stability > b.stability {
				return -1
			}
			return 1
		}
		if a.id < b.id {
			return -1
		}
		if a.id > b.id {
			return 1
		}
		return 0
	})
	if len(cands) > StandingTop {
		cands = cands[:StandingTop]
	}
	out := make([]WordNote, len(cands))
	for i, c := range cands {
		t := texts[c.id]
		out[i] = WordNote{
			LexemeID: c.id, Hebrew: t.TextNFC, English: t.Gloss,
			Misses14: c.misses, Answers14: c.answers,
		}
	}
	return out
}

// pairAndConfused returns the DominantPair key and, for an "other" mix-up, the
// other word's pointed text (never the learner's typed response).
func pairAndConfused(w WordState, askedHebrew string) (pair, with string) {
	key, _, ok := DominantPair(w.Confusions)
	if !ok {
		return "", ""
	}
	pair = key
	kind := ""
	for _, c := range w.Confusions {
		if c.Key == key {
			kind = c.Kind
			break
		}
	}
	if kind != fsrs.MissOther {
		return pair, ""
	}
	a, b, cut := strings.Cut(key, "/")
	if !cut {
		return pair, ""
	}
	switch {
	case a == askedHebrew:
		with = b
	case b == askedHebrew:
		with = a
	}
	return pair, with
}
