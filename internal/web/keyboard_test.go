package web

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/a-h/templ"
	"golang.org/x/text/unicode/norm"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// SPEC-011. The Hebrew on-screen keyboard has three parts: the key table and
// templ (views/keyboard.*), the script (static/js/hekbd.js), and the style
// (static/css/keyboard.css). These tests check each part, and that the script
// agrees with the table. Node runs the script's own unit tests when it is
// installed (TestKeyboardNode). No browser is used, so nothing visual or
// touch-based is checked here.

type keyboardVectors struct {
	Keys      map[string]string `json:"keys"`
	Normalize []struct {
		Name string `json:"name"`
		In   string `json:"in"`
		Out  string `json:"out"`
	} `json:"normalize"`
}

func loadKeyboardVectors(t *testing.T) keyboardVectors {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "keyboard", "vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v keyboardVectors
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func allKeys() []views.HKey {
	var keys []views.HKey
	for _, row := range views.HebrewKeyRows {
		keys = append(keys, row...)
	}
	return keys
}

// TestKeyboardKeyMap: the table is the Israeli layout, 22 letters and 5 final
// forms, each once, and it matches a layout written out apart from it.
func TestKeyboardKeyMap(t *testing.T) {
	v := loadKeyboardVectors(t)
	keys := allKeys()
	if len(keys) != 27 {
		t.Fatalf("%d keys, want 27 (22 letters + 5 final forms)", len(keys))
	}
	if got := []int{len(views.HebrewKeyRows[0]), len(views.HebrewKeyRows[1]), len(views.HebrewKeyRows[2])}; got[0] != 8 || got[1] != 10 || got[2] != 9 || len(views.HebrewKeyRows) != 3 {
		t.Errorf("row lengths %v, want 8, 10, 9 in three rows", got)
	}
	seenCh, seenCode, seenLatin := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var finals, letters []string
	for _, k := range keys {
		r, size := utf8.DecodeRuneInString(k.Ch)
		if size != len(k.Ch) || r < 0x05D0 || r > 0x05EA {
			t.Errorf("key %q is not one Hebrew letter", k.Ch)
		}
		if seenCh[k.Ch] || seenCode[k.Code] || seenLatin[k.Latin] {
			t.Errorf("key %q repeats a letter, code, or hint", k.Ch)
		}
		seenCh[k.Ch], seenCode[k.Code], seenLatin[k.Latin] = true, true, true
		if k.Name == "" || !norm.NFC.IsNormalString(k.Name) {
			t.Errorf("key %q has no NFC letter name", k.Ch)
		}
		if k.Final {
			finals = append(finals, k.Ch)
		} else {
			letters = append(letters, k.Ch)
		}
		// The hint is the US legend of the physical key.
		wantLatin := strings.ToLower(strings.TrimPrefix(k.Code, "Key"))
		switch k.Code {
		case "Semicolon":
			wantLatin = ";"
		case "Comma":
			wantLatin = ","
		case "Period":
			wantLatin = "."
		}
		if k.Latin != wantLatin {
			t.Errorf("key %q hint %q, but code %s is the %q key", k.Ch, k.Latin, k.Code, wantLatin)
		}
		if v.Keys[k.Code] != k.Ch {
			t.Errorf("code %s types %q in the table, the reference layout says %q", k.Code, k.Ch, v.Keys[k.Code])
		}
	}
	if len(v.Keys) != 27 {
		t.Errorf("reference layout has %d keys, want 27", len(v.Keys))
	}
	if got := strings.Join(finals, ""); got != "ןםךףץ" {
		t.Errorf("final forms %q, want ן ם ך ף ץ each on its own key", got)
	}
	if len(letters) != 22 {
		t.Errorf("%d plain letters, want 22", len(letters))
	}
	// ac-1: with a Latin layout, t, c, and comma type alef, bet, tav.
	byLatin := map[string]string{}
	for _, k := range keys {
		byLatin[k.Latin] = k.Ch
	}
	if got := byLatin["t"] + byLatin["c"] + byLatin[","]; got != "אבת" {
		t.Errorf("t, c, comma type %q, want אבת", got)
	}
}

// TestKeyboardTypedAnswerGrades: a word typed from the keys, then normalised
// as hekbd.js does it, is a Good answer for the pointed word, and a final
// letter is not folded into its plain form.
func TestKeyboardTypedAnswerGrades(t *testing.T) {
	byLatin := map[string]string{}
	for _, k := range allKeys() {
		byLatin[k.Latin] = k.Ch
	}
	typed := byLatin["a"] + byLatin["k"] + byLatin["u"] + byLatin["o"]
	if typed != "שלום" {
		t.Fatalf("a k u o typed %q, want שלום", typed)
	}
	if got := fsrs.GradeTyped("\u05e9\u05b8\u05c1\u05dc\u05d5\u05b9\u05dd", units.Normalize(typed+"  ")); got != fsrs.Good {
		t.Errorf("GradeTyped = %v, want Good", got)
	}
	if got := fsrs.GradeTyped("שלום", "שלומ"); got == fsrs.Good {
		t.Error("a plain mem must not grade as a final mem")
	}
	if byLatin["l"] == byLatin["f"] || byLatin["o"] == byLatin["n"] {
		t.Error("a final form shares a key with its plain form")
	}
}

// TestKeyboardNormalizeVectors: the vectors hekbd.js is tested against are
// what units.Normalize and audio.NormalizeText return, so what the keyboard
// submits is the form the audio package keys on.
func TestKeyboardNormalizeVectors(t *testing.T) {
	v := loadKeyboardVectors(t)
	if len(v.Normalize) < 10 {
		t.Fatalf("%d normalize vectors", len(v.Normalize))
	}
	for _, c := range v.Normalize {
		if got := units.Normalize(c.In); got != c.Out {
			t.Errorf("%s: units.Normalize(%q) = %q, vector says %q", c.Name, c.In, got, c.Out)
		}
		if got := audio.NormalizeText(c.In); got != c.Out {
			t.Errorf("%s: audio.NormalizeText(%q) = %q, vector says %q", c.Name, c.In, got, c.Out)
		}
		if !norm.NFC.IsNormalString(c.Out) {
			t.Errorf("%s: vector output %q is not NFC", c.Name, c.Out)
		}
	}
}

func keyboardCases() []componentCase {
	return []componentCase{
		{"hebrewkeyboard-en", views.HebrewKeyboard("answer", "en"), ""},
		{"hebrewkeyboard-he", views.HebrewKeyboard("answer", "he"), ""},
		{"hebrewinput", views.HebrewInput("answer", "answer", templ.Attributes{"aria-label": "Type the Hebrew word", "required": true}), ""},
	}
}

// TestKeyboardGolden: the keyboard and the input render to the committed
// HTML. hekbd.test.js reads the English golden to build its key map, so a
// change to the markup has to be reviewed here first.
// Rewrite with: go test ./internal/web -run TestKeyboardGolden -update
func TestKeyboardGolden(t *testing.T) {
	dir := filepath.Join("testdata", "keyboard")
	want := map[string]bool{}
	for _, c := range keyboardCases() {
		file := filepath.Join(dir, c.name+".golden.html")
		want[filepath.Base(file)] = true
		got := renderComponent(t, c.c, c.children) + "\n"
		if updating() {
			if err := os.WriteFile(file, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("%v\nrun: go test ./internal/web -run TestKeyboardGolden -update", err)
		}
		if exp := strings.ReplaceAll(string(b), "\r\n", "\n"); got != exp {
			t.Errorf("%s differs from its golden file.\n got: %s\nwant: %s", c.name, got, exp)
		}
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.golden.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if !want[filepath.Base(f)] {
			t.Errorf("%s has no test case; delete it", f)
		}
	}
}

// TestKeyboardMarkup: what a screen reader, a touch, a mouse, and the script
// each rely on is in the rendered HTML.
func TestKeyboardMarkup(t *testing.T) {
	for _, lang := range []string{"en", "he", "fr"} {
		html := renderComponent(t, views.HebrewKeyboard("answer", lang), "")
		if !norm.NFC.IsNormalString(html) {
			t.Errorf("%s: keyboard is not NFC", lang)
		}
		if strings.Contains(html, "style=") || strings.Contains(html, "onclick") || strings.Contains(html, "javascript:") {
			t.Errorf("%s: inline style or handler; CSP is style-src 'self', script-src 'self'", lang)
		}
		for _, part := range []string{
			`href="` + views.KeyboardCSS + `"`,
			`<script src="` + views.KeyboardJS + `" defer></script>`,
			`data-hk `, `data-target="answer"`, `role="group"`, `dir="ltr"`,
			`data-act="back"`, `data-act="space"`, `data-act="enter"`, `data-hk-hints`, `aria-pressed="false"`,
		} {
			if !strings.Contains(html, part) {
				t.Errorf("%s: missing %s", lang, part)
			}
		}
		if got, want := strings.Count(html, "<button"), strings.Count(html, `<button type="button"`); got != want || got != 27+4 {
			t.Errorf("%s: %d buttons, %d with type=button, want 31 (27 letters, delete, space, enter, hints) all type=button", lang, got, want)
		}
		if strings.Count(html, `class="hk-row"`) != 4 {
			t.Errorf("%s: want 4 rows (three letter rows and space/enter)", lang)
		}
	}
	html := renderComponent(t, views.HebrewKeyboard("answer", "en"), "")
	for _, k := range allKeys() {
		key := `data-ch="` + k.Ch + `" data-code="` + k.Code + `"`
		if !strings.Contains(html, key) {
			t.Errorf("key %s missing its data-ch and data-code", k.Ch)
		}
		if !strings.Contains(html, `aria-label="`+k.Name+`"`) {
			t.Errorf("key %s has no accessible label %q", k.Ch, k.Name)
		}
		if !strings.Contains(html, `<span class="hk-key__cap" aria-hidden="true">`+k.Ch+`</span>`) {
			t.Errorf("key %s: the drawn letter is not hidden from assistive tech", k.Ch)
		}
		if !strings.Contains(html, `aria-hidden="true" lang="en" dir="ltr">`+k.Latin+`</span>`) {
			t.Errorf("key %s: the hint %q is not a hidden Latin span", k.Ch, k.Latin)
		}
	}
	if n := strings.Count(html, `lang="he" aria-label=`); n != 27 {
		t.Errorf("%d letter keys carry lang=\"he\" with a Hebrew name, want 27", n)
	}
	if n := strings.Count(html, "data-final"); n != 5 {
		t.Errorf("%d keys marked final, want 5", n)
	}
	he := renderComponent(t, views.HebrewKeyboard("answer", "he"), "")
	for _, part := range []string{`aria-label="מקלדת עברית"`, `aria-hidden="true">מחק</span>`, ">רווח<", ">שלח<"} {
		if !strings.Contains(he, part) {
			t.Errorf("Hebrew controls missing %s", part)
		}
	}
	if !strings.Contains(html, `aria-label="Hebrew keyboard"`) || !strings.Contains(html, ">Show key hints<") {
		t.Errorf("English controls missing: %s", html)
	}
}

func TestHebrewInputMarkup(t *testing.T) {
	html := renderComponent(t, views.HebrewInput("answer", "typed", templ.Attributes{"aria-label": "Type it", "required": true}), "")
	for _, part := range []string{
		`type="text"`, `id="answer"`, `name="typed"`, `class="hk-input"`, `dir="rtl"`, `lang="he"`,
		`autocomplete="off"`, `autocapitalize="off"`, `autocorrect="off"`, `spellcheck="false"`, `data-hk-input`,
		`aria-label="Type it"`, ` required`,
	} {
		if !strings.Contains(html, part) {
			t.Errorf("input missing %s in %s", part, html)
		}
	}
	// inputmode=none is set by the script on a touch screen only. Setting it
	// here would take the system keyboard away from a desktop learner.
	if strings.Contains(html, "inputmode") {
		t.Errorf("the server must not set inputmode: %s", html)
	}
	if !strings.Contains(renderComponent(t, views.HebrewInput("a", "b", nil), ""), `dir="rtl"`) {
		t.Error("a nil attrs input is not rtl")
	}
}

// TestKeyboardCSS: the 44px rule and the 360px rule, as far as a stylesheet can
// be checked without a browser, plus the same lint the other sheets get.
func TestKeyboardCSS(t *testing.T) {
	b, err := os.ReadFile("static/css/keyboard.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(b)
	tokens := readText(t, "static/css/tokens.css")
	defined := map[string]string{}
	for _, m := range regexp.MustCompile(`--([a-z0-9-]+)\s*:\s*([^;]+);`).FindAllStringSubmatch(tokens, -1) {
		defined[m[1]] = strings.TrimSpace(m[2])
	}
	for _, m := range regexp.MustCompile(`var\(--([a-z0-9-]+)`).FindAllStringSubmatch(css, -1) {
		if _, ok := defined[m[1]]; !ok {
			t.Errorf("keyboard.css uses undefined token --%s", m[1])
		}
	}
	if hits := transitionOffenders(css); len(hits) != 0 {
		t.Errorf("keyboard.css transitions: %v", hits)
	}
	rule := func(sel string) string {
		for _, r := range parseCSSRules(css) {
			if r.Selector == sel {
				return r.Body
			}
		}
		t.Fatalf("keyboard.css has no %s rule", sel)
		return ""
	}
	if !strings.Contains(rule(".hk-key"), "min-block-size: var(--size-tap)") {
		t.Error(".hk-key is not at least --size-tap tall")
	}
	if !strings.Contains(rule(".hk-hints"), "min-block-size: var(--size-tap)") {
		t.Error(".hk-hints is not at least --size-tap tall")
	}
	if !strings.Contains(rule(".hk-key"), "min-inline-size: calc(var(--space-6) - var(--space-1))") {
		t.Error(".hk-key min width is not space-6 minus space-1 (28px)")
	}
	for _, bad := range []string{"overflow", "position: fixed", "inline-size: 100vw", "min-inline-size: 100vw"} {
		if strings.Contains(css, bad) {
			t.Errorf("keyboard.css has %q, which can cause horizontal scroll or hide the page on a phone", bad)
		}
	}

	// The widest row is ten keys. At 360px the keyboard has --space-2 padding
	// each side and a 1px border, so the row's own width is what is left.
	px := func(name string) int {
		n, err := strconv.Atoi(strings.TrimSuffix(defined[name], "px"))
		if err != nil {
			t.Fatalf("token --%s = %q is not px", name, defined[name])
		}
		return n
	}
	const viewport, keys = 360, 10
	row := viewport - 2*px("space-2") - 2
	keyW := row / keys
	minW := px("space-6") - px("space-1")
	if px("size-tap") != 44 {
		t.Errorf("--size-tap = %d, the spec says 44", px("size-tap"))
	}
	if keyW < minW {
		t.Errorf("ten keys at %dpx leave %dpx each, under the %dpx minimum", viewport, keyW, minW)
	}
	if minW*keys > viewport-2*px("space-2")-2 {
		t.Errorf("ten minimum-width keys (%dpx) overflow a %dpx screen", minW*keys, viewport)
	}
	if minW < 28 {
		t.Errorf("minimum key width %dpx, the spec's floor is 28px", minW)
	}
}

// TestKeyboardJS: the script is small, has no dependency, uses no dynamic
// code, and is served as the file.
func TestKeyboardJS(t *testing.T) {
	b, err := os.ReadFile("static/js/hekbd.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	if len(b) > 4096 {
		t.Errorf("hekbd.js is %d bytes; the budget is 4096 (minified, and this file is not minified)", len(b))
	}
	if !utf8.Valid(b) || !norm.NFC.IsNormalString(js) {
		t.Error("hekbd.js is not NFC UTF-8")
	}
	for _, bad := range []string{"eval(", "new Function", "innerHTML", "document.write", "http://", "https://", "import(", "require(\"", "fetch(", "XMLHttpRequest"} {
		if strings.Contains(js, bad) {
			t.Errorf("hekbd.js contains %q", bad)
		}
	}
	for _, need := range []string{`\p{Cf}`, `.normalize("NFC")`, "data-code", "data-ch", "hekbd:enter", "preventScroll", `"mousedown"`, `"inputmode"`} {
		if !strings.Contains(js, need) {
			t.Errorf("hekbd.js lacks %s", need)
		}
	}
	h := New(nil, nil)
	if got := get(t, h, "/static/js/hekbd.js"); got != js {
		t.Error("served hekbd.js differs from the file")
	}
	if got := get(t, h, "/static/css/keyboard.css"); !strings.Contains(got, ".hk-key") {
		t.Error("keyboard.css is not served")
	}
}

// TestKeyboardNode runs hekbd.test.js, the script's own unit tests, when node
// is installed. It is skipped otherwise, as the browser tests are.
func TestKeyboardNode(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not installed; run: node internal/web/testdata/keyboard/hekbd.test.js")
	}
	out, err := exec.Command(node, filepath.Join("testdata", "keyboard", "hekbd.test.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("hekbd.test.js: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "all passed") {
		t.Fatalf("hekbd.test.js did not finish:\n%s", out)
	}
	t.Run("ReviewGradesStayInView", reviewGradesStayInView)
}
