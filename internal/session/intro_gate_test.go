package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// The intro gate used to hide a card for any role=new lesson_items row with
// no finished lesson_run. These tests pin the rule that replaced it: only a
// published unit in this learner's scope (shared, or learner:<id>) can gate.

func TestReviewQueueNotHiddenByDraftUnit(t *testing.T) {
	svc, store, clk, user, word := gateWord(t, "שָׁלוֹם", "hello", "shalom")
	shared := putPublished(t, store, oneWordUnit("Shared hello", 0, "שָׁלוֹם"), "shared")
	finishPublishedLesson(t, store, user, shared)
	draft := putDraft(t, store, oneWordUnit("Draft hello", 1, "שָׁלוֹם"), "shared")
	if n := roleNewCount(t, store, draft, word); n != 1 {
		t.Fatalf("draft role=new rows = %d, want 1", n)
	}
	if status := unitStatus(t, store, draft); status != "draft" {
		t.Fatalf("draft status = %s", status)
	}
	assertWordServed(t, svc, store, user, word, clk.Now())
}

func TestReviewQueueNotHiddenByOtherLearnersUnit(t *testing.T) {
	svc, store, clk, user, word := gateWord(t, "בֹּקֶר", "morning", "boker")
	other, err := store.InsertUser(context.Background(), "other@example.com", "sub-other", "learner")
	if err != nil {
		t.Fatal(err)
	}
	shared := putPublished(t, store, oneWordUnit("Shared morning", 0, "בֹּקֶר"), "shared")
	finishPublishedLesson(t, store, user, shared)
	theirs := putPublished(t, store, oneWordUnit("Learner 2 morning", 1, "בֹּקֶר"), db.LearnerScope(other))
	if n := roleNewCount(t, store, theirs, word); n != 1 {
		t.Fatalf("other learner role=new rows = %d, want 1", n)
	}
	if got := unitScope(t, store, theirs); got != db.LearnerScope(other) || unitStatus(t, store, theirs) != "published" {
		t.Fatalf("other unit scope %s status %s", got, unitStatus(t, store, theirs))
	}
	assertWordServed(t, svc, store, user, word, clk.Now())
	due, err := store.ListDueCards(context.Background(), other, clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Fatalf("learner 2 has no card, due = %d", len(due))
	}
}

func TestReviewQueueStillGatedByOwnUnfinishedLesson(t *testing.T) {
	svc, store, clk, user, word := gateWord(t, "עֶרֶב", "evening", "erev")
	shared := putPublished(t, store, oneWordUnit("Shared evening", 0, "עֶרֶב"), "shared")
	finishPublishedLesson(t, store, user, shared)
	// The shared lesson is finished, so its intro no longer gates. The
	// learner's own published lesson still introduces the word and is not
	// finished, so the card stays out.
	own := putPublished(t, store, oneWordUnit("Own evening", 1, "עֶרֶב"), db.LearnerScope(user))
	if n := roleNewCount(t, store, own, word); n != 1 || unitStatus(t, store, own) != "published" {
		t.Fatalf("own unit status %s role=new %d", unitStatus(t, store, own), n)
	}
	due, err := store.ListDueCards(context.Background(), user, clk.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Fatalf("due = %d, want the unfinished lesson to gate the card", len(due))
	}
	for _, ref := range drain(t, mustBuild(t, svc, user)) {
		if ref.LexemeID == word {
			t.Fatalf("Review served gated card %+v", ref)
		}
	}
}

// TestLearner1OneCardQueueReproduces is the production shape: learner 1 had
// 17 cards on 9 lexemes and Review showed only גָּר. A draft reintroduces
// every other word as new, which the old gate treated as an unfinished intro.
func TestLearner1OneCardQueueReproduces(t *testing.T) {
	const (
		ani    = "אֲנִי"
		gar    = "גָּר"
		po     = "פֹּה"
		ata    = "אַתָּה"
		eifo   = "אֵיפֹה"
		oved   = "עוֹבֵד"
		lomed  = "לוֹמֵד"
		ivrit  = "עִבְרִית"
		shalom = "שָׁלוֹם"
	)
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	user := mustUser(t, store)
	if user != 1 {
		t.Fatalf("learner id = %d, want 1", user)
	}
	now := clk.Now()
	words := []struct{ text, gloss, translit string }{
		{ani, "I", "ani"},
		{gar, "lives", "gar"},
		{po, "here", "po"},
		{ata, "you", "ata"},
		{eifo, "where", "eifo"},
		{oved, "works", "oved"},
		{lomed, "studies", "lomed"},
		{ivrit, "Hebrew", "ivrit"},
		{shalom, "hello", "shalom"},
	}
	ids := map[string]int64{}
	for _, w := range words {
		ids[w.text] = mustUpsert(t, store, w.text, w.gloss, w.translit)
	}

	sharedWords := []string{ani, gar, po, ata, eifo, oved, lomed, ivrit}
	shared := putPublished(t, store, wordsUnit("Hi, I live here", 0, sharedWords), "shared")
	finishPublishedLesson(t, store, user, shared)
	// Finishing enqueues read and recall. Prod had only the recall card for גָּר.
	if n, err := store.ExecContext(ctx, `DELETE FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = 'read'`, user, ids[gar]); err != nil {
		t.Fatal(err)
	} else if rows, err := n.RowsAffected(); err != nil || rows != 1 {
		t.Fatalf("delete גָּר read = %d, %v", rows, err)
	}

	own := putPublished(t, store, wordsUnit("Learner hello", 1, []string{shalom}), db.LearnerScope(user))
	finishPublishedLesson(t, store, user, own)

	// 8 shared words × read/recall, minus the deleted גָּר read, plus שָׁלוֹם read/recall.
	if got := cardCount(t, store, user); got != 17 {
		t.Fatalf("cards = %d, want 17", got)
	}
	if got := lexemeCount(t, store, user); got != 9 {
		t.Fatalf("lexemes = %d, want 9", got)
	}

	reviewAhead := []struct {
		text  string
		skill string
		days  int
	}{
		{ani, fsrs.SkillRead, 1},
		{ani, fsrs.SkillRecall, 2},
		{po, fsrs.SkillRead, 3},
		{po, fsrs.SkillRecall, 4},
		{shalom, fsrs.SkillRead, 5},
		{shalom, fsrs.SkillRecall, 5},
	}
	for _, row := range reviewAhead {
		shapeCard(t, store, user, ids[row.text], row.skill, fsrs.StateReview, now.Add(time.Duration(row.days)*24*time.Hour), int(fsrs.Good), now.Add(-48*time.Hour))
	}
	shapeCard(t, store, user, ids[gar], fsrs.SkillRecall, fsrs.StateLearning, now, int(fsrs.Hard), now.Add(-time.Hour))
	if err := store.InsertConfusionMiss(ctx, user, fsrs.MissOther, gar+"/"+po, ids[gar], now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}

	// The draft lists every word except גָּר. deriveLessonItems marks those
	// role=new. The old gate hid them because this lesson has no finished run.
	gated := append([]string{}, sharedWords...)
	gated = append(gated, shalom)
	gated = without(gated, gar)
	draft := putDraft(t, store, wordsUnit("Unpublished redo", 2, gated), "shared")
	if unitStatus(t, store, draft) != "draft" {
		t.Fatalf("gating unit status = %s", unitStatus(t, store, draft))
	}
	if n := roleNewCount(t, store, draft, ids[gar]); n != 0 {
		t.Fatalf("draft introduces גָּר (%d rows); the one visible card would be hidden too", n)
	}
	for _, text := range gated {
		if n := roleNewCount(t, store, draft, ids[text]); n != 1 {
			t.Fatalf("draft role=new for %s = %d", text, n)
		}
	}

	due, err := store.ListDueCards(ctx, user, now)
	if err != nil {
		t.Fatal(err)
	}
	study, err := store.ListStudyCards(ctx, user, now, now.Add(-ConfusionWindow))
	if err != nil {
		t.Fatal(err)
	}
	// Today counts ListDueCards. Review is built from ListStudyCards with the
	// same predicate, so the due rows Review can serve are Today's count.
	dueIDs := map[int64]bool{}
	for _, card := range due {
		dueIDs[card.ID] = true
	}
	reviewDue := 0
	for _, card := range study {
		if card.Due.After(now) {
			continue
		}
		reviewDue++
		if !dueIDs[card.ID] {
			t.Fatalf("Review sees due card %d that Today does not", card.ID)
		}
		delete(dueIDs, card.ID)
	}
	if len(due) != reviewDue || len(dueIDs) != 0 {
		t.Fatalf("Today due %d, Review due %d, Today-only %d", len(due), reviewDue, len(dueIDs))
	}

	got := drain(t, mustBuild(t, svc, user))
	if len(due) != 11 {
		t.Fatalf("due = %d, session = %d, want 10 new cards plus גָּר", len(due), len(got))
	}
	lexemes := map[int64]int{}
	newCards := 0
	for _, ref := range got {
		if ref.CardID <= 0 {
			continue
		}
		lexemes[ref.LexemeID]++
		if ref.New {
			newCards++
		}
	}
	if len(lexemes) < 2 || newCards == 0 {
		t.Fatalf("session lexemes = %d, new cards = %d, queue = %d; want more than גָּר, including new cards", len(lexemes), newCards, len(got))
	}
	if lexemes[ids[gar]] == 0 {
		t.Fatal("גָּר recall left the queue")
	}
	newLexemes := 0
	for _, text := range []string{ata, eifo, oved, lomed, ivrit} {
		if lexemes[ids[text]] > 0 {
			newLexemes++
		}
	}
	if newLexemes == 0 {
		t.Fatal("none of the five new words were served")
	}
}

func gateWord(t *testing.T, text, gloss, translit string) (*Service, *db.Store, *fakeClock, int64, int64) {
	t.Helper()
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	return svc, store, clk, user, mustUpsert(t, store, text, gloss, translit)
}

func mustUpsert(t *testing.T, store *db.Store, text, gloss, translit string) int64 {
	t.Helper()
	tr, pos := translit, "noun"
	saved, _, err := store.UpsertLexemes(context.Background(), []lexicon.Input{{
		TextNFC: text, Gloss: gloss, Translit: &tr, POS: &pos,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 {
		t.Fatalf("upsert %s = %d rows", text, len(saved))
	}
	return saved[0].ID
}

func oneWordUnit(title string, position int, word string) units.Unit {
	return wordsUnit(title, position, []string{word})
}

func wordsUnit(title string, position int, words []string) units.Unit {
	return units.Unit{
		Title:     title,
		Situation: "A scene.",
		Concept:   "One idea.",
		Position:  position,
		NewWords:  append([]string{}, words...),
		Lessons: []units.Lesson{{
			Kind: units.KindLesson,
			Activities: []units.Activity{{
				Type: units.ActivityStory,
				Text: strings.Join(words, " "),
			}},
		}},
	}
}

func putPublished(t *testing.T, store *db.Store, doc units.Unit, scope string) int64 {
	t.Helper()
	id := putDraft(t, store, doc, scope)
	if _, err := store.PublishUnit(context.Background(), id, nil); err != nil {
		t.Fatal(err)
	}
	return id
}

func putDraft(t *testing.T, store *db.Store, doc units.Unit, scope string) int64 {
	t.Helper()
	res, err := store.PutUnit(context.Background(), nil, doc, scope, "test")
	if err != nil {
		t.Fatal(err)
	}
	return res.UnitID
}

func finishPublishedLesson(t *testing.T, store *db.Store, userID, unitID int64) {
	t.Helper()
	ctx := context.Background()
	run, err := store.StartPlayerRun(ctx, userID, unitID, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, applied, err := store.ContinuePlayerRun(ctx, userID, run.ID, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !applied || !got.Finished {
		t.Fatalf("finish applied=%v finished=%v", applied, got.Finished)
	}
}

func shapeCard(t *testing.T, store *db.Store, userID, lexemeID int64, skill, state string, due time.Time, rating int, reviewed time.Time) {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := conn.QueryRow(
		`SELECT id FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = ?`,
		userID, lexemeID, skill,
	).Scan(&id); err != nil {
		t.Fatal(err)
	}
	card, err := store.GetCard(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	card.State = state
	card.Due = due
	card.Stability = 4
	card.Difficulty = 5
	card.Reps = 1
	card.LastReview = reviewed
	if err := store.ApplyReview(context.Background(), card, db.ReviewLog{
		Rating: rating, ReviewedAt: reviewed, ElapsedDays: 0, ScheduledDays: 1,
	}); err != nil {
		t.Fatal(err)
	}
}

func assertWordServed(t *testing.T, svc *Service, store *db.Store, userID, lexemeID int64, now time.Time) {
	t.Helper()
	due, err := store.ListDueCards(context.Background(), userID, now)
	if err != nil {
		t.Fatal(err)
	}
	study, err := store.ListStudyCards(context.Background(), userID, now, now.Add(-ConfusionWindow))
	if err != nil {
		t.Fatal(err)
	}
	dueN, studyN := 0, 0
	for _, card := range due {
		if card.LexemeID == lexemeID {
			dueN++
		}
	}
	for _, card := range study {
		if card.LexemeID == lexemeID && !card.Due.After(now) {
			studyN++
		}
	}
	if dueN == 0 || dueN != studyN {
		t.Fatalf("Today due %d, Review due %d for lexeme %d", dueN, studyN, lexemeID)
	}
	served := 0
	for _, ref := range drain(t, mustBuild(t, svc, userID)) {
		if ref.LexemeID == lexemeID {
			served++
		}
	}
	if served == 0 {
		t.Fatal("Review did not serve the card")
	}
}

func roleNewCount(t *testing.T, store *db.Store, unitID, lexemeID int64) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(
		`SELECT COUNT(*) FROM lesson_items WHERE unit_id = ? AND lexeme_id = ? AND role = 'new'`,
		unitID, lexemeID,
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func unitStatus(t *testing.T, store *db.Store, unitID int64) string {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var status string
	if err := conn.QueryRow(`SELECT status FROM units WHERE id = ?`, unitID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func unitScope(t *testing.T, store *db.Store, unitID int64) string {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var scope string
	if err := conn.QueryRow(`SELECT scope FROM units WHERE id = ?`, unitID).Scan(&scope); err != nil {
		t.Fatal(err)
	}
	return scope
}

func cardCount(t *testing.T, store *db.Store, userID int64) int {
	t.Helper()
	return scalarCount(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, userID)
}

func lexemeCount(t *testing.T, store *db.Store, userID int64) int {
	t.Helper()
	return scalarCount(t, store, `SELECT COUNT(DISTINCT lexeme_id) FROM cards WHERE user_id = ?`, userID)
}

func scalarCount(t *testing.T, store *db.Store, query string, arg any) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(query, arg).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func without(words []string, drop string) []string {
	var out []string
	for _, w := range words {
		if w != drop {
			out = append(out, w)
		}
	}
	return out
}
