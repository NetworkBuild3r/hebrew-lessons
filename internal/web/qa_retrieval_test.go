package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
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
	heCard := addCard(t, store, heLearner.user.ID, heLex, fsrs.DirHeEn, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
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
	enCard := addCard(t, store, enLearner.user.ID, enLex, fsrs.DirEnHe, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
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
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Probe", Situation: "A probe.", Concept: "Probe concept.", Position: 0,
		NewWords: []string{promptHe, produceHe, gapHe},
		Lessons: []units.Lesson{
			{Kind: units.KindLesson, Activities: []units.Activity{
				{Type: units.ActivityListenPick, Text: promptHe},
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
	if recall.Code != http.StatusOK || !strings.Contains(recallBody, promptHe) {
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
	if produce.Code != http.StatusOK || !strings.Contains(produceBody, produceHe) || !strings.Contains(produceBody, produceGl) {
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
	if cloze.Code != http.StatusOK || strings.Contains(clozeBody, gapGl) {
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
	if strings.Contains(check, promptLat) || !strings.Contains(check, promptHe) {
		t.Fatalf("checkpoint recall transliteration before the attempt:\n%s", check)
	}
	checkAnswer := postLesson(t, h, cookie, checkPath, checkRun, itoa(promptID), true, "qa-csrf")
	if checkAnswer.Code != http.StatusOK || !strings.Contains(checkAnswer.Body.String(), promptLat) {
		t.Fatalf("checkpoint answer = %d %s", checkAnswer.Code, checkAnswer.Body.String())
	}
	checkProduce := postLesson(t, h, cookie, checkPath, checkRun, "continue", true, "qa-csrf")
	if checkProduce.Code != http.StatusOK || strings.Contains(checkProduce.Body.String(), produceLat) || !strings.Contains(checkProduce.Body.String(), produceHe) {
		t.Fatalf("checkpoint produce = %d %s", checkProduce.Code, checkProduce.Body.String())
	}
}

func testQAPlacementHidesGloss(t *testing.T) {
	t.Helper()
	h, _, learner, _ := newPlacementRouter(t)
	start := learner.post(t, h, "/_hx/placement/start", url.Values{"_csrf": {learner.csrf}}, "", true)
	if start.Code != http.StatusOK {
		t.Fatalf("start = %d %s", start.Code, start.Body.String())
	}
	body := start.Body.String()
	he := regexp.MustCompile(`<bdi class="wb__he heword" lang="he" dir="rtl"[^>]*>([^<]*)</bdi>`).FindStringSubmatch(body)
	if he == nil || he[1] == "" {
		t.Fatalf("placement front has no Hebrew word:\n%s", body)
	}
	gloss := placementGloss(t, he[1])
	if strings.Contains(body, gloss) {
		t.Fatalf("placement front contains the gloss %q:\n%s", gloss, body)
	}
	flip := learner.post(t, h, "/_hx/placement/flip", url.Values{
		"run_id":  {hidden(t, body, "run_id")},
		"item_id": {hidden(t, body, "item_id")},
		"_csrf":   {learner.csrf},
	}, "", true)
	if flip.Code != http.StatusOK || !strings.Contains(flip.Body.String(), gloss) {
		t.Fatalf("placement flip did not show %q: %d %s", gloss, flip.Code, flip.Body.String())
	}
}

func placementGloss(t *testing.T, text string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "internal", "placement", "items.json"))
	if err != nil {
		t.Fatal(err)
	}
	var items []struct {
		Text  string `json:"text_nfc"`
		Gloss string `json:"gloss_en"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Text == text {
			if item.Gloss == "" {
				t.Fatalf("placement item %q has an empty gloss", text)
			}
			return item.Gloss
		}
	}
	t.Fatalf("placement deck has no gloss for %q", text)
	return ""
}
