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

// m7Tables are the seven tables 008 adds (INIT-015/SPEC-004).
var m7Tables = []string{
	"daily_snapshots", "regression_events", "lesson_briefs", "coach_notes",
	"publish_audit", "unit_reports", "weekly_reports",
}

const (
	m7Day = "2026-10-19"
	m7ID  = "0123456789abcdef0123456789abcdef" // 32 hex characters, a 128-bit id.
)

// m7Fresh builds an empty database at 008 through the real runner.
func m7Fresh(t *testing.T) *sql.DB {
	t.Helper()
	conn := openRaw(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return conn
}

// m7At006 builds a database at version 006 that holds data in the evidence tables as well as
// the older ones: learners, words, cards, review history, attempts, exposures, and a unit.
func m7At006(t *testing.T) *sql.DB {
	t.Helper()
	conn := m6At005(t)
	body, err := fs.ReadFile(migrations.FS, "006_evidence.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(context.Background(), conn, "006_evidence.sql", 6, string(body), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("apply 006: %v", err)
	}
	// Production already has the OAuth marker (007, from the v3 branch) when these migrations arrive, so the seed does too.
	oauth, err := fs.ReadFile(migrations.FS, "007_oauth_client_issued.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(context.Background(), conn, "007_oauth_client_issued.sql", 7, string(oauth), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("apply 007: %v", err)
	}
	mustSQL(t, conn, `INSERT INTO units (position, title, situation, concept, status, rev, doc)
		VALUES (1, 'Hello', 'street', 'greetings', 'published', 1, '{"title":"Hello"}')`)
	mustSQL(t, conn, `INSERT INTO unit_revisions (unit_id, rev, doc, actor, created_at) VALUES (1, 1, '{"title":"Hello"}', 'test', ?)`, m6Seeded)
	mustSQL(t, conn, `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, unit_id, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (1, 1, 'read', 'listen_pick', 'lesson', 1, 'p', 'r', 1, 0, 0, 900, ?)`, m6Seeded)
	mustSQL(t, conn, `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, unit_id, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (2, 4, 'spell', 'cloze', 'practice', NULL, 'p', '', 0, 1, 1, 4000, ?)`, m6Seeded)
	mustSQL(t, conn, `INSERT INTO exposures (user_id, lexeme_id, context, unit_id, created_at) VALUES (1, 2, 'story', 1, ?)`, m6Seeded)
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7}) {
		t.Fatalf("seed database versions = %v, want 1..7", got)
	}
	return conn
}

// m7Snapshot records every user table (except schema_migrations) as its SQL and its rows, plus
// every index definition, so a before and an after compare with DeepEqual.
type m7State struct {
	tables  map[string][][]string // table -> rows
	schema  [][]string            // sqlite_master rows of every object
	rowCnts map[string]int
}

func m7Capture(t *testing.T, conn *sql.DB) m7State {
	t.Helper()
	st := m7State{tables: map[string][][]string{}, rowCnts: map[string]int{}}
	names := m6Dump(t, conn, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name <> 'schema_migrations' ORDER BY name`)
	for _, row := range names {
		table := strings.TrimPrefix(row[0], "string:")
		if isM7Table(table) {
			continue
		}
		st.tables[table] = m6Dump(t, conn, `SELECT * FROM "`+table+`" ORDER BY rowid`)
		st.rowCnts[table] = m6One(t, conn, `SELECT COUNT(*) FROM "`+table+`"`)
	}
	st.schema = m6Dump(t, conn, `SELECT type, name, tbl_name, sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' AND name <> 'schema_migrations' ORDER BY name`)
	return st
}

func isM7Table(name string) bool {
	for _, n := range m7Tables {
		if n == name {
			return true
		}
	}
	return false
}

// m7Seed puts two learners, two words, two units, and one brief in place for the constraint tests.
func m7Seed(t *testing.T, conn *sql.DB) {
	t.Helper()
	for _, u := range []string{"a", "b"} {
		mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES (?, ?, 'learner', ?)`, u+"@example.com", "sub-"+u, m6Seeded)
	}
	for _, text := range []string{"שָׁלוֹם", "תּוֹדָה"} {
		mustSQL(t, conn, `INSERT INTO lexemes (text_nfc) VALUES (?)`, text)
	}
	for i := 1; i <= 2; i++ {
		mustSQL(t, conn, `INSERT INTO units (position, title, situation, concept, status, rev, doc)
			VALUES (?, 'Hello', 'street', 'greetings', 'published', 1, '{"title":"Hello"}')`, i)
	}
}

const (
	m7Snap   = `INSERT INTO daily_snapshots (user_id, day, skill, state, count, est_known, retention) VALUES (?, ?, ?, ?, ?, ?, ?)`
	m7Event  = `INSERT INTO regression_events (user_id, lexeme_id, skill, from_state, to_state, day, cause) VALUES (?, ?, ?, ?, ?, ?, ?)`
	m7Brief  = `INSERT INTO lesson_briefs (id, user_id, theme, payload_json, created_at, used_by_unit_id) VALUES (?, ?, ?, ?, ?, ?)`
	m7Note   = `INSERT INTO coach_notes (user_id, author, body, created_at) VALUES (?, ?, ?, ?)`
	m7Audit  = `INSERT INTO publish_audit (unit_id, revision, scope, actor, action, brief_id, issues_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	m7Report = `INSERT INTO unit_reports (unit_id, user_id, reason, note, created_at) VALUES (?, ?, ?, ?, ?)`
	m7Weekly = `INSERT INTO weekly_reports (user_id, week_start, payload_json, summary, created_at) VALUES (?, ?, ?, ?, ?)`
)

// ac-1: 008 applies cleanly after 006 on an empty database.
func TestMigrate007AppliesOnEmptyDatabase(t *testing.T) {
	conn := openRaw(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}) {
		t.Fatalf("versions = %v, want 1..12", got)
	}
	for _, table := range m7Tables {
		if !tableExists(t, conn, table) {
			t.Errorf("table %s is missing", table)
		}
		if n := m6One(t, conn, `SELECT COUNT(*) FROM "`+table+`"`); n != 0 {
			t.Errorf("table %s has %d rows on a new database", table, n)
		}
	}
	if got := m6One(t, conn, `PRAGMA foreign_keys`); got != 1 {
		t.Fatalf("foreign_keys = %d after migrate, want 1", got)
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}
	// A second run is a no-op.
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}) {
		t.Fatalf("versions after rerun = %v", got)
	}
}

// ac-1: 008 applies cleanly to a database seeded at 006, and no existing table, row, or index changes.
func TestMigrate007AppliesOnSeededDatabaseAndLeavesExistingTablesAlone(t *testing.T) {
	conn := m7At006(t)
	before := m7Capture(t, conn)
	for _, table := range []string{"users", "lexemes", "cards", "review_log", "attempts", "exposures", "units", "unit_revisions"} {
		if before.rowCnts[table] == 0 {
			t.Fatalf("seed has no %s rows, so the check below would prove nothing", table)
		}
	}
	if tableExists(t, conn, "daily_snapshots") {
		t.Fatal("daily_snapshots exists before 008")
	}

	// Apply 008 alone. Migrate would go on to 009, which rebuilds confusion_misses on purpose.
	body, err := fs.ReadFile(migrations.FS, "008_learner_model.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(context.Background(), conn, "008_learner_model.sql", 8, string(body), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("migrate 008: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatalf("versions = %v, want 1..8", got)
	}

	after := m7Capture(t, conn)
	if !reflect.DeepEqual(after.rowCnts, before.rowCnts) {
		t.Fatalf("row counts changed\n got: %v\nwant: %v", after.rowCnts, before.rowCnts)
	}
	for table, rows := range before.tables {
		if !reflect.DeepEqual(after.tables[table], rows) {
			t.Errorf("rows of %s changed\n got: %v\nwant: %v", table, after.tables[table], rows)
		}
	}
	// The schema of every older table and index is byte for byte what it was. Only the new objects are added.
	var oldSchema [][]string
	for _, row := range after.schema {
		if !isM7Table(strings.TrimPrefix(row[2], "string:")) {
			oldSchema = append(oldSchema, row)
		}
	}
	if !reflect.DeepEqual(oldSchema, before.schema) {
		t.Fatalf("schema of existing objects changed\n got: %v\nwant: %v", oldSchema, before.schema)
	}
	for _, table := range m7Tables {
		if n := m6One(t, conn, `SELECT COUNT(*) FROM "`+table+`"`); n != 0 {
			t.Errorf("new table %s has %d rows", table, n)
		}
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}
	if got := m6One(t, conn, `PRAGMA foreign_keys`); got != 1 {
		t.Fatalf("foreign_keys = %d, want 1", got)
	}
}

// 008 is one transaction: a failure on its last statement leaves the database at 006 with none of the new tables.
func TestMigrate007AtomicWhenAStatementFails(t *testing.T) {
	conn := m7At006(t)
	// weekly_reports is created last. A table of that name makes the script fail after six tables exist.
	mustSQL(t, conn, `CREATE TABLE weekly_reports (x INTEGER)`)
	err := m6Migrate(conn)
	if err == nil {
		t.Fatal("migration 008 succeeded although weekly_reports already exists")
	}
	if !strings.Contains(err.Error(), "008_learner_model.sql") {
		t.Fatalf("error = %v, want it to name 008_learner_model.sql", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7}) {
		t.Fatalf("versions = %v, want 1..7 (007 is the OAuth marker and runs before the failing 008)", got)
	}
	for _, table := range m7Tables {
		if table == "weekly_reports" {
			continue
		}
		if tableExists(t, conn, table) {
			t.Errorf("table %s survived the failed migration", table)
		}
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'daily_snapshots_%' OR name LIKE 'coach_notes_%' OR name LIKE 'publish_audit_%'`); n != 0 {
		t.Errorf("%d indexes survived the failed migration", n)
	}

	// Once the conflict is gone, the migration applies.
	mustSQL(t, conn, `DROP TABLE weekly_reports`)
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}) {
		t.Fatalf("versions after retry = %v", got)
	}
}

// ac-2: a second row for the same learner, day, skill, and state is rejected.
func TestMigrate007DailySnapshotsRejectDuplicate(t *testing.T) {
	conn := m7Fresh(t)
	m7Seed(t, conn)

	mustSQL(t, conn, m7Snap, 1, m7Day, "read", "known", 40, 38.5, 0.9)
	rejectSQL(t, conn, "same learner, day, skill, state", m7Snap, 1, m7Day, "read", "known", 41, 39.0, 0.8)
	// Changing any one part of the key is a different row.
	mustSQL(t, conn, m7Snap, 1, m7Day, "read", "solid", 10, 10.0, 0.95)
	mustSQL(t, conn, m7Snap, 1, m7Day, "recall", "known", 5, 5.0, nil)
	mustSQL(t, conn, m7Snap, 1, "2026-10-20", "read", "known", 41, 39.0, 0.9)
	mustSQL(t, conn, m7Snap, 2, m7Day, "read", "known", 3, 3.0, nil)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM daily_snapshots`); n != 5 {
		t.Fatalf("daily_snapshots = %d, want 5", n)
	}
	// The stored row is the first one.
	if n := m6One(t, conn, `SELECT count FROM daily_snapshots WHERE user_id = 1 AND day = ? AND skill = 'read' AND state = 'known'`, m7Day); n != 40 {
		t.Fatalf("count = %d, want the first row's 40", n)
	}
}

// ac-3: a coach note body of 501 characters is rejected and one of 500 is accepted.
func TestMigrate007CoachNoteBodyLimit(t *testing.T) {
	conn := m7Fresh(t)
	m7Seed(t, conn)

	mustSQL(t, conn, m7Note, 1, "weekly-coach", strings.Repeat("a", 500), m6Seeded)
	rejectSQL(t, conn, "501 characters", m7Note, 1, "weekly-coach", strings.Repeat("a", 501), m6Seeded)
	// The cap counts characters, not bytes: 500 Hebrew letters are 1000 bytes and fit, 501 do not.
	mustSQL(t, conn, m7Note, 1, "weekly-coach", strings.Repeat("א", 500), m6Seeded)
	rejectSQL(t, conn, "501 Hebrew characters", m7Note, 1, "weekly-coach", strings.Repeat("א", 501), m6Seeded)
	mustSQL(t, conn, m7Note, 1, "weekly-coach", "x", m6Seeded)
	rejectSQL(t, conn, "empty body", m7Note, 1, "weekly-coach", "", m6Seeded)
	rejectSQL(t, conn, "null body", m7Note, 1, "weekly-coach", nil, m6Seeded)
	rejectSQL(t, conn, "blob body", m7Note, 1, "weekly-coach", []byte("not text"), m6Seeded)
	rejectSQL(t, conn, "empty author", m7Note, 1, "", "hello", m6Seeded)
	rejectSQL(t, conn, "long author", m7Note, 1, strings.Repeat("a", 101), "hello", m6Seeded)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM coach_notes`); n != 3 {
		t.Fatalf("coach_notes = %d, want 3", n)
	}
}

// The weekly summary is capped at 600 characters and the key is one report per learner per week.
func TestMigrate007WeeklyReportSummaryLimit(t *testing.T) {
	conn := m7Fresh(t)
	m7Seed(t, conn)

	mustSQL(t, conn, m7Weekly, 1, "2026-10-12", `{"words":12}`, strings.Repeat("a", 600), m6Seeded)
	rejectSQL(t, conn, "601 characters", m7Weekly, 1, "2026-10-19", `{"words":12}`, strings.Repeat("a", 601), m6Seeded)
	mustSQL(t, conn, m7Weekly, 1, "2026-10-19", `{}`, strings.Repeat("א", 600), m6Seeded)
	rejectSQL(t, conn, "601 Hebrew characters", m7Weekly, 1, "2026-10-26", `{}`, strings.Repeat("א", 601), m6Seeded)
	rejectSQL(t, conn, "blob summary", m7Weekly, 1, "2026-10-26", `{}`, []byte("blob"), m6Seeded)
	rejectSQL(t, conn, "second report for the same week", m7Weekly, 1, "2026-10-12", `{}`, "again", m6Seeded)
	mustSQL(t, conn, m7Weekly, 2, "2026-10-12", `{}`, "other learner", m6Seeded)
	rejectSQL(t, conn, "payload is not JSON", m7Weekly, 2, "2026-10-19", `{not json`, "ok", m6Seeded)
	rejectSQL(t, conn, "week_start not a date", m7Weekly, 2, "2026-13-45", `{}`, "ok", m6Seeded)
	rejectSQL(t, conn, "week_start wrong shape", m7Weekly, 2, "10/19/2026", `{}`, "ok", m6Seeded)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM weekly_reports`); n != 3 {
		t.Fatalf("weekly_reports = %d, want 3", n)
	}
}

// The closed vocabularies reject every value outside them and accept every value inside them.
func TestMigrate007ClosedVocabulariesRejectBadValues(t *testing.T) {
	conn := m7Fresh(t)
	m7Seed(t, conn)
	skills := []string{"listen", "read", "recall", "spell"}
	states := []string{"unseen", "seen", "learning", "struggling", "known", "solid", "regressed"}

	// daily_snapshots: every skill and state is accepted.
	for _, skill := range skills {
		for _, state := range states {
			mustSQL(t, conn, m7Snap, 1, m7Day, skill, state, 1, 1.0, nil)
		}
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM daily_snapshots`); n != 28 {
		t.Fatalf("daily_snapshots = %d, want 28", n)
	}
	for _, bad := range []string{"write", "he_en", "", "Read", "READ"} {
		rejectSQL(t, conn, "snapshot skill "+bad, m7Snap, 2, m7Day, bad, "known", 1, 1.0, nil)
	}
	for _, bad := range []string{"new", "mastered", "", "Known", "review"} {
		rejectSQL(t, conn, "snapshot state "+bad, m7Snap, 2, m7Day, "read", bad, 1, 1.0, nil)
	}
	rejectSQL(t, conn, "negative count", m7Snap, 2, m7Day, "read", "known", -1, 1.0, nil)
	rejectSQL(t, conn, "negative est_known", m7Snap, 2, m7Day, "read", "known", 1, -0.5, nil)
	rejectSQL(t, conn, "retention above 1", m7Snap, 2, m7Day, "read", "known", 1, 1.0, 1.01)
	rejectSQL(t, conn, "retention below 0", m7Snap, 2, m7Day, "read", "known", 1, 1.0, -0.01)
	mustSQL(t, conn, m7Snap, 2, m7Day, "read", "known", 1, 1.0, 0.0)
	mustSQL(t, conn, m7Snap, 2, m7Day, "read", "solid", 1, 1.0, 1.0)
	rejectSQL(t, conn, "day not a date", m7Snap, 2, "2026-02-30", "read", "seen", 1, 1.0, nil)
	rejectSQL(t, conn, "day with a time", m7Snap, 2, "2026-10-19T00:00:00Z", "read", "seen", 1, 1.0, nil)
	rejectSQL(t, conn, "empty day", m7Snap, 2, "", "read", "seen", 1, 1.0, nil)
	rejectSQL(t, conn, "null est_known", m7Snap, 2, m7Day, "read", "seen", 1, nil, nil)

	// regression_events: from_state and to_state use the same states, and must differ.
	for _, skill := range skills {
		mustSQL(t, conn, m7Event, 1, 1, skill, "solid", "regressed", m7Day, "decay")
	}
	for _, state := range states[1:] {
		mustSQL(t, conn, m7Event, 1, 2, "read", states[0], state, m7Day, "first answer")
	}
	for _, bad := range []string{"write", "", "he_en"} {
		rejectSQL(t, conn, "event skill "+bad, m7Event, 1, 1, bad, "known", "regressed", m7Day, "decay")
	}
	rejectSQL(t, conn, "event from_state", m7Event, 1, 1, "read", "mastered", "regressed", m7Day, "decay")
	rejectSQL(t, conn, "event to_state", m7Event, 1, 1, "read", "known", "forgotten", m7Day, "decay")
	rejectSQL(t, conn, "no change of state", m7Event, 1, 1, "read", "known", "known", m7Day, "decay")
	rejectSQL(t, conn, "event day", m7Event, 1, 1, "read", "known", "regressed", "yesterday", "decay")
	rejectSQL(t, conn, "empty cause", m7Event, 1, 1, "read", "known", "regressed", m7Day, "")
	rejectSQL(t, conn, "null cause", m7Event, 1, 1, "read", "known", "regressed", m7Day, nil)

	// publish_audit.action: only the three actions are accepted.
	for _, action := range []string{"auto_publish", "unpublish", "report_hide"} {
		mustSQL(t, conn, m7Audit, 1, 1, "learner:1", "claude", action, nil, "[]", m6Seeded)
	}
	for _, bad := range []string{"publish", "approve", "", "Auto_Publish", "hide"} {
		rejectSQL(t, conn, "audit action "+bad, m7Audit, 1, 1, "learner:1", "claude", bad, nil, "[]", m6Seeded)
	}
	rejectSQL(t, conn, "audit revision 0", m7Audit, 1, 0, "learner:1", "claude", "auto_publish", nil, "[]", m6Seeded)
	rejectSQL(t, conn, "audit empty scope", m7Audit, 1, 1, "", "claude", "auto_publish", nil, "[]", m6Seeded)
	rejectSQL(t, conn, "audit empty actor", m7Audit, 1, 1, "learner:1", "", "auto_publish", nil, "[]", m6Seeded)
	rejectSQL(t, conn, "audit issues not JSON", m7Audit, 1, 1, "learner:1", "claude", "auto_publish", nil, "not json", m6Seeded)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM publish_audit`); n != 3 {
		t.Fatalf("publish_audit = %d, want 3", n)
	}
	// issues_json defaults to an empty list.
	mustSQL(t, conn, `INSERT INTO publish_audit (unit_id, revision, scope, actor, action, created_at) VALUES (1, 1, 'global', 'claude', 'unpublish', ?)`, m6Seeded)
	var issues string
	if err := conn.QueryRowContext(context.Background(), `SELECT issues_json FROM publish_audit ORDER BY id DESC LIMIT 1`).Scan(&issues); err != nil || issues != "[]" {
		t.Fatalf("default issues_json = %q, %v", issues, err)
	}
}

// Briefs, reports, and the remaining text columns are plain text with caps.
func TestMigrate007BriefsAndReportsAreCapped(t *testing.T) {
	conn := m7Fresh(t)
	m7Seed(t, conn)

	// lesson_briefs.id is text: a 32 character hex id and a 22 character base64url id both fit.
	mustSQL(t, conn, m7Brief, m7ID, 1, "the market", `{"teach":[1,2]}`, m6Seeded, nil)
	mustSQL(t, conn, m7Brief, "AbCdEfGhIjKlMnOpQrStUv", 1, "greetings", `{}`, m6Seeded, nil)
	var typ string
	if err := conn.QueryRowContext(context.Background(), `SELECT typeof(id) FROM lesson_briefs WHERE id = ?`, m7ID).Scan(&typ); err != nil || typ != "text" {
		t.Fatalf("typeof(lesson_briefs.id) = %q, %v, want text", typ, err)
	}
	rejectSQL(t, conn, "duplicate brief id", m7Brief, m7ID, 2, "other", `{}`, m6Seeded, nil)
	rejectSQL(t, conn, "counter-style id", m7Brief, "1", 1, "short id", `{}`, m6Seeded, nil)
	rejectSQL(t, conn, "empty id", m7Brief, "", 1, "empty id", `{}`, m6Seeded, nil)
	rejectSQL(t, conn, "id of 65 characters", m7Brief, strings.Repeat("a", 65), 1, "long id", `{}`, m6Seeded, nil)
	rejectSQL(t, conn, "integer id", m7Brief, int64(1234567890123456789), 1, "int id", `{}`, m6Seeded, nil)
	rejectSQL(t, conn, "null id", m7Brief, nil, 1, "null id", `{}`, m6Seeded, nil)
	rejectSQL(t, conn, "brief payload not JSON", m7Brief, strings.Repeat("b", 32), 1, "t", `{oops`, m6Seeded, nil)
	rejectSQL(t, conn, "empty theme", m7Brief, strings.Repeat("c", 32), 1, "", `{}`, m6Seeded, nil)
	rejectSQL(t, conn, "theme of 201 characters", m7Brief, strings.Repeat("d", 32), 1, strings.Repeat("t", 201), `{}`, m6Seeded, nil)
	mustSQL(t, conn, m7Brief, strings.Repeat("e", 32), 1, strings.Repeat("t", 200), `{}`, m6Seeded, nil)

	// unit_reports.note is capped at 500 characters and may be empty. reason is 1 to 100.
	mustSQL(t, conn, m7Report, 1, 1, "wrong form", strings.Repeat("n", 500), m6Seeded)
	rejectSQL(t, conn, "report note of 501", m7Report, 1, 1, "wrong form", strings.Repeat("n", 501), m6Seeded)
	rejectSQL(t, conn, "report note of 501 Hebrew characters", m7Report, 1, 1, "wrong form", strings.Repeat("א", 501), m6Seeded)
	rejectSQL(t, conn, "report note as blob", m7Report, 1, 1, "wrong form", []byte("blob"), m6Seeded)
	rejectSQL(t, conn, "null note", m7Report, 1, 1, "wrong form", nil, m6Seeded)
	rejectSQL(t, conn, "empty reason", m7Report, 1, 1, "", "", m6Seeded)
	rejectSQL(t, conn, "reason of 101", m7Report, 1, 1, strings.Repeat("r", 101), "", m6Seeded)
	mustSQL(t, conn, m7Report, 2, 2, "confusing", "", m6Seeded)
	mustSQL(t, conn, `INSERT INTO unit_reports (unit_id, user_id, reason, created_at) VALUES (1, 2, 'typo', ?)`, m6Seeded)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM unit_reports WHERE note = ''`); n != 2 {
		t.Fatalf("reports with an empty note = %d, want 2", n)
	}
}

// Foreign keys: a missing parent is refused, learner data goes with the learner, and the audit trail outlives its unit.
func TestMigrate007ForeignKeysAndDeletes(t *testing.T) {
	conn := m7Fresh(t)
	m7Seed(t, conn)

	// A missing parent is refused in every table.
	rejectSQL(t, conn, "snapshot for a missing user", m7Snap, 99, m7Day, "read", "known", 1, 1.0, nil)
	rejectSQL(t, conn, "event for a missing user", m7Event, 99, 1, "read", "known", "regressed", m7Day, "decay")
	rejectSQL(t, conn, "event for a missing lexeme", m7Event, 1, 99, "read", "known", "regressed", m7Day, "decay")
	rejectSQL(t, conn, "brief for a missing user", m7Brief, m7ID, 99, "t", `{}`, m6Seeded, nil)
	rejectSQL(t, conn, "brief used by a missing unit", m7Brief, m7ID, 1, "t", `{}`, m6Seeded, 99)
	rejectSQL(t, conn, "note for a missing user", m7Note, 99, "coach", "hi", m6Seeded)
	rejectSQL(t, conn, "audit for a missing unit", m7Audit, 99, 1, "global", "claude", "auto_publish", nil, "[]", m6Seeded)
	rejectSQL(t, conn, "audit with a missing brief", m7Audit, 1, 1, "global", "claude", "auto_publish", m7ID, "[]", m6Seeded)
	rejectSQL(t, conn, "report for a missing unit", m7Report, 99, 1, "typo", "", m6Seeded)
	rejectSQL(t, conn, "report for a missing user", m7Report, 1, 99, "typo", "", m6Seeded)
	rejectSQL(t, conn, "weekly report for a missing user", m7Weekly, 99, "2026-10-12", `{}`, "s", m6Seeded)

	// Learner 1 owns one row in each learner table. Learner 2 owns one too, so we can see what survives.
	for _, u := range []int{1, 2} {
		brief := strings.Repeat(string(rune('a'+u)), 32)
		mustSQL(t, conn, m7Snap, u, m7Day, "read", "known", 4, 4.0, 0.9)
		mustSQL(t, conn, m7Event, u, 1, "read", "known", "regressed", m7Day, "decay")
		mustSQL(t, conn, m7Brief, brief, u, "theme", `{}`, m6Seeded, nil)
		mustSQL(t, conn, m7Note, u, "coach", "hello", m6Seeded)
		mustSQL(t, conn, m7Report, 1, u, "typo", "", m6Seeded)
		mustSQL(t, conn, m7Weekly, u, "2026-10-12", `{}`, "summary", m6Seeded)
	}
	learnerTables := []string{"daily_snapshots", "regression_events", "lesson_briefs", "coach_notes", "unit_reports", "weekly_reports"}
	for _, table := range learnerTables {
		if n := m6One(t, conn, `SELECT COUNT(*) FROM "`+table+`"`); n != 2 {
			t.Fatalf("%s = %d before the delete, want 2", table, n)
		}
	}
	// Deleting learner 1 deletes exactly their rows and none of learner 2's.
	mustSQL(t, conn, `DELETE FROM users WHERE id = 1`)
	for _, table := range learnerTables {
		if n := m6One(t, conn, `SELECT COUNT(*) FROM "`+table+`"`); n != 1 {
			t.Errorf("%s = %d after deleting a learner, want 1", table, n)
		}
		if n := m6One(t, conn, `SELECT COUNT(*) FROM "`+table+`" WHERE user_id = 1`); n != 0 {
			t.Errorf("%s still has rows of the deleted learner", table)
		}
	}

	// A word with a regression event cannot be deleted. Word 2 has none, so it can.
	if _, err := conn.ExecContext(context.Background(), `DELETE FROM lexemes WHERE id = 1`); err == nil {
		t.Error("lexeme with regression events was deleted")
	}
	mustSQL(t, conn, `DELETE FROM lexemes WHERE id = 2`)

	// The audit trail outlives its unit and its brief. unit_id and brief_id go to null.
	mustSQL(t, conn, m7Brief, m7ID, 2, "market", `{}`, m6Seeded, 1)
	mustSQL(t, conn, m7Audit, 1, 3, "learner:2", "claude", "auto_publish", m7ID, `[{"code":"x"}]`, m6Seeded)
	mustSQL(t, conn, m7Audit, 2, 1, "learner:2", "claude", "unpublish", nil, "[]", m6Seeded)
	mustSQL(t, conn, m7Report, 1, 2, "typo", "", m6Seeded)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM unit_reports WHERE unit_id = 1`); n != 2 {
		t.Fatalf("reports on unit 1 = %d, want 2", n)
	}
	mustSQL(t, conn, `DELETE FROM units WHERE id = 1`)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM publish_audit`); n != 2 {
		t.Fatalf("publish_audit = %d after deleting a unit, want both rows kept", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM publish_audit WHERE unit_id IS NULL AND revision = 3 AND action = 'auto_publish' AND brief_id = ?`, m7ID); n != 1 {
		t.Fatal("the audit row of the deleted unit was not kept with a null unit_id")
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM publish_audit WHERE unit_id = 2`); n != 1 {
		t.Fatal("the audit row of the other unit changed")
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM unit_reports WHERE unit_id = 1`); n != 0 {
		t.Fatalf("%d reports outlived their unit", n)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM lesson_briefs WHERE id = ? AND used_by_unit_id IS NULL`, m7ID); n != 1 {
		t.Fatal("brief kept a reference to the deleted unit")
	}
	mustSQL(t, conn, `DELETE FROM lesson_briefs WHERE id = ?`, m7ID)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM publish_audit WHERE brief_id IS NULL`); n != 2 {
		t.Fatalf("audit rows with a null brief = %d, want 2 after deleting the brief", n)
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}
}

// The read-path indexes exist with the columns the readers filter on, and every column line in the SQL has an inline comment.
func TestMigrate007IndexesAndColumnComments(t *testing.T) {
	conn := m7Fresh(t)
	wantIdx := map[string][]string{
		"daily_snapshots_user_skill_day":      {"user_id", "skill", "day"},
		"regression_events_user_day":          {"user_id", "day"},
		"regression_events_user_lexeme_skill": {"user_id", "lexeme_id", "skill"},
		"lesson_briefs_user_created":          {"user_id", "created_at"},
		"coach_notes_user_created":            {"user_id", "created_at"},
		"publish_audit_unit_created":          {"unit_id", "created_at"},
		"publish_audit_created":               {"created_at"},
		"unit_reports_unit":                   {"unit_id", "created_at"},
		"unit_reports_user":                   {"user_id", "created_at"},
	}
	for name, want := range wantIdx {
		if got := indexColumns(t, conn, name); !reflect.DeepEqual(got, want) {
			t.Errorf("index %s columns = %v, want %v", name, got, want)
		}
	}
	wantCols := map[string][]string{
		"daily_snapshots":   {"user_id", "day", "skill", "state", "count", "est_known", "retention"},
		"regression_events": {"id", "user_id", "lexeme_id", "skill", "from_state", "to_state", "day", "cause", "kind"}, // kind was added by 010.
		"lesson_briefs":     {"id", "user_id", "theme", "payload_json", "created_at", "used_by_unit_id"},
		"coach_notes":       {"id", "user_id", "author", "body", "created_at"},
		"publish_audit":     {"id", "unit_id", "revision", "scope", "actor", "action", "brief_id", "issues_json", "created_at"},
		"unit_reports":      {"id", "unit_id", "user_id", "reason", "note", "created_at"},
		"weekly_reports":    {"user_id", "week_start", "payload_json", "summary", "created_at"},
	}
	for table, want := range wantCols {
		rows := m6Dump(t, conn, `SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
		var got []string
		for _, r := range rows {
			got = append(got, strings.TrimPrefix(r[0], "string:"))
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s columns = %v, want %v", table, got, want)
		}
	}
	// Primary keys named by the spec.
	pk := func(table string) []string {
		rows := m6Dump(t, conn, `SELECT name FROM pragma_table_info(?) WHERE pk > 0 ORDER BY pk`, table)
		var got []string
		for _, r := range rows {
			got = append(got, strings.TrimPrefix(r[0], "string:"))
		}
		return got
	}
	if got := pk("daily_snapshots"); !reflect.DeepEqual(got, []string{"user_id", "day", "skill", "state"}) {
		t.Errorf("daily_snapshots primary key = %v", got)
	}
	if got := pk("weekly_reports"); !reflect.DeepEqual(got, []string{"user_id", "week_start"}) {
		t.Errorf("weekly_reports primary key = %v", got)
	}
	if got := m6Dump(t, conn, `SELECT type FROM pragma_table_info('lesson_briefs') WHERE name = 'id'`); !reflect.DeepEqual(got, [][]string{{"string:TEXT"}}) {
		t.Errorf("lesson_briefs.id type = %v, want TEXT", got)
	}

	body, err := fs.ReadFile(migrations.FS, "008_learner_model.sql")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	tables := 0
	for _, stmt := range statements(string(body)) {
		if !strings.HasPrefix(stripLeadingComments(stmt), "CREATE TABLE") {
			continue
		}
		tables++
		inBody := false
		for _, line := range strings.Split(stmt, "\n") {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "CREATE TABLE") {
				inBody = true
				continue
			}
			if !inBody || trim == "" || strings.HasPrefix(trim, ")") || strings.HasPrefix(trim, "PRIMARY KEY") || strings.HasPrefix(trim, "UNIQUE") || strings.HasPrefix(trim, "--") {
				continue
			}
			checked++
			if !strings.Contains(line, "-- ") {
				t.Errorf("column line has no inline comment: %q", trim)
			}
		}
	}
	if tables != 7 {
		t.Errorf("008 creates %d tables, want 7", tables)
	}
	// daily_snapshots 7 + regression_events 8 + lesson_briefs 6 + coach_notes 5 + publish_audit 9 + unit_reports 6 + weekly_reports 5.
	if checked != 46 {
		t.Errorf("checked %d column lines, want 46", checked)
	}
}

// 008 only creates. It never alters, drops, or rewrites a table that already exists.
func TestMigrate007OnlyCreatesNewObjects(t *testing.T) {
	body, err := fs.ReadFile(migrations.FS, "008_learner_model.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range statements(string(body)) {
		s := strings.ToUpper(strings.TrimSpace(stripLeadingComments(stmt)))
		switch {
		case strings.HasPrefix(s, "CREATE TABLE "), strings.HasPrefix(s, "CREATE INDEX "), s == "PRAGMA FOREIGN_KEY_CHECK":
		default:
			t.Errorf("008 holds a statement that is not an additive create: %.60q", s)
		}
	}
}
