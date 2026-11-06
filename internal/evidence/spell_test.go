package evidence

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// typeIt is a correct type_it on a recall card. LettersPassed is set by each case.
func (e *env) typeIt(lexeme int64) Attempt {
	return Attempt{
		UserID:   e.user,
		LexemeID: lexeme,
		Skill:    fsrs.SkillRecall,
		Activity: activityTypeIt,
		Source:   SourceReview,
		Prompt:   "dog",
		Response: kelev,
		Correct:  true,
		Latency:  3 * time.Second,
	}
}

// AC1: correct type_it, gate open, no spell card → create card + implied Good + spell attempt.
func TestTypedCorrectCreatesSpellCardAndImpliedGood(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lex := e.lexeme(kelev)
	_, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRecall, State: fsrs.StateReview,
		Due: e.clk.Now().Add(-time.Hour), Stability: 2, Difficulty: 5, Reps: 2,
		LastReview: e.clk.Now().Add(-48 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	a := e.typeIt(lex)
	a.LettersPassed = true
	if _, err := e.rec.Attempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT COUNT(*) FROM attempts WHERE skill = 'spell'`); n != 1 {
		t.Fatalf("spell attempts = %d, want 1", n)
	}
	spell, ok := e.card(lex, fsrs.SkillSpell)
	if !ok {
		t.Fatal("expected a spelling card")
	}
	logs, err := e.store.ReviewLogs(ctx, spell.ID)
	if err != nil || len(logs) != 1 || !logs[0].Implied || logs[0].Rating != int(fsrs.Good) {
		t.Fatalf("spell logs = %+v, %v; want one implied Good", logs, err)
	}
	var activity, source, prompt, response string
	var latency int64
	if err := e.conn().QueryRow(`
		SELECT activity, source, prompt, response, latency_ms FROM attempts WHERE skill = 'spell'`).
		Scan(&activity, &source, &prompt, &response, &latency); err != nil {
		t.Fatal(err)
	}
	if activity != activityTypeIt || source != SourceReview || prompt != "dog" || response != kelev || latency != 3000 {
		t.Fatalf("spell attempt fields = %s %s %q %q %d", activity, source, prompt, response, latency)
	}
}

// AC2: a second correct answer schedules the existing spelling card; no second card.
func TestTypedCorrectSecondSchedulesExistingSpellCard(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lex := e.lexeme(kelev)
	_, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRecall, State: fsrs.StateReview,
		Due: e.clk.Now().Add(-time.Hour), Stability: 2, Difficulty: 5, Reps: 2,
		LastReview: e.clk.Now().Add(-48 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	a := e.typeIt(lex)
	a.LettersPassed = true
	if _, err := e.rec.Attempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	first, ok := e.card(lex, fsrs.SkillSpell)
	if !ok {
		t.Fatal("expected a spelling card after the first answer")
	}
	e.clk.Advance(time.Minute)
	a.Response = kelev
	if _, err := e.rec.Attempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = 'spell'`, e.user, lex); n != 1 {
		t.Fatalf("spell cards = %d, want 1", n)
	}
	second, ok := e.card(lex, fsrs.SkillSpell)
	if !ok || second.ID != first.ID {
		t.Fatalf("spell card id = %d, want existing %d", second.ID, first.ID)
	}
	logs, err := e.store.ReviewLogs(ctx, first.ID)
	if err != nil || len(logs) != 2 {
		t.Fatalf("spell logs = %+v, %v; want 2 implied reviews", logs, err)
	}
	for i, log := range logs {
		if !log.Implied || log.Rating != int(fsrs.Good) {
			t.Fatalf("log[%d] = %+v, want implied Good", i, log)
		}
	}
	if !second.Due.After(first.Due) && !second.Due.Equal(e.clk.Now()) {
		// Due must have moved from the second Good.
		if second.Due.Equal(first.Due) {
			t.Fatalf("second Good did not reschedule: due still %v", second.Due)
		}
	}
}

// AC3: near miss → spelling Hard; wrong and don't-know write nothing for spelling.
func TestTypedNearMissSpellHardWrongAndDontKnowWriteNothing(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lex := e.lexeme(kelev)
	_, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRecall, State: fsrs.StateReview,
		Due: e.clk.Now().Add(-time.Hour), Stability: 2, Difficulty: 5, Reps: 2,
		LastReview: e.clk.Now().Add(-48 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	near := e.typeIt(lex)
	near.LettersPassed = true
	near.NearMiss = true
	near.Response = "כֶּלֶפ" // one letter off; letter pairs stay on the recall path
	near.Confusions = []Confusion{{Kind: fsrs.MissLetterPair, Key: "ב/פ"}}
	if _, err := e.rec.Attempt(ctx, near); err != nil {
		t.Fatal(err)
	}
	spell, ok := e.card(lex, fsrs.SkillSpell)
	if !ok {
		t.Fatal("near miss should create a spelling card when the gate is open")
	}
	logs, err := e.store.ReviewLogs(ctx, spell.ID)
	if err != nil || len(logs) != 1 || !logs[0].Implied || logs[0].Rating != int(fsrs.Hard) {
		t.Fatalf("spell logs = %+v, %v; want one implied Hard", logs, err)
	}
	if n := e.count(`SELECT COUNT(*) FROM confusion_misses WHERE kind = 'letter_pair'`); n != 1 {
		t.Fatalf("letter pairs = %d, want the recall near-miss still logged", n)
	}

	lex2 := e.lexeme(shalom)
	_, err = e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex2, Skill: fsrs.SkillRecall, State: fsrs.StateReview,
		Due: e.clk.Now().Add(-time.Hour), Stability: 2, Difficulty: 5, Reps: 2,
		LastReview: e.clk.Now().Add(-48 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	wrong := e.typeIt(lex2)
	wrong.LettersPassed = true
	wrong.Correct = false
	wrong.Response = "לא"
	if _, err := e.rec.Attempt(ctx, wrong); err != nil {
		t.Fatal(err)
	}
	dont := e.typeIt(lex2)
	dont.LettersPassed = true
	dont.Correct = false
	dont.DontKnow = true
	dont.Response = ""
	e.clk.Advance(time.Second)
	if _, err := e.rec.Attempt(ctx, dont); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT COUNT(*) FROM attempts WHERE lexeme_id = ? AND skill = 'spell'`, lex2); n != 0 {
		t.Fatalf("wrong/dont-know spell attempts = %d, want 0", n)
	}
	if _, ok := e.card(lex2, fsrs.SkillSpell); ok {
		t.Fatal("wrong/dont-know must not create a spelling card")
	}
}

// AC4: gate closed → spell attempt only, no card.
func TestTypedGateClosedSpellAttemptOnly(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lex := e.lexeme(kelev)
	_, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRecall, State: fsrs.StateReview,
		Due: e.clk.Now().Add(-time.Hour), Stability: 2, Difficulty: 5, Reps: 2,
		LastReview: e.clk.Now().Add(-48 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	a := e.typeIt(lex)
	a.LettersPassed = false
	if _, err := e.rec.Attempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT COUNT(*) FROM attempts WHERE skill = 'spell'`); n != 1 {
		t.Fatalf("spell attempts = %d, want 1", n)
	}
	if _, ok := e.card(lex, fsrs.SkillSpell); ok {
		t.Fatal("gate closed must not create a spelling card")
	}
	if n := e.count(`SELECT COUNT(*) FROM review_log WHERE implied = 1`); n != 0 {
		t.Fatalf("implied reviews = %d, want 0 when gate is closed and no card", n)
	}
}

// AC5: implied spelling credit is excluded from RecentReviewRatings and coach accuracy.
func TestTypedSpellCreditExcludedFromAccuracy(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lex := e.lexeme(kelev)
	_, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRecall, State: fsrs.StateReview,
		Due: e.clk.Now().Add(-time.Hour), Stability: 2, Difficulty: 5, Reps: 2,
		LastReview: e.clk.Now().Add(-48 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	beforeRatings, err := e.store.RecentReviewRatings(ctx, e.user, e.clk.Now(), 20)
	if err != nil {
		t.Fatal(err)
	}
	beforeAcc, err := e.store.CoachAccuracy(ctx, e.user, e.clk.Now(), 7*24*time.Hour, 20)
	if err != nil {
		t.Fatal(err)
	}
	a := e.typeIt(lex)
	a.LettersPassed = true
	rating, err := e.rec.Attempt(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	afterRatings, err := e.store.RecentReviewRatings(ctx, e.user, e.clk.Now(), 20)
	if err != nil {
		t.Fatal(err)
	}
	afterAcc, err := e.store.CoachAccuracy(ctx, e.user, e.clk.Now(), 7*24*time.Hour, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterRatings) != len(beforeRatings)+1 || afterRatings[0] != int(rating) {
		t.Fatalf("RecentReviewRatings before=%v after=%v rating=%d; implied spell must not appear",
			beforeRatings, afterRatings, rating)
	}
	if afterAcc.Reviews7d.Count != beforeAcc.Reviews7d.Count+1 {
		t.Fatalf("coach Reviews7d count before=%d after=%d; implied must not inflate",
			beforeAcc.Reviews7d.Count, afterAcc.Reviews7d.Count)
	}
	if n := e.count(`SELECT COUNT(*) FROM review_log WHERE implied = 1`); n != 1 {
		t.Fatalf("implied spell rows = %d, want 1 (present but excluded from accuracy)", n)
	}
}

// AC6: another learner's card id cannot receive spelling credit.
func TestSpellImpliedRefusesOtherLearner(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lex := e.lexeme(kelev)
	id, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex, Skill: fsrs.SkillSpell, State: fsrs.StateReview,
		Due: e.clk.Now().Add(-time.Hour), Stability: 2, Difficulty: 5, Reps: 2,
		LastReview: e.clk.Now().Add(-48 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := e.store.InsertUser(ctx, "other@example.com", "sub-other", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.rec.spellImplied(ctx, other, id, fsrs.Good); err == nil {
		t.Fatal("spellImplied accepted another learner's card")
	}
	if n := e.count(`SELECT COUNT(*) FROM review_log`); n != 0 {
		t.Fatal("refused spellImplied still wrote")
	}
	for _, rating := range []fsrs.Rating{fsrs.Again, fsrs.Easy} {
		if err := e.rec.spellImplied(ctx, e.user, id, rating); err == nil {
			t.Fatalf("rating %d was accepted", rating)
		}
	}
	// Owner with Good still works (and Hard is allowed for spelling).
	if err := e.rec.spellImplied(ctx, e.user, id, fsrs.Hard); err != nil {
		t.Fatal(err)
	}
	logs, err := e.store.ReviewLogs(ctx, id)
	if err != nil || len(logs) != 1 || !logs[0].Implied || logs[0].Rating != int(fsrs.Hard) {
		t.Fatalf("owner Hard = %+v, %v", logs, err)
	}
}
