// Package ucd is the Unicode property table the plan validators use.
// The tables are Unicode 16.0, the version Node 22.14 reports in
// process.versions.unicode. Go 1.26's unicode package is Unicode 15, and
// Go 1.27's is Unicode 17, so lexicon keys must not call unicode.Is.
package ucd

// Is reports a general-category class (\p{L}, \p{M}, \p{N}, \p{P}, \p{S},
// \p{Z}, \p{C}), the specific category Cf, or the whitespace predicates
// js (\s and String.prototype.trim) and ws (Unicode White_Space).
func Is(prop string, r rune) bool {
	switch prop {
	case "L":
		return IsLetter(r)
	case "M":
		return IsMark(r)
	case "N":
		return IsNumber(r)
	case "P":
		return IsPunct(r)
	case "S":
		return IsSymbol(r)
	case "Z":
		return IsSeparator(r)
	case "C":
		return IsOther(r)
	case "Cf":
		return IsFormat(r)
	case "js":
		return JSSpace(r)
	case "ws":
		return WhiteSpace(r)
	default:
		return false
	}
}

// IsLetter is \p{L}: Lu, Ll, Lt, Lm, or Lo.
func IsLetter(r rune) bool { return major(r) == 'L' }

// IsMark is \p{M}.
func IsMark(r rune) bool { return major(r) == 'M' }

// IsNumber is \p{N}.
func IsNumber(r rune) bool { return major(r) == 'N' }

// IsPunct is \p{P}.
func IsPunct(r rune) bool { return major(r) == 'P' }

// IsSymbol is \p{S}.
func IsSymbol(r rune) bool { return major(r) == 'S' }

// IsSeparator is \p{Z}.
func IsSeparator(r rune) bool { return major(r) == 'Z' }

// IsOther is \p{C}, including format characters.
func IsOther(r rune) bool { return major(r) == 'C' }

// IsFormat is \p{Cf}.
func IsFormat(r rune) bool { return category(r) == "Cf" }

// IsEdgeMark is the trimEdges class [\p{P}\p{S}\p{Cf}].
func IsEdgeMark(r rune) bool {
	return IsPunct(r) || IsSymbol(r) || IsFormat(r)
}

func major(r rune) byte {
	name := category(r)
	if name == "" {
		return 'C'
	}
	return name[0]
}
