package views

import "strconv"

// LessonFrame is what the player shows around a step: a progress bar and a back link on
// top, the step heading in the top third, an outline of the lesson's steps on desktop, and
// the "Report a problem" sheet. It carries labels and counts only, never an answer.
type LessonFrame struct {
	UnitID int64
	// Step is the step the learner is on, 1-based. Total is how many steps the lesson has.
	// Done is true on the completion panel, where Step equals Total.
	Step, Total int
	Done        bool
	// Value is the progress bar fill: steps finished. It is Step minus one while a step is
	// being asked, Step once its answer is showing, and Total when the lesson is done.
	Value int
	// Kind is the learner's name for the step ("Listen and pick").
	Kind string
	// Outline is one entry per step.
	Outline []OutlineStep
	// BackHref is Today. ReportHref is the no-script page of the report form, and
	// ReportAction is where the form posts.
	BackHref     string
	ReportHref   string
	ReportAction string
	CSRF         string
}

// OutlineStep is one line of the lesson outline. State is done, current, or todo.
type OutlineStep struct {
	Label string
	State string
}

// ReportReason is one choice of the report sheet. Code is what the server stores.
type ReportReason struct {
	Code  string
	Label string
}

// ReportReasons are the five reasons the sheet offers, in order. Codes are stored in
// unit_reports.reason.
var ReportReasons = []ReportReason{
	{"wrong_hebrew", "Wrong Hebrew"},
	{"wrong_translation", "Wrong translation"},
	{"broken_audio", "Broken audio"},
	{"too_hard", "Too hard"},
	{"other", "Other"},
}

// ReportNoteMax is the longest note the sheet accepts, in characters.
const ReportNoteMax = 500

// ReasonLabel is the label of a stored reason code, or the code itself when it is unknown.
func ReasonLabel(code string) string {
	for _, r := range ReportReasons {
		if r.Code == code {
			return r.Label
		}
	}
	return code
}

// ValidReason reports whether code is one of the sheet's five.
func ValidReason(code string) bool {
	for _, r := range ReportReasons {
		if r.Code == code {
			return true
		}
	}
	return false
}

// StepText is "Step 3 of 8".
func StepText(step, total int) string {
	return "Step " + strconv.Itoa(step) + " of " + strconv.Itoa(total)
}

// LessonLabel is the learner's name for an activity type. An unknown type is shown as
// "Practice", so a new type never leaks its internal name.
func LessonLabel(typ string) string {
	switch typ {
	case "story":
		return "Read the story"
	case "pattern":
		return "See the pattern"
	case "shadow":
		return "Listen and say"
	case "recall":
		return "Recall the meaning"
	case "listen_pick":
		return "Listen and pick"
	case "produce":
		return "Pick the Hebrew"
	case "cloze":
		return "Fill the gap"
	case "say":
		return "Say it aloud"
	}
	return "Practice"
}
