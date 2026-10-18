package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSchemasAndSDKRegistry(t *testing.T) {
	app := newTestApp(t, "")
	if len(app.schemas) != 42 || len(app.toolOrder) != 42 {
		t.Fatalf("tools %d order %d", len(app.schemas), len(app.toolOrder))
	}
	if app.ProtocolServer() == nil {
		t.Fatal("sdk server was not constructed")
	}
	for _, name := range app.toolOrder {
		if _, ok := app.schemas[name]; !ok {
			t.Fatalf("missing schema %s", name)
		}
	}
	if !readOnlyTools["plan_image"] || readOnlyTools["upsert_lesson"] {
		t.Fatal("readOnlyHint set does not match the 18 tools")
	}
	if len(readOnlyTools) != 18 {
		t.Fatalf("read-only tools %d", len(readOnlyTools))
	}
}

func TestInitializeAndToolsListBytes(t *testing.T) {
	app := newTestApp(t, "http://127.0.0.1:18787")
	rec := postMCP(app, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-10-07","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`, nil)
	want := "event: message\ndata: {\"result\":{\"protocolVersion\":\"2024-10-07\",\"capabilities\":{\"tools\":{\"listChanged\":true}},\"serverInfo\":{\"name\":\"hebrew-lessons\",\"version\":\"1.0.0\"}},\"jsonrpc\":\"2.0\",\"id\":1}\n\n"
	if rec.Body.String() != want {
		t.Fatalf("initialize\n got %q\nwant %q", rec.Body.String(), want)
	}
	assertSSEHeaders(t, rec)

	rec = postMCP(app, `{"jsonrpc":"2.0","id":"abc","method":"initialize","params":{"protocolVersion":"2099-01-01","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`, nil)
	if !strings.Contains(rec.Body.String(), `"protocolVersion":"2025-11-25"`) || !strings.Contains(rec.Body.String(), `"id":"abc"`) {
		t.Fatalf("fallback %s", rec.Body.String())
	}

	rec = postMCP(app, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, nil)
	wantList := "event: message\ndata: {\"result\":" + strings.TrimSpace(string(toolsListResult)) + ",\"jsonrpc\":\"2.0\",\"id\":2}\n\n"
	if rec.Body.String() != wantList {
		t.Fatalf("tools/list bytes differ: got %d want %d", rec.Body.Len(), len(wantList))
	}
	if !strings.Contains(rec.Body.String(), `"execution":{"taskSupport":"forbidden"}`) {
		t.Fatal("taskSupport missing")
	}
}

func TestPingNotificationDeleteAndMethod(t *testing.T) {
	app := newTestApp(t, "")
	rec := postMCP(app, `{"jsonrpc":"2.0","id":1,"method":"ping"}`, nil)
	if rec.Code != 200 || rec.Body.String() != "event: message\ndata: {\"result\":{},\"jsonrpc\":\"2.0\",\"id\":1}\n\n" {
		t.Fatalf("ping %d %q", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Length") != "59" {
		t.Fatalf("ping length %q", rec.Header().Get("Content-Length"))
	}

	req := httptest.NewRequest(http.MethodPost, "/mcp/", strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Authorization", "Bearer contract-test-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec = httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != 202 || rec.Body.Len() != 0 {
		t.Fatalf("notification %d %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/mcp", nil)
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Authorization", "Bearer contract-test-token")
	rec = httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.Len() != 0 || rec.Header().Get("Mcp-Session-Id") != "" {
		t.Fatalf("delete %d %q", rec.Code, rec.Body.String())
	}

	rec = postMCP(app, `{"jsonrpc":"2.0","id":3,"method":"resources/list"}`, nil)
	want := "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":3,\"error\":{\"code\":-32601,\"message\":\"Method not found\"}}\n\n"
	if rec.Body.String() != want {
		t.Fatalf("method %q", rec.Body.String())
	}
}

func TestHTTPStatusCases(t *testing.T) {
	app := newTestApp(t, "http://127.0.0.1:18787")

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != 401 || rec.Body.String() != "Unauthorized" || rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("401 %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}
	wantWWW := `Bearer realm="hebrew-lessons", resource_metadata="http://127.0.0.1:18787/.well-known/oauth-protected-resource"`
	if rec.Header().Get("WWW-Authenticate") != wantWWW {
		t.Fatalf("www %q", rec.Header().Get("WWW-Authenticate"))
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("cors on 401")
	}

	req = httptest.NewRequest(http.MethodOptions, "/mcp", nil)
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Origin", "https://x.example")
	rec = httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != 401 || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("options %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	req.Host = "evil.example"
	req.Header.Set("Authorization", "Bearer contract-test-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec = httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != 403 || rec.Body.String() != "Forbidden host" {
		t.Fatalf("403 %d %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/nope", nil)
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Authorization", "Bearer contract-test-token")
	rec = httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	if rec.Code != 404 || rec.Body.String() != "Not found" {
		t.Fatalf("404 %d %q", rec.Code, rec.Body.String())
	}

	rec = postMCP(app, "{nope", nil)
	if rec.Code != 400 || rec.Body.String() != "Request body is not JSON." || rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("bad json %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}

	rec = postMCP(app, "", nil)
	wantEmpty := `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error: Invalid JSON"},"id":null}`
	if rec.Code != 400 || rec.Body.String() != wantEmpty || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("empty %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}

	rec = postMCP(app, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, map[string]string{"Accept": "application/json"})
	want406 := `{"jsonrpc":"2.0","error":{"code":-32000,"message":"Not Acceptable: Client must accept both application/json and text/event-stream"},"id":null}`
	if rec.Code != 406 || rec.Body.String() != want406 {
		t.Fatalf("406 %d %q", rec.Code, rec.Body.String())
	}

	rec = postMCP(app, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, map[string]string{"Content-Type": "text/plain"})
	want415 := `{"jsonrpc":"2.0","error":{"code":-32000,"message":"Unsupported Media Type: Content-Type must be application/json"},"id":null}`
	if rec.Code != 415 || rec.Body.String() != want415 {
		t.Fatalf("415 %d %q", rec.Code, rec.Body.String())
	}

	rec = postMCP(app, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, map[string]string{"MCP-Protocol-Version": "1999-01-01"})
	want400 := `{"jsonrpc":"2.0","error":{"code":-32000,"message":"Bad Request: Unsupported protocol version: 1999-01-01 (supported versions: 2025-11-25, 2025-06-18, 2025-03-26, 2024-11-05, 2024-10-07)"},"id":null}`
	if rec.Code != 400 || rec.Body.String() != want400 {
		t.Fatalf("proto %d %q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/mcp", nil)
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Authorization", "Bearer contract-test-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	rec = httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	want405 := `{"jsonrpc":"2.0","error":{"code":-32000,"message":"Method not allowed."},"id":null}`
	if rec.Code != 405 || rec.Body.String() != want405 || rec.Header().Get("Allow") != "GET, POST, DELETE" {
		t.Fatalf("405 %d %q allow %q", rec.Code, rec.Body.String(), rec.Header().Get("Allow"))
	}
}

func TestToolErrorBytes(t *testing.T) {
	app := newTestApp(t, "")
	cases := []struct {
		body string
		want string
	}{
		{
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
			`event: message
data: {"result":{"content":[{"type":"text","text":"MCP error -32602: Tool nope not found"}],"isError":true},"jsonrpc":"2.0","id":1}

`,
		},
		{
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_learner_profile","arguments":{}}}`,
			`event: message
data: {"result":{"content":[{"type":"text","text":"MCP error -32602: Input validation error: Invalid arguments for tool get_learner_profile: Invalid input: expected string, received undefined at username"}],"isError":true},"jsonrpc":"2.0","id":1}

`,
		},
		{
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_learner_profile","arguments":{"username":""}}}`,
			`event: message
data: {"result":{"content":[{"type":"text","text":"MCP error -32602: Input validation error: Invalid arguments for tool get_learner_profile: Too small: expected string to have >=1 characters at username"}],"isError":true},"jsonrpc":"2.0","id":1}

`,
		},
		{
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_learner_profile","arguments":{"username":1}}}`,
			`event: message
data: {"result":{"content":[{"type":"text","text":"MCP error -32602: Input validation error: Invalid arguments for tool get_learner_profile: Invalid input: expected string, received number at username"}],"isError":true},"jsonrpc":"2.0","id":1}

`,
		},
		{
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"diff_plan","arguments":{"id":"x","from":"bogus","to":1}}}`,
			`event: message
data: {"result":{"content":[{"type":"text","text":"MCP error -32602: Input validation error: Invalid arguments for tool diff_plan: Invalid input at from"}],"isError":true},"jsonrpc":"2.0","id":1}

`,
		},
		{
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"diff_plan","arguments":{"id":"x","from":0,"to":1}}}`,
			`event: message
data: {"result":{"content":[{"type":"text","text":"MCP error -32602: Input validation error: Invalid arguments for tool diff_plan: Too small: expected number to be >=1 at from"}],"isError":true},"jsonrpc":"2.0","id":1}

`,
		},
		{
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_learner_profile","arguments":null}}`,
			"event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32603,\"message\":\"[\\n  {\\n    \\\"expected\\\": \\\"record\\\",\\n    \\\"code\\\": \\\"invalid_type\\\",\\n    \\\"path\\\": [\\n      \\\"params\\\",\\n      \\\"arguments\\\"\\n    ],\\n    \\\"message\\\": \\\"Invalid input: expected record, received null\\\"\\n  }\\n]\"}}\n\n",
		},
	}
	for _, tc := range cases {
		rec := postMCP(app, tc.body, nil)
		if rec.Code != 200 || rec.Body.String() != tc.want {
			t.Fatalf("body %s\n got %q\nwant %q", tc.body, rec.Body.String(), tc.want)
		}
	}
}

func TestAllowedHostsAndListenDefaults(t *testing.T) {
	if !allowedHost("localhost:8787") || !allowedHost("[::1]:8787") || !allowedHost("127.0.0.1") {
		t.Fatal("loopback host rejected")
	}
	t.Setenv("MCP_ALLOWED_HOSTS", "lessons.example")
	if !allowedHost("lessons.example:443") || allowedHost("evil.example") {
		t.Fatal("allowlist")
	}
	t.Setenv("MCP_PORT", "")
	if got, err := mcpPort(); err != nil || got != 8787 {
		t.Fatalf("default port %d %v", got, err)
	}
	t.Setenv("MCP_PORT", "8790")
	if got, err := mcpPort(); err != nil || got != 8790 {
		t.Fatalf("port %d %v", got, err)
	}
	if loopbackHost("0.0.0.0") || !loopbackHost("::1") {
		t.Fatal("loopbackHost")
	}
}

func TestSSEKeepAlive(t *testing.T) {
	app := newTestApp(t, "")
	prev := keepAliveEvery
	keepAliveEvery = 20 * time.Millisecond
	t.Cleanup(func() { keepAliveEvery = prev })

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil).WithContext(ctx)
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Authorization", "Bearer contract-test-token")
	req.Header.Set("Accept", "text/event-stream")
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		app.Handler().ServeHTTP(rec, req)
		close(done)
	}()
	time.Sleep(70 * time.Millisecond)
	cancel()
	<-done
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("get %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Cache-Control") != "no-cache, no-transform" || rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("headers %v", rec.Header())
	}
	if !strings.Contains(rec.Body.String(), ": keepalive\n\n") {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestLessonGoldens(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "worksheets")
	got, err := listLessons(dir)
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "list-lessons.txt", Stringify(got, 2))

	got, err = getLesson(dir, "my-morning")
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "get-lesson.txt", Stringify(got, 2))

	got, err = listWords(dir, "my-morning")
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "list-words-one.txt", Stringify(got, 2))

	app := newTestApp(t, "")
	app.lessonsDir = dir
	text, isErr := app.callTool("list_activity_types", Object())
	if isErr {
		t.Fatal(text)
	}
	compareGolden(t, "activities.txt", text)
}

func TestPlanImageAndAuditGate(t *testing.T) {
	app := newTestApp(t, "")
	text, isErr := app.callTool("plan_image", Object(Field{Key: "prompt", Val: String("a red apple")}))
	if !isErr || text != `{"error":"missing_key","message":"Image generation is not configured."}` {
		t.Fatalf("missing key %v %q", isErr, text)
	}
	t.Setenv("OPENAI_API_KEY", "test-image-key")
	text, isErr = app.callTool("plan_image", Object(
		Field{Key: "prompt", Val: String("a red apple")},
		Field{Key: "count", Val: Int(2)},
	))
	if isErr || text != "{\n  \"count\": 2,\n  \"estimatedDollars\": 0.022\n}" {
		t.Fatalf("estimate %v %q", isErr, text)
	}
	text, isErr = app.callTool("plan_image", Object(Field{Key: "prompt", Val: String("a sexual act")}))
	if !isErr || text != `{"error":"prompt_refused","message":"That prompt is not allowed."}` {
		t.Fatalf("refused %v %q", isErr, text)
	}
	text, isErr = app.callTool("plan_image", Object(
		Field{Key: "prompt", Val: String("a red apple")},
		Field{Key: "count", Val: Int(0)},
	))
	if !isErr || text != `{"error":"invalid_input","message":"The image request is not valid."}` {
		t.Fatalf("zero %v %q", isErr, text)
	}

	text, isErr = app.callTool("get_learner_profile", Object(Field{Key: "username", Val: String("ada")}))
	if !isErr || !strings.Contains(text, "audit row cannot be written") || app.reads != nil {
		t.Fatalf("nil audit %v %q reads %#v", isErr, text, app.reads)
	}

	audit := &memAudit{}
	reader := &memLearner{learner: Learner{Username: "ada", Active: true, AIPlanning: true}}
	app.audit = audit
	app.learner = reader
	reader.profile = Object(Field{Key: "username", Val: String("ada")})
	text, isErr = app.callTool("get_learner_profile", Object(Field{Key: "username", Val: String("ada")}))
	if isErr || text != "{\n  \"username\": \"ada\"\n}" || audit.rows != 1 {
		t.Fatalf("profile %v %q rows %d", isErr, text, audit.rows)
	}
	audit.err = errString("disk full")
	text, isErr = app.callTool("check_text_coverage", Object(
		Field{Key: "username", Val: String("ada")},
		Field{Key: "text", Val: String("שלום")},
	))
	if !isErr || text != `{"error":"invalid_input"}` {
		t.Fatalf("append fail %v %q", isErr, text)
	}

	text, isErr = app.callTool("list_plan_issues", Object(Field{Key: "id", Val: String("plan-1")}))
	if !isErr || !strings.Contains(text, "internal/plans") {
		t.Fatalf("issues %v %q", isErr, text)
	}
	text, isErr = app.callTool("upsert_lesson", Object(
		Field{Key: "slug", Val: String("my-morning")},
		Field{Key: "worksheet", Val: Object()},
	))
	if !isErr || text != "This MCP build does not implement upsert_lesson. Mutating tools are a later pull request; nothing was written." {
		t.Fatalf("mutate %v %q", isErr, text)
	}
}

func TestMissingDatabaseIsExplicit(t *testing.T) {
	app := newTestApp(t, "")
	text, isErr := app.callTool("list_users", Object())
	if !isErr || !strings.Contains(text, "table users is not readable") {
		t.Fatalf("users %v %q", isErr, text)
	}
	text, isErr = app.callTool("list_lessons", Object())
	if isErr {
		t.Fatal(text)
	}
	compareGolden(t, "list-lessons.txt", text)
}

type memAudit struct {
	err  error
	rows int
}

func (m *memAudit) Append(actor, action, target, detail string, at time.Time) error {
	if m.err != nil {
		return m.err
	}
	m.rows++
	return nil
}

type memLearner struct {
	learner Learner
	profile Value
}

func (m *memLearner) Find(username string) (Learner, bool, error) {
	if username != m.learner.Username {
		return Learner{}, false, nil
	}
	return m.learner, true, nil
}

func (m *memLearner) Profile(username string) (Value, error) { return m.profile, nil }

func (m *memLearner) Coverage(username, text string) (Value, error) {
	return Object(Field{Key: "text", Val: String(text)}), nil
}

func newTestApp(t *testing.T, public string) *App {
	t.Helper()
	t.Setenv("MCP_AUTH_TOKEN", "contract-test-token")
	t.Setenv("MCP_PUBLIC_URL", public)
	app, err := New(Options{LessonsDir: filepath.Join(repoRoot(t), "worksheets")})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func postMCP(app *App, body string, extra map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Host = "127.0.0.1:8787"
	req.Header.Set("Authorization", "Bearer contract-test-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range extra {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	app.Handler().ServeHTTP(rec, req)
	return rec
}

func assertSSEHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content-type %q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Cache-Control") != "no-cache, no-transform" {
		t.Fatalf("cache %q", rec.Header().Get("Cache-Control"))
	}
	if rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Fatalf("accel %q", rec.Header().Get("X-Accel-Buffering"))
	}
	if rec.Header().Get("Mcp-Session-Id") != "" {
		t.Fatal("session id")
	}
}

func compareGolden(t *testing.T, name, got string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	if got != string(raw) {
		t.Fatalf("%s mismatch got %d want %d", name, len(got), len(raw))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
