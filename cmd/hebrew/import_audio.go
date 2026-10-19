package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

func runImport(args []string) error {
	fs := flag.NewFlagSet("import-audio", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: hebrew import-audio --from <export-dir-or-tgz> [--dry-run]\n")
		fmt.Fprintf(fs.Output(), "  --from - reads a .tgz from stdin and does not extract it.\n")
		fs.PrintDefaults()
	}
	from := fs.String("from", "", "export directory, .tgz, or - for a .tgz on stdin")
	dry := fs.Bool("dry-run", false, "count clips and write the unmapped report without storing audio")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if *from == "" {
		fs.Usage()
		return errors.New("--from is required")
	}

	cfg, err := configFromEnv()
	if err != nil {
		return err
	}
	setLogLevel(cfg.LogLevel)

	opts := audio.Options{DryRun: *dry}
	exportDir := *from
	if *from == "-" {
		info, statErr := os.Stdin.Stat()
		if statErr == nil && info.Mode()&os.ModeCharDevice != 0 {
			return errors.New("--from - reads a .tgz on stdin")
		}
		opts.Archive = os.Stdin
		exportDir = ""
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := db.Open(ctx, filepath.Join(cfg.DataDir, "hebrew.sqlite"), cfg.Clock)
	if err != nil {
		return err
	}
	defer store.Close()

	rep, err := audio.Import(ctx, store, cfg.DataDir, exportDir, opts)
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stdout, rep.Summary(*dry))
	if rep.HashMismatch > 0 || rep.TextMismatch > 0 || rep.Conflict > 0 || rep.MissingFile > 0 {
		return fmt.Errorf("hash-mismatch=%d text-mismatch=%d conflict=%d missing-file=%d",
			rep.HashMismatch, rep.TextMismatch, rep.Conflict, rep.MissingFile)
	}
	return nil
}
