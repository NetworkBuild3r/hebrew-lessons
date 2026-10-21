package views

// The Hebrew on-screen keyboard (INIT-015/SPEC-011). Typed answers need
// Hebrew input, and most learners have no Hebrew layout installed. The layout
// is the standard Israeli one (SI-1452), in the physical order of a PC
// keyboard, so a learner who later types on a real Hebrew keyboard has
// already learned where the letters are.
//
// HebrewKeyRows is the one key table. The templ renders the on-screen keys
// from it, and hekbd.js builds the physical-key map from the rendered
// data-code and data-ch attributes, so the two can never disagree.

// HKey is one letter key. Ch is the letter. Code is the KeyboardEvent.code of
// the physical key that types it on the Israeli layout. Latin is the legend
// printed on that key on a US keyboard, shown as the key hint. Name is the
// Hebrew letter name, the key's accessible label. Final marks a final form
// (sofit), which has its own key, as on the physical layout.
type HKey struct {
	Ch    string
	Code  string
	Latin string
	Name  string
	Final bool
}

// HebrewKeyRows is the 22 letters and the five final forms (27 keys) in three
// rows. Rows read left to right in the page, as the keys sit on a PC
// keyboard, whatever the direction of the page.
var HebrewKeyRows = [][]HKey{
	{
		{"ק", "KeyE", "e", "קוף", false},
		{"ר", "KeyR", "r", "ריש", false},
		{"א", "KeyT", "t", "אלף", false},
		{"ט", "KeyY", "y", "טית", false},
		{"ו", "KeyU", "u", "ואו", false},
		{"ן", "KeyI", "i", "נון סופית", true},
		{"ם", "KeyO", "o", "מם סופית", true},
		{"פ", "KeyP", "p", "פא", false},
	},
	{
		{"ש", "KeyA", "a", "שין", false},
		{"ד", "KeyS", "s", "דלת", false},
		{"ג", "KeyD", "d", "גימל", false},
		{"כ", "KeyF", "f", "כף", false},
		{"ע", "KeyG", "g", "עין", false},
		{"י", "KeyH", "h", "יוד", false},
		{"ח", "KeyJ", "j", "חית", false},
		{"ל", "KeyK", "k", "למד", false},
		{"ך", "KeyL", "l", "כף סופית", true},
		{"ף", "Semicolon", ";", "פא סופית", true},
	},
	{
		{"ז", "KeyZ", "z", "זיין", false},
		{"ס", "KeyX", "x", "סמך", false},
		{"ב", "KeyC", "c", "בית", false},
		{"ה", "KeyV", "v", "הא", false},
		{"נ", "KeyB", "b", "נון", false},
		{"מ", "KeyN", "n", "מם", false},
		{"צ", "KeyM", "m", "צדי", false},
		{"ת", "Comma", ",", "תו", false},
		{"ץ", "Period", ".", "צדי סופית", true},
	},
}

// LetterName is the Hebrew name of ch in the keyboard table.
// A letter that is not on the table has an empty name.
func LetterName(ch string) string {
	for _, row := range HebrewKeyRows {
		for _, k := range row {
			if k.Ch == ch {
				return k.Name
			}
		}
	}
	return ""
}

// KeyboardCopy is the words on the keyboard's own controls. Lang is the
// language of those words, set on the controls so a screen reader switches voice.
type KeyboardCopy struct {
	Lang   string
	Dir    string
	Group  string // accessible name of the whole keyboard
	Back   string
	Space  string
	Enter  string
	Hints  string // the "show key hints" toggle
	Delete string // accessible name of the backspace key
}

// KeyboardCopyFor returns the control words for lang. Anything but "he" is English.
func KeyboardCopyFor(lang string) KeyboardCopy {
	if lang == "he" {
		return KeyboardCopy{
			Lang: "he", Dir: "rtl",
			Group: "מקלדת עברית", Back: "מחק", Space: "רווח", Enter: "שלח",
			Hints: "הצג רמזי מקשים", Delete: "מחק אות",
		}
	}
	return KeyboardCopy{
		Lang: "en", Dir: "ltr",
		Group: "Hebrew keyboard", Back: "Delete", Space: "Space", Enter: "Enter",
		Hints: "Show key hints", Delete: "Delete the last letter",
	}
}

// Assets the keyboard needs. HebrewKeyboard emits both tags itself, so a page
// that uses it does not edit the shared layout.
const (
	KeyboardCSS = "/static/css/keyboard.css"
	KeyboardJS  = "/static/js/hekbd.js"
)
