package db

import (
	"context"
	"testing"
	"time"
)

func TestKnownLexemesAreRecalledOrProved(t *testing.T) {
	when := time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "ladder-known@example.com", "sub-ladder-known", "learner")
	later := mustLexemeText(t, s, "אַחַר")
	same := mustLexemeText(t, s, "אוֹתוֹ")
	placed := mustLexemeText(t, s, "מִבְחָן")

	// One right on a study day after the card was created: recalled later.
	laterCard := insertLadderCard(t, s, user, later, "recall", dayUTC(2026, 11, 1))
	insertLadderReview(t, s, laterCard, 3, dayUTC(2026, 11, 3))

	// Right only on the taught day: not recalled, not proved.
	sameCard := insertLadderCard(t, s, user, same, "recall", dayUTC(2026, 11, 1))
	insertLadderReview(t, s, sameCard, 3, dayUTC(2026, 11, 1))

	// A card only placement created is not taught.
	placeCard := insertLadderCard(t, s, user, placed, "recall", dayUTC(2026, 11, 1))
	insertLadderReview(t, s, placeCard, 3, dayUTC(2026, 11, 3))
	mustExec(t, s, `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, 'recall', 'test', 'placement', '', '', 1, 0, 0, 0, ?)`,
		user, placed, FormatTime(dayUTC(2026, 11, 3)))

	ids, err := s.KnownLexemeIDs(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != later {
		t.Fatalf("known = %v, want [%d]", ids, later)
	}
}

func insertLadderCard(t *testing.T, s *Store, user, lex int64, skill string, created time.Time) int64 {
	t.Helper()
	id, err := s.InsertCard(context.Background(), Card{
		UserID: user, LexemeID: lex, Skill: skill, State: "review",
		Due: created, Stability: 10, Difficulty: 5, CreatedAt: created,
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertLadderReview(t *testing.T, s *Store, cardID int64, rating int, at time.Time) {
	t.Helper()
	mustExec(t, s, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, ?, ?, 1, 1)`,
		cardID, rating, FormatTime(at))
}

func dayUTC(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 18, 0, 0, 0, time.UTC)
}
