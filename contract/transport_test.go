package contract

import (
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/normalize"
)

func TestUnauthenticatedMCPIs401(t *testing.T) {
	c := mcp.New(mcpURL(t), "")
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	res, err := c.Post(testCtx(t), body, map[string]string{"Authorization": ""})
	if err != nil {
		t.Fatal(err)
	}
	assertPlain(t, res, 401, "Unauthorized")
	want := `Bearer realm="hebrew-lessons", resource_metadata="` + publicURL(t) + `/.well-known/oauth-protected-resource"`
	if got := res.HeaderGet("WWW-Authenticate"); got != want {
		t.Fatalf("WWW-Authenticate\n got %q\nwant %q", got, want)
	}
	assertNoCORS(t, res)
	if res.HeaderGet("Mcp-Session-Id") != "" {
		t.Fatalf("401 issued a session id %q", res.HeaderGet("Mcp-Session-Id"))
	}
}

func TestBadBearerIs401(t *testing.T) {
	c := mcp.New(mcpURL(t), "not-the-token")
	res, err := c.Call(testCtx(t), 1, "tools/list", nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPlain(t, res, 401, "Unauthorized")
	if res.HeaderGet("WWW-Authenticate") == "" {
		t.Fatal("missing WWW-Authenticate")
	}
	assertNoCORS(t, res)
}

func TestBasicAuthIsNotAccepted(t *testing.T) {
	c := mcp.New(mcpURL(t), "")
	res, err := c.Post(testCtx(t), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), map[string]string{
		"Authorization": "Basic " + osToken(t),
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPlain(t, res, 401, "Unauthorized")
}

func TestForbiddenHost(t *testing.T) {
	c := mcpClient(t)
	res, err := c.Post(testCtx(t), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`), map[string]string{
		"Host": "evil.example",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertPlain(t, res, 403, "Forbidden host")
	assertNoCORS(t, res)
}

func TestUnknownPathIs404(t *testing.T) {
	c := mcpClient(t)
	res, err := c.Exchange(testCtx(t), "GET", originOf(t, mcpURL(t))+"/not-an-mcp-route", authHeaders(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPlain(t, res, 404, "Not found")
}

func TestBadJSONIs400(t *testing.T) {
	c := mcpClient(t)
	res, err := c.Post(testCtx(t), []byte("{nope"), nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPlain(t, res, 400, "Request body is not JSON.")
}

func TestMCPHasNoCORS(t *testing.T) {
	c := mcp.New(mcpURL(t), "")
	res, err := c.Exchange(testCtx(t), "OPTIONS", mcpURL(t), map[string]string{
		"Origin":                        "https://x.example",
		"Access-Control-Request-Method": "POST",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertPlain(t, res, 401, "Unauthorized")
	assertNoCORS(t, res)

	ok, err := mcpClient(t).Call(testCtx(t), 1, "tools/list", nil)
	if err != nil {
		t.Fatal(err)
	}
	assertNoCORS(t, ok)
}

func TestStatelessNoSession(t *testing.T) {
	c := mcpClient(t)
	// tools/list without a prior initialize, and without Mcp-Session-Id.
	res, err := c.Call(testCtx(t), 7, "tools/list", nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSSE(t, res)
	if _, ok := res.RPCResult(); !ok {
		t.Fatalf("tools/list without initialize failed: %s", res.Body)
	}
	if res.HeaderGet("Mcp-Session-Id") != "" {
		t.Fatalf("server issued Mcp-Session-Id %q", res.HeaderGet("Mcp-Session-Id"))
	}

	// A caller-supplied session id is not required and is not enforced.
	withID, err := c.Post(testCtx(t), []byte(`{"jsonrpc":"2.0","id":8,"method":"tools/list"}`), map[string]string{
		"Mcp-Session-Id": "not-a-session",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertSSE(t, withID)
	if _, ok := withID.RPCResult(); !ok {
		t.Fatalf("tools/list with an unknown session id failed: %s", withID.Body)
	}
	if withID.HeaderGet("Mcp-Session-Id") != "" {
		t.Fatalf("server echoed a session id %q", withID.HeaderGet("Mcp-Session-Id"))
	}
}

func TestTrailingSlashIsMCP(t *testing.T) {
	c := mcpClient(t)
	url := strings.TrimRight(mcpURL(t), "/") + "/"
	res, err := c.Exchange(testCtx(t), "POST", url, authHeaders(t), []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if err != nil {
		t.Fatal(err)
	}
	assertSSE(t, res)
	result, ok := res.RPCResult()
	if !ok {
		t.Fatalf("ping on /mcp/ failed: %s", res.Body)
	}
	m, _ := result.(map[string]any)
	if len(m) != 0 {
		t.Fatalf("ping result %#v", result)
	}
}

func TestNotificationIs202(t *testing.T) {
	c := mcpClient(t)
	res, err := c.Post(testCtx(t), []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != 202 {
		t.Fatalf("status %d body %s", res.Status, res.Body)
	}
	if len(strings.TrimSpace(string(res.Body))) != 0 {
		t.Fatalf("body %q", res.Body)
	}
	if res.HeaderGet("Mcp-Session-Id") != "" {
		t.Fatal("notification issued a session id")
	}
}

func TestDeleteMCPIsEmpty200(t *testing.T) {
	c := mcpClient(t)
	h := authHeaders(t)
	delete(h, "Content-Type")
	delete(h, "Accept")
	res, err := c.Exchange(testCtx(t), "DELETE", mcpURL(t), h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != 200 {
		t.Fatalf("status %d body %s", res.Status, res.Body)
	}
	if len(strings.TrimSpace(string(res.Body))) != 0 {
		t.Fatalf("body %q", res.Body)
	}
	if res.HeaderGet("Mcp-Session-Id") != "" {
		t.Fatal("DELETE issued a session id")
	}
}

func TestToolFailuresAreIsErrorResults(t *testing.T) {
	c := mcpClient(t)
	cases := []struct {
		name  string
		tool  string
		args  map[string]any
		field string
		proto bool
	}{
		{name: "unknown tool", tool: "nope", args: map[string]any{}, proto: true},
		{name: "missing required", tool: "get_learner_profile", args: map[string]any{}, field: "username"},
		{name: "bad union", tool: "diff_plan", args: map[string]any{"id": "x", "from": "bogus", "to": 1}, field: "from"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := c.Call(testCtx(t), 1, "tools/call", map[string]any{
				"name":      tc.tool,
				"arguments": tc.args,
			})
			if err != nil {
				t.Fatal(err)
			}
			assertSSE(t, res)
			if res.Status != 200 {
				t.Fatalf("status %d body %s", res.Status, res.Body)
			}
			msg := res.Message()
			if msg == nil {
				t.Fatalf("no json-rpc message: %s", res.Body)
			}
			if _, isRPCError := msg["error"]; isRPCError && msg["result"] == nil {
				t.Fatalf("tool failure came back as a JSON-RPC error: %s", res.Body)
			}
			rec := normalize.Classify(res.Status, "", msg)
			if !rec.IsError || rec.TransportForm != normalize.FormIsError {
				t.Fatalf("record %+v body %s", rec, res.Body)
			}
			if tc.proto {
				if rec.Category != normalize.CategoryProtocol {
					t.Fatalf("category %s body %s", rec.Category, res.Body)
				}
				return
			}
			if rec.Category != normalize.CategoryValidation || rec.Message != "<validation>" {
				t.Fatalf("record %+v body %s", rec, res.Body)
			}
			if rec.Field != tc.field {
				t.Fatalf("field %#v body %s", rec.Field, res.Body)
			}
		})
	}
}

// TestJSONRPCFailuresAreNotToolResults records failures Node returns as
// JSON-RPC errors. SDK input validation is not in this list: that one is an
// isError tool result (TestToolFailuresAreIsErrorResults). These are transport
// and protocol rejections from the SDK, frozen as they are.
func TestJSONRPCFailuresAreNotToolResults(t *testing.T) {
	c := mcpClient(t)
	list := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	cases := []struct {
		name    string
		status  int
		code    int
		message string
		call    func(t *testing.T) *mcp.Result
	}{
		{
			name:    "accept must include event-stream",
			status:  406,
			code:    -32000,
			message: "Not Acceptable: Client must accept both application/json and text/event-stream",
			call: func(t *testing.T) *mcp.Result {
				res, err := c.Post(testCtx(t), list, map[string]string{"Accept": "application/json"})
				if err != nil {
					t.Fatal(err)
				}
				return res
			},
		},
		{
			name:    "content-type must be json",
			status:  415,
			code:    -32000,
			message: "Unsupported Media Type: Content-Type must be application/json",
			call: func(t *testing.T) *mcp.Result {
				res, err := c.Post(testCtx(t), list, map[string]string{"Content-Type": "text/plain"})
				if err != nil {
					t.Fatal(err)
				}
				return res
			},
		},
		{
			name:    "unsupported protocol header",
			status:  400,
			code:    -32000,
			message: "Bad Request: Unsupported protocol version: 1999-01-01 (supported versions: 2025-11-25, 2025-06-18, 2025-03-26, 2024-11-05, 2024-10-07)",
			call: func(t *testing.T) *mcp.Result {
				res, err := c.Post(testCtx(t), list, map[string]string{"MCP-Protocol-Version": "1999-01-01"})
				if err != nil {
					t.Fatal(err)
				}
				return res
			},
		},
		{
			name:    "unknown method",
			status:  200,
			code:    -32601,
			message: "Method not found",
			call: func(t *testing.T) *mcp.Result {
				res, err := c.Call(testCtx(t), 3, "resources/list", nil)
				if err != nil {
					t.Fatal(err)
				}
				return res
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := tc.call(t)
			if res.Status != tc.status {
				t.Fatalf("status %d body %s", res.Status, res.Body)
			}
			msg := res.Message()
			if msg == nil {
				t.Fatalf("body %s", res.Body)
			}
			if _, ok := msg["result"]; ok {
				t.Fatalf("got a result, want a JSON-RPC error: %s", res.Body)
			}
			rec := normalize.Classify(res.Status, "", msg)
			if rec.TransportForm != normalize.FormJSONRPC || rec.IsError {
				t.Fatalf("record %+v", rec)
			}
			if rec.JSONRPC.Code == nil || *rec.JSONRPC.Code != tc.code {
				t.Fatalf("code %+v", rec.JSONRPC.Code)
			}
			if rec.Message != tc.message {
				t.Fatalf("message %q", rec.Message)
			}
		})
	}
}

func assertPlain(t *testing.T, res *mcp.Result, status int, body string) {
	t.Helper()
	if res.Status != status {
		t.Fatalf("status %d body %q", res.Status, res.Body)
	}
	if string(res.Body) != body {
		t.Fatalf("body %q", res.Body)
	}
	ct := res.HeaderGet("Content-Type")
	if ct != "text/plain; charset=utf-8" {
		t.Fatalf("content-type %q", ct)
	}
}

func assertSSE(t *testing.T, res *mcp.Result) {
	t.Helper()
	if res.Status != 200 {
		t.Fatalf("status %d body %s", res.Status, res.Body)
	}
	ct := res.HeaderGet("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type %q", ct)
	}
	if !strings.HasPrefix(string(res.Body), "event: message\n") && !strings.HasPrefix(string(res.Body), "event: message\r\n") {
		t.Fatalf("SSE body starts %q", trim(res.Body))
	}
	if cc := res.HeaderGet("Cache-Control"); cc != "no-cache, no-transform" {
		t.Errorf("cache-control %q", cc)
	}
	if xb := res.HeaderGet("X-Accel-Buffering"); xb != "no" {
		t.Errorf("x-accel-buffering %q", xb)
	}
	if res.HeaderGet("Mcp-Session-Id") != "" {
		t.Errorf("issued Mcp-Session-Id %q", res.HeaderGet("Mcp-Session-Id"))
	}
	assertNoCORS(t, res)
}

func assertNoCORS(t *testing.T, res *mcp.Result) {
	t.Helper()
	for _, name := range []string{
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Headers",
		"Access-Control-Allow-Credentials",
		"Access-Control-Max-Age",
		"Access-Control-Expose-Headers",
	} {
		if v := res.HeaderGet(name); v != "" {
			t.Errorf("CORS header %s=%q", name, v)
		}
	}
}

func osToken(t *testing.T) string {
	t.Helper()
	return mcpClient(t).Token
}

func trim(b []byte) string {
	if len(b) > 120 {
		b = b[:120]
	}
	return string(b)
}
