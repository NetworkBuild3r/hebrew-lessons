package units

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Text normalization
//
// Lookup keys and activity text are compared after normalize:
//  1. Drop Unicode format characters (bidi marks, zero-width spaces, BOM).
//  2. Apply Unicode NFC.
//  3. Trim the string and collapse every run of Unicode spaces to one
//     ASCII space.
//
// LexemeByText, Facts, and HasAudio are called with that string. Audio
// stays exact after this step: a different pointing is a different clip,
// because clips are keyed by pointed text.
//
// Lexeme identity then falls back to a consonant skeleton when the exact
// string misses. The skeleton drops niqqud, meteg, rafe, cantillation, and
// the upper dots, and it keeps shin and sin dots because those change the
// letter. The fallback looks the skeleton up as its own key and compares
// it with KnownLexemes (words introduced in earlier units) and this unit's
// newWords. It does not scan the rest of the lexicon; Lookup has no list method.
//
// Tokens split on whitespace, punctuation, and maqaf (U+05BE). A Hebrew
// word keeps its points. A digit run is its own token. Punctuation is not
// a token, so coverage and the story-pointing rule ignore it.
//
// A token is pointed when it contains a vowel point, dagesh, meteg, rafe,
// or qamats qatan (U+05B0–U+05BD, U+05BF, U+05C7). Shin and sin dots alone
// do not count: they do not resolve a vowel homograph. Cantillation alone
// does not count either.
//
// A token is a number when it is all decimal digits, or when it is Hebrew
// numeral notation. A one-letter numeral is a Hebrew letter plus geresh
// (U+05F3), as in ה׳. A longer numeral is Hebrew letters with gershayim
// (U+05F4) immediately before the last letter, as in י״ב or תשפ״ז.
// A geresh inside a loanword, as in צ׳יפס or ג׳ירפה, is not a number.

// Normalize is the shared text key for units and audio.
// Importers store this string, and Lookup.HasAudio is called with it.
func Normalize(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
	s = norm.NFC.String(s)
	var b strings.Builder
	gap := false
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsSpace(r) {
			gap = true
			continue
		}
		if gap && b.Len() > 0 {
			b.WriteByte(' ')
		}
		gap = false
		b.WriteRune(r)
	}
	return b.String()
}

func normalize(s string) string { return Normalize(s) }

func stripNiqqud(s string) string {
	return strings.Map(func(r rune) rune {
		if isStrippedMark(r) {
			return -1
		}
		return r
	}, s)
}

func isStrippedMark(r rune) bool {
	switch {
	case r >= 0x0591 && r <= 0x05AF:
		return true
	case r >= 0x05B0 && r <= 0x05BD:
		return true
	case r == 0x05BF:
		return true
	case r == 0x05C4 || r == 0x05C5 || r == 0x05C7:
		return true
	default:
		return false
	}
}

func isPointed(tok string) bool {
	for _, r := range tok {
		if (r >= 0x05B0 && r <= 0x05BD) || r == 0x05BF || r == 0x05C7 {
			return true
		}
	}
	return false
}

func isHebrewLetter(r rune) bool {
	return r >= 0x05D0 && r <= 0x05EA
}

func isHebrewWord(tok string) bool {
	for _, r := range tok {
		if isHebrewLetter(r) {
			return true
		}
	}
	return false
}

func isNumberTok(tok string) bool {
	if tok == "" {
		return false
	}
	var sig []rune
	for _, r := range tok {
		if isStrippedMark(r) || r == 0x05C1 || r == 0x05C2 {
			continue
		}
		sig = append(sig, r)
	}
	if len(sig) == 0 {
		return false
	}
	digits := true
	for _, r := range sig {
		if r < '0' || r > '9' {
			digits = false
			break
		}
	}
	if digits {
		return true
	}
	// ה׳ : one letter, then geresh.
	if len(sig) == 2 && isHebrewLetter(sig[0]) && sig[1] == 0x05F3 {
		return true
	}
	// תשפ״ז, י״ב: gershayim immediately before the last letter.
	if len(sig) >= 3 && sig[len(sig)-2] == 0x05F4 {
		for i, r := range sig {
			if i == len(sig)-2 {
				continue
			}
			if !isHebrewLetter(r) {
				return false
			}
		}
		return true
	}
	return false
}

func containsHebrew(s string) bool {
	for _, r := range s {
		if isHebrewLetter(r) {
			return true
		}
	}
	return false
}

type tokClass int

const (
	classNone tokClass = iota
	classHebrew
	classDigit
	classLatin
)

func runeClass(r rune) tokClass {
	switch {
	case r >= '0' && r <= '9':
		return classDigit
	case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
		return classLatin
	case isHebrewLetter(r):
		return classHebrew
	case isStrippedMark(r) || r == 0x05C1 || r == 0x05C2 || r == 0x05F3 || r == 0x05F4:
		return classHebrew
	default:
		return classNone
	}
}

func tokenize(s string) []string {
	s = normalize(s)
	var out []string
	var b strings.Builder
	cur := classNone
	flush := func() {
		if b.Len() == 0 {
			return
		}
		out = append(out, b.String())
		b.Reset()
		cur = classNone
	}
	for _, r := range s {
		c := runeClass(r)
		if c == classNone {
			flush()
			continue
		}
		if c == classHebrew && cur == classHebrew {
			b.WriteRune(r)
			continue
		}
		if c == cur && c != classHebrew {
			b.WriteRune(r)
			continue
		}
		if cur != classNone {
			flush()
		}
		cur = c
		b.WriteRune(r)
	}
	flush()
	return out
}

func percentTenths(known, total int) int {
	if total <= 0 {
		return 1000
	}
	return known * 1000 / total
}

func formatPercent(known, total int) string {
	tenths := percentTenths(known, total)
	return itoa(tenths/10) + "." + itoa(tenths%10) + "%"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d [8]byte
	i := len(d)
	for n > 0 {
		i--
		d[i] = byte('0' + n%10)
		n /= 10
	}
	return string(d[i:])
}
