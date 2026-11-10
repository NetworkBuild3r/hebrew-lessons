package evidence

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// A learning step whose next due is later the same study day is stored at the
// next cutoff, and a card already due on that short step is left out of the
// due count until the cutoff. INIT-038/SPEC-006.
func TestSameDayStepIsDueAtTheNextCutoff(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.store.SetUserTimeZone(ctx, e.user, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	loc, err := studyday.ParseZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	zone := studyday.Zone{Name: "America/Denver", Loc: loc, Source: studyday.SourceLearner}
	// 16:00 Denver is inside the study day that ends at 04:00 tomorrow.
	e.clk.t = time.Date(2026, 10, 21, 16, 0, 0, 0, loc)
	lex := e.lexeme(kelev)
	a := e.attempt(lex)
	a.Skill = fsrs.SkillRead
	a.Activity = "read_pick"
	if _, err := e.rec.Attempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	card, ok := e.card(lex, fsrs.SkillRead)
	if !ok {
		t.Fatal("no card")
	}
	cutoff := studyday.DueCutoff(e.clk.t, zone)
	if !card.Due.Equal(cutoff) {
		t.Fatalf("due = %s, want cutoff %s", card.Due, cutoff)
	}
	due, err := e.store.ListDueCards(ctx, e.user, e.clk.t)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Fatalf("same-day step is due now: %+v", due)
	}
	due, err = e.store.ListDueCards(ctx, e.user, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].ID != card.ID {
		t.Fatalf("due at cutoff = %+v, want the card", due)
	}
}

// Cards already scheduled a few minutes after their last review count as due
// at the next cutoff, so they drop out of today's due count. A clipless listen
// card stays out as well (SPEC-003).
func TestDueCountDropsSameDayStepsAndCliplessListen(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	if err := e.store.SetUserTimeZone(ctx, e.user, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	loc, err := studyday.ParseZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	zone := studyday.Zone{Name: "America/Denver", Loc: loc, Source: studyday.SourceLearner}
	now := time.Date(2026, 10, 21, 16, 0, 0, 0, loc)
	e.clk.t = now
	askable := e.lexeme(kelev)
	sameDayWord := e.lexeme(shalom)
	bare := e.lexeme("דָּג")
	reviewed := now.Add(-48 * time.Hour)
	insert := func(lex int64, skill string, due, last time.Time) {
		t.Helper()
		if _, err := e.store.InsertCard(ctx, db.Card{
			UserID: e.user, LexemeID: lex, Skill: skill, State: fsrs.StateReview,
			Due: due, Stability: 4, Difficulty: 5, Reps: 2, LastReview: last,
		}); err != nil {
			t.Fatal(err)
		}
	}
	insert(askable, fsrs.SkillRead, now.Add(-time.Hour), reviewed)
	insert(sameDayWord, fsrs.SkillRecall, now.Add(-10*time.Minute), now.Add(-20*time.Minute))
	insert(bare, fsrs.SkillListen, now.Add(-time.Hour), reviewed)

	raw := e.count(`SELECT COUNT(*) FROM cards WHERE user_id = ? AND suspended = 0 AND due <= ?`, e.user, db.FormatTime(now))
	due, err := e.store.ListDueCards(ctx, e.user, now)
	if err != nil {
		t.Fatal(err)
	}
	if raw != 3 {
		t.Fatalf("raw due = %d, want 3", raw)
	}
	if len(due) != 1 || due[0].LexemeID != askable {
		t.Fatalf("askable = %+v, want only the read card", due)
	}
	if studyday.EffectiveDue(now.Add(-10*time.Minute), now.Add(-20*time.Minute), zone).After(now) == false {
		t.Fatal("fixture same-day step should still be waiting on the cutoff")
	}
}
