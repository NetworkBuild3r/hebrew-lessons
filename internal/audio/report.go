package audio

import (
	"fmt"
	"sort"
	"strings"
)

// Report is the result of one import, including a dry run.
// ByModel counts every mapped file that was read, whether the row was new
// or a duplicate sha256. It does not count skips.
type Report struct {
	Imported        int
	Duplicate       int
	SkippedUnmapped int
	SkippedArchive  int
	MissingFile     int
	ByModel         map[string]int
	Unmapped        []string
	Missing         []string
	ReportPath      string
}

// Summary is the text the command prints. dry marks a run that did not
// write clips or rows.
func (r Report) Summary(dry bool) string {
	mode := ""
	if dry {
		mode = " (dry-run)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "import-audio%s: imported=%d duplicate=%d skipped-unmapped=%d skipped-archive=%d missing-file=%d\n",
		mode, r.Imported, r.Duplicate, r.SkippedUnmapped, r.SkippedArchive, r.MissingFile)
	fmt.Fprintf(&b, "models: %s\n", formatModels(r.ByModel))
	if r.ReportPath != "" {
		fmt.Fprintf(&b, "report: %s\n", r.ReportPath)
	}
	if len(r.Unmapped) > 0 {
		b.WriteString("unmapped:\n")
		for _, path := range r.Unmapped {
			fmt.Fprintf(&b, "  %s\n", path)
		}
	}
	if len(r.Missing) > 0 {
		b.WriteString("missing:\n")
		for _, path := range r.Missing {
			fmt.Fprintf(&b, "  %s\n", path)
		}
	}
	return b.String()
}

func formatModels(counts map[string]int) string {
	order := []string{"eleven_v4", "eleven_v3", "legacy"}
	seen := map[string]bool{}
	var b strings.Builder
	for _, model := range order {
		n := counts[model]
		if n == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%s=%d", model, n)
		seen[model] = true
	}
	var rest []string
	for model, n := range counts {
		if n == 0 || seen[model] {
			continue
		}
		rest = append(rest, model)
	}
	sort.Strings(rest)
	for _, model := range rest {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%s=%d", model, counts[model])
	}
	if b.Len() == 0 {
		return "none"
	}
	return b.String()
}
