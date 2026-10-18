package db

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestExpectedMigrationsMatchNodeFiles(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "server", "db", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	namePattern := regexp.MustCompile(`^\d{3,}-.+\.js$`)
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !namePattern.MatchString(entry.Name()) {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if len(names) != len(ExpectedMigrations) {
		t.Fatalf("migration files: got %v want %d names", names, len(ExpectedMigrations))
	}
	for i, name := range names {
		version, err := strconv.Atoi(strings.SplitN(name, "-", 2)[0])
		if err != nil {
			t.Fatal(err)
		}
		if ExpectedMigrations[i].Version != version || ExpectedMigrations[i].Name != name {
			t.Fatalf("index %d: got %s want %+v", i, name, ExpectedMigrations[i])
		}
	}
}

func TestOpenReadOnlyMigratedDatabase(t *testing.T) {
	file := applyNodeDB(t, "apply")
	conn := openReadOnly(t, file)
	defer conn.Close()

	ctx := context.Background()
	var foreignKeys, busy int
	var journal string
	if err := conn.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 || busy != 5000 || !strings.EqualFold(journal, "wal") {
		t.Fatalf("pragmas: foreign_keys=%d busy_timeout=%d journal_mode=%s", foreignKeys, busy, journal)
	}

	_, err := conn.ExecContext(ctx, `CREATE TABLE go_should_not_write (id INTEGER)`)
	if err == nil {
		t.Fatal("read-only connection accepted a write")
	}

	ledger, err := CheckLedger(ctx, conn)
	if err != nil {
		t.Fatal(err)
	}
	if !ledger.OK {
		t.Fatalf("ledger: %+v", ledger)
	}
	if len(ledger.Versions) != len(ExpectedMigrations) {
		t.Fatalf("versions: %d", len(ledger.Versions))
	}
	for i, row := range ledger.Versions {
		if row != ExpectedMigrations[i] {
			t.Fatalf("row %d: got %+v want %+v", i, row, ExpectedMigrations[i])
		}
	}
}

func TestLedgerMismatches(t *testing.T) {
	cases := []struct {
		mode string
		want string
	}{
		{mode: "missing", want: "missing version 4 (004-fsrs-review-state.js)"},
		{mode: "extra", want: "unexpected version 12 (012-extra.js)"},
		{mode: "wrong-name", want: `version 3 name is "003-not-the-real-name.js", want "003-media-and-ai-planning.js"`},
		{mode: "empty", want: "schema_migrations table is missing"},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			file := applyNodeDB(t, tc.mode)
			conn := openReadOnly(t, file)
			defer conn.Close()
			ledger, err := CheckLedger(context.Background(), conn)
			if err == nil {
				t.Fatal("expected ledger error")
			}
			if ledger.OK {
				t.Fatalf("ledger ok: %+v", ledger)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.want)
			}
			if ledger.Error != err.Error() {
				t.Fatalf("ledger error %q != %q", ledger.Error, err.Error())
			}
		})
	}
}

func TestOpenReadOnlyDoesNotCreateMissingFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "hebrew.sqlite")
	_, err := OpenReadOnly(file)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, statErr := os.Stat(file); !os.IsNotExist(statErr) {
		t.Fatalf("missing file was created: %v", statErr)
	}
}

func TestCheckLedgerNilConnection(t *testing.T) {
	ledger, err := CheckLedger(context.Background(), nil)
	if err == nil || ledger.OK {
		t.Fatal("expected nil connection error")
	}
}

func openReadOnly(t *testing.T, file string) *sql.DB {
	t.Helper()
	conn, err := OpenReadOnly(file)
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func applyNodeDB(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("node", filepath.Join("testdata", "apply-migrations.js"), mode, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node %s: %v\n%s", mode, err, out)
	}
	file := filepath.Join(dir, "hebrew.sqlite")
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("migrated db missing: %v\n%s", err, out)
	}
	return file
}
