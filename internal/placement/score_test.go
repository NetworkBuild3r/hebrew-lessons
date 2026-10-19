package placement

import "testing"

func TestScoreStopsAtExactlySixtyPercent(t *testing.T) {
	if stopAt != 10 || cardCap != 100 {
		t.Fatalf("stopAt=%d cardCap=%d", stopAt, cardCap)
	}
	trip := Score(tally(0, 16, sixOfTen(6)))
	if trip.StopReason != stopBand || trip.Level != -1 || trip.Bands[0].Rated != 10 || trip.Bands[0].StruggleRate != 0.6 {
		t.Fatalf("6/10: stop=%s level=%d band=%+v", trip.StopReason, trip.Level, trip.Bands[0])
	}
	below := Score(tally(0, 16, sixOfTen(5)))
	if below.StopReason != "" || below.Level != -1 || below.Bands[0].Rated != 10 || below.Bands[0].StruggleRate != 0.5 {
		t.Fatalf("5/10: stop=%s level=%d band=%+v", below.StopReason, below.Level, below.Bands[0])
	}
	// A finished earlier band still sets the level when the next one trips.
	var both [bands]BandTally
	both[0] = BandTally{Ratings: repeat(Good, 16), Dealable: 16}
	both[1] = BandTally{Ratings: sixOfTen(6), Dealable: 16}
	got := Score(both)
	if got.Level != 0 || got.StopReason != stopBand {
		t.Fatalf("band 1 trip: level=%d stop=%s", got.Level, got.StopReason)
	}
}

func TestScoreEstimateCapAndExhausted(t *testing.T) {
	for _, tc := range []struct {
		band int
		want int
	}{
		{1, 500},
		{2, 500},
		{3, 1000},
		{4, 2000},
	} {
		var tallies [bands]BandTally
		tallies[tc.band] = BandTally{Ratings: []int{Good}, Dealable: 1}
		got := Score(tallies)
		if got.EstimatedKnownWords != tc.want || got.Level != tc.band {
			t.Fatalf("band %d: estimate=%d level=%d", tc.band, got.EstimatedKnownWords, got.Level)
		}
		if got.SuggestedStartPosition != 0 {
			t.Fatalf("band %d set a start position of %d", tc.band, got.SuggestedStartPosition)
		}
	}

	var band0 [bands]BandTally
	band0[0] = BandTally{Ratings: []int{Easy, Easy, Again}, Dealable: 3}
	if got := Score(band0); got.EstimatedKnownWords != 2 || got.Level != 0 {
		t.Fatalf("band 0 count: %+v", got)
	}

	var third [bands]BandTally
	third[1] = BandTally{Ratings: []int{Good, Hard, Again}, Dealable: 3}
	if got := Score(third); got.EstimatedKnownWords != 167 || got.Level != 1 {
		t.Fatalf("one third of band 1: estimate=%d level=%d", got.EstimatedKnownWords, got.Level)
	}

	hard := Score(tally(0, 16, repeat(Hard, 10)))
	if hard.StopReason != "" || hard.Bands[0].StruggleRate != 0 || hard.Bands[0].Hard != 10 {
		t.Fatalf("hard does not stop: %+v", hard.Bands[0])
	}

	var capped [bands]BandTally
	for band := 0; band < 4; band++ {
		capped[band] = BandTally{Ratings: repeat(Good, 25), Dealable: 25}
	}
	capped[4] = BandTally{Dealable: 25}
	got := Score(capped)
	if got.CardsRated != 100 || got.StopReason != stopCap || got.Level != 3 {
		t.Fatalf("cap: rated=%d stop=%s level=%d", got.CardsRated, got.StopReason, got.Level)
	}

	var done [bands]BandTally
	done[0] = BandTally{Ratings: repeat(Good, 16), Dealable: 16}
	got = Score(done)
	if got.StopReason != stopExhausted || got.Level != 0 {
		t.Fatalf("exhausted: stop=%s level=%d", got.StopReason, got.Level)
	}
}

func tally(band, dealable int, ratings []int) [bands]BandTally {
	var out [bands]BandTally
	out[band] = BandTally{Ratings: ratings, Dealable: dealable}
	return out
}

func sixOfTen(agains int) []int {
	out := make([]int, 10)
	for i := range out {
		out[i] = Good
		if i < agains {
			out[i] = Again
		}
	}
	return out
}

func repeat(rating, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = rating
	}
	return out
}
