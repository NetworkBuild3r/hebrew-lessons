package db

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"
)

func TestPlacementHistoryOrderLimitCountAndTitle(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	user := insertUser(t, s, "learner@example.com", "sub-learner", "learner")
	other := insertUser(t, s, "other@example.com", "sub-other", "learner")

	// Ids are not finish order: the newest finish is inserted first.
	newest := insertFinishedRun(t, s, user, "2026-10-18T08:00:00.000000000Z", 2, 9)
	oldest := insertFinishedRun(t, s, user, "2026-10-01T08:00:00.000000000Z", 0, 1)
	middle := insertFinishedRun(t, s, user, "2026-10-09T08:00:00.000000000Z", 5, 4)
	insertOpenRun(t, s, user)
	insertFinishedRun(t, s, other, "2026-10-19T08:00:00.000000000Z", 2, 40)

	// A published unit at position 0 must not be used for a NULL start_unit.
	insertOnboardingUnit(t, s, 0, "Zero", "published", "shared")
	insertOnboardingUnit(t, s, 2, "Draft mine", "draft", "learner:"+itoa(user))
	insertOnboardingUnit(t, s, 2, "Shared two", "published", "shared")
	insertOnboardingUnit(t, s, 2, "Mine two", "published", "learner:"+itoa(user))
	insertOnboardingUnit(t, s, 5, "Hidden", "published", "learner:999999")
	// The NULL start_unit row stores no position. Set it after insert.
	mustExec(t, s, `UPDATE placement_runs SET start_unit = NULL, words_known = NULL WHERE id = ?`, oldest)

	all, err := s.PlacementHistory(ctx, user, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(idsOfRuns(all), []int64{newest, middle, oldest}) {
		t.Fatalf("limit 0 = %v, want %d %d %d", idsOfRuns(all), newest, middle, oldest)
	}
	for _, r := range all {
		if r.RunID != r.ID {
			t.Fatalf("RunID %d and ID %d differ", r.RunID, r.ID)
		}
	}
	if all[0].StartUnit != 2 || all[0].WordsKnown != 9 || all[0].StartUnitTitle != "Mine two" {
		t.Fatalf("newest %+v, want start 2, 9 words, title Mine two", all[0])
	}
	if all[1].StartUnit != 5 || all[1].StartUnitTitle != "" || all[1].WordsKnown != 4 {
		t.Fatalf("out-of-scope unit supplied a title: %+v", all[1])
	}
	if all[2].StartUnit != 0 || all[2].WordsKnown != 0 || all[2].StartUnitTitle != "" {
		t.Fatalf("NULL snapshot read as %+v", all[2])
	}
	if !all[0].FinishedAt.After(all[1].FinishedAt) || !all[1].FinishedAt.After(all[2].FinishedAt) {
		t.Fatalf("times not newest first: %s %s %s", all[0].FinishedAt, all[1].FinishedAt, all[2].FinishedAt)
	}

	two, err := s.PlacementHistory(ctx, user, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(idsOfRuns(two), []int64{newest, middle}) {
		t.Fatalf("limit 2 = %v", idsOfRuns(two))
	}
	neg, err := s.PlacementHistory(ctx, user, -1)
	if err != nil {
		t.Fatal(err)
	}
	if !sameIDs(idsOfRuns(neg), []int64{newest, middle, oldest}) {
		t.Fatalf("limit -1 = %v", idsOfRuns(neg))
	}
	if n, err := s.PlacementHistoryCount(ctx, user); err != nil || n != 3 {
		t.Fatalf("count = %d %v, want 3", n, err)
	}
	if n, err := s.PlacementHistoryCount(ctx, other); err != nil || n != 1 {
		t.Fatalf("other count = %d %v", n, err)
	}
	if n, err := s.PlacementHistoryCount(ctx, 0); err != nil || n != 0 {
		t.Fatalf("missing user count = %d %v", n, err)
	}
}

func TestSetOnboardedKeepsTheFirstTime(t *testing.T) {
	s := openStore(t, nil)
	ctx := context.Background()
	user := insertUser(t, s, "learner@example.com", "sub-learner", "learner")
	first := time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	second := first.Add(48 * time.Hour)
	if err := s.SetOnboarded(ctx, user, first); err != nil {
		t.Fatal(err)
	}
	if err := s.SetOnboarded(ctx, user, second); err != nil {
		t.Fatal(err)
	}
	u, err := s.UserByEmail(ctx, "learner@example.com")
	if err != nil || u.OnboardedAt == nil || !u.OnboardedAt.Equal(first) {
		t.Fatalf("onboarded %+v %v, want %s", u.OnboardedAt, err, first)
	}
	sess := "hash-onboard"
	if err := s.InsertSession(ctx, sess, user, "csrf", first.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	_, loaded, err := s.SessionUser(ctx, sess)
	if err != nil || loaded.OnboardedAt == nil || !loaded.OnboardedAt.Equal(first) {
		t.Fatalf("session user onboarded %+v %v", loaded.OnboardedAt, err)
	}
	fresh, err := s.CreateUser(ctx, "new@example.com", "sub-new", "learner")
	if err != nil || fresh.OnboardedAt != nil {
		t.Fatalf("new user onboarded %+v %v", fresh.OnboardedAt, err)
	}
	if err := s.SetOnboarded(ctx, 0, first); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing user: %v", err)
	}
}

func insertFinishedRun(t *testing.T, s *Store, userID int64, finished string, startUnit, wordsKnown int) int64 {
	t.Helper()
	res := mustExec(t, s, `INSERT INTO placement_runs (user_id, started_at, finished_at, level, result, seed, sitting_no, start_unit, words_known)
		VALUES (?, ?, ?, 0, '{}', 1, 1, ?, ?)`, userID, finished, finished, startUnit, wordsKnown)
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertOpenRun(t *testing.T, s *Store, userID int64) int64 {
	t.Helper()
	res := mustExec(t, s, `INSERT INTO placement_runs (user_id, started_at, seed) VALUES (?, '2026-10-20T08:00:00.000000000Z', 3)`, userID)
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertOnboardingUnit(t *testing.T, s *Store, position int, title, status, scope string) {
	t.Helper()
	mustExec(t, s, `INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
		VALUES (?, ?, 's', 'c', ?, 1, '{}', ?)`, position, title, status, scope)
}

func idsOfRuns(runs []PlacementHistoryRow) []int64 {
	out := make([]int64, len(runs))
	for i := range runs {
		out[i] = runs[i].ID
	}
	return out
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
