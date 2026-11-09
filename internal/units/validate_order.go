package units

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// orderIssues checks order activities: token count, translation, and accept.
// INIT-029/SPEC-002.
func orderIssues(doc *Unit) []Issue {
	if doc == nil {
		return nil
	}
	var out []Issue
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			if act.Type != ActivityOrder {
				continue
			}
			ptr := at("lessons", i, "activities", j)
			out = append(out, orderTokenIssues(ptr+"/text", act.Text)...)
			out = append(out, orderTranslationIssues(ptr+"/translation", act.Translation)...)
			out = append(out, orderAcceptIssues(ptr, act.Text, act.Accept)...)
		}
	}
	return out
}

func orderTokenIssues(pointer, text string) []Issue {
	n := len(tokenize(text))
	if n >= MinOrderTokens && n <= MaxOrderTokens {
		return nil
	}
	return []Issue{issue(RuleUnitOrderTokens, SeverityBlock, pointer,
		fmt.Sprintf("order line has %d tokens; need %d to %d", n, MinOrderTokens, MaxOrderTokens))}
}

// orderTranslationIssues reuses the English and length checks that
// translationFieldIssues uses, but always as a block, and also blocks a
// missing translation. INIT-029/SPEC-002.
func orderTranslationIssues(pointer, text string) []Issue {
	if strings.TrimSpace(text) == "" {
		return []Issue{issue(RuleUnitOrderTranslation, SeverityBlock, pointer,
			"order line needs an English translation")}
	}
	if translationNotEnglish(text) {
		return []Issue{issue(RuleUnitOrderTranslation, SeverityBlock, pointer,
			"order translation is English plain text, with no Hebrew and no markup")}
	}
	if utf8.RuneCountInString(text) > TranslationMaxLen {
		return []Issue{issue(RuleUnitOrderTranslation, SeverityBlock, pointer,
			fmt.Sprintf("order translation is longer than %d characters", TranslationMaxLen))}
	}
	return nil
}

func orderAcceptIssues(actPointer, text string, accept []string) []Issue {
	want := tokenize(text)
	var out []Issue
	for i, entry := range accept {
		ptr := actPointer + "/accept/" + fmt.Sprint(i)
		got := tokenize(entry)
		if slices.Equal(want, got) {
			out = append(out, issue(RuleUnitOrderAccept, SeverityBlock, ptr,
				"accept entry is the same order as text; list a different order, or omit it"))
			continue
		}
		if !sameTokenBag(want, got) {
			out = append(out, issue(RuleUnitOrderAccept, SeverityBlock, ptr,
				"accept entry must use exactly the same tokens as text, in a different order"))
		}
	}
	return out
}

// sameTokenBag reports whether a and b hold the same tokens with the same
// counts, ignoring order. Tokens are already NFC from tokenize.
func sameTokenBag(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	left := append([]string(nil), a...)
	right := append([]string(nil), b...)
	slices.Sort(left)
	slices.Sort(right)
	return slices.Equal(left, right)
}
