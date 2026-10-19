package placement

import (
	"math"
	"testing"
)

func TestScoreBandWidthsAndBandZero(t *testing.T) {
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
			[]Answer{{ItemID: "w", Yes: true}},
			[]ScoreItem{{ID: "w", Band: bandPtr(tc.band)}},
		)
		if got.EstimatedKnownWords != tc.want || got.Level != tc.band || !got.Bands[tc.band].Passed {
			t.Fatalf("band %d: estimate=%d level=%d passed=%v", tc.band, got.EstimatedKnownWords, got.Level, got.Bands[tc.band].Passed)
		}
		if got.SuggestedStartPosition != 0 {
			t.Fatalf("band %d set a start position of %d", tc.band, got.SuggestedStartPosition)
		}
	}

	items := make([]ScoreItem, 12)
	answers := make([]Answer, 12)
	for i := range items {
		items[i] = ScoreItem{ID: string(rune('a' + i)), Band: bandPtr(0)}
		answers[i] = Answer{ItemID: items[i].ID, Yes: i < 10}
	}
	got := Score(answers, items)
	if got.EstimatedKnownWords != 10 || got.Level != 0 || !got.Bands[0].Passed {
		t.Fatalf("band 0: estimate=%d level=%d band=%+v", got.EstimatedKnownWords, got.Level, got.Bands[0])
	}
	if len(got.Bands) != 5 {
		t.Fatalf("bands = %d, want 5", len(got.Bands))
	}
}

func TestScoreClampRoundAndFalseAlarmOfOne(t *testing.T) {
	if clamp01(-0.4) != 0 || clamp01(1.7) != 1 || clamp01(0.25) != 0.25 {
		t.Fatal("clamp01")
	}
	if math.IsNaN(clamp01(math.NaN())) || clamp01(math.NaN()) != 0 {
		t.Fatal("clamp01(NaN)")
	}

	// h = 0, f = 0.5, corrected = -1. The negative score adds nothing.
	negative := Score(
		[]Answer{{ItemID: "w", Yes: false}, {ItemID: "p", Yes: true}, {ItemID: "q", Yes: false}},
		[]ScoreItem{{ID: "w", Band: bandPtr(1)}, {ID: "p", Pseudo: true}, {ID: "q", Pseudo: true}},
	)
	if negative.Bands[1].Corrected != -1 || negative.EstimatedKnownWords != 0 || negative.Bands[1].Passed {
		t.Fatalf("negative corrected: %+v estimate=%d", negative.Bands[1], negative.EstimatedKnownWords)
	}

	// (1/3) * 500 rounds to 167. The two missing answers count as no.
	third := Score([]Answer{{ItemID: "a", Yes: true}}, []ScoreItem{
		{ID: "a", Band: bandPtr(1)},
		{ID: "b", Band: bandPtr(1)},
		{ID: "c", Band: bandPtr(1)},
	})
	if third.EstimatedKnownWords != 167 || third.Bands[1].Passed {
		t.Fatalf("one third of band 1: estimate=%d passed=%v corrected=%v", third.EstimatedKnownWords, third.Bands[1].Passed, third.Bands[1].Corrected)
	}

	// Exactly 0.8 of band 1 is 400 words and a pass. 0.6 is 300 and a miss.
	four := Score([]Answer{
		{ItemID: "a", Yes: true}, {ItemID: "b", Yes: true}, {ItemID: "c", Yes: true}, {ItemID: "d", Yes: true},
	}, fiveBand(1))
	if !four.Bands[1].Passed || four.Bands[1].Corrected != 0.8 || four.EstimatedKnownWords != 400 {
		t.Fatalf("4/5: %+v estimate=%d", four.Bands[1], four.EstimatedKnownWords)
	}
	three := Score([]Answer{
		{ItemID: "a", Yes: true}, {ItemID: "b", Yes: true}, {ItemID: "c", Yes: true},
	}, fiveBand(1))
	if three.Bands[1].Passed || three.EstimatedKnownWords != 300 {
		t.Fatalf("3/5: %+v estimate=%d", three.Bands[1], three.EstimatedKnownWords)
	}

	// f = 1 stores corrected 0, and the band 0 yes still counts.
	blocked := Score([]Answer{
		{ItemID: "a", Yes: true}, {ItemID: "b", Yes: true}, {ItemID: "p", Yes: true},
	}, []ScoreItem{
		{ID: "a", Band: bandPtr(0)},
		{ID: "b", Band: bandPtr(1)},
		{ID: "p", Pseudo: true},
	})
	if blocked.Level != -1 || blocked.Reliable || blocked.FalseAlarmRate != 1 || blocked.EstimatedKnownWords != 1 {
		t.Fatalf("f=1: level=%d reliable=%v f=%v estimate=%d", blocked.Level, blocked.Reliable, blocked.FalseAlarmRate, blocked.EstimatedKnownWords)
	}
	for _, band := range blocked.Bands {
		if band.Corrected != 0 || band.Passed {
			t.Fatalf("f=1 band %+v", band)
		}
	}
}

func TestScoreAnswerRules(t *testing.T) {
	// The later answer wins. A pseudo-word that also carries a band stays a
	// pseudo-word. A band outside 0–4 is ignored. A nil band is a pseudo-word.
	got := Score([]Answer{
		{ItemID: "w", Yes: false},
		{ItemID: "w", Yes: true},
		{ItemID: "p", Yes: true},
		{ItemID: "z", Yes: true},
		{ItemID: "n", Yes: true},
	}, []ScoreItem{
		{ID: "w", Band: bandPtr(2)},
		{ID: "p", Band: bandPtr(2), Pseudo: true},
		{ID: "z", Band: bandPtr(7)},
		{ID: "n", Band: nil},
	})
	if got.Bands[2].H != 1 || got.Bands[2].F != 1 || got.FalseAlarmRate != 1 {
		t.Fatalf("rules: band2=%+v f=%v", got.Bands[2], got.FalseAlarmRate)
	}
	if got.EstimatedKnownWords != 0 || got.Level != -1 {
		t.Fatalf("rules: estimate=%d level=%d", got.EstimatedKnownWords, got.Level)
	}

	// No pseudo-words leaves f at 0, and the run can pass.
	open := Score([]Answer{{ItemID: "w", Yes: true}}, []ScoreItem{{ID: "w", Band: bandPtr(3)}})
	if open.FalseAlarmRate != 0 || !open.Reliable || open.EstimatedKnownWords != 1000 || !open.Bands[3].Passed {
		t.Fatalf("no pseudo: %+v", open)
	}

	empty := Score(nil, nil)
	if empty.Level != -1 || !empty.Reliable || empty.EstimatedKnownWords != 0 || len(empty.Bands) != 5 {
		t.Fatalf("empty: %+v", empty)
	}
	for _, band := range empty.Bands {
		if band.H != 0 || band.Passed {
			t.Fatalf("empty band %+v", band)
		}
	}
}

func bandPtr(n int) *int {
	b := n
	return &b
}

func fiveBand(band int) []ScoreItem {
	items := make([]ScoreItem, 5)
	for i := range items {
		items[i] = ScoreItem{ID: string(rune('a' + i)), Band: bandPtr(band)}
	}
	return items
}
