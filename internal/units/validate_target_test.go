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
	if got := Validate(context.Background(), &u, lk); len(got) != 0 {
		t.Fatalf("blank target %s", mustJSON(got))
	}
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
	got := targetOnly(Validate(context.Background(), &u, lk))
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
