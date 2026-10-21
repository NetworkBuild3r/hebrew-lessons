package units

import (
	"context"
	"strings"
	"testing"
)

func TestActivityTarget(t *testing.T) {
	u, lk := v1()
	li, ai := 0, 2 // listen_pick of the first two new words
	act := u.Lessons[li].Activities[ai]
	if act.Type != ActivityListenPick {
		t.Fatalf("activity %d is %s", ai, act.Type)
	}
	toks := tokenize(act.Text)
	if len(toks) < 2 {
		t.Fatalf("tokens %#v", toks)
	}
	second := toks[1]

	if got := Validate(context.Background(), &u, lk); len(got) != 0 {
		t.Fatalf("baseline %s", mustJSON(got))
	}
	u.Lessons[li].Activities[ai].Target = "   "
	if got := targetOnly(Validate(context.Background(), &u, lk)); len(got) != 0 {
		t.Fatalf("listen_pick may omit target %s", mustJSON(got))
	}
	u.Lessons[li].Activities[ai].Type = ActivityRecall
	got := targetOnly(Validate(context.Background(), &u, lk))
	if len(got) != 1 || got[0].Severity != SeverityBlock || !strings.Contains(got[0].Msg, "several words") {
		t.Fatalf("blank target %s", mustJSON(got))
	}
	u.Lessons[li].Activities[ai].Type = ActivityListenPick
	// Dagesh before the vowel is the same letters as the NFC token.
	swapped := swapDagesh(second)
	if swapped == second || normalize(swapped) != second {
		t.Fatalf("swapped %q normalizes to %q, want %q", swapped, normalize(swapped), second)
	}
	u.Lessons[li].Activities[ai].Target = swapped
	if got := Validate(context.Background(), &u, lk); len(got) != 0 {
		t.Fatalf("NFC-equivalent target %s", mustJSON(got))
	}

	u.Lessons[li].Activities[ai].Target = "גָּר"
	got = targetOnly(Validate(context.Background(), &u, lk))
	if len(got) != 1 || got[0].Rule != RuleActivityTarget || got[0].Pointer != at("lessons", li, "activities", ai, "target") {
		t.Fatalf("non-token %#v", got)
	}
	if !strings.Contains(got[0].Msg, act.Type) || !strings.Contains(got[0].Msg, "not one of that activity's tokens") || !strings.HasSuffix(strings.TrimSpace(got[0].Msg), ".") {
		t.Fatalf("non-token message %q", got[0].Msg)
	}

	u.Lessons[li].Activities[ai].Text = act.Text + " גָּר"
	u.Lessons[li].Activities[ai].Target = "גָּר"
	got = targetOnly(Validate(context.Background(), &u, lk))
	if len(got) != 1 || !strings.Contains(got[0].Msg, "no lexicon entry") || !strings.Contains(got[0].Msg, act.Type) {
		t.Fatalf("no lexeme %#v", got)
	}

	// The lexicon has only the unpointed spelling. inLexicon would accept it.
	// The player looks up the exact pointed token, so the target is rejected.
	const gar = "גָּר"
	bare := stripNiqqud(gar)
	if bare == gar || bare == "" {
		t.Fatalf("bare %q", bare)
	}
	u, lk = v1()
	lk.lex[normalize(bare)] = glex{ID: 90, POS: "verb", Bare: bare}
	u.Lessons[li].Activities[ai].Text = u.Lessons[li].Activities[ai].Text + " " + gar
	u.Lessons[li].Activities[ai].Target = gar
	probe := &scan{ctx: context.Background(), lk: lk, cache: map[string]*probe{}}
	if !probe.inLexicon(gar) {
		t.Fatal("fixture: the unpointed row should satisfy inLexicon")
	}
	got = targetOnly(Validate(context.Background(), &u, lk))
	if len(got) != 1 || got[0].Rule != RuleActivityTarget {
		t.Fatalf("unpointed lexicon %#v", got)
	}
	msg := got[0].Msg
	if !strings.Contains(msg, ActivityListenPick) || !strings.Contains(msg, "no lexicon entry") || !strings.HasSuffix(strings.TrimSpace(msg), ".") {
		t.Fatalf("unpointed message %q", msg)
	}

	u, lk = v1()
	story := -1
	for i, a := range u.Lessons[0].Activities {
		if a.Type == ActivityStory {
			story = i
		}
	}
	if story < 0 {
		t.Fatal("no story")
	}
	u.Lessons[0].Activities[story].Target = tokenize(u.Lessons[0].Activities[story].Text)[0]
	got = targetOnly(Validate(context.Background(), &u, lk))
	if len(got) != 1 || !strings.Contains(got[0].Msg, ActivityStory) || !strings.Contains(got[0].Msg, "only a recall") {
		t.Fatalf("story target %#v", got)
	}

	for _, typ := range []string{ActivityProduce, ActivityCloze, ActivityRecall} {
		u, lk = v1()
		u.Lessons[li].Activities[ai].Type = typ
		u.Lessons[li].Activities[ai].Target = second
		if got := targetOnly(Validate(context.Background(), &u, lk)); len(got) != 0 {
			t.Fatalf("%s target %#v", typ, got)
		}
	}
}

// swapDagesh moves a dagesh (U+05BC) to before the preceding vowel so the
// string is not NFC. Normalize puts the vowel first again.
func swapDagesh(s string) string {
	var out []rune
	for _, r := range s {
		if r == 0x05BC && len(out) > 0 {
			out = append(out[:len(out)-1], r, out[len(out)-1])
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func targetOnly(issues []Issue) []Issue {
	var out []Issue
	for _, is := range issues {
		if is.Rule == RuleActivityTarget {
			out = append(out, is)
		}
	}
	return out
}

func TestMissingGradedTarget(t *testing.T) {
	u, lk := v1()
	ctx := context.Background()
	single := u.Lessons[0].Activities[0]
	if single.Type != ActivityRecall || normalize(single.Target) != "" {
		t.Fatalf("fixture activity 0: %+v", single)
	}
	if got := targetOnly(Validate(ctx, &u, lk)); len(got) != 0 {
		t.Fatalf("single-token recall %s", mustJSON(got))
	}

	li, ai := 0, 2
	line := &u.Lessons[li].Activities[ai]
	if line.Type != ActivityListenPick {
		t.Fatalf("fixture activity 2: %+v", line)
	}
	wantPtr := at("lessons", li, "activities", ai, "target")
	wantMsg := "activity 3: this line has several words; set target to the exact pointed word to grade."
	line.Target = ""
	if got := targetOnly(Validate(ctx, &u, lk)); len(got) != 0 {
		t.Fatalf("listen_pick omits target %#v", got)
	}
	var got []Issue
	for _, typ := range []string{ActivityRecall, ActivityProduce, ActivityCloze} {
		line.Type = typ
		got = targetOnly(Validate(ctx, &u, lk))
		if len(got) != 1 || got[0].Severity != SeverityBlock || got[0].Pointer != wantPtr || got[0].Msg != wantMsg {
			t.Fatalf("%s %#v", typ, got)
		}
	}
	line.Type = ActivityRecall

	pub := targetOnly(ValidateIn(ctx, &u, lk, ModePublished))
	if len(pub) != 1 || pub[0].Severity != SeverityWarn || pub[0].Pointer != wantPtr || pub[0].Msg != wantMsg {
		t.Fatalf("published recheck %#v", pub)
	}
	for _, is := range ValidateIn(ctx, &u, lk, ModePublished) {
		if is.Severity == SeverityBlock {
			t.Fatalf("published recheck blocked: %+v", is)
		}
	}

	toks := tokenize(line.Text)
	line.Target = toks[0]
	if got := targetOnly(Validate(ctx, &u, lk)); len(got) != 0 {
		t.Fatalf("exact target %s", mustJSON(got))
	}

	// A numeral beside one lexicon word is still one word.
	u, lk = v1()
	u.Lessons[0].Activities[0].Text += " 12"
	u.Lessons[0].Activities[0].Type = ActivityRecall
	if got := targetOnly(Validate(ctx, &u, lk)); len(got) != 0 {
		t.Fatalf("one word plus a numeral %s", mustJSON(got))
	}

	// A target that is set and wrong stays a block when the revision is already published.
	u, lk = v1()
	u.Lessons[li].Activities[ai].Target = "גָּר"
	got = targetOnly(ValidateIn(ctx, &u, lk, ModePublished))
	if len(got) != 1 || got[0].Severity != SeverityBlock || !strings.Contains(got[0].Msg, "not one of that activity's tokens") {
		t.Fatalf("published wrong target %#v", got)
	}
}

// TestLexemeTokenCount is the shared multi-word count. A line is multi-word
// when two or more tokens are exact lexicon entries, the same lookup the
// player uses. Punctuation, numerals, unknown words, and an unpointed-only
// lexicon row do not count. recall, produce, and cloze require target at
// that count. listen_pick does not.
func TestLexemeTokenCount(t *testing.T) {
	const (
		one = "שָׁלוֹם"
		two = "קָפֶה"
		gar = "גָּר"
	)
	bare := stripNiqqud(gar)
	lk := fakeLookup{lex: map[string]glex{
		normalize(one):  {ID: 1, POS: "noun", Gloss: "hello", Bare: stripNiqqud(one)},
		normalize(two):  {ID: 2, POS: "noun", Gloss: "coffee", Bare: stripNiqqud(two)},
		normalize(bare): {ID: 3, POS: "verb", Gloss: "lives", Bare: bare},
	}}
	ctx := context.Background()
	s := &scan{ctx: ctx, lk: lk, cache: map[string]*probe{}}
	line := one + " " + two
	if n := lexemeTokenCount(s, Activity{Text: line}); n != 2 || s.err != nil {
		t.Fatalf("two lexicon words = %d err %v", n, s.err)
	}
	if n := lexemeTokenCount(s, Activity{Text: one + " " + gar}); n != 1 {
		t.Fatalf("unpointed-only word counted: %d", n)
	}
	if n := lexemeTokenCount(s, Activity{Text: one + ", 12!"}); n != 1 {
		t.Fatalf("punctuation or numeral counted: %d", n)
	}
	unknown := "אַלְמוֹנִי"
	if n := lexemeTokenCount(s, Activity{Text: one + " " + unknown}); n != 1 {
		t.Fatalf("unknown word counted: %d", n)
	}

	doc := func(typ, text, target string) Unit {
		return Unit{
			Title: "Cafe", Situation: "Ordering", Concept: "A greeting",
			NewWords: []string{one, two},
			Lessons: []Lesson{{Kind: KindLesson, Activities: []Activity{{
				Type: typ, Text: text, Target: target,
			}}}},
		}
	}
	for _, typ := range []string{ActivityRecall, ActivityProduce, ActivityCloze} {
		got := targetOnly(Validate(ctx, ptrUnit(doc(typ, line, "")), lk))
		if len(got) != 1 || got[0].Severity != SeverityBlock {
			t.Fatalf("%s two lexicon words %#v", typ, got)
		}
		if got := targetOnly(Validate(ctx, ptrUnit(doc(typ, one+" "+unknown, "")), lk)); len(got) != 0 {
			t.Fatalf("%s one lexicon word %#v", typ, got)
		}
	}
	if got := targetOnly(Validate(ctx, ptrUnit(doc(ActivityListenPick, line, "")), lk)); len(got) != 0 {
		t.Fatalf("listen_pick %#v", got)
	}
}

func ptrUnit(u Unit) *Unit { return &u }
