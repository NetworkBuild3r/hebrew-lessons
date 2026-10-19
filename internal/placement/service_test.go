package placement

import (
	"context"
	"encoding/json"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
)

func TestCorrectedBoundary(t *testing.T) {
	if passAt != 0.8 {
		t.Fatalf("passAt = %v, want 0.8", passAt)
	}
	if maxFalseAlarm != 0.25 {
		t.Fatalf("maxFalseAlarm = %v, want 0.25", maxFalseAlarm)
	}
	c, pass := correct(0.8, 0, false)
	if c != 0.8 || !pass {
		t.Fatalf("h=0.8 f=0: corrected=%v passed=%v", c, pass)
	}
	below := math.Nextafter(0.8, 0)
	if _, pass := correct(below, 0, false); pass {
		t.Fatalf("%v passed, want the open side of 0.8", below)
	}
	c, pass = correct(1, 1, true)
	if c != 0 || pass {
		t.Fatalf("f=1: corrected=%v passed=%v", c, pass)
	}
	// (0.9 − 0.5) / (1 − 0.5) = 0.8
	c, pass = correct(0.9, 0.5, false)
	if c != 0.8 || !pass {
		t.Fatalf("h=0.9 f=0.5: corrected=%v passed=%v", c, pass)
	}
}

func TestPassBandBoundaryOnARun(t *testing.T) {
	svc, store := openSvc(t, when)
	cat := mustCatalog(t)
	userID := insertLearner(t, store, "band@example.com", "sub-band")
	runID, items := start(t, svc, userID)
	if len(items) != len(cat) {
		t.Fatalf("dealt %d, catalog %d", len(items), len(cat))
	}
	ctx := context.Background()
	run, err := store.GetPlacementRun(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if !run.StartedAt.Equal(when) {
		t.Fatalf("started %s, want %s", run.StartedAt, when)
	}
	if run.Seed != when.UnixNano() {
		t.Fatalf("seed = %d, want the clock", run.Seed)
	}
	if !sameOrder(items, deal(run.Seed, cat)) {
		t.Fatal("deal order does not match the seed stored on the run")
	}
	if sameOrder(items, cat) {
		t.Fatal("shuffle left the fixture order unchanged")
	}
	body, err := json.Marshal(items[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "band") || strings.Contains(string(body), "pseudo") {
		t.Fatalf("client item leaked: %s", body)
	}
	if err := svc.Answer(ctx, runID, "not-an-item", true); err == nil {
		t.Fatal("unknown item was accepted")
	}
	if _, err := svc.Finish(ctx, runID); err == nil {
		t.Fatal("finish before every answer succeeded")
	}

	// f = 0. Band 0: 10/12 passes. Band 1: 9/12 fails. Higher bands: no.
	yes := map[string]bool{}
	n0, n1 := 0, 0
	for _, it := range cat {
		switch {
		case it.Pseudo:
			yes[it.ID] = false
		case *it.Band == 0 && n0 < 10:
			yes[it.ID] = true
			n0++
		case *it.Band == 1 && n1 < 9:
			yes[it.ID] = true
			n1++
		default:
			yes[it.ID] = false
		}
	}
	answerMap(t, svc, runID, cat, yes)
	result := finish(t, svc, runID)
	if result.Level != 0 || !result.Reliable || result.FalseAlarmRate != 0 {
		t.Fatalf("result level=%d reliable=%v f=%v", result.Level, result.Reliable, result.FalseAlarmRate)
	}
	if !result.Bands[0].Passed || result.Bands[0].H != 10.0/12 || result.Bands[0].Corrected != 10.0/12 {
		t.Fatalf("band 0: %+v", result.Bands[0])
	}
	if result.Bands[1].Passed || result.Bands[1].H != 9.0/12 {
		t.Fatalf("band 1: %+v", result.Bands[1])
	}
	for band := 2; band <= 4; band++ {
		if result.Bands[band].Passed {
			t.Fatalf("band %d passed", band)
		}
	}
	assertSeeds(t, store, cat, yes, result)
	// Band 0 contributes its 10 yes answers. Band 1 is 0.75 × 500 = 375.
	if result.EstimatedKnownWords != 385 || result.SuggestedStartPosition != 0 {
		t.Fatalf("estimate=%d start=%d", result.EstimatedKnownWords, result.SuggestedStartPosition)
	}
	if cards := cardCount(t, store, userID); cards != 0 {
		t.Fatalf("cards = %d, want 0", cards)
	}
	latest, err := svc.Latest(ctx, userID)
	if err != nil || latest == nil || latest.RunID != result.RunID || latest.Level != 0 {
		t.Fatalf("latest = %+v, %v", latest, err)
	}
	if latest.EstimatedKnownWords != 385 || latest.SuggestedStartPosition != 0 {
		t.Fatalf("latest estimate=%d start=%d", latest.EstimatedKnownWords, latest.SuggestedStartPosition)
	}
	var stored storedResult
	saved, err := store.GetPlacementRun(ctx, result.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(saved.Result), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.EstimatedKnownWords != 385 {
		t.Fatalf("stored estimate = %d", stored.EstimatedKnownWords)
	}
	if _, err := svc.Finish(ctx, runID); err == nil {
		t.Fatal("second finish succeeded")
	}
}

func TestUnreliableFalseAlarm(t *testing.T) {
	svc, store := openSvc(t, when)
	cat := mustCatalog(t)
	userID := insertLearner(t, store, "alarm@example.com", "sub-alarm")
	yes := map[string]bool{}
	pseudoYes := 0
	for _, it := range cat {
		if it.Pseudo && pseudoYes < 6 {
			yes[it.ID] = true
			pseudoYes++
			continue
		}
		yes[it.ID] = !it.Pseudo && it.Band != nil && *it.Band == 0
	}
	result := answerYes(t, svc, userID, cat, yes)
	if result.Reliable || result.FalseAlarmRate <= maxFalseAlarm {
		t.Fatalf("reliable=%v f=%v", result.Reliable, result.FalseAlarmRate)
	}
	if result.Level != 0 || !result.Bands[0].Passed {
		t.Fatalf("level=%d band0=%+v", result.Level, result.Bands[0])
	}
	if result.EstimatedKnownWords != 12 {
		t.Fatalf("estimate = %d, want the 12 band 0 yes answers", result.EstimatedKnownWords)
	}
	if n := lexemeCount(t, store); n != 0 {
		t.Fatalf("lexemes = %d, want none while unreliable", n)
	}
	if cards := cardCount(t, store, userID); cards != 0 {
		t.Fatalf("cards = %d", cards)
	}
}

func TestFalseAlarmOfOnePassesNothing(t *testing.T) {
	svc, store := openSvc(t, when)
	cat := mustCatalog(t)
	userID := insertLearner(t, store, "allyes@example.com", "sub-all")
	yes := map[string]bool{}
	for _, it := range cat {
		yes[it.ID] = true
	}
	result := answerYes(t, svc, userID, cat, yes)
	if result.Level != -1 || result.Reliable || result.FalseAlarmRate != 1 {
		t.Fatalf("level=%d reliable=%v f=%v", result.Level, result.Reliable, result.FalseAlarmRate)
	}
	for _, band := range result.Bands {
		if band.Passed || band.Corrected != 0 {
			t.Fatalf("band %+v passed with f=1", band)
		}
	}
	if result.EstimatedKnownWords != 12 {
		t.Fatalf("estimate = %d, want the 12 band 0 yes answers", result.EstimatedKnownWords)
	}
	if n := lexemeCount(t, store); n != 0 {
		t.Fatalf("lexemes = %d, want none when f=1", n)
	}
}

func TestFalseAlarmAtQuarterStillSeeds(t *testing.T) {
	svc, store := openSvc(t, when)
	cat := mustCatalog(t)
	userID := insertLearner(t, store, "edge@example.com", "sub-edge")
	yes := map[string]bool{}
	pseudoYes := 0
	for _, it := range cat {
		if it.Pseudo && pseudoYes < 5 {
			yes[it.ID] = true
			pseudoYes++
			continue
		}
		yes[it.ID] = !it.Pseudo && it.Band != nil && *it.Band == 2
	}
	result := answerYes(t, svc, userID, cat, yes)
	if !result.Reliable || result.FalseAlarmRate != 0.25 || result.Level != 2 {
		t.Fatalf("level=%d reliable=%v f=%v", result.Level, result.Reliable, result.FalseAlarmRate)
	}
	if !result.Bands[2].Passed || result.Bands[2].Corrected != 1 {
		t.Fatalf("band 2: %+v", result.Bands[2])
	}
	if result.EstimatedKnownWords != 500 {
		t.Fatalf("estimate = %d, want band 2's 500 ranks", result.EstimatedKnownWords)
	}
	assertSeeds(t, store, cat, yes, result)
	if cards := cardCount(t, store, userID); cards != 0 {
		t.Fatalf("cards = %d", cards)
	}
}

func TestRetakeKeepsSeedsAndUpdatesLevel(t *testing.T) {
	svc, store := openSvc(t, when)
	ctx := context.Background()
	cat := mustCatalog(t)
	userID := insertLearner(t, store, "retake@example.com", "sub-retake")

	first := answerPattern(t, svc, userID, cat, func(it fixtureItem) bool {
		return !it.Pseudo && it.Band != nil && *it.Band == 1
	})
	if first.Level != 1 || first.EstimatedKnownWords != 500 {
		t.Fatalf("first level = %d estimate = %d", first.Level, first.EstimatedKnownWords)
	}
	firstIDs := mustKnown(t, svc, userID)
	if len(firstIDs) != 12 {
		t.Fatalf("first seeds = %d, want 12", len(firstIDs))
	}

	second := answerPattern(t, svc, userID, cat, func(it fixtureItem) bool {
		return !it.Pseudo && it.Band != nil && *it.Band == 3
	})
	if second.Level != 3 || second.RunID == first.RunID || second.EstimatedKnownWords != 1000 {
		t.Fatalf("second = %+v", second)
	}
	latest, err := svc.Latest(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if latest == nil || latest.RunID != second.RunID || latest.Level != 3 || latest.EstimatedKnownWords != 1000 {
		t.Fatalf("latest = %+v", latest)
	}
	got := mustKnown(t, svc, userID)
	if len(got) != 24 {
		t.Fatalf("seeds after retake = %d, want 24", len(got))
	}
	have := map[int64]struct{}{}
	for _, id := range got {
		have[id] = struct{}{}
	}
	for _, id := range firstIDs {
		if _, ok := have[id]; !ok {
			t.Fatalf("seed %d from the first run was dropped", id)
		}
	}
	if cards := cardCount(t, store, userID); cards != 0 {
		t.Fatalf("cards = %d", cards)
	}
	for _, it := range cat {
		if !it.Pseudo {
			continue
		}
		_, _, _, _, ok, err := store.LexemeByText(ctx, it.TextNFC)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatalf("pseudo %s was written to lexemes", it.ID)
		}
	}
}

func TestKnownLexemeIDsUnion(t *testing.T) {
	svc, store := openSvc(t, when)
	ctx := context.Background()
	cat := mustCatalog(t)
	userID := insertLearner(t, store, "known@example.com", "sub-known")
	answerPattern(t, svc, userID, cat, func(it fixtureItem) bool {
		return !it.Pseudo && it.Band != nil && *it.Band == 0
	})
	seeded := mustKnown(t, svc, userID)

	finishedWord, err := testharness.InsertLexeme(ctx, store, "פֶּתֶק")
	if err != nil {
		t.Fatal(err)
	}
	openWord, err := testharness.InsertLexeme(ctx, store, "כּוֹס")
	if err != nil {
		t.Fatal(err)
	}
	recycleWord, err := testharness.InsertLexeme(ctx, store, "כַּף")
	if err != nil {
		t.Fatal(err)
	}
	learningWord, err := testharness.InsertLexeme(ctx, store, "מַזְלֵג")
	if err != nil {
		t.Fatal(err)
	}
	knownUnseeded, err := testharness.InsertLexeme(ctx, store, "צַלַּחַת")
	if err != nil {
		t.Fatal(err)
	}
	unitID, err := testharness.InsertUnit(ctx, store, "Cafe", "published")
	if err != nil {
		t.Fatal(err)
	}
	if err := testharness.InsertLessonItem(ctx, store, unitID, 0, finishedWord, "new", "recall"); err != nil {
		t.Fatal(err)
	}
	if err := testharness.InsertLessonItem(ctx, store, unitID, 0, recycleWord, "recycle", "recall"); err != nil {
		t.Fatal(err)
	}
	if err := testharness.InsertLessonItem(ctx, store, unitID, 1, openWord, "new", "recall"); err != nil {
		t.Fatal(err)
	}
	done := when
	if _, err := testharness.InsertLessonRun(ctx, store, userID, unitID, 0, when, &done); err != nil {
		t.Fatal(err)
	}
	if _, err := testharness.InsertLessonRun(ctx, store, userID, unitID, 1, when, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertLearnerLexeme(ctx, userID, learningWord, "learning", false, "lesson", when); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertLearnerLexeme(ctx, userID, knownUnseeded, "known", false, "manual", when); err != nil {
		t.Fatal(err)
	}

	got := mustKnown(t, svc, userID)
	have := map[int64]bool{}
	for _, id := range got {
		have[id] = true
	}
	for _, id := range seeded {
		if !have[id] {
			t.Fatalf("seeded id %d missing from the union", id)
		}
	}
	if !have[finishedWord] {
		t.Fatal("finished lesson's new word is missing")
	}
	for _, id := range []int64{openWord, recycleWord, learningWord, knownUnseeded} {
		if have[id] {
			t.Fatalf("lexeme %d should stay out of the union", id)
		}
	}
	if len(got) != len(seeded)+1 {
		t.Fatalf("union = %d, want %d", len(got), len(seeded)+1)
	}

	none, err := svc.KnownLexemeIDs(ctx, userID+99)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("unknown learner ids = %v", none)
	}
	latest, err := svc.Latest(ctx, userID+99)
	if err != nil || latest != nil {
		t.Fatalf("latest missing user = %v, %v", latest, err)
	}

	texts, err := store.PlacementKnownLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	var wantTexts []string
	for _, it := range cat {
		if it.Pseudo || it.Band == nil || *it.Band != 0 {
			continue
		}
		wantTexts = append(wantTexts, it.TextNFC)
	}
	slices.Sort(wantTexts)
	if !slices.Equal(texts, wantTexts) {
		t.Fatalf("placement known = %v, want %v", texts, wantTexts)
	}
	place, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		t.Fatalf("latest placement ok=%v err=%v", ok, err)
	}
	// The inserted unit has no newWords, so it is covered. The next position is 2.
	if place.SuggestedStartPosition != 2 || place.EstimatedKnownWords != 12 || !place.TakenAt.Equal(when) {
		t.Fatalf("placement = %+v", place)
	}
	noneTexts, err := store.PlacementKnownLexemes(ctx, userID+99)
	if err != nil || len(noneTexts) != 0 {
		t.Fatalf("unknown learner texts = %v, %v", noneTexts, err)
	}
	missing, ok, err := store.LatestPlacement(ctx, userID+99)
	if err != nil || ok || missing.SuggestedStartPosition != 0 {
		t.Fatalf("missing placement = %+v ok=%v err=%v", missing, ok, err)
	}
}

func TestSuggestedStartFollowsUnits(t *testing.T) {
	svc, store := openSvc(t, when)
	ctx := context.Background()
	cat := mustCatalog(t)
	userID := insertLearner(t, store, "start@example.com", "sub-start")
	result := answerPattern(t, svc, userID, cat, func(it fixtureItem) bool {
		return !it.Pseudo && it.Band != nil && *it.Band == 0
	})
	if result.SuggestedStartPosition != 0 || result.EstimatedKnownWords != 12 {
		t.Fatalf("before units: estimate=%d start=%d", result.EstimatedKnownWords, result.SuggestedStartPosition)
	}
	if _, err := testharness.InsertUnit(ctx, store, "One", "draft"); err != nil {
		t.Fatal(err)
	}
	latest, err := svc.Latest(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if latest == nil || latest.SuggestedStartPosition != 2 || latest.EstimatedKnownWords != 12 {
		t.Fatalf("after a covered unit: %+v", latest)
	}
}

func TestDealIsDeterministic(t *testing.T) {
	cat := mustCatalog(t)
	a := deal(99, cat)
	b := deal(99, cat)
	if !sameFixture(a, b) {
		t.Fatal("same seed dealt two orders")
	}
	c := deal(100, cat)
	if sameFixture(a, c) {
		t.Fatal("different seeds dealt the same order")
	}
}

var when = time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

func openSvc(t *testing.T, at time.Time) (*Service, *db.Store) {
	t.Helper()
	clk := fixedClock{at}
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return New(store, clk), store
}

func mustCatalog(t *testing.T) []fixtureItem {
	t.Helper()
	cat, err := catalog()
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func insertLearner(t *testing.T, store *db.Store, email, sub string) int64 {
	t.Helper()
	id, err := store.InsertUser(context.Background(), email, sub, "learner")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func start(t *testing.T, svc *Service, userID int64) (int64, []Item) {
	t.Helper()
	runID, items, err := svc.Start(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	return runID, items
}

func answerMap(t *testing.T, svc *Service, runID int64, cat []fixtureItem, yes map[string]bool) {
	t.Helper()
	ctx := context.Background()
	for _, it := range cat {
		if err := svc.Answer(ctx, runID, it.ID, yes[it.ID]); err != nil {
			t.Fatal(err)
		}
	}
}

func answerYes(t *testing.T, svc *Service, userID int64, cat []fixtureItem, yes map[string]bool) Result {
	t.Helper()
	runID, _ := start(t, svc, userID)
	answerMap(t, svc, runID, cat, yes)
	return finish(t, svc, runID)
}

func answerPattern(t *testing.T, svc *Service, userID int64, cat []fixtureItem, yes func(fixtureItem) bool) Result {
	t.Helper()
	answers := map[string]bool{}
	for _, it := range cat {
		answers[it.ID] = yes(it)
	}
	return answerYes(t, svc, userID, cat, answers)
}

func finish(t *testing.T, svc *Service, runID int64) Result {
	t.Helper()
	result, err := svc.Finish(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertSeeds(t *testing.T, store *db.Store, cat []fixtureItem, yes map[string]bool, result Result) {
	t.Helper()
	ctx := context.Background()
	for _, it := range cat {
		_, gloss, pos, verified, ok, err := store.LexemeByText(ctx, it.TextNFC)
		if err != nil {
			t.Fatal(err)
		}
		if it.Pseudo {
			if ok {
				t.Fatalf("pseudo %s was written to lexemes", it.ID)
			}
			continue
		}
		want := result.Reliable && result.Bands[*it.Band].Passed && yes[it.ID]
		if want != ok {
			t.Fatalf("lexeme %s present=%v, want %v", it.ID, ok, want)
		}
		if want && (gloss != it.GlossEN || pos != it.POS || verified) {
			t.Fatalf("lexeme %s gloss=%q pos=%q verified=%v", it.ID, gloss, pos, verified)
		}
	}
}

func lexemeCount(t *testing.T, store *db.Store) int {
	t.Helper()
	n := 0
	ctx := context.Background()
	for _, it := range mustCatalog(t) {
		_, _, _, _, ok, err := store.LexemeByText(ctx, it.TextNFC)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			n++
		}
	}
	return n
}

func cardCount(t *testing.T, store *db.Store, userID int64) int {
	t.Helper()
	n, err := store.CountCards(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func mustKnown(t *testing.T, svc *Service, userID int64) []int64 {
	t.Helper()
	ids, err := svc.KnownLexemeIDs(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	return ids
}

func sameOrder(items []Item, dealt []fixtureItem) bool {
	if len(items) != len(dealt) {
		return false
	}
	for i := range items {
		if items[i].ID != dealt[i].ID || items[i].TextNFC != dealt[i].TextNFC {
			return false
		}
	}
	return true
}

func sameFixture(a, b []fixtureItem) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			return false
		}
	}
	return true
}
