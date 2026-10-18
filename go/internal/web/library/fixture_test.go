package library

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os/exec"
	"slices"
	"testing"
)

func TestEmptyCursorMatchesNodeOrder(t *testing.T) {
	items, next, err := (Demo{}).List(context.Background(), DemoUserID, Filter{}, "", PageSize)
	if err != nil {
		t.Fatal(err)
	}
	if next != "" {
		t.Fatalf("next cursor %q, want empty under the page size", next)
	}
	got := make([]string, len(items))
	for i, item := range items {
		got[i] = item.Word
	}
	keys := make([]string, len(demoItems))
	for i, item := range demoItems {
		keys[i] = item.Word
	}
	want := nodeSort(t, keys)
	if !slices.Equal(got, want) {
		t.Fatalf("order\n got %q\nwant %q", got, want)
	}
	if !slices.IsSorted(got) {
		t.Fatalf("not sorted: %q", got)
	}
}

func TestCursorIsOpaqueKeyset(t *testing.T) {
	first, cursor, err := (Demo{}).List(context.Background(), 7, Filter{}, "", 4)
	if err != nil {
		t.Fatal(err)
	}
	if cursor == "" || len(first) != 4 {
		t.Fatalf("page %d cursor %q", len(first), cursor)
	}
	raw, err := decodeCursor(cursor)
	if err != nil || raw != first[len(first)-1].Word {
		t.Fatalf("cursor word %q err %v, last %q", raw, err, first[len(first)-1].Word)
	}
	second, next, err := (Demo{}).List(context.Background(), 7, Filter{}, cursor, 4)
	if err != nil {
		t.Fatal(err)
	}
	if next == "" || len(second) != 4 {
		t.Fatalf("second page %d next %q", len(second), next)
	}
	seen := map[string]bool{}
	for _, item := range first {
		seen[item.Word] = true
	}
	for _, item := range second {
		if seen[item.Word] {
			t.Fatalf("overlap %q", item.Word)
		}
		if item.Word <= first[len(first)-1].Word {
			t.Fatalf("%q is not after the cursor", item.Word)
		}
	}
	if _, _, err := (Demo{}).List(context.Background(), 1, Filter{}, "%%%", 4); err != ErrInvalidCursor {
		t.Fatalf("bad cursor err %v", err)
	}
	enc, err := json.Marshal(cursorBody{Ord: []string{"nope"}, Word: "other"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCursor(base64.RawURLEncoding.EncodeToString(enc)); err != ErrInvalidCursor {
		t.Fatal("tuple that disagrees with the word was accepted")
	}
}

func TestFiltersMatchTheLibraryRules(t *testing.T) {
	hifil, _, err := (Demo{}).List(context.Background(), 1, Filter{Binyan: "hifil"}, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(hifil) != 1 || hifil[0].Word != "הִסְבִּיר" || hifil[0].Binyan != "hifil" {
		t.Fatalf("hifil: %+v", hifil)
	}
	weak, _, err := (Demo{}).List(context.Background(), 1, Filter{Weak: "1"}, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(weak) != 2 {
		t.Fatalf("weak count %d", len(weak))
	}
	paused, _, err := (Demo{}).List(context.Background(), 1, Filter{Suspended: "1"}, "", 50)
	if err != nil || len(paused) != 1 || !paused[0].Suspended || paused[0].Word != "חָבֵר" {
		t.Fatalf("paused %+v err %v", paused, err)
	}
	active, _, err := (Demo{}).List(context.Background(), 1, Filter{Suspended: "0"}, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range active {
		if item.Suspended {
			t.Fatalf("active included %q", item.Word)
		}
	}
	past, _, err := (Demo{}).List(context.Background(), 1, Filter{Tense: "past"}, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, item := range past {
		found[item.Word] = true
	}
	if !found["כָּתַב"] || !found["הִסְבִּיר"] {
		t.Fatalf("past: %v", found)
	}
	peace, _, err := (Demo{}).List(context.Background(), 1, Filter{Q: "peace"}, "", 50)
	if err != nil || len(peace) != 1 || peace[0].Word != "שָׁלוֹם" {
		t.Fatalf("search %+v %v", peace, err)
	}
}

func nodeSort(t *testing.T, words []string) []string {
	t.Helper()
	payload, err := json.Marshal(words)
	if err != nil {
		t.Fatal(err)
	}
	script := `const w = ` + string(payload) + `; w.sort((a,b)=>a<b?-1:a>b?1:0); process.stdout.write(JSON.stringify(w));`
	out, err := exec.Command("node", "-e", script).Output()
	if err != nil {
		t.Fatal(err)
	}
	var sorted []string
	if err := json.Unmarshal(out, &sorted); err != nil {
		t.Fatal(err)
	}
	return sorted
}
