package mcp_test

// INIT-037/SPEC-007: get_lesson_brief refuses a second consolidation while one is queued.

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcpstore"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestLessonBriefReviewFirstWhenConsolidateQueued(t *testing.T) {
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), apClock{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	adapter := mcpstore.New(store)
	user, err := store.InsertUser(ctx, "pace@example.com", "sub-pace", "learner")
	if err != nil {
		t.Fatal(err)
	}
	e := &apEnv{t: t, store: store, adapter: adapter, ids: map[string]int64{}}
	e.session = e.connect(mcp.Options{
		AuthToken: "secret", Course: adapter, Lexicon: adapter, Lookup: adapter, Units: adapter,
	}, "secret")

	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	now := apNow
	insertLex := func(gloss string) int64 {
		t.Helper()
		res, err := conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES (?, ?)`, gloss, gloss)
		if err != nil {
			t.Fatal(err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	// Reviews so Pace has accuracy; 60 learning words so Pace consolidates.
	reviewLex := insertLex("reviewed")
	res, err := conn.ExecContext(ctx, `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended)
		VALUES (?, ?, ?, ?, ?, 10, 5, 50, 0, ?, 0)`,
		user, reviewLex, fsrs.SkillRecall, fsrs.StateReview, db.FormatTime(now), db.FormatTime(now))
	if err != nil {
		t.Fatal(err)
	}
	cardID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	for i := range 45 {
		at := now.Add(-time.Duration(i+1) * time.Minute)
		e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
			VALUES (?, ?, ?, 0, 1, 0)`, cardID, int(fsrs.Good), db.FormatTime(at))
	}
	for i := range 5 {
		at := now.Add(-time.Duration(50+i) * time.Minute)
		e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
			VALUES (?, ?, ?, 0, 1, 0)`, cardID, int(fsrs.Again), db.FormatTime(at))
	}
	for i := range 60 {
		id := insertLex(fmt.Sprintf("learning-%d", i))
		at := now.Add(-time.Hour)
		e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`, user, id, fsrs.SkillRecall, db.FormatTime(at.Add(-time.Minute)))
		e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`, user, id, fsrs.SkillRecall, db.FormatTime(at))
		e.exec(`INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended)
			VALUES (?, ?, ?, ?, ?, 2, 5, 3, 0, ?, 0)`,
			user, id, fsrs.SkillRecall, fsrs.StateReview, db.FormatTime(now), db.FormatTime(at))
		cid := int64(e.count(`SELECT id FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, id))
		for r := range 3 {
			e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
				VALUES (?, ?, ?, 0, 1, 0)`, cid, int(fsrs.Good), db.FormatTime(at.Add(time.Duration(r)*time.Minute)))
		}
	}
	dueLex := insertLex("due-now")
	e.exec(`INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended)
		VALUES (?, ?, ?, ?, ?, 2, 5, 1, 0, ?, 0)`,
		user, dueLex, fsrs.SkillRecall, fsrs.StateReview, db.FormatTime(now.Add(-time.Hour)), db.FormatTime(now.Add(-time.Hour)))

	// No queued consolidation: brief consolidates.
	brief := decodeInto[mcp.LessonBrief](t, mustOK(t, e.session, "get_lesson_brief", map[string]any{"learnerId": user}))
	if len(brief.NewWords) != 0 {
		t.Fatalf("newWords = %d, want consolidate with none", len(brief.NewWords))
	}

	doc := units.Unit{
		Title: "Consolidate", Situation: "s", Concept: "c", Position: 0, NewWords: nil,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: "אָ"},
		}}},
	}
	saved, err := adapter.PutUnit(ctx, nil, &doc, db.LearnerScope(user))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PublishUnit(ctx, saved.UnitID, nil); err != nil {
		t.Fatal(err)
	}

	before := e.count(`SELECT COUNT(*) FROM lesson_briefs WHERE user_id = ?`, user)
	fail := mustFail(t, e.session, "get_lesson_brief", map[string]any{"learnerId": user})
	if !strings.Contains(strings.ToLower(fail), "review") || !strings.Contains(fail, "due") {
		t.Fatalf("want review-first with due count, got %q", fail)
	}
	after := e.count(`SELECT COUNT(*) FROM lesson_briefs WHERE user_id = ?`, user)
	if after != before {
		t.Fatalf("review-first stored a brief: before=%d after=%d", before, after)
	}
}
