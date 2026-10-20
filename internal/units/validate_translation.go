package units

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// translationIssues checks author-written line translations.
// A listen_pick of two or more lexicon tokens must set translation.
// ModeWrite blocks a missing translation. ModePublished warns, which is how
// a revision published before the field was required keeps validating.
// Language and length apply to every translation, on any activity and on
// every dialogue line, and they keep their catalog severity in both modes.
func translationIssues(ctx context.Context, doc *Unit, lk Lookup, mode Mode) []Issue {
	if doc == nil {
		return nil
	}
	var out []Issue
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			out = append(out, translationFieldIssues(at("lessons", i, "activities", j, "translation"), act.Translation)...)
			for k, line := range act.Lines {
				out = append(out, translationFieldIssues(at("lessons", i, "activities", j, "lines", k, "translation"), line.Translation)...)
			}
		}
	}
	if lk == nil {
		return out
	}
	s := &scan{ctx: ctx, lk: lk, position: doc.Position, cache: map[string]*probe{}}
	sentences := 0
	var firstPtr string
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			if act.Type != ActivityListenPick {
				continue
			}
			n := lexiconTokenCount(s, act)
			if s.err != nil {
				return append(out, lookupIssue(s.err))
			}
			if n < 2 {
				continue
			}
			ptr := at("lessons", i, "activities", j, "translation")
			if sentences == 0 {
				firstPtr = ptr
			}
			sentences++
			if strings.TrimSpace(act.Translation) != "" {
				continue
			}
			sev := SeverityBlock
			if mode == ModePublished {
				sev = SeverityWarn
			}
			out = append(out, issue(RuleUnitTranslationMissing, sev, ptr,
				fmt.Sprintf("activity %d: this listen_pick has several words in the lexicon; set translation to the English of the whole line.", j+1)))
		}
	}
	if sentences > 0 && translatedLineCount(doc) < MinTranslatedLines {
		out = append(out, issue(RuleUnitTranslationDistractors, SeverityWarn, firstPtr,
			fmt.Sprintf("this unit has a sentence listen_pick and fewer than %d translated lines.", MinTranslatedLines)))
	}
	return out
}

// translationFieldIssues checks language and length. An empty translation
// has nothing to check. Whitespace counts toward the length.
func translationFieldIssues(pointer, text string) []Issue {
	if text == "" {
		return nil
	}
	var out []Issue
	if translationNotEnglish(text) {
		out = append(out, issue(RuleUnitTranslationLang, SeverityBlock, pointer,
			"translation contains Hebrew or markup, and a translation is English plain text."))
	}
	if utf8.RuneCountInString(text) > TranslationMaxLen {
		out = append(out, issue(RuleUnitTranslationLength, SeverityWarn, pointer,
			fmt.Sprintf("translation is longer than %d characters.", TranslationMaxLen)))
	}
	return out
}

// translationNotEnglish reports Hebrew in U+0590 through U+05FF, or the same
// markup unit.no_markup rejects. The Hebrew block includes points and Hebrew
// punctuation, not only the letters.
func translationNotEnglish(s string) bool {
	if hasMarkup(s) {
		return true
	}
	for _, form := range []string{s, norm.NFKC.String(s)} {
		for _, r := range form {
			if r >= 0x0590 && r <= 0x05FF {
				return true
			}
		}
	}
	return false
}

// lexiconTokenCount is how many tokens of the activity are Hebrew words in
// the lexicon. An exact pointed entry counts, and so does the unpointed
// spelling, the same match inLexicon uses. A number is not a lexicon token.
// A word that is not in the lexicon is not one either.
func lexiconTokenCount(s *scan, act Activity) int {
	n := 0
	for _, text := range act.Texts() {
		for _, tok := range tokenize(text) {
			if s.err != nil {
				return n
			}
			if !isHebrewWord(tok) || isNumberTok(tok) {
				continue
			}
			if s.inLexicon(tok) {
				n++
			}
		}
	}
	return n
}

// translatedLineCount is every non-empty translation in the unit: each
// activity, then each dialogue line. Blank space is empty. The same English
// on two lines counts twice.
func translatedLineCount(doc *Unit) int {
	n := 0
	for _, les := range doc.Lessons {
		for _, act := range les.Activities {
			if strings.TrimSpace(act.Translation) != "" {
				n++
			}
			for _, line := range act.Lines {
				if strings.TrimSpace(line.Translation) != "" {
					n++
				}
			}
		}
	}
	return n
}
