package contract

import (
	"encoding/json"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/golden"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/normalize"
)

func TestToolsList(t *testing.T) {
	res, err := mcpClient(t).Call(testCtx(t), 2, "tools/list", nil)
	if err != nil {
		t.Fatal(err)
	}
	assertSSE(t, res)
	raw, ok := res.RPCResult()
	if !ok {
		t.Fatalf("tools/list failed: %s", res.Body)
	}
	result, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("result %#v", raw)
	}
	if _, cursor := result["nextCursor"]; cursor {
		t.Fatalf("tools/list paged: %#v", result["nextCursor"])
	}
	prepared := normalize.PrepareToolsList(result).(map[string]any)
	tools, _ := prepared["tools"].([]any)
	if len(tools) != 42 {
		t.Fatalf("tool count %d", len(tools))
	}
	seen := map[string]bool{}
	for _, tool := range tools {
		tm := tool.(map[string]any)
		name, _ := tm["name"].(string)
		title, _ := tm["title"].(string)
		desc, _ := tm["description"].(string)
		if name == "" || title == "" || desc == "" {
			t.Fatalf("tool missing name/title/description: %#v", tm["name"])
		}
		if seen[name] {
			t.Fatalf("duplicate tool %s", name)
		}
		seen[name] = true
		if _, ok := tm["outputSchema"]; ok {
			t.Fatalf("%s has outputSchema", name)
		}
		exec, _ := tm["execution"].(map[string]any)
		if exec["taskSupport"] != "forbidden" {
			t.Fatalf("%s execution %#v", name, tm["execution"])
		}
		schema, ok := tm["inputSchema"].(map[string]any)
		if !ok || schema["type"] != "object" {
			t.Fatalf("%s inputSchema %#v", name, tm["inputSchema"])
		}
		assertNoSDKArtifacts(t, name, schema)
	}
	golden.Compare(t, "golden/mcp/tools-list.json", prepared)
	// The raw capture is the SSE data line before schema canonicalization.
	// Masking drops volatile values; it does not drop $schema or empty properties.
	golden.CompareRaw(t, "golden/mcp/tools-list.raw.json", res.RawPayload)
}

func assertNoSDKArtifacts(t *testing.T, tool string, v any) {
	t.Helper()
	switch n := v.(type) {
	case map[string]any:
		if _, ok := n["$schema"]; ok {
			t.Fatalf("%s still has $schema", tool)
		}
		for _, key := range []string{"minimum", "maximum"} {
			if isSafeBound(n[key]) {
				t.Fatalf("%s kept safe-integer %s", tool, key)
			}
		}
		for _, child := range n {
			assertNoSDKArtifacts(t, tool, child)
		}
	case []any:
		for _, child := range n {
			assertNoSDKArtifacts(t, tool, child)
		}
	}
}

func isSafeBound(v any) bool {
	switch n := v.(type) {
	case json.Number:
		return n.String() == "9007199254740991" || n.String() == "-9007199254740991"
	case float64:
		return n == 9007199254740991 || n == -9007199254740991
	default:
		return false
	}
}
