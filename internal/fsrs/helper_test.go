package fsrs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

func (f *fakeClock) Advance(d time.Duration) { f.t = f.t.Add(d) }

func openReview(t *testing.T) (*Review, *db.Store, *fakeClock) {
	t.Helper()
	clk := &fakeClock{t: time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)}
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return New(store, clk), store, clk
}

func mustUser(t *testing.T, store *db.Store, email string) int64 {
	t.Helper()
	id, err := store.InsertUser(context.Background(), email, "sub-"+email, "learner")
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

func mustCard(t *testing.T, store *db.Store, card db.Card) int64 {
	t.Helper()
	id, err := store.InsertCard(context.Background(), card)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
