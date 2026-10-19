package placement

import "math"

// Rank widths of bands 1–4 in the frequency list. Band 0 has no width;
// its yes answers are counted one by one.
var bandRankSize = [bands]float64{0, 500, 500, 1000, 2000}

// Score applies the corrected-score rule and returns the estimate of known words.
// SuggestedStartPosition is 0. The service fills it once the learner's units
// and known set are known.
//
// The last answer for an item id wins. An item with no answer counts as a no.
// A pseudo-word, and a real-looking item whose band is nil, counts only toward
// the false-alarm rate. A band outside 0–4 is ignored. A band with no items
// has h 0 and does not pass. With no pseudo-words, f is 0 and the run is reliable.
func Score(answers []Answer, items []ScoreItem) Result {
	said := make(map[string]bool, len(answers))
	for _, answer := range answers {
		said[answer.ItemID] = answer.Yes
	}

	yes := make([]int, bands)
	n := make([]int, bands)
	var pseudoYes, pseudoN, band0Yes int
	for _, it := range items {
		know := said[it.ID]
		if it.Pseudo || it.Band == nil {
			pseudoN++
			if know {
				pseudoYes++
			}
			continue
		}
		band := *it.Band
		if band < 0 || band >= bands {
			continue
		}
		n[band]++
		if know {
			yes[band]++
			if band == 0 {
				band0Yes++
			}
		}
	}

	f := 0.0
	blocked := false
	if pseudoN > 0 {
		f = float64(pseudoYes) / float64(pseudoN)
		blocked = pseudoYes == pseudoN
	}
	out := Result{
		FalseAlarmRate: f,
		Reliable:       f <= maxFalseAlarm,
		Level:          -1,
		Bands:          make([]BandScore, bands),
	}
	var sum float64
	for band := 0; band < bands; band++ {
		h := 0.0
		if n[band] > 0 {
			h = float64(yes[band]) / float64(n[band])
		}
		corrected, passed := correct(h, f, blocked)
		out.Bands[band] = BandScore{
			Band:      band,
			H:         h,
			F:         f,
			Corrected: corrected,
			Passed:    passed,
		}
		if passed && band > out.Level {
			out.Level = band
		}
		if band >= 1 {
			sum += clamp01(corrected) * bandRankSize[band]
		}
	}
	out.EstimatedKnownWords = int(math.Round(sum)) + band0Yes
	return out
}

// clamp01 limits a corrected score to the range used as a fraction of a band.
func clamp01(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
