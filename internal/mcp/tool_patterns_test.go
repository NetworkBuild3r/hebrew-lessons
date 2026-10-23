package mcp_test

import (
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestPatternToolsAdminLearnerAndPath(t *testing.T) {
	e := newAP(t)
	learner := e.as("learner-a", nil)
	admin := e.as("static", nil)
	for _, name := range []string{"upsert_patterns", "upsert_explainer", "check_path_order"} {
		msg := mustFail(t, learner, name, map[string]any{})
		if !strings.Contains(msg, "admin") {
			t.Fatalf("%s learner = %s", name, msg)
		}
	}
	listed := mustOK(t, learner, "list_patterns", map[string]any{})
	if !strings.Contains(listed, `"patterns"`) {
		t.Fatalf("learner list = %s", listed)
	}

	cycled := mustFail(t, admin, "upsert_patterns", map[string]any{"patterns": []any{
		map[string]any{"slug": "one", "title": "One", "summary": "The first pattern.", "prereqs": []string{"two"}},
		map[string]any{"slug": "two", "title": "Two", "summary": "The second pattern.", "prereqs": []string{"one"}},
	}})
	if !strings.Contains(cycled, "cycle") || e.count(`SELECT COUNT(*) FROM grammar_patterns`) != 0 {
		t.Fatalf("cycle %s rows %d", cycled, e.count(`SELECT COUNT(*) FROM grammar_patterns`))
	}

	before := e.audits()
	written := decodeInto[struct {
		Written int `json:"written"`
	}](t, mustOK(t, admin, "upsert_patterns", map[string]any{"patterns": []any{
		map[string]any{"slug": "present-tense", "title": "Present tense", "summary": "The verb agrees with the subject."},
		map[string]any{"slug": "past-tense", "title": "Past tense", "summary": "The past adds an ending.", "prereqs": []string{"present-tense"}},
		map[string]any{"slug": "going-to", "title": "Going to", "summary": "Going to plus a verb.", "prereqs": []string{"present-tense"}},
	}}))
	if written.Written != 3 || e.audits() != before+1 || e.text(`SELECT actor FROM publish_audit WHERE action = 'upsert_patterns'`) != "mcp" {
		t.Fatalf("written %+v audits %d", written, e.audits())
	}

	for _, body := range []string{"<script>alert(1)</script>", "See ![x](a.png)", "Go [out](https://example.com/x)"} {
		msg := mustFail(t, admin, "upsert_explainer", map[string]any{
			"slug": "present-tense", "title": "Present tense", "status": "published", "body": body,
		})
		if e.count(`SELECT COUNT(*) FROM explainers`) != 0 {
			t.Fatalf("refused body %q wrote a row: %s", body, msg)
		}
	}
	mustOK(t, admin, "upsert_explainer", map[string]any{
		"slug": "present-tense", "title": "Present tense", "status": "published",
		"body": "## Agreement\n\nThe verb **agrees**.\n\n- {he:אֲנִי|ani}",
	})
	if e.count(`SELECT COUNT(*) FROM explainers WHERE status = 'published'`) != 1 {
		t.Fatal("published explainer was not stored")
	}
	if e.text(`SELECT actor FROM publish_audit WHERE action = 'upsert_explainer'`) != "mcp" {
		t.Fatal("explainer write was not audited as the token")
	}

	present := apDoc("Present last")
	past := apDoc("Past first")
	going := apDoc("Going next")
	filler := apDoc("Filler")
	presentID := decodeInto[apPut](t, mustOK(t, admin, "put_unit", map[string]any{
		"scope": mcp.ScopeShared, "position": 4, "doc": present, "pattern": "present-tense",
	})).UnitID
	pastID := decodeInto[apPut](t, mustOK(t, admin, "put_unit", map[string]any{
		"scope": mcp.ScopeShared, "position": 1, "doc": past, "pattern": "past-tense",
	})).UnitID
	goingID := decodeInto[apPut](t, mustOK(t, admin, "put_unit", map[string]any{
		"scope": mcp.ScopeShared, "position": 2, "doc": going, "pattern": "going-to",
	})).UnitID
	fillerID := decodeInto[apPut](t, mustOK(t, admin, "put_unit", map[string]any{
		"scope": mcp.ScopeShared, "position": 3, "doc": filler,
	})).UnitID

	checked := decodeInto[struct {
		Issues []units.Issue `json:"issues"`
	}](t, mustOK(t, admin, "validate_unit", map[string]any{
		"position": 1, "doc": past, "pattern": "past-tense", "scope": mcp.ScopeShared,
	}))
	if !issueMentions(checked.Issues, units.RulePatternOrder) || !issueMentionsText(checked.Issues, "Present last") {
		t.Fatalf("validate %+v", checked.Issues)
	}

	order := decodeInto[struct {
		Issues []units.Issue `json:"issues"`
		Order  []int64       `json:"order"`
	}](t, mustOK(t, admin, "check_path_order", map[string]any{}))
	if len(order.Issues) != 2 || !issueMentionsText(order.Issues, "Present last") {
		t.Fatalf("path issues %+v", order.Issues)
	}
	want := []int64{fillerID, presentID, pastID, goingID}
	if len(order.Order) != len(want) {
		t.Fatalf("order %v want %v", order.Order, want)
	}
	for i := range want {
		if order.Order[i] != want[i] {
			t.Fatalf("order %v want %v", order.Order, want)
		}
	}

	shaped := decodeInto[struct {
		Issues []units.Issue `json:"issues"`
	}](t, mustOK(t, admin, "validate_unit", map[string]any{
		"position": 0, "doc": units.Unit{
			Title: "Story", Situation: "A cafe", Concept: "Greetings",
			Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
				{Type: units.ActivityRecall, Text: apShalom},
			}}},
		}, "worksheetType": units.SheetStory,
	}))
	if !issueMentions(shaped.Issues, units.RuleWorksheetShape) {
		t.Fatalf("shape %+v", shaped.Issues)
	}

	review := apDoc("Review hour")
	review.NewWords = nil
	saved := decodeInto[apPut](t, mustOK(t, admin, "put_unit", map[string]any{
		"scope": mcp.ScopeShared, "position": 8, "doc": review, "worksheetType": units.SheetReview,
	}))
	e.exec(`UPDATE units SET status = 'published' WHERE id = ?`, saved.UnitID)
	got := decodeInto[struct {
		WorksheetType string `json:"worksheetType"`
	}](t, mustOK(t, admin, "get_unit", map[string]any{"unitId": saved.UnitID}))
	if got.WorksheetType != units.SheetReview {
		t.Fatalf("worksheetType %q", got.WorksheetType)
	}

	ctx := mustOK(t, admin, "get_context", map[string]any{"position": 0})
	for _, uri := range []string{
		"hebrew://templates/topic-conversation",
		"hebrew://templates/review-sheet",
		"hebrew://templates/verb-workshop",
		"hebrew://templates/short-story",
		"hebrew://templates/foundations-reading",
	} {
		if !strings.Contains(ctx, uri) {
			t.Fatalf("get_context lacks %s", uri)
		}
		body := resRead(t, admin, uri)
		lower := strings.ToLower(body)
		for _, banned := range []string{"aaron", "brian", "nelson"} {
			if strings.Contains(lower, banned) {
				t.Fatalf("%s names %s", uri, banned)
			}
		}
	}
}

func issueMentions(issues []units.Issue, rule string) bool {
	for _, issue := range issues {
		if issue.Rule == rule {
			return true
		}
	}
	return false
}

func issueMentionsText(issues []units.Issue, text string) bool {
	for _, issue := range issues {
		if strings.Contains(issue.Msg, text) {
			return true
		}
	}
	return false
}
