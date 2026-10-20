package web

import (
	"context"
	"net/http"
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
	if !strings.Contains(act, "Read the story. Tap a word to see what it means.") {
		t.Fatalf("story prompt missing the flip sentence:\n%s", act)
	}
	if !strings.Contains(act, "Tap a word to see what it means.") || !strings.Contains(act, "icon-flip") {
		t.Fatalf("flip hint missing:\n%s", act)
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
	if !strings.Contains(act, `<script type="module" src="/static/js/wcards.js"></script>`) {
		t.Fatalf("wcards.js missing:\n%s", act)
	}
	if !strings.Contains(act, `name="answer" value="continue"`) {
		t.Fatalf("continue is missing while cards are closed:\n%s", act)
	}
	if !strings.Contains(act, `data-wcards-all`) || !strings.Contains(act, "Show all meanings") || !strings.Contains(act, `aria-pressed="false"`) {
		t.Fatalf("show-all link missing:\n%s", act)
	}
	if !strings.Contains(act, "meanings=all") {
		t.Fatalf("show-all href missing meanings=all:\n%s", act)
	}
	if strings.Contains(act, "style=") || strings.Contains(act, "hx-on") || regexp.MustCompile(`(?i)\son[a-z]+\s*=`).MatchString(act) {
		t.Fatal("activity has an inline style or handler")
	}

	kept := getAuthed(t, h, cookie, storyPath+"?lang=en&from=today")
	if !strings.Contains(kept, "from=today") || !strings.Contains(kept, "lang=en") || !strings.Contains(kept, "meanings=all") {
		t.Fatalf("show-all dropped a query param:\n%s", activityHTML(t, kept))
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
	if !strings.Contains(opened, `aria-pressed="true"`) || !strings.Contains(opened, "Hide meanings") || !strings.Contains(opened, `data-wcards-all`) {
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
		}, "A greeting. Tap a word to see what it means."},
		{units.Unit{
			Title: "Shadow", Position: 1,
			Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
				{Type: units.ActivityShadow, Text: flipHello + " " + flipMorning},
			}}},
		}, "Listen, then say the line. Tap a word to see what it means."},
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
	if !strings.Contains(dlg, "Listen to the conversation. Tap a word to see what it means.") || !strings.Contains(dlg, "data-wcards-all") {
		t.Fatalf("dialogue flip copy missing:\n%s", dlg)
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
