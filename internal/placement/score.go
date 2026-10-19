package placement

import "math"

// Rank widths of bands 1–4 in the frequency list. Band 0 has no width;
// its Good and Easy answers are counted one by one.
var bandRankSize = [bands]float64{0, 500, 500, 1000, 2000}

// Score applies the band rule and returns the estimate of known words.
// SuggestedStartPosition is 0. Finish fills it once the learner's units and
// known set are known.
//
// A band deals at most BandSize words. Once MinRatedForStop cards in that
// band are rated, an Again share of StopAgainMin or more stops the run.
// "I don't know" counts as Again. Hard does not. Exactly StopAgainMin stops.
// Fewer than MinRatedForStop ratings never stop, even when every one is
// Again. Level is the highest band whose block was fully rated without
// stopping, or −1.
//
// StopReason is band_stop when a band stopped the run, cap_cards when
// MaxCards were rated and words remain, exhausted when nothing dealable
// remains, and empty when the learner stopped first. DontKnow counts in
// Rated and in DontKnow. It is not added to the Again field.
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
		if !stopped && tripped(counted[band]) {
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
		if counted[band].rated < block || tripped(counted[band]) {
			break
		}
		out.Level = band
	}
	return out
}

// blockLen is the block the learner has to finish: BandSize words, or
// fewer when the band does not have that many dealable words.
func blockLen(dealable, rated int) int {
	if dealable < rated {
		dealable = rated
	}
	if dealable > BandSize {
		return BandSize
	}
	return dealable
}

// tripped reports that the band has enough ratings and its Again share,
// counting "I don't know" as Again, is at least StopAgainMin.
func tripped(c bandCount) bool {
	if c.rated < MinRatedForStop {
		return false
	}
	return float64(c.again+c.dontKnow) >= StopAgainMin*float64(c.rated)
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
