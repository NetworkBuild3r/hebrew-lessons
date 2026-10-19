package placement

import "math"

// Rank widths of bands 1–4 in the frequency list. Band 0 has no width;
// its Good and Easy answers are counted one by one.
var bandRankSize = [bands]float64{0, 500, 500, 1000, 2000}

// Score applies the struggle rule and returns the estimate of known words.
// SuggestedStartPosition is 0. Finish fills it once the learner's units and
// known set are known.
//
// A band trips once it has at least 10 ratings and (Again + Hard) / Rated
// is at least 0.60. Exactly 0.60 trips. The run then stops, and every band
// above the tripped band is left unrated. Level is the highest band whose
// dealable words were all rated without tripping, or −1.
//
// StopReason is band_stop when a band tripped, cap when 80 cards were rated
// and words remain, exhausted when nothing dealable remains, and empty when
// the learner stopped first.
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
	tripped := false
	remain := false
	for band := 0; band < bands; band++ {
		out.Bands[band] = counted[band].score(band)
		dealable := tallies[band].Dealable
		if dealable < counted[band].rated {
			dealable = counted[band].rated
		}
		if !tripped && dealable > counted[band].rated {
			remain = true
		}
		if !tripped && struggling(counted[band]) {
			tripped = true
		}
		if band == 0 {
			band0Known = counted[band].good + counted[band].easy
			continue
		}
		sum += out.Bands[band].passRate() * bandRankSize[band]
	}
	out.EstimatedKnownWords = int(math.Round(sum)) + band0Known

	switch {
	case tripped:
		out.StopReason = stopBand
	case remain && cardsRated >= cardCap:
		out.StopReason = stopCap
	case !remain:
		out.StopReason = stopExhausted
	}

	for band := 0; band < bands; band++ {
		dealable := tallies[band].Dealable
		if dealable < counted[band].rated {
			dealable = counted[band].rated
		}
		if dealable == 0 {
			continue
		}
		if counted[band].rated < dealable || struggling(counted[band]) {
			break
		}
		out.Level = band
	}
	return out
}

// passRate is the Good-or-better share of a band. It is 0 when nothing was rated.
func (b BandScore) passRate() float64 {
	if b.Rated == 0 {
		return 0
	}
	return float64(b.Good+b.Easy) / float64(b.Rated)
}

type bandCount struct {
	rated, again, hard, good, easy int
}

func (c *bandCount) add(rating int) {
	switch rating {
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
	rate := 0.0
	if c.rated > 0 {
		rate = float64(c.again+c.hard) / float64(c.rated)
	}
	return BandScore{
		Band:         band,
		Rated:        c.rated,
		Again:        c.again,
		Hard:         c.hard,
		Good:         c.good,
		Easy:         c.easy,
		StruggleRate: rate,
	}
}

// struggling reports the stop rule: at least 10 ratings, and Again+Hard
// is at least 60% of them. The comparison is integer, so 6 of 10 trips
// and 5 of 10 does not.
func struggling(c bandCount) bool {
	if c.rated < stopAt {
		return false
	}
	return (c.again+c.hard)*5 >= c.rated*3
}
