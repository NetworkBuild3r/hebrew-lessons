// Package units is the unit document and its validator.
//
// Backend owns these structs and the Validate signature.
// MCP Builder owns validate_*.go and rules.go, which fill in the rules.
//
// # Brief rules (INIT-015/SPEC-020)
//
// ValidateWithBrief runs every rule of Validate and then checks the unit
// against the learner's lesson brief. It takes a BriefFacts that this package
// owns, never a brief.Brief: internal/brief imports internal/db, and internal/db
// imports this package, so the dependency cannot point the other way. The
// publish path in internal/mcpstore converts a brief.Brief into BriefFacts.
//
// ValidateWithBrief replaces the two v2 coverage rules (coverage.story and
// coverage.listening). They judge a passage against the words of earlier
// shared units, which says nothing about one learner. brief.coverage judges the
// same passages against what this learner knows, the words of the brief, and
// this unit's newWords when each resolves to a lexicon entry in its exact
// pointed form. Those newWords stay new for the intro and the minimum uses.
//
// The rule ids below are stable strings. A client, an audit row, and a test
// may match on them, so a rename needs a new id.
//
//	brief.audio_all             A Hebrew field has no audio clip for its exact pointed text (block).
//	brief.coverage              A story or listen_pick passage has too few known words (block). The unit's own newWords count when each is an exact pointed lexicon entry.
//	brief.expired               The brief is older than its time to live (block).
//	brief.gloss_out_of_brief    A word outside the brief and outside known or solid has no gloss (block).
//	brief.new_intro_before_test A new word is checked before any intro or exposure step (block).
//	brief.new_min_uses          A new word appears fewer times than the brief asks (block).
//	brief.not_found             The brief is missing, malformed, or another learner's (block).
//	brief.struggle_reuse        A struggle word is reused too few times, or not in its failing skill (block).
//	brief.text_too_long         A passage has more tokens than the tier allows (block).
//	brief.used                  The brief already built a unit (block).
//	unit.no_markup               A text field contains <, >, or javascript: (block).
//	unit.translation_distractors A unit with a sentence listen_pick has fewer than 4 translated lines (warn).
//	unit.translation_lang        A translation contains Hebrew or markup (block).
//	unit.translation_length      A translation is longer than 120 characters (warn).
//	unit.translation_missing     A listen_pick of two or more lexicon tokens has no translation (block on a new write, warning on an already-published revision).
//
// brief.coverage deliberately enforces the staged tier.min_known_share of the
// brief (0.60 under 100 known words, 0.90 for 100 to 299, 0.95 from 300), not
// the flat targets.coverage of 0.95 that the SPEC-020 text names. At 0.95 a
// beginner brief with 3 to 4 new words in a 20-token passage can never pass,
// and north-star section 7.3 stages the figure for that reason. BriefFacts
// carries the tier's number, and a value of zero or less means 0.95.
//
// # Dialogue (INIT-015/SPEC-026)
//
// A dialogue is a conversation between two people, one woman and one man. It
// is an activity with type "dialogue", no text, and a list of lines. Each line
// is {"speaker": "a" | "b", "text": "<pointed Hebrew>"}. Speaker a is the
// female voice and speaker b is the male voice, fixed for the whole unit. A
// story stays a single-voice narration.
//
// Every text rule judges a dialogue line by line, as if each line were its own
// activity, and an issue points at the line (/lessons/0/activities/2/lines/1/text).
// A line is pointed, mapped to the lexicon, covered by the brief, and held to
// the per-text length limit exactly as a story is. A dialogue is not a way
// around the brief. Audio is checked in the speaker's voice: a clip that
// exists only in the other voice does not count (audio.missing).
//
//	unit.dialogue_lines     A dialogue has fewer than 2 or more than 8 lines, or has text, or a non-dialogue activity has lines (block).
//	unit.dialogue_speakers  A line's speaker is not a or b, or one of the two never speaks (block).
//
// A lookup that can check a clip in a speaker's voice implements
// SpeakerLookup. One that cannot makes a unit with a dialogue fail with
// schema.lookup. It never answers from a clip in the wrong voice.
//
// # Line translation
//
// An activity may carry translation, the author's English for the whole
// text. A dialogue line may carry its own. A listen_pick whose text has two
// or more lexicon tokens must set it on a new write. Rechecking a revision
// that is already published warns instead. The translation is English plain
// text of at most 120 characters. A unit that uses a sentence listen_pick
// should carry at least four translated lines, counting activity translations
// and dialogue line translations, so the player can offer three other choices.
package units

import "context"

// Severity is Issue.Severity. block stops a publish; warn does not.
const (
	SeverityBlock = "block"
	SeverityWarn  = "warn"
)

// Lesson kinds and activity types stored in the unit document.
const (
	KindLesson     = "lesson"
	KindCheckpoint = "checkpoint"

	ActivityListenPick = "listen_pick"
	ActivityRecall     = "recall"
	ActivityProduce    = "produce"
	ActivityCloze      = "cloze"
	ActivityPattern    = "pattern"
	ActivityStory      = "story"
	ActivityShadow     = "shadow"
	ActivityDialogue   = "dialogue"
)

// The two speakers of a dialogue. A is the female voice and B is the male voice.
const (
	SpeakerA = "a"
	SpeakerB = "b"

	// MinDialogueLines and MaxDialogueLines bound one dialogue.
	MinDialogueLines = 2
	MaxDialogueLines = 8
)

// ValidSpeaker reports whether s is a or b. It is exact: "A" and " a" are not speakers.
func ValidSpeaker(s string) bool { return s == SpeakerA || s == SpeakerB }

// Issue is one validator finding. Pointer is a JSON pointer into the document.
type Issue struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Pointer  string `json:"pointer"`
	Msg      string `json:"msg"`
}

// Unit is the JSON document stored in units.doc.
// Position is the course order from the units.position column, which is
// 0 or greater. It is not stored in the document, so the JSON tag is "-".
// The caller sets it before Validate. Validate passes it to KnownLexemes
// as beforePosition. Zero is a real position: it means no earlier unit,
// which is correct for the first unit and also what an unset field does.
type Unit struct {
	Title     string   `json:"title"`
	Situation string   `json:"situation"`
	Concept   string   `json:"concept"`
	NewWords  []string `json:"newWords"`
	Lessons   []Lesson `json:"lessons"`
	Position  int      `json:"-"`
}

// Lesson is one lesson or checkpoint inside a unit.
type Lesson struct {
	Kind       string     `json:"kind"`
	Activities []Activity `json:"activities"`
}

// Activity is one screen. Type is one of the Activity* constants.
// Text is the Hebrew the activity speaks or asks about, when it has any.
// A dialogue has Lines instead of Text.
// Target, on a graded activity (recall, listen_pick, produce, cloze), is the
// token that activity grades. It is that token's exact text, compared after
// Normalize, the same way LexemeByText matches a token. A line of more than
// one Hebrew word must set it. A single-word line may leave it empty, and an
// empty value is the same as omitting it. A new write blocks a missing target.
// Rechecking a revision that is already published warns instead.
// Translation is the author's English for the whole text. A listen_pick whose
// text has two or more lexicon tokens must set it on a new write. Rechecking
// a revision that is already published warns instead. Every other activity
// may omit it.
type Activity struct {
	Type        string         `json:"type"`
	Text        string         `json:"text,omitempty"`
	Translation string         `json:"translation,omitempty" jsonschema:"Natural English translation of the whole text, written by the author. Required on a listen_pick whose text has two or more Hebrew words. Optional on every other type. English only, no markup, at most 120 characters."`
	Lines       []DialogueLine `json:"lines,omitempty"`
	Target      string         `json:"target,omitempty" jsonschema:"The exact pointed token this graded activity grades, in NFC, and one token of its text. That token's exact pointed spelling must be a lexicon entry. The unpointed spelling does not count. Required when the line has more than one token. A single-token line may omit it, and an empty value is the same as omitting it. Used by recall, listen_pick, produce, and cloze."`

	// speaker is set only on the one-line views of a dialogue that the validator
	// builds (see flatten). It is never read from or written to JSON.
	speaker string
}

// DialogueLine is one line of a dialogue. Speaker is a (female voice) or b
// (male voice). Text is pointed Hebrew in NFC. Translation is optional English
// for this line, used as a sentence-choice distractor.
type DialogueLine struct {
	Speaker     string `json:"speaker"`
	Text        string `json:"text"`
	Translation string `json:"translation,omitempty" jsonschema:"Natural English translation of this line. Optional; used as a sentence-choice distractor and for future line reveals."`
}

// Texts returns every Hebrew string the activity speaks, in order: Text for an
// ordinary activity, one entry per line for a dialogue.
func (a Activity) Texts() []string {
	if a.Type == ActivityDialogue {
		out := make([]string, 0, len(a.Lines))
		for _, l := range a.Lines {
			out = append(out, l.Text)
		}
		return out
	}
	return []string{a.Text}
}

// Lookup is the read surface Validate uses. Implementations live next to
// the lexicon and the audio store; the validator does not touch the database.
type Lookup interface {
	// KnownLexemes returns the pointed NFC text of every word introduced as new
	// in a published shared unit whose position is lower than beforePosition.
	// A true map value means that text was introduced earlier. Drafts and
	// learner-scoped units are left out. Validate passes Unit.Position.
	KnownLexemes(ctx context.Context, beforePosition int) (map[string]bool, error)
	// HasAudio reports whether a clip exists for textNFC.
	// Validate passes the activity text after normalize: Unicode NFC,
	// format characters removed, whitespace collapsed. Niqqud stays on
	// the key. The lookup is that exact string, not the consonant
	// skeleton and not the raw activity text.
	HasAudio(ctx context.Context, textNFC string) (bool, error)
	// LexemeByText returns the lexicon id for the exact pointed NFC text.
	// ok is false when the text is not in the lexicon.
	LexemeByText(ctx context.Context, textNFC string) (id int64, ok bool, err error)

	// Facts returns lexicon fields for the exact pointed NFC text.
	// The bool is false when the text is not in the lexicon.
	// Gender is "", "m", "f", or "both". Backend Dev approved this shape.
	Facts(ctx context.Context, textNFC string) (LexemeFacts, bool, error)
}

// SpeakerLookup is an optional extension of Lookup. HasAudioForSpeaker reports
// whether a clip for textNFC exists in the voice of speaker a or b. A dialogue
// line is checked with it, never with HasAudio, because HasAudio accepts a clip
// in any voice. An implementation that cannot tell the voices apart must not
// implement it.
type SpeakerLookup interface {
	HasAudioForSpeaker(ctx context.Context, textNFC, speaker string) (bool, error)
}

// LexemeFacts is the lexicon slice Facts returns. Backend Dev approved
// these fields. Zero values mean "not recorded". Gender is "", "m", "f",
// or "both". Plural is the pointed irregular plural; leave it empty when
// the lexicon does not store one. IrregularPlural is the lexicon's own
// irregular flag. Bare is the unpointed spelling. Gloss is the English gloss
// (lexemes.gloss_en); brief.gloss_out_of_brief reads it, and an empty value
// means none is stored (INIT-015/SPEC-020).
type LexemeFacts struct {
	POS             string
	Gender          string
	Plural          string
	IrregularPlural bool
	Bare            string
	Gloss           string
}
