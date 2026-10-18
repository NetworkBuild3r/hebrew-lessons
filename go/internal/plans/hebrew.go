package plans

import (
	"strings"
	"sync"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/jsstr"
)

// CheckHebrew matches server/plans/hebrew-checks.js checkHebrew.
// plan must be the parsed document. A non-object yields no findings.
func CheckHebrew(plan Value, opt *Options) []Finding {
	if !plan.IsObject() {
		return []Finding{}
	}
	var strs []strItem
	var objs []objItem
	walkPlan(plan, func(s strItem) { strs = append(strs, s) }, func(o objItem) { objs = append(objs, o) })
	var out []Finding
	checkText(plan, strs, &out)
	var words []objItem
	for _, o := range objs {
		if o.owner == "wordEntry" {
			words = append(words, o)
		}
	}
	var lex Lexicon
	if opt != nil {
		lex = opt.Lexicon
	}
	checkLexicon(plan, words, lex, &out)
	checkConsistency(plan, strs, &out)
	if snap, ok := parseSnapshot(opt); ok {
		checkAudio(plan, objs, snap, &out)
	}
	if out == nil {
		out = []Finding{}
	}
	return out
}

type strItem struct {
	owner    string
	prop     string
	value    string
	pointer  string
	identity string
}

type objItem struct {
	owner    string
	node     Value
	pointer  string
	identity string
}

const regenerateNote = "No tool generates audio for a plan field: generate_lesson_audio and " +
	"server/audio.js work on legacy worksheet.js lessons only. Remove the stale clip path " +
	"(replace_part without it) so the old clip stops playing, or put a new .mp3 under the " +
	"audio folder and set its path."

var fieldLanguage = map[string]string{
	"*.id": "exempt", "plan.assigneeId": "exempt", "plan.title": "exempt", "plan.subtitle": "exempt",
	"migration.sourceWorksheet": "exempt", "migration.activityOrder": "exempt",
	"migration.retiredActivityTypes": "exempt", "migration.retiredFields": "exempt",
	"unit.title": "exempt", "unit.theme": "exempt", "unit.frequencyBand": "exempt",
	"lesson.title": "exempt", "lesson.subtitle": "exempt", "lesson.footer": "exempt",
	"lesson.legacyStorageKey": "exempt", "lesson.buildsOn": "exempt",
	"recycle.fromLessonId": "exempt", "wordEntry.fromLessonId": "exempt",
	"imageRef.mediaId": "exempt", "wordEntry.audioPath": "exempt", "wordEntry.sentenceAudioPath": "exempt",
	"patternItem.audioPath": "exempt", "paragraph.audioPath": "exempt", "question.audioPath": "exempt",
	"line.audioPath": "exempt", "listeningBlock.audioPath": "exempt",
	"checkpointBlock.fromLessons": "exempt", "patternRule.title": "exempt", "explainerExample.root": "exempt",
	"lesson.newWords": "he", "recycle.word": "he", "wordEntry.word": "he", "wordEntry.sentence": "he",
	"patternRule.example": "he", "patternRule.focus": "he", "patternItem.choices": "he", "patternItem.pointed": "he",
	"glossPair.he": "he", "cloze.bank": "he", "clozeItem.textPre": "he", "clozeItem.textPost": "he",
	"clozeItem.prefix": "he", "clozeItem.answer": "he", "span.text": "he", "paragraph.text": "he",
	"paragraph.highlights": "he", "line.pointed": "he", "line.youFeminine": "he", "line.speak": "he",
	"line.followUp": "he", "listeningBlock.passage": "he", "shortDialogueBlock.intro": "he",
	"checkpointBlock.wordKeys": "he", "hebrewLine.he": "he", "explainerExample.he": "he",
	"conjugationTableBlock.lemmaKey": "he", "conjugationCell.formKey": "he", "verseReadingBlock.newWords": "he",
	"verseReadingBlock.verse": "he", "versePersonalQuestion.he": "he", "letterIntroBlock.letter": "he",
	"letterSound.pointed": "he", "letterFind.word": "he",
	"wordEntry.meaning": "en", "wordEntry.pos": "en", "glossPair.en": "en", "clozeItem.english": "en",
	"hebrewLine.en": "en", "explainerExample.en": "en", "versePersonalQuestion.en": "en",
	"wordEntry.sound": "latin", "wordEntry.translit": "latin", "explainerExample.translit": "latin",
	"letterSound.sound": "latin",
	"imageRef.alt":      "mixed", "wordEntry.trap": "mixed", "wordEntry.infinitive": "mixed",
	"pattern.tabLabel": "mixed", "pattern.heading": "mixed", "pattern.lede": "mixed", "pattern.prompt": "mixed",
	"pattern.reminders": "mixed", "patternRule.english": "mixed", "patternItem.direct": "mixed", "patternItem.why": "mixed",
	"question.question": "mixed", "question.sampleAnswer": "mixed", "line.question": "mixed", "line.gloss": "mixed",
	"introductionBlock.heading": "mixed", "introductionBlock.note": "mixed", "gradedStoryBlock.notice": "mixed",
	"gradedStoryBlock.glosses": "mixed", "shortDialogueBlock.introEn": "mixed", "fluencyPassBlock.cues": "mixed",
	"explainerBlock.title": "mixed", "explainerSection.heading": "mixed", "explainerSection.paragraphs": "mixed",
	"explainerTable.columns": "mixed", "explainerTable.rows": "mixed", "explainerCallout.title": "mixed",
	"explainerCallout.text": "mixed", "verseRef.book": "mixed", "verseRef.source": "mixed",
	"letterIntroBlock.name": "mixed", "letterSound.vowel": "mixed",
	"typeItBlock.image": "exempt", "typeItBlock.picture": "exempt", "typeItBlock.mediaId": "latin",
	"typeItBlock.prompt": "en", "typeItBlock.answer": "he",
	"exerciseBlock.heading": "mixed", "exerciseChooseItem.prompt": "mixed", "exerciseChooseItem.choices": "mixed",
	"exerciseChooseItem.answer": "mixed", "exerciseChooseItem.why": "en",
	"exerciseTrueFalseItem.statement": "mixed", "exerciseTrueFalseItem.why": "en",
	"exerciseUnscrambleItem.words": "he", "exerciseUnscrambleItem.alt": "he", "exerciseUnscrambleItem.english": "en",
	"exerciseTranslateItem.english": "en", "exerciseTranslateItem.answer": "he", "exerciseTranslateItem.accept": "he",
	"exerciseDictationItem.audioPath": "latin", "exerciseDictationItem.answer": "he", "exerciseDictationItem.accept": "he",
	"assignmentBlock.prompt": "mixed", "assignmentBlock.useWords": "he", "assignmentBlock.homework": "mixed",
}

func languageOf(owner, prop string) (string, bool) {
	if v, ok := fieldLanguage[owner+"."+prop]; ok {
		return v, true
	}
	v, ok := fieldLanguage["*."+prop]
	return v, ok
}

type audioPair struct{ owner, text, clip string }

var audioPairs = []audioPair{
	{"wordEntry", "word", "audioPath"},
	{"wordEntry", "sentence", "sentenceAudioPath"},
	{"patternItem", "pointed", "audioPath"},
	{"paragraph", "text", "audioPath"},
	{"question", "question", "audioPath"},
	{"line", "pointed", "audioPath"},
	{"listeningBlock", "passage", "audioPath"},
}

var wordLevel = map[string]struct{}{
	"wordEntry.word": {}, "letterFind.word": {}, "explainerExample.he": {}, "conjugationCell.formKey": {},
}

const (
	skOther = iota
	skString
	skEnum
	skArray
	skMap
	skObject
	skOneOf
)

type hShape struct {
	kind     int
	def      string
	props    map[string]*hShape
	items    *hShape
	values   *hShape
	branches map[string]*hShape
}

type hIndex struct {
	root    *hShape
	objects map[string]*hShape
}

var (
	hOnce sync.Once
	hIdx  *hIndex
)

func hebrewIndex() *hIndex {
	hOnce.Do(func() { hIdx = buildHebrewIndex(loadSchema().root) })
	return hIdx
}

func buildHebrewIndex(root Value) *hIndex {
	defs, _ := root.Get("definitions")
	objects := map[string]*hShape{}
	var resolve func(Value) (string, Value)
	var objectShape func(string, Value) *hShape
	var shapeOf func(Value, string) *hShape
	resolve = func(node Value) (string, Value) {
		name := ""
		target := node
		for hops := 0; hops < 16 && target.IsObject(); hops++ {
			ref, ok := target.Get("$ref")
			if !ok || ref.K != KindStr {
				break
			}
			name = ""
			if strings.HasPrefix(ref.S, definitions) {
				name = ref.S[len(definitions):]
			}
			if name == "" {
				return name, Value{}
			}
			next, ok := defs.Get(name)
			if !ok {
				return name, Value{}
			}
			target = next
		}
		return name, target
	}
	objectShape = func(name string, node Value) *hShape {
		if s, ok := objects[name]; ok {
			return s
		}
		sh := &hShape{kind: skObject, def: name, props: map[string]*hShape{}}
		objects[name] = sh
		props, _ := node.Get("properties")
		if props.IsObject() {
			for _, p := range props.O {
				sh.props[p.Key] = shapeOf(p.Val, name+"."+p.Key)
			}
		}
		return sh
	}
	shapeOf = func(node Value, fallback string) *hShape {
		name, target := resolve(node)
		if !target.IsObject() {
			return &hShape{kind: skOther}
		}
		if props, ok := target.Get("properties"); ok && props.IsObject() {
			use := name
			if use == "" {
				use = fallback
			}
			return objectShape(use, target)
		}
		if one, ok := target.Get("oneOf"); ok && one.K == KindArr {
			branches := map[string]*hShape{}
			for _, branch := range one.A {
				rname, rnode := resolve(branch)
				if !rnode.IsObject() || rname == "" {
					continue
				}
				props, ok := rnode.Get("properties")
				if !ok || !props.IsObject() {
					continue
				}
				typ, ok := props.Get("type")
				if !ok || !typ.IsObject() {
					continue
				}
				c, ok := typ.Get("const")
				if ok && c.K == KindStr {
					branches[c.S] = objectShape(rname, rnode)
				}
			}
			return &hShape{kind: skOneOf, branches: branches}
		}
		if _, ok := target.Get("enum"); ok {
			return &hShape{kind: skEnum}
		}
		if _, ok := target.Get("const"); ok {
			return &hShape{kind: skEnum}
		}
		var types []Value
		if t, ok := target.Get("type"); ok {
			if t.K == KindArr {
				types = t.A
			} else {
				types = []Value{t}
			}
		}
		if typeHas(types, "string") {
			return &hShape{kind: skString}
		}
		if typeHas(types, "array") {
			if items, ok := target.Get("items"); ok {
				return &hShape{kind: skArray, items: shapeOf(items, fallback)}
			}
		}
		if typeHas(types, "object") {
			if add, ok := target.Get("additionalProperties"); ok && add.IsObject() {
				return &hShape{kind: skMap, values: shapeOf(add, fallback)}
			}
		}
		return &hShape{kind: skOther}
	}
	rootShape := objectShape("plan", root)
	if defs.IsObject() {
		for _, p := range defs.O {
			if p.Val.IsObject() {
				if props, ok := p.Val.Get("properties"); ok && props.IsObject() {
					objectShape(p.Key, p.Val)
				}
			}
		}
	}
	return &hIndex{root: rootShape, objects: objects}
}

func typeHas(types []Value, name string) bool {
	for _, t := range types {
		if t.K == KindStr && t.S == name {
			return true
		}
	}
	return false
}

func walkPlan(plan Value, onString func(strItem), onObject func(objItem)) {
	root := hebrewIndex().root
	var visit func(value Value, sh *hShape, at, identity, owner, prop string)
	visit = func(value Value, sh *hShape, at, identity, owner, prop string) {
		if sh == nil {
			return
		}
		switch sh.kind {
		case skString:
			if value.K == KindStr {
				onString(strItem{owner: owner, prop: prop, value: value.S, pointer: at, identity: identity})
			}
		case skArray:
			if value.K != KindArr {
				return
			}
			for i, item := range value.A {
				itemID := identity
				if item.IsObject() {
					itemID = childIdentity(identity, prop, item, i, true)
				}
				visit(item, sh.items, pointer(at, strconvI(i)), itemID, owner, prop)
			}
		case skMap:
			return
		case skOneOf:
			if !value.IsObject() {
				return
			}
			t, ok := value.Get("type")
			if !ok || t.K != KindStr {
				return
			}
			if br := sh.branches[t.S]; br != nil {
				visit(value, br, at, identity, owner, prop)
			}
		case skObject:
			if !value.IsObject() {
				return
			}
			onObject(objItem{owner: sh.def, node: value, pointer: at, identity: identity})
			for _, p := range value.O {
				next := sh.props[p.Key]
				if next == nil {
					continue
				}
				nextID := identity
				if next.kind == skObject || next.kind == skOneOf {
					nextID = childIdentity(identity, p.Key, p.Val, 0, false)
				}
				visit(p.Val, next, pointer(at, p.Key), nextID, sh.def, p.Key)
			}
		}
	}
	visit(plan, root, "", "", "plan", "")
}

func childIdentity(identity, prop string, node Value, index int, hasIndex bool) string {
	id := ""
	if node.IsObject() {
		if v, ok := node.Get("id"); ok && v.K == KindStr && v.S != "" {
			id = v.S
		}
	}
	base := identity + "/" + prop
	if id == "" {
		if hasIndex {
			return base + "/#" + strconvI(index)
		}
		return base
	}
	return base + "/" + id
}

type hFix struct {
	override      bool
	action        string
	path          string
	field         string
	suggested     *string
	hasCandidates bool
	candidates    []string
	text          *string
	clip          *string
	note          *string
}

func makeH(plan Value, rule, at, message string, extra *hFix) Finding {
	idPath, field := toIDPath(plan, at)
	finding := Finding{Rule: rule, Severity: "warn", Path: at, Message: message, HasField: true, Field: field}
	if extra == nil {
		return finding
	}
	fx := &Fix{Action: actionFor(field), Path: idPath, Field: field}
	if extra.override {
		fx.Action, fx.Path, fx.Field = extra.action, extra.path, extra.field
	}
	fx.Suggested = extra.suggested
	fx.HasCandidates = extra.hasCandidates
	fx.Candidates = extra.candidates
	fx.Text, fx.Clip, fx.Note = extra.text, extra.clip, extra.note
	finding.Fix = fx
	return finding
}

func checkText(plan Value, strs []strItem, out *[]Finding) {
	for _, item := range strs {
		lang, ok := languageOf(item.owner, item.prop)
		if !ok || jsstr.Trim(item.value) == "" {
			continue
		}
		heb := hasHebrewLetter(item.value)
		if lang == "he" {
			if !heb && hasLetter(item.value) {
				*out = append(*out, makeH(plan, "field-language", item.pointer,
					"this field is marked Hebrew and holds no Hebrew letter; it may hold English or a transliteration", nil))
			}
			for _, cl := range pointingClauses(item.value) {
				var extra *hFix
				if cl.suggested != nil {
					s := *cl.suggested
					extra = &hFix{suggested: &s}
				}
				*out = append(*out, makeH(plan, "pointing-form", item.pointer, cl.message, extra))
			}
		} else if (lang == "en" || lang == "latin") && heb && (lang == "latin" || mostlyHebrew(item.value)) {
			which := "Latin-script"
			how := "a "
			plural := ""
			if lang == "en" {
				which = "English"
				how = "mostly "
				plural = "s"
			}
			*out = append(*out, makeH(plan, "field-language", item.pointer,
				"this field is marked "+which+" and holds "+how+"Hebrew letter"+plural+"; the Hebrew may belong in a Hebrew field", nil))
		}
	}
}

func mostlyHebrew(value string) bool {
	return countHebrew(value) > countLatin(value)
}

type clause struct {
	message   string
	suggested *string
}

func pointingClauses(value string) []clause {
	nfc := jsstr.NFC(value)
	stray, doubled := 0, 0
	letter := false
	vowels := 0
	position := 0
	for i := 0; i < len(nfc); {
		r, size := jsstr.Next(nfc, i)
		if size == 0 {
			break
		}
		i += size
		position++
		switch {
		case r >= 0x05D0 && r <= 0x05EA:
			letter = true
			vowels = 0
		case isVowel(r):
			if !letter {
				if stray == 0 {
					stray = position
				}
			} else {
				vowels++
				if vowels == 2 && doubled == 0 {
					doubled = position
				}
			}
		case !isCombining(r):
			letter = false
			vowels = 0
		}
	}
	var out []clause
	if stray != 0 {
		out = append(out, clause{message: "a vowel point at character " + strconvI(stray) + " has no Hebrew letter before it"})
	}
	if doubled != 0 {
		out = append(out, clause{message: "two vowel points sit on one letter, ending at character " + strconvI(doubled)})
	}
	if value != nfc {
		s := nfc
		out = append(out, clause{
			message:   "the text is not in NFC form (letters, points, and marks are in a different order than the stored form)",
			suggested: &s,
		})
	}
	return out
}

func isVowel(r rune) bool {
	return (r >= 0x05B0 && r <= 0x05BB) || r == 0x05C7
}

func isCombining(r rune) bool {
	return (r >= 0x0591 && r <= 0x05BD) || r == 0x05BF || r == 0x05C1 || r == 0x05C2 || r == 0x05C4 || r == 0x05C5 || r == 0x05C7
}

func checkLexicon(plan Value, entries []objItem, lex Lexicon, out *[]Finding) {
	lister, ok := lex.(StrippedLister)
	if lex == nil || !ok {
		return
	}
	var keys []string
	seen := map[string]struct{}{}
	for _, entry := range entries {
		word, ok := entry.node.Get("word")
		if ok && word.K == KindStr && hasHebrewLetter(word.S) {
			k := strippedKey(word.S)
			if _, dup := seen[k]; !dup {
				seen[k] = struct{}{}
				keys = append(keys, k)
			}
		}
	}
	if len(keys) == 0 {
		return
	}
	rows, err := lister.ListByStrippedKeys(keys)
	if err != nil {
		return
	}
	by := map[string][]Value{}
	var order []string
	for _, row := range rows {
		if !row.IsObject() {
			continue
		}
		key, ok := row.Get("key")
		if !ok || key.K != KindStr {
			continue
		}
		k := strippedKey(key.S)
		if _, exists := by[k]; !exists {
			order = append(order, k)
		}
		by[k] = append(by[k], row)
	}
	_ = order
	for _, entry := range entries {
		word, ok := entry.node.Get("word")
		if !ok || word.K != KindStr || !hasHebrewLetter(word.S) {
			continue
		}
		matches := by[strippedKey(word.S)]
		if len(matches) != 1 {
			continue
		}
		row := matches[0]
		ver, _ := row.Get("verified")
		verified := isVerified(ver)
		rk, _ := row.Get("key")
		if exactKey(rk.S) != exactKey(word.S) {
			sug := exactKey(rk.S)
			msg := "the lexicon has one entry with these letters and it is pointed differently"
			if verified {
				msg += "; it is verified, so the lexicon form is the authority"
			} else {
				msg += "; it is not verified, so treat it as a lead and not as the authority"
			}
			*out = append(*out, makeH(plan, "lexicon-form", pointer(entry.pointer, "word"), msg, &hFix{suggested: &sug}))
		}
		if !verified {
			continue
		}
		for _, pair := range [][2]string{{"meaning", "gloss_en"}, {"translit", "translit"}} {
			have, hok := entry.node.Get(pair[0])
			stored, sok := row.Get(pair[1])
			if !hok || !sok || have.K != KindStr || stored.K != KindStr {
				continue
			}
			ht, st := jsstr.Trim(have.S), jsstr.Trim(stored.S)
			if ht == "" || st == "" {
				continue
			}
			if jsstr.ToLower(ht) == jsstr.ToLower(st) {
				continue
			}
			sug := st
			*out = append(*out, makeH(plan, "lexicon-form", pointer(entry.pointer, pair[0]),
				"this "+pair[0]+" differs from the verified lexicon entry, which is the authority", &hFix{suggested: &sug}))
		}
	}
}

type occ struct {
	form    string
	pointer string
	set     string
	hasSet  bool
}

func checkConsistency(plan Value, strs []strItem, out *[]Finding) {
	type group struct {
		forms []string
		count map[string]int
		items []occ
	}
	type lessonGroups struct {
		keys   []string
		groups map[string]*group
	}
	// The loop above lost key order of stripped keys. Rebuild properly.
	ordered := map[string]*lessonGroups{}
	var lorder []string
	for _, item := range strs {
		if _, ok := wordLevel[item.owner+"."+item.prop]; !ok {
			continue
		}
		m := reLessonPrefix.FindString(item.pointer)
		if m == "" || !hasHebrewPoint(item.value) {
			continue
		}
		form := formOf(item.value)
		stripped := strippedKey(form)
		if stripped == "" {
			continue
		}
		lg := ordered[m]
		if lg == nil {
			lg = &lessonGroups{groups: map[string]*group{}}
			ordered[m] = lg
			lorder = append(lorder, m)
		}
		g := lg.groups[stripped]
		if g == nil {
			g = &group{count: map[string]int{}}
			lg.groups[stripped] = g
			lg.keys = append(lg.keys, stripped)
		}
		if g.count[form] == 0 {
			g.forms = append(g.forms, form)
		}
		g.count[form]++
		o := occ{form: form, pointer: item.pointer}
		o.set, o.hasSet = setOf(item)
		g.items = append(g.items, o)
	}
	for _, lesson := range lorder {
		lg := ordered[lesson]
		for _, key := range lg.keys {
			g := lg.groups[key]
			if len(g.forms) < 2 {
				continue
			}
			reference := g.forms[0]
			for _, form := range g.forms[1:] {
				if g.count[form] > g.count[reference] {
					reference = form
				}
			}
			for _, o := range g.items {
				if o.form == reference {
					continue
				}
				different := false
				for _, p := range g.items {
					if p.form == o.form {
						continue
					}
					sameSet := p.hasSet && o.hasSet && p.set == o.set
					if !sameSet {
						different = true
						break
					}
				}
				if !different {
					continue
				}
				var other occ
				for _, p := range g.items {
					if p.form == reference {
						other = p
						break
					}
				}
				op, ofield := toIDPath(plan, other.pointer)
				msg := "this lesson points the same word two ways; this field has " + o.form +
					" and " + op + " (" + ofield + ") has " + reference +
					". Both may be right (a homograph); pick one if they are not"
				cands := append([]string{}, g.forms...)
				*out = append(*out, makeH(plan, "pointing-consistency", o.pointer, msg, &hFix{hasCandidates: true, candidates: cands}))
			}
		}
	}
}

func setOf(item strItem) (string, bool) {
	if item.owner == "wordEntry" {
		return "words", true
	}
	if item.owner == "conjugationCell" {
		return reCellsFormKey.ReplaceAllString(item.pointer, ""), true
	}
	return "", false
}

func formOf(value string) string {
	parts := strings.Split(exactKey(value), " ")
	var kept []string
	for _, part := range parts {
		part = trimEdges(part)
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " ")
}

func checkAudio(plan Value, objs []objItem, snapshot Value, out *[]Finding) {
	before := map[string]struct{ text, clip Value }{}
	walkPlan(snapshot, func(strItem) {}, func(item objItem) {
		for _, pair := range audioPairs {
			if item.owner != pair.owner {
				continue
			}
			key := item.identity + "\x00" + pair.text + "\x00" + pair.clip
			text, _ := item.node.Get(pair.text)
			clip, _ := item.node.Get(pair.clip)
			before[key] = struct{ text, clip Value }{text, clip}
		}
	})
	for _, item := range objs {
		for _, pair := range audioPairs {
			if item.owner != pair.owner {
				continue
			}
			old, ok := before[item.identity+"\x00"+pair.text+"\x00"+pair.clip]
			if !ok {
				continue
			}
			text, _ := item.node.Get(pair.text)
			clip, _ := item.node.Get(pair.clip)
			if text.K != KindStr || old.text.K != KindStr {
				continue
			}
			if clip.K != KindStr || clip.S == "" || old.clip.K != KindStr || clip.S != old.clip.S {
				continue
			}
			if jsstr.Trim(jsstr.NFC(text.S)) == jsstr.Trim(jsstr.NFC(old.text.S)) {
				continue
			}
			clipPtr := pointer(item.pointer, pair.clip)
			cpath, cfield := toIDPath(plan, clipPtr)
			textName, clipName, note := pair.text, pair.clip, regenerateNote
			*out = append(*out, makeH(plan, "audio-stale", pointer(item.pointer, pair.text),
				"this text changed since the last publish and its clip "+pair.clip+" did not, so the clip may no "+
					"longer match the text. This check compares with the published copy; it cannot tell a clip that "+
					"was already stale at publish.",
				&hFix{override: true, action: "replace_part", path: cpath, field: cfield, text: &textName, clip: &clipName, note: &note}))
		}
	}
}

func parseSnapshot(opt *Options) (Value, bool) {
	if opt == nil || opt.SnapshotMode == SnapNone {
		return Value{}, false
	}
	if opt.SnapshotMode == SnapValue {
		if opt.Snapshot.IsObject() {
			return opt.Snapshot, true
		}
		return Value{}, false
	}
	if opt.SnapshotRaw == "" {
		return Value{}, false
	}
	v, err := Parse(opt.SnapshotRaw)
	if err != nil || !v.IsObject() {
		return Value{}, false
	}
	return v, true
}
