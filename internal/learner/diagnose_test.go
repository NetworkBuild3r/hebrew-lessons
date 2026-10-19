package learner

import (
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// word builds a WordState from skill states. A skill not listed is unseen. The overall
// state is the weakest listed skill, as the model computes it.
func word(band, bandStrugglers int, skills map[string]State) WordState {
	w := WordState{LexemeID: 1, Band: band, BandStrugglers: bandStrugglers}
	var listed []State
	for _, skill := range Skills {
		s, ok := skills[skill]
		if !ok {
			s = Unseen
		} else {
			listed = append(listed, s)
		}
		w.Skills = append(w.Skills, SkillState{Skill: skill, State: s})
	}
	w.State = Overall(listed...)
	return w
}

func confusions(key string, n int) []Confusion {
	out := make([]Confusion, n)
	for i := range out {
		out[i] = Confusion{Kind: fsrs.MissLetterPair, Key: key, At: time.Date(2026, 10, 1+i, 0, 0, 0, 0, time.UTC)}
	}
	return out
}

// pattern is evidence that fits exactly one cause.
type pattern struct {
	name  string
	cause Cause
	rule  string
	w     WordState
	conf  []Confusion
}

func patterns() []pattern {
	return []pattern{
		{
			name: "hears but can't read", cause: CauseDecoding, rule: "hears_but_cant_read",
			w: word(-1, 0, map[string]State{fsrs.SkillListen: Known, fsrs.SkillRead: Struggling}),
		},
		{
			name: "reads but can't recall", cause: CauseProduction, rule: "reads_but_cant_recall",
			w: word(-1, 0, map[string]State{fsrs.SkillRead: Solid, fsrs.SkillRecall: Regressed}),
		},
		{
			name: "recalls but can't spell", cause: CauseSpelling, rule: "recalls_but_cant_spell",
			w: word(-1, 0, map[string]State{fsrs.SkillRecall: Known, fsrs.SkillSpell: Struggling}),
		},
		{
			name: "one confusion pair", cause: CauseConfusion, rule: "one_confusion_pair",
			w:    word(-1, 0, map[string]State{fsrs.SkillListen: Struggling}),
			conf: confusions("ב/כ", 3),
		},
		{
			name: "a band slip", cause: CauseBandSlip, rule: "band_slip",
			w: word(1, BandSlipWords, map[string]State{fsrs.SkillListen: Struggling}),
		},
	}
}

// ac-2: each of the five causes is produced by its pattern, and not by the other four.
func TestEachCauseComesFromItsOwnPatternOnly(t *testing.T) {
	ps := patterns()
	if len(ps) != 5 || len(rules) != 5 {
		t.Fatalf("%d patterns and %d rules, want 5 of each", len(ps), len(rules))
	}
	seenCause := map[Cause]bool{}
	for _, p := range ps {
		t.Run(p.name, func(t *testing.T) {
			if got := Diagnose(p.w, p.conf); got != p.cause {
				t.Fatalf("Diagnose = %q, want %q", got, p.cause)
			}
			seenCause[p.cause] = true
			// Exactly one rule fits this evidence, and it is the pattern's own.
			var fits []string
			for _, r := range rules {
				if r.Match(p.w, p.conf) {
					fits = append(fits, r.Name)
				}
			}
			if len(fits) != 1 || fits[0] != p.rule {
				t.Fatalf("rules that fit = %v, want only %s", fits, p.rule)
			}
		})
	}
	if len(seenCause) != 5 {
		t.Fatalf("causes produced = %v, want five different ones", seenCause)
	}
}

// Every other pattern's evidence must not produce this pattern's cause, cross-checked in full.
func TestNoPatternProducesAnotherPatternsCause(t *testing.T) {
	ps := patterns()
	for _, a := range ps {
		for _, b := range ps {
			if a.name == b.name {
				continue
			}
			for _, r := range rules {
				if r.Cause == a.cause && r.Match(b.w, b.conf) {
					t.Errorf("rule %s (cause %s) fits the evidence of %q", r.Name, r.Cause, b.name)
				}
			}
		}
	}
}

// Near misses of each pattern give no cause at all.
func TestNearMissesGiveNoCause(t *testing.T) {
	tests := []struct {
		name string
		w    WordState
		conf []Confusion
	}{
		{"read is only learning, not weak", word(-1, 0, map[string]State{fsrs.SkillListen: Known, fsrs.SkillRead: Learning, fsrs.SkillRecall: Struggling}), nil},
		{"decoding needs listen known, and it is only learning", word(-1, 0, map[string]State{fsrs.SkillListen: Learning, fsrs.SkillRead: Struggling}), nil},
		{"production needs read good, and recall weak", word(-1, 0, map[string]State{fsrs.SkillRead: Known, fsrs.SkillRecall: Learning, fsrs.SkillListen: Struggling}), nil},
		{"spelling needs recall good", word(-1, 0, map[string]State{fsrs.SkillRecall: Learning, fsrs.SkillSpell: Struggling}), nil},
		{"two of the same pair is under the minimum", word(-1, 0, map[string]State{fsrs.SkillListen: Struggling}), confusions("ב/כ", 2)},
		{"4 band strugglers is under the minimum", word(1, BandSlipWords-1, map[string]State{fsrs.SkillListen: Struggling}), nil},
		{"a band slip needs a known band", word(-1, BandSlipWords, map[string]State{fsrs.SkillListen: Struggling}), nil},
		{"the wrong way round: read good, listen weak is not decoding", word(-1, 0, map[string]State{fsrs.SkillRead: Known, fsrs.SkillListen: Struggling}), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Diagnose(tc.w, tc.conf); got != CauseNone {
				t.Fatalf("Diagnose = %q, want none", got)
			}
		})
	}
}

// Only a struggling or regressed word has a cause, whatever its skills look like.
func TestOnlyAStrugglingWordIsDiagnosed(t *testing.T) {
	for _, p := range patterns() {
		for _, state := range []State{Unseen, Seen, Learning, Known, Solid} {
			w := p.w
			w.State = state
			if got := Diagnose(w, p.conf); got != CauseNone {
				t.Errorf("%s with state %s: cause %q, want none", p.name, state, got)
			}
		}
		for _, state := range []State{Struggling, Regressed} {
			w := p.w
			w.State = state
			if got := Diagnose(w, p.conf); got != p.cause {
				t.Errorf("%s with state %s: cause %q, want %q", p.name, state, got, p.cause)
			}
		}
	}
}

// When evidence fits more than one rule, the first rule listed wins.
func TestRulePrecedence(t *testing.T) {
	both := word(1, BandSlipWords, map[string]State{fsrs.SkillListen: Known, fsrs.SkillRead: Struggling})
	if got := Diagnose(both, nil); got != CauseDecoding {
		t.Errorf("decoding and band slip: %q, want decoding", got)
	}
	if got := Diagnose(both, confusions("א/ע", 4)); got != CauseConfusion {
		t.Errorf("confusion and decoding: %q, want confusion", got)
	}
	chain := word(-1, 0, map[string]State{fsrs.SkillListen: Known, fsrs.SkillRead: Known, fsrs.SkillRecall: Struggling, fsrs.SkillSpell: Struggling})
	if got := Diagnose(chain, nil); got != CauseProduction {
		t.Errorf("production and (recall weak, spell weak): %q, want production", got)
	}
	names := []string{}
	for _, r := range Rules() {
		names = append(names, r.Name)
	}
	want := []string{"one_confusion_pair", "hears_but_cant_read", "reads_but_cant_recall", "recalls_but_cant_spell", "band_slip"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("rule order = %v, want %v", names, want)
		}
	}
}

func TestDominantPair(t *testing.T) {
	mix := func(parts ...[]Confusion) []Confusion {
		var out []Confusion
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	tests := []struct {
		name  string
		in    []Confusion
		key   string
		count int
		ok    bool
	}{
		{"none", nil, "", 0, false},
		{"2 of 2 is under the minimum", confusions("א/ע", 2), "", 0, false},
		{"3 of 3", confusions("א/ע", 3), "א/ע", 3, true},
		{"3 of 5 is more than half", mix(confusions("א/ע", 3), confusions("ב/כ", 1), confusions("ס/ש", 1)), "א/ע", 3, true},
		{"3 of 6 is exactly half, not more", mix(confusions("א/ע", 3), confusions("ב/כ", 1), confusions("ס/ש", 1), confusions("ד/ר", 1)), "", 0, false},
		{"3 of 3 different pairs", mix(confusions("א/ע", 1), confusions("ב/כ", 1), confusions("ס/ש", 1)), "", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, count, ok := DominantPair(tc.in)
			if key != tc.key || count != tc.count || ok != tc.ok {
				t.Fatalf("DominantPair = %q, %d, %v; want %q, %d, %v", key, count, ok, tc.key, tc.count, tc.ok)
			}
		})
	}
}
