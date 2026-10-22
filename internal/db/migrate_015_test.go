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

// Migration 015 adds lesson_listen to the exposures.context check by rebuilding the table.
// SQLite cannot change a CHECK in place. These tests run on temporary databases only.

const (
	m14Version = 15
	m14File    = "015_exposures_lesson_listen.sql"
	m14Rows    = `SELECT rowid, id, user_id, lexeme_id, context, unit_id, created_at FROM exposures ORDER BY rowid, id`
	m14Insert  = `INSERT INTO exposures (user_id, lexeme_id, context, unit_id, created_at) VALUES (?, ?, ?, ?, ?)`
)

// m14At012 migrates through version 012 and seeds exposures. It does not apply 015.
// The seed covers every context the old CHECK allows, a NULL unit_id (the only nullable
// column), an omitted id (the rowid default; the table declares no column DEFAULT), and
// an explicit id that leaves a gap.
func m14At012(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	conn := openRaw(t)
	if _, err := conn.ExecContext(ctx, `
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
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		version, err := versionOf(name)
		if err != nil {
			t.Fatal(err)
		}
		if version >= m14Version {
			continue
		}
		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		if err := apply(ctx, conn, name, version, string(body), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}) {
		t.Fatalf("seed database versions = %v, want 1..12", got)
	}
	m14Seed(t, conn)
	return conn
}

func m14Seed(t *testing.T, conn *sql.DB) {
	t.Helper()
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('a@example.com', 'sub-a', 'learner', ?)`, m6Seeded)
	mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES ('b@example.com', 'sub-b', 'learner', ?)`, m6Seeded)
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc) VALUES ('שָׁלוֹם')`)
	mustSQL(t, conn, `INSERT INTO lexemes (text_nfc) VALUES ('תּוֹדָה')`)
	mustSQL(t, conn, `INSERT INTO units (position, title, situation, concept, status, rev, doc) VALUES (0, 't', 's', 'c', 'published', 1, '{}')`)

	// Omitted id and omitted unit_id: the rowid default, and NULL for the only nullable column.
	mustSQL(t, conn, `INSERT INTO exposures (user_id, lexeme_id, context, created_at) VALUES (1, 1, 'story', '2026-01-01T00:00:00Z')`)
	mustSQL(t, conn, m14Insert, 1, 2, "dialogue", nil, "2026-02-02T03:04:05.000000000Z")
	mustSQL(t, conn, m14Insert, 2, 1, "gloss_tap", 1, "2026-03-03T00:00:00Z")
	mustSQL(t, conn, m14Insert, 2, 2, "placement_review", nil, "2026-04-04T00:00:00Z")
	mustSQL(t, conn, m14Insert, 1, 1, "lesson_intro", 1, "2026-05-05T00:00:00Z")
	// An explicit id leaves a gap, the way a deleted row would.
	mustSQL(t, conn, `INSERT INTO exposures (id, user_id, lexeme_id, context, unit_id, created_at) VALUES (40, 2, 2, 'story', NULL, '2026-06-06T00:00:00Z')`)
	mustSQL(t, conn, `INSERT INTO exposures (user_id, lexeme_id, context, created_at) VALUES (1, 2, 'lesson_intro', '2026-07-07T00:00:00.123456789Z')`)
}

// m14Schema is the part of the exposures schema that 015 must not change, plus the CREATE text.
type m14Schema struct {
	tableInfo  [][]string
	indexList  [][]string
	indexXInfo [][]string
	fks        [][]string
	indexSQL   [][]string
	triggers   [][]string
	views      [][]string
	inbound    [][]string
	createSQL  string
}

func m14ReadSchema(t *testing.T, conn *sql.DB) m14Schema {
	t.Helper()
	s := m14Schema{
		tableInfo: m6Dump(t, conn, `SELECT cid, name, type, "notnull", dflt_value, pk FROM pragma_table_info('exposures') ORDER BY cid`),
		indexList: m6Dump(t, conn, `SELECT seq, name, "unique", origin, partial FROM pragma_index_list('exposures') ORDER BY seq, name`),
		fks:       m6Dump(t, conn, `SELECT id, seq, "table", "from", "to", on_update, on_delete, "match" FROM pragma_foreign_key_list('exposures') ORDER BY id, seq`),
		indexSQL:  m6Dump(t, conn, `SELECT name, sql FROM sqlite_master WHERE type = 'index' AND tbl_name = 'exposures' ORDER BY name`),
		triggers:  m6Dump(t, conn, `SELECT name, sql FROM sqlite_master WHERE type = 'trigger' AND tbl_name = 'exposures' ORDER BY name`),
		views:     m6Dump(t, conn, `SELECT name, sql FROM sqlite_master WHERE type = 'view' AND instr(sql, 'exposures') > 0 ORDER BY name`),
		inbound: m6Dump(t, conn, `
			SELECT m.name, f."from", f."to", f.on_update, f.on_delete
			FROM sqlite_master AS m, pragma_foreign_key_list(m.name) AS f
			WHERE m.type = 'table' AND f."table" = 'exposures'
			ORDER BY m.name, f.id, f.seq`),
		createSQL: m14Text(t, conn, `SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'exposures'`),
	}
	for _, idx := range s.indexList {
		name := strings.TrimPrefix(idx[1], "string:")
		rows := m6Dump(t, conn, `SELECT seqno, cid, name, desc, coll, key FROM pragma_index_xinfo(?) ORDER BY seqno`, name)
		for _, row := range rows {
			s.indexXInfo = append(s.indexXInfo, append([]string{"index:" + name}, row...))
		}
	}
	return s
}

func m14Text(t *testing.T, conn *sql.DB, query string, args ...any) string {
	t.Helper()
	var s string
	if err := conn.QueryRowContext(context.Background(), query, args...).Scan(&s); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return s
}

// m14WithoutListen removes the one new token from a CREATE TABLE statement.
func m14WithoutListen(createSQL string) (string, bool) {
	const token = ", 'lesson_listen'"
	if strings.Count(createSQL, token) != 1 {
		return createSQL, false
	}
	return strings.Replace(createSQL, token, "", 1), true
}

func m14SameExceptCheck(t *testing.T, before, after m14Schema) {
	t.Helper()
	if !reflect.DeepEqual(after.tableInfo, before.tableInfo) {
		t.Fatalf("table_info changed\n got: %v\nwant: %v", after.tableInfo, before.tableInfo)
	}
	if !reflect.DeepEqual(after.indexList, before.indexList) {
		t.Fatalf("index_list changed\n got: %v\nwant: %v", after.indexList, before.indexList)
	}
	if !reflect.DeepEqual(after.indexXInfo, before.indexXInfo) {
		t.Fatalf("index_xinfo changed\n got: %v\nwant: %v", after.indexXInfo, before.indexXInfo)
	}
	if !reflect.DeepEqual(after.fks, before.fks) {
		t.Fatalf("foreign_key_list changed\n got: %v\nwant: %v", after.fks, before.fks)
	}
	if !reflect.DeepEqual(after.indexSQL, before.indexSQL) {
		t.Fatalf("index sql changed\n got: %v\nwant: %v", after.indexSQL, before.indexSQL)
	}
	if !reflect.DeepEqual(after.triggers, before.triggers) {
		t.Fatalf("triggers changed\n got: %v\nwant: %v", after.triggers, before.triggers)
	}
	if !reflect.DeepEqual(after.views, before.views) {
		t.Fatalf("views changed\n got: %v\nwant: %v", after.views, before.views)
	}
	if !reflect.DeepEqual(after.inbound, before.inbound) {
		t.Fatalf("inbound foreign keys changed\n got: %v\nwant: %v", after.inbound, before.inbound)
	}
	// RENAME TO exposures stores the table name in quotes. That is SQLite's rewrite, not a column change.
	got := strings.Replace(after.createSQL, `CREATE TABLE "exposures"`, `CREATE TABLE exposures`, 1)
	stripped, ok := m14WithoutListen(got)
	if !ok || stripped != before.createSQL {
		t.Fatalf("create sql differs by more than the lesson_listen check token\n got: %s\nwant: %s", after.createSQL, before.createSQL)
	}
}

func m14AssertSeed(t *testing.T, rows [][]string) {
	t.Helper()
	if len(rows) != 7 {
		t.Fatalf("seed has %d exposures, want 7", len(rows))
	}
	seen := map[string]bool{}
	var nullUnit, setUnit bool
	var ids []string
	for _, row := range rows {
		seen[strings.TrimPrefix(row[4], "string:")] = true
		ids = append(ids, strings.TrimPrefix(row[1], "int64:"))
		if row[5] == "<nil>:<nil>" {
			nullUnit = true
		} else {
			setUnit = true
		}
	}
	for _, c := range []string{"story", "dialogue", "gloss_tap", "placement_review", "lesson_intro"} {
		if !seen[c] {
			t.Fatalf("seed is missing context %s in %v", c, seen)
		}
	}
	if !nullUnit || !setUnit {
		t.Fatal("seed needs a NULL unit_id and a set unit_id")
	}
	if !reflect.DeepEqual(ids, []string{"1", "2", "3", "4", "5", "40", "41"}) {
		t.Fatalf("seed ids = %v, want a gap at 40", ids)
	}
}

// (a) and the schema half of the rebuild: rows stay byte-identical, and the schema matches
// except the new CHECK token. (d) foreign_key_check is empty.
func TestMigrate015RowsAndSchemaMatchExceptTheCheck(t *testing.T) {
	conn := m14At012(t)
	beforeRows := m6Dump(t, conn, m14Rows)
	m14AssertSeed(t, beforeRows)
	beforeSchema := m14ReadSchema(t, conn)
	if strings.Contains(beforeSchema.createSQL, "lesson_listen") {
		t.Fatal("version 012 already lists lesson_listen")
	}
	if len(beforeSchema.inbound) != 0 || len(beforeSchema.triggers) != 0 || len(beforeSchema.views) != 0 {
		t.Fatalf("exposures is referenced, so a plain drop is not safe: inbound %v triggers %v views %v", beforeSchema.inbound, beforeSchema.triggers, beforeSchema.views)
	}

	if err := m6Migrate(conn); err != nil {
		t.Fatalf("migrate 015: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 15}) {
		t.Fatalf("versions = %v, want 1..12 and 15", got)
	}
	afterRows := m6Dump(t, conn, m14Rows)
	if len(afterRows) != len(beforeRows) {
		t.Fatalf("row count %d -> %d", len(beforeRows), len(afterRows))
	}
	if !reflect.DeepEqual(afterRows, beforeRows) {
		t.Fatalf("rows changed\n got: %v\nwant: %v", afterRows, beforeRows)
	}
	m14SameExceptCheck(t, beforeSchema, m14ReadSchema(t, conn))
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'exposures_new%'`); n != 0 {
		t.Fatal("exposures_new was left behind")
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}
	if got := m6One(t, conn, `PRAGMA foreign_keys`); got != 1 {
		t.Fatalf("foreign_keys = %d, want 1", got)
	}
	if got := m6One(t, conn, `PRAGMA legacy_alter_table`); got != 0 {
		t.Fatalf("legacy_alter_table = %d, want 0 (the migration must not turn it on)", got)
	}
}

// (b) lesson_listen is accepted and a context outside the list is still rejected.
func TestMigrate015LessonListenAcceptedBogusContextRejected(t *testing.T) {
	conn := m14At012(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, conn, m14Insert, 1, 1, "lesson_listen", nil, m6Seeded)
	mustSQL(t, conn, m14Insert, 1, 2, "lesson_listen", 1, m6Seeded)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM exposures WHERE context = 'lesson_listen'`); n != 2 {
		t.Fatalf("lesson_listen rows = %d, want 2", n)
	}
	rejectSQL(t, conn, "bogus context", m14Insert, 1, 1, "not_a_context", nil, m6Seeded)
	rejectSQL(t, conn, "empty context", m14Insert, 1, 1, "", nil, m6Seeded)
}

// (d) foreign_key_check stays empty on a database that went through 015 with rows in exposures.
func TestMigrate015ForeignKeyCheckIsEmpty(t *testing.T) {
	conn := m14At012(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, conn, m14Insert, 1, 1, "lesson_listen", 1, m6Seeded)
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}
}

// m14RollbackSQL is the operator recipe in the migration header. The runner must not execute it.
func m14RollbackSQL(t *testing.T) string {
	t.Helper()
	body, err := fs.ReadFile(migrations.FS, m14File)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	const begin = "-- ROLLBACK RECIPE BEGIN"
	const end = "-- ROLLBACK RECIPE END"
	i := strings.Index(text, begin)
	j := strings.Index(text, end)
	if i < 0 || j < 0 || j < i {
		t.Fatal("migration 015 header has no rollback recipe markers")
	}
	var b strings.Builder
	for _, line := range strings.Split(text[i+len(begin):j], "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || trim == "--" {
			continue
		}
		if !strings.HasPrefix(trim, "--") {
			t.Fatalf("rollback recipe line is not a comment: %q", line)
		}
		sqlLine := strings.TrimSpace(strings.TrimPrefix(trim, "--"))
		if sqlLine == "" {
			continue
		}
		b.WriteString(sqlLine)
		b.WriteByte('\n')
	}
	script := b.String()
	if !strings.Contains(script, "UPDATE exposures SET context = 'lesson_intro' WHERE context = 'lesson_listen'") {
		t.Fatalf("rollback recipe does not remap lesson_listen to lesson_intro:\n%s", script)
	}
	if strings.Contains(strings.ToUpper(script), "DELETE FROM EXPOSURES") {
		t.Fatal("rollback recipe deletes exposures rows")
	}
	for _, stmt := range statements(text) {
		bare := m14BareSQL(stmt)
		if strings.Contains(bare, "exposures_old") || strings.Contains(strings.ToUpper(bare), "UPDATE EXPOSURES") {
			t.Fatalf("the runner would execute rollback SQL:\n%s", bare)
		}
	}
	return script
}

func m14BareSQL(stmt string) string {
	var b strings.Builder
	inLine := false
	for i := 0; i < len(stmt); i++ {
		if inLine {
			if stmt[i] == '\n' {
				inLine = false
				b.WriteByte('\n')
			}
			continue
		}
		if stmt[i] == '-' && i+1 < len(stmt) && stmt[i+1] == '-' {
			inLine = true
			i++
			continue
		}
		b.WriteByte(stmt[i])
	}
	return b.String()
}

func m14ExecScript(t *testing.T, conn *sql.DB, script string) {
	t.Helper()
	ctx := context.Background()
	for _, stmt := range statements(script) {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("rollback statement failed: %v\n%s", err, stmt)
		}
	}
}

// (c) the rollback recipe remaps lesson_listen to lesson_intro without dropping rows,
// rebuilds the old CHECK, then Migrate applies 015 again and the remapped rows stay.
func TestMigrate015RollbackRemapsThenReapplies(t *testing.T) {
	conn := m14At012(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	mustSQL(t, conn, m14Insert, 1, 1, "lesson_listen", nil, "2026-08-08T00:00:00Z")
	mustSQL(t, conn, m14Insert, 2, 2, "lesson_listen", 1, "2026-09-09T00:00:00Z")
	before := m6Dump(t, conn, m14Rows)
	count := len(before)
	var listen int
	for _, row := range before {
		if strings.TrimPrefix(row[4], "string:") == "lesson_listen" {
			listen++
		}
	}
	if listen != 2 {
		t.Fatalf("lesson_listen rows before rollback = %d, want 2", listen)
	}

	m14ExecScript(t, conn, m14RollbackSQL(t))

	afterRollback := m6Dump(t, conn, m14Rows)
	if len(afterRollback) != count {
		t.Fatalf("rollback changed the row count %d -> %d", count, len(afterRollback))
	}
	remapped := 0
	for i, row := range afterRollback {
		gotCtx := strings.TrimPrefix(row[4], "string:")
		wantCtx := strings.TrimPrefix(before[i][4], "string:")
		if wantCtx == "lesson_listen" {
			if gotCtx != "lesson_intro" {
				t.Fatalf("row %d context = %s, want lesson_intro", i, gotCtx)
			}
			remapped++
			wantCtx = "lesson_intro"
		}
		if gotCtx != wantCtx {
			t.Fatalf("row %d context = %s, want %s", i, gotCtx, wantCtx)
		}
		restGot := append(append([]string{}, row[:4]...), row[5:]...)
		restWant := append(append([]string{}, before[i][:4]...), before[i][5:]...)
		if !reflect.DeepEqual(restGot, restWant) {
			t.Fatalf("row %d changed beyond context\n got: %v\nwant: %v", i, row, before[i])
		}
	}
	if remapped != listen {
		t.Fatalf("remapped %d rows, want %d", remapped, listen)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM exposures WHERE context = 'lesson_listen'`); n != 0 {
		t.Fatalf("%d lesson_listen rows remain after rollback", n)
	}
	rejectSQL(t, conn, "lesson_listen after rollback", m14Insert, 1, 1, "lesson_listen", nil, m6Seeded)
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}) {
		t.Fatalf("versions after rollback = %v, want 1..12", got)
	}
	rolled := m6Dump(t, conn, m14Rows)

	if err := m6Migrate(conn); err != nil {
		t.Fatalf("re-apply 015: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 15}) {
		t.Fatalf("versions after re-apply = %v", got)
	}
	if got := m6Dump(t, conn, m14Rows); !reflect.DeepEqual(got, rolled) {
		t.Fatalf("re-apply changed rows\n got: %v\nwant: %v", got, rolled)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM exposures`); n != count {
		t.Fatalf("row count after re-apply = %d, want %d", n, count)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM exposures WHERE context = 'lesson_intro' AND created_at IN ('2026-08-08T00:00:00Z', '2026-09-09T00:00:00Z')`); n != 2 {
		t.Fatalf("remapped rows that still read lesson_intro = %d, want 2", n)
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check after re-apply: %v", fk)
	}
}
