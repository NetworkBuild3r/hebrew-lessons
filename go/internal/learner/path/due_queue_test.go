package path

import "testing"

func TestSelectDueInterleavesLessons(t *testing.T) {
	items := []dueItem{
		{word: "a1", direction: "he_en", lesson: "alpha", dueMs: 1, isNew: true},
		{word: "a2", direction: "he_en", lesson: "alpha", dueMs: 2, isNew: true},
		{word: "a3", direction: "he_en", lesson: "alpha", dueMs: 3, isNew: true},
		{word: "b1", direction: "he_en", lesson: "beta", dueMs: 4, isNew: true},
	}
	got := selectDue(items)
	var words []string
	for _, item := range got {
		words = append(words, item.word)
	}
	if stringsJoin(words) != "a1,a2,b1,a3" {
		t.Fatalf("order %v", words)
	}
}

func TestSelectDueKeepsSecondDirectionOfANewWord(t *testing.T) {
	var items []dueItem
	for i := 0; i < reviewNewCap; i++ {
		items = append(items, dueItem{
			word: "n", direction: "he_en", lesson: "", dueMs: int64(i + 1), isNew: true,
		})
		items[i].word = string(rune('a' + i))
	}
	items = append(items, dueItem{
		word: "a", direction: "en_he", lesson: "", dueMs: 100, isNew: true,
	})
	got := selectDue(items)
	if len(got) != reviewNewCap+1 {
		t.Fatalf("len %d", len(got))
	}
	last := got[len(got)-1]
	if last.word != "a" || last.direction != "en_he" {
		t.Fatalf("last %+v", last)
	}
}
