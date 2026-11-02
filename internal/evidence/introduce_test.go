package evidence

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestIntroduceCreatesCardsAtClockNow(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	clk := &fakeClock{t: now}
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	rec := New(store, nil, clk)
	ctx := context.Background()
	user, err := store.InsertUser(ctx, "intro@example.com", "sub-intro", "learner")
	if err != nil {
		t.Fatal(err)
	}
	res, err := store.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES (?, 'hello')`, units.Normalize("שָׁלוֹם"))
	if err != nil {
		t.Fatal(err)
	}
	lex, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	out, err := rec.Introduce(ctx, user, []int64{lex}, func(hasClip bool) []string {
		return []string{"read", "recall"}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Words) != 1 || strings.Join(out.Words[0].Created, ",") != "read,recall" {
		t.Fatalf("introduce = %#v", out)
	}
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var due string
	if err := conn.QueryRow(`SELECT due FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = 'read'`, user, lex).Scan(&due); err != nil {
		t.Fatal(err)
	}
	if due != db.FormatTime(now) {
		t.Fatalf("due = %s, want %s", due, db.FormatTime(now))
	}
	var taught, attempts, exposures, logs int
	_ = conn.QueryRow(`SELECT COUNT(*) FROM taught_words`).Scan(&taught)
	_ = conn.QueryRow(`SELECT COUNT(*) FROM attempts`).Scan(&attempts)
	_ = conn.QueryRow(`SELECT COUNT(*) FROM exposures`).Scan(&exposures)
	_ = conn.QueryRow(`SELECT COUNT(*) FROM review_log`).Scan(&logs)
	if taught != 0 || attempts != 0 || exposures != 0 || logs != 0 {
		t.Fatalf("side tables taught=%d attempts=%d exposures=%d logs=%d", taught, attempts, exposures, logs)
	}
}
