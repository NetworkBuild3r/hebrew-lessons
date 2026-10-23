package learner

import (
	"context"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func TestCanSay(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	phrase := e.lexeme()
	e.exec(`UPDATE lexemes SET text_nfc = ?, gloss_en = ?, translit = ? WHERE id = ?`, "בֹּקֶר טוֹב", "good morning", "boker tov", phrase)
	e.known(e.user, phrase, fsrs.SkillRecall, 5)

	recent := e.lexeme()
	e.known(e.user, recent, fsrs.SkillRecall, 5)
	solid := e.lexeme()
	e.known(e.user, solid, fsrs.SkillRecall, 30)

	old := e.lexeme()
	e.attempts(e.user, old, fsrs.SkillRecall, "RRR")
	e.card(e.user, old, fsrs.SkillRecall, 30, 6, 3, 3, 3)

	hi := e.lexeme()
	e.attempts(e.user, hi, fsrs.SkillRecall, "RR")
	e.card(e.user, hi, fsrs.SkillRecall, 1, 1, 3)
	lo := e.lexeme()
	e.attempts(e.user, lo, fsrs.SkillRecall, "RW")
	e.card(e.user, lo, fsrs.SkillRecall, 1, 1, 3)

	struggle := e.lexeme()
	e.attempts(e.user, struggle, fsrs.SkillRecall, "WWW")
	e.card(e.user, struggle, fsrs.SkillRecall, 5, 1, 1, 1, 1)

	other := e.otherUser()
	theirs := e.lexeme()
	e.known(other, theirs, fsrs.SkillRecall, 30)

	got, err := e.model.CanSay(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{phrase, recent, solid, old}
	if got.Total != len(want) || len(got.Items) != len(want) {
		t.Fatalf("items %d, want %d (%v)", got.Total, len(want), idsOf(got.Items))
	}
	for i, id := range want {
		if got.Items[i].LexemeID != id {
			t.Fatalf("item %d = %d, want %d (%v)", i, got.Items[i].LexemeID, id, idsOf(got.Items))
		}
	}
	if got.Items[0].Translit != "boker tov" || got.Items[0].Gloss != "good morning" {
		t.Fatalf("phrase fields %+v", got.Items[0])
	}
	if got.AddedThisWeek != 3 {
		t.Fatalf("added this week = %d, want 3", got.AddedThisWeek)
	}
	if len(got.Next) != 2 || got.Next[0].LexemeID != hi || got.Next[1].LexemeID != lo {
		t.Fatalf("next %v, want %d then %d", idsOf(got.Next), hi, lo)
	}
	for _, it := range append(got.Items, got.Next...) {
		if it.LexemeID == struggle || it.LexemeID == theirs {
			t.Fatalf("a word that is not sayable appeared: %d", it.LexemeID)
		}
	}

	theirs2, err := e.model.CanSay(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	if len(theirs2.Items) != 1 || theirs2.Items[0].LexemeID != theirs {
		t.Fatalf("other learner sees %v, want only %d", idsOf(theirs2.Items), theirs)
	}
}

func idsOf(items []CanSayItem) []int64 {
	out := make([]int64, len(items))
	for i, it := range items {
		out[i] = it.LexemeID
	}
	return out
}

// A word learned exactly seven days ago still counts; one learned a day earlier does not.
func TestCanSayWeekBoundary(t *testing.T) {
	e := newEnv(t)
	inside := e.lexeme()
	e.attempts(e.user, inside, fsrs.SkillRecall, "RRR")
	e.card(e.user, inside, fsrs.SkillRecall, 30, 5, 3, 3, 3)
	outside := e.lexeme()
	e.attempts(e.user, outside, fsrs.SkillRecall, "RRR")
	e.card(e.user, outside, fsrs.SkillRecall, 30, 6, 3, 3, 3)

	got, err := e.model.CanSay(context.Background(), e.user)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 2 || got.AddedThisWeek != 1 {
		t.Fatalf("total %d added %d, want 2 and 1", got.Total, got.AddedThisWeek)
	}
}

func TestCanSayNeedsALearner(t *testing.T) {
	e := newEnv(t)
	if _, err := e.model.CanSay(context.Background(), 0); err == nil {
		t.Fatal("no learner was accepted")
	}
}
