// Command coursegen builds internal/speech/course.tsv from the frequency
// list, the Wiktionary extract, and course_fixes.tsv.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("coursegen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dictPath := fs.String("dict", "", "kaikki.org Hebrew JSONL (not committed)")
	listPath := fs.String("list", "internal/speech/fdosh.tsv", "frequency list")
	fixPath := fs.String("fixes", "internal/speech/course_fixes.tsv", "hand fixes")
	outPath := fs.String("out", "internal/speech/course.tsv", "course file to write")
	show := fs.String("show", "", "print dictionary entries for this lemma and write nothing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dictPath == "" {
		return fmt.Errorf("coursegen: -dict is required")
	}
	dict, err := os.Open(*dictPath)
	if err != nil {
		return err
	}
	defer dict.Close()
	if *show != "" {
		return Show(dict, *show, stdout)
	}
	lemmas, err := readLemmas(*listPath)
	if err != nil {
		return err
	}
	fixes, err := readFixes(*fixPath)
	if err != nil {
		return err
	}
	body, err := Build(dict, lemmas, fixes)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*outPath, body, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s (%d bytes)\n", *outPath, len(body))
	return nil
}

func readLemmas(path string) ([]Lemma, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseLemmas(string(b))
}

func readFixes(path string) ([]Fix, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseFixes(string(b))
}
