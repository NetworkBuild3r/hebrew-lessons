package app

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestOnePlacementEngine guards a merge bug found on 2026-10-19. Two branches each wired a placement
// engine: the placement screens built the real one, and Today built a second one with no evidence
// recorder. Engine.ready() refuses to run without a recorder, so the second engine's RecheckDue and
// NewRecheck always failed and the 28-day check-in never showed. Today starts a re-check and
// /placement resumes it, so both must hold the same engine, and it must be built once.
func TestOnePlacementEngine(t *testing.T) {
	src, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, line := range strings.Split(string(src), "\n") {
		code := strings.TrimSpace(line)
		if strings.HasPrefix(code, "//") {
			continue
		}
		if strings.Contains(code, "placement.NewEngine(") {
			calls = append(calls, code)
		}
	}
	if len(calls) != 1 {
		t.Fatalf("app.go builds the placement engine %d times, want exactly 1:\n%s", len(calls), strings.Join(calls, "\n"))
	}
	if strings.Contains(calls[0], "nil") {
		t.Fatalf("the placement engine is built without an evidence recorder, so every call fails: %s", calls[0])
	}
	// Both the placement screens (Deck) and Today (Placement) take that one engine.
	for _, field := range []string{"Deck", "Placement"} {
		re := regexp.MustCompile(`(?m)^\s*` + field + `:\s+a\.placementEngine,`)
		if !re.Match(src) {
			t.Fatalf("web.Options.%s is not set to a.placementEngine", field)
		}
	}
}
