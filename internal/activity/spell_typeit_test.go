package activity

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func TestFittingSpellPrefersDictation(t *testing.T) {
	withClip := Facts{HasGloss: true, HasAudio: true, GlossedOthers: 40, VoicedOthers: 40}
	if got := Fitting(fsrs.SkillSpell, withClip); !reflect.DeepEqual(got, []Kind{Dictation}) {
		t.Fatalf("with clip = %v, want [dictation]", got)
	}
	noClip := Facts{HasGloss: true, GlossedOthers: 40}
	if got := Fitting(fsrs.SkillSpell, noClip); !reflect.DeepEqual(got, []Kind{TypeIt}) {
		t.Fatalf("without clip = %v, want [type_it]", got)
	}
}

func TestKindsForSpellListsDictationThenTypeIt(t *testing.T) {
	got := kindsFor(fsrs.SkillSpell)
	want := []Kind{Dictation, TypeIt}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kindsFor(spell) = %v, want %v", got, want)
	}
}

// AC1: a spelling type_it (no clip) grades the SPELL card, credits recall on an
// exact answer, and logs a letter pair on a one-letter near miss.
func TestSpellTypeItGradesSpellAndCreditsRecall(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	window := addWord(t, w.store, "חַלּוֹן", "window", "noun", false)

	due := w.clk.Now().Add(-time.Hour)
	reviewed := w.clk.Now().Add(-48 * time.Hour)
	for _, skill := range []string{fsrs.SkillSpell, fsrs.SkillRecall} {
		_, err := w.store.InsertCard(ctx, db.Card{
			UserID: w.user, LexemeID: window, Skill: skill, State: fsrs.StateReview,
			Due: due, Stability: 2, Difficulty: 5, Reps: 2, LastReview: reviewed,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	spell, ok := w.card(t, w.user, window, fsrs.SkillSpell)
	if !ok {
		t.Fatal("missing spell card")
	}
	recall, ok := w.card(t, w.user, window, fsrs.SkillRecall)
	if !ok {
		t.Fatal("missing recall card")
	}

	// Exact → spelling Good and an implied recall Good.
	b := w.build(t, window, TypeIt, WithSkill(fsrs.SkillSpell))
	if b.item.Skill != fsrs.SkillSpell || !b.item.Typed {
		t.Fatalf("item = %+v, want typed spell", b.item)
	}
	res := w.check(t, b, Response{Text: "חלון"})
	if !res.Correct || res.NearMiss || res.Rating != fsrs.Good {
		t.Fatalf("exact = %+v, want Good", res)
	}
	spellAttempts, err := w.store.ListAttemptsBySkill(ctx, w.user, window, fsrs.SkillSpell)
	mustf(t, err, "spell attempts")
	if len(spellAttempts) != 1 || spellAttempts[0].Activity != string(TypeIt) || !spellAttempts[0].Correct {
		t.Fatalf("spell attempts = %+v", spellAttempts)
	}
	if rs := w.ratings(t, spell.ID); len(rs) != 1 || rs[0] != int(fsrs.Good) {
		t.Fatalf("spell ratings = %v, want [Good]", rs)
	}
	recallLogs, err := w.store.ReviewLogs(ctx, recall.ID)
	mustf(t, err, "recall logs")
	if len(recallLogs) != 1 || !recallLogs[0].Implied || recallLogs[0].Rating != int(fsrs.Good) {
		t.Fatalf("recall logs = %+v, want one implied Good", recallLogs)
	}

	// One-letter confusable miss (ח→כ): spelling Hard, letter pair logged, no new recall credit.
	b = w.build(t, window, TypeIt, WithSkill(fsrs.SkillSpell))
	res = w.check(t, b, Response{Text: "כלון"})
	if !res.Correct || !res.NearMiss || res.Rating != fsrs.Hard {
		t.Fatalf("near miss = %+v, want Hard", res)
	}
	if cons := w.confusions(t); len(cons) != 1 || cons[0] != "letter_pair:ח/כ" {
		t.Fatalf("confusions = %v, want letter_pair:ח/כ", cons)
	}
	if rs := w.ratings(t, spell.ID); len(rs) != 2 || rs[1] != int(fsrs.Hard) {
		t.Fatalf("spell ratings after near miss = %v", rs)
	}
	if n := len(mustReviewLogs(t, w, recall.ID)); n != 1 {
		t.Fatalf("recall logs after near miss = %d, want still 1", n)
	}

	// Wrong / don't-know → Again on the spell card only.
	b = w.build(t, window, TypeIt, WithSkill(fsrs.SkillSpell))
	res = w.check(t, b, Response{DontKnow: true})
	if res.Correct || res.Rating != fsrs.Again {
		t.Fatalf("dont_know = %+v, want Again", res)
	}
	if rs := w.ratings(t, spell.ID); len(rs) != 3 || rs[2] != int(fsrs.Again) {
		t.Fatalf("spell ratings after dont_know = %v", rs)
	}
}

func mustReviewLogs(t *testing.T, w *world, cardID int64) []db.ReviewLog {
	t.Helper()
	logs, err := w.store.ReviewLogs(context.Background(), cardID)
	mustf(t, err, "review logs")
	return logs
}
