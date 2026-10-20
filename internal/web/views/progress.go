package views

import (
	"strconv"
	"strings"
	"time"
)

// The Progress page (INIT-015/SPEC-018). web/progress.go fills these from the learner model,
// the daily snapshots, the regression events, the placement runs, and the weekly reports, and
// progress.templ draws them. Nothing here reads a database.

// SkillTab is one of the four skill choices.
type SkillTab struct {
	Skill   string
	Label   string
	Current bool
}

// LegendItem is one state of the bar: its name, its count, and its share of the lexicon, as text.
type LegendItem struct {
	State string // unseen, seen, struggling, regressed, learning, known, solid
	Label string
	Count int
	Share string // "12%"
}

// ChartTable is the table that carries the same values as a chart.
type ChartTable struct {
	Caption string
	Head    []string
	Rows    [][]string
}

// ChartView is one chart with its words. SVG is markup built in web/chart.go: every dynamic
// value in it is escaped there, and it names no colour. When SVG is empty the chart has nothing
// to draw, and Hint says what to do.
type ChartView struct {
	ID      string
	Title   string
	Note    string
	SVG     string
	Table   ChartTable
	Summary string
	Hint    string
	// Legend is the key under a chart: a line of text per mark.
	Key []string
}

// WeekSkill is one skill's row in "This week". Gained and Lost are the net move in words known
// (known plus solid) between the first and the last snapshot of the week. Struggling is how many
// words were struggling or regressed at the start and at the end.
type WeekSkill struct {
	Skill         string
	Label         string
	Gained        int
	Lost          int
	StruggleStart int
	StruggleEnd   int
}

// WeekWord is a word that slipped, or came back, with the skill and the likely cause.
type WeekWord struct {
	Text       string // pointed Hebrew
	Gloss      string
	Skill      string
	SkillLabel string
	Cause      string // the code: confusion, decoding, ...
	CauseLabel string
	Day        string
}

// WeekView is the "This week" card.
type WeekView struct {
	// Compared is true when the week has two snapshot days to compare. From and To are those days.
	Compared bool
	From, To string
	Skills   []WeekSkill
	// Slipped are words that went from known or solid to struggling or regressed this week.
	Slipped []WeekWord
	// Bands are the levels that slipped this week, as the learner sees them (band plus one).
	Bands []int
	// Back are words that slipped in the last four weeks and are not struggling now.
	Back []WeekWord
}

// PlacementRow is one finished placement, newest first.
// The three columns are the date, "Unit {n}", and "{n} words".
// The unit title is on the card's status line, not in the row.
type PlacementRow struct {
	Date   string // "Oct 19", or "Jan 2, 2006" when the year is not this year
	Unit   string // "Unit 3"
	Words  string // "42 words"
	Latest bool
}

// WeeklyCard is the newest weekly report.
type WeeklyCard struct {
	WeekOf  string
	Summary string
}

// ProgressView is the whole page.
type ProgressView struct {
	Page
	// Empty is true when the learner has no snapshot and has seen no word: nothing to chart.
	Empty bool
	// UnitsText is "1/2" lessons finished, or "" when none are published.
	UnitsText string
	Weekly    *WeeklyCard
	Tabs      []SkillTab
	Skill     string
	SkillName string
	// Known is the estimated words known in the chosen skill now, as text.
	Known     string
	Legend    []LegendItem
	BarSVG    string
	BarTable  ChartTable
	Total     int
	Est       ChartView
	Retention ChartView
	Week      WeekView
	Placement []PlacementRow
	// Placed is true when the learner has at least one finished run.
	Placed bool
	// PlacementStatus is "Unit {n}: {title}", "Unit {n}", or "Not placed yet".
	PlacementStatus string
	// PlacementWhen is the muted "Placed {Mon d}" under the unit. Empty when
	// the learner has not been placed.
	PlacementWhen string
	// PlacementLink is "Retake placement" or "Take placement".
	PlacementLink string
	// PlacementKeep is the reassurance under Retake. Empty unless Placed.
	PlacementKeep string
	// PlacementTotal is the finished-run count. PlacementMore is "Show all ({n})"
	// when the page is showing a short list and the count is over 3.
	PlacementTotal    int
	PlacementMore     string
	PlacementMoreHref string
	// PlacementAll is the full history page: no "Show all" link.
	PlacementAll bool
}

// SignedGain is "+9" for a gain, and "0" for none.
func SignedGain(n int) string {
	if n <= 0 {
		return "0"
	}
	return "+" + strconv.Itoa(n)
}

// SignedLoss is "−2" for a loss (with a real minus sign), and "0" for none.
func SignedLoss(n int) string {
	if n <= 0 {
		return "0"
	}
	return "\u2212" + strconv.Itoa(n)
}

// shortDayOf is "Oct 18" for 2026-10-18, or the text unchanged when it is not a day.
func shortDayOf(day string) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return t.Format("Jan 2")
}

// CauseLabel is the plain words for a regression cause, or for a word's diagnosed cause.
func CauseLabel(cause string) string {
	switch {
	case cause == "confusion":
		return "mixed up with another word"
	case cause == "decoding":
		return "hears it, cannot read it yet"
	case cause == "production":
		return "reads it, cannot recall it yet"
	case cause == "spelling":
		return "recalls it, cannot spell it yet"
	case cause == "band_slip", strings.HasPrefix(cause, "band_slip"):
		return "its whole level is slipping"
	case cause == "unexplained", cause == "":
		return "no clear cause yet"
	}
	return "no clear cause yet"
}

// StateLabel is the learner-facing name of a word state.
func StateLabel(state string) string {
	switch state {
	case "unseen":
		return "Not seen yet"
	case "seen":
		return "Seen"
	case "struggling":
		return "Struggling"
	case "regressed":
		return "Slipping"
	case "learning":
		return "Learning"
	case "known":
		return "Known"
	case "solid":
		return "Solid"
	}
	return state
}
