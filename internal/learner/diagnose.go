package learner

import (
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// Cause is the likely reason a word is a struggle, from the pattern of its skills
// (docs/north-star.md section 4.5). The empty Cause means no pattern fits.
type Cause string

const (
	CauseNone       Cause = ""           // Not struggling, or no pattern fits.
	CauseDecoding   Cause = "decoding"   // Hears it, can't read it.
	CauseProduction Cause = "production" // Reads it, can't recall it.
	CauseSpelling   Cause = "spelling"   // Recalls it, can't spell it.
	CauseConfusion  Cause = "confusion"  // One pair of words mixed up again and again.
	CauseBandSlip   Cause = "band_slip"  // Its whole frequency band is slipping.
)

// The diagnosis thresholds. thresholds_test.go pins them.
const (
	// ConfusionPairMin is how many times one pair must be confused to count as one
	// confusion pair. The pair must also be more than half of the word's confusions,
	// so a word mixed up with many others is not "one pair".
	ConfusionPairMin = 3

	// ConfusionWindowDays is how far back confusion_misses rows count toward a diagnosis.
	ConfusionWindowDays = 30

	// BandSlipWords is how many words of one band must be struggling or regressed, and
	// seen in the last BandSlipDays, before the band counts as slipping.
	BandSlipWords = 5
	BandSlipDays  = 7
)

// Confusion is one time the word was confused with another. Kind and Key are the
// confusion_misses columns. Only its Key and Kind matter to Diagnose.
type Confusion struct {
	Kind string
	Key  string
	At   time.Time
}

// Rule is one named diagnosis pattern.
type Rule struct {
	Name  string
	Cause Cause
	// Match reports whether the pattern fits. It reads only the word and its confusions.
	Match func(w WordState, confusions []Confusion) bool
}

// rules are the five patterns, in the order Diagnose tries them. Each is written so
// that it fits only its own pattern: the three skill rules each need one skill good and
// the next one weak, the confusion rule needs repeated confusions, and the band rule
// needs the word's band to be slipping. A word can fit more than one only when its
// evidence really shows more than one thing, and then the first listed wins: the most
// specific evidence (a named pair) before the skill funnel, and the band last, because
// it says the least about this one word.
var rules = []Rule{
	{
		Name:  "one_confusion_pair",
		Cause: CauseConfusion,
		Match: func(_ WordState, confusions []Confusion) bool {
			_, _, ok := DominantPair(confusions)
			return ok
		},
	},
	{
		Name:  "hears_but_cant_read",
		Cause: CauseDecoding,
		Match: func(w WordState, _ []Confusion) bool {
			return skillGood(w, fsrs.SkillListen) && skillWeak(w, fsrs.SkillRead)
		},
	},
	{
		Name:  "reads_but_cant_recall",
		Cause: CauseProduction,
		Match: func(w WordState, _ []Confusion) bool {
			return skillGood(w, fsrs.SkillRead) && skillWeak(w, fsrs.SkillRecall)
		},
	},
	{
		Name:  "recalls_but_cant_spell",
		Cause: CauseSpelling,
		Match: func(w WordState, _ []Confusion) bool {
			return skillGood(w, fsrs.SkillRecall) && skillWeak(w, fsrs.SkillSpell)
		},
	},
	{
		Name:  "band_slip",
		Cause: CauseBandSlip,
		Match: func(w WordState, _ []Confusion) bool {
			return w.Band >= 0 && w.BandStrugglers >= BandSlipWords
		},
	},
}

// Rules returns the diagnosis rules in the order Diagnose tries them.
func Rules() []Rule { return append([]Rule(nil), rules...) }

// Diagnose names the likely cause of a word's struggle. Only a word that is struggling
// or regressed has one. confusions are the times the word was confused with another
// inside ConfusionWindowDays. The cause is that of the first rule that fits, or CauseNone.
func Diagnose(w WordState, confusions []Confusion) Cause {
	if w.State != Struggling && w.State != Regressed {
		return CauseNone
	}
	for _, r := range rules {
		if r.Match(w, confusions) {
			return r.Cause
		}
	}
	return CauseNone
}

// DominantPair finds the one pair a word is confused with again and again: the key
// seen at least ConfusionPairMin times and in more than half of all the confusions.
func DominantPair(confusions []Confusion) (key string, count int, ok bool) {
	type pair struct{ kind, key string }
	counts := map[pair]int{}
	var top pair
	for _, c := range confusions {
		p := pair{c.Kind, c.Key}
		counts[p]++
		if counts[p] > counts[top] {
			top = p
		}
	}
	n := counts[top]
	if n < ConfusionPairMin || n*2 <= len(confusions) {
		return "", 0, false
	}
	return top.key, n, true
}

// skillGood is a skill the learner has: known or solid.
func skillGood(w WordState, skill string) bool {
	s := w.stateOf(skill)
	return s == Known || s == Solid
}

// skillWeak is a skill the learner is losing: struggling or regressed.
func skillWeak(w WordState, skill string) bool {
	s := w.stateOf(skill)
	return s == Struggling || s == Regressed
}
