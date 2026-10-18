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
type Unit struct {
	Title     string   `json:"title"`
	Situation string   `json:"situation"`
	Concept   string   `json:"concept"`
	NewWords  []string `json:"newWords"`
	Lessons   []Lesson `json:"lessons"`
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
	// KnownLexemes returns the pointed NFC texts this learner already knows.
	KnownLexemes(ctx context.Context, userID int64) ([]string, error)
	// HasAudio reports whether a clip exists for the exact pointed NFC text.
	HasAudio(ctx context.Context, textNFC string) (bool, error)
	// LexemeByText returns the lexicon id for the exact pointed NFC text.
	// ok is false when the text is not in the lexicon.
	LexemeByText(ctx context.Context, textNFC string) (id int64, ok bool, err error)

	// Facts returns lexicon fields for the exact pointed NFC text.
	// ok is false when the text is not in the lexicon.
	//
	// PROPOSED for Backend Dev. Lookup did not expose gender or plural.
	// noun.gender and noun.plural need them, so this is the smallest
	// addition: one method and the struct below. Translit and root are not
	// read by the validator; player guidance in rules.go names them for the
	// MCP rules resource.
	Facts(ctx context.Context, textNFC string) (LexemeFacts, bool, error)
}

// LexemeFacts is the lexicon slice Facts returns.
//
// PROPOSED for Backend Dev, with Facts. Zero values mean "not recorded".
// Gender is "m", "f", or "both". Plural is the pointed irregular plural;
// leave it empty when the lexicon does not store one. IrregularPlural is
// the lexicon's own irregular flag. Bare is the unpointed spelling.
type LexemeFacts struct {
	POS             string
	Gender          string
	Plural          string
	IrregularPlural bool
	Bare            string
}
