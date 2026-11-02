package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestCreateNewCardsInsertsAndIgnores(t *testing.T) {
	when := time.Date(2026, 11, 1, 17, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user, err := s.InsertUser(ctx, "newcards@example.com", "sub-newcards", "learner")
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES (?, 'dog')`, units.Normalize("כֶּלֶב"))
	if err != nil {
		t.Fatal(err)
	}
	lex, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	due := when
	out, err := s.CreateNewCards(ctx, user, []int64{lex}, due, func(hasClip bool) []string {
		skills := []string{"read", "recall"}
		if hasClip {
			skills = append(skills, "listen")
		}
		return skills
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Words) != 1 || strings.Join(out.Words[0].Created, ",") != "read,recall" {
		t.Fatalf("created = %#v", out)
	}
	again, err := s.CreateNewCards(ctx, user, []int64{lex}, due, func(bool) []string {
		return []string{"read", "recall"}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Words) != 1 || len(again.Words[0].Created) != 0 {
		t.Fatalf("repeat = %#v", again)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, user); n != 2 {
		t.Fatalf("cards = %d", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM taught_words`); n != 0 {
		t.Fatalf("taught_words = %d", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM review_log`); n != 0 {
		t.Fatalf("review_log = %d", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM attempts`); n != 0 {
		t.Fatalf("attempts = %d", n)
	}
}

func TestCreateNewCardsSharesInsertWithTaught(t *testing.T) {
	when := time.Date(2026, 11, 1, 17, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user, err := s.InsertUser(ctx, "share@example.com", "sub-share", "learner")
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES (?, 'cat')`, units.Normalize("חָתוּל"))
	if err != nil {
		t.Fatal(err)
	}
	lex, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateNewCards(ctx, user, []int64{lex}, when, func(bool) []string {
		return []string{"read"}
	}); err != nil {
		t.Fatal(err)
	}
	taught, err := s.RecordTaught(ctx, user, []int64{lex}, "2026-11-01", when.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(taught.Words) != 1 {
		t.Fatalf("taught = %#v", taught)
	}
	// read already existed; recall is new.
	got := strings.Join(taught.Words[0].Created, ",")
	if got != "recall" {
		t.Fatalf("taught created %q, want recall only", got)
	}
}
