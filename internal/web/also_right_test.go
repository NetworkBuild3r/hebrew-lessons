package web

import (
	"bytes"
	"context"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
	"github.com/a-h/templ"
)

// INIT-038/SPEC-009: a wrong graded answer can be taken back, and a report
// voids that learner's wrong answers in the unit.

func TestAlsoRightRestoresTheCardBeforeTheAnswer(t *testing.T) {
	start := time.Date(2026, 10, 21, 16, 0, 0, 0, time.UTC)
	clk := &stepClock{t: start}
	store, h := openShell(t, clk, nil, nil)
	a := addLearner(t, store, "a@example.com", "sub-a", start.Add(200*time.Hour))
	b := addLearner(t, store, "b@example.com", "sub-b", start.Add(200*time.Hour))
	lex := addLexeme(t, store, "כֶּלֶב", "dog", "kelev", "m", "")
	ctx := context.Background()
	rec := evidence.New(store, fsrs.New(store, clk), clk)
	first := evidence.Attempt{
		UserID: a.user.ID, LexemeID: lex, Skill: fsrs.SkillRead, Activity: "read_pick",
		Source: evidence.SourceReview, Prompt: "dog", Response: "dog", Correct: true,
	}
	if _, err := rec.Attempt(ctx, first); err != nil {
		t.Fatal(err)
	}
	cardID, ok, err := store.CardID(ctx, a.user.ID, lex, fsrs.SkillRead)
	if err != nil || !ok {
		t.Fatalf("card after the first answer ok=%v err=%v", ok, err)
	}
	before, err := store.GetCard(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	clk.Advance(48 * time.Hour)
	second := first
	second.Correct = false
	second.Response = "cat"
	if _, err := rec.Attempt(ctx, second); err != nil {
		t.Fatal(err)
	}
	ids := attemptIDs(t, store, a.user.ID)
	if len(ids) != 2 {
		t.Fatalf("attempts = %v", ids)
	}
	rightID, wrongID := ids[0], ids[1]
	moved, err := store.GetCard(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	if cardScheduleEqual(before, moved) {
		t.Fatal("the wrong answer did not move the card, so the test proves nothing")
	}

	path := "/_hx/also-right"
	foreign := formPost(t, h, b, path, url.Values{"attempt": {itoa(wrongID)}, "_csrf": {b.csrf}}, nil)
	if foreign.Code != http.StatusForbidden {
		t.Fatalf("another learner = %d", foreign.Code)
	}
	noCSRF := formPost(t, h, a, path, url.Values{"attempt": {itoa(wrongID)}}, nil)
	if noCSRF.Code != http.StatusForbidden {
		t.Fatalf("no csrf = %d", noCSRF.Code)
	}
	right := formPost(t, h, a, path, url.Values{"attempt": {itoa(rightID)}, "_csrf": {a.csrf}}, nil)
	if right.Code != http.StatusBadRequest {
		t.Fatalf("a right answer = %d", right.Code)
	}
	if n := voidCount(t, store); n != 0 {
		t.Fatalf("refused taps wrote %d voids", n)
	}
	if got, err := store.GetCard(ctx, cardID); err != nil || !cardScheduleEqual(got, moved) {
		t.Fatal("a refused tap changed the card")
	}

	tap := formPost(t, h, a, path, url.Values{"attempt": {itoa(wrongID)}, "_csrf": {a.csrf}}, nil)
	if tap.Code != http.StatusOK || !strings.Contains(tap.Body.String(), views.AlsoRightTaken) {
		t.Fatalf("tap = %d\n%s", tap.Code, tap.Body.String())
	}
	got, err := store.GetCard(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	if !cardScheduleEqual(got, before) {
		t.Fatalf("card\n got %s\nbefore %s", formatTestCard(got), formatTestCard(before))
	}
	if n := attemptCount(t, store); n != 2 {
		t.Fatalf("attempts = %d, want 2; a void does not delete the row", n)
	}
	if correct := attemptCorrect(t, store, wrongID); correct != 0 {
		t.Fatalf("wrong attempt correct = %d; a void must not credit it", correct)
	}
	rows := voidRows(t, store, wrongID)
	if len(rows) != 1 || rows[0].reason != db.VoidReasonAlsoRight || rows[0].actor != db.LearnerScope(a.user.ID) {
		t.Fatalf("void rows = %+v", rows)
	}
	if n := voidCount(t, store); n != 1 {
		t.Fatalf("void rows = %d, want 1", n)
	}
}

func TestAlsoRightButtonOnlyAfterAWrongGrade(t *testing.T) {
	render := func(c templ.Component) string {
		t.Helper()
		var buf bytes.Buffer
		if err := c.Render(context.Background(), &buf); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	wrongLesson := render(views.ActivityBlock(views.Activity{
		Feedback: "Not quite. Here it is.", FeedbackClass: "feedback feedback--wrong",
		AlsoRightID: 7, CSRF: "t", Run: "1", Post: "/lesson/1/0", Continue: "Continue",
	}))
	if !strings.Contains(wrongLesson, views.AlsoRightLabel) || !strings.Contains(wrongLesson, `value="7"`) {
		t.Fatalf("wrong lesson feedback:\n%s", wrongLesson)
	}
	rightLesson := render(views.ActivityBlock(views.Activity{
		Feedback: "That's right.", FeedbackClass: "feedback feedback--right",
		AlsoRightID: 7, CSRF: "t", Run: "1", Post: "/lesson/1/0", Continue: "Continue",
	}))
	if strings.Contains(rightLesson, views.AlsoRightLabel) {
		t.Fatalf("right lesson shows the tap:\n%s", rightLesson)
	}
	reveal := render(views.ActivityBlock(views.Activity{
		Feedback: "", CSRF: "t", Run: "1", Post: "/lesson/1/0", Continue: "Continue", AlsoRightID: 7,
	}))
	if strings.Contains(reveal, views.AlsoRightLabel) {
		t.Fatalf("ungraded reveal shows the tap:\n%s", reveal)
	}

	wrongReview := render(views.ActivityFrame(views.ActivityView{
		Kind: views.KindReadPick, Phase: views.ActFeedback, CSRF: "t", CardID: 4,
		Feedback: &views.FeedbackView{Correct: false, Title: "Not quite", AlsoRightID: 42, Hebrew: "כֶּלֶב", Gloss: "dog"},
	}))
	if !strings.Contains(wrongReview, views.AlsoRightLabel) || !strings.Contains(wrongReview, `value="42"`) {
		t.Fatalf("wrong review feedback:\n%s", wrongReview)
	}
	rightReview := render(views.ActivityFrame(views.ActivityView{
		Kind: views.KindReadPick, Phase: views.ActFeedback, CSRF: "t", CardID: 4,
		Feedback: &views.FeedbackView{Correct: true, Title: "Correct", AlsoRightID: 42},
	}))
	if strings.Contains(rightReview, views.AlsoRightLabel) {
		t.Fatalf("right review shows the tap:\n%s", rightReview)
	}
	ask := render(views.ActivityFrame(views.ActivityView{
		Kind: views.KindReadPick, Phase: views.ActAsk, CSRF: "t", CardID: 4, Lead: "What does it mean?",
	}))
	if strings.Contains(ask, views.AlsoRightLabel) {
		t.Fatalf("an unanswered card shows the tap:\n%s", ask)
	}

	graded := feedbackView(activity.Item{Kind: activity.ReadPick}, &answerState{
		res: activity.Result{Correct: false}, attemptID: 9,
	})
	if graded.AlsoRightID != 9 {
		t.Fatalf("wrong grade attempt = %d", graded.AlsoRightID)
	}
	if got := feedbackView(activity.Item{Kind: activity.ReadPick}, &answerState{
		res: activity.Result{Correct: true}, attemptID: 9,
	}); got.AlsoRightID != 0 {
		t.Fatalf("right grade exposed attempt %d", got.AlsoRightID)
	}
	taken := feedbackView(activity.Item{Kind: activity.ReadPick}, &answerState{
		res: activity.Result{Correct: false}, attemptID: 9, takenBack: true,
	})
	if taken.AlsoRightID != 0 || taken.Title != views.AlsoRightTaken {
		t.Fatalf("taken back = %+v", taken)
	}
}

func TestAlsoRightLessonTapAndUngradedReveal(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	ctx := context.Background()
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
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
	earlierID := putLesson(t, store, nil, units.Unit{
		Title: "Earlier", Position: 0, NewWords: []string{"א", "ב", "ג"},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: "א ב ג"},
		}}},
	}, "shared")
	clozeID := putLesson(t, store, nil, units.Unit{
		Title: "Cloze", Position: 1, NewWords: []string{"ד"},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityCloze, Text: "ד ___", Translation: "dalet something"},
		}}},
	}, "shared")
	recallID := putLesson(t, store, nil, units.Unit{
		Title: "Recall", Position: 2, NewWords: []string{"ד"},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: "ד"},
		}}},
	}, "shared")
	publish(t, store, earlierID, clozeID, recallID)
	when := db.FormatTime(now)
	if _, err := store.ExecContext(ctx, `
		INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score)
		VALUES (?, ?, 0, ?, ?, 1)`, learner.user.ID, earlierID, when, when); err != nil {
		t.Fatal(err)
	}

	clozePath := "/lesson/" + itoa(clozeID) + "/0"
	cloze := getLearner(t, h, learner, clozePath)
	if strings.Contains(cloze, views.AlsoRightLabel) || strings.Contains(cloze, "feedback--wrong") {
		t.Fatalf("ungraded cloze reveal:\n%s", cloze)
	}
	clozeRun := mustRunID(t, cloze)
	done := postLesson(t, h, learner.cookie, clozePath, itoa(clozeRun), "continue", true, learner.csrf)
	if done.Code != http.StatusOK {
		t.Fatalf("continue cloze = %d\n%s", done.Code, done.Body.String())
	}

	page := getLearner(t, h, learner, "/lesson/"+itoa(recallID)+"/0")
	if strings.Contains(page, views.AlsoRightLabel) {
		t.Fatal("the ask shows the tap before an answer")
	}
	runID := mustRunID(t, page)
	wrongPick := ""
	for _, id := range choiceValues(t, page) {
		if id != itoa(ids["ד"]) {
			wrongPick = id
			break
		}
	}
	if wrongPick == "" {
		t.Fatalf("no wrong button:\n%s", page)
	}
	graded := postLesson(t, h, learner.cookie, "/lesson/"+itoa(recallID)+"/0", itoa(runID), wrongPick, true, learner.csrf)
	body := graded.Body.String()
	if graded.Code != http.StatusOK || !strings.Contains(body, "feedback--wrong") || !strings.Contains(body, views.AlsoRightLabel) {
		t.Fatalf("wrong grade = %d\n%s", graded.Code, body)
	}
	attemptID := attemptValue(t, body)
	tap := formPost(t, h, learner, "/_hx/also-right", url.Values{
		"attempt": {itoa(attemptID)}, "run": {itoa(runID)}, "_csrf": {learner.csrf},
	}, func(r *http.Request) { r.Header.Set("HX-Request", "true") })
	taken := tap.Body.String()
	if tap.Code != http.StatusOK || !strings.Contains(taken, views.AlsoRightTaken) || strings.Contains(taken, views.AlsoRightLabel) {
		t.Fatalf("tap = %d\n%s", tap.Code, taken)
	}
	if correct := attemptCorrect(t, store, attemptID); correct != 0 {
		t.Fatalf("attempt correct = %d", correct)
	}
	rows := voidRows(t, store, attemptID)
	if len(rows) != 1 || rows[0].reason != db.VoidReasonAlsoRight || rows[0].actor != db.LearnerScope(learner.user.ID) {
		t.Fatalf("void rows = %+v", rows)
	}

	rightLearner := addLearner(t, store, "b@example.com", "sub-b", now)
	if _, err := store.ExecContext(ctx, `
		INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score)
		VALUES (?, ?, 0, ?, ?, 1), (?, ?, 0, ?, ?, 1)`,
		rightLearner.user.ID, earlierID, when, when,
		rightLearner.user.ID, clozeID, when, when); err != nil {
		t.Fatal(err)
	}
	rightPage := getLearner(t, h, rightLearner, "/lesson/"+itoa(recallID)+"/0")
	rightRun := mustRunID(t, rightPage)
	right := postLesson(t, h, rightLearner.cookie, "/lesson/"+itoa(recallID)+"/0", itoa(rightRun), itoa(ids["ד"]), true, rightLearner.csrf)
	if right.Code != http.StatusOK || !strings.Contains(right.Body.String(), "feedback--right") || strings.Contains(right.Body.String(), views.AlsoRightLabel) {
		t.Fatalf("right grade = %d\n%s", right.Code, right.Body.String())
	}
}

func TestReportVoidsThatLearnersWrongsInOneBatch(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	a := addLearner(t, store, "a@example.com", "sub-a", now)
	b := addLearner(t, store, "b@example.com", "sub-b", now)
	for _, l := range []learnerFix{a, b} {
		finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	}
	reported := shellUnit(t, store, "Reported unit", 1, "shared", 1)
	other := shellUnit(t, store, "Other unit", 2, "shared", 1)
	lex := addLexeme(t, store, "כֶּלֶב", "dog", "kelev", "m", "")
	rec := evidence.New(store, fsrs.New(store, fixedClock(now)), fixedClock(now))
	write := func(user, unit int64, correct bool) int64 {
		t.Helper()
		if _, err := rec.Attempt(context.Background(), evidence.Attempt{
			UserID: user, LexemeID: lex, Skill: fsrs.SkillRead, Activity: "recall",
			Source: evidence.SourceLesson, UnitID: unit, Prompt: "p", Response: "r",
			Correct: correct, SkipSchedule: true,
		}); err != nil {
			t.Fatal(err)
		}
		return latestAttemptID(t, store)
	}
	wrong1 := write(a.user.ID, reported, false)
	wrong2 := write(a.user.ID, reported, false)
	right := write(a.user.ID, reported, true)
	otherWrong := write(a.user.ID, other, false)
	theirs := write(b.user.ID, reported, false)

	path := "/lessons/" + itoa(reported) + "/report"
	recd := formPost(t, h, a, path, url.Values{"reason": {"wrong_hebrew"}, "note": {"line"}, "_csrf": {a.csrf}}, nil)
	if recd.Code != http.StatusSeeOther || recd.Header().Get("Location") != "/?reported=1" {
		t.Fatalf("report = %d %q", recd.Code, recd.Header().Get("Location"))
	}
	today := getLearner(t, h, a, "/?reported=1")
	if strings.Contains(today, "Reported unit") || strings.Contains(today, "/lesson/"+itoa(reported)+"/0") {
		t.Fatal("the reported unit is still on Today")
	}
	lessons := getLearner(t, h, a, "/lessons")
	if !strings.Contains(lessons, "Other unit") || strings.Contains(lessons, "Reported unit") {
		t.Fatal("the other unit was hidden too")
	}
	rows := allVoidRows(t, store)
	if len(rows) != 2 {
		t.Fatalf("void rows = %+v, want the two wrong answers", rows)
	}
	batch := rows[0].batch
	for _, row := range rows {
		if row.batch != batch || row.reason != db.VoidReasonReport || row.actor != db.LearnerScope(a.user.ID) {
			t.Fatalf("void row %+v", row)
		}
		if row.attempt != wrong1 && row.attempt != wrong2 {
			t.Fatalf("voided attempt %d", row.attempt)
		}
	}
	for _, id := range []int64{right, otherWrong, theirs} {
		if n := len(voidRows(t, store, id)); n != 0 {
			t.Fatalf("attempt %d has %d voids", id, n)
		}
	}
	if n := attemptCorrect(t, store, wrong1); n != 0 {
		t.Fatalf("voided attempt correct = %d", n)
	}
}

func TestAlsoRightIsNotAnMCPTool(t *testing.T) {
	nameRe := regexp.MustCompile(`Name:\s+"([^"]+)"`)
	err := filepath.WalkDir(filepath.Join("..", "mcp"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(b)
		if strings.Contains(text, "VoidAnswers") || strings.Contains(text, "VoidUnitWrong") {
			t.Errorf("%s calls the void engine", path)
		}
		for _, m := range nameRe.FindAllStringSubmatch(text, -1) {
			if strings.Contains(strings.ToLower(m[1]), "void") {
				t.Errorf("mcp tool %q", m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func cardScheduleEqual(a, b db.Card) bool {
	return a.State == b.State &&
		a.Stability == b.Stability &&
		a.Difficulty == b.Difficulty &&
		a.Reps == b.Reps &&
		a.Lapses == b.Lapses &&
		a.Due.Equal(b.Due) &&
		a.LastReview.Equal(b.LastReview)
}

func formatTestCard(c db.Card) string {
	return c.State + " due=" + c.Due.UTC().Format(time.RFC3339Nano) + " reps=" + itoa(int64(c.Reps)) + " lapses=" + itoa(int64(c.Lapses))
}

func attemptIDs(t *testing.T, store *db.Store, userID int64) []int64 {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := conn.Query(`SELECT id FROM attempts WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}

func latestAttemptID(t *testing.T, store *db.Store) int64 {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := conn.QueryRow(`SELECT id FROM attempts ORDER BY id DESC LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func attemptCount(t *testing.T, store *db.Store) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM attempts`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func attemptCorrect(t *testing.T, store *db.Store, id int64) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(`SELECT correct FROM attempts WHERE id = ?`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func voidCount(t *testing.T, store *db.Store) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM answer_voids`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

type voidRow struct {
	attempt int64
	reason  string
	actor   string
	batch   string
}

func voidRows(t *testing.T, store *db.Store, attemptID int64) []voidRow {
	t.Helper()
	return queryVoids(t, store, `SELECT attempt_id, reason, actor, batch_id FROM answer_voids WHERE attempt_id = ? ORDER BY id`, attemptID)
}

func allVoidRows(t *testing.T, store *db.Store) []voidRow {
	t.Helper()
	return queryVoids(t, store, `SELECT attempt_id, reason, actor, batch_id FROM answer_voids ORDER BY id`)
}

func queryVoids(t *testing.T, store *db.Store, q string, args ...any) []voidRow {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := conn.Query(q, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []voidRow
	for rows.Next() {
		var row voidRow
		if err := rows.Scan(&row.attempt, &row.reason, &row.actor, &row.batch); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func attemptValue(t *testing.T, body string) int64 {
	t.Helper()
	re := regexp.MustCompile(`name="attempt"[^>]*value="(\d+)"|value="(\d+)"[^>]*name="attempt"`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("attempt id missing:\n%s", body)
	}
	raw := m[1]
	if raw == "" {
		raw = m[2]
	}
	var id int64
	for _, c := range raw {
		id = id*10 + int64(c-'0')
	}
	return id
}
