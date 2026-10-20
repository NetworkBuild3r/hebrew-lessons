package web

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readStatic(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func hasFinding(res motionLintResult, rule string) bool {
	for _, v := range res.Violations {
		if strings.Contains(v, "["+rule+"]") {
			return true
		}
	}
	return false
}

// TestMotionLintFixtures is ac-1 and ac-2 on small inputs: what fails and
// what passes.
func TestMotionLintFixtures(t *testing.T) {
	bad := []struct{ name, css, rule string }{
		{"transform animation outside the gate", "@keyframes k{from{transform:none}to{transform:scale(2)}} .a{animation: k var(--dur) var(--ease-out)}", lintMotionGate},
		{"animation-name outside the gate", "@keyframes k{to{transform:scale(2)}} .a{animation-name: k}", lintMotionGate},
		{"unknown keyframes outside the gate", ".a{animation: nope var(--dur)}", lintMotionGate},
		{"transform transition outside the gate", ".a{transition: transform var(--dur) var(--ease-out)}", lintMotionGate},
		{"colour transition outside the gate", ".a{transition-property: opacity, color}", lintMotionGate},
		{"transition on all", ".a{transition: all var(--dur)}", lintMotionGate},
		{"loop outside the gate", "@keyframes k{to{opacity:0}} .a{animation: k var(--dur) infinite}", lintMotionLoop},
		{"loop count outside the gate", ".a{animation-iteration-count: infinite}", lintMotionLoop},
		{"movement in reduce", "@media (prefers-reduced-motion: reduce){@keyframes k{to{transform:scale(2)}} .a{animation: k var(--dur)}}", lintMotionGate},
		{"layout keyframe in the gate", "@media (prefers-reduced-motion: no-preference){@keyframes k{to{inline-size:0}}}", lintMotionKeyframe},
		{"literal ms", "@media (prefers-reduced-motion: no-preference){.a{transition: transform 200ms}}", lintMotionLiteral},
		{"literal s", "@media (prefers-reduced-motion: no-preference){.a{animation-duration: .2s}}", lintMotionLiteral},
		{"literal delay", "@media (prefers-reduced-motion: no-preference){.a{animation: k var(--dur) var(--ease-out) 40ms}}", lintMotionLiteral},
		{"literal in fallback", "@media (prefers-reduced-motion: no-preference){.a{transition-duration: var(--dur, 200ms)}}", lintMotionLiteral},
	}
	for _, c := range bad {
		if res := lintMotion("x.css", c.css, nil); !hasFinding(res, c.rule) {
			t.Errorf("%s: want a %s finding, got %v", c.name, c.rule, res.Violations)
		}
	}
	good := []struct{ name, css string }{
		{"movement inside the gate", "@media (prefers-reduced-motion: no-preference){@keyframes k{to{transform:scale(2)}} .a{animation: k var(--dur) var(--ease-out); transition: transform var(--dur-fast) var(--ease-out)}}"},
		{"gate with another condition", "@media (prefers-reduced-motion: no-preference) and (min-width: 600px){.a{transition: transform var(--dur)}}"},
		{"opacity keyframes anywhere", "@keyframes k{from{opacity:0}to{opacity:1}} .a{animation: k var(--dur-fast) linear both} .b{transition: opacity var(--dur-fast)}"},
		{"none", ".a{animation: none; transition: none}"},
		{"opacity only in reduce", "@media (prefers-reduced-motion: reduce){@keyframes k{from{opacity:0}} .a{animation: k var(--dur-fast) linear}}"},
		{"loop inside the gate", "@media (prefers-reduced-motion: no-preference){@keyframes k{to{transform:scale(2)}} .a{animation: k calc(var(--dur-slow) * 2) var(--ease-in-out) infinite alternate}}"},
		{"no time in the text", ".a{content: \"200ms\"} /* transition: all 1s */"},
		{"token durations", "@media (prefers-reduced-motion: no-preference){.a{animation-duration: var(--dur-base); animation-delay: calc(var(--i, 0) * var(--dur-stagger))}}"},
	}
	for _, c := range good {
		if res := lintMotion("x.css", c.css, nil); len(res.Violations) != 0 {
			t.Errorf("%s: unexpected findings %v", c.name, res.Violations)
		}
	}
}

func TestMotionLintAllowlistIsExact(t *testing.T) {
	allow := []motionAllow{{Rule: lintMotionGate, File: "base.css", Selector: ".old", Count: 1, Why: "test"}}
	css := "@keyframes k{to{transform:scale(2)}} .old{animation: k var(--dur)} .old2{animation: k var(--dur)}"
	res := lintMotion("base.css", css, allow)
	if len(res.Violations) != 1 || res.Allowed[0] != 1 {
		t.Fatalf("the second use must be a violation: allowed %d, %v", res.Allowed[0], res.Violations)
	}
	if res := lintMotion("components.css", ".old{animation: k var(--dur)}", allow); len(res.Violations) != 1 {
		t.Fatalf("the entry is file-specific: %v", res.Violations)
	}
}

// TestShippedCSSMotionLint holds the real tree to the motion rules: ac-1 (no
// movement outside the no-preference gate) and ac-2 (no literal duration).
// The only exceptions are in motionLintAllowlist, with exact counts, and each
// one must have a reduce rule in motion.css.
func TestShippedCSSMotionLint(t *testing.T) {
	files, err := filepath.Glob("static/css/*.css")
	if err != nil || len(files) == 0 {
		t.Fatalf("no css files: %v", err)
	}
	allowed := map[int]int{}
	for _, name := range files {
		res := lintMotion(name, readStatic(t, name), motionLintAllowlist)
		for _, v := range res.Violations {
			t.Error(v)
		}
		for i, n := range res.Allowed {
			allowed[i] += n
		}
	}
	for i, a := range motionLintAllowlist {
		if allowed[i] != a.Count {
			t.Errorf("allowlist entry %s %s %q lists %d uses but the tree has %d; the list is exact, so lower it with the code", a.Rule, a.File, a.Selector, a.Count, allowed[i])
		}
	}
	rules, _ := parseMotionCSS(readStatic(t, "static/css/motion.css"))
	for _, a := range motionLintAllowlist {
		found := false
		for _, r := range rules {
			if r.Ctx == ctxReduce && strings.Contains(r.Selector, a.Override) {
				found = true
			}
		}
		if !found {
			t.Errorf("allowed rule %q in %s has no reduce rule %q in motion.css", a.Selector, a.File, a.Override)
		}
	}
}

// TestDurationTokens is ac-2 for the tokens: none over 400ms, the three named
// steps are right, and reduce is 120ms or less.
func TestDurationTokens(t *testing.T) {
	css := readStatic(t, "static/css/tokens.css")
	normal, reduce := durationTokens(css)
	want := map[string]float64{"--dur-fast": 150, "--dur-base": 250, "--dur-slow": 400}
	for name, ms := range want {
		if normal[name] != ms {
			t.Errorf("%s = %vms, want %vms", name, normal[name], ms)
		}
	}
	if !strings.Contains(css, "--dur: var(--dur-base)") {
		t.Error("--dur is not an alias of --dur-base")
	}
	for name, ms := range normal {
		if ms > 400 {
			t.Errorf("%s is %vms; no duration token may pass 400ms", name, ms)
		}
	}
	if len(reduce) == 0 {
		t.Fatal("no reduce durations")
	}
	for name, ms := range reduce {
		if ms > 120 {
			t.Errorf("%s is %vms under reduce; the limit is 120ms", name, ms)
		}
	}
	for name := range normal {
		if _, ok := reduce[name]; !ok {
			t.Errorf("%s has no reduce value", name)
		}
	}
	for _, token := range []string{"--ease-out:", "--ease-in-out:", "--ease-spring:", "linear(0,", "--motion-press: .98", "--motion-press: 1"} {
		if !strings.Contains(css, token) {
			t.Errorf("tokens.css missing %s", token)
		}
	}
}

// motionPatterns is the named set. Each needs a rule in the no-preference
// block, and one in the reduce block that stands in for it.
var motionPatterns = []struct{ name, noPref, reduce string }{
	{"screen-enter", "::view-transition-new(page-main)", "::view-transition-new(page-main)"},
	{"screen-exit", "::view-transition-old(page-main)", "::view-transition-old(page-main)"},
	{"option-press", ".c-answer:active", ".c-answer:active"},
	{"option-picked", `.c-answer[data-state=selected]`, `.c-answer[data-state=selected]`},
	{"answer-correct", `.c-answer[data-state=correct]`, `.c-answer[data-state=correct]`},
	{"answer-wrong", `.c-answer[data-state=wrong]`, `.c-answer[data-state=wrong]`},
	{"feedback-rise", ".c-sheet{", ".c-sheet{"},
	{"sheet-out", ".c-sheet[data-leaving]", ".c-sheet[data-leaving]"},
	{"card-flip", `[data-motion~=card-flip]`, `[data-motion~=card-flip]`},
	{"progress-fill", ".c-progress::-webkit-progress-value", ".c-progress::-webkit-progress-value"},
	{"orb-pulse", ".c-orb[data-playing]", ".c-orb[data-playing]"},
	{"stagger-in", `[data-motion~=stagger]>*`, `[data-motion~=stagger]>*`},
}

func TestMotionPatternsHaveFallbacks(t *testing.T) {
	css := readStatic(t, "static/css/motion.css")
	rules, frames := parseMotionCSS(css)
	has := func(ctx, frag string) bool {
		for _, r := range rules {
			sel := r.Selector + "{"
			if r.Ctx == ctx && strings.Contains(sel, frag) {
				return true
			}
		}
		return false
	}
	for _, p := range motionPatterns {
		if !has(ctxNoPref, p.noPref) {
			t.Errorf("%s: no rule %q in the no-preference block", p.name, p.noPref)
		}
		if !has(ctxReduce, p.reduce) {
			t.Errorf("%s: no rule %q in the reduce block", p.name, p.reduce)
		}
		// option-press is a transition, so it has no keyframes.
		if _, ok := frames[p.name]; !ok && p.name != "option-press" {
			t.Errorf("%s: no @keyframes of that name", p.name)
		}
	}
	// Every keyframes block moves only transform and opacity.
	if res := lintMotion("motion.css", css, nil); len(res.Violations) != 0 {
		t.Errorf("motion.css: %v", res.Violations)
	}
	if !strings.Contains(css, "@media (prefers-reduced-motion: no-preference)") || !strings.Contains(css, "@media (prefers-reduced-motion: reduce)") {
		t.Error("motion.css needs both a no-preference and a reduce block")
	}
}

var (
	moveProp  = regexp.MustCompile(`(?i)(?:^|[;{}\s])(transform|translate|rotate|scale|perspective|offset-path)\s*:\s*([^;{}]*)`)
	varDur    = regexp.MustCompile(`var\(\s*(--dur[a-z-]*)`)
	durAnimTx = regexp.MustCompile(`(?i)(?:^|[;{}\s])(animation|animation-duration|transition|transition-duration)\s*:\s*([^;{}]*)`)
)

// TestReducedMotionHasNoMovement is the static form of ac-3. Under reduce
// nothing in motion.css sets a transform, every animation is opacity-only (the
// lint above), and every duration resolves to 120ms or less. The browser
// check (motion_browser_test.go, tag browser) measures the real swap.
func TestReducedMotionHasNoMovement(t *testing.T) {
	css := readStatic(t, "static/css/motion.css")
	rules, frames := parseMotionCSS(css)
	_, reduce := durationTokens(readStatic(t, "static/css/tokens.css"))
	n := 0
	for _, r := range rules {
		if r.Ctx != ctxReduce {
			continue
		}
		n++
		for _, m := range moveProp.FindAllStringSubmatch(r.Body, -1) {
			if v := strings.TrimSpace(m[2]); v != "none" && v != "1" {
				t.Errorf("reduce rule %s sets %s: %s", r.Selector, m[1], v)
			}
		}
		for _, m := range durAnimTx.FindAllStringSubmatch(r.Body, -1) {
			for _, d := range varDur.FindAllStringSubmatch(m[2], -1) {
				name := d[1]
				if name == "--dur" {
					name = "--dur-base"
				}
				if ms, ok := reduce[name]; !ok || ms > 120 {
					t.Errorf("reduce rule %s uses %s = %vms (limit 120ms)", r.Selector, name, ms)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no reduce rules")
	}
	for name, f := range frames {
		if strings.HasPrefix(name, "motion-") {
			for p := range f.Props {
				if p != "opacity" {
					t.Errorf("reduce keyframes %s sets %s", name, p)
				}
			}
		}
	}
	// The swap itself: page-main cross-fades with a reduce keyframes name.
	for _, side := range []string{"old", "new"} {
		found := false
		for _, r := range rules {
			if r.Ctx == ctxReduce && strings.Contains(r.Selector, "::view-transition-"+side+"(page-main)") && strings.Contains(r.Body, "motion-fade-") {
				found = true
			}
		}
		if !found {
			t.Errorf("no reduce cross-fade for the %s page snapshot", side)
		}
	}
}

// TestMotionNeverBlocksInput is the static form of ac-4. Nothing in motion.css
// turns off pointer events on content, and nothing in the stylesheet or script
// disables or delays input. The overlay rule is belt and braces only: the
// browser test showed Chromium ignores it, so the guarantee is the
// htmx:beforeTransition handler in motion.js (checked below).
func TestMotionNeverBlocksInput(t *testing.T) {
	css := readStatic(t, "static/css/motion.css")
	rules, _ := parseMotionCSS(css)
	overlay := false
	for _, r := range rules {
		if strings.Contains(r.Body, "pointer-events") {
			if r.Selector == "::view-transition" && regexp.MustCompile(`pointer-events:\s*none`).MatchString(r.Body) {
				overlay = true
				continue
			}
			t.Errorf("%s sets pointer-events; only ::view-transition may", r.Selector)
		}
	}
	if !overlay {
		t.Error("::view-transition must set pointer-events: none so a tap during a swap reaches the control")
	}
	for _, bad := range []string{"inert", " visibility:", "display: none"} {
		if strings.Contains(css, bad) {
			t.Errorf("motion.css contains %q", bad)
		}
	}
	js := readStatic(t, "static/js/motion.js")
	// The one allowed preventDefault cancels a view transition (never an
	// event the user caused). Chromium blocks every tap while a view
	// transition runs, so a swap that brings in answer options must run
	// without one; the browser test proves the first tap registers.
	handler := regexp.MustCompile(`(?s)document\.addEventListener\("htmx:beforeTransition".*?\n  \}\);`)
	loc := handler.FindStringIndex(js)
	if loc == nil {
		t.Fatal("motion.js has no htmx:beforeTransition handler; a swap that brings in answer options would swallow the first tap")
	}
	if !strings.Contains(js[loc[0]:loc[1]], "c-answer") {
		t.Error("the htmx:beforeTransition handler does not look for c-answer")
	}
	js = js[:loc[0]] + js[loc[1]:]
	for _, bad := range []string{"preventDefault", "stopPropagation", ".disabled", "pointer-events", "inert"} {
		if strings.Contains(js, bad) {
			t.Errorf("motion.js contains %q, which can swallow a tap", bad)
		}
	}
	// A pending auto-advance never holds back input: it listens, it does not block.
	for _, n := range []string{"pointerdown", "keydown", "htmx:beforeRequest"} {
		if !strings.Contains(js, `"`+n+`"`) {
			t.Errorf("motion.js does not cancel auto-advance on %s", n)
		}
	}
}

// TestMotionIsNeverTheOnlyCue checks the signs that stay when motion is off.
func TestMotionIsNeverTheOnlyCue(t *testing.T) {
	rules, _ := parseMotionCSS(readStatic(t, "static/css/motion.css"))
	var ring, press bool
	for _, r := range rules {
		if r.Selector == ".c-orb[data-playing]" && r.Ctx == ctxNone && strings.Contains(r.Body, "outline:") {
			ring = true
		}
		if r.Ctx == ctxReduce && strings.Contains(r.Selector, ".c-answer:active") && strings.Contains(r.Body, "opacity:") {
			press = true
		}
	}
	if !ring {
		t.Error("a playing orb needs a static ring outside any motion block")
	}
	if !press {
		t.Error("a pressed option needs an opacity cue under reduce")
	}
	comp := readStatic(t, "static/css/components.css")
	for _, sel := range []string{`.c-answer[data-state="selected"]`, `.c-answer[data-state="correct"]`, `.c-answer[data-state="wrong"]`, ".c-answer__mark"} {
		if !strings.Contains(comp, sel) {
			t.Errorf("components.css lost the static state %s that motion rests on", sel)
		}
	}
}

func TestMotionCSSUsesDefinedTokens(t *testing.T) {
	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`--([a-z0-9-]+)\s*:`).FindAllStringSubmatch(readStatic(t, "static/css/tokens.css"), -1) {
		defined[m[1]] = true
	}
	css := readStatic(t, "static/css/motion.css")
	for _, m := range regexp.MustCompile(`var\(--([a-z0-9-]+)`).FindAllStringSubmatch(css, -1) {
		// --i and --v0/--v are set by the element, not by tokens.css.
		if m[1] == "i" || m[1] == "v0" {
			continue
		}
		if !defined[m[1]] {
			t.Errorf("motion.css uses undefined token --%s", m[1])
		}
	}
	if strings.Contains(css, "http://") || strings.Contains(css, "https://") {
		t.Error("motion.css has an external URL")
	}
}

func TestMotionAssetsAreLinked(t *testing.T) {
	body := get(t, New(nil, nil), "/")
	comp := strings.Index(body, `href="/static/css/components.css"`)
	mot := strings.Index(body, `href="/static/css/motion.css"`)
	if comp < 0 || mot < comp {
		t.Fatal("motion.css must be linked after components.css")
	}
	shell := strings.Index(body, `<script src="/static/js/shell.js" defer></script>`)
	js := strings.Index(body, `<script src="/static/js/motion.js" defer></script>`)
	if shell < 0 || js < shell {
		t.Fatal("motion.js must be a deferred script after shell.js, so it has the last word on globalViewTransitions")
	}
}

// TestMotionJSBudget is aud-2: under 3 KB, no dependency, no inline script,
// and the served file is the file.
func TestMotionJSBudget(t *testing.T) {
	b := readStatic(t, "static/js/motion.js")
	if len(b) == 0 || len(b) > 3072 {
		t.Fatalf("motion.js is %d bytes; the budget is 3072", len(b))
	}
	if body := get(t, New(nil, nil), "/static/js/motion.js"); body != b {
		t.Fatal("served motion.js does not match the file")
	}
	for _, bad := range []string{"eval(", "new Function", "innerHTML", "import ", "require(", "document.write"} {
		if strings.Contains(b, bad) {
			t.Errorf("motion.js uses %s", bad)
		}
	}
	for _, need := range []string{"globalViewTransitions", "prefers-reduced-motion: reduce", "data-playing", `"playing"`, "data-advance", "|| 800"} {
		if !strings.Contains(b, need) {
			t.Errorf("motion.js missing %s", need)
		}
	}
}
