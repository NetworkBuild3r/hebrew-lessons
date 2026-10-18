package views

import (
	"encoding/json"

	"github.com/a-h/templ"
)

// AppName is the pointed Hebrew wordmark. English sits beside mixed runs
// in its own bdi, not inside this string.
const AppName = "דֶּרֶךְ עִבְרִית"

// UnitTitle is the one lesson the shell shows until the queue is wired.
const UnitTitle = "הַבֹּקֶר שֶׁלִּי"

// UI is the shell copy for one language. Lang is he or en.
type UI struct {
	Lang            string
	Dir             string
	Skip            string
	Nav             string
	Today           string
	Review          string
	Progress        string
	Dark            string
	Light           string
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
	ReviewHeading   string
	ReviewLede      string
	ProgressHeading string
	ProgressLede    string
	WordsLabel      string
	UnitsLabel      string
	LessonHeading   string
	LessonLede      string
}

// UIFor returns the English shell for lang "en" and the Hebrew shell otherwise.
func UIFor(lang string) UI {
	if lang == "en" {
		return englishUI()
	}
	return hebrewUI()
}

func hebrewUI() UI {
	return UI{
		Lang: "he", Dir: "rtl",
		Skip: "דַּלֵּג לַתֹּכֶן", Nav: "רָאשִׁי",
		Today: "הַיּוֹם", Review: "חֲזָרָה", Progress: "הִתְקַדְּמוּת",
		Dark: "כֵּהֶה", Light: "בָּהִיר",
		Heading:   "הַיּוֹם",
		DueKicker: "לַחֲזָרָה", DueLabel: "מִלִּים מוּכָנוֹת", DueHint: "עַד עֶשְׂרִים בְּיוֹם",
		Start:      "הַתְחֵל",
		NextKicker: "הַשִּׁיעוּר הַבָּא",
		Situation:  "Getting ready in the morning.",
		Open:       "לַשִּׁיעוּר",
		ZeroHE:     "אֵין חֲזָרוֹת עַכְשָׁיו", ZeroEN: "Nothing due",
		NoneHE: "אֵין שִׁיעוּר בַּתּוֹר", NoneEN: "No lesson queued",
		ReviewHeading: "חֲזָרָה", ReviewLede: "הַתּוֹר עוֹד רֵיק.",
		ProgressHeading: "הִתְקַדְּמוּת", ProgressLede: "מִלִּים שֶׁאֶפְשָׁר לִזְכֹּר, וִיחִידוֹת שֶׁהֻשְׁלְמוּ.",
		WordsLabel: "מִלִּים", UnitsLabel: "יְחִידוֹת",
		LessonHeading: "הַשִּׁיעוּר", LessonLede: "מִכָּאן נִכְנָסִים לַשִּׁיעוּר.",
	}
}

func englishUI() UI {
	return UI{
		Lang: "en", Dir: "ltr",
		Skip: "Skip to content", Nav: "Main",
		Today: "Today", Review: "Review", Progress: "Progress",
		Dark: "Dark", Light: "Light",
		Heading:   "Today",
		DueKicker: "To review", DueLabel: "Words ready", DueHint: "Up to twenty a day",
		Start:      "Start",
		NextKicker: "Next lesson",
		Situation:  "Getting ready in the morning.",
		Open:       "Open lesson",
		ZeroHE:     "אֵין חֲזָרוֹת עַכְשָׁיו", ZeroEN: "Nothing due",
		NoneHE: "אֵין שִׁיעוּר בַּתּוֹר", NoneEN: "No lesson queued",
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
	}
	return name + " · " + AppName
}

// Href keeps the English shell on in-app links.
func (u UI) Href(path string) string {
	if u.Lang != "en" {
		return path
	}
	return path + "?lang=en"
}

// Page is the data the layout shell needs. CSRF is the token auth.CSRFToken
// returned for this request.
type Page struct {
	UI
	Title   string
	CSRF    string
	Current string
}

// NewPage builds a shell page. lang "en" selects the English UI.
func NewPage(lang, csrf, current string) Page {
	ui := UIFor(lang)
	return Page{UI: ui, Title: ui.DocTitle(current), CSRF: csrf, Current: current}
}

// Home is the Today placeholder. Due is the review count. Lesson is false
// when nothing is queued. Words are the new-word chips.
type Home struct {
	UI
	CSRF    string
	Due     int
	DueText string
	Lesson  bool
	Words   []string
}

// Page is the shell around Today.
func (h Home) Page() Page {
	return Page{UI: h.UI, Title: h.DocTitle("today"), CSRF: h.CSRF, Current: "today"}
}

// SampleWords is the pointed chip row for the placeholder lesson.
func SampleWords() []string {
	return []string{"בֹּקֶר", "קָפֶה", "קָם"}
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
	switch name {
	case "today", "review", "progress", "theme", "next", "play", "sound", "help", "check":
		return templ.SafeURL("/static/icons/sprite.svg#icon-" + name)
	default:
		return templ.SafeURL("/static/icons/sprite.svg#icon-help")
	}
}

func iconClass(directional bool) string {
	if directional {
		return "icon icon-directional"
	}
	return "icon"
}

// themeHead is the only inline script: it applies a stored light/dark choice
// before paint, and the toggle writes that choice back to localStorage.
func themeHead() templ.Component {
	return templ.Raw(`<script>(function(){var k="hebrew-theme";try{var t=localStorage.getItem(k);if(t==="light"||t==="dark")document.documentElement.setAttribute("data-theme",t);}catch(e){}function dark(){var t=document.documentElement.getAttribute("data-theme");return t==="dark"||(t!=="light"&&matchMedia("(prefers-color-scheme: dark)").matches);}function paint(){var b=document.getElementById("theme-toggle");if(b)b.setAttribute("aria-pressed",dark()?"true":"false");}document.addEventListener("click",function(ev){var node=ev.target&&ev.target.closest?ev.target:ev.target&&ev.target.parentElement;var b=node&&node.closest&&node.closest("#theme-toggle");if(!b)return;var next=dark()?"light":"dark";document.documentElement.setAttribute("data-theme",next);try{localStorage.setItem(k,next);}catch(err){}paint();});document.addEventListener("DOMContentLoaded",paint);})();</script>`)
}
