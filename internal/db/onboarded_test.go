package db

import (
	"context"
	"io/fs"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

// An existing learner finished placement and started lessons before migration
// 014, so onboarded_at is null. IsOnboarded is still true, and the null is
// filled with the earliest lesson or finished-placement time, not "now" and
// not a placement attempt.
func TestIsOnboardedBackfillsAnExistingUserAt013(t *testing.T) {
	conn := openRaw(t)
	applyThrough(t, conn, 12)
	mustSQL(t, conn, `INSERT INTO schema_migrations (version, applied_at) VALUES (13, '2026-10-19T12:00:00.000000000Z')`)
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('brian@example.com', 'sub-brian', 'learner', '2026-08-01T00:00:00.000000000Z')`)
	var userID int64
	if err := conn.QueryRowContext(context.Background(), `SELECT id FROM users WHERE email = 'brian@example.com'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, conn, `INSERT INTO units (position, title, situation, concept, status, rev, doc) VALUES (1, 'Hello', 'street', 'greetings', 'published', 1, '{}')`)
	var unitID int64
	if err := conn.QueryRowContext(context.Background(), `SELECT id FROM units`).Scan(&unitID); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc) VALUES ('שָׁלוֹם')`)
	var lexemeID int64
	if err := conn.QueryRowContext(context.Background(), `SELECT id FROM lexemes`).Scan(&lexemeID); err != nil {
		t.Fatal(err)
	}
	// Earlier than every real candidate. A placement attempt must not win.
	mustSQL(t, conn, `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, 'read', 'pick', 'placement', 'p', 'r', 1, 0, 0, 10, '2026-09-01T09:00:00.000000000Z')`, userID, lexemeID)
	mustSQL(t, conn, `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, 'read', 'pick', 'lesson', 'p', 'r', 1, 0, 0, 10, '2026-10-01T09:00:00.000000000Z')`, userID, lexemeID)
	mustSQL(t, conn, `INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at) VALUES (?, ?, 0, '2026-10-02T09:00:00.000000000Z')`, userID, unitID)
	mustSQL(t, conn, `INSERT INTO placement_runs (user_id, started_at, seed, sitting_no) VALUES (?, '2026-09-15T09:00:00.000000000Z', 1, 1)`, userID)
	mustSQL(t, conn, `INSERT INTO placement_runs (user_id, started_at, finished_at, level, result, seed, sitting_no)
		VALUES (?, '2026-10-03T08:00:00.000000000Z', '2026-10-03T09:00:00.000000000Z', 1, '{"bands":[]}', 2, 1)`, userID)

	body, err := fs.ReadFile(migrations.FS, "014_onboarding_placement_history.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(context.Background(), conn, "014_onboarding_placement_history.sql", 14, string(body), time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	path := dbFile(t, conn)
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 21, 15, 0, 0, 0, time.UTC)
	store, err := Open(context.Background(), path, fixedClock{now})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	yes, err := store.IsOnboarded(ctx, userID)
	if err != nil || !yes {
		t.Fatalf("IsOnboarded = %v, %v; want true", yes, err)
	}
	want := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	u, err := store.UserByEmail(ctx, "brian@example.com")
	if err != nil || u.OnboardedAt == nil || !u.OnboardedAt.Equal(want) {
		t.Fatalf("onboarded_at %+v %v, want %s", u.OnboardedAt, err, want)
	}
	again, err := store.IsOnboarded(ctx, userID)
	if err != nil || !again {
		t.Fatalf("second IsOnboarded = %v, %v", again, err)
	}
	u, err = store.UserByEmail(ctx, "brian@example.com")
	if err != nil || u.OnboardedAt == nil || !u.OnboardedAt.Equal(want) {
		t.Fatalf("second read moved onboarded_at to %+v %v", u.OnboardedAt, err)
	}
}

func TestIsOnboardedOnlyLessonAttempts(t *testing.T) {
	now := time.Date(2026, 10, 21, 15, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{now})
	ctx := context.Background()
	user := insertUser(t, s, "lesson@example.com", "sub-lesson", "learner")
	lex := insertLexeme(t, s, "שָׁלוֹם")
	at := "2026-10-08T11:00:00.000000000Z"
	mustExec(t, s, `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, 'read', 'pick', 'lesson', 'p', 'r', 1, 0, 0, 10, ?)`, user, lex, at)
	yes, err := s.IsOnboarded(ctx, user)
	if err != nil || !yes {
		t.Fatalf("IsOnboarded = %v, %v; want true", yes, err)
	}
	u, err := s.UserByEmail(ctx, "lesson@example.com")
	want := time.Date(2026, 10, 8, 11, 0, 0, 0, time.UTC)
	if err != nil || u.OnboardedAt == nil || !u.OnboardedAt.Equal(want) {
		t.Fatalf("onboarded_at %+v %v, want %s", u.OnboardedAt, err, want)
	}
}

func TestIsOnboardedOnlyFinishedPlacement(t *testing.T) {
	now := time.Date(2026, 10, 21, 15, 0, 0, 0, time.UTC)
	s := openStore(t, fixedClock{now})
	ctx := context.Background()
	user := insertUser(t, s, "placed@example.com", "sub-placed", "learner")
	res := mustExec(t, s, `INSERT INTO placement_runs (user_id, started_at, finished_at, level, result, seed, sitting_no)
		VALUES (?, '2026-10-04T08:00:00.000000000Z', '2026-10-04T10:00:00.000000000Z', 1, '{"bands":[]}', 1, 1)`, user)
	runID, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `INSERT INTO placement_answers (run_id, item_id, rating, shown_at, answered_at)
		VALUES (?, 'rated', 3, '2026-10-04T10:00:00.000000000Z', '2026-10-04T10:00:00.000000000Z')`, runID)
	yes, err := s.IsOnboarded(ctx, user)
	if err != nil || !yes {
		t.Fatalf("IsOnboarded = %v, %v; want true", yes, err)
	}
	u, err := s.UserByEmail(ctx, "placed@example.com")
	want := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
	if err != nil || u.OnboardedAt == nil || !u.OnboardedAt.Equal(want) {
		t.Fatalf("onboarded_at %+v %v, want %s", u.OnboardedAt, err, want)
	}
}

func TestIsOnboardedAbandonedPlacementIsFalse(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 21, 15, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	user := insertUser(t, s, "left@example.com", "sub-left", "learner")
	lex := insertLexeme(t, s, "שָׁלוֹם")
	mustExec(t, s, `INSERT INTO placement_runs (user_id, started_at, seed, sitting_no) VALUES (?, ?, 1, 1)`, user, ts)
	mustExec(t, s, `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, 'read', 'pick', 'placement', 'p', 'r', 1, 0, 0, 10, ?)`, user, lex, ts)
	yes, err := s.IsOnboarded(ctx, user)
	if err != nil || yes {
		t.Fatalf("IsOnboarded = %v, %v; want false", yes, err)
	}
	u, err := s.UserByEmail(ctx, "left@example.com")
	if err != nil || u.OnboardedAt != nil {
		t.Fatalf("abandoned placement set onboarded_at %+v %v", u.OnboardedAt, err)
	}
}

func TestIsOnboardedFreshUserIsFalse(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 21, 15, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	user := insertUser(t, s, "new@example.com", "sub-new", "learner")
	yes, err := s.IsOnboarded(ctx, user)
	if err != nil || yes {
		t.Fatalf("IsOnboarded = %v, %v; want false", yes, err)
	}
	u, err := s.UserByEmail(ctx, "new@example.com")
	if err != nil || u.OnboardedAt != nil {
		t.Fatalf("fresh user onboarded_at %+v %v", u.OnboardedAt, err)
	}
}

func TestIsOnboardedKeepsAnExistingTime(t *testing.T) {
	s := openStore(t, fixedClock{time.Date(2026, 10, 21, 15, 0, 0, 0, time.UTC)})
	ctx := context.Background()
	user := insertUser(t, s, "kept@example.com", "sub-kept", "learner")
	kept := time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)
	if err := s.SetOnboarded(ctx, user, kept); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `INSERT INTO placement_runs (user_id, started_at, finished_at, level, result, seed, sitting_no)
		VALUES (?, '2026-10-01T08:00:00.000000000Z', '2026-10-01T09:00:00.000000000Z', 1, '{"bands":[]}', 1, 1)`, user)
	yes, err := s.IsOnboarded(ctx, user)
	if err != nil || !yes {
		t.Fatalf("IsOnboarded = %v, %v; want true", yes, err)
	}
	u, err := s.UserByEmail(ctx, "kept@example.com")
	if err != nil || u.OnboardedAt == nil || !u.OnboardedAt.Equal(kept) {
		t.Fatalf("onboarded_at %+v %v, want %s", u.OnboardedAt, err, kept)
	}
}

// The rollback lines in the 014 header comment undo the additive schema.
func TestMigrate014RollbackDropsTheAdditiveSchema(t *testing.T) {
	conn := openRaw(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	var version string
	if err := conn.QueryRowContext(context.Background(), `SELECT sqlite_version()`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	t.Logf("sqlite_version=%s", version)

	for _, stmt := range []string{
		`DROP INDEX placement_runs_user_finished_desc`,
		`ALTER TABLE placement_runs DROP COLUMN words_known`,
		`ALTER TABLE placement_runs DROP COLUMN start_unit`,
		`ALTER TABLE users DROP COLUMN onboarded_at`,
	} {
		if _, err := conn.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if columnCount(t, conn, "users", "onboarded_at") != 0 {
		t.Fatal("users.onboarded_at still exists")
	}
	if columnCount(t, conn, "placement_runs", "start_unit") != 0 || columnCount(t, conn, "placement_runs", "words_known") != 0 {
		t.Fatal("placement snapshot columns still exist")
	}
	var n int
	if err := conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'placement_runs_user_finished_desc'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("placement_runs_user_finished_desc still exists")
	}
}
