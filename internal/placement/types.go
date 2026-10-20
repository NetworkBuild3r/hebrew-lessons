package placement

import (
	"errors"
	"time"
)

// The v3 tuning numbers (north-star section 5.2, INIT-015/SPEC-007). TestV3TuningNumbers pins them.
// The v2 numbers (BandMax, StopMinRated, and the rest of legacy_types.go) belong to the v2 deck,
// which stays until the placement screens move to Engine.
const (
	// BandCount is how many bands the test has. Bands are numbered 1 to BandCount in this package:
	// band N is the deck's band N−1, so band 1 holds the tutor words that the first lessons teach.
	BandCount = 5
	// FirstSize is how many items a skill is asked in a band before the first decision.
	FirstSize = 6
	// ExtraSize is how many more items a mixed first result earns.
	ExtraSize = 6
	// MoveUpFirst is how many right of FirstSize move the skill up a band.
	MoveUpFirst = 5
	// RetryFirst is how many right of FirstSize earn ExtraSize more items. Fewer than this stops the skill.
	RetryFirst = 3
	// MoveUpTwelve is how many right of FirstSize+ExtraSize move the skill up a band.
	MoveUpTwelve = 8
	// OptionCount is the options of a pick item, the right one among them. "I don't know" is another choice.
	OptionCount = 4
	// GuessDivisor corrects for guessing: a wrong answer takes 1/GuessDivisor of a right one, which is
	// what a blind pick among OptionCount options is worth (1 right in OptionCount, OptionCount−1 wrong).
	GuessDivisor = OptionCount - 1
	// LevelShare is the known share a band needs to count toward the level.
	LevelShare = 0.5
	// LettersAsked is the Part 0 items.
	LettersAsked = 6
	// LettersPass is how many right of LettersAsked let reading be tested.
	LettersPass = 5
	// TypingMax is the most words the optional typing part asks. It keeps the part near two minutes.
	TypingMax = 8
	// RecheckAfter is how long after the last placement or re-check a check-in becomes due.
	RecheckAfter = 28 * 24 * time.Hour
	// RecheckSoftUntil is the end of the soft window. Past it a check-in is still due, and it is late.
	RecheckSoftUntil = 42 * 24 * time.Hour
)

// Answer outcomes, as placement_items.outcome stores them.
const (
	OutcomeRight    = "right"
	OutcomeWrong    = "wrong"
	OutcomeDontKnow = "dont_know"
)

// Skills a placement item exercises. They are the attempts.skill values.
const (
	SkillListen = "listen"
	SkillRead   = "read"
	SkillRecall = "recall"
)

// Part is a part of the test.
type Part string

// The parts, in the order they are taken.
const (
	PartLetters   Part = "letters"
	PartListening Part = "listening"
	PartReading   Part = "reading"
	PartTyping    Part = "typing"
)

// Item kinds.
const (
	KindPick = "pick"
	KindType = "type"
)

// Errors the engine returns. The handler maps them to a status; none of them reveals an answer.
var (
	// ErrNotYourRun is returned for an unknown run and for another learner's run. The two are the same
	// error on purpose, so a learner cannot tell whether a run id exists.
	ErrNotYourRun = errors.New("placement: run not found")
	// ErrWrongItem means the item id is not the item the server dealt to this run and left open.
	ErrWrongItem = errors.New("placement: that is not the open item")
	// ErrBadAnswer means the answer does not fit the item: no choice, two choices, an option the item
	// did not offer, or empty typed text.
	ErrBadAnswer = errors.New("placement: the answer does not fit the item")
	// ErrFinished means the run is already finished.
	ErrFinished = errors.New("placement: the run is finished")
	// ErrNotReady means the call does not fit the run's state, for example finishing before the parts are done.
	ErrNotReady = errors.New("placement: the run is not at that step")
	// ErrNoAudio means a clip the test needs has no audio_assets row.
	ErrNoAudio = errors.New("placement: a clip the test needs has no audio")
	// ErrDeckShort means a band has too few words to deal.
	ErrDeckShort = errors.New("placement: the deck is too small for the test")
	// ErrNoPlacement means a re-check was asked for a learner with no finished v3 placement.
	ErrNoPlacement = errors.New("placement: the learner has no finished placement")
)

// Option is one choice of a pick item. ID is the position, "o0" to "o3", and says nothing about
// which option is right. Text is an English gloss, or a letter with its vowel in Part 0.
type Option struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Item is what the client sees of one dealt item. It carries no band, no answer, and no sign of
// how the earlier items went.
//
// ID is the item's token and the only value the answer is accepted for. Listening and letters
// items have AudioURL and no Hebrew text. A reading item has Prompt, the pointed Hebrew, and no
// audio and no transliteration. A typing item has Prompt, the English meaning, and Kind "type".
// NewPart is true on the first item of a part, so the screen can offer a break before it.
type Item struct {
	ID       string   `json:"id"`
	Part     Part     `json:"part"`
	Kind     string   `json:"kind"`
	Seq      int      `json:"seq"`
	NewPart  bool     `json:"new_part,omitempty"`
	AudioURL string   `json:"audio_url,omitempty"`
	Prompt   string   `json:"prompt,omitempty"`
	Options  []Option `json:"options,omitempty"`
}

// AnswerIn is the learner's answer to one item. Exactly one of Option, Typed, and DontKnow is
// set. Option is an Option.ID. Typed is for a typing item.
type AnswerIn struct {
	Option   string `json:"option,omitempty"`
	Typed    string `json:"typed,omitempty"`
	DontKnow bool   `json:"dont_know,omitempty"`
}

// Step is the whole reply to an answer, and to a request for the next item. It never says whether
// an answer was right.
//
// Item is the next item. TypingOffer is true after the reading part, when the learner may take the
// optional typing part (StartTyping) or skip it (Finish). Done is true, with Result set, once the
// run is finished. At most one of Item, TypingOffer, and Done is set.
type Step struct {
	Item        *Item      `json:"item,omitempty"`
	TypingOffer bool       `json:"typing_offer,omitempty"`
	Done        bool       `json:"done,omitempty"`
	Result      *RunResult `json:"result,omitempty"`
}

// BandResult is one band of one skill. Share is the known share, corrected for guessing.
type BandResult struct {
	Band     int     `json:"band"`
	Asked    int     `json:"asked"`
	Right    int     `json:"right"`
	Wrong    int     `json:"wrong"`
	DontKnow int     `json:"dont_know"`
	Share    float64 `json:"share"`
}

// SkillResult is the outcome for listening or reading.
// Tested is false when the skill was not asked: reading when the letters check failed. Level is the
// highest band with a known share of at least LevelShare, 0 when none. Estimate is the estimated
// words known, rounded. Bands lists the bands that were asked, lowest first.
type SkillResult struct {
	Skill    string       `json:"skill"`
	Tested   bool         `json:"tested"`
	Level    int          `json:"level"`
	Estimate int          `json:"estimate"`
	Bands    []BandResult `json:"bands"`
}

// TypingResult counts the optional typing part.
type TypingResult struct {
	Taken bool `json:"taken"`
	Asked int  `json:"asked"`
	Right int  `json:"right"`
}

// Miss is one word the learner got wrong, or said "I don't know" to, for the review at the end.
// Picked is the option picked, or the text typed, and is empty for "I don't know". RightGloss is
// the right meaning, Pointed the pointed form, and AudioID the clip (empty when there is none).
type Miss struct {
	LexemeID   int64  `json:"lexeme_id"`
	Skill      string `json:"skill"`
	Picked     string `json:"picked,omitempty"`
	RightGloss string `json:"right_gloss"`
	Pointed    string `json:"pointed"`
	AudioID    string `json:"audio_id,omitempty"`
}

// RunResult is the result of a finished run.
//
// ReadingReady is false when fewer than LettersPass of the letters were right, and reading was then
// not tested. StartUnit is the course position to open first, the same value as
// db.Placement.SuggestedStartPosition. For a re-check only the re-checked skill is filled, and
// Recheck says what moved.
type RunResult struct {
	RunID        int64         `json:"run_id"`
	UserID       int64         `json:"user_id"`
	Kind         string        `json:"kind"`
	StartedAt    time.Time     `json:"started_at"`
	FinishedAt   time.Time     `json:"finished_at"`
	LettersAsked int           `json:"letters_asked"`
	LettersRight int           `json:"letters_right"`
	ReadingReady bool          `json:"reading_ready"`
	Listening    SkillResult   `json:"listening"`
	Reading      SkillResult   `json:"reading"`
	Typing       TypingResult  `json:"typing"`
	StartUnit    int           `json:"start_unit"`
	Misses       []Miss        `json:"misses"`
	Recheck      *RecheckMoved `json:"recheck,omitempty"`
}

// RecheckMoved is what a re-check found for the skill it tested.
// Moved is up, same, or down. Band is the band that was re-asked.
type RecheckMoved struct {
	Skill         string `json:"skill"`
	Band          int    `json:"band"`
	PreviousLevel int    `json:"previous_level"`
	Level         int    `json:"level"`
	Moved         string `json:"moved"`
}

// RecheckStatus says whether a check-in is due. DueAt is LastAt plus RecheckAfter.
// Late is true from RecheckSoftUntil on. HasPlacement is false, and Due false, when the learner
// has no finished v3 placement.
type RecheckStatus struct {
	HasPlacement bool      `json:"has_placement"`
	Due          bool      `json:"due"`
	Late         bool      `json:"late"`
	LastAt       time.Time `json:"last_at"`
	DueAt        time.Time `json:"due_at"`
}
