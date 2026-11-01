package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Store-level Taught / RemoveTaught coverage beside the evidence package ACs.
func TestTaughtRecordAndRemove(t *testing.T) {
	when := time.Date(2026, 11, 1, 17, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user, err := s.InsertUser(ctx, "taught@example.com", "sub-taught", "learner")
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
	due := time.Date(2026, 11, 2, 11, 0, 0, 0, time.UTC)
	out, err := s.RecordTaught(ctx, user, []int64{lex}, "2026-11-01", due)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Words) != 1 || out.Words[0].Already || len(out.Words[0].Created) != 2 {
		t.Fatalf("taught = %#v", out)
	}
	again, err := s.RecordTaught(ctx, user, []int64{lex}, "2026-11-01", due)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Words) != 1 || !again.Words[0].Already {
		t.Fatalf("repeat = %#v", again)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, user); n != 2 {
		t.Fatalf("cards = %d", n)
	}
	rm, err := s.RemoveTaughtRows(ctx, user, "2026-11-01", []int64{lex})
	if err != nil {
		t.Fatal(err)
	}
	if len(rm.Words) != 1 || len(rm.Words[0].Removed) != 2 || len(rm.Words[0].Kept) != 0 {
		t.Fatalf("remove = %#v", rm)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM cards WHERE user_id = ?`, user); n != 0 {
		t.Fatalf("cards after remove = %d", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM taught_words WHERE learner_id = ?`, user); n != 0 {
		t.Fatalf("taught_words = %d", n)
	}
}

func TestTaughtListenWhenClipStored(t *testing.T) {
	when := time.Date(2026, 11, 1, 17, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user, err := s.InsertUser(ctx, "audio@example.com", "sub-audio", "learner")
	if err != nil {
		t.Fatal(err)
	}
	text := units.Normalize("שָׁלוֹם")
	clip := strings.Repeat("b", 64)
	ok, err := s.InsertAudio(ctx, Asset{ID: clip, TextNFC: text, Voice: "v", Model: "m", Bytes: 4, CreatedAt: when})
	if err != nil || !ok {
		t.Fatalf("audio: %v %v", ok, err)
	}
	res, err := s.conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en, audio_id) VALUES (?, 'hello', ?)`, text, clip)
	if err != nil {
		t.Fatal(err)
	}
	lex, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	due := when.Add(time.Hour)
	out, err := s.RecordTaught(ctx, user, []int64{lex}, "2026-11-01", due)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Words) != 1 {
		t.Fatalf("words = %#v", out)
	}
	got := strings.Join(out.Words[0].Created, ",")
	if got != "read,recall,listen" {
		t.Fatalf("created = %q", got)
	}
}
