package activity

import (
	"slices"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

// Facts is what the planner needs to know about one word to say which activities can
// ask it. They are counts, not the words themselves, so the planner stays pure.
//
// HasGloss and HasAudio are about the word. GlossedOthers is how many other words have a
// gloss, and VoicedOthers how many other words have a gloss and a clip. Fits uses them as
// an upper bound: Build can still refuse with ErrNotEnoughWords when the glosses overlap.
type Facts struct {
	HasGloss      bool
	HasAudio      bool
	GlossedOthers int
	VoicedOthers  int
}

// Fits reports whether the activity can ask the word.
//
//   - listen_pick: the word's clip, and three glossed words for the other options
//   - read_pick: the word's gloss, and three other glossed words
//   - word_match: the word's gloss, and four other glossed words
//   - audio_match: the word's clip and gloss, and three other words with clip and gloss
//   - type_it: the word's gloss (recall, or spelling when there is no clip)
//   - dictation: the word's clip
func (f Facts) Fits(k Kind) bool {
	switch k {
	case ListenPick:
		return f.HasAudio && f.HasGloss && f.GlossedOthers >= OptionCount-1
	case ReadPick:
		return f.HasGloss && f.GlossedOthers >= OptionCount-1
	case WordMatch:
		return f.HasGloss && f.GlossedOthers >= WordMatchPairs-1
	case AudioMatch:
		return f.HasAudio && f.HasGloss && f.VoicedOthers >= AudioMatchClips-1
	case TypeIt:
		return f.HasGloss
	case Dictation:
		return f.HasAudio
	default:
		return false
	}
}

// Fitting lists the checked activities of a skill that can ask the word, in table order.
// Spelling prefers dictation when a clip exists; type_it is only the no-clip fallback
// (ADR-0028 D6), so it is not listed beside dictation as a match rotation.
func Fitting(skill string, f Facts) []Kind {
	var out []Kind
	for _, k := range kindsFor(skill) {
		if f.Fits(k) {
			out = append(out, k)
		}
	}
	if skill == fsrs.SkillSpell {
		for _, k := range out {
			if k == Dictation {
				return []Kind{Dictation}
			}
		}
	}
	return out
}

// SkillView is one skill of one word, as the learner model reports it.
type SkillView struct {
	Skill    string
	State    learner.State
	Accuracy float64
	Attempts int
	HasCard  bool
}

// HasEvidence is true when the skill has an attempt or a card.
func (v SkillView) HasEvidence() bool { return v.Attempts > 0 || v.HasCard }

// ViewsOf turns a learner.WordState into the skill views the planner reads.
func ViewsOf(w learner.WordState) []SkillView {
	out := make([]SkillView, 0, len(w.Skills))
	for _, s := range w.Skills {
		out = append(out, SkillView{
			Skill: s.Skill, State: s.State, Accuracy: s.Accuracy, Attempts: s.Attempts, HasCard: s.HasCard,
		})
	}
	return out
}

func rankOf(s learner.State) int {
	if i := slices.Index(learner.States, s); i >= 0 {
		return i
	}
	return 0
}

// WeakestSkills orders the skills a word can be asked through, weakest first.
//
// Only a skill that is open (the gate) and has a checked activity that fits the word is
// listed. A skill with evidence ranks by its state in the order of learner.States
// (unseen, seen, struggling, regressed, learning, known, solid), then by accuracy, lowest
// first, then by the order of Skills. A skill with no evidence ranks as unseen, so a new
// word is asked through listening first. A skill with no evidence is added to a word that
// has others only while none of those is struggling or regressed: a word that is failing
// in one skill is not opened in another. When nothing is left the open, fitting skills are
// ranked as they are.
func WeakestSkills(views []SkillView, gate Gate, f Facts) []string {
	byName := map[string]SkillView{}
	for _, v := range views {
		byName[v.Skill] = v
	}
	var eligible []SkillView
	for _, skill := range Skills {
		if !gate.Open(skill) || len(Fitting(skill, f)) == 0 {
			continue
		}
		v, ok := byName[skill]
		if !ok {
			v = SkillView{Skill: skill, State: learner.Unseen}
		}
		eligible = append(eligible, v)
	}

	failing := false
	for _, v := range views {
		if v.HasEvidence() && (v.State == learner.Struggling || v.State == learner.Regressed) {
			failing = true
		}
	}
	candidates := eligible
	if failing {
		var kept []SkillView
		for _, v := range eligible {
			if v.HasEvidence() {
				kept = append(kept, v)
			}
		}
		if len(kept) > 0 {
			candidates = kept
		}
	}
	candidates = slices.Clone(candidates)
	slices.SortStableFunc(candidates, func(a, b SkillView) int {
		ra, rb := rankOfView(a), rankOfView(b)
		if ra != rb {
			return ra - rb
		}
		switch {
		case a.Accuracy < b.Accuracy:
			return -1
		case a.Accuracy > b.Accuracy:
			return 1
		}
		return slices.Index(Skills, a.Skill) - slices.Index(Skills, b.Skill)
	})
	out := make([]string, len(candidates))
	for i, v := range candidates {
		out[i] = v.Skill
	}
	return out
}

func rankOfView(v SkillView) int {
	if !v.HasEvidence() {
		return rankOf(learner.Unseen)
	}
	return rankOf(v.State)
}

// Choice is the activity the mix picked for one visit.
type Choice struct {
	// Skill is the skill the activity trains. For Flashcard it is the card's own skill.
	Skill string
	Kind  Kind
}

// Choose picks the skill and the activity for one visit of a word.
//
// It takes the weakest skill that has a checked activity that fits the word and is not in
// avoid, which holds the skills this word has already been asked through in the same
// session. When every skill is in avoid the avoid list is ignored, so a word with two due
// cards is asked through two different skills while it can be. The activity is the skill's
// everyday one, and its match activity on every MatchEvery-th visit through that skill:
// visits counts the visits already given to the skill in this queue. A checked activity
// always beats a flashcard. Only when no checked activity fits at all is the visit a
// Flashcard, through cardSkill.
func Choose(views []SkillView, gate Gate, f Facts, avoid map[string]bool, visits map[string]int, cardSkill string) Choice {
	ranked := WeakestSkills(views, gate, f)
	var skill string
	for _, s := range ranked {
		if !avoid[s] {
			skill = s
			break
		}
	}
	if skill == "" && len(ranked) > 0 {
		skill = ranked[0]
	}
	if skill == "" {
		return Choice{Skill: cardSkill, Kind: Flashcard}
	}
	kinds := Fitting(skill, f)
	kind := kinds[0]
	if len(kinds) > 1 && (visits[skill]+1)%MatchEvery == 0 {
		kind = kinds[1]
	}
	return Choice{Skill: skill, Kind: kind}
}

// Implies reports the harder skill that covers skill: recall covers read, and spell
// covers listen. INIT-027/SPEC-004.
func Implies(skill string) (string, bool) {
	switch skill {
	case fsrs.SkillRead:
		return fsrs.SkillRecall, true
	case fsrs.SkillListen:
		return fsrs.SkillSpell, true
	default:
		return "", false
	}
}

// ChooseSkill picks the activity for one skill in a review visit (ADR-0028 D6).
// Within a skill the everyday kind and the match kind alternate by position in
// that skill's run (visits already counted for the skill). When the gate is
// closed or no checked activity fits, the visit is a Flashcard through that skill.
// Lesson planning still uses Choose, which keeps MatchEvery.
func ChooseSkill(skill string, gate Gate, f Facts, visits map[string]int) Choice {
	if skill == "" || !gate.Open(skill) {
		return Choice{Skill: skill, Kind: Flashcard}
	}
	kinds := Fitting(skill, f)
	if len(kinds) == 0 {
		return Choice{Skill: skill, Kind: Flashcard}
	}
	kind := kinds[0]
	if len(kinds) > 1 && visits[skill]%2 == 1 {
		kind = kinds[1]
	}
	return Choice{Skill: skill, Kind: kind}
}
