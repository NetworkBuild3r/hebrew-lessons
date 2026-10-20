package main

// Two read-only operator commands for the cutover runbook (INIT-015/SPEC-025):
//
//	hebrew migrate --check [--db <file>]
//	hebrew backup --to <file> [--db <file>]
//
// The image is distroless, with no sqlite3 and no tar, so these are subcommands of the binary.
// Neither command applies a migration, and neither prints a row of learner data.

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func runMigrate(args []string) error {
	return migrateCommand(args, os.Stdout, os.Stderr)
}

func runBackup(args []string) error {
	return backupCommand(args, os.Stdout, os.Stderr)
}

// databasePath is the --db value, or DATA_DIR/hebrew.sqlite. It reads DATA_DIR only, so a
// tool pod needs no other setting.
func databasePath(flagValue string) (string, error) {
	if strings.TrimSpace(flagValue) != "" {
		return flagValue, nil
	}
	dir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dir == "" {
		return "", errors.New("DATA_DIR is unset and --db was not given")
	}
	return filepath.Join(dir, "hebrew.sqlite"), nil
}

// migrateCommand is `hebrew migrate --check`. It reports which migrations the database has
// and which this build would apply at start. It is read-only. Without --check it refuses, so
// nobody reads "migrate" as "apply".
func migrateCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hebrew migrate --check [--db <file>]\n")
		fmt.Fprintf(fs.Output(), "  Reads the database read-only and lists applied and pending migrations.\n")
		fmt.Fprintf(fs.Output(), "  It applies nothing. Exit status 1 if a migration version is used twice,\n")
		fmt.Fprintf(fs.Output(), "  the database records a version this build has no file for, or integrity_check fails.\n")
		fs.PrintDefaults()
	}
	check := fs.Bool("check", false, "report applied and pending migrations (required; the command never applies them)")
	dbFlag := fs.String("db", "", "database file to read. Default is $DATA_DIR/hebrew.sqlite")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if !*check {
		fs.Usage()
		return errors.New("migrate: --check is required. This command only reports; migrations are applied when the server starts")
	}
	path, err := databasePath(*dbFlag)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rep, err := db.CheckMigrations(ctx, path)
	if err != nil {
		return err
	}
	printMigrationReport(stdout, rep)
	if n := len(rep.Problems()); n > 0 {
		return fmt.Errorf("migrate --check found %d problem(s)", n)
	}
	return nil
}

func printMigrationReport(w io.Writer, rep db.MigrationReport) {
	fmt.Fprintf(w, "migrate-check: files=%d applied=%d pending=%d\n", len(rep.Files), len(rep.Applied), len(rep.Pending))
	fmt.Fprintf(w, "applied: %s\n", listOrNone(ints(rep.Applied)))
	pending := make([]string, 0, len(rep.Pending))
	for _, f := range rep.Pending {
		pending = append(pending, f.Name)
	}
	fmt.Fprintf(w, "pending: %s\n", listOrNone(pending))
	if len(rep.OutOfOrder) > 0 {
		fmt.Fprintf(w, "pending-below-applied: %s (the runner applies by number, so these still run)\n", strings.Join(ints(rep.OutOfOrder), " "))
	}
	for _, p := range rep.Problems() {
		fmt.Fprintf(w, "problem: %s\n", p)
	}
	fmt.Fprintf(w, "integrity: %s\n", rep.Integrity)
	if n := len(rep.Problems()); n > 0 {
		fmt.Fprintf(w, "result: problems=%d\n", n)
		return
	}
	fmt.Fprintln(w, "result: ok")
}

func ints(vs []int) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		out = append(out, strconv.Itoa(v))
	}
	return out
}

func listOrNone(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, " ")
}

// backupCommand is `hebrew backup --to <file>`. It writes one consistent copy with VACUUM INTO,
// refuses to overwrite, creates the file 0600, and prints the byte count and SHA-256 only.
func backupCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hebrew backup --to <file> [--db <file>]\n")
		fmt.Fprintf(fs.Output(), "  Writes a consistent copy of the database with VACUUM INTO. It is safe while the app runs.\n")
		fmt.Fprintf(fs.Output(), "  It refuses to overwrite, creates the file with mode 0600, and prints bytes and sha256 only.\n")
		fs.PrintDefaults()
	}
	to := fs.String("to", "", "file to create (required). It must not exist")
	dbFlag := fs.String("db", "", "database file to copy. Default is $DATA_DIR/hebrew.sqlite")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if strings.TrimSpace(*to) == "" {
		fs.Usage()
		return errors.New("--to is required")
	}
	src, err := databasePath(*dbFlag)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	res, err := db.Backup(ctx, src, *to)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "bytes=%d sha256=%s\n", res.Bytes, res.SHA256)
	return nil
}
