package views

import "strconv"

// SourceLabel is the short line that says where a card's word came from.
func SourceLabel(source string) string {
	switch source {
	case "course":
		return "Common words"
	case "tutor":
		return "From your tutor"
	default:
		return "From an earlier lesson"
	}
}

// TutorCountLine is the one line under a class title.
func TutorCountLine(lines, inCourse, newWords int) string {
	line := strconv.Itoa(lines) + " lines. "
	if inCourse > 0 {
		line += strconv.Itoa(inCourse) + " words you already have"
	} else {
		line += "No words you already have"
	}
	if newWords > 0 {
		line += ", " + strconv.Itoa(newWords) + " new from this class."
	} else {
		line += "."
	}
	return line
}
