package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type roleTokens struct{}

func (roleTokens) Validate(_ context.Context, token string, _ *http.Request) (Principal, error) {
	switch token {
	case "learner":
		return Principal{Actor: "7", UserID: 7, Role: RoleLearner, Source: "oauth"}, nil
	case "admin":
		return Principal{Actor: "1", UserID: 1, Role: RoleAdmin, Source: "oauth"}, nil
	default:
		return Principal{}, ErrUnauthorized
	}
}

func roleServer(t *testing.T) (*Memory, *httptest.Server) {
	t.Helper()
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	mem.AddLexeme(Lexeme{TextNFC: coffee, Bare: "קפה", Gloss: "coffee", POS: "noun"})
	mem.SetPlacement(7, Placement{Finished: true, Sittings: 1, StopReason: "exhausted"}, []string{hello}, nil)
	mem.SetPlacement(9, Placement{Finished: true, Sittings: 1, StopReason: "exhausted"}, []string{coffee}, nil)
	h := Handler(Options{
		Validator: roleTokens{},
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return mem, srv
}

func TestLearnerPutUnitForbidden(t *testing.T) {
	mem, srv := roleServer(t)
	for _, scope := range []string{ScopeShared, "learner:7"} {
		res := postTool(t, srv.URL, "learner", "put_unit", map[string]any{
			"scope": scope, "position": 0, "doc": helloDoc(),
		}, nil)
		assertDenied(t, res, msgAdminOnly)
	}
	units, err := mem.ListUnits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 0 {
		t.Fatalf("learner put wrote %+v", units)
	}
}

func TestLearnerGetContextOtherLearnerForbidden(t *testing.T) {
	_, srv := roleServer(t)
	other := int64(9)
	res := postTool(t, srv.URL, "learner", "get_context", getContextInput{Position: 0, LearnerID: &other}, nil)
	body := assertDenied(t, res, msgOtherLearner)
	if strings.Contains(body, coffee) {
		t.Fatalf("other learner leaked: %s", body)
	}
}

func TestLearnerGetContextSelfOK(t *testing.T) {
	_, srv := roleServer(t)
	self := int64(7)
	res := postTool(t, srv.URL, "learner", "get_context", getContextInput{Position: 0, LearnerID: &self}, nil)
	body := readBody(t, res)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body %s", res.StatusCode, body)
	}
	if strings.Contains(body, `"isError":true`) || !strings.Contains(body, hello) {
		t.Fatalf("body %s", body)
	}
}

func TestAdminPutUnitShared(t *testing.T) {
	mem, srv := roleServer(t)
	res := postTool(t, srv.URL, "admin", "put_unit", map[string]any{
		"scope": ScopeShared, "position": 0, "doc": helloDoc(),
	}, nil)
	body := readBody(t, res)
	if res.StatusCode != http.StatusOK || strings.Contains(body, `"isError":true`) {
		t.Fatalf("status %d body %s", res.StatusCode, body)
	}
	units, err := mem.ListUnits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 1 || units[0].Scope != ScopeShared || units[0].Status != "draft" {
		t.Fatalf("units %+v", units)
	}
}

func TestAdminGetContextOtherLearner(t *testing.T) {
	_, srv := roleServer(t)
	other := int64(9)
	res := postTool(t, srv.URL, "admin", "get_context", getContextInput{Position: 0, LearnerID: &other}, nil)
	body := readBody(t, res)
	if res.StatusCode != http.StatusOK || strings.Contains(body, `"isError":true`) || !strings.Contains(body, coffee) {
		t.Fatalf("status %d body %s", res.StatusCode, body)
	}
}

func TestStaticTokenIsAdmin(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	mem.AddLexeme(Lexeme{TextNFC: coffee, Bare: "קפה", Gloss: "coffee", POS: "noun"})
	mem.SetPlacement(9, Placement{Finished: true, Sittings: 1, StopReason: "exhausted"}, []string{coffee}, nil)
	h := Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	put := postTool(t, srv.URL, testToken, "put_unit", map[string]any{
		"scope": ScopeShared, "position": 0, "doc": helloDoc(),
	}, nil)
	putBody := readBody(t, put)
	if put.StatusCode != http.StatusOK || strings.Contains(putBody, `"isError":true`) {
		t.Fatalf("put status %d body %s", put.StatusCode, putBody)
	}
	units, err := mem.ListUnits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 1 || units[0].Scope != ScopeShared {
		t.Fatalf("units %+v", units)
	}

	other := int64(9)
	got := postTool(t, srv.URL, testToken, "get_context", getContextInput{Position: 0, LearnerID: &other}, nil)
	gotBody := readBody(t, got)
	if got.StatusCode != http.StatusOK || strings.Contains(gotBody, `"isError":true`) || !strings.Contains(gotBody, coffee) {
		t.Fatalf("get_context status %d body %s", got.StatusCode, gotBody)
	}

	later := postTool(t, srv.URL, testToken, "publish_unit", map[string]any{}, nil)
	laterBody := readBody(t, later)
	if later.StatusCode == http.StatusForbidden || strings.Contains(laterBody, msgAdminOnly) {
		t.Fatalf("static token treated as non-admin: %d %s", later.StatusCode, laterBody)
	}
}

func TestUnlistedWriteToolIsAdminOnly(t *testing.T) {
	_, srv := roleServer(t)
	res := postTool(t, srv.URL, "learner", "publish_unit", map[string]any{}, nil)
	assertDenied(t, res, msgAdminOnly)
}

func TestDuplicateToolNameIsDenied(t *testing.T) {
	mem, srv := roleServer(t)
	doc, err := json.Marshal(helloDoc())
	if err != nil {
		t.Fatal(err)
	}
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"put_unit","Name":"lookup","arguments":{"scope":"shared","position":0,"doc":` + string(doc) + `}}}`
	res := postRaw(t, srv.URL, "learner", body)
	assertDenied(t, res, msgAdminOnly)
	units, err := mem.ListUnits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 0 {
		t.Fatalf("duplicate name wrote %+v", units)
	}
}

func TestExtraMethodKeyIsDenied(t *testing.T) {
	mem, srv := roleServer(t)
	doc, err := json.Marshal(helloDoc())
	if err != nil {
		t.Fatal(err)
	}
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","Method":"notifications/initialized","params":{"name":"put_unit","arguments":{"scope":"learner:7","position":0,"doc":` + string(doc) + `}}}`
	res := postRaw(t, srv.URL, "learner", body)
	assertDenied(t, res, msgAdminOnly)
	units, err := mem.ListUnits(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 0 {
		t.Fatalf("extra method wrote %+v", units)
	}
}

func TestLearnerIDFraction(t *testing.T) {
	_, srv := roleServer(t)
	other := postRaw(t, srv.URL, "learner", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_context","arguments":{"position":0,"learnerId":9.0}}}`)
	body := assertDenied(t, other, msgOtherLearner)
	if strings.Contains(body, coffee) {
		t.Fatalf("9.0 leaked another learner: %s", body)
	}
	frac := postRaw(t, srv.URL, "learner", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_context","arguments":{"position":0,"learnerId":9.5}}}`)
	assertDenied(t, frac, msgOtherLearner)

	self := postRaw(t, srv.URL, "learner", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_context","arguments":{"position":0,"learnerId":7.0}}}`)
	selfBody := readBody(t, self)
	if self.StatusCode != http.StatusOK || strings.Contains(selfBody, `"isError":true`) || !strings.Contains(selfBody, hello) {
		t.Fatalf("7.0 self status %d body %s", self.StatusCode, selfBody)
	}
}

func TestMiddlewareBlocksToolWithoutGate(t *testing.T) {
	h := Handler(Options{
		Validator: roleTokens{},
		extraTools: func(server *mcp.Server) {
			mcp.AddTool(server, &mcp.Tool{
				Name:        "probe_role",
				Description: "Test tool with no role check of its own.",
			}, func(context.Context, *mcp.CallToolRequest, map[string]any) (*mcp.CallToolResult, any, error) {
				return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ran"}}}, "ran", nil
			})
		},
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	learner := postTool(t, srv.URL, "learner", "probe_role", map[string]any{}, nil)
	body := assertDenied(t, learner, msgAdminOnly)
	if strings.Contains(body, "ran") {
		t.Fatalf("learner ran the ungated tool: %s", body)
	}
	admin := postTool(t, srv.URL, "admin", "probe_role", map[string]any{}, nil)
	adminBody := readBody(t, admin)
	if admin.StatusCode != http.StatusOK || strings.Contains(adminBody, `"isError":true`) || !strings.Contains(adminBody, "ran") {
		t.Fatalf("admin status %d body %s", admin.StatusCode, adminBody)
	}
}

func TestLearnerAndAdminMayLookupAndValidate(t *testing.T) {
	_, srv := roleServer(t)
	for _, token := range []string{"learner", "admin"} {
		look := postTool(t, srv.URL, token, "lookup", lookupInput{Query: hello}, nil)
		lookBody := readBody(t, look)
		if look.StatusCode != http.StatusOK || strings.Contains(lookBody, `"isError":true`) {
			t.Fatalf("%s lookup %d %s", token, look.StatusCode, lookBody)
		}
		dry := postTool(t, srv.URL, token, "validate_unit", validateInput{Position: 0, Doc: helloDoc()}, nil)
		dryBody := readBody(t, dry)
		if dry.StatusCode != http.StatusOK || strings.Contains(dryBody, `"isError":true`) {
			t.Fatalf("%s validate %d %s", token, dry.StatusCode, dryBody)
		}
	}
}

func TestCrossOriginBrowserPostRejected(t *testing.T) {
	h := Handler(Options{
		AuthToken:      testToken,
		AllowedOrigins: []string{"https://lessons.example"},
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	cross := postInitializeHeader(t, srv.URL, testToken, map[string]string{
		"Sec-Fetch-Site": "cross-site",
		"Origin":         "https://evil.example",
	})
	if cross.StatusCode != http.StatusForbidden {
		body := readBody(t, cross)
		t.Fatalf("sec-fetch status %d body %s", cross.StatusCode, body)
	}
	cross.Body.Close()

	mismatch := postInitializeHeader(t, srv.URL, testToken, map[string]string{
		"Origin": "https://evil.example",
	})
	if mismatch.StatusCode != http.StatusForbidden {
		body := readBody(t, mismatch)
		t.Fatalf("origin status %d body %s", mismatch.StatusCode, body)
	}
	mismatch.Body.Close()

	plain := postInitializeHeader(t, srv.URL, testToken, nil)
	plainBody := readBody(t, plain)
	if plain.StatusCode != http.StatusOK {
		t.Fatalf("non-browser status %d body %s", plain.StatusCode, plainBody)
	}

	trusted := postInitializeHeader(t, srv.URL, testToken, map[string]string{
		"Sec-Fetch-Site": "cross-site",
		"Origin":         "https://lessons.example",
	})
	trustedBody := readBody(t, trusted)
	if trusted.StatusCode != http.StatusOK {
		t.Fatalf("trusted origin status %d body %s", trusted.StatusCode, trustedBody)
	}
}

func TestAuthErrorDoesNotLeakStoreText(t *testing.T) {
	h := Handler(Options{Validator: leakToken{}})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	res := postInitialize(t, srv.URL, "anything")
	body := readBody(t, res)
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status %d body %s", res.StatusCode, body)
	}
	if strings.Contains(body, "store is closed") || strings.Contains(body, "db:") {
		t.Fatalf("leaked body %s", body)
	}
	if !strings.Contains(body, errAuthUnavailable.Error()) {
		t.Fatalf("body %s", body)
	}
}

func TestSessionCookieAloneIsNotMCPAuth(t *testing.T) {
	h := Handler(Options{AuthToken: testToken})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(initializeBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	req.AddCookie(&http.Cookie{Name: "hebrew_session", Value: "not-a-bearer"})
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, res)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d body %s", res.StatusCode, body)
	}
}

func TestPublishedStaysUntilTheDocumentChanges(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken,
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	}), testToken)
	doc := helloDoc()
	first := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, Position: 0, Doc: doc})
	mem.SetStatus(first.UnitID, "published")
	same := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, UnitID: &first.UnitID, Position: 0, Doc: doc})
	if !same.Unchanged || same.Revision != first.Revision || mem.Status(first.UnitID) != "published" {
		t.Fatalf("unchanged %+v status %q", same, mem.Status(first.UnitID))
	}
	doc.Title = "Hello again"
	changed := call[putOutput](t, session, "put_unit", putInput{Scope: ScopeShared, UnitID: &first.UnitID, Position: 0, Doc: doc})
	if changed.Unchanged || changed.Revision != first.Revision+1 || mem.Status(first.UnitID) != "draft" {
		t.Fatalf("changed %+v status %q", changed, mem.Status(first.UnitID))
	}
}

type leakToken struct{}

func (leakToken) Validate(context.Context, string, *http.Request) (Principal, error) {
	return Principal{}, errors.New("db: store is closed")
}

func assertDenied(t *testing.T, res *http.Response, msg string) string {
	t.Helper()
	body := readBody(t, res)
	if res.StatusCode != http.StatusOK || !strings.Contains(body, `"isError":true`) || !strings.Contains(body, msg) {
		t.Fatalf("status %d body %s", res.StatusCode, body)
	}
	return body
}

func postRaw(t *testing.T, url, token, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
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

func postTool(t *testing.T, url, token, name string, args any, hdr map[string]string) *http.Response {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      name,
			"arguments": args,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func postInitializeHeader(t *testing.T, url, token string, hdr map[string]string) *http.Response {
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
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func readBody(t *testing.T, res *http.Response) string {
	t.Helper()
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// SPEC-021 ac-1: a row for every new tool, for a learner on its own id, a learner on another
// learner's id, an admin, the static token, and no token. A tool a learner may call is
// open on the learner's own id and shut on another's. A tool that writes what the next
// prompt reads is shut to a learner on every id. Each row names the exact text it expects.
func TestAuthzRowsForTheLearnerTools(t *testing.T) {
	e := newLT(t)
	unit := e.seedAuthz()
	doc := helloDoc()
	rows := []struct {
		tool       string
		learnerMay bool
		args       func(id *int64) map[string]any
	}{
		{"get_learner_model", true, func(id *int64) map[string]any { return withID(map[string]any{}, id) }},
		{"get_progress", true, func(id *int64) map[string]any { return withID(map[string]any{}, id) }},
		{"unpublish_unit", true, func(id *int64) map[string]any { return withID(map[string]any{"unitId": unit}, id) }},
		{"validate_unit", true, func(id *int64) map[string]any {
			return withID(map[string]any{"position": 0, "doc": doc, "briefId": "BRIEF7"}, id)
		}},
		{"get_lesson_brief", false, func(id *int64) map[string]any { return withID(map[string]any{}, id) }},
		{"add_coach_note", false, func(id *int64) map[string]any { return withID(map[string]any{"body": "note"}, id) }},
	}
	allowed := func(name string, res *http.Response) {
		t.Helper()
		body := readBody(t, res)
		if res.StatusCode != http.StatusOK || strings.Contains(body, `"isError":true`) {
			t.Fatalf("%s: status %d body %s", name, res.StatusCode, body)
		}
	}
	for _, row := range rows {
		own, other := int64(7), int64(9)
		if row.learnerMay {
			allowed(row.tool+" learner own id", postTool(t, e.srv.URL, "learner-a", row.tool, row.args(&own), nil))
			allowed(row.tool+" learner omitted id", postTool(t, e.srv.URL, "learner-a", row.tool, row.args(nil), nil))
			assertDenied(t, postTool(t, e.srv.URL, "learner-a", row.tool, row.args(&other), nil), msgOtherLearner)
			assertDenied(t, postTool(t, e.srv.URL, "learner-b", row.tool, row.args(&own), nil), msgOtherLearner)
		} else {
			assertDenied(t, postTool(t, e.srv.URL, "learner-a", row.tool, row.args(&own), nil), msgAdminOnly)
			assertDenied(t, postTool(t, e.srv.URL, "learner-a", row.tool, row.args(nil), nil), msgAdminOnly)
			assertDenied(t, postTool(t, e.srv.URL, "learner-a", row.tool, row.args(&other), nil), msgAdminOnly)
		}
		allowed(row.tool+" admin", postTool(t, e.srv.URL, "admin", row.tool, row.args(&own), nil))
		allowed(row.tool+" static token", postTool(t, e.srv.URL, "static", row.tool, row.args(&own), nil))
		for _, token := range []string{"admin", "static"} {
			assertDenied(t, postTool(t, e.srv.URL, token, row.tool, row.args(nil), nil), msgLearnerRequired)
		}
		none := postTool(t, e.srv.URL, "", row.tool, row.args(&own), nil)
		if none.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s with no token: status %d", row.tool, none.StatusCode)
		}
		none.Body.Close()
		bad := postTool(t, e.srv.URL, "not-a-token", row.tool, row.args(&own), nil)
		if bad.StatusCode != http.StatusUnauthorized {
			t.Fatalf("%s with a wrong token: status %d", row.tool, bad.StatusCode)
		}
		bad.Body.Close()
	}
}

func withID(m map[string]any, id *int64) map[string]any {
	if id != nil {
		m["learnerId"] = *id
	}
	return m
}
