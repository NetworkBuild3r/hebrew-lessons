package brief

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

// INIT-036/SPEC-003: the brief tier reads the learner model, so a day-one
// "old known" word is dropped until a second study day lands.
func TestBriefTierDropsDayOneWordUntilSecondDay(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := e.word("day-one", 0)

	day1 := testNow.AddDate(0, 0, -1)
	res := e.exec(`INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended)
		VALUES (?, ?, ?, ?, ?, 10, 5, 3, 0, ?, 0)`,
		e.user, id, fsrs.SkillListen, fsrs.StateReview, db.FormatTime(testNow), db.FormatTime(day1))
	cardID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		at := day1.Add(time.Duration(i) * time.Minute)
		e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 0, 1)`,
			cardID, db.FormatTime(at))
		e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
			e.user, id, fsrs.SkillListen, db.FormatTime(at))
	}

	br := e.build("")
	if br.Tier.KnownWords != 0 || br.Tier.Name != TierBeginner {
		t.Fatalf("day-one tier = %+v, want beginner with 0 known words", br.Tier)
	}
	model := learner.New(e.store, e.clk, learner.WithBands(e.bands))
	sum, err := model.Summary(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Words[learner.Known]+sum.Words[learner.Solid] != 0 {
		t.Fatalf("day-one model known+solid = %d, want 0", sum.Words[learner.Known]+sum.Words[learner.Solid])
	}
	if sum.WaitingForSecondDay != 1 {
		t.Fatalf("WaitingForSecondDay = %d, want 1", sum.WaitingForSecondDay)
	}

	day2 := testNow.Add(-30 * time.Minute)
	e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 1, 1)`,
		cardID, db.FormatTime(day2))
	e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
		e.user, id, fsrs.SkillListen, db.FormatTime(day2))
	e.exec(`UPDATE cards SET reps = 4, last_review = ? WHERE id = ?`, db.FormatTime(day2), cardID)

	br = e.build("")
	if br.Tier.KnownWords != 1 {
		t.Fatalf("day-two tier = %+v, want known_words 1", br.Tier)
	}
	sum, err = model.Summary(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if sum.WaitingForSecondDay != 0 {
		t.Fatalf("WaitingForSecondDay = %d, want 0 after day two", sum.WaitingForSecondDay)
	}
}
