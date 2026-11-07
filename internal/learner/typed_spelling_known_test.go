package learner

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// Planted learner tests for INIT-035/SPEC-003 live here (not internal/app):
// learner already imports evidence, has Store+Model+fakeClock helpers, and
// evidence must not import learner (cycle). The real Recorder write path is used.

func typedSpellRecorder(t *testing.T, e *env) *evidence.Recorder {
	t.Helper()
	rev := fsrs.New(e.store, e.clk)
	return evidence.New(e.store, rev, e.clk)
}

func lexemeText(t *testing.T, e *env, lex int64) string {
	t.Helper()
	ref, ok, err := e.store.LexemeByID(context.Background(), lex)
	if err != nil || !ok {
		t.Fatalf("LexemeByID: ok=%v err=%v", ok, err)
	}
	return ref.TextNFC
}

// typeItRecallCorrect records one correct English→Hebrew type_it on a recall card
// with the letters gate open (real Recorder path → spellImplied).
func typeItRecallCorrect(t *testing.T, e *env, rec *evidence.Recorder, lex int64, hebrew string) {
	t.Helper()
	ctx := context.Background()
	_, err := rec.Attempt(ctx, evidence.Attempt{
		UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRecall,
		Activity: "type_it", Source: evidence.SourceReview,
		Correct: true, Latency: 3 * time.Second, LettersPassed: true,
		Prompt: "word", Response: hebrew,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func advancePastSpellDue(t *testing.T, e *env, lex int64) {
	t.Helper()
	id, ok, err := e.store.CardID(context.Background(), e.user, lex, fsrs.SkillSpell)
	if err != nil || !ok {
		t.Fatalf("spell card: ok=%v err=%v", ok, err)
	}
	card, err := e.store.GetCard(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	next := card.Due.Add(time.Hour)
	// Distinct civil days: jump at least one full day past now as well.
	dayLater := e.clk.Now().Add(26 * time.Hour)
	if dayLater.After(next) {
		next = dayLater
	}
	e.clk.t = next
}

// TestTypedSpellingThreeDaysMakesSpellKnown: three correct type_it answers on
// different days, clock past the spelling card's due each time → Known/Solid.
// Accuracy14 and DueRetention match the exclude-implied golden values.
func TestTypedSpellingThreeDaysMakesSpellKnown(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.clk.t = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	lex := e.lexeme()
	hebrew := lexemeText(t, e, lex)

	_, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRecall,
		State: fsrs.StateNew, Due: e.clk.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := typedSpellRecorder(t, e)

	typeItRecallCorrect(t, e, rec, lex, hebrew)
	advancePastSpellDue(t, e, lex)
	typeItRecallCorrect(t, e, rec, lex, hebrew)
	advancePastSpellDue(t, e, lex)
	typeItRecallCorrect(t, e, rec, lex, hebrew)

	snap, err := e.model.Snapshot(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	w, ok := snap.Words[lex]
	if !ok {
		t.Fatal("word missing from snapshot")
	}
	st := skillOf(w, fsrs.SkillSpell).State
	if st != Known && st != Solid {
		t.Fatalf("spell state = %s, want known or solid (skill=%+v)", st, skillOf(w, fsrs.SkillSpell))
	}

	// Golden: 14-day direction accuracy uses AttemptTally (attempts), so the
	// three spell attempts from creditTypedSpelling count; implied review_log
	// rows are not involved. Spell accuracy = 3/3 = 1.
	standing, err := e.model.Standing(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	var spellAcc *float64
	for _, d := range standing.Directions {
		if d.Skill == fsrs.SkillSpell {
			spellAcc = d.Accuracy14
		}
	}
	if spellAcc == nil || math.Abs(*spellAcc-1.0) > 1e-9 {
		t.Fatalf("spell Accuracy14 = %v, want 1.0 (implied rows excluded from accuracy)", spellAcc)
	}

	// Golden: DueRetention already excludes implied; the three spelling reviews
	// are all implied, so spell contributes nothing. Recall type_it writes
	// non-implied reviews when elapsed_days >= 1.
	retention, err := e.store.DueRetention(ctx, e.user, e.clk.Now().AddDate(0, 0, -RetentionWindowDays), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range retention {
		if row.Skill == fsrs.SkillSpell {
			t.Fatalf("DueRetention includes spell %+v; implied spelling reviews must be excluded", row)
		}
	}
	// Summary.Retention must equal the exclude-implied ratio from DueRetention.
	var due, right int
	for _, row := range retention {
		due += row.Due
		right += row.Right
	}
	var wantRet *float64
	if due > 0 {
		v := float64(right) / float64(due)
		wantRet = &v
	}
	sum, err := e.model.Summary(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if (sum.Retention == nil) != (wantRet == nil) {
		t.Fatalf("Summary.Retention=%v DueRetention golden=%v", sum.Retention, wantRet)
	}
	if sum.Retention != nil && math.Abs(*sum.Retention-*wantRet) > 1e-9 {
		t.Fatalf("Summary.Retention=%v want %v (exclude-implied DueRetention)", *sum.Retention, *wantRet)
	}
}

// TestTypedSpellingNewestWrongNotKnown: same three-day setup, then a wrong
// direct spelling answer → newest outcome wrong → not Known.
func TestTypedSpellingNewestWrongNotKnown(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.clk.t = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	lex := e.lexeme()
	hebrew := lexemeText(t, e, lex)

	_, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRecall,
		State: fsrs.StateNew, Due: e.clk.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := typedSpellRecorder(t, e)

	typeItRecallCorrect(t, e, rec, lex, hebrew)
	advancePastSpellDue(t, e, lex)
	typeItRecallCorrect(t, e, rec, lex, hebrew)
	advancePastSpellDue(t, e, lex)
	typeItRecallCorrect(t, e, rec, lex, hebrew)

	advancePastSpellDue(t, e, lex)
	_, err = rec.Attempt(ctx, evidence.Attempt{
		UserID: e.user, LexemeID: lex, Skill: fsrs.SkillSpell,
		Activity: "type_it", Source: evidence.SourceReview,
		Correct: false, Latency: 3 * time.Second, LettersPassed: true,
		Prompt: "word", Response: "לא",
	})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := e.model.Snapshot(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	st := skillOf(snap.Words[lex], fsrs.SkillSpell).State
	if st == Known || st == Solid {
		t.Fatalf("spell state = %s after newest wrong; want not known", st)
	}
}
