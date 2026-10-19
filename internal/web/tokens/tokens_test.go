package tokens

import (
	"math"
	"os"
	"strings"
	"testing"
)

const sample = `{
  "collection": "Lamplight",
  "source": {},
  "modes": ["light", "dark"],
  "tokens": [%s]
}`

func exportFile(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../static/css/lamplight.tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseExport(t *testing.T) {
	s, err := Parse(exportFile(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Tokens) != 36 || len(s.Tokens) != len(cssNames) {
		t.Fatalf("tokens = %d, names = %d, want 36", len(s.Tokens), len(cssNames))
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	a, err := Build(exportFile(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(exportFile(t))
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("two builds differ")
	}
	if !strings.HasPrefix(a, "/* GENERATED FILE. DO NOT EDIT.") {
		t.Fatal("missing generated header")
	}
	for _, want := range []string{
		"--paper: light-dark(#f5f0e7, #090d1a); /* bg/paper */",
		"--action-bg: light-dark(#10153a, #f2b84b); /* action/bg */",
		"--lamp: #f2b84b; /* accent/lamp */",
		"--hairline: light-dark(#151a2e1a, #eceef614); /* line/hairline */",
		"--radius-pill: 999px; /* radius/pill */",
		"--space-8: 64px; /* space/8 */",
		"--size-cta: 56px; /* size/cta */",
		"color-scheme: light dark;",
		`:root[data-theme="dark"]  { color-scheme: dark; }`,
	} {
		if !strings.Contains(a, want) {
			t.Errorf("output missing %q", want)
		}
	}
	if strings.Contains(a, "\r") {
		t.Error("output has a carriage return")
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]string{
		"unnamed token":   `{"name":"bg/new","type":"color","light":"#000000","dark":"#ffffff"}`,
		"bad colour":      `{"name":"bg/paper","type":"color","light":"#FFF","dark":"#ffffff"}`,
		"uppercase":       `{"name":"bg/paper","type":"color","light":"#F5F0E7","dark":"#ffffff"}`,
		"mode-varying px": `{"name":"space/1","type":"number","unit":"px","light":4,"dark":5}`,
		"missing unit":    `{"name":"space/1","type":"number","light":4,"dark":4}`,
		"unknown type":    `{"name":"space/1","type":"string","light":"4","dark":"4"}`,
	}
	for name, tok := range cases {
		if _, err := Parse([]byte(strings.Replace(sample, "%s", tok, 1))); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestContrastMath(t *testing.T) {
	r, err := Contrast("#000000", "#ffffff")
	if err != nil || math.Abs(r-21) > 1e-9 {
		t.Fatalf("black on white = %v, %v; want 21", r, err)
	}
	r, err = Contrast("#777777", "#ffffff")
	if err != nil || math.Abs(r-4.478) > 0.01 {
		t.Fatalf("#777 on white = %v, %v; want about 4.48", r, err)
	}
	if _, err := Contrast("#151a2e1a", "#ffffff"); err == nil {
		t.Fatal("alpha colour should be rejected")
	}
}
