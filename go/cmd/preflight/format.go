package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type report struct {
	OK         bool     `json:"ok"`
	Version    string   `json:"version"`
	Commit     string   `json:"commit"`
	DBPath     string   `json:"dbPath,omitempty"`
	LessonsDir string   `json:"lessonsDir,omitempty"`
	Checks     []Result `json:"checks,omitempty"`
	Error      string   `json:"error,omitempty"`
}

func (r report) passed() bool {
	if r.Error != "" || len(r.Checks) == 0 {
		return false
	}
	for _, check := range r.Checks {
		if check.Status != statusPass {
			return false
		}
	}
	return true
}

func writeReport(stdout, stderr io.Writer, rep report, jsonOut bool) error {
	human := stderr
	if !jsonOut {
		human = stdout
	}
	writeHuman(human, rep)
	if !jsonOut {
		return nil
	}
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

func writeHuman(w io.Writer, rep report) {
	if rep.DBPath != "" {
		fmt.Fprintf(w, "preflight: db=%s\n", rep.DBPath)
	}
	if rep.LessonsDir != "" {
		fmt.Fprintf(w, "preflight: lessons=%s\n", rep.LessonsDir)
	}
	if rep.Error != "" && len(rep.Checks) == 0 {
		fmt.Fprintf(w, "preflight: %s\n", rep.Error)
		fmt.Fprintln(w, "preflight: FAIL")
		return
	}
	for _, check := range rep.Checks {
		fmt.Fprintf(w, "preflight: %s %s — %s\n", check.Name, check.Status, check.Summary)
		writeDetail(w, check)
	}
	fmt.Fprintln(w, verdict(rep))
}

func verdict(rep report) string {
	if rep.passed() {
		return "preflight: PASS"
	}
	var bad []string
	for _, check := range rep.Checks {
		if check.Status != statusPass {
			bad = append(bad, check.Name)
		}
	}
	if len(bad) == 0 {
		return "preflight: FAIL"
	}
	return "preflight: FAIL (" + strings.Join(bad, ", ") + ")"
}

func writeDetail(w io.Writer, check Result) {
	switch d := check.Detail.(type) {
	case lexiconDetail:
		if d.Pending != "" {
			return
		}
		for _, class := range d.Classes {
			fmt.Fprintf(w, "  %s: %d\n", class.Class, class.Count)
			shown := 0
			for _, ex := range d.Examples {
				if ex.Class != class.Class {
					continue
				}
				shown++
				fmt.Fprintf(w, "    rowid=%d column=%s key=%s exact=%s\n",
					ex.RowID, ex.Column, ex.KeyCodePoints, ex.ExactKeyCodePoints)
			}
			if class.Truncated {
				fmt.Fprintf(w, "    … %d more not shown\n", class.Count-shown)
			}
		}
		fmt.Fprintf(w, "  collisions: %d\n", d.Collisions)
		for _, group := range d.CollisionGroups {
			fmt.Fprintf(w, "    exact=%s\n", group.ExactKeyCodePoints)
			for _, row := range group.Rows {
				fmt.Fprintf(w, "      rowid=%d column=%s key=%s\n",
					row.RowID, row.Column, row.KeyCodePoints)
			}
			if group.Truncated {
				fmt.Fprintf(w, "      … more rows not shown\n")
			}
		}
		if d.Collisions > len(d.CollisionGroups) {
			fmt.Fprintf(w, "    … %d more collision groups not shown\n",
				d.Collisions-len(d.CollisionGroups))
		}
	case lessonDetail:
		for _, ex := range d.Examples {
			fmt.Fprintf(w, "  %s: %s\n", ex.Path, ex.Reason)
		}
		if d.Truncated {
			fmt.Fprintf(w, "  … more lessons not shown\n")
		}
	}
}
