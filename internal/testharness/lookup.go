package testharness

import (
	"context"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// FixtureLookup is a units.Lookup built from fixture text.
// HasAudio is an exact match on the pointed NFC strings in
// testdata/audio_present.json. It does not strip or rewrite niqqud.
// Lexeme audio in the app is audio_id on the lexeme row; this lookup
// answers the text the validator passes to HasAudio.
type FixtureLookup struct {
	audio map[string]bool
	ids   map[string]int64
}

// NewFixtureLookup returns a lookup. audioTexts are the exact strings that
// have a clip. lexemeTexts are lexicon entries, assigned ids from 1.
func NewFixtureLookup(audioTexts, lexemeTexts []string) *FixtureLookup {
	audio := make(map[string]bool, len(audioTexts))
	for _, text := range audioTexts {
		audio[text] = true
	}
	ids := make(map[string]int64, len(lexemeTexts))
	for i, text := range lexemeTexts {
		ids[text] = int64(i + 1)
	}
	return &FixtureLookup{audio: audio, ids: ids}
}

// KnownLexemes returns no earlier-unit words. beforePosition is the unit
// being validated; this fixture has no course, so the map is empty.
// The authoring list is shared units only.
func (l *FixtureLookup) KnownLexemes(context.Context, int) (map[string]bool, error) {
	return map[string]bool{}, nil
}

// Facts reports that textNFC is a fixture lexeme. The fixture stores text
// only, so the returned LexemeFacts is the zero value when found is true.
func (l *FixtureLookup) Facts(_ context.Context, textNFC string) (units.LexemeFacts, bool, error) {
	if l == nil {
		return units.LexemeFacts{}, false, nil
	}
	if _, ok := l.ids[textNFC]; !ok {
		return units.LexemeFacts{}, false, nil
	}
	return units.LexemeFacts{}, true, nil
}

// HasAudio reports whether textNFC is one of the fixture clip texts.
func (l *FixtureLookup) HasAudio(_ context.Context, textNFC string) (bool, error) {
	if l == nil {
		return false, nil
	}
	return l.audio[textNFC], nil
}

// LexemeByText returns the fixture id for the exact pointed text.
func (l *FixtureLookup) LexemeByText(_ context.Context, textNFC string) (int64, bool, error) {
	if l == nil {
		return 0, false, nil
	}
	id, ok := l.ids[textNFC]
	return id, ok, nil
}

var _ units.Lookup = (*FixtureLookup)(nil)
