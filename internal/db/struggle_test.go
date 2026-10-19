package db

import (
	"context"
	"testing"
	"time"
)

func TestLexemeStrugglesOrderAndWindow(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "struggle@example.com", "sub-struggle", "learner")
	otherID := insertUser(t, s, "other-struggle@example.com", "sub-other-struggle", "learner")

	if rows, err := s.LexemeStruggles(ctx, userID, 0); err != nil || len(rows) != 0 {
		t.Fatalf("empty: %v %v", rows, err)
	}

	alef := struggleCard(t, s, userID, "א", "alef", 1)
	bet := struggleCard(t, s, userID, "ב", "bet", 5)
	gimel := struggleCard(t, s, userID, "ג", "gimel", 5)
	dalet := struggleCard(t, s, userID, "ד", "dalet", 5)
	silent := struggleCard(t, s, userID, "ה", "silent", 9)
	_ = silent
	other := struggleCard(t, s, otherID, "ז", "other", 4)
	insertReview(t, s, other, 1, when.Add(-time.Hour))

	week := 7 * 24 * time.Hour
	insertReview(t, s, alef, 1, when.Add(-8*24*time.Hour))
	insertReview(t, s, alef, 1, when.Add(-week))
	insertReview(t, s, alef, 1, when.Add(-time.Hour))
	// One nanosecond before the window is not a 7-day miss. It still
	// counts in AgainRate, which is why this review sits on alef.
	insertReview(t, s, alef, 1, when.Add(-week-time.Nanosecond))
	insertReview(t, s, bet, 1, when.Add(-8*24*time.Hour))
	insertReview(t, s, bet, 3, when.Add(-2*24*time.Hour))
	insertReview(t, s, dalet, 1, when.Add(-8*24*time.Hour))
	insertReview(t, s, dalet, 3, when.Add(-2*24*time.Hour))
	insertReview(t, s, gimel, 3, when.Add(-time.Hour))

	all, err := s.LexemeStruggles(ctx, userID, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"א", "ב", "ד", "ג"}
	if len(all) != len(want) {
		t.Fatalf("struggles = %+v", all)
	}
	for i, text := range want {
		if all[i].TextNFC != text {
			t.Fatalf("order = %v", textsOf(all))
		}
	}
	if all[0].Misses7d != 2 || all[0].Reviews != 4 || all[0].Lapses != 1 || all[0].AgainRate != 1 || all[0].Gloss != "alef" {
		t.Fatalf("alef = %+v", all[0])
	}
	if !all[0].LastReviewAt.Equal(when.Add(-time.Hour)) {
		t.Fatalf("alef last = %s", all[0].LastReviewAt)
	}
	if all[1].Misses7d != 0 || all[1].AgainRate != 0.5 || all[1].Lapses != 5 {
		t.Fatalf("bet = %+v", all[1])
	}
	if all[3].Misses7d != 0 || all[3].AgainRate != 0 {
		t.Fatalf("gimel = %+v", all[3])
	}
	top, err := s.LexemeStruggles(ctx, userID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 2 || top[0].TextNFC != "א" || top[1].TextNFC != "ב" {
		t.Fatalf("top = %v", textsOf(top))
	}
	if rows, err := s.LexemeStruggles(ctx, userID, -1); err != nil || len(rows) != 4 {
		t.Fatalf("all = %v %v", textsOf(rows), err)
	}
}

func TestConfusionCountsWindow(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "confuse@example.com", "sub-confuse", "learner")
	otherID := insertUser(t, s, "confuse-other@example.com", "sub-confuse-other", "learner")
	week := 7 * 24 * time.Hour
	for _, at := range []time.Time{
		when.Add(-8 * 24 * time.Hour),
		when.Add(-week - time.Nanosecond),
	} {
		if err := s.InsertConfusionMiss(ctx, userID, "letter_pair", "ב/כ", at); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.InsertConfusionMiss(ctx, userID, "letter_pair", "ב/כ", when.Add(-week)); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertConfusionMiss(ctx, userID, "agreement", "ב/כ", when.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertConfusionMiss(ctx, userID, "agreement", "הוא/היא", when.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertConfusionMiss(ctx, otherID, "letter_pair", "ב/כ", when.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	got, err := s.ConfusionCounts(ctx, userID, week)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("counts = %+v", got)
	}
	if got[0].Key != "ב/כ" || got[0].Count != 2 || got[0].Kind != "agreement" || !got[0].LastAt.Equal(when.Add(-time.Hour)) {
		t.Fatalf("first = %+v", got[0])
	}
	if got[1].Key != "הוא/היא" || got[1].Count != 1 || got[1].Kind != "agreement" || !got[1].LastAt.Equal(when.Add(-2*time.Hour)) {
		t.Fatalf("second = %+v", got[1])
	}
}

func struggleCard(t *testing.T, s *Store, userID int64, text, gloss string, lapses int) int64 {
	t.Helper()
	id := insertLexeme(t, s, text)
	if _, err := s.exec(context.Background(), `UPDATE lexemes SET gloss_en = ? WHERE id = ?`, gloss, id); err != nil {
		t.Fatal(err)
	}
	cardID, err := s.InsertCard(context.Background(), Card{
		UserID:   userID,
		LexemeID: id,
		Dir:      "he_en",
		State:    "new",
		Due:      time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC),
		Lapses:   lapses,
	})
	if err != nil {
		t.Fatal(err)
	}
	return cardID
}

func insertReview(t *testing.T, s *Store, cardID int64, rating int, at time.Time) {
	t.Helper()
	if _, err := s.exec(context.Background(), `
		INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked)
		VALUES (?, ?, ?, 0, 0, 0)`, cardID, rating, FormatTime(at)); err != nil {
		t.Fatal(err)
	}
}

func textsOf(rows []LexemeStruggle) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.TextNFC
	}
	return out
}
