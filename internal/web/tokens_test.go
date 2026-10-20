package web

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/tokens"
)

const regenerateHint = `regenerate with: go run ./cmd/tokens
or in Docker (PowerShell):
  docker run --rm -v hebrew-gomod:/go/pkg/mod -v hebrew-gobin:/go/bin -v ${PWD}:/src -w /src golang:1.26 go run ./cmd/tokens`

func readText(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// TestTokensCSSIsGenerated is ac-1. tokens.css is the generator's output for
// lamplight.tokens.json plus the static section. A hand edit, or an export
// that was not regenerated, fails here.
func TestTokensCSSIsGenerated(t *testing.T) {
	data, err := os.ReadFile("static/css/lamplight.tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	want, err := tokens.Build(data)
	if err != nil {
		t.Fatal(err)
	}
	have := readText(t, "static/css/tokens.css")
	if have == want {
		return
	}
	hl, wl := strings.Split(have, "\n"), strings.Split(want, "\n")
	line := 0
	for line < len(hl) && line < len(wl) && hl[line] == wl[line] {
		line++
	}
	var got, exp string
	if line < len(hl) {
		got = hl[line]
	}
	if line < len(wl) {
		exp = wl[line]
	}
	t.Fatalf("static/css/tokens.css differs from the generator output at line %d.\n  committed: %s\n  generated: %s\ntokens.css is generated; do not edit it by hand.\n%s", line+1, got, exp, regenerateHint)
}

// colourRoles reads the generated colour custom properties from tokens.css.
// A colour that is the same in both modes is written once and returns twice.
func colourRoles(t *testing.T) map[string][2]string {
	t.Helper()
	css := readText(t, "static/css/tokens.css")
	re := regexp.MustCompile(`(?m)^\s*(--[a-z0-9-]+):\s*(?:light-dark\((#[0-9a-f]{6,8}),\s*(#[0-9a-f]{6,8})\)|(#[0-9a-f]{6,8}));`)
	roles := map[string][2]string{}
	for _, m := range re.FindAllStringSubmatch(css, -1) {
		if m[4] != "" {
			roles[m[1]] = [2]string{m[4], m[4]}
		} else {
			roles[m[1]] = [2]string{m[2], m[3]}
		}
	}
	return roles
}

// contrastPair is a text colour on a background, named by Figma variable.
type contrastPair struct{ fg, bg string }

func (p contrastPair) String() string { return p.fg + " on " + p.bg }

// contrastPairs are the required pairs, all at WCAG AA for normal text (4.5:1)
// in both modes.
var contrastPairs = []contrastPair{
	{"text/primary", "bg/paper"},
	{"text/primary", "bg/surface"},
	{"text/primary", "bg/surface-soft"},
	{"text/primary", "bg/raised"},
	{"text/muted", "bg/paper"},
	{"text/muted", "bg/surface"},
	{"action/text", "action/bg"},
	{"text/on-slate", "bg/slate"},
	{"feedback/go", "feedback/go-soft"},
	{"feedback/stop", "feedback/stop-soft"},
}

// knownFailures lists pairs whose Figma values genuinely fall below 4.5:1.
// The values are not changed here: Figma owns them. Each entry needs a reason,
// and a listed pair that passes fails the test, so the list cannot go stale.
var knownFailures = map[string]string{}

// TestContrastPairs is ac-4. It reads the generated roles, so it checks what
// the browser gets, not the export.
func TestContrastPairs(t *testing.T) {
	roles := colourRoles(t)
	seen := map[string]bool{}
	for _, p := range contrastPairs {
		p := p
		t.Run(strings.ReplaceAll(p.String(), "/", "_"), func(t *testing.T) {
			fg, bg := roles[tokens.CSSName(p.fg)], roles[tokens.CSSName(p.bg)]
			if fg == ([2]string{}) || bg == ([2]string{}) {
				t.Fatalf("no generated role for %s", p)
			}
			var ratios [2]float64
			failed := false
			for i, mode := range []string{"light", "dark"} {
				r, err := tokens.Contrast(fg[i], bg[i])
				if err != nil {
					t.Fatalf("%s %s: %v", p, mode, err)
				}
				ratios[i] = r
				t.Logf("%-5s %-34s %s on %s = %.2f:1", mode, p, fg[i], bg[i], r)
				if r < 4.5 {
					failed = true
				}
			}
			reason, known := knownFailures[p.String()]
			seen[p.String()] = true
			switch {
			case failed && known:
				t.Logf("known failure (%s): light %.2f, dark %.2f", reason, ratios[0], ratios[1])
			case failed:
				t.Errorf("%s is below 4.5:1: light %.2f, dark %.2f", p, ratios[0], ratios[1])
			case known:
				t.Errorf("%s is listed in knownFailures but passes (light %.2f, dark %.2f); remove the entry", p, ratios[0], ratios[1])
			}
		})
	}
	for name := range knownFailures {
		if !seen[name] {
			t.Errorf("knownFailures lists %q, which is not a checked pair", name)
		}
	}
}

// TestTokensNoSecondActionColour keeps the action colour single: the old blue
// primary and the grade tones must not come back as custom properties.
func TestTokensNoSecondActionColour(t *testing.T) {
	css := readText(t, "static/css/tokens.css")
	for _, gone := range []string{"--primary", "--on-primary", "--check", "--note"} {
		if regexp.MustCompile(regexp.QuoteMeta(gone) + `[a-z-]*\s*:`).MatchString(css) {
			t.Errorf("tokens.css still defines %s*", gone)
		}
	}
	for _, want := range []string{"--action-bg:", "--action-text:", "--focus-ring:", "--size-tap:", "--size-cta:", "--radius-control:", "--space-8:"} {
		if !strings.Contains(css, want) {
			t.Errorf("tokens.css missing %s", want)
		}
	}
	for name, figma := range tokens.FigmaNames() {
		if !strings.Contains(css, fmt.Sprintf("%s:", name)) {
			t.Errorf("tokens.css has no %s for Figma %s", name, figma)
		}
	}
}
