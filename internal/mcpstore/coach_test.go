package mcpstore

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

func TestCoachAdapterUsesClockAndNulls(t *testing.T) {
	when := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store, raw := openAdapter(t, fixedClock{when})
	ctx := context.Background()
	user := mustUser(t, raw)
	res, err := raw.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, pos, gloss_en) VALUES ('שָׁלוֹם', 'noun', 'hello')`)
	if err != nil {
		t.Fatal(err)
	}
	lex, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	due := db.FormatTime(when.Add(time.Hour))
	res, err = raw.ExecContext(ctx, `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, suspended)
		VALUES (?, ?, 'read', 'review', ?, 1, 2, 0, 1, 1)`, user, lex, due)
	if err != nil {
		t.Fatal(err)
	}
	cardID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 0, 0)`, cardID, db.FormatTime(when.Add(-24*time.Hour))); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 4, ?, 0, 0)`, cardID, db.FormatTime(when.Add(-8*24*time.Hour))); err != nil {
		t.Fatal(err)
	}
	unit := insertUnitRow(t, raw, "Hello", mcp.ScopeShared, "published", 0, "שָׁלוֹם")
	if _, err := raw.ExecContext(ctx, `INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score) VALUES (?, ?, 0, ?, ?, 1)`, user, unit, db.FormatTime(when.Add(-time.Hour)), db.FormatTime(when.Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}

	page, err := store.LearnerCards(ctx, mcp.CardQuery{UserID: user, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Cards) != 1 || page.NextCursor != "" || page.Cards[0].Translit != nil || page.Cards[0].LastReview != nil || page.Cards[0].LastRating == nil || *page.Cards[0].LastRating != 3 || page.Cards[0].Lapses != 1 {
		t.Fatalf("card %+v", page.Cards)
	}
	if _, err := store.LearnerCards(ctx, mcp.CardQuery{UserID: user, Limit: 10, Cursor: "nope"}); err != mcp.ErrBadCardCursor {
		t.Fatalf("cursor %v", err)
	}
	acc, err := store.RecentAccuracy(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Reviews7d.Count != 1 || acc.Reviews7d.GoodOrEasy != 1 || acc.LastReviews.Count != 2 || acc.LastReviews.GoodOrEasy != 2 || acc.Lessons7d.Finished != 1 || acc.Lessons7d.MeanScore == nil || *acc.Lessons7d.MeanScore != 1 {
		t.Fatalf("accuracy %+v", acc)
	}
}
