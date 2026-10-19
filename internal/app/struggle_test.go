package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

type fakeStruggleStore struct {
	lexemes    []mcp.LexemeStruggle
	confusions []mcp.ConfusionCount
	topN       int
	window     time.Duration
	userID     int64
	failLexeme error
}

func (f *fakeStruggleStore) LexemeStruggles(_ context.Context, userID int64, topN int) ([]mcp.LexemeStruggle, error) {
	f.userID = userID
	f.topN = topN
	if f.failLexeme != nil {
		return nil, f.failLexeme
	}
	return append([]mcp.LexemeStruggle(nil), f.lexemes...), nil
}

func (f *fakeStruggleStore) ConfusionCounts(_ context.Context, userID int64, window time.Duration) ([]mcp.ConfusionCount, error) {
	f.userID = userID
	f.window = window
	return append([]mcp.ConfusionCount(nil), f.confusions...), nil
}

func TestStruggleAdapterKeepsStoreOrder(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	window := 30 * 24 * time.Hour
	store := &fakeStruggleStore{
		lexemes: []mcp.LexemeStruggle{
			{LexemeID: 1, TextNFC: "א", Gloss: "alef", Reviews: 2, Lapses: 0, AgainRate: 0.1, LastReviewAt: when},
			{LexemeID: 2, TextNFC: "ג", Gloss: "gimel", Reviews: 9, Lapses: 8, AgainRate: 0.9, Misses7d: 3, LastReviewAt: when},
		},
		confusions: []mcp.ConfusionCount{
			{Kind: "pair", Key: "recent", Count: 4, LastAt: when},
			{Kind: "root", Key: "zero", Count: 1},
		},
	}
	got, err := newStruggleSource(store).Struggles(context.Background(), 7, 30, window)
	if err != nil {
		t.Fatal(err)
	}
	if store.userID != 7 || store.topN != 30 || store.window != window {
		t.Fatalf("store call user %d top %d window %s", store.userID, store.topN, store.window)
	}
	if len(got.Lexemes) != 2 || got.Lexemes[0].TextNFC != "א" || got.Lexemes[0].Lapses != 0 || got.Lexemes[1].TextNFC != "ג" || got.Lexemes[1].Gloss != "gimel" || got.Lexemes[1].Reviews != 9 {
		t.Fatalf("lexemes reordered %+v", got.Lexemes)
	}
	if !got.Lexemes[1].LastReviewAt.Equal(when) || got.Lexemes[1].Misses7d != 3 {
		t.Fatalf("lexeme fields %+v", got.Lexemes[1])
	}
	if len(got.Confusions) != 2 || got.Confusions[0].Kind != "pair" || got.Confusions[0].Key != "recent" || got.Confusions[0].Count != 4 || !got.Confusions[0].LastAt.Equal(when) || got.Confusions[1].Key != "zero" || !got.Confusions[1].LastAt.IsZero() {
		t.Fatalf("confusions %+v", got.Confusions)
	}
}

func TestStruggleAdapterEmptySlices(t *testing.T) {
	got, err := newStruggleSource(&fakeStruggleStore{}).Struggles(context.Background(), 1, 30, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got.Lexemes == nil || got.Confusions == nil || len(got.Lexemes) != 0 || len(got.Confusions) != 0 {
		t.Fatalf("empty %+v", got)
	}
}

func TestStruggleAdapterStopsOnLexemeError(t *testing.T) {
	store := &fakeStruggleStore{failLexeme: errors.New("boom")}
	if _, err := newStruggleSource(store).Struggles(context.Background(), 1, 1, time.Hour); err == nil {
		t.Fatal("expected error")
	}
	if store.window != 0 {
		t.Fatal("confusion query ran after the lexeme error")
	}
}
