package web

// Asset lints for INIT-015/SPEC-010: icons, brand mark, and illustrations.
// Nothing here opens a browser. These tests read the shipped files and the
// rendered HTML.

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// svgEl is one start tag from an SVG file.
type svgEl struct {
	Name  string
	Attrs map[string]string
}

// parseSVG reads path and fails if it is not well-formed XML. It returns every
// start tag.
func parseSVG(t *testing.T, path string) (raw string, els []svgEl) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dec := xml.NewDecoder(bytes.NewReader(b))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("%s is not well-formed XML: %v", path, err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			el := svgEl{Name: se.Name.Local, Attrs: map[string]string{}}
			for _, a := range se.Attr {
				el.Attrs[a.Name.Local] = a.Value
			}
			els = append(els, el)
		}
	}
	if len(els) == 0 || els[0].Name != "svg" {
		t.Fatalf("%s: root element is not <svg>", path)
	}
	return string(b), els
}

func svgFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.Walk("static", func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(p, ".svg") {
			out = append(out, filepath.ToSlash(p))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func spriteIDs(t *testing.T) map[string]svgEl {
	t.Helper()
	_, els := parseSVG(t, "static/icons/sprite.svg")
	ids := map[string]svgEl{}
	for _, e := range els {
		if e.Name == "symbol" {
			ids[strings.TrimPrefix(e.Attrs["id"], "icon-")] = e
		}
	}
	return ids
}

// tokenColours is every #rrggbb value in the Figma token export, lower case.
func tokenColours(t *testing.T) map[string]bool {
	t.Helper()
	b, err := os.ReadFile("static/css/lamplight.tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Tokens []struct {
			Type  string          `json:"type"`
			Light json.RawMessage `json:"light"`
			Dark  json.RawMessage `json:"dark"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, tok := range doc.Tokens {
		if tok.Type != "color" {
			continue
		}
		for _, raw := range []json.RawMessage{tok.Light, tok.Dark} {
			var s string
			if json.Unmarshal(raw, &s) == nil && len(s) == 7 {
				out[strings.ToLower(s)] = true
			}
		}
	}
	return out
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// templSources lists the .templ files under views/, base name to text.
func templSources(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob("views/*.templ")
	if err != nil || len(files) == 0 {
		t.Fatalf("no templ files: %v", err)
	}
	out := map[string]string{}
	for _, f := range files {
		out[filepath.Base(f)] = readFile(t, f)
	}
	return out
}

// ---- ac-1: no emoji, no stray inline <svg> ---------------------------------

// pictographic reports whether r is in a range that draws an emoji or a text
// glyph used as an icon. Dingbats cover check and cross marks, arrows cover
// the arrow glyphs, and the U+1F000 planes cover the emoji proper.
func pictographic(r rune) bool {
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF:
		return true
	case r >= 0x2190 && r <= 0x21FF: // arrows
		return true
	case r >= 0x2300 && r <= 0x23FF: // misc technical (hourglass, play)
		return true
	case r >= 0x25A0 && r <= 0x25FF: // geometric shapes
		return true
	case r >= 0x2600 && r <= 0x27BF: // misc symbols and dingbats
		return true
	case r >= 0x2B00 && r <= 0x2BFF: // stars, arrows
		return true
	case r == 0x200D || r == 0xFE0F || r == 0x20E3: // joiner, emoji selector, keycap
		return true
	}
	return false
}

func firstPictographic(s string) (rune, bool) {
	for _, r := range s {
		if pictographic(r) {
			return r, true
		}
	}
	return 0, false
}

func TestPictographicDetector(t *testing.T) {
	for _, s := range []string{"✓", "→", "🔥", "⭐", "❌", "▶", "🇮🇱", "1\uFE0F\u20E3"} {
		if _, ok := firstPictographic(s); !ok {
			t.Errorf("%q not detected", s)
		}
	}
	for _, s := range []string{"שָׁלוֹם", "Today · 3", "can’t", "a – b", "“hi”", "−1"} {
		if r, ok := firstPictographic(s); ok {
			t.Errorf("%q flagged for %U", s, r)
		}
	}
}

// TestNoEmojiInTempl is ac-1, part one.
func TestNoEmojiInTempl(t *testing.T) {
	for name, src := range templSources(t) {
		if r, ok := firstPictographic(src); ok {
			t.Errorf("%s contains %U; draw it as a sprite icon", name, r)
		}
	}
}

// TestNoEmojiInViewCopy covers the Go files that feed the templates: an emoji
// in a copy string is an emoji in the page. Arrows in running text are fine,
// so this scan leaves the arrow block out.
func TestNoEmojiInViewCopy(t *testing.T) {
	files, err := filepath.Glob("views/*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		base := filepath.Base(f)
		if strings.HasSuffix(base, "_templ.go") || strings.HasSuffix(base, "_test.go") {
			continue
		}
		for _, r := range readFile(t, f) {
			if pictographic(r) && !(r >= 0x2190 && r <= 0x21FF) {
				t.Errorf("%s contains %U", base, r)
				break
			}
		}
	}
}

// inlineSVGAllowed are the only templ files that may write an <svg> tag:
// the component library, and the icon helpers.
var inlineSVGAllowed = map[string]bool{"components.templ": true, "icons.templ": true}

// TestNoInlineSVGOutsideComponents is ac-1, part two. The icon helpers in
// icons.templ stand in for components.templ, which stays untouched so the
// parallel work on it merges cleanly.
func TestNoInlineSVGOutsideComponents(t *testing.T) {
	for name, src := range templSources(t) {
		if inlineSVGAllowed[name] {
			continue
		}
		if strings.Contains(src, "<svg") {
			t.Errorf("%s has an inline <svg>; use @Icon, @IconClass, @Illustration, or @BrandMark", name)
		}
	}
}

// ---- the sprite ------------------------------------------------------------

// requiredIcons is the set the design names (SPEC-010 scope), by sprite id.
var requiredIcons = []string{
	"speaker", "replay", "slower", "close", "check", "x", "arrow-right", "arrow-left",
	"keyboard", "home", "book", "layers", "chart", "user", "flag", "lamp", "spark",
}

// legacyIcons are the ids the views used before this spec. They must still resolve.
var legacyIcons = []string{"today", "review", "progress", "theme", "next", "play", "sound", "help", "check", "lessons", "tip", "replay"}

func TestSpriteShape(t *testing.T) {
	ids := spriteIDs(t)
	_, els := parseSVG(t, "static/icons/sprite.svg")
	for name, sym := range ids {
		if sym.Attrs["id"] != "icon-"+name || name == "" {
			t.Errorf("symbol id %q is not icon-<name>", sym.Attrs["id"])
		}
		if sym.Attrs["viewBox"] != "0 0 24 24" {
			t.Errorf("icon-%s viewBox = %q, want the 24px grid", name, sym.Attrs["viewBox"])
		}
		if sym.Attrs["stroke"] != "currentColor" || sym.Attrs["stroke-width"] != "2" || sym.Attrs["fill"] != "none" {
			t.Errorf("icon-%s is not a 2px currentColor line icon", name)
		}
		if sym.Attrs["stroke-linecap"] != "round" || sym.Attrs["stroke-linejoin"] != "round" {
			t.Errorf("icon-%s lacks round caps and joins", name)
		}
	}
	for _, e := range els {
		for _, attr := range []string{"fill", "stroke", "color", "style"} {
			v, ok := e.Attrs[attr]
			if !ok {
				continue
			}
			if attr == "style" {
				if e.Name != "svg" {
					t.Errorf("<%s> has a style attribute", e.Name)
				}
				continue
			}
			if v != "none" && v != "currentColor" {
				t.Errorf("<%s> %s=%q; sprite icons use currentColor only", e.Name, attr, v)
			}
		}
	}
	raw := readFile(t, "static/icons/sprite.svg")
	if regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b|rgb|hsl|oklch`).MatchString(strings.ReplaceAll(raw, "currentColor", "")) {
		t.Error("sprite has a literal colour")
	}
	for _, gone := range []string{"icon-streak", "icon-xp"} {
		if strings.Contains(raw, gone) {
			t.Errorf("sprite still ships %s", gone)
		}
	}
}

func TestSpriteMatchesIconNames(t *testing.T) {
	ids := spriteIDs(t)
	for _, n := range views.IconNames {
		if _, ok := ids[n]; !ok {
			t.Errorf("views.IconNames lists %q but the sprite has no icon-%s", n, n)
		}
	}
	listed := map[string]bool{}
	for _, n := range views.IconNames {
		if listed[n] {
			t.Errorf("views.IconNames repeats %q", n)
		}
		listed[n] = true
	}
	for n := range ids {
		if !listed[n] {
			t.Errorf("sprite has icon-%s but views.IconNames does not list it", n)
		}
	}
	for _, n := range append(append([]string{}, requiredIcons...), legacyIcons...) {
		if _, ok := ids[n]; !ok {
			t.Errorf("sprite is missing icon-%s", n)
		}
	}
}

func TestIconHrefFallsBackToHelp(t *testing.T) {
	if got := string(views.IconHref("lamp")); got != "/static/icons/sprite.svg#icon-lamp" {
		t.Errorf("IconHref(lamp) = %q", got)
	}
	for _, bad := range []string{"", "streak", "xp", "../x", "lamp\"onload="} {
		if got := string(views.IconHref(bad)); got != "/static/icons/sprite.svg#icon-help" {
			t.Errorf("IconHref(%q) = %q, want help", bad, got)
		}
	}
}

var (
	iconCallRe  = regexp.MustCompile(`\bIcon(?:Class)?\(\s*"([a-z-]+)"`)
	iconHrefRe  = regexp.MustCompile(`\bIconHref\(\s*"([a-z-]+)"`)
	dockLinkRe  = regexp.MustCompile(`@dockLink\([^,]+,\s*"([a-z-]+)"`)
	gradeIconRe = regexp.MustCompile(`@gradeButton\("[^"]*",\s*"[^"]*",\s*"([a-z-]+)"`)
)

// TestTemplIconsExistInSprite: every icon a template names by literal exists.
func TestTemplIconsExistInSprite(t *testing.T) {
	ids := spriteIDs(t)
	srcs := templSources(t)
	goFiles, _ := filepath.Glob("views/*.go")
	for _, f := range goFiles {
		if !strings.HasSuffix(f, "_templ.go") {
			srcs[filepath.Base(f)] = readFile(t, f)
		}
	}
	found := 0
	for name, src := range srcs {
		for _, re := range []*regexp.Regexp{iconCallRe, iconHrefRe, dockLinkRe, gradeIconRe} {
			for _, m := range re.FindAllStringSubmatch(src, -1) {
				found++
				if _, ok := ids[m[1]]; !ok {
					t.Errorf("%s uses icon %q, which is not in the sprite", name, m[1])
				}
			}
		}
	}
	if found < 10 {
		t.Fatalf("only %d icon references found; the scan is broken", found)
	}
}

// TestRenderedIconsExistInSprite does the same on rendered pages, so a name
// that arrives from a variable is checked too.
func TestRenderedIconsExistInSprite(t *testing.T) {
	ids := spriteIDs(t)
	h := New(nil, nil)
	re := regexp.MustCompile(`/static/icons/sprite\.svg#icon-([a-z-]+)`)
	n := 0
	for _, path := range []string{"/", "/?due=0", "/review", "/progress", "/lesson", "/signin"} {
		for _, m := range re.FindAllStringSubmatch(get(t, h, path), -1) {
			n++
			if _, ok := ids[m[1]]; !ok {
				t.Errorf("%s renders icon-%s, which is not in the sprite", path, m[1])
			}
		}
	}
	if n == 0 {
		t.Fatal("no icons rendered")
	}
}

// ---- ac-3: 20px and 24px, accessible names ---------------------------------

func TestIconSizesInCSS(t *testing.T) {
	base := readFile(t, "static/css/base.css")
	icons := readFile(t, "static/css/icons.css")
	rule := func(css, selector string) string {
		for _, r := range parseCSSRules(css) {
			if r.Selector == selector {
				return r.Body
			}
		}
		return ""
	}
	if !strings.Contains(rule(base, ".icon"), "inline-size: var(--space-5)") {
		t.Error(".icon is not 24px (var(--space-5)) in base.css")
	}
	sm := rule(icons, ".icon-sm")
	if !regexp.MustCompile(`inline-size:\s*20px`).MatchString(sm) || !regexp.MustCompile(`block-size:\s*20px`).MatchString(sm) {
		t.Errorf(".icon-sm is not 20px square: %q", sm)
	}
}

func TestEveryIconRendersAt20And24(t *testing.T) {
	for _, name := range views.IconNames {
		for _, class := range []string{"icon", "icon icon-sm"} {
			html := renderComponent(t, views.IconClass(name, class, ""), "")
			want := `<svg class="` + class + `" aria-hidden="true" focusable="false"><use href="/static/icons/sprite.svg#icon-` + name + `"></use></svg>`
			if html != want {
				t.Errorf("IconClass(%s, %q) = %s", name, class, html)
			}
		}
	}
	// Square 24 grid means the same symbol scales cleanly to both sizes.
	for name, sym := range spriteIDs(t) {
		if sym.Attrs["viewBox"] != "0 0 24 24" {
			t.Errorf("icon-%s is not on the 24 grid", name)
		}
	}
}

func TestIconAccessibleName(t *testing.T) {
	deco := renderComponent(t, views.Icon("check", ""), "")
	if !strings.Contains(deco, `aria-hidden="true"`) || strings.Contains(deco, "role=") || strings.Contains(deco, "<title>") {
		t.Errorf("decorative icon is not aria-hidden only: %s", deco)
	}
	named := renderComponent(t, views.Icon("flag", "Report a problem"), "")
	for _, want := range []string{`role="img"`, `aria-label="Report a problem"`, `<title>Report a problem</title>`} {
		if !strings.Contains(named, want) {
			t.Errorf("labelled icon missing %s: %s", want, named)
		}
	}
	if strings.Contains(named, "aria-hidden") {
		t.Errorf("labelled icon is aria-hidden: %s", named)
	}
	esc := renderComponent(t, views.Icon("flag", `"><script>x</script>`), "")
	if strings.Contains(esc, "<script>") {
		t.Errorf("label is not escaped: %s", esc)
	}
	if unk := renderComponent(t, views.Icon("nope", ""), ""); !strings.Contains(unk, "#icon-help") {
		t.Errorf("unknown icon did not fall back to help: %s", unk)
	}
}

var controlRe = regexp.MustCompile(`(?is)<(a|button)\b([^>]*)>(.*?)</(?:a|button)>`)
var tagRe = regexp.MustCompile(`(?s)<[^>]*>`)

// unnamedControls lists links and buttons with no accessible name: no text
// (an svg <title> counts as text), no aria-label, no aria-labelledby.
func unnamedControls(html string) []string {
	var bad []string
	for _, m := range controlRe.FindAllStringSubmatch(html, -1) {
		attrs, inner := m[2], m[3]
		if strings.Contains(attrs, "aria-label=") || strings.Contains(attrs, "aria-labelledby=") {
			continue
		}
		if strings.TrimSpace(tagRe.ReplaceAllString(inner, "")) != "" {
			continue
		}
		bad = append(bad, strings.TrimSpace(m[0]))
	}
	return bad
}

func TestUnnamedControlDetector(t *testing.T) {
	bad := `<a href="/x"><svg aria-hidden="true"><use href="#a"></use></svg></a><button><svg></svg></button>`
	if got := unnamedControls(bad); len(got) != 2 {
		t.Errorf("detector found %d of 2: %v", len(got), got)
	}
	good := []string{
		`<button aria-label="Play"><svg></svg></button>`,
		`<a href="/x"><svg role="img"><title>Play</title></svg></a>`,
		`<a href="/x"><svg></svg><span>Play</span></a>`,
	}
	for _, g := range good {
		if got := unnamedControls(g); len(got) != 0 {
			t.Errorf("flagged %s", g)
		}
	}
}

// TestIconOnlyControlsHaveNames renders the shell pages and the audio
// controls, and checks that no link or button is nameless.
func TestIconOnlyControlsHaveNames(t *testing.T) {
	h := New(nil, nil)
	pages := map[string]string{}
	for _, path := range []string{"/", "/?due=0", "/?lang=he", "/review", "/progress", "/lesson", "/signin"} {
		pages[path] = get(t, h, path)
	}
	pages["audio orb"] = renderComponent(t, views.AudioOrb("Play the word", nil), "")
	for _, kind := range []string{"read", "recall"} {
		card := views.ReviewCard{Lang: "en", CardID: 1, Kind: kind, Revealed: true, Pointed: "שָׁלוֹם", Unpointed: "שלום", Gloss: "hello", AudioURL: "/audio/1.mp3", Index: 1, Total: 3}
		pages["review "+kind] = renderComponent(t, views.ReviewBody(card), "")
		if !strings.Contains(pages["review "+kind], "/audio/1.mp3") {
			t.Fatalf("review %s did not render its audio link", kind)
		}
	}
	for name, html := range pages {
		for _, bad := range unnamedControls(html) {
			t.Errorf("%s: control with no accessible name: %s", name, bad)
		}
	}
}

// ---- ac-2: illustrations ---------------------------------------------------

// themeVars are the only colour roles an illustration may name.
var themeVars = []string{"--lamp", "--ink", "--surface-soft", "--lamp-ink"}

// legacyIllustrations are older files that predate this rule. The sign-in page
// loads this one as an <img>, and an image cannot read the page's custom
// properties, so it keeps its own colours.
var legacyIllustrations = map[string]string{
	"hero-skyline-night.svg": "sign-in <img>; an image cannot read page tokens",
}

var varFillRe = regexp.MustCompile(`^var\((--[a-z-]+)\)$`)

// colourViolations lists every fill, stroke, or colour that is not none,
// currentColor, or var(--token) from allowed.
func colourViolations(els []svgEl, allowed []string, allowCurrent bool) []string {
	var bad []string
	for _, e := range els {
		for _, attr := range []string{"fill", "stroke", "stop-color", "flood-color", "color"} {
			v, ok := e.Attrs[attr]
			if !ok {
				continue
			}
			if v == "none" || (allowCurrent && v == "currentColor") {
				continue
			}
			m := varFillRe.FindStringSubmatch(v)
			if m == nil {
				bad = append(bad, "<"+e.Name+"> "+attr+"="+v)
				continue
			}
			ok = false
			for _, a := range allowed {
				if a == m[1] {
					ok = true
				}
			}
			if !ok {
				bad = append(bad, "<"+e.Name+"> "+attr+"="+v+" is not a theme colour")
			}
		}
		if _, ok := e.Attrs["style"]; ok {
			bad = append(bad, "<"+e.Name+"> has a style attribute")
		}
		if e.Name == "style" {
			bad = append(bad, "<style> element")
		}
	}
	return bad
}

func TestColourViolationDetector(t *testing.T) {
	els := []svgEl{
		{"path", map[string]string{"fill": "#fff"}},
		{"path", map[string]string{"stroke": "red"}},
		{"path", map[string]string{"fill": "rgb(0,0,0)"}},
		{"path", map[string]string{"fill": "var(--go)"}},
		{"path", map[string]string{"fill": "currentColor"}},
		{"path", map[string]string{"fill": "var(--lamp)", "style": "fill:red"}},
	}
	if got := colourViolations(els, themeVars, false); len(got) != 6 {
		t.Errorf("found %d of 6: %v", len(got), got)
	}
	ok := []svgEl{{"path", map[string]string{"fill": "var(--lamp)", "stroke": "var(--lamp-ink)"}}, {"path", map[string]string{"fill": "none"}}}
	if got := colourViolations(ok, themeVars, false); len(got) != 0 {
		t.Errorf("flagged good input: %v", got)
	}
}

func TestIllustrationSet(t *testing.T) {
	files, err := filepath.Glob("static/illustrations/*.svg")
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, f := range files {
		have[filepath.Base(f)] = true
	}
	for _, n := range views.Illustrations {
		if !have[n+".svg"] {
			t.Errorf("illustration %s.svg is missing", n)
		}
	}
	listed := map[string]bool{}
	for _, n := range views.Illustrations {
		listed[n+".svg"] = true
	}
	for f := range have {
		if _, legacy := legacyIllustrations[f]; !listed[f] && !legacy {
			t.Errorf("%s is in static/illustrations but not in views.Illustrations", f)
		}
	}
	want := []string{"placement-intro", "results", "empty-review", "empty-words", "empty-progress", "band-up", "offline"}
	got := append([]string{}, views.Illustrations...)
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(want, ",") != strings.Join(got, ",") {
		t.Errorf("illustration set = %v, want %v", got, want)
	}
}

// TestIllustrationsUseOnlyTokenColours is ac-2.
func TestIllustrationsUseOnlyTokenColours(t *testing.T) {
	css := readFile(t, "static/css/tokens.css")
	for _, v := range themeVars {
		if !strings.Contains(css, v+":") {
			t.Errorf("tokens.css has no %s", v)
		}
	}
	for _, name := range views.Illustrations {
		path := "static/illustrations/" + name + ".svg"
		raw, els := parseSVG(t, path)
		if bad := colourViolations(els, themeVars, false); len(bad) > 0 {
			t.Errorf("%s: %s", path, strings.Join(bad, "; "))
		}
		if regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b|\b(?:rgba?|hsla?|oklch|oklab|lab|lch|hwb)\(`).MatchString(raw) {
			t.Errorf("%s has a literal colour", path)
		}
		if els[0].Attrs["viewBox"] == "" {
			t.Errorf("%s: root <svg> has no viewBox", path)
		}
		found := false
		for _, e := range els {
			if e.Name == "symbol" && e.Attrs["id"] == "art" && e.Attrs["viewBox"] != "" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s has no <symbol id=\"art\" viewBox=...>", path)
		}
	}
}

func TestIllustrationAllowlistIsReal(t *testing.T) {
	for f, why := range legacyIllustrations {
		if why == "" {
			t.Errorf("%s needs a reason", f)
		}
		if _, err := os.Stat("static/illustrations/" + f); err != nil {
			t.Errorf("allowlist names %s, which is gone: delete the entry", f)
		}
	}
	// The sign-in page loads the legacy file as an image. If it stops, the
	// exception is dead.
	for f := range legacyIllustrations {
		used := false
		for _, src := range templSources(t) {
			if strings.Contains(src, "/static/illustrations/"+f) {
				used = true
			}
		}
		if !used {
			t.Errorf("%s is no longer used by a template; delete the file and the allowlist entry", f)
		}
	}
}

func TestIllustrationComponent(t *testing.T) {
	for _, name := range views.Illustrations {
		deco := renderComponent(t, views.Illustration(name, ""), "")
		want := `<svg class="illo" viewBox="0 0 240 160" aria-hidden="true" focusable="false"><use href="/static/illustrations/` + name + `.svg#art"></use></svg>`
		if deco != want {
			t.Errorf("Illustration(%s) = %s", name, deco)
		}
	}
	named := renderComponent(t, views.Illustration("offline", "You are offline"), "")
	for _, want := range []string{`role="img"`, `aria-label="You are offline"`, `<title>You are offline</title>`} {
		if !strings.Contains(named, want) {
			t.Errorf("labelled illustration missing %s: %s", want, named)
		}
	}
	if out := renderComponent(t, views.Illustration("../../etc/passwd", ""), ""); out != "" {
		t.Errorf("unknown illustration rendered %q", out)
	}
}

// ---- brand mark ------------------------------------------------------------

func TestBrandAssets(t *testing.T) {
	colours := tokenColours(t)
	hex := regexp.MustCompile(`#[0-9a-fA-F]{6}\b`)
	for _, f := range []string{"mark.svg", "mark-maskable.svg"} {
		raw, els := parseSVG(t, "static/brand/"+f)
		if els[0].Attrs["viewBox"] != "0 0 64 64" {
			t.Errorf("%s viewBox = %q", f, els[0].Attrs["viewBox"])
		}
		// A favicon is an image, so it cannot read page tokens. Its colours
		// must be exact token values instead.
		for _, h := range hex.FindAllString(raw, -1) {
			if !colours[strings.ToLower(h)] {
				t.Errorf("%s uses %s, which is not a Lamplight token value", f, h)
			}
		}
		if len(hex.FindAllString(raw, -1)) == 0 {
			t.Errorf("%s has no colour; a favicon cannot inherit one", f)
		}
	}
	raw, els := parseSVG(t, "static/brand/glyph.svg")
	if bad := colourViolations(els, []string{"--kicker-on-ink"}, true); len(bad) > 0 {
		t.Errorf("glyph.svg: %s", strings.Join(bad, "; "))
	}
	if !strings.Contains(raw, `<symbol id="glyph" viewBox="0 0 64 64">`) {
		t.Error("glyph.svg has no symbol id=glyph")
	}

	var manifest struct {
		Name            string `json:"name"`
		StartURL        string `json:"start_url"`
		BackgroundColor string `json:"background_color"`
		ThemeColor      string `json:"theme_color"`
		Icons           []struct {
			Src     string `json:"src"`
			Type    string `json:"type"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
	}
	if err := json.Unmarshal([]byte(readFile(t, "static/brand/manifest.webmanifest")), &manifest); err != nil {
		t.Fatalf("manifest is not JSON: %v", err)
	}
	if manifest.Name == "" || manifest.StartURL != "/" {
		t.Errorf("manifest name/start_url: %q %q", manifest.Name, manifest.StartURL)
	}
	for _, c := range []string{manifest.BackgroundColor, manifest.ThemeColor} {
		if !colours[strings.ToLower(c)] {
			t.Errorf("manifest colour %q is not a token value", c)
		}
	}
	purposes := map[string]bool{}
	for _, ic := range manifest.Icons {
		purposes[ic.Purpose] = true
		if _, err := os.Stat("static" + strings.TrimPrefix(ic.Src, "/static")); err != nil {
			t.Errorf("manifest icon %s does not exist", ic.Src)
		}
	}
	if !purposes["any"] || !purposes["maskable"] {
		t.Errorf("manifest needs an any and a maskable icon, has %v", purposes)
	}
}

func TestBrandWiredIntoPages(t *testing.T) {
	h := New(nil, nil)
	for _, path := range []string{"/", "/signin"} {
		body := get(t, h, path)
		for _, want := range []string{
			`<link rel="icon" href="/static/brand/mark.svg" type="image/svg+xml">`,
			`<link rel="manifest" href="/static/brand/manifest.webmanifest">`,
			`<link rel="stylesheet" href="/static/css/icons.css">`,
			`/static/brand/glyph.svg#glyph`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s is missing %s", path, want)
			}
		}
		if strings.Contains(body, `brand__mark" aria-hidden="true">ע`) {
			t.Errorf("%s still draws the brand mark as a letter", path)
		}
	}
	for _, path := range []string{
		"/static/icons/sprite.svg", "/static/css/icons.css", "/static/brand/mark.svg",
		"/static/brand/mark-maskable.svg", "/static/brand/glyph.svg", "/static/brand/manifest.webmanifest",
		"/static/illustrations/offline.svg", "/static/LICENSES/lucide.txt",
	} {
		get(t, h, path)
	}
}

// ---- safety and size -------------------------------------------------------

var (
	externalRef = regexp.MustCompile(`(?i)(?:href|src)\s*=\s*"\s*(?:[a-z][a-z0-9+.-]*:|//)`)
	eventAttr   = regexp.MustCompile(`(?i)\son[a-z]+\s*=`)
	urlFunc     = regexp.MustCompile(`(?i)url\(\s*['"]?\s*(?:[a-z][a-z0-9+.-]*:|//)`)
)

// TestSVGsAreSafe: no script, foreignObject, external reference, or event
// handler in any shipped SVG.
func TestSVGsAreSafe(t *testing.T) {
	files := svgFiles(t)
	if len(files) < 12 {
		t.Fatalf("found only %d svg files", len(files))
	}
	for _, f := range files {
		raw, els := parseSVG(t, f)
		for _, e := range els {
			switch strings.ToLower(e.Name) {
			case "script", "foreignobject", "iframe", "object", "embed", "image", "a", "animate", "set", "animatetransform", "animatemotion":
				t.Errorf("%s has <%s>", f, e.Name)
			}
			for k, v := range e.Attrs {
				if strings.HasPrefix(strings.ToLower(k), "on") {
					t.Errorf("%s: <%s> has event attribute %s", f, e.Name, k)
				}
				if (k == "href" || k == "src") && !strings.HasPrefix(v, "#") {
					t.Errorf("%s: <%s> %s=%q is not a local fragment", f, e.Name, k, v)
				}
			}
		}
		stripped := strings.ReplaceAll(raw, "http://www.w3.org/2000/svg", "")
		if externalRef.MatchString(stripped) || urlFunc.MatchString(stripped) || eventAttr.MatchString(stripped) {
			t.Errorf("%s has an external reference or an event attribute", f)
		}
		if strings.Contains(stripped, "http://") || strings.Contains(stripped, "https://") {
			t.Errorf("%s names an external URL", f)
		}
		if strings.Contains(raw, "<?xml") || strings.Contains(raw, "<!--") || strings.Contains(raw, "sodipodi") || strings.Contains(raw, "inkscape") || strings.Contains(raw, "<metadata") {
			t.Errorf("%s carries editor metadata", f)
		}
	}
}

// TestSVGSizes: each standalone SVG is under 6 KB and has a viewBox. The icon
// sprite holds every icon in one file and has its own, larger, ceiling.
func TestSVGSizes(t *testing.T) {
	for _, f := range svgFiles(t) {
		info, err := os.Stat(f)
		if err != nil {
			t.Fatal(err)
		}
		limit := int64(6 * 1024)
		if strings.HasSuffix(f, "icons/sprite.svg") {
			// eye and flip, for the teach-step word cards, take the sprite past 8KB.
			limit = 9 * 1024
		}
		if strings.HasSuffix(f, "hero-skyline-night.svg") {
			limit = 9 * 1024 // predates this spec; unchanged
		}
		if info.Size() > limit {
			t.Errorf("%s is %d bytes, over %d", f, info.Size(), limit)
		}
		if strings.HasSuffix(f, "icons/sprite.svg") {
			continue
		}
		_, els := parseSVG(t, f)
		if els[0].Attrs["viewBox"] == "" {
			t.Errorf("%s: root <svg> has no viewBox", f)
		}
	}
}

func TestIconsCSSUsesTokensOnly(t *testing.T) {
	css := readFile(t, "static/css/icons.css")
	res := lintColors("icons.css", css, nil)
	if len(res.Violations) > 0 {
		t.Errorf("icons.css breaks the colour rules: %s", strings.Join(res.Violations, "; "))
	}
	if hits := physicalProperties(css); len(hits) > 0 {
		t.Errorf("icons.css uses physical left/right: %v", hits)
	}
	if transitionOffenders(css) != nil {
		t.Error("icons.css animates a property other than transform or opacity")
	}
	if strings.Contains(css, "http://") || strings.Contains(css, "https://") {
		t.Error("icons.css names an external URL")
	}
}

func TestLucideNoticeShipped(t *testing.T) {
	txt := readFile(t, "static/LICENSES/lucide.txt")
	for _, want := range []string{"ISC License", "Lucide", "Permission to use, copy, modify, and/or distribute"} {
		if !strings.Contains(txt, want) {
			t.Errorf("lucide.txt is missing %q", want)
		}
	}
}
