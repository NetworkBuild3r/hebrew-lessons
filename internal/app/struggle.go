package app

import (
	"context"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

// struggleStore is the read this adapter calls.
//
// Today that is LexemeStruggles(ctx, userID, topN) plus
// ConfusionCounts(ctx, userID, window). Backend Dev has also described
// StruggleStats(ctx, userID, topN) db.Struggle{Lexemes, Confusions{Kind, A, B, Count, LastAt}, Top}.
// That choice is not settled. If StruggleStats is the one that lands,
// change only this file: call StruggleStats, ignore Top, and set each
// confusion key to A/B, or to A when B is empty. Do not re-rank.
type struggleStore interface {
	LexemeStruggles(ctx context.Context, userID int64, topN int) ([]mcp.LexemeStruggle, error)
	ConfusionCounts(ctx context.Context, userID int64, window time.Duration) ([]mcp.ConfusionCount, error)
}

// struggleSource adapts the store read to mcp.StruggleSource.
// It copies the rows. It does not rank them and it does not filter them.
type struggleSource struct {
	store struggleStore
}

func newStruggleSource(store struggleStore) mcp.StruggleSource {
	return struggleSource{store: store}
}

func (a struggleSource) Struggles(ctx context.Context, userID int64, topN int, window time.Duration) (mcp.Struggles, error) {
	lexemes, err := a.store.LexemeStruggles(ctx, userID, topN)
	if err != nil {
		return mcp.Struggles{}, err
	}
	confusions, err := a.store.ConfusionCounts(ctx, userID, window)
	if err != nil {
		return mcp.Struggles{}, err
	}
	if lexemes == nil {
		lexemes = []mcp.LexemeStruggle{}
	}
	if confusions == nil {
		confusions = []mcp.ConfusionCount{}
	}
	return mcp.Struggles{Lexemes: lexemes, Confusions: confusions}, nil
}
