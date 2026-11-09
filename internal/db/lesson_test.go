package db

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// INIT-023/SPEC-002 ac-3: finishing a lesson whose listen_pick targets a new
// word with audio leaves that word with read, recall, and listen cards. A new
// word with no clip gets no listen card.
func TestEnqueueNewWordsAddsListenWhenTargetedWithAudio(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{now})
	ctx := context.Background()
	userID := insertUser(t, s, "a@example.com", "sub-a", "learner")

	const (
		withAudio = "שָׁלוֹם"
		noAudio   = "בֹּקֶר"
	)
	clip := strings.Repeat("ab", 32)
	mustExec(t, s, `INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at) VALUES (?, ?, 'v', 'm', 4, ?)`,
		clip, withAudio, FormatTime(now))
	hello := mustExec(t, s, `INSERT INTO lexemes (text_nfc, gloss_en, audio_id) VALUES (?, 'hello', ?)`, withAudio, clip)
	helloID, err := hello.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	morning := mustExec(t, s, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES (?, 'morning')`, noAudio)
	morningID, err := morning.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	doc := units.Unit{
		Title: "Greetings", NewWords: []string{withAudio, noAudio},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityListenPick, Text: withAudio, Target: withAudio},
			{Type: units.ActivityRecall, Text: noAudio},
			{Type: units.ActivityStory, Text: withAudio + " " + noAudio},
		}}},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	unitRes := mustExec(t, s, `
		INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
		VALUES (1, 'Greetings', 'street', 'greetings', 'published', 1, ?, 'shared')`, string(raw))
	unitID, err := unitRes.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill) VALUES (?, 0, ?, 'new', 'listen_pick')`, unitID, helloID)
	mustExec(t, s, `INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill) VALUES (?, 0, ?, 'new', 'recall')`, unitID, morningID)

	run, err := s.StartPlayerRun(ctx, userID, unitID, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Three activities: continue through each to finish and enqueue.
	for i, total := 0, 3; i < total; i++ {
		if _, _, err := s.ContinuePlayerRun(ctx, userID, run.ID, i, total); err != nil {
			t.Fatalf("continue %d: %v", i, err)
		}
	}

	skills := func(lexemeID int64) map[string]bool {
		t.Helper()
		rows, err := s.conn.QueryContext(ctx, `SELECT skill FROM cards WHERE user_id = ? AND lexeme_id = ?`, userID, lexemeID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		got := map[string]bool{}
		for rows.Next() {
			var skill string
			if err := rows.Scan(&skill); err != nil {
				t.Fatal(err)
			}
			got[skill] = true
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return got
	}

	helloSkills := skills(helloID)
	if !helloSkills["read"] || !helloSkills["recall"] || !helloSkills["listen"] || len(helloSkills) != 3 {
		t.Fatalf("word with audio = %v, want read, recall, listen", helloSkills)
	}
	morningSkills := skills(morningID)
	if !morningSkills["read"] || !morningSkills["recall"] || morningSkills["listen"] || len(morningSkills) != 2 {
		t.Fatalf("word with no audio = %v, want read and recall only", morningSkills)
	}
}
