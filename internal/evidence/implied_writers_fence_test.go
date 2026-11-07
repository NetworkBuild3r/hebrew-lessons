package evidence

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// impliedWriterAllowlist is every non-test Go file under internal/ and cmd/ that
// may set Implied = true (or Implied: true) on a review log. ADR-0030 D3 /
// INIT-035/SPEC-003: only typed spelling and its backfill write implied rows
// onto spelling cards; Recorder.Implied is the move-up path and now refuses spell.
//
// Other hits must be read before adding. Do not silently widen this list.
var impliedWriterAllowlist = map[string]string{
	"internal/evidence/spell.go":    "spellImplied writes typed English→Hebrew credit onto spelling cards",
	"internal/evidence/recorder.go": "Recorder.Implied sets the flag for move-up on non-spell cards only",
	"cmd/hebrew/backfill_spell.go":  "one-time backfill of historical typed spelling credit",
	// Sets AnswerOutcome.Implied after Recorder.Implied; never writes review_log.
	"internal/session/session.go": "AnswerOutcome flag only; cannot target a spell card via review_log",
}

var (
	impliedAssign = regexp.MustCompile(`\bImplied\s*=\s*true\b`)
	impliedLit    = regexp.MustCompile(`\bImplied\s*:\s*true\b`)
)

// TestImpliedWritersFence fails when a new production file sets Implied = true
// outside the allowlist (INIT-035/SPEC-003).
func TestImpliedWritersFence(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "../.."))
	var offenders []string
	for _, top := range []string{"internal", "cmd"} {
		base := filepath.Join(root, top)
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := d.Name()
				if name == "testdata" || name == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sc := bufio.NewScanner(strings.NewReader(string(raw)))
			lineNo := 0
			for sc.Scan() {
				lineNo++
				rawLine := sc.Text()
				line := rawLine
				if strings.Contains(line, "//") {
					// Keep code before a line comment; skip pure-comment lines.
					code, _, _ := strings.Cut(line, "//")
					line = code
				}
				if !impliedAssign.MatchString(line) && !impliedLit.MatchString(line) {
					continue
				}
				if _, allowed := impliedWriterAllowlist[rel]; allowed {
					continue
				}
				offenders = append(offenders, rel+":"+strconv.Itoa(lineNo)+": "+strings.TrimSpace(rawLine))
			}
			return sc.Err()
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("Implied = true outside allowlist:\n%s", strings.Join(offenders, "\n"))
	}
	for rel := range impliedWriterAllowlist {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("allowlist names missing file %s: %v", rel, err)
		}
	}
}
