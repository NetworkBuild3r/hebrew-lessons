package pages

import "strings"

// Text limits match the Node catalog and explainer. Counts are UTF-16 code
// units, which is what JavaScript String length uses. Hebrew and nikud are in
// the BMP, so this is the same as a rune count for those strings. The string
// is never normalized: NFC and NFD stay as they were stored.
const (
	CatalogTextLimit   = 120
	ExplainerTextLimit = 4000
)

// ClipJS returns the prefix of s that fits in n UTF-16 code units.
func ClipJS(s string, n int) string {
	if n <= 0 || s == "" {
		return ""
	}
	units := 0
	for i, r := range s {
		w := 1
		if r > 0xFFFF {
			w = 2
		}
		if units+w > n {
			return s[:i]
		}
		units += w
	}
	return s
}

// HasHebrew reports whether s contains a Hebrew block character, including nikud.
func HasHebrew(s string) bool {
	for _, r := range s {
		if r >= 0x0590 && r <= 0x05FF {
			return true
		}
	}
	return false
}

// HasLatin reports whether s contains an English letter.
func HasLatin(s string) bool {
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return true
		}
	}
	return false
}

// LangDir is the lang and dir for a title or topic. Hebrew wins when both
// scripts are present, matching guides-page.js isHebrew.
func LangDir(s string) (lang, dir string) {
	if HasHebrew(s) {
		return "he", "rtl"
	}
	return "en", "ltr"
}

// UILangDir is the document language and direction for a Go page.
// The interface language decides: he is Hebrew and right to left, and
// anything else (including en) is English and left to right.
func UILangDir(lang string) (uiLang, dir string) {
	if lang == "he" {
		return "he", "rtl"
	}
	return "en", "ltr"
}

// TextPart is one slice of a mixed string. A Hebrew part is a maximal run of
// the Hebrew block. Spaces between Hebrew letters stay inside that run.
// Maqaf (U+05BE) is already in the Hebrew block, so it stays in the run too.
type TextPart struct {
	Hebrew bool
	Text   string
}

// TextParts splits s into Hebrew runs and the text around them.
// The string is not normalized.
func TextParts(s string) []TextPart {
	rs := []rune(s)
	var parts []TextPart
	i := 0
	for i < len(rs) {
		if !isHebrewBlock(rs[i]) {
			j := i + 1
			for j < len(rs) && !isHebrewBlock(rs[j]) {
				j++
			}
			parts = append(parts, TextPart{Text: string(rs[i:j])})
			i = j
			continue
		}
		j := i + 1
		for j < len(rs) {
			if isHebrewBlock(rs[j]) {
				j++
				continue
			}
			k := j
			for k < len(rs) && isRunGap(rs[k]) {
				k++
			}
			if k > j && k < len(rs) && isHebrewBlock(rs[k]) {
				j = k
				continue
			}
			break
		}
		parts = append(parts, TextPart{Hebrew: true, Text: string(rs[i:j])})
		i = j
	}
	return parts
}

func isHebrewBlock(r rune) bool { return r >= 0x0590 && r <= 0x05FF }

func isRunGap(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// isPoint is a Hebrew point (nikud), not a punctuation mark that shares the
// block. Maqaf, paseq, sof pasuq, and nun hafukha stay in the word.
func isPoint(r rune) bool {
	if r >= 0x0591 && r <= 0x05BD {
		return true
	}
	if r == 0x05BF {
		return true
	}
	if r >= 0x05C1 && r <= 0x05C2 {
		return true
	}
	return r >= 0x05C4 && r <= 0x05C7
}

func isBase(r rune) bool { return r >= 0x05D0 && r <= 0x05EA }

// escapeHTML matches HebrewUtils.escapeHtml: & < > ". Apostrophes stay.
func escapeHTML(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
