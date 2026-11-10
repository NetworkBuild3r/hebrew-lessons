package main

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// INIT-038/SPEC-004: hebrew void-evidence. Temp databases only. The dry run
// is the check that a production copy would be left unchanged.

const (
	secretResponse = "SECRET_RESPONSE_DO_NOT_PRINT"
	secretPrompt   = "SECRET_PROMPT_DO_NOT_PRINT"
)

func TestVoidEvidenceDryRunPrintsCountsAndWritesNothing(t *testing.T) {
	path := voidFixture(t, false)
	store := openVoidDB(t, path)
	before := evidenceDigest(t, store)
	_ = store.Close()

	var out, errOut bytes.Buffer
	if err := voidEvidenceCommand([]string{"--db", path}, &out, &errOut); err != nil {
		t.Fatalf("err = %v\n%s%s", err, out.String(), errOut.String())
	}
	got := out.String()
	for _, want := range []string{
		"mode=dry-run",
		"blank_method=matched",
		"matched_answers=2",
		"fallback_answers=2",
		"answers=4",
		"reason=no_cue_blank word=word-blank skill=read count=2",
		"reason=unit_12_34 word=word-12 skill=read count=1",
		"reason=unit_12_34 word=word-34 skill=recall count=1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	for _, hidden := range []string{secretResponse, secretPrompt, "blank line one", "word-cued", "word-11", "word-35", "word-shared", "word-other"} {
		if strings.Contains(got, hidden) {
			t.Errorf("output contains %q:\n%s", hidden, got)
		}
	}
	store = openVoidDB(t, path)
	if after := evidenceDigest(t, store); after != before {
		t.Fatalf("dry run wrote\n before %s\n after  %s", before, after)
	}
}

func TestVoidEvidenceAmbiguousMatchUsesEveryWrongBlank(t *testing.T) {
	path := voidFixture(t, true)
	var out, errOut bytes.Buffer
	if err := voidEvidenceCommand([]string{"--db", path}, &out, &errOut); err != nil {
		t.Fatalf("err = %v\n%s%s", err, out.String(), errOut.String())
	}
	got := out.String()
	for _, want := range []string{
		"blank_method=fallback",
		"matched_answers=2",
		"fallback_answers=4",
		"answers=4",
		"reason=no_cue_blank word=word-cued skill=read count=1",
		"reason=no_cue_blank word=word-multi skill=read count=1",
		"reason=no_cue_blank word=word-nocue-wrong skill=read count=1",
		"reason=no_cue_blank word=word-rev skill=read count=1",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	for _, hidden := range []string{"word-nocue-right", "word-right", secretResponse, "same line", "no cue line"} {
		if strings.Contains(got, hidden) {
			t.Errorf("output contains %q:\n%s", hidden, got)
		}
	}
	store := openVoidDB(t, path)
	if n := countSQL(t, store, `SELECT COUNT(*) FROM answer_voids`); n != 0 {
		t.Fatalf("dry run wrote %d voids", n)
	}
}

func TestVoidEvidenceApplyWithoutBackupRefuses(t *testing.T) {
	path := voidFixture(t, false)
	store := openVoidDB(t, path)
	before := evidenceDigest(t, store)
	_ = store.Close()

	var out, errOut bytes.Buffer
	err := voidEvidenceCommand([]string{"--apply", "--db", path}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "backup-taken") {
		t.Fatalf("err = %v, want a backup-taken refusal\n%s%s", err, out.String(), errOut.String())
	}
	store = openVoidDB(t, path)
	if after := evidenceDigest(t, store); after != before {
		t.Fatalf("refused apply wrote\n before %s\n after  %s", before, after)
	}
	err = voidEvidenceCommand([]string{"--undo", "batch-x", "--db", path}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "backup-taken") {
		t.Fatalf("undo err = %v, want a backup-taken refusal", err)
	}
}

func TestVoidEvidenceApplyAndUndoOnAFixture(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	path := filepath.Join(dir, "hebrew.sqlite")
	ctx := context.Background()
	store, err := db.Open(ctx, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	user, err := store.InsertUser(ctx, "void-apply@example.invalid", "sub-void-apply", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if user != 1 {
		t.Fatalf("learner id = %d, want 1", user)
	}
	if err := store.SetUserTimeZone(ctx, user, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	loc, err := studyday.ParseZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	clk := &voidClock{t: time.Date(2026, 10, 21, 16, 0, 0, 0, loc)}
	rec := evidence.New(store, nil, clk)
	lex := insertLexeme(t, store, "word-12")
	unit := insertVoidUnit(t, store, 12, db.LearnerScope(1), `{"title":"t","lessons":[]}`)
	for i := 0; i < 2; i++ {
		if _, err := rec.Attempt(ctx, evidence.Attempt{
			UserID: user, LexemeID: lex, Skill: fsrs.SkillRead, Activity: "read_pick",
			Source: evidence.SourceReview, UnitID: unit, Prompt: secretPrompt, Response: secretResponse,
			Correct: i == 0, Latency: 2 * time.Second,
		}); err != nil {
			t.Fatal(err)
		}
		clk.t = clk.t.Add(48 * time.Hour)
	}
	before, err := store.GetCard(ctx, mustCardID(t, store, user, lex))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if err := voidEvidenceCommand([]string{"--db", path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "mode=dry-run") || !strings.Contains(out.String(), "answers=2") {
		t.Fatalf("dry run:\n%s", out.String())
	}
	if strings.Contains(out.String(), secretResponse) || strings.Contains(out.String(), secretPrompt) {
		t.Fatalf("dry run printed answer text:\n%s", out.String())
	}

	out.Reset()
	errOut.Reset()
	if err := voidEvidenceCommand([]string{"--apply", "--backup-taken", "--db", path}, &out, &errOut); err != nil {
		t.Fatalf("apply: %v\n%s%s", err, out.String(), errOut.String())
	}
	batch := batchFromOutput(t, out.String())
	store = openVoidDB(t, path)
	if n := countSQL(t, store, `SELECT COUNT(*) FROM answer_voids WHERE undone_at IS NULL`); n != 2 {
		t.Fatalf("voids = %d, want 2", n)
	}
	card, err := store.GetCard(ctx, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if card.State != fsrs.StateNew || card.Reps != 0 || card.Lapses != 0 {
		t.Fatalf("card after voiding both answers = %+v, want a new card", card)
	}
	_ = store.Close()

	out.Reset()
	if err := voidEvidenceCommand([]string{"--undo", batch, "--backup-taken", "--db", path}, &out, &errOut); err != nil {
		t.Fatalf("undo: %v\n%s", err, errOut.String())
	}
	if !strings.Contains(out.String(), "mode=undo") || !strings.Contains(out.String(), "batch="+batch) {
		t.Fatalf("undo output:\n%s", out.String())
	}
	store = openVoidDB(t, path)
	restored, err := store.GetCard(ctx, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.State != before.State || restored.Stability != before.Stability || restored.Difficulty != before.Difficulty ||
		restored.Reps != before.Reps || restored.Lapses != before.Lapses || !restored.Due.Equal(before.Due) || !restored.LastReview.Equal(before.LastReview) {
		t.Fatalf("undo\n got %+v\nwant %+v", restored, before)
	}
}

type voidClock struct{ t time.Time }

func (c *voidClock) Now() time.Time { return c.t }

func batchFromOutput(t *testing.T, out string) string {
	t.Helper()
	for _, field := range strings.Fields(out) {
		if strings.HasPrefix(field, "batch=void-") {
			return strings.TrimPrefix(field, "batch=")
		}
	}
	t.Fatalf("no batch id in\n%s", out)
	return ""
}

func mustCardID(t *testing.T, store *db.Store, user, lex int64) int64 {
	t.Helper()
	id, ok, err := store.CardID(context.Background(), user, lex, fsrs.SkillRead)
	if err != nil || !ok {
		t.Fatalf("card id ok=%v err=%v", ok, err)
	}
	return id
}

func openVoidDB(t *testing.T, path string) *db.Store {
	t.Helper()
	store, err := db.Open(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func evidenceDigest(t *testing.T, store *db.Store) string {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, table := range []string{"attempts", "review_log", "confusion_misses", "answer_voids", "cards"} {
		fmt.Fprintf(&b, "%s=%d\n", table, countSQL(t, store, `SELECT COUNT(*) FROM `+table))
	}
	rows, err := conn.Query(`
		SELECT id, state, due, stability, difficulty, reps, lapses, COALESCE(last_review, '')
		FROM cards ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var state, due, last string
		var stability, difficulty float64
		var reps, lapses int
		if err := rows.Scan(&id, &state, &due, &stability, &difficulty, &reps, &lapses, &last); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&b, "card %d %s %s %.17g %.17g %d %d %s\n", id, state, due, stability, difficulty, reps, lapses, last)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func voidFixture(t *testing.T, ambiguous bool) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	path := filepath.Join(dir, "hebrew.sqlite")
	ctx := context.Background()
	store, err := db.Open(ctx, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	user, err := store.InsertUser(ctx, "void-dry@example.invalid", "sub-void-dry", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if user != voidLearnerID {
		t.Fatalf("learner id = %d, want %d", user, voidLearnerID)
	}
	other, err := store.InsertUser(ctx, "void-other@example.invalid", "sub-void-other", "learner")
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)
	if ambiguous {
		seedAmbiguousBlanks(t, store, user, at)
		return path
	}
	word12 := insertLexeme(t, store, "word-12")
	word34 := insertLexeme(t, store, "word-34")
	wordBlank := insertLexeme(t, store, "word-blank")
	wordCued := insertLexeme(t, store, "word-cued")
	word11 := insertLexeme(t, store, "word-11")
	word35 := insertLexeme(t, store, "word-35")
	wordShared := insertLexeme(t, store, "word-shared")
	wordOther := insertLexeme(t, store, "word-other")

	u12 := insertVoidUnit(t, store, 12, db.LearnerScope(1), `{"title":"12","lessons":[{"kind":"lesson","activities":[{"type":"recall","text":"word-12"}]}]}`)
	u34 := insertVoidUnit(t, store, 34, db.LearnerScope(1), `{"title":"34","lessons":[{"kind":"lesson","activities":[{"type":"recall","text":"word-34"}]}]}`)
	u11 := insertVoidUnit(t, store, 11, db.LearnerScope(1), `{"title":"11","lessons":[]}`)
	u35 := insertVoidUnit(t, store, 35, db.LearnerScope(1), `{"title":"35","lessons":[]}`)
	shared := insertVoidUnit(t, store, 20, "shared", `{"title":"shared","lessons":[]}`)
	blank := insertVoidUnit(t, store, 3, db.LearnerScope(1), `{
		"title":"blank","lessons":[{"kind":"lesson","activities":[
			{"type":"cloze","text":"blank line one","translation":""},
			{"type":"cloze","text":"cued line","translation":"I am here"}
		]}]
	}`)

	insertVoidAttempt(t, store, user, word12, "read", "recall", "lesson", u12, true, at)
	insertVoidAttempt(t, store, user, word34, "recall", "recall", "lesson", u34, false, at.Add(time.Minute))
	insertVoidAttempt(t, store, user, word11, "read", "recall", "lesson", u11, false, at.Add(2*time.Minute))
	insertVoidAttempt(t, store, user, word35, "read", "recall", "lesson", u35, false, at.Add(3*time.Minute))
	insertVoidAttempt(t, store, user, wordShared, "read", "recall", "lesson", shared, false, at.Add(4*time.Minute))
	insertVoidAttempt(t, store, other, wordOther, "read", "recall", "lesson", u12, false, at.Add(5*time.Minute))
	insertCloze(t, store, user, wordBlank, blank, "blank line one", true, at.Add(6*time.Minute))
	insertCloze(t, store, user, wordBlank, blank, "blank line one", false, at.Add(7*time.Minute))
	insertCloze(t, store, user, wordCued, blank, "cued line", false, at.Add(8*time.Minute))
	return path
}

func seedAmbiguousBlanks(t *testing.T, store *db.Store, user int64, at time.Time) {
	t.Helper()
	multi := insertLexeme(t, store, "word-multi")
	rev := insertLexeme(t, store, "word-rev")
	cued := insertLexeme(t, store, "word-cued")
	right := insertLexeme(t, store, "word-right")
	nocueWrong := insertLexeme(t, store, "word-nocue-wrong")
	nocueRight := insertLexeme(t, store, "word-nocue-right")

	multiUnit := insertVoidUnit(t, store, 4, db.LearnerScope(1), `{
		"title":"multi","lessons":[{"kind":"lesson","activities":[
			{"type":"cloze","text":"same line","translation":""},
			{"type":"cloze","text":"same line","translation":"hello"}
		]}]
	}`)
	revUnit := insertVoidUnit(t, store, 5, db.LearnerScope(1), `{
		"title":"rev","lessons":[{"kind":"lesson","activities":[
			{"type":"cloze","text":"rev line","translation":"hello"}
		]}]
	}`)
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO unit_revisions (unit_id, rev, doc, actor, created_at)
		VALUES (?, 1, ?, 'test', ?)`,
		revUnit, `{"title":"rev","lessons":[{"kind":"lesson","activities":[{"type":"cloze","text":"rev line","translation":""}]}]}`,
		db.FormatTime(at)); err != nil {
		t.Fatal(err)
	}
	clear := insertVoidUnit(t, store, 6, db.LearnerScope(1), `{
		"title":"clear","lessons":[{"kind":"lesson","activities":[
			{"type":"cloze","text":"cued line","translation":"I am here"},
			{"type":"cloze","text":"no cue line","translation":""}
		]}]
	}`)
	insertCloze(t, store, user, multi, multiUnit, "same line", false, at)
	insertCloze(t, store, user, rev, revUnit, "rev line", false, at.Add(time.Minute))
	insertCloze(t, store, user, cued, clear, "cued line", false, at.Add(2*time.Minute))
	insertCloze(t, store, user, right, clear, "cued line", true, at.Add(3*time.Minute))
	insertCloze(t, store, user, nocueWrong, clear, "no cue line", false, at.Add(4*time.Minute))
	insertCloze(t, store, user, nocueRight, clear, "no cue line", true, at.Add(5*time.Minute))
}

func insertVoidUnit(t *testing.T, store *db.Store, position int, scope, doc string) int64 {
	t.Helper()
	res, err := store.ExecContext(context.Background(), `
		INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
		VALUES (?, 't', 's', 'c', 'published', 1, ?, ?)`, position, doc, scope)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertVoidAttempt(t *testing.T, store *db.Store, user, lex int64, skill, activity, source string, unit int64, correct bool, at time.Time) {
	t.Helper()
	right := 0
	if correct {
		right = 1
	}
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO attempts (
			user_id, lexeme_id, skill, activity, source, unit_id, prompt, response,
			correct, dont_know, hint_used, latency_ms, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 1000, ?)`,
		user, lex, skill, activity, source, unit, secretPrompt, secretResponse, right, db.FormatTime(at),
	); err != nil {
		t.Fatal(err)
	}
}

func insertCloze(t *testing.T, store *db.Store, user, lex, unit int64, prompt string, correct bool, at time.Time) {
	t.Helper()
	right := 0
	if correct {
		right = 1
	}
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO attempts (
			user_id, lexeme_id, skill, activity, source, unit_id, prompt, response,
			correct, dont_know, hint_used, latency_ms, created_at
		) VALUES (?, ?, 'read', 'cloze', 'lesson', ?, ?, ?, ?, 0, 0, 1000, ?)`,
		user, lex, unit, prompt, secretResponse, right, db.FormatTime(at),
	); err != nil {
		t.Fatal(err)
	}
}
