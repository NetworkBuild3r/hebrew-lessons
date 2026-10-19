package db

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestActivityPool(t *testing.T) {
	when := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "a@example.com", "sub-a", "learner")
	other := insertUser(t, s, "b@example.com", "sub-b", "learner")

	clip := "a3f1c0d2e4b6a8c0d2e4f6a8b0c2d4e6f8a0b2c4d6e8f0a2b4c6d8e0f2a4b6c8"
	if _, err := s.InsertAudio(ctx, Asset{ID: clip, TextNFC: "בית", Voice: "v", Model: "m", CreatedAt: when}); err != nil {
		t.Fatal(err)
	}
	house := insertLexeme(t, s, "בית")
	tree := insertLexeme(t, s, "עץ")
	dog := insertLexeme(t, s, "כלב")
	if _, err := s.ExecContext(ctx, `UPDATE lexemes SET gloss_en = 'house', pos = 'noun', audio_id = ? WHERE id = ?`, clip, house); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExecContext(ctx, `UPDATE lexemes SET gloss_en = 'tree', pos = 'noun' WHERE id = ?`, tree); err != nil {
		t.Fatal(err)
	}
	// The learner has a card for the tree and an attempt at the dog. The other learner has neither.
	if _, err := s.InsertCard(ctx, Card{UserID: user, LexemeID: tree, Skill: "read", State: "review", Due: when, Stability: 1, Difficulty: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.WriteAttempt(ctx, AttemptWrite{Attempt: Attempt{
		UserID: user, LexemeID: dog, Skill: "listen", Activity: "listen_pick", Source: "practice", Prompt: "p", Response: "r", CreatedAt: when,
	}}); err != nil {
		t.Fatal(err)
	}

	got, err := s.ActivityPool(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	want := []PoolWord{
		{ID: house, TextNFC: "בית", Gloss: "house", POS: "noun", AudioID: clip, Seen: false},
		{ID: tree, TextNFC: "עץ", Gloss: "tree", POS: "noun", Seen: true},
		{ID: dog, TextNFC: "כלב", Seen: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pool = %+v\nwant   %+v", got, want)
	}
	got, err = s.ActivityPool(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range got {
		if w.Seen {
			t.Fatalf("%+v is seen by a learner with no evidence", w)
		}
	}
}

func TestWordConfusions(t *testing.T) {
	when := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "a@example.com", "sub-a", "learner")
	other := insertUser(t, s, "b@example.com", "sub-b", "learner")
	a, b := insertLexeme(t, s, "אאא"), insertLexeme(t, s, "בבב")

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.InsertConfusionMiss(ctx, user, "other", "אאא/בבב", a, when.Add(-48*time.Hour)))
	must(s.InsertConfusionMiss(ctx, user, "letter_pair", "ב/כ", a, when.Add(-24*time.Hour)))
	must(s.InsertConfusionMiss(ctx, user, "other", "אאא/בבב", b, when.Add(-time.Hour)))       // another word
	must(s.InsertConfusionMiss(ctx, user, "other", "אאא/בבב", a, when.Add(-40*24*time.Hour))) // too old
	must(s.InsertConfusionMiss(ctx, other, "other", "אאא/בבב", a, when.Add(-time.Hour)))      // another learner
	must(s.InsertConfusionMiss(ctx, user, "agreement", "הוא/היא", 0, when.Add(-time.Hour)))   // tied to no word

	got, err := s.WordConfusions(ctx, user, a, when.Add(-30*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	want := []WordConfusion{
		{Kind: "letter_pair", Key: "ב/כ", At: when.Add(-24 * time.Hour)},
		{Kind: "other", Key: "אאא/בבב", At: when.Add(-48 * time.Hour)},
	}
	if len(got) != len(want) {
		t.Fatalf("confusions = %+v, want %+v (newest first)", got, want)
	}
	for i := range want {
		if got[i].Kind != want[i].Kind || got[i].Key != want[i].Key || !got[i].At.Equal(want[i].At) {
			t.Fatalf("confusions[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// WriteAttempt writes the single confusion and the extra ones with the attempt, or none of them.
func TestWriteAttemptExtraConfusions(t *testing.T) {
	when := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "a@example.com", "sub-a", "learner")
	lex := insertLexeme(t, s, "אאא")
	attempt := Attempt{UserID: user, LexemeID: lex, Skill: "spell", Activity: "dictation", Source: "practice", Prompt: "p", Response: "r", CreatedAt: when}

	if _, err := s.WriteAttempt(ctx, AttemptWrite{
		Attempt:   attempt,
		Confusion: &Confusion{Kind: "other", Key: "x/y"},
		Extra:     []Confusion{{Kind: "letter_pair", Key: "ם/ן"}, {Kind: "letter_pair", Key: "ח/כ"}},
	}); err != nil {
		t.Fatal(err)
	}
	var n int
	conn, _ := s.DB()
	if err := conn.QueryRow(`SELECT COUNT(*) FROM confusion_misses WHERE learner_id = ? AND lexeme_id = ?`, user, lex).Scan(&n); err != nil || n != 3 {
		t.Fatalf("%d confusion rows, %v; want 3", n, err)
	}

	// A bad row rolls the attempt back with the rest.
	if _, err := s.WriteAttempt(ctx, AttemptWrite{
		Attempt: attempt,
		Extra:   []Confusion{{Kind: "letter_pair", Key: "ט/ת"}, {Kind: "not-a-kind", Key: "z"}},
	}); err == nil {
		t.Fatal("a confusion of an unknown kind was accepted")
	}
	if err := conn.QueryRow(`SELECT COUNT(*) FROM attempts`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("%d attempts, %v; want 1: the failed write must leave nothing", n, err)
	}
	if err := conn.QueryRow(`SELECT COUNT(*) FROM confusion_misses`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("%d confusion rows, %v; want 3", n, err)
	}
}

func TestListStudyCardsIncludesEverySkill(t *testing.T) {
	when := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "a@example.com", "sub-a", "learner")
	for i, skill := range []string{"read", "recall", "listen", "spell"} {
		lex := insertLexeme(t, s, "מילה"+string(rune('א'+i)))
		if _, err := s.InsertCard(ctx, Card{UserID: user, LexemeID: lex, Skill: skill, State: "review", Due: when, Stability: 1, Difficulty: 5}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.ListStudyCards(ctx, user, when, when.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	skills := map[string]bool{}
	for _, c := range got {
		skills[c.Skill] = true
	}
	if len(got) != 4 || !skills["listen"] || !skills["spell"] || !skills["read"] || !skills["recall"] {
		t.Fatalf("study cards = %+v", got)
	}
}
