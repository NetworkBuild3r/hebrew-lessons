package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhysicalPropertyScanner(t *testing.T) {
	bad := []string{
		"a{margin-left:1px}",
		"a{padding-right: 4px}",
		"a{left:0}",
		"a{text-align:right}",
		"a{float: left}",
		"a{border-top-left-radius: 4px}",
	}
	for _, css := range bad {
		if hits := physicalProperties(css); len(hits) == 0 {
			t.Errorf("expected a hit in %s", css)
		}
	}
	good := []string{
		"a{margin-inline-start:1px}",
		"a{padding-inline: 4px}",
		"a{inset-inline: 0}",
		"a{text-align: start}",
		"a{float: inline-start}",
		"a{border-start-start-radius: 4px}",
		"a{color: /* margin-left: 1px */ oklch(50% 0.1 20)}",
		`a{font-family:"left"}`,
	}
	for _, css := range good {
		if hits := physicalProperties(css); len(hits) != 0 {
			t.Errorf("%s flagged %v", css, hits)
		}
	}
}

func TestShippedCSSHasNoPhysicalProperties(t *testing.T) {
	matches, err := filepath.Glob("static/css/*.css")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no css files")
	}
	for _, name := range matches {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		hits := physicalProperties(string(b))
		if len(hits) != 0 {
			t.Errorf("%s uses physical left/right: %s", name, strings.Join(hits, "; "))
		}
		if strings.Contains(string(b), "http://") || strings.Contains(string(b), "https://") {
			t.Errorf("%s has an external URL", name)
		}
	}
	base, err := os.ReadFile("static/css/base.css")
	if err != nil {
		t.Fatal(err)
	}
	text := string(base)
	for _, name := range []string{"Noto", "Frank", "Arial", "Georgia", "system-ui"} {
		if strings.Contains(text, name) {
			t.Errorf("base.css names a font family (%s); names belong in tokens.css", name)
		}
	}
	if strings.Contains(text, "overflow:hidden") || strings.Contains(text, "overflow: hidden") {
		t.Error("base.css hides overflow; pointed text must stay visible")
	}
	for _, token := range []string{"var(--font-ui)", "var(--font-display)", "var(--lh-he-body)", "var(--lh-he-display)", "padding-block: .1em"} {
		if !strings.Contains(text, token) {
			t.Errorf("base.css missing %s", token)
		}
	}
}
