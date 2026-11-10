package web

import (
	"regexp"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

const (
	lsHello   = "שָׁלוֹם"
	lsMorning = "בֹּקֶר"
	lsNight   = "לַיְלָה"
	lsEvening = "עֶרֶב"
	lsThanks  = "תּוֹדָה"
)

// heardButton is one answer button of a listening step: its value and the Hebrew it shows.
type heardButton struct{ value, line string }

var heardButtonRE = regexp.MustCompile(`<button name="answer" type="submit" value="(\d+)" class="c-answer".*?<bdi lang="he" dir="rtl">([^<]*)</bdi>`)

func heardButtons(t *testing.T, body string) []heardButton {
	t.Helper()
	var out []heardButton
	for _, m := range heardButtonRE.FindAllStringSubmatch(body, -1) {
		out = append(out, heardButton{value: m[1], line: m[2]})
	}
	if len(out) == 0 {
		t.Fatalf("no Hebrew answer buttons:\n%s", body)
	}
	return out
}

// listenFixture is one lesson whose first step is a listening question on "hello morning".
func listenFixture(t *testing.T, words ...string) (lessonFixture, int64, map[string]int64, string) {
	t.Helper()
	return listenFixtureAt(t, "https://learn.example", words...)
}

// listenFixtureAt is listenFixture for a given public origin.
func listenFixtureAt(t *testing.T, origin string, words ...string) (lessonFixture, int64, map[string]int64, string) {
	t.Helper()
	fx := openLessonPlayerAt(t, origin)
	sp := func(s string) *string { return &s }
	inputs := make([]lexicon.Input, len(words))
	for i, w := range words {
		inputs[i] = lexicon.Input{TextNFC: w, Gloss: []string{"hello", "morning", "night", "evening", "thanks", "water", "house", "book"}[i], Translit: sp("t" + itoa(int64(i))), POS: sp("noun"), Gender: sp("m")}
	}
	saved, _, err := fx.store.UpsertLexemes(fx.ctx, inputs)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for _, row := range saved {
		ids[row.TextNFC] = row.ID
	}
	line := lsHello + " " + lsMorning
	acts := []units.Activity{
		{Type: units.ActivityListenPick, Text: line},
		{Type: units.ActivityRecall, Text: lsHello},
		// A recall of a line names one word and lays the line out as cards.
		{Type: units.ActivityRecall, Text: line, Target: lsMorning},
	}
	// Every word of a real unit is used in some activity, which is what puts it in the pool.
	for _, w := range words[min(2, len(words)):] {
		acts = append(acts, units.Activity{Type: units.ActivityRecall, Text: w})
	}
	unitID := putLesson(t, fx.store, nil, units.Unit{
		Title: "Greetings", Position: 1, NewWords: words,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: acts}},
	}, "shared")
	publish(t, fx.store, unitID)
	clip := strings.Repeat("ab", 32)
	word := strings.Repeat("cd", 32)
	for _, asset := range []db.Asset{
		{ID: clip, TextNFC: units.Normalize(line), Voice: "voice", Model: "eleven_v4", Bytes: 4},
		{ID: word, TextNFC: lsHello, Voice: "voice", Model: "eleven_v4", Bytes: 4},
	} {
		if _, err := fx.store.InsertAudio(fx.ctx, asset); err != nil {
			t.Fatal(err)
		}
	}
	return fx, unitID, ids, line
}

// A multi-word listen_pick is an ungraded reveal: the line is shown, and
// posting a lexeme id writes no attempt. INIT-038/SPEC-008.
func TestMultiWordListenPickIsReveal(t *testing.T) {
	fx, unitID, ids, _ := listenFixture(t, lsHello, lsMorning, lsNight, lsEvening, lsThanks)
	path := "/lesson/" + itoa(unitID) + "/0"
	ask := activityHTML(t, getAuthed(t, fx.h, fx.cookie, path))
	if strings.Contains(ask, `class="choices"`) {
		t.Fatalf("multi-word listen_pick rendered buttons:\n%s", ask)
	}
	if !strings.Contains(ask, "This is what you heard.") {
		t.Fatalf("prompt:\n%s", ask)
	}
	if !strings.Contains(ask, lsHello) || !strings.Contains(ask, lsMorning) {
		t.Fatalf("the line is hidden:\n%s", ask)
	}
	runID := runValue(t, ask)
	posted := postLesson(t, fx.h, fx.cookie, path, runID, itoa(ids[lsHello]), true, lessonCSRF)
	if strings.Contains(posted.Body.String(), "feedback--") {
		t.Fatalf("posting an id graded the reveal:\n%s", posted.Body.String())
	}
	rows, err := fx.store.ListAttempts(fx.ctx, fx.userID, ids[lsHello])
	if err != nil || len(rows) != 0 {
		t.Fatalf("attempts = %+v, %v", rows, err)
	}
	recall := activityHTML(t, postLesson(t, fx.h, fx.cookie, path, runID, "continue", true, lessonCSRF).Body.String())
	if !strings.Contains(recall, lsHello) || !strings.Contains(recall, `class="choices"`) {
		t.Fatalf("continue did not reach the recall:\n%s", recall)
	}
}

// Two lexicon words are still a reveal, even when the unit cannot fill a
// meaning question. INIT-038/SPEC-008.
func TestTwoWordListenPickDoesNotAskMeaning(t *testing.T) {
	fx, unitID, _, _ := listenFixture(t, lsHello, lsMorning)
	ask := activityHTML(t, getAuthed(t, fx.h, fx.cookie, "/lesson/"+itoa(unitID)+"/0"))
	if strings.Contains(ask, `class="choices"`) || strings.Contains(ask, "Listen, then pick the meaning.") {
		t.Fatalf("two-word listen_pick is still a question:\n%s", ask)
	}
	if !strings.Contains(ask, lsHello) || !strings.Contains(ask, lsMorning) || !strings.Contains(ask, "This is what you heard.") {
		t.Fatalf("two-word listen_pick hid the line:\n%s", ask)
	}
}

func TestReplaceToken(t *testing.T) {
	cases := []struct{ name, text, from, to, want string }{
		{"whole word", lsHello + " " + lsMorning, lsHello, lsNight, lsNight + " " + lsMorning},
		{"last word", lsHello + " " + lsMorning, lsMorning, lsNight, lsHello + " " + lsNight},
		{"only the first", lsHello + " " + lsHello, lsHello, lsNight, lsNight + " " + lsHello},
		{"not found", lsHello, lsMorning, lsNight, lsHello},
		{"inside a longer word", "ב" + lsHello, lsHello, lsNight, "ב" + lsHello},
		{"after a longer word", "ב" + lsHello + " " + lsHello, lsHello, lsNight, "ב" + lsHello + " " + lsNight},
		{"empty from", lsHello, "", lsNight, lsHello},
	}
	for _, c := range cases {
		if got := replaceToken(c.text, c.from, c.to); got != c.want {
			t.Errorf("%s: replaceToken(%q, %q, %q) = %q, want %q", c.name, c.text, c.from, c.to, got, c.want)
		}
	}
}

func TestLearnerGlossDropsTheHebrewNote(t *testing.T) {
	cases := map[string]string{
		"work(s) (m.sg. present of לַעֲבוֹד)":                "work(s) (m.sg. present)",
		"study, learn (m.sg. present of לִלְמוֹד)":           "study, learn (m.sg. present)",
		"live, reside (I/he lives; m.sg. present of לָגוּר)": "live, reside (I/he lives; m.sg. present)",
		"hello": "hello",
		"":      "",
	}
	for in, want := range cases {
		if got := learnerGloss(in); got != want {
			t.Errorf("learnerGloss(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHeardChoicesDropsLinesThatDoNotDiffer(t *testing.T) {
	target := db.LexemeView{ID: 1, Text: lsHello}
	buttons := []views.Choice{
		{Value: "1", Text: lsHello, Hebrew: true},
		{Value: "2", Text: lsNight, Hebrew: true},
		{Value: "3", Text: lsNight, Hebrew: true}, // the same line again
		{Value: "4", Text: lsHello, Hebrew: true}, // a swap that changes nothing
	}
	got := heardChoices(buttons, lsHello+" "+lsMorning, target)
	if len(got) != 2 || got[0].Text != lsHello+" "+lsMorning || got[1].Text != lsNight+" "+lsMorning {
		t.Fatalf("heardChoices = %+v", got)
	}
	for _, c := range got {
		if !c.Sentence || c.Translit != "" {
			t.Fatalf("a line button is not marked as a sentence: %+v", c)
		}
	}
	// A single word is its own line, and the wrong buttons are other words.
	one := heardChoices([]views.Choice{{Value: "1", Text: lsHello}, {Value: "2", Text: lsNight}}, lsHello, target)
	if len(one) != 2 || one[0].Text != lsHello || one[1].Text != lsNight {
		t.Fatalf("single word: %+v", one)
	}
}
