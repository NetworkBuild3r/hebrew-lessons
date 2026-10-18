package contract

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/golden"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/seed"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/sqlite"
)

func TestMain(m *testing.M) {
	// The runner also exports TZ=UTC before the server starts. Pin it here so
	// a direct go test cannot drift.
	if err := os.Setenv("TZ", "UTC"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	flag.Parse()
	for _, key := range []string{"CONTRACT_WEB_URL", "CONTRACT_MCP_URL", "CONTRACT_MCP_TOKEN", "CONTRACT_MCP_PUBLIC_URL", "CONTRACT_DATA_DIR"} {
		if os.Getenv(key) == "" {
			fmt.Fprintf(os.Stderr, "missing %s\nRun contract/run.sh, which boots the server and exports the CONTRACT_* variables.\n", key)
			os.Exit(2)
		}
	}
	dataDir := os.Getenv("CONTRACT_DATA_DIR")
	dbPath := filepath.Join(dataDir, "hebrew.sqlite")
	var err error
	schemaAfterBoot, err = sqlite.Dump(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "schema after boot: %v\n", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	err = seed.Run(ctx, seed.Options{
		WebURL:    os.Getenv("CONTRACT_WEB_URL"),
		MCPURL:    os.Getenv("CONTRACT_MCP_URL"),
		Token:     os.Getenv("CONTRACT_MCP_TOKEN"),
		AdminUser: os.Getenv("CONTRACT_ADMIN_USER"),
		AdminPass: os.Getenv("CONTRACT_ADMIN_PASS"),
		DataDir:   dataDir,
		UpdateDB:  golden.Update(),
	})
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		printLogTail(filepath.Join(dataDir, "server.log"))
		os.Exit(1)
	}
	schemaAfterSeed, err = sqlite.Dump(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "schema after seed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func printLogTail(path string) {
	body, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(body), "\n")
	if len(lines) > 80 {
		lines = lines[len(lines)-80:]
	}
	fmt.Fprintf(os.Stderr, "--- server.log (tail) ---\n%s\n", strings.Join(lines, "\n"))
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
