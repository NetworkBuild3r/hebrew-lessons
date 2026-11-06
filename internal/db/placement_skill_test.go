package db

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"
)

// Authored-by-spec: INIT-032/SPEC-002

func TestPlacementSkillBandsListeningOnly(t *testing.T) {
	when := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "listen-only@example.com", "sub-listen-only", "learner")

	runID, err := s.StartPlacementV3(ctx, userID, when, 1, RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	lex := mustLexemeText(t, s, "שָׁלוֹם")
	claimItem(t, s, runID, 1, PartLetters, "", 0, "alef", "right", 0, when)
	claimItem(t, s, runID, 2, PartListening, "listen", 1, "w-1", "right", lex, when)
	claimItem(t, s, runID, 3, PartListening, "listen", 1, "w-2", "wrong", lex, when)
	// Typing must not invent a recall (or read) entry when the skill filter is kept.
	claimItem(t, s, runID, 4, PartTyping, "recall", 1, "w-1-type", "right", lex, when)
	finishV3Zeros(t, s, runID, when.Add(time.Minute))

	got, err := s.PlacementSkillBands(ctx, userID, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["listen"]; !ok {
		t.Fatalf("got %+v, want listen present", got)
	}
	// Dropping the skill filter (or copying one run onto every skill) would put
	// listen bands under read. This fixture has no reading items, so read must
	// stay absent — that is the mutation that must fail the listening-only case.
	for _, skill := range []string{"read", "recall", "spell"} {
		if _, ok := got[skill]; ok {
			t.Fatalf("got skill %q bands %+v, want absent on a listening-only run", skill, got[skill])
		}
	}
	if len(got) != 1 {
		t.Fatalf("got %d skills %+v, want only listen", len(got), got)
	}
}

func TestPlacementSkillBandsReadFromOlderListenFromNewer(t *testing.T) {
	when := time.Date(2026, 11, 6, 11, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "cross-run@example.com", "sub-cross-run", "learner")
	lex := mustLexemeText(t, s, "בַּיִת")

	readRun, err := s.StartPlacementV3(ctx, userID, when, 2, RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	claimItem(t, s, readRun, 1, PartReading, "read", 2, "r-1", "right", lex, when)
	claimItem(t, s, readRun, 2, PartReading, "read", 2, "r-2", "wrong", lex, when)
	finishV3Zeros(t, s, readRun, when.Add(time.Minute))

	listenRun, err := s.StartPlacementV3(ctx, userID, when.Add(2*time.Minute), 3, RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	claimItem(t, s, listenRun, 1, PartListening, "listen", 1, "l-1", "right", lex, when.Add(2*time.Minute))
	claimItem(t, s, listenRun, 2, PartListening, "listen", 1, "l-2", "dont_know", lex, when.Add(2*time.Minute))
	finishV3Zeros(t, s, listenRun, when.Add(3*time.Minute))

	got, err := s.PlacementSkillBands(ctx, userID, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	listen, ok := got["listen"]
	if !ok || len(listen.Bands) != 1 || listen.Bands[0].Band != 0 || listen.Bands[0].Rated != 2 || listen.Bands[0].Good != 1 || listen.Bands[0].DontKnow != 1 {
		t.Fatalf("listen %+v, want band 0 from the newer run (1 right, 1 don't-know)", listen)
	}
	read, ok := got["read"]
	if !ok || len(read.Bands) != 1 || read.Bands[0].Band != 1 || read.Bands[0].Rated != 2 || read.Bands[0].Good != 1 || read.Bands[0].Again != 1 {
		t.Fatalf("read %+v, want band 1 from the older run (1 right, 1 wrong)", read)
	}
	if _, ok := got["recall"]; ok {
		t.Fatalf("recall present: %+v", got["recall"])
	}
}

func TestPlacementSkillBandsV2MapsToReadOnly(t *testing.T) {
	when := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "v2-bands@example.com", "sub-v2-bands", "learner")

	runID, err := s.InsertPlacementRun(ctx, userID, when, 9)
	if err != nil {
		t.Fatal(err)
	}
	bands := []BandScore{{
		Band: 0, Rated: 4, Good: 3, Again: 1, GoodPlusRate: 0.75,
	}}
	body, err := json.Marshal(placementResultJSON{Bands: bands, EstimatedKnownWords: 10})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishPlacement(ctx, runID, when.Add(time.Minute), 0, "exhausted", string(body)); err != nil {
		t.Fatal(err)
	}

	got, err := s.PlacementSkillBands(ctx, userID, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	read, ok := got["read"]
	if !ok || len(read.Bands) != 1 || read.Bands[0].Band != 0 || read.Bands[0].Rated != 4 || read.Bands[0].Good != 3 || read.Bands[0].GoodPlusRate != 0.75 {
		t.Fatalf("read %+v, want the stored v2 bands", read)
	}
	for _, skill := range []string{"listen", "recall", "spell"} {
		if _, ok := got[skill]; ok {
			t.Fatalf("v2 run put bands on %q: %+v", skill, got[skill])
		}
	}
}

func TestPlacementSkillBandsIgnoresUnfinishedNewest(t *testing.T) {
	when := time.Date(2026, 11, 6, 13, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "unfinished@example.com", "sub-unfinished", "learner")
	lex := mustLexemeText(t, s, "מַיִם")

	finishedID, err := s.StartPlacementV3(ctx, userID, when, 4, RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	claimItem(t, s, finishedID, 1, PartListening, "listen", 1, "fin-1", "right", lex, when)
	claimItem(t, s, finishedID, 2, PartListening, "listen", 1, "fin-2", "right", lex, when)
	finishV3Zeros(t, s, finishedID, when.Add(time.Minute))

	openID, err := s.StartPlacementV3(ctx, userID, when.Add(2*time.Minute), 5, RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	claimItem(t, s, openID, 1, PartListening, "listen", 3, "open-1", "wrong", lex, when.Add(2*time.Minute))

	got, err := s.PlacementSkillBands(ctx, userID, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	listen, ok := got["listen"]
	if !ok || len(listen.Bands) != 1 || listen.Bands[0].Band != 0 || listen.Bands[0].Rated != 2 || listen.Bands[0].Good != 2 || listen.Bands[0].Again != 0 {
		t.Fatalf("listen %+v, want the finished run (2 right in band 1), not the open band-3 wrong", listen)
	}
}

func TestPlacementSkillBandsLearner1Run4Shape(t *testing.T) {
	when := time.Date(2026, 11, 6, 14, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "run4-shape@example.com", "sub-run4-shape", "learner")
	lex := mustLexemeText(t, s, "אֶרֶץ")

	runID, err := s.StartPlacementV3(ctx, userID, when, 4, RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	seq := 1
	claimItem(t, s, runID, seq, PartLetters, "", 0, "letter", "right", 0, when)
	seq++
	// Listening bands 1–3 shaped like learner 1 run 4: 8/12, 5/6, 5/12.
	seedListenBand(t, s, runID, &seq, 1, 8, 4, 0, lex, when)
	seedListenBand(t, s, runID, &seq, 2, 5, 1, 0, lex, when)
	seedListenBand(t, s, runID, &seq, 3, 5, 7, 0, lex, when)

	// Production stores the guessing-corrected share (v3Share), not right/rated.
	// 8/4 of 12 → ~0.556; 5/1 of 6 → ~0.778; 5/7 of 12 → ~0.222.
	wantRates := []float64{
		v3Share(8, 4, 12),
		v3Share(5, 1, 6),
		v3Share(5, 7, 12),
	}
	stored := []BandScore{
		{Band: 0, Rated: 12, Good: 8, Again: 4, GoodPlusRate: wantRates[0]},
		{Band: 1, Rated: 6, Good: 5, Again: 1, GoodPlusRate: wantRates[1]},
		{Band: 2, Rated: 12, Good: 5, Again: 7, GoodPlusRate: wantRates[2]},
	}
	body, err := json.Marshal(placementResultJSON{Bands: stored})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishPlacement(ctx, runID, when.Add(time.Minute), 1, "band_stop", string(body)); err != nil {
		t.Fatal(err)
	}

	got, err := s.PlacementSkillBands(ctx, userID, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	listen, ok := got["listen"]
	if !ok {
		t.Fatal("listen absent")
	}
	if len(listen.Bands) != 3 {
		t.Fatalf("listen bands %+v, want 3", listen)
	}
	rawRates := []float64{8.0 / 12.0, 5.0 / 6.0, 5.0 / 12.0}
	for i, want := range wantRates {
		if listen.Bands[i].Band != i {
			t.Fatalf("band[%d]=%d, want %d", i, listen.Bands[i].Band, i)
		}
		if math.Abs(listen.Bands[i].GoodPlusRate-want) > 1e-12 {
			t.Fatalf("band %d GoodPlusRate=%v, want corrected %v", i, listen.Bands[i].GoodPlusRate, want)
		}
		if math.Abs(listen.Bands[i].GoodPlusRate-rawRates[i]) < 1e-12 {
			t.Fatalf("band %d rate %v equals raw right/rated; must use v3Share", i, listen.Bands[i].GoodPlusRate)
		}
		if math.Abs(listen.Bands[i].GoodPlusRate-stored[i].GoodPlusRate) > 1e-12 {
			t.Fatalf("band %d rate %v != stored result.bands rate %v", i, listen.Bands[i].GoodPlusRate, stored[i].GoodPlusRate)
		}
		if listen.Bands[i].Rated != stored[i].Rated || listen.Bands[i].Good != stored[i].Good || listen.Bands[i].Again != stored[i].Again {
			t.Fatalf("band %d tally %+v, want rated/good/again matching stored %+v", i, listen.Bands[i], stored[i])
		}
	}
}

func finishV3Zeros(t *testing.T, s *Store, runID int64, finished time.Time) {
	t.Helper()
	zeros := make([]BandScore, 5)
	for i := range zeros {
		zeros[i].Band = i
	}
	body, err := json.Marshal(placementResultJSON{Bands: zeros})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishPlacement(context.Background(), runID, finished, -1, "exhausted", string(body)); err != nil {
		t.Fatal(err)
	}
}

func seedListenBand(t *testing.T, s *Store, runID int64, seq *int, band, right, wrong, dont int, lex int64, at time.Time) {
	t.Helper()
	for i := 0; i < right; i++ {
		claimItem(t, s, runID, *seq, PartListening, "listen", band, itemID("r", band, i), "right", lex, at)
		*seq++
	}
	for i := 0; i < wrong; i++ {
		claimItem(t, s, runID, *seq, PartListening, "listen", band, itemID("w", band, i), "wrong", lex, at)
		*seq++
	}
	for i := 0; i < dont; i++ {
		claimItem(t, s, runID, *seq, PartListening, "listen", band, itemID("d", band, i), "dont_know", lex, at)
		*seq++
	}
}

func itemID(kind string, band, i int) string {
	return fmt.Sprintf("%s-b%d-%d", kind, band, i)
}

func TestPlacementSkillBandsRunAtFromOlderReading(t *testing.T) {
	when := time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "runat@example.com", "sub-runat", "learner")
	lex := mustLexemeText(t, s, "סֵפֶר")

	readFinish := when.Add(time.Minute)
	readRun, err := s.StartPlacementV3(ctx, userID, when, 2, RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	claimItem(t, s, readRun, 1, PartReading, "read", 2, "r-1", "right", lex, when)
	claimItem(t, s, readRun, 2, PartReading, "read", 2, "r-2", "wrong", lex, when)
	finishV3Zeros(t, s, readRun, readFinish)

	listenFinish := when.Add(3 * time.Minute)
	listenRun, err := s.StartPlacementV3(ctx, userID, when.Add(2*time.Minute), 3, RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	claimItem(t, s, listenRun, 1, PartListening, "listen", 1, "l-1", "right", lex, when.Add(2*time.Minute))
	finishV3Zeros(t, s, listenRun, listenFinish)

	got, err := s.PlacementSkillBands(ctx, userID, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	read, ok := got["read"]
	if !ok {
		t.Fatal("read absent")
	}
	if !read.RunAt.Equal(readFinish) {
		t.Fatalf("read RunAt = %s, want older reading finish %s", read.RunAt, readFinish)
	}
	listen, ok := got["listen"]
	if !ok {
		t.Fatal("listen absent")
	}
	if !listen.RunAt.Equal(listenFinish) {
		t.Fatalf("listen RunAt = %s, want newer listen finish %s", listen.RunAt, listenFinish)
	}

	// AsOf between the two finishes sees only reading.
	mid := when.Add(2 * time.Minute)
	past, err := s.PlacementSkillBands(ctx, userID, mid)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := past["listen"]; ok {
		t.Fatalf("listen present as of %s: %+v", mid, past["listen"])
	}
	if r, ok := past["read"]; !ok || !r.RunAt.Equal(readFinish) {
		t.Fatalf("read as of mid = %+v, want older reading", past["read"])
	}
}
