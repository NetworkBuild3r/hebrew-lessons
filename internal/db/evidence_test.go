package db

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestCorrectLatencies(t *testing.T) {
	when := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "a@example.com", "sub-a", "learner")
	other := insertUser(t, s, "b@example.com", "sub-b", "learner")
	lex := insertLexeme(t, s, "כלב")

	write := func(userID int64, activity string, correct bool, latency int64, at time.Time) {
		t.Helper()
		_, err := s.WriteAttempt(ctx, AttemptWrite{Attempt: Attempt{
			UserID: userID, LexemeID: lex, Skill: "read", Activity: activity, Source: "review",
			Prompt: "p", Response: "r", Correct: correct, LatencyMS: latency, CreatedAt: at,
		}})
		if err != nil {
			t.Fatal(err)
		}
	}
	write(user, "pick", true, 1000, when.Add(1*time.Minute))
	write(user, "pick", true, 2000, when.Add(2*time.Minute))
	write(user, "pick", false, 300, when.Add(3*time.Minute)) // wrong: left out
	write(user, "pick", true, 0, when.Add(4*time.Minute))    // not timed: left out
	write(user, "pick", true, 3000, when.Add(5*time.Minute))
	write(user, "type", true, 9000, when.Add(6*time.Minute))  // other activity
	write(other, "pick", true, 7000, when.Add(7*time.Minute)) // other learner

	got, err := s.CorrectLatencies(ctx, user, "pick", 50)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{3000, 2000, 1000}; !reflect.DeepEqual(got, want) {
		t.Fatalf("latencies = %v, want %v (newest first)", got, want)
	}
	got, err = s.CorrectLatencies(ctx, user, "pick", 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{3000, 2000}; !reflect.DeepEqual(got, want) {
		t.Fatalf("limited latencies = %v, want %v", got, want)
	}
	if got, err := s.CorrectLatencies(ctx, user, "pick", 0); err != nil || got != nil {
		t.Fatalf("limit 0 = %v, %v", got, err)
	}
	if got, err := s.CorrectLatencies(ctx, user, "none", 5); err != nil || len(got) != 0 {
		t.Fatalf("unknown activity = %v, %v", got, err)
	}
}

func TestWriteAttemptStepCanLeaveCardsAlone(t *testing.T) {
	when := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "a@example.com", "sub-a", "learner")
	lex := insertLexeme(t, s, "כלב")
	var sawPrev bool
	res, err := s.WriteAttempt(ctx, AttemptWrite{
		Attempt: Attempt{UserID: user, LexemeID: lex, Skill: "read", Activity: "pick", Source: "review", Correct: true, CreatedAt: when},
		Step: func(prev *Card) (Card, ReviewLog, bool, error) {
			sawPrev = prev != nil
			return Card{}, ReviewLog{}, false, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sawPrev || res.Scheduled || res.CardID != 0 || res.AttemptID == 0 {
		t.Fatalf("result = %+v, sawPrev %v", res, sawPrev)
	}
	var cards int
	if err := s.queryRow(ctx, `SELECT COUNT(*) FROM cards`).Scan(&cards); err != nil || cards != 0 {
		t.Fatalf("cards = %d, %v", cards, err)
	}
}

func TestWriteAttemptRefusesAMissingLearnerOrWord(t *testing.T) {
	when := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	for _, a := range []Attempt{{LexemeID: 1}, {UserID: 1}, {}} {
		if _, err := s.WriteAttempt(context.Background(), AttemptWrite{Attempt: a}); err == nil {
			t.Fatalf("accepted %+v", a)
		}
	}
}
