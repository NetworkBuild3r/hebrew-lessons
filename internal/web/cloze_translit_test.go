package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Choice transliterations are the readings of the buttons, fixed here so the
// assertion does not read them back out of the handler.
var clozeChoiceTranslits = []string{"TL-lomed", "TL-kotev", "TL-okhel", "TL-shoteh"}

// TestClozeHidesChoiceTranslitUntilGraded is the cloze leak: the buttons,
// including the right one, must not show a transliteration before the answer.
// The graded page shows the right one's reading. A full page and an htmx
// swap both do this. The other words in the line still show their reading.
func TestClozeHidesChoiceTranslitUntilGraded(t *testing.T) {
	const (
		ani    = "אֲנִי"
		lomed  = "לוֹמֵד"
		ivrit  = "עִבְרִית"
		kotev  = "כּוֹתֵב"
		okhel  = "אוֹכֵל"
		shoteh = "שׁוֹתֶה"
	)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: ani, Gloss: "I", Translit: sp("TL-ani"), POS: sp("pron")},
		{TextNFC: lomed, Gloss: "studies", Translit: sp("TL-lomed"), POS: sp("verb")},
		{TextNFC: ivrit, Gloss: "Hebrew", Translit: sp("TL-ivrit"), POS: sp("noun")},
		{TextNFC: kotev, Gloss: "writes", Translit: sp("TL-kotev"), POS: sp("verb")},
		{TextNFC: okhel, Gloss: "eats", Translit: sp("TL-okhel"), POS: sp("verb")},
		{TextNFC: shoteh, Gloss: "drinks", Translit: sp("TL-shoteh"), POS: sp("verb")},
	})
	if err != nil {
		t.Fatal(err)
	}
	var lomedID int64
	for _, row := range saved {
		if row.TextNFC == lomed {
			lomedID = row.ID
		}
	}
	if lomedID == 0 {
		t.Fatal("fixture did not store the cloze answer")
	}
	// The other verbs are in a later step, so they can be buttons and are not
	// already on the cloze line. Their readings are the ones that must stay hidden.
	line := ani + " " + lomed + " " + ivrit
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Studies", Situation: "Class.", Position: 1,
		NewWords: []string{lomed},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: ani},
			{Type: units.ActivityCloze, Text: line, Target: lomed},
			{Type: units.ActivityStory, Text: kotev + " " + okhel + " " + shoteh},
		}}},
	}, "shared")
	publish(t, store, unitID)

	ada, err := store.CreateUser(ctx, "ada@example.com", "sub-ada", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	bea, err := store.CreateUser(ctx, "bea@example.com", "sub-bea", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: []string{"ada@example.com", "bea@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	path := "/lesson/" + itoa(unitID) + "/0"
	adaCookie := mintSession(t, store, ada.ID, "ada-csrf", now.Add(time.Hour))
	beaCookie := mintSession(t, store, bea.ID, "bea-csrf", now.Add(time.Hour))

	t.Run("full page", func(t *testing.T) {
		ask, graded := clozePair(t, h, adaCookie, path, "ada-csrf", lomedID, false)
		assertUngradedCloze(t, ask)
		assertGradedCloze(t, graded)
		if !strings.Contains(ask, "<html") || !strings.Contains(graded, "<html") {
			t.Fatal("a plain lesson response is not a full page")
		}
	})
	t.Run("htmx", func(t *testing.T) {
		ask, graded := clozePair(t, h, beaCookie, path, "bea-csrf", lomedID, true)
		assertUngradedCloze(t, ask)
		assertGradedCloze(t, graded)
		if strings.Contains(ask, "<html") || strings.Contains(graded, "<html") {
			t.Fatal("an htmx lesson response included the full page")
		}
	})
}

// clozePair walks story → cloze ask → graded cloze. hx chooses the swap.
func clozePair(t *testing.T, h http.Handler, cookie *http.Cookie, path, csrf string, answerID int64, hx bool) (ask, graded string) {
	t.Helper()
	story := getAuthed(t, h, cookie, path)
	runID := runValue(t, story)
	askRec := postLessonHX(t, h, cookie, path, runID, "continue", csrf, hx)
	if askRec.Code != http.StatusOK {
		t.Fatalf("continue = %d\n%s", askRec.Code, askRec.Body.String())
	}
	ask = askRec.Body.String()
	gradedRec := postLessonHX(t, h, cookie, path, runID, itoa(answerID), csrf, hx)
	if gradedRec.Code != http.StatusOK {
		t.Fatalf("answer = %d\n%s", gradedRec.Code, gradedRec.Body.String())
	}
	return ask, gradedRec.Body.String()
}

func assertUngradedCloze(t *testing.T, body string) {
	t.Helper()
	choices := choiceRegion(body)
	if !strings.Contains(choices, "לוֹמֵד") {
		t.Fatalf("ungraded cloze dropped the right word:\n%s", choices)
	}
	for _, tr := range clozeChoiceTranslits {
		if strings.Contains(body, tr) {
			t.Fatalf("ungraded cloze shows choice translit %s:\n%s", tr, choices)
		}
	}
	if !strings.Contains(body, "TL-ivrit") || !strings.Contains(body, "TL-ani") {
		t.Fatalf("ungraded cloze hid the other words' translit:\n%s", body)
	}
}

func assertGradedCloze(t *testing.T, body string) {
	t.Helper()
	if strings.Contains(body, "___") || !strings.Contains(body, "לוֹמֵד") {
		t.Fatalf("graded cloze did not fill the gap:\n%s", body)
	}
	if !strings.Contains(body, "TL-lomed") {
		t.Fatalf("graded cloze hides the answer translit:\n%s", body)
	}
	if !strings.Contains(body, `class="wb__tr"`) {
		t.Fatalf("graded cloze translit is not a reading:\n%s", body)
	}
}

func postLessonHX(t *testing.T, h http.Handler, cookie *http.Cookie, path, runID, answer, csrf string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("run", runID)
	form.Set("answer", answer)
	form.Set("_csrf", csrf)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Host = "learn.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("X-CSRF-Token", csrf)
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
