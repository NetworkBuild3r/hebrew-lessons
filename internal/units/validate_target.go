package units

import (
	"context"
	"fmt"
)

// gradedActivity is an activity the player scores. Those four share Activity,
// and they are the only types that may set Target.
func gradedActivity(typ string) bool {
	switch typ {
	case ActivityRecall, ActivityListenPick, ActivityProduce, ActivityCloze:
		return true
	default:
		return false
	}
}

// targetRequired is a graded activity that grades one word of a multi-word
// line. listen_pick may omit target. Its translation is collected and
// validated now, and whole-sentence choices are planned with #139.
// produce grades one word, the same as recall and cloze.
func targetRequired(typ string) bool {
	switch typ {
	case ActivityRecall, ActivityProduce, ActivityCloze:
		return true
	default:
		return false
	}
}

// targetToken returns the activity token whose normalized text equals the
// target. The comparison is Normalize on both sides, the same key LexemeByText
// uses. The returned token is the activity's own spelling, so a
// differently composed target still names that token.
func targetToken(act Activity) (string, bool) {
	want := normalize(act.Target)
	if want == "" {
		return "", false
	}
	for _, text := range act.Texts() {
		for _, tok := range tokenize(text) {
			if normalize(tok) == want {
				return tok, true
			}
		}
	}
	return "", false
}

// targetIssues checks every graded activity's target.
// A recall, produce, or cloze of more than one lexicon word must name the
// word it grades. listen_pick may omit target. ModeWrite blocks a missing
// target. ModePublished warns, which is how the current published revision
// keeps validating. A target that is set and is not one of the activity's
// tokens, or whose token has no lexicon entry, is a block in both modes.
// The two cases are separate sentences. The check runs on the real document,
// not the flattened copy.
func targetIssues(ctx context.Context, doc *Unit, lk Lookup, mode Mode) []Issue {
	if doc == nil {
		return nil
	}
	var out []Issue
	type hit struct {
		lesson, activity int
		tok              string
		act              Activity
	}
	var need []hit
	var s *scan
	if lk != nil {
		s = &scan{ctx: ctx, lk: lk, position: doc.Position, cache: map[string]*probe{}}
	}
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			ptr := at("lessons", i, "activities", j, "target")
			if normalize(act.Target) == "" {
				if s == nil || !targetRequired(act.Type) {
					continue
				}
				n := lexemeTokenCount(s, act)
				if s.err != nil {
					return append(out, lookupIssue(s.err))
				}
				if n > 1 {
					sev := SeverityBlock
					if mode == ModePublished {
						sev = SeverityWarn
					}
					out = append(out, issue(RuleActivityTarget, sev, ptr,
						fmt.Sprintf("activity %d: this line has several words; set target to the exact pointed word to grade.", j+1)))
				}
				continue
			}
			where := fmt.Sprintf("The %s activity in lesson %d, activity %d,", act.Type, i+1, j+1)
			if !gradedActivity(act.Type) {
				out = append(out, issue(RuleActivityTarget, SeverityBlock, ptr,
					fmt.Sprintf("%s has a target, and only a recall, listen_pick, produce, or cloze activity may set one.", where)))
				continue
			}
			tok, ok := targetToken(act)
			if !ok {
				out = append(out, issue(RuleActivityTarget, SeverityBlock, ptr,
					fmt.Sprintf("%s has a target %s that is not one of that activity's tokens.", where, strconvQuote(act.Target))))
				continue
			}
			need = append(need, hit{lesson: i, activity: j, tok: tok, act: act})
		}
	}
	if len(need) == 0 || s == nil {
		return out
	}
	for _, h := range need {
		// The player looks the token up by its exact pointed text (LexemesByText).
		// The consonant skeleton that inLexicon accepts is not enough: a lexicon
		// row for גר does not render choices for a target גָּר.
		if s.probeLex(normalize(h.tok)) {
			continue
		}
		if s.err != nil {
			return append(out, lookupIssue(s.err))
		}
		ptr := at("lessons", h.lesson, "activities", h.activity, "target")
		where := fmt.Sprintf("The %s activity in lesson %d, activity %d,", h.act.Type, h.lesson+1, h.activity+1)
		out = append(out, issue(RuleActivityTarget, SeverityBlock, ptr,
			fmt.Sprintf("%s has a target %s whose token has no lexicon entry.", where, strconvQuote(h.tok))))
	}
	return out
}

// lexemeTokenCount is how many tokens of the activity are lexicon entries.
// The match is LexemeByText on the token's exact pointed text, the same
// lookup the player uses (LexemesByText on each token of the activity).
// Punctuation is not a token. A numeral, an unknown word, and a word whose
// only lexicon row is the unpointed spelling are not counted. The unpointed
// fallback that inLexicon accepts does not count here. translation_missing
// uses this same count.
func lexemeTokenCount(s *scan, act Activity) int {
	n := 0
	for _, text := range act.Texts() {
		for _, tok := range tokenize(text) {
			if s.err != nil {
				return n
			}
			if s.probeLex(normalize(tok)) {
				n++
			}
		}
	}
	return n
}
