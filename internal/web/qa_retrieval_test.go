package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// Retrieval paths that exist hide the answer string the product already
// withholds. Multiple-choice lessons still print the correct option, and
// there is no practice tool on MCP. Those stays are skips: the UI and the
// tool are not this PR's to add.
func TestQARetrievalBeforeReveal(t *testing.T) {
	t.Run("mcp practice item", func(t *testing.T) {
		t.Skip("needs practice item tool: owner MCP Builder")
	})
	t.Run("typed answer", func(t *testing.T) {
		t.Skip("needs a typed-answer practice handler: owner Brian (North Star UI)")
	})
	t.Run("recall omits gloss", func(t *testing.T) {
		t.Skip("needs a recall response that omits the gloss before the attempt: owner Brian (North Star UI)")
	})
	t.Run("listen_pick omits gloss", func(t *testing.T) {
		t.Skip("needs a listen_pick response that omits the gloss before the attempt: owner Brian (North Star UI)")
	})
	t.Run("produce omits Hebrew", func(t *testing.T) {
		t.Skip("needs an en→he produce response that omits the Hebrew before the attempt: owner Brian (North Star UI)")
	})
	t.Run("cloze omits Hebrew option", func(t *testing.T) {
		t.Skip("needs a cloze response that omits the gapped Hebrew before the attempt: owner Brian (North Star UI)")
	})
	t.Run("checkpoint omits answer choice", func(t *testing.T) {
		t.Skip("needs a checkpoint response that omits the answer choice before the attempt: owner Brian (North Star UI)")
	})

	t.Run("review", func(t *testing.T) {
		testQAReviewHidesAnswer(t)
	})
	t.Run("lesson and checkpoint", func(t *testing.T) {
		testQALessonHidesAnswer(t)
	})
	t.Run("placement", func(t *testing.T) {
		testQAPlacementHidesGloss(t)
	})
}

func testQAReviewHidesAnswer(t *testing.T) {
	t.Helper()
	const (
		heGloss = "zephyrmail"
		heText  = "צָב"
		heLat   = "xqstav"
		enGloss = "quartzbill"
		enText  = "דָּג"
		enLat   = "qdag"
	)
	store, h, now := openApp(t)
	reviewed := now.Add(-48 * time.Hour)

	heLearner := addLearner(t, store, "a@example.com", "sub-a", now)
	heLex := addLexeme(t, store, heText, heGloss, heLat, "m", "")
	heCard := addCard(t, store, heLearner.user.ID, heLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	front := getLearner(t, h, heLearner, "/review")
	if strings.Contains(front, heGloss) {
		t.Fatalf("he→en review shows the gloss before the attempt:\n%s", front)
	}
	if !strings.Contains(front, heText) {
		t.Fatalf("he→en review hid the Hebrew prompt:\n%s", front)
	}
	early := postGrade(t, h, heLearner, heCard, "3", "https://learn.example")
	if early.Code != http.StatusBadRequest || strings.Contains(early.Body.String(), heGloss) {
		t.Fatalf("grade without reveal = %d %s", early.Code, early.Body.String())
	}
	shown := postReveal(t, h, heLearner, heCard, "https://learn.example")
	if shown.Code != http.StatusOK || !strings.Contains(shown.Body.String(), heGloss) {
		t.Fatalf("reveal did not show the gloss: %d %s", shown.Code, shown.Body.String())
	}

	enLearner := addLearner(t, store, "en@example.com", "sub-en", now)
	enLex := addLexeme(t, store, enText, enGloss, enLat, "m", "")
	enCard := addCard(t, store, enLearner.user.ID, enLex, fsrs.SkillRecall, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	prod := getLearner(t, h, enLearner, "/review")
	if strings.Contains(prod, enText) || strings.Contains(prod, enLat) {
		t.Fatalf("en→he review shows the Hebrew before the attempt:\n%s", prod)
	}
	if !strings.Contains(prod, enGloss) {
		t.Fatalf("en→he review hid the English prompt:\n%s", prod)
	}
	back := postReveal(t, h, enLearner, enCard, "https://learn.example")
	if back.Code != http.StatusOK || !strings.Contains(back.Body.String(), enText) {
		t.Fatalf("reveal did not show the Hebrew: %d %s", back.Code, back.Body.String())
	}
}

func testQALessonHidesAnswer(t *testing.T) {
	t.Helper()
	const (
		promptHe   = "צָב"
		promptGl   = "zephyrmail"
		promptLat  = "xqstav"
		produceHe  = "דָּג"
		produceGl  = "quartzbill"
		produceLat = "qdag"
		gapHe      = "לֶקֶס"
		gapGl      = "lumenwort"
		gapLat     = "xlumen"
	)
	// Outside every activity line, so recall and cloze stay graded.
	// Fewer than three buttons is a reveal (#105), and a reveal shows the gloss.
	outHe := [3]string{"נֵר", "גֶּשֶׁר", "דֶּלֶת"}
	outGl := [3]string{"lampwick", "stonebridge", "oakdoor"}
	outLat := [3]string{"xner", "xgesher", "xdelet"}
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := t.Context()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.CreateUser(ctx, "a@example.com", "sub-a", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	promptID := addLexeme(t, store, promptHe, promptGl, promptLat, "m", "")
	produceID := addLexeme(t, store, produceHe, produceGl, produceLat, "m", "")
	gapID := addLexeme(t, store, gapHe, gapGl, gapLat, "m", "")
	for i := range outHe {
		addLexeme(t, store, outHe[i], outGl[i], outLat[i], "m", "")
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Probe", Situation: "A probe.", Concept: "Probe concept.", Position: 0,
		NewWords: []string{promptHe, produceHe, gapHe, outHe[0], outHe[1], outHe[2]},
		Lessons: []units.Lesson{
			{Kind: units.KindLesson, Activities: []units.Activity{
				// The outside words have to appear in an activity or they are
				// not lesson items, and the cloze then has no pool to grade from.
				{Type: units.ActivityListenPick, Text: promptHe + " " + outHe[0] + " " + outHe[1] + " " + outHe[2]},
				{Type: units.ActivityRecall, Text: promptHe},
				{Type: units.ActivityProduce, Text: produceHe},
				{Type: units.ActivityCloze, Text: gapHe + " " + promptHe},
			}},
			{Kind: units.KindCheckpoint, Activities: []units.Activity{
				{Type: units.ActivityRecall, Text: promptHe},
				{Type: units.ActivityProduce, Text: produceHe},
			}},
		},
	}, "shared")
	publish(t, store, unitID)
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: []string{"a@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	cookie := mintSession(t, store, user.ID, "qa-csrf", now.Add(time.Hour))
	lessonPath := "/lesson/" + itoa(unitID) + "/0"
	checkPath := "/lesson/" + itoa(unitID) + "/1"

	listen := getAuthed(t, h, cookie, "/lesson")
	runID := runValue(t, listen)
	for _, hidden := range []string{promptHe, produceHe, gapHe, promptLat, produceLat, gapLat} {
		if strings.Contains(listen, hidden) {
			t.Fatalf("listen_pick shows %q before the attempt:\n%s", hidden, listen)
		}
	}
	if !strings.Contains(listen, promptGl) {
		t.Fatalf("listen_pick did not render the gloss choices:\n%s", listen)
	}
	listenAnswer := postLesson(t, h, cookie, lessonPath, runID, itoa(promptID), true, "qa-csrf")
	if listenAnswer.Code != http.StatusOK || !strings.Contains(listenAnswer.Body.String(), promptHe) || !strings.Contains(listenAnswer.Body.String(), promptLat) {
		t.Fatalf("listen_pick answer = %d %s", listenAnswer.Code, listenAnswer.Body.String())
	}

	recall := postLesson(t, h, cookie, lessonPath, runID, "continue", true, "qa-csrf")
	recallBody := recall.Body.String()
	if recall.Code != http.StatusOK || !strings.Contains(recallBody, promptHe) || !strings.Contains(recallBody, `class="choices"`) {
		t.Fatalf("recall prompt = %d %s", recall.Code, recallBody)
	}
	for _, hidden := range []string{promptLat, produceLat, gapLat} {
		if strings.Contains(recallBody, hidden) {
			t.Fatalf("recall shows %q before the attempt:\n%s", hidden, recallBody)
		}
	}
	recallAnswer := postLesson(t, h, cookie, lessonPath, runID, itoa(promptID), true, "qa-csrf")
	if recallAnswer.Code != http.StatusOK || !strings.Contains(recallAnswer.Body.String(), promptLat) {
		t.Fatalf("recall answer hid the transliteration: %d %s", recallAnswer.Code, recallAnswer.Body.String())
	}

	produce := postLesson(t, h, cookie, lessonPath, runID, "continue", true, "qa-csrf")
	produceBody := produce.Body.String()
	if produce.Code != http.StatusOK || !strings.Contains(produceBody, produceHe) || !strings.Contains(produceBody, produceGl) || !strings.Contains(produceBody, `class="choices"`) {
		t.Fatalf("produce prompt = %d %s", produce.Code, produceBody)
	}
	for _, hidden := range []string{promptLat, produceLat, gapLat} {
		if strings.Contains(produceBody, hidden) {
			t.Fatalf("produce shows %q before the attempt:\n%s", hidden, produceBody)
		}
	}
	produceAnswer := postLesson(t, h, cookie, lessonPath, runID, itoa(produceID), true, "qa-csrf")
	if produceAnswer.Code != http.StatusOK || !strings.Contains(produceAnswer.Body.String(), produceLat) {
		t.Fatalf("produce answer hid the transliteration: %d %s", produceAnswer.Code, produceAnswer.Body.String())
	}

	cloze := postLesson(t, h, cookie, lessonPath, runID, "continue", true, "qa-csrf")
	clozeBody := cloze.Body.String()
	if cloze.Code != http.StatusOK || strings.Contains(clozeBody, gapGl) || !strings.Contains(clozeBody, `class="choices"`) {
		t.Fatalf("cloze shows the gap gloss before the attempt: %d %s", cloze.Code, clozeBody)
	}
	if !strings.Contains(clozeBody, gapHe) || !strings.Contains(clozeBody, promptHe) {
		t.Fatalf("cloze hid the choice or the other word:\n%s", clozeBody)
	}
	clozeAnswer := postLesson(t, h, cookie, lessonPath, runID, itoa(gapID), true, "qa-csrf")
	if clozeAnswer.Code != http.StatusOK || !strings.Contains(clozeAnswer.Body.String(), gapGl) {
		t.Fatalf("cloze answer hid the gloss: %d %s", clozeAnswer.Code, clozeAnswer.Body.String())
	}

	done := postLesson(t, h, cookie, lessonPath, runID, "continue", true, "qa-csrf")
	if done.Code != http.StatusOK || !strings.Contains(done.Body.String(), "Lesson complete.") {
		t.Fatalf("finish = %d %s", done.Code, done.Body.String())
	}

	check := getAuthed(t, h, cookie, "/lesson")
	checkRun := runValue(t, check)
	if strings.Contains(check, promptLat) || !strings.Contains(check, promptHe) || !strings.Contains(check, `class="choices"`) {
		t.Fatalf("checkpoint recall transliteration before the attempt:\n%s", check)
	}
	checkAnswer := postLesson(t, h, cookie, checkPath, checkRun, itoa(promptID), true, "qa-csrf")
	if checkAnswer.Code != http.StatusOK || !strings.Contains(checkAnswer.Body.String(), promptLat) {
		t.Fatalf("checkpoint answer = %d %s", checkAnswer.Code, checkAnswer.Body.String())
	}
	checkProduce := postLesson(t, h, cookie, checkPath, checkRun, "continue", true, "qa-csrf")
	if checkProduce.Code != http.StatusOK || strings.Contains(checkProduce.Body.String(), produceLat) || !strings.Contains(checkProduce.Body.String(), produceHe) || !strings.Contains(checkProduce.Body.String(), `class="choices"`) {
		t.Fatalf("checkpoint produce = %d %s", checkProduce.Code, checkProduce.Body.String())
	}
}

// The warm-up opens on a letters item: the learner hears a syllable and picks
// the letter. The Latin sound is the answer key. It stays off the screen
// before the attempt and after it, because a placement pick does not say
// whether the choice was right. The heard letter is one of the buttons.
func testQAPlacementHidesGloss(t *testing.T) {
	t.Helper()
	h, store, learner, _ := newPlacementRouter(t)
	start := learner.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {learner.csrf}}, "", true)
	if start.Code != http.StatusOK {
		t.Fatalf("start = %d %s", start.Code, start.Body.String())
	}
	body := start.Body.String()
	if screenOf(body) != views.PlaceLetters || phaseOf(body) != views.PlaceAsk {
		t.Fatalf("opening is %q %q, want letters ask:\n%s", screenOf(body), phaseOf(body), body)
	}
	if !strings.Contains(body, "Warm-up") || !strings.Contains(body, "Which letter did you hear?") {
		t.Fatalf("letters opening is missing its question:\n%s", body)
	}
	if strings.Contains(body, `class="pl__he"`) {
		t.Fatalf("letters opening shows a Hebrew word:\n%s", body)
	}
	if strings.Contains(body, "Answer saved") {
		t.Fatalf("letters opening already says the answer was saved:\n%s", body)
	}
	if n := strings.Count(body, "data-pl-option"); n != 4 {
		t.Fatalf("letter buttons = %d, want 4", n)
	}
	letter, correct, wrong := openLetter(t, store, body)
	if !strings.Contains(body, correct) {
		t.Fatalf("the heard letter %q is not a choice:\n%s", correct, body)
	}
	if soundShown(body, letter.Sound) {
		t.Fatalf("the ask shows the sound %q:\n%s", letter.Sound, body)
	}

	picked := learner.post(t, h, "/_hx/placement/answer", url.Values{
		"run_id": {hidden(t, body, "run_id")},
		"item":   {hidden(t, body, "item")},
		"option": {wrong},
		"_csrf":  {learner.csrf},
	}, "", true)
	if picked.Code != http.StatusOK {
		t.Fatalf("answer = %d %s", picked.Code, picked.Body.String())
	}
	after := picked.Body.String()
	if screenOf(after) != views.PlaceLetters || phaseOf(after) != views.PlacePicked {
		t.Fatalf("after the pick is %q %q, want letters picked", screenOf(after), phaseOf(after))
	}
	if !strings.Contains(after, "Answer saved") {
		t.Fatalf("the pick did not confirm the answer was saved:\n%s", after)
	}
	if soundShown(after, letter.Sound) || verdictShown(after) {
		t.Fatalf("the pick revealed the sound %q or a verdict:\n%s", letter.Sound, after)
	}
}

func openLetter(t *testing.T, store *db.Store, body string) (placement.Letter, string, string) {
	t.Helper()
	runID, err := strconv.ParseInt(hidden(t, body, "run_id"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.ListPlacementItems(t.Context(), runID)
	if err != nil {
		t.Fatal(err)
	}
	var open db.PlacementItem
	found := false
	for _, it := range items {
		if it.Outcome == "" {
			open, found = it, true
			break
		}
	}
	if !found || open.Part != "letters" || !open.CorrectIdx.Valid {
		t.Fatalf("open item = %+v", open)
	}
	var stored struct {
		Options []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"options"`
	}
	if err := json.Unmarshal([]byte(open.Options), &stored); err != nil {
		t.Fatal(err)
	}
	if int(open.CorrectIdx.Int64) >= len(stored.Options) {
		t.Fatalf("correct index %d of %d", open.CorrectIdx.Int64, len(stored.Options))
	}
	correct := stored.Options[open.CorrectIdx.Int64]
	var wrong string
	for _, opt := range stored.Options {
		if opt.ID != correct.ID {
			wrong = opt.ID
			break
		}
	}
	if wrong == "" || correct.Text == "" {
		t.Fatalf("options = %+v", stored.Options)
	}
	letters, err := placement.Letters()
	if err != nil {
		t.Fatal(err)
	}
	for _, letter := range letters {
		if letter.ID == open.ItemID {
			if letter.LetterNFC != correct.Text || letter.Sound == "" {
				t.Fatalf("letter %s text %q sound %q, correct option %q", letter.ID, letter.LetterNFC, letter.Sound, correct.Text)
			}
			return letter, correct.Text, wrong
		}
	}
	t.Fatalf("letters.json has no %s", open.ItemID)
	return placement.Letter{}, "", ""
}

func soundShown(html, sound string) bool {
	text := regexp.MustCompile(`(?s)<script\b.*?</script>`).ReplaceAllString(html, " ")
	text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, " ")
	return regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(sound) + `\b`).MatchString(text)
}

func verdictShown(html string) bool {
	text := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(html, " ")
	return regexp.MustCompile(`(?i)\b(correct|incorrect|not quite|right answer)\b`).MatchString(text)
}
