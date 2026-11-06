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
)

// INIT-032/SPEC-008: hebrew backfill-spell. Temp databases only.

const (
	backfillEmail = "backfill-marker@example.invalid"
	kelev         = "כֶּלֶב"
	kelevNear     = "כֶּלֶפ" // one letter off → Hard
	shalom        = "שָׁלוֹם"
)

type backfillFixture struct {
	path   string
	user   int64
	kelev  int64
	shalom int64
	t0     time.Time
}

func backfillDB(t *testing.T) backfillFixture {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	path := filepath.Join(dir, "hebrew.sqlite")
	ctx := context.Background()
	store, err := db.Open(ctx, path, nil)
	if err != nil {
		t.Fatal(err)
	}

	user, err := store.InsertUser(ctx, backfillEmail, "sub-backfill", "learner")
	if err != nil {
		t.Fatal(err)
	}
	kelevID := insertLexeme(t, store, kelev)
	shalomID := insertLexeme(t, store, shalom)
	t0 := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

	// Correct type_it on recall → spelling Good.
	insertTypedRecall(t, store, user, kelevID, "dog", kelev, true, false, t0)
	// Near miss → spelling Hard.
	insertTypedRecall(t, store, user, kelevID, "dog", kelevNear, true, false, t0.Add(time.Minute))
	// Wrong → skipped.
	insertTypedRecall(t, store, user, shalomID, "peace", "לא", false, false, t0.Add(2*time.Minute))
	// Don't-know → skipped.
	insertTypedRecall(t, store, user, shalomID, "peace", "", false, true, t0.Add(3*time.Minute))
	// Exact on a second lexeme → Good + second card.
	insertTypedRecall(t, store, user, shalomID, "peace", shalom, true, false, t0.Add(4*time.Minute))

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return backfillFixture{path: path, user: user, kelev: kelevID, shalom: shalomID, t0: t0}
}

func openBackfillDB(t *testing.T, path string) *db.Store {
	t.Helper()
	store, err := db.Open(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func learnerFlag(fx backfillFixture) []string {
	return []string{"--learner", strconv.FormatInt(fx.user, 10), "--db", fx.path}
}

func insertLexeme(t *testing.T, store *db.Store, text string) int64 {
	t.Helper()
	res, err := store.ExecContext(context.Background(), `INSERT INTO lexemes (text_nfc) VALUES (?)`, text)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertTypedRecall(t *testing.T, store *db.Store, user, lexeme int64, prompt, response string, correct, dontKnow bool, at time.Time) {
	t.Helper()
	_, err := store.WriteAttempt(context.Background(), db.AttemptWrite{Attempt: db.Attempt{
		UserID:    user,
		LexemeID:  lexeme,
		Skill:     fsrs.SkillRecall,
		Activity:  activityTypeIt,
		Source:    "review",
		Prompt:    prompt,
		Response:  response,
		Correct:   correct,
		DontKnow:  dontKnow,
		LatencyMS: 3000,
		CreatedAt: at,
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func countSQL(t *testing.T, store *db.Store, query string, args ...any) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func digestRows(t *testing.T, store *db.Store) (attempts, cards, reviews int) {
	t.Helper()
	attempts = countSQL(t, store, `SELECT COUNT(*) FROM attempts`)
	cards = countSQL(t, store, `SELECT COUNT(*) FROM cards`)
	reviews = countSQL(t, store, `SELECT COUNT(*) FROM review_log`)
	return
}

func TestBackfillSpellDryRunPrintsCountsAndWritesNothing(t *testing.T) {
	fx := backfillDB(t)
	store := openBackfillDB(t, fx.path)
	beforeA, beforeC, beforeR := digestRows(t, store)
	_ = store.Close()

	var out, errOut bytes.Buffer
	if err := backfillSpellCommand(learnerFlag(fx), &out, &errOut); err != nil {
		t.Fatalf("err = %v\n%s%s", err, out.String(), errOut.String())
	}
	got := out.String()
	for _, want := range []string{
		"mode=dry-run",
		"learner=" + strconv.FormatInt(fx.user, 10),
		"lexemes=2",
		"cards_to_create=2",
		"reviews_to_write=3",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
	store = openBackfillDB(t, fx.path)
	afterA, afterC, afterR := digestRows(t, store)
	if afterA != beforeA || afterC != beforeC || afterR != beforeR {
		t.Fatalf("dry run wrote rows: attempts %d→%d cards %d→%d reviews %d→%d",
			beforeA, afterA, beforeC, afterC, beforeR, afterR)
	}
}

func TestBackfillSpellApplyTwiceMatchesOnce(t *testing.T) {
	fx := backfillDB(t)
	var out, errOut bytes.Buffer
	args := append(learnerFlag(fx), "--apply", "--backup-taken")
	if err := backfillSpellCommand(args, &out, &errOut); err != nil {
		t.Fatalf("first apply: %v\n%s%s", err, out.String(), errOut.String())
	}
	store := openBackfillDB(t, fx.path)
	spellAttempts := countSQL(t, store, `SELECT COUNT(*) FROM attempts WHERE skill = 'spell'`)
	spellCards := countSQL(t, store, `SELECT COUNT(*) FROM cards WHERE skill = 'spell'`)
	implied := countSQL(t, store, `SELECT COUNT(*) FROM review_log WHERE implied = 1`)
	if spellAttempts != 3 || spellCards != 2 || implied != 3 {
		t.Fatalf("after first apply: attempts=%d cards=%d implied=%d; want 3, 2, 3",
			spellAttempts, spellCards, implied)
	}
	_ = store.Close()

	out.Reset()
	errOut.Reset()
	if err := backfillSpellCommand(args, &out, &errOut); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if !strings.Contains(out.String(), "reviews_to_write=0") || !strings.Contains(out.String(), "cards_to_create=0") {
		t.Fatalf("second apply should be a no-op:\n%s", out.String())
	}
	store = openBackfillDB(t, fx.path)
	if countSQL(t, store, `SELECT COUNT(*) FROM attempts WHERE skill = 'spell'`) != spellAttempts {
		t.Fatal("second apply changed spell attempts")
	}
	if countSQL(t, store, `SELECT COUNT(*) FROM cards WHERE skill = 'spell'`) != spellCards {
		t.Fatal("second apply changed spell cards")
	}
	if countSQL(t, store, `SELECT COUNT(*) FROM review_log WHERE implied = 1`) != implied {
		t.Fatal("second apply changed implied reviews")
	}
}

func TestBackfillSpellNearMissHardWrongSkipped(t *testing.T) {
	fx := backfillDB(t)
	var out, errOut bytes.Buffer
	args := append(learnerFlag(fx), "--apply", "--backup-taken")
	if err := backfillSpellCommand(args, &out, &errOut); err != nil {
		t.Fatal(err)
	}

	store := openBackfillDB(t, fx.path)
	// Near-miss review on kelev is Hard.
	var hardRating int
	err := mustConn(t, store).QueryRowContext(context.Background(), `
		SELECT rl.rating
		FROM review_log AS rl
		JOIN cards AS c ON c.id = rl.card_id
		WHERE c.user_id = ? AND c.lexeme_id = ? AND c.skill = 'spell' AND rl.implied = 1
		ORDER BY rl.reviewed_at, rl.id
		LIMIT 1 OFFSET 1`, fx.user, fx.kelev).Scan(&hardRating)
	if err != nil {
		t.Fatal(err)
	}
	if hardRating != int(fsrs.Hard) {
		t.Fatalf("near-miss rating = %d, want Hard (%d)", hardRating, fsrs.Hard)
	}

	// First kelev credit is Good.
	var goodRating int
	err = mustConn(t, store).QueryRowContext(context.Background(), `
		SELECT rl.rating
		FROM review_log AS rl
		JOIN cards AS c ON c.id = rl.card_id
		WHERE c.user_id = ? AND c.lexeme_id = ? AND c.skill = 'spell' AND rl.implied = 1
		ORDER BY rl.reviewed_at, rl.id
		LIMIT 1`, fx.user, fx.kelev).Scan(&goodRating)
	if err != nil {
		t.Fatal(err)
	}
	if goodRating != int(fsrs.Good) {
		t.Fatalf("exact rating = %d, want Good (%d)", goodRating, fsrs.Good)
	}

	wrongSpell := countSQL(t, store, `
		SELECT COUNT(*) FROM attempts
		WHERE user_id = ? AND lexeme_id = ? AND skill = 'spell'
		  AND created_at IN (?, ?)`,
		fx.user, fx.shalom,
		db.FormatTime(fx.t0.Add(2*time.Minute)),
		db.FormatTime(fx.t0.Add(3*time.Minute)),
	)
	if wrongSpell != 0 {
		t.Fatalf("wrong/dont-know produced %d spell attempts", wrongSpell)
	}
}

func TestBackfillSpellApplyWithoutBackupTakenWritesNothing(t *testing.T) {
	fx := backfillDB(t)
	store := openBackfillDB(t, fx.path)
	beforeA, beforeC, beforeR := digestRows(t, store)
	_ = store.Close()

	var out, errOut bytes.Buffer
	args := append(learnerFlag(fx), "--apply")
	err := backfillSpellCommand(args, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--backup-taken") {
		t.Fatalf("err = %v, want refusal naming --backup-taken", err)
	}
	if out.Len() != 0 {
		t.Errorf("printed %q on refusal", out.String())
	}
	store = openBackfillDB(t, fx.path)
	afterA, afterC, afterR := digestRows(t, store)
	if afterA != beforeA || afterC != beforeC || afterR != beforeR {
		t.Fatal("--apply without --backup-taken wrote rows")
	}
}

func TestBackfillSpellOutputHasNoLexemeTextOrEmail(t *testing.T) {
	fx := backfillDB(t)
	var out, errOut bytes.Buffer
	args := append(learnerFlag(fx), "--apply", "--backup-taken")
	if err := backfillSpellCommand(args, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	all := out.String() + errOut.String()
	for _, secret := range []string{backfillEmail, kelev, kelevNear, shalom, "dog", "peace"} {
		if strings.Contains(all, secret) {
			t.Errorf("output contains %q:\n%s", secret, all)
		}
	}
}

func mustConn(t *testing.T, store *db.Store) *sql.DB {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	return conn
}
