package units

import "fmt"

// Noun gender and plural come from Lookup.Facts, not from Hebrew morphology.
// The validator does not guess a gender from the ending and does not decide
// that a plural is irregular because it fails to end in ים or ות.
//
// A newWords entry is a noun when Facts reports POS "noun" or "n"
// (case-insensitive). Other parts of speech are not checked.
//
// Gender blocks when that noun's gender is not "m", "f", or "both" after
// trimming and lowercasing. Empty and any other value count as missing.
// The pointer is the newWords entry.
//
// Irregular plural: the lexicon says so. A noun is irregular when
// IrregularPlural is true or Plural normalizes to a non-empty string.
// A stored plural is itself the signal; regular plurals are left empty.
// The unit gives that plural when some activity token matches it under
// normalize and the niqqud-insensitive skeleton, anywhere in the unit.
// When the flag is set and Plural is empty, the unit cannot give a form,
// so noun.plural warns. The pointer is the newWords entry.

func nounIssues(s *scan, doc *Unit, words []wordRef) []Issue {
	var out []Issue
	var tokens []string
	for _, les := range doc.Lessons {
		for _, act := range les.Activities {
			tokens = append(tokens, tokenize(act.Text)...)
		}
	}
	for _, w := range words {
		if w.dup || w.norm == "" {
			continue
		}
		f, ok := s.factsFor(w.norm)
		if s.err != nil {
			return out
		}
		if !ok || !nounPOS(f.POS) {
			continue
		}
		if !genderOK(f.Gender) {
			out = append(out, issue(RuleNounGender, SeverityBlock, at("newWords", w.index),
				fmt.Sprintf("new noun %s has no gender; expected m, f, or both", strconvQuote(w.raw))))
		}
		plural := normalize(f.Plural)
		if !f.IrregularPlural && plural == "" {
			continue
		}
		if plural == "" || !unitHasToken(tokens, plural) {
			msg := fmt.Sprintf("new noun %s has an irregular plural that the unit does not show", strconvQuote(w.raw))
			if plural != "" {
				msg = fmt.Sprintf("new noun %s has irregular plural %s, and the unit does not show it", strconvQuote(w.raw), strconvQuote(plural))
			}
			out = append(out, issue(RuleNounPlural, SeverityWarn, at("newWords", w.index), msg))
		}
	}
	return out
}

func unitHasToken(tokens []string, plural string) bool {
	pb := stripNiqqud(plural)
	for _, tok := range tokens {
		n := normalize(tok)
		if n == plural || stripNiqqud(n) == pb {
			return true
		}
	}
	return false
}
