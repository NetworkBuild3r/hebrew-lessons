package mcp

import "testing"

func TestAppendFutureCellConcepts(t *testing.T) {
	got := appendFutureCellConcepts(nil)
	if len(got) != len(futureCellConcepts) {
		t.Fatalf("len %d", len(got))
	}
	key, _ := got[0].Get("key")
	sortKey, _ := got[0].Get("sort")
	he, _ := got[0].Get("label_he")
	colour, _ := got[0].Get("colour")
	if key.Str != "cell:future.1s" || !sortKey.IsInt() {
		t.Fatalf("first %#v sort %#v", key, sortKey)
	}
	n, _ := sortKey.Float()
	if n != 1 || he.Str != "אני" || colour.Kind != KindNull {
		t.Fatalf("sort %v he %q colour %v", n, he.Str, colour.Kind)
	}

	seeded := []Value{grammarConcept("binyan:paal", "binyan", "paal", String("פָּעַל"), Null(), 32)}
	withSeed := appendFutureCellConcepts(seeded)
	if len(withSeed) != 1+len(futureCellConcepts) {
		t.Fatalf("len %d", len(withSeed))
	}
	last := withSeed[len(withSeed)-1]
	lastKey, _ := last.Get("key")
	lastSort, _ := last.Get("sort")
	ln, _ := lastSort.Float()
	if lastKey.Str != "cell:future.3p" || ln != 32+float64(len(futureCellConcepts)) {
		t.Fatalf("last %s sort %v", lastKey.Str, ln)
	}

	again := appendFutureCellConcepts(withSeed)
	if len(again) != len(withSeed) {
		t.Fatalf("duplicate append %d", len(again))
	}
}
