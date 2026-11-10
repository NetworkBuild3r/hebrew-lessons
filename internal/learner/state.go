package learner

import (
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner/rule"
)

// State is where a word stands in one skill, or overall.
type State string

// The seven states of docs/north-star.md section 4.3.
const (
	Unseen     State = "unseen"     // No evidence at all.
	Seen       State = "seen"       // Shown, or answered without a card yet, and not struggling.
	Learning   State = "learning"   // A card that is not yet known: young, or not recalled well enough.
	Struggling State = "struggling" // Wrong too often lately.
	Known      State = "known"      // Recalled, and the last answers were right.
	Solid      State = "solid"      // Known, and stable for weeks.
	Regressed  State = "regressed"  // Was learned, and has faded.
)

// States lists every state from weakest to strongest. A word's overall state is
// its weakest skill that has evidence, so this order is the order of "weakest".
var States = []State{Unseen, Seen, Struggling, Regressed, Learning, Known, Solid}

// Valid reports whether s is one of the seven states.
func (s State) Valid() bool { return s.rank() >= 0 }

func (s State) rank() int {
	for i, v := range States {
		if v == s {
			return i
		}
	}
	return -1
}

// Skills lists the four skills in the order the model reports them.
var Skills = []string{fsrs.SkillListen, fsrs.SkillRead, fsrs.SkillRecall, fsrs.SkillSpell}

// The thresholds. They are product choices from SPEC-013 and north-star section 4.3.
// thresholds_test.go pins every one, so changing a value is a deliberate edit.
const (
	// StruggleAgainWindow and StruggleAgainCount: this many Again among the newest
	// StruggleAgainWindow outcomes is struggling. 3 of the last 8 is struggling, 2 is not.
	StruggleAgainWindow = 8
	StruggleAgainCount  = 3

	// AccuracyWindow is how many of the newest outcomes the accuracy rule reads.
	// AccuracyMinOutcomes is the fewest outcomes in that window before the rule applies.
	// AccuracyFloorPercent is the floor: accuracy under it is struggling, and exactly
	// at it is not. 3 right of 5 is exactly 60%.
	AccuracyWindow       = 20
	AccuracyMinOutcomes  = 5
	AccuracyFloorPercent = 60

	// KnownRetrievability is the recall chance rule version 4 required, and
	// KnownRecentRight is how many of the newest outcomes had to be right.
	// Counts no longer read them. KnownRule4 still does (ADR-0033 D4).
	KnownRetrievability = rule.KnownRetrievability
	KnownRecentRight    = rule.KnownRecentRight

	// KnownStudyDays is how many distinct study days rule version 4 required
	// (ADR-0031 D1). Counts no longer read it. KnownRule4 still does.
	KnownStudyDays = rule.KnownStudyDays

	// ProvedSuccessDays is how many distinct study days must carry a checked
	// right answer before a skill is proved. ProvedSpanDays is the minimum
	// gap from the first of those days to the last (ADR-0033 D4).
	ProvedSuccessDays = rule.ProvedSuccessDays
	ProvedSpanDays    = rule.ProvedSpanDays

	// SolidStabilityDays is the card stability a solid word needs, on top of being known.
	SolidStabilityDays = 21.0

	// RegressedRetrievability is the recall chance below which a word that was learned
	// counts as regressed.
	RegressedRetrievability = 0.70

	// LearningSuccesses is how many successful reviews (Hard, Good, or Easy) a card needs
	// before it is no longer young. A card younger than its third success is learning.
	LearningSuccesses = rule.LearningSuccesses
)

// Evidence is what the rules read for one word in one skill. It holds counts, so the
// query layer can fill it without returning every attempt.
//
// The counts are over the skill's outcomes, newest first. Window is how many of the
// newest AccuracyWindow there are, and WindowCorrect how many of those were right.
// AgainOf is how many of the newest StruggleAgainWindow there are, and Agains how
// many of those were not right. Recent and RecentRight are the same for the newest
// KnownRecentRight. OutcomeCounts builds them from a list.
type Evidence struct {
	Outcomes      int
	Window        int
	WindowCorrect int
	AgainOf       int
	Agains        int
	Recent        int
	RecentRight   int

	HasCard        bool    // The learner has a card for this word and skill.
	Successes      int     // Reviews of that card rated Hard, Good, or Easy.
	Stability      float64 // Card stability in days. 0 with no card.
	Retrievability float64 // Chance of recall now, 0 to 1. 0 with no card.
	// SuccessDays is how many distinct study days carry a checked success
	// (INIT-036/SPEC-002; filled by the query layer).
	SuccessDays int
	// FirstSuccessDay and LastSuccessDay are the earliest and latest of those
	// study days, YYYY-MM-DD. Both are empty when SuccessDays is 0.
	// INIT-038/SPEC-002.
	FirstSuccessDay string
	LastSuccessDay  string
	// Taught is true when this word counts as taught (ADR-0033 D5).
	// TaughtDay is that study day, YYYY-MM-DD.
	Taught    bool
	TaughtDay string
	// LatestRight is true when the newest checked answer was right.
	// It is meaningful only when Outcomes > 0.
	LatestRight bool

	Exposed bool // The word was shown with no question asked.
}

// Ladder is one rung of the proved ladder for a taught word in one skill.
// An empty ladder means the word is not taught, so it is not counted.
type Ladder string

// The four rungs (ADR-0033 D4). A taught word is on exactly one of them.
const (
	Proved        Ladder = "proved"
	RecalledLater Ladder = "recalled_later"
	Slipping      Ladder = "slipping"
	NotYetAsked   Ladder = "not_yet_asked"
)

// ValidLadder reports whether s is one of the four rungs.
func ValidLadder(s string) bool {
	switch Ladder(s) {
	case Proved, RecalledLater, Slipping, NotYetAsked:
		return true
	default:
		return false
	}
}

// LadderCounts is how many taught words sit on each rung of one skill.
// The four rung counts add up to Taught.
type LadderCounts struct {
	Taught        int
	Proved        int
	RecalledLater int
	Slipping      int
	NotYetAsked   int
}

// Add records one word on rung.
func (c *LadderCounts) Add(rung Ladder) {
	switch rung {
	case Proved:
		c.Proved++
	case RecalledLater:
		c.RecalledLater++
	case Slipping:
		c.Slipping++
	case NotYetAsked:
		c.NotYetAsked++
	default:
		return
	}
	c.Taught++
}

// OutcomeCounts builds the outcome counts of Evidence from a list of outcomes,
// newest first, true for a right answer. It is the reference for what the query
// layer counts, and the way tests describe a history.
func OutcomeCounts(newestFirst []bool) Evidence {
	e := Evidence{Outcomes: len(newestFirst)}
	for i, ok := range newestFirst {
		if i < AccuracyWindow {
			e.Window++
			if ok {
				e.WindowCorrect++
			}
		}
		if i < StruggleAgainWindow {
			e.AgainOf++
			if !ok {
				e.Agains++
			}
		}
		if i < KnownRecentRight {
			e.Recent++
			if ok {
				e.RecentRight++
			}
		}
	}
	return e
}

// HasEvidence reports whether the skill has anything beyond an exposure: an
// outcome or a card.
func (e Evidence) HasEvidence() bool { return e.Outcomes > 0 || e.HasCard }

// StrugglingByAgain is the first struggling rule: 3 or more Again among the newest 8 outcomes.
func StrugglingByAgain(e Evidence) bool { return e.Agains >= StruggleAgainCount }

// StrugglingByAccuracy is the second struggling rule: accuracy under 60% over at
// least 5 outcomes, among the newest 20. It is compared in whole numbers, so
// exactly 60% is not under.
func StrugglingByAccuracy(e Evidence) bool {
	return e.Window >= AccuracyMinOutcomes && e.WindowCorrect*100 < AccuracyFloorPercent*e.Window
}

// IsStruggling is either struggling rule.
func IsStruggling(e Evidence) bool { return StrugglingByAgain(e) || StrugglingByAccuracy(e) }

// WasLearned reports whether the card has passed its learning stage: it has had
// LearningSuccesses successful reviews. It is how the model reads "was known",
// because no per-word history of states is kept yet.
func WasLearned(e Evidence) bool { return e.HasCard && e.Successes >= LearningSuccesses }

// IsKnown is rule version 4: recall chance at least 85%, the newest 3 outcomes
// all right, and checked successes on at least KnownStudyDays study days
// (ADR-0031 D1). Counts use LadderOf. This stays so StateOf and KnownRule4
// can still name the old rule (ADR-0033 D4).
func IsKnown(e Evidence) bool {
	return e.Retrievability >= KnownRetrievability &&
		e.Recent >= KnownRecentRight && e.RecentRight >= KnownRecentRight &&
		e.SuccessDays >= KnownStudyDays
}

// KnownRule4 is rule version 4. It stays callable so a comparison can show the
// old count beside the ladder (ADR-0033 D4). It does not decay a ladder rung.
func KnownRule4(e Evidence) bool { return IsKnown(e) }

// SameDayKnown is the rule-version-4 check without the second study day.
// The thresholds live in internal/learner/rule. skillOldKnown calls that package.
func SameDayKnown(e Evidence) bool { return rule.SameDayKnown(ruleInput(e)) }

// LadderOf is the proved ladder for one word in one skill (ADR-0033 D4).
// ok is false when the word is not taught; that word is not counted.
// A voided answer is already absent from e. Recall chance is not read.
//
//	proved: right on at least 3 study days spanning at least 7 days, newest answer right
//	recalled later: right on a study day after the taught day, and not proved
//	slipping: that proved or recalled-later bar was met, and the newest answer is wrong
//	not yet asked: taught, and neither bar was met
func LadderOf(e Evidence) (Ladder, bool) {
	rung, ok := rule.Ladder(ruleInput(e))
	return Ladder(rung), ok
}

// RecalledOrProved reports the looser rung: recalled later or proved.
// Cold-check passes sit here and are not proved (ADR-0033 D6, D7).
func RecalledOrProved(e Evidence) bool { return rule.RecalledOrProved(ruleInput(e)) }

func ruleInput(e Evidence) rule.Input {
	return rule.Input{
		Outcomes: e.Outcomes, Recent: e.Recent, RecentRight: e.RecentRight,
		Successes: e.Successes, SuccessDays: e.SuccessDays, HasCard: e.HasCard,
		LatestRight: e.LatestRight, Taught: e.Taught, Retrievability: e.Retrievability,
		First: e.FirstSuccessDay, Last: e.LastSuccessDay, TaughtDay: e.TaughtDay,
	}
}

// StateOf is the state of one word in one skill. The rules are checked in this
// order and the first that applies wins:
//
//  1. no outcome and no card: seen when the word was exposed, otherwise unseen
//  2. struggling: 3 or more Again in the newest 8, or accuracy under 60% over at least 5
//  3. regressed: the card was learned, and recall chance is under 70%
//  4. no card: seen (answered, but nothing is scheduled yet)
//  5. a card younger than its third successful review: learning
//  6. known (including two study days of checked success) and stable for 21 days or more: solid
//  7. known: known
//  8. anything else: learning (including old same-day known that still needs a later day)
//
// Struggling comes before regressed, as north-star 4.3 says: a word that is both is struggling.
func StateOf(e Evidence) State {
	if !e.HasEvidence() {
		if e.Exposed {
			return Seen
		}
		return Unseen
	}
	if IsStruggling(e) {
		return Struggling
	}
	if WasLearned(e) && e.Retrievability < RegressedRetrievability {
		return Regressed
	}
	if !e.HasCard {
		return Seen
	}
	if e.Successes < LearningSuccesses {
		return Learning
	}
	if IsKnown(e) {
		if e.Stability >= SolidStabilityDays {
			return Solid
		}
		return Known
	}
	return Learning
}

// Overall is the weakest of the given states, by the order of States. The caller
// passes only the skills that have evidence. With none it returns Unseen.
func Overall(skills ...State) State {
	if len(skills) == 0 {
		return Unseen
	}
	weakest := skills[0]
	for _, s := range skills[1:] {
		if s.rank() < weakest.rank() {
			weakest = s
		}
	}
	return weakest
}
