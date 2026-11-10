package db

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

// ReplaceDay stores the rule version it is given and never edits review_log.
// Literal 5 matches jobs.RuleVersion after the bump (db must not import jobs).
func TestReplaceDayStampsRuleVersion5LeavesReviewLog(t *testing.T) {
	const ruleVersion = 5
	when := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{when})
	ctx := context.Background()
	user := insertUser(t, s, "rv4@example.com", "sub-rv4", "learner")

	lex, err := s.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES ('א', 'a')`)
	if err != nil {
		t.Fatal(err)
	}
	lexID, err := lex.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	cardID, err := s.InsertCard(ctx, Card{
		UserID: user, LexemeID: lexID, Skill: "read",
		State: "review", Due: when, Stability: 10, Difficulty: 5,
		Reps: 1, LastReview: when.Add(-time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ExecContext(ctx, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 0, 1)`,
		cardID, FormatTime(when.Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	before := reviewLogChecksum(t, s)

	day := "2026-11-07"
	rows := []DailySnapshotRow{{
		Skill: "read", State: "learning", Count: 1, EstKnown: 0,
	}}
	if err := s.InDailyTx(ctx, func(tx *DailyTx) error {
		return tx.ReplaceDay(ctx, user, day, rows, 0, ruleVersion)
	}); err != nil {
		t.Fatal(err)
	}
	snaps, err := s.SnapshotRows(ctx, user, day, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 1 || snaps[0].RuleVersion != ruleVersion {
		t.Fatalf("snapshots = %+v, want rule_version %d", snaps, ruleVersion)
	}
	totals, err := s.ListTotals(ctx, user, day, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(totals) != 1 || totals[0].RuleVersion != ruleVersion {
		t.Fatalf("totals = %+v, want rule_version %d", totals, ruleVersion)
	}
	after := reviewLogChecksum(t, s)
	if after != before {
		t.Fatalf("review_log changed: %s → %s", before, after)
	}

	// Second ReplaceDay with the same stamp; review_log stays byte-identical.
	if err := s.InDailyTx(ctx, func(tx *DailyTx) error {
		return tx.ReplaceDay(ctx, user, day, rows, 0, ruleVersion)
	}); err != nil {
		t.Fatal(err)
	}
	if got := reviewLogChecksum(t, s); got != before {
		t.Fatalf("review_log changed on second replace: %s → %s", before, got)
	}
}

func reviewLogChecksum(t *testing.T, s *Store) string {
	t.Helper()
	ctx := context.Background()
	rows, err := s.conn.QueryContext(ctx, `SELECT * FROM review_log ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	h := sha256.New()
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		for _, v := range vals {
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			fmt.Fprintf(h, "%v|", v)
		}
		fmt.Fprintln(h)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
