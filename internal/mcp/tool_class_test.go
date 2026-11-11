package mcp_test

import (
	"strings"
	"testing"
)

func TestImportClassSheetAndStoryChecks(t *testing.T) {
	e := taughtAP(t)
	learner := e.asTaught("learner-a")
	raw := mustOK(t, learner, "import_class_sheet", map[string]any{
		"ref": "class-1", "classOn": "2026-11-01", "kind": "class", "title": "Morning",
		"body": "tutor: שָׁלוֹם || peace\n",
	})
	if !strings.Contains(raw, `"passed":true`) {
		t.Fatal(raw)
	}
	again := mustOK(t, learner, "import_class_sheet", map[string]any{
		"ref": "class-1", "classOn": "2026-11-01", "kind": "class", "title": "Morning",
		"body": "tutor: שָׁלוֹם || peace\n",
	})
	if !strings.Contains(again, `"unchanged":true`) {
		t.Fatal(again)
	}
	notes := mustOK(t, learner, "import_class_sheet", map[string]any{
		"ref": "notes", "classOn": "2026-11-01", "kind": "notes", "body": "tutor: שָׁלוֹם || peace",
	})
	if strings.Contains(notes, `"passed":true`) {
		t.Fatal(notes)
	}
	queue := mustOK(t, learner, "get_class_queue", map[string]any{})
	if !strings.Contains(queue, `"ready":1`) {
		t.Fatal(queue)
	}
	story := mustOK(t, learner, "put_story", map[string]any{
		"text": "hello", "known": []any{"שָׁלוֹם"}, "mustUse": []any{"מַיִם"},
	})
	for _, id := range []string{"story.no_hebrew", "story.coverage", "story.missing_word"} {
		if !strings.Contains(story, id) {
			t.Fatalf("missing %s in %s", id, story)
		}
	}
	if strings.Contains(story, `"passed":true`) {
		t.Fatal(story)
	}
}
