package web

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// INIT-023/SPEC-001: every .orb must resolve audio the way clips.js audioOf does
// (data-audio naming an <audio> on the page, or a .clip parent that holds one).

var (
	orbButtonRE = regexp.MustCompile(`(?is)<button\b([^>]*)>`)
	orbClassRE  = regexp.MustCompile(`\bclass="([^"]*)"`)
	orbAudioRE  = regexp.MustCompile(`\bdata-audio="([^"]*)"`)
	audioIDRE   = regexp.MustCompile(`(?is)<audio\b([^>]*)>`)
	idAttrRE    = regexp.MustCompile(`\bid="([^"]*)"`)
	clipOpenRE  = regexp.MustCompile(`(?is)<div\b([^>]*\bclass="[^"]*\bclip\b[^"]*"[^>]*)>`)
)

func TestOrbButtonsCarryAudio(t *testing.T) {
	progress := renderView(t, views.Progress(views.ProgressView{
		Page: views.Page{UI: views.UIFor("en", views.AddressMasculine)},
		CanSay: &views.CanSayPanel{
			TotalText: "2 things",
			AddedText: "1 this week",
			Items: []views.CanSayWord{
				{He: "פֹּה", Gloss: "here", Audio: "/audio/abc.mp3"},
				{He: "בֹּקֶר", Gloss: "morning"},
			},
			Next: []views.CanSayWord{
				{He: "כָּתַב", Gloss: "he wrote", Audio: "/audio/def.mp3"},
			},
		},
	}))
	assertCansayClips(t, progress)
	assertOrbsWired(t, progress, "progress")

	isaiah := renderView(t, views.IsaiahPage(views.IsaiahView{
		Page: views.Page{UI: views.UIFor("en", views.AddressMasculine), Title: "Isaiah"},
		Verse: &views.IsaiahVerse{
			Heading: "Isaiah 6:1",
			He:      "קָדוֹשׁ",
			En:      "Holy.",
			Audio:   "/audio/verse.mp3",
		},
	}))
	assertVerseClip(t, isaiah)
	assertOrbsWired(t, isaiah, "isaiah")

	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	learner := addLearner(t, store, "a@example.com", "sub-orb", now)

	today := getLearner(t, h, learner, "/")
	assertOrbsWired(t, today, "today")

	const family = "שׁ־מ־ע"
	hear := units.Normalize("שָׁמַע")
	news := units.Normalize("שְׁמוּעָה")
	for _, row := range []struct{ text, gloss string }{
		{hear, "hear"},
		{news, "news"},
	} {
		if _, err := store.ExecContext(t.Context(),
			`INSERT INTO lexemes (text_nfc, gloss_en, pos, root) VALUES (?, ?, 'noun', ?)`,
			row.text, row.gloss, family); err != nil {
			t.Fatal(err)
		}
	}
	roots := getLearner(t, h, learner, "/roots")
	assertOrbsWired(t, roots, "roots")
	rootCard := getLearner(t, h, learner, "/roots/"+url.PathEscape(family))
	assertOrbsWired(t, rootCard, "roots card")

	sp := func(s string) *string { return &s }
	hello := units.Normalize("שָׁלוֹם")
	if _, _, err := store.UpsertLexemes(t.Context(), []lexicon.Input{
		{TextNFC: hello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
	}); err != nil {
		t.Fatal(err)
	}
	phraseID := strings.Repeat("ab", 32)
	if _, err := store.InsertAudio(t.Context(), db.Asset{
		ID: phraseID, TextNFC: hello, Voice: "voice", Model: "eleven_v4", Bytes: 4,
	}); err != nil {
		t.Fatal(err)
	}
	id := putLesson(t, store, nil, units.Unit{
		Title: "Orb lesson", Position: 1, NewWords: []string{hello},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: hello},
		}}},
	}, "shared")
	publish(t, store, id)
	lesson := getLearner(t, h, learner, "/lesson/"+itoa(id)+"/0")
	assertOrbsWired(t, lesson, "lesson")
}

// TestOrbCansayClipResolvesInClipsJS is ac-4 without a live browser: the Progress
// clip HTML must contain <audio> beside the orb, and clips.js audioOf must find it.
func TestOrbCansayClipResolvesInClipsJS(t *testing.T) {
	body := renderView(t, views.Progress(views.ProgressView{
		Page: views.Page{UI: views.UIFor("en", views.AddressMasculine)},
		CanSay: &views.CanSayPanel{
			TotalText: "1 thing",
			AddedText: "1 this week",
			Items:     []views.CanSayWord{{He: "פֹּה", Gloss: "here", Audio: "/audio/abc.mp3"}},
		},
	}))
	item := cansayItemHTML(t, body, "פֹּה")
	clip := regexp.MustCompile(`(?s)<div class="clip">(.*?)</div>`).FindStringSubmatch(item)
	if clip == nil {
		t.Fatalf("cansay item has no .clip:\n%s", item)
	}
	inner := clip[1]
	if !strings.Contains(inner, `<audio id="cansay-0"`) || !strings.Contains(inner, `src="/audio/abc.mp3"`) {
		t.Fatalf("clip is missing the word audio:\n%s", inner)
	}
	if !strings.Contains(inner, `class="orb orb--word"`) || !strings.Contains(inner, `data-src="/audio/abc.mp3"`) {
		t.Fatalf("clip is missing the orb:\n%s", inner)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Log("node not installed; HTML shape already proves clips.js audioOf would resolve via .clip")
		return
	}
	script := filepath.Join(t.TempDir(), "cansay-clip.js")
	js := `
const fs = require("fs");
const vm = require("vm");
const src = fs.readFileSync(process.argv[2], "utf8");
const html = fs.readFileSync(process.argv[3], "utf8");
function makeEl(tag) {
  const el = { tagName: String(tag).toUpperCase(), attrs: {}, kids: [], parent: null, className: "", isConnected: true };
  el.classList = { add(c) { el.className = (el.className + " " + c).trim(); } };
  el.setAttribute = (k, v) => { el.attrs[k] = String(v); };
  el.getAttribute = (k) => (Object.prototype.hasOwnProperty.call(el.attrs, k) ? el.attrs[k] : null);
  el.hasAttribute = (k) => Object.prototype.hasOwnProperty.call(el.attrs, k);
  el.removeAttribute = (k) => { delete el.attrs[k]; };
  el.append = (child) => { child.parent = el; el.kids.push(child); };
  el.closest = (sel) => { let n = el; while (n) { if (matches(n, sel)) return n; n = n.parent; } return null; };
  el.querySelector = (sel) => query(el, sel)[0] || null;
  el.querySelectorAll = (sel) => query(el, sel);
  el.play = () => ({ then(ok) { if (ok) ok(); return this; } });
  el.pause = () => {};
  return el;
}
function hasClass(el, name) { return (el.className || "").split(/\s+/).indexOf(name) >= 0; }
function matches(el, sel) {
  if (!el || !el.tagName) return false;
  if (sel === "audio") return el.tagName === "AUDIO";
  if (sel === ".clip") return hasClass(el, "clip");
  if (sel === ".orb") return hasClass(el, "orb");
  return false;
}
function query(root, sel) {
  const out = [];
  (function rec(n) {
    if (sel === ".clip > audio" && hasClass(n, "clip")) {
      n.kids.forEach((k) => { if (k.tagName === "AUDIO") out.push(k); });
    } else if (n !== root && matches(n, sel)) out.push(n);
    (n.kids || []).forEach(rec);
  })(root);
  return out;
}
const listeners = [];
const doc = makeEl("#document");
doc.documentElement = makeEl("html");
doc.append(doc.documentElement);
doc.addEventListener = (name, fn) => { listeners.push({ name, fn }); };
doc.querySelectorAll = (sel) => query(doc, sel);
doc.getElementById = (id) => {
  let found = null;
  (function rec(n) {
    if (n.getAttribute && n.getAttribute("id") === id) found = n;
    (n.kids || []).forEach(rec);
  })(doc);
  return found;
};
const body = makeEl("body");
doc.append(body);
const clip = makeEl("div");
clip.className = "clip";
const audio = makeEl("audio");
audio.setAttribute("id", "cansay-0");
audio.setAttribute("src", "/audio/abc.mp3");
const orb = makeEl("button");
orb.className = "orb orb--word";
orb.setAttribute("data-src", "/audio/abc.mp3");
orb.setAttribute("data-state", "idle");
orb.setAttribute("data-label", "Play here");
clip.append(audio);
clip.append(orb);
body.append(clip);
if (!html.includes('id="cansay-0"') || !html.includes('class="clip"')) {
  console.error("rendered HTML missing clip audio");
  process.exit(1);
}
const g = { document: doc, setTimeout() { return 1; }, clearTimeout() {} };
g.window = g;
vm.runInNewContext(src, g);
const click = listeners.find((l) => l.name === "click");
if (!click) { console.error("clips.js registered no click handler"); process.exit(1); }
click.fn({ target: orb, preventDefault() {} });
if (orb.getAttribute("data-state") === "error") {
  console.error("clips.js painted error; audioOf did not resolve the .clip audio");
  process.exit(1);
}
if (orb.getAttribute("data-state") !== "playing" && orb.getAttribute("data-state") !== "loading" && orb.getAttribute("data-state") !== "idle") {
  console.error("unexpected state " + orb.getAttribute("data-state"));
  process.exit(1);
}
`
	if err := os.WriteFile(script, []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
	htmlPath := filepath.Join(t.TempDir(), "cansay.html")
	if err := os.WriteFile(htmlPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", script, filepath.Join("static", "js", "clips.js"), htmlPath).CombinedOutput()
	if err != nil {
		t.Fatalf("clips.js resolve: %v\n%s", err, out)
	}
}

func assertCansayClips(t *testing.T, body string) {
	t.Helper()
	withAudio := cansayItemHTML(t, body, "פֹּה")
	if !regexp.MustCompile(`(?s)<div class="clip">.*<audio id="cansay-0"[^>]*src="/audio/abc.mp3".*</div>`).MatchString(withAudio) {
		t.Fatalf("cansay item with audio lacks <audio> inside .clip:\n%s", withAudio)
	}
	if !strings.Contains(withAudio, `class="orb orb--word"`) {
		t.Fatalf("cansay clip lacks its orb:\n%s", withAudio)
	}
	silent := cansayItemHTML(t, body, "בֹּקֶר")
	if strings.Contains(silent, "<audio") || strings.Contains(silent, `class="orb`) {
		t.Fatalf("word without a clip showed a play control:\n%s", silent)
	}
	next := cansayItemHTML(t, body, "כָּתַב")
	if !strings.Contains(next, `id="cansay-2"`) || !strings.Contains(next, `src="/audio/def.mp3"`) {
		t.Fatalf("Almost there row did not use a unique cansay id:\n%s", next)
	}
}

func assertVerseClip(t *testing.T, body string) {
	t.Helper()
	header := regexp.MustCompile(`(?s)<header class="isaiah-top">(.*?)</header>`).FindStringSubmatch(body)
	if header == nil {
		t.Fatalf("isaiah header missing:\n%s", body)
	}
	top := header[1]
	if !regexp.MustCompile(`(?s)<div class="clip">.*<audio id="verse-clip"[^>]*src="/audio/verse.mp3"`).MatchString(top) {
		t.Fatalf("verse orb is not in a .clip with its audio:\n%s", top)
	}
	if !strings.Contains(top, `class="orb orb--word"`) {
		t.Fatalf("verse clip lacks its orb:\n%s", top)
	}
}

func cansayItemHTML(t *testing.T, body, he string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<li class="cansay__item">(.*?)</li>`)
	for _, m := range re.FindAllStringSubmatch(body, -1) {
		if strings.Contains(m[1], he) {
			return m[0]
		}
	}
	t.Fatalf("no cansay item for %s", he)
	return ""
}

// assertOrbsWired fails when any button.orb cannot resolve audio the way clips.js does.
func assertOrbsWired(t *testing.T, body, page string) {
	t.Helper()
	audioIDs := map[string]bool{}
	for _, m := range audioIDRE.FindAllStringSubmatch(body, -1) {
		id := idAttrRE.FindStringSubmatch(m[1])
		if id != nil {
			audioIDs[id[1]] = true
		}
	}
	clipRanges := clipRanges(body)
	for _, m := range orbButtonRE.FindAllStringSubmatchIndex(body, -1) {
		attrs := body[m[2]:m[3]]
		classMatch := orbClassRE.FindStringSubmatch(attrs)
		if classMatch == nil || !hasClassToken(classMatch[1], "orb") {
			continue
		}
		if audio := orbAudioRE.FindStringSubmatch(attrs); audio != nil {
			if audioIDs[audio[1]] {
				continue
			}
			t.Errorf("%s: orb data-audio=%q names no <audio> on the page", page, audio[1])
			continue
		}
		start := m[0]
		if !insideClip(clipRanges, start) {
			t.Errorf("%s: orb at %d has neither data-audio nor a .clip parent:\n%s", page, start, body[start:min(len(body), start+180)])
		}
	}
}

func hasClassToken(classAttr, want string) bool {
	for _, c := range strings.Fields(classAttr) {
		if c == want {
			return true
		}
	}
	return false
}

type span struct{ start, end int }

func clipRanges(body string) []span {
	var out []span
	for _, m := range clipOpenRE.FindAllStringIndex(body, -1) {
		start := m[0]
		end := findClipEnd(body, m[1])
		if end > start {
			out = append(out, span{start, end})
		}
	}
	return out
}

// findClipEnd walks from just after the opening <div ...> of a .clip to its matching </div>.
func findClipEnd(body string, afterOpen int) int {
	depth := 1
	i := afterOpen
	for i < len(body) && depth > 0 {
		nextOpen := strings.Index(body[i:], "<div")
		nextClose := strings.Index(body[i:], "</div>")
		if nextClose < 0 {
			return -1
		}
		if nextOpen >= 0 && nextOpen < nextClose {
			depth++
			i += nextOpen + 4
			continue
		}
		depth--
		i += nextClose + len("</div>")
	}
	if depth != 0 {
		return -1
	}
	return i
}

func insideClip(ranges []span, pos int) bool {
	for _, r := range ranges {
		if pos >= r.start && pos < r.end {
			return true
		}
	}
	return false
}
