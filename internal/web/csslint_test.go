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
	for _, token := range []string{"var(--font-ui)", "var(--font-display)", "var(--lh-he-body)", "var(--lh-he-display)", "padding-block: .1em", ".htmx-settling", ".htmx-added", "view-transition-name: page-main", ".rail > .rail-brand"} {
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

func TestColorLintFixtures(t *testing.T) {
	// ac-2: each of these must fail, and the finding must name the rule.
	bad := []struct {
		name, file, css, rule string
	}{
		{"--go outside feedback", "base.css", ".card { color: var(--go); }", lintFeedbackToken},
		{"--go-soft outside feedback", "base.css", ".ladder li { background: var(--go-soft); }", lintFeedbackToken},
		{"--stop outside feedback", "base.css", ".btn { border-color: var(--stop); }", lintFeedbackToken},
		{"--stop-soft in a mixed list", "base.css", ".feedback-wrong, .grade { background: var(--stop-soft); }", lintFeedbackToken},
		{"hex colour in base.css", "base.css", ".card { color: #ff0000; }", lintColorLiteral},
		{"short hex", "components.css", ".c-chip { background: #fff; }", lintColorLiteral},
		{"rgb()", "base.css", ".a { color: rgb(0 0 0 / .5); }", lintColorLiteral},
		{"hsl()", "base.css", ".a { color: hsl(10 20% 30%); }", lintColorLiteral},
		{"oklch()", "base.css", ".a { color: oklch(50% .1 20); }", lintColorLiteral},
		{"blue primary", "base.css", ".btn-primary { background: var(--primary); }", lintLegacyToken},
		{"on-primary", "base.css", ".btn-primary { color: var(--on-primary); }", lintLegacyToken},
		{"check tone", "base.css", ".x { color: var(--check-soft); }", lintLegacyToken},
		{"note tone", "base.css", ".x { color: var(--note); }", lintLegacyToken},
		{"per-grade tone selector", "base.css", `.grade[data-tone="go"] { background: var(--surface); }`, lintGradeColor},
		{"grade with lamp fill", "base.css", ".grade { background: var(--lamp); }", lintGradeColor},
	}
	for _, c := range bad {
		res := lintColors(c.file, c.css, nil)
		found := false
		for _, v := range res.Violations {
			if strings.Contains(v, "["+c.rule+"]") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want a %s finding, got %v", c.name, c.rule, res.Violations)
		}
	}
	good := []struct{ name, file, css string }{
		{"--go in a .feedback- rule", "base.css", ".feedback--right { color: var(--go); }"},
		{"--stop in a .feedback- rule", "base.css", ".feedback--wrong { color: var(--stop); }"},
		{"--go in a correct state", "components.css", `.c-answer[data-state="correct"] { background: var(--go-soft); border-color: var(--go); }`},
		{"--stop in a wrong state, unquoted", "components.css", ".c-answer[data-state=wrong] .c-answer__mark { color: var(--stop); }"},
		{"selector list of feedback rules", "base.css", `.feedback--right, .x[data-state="correct"] { color: var(--go); }`},
		{"hex in tokens.css", "tokens.css", ":root { --paper: #f5f0e7; --go: #0e7a55; --x: oklch(50% 0 0); }"},
		{"hex inside a comment", "base.css", "/* #ff0000 */ .a { color: var(--text); }"},
		{"hex-like text in a string", "base.css", `.a::after { content: "#fff"; }`},
		{"colour roles", "base.css", ".a { color: var(--text); background: color-mix(in oklch, var(--lamp) 18%, transparent); }"},
		{"grade with neutral roles", "base.css", ".grade { background: var(--surface-soft); color: var(--text); border: 1px solid var(--hairline-strong); }"},
		{"action roles", "base.css", ".btn-primary { background: var(--action-bg); color: var(--action-text); }"},
	}
	for _, c := range good {
		if res := lintColors(c.file, c.css, nil); len(res.Violations) != 0 {
			t.Errorf("%s: unexpected findings %v", c.name, res.Violations)
		}
	}
}

func TestColorLintAllowlistIsExact(t *testing.T) {
	allow := []colorLintAllow{{Rule: lintColorLiteral, File: "base.css", Selector: ".gbtn", Count: 2, Why: "test"}}
	res := lintColors("base.css", ".gbtn { --a: #fff; --b: #000; } .gbtn:hover { --c: #123; }", allow)
	if len(res.Violations) != 1 || res.Allowed[0] != 2 {
		t.Fatalf("a third use must be a violation: allowed %d, %v", res.Allowed[0], res.Violations)
	}
	res = lintColors("components.css", ".gbtn { --a: #fff; }", allow)
	if len(res.Violations) != 1 {
		t.Fatalf("the entry is file-specific: %v", res.Violations)
	}
	res = lintColors("base.css", ".card { --a: #fff; }", allow)
	if len(res.Violations) != 1 {
		t.Fatalf("the entry is selector-specific: %v", res.Violations)
	}
}

func TestParseCSSRules(t *testing.T) {
	css := `@font-face { font-family: "A"; src: url("x.woff2"); }
:root[data-theme="dark"] .a, .b { color: var(--text); }
@media (min-width: 1024px) { .c { margin-inline: 0; } @supports (x: y) { .d { color: var(--muted); } } }
@keyframes k { from { opacity: 0; } to { opacity: 1; } }
@import "x.css";
.e { content: "}{"; }`
	var sels []string
	for _, r := range parseCSSRules(css) {
		sels = append(sels, r.Selector)
	}
	want := []string{"@font-face", `:root[data-theme=dark].a,.b`, ".c", ".d", "@keyframesk", ".e"}
	if strings.Join(sels, "|") != strings.Join(want, "|") {
		t.Fatalf("selectors = %q, want %q", sels, want)
	}
}

// TestShippedCSSColorLint holds the real tree to the colour rules. The only
// allowed exceptions are in colorLintAllowlist, with exact counts.
func TestShippedCSSColorLint(t *testing.T) {
	files, err := filepath.Glob("static/css/*.css")
	if err != nil || len(files) == 0 {
		t.Fatalf("no css files: %v", err)
	}
	allowed := map[int]int{}
	for _, name := range files {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		res := lintColors(name, string(b), colorLintAllowlist)
		for _, v := range res.Violations {
			t.Error(v)
		}
		for i, n := range res.Allowed {
			allowed[i] += n
		}
	}
	for i, a := range colorLintAllowlist {
		if allowed[i] != a.Count {
			t.Errorf("allowlist entry %s %s %q lists %d uses but the tree has %d; the list is exact, so lower it with the code (%s)", a.Rule, a.File, a.Selector, a.Count, allowed[i], a.Why)
		}
	}
}

func TestComponentsCSS(t *testing.T) {
	b, err := os.ReadFile("static/css/components.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(b)
	tokens, err := os.ReadFile("static/css/tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`--([a-z0-9-]+)\s*:`).FindAllStringSubmatch(string(tokens), -1) {
		defined[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`var\(--([a-z0-9-]+)`).FindAllStringSubmatch(css, -1) {
		if !defined[m[1]] {
			t.Errorf("components.css uses undefined token --%s", m[1])
		}
	}
	for _, token := range []string{"min-block-size: var(--size-tap)", "min-block-size: var(--size-cta)", "var(--radius-control)", "var(--radius-pill)", "var(--space-3)"} {
		if !strings.Contains(css, token) {
			t.Errorf("components.css missing %s", token)
		}
	}
	if hits := transitionOffenders(css); len(hits) != 0 {
		t.Errorf("components.css transitions: %v", hits)
	}
	// AnswerOption styles exactly four states.
	states := map[string]bool{}
	for _, m := range regexp.MustCompile(`\.c-answer\[data-state="([a-z]+)"\]`).FindAllStringSubmatch(css, -1) {
		states[m[1]] = true
	}
	for _, s := range []string{"selected", "correct", "wrong"} {
		if !states[s] {
			t.Errorf("components.css has no .c-answer[data-state=%q] rule", s)
		}
	}
	if len(states) != 3 {
		t.Errorf("components.css styles states %v; default is the bare .c-answer, so want exactly selected, correct, wrong", states)
	}
}

// TestFlipCSSScope pins the no-JS disclosure. Hiding the back or stacking
// the faces stays inside @supports selector(::details-content). Movement
// stays inside prefers-reduced-motion: no-preference. The reduce block may
// reset transform, translate, and scale to none.
func TestFlipCSSScope(t *testing.T) {
	css, err := os.ReadFile(filepath.Join("static", "css", "flip.css"))
	if err != nil {
		t.Fatal(err)
	}
	if hits := flipCSSViolations(string(css)); len(hits) != 0 {
		t.Fatalf("flip.css: %s", strings.Join(hits, "; "))
	}
	bad := string(css) + "\n.wcard { transform: rotateY(90deg); }\n"
	if hits := flipCSSViolations(bad); len(hits) == 0 {
		t.Fatal("a rotation outside prefers-reduced-motion: no-preference was allowed")
	}
}

func flipCSSViolations(css string) []string {
	css = cssComment.ReplaceAllString(css, " ")
	var hits []string
	var walk func(s string, supports bool, motion string)
	walk = func(s string, supports bool, motion string) {
		start := 0
		for i := 0; i < len(s); {
			switch s[i] {
			case '"', '\'':
				i = skipCSSString(s, i)
				continue
			case ';':
				start = i + 1
			case '{':
				end := matchCSSBrace(s, i)
				prelude := strings.TrimSpace(s[start:i])
				body := s[i+1 : end]
				fields := strings.Fields(prelude + " ")
				name := strings.ToLower(fields[0])
				switch {
				case name == "@media":
					next := motion
					low := strings.ToLower(prelude)
					if strings.Contains(low, "prefers-reduced-motion") {
						if strings.Contains(low, "no-preference") {
							next = "no-preference"
						} else if strings.Contains(low, "reduce") {
							next = "reduce"
						}
					}
					walk(body, supports, next)
				case name == "@supports":
					next := supports
					low := strings.ToLower(prelude)
					if strings.Contains(low, "selector(::details-content)") && !strings.Contains(low, "not") {
						next = true
					}
					walk(body, next, motion)
				case strings.HasPrefix(name, "@"):
				default:
					flipCSSRule(prelude, body, supports, motion, &hits)
				}
				i = end + 1
				start = i
				continue
			}
			i++
		}
	}
	walk(css, false, "")
	return hits
}

func flipCSSRule(sel, body string, supports bool, motion string, hits *[]string) {
	for _, decl := range strings.Split(body, ";") {
		decl = strings.TrimSpace(decl)
		if decl == "" || !strings.Contains(decl, ":") {
			continue
		}
		prop, val, _ := strings.Cut(decl, ":")
		prop = strings.ToLower(strings.TrimSpace(prop))
		val = strings.TrimSpace(val)
		low := strings.ToLower(val)
		if (prop == "opacity" && low == "0") || (prop == "visibility" && strings.Contains(low, "hidden")) || prop == "grid-area" {
			if !supports {
				*hits = append(*hits, sel+" "+prop+" outside @supports selector(::details-content)")
			}
		}
		moves := prop == "transform" || prop == "translate" || prop == "scale" || prop == "rotate" ||
			strings.Contains(low, "rotate") || strings.Contains(low, "translate") || strings.Contains(low, "scale(")
		if !moves || motion == "no-preference" {
			continue
		}
		if motion == "reduce" && low == "none" && (prop == "transform" || prop == "translate" || prop == "scale" || prop == "rotate") {
			continue
		}
		*hits = append(*hits, sel+" "+prop+": "+val+" outside prefers-reduced-motion: no-preference")
	}
}
