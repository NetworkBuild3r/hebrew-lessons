package web

import (
	"regexp"
	"slices"
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
		inputs[i] = lexicon.Input{TextNFC: w, Gloss: "gloss-" + itoa(int64(i)), Translit: sp("t" + itoa(int64(i))), POS: sp("noun"), Gender: sp("m")}
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

// The listening question asks what was heard, and its answers are Hebrew lines: the line
// that played, and the same line with the practised word swapped for another.
func TestListenPickOffersHebrewLines(t *testing.T) {
	fx, unitID, ids, line := listenFixture(t, lsHello, lsMorning, lsNight, lsEvening, lsThanks)
	path := "/lesson/" + itoa(unitID) + "/0"
	body := getAuthed(t, fx.h, fx.cookie, path)
	ask := activityHTML(t, body)

	if !strings.Contains(ask, "What did you hear?") || strings.Contains(ask, "pick the meaning") {
		t.Fatalf("the prompt does not ask what was heard:\n%s", ask)
	}
	buttons := heardButtons(t, ask)
	if len(buttons) != 3 {
		t.Fatalf("buttons = %+v, want the heard line and two wrong lines", buttons)
	}

	heard := 0
	shown := map[string]bool{}
	for _, b := range buttons {
		if shown[b.line] {
			t.Fatalf("a line is offered twice: %q", b.line)
		}
		shown[b.line] = true
		if b.line == line {
			heard++
			if b.value != itoa(ids[lsHello]) {
				t.Fatalf("the heard line posts %s, want the target %d", b.value, ids[lsHello])
			}
			continue
		}
		// A wrong line keeps the rest of the line and changes only the practised word.
		if !strings.HasSuffix(b.line, " "+lsMorning) || strings.Contains(b.line, lsHello) || strings.Count(b.line, lsMorning) != 1 {
			t.Fatalf("wrong line %q is not the heard line with one word swapped", b.line)
		}
	}
	if heard != 1 {
		t.Fatalf("%d buttons are the heard line, want exactly 1: %+v", heard, buttons)
	}

	// The Hebrew is the label only. Nothing in an attribute names it, so the answer is not in the markup.
	if containsHebrew(attrValues(ask)) {
		t.Fatalf("an attribute carries Hebrew:\n%s", attrValues(ask))
	}
	// The line is not shown anywhere else before the answer.
	if strings.Count(ask, line) != 1 {
		t.Fatalf("the heard line shows %d times before the answer, want once (its button)", strings.Count(ask, line))
	}
	for _, want := range []string{`class="listen-stage"`, `id="line-clip"`, `class="orb orb--hero"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("the listening step is missing %s:\n%s", want, body)
		}
	}
	if strings.Contains(ask, "spotlight") {
		t.Fatal("the answer layout shows before the answer")
	}

	// The same buttons come back on a reload.
	again := heardButtons(t, activityHTML(t, getAuthed(t, fx.h, fx.cookie, path)))
	if !slices.Equal(buttons, again) {
		t.Fatalf("reload changed the buttons: %+v then %+v", buttons, again)
	}
}

// A wrong line is graded wrong, and records the word it was taken for. The right line is
// graded right and shows the words big and centered, with the play control below.
func TestListenPickGradesAndSpotlightsTheAnswer(t *testing.T) {
	fx, unitID, ids, line := listenFixture(t, lsHello, lsMorning, lsNight, lsEvening, lsThanks)
	path := "/lesson/" + itoa(unitID) + "/0"
	ask := activityHTML(t, getAuthed(t, fx.h, fx.cookie, path))
	runID := runValue(t, ask)
	var wrong heardButton
	for _, b := range heardButtons(t, ask) {
		if b.line != line {
			wrong = b
			break
		}
	}

	missed := activityHTML(t, postLesson(t, fx.h, fx.cookie, path, runID, wrong.value, true, lessonCSRF).Body.String())
	if !strings.Contains(missed, "feedback--wrong") {
		t.Fatalf("a wrong line was not graded wrong:\n%s", missed)
	}
	rows, err := fx.store.ListAttempts(fx.ctx, fx.userID, ids[lsHello])
	if err != nil || len(rows) != 1 || rows[0].Correct || rows[0].Activity != "listen_pick" {
		t.Fatalf("attempts = %+v, %v", rows, err)
	}
	if strings.Contains(missed, `class="choices"`) {
		t.Fatal("the answers are still offered after the step was answered")
	}
	_, words, ok := strings.Cut(missed, `class="spotlight spotlight--cards"`)
	if !ok || !strings.Contains(words, "wcard") {
		t.Fatalf("the answered step has no spotlight:\n%s", missed)
	}
	for _, want := range []string{lsHello, lsMorning} {
		if !strings.Contains(words, want) {
			t.Fatalf("the spotlight does not show %s:\n%s", want, words)
		}
	}
	// The play control is below the words, not above them.
	if strings.Index(words, `id="line-clip"`) < strings.Index(words, "word-blocks--spot") {
		t.Fatalf("the play control is above the words:\n%s", words)
	}
	if strings.Count(missed, `id="line-clip"`) != 1 {
		t.Fatalf("the step has %d line clips, want 1", strings.Count(missed, `id="line-clip"`))
	}

	// The next step is a recall: its Hebrew word is big and centered with the play control below.
	recall := activityHTML(t, postLesson(t, fx.h, fx.cookie, path, runID, "continue", true, lessonCSRF).Body.String())
	_, words, ok = strings.Cut(recall, `class="spotlight spotlight--cards"`)
	if !ok || !strings.Contains(words, lsHello) || !strings.Contains(words, "wcard") {
		t.Fatalf("the recall prompt has no spotlight on its word:\n%s", recall)
	}
	if strings.Index(words, `id="line-clip"`) < strings.Index(words, lsHello) {
		t.Fatalf("the recall play control is above the word:\n%s", words)
	}
}

// With too few other words to make wrong lines, the step asks for the meaning as before.
func TestListenPickFallsBackToMeaning(t *testing.T) {
	fx, unitID, ids, _ := listenFixture(t, lsHello, lsMorning)
	ask := activityHTML(t, getAuthed(t, fx.h, fx.cookie, "/lesson/"+itoa(unitID)+"/0"))
	if !strings.Contains(ask, "Listen, then pick the meaning.") || strings.Contains(ask, "What did you hear?") {
		t.Fatalf("the fallback prompt is wrong:\n%s", ask)
	}
	if containsHebrew(ask) {
		t.Fatalf("the meaning question shows Hebrew before the answer:\n%s", ask)
	}
	if !strings.Contains(ask, `value="`+itoa(ids[lsHello])+`"`) || !strings.Contains(ask, `value="`+itoa(ids[lsMorning])+`"`) {
		t.Fatalf("the meaning question lost its buttons:\n%s", ask)
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
