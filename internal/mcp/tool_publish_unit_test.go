package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPublishUnitBlocksAndRepublish(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	mem.AddLexeme(Lexeme{TextNFC: coffee, Bare: "קפה", Gloss: "coffee", POS: "noun", Gender: "m"})
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	}), testToken)

	bad := helloDoc()
	bad.Title = ""
	draft := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: bad})
	blocked, res := callResult[publishOutput](t, session, "publish_unit", publishInput{UnitID: draft.UnitID})
	if !res.IsError {
		t.Fatal("invalid unit published")
	}
	text := resultText(res)
	if !strings.Contains(text, units.RuleSchemaRequired) || !strings.Contains(text, "/title") {
		t.Fatalf("block text %s", text)
	}
	if !hasRule(blocked.Issues, units.RuleSchemaRequired) || blocked.Status == "published" || blocked.Unchanged {
		t.Fatalf("blocked %+v", blocked)
	}
	for _, issue := range blocked.Issues {
		if issue.Rule == units.RuleSchemaRequired && issue.Pointer != "/title" {
			t.Fatalf("pointer %+v", issue)
		}
	}
	if mem.Status(draft.UnitID) != "draft" {
		t.Fatalf("status %s", mem.Status(draft.UnitID))
	}
	same := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, UnitID: &draft.UnitID, Position: 0, Doc: bad})
	if !same.Unchanged || same.Revision != draft.Revision || same.Published {
		t.Fatalf("publish wrote %+v", same)
	}

	doc := helloDoc()
	valid := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 1, Doc: doc})
	coffeeDoc := doc
	coffeeDoc.Title = "Coffee"
	coffeeDoc.NewWords = []string{coffee}
	coffeeDoc.Lessons = []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{{Type: units.ActivityRecall, Text: coffee}}}}
	held := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: coffeeDoc})
	if mem.Status(held.UnitID) != "draft" {
		t.Fatalf("coffee status %s", mem.Status(held.UnitID))
	}
	before, err := mem.KnownLexemes(t.Context(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if before[hello] || before[coffee] {
		t.Fatalf("drafts counted %+v", before)
	}
	ctx := getContext(t, session, 2)
	if containsWord(ctx.KnownLexemes, hello) || containsWord(ctx.KnownLexemes, coffee) {
		t.Fatalf("context drafts %+v", ctx.KnownLexemes)
	}

	first := call[publishOutput](t, session, "publish_unit", publishInput{UnitID: valid.UnitID})
	if first.Unchanged || first.Revision != valid.Revision || first.Status != "published" || first.UnitID != valid.UnitID {
		t.Fatalf("publish %+v", first)
	}
	if !hasRule(first.Issues, units.RuleBudgetLessonFew) || hasBlock(first.Issues) {
		t.Fatalf("warnings %+v", first.Issues)
	}
	if mem.Status(valid.UnitID) != "published" {
		t.Fatalf("status %s", mem.Status(valid.UnitID))
	}
	second := call[publishOutput](t, session, "publish_unit", publishInput{UnitID: valid.UnitID, Revision: &first.Revision})
	if !second.Unchanged || second.Revision != first.Revision || second.Status != "published" {
		t.Fatalf("republish %+v", second)
	}
	after, err := mem.KnownLexemes(t.Context(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if !after[hello] || after[coffee] {
		t.Fatalf("known %+v", after)
	}
	ctx = getContext(t, session, 2)
	if !containsWord(ctx.KnownLexemes, hello) || containsWord(ctx.KnownLexemes, coffee) {
		t.Fatalf("context published %+v", ctx.KnownLexemes)
	}

	changed := doc
	changed.Title = "Hello again"
	edited := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, UnitID: &valid.UnitID, Position: 1, Doc: changed})
	if edited.Unchanged || edited.Revision != 2 || mem.Status(valid.UnitID) != "draft" {
		t.Fatalf("edit %+v status %s", edited, mem.Status(valid.UnitID))
	}
	restored := call[publishOutput](t, session, "publish_unit", publishInput{UnitID: valid.UnitID, Revision: &first.Revision})
	if restored.Unchanged || restored.Revision != 1 || restored.Status != "published" {
		t.Fatalf("restore %+v", restored)
	}
	rows := mustList(t, mem)
	var title string
	for _, row := range rows {
		if row.ID == valid.UnitID {
			title = row.Title
		}
	}
	if title != doc.Title {
		t.Fatalf("title %q", title)
	}
	later := changed
	later.Title = "Hello later"
	next := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, UnitID: &valid.UnitID, Position: 1, Doc: later})
	if next.Unchanged || next.Revision != 3 || mem.Status(valid.UnitID) != "draft" {
		t.Fatalf("after rewind %+v status %s", next, mem.Status(valid.UnitID))
	}
}

func TestPutUnitPublishFlag(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	}), testToken)

	learner := call[putOutput](t, session, "put_unit", putInput{Scope: "learner:7", Position: 0, Doc: helloDoc(), Publish: true})
	if !learner.Published || learner.Unchanged || mem.Status(learner.UnitID) != "published" {
		t.Fatalf("learner publish %+v status %s", learner, mem.Status(learner.UnitID))
	}
	if !hasRule(learner.Issues, units.RuleBudgetLessonFew) || hasBlock(learner.Issues) {
		t.Fatalf("learner warnings %+v", learner.Issues)
	}
	again := call[putOutput](t, session, "put_unit", putInput{Scope: "learner:7", UnitID: &learner.UnitID, Position: 0, Doc: helloDoc(), Publish: true})
	if !again.Published || !again.Unchanged || again.Revision != learner.Revision || mem.Status(learner.UnitID) != "published" {
		t.Fatalf("learner republish %+v status %s", again, mem.Status(learner.UnitID))
	}

	shared := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 1, Doc: helloDoc(), Publish: true})
	if shared.Published || mem.Status(shared.UnitID) != "draft" {
		t.Fatalf("shared publish %+v status %s", shared, mem.Status(shared.UnitID))
	}

	blockedDoc := helloDoc()
	blockedDoc.Title = ""
	blocked := call[putOutput](t, session, "put_unit", putInput{Scope: "learner:7", Position: 2, Doc: blockedDoc, Publish: true})
	if blocked.Published || mem.Status(blocked.UnitID) != "draft" || !hasRule(blocked.Issues, units.RuleSchemaRequired) {
		t.Fatalf("blocked learner %+v status %s", blocked, mem.Status(blocked.UnitID))
	}
}

func TestPublishUnitPinsValidatedRevision(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	writer := &publishAfterSave{UnitWriter: mem}
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     writer,
	}), testToken)

	valid := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: helloDoc()})
	writer.inject = func() {
		bad := helloDoc()
		bad.Title = ""
		bad.Position = 0
		id := valid.UnitID
		if _, err := mem.PutUnit(t.Context(), &id, &bad, ScopeShared); err != nil {
			t.Errorf("inject save: %v", err)
		}
	}
	published := call[publishOutput](t, session, "publish_unit", publishInput{UnitID: valid.UnitID})
	if published.Revision != valid.Revision || published.Status != "published" || published.Unchanged {
		t.Fatalf("published %+v", published)
	}
	current := mem.byID(valid.UnitID)
	if current == nil || current.status != "published" || current.rev != valid.Revision || current.doc.Title != "Hello" {
		t.Fatalf("current rev %d title %q status %s", current.rev, current.doc.Title, current.status)
	}
}

func TestPublishUnitOmittedRevisionDoesNotPanic(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     missingRevision{UnitWriter: mem},
	}), testToken)
	draft := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: helloDoc()})
	msg := callErr(t, session, "publish_unit", publishInput{UnitID: draft.UnitID})
	if !strings.Contains(msg, "no revision 1") || mem.Status(draft.UnitID) != "draft" {
		t.Fatalf("omitted revision %q status %s", msg, mem.Status(draft.UnitID))
	}
}

// publishAfterSave writes a new draft between validation and publish.
type publishAfterSave struct {
	UnitWriter
	inject func()
}

func (w *publishAfterSave) PublishUnit(ctx context.Context, id int64, revision *int) (PutResult, error) {
	if w.inject != nil {
		w.inject()
		w.inject = nil
	}
	return w.UnitWriter.PublishUnit(ctx, id, revision)
}

// missingRevision makes PublishUnit report a missing revision. LoadUnit
// still returns the current row, which is the omitted-revision path.
type missingRevision struct{ UnitWriter }

func (missingRevision) PublishUnit(context.Context, int64, *int) (PutResult, error) {
	return PutResult{}, ErrRevisionNotFound
}

func TestPublishUnitDeniesLearnerAndAnonymous(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	h := Handler(Options{
		Validator: roleTokens{},
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	})
	admin := connectMCP(t, h, "admin")
	draft := call[putOutput](t, admin, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: helloDoc()})
	learner := connectMCP(t, h, "learner")
	msg := callErr(t, learner, "publish_unit", publishInput{UnitID: draft.UnitID})
	if !strings.Contains(msg, msgAdminOnly) || mem.Status(draft.UnitID) != "draft" {
		t.Fatalf("learner %q status %s", msg, mem.Status(draft.UnitID))
	}

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	anon := postTool(t, srv.URL, "", "publish_unit", publishInput{UnitID: draft.UnitID}, nil)
	body := readBody(t, anon)
	if anon.StatusCode != http.StatusUnauthorized || body != "Unauthorized" || mem.Status(draft.UnitID) != "draft" {
		t.Fatalf("anonymous %d %q status %s", anon.StatusCode, body, mem.Status(draft.UnitID))
	}
}

func hasBlock(issues []units.Issue) bool {
	for _, issue := range issues {
		if issue.Severity == units.SeverityBlock {
			return true
		}
	}
	return false
}

func containsWord(words []string, word string) bool {
	for _, w := range words {
		if w == word {
			return true
		}
	}
	return false
}

func callResult[T any](t *testing.T, session *mcp.ClientSession, name string, args any) (T, *mcp.CallToolResult) {
	t.Helper()
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out, res
}
