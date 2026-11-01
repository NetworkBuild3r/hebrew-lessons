package evidence

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

func denverZone(t *testing.T) studyday.Zone {
	t.Helper()
	loc, err := studyday.ParseZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	return studyday.Zone{Name: "America/Denver", Loc: loc, Source: studyday.SourceLearner}
}

func taughtEnv(t *testing.T, now time.Time) *env {
	t.Helper()
	clk := &fakeClock{t: now}
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	user, err := store.InsertUser(context.Background(), "a@example.com", "sub-a", "learner")
	if err != nil {
		t.Fatal(err)
	}
	return &env{t: t, store: store, rec: New(store, nil, clk), clk: clk, user: user}
}

func (e *env) setAudio(lexemeID int64, text string) {
	e.t.Helper()
	ctx := context.Background()
	clip := strings.Repeat("a", 64)
	ok, err := e.store.InsertAudio(ctx, db.Asset{
		ID: clip, TextNFC: text, Voice: "v", Model: "m", Bytes: 4, CreatedAt: e.clk.t,
	})
	if err != nil || !ok {
		e.t.Fatalf("InsertAudio: %v %v", ok, err)
	}
	if _, err := e.store.ExecContext(ctx, `UPDATE lexemes SET audio_id = ? WHERE id = ?`, clip, lexemeID); err != nil {
		e.t.Fatal(err)
	}
}

func skillsOf(got []string) string {
	return strings.Join(got, ",")
}

// ac-1: Taught for three words with no cards, Denver, on = 2026-11-01.
func TestTaughtCreatesNewCards(t *testing.T) {
	now := time.Date(2026, 11, 1, 17, 0, 0, 0, time.UTC)
	e := taughtEnv(t, now)
	z := denverZone(t)
	a := e.lexeme(kelev)
	b := e.lexeme(kala)
	c := e.lexeme(shalom)
	e.setAudio(c, shalom)

	wantDue := time.Date(2026, 11, 2, 11, 0, 0, 0, time.UTC)
	res, err := e.rec.Taught(context.Background(), e.user, []int64{a, b, c}, "2026-11-01", z)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Words) != 3 {
		t.Fatalf("words = %d, want 3", len(res.Words))
	}
	for _, w := range res.Words {
		if w.Already || !w.Due.Equal(wantDue) {
			t.Fatalf("word %#v, want due %v", w, wantDue)
		}
		switch w.LexemeID {
		case a, b:
			if skillsOf(w.Created) != "read,recall" || len(w.HadCard) != 0 {
				t.Fatalf("no-audio word %#v", w)
			}
		case c:
			if skillsOf(w.Created) != "read,recall,listen" || len(w.HadCard) != 0 {
				t.Fatalf("audio word %#v", w)
			}
		}
	}

	for _, id := range []int64{a, b} {
		for _, skill := range []string{"read", "recall"} {
			card, ok := e.card(id, skill)
			if !ok || card.State != "new" || !card.Due.Equal(wantDue) || card.Reps != 0 {
				t.Fatalf("%d/%s = %#v ok=%v", id, skill, card, ok)
			}
		}
		if _, ok := e.card(id, "listen"); ok {
			t.Fatalf("%d got a listen card without audio", id)
		}
	}
	for _, skill := range []string{"read", "recall", "listen"} {
		card, ok := e.card(c, skill)
		if !ok || card.State != "new" || !card.Due.Equal(wantDue) {
			t.Fatalf("audio %s = %#v ok=%v", skill, card, ok)
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM attempts`); n != 0 {
		t.Fatalf("attempts = %d", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM review_log`); n != 0 {
		t.Fatalf("review_log = %d", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM taught_words WHERE learner_id = ?`, e.user); n != 3 {
		t.Fatalf("taught_words = %d", n)
	}
}

// ac-2: existing read card in review is left byte-for-byte; hadCard reports read.
func TestTaughtLeavesExistingCardUnchanged(t *testing.T) {
	now := time.Date(2026, 11, 1, 17, 0, 0, 0, time.UTC)
	e := taughtEnv(t, now)
	z := denverZone(t)
	lex := e.lexeme(kelev)
	ctx := context.Background()
	due := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	cardID, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex, Skill: "read", State: "review",
		Due: due, Stability: 4.5, Difficulty: 5.2, Reps: 3, Lapses: 1,
		LastReview: time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := e.store.GetCard(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}

	res, err := e.rec.Taught(ctx, e.user, []int64{lex}, "2026-11-01", z)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Words) != 1 || skillsOf(res.Words[0].HadCard) != "read" {
		t.Fatalf("result = %#v", res.Words)
	}
	if skillsOf(res.Words[0].Created) != "recall" {
		t.Fatalf("created = %v, want recall only", res.Words[0].Created)
	}
	after, err := e.store.GetCard(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("card changed\n before: %#v\n after: %#v", before, after)
	}
}

// ac-3: bad on / too many ids / zero id write nothing.
func TestTaughtRejectsBadOnAndIds(t *testing.T) {
	now := time.Date(2026, 11, 1, 17, 0, 0, 0, time.UTC)
	e := taughtEnv(t, now)
	z := denverZone(t)
	lex := e.lexeme(kelev)
	ctx := context.Background()

	cases := []struct {
		name string
		on   string
		ids  []int64
	}{
		{"next day", "2026-11-02", []int64{lex}},
		{"31 days back", "2026-10-01", []int64{lex}},
		{"zero id", "2026-11-01", []int64{0}},
		{"51 ids", "2026-11-01", make([]int64, 51)},
	}
	cases[3].ids[0] = lex
	for i := 1; i < 51; i++ {
		cases[3].ids[i] = lex // duplicates still 51 entries
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.rec.Taught(ctx, e.user, tc.ids, tc.on, z)
			if err == nil {
				t.Fatal("expected error")
			}
			if n := e.count(`SELECT COUNT(*) FROM taught_words`); n != 0 {
				t.Fatalf("taught_words = %d after %s", n, tc.name)
			}
			if n := e.count(`SELECT COUNT(*) FROM cards`); n != 0 {
				t.Fatalf("cards = %d after %s", n, tc.name)
			}
		})
	}
}

// ac-4: RemoveTaught keeps a reviewed recall card and removes the rest.
func TestRemoveTaughtKeepsReviewed(t *testing.T) {
	now := time.Date(2026, 11, 1, 17, 0, 0, 0, time.UTC)
	e := taughtEnv(t, now)
	z := denverZone(t)
	lex := e.lexeme(kelev)
	ctx := context.Background()
	if _, err := e.rec.Taught(ctx, e.user, []int64{lex}, "2026-11-01", z); err != nil {
		t.Fatal(err)
	}
	recall, ok := e.card(lex, "recall")
	if !ok {
		t.Fatal("no recall card")
	}
	recall.State = "review"
	recall.Reps = 1
	recall.Stability = 1.2
	recall.Difficulty = 5
	recall.LastReview = now
	recall.Due = now.Add(24 * time.Hour)
	if err := e.store.ApplyReview(ctx, recall, db.ReviewLog{
		CardID: recall.ID, Rating: 3, ReviewedAt: now, ScheduledDays: 1,
	}); err != nil {
		t.Fatal(err)
	}

	res, err := e.rec.RemoveTaught(ctx, e.user, "2026-11-01", []int64{lex})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Words) != 1 {
		t.Fatalf("words = %#v", res.Words)
	}
	w := res.Words[0]
	if skillsOf(w.Kept) != "recall" {
		t.Fatalf("kept = %v", w.Kept)
	}
	if skillsOf(w.Removed) != "read" {
		t.Fatalf("removed = %v", w.Removed)
	}
	if _, ok := e.card(lex, "read"); ok {
		t.Fatal("read card should be gone")
	}
	if _, ok := e.card(lex, "recall"); !ok {
		t.Fatal("recall card should remain")
	}
	if n := e.count(`SELECT COUNT(*) FROM taught_words WHERE learner_id = ?`, e.user); n != 0 {
		t.Fatalf("taught_words left = %d", n)
	}
}

// ac-5: RemoveTaught never deletes a card the taught row did not create.
func TestRemoveTaughtSkipsForeignCards(t *testing.T) {
	now := time.Date(2026, 11, 1, 17, 0, 0, 0, time.UTC)
	e := taughtEnv(t, now)
	z := denverZone(t)
	lex := e.lexeme(kelev)
	ctx := context.Background()
	// Lesson-style listen card created before Taught (Taught will only create read+recall).
	listenID, err := e.store.InsertCard(ctx, db.Card{
		UserID: e.user, LexemeID: lex, Skill: "listen", State: "new", Due: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.rec.Taught(ctx, e.user, []int64{lex}, "2026-11-01", z); err != nil {
		t.Fatal(err)
	}
	res, err := e.rec.RemoveTaught(ctx, e.user, "2026-11-01", []int64{lex})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Words) != 1 || len(res.Words[0].Removed) != 2 {
		t.Fatalf("remove = %#v", res.Words)
	}
	got, err := e.store.GetCard(ctx, listenID)
	if err != nil {
		t.Fatalf("lesson listen card was deleted: %v", err)
	}
	if got.ID != listenID || got.Skill != "listen" {
		t.Fatalf("listen card = %#v", got)
	}
}

// ac-7: learner A's Taught / RemoveTaught never touch learner B.
func TestTaughtLearnerIsolation(t *testing.T) {
	now := time.Date(2026, 11, 1, 17, 0, 0, 0, time.UTC)
	e := taughtEnv(t, now)
	z := denverZone(t)
	ctx := context.Background()
	b, err := e.store.InsertUser(ctx, "b@example.com", "sub-b", "learner")
	if err != nil {
		t.Fatal(err)
	}
	lex := e.lexeme(kelev)
	if _, err := e.store.InsertCard(ctx, db.Card{
		UserID: b, LexemeID: lex, Skill: "read", State: "new", Due: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.rec.Taught(ctx, e.user, []int64{lex}, "2026-11-01", z); err != nil {
		t.Fatal(err)
	}
	if n := e.count(`SELECT COUNT(*) FROM cards WHERE user_id = ?`, b); n != 1 {
		t.Fatalf("B cards after Taught = %d", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM taught_words WHERE learner_id = ?`, b); n != 0 {
		t.Fatalf("B taught_words = %d", n)
	}
	bRead, ok, err := e.store.CardID(ctx, b, lex, "read")
	if err != nil || !ok {
		t.Fatalf("B card id: %v %v", bRead, err)
	}
	before, err := e.store.GetCard(ctx, bRead)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.rec.RemoveTaught(ctx, e.user, "2026-11-01", []int64{lex}); err != nil {
		t.Fatal(err)
	}
	after, err := e.store.GetCard(ctx, bRead)
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("B card changed\n before %#v\n after %#v", before, after)
	}
	if n := e.count(`SELECT COUNT(*) FROM cards WHERE user_id = ?`, e.user); n != 0 {
		t.Fatalf("A cards after remove = %d", n)
	}
}
