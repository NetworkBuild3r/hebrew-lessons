package plans

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/jsstr"
)

const (
	draft07     = "http://json-schema.org/draft-07/schema#"
	definitions = "#/definitions/"
	v100        = "1.0.0"
	v110        = "1.1.0"
	v120        = "1.2.0"
)

// jsSpaceContents is the character set of JavaScript \s in Node 22, without
// the surrounding brackets, so it can sit inside or outside a class.
const jsSpaceContents = `\t\n\v\f\r \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

var schemaKeywords = map[string]struct{}{
	"$schema": {}, "$id": {}, "$ref": {}, "title": {}, "description": {}, "definitions": {},
	"type": {}, "const": {}, "enum": {}, "properties": {}, "required": {}, "additionalProperties": {}, "maxProperties": {},
	"items": {}, "minItems": {}, "maxItems": {}, "minLength": {}, "maxLength": {}, "pattern": {}, "minimum": {}, "maximum": {},
	"allOf": {}, "oneOf": {}, "not": {}, "if": {}, "then": {}, "else": {},
}

var schemaTypes = map[string]struct{}{
	"null": {}, "boolean": {}, "object": {}, "array": {}, "number": {}, "integer": {}, "string": {},
}

var markupDefs = map[string]struct{}{
	"plainText": {}, "altText": {}, "text80": {}, "text120": {}, "text200": {}, "text300": {}, "text600": {}, "text1500": {},
	"tableCell": {}, "lexiconKey": {},
}

var n10Lists = mustCompileAll(
	`^/units$`,
	`^/units/\d+/lessons$`,
	`^/units/\d+/lessons/\d+/blocks$`,
	`^/units/\d+/lessons/\d+/blocks/\d+/words$`,
	`^/units/\d+/lessons/\d+/blocks/\d+/items$`,
	`^/units/\d+/lessons/\d+/blocks/\d+/items/\d+/choices$`,
	`^/units/\d+/lessons/\d+/blocks/\d+/items/\d+/words$`,
	`^/units/\d+/lessons/\d+/blocks/\d+/items/\d+/alt$`,
	`^/units/\d+/lessons/\d+/blocks/\d+/items/\d+/accept$`,
	`^/units/\d+/lessons/\d+/blocks/\d+/useWords$`,
	`^/units/\d+/lessons/\d+/blocks/\d+/homework$`,
)

var (
	reWordMeaning  = regexp.MustCompile(`^/units/\d+/lessons/\d+/blocks/\d+/words/\d+/meaning$`)
	reBlockChild   = regexp.MustCompile(`^/units/(\d+)/lessons/(\d+)/blocks/(\d+)/([^/]+)$`)
	reIntroWord    = regexp.MustCompile(`^/units/(\d+)/lessons/(\d+)/blocks/(\d+)/words/\d+$`)
	reVerseSource  = regexp.MustCompile(`^/units/(\d+)/lessons/(\d+)/blocks/(\d+)/ref/source$`)
	reCellAt       = regexp.MustCompile(`^/units/(\d+)/lessons/(\d+)/blocks/(\d+)/cells/(\d+)/cell$`)
	reTense        = regexp.MustCompile(`^/units/(\d+)/lessons/(\d+)/blocks/(\d+)/tense$`)
	reTypeIt       = regexp.MustCompile(`^/units/(\d+)/lessons/(\d+)/blocks/(\d+)`)
	reLessonPrefix = regexp.MustCompile(`^/units/\d+/lessons/\d+`)
	reCellsFormKey = regexp.MustCompile(`/cells/\d+/formKey$`)
	reGlossMap     = regexp.MustCompile(`^/units/\d+/lessons/\d+/blocks/\d+/glosses$`)
)

func mustCompileAll(patterns ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(patterns))
	for i, p := range patterns {
		out[i] = regexp.MustCompile(p)
	}
	return out
}

type loadedSchema struct {
	root      Value
	patterns  map[string]*regexp.Regexp
	versions  []string
	markup    *regexp.Regexp
	textLimit int
	recipes   map[string]struct{}
	v110Types map[string]struct{}
	v120Types map[string]struct{}
	cellCodes map[string]struct{}
	keyLimit  int
	audioPath *regexp.Regexp
	audioRaw  string
}

type schemaScope struct {
	root     Value
	patterns map[string]*regexp.Regexp
	def      string
}

type schemaErr struct {
	path     string
	keyword  string
	def      string
	property string
	message  string
}

var (
	schemaOnce sync.Once
	schemaVal  *loadedSchema
	schemaErrL error
)

func loadSchema() *loadedSchema {
	schemaOnce.Do(func() {
		schemaVal, schemaErrL = readSchema()
	})
	if schemaErrL != nil {
		panic(schemaErrL)
	}
	return schemaVal
}

func schemaFile() string {
	if p := os.Getenv("HEBREW_SCHEMA"); p != "" {
		return p
	}
	const rel = "docs/decisions/ADR-0003-assets/lesson-plan.schema.json"
	starts := []string{"."}
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	for _, start := range starts {
		dir := start
		for range 8 {
			cand := filepath.Join(dir, rel)
			if st, err := os.Stat(cand); err == nil && !st.IsDir() {
				return cand
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return filepath.Join("..", rel)
}

func readSchema() (*loadedSchema, error) {
	raw, err := os.ReadFile(schemaFile())
	if err != nil {
		return nil, fmt.Errorf("lesson-plan schema: %w", err)
	}
	root, err := Parse(string(raw))
	if err != nil || !root.IsObject() {
		return nil, fmt.Errorf("lesson-plan schema is not JSON Schema draft-07")
	}
	sch, _ := root.Get("$schema")
	if sch.K != KindStr || sch.S != draft07 {
		return nil, fmt.Errorf("lesson-plan schema is not JSON Schema draft-07")
	}
	patterns := map[string]*regexp.Regexp{}
	if err := assertSupported(root, root, "#", patterns); err != nil {
		return nil, err
	}
	props, _ := root.Get("properties")
	version, _ := props.Get("schemaVersion")
	var versions []string
	if c, ok := version.Get("const"); ok {
		versions = []string{jsonScalarString(c)}
	} else if en, ok := version.Get("enum"); ok {
		for _, item := range listOf(en) {
			versions = append(versions, jsonScalarString(item))
		}
	}
	defs, _ := root.Get("definitions")
	text, _ := defs.Get("plainText")
	notNode, _ := text.Get("not")
	markupPat, _ := notNode.StrKey("pattern")
	markup := patterns[markupPat]
	maxLen, hasMax := text.Get("maxLength")
	if len(versions) == 0 || markup == nil || !hasMax || !isJSInteger(maxLen.N) {
		return nil, fmt.Errorf("lesson-plan schema lacks schemaVersion or the plainText definition")
	}
	recipeDef, _ := defs.Get("recipe")
	var recipes []string
	if en, ok := recipeDef.Get("enum"); ok {
		for _, item := range listOf(en) {
			if item.K == KindStr {
				recipes = append(recipes, item.S)
			}
		}
	}
	v110Types := notEnum(defs, "v110BlockType")
	v120Types := notEnum(defs, "v120BlockType")
	keyDef, _ := defs.Get("lexiconKey")
	cellDef, _ := defs.Get("cellCode")
	var cells []string
	if en, ok := cellDef.Get("enum"); ok {
		for _, item := range listOf(en) {
			if item.K == KindStr {
				cells = append(cells, item.S)
			}
		}
	}
	audioDef, _ := defs.Get("audioPath")
	audioRaw, _ := audioDef.StrKey("pattern")
	audio := patterns[audioRaw]
	keyMax, hasKey := keyDef.Get("maxLength")
	if !contains(versions, v100) || !contains(versions, v110) || !contains(versions, v120) || len(recipes) == 0 ||
		len(v110Types) == 0 || len(v120Types) == 0 || len(cells) == 0 || !keyDef.IsObject() ||
		!hasKey || !isJSInteger(keyMax.N) || audio == nil {
		return nil, fmt.Errorf("lesson-plan schema lacks version, recipe, block-type, cell, lexicon, or audioPath definitions")
	}
	return &loadedSchema{
		root:      root,
		patterns:  patterns,
		versions:  versions,
		markup:    markup,
		textLimit: int(maxLen.N),
		recipes:   stringSet(recipes),
		v110Types: stringSet(v110Types),
		v120Types: stringSet(v120Types),
		cellCodes: stringSet(cells),
		keyLimit:  int(keyMax.N),
		audioPath: audio,
		audioRaw:  audioRaw,
	}, nil
}

func (v Value) StrKey(key string) (string, bool) {
	child, ok := v.Get(key)
	if !ok || child.K != KindStr {
		return "", false
	}
	return child.S, true
}

func notEnum(defs Value, name string) []string {
	def, ok := defs.Get(name)
	if !ok {
		return nil
	}
	n, ok := def.Get("not")
	if !ok {
		return nil
	}
	en, ok := n.Get("enum")
	if !ok {
		return nil
	}
	var out []string
	for _, item := range listOf(en) {
		if item.K == KindStr {
			out = append(out, item.S)
		}
	}
	return out
}

func stringSet(items []string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, s := range items {
		m[s] = struct{}{}
	}
	return m
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func jsonScalarString(v Value) string {
	if v.K == KindStr {
		return v.S
	}
	return Stringify(v)
}

func assertSupported(node, root Value, where string, patterns map[string]*regexp.Regexp) error {
	if !node.IsObject() {
		return fmt.Errorf("schema node at %s is not an object", where)
	}
	for _, p := range node.O {
		if _, ok := schemaKeywords[p.Key]; !ok {
			return fmt.Errorf("schema keyword %s at %s is not implemented", p.Key, where)
		}
	}
	if _, ok := node.Get("$ref"); ok {
		if err := assertLocalRef(node, root, where); err != nil {
			return err
		}
	}
	if t, ok := node.Get("type"); ok {
		types := []Value{t}
		if t.K == KindArr {
			types = t.A
		}
		for _, typ := range types {
			if typ.K != KindStr {
				return fmt.Errorf("schema type at %s is not implemented", where)
			}
			if _, ok := schemaTypes[typ.S]; !ok {
				return fmt.Errorf("schema type %s at %s is not implemented", typ.S, where)
			}
		}
	}
	if pat, ok := node.Get("pattern"); ok {
		if pat.K != KindStr {
			return fmt.Errorf("schema pattern at %s is not a string", where)
		}
		re, err := compileJSPattern(pat.S)
		if err != nil {
			return fmt.Errorf("schema pattern %s at %s: %w", pat.S, where, err)
		}
		patterns[pat.S] = re
	}
	if items, ok := node.Get("items"); ok && items.K == KindArr {
		return fmt.Errorf("tuple items at %s are not implemented", where)
	}
	for _, key := range []string{"items", "not", "if", "then", "else"} {
		if child, ok := node.Get(key); ok {
			if err := assertSupported(child, root, where+"/"+key, patterns); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"allOf", "oneOf"} {
		if arr, ok := node.Get(key); ok {
			for i, child := range listOf(arr) {
				if err := assertSupported(child, root, fmt.Sprintf("%s/%s/%d", where, key, i), patterns); err != nil {
					return err
				}
			}
		}
	}
	for _, key := range []string{"properties", "definitions"} {
		child, ok := node.Get(key)
		if !ok || !child.IsObject() {
			continue
		}
		for _, p := range child.O {
			if err := assertSupported(p.Val, root, where+"/"+key+"/"+p.Key, patterns); err != nil {
				return err
			}
		}
	}
	if add, ok := node.Get("additionalProperties"); ok && add.IsObject() {
		if err := assertSupported(add, root, where+"/additionalProperties", patterns); err != nil {
			return err
		}
	}
	return nil
}

func assertLocalRef(node, root Value, where string) error {
	seen := map[string]struct{}{}
	target := node
	for target.IsObject() {
		ref, ok := target.Get("$ref")
		if !ok {
			break
		}
		refStr := ""
		if ref.K == KindStr {
			refStr = ref.S
		}
		name := ""
		if strings.HasPrefix(refStr, definitions) {
			name = refStr[len(definitions):]
		}
		defs, _ := root.Get("definitions")
		if name == "" || !defs.IsObject() || !defs.Has(name) {
			return fmt.Errorf("schema $ref %s at %s is not a local, acyclic definition", refStr, where)
		}
		if _, ok := seen[name]; ok {
			return fmt.Errorf("schema $ref %s at %s is not a local, acyclic definition", refStr, where)
		}
		seen[name] = struct{}{}
		target, _ = defs.Get(name)
	}
	return nil
}

// compileJSPattern compiles a JavaScript RegExp with the u flag.
// \uXXXX becomes RE2 \x{XXXX}. \s becomes JavaScript's whitespace class, which
// is wider than RE2 \s. A trailing $ becomes \z because RE2's $ also matches
// before a final newline and Node 22's $ does not.
func compileJSPattern(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(translateJSPattern(pattern))
}

func translateJSPattern(p string) string {
	var b strings.Builder
	inClass := false
	for i := 0; i < len(p); {
		c := p[i]
		if c == '\\' && i+1 < len(p) {
			n := p[i+1]
			if n == 'u' {
				if cp, size, ok := readUEscape(p[i:]); ok {
					fmt.Fprintf(&b, `\x{%x}`, cp)
					i += size
					continue
				}
			}
			if n == 's' || n == 'S' {
				contents := jsSpaceContents
				if n == 's' {
					if inClass {
						b.WriteString(contents)
					} else {
						b.WriteString("[" + contents + "]")
					}
				} else if inClass {
					b.WriteString(contents) // unreachable in this schema; kept literal-safe
				} else {
					b.WriteString("[^" + contents + "]")
				}
				i += 2
				continue
			}
			b.WriteByte('\\')
			b.WriteByte(n)
			i += 2
			continue
		}
		if c == '[' && !inClass {
			inClass = true
		} else if c == ']' && inClass {
			inClass = false
		} else if c == '$' && !inClass {
			b.WriteString(`\z`)
			i++
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

func readUEscape(s string) (rune, int, bool) {
	// s starts with \u
	if len(s) >= 6 && s[0] == '\\' && s[1] == 'u' {
		if s[2] == '{' {
			j := 3
			var n rune
			digits := 0
			for j < len(s) && s[j] != '}' {
				d, ok := hexVal(s[j])
				if !ok || digits == 6 {
					return 0, 0, false
				}
				n = n<<4 | d
				digits++
				j++
			}
			if digits == 0 || j >= len(s) || s[j] != '}' || n > 0x10FFFF {
				return 0, 0, false
			}
			return n, j + 1, true
		}
		if len(s) >= 6 {
			var n rune
			for k := 2; k < 6; k++ {
				d, ok := hexVal(s[k])
				if !ok {
					return 0, 0, false
				}
				n = n<<4 | d
			}
			return n, 6, true
		}
	}
	return 0, 0, false
}

func hexVal(c byte) (rune, bool) {
	switch {
	case c >= '0' && c <= '9':
		return rune(c - '0'), true
	case c >= 'a' && c <= 'f':
		return rune(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return rune(c-'A') + 10, true
	default:
		return 0, false
	}
}

func checkSchema(plan Value, schema *loadedSchema, f *findings) {
	var errs []schemaErr
	check(plan, schema.root, schemaScope{root: schema.root, patterns: schema.patterns}, "", &errs)
	for _, err := range errs {
		rule, message := ruleForSchemaError(err, plan, schema)
		f.add(rule, err.path, message, "")
	}
}

func ruleForSchemaError(err schemaErr, plan Value, schema *loadedSchema) (string, string) {
	at := err.path
	v11 := false
	if sv, ok := plan.Get("schemaVersion"); ok && sv.K == KindStr && (sv.S == v110 || sv.S == v120) {
		v11 = true
	}
	if err.def == "v110Only" {
		return "schema", leaf(at) + " needs schemaVersion 1.1.0"
	}
	if err.def == "v110BlockType" {
		return "schema", "this block type needs schemaVersion 1.1.0"
	}
	if err.def == "v120Only" {
		if leaf(at) == "tense" {
			if m := reTense.FindStringSubmatch(at); m != nil {
				u, l, b := atoi(m[1]), atoi(m[2]), atoi(m[3])
				if block, ok := blockAt(plan, u, l, b); ok {
					if t, ok := block.Get("tense"); ok && t.K == KindStr && t.S == "future" {
						return "schema", "future tense needs schemaVersion 1.2.0"
					}
				}
			}
		}
		return "schema", leaf(at) + " needs schemaVersion 1.2.0"
	}
	if err.def == "v120BlockType" {
		return "schema", "this block type needs schemaVersion 1.2.0"
	}
	if err.def == "acceptString" && err.keyword == "not" {
		return "schema", "accept must not include vowel points or cantillation"
	}
	if err.def == "acceptString" && err.keyword == "pattern" {
		return "schema", "accept must include at least one Hebrew letter"
	}
	if err.keyword == "not" {
		if _, ok := markupDefs[err.def]; ok {
			return "html", msgHTML
		}
	}
	if err.def == "cellCode" || err.def == "presentCell" || err.def == "pastCell" || err.def == "futureCell" {
		cell := cellAt(plan, at)
		_, in := schema.cellCodes[cell]
		if err.def == "cellCode" || !in {
			return "schema", "not a present or past cell code"
		}
		if v11 {
			return "N13", "this cell does not belong to the table tense"
		}
		return "schema", "this cell does not belong to the table tense"
	}
	if v11 && (err.keyword == "required" || err.keyword == "minLength") && verseSourceAt(plan, at) {
		return "N19", "a verse-reading needs a non-empty ref.source naming its edition"
	}
	if err.def == "lexiconKey" && err.keyword == "pattern" {
		return "schema", "a lexicon key needs a Hebrew letter"
	}
	if err.keyword == "required" && err.property == "alt" {
		return "N8", "an image needs alt text"
	}
	if err.def == "altText" {
		return "N8", "alt " + err.message
	}
	if err.def == "audioPath" {
		if err.keyword == "pattern" {
			return "audio-path", "audio path must be a relative .mp3 path with no scheme or leading slash"
		}
		return "audio-path", "audio path " + err.message
	}
	if err.def == "plainText" && err.keyword == "maxLength" {
		return "N11", err.message
	}
	if reWordMeaning.MatchString(at) {
		if err.keyword == "required" {
			return "meaning", "every word needs a first-language meaning"
		}
		return "meaning", "meaning " + err.message
	}
	if err.keyword == "maxItems" {
		for _, re := range n10Lists {
			if re.MatchString(at) {
				return "N10", err.message
			}
		}
	}
	if err.keyword == "additionalProperties" {
		if err.property == "image" || err.property == "mediaId" {
			return "image-placement", msgImage
		}
		if m := reBlockChild.FindStringSubmatch(at); m != nil && (m[4] == "words" || m[4] == "newWords") {
			if block, ok := blockAt(plan, atoi(m[1]), atoi(m[2]), atoi(m[3])); ok {
				if t, ok := block.Get("type"); ok && t.K == KindStr && t.S == "fluency-pass" {
					return "N12", msgFluencyList
				}
			}
		}
	}
	if err.keyword == "pattern" && strings.HasSuffix(at, "/mediaId") {
		return "schema", "media id must be an opaque slug, not a URL, path, or filename"
	}
	return "schema", err.message
}

func leaf(at string) string {
	i := strings.LastIndex(at, "/")
	return at[i+1:]
}

func check(instance, node Value, ctx schemaScope, at string, errs *[]schemaErr) {
	schema, scope := resolve(node, ctx)
	if c, ok := schema.Get("const"); ok && !sameJSON(instance, c) {
		push(errs, at, "const", scope.def, "must be "+Stringify(c), "")
		return
	}
	if en, ok := schema.Get("enum"); ok && en.K == KindArr {
		match := false
		var parts []string
		for _, item := range en.A {
			parts = append(parts, Stringify(item))
			if sameJSON(instance, item) {
				match = true
			}
		}
		if !match {
			push(errs, at, "enum", scope.def, "must be one of "+strings.Join(parts, ", "), "")
			return
		}
	}
	if t, ok := schema.Get("type"); ok && !typeMatches(instance, t) {
		names := []string{}
		if t.K == KindArr {
			for _, item := range t.A {
				names = append(names, jsonScalarString(item))
			}
		} else {
			names = append(names, jsonScalarString(t))
		}
		push(errs, at, "type", scope.def, "must be "+strings.Join(names, " or "), "")
		return
	}
	overLength := false
	switch instance.K {
	case KindStr:
		overLength = checkString(instance.S, schema, scope, at, errs)
	case KindNum:
		checkNumber(instance.N, schema, scope, at, errs)
	case KindArr:
		checkArray(instance, schema, scope, at, errs)
	case KindObj:
		checkObject(instance, schema, scope, at, errs)
	}
	if n, ok := schema.Get("not"); ok && !overLength && matches(instance, n, scope) {
		push(errs, at, "not", scope.def, "matches a forbidden pattern", "")
	}
	if all, ok := schema.Get("allOf"); ok {
		for _, part := range listOf(all) {
			check(instance, part, scope, at, errs)
		}
	}
	if _, ok := schema.Get("if"); ok {
		ifNode, _ := schema.Get("if")
		var branch Value
		var has bool
		if matches(instance, ifNode, scope) {
			branch, has = schema.Get("then")
		} else {
			branch, has = schema.Get("else")
		}
		if has {
			check(instance, branch, scope, at, errs)
		}
	}
	if one, ok := schema.Get("oneOf"); ok && one.K == KindArr {
		checkOneOf(instance, one.A, scope, at, errs)
	}
}

func resolve(node Value, ctx schemaScope) (Value, schemaScope) {
	schema := node
	def := ctx.def
	for schema.IsObject() {
		ref, ok := schema.Get("$ref")
		if !ok || ref.K != KindStr {
			break
		}
		def = ref.S[len(definitions):]
		defs, _ := ctx.root.Get("definitions")
		schema, _ = defs.Get(def)
	}
	if def == ctx.def {
		return schema, ctx
	}
	return schema, schemaScope{root: ctx.root, patterns: ctx.patterns, def: def}
}

func matches(instance, node Value, ctx schemaScope) bool {
	var errs []schemaErr
	check(instance, node, ctx, "", &errs)
	return len(errs) == 0
}

func checkString(value string, schema Value, ctx schemaScope, at string, errs *[]schemaErr) bool {
	length := jsstr.CodePoints(value)
	if min, ok := schema.Get("minLength"); ok && min.K == KindNum && length < int(min.N) {
		msg := "must be at least " + numString(min.N) + " characters"
		if min.N == 1 {
			msg = "must not be empty"
		}
		push(errs, at, "minLength", ctx.def, msg, "")
	}
	if max, ok := schema.Get("maxLength"); ok && max.K == KindNum && length > int(max.N) {
		push(errs, at, "maxLength", ctx.def, "must be at most "+numString(max.N)+" characters", "")
		return true
	}
	if pat, ok := schema.Get("pattern"); ok && pat.K == KindStr {
		re := ctx.patterns[pat.S]
		if re != nil && !re.MatchString(value) {
			push(errs, at, "pattern", ctx.def, "does not match the required format", "")
		}
	}
	return false
}

func checkNumber(value float64, schema Value, ctx schemaScope, at string, errs *[]schemaErr) {
	if min, ok := schema.Get("minimum"); ok && min.K == KindNum && value < min.N {
		push(errs, at, "minimum", ctx.def, "must be at least "+numString(min.N), "")
	}
	if max, ok := schema.Get("maximum"); ok && max.K == KindNum && value > max.N {
		push(errs, at, "maximum", ctx.def, "must be at most "+numString(max.N), "")
	}
}

func checkArray(value, schema Value, ctx schemaScope, at string, errs *[]schemaErr) {
	n := len(value.A)
	if min, ok := schema.Get("minItems"); ok && min.K == KindNum && n < int(min.N) {
		msg := "must have at least " + numString(min.N) + " item"
		if min.N != 1 {
			msg += "s"
		}
		push(errs, at, "minItems", ctx.def, msg, "")
	}
	if max, ok := schema.Get("maxItems"); ok && max.K == KindNum && n > int(max.N) {
		push(errs, at, "maxItems", ctx.def, "must have at most "+numString(max.N)+" items", "")
	}
	if items, ok := schema.Get("items"); ok {
		for i, item := range value.A {
			check(item, items, ctx, pointer(at, strconvI(i)), errs)
		}
	}
}

func checkObject(value, schema Value, ctx schemaScope, at string, errs *[]schemaErr) {
	if max, ok := schema.Get("maxProperties"); ok && max.K == KindNum && len(value.O) > int(max.N) {
		push(errs, at, "maxProperties", ctx.def, "must have at most "+numString(max.N)+" entries", "")
	}
	if req, ok := schema.Get("required"); ok {
		for _, key := range listOf(req) {
			if key.K != KindStr {
				continue
			}
			if !value.Has(key.S) {
				push(errs, pointer(at, key.S), "required", ctx.def, "missing required field: "+key.S, key.S)
			}
		}
	}
	props, _ := schema.Get("properties")
	add, hasAdd := schema.Get("additionalProperties")
	for _, p := range value.O {
		child := pointer(at, p.Key)
		if props.IsObject() && props.Has(p.Key) {
			sub, _ := props.Get(p.Key)
			check(p.Val, sub, ctx, child, errs)
		} else if hasAdd && add.K == KindBool && !add.B {
			push(errs, child, "additionalProperties", ctx.def, "unknown property", p.Key)
		} else if hasAdd && add.IsObject() {
			check(p.Val, add, ctx, child, errs)
		}
	}
}

func checkOneOf(value Value, branches []Value, ctx schemaScope, at string, errs *[]schemaErr) {
	matched := 0
	var declared []schemaErr
	haveDeclared := false
	for _, branch := range branches {
		var branchErrs []schemaErr
		check(value, branch, ctx, at, &branchErrs)
		if len(branchErrs) == 0 {
			matched++
		}
		if value.IsObject() {
			if t, ok := value.Get("type"); ok {
				if disc, ok := discriminator(branch, ctx); ok && jsEqual(disc, t) {
					declared = branchErrs
					haveDeclared = true
				}
			}
		}
	}
	if matched == 1 {
		return
	}
	if matched > 1 {
		push(errs, at, "oneOf", ctx.def, "matches more than one allowed shape", "")
		return
	}
	if haveDeclared {
		*errs = append(*errs, declared...)
		return
	}
	msg := "must be an object of a known type"
	if value.IsObject() {
		msg = "unknown block type"
	}
	push(errs, at, "oneOf", ctx.def, msg, "")
}

func discriminator(branch Value, ctx schemaScope) (Value, bool) {
	schema, _ := resolve(branch, ctx)
	props, ok := schema.Get("properties")
	if !ok || !props.IsObject() {
		return Value{}, false
	}
	typ, ok := props.Get("type")
	if !ok || !typ.IsObject() {
		return Value{}, false
	}
	c, ok := typ.Get("const")
	if !ok {
		return Value{}, false
	}
	return c, true
}

func typeMatches(instance, typ Value) bool {
	types := []Value{typ}
	if typ.K == KindArr {
		types = typ.A
	}
	for _, name := range types {
		if name.K != KindStr {
			continue
		}
		switch name.S {
		case "null":
			if instance.K == KindNull {
				return true
			}
		case "boolean":
			if instance.K == KindBool {
				return true
			}
		case "object":
			if instance.IsObject() {
				return true
			}
		case "array":
			if instance.K == KindArr {
				return true
			}
		case "number":
			if instance.K == KindNum && isFinite(instance.N) {
				return true
			}
		case "integer":
			if instance.K == KindNum && isJSInteger(instance.N) {
				return true
			}
		case "string":
			if instance.K == KindStr {
				return true
			}
		}
	}
	return false
}

func sameJSON(a, b Value) bool {
	if a.K != b.K {
		return false
	}
	switch a.K {
	case KindNull:
		return true
	case KindBool:
		return a.B == b.B
	case KindNum:
		return a.N == b.N
	case KindStr:
		return a.S == b.S
	case KindArr:
		if len(a.A) != len(b.A) {
			return false
		}
		for i := range a.A {
			if !sameJSON(a.A[i], b.A[i]) {
				return false
			}
		}
		return true
	case KindObj:
		if len(a.O) != len(b.O) {
			return false
		}
		for _, p := range a.O {
			bv, ok := b.Get(p.Key)
			if !ok || !sameJSON(p.Val, bv) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// jsEqual is JavaScript === for JSON primitives. Objects and arrays never match.
func jsEqual(a, b Value) bool {
	if a.K != b.K {
		return false
	}
	switch a.K {
	case KindNull:
		return true
	case KindBool:
		return a.B == b.B
	case KindNum:
		return a.N == b.N
	case KindStr:
		return a.S == b.S
	default:
		return false
	}
}

func push(errs *[]schemaErr, at, keyword, def, message, property string) {
	*errs = append(*errs, schemaErr{path: at, keyword: keyword, def: def, property: property, message: message})
}

func listOf(v Value) []Value {
	if v.K != KindArr {
		return nil
	}
	return v.A
}

func pointer(base, key string) string {
	var b strings.Builder
	b.Grow(len(base) + len(key) + 1)
	b.WriteString(base)
	b.WriteByte('/')
	for _, r := range key {
		switch r {
		case '~':
			b.WriteString("~0")
		case '/':
			b.WriteString("~1")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func strconvI(i int) string {
	return numString(float64(i))
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func blockAt(plan Value, u, l, b int) (Value, bool) {
	units, _ := plan.Get("units")
	unit := indexAt(units, u)
	if !unit.IsObject() {
		return Value{}, false
	}
	lessons, _ := unit.Get("lessons")
	lesson := indexAt(lessons, l)
	if !lesson.IsObject() {
		return Value{}, false
	}
	blocks, _ := lesson.Get("blocks")
	block := indexAt(blocks, b)
	if !block.IsObject() {
		return Value{}, false
	}
	return block, true
}

func indexAt(arr Value, i int) Value {
	if arr.K != KindArr || i < 0 || i >= len(arr.A) {
		return Value{}
	}
	return arr.A[i]
}

func cellAt(plan Value, at string) string {
	m := reCellAt.FindStringSubmatch(at)
	if m == nil {
		return ""
	}
	block, ok := blockAt(plan, atoi(m[1]), atoi(m[2]), atoi(m[3]))
	if !ok {
		return ""
	}
	cells, _ := block.Get("cells")
	row := indexAt(cells, atoi(m[4]))
	if !row.IsObject() {
		return ""
	}
	cell, ok := row.Get("cell")
	if !ok || cell.K != KindStr {
		return ""
	}
	return cell.S
}

func verseSourceAt(plan Value, at string) bool {
	m := reVerseSource.FindStringSubmatch(at)
	if m == nil {
		return false
	}
	block, ok := blockAt(plan, atoi(m[1]), atoi(m[2]), atoi(m[3]))
	if !ok {
		return false
	}
	t, ok := block.Get("type")
	return ok && t.K == KindStr && t.S == "verse-reading"
}

func typeItAt(plan Value, at string) bool {
	m := reTypeIt.FindStringSubmatch(at)
	if m == nil {
		return false
	}
	block, ok := blockAt(plan, atoi(m[1]), atoi(m[2]), atoi(m[3]))
	if !ok {
		return false
	}
	t, ok := block.Get("type")
	return ok && t.K == KindStr && t.S == "type-it"
}

func introWordAt(plan Value, at string) bool {
	m := reIntroWord.FindStringSubmatch(at)
	if m == nil {
		return false
	}
	block, ok := blockAt(plan, atoi(m[1]), atoi(m[2]), atoi(m[3]))
	if !ok {
		return false
	}
	t, ok := block.Get("type")
	return ok && t.K == KindStr && t.S == "introduction"
}
