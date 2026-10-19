// Package choices builds the buttons for one graded ask.
// It does not read the database. Callers pass the lexemes they already loaded.
package choices

import (
	"cmp"
	"crypto/sha256"
	"encoding/binary"
	"math/rand/v2"
	"slices"
)

// ChoiceSeed identifies one ask. Pick hashes every field, in this order:
//
//  1. RunID, the lesson run id
//  2. Step, the zero-based activity index
//  3. Attempt, the attempt number on that step, or 0 when the run stores none
//  4. TargetID, the target lexeme id
//
// lesson_runs has no attempt column, so the player passes Attempt 0.
// A reload or an htmx re-render that repeats these four values returns
// the same buttons in the same order.
type ChoiceSeed struct {
	RunID    int64
	Step     int
	Attempt  int
	TargetID int64
}

// Lexeme is one dictionary row Pick can place on a button.
// Text is the pointed text_nfc. Pick rejects a second row with the same
// ID or the same Text. POS is the part of speech. Earlier is false when
// the row belongs to the unit being asked, and true when it was taught
// in an earlier unit and is only a fallback.
type Lexeme struct {
	ID      int64
	Text    string
	POS     string
	Earlier bool
}

// Pick returns the buttons for one ask, target included, in button order.
//
// n is the number of buttons including the target. The lesson player passes
// 3, the target plus the two distractors it has always shown. The slice is
// shorter when pool does not hold that many other lexemes. The result is
// nil when target.ID is 0 or n is less than 1.
//
// The target is always the target argument, exactly once. Distractors are
// other rows in pool. A row is skipped when its ID is 0, its ID equals the
// target, or its Text equals the target Text. Duplicate IDs and duplicate
// Texts are dropped. Text order, then ID, decides which duplicate is kept,
// so the result does not depend on pool order and Pick does not mutate pool.
//
// Distractors are taken in four tiers. Each tier is shuffled with the seeded
// generator, then Pick takes only as many as it still needs:
//
//  1. Same part of speech, from this unit (Earlier false)
//  2. Any other part of speech from this unit
//  3. Same part of speech from earlier units
//  4. Any other part of speech from earlier units
//
// The target and those distractors are then shuffled into button order.
//
// The generator is a math/rand/v2 PCG. Its two seeds are the first and
// second little-endian uint64 values of the SHA-256 digest of ChoiceSeed.
// Those four fields are written, in the order above, as four little-endian
// uint64 values. Step and Attempt are converted to uint64. A tier that is
// not needed is not shuffled, so extra earlier rows do not change the
// buttons once this unit has already supplied every distractor.
func Pick(seed ChoiceSeed, target Lexeme, pool []Lexeme, n int) []Lexeme {
	if target.ID == 0 || n < 1 {
		return nil
	}
	rng := newPCG(seed)
	want := n - 1
	var distractors []Lexeme
	for _, tier := range distractorTiers(pool, target) {
		if len(distractors) >= want {
			break
		}
		distractors = append(distractors, take(rng, tier, want-len(distractors))...)
	}
	out := make([]Lexeme, 0, 1+len(distractors))
	out = append(out, target)
	out = append(out, distractors...)
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// newPCG seeds a PCG from the SHA-256 of ChoiceSeed. The layout is part of
// the contract: four little-endian uint64 fields, then the first 16 digest
// bytes as two little-endian uint64 seeds.
func newPCG(seed ChoiceSeed) *rand.Rand {
	var buf [32]byte
	binary.LittleEndian.PutUint64(buf[0:8], uint64(seed.RunID))
	binary.LittleEndian.PutUint64(buf[8:16], uint64(seed.Step))
	binary.LittleEndian.PutUint64(buf[16:24], uint64(seed.Attempt))
	binary.LittleEndian.PutUint64(buf[24:32], uint64(seed.TargetID))
	sum := sha256.Sum256(buf[:])
	return rand.New(rand.NewPCG(
		binary.LittleEndian.Uint64(sum[0:8]),
		binary.LittleEndian.Uint64(sum[8:16]),
	))
}

func distractorTiers(pool []Lexeme, target Lexeme) [][]Lexeme {
	var unit, earlier []Lexeme
	for _, lex := range pool {
		if lex.ID == 0 || lex.ID == target.ID || lex.Text == target.Text {
			continue
		}
		if lex.Earlier {
			earlier = append(earlier, lex)
		} else {
			unit = append(unit, lex)
		}
	}
	unit = dedupe(unit)
	earlier = dedupe(earlier)
	seenID := map[int64]bool{}
	seenText := map[string]bool{}
	for _, lex := range unit {
		seenID[lex.ID] = true
		seenText[lex.Text] = true
	}
	var filtered []Lexeme
	for _, lex := range earlier {
		if seenID[lex.ID] || seenText[lex.Text] {
			continue
		}
		seenID[lex.ID] = true
		seenText[lex.Text] = true
		filtered = append(filtered, lex)
	}
	unitSame, unitOther := splitPOS(unit, target.POS)
	earlierSame, earlierOther := splitPOS(filtered, target.POS)
	return [][]Lexeme{unitSame, unitOther, earlierSame, earlierOther}
}

func dedupe(rows []Lexeme) []Lexeme {
	if len(rows) == 0 {
		return nil
	}
	rows = slices.Clone(rows)
	slices.SortFunc(rows, func(a, b Lexeme) int {
		return cmp.Or(cmp.Compare(a.Text, b.Text), cmp.Compare(a.ID, b.ID))
	})
	out := make([]Lexeme, 0, len(rows))
	seenID := map[int64]bool{}
	seenText := map[string]bool{}
	for _, lex := range rows {
		if seenID[lex.ID] || seenText[lex.Text] {
			continue
		}
		seenID[lex.ID] = true
		seenText[lex.Text] = true
		out = append(out, lex)
	}
	return out
}

func splitPOS(rows []Lexeme, pos string) (same, other []Lexeme) {
	for _, lex := range rows {
		if lex.POS == pos {
			same = append(same, lex)
		} else {
			other = append(other, lex)
		}
	}
	return same, other
}

func take(rng *rand.Rand, rows []Lexeme, n int) []Lexeme {
	if n <= 0 || len(rows) == 0 {
		return nil
	}
	cp := slices.Clone(rows)
	rng.Shuffle(len(cp), func(i, j int) { cp[i], cp[j] = cp[j], cp[i] })
	if n > len(cp) {
		n = len(cp)
	}
	out := make([]Lexeme, n)
	copy(out, cp[:n])
	return out
}
