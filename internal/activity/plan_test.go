package activity

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

var (
	allFacts = Facts{HasGloss: true, HasAudio: true, GlossedOthers: 50, VoicedOthers: 50}
	open     = Gate{LettersPassed: true}
)

func view(skill string, state learner.State, accuracy float64, attempts int) SkillView {
	return SkillView{Skill: skill, State: state, Accuracy: accuracy, Attempts: attempts, HasCard: attempts > 0}
}

func TestSpecTable(t *testing.T) {
	want := []struct {
		kind  Kind
		skill string
		typed bool
		multi bool
	}{
		{ListenPick, fsrs.SkillListen, false, false},
		{ReadPick, fsrs.SkillRead, false, false},
		{WordMatch, fsrs.SkillRead, false, true},
		{AudioMatch, fsrs.SkillListen, false, true},
		{TypeIt, fsrs.SkillRecall, true, false},
		{Dictation, fsrs.SkillSpell, true, false},
	}
	for _, w := range want {
		s, ok := SpecOf(w.kind)
		if !ok || s.Skill != w.skill || !s.Checked || s.Typed != w.typed || s.Multi != w.multi {
			t.Errorf("SpecOf(%s) = %+v, %v", w.kind, s, ok)
		}
	}
	if s, ok := SpecOf(Flashcard); !ok || s.Checked || s.Skill != "" {
		t.Errorf("flashcard = %+v, %v", s, ok)
	}
	if _, ok := SpecOf("cloze"); ok {
		t.Error("an unknown kind has a spec")
	}
}

func TestFits(t *testing.T) {
	for _, tc := range []struct {
		name  string
		facts Facts
		fits  map[Kind]bool
	}{
		{"everything", allFacts, map[Kind]bool{
			ListenPick: true, ReadPick: true, WordMatch: true, AudioMatch: true, TypeIt: true, Dictation: true}},
		{"no audio", Facts{HasGloss: true, GlossedOthers: 50}, map[Kind]bool{
			ReadPick: true, WordMatch: true, TypeIt: true}},
		{"no gloss", Facts{HasAudio: true, GlossedOthers: 50, VoicedOthers: 50}, map[Kind]bool{Dictation: true}},
		{"too few other glosses", Facts{HasGloss: true, HasAudio: true, GlossedOthers: 2, VoicedOthers: 2}, map[Kind]bool{
			TypeIt: true, Dictation: true}},
		{"enough for a pick, not a match", Facts{HasGloss: true, HasAudio: true, GlossedOthers: 3, VoicedOthers: 3}, map[Kind]bool{
			ListenPick: true, ReadPick: true, AudioMatch: true, TypeIt: true, Dictation: true}},
		{"nothing", Facts{}, map[Kind]bool{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, s := range Specs() {
				if got := tc.facts.Fits(s.Kind); got != tc.fits[s.Kind] {
					t.Errorf("Fits(%s) = %v, want %v", s.Kind, got, tc.fits[s.Kind])
				}
			}
		})
	}
}

func TestGate(t *testing.T) {
	closed := Gate{}
	if !closed.Open(fsrs.SkillListen) {
		t.Error("listening is open from the start")
	}
	for _, s := range []string{fsrs.SkillRead, fsrs.SkillRecall, fsrs.SkillSpell} {
		if closed.Open(s) {
			t.Errorf("%s is open before the letters check", s)
		}
		if !open.Open(s) {
			t.Errorf("%s is shut after the letters check", s)
		}
	}
}

func TestWeakestSkills(t *testing.T) {
	for _, tc := range []struct {
		name  string
		views []SkillView
		gate  Gate
		facts Facts
		want  []string
	}{
		{
			name: "lowest accuracy at the same state goes first",
			views: []SkillView{
				view("listen", learner.Learning, 0.3, 6), view("read", learner.Learning, 0.9, 6),
				view("recall", learner.Learning, 0.6, 6), view("spell", learner.Learning, 0.7, 6),
			},
			gate: open, facts: allFacts, want: []string{"listen", "recall", "spell", "read"},
		},
		{
			name: "a weaker state beats a better accuracy",
			views: []SkillView{
				view("listen", learner.Known, 0.5, 6), view("read", learner.Struggling, 0.55, 6),
				view("recall", learner.Solid, 0.99, 6), view("spell", learner.Learning, 0.9, 6),
			},
			gate: open, facts: allFacts, want: []string{"read", "spell", "listen", "recall"},
		},
		{
			name: "a new word is asked through listening first",
			views: []SkillView{
				{Skill: "listen", State: learner.Unseen}, {Skill: "read", State: learner.Unseen},
				{Skill: "recall", State: learner.Unseen}, {Skill: "spell", State: learner.Unseen},
			},
			gate: open, facts: allFacts, want: []string{"listen", "read", "recall", "spell"},
		},
		{
			name: "a skill with no evidence is not opened while another is failing",
			views: []SkillView{
				view("listen", learner.Struggling, 0.2, 8), view("read", learner.Known, 0.9, 8),
				{Skill: "recall", State: learner.Unseen}, {Skill: "spell", State: learner.Unseen},
			},
			gate: open, facts: allFacts, want: []string{"listen", "read"},
		},
		{
			name: "regressed counts as failing",
			views: []SkillView{
				view("listen", learner.Regressed, 0.5, 8), view("read", learner.Known, 0.9, 8),
				{Skill: "recall", State: learner.Unseen}, {Skill: "spell", State: learner.Unseen},
			},
			gate: open, facts: allFacts, want: []string{"listen", "read"},
		},
		{
			name: "a skill with no evidence is opened when the others are fine",
			views: []SkillView{
				view("listen", learner.Known, 0.9, 8), view("read", learner.Learning, 0.8, 8),
				{Skill: "recall", State: learner.Unseen}, {Skill: "spell", State: learner.Unseen},
			},
			gate: open, facts: allFacts, want: []string{"recall", "spell", "read", "listen"},
		},
		{
			name: "before the letters check only listening is asked",
			views: []SkillView{
				view("listen", learner.Known, 0.9, 8), view("read", learner.Struggling, 0.1, 8),
				view("recall", learner.Struggling, 0.1, 8), view("spell", learner.Struggling, 0.1, 8),
			},
			gate: Gate{}, facts: allFacts, want: []string{"listen"},
		},
		{
			name: "a skill with no activity that fits is left out",
			views: []SkillView{
				view("listen", learner.Learning, 0.9, 8), view("read", learner.Learning, 0.1, 8),
				view("recall", learner.Learning, 0.5, 8), view("spell", learner.Learning, 0.5, 8),
			},
			gate: open, facts: Facts{HasGloss: true}, // no clip and no other words: only typing fits
			want: []string{"recall"},
		},
		{
			name:  "no model data ranks every skill as unseen",
			views: nil,
			gate:  open, facts: allFacts, want: []string{"listen", "read", "recall", "spell"},
		},
		{
			name:  "nothing fits",
			views: []SkillView{view("read", learner.Learning, 0.5, 4)},
			gate:  open, facts: Facts{}, want: []string{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := WeakestSkills(tc.views, tc.gate, tc.facts)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("WeakestSkills = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestChoose(t *testing.T) {
	// ac-2: the mix picks listen_pick for the word whose listen accuracy is lowest.
	views := []SkillView{
		view("listen", learner.Learning, 0.25, 8), view("read", learner.Learning, 0.9, 8),
		view("recall", learner.Learning, 0.8, 8), view("spell", learner.Learning, 0.7, 8),
	}
	got := Choose(views, open, allFacts, nil, nil, fsrs.SkillRead)
	if got.Kind != ListenPick || got.Skill != "listen" {
		t.Fatalf("Choose = %+v, want listen_pick", got)
	}

	t.Run("the third visit through a skill is its match activity", func(t *testing.T) {
		visits := map[string]int{}
		var kinds []Kind
		for i := 0; i < 6; i++ {
			c := Choose(views, open, allFacts, nil, visits, fsrs.SkillRead)
			kinds = append(kinds, c.Kind)
			visits[c.Skill]++
		}
		want := []Kind{ListenPick, ListenPick, AudioMatch, ListenPick, ListenPick, AudioMatch}
		if !reflect.DeepEqual(kinds, want) {
			t.Fatalf("kinds = %v, want %v", kinds, want)
		}
	})

	t.Run("a match activity that does not fit is skipped", func(t *testing.T) {
		f := Facts{HasGloss: true, HasAudio: true, GlossedOthers: 10, VoicedOthers: 1}
		visits := map[string]int{"listen": 2}
		if c := Choose(views, open, f, nil, visits, fsrs.SkillRead); c.Kind != ListenPick {
			t.Fatalf("Choose = %+v, want listen_pick: too few clips for audio_match", c)
		}
	})

	t.Run("a word asked before is asked through another skill", func(t *testing.T) {
		c := Choose(views, open, allFacts, map[string]bool{"listen": true}, nil, fsrs.SkillRead)
		if c.Skill != "spell" || c.Kind != Dictation {
			t.Fatalf("Choose = %+v, want the next weakest skill", c)
		}
		all := map[string]bool{"listen": true, "read": true, "recall": true, "spell": true}
		if c := Choose(views, open, allFacts, all, nil, fsrs.SkillRead); c.Skill != "listen" {
			t.Fatalf("Choose = %+v, want the avoid list ignored when every skill is in it", c)
		}
	})

	t.Run("flashcard only when no checked activity fits", func(t *testing.T) {
		c := Choose(views, open, Facts{}, nil, nil, fsrs.SkillRecall)
		if c.Kind != Flashcard || c.Skill != fsrs.SkillRecall {
			t.Fatalf("Choose = %+v, want a flashcard through the card's skill", c)
		}
	})

	t.Run("never a flashcard when a checked activity fits", func(t *testing.T) {
		rng := rand.New(rand.NewPCG(7, 7))
		states := []learner.State{learner.Unseen, learner.Seen, learner.Struggling, learner.Regressed, learner.Learning, learner.Known, learner.Solid}
		for n := 0; n < 5000; n++ {
			var vs []SkillView
			for _, s := range Skills {
				state := states[rng.IntN(len(states))]
				attempts := rng.IntN(3) * 4
				vs = append(vs, SkillView{Skill: s, State: state, Accuracy: rng.Float64(), Attempts: attempts, HasCard: attempts > 0})
			}
			facts := Facts{
				HasGloss: rng.IntN(4) > 0, HasAudio: rng.IntN(4) > 0,
				GlossedOthers: rng.IntN(8), VoicedOthers: rng.IntN(8),
			}
			gate := Gate{LettersPassed: rng.IntN(2) == 0}
			fits := false
			for _, s := range Skills {
				if gate.Open(s) && len(Fitting(s, facts)) > 0 {
					fits = true
				}
			}
			c := Choose(vs, gate, facts, nil, map[string]int{"listen": rng.IntN(5), "read": rng.IntN(5)}, fsrs.SkillRead)
			if fits && c.Kind == Flashcard {
				t.Fatalf("flashcard chosen although a checked activity fits: views %+v facts %+v gate %+v", vs, facts, gate)
			}
			if !fits && c.Kind != Flashcard {
				t.Fatalf("%+v chosen although nothing fits", c)
			}
			if c.Kind != Flashcard {
				if spec, _ := SpecOf(c.Kind); spec.Skill != c.Skill || !facts.Fits(c.Kind) || !gate.Open(c.Skill) {
					t.Fatalf("choice %+v does not fit: facts %+v gate %+v", c, facts, gate)
				}
			}
		}
	})
}

func TestViewsOf(t *testing.T) {
	w := learner.WordState{Skills: []learner.SkillState{
		{Skill: "listen", State: learner.Known, Attempts: 4, Accuracy: 0.75, HasCard: true},
		{Skill: "read", State: learner.Unseen},
	}}
	got := ViewsOf(w)
	want := []SkillView{
		{Skill: "listen", State: learner.Known, Accuracy: 0.75, Attempts: 4, HasCard: true},
		{Skill: "read", State: learner.Unseen},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ViewsOf = %+v", got)
	}
	if got[1].HasEvidence() || !got[0].HasEvidence() {
		t.Fatal("HasEvidence is wrong")
	}
}
