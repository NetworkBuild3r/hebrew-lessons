package plans

import (
	"math"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/jsstr"
	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/ucd"
)

func exactKey(value string) string {
	return jsstr.Trim(jsstr.CollapseSpace(jsstr.NFC(value)))
}

func strippedKey(value string) string {
	return exactKey(stripPoints(exactKey(value)))
}

func stripPoints(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := jsstr.Next(s, i)
		if size == 0 {
			break
		}
		if r < 0x0591 || r > 0x05C7 {
			writeRune(&b, r)
		}
		i += size
	}
	return b.String()
}

func writeRune(b *strings.Builder, r rune) {
	if r >= 0xD800 && r <= 0xDFFF {
		b.WriteByte(0xE0 | byte(r>>12))
		b.WriteByte(0x80 | byte((r>>6)&0x3F))
		b.WriteByte(0x80 | byte(r&0x3F))
		return
	}
	b.WriteRune(r)
}

func tokenize(value string) []string {
	key := exactKey(value)
	if key == "" {
		return nil
	}
	parts := strings.Split(key, " ")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = trimEdges(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func trimEdges(token string) string {
	start, end := 0, len(token)
	for start < end {
		r, size := jsstr.Next(token, start)
		if size == 0 || !isEdgeMark(r) {
			break
		}
		start += size
	}
	for end > start {
		r, size := prevRune(token, end)
		if size == 0 || !isEdgeMark(r) {
			break
		}
		end -= size
	}
	if start == 0 && end == len(token) {
		return token
	}
	return token[start:end]
}

func prevRune(s string, end int) (rune, int) {
	if end == 0 {
		return 0, 0
	}
	start := end - 1
	for start > 0 && start > end-4 && s[start]&0xC0 == 0x80 {
		start--
	}
	r, size := jsstr.Next(s, start)
	if size == 0 || start+size != end {
		return rune(s[end-1]), 1
	}
	return r, size
}

func isEdgeMark(r rune) bool {
	return ucd.IsEdgeMark(r)
}

func hasHebrewLetter(s string) bool {
	for i := 0; i < len(s); {
		r, size := jsstr.Next(s, i)
		if size == 0 {
			break
		}
		if r >= 0x05D0 && r <= 0x05EA {
			return true
		}
		i += size
	}
	return false
}

func hasHebrewPoint(s string) bool {
	for i := 0; i < len(s); {
		r, size := jsstr.Next(s, i)
		if size == 0 {
			break
		}
		if isHebrewPoint(r) {
			return true
		}
		i += size
	}
	return false
}

func isHebrewPoint(r rune) bool {
	return (r >= 0x05B0 && r <= 0x05BD) || r == 0x05BF || r == 0x05C1 || r == 0x05C2 || r == 0x05C7
}

func hasLetter(s string) bool {
	for i := 0; i < len(s); {
		r, size := jsstr.Next(s, i)
		if size == 0 {
			break
		}
		if ucd.IsLetter(r) {
			return true
		}
		i += size
	}
	return false
}

func countHebrew(s string) int {
	n := 0
	for i := 0; i < len(s); {
		r, size := jsstr.Next(s, i)
		if size == 0 {
			break
		}
		if r >= 0x05D0 && r <= 0x05EA {
			n++
		}
		i += size
	}
	return n
}

func countLatin(s string) int {
	n := 0
	for i := 0; i < len(s); {
		r, size := jsstr.Next(s, i)
		if size == 0 {
			break
		}
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			n++
		}
		i += size
	}
	return n
}

var functionWords = func() []string {
	raw := []string{"אֲנִי", "אַתָּה", "אַתְּ", "הוּא", "הִיא", "אֲנַחְנוּ", "זֶה", "לֹא", "כֵּן"}
	out := make([]string, len(raw))
	for i, w := range raw {
		out[i] = jsstr.NFC(w)
	}
	return out
}()

var kinshipStripped = func() map[string]struct{} {
	m := map[string]struct{}{}
	for _, w := range []string{"אם", "אב", "אח", "אחות", "בן", "בת"} {
		m[jsstr.NFC(w)] = struct{}{}
	}
	return m
}()

func checkEstimates(lessons []lessonEntry, ctx valContext, f *findings) {
	for i := range lessons {
		checkCoverage(&lessons[i], f)
	}
	var words []string
	for _, entry := range lessons {
		for _, w := range entry.newWords {
			words = append(words, w.raw)
		}
		for _, w := range entry.verseWords {
			words = append(words, w.raw)
		}
		for _, w := range entry.recycled {
			words = append(words, w.raw)
		}
		for _, foc := range entry.focus {
			words = append(words, foc.raw)
		}
	}
	units := textUnits(lessons, ctx)
	cat := buildCatalog(words)
	find := occurrences(units, cat)
	multi := len(lessons) > 1
	for i := range lessons {
		entry := &lessons[i]
		for _, word := range entry.newWords {
			hits := find(word.raw)
			here := countOccurrences(hits, word, entry, f)
			if !multi {
				continue
			}
			used := map[string]struct{}{}
			for _, h := range here {
				used[h.unit.sentence] = struct{}{}
			}
			later := false
			for _, h := range hits {
				if h.unit.lesson > entry.index {
					if _, ok := used[h.unit.sentence]; !ok {
						later = true
						break
					}
				}
			}
			if !later {
				f.add("later-context", word.at, "no later lesson uses this word in a different sentence", "")
			}
		}
		for _, word := range entry.verseWords {
			countOccurrences(find(word.raw), word, entry, f)
		}
		for _, foc := range entry.focus {
			seen := false
			for _, h := range find(foc.raw) {
				u := h.unit
				if (u.kind == "story" || u.kind == "dialogue") && (u.lesson == entry.index || (u.lesson > entry.index && u.unit == entry.u)) {
					seen = true
					break
				}
			}
			if !seen {
				f.add("pattern-focus", foc.at, "the pattern focus is in no story or dialogue in this lesson or later in the unit", "")
			}
		}
	}
}

type hit struct {
	unit  textUnit
	count int
}

func countOccurrences(hits []hit, word wordRef, entry *lessonEntry, f *findings) []hit {
	var here []hit
	count := 0
	for _, h := range hits {
		if h.unit.lesson == entry.index {
			here = append(here, h)
			count += h.count
		}
	}
	if count < 2 {
		f.add("N6", word.at, "appears "+strconvI(count)+" time(s) in this lesson; N6 asks for 2", "")
	}
	return here
}

func checkCoverage(entry *lessonEntry, f *findings) {
	raws := make([]string, 0, len(entry.newWords)+len(entry.recycled)+len(functionWords))
	for _, w := range entry.newWords {
		raws = append(raws, w.raw)
	}
	for _, w := range entry.recycled {
		raws = append(raws, w.raw)
	}
	raws = append(raws, functionWords...)
	words := buildCatalog(raws)
	for _, item := range entry.blocks {
		known, total := 0, 0
		switch item.typ {
		case "graded-story":
			var sharedVals []string
			if g, ok := item.block.Get("glosses"); ok && g.IsObject() {
				for _, p := range g.O {
					sharedVals = append(sharedVals, p.Key)
				}
			}
			if sup, ok := item.block.Get("support"); ok {
				for _, pair := range listOf(sup) {
					if pair.IsObject() {
						if he, ok := pair.Get("he"); ok && he.K == KindStr {
							sharedVals = append(sharedVals, he.S)
						}
					}
				}
			}
			shared := treatedTokens(sharedVals)
			paras, _ := item.block.Get("paragraphs")
			for _, paragraph := range listOf(paras) {
				if !paragraph.IsObject() {
					continue
				}
				var ownVals []string
				spans, _ := paragraph.Get("spans")
				for _, span := range listOf(spans) {
					if span.IsObject() {
						if tx, ok := span.Get("text"); ok && tx.K == KindStr {
							ownVals = append(ownVals, tx.S)
						}
					}
				}
				own := treatedTokens(ownVals)
				text, _ := paragraph.Get("text")
				k, t := coverageCount(text, words, func(token string) bool {
					_, a := shared[token]
					_, b := own[token]
					return a || b
				})
				known += k
				total += t
			}
		case "listening":
			text, _ := item.block.Get("passage")
			known, total = coverageCount(text, words, func(string) bool { return false })
		default:
			continue
		}
		if total != 0 && known*100 < 95*total {
			pct := int(math.Floor(float64(known*100) / float64(total)))
			f.add("N3", item.at, "about "+strconvI(pct)+"% of running words are known; N3 asks for 95%", "")
		}
	}
}

func treatedTokens(values []string) map[string]struct{} {
	tokens := map[string]struct{}{}
	for _, value := range values {
		for _, token := range tokenize(value) {
			tokens[strippedKey(token)] = struct{}{}
		}
	}
	return tokens
}

func coverageCount(value Value, words cat, treated func(string) bool) (known, total int) {
	var exact []string
	if value.K == KindStr {
		exact = tokenize(value.S)
	}
	unit := textUnit{exact: exact}
	unit.stripped = make([]string, len(exact))
	for i, tok := range exact {
		unit.stripped[i] = strippedKey(tok)
	}
	i := 0
	for i < len(exact) {
		longest := 0
		walkTrie(words, unit, i, func(_ *catEntry, length int) { longest = length })
		if longest > 0 {
			known += longest
			i += longest
		} else {
			if treated(unit.stripped[i]) {
				known++
			}
			i++
		}
	}
	return known, len(exact)
}

type textUnit struct {
	lesson   int
	unit     int
	kind     string
	exact    []string
	stripped []string
	sentence string
}

func textUnits(lessons []lessonEntry, ctx valContext) []textUnit {
	var units []textUnit
	add := func(entry *lessonEntry, kind, value string) {
		exact := tokenize(value)
		if len(exact) == 0 {
			return
		}
		stripped := make([]string, len(exact))
		for i, tok := range exact {
			stripped[i] = strippedKey(tok)
		}
		units = append(units, textUnit{
			lesson: entry.index, unit: entry.u, kind: kind,
			exact: exact, stripped: stripped, sentence: strings.Join(stripped, " "),
		})
	}
	for i := range lessons {
		entry := &lessons[i]
		for _, item := range entry.blocks {
			switch item.typ {
			case "introduction":
				words, _ := item.block.Get("words")
				for _, word := range listOf(words) {
					if word.IsObject() {
						if s, ok := word.Get("sentence"); ok && s.K == KindStr {
							add(entry, "example", s.S)
						}
					}
				}
				cloze, _ := item.block.Get("cloze")
				if !cloze.IsObject() {
					cloze = VObj()
				}
				items, _ := cloze.Get("items")
				for _, it := range listOf(items) {
					if !it.IsObject() {
						continue
					}
					var parts []string
					for _, key := range []string{"textPre", "answer", "textPost"} {
						if p, ok := it.Get(key); ok && p.K == KindStr {
							parts = append(parts, p.S)
						}
					}
					add(entry, "cloze", strings.Join(parts, " "))
				}
			case "graded-story":
				paras, _ := item.block.Get("paragraphs")
				for _, paragraph := range listOf(paras) {
					if paragraph.IsObject() {
						if tx, ok := paragraph.Get("text"); ok && tx.K == KindStr {
							add(entry, "story", tx.S)
						}
					}
				}
			case "listening":
				if p, ok := item.block.Get("passage"); ok && p.K == KindStr {
					add(entry, "listening", p.S)
				}
			case "short-dialogue":
				lines, _ := item.block.Get("lines")
				for _, line := range listOf(lines) {
					if !line.IsObject() {
						continue
					}
					if p, ok := line.Get("pointed"); ok && p.K == KindStr {
						add(entry, "dialogue", p.S)
					}
					if p, ok := line.Get("followUp"); ok && p.K == KindStr {
						add(entry, "dialogue", p.S)
					}
				}
			case "verse-reading":
				if !ctx.v110 {
					continue
				}
				if v, ok := item.block.Get("verse"); ok && v.K == KindStr {
					add(entry, "verse", v.S)
				}
				qs, _ := item.block.Get("questions")
				for _, q := range listOf(qs) {
					if q.IsObject() {
						if he, ok := q.Get("he"); ok && he.K == KindStr {
							add(entry, "verse", he.S)
						}
					}
				}
			}
		}
	}
	return units
}

type catEntry struct {
	exact []string
}

type trieNode struct {
	next    map[string]*trieNode
	entries []*catEntry
	blank   bool
	entry   *catEntry
}

func newTrie() *trieNode { return &trieNode{next: map[string]*trieNode{}} }

type cat struct {
	byExact map[string]*catEntry
	loose   *trieNode
	strict  *trieNode
}

func buildCatalog(words []string) cat {
	c := cat{byExact: map[string]*catEntry{}, loose: newTrie(), strict: newTrie()}
	for _, raw := range words {
		exact := tokenize(raw)
		key := strings.Join(exact, " ")
		if len(exact) == 0 || c.byExact[key] != nil {
			continue
		}
		entry := &catEntry{exact: exact}
		c.byExact[key] = entry
		stripped := make([]string, len(exact))
		blank := true
		for i, tok := range exact {
			stripped[i] = strippedKey(tok)
			if stripped[i] != "" {
				blank = false
			}
		}
		end := descend(c.loose, stripped)
		end.entries = append(end.entries, entry)
		end.blank = blank
		descend(c.strict, exact).entry = entry
	}
	return c
}

func descend(node *trieNode, tokens []string) *trieNode {
	at := node
	for _, token := range tokens {
		next := at.next[token]
		if next == nil {
			next = newTrie()
			at.next[token] = next
		}
		at = next
	}
	return at
}

func walkTrie(words cat, unit textUnit, start int, onHit func(*catEntry, int)) {
	loose := words.loose
	var strict *trieNode = words.strict
	for i := start; i < len(unit.exact); i++ {
		next := loose.next[unit.stripped[i]]
		if next == nil {
			return
		}
		loose = next
		if strict != nil {
			strict = strict.next[unit.exact[i]]
		}
		var entry *catEntry
		if len(loose.entries) == 1 && !loose.blank {
			entry = loose.entries[0]
		} else if strict != nil {
			entry = strict.entry
		}
		if entry != nil {
			onHit(entry, i-start+1)
		}
	}
}

func occurrences(units []textUnit, words cat) func(string) []hit {
	type acc struct {
		order []int
		count map[int]int
	}
	found := map[*catEntry]*acc{}
	for u := range units {
		unit := units[u]
		for start := 0; start < len(unit.exact); start++ {
			walkTrie(words, unit, start, func(entry *catEntry, _ int) {
				a := found[entry]
				if a == nil {
					a = &acc{count: map[int]int{}}
					found[entry] = a
				}
				if _, ok := a.count[u]; !ok {
					a.order = append(a.order, u)
				}
				a.count[u]++
			})
		}
	}
	return func(raw string) []hit {
		entry := words.byExact[strings.Join(tokenize(raw), " ")]
		if entry == nil {
			return nil
		}
		a := found[entry]
		if a == nil {
			return nil
		}
		out := make([]hit, 0, len(a.order))
		for _, u := range a.order {
			out = append(out, hit{unit: units[u], count: a.count[u]})
		}
		return out
	}
}
