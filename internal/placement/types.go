package placement

import "time"

// Ratings a learner can give a card. They match the FSRS grades, plus 0.
const (
	// DontKnow is "I don't know this", offered before the card is flipped.
	DontKnow = 0
	// Again means the word was missed.
	Again = 1
	// Hard means the word came back with difficulty.
	Hard = 2
	// Good means the word came back.
	Good = 3
	// Easy means the word came back easily.
	Easy = 4
)

const (
	// passAt is the Good-or-better rate at which a band is passed.
	// A rate of exactly 0.5 passes.
	passAt = 0.5
	// maxFalseAlarm is the highest Good-or-Easy rate on pseudo-words that
	// still seeds words. A rate above this marks the run unreliable.
	maxFalseAlarm = 0.25

	capCards = "cards"
	capTime  = "time"
	capBand  = "band"

	// bands is the count of frequency bands, 0 through 4.
	bands = 5
)

// Card is one word dealt to the learner.
// GlossEN and AudioURL travel with the card. The client shows them after
// the flip. Band and whether the word was invented are not on this type.
type Card struct {
	ID       string `json:"id"`
	TextNFC  string `json:"text_nfc"`
	GlossEN  string `json:"gloss_en"`
	AudioURL string `json:"audio_url"`
}

// Rated is the server's note after a rating is stored.
// IsPseudo is how the client learns the word was invented. The card itself
// does not carry that flag, so a pseudo-word is not given away before the rating.
type Rated struct {
	IsPseudo bool `json:"is_pseudo"`
}

// BandScore is one frequency band on a finished run.
// Shown counts real words that were rated, including "I don't know".
// GoodPlusRate is (Good + Easy) / Shown. It is 0 when Shown is 0.
type BandScore struct {
	Band         int     `json:"band"`
	Shown        int     `json:"shown"`
	Again        int     `json:"again"`
	Hard         int     `json:"hard"`
	Good         int     `json:"good"`
	Easy         int     `json:"easy"`
	DontKnow     int     `json:"dontknow"`
	GoodPlusRate float64 `json:"good_plus_rate"`
}

// Answer is one rating passed to Score.
// A later answer for the same item id replaces an earlier one.
// An item with no answer is not counted as shown.
type Answer struct {
	ItemID string `json:"item_id"`
	Rating int    `json:"rating"`
}

// ScoreItem is one row Score can see.
// Band is nil when the row is a pseudo-word. Pseudo is true for an invented word.
type ScoreItem struct {
	ID     string
	Band   *int
	Pseudo bool
}

// Result is one finished placement run.
// Level is the highest band whose Good-or-better rate is at least 0.5, or −1
// when none reached that. Reliable is false when the pseudo Good-or-Easy rate
// is above 0.25. An unreliable run still records a level, and it seeds nothing.
// Cap is cards, time, or band when a cap stopped the deck, and empty when the
// learner finished without one.
//
// EstimatedKnownWords sums, over bands 1–4, the Good-or-better rate times that
// band's width in the frequency list (500, 500, 1000, 2000), then adds the
// number of band 0 words rated Good or Easy.
// SuggestedStartPosition is the lowest units.position whose newWords are less
// than 80% covered by the learner's known set. It is one past the highest
// position when every unit is covered, and 0 when there are no units.
// Score leaves it at 0. Finish fills it from the units stored now.
type Result struct {
	RunID                  int64       `json:"run_id"`
	UserID                 int64       `json:"user_id"`
	StartedAt              time.Time   `json:"started_at"`
	FinishedAt             time.Time   `json:"finished_at"`
	FalseAlarmRate         float64     `json:"false_alarm_rate"`
	Reliable               bool        `json:"reliable"`
	Level                  int         `json:"level"`
	Bands                  []BandScore `json:"bands"`
	EstimatedKnownWords    int         `json:"estimated_known_words"`
	SuggestedStartPosition int         `json:"suggested_start_position"`
	Cap                    string      `json:"cap,omitempty"`
}

// storedResult is the JSON object kept in placement_runs.result.
// SuggestedStartPosition is not stored. It is computed from the current units.
// Cap is a column, not a field of this object.
type storedResult struct {
	Bands               []BandScore `json:"bands"`
	EstimatedKnownWords int         `json:"estimated_known_words"`
}
