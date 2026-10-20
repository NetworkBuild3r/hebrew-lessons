package views

import "strconv"

// Onboarding copy. One string, one slot. Apostrophes are U+2019.
// The welcome meta row is Listening and Reading only. DESIGN.md §8.6's
// "about 40–60 items, roughly 10–12 minutes" does not match the live test:
// 6 letters, then 6 or 12 items per band per skill across 5 bands, then up
// to 8 typing items. A clear run is already about 74 items. The time item
// is left out rather than shown as a fixed estimate.
const (
	WelcomeTitle = "Welcome to Derekh Ivrit"
	WelcomeLede  = "Let’s find the right place for you to start."

	ChoiceFindTitle = "Find your level"
	ChoiceFindBody  = "A short check that skips what you already know."
	ChoiceFindCTA   = "Start placement"

	ChoiceBasicsTitle = "Start from the basics"
	ChoiceBasicsBody  = "Begin with the alef-bet and your first words."

	WelcomeNote = "You can take placement any time from Progress."

	PlacementTitle = "Placement"
	PlacedNone     = "Not placed yet"
	LinkRetake     = "Retake placement"
	LinkTake       = "Take placement"
	RetakeKeep     = "Retaking won’t erase anything you’ve learned."
	PastResults    = "Past results"
	LatestTag      = "Latest"
	NoResultsYet   = "No results yet."
	ResultLede     = "We’ll review anything you missed along the way."
	ResultBasics   = "Start from the basics instead"
	ResultStart    = "Start"
	KnewNothingH   = "Let’s start from the basics"
	SkipLabel      = "I don’t know"
	SkipHint       = "It’s fine to skip. Guessing makes your level less accurate."
	PathPast       = "Placed past. Its words come back in review."
	PathStart      = "You start here"
	PathNext       = "Next"
)

// WelcomeMeta is the primary card's meta row. No time estimate: see the
// comment on the constants above.
var WelcomeMeta = []string{"Listening", "Reading"}

// UnitLabel is "Unit {n}: {title}", or "Unit {n}" when title is empty.
func UnitLabel(n int, title string) string {
	label := "Unit " + strconv.Itoa(n)
	if title == "" {
		return label
	}
	return label + ": " + title
}

// PlacedOn is the muted line under the unit, "Placed {Mon d}".
func PlacedOn(when string) string {
	return "Placed " + when
}

// WordsKnownLabel is the history column "{n} words".
func WordsKnownLabel(n int) string {
	return strconv.Itoa(n) + " words"
}

// ShowAllLabel is "Show all ({n})".
func ShowAllLabel(n int) string {
	return "Show all (" + strconv.Itoa(n) + ")"
}

// ResultHeadline is the results H1. An empty title omits ": {name}".
func ResultHeadline(n int, title string) string {
	return "You’re starting at " + UnitLabel(n, title)
}
