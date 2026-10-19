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
// same passages against what this learner knows and the words of the brief.
//
// The rule ids below are stable strings. A client, an audit row, and a test
// may match on them, so a rename needs a new id.
//
//	brief.audio_all             A Hebrew field has no audio clip for its exact pointed text (block).
//	brief.coverage              A story or listen_pick passage has too few known words (block).
//	brief.expired               The brief is older than its time to live (block).
//	brief.gloss_out_of_brief    A word outside the brief and outside known or solid has no gloss (block).
//	brief.new_intro_before_test A new word is checked before any intro or exposure step (block).
//	brief.new_min_uses          A new word appears fewer times than the brief asks (block).
//	brief.not_found             The brief is missing, malformed, or another learner's (block).
//	brief.struggle_reuse        A struggle word is reused too few times, or not in its failing skill (block).
//	brief.text_too_long         A passage has more tokens than the tier allows (block).
//	brief.used                  The brief already built a unit (block).
//	unit.no_markup              A text field contains <, >, or javascript: (block).
//
// brief.coverage deliberately enforces the staged tier.min_known_share of the
// brief (0.60 under 100 known words, 0.90 for 100 to 299, 0.95 from 300), not
// the flat targets.coverage of 0.95 that the SPEC-020 text names. At 0.95 a
// beginner brief with 3 to 4 new words in a 20-token passage can never pass,
// and north-star section 7.3 stages the figure for that reason. BriefFacts
// carries the tier's number, and a value of zero or less means 0.95.
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
