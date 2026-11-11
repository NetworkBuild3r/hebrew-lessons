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
	// Eyebrow and Heading are what the Figma lesson screens print above the step: a small
	// caps line and a 22px title. When they are empty the frame falls back to "Step 3 of 8"
	// and Kind. Neither ever holds an answer.
	Eyebrow, Heading string
	// NoStages hides the See, Check, Read, Write, Use row. An easy lesson has no new words,
	// so the row would promise a lesson shape it does not have. Figma Easy lesson (255:231).
	NoStages bool
	// Stage is the Figma stage for this step: See, Check, Read, Write, or Use.
	// Empty on the completion panel, where no stage is current.
	Stage string
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

// LessonStages is the stage row on a lesson, in the order the Figma board shows.
var LessonStages = []string{"See", "Check", "Read", "Write", "Use"}

// LessonStageChip is one pill in that row. State is "now" or "idle".
type LessonStageChip struct {
	Label string
	State string
}

// LessonStage maps an activity type onto the Figma stage row.
// See is the words, Check is a meaning question, Read is a line to read,
// Write is making the Hebrew, and Use is a sentence that is shown.
func LessonStage(typ string) string {
	switch typ {
	case "pattern", "say":
		return "See"
	case "recall", "listen_pick":
		return "Check"
	case "story", "dialogue", "shadow":
		return "Read"
	case "produce", "cloze":
		return "Write"
	case "order":
		return "Use"
	default:
		return ""
	}
}

// LessonStageChips marks the current stage. An empty current leaves every pill idle.
func LessonStageChips(current string) []LessonStageChip {
	out := make([]LessonStageChip, len(LessonStages))
	for i, label := range LessonStages {
		state := "idle"
		if label == current {
			state = "now"
		}
		out[i] = LessonStageChip{Label: label, State: state}
	}
	return out
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
	case "order":
		return "Put the words in order" // INIT-029/SPEC-004
	}
	return "Practice"
}
