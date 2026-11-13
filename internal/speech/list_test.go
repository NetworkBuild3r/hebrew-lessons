package speech

import "testing"

func TestListCoversTheSubtitleCorpus(t *testing.T) {
	n, err := Len()
	if err != nil {
		t.Fatal(err)
	}
	if n != 5000 {
		t.Fatalf("lemmas = %d, want 5000", n)
	}
	first, err := Coverage(30)
	if err != nil {
		t.Fatal(err)
	}
	if first < 0.47 || first > 0.49 {
		t.Fatalf("first 30 cover %.3f, want about 0.48", first)
	}
	all, err := Coverage(5000)
	if err != nil {
		t.Fatal(err)
	}
	if all < 0.90 || all > 0.92 {
		t.Fatalf("5000 cover %.3f, want about 0.91", all)
	}
}

func TestSampleLessonStaysOnTheList(t *testing.T) {
	off, err := Allows(Sample().Graded())
	if err != nil {
		t.Fatal(err)
	}
	if len(off) != 0 {
		t.Fatalf("off the list: %v", off)
	}
	off, err = Allows([]string{"פִּלְפֵּל"})
	if err != nil {
		t.Fatal(err)
	}
	if len(off) != 1 {
		t.Fatalf("a word that is not on the list was allowed: %v", off)
	}
}

func TestTypeStepAcceptsOnlyTheListedWord(t *testing.T) {
	ok, err := GradeType("בְּ")
	if err != nil || !ok {
		t.Fatalf("pointed in: ok=%v err=%v", ok, err)
	}
	ok, err = GradeType("ב")
	if err != nil || !ok {
		t.Fatalf("unpointed in: ok=%v err=%v", ok, err)
	}
	ok, err = GradeType("הוּא")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("a different word on the list was marked right")
	}
	ok, err = GradeType("פִּלְפֵּל")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("a word off the list was marked right")
	}
}

func TestMapShowsHowFarTheCommonWordsGo(t *testing.T) {
	he, ok, err := Lookup("הוא")
	if err != nil || !ok {
		t.Fatalf("הוא lookup ok=%v err=%v", ok, err)
	}
	if he.TimesLabel() != "121" {
		t.Fatalf("הוא times = %s, want 121 in every 1,000", he.TimesLabel())
	}
	if he.Through < 0.11 || he.Through > 0.13 {
		t.Fatalf("הוא through = %.3f, want about 0.12", he.Through)
	}
	steps, err := Map()
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) == 0 || steps[0].Words != 30 || steps[0].Share < 0.47 || steps[0].Share > 0.49 {
		t.Fatalf("first rung = %+v, want 30 words at about 48%%", steps)
	}
	last := steps[len(steps)-1]
	if last.Words != 5000 || last.Share >= FollowMin {
		t.Fatalf("the list reaches %+v, and 95%% is past it", last)
	}
}

func TestNextSliceWalksTheListAndKeepsAKnownSentence(t *testing.T) {
	rows, _, err := entries()
	if err != nil {
		t.Fatal(err)
	}
	first := Sample()
	next := Next()
	if len(first.Words) != 6 || len(next.Words) != 6 {
		t.Fatalf("slices = %d and %d, want 6 and 6", len(first.Words), len(next.Words))
	}
	for i, w := range append(first.Graded(), next.Graded()...) {
		e, ok, err := Lookup(w)
		if err != nil || !ok {
			t.Fatalf("word %d %s: ok=%v err=%v", i, w, ok, err)
		}
		if e.Lemma != rows[i].Lemma || e.Rank != i+1 {
			t.Fatalf("word %d = %s rank %d, want %s rank %d", i, e.Lemma, e.Rank, rows[i].Lemma, i+1)
		}
	}
	kept, err := SentenceKept(next.Kept, next.Known, next.NewLemma)
	if err != nil || !kept {
		t.Fatalf("kept sentence: kept=%v err=%v", kept, err)
	}
	short, err := SentenceKept("הוּא הָיָה", next.Known, "היה")
	if err != nil {
		t.Fatal(err)
	}
	if short {
		t.Fatal("a short line with one new word was kept under 95 percent")
	}
	off, err := SentenceKept(next.Kept+" פִּלְפֵּל", next.Known, next.NewLemma)
	if err != nil {
		t.Fatal(err)
	}
	if off {
		t.Fatal("a word off the list was kept")
	}
}
