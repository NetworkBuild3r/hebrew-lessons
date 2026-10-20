package web

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func TestCardMeta(t *testing.T) {
	if got := views.CardMeta("verb", "m"); got != "verb · m" {
		t.Fatalf("CardMeta(verb, m) = %q", got)
	}
	if got := views.CardMeta("pron", ""); got != "pronoun" {
		t.Fatalf("CardMeta(pron) = %q", got)
	}
	if got := views.CardMeta("adj", "f"); got != "adjective · f" {
		t.Fatalf("CardMeta(adj, f) = %q", got)
	}
	if got := views.CardMeta("", ""); got != "" {
		t.Fatalf("CardMeta empty = %q", got)
	}
	if got := views.CardMeta("", "x"); got != "" {
		t.Fatalf("CardMeta unknown gender = %q", got)
	}
	if got := views.POSLabel("question word"); got != "question word" {
		t.Fatalf("POSLabel question = %q", got)
	}
}

const (
	flipHello   = "שָׁלוֹם"
	flipMorning = "בֹּקֶר"
	flipEvening = "עֶרֶב"
	flipNight   = "לַיְלָה"
	flipWater   = "מַיִם"
)

// flipLesson publishes one unit as the learner's next lesson and returns its page.
func flipLesson(t *testing.T, doc units.Unit, withAudio bool) (h http.Handler, cookie *http.Cookie, path, dbPath string) {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	dbPath = filepath.Join(t.TempDir(), "hebrew.sqlite")
	store, err := db.Open(ctx, dbPath, fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	brian, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	if _, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: flipHello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: flipMorning, Gloss: "morning", Translit: sp("boker"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: flipEvening, Gloss: "evening", Translit: sp("erev"), Gender: sp("m"), POS: sp("verb")},
		{TextNFC: flipNight, Gloss: "night", Translit: sp("layla"), Gender: sp("f"), POS: sp("noun")},
		{TextNFC: flipWater, Gloss: "water", Translit: sp("mayim"), POS: sp("noun")},
	}); err != nil {
		t.Fatal(err)
	}
	if doc.Position == 0 {
		doc.Position = 1
	}
	id := putLesson(t, store, nil, doc, "shared")
	publish(t, store, id)
	if withAudio {
		if _, err := store.InsertAudio(ctx, db.Asset{
			ID: strings.Repeat("cd", 32), TextNFC: units.Normalize(flipHello), Voice: "voice", Model: "eleven_v4", Bytes: 4,
		}); err != nil {
			t.Fatal(err)
		}
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h = NewOptions(nil, svc, Options{Store: store})
	cookie = mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour))
	path = "/lesson/" + itoa(id) + "/0"
	return h, cookie, path, dbPath
}

func TestFlipCardsOnTeachSteps(t *testing.T) {
	pool := flipEvening + " " + flipNight + " " + flipWater
	storyText := flipHello + " " + flipMorning + " " + flipEvening + " OK"
	h, cookie, storyPath, dbPath := flipLesson(t, units.Unit{
		Title: "Shared hello", Situation: "Saying hello.", Concept: "Greeting", Position: 1,
		NewWords: []string{flipHello, flipMorning},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: storyText},
		}}},
	}, true)
	body := getAuthed(t, h, cookie, storyPath)
	act := activityHTML(t, body)
	if !strings.Contains(act, "Read the story. Each word shows its sound and its meaning.") {
		t.Fatalf("story prompt missing:\n%s", act)
	}
	if strings.Count(act, "Tap a word to see what it means.") != 1 || !strings.Contains(act, "icon-flip") {
		t.Fatalf("flip hint should be the only tap sentence:\n%s", act)
	}
	cards := regexp.MustCompile(`(?s)<details class="wcard[^"]*".*?</details>`).FindAllString(act, -1)
	if len(cards) != 3 {
		t.Fatalf("story cards = %d, want 3:\n%s", len(cards), act)
	}
	var sawNew, sawPlain bool
	for _, card := range cards {
		if strings.Contains(card, "orb") || strings.Contains(card, "<button") || strings.Contains(card, "<audio") {
			t.Fatalf("a card contains the play control:\n%s", card)
		}
		if !strings.Contains(card, "wcard__he") || !strings.Contains(card, "wcard__tr") || !strings.Contains(card, "wcard__gl") {
			t.Fatalf("card is missing a face:\n%s", card)
		}
		front, back, ok := strings.Cut(card, "</summary>")
		if !ok || !strings.Contains(front, "wcard__he") || !strings.Contains(front, "wcard__tr") || strings.Contains(front, "wcard__gl") {
			t.Fatalf("front/back split wrong:\n%s", card)
		}
		if !strings.Contains(back, "wcard__gl") {
			t.Fatalf("meaning is not on the back:\n%s", card)
		}
		if strings.Contains(card, `wcard--new`) {
			sawNew = true
			if !strings.Contains(front, ">New<") {
				t.Fatalf("new card has no New label:\n%s", card)
			}
		}
		if strings.Contains(card, flipEvening) && strings.Contains(card, "verb · m") {
			sawPlain = true
		}
	}
	if !sawNew || !sawPlain {
		t.Fatalf("want a new card and a verb · m card:\n%s", act)
	}
	if !strings.Contains(act, `<span class="wpunct">OK</span>`) {
		t.Fatalf("plain token is not outside a card:\n%s", act)
	}
	if !strings.Contains(act, `class="orb orb--word"`) || !strings.Contains(act, "/static/js/clips.js") {
		t.Fatalf("word orb or clips.js missing:\n%s", act)
	}
	rowsAt := strings.Index(act, `class="clip-rows"`)
	lineAt := strings.Index(act, `class="wline"`)
	if rowsAt < 0 || lineAt < 0 || rowsAt < lineAt {
		t.Fatalf("no-JS word rows are missing under the line:\n%s", act)
	}
	line := act[lineAt:rowsAt]
	if strings.Contains(line, "<audio") {
		t.Fatalf("the card slot still holds the native bar:\n%s", line)
	}
	rows := act[rowsAt:]
	if end := strings.Index(rows, "<form"); end >= 0 {
		rows = rows[:end]
	}
	if strings.Count(rows, `class="clip-row"`) != 1 {
		t.Fatalf("no-JS word rows = %d, want 1:\n%s", strings.Count(rows, `class="clip-row"`), rows)
	}
	for _, need := range []string{flipHello, "shalom", "controls", `aria-label="Play ` + flipHello + `"`, `id="word-clip-0"`} {
		if !strings.Contains(rows, need) {
			t.Fatalf("word row missing %s:\n%s", need, rows)
		}
	}
	if !strings.Contains(line, `data-audio="word-clip-0"`) {
		t.Fatalf("word orb does not name its bar:\n%s", line)
	}
	if !strings.Contains(act, `<script type="module" src="/static/js/wcards.js"></script>`) {
		t.Fatalf("wcards.js missing:\n%s", act)
	}
	if !strings.Contains(act, `name="answer" value="continue"`) {
		t.Fatalf("continue is missing while cards are closed:\n%s", act)
	}
	if !strings.Contains(act, `<button type="submit" class="c-button" data-kind="text" data-wcards-all>`) || !strings.Contains(act, "Show all meanings") {
		t.Fatalf("show-all button missing:\n%s", act)
	}
	if strings.Contains(act, "aria-pressed") {
		t.Fatalf("show-all still exposes aria-pressed:\n%s", act)
	}
	if !strings.Contains(act, `name="meanings" value="all"`) {
		t.Fatalf("show-all form missing meanings=all:\n%s", act)
	}
	if strings.Contains(act, "style=") || strings.Contains(act, "hx-on") || regexp.MustCompile(`(?i)\son[a-z]+\s*=`).MatchString(act) {
		t.Fatal("activity has an inline style or handler")
	}

	kept := getAuthed(t, h, cookie, storyPath+"?lang=en&from=today")
	keptAct := activityHTML(t, kept)
	if !strings.Contains(keptAct, `name="from" value="today"`) || !strings.Contains(keptAct, `name="lang" value="en"`) || !strings.Contains(keptAct, `name="meanings" value="all"`) {
		t.Fatalf("show-all dropped a query param:\n%s", keptAct)
	}

	before := scalar(t, dbPath, `SELECT COUNT(*) FROM attempts`)
	runID := runValue(t, body)
	ixBefore := scalar(t, dbPath, `SELECT activity_ix FROM lesson_runs WHERE id = ?`, runID)
	openAll := getAuthed(t, h, cookie, storyPath+"?meanings=all")
	if scalar(t, dbPath, `SELECT COUNT(*) FROM attempts`) != before {
		t.Fatal("?meanings=all wrote an attempt")
	}
	if scalar(t, dbPath, `SELECT activity_ix FROM lesson_runs WHERE id = ?`, runID) != ixBefore {
		t.Fatal("?meanings=all advanced the run")
	}
	opened := activityHTML(t, openAll)
	openCards := regexp.MustCompile(`<details class="wcard[^"]*"[^>]*>`).FindAllString(opened, -1)
	if len(openCards) != 3 {
		t.Fatalf("open cards = %d:\n%s", len(openCards), opened)
	}
	for _, tag := range openCards {
		if !strings.Contains(tag, "open") {
			t.Fatalf("card not open: %s", tag)
		}
	}
	if !strings.Contains(opened, "Hide meanings") || !strings.Contains(opened, `data-wcards-all`) || strings.Contains(opened, "aria-pressed") {
		t.Fatalf("open state label missing:\n%s", opened)
	}

	oneH, oneCookie, onePath, _ := flipLesson(t, units.Unit{
		Title: "One word", Position: 1,
		NewWords: []string{flipHello},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: flipHello},
		}}},
	}, false)
	one := activityHTML(t, getAuthed(t, oneH, oneCookie, onePath))
	if !strings.Contains(one, "<details class=\"wcard") || strings.Contains(one, "data-wcards-all") {
		t.Fatalf("a single card should have no show-all control:\n%s", one)
	}

	for _, c := range []struct {
		doc    units.Unit
		prompt string
	}{
		{units.Unit{
			Title: "Pattern", Concept: "A greeting", Position: 1,
			Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
				{Type: units.ActivityPattern, Text: flipHello + " " + flipMorning},
			}}},
		}, "A greeting"},
		{units.Unit{
			Title: "Shadow", Position: 1,
			Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
				{Type: units.ActivityShadow, Text: flipHello + " " + flipMorning},
			}}},
		}, "Listen, then say the line."},
	} {
		hh, ck, p, _ := flipLesson(t, c.doc, false)
		page := activityHTML(t, getAuthed(t, hh, ck, p))
		if !strings.Contains(page, "<details class=\"wcard") || !strings.Contains(page, c.prompt) {
			t.Fatalf("teach step missing cards or prompt %q:\n%s", c.prompt, page)
		}
	}

	dh, dCookie, dPath, _ := flipLesson(t, units.Unit{
		Title: "Talk", Position: 1,
		NewWords: []string{flipHello},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{{
			Type: units.ActivityDialogue,
			Lines: []units.DialogueLine{
				{Speaker: "a", Text: flipHello},
				{Speaker: "b", Text: flipMorning},
			},
		}}}},
	}, false)
	dlg := activityHTML(t, getAuthed(t, dh, dCookie, dPath))
	if strings.Count(dlg, "<details class=\"wcard") < 2 || strings.Contains(dlg, "orb--word") {
		t.Fatalf("dialogue cards or a per-word orb:\n%s", dlg)
	}
	if !strings.Contains(dlg, "Listen to the conversation. Tap a line to hear it again.") || !strings.Contains(dlg, "data-wcards-all") {
		t.Fatalf("dialogue flip copy missing:\n%s", dlg)
	}
	if strings.Count(dlg, "Tap a word to see what it means.") != 1 {
		t.Fatalf("dialogue repeats the tap sentence:\n%s", dlg)
	}
	if strings.Contains(dlg, `wcard--new`) && !strings.Contains(dlg, ">New<") {
		t.Fatal("dialogue new word has no label")
	}

	assertNoAnswerLeak := func(name string, doc units.Unit, gloss, translit string) {
		t.Helper()
		hh, ck, p, _ := flipLesson(t, doc, false)
		page := activityHTML(t, getAuthed(t, hh, ck, p))
		if strings.Contains(page, "<details") || strings.Contains(page, "wcard") {
			t.Fatalf("%s rendered a flip card:\n%s", name, page)
		}
		head := page
		if i := strings.Index(page, `class="choices"`); i >= 0 {
			head = page[:i]
		}
		for _, leak := range []string{gloss, translit, "wcard__gl", "wb__gl"} {
			if leak != "" && strings.Contains(head, leak) {
				t.Fatalf("%s shows %q before grading:\n%s", name, leak, head)
			}
		}
	}
	distractors := []units.Activity{{Type: units.ActivityStory, Text: pool}}
	assertNoAnswerLeak("recall", units.Unit{
		Title: "Recall", Position: 1, NewWords: []string{flipHello},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: append([]units.Activity{
			{Type: units.ActivityRecall, Text: flipHello},
		}, distractors...)}},
	}, ">hello<", "shalom")
	assertNoAnswerLeak("listen_pick", units.Unit{
		Title: "Listen", Position: 1, NewWords: []string{flipHello},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: append([]units.Activity{
			{Type: units.ActivityListenPick, Text: flipHello},
		}, distractors...)}},
	}, ">hello<", "shalom")
	assertNoAnswerLeak("cloze", units.Unit{
		Title: "Cloze", Position: 1, NewWords: []string{flipHello},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: append([]units.Activity{
			{Type: units.ActivityCloze, Text: flipHello + " " + flipMorning, Target: flipHello},
		}, distractors...)}},
	}, "hello", "shalom")
}

// TestWCardsLabelFollowsToggle runs wcards.js under node. Show all sets the
// label to "Hide meanings". Closing one card fires a capture-phase toggle,
// and the label returns to "Show all meanings". Space activates the button.
func TestWCardsLabelFollowsToggle(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	script := filepath.Join(t.TempDir(), "wcards-check.js")
	if err := os.WriteFile(script, []byte(wcardCheckJS), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", script, filepath.Join("static", "js", "wcards.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("wcards.js: %v\n%s", err, out)
	}
}

const wcardCheckJS = `
const fs = require("fs");
const vm = require("vm");
const src = fs.readFileSync(process.argv[2], "utf8");
const fails = [];
function check(cond, msg) { if (!cond) fails.push(msg); }

function el(tag, attrs, className) {
  const node = { tag: tag, attrs: attrs || {}, className: className || "", kids: [], parent: null, text: "" };
  node.setAttribute = (k, v) => { node.attrs[k] = String(v); };
  node.getAttribute = (k) => (Object.prototype.hasOwnProperty.call(node.attrs, k) ? node.attrs[k] : null);
  node.hasAttribute = (k) => Object.prototype.hasOwnProperty.call(node.attrs, k);
  node.closest = (sel) => { let n = node; while (n) { if (matches(n, sel)) return n; n = n.parent; } return null; };
  node.querySelector = (sel) => query(node, sel)[0] || null;
  node.querySelectorAll = (sel) => query(node, sel);
  node.matches = (sel) => matches(node, sel);
  Object.defineProperty(node, "textContent", { get() { return node.text; }, set(v) { node.text = String(v); } });
  Object.defineProperty(node, "open", { get() { return !!node._open; }, set(v) { node._open = !!v; }, configurable: true });
  return node;
}
function add(parent, child) { child.parent = parent; parent.kids.push(child); return child; }
function hasClass(node, name) { return (node.className || "").split(/\s+/).indexOf(name) >= 0; }
function matches(node, sel) {
  if (sel === "#activity") return node.attrs.id === "activity";
  if (sel === "details.wcard") return node.tag === "details" && hasClass(node, "wcard");
  if (sel === "[data-wcards-all]") return node.hasAttribute("data-wcards-all");
  if (sel === "[data-wcards-label]") return node.hasAttribute("data-wcards-label");
  return false;
}
function query(root, sel) {
  const out = [];
  (function rec(n) { n.kids.forEach((k) => { if (matches(k, sel)) out.push(k); rec(k); }); })(root);
  return out;
}

const listeners = [];
const doc = el("#document");
doc.addEventListener = (name, fn) => { listeners.push({ name, fn }); };
function fire(name, target, extra) {
  const ev = Object.assign({ target, preventDefault() {}, stopPropagation() {} }, extra || {});
  listeners.filter((l) => l.name === name).forEach((l) => l.fn(ev));
  return ev;
}

const activity = add(doc, el("div", { id: "activity" }));
const form = add(activity, el("form"));
const btn = add(form, el("button", { "data-wcards-all": "" }));
const label = add(btn, el("span", { "data-wcards-label": "" }));
label.text = "Show all meanings";
const a = add(activity, el("details", {}, "wcard"));
const b = add(activity, el("details", {}, "wcard"));

const sandbox = { document: doc };
vm.runInNewContext(src, sandbox);

fire("click", btn);
check(a.open && b.open, "Show all did not open both cards");
check(label.text === "Hide meanings", "Show all left the label as " + label.text);

b.open = false;
fire("toggle", b);
check(label.text === "Show all meanings", "closing one card left the label as " + label.text);
check(a.open && !b.open, "the toggle listener changed which cards are open");

fire("keydown", btn, { key: " " });
check(a.open && b.open, "Space did not open the remaining card");
check(label.text === "Hide meanings", "Space left the label as " + label.text);

const before = listeners.length;
vm.runInNewContext(src, sandbox);
fire("keydown", btn, { key: " " });
check(listeners.length === before, "loading the script again added listeners");
check(!a.open && !b.open, "a doubled Space listener toggled twice");
check(label.text === "Show all meanings", "Space close left the label as " + label.text);

const submit = fire("submit", form);
check(label.text === "Show all meanings", "a stray submit changed the label");

if (fails.length) {
  console.error(fails.join("\n"));
  process.exit(1);
}
`
