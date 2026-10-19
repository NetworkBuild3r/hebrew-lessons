package placement

import (
	"math"
	"testing"
)

func TestScoreBandWidthsAndBandZero(t *testing.T) {
	if passAt != 0.5 || maxFalseAlarm != 0.25 {
		t.Fatalf("thresholds pass=%v false-alarm=%v", passAt, maxFalseAlarm)
	}
	for _, tc := range []struct {
		band int
		want int
	}{
		{1, 500},
		{2, 500},
		{3, 1000},
		{4, 2000},
	} {
		got := Score(
			[]Answer{{ItemID: "w", Rating: Good}},
			[]ScoreItem{{ID: "w", Band: bandPtr(tc.band)}},
		)
		if got.EstimatedKnownWords != tc.want || got.Level != tc.band || got.Bands[tc.band].GoodPlusRate != 1 {
			t.Fatalf("band %d: estimate=%d level=%d rate=%v", tc.band, got.EstimatedKnownWords, got.Level, got.Bands[tc.band].GoodPlusRate)
		}
		if got.SuggestedStartPosition != 0 {
			t.Fatalf("band %d set a start position of %d", tc.band, got.SuggestedStartPosition)
		}
	}

	items := make([]ScoreItem, 12)
	answers := make([]Answer, 12)
	for i := range items {
		items[i] = ScoreItem{ID: string(rune('a' + i)), Band: bandPtr(0)}
		rating := Again
		if i < 10 {
			rating = Easy
		}
		answers[i] = Answer{ItemID: items[i].ID, Rating: rating}
	}
	got := Score(answers, items)
	if got.EstimatedKnownWords != 10 || got.Level != 0 || got.Bands[0].Easy != 10 || got.Bands[0].Again != 2 {
		t.Fatalf("band 0: estimate=%d level=%d band=%+v", got.EstimatedKnownWords, got.Level, got.Bands[0])
	}
	if len(got.Bands) != 5 {
		t.Fatalf("bands = %d, want 5", len(got.Bands))
	}
}

func TestScoreHalfPassesAndBelowFails(t *testing.T) {
	items := make([]ScoreItem, 12)
	for i := range items {
		items[i] = ScoreItem{ID: string(rune('a' + i)), Band: bandPtr(0)}
	}
	half := make([]Answer, 12)
	below := make([]Answer, 12)
	for i := range items {
		halfRating := Again
		belowRating := Again
		if i < 6 {
			halfRating = Good
		}
		if i < 5 {
			belowRating = Good
		}
		half[i] = Answer{ItemID: items[i].ID, Rating: halfRating}
		below[i] = Answer{ItemID: items[i].ID, Rating: belowRating}
	}
	got := Score(half, items)
	if got.Level != 0 || got.Bands[0].GoodPlusRate != 0.5 || got.Bands[0].Shown != 12 {
		t.Fatalf("exactly half: level=%d band=%+v", got.Level, got.Bands[0])
	}
	miss := Score(below, items)
	if miss.Level != -1 || miss.Bands[0].Good != 5 || miss.Bands[0].GoodPlusRate >= 0.5 {
		t.Fatalf("below half: level=%d band=%+v", miss.Level, miss.Bands[0])
	}
}

func TestScoreEstimateRoundsAndIgnoresMissing(t *testing.T) {
	// One Good of three rated words is 1/3 of the band, which rounds to 167.
	// A fourth item with no answer is not shown.
	third := Score([]Answer{
		{ItemID: "a", Rating: Good},
		{ItemID: "b", Rating: Hard},
		{ItemID: "c", Rating: Again},
	}, []ScoreItem{
		{ID: "a", Band: bandPtr(1)},
		{ID: "b", Band: bandPtr(1)},
		{ID: "c", Band: bandPtr(1)},
		{ID: "d", Band: bandPtr(1)},
	})
	if third.EstimatedKnownWords != 167 || third.Level != -1 || third.Bands[1].Shown != 3 {
		t.Fatalf("one third: estimate=%d level=%d band=%+v", third.EstimatedKnownWords, third.Level, third.Bands[1])
	}
	if math.Abs(third.Bands[1].GoodPlusRate-1.0/3.0) > 1e-12 {
		t.Fatalf("rate = %v", third.Bands[1].GoodPlusRate)
	}

	// Band 1 at 1 and band 2 at 0.5 sum to 750. Band 0 adds its two known words.
	sum := Score([]Answer{
		{ItemID: "z", Rating: Good},
		{ItemID: "y", Rating: Easy},
		{ItemID: "a", Rating: Good},
		{ItemID: "b", Rating: Good},
		{ItemID: "c", Rating: Again},
	}, []ScoreItem{
		{ID: "z", Band: bandPtr(0)},
		{ID: "y", Band: bandPtr(0)},
		{ID: "a", Band: bandPtr(1)},
		{ID: "b", Band: bandPtr(2)},
		{ID: "c", Band: bandPtr(2)},
	})
	if sum.EstimatedKnownWords != 752 || sum.Level != 2 {
		t.Fatalf("sum: estimate=%d level=%d bands=%+v", sum.EstimatedKnownWords, sum.Level, sum.Bands)
	}
}

func TestScoreFalseAlarm(t *testing.T) {
	// Hard on a pseudo-word is not a false alarm.
	hard := Score(
		[]Answer{{ItemID: "p", Rating: Hard}, {ItemID: "w", Rating: Good}},
		[]ScoreItem{{ID: "p", Pseudo: true}, {ID: "w", Band: bandPtr(1)}},
	)
	if hard.FalseAlarmRate != 0 || !hard.Reliable || hard.EstimatedKnownWords != 500 {
		t.Fatalf("hard pseudo: %+v", hard)
	}

	// 1 of 4 is exactly 0.25 and still reliable.
	quarter := Score([]Answer{
		{ItemID: "p1", Rating: Easy},
		{ItemID: "p2", Rating: Again},
		{ItemID: "p3", Rating: Hard},
		{ItemID: "p4", Rating: DontKnow},
		{ItemID: "w", Rating: Good},
	}, []ScoreItem{
		{ID: "p1", Pseudo: true},
		{ID: "p2", Pseudo: true},
		{ID: "p3", Pseudo: true},
		{ID: "p4", Pseudo: true},
		{ID: "w", Band: bandPtr(1)},
	})
	if quarter.FalseAlarmRate != 0.25 || !quarter.Reliable {
		t.Fatalf("quarter: f=%v reliable=%v", quarter.FalseAlarmRate, quarter.Reliable)
	}

	// 2 of 4 is above 0.25. The level is still recorded.
	over := Score([]Answer{
		{ItemID: "p1", Rating: Good},
		{ItemID: "p2", Rating: Easy},
		{ItemID: "p3", Rating: Again},
		{ItemID: "p4", Rating: Again},
		{ItemID: "w", Rating: Good},
	}, []ScoreItem{
		{ID: "p1", Pseudo: true},
		{ID: "p2", Pseudo: true},
		{ID: "p3", Pseudo: true},
		{ID: "p4", Pseudo: true},
		{ID: "w", Band: bandPtr(2)},
	})
	if over.FalseAlarmRate != 0.5 || over.Reliable || over.Level != 2 || over.EstimatedKnownWords != 500 {
		t.Fatalf("over: f=%v reliable=%v level=%d estimate=%d", over.FalseAlarmRate, over.Reliable, over.Level, over.EstimatedKnownWords)
	}

	none := Score([]Answer{{ItemID: "w", Rating: DontKnow}}, []ScoreItem{{ID: "w", Band: bandPtr(0)}})
	if none.FalseAlarmRate != 0 || !none.Reliable || none.Level != -1 || none.EstimatedKnownWords != 0 || none.Bands[0].DontKnow != 1 {
		t.Fatalf("no pseudo: %+v", none)
	}
}

func bandPtr(band int) *int { return &band }
