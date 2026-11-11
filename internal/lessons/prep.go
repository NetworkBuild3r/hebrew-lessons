package lessons

import "strings"

// SpeakingPrep is the copyable page for a speaking session outside the app.
// It lists this week's words and the tutor's sentences. It is not a lesson.
func SpeakingPrep(classOn string, words, tutorLines []string) string {
	var b strings.Builder
	b.WriteString("This week's words")
	if classOn != "" {
		b.WriteString(" (")
		b.WriteString(classOn)
		b.WriteString(")")
	}
	b.WriteString("\n")
	if len(words) == 0 {
		b.WriteString("(none)\n")
	}
	for _, w := range words {
		b.WriteString(w)
		b.WriteString("\n")
	}
	b.WriteString("\nTutor lines\n")
	if len(tutorLines) == 0 {
		b.WriteString("(none)\n")
	}
	for _, line := range tutorLines {
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
