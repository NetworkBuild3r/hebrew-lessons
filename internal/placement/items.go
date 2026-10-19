package placement

import (
	_ "embed"
	"encoding/json"
	"math/rand/v2"
	"strings"
	"sync"

	"golang.org/x/text/unicode/norm"
)

//go:embed items.json
var itemsJSON []byte

// fixtureItem is one row of items.json.
// Band is null for a pseudo-word. Verified stays false until a person checks the row.
type fixtureItem struct {
	ID       string `json:"id"`
	TextNFC  string `json:"text_nfc"`
	Band     *int   `json:"band"`
	Pseudo   bool   `json:"pseudo"`
	GlossEN  string `json:"gloss_en"`
	POS      string `json:"pos"`
	Verified bool   `json:"verified"`
	Source   string `json:"source"`
}

var (
	loadOnce sync.Once
	loaded   []fixtureItem
	loadErr  error
)

func catalog() ([]fixtureItem, error) {
	loadOnce.Do(func() {
		dec := json.NewDecoder(strings.NewReader(string(itemsJSON)))
		dec.DisallowUnknownFields()
		var items []fixtureItem
		if err := dec.Decode(&items); err != nil {
			loadErr = err
			return
		}
		loaded = items
	})
	return loaded, loadErr
}

// deal returns catalog order shuffled by seed.
// The same seed always returns the same order. The catalog slice is not modified.
func deal(seed int64, items []fixtureItem) []fixtureItem {
	out := make([]fixtureItem, len(items))
	copy(out, items)
	r := rand.New(rand.NewPCG(uint64(seed), uint64(seed)))
	r.Shuffle(len(out), func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	return out
}

func (it fixtureItem) public() Item {
	return Item{ID: it.ID, TextNFC: it.TextNFC}
}

// unpointed drops Hebrew points, leaving the letters.
func unpointed(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 0x0591 && r <= 0x05C7 {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isNFC(s string) bool {
	return norm.NFC.IsNormalString(s)
}
