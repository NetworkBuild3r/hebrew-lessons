// Command tokens writes internal/web/static/css/tokens.css from the Figma
// variable export and the hand-written static section.
//
//	go run ./cmd/tokens          # rewrite tokens.css
//	go run ./cmd/tokens -check   # exit 1 when tokens.css is out of date
//
// Run it from the repository root, or pass -in and -out.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/tokens"
)

func main() {
	in := flag.String("in", "internal/web/static/css/lamplight.tokens.json", "Figma variable export")
	out := flag.String("out", "internal/web/static/css/tokens.css", "generated stylesheet")
	check := flag.Bool("check", false, "compare instead of writing")
	flag.Parse()
	if err := run(*in, *out, *check); err != nil {
		fmt.Fprintln(os.Stderr, "tokens:", err)
		os.Exit(1)
	}
}

func run(in, out string, check bool) error {
	data, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	css, err := tokens.Build(data)
	if err != nil {
		return err
	}
	if check {
		have, err := os.ReadFile(out)
		if err != nil {
			return err
		}
		if !bytes.Equal(bytes.ReplaceAll(have, []byte("\r\n"), []byte("\n")), []byte(css)) {
			return fmt.Errorf("%s is out of date; run: go run ./cmd/tokens", out)
		}
		return nil
	}
	if strings.Contains(css, "\r") {
		return fmt.Errorf("generated css contains a carriage return")
	}
	return os.WriteFile(out, []byte(css), 0o644)
}
