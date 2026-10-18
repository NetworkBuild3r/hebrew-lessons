package units

import "fmt"

// Coverage is by token. Punctuation is not a token. A token counts as known
// when it is a number, a name (lexicon part of speech name, propn, or
// proper noun), a lexeme in Lookup.KnownLexemes, or one of this unit's
// newWords introduced in an earlier activity. The introducing activity
// itself does not yet count the word as known. Matching uses normalize
// and the niqqud-insensitive skeleton described in validate_text.go.
//
// Percent is known/total, truncated to one decimal (not rounded). The
// message also carries the exact fraction. Below 90% blocks. From 90% up
// to but not including 95% warns. 95% is clean.

func coverageIssues(s *scan, doc *Unit, words []wordRef) []Issue {
	var out []Issue
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			rule, label := coverageRule(act.Type)
			if rule == "" {
				continue
			}
			toks := coverageTokens(act.Text)
			if len(toks) == 0 {
				continue
			}
			known := 0
			for _, tok := range toks {
				if coverageKnown(s, tok, words, i, j) {
					if s.err != nil {
						return out
					}
					known++
					continue
				}
				if s.err != nil {
					return out
				}
			}
			tenths := percentTenths(known, len(toks))
			blockLine := CoverageBlockBelow * 10
			warnLine := CoverageWarnBelow * 10
			if tenths >= warnLine {
				continue
			}
			sev := SeverityWarn
			band := fmt.Sprintf("from %d%% up to but not including %d%% warns", CoverageBlockBelow, CoverageWarnBelow)
			if tenths < blockLine {
				sev = SeverityBlock
				band = fmt.Sprintf("below %d%% blocks", CoverageBlockBelow)
			}
			msg := fmt.Sprintf("%s coverage is %s (%d/%d tokens); %s", label, formatPercent(known, len(toks)), known, len(toks), band)
			out = append(out, issue(rule, sev, at("lessons", i, "activities", j), msg))
		}
	}
	return out
}

func coverageRule(typ string) (string, string) {
	switch typ {
	case ActivityStory:
		return RuleCoverageStory, "story"
	case ActivityListenPick:
		return RuleCoverageListening, "listening"
	default:
		return "", ""
	}
}

func coverageTokens(text string) []string {
	var out []string
	for _, tok := range tokenize(text) {
		if isHebrewWord(tok) || isNumberTok(tok) {
			out = append(out, tok)
		}
	}
	return out
}

func coverageKnown(s *scan, tok string, words []wordRef, lesson, activity int) bool {
	if isNumberTok(tok) || s.isName(tok) {
		return true
	}
	if s.learnerKnows(tok) {
		return true
	}
	n := normalize(tok)
	b := stripNiqqud(n)
	for _, w := range words {
		if w.dup || !w.found {
			continue
		}
		if w.norm != n && w.bare != b {
			continue
		}
		if wordBefore(w, lesson, activity) {
			return true
		}
	}
	return false
}
