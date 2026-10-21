package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"golang.org/x/text/unicode/norm"
)

// TestLatestPlacementV3ReadsAnsweredItems is a finished v3 row shaped like the
// production read: placement_answers is empty, so a count of that table is 0,
// and the stored result JSON has five all-zero bands. The answered
// placement_items are the real record. cardsRated, the band tallies, and the
// known and learning words come back from those items with no migration.
func TestLatestPlacementV3ReadsAnsweredItems(t *testing.T) {
	when := time.Date(2026, 10, 19, 17, 23, 6, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "v3-read@example.com", "sub-v3-read", "learner")

	runID, err := s.StartPlacementV3(ctx, userID, when, 7, RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}

	// Dagesh before patah is not canonical order. The read returns pointed NFC.
	bayitStored := "ב\u05BC\u05B7י\u05B4ת"
	bayit := norm.NFC.String(bayitStored)
	if bayit == bayitStored {
		t.Fatal("stored bayit is already NFC; the check needs a non-canonical spelling")
	}
	const (
		echad  = "אֶחָד"
		geshem = "גֶּשֶׁם"
		delet  = "דֶּלֶת"
	)
	ids := map[string]int64{
		bayitStored: mustLexemeText(t, s, bayitStored),
		echad:       mustLexemeText(t, s, echad),
		geshem:      mustLexemeText(t, s, geshem),
		delet:       mustLexemeText(t, s, delet),
	}

	// A letters answer is the warm-up. It is not a word card.
	claimItem(t, s, runID, 1, PartLetters, "", 0, "alef", "right", 0, when)
	claimItem(t, s, runID, 2, PartListening, "listen", 1, "w-echad", "right", ids[echad], when)
	claimItem(t, s, runID, 3, PartListening, "listen", 1, "w-bayit", "right", ids[bayitStored], when)
	claimItem(t, s, runID, 4, PartReading, "read", 2, "w-delet", "wrong", ids[delet], when)
	claimItem(t, s, runID, 5, PartReading, "read", 3, "w-geshem", "dont_know", ids[geshem], when)
	claimItem(t, s, runID, 6, PartTyping, "recall", 1, "w-echad-type", "right", ids[echad], when)

	zeros := make([]BandScore, 5)
	for i := range zeros {
		zeros[i].Band = i
	}
	body, err := json.Marshal(placementResultJSON{Bands: zeros})
	if err != nil {
		t.Fatal(err)
	}
	// The stored row looks empty: no stop reason, level −1, zero bands, and
	// no placement_answers. The items above are the only record.
	if err := s.FinishPlacement(ctx, runID, when.Add(time.Minute), -1, "", string(body)); err != nil {
		t.Fatal(err)
	}
	if n, err := s.countAnswerRatings(ctx, runID); err != nil || n != 0 {
		t.Fatalf("placement_answers ratings = %d %v, want 0 so the fixture matches production", n, err)
	}

	place, ok, err := s.LatestPlacement(ctx, userID)
	if err != nil || !ok || !place.Finished {
		t.Fatalf("latest ok=%v finished=%v err=%v", ok, place.Finished, err)
	}
	if place.CardsRated != 5 {
		t.Fatalf("cardsRated=%d, want 5 answered word cards (the letter is not one)", place.CardsRated)
	}
	if len(place.BandScores) != 3 {
		t.Fatalf("bands %+v, want the three bands the word items used", place.BandScores)
	}
	if b := place.BandScores[0]; b.Band != 0 || b.Rated != 2 || b.Good != 2 || b.Again != 0 || b.DontKnow != 0 || b.GoodPlusRate != 1 {
		t.Fatalf("band 0 %+v, want 2 right", b)
	}
	if b := place.BandScores[1]; b.Band != 1 || b.Rated != 1 || b.Again != 1 || b.Good != 0 || b.GoodPlusRate != 0 {
		t.Fatalf("band 1 %+v, want 1 wrong", b)
	}
	if b := place.BandScores[2]; b.Band != 2 || b.Rated != 1 || b.DontKnow != 1 || b.Good != 0 || b.Again != 0 {
		t.Fatalf("band 2 %+v, want 1 don't-know", b)
	}

	known, err := s.PlacementKnownLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	wantKnown := []string{echad, bayit}
	if len(known) != len(wantKnown) {
		t.Fatalf("known %v, want %v", known, wantKnown)
	}
	for i := range wantKnown {
		if known[i] != wantKnown[i] {
			t.Fatalf("known %v, want %v", known, wantKnown)
		}
	}
	learning, err := s.PlacementLearningLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	wantLearning := []string{geshem, delet}
	if len(learning) != len(wantLearning) {
		t.Fatalf("learning %v, want %v", learning, wantLearning)
	}
	for i := range wantLearning {
		if learning[i] != wantLearning[i] {
			t.Fatalf("learning %v, want %v", learning, wantLearning)
		}
	}

	// A v2 row is still read from placement_answers, not from items.
	v2ID := insertUser(t, s, "v2-read@example.com", "sub-v2-read", "learner")
	v2Run, err := s.InsertPlacementRun(ctx, v2ID, when, 3)
	if err != nil {
		t.Fatal(err)
	}
	rateAnswer(t, s, v2Run, "card-v2", ids[echad], Good, when)
	v2, ok, err := s.LatestPlacement(ctx, v2ID)
	if err != nil || !ok || v2.CardsRated != 1 {
		t.Fatalf("v2 cardsRated=%d ok=%v err=%v", v2.CardsRated, ok, err)
	}
	v2Known, err := s.PlacementKnownLexemes(ctx, v2ID)
	if err != nil || len(v2Known) != 1 || v2Known[0] != echad {
		t.Fatalf("v2 known %v %v", v2Known, err)
	}
}

func mustLexemeText(t *testing.T, s *Store, text string) int64 {
	t.Helper()
	id, err := s.UpsertPlacementLexeme(context.Background(), text, "gloss", "noun", "x", "")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func claimItem(t *testing.T, s *Store, runID int64, seq int, part, skill string, band int, itemID, outcome string, lexemeID int64, at time.Time) {
	t.Helper()
	ctx := context.Background()
	it := PlacementItem{
		RunID: runID, Seq: seq, Part: part, Skill: skill, Band: band, ItemID: itemID,
		Options: `{"options":[]}`, ShownAt: at,
	}
	if err := s.InsertPlacementItem(ctx, it); err != nil {
		t.Fatal(err)
	}
	ok, err := s.ClaimPlacementItem(ctx, runID, seq, outcome, "", lexemeID, at, 0)
	if err != nil || !ok {
		t.Fatalf("claim %s: ok=%v err=%v", itemID, ok, err)
	}
}

// A fault during the finish rolls the snapshot and onboarded_at back together.
// A retry after the fault is gone writes finished_at, start_unit, words_known,
// and onboarded_at.
func TestFinishPlacementRollsBackTheSnapshotAndOnboardedAt(t *testing.T) {
	when := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "atomic@example.com", "sub-atomic", "learner")
	insertUnit(t, s, "published")
	runID, err := s.StartPlacementV3(ctx, user, when, 4, RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	claimItem(t, s, runID, 1, PartListening, "listen", 1, "w-atomic", "right", mustLexemeText(t, s, "אֶחָד"), when)

	mustExec(t, s, `CREATE TRIGGER abort_onboard BEFORE UPDATE ON users BEGIN SELECT RAISE(ABORT, 'injected'); END`)
	if err := s.FinishPlacement(ctx, runID, when.Add(time.Minute), 2, "", `{"bands":[{"band":0,"good":1}]}`); err == nil {
		t.Fatal("users update fault was ignored")
	}
	assertFinishUnset(t, s, user, runID)
	mustExec(t, s, `DROP TRIGGER abort_onboard`)

	mustExec(t, s, `CREATE TRIGGER abort_snapshot BEFORE UPDATE ON placement_runs WHEN NEW.words_known IS NOT NULL BEGIN SELECT RAISE(ABORT, 'injected'); END`)
	if err := s.FinishPlacement(ctx, runID, when.Add(time.Minute), 2, "", `{"bands":[{"band":0,"good":1}]}`); err == nil {
		t.Fatal("snapshot update fault was ignored")
	}
	assertFinishUnset(t, s, user, runID)
	mustExec(t, s, `DROP TRIGGER abort_snapshot`)

	if err := s.FinishPlacement(ctx, runID, when.Add(time.Minute), 2, "", `{"bands":[{"band":0,"good":1}]}`); err != nil {
		t.Fatal(err)
	}
	var finished string
	var startUnit, wordsKnown sql.NullInt64
	var onboarded sql.NullString
	if err := s.queryRow(ctx, `SELECT finished_at, start_unit, words_known FROM placement_runs WHERE id = ?`, runID).Scan(&finished, &startUnit, &wordsKnown); err != nil {
		t.Fatal(err)
	}
	if err := s.queryRow(ctx, `SELECT onboarded_at FROM users WHERE id = ?`, user).Scan(&onboarded); err != nil {
		t.Fatal(err)
	}
	if finished == "" || !startUnit.Valid || !wordsKnown.Valid || wordsKnown.Int64 < 1 || !onboarded.Valid {
		t.Fatalf("retry left finished=%q start=%v words=%v onboarded=%v", finished, startUnit, wordsKnown, onboarded)
	}
}

func assertFinishUnset(t *testing.T, s *Store, user, runID int64) {
	t.Helper()
	ctx := context.Background()
	var finished, startUnit, wordsKnown, onboarded sql.NullString
	if err := s.queryRow(ctx, `SELECT finished_at, start_unit, words_known FROM placement_runs WHERE id = ?`, runID).Scan(&finished, &startUnit, &wordsKnown); err != nil {
		t.Fatal(err)
	}
	if err := s.queryRow(ctx, `SELECT onboarded_at FROM users WHERE id = ?`, user).Scan(&onboarded); err != nil {
		t.Fatal(err)
	}
	if finished.Valid || startUnit.Valid || wordsKnown.Valid || onboarded.Valid {
		t.Fatalf("partial finish survived: finished=%v start=%v words=%v onboarded=%v", finished, startUnit, wordsKnown, onboarded)
	}
}

// A finished v3 run with no answered item is the same class as the v2 auto-close:
// finished_at is set and nothing was rated. It is not the learner's latest result.
// An older run that has an answered item is.
func TestLatestFinishedPlacementV3SkipsAnUnscoredClose(t *testing.T) {
	when := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "unscored-v3@example.com", "sub-unscored-v3", "learner")

	scored, err := s.StartPlacementV3(ctx, user, when, 1, RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	claimItem(t, s, scored, 1, PartListening, "listen", 1, "w-one", "right", mustLexemeText(t, s, "אֶחָד"), when)
	if err := s.FinishPlacement(ctx, scored, when.Add(time.Minute), 1, "", `{"bands":[{"band":0,"good":1}]}`); err != nil {
		t.Fatal(err)
	}

	blank, err := s.StartPlacementV3(ctx, user, when.Add(2*time.Minute), 2, RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.InsertPlacementItem(ctx, PlacementItem{
		RunID: blank, Seq: 1, Part: PartListening, Skill: "listen", Band: 1, ItemID: "w-blank",
		Options: `{"options":[]}`, ShownAt: when,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishPlacement(ctx, blank, when.Add(3*time.Minute), -1, "", `{"bands":[],"estimated_known_words":0}`); err != nil {
		t.Fatal(err)
	}

	got, _, ok, err := s.LatestFinishedPlacementV3(ctx, user, 0)
	if err != nil || !ok || got.ID != scored {
		t.Fatalf("latest finished v3 = id %d ok %v err %v, want scored run %d", got.ID, ok, err, scored)
	}

	v2 := insertUser(t, s, "v2-close@example.com", "sub-v2-close", "learner")
	closed, err := s.InsertPlacementRun(ctx, v2, when, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishPlacement(ctx, closed, when.Add(time.Minute), -1, "", `{"bands":[],"estimated_known_words":0}`); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := s.LatestFinishedPlacementV3(ctx, v2, 0); err != nil || ok {
		t.Fatalf("v2 auto-close returned ok=%v err=%v", ok, err)
	}
}

// countAnswerRatings is the v2 count, used to show the fixture has no answers.
func (s *Store) countAnswerRatings(ctx context.Context, runID int64) (int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	var n int
	err = conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM placement_answers
		WHERE run_id = ? AND rating IS NOT NULL`, runID).Scan(&n)
	return n, err
}
