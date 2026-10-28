package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

const (
	m6Seeded = "2020-01-01T00:00:00Z"
	// m6Version is the migration under test.
	m6Version = 6
)

// m6At005 builds a database at version 005 from the embedded migrations. It
// stops before 006, records each applied version in schema_migrations, and
// seeds learners, words, cards in both directions, and review history.
func m6At005(t *testing.T) *sql.DB {
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
		if version >= m6Version {
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
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("seed database versions = %v, want 1..5", got)
	}
	m6Seed(t, conn)
	return conn
}

func m6Seed(t *testing.T, conn *sql.DB) {
	t.Helper()
	for _, u := range []string{"a", "b", "c"} {
		mustSQL(t, conn, `INSERT INTO users (email, google_sub, role, created_at) VALUES (?, ?, 'learner', ?)`,
			u+"@example.com", "sub-"+u, m6Seeded)
	}
	for _, text := range []string{"שָׁלוֹם", "תּוֹדָה", "בְּבַקָּשָׁה", "לֶחֶם"} {
		mustSQL(t, conn, `INSERT INTO lexemes (text_nfc) VALUES (?)`, text)
	}
	// user, lexeme, dir, state, due, stability, difficulty, reps, lapses, last_review, suspended
	type card struct {
		user, lexeme  int
		dir, state    string
		stability     float64
		difficulty    float64
		reps, lapses  int
		lastReview    any
		suspended     int
		reviewRatings []int
	}
	cards := []card{
		{1, 1, "he_en", "review", 12.5, 4.25, 6, 1, "2026-09-01T08:00:00Z", 0, []int{3, 3, 1, 4}},
		{1, 1, "en_he", "learning", 1.5, 6.5, 2, 0, "2026-09-02T08:00:00Z", 0, []int{2, 3}},
		{1, 2, "he_en", "new", 0, 0, 0, 0, nil, 0, nil},
		{1, 3, "en_he", "relearning", 0.5, 8, 9, 3, "2026-09-03T08:00:00Z", 1, []int{1, 1, 3}},
		{2, 1, "he_en", "review", 30, 3, 11, 0, "2026-09-04T08:00:00Z", 0, []int{4, 4, 3}},
		{2, 4, "en_he", "new", 0, 0, 0, 0, nil, 0, nil},
		{3, 2, "he_en", "learning", 2, 5, 1, 0, "2026-09-05T08:00:00Z", 0, []int{3}},
	}
	for _, c := range cards {
		res, err := conn.ExecContext(context.Background(), `
			INSERT INTO cards (user_id, lexeme_id, dir, state, due, stability, difficulty, reps, lapses, last_review, suspended)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			c.user, c.lexeme, c.dir, c.state, "2026-10-01T00:00:00Z", c.stability, c.difficulty, c.reps, c.lapses, c.lastReview, c.suspended)
		if err != nil {
			t.Fatal(err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		for i, rating := range c.reviewRatings {
			mustSQL(t, conn, `
				INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days)
				VALUES (?, ?, ?, ?, ?)`, id, rating, fmt.Sprintf("2026-08-%02dT07:00:00Z", i+1), i, i+1)
		}
	}
	// Leave a gap in the card ids, as a deleted card would. Card 3 has no history.
	mustSQL(t, conn, `DELETE FROM cards WHERE id = 3`)
}

func m6Applied(t *testing.T, conn *sql.DB) []int {
	t.Helper()
	rows, err := conn.QueryContext(context.Background(), `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatal(err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// m6Dump returns every row of a query as strings, so two snapshots compare with DeepEqual.
func m6Dump(t *testing.T, conn *sql.DB, query string, args ...any) [][]string {
	t.Helper()
	rows, err := conn.QueryContext(context.Background(), query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var out [][]string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		row := make([]string, len(cols))
		for i, v := range vals {
			row[i] = fmt.Sprintf("%T:%v", v, v)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func m6One(t *testing.T, conn *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := conn.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func m6Migrate(conn *sql.DB) error {
	return Migrate(context.Background(), conn, fixedClock{time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)})
}

const (
	m6OldCards = `SELECT id, user_id, lexeme_id, dir, state, due, stability, difficulty, reps, lapses, last_review, suspended FROM cards ORDER BY id`
	m6NewCards = `SELECT id, user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended FROM cards ORDER BY id`
	m6Log      = `SELECT * FROM review_log ORDER BY id`
)

// mapDir applies the spec's mapping to the dir column of a dumped row.
func m6MapDir(rows [][]string) [][]string {
	out := make([][]string, len(rows))
	for i, row := range rows {
		cp := append([]string(nil), row...)
		switch cp[3] {
		case "string:he_en":
			cp[3] = "string:read"
		case "string:en_he":
			cp[3] = "string:recall"
		default:
			cp[3] = "unmapped:" + cp[3]
		}
		out[i] = cp
	}
	return out
}

// ac-1: the rebuild keeps every card, every id, the mapped skill, and every review_log row.
func TestMigrate006KeepsCardsIdsAndReviewHistory(t *testing.T) {
	conn := m6At005(t)

	oldCards := m6Dump(t, conn, m6OldCards)
	oldLog := m6Dump(t, conn, m6Log)
	if len(oldCards) != 6 || len(oldLog) == 0 {
		t.Fatalf("seed has %d cards and %d review_log rows", len(oldCards), len(oldLog))
	}
	var he, en int
	for _, row := range oldCards {
		switch row[3] {
		case "string:he_en":
			he++
		case "string:en_he":
			en++
		}
	}
	if he == 0 || en == 0 {
		t.Fatalf("seed must hold both directions, has he_en=%d en_he=%d", he, en)
	}
	// The seed must have a gap, so card ids are not just 1..n.
	if oldCards[len(oldCards)-1][0] == fmt.Sprintf("int64:%d", len(oldCards)) {
		t.Fatalf("seed card ids have no gap: %v", oldCards)
	}

	if err := m6Migrate(conn); err != nil {
		t.Fatalf("migrate 006: %v", err)
	}

	// Migrate applies every pending file, so 008 (INIT-015/SPEC-004) and 009 (INIT-015/SPEC-013) are recorded too.
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}) {
		t.Fatalf("versions after = %v", got)
	}
	cols := columnNames(t, conn, "cards")
	if cols["dir"] || !cols["skill"] {
		t.Fatalf("cards columns = %v, want skill and no dir", cols)
	}

	newCards := m6Dump(t, conn, m6NewCards)
	if len(newCards) != len(oldCards) {
		t.Fatalf("card count %d -> %d", len(oldCards), len(newCards))
	}
	if want := m6MapDir(oldCards); !reflect.DeepEqual(newCards, want) {
		t.Fatalf("cards changed beyond the dir mapping\n got: %v\nwant: %v", newCards, want)
	}
	for i, row := range newCards {
		if row[0] != oldCards[i][0] {
			t.Fatalf("card id changed: %s -> %s", oldCards[i][0], row[0])
		}
		if row[3] != "string:read" && row[3] != "string:recall" {
			t.Fatalf("card %s has skill %s", row[0], row[3])
		}
	}

	newLog := m6Dump(t, conn, m6Log)
	if len(newLog) != len(oldLog) {
		t.Fatalf("review_log count %d -> %d", len(oldLog), len(newLog))
	}
	if !reflect.DeepEqual(newLog, oldLog) {
		t.Fatalf("review_log rows changed\n got: %v\nwant: %v", newLog, oldLog)
	}
	// Every log row still points at a card, and at the same card.
	if n := m6One(t, conn, `SELECT COUNT(*) FROM review_log r LEFT JOIN cards c ON c.id = r.card_id WHERE c.id IS NULL`); n != 0 {
		t.Fatalf("%d review_log rows have no card", n)
	}
	if fk := m6Dump(t, conn, `PRAGMA foreign_key_check`); len(fk) != 0 {
		t.Fatalf("foreign_key_check: %v", fk)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_temp_master WHERE name = 'review_log_keep'`); n != 0 {
		t.Fatal("temp table review_log_keep was left behind")
	}
}

// The rebuilt table keeps its index, its unique key on the new column, its CHECKs, and the foreign keys on both sides.
func TestMigrate006RebuiltCardsKeepsConstraints(t *testing.T) {
	conn := m6At005(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	if got := m6One(t, conn, `PRAGMA foreign_keys`); got != 1 {
		t.Fatalf("foreign_keys = %d after migrate, want 1", got)
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'cards_user_due' AND tbl_name = 'cards'`); n != 1 {
		t.Fatal("cards_user_due index is missing")
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE name LIKE 'cards_new%'`); n != 0 {
		t.Fatal("cards_new was left behind")
	}

	const insert = `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses)
		VALUES (?, ?, ?, 'new', ?, 0, 0, 0, 0)`
	rejectSQL(t, conn, "duplicate user+lexeme+skill", insert, 1, 1, "read", m6Seeded)
	rejectSQL(t, conn, "old direction value", insert, 2, 2, "he_en", m6Seeded)
	rejectSQL(t, conn, "unknown skill", insert, 2, 2, "write", m6Seeded)
	mustSQL(t, conn, insert, 1, 1, "listen", m6Seeded)
	mustSQL(t, conn, insert, 1, 1, "spell", m6Seeded)
	rejectSQL(t, conn, "missing user", insert, 99, 1, "read", m6Seeded)

	// review_log still enforces its foreign key and still cascades from cards.
	rejectSQL(t, conn, "orphan review", `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (9999, 3, ?, 0, 0)`, m6Seeded)
	card := m6One(t, conn, `SELECT id FROM cards WHERE user_id = 1 AND lexeme_id = 1 AND skill = 'read'`)
	if m6One(t, conn, `SELECT COUNT(*) FROM review_log WHERE card_id = ?`, card) == 0 {
		t.Fatal("seed card has no review history")
	}
	mustSQL(t, conn, `DELETE FROM cards WHERE id = ?`, card)
	if n := m6One(t, conn, `SELECT COUNT(*) FROM review_log WHERE card_id = ?`, card); n != 0 {
		t.Fatalf("%d review_log rows survived the card delete", n)
	}
}

// ac-1 on the real path: Open applies every migration to an empty file and the cards table is usable.
func TestMigrate006FreshDatabaseHasSkillCards(t *testing.T) {
	s := openAt(t, t.TempDir(), nil)
	userID := insertUser(t, s, "a@example.com", "sub-a", "learner")
	lexemeID := insertLexeme(t, s, "שָׁלוֹם")
	for _, skill := range []string{"listen", "read", "recall", "spell"} {
		mustExec(t, s, `INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses)
			VALUES (?, ?, ?, 'new', ?, 0, 0, 0, 0)`, userID, lexemeID, skill, ts)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM cards`); n != 4 {
		t.Fatalf("cards = %d, want one per skill", n)
	}
}

// ac-2: attempts and exposures enforce their vocabularies and keys.
func TestMigrate006AttemptsRejectUnknownSkillOrSource(t *testing.T) {
	s := openAt(t, t.TempDir(), nil)
	userID := insertUser(t, s, "a@example.com", "sub-a", "learner")
	lexemeID := insertLexeme(t, s, "שָׁלוֹם")
	unitID := insertUnit(t, s, "published")

	const insert = `INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, unit_id, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
		VALUES (?, ?, ?, 'listen_pick', ?, ?, 'p', 'r', ?, ?, ?, ?, ?)`
	good := func(skill, source string, unit any, correct, dk, hint, latency int) error {
		_, err := s.exec(context.Background(), insert, userID, lexemeID, skill, source, unit, correct, dk, hint, latency, ts)
		return err
	}

	for _, skill := range []string{"listen", "read", "recall", "spell"} {
		for _, source := range []string{"placement", "review", "lesson", "practice"} {
			if err := good(skill, source, nil, 1, 0, 0, 0); err != nil {
				t.Fatalf("attempt %s/%s: %v", skill, source, err)
			}
		}
	}
	if n := count(t, s, `SELECT COUNT(*) FROM attempts`); n != 16 {
		t.Fatalf("attempts = %d, want 16", n)
	}

	bad := []struct {
		name string
		err  error
	}{
		{"unknown skill", good("write", "review", nil, 1, 0, 0, 10)},
		{"old direction as skill", good("he_en", "review", nil, 1, 0, 0, 10)},
		{"empty skill", good("", "review", nil, 1, 0, 0, 10)},
		{"unknown source", good("read", "homework", nil, 1, 0, 0, 10)},
		{"empty source", good("read", "", nil, 1, 0, 0, 10)},
		{"correct 2", good("read", "review", nil, 2, 0, 0, 10)},
		{"dont_know 2", good("read", "review", nil, 1, 2, 0, 10)},
		{"hint_used 2", good("read", "review", nil, 1, 0, 2, 10)},
		{"negative latency", good("read", "review", nil, 1, 0, 0, -1)},
		{"missing unit", good("read", "lesson", 9999, 1, 0, 0, 10)},
	}
	for _, c := range bad {
		if c.err == nil {
			t.Errorf("%s: insert was accepted", c.name)
			continue
		}
		if !strings.Contains(strings.ToLower(c.err.Error()), "constraint") {
			t.Errorf("%s: error = %v, want a constraint failure", c.name, c.err)
		}
	}
	if n := count(t, s, `SELECT COUNT(*) FROM attempts`); n != 16 {
		t.Fatalf("rejected rows were stored: attempts = %d", n)
	}

	// Foreign keys: a missing learner or word is refused.
	if _, err := s.exec(context.Background(), insert, 9999, lexemeID, "read", "review", nil, 1, 0, 0, 5, ts); err == nil {
		t.Error("attempt for a missing user was accepted")
	}
	if _, err := s.exec(context.Background(), insert, userID, 9999, "read", "review", nil, 1, 0, 0, 5, ts); err == nil {
		t.Error("attempt for a missing lexeme was accepted")
	}

	// unit_id is nullable and set null when the unit goes away; the attempt stays.
	if err := good("read", "lesson", unitID, 0, 1, 1, 1200); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `DELETE FROM units WHERE id = ?`, unitID)
	if n := count(t, s, `SELECT COUNT(*) FROM attempts WHERE source = 'lesson' AND unit_id IS NULL`); n != 5 {
		// four seeded lesson attempts with a null unit plus the one whose unit was deleted.
		t.Fatalf("lesson attempts with a null unit = %d, want 5", n)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM attempts WHERE dont_know = 1 AND hint_used = 1 AND latency_ms = 1200`); n != 1 {
		t.Fatalf("attempt that outlived its unit = %d rows, want 1", n)
	}

	// A word with attempts cannot be deleted. Deleting the learner deletes their attempts.
	if _, err := s.exec(context.Background(), `DELETE FROM lexemes WHERE id = ?`, lexemeID); err == nil {
		t.Error("lexeme with attempts was deleted")
	}
	mustExec(t, s, `DELETE FROM users WHERE id = ?`, userID)
	if n := count(t, s, `SELECT COUNT(*) FROM attempts`); n != 0 {
		t.Fatalf("attempts left after user delete = %d", n)
	}
}

func TestMigrate006ExposuresRejectUnknownContext(t *testing.T) {
	s := openAt(t, t.TempDir(), nil)
	userID := insertUser(t, s, "a@example.com", "sub-a", "learner")
	lexemeID := insertLexeme(t, s, "שָׁלוֹם")
	unitID := insertUnit(t, s, "published")

	const insert = `INSERT INTO exposures (user_id, lexeme_id, context, unit_id, created_at) VALUES (?, ?, ?, ?, ?)`
	put := func(context_ string, unit any) error {
		_, err := s.exec(context.Background(), insert, userID, lexemeID, context_, unit, ts)
		return err
	}
	for _, c := range []string{"story", "dialogue", "gloss_tap", "placement_review", "lesson_intro"} {
		if err := put(c, nil); err != nil {
			t.Fatalf("context %s: %v", c, err)
		}
	}
	for _, c := range []string{"", "lesson", "listen", "Story"} {
		err := put(c, nil)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "constraint") {
			t.Errorf("context %q: error = %v, want a constraint failure", c, err)
		}
	}
	if _, err := s.exec(context.Background(), insert, 9999, lexemeID, "story", nil, ts); err == nil {
		t.Error("exposure for a missing user was accepted")
	}
	if _, err := s.exec(context.Background(), insert, userID, 9999, "story", nil, ts); err == nil {
		t.Error("exposure for a missing lexeme was accepted")
	}
	if err := put("lesson_intro", unitID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `DELETE FROM units WHERE id = ?`, unitID)
	if n := count(t, s, `SELECT COUNT(*) FROM exposures WHERE unit_id IS NULL`); n != 6 {
		t.Fatalf("exposures with a null unit = %d, want 6", n)
	}
	if _, err := s.exec(context.Background(), `DELETE FROM lexemes WHERE id = ?`, lexemeID); err == nil {
		t.Error("lexeme with exposures was deleted")
	}
	if n := count(t, s, `SELECT COUNT(*) FROM exposures`); n != 6 {
		t.Fatalf("exposures = %d, want 6", n)
	}
}

// The new tables carry the indexes the spec names, and every column has an inline comment in the SQL.
func TestMigrate006IndexesAndColumnComments(t *testing.T) {
	conn := m6At005(t)
	if err := m6Migrate(conn); err != nil {
		t.Fatal(err)
	}
	wantIdx := map[string][]string{
		"attempts_user_lexeme_skill": {"user_id", "lexeme_id", "skill", "created_at"},
		"attempts_user_created":      {"user_id", "created_at"},
		"exposures_user_lexeme":      {"user_id", "lexeme_id"},
		"cards_user_due":             {"user_id", "due"},
	}
	for name, want := range wantIdx {
		rows := m6Dump(t, conn, `SELECT name FROM pragma_index_info(?) ORDER BY seqno`, name)
		var got []string
		for _, r := range rows {
			got = append(got, strings.TrimPrefix(r[0], "string:"))
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("index %s columns = %v, want %v", name, got, want)
		}
	}
	wantCols := map[string][]string{
		"attempts":  {"id", "user_id", "lexeme_id", "skill", "activity", "source", "unit_id", "prompt", "response", "correct", "dont_know", "hint_used", "latency_ms", "created_at"},
		"exposures": {"id", "user_id", "lexeme_id", "context", "unit_id", "created_at"},
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

	body, err := fs.ReadFile(migrations.FS, "006_evidence.sql")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, stmt := range statements(string(body)) {
		if !strings.HasPrefix(strings.TrimSpace(strings.TrimLeft(stripLeadingComments(stmt), " \n")), "CREATE TABLE") {
			continue
		}
		inBody := false
		for _, line := range strings.Split(stmt, "\n") {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "CREATE TABLE") {
				inBody = true
				continue
			}
			if !inBody || trim == "" || strings.HasPrefix(trim, ")") || strings.HasPrefix(trim, "UNIQUE") || strings.HasPrefix(trim, "--") {
				continue
			}
			checked++
			if !strings.Contains(line, "-- ") {
				t.Errorf("column line has no inline comment: %q", trim)
			}
		}
	}
	// attempts 14 + exposures 6 + cards_new 12.
	if checked != 32 {
		t.Errorf("checked %d column lines, want 32", checked)
	}
}

func stripLeadingComments(s string) string {
	for {
		s = strings.TrimLeft(s, " \t\r\n")
		if !strings.HasPrefix(s, "--") {
			return s
		}
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			return ""
		}
		s = s[i+1:]
	}
}

// m6AssertAt005 checks the database is exactly where it was before 006 ran.
func m6AssertAt005(t *testing.T, conn *sql.DB, wantCards, wantLog [][]string) {
	t.Helper()
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("versions = %v, want [1 2 3 4 5]", got)
	}
	cols := columnNames(t, conn, "cards")
	if !cols["dir"] || cols["skill"] {
		t.Fatalf("cards columns = %v, want dir and no skill", cols)
	}
	for _, table := range []string{"attempts", "exposures", "cards_new"} {
		if tableExists(t, conn, table) {
			t.Errorf("table %s exists after the failed migration", table)
		}
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_temp_master WHERE name = 'review_log_keep'`); n != 0 {
		t.Error("temp table review_log_keep survived the rollback")
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE name IN ('attempts_user_lexeme_skill', 'attempts_user_created', 'exposures_user_lexeme')`); n != 0 {
		t.Error("evidence indexes exist after the failed migration")
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'cards_user_due'`); n != 1 {
		t.Error("cards_user_due index is gone")
	}
	if got := m6Dump(t, conn, m6OldCards); !reflect.DeepEqual(got, wantCards) {
		t.Fatalf("cards changed\n got: %v\nwant: %v", got, wantCards)
	}
	if got := m6Dump(t, conn, m6Log); !reflect.DeepEqual(got, wantLog) {
		t.Fatalf("review_log changed\n got: %v\nwant: %v", got, wantLog)
	}
	if got := m6One(t, conn, `PRAGMA foreign_keys`); got != 1 {
		t.Fatalf("foreign_keys = %d, want 1", got)
	}
}

// ac-4: a failure in the copy step, after the new tables exist, leaves the database at 005.
func TestMigrate006AtomicWhenCopyFails(t *testing.T) {
	conn := m6At005(t)
	wantLog := m6Dump(t, conn, m6Log)

	// A dir outside he_en and en_he can only exist when CHECKs are skipped. The mapping then gives NULL.
	mustSQL(t, conn, `PRAGMA ignore_check_constraints = ON`)
	mustSQL(t, conn, `INSERT INTO cards (user_id, lexeme_id, dir, state, due, stability, difficulty, reps, lapses)
		VALUES (3, 1, 'xx_yy', 'new', '2026-10-01T00:00:00Z', 0, 0, 0, 0)`)
	mustSQL(t, conn, `PRAGMA ignore_check_constraints = OFF`)
	wantCards := m6Dump(t, conn, m6OldCards)

	err := m6Migrate(conn)
	if err == nil {
		t.Fatal("migration 006 succeeded on a card with an unknown dir")
	}
	if !strings.Contains(err.Error(), "006_evidence.sql") || !strings.Contains(strings.ToLower(err.Error()), "constraint") {
		t.Fatalf("error = %v, want a constraint failure from 006_evidence.sql", err)
	}
	m6AssertAt005(t, conn, wantCards, wantLog)

	// The runner left the connection usable and the migration is retryable once the data is fixed.
	mustSQL(t, conn, `DELETE FROM cards WHERE dir = 'xx_yy'`)
	wantCards = m6Dump(t, conn, m6OldCards)
	if err := m6Migrate(conn); err != nil {
		t.Fatalf("retry after fixing the data: %v", err)
	}
	// The retry applies 006, then 008 (INIT-015/SPEC-004), then 009 (INIT-015/SPEC-013).
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}) {
		t.Fatalf("versions after retry = %v", got)
	}
	if got := m6Dump(t, conn, m6NewCards); !reflect.DeepEqual(got, m6MapDir(wantCards)) {
		t.Fatalf("cards after retry\n got: %v", got)
	}
}

// ac-4: a failure after cards was dropped and review_log was emptied still leaves 005 intact.
// The orphan review_log row can only be written with foreign keys off. The restore step then
// refuses it, after the DROP TABLE has already cascaded.
func TestMigrate006AtomicWhenFailureComesAfterTheDrop(t *testing.T) {
	conn := m6At005(t)

	mustSQL(t, conn, `PRAGMA foreign_keys = OFF`)
	mustSQL(t, conn, `INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days)
		VALUES (424242, 3, '2026-08-01T07:00:00Z', 0, 1)`)
	mustSQL(t, conn, `PRAGMA foreign_keys = ON`)
	wantCards := m6Dump(t, conn, m6OldCards)
	wantLog := m6Dump(t, conn, m6Log)
	if len(wantLog) == 0 {
		t.Fatal("no review_log rows")
	}

	err := m6Migrate(conn)
	if err == nil {
		t.Fatal("migration 006 succeeded with an orphan review_log row")
	}
	if !strings.Contains(err.Error(), "006_evidence.sql") || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Fatalf("error = %v, want a foreign key failure from 006_evidence.sql", err)
	}
	t.Logf("migration failed as intended: %v", err)
	// Every cascaded review_log row is back, the cards are back, and the orphan is as it was.
	m6AssertAt005(t, conn, wantCards, wantLog)
}

// The runner applies a script in one transaction, so a failing statement after a
// successful one rolls both back. This pins that for the statements 006 relies on.
func TestMigrate006RunnerRollsBackDDL(t *testing.T) {
	conn := openRaw(t)
	script := `
		CREATE TABLE keep_me (id INTEGER PRIMARY KEY);
		CREATE TEMP TABLE keep_me_temp AS SELECT * FROM keep_me;
		INSERT INTO keep_me (id) VALUES (1);
		INSERT INTO keep_me (id) VALUES (1);`
	mustSQL(t, conn, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`)
	err := apply(context.Background(), conn, "900_test.sql", 900, script, time.Now())
	if err == nil {
		t.Fatal("apply succeeded")
	}
	if tableExists(t, conn, "keep_me") {
		t.Fatal("DDL from the failed script was not rolled back")
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM sqlite_temp_master WHERE name = 'keep_me_temp'`); n != 0 {
		t.Fatal("temp table from the failed script survived")
	}
	if n := m6One(t, conn, `SELECT COUNT(*) FROM schema_migrations`); n != 0 {
		t.Fatal("failed script was recorded")
	}
}
