package web

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math/rand/v2"
	"strings"
	"unicode"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// sentenceListenChoicesAreTranslations is the sentence listen_pick choice
// rule. A line of two or more lexicon words is asked as the author's English
// of the whole line, including when target credits one word of that line.
const sentenceListenChoicesAreTranslations = true

// sentenceDistractorCap is how many other English lines a sentence listen_pick
// shows. Fewer than that is the whole set. Glosses are not used to fill it.
const sentenceDistractorCap = 4

// sentenceListenPick reports a listen_pick whose choices are whole-line
// English. A single lexicon word stays on the gloss buttons.
func sentenceListenPick(act units.Activity, lex map[string]db.LexemeView) bool {
	if act.Type != units.ActivityListenPick || lexemeTokenCount(act, lex) < 2 {
		return false
	}
	if strings.TrimSpace(act.Target) != "" && !sentenceListenChoicesAreTranslations {
		return false
	}
	return true
}

// listenPickTarget is the word a listen_pick credits.
// A set target credits only that word, and does not fall back when the token
// is missing. With no target, a line of one lexicon word credits that word.
// Two or more lexicon words credit nobody: the caller logs an exposure.
func listenPickTarget(act units.Activity, lex map[string]db.LexemeView) db.LexemeView {
	if tok, ok := tokenForTarget(actTokens(act), act.Target); ok {
		if view, found := lex[tok]; found {
			return view
		}
	}
	if strings.TrimSpace(act.Target) != "" {
		return db.LexemeView{}
	}
	var only db.LexemeView
	n := 0
	for _, tok := range actTokens(act) {
		view, ok := lex[tok]
		if !ok {
			continue
		}
		n++
		only = view
	}
	if n == 1 {
		return only
	}
	return db.LexemeView{}
}

// listenLexemeIDs is every lexicon token on the line, in order, once each.
func listenLexemeIDs(act units.Activity, lex map[string]db.LexemeView) []int64 {
	var ids []int64
	seen := map[int64]bool{}
	for _, tok := range actTokens(act) {
		view, ok := lex[tok]
		if !ok || view.ID == 0 || seen[view.ID] {
			continue
		}
		seen[view.ID] = true
		ids = append(ids, view.ID)
	}
	return ids
}

// foldTranslation is the key the unit contract uses to tell English lines
// apart: lowercase, punctuation removed, and runs of space collapsed to one.
// units.foldTranslation is the same function. It is not exported.
func foldTranslation(s string) string {
	var b strings.Builder
	gap := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsPunct(r) {
			continue
		}
		if unicode.IsSpace(r) {
			gap = true
			continue
		}
		if gap && b.Len() > 0 {
			b.WriteByte(' ')
		}
		gap = false
		b.WriteRune(r)
	}
	return b.String()
}

// sentenceChoices is the answer plus up to sentenceDistractorCap other lines.
// lines is every English line in the unit, including the answer. A line that
// folds to the answer, or to a line already kept, is dropped. The button
// value is the author's text, which is what grading compares.
func sentenceChoices(answer string, lines []string, rng *rand.Rand) []views.Choice {
	if foldTranslation(answer) == "" || rng == nil {
		return nil
	}
	seen := map[string]bool{foldTranslation(answer): true}
	distractors := make([]string, 0, sentenceDistractorCap)
	for _, line := range lines {
		if len(distractors) >= sentenceDistractorCap {
			break
		}
		key := foldTranslation(line)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		distractors = append(distractors, line)
	}
	options := make([]string, 0, 1+len(distractors))
	options = append(options, answer)
	options = append(options, distractors...)
	rng.Shuffle(len(options), func(i, j int) {
		options[i], options[j] = options[j], options[i]
	})
	out := make([]views.Choice, len(options))
	for i, text := range options {
		out[i] = views.Choice{Value: text, Label: text}
	}
	return out
}

// sentenceChoiceRand shuffles one ask. The seed is the run id, the activity
// index, and the Hebrew line, so a reload keeps the order and another run
// does not.
func sentenceChoiceRand(run db.PlayerRun, act units.Activity) *rand.Rand {
	h := sha256.New()
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(run.ID))
	h.Write(buf[:])
	binary.LittleEndian.PutUint64(buf[:], uint64(run.ActivityIx))
	h.Write(buf[:])
	h.Write([]byte(act.Text))
	sum := h.Sum(nil)
	return rand.New(rand.NewPCG(
		binary.LittleEndian.Uint64(sum[0:8]),
		binary.LittleEndian.Uint64(sum[8:16]),
	))
}

// unitEnglishLines is every author-written English line in this unit:
// activity translations, then each dialogue line's translation, in document order.
func (p player) unitEnglishLines(ctx context.Context, unitID int64) ([]string, error) {
	if p.store == nil || unitID <= 0 {
		return nil, nil
	}
	snap, err := p.store.LoadUnit(ctx, unitID, nil)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, les := range snap.Doc.Lessons {
		for _, act := range les.Activities {
			if act.Translation != "" {
				lines = append(lines, act.Translation)
			}
			for _, line := range act.Lines {
				if line.Translation != "" {
					lines = append(lines, line.Translation)
				}
			}
		}
	}
	return lines, nil
}
