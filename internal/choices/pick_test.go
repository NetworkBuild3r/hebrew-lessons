package choices_test

import (
	"cmp"
	"reflect"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/choices"
)

func TestPick(t *testing.T) {
	t.Run("reload is stable and ignores pool order", func(t *testing.T) {
		pool := eightLexemes()
		target := pool[4]
		seed := choices.ChoiceSeed{RunID: 11, Step: 2, Attempt: 0, TargetID: target.ID}
		first := choices.Pick(seed, target, pool, 3)
		second := choices.Pick(seed, target, pool, 3)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("reload changed the buttons\n%v\n%v", first, second)
		}
		rev := slices.Clone(pool)
		slices.Reverse(rev)
		if !reflect.DeepEqual(first, choices.Pick(seed, target, rev, 3)) {
			t.Fatal("pool order changed the buttons")
		}
		before := slices.Clone(pool)
		choices.Pick(seed, target, pool, 3)
		if !slices.Equal(before, pool) {
			t.Fatal("Pick mutated the pool")
		}
	})

	t.Run("same part of speech is preferred", func(t *testing.T) {
		pool := eightLexemes()
		for _, runID := range []int64{1, 2, 9} {
			for _, target := range pool {
				got := choices.Pick(seedFor(runID, 0, 0, target), target, pool, 3)
				if !samePOS(got, target) {
					t.Fatalf("run %d target %d POS %s lost its matches: %+v", runID, target.ID, target.POS, got)
				}
			}
		}
	})

	t.Run("fallback fills the unit before earlier units", func(t *testing.T) {
		target := lx(1, "א", "noun", false)
		// Two unit verbs are enough. Earlier nouns, even same POS, stay out.
		unitVerbs := []choices.Lexeme{lx(2, "ב", "verb", false), lx(3, "ג", "verb", false)}
		earlierNouns := []choices.Lexeme{lx(4, "ד", "noun", true), lx(5, "ה", "noun", true), lx(6, "ו", "noun", true)}
		got := choices.Pick(seedFor(3, 1, 0, target), target, append(append([]choices.Lexeme{target}, unitVerbs...), earlierNouns...), 3)
		if !sameIDs(distractors(got, target), []int64{2, 3}) {
			t.Fatalf("unit verbs = %+v, want ids 2 and 3", got)
		}
		// One unit lexeme, then an earlier same-POS row, then an earlier other.
		one := lx(7, "ז", "verb", false)
		earlierSame := lx(8, "ח", "noun", true)
		earlierOther := lx(9, "ט", "verb", true)
		got = choices.Pick(seedFor(3, 1, 0, target), target, []choices.Lexeme{target, one, earlierSame, earlierOther}, 3)
		ids := distractors(got, target)
		if !containsID(ids, 7) || !containsID(ids, 8) || containsID(ids, 9) {
			t.Fatalf("fallback = %+v, want unit verb 7 and earlier noun 8", got)
		}
		// A one-word unit takes both distractors from earlier units.
		got = choices.Pick(seedFor(4, 0, 0, target), target, []choices.Lexeme{target, earlierSame, earlierOther, lx(10, "י", "verb", true)}, 3)
		if len(got) != 3 || containsID(distractors(got, target), target.ID) {
			t.Fatalf("earlier fill = %+v", got)
		}
		for _, lex := range got {
			if lex.ID != target.ID && !lex.Earlier {
				t.Fatalf("distractor %+v is not from an earlier unit", lex)
			}
		}
	})

	t.Run("earlier rows do not move buttons the unit already filled", func(t *testing.T) {
		pool := eightLexemes()
		target := pool[0]
		seed := seedFor(6, 1, 0, target)
		without := choices.Pick(seed, target, pool, 3)
		with := append(slices.Clone(pool), lx(90, "ק", "noun", true), lx(91, "ר", "verb", true))
		if !reflect.DeepEqual(without, choices.Pick(seed, target, with, 3)) {
			t.Fatal("unused earlier rows changed the buttons")
		}
	})

	t.Run("duplicate id or text is dropped", func(t *testing.T) {
		target := lx(1, "א", "noun", false)
		pool := []choices.Lexeme{
			target,
			lx(9, "ב", "noun", false),
			lx(2, "ב", "noun", false),
			lx(2, "ג", "verb", false),
			lx(4, "א", "noun", false),
			lx(3, "ד", "verb", true),
			lx(2, "ה", "noun", true),
		}
		got := choices.Pick(seedFor(1, 0, 0, target), target, pool, 3)
		assertClean(t, got, target)
		if !containsID(idsOf(got), 2) || containsID(idsOf(got), 9) {
			t.Fatalf("kept the wrong duplicate: %+v", got)
		}
	})

	t.Run("seed fields change the buttons", func(t *testing.T) {
		pool := eightLexemes()
		target := pool[5]
		base := seedFor(5, 1, 0, target)
		baseGot := choices.Pick(base, target, pool, 3)
		alts := []choices.ChoiceSeed{
			{RunID: 6, Step: base.Step, Attempt: base.Attempt, TargetID: base.TargetID},
			{RunID: base.RunID, Step: 4, Attempt: base.Attempt, TargetID: base.TargetID},
			{RunID: base.RunID, Step: base.Step, Attempt: 2, TargetID: base.TargetID},
			{RunID: base.RunID, Step: base.Step, Attempt: base.Attempt, TargetID: base.TargetID + 50},
		}
		names := []string{"run", "step", "attempt", "target id"}
		for i, alt := range alts {
			changed := false
			try := alt
			for k := 0; k < 12; k++ {
				switch i {
				case 0:
					try.RunID = int64(k + 1)
					if try.RunID == base.RunID {
						continue
					}
				case 1:
					try.Step = k
					if try.Step == base.Step {
						continue
					}
				case 2:
					try.Attempt = k
					if try.Attempt == base.Attempt {
						continue
					}
				default:
					try.TargetID = int64(100 + k)
				}
				if !reflect.DeepEqual(baseGot, choices.Pick(try, target, pool, 3)) {
					changed = true
					break
				}
			}
			if !changed {
				t.Fatalf("%s is not part of the seed", names[i])
			}
		}
	})

	t.Run("distractor sets vary across targets and runs", func(t *testing.T) {
		pool := eightLexemes()
		sets := map[string]bool{}
		for _, target := range pool {
			got := choices.Pick(seedFor(8, 0, 0, target), target, pool, 3)
			sets[idKey(distractors(got, target))] = true
		}
		if len(sets) < 2 {
			t.Fatalf("one distractor set for every target: %v", sets)
		}
		target := pool[6]
		runs := map[string]bool{}
		for runID := int64(1); runID <= 24; runID++ {
			got := choices.Pick(seedFor(runID, 0, 0, target), target, pool, 3)
			runs[idKey(distractors(got, target))] = true
		}
		if len(runs) < 2 {
			t.Fatalf("run id did not change distractors for target %d: %v", target.ID, runs)
		}
	})

	t.Run("regression correct answer is not always the third button", func(t *testing.T) {
		// The old picker took the two alphabetically first lexemes and then
		// sorted the buttons the same way, so any target that sorted third
		// or later was always button index 2.
		pool := eightLexemes()
		sorted := slices.Clone(pool)
		slices.SortFunc(sorted, func(a, b choices.Lexeme) int {
			return cmp.Or(cmp.Compare(a.Text, b.Text), cmp.Compare(a.ID, b.ID))
		})
		seedRun := int64(1)
		var all, late []int
		for i, target := range sorted {
			got := choices.Pick(seedFor(seedRun, 0, 0, target), target, pool, 3)
			idx := indexOf(got, target.ID)
			if idx < 0 {
				t.Fatalf("target %d missing from %+v", target.ID, got)
			}
			all = append(all, idx)
			if i >= 2 {
				late = append(late, idx)
			}
		}
		if len(uniqueInts(all)) < 2 {
			t.Fatalf("correct index takes one value: %v", all)
		}
		if len(uniqueInts(late)) < 2 || allEqual(late, 2) {
			t.Fatalf("targets that sort 3rd or later stay on one index: %v", late)
		}
	})

	t.Run("no target is always the odd one out", func(t *testing.T) {
		pool := eightLexemes()
		oddPOS := 0
		oddLen := 0
		for _, runID := range []int64{1, 4, 15} {
			oddPOS = 0
			oddLen = 0
			for _, target := range pool {
				got := choices.Pick(seedFor(runID, 3, 0, target), target, pool, 3)
				if !samePOS(got, target) {
					oddPOS++
				}
				if uniqueLen(got, target) {
					oddLen++
				}
			}
			if oddPOS == len(pool) {
				t.Fatalf("run %d: every target is the only member of its part of speech", runID)
			}
			if oddPOS != 0 {
				t.Fatalf("run %d: %d targets are the only member of their part of speech", runID, oddPOS)
			}
			if oddLen == len(pool) {
				t.Fatalf("run %d: every target has a unique length", runID)
			}
		}
	})

	t.Run("distractors do not repeat across a lesson run", func(t *testing.T) {
		pool := eightLexemes()
		// One duplicate text and one duplicate id sit in the pool on purpose.
		pool = append(pool, pool[1], choices.Lexeme{ID: pool[2].ID, Text: "ש", POS: "noun"})
		const runID = 42
		// Step 1 shows two choice sets (two attempts). The others show one.
		steps := []struct {
			step int
			sets []struct {
				target  choices.Lexeme
				attempt int
			}
		}{
			{step: 0, sets: []struct {
				target  choices.Lexeme
				attempt int
			}{{pool[0], 0}}},
			{step: 1, sets: []struct {
				target  choices.Lexeme
				attempt int
			}{{pool[3], 0}, {pool[4], 1}}},
			{step: 2, sets: []struct {
				target  choices.Lexeme
				attempt int
			}{{pool[5], 0}}},
			{step: 3, sets: []struct {
				target  choices.Lexeme
				attempt int
			}{{pool[6], 0}, {pool[7], 0}}},
		}
		for _, step := range steps {
			for _, set := range step.sets {
				got := choices.Pick(choices.ChoiceSeed{
					RunID: runID, Step: step.step, Attempt: set.attempt, TargetID: set.target.ID,
				}, set.target, pool, 3)
				assertClean(t, got, set.target)
			}
		}
	})
}

func TestPickEdges(t *testing.T) {
	pool := eightLexemes()
	target := pool[0]
	if choices.Pick(seedFor(1, 0, 0, target), choices.Lexeme{}, pool, 3) != nil {
		t.Fatal("zero target id returned buttons")
	}
	if choices.Pick(seedFor(1, 0, 0, target), target, pool, 0) != nil {
		t.Fatal("n < 1 returned buttons")
	}
	only := choices.Pick(seedFor(1, 0, 0, target), target, nil, 1)
	if len(only) != 1 || only[0].ID != target.ID {
		t.Fatalf("n = 1: %+v", only)
	}
	short := choices.Pick(seedFor(1, 0, 0, target), target, nil, 3)
	if len(short) != 1 || short[0].ID != target.ID {
		t.Fatalf("empty pool: %+v", short)
	}
}

// eightLexemes is four nouns and four verbs. Lengths repeat inside each part
// of speech (one letter, then three words of two letters) so a same-POS
// distractor shares a length with the short targets. Texts sort in this order.
func eightLexemes() []choices.Lexeme {
	return []choices.Lexeme{
		lx(1, "א", "noun", false),
		lx(2, "אב", "noun", false),
		lx(3, "אג", "noun", false),
		lx(4, "אד", "noun", false),
		lx(5, "ב", "verb", false),
		lx(6, "בא", "verb", false),
		lx(7, "בג", "verb", false),
		lx(8, "בד", "verb", false),
	}
}

func lx(id int64, text, pos string, earlier bool) choices.Lexeme {
	return choices.Lexeme{ID: id, Text: text, POS: pos, Earlier: earlier}
}

func seedFor(run int64, step, attempt int, target choices.Lexeme) choices.ChoiceSeed {
	return choices.ChoiceSeed{RunID: run, Step: step, Attempt: attempt, TargetID: target.ID}
}

func assertClean(t *testing.T, got []choices.Lexeme, target choices.Lexeme) {
	t.Helper()
	if len(got) == 0 {
		t.Fatal("no buttons")
	}
	seenID := map[int64]bool{}
	seenText := map[string]bool{}
	found := 0
	for _, lex := range got {
		if lex.ID == 0 || seenID[lex.ID] || seenText[lex.Text] {
			t.Fatalf("duplicate or blank button %+v in %+v", lex, got)
		}
		seenID[lex.ID] = true
		seenText[lex.Text] = true
		if lex.ID == target.ID || lex.Text == target.Text {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("target %+v appears %d times in %+v", target, found, got)
	}
}

func samePOS(got []choices.Lexeme, target choices.Lexeme) bool {
	for _, lex := range got {
		if lex.ID != target.ID && lex.POS == target.POS {
			return true
		}
	}
	return false
}

func uniqueLen(got []choices.Lexeme, target choices.Lexeme) bool {
	n := utf8.RuneCountInString(target.Text)
	for _, lex := range got {
		if lex.ID != target.ID && utf8.RuneCountInString(lex.Text) == n {
			return false
		}
	}
	return true
}

func distractors(got []choices.Lexeme, target choices.Lexeme) []int64 {
	var ids []int64
	for _, lex := range got {
		if lex.ID != target.ID {
			ids = append(ids, lex.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

func idsOf(got []choices.Lexeme) []int64 {
	ids := make([]int64, len(got))
	for i, lex := range got {
		ids[i] = lex.ID
	}
	return ids
}

func containsID(ids []int64, id int64) bool {
	return slices.Contains(ids, id)
}

func sameIDs(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func idKey(ids []int64) string {
	b := make([]byte, 0, len(ids)*2)
	for _, id := range ids {
		b = append(b, byte(id))
	}
	return string(b)
}

func indexOf(got []choices.Lexeme, id int64) int {
	for i, lex := range got {
		if lex.ID == id {
			return i
		}
	}
	return -1
}

func uniqueInts(vals []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, v := range vals {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func allEqual(vals []int, want int) bool {
	if len(vals) == 0 {
		return false
	}
	for _, v := range vals {
		if v != want {
			return false
		}
	}
	return true
}
