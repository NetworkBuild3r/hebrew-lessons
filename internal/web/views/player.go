package views

import "github.com/a-h/templ"

// Screen is one lesson page. Empty is the honest state when nothing
// published is waiting. Title, Gloss, and Situation describe the unit.
// Activity is the current step, or the completion panel when Done is set.
type Screen struct {
	Empty     bool
	Title     string
	Gloss     string
	Situation string
	Activity  Activity
	// Frame is the lesson frame around the step: progress bar, step heading, outline,
	// and the report sheet (INIT-015/SPEC-017).
	Frame LessonFrame
}

// Activity is one step, swapped in place by htmx. Done replaces the step
// with the score. Post is the lesson path the form submits to. HomeHref
// is Today, including a language query when the shell is Hebrew.
type Activity struct {
	Done   bool
	Score  string
	Prompt string
	// PromptWord is the Hebrew word a multi-word recall names. The template
	// writes it as: What does <bdi lang="he" dir="rtl">WORD</bdi> mean?
	// Empty for every other prompt, including a recall of one word.
	PromptWord string
	Progress   string
	Audio      string
	// AudioLabel is the accessible name of the line clip, such as "Play the line"
	// or "Play" while the spelling is hidden. AudioHebrew is the one word in
	// that name, when the line is a single word and the spelling is shown.
	// Both are empty when Audio is empty.
	AudioLabel    string
	AudioHebrew   string
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
	// Dialogue is set for a dialogue step. It replaces Words and Audio.
	Dialogue *Dialogue
}

// Dialogue is a conversation between two people, shown line by line in order.
type Dialogue struct {
	Lines []DialogueLine
}

// DialogueLine is one line. Speaker is a or b, Name is the speaker's fixed name
// (SpeakerName), and Words are the line's words with their sound and meaning.
// Audio is the clip in the speaker's voice, empty when there is none, and then
// the line has no play control.
type DialogueLine struct {
	Index   int
	Speaker string
	Name    string
	Words   []Word
	Audio   string
}

// The two speakers' names. They are neutral first names, fixed in copy, so the
// learner can follow who is talking: a is the woman's voice and b is the man's.
const (
	SpeakerNameA = "Dana"
	SpeakerNameB = "Avi"
)

// SpeakerName is the fixed name shown for a speaker. An unknown speaker has none.
func SpeakerName(speaker string) string {
	switch speaker {
	case "a":
		return SpeakerNameA
	case "b":
		return SpeakerNameB
	default:
		return ""
	}
}

// dialogueLineClass is the CSS class of a line. The two speakers sit at opposite
// ends of the column, so a line's place tells who is speaking as well as its name.
func dialogueLineClass(speaker string) string {
	if speaker == "b" {
		return "dlg__line dlg__line--b"
	}
	return "dlg__line dlg__line--a"
}

// dialogueClipID is the id of the audio element of line i.
func dialogueClipID(i int) string { return "dlg-clip-" + Num(i) }

// dialogueOrbAttrs are the attributes of a line's play control. data-dlg-line
// marks it for dialogue.js, and data-audio names its clip so the orb pulses
// while the clip plays (motion.js).
func dialogueOrbAttrs(i int) templ.Attributes {
	return templ.Attributes{"data-audio": dialogueClipID(i), "data-dlg-line": Num(i)}
}

// wordBlocksClass is the activity line. A multi-word recall centres that line
// under the hero orb and lays transliteration, gloss, and orb on one grid.
func wordBlocksClass(a Activity) string {
	if a.PromptWord != "" {
		return "word-blocks word-blocks--recall"
	}
	return "word-blocks"
}

// orbClass is the lesson clip button. The hero is the line (56px). A word orb
// is the 28px disc inside a 44px hit box. Neither is the dialogue c-orb.
func orbClass(hero bool) string {
	if hero {
		return "orb orb--hero"
	}
	return "orb orb--word"
}

// clipClass groups the native audio and its orb. The hero sits on the prompt.
func clipClass(hero bool) string {
	if hero {
		return "clip clip--hero"
	}
	return "clip"
}

// hasClip reports whether the step has a line clip or a per-word clip.
func hasClip(a Activity) bool {
	if a.Audio != "" {
		return true
	}
	for _, w := range a.Words {
		if w.Audio != "" {
			return true
		}
	}
	return false
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
	// Target is the word a multi-word recall is asking about. The line marks
	// it, and its transliteration and gloss stay hidden until the answer.
	Target bool
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
