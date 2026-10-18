// Package sqlite reads a hebrew.sqlite file. It does not migrate or write
// through the server. modernc.org/sqlite is pure Go, so the harness stays cgo-free.
package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/normalize"
	_ "modernc.org/sqlite"
)

// Dump reads sqlite_master, schema_migrations, user_version, and journal_mode.
// The text is what golden/db/schema.sql compares. applied_at is masked.
func Dump(path string) (string, error) {
	db, err := openReadOnly(path)
	if err != nil {
		return "", err
	}
	defer db.Close()

	journal, err := pragmaText(db, "journal_mode")
	if err != nil {
		return "", err
	}
	userVersion, err := pragmaText(db, "user_version")
	if err != nil {
		return "", err
	}
	migrations, err := migrationRows(db)
	if err != nil {
		return "", err
	}
	objects, err := masterRows(db)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "journal_mode: %s\n", journal)
	fmt.Fprintf(&b, "user_version: %s\n", userVersion)
	b.WriteString("\n# schema_migrations\n")
	for _, row := range migrations {
		fmt.Fprintf(&b, "%d %s %s\n", row.version, row.name, row.applied)
	}
	b.WriteString("\n# sqlite_master\n")
	for _, obj := range objects {
		fmt.Fprintf(&b, "--- %s %s", obj.typ, obj.name)
		if obj.tbl != "" && obj.tbl != obj.name {
			fmt.Fprintf(&b, " on %s", obj.tbl)
		}
		b.WriteByte('\n')
		if obj.sql != "" {
			b.WriteString(obj.sql)
			if !strings.HasSuffix(obj.sql, "\n") {
				b.WriteByte('\n')
			}
		}
	}
	return b.String(), nil
}

// Snapshot copies a live database into dest as one file. The server may have
// the source open in WAL mode. The copy is checkpointed so the committed
// fixture does not need a sidecar -wal file.
func Snapshot(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", rwDSN(src))
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA busy_timeout = 10000"); err != nil {
		return err
	}
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return err
	}
	tmp := dest + ".tmp"
	os.Remove(tmp)
	quoted := strings.ReplaceAll(tmp, "'", "''")
	if _, err := db.Exec("VACUUM INTO '" + quoted + "'"); err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}
	// VACUUM INTO starts a new file at the default journal mode. Match the
	// server, which sets WAL on open, then fold the sidecar back in.
	copy, err := sql.Open("sqlite", rwDSN(tmp))
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if _, err := copy.Exec("PRAGMA journal_mode = WAL"); err != nil {
		copy.Close()
		os.Remove(tmp)
		return err
	}
	if _, err := copy.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		copy.Close()
		os.Remove(tmp)
		return err
	}
	copy.Close()
	os.Remove(tmp + "-wal")
	os.Remove(tmp + "-shm")
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func openReadOnly(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", roDSN(path))
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func roDSN(path string) string {
	return "file:" + filepath.ToSlash(path) + "?mode=ro"
}

func rwDSN(path string) string {
	return "file:" + filepath.ToSlash(path) + "?mode=rw"
}

func pragmaText(db *sql.DB, name string) (string, error) {
	var v any
	if err := db.QueryRow("PRAGMA " + name).Scan(&v); err != nil {
		return "", err
	}
	switch n := v.(type) {
	case string:
		return n, nil
	case int64:
		return strconv.FormatInt(n, 10), nil
	case []byte:
		return string(n), nil
	default:
		return fmt.Sprint(n), nil
	}
}

type migration struct {
	version int
	name    string
	applied string
}

func migrationRows(db *sql.DB) ([]migration, error) {
	rows, err := db.Query("SELECT version, name, applied_at FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []migration
	for rows.Next() {
		var row migration
		if err := rows.Scan(&row.version, &row.name, &row.applied); err != nil {
			return nil, err
		}
		row.applied = normalize.MaskString(row.applied)
		out = append(out, row)
	}
	return out, rows.Err()
}

type object struct {
	typ  string
	name string
	tbl  string
	sql  string
}

func masterRows(db *sql.DB) ([]object, error) {
	rows, err := db.Query("SELECT type, name, tbl_name, sql FROM sqlite_master")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []object
	for rows.Next() {
		var obj object
		var sqlText sql.NullString
		if err := rows.Scan(&obj.typ, &obj.name, &obj.tbl, &sqlText); err != nil {
			return nil, err
		}
		if sqlText.Valid {
			obj.sql = normalizeSQL(sqlText.String)
		}
		out = append(out, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if typeRank(out[i].typ) != typeRank(out[j].typ) {
			return typeRank(out[i].typ) < typeRank(out[j].typ)
		}
		if out[i].name != out[j].name {
			return out[i].name < out[j].name
		}
		return out[i].tbl < out[j].tbl
	})
	return out, nil
}

func typeRank(typ string) int {
	switch typ {
	case "table":
		return 0
	case "index":
		return 1
	case "trigger":
		return 2
	case "view":
		return 3
	default:
		return 9
	}
}

func normalizeSQL(sqlText string) string {
	sqlText = strings.ReplaceAll(sqlText, "\r\n", "\n")
	lines := strings.Split(sqlText, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}
