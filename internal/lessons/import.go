// Package lessons builds a lesson from a learner's class sheets.
// Code chooses the words, the passage, and the keys. An agent may supply
// only an optional story, and only after CheckStory accepts it.
package lessons

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Sheet is one dated class, as exported. Study notes, a zip, and html are refused.
type Sheet struct {
	Ref      string
	ClassOn  string // YYYY-MM-DD
	Title    string
	Kind     string // "class" or "notes"
	Modified time.Time
	Body     string
}

// Line is one stored line. Tutor and Learner are never both set.
type Line struct {
	Position   int
	Tutor      string
	Learner    string
	Gloss      string
	HasPoints  bool
	Held       bool
	HeldReason string
}

// Import is the result of reading one sheet.
type Import struct {
	Ref       string
	ClassOn   string
	Title     string
	Lines     []Line
	Passed    bool
	Note      string
	Unchanged bool
}

// ImportSheet reads one export. A sheet that is not a dated class does not pass.
// Importing the same modified time again changes nothing. Points are never invented.
func ImportSheet(in Sheet, already time.Time) (Import, error) {
	out := Import{Ref: strings.TrimSpace(in.Ref), ClassOn: strings.TrimSpace(in.ClassOn), Title: strings.TrimSpace(in.Title)}
	if out.Ref == "" {
		return out, fmt.Errorf("lessons: sheet needs a file reference")
	}
	if refusedKind(in) {
		out.Note = "only a dated class sheet can be loaded"
		return out, nil
	}
	if _, err := time.Parse(time.DateOnly, out.ClassOn); err != nil {
		out.Note = "the class date is missing or not YYYY-MM-DD"
		return out, nil
	}
	if !already.IsZero() && already.Equal(in.Modified) {
		out.Unchanged = true
		out.Passed = true
		out.Note = "this export was already imported"
		return out, nil
	}
	var lines []Line
	tutor := 0
	for i, raw := range strings.Split(in.Body, "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		line := parseLine(i, raw)
		if line.Tutor != "" {
			tutor++
		}
		lines = append(lines, line)
	}
	out.Lines = lines
	if tutor == 0 {
		out.Note = "the sheet has no tutor line"
		return out, nil
	}
	out.Passed = true
	out.Note = fmt.Sprintf("%d lines, %d from the tutor", len(lines), tutor)
	return out, nil
}

func refusedKind(in Sheet) bool {
	kind := strings.ToLower(strings.TrimSpace(in.Kind))
	if kind == "notes" || strings.Contains(kind, "study") {
		return true
	}
	ref := strings.ToLower(in.Ref)
	return strings.HasSuffix(ref, ".zip") || strings.HasSuffix(ref, ".html") || strings.HasSuffix(ref, ".htm")
}

func parseLine(position int, raw string) Line {
	line := Line{Position: position}
	if utf8.RuneCountInString(raw) > 2000 {
		line.Held = true
		line.HeldReason = "line is cut off"
		return line
	}
	who, rest, ok := strings.Cut(raw, ":")
	if !ok {
		line.Held = true
		line.HeldReason = "line has no speaker"
		return line
	}
	text, gloss, _ := strings.Cut(rest, "||")
	text = strings.TrimSpace(text)
	gloss = strings.TrimSpace(gloss)
	if text == "" {
		line.Held = true
		line.HeldReason = "line has no Hebrew"
		return line
	}
	switch strings.ToLower(strings.TrimSpace(who)) {
	case "tutor", "t":
		line.Tutor = text
	case "learner", "l":
		line.Learner = text
	default:
		line.Held = true
		line.HeldReason = "speaker is not tutor or learner"
		return line
	}
	line.Gloss = gloss
	line.HasPoints = hasPoints(text)
	return line
}

func hasPoints(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Mn) || (r >= 0x0591 && r <= 0x05C7) {
			return true
		}
	}
	return false
}
