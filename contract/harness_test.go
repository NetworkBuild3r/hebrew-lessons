package contract

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/mcp"
)

func TestMain(m *testing.M) {
	// The runner also exports TZ=UTC before the server starts. Pin it here so
	// a direct go test cannot drift.
	if err := os.Setenv("TZ", "UTC"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	flag.Parse()
	for _, key := range []string{"CONTRACT_WEB_URL", "CONTRACT_MCP_URL", "CONTRACT_MCP_TOKEN", "CONTRACT_MCP_PUBLIC_URL"} {
		if os.Getenv(key) == "" {
			fmt.Fprintf(os.Stderr, "missing %s\nRun contract/run.sh, which boots the server and exports the CONTRACT_* variables.\n", key)
			os.Exit(2)
		}
	}
	os.Exit(m.Run())
}

func mcpClient(t *testing.T) *mcp.Client {
	t.Helper()
	return mcp.New(os.Getenv("CONTRACT_MCP_URL"), os.Getenv("CONTRACT_MCP_TOKEN"))
}

func mcpURL(t *testing.T) string {
	t.Helper()
	return os.Getenv("CONTRACT_MCP_URL")
}

func publicURL(t *testing.T) string {
	t.Helper()
	return strings.TrimRight(os.Getenv("CONTRACT_MCP_PUBLIC_URL"), "/")
}

func originOf(t *testing.T, raw string) string {
	t.Helper()
	origin, err := mcp.Origin(raw)
	if err != nil {
		t.Fatal(err)
	}
	return origin
}

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func authHeaders(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		"Authorization": "Bearer " + os.Getenv("CONTRACT_MCP_TOKEN"),
		"Content-Type":  "application/json",
		"Accept":        "application/json, text/event-stream",
	}
}
