package db

import (
	"context"
	"testing"
	"time"
)

func TestStruggleStatsOrderWindowAndPairs(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "struggle@example.com", "sub-struggle", "learner")
	otherID := insertUser(t, s, "other-struggle@example.com", "sub-other-struggle", "learner")

	empty, err := s.StruggleStats(ctx, userID, 0)
	if err != nil || len(empty.Lexemes) != 0 || len(empty.Top) != 0 || len(empty.Confusions) != 0 {
		t.Fatalf("empty: %+v %v", empty, err)
	}

	alef := struggleCard(t, s, userID, "א", "alef", 1)
	bet := struggleCard(t, s, userID, "ב", "bet", 5)
	gimel := struggleCard(t, s, userID, "ג", "gimel", 5)
	dalet := struggleCard(t, s, userID, "ד", "dalet", 5)
	_ = struggleCard(t, s, userID, "ה", "silent", 9)
	other := struggleCard(t, s, otherID, "ז", "other", 4)
	insertReview(t, s, other, 1, when.Add(-time.Hour))

	week := 7 * 24 * time.Hour
	insertReview(t, s, alef, 1, when.Add(-8*24*time.Hour))
	insertReview(t, s, alef, 1, when.Add(-week))
	insertReview(t, s, alef, 1, when.Add(-time.Hour))
	// One nanosecond before the window is not a 7-day miss. It still counts in AgainRate.
	insertReview(t, s, alef, 1, when.Add(-week-time.Nanosecond))
	insertReview(t, s, bet, 1, when.Add(-8*24*time.Hour))
	insertReview(t, s, bet, 3, when.Add(-2*24*time.Hour))
	insertReview(t, s, dalet, 1, when.Add(-8*24*time.Hour))
	insertReview(t, s, dalet, 3, when.Add(-2*24*time.Hour))
	insertReview(t, s, gimel, 3, when.Add(-time.Hour))

	var alefID int64
	if err := s.queryRow(ctx, `SELECT lexeme_id FROM cards WHERE id = ?`, alef).Scan(&alefID); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertConfusionMiss(ctx, userID, "letter_pair", "ב/כ", alefID, when.Add(-week)); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertConfusionMiss(ctx, userID, "letter_pair", "ב/כ", alefID, when.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	// Outside the window, and not on this lexeme's keys.
	if err := s.InsertConfusionMiss(ctx, userID, "letter_pair", "ג/ק", alefID, when.Add(-week-time.Nanosecond)); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertConfusionMiss(ctx, userID, "agreement", "הוא/היא", 0, when.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertConfusionMiss(ctx, otherID, "letter_pair", "ב/כ", 0, when.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	got, err := s.StruggleStats(ctx, userID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if texts := textsOf(got.Lexemes); !sameTexts(texts, []string{"א", "ב", "ג", "ד"}) {
		t.Fatalf("lexemes = %v", texts)
	}
	alefRow := got.Lexemes[0]
	if alefRow.Misses7d != 2 || alefRow.Reviews != 4 || alefRow.Lapses != 1 || alefRow.AgainRate != 1 || alefRow.Gloss != "alef" {
		t.Fatalf("alef = %+v", alefRow)
	}
	if !alefRow.LastSeen.Equal(when.Add(-time.Hour)) {
		t.Fatalf("alef last = %s", alefRow.LastSeen)
	}
	if len(alefRow.ConfusionKeys) != 1 || alefRow.ConfusionKeys[0] != "ב/כ" {
		t.Fatalf("alef keys = %v", alefRow.ConfusionKeys)
	}
	if texts := textsOf(got.Top); !sameTexts(texts, []string{"א", "ב"}) {
		t.Fatalf("top = %v", texts)
	}
	if got.Top[1].Misses7d != 0 || got.Top[1].AgainRate != 0.5 || got.Top[1].Lapses != 5 {
		t.Fatalf("bet = %+v", got.Top[1])
	}

	all, err := s.StruggleStats(ctx, userID, 0)
	if err != nil || len(all.Top) != 0 || len(all.Lexemes) != 4 {
		t.Fatalf("topN 0: top=%v lexemes=%v err=%v", textsOf(all.Top), textsOf(all.Lexemes), err)
	}
	ranked, err := s.StruggleStats(ctx, userID, len(all.Lexemes))
	if err != nil {
		t.Fatal(err)
	}
	if texts := textsOf(ranked.Top); !sameTexts(texts, []string{"א", "ב", "ד", "ג"}) {
		t.Fatalf("ranked = %v", texts)
	}
	gimelRow := ranked.Top[3]
	if gimelRow.TextNFC != "ג" || gimelRow.Misses7d != 0 || gimelRow.AgainRate != 0 {
		t.Fatalf("gimel = %+v", gimelRow)
	}
	none, err := s.StruggleStats(ctx, userID, -1)
	if err != nil || len(none.Top) != 0 {
		t.Fatalf("topN -1: %v %v", textsOf(none.Top), err)
	}

	if len(got.Confusions) != 2 {
		t.Fatalf("confusions = %+v", got.Confusions)
	}
	pair := got.Confusions[0]
	if pair.Kind != "letter_pair" || pair.A != "ב" || pair.B != "כ" || pair.Count != 2 || !pair.LastAt.Equal(when.Add(-time.Hour)) {
		t.Fatalf("pair = %+v", pair)
	}
	agreement := got.Confusions[1]
	if agreement.Kind != "agreement" || agreement.A != "הוא/היא" || agreement.B != "" || agreement.Count != 1 || !agreement.LastAt.Equal(when.Add(-2*time.Hour)) {
		t.Fatalf("agreement = %+v", agreement)
	}
}

func TestChosenAndConfusionLexeme(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "chosen@example.com", "sub-chosen", "learner")
	lexemeID := insertLexeme(t, s, "שָׁלוֹם")
	cardID, err := s.InsertCard(ctx, Card{
		UserID: userID, LexemeID: lexemeID, Dir: "he_en", State: "new", Due: when,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyReview(ctx, Card{
		ID: cardID, State: "learning", Due: when, LastReview: when,
	}, ReviewLog{CardID: cardID, Rating: 3, ReviewedAt: when}); err != nil {
		t.Fatal(err)
	}
	var raw *string
	if err := s.queryRow(ctx, `SELECT chosen FROM review_log`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw != nil {
		t.Fatalf("chosen = %q, want NULL", *raw)
	}
	if err := s.ApplyReview(ctx, Card{
		ID: cardID, State: "learning", Due: when, LastReview: when,
	}, ReviewLog{CardID: cardID, Rating: 1, ReviewedAt: when, Chosen: "שָׁלוֹם"}); err != nil {
		t.Fatal(err)
	}
	logs, err := s.ReviewLogs(ctx, cardID)
	if err != nil || len(logs) != 2 || logs[0].Chosen != "" || logs[1].Chosen != "שָׁלוֹם" {
		t.Fatalf("logs = %+v %v", logs, err)
	}

	if err := s.InsertConfusionMiss(ctx, userID, "letter_pair", "ב/כ", 0, when); err != nil {
		t.Fatal(err)
	}
	var stored *int64
	if err := s.queryRow(ctx, `SELECT lexeme_id FROM confusion_misses`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != nil {
		t.Fatalf("lexeme_id = %v, want NULL", *stored)
	}
	if err := s.InsertConfusionMiss(ctx, userID, "agreement", "הוא/היא", lexemeID, when); err != nil {
		t.Fatal(err)
	}
	if err := s.queryRow(ctx, `SELECT lexeme_id FROM confusion_misses WHERE kind = 'agreement'`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == nil || *stored != lexemeID {
		t.Fatalf("lexeme_id = %v, want %d", stored, lexemeID)
	}
	if _, err := s.exec(ctx, `INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at) VALUES (?, 'letter_pair', 'ב/כ', 99999, ?)`, userID, FormatTime(when)); err == nil {
		t.Fatal("unknown lexeme was accepted")
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

func sameTexts(got, want []string) bool {
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
