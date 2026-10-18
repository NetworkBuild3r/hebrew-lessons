package contract

import (
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/golden"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/normalize"
)

func TestInitialize(t *testing.T) {
	res, result := initialize(t, "2025-11-25")
	assertSSE(t, res)
	golden.Compare(t, "golden/mcp/initialize.json", normalize.PrepareInitialize(result))
}

func TestInitializeNegotiatesProtocolVersion(t *testing.T) {
	_, echoed := initialize(t, "2025-06-18")
	if echoed["protocolVersion"] != "2025-06-18" {
		t.Fatalf("supported version was not echoed: %#v", echoed["protocolVersion"])
	}
	_, fallback := initialize(t, "2099-01-01")
	if fallback["protocolVersion"] != "2025-11-25" {
		t.Fatalf("unsupported version: %#v", fallback["protocolVersion"])
	}
	for _, result := range []map[string]any{echoed, fallback} {
		info, _ := result["serverInfo"].(map[string]any)
		if info["name"] != "hebrew-lessons" || info["version"] != "1.0.0" {
			t.Fatalf("serverInfo %#v", info)
		}
		caps, _ := result["capabilities"].(map[string]any)
		tools, _ := caps["tools"].(map[string]any)
		if len(caps) != 1 || tools["listChanged"] != true {
			t.Fatalf("capabilities %#v", caps)
		}
	}
}

func initialize(t *testing.T, version string) (*mcp.Result, map[string]any) {
	t.Helper()
	res, err := mcpClient(t).Call(testCtx(t), 1, "initialize", map[string]any{
		"protocolVersion": version,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "contract-harness", "version": "0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := res.RPCResult()
	if !ok {
		t.Fatalf("initialize failed: %s", res.Body)
	}
	result, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("result %#v", raw)
	}
	return res, result
}
