package web

import (
	"encoding/json"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// TestQAChoiceButtons is the player-level check for the old multiple-choice
// leak: distractors were the unit's first two words alphabetically, sorted,
// so the correct button was always index 2. choices.Pick is covered in
// internal/choices/pick_test.go. This test only talks to the HTTP handler.
func TestQAChoiceButtons(t *testing.T) {
	fx := openLessonPlayer(t)
	// A second learner needs to be on the allowlist. Lookup drops a session
	// whose address is not allowed, which would 303 the choice page.
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         fixedClock(fx.now),
		Store:         fx.store,
		AllowedEmails: []string{"brian@example.com", "quinn@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	fx.h = NewOptions(nil, svc, Options{Store: fx.store})
	doc := loadQAChoiceUnit(t, fx)
	if len(doc.Lexemes) < 5 || len(doc.Unit.NewWords) < 5 {
		t.Fatalf("fixture has %d lexemes and %d new words", len(doc.Lexemes), len(doc.Unit.NewWords))
	}
	byText := map[string]qaLexeme{}
	for _, lex := range doc.Lexemes {
		byText[lex.Text] = lex
	}
	lesson := doc.Unit.Lessons[0]
	check := doc.Unit.Lessons[1]
	if lesson.Kind != units.KindLesson || check.Kind != units.KindCheckpoint {
		t.Fatalf("fixture kinds %s, %s", lesson.Kind, check.Kind)
	}
	assertGradedTypes(t, lesson.Activities)
	assertGradedTypes(t, check.Activities)

	path := "/lesson/" + itoa(doc.UnitID) + "/0"
	checkPath := "/lesson/" + itoa(doc.UnitID) + "/1"
	primary := qaClient{h: fx.h, cookie: fx.cookie, csrf: lessonCSRF}
	other := qaSecondLearner(t, fx)

	var lessonAsks []qaAsk
	differed := false
	gradedRight := false
	gradedWrong := false
	for step, act := range lesson.Activities {
		first := getAuthed(t, fx.h, fx.cookie, path)
		again := getAuthed(t, fx.h, fx.cookie, path)
		otherFirst := getAuthed(t, other.h, other.cookie, path)
		otherAgain := getAuthed(t, other.h, other.cookie, path)

		ask := qaReadAsk(t, first, act, byText)
		againAsk := qaReadAsk(t, again, act, byText)
		otherAsk := qaReadAsk(t, otherFirst, act, byText)
		otherAgainAsk := qaReadAsk(t, otherAgain, act, byText)
		if ask.runID != againAsk.runID || otherAsk.runID != otherAgainAsk.runID {
			t.Fatalf("step %d run id changed on re-render: %s/%s and %s/%s", step, ask.runID, againAsk.runID, otherAsk.runID, otherAgainAsk.runID)
		}
		if ask.runID == otherAsk.runID {
			t.Fatalf("step %d both learners share run %s", step, ask.runID)
		}
		assertSameButtons(t, "reload", step, ask, againAsk)
		assertSameButtons(t, "other reload", step, otherAsk, otherAgainAsk)
		assertChoiceInvariants(t, step, act.Type, otherAsk)
		if !sameButtonOrder(ask, otherAsk) {
			differed = true
		}
		assertChoiceInvariants(t, step, act.Type, ask)
		lessonAsks = append(lessonAsks, ask)

		if step == 0 {
			distractor := ask.distractorValue()
			gradeChoice(t, primary, path, ask.runID, distractor, "feedback--wrong")
			gradedWrong = true
		} else {
			gradeChoice(t, primary, path, ask.runID, ask.targetID, "feedback--right")
			gradedRight = true
		}
		advanceChoice(t, primary, path, ask.runID)
		gradeChoice(t, other, path, otherAsk.runID, otherAsk.targetID, "feedback--right")
		advanceChoice(t, other, path, otherAsk.runID)
	}
	if !differed {
		t.Fatal("a second run id did not change button order or distractors for any target")
	}
	if !gradedRight || !gradedWrong {
		t.Fatal("grading did not see both a correct button and a distractor")
	}
	assertChoiceSpread(t, "lesson", lessonAsks)

	var checkAsks []qaAsk
	for step, act := range check.Activities {
		first := getAuthed(t, fx.h, fx.cookie, checkPath)
		again := getAuthed(t, fx.h, fx.cookie, checkPath)
		ask := qaReadAsk(t, first, act, byText)
		againAsk := qaReadAsk(t, again, act, byText)
		if ask.runID != againAsk.runID {
			t.Fatalf("checkpoint step %d run id changed on re-render: %s then %s", step, ask.runID, againAsk.runID)
		}
		assertSameButtons(t, "checkpoint reload", step, ask, againAsk)
		assertChoiceInvariants(t, step, act.Type, ask)
		checkAsks = append(checkAsks, ask)
		gradeChoice(t, primary, checkPath, ask.runID, ask.targetID, "feedback--right")
		advanceChoice(t, primary, checkPath, ask.runID)
	}
	assertChoiceSpread(t, "checkpoint", checkAsks)
}

type qaLexeme struct {
	Text     string `json:"text"`
	Gloss    string `json:"gloss"`
	Translit string `json:"translit"`
	POS      string `json:"pos"`
	Gender   string `json:"gender"`
	ID       string `json:"-"`
}

type qaChoiceDoc struct {
	Lexemes []qaLexeme `json:"lexemes"`
	Unit    units.Unit `json:"unit"`
	UnitID  int64      `json:"-"`
}

type qaButton struct {
	value    string
	text     string
	attrKey  string
	skeleton string
}

type qaAsk struct {
	runID       string
	targetID    string
	targetText  string
	wantLabel   string
	pos         int
	values      []string
	texts       []string
	distractors []string
	buttons     []qaButton
}

func (a qaAsk) distractorValue() string {
	for _, value := range a.values {
		if value != a.targetID {
			return value
		}
	}
	return ""
}

type qaClient struct {
	h      http.Handler
	cookie *http.Cookie
	csrf   string
}

func loadQAChoiceUnit(t *testing.T, fx lessonFixture) qaChoiceDoc {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "web", "testdata", "qa_choice_unit.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc qaChoiceDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Unit.Lessons) != 2 {
		t.Fatalf("fixture lessons = %d", len(doc.Unit.Lessons))
	}
	inputs := make([]lexicon.Input, len(doc.Lexemes))
	for i, lex := range doc.Lexemes {
		gloss, translit, pos, gender := lex.Gloss, lex.Translit, lex.POS, lex.Gender
		inputs[i] = lexicon.Input{
			TextNFC:  lex.Text,
			Gloss:    gloss,
			Translit: &translit,
			POS:      &pos,
			Gender:   &gender,
		}
	}
	saved, _, err := fx.store.UpsertLexemes(fx.ctx, inputs)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, row := range saved {
		ids[row.TextNFC] = itoa(row.ID)
	}
	for i := range doc.Lexemes {
		id, ok := ids[doc.Lexemes[i].Text]
		if !ok || id == "" {
			t.Fatalf("lexeme %s was not stored", doc.Lexemes[i].Text)
		}
		doc.Lexemes[i].ID = id
	}
	doc.Unit.Position = 0
	doc.UnitID = putLesson(t, fx.store, nil, doc.Unit, "shared")
	publish(t, fx.store, doc.UnitID)
	return doc
}

func qaSecondLearner(t *testing.T, fx lessonFixture) qaClient {
	t.Helper()
	user, err := fx.store.CreateUser(fx.ctx, "quinn@example.com", "sub-quinn", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	const csrf = "quinn-csrf"
	return qaClient{
		h:      fx.h,
		cookie: mintSession(t, fx.store, user.ID, csrf, fx.now.Add(time.Hour)),
		csrf:   csrf,
	}
}

func assertGradedTypes(t *testing.T, acts []units.Activity) {
	t.Helper()
	seen := map[string]bool{}
	for _, act := range acts {
		seen[act.Type] = true
	}
	for _, typ := range []string{
		units.ActivityRecall,
		units.ActivityListenPick,
		units.ActivityProduce,
		units.ActivityCloze,
	} {
		if !seen[typ] {
			t.Fatalf("activities %v omit %s", acts, typ)
		}
	}
}

func qaReadAsk(t *testing.T, body string, act units.Activity, byText map[string]qaLexeme) qaAsk {
	t.Helper()
	lex, ok := byText[act.Text]
	if !ok {
		t.Fatalf("activity %s text %s is not a fixture lexeme", act.Type, act.Text)
	}
	prompt := qaPrompt(act.Type)
	if prompt == "" || !strings.Contains(body, prompt) {
		t.Fatalf("%s prompt %q missing:\n%s", act.Type, prompt, choiceRegion(body))
	}
	buttons := parseChoiceButtons(t, body)
	values := make([]string, len(buttons))
	texts := make([]string, len(buttons))
	for i, button := range buttons {
		values[i] = button.value
		texts[i] = button.text
	}
	pos := slices.Index(values, lex.ID)
	var distractors []string
	for _, value := range values {
		if value != lex.ID {
			distractors = append(distractors, value)
		}
	}
	slices.Sort(distractors)
	return qaAsk{
		runID:       runValue(t, body),
		targetID:    lex.ID,
		targetText:  lex.Text,
		wantLabel:   wantChoiceLabel(act, lex),
		pos:         pos,
		values:      values,
		texts:       texts,
		distractors: distractors,
		buttons:     buttons,
	}
}

func qaPrompt(typ string) string {
	switch typ {
	case units.ActivityRecall:
		return "What does this Hebrew mean?"
	case units.ActivityListenPick:
		return "Listen, then pick the meaning."
	case units.ActivityProduce:
		return "Pick the Hebrew"
	case units.ActivityCloze:
		return "Pick the missing word."
	default:
		return ""
	}
}

func wantChoiceLabel(act units.Activity, lex qaLexeme) string {
	switch act.Type {
	case units.ActivityProduce:
		return lex.Text
	case units.ActivityCloze:
		return lex.Text + " " + lex.Translit
	default:
		return lex.Gloss
	}
}

func assertSameButtons(t *testing.T, label string, step int, a, b qaAsk) {
	t.Helper()
	if !sameButtonOrder(a, b) {
		t.Fatalf("%s step %d buttons %v %q then %v %q", label, step, a.values, a.texts, b.values, b.texts)
	}
}

func sameButtonOrder(a, b qaAsk) bool {
	return slices.Equal(a.values, b.values) && slices.Equal(a.texts, b.texts)
}

func assertChoiceInvariants(t *testing.T, step int, typ string, ask qaAsk) {
	t.Helper()
	if len(ask.buttons) != 3 {
		t.Fatalf("%s step %d has %d choice buttons, want 3: %v", typ, step, len(ask.buttons), ask.values)
	}
	if ask.pos < 0 || countValue(ask.values, ask.targetID) != 1 {
		t.Fatalf("%s step %d target %s (%s) appears %d times in %v", typ, step, ask.targetText, ask.targetID, countValue(ask.values, ask.targetID), ask.values)
	}
	if repeated(ask.values) {
		t.Fatalf("%s step %d repeats a lexeme id: %v", typ, step, ask.values)
	}
	if repeated(ask.texts) {
		t.Fatalf("%s step %d repeats visible text: %q", typ, step, ask.texts)
	}
	if ask.buttons[ask.pos].text != ask.wantLabel {
		t.Fatalf("%s step %d target button text %q, want %q", typ, step, ask.buttons[ask.pos].text, ask.wantLabel)
	}
	labelHits := 0
	for _, text := range ask.texts {
		if text == ask.wantLabel {
			labelHits++
		}
	}
	if labelHits != 1 {
		t.Fatalf("%s step %d target label %q appears %d times in %q", typ, step, ask.wantLabel, labelHits, ask.texts)
	}
	key := ask.buttons[0].attrKey
	skeleton := ask.buttons[0].skeleton
	for i, button := range ask.buttons {
		if button.attrKey != key {
			t.Fatalf("%s step %d button %d attributes differ from the first\n%s\n%s", typ, step, i, key, button.attrKey)
		}
		if button.skeleton != skeleton {
			t.Fatalf("%s step %d button %d markup differs apart from value and label\n%s\n%s", typ, step, i, skeleton, button.skeleton)
		}
	}
}

func assertChoiceSpread(t *testing.T, label string, asks []qaAsk) {
	t.Helper()
	if len(asks) < 5 {
		t.Fatalf("%s rendered %d targets", label, len(asks))
	}
	posCount := map[int]int{}
	sets := map[string]int{}
	targets := map[string]string{}
	for _, ask := range asks {
		posCount[ask.pos]++
		sets[strings.Join(ask.distractors, ",")]++
		targets[ask.targetID] = ask.targetText
	}
	if len(posCount) < 2 {
		t.Fatalf("%s correct position is constant: %v", label, posCount)
	}
	for pos, n := range posCount {
		if n == len(asks) {
			t.Fatalf("%s position %d holds every correct answer (%d)", label, pos, n)
		}
	}
	if len(sets) < 2 {
		t.Fatalf("%s every target shares one distractor set: %v", label, sets)
	}
	for id, text := range targets {
		found := false
		for _, ask := range asks {
			if ask.targetID == id {
				continue
			}
			if slices.Contains(ask.distractors, id) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s target %s (%s) is absent from every other target's distractors", label, text, id)
		}
	}
}

func gradeChoice(t *testing.T, client qaClient, path, runID, answer, wantClass string) {
	t.Helper()
	res := postLesson(t, client.h, client.cookie, path, runID, answer, true, client.csrf)
	body := res.Body.String()
	if res.Code != http.StatusOK || !strings.Contains(body, wantClass) {
		t.Fatalf("grade %s = %d, want %s\n%s", answer, res.Code, wantClass, body)
	}
	other := "feedback--right"
	if wantClass == "feedback--right" {
		other = "feedback--wrong"
	}
	if strings.Contains(body, other) {
		t.Fatalf("grade %s includes %s\n%s", answer, other, body)
	}
}

func advanceChoice(t *testing.T, client qaClient, path, runID string) {
	t.Helper()
	res := postLesson(t, client.h, client.cookie, path, runID, "continue", true, client.csrf)
	if res.Code != http.StatusOK {
		t.Fatalf("continue = %d %s", res.Code, res.Body.String())
	}
}

func parseChoiceButtons(t *testing.T, body string) []qaButton {
	t.Helper()
	block := choiceRegion(body)
	var out []qaButton
	rest := block
	for {
		i := strings.Index(rest, "<button")
		if i < 0 {
			break
		}
		rest = rest[i:]
		endTag := strings.Index(rest, "</button>")
		if endTag < 0 {
			t.Fatalf("unclosed choice button:\n%s", rest)
		}
		full := rest[:endTag+len("</button>")]
		gt := indexTagEnd(full)
		if gt < 0 {
			t.Fatalf("choice button tag:\n%s", full)
		}
		attrs := parseTagAttrs(full[:gt+1])
		inner := full[gt+1 : endTag]
		out = append(out, qaButton{
			value:    attrs["value"],
			text:     visibleText(inner),
			attrKey:  attrKey(attrs),
			skeleton: buttonSkeleton(full),
		})
		rest = rest[endTag+len("</button>"):]
	}
	if len(out) == 0 {
		t.Fatalf("no choice buttons:\n%s", block)
	}
	return out
}

func choiceRegion(body string) string {
	const open = `<div class="choices">`
	i := strings.Index(body, open)
	if i < 0 {
		return body
	}
	rest := body[i:]
	j := strings.Index(rest, "</div>")
	if j < 0 {
		return rest
	}
	return rest[:j]
}

func indexTagEnd(s string) int {
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		switch {
		case quote != 0:
			if s[i] == quote {
				quote = 0
			}
		case s[i] == '"' || s[i] == '\'':
			quote = s[i]
		case s[i] == '>':
			return i
		}
	}
	return -1
}

var (
	attrRE     = regexp.MustCompile(`([^\s=]+)="([^"]*)"`)
	textNodeRE = regexp.MustCompile(`>([^<]*)<`)
	tagRE      = regexp.MustCompile(`<[^>]*>`)
)

func parseTagAttrs(tag string) map[string]string {
	out := map[string]string{}
	for _, m := range attrRE.FindAllStringSubmatch(tag, -1) {
		out[m[1]] = html.UnescapeString(m[2])
	}
	return out
}

func attrKey(attrs map[string]string) string {
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		if key == "value" {
			continue
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var b strings.Builder
	for _, key := range keys {
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(attrs[key])
		b.WriteByte(';')
	}
	return b.String()
}

func buttonSkeleton(full string) string {
	masked := attrRE.ReplaceAllStringFunc(full, func(match string) string {
		m := attrRE.FindStringSubmatch(match)
		if m[1] == "value" {
			return m[1] + `="#"`
		}
		return match
	})
	return textNodeRE.ReplaceAllStringFunc(masked, func(match string) string {
		inner := match[1 : len(match)-1]
		if strings.TrimSpace(inner) == "" {
			return match
		}
		return ">#<"
	})
}

func visibleText(inner string) string {
	plain := tagRE.ReplaceAllString(inner, " ")
	return strings.Join(strings.Fields(html.UnescapeString(plain)), " ")
}
