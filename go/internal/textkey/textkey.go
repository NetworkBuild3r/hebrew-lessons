// Package textkey is the ADR-0001 exactKey used for a word.
//
// For a string, these Node functions are the same steps: NFC, collapse
// ECMAScript \s to one space, trim. None of them lowercase.
//
//	server/vocab-index.js exactKey
//	server/learner/library.js (that export)
//	server/db/repos/user-words.js exactKey
//	server/db/repos/lexicon.js exactKey
//	server/learner/mix.js exactKey
//	server/plans/validate.js exactKey
//	server/plans/hebrew-checks.js exactKey
//	shared/player/review.js keyFor
//	shared/activities/story.js exactKey
//
// The null wrappers differ. The library turns null and undefined into "".
// The plan checkers use String(value), so null becomes "null" and undefined
// becomes "undefined". This function takes a Go string, so that split does
// not arise. Call sites in the plan checkers pass strings.
//
// Other helpers are different operations and are not this function:
// plan-render safeLexiconKey trims and does not collapse \s;
// plan-builder factsFromPlan bare() strips U+0591–U+05C7 and does not trim;
// lexicon validate asText is NFC only; lexicon seed cleanField is NFC and
// trim for gloss text; strippedKey and vowel-strip helpers remove points.
package textkey

import (
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// jsSpace is the ECMAScript WhiteSpace and LineTerminator set, which is
// what /\s/ and String.prototype.trim use when the unicode flag is absent.
//
// Node 22.14.0 reports process.versions.unicode 16.0. The set below is
// every scalar that Node's /\s/ and trim accept. It is not Go's
// unicode.Zs and it is not unicode.IsSpace: U+0085 is absent, and U+180E
// is absent. U+FEFF is present.
//
//	U+0009 U+000A U+000B U+000C U+000D U+0020 U+00A0
//	U+1680 U+2000 U+2001 U+2002 U+2003 U+2004 U+2005 U+2006
//	U+2007 U+2008 U+2009 U+200A U+2028 U+2029 U+202F U+205F
//	U+3000 U+FEFF
func jsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ',
		'\u00A0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005',
		'\u2006', '\u2007', '\u2008', '\u2009', '\u200A',
		'\u2028', '\u2029', '\u202F', '\u205F', '\u3000', '\uFEFF':
		return true
	default:
		return false
	}
}

// ExactKey is NFC, then a collapse of jsSpace runs to one U+0020, then a
// trim of jsSpace. A string that is not valid UTF-8 returns empty. Node
// strings are well-formed UTF-16, so the golden inputs are valid UTF-8.
func ExactKey(s string) string {
	if s == "" || !utf8.ValidString(s) {
		return ""
	}
	s = norm.NFC.String(s)
	return trimSpace(collapseSpace(s))
}

func collapseSpace(s string) string {
	var b []byte
	space := false
	wrote := false
	for _, r := range s {
		if jsSpace(r) {
			if wrote {
				space = true
			}
			continue
		}
		if space {
			b = append(b, ' ')
			space = false
		}
		b = utf8.AppendRune(b, r)
		wrote = true
	}
	return string(b)
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) {
		r, size := utf8.DecodeRuneInString(s[start:])
		if !jsSpace(r) {
			break
		}
		start += size
	}
	end := len(s)
	for end > start {
		r, size := utf8.DecodeLastRuneInString(s[:end])
		if !jsSpace(r) {
			break
		}
		end -= size
	}
	return s[start:end]
}
