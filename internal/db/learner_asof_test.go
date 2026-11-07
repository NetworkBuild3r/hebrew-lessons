package db

import (
	"context"
	"testing"
	"time"
)

func testLearnerQuery() LearnerQuery {
	return LearnerQuery{AgainWindow: 5, AccuracyWindow: 10, RecentWindow: 3}
}

func TestLearnerEvidenceAsOfFiltersAttemptsAndReviews(t *testing.T) {
	when := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	early := when
	late := when.Add(2 * time.Hour)
	asOf := when.Add(time.Hour)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "asof@example.com", "sub-asof", "learner")
	lex := mustLexemeText(t, s, "מַיִם")

	mustExec(t, s, `INSERT INTO attempts (
		user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at
	) VALUES (?, ?, 'read', 'listen_pick', 'practice', 'p', 'r', 1, 0, 0, 100, ?)`, user, lex, FormatTime(early))
	mustExec(t, s, `INSERT INTO attempts (
		user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at
	) VALUES (?, ?, 'read', 'listen_pick', 'practice', 'p', 'r', 0, 0, 0, 100, ?)`, user, lex, FormatTime(late))

	cardID, err := s.InsertCard(ctx, Card{
		UserID: user, LexemeID: lex, Skill: "read", State: "review",
		Due: when, Stability: 1, Difficulty: 5, Reps: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
		VALUES (?, 3, ?, 1, 2, 0)`, cardID, FormatTime(early))
	mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
		VALUES (?, 1, ?, 1, 1, 0)`, cardID, FormatTime(late))

	q := testLearnerQuery()
	q.AsOf = asOf
	ev, err := s.LearnerEvidence(ctx, user, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.Skills) != 1 || ev.Skills[0].Outcomes != 1 {
		t.Fatalf("as-of skills = %+v, want one outcome from the earlier attempt", ev.Skills)
	}
	if ev.Skills[0].Reviews != 1 || ev.Skills[0].ReviewAgains != 0 {
		t.Fatalf("as-of card reviews = %+v, want only the earlier Good", ev.Skills[0])
	}

	hist, err := s.CardHistories(ctx, user, asOf)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 || len(hist[0].Reviews) != 1 || hist[0].Reviews[0].Rating != 3 {
		t.Fatalf("CardHistories as-of = %+v, want one Good row", hist)
	}
	full, err := s.CardHistories(ctx, user, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(full) != 1 || len(full[0].Reviews) != 2 {
		t.Fatalf("CardHistories full = %+v, want two rows", full)
	}

	ret, err := s.DueRetention(ctx, user, early.Add(-time.Minute), asOf)
	if err != nil {
		t.Fatal(err)
	}
	if len(ret) != 1 || ret[0].Due != 1 || ret[0].Right != 1 {
		t.Fatalf("DueRetention as-of = %+v, want one right due review", ret)
	}
}

func TestLearnerEvidenceZeroAsOfUnchanged(t *testing.T) {
	when := time.Date(2026, 11, 6, 11, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "zero-asof@example.com", "sub-zero-asof", "learner")
	lex := mustLexemeText(t, s, "בַּיִת")
	mustExec(t, s, `INSERT INTO attempts (
		user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at
	) VALUES (?, ?, 'read', 'listen_pick', 'practice', 'p', 'r', 1, 0, 0, 50, ?)`, user, lex, FormatTime(when))
	mustExec(t, s, `INSERT INTO attempts (
		user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at
	) VALUES (?, ?, 'read', 'listen_pick', 'practice', 'p', 'r', 0, 0, 0, 50, ?)`, user, lex, FormatTime(when.Add(time.Minute)))

	live, err := s.LearnerEvidence(ctx, user, testLearnerQuery())
	if err != nil {
		t.Fatal(err)
	}
	bound, err := s.LearnerEvidence(ctx, user, testLearnerQuery()) // AsOf zero
	if err != nil {
		t.Fatal(err)
	}
	if live.Skills[0].Outcomes != 2 || bound.Skills[0].Outcomes != 2 {
		t.Fatalf("zero AsOf outcomes live=%d bound=%d, want 2", live.Skills[0].Outcomes, bound.Skills[0].Outcomes)
	}
}

func TestLearnerEvidenceAsOfFiltersExposures(t *testing.T) {
	when := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "exp-asof@example.com", "sub-exp-asof", "learner")
	lex := mustLexemeText(t, s, "שָׁלוֹם")
	mustExec(t, s, `INSERT INTO exposures (user_id, lexeme_id, context, created_at) VALUES (?, ?, 'story', ?)`,
		user, lex, FormatTime(when.Add(-time.Hour)))
	mustExec(t, s, `INSERT INTO exposures (user_id, lexeme_id, context, created_at) VALUES (?, ?, 'story', ?)`,
		user, lex, FormatTime(when.Add(time.Hour)))

	q := testLearnerQuery()
	q.AsOf = when
	ev, err := s.LearnerEvidence(ctx, user, q)
	if err != nil {
		t.Fatal(err)
	}
	if exp := ev.Exposures[lex]; exp.Count != 1 {
		t.Fatalf("as-of exposures = %+v, want count 1", exp)
	}
	live, err := s.LearnerEvidence(ctx, user, testLearnerQuery())
	if err != nil {
		t.Fatal(err)
	}
	if live.Exposures[lex].Count != 2 {
		t.Fatalf("live exposures = %+v, want count 2", live.Exposures[lex])
	}
}

func TestCardHistoriesIncludesUnreviewedAndStoredFields(t *testing.T) {
	when := time.Date(2026, 11, 6, 13, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "hist@example.com", "sub-hist", "learner")
	lex := mustLexemeText(t, s, "סֵפֶר")
	id, err := s.InsertCard(ctx, Card{
		UserID: user, LexemeID: lex, Skill: "read", State: "new",
		Due: when, CreatedAt: when,
	})
	if err != nil {
		t.Fatal(err)
	}
	full, err := s.CardHistories(ctx, user, time.Time{})
	if err != nil || len(full) != 1 || full[0].CardID != id || len(full[0].Reviews) != 0 {
		t.Fatalf("unreviewed CardHistories = %+v, err=%v", full, err)
	}
	if full[0].State != "new" || full[0].Stability != 0 || !full[0].CreatedAt.Equal(when) {
		t.Fatalf("stored fields = %+v", full[0])
	}
	before, err := s.CardHistories(ctx, user, when.Add(-time.Minute))
	if err != nil || len(before) != 0 {
		t.Fatalf("before creation = %+v, err=%v", before, err)
	}
}
