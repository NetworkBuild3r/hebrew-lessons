package web

import (
	"cmp"
	"context"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/choices"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// TestAskChoicesFallback is the reviewer's fill order.
// (a) a pronoun, a verb target, and an adverb reach at least two buttons,
// filled from another part of speech or an earlier unit.
// (b) a single-word recall of that verb has more than one button.
// (c) when enough same-part-of-speech words exist, other parts and the
// line's own words stay off the buttons.
// A second word with the answer's gloss is never a button.
func TestAskChoicesFallback(t *testing.T) {
	pron := db.LexemeView{ID: 1, Text: "אֲנִי", Gloss: "I", POS: "pron", Translit: "ani"}
	verb := db.LexemeView{ID: 2, Text: "גָּר", Gloss: "lives", POS: "verb", Translit: "gar"}
	adv := db.LexemeView{ID: 3, Text: "פֹּה", Gloss: "here", POS: "adv", Translit: "po"}
	noun := db.LexemeView{ID: 4, Text: "מַיִם", Gloss: "water", POS: "noun", Translit: "mayim"}
	twin := db.LexemeView{ID: 5, Text: "יוֹשֵׁב", Gloss: "lives", POS: "verb", Translit: "yoshev"}
	other := db.LexemeView{ID: 6, Text: "כּוֹתֵב", Gloss: "writes", POS: "verb", Translit: "kotev"}
	eats := db.LexemeView{ID: 7, Text: "אוֹכֵל", Gloss: "eats", POS: "verb", Translit: "okhel"}
	drinks := db.LexemeView{ID: 8, Text: "שׁוֹתֶה", Gloss: "drinks", POS: "verb", Translit: "shoteh"}
	seed := choices.ChoiceSeed{RunID: 9, Step: 0, TargetID: verb.ID}
	line := choiceExclude{ids: map[int64]bool{pron.ID: true, adv.ID: true}, glosses: map[string]bool{"i": true, "here": true}}

	t.Run("tiny unit", func(t *testing.T) {
		got := askChoices(units.ActivityRecall, []db.LexemeView{pron, verb, adv}, []db.LexemeView{noun}, verb, seed, line)
		if len(got) < 2 {
			t.Fatalf("buttons = %d, want at least 2: %+v", len(got), got)
		}
		if !choiceHas(got, "4") {
			t.Fatalf("earlier other-POS word did not fill before the line: %+v", got)
		}
		if choiceHas(got, "5") {
			t.Fatal("a gloss duplicate of the answer was a button")
		}
	})

	t.Run("single word", func(t *testing.T) {
		got := askChoices(units.ActivityRecall, []db.LexemeView{pron, verb, adv}, nil, verb, seed, choiceExclude{})
		if len(got) < 2 {
			t.Fatalf("single-word buttons = %d: %+v", len(got), got)
		}
		if !choiceHas(got, "1") || !choiceHas(got, "3") {
			t.Fatalf("other parts of speech in the unit were not offered: %+v", got)
		}
	})

	t.Run("same POS preferred", func(t *testing.T) {
		unit := []db.LexemeView{pron, verb, adv, other, twin}
		earlier := []db.LexemeView{eats, drinks, noun}
		got := askChoices(units.ActivityRecall, unit, earlier, verb, seed, line)
		if len(got) != lessonWideChoices {
			t.Fatalf("buttons = %d, want %d: %+v", len(got), lessonWideChoices, got)
		}
		for _, id := range []string{"2", "6", "7", "8"} {
			if !choiceHas(got, id) {
				t.Errorf("missing same-POS %s in %+v", id, got)
			}
		}
		for _, id := range []string{"1", "3", "4", "5"} {
			if choiceHas(got, id) {
				t.Errorf("button %s is the line, another POS, or the answer's gloss", id)
			}
		}
	})

	t.Run("alone is not graded", func(t *testing.T) {
		if got := askChoices(units.ActivityRecall, []db.LexemeView{verb}, nil, verb, seed, choiceExclude{}); got != nil {
			t.Fatalf("a one-word lexicon still asks: %+v", got)
		}
		if got := askChoices(units.ActivityCloze, []db.LexemeView{verb, twin}, nil, verb, seed, choiceExclude{}); got != nil {
			t.Fatalf("a gloss duplicate still counts as a second option: %+v", got)
		}
	})
}

func TestLessonChoicesSkipEmptyEnglishGloss(t *testing.T) {
	target := db.LexemeView{ID: 1, Text: "א", Gloss: "aleph", POS: "noun", Translit: "alef"}
	blank := db.LexemeView{ID: 2, Text: "ב", Gloss: "  ", POS: "noun", Translit: "bet"}
	other := db.LexemeView{ID: 3, Text: "ג", Gloss: "gimel", POS: "noun", Translit: "gimel"}
	verb := db.LexemeView{ID: 4, Text: "ד", Gloss: "dalet", POS: "verb", Translit: "dalet"}
	unit := []db.LexemeView{target, blank, other, verb}
	seed := choices.ChoiceSeed{RunID: 1, Step: 0, Attempt: 0, TargetID: target.ID}

	english := lessonChoices(unit, nil, target, seed, false, false, lessonChoiceButtons, choiceExclude{})
	if choiceHas(english, "2") || !choiceHas(english, "3") || !choiceHas(english, "4") || !choiceHas(english, "1") {
		t.Fatalf("english buttons = %+v", english)
	}
	for _, choice := range english {
		if choice.Hebrew || choice.Text != "" {
			t.Fatalf("english button shows Hebrew: %+v", choice)
		}
	}

	hebrew := lessonChoices(unit, nil, target, seed, true, true, lessonChoiceButtons, choiceExclude{})
	if !choiceHas(hebrew, "2") || !choiceHas(hebrew, "3") || choiceHas(hebrew, "4") {
		t.Fatalf("hebrew buttons = %+v", hebrew)
	}
	for _, choice := range hebrew {
		if !choice.Hebrew {
			t.Fatalf("hebrew button lost its flag: %+v", choice)
		}
		if choice.Value == "1" && (choice.Text != "א" || choice.Translit != "alef") {
			t.Fatalf("cloze target button = %+v", choice)
		}
	}
}

func TestLessonChoiceGradeAndReload(t *testing.T) {
	letters := []string{"א", "ב", "ג", "ד", "ה", "ו", "ז", "ח"}
	fx := openLessonPlayer(t)
	ctx := fx.ctx
	sp := func(s string) *string { return &s }
	inputs := make([]lexicon.Input, len(letters))
	for i, text := range letters {
		inputs[i] = lexicon.Input{
			TextNFC:  text,
			Gloss:    "gloss-" + strconv.Itoa(i),
			Translit: sp("t-" + strconv.Itoa(i)),
			POS:      sp("noun"),
			Gender:   sp("m"),
		}
	}
	if _, _, err := fx.store.UpsertLexemes(ctx, inputs); err != nil {
		t.Fatal(err)
	}
	var recalls []units.Activity
	for _, text := range letters {
		recalls = append(recalls, units.Activity{Type: units.ActivityRecall, Text: text})
	}
	lessonActs := append(append([]units.Activity{}, recalls...),
		units.Activity{Type: units.ActivityListenPick, Text: letters[0]},
		units.Activity{Type: units.ActivityProduce, Text: letters[3]},
		units.Activity{Type: units.ActivityCloze, Text: letters[7]},
	)
	doc := units.Unit{
		Title: "Letters", Position: 1, NewWords: letters,
		Lessons: []units.Lesson{
			{Kind: units.KindLesson, Activities: lessonActs},
			{Kind: units.KindCheckpoint, Activities: []units.Activity{
				{Type: units.ActivityRecall, Text: letters[5]},
			}},
		},
	}
	unitID := putLesson(t, fx.store, nil, doc, "shared")
	publish(t, fx.store, unitID)
	pool, err := fx.store.UnitLexemes(ctx, unitID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pool) < 8 {
		t.Fatalf("unit lexemes = %d", len(pool))
	}
	earlier, err := fx.store.EarlierUnitLexemes(ctx, fx.userID, doc.Position)
	if err != nil {
		t.Fatal(err)
	}
	sorted := slices.Clone(pool)
	slices.SortFunc(sorted, func(a, b db.LexemeView) int {
		return cmp.Or(cmp.Compare(a.Text, b.Text), cmp.Compare(a.ID, b.ID))
	})
	late := map[int64]bool{}
	for _, lex := range sorted[2:] {
		late[lex.ID] = true
	}

	path := "/lesson/" + itoa(unitID) + "/0"
	lateIdx := playChoiceSteps(t, fx, path, lessonActs, pool, earlier, late)
	if len(uniqueInts(lateIdx)) < 2 || allInts(lateIdx, 2) {
		t.Fatalf("recall targets that sort 3rd or later stay on one button: %v", lateIdx)
	}

	check := "/lesson/" + itoa(unitID) + "/1"
	playChoiceSteps(t, fx, check, doc.Lessons[1].Activities, pool, earlier, nil)
}

func TestLessonChoicesUseEarlierLexemes(t *testing.T) {
	fx := openLessonPlayer(t)
	ctx := fx.ctx
	sp := func(s string) *string { return &s }
	saved, _, err := fx.store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: "א", Gloss: "alef", Translit: sp("alef"), POS: sp("noun"), Gender: sp("m")},
		{TextNFC: "ב", Gloss: "bet", Translit: sp("bet"), POS: sp("noun"), Gender: sp("m")},
		{TextNFC: "ג", Gloss: "gimel", Translit: sp("gimel"), POS: sp("verb"), Gender: sp("m")},
		{TextNFC: "ד", Gloss: "dalet", Translit: sp("dalet"), POS: sp("verb"), Gender: sp("m")},
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for _, row := range saved {
		ids[row.TextNFC] = row.ID
	}
	earlierID := putLesson(t, fx.store, nil, units.Unit{
		Title: "Earlier", Position: 0, NewWords: []string{"א", "ב", "ג"},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: "א ב ג"},
		}}},
	}, "shared")
	currentID := putLesson(t, fx.store, nil, units.Unit{
		Title: "Current", Position: 1, NewWords: []string{"ד"},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: "ד"},
		}}},
	}, "shared")
	publish(t, fx.store, earlierID, currentID)
	when := db.FormatTime(fx.now)
	if _, err := fx.store.ExecContext(ctx, `
		INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score)
		VALUES (?, ?, 0, ?, ?, 1)`, fx.userID, earlierID, when, when); err != nil {
		t.Fatal(err)
	}

	path := "/lesson/" + itoa(currentID) + "/0"
	body := getAuthed(t, fx.h, fx.cookie, path)
	again := getAuthed(t, fx.h, fx.cookie, path)
	got := choiceValues(t, body)
	if !slices.Equal(got, choiceValues(t, again)) {
		t.Fatalf("reload order %v then %v", got, choiceValues(t, again))
	}
	pool, err := fx.store.UnitLexemes(ctx, currentID)
	if err != nil {
		t.Fatal(err)
	}
	earlier, err := fx.store.EarlierUnitLexemes(ctx, fx.userID, 1)
	if err != nil {
		t.Fatal(err)
	}
	target := viewByText(t, append(append([]db.LexemeView{}, pool...), earlier...), "ד")
	runID := mustRunID(t, again)
	want := choiceIDs(askChoices(units.ActivityRecall, pool, earlier, target, choices.ChoiceSeed{
		RunID: runID, Step: 0, Attempt: 0, TargetID: target.ID,
	}, choiceExclude{}))
	if !slices.Equal(got, want) {
		t.Fatalf("buttons %v want %v", got, want)
	}
	if len(got) != lessonWideChoices {
		t.Fatalf("buttons = %d, want %d: %v", len(got), lessonWideChoices, got)
	}
	if !slices.Contains(got, itoa(ids["ג"])) {
		t.Fatalf("same part of speech from the earlier unit missing: %v", got)
	}
	// The one earlier verb is preferred. The two nouns fill the remaining
	// buttons so the ask still has four.
	if !slices.Contains(got, itoa(ids["א"])) || !slices.Contains(got, itoa(ids["ב"])) {
		t.Fatalf("other parts of speech did not fill the remaining buttons: %v", got)
	}
	ans := postLesson(t, fx.h, fx.cookie, path, itoa(runID), itoa(ids["ד"]), true, lessonCSRF)
	if ans.Code != http.StatusOK || !strings.Contains(ans.Body.String(), "feedback--right") {
		t.Fatalf("posting the target id: %d %s", ans.Code, ans.Body.String())
	}
}

type lessonFixture struct {
	h      http.Handler
	store  *db.Store
	cookie *http.Cookie
	ctx    context.Context
	userID int64
	now    time.Time
}

const lessonCSRF = "brian-csrf"

func openLessonPlayer(t *testing.T) lessonFixture {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	brian, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
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
	return lessonFixture{
		h:      NewOptions(nil, svc, Options{Store: store}),
		store:  store,
		cookie: mintSession(t, store, brian.ID, lessonCSRF, now.Add(time.Hour)),
		ctx:    ctx,
		userID: brian.ID,
		now:    now,
	}
}

func playChoiceSteps(t *testing.T, fx lessonFixture, path string, acts []units.Activity, pool, earlier []db.LexemeView, late map[int64]bool) []int {
	t.Helper()
	body := getAuthed(t, fx.h, fx.cookie, path)
	var lateIdx []int
	for step, act := range acts {
		again := getAuthed(t, fx.h, fx.cookie, path)
		got := choiceValues(t, body)
		if !slices.Equal(got, choiceValues(t, again)) {
			t.Fatalf("%s step %d reload order %v then %v", path, step, got, choiceValues(t, again))
		}
		runID := mustRunID(t, again)
		target := viewByText(t, pool, act.Text)
		want := choiceIDs(askChoices(act.Type, pool, earlier, target, choices.ChoiceSeed{
			RunID: runID, Step: step, Attempt: 0, TargetID: target.ID,
		}, choiceExclude{}))
		if !slices.Equal(got, want) {
			t.Fatalf("%s step %d %s buttons %v want %v", path, step, act.Type, got, want)
		}
		if repeated(got) {
			t.Fatalf("%s step %d repeats a button: %v", path, step, got)
		}
		id := itoa(target.ID)
		idx := slices.Index(got, id)
		if idx < 0 || countValue(got, id) != 1 {
			t.Fatalf("%s step %d target %s in %v", path, step, id, got)
		}
		if late[target.ID] && act.Type == units.ActivityRecall {
			lateIdx = append(lateIdx, idx)
		}
		ans := postLesson(t, fx.h, fx.cookie, path, itoa(runID), id, true, lessonCSRF)
		if ans.Code != http.StatusOK || !strings.Contains(ans.Body.String(), "feedback--right") {
			t.Fatalf("%s step %d posting id %s at index %d: %d %s", path, step, id, idx, ans.Code, ans.Body.String())
		}
		cont := postLesson(t, fx.h, fx.cookie, path, itoa(runID), "continue", true, lessonCSRF)
		if cont.Code != http.StatusOK {
			t.Fatalf("continue %d %s", cont.Code, cont.Body.String())
		}
		body = cont.Body.String()
	}
	if !strings.Contains(body, "Lesson complete.") {
		t.Fatalf("%s did not finish:\n%s", path, body)
	}
	return lateIdx
}

var choiceValueRE = regexp.MustCompile(`<button name="answer" type="submit" value="([^"]*)" class="c-answer"`)

func choiceValues(t *testing.T, body string) []string {
	t.Helper()
	ms := choiceValueRE.FindAllStringSubmatch(body, -1)
	if len(ms) == 0 {
		t.Fatalf("no choice buttons:\n%s", body)
	}
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m[1]
	}
	return out
}

func choiceIDs(picked []views.Choice) []string {
	out := make([]string, len(picked))
	for i, choice := range picked {
		out[i] = choice.Value
	}
	return out
}

func viewByText(t *testing.T, pool []db.LexemeView, text string) db.LexemeView {
	t.Helper()
	for _, lex := range pool {
		if lex.Text == text {
			return lex
		}
	}
	t.Fatalf("lexeme %s is not in the pool", text)
	return db.LexemeView{}
}

func mustRunID(t *testing.T, body string) int64 {
	t.Helper()
	id, err := strconv.ParseInt(runValue(t, body), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func choiceHas(buttons []views.Choice, value string) bool {
	for _, choice := range buttons {
		if choice.Value == value {
			return true
		}
	}
	return false
}

func repeated(vals []string) bool {
	seen := map[string]bool{}
	for _, v := range vals {
		if seen[v] {
			return true
		}
		seen[v] = true
	}
	return false
}

func countValue(vals []string, want string) int {
	n := 0
	for _, v := range vals {
		if v == want {
			n++
		}
	}
	return n
}

func uniqueInts(vals []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, v := range vals {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func allInts(vals []int, want int) bool {
	if len(vals) == 0 {
		return false
	}
	for _, v := range vals {
		if v != want {
			return false
		}
	}
	return true
}
