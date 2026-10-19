package learner

import (
	"context"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
)

// placementBands gives a lexeme the band of the placement item it was dealt as.
//
// The lexicon has no band column. A word has a band only when some placement run has rated
// it, which links the word to a deck item, and the deck says the item's band. A word that was
// never placed has no band, and the band filter and the band-slip cause leave it out. The link
// carries no learner, so every learner sees the same bands.
type placementBands struct{ store *db.Store }

func (p placementBands) LexemeBands(ctx context.Context) (map[int64]int, error) {
	if p.store == nil {
		return nil, nil
	}
	pairs, err := p.store.PlacementItemLexemes(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int, len(pairs))
	for _, pair := range pairs {
		_, band, ok := placement.Lookup(pair.ItemID)
		if !ok {
			continue
		}
		// A word dealt in two bands (a deck that changed) takes the lower, more common one.
		if prev, seen := out[pair.LexemeID]; !seen || band < prev {
			out[pair.LexemeID] = band
		}
	}
	return out, nil
}
