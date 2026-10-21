package db

import (
	"context"
	"database/sql"
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

// Migration 014 adds nullable columns and one index. It does not rebuild a table.
// 013_learner_tz is reserved by another branch and is not in this binary, so a
// database at 013 is one whose schema_migrations already records 13 and whose
// tables are still the 012 shape, with rows already in them.

func TestMigrate014OnAFreshDatabase(t *testing.T) {
	conn := openRaw(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	assertOnboardingColumns(t, conn)
	var n int
	if err := conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM schema_migrations WHERE version = 14`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("version 14 recorded %d times", n)
	}
}

func TestMigrate014OnADatabaseAt013KeepsNullsAndOldReads(t *testing.T) {
	conn := openRaw(t)
	applyThrough(t, conn, 12)
	mustSQL(t, conn, `INSERT INTO schema_migrations (version, applied_at) VALUES (13, '2026-10-19T12:00:00.000000000Z')`)
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('old@example.com', 'sub-old', 'learner', '2026-10-01T00:00:00.000000000Z')`)
	var userID int64
	if err := conn.QueryRowContext(context.Background(), `SELECT id FROM users WHERE email = 'old@example.com'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	scoredRes, err := conn.ExecContext(context.Background(), `INSERT INTO placement_runs (user_id, started_at, finished_at, level, stop_reason, result, seed, sitting_no)
		VALUES (?, '2026-10-02T08:00:00.000000000Z', '2026-10-02T08:10:00.000000000Z', 1, 'exhausted', '{"bands":[],"estimated_known_words":4}', 7, 1)`, userID)
	if err != nil {
		t.Fatal(err)
	}
	scoredID, err := scoredRes.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	mustSQL(t, conn, `INSERT INTO placement_answers (run_id, item_id, rating, shown_at, answered_at)
		VALUES (?, 'rated', 3, '2026-10-02T08:10:00.000000000Z', '2026-10-02T08:10:00.000000000Z')`, scoredID)
	// Three closes that set finished_at and rate nothing. They stay in the table
	// and are not a finished result: Start's auto-close, the old v2 Finish, and
	// finishNow's exhausted close.
	autoRes, err := conn.ExecContext(context.Background(), `INSERT INTO placement_runs (user_id, started_at, finished_at, level, stop_reason, result, seed, sitting_no)
		VALUES (?, '2026-09-01T08:00:00.000000000Z', '2026-09-01T08:05:00.000000000Z', -1, NULL, '{"bands":[],"estimated_known_words":0}', 1, 1)`, userID)
	if err != nil {
		t.Fatal(err)
	}
	autoID, err := autoRes.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	oldRes, err := conn.ExecContext(context.Background(), `INSERT INTO placement_runs (user_id, started_at, finished_at, level, stop_reason, result, seed, sitting_no)
		VALUES (?, '2026-09-02T08:00:00.000000000Z', '2026-09-02T08:05:00.000000000Z', -1, NULL, ?, 2, 1)`, userID, fiveZeroBands(t))
	if err != nil {
		t.Fatal(err)
	}
	oldID, err := oldRes.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	exRes, err := conn.ExecContext(context.Background(), `INSERT INTO placement_runs (user_id, started_at, finished_at, level, stop_reason, result, seed, sitting_no)
		VALUES (?, '2026-09-03T08:00:00.000000000Z', '2026-09-03T08:05:00.000000000Z', -1, 'exhausted', '{"bands":[],"estimated_known_words":0}', 3, 1)`, userID)
	if err != nil {
		t.Fatal(err)
	}
	exhaustedID, err := exRes.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	mustSQL(t, conn, `INSERT INTO placement_runs (user_id, started_at, seed, sitting_no)
		VALUES (?, '2026-10-03T08:00:00.000000000Z', 8, 1)`, userID)

	body, err := fs.ReadFile(migrations.FS, "014_onboarding_placement_history.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(context.Background(), conn, "014_onboarding_placement_history.sql", 14, string(body), time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	assertOnboardingColumns(t, conn)

	var email, role, created string
	var onboarded sql.NullString
	if err := conn.QueryRowContext(context.Background(), `SELECT email, google_sub, role, created_at, onboarded_at FROM users WHERE id = ?`, userID).Scan(&email, new(string), &role, &created, &onboarded); err != nil {
		t.Fatal(err)
	}
	if email != "old@example.com" || role != "learner" || created != "2026-10-01T00:00:00.000000000Z" || !onboarded.Valid || onboarded.String != "2026-09-02T08:05:00.000000000Z" {
		t.Fatalf("old user email=%s role=%s created=%s onboarded=%v", email, role, created, onboarded)
	}

	var started, finished string
	var level int
	var startUnit, wordsKnown sql.NullInt64
	var finishedRows int
	if err := conn.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM placement_runs WHERE finished_at IS NOT NULL`).Scan(&finishedRows); err != nil {
		t.Fatal(err)
	}
	if finishedRows != 4 {
		t.Fatalf("finished rows = %d, want the scored run and the three unscored closes kept", finishedRows)
	}
	if err := conn.QueryRowContext(context.Background(), `
		SELECT started_at, finished_at, level, start_unit, words_known
		FROM placement_runs WHERE id = ?`, scoredID).Scan(&started, &finished, &level, &startUnit, &wordsKnown); err != nil {
		t.Fatal(err)
	}
	if started != "2026-10-02T08:00:00.000000000Z" || finished != "2026-10-02T08:10:00.000000000Z" || level != 1 || startUnit.Valid || wordsKnown.Valid {
		t.Fatalf("old run started=%s finished=%s level=%d start=%v words=%v", started, finished, level, startUnit, wordsKnown)
	}
	var openStart sql.NullInt64
	if err := conn.QueryRowContext(context.Background(), `SELECT start_unit FROM placement_runs WHERE finished_at IS NULL`).Scan(&openStart); err != nil {
		t.Fatal(err)
	}
	if openStart.Valid {
		t.Fatalf("open run start_unit = %v, want NULL", openStart)
	}

	// The same rows still load through the store.
	path := dbFile(t, conn)
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(context.Background(), path, fixedClock{time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	u, err := store.UserByEmail(context.Background(), "old@example.com")
	wantOnboarded := time.Date(2026, 9, 2, 8, 5, 0, 0, time.UTC)
	if err != nil || u.OnboardedAt == nil || !u.OnboardedAt.Equal(wantOnboarded) || u.Email != "old@example.com" {
		t.Fatalf("loaded user %+v %v", u, err)
	}
	runs, err := store.PlacementHistory(context.Background(), u.ID, 0)
	if err != nil || !sameIDs(idsOfRuns(runs), []int64{scoredID, exhaustedID, oldID}) {
		t.Fatalf("history %+v %v, want %d %d %d and not the auto-close %d", runs, err, scoredID, exhaustedID, oldID, autoID)
	}
	if n, err := store.PlacementHistoryCount(context.Background(), u.ID); err != nil || n != 3 {
		t.Fatalf("count %d %v, want 3", n, err)
	}
}

// Migration 014 backfills onboarded_at once, from rows written before the
// column existed. Each learner is seeded through the store methods that write
// those rows. IsOnboarded only reads the column afterwards.
func TestMigrate014BackfillsOnboardedAt(t *testing.T) {
	conn := openRaw(t)
	applyThrough(t, conn, 12)
	mustSQL(t, conn, `INSERT INTO schema_migrations (version, applied_at) VALUES (13, '2026-10-19T12:00:00.000000000Z')`)

	body, err := fs.ReadFile(migrations.FS, "014_onboarding_placement_history.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), engineStartAutoCloseResult) {
		t.Fatal("014 backfill does not exclude the Engine.Start close result")
	}

	clk := &setClock{t: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	s := &Store{conn: conn, clock: clk}
	ctx := context.Background()
	unit := insertUnit(t, s, "published")
	lex, err := s.UpsertPlacementLexeme(ctx, "שָׁלוֹם", "peace", "noun", "shalom", "")
	if err != nil {
		t.Fatal(err)
	}

	finishedAt := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
	attemptAt := time.Date(2026, 10, 8, 11, 0, 0, 0, time.UTC)
	lessonAt := time.Date(2026, 10, 7, 8, 30, 0, 0, time.UTC)
	autoAt := time.Date(2026, 9, 1, 8, 5, 0, 0, time.UTC)
	brianLesson := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	brianAttempt := time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)
	brianFinish := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
	brianPlacementAttempt := time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)

	type seeded struct {
		id      int64
		want    *time.Time
		history int
	}
	cases := map[string]*seeded{}
	seed := func(name, email string) int64 {
		t.Helper()
		clk.t = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
		u, err := s.CreateUser(ctx, email, "sub-"+email, "learner")
		if err != nil {
			t.Fatal(err)
		}
		cases[name] = &seeded{id: u.ID}
		return u.ID
	}

	finishedUser := seed("finished_placement", "finished@example.com")
	finishScoredV2(t, s, finishedUser, finishedAt)
	cases["finished_placement"].want = &finishedAt
	cases["finished_placement"].history = 1

	autoUser := seed("auto_close_alone", "auto@example.com")
	closeLikeStart(t, s, autoUser, autoAt)

	autoV3 := seed("auto_close_and_abandoned_v3", "autov3@example.com")
	closeLikeStart(t, s, autoV3, autoAt)
	if _, err := s.StartPlacementV3(ctx, autoV3, autoAt.Add(time.Hour), 9, RunInitial, "", 0); err != nil {
		t.Fatal(err)
	}

	attemptUser := seed("lesson_attempt_only", "attempt@example.com")
	writeLessonAttempt(t, s, attemptUser, lex, "lesson", attemptAt)
	cases["lesson_attempt_only"].want = &attemptAt

	lessonUser := seed("lesson_run_only", "lesson@example.com")
	clk.t = lessonAt
	if _, err := s.StartPlayerRun(ctx, lessonUser, unit, 0); err != nil {
		t.Fatal(err)
	}
	cases["lesson_run_only"].want = &lessonAt

	seed("nothing", "nothing@example.com")

	brian := seed("brian", "brian-shape@example.com")
	closeLikeStart(t, s, brian, autoAt)
	writeLessonAttempt(t, s, brian, lex, "placement", brianPlacementAttempt)
	clk.t = brianLesson
	if _, err := s.StartPlayerRun(ctx, brian, unit, 0); err != nil {
		t.Fatal(err)
	}
	writeLessonAttempt(t, s, brian, lex, "lesson", brianAttempt)
	finishScoredV2(t, s, brian, brianFinish)
	cases["brian"].want = &brianLesson
	cases["brian"].history = 1

	if err := apply(ctx, conn, "014_onboarding_placement_history.sql", 14, string(body), time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{
		"finished_placement",
		"auto_close_alone",
		"auto_close_and_abandoned_v3",
		"lesson_attempt_only",
		"lesson_run_only",
		"nothing",
		"brian",
	} {
		c := cases[name]
		t.Run(name, func(t *testing.T) {
			var got sql.NullString
			if err := conn.QueryRowContext(ctx, `SELECT onboarded_at FROM users WHERE id = ?`, c.id).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if c.want == nil {
				if got.Valid {
					t.Fatalf("onboarded_at = %s, want NULL", got.String)
				}
			} else if !got.Valid || got.String != FormatTime(*c.want) {
				t.Fatalf("onboarded_at = %v, want %s", got, FormatTime(*c.want))
			}
			yes, err := s.IsOnboarded(ctx, c.id)
			if err != nil || yes != (c.want != nil) {
				t.Fatalf("IsOnboarded = %v, %v; want %v", yes, err, c.want != nil)
			}
			n, err := s.PlacementHistoryCount(ctx, c.id)
			if err != nil || n != c.history {
				t.Fatalf("PlacementHistoryCount = %d, %v; want %d", n, err, c.history)
			}
		})
	}

	var openV3 int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM placement_runs WHERE user_id = ? AND finished_at IS NULL`, autoV3).Scan(&openV3); err != nil {
		t.Fatal(err)
	}
	if openV3 != 1 {
		t.Fatalf("abandoned v3 runs = %d, want 1", openV3)
	}
	var autoKept int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM placement_runs WHERE user_id = ? AND finished_at IS NOT NULL`, autoUser).Scan(&autoKept); err != nil {
		t.Fatal(err)
	}
	if autoKept != 1 {
		t.Fatalf("auto-closed rows kept = %d, want 1", autoKept)
	}
}

type setClock struct{ t time.Time }

func (c *setClock) Now() time.Time { return c.t }

func finishScoredV2(t *testing.T, s *Store, user int64, finished time.Time) {
	t.Helper()
	ctx := context.Background()
	lex, err := s.UpsertPlacementLexeme(ctx, "שָׁלוֹם", "peace", "noun", "shalom", "")
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.InsertPlacementRun(ctx, user, finished.Add(-time.Hour), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ShowPlacementCard(ctx, run, "card", 1, finished.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.RatePlacementAnswer(ctx, run, "card", 3, lex, finished); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishPlacement(ctx, run, finished, 1, "band_stop", `{"bands":[{"band":0,"good":1}]}`); err != nil {
		t.Fatal(err)
	}
}

func closeLikeStart(t *testing.T, s *Store, user int64, finished time.Time) {
	t.Helper()
	ctx := context.Background()
	run, err := s.InsertPlacementRun(ctx, user, finished.Add(-5*time.Minute), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.FinishPlacement(ctx, run, finished, -1, "", engineStartAutoCloseResult); err != nil {
		t.Fatal(err)
	}
}

func writeLessonAttempt(t *testing.T, s *Store, user, lex int64, source string, at time.Time) {
	t.Helper()
	if _, err := s.WriteAttempt(context.Background(), AttemptWrite{Attempt: Attempt{
		UserID: user, LexemeID: lex, Skill: "read", Activity: "pick", Source: source,
		Prompt: "p", Response: "r", Correct: true, CreatedAt: at,
	}}); err != nil {
		t.Fatal(err)
	}
}

func assertOnboardingColumns(t *testing.T, conn *sql.DB) {
	t.Helper()
	if got := columnType(t, conn, "users", "onboarded_at"); got != "TEXT" || columnNotNull(t, conn, "users", "onboarded_at") {
		t.Fatalf("users.onboarded_at type=%s notnull=%v", got, columnNotNull(t, conn, "users", "onboarded_at"))
	}
	if got := columnType(t, conn, "placement_runs", "start_unit"); got != "INTEGER" || columnNotNull(t, conn, "placement_runs", "start_unit") {
		t.Fatalf("start_unit type=%s notnull=%v", got, columnNotNull(t, conn, "placement_runs", "start_unit"))
	}
	if got := columnType(t, conn, "placement_runs", "words_known"); got != "INTEGER" || columnNotNull(t, conn, "placement_runs", "words_known") {
		t.Fatalf("words_known type=%s notnull=%v", got, columnNotNull(t, conn, "placement_runs", "words_known"))
	}
	cols := indexXInfo(t, conn, "placement_runs_user_finished_desc")
	if len(cols) != 2 || cols[0].name != "user_id" || cols[0].desc || cols[1].name != "finished_at" || !cols[1].desc {
		t.Fatalf("index = %+v, want (user_id, finished_at DESC)", cols)
	}
}

func applyThrough(t *testing.T, conn *sql.DB, version int) {
	t.Helper()
	if _, err := conn.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		v, err := versionOf(name)
		if err != nil {
			t.Fatal(err)
		}
		if v > version {
			continue
		}
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := apply(context.Background(), conn, name, v, string(body), time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}) {
		t.Fatalf("applied through 012 = %v", got)
	}
}

type indexCol struct {
	name string
	desc bool
}

func indexXInfo(t *testing.T, conn *sql.DB, index string) []indexCol {
	t.Helper()
	rows, err := conn.QueryContext(context.Background(), `SELECT name, "desc" FROM pragma_index_xinfo(?) WHERE key = 1 ORDER BY seqno`, index)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []indexCol
	for rows.Next() {
		var c indexCol
		var desc int
		if err := rows.Scan(&c.name, &desc); err != nil {
			t.Fatal(err)
		}
		c.desc = desc != 0
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func columnType(t *testing.T, conn *sql.DB, table, column string) string {
	t.Helper()
	var typ string
	err := conn.QueryRowContext(context.Background(), `SELECT type FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&typ)
	if err != nil {
		t.Fatalf("%s.%s: %v", table, column, err)
	}
	return typ
}

func columnNotNull(t *testing.T, conn *sql.DB, table, column string) bool {
	t.Helper()
	var n int
	if err := conn.QueryRowContext(context.Background(), `SELECT "notnull" FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n != 0
}

// m14Align drops the additive schema of migration 014 and trims the columns
// it appends, so a test of an older migration can still compare the rest.
// start_unit and words_known stay null. users.onboarded_at may be a time:
// the backfill writes it when a qualifying placement or lesson already exists.
func m14Align(t *testing.T, before, after m7State) (m7State, m7State) {
	t.Helper()
	for _, table := range []string{"users", "placement_runs"} {
		brows := before.tables[table]
		arows := after.tables[table]
		width := 0
		if len(brows) > 0 {
			width = len(brows[0])
		}
		for i, row := range arows {
			if width == 0 {
				break
			}
			if len(row) < width {
				t.Fatalf("%s row has %d columns, before has %d", table, len(row), width)
			}
			for j, extra := range row[width:] {
				if extra == "<nil>:<nil>" {
					continue
				}
				if table == "users" && j == 0 && strings.HasPrefix(extra, "string:") && len(extra) > len("string:") {
					continue
				}
				t.Fatalf("%s extra column %q is not null", table, extra)
			}
			arows[i] = row[:width]
		}
		if width > 0 {
			after.tables[table] = arows
		}
		before.schema = m8SchemaWithout(before.schema, table)
		after.schema = m8SchemaWithout(after.schema, table)
	}
	after.schema = m8SchemaWithoutName(after.schema, "placement_runs_user_finished_desc")
	return before, after
}

func dbFile(t *testing.T, conn *sql.DB) string {
	t.Helper()
	var path string
	if err := conn.QueryRowContext(context.Background(), `SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&path); err != nil {
		t.Fatal(err)
	}
	return path
}
