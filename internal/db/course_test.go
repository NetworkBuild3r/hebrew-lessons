package db

import (
	"context"
	"testing"
	"time"
)

func TestSyncCourseAddsOnceAndKeepsStoredWords(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	if _, err := s.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, bare, gloss_en, translit) VALUES ('בַּיִת', 'בית', 'home', 'bayit')`); err != nil {
		t.Fatal(err)
	}
	rows := []CourseRow{
		{Order: 1, Rank: 1, Lemma: "בית", Pointed: "בַּיִת", Translit: "bayit", Gloss: "house", POS: "noun", Gender: "m"},
		{Order: 2, Rank: 2, Lemma: "אם", Pointed: "אִם", Translit: "im", Gloss: "if", POS: "conjunction"},
		{Order: 3, Rank: 3, Lemma: "עם", Pointed: "עִם", Translit: "im", Gloss: "with", POS: "preposition"},
	}
	got, err := s.SyncCourse(ctx, rows)
	if err != nil {
		t.Fatal(err)
	}
	if got.Added != 2 || got.Linked != 1 || got.Skipped != 0 {
		t.Fatalf("sync = %+v, want added 2 linked 1", got)
	}
	conn, err := s.DB()
	if err != nil {
		t.Fatal(err)
	}
	var gloss string
	if err := conn.QueryRow(`SELECT gloss_en FROM lexemes WHERE text_nfc = 'בַּיִת'`).Scan(&gloss); err != nil {
		t.Fatal(err)
	}
	if gloss != "home" {
		t.Fatalf("stored gloss = %q, want home", gloss)
	}
	again, err := s.SyncCourse(ctx, rows)
	if err != nil {
		t.Fatal(err)
	}
	if again.Added != 0 || again.Linked != 0 || again.Moved != 0 {
		t.Fatalf("second sync = %+v, want zeros", again)
	}
}

func TestSyncCourseSkipsAmbiguousSpelling(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	if _, err := s.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, bare, gloss_en) VALUES ('אָב', 'אב', 'a')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, bare, gloss_en) VALUES ('אַב', 'אב', 'b')`); err != nil {
		t.Fatal(err)
	}
	got, err := s.SyncCourse(ctx, []CourseRow{{
		Order: 1, Rank: 1, Lemma: "אב", Pointed: "אָב", Translit: "av", Gloss: "father", POS: "noun", Gender: "m",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Skipped != 1 || got.Added != 0 || got.Linked != 0 {
		t.Fatalf("sync = %+v, want skipped 1", got)
	}
}

func TestSyncCourseMovesOrder(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	row := CourseRow{Order: 1, Rank: 4, Lemma: "אם", Pointed: "אִם", Translit: "im", Gloss: "if", POS: "conjunction"}
	if _, err := s.SyncCourse(ctx, []CourseRow{row}); err != nil {
		t.Fatal(err)
	}
	row.Order = 9
	got, err := s.SyncCourse(ctx, []CourseRow{row})
	if err != nil {
		t.Fatal(err)
	}
	if got.Moved != 1 || got.Added != 0 {
		t.Fatalf("sync = %+v, want moved 1", got)
	}
	conn, err := s.DB()
	if err != nil {
		t.Fatal(err)
	}
	var order int
	if err := conn.QueryRow(`SELECT course_order FROM course_words`).Scan(&order); err != nil {
		t.Fatal(err)
	}
	if order != 9 {
		t.Fatalf("order = %d, want 9", order)
	}
}

func TestCourseCardLexemes(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	user, err := s.InsertUser(ctx, "cards@example.com", "sub-cards", "learner")
	if err != nil {
		t.Fatal(err)
	}
	rows := []CourseRow{
		{Order: 1, Rank: 1, Lemma: "אם", Pointed: "אִם", Translit: "im", Gloss: "if", POS: "conjunction"},
		{Order: 2, Rank: 2, Lemma: "עם", Pointed: "עִם", Translit: "im", Gloss: "with", POS: "preposition"},
		{Order: 3, Rank: 3, Lemma: "לא", Pointed: "לֹא", Translit: "lo", Gloss: "not", POS: "adverb"},
	}
	if _, err := s.SyncCourse(ctx, rows); err != nil {
		t.Fatal(err)
	}
	course, err := s.CourseLexemes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range course[:2] {
		if _, err := s.InsertCard(ctx, Card{
			UserID: user, LexemeID: row.LexemeID, Skill: "read", State: "new",
			Due: time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.CourseCardLexemes(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || !got[course[0].LexemeID] || !got[course[1].LexemeID] || got[course[2].LexemeID] {
		t.Fatalf("cards = %v", got)
	}
}

func TestUnfinishedIntroHidesANewWord(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	user, err := s.InsertUser(ctx, "learner@example.com", "sub", "learner")
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES ('אִם', 'if')`)
	if err != nil {
		t.Fatal(err)
	}
	lexeme, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	res, err = s.ExecContext(ctx, `
		INSERT INTO units (position, title, situation, concept, status, rev, doc)
		VALUES (1, 't', 's', 'c', 'published', 1, '{"title":"t"}')`)
	if err != nil {
		t.Fatal(err)
	}
	unit, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExecContext(ctx, `
		INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill)
		VALUES (?, 0, ?, 'new', 'recall')`, unit, lexeme); err != nil {
		t.Fatal(err)
	}
	got, err := s.LexemesBehindUnfinishedIntro(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	if !got[lexeme] {
		t.Fatal("unfinished intro was not reported")
	}
}
