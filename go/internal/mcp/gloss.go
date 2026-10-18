package mcp

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

func (a *App) suggestGlosses(id, path string) (Value, error) {
	read, err := a.readPart(id, path)
	if err != nil {
		return Value{}, err
	}
	if _, bad := read.Get("error"); bad {
		return read, nil
	}
	part, _ := read.Get("part")
	typ, _ := part.Get("type")
	if part.Kind != KindObject || typ.Kind != KindString || typ.Str != "graded-story" {
		return planErr("not_a_story", Field{Key: "path", Val: String(path)}), nil
	}
	draft, err := a.suggestForBlock(part)
	if err != nil {
		return Value{}, err
	}
	rev, _ := read.Get("revision")
	return Object(
		Field{Key: "revision", Val: rev},
		Field{Key: "suggestions", Val: draft.suggestions},
		Field{Key: "unmatched", Val: draft.unmatched},
		Field{Key: "skipped", Val: Int(int64(draft.skipped))},
	), nil
}

type glossDraft struct {
	suggestions Value
	unmatched   Value
	skipped     int
}

func (a *App) suggestForBlock(block Value) (glossDraft, error) {
	paragraphs, _ := block.Get("paragraphs")
	var b strings.Builder
	for i, p := range paragraphs.Arr {
		text, _ := p.Get("text")
		if text.Kind == KindString {
			if i > 0 && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(text.Str)
		}
	}
	var skip []string
	if glosses, ok := block.Get("glosses"); ok && glosses.Kind == KindObject {
		for _, f := range glosses.Obj {
			skip = append(skip, f.Key)
		}
	}
	if support, ok := block.Get("support"); ok && support.Kind == KindArray {
		for _, pair := range support.Arr {
			he, ok := pair.Get("he")
			if ok && he.Kind == KindString && he.Str != "" {
				skip = append(skip, he.Str)
			}
		}
	}
	room := 80 - len(skip)
	if room < 0 {
		room = 0
	}
	return a.suggestGlossText(b.String(), skip, room)
}

func (a *App) suggestGlossText(text string, skip []string, max int) (glossDraft, error) {
	if max > 80 {
		max = 80
	}
	have := map[string]bool{}
	for _, word := range skip {
		have[foldHebrew(word)] = true
	}
	var suggestions []Value
	var unmatched []Value
	skipped := 0
	for _, word := range tokensOf(text) {
		if have[foldHebrew(word)] {
			skipped++
			continue
		}
		if len(suggestions)+len(unmatched) >= max {
			continue
		}
		found, err := a.lookupWord(word)
		if err != nil {
			return glossDraft{}, err
		}
		if found.short != "" {
			senses := make([]Value, 0, len(found.meanings))
			limit := len(found.meanings)
			if limit > 4 {
				limit = 4
			}
			for _, m := range found.meanings[:limit] {
				senses = append(senses, Object(
					Field{Key: "pos", Val: String(m.pos)},
					Field{Key: "en", Val: String(m.en)},
				))
			}
			form := Null()
			if found.form != "" {
				form = String(found.form)
			}
			suggestions = append(suggestions, Object(
				Field{Key: "word", Val: String(word)},
				Field{Key: "meaning", Val: String(found.short)},
				Field{Key: "form", Val: form},
				Field{Key: "senses", Val: Array(senses...)},
			))
		} else {
			unmatched = append(unmatched, String(word))
		}
	}
	if suggestions == nil {
		suggestions = []Value{}
	}
	if unmatched == nil {
		unmatched = []Value{}
	}
	return glossDraft{Array(suggestions...), Array(unmatched...), skipped}, nil
}

var hebrewToken = regexp.MustCompile(`[\x{05D0}-\x{05EA}][\x{05D0}-\x{05EA}\x{0591}-\x{05C7}\x{05BE}'\x{05F3}\x{05F4}"]*`)

func tokensOf(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, token := range hebrewToken.FindAllString(text, -1) {
		word := strings.TrimRight(token, "\u05BE'\u05F3\u05F4\"")
		fold := foldHebrew(word)
		if utf8.RuneCountInString(fold) < 2 || seen[fold] {
			continue
		}
		seen[fold] = true
		out = append(out, word)
	}
	return out
}

func foldHebrew(text string) string {
	var b strings.Builder
	for _, r := range exactKey(text) {
		if r >= 0x0591 && r <= 0x05C7 {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

type meaning struct {
	pos, en, source string
}

type looked struct {
	short    string
	form     string
	meanings []meaning
}

func (a *App) lookupWord(word string) (looked, error) {
	raw := exactKey(word)
	runes := []rune(raw)
	if len(runes) > 60 {
		runes = runes[:60]
	}
	clipped := string(runes)
	fold := foldHebrew(clipped)
	var result looked
	if fold == "" || !strings.ContainsFunc(fold, func(r rune) bool { return r >= 0x05D0 && r <= 0x05EA }) {
		return result, nil
	}
	if a.db == nil {
		return result, fmt.Errorf("%s", dbUnavailable("suggest_glosses", "lexicon_entries"))
	}
	infinitive := []rune(fold)[0] == 'ל'
	seen := map[string]bool{}
	push := func(pos, en, source string) {
		key := pos + "\n" + strings.ToLower(en)
		if en == "" || seen[key] || len(result.meanings) >= 8 {
			return
		}
		seen[key] = true
		result.meanings = append(result.meanings, meaning{pos, en, source})
	}
	rows, err := a.db.Query(`SELECT key, gloss_en, pos FROM lexicon_entries`)
	if err != nil {
		return result, sqlErr("suggest_glosses", "lexicon_entries", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key, gloss string
		var pos sqlNullStr
		if err := rows.Scan(&key, &gloss, &pos); err != nil {
			return result, err
		}
		if foldHebrew(key) == fold && gloss != "" {
			push(pos.s, gloss, "lexicon")
		}
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	exact, err := a.entriesForFolds([]string{fold}, 24)
	if err != nil {
		return result, err
	}
	for _, row := range everydayFirst(exact, infinitive) {
		push(row.pos, row.en, "dictionary")
	}
	glosses, err := a.glossesForFold(fold, 6)
	if err != nil {
		return result, err
	}
	if len(glosses) > 0 {
		joined := strings.Join(glosses, ", ")
		push("", shortGloss(joined), "wiktionary")
	}
	if len(exact) == 0 {
		cands := candidates(fold)
		rows, err := a.entriesForFolds(cands, 24)
		if err != nil {
			return result, err
		}
		if len(rows) > 0 {
			result.form = rows[0].he
			var same []dictEntry
			for _, row := range rows {
				if row.fold == rows[0].fold {
					same = append(same, row)
				}
			}
			for _, row := range everydayFirst(same, infinitive) {
				push(row.pos, row.en, "dictionary")
			}
		}
	}
	if len(result.meanings) > 0 {
		result.short = shortGloss(result.meanings[0].en)
	}
	return result, nil
}

type dictEntry struct {
	he, fold, pos, en string
}

func (a *App) entriesForFolds(folds []string, limit int) ([]dictEntry, error) {
	var list []string
	for _, f := range folds {
		if f != "" {
			list = append(list, f)
		}
	}
	if len(list) == 0 {
		return nil, nil
	}
	marks := strings.Repeat("?,", len(list))
	marks = marks[:len(marks)-1]
	args := make([]any, len(list))
	for i, f := range list {
		args[i] = f
	}
	rows, err := a.db.Query(`SELECT he, he_fold, pos, en FROM dictionary_entries WHERE he_fold IN (`+marks+`) ORDER BY id`, args...)
	if err != nil {
		if isNoTable(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var got []dictEntry
	for rows.Next() {
		var e dictEntry
		if err := rows.Scan(&e.he, &e.fold, &e.pos, &e.en); err != nil {
			return nil, err
		}
		got = append(got, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	index := map[string]int{}
	for i, f := range list {
		if _, ok := index[f]; !ok {
			index[f] = i
		}
	}
	for i := 1; i < len(got); i++ {
		j := i
		for j > 0 && index[got[j].fold] < index[got[j-1].fold] {
			got[j], got[j-1] = got[j-1], got[j]
			j--
		}
	}
	if len(got) > limit {
		got = got[:limit]
	}
	return got, nil
}

func (a *App) glossesForFold(fold string, limit int) ([]string, error) {
	if fold == "" {
		return nil, nil
	}
	rows, err := a.db.Query(`SELECT en FROM dictionary_glosses WHERE he_fold = ? ORDER BY length(en) LIMIT ?`, fold, limit)
	if err != nil {
		if isNoTable(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var en string
		if err := rows.Scan(&en); err != nil {
			return nil, err
		}
		out = append(out, en)
	}
	return out, rows.Err()
}

func shortGloss(text string) string {
	parts := regexp.MustCompile(`[;,]`).Split(text, -1)
	var kept []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			kept = append(kept, p)
		}
		if len(kept) == 3 {
			break
		}
	}
	return strings.Join(kept, ", ")
}

func everydayFirst(rows []dictEntry, infinitive bool) []dictEntry {
	type scored struct {
		row dictEntry
		i   int
		n   int
	}
	items := make([]scored, len(rows))
	for i, row := range rows {
		n := 0
		if regexp.MustCompile(`(?i)^be\s`).MatchString(row.en) {
			n += 2
		}
		if regexp.MustCompile(`(?i)\(comput|, name\b|\bname \(`).MatchString(row.en) {
			n += 1
		}
		if !infinitive && strings.Contains(row.pos, "verb") {
			n += 1
		}
		items[i] = scored{row, i, n}
	}
	for i := 1; i < len(items); i++ {
		j := i
		for j > 0 && (items[j].n < items[j-1].n || (items[j].n == items[j-1].n && items[j].i < items[j-1].i)) {
			items[j], items[j-1] = items[j-1], items[j]
			j--
		}
	}
	out := make([]dictEntry, len(items))
	for i, item := range items {
		out[i] = item.row
	}
	return out
}

var finalLetter = map[rune]rune{'צ': 'ץ', 'כ': 'ך', 'מ': 'ם', 'נ': 'ן', 'פ': 'ף'}
var prefixes = []rune{'ה', 'ו', 'ב', 'כ', 'ל', 'מ', 'ש'}
var suffixes = []string{"ים", "ות", "ה", "ת"}

func candidates(fold string) []string {
	out := []string{}
	seen := map[string]bool{fold: true}
	add := func(form string) bool {
		if utf8.RuneCountInString(form) < 2 {
			return false
		}
		rs := []rune(form)
		last := rs[len(rs)-1]
		shaped := form
		if rep, ok := finalLetter[last]; ok {
			shaped = string(rs[:len(rs)-1]) + string(rep)
		}
		fresh := false
		for _, variant := range []string{form, shaped} {
			if seen[variant] {
				continue
			}
			seen[variant] = true
			out = append(out, variant)
			fresh = true
		}
		return fresh
	}
	prefixForms := func(form string) []string {
		rs := []rune(form)
		var forms []string
		if len(rs) >= 3 && isPrefix(rs[0]) {
			forms = append(forms, string(rs[1:]))
		}
		if len(rs) >= 4 && isPrefix(rs[0]) && isPrefix(rs[1]) {
			forms = append(forms, string(rs[2:]))
		}
		return forms
	}
	suffixForms := func(form string) []string {
		var forms []string
		rs := []rune(form)
		for _, s := range suffixes {
			sr := []rune(s)
			if strings.HasSuffix(form, s) && len(rs)-len(sr) >= 2 {
				forms = append(forms, string(rs[:len(rs)-len(sr)]))
			}
		}
		return forms
	}
	innerForms := func(form string) []string {
		rs := []rune(form)
		var forms []string
		for i := 1; i < len(rs)-1; i++ {
			if rs[i] == 'ו' || rs[i] == 'י' {
				forms = append(forms, string(rs[:i])+string(rs[i+1:]))
			}
		}
		return forms
	}
	layer := func(bases []string, step func(string) []string) []string {
		var next []string
		for _, base := range bases {
			for _, form := range step(base) {
				if add(form) {
					next = append(next, form)
				}
			}
		}
		return next
	}
	withPrefix := layer([]string{fold}, prefixForms)
	stems := append([]string{fold}, withPrefix...)
	withInner := layer(stems, innerForms)
	withSuffix := layer(append(append([]string{}, stems...), withInner...), suffixForms)
	layer(withSuffix, innerForms)
	layer(withSuffix, prefixForms)
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

func isPrefix(r rune) bool {
	for _, p := range prefixes {
		if r == p {
			return true
		}
	}
	return false
}
