package placement

import "math"

// Rank widths of bands 1–4 in the frequency list. Band 0 has no width;
// its Good and Easy answers are counted one by one.
var bandRankSize = [bands]float64{0, 500, 500, 1000, 2000}

// Score applies the block rule and returns the estimate of known words.
// SuggestedStartPosition is 0. Finish fills it once the learner's units and
// known set are known. Finish also sets cap_time from the clock; Score does
// not see the clock, so it leaves that reason empty.
//
// A band's block is min(BlockSize, Dealable) words. Once that block is
// rated, (Good + Easy) / Rated below StopGoodPlusMin stops the run, and
// every band above it is left unrated. Exactly StopGoodPlusMin does not
// stop. Level is the highest block that was fully rated without stopping,
// or −1.
//
// StopReason is band_stop when a block stopped the run, cap_cards when
// MaxCards were rated and words remain, exhausted when nothing dealable
// remains, and empty when the learner stopped first. DontKnow counts in
// Rated and in DontKnow, not in Again.
func Score(tallies [bands]BandTally) Result {
	var counted [bands]bandCount
	var cardsRated int
	for band := 0; band < bands; band++ {
		for _, rating := range tallies[band].Ratings {
			counted[band].add(rating)
		}
		cardsRated += counted[band].rated
	}

	out := Result{
		Level:      -1,
		CardsRated: cardsRated,
		Bands:      make([]BandScore, bands),
	}
	var sum float64
	band0Known := 0
	stopped := false
	remain := false
	for band := 0; band < bands; band++ {
		out.Bands[band] = counted[band].score(band)
		block := blockLen(tallies[band].Dealable, counted[band].rated)
		if !stopped && counted[band].rated < block {
			remain = true
		}
		if !stopped && block > 0 && counted[band].rated >= block && belowGoodPlus(counted[band]) {
			stopped = true
		}
		if band == 0 {
			band0Known = counted[band].good + counted[band].easy
			continue
		}
		sum += out.Bands[band].GoodPlusRate * bandRankSize[band]
	}
	out.EstimatedKnownWords = int(math.Round(sum)) + band0Known

	switch {
	case stopped:
		out.StopReason = stopBand
	case remain && cardsRated >= MaxCards:
		out.StopReason = stopCards
	case !remain:
		out.StopReason = stopExhausted
	}

	for band := 0; band < bands; band++ {
		block := blockLen(tallies[band].Dealable, counted[band].rated)
		if block == 0 {
			continue
		}
		if counted[band].rated < block || belowGoodPlus(counted[band]) {
			break
		}
		out.Level = band
	}
	return out
}

// blockLen is the block the learner has to finish: BlockSize words, or
// fewer when the band does not have that many dealable words.
func blockLen(dealable, rated int) int {
	if dealable < rated {
		dealable = rated
	}
	if dealable > BlockSize {
		return BlockSize
	}
	return dealable
}

// belowGoodPlus reports (Good + Easy) / Rated < StopGoodPlusMin.
// A block with no ratings does not stop. Exactly StopGoodPlusMin does not.
func belowGoodPlus(c bandCount) bool {
	if c.rated == 0 {
		return false
	}
	return float64(c.good+c.easy) < StopGoodPlusMin*float64(c.rated)
}

type bandCount struct {
	rated, again, hard, good, easy, dontKnow int
}

func (c *bandCount) add(rating int) {
	switch rating {
	case DontKnow:
		c.dontKnow++
	case Again:
		c.again++
	case Hard:
		c.hard++
	case Good:
		c.good++
	case Easy:
		c.easy++
	default:
		return
	}
	c.rated++
}

func (c bandCount) score(band int) BandScore {
	return BandScore{
		Band:         band,
		Rated:        c.rated,
		Again:        c.again,
		Hard:         c.hard,
		Good:         c.good,
		Easy:         c.easy,
		DontKnow:     c.dontKnow,
		GoodPlusRate: goodPlusRate(c),
	}
}

func goodPlusRate(c bandCount) float64 {
	if c.rated == 0 {
		return 0
	}
	return float64(c.good+c.easy) / float64(c.rated)
}
