package views

import (
	"strings"

	"github.com/a-h/templ"
)

// The activity screens (INIT-015/SPEC-016): one frame for every activity.
// Progress sits at the top, the prompt in the middle, the answers in the thumb
// zone, and one gold action. A checked answer is followed by a feedback sheet
// that shows the right response (RSCH-006 [11]), not right or wrong alone.
//
// This file and activity.templ are the practice screens. The placement views
// (placement.templ) do not use anything from here: placement gives no
// feedback, so it must not share a component that could show any. A build-time
// test (activity_test.go) checks that.

// Phases of a checked activity.
const (
	// ActAsk is the question, before an answer.
	ActAsk = "ask"
	// ActFeedback is the screen after a checked answer: the options show
	// Correct or Wrong and the feedback sheet is up.
	ActFeedback = "feedback"
)

// Kinds the frame knows. They are the strings of activity.Kind; views does not
// import the engine, which keeps the screens a pure function of their data.
const (
	KindListenPick = "listen_pick"
	KindReadPick   = "read_pick"
	KindWordMatch  = "word_match"
	KindAudioMatch = "audio_match"
	KindTypeIt     = "type_it"
	KindDictation  = "dictation"
)

// The grade labels of the flashcard. They are neutral on purpose: no colour,
// no word that says right or wrong. The order is rating 1 to 4.
var GradeLabels = [4]string{"Forgot", "Hard", "Got it", "Easy"}

// OptionView is one answer of a pick item.
type OptionView struct {
	Text  string
	State AnswerState
}

// TileView is one tile of a match item. Key is the keyboard badge: a digit for
// the left column, a letter for the right. Name is the accessible name, which
// says what the tile is, so a screen-reader learner can pair without seeing it.
type TileView struct {
	Text     string
	Hebrew   bool
	AudioURL string
	Key      string
	Index    int
	Name     string
	State    AnswerState
	// Pair is the pair number once the tile is in a made pair. Empty before.
	Pair string
}

// ConfusionView is the line "You picked X. That's Y." Lead is "You picked" or
// "You typed". Picked is X. Thats is Y, a Hebrew word. PickedHebrew says X is
// Hebrew too (a typed answer).
type ConfusionView struct {
	Lead         string
	Picked       string
	PickedHebrew bool
	Thats        string
}

// SegView is a stretch of what the learner typed. Wrong letters are underlined.
// Missing is a letter the learner left out.
type SegView struct {
	Text    string
	Wrong   bool
	Missing bool
}

// PairView is a pair the sheet spells out after a match item: the word, what
// it means, and its clip.
type PairView struct {
	Hebrew   string
	Gloss    string
	AudioURL string
}

// FeedbackView is the sheet after a checked answer. It is built only from an
// activity.Result with Feedback set.
type FeedbackView struct {
	Correct  bool
	NearMiss bool
	DontKnow bool
	// Title is the one-line outcome: Correct, Not quite, Almost, or the line for a
	// learner who did not know.
	Title string
	// Answer is the right response: the gloss for a pick or match item, the pointed
	// Hebrew for a typed one.
	Answer string
	// Hebrew is the pointed form of the word asked.
	Hebrew string
	Gloss  string
	// AudioURL is the clip of the word asked. Empty when the word has none.
	AudioURL   string
	Typed      []SegView
	TypedLabel string
	Confusions []ConfusionView
	Pairs      []PairView
}

// ActivityView is one checked activity, as a screen.
type ActivityView struct {
	Kind   string
	Phase  string
	CSRF   string
	Lang   string
	CardID int64
	// Token is the item token the answer post carries.
	Token string
	// Index and Total place the visit in the session.
	Index int
	Total int
	// Lead is the instruction above the prompt.
	Lead string
	// Hebrew, Gloss and AudioURL are the prompt, whichever the skill allows.
	Hebrew   string
	Gloss    string
	AudioURL string
	Options  []OptionView
	Left     []TileView
	Right    []TileView
	// HebrewCol marks the left column of a match item as Hebrew text, which is
	// laid out right to left.
	HebrewCol bool
	Feedback  *FeedbackView
}

// Typed reports whether the activity is answered with the Hebrew keyboard.
func (a ActivityView) Typed() bool { return a.Kind == KindTypeIt || a.Kind == KindDictation }

// Match reports whether the activity pairs two columns.
func (a ActivityView) Match() bool { return a.Kind == KindWordMatch || a.Kind == KindAudioMatch }

// Pick reports whether the activity is a four-option pick.
func (a ActivityView) Pick() bool { return a.Kind == KindListenPick || a.Kind == KindReadPick }

// Asking is true before the answer.
func (a ActivityView) Asking() bool { return a.Phase != ActFeedback }

// Done is how many visits are finished when the screen is shown.
func (a ActivityView) Done() int {
	if a.Asking() {
		return a.Index - 1
	}
	return a.Index
}

// Region is the accessible name of the activity frame.
func (a ActivityView) Region() string {
	if a.Asking() {
		return a.Lead
	}
	return "Feedback"
}

// OptionKeys are the keyboard badges of a pick item, in order.
var OptionKeys = []string{"1", "2", "3", "4"}

// MatchKeys are the right-column badges of a match item. The left column uses
// the digits.
const MatchKeys = "ABCDE"

// homeHref is the Today link that keeps the interface language.
func homeHref(lang string) string {
	if lang == "he" {
		return "/?lang=he"
	}
	return "/"
}

// lowerKey is the data-hotkey of a badge: digits stay, letters go lower case.
func lowerKey(k string) string { return strings.ToLower(k) }

// actAttrs are the attributes of an answer option or a tile. Everything a
// script needs is a data attribute, so no script needs inline style.
func optionAttrs(key string, i int, asking bool) templ.Attributes {
	attrs := templ.Attributes{
		"data-pick":         Num(i),
		"data-hotkey":       lowerKey(key),
		"aria-keyshortcuts": key,
	}
	if !asking {
		attrs["aria-disabled"] = "true"
	}
	return attrs
}

func tileAttrs(t TileView, side string, asking bool) templ.Attributes {
	attrs := templ.Attributes{
		"data-tile":         side,
		"data-index":        Num(t.Index),
		"data-hotkey":       lowerKey(t.Key),
		"data-label":        t.Name,
		"aria-label":        t.Name,
		"aria-keyshortcuts": t.Key,
	}
	if t.Pair != "" {
		attrs["data-locked"] = ""
		attrs["aria-disabled"] = "true"
	}
	if !asking {
		attrs["aria-disabled"] = "true"
	}
	return attrs
}

// orbAttrs are the attributes of an audio orb: which audio element it plays and
// where the clip is. A prompt orb also carries data-hint: playing it a second
// time before answering counts as a hint (activity.Response.Hinted).
func orbAttrs(audioID, url string, prompt bool) templ.Attributes {
	attrs := templ.Attributes{"data-audio": audioID, "data-src": url}
	if prompt {
		attrs["data-hint"] = "replay"
	}
	return attrs
}

// buttonSubmit makes a Button a submit button. name and value are optional.
func submitAttrs(extra templ.Attributes) templ.Attributes {
	attrs := templ.Attributes{"type": "submit"}
	for k, v := range extra {
		attrs[k] = v
	}
	return attrs
}

// primaryAttrs is the gold action: it submits, and a script finds it by
// data-act-primary. disabled starts it off until there is an answer.
func primaryAttrs(disabled bool) templ.Attributes {
	attrs := submitAttrs(templ.Attributes{"data-act-primary": ""})
	if disabled {
		attrs["disabled"] = true
	}
	return attrs
}

func quietSubmit(name, value string) templ.Attributes {
	return submitAttrs(templ.Attributes{"name": name, "value": value, "formnovalidate": true})
}

// feedbackClass is the class of the sheet content, which the stylesheet keys the
// feedback colours on.
func feedbackClass(f *FeedbackView) string {
	if f == nil {
		return "fb"
	}
	if f.Correct {
		return "fb feedback-right"
	}
	return "fb feedback-wrong"
}

// flashPhase is the phase attribute of the flashcard frame.
func flashPhase(c ReviewCard) string {
	if c.Revealed {
		return ActFeedback
	}
	return ActAsk
}

// flashDone is the visits finished when the flashcard is shown.
func flashDone(c ReviewCard) int {
	if c.Revealed {
		return c.Index
	}
	return c.Index - 1
}

// flashLead is the line under the prompt of a flashcard.
func flashLead(c ReviewCard) string {
	if c.Revealed {
		return "How well did you remember it?"
	}
	switch c.Kind {
	case "recall":
		return "Say it in Hebrew, then check."
	case "read":
		return "Say what it means, then check."
	default:
		return "Say it, then check."
	}
}

// flashHebrew is the Hebrew a flashcard shows: unpointed once the card is
// stable enough (fsrs.ShowUnpointed), pointed otherwise.
func flashHebrew(c ReviewCard) string {
	if c.ShowUnpointed {
		return c.Unpointed
	}
	return c.Pointed
}

// flashTranslit is the transliteration the flashcard may show on this render. On
// recall it names the answer, so it waits for the reveal.
func flashTranslit(c ReviewCard) string {
	if answerSide(c) {
		return c.Translit
	}
	return ""
}
