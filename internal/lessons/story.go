package lessons

import "strings"

// StoryFailure is one reason a story cannot be saved.
type StoryFailure struct {
	ID   string
	Word string
}

// CheckStory is the one check an agent must pass before a story is stored.
// known is the taught set plus this lesson's new words. mustUse are due words
// the story was told to include. earlier are stories already stored, so a
// repeat fails. The story is never a graded key.
func CheckStory(text string, known map[string]bool, mustUse, earlier []string) []StoryFailure {
	var out []StoryFailure
	text = strings.TrimSpace(text)
	if text == "" {
		return []StoryFailure{{ID: "story.empty"}}
	}
	if len([]rune(text)) > 400 {
		out = append(out, StoryFailure{ID: "brief.text_too_long"})
	}
	if strings.Contains(text, "<") || strings.Contains(text, ">") {
		out = append(out, StoryFailure{ID: "unit.no_markup"})
	}
	for _, prev := range earlier {
		if strings.TrimSpace(prev) == text {
			out = append(out, StoryFailure{ID: "unit.story_repeat"})
		}
	}
	toks := tokens(text)
	if len(toks) == 0 {
		out = append(out, StoryFailure{ID: "story.no_hebrew"})
	} else if !hasPoints(text) {
		out = append(out, StoryFailure{ID: "story.unpointed"})
	}
	if coverage(text, known) < 0.90 {
		out = append(out, StoryFailure{ID: "story.coverage"})
	}
	have := map[string]bool{}
	for _, t := range toks {
		have[t] = true
		if len(known) > 0 && !known[t] {
			out = append(out, StoryFailure{ID: "story.unmapped", Word: t})
		}
	}
	for _, w := range mustUse {
		if !have[w] {
			out = append(out, StoryFailure{ID: "story.missing_word", Word: w})
		}
	}
	return out
}
