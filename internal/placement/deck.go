package placement

import (
	"math/rand/v2"
	"sort"
)

// deck returns the fixture in band order, each band shuffled by seed.
// Items are sorted by id before the shuffle, so catalog order does not matter.
// The same seed always returns the same order.
func deck(seed int64, items []fixtureItem) []fixtureItem {
	groups := make([][]fixtureItem, bands)
	for _, it := range items {
		if it.Band < 0 || it.Band >= bands {
			continue
		}
		groups[it.Band] = append(groups[it.Band], it)
	}
	var out []fixtureItem
	for band := 0; band < bands; band++ {
		sort.Slice(groups[band], func(i, j int) bool { return groups[band][i].ID < groups[band][j].ID })
		r := rand.New(rand.NewPCG(uint64(seed), uint64(band)+1))
		r.Shuffle(len(groups[band]), func(i, j int) {
			groups[band][i], groups[band][j] = groups[band][j], groups[band][i]
		})
		out = append(out, groups[band]...)
	}
	return out
}
