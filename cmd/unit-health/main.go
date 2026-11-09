// Command unit-health lists published units and their ModeWrite block counts.
//
//	unit-health --learner N [--db file] [--unhealthy]
//
// Read-only (PRAGMA query_only). Never calls put_unit, publish_unit,
// unpublish_unit, or any upsert_*. INIT-037/SPEC-009.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcpstore"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "unit-health: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("unit-health", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: unit-health --learner N [--db <file>] [--unhealthy]\n")
		fmt.Fprintf(fs.Output(), "  Read-only (PRAGMA query_only). Lists published units in scope with\n")
		fmt.Fprintf(fs.Output(), "  id, position, state, blocks, and rule ids under ModeWrite with no brief.\n")
		fs.PrintDefaults()
	}
	learnerID := fs.Int64("learner", 0, "learner id (required; shared units plus this learner's)")
	dbFlag := fs.String("db", "", "database file. Default is $DATA_DIR/hebrew.sqlite")
	unhealthyOnly := fs.Bool("unhealthy", false, "print only units with blocks > 0 or unit.same_shape")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if *learnerID <= 0 {
		return errors.New("--learner N is required (positive id)")
	}
	path, err := databasePath(*dbFlag)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	raw, err := db.OpenQueryOnly(ctx, path, nil)
	if err != nil {
		return err
	}
	defer raw.Close()

	store := mcpstore.New(raw)
	id := *learnerID
	rows, err := mcp.SweepPublished(ctx, store, store, store, &id)
	if err != nil {
		return err
	}
	if *unhealthyOnly {
		rows = mcp.UnhealthyPublished(rows)
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}

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
