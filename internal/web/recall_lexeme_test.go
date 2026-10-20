package web

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/choices"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Pointed forms already used by the lesson and lexicon tests. Glosses are
// single words on purpose: a recall choice that shows a phrase or a sentence
// is a failure even when the button value is some lexeme id.
const (
	rwShalom  = "שָׁלוֹם"
	rwBoker   = "בֹּקֶר"
	rwSefer   = "סֵפֶר"
	rwAni     = "אֲנִי"
	rwGar     = "גָּר"
	rwPo      = "פֹּה"
	rwKen     = "כֵּן"
	rwLo      = "לֹא"
	rwMayim   = "מַיִם"
	rwBayit   = "בַּיִת"
	rwGam     = "גַּם"
	rwRak     = "רַק"
	rwAval    = "אֲבָל"
	rwOkhel   = "אוֹכֵל"
	rwShote   = "שׁוֹתֶה"
	rwRoeh    = "רוֹאֶה"
	rwEifo    = "אֵיפֹה"
	rwAkhshav = "עַכְשָׁיו"
	rwSham    = "שָׁם"
)

// v3OneWordPrompt is the ask sentence a recall of one word has rendered
// since before the multi-word prompt. The multi-word question names the
// word; this one must not.
const v3OneWordPrompt = `<p class="lede" lang="en"><bdi lang="en" dir="ltr">What does this Hebrew mean?</bdi></p>`

type recallWord struct {
	text, gloss, translit, pos, gender string
}

func (w recallWord) input() lexicon.Input {
	sp := func(s string) *string { return &s }
	in := lexicon.Input{TextNFC: units.Normalize(w.text), Gloss: w.gloss, Translit: sp(w.translit), POS: sp(w.pos)}
	if w.gender != "" {
		in.Gender = sp(w.gender)
	}
	return in
}

func recallLexicon() []recallWord {
	return []recallWord{
		{rwShalom, "hello", "shalom", "noun", "m"},
		{rwBoker, "morning", "boker", "noun", "m"},
		{rwSefer, "book", "sefer", "noun", "m"},
		{rwAni, "I", "ani", "pron", ""},
		{rwGar, "lives", "gar", "verb", ""},
		{rwPo, "here", "po", "adv", ""},
		{rwKen, "yes", "ken", "particle", ""},
		{rwLo, "no", "lo", "particle", ""},
		{rwMayim, "water", "mayim", "noun", "m"},
		{rwBayit, "house", "bayit", "noun", "m"},
		{rwGam, "also", "gam", "particle", ""},
		{rwRak, "only", "rak", "particle", ""},
		{rwAval, "but", "aval", "particle", ""},
		{rwOkhel, "eats", "okhel", "verb", ""},
		{rwShote, "drinks", "shote", "verb", ""},
		{rwRoeh, "sees", "roeh", "verb", ""},
		{rwEifo, "where", "eifo", "adv", ""},
		{rwAkhshav, "now", "akhshav", "adv", ""},
		{rwSham, "there", "sham", "adv", ""},
	}
}

// recallStep is one recall activity. target is the Hebrew the player must
// ask about: the only word, or the first new word of a line, or the first
// lexeme when the line has no new word.
type recallStep struct {
	line   string
	target string
	multi  bool
}

type recallUnit struct {
	title    string
	position int
	newWords []string
	steps    []recallStep
}

func recallCourse() []recallUnit {
	return []recallUnit{
		{
			title: "One word", position: 0,
			// At least four words of each part of speech, so a recall can draw
			// three same-POS distractors from this unit. Later units draw
			// those words once the line's own glosses are blocked.
			newWords: []string{
				rwShalom, rwBoker, rwSefer, rwKen,
				rwMayim, rwBayit, rwGam, rwRak, rwAval,
				rwOkhel, rwShote, rwRoeh, rwGar,
				rwEifo, rwAkhshav, rwSham, rwPo, rwLo,
			},
			steps: []recallStep{
				{line: rwShalom, target: rwShalom},
				{line: rwBoker, target: rwBoker},
				{line: rwSefer, target: rwSefer},
				{line: rwKen, target: rwKen},
				{line: rwMayim, target: rwMayim},
				{line: rwBayit, target: rwBayit},
				{line: rwGam, target: rwGam},
				{line: rwRak, target: rwRak},
				{line: rwAval, target: rwAval},
				{line: rwOkhel, target: rwOkhel},
				{line: rwShote, target: rwShote},
				{line: rwRoeh, target: rwRoeh},
				{line: rwGar, target: rwGar},
				{line: rwEifo, target: rwEifo},
				{line: rwAkhshav, target: rwAkhshav},
				{line: rwSham, target: rwSham},
				{line: rwPo, target: rwPo},
				{line: rwLo, target: rwLo},
			},
		},
		{
			title: "A line", position: 1,
			newWords: []string{rwGar, rwLo},
			steps: []recallStep{
				{line: rwAni + " " + rwGar + " " + rwPo, target: rwGar, multi: true},
				{line: rwKen + " " + rwLo + " " + rwSefer, target: rwLo, multi: true},
				{line: rwShalom + " " + rwBoker, target: rwShalom, multi: true},
				{line: rwGar, target: rwGar},
			},
		},
		{
			title: "Last word", position: 2,
			newWords: []string{rwPo},
			steps: []recallStep{
				{line: rwAni + " " + rwPo, target: rwPo, multi: true},
				{line: rwLo, target: rwLo},
			},
		},
	}
}

type recallFixture struct {
	h     http.Handler
	store *db.Store
	lex   map[string]db.LexemeView
	units []struct {
		id       int64
		position int
		spec     recallUnit
	}
}

func openRecallFixture(t *testing.T, emails []string) recallFixture {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	var inputs []lexicon.Input
	for _, w := range recallLexicon() {
		inputs = append(inputs, w.input())
	}
	if _, _, err := store.UpsertLexemes(ctx, inputs); err != nil {
		t.Fatal(err)
	}
	texts := make([]string, 0, len(inputs))
	for _, in := range inputs {
		texts = append(texts, units.Normalize(in.TextNFC))
	}
	lex, err := store.LexemesByText(ctx, texts)
	if err != nil {
		t.Fatal(err)
	}
	fx := recallFixture{store: store, lex: lex}
	for _, unit := range recallCourse() {
		acts := make([]units.Activity, len(unit.steps))
		for i, step := range unit.steps {
			acts[i] = units.Activity{Type: units.ActivityRecall, Text: step.line}
		}
		id := putLesson(t, store, nil, units.Unit{
			Title: unit.title, Situation: "Recall.", Position: unit.position,
			NewWords: unit.newWords,
			Lessons:  []units.Lesson{{Kind: units.KindLesson, Activities: acts}},
		}, "shared")
		fx.units = append(fx.units, struct {
			id       int64
			position int
			spec     recallUnit
		}{id, unit.position, unit})
	}
	ids := make([]int64, len(fx.units))
	for i, u := range fx.units {
		ids[i] = u.id
	}
	publish(t, store, ids...)
	for _, email := range emails {
		if _, err := store.CreateUser(ctx, email, "sub-"+email, auth.RoleLearner); err != nil {
			t.Fatal(err)
		}
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: fixedClock(now), Store: store,
		AllowedEmails: emails,
	})
	if err != nil {
		t.Fatal(err)
	}
	fx.h = NewOptions(nil, svc, Options{Store: store})
	return fx
}

func recallSession(t *testing.T, fx recallFixture, email, csrf string) *http.Cookie {
	t.Helper()
	ctx := context.Background()
	user, err := fx.store.UserByEmail(ctx, email)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	return mintSession(t, fx.store, user.ID, csrf, now.Add(time.Hour))
}

// TestRecallChoicesStayOnTheTargetLexeme walks every recall step of several
// seeded units, once per learner. Each learner is a different lesson run, so
// choices.Pick sees a different seed. The marked word (or the one bdi, for a
// single word) and the correct button are that word's lexeme. Every button
// is some word's gloss, never a phrase or a sentence. The target's
// transliteration stays hidden until the answer, and the dashed slot then
// fills with the gloss.
func TestRecallChoicesStayOnTheTargetLexeme(t *testing.T) {
	const learners = 8
	emails := make([]string, learners)
	for i := range emails {
		emails[i] = fmt.Sprintf("recall%d@example.com", i)
	}
	fx := openRecallFixture(t, emails)
	// orderKey -> choice-id sequence -> how many learners produced it.
	orders := map[string]map[string]int{}
	for i, email := range emails {
		cookie := recallSession(t, fx, email, fmt.Sprintf("csrf-%d", i))
		user, err := fx.store.UserByEmail(context.Background(), email)
		if err != nil {
			t.Fatal(err)
		}
		for _, unit := range fx.units {
			walkRecallUnit(t, fx, cookie, fmt.Sprintf("csrf-%d", i), user.ID, unit.id, unit.position, unit.spec, orders)
		}
	}
	varied := 0
	for key, set := range orders {
		if len(set) < 2 {
			continue
		}
		varied++
		var sample []string
		for order := range set {
			sample = append(sample, order)
			if len(sample) == 2 {
				break
			}
		}
		t.Logf("%s varied across seeds: %s", key, strings.Join(sample, " | "))
	}
	if varied == 0 {
		t.Fatalf("choices.Pick did not vary across %d run ids: %+v", learners, orders)
	}
	t.Logf("%d of %d recall steps showed more than one button order", varied, len(orders))
}

func walkRecallUnit(t *testing.T, fx recallFixture, cookie *http.Cookie, csrf string, userID, unitID int64, position int, spec recallUnit, orders map[string]map[string]int) {
	t.Helper()
	path := "/lesson/" + itoa(unitID) + "/0"
	body := getAuthed(t, fx.h, cookie, path)
	runID := runValue(t, body)
	runN, err := strconv.ParseInt(runID, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	unitLex, err := fx.store.UnitLexemes(ctx, unitID)
	if err != nil {
		t.Fatal(err)
	}
	earlier, err := fx.store.EarlierUnitLexemes(ctx, userID, position)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[int64]db.LexemeView{}
	for _, view := range fx.lex {
		byID[view.ID] = view
	}
	for step, specStep := range spec.steps {
		target, ok := fx.lex[units.Normalize(specStep.target)]
		if !ok {
			t.Fatalf("unit %d step %d: target %q is not in the lexicon", position, step, specStep.target)
		}
		ask := activityHTML(t, body)
		checkRecallAsk(t, ask, specStep, target, byID)
		seed := choices.ChoiceSeed{RunID: runN, Step: step, Attempt: 0, TargetID: target.ID}
		act := units.Activity{Type: units.ActivityRecall, Text: specStep.line}
		want := askChoices(act.Type, unitLex, earlier, target, seed, lineMateExclude(act, fx.lex, target))
		got := parseRecallChoices(t, ask)
		if len(got) != len(want) || len(got) != lessonWideChoices {
			t.Fatalf("unit %d step %d seed %+v: %d buttons, askChoices returned %d (recall asks for %d)\n%s", position, step, seed, len(got), len(want), lessonWideChoices, ask)
		}
		ids := make([]string, len(got))
		for i := range got {
			if got[i].id != want[i].Value || got[i].label != want[i].Label {
				t.Fatalf("unit %d step %d seed %+v: button %d = %s %q, Pick = %s %q", position, step, seed, i, got[i].id, got[i].label, want[i].Value, want[i].Label)
			}
			ids[i] = got[i].id
		}
		key := fmt.Sprintf("%s/step%d", spec.title, step)
		if orders[key] == nil {
			orders[key] = map[string]int{}
		}
		orders[key][strings.Join(ids, ",")]++

		revealed := activityHTML(t, postLesson(t, fx.h, cookie, path, runID, itoa(target.ID), true, csrf).Body.String())
		checkRecallReveal(t, revealed, specStep, target)
		next := postLesson(t, fx.h, cookie, path, runID, "continue", true, csrf)
		if next.Code != http.StatusOK {
			t.Fatalf("continue unit %d step %d = %d\n%s", position, step, next.Code, next.Body.String())
		}
		body = next.Body.String()
	}
	if !strings.Contains(body, "Lesson complete.") {
		t.Fatalf("unit %q did not finish:\n%s", spec.title, body)
	}
}

type recallChoice struct {
	id, label string
}

func parseRecallChoices(t *testing.T, ask string) []recallChoice {
	t.Helper()
	_, choicesHTML, ok := strings.Cut(ask, `class="choices"`)
	if !ok {
		t.Fatalf("recall has no choices:\n%s", ask)
	}
	var out []recallChoice
	for _, m := range regexp.MustCompile(`(?s)<button\b([^>]*)>(.*?)</button>`).FindAllStringSubmatch(choicesHTML, -1) {
		open, inner := m[1], m[2]
		if !strings.Contains(open, `class="c-answer"`) && !strings.Contains(m[0], `class="c-answer"`) {
			continue
		}
		// class is written after the attributes, so it may sit past the first '>'.
		// Match the whole button instead.
		full := m[0]
		if !strings.Contains(full, `class="c-answer"`) {
			continue
		}
		vm := regexp.MustCompile(`\bvalue="([^"]*)"`).FindStringSubmatch(open)
		if vm == nil {
			t.Fatalf("choice button has no value:\n%s", full)
		}
		lm := regexp.MustCompile(`(?s)<span class="c-answer__label">(.*?)</span>`).FindStringSubmatch(inner)
		if lm == nil {
			t.Fatalf("choice button has no label:\n%s", full)
		}
		label := strings.TrimSpace(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(lm[1], ""))
		out = append(out, recallChoice{id: vm[1], label: label})
	}
	return out
}

func checkRecallAsk(t *testing.T, ask string, step recallStep, target db.LexemeView, byID map[int64]db.LexemeView) {
	t.Helper()
	head, choicesHTML, ok := strings.Cut(ask, `class="choices"`)
	if !ok {
		t.Fatalf("recall has no choices:\n%s", ask)
	}
	if step.multi {
		prompt := `<bdi lang="en" dir="ltr">What does </bdi><bdi class="recall-prompt" lang="he" dir="rtl">` + target.Text + `</bdi><bdi lang="en" dir="ltr"> mean?</bdi>`
		if !strings.Contains(head, prompt) || strings.Contains(ask, "What does this Hebrew mean?") {
			t.Fatalf("multi-word prompt is not the target word %q:\n%s", target.Text, head)
		}
		card := elementFrom(head, `<span class="wb wb--card"><span class="recall-he"><mark class="target">`)
		if card == "" || strings.Count(head, `class="target"`) != 1 {
			t.Fatalf("multi-word recall does not mark exactly one target:\n%s", head)
		}
		word := markedWord(card)
		if word != target.Text {
			t.Fatalf("marked word = %q, want %q\n%s", word, target.Text, card)
		}
		if strings.Contains(card, `class="wb__tr"`) || strings.Contains(card, ">"+target.Translit+"<") || strings.Contains(card, `target-slot--filled`) || strings.Contains(card, ">"+target.Gloss+"<") {
			t.Fatalf("target transliteration or gloss is visible before the answer:\n%s", card)
		}
		if !strings.Contains(card, `class="target-slot"`) || !strings.Contains(card, ">?</span>") {
			t.Fatalf("the target slot is not the empty dash:\n%s", card)
		}
	} else {
		if !strings.Contains(head, v3OneWordPrompt) || strings.Contains(head, `class="target"`) || strings.Contains(head, "target-slot") || strings.Contains(head, `lang="he" dir="rtl">`+target.Text+`</bdi><bdi lang="en"`) {
			t.Fatalf("one-word recall changed its prompt or grew a mark:\n%s", head)
		}
		block := v3AskWordBlock(target.Text)
		if !strings.Contains(head, block) {
			t.Fatalf("one-word recall is not the v3 word block %s:\n%s", block, head)
		}
		if strings.Contains(head, `class="wb__tr"`) || strings.Contains(head, ">"+target.Translit+"<") {
			t.Fatalf("one-word transliteration is visible before the answer:\n%s", head)
		}
	}
	got := parseRecallChoices(t, ask)
	var correct string
	for _, c := range got {
		id, err := strconv.ParseInt(c.id, 10, 64)
		if err != nil {
			t.Fatalf("choice value %q is not a lexeme id", c.id)
		}
		view, ok := byID[id]
		if !ok {
			t.Fatalf("choice %s is not a seeded lexeme", c.id)
		}
		if c.label != view.Gloss {
			t.Fatalf("choice %s label %q, want the word gloss %q", c.id, c.label, view.Gloss)
		}
		if strings.Contains(c.label, " ") || strings.Contains(c.label, "\u00a0") {
			t.Fatalf("choice %q is a phrase gloss, not a word gloss", c.label)
		}
		if id == target.ID {
			correct = c.label
		}
	}
	if correct != target.Gloss {
		t.Fatalf("no button is %s (%q); choices:\n%s", itoa(target.ID), target.Gloss, choicesHTML)
	}
	for _, phrase := range []string{"I lives here", "I live here", "hello morning", "yes no book", "I here"} {
		if strings.Contains(choicesHTML, phrase) {
			t.Fatalf("choices include the sentence gloss %q:\n%s", phrase, choicesHTML)
		}
	}
}

func checkRecallReveal(t *testing.T, revealed string, step recallStep, target db.LexemeView) {
	t.Helper()
	if step.multi {
		card := elementFrom(revealed, `<span class="wb wb--card"><span class="recall-he"><mark class="target">`)
		if card == "" {
			t.Fatalf("the target mark is gone after the answer:\n%s", revealed)
		}
		if !strings.Contains(card, `target-slot--filled`) || !strings.Contains(card, ">"+target.Gloss+"<") {
			t.Fatalf("the slot did not fill with %q:\n%s", target.Gloss, card)
		}
		if !strings.Contains(card, `class="wb__tr"`) || !strings.Contains(card, ">"+target.Translit+"<") {
			t.Fatalf("the target transliteration did not appear after the answer:\n%s", card)
		}
		return
	}
	block := v3RevealWordBlock(target.Text, target.Translit, target.Gloss)
	if !strings.Contains(revealed, block) || strings.Contains(revealed, `class="target"`) || strings.Contains(revealed, "target-slot") {
		t.Fatalf("one-word reveal is not the v3 word block %s:\n%s", block, revealed)
	}
}

func v3AskWordBlock(text string) string {
	return `<span class="wb wb--card"><bdi class="wb__he" lang="he" dir="rtl">` + text + `</bdi> </span>`
}

func v3RevealWordBlock(text, translit, gloss string) string {
	return `<span class="wb wb--card"><bdi class="wb__he" lang="he" dir="rtl">` + text + `</bdi> <bdi class="wb__tr" lang="he-Latn" dir="ltr">` + translit + `</bdi> <bdi class="wb__gl" lang="en" dir="ltr">` + gloss + `</bdi></span>`
}

func markedWord(card string) string {
	const open = `<bdi class="wb__he" lang="he" dir="rtl">`
	i := strings.Index(card, open)
	if i < 0 {
		return ""
	}
	rest := card[i+len(open):]
	word, _, _ := strings.Cut(rest, "</bdi>")
	return word
}

// elementFrom returns the element that starts at the first occurrence of
// start, counting nested spans so an inner slot does not end the card.
func elementFrom(html, start string) string {
	i := strings.Index(html, start)
	if i < 0 {
		return ""
	}
	// start is inside the opening span. Back up to that span.
	open := strings.LastIndex(html[:i], "<span")
	if open < 0 {
		open = i
	}
	depth := 0
	for j := open; j < len(html); j++ {
		switch {
		case strings.HasPrefix(html[j:], "<span"):
			depth++
		case strings.HasPrefix(html[j:], "</span>"):
			depth--
			if depth == 0 {
				return html[open : j+len("</span>")]
			}
		}
	}
	return ""
}

// TestOneWordRecallPromptUnchangedFromV3 is the one-word regression. A recall
// of a single lexeme keeps the v3 question and the v3 word block: Hebrew
// only, no mark, no dashed slot, no word named in the question. The answer
// fills that same word block with the transliteration and the gloss.
func TestOneWordRecallPromptUnchangedFromV3(t *testing.T) {
	fx := openRecallFixture(t, []string{"v3recall@example.com"})
	cookie := recallSession(t, fx, "v3recall@example.com", "csrf-v3")
	// The first published unit is the one-word recalls. Its first step is shalom.
	unitID := fx.units[0].id
	target := fx.lex[units.Normalize(rwShalom)]
	body := getAuthed(t, fx.h, cookie, "/lesson/"+itoa(unitID)+"/0")
	ask := activityHTML(t, body)
	head, _, ok := strings.Cut(ask, `class="choices"`)
	if !ok {
		t.Fatalf("one-word recall has no choices:\n%s", ask)
	}
	if !strings.Contains(ask, v3OneWordPrompt) {
		t.Fatalf("prompt is not the v3 sentence:\n%s", head)
	}
	block := v3AskWordBlock(target.Text)
	if !strings.Contains(head, block) {
		t.Fatalf("word markup is not the v3 card %s:\n%s", block, head)
	}
	for _, gone := range []string{
		`class="target"`, "target-slot", "data-target", "This word", "What does “",
		`<bdi lang="he" dir="rtl">` + target.Text + `</bdi><bdi lang="en"`,
	} {
		if strings.Contains(ask, gone) {
			t.Errorf("one-word recall contains %q", gone)
		}
	}
	for _, gone := range []string{`class="wb__tr"`, `class="wb__gl"`, ">" + target.Translit + "<", ">" + target.Gloss + "<"} {
		if strings.Contains(head, gone) {
			t.Errorf("one-word prompt shows %q before the answer", gone)
		}
	}
	if !strings.Contains(ask, `value="`+itoa(target.ID)+`"`) {
		t.Fatalf("the correct choice is not the lexeme id:\n%s", ask)
	}
	path := "/lesson/" + itoa(unitID) + "/0"
	revealed := activityHTML(t, postLesson(t, fx.h, cookie, path, runValue(t, body), itoa(target.ID), true, "csrf-v3").Body.String())
	filled := v3RevealWordBlock(target.Text, target.Translit, target.Gloss)
	if !strings.Contains(revealed, filled) {
		t.Fatalf("reveal is not the v3 word block %s:\n%s", filled, revealed)
	}
	if strings.Contains(revealed, `class="target"`) || strings.Contains(revealed, "target-slot") {
		t.Fatalf("one-word reveal grew the multi-word mark:\n%s", revealed)
	}
}

// TestRecallNoJSFallbackMarkup is the no-JS contract for a lesson clip.
// The line clip holds one native audio element with controls and one orb.
// A recall word's orb stays under the word and names its bar with
// data-audio. That bar is the only control in a .clip-rows row, so the
// no-JS audio is not squeezed into the word column. Without clips.js the
// bar is the control. This is the fallback the lead allowed.
func TestRecallNoJSFallbackMarkup(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.CreateUser(ctx, "clips@example.com", "sub-clips", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	if _, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: rwAni, Gloss: "I", Translit: sp("ani"), POS: sp("pron")},
		{TextNFC: rwGar, Gloss: "lives", Translit: sp("gar"), POS: sp("verb")},
		{TextNFC: rwPo, Gloss: "here", Translit: sp("po"), POS: sp("adv")},
	}); err != nil {
		t.Fatal(err)
	}
	line := rwAni + " " + rwGar + " " + rwPo
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Clips", Situation: "A line.", Position: 0,
		NewWords: []string{rwGar},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: line},
		}}},
	}, "shared")
	publish(t, store, unitID)
	ids := []string{strings.Repeat("a1", 32), strings.Repeat("b2", 32), strings.Repeat("c3", 32), strings.Repeat("d4", 32)}
	texts := []string{units.Normalize(line), units.Normalize(rwAni), units.Normalize(rwGar), units.Normalize(rwPo)}
	for i, id := range ids {
		if _, err := store.InsertAudio(ctx, db.Asset{ID: id, TextNFC: texts[i], Voice: "voice", Model: "eleven_v4", Bytes: 4}); err != nil {
			t.Fatal(err)
		}
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: fixedClock(now), Store: store,
		AllowedEmails: []string{"clips@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	cookie := mintSession(t, store, user.ID, "csrf-clips", now.Add(time.Hour))
	body := getAuthed(t, h, cookie, "/lesson/"+itoa(unitID)+"/0")
	ask := activityHTML(t, body)
	heroes := regexp.MustCompile(`(?s)<div class="clip clip--hero">(.*?)</div>`).FindAllStringSubmatch(ask, -1)
	if len(heroes) != 1 {
		t.Fatalf("hero clips = %d, want the line\n%s", len(heroes), ask)
	}
	hero := heroes[0][1]
	if strings.Count(hero, "<audio ") != 1 || strings.Count(hero, "<button ") != 1 || !strings.Contains(hero, `class="orb orb--hero"`) {
		t.Fatalf("the line clip is not one audio and one orb:\n%s", hero)
	}
	if !regexp.MustCompile(`<audio\b[^>]*\bcontrols\b`).MatchString(hero) || !strings.Contains(hero, `preload="none"`) || !strings.Contains(hero, `id="line-clip"`) {
		t.Fatalf("the line audio is not a preload=none control:\n%s", hero)
	}
	rowsAt := strings.Index(ask, `<div class="clip-rows">`)
	if rowsAt < 0 {
		t.Fatalf("recall words have no .clip-rows fallback:\n%s", ask)
	}
	rowsHTML := ask[rowsAt:]
	rowAudios := regexp.MustCompile(`<div class="clip"><audio\b([^>]*)></audio></div>`).FindAllStringSubmatch(rowsHTML, -1)
	wordOrbs := regexp.MustCompile(`<button type="button" class="orb orb--word"[^>]*data-audio="([^"]+)"`).FindAllStringSubmatch(ask, -1)
	if len(rowAudios) != 3 || len(wordOrbs) != 3 {
		t.Fatalf("word bars = %d, word orbs = %d, want one of each for three words\n%s", len(rowAudios), len(wordOrbs), ask)
	}
	for _, audio := range rowAudios {
		if strings.Contains(audio[0], "<button ") {
			t.Fatalf("a no-JS word row also contains an orb:\n%s", audio[0])
		}
	}
	seen := map[string]int{}
	for _, orb := range wordOrbs {
		seen[orb[1]]++
	}
	for _, audio := range rowAudios {
		attrs := audio[1]
		if !regexp.MustCompile(`\bcontrols\b`).MatchString(attrs) || !strings.Contains(attrs, `preload="none"`) {
			t.Fatalf("word bar is not a preload=none control:\n%s", attrs)
		}
		id := regexp.MustCompile(`\bid="([^"]+)"`).FindStringSubmatch(attrs)
		if id == nil {
			t.Fatalf("word bar has no id:\n%s", attrs)
		}
		seen[id[1]]++
	}
	for id, n := range seen {
		if n != 2 {
			t.Fatalf("clip %s is referenced %d times, want one orb and one bar", id, n)
		}
	}
	if strings.Count(ask, "<audio ") != 4 {
		t.Fatalf("audio elements = %d, want the line and three words\n%s", strings.Count(ask, "<audio "), ask)
	}
	if !strings.Contains(ask, "/static/js/clips.js") {
		t.Fatal("clips.js is missing; the JS-on orb has nothing to upgrade")
	}
}

// TestGradedRecallAndClozeNeedThreeChoices locks the floor #105 enforces.
// A graded recall or cloze renders at least three choice buttons. Fewer than
// three is a reveal: no answer buttons, and posting a lexeme id does not
// record an attempt. The floor is #105's; this test only checks it.
func TestGradedRecallAndClozeNeedThreeChoices(t *testing.T) {
	const learners = 6
	orders := map[string]map[string]int{}
	emails := make([]string, learners)
	for i := range emails {
		emails[i] = fmt.Sprintf("floor%d@example.com", i)
	}
	fx := openVerbChoiceFixture(t, emails, []units.Activity{
		{Type: units.ActivityRecall, Text: rwGar},
		{Type: units.ActivityCloze, Text: rwGar},
		{Type: units.ActivityRecall, Text: rwOkhel},
		{Type: units.ActivityRecall, Text: rwShote},
	}, []string{rwGar, rwOkhel, rwShote})
	for i := 0; i < learners; i++ {
		email := fmt.Sprintf("floor%d@example.com", i)
		csrf := fmt.Sprintf("csrf-floor-%d", i)
		user, err := fx.store.UserByEmail(context.Background(), email)
		if err != nil {
			t.Fatal(err)
		}
		cookie := recallSession(t, fx.recallFixture, email, csrf)
		path := "/lesson/" + itoa(fx.unitID) + "/0"
		body := getAuthed(t, fx.h, cookie, path)
		for step, typ := range []string{units.ActivityRecall, units.ActivityCloze} {
			ask := activityHTML(t, body)
			ids := gradedChoiceValues(ask)
			if len(ids) < lessonMinChoices {
				t.Fatalf("%s learner %d step %d has %d buttons, a graded ask needs at least %d:\n%s", typ, i, step, len(ids), lessonMinChoices, ask)
			}
			if !containsString(ids, itoa(fx.targetID)) {
				t.Fatalf("%s learner %d step %d buttons %v omit the target %d", typ, i, step, ids, fx.targetID)
			}
			key := typ
			if orders[key] == nil {
				orders[key] = map[string]int{}
			}
			orders[key][strings.Join(ids, ",")]++
			graded := postLesson(t, fx.h, cookie, path, runValue(t, body), itoa(fx.targetID), true, csrf)
			if graded.Code != http.StatusOK || !strings.Contains(graded.Body.String(), "feedback--right") {
				t.Fatalf("%s learner %d grade: %d\n%s", typ, i, graded.Code, graded.Body.String())
			}
			if n := countRows(t, fx.store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, user.ID); n != step+1 {
				t.Fatalf("%s learner %d attempts = %d, want %d", typ, i, n, step+1)
			}
			next := postLesson(t, fx.h, cookie, path, runValue(t, body), "continue", true, csrf)
			if next.Code != http.StatusOK {
				t.Fatalf("%s learner %d continue: %d\n%s", typ, i, next.Code, next.Body.String())
			}
			body = next.Body.String()
		}
	}
	for typ, set := range orders {
		if len(set) < 2 {
			t.Fatalf("%s button order did not vary across %d learners: %+v", typ, learners, set)
		}
	}

	for i := 0; i < 4; i++ {
		for _, typ := range []string{units.ActivityRecall, units.ActivityCloze} {
			email := fmt.Sprintf("thin-%s-%d@example.com", typ, i)
			csrf := fmt.Sprintf("csrf-thin-%s-%d", typ, i)
			fx := openThinChoiceFixture(t, email, typ)
			cookie := recallSession(t, fx.recallFixture, email, csrf)
			path := "/lesson/" + itoa(fx.unitID) + "/0"
			body := getAuthed(t, fx.h, cookie, path)
			ask := activityHTML(t, body)
			if ids := gradedChoiceValues(ask); len(ids) > 0 || strings.Contains(ask, `class="choices"`) {
				t.Fatalf("%s learner %d rendered %v buttons; fewer than 3 must be a reveal:\n%s", typ, i, ids, ask)
			}
			if !strings.Contains(ask, `value="continue"`) {
				t.Fatalf("%s learner %d reveal has no Continue:\n%s", typ, i, ask)
			}
			posted := postLesson(t, fx.h, cookie, path, runValue(t, body), itoa(fx.targetID), true, csrf)
			if posted.Code != http.StatusOK {
				t.Fatalf("%s learner %d post id: %d\n%s", typ, i, posted.Code, posted.Body.String())
			}
			if strings.Contains(posted.Body.String(), "feedback--right") || strings.Contains(posted.Body.String(), `class="choices"`) {
				t.Fatalf("%s learner %d graded a reveal:\n%s", typ, i, posted.Body.String())
			}
			if n := countRows(t, fx.store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, fx.userID); n != 0 {
				t.Fatalf("%s learner %d recorded %d attempts on a reveal", typ, i, n)
			}
			done := postLesson(t, fx.h, cookie, path, runValue(t, body), "continue", true, csrf)
			if done.Code != http.StatusOK || !strings.Contains(done.Body.String(), "Lesson complete.") {
				t.Fatalf("%s learner %d continue: %d\n%s", typ, i, done.Code, done.Body.String())
			}
			if n := countRows(t, fx.store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, fx.userID); n != 0 {
				t.Fatalf("%s learner %d continue recorded %d attempts", typ, i, n)
			}
		}
	}
}

type choiceFixture struct {
	recallFixture
	unitID, userID, targetID int64
}

func openThinChoiceFixture(t *testing.T, email, typ string) choiceFixture {
	t.Helper()
	return openVerbChoiceFixture(t, []string{email}, []units.Activity{
		{Type: typ, Text: rwGar + " " + rwPo},
	}, []string{rwGar, rwPo})
}

func openVerbChoiceFixture(t *testing.T, emails []string, acts []units.Activity, words []string) choiceFixture {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	var user db.User
	for _, email := range emails {
		created, err := store.CreateUser(ctx, email, "sub-"+email, auth.RoleLearner)
		if err != nil {
			t.Fatal(err)
		}
		if user.ID == 0 {
			user = created
		}
	}
	var inputs []lexicon.Input
	for _, w := range recallLexicon() {
		inputs = append(inputs, w.input())
	}
	if _, _, err := store.UpsertLexemes(ctx, inputs); err != nil {
		t.Fatal(err)
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Choices", Situation: "Recall.", Position: 0,
		NewWords: words,
		Lessons:  []units.Lesson{{Kind: units.KindLesson, Activities: acts}},
	}, "shared")
	publish(t, store, unitID)
	target := rwGar
	lex, err := store.LexemesByText(ctx, []string{units.Normalize(target)})
	if err != nil {
		t.Fatal(err)
	}
	view, ok := lex[units.Normalize(target)]
	if !ok {
		t.Fatal("target missing")
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: fixedClock(now), Store: store,
		AllowedEmails: emails,
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	return choiceFixture{
		recallFixture: recallFixture{h: h, store: store},
		unitID:        unitID,
		userID:        user.ID,
		targetID:      view.ID,
	}
}

func gradedChoiceValues(html string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`(?s)<button\b([^>]*)>`).FindAllStringSubmatch(html, -1) {
		open := m[1]
		if !strings.Contains(open, `name="answer"`) {
			continue
		}
		vm := regexp.MustCompile(`value="(\d+)"`).FindStringSubmatch(open)
		if vm == nil {
			continue
		}
		out = append(out, vm[1])
	}
	return out
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
