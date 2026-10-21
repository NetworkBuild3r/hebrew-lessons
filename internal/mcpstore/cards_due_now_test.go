package mcpstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

func TestStoreCardsDueNowUsesListDueCards(t *testing.T) {
	when := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	store, raw := openAdapter(t, fixedClock{when})
	ctx := context.Background()
	user := mustUser(t, raw)
	other := mustOtherUser(t, raw)
	skills := []string{"listen", "read", "recall", "spell"}
	states := []string{"new", "learning", "review", "relearning"}
	for i, state := range states {
		insertDueCard(t, raw, user, "due-"+state, skills[i], state, when, false)
	}
	insertDueCard(t, raw, user, "due-later", "listen", "review", when.Add(time.Nanosecond), false)
	insertDueCard(t, raw, user, "due-suspended", "read", "new", when, true)
	insertDueCard(t, raw, other, "due-other", "recall", "learning", when, false)

	got, err := store.CardsDueNow(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	want := mcp.CardsDueNow{New: 1, Learning: 1, Review: 1, Relearning: 1, Total: 4}
	if got != want {
		t.Fatalf("cards due now %+v", got)
	}
	listed, err := raw.ListDueCards(ctx, user, when)
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != len(listed) {
		t.Fatalf("total %d list %d", got.Total, len(listed))
	}
}

func TestStoreCardsDueNowErrorLegs(t *testing.T) {
	ctx := context.Background()
	if _, err := (*Store)(nil).CardsDueNow(ctx, 1); err == nil {
		t.Fatal("nil store")
	}
	if _, err := New(nil).CardsDueNow(ctx, 1); err == nil {
		t.Fatal("nil database")
	}

	when := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	store, raw := openAdapter(t, fixedClock{when})
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.CardsDueNow(cancelled, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CardsDueNow(ctx, 1); err == nil {
		t.Fatal("closed database")
	}
}

func insertDueCard(t *testing.T, raw *db.Store, user int64, text, skill, state string, due time.Time, suspended bool) {
	t.Helper()
	lex := mustLexeme(t, raw, text)
	if _, err := raw.InsertCard(context.Background(), db.Card{
		UserID: user, LexemeID: lex, Skill: skill, State: state, Due: due, Suspended: suspended,
	}); err != nil {
		t.Fatal(err)
	}
}
