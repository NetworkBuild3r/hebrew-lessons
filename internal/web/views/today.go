package views

import (
	"strconv"
	"strings"
)

// TodayAction is the one next action Today offers. Exactly one gold button renders for
// every action except ActionNone. The order the shell picks them in is documented on
// web.ChooseTodayAction.
type TodayAction string

const (
	ActionResumePlacement TodayAction = "resume_placement"
	ActionStartPlacement  TodayAction = "start_placement"
	ActionContinueLesson  TodayAction = "continue_lesson"
	ActionCheckin         TodayAction = "checkin"
	ActionReviews         TodayAction = "reviews"
	ActionNextLesson      TodayAction = "next_lesson"
	// ActionNone is a learner with nothing to do: no gold button, and the cards say what to do next.
	ActionNone TodayAction = "none"
)

// TodayActions lists every action in the order the shell prefers them.
// Placement is not among them.
var TodayActions = []TodayAction{
	ActionContinueLesson, ActionCheckin, ActionReviews, ActionNextLesson, ActionNone,
}

// WatchWord is one row of "Words to watch": a struggling word and the skill that fails.
type WatchWord struct {
	Text  string // Pointed Hebrew.
	Gloss string // English meaning.
	Skill string // listen, read, recall or spell.
}

// SlipAlert is a band-slip event still inside its seven days. Level is the band as the
// learner sees it (the placement level, band plus one). Words is how many of that
// band's words are struggling or regressed now, which is the size of the review set.
type SlipAlert struct {
	Level int
	Band  int // The band the job wrote, 0 to 4. The review set is built from it.
	Words int
	Day   string
}

// TodayView is what the shell adds to Home for one visit.
type TodayView struct {
	Action TodayAction
	// Step and Steps describe a lesson in progress: Step is the activity the learner is on, 1-based.
	Step, Steps int
	// CheckinSkill is the skill the check-in asks (listen or read). CheckinLate is true
	// from 42 days after the last placement or check-in. CheckinWeeks is how long it has been.
	CheckinSkill string
	CheckinLate  bool
	CheckinWeeks int
	// DailyLine is "12 reviews due · about 6 minutes". Empty when nothing is due.
	DailyLine string
	// Watch lists up to five struggling words, worst first.
	Watch []WatchWord
	// Slips lists the band-slip alerts to show, newest first.
	Slips []SlipAlert
	// Reported is true on the visit right after a report, so Today can say the lesson is hidden.
	Reported bool
}

// CheckinHint is the line under the check-in title: how long it has been, and what the
// short sitting does.
func CheckinHint(skill string, weeks int) string {
	what := "listening"
	if skill == "read" {
		what = "reading"
	}
	since := "It has been a while since your last check."
	if weeks >= 1 {
		since = "It has been " + CountText(weeks, "week", "weeks") + " since your last check."
	}
	return since + " Three minutes of " + what + " finds the words you have gained, and any that are slipping."
}

// SkillLabel is the learner-facing name of a skill.
func SkillLabel(skill string) string {
	switch skill {
	case "listen":
		return "Listening"
	case "read":
		return "Reading"
	case "recall":
		return "Recall"
	case "spell":
		return "Spelling"
	}
	return skill
}

// CountText is "1 review" or "12 reviews".
func CountText(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// MinutesFor is the time a day of reviews takes: half a minute a review, at least one
// minute. A visit asks at most sessionCap reviews, so more than that does not add time.
func MinutesFor(due, sessionCap int) int {
	if due <= 0 {
		return 0
	}
	n := min(due, sessionCap)
	m := (n + 1) / 2
	return max(m, 1)
}

// DailyLineFor is the streak-free daily line: how many reviews are due and about how long
// they take. It says nothing when none are due.
func DailyLineFor(due, sessionCap int) string {
	if due <= 0 {
		return ""
	}
	m := MinutesFor(due, sessionCap)
	unit := "minutes"
	if m == 1 {
		unit = "minute"
	}
	return CountText(due, "review", "reviews") + " due · about " + strconv.Itoa(m) + " " + unit
}

// Initials is up to two capital letters of a name or an email, for the Me page.
func Initials(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if at := strings.IndexByte(name, '@'); at > 0 {
		name = name[:at]
	}
	var out []rune
	for _, part := range strings.FieldsFunc(name, func(r rune) bool { return r == ' ' || r == '.' || r == '_' || r == '-' }) {
		for _, r := range part {
			out = append(out, []rune(strings.ToUpper(string(r)))[0])
			break
		}
		if len(out) == 2 {
			break
		}
	}
	return string(out)
}
