//go:build smoke

package smoke

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Staging origins. Checks skip when the matching variable is unset, so a
// local go test does not call these hosts.
const (
	stagingWebURL = "https://hebrew-next.ibhacked.us"
	stagingMCPURL = "https://hebrew-mcp-next.ibhacked.us/mcp"
)

var noRedirect = &http.Client{
	Timeout: 20 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

var followRedirect = &http.Client{Timeout: 20 * time.Second}

func TestMain(m *testing.M) {
	if err := refuseProdSmoke(os.Getenv("SMOKE_WEB_URL"), os.Getenv("SMOKE_MCP_URL"), os.Getenv("SMOKE_ALLOW_PROD")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestWebHealthz(t *testing.T) {
	t.Parallel()
	base := requireEnv(t, "SMOKE_WEB_URL", stagingWebURL)
	status, body := get(t, noRedirect, mustJoin(t, base, "/healthz"))
	assertHealth(t, status, body)
}

func TestMCPHealthz(t *testing.T) {
	t.Parallel()
	base := requireEnv(t, "SMOKE_MCP_URL", stagingMCPURL)
	status, body := get(t, noRedirect, mustJoin(t, base, "/healthz"))
	assertHealth(t, status, body)
}

func TestOAuthDiscovery(t *testing.T) {
	t.Parallel()
	base := requireEnv(t, "SMOKE_WEB_URL", stagingWebURL)
	for _, path := range []string{
		"/.well-known/oauth-authorization-server",
		"/.well-known/oauth-protected-resource",
	} {
		status, body := get(t, noRedirect, mustJoin(t, base, path))
		if status != http.StatusOK {
			t.Fatalf("GET %s = %d %s", path, status, snippet(body))
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(body, &doc); err != nil || len(doc) == 0 {
			t.Fatalf("GET %s body %s", path, snippet(body))
		}
	}
}

func TestWebRoot(t *testing.T) {
	t.Parallel()
	base := requireEnv(t, "SMOKE_WEB_URL", stagingWebURL)
	status, body := get(t, followRedirect, mustJoin(t, base, "/"))
	if status >= 500 {
		t.Fatalf("GET / = %d %s", status, snippet(body))
	}
}

func TestMCPInitializeAndToolsList(t *testing.T) {
	t.Parallel()
	token := strings.TrimSpace(os.Getenv("SMOKE_MCP_TOKEN"))
	if token == "" {
		t.Skip("SMOKE_MCP_TOKEN is unset")
	}
	endpoint := requireEnv(t, "SMOKE_MCP_URL", stagingMCPURL)
	session, initBody := postRPC(t, endpoint, token, "", map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "hebrew-smoke", "version": "0"},
		},
	})
	if len(initBody) == 0 || string(initBody) == "null" {
		t.Fatal("initialize returned no result")
	}
	postNotify(t, endpoint, token, session)
	_, listBody := postRPC(t, endpoint, token, session, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/list",
	})
	var result map[string]json.RawMessage
	if err := json.Unmarshal(listBody, &result); err != nil {
		t.Fatalf("tools/list result: %v", err)
	}
	raw, ok := result["tools"]
	if !ok {
		t.Fatalf("tools/list result has no tools: %s", snippet(listBody))
	}
	var tools []json.RawMessage
	if err := json.Unmarshal(raw, &tools); err != nil {
		t.Fatalf("tools/list tools: %v", err)
	}
}

func requireEnv(t *testing.T, key, staging string) string {
	t.Helper()
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		t.Skipf("%s is unset (staging %s)", key, staging)
	}
	return v
}

func assertHealth(t *testing.T, status int, body []byte) {
	t.Helper()
	var doc struct {
		OK bool `json:"ok"`
	}
	if status != http.StatusOK || json.Unmarshal(body, &doc) != nil || !doc.OK {
		t.Fatalf("healthz = %d %s", status, snippet(body))
	}
}

func mustJoin(t *testing.T, raw, path string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		t.Fatalf("%q is not an absolute http(s) URL", raw)
	}
	u.Path = path
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func get(t *testing.T, client *http.Client, rawURL string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "hebrew-smoke")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body
}

func postRPC(t *testing.T, endpoint, token, session string, payload any) (string, json.RawMessage) {
	t.Helper()
	status, header, body := postJSON(t, endpoint, token, session, payload)
	if status != http.StatusOK {
		t.Fatalf("%s = %d %s", payloadMethod(payload), status, snippet(body))
	}
	msg := decodeRPC(t, header.Get("Content-Type"), body)
	if msg.Error != nil {
		t.Fatalf("%s error %d %s", payloadMethod(payload), msg.Error.Code, msg.Error.Message)
	}
	return header.Get("Mcp-Session-Id"), msg.Result
}

func postNotify(t *testing.T, endpoint, token, session string) {
	t.Helper()
	status, header, body := postJSON(t, endpoint, token, session, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})
	if status/100 != 2 {
		t.Fatalf("notifications/initialized = %d %s", status, snippet(body))
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || (!json.Valid(trimmed) && !strings.Contains(header.Get("Content-Type"), "event-stream")) {
		return
	}
	msg := decodeRPC(t, header.Get("Content-Type"), body)
	if msg.Error != nil {
		t.Fatalf("notifications/initialized error %d %s", msg.Error.Code, msg.Error.Message)
	}
}

func postJSON(t *testing.T, endpoint, token, session string, payload any) (int, http.Header, []byte) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("User-Agent", "hebrew-smoke")
	req.Header.Set("Authorization", "Bearer "+token)
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
	}
	resp, err := noRedirect.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header.Clone(), body
}

type rpcMessage struct {
	Result json.RawMessage
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
}

func decodeRPC(t *testing.T, contentType string, body []byte) rpcMessage {
	t.Helper()
	payload := body
	if strings.Contains(contentType, "text/event-stream") || !json.Valid(body) {
		var last []byte
		for _, line := range bytes.Split(body, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
				continue
			}
			last = data
		}
		if len(last) == 0 {
			t.Fatalf("event stream had no data: %s", snippet(body))
		}
		payload = last
	}
	var msg struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("rpc: %v (%s)", err, snippet(payload))
	}
	return rpcMessage{Result: msg.Result, Error: msg.Error}
}

func payloadMethod(payload any) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "rpc"
	}
	var doc struct {
		Method string `json:"method"`
	}
	if json.Unmarshal(raw, &doc) != nil || doc.Method == "" {
		return "rpc"
	}
	return doc.Method
}

func snippet(b []byte) string {
	if len(b) > 300 {
		b = b[:300]
	}
	return fmt.Sprintf("%q", b)
}
