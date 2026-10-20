package units

import (
	"fmt"
	"strings"
)

// audioIssues runs on the flattened document. An activity that came from a
// dialogue line carries its speaker, and its clip is looked up in that
// speaker's voice (SpeakerLookup). A line with an unknown speaker is skipped
// here, because unit.dialogue_speakers already blocks it.
func audioIssues(s *scan, doc *Unit) []Issue {
	var out []Issue
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			if strings.TrimSpace(act.Text) == "" || !containsHebrew(act.Text) {
				continue
			}
			msg := "Hebrew text has no audio clip"
			var has bool
			if act.Type == ActivityDialogue {
				if !ValidSpeaker(act.speaker) {
					continue
				}
				has = s.hasAudioFor(act.Text, act.speaker)
				msg = fmt.Sprintf("Hebrew text has no audio clip in the voice of speaker %s", act.speaker)
			} else {
				has = s.hasAudio(act.Text)
			}
			if has {
				continue
			}
			if s.err != nil {
				return out
			}
			out = append(out, issue(RuleAudioMissing, SeverityBlock, at("lessons", i, "activities", j, "text"), msg))
		}
	}
	return out
}
