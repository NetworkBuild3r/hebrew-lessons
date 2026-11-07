package main

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/jobs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// INIT-032/SPEC-009: hebrew audit. Temp databases only.

func TestAuditFixtureMatchesAndExits0(t *testing.T) {
	fx := auditFixture(t)
	var stdout, stderr bytes.Buffer
	err := auditCommand([]string{"--learner", strconv.FormatInt(fx.user, 10), "--db", fx.path}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("audit: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[len(fields)-1] == "no" {
			t.Fatalf("mismatch line: %s\nfull:\n%s", line, out)
		}
	}
	for _, name := range []string{
		"known_any_skill", "spell_cards", "zone",
		"history_rule1_days", "history_old_rule_days", "history_last_day", "replay_mismatch",
		"review_skipped_no_audio", "read.verdict", "recall.accuracy14",
	} {
		if !strings.Contains(out, name) {
			t.Fatalf("missing row %s:\n%s", name, out)
		}
	}
}

func TestAuditOldRuleDaysExits1(t *testing.T) {
	fx := auditFixture(t)
	ctx := context.Background()
	store, err := db.Open(ctx, fx.path, fixedClock{time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	day := "2026-11-03"
	if err := store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		return tx.InsertTotals(ctx, fx.user, day, 1, 2) // rule_version 2 < jobs.RuleVersion 3
	}); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err = auditCommand([]string{"--learner", strconv.FormatInt(fx.user, 10), "--db", fx.path}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("want exit 1 with old-rule row; stdout:\n%s", stdout.String())
	}
	qe, ok := err.(interface{ ExitCode() int })
	if !ok || qe.ExitCode() != 1 {
		t.Fatalf("err=%v, want exit 1", err)
	}
	if !strings.Contains(err.Error(), "history_old_rule_days") {
		t.Fatalf("error %q does not name history_old_rule_days", err)
	}
}

func TestAuditRule1DaysThenRebuild(t *testing.T) {
	fx := auditFixture(t)
	ctx := context.Background()
	store, err := db.Open(ctx, fx.path, fixedClock{time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	day := "2026-11-04"
	if err := store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		if err := tx.InsertSnapshot(ctx, fx.user, day, []db.DailySnapshotRow{
			{Skill: "read", State: "known", Count: 1, EstKnown: 1, RuleVersion: 1},
		}); err != nil {
			return err
		}
		return tx.InsertTotals(ctx, fx.user, day, 1, 1)
	}); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err = auditCommand([]string{"--learner", strconv.FormatInt(fx.user, 10), "--db", fx.path}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("want exit 1 with rule-1 row; stdout:\n%s", stdout.String())
	}
	qe, ok := err.(interface{ ExitCode() int })
	if !ok || qe.ExitCode() != 1 {
		t.Fatalf("err=%v, want exit 1", err)
	}
	if !strings.Contains(err.Error(), "history_rule1_days") {
		t.Fatalf("error %q does not name history_rule1_days", err)
	}

	store, err = db.Open(ctx, fx.path, fixedClock{time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	model := learner.New(store, store.Clock())
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	z := mustAuditZone(t, "America/Denver", loc)
	at, err := jobs.EndOfDay(day, z)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := model.SnapshotAt(ctx, fx.user, at)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]db.DailySnapshotRow, 0, len(learner.Skills))
	for _, sk := range snap.Summary.Skills {
		rows = append(rows, db.DailySnapshotRow{
			Skill: sk.Skill, State: "known",
			Count:    sk.Counts[learner.Known] + sk.Counts[learner.Solid],
			EstKnown: sk.EstimatedKnown, RuleVersion: jobs.RuleVersion,
		})
	}
	if err := store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		return tx.ReplaceDay(ctx, fx.user, day, rows, snap.Summary.KnownAnySkill, jobs.RuleVersion)
	}); err != nil {
		store.Close()
		t.Fatal(err)
	}
	// Also restamp yesterday from the fixture if it lacked totals rule version 2.
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	err = auditCommand([]string{"--learner", strconv.FormatInt(fx.user, 10), "--db", fx.path}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("after rebuild: %v\nstdout:\n%s", err, stdout.String())
	}
}

func mustAuditZone(t *testing.T, name string, loc *time.Location) studyday.Zone {
	t.Helper()
	return studyday.Zone{Name: name, Loc: loc, Source: studyday.SourceLearner}
}

func TestAuditPlantedMismatchExits1NamesRow(t *testing.T) {
	fx := auditFixture(t)
	ctx := context.Background()
	store, err := db.OpenQueryOnly(ctx, fx.path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	rows, err := runAuditDB(ctx, store, fx.user)
	if err != nil {
		t.Fatal(err)
	}
	planted := plantMismatch(rows, "known_any_skill")
	if name := firstMismatch(planted); name != "known_any_skill" {
		t.Fatalf("firstMismatch=%q, want known_any_skill", name)
	}
	var stdout bytes.Buffer
	printAuditRows(&stdout, fx.user, planted)
	if !strings.Contains(stdout.String(), "known_any_skill") || !strings.Contains(stdout.String(), "no") {
		t.Fatalf("planted output:\n%s", stdout.String())
	}
	err = quietExit{code: 1, msg: "audit: mismatch on known_any_skill"}
	code, _ := statusCode(err)
	if code != 1 {
		t.Fatalf("exit code=%d, want 1", code)
	}
	if !strings.Contains(err.Error(), "known_any_skill") {
		t.Fatalf("error %q does not name the row", err)
	}
}

func TestAuditQueryOnlyWriteFails(t *testing.T) {
	fx := auditFixture(t)
	ctx := context.Background()
	store, err := db.OpenQueryOnly(ctx, fx.path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	_, err = store.ExecContext(ctx, `UPDATE users SET role = role WHERE id = ?`, fx.user)
	if err == nil {
		t.Fatal("expected write to fail under PRAGMA query_only")
	}
	// Also prove a second connection opened for audit cannot write via sql.
	conn, err := sql.Open("sqlite", "file:"+filepath.ToSlash(fx.path)+"?mode=rw")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `PRAGMA query_only = ON`); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, `INSERT INTO lexemes (text_nfc) VALUES ('x')`); err == nil {
		t.Fatal("expected INSERT to fail under query_only")
	}
}

func TestAuditRecomputeAgreesWithSummary(t *testing.T) {
	fx := auditFixture(t)
	ctx := context.Background()
	store, err := db.OpenQueryOnly(ctx, fx.path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	model := learner.New(store, nil)
	re, err := model.Recompute(ctx, fx.user)
	if err != nil {
		t.Fatal(err)
	}
	sum, err := model.Summary(ctx, fx.user)
	if err != nil {
		t.Fatal(err)
	}
	if re.KnownAnySkill == 0 {
		t.Fatal("fixture KnownAnySkill is 0")
	}
	if re.KnownAnySkill != sum.KnownAnySkill {
		t.Fatalf("recompute KnownAnySkill=%d summary=%d", re.KnownAnySkill, sum.KnownAnySkill)
	}
	if re.SpellCards < 1 {
		t.Fatalf("spell cards=%d, want at least 1 from typed credit", re.SpellCards)
	}
	if !re.ZoneOK || re.Zone != "America/Denver" {
		t.Fatalf("zone=%q ok=%v, want America/Denver", re.Zone, re.ZoneOK)
	}
}

type auditFix struct {
	path string
	user int64
}

func auditFixture(t *testing.T) auditFix {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	path := filepath.Join(dir, "hebrew.sqlite")
	when := time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, err := db.Open(ctx, path, fixedClock{when})
	if err != nil {
		t.Fatal(err)
	}

	user, err := store.CreateUser(ctx, "audit@example.invalid", "sub-audit", "learner")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetUserTimeZone(ctx, user.ID, "America/Denver"); err != nil {
		t.Fatal(err)
	}

	// One known/solid card per skill + typed spelling credit (spell card).
	lexemes := []string{"אֶרֶץ", "שָׁלוֹם", "כֶּלֶב", "מַיִם"}
	skills := []string{fsrs.SkillListen, fsrs.SkillRead, fsrs.SkillRecall, fsrs.SkillSpell}
	ids := make([]int64, len(lexemes))
	for i, text := range lexemes {
		ids[i] = insertLexeme(t, store, text)
		seedAuditKnown(t, store, user.ID, ids[i], skills[i], when)
	}
	// Correct type_it on a fifth word credits spelling (extra spell card).
	typed := insertLexeme(t, store, "לֶחֶם")
	seedAuditKnown(t, store, user.ID, typed, fsrs.SkillRecall, when)
	insertTypedRecall(t, store, user.ID, typed, "bread", "לֶחֶם", true, false, when.Add(-time.Hour))
	// Apply spelling credit the same way the product does: spell attempt + card.
	spellAt := []time.Time{when.AddDate(0, 0, -3), when.AddDate(0, 0, -2), when.AddDate(0, 0, -1)}
	spell := replayedCard(t, spellAt)
	spell.UserID, spell.LexemeID, spell.Skill = user.ID, typed, fsrs.SkillSpell
	if _, err := store.InsertCard(ctx, spell); err != nil {
		t.Fatal(err)
	}
	for _, at := range spellAt {
		if _, err := store.ExecContext(ctx, `
			INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
			VALUES ((SELECT id FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = 'spell'), 3, ?, 1, 1, 0)`,
			user.ID, typed, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
		if _, err := store.ExecContext(ctx, `
			INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, 'spell', 'type_it', 'review', 'bread', 'לֶחֶם', 1, 0, 0, 3000, ?)`,
			user.ID, typed, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}

	// Yesterday snapshot only (no today) — Denver civil yesterday relative to when.
	yesterday := "2026-11-05"
	if err := store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		return tx.InsertSnapshot(ctx, user.ID, yesterday, []db.DailySnapshotRow{
			{Skill: "read", State: "known", Count: 1, EstKnown: 1},
		})
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return auditFix{path: path, user: user.ID}
}

type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

// replayedCard is what FSRS makes of Good reviews at these times, so a card stored with it
// agrees with a replay of its review_log (replay_mismatch 0).
func replayedCard(t *testing.T, reviews []time.Time) db.Card {
	t.Helper()
	sched := fsrs.New(nil, nil)
	var card *db.Card
	for _, at := range reviews {
		next, _, err := sched.ScheduleNext(card, at, fsrs.Rating(3))
		if err != nil {
			t.Fatal(err)
		}
		card = &next
	}
	return *card
}

func seedAuditKnown(t *testing.T, store *db.Store, user, lexeme int64, skill string, now time.Time) {
	t.Helper()
	ctx := context.Background()
	last := now.AddDate(0, 0, -1)
	reviews := []time.Time{last.Add(-48 * time.Hour), last.Add(-24 * time.Hour), last}
	card := replayedCard(t, reviews)
	card.UserID, card.LexemeID, card.Skill = user, lexeme, skill
	id, err := store.InsertCard(ctx, card)
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range reviews {
		if _, err := store.ExecContext(ctx, `
			INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, implied)
			VALUES (?, 3, ?, 1, 1, 0)`, id, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
		if _, err := store.ExecContext(ctx, `
			INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
			user, lexeme, skill, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAuditReplayMismatchExits1(t *testing.T) {
	fx := auditFixture(t)
	ctx := context.Background()
	store, err := db.Open(ctx, fx.path, fixedClock{time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `
		UPDATE cards SET stability = stability + 1
		WHERE id = (SELECT MIN(card_id) FROM review_log)`); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err = auditCommand([]string{"--learner", strconv.FormatInt(fx.user, 10), "--db", fx.path}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "replay_mismatch") {
		t.Fatalf("a card that disagrees with its replay should fail on replay_mismatch; err=%v\n%s", err, stdout.String())
	}
}

func TestAuditZonelessLearnerStillRecomputesHistory(t *testing.T) {
	fx := auditFixture(t)
	t.Setenv("TZ", "America/Denver")
	ctx := context.Background()
	store, err := db.Open(ctx, fx.path, fixedClock{time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `UPDATE users SET time_zone = NULL WHERE id = ?`, fx.user); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		return tx.InsertTotals(ctx, fx.user, "2026-11-04", 999, jobs.RuleVersion)
	}); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	err = auditCommand([]string{"--learner", strconv.FormatInt(fx.user, 10), "--db", fx.path}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "history_last_day") {
		t.Fatalf("a stored day of 999 must not agree with itself when the learner has no zone; err=%v\n%s", err, stdout.String())
	}
}
