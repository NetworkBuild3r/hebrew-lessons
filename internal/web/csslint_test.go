package web

import (
	"os"
	"path/filepath"
	"regexp"
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
	for _, token := range []string{"var(--font-ui)", "var(--font-display)", "var(--lh-he-body)", "var(--lh-he-display)", "padding-block: .1em", ".htmx-settling", ".htmx-added", "view-transition-name: page-main", ".rail > .rail-brand", ".rail > .rail-theme"} {
		if !strings.Contains(text, token) {
			t.Errorf("base.css missing %s", token)
		}
	}
	if strings.Contains(text, ".htmx-swapping") {
		t.Error("base.css still fades the swap target out; that blanks the page between tabs")
	}
	for _, name := range []string{"today-due", "today-next", "he-display", "shell-dock", "activity"} {
		if strings.Contains(text, "view-transition-name: "+name) {
			t.Errorf("base.css still names %s, so chrome or cards leave the root snapshot", name)
		}
	}
	if !strings.Contains(text, "::view-transition-old(root)") || !strings.Contains(text, "animation: none") {
		t.Error("root view transition should stay still")
	}
	if hits := transitionOffenders(text); len(hits) != 0 {
		t.Errorf("base.css transitions a property other than transform or opacity: %s", strings.Join(hits, "; "))
	}
	tokens, err := os.ReadFile("static/css/tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`--([a-z0-9-]+)\s*:`).FindAllStringSubmatch(string(tokens), -1) {
		defined[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`var\(--([a-z0-9-]+)`).FindAllStringSubmatch(text, -1) {
		// --gbtn-* are Google's required brand colours. They live on .gbtn,
		// not in tokens.css, and they are the one exception.
		if strings.HasPrefix(m[1], "gbtn-") {
			continue
		}
		if !defined[m[1]] {
			t.Errorf("base.css uses undefined token --%s", m[1])
		}
	}
	for _, hex := range []string{"#FFFFFF", "#747775", "#1F1F1F", "#131314", "#8E918F", "#E3E3E3"} {
		if !strings.Contains(text, hex) {
			t.Errorf("Google button missing brand colour %s", hex)
		}
	}
}

func TestTransitionLint(t *testing.T) {
	good := []string{
		"a{transition-property: opacity, transform}",
		"a{transition-property: transform}",
		"a{transition: none}",
		"a{transition: opacity var(--dur-base) var(--ease-out), transform var(--dur-base) cubic-bezier(.2, .8, .2, 1)}",
		"a{color: /* transition: color 1s */ transparent}",
	}
	for _, css := range good {
		if hits := transitionOffenders(css); len(hits) != 0 {
			t.Errorf("%s flagged %v", css, hits)
		}
	}
	bad := []string{
		"a{transition: color 200ms}",
		"a{transition: all 200ms}",
		"a{transition: 200ms}",
		"a{transition-property: margin}",
		"a{transition-property: opacity, background-color}",
	}
	for _, css := range bad {
		if hits := transitionOffenders(css); len(hits) == 0 {
			t.Errorf("expected a hit in %s", css)
		}
	}
}
