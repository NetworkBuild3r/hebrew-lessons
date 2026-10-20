package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// TestReviewClipFromAudioAsset is the review silence regression.
// Production lexemes keep audio_id empty: import and generate write
// audio_assets and the mp3, and the lesson player finds the clip with
// AudioByText. A review that only read audio_id rendered no <audio> and no orb.
func TestReviewClipFromAudioAsset(t *testing.T) {
	store, h, now, audioDir := openReviewAudio(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	text := "שָׁלוֹם"
	mp3 := reviewMP3(8)
	id := putReviewClip(t, store, audioDir, text, mp3)
	// audio_id is a foreign key, so the stale clip is a real row for a different
	// string. AudioByText of this word must still win, the way the lesson player does.
	stale := strings.Repeat("ab", 32)
	if _, err := store.InsertAudio(context.Background(), db.Asset{
		ID: stale, TextNFC: "not-this-word", Voice: "voice", Model: "legacy", Bytes: 1,
	}); err != nil {
		t.Fatal(err)
	}
	lex := addLexeme(t, store, text, "hello", "shalom", "m", stale)
	reviewed := now.Add(-48 * time.Hour)
	addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	body := getLearner(t, h, learner, "/review")
	assertReviewClip(t, body, id, true)
	if strings.Contains(body, "/audio/"+stale+".mp3") {
		t.Fatal("review played the empty-in-production lexeme audio_id instead of the audio_assets clip")
	}
	if strings.Contains(body, "onclick=") || strings.Contains(body, "hx-on") {
		t.Fatal("review clip uses an inline handler")
	}
	getAudio(t, h, id, mp3)
}

// TestReviewRecallRevealClip checks the htmx card swap. Recall hides the
// answer clip until reveal. The fragment is what replaces #review-card, so
// it carries the orb and a non-empty src, and it does not carry clips.js:
// that script stays on the page and upgrades the swap.
func TestReviewRecallRevealClip(t *testing.T) {
	store, h, now, audioDir := openReviewAudio(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	text := "בֹּקֶר"
	mp3 := reviewMP3(4)
	id := putReviewClip(t, store, audioDir, text, mp3)
	lex := addLexeme(t, store, text, "morning", "boker", "f", "")
	reviewed := now.Add(-48 * time.Hour)
	cardID := addCard(t, store, learner.user.ID, lex, fsrs.SkillRecall, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	page := getLearner(t, h, learner, "/review")
	if strings.Count(page, "/static/js/clips.js") != 1 {
		t.Fatalf("clips.js included %d times", strings.Count(page, "/static/js/clips.js"))
	}
	if strings.Contains(page, "/audio/") || strings.Contains(page, "<audio") || strings.Contains(page, `class="orb`) {
		t.Fatal("recall showed the answer clip before reveal")
	}

	shown := postReveal(t, h, learner, cardID, "https://learn.example")
	if shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d\n%s", shown.Code, shown.Body.String())
	}
	frag := shown.Body.String()
	assertReviewClip(t, frag, id, false)
	if strings.Contains(frag, "<script") || strings.Contains(frag, "onclick=") || strings.Contains(frag, "hx-on") {
		t.Fatal("the swapped card includes a script or an inline handler")
	}
	getAudio(t, h, id, mp3)
}

// TestReviewClipFallsBackToLexemeAudioID keeps a fixture that sets audio_id
// and does not insert audio_assets playable. The file is still served.
func TestReviewClipFallsBackToLexemeAudioID(t *testing.T) {
	store, h, now, audioDir := openReviewAudio(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	mp3 := reviewMP3(2)
	sum := sha256.Sum256(mp3)
	id := hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(audioDir, id+".mp3"), mp3, 0o644); err != nil {
		t.Fatal(err)
	}
	// The row exists so the lexeme foreign key holds, but its text is not this
	// word, so AudioByText misses and the card uses audio_id.
	if _, err := store.InsertAudio(context.Background(), db.Asset{
		ID: id, TextNFC: "not-mayim", Voice: "voice", Model: "legacy", Bytes: int64(len(mp3)),
	}); err != nil {
		t.Fatal(err)
	}
	lex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", id)
	reviewed := now.Add(-48 * time.Hour)
	addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	body := getLearner(t, h, learner, "/review")
	assertReviewClip(t, body, id, true)
	getAudio(t, h, id, mp3)
}

var reviewClipAudio = regexp.MustCompile(`<audio id="act-card-clip" src="(/audio/[0-9a-f]{64}\.mp3)" preload="none" controls>`)

func assertReviewClip(t *testing.T, body, id string, page bool) {
	t.Helper()
	src := "/audio/" + id + ".mp3"
	m := reviewClipAudio.FindStringSubmatch(body)
	if m == nil || m[1] != src {
		t.Fatalf("audio src = %v, want %s\n%s", m, src, body)
	}
	for _, want := range []string{
		`class="clip clip--hero"`,
		`class="orb orb--hero"`,
		`data-src="` + src + `"`,
		`aria-label="Play the word"`,
		`type="button"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Count(body, `id="act-card-clip"`) != 1 || strings.Count(body, `class="orb orb--hero"`) != 1 {
		t.Fatalf("want exactly one clip and one orb, got audio %d orb %d", strings.Count(body, `id="act-card-clip"`), strings.Count(body, `class="orb orb--hero"`))
	}
	if strings.Contains(body, `class="c-orb"`) {
		t.Fatal("the flashcard still uses the dialogue orb")
	}
	scripts := strings.Count(body, "/static/js/clips.js")
	if page && scripts != 1 {
		t.Fatalf("clips.js included %d times", scripts)
	}
	if !page && scripts != 0 {
		t.Fatalf("card fragment included clips.js %d times", scripts)
	}
}

func getAudio(t *testing.T, h http.Handler, id string, mp3 []byte) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/audio/"+id+".mp3", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /audio/%s.mp3 = %d", id, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "audio/mpeg" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), mp3) {
		t.Fatalf("audio body = %d bytes, want %d", rec.Body.Len(), len(mp3))
	}
}

func openReviewAudio(t *testing.T) (*db.Store, http.Handler, time.Time, string) {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	store, err := db.Open(context.Background(), filepath.Join(dir, "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	audioDir := filepath.Join(dir, "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		CookieSecure:  false,
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: []string{"a@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	study := session.New(store, fixedClock(now), 1, session.WithGate(func(context.Context, int64) (activity.Gate, error) {
		return activity.Gate{LettersPassed: false}, nil
	}))
	h := NewOptions(nil, svc, Options{Store: store, Runs: store, study: study, AudioDir: audioDir})
	return store, h, now, audioDir
}

// reviewMP3 is distinct bytes per n. serveAudio does not parse the file.
func reviewMP3(n byte) []byte {
	return []byte{0xFF, 0xFB, 0x90, 0x00, n}
}

func putReviewClip(t *testing.T, store *db.Store, audioDir, text string, mp3 []byte) string {
	t.Helper()
	sum := sha256.Sum256(mp3)
	id := hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(audioDir, id+".mp3"), mp3, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertAudio(context.Background(), db.Asset{
		ID: id, TextNFC: units.Normalize(text), Voice: "voice", Model: "eleven_v4", Bytes: int64(len(mp3)),
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

// TestReviewReadPickFeedbackClipFromAudioAsset answers a read_pick for a
// lexeme whose audio_id is empty. The clip row is the one import and generate
// write (sha256 id, normalized text, no lexeme column). The feedback fb-clip
// src is the URL AudioByText resolves.
func TestReviewReadPickFeedbackClipFromAudioAsset(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openReview(t, fixedClock(now), "https://learn.example")
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	words := seedAssetLexicon(t, store, learner.user.ID, now)
	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-activity") != "read_pick" {
		t.Fatalf("served %s, want read_pick: %s", attr(t, page, "data-activity"), cardAttrs(page))
	}
	if strings.Contains(page, "/audio/") || strings.Contains(page, "<audio") {
		t.Fatal("a reading prompt rendered a clip")
	}
	asked := hebrewOnPage(t, page, words)
	rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "choice", pickChoice(t, page, asked.gloss)))
	if rec.Code != http.StatusOK {
		t.Fatalf("answer = %d\n%s", rec.Code, rec.Body.String())
	}
	assertFBClip(t, store, sheetOf(t, rec.Body.String()), "fb-clip", asked.he)
}

// TestReviewWordMatchFeedbackClipFromAudioAsset answers a word_match the same
// way: no audio_id, an audio_assets row per word. A swapped pair puts the
// Hebrew on the feedback sheet, and each fb-clip src is AudioByText of that
// Hebrew.
func TestReviewWordMatchFeedbackClipFromAudioAsset(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openReview(t, fixedClock(now), "https://learn.example")
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	words := seedAssetLexicon(t, store, learner.user.ID, now)
	page := getLearner(t, h, learner, "/review")
	for visit := 1; visit <= 2; visit++ {
		if attr(t, page, "data-activity") != "read_pick" {
			t.Fatalf("visit %d is %s, want read_pick: %s", visit, attr(t, page, "data-activity"), cardAttrs(page))
		}
		asked := hebrewOnPage(t, page, words)
		rec := actPost(t, h, learner, "/_hx/review/answer", answerForm(page, "choice", pickChoice(t, page, asked.gloss)))
		if rec.Code != http.StatusOK {
			t.Fatalf("visit %d answer = %d\n%s", visit, rec.Code, rec.Body.String())
		}
		page = actPost(t, h, learner, "/_hx/review/next", nextForm(rec.Body.String())).Body.String()
	}
	if attr(t, page, "data-activity") != "word_match" {
		t.Fatalf("third visit is %s, want word_match: %s", attr(t, page, "data-activity"), cardAttrs(page))
	}
	if strings.Contains(page, "/audio/") || strings.Contains(page, "<audio") {
		t.Fatal("a word_match question rendered a clip")
	}
	rec := actPost(t, h, learner, "/_hx/review/answer", swappedWordMatch(t, page, words))
	if rec.Code != http.StatusOK {
		t.Fatalf("word_match = %d\n%s", rec.Code, rec.Body.String())
	}
	sheet := sheetOf(t, rec.Body.String())
	pairs := fbPairRe.FindAllStringSubmatch(sheet, -1)
	if len(pairs) == 0 {
		t.Fatalf("a swapped word_match has no feedback pairs:\n%s", sheet)
	}
	for _, p := range pairs {
		he := fbHeRe.FindStringSubmatch(p[1])
		src := fbClipRe.FindStringSubmatch(p[1])
		if he == nil || src == nil {
			t.Fatalf("a feedback pair has no Hebrew or no fb-clip:\n%s", p[1])
		}
		if src[1] != audioByTextURL(t, store, he[1]) {
			t.Fatalf("fb-clip %s for %s, AudioByText wants %s", src[1], he[1], audioByTextURL(t, store, he[1]))
		}
	}
}

// seedAssetLexicon is five glossed words with no audio_id. Each has an
// audio_assets row and an mp3 named by the sha256 of its bytes, which is how
// import and generate store a clip. The first three are due and struggling
// at reading, so the queue asks them through read_pick and then word_match
// instead of an unseen recall skill.
func seedAssetLexicon(t *testing.T, store *db.Store, userID int64, now time.Time) []lexWord {
	t.Helper()
	words := seedLexicon(t, store, now, 5, false, true)
	dir := filepath.Join(t.TempDir(), "audio")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range words {
		body := reviewMP3(byte(i + 1))
		sum := sha256.Sum256(body)
		id := hex.EncodeToString(sum[:])
		if err := os.WriteFile(filepath.Join(dir, id+".mp3"), body, 0o644); err != nil {
			t.Fatal(err)
		}
		text := units.Normalize(words[i].he)
		if _, err := store.InsertAudio(context.Background(), db.Asset{
			ID: id, TextNFC: text, Voice: "voice", Model: "eleven_v4", Bytes: int64(len(body)),
		}); err != nil {
			t.Fatal(err)
		}
		words[i].audio = id
	}
	giveCards(t, store, userID, now, words[:3])
	for _, w := range words[:3] {
		for i := 0; i < 3; i++ {
			at := now.Add(-time.Duration(i+1) * time.Hour)
			if _, err := store.ExecContext(context.Background(), `
				INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
				VALUES (?, ?, 'read', 'read_pick', 'review', '', '', 0, 0, 0, 0, ?)`,
				userID, w.id, db.FormatTime(at)); err != nil {
				t.Fatal(err)
			}
		}
	}
	return words
}

func hebrewOnPage(t *testing.T, page string, words []lexWord) lexWord {
	t.Helper()
	var hit lexWord
	n := 0
	for _, w := range words {
		if strings.Contains(page, w.he) {
			hit = w
			n++
		}
	}
	if n != 1 {
		t.Fatalf("page shows %d lexicon words, want 1", n)
	}
	return hit
}

func pickChoice(t *testing.T, page, gloss string) string {
	t.Helper()
	for _, o := range pickRe.FindAllStringSubmatch(page, -1) {
		if o[2] == gloss {
			return o[1]
		}
	}
	t.Fatalf("gloss %q is not an option", gloss)
	return ""
}

func audioByTextURL(t *testing.T, store *db.Store, text string) string {
	t.Helper()
	asset, ok, err := store.AudioByText(context.Background(), units.Normalize(text))
	if err != nil || !ok || asset.ID == "" {
		t.Fatalf("AudioByText(%q) = %+v ok=%v err=%v", text, asset, ok, err)
	}
	return "/audio/" + asset.ID + ".mp3"
}

func assertFBClip(t *testing.T, store *db.Store, sheet, id, he string) {
	t.Helper()
	want := audioByTextURL(t, store, he)
	if !strings.Contains(sheet, `<audio id="`+id+`" src="`+want+`"`) {
		t.Fatalf("feedback %s is not %s\n%s", id, want, sheet)
	}
}

var (
	fbPairRe = regexp.MustCompile(`(?s)<li class="fb__pair">(.*?)</li>`)
	fbHeRe   = regexp.MustCompile(`<bdi lang="he" dir="rtl">([^<]*)</bdi>`)
	fbClipRe = regexp.MustCompile(`<audio id="fb-clip-\d+" src="(/audio/[0-9a-f]{64}\.mp3)"`)
	btnRe    = regexp.MustCompile(`(?s)<button\b.*?</button>`)
	enBdiRe  = regexp.MustCompile(`<bdi lang="en" dir="ltr">([^<]*)</bdi>`)
)

func swappedWordMatch(t *testing.T, page string, words []lexWord) url.Values {
	t.Helper()
	byHe := map[string]string{}
	for _, w := range words {
		byHe[w.he] = w.gloss
	}
	glossIx := map[string]string{}
	var left [][2]string
	for _, b := range btnRe.FindAllString(page, -1) {
		end := strings.Index(b, ">")
		open := b[:end]
		tile := tagAttr(open, "data-tile")
		ix := tagAttr(open, "data-index")
		switch tile {
		case "r":
			g := enBdiRe.FindStringSubmatch(b)
			if g == nil {
				t.Fatalf("a gloss tile has no English: %s", b)
			}
			glossIx[g[1]] = ix
		case "l":
			h := fbHeRe.FindStringSubmatch(b)
			if h == nil {
				t.Fatalf("a Hebrew tile has no Hebrew: %s", b)
			}
			left = append(left, [2]string{ix, h[1]})
		}
	}
	form := answerForm(page)
	if len(left) != 5 {
		t.Fatalf("%d Hebrew tiles, want 5", len(left))
	}
	for _, pair := range left {
		gloss, ok := byHe[pair[1]]
		if !ok {
			t.Fatalf("tile Hebrew %q is not in the lexicon", pair[1])
		}
		right, ok := glossIx[gloss]
		if !ok {
			t.Fatalf("no gloss tile for %q", gloss)
		}
		form.Set("pair_"+pair[0], right)
	}
	a, b := form.Get("pair_0"), form.Get("pair_1")
	if a == "" || b == "" || a == b {
		t.Fatalf("pair_0=%q pair_1=%q, cannot swap", a, b)
	}
	form.Set("pair_0", b)
	form.Set("pair_1", a)
	return form
}

func tagAttr(tag, name string) string {
	m := regexp.MustCompile(name + `="([^"]*)"`).FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	return m[1]
}

// TestReviewOrbSpacePlaysOnce runs activity.js and clips.js under Node.
// Space on a review card starts the button.orb clip once. A click is left to
// clips.js, so the two listeners do not both call play(). Space on a dialogue
// c-orb still starts that clip.
func TestReviewOrbSpacePlaysOnce(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	script := filepath.Join(t.TempDir(), "review-space.js")
	if err := os.WriteFile(script, []byte(reviewSpaceCheckJS), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", script,
		filepath.Join("static", "js", "activity.js"),
		filepath.Join("static", "js", "clips.js"),
	).CombinedOutput()
	if err != nil {
		t.Fatalf("review space: %v\n%s", err, out)
	}
}

const reviewSpaceCheckJS = `
const fs = require("fs");
const vm = require("vm");
const activitySrc = fs.readFileSync(process.argv[2], "utf8");
const clipsSrc = fs.readFileSync(process.argv[3], "utf8");
const fails = [];
function check(cond, msg) { if (!cond) fails.push(msg); }

function makeEl(tag) {
  const el = {
    tagName: String(tag).toUpperCase(),
    attrs: {},
    kids: [],
    parent: null,
    isConnected: true,
    className: "",
    paused: true,
    currentTime: 0,
    ended: false,
    plays: 0,
  };
  el.classList = { add(c) { el.className = (el.className + " " + c).trim(); } };
  el.setAttribute = (k, v) => { el.attrs[k] = String(v); };
  el.getAttribute = (k) => (Object.prototype.hasOwnProperty.call(el.attrs, k) ? el.attrs[k] : null);
  el.hasAttribute = (k) => Object.prototype.hasOwnProperty.call(el.attrs, k);
  el.removeAttribute = (k) => { delete el.attrs[k]; };
  el.append = (child) => { child.parent = el; el.kids.push(child); };
  el.focus = () => {};
  el.closest = (sel) => { let n = el; while (n) { if (matches(n, sel)) return n; n = n.parent; } return null; };
  el.querySelector = (sel) => query(el, sel, false)[0] || null;
  el.querySelectorAll = (sel) => query(el, sel, false);
  el.play = () => { el.plays++; el.paused = false; const p = { then(ok) { if (ok) ok(); return p; } }; return p; };
  return el;
}
function hasClass(el, name) { return (el.className || "").split(/\s+/).indexOf(name) >= 0; }
function matches(el, sel) {
  if (!el || !el.tagName) return false;
  if (sel === "audio") return el.tagName === "AUDIO";
  if (sel === ".clip") return hasClass(el, "clip");
  if (sel === ".orb") return hasClass(el, "orb");
  if (sel === ".clip > audio") return false;
  if (sel === ".c-orb[data-src]") return hasClass(el, "c-orb") && el.hasAttribute("data-src");
  if (sel === ".c-orb[data-hint]") return hasClass(el, "c-orb") && el.hasAttribute("data-hint");
  if (sel === ".orb[data-src]") return hasClass(el, "orb") && el.hasAttribute("data-src");
  if (sel === ".act[data-phase]") return hasClass(el, "act") && el.hasAttribute("data-phase");
  if (sel === "button, a, input, textarea, select, summary, [contenteditable]") {
    return el.tagName === "BUTTON" || el.tagName === "A" || el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.tagName === "SELECT" || el.tagName === "SUMMARY";
  }
  return false;
}
function query(root, sel, includeSelf) {
  const out = [];
  (function rec(n, self) {
    if (self && sel === ".clip > audio" && hasClass(n, "clip")) {
      n.kids.forEach((k) => { if (k.tagName === "AUDIO") out.push(k); });
    } else if (self && matches(n, sel)) out.push(n);
    n.kids.forEach((k) => rec(k, true));
  })(root, includeSelf);
  return out;
}

const listeners = [];
const doc = makeEl("#document");
doc.documentElement = makeEl("html");
doc.append(doc.documentElement);
doc.readyState = "complete";
doc.addEventListener = (name, fn) => { listeners.push({ name, fn }); };
doc.querySelector = (sel) => query(doc, sel, false)[0] || null;
doc.querySelectorAll = (sel) => query(doc, sel, false);
doc.getElementById = (id) => {
  const all = [];
  (function rec(n) { if (n.getAttribute && n.getAttribute("id") === id) all.push(n); n.kids.forEach(rec); })(doc);
  return all[0] || null;
};
function fire(name, target) {
  listeners.filter((l) => l.name === name).forEach((l) => l.fn({
    key: name === "keydown" ? " " : "",
    target,
    ctrlKey: false, metaKey: false, altKey: false, isComposing: false,
    preventDefault() { this.defaultPrevented = true; },
  }));
}

const body = makeEl("body");
doc.append(body);
const frame = makeEl("section");
frame.className = "act";
frame.setAttribute("data-phase", "grade");
body.append(frame);

const prompt = makeEl("audio");
prompt.setAttribute("id", "act-prompt");
const corb = makeEl("button");
corb.className = "c-orb";
corb.setAttribute("data-src", "/prompt.mp3");
corb.setAttribute("data-audio", "act-prompt");
corb.setAttribute("data-hint", "replay");
frame.append(prompt);
frame.append(corb);

const windowMock = { document: doc, setTimeout() { return 1; }, clearTimeout() {} };
windowMock.window = windowMock;
vm.runInNewContext(activitySrc, { window: windowMock, globalThis: windowMock });
fire("keydown", body);
check(prompt.plays === 1, "Space did not play the dialogue orb once: " + prompt.plays);

frame.kids = [];
prompt.parent = null;
corb.parent = null;
const wrap = makeEl("div");
wrap.className = "clip clip--hero";
const word = makeEl("audio");
word.setAttribute("id", "act-card-clip");
const orb = makeEl("button");
orb.className = "orb orb--hero";
orb.setAttribute("data-src", "/word.mp3");
orb.setAttribute("data-state", "idle");
wrap.append(word);
wrap.append(orb);
frame.append(wrap);

vm.runInNewContext(clipsSrc, { window: windowMock });
const before = word.plays;
fire("keydown", body);
check(word.plays === before + 1, "Space on the review orb played " + (word.plays - before) + " times, want 1");
check(prompt.plays === 1, "Space on the review card also played the dialogue clip");
const afterSpace = word.plays;
fire("click", orb);
check(word.plays === afterSpace + 1, "clicking the review orb played " + (word.plays - afterSpace) + " times, want 1");
fire("keydown", orb);
check(word.plays === afterSpace + 1, "Space on the focused orb also started a play; the click from the button is the one play");

if (fails.length) {
  console.error(fails.join("\n"));
  process.exit(1);
}
`
