package views

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/a-h/templ"
)

// AppName is the pointed Hebrew wordmark. English sits beside mixed runs
// in its own bdi, not inside this string.
const AppName = "דֶּרֶךְ עִבְרִית"

// Sign-in copy. One interface language at a time. English is the default;
// ?lang=he is the Hebrew shell. GoogleSignInStart is auth's browser
// sign-in route. The shell only links to it.
//
// SignInGoogleHE is Google's Hebrew button label, unpointed, with the
// word Google rendered separately in Latin. SignInWord is the pointed
// teaser on the night slate. SignInLedeHE points עִבְרִית with sheva
// (U+05B0) under the bet.
const (
	SignInTitleHE     = "כְּנִיסָה"
	SignInTitleEN     = "Sign in"
	SignInLedeHE      = "עִבְרִית, כְּמוֹ שֶׁהַמֶּחְקָר אוֹמֵר"
	SignInLedeEN      = "Hebrew, the way research says works"
	SignInGoogleHE    = "כניסה באמצעות"
	SignInGoogleEN    = "Sign in with Google"
	SignInWord        = "כִּסֵּא"
	SignInWordGloss   = "(kisé, chair)"
	GoogleSignInStart = "/auth/google/start"
)

// UI is the shell copy for one language. Lang is he or en.
type UI struct {
	Lang            string
	Dir             string
	Skip            string
	Nav             string
	Today           string
	Review          string
	Progress        string
	Lessons         string
	Me              string
	Dark            string
	Light           string
	Logout          string
	Heading         string
	DueKicker       string
	DueLabel        string
	DueHint         string
	Start           string
	NextKicker      string
	Situation       string
	Open            string
	ZeroHE          string
	ZeroEN          string
	NoneHE          string
	NoneEN          string
	NoneMoreHE      string
	NoneMoreEN      string
	ReviewHeading   string
	ReviewLede      string
	ProgressHeading string
	ProgressLede    string
	WordsLabel      string
	UnitsLabel      string
	LessonHeading   string
	LessonLede      string
}

// AddressGender is "m" or "f": how Hebrew copy addresses the learner.
// An empty users.address_gender is masculine.
const (
	AddressMasculine = "m"
	AddressFeminine  = "f"
)

// Address picks Hebrew copy for address_gender. Anything other than "f" is masculine.
func Address(gender, masculine, feminine string) string {
	if gender == AddressFeminine {
		return feminine
	}
	return masculine
}

// UIFor returns the Hebrew shell for lang "he" and the English shell otherwise.
// An empty lang is English: a request with no ?lang and no hebrew_lang cookie
// uses this default. addressGender selects masculine or feminine Hebrew imperatives.
func UIFor(lang, addressGender string) UI {
	if lang == "he" {
		return hebrewUI(addressGender)
	}
	return englishUI()
}

func hebrewUI(addressGender string) UI {
	return UI{
		Lang: "he", Dir: "rtl",
		// Lessons and Me are the two destinations SPEC-017 added. Not yet checked against Pealim.
		Lessons: "שִׁעוּרִים", Me: "אֲנִי",
		Skip: Address(addressGender, "דַּלֵּג לַתֹּכֶן", "דַּלְּגִי לַתֹּכֶן"), Nav: "רָאשִׁי",
		Today: "הַיּוֹם", Review: "חֲזָרָה", Progress: "הִתְקַדְּמוּת",
		Dark: "כֵּהֶה", Light: "בָּהִיר", Logout: "יְצִיאָה",
		Heading:   "הַיּוֹם",
		DueKicker: "לַחֲזָרָה", DueLabel: "מִלִּים מֻכָּנוֹת", DueHint: "עַד עֶשְׂרִים בְּיוֹם",
		Start:      Address(addressGender, "הַתְחֵל", "הַתְחִילִי"),
		NextKicker: "הַשִּׁעוּר הַבָּא",
		Open:       "לַשִּׁעוּר",
		ZeroHE:     "אֵין חֲזָרוֹת עַכְשָׁיו", ZeroEN: "Nothing due",
		NoneHE: "אֵין שִׁעוּר בַּתּוֹר", NoneEN: "No lesson published yet",
		NoneMoreHE: "הַשִּׁעוּר הַבָּא יוֹפִיעַ כָּאן", NoneMoreEN: "Your next lesson will appear here",
		ReviewHeading: "חֲזָרָה", ReviewLede: "הַתּוֹר עוֹד רֵיק.",
		ProgressHeading: "הִתְקַדְּמוּת", ProgressLede: "מִלִּים שֶׁאֶפְשָׁר לִזְכֹּר, וִיחִידוֹת שֶׁהֻשְׁלְמוּ.",
		WordsLabel: "מִלִּים", UnitsLabel: "יְחִידוֹת",
		LessonHeading: "הַשִּׁעוּר", LessonLede: "מִכָּאן נִכְנָסִים לַשִּׁעוּר.",
	}
}

func englishUI() UI {
	return UI{
		Lang: "en", Dir: "ltr",
		Skip: "Skip to content", Nav: "Main",
		Today: "Today", Review: "Review", Progress: "Progress",
		Lessons: "Lessons", Me: "Me",
		Dark: "Dark", Light: "Light", Logout: "Log out",
		Heading:   "Today",
		DueKicker: "To review", DueLabel: "Words ready", DueHint: "Up to twenty a day",
		Start:      "Start",
		NextKicker: "Next lesson",
		Open:       "Open lesson",
		ZeroHE:     "אֵין חֲזָרוֹת עַכְשָׁיו", ZeroEN: "Nothing due",
		NoneHE: "אֵין שִׁעוּר בַּתּוֹר", NoneEN: "No lesson published yet",
		NoneMoreHE: "הַשִּׁעוּר הַבָּא יוֹפִיעַ כָּאן", NoneMoreEN: "Your next lesson will appear here",
		ReviewHeading: "Review", ReviewLede: "The queue is still empty.",
		ProgressHeading: "Progress", ProgressLede: "Words you can remember, and units you have finished.",
		WordsLabel: "Words", UnitsLabel: "Units",
		LessonHeading: "Lesson", LessonLede: "The lesson opens from here.",
	}
}

// DocTitle is the document title for a nav id: today, review, progress, lesson.
func (u UI) DocTitle(current string) string {
	name := u.Heading
	switch current {
	case "review":
		name = u.ReviewHeading
	case "progress":
		name = u.ProgressHeading
	case "lesson":
		name = u.LessonHeading
	case "lessons":
		name = u.Lessons
	case "me":
		name = u.Me
	case "signin":
		if u.Lang == "en" {
			name = SignInTitleEN
		} else {
			name = SignInTitleHE
		}
	}
	return name + " · " + AppName
}

// Href keeps the Hebrew shell on in-app links. English is the default, so
// those links stay bare; ?lang=en is only needed to override a Hebrew cookie.
func (u UI) Href(path string) string {
	if u.Lang == "he" {
		return path + "?lang=he"
	}
	return path
}

// Page is the data the layout shell needs. CSRF is the token auth.CSRFToken
// returned for this request.
type Page struct {
	UI
	Title   string
	CSRF    string
	Current string
}

// NewPage builds a shell page. lang "he" selects the Hebrew UI. Any other
// value, including empty, selects English.
func NewPage(lang, csrf, current string) Page {
	ui := UIFor(lang, AddressMasculine)
	return Page{UI: ui, Title: ui.DocTitle(current), CSRF: csrf, Current: current}
}

// PlacementVM is the Today placement card.
// The same fields live on internal/web/viewmodel.go as HomeVM.Placement.
// Nil means the learner has never taken the check.
// A non-nil value with Active false and Finished false is the shell built
// without a placement store, so the card stays off.
type PlacementVM struct {
	Active              bool
	Finished            bool
	CardsRated          int
	Level               int
	EstimatedKnownWords int
	TakenAt             time.Time
}

// Home is Today. Due is the review count from the scheduler.
// Lesson is false when no published lesson is waiting. LessonTitle is that
// lesson's unit title. LessonGloss is the transliteration and English
// meaning, including the parentheses. LessonHref is the path of that lesson,
// without a language query. Words are its new-word chips.
// KnownText is the known-word count. Placement is nil when the learner has
// never taken the check. PlacementState is "", "active", or "finished".
type Home struct {
	UI
	CSRF           string
	Due            int
	DueText        string
	KnownText      string
	Lesson         bool
	LessonTitle    string
	LessonGloss    string
	LessonHref     string
	Words          []Chip
	Placement      *PlacementVM
	PlacementState string
	PlacementWords string
	PlacementLevel string
	PlacementRated string
	PlacementAt    string
	// Today is what the shell decided for this visit: the one next action and the
	// cards beside it (INIT-015/SPEC-017).
	Today TodayView
}

// ReviewCard is one study visit, or Done when the queue is finished.
type ReviewCard struct {
	CSRF          string
	Lang          string
	Lede          string
	CardID        int64
	Kind          string
	Pointed       string
	Unpointed     string
	ShowUnpointed bool
	AudioURL      string
	Gloss         string
	Translit      string
	Gender        string
	Index         int
	Total         int
	Retry         bool
	Done          bool
	Revealed      bool
	Answered      int
	Correct       int
	// Act is the checked activity this visit is asked through. Nil means the card is
	// a flashcard (or a drill), graded by the learner.
	Act *ActivityView
	// Sides, ExampleText, ExampleTranslit, and ExampleGloss are the drill answer.
	// They are empty until the card is revealed, so the front has no answer markup.
	// TellApart and DageshNote are the same: empty when the data has no cue.
	// DageshHard is the position line after DageshNote for בּ/ב, כּ/כ, and פּ/פ.
	Sides           []DrillSide
	ExampleText     string
	ExampleTranslit string
	ExampleGloss    string
	TellApart       string
	DageshNote      string
	DageshHard      string
}

// DrillSide is one letter, or one agreement form, on the back of a drill card.
// For a letter, Name, NameTranslit, and Sound come from the alef-bet table:
// the name with niqqud, its transliteration, and the plain-English sound.
// The keyboard name table is separate and still has no niqqud. Translit and
// Gloss are the lexicon reading and meaning of an agreement form. An example
// word is carried on the card, not on the side.
type DrillSide struct {
	Hebrew       string
	Name         string
	NameTranslit string
	Sound        string
	Translit     string
	Gloss        string
}

// answerSide reports whether this render may show the transliteration and the clip.
// That follows the mode's hold, not a list of card types. A revealed card may.
// Read and listen hold neither the transliteration nor the clip, so both stay
// on the front. Every other current mode holds both, so both wait.
func answerSide(c ReviewCard) bool {
	if c.Revealed {
		return true
	}
	hold := ReviewModeFor(c.Kind).Hold
	return !hold.Translit && !hold.Audio
}

// Flag is the attribute value for a bool.
func Flag(on bool) string {
	if on {
		return "true"
	}
	return "false"
}

// Num is the decimal form of n for a template.
func Num(n int) string { return strconv.Itoa(n) }

// Num64 is the decimal form of n for a template.
func Num64(n int64) string { return strconv.FormatInt(n, 10) }

// Page is the shell around Today.
func (h Home) Page() Page {
	return Page{UI: h.UI, Title: h.DocTitle("today"), CSRF: h.CSRF, Current: "today"}
}

// Chip is one new-word chip. Gender is "m" or "f" for a noun, otherwise empty.
// Gloss is the English transliteration and meaning, including the parentheses,
// so the Hebrew word is never the only thing on the chip.
type Chip struct {
	Text   string
	Gender string
	Gloss  string
}

// GenderLabel is the noun chip ז׳ or נ׳. Other values have no chip.
func GenderLabel(gender string) string {
	switch gender {
	case AddressMasculine:
		return "ז׳"
	case AddressFeminine:
		return "נ׳"
	default:
		return ""
	}
}

// BtnClass is the one primary on Today. primary is the filled action.
func BtnClass(primary bool) string {
	if primary {
		return "btn btn-primary"
	}
	return "btn btn-tonal"
}

// HXHeaders is the JSON object htmx sends as request headers from <body hx-headers>.
func HXHeaders(token string) string {
	b, err := json.Marshal(map[string]string{"X-CSRF-Token": token})
	if err != nil {
		return "{}"
	}
	return string(b)
}

// IconHref points at one symbol in the vendored sprite. Unknown names fall
// back to help so a typo cannot request streak or xp.
func IconHref(name string) templ.SafeURL {
	return templ.SafeURL(iconRef(name))
}

func iconClass(directional bool) string {
	if directional {
		return "icon icon-directional"
	}
	return "icon"
}

// ThemeBoot is the only inline script. It applies a stored light or dark
// choice before first paint. The toggle and the htmx motion live in
// /static/js/shell.js. Keep this under 300 bytes; ThemeBootCSP is its hash.
const ThemeBoot = `(function(){try{var t=localStorage.getItem("hebrew-theme");if(t==="light"||t==="dark")document.documentElement.setAttribute("data-theme",t)}catch(e){}})()`

// ThemeBootCSP is the Content-Security-Policy script-src hash of ThemeBoot:
// sha256 of the exact script bytes, standard base64, without quotes.
const ThemeBootCSP = "sha256-J/VHwnDnlr+GqZg8u44o21pUuS0tshGfqO3bcm2kCfA="

// themeHead prints ThemeBoot in the document head.
func themeHead() templ.Component {
	return templ.Raw("<script>" + ThemeBoot + "</script>")
}
