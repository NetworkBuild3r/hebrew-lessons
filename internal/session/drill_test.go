package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func seedLetterMisses(t *testing.T, store *db.Store, clk *fakeClock, userID int64, key string, n int) {
	t.Helper()
	rev := fsrs.New(store, clk)
	for i := 0; i < n; i++ {
		if err := rev.RecordMiss(context.Background(), userID, fsrs.MissLetterPair, key, 0); err != nil {
			t.Fatal(err)
		}
		clk.Advance(time.Second)
	}
}

func seedAgreementMisses(t *testing.T, store *db.Store, clk *fakeClock, userID int64, key string, n int) {
	t.Helper()
	rev := fsrs.New(store, clk)
	for i := 0; i < n; i++ {
		if err := rev.RecordMiss(context.Background(), userID, fsrs.MissAgreement, key, 0); err != nil {
			t.Fatal(err)
		}
		clk.Advance(time.Second)
	}
}

func nextDrill(t *testing.T, sess *Session, kind string) CardRef {
	t.Helper()
	for i := 0; i < SessionCap+DrillCap+5; i++ {
		card, ok := sess.Next()
		if !ok {
			t.Fatalf("no %s drill left", kind)
		}
		if card.Kind == kind {
			return card
		}
	}
	t.Fatalf("no %s drill found", kind)
	return CardRef{}
}

func countDrillAnswers(t *testing.T, store *db.Store, userID int64) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM drill_answers WHERE learner_id = ?`, userID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func countClears(t *testing.T, store *db.Store, userID int64, kind, key string) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(
		`SELECT COUNT(*) FROM confusion_clears WHERE learner_id = ? AND kind = ? AND key = ?`,
		userID, kind, key,
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func countKindInQueue(sess *Session, kind, key string) int {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	n := 0
	for _, c := range sess.cards[sess.i:] {
		if c.Kind == kind && c.Key == key {
			n++
		}
	}
	return n
}

// ac-1: at most DrillSteps per pair and DrillCap per session.
func TestDrillStepsAndCap(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	pairs := []string{"ב/כ", "ג/ק", "ד/ת"}
	for _, key := range pairs {
		seedLetterMisses(t, store, clk, userID, key, MinMissesForDrill)
	}
	got := drainAll(t, mustBuild(t, svc, userID))
	byKey := map[string]int{}
	letters := 0
	for _, card := range got {
		if card.Kind != KindLetter {
			continue
		}
		letters++
		byKey[card.Key]++
	}
	if letters > DrillCap {
		t.Fatalf("letter drills = %d, cap %d", letters, DrillCap)
	}
	for key, n := range byKey {
		if n > DrillSteps {
			t.Fatalf("key %s has %d steps, want at most %d", key, n, DrillSteps)
		}
	}
	if letters != DrillCap {
		t.Fatalf("letters = %d, want DrillCap %d for three pairs", letters, DrillCap)
	}
}

func TestDrillSinceSkipsTheClearingAnswer(t *testing.T) {
	clear := time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)
	miss := clear.Add(-time.Hour)
	since := drillSince(miss, true, clear, true)
	if !since.After(clear) {
		t.Fatalf("since = %s, want after the clear", since)
	}
	if got := drillSince(clear.Add(time.Minute), true, clear, true); !got.Equal(clear.Add(time.Minute)) {
		t.Fatalf("a later miss bound = %s", got)
	}
}

// ac-2: three right answers clear and drop later steps; next build adds none.
func TestDrillClearDropsRemainingAndNextBuild(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	seedLetterMisses(t, store, clk, userID, "ב/כ", MinMissesForDrill)

	// Two prior rights so the first served step clears.
	for i := 0; i < DrillClearStreak-1; i++ {
		if err := store.InsertDrillAnswer(ctx, db.DrillAnswer{
			LearnerID: userID, Kind: fsrs.MissLetterPair, Key: "ב/כ",
			Correct: true, LatencyMS: 500, At: clk.Now(),
		}); err != nil {
			t.Fatal(err)
		}
		clk.Advance(time.Second)
	}

	sess := mustBuild(t, svc, userID)
	drill := nextDrill(t, sess, KindLetter)
	if remaining := countKindInQueue(sess, KindLetter, "ב/כ"); remaining < 1 {
		t.Fatalf("want later ב/כ steps in queue, got %d", remaining)
	}
	out, err := sess.DrillAnswered(ctx, drill.CardID, true, 400*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Cleared || out.Streak < DrillClearStreak {
		t.Fatalf("outcome = %+v, want cleared", out)
	}
	if countClears(t, store, userID, fsrs.MissLetterPair, "ב/כ") != 1 {
		t.Fatal("want one confusion_clears row")
	}
	if remaining := countKindInQueue(sess, KindLetter, "ב/כ"); remaining != 0 {
		t.Fatalf("remaining ב/כ steps = %d, want 0", remaining)
	}

	rebuild := drainAll(t, mustBuild(t, svc, userID))
	for _, card := range rebuild {
		if card.Kind == KindLetter && card.Key == "ב/כ" {
			t.Fatalf("rebuild still has ב/כ drill: %+v", card)
		}
	}
}

// ac-3: right, wrong, right, right does not clear; a further right does.
func TestDrillWrongResetsStreak(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	seedLetterMisses(t, store, clk, userID, "ב/כ", MinMissesForDrill)

	insert := func(correct bool) {
		t.Helper()
		if err := store.InsertDrillAnswer(ctx, db.DrillAnswer{
			LearnerID: userID, Kind: fsrs.MissLetterPair, Key: "ב/כ",
			Correct: correct, LatencyMS: 400, At: clk.Now(),
		}); err != nil {
			t.Fatal(err)
		}
		clk.Advance(time.Second)
	}
	// right, wrong, right, right — trailing streak is 2, so no clear yet.
	insert(true)
	insert(false)
	insert(true)
	insert(true)
	if countClears(t, store, userID, fsrs.MissLetterPair, "ב/כ") != 0 {
		t.Fatal("R,W,R,R must not clear")
	}

	sess := mustBuild(t, svc, userID)
	drill := nextDrill(t, sess, KindLetter)
	out, err := sess.DrillAnswered(ctx, drill.CardID, true, 300*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Cleared || out.Streak < DrillClearStreak {
		t.Fatalf("further right = %+v, want clear", out)
	}
	if countClears(t, store, userID, fsrs.MissLetterPair, "ב/כ") != 1 {
		t.Fatal("want one clear after the further right")
	}
}

// ac-4: slow right breaks the run once 10 timed latencies exist, not before.
func TestDrillSlowBreaksStreak(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	seedLetterMisses(t, store, clk, userID, "ב/כ", MinMissesForDrill)

	seedTimedRights := func(n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			if err := store.InsertDrillAnswer(ctx, db.DrillAnswer{
				LearnerID: userID, Kind: fsrs.MissLetterPair, Key: "ב/כ",
				Correct: true, LatencyMS: 1000, At: clk.Now(),
			}); err != nil {
				t.Fatal(err)
			}
			clk.Advance(time.Second)
		}
	}

	t.Run("with 10 timed rights a slow answer does not count", func(t *testing.T) {
		seedTimedRights(DrillMedianMin)
		// Untimed rights build the streak without changing the median window.
		for i := 0; i < DrillClearStreak-1; i++ {
			if err := store.InsertDrillAnswer(ctx, db.DrillAnswer{
				LearnerID: userID, Kind: fsrs.MissLetterPair, Key: "ב/כ",
				Correct: true, LatencyMS: 0, At: clk.Now(),
			}); err != nil {
				t.Fatal(err)
			}
			clk.Advance(time.Second)
		}
		sess := mustBuild(t, svc, userID)
		drill := nextDrill(t, sess, KindLetter)
		out, err := sess.DrillAnswered(ctx, drill.CardID, true, 2500*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		if out.Cleared || out.Streak != 0 {
			t.Fatalf("slow right = %+v, want streak 0", out)
		}
	})

	t.Run("with 9 timed rights a slow answer still counts", func(t *testing.T) {
		svc, store, clk := openSvc(t, 1)
		userID := mustUser(t, store)
		seedLetterMisses(t, store, clk, userID, "ב/כ", MinMissesForDrill)
		for i := 0; i < DrillMedianMin-1; i++ {
			if err := store.InsertDrillAnswer(ctx, db.DrillAnswer{
				LearnerID: userID, Kind: fsrs.MissLetterPair, Key: "ב/כ",
				Correct: true, LatencyMS: 1000, At: clk.Now(),
			}); err != nil {
				t.Fatal(err)
			}
			clk.Advance(time.Second)
		}
		for i := 0; i < DrillClearStreak-1; i++ {
			if err := store.InsertDrillAnswer(ctx, db.DrillAnswer{
				LearnerID: userID, Kind: fsrs.MissLetterPair, Key: "ב/כ",
				Correct: true, LatencyMS: 0, At: clk.Now(),
			}); err != nil {
				t.Fatal(err)
			}
			clk.Advance(time.Second)
		}
		sess := mustBuild(t, svc, userID)
		drill := nextDrill(t, sess, KindLetter)
		out, err := sess.DrillAnswered(ctx, drill.CardID, true, 2500*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		if !out.Cleared {
			t.Fatalf("with 9 timed rights slow still counts: %+v", out)
		}
	})
}

// After a clear, one new miss adds no drill; two new misses add one.
func TestDrillReturnsOnlyAfterTwoNewMisses(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	seedLetterMisses(t, store, clk, userID, "ב/כ", MinMissesForDrill)
	sess := mustBuild(t, svc, userID)
	for i := 0; i < DrillClearStreak; i++ {
		drill := nextDrill(t, sess, KindLetter)
		if _, err := sess.DrillAnswered(ctx, drill.CardID, true, 200*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		clk.Advance(time.Second)
	}
	if countClears(t, store, userID, fsrs.MissLetterPair, "ב/כ") != 1 {
		t.Fatal("want a clear before the rebuild checks")
	}

	seedLetterMisses(t, store, clk, userID, "ב/כ", 1)
	for _, card := range drainAll(t, mustBuild(t, svc, userID)) {
		if card.Kind == KindLetter && card.Key == "ב/כ" {
			t.Fatalf("one new miss still queued drill: %+v", card)
		}
	}

	seedLetterMisses(t, store, clk, userID, "ב/כ", 1)
	found := false
	for _, card := range drainAll(t, mustBuild(t, svc, userID)) {
		if card.Kind == KindLetter && card.Key == "ב/כ" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("two new misses should add a ב/כ drill")
	}
}

// ac-7: pattern DrillAnswered writes a right or wrong drill answer; a wrong
// answer is not asked again in that sitting. INIT-029/SPEC-005.
func TestPatternRecordGradesDrill(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	seedAgreementMisses(t, store, clk, userID, "הוא/היא", MinMissesForDrill)
	sess := mustBuild(t, svc, userID)
	drill := nextDrill(t, sess, KindPattern)
	if _, err := sess.DrillAnswered(ctx, drill.CardID, true, 200*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	answers, err := store.DrillAnswersSince(ctx, userID, fsrs.MissAgreement, "הוא/היא", time.Time{})
	if err != nil || len(answers) != 1 || !answers[0].Correct || answers[0].LatencyMS != 200 {
		t.Fatalf("good answers = %+v, %v", answers, err)
	}

	sess = mustBuild(t, svc, userID)
	drill = nextDrill(t, sess, KindPattern)
	if _, err := sess.DrillAnswered(ctx, drill.CardID, false, 400*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	answers, err = store.DrillAnswersSince(ctx, userID, fsrs.MissAgreement, "הוא/היא", time.Time{})
	if err != nil || len(answers) < 2 || answers[len(answers)-1].Correct {
		t.Fatalf("wrong answers = %+v, %v", answers, err)
	}
	found := false
	for i := 0; i < DrillSteps+4; i++ {
		next, ok := sess.Next()
		if !ok {
			break
		}
		if next.CardID == drill.CardID {
			found = true
			break
		}
	}
	if found {
		t.Fatal("a wrong pattern pick was asked again in the same sitting")
	}
}

// ac-8: DrillAnswered refuses study cards, unserved cards, and a second call.
func TestDrillAnsweredRefusals(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	fillPool(t, store, 30)
	lex := addVoicedLexeme(t, store, "ספר-drill", "book-drill", false)
	cardID := addCardFor(t, store, userID, lex, fsrs.SkillRead, clk.Now().Add(-time.Hour), int(fsrs.Good), clk.Now().Add(-48*time.Hour))
	seedLetterMisses(t, store, clk, userID, "ב/כ", MinMissesForDrill)
	sess := mustBuild(t, svc, userID)

	study, ok := sess.Next()
	if !ok || study.CardID != cardID {
		t.Fatalf("first = %+v ok=%v, want study card %d", study, ok, cardID)
	}
	if _, err := sess.DrillAnswered(ctx, study.CardID, true, time.Second); !errors.Is(err, ErrNotDrill) {
		t.Fatalf("study DrillAnswered err = %v, want ErrNotDrill", err)
	}
	if countDrillAnswers(t, store, userID) != 0 {
		t.Fatal("study refusal wrote a drill answer")
	}

	drill := nextDrill(t, sess, KindLetter)
	if _, err := sess.DrillAnswered(ctx, drill.CardID+1, true, time.Second); !errors.Is(err, ErrNotServed) {
		t.Fatalf("unserved err = %v, want ErrNotServed", err)
	}
	if _, err := sess.DrillAnswered(ctx, drill.CardID, true, 100*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if _, err := sess.DrillAnswered(ctx, drill.CardID, true, 100*time.Millisecond); !errors.Is(err, ErrNotServed) {
		t.Fatalf("second call err = %v, want ErrNotServed", err)
	}
	if countDrillAnswers(t, store, userID) != 1 {
		t.Fatalf("answers = %d, want 1", countDrillAnswers(t, store, userID))
	}
}

func TestDrillAnsweredLearnerIsolation(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	a := mustUser(t, store)
	b, err := store.InsertUser(ctx, "other@example.com", "sub-other", "learner")
	if err != nil {
		t.Fatal(err)
	}
	seedLetterMisses(t, store, clk, a, "ב/כ", MinMissesForDrill)
	seedLetterMisses(t, store, clk, b, "ב/כ", MinMissesForDrill)

	sessA := mustBuild(t, svc, a)
	drill := nextDrill(t, sessA, KindLetter)
	if _, err := sessA.DrillAnswered(ctx, drill.CardID, true, 200*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if countDrillAnswers(t, store, a) != 1 || countDrillAnswers(t, store, b) != 0 {
		t.Fatalf("a=%d b=%d", countDrillAnswers(t, store, a), countDrillAnswers(t, store, b))
	}
}
