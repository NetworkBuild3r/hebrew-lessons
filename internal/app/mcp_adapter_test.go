package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPToolReadsStore proves newMCP passes the database adapter.
// A tool call returns the lexeme row instead of isError.
func TestMCPToolReadsStore(t *testing.T) {
	a := newTestApp(t, Config{DataDir: t.TempDir(), MCPAuthToken: "test-token"})
	ctx := context.Background()
	id, err := testharness.InsertLexeme(ctx, a.Store(), "שָׁלוֹם")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store().ExecContext(ctx, `UPDATE lexemes SET gloss_en = 'peace', bare = 'שלום', gender = 'm', translit = 'shalom' WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}

	session := connectAppMCP(t, a.MCP(), "test-token")
	lookup := callAppTool(t, session, "lookup", map[string]any{"query": "שלום"})
	var found struct {
		Hits []struct {
			Text  string `json:"text_nfc"`
			Gloss string `json:"gloss"`
		} `json:"hits"`
		Suggestions []any `json:"suggestions"`
	}
	decodeStructured(t, lookup, &found)
	if len(found.Hits) != 1 || found.Hits[0].Text != "שָׁלוֹם" || found.Hits[0].Gloss != "peace" || len(found.Suggestions) != 0 {
		t.Fatalf("lookup %+v", found)
	}

	if _, err := a.Store().ExecContext(ctx, `
		INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
		VALUES (0, 'Hello', 'Street', 'Greeting', 'draft', 1, ?, 'shared')`,
		`{"title":"Hello","newWords":["שָׁלוֹם"]}`); err != nil {
		t.Fatal(err)
	}

	outline := callAppTool(t, session, "get_context", map[string]any{"position": 1})
	var ctxOut struct {
		Outline []struct {
			Title  string `json:"title"`
			Scope  string `json:"scope"`
			Status string `json:"status"`
		} `json:"outline"`
		Known []string `json:"knownLexemes"`
	}
	decodeStructured(t, outline, &ctxOut)
	if len(ctxOut.Outline) != 1 || ctxOut.Outline[0].Title != "Hello" || ctxOut.Outline[0].Scope != "shared" || ctxOut.Outline[0].Status != "draft" {
		t.Fatalf("outline %+v", ctxOut.Outline)
	}
	if len(ctxOut.Known) != 0 {
		t.Fatalf("draft known %+v", ctxOut.Known)
	}

	if _, err := a.Store().ExecContext(ctx, `UPDATE units SET status = 'published' WHERE position = 0`); err != nil {
		t.Fatal(err)
	}
	published := callAppTool(t, session, "get_context", map[string]any{"position": 1})
	decodeStructured(t, published, &ctxOut)
	if len(ctxOut.Outline) != 1 || ctxOut.Outline[0].Status != "published" {
		t.Fatalf("published outline %+v", ctxOut.Outline)
	}
	if len(ctxOut.Known) != 1 || ctxOut.Known[0] != "שָׁלוֹם" {
		t.Fatalf("known %+v", ctxOut.Known)
	}
}

func connectAppMCP(t *testing.T, h http.Handler, token string) *mcp.ClientSession {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	client := mcp.NewClient(&mcp.Implementation{Name: "adapter-test", Version: "0"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:             srv.URL + "/mcp",
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
		HTTPClient:           &http.Client{Transport: bearerRoundTrip{token: token}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

type bearerRoundTrip struct{ token string }

func (b bearerRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func callAppTool(t *testing.T, session *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s isError: %s", name, toolText(res))
	}
	return res
}

func decodeStructured(t *testing.T, res *mcp.CallToolResult, dest any) {
	t.Helper()
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
}

func toolText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}
