package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

func (f *fakeClock) Advance(d time.Duration) { f.t = f.t.Add(d) }

func openSvc(t *testing.T, seed int64) (*Service, *db.Store, *fakeClock) {
	t.Helper()
	clk := &fakeClock{t: time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)}
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return New(store, clk, seed), store, clk
}

func mustUser(t *testing.T, store *db.Store) int64 {
	t.Helper()
	id, err := store.InsertUser(context.Background(), "learner@example.com", "sub-learner", "learner")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustLexeme(t *testing.T, store *db.Store, text string) int64 {
	t.Helper()
	id, err := testharness.InsertLexeme(context.Background(), store, text)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertReview(t *testing.T, store *db.Store, cardID int64, rating int, at time.Time) {
	t.Helper()
	_, err := store.ExecContext(context.Background(), `
		INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked)
		VALUES (?, ?, ?, 0, 0, 0)`,
		cardID, rating, db.FormatTime(at))
	if err != nil {
		t.Fatal(err)
	}
}

func addCard(t *testing.T, store *db.Store, userID int64, text, dir string, due time.Time, lapses, rating int, reviewed time.Time, suspended bool) int64 {
	t.Helper()
	id, err := store.InsertCard(context.Background(), db.Card{
		UserID:     userID,
		LexemeID:   mustLexeme(t, store, text),
		Dir:        dir,
		State:      fsrs.StateReview,
		Due:        due,
		Stability:  1,
		Difficulty: 5,
		Reps:       1,
		Lapses:     lapses,
		LastReview: reviewed,
		Suspended:  suspended,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rating != 0 {
		insertReview(t, store, id, rating, reviewed)
	}
	return id
}

func addAccuracy(t *testing.T, store *db.Store, userID int64, good, again int, now time.Time) {
	t.Helper()
	id, err := store.InsertCard(context.Background(), db.Card{
		UserID:     userID,
		LexemeID:   mustLexeme(t, store, "hist"),
		Dir:        fsrs.DirHeEn,
		State:      fsrs.StateReview,
		Due:        now.Add(365 * 24 * time.Hour),
		Stability:  1,
		Difficulty: 5,
		Reps:       good + again,
		LastReview: now,
		Suspended:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	total := good + again
	for i := 0; i < total; i++ {
		rating := int(fsrs.Good)
		if i >= good {
			rating = int(fsrs.Again)
		}
		insertReview(t, store, id, rating, now.Add(-time.Duration(i)*time.Minute))
	}
}

func mustBuild(t *testing.T, svc *Service, userID int64) *Session {
	t.Helper()
	sess, err := svc.BuildSession(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	if sess.UserID() != userID {
		t.Fatalf("UserID = %d, want %d", sess.UserID(), userID)
	}
	return sess
}

func drain(t *testing.T, sess *Session) []CardRef {
	t.Helper()
	var out []CardRef
	for {
		card, ok := sess.Next()
		if !ok {
			return out
		}
		out = append(out, card)
		if len(out) > SessionCap+DrillCap+DrillSteps {
			t.Fatalf("queue did not end (%s)", pattern(out))
		}
	}
}

func counts(cards []CardRef) (struggle, known int) {
	for _, card := range cards {
		if card.Struggling {
			struggle++
		} else {
			known++
		}
	}
	return struggle, known
}

func assertShape(t *testing.T, cards []CardRef) {
	t.Helper()
	if len(cards) > SessionCap {
		t.Fatalf("len = %d, cap %d (%s)", len(cards), SessionCap, pattern(cards))
	}
	if i := struggleRun(cards); i >= 0 {
		t.Fatalf("more than %d struggling cards in a row at %d: %s", MaxStruggleRun, i, pattern(cards))
	}
	s, k := counts(cards)
	if s >= 2 && k >= 2 && switches(cards) < 2 {
		t.Fatalf("queued in two blocks: %s", pattern(cards))
	}
}

func TestSessionCap(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-48 * time.Hour)
	for i := 0; i < 25; i++ {
		due := clk.Now().Add(time.Duration(i)*time.Minute - 2*time.Hour)
		rating := int(fsrs.Good)
		if i >= 12 {
			rating = int(fsrs.Hard)
		}
		addCard(t, store, userID, fmt.Sprintf("cap-%02d", i), fsrs.DirHeEn, due, 0, rating, reviewed, false)
	}
	got := drain(t, mustBuild(t, svc, userID))
	if len(got) != SessionCap {
		t.Fatalf("len = %d, want %d", len(got), SessionCap)
	}
	seen := map[int64]int{}
	for _, card := range got {
		seen[card.CardID]++
		if seen[card.CardID] != 1 {
			t.Fatalf("card %d queued %d times before any miss", card.CardID, seen[card.CardID])
		}
	}
	assertShape(t, got)
}

func TestInterleaveStruggleRun(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-48 * time.Hour)
	for i := 0; i < 15; i++ {
		due := clk.Now().Add(time.Duration(i)*time.Minute - 2*time.Hour)
		addCard(t, store, userID, fmt.Sprintf("s-%02d", i), fsrs.DirHeEn, due, 0, int(fsrs.Hard), reviewed, false)
	}
	for i := 0; i < 2; i++ {
		due := clk.Now().Add(time.Duration(i)*time.Minute - 30*time.Minute)
		addCard(t, store, userID, fmt.Sprintf("k-%02d", i), fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false)
	}
	got := drain(t, mustBuild(t, svc, userID))
	s, k := counts(got)
	if s != 15 || k != 2 {
		t.Fatalf("struggle=%d known=%d (%s)", s, k, pattern(got))
	}
	assertShape(t, got)
	if switches(got) < 2 {
		t.Fatalf("blocked: %s", pattern(got))
	}
}

func TestMixFollowsAccuracy(t *testing.T) {
	cases := []struct {
		name        string
		good, again int
		accuracy    float64
	}{
		{name: "below 80", good: 0, again: RecentReviews, accuracy: 0},
		{name: "between", good: 42, again: 8, accuracy: 42.0 / RecentReviews},
		{name: "above 90", good: RecentReviews, again: 0, accuracy: 1},
	}
	var knownCounts []int
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, store, clk := openSvc(t, 1)
			userID := mustUser(t, store)
			reviewed := clk.Now().Add(-48 * time.Hour)
			for i := 0; i < 20; i++ {
				due := clk.Now().Add(time.Duration(i)*time.Minute - 3*time.Hour)
				addCard(t, store, userID, fmt.Sprintf("k-%02d", i), fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false)
			}
			for i := 0; i < 20; i++ {
				due := clk.Now().Add(time.Duration(i)*time.Minute - 2*time.Hour)
				addCard(t, store, userID, fmt.Sprintf("s-%02d", i), fsrs.DirEnHe, due, 0, int(fsrs.Hard), reviewed, false)
			}
			addAccuracy(t, store, userID, tc.good, tc.again, clk.Now())
			got := drain(t, mustBuild(t, svc, userID))
			assertShape(t, got)
			if len(got) != SessionCap {
				t.Fatalf("len = %d, want %d (%s)", len(got), SessionCap, pattern(got))
			}
			plan := PlanFor(tc.accuracy)
			s, k := counts(got)
			if s != plan.Struggle || k != plan.Known {
				t.Fatalf("struggle=%d known=%d, plan %+v (%s)", s, k, plan, pattern(got))
			}
			knownCounts = append(knownCounts, k)
		})
	}
	if len(knownCounts) != 3 || !(knownCounts[0] > knownCounts[1] && knownCounts[1] > knownCounts[2]) {
		t.Fatalf("known counts = %v, want low > default > high", knownCounts)
	}
}

func TestRequeueMissOnce(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-48 * time.Hour)
	var ids []int64
	for i := 0; i < SessionCap+5; i++ {
		due := clk.Now().Add(time.Duration(i)*time.Minute - 2*time.Hour)
		ids = append(ids, addCard(t, store, userID, fmt.Sprintf("q-%02d", i), fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false))
	}
	sess := mustBuild(t, svc, userID)
	first, ok := sess.Next()
	if !ok || first.CardID != ids[0] {
		t.Fatalf("first = %+v ok=%v, want %d", first, ok, ids[0])
	}
	if err := sess.Record(ctx, ids[1], fsrs.Again); err == nil {
		t.Fatal("graded a card that was not just served")
	}
	logs, err := store.ReviewLogs(ctx, ids[1])
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("unserved card gained a review: %+v", logs)
	}
	if err := sess.Record(ctx, first.CardID, fsrs.Again); err != nil {
		t.Fatal(err)
	}

	between := 0
	seen := 1
	visits := []int64{first.CardID}
	unique := map[int64]struct{}{first.CardID: {}}
	for {
		card, ok := sess.Next()
		if !ok {
			break
		}
		visits = append(visits, card.CardID)
		unique[card.CardID] = struct{}{}
		if card.CardID != first.CardID {
			if seen == 1 {
				between++
			}
			continue
		}
		seen++
		if seen == 2 && between < RequeueGap {
			t.Fatalf("requeued after %d cards, want at least %d", between, RequeueGap)
		}
		if seen > 2 {
			t.Fatalf("card %d appeared %d times", first.CardID, seen)
		}
		if err := sess.Record(ctx, card.CardID, fsrs.Again); err != nil {
			t.Fatal(err)
		}
	}
	if seen != 2 {
		t.Fatalf("appearances = %d, want 2", seen)
	}
	if between < RequeueGap {
		t.Fatalf("gap = %d, want at least %d", between, RequeueGap)
	}
	if len(unique) != SessionCap || len(visits) != SessionCap+1 {
		t.Fatalf("unique=%d visits=%d, want the requeue to count once toward the cap", len(unique), len(visits))
	}
	logs, err = store.ReviewLogs(ctx, first.CardID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 3 || logs[1].Rating != int(fsrs.Again) || logs[2].Rating != int(fsrs.Again) {
		t.Fatalf("log = %+v, want the original grade plus two Again ratings", logs)
	}
}

func TestConfusionMarksStruggling(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-time.Hour)
	due := clk.Now().Add(-time.Minute)
	he := addCard(t, store, userID, "בלבול", fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false)
	// Same lexeme, other direction. Insert directly so the text stays unique.
	lexemeID := lexemeOf(t, store, he)
	en, err := store.InsertCard(ctx, db.Card{
		UserID: userID, LexemeID: lexemeID, Dir: fsrs.DirEnHe, State: fsrs.StateReview,
		Due: due, Stability: 1, Difficulty: 5, Reps: 1, LastReview: reviewed,
	})
	if err != nil {
		t.Fatal(err)
	}
	insertReview(t, store, en, int(fsrs.Good), reviewed)
	other := addCard(t, store, userID, "יציב", fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false)
	if err := fsrs.New(store, clk).RecordMiss(ctx, userID, fsrs.MissLetterPair, "ב/כ", lexemeID); err != nil {
		t.Fatal(err)
	}

	got := drain(t, mustBuild(t, svc, userID))
	flags := map[int64]bool{}
	for _, card := range got {
		flags[card.CardID] = card.Struggling
	}
	if !flags[he] || !flags[en] {
		t.Fatalf("confusion did not mark both directions: %+v", got)
	}
	if flags[other] {
		t.Fatalf("unrelated card was struggling: %+v", got)
	}

	clk.Advance(8 * 24 * time.Hour)
	got = drain(t, mustBuild(t, svc, userID))
	seen := map[int64]bool{}
	for _, card := range got {
		seen[card.CardID] = card.Struggling
	}
	for _, id := range []int64{he, en, other} {
		struggling, ok := seen[id]
		if !ok {
			t.Fatalf("card %d left the queue after the confusion window: %+v", id, got)
		}
		if struggling {
			t.Fatalf("card %d stayed struggling after 8 days: %+v", id, got)
		}
	}
}

func TestLapseRules(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	now := clk.Now()
	due := now.Add(-time.Minute)
	lapsed := addCard(t, store, userID, "נפילות", fsrs.DirHeEn, due, StruggleLapses, int(fsrs.Good), now.Add(-time.Hour), false)
	recent := addCard(t, store, userID, "טרי", fsrs.DirHeEn, due, 1, int(fsrs.Again), now.Add(-48*time.Hour), false)
	insertReview(t, store, recent, int(fsrs.Good), now.Add(-24*time.Hour))
	old := addCard(t, store, userID, "ישן", fsrs.DirHeEn, due, 1, int(fsrs.Again), now.Add(-8*24*time.Hour), false)
	insertReview(t, store, old, int(fsrs.Good), now.Add(-time.Hour))
	clean := addCard(t, store, userID, "נקי", fsrs.DirHeEn, due, 0, int(fsrs.Good), now.Add(-time.Hour), false)

	got := drain(t, mustBuild(t, svc, userID))
	flags := map[int64]bool{}
	for _, card := range got {
		flags[card.CardID] = card.Struggling
	}
	if _, ok := flags[recent]; ok {
		t.Fatalf("recent lapse was queued: %+v", got)
	}
	if struggling, ok := flags[lapsed]; !ok || !struggling {
		t.Fatalf("lapses >= %d card = present %v struggling %v (%+v)", StruggleLapses, ok, struggling, got)
	}
	if flags[old] || flags[clean] {
		t.Fatalf("recovered or clean card was struggling: %+v", got)
	}
	if _, ok := flags[old]; !ok {
		t.Fatal("an Again older than the window excluded a Good card")
	}
	if _, ok := flags[clean]; !ok {
		t.Fatal("clean known card was left out")
	}
}

func TestNotDueKnownOnlyFillTheGap(t *testing.T) {
	t.Run("fill the known quota", func(t *testing.T) {
		svc, store, clk := openSvc(t, 1)
		userID := mustUser(t, store)
		reviewed := clk.Now().Add(-time.Hour)
		var dueIDs, laterIDs []int64
		for i := 0; i < 2; i++ {
			due := clk.Now().Add(time.Duration(i)*time.Minute - 2*time.Hour)
			dueIDs = append(dueIDs, addCard(t, store, userID, fmt.Sprintf("due-%02d", i), fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false))
		}
		for i := 0; i < 30; i++ {
			due := clk.Now().Add(time.Duration(i+1) * time.Minute)
			laterIDs = append(laterIDs, addCard(t, store, userID, fmt.Sprintf("later-%02d", i), fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false))
		}
		got := drain(t, mustBuild(t, svc, userID))
		want := PlanFor(1).Known
		if len(got) != want {
			t.Fatalf("len = %d, want known quota %d", len(got), want)
		}
		if got[0].CardID != dueIDs[0] || got[1].CardID != dueIDs[1] {
			t.Fatalf("due cards = %d %d, want %v", got[0].CardID, got[1].CardID, dueIDs)
		}
		seen := map[int64]bool{}
		for _, card := range got {
			seen[card.CardID] = true
			if card.Struggling {
				t.Fatalf("known filler marked struggling: %+v", card)
			}
		}
		for _, id := range dueIDs {
			if !seen[id] {
				t.Fatalf("due card %d missing", id)
			}
		}
		extra := 0
		for _, id := range laterIDs {
			if seen[id] {
				extra++
			}
		}
		if extra != want-len(dueIDs) {
			t.Fatalf("later cards = %d, want %d", extra, want-len(dueIDs))
		}
	})
	t.Run("skip later cards when due known fill the cap", func(t *testing.T) {
		svc, store, clk := openSvc(t, 1)
		userID := mustUser(t, store)
		reviewed := clk.Now().Add(-time.Hour)
		dueSet := map[int64]bool{}
		for i := 0; i < 20; i++ {
			due := clk.Now().Add(time.Duration(i)*time.Minute - 2*time.Hour)
			dueSet[addCard(t, store, userID, fmt.Sprintf("due-%02d", i), fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false)] = true
		}
		for i := 0; i < 10; i++ {
			due := clk.Now().Add(time.Duration(i+1) * time.Hour)
			addCard(t, store, userID, fmt.Sprintf("later-%02d", i), fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false)
		}
		got := drain(t, mustBuild(t, svc, userID))
		if len(got) != SessionCap {
			t.Fatalf("len = %d, want %d", len(got), SessionCap)
		}
		for _, card := range got {
			if !dueSet[card.CardID] {
				t.Fatalf("not-yet-due card %d was queued", card.CardID)
			}
		}
	})
}

func TestEmptyDeck(t *testing.T) {
	svc, store, _ := openSvc(t, 1)
	userID := mustUser(t, store)
	sess := mustBuild(t, svc, userID)
	if card, ok := sess.Next(); ok {
		t.Fatalf("empty deck served %+v", card)
	}
	if _, ok := sess.Next(); ok {
		t.Fatal("second next on an empty deck returned a card")
	}
	suspended := addCard(t, store, userID, "מוסתר", fsrs.DirHeEn, time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC), 0, int(fsrs.Good), time.Date(2026, 3, 1, 7, 0, 0, 0, time.UTC), true)
	sess = mustBuild(t, svc, userID)
	if card, ok := sess.Next(); ok {
		t.Fatalf("suspended card %d served as %+v", suspended, card)
	}
}

func TestDeterministic(t *testing.T) {
	_, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-2 * time.Hour)
	due := clk.Now().Add(-time.Hour)
	for i := 0; i < 25; i++ {
		addCard(t, store, userID, fmt.Sprintf("tie-%02d", i), fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false)
	}
	a := idsOf(drain(t, mustBuild(t, New(store, clk, 1), userID)))
	b := idsOf(drain(t, mustBuild(t, New(store, clk, 1), userID)))
	c := idsOf(drain(t, mustBuild(t, New(store, clk, 2), userID)))
	if !sameIDs(a, b) {
		t.Fatalf("same clock and seed differed\n%v\n%v", a, b)
	}
	if sameIDs(a, c) {
		t.Fatalf("seed 2 matched seed 1: %v", a)
	}
	if len(a) != SessionCap {
		t.Fatalf("len = %d, want %d", len(a), SessionCap)
	}
}

func TestConcurrentRecordWritesOnce(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-48 * time.Hour)
	var ids []int64
	for i := 0; i < 8; i++ {
		due := clk.Now().Add(time.Duration(i)*time.Minute - 2*time.Hour)
		ids = append(ids, addCard(t, store, userID, fmt.Sprintf("race-%02d", i), fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false))
	}
	sess := mustBuild(t, svc, userID)
	first, ok := sess.Next()
	if !ok || first.CardID != ids[0] {
		t.Fatalf("first = %+v ok=%v, want %d", first, ok, ids[0])
	}
	before, err := store.ReviewLogs(ctx, first.CardID)
	if err != nil {
		t.Fatal(err)
	}

	const n = 4
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = sess.Record(ctx, first.CardID, fsrs.Again)
		}(i)
	}
	close(start)
	wg.Wait()

	okN := 0
	for _, err := range errs {
		if err == nil {
			okN++
		}
	}
	if okN != 1 {
		t.Fatalf("successes = %d, want 1 (%v)", okN, errs)
	}
	logs, err := store.ReviewLogs(ctx, first.CardID)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != len(before)+1 {
		t.Fatalf("review_log rows = %d, want exactly 1 new row (had %d)", len(logs), len(before))
	}
	requeues := 0
	for {
		card, ok := sess.Next()
		if !ok {
			break
		}
		if card.CardID == first.CardID {
			requeues++
		}
	}
	if requeues != 1 {
		t.Fatalf("re-queues = %d, want at most 1 and the successful Again to insert once", requeues)
	}
}

func TestRecordRestoresClaimWhenGradeFails(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-time.Hour)
	due := clk.Now().Add(-time.Minute)
	id := addCard(t, store, userID, "נכשל", fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false)
	sess := mustBuild(t, svc, userID)
	card, ok := sess.Next()
	if !ok || card.CardID != id {
		t.Fatalf("first = %+v ok=%v", card, ok)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sess.Record(ctx, card.CardID, fsrs.Again); err == nil {
		t.Fatal("cancelled context graded the card")
	}
	if err := sess.Record(context.Background(), card.CardID, fsrs.Good); err != nil {
		t.Fatal(err)
	}
	logs, err := store.ReviewLogs(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 || logs[1].Rating != int(fsrs.Good) {
		t.Fatalf("log = %+v, want the fixture grade plus one Good", logs)
	}
}

func TestLatestRatingIgnoresTheFuture(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	now := clk.Now()
	id := addCard(t, store, userID, "עתיד", fsrs.DirEnHe, now.Add(-time.Minute), 0, int(fsrs.Good), now.Add(-time.Hour), false)
	insertReview(t, store, id, int(fsrs.Hard), now.Add(24*time.Hour))
	sess := mustBuild(t, svc, userID)
	card, ok := sess.Next()
	if !ok || card.CardID != id || card.Struggling {
		t.Fatalf("card = %+v ok=%v, want the Good at or before now", card, ok)
	}
}

func TestBuildSessionUsesOneNow(t *testing.T) {
	storeClk := &fakeClock{t: time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)}
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), storeClk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	// Eight days ahead of the store. A confusion at the store's now is inside
	// the store's 7-day window and outside the window ending at this instant.
	svcClk := &fakeClock{t: storeClk.Now().Add(8 * 24 * time.Hour)}
	svc := New(store, svcClk, 1)
	userID := mustUser(t, store)
	now := svcClk.Now()
	id := addCard(t, store, userID, "שעון", fsrs.DirEnHe, now.Add(-time.Minute), 0, int(fsrs.Good), now.Add(-time.Hour), false)
	if err := fsrs.New(store, storeClk).RecordMiss(context.Background(), userID, fsrs.MissLetterPair, "ב/כ", lexemeOf(t, store, id)); err != nil {
		t.Fatal(err)
	}
	sess := mustBuild(t, svc, userID)
	card, ok := sess.Next()
	if !ok || card.CardID != id || card.Struggling {
		t.Fatalf("card = %+v ok=%v, want the confusion aged out on the build clock", card, ok)
	}
}

func TestDrillsFollowThePlan(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-48 * time.Hour)
	for i := 0; i < 20; i++ {
		due := clk.Now().Add(time.Duration(i)*time.Minute - 3*time.Hour)
		addCard(t, store, userID, fmt.Sprintf("k-%02d", i), fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false)
	}
	for i := 0; i < 20; i++ {
		due := clk.Now().Add(time.Duration(i)*time.Minute - 2*time.Hour)
		addCard(t, store, userID, fmt.Sprintf("s-%02d", i), fsrs.DirEnHe, due, 0, int(fsrs.Hard), reviewed, false)
	}
	rev := fsrs.New(store, clk)
	for i := 0; i < MinMissesForDrill; i++ {
		if err := rev.RecordMiss(ctx, userID, fsrs.MissLetterPair, "ב/כ", 0); err != nil {
			t.Fatal(err)
		}
		if err := rev.RecordMiss(ctx, userID, fsrs.MissAgreement, "הוא/היא", 0); err != nil {
			t.Fatal(err)
		}
	}
	// One miss of another pair is under the threshold.
	if err := rev.RecordMiss(ctx, userID, fsrs.MissLetterPair, "ג/ק", 0); err != nil {
		t.Fatal(err)
	}

	got := drainAll(t, mustBuild(t, svc, userID))
	// Twenty Good and twenty Hard reviews: accuracy 0.5, under the low band.
	plan := PlanFor(0.5)
	s, k, letters, patterns := 0, 0, 0, 0
	letterSteps := map[int]string{}
	patternSteps := map[int]string{}
	for _, card := range got {
		switch card.Kind {
		case KindLetter:
			letters++
			letterSteps[card.Step] = card.Key
			if card.CardID >= 0 {
				t.Fatalf("letter drill used a real card id: %+v", card)
			}
		case KindPattern:
			patterns++
			patternSteps[card.Step] = card.Key
		default:
			if card.Struggling {
				s++
			} else {
				k++
			}
		}
	}
	if k != plan.Known || s != plan.Struggle {
		t.Fatalf("known=%d struggle=%d, plan %+v", k, s, plan)
	}
	if letters != DrillSteps || patterns != DrillSteps {
		t.Fatalf("letters=%d patterns=%d, want %d each", letters, patterns, DrillSteps)
	}
	for step := 0; step < DrillSteps; step++ {
		if letterSteps[step] != "ב/כ" || patternSteps[step] != "הוא/היא" {
			t.Fatalf("step %d letter %q pattern %q", step, letterSteps[step], patternSteps[step])
		}
	}
	// The study cards stay in their planned order, with a letter and a
	// pattern woven after each of the first drill steps.
	if got[0].Kind == KindLetter || got[0].Kind == KindPattern {
		t.Fatalf("drills replaced the first study card: %+v", got[0])
	}
	if got[1].Kind != KindLetter || got[2].Kind != KindPattern {
		t.Fatalf("weave = %s %s, want letter then pattern", got[1].Kind, got[2].Kind)
	}
}

func TestDrillCap(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-48 * time.Hour)
	for i := 0; i < 20; i++ {
		due := clk.Now().Add(time.Duration(i)*time.Minute - 3*time.Hour)
		addCard(t, store, userID, fmt.Sprintf("k-%02d", i), fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false)
	}
	for i := 0; i < 20; i++ {
		due := clk.Now().Add(time.Duration(i)*time.Minute - 2*time.Hour)
		addCard(t, store, userID, fmt.Sprintf("s-%02d", i), fsrs.DirEnHe, due, 0, int(fsrs.Hard), reviewed, false)
	}
	// Six pairs at the miss threshold are 24 letter steps. DrillCap keeps 20.
	pairs := []string{"ב/כ", "ג/ק", "ד/ת", "א/ע", "ח/כ", "ט/ת"}
	rev := fsrs.New(store, clk)
	for _, key := range pairs {
		for i := 0; i < MinMissesForDrill; i++ {
			if err := rev.RecordMiss(ctx, userID, fsrs.MissLetterPair, key, 0); err != nil {
				t.Fatal(err)
			}
		}
	}
	got := drainAll(t, mustBuild(t, svc, userID))
	plan := PlanFor(0.5)
	if plan.Known != 19 || plan.Struggle != 1 {
		t.Fatalf("plan = %+v, want 19/1", plan)
	}
	s, k, letters := 0, 0, 0
	for _, card := range got {
		switch card.Kind {
		case KindLetter:
			letters++
		case KindPattern:
			t.Fatalf("pattern drill in a letter-only cap: %+v", card)
		default:
			if card.Struggling {
				s++
			} else {
				k++
			}
		}
	}
	if k != plan.Known || s != plan.Struggle {
		t.Fatalf("known=%d struggle=%d, plan %+v", k, s, plan)
	}
	if letters != DrillCap {
		t.Fatalf("letters = %d, want DrillCap %d", letters, DrillCap)
	}
}

func TestDrillsAtBandPlans(t *testing.T) {
	cases := []struct {
		name         string
		good, again  int
		wantK, wantS int
	}{
		{name: "in-band 16/4", good: 45, again: 5, wantK: 16, wantS: 4},
		{name: "high 13/7", good: 48, again: 2, wantK: 13, wantS: 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, store, clk := openSvc(t, 1)
			ctx := context.Background()
			userID := mustUser(t, store)
			now := clk.Now()
			addAccuracy(t, store, userID, tc.good, tc.again, now)
			reviewed := now.Add(-48 * time.Hour)
			for i := 0; i < 20; i++ {
				due := now.Add(time.Duration(i)*time.Minute - 3*time.Hour)
				addCard(t, store, userID, fmt.Sprintf("k-%02d", i), fsrs.DirHeEn, due, 0, int(fsrs.Good), reviewed, false)
			}
			for i := 0; i < 20; i++ {
				due := now.Add(time.Duration(i)*time.Minute - 2*time.Hour)
				addCard(t, store, userID, fmt.Sprintf("s-%02d", i), fsrs.DirEnHe, due, 0, int(fsrs.Hard), reviewed, false)
			}
			rev := fsrs.New(store, clk)
			for i := 0; i < MinMissesForDrill; i++ {
				if err := rev.RecordMiss(ctx, userID, fsrs.MissLetterPair, "ב/כ", 0); err != nil {
					t.Fatal(err)
				}
				if err := rev.RecordMiss(ctx, userID, fsrs.MissAgreement, "הוא/היא", 0); err != nil {
					t.Fatal(err)
				}
			}
			got := drainAll(t, mustBuild(t, svc, userID))
			accuracy := float64(tc.good) / float64(tc.good+tc.again)
			plan := PlanFor(accuracy)
			if plan.Known != tc.wantK || plan.Struggle != tc.wantS {
				t.Fatalf("plan = %+v, want %d/%d", plan, tc.wantK, tc.wantS)
			}
			s, k, letters, patterns := 0, 0, 0, 0
			for _, card := range got {
				switch card.Kind {
				case KindLetter:
					letters++
				case KindPattern:
					patterns++
				default:
					if card.Struggling {
						s++
					} else {
						k++
					}
				}
			}
			if k != plan.Known || s != plan.Struggle {
				t.Fatalf("known=%d struggle=%d, plan %+v", k, s, plan)
			}
			if letters != DrillSteps || patterns != DrillSteps {
				t.Fatalf("letters=%d patterns=%d, want %d each", letters, patterns, DrillSteps)
			}
		})
	}
}

func TestRevealMarksPeeked(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-48 * time.Hour)
	first := addCard(t, store, userID, "גלוי", fsrs.DirHeEn, clk.Now().Add(-2*time.Hour), 0, int(fsrs.Good), reviewed, false)
	second := addCard(t, store, userID, "נסתר", fsrs.DirHeEn, clk.Now().Add(-time.Hour), 0, int(fsrs.Good), reviewed, false)
	sess := mustBuild(t, svc, userID)

	hidden, ok := sess.Next()
	if !ok || hidden.CardID != first {
		t.Fatalf("first = %+v ok=%v", hidden, ok)
	}
	if err := sess.Record(ctx, hidden.CardID, fsrs.Good); err != nil {
		t.Fatal(err)
	}
	hiddenLogs := mustReviewLogs(t, store, first)
	if len(hiddenLogs) != 2 || hiddenLogs[1].Peeked {
		t.Fatalf("unrevealed log = %+v, want peeked false", hiddenLogs)
	}

	shown, ok := sess.Next()
	if !ok || shown.CardID != second {
		t.Fatalf("second = %+v ok=%v", shown, ok)
	}
	if err := sess.Reveal(otherCard(shown.CardID)); !errors.Is(err, ErrNotServed) {
		t.Fatalf("reveal of another card = %v", err)
	}
	if err := sess.Reveal(shown.CardID); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := sess.Record(cancelled, shown.CardID, fsrs.Good); err == nil {
		t.Fatal("cancelled context graded the revealed card")
	}
	if err := sess.Record(ctx, shown.CardID, fsrs.Good); err != nil {
		t.Fatal(err)
	}
	shownLogs := mustReviewLogs(t, store, second)
	if len(shownLogs) != 2 || !shownLogs[1].Peeked || shownLogs[1].Rating != int(fsrs.Good) {
		t.Fatalf("revealed log = %+v, want peeked true", shownLogs)
	}
}

func otherCard(id int64) int64 {
	if id == 0 {
		return 1
	}
	return id + 1
}

func mustReviewLogs(t *testing.T, store *db.Store, cardID int64) []db.ReviewLog {
	t.Helper()
	logs, err := store.ReviewLogs(context.Background(), cardID)
	if err != nil {
		t.Fatal(err)
	}
	return logs
}

func drainAll(t *testing.T, sess *Session) []CardRef {
	t.Helper()
	var out []CardRef
	for {
		card, ok := sess.Next()
		if !ok {
			return out
		}
		out = append(out, card)
		if len(out) > SessionCap+DrillCap+DrillSteps {
			t.Fatalf("queue did not end")
		}
	}
}

func lexemeOf(t *testing.T, store *db.Store, cardID int64) int64 {
	t.Helper()
	card, err := store.GetCard(context.Background(), cardID)
	if err != nil {
		t.Fatal(err)
	}
	return card.LexemeID
}
