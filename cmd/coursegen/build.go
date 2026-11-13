package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/speech"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Lemma is one frequency-list row the course is built from.
type Lemma struct {
	Text string
	Rank int
}

// Fix is one hand-kept correction. Several split lines share a rank and lemma.
type Fix struct {
	Rank       int
	Lemma      string
	Action     string
	Pointed    string
	Translit   string
	Gloss      string
	POS        string
	Gender     string
	Root       string
	Binyan     string
	Reason     string
	ReviewedBy string
	ReviewedOn string
}

type courseRow struct {
	Order      int
	Rank       int
	Lemma      string
	Pointed    string
	Translit   string
	Gloss      string
	POS        string
	Gender     string
	Root       string
	Binyan     string
	Present    bool
	Kind       string
	Forms      []string
	Status     string
	Check      string
	Source     string
	ReviewedBy string
}

// Build reads the dictionary and returns the course file bytes.
// The same inputs always return the same bytes.
func Build(dict io.Reader, lemmas []Lemma, fixes []Fix) ([]byte, error) {
	byKey, err := indexDict(dict)
	if err != nil {
		return nil, err
	}
	grouped := map[string][]Fix{}
	for _, f := range fixes {
		k := strconv.Itoa(f.Rank) + "\t" + f.Lemma
		grouped[k] = append(grouped[k], f)
	}
	var rows []courseRow
	for _, lem := range lemmas {
		base := fromPick(lem, pick(byKey[speech.LemmaKey(lem.Text)]))
		rows = append(rows, applyFixes(base, grouped[strconv.Itoa(lem.Rank)+"\t"+lem.Text])...)
	}
	assignStatus(rows)
	for i := range rows {
		rows[i].Order = i + 1
	}
	var b bytes.Buffer
	b.WriteString("order\trank\tlemma\tpointed\ttranslit\tgloss\tpos\tgender\troot\tbinyan\tpresent\tkind\tforms\tstatus\tcheck\tsource\treviewed_by\n")
	for _, r := range rows {
		b.WriteString(strings.Join(r.fields(), "\t"))
		b.WriteByte('\n')
	}
	return b.Bytes(), nil
}

func (r courseRow) fields() []string {
	present := "0"
	if r.Present {
		present = "1"
	}
	clean := func(s string) string {
		return strings.NewReplacer("\t", " ", "\n", " ", "\r", "").Replace(s)
	}
	return []string{
		strconv.Itoa(r.Order), strconv.Itoa(r.Rank), clean(r.Lemma), clean(r.Pointed),
		clean(r.Translit), clean(r.Gloss), clean(r.POS), clean(r.Gender),
		clean(r.Root), clean(r.Binyan), present, r.Kind,
		clean(strings.Join(r.Forms, "|")), r.Status, r.Check, r.Source, clean(r.ReviewedBy),
	}
}

func (r courseRow) word() speech.CourseWord {
	return speech.CourseWord{
		Rank: r.Rank, Lemma: r.Lemma, Pointed: r.Pointed, Translit: r.Translit,
		Gloss: r.Gloss, POS: r.POS, Gender: r.Gender, Root: r.Root, Binyan: r.Binyan,
		Present: r.Present, Kind: r.Kind,
	}
}

func assignStatus(rows []courseRow) {
	seen := map[string]bool{}
	for i := range rows {
		r := &rows[i]
		if r.Status == "dropped" {
			continue
		}
		if r.Source == "none" && r.Pointed == "" && r.Gloss == "" {
			r.Status, r.Check = "draft", "entry.nodict"
			continue
		}
		if id := speech.CourseCheck(r.word()); id != "" {
			r.Status, r.Check = "draft", id
			continue
		}
		key := speech.LemmaKey(r.Pointed)
		if key != "" && seen[key] {
			r.Status, r.Check = "draft", "course.duplicate"
			continue
		}
		if key != "" {
			seen[key] = true
		}
		if strings.TrimSpace(r.ReviewedBy) == "" {
			r.Status, r.Check = "draft", "review.missing"
			continue
		}
		r.Status, r.Check = "ready", ""
	}
}

func fromPick(lem Lemma, p *picked) courseRow {
	if p == nil {
		return courseRow{Rank: lem.Rank, Lemma: lem.Text, Kind: "word", Source: "none"}
	}
	return courseRow{
		Rank: lem.Rank, Lemma: lem.Text, Pointed: p.Pointed, Translit: p.Translit,
		Gloss: p.Gloss, POS: p.POS, Gender: p.Gender, Root: p.Root, Binyan: p.Binyan,
		Present: p.Present, Kind: "word", Forms: p.Forms, Source: "wiktionary",
	}
}

func applyFixes(base courseRow, fs []Fix) []courseRow {
	if len(fs) == 0 {
		return []courseRow{base}
	}
	switch fs[0].Action {
	case "drop":
		base.Status = "dropped"
		base.Source = "fix"
		return []courseRow{base}
	case "prefix":
		r := courseRow{Rank: base.Rank, Lemma: base.Lemma, Kind: "prefix", POS: "prefix", Source: "fix"}
		fill(&r, fs[0])
		r.Kind, r.POS = "prefix", "prefix"
		r.ReviewedBy = fs[0].ReviewedBy
		return []courseRow{r}
	case "split":
		out := make([]courseRow, 0, len(fs))
		for _, f := range fs {
			r := courseRow{Rank: base.Rank, Lemma: base.Lemma, Kind: "word", Source: "fix"}
			fill(&r, f)
			r.ReviewedBy = f.ReviewedBy
			out = append(out, r)
		}
		return out
	case "fix":
		fill(&base, fs[0])
		base.Source = "fix"
		base.ReviewedBy = fs[0].ReviewedBy
		return []courseRow{base}
	case "keep":
		base.ReviewedBy = fs[0].ReviewedBy
		return []courseRow{base}
	default:
		return []courseRow{base}
	}
}

func fill(r *courseRow, f Fix) {
	if f.Pointed != "" {
		r.Pointed = units.Normalize(f.Pointed)
	}
	if f.Translit != "" {
		r.Translit = strings.TrimSpace(f.Translit)
	}
	if f.Gloss != "" {
		r.Gloss = strings.TrimSpace(f.Gloss)
	}
	if f.POS != "" {
		r.POS = f.POS
	}
	if f.Gender != "" {
		r.Gender = f.Gender
	}
	if f.Root != "" {
		r.Root = f.Root
	}
	if f.Binyan != "" {
		r.Binyan = f.Binyan
	}
	if f.POS != "" && f.POS != "verb" {
		r.Present = false
		r.Root, r.Binyan = "", ""
		if f.Root != "" {
			r.Root = f.Root
		}
		if f.Binyan != "" {
			r.Binyan = f.Binyan
		}
	}
}

// ParseLemmas reads a frequency TSV. A header row is skipped.
func ParseLemmas(s string) ([]Lemma, error) {
	var out []Lemma
	for i, line := range splitLines(s) {
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			return nil, fmt.Errorf("coursegen: list line %d", i+1)
		}
		if i == 0 && parts[0] == "LEMMA" {
			continue
		}
		rank, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("coursegen: list line %d: %w", i+1, err)
		}
		out = append(out, Lemma{Text: parts[0], Rank: rank})
	}
	return out, nil
}

// ParseFixes reads course_fixes.tsv.
func ParseFixes(s string) ([]Fix, error) {
	lines := splitLines(s)
	if len(lines) == 0 {
		return nil, fmt.Errorf("coursegen: fixes are empty")
	}
	var out []Fix
	for i, line := range lines {
		parts := strings.Split(line, "\t")
		if i == 0 && parts[0] == "rank" {
			continue
		}
		if len(parts) != 13 {
			return nil, fmt.Errorf("coursegen: fix line %d has %d fields", i+1, len(parts))
		}
		rank, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("coursegen: fix line %d: %w", i+1, err)
		}
		out = append(out, Fix{
			Rank: rank, Lemma: parts[1], Action: parts[2], Pointed: parts[3],
			Translit: parts[4], Gloss: parts[5], POS: parts[6], Gender: parts[7],
			Root: parts[8], Binyan: parts[9], Reason: parts[10],
			ReviewedBy: parts[11], ReviewedOn: parts[12],
		})
	}
	return out, nil
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// Show writes every dictionary entry for one lemma.
func Show(dict io.Reader, lemma string, w io.Writer) error {
	want := speech.LemmaKey(lemma)
	sc := bufio.NewScanner(dict)
	sc.Buffer(make([]byte, 64*1024), 32*1024*1024)
	n := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e rawEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return err
		}
		if speech.LemmaKey(e.Word) != want {
			continue
		}
		n++
		fmt.Fprintf(w, "word %s pos %s\n", e.Word, e.POS)
		ht := e.head()
		fmt.Fprintf(w, "  pointed %s translit %s gender %s pattern %s\n", ht.arg("wv"), ht.arg("tr"), ht.arg("g"), ht.arg("1"))
		for _, t := range e.Etymology {
			if t.Name == "he-rootbox" {
				fmt.Fprintf(w, "  root %s\n", t.arg("1"))
			}
		}
		for _, s := range e.Senses {
			gloss := ""
			if len(s.Glosses) > 0 {
				gloss = s.Glosses[0]
			}
			fmt.Fprintf(w, "  sense %s tags %s\n", gloss, strings.Join(s.Tags, ","))
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if n == 0 {
		fmt.Fprintf(w, "no entry for %s\n", lemma)
	}
	return nil
}

type picked struct {
	Pointed, Translit, Gloss, POS, Gender, Root, Binyan string
	Present                                             bool
	Forms                                               []string
}

func pick(entries []rawEntry) *picked {
	bestPri := len(posOrder)
	var best *picked
	for i := range entries {
		e := &entries[i]
		pos, ok := posMap[e.POS]
		if !ok {
			continue
		}
		pri := posRank[pos]
		if pri >= bestPri {
			continue
		}
		gloss, ok := usableGloss(e)
		if !ok {
			continue
		}
		p := buildPick(e, pos, gloss)
		best = &p
		bestPri = pri
	}
	return best
}

func buildPick(e *rawEntry, pos, gloss string) picked {
	ht := e.head()
	pointed := firstNonEmpty(ht.arg("wv"), ht.arg("head"), canonical(e), e.Word)
	pointed = units.Normalize(pointed)
	translit := firstNonEmpty(ht.arg("tr"), romanization(e))
	p := picked{
		Pointed: pointed, Translit: translit, Gloss: shapeGloss(gloss), POS: pos,
		Forms: formsOf(e, speech.LemmaKey(e.Word)),
	}
	if pos == "noun" {
		g := ht.arg("g")
		if g != "" && (g[0] == 'm' || g[0] == 'f') {
			p.Gender = g[:1]
		}
	}
	if pos == "verb" {
		p.Binyan = binyanMap[ht.arg("1")]
		for _, t := range e.Etymology {
			if t.Name == "he-rootbox" && t.arg("1") != "" {
				p.Root = t.arg("1")
			}
		}
		if p.Root == "" && p.Binyan == "paal" {
			letters := []rune(speech.LemmaKey(pointed))
			if len(letters) == 3 {
				p.Root = string(letters[0]) + "־" + string(letters[1]) + "־" + string(letters[2])
			}
		}
		for _, f := range e.Forms {
			if hasTag(f.Tags, "present") {
				p.Present = true
				break
			}
		}
	}
	return p
}

var posOrder = []string{
	"pronoun", "preposition", "conjunction", "determiner", "particle",
	"adverb", "verb", "adjective", "numeral", "noun", "interjection",
}

var posMap = map[string]string{
	"noun": "noun", "verb": "verb", "adj": "adjective", "adv": "adverb",
	"prep": "preposition", "pron": "pronoun", "conj": "conjunction",
	"det": "determiner", "article": "determiner", "particle": "particle",
	"intj": "interjection", "num": "numeral",
}

var posRank = func() map[string]int {
	m := map[string]int{}
	for i, p := range posOrder {
		m[p] = i
	}
	return m
}()

var binyanMap = map[string]string{
	"pa": "paal", "nif": "nifal", "pi": "piel", "pu": "pual",
	"hif": "hifil", "huf": "hufal", "hit": "hitpael",
}

var badGloss = regexp.MustCompile(`(?i)^(defective|alternative|archaic|obsolete|misspelling|excessive|plural of|feminine of|masculine of|construct|inflection of|form of|abbreviation of|initialism of|nonstandard|pausal|eye dialect|clipping of)`)

func usableGloss(e *rawEntry) (string, bool) {
	for _, s := range e.Senses {
		if len(s.Glosses) == 0 || jsonPresent(s.FormOf) || jsonPresent(s.AltOf) {
			continue
		}
		skip := false
		for _, tag := range s.Tags {
			switch tag {
			case "form-of", "alt-of", "obsolete", "archaic", "misspelling":
				skip = true
			}
		}
		if skip || badGloss.MatchString(s.Glosses[0]) {
			continue
		}
		return s.Glosses[0], true
	}
	return "", false
}

func shapeGloss(g string) string {
	for {
		i := strings.IndexByte(g, '(')
		if i < 0 {
			break
		}
		j := strings.IndexByte(g[i:], ')')
		if j < 0 {
			break
		}
		g = g[:i] + g[i+j+1:]
	}
	for _, sep := range []string{";", ":", ","} {
		if i := strings.Index(g, sep); i >= 0 {
			g = g[:i]
		}
	}
	g = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(g), "."))
	g = strings.Join(strings.Fields(g), " ")
	if g == "" || g == "I" || strings.HasPrefix(g, "I ") {
		return g
	}
	r := []rune(g)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func formsOf(e *rawEntry, lemma string) []string {
	var out []string
	seen := map[string]bool{lemma: true}
	for _, f := range e.Forms {
		if skipForm(f.Tags) || !hebrewForm(f.Form) {
			continue
		}
		k := speech.LemmaKey(f.Form)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
		if len(out) == 40 {
			break
		}
	}
	return out
}

func skipForm(tags []string) bool {
	for _, tag := range tags {
		switch tag {
		case "romanization", "table-tags", "inflection-template", "class":
			return true
		}
		if strings.HasPrefix(tag, "error") {
			return true
		}
	}
	return false
}

func hebrewForm(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 0x05D0 && r <= 0x05EA:
		case r >= 0x0591 && r <= 0x05C7:
		case r == ' ' || r == 0x05F3 || r == 0x05F4:
		default:
			return false
		}
	}
	return true
}

func canonical(e *rawEntry) string {
	for _, f := range e.Forms {
		if hasTag(f.Tags, "canonical") {
			return f.Form
		}
	}
	return ""
}

func romanization(e *rawEntry) string {
	for _, f := range e.Forms {
		if hasTag(f.Tags, "romanization") {
			return f.Form
		}
	}
	return ""
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func indexDict(r io.Reader) (map[string][]rawEntry, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 32*1024*1024)
	out := map[string][]rawEntry{}
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var e rawEntry
		if err := json.Unmarshal([]byte(text), &e); err != nil {
			return nil, fmt.Errorf("coursegen: dict line %d: %w", line, err)
		}
		k := speech.LemmaKey(e.Word)
		if k == "" {
			continue
		}
		out[k] = append(out[k], e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func jsonPresent(raw json.RawMessage) bool {
	return len(raw) > 0 && string(raw) != "null"
}

type rawEntry struct {
	Word      string        `json:"word"`
	POS       string        `json:"pos"`
	Senses    []rawSense    `json:"senses"`
	Heads     []rawTemplate `json:"head_templates"`
	Forms     []rawForm     `json:"forms"`
	Etymology []rawTemplate `json:"etymology_templates"`
}

func (e *rawEntry) head() rawTemplate {
	if len(e.Heads) == 0 {
		return rawTemplate{}
	}
	return e.Heads[0]
}

type rawSense struct {
	Glosses []string        `json:"glosses"`
	Tags    []string        `json:"tags"`
	FormOf  json.RawMessage `json:"form_of"`
	AltOf   json.RawMessage `json:"alt_of"`
}

type rawForm struct {
	Form string   `json:"form"`
	Tags []string `json:"tags"`
}

type rawTemplate struct {
	Name string                     `json:"name"`
	Args map[string]json.RawMessage `json:"args"`
}

func (t rawTemplate) arg(key string) string {
	raw, ok := t.Args[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}
