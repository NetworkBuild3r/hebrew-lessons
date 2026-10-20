package session

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

// fixedClock is the instant BuildSession read once. The learner model reads it, so the
// whole build sees one now.
type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

// onceBands reads the placement bands the first time the model asks and keeps them for the
// rest of one build. The model asks once per word otherwise.
type onceBands struct {
	src  learner.BandSource
	once sync.Once
	m    map[int64]int
	err  error
}

func (o *onceBands) LexemeBands(ctx context.Context) (map[int64]int, error) {
	o.once.Do(func() {
		if o.src != nil {
			o.m, o.err = o.src.LexemeBands(ctx)
		}
	})
	return o.m, o.err
}

// factIndex says, for every lexeme, which activities can ask it. It is built from one read
// of the lexicon.
type factIndex struct {
	words   map[int64]db.PoolWord
	glossed int
	voiced  int
}

func (s *Service) factsByLexeme(ctx context.Context, userID int64) (factIndex, error) {
	pool, err := s.store.ActivityPool(ctx, userID)
	if err != nil {
		return factIndex{}, err
	}
	idx := factIndex{words: make(map[int64]db.PoolWord, len(pool))}
	for _, w := range pool {
		idx.words[w.ID] = w
		if strings.TrimSpace(w.Gloss) != "" {
			idx.glossed++
			if w.AudioID != "" {
				idx.voiced++
			}
		}
	}
	return idx, nil
}

// of is the facts of one lexeme. A word the lexicon does not have has none, so every visit
// of it is a flashcard.
func (f factIndex) of(lexemeID int64) activity.Facts {
	w, ok := f.words[lexemeID]
	if !ok {
		return activity.Facts{}
	}
	hasGloss := strings.TrimSpace(w.Gloss) != ""
	hasAudio := w.AudioID != ""
	facts := activity.Facts{HasGloss: hasGloss, HasAudio: hasAudio, GlossedOthers: f.glossed, VoicedOthers: f.voiced}
	if hasGloss {
		facts.GlossedOthers--
		if hasAudio {
			facts.VoicedOthers--
		}
	}
	return facts
}
