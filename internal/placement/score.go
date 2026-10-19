package placement

import "math"

// Rank widths of bands 1–4 in the frequency list. Band 0 has no width;
// its Good and Easy answers are counted one by one.
var bandRankSize = [bands]float64{0, 500, 500, 1000, 2000}

// Score applies the Good-or-better rule and returns the estimate of known words.
// SuggestedStartPosition is 0. Finish fills it once the learner's units and
// known set are known.
//
// The last answer for an item id wins. An item with no answer is not shown.
// A pseudo-word, and a real-looking item whose band is nil, counts only toward
// the false-alarm rate. Good or Easy on a pseudo-word is a false alarm. Hard
// is recorded on the answer and is not a false alarm. A band outside 0–4 is
// ignored. A band with no rated real words has a Good-or-better rate of 0 and
// does not set the level. With no rated pseudo-words, the false-alarm rate is
// 0 and the run is reliable.
func Score(answers []Answer, items []ScoreItem) Result {
	rating := make(map[string]int, len(answers))
	for _, answer := range answers {
		rating[answer.ItemID] = answer.Rating
	}

	var counted [bands]bandCount
	var pseudoShown, falseAlarms int
	for _, it := range items {
		grade, ok := rating[it.ID]
		if !ok {
			continue
		}
		if it.Pseudo || it.Band == nil {
			pseudoShown++
			if grade == Good || grade == Easy {
				falseAlarms++
			}
			continue
		}
		band := *it.Band
		if band < 0 || band >= bands {
			continue
		}
		counted[band].add(grade)
	}

	f := 0.0
	if pseudoShown > 0 {
		f = float64(falseAlarms) / float64(pseudoShown)
	}
	out := Result{
		FalseAlarmRate: f,
		Reliable:       f <= maxFalseAlarm,
		Level:          -1,
		Bands:          make([]BandScore, bands),
	}
	var sum float64
	band0Known := 0
	for band := 0; band < bands; band++ {
		score := counted[band].score(band)
		out.Bands[band] = score
		if counted[band].shown > 0 && passed(counted[band].good, counted[band].easy, counted[band].shown) && band > out.Level {
			out.Level = band
		}
		if band == 0 {
			band0Known = counted[band].good + counted[band].easy
			continue
		}
		sum += score.GoodPlusRate * bandRankSize[band]
	}
	out.EstimatedKnownWords = int(math.Round(sum)) + band0Known
	return out
}

type bandCount struct {
	shown, again, hard, good, easy, dont int
}

func (c *bandCount) add(rating int) {
	c.shown++
	switch rating {
	case DontKnow:
		c.dont++
	case Again:
		c.again++
	case Hard:
		c.hard++
	case Good:
		c.good++
	case Easy:
		c.easy++
	default:
		c.shown--
	}
}

func (c bandCount) score(band int) BandScore {
	return BandScore{
		Band:         band,
		Shown:        c.shown,
		Again:        c.again,
		Hard:         c.hard,
		Good:         c.good,
		Easy:         c.easy,
		DontKnow:     c.dont,
		GoodPlusRate: rate(c.good, c.easy, c.shown),
	}
}

func rate(good, easy, shown int) float64 {
	if shown == 0 {
		return 0
	}
	return float64(good+easy) / float64(shown)
}

// passed reports a Good-or-better rate of at least passAt.
// The comparison is integer so exactly half the words passes.
func passed(good, easy, shown int) bool {
	if shown == 0 {
		return false
	}
	return (good+easy)*2 >= shown
}
