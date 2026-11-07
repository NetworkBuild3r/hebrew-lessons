package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/jobs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

type historyFix struct {
	path string
	user int64
	clk  fixedClock
}

func historyFixture(t *testing.T) historyFix {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	t.Setenv("TZ", "America/Chicago")
	path := filepath.Join(dir, "hebrew.sqlite")
	when := time.Date(2026, 10, 22, 17, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, err := db.Open(ctx, path, fixedClock{when})
	if err != nil {
		t.Fatal(err)
	}
	user, err := store.CreateUser(ctx, "history@example.invalid", "sub-history", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserTimeZone(ctx, user.ID, "America/Chicago"); err != nil {
		t.Fatal(err)
	}
	lex, err := store.ExecContext(ctx, `INSERT INTO lexemes (text_nfc, gloss_en) VALUES ('שלום', 'hello')`)
	if err != nil {
		t.Fatal(err)
	}
	lexID, err := lex.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	// Evidence across three days so as-of snapshots differ.
	seedKnown(t, store, user.ID, lexID, fsrs.SkillListen, time.Date(2026, 10, 18, 18, 0, 0, 0, time.UTC), 100)
	seedKnown(t, store, user.ID, lexID, fsrs.SkillRead, time.Date(2026, 10, 19, 18, 0, 0, 0, time.UTC), 10)
	atWrong := time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		if _, err := store.ExecContext(ctx, `
			INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', 0, 0, 0, 0, ?)`,
			user.ID, lexID, fsrs.SkillListen, db.FormatTime(atWrong.Add(time.Duration(i)*time.Minute))); err != nil {
			t.Fatal(err)
		}
	}

	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := studyday.NewResolver(store, loc)
	if err != nil {
		t.Fatal(err)
	}
	daily := jobs.NewDaily(store, fixedClock{when}, resolver, jobs.WithAppZone(loc), jobs.WithOwner("seed"))
	if _, err := daily.CatchUp(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return historyFix{path: path, user: user.ID, clk: fixedClock{when}}
}

func seedKnown(t *testing.T, store *db.Store, user, lexeme int64, skill string, at time.Time, stability float64) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := store.ExecContext(ctx, `
			INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
			user, lexeme, skill, db.FormatTime(at.Add(-time.Duration(3-i)*time.Minute))); err != nil {
			t.Fatal(err)
		}
	}
	last := at.AddDate(0, 0, -1)
	id, err := store.InsertCard(ctx, db.Card{
		UserID: user, LexemeID: lexeme, Skill: skill, State: fsrs.StateReview,
		Due: at, Stability: stability, Difficulty: 5, Reps: 3, LastReview: last,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := store.ExecContext(ctx, `
			INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days)
			VALUES (?, 3, ?, 0, 1)`, id, db.FormatTime(last.Add(-time.Duration(2-i)*24*time.Hour))); err != nil {
			t.Fatal(err)
		}
	}
}

func tableChecksum(t *testing.T, path, table string) (count int, sum string) {
	t.Helper()
	ctx := context.Background()
	store, err := db.OpenQueryOnly(ctx, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	rows, err := conn.QueryContext(ctx, `SELECT * FROM `+table+` ORDER BY 1, 2, 3, 4`)
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
	return count, fmt.Sprintf("%x", h.Sum(nil))
}

func historyRangeArgs(fx historyFix, apply bool) []string {
	args := []string{
		"rebuild",
		"--learner", strconv.FormatInt(fx.user, 10),
		"--db", fx.path,
		"--from", "2026-10-18",
		"--to", "2026-10-21",
	}
	if apply {
		args = append(args, "--apply")
	}
	return args
}

func TestHistoryRebuildDryRunChangesNoRow(t *testing.T) {
	fx := historyFixture(t)
	snapN, snapSum := tableChecksum(t, fx.path, "daily_snapshots")
	totN, totSum := tableChecksum(t, fx.path, "daily_totals")
	if snapN == 0 || totN == 0 {
		t.Fatalf("fixture empty: snapshots=%d totals=%d", snapN, totN)
	}

	var stdout, stderr bytes.Buffer
	err := historyCommand(historyRangeArgs(fx, false), &stdout, &stderr)
	if err != nil {
		t.Fatalf("dry-run: %v\n%s\n%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "mode=dry-run") {
		t.Fatalf("stdout:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "applied") {
		t.Fatalf("dry-run mentioned applied:\n%s", stdout.String())
	}

	snapN2, snapSum2 := tableChecksum(t, fx.path, "daily_snapshots")
	totN2, totSum2 := tableChecksum(t, fx.path, "daily_totals")
	if snapN2 != snapN || snapSum2 != snapSum {
		t.Fatalf("daily_snapshots changed: count %d→%d sum %s→%s", snapN, snapN2, snapSum, snapSum2)
	}
	if totN2 != totN || totSum2 != totSum {
		t.Fatalf("daily_totals changed: count %d→%d sum %s→%s", totN, totN2, totSum, totSum2)
	}
}

func TestHistoryRebuildApplyTwiceIdempotentAndEventsUntouched(t *testing.T) {
	fx := historyFixture(t)
	ctx := context.Background()

	eventsBefore := countRegressions(t, fx.path)

	var first, second bytes.Buffer
	args := historyRangeArgs(fx, true)
	if err := historyCommand(args, &first, &bytes.Buffer{}); err != nil {
		t.Fatalf("first apply: %v\n%s", err, first.String())
	}
	if !strings.Contains(first.String(), "applied") {
		t.Fatalf("first apply stdout:\n%s", first.String())
	}
	snapN1, snapSum1 := tableChecksum(t, fx.path, "daily_snapshots")
	totN1, totSum1 := tableChecksum(t, fx.path, "daily_totals")

	if err := historyCommand(args, &second, &bytes.Buffer{}); err != nil {
		t.Fatalf("second apply: %v\n%s", err, second.String())
	}
	snapN2, snapSum2 := tableChecksum(t, fx.path, "daily_snapshots")
	totN2, totSum2 := tableChecksum(t, fx.path, "daily_totals")
	if snapN1 != snapN2 || snapSum1 != snapSum2 {
		t.Fatalf("daily_snapshots changed on second apply")
	}
	if totN1 != totN2 || totSum1 != totSum2 {
		t.Fatalf("daily_totals changed on second apply")
	}
	// New-side of the printed table is identical across applies.
	if historyNewSide(first.String()) != historyNewSide(second.String()) {
		t.Fatalf("new-side tables differ\nfirst:\n%s\nsecond:\n%s", first.String(), second.String())
	}
	if eventsAfter := countRegressions(t, fx.path); eventsAfter != eventsBefore {
		t.Fatalf("regression events %d → %d", eventsBefore, eventsAfter)
	}

	store, err := db.OpenQueryOnly(ctx, fx.path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rows, err := store.SnapshotRows(ctx, fx.user, "2026-10-18", "2026-10-21")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.RuleVersion != jobs.RuleVersion {
			t.Fatalf("rule_version=%d on %s %s, want %d", r.RuleVersion, r.Day, r.Skill, jobs.RuleVersion)
		}
	}
}

// historyNewSide keeps only the →new half of each cell so two applies compare equal
// even when the first write fills previously missing old values.
func historyNewSide(out string) string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "history rebuild:") || strings.HasPrefix(line, "applied ") || line == "" {
			continue
		}
		if strings.HasPrefix(line, "day") {
			lines = append(lines, line)
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		var b strings.Builder
		b.WriteString(fields[0])
		for _, f := range fields[1:] {
			if i := strings.LastIndex(f, "→"); i >= 0 {
				b.WriteByte(' ')
				b.WriteString(f[i+len("→"):])
			}
		}
		lines = append(lines, b.String())
	}
	return strings.Join(lines, "\n")
}

func countRegressions(t *testing.T, path string) int {
	t.Helper()
	ctx := context.Background()
	store, err := db.OpenQueryOnly(ctx, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM regression_events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestHistoryRebuildRefusesWhileDailyLockHeld(t *testing.T) {
	fx := historyFixture(t)
	ctx := context.Background()
	// history rebuild uses the system clock when opening the DB; stamp the lock with now
	// so DailyLockHolder still sees a live holder.
	store, err := db.Open(ctx, fx.path, nil)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := store.TryLockJob(ctx, jobs.DailyLockKey, "job-b", store.Clock().Now(), jobs.LockTTL)
	if err != nil || !ok {
		store.Close()
		t.Fatalf("lock = %v %v", ok, err)
	}
	store.Close()

	var stdout, stderr bytes.Buffer
	err = historyCommand(historyRangeArgs(fx, true), &stdout, &stderr)
	if err == nil {
		t.Fatal("expected refusal while locked")
	}
	if !strings.Contains(err.Error(), "job-b") {
		t.Fatalf("error %q does not name holder", err)
	}
	code, _ := statusCode(err)
	if code != 1 {
		t.Fatalf("exit code=%d, want 1", code)
	}
}

func TestHistoryRebuildUnknownLearnerExits1(t *testing.T) {
	fx := historyFixture(t)
	var stdout, stderr bytes.Buffer
	err := historyCommand([]string{"rebuild", "--learner", "99999", "--db", fx.path}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected unknown learner error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error %q", err)
	}
	code, _ := statusCode(err)
	if code != 1 {
		t.Fatalf("exit code=%d, want 1", code)
	}
}

func TestHistoryRebuildNoEvidenceExits1(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	t.Setenv("TZ", "America/Chicago")
	path := filepath.Join(dir, "hebrew.sqlite")
	ctx := context.Background()
	when := time.Date(2026, 10, 22, 17, 0, 0, 0, time.UTC)
	store, err := db.Open(ctx, path, fixedClock{when})
	if err != nil {
		t.Fatal(err)
	}
	user, err := store.CreateUser(ctx, "empty@example.invalid", "sub-empty", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err = historyCommand([]string{"rebuild", "--learner", strconv.FormatInt(user.ID, 10), "--db", path}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected no evidence error")
	}
	if !strings.Contains(err.Error(), "no evidence") {
		t.Fatalf("error %q", err)
	}
}
