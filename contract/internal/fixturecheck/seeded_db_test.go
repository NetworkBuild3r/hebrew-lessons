package fixturecheck

import (
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestSeededDatabaseOmitsSecrets opens the committed fixture read-only.
// Table and column names come from sqlite_master and pragma_table_info.
func TestSeededDatabaseOmitsSecrets(t *testing.T) {
	root := moduleRoot(t)
	dbPath := filepath.Join(root, "fixtures", "db", "seeded.sqlite")
	for _, side := range []string{dbPath + "-wal", dbPath + "-shm"} {
		if _, err := os.Stat(side); err == nil {
			t.Errorf("sidecar must not sit next to the fixture: %s", filepath.Base(side))
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	File(t, dbPath)

	// immutable keeps a WAL database from creating -wal/-shm sidecars on open.
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro&immutable=1")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA query_only = ON"); err != nil {
		t.Fatal(err)
	}

	tables := tableColumns(t, db)
	if _, ok := tables["sessions"]; !ok {
		t.Fatal("schema has no sessions table")
	}
	if _, ok := tables["users"]; !ok {
		t.Fatal("schema has no users table")
	}

	var sessions int
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Fatalf("sessions rows = %d, want 0", sessions)
	}

	emailCols := emailColumns(tables["users"])
	if len(emailCols) == 0 {
		t.Fatal("users table has no email column to check")
	}
	assertUsers(t, db, emailCols)

	holders := oauthHoldingTables(tables)
	for _, name := range holders {
		var n int
		q := "SELECT COUNT(*) FROM " + quoteIdent(name)
		if err := db.QueryRow(q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s holds %d oauth client, code, or token rows", name, n)
		}
	}

	for _, side := range []string{dbPath + "-wal", dbPath + "-shm"} {
		if _, err := os.Stat(side); err == nil {
			t.Errorf("opening the fixture created %s", filepath.Base(side))
		}
	}
}

func assertUsers(t *testing.T, db *sql.DB, emailCols []string) {
	t.Helper()
	cols := append([]string{"username", "provider_subject"}, emailCols...)
	quoted := make([]string, len(cols))
	for i, name := range cols {
		quoted[i] = quoteIdent(name)
	}
	rows, err := db.Query("SELECT " + strings.Join(quoted, ", ") + " FROM users ORDER BY username")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		dest := make([]any, len(cols))
		for i := range vals {
			dest[i] = &vals[i]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatal(err)
		}
		if !vals[0].Valid || vals[0].String == "" {
			t.Fatal("user row has no username")
		}
		got = append(got, vals[0].String)
		for i := 1; i < len(vals); i++ {
			if vals[i].Valid && strings.TrimSpace(vals[i].String) != "" {
				t.Errorf("user %s column %s = %q", vals[0].String, cols[i], vals[i].String)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []string{"contract-admin", "learner", "learner-off"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("users = %q, want %q", got, want)
	}
}

func tableColumns(t *testing.T, db *sql.DB) map[string][]string {
	t.Helper()
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string][]string{}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		info, err := db.Query("SELECT name FROM pragma_table_info(" + quoteIdent(name) + ")")
		if err != nil {
			t.Fatal(err)
		}
		var cols []string
		for info.Next() {
			var col string
			if err := info.Scan(&col); err != nil {
				info.Close()
				t.Fatal(err)
			}
			cols = append(cols, col)
		}
		if err := info.Err(); err != nil {
			info.Close()
			t.Fatal(err)
		}
		info.Close()
		out[name] = cols
	}
	return out
}

func emailColumns(cols []string) []string {
	var out []string
	for _, col := range cols {
		if strings.Contains(strings.ToLower(col), "email") {
			out = append(out, col)
		}
	}
	sort.Strings(out)
	return out
}

// oauthColumn is a column that stores an OAuth client, authorization code,
// or OAuth token. sessions.token_hash is a login session, checked separately.
var oauthColumn = regexp.MustCompile(`(?i)^(client_id|client_secret|authorization_code|auth_code|refresh_token|access_token|code_challenge|code_verifier)$`)

var oauthTable = regexp.MustCompile(`(?i)oauth`)

func oauthHoldingTables(tables map[string][]string) []string {
	var out []string
	for name, cols := range tables {
		if name == "sessions" {
			continue
		}
		if oauthTable.MatchString(name) {
			out = append(out, name)
			continue
		}
		for _, col := range cols {
			if oauthColumn.MatchString(col) {
				out = append(out, name)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func quoteIdent(name string) string {
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(name) {
		panic("unexpected sql ident " + name)
	}
	return `"` + name + `"`
}
