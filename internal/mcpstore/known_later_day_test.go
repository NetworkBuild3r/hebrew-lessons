package mcpstore

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/brief"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

// INIT-036/SPEC-003: the validator coverage set reads known/solid from the
// learner model, so a day-one word drops out until a second study day.
func TestCoverageSetDropsDayOneWordUntilSecondDay(t *testing.T) {
	ctx := context.Background()
	when := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	store, raw := openAdapter(t, fixedClock{when})
	user := mustUser(t, raw)

	lex, err := raw.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES ('מַיִם', 'water')`)
	if err != nil {
		t.Fatal(err)
	}
	lexID, err := lex.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	day1 := when.AddDate(0, 0, -1)
	cardID, err := raw.InsertCard(ctx, db.Card{
		UserID: user, LexemeID: lexID, Skill: fsrs.SkillRead,
		State: fsrs.StateReview, Due: when, Stability: 10, Difficulty: 5,
		Reps: 3, LastReview: day1,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		at := day1.Add(time.Duration(i) * time.Minute)
		if _, err := raw.ExecContext(ctx, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 0, 1)`,
			cardID, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
		if _, err := raw.ExecContext(ctx, `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
			user, lexID, fsrs.SkillRead, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}

	model := learner.New(raw, nil)
	known, err := knownLexemeIDs(ctx, model, user)
	if err != nil {
		t.Fatal(err)
	}
	if known[lexID] {
		t.Fatal("day-one word must not be in the coverage known set")
	}

	// Persist a brief so BriefFacts can return the coverage map.
	payload, err := json.Marshal(brief.Plan{
		V: brief.Version, Tier: brief.TierFor(0),
		Targets: brief.Targets{Coverage: 0.95, NewWordMinUses: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	briefID := "AAAAAAAAAAAAAAAAAAAAAA"
	if _, err := raw.ExecContext(ctx, `INSERT INTO lesson_briefs (id, user_id, theme, payload_json, created_at) VALUES (?, ?, 'general', ?, ?)`,
		briefID, user, string(payload), db.FormatTime(when.Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	facts, err := store.BriefFacts(ctx, user, briefID)
	if err != nil {
		t.Fatal(err)
	}
	if facts.KnownLexemeIDs[lexID] {
		t.Fatal("BriefFacts day-one KnownLexemeIDs must omit the word")
	}

	day2 := when.Add(-30 * time.Minute)
	if _, err := raw.ExecContext(ctx, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 1, 1)`,
		cardID, db.FormatTime(day2)); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
		user, lexID, fsrs.SkillRead, db.FormatTime(day2)); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `UPDATE cards SET reps = 4, last_review = ? WHERE id = ?`,
		db.FormatTime(day2), cardID); err != nil {
		t.Fatal(err)
	}

	known, err = knownLexemeIDs(ctx, model, user)
	if err != nil {
		t.Fatal(err)
	}
	if !known[lexID] {
		t.Fatal("day-two word must be in the coverage known set")
	}
	facts, err = store.BriefFacts(ctx, user, briefID)
	if err != nil {
		t.Fatal(err)
	}
	if !facts.KnownLexemeIDs[lexID] {
		t.Fatal("BriefFacts day-two KnownLexemeIDs must include the word")
	}
}
