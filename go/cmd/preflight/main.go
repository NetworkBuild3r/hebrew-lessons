// Command preflight checks a restored backup before the Go cutover.
//
// It opens hebrew.sqlite read-only, checks the migration ledger, and
// reports lexicon-key and lesson round-trip findings when those
// dependencies are wired. It never writes.
package main

import "os"

// version and commit are set by the image build:
//
//	-X main.version -X main.commit
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
