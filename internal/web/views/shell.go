package views

import "strconv"

// Shell helpers shared by the destinations SPEC-017 added: Lessons, Me, and the report page.

// ThemeGroupLabel names the Light, Dark, Auto group on Me.
const ThemeGroupLabel = "Theme"

// navCurrent reports whether the dock item id is the current page. The lesson player is
// part of Lessons, so Lessons is current there too.
func navCurrent(current, id string) bool {
	if current == id {
		return true
	}
	return id == "lessons" && current == "lesson"
}

// ReportPath is where a lesson's report form posts and where its no-script page lives.
// id is the unit id.
func ReportPath(id int64) string { return "/lessons/" + strconv.FormatInt(id, 10) + "/report" }

// StepsDone is "2 of 5 lessons done".
func StepsDone(done, total int) string {
	return strconv.Itoa(done) + " of " + strconv.Itoa(total) + " done"
}

// MeView is the Me page: who is signed in, the theme, the language, and log out.
type MeView struct {
	Page
	// Name is the signed-in learner's email, the only name the app has.
	Name string
}

// LessonRow is one unit on the Lessons list. State is done, next, or later.
type LessonRow struct {
	Title   string // Pointed Hebrew.
	Gloss   string // "(translit, meaning)", or empty.
	Mine    bool   // Built for this learner.
	State   string
	Lessons int
	Done    int
	Href    string // Where "next" opens. Empty for the rest.
}

// LessonsView is the Lessons list. Rows are the learner's published units in order, with
// any unit they reported already left out.
type LessonsView struct {
	Page
	Rows []LessonRow
}

// ReportView is the no-script page of the report form.
type ReportView struct {
	Page
	UnitID int64
	Title  string
	// Reason and Note are what the learner had typed when the post was refused, so the
	// form keeps them. Error is the plain-language reason it was refused.
	Reason string
	Note   string
	Error  string
}
