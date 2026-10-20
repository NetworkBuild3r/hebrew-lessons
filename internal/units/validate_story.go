package units

import "fmt"

// Story pointing
//
// story.unpointed blocks when a story activity contains a Hebrew word
// token with no niqqud. listen_pick and the other activity types are not
// checked. The pointer is the activity text.
//
// Ignored, and not reported:
//   - punctuation (it is not a token)
//   - numbers: decimal digits, or gematria in rank order (ה׳, תשפ״ז, ט״ו).
//     Acronyms such as צה״ל and ע״י, and loanwords such as צ׳יפס, are
//     words. ASCII quotes are punctuation, so י"ב is the two words י and ב,
//     and each is flagged. Spellings that are also valid numerals, such as
//     ת״א and ש״ס, are numbers.
//   - names, meaning a lexicon entry whose part of speech is name, propn,
//     or proper noun, matched on the normalized token or its consonant
//     skeleton
//
// Homographs are not exempt. An unpointed token blocks even when its
// consonant skeleton is shared by more than one lexicon entry. The rule
// does not need a homograph flag to do that: every unpointed content word
// blocks, and a homograph is a content word unless it is a name.
//
// Other exemptions: none. A word with only a shin or sin dot, or only
// cantillation, is unpointed. Latin text is not Hebrew, so this rule
// skips it. Lexeme checks also skip Latin; they apply to Hebrew word tokens.

// story.unmapped is the story-token half of the lexicon check.
// lexeme.unknown does not also report that token.
//
// A content word maps when LexemeByText hits its normalized form, or
// hits its consonant skeleton as its own key. That is the same lookup
// inLexicon uses. A declared new word is not a separate pass: the token
// maps only when it is itself a lexicon entry, which is what a gloss
// needs. Names (part of speech name, propn, or proper noun) and numbers
// are exempt. Punctuation is not a token. Latin text is not Hebrew.
//
// An unpointed homograph maps only when that lookup hits. Sharing a bare
// spelling with some other lexicon entry does not count; the validator
// does not scan the lexicon by bare form.

func storyUnmappedIssues(s *scan, doc *Unit) []Issue {
	var out []Issue
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			if !storyLike(act.Type) {
				continue
			}
			var bad []string
			seen := map[string]bool{}
			for _, tok := range tokenize(act.Text) {
				if !isHebrewWord(tok) || isNumberTok(tok) || s.isName(tok) {
					if s.err != nil {
						return out
					}
					continue
				}
				if s.inLexicon(tok) {
					if s.err != nil {
						return out
					}
					continue
				}
				if s.err != nil {
					return out
				}
				n := normalize(tok)
				if seen[n] {
					continue
				}
				seen[n] = true
				bad = append(bad, tok)
			}
			if len(bad) == 0 {
				continue
			}
			out = append(out, issue(RuleStoryUnmapped, SeverityBlock, at("lessons", i, "activities", j, "text"),
				fmt.Sprintf("story word %s does not map to a lexeme; names and numbers are exempt", quoteList(bad))))
		}
	}
	return out
}

func storyPointIssues(s *scan, doc *Unit) []Issue {
	var out []Issue
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			if !storyLike(act.Type) {
				continue
			}
			var bad []string
			seen := map[string]bool{}
			for _, tok := range tokenize(act.Text) {
				if !isHebrewWord(tok) || isNumberTok(tok) || isPointed(tok) {
					continue
				}
				if s.isName(tok) {
					if s.err != nil {
						return out
					}
					continue
				}
				if s.err != nil {
					return out
				}
				if seen[tok] {
					continue
				}
				seen[tok] = true
				bad = append(bad, tok)
			}
			if len(bad) == 0 {
				continue
			}
			out = append(out, issue(RuleStoryUnpointed, SeverityBlock, at("lessons", i, "activities", j, "text"),
				fmt.Sprintf("story has unpointed Hebrew %s; names, numbers, and punctuation are ignored, and homographs stay pointed", quoteList(bad))))
		}
	}
	return out
}
