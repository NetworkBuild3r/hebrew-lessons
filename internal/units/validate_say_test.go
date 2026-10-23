package units

import (
	"context"
	"strings"
	"testing"
)

func TestSayStepNeedsPromptAndAudio(t *testing.T) {
	u, lk := v1()
	word := u.NewWords[0]
	act := &u.Lessons[0].Activities[0]
	act.Type = ActivitySay
	act.Text = word
	act.Target = ""
	act.Lines = nil
	act.PromptEN = "good morning"
	lk.audio[normalize(word)] = true
	if got := Validate(context.Background(), &u, lk); len(got) != 0 {
		t.Fatalf("a say step with a cue and a clip: %#v", got)
	}

	act.PromptEN = ""
	got := Validate(context.Background(), &u, lk)
	if !hasRule(got, RuleSayPrompt) {
		t.Fatalf("missing cue: %#v", got)
	}

	act.PromptEN = strings.Repeat("a", PromptMaxRunes+1)
	got = Validate(context.Background(), &u, lk)
	if !hasRule(got, RuleSayPrompt) {
		t.Fatalf("long cue: %#v", got)
	}

	act.PromptEN = "\u05e9\u05b8\u05dc\u05d5\u05b9\u05dd"
	got = Validate(context.Background(), &u, lk)
	if !hasRule(got, RuleSayPrompt) {
		t.Fatalf("Hebrew cue: %#v", got)
	}

	act.PromptEN = "good morning"
	lk.audio = nil
	got = Validate(context.Background(), &u, lk)
	if !hasRule(got, RuleSayAudio) {
		t.Fatalf("missing clip: %#v", got)
	}
}
