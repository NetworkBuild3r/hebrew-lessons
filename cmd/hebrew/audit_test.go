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
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
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
	if !strings.Contains(out, "known_any_skill") || !strings.Contains(out, "spell_cards") || !strings.Contains(out, "zone") {
		t.Fatalf("missing rows:\n%s", out)
	}
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
	if _, err := store.InsertCard(ctx, db.Card{
		UserID: user.ID, LexemeID: typed, Skill: fsrs.SkillSpell, State: fsrs.StateReview,
		Due: when, Stability: 10, Difficulty: 5, Reps: 3, LastReview: when.AddDate(0, 0, -1),
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		at := when.AddDate(0, 0, -(3 - i))
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

func seedAuditKnown(t *testing.T, store *db.Store, user, lexeme int64, skill string, now time.Time) {
	t.Helper()
	ctx := context.Background()
	last := now.AddDate(0, 0, -1)
	id, err := store.InsertCard(ctx, db.Card{
		UserID: user, LexemeID: lexeme, Skill: skill, State: fsrs.StateReview,
		Due: now, Stability: 10, Difficulty: 5, Reps: 3, LastReview: last,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		at := last.Add(-time.Duration(2-i) * 24 * time.Hour)
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
