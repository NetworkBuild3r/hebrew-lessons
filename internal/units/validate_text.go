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
// A token is a number when it is all decimal digits, or when it is a
// Hebrew numeral in standard gematria order. A one-letter numeral is one
// non-final letter plus geresh (U+05F3), as in ה׳. A longer numeral is
// non-final letters with gershayim (U+05F4) immediately before the last
// letter, as in י״ב or תשפ״ז.
//
// Reading order is string order: the first rune is the rightmost letter.
// Letter values must be non-increasing in that order. The only repeat
// allowed is ת, as in תתק and תתקע״ה. Fifteen and sixteen are only ט״ו
// and ט״ז, so י״ה and י״ו are words, including when that pair is the
// tens and the ones of a longer number. Final forms (ךםןףץ) are never
// numerals. A geresh inside a loanword, as in צ׳יפס, is not a number.
// ASCII quotes (י"ב, ה') are not U+05F3 or U+05F4, so they are not
// numerals either.

func normalize(s string) string {
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
	return hebrewNumeral(sig)
}

// hebrewNumeral reports whether sig is standard gematria notation.
// The caller has already dropped niqqud. See the number comment above.
func hebrewNumeral(sig []rune) bool {
	var letters []rune
	switch {
	case len(sig) == 2 && sig[1] == 0x05F3:
		letters = sig[:1]
	case len(sig) >= 3 && sig[len(sig)-2] == 0x05F4:
		letters = make([]rune, 0, len(sig)-1)
		for i, r := range sig {
			if i == len(sig)-2 {
				continue
			}
			letters = append(letters, r)
		}
	default:
		return false
	}
	var prev int
	for i, r := range letters {
		v, ok := gematriaValue(r)
		if !ok {
			return false
		}
		if i == 0 {
			prev = v
			continue
		}
		if v > prev {
			return false
		}
		if v == prev && r != 'ת' {
			return false
		}
		// 15 and 16 are ט״ו and ט״ז, never י followed by ה or ו.
		if letters[i-1] == 'י' && (r == 'ה' || r == 'ו') {
			return false
		}
		prev = v
	}
	return len(letters) > 0
}

// gematriaValue is the standard value of a non-final letter.
// Final forms are not numerals, so they return false.
func gematriaValue(r rune) (int, bool) {
	switch r {
	case 'א':
		return 1, true
	case 'ב':
		return 2, true
	case 'ג':
		return 3, true
	case 'ד':
		return 4, true
	case 'ה':
		return 5, true
	case 'ו':
		return 6, true
	case 'ז':
		return 7, true
	case 'ח':
		return 8, true
	case 'ט':
		return 9, true
	case 'י':
		return 10, true
	case 'כ':
		return 20, true
	case 'ל':
		return 30, true
	case 'מ':
		return 40, true
	case 'נ':
		return 50, true
	case 'ס':
		return 60, true
	case 'ע':
		return 70, true
	case 'פ':
		return 80, true
	case 'צ':
		return 90, true
	case 'ק':
		return 100, true
	case 'ר':
		return 200, true
	case 'ש':
		return 300, true
	case 'ת':
		return 400, true
	default:
		return 0, false
	}
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
