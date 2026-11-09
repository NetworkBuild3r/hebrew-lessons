package mcpstore

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// TestLaterDayResultsPracticeResultShare wires evidence.LaterDayResults through
// the mcpstore adapter. INIT-029/SPEC-008.
func TestLaterDayResultsPracticeResultShare(t *testing.T) {
	ctx := context.Background()
	adapter, raw := openAdapter(t, nil)
	user := mustUser(t, raw)
	loc, err := studyday.ParseZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	zone := studyday.Zone{Name: "America/Denver", Loc: loc, Source: studyday.SourceLearner}
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	day1 := time.Date(2026, 5, 10, 15, 0, 0, 0, loc)
	day2 := time.Date(2026, 5, 11, 15, 0, 0, 0, loc)

	lex, err := testharness.InsertLexeme(ctx, raw, "א")
	if err != nil {
		t.Fatal(err)
	}
	insertLaterDayAttempt(t, raw, user, lex, "cloze", "lesson", true, day1)
	insertLaterDayAttempt(t, raw, user, lex, "read_pick", "review", true, day2)

	got, err := adapter.LaterDayResults(ctx, user, now, zone)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Kind != "cloze" || got[0].Right != 1 || got[0].Total != 1 || got[0].Enough {
		t.Fatalf("got %+v", got)
	}
}

func insertLaterDayAttempt(t *testing.T, store *db.Store, user, lexeme int64, activity, source string, correct bool, at time.Time) {
	t.Helper()
	_, err := store.WriteAttempt(context.Background(), db.AttemptWrite{
		Attempt: db.Attempt{
			UserID:    user,
			LexemeID:  lexeme,
			Skill:     "read",
			Activity:  activity,
			Source:    source,
			Prompt:    "p",
			Response:  "r",
			Correct:   correct,
			LatencyMS: 1000,
			CreatedAt: at,
		},
		Step: func(prev *db.Card) (db.Card, db.ReviewLog, bool, error) {
			return db.Card{}, db.ReviewLog{}, false, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
