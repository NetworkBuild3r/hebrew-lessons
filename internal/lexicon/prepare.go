package lexicon

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Status values upsert_lexemes reports for one row.
const (
	StatusCreated   = "created"
	StatusUpdated   = "updated"
	StatusUnchanged = "unchanged"
)

// MaxBatch is the most entries one upsert_lexemes call accepts.
const MaxBatch = 200

const (
	maxTextNFC  = 64
	maxGloss    = 256
	maxTranslit = 64
	maxPOS      = 16
	maxPlural   = 64
	maxRoot     = 16
)

// Input is one upsert_lexemes entry before normalization.
// A nil pointer means the caller omitted the field. On an update, an
// omitted pos, gender, plural, translit, root, binyan, or verified keeps
// the stored value. A non-nil pointer replaces it, and an empty string
// clears a text column. A new lexeme requires translit: a missing or empty value
// is rejected by the writer, which can see whether the row already
// exists. An empty bare pointer is treated as omitted, and the
// consonant skeleton is derived.
type Input struct {
	TextNFC  string
	Bare     *string
	Gloss    string
	Translit *string
	POS      *string
	Gender   *string
	Plural   *string
	Root     *string
	Binyan   *string
	Verified *bool
}

// Row is one entry ready to store.
// TextNFC is units.Normalize, the same string audio.NormalizeText stores.
// Bare is units.StripNiqqud of that text. A nil POS, Gender, Plural, Root,
// or Binyan means the caller omitted it: a new row stores empty, and an
// existing row keeps the column. Root, when set, is stored as consonants
// joined by a maqaf. A nil or empty Translit is rejected for a new
// row. An existing row keeps translit when it is omitted. Verified nil
// means a new row is unverified and an existing row keeps its flag.
type Row struct {
	TextNFC  string
	Bare     string
	Gloss    string
	Translit *string
	POS      *string
	Gender   *string
	Plural   *string
	Root     *string
	Binyan   *string
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
	var b strings.Builder
	b.WriteString("The batch was rejected and nothing was written.")
	if e == nil {
		return b.String()
	}
	for _, issue := range e.Issues {
		b.WriteByte('\n')
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
		return nil, &Invalid{Issues: []Issue{{Pointer: "/entries", Msg: "The entries list is empty."}}}
	}
	if len(entries) > MaxBatch {
		return nil, &Invalid{Issues: []Issue{{
			Pointer: "/entries",
			Msg:     fmt.Sprintf("A batch may contain at most %d entries.", MaxBatch),
		}}}
	}
	var issues []Issue
	rows := make([]Row, 0, len(entries))
	seen := map[string]int{}
	for i, in := range entries {
		text, textOK := textOf(i, in.TextNFC, &issues)
		gloss, glossOK := glossOf(i, in.Gloss, &issues)
		pos, posOK := limited(i, "pos", "The pos value", in.POS, maxPOS, &issues)
		translit, translitOK := limited(i, "translit", "The translit value", in.Translit, maxTranslit, &issues)
		plural, pluralOK := pluralOf(i, in.Plural, &issues)
		gender, genderOK := genderOf(i, in.Gender, &issues)
		root, rootOK := rootOf(i, in.Root, &issues)
		binyan, binyanOK := binyanOf(i, in.Binyan, &issues)
		if !textOK || !glossOK || !posOK || !translitOK || !pluralOK || !genderOK || !rootOK || !binyanOK {
			continue
		}
		bare, bareOK := bareOf(i, text, in.Bare, &issues)
		if !bareOK {
			continue
		}
		if prev, ok := seen[text]; ok {
			issues = append(issues, Issue{
				Pointer: fieldPtr(i, "text_nfc"),
				Msg:     "The text_nfc value duplicates /entries/" + strconv.Itoa(prev) + ".",
			})
			continue
		}
		seen[text] = i
		rows = append(rows, Row{
			TextNFC:  text,
			Bare:     bare,
			Gloss:    gloss,
			Translit: translit,
			POS:      pos,
			Gender:   gender,
			Plural:   plural,
			Root:     root,
			Binyan:   binyan,
			Verified: in.Verified,
		})
	}
	if len(issues) > 0 {
		return nil, &Invalid{Issues: issues}
	}
	return rows, nil
}

func textOf(i int, raw string, issues *[]Issue) (string, bool) {
	text := units.Normalize(raw)
	if text == "" {
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, "text_nfc"), Msg: "The text_nfc value is empty."})
		return "", false
	}
	if utf8.RuneCountInString(text) > maxTextNFC {
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, "text_nfc"), Msg: "The text_nfc value is longer than 64 characters."})
		return "", false
	}
	if !hebrewValue(i, "text_nfc", "The text_nfc value", text, issues) {
		return "", false
	}
	return text, true
}

func glossOf(i int, raw string, issues *[]Issue) (string, bool) {
	gloss := strings.TrimSpace(raw)
	if gloss == "" {
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, "gloss_en"), Msg: "The gloss_en value is empty."})
		return "", false
	}
	if utf8.RuneCountInString(gloss) > maxGloss {
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, "gloss_en"), Msg: "The gloss_en value is longer than 256 characters."})
		return "", false
	}
	return gloss, true
}

func limited(i int, field, label string, raw *string, max int, issues *[]Issue) (*string, bool) {
	if raw == nil {
		return nil, true
	}
	v := strings.TrimSpace(*raw)
	if utf8.RuneCountInString(v) > max {
		*issues = append(*issues, Issue{
			Pointer: fieldPtr(i, field),
			Msg:     fmt.Sprintf("%s is longer than %d characters.", label, max),
		})
		return nil, false
	}
	return &v, true
}

func pluralOf(i int, raw *string, issues *[]Issue) (*string, bool) {
	if raw == nil {
		return nil, true
	}
	v := units.Normalize(*raw)
	if v == "" {
		return &v, true
	}
	if utf8.RuneCountInString(v) > maxPlural {
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, "plural"), Msg: "The plural value is longer than 64 characters."})
		return nil, false
	}
	if !hebrewValue(i, "plural", "The plural value", v, issues) {
		return nil, false
	}
	return &v, true
}

func genderOf(i int, raw *string, issues *[]Issue) (*string, bool) {
	if raw == nil {
		return nil, true
	}
	g := strings.TrimSpace(*raw)
	switch g {
	case "", "m", "f", "both":
		return &g, true
	default:
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, "gender"), Msg: "Gender must be m, f, or both."})
		return nil, false
	}
}

func bareOf(i int, text string, raw *string, issues *[]Issue) (string, bool) {
	want := units.StripNiqqud(text)
	if want == "" {
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, "text_nfc"), Msg: "The text_nfc value has no consonants."})
		return "", false
	}
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return want, true
	}
	got := units.StripNiqqud(units.Normalize(*raw))
	if got != want {
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, "bare"), Msg: "The bare value does not match text_nfc."})
		return "", false
	}
	return want, true
}

func hebrewValue(i int, field, label, s string, issues *[]Issue) bool {
	letters := 0
	for _, r := range s {
		if !hebrewRune(r) {
			*issues = append(*issues, Issue{
				Pointer: fieldPtr(i, field),
				Msg:     label + " may contain only Hebrew letters, points, spaces, maqaf, and geresh.",
			})
			return false
		}
		if r >= 0x05D0 && r <= 0x05EA {
			letters++
		}
	}
	if letters == 0 {
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, field), Msg: label + " must include a Hebrew letter."})
		return false
	}
	return true
}

func hebrewRune(r rune) bool {
	switch {
	case r >= 0x05D0 && r <= 0x05EA:
		return true
	case r >= 0x0591 && r <= 0x05C7:
		return true
	case r == ' ' || r == 0x05F3 || r == 0x05F4:
		return true
	default:
		return false
	}
}

func fieldPtr(i int, field string) string {
	return "/entries/" + strconv.Itoa(i) + "/" + field
}

// TranslitBlank reports a missing or empty transliteration.
func TranslitBlank(row Row) bool {
	return row.Translit == nil || strings.TrimSpace(*row.Translit) == ""
}

// NewTranslitIssue is the per-entry error for a new lexeme with no translit.
func NewTranslitIssue(i int) Issue {
	return Issue{
		Pointer: fieldPtr(i, "translit"),
		Msg:     "A new lexeme requires a transliteration.",
	}
}

// binyanim are the verb patterns a lexicon row may name. The empty string clears the column.
var binyanim = map[string]bool{
	"paal": true, "nifal": true, "piel": true, "pual": true,
	"hifil": true, "hufal": true, "hitpael": true,
}

const rootMsg = "A root is 2 to 4 Hebrew consonants separated by a maqaf or a hyphen. Shin and sin dots stay, and other points are removed."

// rootOf normalizes a root. Nil keeps the stored value. An empty string clears it.
// A set value is stored as its consonants joined by a maqaf (U+05BE), with the shin
// and sin dots kept and every other point removed.
func rootOf(i int, raw *string, issues *[]Issue) (*string, bool) {
	if raw == nil {
		return nil, true
	}
	v := strings.TrimSpace(*raw)
	if v == "" {
		return &v, true
	}
	if utf8.RuneCountInString(v) > maxRoot {
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, "root"), Msg: "The root value is longer than 16 characters."})
		return nil, false
	}
	norm, ok := normalizeRoot(v)
	if !ok {
		*issues = append(*issues, Issue{Pointer: fieldPtr(i, "root"), Msg: rootMsg})
		return nil, false
	}
	return &norm, true
}

// binyanOf checks a verb pattern. Nil keeps the stored value. An empty string clears it.
func binyanOf(i int, raw *string, issues *[]Issue) (*string, bool) {
	if raw == nil {
		return nil, true
	}
	v := strings.ToLower(strings.TrimSpace(*raw))
	if v == "" || binyanim[v] {
		return &v, true
	}
	*issues = append(*issues, Issue{
		Pointer: fieldPtr(i, "binyan"),
		Msg:     "Binyan must be paal, nifal, piel, pual, hifil, hufal, hitpael, or empty.",
	})
	return nil, false
}

// normalizeRoot splits a root into its consonants. A separator is a maqaf, a hyphen, or a
// space. A shin or sin dot stays on its letter. Niqqud is dropped. Anything else, or a count
// outside 2 to 4, is rejected.
func normalizeRoot(s string) (string, bool) {
	var letters [][]rune
	var cur []rune
	flush := func() {
		if cur != nil {
			letters = append(letters, cur)
			cur = nil
		}
	}
	for _, r := range units.Normalize(s) {
		switch {
		case r >= 0x05D0 && r <= 0x05EA:
			flush()
			cur = []rune{r}
		case r == 0x05C1 || r == 0x05C2:
			if cur == nil {
				return "", false
			}
			cur = append(cur, r)
		case isRootPoint(r):
		case r == 0x05BE || r == '-' || r == ' ':
			if cur == nil {
				return "", false
			}
			flush()
		default:
			return "", false
		}
	}
	flush()
	if len(letters) < 2 || len(letters) > 4 {
		return "", false
	}
	var b strings.Builder
	for i, c := range letters {
		if i > 0 {
			b.WriteRune(0x05BE)
		}
		for _, r := range c {
			b.WriteRune(r)
		}
	}
	return b.String(), true
}

// isRootPoint is a Hebrew point a root ignores: the vowel and cantillation marks, but not the
// shin and sin dots, which distinguish letters.
func isRootPoint(r rune) bool {
	switch {
	case r >= 0x0591 && r <= 0x05AF:
		return true
	case r >= 0x05B0 && r <= 0x05BD:
		return true
	case r == 0x05BF || r == 0x05C4 || r == 0x05C5 || r == 0x05C7:
		return true
	default:
		return false
	}
}
