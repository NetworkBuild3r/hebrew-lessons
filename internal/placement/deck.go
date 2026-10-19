package placement

import (
	"math/rand/v2"
	"sort"
	"time"
)

const (
	// realsPerBlock is how many real words a band deals, at most.
	realsPerBlock = 16
	// pseudosPerBlock is how many invented words are mixed into each band.
	pseudosPerBlock = 4
	// cardCap is the most cards one run deals.
	cardCap = 100
	// timeCap is how long a run may deal cards after it started.
	timeCap = 5 * time.Minute
)

// blocks deals one interleaved block per band.
// Real words are sorted by id and capped at realsPerBlock. Invented words are
// sorted by id, shuffled by seed, then taken four at a time without reuse.
// Each block is shuffled with a stream derived from the seed and the band, so
// the same seed always deals the same cards.
func blocks(seed int64, items []fixtureItem) [][]fixtureItem {
	reals := make([][]fixtureItem, bands)
	var pseudos []fixtureItem
	for _, it := range items {
		if it.Pseudo || it.Band == nil {
			pseudos = append(pseudos, it)
			continue
		}
		band := *it.Band
		if band < 0 || band >= bands {
			continue
		}
		reals[band] = append(reals[band], it)
	}
	sortItems(pseudos)
	for band := range reals {
		sortItems(reals[band])
		if len(reals[band]) > realsPerBlock {
			reals[band] = reals[band][:realsPerBlock]
		}
	}
	shuffle(uint64(seed), uint64(seed), pseudos)

	out := make([][]fixtureItem, bands)
	at := 0
	for band := 0; band < bands; band++ {
		block := make([]fixtureItem, 0, len(reals[band])+pseudosPerBlock)
		block = append(block, reals[band]...)
		n := pseudosPerBlock
		if at >= len(pseudos) {
			n = 0
		} else if at+n > len(pseudos) {
			n = len(pseudos) - at
		}
		if n > 0 {
			block = append(block, pseudos[at:at+n]...)
			at += n
		}
		shuffle(uint64(seed), uint64(band)+1, block)
		out[band] = block
	}
	return out
}

func sortItems(items []fixtureItem) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}

func shuffle(seed, stream uint64, items []fixtureItem) {
	r := rand.New(rand.NewPCG(seed, stream))
	r.Shuffle(len(items), func(i, j int) {
		items[i], items[j] = items[j], items[i]
	})
}

// blockProgress reports whether every card in the block has a rating, and how
// the real words in it were rated. An unrated card keeps complete false.
func blockProgress(block []fixtureItem, ratingOf func(id string) (int, bool)) (complete bool, realShown, goodPlus int) {
	complete = true
	for _, it := range block {
		rating, ok := ratingOf(it.ID)
		if !ok {
			complete = false
			continue
		}
		if it.Pseudo || it.Band == nil {
			continue
		}
		realShown++
		if rating == Good || rating == Easy {
			goodPlus++
		}
	}
	return complete, realShown, goodPlus
}
