package web

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

// spellSkip is Latin that is also an ordinary English word. A lesson must
// not turn "to" or "he" into Hebrew just because a lexicon row uses that spelling.
var spellSkip = map[string]bool{
	"a": true, "am": true, "an": true, "as": true, "at": true, "be": true,
	"by": true, "do": true, "he": true, "if": true, "in": true, "is": true,
	"it": true, "me": true, "my": true, "no": true, "of": true, "on": true,
	"or": true, "so": true, "to": true, "up": true, "us": true, "we": true,
}

// spellHebrew writes a lexicon word in pointed Hebrew wherever the sentence
// spells that word's transliteration. "lama li" becomes לָמָּה לִי. A one-off
// English word in spellSkip stays English, and two different words that share
// a transliteration are left alone.
func spellHebrew(text string, lex map[string]db.LexemeView) string {
	if text == "" || len(lex) == 0 {
		return text
	}
	byLatin := map[string]string{}
	ambiguous := map[string]bool{}
	for _, view := range lex {
		latin := strings.ToLower(strings.TrimSpace(view.Translit))
		if latin == "" || spellSkip[latin] || view.Text == "" {
			continue
		}
		if prev, ok := byLatin[latin]; ok && prev != view.Text {
			ambiguous[latin] = true
			continue
		}
		byLatin[latin] = view.Text
	}
	for latin := range ambiguous {
		delete(byLatin, latin)
	}
	if len(byLatin) == 0 {
		return text
	}
	latins := make([]string, 0, len(byLatin))
	for latin := range byLatin {
		latins = append(latins, latin)
	}
	sort.Slice(latins, func(i, j int) bool {
		if len(latins[i]) != len(latins[j]) {
			return len(latins[i]) > len(latins[j])
		}
		return latins[i] < latins[j]
	})
	lower := strings.ToLower(text)
	var b strings.Builder
	for i := 0; i < len(text); {
		matched := false
		if spellBefore(text, i) && spellStart(text[i]) {
			for _, latin := range latins {
				end := i + len(latin)
				if end <= len(lower) && lower[i:end] == latin && spellAfter(text, end) {
					b.WriteString(byLatin[latin])
					i = end
					matched = true
					break
				}
			}
		}
		if matched {
			continue
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		b.WriteRune(r)
		i += size
	}
	return b.String()
}

func spellBefore(text string, i int) bool {
	if i <= 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(text[:i])
	return !spellWord(r)
}

func spellAfter(text string, i int) bool {
	if i >= len(text) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(text[i:])
	return !spellWord(r)
}

func spellStart(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '\''
}

func spellWord(r rune) bool {
	return unicode.IsLetter(r) || r == '\'' || r == '’'
}
