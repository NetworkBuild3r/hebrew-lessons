package placement

import (
	"testing"
	"time"
)

func TestScoreBlockRule(t *testing.T) {
	if BlockSize != 20 || StopGoodPlusMin != 0.50 || MaxCards != 100 || MaxDuration != 10*time.Minute {
		t.Fatalf("BlockSize=%d min=%v cards=%d duration=%s", BlockSize, StopGoodPlusMin, MaxCards, MaxDuration)
	}

	// Exactly StopGoodPlusMin does not stop. Later bands are still open.
	pass := openTail()
	pass[0] = BandTally{Ratings: append(repeat(Good, 10), repeat(Again, 10)...), Dealable: BlockSize}
	got := Score(pass)
	if got.StopReason != "" || got.Level != 0 || got.Bands[0].Rated != BlockSize || got.Bands[0].GoodPlusRate != 0.5 {
		t.Fatalf("10/20: stop=%s level=%d band=%+v", got.StopReason, got.Level, got.Bands[0])
	}

	trip := Score(tally(0, BlockSize, append(repeat(Good, 9), repeat(Again, 11)...)))
	if trip.StopReason != stopBand || trip.Level != -1 || trip.Bands[0].Again != 11 || trip.Bands[0].GoodPlusRate != 9.0/20 {
		t.Fatalf("9/20: stop=%s level=%d band=%+v", trip.StopReason, trip.Level, trip.Bands[0])
	}

	dont := Score(tally(0, BlockSize, append(repeat(DontKnow, 11), repeat(Good, 9)...)))
	if dont.StopReason != stopBand || dont.Level != -1 || dont.Bands[0].DontKnow != 11 || dont.Bands[0].Again != 0 || dont.Bands[0].GoodPlusRate != 9.0/20 {
		t.Fatalf("dont know: stop=%s level=%d band=%+v", dont.StopReason, dont.Level, dont.Bands[0])
	}

	// A finished earlier band still sets the level when the next one trips.
	var both [bands]BandTally
	both[0] = BandTally{Ratings: repeat(Good, BlockSize), Dealable: BlockSize}
	both[1] = BandTally{Ratings: append(repeat(Again, 11), repeat(Good, 9)...), Dealable: BlockSize}
	got = Score(both)
	if got.Level != 0 || got.StopReason != stopBand {
		t.Fatalf("band 1 trip: level=%d stop=%s", got.Level, got.StopReason)
	}

	// Hard is not Good or Easy, so a block of Hards stops.
	hard := Score(tally(0, BlockSize, repeat(Hard, BlockSize)))
	if hard.StopReason != stopBand || hard.Level != -1 || hard.Bands[0].Hard != BlockSize || hard.Bands[0].GoodPlusRate != 0 {
		t.Fatalf("hard stops: stop=%s level=%d band=%+v", hard.StopReason, hard.Level, hard.Bands[0])
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

	// One Good out of three is below the line, so the band does not count.
	var third [bands]BandTally
	third[1] = BandTally{Ratings: []int{Good, Hard, Again}, Dealable: 3}
	if got := Score(third); got.EstimatedKnownWords != 167 || got.Level != -1 || got.StopReason != stopBand {
		t.Fatalf("one third of band 1: estimate=%d level=%d stop=%s", got.EstimatedKnownWords, got.Level, got.StopReason)
	}

	// Ratings past the block still count toward the card cap while a later block is open.
	var capped [bands]BandTally
	capped[0] = BandTally{Ratings: repeat(Good, 40), Dealable: 40}
	for band := 1; band < 4; band++ {
		capped[band] = BandTally{Ratings: repeat(Good, BlockSize), Dealable: BlockSize}
	}
	capped[4] = BandTally{Dealable: BlockSize}
	got := Score(capped)
	if got.CardsRated != MaxCards || got.StopReason != stopCards || got.Level != 3 {
		t.Fatalf("cap: rated=%d stop=%s level=%d", got.CardsRated, got.StopReason, got.Level)
	}

	var done [bands]BandTally
	done[0] = BandTally{Ratings: repeat(Good, BlockSize), Dealable: BlockSize}
	got = Score(done)
	if got.StopReason != stopExhausted || got.Level != 0 {
		t.Fatalf("exhausted: stop=%s level=%d", got.StopReason, got.Level)
	}
}

func openTail() [bands]BandTally {
	var out [bands]BandTally
	for band := 1; band < bands; band++ {
		out[band] = BandTally{Dealable: BlockSize}
	}
	return out
}

func tally(band, dealable int, ratings []int) [bands]BandTally {
	var out [bands]BandTally
	out[band] = BandTally{Ratings: ratings, Dealable: dealable}
	return out
}

func repeat(rating, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = rating
	}
	return out
}
