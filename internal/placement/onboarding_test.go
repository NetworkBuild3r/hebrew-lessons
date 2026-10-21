package placement

import (
	"context"
	"database/sql"
	"strconv"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// A finished v3 run stores the start unit and the distinct Good lexemes, and sets
// onboarded_at. A second finish keeps the first onboarded_at. An unfinished run
// is absent from history and leaves onboarded_at null. words_known is the value
// stored at finish: changing the items afterwards does not change history.
func TestFinishRecordsOnboardingAndHistoryIgnoresAnOpenRun(t *testing.T) {
	when := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)
	clk := movingClock{t: &when}
	e, store := openEngine(t, clk, 40)
	ctx := context.Background()
	user := newLearner(t, store, "onboard")
	insertHistoryUnit(t, store, 2, "Gate", "published", "shared", `{"newWords":["no-such-word"]}`)

	runID, _, _ := drive(t, e, user, script{letters: 6, pattern: knowsOnly(1), typing: "right"})
	items := itemsOf(t, e, runID)
	distinct := map[int64]struct{}{}
	rightItems := 0
	for _, it := range items {
		if it.Outcome == OutcomeRight && it.LexemeID != 0 {
			rightItems++
			distinct[it.LexemeID] = struct{}{}
		}
	}
	if rightItems == 0 || len(distinct) == 0 || rightItems == len(distinct) {
		t.Fatalf("right items %d distinct %d: the run must rate one lexeme Good more than once", rightItems, len(distinct))
	}

	var startUnit, wordsKnown sql.NullInt64
	var finished string
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, `SELECT start_unit, words_known, finished_at FROM placement_runs WHERE id = ?`, runID).Scan(&startUnit, &wordsKnown, &finished); err != nil {
		t.Fatal(err)
	}
	if !startUnit.Valid || startUnit.Int64 != 2 || !wordsKnown.Valid || int(wordsKnown.Int64) != len(distinct) {
		t.Fatalf("stored start=%v words=%v, want start 2 and %d words", startUnit, wordsKnown, len(distinct))
	}

	u, err := store.UserByEmail(ctx, "onboard@example.com")
	if err != nil || u.OnboardedAt == nil || !u.OnboardedAt.Equal(when) {
		t.Fatalf("onboarded %+v %v, want %s", u.OnboardedAt, err, when)
	}
	hist, err := store.PlacementHistory(ctx, user, 0)
	if err != nil || len(hist) != 1 || hist[0].ID != runID || hist[0].StartUnit != 2 || hist[0].WordsKnown != len(distinct) || hist[0].StartUnitTitle != "Gate" {
		t.Fatalf("history %+v %v", hist, err)
	}
	if n, err := store.PlacementHistoryCount(ctx, user); err != nil || n != 1 {
		t.Fatalf("count %d %v", n, err)
	}

	// The stored count is not recomputed from the items.
	if _, err := conn.ExecContext(ctx, `UPDATE placement_items SET outcome = 'wrong' WHERE run_id = ?`, runID); err != nil {
		t.Fatal(err)
	}
	again, err := store.PlacementHistory(ctx, user, 1)
	if err != nil || len(again) != 1 || again[0].WordsKnown != len(distinct) || again[0].StartUnitTitle != "Gate" {
		t.Fatalf("history after rewriting items: %+v %v", again, err)
	}

	first := when
	when = when.Add(72 * time.Hour)
	second, _, _ := drive(t, e, user, script{letters: 6, pattern: knowsOnly(1), typing: "skip"})
	if second == runID {
		t.Fatal("second drive reused the finished run")
	}
	u, err = store.UserByEmail(ctx, "onboard@example.com")
	if err != nil || u.OnboardedAt == nil || !u.OnboardedAt.Equal(first) {
		t.Fatalf("second finish moved onboarded_at to %+v (%v), want %s", u.OnboardedAt, err, first)
	}
	hist, err = store.PlacementHistory(ctx, user, 10)
	if err != nil || len(hist) != 2 || hist[0].ID != second || hist[1].ID != runID {
		t.Fatalf("history after the second finish: %+v %v", hist, err)
	}

	// An open run is not history and does not set onboarded_at.
	left := newLearner(t, store, "left")
	if _, err := e.Start(ctx, left); err != nil {
		t.Fatal(err)
	}
	if runs, err := store.PlacementHistory(ctx, left, 0); err != nil || len(runs) != 0 {
		t.Fatalf("open run is in history: %+v %v", runs, err)
	}
	if n, err := store.PlacementHistoryCount(ctx, left); err != nil || n != 0 {
		t.Fatalf("open run count %d %v", n, err)
	}
	leftUser, err := store.UserByEmail(ctx, "left@example.com")
	if err != nil || leftUser.OnboardedAt != nil {
		t.Fatalf("abandoned run set onboarded_at %+v %v", leftUser.OnboardedAt, err)
	}
}

// Closing an open v2 run so a v3 run can start is not onboarding.
func TestRetiringAnOpenV2RunDoesNotOnboard(t *testing.T) {
	when := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	e, store := openEngine(t, fixedClock{when}, 41)
	ctx := context.Background()
	user := newLearner(t, store, "retire")
	if _, err := store.InsertPlacementRun(ctx, user, when.Add(-time.Hour), 6); err != nil {
		t.Fatal(err)
	}
	before, err := store.EnsureOnboarded(ctx, user)
	if err != nil || before {
		t.Fatalf("open v2 run EnsureOnboarded = %v %v, want false", before, err)
	}
	runID, err := e.Start(ctx, user)
	if err != nil {
		t.Fatal(err)
	}
	// The new v3 run is left open with no answers. That is the abandon.
	if n := countRows(t, store, `SELECT COUNT(*) FROM placement_items WHERE run_id = ? AND outcome IS NOT NULL`, runID); n != 0 {
		t.Fatalf("abandoned v3 run has %d answers", n)
	}
	after, err := store.EnsureOnboarded(ctx, user)
	if err != nil || after {
		t.Fatalf("after Start and abandon EnsureOnboarded = %v %v, want false", after, err)
	}
	u, err := store.UserByEmail(ctx, "retire@example.com")
	if err != nil || u.OnboardedAt != nil {
		t.Fatalf("retiring the v2 run set onboarded_at %+v %v", u.OnboardedAt, err)
	}
	if n, err := store.PlacementHistoryCount(ctx, user); err != nil || n != 0 {
		t.Fatalf("history count %d %v, want 0", n, err)
	}
	runs, err := store.PlacementHistory(ctx, user, 0)
	if err != nil || len(runs) != 0 {
		t.Fatalf("history %+v %v, want none", runs, err)
	}
}

// A published unit outside the learner's scope does not supply the title.
// The learner's own published unit does, ahead of a shared unit at the same position.
func TestStartUnitTitleUsesTheLearnersPublishedUnit(t *testing.T) {
	when := time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)
	e, store := openEngine(t, fixedClock{when}, 42)
	ctx := context.Background()

	outsider := newLearner(t, store, "outsider")
	insertHistoryUnit(t, store, 4, "Hidden", "published", "learner:999999", `{"newWords":["no-such-word"]}`)
	drive(t, e, outsider, script{letters: 6, pattern: knowsOnly(1), typing: "skip"})
	hist, err := store.PlacementHistory(ctx, outsider, 1)
	if err != nil || len(hist) != 1 || hist[0].StartUnit != 4 || hist[0].StartUnitTitle != "" {
		t.Fatalf("out-of-scope title: %+v %v", hist, err)
	}

	owner := newLearner(t, store, "owner")
	insertHistoryUnit(t, store, 4, "Shared", "published", "shared", `{"newWords":["no-such-word"]}`)
	insertHistoryUnit(t, store, 4, "Draft mine", "draft", "learner:"+formatID(owner), `{"newWords":["no-such-word"]}`)
	insertHistoryUnit(t, store, 4, "Mine", "published", "learner:"+formatID(owner), `{"newWords":["no-such-word"]}`)
	drive(t, e, owner, script{letters: 6, pattern: knowsOnly(1), typing: "skip"})
	hist, err = store.PlacementHistory(ctx, owner, 1)
	if err != nil || len(hist) != 1 || hist[0].StartUnit != 4 || hist[0].StartUnitTitle != "Mine" {
		t.Fatalf("learner title: %+v %v", hist, err)
	}
}

func insertHistoryUnit(t *testing.T, store *db.Store, position int, title, status, scope, doc string) {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), `INSERT INTO units (position, title, situation, concept, status, rev, doc, scope)
		VALUES (?, ?, 's', 'c', ?, 1, ?, ?)`, position, title, status, doc, scope); err != nil {
		t.Fatal(err)
	}
}

func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}
