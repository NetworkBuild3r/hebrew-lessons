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
//   - numbers, including decimal digits and Hebrew numerals or
//     abbreviations written with geresh or gershayim
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

func storyPointIssues(s *scan, doc *Unit) []Issue {
	var out []Issue
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			if act.Type != ActivityStory {
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
