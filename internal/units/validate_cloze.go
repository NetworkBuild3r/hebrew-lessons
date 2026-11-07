package units

import (
	"encoding/json"
	"fmt"
	"strings"
)

// clozeIssues checks that a cloze can only have one right answer. The gap
// needs at least one other word on the line, and the line needs its English,
// which the player shows as the cue. Without both, every word of the same
// part of speech fits the gap. ModeWrite blocks. ModePublished warns, so the
// current published revision keeps serving until it is rewritten.
func clozeIssues(doc *Unit, mode Mode) []Issue {
	if doc == nil {
		return nil
	}
	sev := SeverityBlock
	if mode == ModePublished {
		sev = SeverityWarn
	}
	var out []Issue
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			if act.Type != ActivityCloze {
				continue
			}
			ptr := at("lessons", i, "activities", j)
			if n := len(tokenize(act.Text)); n < MinClozeTokens {
				out = append(out, issue(RuleUnitClozeContext, sev, ptr+"/text",
					fmt.Sprintf("activity %d: this cloze line has %d of the %d words a gap needs, so nothing on the line points to the answer. Use produce for a single word.", j+1, n, MinClozeTokens)))
			}
			if strings.TrimSpace(act.Translation) == "" {
				out = append(out, issue(RuleUnitClozeTranslation, sev, ptr+"/translation",
					fmt.Sprintf("activity %d: this cloze has no translation; set it to the English of the whole line. The learner sees it as the cue, so only one word fits the gap.", j+1)))
			}
		}
	}
	return out
}

// lessonRepeatIssues flags a lesson whose activities are the same, in the
// same order, as an earlier lesson of the unit. A checkpoint reviews what the
// lessons taught and is not compared. An empty lesson is left to the schema
// rules.
func lessonRepeatIssues(doc *Unit, mode Mode) []Issue {
	if doc == nil {
		return nil
	}
	sev := SeverityBlock
	if mode == ModePublished {
		sev = SeverityWarn
	}
	seen := map[string]int{}
	var out []Issue
	for i, les := range doc.Lessons {
		if les.Kind != KindLesson || len(les.Activities) == 0 {
			continue
		}
		raw, err := json.Marshal(les.Activities)
		if err != nil {
			continue
		}
		key := string(raw)
		if first, ok := seen[key]; ok {
			out = append(out, issue(RuleUnitLessonRepeat, sev, at("lessons", i),
				fmt.Sprintf("lesson %d repeats lesson %d step for step; write new lines that use the words in a different way.", i+1, first+1)))
			continue
		}
		seen[key] = i
	}
	return out
}
