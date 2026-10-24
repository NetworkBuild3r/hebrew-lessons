package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestMyStuffIsolationAndSections(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	a := addLearner(t, store, "a@example.com", "sub-a", now)
	b := addLearner(t, store, "b@example.com", "sub-b", now)
	ctx := context.Background()

	home := getLearner(t, h, a, "/")
	if strings.Contains(home, `class="section-jump"`) || strings.Contains(home, `class="rail-more"`) {
		t.Fatal("empty sections were shown")
	}
	if dockLinks(home) != 5 || !strings.Contains(home, ">My stuff<") {
		t.Fatalf("dock = %d", dockLinks(home))
	}
	if code := learnerStatus(t, h, a, "/isaiah"); code != http.StatusNotFound {
		t.Fatalf("isaiah = %d", code)
	}

	root := "\u05e9\u05c1\u05be\u05de\u05be\u05e2"
	hear := units.Normalize("\u05e9\u05b8\u05c1\u05de\u05b7\u05e2")
	past := units.Normalize("\u05e9\u05b8\u05c1\u05de\u05b7\u05e2\u05b0\u05ea\u05bc\u05b4\u05d9")
	lemma := insertLexemeRow(t, store, hear, "hear", "verb", root)
	other := insertLexemeRow(t, store, past, "I heard", "verb", root)
	knowWord(t, store, a.user.ID, lemma, now)
	if err := store.UpsertVerbForms(ctx, []db.VerbFormInput{
		{LemmaID: lemma, Tense: "present", Person: "1", Number: "sg", Gender: "m", FormHe: hear, FormLexemeID: lemma},
		{LemmaID: lemma, Tense: "past", Person: "1", Number: "sg", FormHe: past, FormLexemeID: other},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPatterns(ctx, []db.PatternInput{{
		Slug: "present-tense", Title: "Present tense", Summary: "The verb agrees.",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertExplainer(ctx, db.Explainer{
		Slug: "present-tense", Title: "Present tense", Body: "The verb agrees.", Status: "published",
	}); err != nil {
		t.Fatal(err)
	}
	personal := units.Normalize("\u05d0\u05b2\u05e0\u05b4\u05d9 \u05d4\u05d5\u05b9\u05dc\u05b5\u05da\u05b0")
	commonWrong := units.Normalize("\u05d0\u05b2\u05e0\u05b4\u05d9 \u05d4\u05d5\u05b9\u05dc\u05b6\u05db\u05b6\u05ea")
	secret := "SECRETPAIR"
	if err := store.RecordMistakes(ctx, a.user.ID, []db.MistakeInput{
		{Wrong: personal, Right: hear, Note: "agreement", Pattern: "present-tense", Source: "tutor"},
	}); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := store.RecordMistakes(ctx, a.user.ID, []db.MistakeInput{
			{Wrong: past, Right: hear, Source: "tutor"},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.RecordMistakes(ctx, b.user.ID, []db.MistakeInput{
		{Wrong: secret, Right: "SECRET-RIGHT", Source: "app"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertCommonMistakes(ctx, []db.CommonMistakeInput{
		{Pattern: "present-tense", Wrong: commonWrong, Right: hear, Explain: "Use the present.", Status: "published"},
		{Pattern: "present-tense", Wrong: secret, Right: hear, Explain: "A draft.", Status: "draft"},
	}); err != nil {
		t.Fatal(err)
	}
	isaiahHe := units.Normalize("\u05e7\u05b8\u05d3\u05d5\u05b9\u05e9\u05c1")
	if _, err := store.ExecContext(ctx, `
		INSERT INTO tanakh_verses (book, chapter, verse, he, he_plain, en, source_version, license)
		VALUES ('isaiah', 6, 1, ?, ?, 'Holy, holy, holy.', 'test', 'Public Domain')`,
		isaiahHe, units.Plain(isaiahHe)); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertVerseNotes(ctx, "isaiah", 6, 1, []db.VerseNoteInput{
		{Layer: db.LayerRoots, Body: "In the temple."},
		{Layer: db.LayerBridge, Body: "DRAFTNOTE stays hidden."},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CheckVerseNote(ctx, "isaiah", 6, 1, db.LayerRoots, a.user.ID); err != nil {
		t.Fatal(err)
	}

	home = getLearner(t, h, a, "/")
	if dockLinks(home) != 5 {
		t.Fatalf("dock grew to %d", dockLinks(home))
	}
	for _, href := range []string{`href="/learn"`, `href="/roots"`, `href="/isaiah"`, `class="section-jump"`, `class="rail-more"`} {
		if !strings.Contains(home, href) {
			t.Errorf("home lacks %s", href)
		}
	}
	me := getLearner(t, h, a, "/me")
	for _, label := range []string{">Words<", ">Verbs<", ">Mistakes<", ">You can now say<"} {
		if !strings.Contains(me, label) {
			t.Errorf("me lacks %s", label)
		}
	}

	words := getLearner(t, h, a, "/me/words")
	if !strings.Contains(words, hear) || strings.Contains(words, past) {
		t.Fatal("word bank did not keep to words the learner has met")
	}
	known := getLearner(t, h, a, "/me/words?state=known")
	if !strings.Contains(known, hear) {
		t.Fatal("known filter dropped the known word")
	}
	if !strings.Contains(known, `href="/roots/`) {
		t.Fatal("a visible root was not linked")
	}

	verbs := getLearner(t, h, a, "/me/verbs")
	if strings.Count(verbs, `data-known="true"`) != 1 || strings.Count(verbs, `data-known="false"`) != 1 {
		t.Fatalf("verb marks:\n%s", verbs)
	}
	if !strings.Contains(verbs, `class="stuff-scroll"`) {
		t.Fatal("the verb table does not scroll inside its card")
	}

	mistakes := getLearner(t, h, a, "/me/mistakes")
	if strings.Contains(mistakes, secret) || !strings.Contains(mistakes, personal) {
		t.Fatal("mistakes were not kept to the signed-in learner")
	}
	if pastAt, personalAt := strings.Index(mistakes, past), strings.Index(mistakes, personal); pastAt < 0 || personalAt < 0 || pastAt > personalAt {
		t.Fatal("mistakes were not ordered by times")
	}
	if !strings.Contains(mistakes, `href="/learn/present-tense"`) {
		t.Fatal("a patterned mistake did not link its explainer")
	}

	note := getLearner(t, h, a, "/learn/present-tense")
	if !strings.Contains(note, "Learners often say") || !strings.Contains(note, commonWrong) || strings.Contains(note, personal) || strings.Contains(note, "A draft.") {
		t.Fatal("the explainer did not stay on published shared mistakes")
	}
	isaiah := getLearner(t, h, a, "/isaiah")
	if !strings.Contains(isaiah, "Chapter 6") || strings.Contains(isaiah, "DRAFTNOTE") {
		t.Fatal("isaiah chapter list showed a draft or hid the chapter")
	}
	verse := getLearner(t, h, a, "/isaiah/6/1")
	if !strings.Contains(verse, isaiahHe) || !strings.Contains(verse, "In the temple.") || strings.Contains(verse, "DRAFTNOTE") {
		t.Fatal("the reader showed a draft or hid the checked note")
	}
}

func dockLinks(body string) int {
	nav := regexp.MustCompile(`(?s)<nav class="dock".*?</nav>`).FindString(body)
	return strings.Count(nav, `class="dock__link"`)
}

func learnerStatus(t *testing.T, h http.Handler, learner learnerFix, path string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(learner.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func insertLexemeRow(t *testing.T, store *db.Store, text, gloss, pos, root string) int64 {
	t.Helper()
	res, err := store.ExecContext(context.Background(),
		`INSERT INTO lexemes (text_nfc, gloss_en, pos, root) VALUES (?, ?, ?, ?)`, text, gloss, pos, root)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func knowWord(t *testing.T, store *db.Store, user, lexeme int64, now time.Time) {
	t.Helper()
	ctx := context.Background()
	for i := range 3 {
		at := now.Add(-time.Hour).Add(-time.Duration(2-i) * time.Minute)
		if _, err := store.ExecContext(ctx, `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, 'read', 'test', 'review', '', '', 1, 0, 0, 0, ?)`, user, lexeme, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}
	last := now.Add(-24 * time.Hour)
	id, err := store.InsertCard(ctx, db.Card{
		UserID: user, LexemeID: lexeme, Skill: fsrs.SkillRead, State: fsrs.StateReview,
		Due: now, Stability: 10, Difficulty: 5, Reps: 3, LastReview: last,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		at := last.Add(-time.Duration(2-i) * 24 * time.Hour)
		if _, err := store.ExecContext(ctx, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 0, 1)`, id, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}
}
