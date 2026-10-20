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

// One English question per card type. A Designer ruling changes one line.
const (
	PromptRead       = "What does this word mean?"
	PromptRecall     = "How do you say this in Hebrew?"
	PromptLetter     = "What sound does this letter make?"
	PromptPattern    = "What does this pattern mean?"
	PromptPatternSay = "How do you say this in Hebrew?"
	PromptGrade      = "How well did you remember it?"
	PromptListen     = "What does the word you hear mean?"
	PromptSpell      = "How do you spell this word?"
)

// LetterPrompt is the question for a letter drill.
// A pair is still one letter on the front, so the question does not change.
func LetterPrompt(string) string {
	return PromptLetter
}

// PatternPrompt is the one question for an agreement drill.
// A Hebrew key asks for the meaning. A key with no Hebrew letter asks how
// to say that English in Hebrew.
func PatternPrompt(key string) string {
	if hasHebrew(key) || key == "" {
		return PromptPattern
	}
	return PromptPatternSay
}

func hasHebrew(s string) bool {
	for _, r := range s {
		if r >= 0x0590 && r <= 0x05FF {
			return true
		}
	}
	return false
}

// askedSide is the letter the front showed.
func askedSide(c ReviewCard) DrillSide {
	for _, side := range c.Sides {
		if side.Hebrew == c.Pointed {
			return side
		}
	}
	if len(c.Sides) == 1 {
		return c.Sides[0]
	}
	return DrillSide{Hebrew: c.Pointed}
}

// DageshNote is the one-line contrast for a pair that is the same letter
// with and without a dagesh. Both sounds have to be stored. A missing sound
// returns empty rather than a guessed consonant.
func DageshNote(sides []DrillSide) string {
	if len(sides) != 2 {
		return ""
	}
	a, b := sides[0], sides[1]
	if a.Sound == "" || b.Sound == "" || stripDagesh(a.Hebrew) != stripDagesh(b.Hebrew) {
		return ""
	}
	aDot := strings.ContainsRune(a.Hebrew, '\u05BC')
	bDot := strings.ContainsRune(b.Hebrew, '\u05BC')
	if aDot == bDot {
		return ""
	}
	with, without := a, b
	if !aDot {
		with, without = b, a
	}
	return "With the dot: " + with.Sound + ". Without: " + without.Sound + "."
}

func stripDagesh(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\u05BC' {
			return -1
		}
		return r
	}, s)
}

// flashLead is the line under the prompt of a flashcard.
func flashLead(c ReviewCard) string {
	if c.Revealed {
		return PromptGrade
	}
	return flashPrompt(c.Kind, c.Pointed)
}

func flashPrompt(kind, key string) string {
	switch kind {
	case "recall":
		return PromptRecall
	case "letter":
		return LetterPrompt(key)
	case "pattern":
		return PatternPrompt(key)
	case "listen":
		return PromptListen
	case "spell":
		return PromptSpell
	default:
		return PromptRead
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
