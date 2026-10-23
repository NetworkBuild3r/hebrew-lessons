package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

// INIT-015/SPEC-025: CheckMigrations (hebrew migrate --check) and Backup (hebrew backup).
// Every database here is a temp file. None of these tests touches a real database.

func fileDigest(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// migratedFile is a closed database file with every embedded migration applied and one user.
func migratedFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(context.Background(), filepath.Join(dir, "hebrew.sqlite"), fixedClock{time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	insertUser(t, s, "marker-person@example.invalid", "sub-marker", "learner")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "hebrew.sqlite")
}

func rawExec(t *testing.T, path, query string, args ...any) {
	t.Helper()
	conn, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetMaxOpenConns(1)
	if _, err := conn.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func rawCount(t *testing.T, path, query string) int {
	t.Helper()
	conn, err := openReadOnly(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var n int
	if err := conn.QueryRowContext(context.Background(), query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func versions(files []MigrationFile) []int {
	var out []int
	for _, f := range files {
		out = append(out, f.Version)
	}
	return out
}

func TestCheckMigrationsOnAFullyMigratedDatabase(t *testing.T) {
	path := migratedFile(t)
	rep, err := CheckMigrations(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	if !reflect.DeepEqual(rep.Applied, want) {
		t.Errorf("Applied = %v, want %v", rep.Applied, want)
	}
	if len(rep.Pending) != 0 || len(rep.OutOfOrder) != 0 || len(rep.Unknown) != 0 || len(rep.Duplicates) != 0 {
		t.Errorf("pending=%v outOfOrder=%v unknown=%v dup=%v, want none", rep.Pending, rep.OutOfOrder, rep.Unknown, rep.Duplicates)
	}
	if rep.Integrity != "ok" {
		t.Errorf("Integrity = %q, want ok", rep.Integrity)
	}
	if got := rep.Problems(); len(got) != 0 {
		t.Errorf("Problems = %v, want none", got)
	}
	if !reflect.DeepEqual(versions(rep.Files), want) {
		t.Errorf("Files = %v", versions(rep.Files))
	}
}

// The deployed production database: versions 1 to 5 and 7, built the way the migration test
// builds it. The report must say 6, 8, 9, 10, 11, 12 are pending and that 6 is below 7.
func TestCheckMigrationsOnTheDeployedV3Database(t *testing.T) {
	conn := m6At005(t)
	body, err := fs.ReadFile(migrations.FS, "007_oauth_client_issued.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(context.Background(), conn, "007_oauth_client_issued.sql", 7, string(body), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	var path string
	if err := conn.QueryRowContext(context.Background(), `SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&path); err != nil {
		t.Fatal(err)
	}

	rep, err := CheckMigrations(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{1, 2, 3, 4, 5, 7}; !reflect.DeepEqual(rep.Applied, want) {
		t.Errorf("Applied = %v, want %v", rep.Applied, want)
	}
	if want := []int{6, 8, 9, 10, 11, 12, 13, 14, 15}; !reflect.DeepEqual(versions(rep.Pending), want) {
		t.Errorf("Pending = %v, want %v", versions(rep.Pending), want)
	}
	if want := []int{6}; !reflect.DeepEqual(rep.OutOfOrder, want) {
		t.Errorf("OutOfOrder = %v, want %v", rep.OutOfOrder, want)
	}
	if got := rep.Problems(); len(got) != 0 {
		t.Errorf("a pending migration is not a problem, got %v", got)
	}
	// The check applied nothing.
	if got := m6Applied(t, conn); !reflect.DeepEqual(got, []int{1, 2, 3, 4, 5, 7}) {
		t.Errorf("versions after the check = %v", got)
	}
}

func TestCheckMigrationsFlagsAVersionThisBuildHasNoFileFor(t *testing.T) {
	path := migratedFile(t)
	rawExec(t, path, `INSERT INTO schema_migrations (version, applied_at) VALUES (99, '2026-10-19T12:00:00Z')`)
	rep, err := CheckMigrations(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rep.Unknown, []int{99}) {
		t.Fatalf("Unknown = %v, want [99]", rep.Unknown)
	}
	probs := rep.Problems()
	if len(probs) != 1 || !strings.Contains(probs[0], "99") {
		t.Fatalf("Problems = %v, want one that names 99", probs)
	}
}

func TestCheckMigrationsFlagsTwoFilesWithOneVersion(t *testing.T) {
	path := migratedFile(t)
	files := fstest.MapFS{
		"001_init.sql":  {Data: []byte("SELECT 1;")},
		"001_other.sql": {Data: []byte("SELECT 1;")},
	}
	rep, err := checkMigrations(context.Background(), path, files)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Duplicates) != 1 || !strings.Contains(rep.Duplicates[0], "001_init.sql") || !strings.Contains(rep.Duplicates[0], "001_other.sql") {
		t.Fatalf("Duplicates = %v", rep.Duplicates)
	}
	found := false
	for _, p := range rep.Problems() {
		if strings.Contains(p, "used by") {
			found = true
		}
	}
	if !found {
		t.Fatalf("a duplicate version is a problem, got %v", rep.Problems())
	}
}

func TestCheckMigrationsOnAnEmptyFileSaysEverythingIsPending(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hebrew.sqlite")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	rep, err := CheckMigrations(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Applied) != 0 || len(rep.Pending) != len(rep.Files) || len(rep.Files) == 0 {
		t.Fatalf("applied=%v pending=%d files=%d", rep.Applied, len(rep.Pending), len(rep.Files))
	}
	if got := rep.Problems(); len(got) != 0 {
		t.Fatalf("Problems = %v", got)
	}
}

func TestCheckMigrationsIsReadOnly(t *testing.T) {
	path := migratedFile(t)
	dir := filepath.Dir(path)
	before := fileDigest(t, path)
	beforeNames := dirNames(t, dir)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CheckMigrations(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if got := fileDigest(t, path); got != before {
		t.Error("CheckMigrations changed the database file")
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(info.ModTime()) {
		t.Error("CheckMigrations changed the modification time")
	}
	// SQLite opens a WAL database read-only through -shm and -wal files. They may appear next to
	// the file. They hold no data: the WAL stays empty, and nothing else is added.
	for _, name := range dirNames(t, dir) {
		switch name {
		case "hebrew.sqlite", "hebrew.sqlite-shm":
		case "hebrew.sqlite-wal":
			if wi, err := os.Stat(filepath.Join(dir, name)); err != nil || wi.Size() != 0 {
				t.Errorf("the WAL file is not empty after a read-only check: %v %v", wi, err)
			}
		default:
			t.Errorf("unexpected file %s (before %v)", name, beforeNames)
		}
	}
}

func TestCheckMigrationsDoesNotCreateAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "hebrew.sqlite")
	if _, err := CheckMigrations(context.Background(), path); err == nil {
		t.Fatal("a missing file was accepted")
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("CheckMigrations created the file")
	}
	if _, err := os.Stat(filepath.Dir(path)); err == nil {
		t.Fatal("CheckMigrations created the directory")
	}
}

func TestReadOnlyConnectionRefusesWrites(t *testing.T) {
	path := migratedFile(t)
	conn, err := openReadOnly(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, err = conn.ExecContext(context.Background(), `DELETE FROM schema_migrations`)
	if err == nil {
		t.Fatal("a write on the read-only connection succeeded")
	}
	if rawCount(t, path, `SELECT COUNT(*) FROM schema_migrations`) != 15 {
		t.Fatal("schema_migrations changed")
	}
}

// Damage a page of a real index, and the report must say the database is not intact.
func TestCheckMigrationsReportsAnIntegrityFailure(t *testing.T) {
	path := migratedFile(t)
	conn, err := openReadOnly(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	var root, pageSize int
	if err := conn.QueryRowContext(context.Background(),
		`SELECT rootpage FROM sqlite_master WHERE type = 'index' AND name = 'idx_attempts_user_lexeme_skill_at'`).Scan(&root); err != nil {
		// Fall back to any real index with a root page.
		if err := conn.QueryRowContext(context.Background(),
			`SELECT rootpage FROM sqlite_master WHERE type = 'index' AND name NOT LIKE 'sqlite_autoindex%' ORDER BY name LIMIT 1`).Scan(&root); err != nil {
			t.Fatal(err)
		}
	}
	if err := conn.QueryRowContext(context.Background(), `PRAGMA page_size`).Scan(&pageSize); err != nil {
		t.Fatal(err)
	}
	conn.Close()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	start := (root - 1) * pageSize
	for i := start; i < start+pageSize && i < len(b); i++ {
		b[i] = 0xFF
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}

	rep, err := CheckMigrations(context.Background(), path)
	if err != nil {
		// SQLite may refuse the damaged page outright. That is also "not ok".
		t.Logf("sqlite refused the damaged file: %v", err)
		return
	}
	if rep.Integrity == "ok" {
		t.Fatalf("Integrity = ok for a damaged database")
	}
	if len(rep.Problems()) == 0 {
		t.Fatal("a failed integrity_check is a problem")
	}
	if strings.Contains(rep.Integrity, "page") || strings.Contains(rep.Integrity, "index") {
		t.Fatalf("Integrity %q keeps the SQLite message", rep.Integrity)
	}
}

// ---- Backup ----

func TestBackupCopiesTheDatabaseAndReportsItsDigest(t *testing.T) {
	src := migratedFile(t)
	dst := filepath.Join(t.TempDir(), "out", "backup.sqlite")
	res, err := Backup(context.Background(), src, dst)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if res.Bytes != info.Size() || res.Bytes == 0 {
		t.Errorf("Bytes = %d, file is %d", res.Bytes, info.Size())
	}
	if res.SHA256 != fileDigest(t, dst) {
		t.Errorf("SHA256 = %s, want %s", res.SHA256, fileDigest(t, dst))
	}
	if runtime.GOOS != "windows" {
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Errorf("mode = %o, want 600", mode)
		}
		dirInfo, err := os.Stat(filepath.Dir(dst))
		if err != nil {
			t.Fatal(err)
		}
		if mode := dirInfo.Mode().Perm(); mode != 0o700 {
			t.Errorf("created directory mode = %o, want 700", mode)
		}
	}
	// The copy is a working database with the same data.
	if got := rawCount(t, dst, `SELECT COUNT(*) FROM users WHERE email = 'marker-person@example.invalid'`); got != 1 {
		t.Errorf("users in the backup = %d, want 1", got)
	}
	if got := rawCount(t, dst, `SELECT COUNT(*) FROM schema_migrations`); got != 15 {
		t.Errorf("schema_migrations in the backup = %d, want 15", got)
	}
	rep, err := CheckMigrations(context.Background(), dst)
	if err != nil || rep.Integrity != "ok" || len(rep.Problems()) != 0 {
		t.Errorf("the backup fails its own check: %v %+v", err, rep)
	}
}

func TestBackupRefusesToOverwrite(t *testing.T) {
	src := migratedFile(t)
	dst := filepath.Join(t.TempDir(), "keep.sqlite")
	if err := os.WriteFile(dst, []byte("do not touch"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Backup(context.Background(), src, dst)
	if !errors.Is(err, ErrBackupExists) {
		t.Fatalf("err = %v, want ErrBackupExists", err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "do not touch" {
		t.Fatalf("the existing file was changed: %q", b)
	}
	// A second backup to the same path, after a good first one, is also refused.
	good := filepath.Join(t.TempDir(), "good.sqlite")
	if _, err := Backup(context.Background(), src, good); err != nil {
		t.Fatal(err)
	}
	digest := fileDigest(t, good)
	if _, err := Backup(context.Background(), src, good); !errors.Is(err, ErrBackupExists) {
		t.Fatalf("second backup err = %v, want ErrBackupExists", err)
	}
	if fileDigest(t, good) != digest {
		t.Fatal("the first backup was changed by the second attempt")
	}
}

func TestBackupRefusesToWriteOverItsSource(t *testing.T) {
	src := migratedFile(t)
	before := fileDigest(t, src)
	if _, err := Backup(context.Background(), src, src); !errors.Is(err, ErrBackupExists) {
		t.Fatalf("err = %v, want ErrBackupExists", err)
	}
	if fileDigest(t, src) != before {
		t.Fatal("the source changed")
	}
}

func TestBackupDoesNotFollowASymlinkAtTheDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	src := migratedFile(t)
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	if err := os.WriteFile(victim, []byte("victim"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.sqlite")
	if err := os.Symlink(victim, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Backup(context.Background(), src, link); !errors.Is(err, ErrBackupExists) {
		t.Fatalf("err = %v, want ErrBackupExists", err)
	}
	if b, _ := os.ReadFile(victim); string(b) != "victim" {
		t.Fatalf("the symlink target was written: %q", b)
	}
}

func TestBackupOfAMissingSourceCreatesNothing(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "new", "backup.sqlite")
	if _, err := Backup(context.Background(), filepath.Join(dir, "absent.sqlite"), dst); err == nil {
		t.Fatal("a missing source was accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "new")); err == nil {
		t.Fatal("Backup created the destination directory for a source that does not exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "absent.sqlite")); err == nil {
		t.Fatal("Backup created the source")
	}
}

func TestBackupOfANonDatabaseLeavesNoFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "notadb.sqlite")
	if err := os.WriteFile(src, []byte(strings.Repeat("this is not a sqlite file ", 100)), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "backup.sqlite")
	if _, err := Backup(context.Background(), src, dst); err == nil {
		t.Fatal("a file that is not a database was backed up")
	}
	if _, err := os.Stat(dst); err == nil {
		t.Fatal("a failed backup left its file behind")
	}
}

// The app holds the database open in WAL mode, and the newest rows can still be in the WAL file.
// A plain copy of the main file would miss them. VACUUM INTO does not.
func TestBackupWhileTheAppHasTheDatabaseOpenKeepsUncheckpointedRows(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(context.Background(), filepath.Join(dir, "hebrew.sqlite"), fixedClock{time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i := 0; i < 40; i++ {
		insertUser(t, s, "live-"+string(rune('a'+i%26))+string(rune('a'+i/26))+"@example.invalid", "sub-live-"+string(rune('a'+i%26))+string(rune('a'+i/26)), "learner")
	}
	wal := filepath.Join(dir, "hebrew.sqlite-wal")
	if info, err := os.Stat(wal); err != nil || info.Size() == 0 {
		t.Fatalf("the test needs rows in the WAL file, stat = %v %v", info, err)
	}

	dst := filepath.Join(t.TempDir(), "backup.sqlite")
	if _, err := Backup(context.Background(), filepath.Join(dir, "hebrew.sqlite"), dst); err != nil {
		t.Fatal(err)
	}
	if got := rawCount(t, dst, `SELECT COUNT(*) FROM users WHERE email LIKE 'live-%'`); got != 40 {
		t.Fatalf("the backup holds %d of 40 rows written before it ran", got)
	}
	// The running app is not disturbed: it can still write.
	insertUser(t, s, "after@example.invalid", "sub-after", "learner")
}

// Backup applies no migration. The source is the deployed production state, and it stays so.
func TestBackupAppliesNoMigration(t *testing.T) {
	conn := m6At005(t)
	body, err := fs.ReadFile(migrations.FS, "007_oauth_client_issued.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(context.Background(), conn, "007_oauth_client_issued.sql", 7, string(body), time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	var src string
	if err := conn.QueryRowContext(context.Background(), `SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&src); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "backup.sqlite")
	if _, err := Backup(context.Background(), src, dst); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{src, dst} {
		rep, err := CheckMigrations(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		if want := []int{1, 2, 3, 4, 5, 7}; !reflect.DeepEqual(rep.Applied, want) {
			t.Errorf("%s: Applied = %v, want %v", filepath.Base(path), rep.Applied, want)
		}
		if got := rawCount(t, path, `SELECT COUNT(*) FROM pragma_table_info('cards') WHERE name = 'dir'`); got != 1 {
			t.Errorf("%s: cards.dir is gone, so a migration ran", filepath.Base(path))
		}
	}
}

// A copy that fails integrity_check is removed, and Backup returns an error and no digest.
func TestBackupRemovesACopyThatFailsItsIntegrityCheck(t *testing.T) {
	src := migratedFile(t)
	dst := filepath.Join(t.TempDir(), "backup.sqlite")
	orig := verifyFile
	verifyFile = func(context.Context, string) (string, error) { return "3 problems", nil }
	t.Cleanup(func() { verifyFile = orig })

	res, err := Backup(context.Background(), src, dst)
	if err == nil || !strings.Contains(err.Error(), "integrity_check") {
		t.Fatalf("err = %v, want an integrity_check failure", err)
	}
	if res.SHA256 != "" || res.Bytes != 0 {
		t.Errorf("a failed backup returned a result: %+v", res)
	}
	if _, err := os.Stat(dst); err == nil {
		t.Fatal("the failed copy was left on disk")
	}
}
