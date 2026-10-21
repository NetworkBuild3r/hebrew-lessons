package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestHomeFromRealData(t *testing.T) {
	store, h, now := openApp(t)
	ctx := context.Background()
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	other, err := store.CreateUser(ctx, "b@example.com", "sub-b", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}

	known := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	hard := addLexeme(t, store, "בֹּקֶר", "morning", "boker", "m", "")
	later := addLexeme(t, store, "עֶרֶב", "evening", "erev", "m", "")
	foreign := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	reviewed := now.Add(-48 * time.Hour)
	addCard(t, store, learner.user.ID, known, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	addCard(t, store, learner.user.ID, hard, fsrs.SkillRead, now.Add(-time.Minute), 1, int(fsrs.Hard), reviewed)
	addCard(t, store, learner.user.ID, later, fsrs.SkillRead, now.Add(24*time.Hour), 1, 0, time.Time{})
	addCard(t, store, other.ID, foreign, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	draft, err := testharness.InsertUnit(ctx, store, "Draft", "draft")
	if err != nil {
		t.Fatal(err)
	}
	done, err := testharness.InsertUnit(ctx, store, "Done", "published")
	if err != nil {
		t.Fatal(err)
	}
	next, err := testharness.InsertUnit(ctx, store, "Cafe", "published")
	if err != nil {
		t.Fatal(err)
	}
	publishUnit(t, store, draft, 0, "Draft", "street", `{"lessons":[{}]}`, "draft")
	publishUnit(t, store, done, 1, "Done", "street", `{"lessons":[{}]}`, "published")
	publishUnit(t, store, next, 2, "Cafe", "At the cafe", `{"lessons":[{}]}`, "published")
	finished := now
	if _, err := testharness.InsertLessonRun(ctx, store, learner.user.ID, done, 0, now.Add(-time.Hour), &finished); err != nil {
		t.Fatal(err)
	}

	body := getLearner(t, h, learner, "/")
	if !strings.Contains(body, `data-due="2"`) || !strings.Contains(body, `data-known="1"`) {
		t.Fatalf("due/known: %s", articleAttrs(body))
	}
	if !strings.Contains(body, "Cafe") || !strings.Contains(body, "At the cafe") || strings.Contains(body, ">Draft<") || strings.Contains(body, ">Done<") {
		t.Fatal("next unit is not the first unfinished published unit")
	}
	if strings.Contains(body, "data-placement=") {
		t.Fatal("a learner with no run rendered a placement")
	}
	progress := getLearner(t, h, learner, "/progress")
	// Progress (SPEC-018) shows the learner model by skill. What is left of the old three counts
	// is the lessons finished; the known count and the due count are on Today.
	if !strings.Contains(progress, `>1/2</bdi>`) {
		t.Fatalf("progress missing the lessons finished\n%s", progress)
	}

	runID, err := store.InsertPlacementRun(ctx, learner.user.ID, now, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `
		INSERT INTO placement_answers (run_id, item_id, rating, shown_at, answered_at)
		VALUES (?, 'cafe', 3, ?, ?)`, runID, db.FormatTime(now), db.FormatTime(now)); err != nil {
		t.Fatal(err)
	}
	active := getLearner(t, h, learner, "/")
	if !strings.Contains(active, `data-placement="active"`) || !strings.Contains(active, `data-placement-level="-1"`) || !strings.Contains(active, `data-placement-rated="1"`) || !strings.Contains(active, `data-placement-words="0"`) || !strings.Contains(active, `data-placement-at="`+db.FormatTime(now)+`"`) {
		t.Fatalf("active placement: %s", articleAttrs(active))
	}

	doneAt := now.Add(time.Hour)
	if err := store.FinishPlacement(ctx, runID, doneAt, 2, "cap_cards", `{"estimated_known_words":40,"bands":[]}`); err != nil {
		t.Fatal(err)
	}
	scored := getLearner(t, h, learner, "/")
	if strings.Contains(scored, `data-placement="active"`) || !strings.Contains(scored, `data-placement="finished"`) || !strings.Contains(scored, `data-placement-level="2"`) || !strings.Contains(scored, `data-placement-words="40"`) || !strings.Contains(scored, `data-placement-at="`+db.FormatTime(doneAt)+`"`) {
		t.Fatalf("finished placement: %s", articleAttrs(scored))
	}
}

func TestFakeTodayDataIsGone(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	forbidden := []string{"SampleWords", "FakeLearner", "Getting ready in the morning", "הַבֹּקֶר שֶׁלִּי"}
	err := filepath.Walk(filepath.Dir(file), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".templ") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(body)
		for _, phrase := range forbidden {
			if strings.Contains(text, phrase) {
				rel, _ := filepath.Rel(filepath.Dir(file), path)
				t.Errorf("%s still contains %q", rel, phrase)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestReviewBuildsSession(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	firstText := "שָׁלוֹם"
	secondText := "בֹּקֶר"
	audio := strings.Repeat("ab", 32)
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at)
		VALUES (?, ?, 'voice', 'model', 3, ?)`, audio, firstText, db.FormatTime(now)); err != nil {
		t.Fatal(err)
	}
	first := addLexeme(t, store, firstText, "hello", "shalom", "m", audio)
	second := addLexeme(t, store, secondText, "morning", "boker", "f", "")
	reviewed := now.Add(-48 * time.Hour)
	addCard(t, store, learner.user.ID, first, fsrs.SkillRead, now.Add(-2*time.Hour), 1, int(fsrs.Good), reviewed)
	addCard(t, store, learner.user.ID, second, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	body := getLearner(t, h, learner, "/review")
	if !strings.Contains(body, `data-kind="read"`) || !strings.Contains(body, `data-pointed="`+firstText+`"`) || !strings.Contains(body, `data-show-unpointed="false"`) || !strings.Contains(body, `data-translit="shalom"`) || !strings.Contains(body, `data-index="1"`) || !strings.Contains(body, `data-total="2"`) || !strings.Contains(body, `data-revealed="false"`) {
		t.Fatalf("first card: %s", cardAttrs(body))
	}
	if !strings.Contains(body, `data-src="/audio/`+audio+`.mp3"`) || !strings.Contains(body, "shalom") || !strings.Contains(body, `action="/_hx/review/reveal"`) {
		t.Fatal("prompt, audio, or reveal form missing")
	}
	if strings.Contains(body, "hello") || strings.Contains(body, "ז׳") || strings.Contains(body, `name="rating"`) {
		t.Fatal("the answer or the grade buttons rendered before reveal")
	}
	if strings.Contains(cardAttrs(body), secondText) {
		t.Fatal("the second card rendered first")
	}
	again := getLearner(t, h, learner, "/review")
	if cardAttrs(again) != cardAttrs(body) {
		t.Fatal("a second GET built a new queue")
	}

	en := addLearner(t, store, "en@example.com", "sub-en", now)
	enAudio := strings.Repeat("cd", 32)
	enText := "כִּסֵּא"
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at)
		VALUES (?, ?, 'voice', 'model', 3, ?)`, enAudio, enText, db.FormatTime(now)); err != nil {
		t.Fatal(err)
	}
	word := addLexeme(t, store, enText, "chair", "kise", "m", enAudio)
	enCard := addCard(t, store, en.user.ID, word, fsrs.SkillRecall, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	prod := getLearner(t, h, en, "/review")
	if !strings.Contains(prod, `data-kind="recall"`) || strings.Contains(prod, enText) || !strings.Contains(prod, "chair") || !strings.Contains(prod, `data-revealed="false"`) {
		t.Fatalf("recall card leaked the answer: %s", cardAttrs(prod))
	}
	if strings.Contains(prod, "data-translit") || strings.Contains(prod, "kise") || strings.Contains(prod, "/audio/") {
		t.Fatalf("recall leaked translit or audio before reveal: %s", prod)
	}
	shown := postReveal(t, h, en, enCard, "https://learn.example")
	shownBody := shown.Body.String()
	if shown.Code != http.StatusOK || !strings.Contains(shownBody, `data-pointed="`+enText+`"`) || !strings.Contains(shownBody, "chair") || !strings.Contains(shownBody, `data-revealed="true"`) || !strings.Contains(shownBody, `name="rating"`) || !strings.Contains(shownBody, `data-translit="kise"`) || !strings.Contains(shownBody, "kise") || !strings.Contains(shownBody, `data-src="/audio/`+enAudio+`.mp3"`) {
		t.Fatalf("recall reveal = %d %s", shown.Code, shownBody)
	}
}

func TestReviewShowUnpointed(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	text := "שָׁלוֹם"
	lex := addLexeme(t, store, text, "hello", "shalom", "m", "")
	reviewed := now.Add(-48 * time.Hour)
	id := addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 21, int(fsrs.Good), reviewed)
	insertReview(t, store, id, int(fsrs.Good), reviewed.Add(time.Minute))
	insertReview(t, store, id, int(fsrs.Good), reviewed.Add(2*time.Minute))
	body := getLearner(t, h, learner, "/review")
	plain := units.Plain(text)
	if !strings.Contains(body, `data-show-unpointed="true"`) || !strings.Contains(body, `data-pointed="`+text+`"`) || !strings.Contains(body, `data-unpointed="`+plain+`"`) {
		t.Fatalf("unpointed card: %s", cardAttrs(body))
	}
	count := regexp.MustCompile(`(?s)<div class="act__word">.*?</div>`).FindString(body)
	if !strings.Contains(count, plain) || strings.Contains(count, text) {
		t.Fatalf("prompt = %s, want the unpointed form", count)
	}

	store2, h2, now2 := openApp(t)
	learner2 := addLearner(t, store2, "a@example.com", "sub-a", now2)
	lex2 := addLexeme(t, store2, text, "hello", "shalom", "m", "")
	lapsed := addCard(t, store2, learner2.user.ID, lex2, fsrs.SkillRead, now2.Add(-time.Hour), 40, int(fsrs.Good), reviewed)
	insertReview(t, store2, lapsed, int(fsrs.Good), reviewed.Add(time.Minute))
	insertReview(t, store2, lapsed, int(fsrs.Again), reviewed.Add(2*time.Minute))
	pointed := getLearner(t, h2, learner2, "/review")
	if !strings.Contains(pointed, `data-show-unpointed="false"`) || !strings.Contains(pointed, `data-pointed="`+text+`"`) {
		t.Fatalf("lapsed card: %s", cardAttrs(pointed))
	}
	count = regexp.MustCompile(`(?s)<div class="act__word">.*?</div>`).FindString(pointed)
	if !strings.Contains(count, text) {
		t.Fatalf("prompt = %s, want the pointed form", count)
	}
}

func TestReviewGrade(t *testing.T) {
	store, h, now := openApp(t)
	ctx := context.Background()
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	other := addLearner(t, store, "b@example.com", "sub-b", now)
	reviewed := now.Add(-48 * time.Hour)
	firstLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	secondLex := addLexeme(t, store, "בֹּקֶר", "morning", "boker", "m", "")
	foreignLex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	first := addCard(t, store, learner.user.ID, firstLex, fsrs.SkillRead, now.Add(-2*time.Hour), 1, int(fsrs.Good), reviewed)
	second := addCard(t, store, learner.user.ID, secondLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	foreign := addCard(t, store, other.user.ID, foreignLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	before, err := store.GetCard(ctx, first)
	if err != nil {
		t.Fatal(err)
	}

	missing := postGrade(t, h, learner, 1, "3", "https://learn.example")
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("missing study session = %d", missing.Code)
	}

	page := getLearner(t, h, learner, "/review")
	cardID := attr(t, page, "data-card")
	if cardID != fmt.Sprint(first) {
		t.Fatalf("served %s, want %d", cardID, first)
	}

	bad := postGrade(t, h, learner, first, "9", "https://learn.example")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad rating = %d", bad.Code)
	}
	if n := len(mustLogs(t, store, first)); n != 1 {
		t.Fatalf("bad rating wrote a log, count %d", n)
	}
	still := getLearner(t, h, learner, "/review")
	if attr(t, still, "data-card") != fmt.Sprint(first) {
		t.Fatal("a rejected grade advanced the queue")
	}

	unknown := postGrade(t, h, learner, 99999, "3", "https://learn.example")
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown card = %d", unknown.Code)
	}
	stolen := postGrade(t, h, learner, foreign, "3", "https://learn.example")
	if stolen.Code != http.StatusBadRequest {
		t.Fatalf("foreign card = %d", stolen.Code)
	}
	if n := len(mustLogs(t, store, foreign)); n != 1 {
		t.Fatalf("foreign card log count %d", n)
	}

	noToken := postGradeRaw(t, h, learner.cookie, "", first, "3", "https://learn.example")
	if noToken.Code != http.StatusForbidden {
		t.Fatalf("missing csrf = %d", noToken.Code)
	}
	evil := postGrade(t, h, learner, first, "3", "https://evil.example")
	if evil.Code != http.StatusForbidden {
		t.Fatalf("bad origin = %d", evil.Code)
	}

	hidden := postGrade(t, h, learner, first, "3", "https://learn.example")
	if hidden.Code != http.StatusBadRequest {
		t.Fatalf("unrevealed grade = %d, want 400", hidden.Code)
	}
	if n := len(mustLogs(t, store, first)); n != 1 {
		t.Fatalf("unrevealed grade wrote a log, count %d", n)
	}
	if attr(t, getLearner(t, h, learner, "/review"), "data-card") != fmt.Sprint(first) {
		t.Fatal("a rejected grade advanced the queue")
	}
	next := gradeRevealed(t, h, learner, first, "3", "https://learn.example")
	if next.Code != http.StatusOK {
		t.Fatalf("good grade = %d\n%s", next.Code, next.Body.String())
	}
	body := next.Body.String()
	if !strings.Contains(body, `data-card="`+fmt.Sprint(second)+`"`) || !strings.Contains(body, "בֹּקֶר") || !strings.Contains(body, `data-answered="1"`) || !strings.Contains(body, `data-correct="1"`) {
		t.Fatalf("next card: %s", body)
	}
	logs := mustLogs(t, store, first)
	if len(logs) != 2 || logs[1].Rating != int(fsrs.Good) || !logs[1].Peeked {
		t.Fatalf("log = %+v, want a revealed Good", logs)
	}
	after, err := store.GetCard(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if after.Stability == before.Stability || after.Due.Equal(before.Due) {
		t.Fatalf("schedule unchanged: before %+v after %+v", before, after)
	}
	secondGrade := postGrade(t, h, learner, first, "3", "https://learn.example")
	if secondGrade.Code != http.StatusBadRequest {
		t.Fatalf("second grade of the same card = %d", secondGrade.Code)
	}
	if n := len(mustLogs(t, store, first)); n != 2 {
		t.Fatalf("second grade wrote another log, count %d", n)
	}

	// Again puts the served card back once.
	againStore, againH, againNow := openApp(t)
	one := addLearner(t, againStore, "a@example.com", "sub-a", againNow)
	lex := addLexeme(t, againStore, "שָׁלוֹם", "hello", "shalom", "m", "")
	card := addCard(t, againStore, one.user.ID, lex, fsrs.SkillRead, againNow.Add(-time.Hour), 1, int(fsrs.Good), againNow.Add(-48*time.Hour))
	getLearner(t, againH, one, "/review")
	retry := gradeRevealed(t, againH, one, card, "1", "https://learn.example")
	if retry.Code != http.StatusOK {
		t.Fatalf("again = %d\n%s", retry.Code, retry.Body.String())
	}
	if !strings.Contains(retry.Body.String(), `data-card="`+fmt.Sprint(card)+`"`) || !strings.Contains(retry.Body.String(), `data-retry="true"`) || !strings.Contains(retry.Body.String(), `data-correct="0"`) || !strings.Contains(retry.Body.String(), `data-answered="1"`) {
		t.Fatalf("requeue: %s", retry.Body.String())
	}
	againLogs := mustLogs(t, againStore, card)
	if len(againLogs) != 2 || againLogs[1].Rating != int(fsrs.Again) {
		t.Fatalf("again log = %+v", againLogs)
	}
	row, err := againStore.GetCard(context.Background(), card)
	if err != nil {
		t.Fatal(err)
	}
	if row.Lapses < 1 {
		t.Fatalf("lapses = %d", row.Lapses)
	}
}

func TestReviewLearnersAreIsolated(t *testing.T) {
	store, h, now := openApp(t)
	a := addLearner(t, store, "a@example.com", "sub-a", now)
	b := addLearner(t, store, "b@example.com", "sub-b", now)
	reviewed := now.Add(-48 * time.Hour)
	aLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	bLex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	aCard := addCard(t, store, a.user.ID, aLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	bCard := addCard(t, store, b.user.ID, bLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)

	aPage := getLearner(t, h, a, "/review")
	bPage := getLearner(t, h, b, "/review")
	if !strings.Contains(aPage, "שָׁלוֹם") || strings.Contains(aPage, "מַיִם") {
		t.Fatal("learner A saw the other queue")
	}
	if !strings.Contains(bPage, "מַיִם") || strings.Contains(bPage, "שָׁלוֹם") {
		t.Fatal("learner B saw the other queue")
	}
	graded := gradeRevealed(t, h, a, aCard, "3", "https://learn.example")
	if graded.Code != http.StatusOK {
		t.Fatalf("grade A = %d", graded.Code)
	}
	if n := len(mustLogs(t, store, bCard)); n != 1 {
		t.Fatalf("B log count = %d", n)
	}
	if n := len(mustLogs(t, store, aCard)); n != 2 {
		t.Fatalf("A log count = %d", n)
	}
	bAgain := getLearner(t, h, b, "/review")
	if !strings.Contains(bAgain, `data-card="`+fmt.Sprint(bCard)+`"`) || !strings.Contains(bAgain, "מַיִם") {
		t.Fatal("B's queue moved when A graded")
	}
}

func TestReviewDrillKinds(t *testing.T) {
	store, h, now := openApp(t)
	ctx := context.Background()
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	lex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	card := addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
	rev := fsrs.New(store, store.Clock())
	for i := 0; i < session.MinMissesForDrill; i++ {
		if err := rev.RecordMiss(ctx, learner.user.ID, fsrs.MissLetterPair, "ב/כ", 0); err != nil {
			t.Fatal(err)
		}
		if err := rev.RecordMiss(ctx, learner.user.ID, fsrs.MissAgreement, "הוא/היא", 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := rev.RecordMiss(ctx, learner.user.ID, fsrs.MissLetterPair, "ג/ק", 0); err != nil {
		t.Fatal(err)
	}
	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-kind") != fsrs.SkillRead || attr(t, page, "data-card") != fmt.Sprint(card) {
		t.Fatalf("drill replaced the study card: %s", cardAttrs(page))
	}
	letter := gradeRevealed(t, h, learner, card, "3", "https://learn.example")
	if letter.Code != http.StatusOK || attr(t, letter.Body.String(), "data-kind") != session.KindLetter || !strings.Contains(letter.Body.String(), "ב/כ") {
		t.Fatalf("letter drill: %d %s", letter.Code, letter.Body.String())
	}
	if strings.Contains(letter.Body.String(), "ג/ק") {
		t.Fatal("a pair under the miss threshold was drilled")
	}
	letterID := attr(t, letter.Body.String(), "data-card")
	pattern := gradeRevealed(t, h, learner, mustInt(t, letterID), "3", "https://learn.example")
	if pattern.Code != http.StatusOK || attr(t, pattern.Body.String(), "data-kind") != session.KindPattern || !strings.Contains(pattern.Body.String(), "הוא/היא") {
		t.Fatalf("pattern drill: %d %s", pattern.Code, pattern.Body.String())
	}
	if n := len(mustLogs(t, store, card)); n != 2 {
		t.Fatalf("drill grades wrote review_log, count %d", n)
	}
}

func TestReviewReveal(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	reviewed := now.Add(-48 * time.Hour)
	lex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	card := addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	page := getLearner(t, h, learner, "/review")
	if strings.Contains(page, "hello") || strings.Contains(page, "ז׳") || strings.Contains(page, `name="rating"`) {
		t.Fatal("the back rendered before reveal")
	}

	noToken := postRevealRaw(t, h, learner.cookie, "", card, "https://learn.example", true)
	if noToken.Code != http.StatusForbidden {
		t.Fatalf("missing csrf = %d", noToken.Code)
	}
	evil := postRevealRaw(t, h, learner.cookie, learner.csrf, card, "https://evil.example", true)
	if evil.Code != http.StatusForbidden {
		t.Fatalf("bad origin = %d", evil.Code)
	}
	still := getLearner(t, h, learner, "/review")
	if strings.Contains(still, "hello") || strings.Contains(still, `data-revealed="true"`) {
		t.Fatal("a rejected reveal showed the back")
	}

	shown := postReveal(t, h, learner, card, "https://learn.example")
	if shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d\n%s", shown.Code, shown.Body.String())
	}
	body := shown.Body.String()
	if !strings.Contains(body, "hello") || !strings.Contains(body, "ז׳") || !strings.Contains(body, `name="rating"`) || !strings.Contains(body, `data-revealed="true"`) || !strings.Contains(body, `data-card="`+fmt.Sprint(card)+`"`) {
		t.Fatalf("revealed card: %s", body)
	}

	graded := postGrade(t, h, learner, card, "3", "https://learn.example")
	if graded.Code != http.StatusOK {
		t.Fatalf("grade after reveal = %d\n%s", graded.Code, graded.Body.String())
	}
	logs := mustLogs(t, store, card)
	if len(logs) != 2 || !logs[1].Peeked || logs[1].Rating != int(fsrs.Good) {
		t.Fatalf("log = %+v, want peeked true", logs)
	}
}

func TestReviewDoneSessionRebuilds(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	reviewed := now.Add(-48 * time.Hour)
	firstLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	first := addCard(t, store, learner.user.ID, firstLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	getLearner(t, h, learner, "/review")
	done := gradeRevealed(t, h, learner, first, "3", "https://learn.example")
	if done.Code != http.StatusOK || !strings.Contains(done.Body.String(), `data-done="true"`) {
		t.Fatalf("done = %d %s", done.Code, done.Body.String())
	}
	if err := store.SetCardSuspended(context.Background(), first, true); err != nil {
		t.Fatal(err)
	}
	secondLex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	second := addCard(t, store, learner.user.ID, secondLex, fsrs.SkillRead, now.Add(-time.Minute), 1, int(fsrs.Good), reviewed)
	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-card") != fmt.Sprint(second) || strings.Contains(page, "שָׁלוֹם") || !strings.Contains(page, "מַיִם") || strings.Contains(page, `data-done="true"`) {
		t.Fatalf("rebuilt session: %s", cardAttrs(page))
	}
}

func TestStudyQueueRollsOverAt4AM(t *testing.T) {
	loc := mustDenver(t)
	start := time.Date(2026, 10, 18, 23, 0, 0, 0, loc)
	reuse := time.Date(2026, 10, 19, 3, 30, 0, 0, loc)
	rebuild := time.Date(2026, 10, 19, 4, 1, 0, 0, loc)
	store, h, clk := openStepped(t, start)
	learner := addLearner(t, store, "a@example.com", "sub-a", start)
	if _, err := store.ExecContext(context.Background(), `UPDATE sessions SET expires_at = ? WHERE user_id = ?`, db.FormatTime(start.Add(7*24*time.Hour)), learner.user.ID); err != nil {
		t.Fatal(err)
	}
	reviewed := start.Add(-48 * time.Hour)
	firstLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	first := addCard(t, store, learner.user.ID, firstLex, fsrs.SkillRead, start.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-card") != fmt.Sprint(first) {
		t.Fatalf("first serve = %s", cardAttrs(page))
	}
	if shown := postReveal(t, h, learner, first, "https://learn.example"); shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d", shown.Code)
	}

	clk.Advance(reuse.Sub(start))
	if err := store.SetCardSuspended(context.Background(), first, true); err != nil {
		t.Fatal(err)
	}
	secondLex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	second := addCard(t, store, learner.user.ID, secondLex, fsrs.SkillRead, reuse.Add(-time.Minute), 1, int(fsrs.Good), reviewed)
	kept := getLearner(t, h, learner, "/review")
	if attr(t, kept, "data-card") != fmt.Sprint(first) || !strings.Contains(kept, "שָׁלוֹם") {
		t.Fatalf("03:30 local rebuilt the 23:00 queue: %s", cardAttrs(kept))
	}

	clk.Advance(rebuild.Sub(reuse))
	stale := postGradeRawHX(t, h, learner.cookie, learner.csrf, first, "3", "https://learn.example", false)
	if stale.Code != http.StatusSeeOther {
		t.Fatalf("grade on a previous study day = %d, want 303", stale.Code)
	}
	if loc := stale.Header().Get("Location"); loc != "/review" {
		t.Fatalf("Location = %q, want /review", loc)
	}
	if n := len(mustLogs(t, store, first)); n != 1 {
		t.Fatalf("stale grade wrote a log, count %d", n)
	}
	rebuilt := getLearner(t, h, learner, "/review")
	if attr(t, rebuilt, "data-card") != fmt.Sprint(second) || strings.Contains(rebuilt, "שָׁלוֹם") || !strings.Contains(rebuilt, "מַיִם") {
		t.Fatalf("04:01 local did not rebuild: %s", cardAttrs(rebuilt))
	}
}

func TestStudyQueueIgnoresUTCMidnight(t *testing.T) {
	loc := mustDenver(t)
	start := time.Date(2026, 10, 18, 17, 0, 0, 0, loc)
	later := time.Date(2026, 10, 18, 19, 0, 0, 0, loc)
	midnight := time.Date(2026, 10, 18, 18, 0, 0, 0, loc)
	if !midnight.After(start) || !midnight.Before(later) {
		t.Fatal("Denver 18:00 does not fall between the build and the check")
	}
	if !midnight.Equal(time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("Denver 18:00 = %s, want UTC midnight", midnight.UTC())
	}
	if studyday.Key(start, loc) != studyday.Key(later, loc) {
		t.Fatal("17:00 and 19:00 Denver are not the same study day")
	}

	store, h, clk := openStepped(t, start)
	learner := addLearner(t, store, "a@example.com", "sub-a", start)
	if _, err := store.ExecContext(context.Background(), `UPDATE sessions SET expires_at = ? WHERE user_id = ?`, db.FormatTime(start.Add(7*24*time.Hour)), learner.user.ID); err != nil {
		t.Fatal(err)
	}
	reviewed := start.Add(-48 * time.Hour)
	firstLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	first := addCard(t, store, learner.user.ID, firstLex, fsrs.SkillRead, start.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-card") != fmt.Sprint(first) {
		t.Fatalf("first serve = %s", cardAttrs(page))
	}

	clk.Advance(later.Sub(start))
	if err := store.SetCardSuspended(context.Background(), first, true); err != nil {
		t.Fatal(err)
	}
	secondLex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	addCard(t, store, learner.user.ID, secondLex, fsrs.SkillRead, later.Add(-time.Minute), 1, int(fsrs.Good), reviewed)
	kept := getLearner(t, h, learner, "/review")
	if attr(t, kept, "data-card") != fmt.Sprint(first) || !strings.Contains(kept, "שָׁלוֹם") || strings.Contains(kept, "מַיִם") {
		t.Fatalf("UTC midnight rebuilt the queue: %s", cardAttrs(kept))
	}
}

// TestTodayCountMatchesReviewQueue is 22:00 local. A card due at 02:00 tomorrow
// is still this study day. A card due at 05:00 tomorrow is the next one.
// Today’s count and the review queue are that one card.
func TestTodayCountMatchesReviewQueue(t *testing.T) {
	loc := mustDenver(t)
	now := time.Date(2026, 10, 18, 22, 0, 0, 0, loc)
	early := time.Date(2026, 10, 19, 2, 0, 0, 0, loc)
	late := time.Date(2026, 10, 19, 5, 0, 0, 0, loc)
	if !early.Before(studyday.DueCutoff(now, loc)) || !late.After(studyday.DueCutoff(now, loc)) {
		t.Fatalf("cutoff %s early %s late %s", studyday.DueCutoff(now, loc), early, late)
	}
	store, h, _ := openStepped(t, now)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	soonLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	soon := addCard(t, store, learner.user.ID, soonLex, fsrs.SkillRead, early, 1, 0, time.Time{})
	laterLex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	addCard(t, store, learner.user.ID, laterLex, fsrs.SkillRead, late, 1, 0, time.Time{})

	home := getLearner(t, h, learner, "/")
	if !strings.Contains(home, `data-due="1"`) {
		t.Fatalf("home due count: %s", articleAttrs(home))
	}
	page := getLearner(t, h, learner, "/review")
	if attr(t, page, "data-card") != fmt.Sprint(soon) || attr(t, page, "data-total") != "1" || strings.Contains(page, "מַיִם") || !strings.Contains(page, "שָׁלוֹם") {
		t.Fatalf("review served %s", cardAttrs(page))
	}
}

// TestTodayCountMatchesReviewAcrossRollover pins Today’s due count to the
// number of cards Review serves at 23:30, 03:59, and 04:01 local.
//
// Three new cards, due 20:00, 02:00, and 12:00. Under the 4:00 rollover the
// counts are 2, 2, and 3. A cutoff of “due now” would be 1, 2, and 2.
// TestTodayAndReviewIncludeTheCardDueAtTheCutoff is the inclusive learn-ahead
// cap. A new card due at the next 04:00 is in Today and in Review. A card due
// one nanosecond later is in neither.
func TestTodayAndReviewIncludeTheCardDueAtTheCutoff(t *testing.T) {
	loc := mustDenver(t)
	now := time.Date(2026, 10, 18, 22, 0, 0, 0, loc)
	cutoff := studyday.DueCutoff(now, loc)
	store, h, _ := openStepped(t, now)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	onLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	offLex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	on := addCard(t, store, learner.user.ID, onLex, fsrs.SkillRead, cutoff, 1, 0, time.Time{})
	addCard(t, store, learner.user.ID, offLex, fsrs.SkillRead, cutoff.Add(time.Nanosecond), 1, 0, time.Time{})

	home := getLearner(t, h, learner, "/")
	page := getLearner(t, h, learner, "/review")
	due := attr(t, articleAttrs(home), "data-due")
	if due != "1" || attr(t, page, "data-total") != "1" || attr(t, page, "data-card") != fmt.Sprint(on) || strings.Contains(page, "מַיִם") {
		t.Fatalf("cutoff %s: today %q review %s", cutoff, due, cardAttrs(page))
	}
}

// TestReviewUsesTheLearnerZoneNotDenver is 04:30 in Jerusalem, which is still
// the previous evening in Denver. The card is due for Jerusalem's cutoff and
// not for Denver's.
func TestReviewUsesTheLearnerZoneNotDenver(t *testing.T) {
	jerusalem, err := time.LoadLocation("Asia/Jerusalem")
	if err != nil {
		t.Fatal(err)
	}
	denver := mustDenver(t)
	now := time.Date(2026, 1, 15, 4, 30, 0, 0, jerusalem)
	due := time.Date(2026, 1, 15, 18, 0, 0, 0, time.UTC)
	if !due.After(studyday.DueCutoff(now, denver)) || due.After(studyday.DueCutoff(now, jerusalem)) {
		t.Fatalf("denver cutoff %s jerusalem cutoff %s due %s", studyday.DueCutoff(now, denver), studyday.DueCutoff(now, jerusalem), due)
	}
	store, h, _ := openStepped(t, now)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	if err := store.SetLearnerTZ(context.Background(), learner.user.ID, "Asia/Jerusalem"); err != nil {
		t.Fatal(err)
	}
	lex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	card := addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, due, 1, 0, time.Time{})

	home := getLearner(t, h, learner, "/")
	page := getLearner(t, h, learner, "/review")
	if attr(t, articleAttrs(home), "data-due") != "1" || attr(t, page, "data-total") != "1" || attr(t, page, "data-card") != fmt.Sprint(card) {
		t.Fatalf("today %s review %s", articleAttrs(home), cardAttrs(page))
	}
}

func TestLearnerHomeRejectsAMissingStore(t *testing.T) {
	s := newServer(nil)
	if _, _, err := s.learnerHome(context.Background(), 1, "learner", "Asia/Jerusalem"); err == nil || !strings.Contains(err.Error(), "store clock is not configured") {
		t.Fatalf("nil store: %v", err)
	}
	var none *server
	if _, _, err := none.learnerHome(context.Background(), 1, "learner", ""); err == nil || !strings.Contains(err.Error(), "store clock is not configured") {
		t.Fatalf("nil server: %v", err)
	}
}

func TestSameStudyDay(t *testing.T) {
	denver := mustDenver(t)
	night := time.Date(2026, 10, 18, 23, 30, 0, 0, denver)
	before := time.Date(2026, 10, 19, 3, 59, 0, 0, denver)
	at := time.Date(2026, 10, 19, 4, 0, 0, 0, denver)
	if sameStudyDay(time.Time{}, night, denver) {
		t.Fatal("a zero built time counted as this study day")
	}
	if !sameStudyDay(night, before, denver) || !sameStudyDay(night, before, nil) {
		t.Fatal("23:30 and 03:59 are not the same study day")
	}
	if sameStudyDay(night, at, denver) {
		t.Fatal("04:00 stayed on the previous study day")
	}
}

func TestTodayCountMatchesReviewAcrossRollover(t *testing.T) {
	loc := mustDenver(t)
	night := time.Date(2026, 10, 18, 23, 30, 0, 0, loc)
	before := time.Date(2026, 10, 19, 3, 59, 0, 0, loc)
	after := time.Date(2026, 10, 19, 4, 1, 0, 0, loc)
	if !studyday.DueCutoff(night, loc).Equal(studyday.DueCutoff(before, loc)) || !studyday.DueCutoff(after, loc).After(studyday.DueCutoff(before, loc)) {
		t.Fatal("the three clocks do not straddle one rollover")
	}

	store, h, clk := openStepped(t, night)
	learner := addLearner(t, store, "a@example.com", "sub-a", night)
	if _, err := store.ExecContext(context.Background(), `UPDATE sessions SET expires_at = ? WHERE user_id = ?`, db.FormatTime(night.Add(7*24*time.Hour)), learner.user.ID); err != nil {
		t.Fatal(err)
	}
	overdueLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	earlyLex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
	laterLex := addLexeme(t, store, "בֹּקֶר", "morning", "boker", "m", "")
	addCard(t, store, learner.user.ID, overdueLex, fsrs.SkillRead, time.Date(2026, 10, 18, 20, 0, 0, 0, loc), 1, 0, time.Time{})
	addCard(t, store, learner.user.ID, earlyLex, fsrs.SkillRead, time.Date(2026, 10, 19, 2, 0, 0, 0, loc), 1, 0, time.Time{})
	addCard(t, store, learner.user.ID, laterLex, fsrs.SkillRead, time.Date(2026, 10, 19, 12, 0, 0, 0, loc), 1, 0, time.Time{})

	check := func(at time.Time, want string) {
		t.Helper()
		home := getLearner(t, h, learner, "/")
		page := getLearner(t, h, learner, "/review")
		due := attr(t, articleAttrs(home), "data-due")
		served := attr(t, page, "data-total")
		if due != want || served != want || due != served {
			t.Fatalf("%s local: today %q review %q, want %s", at.Format("15:04"), due, served, want)
		}
	}
	check(night, "2")
	clk.Advance(before.Sub(night))
	check(before, "2")
	clk.Advance(after.Sub(before))
	check(after, "3")
}

func TestStudyQueueMapIsBounded(t *testing.T) {
	store, _, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	s := newServer(store)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := 0; i < maxStudyQueues+5; i++ {
		s.queues[fmt.Sprintf("k%03d", i)] = &studyQueue{
			userID:  learner.user.ID,
			builtAt: now.Add(time.Duration(i) * time.Second),
		}
	}
	s.queues["old"] = &studyQueue{userID: learner.user.ID, builtAt: now.Add(-48 * time.Hour)}
	s.queues["nil"] = nil
	s.evictLocked(now)
	if len(s.queues) != maxStudyQueues {
		t.Fatalf("len = %d, want %d", len(s.queues), maxStudyQueues)
	}
	if _, ok := s.queues["k000"]; ok {
		t.Fatal("the oldest queue stayed")
	}
	if _, ok := s.queues["old"]; ok {
		t.Fatal("a queue from another study day stayed")
	}
	if _, ok := s.queues["nil"]; ok {
		t.Fatal("a nil queue stayed")
	}
	q, err := s.queueLocked(context.Background(), "fresh", learner.user.ID, studyday.Load(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.queues) != maxStudyQueues {
		t.Fatalf("len after a new queue = %d, want %d", len(s.queues), maxStudyQueues)
	}
	if s.queues["fresh"] != q {
		t.Fatal("the new queue was not stored")
	}
	if _, ok := s.queues["k005"]; ok {
		t.Fatal("the oldest remaining queue was not dropped to make room")
	}
}

func TestReviewGradeDBError(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	lex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	card := addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
	getLearner(t, h, learner, "/review")
	shown := postReveal(t, h, learner, card, "https://learn.example")
	if shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d", shown.Code)
	}
	if _, err := store.ExecContext(context.Background(), `DELETE FROM cards WHERE id = ?`, card); err != nil {
		t.Fatal(err)
	}
	got := postGrade(t, h, learner, card, "3", "https://learn.example")
	if got.Code != http.StatusInternalServerError {
		t.Fatalf("db error = %d, want 500", got.Code)
	}
}

func TestReviewGradeWithoutHTMX(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	reviewed := now.Add(-48 * time.Hour)
	firstLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	secondLex := addLexeme(t, store, "בֹּקֶר", "morning", "boker", "m", "")
	first := addCard(t, store, learner.user.ID, firstLex, fsrs.SkillRead, now.Add(-2*time.Hour), 1, int(fsrs.Good), reviewed)
	second := addCard(t, store, learner.user.ID, secondLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
	getLearner(t, h, learner, "/review")

	shown := postRevealRaw(t, h, learner.cookie, learner.csrf, first, "https://learn.example", false)
	if shown.Code != http.StatusSeeOther || shown.Header().Get("Location") != "/review" || strings.Contains(shown.Body.String(), `id="review-card"`) {
		t.Fatalf("plain reveal = %d location %q body %s", shown.Code, shown.Header().Get("Location"), shown.Body.String())
	}
	revealed := getLearner(t, h, learner, "/review")
	if !strings.Contains(revealed, "<!doctype html>") || !strings.Contains(revealed, "hello") || !strings.Contains(revealed, `name="rating"`) || !strings.Contains(revealed, `data-revealed="true"`) {
		t.Fatal("the follow-up GET did not render the revealed card")
	}

	graded := postGradeRawHX(t, h, learner.cookie, learner.csrf, first, "3", "https://learn.example", false)
	if graded.Code != http.StatusSeeOther || graded.Header().Get("Location") != "/review" || strings.Contains(graded.Body.String(), `id="review-card"`) {
		t.Fatalf("plain grade = %d location %q body %s", graded.Code, graded.Header().Get("Location"), graded.Body.String())
	}
	logs := mustLogs(t, store, first)
	if len(logs) != 2 || !logs[1].Peeked {
		t.Fatalf("plain grade log = %+v", logs)
	}
	next := getLearner(t, h, learner, "/review")
	if !strings.Contains(next, "<!doctype html>") || attr(t, next, "data-card") != fmt.Sprint(second) || !strings.Contains(next, "בֹּקֶר") || strings.Contains(next, `data-revealed="true"`) {
		t.Fatalf("next page: %s", cardAttrs(next))
	}
	revealedSecond := postRevealRaw(t, h, learner.cookie, learner.csrf, second, "https://learn.example", false)
	if revealedSecond.Code != http.StatusSeeOther || revealedSecond.Header().Get("Location") != "/review" {
		t.Fatalf("plain reveal of the next card = %d location %q", revealedSecond.Code, revealedSecond.Header().Get("Location"))
	}
	req := gradeRequest(t, learner.cookie, learner.csrf, second, "3", "https://learn.example", false)
	req.AddCookie(&http.Cookie{Name: "hebrew_lang", Value: "he"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/review?lang=he" {
		t.Fatalf("hebrew plain grade = %d location %q", rec.Code, rec.Header().Get("Location"))
	}
}

type learnerFix struct {
	user   db.User
	cookie *http.Cookie
	csrf   string
}

type stepClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *stepClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func mustDenver(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func openApp(t *testing.T) (*db.Store, http.Handler, time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openAt(t, fixedClock(now))
	return store, h, now
}

func openStepped(t *testing.T, start time.Time) (*db.Store, http.Handler, *stepClock) {
	t.Helper()
	clk := &stepClock{t: start}
	store, h := openAt(t, clk)
	return store, h, clk
}

func openAt(t *testing.T, clk clock.Clock) (*db.Store, http.Handler) {
	t.Helper()
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		CookieSecure:  false,
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"a@example.com", "b@example.com", "en@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// These tests are about the self-graded card. A closed gate leaves only the
	// listening skill, and these lexicons are too small to build a pick from, so every
	// visit is a flashcard. The checked activities have their own tests (activity_test.go).
	study := session.New(store, clk, 1, session.WithGate(func(context.Context, int64) (activity.Gate, error) {
		return activity.Gate{LettersPassed: false}, nil
	}))
	return store, NewOptions(nil, svc, Options{Store: store, Runs: store, study: study})
}

func addLearner(t *testing.T, store *db.Store, email, sub string, now time.Time) learnerFix {
	t.Helper()
	user, err := store.CreateUser(context.Background(), email, sub, auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 32)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(id)
	csrf := "csrf-" + email
	if err := store.InsertSession(context.Background(), hex.EncodeToString(sum[:]), user.ID, csrf, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	raw := base64.RawURLEncoding.EncodeToString(id)
	return learnerFix{user: user, csrf: csrf, cookie: &http.Cookie{Name: "hebrew_session", Value: raw}}
}

func addLexeme(t *testing.T, store *db.Store, text, gloss, translit, gender, audio string) int64 {
	t.Helper()
	id, err := testharness.InsertLexeme(context.Background(), store, text)
	if err != nil {
		t.Fatal(err)
	}
	var audioID any
	if audio != "" {
		audioID = audio
	}
	if _, err := store.ExecContext(context.Background(), `
		UPDATE lexemes SET gloss_en = ?, translit = ?, gender = ?, audio_id = ? WHERE id = ?`,
		gloss, translit, gender, audioID, id); err != nil {
		t.Fatal(err)
	}
	return id
}

func addCard(t *testing.T, store *db.Store, userID, lexemeID int64, skill string, due time.Time, stability float64, rating int, reviewed time.Time) int64 {
	t.Helper()
	id, err := store.InsertCard(context.Background(), db.Card{
		UserID:     userID,
		LexemeID:   lexemeID,
		Skill:      skill,
		State:      fsrs.StateReview,
		Due:        due,
		Stability:  stability,
		Difficulty: 5,
		Reps:       1,
		Lapses:     0,
		LastReview: reviewed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rating != 0 {
		insertReview(t, store, id, rating, reviewed)
	}
	return id
}

func insertReview(t *testing.T, store *db.Store, cardID int64, rating int, at time.Time) {
	t.Helper()
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked)
		VALUES (?, ?, ?, 0, 0, 0)`, cardID, rating, db.FormatTime(at)); err != nil {
		t.Fatal(err)
	}
}

func publishUnit(t *testing.T, store *db.Store, id int64, position int, title, situation, doc, status string) {
	t.Helper()
	if _, err := store.ExecContext(context.Background(), `
		UPDATE units SET position = ?, title = ?, situation = ?, doc = ?, status = ? WHERE id = ?`,
		position, title, situation, doc, status, id); err != nil {
		t.Fatal(err)
	}
}

func getLearner(t *testing.T, h http.Handler, learner learnerFix, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(learner.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d\n%s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func postGrade(t *testing.T, h http.Handler, learner learnerFix, cardID int64, rating, origin string) *httptest.ResponseRecorder {
	t.Helper()
	return postGradeRaw(t, h, learner.cookie, learner.csrf, cardID, rating, origin)
}

func gradeRevealed(t *testing.T, h http.Handler, learner learnerFix, cardID int64, rating, origin string) *httptest.ResponseRecorder {
	t.Helper()
	shown := postReveal(t, h, learner, cardID, origin)
	if shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d\n%s", shown.Code, shown.Body.String())
	}
	return postGrade(t, h, learner, cardID, rating, origin)
}

func postGradeRaw(t *testing.T, h http.Handler, cookie *http.Cookie, csrf string, cardID int64, rating, origin string) *httptest.ResponseRecorder {
	t.Helper()
	return postGradeRawHX(t, h, cookie, csrf, cardID, rating, origin, true)
}

func postGradeRawHX(t *testing.T, h http.Handler, cookie *http.Cookie, csrf string, cardID int64, rating, origin string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	req := gradeRequest(t, cookie, csrf, cardID, rating, origin, hx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func gradeRequest(t *testing.T, cookie *http.Cookie, csrf string, cardID int64, rating, origin string, hx bool) *http.Request {
	t.Helper()
	form := url.Values{}
	form.Set("card_id", fmt.Sprint(cardID))
	form.Set("rating", rating)
	if csrf != "" {
		form.Set("_csrf", csrf)
	}
	req := httptest.NewRequest(http.MethodPost, "/_hx/review/grade", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

func postReveal(t *testing.T, h http.Handler, learner learnerFix, cardID int64, origin string) *httptest.ResponseRecorder {
	t.Helper()
	return postRevealRaw(t, h, learner.cookie, learner.csrf, cardID, origin, true)
}

func postRevealRaw(t *testing.T, h http.Handler, cookie *http.Cookie, csrf string, cardID int64, origin string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("card_id", fmt.Sprint(cardID))
	if csrf != "" {
		form.Set("_csrf", csrf)
	}
	req := httptest.NewRequest(http.MethodPost, "/_hx/review/reveal", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func mustLogs(t *testing.T, store *db.Store, cardID int64) []db.ReviewLog {
	t.Helper()
	logs, err := store.ReviewLogs(context.Background(), cardID)
	if err != nil {
		t.Fatal(err)
	}
	return logs
}

func mustInt(t *testing.T, s string) int64 {
	t.Helper()
	var n int64
	if _, err := fmt.Sscan(s, &n); err != nil {
		t.Fatal(err)
	}
	return n
}

func attr(t *testing.T, body, name string) string {
	t.Helper()
	m := regexp.MustCompile(regexp.QuoteMeta(name) + `="([^"]*)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("missing %s in %s", name, body)
	}
	return m[1]
}

func articleAttrs(body string) string {
	m := regexp.MustCompile(`<article class="today"[^>]*>`).FindString(body)
	if m == "" {
		return body
	}
	return m
}

func cardAttrs(body string) string {
	m := regexp.MustCompile(`<div id="review-card"[^>]*>`).FindString(body)
	if m == "" {
		return body
	}
	return m
}
