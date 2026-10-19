package lexicon

import (
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Status values upsert_lexemes reports for one row.
const (
	StatusCreated   = "created"
	StatusUpdated   = "updated"
	StatusUnchanged = "unchanged"
)

// Input is one upsert_lexemes entry before normalization.
// A nil pointer means the caller omitted the field.
// An empty bare pointer is treated as omitted, and the consonant
// skeleton is derived. Gender, plural, and translit empty mean the
// column stays unset.
type Input struct {
	TextNFC  string
	Bare     *string
	Gloss    string
	Translit *string
	POS      string
	Gender   *string
	Plural   *string
	Verified *bool
}

// Row is one entry ready to store.
// TextNFC is units.Normalize, the same string audio.NormalizeText stores.
// Bare is units.StripNiqqud of that text. Empty Gender, Plural, and
// Translit are stored as NULL. Verified nil means a new row is unverified
// and an existing row keeps its flag.
type Row struct {
	TextNFC  string
	Bare     string
	Gloss    string
	Translit string
	POS      string
	Gender   string
	Plural   string
	Verified *bool
}

// Issue is one rejected entry. Pointer is a JSON pointer into the
// upsert_lexemes arguments, such as /entries/0/gender.
type Issue struct {
	Pointer string
	Msg     string
}

// Invalid is a batch the writer must not start. Every issue names the
// entry that failed. The caller rejects the whole batch.
type Invalid struct {
	Issues []Issue
}

func (e *Invalid) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return "invalid lexeme batch"
	}
	var b strings.Builder
	for i, issue := range e.Issues {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(issue.Pointer)
		b.WriteString(": ")
		b.WriteString(issue.Msg)
	}
	return b.String()
}

// Prepare normalizes a batch. text_nfc uses units.Normalize, which is the
// audio importer's NormalizeText. bare uses units.StripNiqqud when the
// caller omits it. Any issue rejects the batch: the returned rows are
// nil, and nothing is written.
func Prepare(entries []Input) ([]Row, *Invalid) {
	if len(entries) == 0 {
		return nil, &Invalid{Issues: []Issue{{Pointer: "/entries", Msg: "entries is empty"}}}
	}
	var issues []Issue
	rows := make([]Row, 0, len(entries))
	seen := map[string]int{}
	for i, in := range entries {
		text := units.Normalize(in.TextNFC)
		if text == "" {
			issues = append(issues, Issue{Pointer: fieldPtr(i, "text_nfc"), Msg: "text_nfc is empty"})
			continue
		}
		gloss := strings.TrimSpace(in.Gloss)
		if gloss == "" {
			issues = append(issues, Issue{Pointer: fieldPtr(i, "gloss_en"), Msg: "gloss_en is empty"})
		}
		bare, bareOK := bareOf(i, text, in.Bare, &issues)
		gender, genderOK := genderOf(i, in.Gender, &issues)
		if !bareOK || !genderOK || gloss == "" {
			continue
		}
		if prev, ok := seen[text]; ok {
			issues = append(issues, Issue{
				Pointer: fieldPtr(i, "text_nfc"),
				Msg:     "duplicate text_nfc of /entries/" + strconv.Itoa(prev),
			})
			continue
		}
		seen[text] = i
		row := Row{
			TextNFC:  text,
			Bare:     bare,
			Gloss:    gloss,
			POS:      strings.TrimSpace(in.POS),
			Gender:   gender,
			Verified: in.Verified,
		}
		if in.Translit != nil {
			row.Translit = strings.TrimSpace(*in.Translit)
		}
		if in.Plural != nil {
			row.Plural = units.Normalize(*in.Plural)
		}
		rows = append(rows, row)
	}
	if len(issues) > 0 {
		return nil, &Invalid{Issues: issues}
	}
	return rows, nil
}

func bareOf(i int, text string, raw *string, issues *[]Issue) (string, bool) {
	want := units.StripNiqqud(text)
	if want == "" {
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, "text_nfc"), Msg: "text_nfc has no consonants"})
		return "", false
	}
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return want, true
	}
	got := units.StripNiqqud(units.Normalize(*raw))
	if got != want {
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, "bare"), Msg: "bare does not match text_nfc"})
		return "", false
	}
	return want, true
}

func genderOf(i int, raw *string, issues *[]Issue) (string, bool) {
	if raw == nil {
		return "", true
	}
	g := strings.TrimSpace(*raw)
	switch g {
	case "", "m", "f", "both":
		return g, true
	default:
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, "gender"), Msg: "gender must be m, f, or both"})
		return "", false
	}
}

func fieldPtr(i int, field string) string {
	return "/entries/" + strconv.Itoa(i) + "/" + field
}
