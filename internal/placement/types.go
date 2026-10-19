package placement

import "time"

// Ratings stored on a placement answer.
// 0 is "I don't know this", sent before the flip. It is graded as Again
// and counted as DontKnow, not as Again.
// 1 is Again, 2 Hard, 3 Good, 4 Easy.
const (
	DontKnow = 0
	Again    = 1
	Hard     = 2
	Good     = 3
	Easy     = 4
)

const (
	// bands is the count of frequency bands, 0 through 4.
	bands = 5
	// BlockSize is how many real words one band deals before the stop rule.
	BlockSize = 20
	// StopGoodPlusMin is the Good-or-Easy share a finished block must reach.
	// Below this, the run stops and higher bands are not dealt. Exactly this
	// share does not stop.
	StopGoodPlusMin = 0.50
	// MaxCards is the most cards one run rates.
	MaxCards = 100
	// MaxDuration is the longest a run may deal cards, measured with clock.Clock.
	MaxDuration = 10 * time.Minute

	stopBand      = "band_stop"
	stopCards     = "cap_cards"
	stopTime      = "cap_time"
	stopExhausted = "exhausted"
)

// Card is one word dealt to the learner.
// TextNFC and AudioURL are the front. GlossEN, Translit, and Gender are the
// back. They travel together so the client can flip without another request.
// AudioURL is /audio/<id>.mp3 when a clip exists for the exact pointed text,
// and empty otherwise.
type Card struct {
	ID       string `json:"id"`
	TextNFC  string `json:"text_nfc"`
	AudioURL string `json:"audio_url"`
	GlossEN  string `json:"gloss_en"`
	Translit string `json:"translit"`
	Gender   string `json:"gender"`
}

// BandScore is one frequency band on a run.
// GoodPlusRate is (Good + Easy) / Rated. It is 0 when Rated is 0.
// DontKnow counts "I don't know this" answers. Those are not Again.
type BandScore struct {
	Band         int     `json:"band"`
	Rated        int     `json:"rated"`
	Again        int     `json:"again"`
	Hard         int     `json:"hard"`
	Good         int     `json:"good"`
	Easy         int     `json:"easy"`
	DontKnow     int     `json:"dont_know"`
	GoodPlusRate float64 `json:"good_plus_rate"`
}

// BandTally is one band's ratings passed to Score.
// Dealable is how many words in the band can still be shown, including the
// ones already rated. A word that already had a card is not dealable.
type BandTally struct {
	Ratings  []int
	Dealable int
}

// Result is one placement run.
// Level is the highest band whose block was fully rated at or above
// StopGoodPlusMin, or −1 when none was.
// StopReason is band_stop, cap_cards, cap_time, exhausted, or empty when
// the run is still in progress or the learner finished it first.
//
// EstimatedKnownWords sums, over bands 1–4, GoodPlusRate times that band's
// width in the frequency list (500, 500, 1000, 2000), then adds the number
// of band 0 words rated Good or Easy. A band with no ratings adds nothing.
// SuggestedStartPosition is the lowest units.position whose newWords are
// less than 80% covered by the learner's known set. It is one past the
// highest position when every unit is covered, and 0 when there are no units.
// Score leaves it at 0. Finish fills it from the units stored now.
type Result struct {
	RunID                  int64       `json:"run_id"`
	UserID                 int64       `json:"user_id"`
	StartedAt              time.Time   `json:"started_at"`
	FinishedAt             time.Time   `json:"finished_at"`
	Level                  int         `json:"level"`
	CardsRated             int         `json:"cards_rated"`
	StopReason             string      `json:"stop_reason,omitempty"`
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
