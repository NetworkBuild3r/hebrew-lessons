package views

// Screen is one lesson page. Empty is the honest state when nothing
// published is waiting. Title, Gloss, and Situation describe the unit.
// Activity is the current step, or the completion panel when Done is set.
type Screen struct {
	Empty     bool
	Title     string
	Gloss     string
	Situation string
	Activity  Activity
}

// Activity is one step, swapped in place by htmx. Done replaces the step
// with the score. Post is the lesson path the form submits to. HomeHref
// is Today, including a language query when the shell is Hebrew.
type Activity struct {
	Done          bool
	Score         string
	Prompt        string
	Progress      string
	Audio         string
	Words         []Word
	Feedback      string
	FeedbackClass string
	Choices       []Choice
	CSRF          string
	Run           string
	Post          string
	Continue      string
	Asking        bool
	HomeHref      string
}

// Word is one token in an activity. Plain is a number or a Latin token.
// Gap is the cloze blank. Show* gates the three learner fields: pointed
// Hebrew, transliteration, and the English gloss.
type Word struct {
	Text         string
	Translit     string
	Gloss        string
	Gender       string
	Audio        string
	ShowText     bool
	ShowTranslit bool
	ShowGloss    bool
	Gap          bool
	Plain        bool
}

// shown returns value when the field is visible.
func shown(ok bool, value string) string {
	if !ok {
		return ""
	}
	return value
}

// Choice is one answer button. Value is the lexeme id, never the pointed text.
// Hebrew choices show the pointed word. Translit is set only when that
// choice should show a transliteration. English choices show Label, the gloss.
type Choice struct {
	Value    string
	Label    string
	Text     string
	Translit string
	Hebrew   bool
}
