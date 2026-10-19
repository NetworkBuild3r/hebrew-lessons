package audio

import (
	"fmt"
	"sort"
	"strings"
)

// Report is the result of one import, including a dry run.
// ByModel counts every mapped file that passed both hash checks and was
// not a same-bytes/different-text conflict. Duplicates and aliases are
// included. Mismatches, conflicts, and unmapped files are not.
type Report struct {
	Imported     int
	Duplicate    int
	Unmapped     int
	HashMismatch int
	TextMismatch int
	Conflict     int
	Alias        int
	MissingFile  int
	ByModel      map[string]int

	UnmappedItems  []UnmappedItem
	Missing        []string
	HashMismatches []Mismatch
	TextMismatches []Mismatch
	Conflicts      []Conflict

	ReportPath   string
	ManifestPath string
}

// UnmappedItem is a clip that was not stored because the map left it out
// or its text was empty after normalization.
type UnmappedItem struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Mismatch is a clip whose bytes or legacy text hash did not match the map.
// It is not stored.
type Mismatch struct {
	Path string `json:"path"`
	Want string `json:"want"`
	Got  string `json:"got"`
}

// Conflict is one sha256 whose paths disagree on the text.
type Conflict struct {
	SHA256 string         `json:"sha256"`
	Clips  []ConflictClip `json:"clips"`
}

// ConflictClip is one path in a same-bytes/different-text conflict.
type ConflictClip struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

// ManifestEntry maps one export path to the sha256 id and the normalized text.
// Alias is true when another stored asset with a lower id has the same
// text, voice, and model. Conflict is true when this path's text was not
// stored because the same bytes were already stored under a different text.
type ManifestEntry struct {
	Path     string `json:"path"`
	ID       string `json:"id"`
	Text     string `json:"text"`
	Voice    string `json:"voice"`
	Model    string `json:"model"`
	Alias    bool   `json:"alias"`
	Conflict bool   `json:"conflict,omitempty"`
}

// Summary is the text the command prints. dry marks a run that did not
// write clips or rows.
func (r Report) Summary(dry bool) string {
	mode := ""
	if dry {
		mode = " (dry-run)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "import-audio%s: imported=%d duplicate=%d unmapped=%d hash-mismatch=%d text-mismatch=%d conflict=%d alias=%d missing-file=%d\n",
		mode, r.Imported, r.Duplicate, r.Unmapped, r.HashMismatch, r.TextMismatch, r.Conflict, r.Alias, r.MissingFile)
	fmt.Fprintf(&b, "models: %s\n", formatModels(r.ByModel))
	if r.ReportPath != "" {
		fmt.Fprintf(&b, "report: %s\n", r.ReportPath)
	}
	if r.ManifestPath != "" {
		fmt.Fprintf(&b, "manifest: %s\n", r.ManifestPath)
	}
	writePaths(&b, "unmapped", unmappedPaths(r.UnmappedItems))
	writePaths(&b, "missing", r.Missing)
	writePaths(&b, "hash-mismatch", mismatchPaths(r.HashMismatches))
	writePaths(&b, "text-mismatch", mismatchPaths(r.TextMismatches))
	if len(r.Conflicts) > 0 {
		b.WriteString("conflict:\n")
		for _, c := range r.Conflicts {
			for _, clip := range c.Clips {
				fmt.Fprintf(&b, "  %s %s %s\n", c.SHA256, clip.Path, clip.Text)
			}
		}
	}
	return b.String()
}

func unmappedPaths(items []UnmappedItem) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Path
	}
	return out
}

func mismatchPaths(items []Mismatch) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Path
	}
	return out
}

func writePaths(b *strings.Builder, label string, paths []string) {
	if len(paths) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", label)
	for _, path := range paths {
		fmt.Fprintf(b, "  %s\n", path)
	}
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
