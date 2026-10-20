package views

import (
	"strconv"

	"github.com/a-h/templ"
)

// The placement screens (INIT-015/SPEC-012). The v3 engine (internal/placement) deals one item at a
// time, and a screen shows one item. Every screen has one thing to do:
//
//   - intro, resume, break, offer: one primary Button;
//   - letters, listening, reading: pick an option (the answers are the action; listening and letters
//     also have one gold audio orb, and "I don't know" is a quiet button);
//   - typing: the Hebrew keyboard, and one primary Check;
//   - results and review: one primary Button.
//
// Placement gives no right or wrong feedback (D1): nothing here knows whether an answer was right. The
// only answer states are Default and Selected. The practice screens (activity.templ) are separate on
// purpose, and a build-time test (activity_test.go) checks that placement uses nothing that can show a
// verdict. The language of these screens is English, as the practice screens are; Today's placement
// card (PlacementFor) is the only part with Hebrew chrome.

// Screens of the placement flow.
const (
	PlaceIntro     = "intro"
	PlaceResume    = "resume"
	PlaceLetters   = "letters"
	PlaceListening = "listening"
	PlaceReading   = "reading"
	PlaceTyping    = "typing"
	PlaceBreak     = "break"
	PlaceOffer     = "offer"
	PlaceResults   = "results"
	PlaceReview    = "review"
)

// Phases of a pick screen. Ask is the question. Picked is the neutral state after a tap: the chosen
// option is Selected, the others dim, and the screen moves on by itself.
const (
	PlaceAsk    = "ask"
	PlacePicked = "picked"
)

// Assets of the placement screens. The stylesheet is linked from the layout head, so a boosted
// navigation does not flash unstyled. The script is loaded by the page.
const (
	PlacementCSS = "/static/css/placement.css"
	PlacementJS  = "/static/js/placement.js"
)

// PlaceOptionCount is how many options a pick screen shows. It is placement.OptionCount, which views does
// not import.
const PlaceOptionCount = 4

// PlaceMaxReplaceTries is how many times a screen asks for a new word when its clip will not play.
// After that the screen says so and stays, so a broken audio path cannot loop.
const PlaceMaxReplaceTries = 3

// PlacementCopy is the placement chrome on Today's card, in the interface language. The placement
// screens themselves are English only.
type PlacementCopy struct {
	Lang        string
	Kicker      string
	TakeTitle   string
	TakeHint    string
	TodayResume string
	Heading     string
}

// PlacementFor returns the English chrome unless lang is he.
func PlacementFor(lang string) PlacementCopy {
	if lang == "he" {
		return PlacementCopy{
			Lang:        "he",
			Kicker:      "רשות",
			TakeTitle:   "בְּדִיקַת רָמָה",
			TakeHint:    "בְּדִיקָה קְצָרָה שֶׁל מִלִּים מוּכָּרוֹת. אֶפְשָׁר לַעֲזֹב וְלַחֲזֹר. הַיּוֹם נִשְׁאַר פָּתוּחַ.",
			TodayResume: "חֲזָרָה לַבְּדִיקָה",
			Heading:     "בוא נבדוק איפה אתה באמת",
		}
	}
	return PlacementCopy{
		Lang:        "en",
		Kicker:      "Optional",
		TakeTitle:   "Take the placement check",
		TakeHint:    "A short check of words you know. You can leave and come back. Today stays open.",
		TodayResume: "Resume placement",
		Heading:     "Let’s find where you really are.",
	}
}

// PlaceOptionView is one answer of a pick item. State is Default or Selected, never anything else.
type PlaceOptionView struct {
	ID     string
	Text   string
	Hebrew bool
	Key    string
	State  AnswerState
	Dim    bool
}

// PlaceLevelView is one skill's card on the results screen.
type PlaceLevelView struct {
	Skill  string
	Icon   string
	Tested bool
	Level  int
	Blurb  string
	Words  int
}

// PlaceWorkView is one row of "We'll work on first".
type PlaceWorkView struct {
	Pointed string
	Gloss   string
	Note    string
}

// PlacePathRow is one unit on the result path.
type PlacePathRow struct {
	Label string // "Unit 3: At home" or "Unit 3"
	Note  string
	Kind  string // past, start, next
}

// PlaceResultView is the results screen.
type PlaceResultView struct {
	Headline     string
	Sub          string
	Words        int
	Listening    PlaceLevelView
	Reading      PlaceLevelView
	Work         []PlaceWorkView
	Misses       int
	ReviewURL    string
	LettersFirst bool
	KnewNothing  bool
	StartHref    string
	Path         []PlacePathRow
}

// PlaceReviewRow is one missed word: what it means, what the learner picked, the pointed Hebrew, and
// its clip. PickedKind is picked, typed, or idk.
type PlaceReviewRow struct {
	ID         string
	Gloss      string
	Pointed    string
	AudioURL   string
	Picked     string
	PickedKind string
}

// PlaceReviewGroup is the misses of one part.
type PlaceReviewGroup struct {
	Title string
	Rows  []PlaceReviewRow
}

// PlaceReviewView is the review-of-misses screen.
type PlaceReviewView struct {
	Title  string
	Total  int
	Groups []PlaceReviewGroup
}

// PlacementView is one render of a placement screen, a full page or the fragment an htmx write
// returns. Only the fields of its Screen are set.
type PlacementView struct {
	Page
	Screen string
	Phase  string
	RunID  string
	// Token is the item id the answer is posted for.
	Token string
	// Options are the answers of a pick item.
	Options []PlaceOptionView
	// AudioURL is the clip of a listening or letters item. Prompt is the pointed Hebrew of a reading
	// item, or the English meaning of a typing item.
	AudioURL string
	Prompt   string
	// Index and Total place the item in its part: "4 / 12".
	Index int
	Total int
	// Tries is how many replacements this item has had. AudioFailed says a clip cannot be played and no
	// replacement is coming, so the screen tells the learner.
	Tries       int
	AudioFailed bool
	// Part is the number of the part just finished, on a break or an offer screen.
	Part   int
	Result *PlaceResultView
	Review *PlaceReviewView
}

// Asking is true on a pick screen before the tap.
func (v PlacementView) Asking() bool { return v.Phase != PlacePicked }

// Eyebrow is the small caps line above the title.
func (v PlacementView) Eyebrow() string {
	switch v.Screen {
	case PlaceLetters:
		return "Warm-up"
	case PlaceListening:
		return "Listening"
	case PlaceReading:
		return "Reading"
	case PlaceTyping:
		return "Typing"
	}
	return ""
}

// Question is the heading of an item screen.
func (v PlacementView) Question() string {
	switch v.Screen {
	case PlaceLetters:
		return "Which letter did you hear?"
	case PlaceListening:
		return "Listen, then pick the meaning."
	case PlaceReading:
		return "What does this word mean?"
	case PlaceTyping:
		return "Type this word in Hebrew"
	}
	return ""
}

// Count is "Placement, 4 of 12". The part's length is still the engine's.
func (v PlacementView) Count() string {
	return "Placement, " + strconv.Itoa(v.Index) + " of " + strconv.Itoa(v.Total)
}

// PartLabel is the part name in the header: Letters, Listening, Reading, Typing.
func (v PlacementView) PartLabel() string {
	switch v.Screen {
	case PlaceLetters:
		return "Letters"
	case PlaceListening:
		return "Listening"
	case PlaceReading:
		return "Reading"
	case PlaceTyping:
		return "Typing"
	}
	return ""
}

// AskHint is the line under a listening prompt. Other parts keep the shorter line.
func (v PlacementView) AskHint() string {
	if v.Screen == PlaceListening {
		return "Pick the meaning, or skip it"
	}
	return "Listen, then pick."
}

// ProgressName is the accessible name of the progress bar.
func (v PlacementView) ProgressName() string {
	return v.Eyebrow() + ": item " + strconv.Itoa(v.Index) + " of " + strconv.Itoa(v.Total)
}

// HasAudio is true on a screen that plays a clip.
func (v PlacementView) HasAudio() bool { return v.AudioURL != "" }

// PlaceKey is the keyboard badge and hotkey of the nth option.
func PlaceKey(i int) string { return strconv.Itoa(i + 1) }

// plOptionAttrs are the attributes of one answer option. While asking it is a submit button that posts
// the choice. After the tap it is an inert button, so the only thing a tap can do is move on.
func plOptionAttrs(o PlaceOptionView, asking bool) templ.Attributes {
	attrs := templ.Attributes{
		"data-hotkey":       o.Key,
		"aria-keyshortcuts": o.Key,
		"data-pl-option":    o.ID,
	}
	if asking {
		attrs["type"] = "submit"
		attrs["name"] = "option"
		attrs["value"] = o.ID
	} else {
		attrs["type"] = "button"
		attrs["aria-disabled"] = "true"
		if o.Dim {
			attrs["data-dim"] = ""
		}
	}
	return attrs
}

// plOrbAttrs are the attributes of the gold audio orb. It names the clip it plays, so motion.js can mark
// it as playing. After the tap the orb is inert.
func plOrbAttrs(asking bool) templ.Attributes {
	attrs := templ.Attributes{"data-audio": "pl-clip", "data-gold": "", "data-pl-play": ""}
	if !asking {
		attrs["aria-disabled"] = "true"
	}
	return attrs
}

// plChipAttrs are the attributes of a Replay or Slower chip. marker is its data attribute. After the tap
// a chip is inert, and a tap on it moves the screen on like a tap anywhere else.
func plChipAttrs(marker string, live bool) templ.Attributes {
	attrs := templ.Attributes{marker: ""}
	if !live {
		attrs["aria-disabled"] = "true"
	}
	return attrs
}

// plRootAttrs are the data attributes of the #placement root: the screen, the phase of a pick screen, and
// the state of its clip, which the script updates as the clip loads and plays.
func plRootAttrs(v PlacementView) templ.Attributes {
	attrs := templ.Attributes{"data-screen": v.Screen}
	if v.Phase != "" {
		attrs["data-phase"] = v.Phase
	}
	switch {
	case v.AudioFailed:
		attrs["data-audio-state"] = "error"
	case v.HasAudio() && v.Asking():
		attrs["data-audio-state"] = "idle"
	}
	return attrs
}

// plPrimaryAttrs is the one primary action of a screen: a submit button.
func plPrimaryAttrs(extra templ.Attributes) templ.Attributes {
	attrs := templ.Attributes{"type": "submit", "data-pl-primary": ""}
	for k, v := range extra {
		attrs[k] = v
	}
	return attrs
}

// plQuietAttrs is a secondary submit button. formnovalidate lets it through a required field.
func plQuietAttrs(name, value string) templ.Attributes {
	attrs := templ.Attributes{"type": "submit", "formnovalidate": true}
	if name != "" {
		attrs["name"] = name
		attrs["value"] = value
	}
	return attrs
}

// plBreakTitle is the heading of a break screen.
func plBreakTitle(part int) string { return "Part " + strconv.Itoa(part) + " done. Take a breath." }

// plWords formats a count of words with a thousands separator below ten thousand.
func plWords(n int) string {
	s := strconv.Itoa(n)
	if n >= 1000 && n < 1000000 {
		return s[:len(s)-3] + "," + s[len(s)-3:]
	}
	return s
}

// plSegments is the five-step meter of a level.
func plSegments(level int) []bool {
	out := make([]bool, 5)
	for i := range out {
		out[i] = i < level
	}
	return out
}

// plLevelName is "Level 3", or "Not tested yet".
func plLevelName(l PlaceLevelView) string {
	if !l.Tested {
		return "Not tested yet"
	}
	return "Level " + strconv.Itoa(l.Level)
}

// plMisses is "Review my 6 misses".
func plMisses(n int) string {
	if n == 1 {
		return "Review my 1 miss"
	}
	return "Review my " + strconv.Itoa(n) + " misses"
}

// plPickedLead is the start of the line under a missed word: "You picked “door”", "You said “I don’t know”", or "You typed".
func plPickedLead(kind string) string {
	switch kind {
	case "idk":
		return "You said “I don’t know”"
	case "typed":
		return "You typed"
	}
	return "You picked"
}
