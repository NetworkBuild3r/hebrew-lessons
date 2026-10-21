package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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

func TestNilServiceSeed(t *testing.T) {
	var s *Service
	if got := s.Seed(); got != 0 {
		t.Fatalf("nil service seed = %d, want 0", got)
	}
}

func TestSeedReportsTheConstructorSeed(t *testing.T) {
	svc, _, _ := openSvc(t, 42)
	if svc.Seed() != 42 {
		t.Fatalf("seed = %d, want 42", svc.Seed())
	}
	over := New(nil, nil, 1, WithSeed(7))
	if over.Seed() != 7 {
		t.Fatalf("WithSeed = %d, want 7", over.Seed())
	}
}

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

func addCard(t *testing.T, store *db.Store, userID int64, text, skill string, due time.Time, lapses, rating int, reviewed time.Time, suspended bool) int64 {
	t.Helper()
	id, err := store.InsertCard(context.Background(), db.Card{
		UserID:     userID,
		LexemeID:   mustLexeme(t, store, text),
		Skill:      skill,
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
		Skill:      fsrs.SkillRead,
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
		if len(out) > SessionCap+MaxNewPerSession+DrillCap+DrillSteps {
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
		addCard(t, store, userID, fmt.Sprintf("cap-%02d", i), fsrs.SkillRead, due, 0, rating, reviewed, false)
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
		addCard(t, store, userID, fmt.Sprintf("s-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Hard), reviewed, false)
	}
	for i := 0; i < 2; i++ {
		due := clk.Now().Add(time.Duration(i)*time.Minute - 30*time.Minute)
		addCard(t, store, userID, fmt.Sprintf("k-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
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
				addCard(t, store, userID, fmt.Sprintf("k-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
			}
			for i := 0; i < 20; i++ {
				due := clk.Now().Add(time.Duration(i)*time.Minute - 2*time.Hour)
				addCard(t, store, userID, fmt.Sprintf("s-%02d", i), fsrs.SkillRecall, due, 0, int(fsrs.Hard), reviewed, false)
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
		ids = append(ids, addCard(t, store, userID, fmt.Sprintf("q-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false))
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
	revealAndRecord(t, sess, ctx, first.CardID, fsrs.Again)

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
		revealAndRecord(t, sess, ctx, card.CardID, fsrs.Again)
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
	he := addCard(t, store, userID, "בלבול", fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
	// Same lexeme, other direction. Insert directly so the text stays unique.
	lexemeID := lexemeOf(t, store, he)
	en, err := store.InsertCard(ctx, db.Card{
		UserID: userID, LexemeID: lexemeID, Skill: fsrs.SkillRecall, State: fsrs.StateReview,
		Due: due, Stability: 1, Difficulty: 5, Reps: 1, LastReview: reviewed,
	})
	if err != nil {
		t.Fatal(err)
	}
	insertReview(t, store, en, int(fsrs.Good), reviewed)
	other := addCard(t, store, userID, "יציב", fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
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
	lapsed := addCard(t, store, userID, "נפילות", fsrs.SkillRead, due, StruggleLapses, int(fsrs.Good), now.Add(-time.Hour), false)
	recent := addCard(t, store, userID, "טרי", fsrs.SkillRead, due, 1, int(fsrs.Again), now.Add(-48*time.Hour), false)
	insertReview(t, store, recent, int(fsrs.Good), now.Add(-24*time.Hour))
	old := addCard(t, store, userID, "ישן", fsrs.SkillRead, due, 1, int(fsrs.Again), now.Add(-8*24*time.Hour), false)
	insertReview(t, store, old, int(fsrs.Good), now.Add(-time.Hour))
	clean := addCard(t, store, userID, "נקי", fsrs.SkillRead, due, 0, int(fsrs.Good), now.Add(-time.Hour), false)

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
			dueIDs = append(dueIDs, addCard(t, store, userID, fmt.Sprintf("due-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false))
		}
		for i := 0; i < 30; i++ {
			due := clk.Now().Add(time.Duration(i+1) * time.Minute)
			laterIDs = append(laterIDs, addCard(t, store, userID, fmt.Sprintf("later-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false))
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
			dueSet[addCard(t, store, userID, fmt.Sprintf("due-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)] = true
		}
		for i := 0; i < 10; i++ {
			due := clk.Now().Add(time.Duration(i+1) * time.Hour)
			addCard(t, store, userID, fmt.Sprintf("later-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
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
	suspended := addCard(t, store, userID, "מוסתר", fsrs.SkillRead, time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC), 0, int(fsrs.Good), time.Date(2026, 3, 1, 7, 0, 0, 0, time.UTC), true)
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
		addCard(t, store, userID, fmt.Sprintf("tie-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
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
		ids = append(ids, addCard(t, store, userID, fmt.Sprintf("race-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false))
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
	if err := sess.Reveal(first.CardID); err != nil {
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
	id := addCard(t, store, userID, "נכשל", fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
	sess := mustBuild(t, svc, userID)
	card, ok := sess.Next()
	if !ok || card.CardID != id {
		t.Fatalf("first = %+v ok=%v", card, ok)
	}
	if err := sess.Reveal(card.CardID); err != nil {
		t.Fatal(err)
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
	id := addCard(t, store, userID, "עתיד", fsrs.SkillRecall, now.Add(-time.Minute), 0, int(fsrs.Good), now.Add(-time.Hour), false)
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
	id := addCard(t, store, userID, "שעון", fsrs.SkillRecall, now.Add(-time.Minute), 0, int(fsrs.Good), now.Add(-time.Hour), false)
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
		addCard(t, store, userID, fmt.Sprintf("k-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
	}
	for i := 0; i < 20; i++ {
		due := clk.Now().Add(time.Duration(i)*time.Minute - 2*time.Hour)
		addCard(t, store, userID, fmt.Sprintf("s-%02d", i), fsrs.SkillRecall, due, 0, int(fsrs.Hard), reviewed, false)
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
		addCard(t, store, userID, fmt.Sprintf("k-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
	}
	for i := 0; i < 20; i++ {
		due := clk.Now().Add(time.Duration(i)*time.Minute - 2*time.Hour)
		addCard(t, store, userID, fmt.Sprintf("s-%02d", i), fsrs.SkillRecall, due, 0, int(fsrs.Hard), reviewed, false)
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
				addCard(t, store, userID, fmt.Sprintf("k-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
			}
			for i := 0; i < 20; i++ {
				due := now.Add(time.Duration(i)*time.Minute - 2*time.Hour)
				addCard(t, store, userID, fmt.Sprintf("s-%02d", i), fsrs.SkillRecall, due, 0, int(fsrs.Hard), reviewed, false)
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

func TestRecordRejectsUnrevealed(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-48 * time.Hour)
	first := addCard(t, store, userID, "נסתר", fsrs.SkillRead, clk.Now().Add(-time.Hour), 0, int(fsrs.Good), reviewed, false)
	sess := mustBuild(t, svc, userID)
	card, ok := sess.Next()
	if !ok || card.CardID != first {
		t.Fatalf("first = %+v ok=%v", card, ok)
	}
	before, err := store.GetCard(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	err = sess.Record(ctx, card.CardID, fsrs.Good)
	if !errors.Is(err, ErrNotRevealed) {
		t.Fatalf("unrevealed grade = %v", err)
	}
	logs := mustReviewLogs(t, store, first)
	if len(logs) != 1 {
		t.Fatalf("unrevealed grade wrote a log: %+v", logs)
	}
	after, err := store.GetCard(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if after.Due != before.Due || after.Stability != before.Stability || after.Reps != before.Reps {
		t.Fatalf("unrevealed grade changed the card: before %+v after %+v", before, after)
	}
	// The serve is still the current card, so reveal-then-grade can proceed.
	if err := sess.Reveal(card.CardID); err != nil {
		t.Fatal(err)
	}
	if err := sess.Record(ctx, card.CardID, fsrs.Good); err != nil {
		t.Fatalf("reveal after a rejected grade = %v", err)
	}
}

func TestRevealThenGrade(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-48 * time.Hour)
	first := addCard(t, store, userID, "גלוי", fsrs.SkillRead, clk.Now().Add(-time.Hour), 0, int(fsrs.Good), reviewed, false)
	sess := mustBuild(t, svc, userID)
	shown, ok := sess.Next()
	if !ok || shown.CardID != first {
		t.Fatalf("first = %+v ok=%v", shown, ok)
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
	shownLogs := mustReviewLogs(t, store, first)
	if len(shownLogs) != 2 || !shownLogs[1].Peeked || shownLogs[1].Rating != int(fsrs.Good) {
		t.Fatalf("revealed log = %+v, want peeked true", shownLogs)
	}
}

func revealAndRecord(t *testing.T, sess *Session, ctx context.Context, cardID int64, rating fsrs.Rating) {
	t.Helper()
	if err := sess.Reveal(cardID); err != nil {
		t.Fatal(err)
	}
	if err := sess.Record(ctx, cardID, rating); err != nil {
		t.Fatal(err)
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

func TestFinishedLessonWordsEnterReview(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	now := clk.Now()
	// The later unit is enqueued first, so its card ids are lower.
	// Lesson position has to win over that id order.
	lateWord := mustLexeme(t, store, "מְאֻחָר")
	earlyWord := mustLexeme(t, store, "מֻקְדָּם")
	waitingWord := mustLexeme(t, store, "מַמְתִּין")
	lateUnit := insertUnitAt(t, store, 4)
	earlyUnit := insertUnitAt(t, store, 1)
	waitingUnit := insertUnitAt(t, store, 0)
	finishLesson(t, store, userID, lateUnit, 0, []int64{lateWord}, now)
	finishLesson(t, store, userID, earlyUnit, 0, []int64{earlyWord}, now)
	if err := testharness.InsertLessonItem(ctx, store, waitingUnit, 0, waitingWord, "new", "recall"); err != nil {
		t.Fatal(err)
	}
	lateHe, lateEn := enqueuePair(t, store, userID, lateWord, now)
	earlyHe, earlyEn := enqueuePair(t, store, userID, earlyWord, now)
	waitingHe, waitingEn := enqueuePair(t, store, userID, waitingWord, now)

	got := drain(t, mustBuild(t, svc, userID))
	// Lesson position still beats card id. The two skills of one word are
	// not left adjacent: early read, late read, early recall, late recall.
	want := []int64{earlyHe, lateHe, earlyEn, lateEn}
	if ids := idsOf(got); !sameIDs(ids, want) {
		t.Fatalf("queue = %v, want %v", ids, want)
	}
	if i, id := adjacentLexemeAt(got); i >= 0 {
		t.Fatalf("lexeme %d sits adjacent at %d", id, i)
	}
	for _, card := range got {
		if !card.New || (card.Skill != fsrs.SkillRead && card.Skill != fsrs.SkillRecall) {
			t.Fatalf("card %+v, want a new read or recall card", card)
		}
	}
	for _, id := range []int64{waitingHe, waitingEn} {
		for _, card := range got {
			if card.CardID == id {
				t.Fatalf("unfinished lesson card %d was queued", id)
			}
		}
	}
}

func TestNewCardCaps(t *testing.T) {
	t.Run("five", func(t *testing.T) {
		svc, store, clk := openSvc(t, 1)
		userID := mustUser(t, store)
		now := clk.Now()
		unit := insertUnitAt(t, store, 1)
		var words []int64
		for i := 0; i < 4; i++ {
			words = append(words, mustLexeme(t, store, fmt.Sprintf("חָדָשׁ-%d", i)))
		}
		finishLesson(t, store, userID, unit, 0, words, now)
		var ids []int64
		for _, word := range words {
			he, en := enqueuePair(t, store, userID, word, now)
			ids = append(ids, he, en)
		}
		got := drain(t, mustBuild(t, svc, userID))
		if len(got) != MaxNewPerSession {
			t.Fatalf("len = %d, want %d", len(got), MaxNewPerSession)
		}
		// The five earliest new cards, with the two skills of one word split.
		want := []int64{ids[0], ids[2], ids[1], ids[3], ids[4]}
		if !sameIDs(idsOf(got), want) {
			t.Fatalf("queue = %v, want %v", idsOf(got), want)
		}
		for _, card := range got {
			if !card.New {
				t.Fatalf("card %+v, want a new card", card)
			}
		}
		if i, id := adjacentLexemeAt(got); i >= 0 {
			t.Fatalf("lexeme %d sits adjacent at %d", id, i)
		}
	})
	t.Run("quarter", func(t *testing.T) {
		svc, store, clk := openSvc(t, 1)
		userID := mustUser(t, store)
		now := clk.Now()
		reviewed := now.Add(-48 * time.Hour)
		known := map[int64]bool{}
		for i := 0; i < 3; i++ {
			id := addCard(t, store, userID, fmt.Sprintf("יָדוּעַ-%d", i), fsrs.SkillRead, now.Add(-time.Hour), 0, int(fsrs.Good), reviewed, false)
			known[id] = true
		}
		unit := insertUnitAt(t, store, 2)
		var words []int64
		for i := 0; i < 4; i++ {
			words = append(words, mustLexeme(t, store, fmt.Sprintf("קָטָן-%d", i)))
		}
		finishLesson(t, store, userID, unit, 0, words, now)
		var fresh []int64
		for _, word := range words {
			he, en := enqueuePair(t, store, userID, word, now)
			fresh = append(fresh, he, en)
		}
		got := drain(t, mustBuild(t, svc, userID))
		var newIDs []int64
		seenKnown := map[int64]bool{}
		for _, card := range got {
			if card.New {
				newIDs = append(newIDs, card.CardID)
				continue
			}
			if !known[card.CardID] {
				t.Fatalf("unexpected card %+v", card)
			}
			seenKnown[card.CardID] = true
		}
		if len(seenKnown) != len(known) {
			t.Fatalf("known cards = %v, want %v", seenKnown, known)
		}
		// 3 reviewed cards. sessionSize = 3+2 = 5.
		// min(5, max(2, floor(0.25*5))) = min(5, max(2, 1)) = 2.
		// The share alone admitted 1; MinNewPerSession raises it to 2.
		if len(newIDs) != 2 || newIDs[0] != fresh[0] || newIDs[1] != fresh[1] {
			t.Fatalf("new = %v, want the first two new cards %v", newIDs, fresh[:2])
		}
	})
}

func TestNewCardFloor(t *testing.T) {
	// sessionSize is the reviewed cards in the built queue plus the new
	// cards. newCount = min(5, max(2, floor(0.25 * sessionSize))) when
	// reviews are due. Zero reviews uses the full cap of 5.
	//
	// Ten reviews: the consistent count is 3. sessionSize = 10+3 = 13,
	// floor(0.25*13) = 3, min(5, max(2, 3)) = 3.
	cases := []struct {
		reviews int
		wantNew int
	}{
		{reviews: 0, wantNew: 5},
		{reviews: 1, wantNew: 2},
		{reviews: 2, wantNew: 2},
		{reviews: 10, wantNew: 3},
	}
	// Four finished-lesson words are eight unreviewed cards, more than
	// every allowance in this table, so the cap is what limits the queue.
	const newWords = 4
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d reviews", tc.reviews), func(t *testing.T) {
			svc, store, clk := openSvc(t, 1)
			userID := mustUser(t, store)
			now := clk.Now()
			reviewedAt := now.Add(-48 * time.Hour)
			known := map[int64]bool{}
			for i := 0; i < tc.reviews; i++ {
				id := addCard(t, store, userID, fmt.Sprintf("נִסְקָר-%d", i), fsrs.SkillRead, now.Add(-time.Hour), 0, int(fsrs.Good), reviewedAt, false)
				known[id] = true
			}
			unit := insertUnitAt(t, store, 1)
			var words []int64
			for i := 0; i < newWords; i++ {
				words = append(words, mustLexeme(t, store, fmt.Sprintf("חָדָשׁ-%d", i)))
			}
			finishLesson(t, store, userID, unit, 0, words, now)
			var fresh []int64
			for _, word := range words {
				he, en := enqueuePair(t, store, userID, word, now)
				fresh = append(fresh, he, en)
			}
			if tc.wantNew >= len(fresh) {
				t.Fatalf("want %d new is not below the %d unreviewed candidates", tc.wantNew, len(fresh))
			}
			got := drain(t, mustBuild(t, svc, userID))
			var newIDs []int64
			seenKnown := map[int64]bool{}
			for _, card := range got {
				if card.New {
					newIDs = append(newIDs, card.CardID)
					continue
				}
				if card.Kind == KindLetter || card.Kind == KindPattern {
					t.Fatalf("drill %+v is not part of this session", card)
				}
				if !known[card.CardID] {
					t.Fatalf("unexpected card %+v", card)
				}
				seenKnown[card.CardID] = true
			}
			if len(seenKnown) != tc.reviews {
				t.Fatalf("reviewed = %d, want %d", len(seenKnown), tc.reviews)
			}
			sessionSize := len(seenKnown) + len(newIDs)
			if len(newIDs) != tc.wantNew {
				t.Fatalf("new = %d, want %d (sessionSize %d, candidates %d)", len(newIDs), tc.wantNew, sessionSize, len(fresh))
			}
			allowed := map[int64]bool{}
			for _, id := range fresh[:tc.wantNew] {
				allowed[id] = true
			}
			for _, id := range newIDs {
				if !allowed[id] {
					t.Fatalf("new card %d is outside the earliest %d of %v", id, tc.wantNew, fresh)
				}
			}
		})
	}
}

func insertUnitAt(t *testing.T, store *db.Store, position int) int64 {
	t.Helper()
	res, err := store.ExecContext(context.Background(), `
		INSERT INTO units (position, title, situation, concept, status, rev, doc)
		VALUES (?, 't', 's', 'c', 'published', 1, '{"title":"t"}')`, position)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func finishLesson(t *testing.T, store *db.Store, userID, unitID int64, lessonIx int, words []int64, when time.Time) {
	t.Helper()
	ctx := context.Background()
	for _, word := range words {
		if err := testharness.InsertLessonItem(ctx, store, unitID, lessonIx, word, "new", "recall"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := testharness.InsertLessonRun(ctx, store, userID, unitID, lessonIx, when.Add(-time.Hour), &when); err != nil {
		t.Fatal(err)
	}
}

// enqueuePair inserts the read card and then the recall card, due now,
// with state new and no review_log row. That is the row enqueueNewWords writes.
func enqueuePair(t *testing.T, store *db.Store, userID, lexemeID int64, due time.Time) (he, en int64) {
	t.Helper()
	ctx := context.Background()
	for _, skill := range []string{fsrs.SkillRead, fsrs.SkillRecall} {
		id, err := store.InsertCard(ctx, db.Card{
			UserID:   userID,
			LexemeID: lexemeID,
			Skill:    skill,
			State:    "new",
			Due:      due,
		})
		if err != nil {
			t.Fatal(err)
		}
		if skill == fsrs.SkillRead {
			he = id
		} else {
			en = id
		}
	}
	return he, en
}

func TestDrillCardIDsStayWithThePair(t *testing.T) {
	pairs := []db.ConfusionPair{
		{Kind: fsrs.MissLetterPair, A: "ג", B: "פ", Count: MinMissesForDrill},
		{Kind: fsrs.MissLetterPair, A: "ב", B: "כ", Count: MinMissesForDrill},
	}
	first := drillCandidates(pairs)
	second := drillCandidates(pairs)
	if len(first) != DrillSteps*2 || len(second) != len(first) {
		t.Fatalf("steps = %d and %d, want %d", len(first), len(second), DrillSteps*2)
	}
	seen := map[int64]string{}
	for i, card := range first {
		if card.ID >= 0 {
			t.Fatalf("step %d id %d is not a drill id", card.Step, card.ID)
		}
		if second[i].ID != card.ID || second[i].Key != card.Key || second[i].Step != card.Step {
			t.Fatalf("rebuild changed %+v to %+v", card, second[i])
		}
		if prev, ok := seen[card.ID]; ok {
			t.Fatalf("id %d used by %s and %s/%d", card.ID, prev, card.Key, card.Step)
		}
		seen[card.ID] = card.Key + "/" + fmt.Sprint(card.Step)
	}
	if first[0].ID == first[DrillSteps].ID {
		t.Fatal("two pairs share a card id")
	}
	if first[0].ID == -1 && first[1].ID == -2 {
		t.Fatal("drill ids are still the sequential -1, -2 series")
	}
}

// TestDrillCardIDSkipsATakenHash asks for an id that is already used.
// The next id is the one below it, and it stays negative.
func TestDrillCardIDSkipsATakenHash(t *testing.T) {
	fresh := map[int64]struct{}{}
	hashed := drillCardID(KindLetter, "ג/פ", 0, fresh)
	if hashed >= 0 {
		t.Fatalf("hashed id %d is not negative", hashed)
	}
	used := map[int64]struct{}{hashed: {}}
	got := drillCardID(KindLetter, "ג/פ", 0, used)
	want := hashed - 1
	if want == 0 {
		want = -1
	}
	if got != want || got == 0 || got >= 0 {
		t.Fatalf("collision id = %d, want %d", got, want)
	}
}

// TestDrillCardIDZeroHashIsNegativeOne is the hash whose low 31 bits are 0.
// 1980525264 is that step for letter ג/פ. The id is -1, not 0 and not 1.
func TestDrillCardIDZeroHashIsNegativeOne(t *testing.T) {
	used := map[int64]struct{}{}
	got := drillCardID(KindLetter, "ג/פ", 1980525264, used)
	if got != -1 {
		t.Fatalf("zero hash id = %d, want -1", got)
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

// learner1 is the review shape that used to come out in one fixed order:
// one due learning card, ten new cards (five words, read and recall), and
// six future review cards.
type learner1 struct {
	learning int64
	future   []int64
	firstNew []int64 // the earliest new pair, one lexeme
}

func seedLearner1(t *testing.T, store *db.Store, now time.Time, userID int64) learner1 {
	t.Helper()
	ctx := context.Background()
	var fix learner1
	lex := mustLexeme(t, store, "לָמַד")
	var err error
	fix.learning, err = store.InsertCard(ctx, db.Card{
		UserID: userID, LexemeID: lex, Skill: fsrs.SkillRead, State: fsrs.StateLearning,
		Due: now.Add(-time.Minute), Stability: 0.5, Difficulty: 5, Reps: 1, Lapses: 0,
		LastReview: now.Add(-2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	insertReview(t, store, fix.learning, int(fsrs.Hard), now.Add(-2*time.Hour))

	for i := 0; i < 6; i++ {
		id := addCard(t, store, userID, fmt.Sprintf("עָתִיד-%d", i), fsrs.SkillRead,
			now.Add(time.Duration(i+1)*time.Hour), 0, int(fsrs.Good), now.Add(-48*time.Hour), false)
		fix.future = append(fix.future, id)
	}

	unit := insertUnitAt(t, store, 1)
	var words []int64
	for i := 0; i < 5; i++ {
		words = append(words, mustLexeme(t, store, fmt.Sprintf("חָדָשׁ-%d", i)))
	}
	finishLesson(t, store, userID, unit, 0, words, now)
	for i, word := range words {
		he, en := enqueuePair(t, store, userID, word, now)
		if i == 0 {
			fix.firstNew = []int64{he, en}
		}
	}
	return fix
}

func queueLearner1(t *testing.T, seed int64) ([]CardRef, learner1) {
	t.Helper()
	svc, store, clk := openSvc(t, 0)
	svc = New(store, clk, 0, WithSeed(seed))
	userID := mustUser(t, store)
	fix := seedLearner1(t, store, clk.Now(), userID)
	return drain(t, mustBuild(t, svc, userID)), fix
}

func TestLearner1ReviewIsShuffledPerSeed(t *testing.T) {
	a, fix := queueLearner1(t, 1)
	b, _ := queueLearner1(t, 1)
	c, _ := queueLearner1(t, 2)
	assertLearner1Queue(t, a, fix)
	assertLearner1Queue(t, c, fix)
	if !sameIDs(idsOf(a), idsOf(b)) {
		t.Fatalf("same seed differed\n%v\n%v", idsOf(a), idsOf(b))
	}
	if sameIDs(idsOf(a), idsOf(c)) {
		t.Fatalf("seeds 1 and 2 built the same queue: %v", idsOf(a))
	}
	// Seats for this fixture. The studied cards stay in due order.
	// Seed 1 places the new pair at slots 2 and 6. Seed 2 places them at 3 and 6.
	if !sameIDs(idsOf(a), []int64{1, 2, 8, 3, 4, 5, 9, 6, 7}) {
		t.Fatalf("seed 1 = %v", idsOf(a))
	}
	if !sameIDs(idsOf(c), []int64{1, 2, 3, 8, 4, 5, 9, 6, 7}) {
		t.Fatalf("seed 2 = %v", idsOf(c))
	}
	if !sameIDs(studiedIDs(a), studiedIDs(c)) {
		t.Fatalf("seed changed the known/struggle order\n%v\n%v", studiedIDs(a), studiedIDs(c))
	}
}

func assertLearner1Queue(t *testing.T, cards []CardRef, fix learner1) {
	t.Helper()
	if sessionLexemeCount(cards) < 2 {
		t.Fatalf("lexemes = %d, want more than one (%s)", sessionLexemeCount(cards), describeQueue(cards))
	}
	if i, id := adjacentLexemeAt(cards); i >= 0 {
		t.Fatalf("lexeme %d sits adjacent at %d: %s", id, i, describeQueue(cards))
	}
	if newCardsAreSuffix(cards) {
		t.Fatalf("new cards are a suffix: %s", describeQueue(cards))
	}
	fresh := 0
	seen := map[int64]int{}
	for _, card := range cards {
		if card.Kind == KindLetter || card.Kind == KindPattern {
			t.Fatalf("drill %+v is not part of this shape", card)
		}
		seen[card.CardID]++
		if seen[card.CardID] != 1 {
			t.Fatalf("card %d queued %d times before a miss", card.CardID, seen[card.CardID])
		}
		if card.New {
			fresh++
			if fresh > MaxNewPerSession {
				t.Fatalf("new cards = %d, cap %d", fresh, MaxNewPerSession)
			}
		}
	}
	studied := 0
	for _, card := range cards {
		if !card.New {
			studied++
		}
	}
	if studied > SessionCap {
		t.Fatalf("studied = %d, cap %d", studied, SessionCap)
	}
	if fresh != newAllowance(studied, 10) {
		t.Fatalf("new = %d, allowance %d for %d studied", fresh, newAllowance(studied, 10), studied)
	}
	if seen[fix.learning] != 1 {
		t.Fatalf("learning card %d seen %d times", fix.learning, seen[fix.learning])
	}
	for _, id := range fix.future {
		if seen[id] != 1 {
			t.Fatalf("future card %d seen %d times", id, seen[id])
		}
	}
	for _, id := range fix.firstNew {
		if seen[id] != 1 {
			t.Fatalf("first new card %d was not queued (seen %d)", id, seen[id])
		}
	}
	if lexemeOfCards(cards, fix.firstNew[0]) != lexemeOfCards(cards, fix.firstNew[1]) {
		t.Fatal("the first new pair is not one lexeme")
	}
	if i := struggleRun(cards); i >= 0 {
		t.Fatalf("struggle run at %d: %s", i, describeQueue(cards))
	}
	// The read card of a new pair stays ahead of its recall card.
	if indexOf(cards, fix.firstNew[0]) > indexOf(cards, fix.firstNew[1]) {
		t.Fatalf("new pair order flipped: %s", describeQueue(cards))
	}
}

func TestAgainReturnsOnceAfterSeededGap(t *testing.T) {
	_, store, clk := openSvc(t, 1)
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-48 * time.Hour)
	for i := 0; i < 12; i++ {
		due := clk.Now().Add(time.Duration(i)*time.Minute - 2*time.Hour)
		addCard(t, store, userID, fmt.Sprintf("פַּעַר-%02d", i), fsrs.SkillRead, due, 0, int(fsrs.Good), reviewed, false)
	}
	// Build every sitting before any grade so the rows stay the fixture.
	seen := map[int][]int64{}
	var sessions []*Session
	var seeds []int64
	for seed := int64(0); seed < 80 && len(seen) < 3; seed++ {
		span := requeueSpan(seed)
		if span < RequeueGap || span > RequeueGap+2 {
			t.Fatalf("seed %d span %d, want 3..5", seed, span)
		}
		if _, ok := seen[span]; ok {
			continue
		}
		seen[span] = append([]int64(nil), seed)
		seeds = append(seeds, seed)
		sessions = append(sessions, mustBuild(t, New(store, clk, seed), userID))
	}
	if len(seen) != 3 {
		t.Fatalf("spans = %v, want a sitting gap of 3, 4, and 5", seen)
	}
	for i, seed := range seeds {
		again := measureAgain(t, sessions[i])
		if again.appearances != 2 {
			t.Fatalf("seed %d appearances = %d, want 2", seed, again.appearances)
		}
		if again.between != requeueSpan(seed) {
			t.Fatalf("seed %d gap = %d, want %d", seed, again.between, requeueSpan(seed))
		}
		if again.between < 3 || again.between > 5 {
			t.Fatalf("seed %d gap = %d, want 3..5", seed, again.between)
		}
	}
	// The same seed queues the miss the same distance away.
	again := measureAgain(t, mustBuild(t, New(store, clk, seeds[0]), userID))
	if again.between != requeueSpan(seeds[0]) || again.appearances != 2 {
		t.Fatalf("repeat seed %+v", again)
	}
}

func TestAgainClampsToTheQueueEnd(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-48 * time.Hour)
	first := addCard(t, store, userID, "רִאשׁוֹן", fsrs.SkillRead, clk.Now().Add(-2*time.Hour), 0, int(fsrs.Good), reviewed, false)
	second := addCard(t, store, userID, "שֵׁנִי", fsrs.SkillRead, clk.Now().Add(-time.Hour), 0, int(fsrs.Good), reviewed, false)
	sess := mustBuild(t, svc, userID)
	card, ok := sess.Next()
	if !ok || card.CardID != first {
		t.Fatalf("first = %+v ok=%v, want %d", card, ok, first)
	}
	revealAndRecord(t, sess, ctx, first, fsrs.Again)
	next, ok := sess.Next()
	if !ok || next.CardID != second {
		t.Fatalf("next = %+v ok=%v, want %d", next, ok, second)
	}
	back, ok := sess.Next()
	if !ok || back.CardID != first {
		t.Fatalf("requeue = %+v ok=%v, want the miss at the end", back, ok)
	}
	if _, ok := sess.Next(); ok {
		t.Fatal("the clamped miss was queued again")
	}
}

func TestOneLexemePoolEnds(t *testing.T) {
	t.Run("one card", func(t *testing.T) {
		svc, store, clk := openSvc(t, 1)
		userID := mustUser(t, store)
		id := addCard(t, store, userID, "יָחִיד", fsrs.SkillRead, clk.Now().Add(-time.Hour), 0, int(fsrs.Good), clk.Now().Add(-48*time.Hour), false)
		sess := mustBuild(t, svc, userID)
		card, ok := sess.Next()
		if !ok || card.CardID != id {
			t.Fatalf("first = %+v ok=%v", card, ok)
		}
		revealAndRecord(t, sess, context.Background(), id, fsrs.Again)
		if again, ok := sess.Next(); ok {
			t.Fatalf("one lexeme came back as %+v", again)
		}
	})
	t.Run("two cards one lexeme", func(t *testing.T) {
		svc, store, clk := openSvc(t, 1)
		ctx := context.Background()
		userID := mustUser(t, store)
		lex := mustLexeme(t, store, "זּוּג")
		reviewed := clk.Now().Add(-48 * time.Hour)
		read, err := store.InsertCard(ctx, db.Card{
			UserID: userID, LexemeID: lex, Skill: fsrs.SkillRead, State: fsrs.StateReview,
			Due: clk.Now().Add(-2 * time.Hour), Stability: 1, Difficulty: 5, Reps: 1, LastReview: reviewed,
		})
		if err != nil {
			t.Fatal(err)
		}
		insertReview(t, store, read, int(fsrs.Good), reviewed)
		recall, err := store.InsertCard(ctx, db.Card{
			UserID: userID, LexemeID: lex, Skill: fsrs.SkillRecall, State: fsrs.StateReview,
			Due: clk.Now().Add(-time.Hour), Stability: 1, Difficulty: 5, Reps: 1, LastReview: reviewed,
		})
		if err != nil {
			t.Fatal(err)
		}
		insertReview(t, store, recall, int(fsrs.Good), reviewed)
		sess := mustBuild(t, svc, userID)
		first, ok := sess.Next()
		if !ok || first.CardID != read {
			t.Fatalf("first = %+v ok=%v, want %d", first, ok, read)
		}
		revealAndRecord(t, sess, ctx, read, fsrs.Again)
		second, ok := sess.Next()
		if !ok || second.CardID != recall {
			t.Fatalf("second = %+v ok=%v, want %d", second, ok, recall)
		}
		if extra, ok := sess.Next(); ok {
			t.Fatalf("one lexeme queued %+v after the pair", extra)
		}
	})
	t.Run("word beside a letter drill", func(t *testing.T) {
		svc, store, clk := openSvc(t, 1)
		ctx := context.Background()
		userID := mustUser(t, store)
		id := addCard(t, store, userID, "בּוֹדֵד", fsrs.SkillRead, clk.Now().Add(-time.Hour), 0, int(fsrs.Good), clk.Now().Add(-48*time.Hour), false)
		rev := fsrs.New(store, clk)
		for i := 0; i < MinMissesForDrill; i++ {
			if err := rev.RecordMiss(ctx, userID, fsrs.MissLetterPair, "ב/כ", 0); err != nil {
				t.Fatal(err)
			}
		}
		sess := mustBuild(t, svc, userID)
		card, ok := sess.Next()
		if !ok || card.CardID != id {
			t.Fatalf("first = %+v ok=%v", card, ok)
		}
		revealAndRecord(t, sess, ctx, id, fsrs.Again)
		sawDrill := false
		for n := 0; n < DrillSteps+2; n++ {
			next, ok := sess.Next()
			if !ok {
				break
			}
			if next.CardID == id {
				t.Fatalf("Again on the only word was requeued as %+v", next)
			}
			if next.Kind == KindLetter {
				sawDrill = true
			}
		}
		if !sawDrill {
			t.Fatal("the letter drill was not in the queue")
		}
	})
}

func TestDrillStillRequeuesInAOneLexemePool(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	id := addCard(t, store, userID, "קֶדֶם", fsrs.SkillRead, clk.Now().Add(-time.Hour), 0, int(fsrs.Good), clk.Now().Add(-48*time.Hour), false)
	rev := fsrs.New(store, clk)
	for i := 0; i < MinMissesForDrill; i++ {
		if err := rev.RecordMiss(ctx, userID, fsrs.MissLetterPair, "ב/כ", 0); err != nil {
			t.Fatal(err)
		}
	}
	sess := mustBuild(t, svc, userID)
	card, ok := sess.Next()
	if !ok || card.CardID != id {
		t.Fatalf("first = %+v ok=%v", card, ok)
	}
	revealAndRecord(t, sess, ctx, id, fsrs.Good)
	drill, ok := sess.Next()
	if !ok || drill.Kind != KindLetter || drill.CardID >= 0 {
		t.Fatalf("drill = %+v ok=%v", drill, ok)
	}
	revealAndRecord(t, sess, ctx, drill.CardID, fsrs.Again)
	found := false
	for n := 0; n < DrillSteps+2; n++ {
		next, ok := sess.Next()
		if !ok {
			break
		}
		if next.CardID == drill.CardID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("a missed drill in a one-lexeme pool was not queued again")
	}
}

// A letter step sits between two words. Skipping that step has to keep
// scanning, or the second word is invisible and the miss is not put back.
func TestMissRequeuesWhenADrillSitsBetweenTwoWords(t *testing.T) {
	svc, store, clk := openSvc(t, 1)
	ctx := context.Background()
	userID := mustUser(t, store)
	reviewed := clk.Now().Add(-48 * time.Hour)
	first := addCard(t, store, userID, "רִאשׁוֹן", fsrs.SkillRead, clk.Now().Add(-2*time.Hour), 0, int(fsrs.Good), reviewed, false)
	addCard(t, store, userID, "שֵׁנִי", fsrs.SkillRead, clk.Now().Add(-time.Hour), 0, int(fsrs.Good), reviewed, false)
	rev := fsrs.New(store, clk)
	for i := 0; i < MinMissesForDrill; i++ {
		if err := rev.RecordMiss(ctx, userID, fsrs.MissLetterPair, "ב/כ", 0); err != nil {
			t.Fatal(err)
		}
	}
	sess := mustBuild(t, svc, userID)
	card, ok := sess.Next()
	if !ok || card.CardID != first {
		t.Fatalf("first = %+v ok=%v, want %d", card, ok, first)
	}
	revealAndRecord(t, sess, ctx, first, fsrs.Again)
	for n := 0; n < DrillSteps+4; n++ {
		next, ok := sess.Next()
		if !ok {
			t.Fatal("the miss was not put back")
		}
		if next.CardID == first {
			return
		}
	}
	t.Fatal("the miss was not put back")
}

type againMeasure struct {
	between     int
	appearances int
}

func measureAgain(t *testing.T, sess *Session) againMeasure {
	t.Helper()
	ctx := context.Background()
	first, ok := sess.Next()
	if !ok {
		t.Fatal("empty queue")
	}
	revealAndRecord(t, sess, ctx, first.CardID, fsrs.Again)
	got := againMeasure{appearances: 1}
	for n := 0; n < SessionCap+5; n++ {
		card, ok := sess.Next()
		if !ok {
			return got
		}
		if card.CardID != first.CardID {
			if got.appearances == 1 {
				got.between++
			}
			continue
		}
		got.appearances++
		if got.appearances > 2 {
			t.Fatalf("card %d appeared %d times", first.CardID, got.appearances)
		}
	}
	t.Fatal("queue did not end")
	return got
}

func studiedIDs(cards []CardRef) []int64 {
	var out []int64
	for _, card := range cards {
		if card.New || card.Kind == KindLetter || card.Kind == KindPattern {
			continue
		}
		out = append(out, card.CardID)
	}
	return out
}

func sessionLexemeCount(cards []CardRef) int {
	seen := map[int64]struct{}{}
	for _, card := range cards {
		if card.LexemeID == 0 || card.Kind == KindLetter || card.Kind == KindPattern {
			continue
		}
		seen[card.LexemeID] = struct{}{}
	}
	return len(seen)
}

func adjacentLexemeAt(cards []CardRef) (int, int64) {
	for i := 1; i < len(cards); i++ {
		a, b := cards[i-1], cards[i]
		if a.LexemeID == 0 || b.LexemeID == 0 {
			continue
		}
		if a.Kind == KindLetter || a.Kind == KindPattern || b.Kind == KindLetter || b.Kind == KindPattern {
			continue
		}
		if a.LexemeID == b.LexemeID {
			return i, a.LexemeID
		}
	}
	return -1, 0
}

func newCardsAreSuffix(cards []CardRef) bool {
	seenNew := false
	for _, card := range cards {
		if card.New {
			seenNew = true
			continue
		}
		if seenNew {
			return false
		}
	}
	return seenNew
}

func lexemeOfCards(cards []CardRef, id int64) int64 {
	for _, card := range cards {
		if card.CardID == id {
			return card.LexemeID
		}
	}
	return 0
}

func indexOf(cards []CardRef, id int64) int {
	for i, card := range cards {
		if card.CardID == id {
			return i
		}
	}
	return -1
}

func describeQueue(cards []CardRef) string {
	parts := make([]string, len(cards))
	for i, card := range cards {
		mark := "K"
		if card.Struggling {
			mark = "S"
		}
		if card.New {
			mark = "N"
		}
		parts[i] = fmt.Sprintf("%d:%s/%d", card.CardID, mark, card.LexemeID)
	}
	return strings.Join(parts, " ")
}
