package plans

import (
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/jsstr"
)

const (
	msgNotJSON     = "the plan is not a JSON document"
	msgHTML        = "markup is not allowed in plan text"
	msgImage       = "images are allowed only on introduction words"
	msgFluencyList = "a fluency-pass block has no word list"
	msgRomanized   = "the correct answer has no Hebrew letter"
)

// ruleSeverity is the registry severity used when a finding does not override it.
var ruleSeverity = map[string]string{
	"schema": "block", "N1": "block", "N2": "warn", "N3": "warn", "N5": "block", "N6": "warn",
	"N7": "warn", "N8": "block", "N9": "block", "N10": "block", "N11": "block", "N12": "block",
	"N13": "block", "N14": "warn", "N15": "warn", "N16": "inform", "N17": "inform", "N18": "warn",
	"N19": "block", "N20": "block", "N21": "block", "N22": "block", "N23": "warn",
	"later-context": "warn", "later-checkpoint": "warn", "recall-day": "inform",
	"frequency-band": "inform", "pattern-focus": "warn", "pointed-word": "warn",
	"romanized-answer": "block", "questions": "inform", "queue-order": "inform",
	"meaning": "block", "image-placement": "block", "picture-on-recall": "warn",
	"closed-set": "warn", "html": "block", "audio-path": "block",
	"pointing-form": "warn", "field-language": "warn", "lexicon-form": "warn",
	"pointing-consistency": "warn", "audio-stale": "warn",
}

// Lexicon is the N13/N14 lookup. Find is called with an exactKey.
// A non-object row is treated as missing.
type Lexicon interface {
	Find(exactKey string) (Value, bool)
}

// StrippedLister is optional. lexicon-form calls it; a missing method or an
// error (the Node function threw) produces no lexicon-form finding.
type StrippedLister interface {
	ListByStrippedKeys(keys []string) ([]Value, error)
}

// SnapshotMode says how the published copy was passed to checkHebrew.
type SnapshotMode int

const (
	// SnapNone means no snapshot argument.
	SnapNone SnapshotMode = iota
	// SnapRaw is a string, including the empty string. JSON text is parsed.
	SnapRaw
	// SnapValue is an already-parsed JSON value.
	SnapValue
)

// Options carries the lexicon and the published snapshot. Nil is no options.
type Options struct {
	Lexicon      Lexicon
	SnapshotMode SnapshotMode
	SnapshotRaw  string
	Snapshot     Value
}

// Finding is one validator or Hebrew-check result.
// Field and Fix are present only for the Hebrew checks.
type Finding struct {
	Rule     string
	Severity string
	Path     string
	Message  string
	HasField bool
	Field    string
	Fix      *Fix
}

// Fix is the suggested part-operation on a Hebrew finding.
type Fix struct {
	Action        string
	Path          string
	Field         string
	Suggested     *string
	HasCandidates bool
	Candidates    []string
	Text          *string
	Clip          *string
	Note          *string
}

// Result is validatePlan's return value. Empty slices are empty arrays.
type Result struct {
	OK       bool
	Blocks   []Finding
	Warnings []Finding
	Informs  []Finding
}

// ValidatePlan matches server/plans/validate.js validatePlan.
// doc is either the JSON text (string) or a parsed Value.
// A string is measured from JSON.stringify(JSON.parse(doc)).
// A Value is measured from the first Stringify, matching object input.
func ValidatePlan(doc any, opt *Options) Result {
	f := newFindings()
	plan, bytes, errMsg := canonical(doc)
	if errMsg != "" {
		f.add("schema", "", errMsg, "")
		return f.result()
	}
	schema := loadSchema()
	if !plan.IsObject() {
		f.add("schema", "", "a plan must be a JSON object", "")
		return f.result()
	}
	sv, _ := plan.Get("schemaVersion")
	svStr := ""
	if sv.K == KindStr {
		svStr = sv.S
	}
	if !contains(schema.versions, svStr) {
		f.add("schema", "/schemaVersion", "schemaVersion must be "+strings.Join(schema.versions, " or ")+"; other versions are not parsed", "")
		return f.result()
	}
	ctx := valContext{
		v110:     svStr == v110 || svStr == v120,
		v120:     svStr == v120,
		recipes:  schema.recipes,
		keyLimit: schema.keyLimit,
	}
	ctx.lex, ctx.lexAvailable = lexiconOf(opt)
	lessons := lessonsOf(plan, ctx)
	if !checkSizes(lessons, bytes, f) {
		return f.result()
	}
	checkSchema(plan, schema, f)
	checkStrings(plan, schema, f)
	checkLessonRules(plan, lessons, ctx, f)
	checkEstimates(lessons, ctx, f)
	checkHebrewRules(plan, opt, f)
	return f.result()
}

func checkHebrewRules(plan Value, opt *Options, f *findings) {
	for _, item := range CheckHebrew(plan, opt) {
		sev := item.Severity
		if sev == "" {
			sev = ruleSeverity[item.Rule]
		}
		f.addExtra(item.Rule, item.Path, item.Message, sev, true, item.Field, item.Fix)
	}
}

type valContext struct {
	v110         bool
	v120         bool
	recipes      map[string]struct{}
	keyLimit     int
	lex          *memoLex
	lexAvailable bool
}

type memoLex struct {
	src  Lexicon
	rows map[string]Value
	have map[string]bool
}

func lexiconOf(opt *Options) (*memoLex, bool) {
	if opt == nil || opt.Lexicon == nil {
		return &memoLex{}, false
	}
	return &memoLex{src: opt.Lexicon, rows: map[string]Value{}, have: map[string]bool{}}, true
}

func (m *memoLex) find(key string) Value {
	if m == nil || m.src == nil {
		return VNull()
	}
	norm := exactKey(key)
	if m.have[norm] {
		return m.rows[norm]
	}
	row, ok := m.src.Find(norm)
	if !ok || !row.IsObject() {
		row = VNull()
	}
	m.have[norm] = true
	m.rows[norm] = row
	return row
}

func canonical(doc any) (Value, int, string) {
	var source string
	asString := false
	switch d := doc.(type) {
	case string:
		source = d
		asString = true
	case Value:
		source = Stringify(d)
	default:
		return Value{}, 0, msgNotJSON
	}
	plan, err := Parse(source)
	if err != nil {
		return Value{}, 0, msgNotJSON
	}
	compact := source
	if asString {
		compact = Stringify(plan)
	}
	return plan, len(compact), ""
}

type findings struct {
	seen  map[string]struct{}
	items []Finding
}

func newFindings() *findings {
	return &findings{seen: map[string]struct{}{}}
}

func (f *findings) add(rule, at, message, severity string) {
	f.addExtra(rule, at, message, severity, false, "", nil)
}

func (f *findings) addExtra(rule, at, message, severity string, extra bool, field string, fix *Fix) {
	key := rule + "\x00" + at
	if extra {
		key += "\x00" + message
	}
	if _, ok := f.seen[key]; ok {
		return
	}
	f.seen[key] = struct{}{}
	if severity == "" {
		severity = ruleSeverity[rule]
	}
	item := Finding{Rule: rule, Severity: severity, Path: at, Message: message}
	if extra {
		item.HasField = true
		item.Field = field
		item.Fix = fix
	}
	f.items = append(f.items, item)
}

func (f *findings) result() Result {
	r := Result{Blocks: []Finding{}, Warnings: []Finding{}, Informs: []Finding{}}
	for _, item := range f.items {
		switch item.Severity {
		case "block":
			r.Blocks = append(r.Blocks, item)
		case "warn":
			r.Warnings = append(r.Warnings, item)
		case "inform":
			r.Informs = append(r.Informs, item)
		}
	}
	r.OK = len(r.Blocks) == 0
	return r
}

type frame struct {
	value  Value
	at     string
	hasKey bool
	key    string
	parent string
}

func checkStrings(plan Value, schema *loadedSchema, f *findings) {
	stack := []frame{{value: plan, at: ""}}
	for len(stack) > 0 {
		fr := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if fr.hasKey {
			checkKey(plan, schema, fr.key, fr.parent, fr.at, f)
		}
		switch fr.value.K {
		case KindStr:
			if schema.markup.MatchString(fr.value.S) {
				f.add("html", fr.at, msgHTML, "")
			}
		case KindArr:
			for i := len(fr.value.A) - 1; i >= 0; i-- {
				stack = append(stack, frame{value: fr.value.A[i], at: pointer(fr.at, strconvI(i)), parent: fr.at})
			}
		case KindObj:
			for i := len(fr.value.O) - 1; i >= 0; i-- {
				p := fr.value.O[i]
				stack = append(stack, frame{
					value: p.Val, at: pointer(fr.at, p.Key), hasKey: true, key: p.Key, parent: fr.at,
				})
			}
		}
	}
}

func checkKey(plan Value, schema *loadedSchema, key, parent, at string, f *findings) {
	if schema.markup.MatchString(key) {
		f.add("html", at, msgHTML, "")
	}
	if jsstr.CodePoints(key) > schema.textLimit {
		f.add("N11", at, "key must be at most "+strconvI(schema.textLimit)+" characters", "")
	}
	if reGlossMap.MatchString(parent) {
		return
	}
	if typeItAt(plan, parent) || typeItAt(plan, at) {
		return
	}
	if key == "image" && !introWordAt(plan, parent) {
		f.add("image-placement", at, msgImage, "")
	}
	if key == "mediaId" {
		imageAt := ""
		has := false
		if strings.HasSuffix(parent, "/image") {
			imageAt = parent[:len(parent)-len("/image")]
			has = true
		} else if strings.HasSuffix(parent, "/picture") {
			imageAt = parent[:len(parent)-len("/picture")]
			has = true
		}
		if !has || !introWordAt(plan, imageAt) {
			f.add("image-placement", at, msgImage, "")
		}
	}
}

func checkSizes(lessons []lessonEntry, planBytes int, f *findings) bool {
	const planLimit = 512 * 1024
	const lessonLimit = 128 * 1024
	const blockLimit = 32 * 1024
	if planBytes > planLimit {
		f.add("N9", "", "the plan is "+strconvI(planBytes)+" bytes of JSON; N9 allows "+strconvI(planLimit), "")
	}
	for _, entry := range lessons {
		lessonBytes := len(Stringify(entry.lesson))
		if lessonBytes > lessonLimit {
			f.add("N9", entry.at, "the lesson is "+strconvI(lessonBytes)+" bytes of JSON; N9 allows "+strconvI(lessonLimit), "")
		}
		for _, item := range entry.blocks {
			blockBytes := len(Stringify(item.raw))
			if blockBytes > blockLimit {
				f.add("N9", item.at, "the block is "+strconvI(blockBytes)+" bytes of JSON; N9 allows "+strconvI(blockLimit), "")
			}
		}
	}
	return planBytes <= planLimit
}

type lessonEntry struct {
	lesson     Value
	u          int
	at         string
	index      int
	blocks     []blockEntry
	recipe     string
	newWords   []wordRef
	verseWords []wordRef
	recycled   []wordRef
	focus      []focusRef
}

type blockEntry struct {
	raw   Value
	block Value
	typ   string
	at    string
}

type wordRef struct {
	key string
	raw string
	at  string
}

type focusRef struct {
	raw string
	at  string
}

func lessonsOf(plan Value, ctx valContext) []lessonEntry {
	var lessons []lessonEntry
	units, _ := plan.Get("units")
	for u, unit := range listOf(units) {
		if !unit.IsObject() {
			continue
		}
		ls, _ := unit.Get("lessons")
		for l, lesson := range listOf(ls) {
			if !lesson.IsObject() {
				continue
			}
			at := "/units/" + strconvI(u) + "/lessons/" + strconvI(l)
			var blocks []blockEntry
			bs, _ := lesson.Get("blocks")
			for b, block := range listOf(bs) {
				be := blockEntry{raw: block, at: at + "/blocks/" + strconvI(b)}
				if block.IsObject() {
					be.block = block
					if t, ok := block.Get("type"); ok && t.K == KindStr {
						be.typ = t.S
					}
				} else {
					be.block = VObj()
				}
				blocks = append(blocks, be)
			}
			nw, _ := lesson.Get("newWords")
			newWords := declared(listOf(nw), at+"/newWords")
			recVals := []Value{}
			recs, _ := lesson.Get("recycles")
			for _, item := range listOf(recs) {
				if item.IsObject() {
					if w, ok := item.Get("word"); ok {
						recVals = append(recVals, w)
					} else {
						recVals = append(recVals, VNull())
					}
				} else {
					recVals = append(recVals, VNull())
				}
			}
			entry := lessonEntry{
				lesson: lesson, u: u, at: at, index: len(lessons), blocks: blocks,
				recipe:   recipeOf(lesson, ctx),
				newWords: newWords,
				recycled: declared(recVals, at+"/recycles"),
				focus:    focusOf(blocks),
			}
			if ctx.v110 {
				entry.verseWords = verseWordsOf(blocks, newWords)
			}
			lessons = append(lessons, entry)
		}
	}
	return lessons
}

func recipeOf(lesson Value, ctx valContext) string {
	if !ctx.v110 {
		return ""
	}
	r, ok := lesson.Get("recipe")
	if !ok || r.K != KindStr {
		return ""
	}
	if _, known := ctx.recipes[r.S]; !known {
		return ""
	}
	return r.S
}

func verseWordsOf(blocks []blockEntry, newWords []wordRef) []wordRef {
	seen := map[string]struct{}{}
	for _, w := range newWords {
		seen[w.key] = struct{}{}
	}
	var words []wordRef
	for _, item := range blocks {
		if item.typ != "verse-reading" {
			continue
		}
		arr, _ := item.block.Get("newWords")
		for i, value := range listOf(arr) {
			if value.K != KindStr {
				continue
			}
			key := exactKey(value.S)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			words = append(words, wordRef{key: key, raw: value.S, at: item.at + "/newWords/" + strconvI(i)})
		}
	}
	return words
}

func declared(values []Value, at string) []wordRef {
	seen := map[string]struct{}{}
	var out []wordRef
	for i, value := range values {
		if value.K != KindStr {
			continue
		}
		key := exactKey(value.S)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, wordRef{key: key, raw: value.S, at: pointer(at, strconvI(i))})
	}
	return out
}

func focusOf(blocks []blockEntry) []focusRef {
	var focus []focusRef
	for _, item := range blocks {
		if item.typ != "introduction" {
			continue
		}
		pat, ok := item.block.Get("pattern")
		if !ok || !pat.IsObject() {
			continue
		}
		rules, _ := pat.Get("rules")
		for r, rule := range listOf(rules) {
			if !rule.IsObject() {
				continue
			}
			arr, _ := rule.Get("focus")
			for i, value := range listOf(arr) {
				if value.K != KindStr {
					continue
				}
				focus = append(focus, focusRef{
					raw: value.S,
					at:  item.at + "/pattern/rules/" + strconvI(r) + "/focus/" + strconvI(i),
				})
			}
		}
	}
	return focus
}

func (r Result) value() Value {
	return VObj(
		Pair{Key: "ok", Val: VBool(r.OK)},
		Pair{Key: "blocks", Val: findingsValue(r.Blocks)},
		Pair{Key: "warnings", Val: findingsValue(r.Warnings)},
		Pair{Key: "informs", Val: findingsValue(r.Informs)},
	)
}

func findingsValue(items []Finding) Value {
	arr := make([]Value, len(items))
	for i, it := range items {
		arr[i] = it.value()
	}
	return VArr(arr...)
}

func (it Finding) value() Value {
	pairs := []Pair{
		{Key: "rule", Val: VStr(it.Rule)},
		{Key: "severity", Val: VStr(it.Severity)},
		{Key: "path", Val: VStr(it.Path)},
		{Key: "message", Val: VStr(it.Message)},
	}
	if it.HasField {
		pairs = append(pairs, Pair{Key: "field", Val: VStr(it.Field)})
	}
	if it.Fix != nil {
		pairs = append(pairs, Pair{Key: "fix", Val: it.Fix.value()})
	}
	return VObj(pairs...)
}

func (fx Fix) value() Value {
	pairs := []Pair{
		{Key: "action", Val: VStr(fx.Action)},
		{Key: "path", Val: VStr(fx.Path)},
		{Key: "field", Val: VStr(fx.Field)},
	}
	if fx.Suggested != nil {
		pairs = append(pairs, Pair{Key: "suggested", Val: VStr(*fx.Suggested)})
	}
	if fx.HasCandidates {
		arr := make([]Value, len(fx.Candidates))
		for i, c := range fx.Candidates {
			arr[i] = VStr(c)
		}
		pairs = append(pairs, Pair{Key: "candidates", Val: VArr(arr...)})
	}
	if fx.Text != nil {
		pairs = append(pairs, Pair{Key: "text", Val: VStr(*fx.Text)})
	}
	if fx.Clip != nil {
		pairs = append(pairs, Pair{Key: "clip", Val: VStr(*fx.Clip)})
	}
	if fx.Note != nil {
		pairs = append(pairs, Pair{Key: "note", Val: VStr(*fx.Note)})
	}
	return VObj(pairs...)
}

// JSON returns the canonical JSON.stringify form of a validation result.
func (r Result) JSON() string { return Stringify(r.value()) }
