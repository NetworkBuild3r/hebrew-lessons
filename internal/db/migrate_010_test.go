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

// Tests for migration 010 (INIT-015/SPEC-014): job_state is new, and regression_events is
// rebuilt so a band_slip row can leave out its word. They run on temporary databases only.

const (
	m9Select = `SELECT id, user_id, lexeme_id, skill, from_state, to_state, day, cause FROM regression_events ORDER BY id`
	m9Ins    = `INSERT INTO regression_events (user_id, lexeme_id, skill, from_state, to_state, day, cause) VALUES (?, ?, ?, ?, ?, ?, ?)`
	m9Ins2   = `INSERT INTO regression_events (user_id, kind, lexeme_id, skill, from_state, to_state, day, cause) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
)

var m9Versions = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

// m9At008 builds a database at version 009 with data in every older table, then adds
// regression_events rows of a few shapes and a gap in the ids.
func m9At008(t *testing.T) *sql.DB {
	t.Helper()
	conn := m8At007(t)
	body, err := fs.ReadFile(migrations.FS, "009_confusion_other.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(context.Background(), conn, "009_confusion_other.sql", 9, string(body), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("apply 009: %v", err)
	}
	mustSQL(t, conn, m9Ins, 1, 1, "read", "known", "struggling", "2026-10-01", "decoding")
	mustSQL(t, conn, m9Ins, 1, 1, "listen", "solid", "regressed", "2026-10-02", "unexplained")
	mustSQL(t, conn, m9Ins, 2, 2, "spell", "known", "struggling", "2026-10-03", "spelling")
	mustSQL(t, conn, m9Ins, 3, 4, "recall", "unseen", "seen", "2026-10-04", "first answer")
	mustSQL(t, conn, m9Ins, 1, 2, "read", "learning", "known", "2026-10-05", "learned")
	// Leave a gap in the ids, as a deleted row would.
	mustSQL(t, conn, `DELETE FROM regression_events WHERE id = 2`)
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Fatalf("seed database versions = %v, want 1..9", got)
	}
	return conn
}

// Every row and id survives, as a word row, and the indexes and keys are back.
func TestMigrate009KeepsRowsIdsAndIndexes(t *testing.T) {
	conn := m9At008(t)
	before := m6Dump(t, conn, m9Select)
	if len(before) != 4 {
		t.Fatalf("seed has %d event rows, want 4", len(before))
	}
	others := m7Capture(t, conn)

	if err := m6Migrate(conn); err != nil {
		t.Fatalf("migrate 010: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, m9Versions) {
		t.Fatalf("versions = %v, want 1..12", got)
	}
	if got := m6Dump(t, conn, m9Select); !reflect.DeepEqual(got, before) {
		t.Fatalf("rows changed\n got: %v\nwant: %v", got, before)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM regression_events WHERE kind = 'word'`); n != 4 {
		t.Fatalf("%d rows have kind word, want 4", n)
	}
	if got := indexColumns(t, conn, "regression_events_user_day"); !reflect.DeepEqual(got, []string{"user_id", "day"}) {
		t.Fatalf("index regression_events_user_day columns = %v", got)
	}
	if got := indexColumns(t, conn, "regression_events_user_lexeme_skill"); !reflect.DeepEqual(got, []string{"user_id", "lexeme_id", "skill"}) {
		t.Fatalf("index regression_events_user_lexeme_skill columns = %v", got)
	}
	for _, idx := range []string{"regression_events_user_day", "regression_events_user_lexeme_skill"} {
		if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ? AND tbl_name = 'regression_events'`, idx); n != 1 {
			t.Fatalf("index %s is not on regression_events", idx)
		}
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'regression_events_new%'`); n != 0 {
		t.Fatal("regression_events_new was left behind")
	}
	cols := m6Dump(t, conn, `SELECT name FROM pragma_table_info('regression_events') ORDER BY cid`)
	want := [][]string{{"string:id"}, {"string:user_id"}, {"string:lexeme_id"}, {"string:skill"}, {"string:from_state"}, {"string:to_state"}, {"string:day"}, {"string:cause"}, {"string:kind"}}
	if !reflect.DeepEqual(cols, want) {
		t.Fatalf("columns = %v", cols)
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}
	if got := m6One(t, conn, `PRAGMA foreign_keys`); got != 1 {
		t.Fatalf("foreign_keys = %d, want 1", got)
	}
	// The new rows get ids after the old ones.
	mustSQL(t, conn, m9Ins, 1, 1, "read", "known", "struggling", "2026-10-06", "decoding")
	if got := m6One(t, conn, `SELECT MAX(id) FROM regression_events`); got != 6 {
		t.Fatalf("next id = %d, want 6", got)
	}

	// Every other table that existed before, with its rows and indexes, is exactly as it was.
	// m7Capture leaves out the 008 tables, and the new job_state table is not in the old capture.
	after := m7Capture(t, conn)
	delete(after.tables, "job_state")
	delete(after.rowCnts, "job_state")
	// 011 adds placement_v3_runs and placement_items, which no earlier migration may touch.
	for _, name := range []string{"placement_v3_runs", "placement_items"} {
		delete(after.tables, name)
		delete(after.rowCnts, name)
		after.schema = m8SchemaWithout(after.schema, name)
	}
	after = m16Aside(m15Aside(m13Aside(after)))
	others = m16Aside(m15Aside(m13Aside(others)))
	if !reflect.DeepEqual(after.tables, others.tables) {
		t.Fatal("rows of another table changed")
	}
	if !reflect.DeepEqual(after.rowCnts, others.rowCnts) {
		t.Fatalf("row counts changed\n got: %v\nwant: %v", after.rowCnts, others.rowCnts)
	}
	gotSchema := m8SchemaWithoutName(m8SchemaWithout(m8SchemaWithout(after.schema, "regression_events"), "job_state"), "attempts_user_lexeme_skill_id")
	wantSchema := m8SchemaWithout(others.schema, "regression_events")
	if !reflect.DeepEqual(gotSchema, wantSchema) {
		t.Fatal("schema of another object changed")
	}
}

// The rebuilt table takes a band_slip row with no word, and still refuses what 008 refused.
func TestMigrate009KindsNullsAndChecks(t *testing.T) {
	conn := m9At008(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	const day = "2026-10-19"
	skills := []string{"listen", "read", "recall", "spell"}
	states := []string{"unseen", "seen", "learning", "struggling", "known", "solid", "regressed"}

	// Word rows work as in 008, and kind defaults to word.
	for _, skill := range skills {
		mustSQL(t, conn, m9Ins, 1, 1, skill, "solid", "regressed", day, "decay")
	}
	for _, to := range states[1:] {
		mustSQL(t, conn, m9Ins, 1, 2, "read", states[0], to, day, "first answer")
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM regression_events WHERE kind <> 'word' AND day = ?`, day); n != 0 {
		t.Fatalf("%d rows without the default kind", n)
	}
	// A band_slip row names no word, skill, or state.
	mustSQL(t, conn, m9Ins2, 1, "band_slip", nil, nil, nil, nil, day, "band_slip band=2 known share 100% to 80% since 2026-10-12")
	mustSQL(t, conn, m9Ins2, 1, "band_slip", nil, nil, nil, nil, day, "band_slip band=3 known share 50% to 30% since 2026-10-12")

	for _, bad := range []string{"write", "", "he_en"} {
		rejectSQL(t, conn, "skill "+bad, m9Ins, 1, 1, bad, "known", "regressed", day, "decay")
	}
	rejectSQL(t, conn, "from_state", m9Ins, 1, 1, "read", "mastered", "regressed", day, "decay")
	rejectSQL(t, conn, "to_state", m9Ins, 1, 1, "read", "known", "forgotten", day, "decay")
	// The check that from and to differ still holds.
	rejectSQL(t, conn, "from_state = to_state", m9Ins, 1, 1, "read", "known", "known", day, "decay")
	rejectSQL(t, conn, "band_slip with equal states", m9Ins2, 1, "band_slip", nil, nil, "known", "known", day, "x")
	rejectSQL(t, conn, "day", m9Ins, 1, 1, "read", "known", "regressed", "yesterday", "decay")
	rejectSQL(t, conn, "day not a date", m9Ins, 1, 1, "read", "known", "regressed", "2026-02-30", "decay")
	rejectSQL(t, conn, "empty cause", m9Ins, 1, 1, "read", "known", "regressed", day, "")
	rejectSQL(t, conn, "null cause", m9Ins, 1, 1, "read", "known", "regressed", day, nil)
	rejectSQL(t, conn, "unknown kind", m9Ins2, 1, "other", 1, "read", "known", "regressed", day, "decay")
	rejectSQL(t, conn, "null kind", m9Ins2, 1, nil, 1, "read", "known", "regressed", day, "decay")

	// A word row must still name its word, skill, and both states.
	rejectSQL(t, conn, "word without lexeme", m9Ins, 1, nil, "read", "known", "regressed", day, "decay")
	rejectSQL(t, conn, "word without skill", m9Ins, 1, 1, nil, "known", "regressed", day, "decay")
	rejectSQL(t, conn, "word without from_state", m9Ins, 1, 1, "read", nil, "regressed", day, "decay")
	rejectSQL(t, conn, "word without to_state", m9Ins, 1, 1, "read", "known", nil, day, "decay")
	// A band_slip row must not name a word, a skill, or a state.
	rejectSQL(t, conn, "band_slip with lexeme", m9Ins2, 1, "band_slip", 1, nil, nil, nil, day, "x")
	rejectSQL(t, conn, "band_slip with skill", m9Ins2, 1, "band_slip", nil, "read", nil, nil, day, "x")
	rejectSQL(t, conn, "band_slip with from_state", m9Ins2, 1, "band_slip", nil, nil, "known", nil, day, "x")
	rejectSQL(t, conn, "band_slip with to_state", m9Ins2, 1, "band_slip", nil, nil, nil, "regressed", day, "x")

	// Keys still hold.
	rejectSQL(t, conn, "missing learner", m9Ins, 99, 1, "read", "known", "regressed", day, "decay")
	rejectSQL(t, conn, "missing lexeme", m9Ins, 1, 99999, "read", "known", "regressed", day, "decay")
	rejectSQL(t, conn, "band_slip missing learner", m9Ins2, 99, "band_slip", nil, nil, nil, nil, day, "x")

	// A word with events cannot be deleted, as before. This word has no other references.
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc) VALUES ('fresh')`)
	fresh := m6One(t, conn, `SELECT id FROM lexemes WHERE text_nfc = 'fresh'`)
	mustSQL(t, conn, m9Ins, 3, fresh, "read", "known", "struggling", day, "decay")
	if _, err := conn.ExecContext(context.Background(), `DELETE FROM lexemes WHERE id = ?`, fresh); err == nil {
		t.Error("lexeme with an event was deleted")
	}
	// Deleting the learner deletes their events, band_slip rows included.
	mustSQL(t, conn, `DELETE FROM users WHERE id = 1`)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM regression_events WHERE user_id = 1`); n != 0 {
		t.Fatalf("%d events of a deleted learner remain", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM regression_events WHERE user_id = 3`); n != 2 {
		t.Fatalf("another learner's events = %d, want 2", n)
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}
}

// job_state is a key-value table with a text key, a text value, and a time.
func TestMigrate009JobState(t *testing.T) {
	conn := m7Fresh(t)
	const ins = `INSERT INTO job_state (key, value, updated_at) VALUES (?, ?, ?)`
	mustSQL(t, conn, ins, "daily.lock", "host-1", m6Seeded)
	mustSQL(t, conn, ins, "daily.words.1", `{"day":"2026-10-19","w":{}}`, m6Seeded)
	rejectSQL(t, conn, "duplicate key", ins, "daily.lock", "host-2", m6Seeded)
	rejectSQL(t, conn, "empty key", ins, "", "v", m6Seeded)
	rejectSQL(t, conn, "null key", ins, nil, "v", m6Seeded)
	rejectSQL(t, conn, "null value", ins, "k", nil, m6Seeded)
	rejectSQL(t, conn, "blob value", ins, "k", []byte{1, 2}, m6Seeded)
	rejectSQL(t, conn, "null time", ins, "k", "v", nil)
	rejectSQL(t, conn, "long key", ins, strings.Repeat("k", 201), "v", m6Seeded)
	mustSQL(t, conn, ins, strings.Repeat("k", 200), "v", m6Seeded)
	cols := m6Dump(t, conn, `SELECT name FROM pragma_table_info('job_state') ORDER BY cid`)
	if want := [][]string{{"string:key"}, {"string:value"}, {"string:updated_at"}}; !reflect.DeepEqual(cols, want) {
		t.Fatalf("columns = %v", cols)
	}
}

// 010 applies on a new database through the real runner, and a second run does nothing.
func TestMigrate009AppliesOnEmptyDatabase(t *testing.T) {
	conn := openRaw(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, m9Versions) {
		t.Fatalf("versions = %v, want 1..12", got)
	}
	for _, table := range []string{"job_state", "regression_events", "daily_snapshots"} {
		if !tableExists(t, conn, table) {
			t.Errorf("table %s is missing", table)
		}
		if n := m6One(t, conn, `SELECT COUNT(*) FROM "`+table+`"`); n != 0 {
			t.Errorf("table %s has %d rows on a new database", table, n)
		}
	}
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('a@example.com', 'sub-a', 'learner', ?)`, m6Seeded)
	mustSQL(t, conn, m9Ins2, 1, "band_slip", nil, nil, nil, nil, "2026-10-19", "band_slip band=1 known share 90% to 70% since 2026-10-12")
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, m9Versions) {
		t.Fatalf("versions after rerun = %v", got)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM regression_events`); n != 1 {
		t.Fatalf("rerun changed the rows: %d", n)
	}
}

// The rebuild is only safe because nothing points at regression_events. This fails if that changes.
func TestMigrate009NothingReferencesRegressionEvents(t *testing.T) {
	for name, conn := range map[string]*sql.DB{"at 009": m9At008(t), "at 010": m7Fresh(t)} {
		if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE sql LIKE '%REFERENCES regression_events%' OR sql LIKE '%REFERENCES "regression_events"%'`); n != 0 {
			t.Fatalf("%s: %d objects reference regression_events, so dropping it could cascade", name, n)
		}
		if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE type IN ('view', 'trigger') AND sql LIKE '%regression_events%'`); n != 0 {
			t.Fatalf("%s: %d views or triggers use regression_events", name, n)
		}
		// Child tables found, by foreign key list, across every table.
		var children []string
		for _, row := range m6Dump(t, conn, `SELECT m.name FROM sqlite_master AS m, pragma_foreign_key_list(m.name) AS f WHERE m.type = 'table' AND f."table" = 'regression_events'`) {
			children = append(children, row[0])
		}
		if len(children) != 0 {
			t.Fatalf("%s: tables with a foreign key to regression_events: %v", name, children)
		}
		if n := m6One(t, conn, `SELECT COUNT(*) FROM pragma_foreign_key_list('regression_events')`); n != 2 {
			t.Fatalf("%s: regression_events has %d foreign keys, want 2 (users and lexemes)", name, n)
		}
	}
}

// A failure after the old table was dropped still leaves the database at 009. A view on
// regression_events makes the rename fail, which is after the DROP TABLE.
func TestMigrate009AtomicWhenFailureComesAfterTheDrop(t *testing.T) {
	conn := m9At008(t)
	mustSQL(t, conn, `CREATE VIEW m9_events AS SELECT id, cause FROM regression_events`)
	wantRows := m6Dump(t, conn, m9Select)
	const schemaQ = `SELECT type, name, tbl_name, sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' AND name <> 'schema_migrations' ORDER BY name`
	wantSchema := m6Dump(t, conn, schemaQ)

	err := m6Migrate(conn)
	if err == nil {
		t.Fatal("migration 010 succeeded with a view on regression_events")
	}
	if !strings.Contains(err.Error(), "010_daily_job.sql") {
		t.Fatalf("error = %v, want it to name 010_daily_job.sql", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Fatalf("versions = %v, want 1..9", got)
	}
	if got := m6Dump(t, conn, m9Select); !reflect.DeepEqual(got, wantRows) {
		t.Fatalf("rows changed\n got: %v\nwant: %v", got, wantRows)
	}
	if got := m6Dump(t, conn, schemaQ); !reflect.DeepEqual(got, wantSchema) {
		t.Fatal("schema changed after the rollback (job_state or regression_events_new survived)")
	}
	rejectSQL(t, conn, "a row with no word before 010", m9Ins, 1, nil, nil, nil, nil, "2026-10-19", "x")

	mustSQL(t, conn, `DROP VIEW m9_events`)
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, m9Versions) {
		t.Fatalf("versions after retry = %v", got)
	}
	if got := m6Dump(t, conn, m9Select); !reflect.DeepEqual(got, wantRows) {
		t.Fatalf("rows after retry\n got: %v\nwant: %v", got, wantRows)
	}
}
