package placement

import "time"

// Ratings stored on a placement answer.
// 0 is "I don't know this", sent before the flip. It is graded as Again.
// BandScore counts it as DontKnow, not as Again. The stop rule counts it
// as Again.
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
	// BandSize is the most real words one band deals.
	BandSize = 20
	// MinRatedForStop is how many ratings a band needs before the stop rule applies.
	MinRatedForStop = 10
	// StopAgainMin is the Again share that stops a band. "I don't know" counts
	// as Again. Exactly this share stops. Hard does not count.
	StopAgainMin = 0.60
	// SittingSize is how many answers one sitting takes before Next asks for a break.
	SittingSize = 20
	// MaxCards is the most cards one run rates.
	MaxCards = 100

	stopBand      = "band_stop"
	stopCards     = "cap_cards"
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

// NextResult is one step of the deck.
// Card is the word to show. Break means the current sitting has SittingSize
// answers: the run stays unfinished, and the client offers "Done for today"
// and "Keep going". Done means the run has been scored because a band
// stopped, the card cap was hit, or nothing dealable remains.
// Break and Done are not set together. A finished run returns an error.
type NextResult struct {
	Card  *Card
	Break bool
	Done  bool
}

// BandScore is one frequency band on a run.
// GoodPlusRate is (Good + Easy) / Rated. It is 0 when Rated is 0.
// DontKnow counts "I don't know this" answers. The Again field does not
// include them. The stop rule does.
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
// Level is the highest band whose block was fully rated without the Again
// rate reaching StopAgainMin, or −1 when none was.
// StopReason is band_stop, cap_cards, exhausted, or empty when the run is
// still in progress or the learner finished it first.
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
