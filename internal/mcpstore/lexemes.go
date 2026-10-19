package mcpstore

import (
	"context"
	"errors"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
)

// UpsertLexemes calls db.UpsertLexemes. A *lexicon.Invalid batch is
// returned as that error so the tool can show the JSON pointer. Other
// store errors stay wrapped for the tool to hide.
func (s *Store) UpsertLexemes(ctx context.Context, entries []lexicon.Input) ([]mcp.LexemeResult, []int64, error) {
	if err := s.ready(ctx); err != nil {
		return nil, nil, err
	}
	saved, rebuilt, err := s.db.UpsertLexemes(ctx, entries)
	if err != nil {
		var bad *lexicon.Invalid
		if errors.As(err, &bad) {
			return nil, nil, bad
		}
		return nil, nil, err
	}
	out := make([]mcp.LexemeResult, len(saved))
	for i, row := range saved {
		out[i] = mcp.LexemeResult{TextNFC: row.TextNFC, ID: row.ID, Status: row.Status}
	}
	if rebuilt == nil {
		rebuilt = []int64{}
	}
	return out, rebuilt, nil
}
