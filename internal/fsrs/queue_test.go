package fsrs

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/dbtest"
)

func TestAssembleQueue(t *testing.T) {
	if MaxQueue != 20 {
		t.Fatalf("MaxQueue = %d, want 20", MaxQueue)
	}
	t.Run("cap", func(t *testing.T) {
		cards := make([]Item, 25)
		for i := range cards {
			cards[i] = Item{Kind: ItemCard, CardID: int64(i + 1), LexemeID: int64(i + 1)}
		}
		got := assembleQueue(cards, nil, nil)
		if len(got) != MaxQueue {
			t.Fatalf("len = %d, want %d", len(got), MaxQueue)
		}
		if got[0].CardID != 1 || got[19].CardID != 20 {
			t.Fatalf("kept cards %d..%d, want 1..20", got[0].CardID, got[19].CardID)
		}
	})
	t.Run("separate same lexeme", func(t *testing.T) {
		got := assembleQueue([]Item{
			{Kind: ItemCard, CardID: 1, LexemeID: 5},
			{Kind: ItemCard, CardID: 2, LexemeID: 5},
			{Kind: ItemCard, CardID: 3, LexemeID: 6},
		}, nil, nil)
		want := []int64{1, 3, 2}
		if len(got) != len(want) {
			t.Fatalf("len = %d, want %d", len(got), len(want))
		}
		for i, id := range want {
			if got[i].CardID != id {
				t.Fatalf("item %d card = %d, want %d", i, got[i].CardID, id)
			}
		}
		if adjacentLexeme(got) {
			t.Fatalf("adjacent lexemes: %+v", got)
		}
	})
	t.Run("drop same lexeme when nothing separates them", func(t *testing.T) {
		got := assembleQueue([]Item{
			{Kind: ItemCard, CardID: 1, LexemeID: 5},
			{Kind: ItemCard, CardID: 2, LexemeID: 5},
		}, nil, nil)
		if len(got) != 1 || got[0].CardID != 1 {
			t.Fatalf("queue = %+v, want only the first card", got)
		}
	})
	t.Run("alternate three types", func(t *testing.T) {
		cards := []Item{
			{Kind: ItemCard, CardID: 1, LexemeID: 1},
			{Kind: ItemCard, CardID: 2, LexemeID: 2},
		}
		var letters, patterns []Item
		for i := 0; i < DrillItems; i++ {
			letters = append(letters, Item{Kind: ItemLetter, Key: "ב/כ", Index: i})
			patterns = append(patterns, Item{Kind: ItemPattern, Key: "הוא/היא", Index: i})
		}
		got := assembleQueue(cards, letters, patterns)
		want := []Kind{ItemCard, ItemLetter, ItemPattern, ItemCard, ItemLetter, ItemPattern, ItemLetter, ItemPattern, ItemLetter, ItemPattern}
		if len(got) != len(want) {
			t.Fatalf("len = %d, want %d (%+v)", len(got), len(want), kindsOf(got))
		}
		for i, kind := range want {
			if got[i].Kind != kind {
				t.Fatalf("item %d kind = %s, want %s (%v)", i, got[i].Kind, kind, kindsOf(got))
			}
		}
		if adjacentLexeme(got) {
			t.Fatalf("adjacent lexemes: %+v", got)
		}
	})
	t.Run("drill items stay inside a full card list", func(t *testing.T) {
		cards := make([]Item, MaxQueue)
		for i := range cards {
			cards[i] = Item{Kind: ItemCard, CardID: int64(i + 1), LexemeID: int64(i + 1)}
		}
		letters := make([]Item, DrillItems)
		for i := range letters {
			letters[i] = Item{Kind: ItemLetter, Key: "ב/כ", Index: i}
		}
		got := assembleQueue(cards, letters, nil)
		if len(got) != MaxQueue {
			t.Fatalf("len = %d, want %d", len(got), MaxQueue)
		}
		if n := countKind(got, ItemLetter); n != DrillItems {
			t.Fatalf("letter items = %d, want %d", n, DrillItems)
		}
	})
}

func TestQueueCapAdjacencyAndRequeue(t *testing.T) {
	rev, store, clk := openReview(t)
	ctx := context.Background()
	userID := mustUser(t, store, "learner@example.com")
	var ids []int64
	for i := 0; i < 25; i++ {
		ids = append(ids, mustCard(t, store, db.Card{
			UserID:   userID,
			LexemeID: mustLexeme(t, store, "word-"+strconv.Itoa(i)),
			Dir:      DirHeEn,
			Due:      clk.Now().Add(-time.Duration(25-i) * time.Minute),
		}))
	}
	sess, err := rev.Queue(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sess.Items) != MaxQueue {
		t.Fatalf("len = %d, want %d", len(sess.Items), MaxQueue)
	}
	for i, item := range sess.Items {
		if item.Kind != ItemCard || item.CardID != ids[i] {
			t.Fatalf("item %d = %+v, want card %d", i, item, ids[i])
		}
	}
	if adjacentLexeme(sess.Items) {
		t.Fatal("adjacent lexemes in the capped queue")
	}
	first := sess.Items[0]
	if !sess.RequeueMiss(first) {
		t.Fatal("first miss was not requeued")
	}
	if len(sess.Items) != MaxQueue+1 {
		t.Fatalf("len after miss = %d, want %d", len(sess.Items), MaxQueue+1)
	}
	last := sess.Items[len(sess.Items)-1]
	if last.CardID != first.CardID {
		t.Fatalf("requeued card = %d, want %d", last.CardID, first.CardID)
	}
	if sess.RequeueMiss(first) {
		t.Fatal("the same miss was requeued twice")
	}
	if len(sess.Items) != MaxQueue+1 {
		t.Fatalf("len after second miss = %d, want %d", len(sess.Items), MaxQueue+1)
	}
	if !sess.RequeueMiss(sess.Items[1]) {
		t.Fatal("a different miss was not requeued")
	}
	if len(sess.Items) != MaxQueue+2 {
		t.Fatalf("len after a second item = %d, want %d", len(sess.Items), MaxQueue+2)
	}
}

func TestQueueSeparatesSameLexeme(t *testing.T) {
	rev, store, clk := openReview(t)
	ctx := context.Background()
	userID := mustUser(t, store, "learner@example.com")
	lexA := mustLexeme(t, store, "כלב")
	lexB := mustLexeme(t, store, "חתול")
	due := clk.Now().Add(-time.Hour)
	first := mustCard(t, store, db.Card{UserID: userID, LexemeID: lexA, Dir: DirHeEn, Due: due})
	second := mustCard(t, store, db.Card{UserID: userID, LexemeID: lexA, Dir: DirEnHe, Due: due})
	third := mustCard(t, store, db.Card{UserID: userID, LexemeID: lexB, Dir: DirHeEn, Due: due})
	sess, err := rev.Queue(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{first, third, second}
	if len(sess.Items) != len(want) {
		t.Fatalf("len = %d, want %d (%+v)", len(sess.Items), len(want), sess.Items)
	}
	for i, id := range want {
		if sess.Items[i].CardID != id {
			t.Fatalf("item %d = %d, want %d", i, sess.Items[i].CardID, id)
		}
	}
	if adjacentLexeme(sess.Items) {
		t.Fatalf("adjacent lexemes: %+v", sess.Items)
	}
}

func TestQueueAlternatesDrills(t *testing.T) {
	rev, store, clk := openReview(t)
	ctx := context.Background()
	userID := mustUser(t, store, "learner@example.com")
	for i, text := range []string{"אחד", "שניים"} {
		mustCard(t, store, db.Card{
			UserID:   userID,
			LexemeID: mustLexeme(t, store, text),
			Dir:      DirHeEn,
			Due:      clk.Now().Add(-time.Duration(2-i) * time.Hour),
		})
	}
	if err := rev.RecordMiss(ctx, userID, MissLetterPair, "ב/כ"); err != nil {
		t.Fatal(err)
	}
	if err := rev.RecordMiss(ctx, userID, MissLetterPair, "ב/כ"); err != nil {
		t.Fatal(err)
	}
	if err := rev.RecordMiss(ctx, userID, MissAgreement, "הוא/היא"); err != nil {
		t.Fatal(err)
	}
	if err := rev.RecordMiss(ctx, userID, MissAgreement, "הוא/היא"); err != nil {
		t.Fatal(err)
	}
	sess, err := rev.Queue(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	want := []Kind{ItemCard, ItemLetter, ItemPattern, ItemCard, ItemLetter, ItemPattern, ItemLetter, ItemPattern, ItemLetter, ItemPattern}
	if len(sess.Items) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(sess.Items), len(want), kindsOf(sess.Items))
	}
	for i, kind := range want {
		if sess.Items[i].Kind != kind {
			t.Fatalf("item %d kind = %s, want %s (%v)", i, sess.Items[i].Kind, kind, kindsOf(sess.Items))
		}
	}
	if adjacentLexeme(sess.Items) {
		t.Fatal("adjacent lexemes")
	}
	if countKind(sess.Items, ItemLetter) != DrillItems || countKind(sess.Items, ItemPattern) != DrillItems {
		t.Fatalf("drills were repeated: %+v", sess.Items)
	}
}

func TestQueueHoldsNewWordsUntilIntroFinishes(t *testing.T) {
	rev, store, clk := openReview(t)
	ctx := context.Background()
	userID := mustUser(t, store, "learner@example.com")
	otherID := mustUser(t, store, "other@example.com")
	newID := mustLexeme(t, store, "חדש")
	oldID := mustLexeme(t, store, "ישן")
	recycleID := mustLexeme(t, store, "חוזר")
	due := clk.Now().Add(-time.Minute)
	newCard := mustCard(t, store, db.Card{UserID: userID, LexemeID: newID, Dir: DirHeEn, Due: due})
	oldCard := mustCard(t, store, db.Card{UserID: userID, LexemeID: oldID, Dir: DirHeEn, Due: due})
	recycleCard := mustCard(t, store, db.Card{UserID: userID, LexemeID: recycleID, Dir: DirEnHe, Due: due})
	unitID, err := dbtest.InsertUnit(ctx, store, "Hello", "draft")
	if err != nil {
		t.Fatal(err)
	}
	if err := dbtest.InsertLessonItem(ctx, store, unitID, 0, newID, "new", "recall"); err != nil {
		t.Fatal(err)
	}
	if err := dbtest.InsertLessonItem(ctx, store, unitID, 0, recycleID, "recycle", "recall"); err != nil {
		t.Fatal(err)
	}

	assertCards(t, rev, userID, []int64{oldCard, recycleCard})

	started := clk.Now()
	if _, err := dbtest.InsertLessonRun(ctx, store, userID, unitID, 0, started, nil); err != nil {
		t.Fatal(err)
	}
	assertCards(t, rev, userID, []int64{oldCard, recycleCard})

	finished := clk.Now()
	if _, err := dbtest.InsertLessonRun(ctx, store, otherID, unitID, 0, started, &finished); err != nil {
		t.Fatal(err)
	}
	assertCards(t, rev, userID, []int64{oldCard, recycleCard})

	if _, err := dbtest.InsertLessonRun(ctx, store, userID, unitID, 0, started, &finished); err != nil {
		t.Fatal(err)
	}
	assertCards(t, rev, userID, []int64{newCard, oldCard, recycleCard})
}

func TestConfusionDrills(t *testing.T) {
	if ConfusionWindow != 7*24*time.Hour || MinMissesForDrill != 2 || DrillItems != 4 {
		t.Fatalf("window=%s min=%d items=%d", ConfusionWindow, MinMissesForDrill, DrillItems)
	}
	t.Run("one miss", func(t *testing.T) {
		rev, store, _ := openReview(t)
		userID := mustUser(t, store, "learner@example.com")
		if err := rev.RecordMiss(context.Background(), userID, MissLetterPair, "ב/כ"); err != nil {
			t.Fatal(err)
		}
		assertNoDrill(t, rev, userID)
	})
	t.Run("two misses seven days apart", func(t *testing.T) {
		rev, store, clk := openReview(t)
		ctx := context.Background()
		userID := mustUser(t, store, "learner@example.com")
		if err := rev.RecordMiss(ctx, userID, MissLetterPair, "ב/כ"); err != nil {
			t.Fatal(err)
		}
		if err := rev.RecordMiss(ctx, userID, MissLetterPair, "ג/ק"); err != nil {
			t.Fatal(err)
		}
		clk.Advance(7 * 24 * time.Hour)
		if err := rev.RecordMiss(ctx, userID, MissLetterPair, "ב/כ"); err != nil {
			t.Fatal(err)
		}
		sess := mustQueue(t, rev, userID)
		if indexes := drillIndexes(sess.Items, ItemLetter, "ב/כ"); !sameInts(indexes, []int{0, 1, 2, 3}) {
			t.Fatalf("ב/כ drill = %v, want [0 1 2 3]", indexes)
		}
		if n := len(drillIndexes(sess.Items, ItemLetter, "ג/ק")); n != 0 {
			t.Fatalf("one miss of ג/ק produced %d drill items", n)
		}
	})
	t.Run("two misses eight days apart", func(t *testing.T) {
		rev, store, clk := openReview(t)
		ctx := context.Background()
		userID := mustUser(t, store, "learner@example.com")
		if err := rev.RecordMiss(ctx, userID, MissLetterPair, "ב/כ"); err != nil {
			t.Fatal(err)
		}
		clk.Advance(8 * 24 * time.Hour)
		if err := rev.RecordMiss(ctx, userID, MissLetterPair, "ב/כ"); err != nil {
			t.Fatal(err)
		}
		assertNoDrill(t, rev, userID)
	})
	t.Run("five misses one drill", func(t *testing.T) {
		rev, store, _ := openReview(t)
		ctx := context.Background()
		userID := mustUser(t, store, "learner@example.com")
		for i := 0; i < 5; i++ {
			if err := rev.RecordMiss(ctx, userID, MissLetterPair, "ב/כ"); err != nil {
				t.Fatal(err)
			}
		}
		sess := mustQueue(t, rev, userID)
		if indexes := drillIndexes(sess.Items, ItemLetter, "ב/כ"); !sameInts(indexes, []int{0, 1, 2, 3}) {
			t.Fatalf("drill = %v, want one 4-item drill", indexes)
		}
	})
	t.Run("agreement is a pattern drill", func(t *testing.T) {
		rev, store, _ := openReview(t)
		ctx := context.Background()
		userID := mustUser(t, store, "learner@example.com")
		if err := rev.RecordMiss(ctx, userID, "root", "ב/כ"); err == nil {
			t.Fatal("bad kind was accepted")
		}
		if err := rev.RecordMiss(ctx, userID, MissAgreement, ""); err == nil {
			t.Fatal("empty key was accepted")
		}
		for i := 0; i < 2; i++ {
			if err := rev.RecordMiss(ctx, userID, MissAgreement, "הוא/היא"); err != nil {
				t.Fatal(err)
			}
		}
		sess := mustQueue(t, rev, userID)
		if indexes := drillIndexes(sess.Items, ItemPattern, "הוא/היא"); !sameInts(indexes, []int{0, 1, 2, 3}) {
			t.Fatalf("pattern drill = %v", indexes)
		}
		if countKind(sess.Items, ItemLetter) != 0 {
			t.Fatal("agreement miss became a letter drill")
		}
	})
}

func assertCards(t *testing.T, rev *Review, userID int64, want []int64) {
	t.Helper()
	sess := mustQueue(t, rev, userID)
	if len(sess.Items) != len(want) {
		t.Fatalf("len = %d, want %d (%+v)", len(sess.Items), len(want), sess.Items)
	}
	got := map[int64]bool{}
	for _, item := range sess.Items {
		got[item.CardID] = true
	}
	for _, id := range want {
		if !got[id] {
			t.Fatalf("missing card %d in %+v", id, sess.Items)
		}
	}
}

func assertNoDrill(t *testing.T, rev *Review, userID int64) {
	t.Helper()
	sess := mustQueue(t, rev, userID)
	if len(sess.Items) != 0 {
		t.Fatalf("queue = %+v, want no drill", sess.Items)
	}
}

func mustQueue(t *testing.T, rev *Review, userID int64) *Session {
	t.Helper()
	sess, err := rev.Queue(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	return sess
}

func countKind(items []Item, kind Kind) int {
	n := 0
	for _, item := range items {
		if item.Kind == kind {
			n++
		}
	}
	return n
}

func kindsOf(items []Item) []Kind {
	out := make([]Kind, len(items))
	for i, item := range items {
		out[i] = item.Kind
	}
	return out
}

func adjacentLexeme(items []Item) bool {
	for i := 1; i < len(items); i++ {
		if items[i].LexemeID != 0 && items[i].LexemeID == items[i-1].LexemeID {
			return true
		}
	}
	return false
}

func drillIndexes(items []Item, kind Kind, key string) []int {
	var out []int
	for _, item := range items {
		if item.Kind == kind && item.Key == key {
			out = append(out, item.Index)
		}
	}
	return out
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
