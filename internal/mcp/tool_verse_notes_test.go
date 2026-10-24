package mcp_test

import (
	"context"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestVerseNotesStayDraftAndLearnersSeeCheckedOnly(t *testing.T) {
	e := newAP(t)
	admin := e.as("static", nil)
	learner := e.as("learner-a", nil)
	ctx := context.Background()
	word := units.Normalize("\u05e7\u05b8\u05d3\u05d5\u05b9\u05e9\u05c1")
	if _, err := e.store.ExecContext(ctx, `
		INSERT INTO tanakh_verses (book, chapter, verse, he, he_plain, en, source_version, license)
		VALUES ('isaiah', 6, 1, ?, ?, 'Holy.', 'test', 'Public Domain')`, word, units.Plain(word)); err != nil {
		t.Fatal(err)
	}

	if msg := mustFail(t, learner, "upsert_verse_notes", map[string]any{
		"chapter": 6, "verse": 1, "notes": []any{map[string]any{"layer": "roots", "body": "A root."}},
	}); !strings.Contains(msg, "admin") {
		t.Fatalf("learner write = %s", msg)
	}

	tools, err := admin.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "check_verse" || strings.HasPrefix(tool.Name, "check_verse") {
			t.Fatalf("an MCP tool can check a verse: %s", tool.Name)
		}
	}

	mustOK(t, admin, "upsert_verse_notes", map[string]any{
		"chapter": 6,
		"verse":   1,
		"notes": []any{map[string]any{
			"layer": "roots", "body": "The root is set apart.", "status": "checked",
		}},
		"glosses": []any{map[string]any{
			"position": 0, "wordHe": word, "glossEn": "holy", "root": "\u05e7\u05d3\u05e9", "status": "checked",
		}},
	})
	if e.count(`SELECT COUNT(*) FROM verse_notes WHERE status = 'draft'`) != 1 {
		t.Fatal("a note that said checked was not stored as a draft")
	}
	if e.count(`SELECT COUNT(*) FROM verse_word_glosses WHERE status = 'draft'`) != 1 {
		t.Fatal("a gloss that said checked was not stored as a draft")
	}

	learnerView := mustOK(t, learner, "get_verse", map[string]any{"chapter": 6, "verse": 1})
	if strings.Contains(learnerView, "set apart") || strings.Contains(learnerView, "holy") {
		t.Fatalf("a learner saw a draft: %s", learnerView)
	}
	if !strings.Contains(learnerView, word) || !strings.Contains(learnerView, "Holy.") {
		t.Fatalf("verse text = %s", learnerView)
	}

	if err := e.store.CheckVerseNote(ctx, "isaiah", 6, 1, db.LayerRoots, e.a); err != nil {
		t.Fatal(err)
	}
	adminView := mustOK(t, admin, "get_verse", map[string]any{"chapter": 6, "verse": 1})
	if !strings.Contains(adminView, "set apart") || !strings.Contains(adminView, `"status":"draft"`) {
		t.Fatalf("admin view = %s", adminView)
	}
	learnerView = mustOK(t, learner, "get_verse", map[string]any{"chapter": 6, "verse": 1})
	if !strings.Contains(learnerView, "set apart") || strings.Contains(learnerView, "holy") || strings.Contains(learnerView, "draft") {
		t.Fatalf("learner after the check = %s", learnerView)
	}
}
