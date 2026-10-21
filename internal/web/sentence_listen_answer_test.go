package web

import (
	"context"
	"html"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Distractors are drawn from a pool larger than the cap. Two items in one
// unit must not keep one document-order set.
func TestSentenceDistractorsDifferAcrossItems(t *testing.T) {
	answer := slAnswer
	heard := units.Normalize(slAni + " " + slGar + " " + slPo)
	lines := []string{
		"I live there", "I live nearby", "We live here", "I stay here",
		"I live upstairs", "People live here", "I live alone", "They live here",
		"I live quietly", "We stay here", "I remain here", "People stay here",
	}
	var same []spokenLine
	for i, en := range lines {
		same = append(same, spokenLine{Hebrew: "line-" + itoa(int64(i)), English: en})
	}
	seen := map[string]bool{}
	for _, text := range []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta"} {
		got := sentenceDistractors(answer, heard, same, nil, sentenceChoiceRand(db.PlayerRun{ID: 11}, units.Activity{Text: text}))
		if len(got) != sentenceChoiceCount-1 {
			t.Fatalf("item %q distractors = %q, want %d", text, got, sentenceChoiceCount-1)
		}
		seen[labelSet(got)] = true
	}
	if len(seen) < 2 {
		t.Fatalf("items in one unit shared one distractor set: %v", seen)
	}
}

func TestSentenceDistractorsExcludeSameHebrewAndNearDuplicates(t *testing.T) {
	answer := slAnswer
	heard := units.Normalize(slAni + " " + slGar + " " + slPo)
	same := []spokenLine{
		{Hebrew: heard, English: "People stay indoors"},
		{Hebrew: "a", English: "I live here now"},
		{Hebrew: "b", English: "Live here I do"},
		{Hebrew: "c", English: "I live there"},
		{Hebrew: "d", English: "I live nearby"},
	}
	got := sentenceDistractors(answer, heard, same, nil, sentenceChoiceRand(db.PlayerRun{ID: 4}, units.Activity{Text: heard}))
	if slices.Contains(got, "People stay indoors") {
		t.Fatalf("same Hebrew was offered: %q", got)
	}
	if slices.Contains(got, "I live here now") || slices.Contains(got, "Live here I do") {
		t.Fatalf("near duplicate was offered: %q", got)
	}
	if !sameStrings(got, []string{"I live there", "I live nearby"}) {
		t.Fatalf("distractors = %q, want the two lines that are not the same Hebrew and not near duplicates", got)
	}
}

func TestSentenceDistractorsPreferSameUnitThenEarlier(t *testing.T) {
	answer := slAnswer
	heard := units.Normalize(slAni + " " + slGar + " " + slPo)
	same := []spokenLine{
		{Hebrew: "s1", English: "I live there"},
		{Hebrew: "s2", English: "I live nearby"},
		{Hebrew: "s3", English: "We live here"},
		{Hebrew: "long", English: "The afternoon weather stays pleasant outside"},
	}
	earlier := []spokenLine{
		{Hebrew: "e1", English: "I stay here"},
		{Hebrew: "e2", English: "I live upstairs"},
		{Hebrew: "e3", English: "People live here"},
	}
	for i := int64(1); i <= 24; i++ {
		got := sentenceDistractors(answer, heard, same, earlier, sentenceChoiceRand(db.PlayerRun{ID: i}, units.Activity{Text: "prefer"}))
		if !sameStrings(got, []string{"I live there", "I live nearby", "We live here"}) {
			t.Fatalf("seed %d distractors = %q, want the three same-unit lines", i, got)
		}
	}
}

func TestSentenceAskUsesFourAtMostAndUngradesBelowThree(t *testing.T) {
	if sentenceChoiceCount != 4 || sentenceMinChoices != 3 {
		t.Fatalf("count = %d min = %d, want 4 and 3", sentenceChoiceCount, sentenceMinChoices)
	}
	key := []byte("sentence-listen-test-key")
	run := db.PlayerRun{ID: 9, ActivityIx: 1}
	act := units.Activity{Text: slAni + " " + slGar, Translation: slAnswer}
	one := []spokenLine{{Hebrew: "a", English: "I live there"}}
	if sentenceAsk(key, run, act, one, nil) != nil {
		t.Fatal("one distractor was still a question")
	}
	two := append(one, spokenLine{Hebrew: "b", English: "I live nearby"})
	got := sentenceAsk(key, run, act, two, nil)
	if len(got) != 3 {
		t.Fatalf("two distractors produced %d buttons, want 3", len(got))
	}
	labels := make([]string, len(got))
	for i, c := range got {
		labels[i] = c.Label
		if c.Value == "" || c.Value == c.Label {
			t.Fatalf("choice %q value = %q, want an opaque id", c.Label, c.Value)
		}
	}
	if !sameStrings(labels, []string{slAnswer, "I live there", "I live nearby"}) {
		t.Fatalf("buttons = %q", labels)
	}
	if _, ok := sentenceChoiceIndex(key, run, got, slAnswer); ok {
		t.Fatal("the English string matched a choice id")
	}
	if ix, ok := sentenceChoiceIndex(key, run, got, got[1].Value); !ok || ix != 1 {
		t.Fatalf("id lookup = %d %v", ix, ok)
	}
}

func TestSentenceListenItemsInOneUnitDoNotShareDistractors(t *testing.T) {
	line := slAni + " " + slGar + " " + slPo
	other := slBayit + " " + slMayim
	acts := []units.Activity{
		{Type: units.ActivityListenPick, Text: line, Translation: slAnswer},
		{Type: units.ActivityListenPick, Text: other, Translation: slAnswer},
	}
	for i, en := range []string{
		"I live there", "I live nearby", "We live here", "I stay here",
		"I live upstairs", "People live here", "I live alone", "They live here",
		"I live quietly", "We stay here", "I remain here", "People stay here",
	} {
		acts = append(acts, units.Activity{Type: units.ActivityStory, Text: "שורה " + itoa(int64(i)), Translation: en})
	}
	env := openSentenceLesson(t, acts, &listenRec{})
	body := getAuthed(t, env.h, env.cookie, env.path)
	first := sentenceLabels(t, body)
	rec := postLesson(t, env.h, env.cookie, env.path, runValue(t, body), choiceValue(t, body, slAnswer), true, env.csrf)
	if rec.Code != http.StatusOK {
		t.Fatalf("answer = %d\n%s", rec.Code, rec.Body.String())
	}
	next := postLesson(t, env.h, env.cookie, env.path, runValue(t, body), "continue", true, env.csrf)
	second := sentenceLabels(t, next.Body.String())
	if len(first) != 4 || len(second) != 4 || !slices.Contains(first, slAnswer) || !slices.Contains(second, slAnswer) {
		t.Fatalf("choices %q and %q, want 4 including the translation", first, second)
	}
	if labelSet(first) == labelSet(second) {
		t.Fatalf("two items shared one distractor set: %q", first)
	}
}

func TestSentenceListenFillsFromEarlierUnits(t *testing.T) {
	line := slAni + " " + slGar + " " + slPo
	env := openSentenceLesson(t, []units.Activity{
		{Type: units.ActivityListenPick, Text: line, Translation: slAnswer},
		{Type: units.ActivityRecall, Text: slBayit, Target: slBayit, Translation: "Good evening"},
	}, nil)
	earlierID := putLesson(t, env.store, nil, units.Unit{
		Title: "Before", Situation: "Earlier lines.", Position: 0,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: "קודם", Translation: "I live there"},
			{Type: units.ActivityStory, Text: "אַחַר", Translation: "I live nearby"},
		}}},
	}, "shared")
	publish(t, env.store, earlierID)
	finishLesson(t, env, earlierID)
	laterID := putLesson(t, env.store, nil, units.Unit{
		Title: "Later", Situation: "A later line.", Position: 2,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: "אַחֲרֵי", Translation: "The late train leaves"},
		}}},
	}, "shared")
	publish(t, env.store, laterID)

	lesson, err := env.store.CourseLesson(context.Background(), env.userID, env.unitID, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, earlier, err := (player{store: env.store}).spokenLines(context.Background(), env.userID, lesson)
	if err != nil {
		t.Fatal(err)
	}
	var earlierEnglish []string
	for _, line := range earlier {
		earlierEnglish = append(earlierEnglish, line.English)
	}
	if !sameStrings(earlierEnglish, []string{"I live there", "I live nearby"}) {
		t.Fatalf("earlier lines = %q", earlierEnglish)
	}

	labels := sentenceLabels(t, getAuthed(t, env.h, env.cookie, env.path))
	if !sameStrings(labels, []string{slAnswer, "Good evening", "I live there", "I live nearby"}) {
		t.Fatalf("choices = %q, want the same-unit line plus the earlier lines", labels)
	}
}

func TestSentenceListenFewerThanThreePlaysUngraded(t *testing.T) {
	line := slAni + " " + slGar + " " + slPo
	env := openSentenceLesson(t, []units.Activity{{
		Type: units.ActivityListenPick, Text: line, Translation: slAnswer,
		Lines: []units.DialogueLine{{Speaker: "a", Text: slBayit, Translation: "Good evening"}},
	}}, &listenRec{})
	body := getAuthed(t, env.h, env.cookie, env.path)
	ask := activityHTML(t, body)
	if strings.Contains(ask, `class="choices`) || strings.Contains(ask, slAnswer) || strings.Contains(ask, "Good evening") {
		t.Fatalf("ungraded ask showed a question or the answer:\n%s", ask)
	}
	if strings.Contains(ask, `class="activity-word"`) {
		t.Fatal("ungraded ask reserved empty word slots")
	}
	if !strings.Contains(ask, "Listen, then pick what it means.") || !strings.Contains(ask, `value="continue"`) {
		t.Fatalf("ungraded ask:\n%s", ask)
	}
	rec := postLesson(t, env.h, env.cookie, env.path, runValue(t, body), "continue", true, env.csrf)
	reveal := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(reveal, `class="lp-fb__tr"`) || !strings.Contains(reveal, slAnswer) {
		t.Fatalf("reveal = %d\n%s", rec.Code, reveal)
	}
	for _, banned := range []string{"Right.", "That's right.", "Here it is.", "Not quite.", `class="choices`, `class="c-answer"`} {
		if strings.Contains(reveal, banned) {
			t.Fatalf("ungraded reveal contains %q\n%s", banned, reveal)
		}
	}
	if len(env.rec.attempts) != 0 || len(env.rec.exposures) != 1 {
		t.Fatalf("attempts %d exposures %d", len(env.rec.attempts), len(env.rec.exposures))
	}
	done := postLesson(t, env.h, env.cookie, env.path, runValue(t, body), "continue", true, env.csrf)
	if done.Code != http.StatusOK || !strings.Contains(done.Body.String(), "Lesson complete.") {
		t.Fatalf("finish = %d\n%s", done.Code, done.Body.String())
	}
	if len(env.rec.exposures) != 1 {
		t.Fatalf("continuing the reveal logged %d exposures", len(env.rec.exposures))
	}
	if n := scalar(t, env.dbPath, `SELECT COUNT(*) FROM lesson_runs WHERE finished_at IS NOT NULL AND score = 1`); n != 1 {
		t.Fatalf("finished with a full score = %d, want 1", n)
	}
}

func TestSentenceListenKeepsMarkedChoices(t *testing.T) {
	env := openSentenceLesson(t, sentenceActivities(false), &listenRec{})
	body := getAuthed(t, env.h, env.cookie, env.path)
	if strings.Contains(body, `class="activity-word"`) || strings.Contains(body, `class="lp-reveal"`) {
		t.Fatal("the ask shows the line or an empty word slot")
	}
	right := postLesson(t, env.h, env.cookie, env.path, runValue(t, body), choiceValue(t, body, slAnswer), true, env.csrf)
	assertMarked(t, right.Body.String(), slAnswer, "")
	reload := getAuthed(t, env.h, env.cookie, env.path)
	assertMarked(t, reload, slAnswer, "")

	missEnv := openSentenceLesson(t, sentenceActivities(false), &listenRec{})
	missBody := getAuthed(t, missEnv.h, missEnv.cookie, missEnv.path)
	miss := postLesson(t, missEnv.h, missEnv.cookie, missEnv.path, runValue(t, missBody), choiceValue(t, missBody, slHouse), true, missEnv.csrf)
	assertMarked(t, miss.Body.String(), slAnswer, slHouse)
}

func TestSentenceListenRevealShowsTheTranslation(t *testing.T) {
	env := openSentenceLesson(t, sentenceActivities(false), &listenRec{})
	if _, err := env.store.InsertAudio(context.Background(), db.Asset{
		ID: strings.Repeat("ab", 32), TextNFC: units.Normalize(slAni + " " + slGar + " " + slPo),
		Voice: "narrator", Model: "eleven_v4", Bytes: 8,
	}); err != nil {
		t.Fatal(err)
	}
	body := getAuthed(t, env.h, env.cookie, env.path)
	rec := postLesson(t, env.h, env.cookie, env.path, runValue(t, body), choiceValue(t, body, slAnswer), true, env.csrf)
	got := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("answer = %d\n%s", rec.Code, got)
	}
	mustOrder(t, got,
		"clip--hero",
		`class="lp-reveal"`,
		`class="word-blocks" dir="rtl"`,
		`class="lp-fb__tr"`,
		`class="word-blocks fbline"`,
		`class="choices choices--sentence"`,
		`class="lf-result"`,
		`role="status"`,
	)
	tr := between(t, got, `class="lp-fb__tr"`, "</p>")
	if !strings.Contains(tr, "“I live here”") || strings.Contains(tr, "lives") {
		t.Fatalf("translation line = %q", tr)
	}
	glosses := between(t, got, `class="word-blocks fbline"`, "</div>")
	for _, g := range []string{">I<", ">lives<", ">here<"} {
		if !strings.Contains(glosses, g) {
			t.Fatalf("muted glosses missing %s:\n%s", g, glosses)
		}
	}
	if strings.Contains(got, "I lives here") || strings.Contains(got, "I / lives") {
		t.Fatal("the reveal stitches the glosses into the English line")
	}
}

func TestSentenceListenFeedbackNamesAMissingWord(t *testing.T) {
	env := openSentenceLesson(t, sentenceActivities(false), &listenRec{})
	body := getAuthed(t, env.h, env.cookie, env.path)
	right := postLesson(t, env.h, env.cookie, env.path, runValue(t, body), choiceValue(t, body, slAnswer), true, env.csrf)
	rightBody := right.Body.String()
	if !strings.Contains(rightBody, "Right. “I live here.”") || strings.Contains(rightBody, "You picked") || strings.Contains(rightBody, "Here it is.") || strings.Contains(rightBody, "That's right.") {
		t.Fatalf("right copy:\n%s", rightBody)
	}
	if !strings.Contains(rightBody, `role="status"`) {
		t.Fatal("the answer sheet is not a status")
	}

	missEnv := openSentenceLesson(t, sentenceActivities(false), &listenRec{})
	missBody := getAuthed(t, missEnv.h, missEnv.cookie, missEnv.path)
	miss := postLesson(t, missEnv.h, missEnv.cookie, missEnv.path, runValue(t, missBody), choiceValue(t, missBody, slHouse), true, missEnv.csrf)
	got := miss.Body.String()
	for _, want := range []string{
		"Not quite. It says “I live here.”",
		"You picked “This is a house”.",
		`lang="he" dir="rtl">` + slAni,
		"ani, I.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("miss copy missing %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "Here it is.") || strings.Contains(got, "That's right.") || strings.Contains(got, "Right.") {
		t.Fatalf("miss copy uses the old line:\n%s", got)
	}
}

func TestSentenceListenFeedbackOmitsTheMissingWordWhenNone(t *testing.T) {
	acts := sentenceActivities(false)
	acts = append(acts, units.Activity{
		Type: units.ActivityProduce, Text: slAni + " " + slGar + " " + slPo + " " + slBayit,
		Target: slBayit, Translation: "We live here too",
	})
	env := openSentenceLesson(t, acts, &listenRec{})
	body := getAuthed(t, env.h, env.cookie, env.path)
	rec := postLesson(t, env.h, env.cookie, env.path, runValue(t, body), choiceValue(t, body, "We live here too"), true, env.csrf)
	got := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(got, "Not quite. It says “I live here.”") {
		t.Fatalf("miss = %d\n%s", rec.Code, got)
	}
	if strings.Contains(got, "You picked") || strings.Contains(got, `class="lp-miss"`) {
		t.Fatalf("a line that has every heard word still drew a contrast:\n%s", got)
	}
}

func TestSentenceListenRejectsTheEnglishString(t *testing.T) {
	for _, hx := range []bool{true, false} {
		env := openSentenceLesson(t, sentenceActivities(false), &listenRec{})
		body := getAuthed(t, env.h, env.cookie, env.path)
		rec := postSentence(t, env, runValue(t, body), slAnswer, hx)
		got := rec.Body.String()
		if rec.Code != http.StatusOK || strings.Contains(got, "feedback--right") || strings.Contains(got, "feedback--wrong") || strings.Contains(got, `data-state="correct"`) {
			t.Fatalf("hx %v accepted the English string\n%s", hx, got)
		}
		if len(sentenceLabels(t, got)) != 4 {
			t.Fatal("rejecting the English string left the ask")
		}
		if len(env.rec.exposures) != 0 || len(env.rec.attempts) != 0 {
			t.Fatal("a rejected post was recorded")
		}
	}
}

func TestSentenceListenPromptAndEnglishTitle(t *testing.T) {
	env := openSentenceLesson(t, sentenceActivities(false), nil)
	body := getAuthed(t, env.h, env.cookie, env.path)
	if !strings.Contains(body, "Listen, then pick what it means.") {
		t.Fatal("sentence prompt missing")
	}
	if strings.Contains(body, `<bdi lang="he" dir="rtl">At home</bdi>`) || !strings.Contains(body, `<bdi lang="en" dir="ltr">At home</bdi>`) {
		t.Fatal("the English title is wrapped as Hebrew")
	}
	if _, err := env.store.ExecContext(context.Background(), `UPDATE units SET title = ? WHERE id = ?`, "בַּיִת", env.unitID); err != nil {
		t.Fatal(err)
	}
	hebrew := getAuthed(t, env.h, env.cookie, env.path)
	if !strings.Contains(hebrew, `<bdi lang="he" dir="rtl">בַּיִת</bdi>`) {
		t.Fatal("a Hebrew title was not wrapped as Hebrew")
	}
}

func TestSentenceChoicesStayOneColumn(t *testing.T) {
	css := readStatic(t, "static/css/shell.css")
	block := between(t, css, ".choices--sentence {", "}")
	if !strings.Contains(block, "grid-template-columns: 1fr;") {
		t.Fatalf("sentence choices are not one column:\n%s", block)
	}
	wide := strings.Index(css, "@media (min-width: 1024px)")
	if wide < 0 {
		t.Fatal("missing the 1024px rule")
	}
	rest := css[wide:]
	two := strings.Index(rest, ".lesson-frame .choices { grid-template-columns: 1fr 1fr; }")
	one := strings.Index(rest, ".lesson-frame .choices--sentence { grid-template-columns: 1fr; }")
	if two < 0 || one < 0 || one < two {
		t.Fatal("the wide layout does not put sentence choices back to one column")
	}
	if !strings.Contains(css, ".lesson-frame form:has(.lf-result) { padding-block-end: 16rem; }") {
		t.Fatal("the step is not padded by the sheet")
	}
	if !strings.Contains(css, ".lesson-frame .lf-result .btn-primary { min-block-size: var(--size-cta); }") {
		t.Fatal("Continue is not the 56px control")
	}
	if !strings.Contains(readStatic(t, "static/css/tokens.css"), "--size-cta: 56px;") {
		t.Fatal("--size-cta is not 56px")
	}
	if !strings.Contains(css, ".fbline .wb__gl { font-size: var(--fs-small); color: var(--muted); }") {
		t.Fatal("reveal glosses are not muted")
	}
}

func assertMarked(t *testing.T, body, right, wrong string) {
	t.Helper()
	if !strings.Contains(body, `class="choices choices--sentence"`) {
		t.Fatalf("choices disappeared:\n%s", body)
	}
	got := markedChoices(t, body)
	if len(got) != 4 {
		t.Fatalf("marked choices = %+v, want 4", got)
	}
	var rights, wrongs int
	for _, c := range got {
		if !c.disabled {
			t.Fatalf("choice %q is still enabled", c.label)
		}
		switch c.state {
		case "correct":
			rights++
			if c.label != right || c.tag != "Right" || !c.mark {
				t.Fatalf("correct choice = %+v", c)
			}
		case "wrong":
			wrongs++
			if c.label != wrong || c.tag != "Your pick" || !c.mark {
				t.Fatalf("wrong choice = %+v, want %q", c, wrong)
			}
		case "dim":
			if c.tag != "" || c.mark {
				t.Fatalf("dim choice = %+v", c)
			}
		default:
			t.Fatalf("choice state %q", c.state)
		}
	}
	wantWrong := 0
	if wrong != "" {
		wantWrong = 1
	}
	if rights != 1 || wrongs != wantWrong {
		t.Fatalf("right %d wrong %d in %+v", rights, wrongs, got)
	}
}

type markedChoice struct {
	label, state, tag string
	disabled, mark    bool
}

func markedChoices(t *testing.T, body string) []markedChoice {
	t.Helper()
	var out []markedChoice
	for _, m := range regexp.MustCompile(`(?s)<button\b([^>]*)>(.*?)</button>`).FindAllStringSubmatch(choiceRegion(body), -1) {
		inner := m[2]
		tag := ""
		if i := strings.Index(inner, `class="lp-tag"`); i >= 0 {
			rest := inner[i:]
			if gt := strings.Index(rest, ">"); gt >= 0 {
				rest = rest[gt+1:]
			}
			if end := strings.Index(rest, "</span>"); end >= 0 {
				rest = rest[:end]
			}
			tag = strings.TrimSpace(stripTags(rest))
		}
		out = append(out, markedChoice{
			label:    html.UnescapeString(strings.TrimSpace(stripTags(labelSpan(inner)))),
			state:    attrFromTag(m[1], "data-state"),
			tag:      html.UnescapeString(tag),
			disabled: strings.Contains(m[1], "disabled"),
			mark:     strings.Contains(inner, "c-answer__mark"),
		})
	}
	if len(out) == 0 {
		t.Fatalf("no marked choices:\n%s", choiceRegion(body))
	}
	return out
}

func labelSet(labels []string) string {
	cp := slices.Clone(labels)
	slices.Sort(cp)
	return strings.Join(cp, "|")
}

func mustOrder(t *testing.T, body string, parts ...string) {
	t.Helper()
	prev := -1
	for _, part := range parts {
		i := strings.Index(body, part)
		if i < 0 || i <= prev {
			t.Fatalf("%q is out of order", part)
		}
		prev = i
	}
}

func between(t *testing.T, body, start, end string) string {
	t.Helper()
	i := strings.Index(body, start)
	if i < 0 {
		t.Fatalf("missing %q", start)
	}
	rest := body[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		t.Fatalf("missing %q after %q", end, start)
	}
	return rest[:j]
}

func finishLesson(t *testing.T, env sentenceEnv, unitID int64) {
	t.Helper()
	stamp := db.FormatTime(env.now)
	if _, err := env.store.ExecContext(context.Background(), `
		INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score)
		VALUES (?, ?, 0, ?, ?, 1)`, env.userID, unitID, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}
