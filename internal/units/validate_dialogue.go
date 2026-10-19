package units

import (
	"fmt"
	"strconv"
	"strings"
)

// Dialogue (INIT-015/SPEC-026).
//
// Every text rule in this package reads one string per activity. A dialogue has
// one string per line, and each line has to be judged on its own: a long line
// the learner knows must not hide a line they do not, and the per-text length
// limit is per line. So the validator does not teach every rule about lines.
// It flattens the document first: each dialogue becomes one activity per line,
// typed "dialogue", carrying that line's text and speaker. The text rules run
// on the flat copy unchanged. remapIssues then turns a pointer into the flat
// copy back into a pointer into the real document, so a client sees
// /lessons/0/activities/2/lines/1/text and never the flat index.
//
// The shape of the dialogue itself (line count, speakers, empty text) is
// checked on the real document by dialogueIssues and schemaIssues.

// flatRef says where a flat activity came from: activity act of the lesson, and
// line of that activity, or -1 when the activity is not a dialogue.
type flatRef struct{ act, line int }

// flatten returns doc with every dialogue replaced by one activity per line.
// refs[i][j] is the origin of flat activity j of lesson i. A document with no
// dialogue is returned as it is, with nil refs, and remapIssues then does nothing.
func flatten(doc *Unit) (*Unit, [][]flatRef) {
	has := false
	for _, les := range doc.Lessons {
		for _, act := range les.Activities {
			if act.Type == ActivityDialogue {
				has = true
			}
		}
	}
	if !has {
		return doc, nil
	}
	flat := *doc
	flat.Lessons = make([]Lesson, len(doc.Lessons))
	refs := make([][]flatRef, len(doc.Lessons))
	for i, les := range doc.Lessons {
		out := les
		out.Activities = nil
		for j, act := range les.Activities {
			if act.Type != ActivityDialogue {
				out.Activities = append(out.Activities, act)
				refs[i] = append(refs[i], flatRef{act: j, line: -1})
				continue
			}
			for k, line := range act.Lines {
				out.Activities = append(out.Activities, Activity{Type: ActivityDialogue, Text: line.Text, speaker: line.Speaker})
				refs[i] = append(refs[i], flatRef{act: j, line: k})
			}
		}
		flat.Lessons[i] = out
	}
	return &flat, refs
}

// remapIssues rewrites the pointers of issues raised on a flat copy. refs is
// what flatten returned. A pointer that is not into /lessons/i/activities/j is
// left alone. The input slice is changed in place and returned.
func remapIssues(issues []Issue, refs [][]flatRef) []Issue {
	if refs == nil {
		return issues
	}
	for i := range issues {
		issues[i].Pointer = remapPointer(issues[i].Pointer, refs)
	}
	return issues
}

func remapPointer(p string, refs [][]flatRef) string {
	parts := strings.Split(p, "/")
	// "", "lessons", i, "activities", j, rest...
	if len(parts) < 5 || parts[0] != "" || parts[1] != "lessons" || parts[3] != "activities" {
		return p
	}
	li, err := strconv.Atoi(parts[2])
	if err != nil || li < 0 || li >= len(refs) {
		return p
	}
	fj, err := strconv.Atoi(parts[4])
	if err != nil || fj < 0 || fj >= len(refs[li]) {
		return p
	}
	ref := refs[li][fj]
	out := "/lessons/" + parts[2] + "/activities/" + strconv.Itoa(ref.act)
	if ref.line >= 0 {
		out += "/lines/" + strconv.Itoa(ref.line)
	}
	if len(parts) > 5 {
		out += "/" + strings.Join(parts[5:], "/")
	}
	return out
}

// storyLike reports an activity type whose Hebrew is running text: it must be
// pointed and every content word must map to the lexicon.
func storyLike(typ string) bool {
	return typ == ActivityStory || typ == ActivityDialogue
}

// dialogueIssues applies unit.dialogue_lines and unit.dialogue_speakers to the
// real document.
func dialogueIssues(doc *Unit) []Issue {
	var out []Issue
	for i, les := range doc.Lessons {
		for j, act := range les.Activities {
			base := at("lessons", i, "activities", j)
			if act.Type != ActivityDialogue {
				if len(act.Lines) > 0 {
					out = append(out, issue(RuleUnitDialogueLines, SeverityBlock, base+"/lines",
						fmt.Sprintf("lines belong to a dialogue activity, and this one is %q", act.Type)))
				}
				continue
			}
			if strings.TrimSpace(act.Text) != "" {
				out = append(out, issue(RuleUnitDialogueLines, SeverityBlock, base+"/text",
					"a dialogue has lines, not text; move the text into lines"))
			}
			n := len(act.Lines)
			if n < MinDialogueLines || n > MaxDialogueLines {
				out = append(out, issue(RuleUnitDialogueLines, SeverityBlock, base+"/lines",
					fmt.Sprintf("a dialogue has %d lines; it needs %d to %d", n, MinDialogueLines, MaxDialogueLines)))
			}
			spoke := map[string]bool{}
			for k, line := range act.Lines {
				if !ValidSpeaker(line.Speaker) {
					out = append(out, issue(RuleUnitDialogueSpeakers, SeverityBlock, base+"/lines/"+strconv.Itoa(k)+"/speaker",
						fmt.Sprintf("speaker %q is not a (female voice) or b (male voice)", line.Speaker)))
					continue
				}
				spoke[line.Speaker] = true
			}
			// One line cannot have two speakers, so unit.dialogue_lines already
			// says what is wrong. Past that, both voices must be heard.
			if n >= MinDialogueLines {
				for _, sp := range []string{SpeakerA, SpeakerB} {
					if !spoke[sp] {
						out = append(out, issue(RuleUnitDialogueSpeakers, SeverityBlock, base+"/lines",
							fmt.Sprintf("speaker %s has no line; a dialogue needs both a and b", sp)))
					}
				}
			}
		}
	}
	return out
}
