package units

import "fmt"

func lexemeIssues(s *scan, doc *Unit, words []wordRef) []Issue {
	var out []Issue
	declared := map[string]bool{}
	for _, w := range words {
		if w.norm == "" || w.dup {
			continue
		}
		declared[w.norm] = true
		declared[w.bare] = true
		if !s.inLexicon(w.norm) {
			if s.err != nil {
				return out
			}
			out = append(out, issue(RuleLexemeUnknown, SeverityBlock, at("newWords", w.index),
				fmt.Sprintf("new word %s is not in the lexicon", strconvQuote(w.raw))))
		}
	}
	if s.err != nil {
		return out
	}
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			var unknown []string
			seen := map[string]bool{}
			for _, tok := range tokenize(act.Text) {
				if !isHebrewWord(tok) || isNumberTok(tok) || s.isName(tok) {
					if s.err != nil {
						return out
					}
					continue
				}
				n := normalize(tok)
				b := stripNiqqud(n)
				if declared[n] || declared[b] {
					continue
				}
				if s.inLexicon(n) {
					continue
				}
				if s.err != nil {
					return out
				}
				if !seen[n] {
					seen[n] = true
					unknown = append(unknown, tok)
				}
			}
			if len(unknown) > 0 {
				out = append(out, issue(RuleLexemeUnknown, SeverityBlock, at("lessons", i, "activities", j, "text"),
					fmt.Sprintf("unknown lexeme %s", quoteList(unknown))))
			}
		}
	}
	return out
}
