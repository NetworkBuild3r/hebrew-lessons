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
	mustSQL(t, conn, `INSERT INTO placement_runs (user_id, started_at, finished_at, level, stop_reason, result, seed, sitting_no)
		VALUES (?, '2026-10-02T08:00:00.000000000Z', '2026-10-02T08:10:00.000000000Z', 1, 'exhausted', '{"bands":[],"estimated_known_words":4}', 7, 1)`, userID)
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
	if email != "old@example.com" || role != "learner" || created != "2026-10-01T00:00:00.000000000Z" || onboarded.Valid {
		t.Fatalf("old user email=%s role=%s created=%s onboarded=%v", email, role, created, onboarded)
	}

	var started, finished string
	var level int
	var startUnit, wordsKnown sql.NullInt64
	if err := conn.QueryRowContext(context.Background(), `
		SELECT started_at, finished_at, level, start_unit, words_known
		FROM placement_runs WHERE finished_at IS NOT NULL`).Scan(&started, &finished, &level, &startUnit, &wordsKnown); err != nil {
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
	if err != nil || u.OnboardedAt != nil || u.Email != "old@example.com" {
		t.Fatalf("loaded user %+v %v", u, err)
	}
	runs, err := store.PlacementHistory(context.Background(), u.ID, 0)
	if err != nil || len(runs) != 1 || runs[0].StartUnit != 0 || runs[0].WordsKnown != 0 || runs[0].StartUnitTitle != "" || runs[0].FinishedAt.IsZero() {
		t.Fatalf("history %+v %v", runs, err)
	}
	if n, err := store.PlacementHistoryCount(context.Background(), u.ID); err != nil || n != 1 {
		t.Fatalf("count %d %v", n, err)
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

// m14Align drops the additive schema of migration 014 and trims the null columns
// it appends, so a test of an older migration can still compare the rest.
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
			for _, extra := range row[width:] {
				if extra != "<nil>:<nil>" {
					t.Fatalf("%s extra column %q is not null", table, extra)
				}
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
