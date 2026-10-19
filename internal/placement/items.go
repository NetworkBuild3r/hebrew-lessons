package placement

import (
	_ "embed"
	"encoding/json"
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

func (it fixtureItem) card() Card {
	return Card{ID: it.ID, TextNFC: it.TextNFC, GlossEN: it.GlossEN}
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
