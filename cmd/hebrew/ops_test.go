package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// INIT-015/SPEC-025: `hebrew migrate --check` and `hebrew backup --to`. Temp files only.

const opsMarker = "marker-person@example.invalid"

// opsDB makes a migrated database in a temp DATA_DIR with one learner whose email is opsMarker.
func opsDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	path := filepath.Join(dir, "hebrew.sqlite")
	store, err := db.Open(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(context.Background(),
		`INSERT INTO users (email, google_sub, role, created_at) VALUES (?, 'sub-marker', 'learner', '2026-10-19T12:00:00Z')`, opsMarker); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func opsExec(t *testing.T, path, query string) {
	t.Helper()
	conn, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), query); err != nil {
		t.Fatal(err)
	}
}

func opsDigest(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestMigrateRequiresCheck(t *testing.T) {
	path := opsDB(t)
	before := opsDigest(t, path)
	var out, errOut bytes.Buffer
	err := migrateCommand(nil, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--check") {
		t.Fatalf("err = %v, want a refusal that names --check", err)
	}
	if out.Len() != 0 {
		t.Errorf("printed %q", out.String())
	}
	if opsDigest(t, path) != before {
		t.Error("the database changed")
	}
}

func TestMigrateCheckReportsAFullyMigratedDatabase(t *testing.T) {
	opsDB(t)
	var out, errOut bytes.Buffer
	if err := migrateCommand([]string{"--check"}, &out, &errOut); err != nil {
		t.Fatalf("err = %v\n%s%s", err, out.String(), errOut.String())
	}
	got := out.String()
	for _, want := range []string{
		"migrate-check: files=16 applied=16 pending=0\n",
		"applied: 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16\n",
		"pending: (none)\n",
		"integrity: ok\n",
		"result: ok\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
}

func TestMigrateCheckListsPendingMigrationsOnTheDeployedDatabase(t *testing.T) {
	path := opsDB(t)
	// The production database: 006 and 008 to 011 not recorded.
	opsExec(t, path, `DELETE FROM schema_migrations WHERE version IN (6, 8, 9, 10, 11, 12, 13, 14, 15)`)
	var out, errOut bytes.Buffer
	if err := migrateCommand([]string{"--check"}, &out, &errOut); err != nil {
		t.Fatalf("pending migrations are not a failure: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"applied=7 pending=9",
		"applied: 1 2 3 4 5 7 16\n",
		"pending: 006_evidence.sql 008_learner_model.sql 009_confusion_other.sql 010_daily_job.sql 011_placement_v3.sql 012_attempt_list_index.sql 013_patterns.sql 014_mistakes_verbs.sql 015_attempt_source_self.sql\n",
		"pending-below-applied: 6 ",
		"result: ok\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output lacks %q:\n%s", want, got)
		}
	}
}

func TestMigrateCheckFailsOnAVersionWithNoFile(t *testing.T) {
	path := opsDB(t)
	opsExec(t, path, `INSERT INTO schema_migrations (version, applied_at) VALUES (42, '2026-10-19T12:00:00Z')`)
	var out, errOut bytes.Buffer
	err := migrateCommand([]string{"--check"}, &out, &errOut)
	if err == nil {
		t.Fatalf("exit status was 0:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "problem: version 42 is recorded") || !strings.Contains(out.String(), "result: problems=1") {
		t.Errorf("output:\n%s", out.String())
	}
}

func TestMigrateCheckPrintsNoRowData(t *testing.T) {
	path := opsDB(t)
	opsExec(t, path, `DELETE FROM schema_migrations WHERE version = 11`)
	var out, errOut bytes.Buffer
	if err := migrateCommand([]string{"--check", "--db", path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	all := out.String() + errOut.String()
	for _, secret := range []string{opsMarker, "sub-marker", "learner"} {
		if strings.Contains(all, secret) {
			t.Errorf("output contains row data %q:\n%s", secret, all)
		}
	}
}

func TestMigrateCheckDoesNotChangeTheDatabaseOrCreateOne(t *testing.T) {
	path := opsDB(t)
	opsExec(t, path, `DELETE FROM schema_migrations WHERE version IN (8, 9, 10, 11)`)
	before := opsDigest(t, path)
	var out, errOut bytes.Buffer
	if err := migrateCommand([]string{"--check"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if opsDigest(t, path) != before {
		t.Error("migrate --check changed the database")
	}

	missing := filepath.Join(t.TempDir(), "absent.sqlite")
	out.Reset()
	if err := migrateCommand([]string{"--check", "--db", missing}, &out, &errOut); err == nil {
		t.Error("a missing file was accepted")
	}
	if _, err := os.Stat(missing); err == nil {
		t.Error("migrate --check created the file")
	}
}

func TestMigrateCheckNeedsADatabaseLocation(t *testing.T) {
	t.Setenv("DATA_DIR", "")
	var out, errOut bytes.Buffer
	if err := migrateCommand([]string{"--check"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "DATA_DIR") {
		t.Fatalf("err = %v", err)
	}
}

var backupLine = regexp.MustCompile(`^bytes=([0-9]+) sha256=([0-9a-f]{64})\n$`)

func TestBackupPrintsBytesAndDigestOnly(t *testing.T) {
	src := opsDB(t)
	dst := filepath.Join(t.TempDir(), "b.sqlite")
	var out, errOut bytes.Buffer
	if err := backupCommand([]string{"--to", dst}, &out, &errOut); err != nil {
		t.Fatalf("err = %v\n%s", err, errOut.String())
	}
	m := backupLine.FindStringSubmatch(out.String())
	if m == nil {
		t.Fatalf("output %q is not `bytes=N sha256=HEX`", out.String())
	}
	if m[2] != opsDigest(t, dst) {
		t.Errorf("printed sha256 %s, file is %s", m[2], opsDigest(t, dst))
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if m[1] != strconv.FormatInt(info.Size(), 10) {
		t.Errorf("printed bytes %s, file is %d", m[1], info.Size())
	}
	for _, leak := range []string{opsMarker, filepath.Base(dst), filepath.Base(src)} {
		if strings.Contains(out.String()+errOut.String(), leak) {
			t.Errorf("output contains %q", leak)
		}
	}
}

func TestBackupRefusesToOverwrite(t *testing.T) {
	opsDB(t)
	dst := filepath.Join(t.TempDir(), "b.sqlite")
	var out, errOut bytes.Buffer
	if err := backupCommand([]string{"--to", dst}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	first := opsDigest(t, dst)
	out.Reset()
	if err := backupCommand([]string{"--to", dst}, &out, &errOut); err == nil {
		t.Fatal("the second backup to one path succeeded")
	}
	if out.Len() != 0 {
		t.Errorf("a refused backup printed %q", out.String())
	}
	if opsDigest(t, dst) != first {
		t.Error("the first backup was overwritten")
	}
}

func TestBackupRequiresTo(t *testing.T) {
	opsDB(t)
	var out, errOut bytes.Buffer
	if err := backupCommand(nil, &out, &errOut); err == nil || !strings.Contains(err.Error(), "--to") {
		t.Fatalf("err = %v", err)
	}
	if err := backupCommand([]string{"--to", filepath.Join(t.TempDir(), "x"), "extra"}, &out, &errOut); err == nil {
		t.Fatal("a stray argument was accepted")
	}
}

func TestBackupDoesNotChangeTheSource(t *testing.T) {
	src := opsDB(t)
	opsExec(t, src, `DELETE FROM schema_migrations WHERE version IN (9, 10, 11, 12)`)
	before := opsDigest(t, src)
	var out, errOut bytes.Buffer
	if err := backupCommand([]string{"--to", filepath.Join(t.TempDir(), "b.sqlite")}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if opsDigest(t, src) != before {
		t.Error("backup changed the source database")
	}
	if got := countRowsAt(t, src, `SELECT COUNT(*) FROM schema_migrations`); got != 12 {
		t.Errorf("schema_migrations has %d rows, backup applied a migration", got)
	}
}

func countRowsAt(t *testing.T, path, query string) int {
	t.Helper()
	conn, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var n int
	if err := conn.QueryRow(query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Both commands are reachable from the binary's own dispatch.
func TestOperatorCommandsAreDispatched(t *testing.T) {
	opsDB(t)
	if err := run([]string{"migrate"}); err == nil || !strings.Contains(err.Error(), "--check") {
		t.Errorf("run(migrate) = %v, want the --check refusal, not unknown command", err)
	}
	if err := run([]string{"backup"}); err == nil || !strings.Contains(err.Error(), "--to") {
		t.Errorf("run(backup) = %v, want the --to refusal, not unknown command", err)
	}
}
