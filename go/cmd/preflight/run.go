package main

import (
	"context"
	"fmt"
	"io"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/config"
	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/db"
)

func run(args []string, stdout, stderr io.Writer) int {
	return runWith(args, stdout, stderr, productionDeps())
}

func runWith(args []string, stdout, stderr io.Writer, deps Deps) int {
	jsonOut, help, err := parseArgs(args)
	if help {
		fmt.Fprint(stdout, usageText)
		return 0
	}
	if err != nil {
		fmt.Fprintf(stderr, "preflight: %s\n%s", err.Error(), usageText)
		return 2
	}
	rep := execute(context.Background(), deps)
	if writeErr := writeReport(stdout, stderr, rep, jsonOut); writeErr != nil {
		fmt.Fprintf(stderr, "preflight: write report: %v\n", writeErr)
		return 1
	}
	if rep.passed() {
		return 0
	}
	return 1
}

// execute opens the database read-only and runs every check. deps is
// productionDeps in the binary. Tests pass a Deps whose ExactKey and
// RoundTrip are filled in.
func execute(ctx context.Context, deps Deps) report {
	cfg, err := config.Load()
	if err != nil {
		return report{Version: version, Commit: commit, Error: err.Error()}
	}
	rep := report{
		Version:    version,
		Commit:     commit,
		DBPath:     cfg.DBPath,
		LessonsDir: cfg.LessonsDir,
	}
	conn, err := db.OpenReadOnly(cfg.DBPath)
	if err != nil {
		rep.Error = err.Error()
		return rep
	}
	defer conn.Close()

	env := Env{
		DB:         conn,
		DBPath:     cfg.DBPath,
		LessonsDir: cfg.LessonsDir,
		Deps:       deps,
	}
	for _, check := range checks() {
		rep.Checks = append(rep.Checks, check.Run(ctx, env))
	}
	rep.OK = rep.passed()
	return rep
}

func parseArgs(args []string) (jsonOut bool, help bool, err error) {
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "-h", "--help":
			help = true
		default:
			return false, false, fmt.Errorf("unknown argument %q", arg)
		}
	}
	return jsonOut, help, nil
}

const usageText = `preflight [--json]

Read-only cutover checks for a restored backup. Paths come from the
environment, the same way the server resolves them (go/internal/config):

  HEBREW_ROOT   app root (default: working directory)
  DATA_DIR      database directory (default: $HEBREW_ROOT/data)
  LESSONS_DIR   worksheets directory (default: $HEBREW_ROOT/worksheets)

The database file is $DATA_DIR/hebrew.sqlite, opened with the read-only
helper in go/internal/db. Lesson files are $LESSONS_DIR/<name>/worksheet.js.

Exit 0 when every check passes. Exit 1 when a check fails or is not wired.
Exit 2 on a usage error. This command does not migrate and does not write.

--json writes the report as JSON on stdout and the human report on stderr.
Without --json the human report is on stdout.
`
