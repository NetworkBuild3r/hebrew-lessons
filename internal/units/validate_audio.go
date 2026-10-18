package units

import "strings"

func audioIssues(s *scan, doc *Unit) []Issue {
	var out []Issue
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			if strings.TrimSpace(act.Text) == "" || !containsHebrew(act.Text) {
				continue
			}
			if s.hasAudio(act.Text) {
				continue
			}
			if s.err != nil {
				return out
			}
			out = append(out, issue(RuleAudioMissing, SeverityBlock, at("lessons", i, "activities", j, "text"),
				"Hebrew text has no audio clip"))
		}
	}
	return out
}
