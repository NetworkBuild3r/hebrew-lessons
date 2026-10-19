package units

import (
	"fmt"
	"strings"
)

func schemaIssues(doc *Unit) []Issue {
	var out []Issue
	if strings.TrimSpace(doc.Title) == "" {
		out = append(out, issue(RuleSchemaRequired, SeverityBlock, at("title"), "title is required"))
	}
	if strings.TrimSpace(doc.Situation) == "" {
		out = append(out, issue(RuleSchemaRequired, SeverityBlock, at("situation"), "situation is required"))
	}
	if strings.TrimSpace(doc.Concept) == "" {
		out = append(out, issue(RuleSchemaRequired, SeverityBlock, at("concept"), "concept is required"))
	}
	if len(doc.Lessons) == 0 {
		out = append(out, issue(RuleSchemaRequired, SeverityBlock, at("lessons"), "lessons are required"))
	}
	for i, raw := range doc.NewWords {
		if strings.TrimSpace(raw) == "" {
			out = append(out, issue(RuleSchemaRequired, SeverityBlock, at("newWords", i), "new word is empty"))
		}
	}
	for i, les := range doc.Lessons {
		kind := strings.TrimSpace(les.Kind)
		switch kind {
		case "":
			out = append(out, issue(RuleSchemaRequired, SeverityBlock, at("lessons", i, "kind"), "lesson kind is required"))
		case KindLesson, KindCheckpoint:
		default:
			out = append(out, issue(RuleSchemaKind, SeverityBlock, at("lessons", i, "kind"), fmt.Sprintf("lesson kind %q is not lesson or checkpoint", les.Kind)))
		}
		if len(les.Activities) == 0 {
			out = append(out, issue(RuleSchemaRequired, SeverityBlock, at("lessons", i, "activities"), "lesson activities are required"))
		}
		for j, act := range les.Activities {
			typ := strings.TrimSpace(act.Type)
			switch {
			case typ == "":
				out = append(out, issue(RuleSchemaRequired, SeverityBlock, at("lessons", i, "activities", j, "type"), "activity type is required"))
			case !knownActivity(typ):
				out = append(out, issue(RuleSchemaActivity, SeverityBlock, at("lessons", i, "activities", j, "type"), fmt.Sprintf("unknown activity type %q", act.Type)))
			}
			if typ == ActivityDialogue {
				// A dialogue has lines, not text. dialogueIssues checks the line
				// count and the speakers; each line still needs its text.
				for k, line := range act.Lines {
					if strings.TrimSpace(line.Text) == "" {
						out = append(out, issue(RuleSchemaRequired, SeverityBlock, at("lessons", i, "activities", j, "lines", k, "text"), "dialogue line text is required"))
					}
				}
				continue
			}
			if strings.TrimSpace(act.Text) == "" {
				out = append(out, issue(RuleSchemaRequired, SeverityBlock, at("lessons", i, "activities", j, "text"), "activity text is required"))
			}
		}
	}
	return out
}

func refIssues(words []wordRef) []Issue {
	var out []Issue
	for _, w := range words {
		if w.norm == "" {
			continue
		}
		if w.dup {
			out = append(out, issue(RuleSchemaRef, SeverityBlock, at("newWords", w.index), fmt.Sprintf("duplicate new word %s", strconvQuote(w.raw))))
			continue
		}
		if !w.found {
			out = append(out, issue(RuleSchemaRef, SeverityBlock, at("newWords", w.index), fmt.Sprintf("new word %s never appears in a lesson", strconvQuote(w.raw))))
		}
	}
	return out
}

func strconvQuote(s string) string {
	return quoteList([]string{s})
}
