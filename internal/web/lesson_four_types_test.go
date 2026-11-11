package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Spellings are the lexicon text_nfc values from lookup on 2026-11-10.
const (
	fourPeace    = "שָׁלוֹם"
	fourMorning  = "בּוֹקֶר"
	fourWater    = "מַיִם"
	fourCoffee   = "קָפֶה"
	fourI        = "אֲנִי"
	fourHeWants  = "רוֹצֶה"
	fourSheWants = "רוֹצָה"
)

// TestFourLessonTypes walks one word lesson, one conjugation lesson,
// one listening lesson, and one easy lesson. Spelling is not a fifth type.
func TestFourLessonTypes(t *testing.T) {
	now := time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC)
	store, h := openShellAt(t, fixedClock(now), "https://learn.example", nil, nil)
	ids := seedFourLessons(t, store)
	learner := addLearner(t, store, "learner1@example.com", "sub-four", now)
	byText := fourIDs(t, store)

	word := walkLesson(t, h, learner, fmt.Sprintf("/lesson/%d/0", ids.word))
	if !hasStage(word.screens[0], "See") {
		t.Fatalf("word lesson did not open on See:\n%s", clip(word.screens[0], 500))
	}
	if !strings.Contains(word.screens[0], "See the four words") || !strings.Contains(word.screens[0], "See the words") {
		t.Fatalf("word lesson did not show its words:\n%s", clip(word.screens[0], 400))
	}
	// See shows a card for each new word, with its meaning, and a Next word button on a phone.
	if strings.Count(word.screens[0], `data-see-card`) != 4 || !strings.Contains(word.screens[0], "data-see-next") {
		t.Fatalf("See is not four cards with a Next word button:\n%s", clip(word.screens[0], 900))
	}
	if strings.Contains(word.screens[0], `class="wcard`) {
		t.Fatalf("See still shows tiles that hide the meaning:\n%s", clip(word.screens[0], 900))
	}
	// Write is typed: the same lesson reaches a Hebrew field, not three buttons.
	typed := false
	for _, s := range word.screens {
		if strings.Contains(s, `data-kind="produce"`) && strings.Contains(s, "data-hk-input") {
			typed = true
			if strings.Contains(s, `class="choices"`) || !hasStage(s, "Write") {
				t.Fatalf("the Write step is not a typed field:\n%s", clip(s, 600))
			}
		}
	}
	if !typed {
		t.Fatal("the word lesson never reached a typed Write step")
	}
	if !word.done {
		t.Fatalf("word lesson did not finish: %s", word.lastStep)
	}

	conj := walkLesson(t, h, learner, fmt.Sprintf("/lesson/%d/0", ids.conjugation))
	if !strings.Contains(conj.screens[0], "He wants and she wants") {
		t.Fatalf("conjugation lesson did not show the forms:\n%s", clip(conj.screens[0], 400))
	}
	if !conj.done {
		t.Fatalf("conjugation lesson did not finish: %s", conj.lastStep)
	}

	listen := walkLesson(t, h, learner, fmt.Sprintf("/lesson/%d/0", ids.listening))
	if !strings.Contains(listen.screens[0], "Listen") {
		t.Fatalf("listening lesson did not open on a listen step:\n%s", clip(listen.screens[0], 400))
	}
	if !listen.done {
		t.Fatalf("listening lesson did not finish: %s", listen.lastStep)
	}

	easy := walkLesson(t, h, learner, fmt.Sprintf("/lesson/%d/0", ids.easy))
	if !strings.Contains(easy.screens[0], "Read words you already have") || !strings.Contains(easy.screens[0], "No new words") {
		t.Fatalf("easy lesson did not open on the passage:\n%s", clip(easy.screens[0], 400))
	}
	// An easy lesson has no See, Check, Read, Write, Use row to promise.
	for _, s := range easy.screens {
		if strings.Contains(s, `class="lf-stages"`) {
			t.Fatalf("an easy lesson shows the stage row:\n%s", clip(s, 600))
		}
	}
	if !easy.done {
		t.Fatalf("easy lesson did not finish: %s", easy.lastStep)
	}

	t.Logf("word screens %d, conjugation %d, listening %d, easy %d", len(word.screens), len(conj.screens), len(listen.screens), len(easy.screens))
	t.Logf("listen target %s", byText[fourCoffee])
}

// TestSeedLocalFourLessons writes the same four lessons into data/local-dev
// when SEED_LOCAL_LESSONS=1, so the dev server can play them.
func TestSeedLocalFourLessons(t *testing.T) {
	if os.Getenv("SEED_LOCAL_LESSONS") != "1" {
		t.Skip("set SEED_LOCAL_LESSONS=1 to write the four lessons into data/local-dev")
	}
	path := filepath.Join("..", "..", "data", "local-dev", "hebrew.sqlite")
	store, err := db.Open(context.Background(), path, clock.System())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM units`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n > 0 {
		t.Fatalf("data/local-dev already has %d units; not adding a second set", n)
	}
	ids := seedFourLessons(t, store)
	t.Logf("word %d conjugation %d listening %d easy %d", ids.word, ids.conjugation, ids.listening, ids.easy)
}

type fourLessonIDs struct {
	word, conjugation, listening, easy int64
}

func seedFourLessons(t *testing.T, store *db.Store) fourLessonIDs {
	t.Helper()
	ctx := context.Background()
	sp := func(s string) *string { return &s }
	if _, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: fourPeace, Gloss: "peace", Translit: sp("shalom"), POS: sp("noun"), Gender: sp("m")},
		{TextNFC: fourMorning, Gloss: "morning", Translit: sp("boker"), POS: sp("noun"), Gender: sp("m")},
		{TextNFC: fourWater, Gloss: "water", Translit: sp("mayim"), POS: sp("noun"), Gender: sp("m")},
		{TextNFC: fourCoffee, Gloss: "coffee", Translit: sp("kafe"), POS: sp("noun"), Gender: sp("m")},
		{TextNFC: fourI, Gloss: "I", Translit: sp("ani"), POS: sp("pron")},
		{TextNFC: fourHeWants, Gloss: "wants (he)", Translit: sp("rotse"), POS: sp("verb")},
		{TextNFC: fourSheWants, Gloss: "wants (she)", Translit: sp("rotsa"), POS: sp("verb")},
	}); err != nil {
		t.Fatal(err)
	}
	words := joinFour(fourPeace, fourMorning, fourWater, fourCoffee)
	wantCoffee := joinFour(fourI, fourHeWants, fourCoffee)
	passage := joinFour(fourPeace, fourMorning, fourI, fourHeWants, fourCoffee, fourWater)
	known := joinFour(fourPeace, fourMorning, fourI, fourHeWants, fourCoffee, fourI, fourHeWants, fourWater)

	word := publishLesson(t, store, units.Unit{
		Title: "Word lesson", Situation: "Morning coffee.", Concept: "See the four words", Position: 0,
		NewWords: []string{fourPeace, fourMorning, fourWater, fourCoffee},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityPattern, Text: words},
			{Type: units.ActivityRecall, Text: fourPeace},
			{Type: units.ActivityStory, Text: passage},
			{Type: units.ActivityProduce, Text: fourCoffee},
			{Type: units.ActivityOrder, Text: wantCoffee, Translation: "I want coffee"},
		}}},
	})
	conj := publishLesson(t, store, units.Unit{
		Title: "Conjugation", Situation: "He wants and she wants.", Concept: "He wants and she wants", Position: 1,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityPattern, Text: joinFour(fourHeWants, fourSheWants)},
			{Type: units.ActivityRecall, Text: fourSheWants},
			{Type: units.ActivityStory, Text: joinFour(fourHeWants, fourCoffee, fourSheWants, fourCoffee)},
		}}},
	})
	listen := publishLesson(t, store, units.Unit{
		Title: "Listening", Situation: "A new line, from words you know.", Concept: "Hear a new line", Position: 2,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityListenPick, Text: joinFour(fourPeace, fourMorning, fourI, fourHeWants, fourWater), Translation: "Good morning. I want water.", Target: fourWater},
		}}},
	})
	easy := publishLesson(t, store, units.Unit{
		Title: "Easy lesson", Situation: "No new words.", Concept: "Words you already have", Position: 3,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: known},
			{Type: units.ActivityRecall, Text: fourWater},
		}}},
	})
	return fourLessonIDs{word: word, conjugation: conj, listening: listen, easy: easy}
}

func publishLesson(t *testing.T, store *db.Store, doc units.Unit) int64 {
	t.Helper()
	id := putLesson(t, store, nil, doc, "shared")
	publish(t, store, id)
	return id
}

func joinFour(parts ...string) string {
	return strings.Join(parts, " ")
}

func fourIDs(t *testing.T, store *db.Store) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, text := range []string{fourPeace, fourMorning, fourWater, fourCoffee, fourI, fourHeWants, fourSheWants} {
		id, _, _, _, ok, err := store.LexemeByText(context.Background(), text)
		if err != nil || !ok {
			t.Fatalf("lexeme %s: %v ok=%v", text, err, ok)
		}
		out[text] = fmt.Sprint(id)
	}
	return out
}

type lessonWalk struct {
	screens  []string
	done     bool
	lastStep string
}

func hasStage(body, name string) bool {
	return strings.Contains(body, `data-state="now" aria-current="step"><bdi lang="en" dir="ltr">`+name+`</bdi>`)
}

func walkLesson(t *testing.T, h http.Handler, learner learnerFix, path string) lessonWalk {
	t.Helper()
	body := getAuthed(t, h, learner.cookie, path)
	runID := runValue(t, body)
	var walk lessonWalk
	for i := 0; i < 16; i++ {
		walk.screens = append(walk.screens, body)
		walk.lastStep = dataStep(body)
		if lessonDone(body) {
			walk.done = true
			return walk
		}
		before := body
		var posted *httptest.ResponseRecorder
		if strings.Contains(body, "data-order") && !strings.Contains(body, "That's right") && !strings.Contains(body, "Not quite") {
			posted = postBuiltSentence(t, h, learner, path, runID, body)
		} else {
			posted = postLesson(t, h, learner.cookie, path, runID, answerOnScreen(body), true, learner.csrf)
		}
		if posted.Code != 200 {
			t.Fatalf("%s post = %d\n%s", path, posted.Code, clip(posted.Body.String(), 300))
		}
		body = posted.Body.String()
		if body == before {
			t.Fatalf("%s stuck on step %s\n%s", path, dataStep(body), clip(body, 600))
		}
	}
	return walk
}

func postBuiltSentence(t *testing.T, h http.Handler, learner learnerFix, path, runID, body string) *httptest.ResponseRecorder {
	t.Helper()
	served := servedOrderChips(t, body)
	form := url.Values{}
	form.Set("run", runID)
	form.Set("answer", "check")
	form.Set("_csrf", learner.csrf)
	for _, p := range positionsForTokens(served, []string{fourI, fourHeWants, fourCoffee}) {
		form.Add("order", strconv.Itoa(p))
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Host = "learn.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("X-CSRF-Token", learner.csrf)
	req.AddCookie(learner.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func lessonDone(body string) bool {
	return strings.Contains(body, "Lesson complete") || strings.Contains(body, ">All done<")
}

func answerOnScreen(body string) string {
	if strings.Contains(body, "That's right") || strings.Contains(body, "Not quite") {
		return "continue"
	}
	// A typed Write step: the heading is the English cue, so type the Hebrew for it.
	if strings.Contains(body, "data-hk-input") {
		for gloss, he := range map[string]string{"coffee": fourCoffee, "peace": fourPeace, "water": fourWater} {
			if strings.Contains(body, `lang="en" dir="ltr">`+gloss+`</h1>`) {
				return he
			}
		}
	}
	for _, tag := range regexp.MustCompile(`<button\b[^>]*>`).FindAllString(body, -1) {
		if !strings.Contains(tag, `name="answer"`) {
			continue
		}
		if m := regexp.MustCompile(`value="(\d+)"`).FindStringSubmatch(tag); m != nil {
			return m[1]
		}
		if m := regexp.MustCompile(`value="([^"]+)"`).FindStringSubmatch(tag); m != nil {
			return m[1]
		}
	}
	for _, answer := range []string{"continue", "next", "said"} {
		if strings.Contains(body, `value="`+answer+`"`) {
			return answer
		}
	}
	return "continue"
}
