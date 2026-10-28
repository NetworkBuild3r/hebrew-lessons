package db

import (
	"context"
	"database/sql"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

// Tests for migration 011 (INIT-015/SPEC-008): placement_v3_runs and placement_items are new, and
// nothing that exists is changed. They run on temporary databases only. The runner applies the file in one
// transaction, so a database at 010 is the only starting point that matters besides an empty one.

const m10SeedRun = `INSERT INTO placement_runs (user_id, started_at, finished_at, level, stop_reason, result, seed, sitting_no)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

// m10At009 builds a database at version 010 with a finished v2 run, an open v2 run, and answers on both.
func m10At009(t *testing.T) *sql.DB {
	t.Helper()
	conn := m9At008(t)
	body, err := fs.ReadFile(migrations.FS, "010_daily_job.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(context.Background(), conn, "010_daily_job.sql", 10, string(body), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("apply 010: %v", err)
	}
	mustSQL(t, conn, m10SeedRun, 1, "2026-10-01T08:00:00.000000000Z", "2026-10-01T08:10:00.000000000Z", 2, "band_stop",
		`{"bands":[{"band":0,"rated":10,"again":0,"hard":0,"good":10,"easy":0,"dont_know":0,"good_plus_rate":1}],"estimated_known_words":10}`, 41, 1)
	mustSQL(t, conn, m10SeedRun, 2, "2026-10-02T08:00:00.000000000Z", nil, nil, nil, nil, 42, 2)
	mustSQL(t, conn, `INSERT INTO placement_answers (run_id, item_id, rating, shown_at, answered_at, lexeme_id, sitting_no) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		1, "boker", 3, "2026-10-01T08:01:00.000000000Z", "2026-10-01T08:01:05.000000000Z", 1, 1)
	mustSQL(t, conn, `INSERT INTO placement_answers (run_id, item_id, rating, shown_at, answered_at, lexeme_id, sitting_no) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		2, "kafe", nil, "2026-10-02T08:01:00.000000000Z", nil, nil, 1)
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}) {
		t.Fatalf("seed database versions = %v, want 1..10", got)
	}
	return conn
}

// A database already at 010 gets the two tables, and every v2 run and answer is exactly as it was.
func TestMigrate010OnA009DatabaseKeepsV2Placement(t *testing.T) {
	conn := m10At009(t)
	runs := m6Dump(t, conn, `SELECT * FROM placement_runs ORDER BY id`)
	answers := m6Dump(t, conn, `SELECT * FROM placement_answers ORDER BY run_id, item_id`)
	before := m7Capture(t, conn)

	if err := m6Migrate(conn); err != nil {
		t.Fatalf("migrate 011: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}) {
		t.Fatalf("versions = %v, want 1..12", got)
	}
	if got := m6Dump(t, conn, `SELECT * FROM placement_runs ORDER BY id`); !reflect.DeepEqual(got, runs) {
		t.Fatalf("placement_runs changed\n got: %v\nwant: %v", got, runs)
	}
	if got := m6Dump(t, conn, `SELECT * FROM placement_answers ORDER BY run_id, item_id`); !reflect.DeepEqual(got, answers) {
		t.Fatalf("placement_answers changed\n got: %v\nwant: %v", got, answers)
	}
	for _, table := range []string{"placement_v3_runs", "placement_items"} {
		if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table); n != 1 {
			t.Fatalf("table %s is missing", table)
		}
		if n := m6One(t, conn, `SELECT COUNT(*) FROM `+table); n != 0 {
			t.Fatalf("table %s has %d rows after the migration, want 0", table, n)
		}
	}
	// A v2 run has no v3 row, which is how the engine tells the two apart.
	if n := m6One(t, conn, `SELECT COUNT(*) FROM placement_runs AS r JOIN placement_v3_runs AS v ON v.run_id = r.id`); n != 0 {
		t.Fatalf("%d old runs look like v3 runs", n)
	}

	// Every existing table, row, and index is as it was. m7Capture covers all tables but the 008 ones, which 011 does not touch.
	after := m7Capture(t, conn)
	for _, name := range []string{"placement_v3_runs", "placement_items"} {
		delete(after.tables, name)
		delete(after.rowCnts, name)
		after.schema = m8SchemaWithout(after.schema, name)
	}
	after = m19Aside(m17Aside(m16Aside(m15Aside(m13Aside(after)))))
	before = m19Aside(m17Aside(m16Aside(m15Aside(m13Aside(before)))))
	if !reflect.DeepEqual(after.tables, before.tables) {
		t.Fatal("rows of an existing table changed")
	}
	if !reflect.DeepEqual(after.rowCnts, before.rowCnts) {
		t.Fatalf("row counts changed\n got: %v\nwant: %v", after.rowCnts, before.rowCnts)
	}
	if !reflect.DeepEqual(m8SchemaWithoutName(after.schema, "attempts_user_lexeme_skill_id"), before.schema) {
		t.Fatal("the schema of an existing object changed")
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}

	// A new v3 run can be opened beside the old ones: the finished v2 run does not block it, and the open one must be closed first.
	mustSQL(t, conn, `UPDATE placement_runs SET finished_at = ? WHERE id = 2`, "2026-10-03T08:00:00.000000000Z")
	mustSQL(t, conn, m10SeedRun, 1, "2026-10-04T08:00:00.000000000Z", nil, nil, nil, nil, 43, 1)
	mustSQL(t, conn, `INSERT INTO placement_v3_runs (run_id, kind) VALUES (3, 'initial')`)
	if got := m6One(t, conn, `SELECT COUNT(*) FROM placement_v3_runs`); got != 1 {
		t.Fatalf("v3 runs = %d, want 1", got)
	}
}

// An empty database gets the same two tables from the full chain.
func TestMigrate010OnAFreshDatabase(t *testing.T) {
	conn := openRaw(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}) {
		t.Fatalf("versions = %v, want 1..12", got)
	}
	cols := m6Dump(t, conn, `SELECT name FROM pragma_table_info('placement_items') ORDER BY cid`)
	want := [][]string{{"string:id"}, {"string:run_id"}, {"string:seq"}, {"string:part"}, {"string:skill"}, {"string:band"}, {"string:item_id"},
		{"string:options"}, {"string:correct_idx"}, {"string:outcome"}, {"string:picked"}, {"string:lexeme_id"}, {"string:shown_at"},
		{"string:answered_at"}, {"string:latency_ms"}}
	if !reflect.DeepEqual(cols, want) {
		t.Fatalf("placement_items columns = %v", cols)
	}
	// Running the chain again changes nothing.
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if got := m6Applied(t, conn); len(got) != 19 {
		t.Fatalf("versions after a second migrate = %v", got)
	}
}

// The tables refuse what the engine must never write.
func TestMigrate010Constraints(t *testing.T) {
	conn := m10At009(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, conn, `UPDATE placement_runs SET finished_at = '2026-10-03T08:00:00.000000000Z' WHERE id = 2`)
	mustSQL(t, conn, m10SeedRun, 1, "2026-10-04T08:00:00.000000000Z", nil, nil, nil, nil, 43, 1)
	const run = 3
	mustSQL(t, conn, `INSERT INTO placement_v3_runs (run_id, kind) VALUES (?, 'initial')`, run)

	refuse := func(name, query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(query, args...); err == nil {
			t.Errorf("%s: the insert was accepted", name)
		}
	}
	// A run is described once, a recheck names its skill and band, an initial run names neither.
	refuse("second v3 row for a run", `INSERT INTO placement_v3_runs (run_id, kind) VALUES (?, 'initial')`, run)
	refuse("v3 row for no run", `INSERT INTO placement_v3_runs (run_id, kind) VALUES (999, 'initial')`)
	refuse("unknown kind", `INSERT INTO placement_v3_runs (run_id, kind) VALUES (1, 'weekly')`)
	refuse("recheck without a band", `INSERT INTO placement_v3_runs (run_id, kind, recheck_skill) VALUES (1, 'recheck', 'read')`)
	refuse("initial with a band", `INSERT INTO placement_v3_runs (run_id, kind, recheck_skill, recheck_band) VALUES (1, 'initial', 'read', 2)`)
	refuse("recheck of recall", `INSERT INTO placement_v3_runs (run_id, kind, recheck_skill, recheck_band) VALUES (1, 'recheck', 'recall', 2)`)
	refuse("recheck of band 6", `INSERT INTO placement_v3_runs (run_id, kind, recheck_skill, recheck_band) VALUES (1, 'recheck', 'read', 6)`)
	refuse("unknown phase", `UPDATE placement_v3_runs SET phase = 'done' WHERE run_id = 3`)

	const ins = `INSERT INTO placement_items (run_id, seq, part, skill, band, item_id, options, correct_idx, shown_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	const at = "2026-10-04T08:01:00.000000000Z"
	mustSQL(t, conn, ins, run, 1, "listening", "listen", 1, "boker", `{"options":[]}`, 2, at)
	refuse("same word twice in listening", ins, run, 2, "listening", "listen", 1, "boker", `{"options":[]}`, 2, at)
	refuse("same word in reading after listening", ins, run, 2, "reading", "read", 1, "boker", `{"options":[]}`, 2, at)
	refuse("same sequence twice", ins, run, 1, "listening", "listen", 1, "kafe", `{"options":[]}`, 2, at)
	refuse("unknown part", ins, run, 2, "speaking", "listen", 1, "kafe", `{"options":[]}`, 2, at)
	refuse("unknown skill", ins, run, 2, "listening", "spell", 1, "kafe", `{"options":[]}`, 2, at)
	refuse("band 6", ins, run, 2, "listening", "listen", 6, "kafe", `{"options":[]}`, 2, at)
	refuse("options that are not JSON", ins, run, 2, "listening", "listen", 1, "kafe", `not json`, 2, at)
	refuse("item for no run", ins, 999, 1, "listening", "listen", 1, "kafe", `{"options":[]}`, 2, at)
	// The typing part asks a word again on purpose.
	mustSQL(t, conn, ins, run, 2, "typing", "recall", 1, "boker", `{"options":[]}`, nil, at)
	// An item has an outcome exactly when it has an answer time.
	refuse("outcome without a time", `UPDATE placement_items SET outcome = 'right' WHERE run_id = 3 AND seq = 2`)
	refuse("unknown outcome", `UPDATE placement_items SET outcome = 'maybe', answered_at = ? WHERE run_id = 3 AND seq = 2`, at)
	mustSQL(t, conn, `UPDATE placement_items SET outcome = 'dont_know', answered_at = ? WHERE run_id = 3 AND seq = 2`, at)

	// Deleting the run deletes both the v3 row and the items.
	mustSQL(t, conn, `DELETE FROM placement_runs WHERE id = 3`)
	for _, table := range []string{"placement_v3_runs", "placement_items"} {
		if n := m6One(t, conn, `SELECT COUNT(*) FROM `+table); n != 0 {
			t.Errorf("%s keeps %d rows after its run was deleted", table, n)
		}
	}
	if !strings.Contains(m6SQLOf(t, conn, "placement_items_no_repeat"), "WHERE part IN") {
		t.Error("the no-repeat index is not partial")
	}
}

func m6SQLOf(t *testing.T, conn *sql.DB, name string) string {
	t.Helper()
	var s string
	if err := conn.QueryRow(`SELECT sql FROM sqlite_master WHERE name = ?`, name).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}
