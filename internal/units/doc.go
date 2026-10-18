// Package units is the unit document and its validator.
//
// Backend owns these structs and the Validate signature.
// MCP Builder owns validate_*.go and rules.go, which fill in the rules.
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
)

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
// Further fields land with the rules.
type Activity struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Lookup is the read surface Validate uses. Implementations live next to
// the lexicon and the audio store; the validator does not touch the database.
type Lookup interface {
	// KnownLexemes returns the pointed NFC text of every word introduced as new
	// in a unit whose position is lower than beforePosition. A true map value
	// means that text was introduced earlier. Validation runs at authoring
	// time and the course is global, so this is not scoped to a learner.
	// Validate passes Unit.Position.
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

// LexemeFacts is the lexicon slice Facts returns. Backend Dev approved
// these fields. Zero values mean "not recorded". Gender is "", "m", "f",
// or "both". Plural is the pointed irregular plural; leave it empty when
// the lexicon does not store one. IrregularPlural is the lexicon's own
// irregular flag. Bare is the unpointed spelling.
type LexemeFacts struct {
	POS             string
	Gender          string
	Plural          string
	IrregularPlural bool
	Bare            string
}
