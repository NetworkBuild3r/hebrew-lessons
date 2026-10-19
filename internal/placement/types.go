package placement

import "time"

// Pass thresholds. Tests pin these to the team-lead ruling.
const (
	// passAt is the corrected score at which a band passes.
	passAt = 0.8
	// maxFalseAlarm is the highest pseudo yes-rate that still seeds words.
	// A rate above this marks the run unreliable.
	maxFalseAlarm = 0.25
)

// Item is one word shown to the learner.
// The frequency band and whether the word was invented are not on this type.
type Item struct {
	ID      string `json:"id"`
	TextNFC string `json:"text_nfc"`
}

// BandScore is one frequency band on a finished run.
// H is the yes-rate on that band's real words.
// F is the yes-rate on the pseudo-words, the same value on every band.
// Corrected is (H − F) / (1 − F). When F is 1, Corrected is 0 and Passed is false.
type BandScore struct {
	Band      int     `json:"band"`
	H         float64 `json:"h"`
	F         float64 `json:"f"`
	Corrected float64 `json:"corrected"`
	Passed    bool    `json:"passed"`
}

// Answer is one yes or no passed to Score.
// A later answer for the same item id replaces an earlier one.
type Answer struct {
	ItemID string `json:"item_id"`
	Yes    bool   `json:"yes"`
}

// ScoreItem is one row Score can see.
// Band is nil when the row is a pseudo-word. Pseudo is true for an invented word.
type ScoreItem struct {
	ID     string
	Band   *int
	Pseudo bool
}

// Result is one finished placement run.
// Level is the highest passed band, or −1 when none passed.
// Reliable is false when the pseudo yes-rate is above 0.25. An unreliable
// run still records a level, and it does not seed lexemes.
//
// EstimatedKnownWords sums, over bands 1–4, clamp(corrected, 0, 1) times
// that band's width in the frequency list (500, 500, 1000, 2000), then adds
// the number of band 0 words answered yes.
// SuggestedStartPosition is the lowest units.position whose newWords are
// less than 80% covered by the learner's known set. It is one past the
// highest position when every unit is covered, and 0 when there are no units.
// Score leaves it at 0. Finish and Latest fill it from the units stored now.
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
}

// storedResult is the JSON object kept in placement_runs.result.
// SuggestedStartPosition is not stored. It is computed from the current units.
type storedResult struct {
	Bands               []BandScore `json:"bands"`
	EstimatedKnownWords int         `json:"estimated_known_words"`
}
