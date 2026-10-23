package mcp_test

import (
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestMistakeToolsScopeAndVerbBatch(t *testing.T) {
	e := newAP(t)
	learner := e.as("learner-a", nil)
	other := e.as("learner-b", nil)
	admin := e.as("static", nil)
	wrong := units.Normalize("\u05d0\u05b2\u05e0\u05b4\u05d9 \u05d4\u05d5\u05b9\u05dc\u05b5\u05da\u05b0")
	right := units.Normalize("\u05d0\u05b2\u05e0\u05b4\u05d9 \u05d4\u05d5\u05b9\u05dc\u05b6\u05db\u05b6\u05ea")

	for _, name := range []string{"upsert_common_mistakes", "upsert_verb_forms"} {
		msg := mustFail(t, learner, name, map[string]any{})
		if !strings.Contains(msg, "admin") {
			t.Fatalf("%s learner = %s", name, msg)
		}
	}
	if msg := mustFail(t, admin, "record_mistakes", map[string]any{"mistakes": []any{
		map[string]any{"wrong": wrong, "right": right},
	}}); !strings.Contains(msg, "learnerId") {
		t.Fatalf("admin without a learner = %s", msg)
	}
	if msg := mustFail(t, learner, "record_mistakes", map[string]any{
		"learnerId": e.b, "mistakes": []any{map[string]any{"wrong": wrong, "right": right}},
	}); !strings.Contains(msg, "your own") {
		t.Fatalf("other learner = %s", msg)
	}

	mustOK(t, learner, "record_mistakes", map[string]any{"mistakes": []any{
		map[string]any{"wrong": wrong, "right": right, "note": "agreement"},
		map[string]any{"wrong": wrong, "right": right},
	}})
	mustOK(t, other, "record_mistakes", map[string]any{"mistakes": []any{
		map[string]any{"wrong": "secret-wrong", "right": "secret-right"},
	}})
	own := mustOK(t, learner, "get_learner_model", map[string]any{})
	if !strings.Contains(own, right) || strings.Contains(own, "secret-wrong") {
		t.Fatalf("model leaked or dropped a mistake: %s", own)
	}
	if !strings.Contains(own, `"times":2`) {
		t.Fatalf("repeat did not increment: %s", own)
	}
	theirs := mustOK(t, other, "get_learner_model", map[string]any{})
	if strings.Contains(theirs, right) || !strings.Contains(theirs, "secret-right") {
		t.Fatalf("the other model = %s", theirs)
	}
	listed := mustOK(t, learner, "list_patterns", map[string]any{})
	if strings.Contains(listed, wrong) || strings.Contains(listed, "secret-wrong") {
		t.Fatalf("shared list contains a personal mistake: %s", listed)
	}

	long := strings.Repeat("א", 201)
	if msg := mustFail(t, learner, "record_mistakes", map[string]any{"mistakes": []any{
		map[string]any{"wrong": long, "right": right},
	}}); e.count(`SELECT COUNT(*) FROM learner_mistakes WHERE learner_id = ?`, e.a) != 1 {
		t.Fatalf("an over-long form wrote a row: %s", msg)
	}

	brief := mustOK(t, admin, "get_lesson_brief", map[string]any{"learnerId": e.a})
	if !strings.Contains(brief, `"fixMistakes"`) || !strings.Contains(brief, right) || strings.Contains(brief, "secret-wrong") {
		t.Fatalf("brief = %s", brief)
	}

	lemma := e.ids[apShalom]
	if errText := mustFail(t, admin, "upsert_verb_forms", map[string]any{"forms": []any{
		map[string]any{"lemmaId": lemma, "tense": "infinitive", "formHe": right},
		map[string]any{"lemmaId": 99999, "tense": "infinitive", "formHe": wrong},
	}}); e.count(`SELECT COUNT(*) FROM verb_forms`) != 0 || !strings.Contains(errText, "nothing was written") {
		t.Fatalf("missing lemma wrote rows: %s", errText)
	}
	mustOK(t, admin, "upsert_verb_forms", map[string]any{"forms": []any{
		map[string]any{"lemmaId": lemma, "tense": "present", "person": "1", "number": "sg", "gender": "m", "formHe": right, "translit": "holekhet"},
	}})
	if e.count(`SELECT COUNT(*) FROM verb_forms`) != 1 {
		t.Fatal("the valid form was not stored")
	}
	mustOK(t, admin, "upsert_common_mistakes", map[string]any{"mistakes": []any{
		map[string]any{"wrong": wrong, "right": right, "explain": "Agreement.", "status": "published"},
	}})
	if e.count(`SELECT COUNT(*) FROM common_mistakes`) != 1 {
		t.Fatal("shared mistake was not stored")
	}
	if e.count(`SELECT COUNT(*) FROM pragma_table_info('common_mistakes') WHERE name LIKE '%learner%'`) != 0 {
		t.Fatal("common_mistakes has a learner column")
	}
}
