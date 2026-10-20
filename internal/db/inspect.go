package db

// Read-only inspection and backup of a database file (INIT-015/SPEC-025).
//
// Nothing here applies a migration or writes to the database it reads. The runbook runs these
// against a production file, so every function opens the source with mode=ro and returns no
// row data: counts, version numbers, migration file names, and a digest only.

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/migrations"
)

// MigrationFile is one embedded migration, named by file and numbered by its prefix.
type MigrationFile struct {
	Version int
	Name    string
}

// MigrationReport is what CheckMigrations found. It holds versions and file names. It never
// holds a row of learner data.
type MigrationReport struct {
	// Files is every migration file in this build, in version order.
	Files []MigrationFile
	// Applied is the versions recorded in schema_migrations, ascending.
	Applied []int
	// Pending is the files whose version is not recorded. Migrate applies them at start.
	Pending []MigrationFile
	// OutOfOrder is the pending versions that are lower than the highest applied version.
	// That is expected on a database that recorded 007_oauth_client_issued before 006 existed:
	// the runner applies by number, not by order, so these still run.
	OutOfOrder []int
	// Unknown is the recorded versions that have no file in this build. A database that a
	// newer build migrated looks like this to an older one.
	Unknown []int
	// Duplicates describes two files that share a version. Migrate refuses to start on one.
	Duplicates []string
	// Integrity is "ok" or a count of problems. The messages are not kept.
	Integrity string
}

// Problems lists what makes the check fail. Pending and out-of-order migrations are not problems.
func (r MigrationReport) Problems() []string {
	var out []string
	out = append(out, r.Duplicates...)
	for _, v := range r.Unknown {
		out = append(out, fmt.Sprintf("version %d is recorded in the database and has no migration file in this build", v))
	}
	if r.Integrity != "" && r.Integrity != "ok" {
		out = append(out, "integrity_check: "+r.Integrity)
	}
	return out
}

// CheckMigrations opens the file at path read-only and compares schema_migrations with the
// migrations embedded in this build. It does not apply anything and does not create the file.
func CheckMigrations(ctx context.Context, path string) (MigrationReport, error) {
	return checkMigrations(ctx, path, migrations.FS)
}

func checkMigrations(ctx context.Context, path string, files fs.FS) (MigrationReport, error) {
	var rep MigrationReport
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return rep, fmt.Errorf("db: read migrations: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	byVersion := map[int]string{}
	for _, name := range names {
		v, err := versionOf(name)
		if err != nil {
			return rep, err
		}
		if prev, dup := byVersion[v]; dup {
			rep.Duplicates = append(rep.Duplicates, fmt.Sprintf("migration version %d is used by %s and %s", v, prev, name))
			continue
		}
		byVersion[v] = name
		rep.Files = append(rep.Files, MigrationFile{Version: v, Name: name})
	}
	sort.Slice(rep.Files, func(i, j int) bool { return rep.Files[i].Version < rep.Files[j].Version })

	conn, err := openReadOnly(ctx, path)
	if err != nil {
		return rep, err
	}
	defer conn.Close()

	var tables int
	if err := conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'`).Scan(&tables); err != nil {
		return rep, fmt.Errorf("db: read schema: %w", err)
	}
	applied := map[int]bool{}
	if tables == 1 {
		rows, err := conn.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
		if err != nil {
			return rep, fmt.Errorf("db: read schema_migrations: %w", err)
		}
		for rows.Next() {
			var v int
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				return rep, fmt.Errorf("db: read schema_migrations: %w", err)
			}
			rep.Applied = append(rep.Applied, v)
			applied[v] = true
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return rep, fmt.Errorf("db: read schema_migrations: %w", err)
		}
		rows.Close()
	}

	highest := 0
	for _, v := range rep.Applied {
		if v > highest {
			highest = v
		}
		if _, ok := byVersion[v]; !ok {
			rep.Unknown = append(rep.Unknown, v)
		}
	}
	for _, f := range rep.Files {
		if applied[f.Version] {
			continue
		}
		rep.Pending = append(rep.Pending, f)
		if f.Version < highest {
			rep.OutOfOrder = append(rep.OutOfOrder, f.Version)
		}
	}

	rep.Integrity, err = integrity(ctx, conn)
	if err != nil {
		return rep, err
	}
	return rep, nil
}

// integrity runs PRAGMA integrity_check and reduces it to "ok" or a count. The messages can
// name a table or index, and the caller prints this value, so it keeps none of them.
func integrity(ctx context.Context, conn *sql.DB) (string, error) {
	rows, err := conn.QueryContext(ctx, `PRAGMA integrity_check`)
	if err != nil {
		return "", fmt.Errorf("db: integrity_check: %w", err)
	}
	defer rows.Close()
	n := 0
	first := ""
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return "", fmt.Errorf("db: integrity_check: %w", err)
		}
		if n == 0 {
			first = line
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("db: integrity_check: %w", err)
	}
	if n == 1 && first == "ok" {
		return "ok", nil
	}
	return strconv.Itoa(n) + " problems", nil
}

// openReadOnly opens an existing database file for reading. The file is never created, and
// the connection refuses writes to it.
func openReadOnly(ctx context.Context, path string) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("db: path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("db: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("db: %s is not a regular file", abs)
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	q := url.Values{}
	q.Set("mode", "ro")
	q.Set("_busy_timeout", strconv.Itoa(busyTimeoutMS))
	u.RawQuery = q.Encode()
	conn, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", abs, err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: open %s: %w", abs, err)
	}
	return conn, nil
}

// verifyFile opens the file read-only and returns its integrity verdict. Backup runs it on the
// copy it made. It is a variable so a test can stand in for a copy that fails the check, which a
// real VACUUM INTO does not produce.
var verifyFile = func(ctx context.Context, path string) (string, error) {
	conn, err := openReadOnly(ctx, path)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	return integrity(ctx, conn)
}

// BackupResult is the size and SHA-256 of a finished backup file.
type BackupResult struct {
	Bytes  int64
	SHA256 string
}

// ErrBackupExists is returned when the destination is already there. Backup never overwrites.
var ErrBackupExists = errors.New("db: backup destination already exists, refusing to overwrite")

// Backup writes a consistent copy of the database at src to dst with VACUUM INTO. It is safe
// while the app is running: the source is opened read-only and read in one transaction, so the
// copy is one point in time even when a writer is active in WAL mode.
//
// dst is created with mode 0600 before SQLite writes to it, and an existing file is refused
// (ErrBackupExists). A copy that fails its integrity check is removed. Backup applies no
// migration and returns no row data.
func Backup(ctx context.Context, src, dst string) (BackupResult, error) {
	var res BackupResult
	if src == "" || dst == "" {
		return res, errors.New("db: backup needs a source and a destination")
	}
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return res, err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return res, err
	}
	srcInfo, err := os.Stat(srcAbs)
	if err != nil {
		return res, fmt.Errorf("db: %w", err)
	}
	if dstInfo, err := os.Stat(dstAbs); err == nil && os.SameFile(srcInfo, dstInfo) {
		return res, ErrBackupExists
	}

	if err := os.MkdirAll(filepath.Dir(dstAbs), 0o700); err != nil {
		return res, fmt.Errorf("db: create directory for the backup: %w", err)
	}
	// O_EXCL is the refusal to overwrite, and it holds against a symlink. The file exists with
	// mode 0600 before SQLite writes a byte, and VACUUM INTO accepts an empty file.
	f, err := os.OpenFile(dstAbs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return res, ErrBackupExists
		}
		return res, fmt.Errorf("db: create the backup: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(dstAbs)
		return res, fmt.Errorf("db: create the backup: %w", err)
	}
	fail := func(err error) (BackupResult, error) {
		os.Remove(dstAbs)
		return BackupResult{}, err
	}

	conn, err := openReadOnly(ctx, srcAbs)
	if err != nil {
		return fail(err)
	}
	_, err = conn.ExecContext(ctx, `VACUUM INTO ?`, dstAbs)
	conn.Close()
	if err != nil {
		return fail(fmt.Errorf("db: vacuum into the backup: %w", err))
	}

	verdict, err := verifyFile(ctx, dstAbs)
	if err != nil {
		return fail(err)
	}
	if verdict != "ok" {
		return fail(fmt.Errorf("db: the backup failed integrity_check (%s) and was removed", verdict))
	}

	out, err := os.OpenFile(dstAbs, os.O_RDWR, 0)
	if err != nil {
		return fail(fmt.Errorf("db: read the backup: %w", err))
	}
	defer out.Close()
	if err := out.Sync(); err != nil {
		return fail(fmt.Errorf("db: sync the backup: %w", err))
	}
	h := sha256.New()
	n, err := io.Copy(h, out)
	if err != nil {
		return fail(fmt.Errorf("db: read the backup: %w", err))
	}
	if err := os.Chmod(dstAbs, 0o600); err != nil {
		return fail(fmt.Errorf("db: set the backup mode: %w", err))
	}
	return BackupResult{Bytes: n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}
