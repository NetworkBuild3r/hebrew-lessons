package learner

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

func TestRecomputeDirectionsMatchesStanding(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		e.lexeme()
	}
	seedAttemptWindow(t, e, fsrs.SkillRecall, 50, 18)
	seedAttemptWindow(t, e, fsrs.SkillRead, 189, 33)

	standing, err := e.model.Standing(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	dirs, err := e.model.RecomputeDirections(ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != len(standing.Directions) {
		t.Fatalf("dirs=%d standing=%d", len(dirs), len(standing.Directions))
	}
	for i, d := range dirs {
		s := standing.Directions[i]
		if d.Skill != s.Skill || d.Verdict != s.Verdict {
			t.Fatalf("%s recompute=%+v standing=%+v", d.Skill, d, s)
		}
		if FormatAccuracy(d.Accuracy14) != FormatAccuracy(s.Accuracy14) {
			t.Fatalf("%s accuracy recompute=%v standing=%v", d.Skill, d.Accuracy14, s.Accuracy14)
		}
	}
}

func TestCountRule1Days(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	n, err := CountRule1Days(ctx, e.store, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("rule1=%d, want 0", n)
	}
	day := "2026-11-01"
	if err := e.store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		return tx.InsertTotals(ctx, e.user, day, 3, 1)
	}); err != nil {
		t.Fatal(err)
	}
	n, err = CountRule1Days(ctx, e.store, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rule1=%d, want 1", n)
	}
}

func TestCountOlderRuleDays(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	n, err := CountOlderRuleDays(ctx, e.store, e.user, 3)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("older=%d, want 0", n)
	}
	if err := e.store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		if err := tx.InsertTotals(ctx, e.user, "2026-11-01", 1, 2); err != nil {
			return err
		}
		return tx.InsertTotals(ctx, e.user, "2026-11-02", 2, 3)
	}); err != nil {
		t.Fatal(err)
	}
	n, err = CountOlderRuleDays(ctx, e.store, e.user, 3)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("older=%d, want 1 (the rule-2 day)", n)
	}
}

func TestHistoryFactsLastDayMatchesSnapshotAt(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	a := e.lexeme()
	e.known(e.user, a, fsrs.SkillRead, 20)
	if err := e.store.SetUserTimeZone(ctx, e.user, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	day := "2026-11-05"
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	// Midnight that starts the next civil day in the learner zone (same as jobs.EndOfDay).
	end := time.Date(2026, 11, 6, 0, 0, 0, 0, loc)
	snap, err := e.model.SnapshotAt(ctx, e.user, end)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		rows := []db.DailySnapshotRow{{
			Skill: "read", State: "known",
			Count: snap.Summary.KnownAnySkill, EstKnown: float64(snap.Summary.KnownAnySkill),
			RuleVersion: 3,
		}}
		return tx.ReplaceDay(ctx, e.user, day, rows, snap.Summary.KnownAnySkill, 3)
	}); err != nil {
		t.Fatal(err)
	}
	facts, err := e.model.HistoryFacts(ctx, e.user, func(d string) (time.Time, error) {
		parsed, err := time.ParseInLocation("2006-01-02", d, loc)
		if err != nil {
			return time.Time{}, err
		}
		return parsed.AddDate(0, 0, 1), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if facts.HistoryLastDay != day {
		t.Fatalf("last day=%q want %s", facts.HistoryLastDay, day)
	}
	if facts.HistoryLastKnownAny != facts.SnapshotAtKnownAny {
		t.Fatalf("totals=%d SnapshotAt=%d", facts.HistoryLastKnownAny, facts.SnapshotAtKnownAny)
	}
	if facts.HistoryRule1Days != 0 {
		t.Fatalf("rule1=%d", facts.HistoryRule1Days)
	}
}
