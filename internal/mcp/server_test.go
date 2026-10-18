package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	testToken = "secret"
	hello     = "שָׁלוֹם"
	helloBare = "שלום"
	coffee    = "קָפֶה"
)

func TestAuthRejectAndAccept(t *testing.T) {
	h := Handler(Options{AuthToken: testToken})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	missing := postInitialize(t, srv.URL, "")
	if missing.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d", missing.StatusCode)
	}
	if !strings.Contains(missing.Header.Get("WWW-Authenticate"), "Bearer") {
		t.Fatalf("challenge %q", missing.Header.Get("WWW-Authenticate"))
	}
	missing.Body.Close()

	wrong := postInitialize(t, srv.URL, "nope")
	if wrong.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong token status = %d", wrong.StatusCode)
	}
	wrong.Body.Close()

	// A different length must also fail. The compare hashes both sides.
	short := postInitialize(t, srv.URL, "x")
	if short.StatusCode != http.StatusUnauthorized {
		t.Fatalf("short token status = %d", short.StatusCode)
	}
	short.Body.Close()

	session := connectMCP(t, h, testToken)
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 4 {
		t.Fatalf("tools = %d", len(tools.Tools))
	}
}

func TestEmptyConfiguredTokenRejects(t *testing.T) {
	h := Handler(Options{})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	res := postInitialize(t, srv.URL, testToken)
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestAcceptHeaderReachesSDK(t *testing.T) {
	h := Handler(Options{AuthToken: testToken})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(initializeBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d body %s", res.StatusCode, body)
	}
	if !strings.Contains(string(body), "Accept") {
		t.Fatalf("body %s", body)
	}
}

func TestChainAcceptsStaticToken(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	h := Handler(Options{
		Validator: Chain{rejectToken{}, StaticToken{Token: testToken, Actor: "author-7"}},
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	})
	session := connectMCP(t, h, testToken)
	out := call[putOutput](t, session, "put_unit", putInput{Position: 0, Doc: helloDoc()})
	if mem.Actor(out.UnitID) != "author-7" {
		t.Fatalf("actor = %q", mem.Actor(out.UnitID))
	}
}

func TestToolsResourcesAndIdempotency(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	mem.AddLexeme(Lexeme{
		TextNFC:  coffee,
		Bare:     "קפה",
		Gloss:    "coffee",
		POS:      "noun",
		Gender:   "m",
		Translit: "kafe",
	})
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	}), testToken)

	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	gotNames := map[string]bool{}
	for _, tool := range tools.Tools {
		gotNames[tool.Name] = true
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), `"type":"object"`) && !strings.Contains(string(raw), `"type": "object"`) {
			t.Fatalf("%s schema %s", tool.Name, raw)
		}
		if !strings.Contains(string(raw), "additionalProperties") {
			t.Fatalf("%s schema is not closed: %s", tool.Name, raw)
		}
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint && tool.Name != "put_unit" {
			t.Fatalf("%s annotations %#v", tool.Name, tool.Annotations)
		}
	}
	for _, name := range []string{"get_context", "lookup", "validate_unit", "put_unit"} {
		if !gotNames[name] {
			t.Fatalf("missing %s in %#v", name, gotNames)
		}
	}
	if tools.Tools[indexOf(tools, "put_unit")].Annotations.ReadOnlyHint {
		t.Fatal("put_unit is marked read-only")
	}
	if !tools.Tools[indexOf(tools, "put_unit")].Annotations.IdempotentHint {
		t.Fatal("put_unit is not marked idempotent")
	}

	resources, err := session.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources.Resources) != 3 {
		t.Fatalf("resources = %d", len(resources.Resources))
	}
	schemaText := readResource(t, session, resourceUnitSchema)
	if !strings.Contains(schemaText, units.ActivityListenPick) || !strings.Contains(schemaText, "additionalProperties") {
		t.Fatalf("unit schema %s", schemaText)
	}
	catalogText := readResource(t, session, resourceActivities)
	for _, typ := range []string{
		units.ActivityListenPick, units.ActivityRecall, units.ActivityProduce,
		units.ActivityCloze, units.ActivityPattern, units.ActivityStory, units.ActivityShadow,
	} {
		if !strings.Contains(catalogText, typ) {
			t.Fatalf("catalog missing %s", typ)
		}
	}
	rulesText := readResource(t, session, resourceRules)
	if !strings.Contains(rulesText, units.RuleAudioMissing) || !strings.Contains(rulesText, `"id":"N1"`) {
		t.Fatalf("rules %s", rulesText)
	}
	if !strings.Contains(rulesText, `"maxNewWordsPerUnit":8`) {
		t.Fatalf("budgets %s", rulesText)
	}

	ctx := getContext(t, session, 0)
	if ctx.Placement.Taken {
		t.Fatal("placement was taken")
	}
	if ctx.Budgets != currentBudgets() {
		t.Fatalf("budgets %+v", ctx.Budgets)
	}
	if len(ctx.Outline) != 0 || len(ctx.KnownLexemes) != 0 {
		t.Fatalf("context %+v", ctx)
	}

	hit := call[lookupOutput](t, session, "lookup", lookupInput{Query: hello})
	if len(hit.Hits) != 1 || hit.Hits[0].Gloss != "hello; peace" || hit.Hits[0].Translit != "shalom" || !hit.Hits[0].AudioPresent {
		t.Fatalf("pointed hit %+v", hit)
	}
	bareHit := call[lookupOutput](t, session, "lookup", lookupInput{Query: helloBare})
	if len(bareHit.Hits) != 1 || bareHit.Hits[0].TextNFC != hello {
		t.Fatalf("bare hit %+v", bareHit)
	}
	glossHit := call[lookupOutput](t, session, "lookup", lookupInput{Query: "coffee"})
	if len(glossHit.Hits) != 1 || glossHit.Hits[0].POS != "noun" || glossHit.Hits[0].Gender != "m" {
		t.Fatalf("gloss hit %+v", glossHit)
	}
	miss := call[lookupOutput](t, session, "lookup", lookupInput{Query: "coff"})
	if len(miss.Hits) != 0 || len(miss.Suggestions) != 1 || miss.Suggestions[0].TextNFC != coffee {
		t.Fatalf("suggestions %+v", miss)
	}
	if msg := callErr(t, session, "lookup", lookupInput{}); !strings.Contains(msg, "Query is empty") {
		t.Fatalf("empty query %q", msg)
	}

	doc := helloDoc()
	want := units.Validate(t.Context(), &doc, mem)
	dry := call[validateOutput](t, session, "validate_unit", validateInput{Position: 0, Doc: doc})
	if !sameIssues(dry.Issues, want) {
		t.Fatalf("validate issues %#v\nwant %#v", dry.Issues, want)
	}
	blocks, warns := countIssues(want)
	if dry.OK != (blocks == 0) || dry.Blocks != blocks || dry.Warns != warns {
		t.Fatalf("summary %+v blocks %d warns %d", dry, blocks, warns)
	}
	if len(mustList(t, mem)) != 0 {
		t.Fatal("validate_unit wrote a unit")
	}

	first := call[putOutput](t, session, "put_unit", putInput{Position: 0, Doc: doc})
	if first.UnitID == 0 || first.Revision != 1 || first.Unchanged {
		t.Fatalf("first put %+v", first)
	}
	if !sameIssues(first.Issues, want) {
		t.Fatalf("put issues %#v", first.Issues)
	}
	if mem.Actor(first.UnitID) != defaultActor {
		t.Fatalf("actor %q", mem.Actor(first.UnitID))
	}
	second := call[putOutput](t, session, "put_unit", putInput{UnitID: &first.UnitID, Position: 0, Doc: doc})
	if second.Revision != 1 || !second.Unchanged || second.UnitID != first.UnitID {
		t.Fatalf("idempotent put %+v", second)
	}
	moved := doc
	moved.Title = "Hello again"
	third := call[putOutput](t, session, "put_unit", putInput{UnitID: &first.UnitID, Position: 0, Doc: moved})
	if third.Unchanged || third.Revision != 2 {
		t.Fatalf("changed put %+v", third)
	}
	shifted := call[putOutput](t, session, "put_unit", putInput{UnitID: &first.UnitID, Position: 2, Doc: moved})
	if shifted.Unchanged || shifted.Revision != 3 {
		t.Fatalf("position change %+v", shifted)
	}

	bad := doc
	bad.Title = ""
	saved := call[putOutput](t, session, "put_unit", putInput{Position: 1, Doc: bad})
	if saved.Unchanged || saved.Revision != 1 {
		t.Fatalf("blocked draft %+v", saved)
	}
	if !hasRule(saved.Issues, units.RuleSchemaRequired) {
		t.Fatalf("blocked draft issues %#v", saved.Issues)
	}

	missing := int64(99)
	if msg := callErr(t, session, "put_unit", putInput{UnitID: &missing, Position: 0, Doc: doc}); !strings.Contains(msg, "No unit has id 99") {
		t.Fatalf("missing unit %q", msg)
	}
	before := len(mustList(t, mem))
	if msg := callErr(t, session, "put_unit", map[string]any{"position": 0, "doc": "not-a-unit"}); !strings.Contains(msg, `want "object"`) {
		t.Fatalf("malformed %q", msg)
	}
	if msg := callErr(t, session, "put_unit", map[string]any{"position": 0, "doc": map[string]any{"nope": true}}); !strings.Contains(msg, "nope") {
		t.Fatalf("unknown field %q", msg)
	}
	if msg := callErr(t, session, "put_unit", map[string]any{
		"position": 0,
		"doc":      map[string]any{"title": 1, "situation": "x", "concept": "y", "newWords": []any{}, "lessons": []any{}},
	}); !strings.Contains(msg, "title") {
		t.Fatalf("wrong title type %q", msg)
	}
	if len(mustList(t, mem)) != before {
		t.Fatal("malformed put wrote a unit")
	}
	if msg := callErr(t, session, "validate_unit", validateInput{Position: -1, Doc: doc}); !strings.Contains(msg, "Position must be 0 or greater") {
		t.Fatalf("negative %q", msg)
	}

	mem.SetPlacement(Placement{
		Taken:                  true,
		TakenAt:                "2026-10-18T12:00:00Z",
		EstimatedKnownWords:    40,
		AdjustedScore:          0.25,
		FalseAlarmRate:         0.05,
		SuggestedStartPosition: 0,
		KnownLexemes:           []string{hello},
	})
	again := getContext(t, session, 3)
	if !again.Placement.Taken || again.Placement.SuggestedStartPosition != 0 || again.Placement.EstimatedKnownWords != 40 {
		t.Fatalf("placement %+v", again.Placement)
	}
	if len(again.Placement.KnownLexemes) != 1 || again.Placement.KnownLexemes[0] != hello {
		t.Fatalf("placement lexemes %+v", again.Placement.KnownLexemes)
	}
	found := false
	for _, word := range again.KnownLexemes {
		if word == hello {
			found = true
		}
	}
	if !found {
		t.Fatalf("known %+v", again.KnownLexemes)
	}
	var sawHello bool
	for _, row := range again.Outline {
		if row.Title == "Hello again" && row.Position == 2 && row.Status == "draft" {
			sawHello = true
		}
	}
	if !sawHello {
		t.Fatalf("outline %+v", again.Outline)
	}
}

func TestNilStoresAreToolErrors(t *testing.T) {
	session := connectMCP(t, Handler(Options{AuthToken: testToken}), testToken)
	if msg := callErr(t, session, "get_context", getContextInput{}); !strings.Contains(msg, "not connected") {
		t.Fatalf("get_context %q", msg)
	}
	if msg := callErr(t, session, "put_unit", putInput{Doc: helloDoc()}); !strings.Contains(msg, "not saved") {
		t.Fatalf("put_unit %q", msg)
	}
}

func TestMemoryPutRace(t *testing.T) {
	mem := NewMemory()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			doc := helloDoc()
			doc.Title = doc.Title + string(rune('a'+i))
			if _, err := mem.PutDraftUnit(context.Background(), PutDraft{Position: i, Doc: &doc, Actor: "mcp"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	rows, err := mem.ListUnits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 8 {
		t.Fatalf("units = %d", len(rows))
	}
}

type rejectToken struct{}

func (rejectToken) Validate(context.Context, string, *http.Request) (Principal, error) {
	return Principal{}, ErrUnauthorized
}

func helloLexeme() Lexeme {
	return Lexeme{
		TextNFC:      hello,
		Bare:         helloBare,
		Gloss:        "hello; peace",
		POS:          "interjection",
		Translit:     "shalom",
		AudioPresent: true,
	}
}

func helloDoc() units.Unit {
	return units.Unit{
		Title:     "Hello",
		Situation: "Meeting a friend",
		Concept:   "Greeting",
		NewWords:  []string{hello},
		Lessons: []units.Lesson{{
			Kind: units.KindLesson,
			Activities: []units.Activity{
				{Type: units.ActivityListenPick, Text: hello},
				{Type: units.ActivityRecall, Text: hello},
			},
		}},
	}
}

func getContext(t *testing.T, session *mcp.ClientSession, position int) getContextOutput {
	t.Helper()
	return call[getContextOutput](t, session, "get_context", getContextInput{Position: position})
}

func mustList(t *testing.T, mem *Memory) []UnitOutline {
	t.Helper()
	rows, err := mem.ListUnits(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func indexOf(tools *mcp.ListToolsResult, name string) int {
	for i, tool := range tools.Tools {
		if tool.Name == name {
			return i
		}
	}
	return 0
}

func sameIssues(got, want []units.Issue) bool {
	if len(want) == 0 {
		return len(got) == 0
	}
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func countIssues(issues []units.Issue) (blocks, warns int) {
	for _, issue := range issues {
		switch issue.Severity {
		case units.SeverityBlock:
			blocks++
		case units.SeverityWarn:
			warns++
		}
	}
	return blocks, warns
}

func hasRule(issues []units.Issue, rule string) bool {
	for _, issue := range issues {
		if issue.Rule == rule {
			return true
		}
	}
	return false
}

func readResource(t *testing.T, session *mcp.ClientSession, uri string) string {
	t.Helper()
	res, err := session.ReadResource(t.Context(), &mcp.ReadResourceParams{URI: uri})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("%s contents %d", uri, len(res.Contents))
	}
	return res.Contents[0].Text
}

func call[T any](t *testing.T, session *mcp.ClientSession, name string, args any) T {
	t.Helper()
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return decode[T](t, res)
}

func callErr(t *testing.T, session *mcp.ClientSession, name string, args any) string {
	t.Helper()
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s protocol error: %v", name, err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("%s: want isError", name)
	}
	return resultText(res)
}

func decode[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(res))
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

func resultText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func connectMCP(t *testing.T, h http.Handler, token string) *mcp.ClientSession {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	client := mcp.NewClient(&mcp.Implementation{Name: "author-test", Version: "0"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:             srv.URL,
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
		HTTPClient:           &http.Client{Transport: bearerTrip{token: token}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

type bearerTrip struct{ token string }

func (b bearerTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if b.token != "" {
		r.Header.Set("Authorization", "Bearer "+b.token)
	}
	return http.DefaultTransport.RoundTrip(r)
}

const initializeBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`

func postInitialize(t *testing.T, url, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(initializeBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
