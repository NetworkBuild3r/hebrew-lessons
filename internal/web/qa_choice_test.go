package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
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
	return loadQAChoiceFile(t, fx, "qa_choice_unit.json")
}

func loadQAChoiceFile(t *testing.T, fx lessonFixture, name string) qaChoiceDoc {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "web", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var doc qaChoiceDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Unit.Lessons) < 1 {
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
	lex, ok := qaActivityLexeme(act, byText)
	if !ok {
		t.Fatalf("activity %s text %s target %s is not a fixture lexeme", act.Type, act.Text, act.Target)
	}
	if !qaSawPrompt(act.Type, body, lex.Text) {
		t.Fatalf("%s prompt missing for %s:\n%s", act.Type, lex.Text, choiceRegion(body))
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
		wantLabel:   wantChoiceLabelIn(body, act, lex),
		pos:         pos,
		values:      values,
		texts:       texts,
		distractors: distractors,
		buttons:     buttons,
	}
}

func qaActivityLexeme(act units.Activity, byText map[string]qaLexeme) (qaLexeme, bool) {
	if lex, ok := byText[act.Text]; ok {
		return lex, true
	}
	tok := act.Target
	if tok == "" {
		tokens := units.Tokenize(act.Text)
		if len(tokens) == 0 {
			return qaLexeme{}, false
		}
		tok = tokens[0]
	}
	lex, ok := byText[tok]
	return lex, ok
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

func qaSawPrompt(typ, body, target string) bool {
	switch typ {
	case units.ActivityRecall:
		if strings.Contains(body, qaPrompt(typ)) {
			return true
		}
		return target != "" && strings.Contains(body, "What does ") && strings.Contains(body, target) && strings.Contains(body, " mean?")
	case units.ActivityListenPick:
		// A listening step asks what was heard, or the meaning when the lesson has too few
		// other words to make wrong lines.
		return strings.Contains(body, listenAskPrompt) || strings.Contains(body, listenMeaningPrompt)
	default:
		prompt := qaPrompt(typ)
		return prompt != "" && strings.Contains(body, prompt)
	}
}

// wantChoiceLabelIn is the label of the right button. A listening step that asks what was
// heard has the heard line on it.
func wantChoiceLabelIn(body string, act units.Activity, lex qaLexeme) string {
	if act.Type == units.ActivityListenPick && strings.Contains(body, listenAskPrompt) {
		return units.Normalize(act.Text)
	}
	return wantChoiceLabel(act, lex)
}

func wantChoiceLabel(act units.Activity, lex qaLexeme) string {
	switch act.Type {
	case units.ActivityProduce:
		return lex.Text
	case units.ActivityCloze:
		return lex.Text
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
	// Recall and cloze aim for four. listen_pick and produce stay at three.
	// The floor is three: two eligible options are a reveal, not an ask.
	if len(ask.buttons) < 3 {
		t.Fatalf("%s step %d has %d choice buttons, want at least 3: %v", typ, step, len(ask.buttons), ask.values)
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

// TestQAChoiceFloor is the two-option pool. The unit has the target and one
// other lexeme, so a recall or a cloze can fill exactly two buttons. Three is
// the floor, so that page is a reveal: no choice buttons, and posting the
// target id or Continue records no attempt.
func TestQAChoiceFloor(t *testing.T) {
	fx := openLessonPlayer(t)
	emails := []string{"brian@example.com", "qa-floor-a@example.com", "qa-floor-b@example.com"}
	fx.h = qaHandler(t, fx, emails)
	doc := loadQAChoiceFile(t, fx, "qa_choice_two.json")
	if len(doc.Lexemes) != 2 {
		t.Fatalf("two-option fixture has %d lexemes, want 2", len(doc.Lexemes))
	}
	left := qaNormGloss(doc.Lexemes[0].Gloss)
	right := qaNormGloss(doc.Lexemes[1].Gloss)
	if left == "" || right == "" || left == right {
		t.Fatalf("two-option fixture glosses %q and %q are not two distinct options", doc.Lexemes[0].Gloss, doc.Lexemes[1].Gloss)
	}
	lesson := doc.Unit.Lessons[0]
	seen := map[string]bool{}
	for _, act := range lesson.Activities {
		seen[act.Type] = true
	}
	if !seen[units.ActivityRecall] || !seen[units.ActivityCloze] {
		t.Fatalf("two-option fixture activities = %+v", lesson.Activities)
	}
	byText := qaLexemeIndex(doc)
	type floorLearner struct {
		client qaClient
		userID int64
	}
	learners := []floorLearner{{
		client: qaClient{h: fx.h, cookie: fx.cookie, csrf: lessonCSRF},
		userID: fx.userID,
	}}
	for i, email := range emails[1:] {
		client := qaNamedLearner(t, fx, email, fmt.Sprintf("floor-%d-csrf", i))
		user, err := fx.store.UserByEmail(fx.ctx, email)
		if err != nil {
			t.Fatal(err)
		}
		learners = append(learners, floorLearner{client: client, userID: user.ID})
	}
	path := "/lesson/" + itoa(doc.UnitID) + "/0"
	runs := map[string]bool{}
	for li, learner := range learners {
		client := learner.client
		for step, act := range lesson.Activities {
			body := getAuthed(t, client.h, client.cookie, path)
			if !qaSawPrompt(act.Type, body, act.Text) {
				t.Fatalf("learner %d step %d is not %s %s", li, step, act.Type, act.Text)
			}
			runID := runValue(t, body)
			runs[runID] = true
			if strings.Contains(body, `<div class="choices">`) {
				t.Fatalf("learner %d %s step %d rendered choices; two eligible options are a reveal", li, act.Type, step)
			}
			if !strings.Contains(body, `value="continue"`) {
				t.Fatalf("learner %d %s step %d reveal has no Continue", li, act.Type, step)
			}
			lex, ok := qaActivityLexeme(act, byText)
			if !ok {
				t.Fatalf("learner %d step %d target %s is missing", li, step, act.Text)
			}
			posted := postLesson(t, client.h, client.cookie, path, runID, lex.ID, true, client.csrf)
			if posted.Code != http.StatusOK {
				t.Fatalf("learner %d %s step %d post id: %d\n%s", li, act.Type, step, posted.Code, posted.Body.String())
			}
			got := posted.Body.String()
			if strings.Contains(got, "feedback--right") || strings.Contains(got, `class="choices"`) {
				t.Fatalf("learner %d %s step %d graded a reveal:\n%s", li, act.Type, step, got)
			}
			if n := countRows(t, fx.store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, learner.userID); n != 0 {
				t.Fatalf("learner %d %s step %d recorded %d attempts after posting the target", li, act.Type, step, n)
			}
			advanceChoice(t, client, path, runID)
			if n := countRows(t, fx.store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, learner.userID); n != 0 {
				t.Fatalf("learner %d %s step %d recorded %d attempts after Continue", li, act.Type, step, n)
			}
		}
	}
	if len(runs) < len(learners) {
		t.Fatalf("two-option learners shared runs: %d runs for %d learners", len(runs), len(learners))
	}
}

// TestQAChoicePromptGlosses checks recall and cloze distractors against the
// glosses a learner can already see or hear. A multi-word line shows the
// other words' glosses on a recall ask, and a clip's stored text resolves to
// those glosses when the ask plays it. Comparison folds case and whitespace.
func TestQAChoicePromptGlosses(t *testing.T) {
	fx := openLessonPlayer(t)
	emails := []string{"brian@example.com", "qa-gloss-a@example.com", "qa-gloss-b@example.com"}
	fx.h = qaHandler(t, fx, emails)
	doc := loadQAChoiceFile(t, fx, "qa_choice_prompt.json")
	if len(doc.Unit.Lessons) < 1 || len(doc.Unit.Lessons[0].Activities) < 4 {
		t.Fatalf("prompt fixture has %d lessons", len(doc.Unit.Lessons))
	}
	lesson := doc.Unit.Lessons[0]
	assertPromptDecoys(t, fx, doc, lesson.Activities)
	qaInsertPromptClips(t, fx, lesson.Activities)
	byText := qaLexemeIndex(doc)
	byID := map[string]qaLexeme{}
	for _, lex := range doc.Lexemes {
		byID[lex.ID] = lex
	}
	clients := []qaClient{{h: fx.h, cookie: fx.cookie, csrf: lessonCSRF}}
	clients = append(clients,
		qaNamedLearner(t, fx, emails[1], "gloss-a-csrf"),
		qaNamedLearner(t, fx, emails[2], "gloss-b-csrf"),
	)
	path := "/lesson/" + itoa(doc.UnitID) + "/0"
	runs := map[string]bool{}
	seen := map[string]int{}
	for _, client := range clients {
		for step, act := range lesson.Activities {
			body := getAuthed(t, client.h, client.cookie, path)
			ask := qaReadAsk(t, body, act, byText)
			if ask.runID != "" {
				runs[ask.runID] = true
			}
			seen[act.Type]++
			assertChoiceInvariants(t, step, act.Type, ask)
			assertPromptGlosses(t, step, act, body, ask, byText, byID, fx)
			gradeChoice(t, client, path, ask.runID, ask.targetID, "feedback--right")
			advanceChoice(t, client, path, ask.runID)
		}
	}
	if seen[units.ActivityRecall] < len(clients) || seen[units.ActivityCloze] < len(clients) {
		t.Fatalf("prompt coverage = %v", seen)
	}
	if len(runs) < len(clients) {
		t.Fatalf("prompt learners shared runs: %v", runs)
	}
}

// assertPromptDecoys requires every gloss twin to be a unit word, and each
// twin to sit on a line the other twin is not on. A word that is only in
// newWords never reaches UnitLexemes, so it cannot be a distractor.
func assertPromptDecoys(t *testing.T, fx lessonFixture, doc qaChoiceDoc, acts []units.Activity) {
	t.Helper()
	rows, err := fx.store.UnitLexemes(fx.ctx, doc.UnitID)
	if err != nil {
		t.Fatal(err)
	}
	inUnit := map[string]bool{}
	for _, row := range rows {
		inUnit[row.Text] = true
	}
	byGloss := map[string][]string{}
	for _, lex := range doc.Lexemes {
		gloss := qaNormGloss(lex.Gloss)
		if gloss == "" {
			t.Fatalf("lexeme %s has no gloss", lex.Text)
		}
		byGloss[gloss] = append(byGloss[gloss], lex.Text)
	}
	decoys := 0
	for gloss, texts := range byGloss {
		if len(texts) < 2 {
			continue
		}
		for _, text := range texts {
			if !inUnit[text] {
				t.Fatalf("decoy %s (gloss %q) is not in the unit words", text, gloss)
			}
			decoys++
		}
	}
	if decoys < 2 {
		t.Fatal("prompt fixture has no gloss decoy in the unit words")
	}
	outside := map[string]bool{}
	for _, act := range acts {
		onLine := map[string]bool{}
		for _, tok := range units.Tokenize(act.Text) {
			onLine[tok] = true
		}
		for gloss, texts := range byGloss {
			if len(texts) < 2 {
				continue
			}
			var present, absent []string
			for _, text := range texts {
				if onLine[text] {
					present = append(present, text)
				} else {
					absent = append(absent, text)
				}
			}
			if len(present) == 0 || len(absent) == 0 {
				continue
			}
			for _, text := range absent {
				outside[text] = true
				if !inUnit[text] {
					t.Fatalf("%s step gloss %q decoy %s is not a unit word", act.Type, gloss, text)
				}
			}
		}
	}
	for gloss, texts := range byGloss {
		if len(texts) < 2 {
			continue
		}
		for _, text := range texts {
			if !outside[text] {
				t.Fatalf("decoy %s (gloss %q) is never outside a line that shows that gloss", text, gloss)
			}
		}
	}
}

func qaLexemeIndex(doc qaChoiceDoc) map[string]qaLexeme {
	byText := map[string]qaLexeme{}
	for _, lex := range doc.Lexemes {
		byText[lex.Text] = lex
	}
	return byText
}

func qaHandler(t *testing.T, fx lessonFixture, emails []string) http.Handler {
	t.Helper()
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         fixedClock(fx.now),
		Store:         fx.store,
		AllowedEmails: emails,
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewOptions(nil, svc, Options{Store: fx.store})
}

func qaNamedLearner(t *testing.T, fx lessonFixture, email, csrf string) qaClient {
	t.Helper()
	user, err := fx.store.CreateUser(fx.ctx, email, "sub-"+email, auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	return qaClient{
		h:      fx.h,
		cookie: mintSession(t, fx.store, user.ID, csrf, fx.now.Add(time.Hour)),
		csrf:   csrf,
	}
}

func qaInsertPromptClips(t *testing.T, fx lessonFixture, acts []units.Activity) {
	t.Helper()
	seen := map[string]bool{}
	var texts []string
	add := func(text string) {
		text = units.Normalize(text)
		if text == "" || seen[text] {
			return
		}
		seen[text] = true
		texts = append(texts, text)
	}
	for _, act := range acts {
		add(act.Text)
		for _, tok := range units.Tokenize(act.Text) {
			add(tok)
		}
	}
	for i, text := range texts {
		id := fmt.Sprintf("%064x", i+1)
		inserted, err := fx.store.InsertAudio(fx.ctx, db.Asset{
			ID: id, TextNFC: text, Voice: "voice", Model: "eleven_v4", Bytes: 8,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !inserted {
			t.Fatalf("clip %s for %s was already stored", id, text)
		}
	}
}

func assertPromptGlosses(t *testing.T, step int, act units.Activity, body string, ask qaAsk, byText map[string]qaLexeme, byID map[string]qaLexeme, fx lessonFixture) {
	t.Helper()
	var help []string
	for _, tok := range units.Tokenize(act.Text) {
		if tok == ask.targetText {
			continue
		}
		lex, ok := byText[tok]
		if !ok {
			continue
		}
		gloss := qaNormGloss(lex.Gloss)
		if gloss == "" {
			t.Fatalf("%s step %d other word %s has no gloss", act.Type, step, tok)
		}
		help = append(help, gloss)
	}
	if len(help) == 0 {
		t.Fatalf("%s step %d has no other word on the line", act.Type, step)
	}
	prompt := qaPromptRegion(t, body)
	visible := qaVisibleGlosses(prompt)
	heard := qaHeardGlosses(t, fx, prompt)
	for _, gloss := range help {
		switch act.Type {
		case units.ActivityRecall:
			if !visible[gloss] {
				t.Fatalf("recall step %d hides the other word's gloss %q; visible %v", step, gloss, visible)
			}
			if !heard[gloss] {
				t.Fatalf("recall step %d does not play the other word's gloss %q; heard %v", step, gloss, heard)
			}
		case units.ActivityCloze:
			if !heard[gloss] {
				t.Fatalf("cloze step %d does not play the other word's gloss %q; heard %v", step, gloss, heard)
			}
		default:
			t.Fatalf("prompt gloss check is for recall and cloze, got %s", act.Type)
		}
	}
	forbidden := map[string]bool{}
	for gloss := range visible {
		forbidden[gloss] = true
	}
	for gloss := range heard {
		forbidden[gloss] = true
	}
	if len(forbidden) == 0 {
		t.Fatalf("%s step %d prompt exposes no gloss", act.Type, step)
	}
	for _, button := range ask.buttons {
		if button.value == ask.targetID {
			continue
		}
		lex, ok := byID[button.value]
		if !ok {
			t.Fatalf("%s step %d distractor %s is not in the fixture", act.Type, step, button.value)
		}
		for _, candidate := range []string{qaNormGloss(lex.Gloss), qaNormGloss(button.text)} {
			if candidate == "" || !forbidden[candidate] {
				continue
			}
			t.Fatalf("%s step %d distractor %s (%q / %q) matches a prompt gloss %q\nvisible %v\nheard %v",
				act.Type, step, button.value, lex.Gloss, button.text, candidate, visible, heard)
		}
	}
}

func qaPromptRegion(t *testing.T, body string) string {
	t.Helper()
	act := activityHTML(t, body)
	if i := strings.Index(act, `<div class="choices">`); i >= 0 {
		return act[:i]
	}
	return act
}

func qaVisibleGlosses(prompt string) map[string]bool {
	out := map[string]bool{}
	for _, m := range qaGlossRE.FindAllStringSubmatch(prompt, -1) {
		if gloss := qaNormGloss(m[1]); gloss != "" {
			out[gloss] = true
		}
	}
	return out
}

func qaHeardGlosses(t *testing.T, fx lessonFixture, prompt string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	ids := qaAudioRE.FindAllStringSubmatch(prompt, -1)
	if len(ids) == 0 {
		t.Fatalf("prompt has no clip:\n%s", prompt)
	}
	seen := map[string]bool{}
	for _, m := range ids {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		text := qaClipText(t, fx, m[1])
		add := func(token string) {
			if gloss := qaNormGloss(qaStoredGloss(t, fx, token)); gloss != "" {
				out[gloss] = true
			}
		}
		add(text)
		for _, tok := range units.Tokenize(text) {
			add(tok)
		}
	}
	return out
}

func qaClipText(t *testing.T, fx lessonFixture, id string) string {
	t.Helper()
	conn, err := fx.store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var text string
	err = conn.QueryRowContext(fx.ctx, `SELECT text_nfc FROM audio_assets WHERE id = ?`, id).Scan(&text)
	if err != nil {
		t.Fatalf("clip %s text: %v", id, err)
	}
	if strings.TrimSpace(text) == "" {
		t.Fatalf("clip %s has no text", id)
	}
	return text
}

func qaStoredGloss(t *testing.T, fx lessonFixture, text string) string {
	t.Helper()
	conn, err := fx.store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var gloss string
	err = conn.QueryRowContext(fx.ctx, `SELECT gloss_en FROM lexemes WHERE text_nfc = ?`, units.Normalize(text)).Scan(&gloss)
	if errors.Is(err, sql.ErrNoRows) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return gloss
}

func qaNormGloss(gloss string) string {
	return strings.ToLower(strings.Join(strings.Fields(html.UnescapeString(gloss)), " "))
}

var (
	qaGlossRE = regexp.MustCompile(`(?s)<bdi class="[^"]*\bwb__gl\b[^"]*"[^>]*>(.*?)</bdi>`)
	qaAudioRE = regexp.MustCompile(`/audio/([0-9a-fA-F]{64})\.mp3`)
)
