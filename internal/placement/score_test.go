package placement

import "testing"

func TestScoreAgainStop(t *testing.T) {
	if BandSize != 20 || MinRatedForStop != 10 || StopAgainMin != 0.60 || SittingSize != 20 || MaxCards != 100 {
		t.Fatalf("BandSize=%d minRated=%d again=%v sitting=%d cards=%d", BandSize, MinRatedForStop, StopAgainMin, SittingSize, MaxCards)
	}

	// Exactly 6/10 stops. Later bands stay unrated.
	trip := openTail()
	trip[0] = BandTally{Ratings: append(repeat(Again, 6), repeat(Good, 4)...), Dealable: BandSize}
	got := Score(trip)
	if got.StopReason != stopBand || got.Level != -1 || got.Bands[0].Rated != 10 || got.Bands[0].Again != 6 {
		t.Fatalf("6/10: stop=%s level=%d band=%+v", got.StopReason, got.Level, got.Bands[0])
	}

	// Nine Agains are under the minimum, so the band does not stop.
	nine := openTail()
	nine[0] = BandTally{Ratings: repeat(Again, 9), Dealable: BandSize}
	got = Score(nine)
	if got.StopReason != "" || got.Level != -1 || got.Bands[0].Rated != 9 || got.Bands[0].Again != 9 {
		t.Fatalf("9 agains: stop=%s level=%d band=%+v", got.StopReason, got.Level, got.Bands[0])
	}

	// 5/10 is under the line, so the band stays open.
	below := openTail()
	below[0] = BandTally{Ratings: append(repeat(Again, 5), repeat(Good, 5)...), Dealable: BandSize}
	got = Score(below)
	if got.StopReason != "" || got.Level != -1 || got.Bands[0].Rated != 10 {
		t.Fatalf("5/10: stop=%s level=%d band=%+v", got.StopReason, got.Level, got.Bands[0])
	}

	// "I don't know" counts as Again for the stop, and stays in DontKnow.
	dont := openTail()
	dont[0] = BandTally{Ratings: append(repeat(DontKnow, 6), repeat(Good, 4)...), Dealable: BandSize}
	got = Score(dont)
	if got.StopReason != stopBand || got.Level != -1 || got.Bands[0].DontKnow != 6 || got.Bands[0].Again != 0 {
		t.Fatalf("dont know: stop=%s level=%d band=%+v", got.StopReason, got.Level, got.Bands[0])
	}

	// Hard is not Again, so ten Hards do not stop.
	hard := openTail()
	hard[0] = BandTally{Ratings: repeat(Hard, MinRatedForStop), Dealable: BandSize}
	got = Score(hard)
	if got.StopReason != "" || got.Bands[0].Hard != MinRatedForStop || got.Bands[0].Again != 0 {
		t.Fatalf("hard: stop=%s band=%+v", got.StopReason, got.Bands[0])
	}

	// A finished earlier band still sets the level when the next one trips.
	var both [bands]BandTally
	both[0] = BandTally{Ratings: repeat(Good, BandSize), Dealable: BandSize}
	both[1] = BandTally{Ratings: append(repeat(Again, 6), repeat(Good, 4)...), Dealable: BandSize}
	got = Score(both)
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
		if got.EstimatedKnownWords != tc.want || got.Level != tc.band || got.StopReason != stopExhausted {
			t.Fatalf("band %d: estimate=%d level=%d stop=%s", tc.band, got.EstimatedKnownWords, got.Level, got.StopReason)
		}
		if got.SuggestedStartPosition != 0 {
			t.Fatalf("band %d set a start position of %d", tc.band, got.SuggestedStartPosition)
		}
	}

	var band0 [bands]BandTally
	band0[0] = BandTally{Ratings: []int{Easy, Easy, Again}, Dealable: 3}
	if got := Score(band0); got.EstimatedKnownWords != 2 || got.Level != 0 || got.StopReason != stopExhausted {
		t.Fatalf("band 0 count: estimate=%d level=%d stop=%s", got.EstimatedKnownWords, got.Level, got.StopReason)
	}

	// Three ratings are under the stop minimum, so the band counts.
	var third [bands]BandTally
	third[1] = BandTally{Ratings: []int{Good, Hard, Again}, Dealable: 3}
	if got := Score(third); got.EstimatedKnownWords != 167 || got.Level != 1 || got.StopReason != stopExhausted {
		t.Fatalf("one third of band 1: estimate=%d level=%d stop=%s", got.EstimatedKnownWords, got.Level, got.StopReason)
	}

	// Ratings past the band still count toward the card cap while a later band is open.
	var capped [bands]BandTally
	capped[0] = BandTally{Ratings: repeat(Good, 40), Dealable: 40}
	for band := 1; band < 4; band++ {
		capped[band] = BandTally{Ratings: repeat(Good, BandSize), Dealable: BandSize}
	}
	capped[4] = BandTally{Dealable: BandSize}
	got := Score(capped)
	if got.CardsRated != MaxCards || got.StopReason != stopCards || got.Level != 3 {
		t.Fatalf("cap: rated=%d stop=%s level=%d", got.CardsRated, got.StopReason, got.Level)
	}

	var done [bands]BandTally
	done[0] = BandTally{Ratings: repeat(Good, BandSize), Dealable: BandSize}
	got = Score(done)
	if got.StopReason != stopExhausted || got.Level != 0 {
		t.Fatalf("exhausted: stop=%s level=%d", got.StopReason, got.Level)
	}
}

func openTail() [bands]BandTally {
	var out [bands]BandTally
	for band := 1; band < bands; band++ {
		out[band] = BandTally{Dealable: BandSize}
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
