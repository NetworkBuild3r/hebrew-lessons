package learner

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// INIT-036/SPEC-003: known needs checked successes on two study days.
func TestKnownNeedsALaterStudyDay(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	lex := e.lexeme()

	// Day one is yesterday afternoon; day two is today — distinct study days.
	day1 := testNow.AddDate(0, 0, -1)
	cardID, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex, Skill: fsrs.SkillRead,
		State: fsrs.StateReview, Due: testNow, Stability: 10, Difficulty: 5,
		Reps: 3, LastReview: day1,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Three checked Goods on the same study day.
	for i := range 3 {
		at := day1.Add(time.Duration(i) * time.Minute)
		e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 0, 1)`,
			cardID, db.FormatTime(at))
		e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
			e.user, lex, fsrs.SkillRead, db.FormatTime(at))
	}

	sum, err := e.model.Summary(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	w := e.word(lex)
	st := skillOf(w, fsrs.SkillRead).State
	if st != Learning {
		t.Fatalf("day-one skill = %s, want learning (word=%+v)", st, skillOf(w, fsrs.SkillRead))
	}
	if sum.KnownAnySkill != 0 {
		t.Fatalf("KnownAnySkill = %d, want 0 on day one", sum.KnownAnySkill)
	}
	if sum.WaitingForSecondDay != 1 {
		t.Fatalf("WaitingForSecondDay = %d, want 1", sum.WaitingForSecondDay)
	}

	// One more checked Good on a later study day → Known.
	day2 := testNow.Add(-30 * time.Minute)
	e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 1, 1)`,
		cardID, db.FormatTime(day2))
	e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
		e.user, lex, fsrs.SkillRead, db.FormatTime(day2))
	e.exec(`UPDATE cards SET reps = 4, last_review = ?, stability = 10 WHERE id = ?`,
		db.FormatTime(day2), cardID)

	sum, err = e.model.Summary(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	w = e.word(lex)
	st = skillOf(w, fsrs.SkillRead).State
	if st != Known {
		t.Fatalf("after day two skill = %s, want known", st)
	}
	if sum.KnownAnySkill != 1 {
		t.Fatalf("KnownAnySkill = %d, want 1 after day two", sum.KnownAnySkill)
	}
	if sum.WaitingForSecondDay != 0 {
		t.Fatalf("WaitingForSecondDay = %d, want 0 after day two", sum.WaitingForSecondDay)
	}

	// Stability at the solid bar → Solid.
	e.exec(`UPDATE cards SET stability = 21 WHERE id = ?`, cardID)
	w = e.word(lex)
	if st = skillOf(w, fsrs.SkillRead).State; st != Solid {
		t.Fatalf("stable day-two skill = %s, want solid", st)
	}
}

func TestStrugglingAndRegressedStillWinOverDayRule(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	struggle := e.lexeme()
	e.attempts(e.user, struggle, fsrs.SkillRead, "RRRRWWW")
	e.card(e.user, struggle, fsrs.SkillRead, 5, 1, 3, 3, 3)

	regress := e.lexeme()
	e.attempts(e.user, regress, fsrs.SkillRead, "RRRRWW")
	// Low stability, last review 60 days ago → recall chance under the regressed bar.
	e.card(e.user, regress, fsrs.SkillRead, 5, 60, 3, 3, 3)

	if st := skillOf(e.word(struggle), fsrs.SkillRead).State; st != Struggling {
		t.Fatalf("struggle = %s, want struggling", st)
	}
	if st := skillOf(e.word(regress), fsrs.SkillRead).State; st != Regressed {
		t.Fatalf("regress = %s, want regressed", st)
	}
	sum, err := e.model.Summary(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if sum.WaitingForSecondDay != 0 {
		t.Fatalf("WaitingForSecondDay = %d, want 0 (struggling/regressed are not waiting)", sum.WaitingForSecondDay)
	}
}
