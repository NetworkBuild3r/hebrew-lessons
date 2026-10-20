package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCoachCardsOrderPageAndAccuracy(t *testing.T) {
	s := openStore(t, nil)
	user := insertUser(t, s, "coach@example.com", "sub-coach", "learner")
	other := insertUser(t, s, "other@example.com", "sub-other", "learner")
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	dueEarly := FormatTime(now)
	dueLate := FormatTime(now.Add(time.Hour))
	lastNew := FormatTime(now.Add(-time.Hour))
	lastOld := FormatTime(now.Add(-2 * time.Hour))

	pointed := insertLexeme(t, s, "שָׁלוֹם")
	mustExec(t, s, `UPDATE lexemes SET translit = 'shalom', pos = 'interjection', gloss_en = 'hello' WHERE id = ?`, pointed)
	bare := insertLexeme(t, s, "קָפֶה")
	mustExec(t, s, `UPDATE lexemes SET pos = 'noun', gloss_en = 'coffee' WHERE id = ?`, bare)
	extra := insertLexeme(t, s, "סֵפֶר")
	otherLex := insertLexeme(t, s, "דָּג")

	lateNew := insertCoachCard(t, s, user, pointed, "read", "review", dueLate, lastNew, 0)
	lateOld := insertCoachCard(t, s, user, bare, "recall", "learning", dueLate, lastOld, 0)
	lateNone := insertCoachCard(t, s, user, extra, "read", "new", dueLate, "", 1)
	early := insertCoachCard(t, s, user, pointed, "recall", "relearning", dueEarly, "", 0)
	insertCoachCard(t, s, other, otherLex, "read", "review", dueEarly, "", 0)

	mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 4, ?, 0, 0)`, lateNew, FormatTime(now.Add(-8*24*time.Hour)))
	mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 1, ?, 0, 0)`, lateNew, FormatTime(now.Add(-48*time.Hour)))
	mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 0, 0)`, lateNew, FormatTime(now.Add(-24*time.Hour)))
	mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 4, ?, 0, 0)`, early, FormatTime(now.Add(time.Hour)))
	otherCard := insertCoachCard(t, s, other, otherLex, "recall", "review", dueEarly, "", 0)
	mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 0, 0)`, otherCard, FormatTime(now.Add(-time.Hour)))

	unit := insertUnit(t, s, "published")
	insertLesson(t, s, user, unit, 0, FormatTime(now.Add(-time.Hour)), 0.5)
	insertLesson(t, s, user, unit, 1, FormatTime(now.Add(-2*time.Hour)), nil)
	insertLesson(t, s, user, unit, 2, FormatTime(now.Add(-3*time.Hour)), 1.0)
	insertLesson(t, s, user, unit, 3, FormatTime(now.Add(-8*24*time.Hour)), 1.0)
	mustExec(t, s, `INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at) VALUES (?, ?, 4, ?)`, user, unit, FormatTime(now.Add(-time.Hour)))
	insertLesson(t, s, other, unit, 0, FormatTime(now.Add(-time.Hour)), 1.0)

	ctx := context.Background()
	page, err := s.CoachCards(ctx, CoachQuery{UserID: user, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page.NextCursor == "" || !sameIDs(idsOf(page), []int64{early, lateNew}) {
		t.Fatalf("page1 %+v", idsOf(page))
	}
	if !page.Cards[1].HasTranslit || page.Cards[1].Translit != "shalom" || !page.Cards[1].HasRating || page.Cards[1].LastRating != 3 || page.Cards[1].LastText == "" {
		t.Fatalf("rated %+v", page.Cards[1])
	}
	if !page.Cards[0].HasTranslit || page.Cards[0].Translit != "shalom" || !page.Cards[0].HasRating || page.Cards[0].LastRating != 4 || page.Cards[0].LastText != "" || page.Cards[0].Gloss != "hello" {
		t.Fatalf("early %+v", page.Cards[0])
	}
	rest, err := s.CoachCards(ctx, CoachQuery{UserID: user, Limit: 2, Cursor: page.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if rest.NextCursor != "" || !sameIDs(idsOf(rest), []int64{lateOld, lateNone}) {
		t.Fatalf("page2 %+v cursor %q", idsOf(rest), rest.NextCursor)
	}
	if rest.Cards[1].State != "new" || rest.Cards[0].Gloss != "coffee" {
		t.Fatalf("suspended or gloss %+v", rest.Cards)
	}

	var walked []int64
	cursor := ""
	for {
		step, err := s.CoachCards(ctx, CoachQuery{UserID: user, Limit: 1, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		if len(step.Cards) != 1 {
			t.Fatalf("step %+v", step.Cards)
		}
		walked = append(walked, step.Cards[0].ID)
		if step.NextCursor == "" {
			break
		}
		cursor = step.NextCursor
	}
	want := []int64{early, lateNew, lateOld, lateNone}
	if len(walked) != len(want) {
		t.Fatalf("walked %v", walked)
	}
	for i := range want {
		if walked[i] != want[i] {
			t.Fatalf("walked %v want %v", walked, want)
		}
	}

	onlyReview, err := s.CoachCards(ctx, CoachQuery{UserID: user, Limit: 10, State: "review"})
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(idsOf(onlyReview), []int64{lateNew}) {
		t.Fatalf("state %+v", idsOf(onlyReview))
	}
	before := now.Add(time.Hour)
	earlyOnly, err := s.CoachCards(ctx, CoachQuery{UserID: user, Limit: 10, DueBefore: &before})
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(idsOf(earlyOnly), []int64{early}) {
		t.Fatalf("dueBefore %+v", idsOf(earlyOnly))
	}
	if _, err := s.CoachCards(ctx, CoachQuery{UserID: user, Limit: 10, Cursor: "nope"}); !errors.Is(err, ErrBadCoachCursor) {
		t.Fatalf("cursor %v", err)
	}

	acc, err := s.CoachAccuracy(ctx, user, now, 7*24*time.Hour, 50)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Reviews7d.Count != 2 || acc.Reviews7d.GoodOrEasy != 1 {
		t.Fatalf("7d %+v", acc.Reviews7d)
	}
	if acc.LastReviews.Count != 3 || acc.LastReviews.GoodOrEasy != 2 {
		t.Fatalf("last %+v", acc.LastReviews)
	}
	if acc.Lessons7d.Finished != 3 || acc.Lessons7d.Scored != 2 || acc.Lessons7d.ScoreSum != 1.5 {
		t.Fatalf("lessons %+v", acc.Lessons7d)
	}
}

func insertCoachCard(t *testing.T, s *Store, user, lex int64, skill, state, due, last string, suspended int) int64 {
	t.Helper()
	var lastArg any
	if last != "" {
		lastArg = last
	}
	res := mustExec(t, s, `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended)
		VALUES (?, ?, ?, ?, ?, 1.5, 2.5, 4, 3, ?, ?)`, user, lex, skill, state, due, lastArg, suspended)
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertLesson(t *testing.T, s *Store, user, unit int64, ix int, finished string, score any) {
	t.Helper()
	mustExec(t, s, `INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score)
		VALUES (?, ?, ?, ?, ?, ?)`, user, unit, ix, finished, finished, score)
}

func sameIDs(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func idsOf(page CoachPage) []int64 {
	out := make([]int64, len(page.Cards))
	for i, card := range page.Cards {
		out[i] = card.ID
	}
	return out
}
