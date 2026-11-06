package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDailyReplaceDayIdempotentAndRollback(t *testing.T) {
	when := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "daily@example.com", "sub-daily", "learner")
	day := "2026-11-05"
	first := []DailySnapshotRow{{
		Skill: "read", State: "known", Count: 3, EstKnown: 3, RuleVersion: 1,
	}}
	if err := s.InDailyTx(ctx, func(tx *DailyTx) error {
		return tx.ReplaceDay(ctx, user, day, first, 3, 1)
	}); err != nil {
		t.Fatal(err)
	}

	second := []DailySnapshotRow{
		{Skill: "read", State: "known", Count: 5, EstKnown: 5},
		{Skill: "listen", State: "known", Count: 2, EstKnown: 2},
	}
	if err := s.InDailyTx(ctx, func(tx *DailyTx) error {
		return tx.ReplaceDay(ctx, user, day, second, 7, 2)
	}); err != nil {
		t.Fatal(err)
	}

	snaps, err := s.SnapshotRows(ctx, user, day, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("snapshots after replace = %d, want 2", len(snaps))
	}
	for _, row := range snaps {
		if row.RuleVersion != 2 {
			t.Fatalf("snapshot %+v rule_version want 2", row)
		}
	}
	totals, err := s.ListTotals(ctx, user, day, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 1 || totals[0].KnownAnySkill != 7 || totals[0].RuleVersion != 2 {
		t.Fatalf("totals = %+v, want known_any 7 rule 2", totals)
	}

	// Rollback mid-replace leaves the previous rows.
	err = s.InDailyTx(ctx, func(tx *DailyTx) error {
		if err := tx.ReplaceDay(ctx, user, day, []DailySnapshotRow{{
			Skill: "spell", State: "known", Count: 1, EstKnown: 1,
		}}, 1, 3); err != nil {
			return err
		}
		return errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}
	snaps, err = s.SnapshotRows(ctx, user, day, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("after rollback snapshots = %d, want previous 2", len(snaps))
	}
	totals, err = s.ListTotals(ctx, user, day, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 1 || totals[0].RuleVersion != 2 || totals[0].KnownAnySkill != 7 {
		t.Fatalf("after rollback totals = %+v, want previous rule 2 / 7", totals)
	}
}

func TestDailyInsertSnapshotRuleVersionDefault(t *testing.T) {
	when := time.Date(2026, 11, 6, 13, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "snap-rv@example.com", "sub-snap-rv", "learner")
	day := "2026-11-04"
	if err := s.InDailyTx(ctx, func(tx *DailyTx) error {
		if err := tx.InsertSnapshot(ctx, user, day, []DailySnapshotRow{{
			Skill: "read", State: "seen", Count: 1, EstKnown: 0,
		}}); err != nil {
			return err
		}
		return tx.InsertTotals(ctx, user, day, 0, 0)
	}); err != nil {
		t.Fatal(err)
	}
	snaps, err := s.SnapshotRows(ctx, user, day, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].RuleVersion != 1 {
		t.Fatalf("snapshots = %+v, want rule_version 1", snaps)
	}
	totals, err := s.ListTotals(ctx, user, day, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 1 || totals[0].RuleVersion != 1 {
		t.Fatalf("totals = %+v, want rule_version 1", totals)
	}
}
