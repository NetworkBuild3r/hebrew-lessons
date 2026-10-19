package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestLatestPlacementAndKnownTexts(t *testing.T) {
	when := time.Date(2026, 10, 18, 15, 4, 5, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "place@example.com", "sub-place", "learner")
	otherID := insertUser(t, s, "other@example.com", "sub-other", "learner")

	place, ok, err := s.LatestPlacement(ctx, userID)
	if err != nil || ok || !place.TakenAt.IsZero() || place.SuggestedStartPosition != 0 || place.EstimatedKnownWords != 0 {
		t.Fatalf("no run: %+v ok=%v err=%v", place, ok, err)
	}
	if texts, err := s.PlacementKnownLexemes(ctx, userID); err != nil || len(texts) != 0 {
		t.Fatalf("no seeds: %v %v", texts, err)
	}

	runID, err := s.InsertPlacementRun(ctx, userID, when, 7)
	if err != nil {
		t.Fatal(err)
	}
	place, ok, err = s.LatestPlacement(ctx, userID)
	if err != nil || ok {
		t.Fatalf("in progress: %+v ok=%v err=%v", place, ok, err)
	}

	const (
		alef  = "אֶחָד"
		bet   = "בַּיִת"
		gimel = "גֶּשֶׁם"
		dalet = "דֶּלֶת"
		he    = "הֵם"
		vav   = "וֶרֶד"
		zayin = "זַיִת"
	)
	seeded := []string{alef, bet, zayin}
	for _, text := range seeded {
		id := insertLexeme(t, s, text)
		if err := s.InsertLearnerLexeme(ctx, userID, id, "known", true, "placement", when); err != nil {
			t.Fatal(err)
		}
	}
	// Another learner's seeded word, a known-but-unseeded word, and a learning row stay out.
	otherWord := insertLexeme(t, s, vav)
	if err := s.InsertLearnerLexeme(ctx, otherID, otherWord, "known", true, "placement", when); err != nil {
		t.Fatal(err)
	}
	unseeded := insertLexeme(t, s, dalet)
	if err := s.InsertLearnerLexeme(ctx, userID, unseeded, "known", false, "manual", when); err != nil {
		t.Fatal(err)
	}
	learning := insertLexeme(t, s, he)
	if err := s.InsertLearnerLexeme(ctx, userID, learning, "learning", true, "lesson", when); err != nil {
		t.Fatal(err)
	}
	// A finished lesson's new word is in the coverage set, and not in PlacementKnownLexemes.
	lessonWord := insertLexeme(t, s, gimel)
	lessonUnit := insertUnitDoc(t, s, 0, "draft", `{"title":"t","newWords":["`+gimel+`"]}`)
	if err := s.execLesson(ctx, lessonUnit, 0, lessonWord, "new", userID, &when); err != nil {
		t.Fatal(err)
	}

	got, err := s.PlacementKnownLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	wantTexts := []string{alef, bet, zayin}
	if len(got) != len(wantTexts) {
		t.Fatalf("seeded texts = %v, want %v", got, wantTexts)
	}
	for i := range wantTexts {
		if got[i] != wantTexts[i] {
			t.Fatalf("seeded texts = %v, want %v", got, wantTexts)
		}
	}
	learningTexts, err := s.PlacementLearningLexemes(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(learningTexts) != 1 || learningTexts[0] != he {
		t.Fatalf("learning texts = %v, want [%s]", learningTexts, he)
	}

	body, err := json.Marshal(placementResultJSON{
		Bands: []BandScore{{
			Band: 1, Shown: 12, Good: 12, GoodPlusRate: 1,
		}},
		EstimatedKnownWords: 42,
	})
	if err != nil {
		t.Fatal(err)
	}
	finished := when.Add(time.Minute)
	if err := s.FinishPlacement(ctx, runID, finished, 0.25, true, 1, "", string(body)); err != nil {
		t.Fatal(err)
	}

	// No extra units yet: the lesson unit at position 0 lists only the lesson word, which is known.
	place, ok, err = s.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		t.Fatalf("finished: ok=%v err=%v", ok, err)
	}
	if !place.TakenAt.Equal(finished) || !place.Reliable || place.FalseAlarmRate != 0.25 || place.Level != 1 {
		t.Fatalf("header = %+v", place)
	}
	if place.EstimatedKnownWords != 42 || len(place.BandScores) != 1 || place.BandScores[0].GoodPlusRate != 1 || place.BandScores[0].Shown != 12 {
		t.Fatalf("scores = %+v", place)
	}
	if place.SuggestedStartPosition != 1 {
		t.Fatalf("covered lesson unit start = %d, want 1", place.SuggestedStartPosition)
	}

	// 4 of 5 is exactly 80% and counts as covered. 2 of 5 is not.
	// A newWords value that is not an array of strings counts as coverage 0.
	insertUnitDoc(t, s, 3, "published", `{"newWords":["`+alef+`","`+bet+`","`+gimel+`","`+zayin+`","`+dalet+`"]}`)
	insertUnitDoc(t, s, 11, "draft", `{"newWords":[]}`)
	insertUnitDoc(t, s, 8, "published", `{"newWords":["`+alef+`","`+bet+`","`+dalet+`","`+he+`","`+vav+`"]}`)
	insertUnitDoc(t, s, 4, "published", `{"newWords":{"no":"pe"}}`)
	insertUnitDoc(t, s, 2, "published", `{"newWords":["`+alef+`","`+bet+`","`+gimel+`","`+zayin+`"]}`)

	place, ok, err = s.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if place.SuggestedStartPosition != 4 {
		t.Fatalf("start = %d, want the lowest uncovered position 4", place.SuggestedStartPosition)
	}

	// A unit added later moves the suggestion. The stored estimate does not change.
	insertUnitDoc(t, s, 1, "draft", `{"newWords":["`+dalet+`"]}`)
	place, ok, err = s.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if place.SuggestedStartPosition != 1 || place.EstimatedKnownWords != 42 {
		t.Fatalf("after insert: start=%d estimate=%d", place.SuggestedStartPosition, place.EstimatedKnownWords)
	}

	// A learner with no units and a finished run starts at 0.
	otherRun, err := s.InsertPlacementRun(ctx, otherID, when, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishPlacement(ctx, otherRun, finished, 0, false, -1, "", `{"bands":[],"estimated_known_words":0}`); err != nil {
		t.Fatal(err)
	}
	// Units are global, but coverage is this learner's known set. The unit at
	// position 0 lists a word only the first learner was taught, so this
	// learner starts there.
	otherPlace, ok, err := s.LatestPlacement(ctx, otherID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if otherPlace.Level != -1 || otherPlace.Reliable || otherPlace.SuggestedStartPosition != 0 {
		t.Fatalf("other = %+v", otherPlace)
	}
	if texts, err := s.PlacementKnownLexemes(ctx, otherID); err != nil || len(texts) != 1 || texts[0] != vav {
		t.Fatalf("other texts = %v %v", texts, err)
	}
}

func TestSuggestedStartWithNoUnitsIsZero(t *testing.T) {
	when := time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	userID := insertUser(t, s, "empty@example.com", "sub-empty", "learner")
	runID, err := s.InsertPlacementRun(ctx, userID, when, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishPlacement(ctx, runID, when, 0, true, -1, "", `{"estimated_known_words":3}`); err != nil {
		t.Fatal(err)
	}
	place, ok, err := s.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if place.SuggestedStartPosition != 0 || place.EstimatedKnownWords != 3 {
		t.Fatalf("place = %+v", place)
	}

	// Every unit covered: one past the highest position.
	insertUnitDoc(t, s, 6, "published", `{"title":"t"}`)
	insertUnitDoc(t, s, 2, "draft", `{"newWords":null}`)
	place, ok, err = s.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		t.Fatal(err)
	}
	if place.SuggestedStartPosition != 7 {
		t.Fatalf("all covered start = %d, want 7", place.SuggestedStartPosition)
	}
}

func insertUnitDoc(t *testing.T, s *Store, position int, status, doc string) int64 {
	t.Helper()
	res, err := s.exec(context.Background(), `
		INSERT INTO units (position, title, situation, concept, status, rev, doc)
		VALUES (?, 't', 's', 'c', ?, 1, ?)`, position, status, doc)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (s *Store) execLesson(ctx context.Context, unitID int64, lessonIx int, lexemeID int64, role string, userID int64, finished *time.Time) error {
	if _, err := s.exec(ctx, `
		INSERT INTO lesson_items (unit_id, lesson_ix, lexeme_id, role, skill)
		VALUES (?, ?, ?, ?, 'recall')`, unitID, lessonIx, lexemeID, role); err != nil {
		return err
	}
	var done any
	if finished != nil {
		done = placementTime(*finished)
	}
	_, err := s.exec(ctx, `
		INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?)`, userID, unitID, lessonIx, placementTime(*finished), done)
	return err
}
