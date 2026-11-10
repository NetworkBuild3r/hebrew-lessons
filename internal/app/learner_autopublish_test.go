package app

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// INIT-038/SPEC-007. The process default keeps a learner publish off Today,
// and the note names ADR-0033. LEARNER_AUTO_PUBLISH on restores the briefId note.
func TestAppLearnerAutoPublishDefaultOff(t *testing.T) {
	a := newTestApp(t, Config{DataDir: t.TempDir(), MCPAuthToken: "test-token"})
	id, err := a.Store().InsertUser(context.Background(), "one@example.com", "sub-one", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 {
		t.Fatalf("learner id %d, want 1", id)
	}
	session := connectAppMCP(t, a.MCP(), "test-token")
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "put_unit", Arguments: map[string]any{
		"scope": "learner:1", "position": 0, "publish": true,
		"doc": map[string]any{"title": "Hello", "situation": "s", "concept": "c", "newWords": []any{}, "lessons": []any{}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("put_unit %s", toolText(res))
	}
	var out struct {
		UnitID        int64  `json:"unitId"`
		Published     bool   `json:"published"`
		PublishReason string `json:"publishReason"`
	}
	decodeStructured(t, res, &out)
	if out.Published || !strings.Contains(out.PublishReason, "ADR-0033") {
		t.Fatalf("put %+v", out)
	}
	conn, err := a.Store().DB()
	if err != nil {
		t.Fatal(err)
	}
	var status, scope string
	if err := conn.QueryRowContext(t.Context(), `SELECT status, scope FROM units WHERE id = ?`, out.UnitID).Scan(&status, &scope); err != nil {
		t.Fatal(err)
	}
	if status != "draft" || scope != "learner:1" {
		t.Fatalf("stored %s %s", status, scope)
	}

	on := newTestApp(t, Config{DataDir: t.TempDir(), MCPAuthToken: "test-token", LearnerAutoPublish: true})
	if _, err := on.Store().InsertUser(context.Background(), "one@example.com", "sub-one", "learner"); err != nil {
		t.Fatal(err)
	}
	session = connectAppMCP(t, on.MCP(), "test-token")
	res, err = session.CallTool(t.Context(), &mcp.CallToolParams{Name: "put_unit", Arguments: map[string]any{
		"scope": "learner:1", "position": 0, "publish": true,
		"doc": map[string]any{"title": "Hello", "situation": "s", "concept": "c", "newWords": []any{}, "lessons": []any{}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("on put_unit %s", toolText(res))
	}
	decodeStructured(t, res, &out)
	if out.Published || strings.Contains(out.PublishReason, "ADR-0033") || !strings.Contains(out.PublishReason, "briefId") {
		t.Fatalf("on put %+v", out)
	}
}
