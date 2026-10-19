package audio

import (
	"context"
	"errors"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// StoreAudio is a units.Lookup whose HasAudio answers from audio_assets.
// Validate passes units.Normalize(text). HasAudio runs that same normalizer
// again, which is idempotent, then AudioByText.
//
// The other Lookup methods are lexicon data. They delegate to Inner.
// A nil Inner reports no lexemes.
type StoreAudio struct {
	Inner units.Lookup
	Store *db.Store
}

// HasAudio reports whether AudioByText finds a clip for text.
func (s StoreAudio) HasAudio(ctx context.Context, text string) (bool, error) {
	if s.Store == nil {
		return false, errors.New("audio: store is nil")
	}
	_, ok, err := s.Store.AudioByText(ctx, NormalizeText(text))
	return ok, err
}

// KnownLexemes delegates to Inner.
func (s StoreAudio) KnownLexemes(ctx context.Context, beforePosition int) (map[string]bool, error) {
	if s.Inner == nil {
		return map[string]bool{}, nil
	}
	return s.Inner.KnownLexemes(ctx, beforePosition)
}

// LexemeByText delegates to Inner.
func (s StoreAudio) LexemeByText(ctx context.Context, textNFC string) (int64, bool, error) {
	if s.Inner == nil {
		return 0, false, nil
	}
	return s.Inner.LexemeByText(ctx, textNFC)
}

// Facts delegates to Inner.
func (s StoreAudio) Facts(ctx context.Context, textNFC string) (units.LexemeFacts, bool, error) {
	if s.Inner == nil {
		return units.LexemeFacts{}, false, nil
	}
	return s.Inner.Facts(ctx, textNFC)
}

var _ units.Lookup = StoreAudio{}
