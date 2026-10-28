package web

import (
	"context"
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
	if got := views.CardMeta("", "m"); got != "m" {
		t.Fatalf("CardMeta gender only = %q", got)
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
		{TextNFC: flipEvening, Gloss: "evening", Translit: sp("erev"), Gender: sp("m"), POS: sp("noun")},
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
	if strings.Count(act, "Tap a word to see what it means.") != 1 {
		t.Fatalf("flip hint should be the only tap sentence:\n%s", act)
	}
	cards := regexp.MustCompile(`(?s)<details class="wcard[^"]*".*?</details>`).FindAllString(act, -1)
	if len(cards) != 3 {
		t.Fatalf("story cards = %d, want 3:\n%s", len(cards), act)
	}
	var sawNew, sawPlain bool
	for _, card := range cards {
		front, _, ok := strings.Cut(card, "</summary>")
		if !ok {
			t.Fatalf("card has no summary:\n%s", card)
		}
		if strings.Contains(front, "orb") || strings.Contains(front, "<button") || strings.Contains(front, "<audio") {
			t.Fatalf("the word tile contains the play control:\n%s", card)
		}
		if !strings.Contains(card, "wcard__he") || !strings.Contains(card, "wcard__tr") || !strings.Contains(card, "wcard__gl") {
			t.Fatalf("card is missing a face:\n%s", card)
		}
		front, back, ok := strings.Cut(card, "</summary>")
		if !ok || !strings.Contains(front, "wcard__he") || strings.Contains(front, "wcard__tr") || strings.Contains(front, "wcard__gl") {
			t.Fatalf("the front should be the Hebrew word:\n%s", card)
		}
		if !strings.Contains(back, "wcard__gl") || !strings.Contains(back, "wcard__tr") {
			t.Fatalf("the reading and the meaning sit on the back:\n%s", card)
		}
		if strings.Contains(card, `wcard--new`) {
			sawNew = true
			if !strings.Contains(front, ">New<") {
				t.Fatalf("new card has no New label:\n%s", card)
			}
		}
		if strings.Contains(card, flipEvening) && strings.Contains(card, "noun · m") {
			sawPlain = true
		}
	}
	if !sawNew || !sawPlain {
		t.Fatalf("want a new card and a noun · m card:\n%s", act)
	}
	if !strings.Contains(act, `<span class="wpunct" lang="he">OK</span>`) {
		t.Fatalf("plain token is not outside a card:\n%s", act)
	}
	if !strings.Contains(act, `class="orb orb--word"`) || !strings.Contains(act, `lang="en" aria-label="Play `) || !strings.Contains(act, "/static/js/clips.js") {
		t.Fatalf("word orb or clips.js missing:\n%s", act)
	}
	lineTag := act[strings.Index(act, `<div class="wline"`):]
	lineTag = lineTag[:strings.Index(lineTag, ">")]
	if strings.Contains(lineTag, "lang=") {
		t.Fatalf("the line sets a language on the orbs: %s", lineTag)
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
	if strings.Contains(act, `role="button"`) || strings.Contains(act, "aria-pressed") {
		t.Fatalf("server HTML upgrades the show-all link:\n%s", act)
	}
	closedHref := showAllHref(t, act)
	if !strings.Contains(closedHref, "meanings=all") || !strings.Contains(act, "Show all meanings") {
		t.Fatalf("closed show-all href = %q\n%s", closedHref, act)
	}
	if strings.Contains(act, "style=") || strings.Contains(act, "hx-on") || regexp.MustCompile(`(?i)\son[a-z]+\s*=`).MatchString(act) {
		t.Fatal("activity has an inline style or handler")
	}

	kept := getAuthed(t, h, cookie, storyPath+"?lang=en&from=today")
	keptHref := showAllHref(t, activityHTML(t, kept))
	if !strings.Contains(keptHref, "from=today") || !strings.Contains(keptHref, "lang=en") || !strings.Contains(keptHref, "meanings=all") {
		t.Fatalf("show-all dropped a query param: %q", keptHref)
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
	openCards := regexp.MustCompile(`<details class="wcard(?: |")[^>]*>`).FindAllString(opened, -1)
	if len(openCards) != 3 {
		t.Fatalf("open cards = %d:\n%s", len(openCards), opened)
	}
	for _, tag := range openCards {
		if !strings.Contains(tag, "open") {
			t.Fatalf("card not open: %s", tag)
		}
	}
	openHref := showAllHref(t, opened)
	if strings.Contains(openHref, "meanings=all") || !strings.Contains(opened, "Hide meanings") || strings.Contains(opened, "aria-pressed") || strings.Contains(opened, `role="button"`) {
		t.Fatalf("open show-all href = %q\n%s", openHref, opened)
	}
	closedAgain := activityHTML(t, getAuthed(t, h, cookie, openHref))
	for _, tag := range regexp.MustCompile(`<details class="wcard(?: |")[^>]*>`).FindAllString(closedAgain, -1) {
		if strings.Contains(tag, " open") || strings.HasSuffix(tag, " open>") {
			t.Fatalf("hide link left a card open: %s", tag)
		}
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
	if !strings.Contains(one, `class="wline"`) {
		t.Fatalf("one flip word should share the flip line:\n%s", one)
	}
	if strings.Contains(one, "clip-rows") {
		t.Fatalf("a flip word with no audio has clip rows:\n%s", one)
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
		// A recall word is a card the learner can tap. Its back is only how the word sounds:
		// the meaning is never in the markup before the answer.
		if name != "recall" && (strings.Contains(page, "<details") || strings.Contains(page, "wcard")) {
			t.Fatalf("%s rendered a flip card:\n%s", name, page)
		}
		if name == "recall" && strings.Contains(page, "wcard__gl") {
			t.Fatalf("recall put the meaning on the card before grading:\n%s", page)
		}
		head := page
		if name == "recall" {
			head = stripSound(page)
		}
		if i := strings.Index(head, `class="choices"`); i >= 0 {
			head = head[:i]
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

func TestFlipCardIsNativeDisclosure(t *testing.T) {
	h, cookie, path, _ := flipLesson(t, units.Unit{
		Title: "Cards", Position: 1,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: flipHello + " " + flipMorning + " " + flipEvening},
		}}},
	}, false)
	act := activityHTML(t, getAuthed(t, h, cookie, path))
	cards := regexp.MustCompile(`(?s)<details class="wcard[^"]*"[^>]*>.*?</details>`).FindAllString(act, -1)
	if len(cards) != 3 {
		t.Fatalf("cards = %d:\n%s", len(cards), act)
	}
	for _, card := range cards {
		body := strings.TrimSpace(card[strings.Index(card, ">")+1:])
		if !strings.HasPrefix(body, "<summary") {
			t.Fatalf("summary is not the first child:\n%s", card)
		}
		front, back, ok := strings.Cut(card, "</summary>")
		if !ok || strings.Contains(front, "wcard__gl") || !strings.Contains(back, "wcard__gl") {
			t.Fatalf("gloss is not inside details and outside summary:\n%s", card)
		}
		for _, bad := range []string{" hidden", "hidden=", " inert", "inert=", "aria-hidden", "style="} {
			if strings.Contains(card, bad) {
				t.Fatalf("card has %s:\n%s", bad, card)
			}
		}
	}
}

func TestFlipCardsFullPageShowAll(t *testing.T) {
	h, cookie, path, _ := flipStory(t)
	for _, p := range []string{path, "/lesson"} {
		body := getAuthed(t, h, cookie, p)
		if !strings.Contains(body, "<html") {
			t.Fatalf("GET %s is not a full page", p)
		}
		assertTeachShowAll(t, body)
	}
}

func TestFlipCardsHTMXContinueShowAll(t *testing.T) {
	h, cookie, path, dbPath := flipAfterRecall(t)
	body := continueIntoTeach(t, h, cookie, path, dbPath, true)
	if strings.Contains(body, "<html") {
		t.Fatalf("htmx continue returned a full page:\n%s", body[:400])
	}
	assertTeachShowAll(t, body)
	if !strings.Contains(body, ">evening<") {
		t.Fatalf("htmx teach step is missing the back-face meaning:\n%s", body)
	}
}

func TestFlipCardsPlainContinueShowAll(t *testing.T) {
	h, cookie, path, dbPath := flipAfterRecall(t)
	body := continueIntoTeach(t, h, cookie, path, dbPath, false)
	if !strings.Contains(body, "<html") {
		t.Fatalf("plain continue did not return a full page:\n%s", body[:400])
	}
	assertTeachShowAll(t, body)
	if !strings.Contains(body, ">evening<") {
		t.Fatalf("plain continue is missing the back-face meaning:\n%s", body)
	}
}

func TestFlipCardsContinueAdvances(t *testing.T) {
	h, cookie, path, dbPath := flipThreeStories(t)
	body := getAuthed(t, h, cookie, path)
	runID := runValue(t, body)
	if code := postLesson(t, h, cookie, path, runID, "continue", true, lessonCSRF).Code; code != http.StatusOK {
		t.Fatalf("first continue = %d", code)
	}
	if got := scalar(t, dbPath, `SELECT activity_ix FROM lesson_runs WHERE id = ?`, runID); got != 1 {
		t.Fatalf("activity_ix after the first continue = %d, want 1", got)
	}
	page := getAuthed(t, h, cookie, path)
	const want = `<button type="submit" class="btn btn-primary" name="answer" value="continue">`
	if !strings.Contains(page, want) {
		t.Fatalf("Continue is missing or blocked:\n%s", activityHTML(t, page))
	}
	if code := postLesson(t, h, cookie, path, runID, "continue", true, lessonCSRF).Code; code != http.StatusOK {
		t.Fatalf("second continue = %d", code)
	}
	if got := scalar(t, dbPath, `SELECT activity_ix FROM lesson_runs WHERE id = ?`, runID); got != 2 {
		t.Fatalf("activity_ix after Continue = %d, want 2", got)
	}
	if scalar(t, dbPath, `SELECT finished_at IS NULL FROM lesson_runs WHERE id = ?`, runID) != 1 {
		t.Fatal("Continue finished the lesson")
	}
}

func TestMeaningsAllDoesNotAdvance(t *testing.T) {
	h, cookie, path, dbPath := flipThreeStories(t)
	body := getAuthed(t, h, cookie, path)
	runID := runValue(t, body)
	if code := postLesson(t, h, cookie, path, runID, "continue", true, lessonCSRF).Code; code != http.StatusOK {
		t.Fatalf("continue = %d", code)
	}
	before := scalar(t, dbPath, `SELECT activity_ix FROM lesson_runs WHERE id = ?`, runID)
	if before != 1 {
		t.Fatalf("fixture is not past step 0: ix %d", before)
	}
	if scalar(t, dbPath, `SELECT finished_at IS NULL FROM lesson_runs WHERE id = ?`, runID) != 1 {
		t.Fatal("the run was already finished")
	}
	attempts := scalar(t, dbPath, `SELECT COUNT(*) FROM attempts`)
	_ = getAuthed(t, h, cookie, path+"?meanings=all")
	if got := scalar(t, dbPath, `SELECT activity_ix FROM lesson_runs WHERE id = ?`, runID); got != before {
		t.Fatalf("?meanings=all moved activity_ix from %d to %d", before, got)
	}
	if scalar(t, dbPath, `SELECT finished_at IS NULL FROM lesson_runs WHERE id = ?`, runID) != 1 {
		t.Fatal("?meanings=all finished the run")
	}
	if scalar(t, dbPath, `SELECT COUNT(*) FROM attempts`) != attempts {
		t.Fatal("?meanings=all wrote an attempt")
	}
}

func flipStory(t *testing.T) (http.Handler, *http.Cookie, string, string) {
	t.Helper()
	return flipLesson(t, units.Unit{
		Title: "Story", Position: 1,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: flipHello + " " + flipMorning + " " + flipEvening},
		}}},
	}, false)
}

func flipThreeStories(t *testing.T) (http.Handler, *http.Cookie, string, string) {
	t.Helper()
	return flipLesson(t, units.Unit{
		Title: "Three", Position: 1,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: flipHello + " " + flipMorning},
			{Type: units.ActivityStory, Text: flipEvening + " " + flipNight + " " + flipWater},
			{Type: units.ActivityStory, Text: flipHello + " " + flipMorning},
		}}},
	}, false)
}

func flipAfterRecall(t *testing.T) (http.Handler, *http.Cookie, string, string) {
	t.Helper()
	return flipLesson(t, units.Unit{
		Title: "After", Position: 1, NewWords: []string{flipHello},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: flipHello},
			{Type: units.ActivityStory, Text: flipEvening + " " + flipNight + " " + flipWater},
			{Type: units.ActivityStory, Text: flipHello + " " + flipMorning},
		}}},
	}, false)
}

func continueIntoTeach(t *testing.T, h http.Handler, cookie *http.Cookie, path, dbPath string, hx bool) string {
	t.Helper()
	body := getAuthed(t, h, cookie, path)
	runID := runValue(t, body)
	id := scalar(t, dbPath, `SELECT id FROM lexemes WHERE text_nfc = ?`, flipHello)
	ans := postLesson(t, h, cookie, path, runID, itoa(int64(id)), true, lessonCSRF)
	if ans.Code != http.StatusOK {
		t.Fatalf("answer = %d\n%s", ans.Code, ans.Body.String())
	}
	var cont *httptest.ResponseRecorder
	if hx {
		cont = postLesson(t, h, cookie, path, runID, "continue", true, lessonCSRF)
	} else {
		cont = postLessonPlain(t, h, cookie, path, runID, "continue")
	}
	if cont.Code != http.StatusOK {
		t.Fatalf("continue = %d\n%s", cont.Code, cont.Body.String())
	}
	if scalar(t, dbPath, `SELECT activity_ix FROM lesson_runs WHERE id = ?`, runID) != 1 {
		t.Fatalf("continue did not reach the teach step")
	}
	return cont.Body.String()
}

func postLessonPlain(t *testing.T, h http.Handler, cookie *http.Cookie, path, runID, answer string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("run", runID)
	form.Set("answer", answer)
	form.Set("_csrf", lessonCSRF)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Host = "learn.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("X-CSRF-Token", lessonCSRF)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestFlipShellDefaults pins the shell and the show-all query.
// An empty lang is the English shell. lang=he is the Hebrew shell.
// Any other value stays English: Hebrew is opt-in. A Hebrew page whose
// request omitted lang puts lang=he on the show-all link, and an English
// page does not grow one.
func TestFlipShellDefaults(t *testing.T) {
	empty := lessonQueryFrom(httptest.NewRequest(http.MethodGet, "/lesson/1/0", nil), "")
	if empty.meaningsHref != "/lesson/1/0?meanings=all" {
		t.Fatalf("empty lang href = %q", empty.meaningsHref)
	}
	english := lessonQueryFrom(httptest.NewRequest(http.MethodGet, "/lesson/1/0", nil), "en")
	if english.meaningsHref != "/lesson/1/0?meanings=all" {
		t.Fatalf("english href = %q", english.meaningsHref)
	}
	hebrew := lessonQueryFrom(httptest.NewRequest(http.MethodGet, "/lesson/1/0", nil), "he")
	if hebrew.meaningsHref != "/lesson/1/0?lang=he&meanings=all" {
		t.Fatalf("hebrew href = %q", hebrew.meaningsHref)
	}
	other := lessonQueryFrom(httptest.NewRequest(http.MethodGet, "/lesson/1/0?lang=fr", nil), "en")
	if other.meaningsHref != "/lesson/1/0?lang=fr&meanings=all" {
		t.Fatalf("other href = %q", other.meaningsHref)
	}

	h, cookie, path, _ := flipLesson(t, units.Unit{
		Title: "Shell", Position: 1,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: flipHello + " " + flipMorning},
		}}},
	}, false)

	plain := getAuthed(t, h, cookie, path)
	if !strings.Contains(plain, `<html lang="en" dir="ltr"`) {
		t.Fatal("empty lang is not the English shell")
	}
	plainHref := showAllHref(t, activityHTML(t, plain))
	if strings.Contains(plainHref, "lang=") {
		t.Fatalf("English show-all gained lang: %q", plainHref)
	}

	heBody := getAuthedCookies(t, h, path, cookie, &http.Cookie{Name: langCookieName, Value: "he"})
	if !strings.Contains(heBody, `<html lang="he" dir="rtl"`) {
		t.Fatal("a Hebrew choice is not the Hebrew shell")
	}
	heHref := showAllHref(t, activityHTML(t, heBody))
	if !strings.Contains(heHref, "lang=he") {
		t.Fatalf("Hebrew show-all dropped lang: %q", heHref)
	}

	optIn := getAuthed(t, h, cookie, path+"?lang=he")
	if !strings.Contains(optIn, `<html lang="he" dir="rtl"`) {
		t.Fatal("lang=he is not the Hebrew shell")
	}

	foreign := getAuthed(t, h, cookie, path+"?lang=fr")
	if !strings.Contains(foreign, `<html lang="en" dir="ltr"`) {
		t.Fatal("lang=fr is not the English shell")
	}
	foreignHref := showAllHref(t, activityHTML(t, foreign))
	if !strings.Contains(foreignHref, "lang=fr") || strings.Contains(foreignHref, "lang=he") {
		t.Fatalf("lang=fr href = %q", foreignHref)
	}
}

func getAuthedCookies(t *testing.T, h http.Handler, path string, cookies ...*http.Cookie) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "learn.example"
	for _, c := range cookies {
		req.AddCookie(c)
	}
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d\n%s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func assertTeachShowAll(t *testing.T, body string) {
	t.Helper()
	act := activityHTML(t, body)
	href := showAllHref(t, act)
	if !strings.Contains(href, "meanings=all") || !strings.HasPrefix(href, "/lesson") {
		t.Fatalf("show-all href = %q", href)
	}
	if !strings.Contains(act, "<details class=\"wcard") || !strings.Contains(act, "wcard__gl") {
		t.Fatalf("teach step has no cards:\n%s", act)
	}
	if !strings.Contains(act, `/static/js/wcards.js`) {
		t.Fatal("wcards.js missing")
	}
	if strings.Contains(act, `role="button"`) || strings.Contains(act, "aria-pressed") {
		t.Fatalf("server HTML upgrades the link:\n%s", act)
	}
}

func showAllHref(t *testing.T, html string) string {
	t.Helper()
	tag := regexp.MustCompile(`<a\b[^>]*\bdata-wcards-all\b[^>]*>`).FindString(html)
	if tag == "" {
		t.Fatalf("show-all link missing:\n%s", html)
	}
	m := regexp.MustCompile(`href="([^"]*)"`).FindStringSubmatch(tag)
	if m == nil {
		t.Fatalf("show-all href missing: %s", tag)
	}
	return strings.NewReplacer("&amp;", "&", "&#34;", `"`, "&#39;", "'").Replace(m[1])
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
const btn = add(activity, el("a", { "data-wcards-all": "" }));
const label = add(btn, el("span", { "data-wcards-label": "" }));
label.text = "Show all meanings";
const a = add(activity, el("details", {}, "wcard"));
const b = add(activity, el("details", {}, "wcard"));

const sandbox = { document: doc };
vm.runInNewContext(src, sandbox);
check(btn.getAttribute("role") === "button", "load did not set role=button");
check(btn.getAttribute("aria-pressed") === null, "load set aria-pressed");

fire("click", btn);
check(a.open && b.open, "Show all did not open both cards");
check(label.text === "Hide meanings", "Show all left the label as " + label.text);

b.open = false;
fire("toggle", b);
check(label.text === "Show all meanings", "closing one card left the label as " + label.text);
check(a.open && !b.open, "the toggle listener changed which cards are open");

fire("keydown", btn, { key: " ", repeat: true });
check(a.open && !b.open, "a repeated Space toggled the cards");
check(label.text === "Show all meanings", "a repeated Space changed the label");

fire("keydown", btn, { key: " " });
check(a.open && b.open, "Space did not open the remaining card");
check(label.text === "Hide meanings", "Space left the label as " + label.text);

const swapped = add(activity, el("div"));
const late = add(swapped, el("a", { "data-wcards-all": "" }));
fire("htmx:load", swapped, { detail: { elt: swapped } });
check(late.getAttribute("role") === "button", "htmx:load did not set role=button");
check(late.getAttribute("aria-pressed") === null, "htmx:load set aria-pressed");

const before = listeners.length;
vm.runInNewContext(src, sandbox);
fire("keydown", btn, { key: " " });
check(listeners.length === before, "loading the script again added listeners");
check(!a.open && !b.open, "a doubled Space listener toggled twice");
check(label.text === "Show all meanings", "Space close left the label as " + label.text);

if (fails.length) {
  console.error(fails.join("\n"));
  process.exit(1);
}
`
