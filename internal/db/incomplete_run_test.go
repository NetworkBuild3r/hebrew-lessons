package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// The three closes that set finished_at and leave no ratings. None of them is
// a finished result: history and the onboarding backfill skip them, and
// LatestPlacement marks the newest one incomplete.
func TestPlacementHistoryAndCountSkipUnscoredFinishes(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 21, 15, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	user := insertUser(t, s, "brian@example.com", "sub-brian", "learner")

	closes := insertUnscoredCloses(t, s, user)
	nullRating := insertClosedRun(t, s, user, "2026-09-04T08:00:00.000000000Z", "2026-09-04T08:05:00.000000000Z", -1, "", `{"bands":[],"estimated_known_words":0}`)
	mustExec(t, s, `INSERT INTO placement_answers (run_id, item_id, rating, shown_at)
		VALUES (?, 'shown', NULL, '2026-09-04T08:05:00.000000000Z')`, nullRating)
	unanswered := insertClosedRun(t, s, user, "2026-09-05T08:00:00.000000000Z", "2026-09-05T08:05:00.000000000Z", -1, "", `{"bands":[],"estimated_known_words":0}`)
	mustExec(t, s, `INSERT INTO placement_items (run_id, seq, part, item_id, options, shown_at)
		VALUES (?, 1, 'letters', 'alef', '{}', '2026-09-05T08:05:00.000000000Z')`, unanswered)

	v2 := insertClosedRun(t, s, user, "2026-10-04T08:00:00.000000000Z", "2026-10-04T09:00:00.000000000Z", 2, "band_stop", `{"bands":[],"estimated_known_words":80}`)
	mustExec(t, s, `INSERT INTO placement_answers (run_id, item_id, rating, shown_at, answered_at)
		VALUES (?, 'rated', 3, '2026-10-04T09:00:00.000000000Z', '2026-10-04T09:00:00.000000000Z')`, v2)
	v3 := insertClosedRun(t, s, user, "2026-10-05T08:00:00.000000000Z", "2026-10-05T09:00:00.000000000Z", 3, "exhausted", `{"estimated_known_words":90}`)
	mustExec(t, s, `INSERT INTO placement_items (run_id, seq, part, item_id, options, outcome, shown_at, answered_at)
		VALUES (?, 1, 'listening', 'w', '{}', 'right', '2026-10-05T09:00:00.000000000Z', '2026-10-05T09:00:00.000000000Z')`, v3)
	insertOpenRun(t, s, user)

	runs, err := s.PlacementHistory(ctx, user, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(idsOfRuns(runs), []int64{v3, v2}) {
		t.Fatalf("history %v, want scored runs %d %d and not %d %d %d %d %d", idsOfRuns(runs), v3, v2, closes.auto, closes.oldFinish, closes.exhausted, nullRating, unanswered)
	}
	if n, err := s.PlacementHistoryCount(ctx, user); err != nil || n != 2 {
		t.Fatalf("count %d %v, want 2", n, err)
	}
	var kept int
	if err := s.queryRow(ctx, `SELECT COUNT(*) FROM placement_runs WHERE user_id = ? AND finished_at IS NOT NULL`, user).Scan(&kept); err != nil {
		t.Fatal(err)
	}
	if kept != 7 {
		t.Fatalf("stored finished rows %d, want all 7 kept", kept)
	}
}

func TestEnsureOnboardedIgnoresUnscoredFinishes(t *testing.T) {
	now := time.Date(2026, 10, 21, 15, 0, 0, 0, time.UTC)
	for _, shape := range []string{"auto-close", "old finish", "exhausted"} {
		t.Run(shape, func(t *testing.T) {
			s := openStore(t, fixedClock{now})
			ctx := context.Background()
			user := insertUser(t, s, shape+"@example.com", "sub-"+shape, "learner")
			closes := insertUnscoredCloses(t, s, user)
			var keep int64
			switch shape {
			case "auto-close":
				keep = closes.auto
			case "old finish":
				keep = closes.oldFinish
			default:
				keep = closes.exhausted
			}
			mustExec(t, s, `DELETE FROM placement_runs WHERE user_id = ? AND id <> ?`, user, keep)
			yes, err := s.EnsureOnboarded(ctx, user)
			if err != nil || yes {
				t.Fatalf("EnsureOnboarded = %v, %v; want false", yes, err)
			}
			u, err := s.UserByEmail(ctx, shape+"@example.com")
			if err != nil || u.OnboardedAt != nil {
				t.Fatalf("onboarded_at %+v %v", u.OnboardedAt, err)
			}
		})
	}

	s := openStore(t, fixedClock{now})
	ctx := context.Background()
	user := insertUser(t, s, "brian@example.com", "sub-brian", "learner")
	insertUnscoredCloses(t, s, user)
	lex := insertLexeme(t, s, "שָׁלוֹם")
	const lessonAt = "2026-10-01T09:00:00.000000000Z"
	mustExec(t, s, `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, 'read', 'pick', 'lesson', 'p', 'r', 1, 0, 0, 10, ?)`, user, lex, lessonAt)
	yes, err := s.EnsureOnboarded(ctx, user)
	if err != nil || !yes {
		t.Fatalf("EnsureOnboarded = %v, %v; want true", yes, err)
	}
	want := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	u, err := s.UserByEmail(ctx, "brian@example.com")
	if err != nil || u.OnboardedAt == nil || !u.OnboardedAt.Equal(want) {
		t.Fatalf("onboarded_at %+v %v, want the lesson time %s", u.OnboardedAt, err, want)
	}
}

func TestLatestPlacementMarksUnscoredFinishIncomplete(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 21, 15, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	user := insertUser(t, s, "brian@example.com", "sub-brian", "learner")
	scored := insertClosedRun(t, s, user, "2026-08-01T08:00:00.000000000Z", "2026-08-01T09:00:00.000000000Z", 2, "band_stop", `{"bands":[],"estimated_known_words":80}`)
	mustExec(t, s, `INSERT INTO placement_answers (run_id, item_id, rating, shown_at, answered_at)
		VALUES (?, 'rated', 3, '2026-08-01T09:00:00.000000000Z', '2026-08-01T09:00:00.000000000Z')`, scored)
	closes := insertUnscoredCloses(t, s, user)

	place, ok, err := s.LatestPlacement(ctx, user)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	wantAt := time.Date(2026, 9, 3, 8, 5, 0, 0, time.UTC)
	if place.Finished || !place.Incomplete || place.Level != -1 || place.StopReason != "exhausted" || place.CardsRated != 0 || !place.TakenAt.Equal(wantAt) || place.Sittings != 1 {
		t.Fatalf("latest %+v, want the exhausted close marked incomplete", place)
	}
	if len(place.BandScores) != 0 {
		t.Fatalf("bands %+v, want the empty exhausted result", place.BandScores)
	}
	runs, err := s.PlacementHistory(ctx, user, 0)
	if err != nil || len(runs) != 1 || runs[0].ID != scored {
		t.Fatalf("history %+v %v, want the scored run %d", runs, err, scored)
	}
	_ = closes
}

type unscoredCloses struct {
	auto, oldFinish, exhausted int64
}

// insertUnscoredCloses seeds the three zero-rating finishes, oldest first.
// The exhausted close is the newest. Each has finished_at, level -1, and no
// placement_answers. The auto-close and the old Finish leave stop_reason NULL.
// The old Finish stores five all-zero bands. finishNow stores exhausted and
// empty bands.
func insertUnscoredCloses(t *testing.T, s *Store, userID int64) unscoredCloses {
	t.Helper()
	return unscoredCloses{
		auto:      insertClosedRun(t, s, userID, "2026-09-01T08:00:00.000000000Z", "2026-09-01T08:05:00.000000000Z", -1, "", `{"bands":[],"estimated_known_words":0}`),
		oldFinish: insertClosedRun(t, s, userID, "2026-09-02T08:00:00.000000000Z", "2026-09-02T08:05:00.000000000Z", -1, "", fiveZeroBands(t)),
		exhausted: insertClosedRun(t, s, userID, "2026-09-03T08:00:00.000000000Z", "2026-09-03T08:05:00.000000000Z", -1, "exhausted", `{"bands":[],"estimated_known_words":0}`),
	}
}

func insertClosedRun(t *testing.T, s *Store, userID int64, started, finished string, level int, stop, result string) int64 {
	t.Helper()
	var stopArg any
	if stop != "" {
		stopArg = stop
	}
	res := mustExec(t, s, `INSERT INTO placement_runs (user_id, started_at, finished_at, level, stop_reason, result, seed, sitting_no)
		VALUES (?, ?, ?, ?, ?, ?, 1, 1)`, userID, started, finished, level, stopArg, result)
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func fiveZeroBands(t *testing.T) string {
	t.Helper()
	bands := make([]BandScore, 5)
	for i := range bands {
		bands[i].Band = i
	}
	raw, err := json.Marshal(placementResultJSON{Bands: bands})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
