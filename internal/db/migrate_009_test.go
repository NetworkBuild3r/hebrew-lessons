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

// Tests for migration 009 (INIT-015/SPEC-013): confusion_misses gains the other kind.
// They run on temporary databases only.

const m8Select = `SELECT id, learner_id, kind, key, at, lexeme_id FROM confusion_misses ORDER BY id`

// m8At007 builds a database at version 008 with data in every older table, then adds
// confusion rows: both old kinds, rows with and without a lexeme, and a gap in the ids.
func m8At007(t *testing.T) *sql.DB {
	t.Helper()
	conn := m7At006(t)
	body, err := fs.ReadFile(migrations.FS, "008_learner_model.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(context.Background(), conn, "008_learner_model.sql", 8, string(body), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("apply 008: %v", err)
	}
	const ins = `INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at) VALUES (?, ?, ?, ?, ?)`
	mustSQL(t, conn, ins, 1, "letter_pair", "ב/כ", 1, "2026-10-01T08:00:00.000000000Z")
	mustSQL(t, conn, ins, 1, "letter_pair", "ב/כ", nil, "2026-10-02T08:00:00.000000000Z")
	mustSQL(t, conn, ins, 2, "agreement", "הוא/היא", 2, "2026-10-03T08:00:00.000000000Z")
	mustSQL(t, conn, ins, 3, "letter_pair", "ס/ש", 4, "2026-10-04T08:00:00Z")
	mustSQL(t, conn, ins, 1, "agreement", "הם/הן", nil, "2026-10-05T08:00:00.000000000Z")
	// Leave a gap in the ids, as a deleted row would.
	mustSQL(t, conn, `DELETE FROM confusion_misses WHERE id = 2`)
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatalf("seed database versions = %v, want 1..8", got)
	}
	return conn
}

// ac (carry-over): a seeded database keeps every row and id, and the table, index, and keys still work.
func TestMigrate008KeepsRowsIdsAndIndex(t *testing.T) {
	conn := m8At007(t)
	before := m6Dump(t, conn, m8Select)
	if len(before) != 4 {
		t.Fatalf("seed has %d confusion rows, want 4", len(before))
	}
	if before[len(before)-1][0] == "int64:4" {
		t.Fatalf("seed ids have no gap: %v", before)
	}
	others := m7Capture(t, conn)

	if err := m6Migrate(conn); err != nil {
		t.Fatalf("migrate 009: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}) {
		t.Fatalf("versions = %v, want 1..11", got)
	}
	if got := m6Dump(t, conn, m8Select); !reflect.DeepEqual(got, before) {
		t.Fatalf("rows changed\n got: %v\nwant: %v", got, before)
	}
	if got := indexColumns(t, conn, "confusion_misses_learner_key_at"); !reflect.DeepEqual(got, []string{"learner_id", "key", "at"}) {
		t.Fatalf("index columns = %v", got)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'confusion_misses_learner_key_at' AND tbl_name = 'confusion_misses'`); n != 1 {
		t.Fatal("index confusion_misses_learner_key_at is not on confusion_misses")
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'confusion_misses_new%'`); n != 0 {
		t.Fatal("confusion_misses_new was left behind")
	}
	cols := m6Dump(t, conn, `SELECT name FROM pragma_table_info('confusion_misses') ORDER BY cid`)
	want := [][]string{{"string:id"}, {"string:learner_id"}, {"string:kind"}, {"string:key"}, {"string:at"}, {"string:lexeme_id"}}
	if !reflect.DeepEqual(cols, want) {
		t.Fatalf("columns = %v", cols)
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}
	if got := m6One(t, conn, `PRAGMA foreign_keys`); got != 1 {
		t.Fatalf("foreign_keys = %d, want 1", got)
	}

	// Every other table, row, and index is exactly as it was.
	after := m7Capture(t, conn)
	delete(others.tables, "confusion_misses")
	delete(others.rowCnts, "confusion_misses")
	delete(after.tables, "confusion_misses")
	delete(after.rowCnts, "confusion_misses")
	// 010 adds job_state and rebuilds regression_events, so they are not part of what 009 must leave alone.
	delete(after.tables, "job_state")
	delete(after.rowCnts, "job_state")
	// 011 adds placement_v3_runs and placement_items, which no earlier migration may touch.
	for _, name := range []string{"placement_v3_runs", "placement_items"} {
		delete(after.tables, name)
		delete(after.rowCnts, name)
		after.schema = m8SchemaWithout(after.schema, name)
	}
	if !reflect.DeepEqual(after.tables, others.tables) {
		t.Fatal("rows of another table changed")
	}
	if !reflect.DeepEqual(after.rowCnts, others.rowCnts) {
		t.Fatalf("row counts changed\n got: %v\nwant: %v", after.rowCnts, others.rowCnts)
	}
	if !reflect.DeepEqual(m8SchemaWithout(m8SchemaWithout(m8SchemaWithout(after.schema, "confusion_misses"), "job_state"), "regression_events"), m8SchemaWithout(m8SchemaWithout(others.schema, "confusion_misses"), "regression_events")) {
		t.Fatal("schema of another object changed")
	}
}

// m8SchemaWithout drops the sqlite_master rows that belong to one table.
func m8SchemaWithout(schema [][]string, table string) [][]string {
	var out [][]string
	for _, row := range schema {
		if strings.TrimPrefix(row[2], "string:") != table {
			out = append(out, row)
		}
	}
	return out
}

// The rebuilt table takes the new kind, still refuses any other, and keeps its keys.
func TestMigrate008KindsAndKeys(t *testing.T) {
	conn := m8At007(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	const ins = `INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at) VALUES (?, ?, ?, ?, ?)`
	for _, kind := range []string{"letter_pair", "agreement", "other"} {
		mustSQL(t, conn, ins, 1, kind, "k", 1, m6Seeded)
	}
	rejectSQL(t, conn, "unknown kind", ins, 1, "root", "k", 1, m6Seeded)
	rejectSQL(t, conn, "empty kind", ins, 1, "", "k", 1, m6Seeded)
	rejectSQL(t, conn, "null key", ins, 1, "other", nil, 1, m6Seeded)
	rejectSQL(t, conn, "null time", ins, 1, "other", "k", 1, nil)
	rejectSQL(t, conn, "missing learner", ins, 99, "other", "k", 1, m6Seeded)
	rejectSQL(t, conn, "missing lexeme", ins, 1, "other", "k", 99999, m6Seeded)
	mustSQL(t, conn, ins, 1, "other", "k", nil, m6Seeded)

	// A word with confusion rows cannot be deleted, as before. This word has no other references.
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc) VALUES ('חָדָשׁ')`)
	fresh := m6One(t, conn, `SELECT id FROM lexemes WHERE text_nfc = 'חָדָשׁ'`)
	mustSQL(t, conn, ins, 3, "other", "k", fresh, m6Seeded)
	if _, err := conn.ExecContext(context.Background(), `DELETE FROM lexemes WHERE id = ?`, fresh); err == nil {
		t.Error("lexeme with a confusion row was deleted")
	}
	// Deleting the learner deletes their rows.
	mustSQL(t, conn, `DELETE FROM users WHERE id = 1`)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM confusion_misses WHERE learner_id = 1`); n != 0 {
		t.Fatalf("%d rows of a deleted learner remain", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM confusion_misses WHERE learner_id = 3`); n != 2 {
		t.Fatalf("another learner's rows = %d, want 2", n)
	}
}

// 009 applies on a new database through the real runner, and a second run does nothing.
func TestMigrate008AppliesOnEmptyDatabase(t *testing.T) {
	conn := openRaw(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}) {
		t.Fatalf("versions = %v", got)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM confusion_misses`); n != 0 {
		t.Fatalf("%d rows on a new database", n)
	}
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('a@example.com', 'sub-a', 'learner', ?)`, m6Seeded)
	mustSQL(t, conn, `INSERT INTO confusion_misses (learner_id, kind, key, at) VALUES (1, 'other', 'א/ב', ?)`, m6Seeded)
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}) {
		t.Fatalf("versions after rerun = %v", got)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM confusion_misses`); n != 1 {
		t.Fatalf("rerun changed the rows: %d", n)
	}
}

// The rebuild is only safe because nothing points at confusion_misses. This fails if that changes.
func TestMigrate008NothingReferencesConfusionMisses(t *testing.T) {
	conn := m8At007(t)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE sql LIKE '%REFERENCES confusion_misses%' OR sql LIKE '%REFERENCES "confusion_misses"%'`); n != 0 {
		t.Fatalf("%d objects reference confusion_misses, so dropping it could cascade", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE type IN ('view', 'trigger') AND sql LIKE '%confusion_misses%'`); n != 0 {
		t.Fatalf("%d views or triggers use confusion_misses", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM pragma_foreign_key_list('confusion_misses')`); n != 2 {
		t.Fatalf("confusion_misses has %d foreign keys, want 2 (users and lexemes)", n)
	}
}

// m8AssertAt007 checks the database is exactly where it was before 009 ran.
func m8AssertAt007(t *testing.T, conn *sql.DB, wantRows [][]string, wantSchema [][]string) {
	t.Helper()
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatalf("versions = %v, want 1..8", got)
	}
	if got := m6Dump(t, conn, m8Select); !reflect.DeepEqual(got, wantRows) {
		t.Fatalf("confusion rows changed\n got: %v\nwant: %v", got, wantRows)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'confusion_misses_new%'`); n != 0 {
		t.Fatal("confusion_misses_new survived the rollback")
	}
	got := m6Dump(t, conn, `SELECT type, name, tbl_name, sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' AND name <> 'schema_migrations' AND name NOT LIKE 'm8_%' ORDER BY name`)
	if !reflect.DeepEqual(got, wantSchema) {
		t.Fatalf("schema changed\n got: %v\nwant: %v", got, wantSchema)
	}
	// The old CHECK is still in force, so the table is the old one.
	rejectSQL(t, conn, "other before 009", `INSERT INTO confusion_misses (learner_id, kind, key, at) VALUES (1, 'other', 'k', ?)`, m6Seeded)
	if got := m6One(t, conn, `PRAGMA foreign_keys`); got != 1 {
		t.Fatalf("foreign_keys = %d, want 1", got)
	}
}

// A failure in the copy step, after the new table exists, leaves the database at 008.
func TestMigrate008AtomicWhenCopyFails(t *testing.T) {
	conn := m8At007(t)
	// An orphan lexeme id can only be stored with foreign keys off. The copy then refuses it.
	mustSQL(t, conn, `PRAGMA foreign_keys = OFF`)
	mustSQL(t, conn, `INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at) VALUES (1, 'letter_pair', 'ד/ר', 424242, ?)`, m6Seeded)
	mustSQL(t, conn, `PRAGMA foreign_keys = ON`)
	wantRows := m6Dump(t, conn, m8Select)
	wantSchema := m6Dump(t, conn, `SELECT type, name, tbl_name, sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' AND name <> 'schema_migrations' AND name NOT LIKE 'm8_%' ORDER BY name`)

	err := m6Migrate(conn)
	if err == nil {
		t.Fatal("migration 009 succeeded with an orphan confusion row")
	}
	if !strings.Contains(err.Error(), "009_confusion_other.sql") || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Fatalf("error = %v, want a foreign key failure from 009_confusion_other.sql", err)
	}
	m8AssertAt007(t, conn, wantRows, wantSchema)

	// The connection is usable and the migration is retryable once the data is fixed.
	mustSQL(t, conn, `DELETE FROM confusion_misses WHERE lexeme_id = 424242`)
	wantRows = m6Dump(t, conn, m8Select)
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("retry after fixing the data: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}) {
		t.Fatalf("versions after retry = %v", got)
	}
	if got := m6Dump(t, conn, m8Select); !reflect.DeepEqual(got, wantRows) {
		t.Fatalf("rows after retry\n got: %v\nwant: %v", got, wantRows)
	}
}

// A failure after the old table was dropped still leaves the database at 008. A view on
// confusion_misses makes the rename fail, which is after the DROP TABLE.
func TestMigrate008AtomicWhenFailureComesAfterTheDrop(t *testing.T) {
	conn := m8At007(t)
	mustSQL(t, conn, `CREATE VIEW m8_confusions AS SELECT id, kind FROM confusion_misses`)
	wantRows := m6Dump(t, conn, m8Select)
	wantSchema := m6Dump(t, conn, `SELECT type, name, tbl_name, sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' AND name <> 'schema_migrations' AND name NOT LIKE 'm8_%' ORDER BY name`)

	err := m6Migrate(conn)
	if err == nil {
		t.Fatal("migration 009 succeeded with a view on confusion_misses")
	}
	if !strings.Contains(err.Error(), "009_confusion_other.sql") {
		t.Fatalf("error = %v, want it to name 009_confusion_other.sql", err)
	}
	t.Logf("migration failed as intended: %v", err)
	m8AssertAt007(t, conn, wantRows, wantSchema)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'view' AND name = 'm8_confusions'`); n != 1 {
		t.Fatal("the view is gone")
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'confusion_misses_learner_key_at'`); n != 1 {
		t.Fatal("the index was lost in the rollback")
	}

	mustSQL(t, conn, `DROP VIEW m8_confusions`)
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}) {
		t.Fatalf("versions after retry = %v", got)
	}
}

// The new kind works through the store: a row written for it is read back with its two words split.
func TestMigrate008OtherKindThroughStore(t *testing.T) {
	s := openAt(t, t.TempDir(), nil)
	userID := insertUser(t, s, "a@example.com", "sub-a", "learner")
	lexemeID := insertLexeme(t, s, "שָׁלוֹם")
	at := time.Date(2026, 10, 19, 9, 0, 0, 0, time.UTC)
	if err := s.InsertConfusionMiss(context.Background(), userID, "other", "כֶּלֶב/שָׁלוֹם", lexemeID, at); err != nil {
		t.Fatal(err)
	}
	pairs, err := s.RecentConfusionPairs(context.Background(), userID, at.Add(-time.Hour))
	if err != nil || len(pairs) != 1 {
		t.Fatalf("pairs = %+v, %v", pairs, err)
	}
	if pairs[0].Kind != "other" || pairs[0].A != "כֶּלֶב" || pairs[0].B != "שָׁלוֹם" || pairs[0].Count != 1 {
		t.Fatalf("pair = %+v", pairs[0])
	}
}
