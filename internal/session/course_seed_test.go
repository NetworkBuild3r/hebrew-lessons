package session

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func addCourseLexeme(t *testing.T, store *db.Store, n, order, rank int) int64 {
	t.Helper()
	text := "קוּרְס" + strconv.Itoa(n)
	id := mustLexeme(t, store, text)
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO course_words (lexeme_id, course_order, list_rank, lemma, linked, added_at)
		VALUES (?, ?, ?, ?, 0, ?)`,
		id, order, rank, text, db.FormatTime(time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Fatal(err)
	}
	return id
}

func newCourseLexemes(t *testing.T, store *db.Store, user int64, ids []int64) map[int64]bool {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]bool{}
	for _, id := range ids {
		var n int
		if err := conn.QueryRow(`SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ? AND state = 'new'`, user, id).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			got[id] = true
		}
	}
	return got
}

func TestSeedTakesCourseWordsInOrder(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	now := clk.Now()
	var ids []int64
	for i := 1; i <= 10; i++ {
		ids = append(ids, addCourseLexeme(t, store, i, i, i))
	}
	mapped := mapDeckLexemes(t, store, user, 6, now)
	seedAccuracy(t, store, user, now, 48, 50)

	mustBuild(t, svc, user)
	got := newCourseLexemes(t, store, user, ids)
	for i := 0; i < 8; i++ {
		if !got[ids[i]] {
			t.Fatalf("course word %d was not introduced", i+1)
		}
	}
	for i := 8; i < 10; i++ {
		if got[ids[i]] {
			t.Fatalf("course word %d was introduced past the tier", i+1)
		}
	}
	for _, m := range mapped {
		if n := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ? AND state = 'new'`, user, m.LexemeID); n != 0 {
			t.Fatalf("deck word %s was seeded while course words remained", m.ItemID)
		}
	}
}

func TestSeedTierFollowsAccuracy(t *testing.T) {
	t.Run("mid", func(t *testing.T) {
		assertCourseSeed(t, 44, 50, 5)
	})
	t.Run("under target", func(t *testing.T) {
		assertCourseSeed(t, 40, 50, 0)
	})
	t.Run("no history", func(t *testing.T) {
		assertCourseSeed(t, 0, 0, 5)
	})
}

func assertCourseSeed(t *testing.T, good, total, want int) {
	t.Helper()
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	var ids []int64
	for i := 1; i <= 10; i++ {
		ids = append(ids, addCourseLexeme(t, store, i, i, i))
	}
	if total > 0 {
		seedAccuracy(t, store, user, clk.Now(), good, total)
	}
	mustBuild(t, svc, user)
	got := 0
	for _, id := range ids {
		if newCourseLexemes(t, store, user, []int64{id})[id] {
			got++
		}
	}
	if got != want {
		t.Fatalf("introduced %d, want %d", got, want)
	}
}

func TestSeedFallsBackToDeck(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	now := clk.Now()
	course := []int64{addCourseLexeme(t, store, 1, 1, 1), addCourseLexeme(t, store, 2, 2, 2)}
	mapped := mapDeckLexemes(t, store, user, 10, now)
	seedAccuracy(t, store, user, now, 48, 50)
	mustBuild(t, svc, user)
	for _, id := range course {
		if !newCourseLexemes(t, store, user, []int64{id})[id] {
			t.Fatal("course word was not introduced")
		}
	}
	for i := 0; i < 6; i++ {
		if n := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, mapped[i].LexemeID); n == 0 {
			t.Fatalf("deck word %d was not the fallback", i)
		}
	}
	if n := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, mapped[6].LexemeID); n != 0 {
		t.Fatal("seventh deck word was seeded")
	}
}

func TestSeedSkipsSuspendedCourseWord(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	now := clk.Now()
	first := addCourseLexeme(t, store, 1, 1, 1)
	second := addCourseLexeme(t, store, 2, 2, 2)
	if _, err := store.InsertCard(context.Background(), db.Card{
		UserID: user, LexemeID: first, Skill: fsrs.SkillRead, State: fsrs.StateReview,
		Due: now.Add(-time.Hour), Stability: 1, Difficulty: 5, Reps: 1, Suspended: true,
	}); err != nil {
		t.Fatal(err)
	}
	seedAccuracy(t, store, user, now, 48, 50)
	before := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, first)
	mustBuild(t, svc, user)
	after := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, first)
	if after != before {
		t.Fatalf("suspended word cards %d to %d", before, after)
	}
	if !newCourseLexemes(t, store, user, []int64{second})[second] {
		t.Fatal("the next course word was not introduced")
	}
}

func TestSeedSkipsWordBehindUnfinishedIntro(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	now := clk.Now()
	held := addCourseLexeme(t, store, 1, 1, 1)
	next := addCourseLexeme(t, store, 2, 2, 2)
	unit := insertUnitAt(t, store, 1)
	if err := testharness.InsertLessonItem(context.Background(), store, unit, 0, held, "new", "recall"); err != nil {
		t.Fatal(err)
	}
	seedAccuracy(t, store, user, now, 48, 50)
	mustBuild(t, svc, user)
	if n := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, held); n != 0 {
		t.Fatal("a word behind an unfinished lesson was introduced")
	}
	if !newCourseLexemes(t, store, user, []int64{next})[next] {
		t.Fatal("the next course word was not introduced")
	}
}

func TestNewCardsOrderByCourseRank(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	now := clk.Now()
	byRank := map[int]int64{}
	for _, rank := range []int{30, 5, 12} {
		id := addCourseLexeme(t, store, rank, rank, rank)
		byRank[rank] = id
		if _, err := store.InsertCard(context.Background(), db.Card{
			UserID: user, LexemeID: id, Skill: fsrs.SkillRead, State: "new", Due: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	var got []int
	for _, ref := range drain(t, mustBuild(t, svc, user)) {
		for rank, id := range byRank {
			if ref.LexemeID == id {
				got = append(got, rank)
			}
		}
	}
	want := []int{5, 12, 30}
	if len(got) < len(want) {
		t.Fatalf("served %v, want %v", got, want)
	}
	for i, rank := range want {
		if got[i] != rank {
			t.Fatalf("served %v, want %v", got, want)
		}
	}
}

func TestBuildLessonIntroducesItsWords(t *testing.T) {
	svc, store, _ := openSvc(t, 1)
	user := mustUser(t, store)
	var ids []int64
	for i := 1; i <= 3; i++ {
		ids = append(ids, addCourseLexeme(t, store, i, i, i))
	}
	before := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, user)
	sess, err := svc.BuildLesson(context.Background(), user, ids)
	if err != nil {
		t.Fatal(err)
	}
	after := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, user)
	if after <= before {
		t.Fatalf("cards before %d after %d, want the lesson's words", before, after)
	}
	for _, id := range ids {
		if !newCourseLexemes(t, store, user, []int64{id})[id] {
			t.Fatalf("word %d has no card", id)
		}
	}
	if _, total, _ := sess.Place(); total == 0 {
		t.Fatal("the lesson session is empty")
	}
}

func TestBuildLessonLeavesOtherWordsAlone(t *testing.T) {
	svc, store, _ := openSvc(t, 1)
	user := mustUser(t, store)
	inside := addCourseLexeme(t, store, 1, 1, 1)
	outside := addCourseLexeme(t, store, 2, 2, 2)
	if _, err := svc.BuildLesson(context.Background(), user, []int64{inside}); err != nil {
		t.Fatal(err)
	}
	if n := countTable(t, store, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, outside); n != 0 {
		t.Fatal("a word outside the lesson got a card")
	}
}

func TestCardsCarryTheirSource(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	now := clk.Now()
	course := addCourseLexeme(t, store, 1, 1, 1)
	tutorWord := mustLexeme(t, store, "כֶּלֶב")
	other := mustLexeme(t, store, "פִּלְפֵּל")
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO class_sheets (learner_id, sheet_ref, class_on, title, status, created_at)
		VALUES (?, 'week-1', '2026-11-10', 'Week 1', 'passed', ?)`,
		user, db.FormatTime(now)); err != nil {
		t.Fatal(err)
	}
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var sheetID int64
	if err := conn.QueryRow(`SELECT id FROM class_sheets WHERE learner_id = ?`, user).Scan(&sheetID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO class_lines (sheet_id, position, tutor_text, gloss_en, has_points, state)
		VALUES (?, 1, 'כֶּלֶב', 'dog', 1, 'ok')`, sheetID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{course, tutorWord, other} {
		if _, err := store.InsertCard(context.Background(), db.Card{
			UserID: user, LexemeID: id, Skill: fsrs.SkillRead, State: "new", Due: now,
		}); err != nil {
			t.Fatal(err)
		}
	}
	got := map[int64]Source{}
	for _, ref := range drain(t, mustBuild(t, svc, user)) {
		if ref.CardID > 0 {
			got[ref.LexemeID] = ref.Source
		}
	}
	if got[course] != SourceCourse {
		t.Fatalf("course word source = %q", got[course])
	}
	if got[tutorWord] != SourceTutor {
		t.Fatalf("tutor word source = %q", got[tutorWord])
	}
	if got[other] != SourceOther {
		t.Fatalf("other word source = %q", got[other])
	}
	_ = svc
}

func TestCourseWaitingMatchesSeed(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	user := mustUser(t, store)
	for i := 1; i <= 10; i++ {
		addCourseLexeme(t, store, i, i, i)
	}
	seedAccuracy(t, store, user, clk.Now(), 48, 50)
	n, _, err := svc.CourseWaiting(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	before := countTable(t, store, `SELECT COUNT(DISTINCT lexeme_id) FROM cards WHERE user_id = ? AND state = 'new'`, user)
	mustBuild(t, svc, user)
	after := countTable(t, store, `SELECT COUNT(DISTINCT lexeme_id) FROM cards WHERE user_id = ? AND state = 'new'`, user)
	if after-before != n {
		t.Fatalf("waiting %d, introduced %d", n, after-before)
	}
}
