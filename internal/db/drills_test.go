package db

import (
	"context"
	"testing"
	"time"
)

func TestDrillAnswersAndClearsLearnerIsolation(t *testing.T) {
	when := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	a := insertUser(t, s, "drill-a@example.com", "sub-drill-a", "learner")
	b := insertUser(t, s, "drill-b@example.com", "sub-drill-b", "learner")

	if err := s.InsertDrillAnswer(ctx, DrillAnswer{
		LearnerID: a, Kind: "letter_pair", Key: "ב/כ", Correct: true, LatencyMS: 400, At: when.Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertDrillAnswer(ctx, DrillAnswer{
		LearnerID: b, Kind: "letter_pair", Key: "ב/כ", Correct: false, LatencyMS: 900, At: when,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertConfusionClear(ctx, a, "letter_pair", "ב/כ", when.Add(-30*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertConfusionClear(ctx, b, "letter_pair", "ב/כ", when); err != nil {
		t.Fatal(err)
	}

	got, err := s.DrillAnswersSince(ctx, a, "letter_pair", "ב/כ", when.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].LearnerID != a || !got[0].Correct || got[0].LatencyMS != 400 {
		t.Fatalf("learner A answers = %+v", got)
	}
	gotB, err := s.DrillAnswersSince(ctx, b, "letter_pair", "ב/כ", when.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(gotB) != 1 || gotB[0].LearnerID != b || gotB[0].Correct {
		t.Fatalf("learner B answers = %+v", gotB)
	}

	clearA, ok, err := s.LatestConfusionClear(ctx, a, "letter_pair", "ב/כ")
	if err != nil || !ok || !clearA.Equal(when.Add(-30*time.Second)) {
		t.Fatalf("clear A = %v ok=%v err=%v", clearA, ok, err)
	}
	clearB, ok, err := s.LatestConfusionClear(ctx, b, "letter_pair", "ב/כ")
	if err != nil || !ok || !clearB.Equal(when) {
		t.Fatalf("clear B = %v ok=%v err=%v", clearB, ok, err)
	}

	lats, err := s.CorrectDrillLatencies(ctx, a, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(lats) != 1 || lats[0] != 400 {
		t.Fatalf("latencies A = %v", lats)
	}
	latsB, err := s.CorrectDrillLatencies(ctx, b, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(latsB) != 0 {
		t.Fatalf("latencies B = %v, want none (wrong answer)", latsB)
	}
}

func TestLastConfusionMissAndClearOrdering(t *testing.T) {
	when := time.Date(2026, 11, 1, 15, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "miss-order@example.com", "sub-miss-order", "learner")

	if _, ok, err := s.LastConfusionMiss(ctx, user, "letter_pair", "ב/כ"); err != nil || ok {
		t.Fatalf("empty miss: ok=%v err=%v", ok, err)
	}
	if _, ok, err := s.LatestConfusionClear(ctx, user, "letter_pair", "ב/כ"); err != nil || ok {
		t.Fatalf("empty clear: ok=%v err=%v", ok, err)
	}

	if err := s.InsertConfusionMiss(ctx, user, "letter_pair", "ב/כ", 0, when.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertConfusionMiss(ctx, user, "letter_pair", "ב/כ", 0, when.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	miss, ok, err := s.LastConfusionMiss(ctx, user, "letter_pair", "ב/כ")
	if err != nil || !ok || !miss.Equal(when.Add(-time.Hour)) {
		t.Fatalf("last miss = %v ok=%v err=%v", miss, ok, err)
	}

	if err := s.InsertConfusionClear(ctx, user, "letter_pair", "ב/כ", when.Add(-90*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertConfusionClear(ctx, user, "letter_pair", "ב/כ", when.Add(-30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	clear, ok, err := s.LatestConfusionClear(ctx, user, "letter_pair", "ב/כ")
	if err != nil || !ok || !clear.Equal(when.Add(-30*time.Minute)) {
		t.Fatalf("latest clear = %v ok=%v err=%v", clear, ok, err)
	}
}

// ac-5: with two misses, a clear, then one miss, RecentConfusionPairs reports 1.
// Without the clear it reports 3.
func TestRecentConfusionPairsRespectsClear(t *testing.T) {
	when := time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "clear-pairs@example.com", "sub-clear-pairs", "learner")
	since := when.Add(-24 * time.Hour)

	if err := s.InsertConfusionMiss(ctx, user, "letter_pair", "ב/כ", 0, when.Add(-3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertConfusionMiss(ctx, user, "letter_pair", "ב/כ", 0, when.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertConfusionMiss(ctx, user, "letter_pair", "ב/כ", 0, when.Add(-30*time.Minute)); err != nil {
		t.Fatal(err)
	}

	before, err := s.RecentConfusionPairs(ctx, user, since)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].A != "ב" || before[0].B != "כ" || before[0].Count != 3 {
		t.Fatalf("without clear = %+v, want count 3", before)
	}

	if err := s.InsertConfusionClear(ctx, user, "letter_pair", "ב/כ", when.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	after, err := s.RecentConfusionPairs(ctx, user, since)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].A != "ב" || after[0].B != "כ" || after[0].Count != 1 {
		t.Fatalf("with clear = %+v, want count 1", after)
	}
}

// ac-6: an implied review is left out of accuracy counts; a normal row is still counted.
func TestImpliedReviewsExcludedFromAccuracy(t *testing.T) {
	when := time.Date(2026, 11, 1, 20, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "implied@example.com", "sub-implied", "learner")
	lex := insertLexeme(t, s, "בַּיִת")
	cardID, err := s.InsertCard(ctx, Card{
		UserID: user, LexemeID: lex, Skill: "read", State: "review",
		Due: when, Stability: 2, Difficulty: 5, Reps: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	normal := ReviewLog{CardID: cardID, Rating: 3, ReviewedAt: when.Add(-time.Hour), ElapsedDays: 1, ScheduledDays: 2}
	if err := s.ApplyReview(ctx, Card{
		ID: cardID, UserID: user, LexemeID: lex, Skill: "read", State: "review",
		Due: when.Add(48 * time.Hour), Stability: 3, Difficulty: 5, Reps: 2, LastReview: when.Add(-time.Hour),
	}, normal); err != nil {
		t.Fatal(err)
	}
	implied := ReviewLog{CardID: cardID, Rating: 3, ReviewedAt: when.Add(-30 * time.Minute), ElapsedDays: 1, ScheduledDays: 2, Implied: true}
	if err := s.ApplyReview(ctx, Card{
		ID: cardID, UserID: user, LexemeID: lex, Skill: "read", State: "review",
		Due: when.Add(72 * time.Hour), Stability: 4, Difficulty: 5, Reps: 3, LastReview: when.Add(-30 * time.Minute),
	}, implied); err != nil {
		t.Fatal(err)
	}

	logs, err := s.ReviewLogs(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 || logs[0].Implied || !logs[1].Implied {
		t.Fatalf("ReviewLogs = %+v", logs)
	}

	ratings, err := s.RecentReviewRatings(ctx, user, when, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ratings) != 1 || ratings[0] != 3 {
		t.Fatalf("RecentReviewRatings = %v, want one normal Good", ratings)
	}

	acc, err := s.CoachAccuracy(ctx, user, when, 7*24*time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Reviews7d.Count != 1 || acc.Reviews7d.GoodOrEasy != 1 {
		t.Fatalf("Reviews7d = %+v, want count 1", acc.Reviews7d)
	}
	if acc.LastReviews.Count != 1 || acc.LastReviews.GoodOrEasy != 1 {
		t.Fatalf("LastReviews = %+v, want count 1", acc.LastReviews)
	}

	ret, err := s.DueRetention(ctx, user, when.Add(-24*time.Hour), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ret) != 1 || ret[0].Due != 1 || ret[0].Right != 1 {
		t.Fatalf("DueRetention = %+v, want one non-implied due review", ret)
	}

	struggle, err := s.StruggleStats(ctx, user, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(struggle.Lexemes) != 1 || struggle.Lexemes[0].Reviews != 1 {
		t.Fatalf("StruggleStats reviews = %+v, want 1 (implied excluded)", struggle.Lexemes)
	}
}

func TestCorrectDrillLatenciesNewestTimedRight(t *testing.T) {
	when := time.Date(2026, 11, 1, 21, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "lat@example.com", "sub-lat", "learner")

	answers := []DrillAnswer{
		{LearnerID: user, Kind: "letter_pair", Key: "א/ע", Correct: true, LatencyMS: 100, At: when.Add(-4 * time.Minute)},
		{LearnerID: user, Kind: "letter_pair", Key: "א/ע", Correct: true, LatencyMS: 0, At: when.Add(-3 * time.Minute)},
		{LearnerID: user, Kind: "letter_pair", Key: "א/ע", Correct: false, LatencyMS: 200, At: when.Add(-2 * time.Minute)},
		{LearnerID: user, Kind: "letter_pair", Key: "א/ע", Correct: true, LatencyMS: 300, At: when.Add(-time.Minute)},
		{LearnerID: user, Kind: "letter_pair", Key: "א/ע", Correct: true, LatencyMS: 250, At: when},
	}
	for _, a := range answers {
		if err := s.InsertDrillAnswer(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.CorrectDrillLatencies(ctx, user, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 250 || got[1] != 300 {
		t.Fatalf("latencies = %v, want [250 300]", got)
	}
	since := when.Add(-90 * time.Second)
	window, err := s.DrillAnswersSince(ctx, user, "letter_pair", "א/ע", since)
	if err != nil {
		t.Fatal(err)
	}
	if len(window) != 2 || window[0].LatencyMS != 300 || window[1].LatencyMS != 250 {
		t.Fatalf("since window = %+v", window)
	}
}
