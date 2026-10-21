package units

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// translationIssues checks author-written line translations.
// A listen_pick of two or more words that are in the lexicon must set
// translation. The count is lexemeTokenCount, the same one activity.target
// uses. ModeWrite blocks a missing translation. ModePublished warns, which
// is how the current published revision keeps validating.
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
			n := lexemeTokenCount(s, act)
			if s.err != nil {
				return append(out, lookupIssue(s.err))
			}
			// Hebrew Dev's ruling of 2026-10-20: a line with exactly one lexicon word credits that word.
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

// translatedLineCount is the number of different English lines in the unit.
// Each activity translation and each dialogue-line translation is folded
// first: lowercase, strip punctuation, collapse spaces. Blank space is
// empty. Two lines that fold to the same English count once.
func translatedLineCount(doc *Unit) int {
	seen := map[string]bool{}
	var lines []string
	for _, les := range doc.Lessons {
		for _, act := range les.Activities {
			lines = append(lines, act.Translation)
			for _, line := range act.Lines {
				lines = append(lines, line.Translation)
			}
		}
	}
	n := 0
	for _, text := range lines {
		key := foldTranslation(text)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		n++
	}
	return n
}

// foldTranslation is the key used to tell English lines apart: lowercase,
// punctuation removed, and runs of space collapsed to one.
func foldTranslation(s string) string {
	var b strings.Builder
	gap := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsPunct(r) {
			continue
		}
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
