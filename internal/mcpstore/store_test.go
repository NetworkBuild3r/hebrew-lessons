package mcpstore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	sdauth "github.com/modelcontextprotocol/go-sdk/auth"
)

const (
	hello  = "שָׁלוֹם"
	coffee = "קָפֶה"
	book   = "סֵפֶר"
	fish   = "דָּג"
	house  = "בַּיִת"
	whole  = "שָׁלֵם"
)

func TestUnitsInScopeFiltersAndOrders(t *testing.T) {
	ctx := context.Background()
	store, raw := openAdapter(t, nil)
	userA := mustUser(t, raw)
	userB := mustOtherUser(t, raw)
	scopeA := "learner:" + strconv.FormatInt(userA, 10)
	scopeB := "learner:" + strconv.FormatInt(userB, 10)
	early := insertUnitRow(t, raw, "Early", mcp.ScopeShared, "draft", 0, hello)
	middle, err := testharness.InsertUnit(ctx, raw, "Middle", "draft")
	if err != nil {
		t.Fatal(err)
	}
	late := insertUnitRow(t, raw, "Late", mcp.ScopeShared, "draft", 2, coffee)
	mine := insertUnitRow(t, raw, "Mine", scopeA, "draft", 0, book)
	theirs := insertUnitRow(t, raw, "Theirs", scopeB, "draft", 5, fish)

	shared, err := store.UnitsInScope(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ids(shared) != joinIDs(early, middle, late) {
		t.Fatalf("shared = %+v", shared)
	}
	mineRows, err := store.UnitsInScope(ctx, &userA)
	if err != nil {
		t.Fatal(err)
	}
	if ids(mineRows) != joinIDs(early, mine, middle, late) {
		t.Fatalf("learner A = %+v", mineRows)
	}
	for _, row := range mineRows {
		if row.Scope == scopeB || row.ID == theirs || row.Title == "Theirs" {
			t.Fatalf("learner A saw %+v", mineRows)
		}
	}
	otherRows, err := store.UnitsInScope(ctx, &userB)
	if err != nil {
		t.Fatal(err)
	}
	if ids(otherRows) != joinIDs(early, middle, late, theirs) {
		t.Fatalf("learner B = %+v", otherRows)
	}
	for _, row := range otherRows {
		if row.Scope == scopeA || row.ID == mine || row.Title == "Mine" {
			t.Fatalf("learner B saw %+v", otherRows)
		}
	}
}

func TestLatestPlacementMapsFinishedRun(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, raw := openAdapter(t, fixedClock{when})
	userID := mustUser(t, raw)

	none, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || ok || none.Sittings != 0 {
		t.Fatalf("empty placement %+v ok=%v err=%v", none, ok, err)
	}

	if _, err := raw.ExecContext(ctx, `
		INSERT INTO units (position, title, situation, concept, status, rev, doc)
		VALUES (4, 'Later', 's', 'c', 'draft', 1, '{"title":"Later","newWords":["דָּג"]}')`); err != nil {
		t.Fatal(err)
	}
	started := when.Add(-time.Hour)
	runID, err := raw.InsertPlacementRun(ctx, userID, started, 7)
	if err != nil {
		t.Fatal(err)
	}
	word := mustLexeme(t, raw, hello)
	for i := 0; i < 4; i++ {
		ratePlacement(t, raw, runID, "card-"+strconv.Itoa(i), db.Good, word, started)
	}
	if err := raw.AdvancePlacementSitting(ctx, runID); err != nil {
		t.Fatal(err)
	}
	const result = `{"bands":[{"band":0,"rated":4,"again":1,"hard":1,"good":1,"easy":1,"dont_know":0,"good_plus_rate":0.5}],"estimated_known_words":40}`
	finished := when
	if err := raw.FinishPlacement(ctx, runID, finished, 2, "cap_cards", result); err != nil {
		t.Fatal(err)
	}

	row, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if !row.Finished || row.Sittings != 2 || !row.TakenAt.Equal(finished) || row.Level != 2 || row.CardsRated != 4 || row.StopReason != "cap_cards" {
		t.Fatalf("placement %+v", row)
	}
	if row.EstimatedKnownWords != 40 || row.SuggestedStartPosition != 4 {
		t.Fatalf("estimate %+v", row)
	}
	if len(row.BandScores) != 1 || row.BandScores[0].Band != 0 || row.BandScores[0].Rated != 4 || row.BandScores[0].Again != 1 || row.BandScores[0].Hard != 1 || row.BandScores[0].Good != 1 || row.BandScores[0].Easy != 1 || row.BandScores[0].DontKnow != 0 || row.BandScores[0].GoodPlusRate != 0.5 {
		t.Fatalf("bands %+v", row.BandScores)
	}

	otherID := mustOtherUser(t, raw)
	otherRun, err := raw.InsertPlacementRun(ctx, otherID, when.Add(time.Hour), 99)
	if err != nil {
		t.Fatal(err)
	}
	ratePlacement(t, raw, otherRun, "house", db.Easy, mustLexeme(t, raw, house), when)
	if err := raw.FinishPlacement(ctx, otherRun, when.Add(time.Hour), 4, "exhausted", `{"bands":[],"estimated_known_words":7}`); err != nil {
		t.Fatal(err)
	}
	again, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		t.Fatalf("after other ok=%v err=%v", ok, err)
	}
	if !again.Finished || again.Sittings != 2 || again.Level != 2 || again.StopReason != "cap_cards" || again.CardsRated != 4 || again.EstimatedKnownWords != 40 {
		t.Fatalf("learner A saw B's placement %+v", again)
	}
	other, ok, err := store.LatestPlacement(ctx, otherID)
	if err != nil || !ok || other.StopReason != "exhausted" || other.Level != 4 || other.Sittings != 1 || other.EstimatedKnownWords != 7 {
		t.Fatalf("learner B placement %+v ok=%v err=%v", other, ok, err)
	}
}

func TestLatestPlacementInProgress(t *testing.T) {
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, raw := openAdapter(t, fixedClock{when})
	userID := mustUser(t, raw)
	runID, err := raw.InsertPlacementRun(ctx, userID, when, 3)
	if err != nil {
		t.Fatal(err)
	}
	ratePlacement(t, raw, runID, "card-1", db.Good, mustLexeme(t, raw, hello), when)
	row, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if row.Finished || row.Sittings != 1 || row.CardsRated != 1 || row.Level != -1 || row.StopReason != "" || !row.TakenAt.Equal(when) {
		t.Fatalf("in progress %+v", row)
	}
	if err := raw.AdvancePlacementSitting(ctx, runID); err != nil {
		t.Fatal(err)
	}
	row, ok, err = store.LatestPlacement(ctx, userID)
	if err != nil || !ok || row.Sittings != 2 {
		t.Fatalf("sitting %+v ok=%v err=%v", row, ok, err)
	}

	finishedAt := when.Add(time.Minute)
	if err := raw.FinishPlacement(ctx, runID, finishedAt, 0, "band_stop", `{"bands":[],"estimated_known_words":0}`); err != nil {
		t.Fatal(err)
	}
	retakeAt := finishedAt.Add(time.Minute)
	if _, err := raw.InsertPlacementRun(ctx, userID, retakeAt, 9); err != nil {
		t.Fatal(err)
	}
	again, ok, err := store.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		t.Fatalf("retake ok=%v err=%v", ok, err)
	}
	if again.Finished || again.Sittings != 1 || !again.TakenAt.Equal(retakeAt) || again.Level != -1 || again.StopReason != "" {
		t.Fatalf("retake %+v", again)
	}
}

func TestPlacementLexemeLists(t *testing.T) {
	when := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, raw := openAdapter(t, fixedClock{when})
	userID := mustUser(t, raw)
	knownID := mustLexeme(t, raw, coffee)
	learnID := mustLexeme(t, raw, hello)
	otherID := mustLexeme(t, raw, book)
	older, err := raw.InsertPlacementRun(ctx, userID, when, 1)
	if err != nil {
		t.Fatal(err)
	}
	ratePlacement(t, raw, older, "old-book", db.Easy, otherID, when)
	if err := raw.FinishPlacement(ctx, older, when, 0, "exhausted", `{"bands":[],"estimated_known_words":0}`); err != nil {
		t.Fatal(err)
	}
	latest, err := raw.InsertPlacementRun(ctx, userID, when.Add(time.Minute), 2)
	if err != nil {
		t.Fatal(err)
	}
	ratePlacement(t, raw, latest, "coffee", db.Good, knownID, when)
	ratePlacement(t, raw, latest, "hello", db.Again, learnID, when)

	gotKnown, err := store.PlacementKnownLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	wantKnown, err := raw.PlacementKnownLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(gotKnown, "\n") != strings.Join(wantKnown, "\n") || len(gotKnown) != 1 || gotKnown[0] != coffee {
		t.Fatalf("known %+v store %+v", gotKnown, wantKnown)
	}
	gotLearn, err := store.PlacementLearningLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotLearn) != 1 || gotLearn[0] != hello {
		t.Fatalf("learning %+v", gotLearn)
	}

	userB := mustOtherUser(t, raw)
	houseID := mustLexeme(t, raw, house)
	otherRun, err := raw.InsertPlacementRun(ctx, userB, when.Add(2*time.Minute), 3)
	if err != nil {
		t.Fatal(err)
	}
	ratePlacement(t, raw, otherRun, "house", db.Easy, houseID, when)
	otherKnown, err := store.PlacementKnownLexemes(ctx, userB)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherKnown) != 1 || otherKnown[0] != house {
		t.Fatalf("learner B known %+v", otherKnown)
	}
	gotKnown, err = store.PlacementKnownLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotKnown) != 1 || gotKnown[0] != coffee || containsWord(gotKnown, house) {
		t.Fatalf("learner A known after B %+v", gotKnown)
	}
	gotLearn, err = store.PlacementLearningLexemes(ctx, userID)
	if err != nil || len(gotLearn) != 1 || gotLearn[0] != hello || containsWord(gotLearn, house) {
		t.Fatalf("learner A learning after B %+v err=%v", gotLearn, err)
	}
}

func TestStruggleStats(t *testing.T) {
	when := time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, raw := openAdapter(t, fixedClock{when})
	userID := mustUser(t, raw)
	recent := when.Add(-time.Hour)
	old := when.Add(-9 * 24 * time.Hour)

	alefID := mustLexeme(t, raw, "א")
	betID := mustLexeme(t, raw, "ב")
	gimelID := mustLexeme(t, raw, "ג")
	alef := mustCard(t, raw, userID, alefID, 9)
	bet := mustCard(t, raw, userID, betID, 0)
	gimel := mustCard(t, raw, userID, gimelID, 3)
	mustGloss(t, raw, "א", "alef")
	mustGloss(t, raw, "ב", "bet")
	mustGloss(t, raw, "ג", "gimel")
	insertReviews(t, raw, alef, old, 1, 1)
	insertReviews(t, raw, bet, recent, 1, 1, 3, 3)
	insertReviews(t, raw, gimel, recent, 1, 1, 3, 3, 3, 3, 3, 3, 3, 3)

	// The key text mentions א and ג. It is stored with no lexeme, so it must
	// not land on either word. Gimel's key is ב/כ, which does not contain ג.
	if err := raw.InsertConfusionMiss(ctx, userID, "letter_pair", "ג/א", 0, recent); err != nil {
		t.Fatal(err)
	}
	if err := raw.InsertConfusionMiss(ctx, userID, "letter_pair", "ב/כ", gimelID, recent); err != nil {
		t.Fatal(err)
	}
	if err := raw.InsertConfusionMiss(ctx, userID, "letter_pair", "ב/כ", gimelID, recent.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := raw.InsertConfusionMiss(ctx, userID, "agreement", "א", 0, recent); err != nil {
		t.Fatal(err)
	}
	if err := raw.InsertConfusionMiss(ctx, userID, "letter_pair", "ב/כ", gimelID, old); err != nil {
		t.Fatal(err)
	}
	otherID := mustOtherUser(t, raw)
	zayinID := mustLexeme(t, raw, "ז")
	mustGloss(t, raw, "ז", "zayin")
	zayin := mustCard(t, raw, otherID, zayinID, 4)
	insertReviews(t, raw, zayin, recent, 1, 1, 1)
	if err := raw.InsertConfusionMiss(ctx, otherID, "letter_pair", "ז/ק", zayinID, recent); err != nil {
		t.Fatal(err)
	}

	stats, err := store.StruggleStats(ctx, userID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Lexemes) != 3 || stats.Lexemes[0].TextNFC != "א" || stats.Lexemes[1].TextNFC != "ב" || stats.Lexemes[2].TextNFC != "ג" {
		t.Fatalf("sorted %+v", stats.Lexemes)
	}
	if len(stats.Top) != 1 || stats.Top[0] != "ג" {
		t.Fatalf("top %+v", stats.Top)
	}
	gimelRow := stats.Lexemes[2]
	if gimelRow.Gloss != "gimel" || gimelRow.Reviews != 10 || gimelRow.Lapses != 3 || gimelRow.Misses7d != 2 || gimelRow.AgainRate != float64(2)/float64(10) || !gimelRow.LastSeen.Equal(recent) {
		t.Fatalf("gimel %+v", gimelRow)
	}
	if len(gimelRow.ConfusionKeys) != 1 || gimelRow.ConfusionKeys[0] != "ב/כ" {
		t.Fatalf("keys %+v", gimelRow.ConfusionKeys)
	}
	if len(stats.Lexemes[0].ConfusionKeys) != 0 {
		t.Fatalf("alef keys %+v", stats.Lexemes[0].ConfusionKeys)
	}
	if len(stats.Confusions) != 3 {
		t.Fatalf("confusions %+v", stats.Confusions)
	}
	var sawAgreement, sawGimel, sawBet bool
	for _, row := range stats.Confusions {
		switch {
		case row.Kind == "agreement" && row.A == "א" && row.B == "" && row.Count == 1:
			sawAgreement = true
		case row.Kind == "letter_pair" && row.A == "ג" && row.B == "א" && row.Count == 1:
			sawGimel = true
		case row.Kind == "letter_pair" && row.A == "ב" && row.B == "כ" && row.Count == 2:
			sawBet = true
		}
	}
	if !sawAgreement || !sawGimel || !sawBet {
		t.Fatalf("confusions %+v", stats.Confusions)
	}
	if len(stats.Lexemes[1].ConfusionKeys) != 0 {
		t.Fatalf("bet keys %+v", stats.Lexemes[1].ConfusionKeys)
	}
	for _, row := range stats.Lexemes {
		if row.TextNFC == "ז" {
			t.Fatalf("learner A saw B's lexeme %+v", stats.Lexemes)
		}
	}
	for _, row := range stats.Confusions {
		if row.A == "ז" || row.B == "ז" || row.A == "ק" {
			t.Fatalf("learner A saw B's confusion %+v", stats.Confusions)
		}
	}
	other, err := store.StruggleStats(ctx, otherID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(other.Top) != 1 || other.Top[0] != "ז" || len(other.Lexemes) != 1 || other.Lexemes[0].TextNFC != "ז" {
		t.Fatalf("learner B struggle %+v", other)
	}
	for _, row := range other.Lexemes {
		if row.TextNFC == "א" || row.TextNFC == "ב" || row.TextNFC == "ג" {
			t.Fatalf("learner B saw A's lexeme %+v", other.Lexemes)
		}
	}
}

func TestCompletedSharedUnits(t *testing.T) {
	when := time.Date(2026, 10, 18, 8, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, raw := openAdapter(t, fixedClock{when})
	userID := mustUser(t, raw)
	earlyID, err := testharness.InsertUnit(ctx, raw, "Early", "published")
	if err != nil {
		t.Fatal(err)
	}
	lateID, err := testharness.InsertUnit(ctx, raw, "Late", "published")
	if err != nil {
		t.Fatal(err)
	}
	openID, err := testharness.InsertUnit(ctx, raw, "Open", "published")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `UPDATE units SET position = 0, doc = '{"title":"Early","lessons":[{}]}' WHERE id = ?`, earlyID); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `UPDATE units SET position = 2, doc = '{"title":"Late","lessons":[{},{}]}' WHERE id = ?`, lateID); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `UPDATE units SET position = 1, doc = '{"title":"Open","lessons":[{}]}' WHERE id = ?`, openID); err != nil {
		t.Fatal(err)
	}
	bookID := mustLexeme(t, raw, book)
	coffeeID := mustLexeme(t, raw, coffee)
	fishID := mustLexeme(t, raw, fish)
	if err := testharness.InsertLessonItem(ctx, raw, earlyID, 0, bookID, "new", "recall"); err != nil {
		t.Fatal(err)
	}
	if err := testharness.InsertLessonItem(ctx, raw, lateID, 0, coffeeID, "new", "recall"); err != nil {
		t.Fatal(err)
	}
	if err := testharness.InsertLessonItem(ctx, raw, lateID, 1, coffeeID, "recycle", "produce"); err != nil {
		t.Fatal(err)
	}
	if err := testharness.InsertLessonItem(ctx, raw, openID, 0, fishID, "new", "recall"); err != nil {
		t.Fatal(err)
	}
	earlyDone := when.Add(-2 * time.Hour)
	lateLesson0 := when.Add(-time.Hour)
	lateLesson1 := when
	if _, err := testharness.InsertLessonRun(ctx, raw, userID, earlyID, 0, earlyDone.Add(-time.Minute), &earlyDone); err != nil {
		t.Fatal(err)
	}
	if _, err := testharness.InsertLessonRun(ctx, raw, userID, lateID, 0, lateLesson0.Add(-time.Minute), &lateLesson0); err != nil {
		t.Fatal(err)
	}
	if _, err := testharness.InsertLessonRun(ctx, raw, userID, lateID, 1, lateLesson1.Add(-time.Minute), &lateLesson1); err != nil {
		t.Fatal(err)
	}
	if _, err := testharness.InsertLessonRun(ctx, raw, userID, openID, 0, when, nil); err != nil {
		t.Fatal(err)
	}
	custom := insertUnitRow(t, raw, "Custom", "learner:7", "draft", 0, house)
	if _, err := testharness.InsertLessonRun(ctx, raw, userID, custom, 0, when, &when); err != nil {
		t.Fatal(err)
	}

	rows, err := store.CompletedSharedUnits(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].UnitID != earlyID || rows[0].Position != 0 || !rows[0].CompletedAt.Equal(earlyDone) || rows[1].UnitID != lateID || rows[1].Position != 2 || !rows[1].CompletedAt.Equal(lateLesson1) {
		t.Fatalf("completed %+v", rows)
	}
}

func TestLearnerKnownLexemes(t *testing.T) {
	when := time.Date(2026, 10, 18, 8, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, raw := openAdapter(t, fixedClock{when})
	userID := mustUser(t, raw)
	helloID := mustLexeme(t, raw, hello)
	coffeeID := mustLexeme(t, raw, coffee)
	bookID := mustLexeme(t, raw, book)
	fishID := mustLexeme(t, raw, fish)
	houseID := mustLexeme(t, raw, house)

	helloCard := mustCard(t, raw, userID, helloID, 0)
	insertReviews(t, raw, helloCard, when.Add(-time.Hour), 1)
	insertReviews(t, raw, helloCard, when, 4)
	coffeeCard := mustCard(t, raw, userID, coffeeID, 0)
	insertReviews(t, raw, coffeeCard, when, 1)
	enHe := db.Card{UserID: userID, LexemeID: coffeeID, Dir: "en_he", State: "review", Due: when, LastReview: when}
	enHeID, err := raw.InsertCard(ctx, enHe)
	if err != nil {
		t.Fatal(err)
	}
	insertReviews(t, raw, enHeID, when, 4)

	doneID, err := testharness.InsertUnit(ctx, raw, "Done", "published")
	if err != nil {
		t.Fatal(err)
	}
	openID, err := testharness.InsertUnit(ctx, raw, "Open", "published")
	if err != nil {
		t.Fatal(err)
	}
	if err := testharness.InsertLessonItem(ctx, raw, doneID, 0, bookID, "new", "recall"); err != nil {
		t.Fatal(err)
	}
	if err := testharness.InsertLessonItem(ctx, raw, openID, 0, fishID, "new", "recall"); err != nil {
		t.Fatal(err)
	}
	if _, err := testharness.InsertLessonRun(ctx, raw, userID, doneID, 0, when, &when); err != nil {
		t.Fatal(err)
	}
	if _, err := testharness.InsertLessonRun(ctx, raw, userID, openID, 0, when, nil); err != nil {
		t.Fatal(err)
	}
	custom := insertUnitRow(t, raw, "Custom", "learner:7", "draft", 3, house)
	if err := testharness.InsertLessonItem(ctx, raw, custom, 0, houseID, "new", "recall"); err != nil {
		t.Fatal(err)
	}
	if _, err := testharness.InsertLessonRun(ctx, raw, userID, custom, 0, when, &when); err != nil {
		t.Fatal(err)
	}

	got, err := store.LearnerKnownLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != house+","+book+","+hello {
		t.Fatalf("known %+v", got)
	}

	otherID := mustOtherUser(t, raw)
	fishCard := mustCard(t, raw, otherID, fishID, 0)
	insertReviews(t, raw, fishCard, when, 4)
	other, err := store.LearnerKnownLexemes(ctx, otherID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(other, ",") != fish {
		t.Fatalf("learner B known %+v", other)
	}
	got, err = store.LearnerKnownLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != house+","+book+","+hello || containsWord(got, fish) {
		t.Fatalf("learner A known after B %+v", got)
	}
}

func TestSearchLexemes(t *testing.T) {
	ctx := context.Background()
	store, raw := openAdapter(t, nil)
	helloID := mustLexeme(t, raw, hello)
	if _, err := raw.ExecContext(ctx, `
		UPDATE lexemes
		SET bare = 'שלום', gloss_en = 'hello; peace', pos = 'interjection', gender = 'm', translit = 'shalom', root = 'שלם'
		WHERE id = ?`, helloID); err != nil {
		t.Fatal(err)
	}
	wholeID := mustLexeme(t, raw, whole)
	if _, err := raw.ExecContext(ctx, `UPDATE lexemes SET bare = 'שלם', gloss_en = 'whole' WHERE id = ?`, wholeID); err != nil {
		t.Fatal(err)
	}
	coffeeID := mustLexeme(t, raw, coffee)
	if _, err := raw.ExecContext(ctx, `
		UPDATE lexemes SET bare = 'קפה', gloss_en = 'coffee', gender = 'm', plural = 'קָפִים', irregular_plural = 1, translit = 'kafe'
		WHERE id = ?`, coffeeID); err != nil {
		t.Fatal(err)
	}
	audioID := strings.Repeat("ab", 32)
	if _, err := raw.ExecContext(ctx, `
		INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at)
		VALUES (?, ?, 'voice', 'eleven_v3', 4, ?)`, audioID, hello, db.FormatTime(time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Fatal(err)
	}

	hits, suggestions, err := store.SearchLexemes(ctx, "Peace", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(suggestions) != 0 || len(hits) != 1 || hits[0].TextNFC != hello || hits[0].Gloss != "hello; peace" || hits[0].Gender != "m" || hits[0].Translit != "shalom" || hits[0].Plural != "" || !hits[0].AudioPresent || hits[0].Root != "שלם" || hits[0].Bare != "שלום" {
		t.Fatalf("gloss hit %+v suggestions %+v", hits, suggestions)
	}

	hits, suggestions, err = store.SearchLexemes(ctx, "של", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 || len(suggestions) != 0 || hits[0].TextNFC != whole || hits[1].TextNFC != hello {
		t.Fatalf("prefix %+v suggestions %+v", hits, suggestions)
	}
	limited, _, err := store.SearchLexemes(ctx, "של", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 || limited[0].TextNFC != whole {
		t.Fatalf("limit %+v", limited)
	}

	hits, suggestions, err = store.SearchLexemes(ctx, "שלוכ", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 || len(suggestions) != 1 || suggestions[0].TextNFC != hello {
		t.Fatalf("suggestion hits %+v suggestions %+v", hits, suggestions)
	}

	coffeeHits, _, err := store.SearchLexemes(ctx, coffee, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(coffeeHits) != 1 || coffeeHits[0].Plural != "קָפִים" || coffeeHits[0].Translit != "kafe" || coffeeHits[0].AudioPresent {
		t.Fatalf("coffee %+v", coffeeHits)
	}
}

// TestPutUnitIdempotentScopeAndItems calls mcp.UnitWriter.PutUnit.
func TestPutUnitIdempotentScopeAndItems(t *testing.T) {
	when := time.Date(2026, 10, 18, 11, 0, 0, 0, time.UTC)
	store, raw := openAdapter(t, fixedClock{when})
	helloID := mustLexeme(t, raw, hello)
	ctx := withActor(t, "author-7")
	doc := unitDoc("Hello", hello)
	doc.Lessons = []units.Lesson{
		{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityListenPick, Text: hello},
			{Type: units.ActivityRecall, Text: hello},
		}},
		{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityProduce, Text: hello + " לֹא"},
		}},
	}
	doc.NewWords = []string{hello, "לֹא"}

	first, err := store.PutUnit(ctx, nil, &doc, mcp.ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != 1 || first.Unchanged || first.UnitID == 0 {
		t.Fatalf("create %+v", first)
	}
	second, err := store.PutUnit(ctx, &first.UnitID, &doc, " shared ")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Unchanged || second.Revision != 1 || second.UnitID != first.UnitID {
		t.Fatalf("idempotent %+v", second)
	}
	if _, err := raw.ExecContext(ctx, `UPDATE units SET status = 'published' WHERE id = ?`, first.UnitID); err != nil {
		t.Fatal(err)
	}
	held, err := store.PutUnit(ctx, &first.UnitID, &doc, mcp.ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	if !held.Unchanged || held.Revision != 1 || unitStatus(t, raw, first.UnitID) != "published" {
		t.Fatalf("published hold %+v status %s", held, unitStatus(t, raw, first.UnitID))
	}

	retarget, err := store.PutUnit(withActor(t, "author-9"), &first.UnitID, &doc, "learner:7")
	if err != nil {
		t.Fatal(err)
	}
	if retarget.Unchanged || retarget.Revision != 2 || unitStatus(t, raw, first.UnitID) != "draft" {
		t.Fatalf("scope change %+v status %s", retarget, unitStatus(t, raw, first.UnitID))
	}

	revs := revisions(t, raw, first.UnitID)
	if len(revs) != 2 || revs[0].actor != "author-7" || revs[1].actor != "author-9" || revs[0].doc != revs[1].doc || revs[0].at != db.FormatTime(when) || revs[1].at != db.FormatTime(when) {
		t.Fatalf("revisions %+v", revs)
	}
	if scope := unitScope(t, raw, first.UnitID); scope != "learner:7" {
		t.Fatalf("scope = %s", scope)
	}
	items := lessonItems(t, raw, first.UnitID)
	if len(items) != 2 || items[0].lesson != 0 || items[0].lexeme != helloID || items[0].role != "new" || items[0].skill != units.ActivityListenPick || items[1].lesson != 1 || items[1].role != "recycle" || items[1].skill != units.ActivityProduce {
		t.Fatalf("items %+v", items)
	}

	missing := int64(999)
	_, err = store.PutUnit(ctx, &missing, &doc, mcp.ScopeShared)
	if !errors.Is(err, mcp.ErrUnitNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	blocked := doc
	blocked.Title = ""
	saved, err := store.PutUnit(ctx, nil, &blocked, mcp.ScopeShared)
	if err != nil || saved.UnitID == 0 || saved.Revision != 1 {
		t.Fatalf("blocked draft %+v err=%v", saved, err)
	}
}

// TestPutUnitPublishedEditAndUnchanged calls mcp.UnitWriter.PutUnit.
// An unchanged put leaves a published unit published with no new
// revision. An edit sets it back to draft.
func TestPutUnitPublishedEditAndUnchanged(t *testing.T) {
	when := time.Date(2026, 10, 18, 11, 0, 0, 0, time.UTC)
	store, raw := openAdapter(t, fixedClock{when})
	ctx := withActor(t, "author-7")
	doc := unitDoc("Hello", hello)

	first, err := store.PutUnit(ctx, nil, &doc, mcp.ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `UPDATE units SET status = 'published' WHERE id = ?`, first.UnitID); err != nil {
		t.Fatal(err)
	}
	held, err := store.PutUnit(ctx, &first.UnitID, &doc, mcp.ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	if !held.Unchanged || held.Revision != 1 || unitStatus(t, raw, first.UnitID) != "published" {
		t.Fatalf("unchanged %+v status %s", held, unitStatus(t, raw, first.UnitID))
	}
	if n := len(revisions(t, raw, first.UnitID)); n != 1 {
		t.Fatalf("unchanged revisions = %d, want 1", n)
	}

	doc.Title = "Hello again"
	edited, err := store.PutUnit(ctx, &first.UnitID, &doc, mcp.ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	if edited.Unchanged || edited.Revision != 2 || edited.UnitID != first.UnitID || unitStatus(t, raw, first.UnitID) != "draft" {
		t.Fatalf("edited %+v status %s", edited, unitStatus(t, raw, first.UnitID))
	}
	if n := len(revisions(t, raw, first.UnitID)); n != 2 {
		t.Fatalf("edited revisions = %d, want 2", n)
	}
}

func TestLookup(t *testing.T) {
	ctx := context.Background()
	store, raw := openAdapter(t, nil)
	helloID := mustLexeme(t, raw, hello)
	if _, err := raw.ExecContext(ctx, `
		UPDATE lexemes SET pos = 'interjection', gender = 'm', bare = 'שלום', plural = NULL, irregular_plural = 0
		WHERE id = ?`, helloID); err != nil {
		t.Fatal(err)
	}
	coffeeID := mustLexeme(t, raw, coffee)
	if _, err := raw.ExecContext(ctx, `
		UPDATE lexemes SET pos = 'noun', gender = 'm', plural = 'קָפִים', irregular_plural = 1, bare = 'קפה'
		WHERE id = ?`, coffeeID); err != nil {
		t.Fatal(err)
	}
	audioID := strings.Repeat("cd", 32)
	if _, err := raw.ExecContext(ctx, `
		INSERT INTO audio_assets (id, text_nfc, voice, model, bytes, created_at)
		VALUES (?, ?, 'voice', 'eleven_v3', 4, ?)`, audioID, hello, db.FormatTime(time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Fatal(err)
	}

	id, ok, err := store.LexemeByText(ctx, hello)
	if err != nil || !ok || id != helloID {
		t.Fatalf("lexeme id=%d ok=%v err=%v", id, ok, err)
	}
	if _, ok, err := store.LexemeByText(ctx, "missing"); err != nil || ok {
		t.Fatalf("missing ok=%v err=%v", ok, err)
	}
	has, err := store.HasAudio(ctx, hello)
	if err != nil || !has {
		t.Fatalf("audio %v %v", has, err)
	}
	has, err = store.HasAudio(ctx, coffee)
	if err != nil || has {
		t.Fatalf("coffee audio %v %v", has, err)
	}
	facts, ok, err := store.Facts(ctx, coffee)
	if err != nil || !ok || facts.POS != "noun" || facts.Gender != "m" || facts.Plural != "קָפִים" || !facts.IrregularPlural || facts.Bare != "קפה" {
		t.Fatalf("facts %+v ok=%v err=%v", facts, ok, err)
	}
	if _, ok, err := store.Facts(ctx, "missing"); err != nil || ok {
		t.Fatalf("missing facts ok=%v err=%v", ok, err)
	}

	insertUnitRow(t, raw, "One", mcp.ScopeShared, "draft", 0, hello)
	insertUnitRow(t, raw, "Two", mcp.ScopeShared, "draft", 3, coffee)
	known, err := store.KnownLexemes(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(known) != 1 || !known[hello] {
		t.Fatalf("before 1 %+v", known)
	}
	none, err := store.KnownLexemes(ctx, 0)
	if err != nil || len(none) != 0 {
		t.Fatalf("before 0 %+v err=%v", none, err)
	}
	both, err := store.KnownLexemes(ctx, 4)
	if err != nil || !both[hello] || !both[coffee] {
		t.Fatalf("before 4 %+v err=%v", both, err)
	}
}

func TestKnownLexemesSharedOnly(t *testing.T) {
	ctx := context.Background()
	store, raw := openAdapter(t, nil)
	insertUnitRow(t, raw, "Private", "learner:8", "draft", 0, fish)
	insertUnitRow(t, raw, "SharedHello", mcp.ScopeShared, "draft", 0, hello)

	privateOnly, err := store.KnownLexemes(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if privateOnly[fish] || !privateOnly[hello] || len(privateOnly) != 1 {
		t.Fatalf("before shared fish %+v", privateOnly)
	}
	if issues := units.Validate(ctx, storyUnit(2, fish), store); !hasRule(issues, units.RuleCoverageStory) {
		t.Fatalf("private fish counted as known: %+v", issues)
	}
	if issues := units.Validate(ctx, storyUnit(2, hello), store); hasRule(issues, units.RuleCoverageStory) {
		t.Fatalf("shared hello missing from coverage: %+v", issues)
	}

	insertUnitRow(t, raw, "SharedFish", mcp.ScopeShared, "draft", 1, fish)
	shared, err := store.KnownLexemes(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !shared[fish] || !shared[hello] {
		t.Fatalf("shared fish %+v", shared)
	}
	if issues := units.Validate(ctx, storyUnit(2, fish), store); hasRule(issues, units.RuleCoverageStory) {
		t.Fatalf("shared fish missing from coverage: %+v", issues)
	}
}

func storyUnit(position int, word string) *units.Unit {
	return &units.Unit{
		Position:  position,
		Title:     "Check",
		Situation: "Street",
		Concept:   "One idea",
		Lessons: []units.Lesson{{
			Kind: units.KindLesson,
			Activities: []units.Activity{{
				Type: units.ActivityStory,
				Text: word,
			}},
		}},
	}
}

func hasRule(issues []units.Issue, rule string) bool {
	for _, iss := range issues {
		if iss.Rule == rule {
			return true
		}
	}
	return false
}

func containsWord(words []string, word string) bool {
	for _, w := range words {
		if w == word {
			return true
		}
	}
	return false
}

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

func openAdapter(t *testing.T, clk clock.Clock) (*Store, *db.Store) {
	t.Helper()
	raw, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	return New(raw), raw
}

func mustUser(t *testing.T, raw *db.Store) int64 {
	t.Helper()
	return insertUser(t, raw, "learner@example.com", "sub-learner")
}

func mustOtherUser(t *testing.T, raw *db.Store) int64 {
	t.Helper()
	return insertUser(t, raw, "other@example.com", "sub-other")
}

func insertUser(t *testing.T, raw *db.Store, email, sub string) int64 {
	t.Helper()
	id, err := raw.InsertUser(context.Background(), email, sub, "learner")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustLexeme(t *testing.T, raw *db.Store, text string) int64 {
	t.Helper()
	id, err := testharness.InsertLexeme(context.Background(), raw, text)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustGloss(t *testing.T, raw *db.Store, text, gloss string) {
	t.Helper()
	if _, err := raw.ExecContext(context.Background(), `UPDATE lexemes SET gloss_en = ? WHERE text_nfc = ?`, gloss, text); err != nil {
		t.Fatal(err)
	}
}

func mustCard(t *testing.T, raw *db.Store, userID, lexemeID int64, lapses int) int64 {
	t.Helper()
	when := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	id, err := raw.InsertCard(context.Background(), db.Card{
		UserID: userID, LexemeID: lexemeID, Dir: "he_en", State: "review",
		Due: when, LastReview: when, Lapses: lapses, Reps: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertReviews(t *testing.T, raw *db.Store, cardID int64, at time.Time, ratings ...int) {
	t.Helper()
	for _, rating := range ratings {
		if _, err := raw.ExecContext(context.Background(), `
			INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days)
			VALUES (?, ?, ?, 0, 1)`, cardID, rating, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}
}

func unitDoc(title, word string) units.Unit {
	return units.Unit{
		Title: title, Situation: "Street", Concept: "One idea",
		NewWords: []string{word},
		Lessons: []units.Lesson{{
			Kind:       units.KindLesson,
			Activities: []units.Activity{{Type: units.ActivityRecall, Text: word}},
		}},
	}
}

func insertUnitRow(t *testing.T, raw *db.Store, title, scope, status string, position int, word string) int64 {
	t.Helper()
	doc, err := json.Marshal(map[string]any{
		"title":    title,
		"newWords": []string{word},
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := raw.ExecContext(context.Background(), `
		INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
		VALUES (?, ?, 'Street', 'One idea', ?, 1, ?, ?)`,
		position, title, status, string(doc), scope)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func ids(rows []mcp.UnitOutline) string {
	parts := make([]string, len(rows))
	for i, row := range rows {
		parts[i] = strconv.FormatInt(row.ID, 10)
	}
	return strings.Join(parts, ",")
}

func joinIDs(ids ...int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}

type revisionRow struct {
	actor string
	doc   string
	at    string
}

func revisions(t *testing.T, raw *db.Store, unitID int64) []revisionRow {
	t.Helper()
	conn, err := raw.DB()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := conn.QueryContext(context.Background(), `
		SELECT actor, doc, created_at FROM unit_revisions
		WHERE unit_id = ? ORDER BY rev`, unitID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []revisionRow
	for rows.Next() {
		var row revisionRow
		if err := rows.Scan(&row.actor, &row.doc, &row.at); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func ratePlacement(t *testing.T, raw *db.Store, runID int64, item string, rating int, lexemeID int64, when time.Time) {
	t.Helper()
	ctx := context.Background()
	if err := raw.ShowPlacementCard(ctx, runID, item, 1, when); err != nil {
		t.Fatal(err)
	}
	if err := raw.RatePlacementAnswer(ctx, runID, item, rating, lexemeID, when); err != nil {
		t.Fatal(err)
	}
}

func unitScope(t *testing.T, raw *db.Store, unitID int64) string {
	t.Helper()
	conn, err := raw.DB()
	if err != nil {
		t.Fatal(err)
	}
	var scope string
	if err := conn.QueryRowContext(context.Background(), `SELECT scope FROM units WHERE id = ?`, unitID).Scan(&scope); err != nil {
		t.Fatal(err)
	}
	return scope
}

func unitStatus(t *testing.T, raw *db.Store, unitID int64) string {
	t.Helper()
	conn, err := raw.DB()
	if err != nil {
		t.Fatal(err)
	}
	var status string
	if err := conn.QueryRowContext(context.Background(), `SELECT status FROM units WHERE id = ?`, unitID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

type itemRow struct {
	lesson int
	lexeme int64
	role   string
	skill  string
}

func lessonItems(t *testing.T, raw *db.Store, unitID int64) []itemRow {
	t.Helper()
	conn, err := raw.DB()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := conn.QueryContext(context.Background(), `
		SELECT lesson_ix, lexeme_id, role, skill FROM lesson_items
		WHERE unit_id = ? ORDER BY lesson_ix, lexeme_id`, unitID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []itemRow
	for rows.Next() {
		var row itemRow
		if err := rows.Scan(&row.lesson, &row.lexeme, &row.role, &row.skill); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func withActor(t *testing.T, actor string) context.Context {
	t.Helper()
	var ctx context.Context
	h := sdauth.RequireBearerToken(func(context.Context, string, *http.Request) (*sdauth.TokenInfo, error) {
		return &sdauth.TokenInfo{
			UserID:     actor,
			Expiration: time.Now().Add(time.Hour),
			Extra:      map[string]any{"source": "token"},
		}, nil
	}, &sdauth.RequireBearerTokenOptions{AllowMissingExpiration: true})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if ctx == nil || rr.Code != http.StatusNoContent {
		t.Fatalf("principal context status %d", rr.Code)
	}
	return ctx
}
