package mcp

import (
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// TestCatalogAndRulesIncludeOrder finds order in the activity catalog and the
// four new rule ids in hebrew://rules. INIT-029/SPEC-008.
func TestCatalogAndRulesIncludeOrder(t *testing.T) {
	session := connectMCP(t, Handler(Options{AuthToken: testToken}), testToken)

	catalogText := readResource(t, session, resourceActivities)
	if !strings.Contains(catalogText, `"type":"`+units.ActivityOrder+`"`) {
		t.Fatalf("catalog missing order: %s", catalogText)
	}
	for _, want := range []string{"translation is the English cue", "text is the right order", "accept lists other full orders"} {
		if !strings.Contains(catalogText, want) {
			t.Fatalf("catalog purpose missing %q: %s", want, catalogText)
		}
	}

	rulesText := readResource(t, session, resourceRules)
	for _, id := range []string{
		units.RuleUnitOrderTokens,
		units.RuleUnitOrderTranslation,
		units.RuleUnitOrderAccept,
		units.RuleBriefOrderKnown,
	} {
		if !strings.Contains(rulesText, `"id":"`+id+`"`) {
			t.Fatalf("hebrew://rules missing %s: %s", id, rulesText)
		}
	}

	guide := readResource(t, session, resourceAuthoring)
	for _, want := range []string{
		"one or two `order` steps",
		"list any other natural word order in `accept`",
		"practiceResults",
		`"not enough yet"`,
		"keep the default mix",
	} {
		if !strings.Contains(guide, want) {
			t.Fatalf("authoring guide missing %q", want)
		}
	}
}

// TestGetContextPracticeResults returns practiceResults with a learner and
// omits the field without one. INIT-029/SPEC-008.
func TestGetContextPracticeResults(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	mem.SetPracticeResults(7, []evidence.PracticeResult{
		{Kind: "cloze", Right: 18, Total: 20, Enough: true},
		{Kind: "order", Right: 3, Total: 5, Enough: false},
	})
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken, Course: mem, Lexicon: mem, Lookup: mem, Units: mem,
	}), testToken)

	sharedRaw := callRaw(t, session, "get_context", getContextInput{Position: 0})
	if strings.Contains(sharedRaw, `"practiceResults"`) {
		t.Fatalf("without learnerId practiceResults must be omitted: %s", sharedRaw)
	}

	id := int64(7)
	raw := callRaw(t, session, "get_context", getContextInput{Position: 0, LearnerID: &id})
	if !strings.Contains(raw, `"practiceResults"`) {
		t.Fatalf("with learnerId missing practiceResults: %s", raw)
	}
	out := call[getContextOutput](t, session, "get_context", getContextInput{Position: 0, LearnerID: &id})
	if len(out.PracticeResults) != 2 {
		t.Fatalf("practiceResults %+v", out.PracticeResults)
	}
	byKind := map[string]PracticeResultView{}
	for _, r := range out.PracticeResults {
		byKind[r.Kind] = r
	}
	cloze := byKind["cloze"]
	if cloze.Count != 20 || cloze.Share == nil || *cloze.Share != 0.9 || cloze.Note != "" {
		t.Fatalf("cloze %+v", cloze)
	}
	order := byKind["order"]
	if order.Count != 5 || order.Share != nil || order.Note != notEnoughYet {
		t.Fatalf("order %+v", order)
	}
}

// TestSmokeGetContextPracticeResultsOwnOnly checks a smoke-token learner can
// read only their own practiceResults. INIT-029/SPEC-008.
func TestSmokeGetContextPracticeResultsOwnOnly(t *testing.T) {
	mem, srv := readOnlyHandler(t, false)
	mem.SetPracticeResults(readOnlyLearner, []evidence.PracticeResult{
		{Kind: "cloze", Right: 20, Total: 20, Enough: true},
	})
	mem.SetPracticeResults(9, []evidence.PracticeResult{
		{Kind: "order", Right: 20, Total: 20, Enough: true},
	})

	self := readOnlyLearner
	got := postTool(t, srv.URL, readOnlySmokeToken, "get_context", getContextInput{Position: 0, LearnerID: &self}, nil)
	body := readBody(t, got)
	if got.StatusCode != 200 || strings.Contains(body, `"isError":true`) {
		t.Fatalf("self get_context status/body")
	}
	if !strings.Contains(body, `"practiceResults"`) || !strings.Contains(body, `"kind":"cloze"`) {
		t.Fatalf("self missing own practiceResults")
	}
	if strings.Contains(body, `"kind":"order"`) {
		t.Fatalf("self saw another learner's practice kind")
	}

	other := int64(9)
	denied := postTool(t, srv.URL, readOnlySmokeToken, "get_context", getContextInput{Position: 0, LearnerID: &other}, nil)
	deniedBody := assertDenied(t, denied, msgOtherLearner)
	if strings.Contains(deniedBody, `"practiceResults"`) || strings.Contains(deniedBody, `"kind":"order"`) {
		t.Fatalf("other learner practiceResults leaked")
	}

	// Omitted learnerId uses the smoke learner's own id.
	omitted := postTool(t, srv.URL, readOnlySmokeToken, "get_context", getContextInput{Position: 0}, nil)
	omittedBody := readBody(t, omitted)
	if omitted.StatusCode != 200 || strings.Contains(omittedBody, `"isError":true`) {
		t.Fatalf("omitted get_context status/body")
	}
	if !strings.Contains(omittedBody, `"kind":"cloze"`) || strings.Contains(omittedBody, `"kind":"order"`) {
		t.Fatalf("omitted learnerId must return own practiceResults only")
	}
}
