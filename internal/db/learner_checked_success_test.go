package db

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// INIT-036/SPEC-002: checked successes, study days, strict knownLexemeUnion,
// waiting-for-second-day count, and cross-learner isolation.

func insertCheckedSuccesses(t *testing.T, s *Store, cardID int64, times []time.Time, rating int) {
	t.Helper()
	for _, at := range times {
		mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, implied)
			VALUES (?, ?, ?, 1, 2, 0, 0)`, cardID, rating, FormatTime(at))
	}
}

func seedStrictReadCard(t *testing.T, s *Store, user, lex int64, when time.Time) int64 {
	t.Helper()
	cardID, err := s.InsertCard(context.Background(), Card{
		UserID: user, LexemeID: lex, Skill: "read", State: "review",
		Due: when, Stability: 10, Difficulty: 5, Reps: 3, LastReview: when,
	})
	if err != nil {
		t.Fatal(err)
	}
	day1 := when.Add(-48 * time.Hour)
	insertCheckedSuccesses(t, s, cardID, []time.Time{day1, day1.Add(time.Hour), when}, 3)
	return cardID
}

func TestLearnerEvidenceCheckedSuccessDays(t *testing.T) {
	when := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()

	tests := []struct {
		name          string
		setup         func(t *testing.T, s *Store, user, lex int64)
		wantSuccesses int
		wantDays      int
		wantRecent    int
		wantRight     int
	}{
		{
			name: "three same-day rights: three successes, one day",
			setup: func(t *testing.T, s *Store, user, lex int64) {
				cardID, err := s.InsertCard(ctx, Card{
					UserID: user, LexemeID: lex, Skill: "read", State: "review",
					Due: when, Stability: 10, LastReview: when, Reps: 3,
				})
				if err != nil {
					t.Fatal(err)
				}
				base := when.Add(-2 * time.Hour)
				insertCheckedSuccesses(t, s, cardID, []time.Time{
					base, base.Add(30 * time.Minute), base.Add(time.Hour),
				}, 3)
			},
			wantSuccesses: 3, wantDays: 1, wantRecent: 3, wantRight: 3,
		},
		{
			name: "three rights across two study days: two days",
			setup: func(t *testing.T, s *Store, user, lex int64) {
				cardID, err := s.InsertCard(ctx, Card{
					UserID: user, LexemeID: lex, Skill: "read", State: "review",
					Due: when, Stability: 10, LastReview: when, Reps: 3,
				})
				if err != nil {
					t.Fatal(err)
				}
				day1 := when.Add(-48 * time.Hour)
				insertCheckedSuccesses(t, s, cardID, []time.Time{
					day1, day1.Add(time.Hour), when,
				}, 3)
			},
			wantSuccesses: 3, wantDays: 2, wantRecent: 3, wantRight: 3,
		},
		{
			name: "peeked review adds no success, day, or newest-three right",
			setup: func(t *testing.T, s *Store, user, lex int64) {
				cardID, err := s.InsertCard(ctx, Card{
					UserID: user, LexemeID: lex, Skill: "read", State: "review",
					Due: when, Stability: 1, LastReview: when,
				})
				if err != nil {
					t.Fatal(err)
				}
				mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, implied)
					VALUES (?, 3, ?, 1, 2, 1, 0)`, cardID, FormatTime(when))
				mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, implied)
					VALUES (?, 3, ?, 1, 2, 1, 0)`, cardID, FormatTime(when.Add(time.Minute)))
				mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, implied)
					VALUES (?, 3, ?, 1, 2, 1, 0)`, cardID, FormatTime(when.Add(2*time.Minute)))
			},
			wantSuccesses: 0, wantDays: 0, wantRecent: 0, wantRight: 0,
		},
		{
			name: "say self-check (peeked) adds no success, day, or newest-three right",
			setup: func(t *testing.T, s *Store, user, lex int64) {
				cardID, err := s.InsertCard(ctx, Card{
					UserID: user, LexemeID: lex, Skill: "recall", State: "review",
					Due: when, Stability: 1, LastReview: when,
				})
				if err != nil {
					t.Fatal(err)
				}
				// Historical say self-check rows (peeked review_log + say attempt).
				mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, implied)
					VALUES (?, 3, ?, 1, 2, 1, 0)`, cardID, FormatTime(when))
				mustExec(t, s, `INSERT INTO attempts (
					user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at
				) VALUES (?, ?, 'recall', 'say', 'self', 'p', 'got it', 1, 0, 0, 0, ?)`,
					user, lex, FormatTime(when))
			},
			wantSuccesses: 0, wantDays: 0, wantRecent: 0, wantRight: 0,
		},
		{
			name: "typed-spelling implied credit still counts (ADR-0030)",
			setup: func(t *testing.T, s *Store, user, lex int64) {
				cardID, err := s.InsertCard(ctx, Card{
					UserID: user, LexemeID: lex, Skill: "spell", State: "review",
					Due: when, Stability: 10, LastReview: when, Reps: 3,
				})
				if err != nil {
					t.Fatal(err)
				}
				day1 := when.Add(-48 * time.Hour)
				for i, at := range []time.Time{day1, day1.Add(time.Hour), when} {
					_ = i
					mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked, implied)
						VALUES (?, 3, ?, 1, 2, 0, 1)`, cardID, FormatTime(at))
				}
			},
			wantSuccesses: 3, wantDays: 2, wantRecent: 0, wantRight: 0, // implied rows are not outcomes
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			user := insertUser(t, s, fmt.Sprintf("checked-%d@example.com", i), fmt.Sprintf("sub-checked-%d", i), "learner")
			lex := mustLexemeText(t, s, fmt.Sprintf("מִלָּה%d", i))
			tc.setup(t, s, user, lex)
			ev, err := s.LearnerEvidence(ctx, user, testLearnerQuery())
			if err != nil {
				t.Fatal(err)
			}
			if len(ev.Skills) != 1 {
				t.Fatalf("skills = %+v, want 1", ev.Skills)
			}
			got := ev.Skills[0]
			if got.ReviewSuccesses != tc.wantSuccesses || got.SuccessDays != tc.wantDays {
				t.Fatalf("successes/days = %d/%d, want %d/%d (skill=%+v)",
					got.ReviewSuccesses, got.SuccessDays, tc.wantSuccesses, tc.wantDays, got)
			}
			if got.Recent != tc.wantRecent || got.RecentRight != tc.wantRight {
				t.Fatalf("recent/right = %d/%d, want %d/%d", got.Recent, got.RecentRight, tc.wantRecent, tc.wantRight)
			}
		})
	}
}

func TestLearnerEvidenceStudyDayRollover(t *testing.T) {
	// 02:00 local counts toward the previous study day (4:00 rollover).
	when := time.Date(2026, 11, 8, 18, 0, 0, 0, time.UTC) // clock
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "rollover@example.com", "sub-rollover", "learner")
	if err := s.SetUserTimeZone(ctx, user, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	lex := mustLexemeText(t, s, "גְּבוּל")
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	// 02:00 Denver on Nov 8 → previous study day Nov 7.
	at0200 := time.Date(2026, 11, 8, 2, 0, 0, 0, loc)
	// 10:00 Denver on Nov 8 → study day Nov 8.
	at1000 := time.Date(2026, 11, 8, 10, 0, 0, 0, loc)
	cardID, err := s.InsertCard(ctx, Card{
		UserID: user, LexemeID: lex, Skill: "read", State: "review",
		Due: when, Stability: 10, LastReview: at1000, Reps: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	insertCheckedSuccesses(t, s, cardID, []time.Time{at0200, at1000}, 3)

	ev, err := s.LearnerEvidence(ctx, user, testLearnerQuery())
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.Skills) != 1 || ev.Skills[0].SuccessDays != 2 {
		t.Fatalf("rollover SuccessDays = %+v, want 2", ev.Skills)
	}
}

func TestKnownLexemeUnionStrictRule(t *testing.T) {
	when := time.Date(2026, 11, 8, 16, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "union@example.com", "sub-union", "learner")
	other := insertUser(t, s, "union-other@example.com", "sub-union-other", "learner")
	single := mustLexemeText(t, s, "אַחַת")
	strict := mustLexemeText(t, s, "שְׁתַּיִם")
	otherLex := mustLexemeText(t, s, "שָׁלוֹשׁ")

	// One Good read answer — not enough for ADR-0031.
	cardID, err := s.InsertCard(ctx, Card{
		UserID: user, LexemeID: single, Skill: "read", State: "review",
		Due: when, Stability: 10, LastReview: when, Reps: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	insertCheckedSuccesses(t, s, cardID, []time.Time{when}, 3)

	seedStrictReadCard(t, s, user, strict, when)
	seedStrictReadCard(t, s, other, otherLex, when)

	ids, err := s.KnownLexemeIDs(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != strict {
		t.Fatalf("KnownLexemeIDs = %v, want [%d]", ids, strict)
	}
	texts, err := s.LearnerKnownLexemes(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 1 || texts[0] != "שְׁתַּיִם" {
		t.Fatalf("LearnerKnownLexemes = %v", texts)
	}
	// Another learner's strict-known word must not appear.
	for _, id := range ids {
		if id == otherLex {
			t.Fatal("other learner's lexeme leaked into KnownLexemeIDs")
		}
	}
	otherIDs, err := s.KnownLexemeIDs(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherIDs) != 1 || otherIDs[0] != otherLex {
		t.Fatalf("other KnownLexemeIDs = %v, want [%d]", otherIDs, otherLex)
	}
}

func TestWaitingForSecondDayCount(t *testing.T) {
	when := time.Date(2026, 11, 8, 17, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "waiting@example.com", "sub-waiting", "learner")
	other := insertUser(t, s, "waiting-other@example.com", "sub-waiting-other", "learner")
	waiting := mustLexemeText(t, s, "מְחַכֶּה")
	known := mustLexemeText(t, s, "יָדוּעַ")
	otherLex := mustLexemeText(t, s, "אַחֵר")

	// Old known on one day only → waiting.
	cardID, err := s.InsertCard(ctx, Card{
		UserID: user, LexemeID: waiting, Skill: "read", State: "review",
		Due: when, Stability: 10, LastReview: when, Reps: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	base := when.Add(-2 * time.Hour)
	insertCheckedSuccesses(t, s, cardID, []time.Time{
		base, base.Add(20 * time.Minute), base.Add(40 * time.Minute),
	}, 3)

	seedStrictReadCard(t, s, user, known, when)
	// Other learner waiting must not inflate this learner's count.
	otherCard, err := s.InsertCard(ctx, Card{
		UserID: other, LexemeID: otherLex, Skill: "read", State: "review",
		Due: when, Stability: 10, LastReview: when, Reps: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	insertCheckedSuccesses(t, s, otherCard, []time.Time{
		base, base.Add(20 * time.Minute), base.Add(40 * time.Minute),
	}, 3)

	ev, err := s.LearnerEvidence(ctx, user, testLearnerQuery())
	if err != nil {
		t.Fatal(err)
	}
	if ev.WaitingForSecondDay != 1 {
		t.Fatalf("WaitingForSecondDay = %d, want 1 (skills=%+v)", ev.WaitingForSecondDay, ev.Skills)
	}
	otherEv, err := s.LearnerEvidence(ctx, other, testLearnerQuery())
	if err != nil {
		t.Fatal(err)
	}
	if otherEv.WaitingForSecondDay != 1 {
		t.Fatalf("other WaitingForSecondDay = %d, want 1", otherEv.WaitingForSecondDay)
	}
}

func TestLearnerEvidenceIgnoresOtherLearnerRows(t *testing.T) {
	when := time.Date(2026, 11, 8, 15, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	a := insertUser(t, s, "iso-a@example.com", "sub-iso-a", "learner")
	b := insertUser(t, s, "iso-b@example.com", "sub-iso-b", "learner")
	lex := mustLexemeText(t, s, "בִּלְעָדַי")

	seedStrictReadCard(t, s, b, lex, when)

	ev, err := s.LearnerEvidence(ctx, a, testLearnerQuery())
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.Skills) != 0 || ev.WaitingForSecondDay != 0 {
		t.Fatalf("learner A saw B's evidence: %+v", ev)
	}
	ids, err := s.KnownLexemeIDs(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("learner A KnownLexemeIDs = %v, want empty", ids)
	}
}
