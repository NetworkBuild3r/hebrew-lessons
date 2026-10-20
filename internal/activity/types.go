package activity

import (
	"errors"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

// Kind is one activity. The string is what attempts.activity stores.
type Kind string

// The activities of north-star section 6 that SPEC-015 owns.
const (
	ListenPick Kind = "listen_pick"
	ReadPick   Kind = "read_pick"
	WordMatch  Kind = "word_match"
	AudioMatch Kind = "audio_match"
	TypeIt     Kind = "type_it"
	Dictation  Kind = "dictation"
	// Flashcard is the self-graded fallback. It has no item and no Check: session.Record
	// grades it through evidence.Recorder.SelfGrade.
	Flashcard Kind = "flashcard"
)

// Spec says what an activity trains and how it is answered.
type Spec struct {
	Kind Kind
	// Skill is the skill every attempt of the activity trains. Empty for Flashcard.
	Skill string
	// Checked is false only for Flashcard.
	Checked bool
	// Typed activities are answered with text. The others are answered with picks.
	Typed bool
	// Multi activities grade several words at once. Their item names a set of words.
	Multi bool
}

var specs = []Spec{
	{Kind: ListenPick, Skill: fsrs.SkillListen, Checked: true},
	{Kind: ReadPick, Skill: fsrs.SkillRead, Checked: true},
	{Kind: WordMatch, Skill: fsrs.SkillRead, Checked: true, Multi: true},
	{Kind: AudioMatch, Skill: fsrs.SkillListen, Checked: true, Multi: true},
	{Kind: TypeIt, Skill: fsrs.SkillRecall, Checked: true, Typed: true},
	{Kind: Dictation, Skill: fsrs.SkillSpell, Checked: true, Typed: true},
	{Kind: Flashcard},
}

// Specs lists every activity, in table order.
func Specs() []Spec { return append([]Spec(nil), specs...) }

// SpecOf is the spec of a kind.
func SpecOf(k Kind) (Spec, bool) {
	for _, s := range specs {
		if s.Kind == k {
			return s, true
		}
	}
	return Spec{}, false
}

// Skills is every skill that has a checked activity, in the order ties are broken.
var Skills = []string{fsrs.SkillListen, fsrs.SkillRead, fsrs.SkillRecall, fsrs.SkillSpell}

// kindsFor lists the checked activities of a skill, in table order. The first is the
// everyday one and a multi-word activity follows it.
func kindsFor(skill string) []Kind {
	var out []Kind
	for _, s := range specs {
		if s.Checked && s.Skill == skill {
			out = append(out, s.Kind)
		}
	}
	return out
}

const (
	// OptionCount is how many options a pick item shows: one answer and three distractors.
	OptionCount = 4
	// WordMatchPairs is how many Hebrew-English pairs a word_match item shows.
	WordMatchPairs = 5
	// AudioMatchClips is how many clips and glosses an audio_match item shows.
	AudioMatchClips = 4

	// TokenTTL is how long an item can be answered after Build.
	TokenTTL = 30 * time.Minute
	// MaxPending is the most unanswered items the engine keeps. The oldest go first.
	MaxPending = 4096

	// MatchEvery is how often the mix swaps the everyday activity of a skill for its
	// match activity: the third visit through a skill, then the sixth, and so on.
	MatchEvery = 3

	// MaxPairDistance is the largest edit distance at which a typed answer's wrong
	// letters are logged as letter pairs. Further off, the answer is a different word,
	// not a mixed-up letter.
	MaxPairDistance = 2

	// MaxResponseBytes bounds a typed response before it is graded.
	MaxResponseBytes = 400
)

// Gate says which skills the learner can be asked through (north-star section 4.1).
// Listening is open from the start. Anything that shows or asks for written Hebrew waits
// for the letters check, so read, recall (typing) and spell wait for LettersPassed.
type Gate struct{ LettersPassed bool }

// OpenGate is a gate with everything open. No code decides the letters check yet, so
// it is what the session service uses unless it is given another gate.
func OpenGate() Gate { return Gate{LettersPassed: true} }

// Open reports whether the skill can be asked.
func (g Gate) Open(skill string) bool {
	if skill == fsrs.SkillListen {
		return true
	}
	return g.LettersPassed
}

// Prompt is what a single-word item shows. Only the fields the skill allows are set:
// a listening or dictation item has audio and no Hebrew text, a reading item has pointed
// Hebrew and no audio and no transliteration, and typing from English has the gloss only.
type Prompt struct {
	Hebrew  string `json:"hebrew,omitempty"`
	Gloss   string `json:"gloss,omitempty"`
	AudioID string `json:"audio_id,omitempty"`
}

// Cue is one entry of the left column of a match item: Hebrew text or a clip.
type Cue struct {
	Hebrew  string `json:"hebrew,omitempty"`
	AudioID string `json:"audio_id,omitempty"`
}

// Item is one question, as the client sees it. It carries no answer. The server keeps
// the answer under Token, and Check finds it there.
type Item struct {
	Token string `json:"token"`
	Kind  Kind   `json:"kind"`
	Skill string `json:"skill"`
	// Prompt is set on single-word items.
	Prompt Prompt `json:"prompt"`
	// Options are the glosses of a pick item, in the order shown.
	Options []string `json:"options,omitempty"`
	// Left and Right are the two columns of a match item. The learner pairs each Left
	// entry with one Right entry.
	Left  []Cue    `json:"left,omitempty"`
	Right []string `json:"right,omitempty"`
	// Typed items are answered with text.
	Typed     bool      `json:"typed,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Response is what the learner answered.
//
// Choice is the index into Options for a pick item. Pairs is, for a match item, the
// index into Right chosen for each Left entry, so it holds each index exactly once.
// Text is a typed answer. DontKnow gives up. Hinted says the learner used a hint or a
// replay before answering, which caps a right answer at Hard.
type Response struct {
	Choice   int    `json:"choice"`
	Pairs    []int  `json:"pairs,omitempty"`
	Text     string `json:"text,omitempty"`
	DontKnow bool   `json:"dont_know,omitempty"`
	Hinted   bool   `json:"hinted,omitempty"`
}

// Confused is one mix-up Check found: the asked word taken for another word, or a
// letter taken for another letter.
type Confused struct {
	Kind string `json:"kind"`
	Key  string `json:"key"`
	// Word is the asked word's pointed text.
	Word string `json:"word,omitempty"`
	// With is the pointed text of the word it was taken for, when there is one.
	With string `json:"with,omitempty"`
}

// PairResult is the outcome of one entry of a match item.
type PairResult struct {
	Left    int    `json:"left"`
	Chosen  int    `json:"chosen"`
	Correct bool   `json:"correct"`
	Text    string `json:"text,omitempty"`
	Gloss   string `json:"gloss,omitempty"`
	AudioID string `json:"audio_id,omitempty"`
}

// Result is what Check returns.
//
// Correct is the outcome of the item. For a match item it is true only when every pair is
// right. A typed answer that is one letter off is Correct with NearMiss set, and is rated
// Hard. Rating is the FSRS rating of the asked word. Feedback is false for a placement
// item, and then Answer, Text, Gloss, AudioID, Confused and Pairs are empty: placement
// shows nothing until its review screen.
type Result struct {
	Correct  bool
	NearMiss bool
	Rating   fsrs.Rating
	Feedback bool
	// Answer is the right response: the gloss for a pick or match item, the pointed
	// Hebrew for a typed one.
	Answer   string
	Text     string
	Gloss    string
	AudioID  string
	Confused []Confused
	Pairs    []PairResult
}

// Errors Check and Build return.
var (
	ErrNotChecked      = errors.New("activity: flashcards are self-graded and have no item")
	ErrUnknownActivity = errors.New("activity: unknown activity")
	ErrUnknownWord     = errors.New("activity: unknown word")
	ErrNoGloss         = errors.New("activity: the word has no gloss")
	ErrNoAudio         = errors.New("activity: the word has no audio")
	ErrNotEnoughWords  = errors.New("activity: not enough fair distractors")
	ErrBadSource       = errors.New("activity: unknown source")
	ErrTokenInvalid    = errors.New("activity: item token is not valid")
	ErrTokenExpired    = errors.New("activity: item token has expired")
	ErrTokenUnknown    = errors.New("activity: item is not pending")
	ErrTokenUser       = errors.New("activity: item belongs to another learner")
	ErrBadResponse     = errors.New("activity: response does not fit the item")
	ErrNotConfigured   = errors.New("activity: engine is not configured")
)
