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
// Verified is true once a person has checked the row.
// Gender is m, f, both, or empty when the form does not mark gender.
type fixtureItem struct {
	ID       string `json:"id"`
	TextNFC  string `json:"text_nfc"`
	Band     int    `json:"band"`
	GlossEN  string `json:"gloss_en"`
	Translit string `json:"translit"`
	Gender   string `json:"gender"`
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
	return Card{
		ID:       it.ID,
		TextNFC:  it.TextNFC,
		GlossEN:  it.GlossEN,
		Translit: it.Translit,
		Gender:   it.Gender,
	}
}

// Lookup is the fixture row for an item id.
// The band is 0 through 4. AudioURL is empty; the handler fills a clip
// only for the card Next already presented.
func Lookup(id string) (Card, int, bool) {
	items, err := catalog()
	if err != nil {
		return Card{}, 0, false
	}
	for _, it := range items {
		if it.ID == id {
			return it.card(), it.Band, true
		}
	}
	return Card{}, 0, false
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
