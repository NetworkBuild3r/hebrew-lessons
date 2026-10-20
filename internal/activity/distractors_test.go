package activity

import (
	"context"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func TestGlossTokens(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"house", []string{"house"}},
		{"to write; to be written", []string{"write", "written"}},
		{"the big house (m.)", []string{"big", "house"}},
		{"eating", []string{"eat"}},
		{"Dogs", []string{"dog"}},
		{"", nil},
		{"to be", nil},
	} {
		if got := GlossTokens(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("GlossTokens(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestGlossesOverlap(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"house", "house", true},
		{"house", "house, home", true},
		{"to eat", "eating", true},
		{"the house", "a house", true},
		{"house", "tree", false},
		{"to write", "to read", false},
		{"big (m.)", "small (m.)", false},
		{"", "tree", true},
		{"", "", true},
	} {
		if got := GlossesOverlap(tc.a, tc.b); got != tc.want {
			t.Errorf("GlossesOverlap(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
		if got := GlossesOverlap(tc.b, tc.a); got != tc.want {
			t.Errorf("GlossesOverlap(%q, %q) = %v, want %v (reversed)", tc.b, tc.a, got, tc.want)
		}
	}
}

func word(id int64, text, gloss, pos string, band int) Word {
	return Word{ID: id, Text: text, Gloss: gloss, POS: pos, Band: band}
}

func TestPickIsFair(t *testing.T) {
	answer := word(1, "בית", "house", "noun", 1)
	pool := []Word{
		answer,
		word(2, "בית", "tree", "noun", 1), // same spelling as the answer
		word(3, "בַּיִת", "tree", "noun", 1),          // same spelling, pointed
		word(4, "בַּיְתָה", "house, home", "noun", 1), // gloss overlaps
		word(5, "עץ", "tree", "noun", 1),
		word(6, "דג", "fish", "noun", 1),
		word(7, "דג", "to fish", "noun", 1),  // same spelling as 6
		word(8, "דגה", "fishing", "noun", 1), // gloss overlaps 6
		word(9, "כלב", "dog", "noun", 1),
		word(10, "חתול", "", "noun", 1), // no gloss
	}
	got, err := Pick(answer, pool, 3, Need{}, nil, false, rand.New(rand.NewPCG(1, 2)))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d distractors", len(got))
	}
	seenGloss := map[string]bool{}
	seenText := map[string]bool{}
	for _, w := range got {
		if w.ID == answer.ID || Normalize(w.Text) == Normalize(answer.Text) || GlossesOverlap(w.Gloss, answer.Gloss) {
			t.Fatalf("%+v is not a fair distractor of %+v", w, answer)
		}
		if seenText[Normalize(w.Text)] {
			t.Fatalf("%+v repeats a spelling", w)
		}
		for g := range seenGloss {
			if GlossesOverlap(g, w.Gloss) {
				t.Fatalf("%q overlaps another option %q", w.Gloss, g)
			}
		}
		seenText[Normalize(w.Text)] = true
		seenGloss[w.Gloss] = true
	}
}

func TestPickNeedsEnoughFairWords(t *testing.T) {
	answer := word(1, "בית", "house", "noun", 1)
	pool := []Word{answer, word(2, "עץ", "tree", "noun", 1), word(3, "דג", "fish", "noun", 1)}
	if _, err := Pick(answer, pool, 3, Need{}, nil, false, rand.New(rand.NewPCG(1, 2))); err != ErrNotEnoughWords {
		t.Fatalf("err = %v, want ErrNotEnoughWords", err)
	}
	if got, err := Pick(answer, pool, 2, Need{}, nil, false, rand.New(rand.NewPCG(1, 2))); err != nil || len(got) != 2 {
		t.Fatalf("Pick(2) = %v, %v", got, err)
	}
}

func TestPickTakesTheAnswersBandAndPartOfSpeechFirst(t *testing.T) {
	answer := word(1, "אאא", "alpha", "noun", 2)
	var pool []Word
	pool = append(pool, answer)
	id := int64(10)
	add := func(n int, pos string, band int, tag string) {
		for i := 0; i < n; i++ {
			id++
			pool = append(pool, word(id, fakeHebrew(int(id)), fakeEnglish(int(id)), pos, band))
		}
	}
	add(5, "noun", 2, "same")
	add(30, "noun", 3, "other band")
	add(30, "verb", 2, "other pos")
	add(30, "verb", 3, "neither")
	for seed := uint64(0); seed < 200; seed++ {
		got, err := Pick(answer, pool, 3, Need{}, nil, false, rand.New(rand.NewPCG(seed, 9)))
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range got {
			if w.Band != 2 || w.POS != "noun" {
				t.Fatalf("seed %d: %+v is outside the band and part of speech while the group had enough words", seed, w)
			}
		}
	}
}

func TestPickWidensWhenTheGroupIsTooSmall(t *testing.T) {
	answer := word(1, "אאא", "alpha", "noun", 2)
	pool := []Word{
		answer,
		word(2, fakeHebrew(2), fakeEnglish(2), "noun", 2), // the whole group
		word(3, fakeHebrew(3), fakeEnglish(3), "noun", 3), // same pos
		word(4, fakeHebrew(4), fakeEnglish(4), "verb", 2), // same band
		word(5, fakeHebrew(5), fakeEnglish(5), "verb", 3), // neither
	}
	got, err := Pick(answer, pool, 3, Need{}, nil, false, rand.New(rand.NewPCG(1, 2)))
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{2, 3, 4}
	var ids []int64
	for _, w := range got {
		ids = append(ids, w.ID)
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want the group, then same part of speech, then same band: %v", ids, want)
	}
}

func TestPickPrefersAConfusionPartner(t *testing.T) {
	answer := word(1, "אאא", "alpha", "noun", 2)
	pool := []Word{answer}
	for i := 2; i < 40; i++ {
		pool = append(pool, word(int64(i), fakeHebrew(i), fakeEnglish(i), "noun", 2))
	}
	partner := pool[17]
	prefer := func(w Word) bool { return w.ID == partner.ID }
	for seed := uint64(0); seed < 100; seed++ {
		got, err := Pick(answer, pool, 3, Need{}, prefer, false, rand.New(rand.NewPCG(seed, 3)))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, w := range got {
			found = found || w.ID == partner.ID
		}
		if !found {
			t.Fatalf("seed %d: the confusion partner was not chosen: %+v", seed, got)
		}
	}
	// A partner whose gloss is a synonym of the answer is still refused.
	pool[17].Gloss = "alpha, first"
	got, err := Pick(answer, pool, 3, Need{}, prefer, false, rand.New(rand.NewPCG(1, 3)))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range got {
		if w.ID == partner.ID {
			t.Fatalf("a synonym was chosen because it was preferred: %+v", w)
		}
	}
}

func TestPickWithAudioNeed(t *testing.T) {
	answer := word(1, "אאא", "alpha", "noun", 1)
	pool := []Word{answer}
	for i := 2; i < 12; i++ {
		w := word(int64(i), fakeHebrew(i), fakeEnglish(i), "noun", 1)
		if i%2 == 0 {
			w.AudioID = "clip"
		}
		pool = append(pool, w)
	}
	got, err := Pick(answer, pool, 3, Need{Audio: true}, nil, false, rand.New(rand.NewPCG(1, 1)))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range got {
		if w.AudioID == "" {
			t.Fatalf("%+v has no clip", w)
		}
	}
	if _, err := Pick(answer, pool, 6, Need{Audio: true}, nil, false, rand.New(rand.NewPCG(1, 1))); err != ErrNotEnoughWords {
		t.Fatalf("err = %v, want ErrNotEnoughWords with only five clips", err)
	}
}

func TestPickIsRepeatable(t *testing.T) {
	answer := word(1, "אאא", "alpha", "noun", 1)
	pool := []Word{answer}
	for i := 2; i < 60; i++ {
		pool = append(pool, word(int64(i), fakeHebrew(i), fakeEnglish(i), "noun", 1))
	}
	a, _ := Pick(answer, pool, 3, Need{}, nil, false, rand.New(rand.NewPCG(5, 5)))
	b, _ := Pick(answer, pool, 3, Need{}, nil, false, rand.New(rand.NewPCG(5, 5)))
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("same seed, different choice: %v vs %v", a, b)
	}
}

// ac-1: across 10,000 built items no distractor equals the answer or shares its gloss, none
// is repeated inside an item, and every distractor is in the answer's band and part of speech
// when that group has enough words.
func TestBuiltItemsAreFair(t *testing.T) {
	bands := fixedBands{}
	e, store, _ := openEngine(t, WithBands(bands))
	lex := fillLexicon(t, store, 120, nil)
	// Bands 0..2, noun/verb: 20 words in each group, so every group has enough words.
	for i, id := range lex.ids {
		bands[id] = i % 3
		w := lex.words[id]
		w.Band = i % 3
		lex.words[id] = w
	}
	// Twenty near-synonyms: a twin of a word, same band and part of speech, gloss shares a word.
	// They are in the answer's own group, so a bad filter would pick them.
	for k := 0; k < 20; k++ {
		base := lex.words[lex.ids[k*6]]
		id := addWord(t, store, fakeHebrew(1000+k), base.Gloss+", twin", base.POS, true)
		bands[id] = base.Band
		lex.ids = append(lex.ids, id)
		lex.words[id] = Word{ID: id, Text: fakeHebrew(1000 + k), Gloss: base.Gloss + ", twin", POS: base.POS, Band: base.Band}
	}
	user := addUser(t, store, "ac1")
	// A past confusion for some words, so a partner is sometimes preferred.
	for k := 0; k < 30; k++ {
		a, b := lex.words[lex.ids[k*3]], lex.words[lex.ids[k*3+3]]
		x, y := a.Text, b.Text
		if x > y {
			x, y = y, x
		}
		mustf(t, store.InsertConfusionMiss(context.Background(), user, fsrs.MissOther, x+"/"+y, a.ID, time.Date(2026, 2, 27, 9, 0, 0, 0, time.UTC)), "confusion")
	}

	kinds := []Kind{ReadPick, ListenPick, WordMatch, AudioMatch}
	ctx := context.Background()
	const items = 10000
	for n := 0; n < items; n++ {
		anchor := lex.words[lex.ids[n%len(lex.ids)]]
		kind := kinds[n%len(kinds)]
		b, err := e.build(ctx, user, anchor.ID, kind)
		if err != nil {
			t.Fatalf("item %d (%s of %q): %v", n, kind, anchor.Text, err)
		}
		bandPosPool := 0
		for _, w := range lex.words {
			if w.ID != anchor.ID && w.Band == anchor.Band && w.POS == anchor.POS && !GlossesOverlap(w.Gloss, anchor.Gloss) {
				bandPosPool++
			}
		}
		seenID := map[int64]bool{}
		seenText := map[string]bool{}
		for i, w := range b.p.words {
			if seenID[w.ID] || seenText[w.Text] {
				t.Fatalf("item %d repeats %+v", n, w)
			}
			seenID[w.ID], seenText[w.Text] = true, true
			for _, o := range b.p.words[:i] {
				if GlossesOverlap(o.Gloss, w.Gloss) {
					t.Fatalf("item %d: options %q and %q share a gloss", n, o.Gloss, w.Gloss)
				}
			}
			if w.ID == anchor.ID {
				continue
			}
			if w.Text == anchor.Text || GlossesOverlap(w.Gloss, anchor.Gloss) {
				t.Fatalf("item %d: distractor %+v equals or is a synonym of the answer %+v", n, w, anchor)
			}
			if bandPosPool >= len(b.p.words)-1 && (w.Band != anchor.Band || w.POS != anchor.POS) {
				t.Fatalf("item %d: distractor %+v is outside the band and part of speech of %+v, which has %d fair words", n, w, anchor, bandPosPool)
			}
		}
		if !seenID[anchor.ID] {
			t.Fatalf("item %d does not contain the answer", n)
		}
	}
}

// A recent confusion about a word makes its partner an option, whichever group it is in the
// pool, provided it is fair.
func TestBuiltItemPrefersTheConfusionPartner(t *testing.T) {
	e, store, clk := openEngine(t)
	lex := fillLexicon(t, store, 60, nil)
	user := addUser(t, store, "partner")
	asked, partner := lex.words[lex.ids[10]], lex.words[lex.ids[20]] // both nouns
	x, y := asked.Text, partner.Text
	if x > y {
		x, y = y, x
	}
	mustf(t, store.InsertConfusionMiss(context.Background(), user, fsrs.MissOther, x+"/"+y, asked.ID, clk.Now().Add(-24*time.Hour)), "confusion")
	for n := 0; n < 50; n++ {
		b, err := e.build(context.Background(), user, asked.ID, ReadPick)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, w := range b.p.words {
			found = found || w.ID == partner.ID
		}
		if !found {
			t.Fatalf("build %d: partner %q not among the options %+v", n, partner.Text, b.p.words)
		}
	}
	// A confusion from 40 days ago has aged out of the 30-day window.
	other := addUser(t, store, "old")
	mustf(t, store.InsertConfusionMiss(context.Background(), other, fsrs.MissOther, x+"/"+y, asked.ID, clk.Now().Add(-40*24*time.Hour)), "old confusion")
	hits := 0
	for n := 0; n < 50; n++ {
		b, err := e.build(context.Background(), other, asked.ID, ReadPick)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range b.p.words {
			if w.ID == partner.ID {
				hits++
			}
		}
	}
	if hits > 25 {
		t.Fatalf("an aged-out confusion still pulled its partner in %d of 50 builds", hits)
	}
}

func TestPartnerTestLetterPair(t *testing.T) {
	anchor := Word{ID: 1, Text: "חלון"}
	test := partnerTest(anchor, []db.WordConfusion{{Kind: fsrs.MissLetterPair, Key: "ם/ן"}})
	if test == nil || !test(Word{ID: 2, Text: "חלום"}) {
		t.Fatal("a word one letter off in the confused pair is a partner")
	}
	if test(Word{ID: 3, Text: "חלוק"}) {
		t.Fatal("a word one letter off in another pair is not")
	}
	// The recorder stores the pair of two picked words with final letters folded.
	folded := partnerTest(anchor, []db.WordConfusion{{Kind: fsrs.MissLetterPair, Key: "מ/נ"}})
	if folded == nil || !folded(Word{ID: 2, Text: "חלום"}) {
		t.Fatal("the folded key matches too")
	}
	if partnerTest(anchor, nil) != nil {
		t.Fatal("no confusions, no test")
	}
}
