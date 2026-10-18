package units

import (
	"fmt"
	"strings"
)

func budgetIssues(doc *Unit, words []wordRef) []Issue {
	var out []Issue
	if len(doc.NewWords) > MaxNewWordsPerUnit {
		for i := MaxNewWordsPerUnit; i < len(doc.NewWords); i++ {
			out = append(out, issue(RuleBudgetUnit, SeverityBlock, at("newWords", i),
				fmt.Sprintf("new word %d is past the maximum of %d new words in a unit", i+1, MaxNewWordsPerUnit)))
		}
	}
	counts := map[int]int{}
	for _, w := range words {
		if w.dup || w.norm == "" || !w.found {
			continue
		}
		counts[w.lesson]++
	}
	for i, les := range doc.Lessons {
		kind := strings.TrimSpace(les.Kind)
		n := counts[i]
		switch kind {
		case KindCheckpoint:
			if n > 0 {
				out = append(out, issue(RuleSchemaCheckpoint, SeverityBlock, at("lessons", i),
					fmt.Sprintf("checkpoint introduces %d new word(s); a checkpoint introduces nothing", n)))
			}
		case KindLesson:
			if n < MinNewWordsPerLesson || n > MaxNewWordsPerLesson {
				out = append(out, issue(RuleBudgetLesson, SeverityBlock, at("lessons", i),
					fmt.Sprintf("lesson introduces %d new words; %d to %d are allowed", n, MinNewWordsPerLesson, MaxNewWordsPerLesson)))
			}
		}
	}
	return out
}
