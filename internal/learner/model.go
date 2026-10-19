package learner

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
)

const (
	// DefaultPageSize is how many words List returns when Filter.Limit is not set.
	DefaultPageSize = 50
	// MaxPageSize is the most words a page holds. A bigger Limit is cut to it.
	MaxPageSize = 200

	// RetentionWindowDays is how far back Summary reads due reviews for retention.
	RetentionWindowDays = 30

	// MissWindowDays is how far back review_log Agains and confusion keys count in a
	// WordState's Review stats. It is the 7-day window get_context has always used.
	MissWindowDays = 7
)

// ErrBadCursor means a cursor was not one this model issued.
var ErrBadCursor = errors.New("learner: bad cursor")

// ErrUnknownWord means the lexeme does not exist.
var ErrUnknownWord = errors.New("learner: unknown word")

// SkillState is one skill of one word.
type SkillState struct {
	Skill string
	State State
	// Attempts is every outcome of the skill. Window outcomes were read for the accuracy.
	Attempts       int
	Window         int
	Accuracy       float64 // Share of the newest AccuracyWindow outcomes that were right. 0 when none.
	AgainRecent    int     // Again among the newest StruggleAgainWindow outcomes.
	LastSeen       time.Time
	HasCard        bool
	Stability      float64 // Card stability in days. 0 with no card.
	Retrievability float64 // Chance of recall now. 0 with no card.
	Successes      int     // Successful reviews of the card.
	WasLearned     bool    // The card has passed its learning stage. See WasLearned.
}

// ReviewStats is the review_log side of a word, summed over its cards. get_context shows it.
type ReviewStats struct {
	Reviews       int
	Lapses        int // Sum of cards.lapses.
	Agains        int // Reviews rated Again.
	AgainRate     float64
	Misses7d      int // Agains in the last MissWindowDays.
	LastReview    time.Time
	ConfusionKeys []string // Keys confused in the last MissWindowDays, sorted.
}

// WordState is everything the model says about one word.
type WordState struct {
	LexemeID int64
	Text     string // Pointed NFC. Filled for the words List and Word return.
	Gloss    string
	// Band is the placement frequency band, 0 to 4, or -1 when the word is not in the placement deck.
	Band int

	// State is the overall state: the weakest skill that has evidence, or seen or unseen with none.
	State State
	// Skills has one entry per skill, in the order of Skills.
	Skills []SkillState
	Cause  Cause

	Attempts int
	Accuracy float64 // Right share of the newest outcomes across skills. 0 when none.
	LastSeen time.Time
	// Stability and Retrievability are those of the weakest skill that has a card.
	Stability      float64
	Retrievability float64
	Exposures      int

	// BandStrugglers is how many words of this word's band are struggling or regressed and
	// were seen in the last BandSlipDays, this one included. 0 when the band is unknown.
	BandStrugglers int

	Review     ReviewStats
	Confusions []Confusion // In the last ConfusionWindowDays.
}

func (w WordState) stateOf(skill string) State {
	for _, s := range w.Skills {
		if s.Skill == skill {
			return s.State
		}
	}
	return Unseen
}

// Filter narrows List. The zero Filter lists every word.
type Filter struct {
	// State keeps words in this state. With Skill set it is the state of that skill,
	// otherwise the overall state.
	State State
	// Skill keeps the state filter to one skill. One of Skills, or empty.
	Skill string
	// Band keeps words of one placement band, 0 to 4. Nil keeps every band.
	Band *int
	// Limit is the page size. 0 is DefaultPageSize, and more than MaxPageSize is cut to it.
	Limit int
	// After continues from the cursor a previous page returned. Empty starts at the first.
	After Cursor
}

// Cursor says where the next page of List begins. It is opaque. An empty Cursor
// returned by List means there is no next page.
type Cursor string

func encodeCursor(id int64) Cursor {
	return Cursor(base64.RawURLEncoding.EncodeToString([]byte("w1:" + strconv.FormatInt(id, 10))))
}

func decodeCursor(c Cursor) (int64, error) {
	if c == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(string(c))
	if err != nil {
		return 0, ErrBadCursor
	}
	rest, ok := strings.CutPrefix(string(raw), "w1:")
	if !ok {
		return 0, ErrBadCursor
	}
	id, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || id < 0 {
		return 0, ErrBadCursor
	}
	return id, nil
}

// SkillSummary is the counts of one skill.
type SkillSummary struct {
	Skill string
	// Counts has one entry per state, over every word in the lexicon.
	Counts map[State]int
	// EstimatedKnown is the estimated number of words known in this skill. The placement
	// estimate stands for words with no evidence, and the model counts (known plus solid)
	// replace it for words with evidence. See estimateKnown.
	EstimatedKnown float64
	// Retention is the share of due reviews in the last RetentionWindowDays not rated Again.
	// Nil when there were none.
	Retention  *float64
	DueReviews int
}

// Summary is the learner's picture across all words.
type Summary struct {
	Skills []SkillSummary // In the order of Skills.
	// Words counts the overall states over every word in the lexicon.
	Words map[State]int
	// Retention and DueReviews are over all skills.
	Retention  *float64
	DueReviews int
}

// BandSource says which placement band a lexeme is in.
type BandSource interface {
	// LexemeBands returns the band, 0 to 4, of every lexeme it knows. A lexeme it does not
	// know is left out. It must not depend on the learner.
	LexemeBands(ctx context.Context) (map[int64]int, error)
}

// Option changes how New builds a Model.
type Option func(*Model)

// WithBands sets where bands come from. The default is the placement deck.
func WithBands(b BandSource) Option {
	return func(m *Model) { m.bands = b }
}

// Model reads a learner's evidence and applies the rules. It holds no learner data.
type Model struct {
	store *db.Store
	clock clock.Clock
	bands BandSource
}

// New builds a model. A nil clk uses the clock the store was opened with.
func New(store *db.Store, clk clock.Clock, opts ...Option) *Model {
	if clk == nil && store != nil {
		clk = store.Clock()
	}
	m := &Model{store: store, clock: clk}
	m.bands = placementBands{store: store}
	for _, o := range opts {
		o(m)
	}
	return m
}

func (m *Model) ready(userID int64) error {
	if m == nil || m.store == nil {
		return errors.New("learner: model is not configured")
	}
	if m.clock == nil {
		return errors.New("learner: clock is not configured")
	}
	if userID <= 0 {
		return errors.New("learner: no learner")
	}
	return nil
}

// Word returns the state of one word for the learner. A word with no evidence is unseen.
func (m *Model) Word(ctx context.Context, userID, lexemeID int64) (WordState, error) {
	if err := m.ready(userID); err != nil {
		return WordState{}, err
	}
	if lexemeID <= 0 {
		return WordState{}, ErrUnknownWord
	}
	texts, err := m.store.LexemeTexts(ctx, []int64{lexemeID})
	if err != nil {
		return WordState{}, err
	}
	text, ok := texts[lexemeID]
	if !ok {
		return WordState{}, ErrUnknownWord
	}
	now := m.clock.Now()
	ev, err := m.store.LearnerEvidence(ctx, userID, m.query(now, lexemeID))
	if err != nil {
		return WordState{}, err
	}
	bands, err := m.bands.LexemeBands(ctx)
	if err != nil {
		return WordState{}, err
	}
	words := m.evaluate(ev, bands, now)
	w, ok := words[lexemeID]
	if !ok {
		w = unseenWord(lexemeID, bandOf(bands, lexemeID))
	}
	if weak(w.State) && w.Band >= 0 {
		// The band context needs every word of the band, so read them all.
		all, err := m.store.LearnerEvidence(ctx, userID, m.query(now, 0))
		if err != nil {
			return WordState{}, err
		}
		allWords := m.evaluate(all, bands, now)
		w.BandStrugglers = allWords[lexemeID].BandStrugglers
		w.Cause = Diagnose(w, w.Confusions)
	}
	w.Text, w.Gloss = text.TextNFC, text.Gloss
	return w, nil
}

// List returns the learner's words that match the filter, by lexeme id, one page at a time.
// The second result is the cursor of the next page, or empty at the end.
// Every word that was ever shown or asked has its state. Words with no evidence are
// unseen, and they are listed too when the filter allows.
func (m *Model) List(ctx context.Context, userID int64, f Filter) ([]WordState, Cursor, error) {
	if err := m.ready(userID); err != nil {
		return nil, "", err
	}
	if f.State != "" && !f.State.Valid() {
		return nil, "", fmt.Errorf("learner: unknown state %q", f.State)
	}
	if f.Skill != "" && !slices.Contains(Skills, f.Skill) {
		return nil, "", fmt.Errorf("learner: unknown skill %q", f.Skill)
	}
	if f.Band != nil && (*f.Band < 0 || *f.Band > 4) {
		return nil, "", fmt.Errorf("learner: band %d is outside 0..4", *f.Band)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	limit = min(limit, MaxPageSize)
	after, err := decodeCursor(f.After)
	if err != nil {
		return nil, "", err
	}

	now := m.clock.Now()
	ids, err := m.store.LexemeIDs(ctx)
	if err != nil {
		return nil, "", err
	}
	ev, err := m.store.LearnerEvidence(ctx, userID, m.query(now, 0))
	if err != nil {
		return nil, "", err
	}
	bands, err := m.bands.LexemeBands(ctx)
	if err != nil {
		return nil, "", err
	}
	words := m.evaluate(ev, bands, now)

	var page []WordState
	var next Cursor
	start, _ := slices.BinarySearch(ids, after+1)
	for _, id := range ids[start:] {
		w, ok := words[id]
		if !ok {
			w = unseenWord(id, bandOf(bands, id))
		}
		if !f.matches(w) {
			continue
		}
		if len(page) == limit {
			next = encodeCursor(page[len(page)-1].LexemeID)
			break
		}
		page = append(page, w)
	}

	if len(page) > 0 {
		pageIDs := make([]int64, len(page))
		for i, w := range page {
			pageIDs[i] = w.LexemeID
		}
		texts, err := m.store.LexemeTexts(ctx, pageIDs)
		if err != nil {
			return nil, "", err
		}
		for i := range page {
			t := texts[page[i].LexemeID]
			page[i].Text, page[i].Gloss = t.TextNFC, t.Gloss
		}
	}
	return page, next, nil
}

func (f Filter) matches(w WordState) bool {
	if f.Band != nil && w.Band != *f.Band {
		return false
	}
	if f.State == "" {
		return true
	}
	if f.Skill != "" {
		return w.stateOf(f.Skill) == f.State
	}
	return w.State == f.State
}

// Summary counts the learner's words per skill and state, estimates the words known per
// skill, and reads retention over the last RetentionWindowDays.
func (m *Model) Summary(ctx context.Context, userID int64) (Summary, error) {
	if err := m.ready(userID); err != nil {
		return Summary{}, err
	}
	now := m.clock.Now()
	ids, err := m.store.LexemeIDs(ctx)
	if err != nil {
		return Summary{}, err
	}
	ev, err := m.store.LearnerEvidence(ctx, userID, m.query(now, 0))
	if err != nil {
		return Summary{}, err
	}
	bands, err := m.bands.LexemeBands(ctx)
	if err != nil {
		return Summary{}, err
	}
	words := m.evaluate(ev, bands, now)
	placed, havePlacement, err := m.store.PlacementBands(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	retention, err := m.store.DueRetention(ctx, userID, now.AddDate(0, 0, -RetentionWindowDays))
	if err != nil {
		return Summary{}, err
	}

	out := Summary{Words: map[State]int{}}
	for _, s := range States {
		out.Words[s] = 0
	}
	// Words with no evidence at all are unseen overall and in every skill.
	unseen := len(ids)
	for _, id := range ids {
		if _, ok := words[id]; ok {
			unseen--
		}
	}
	out.Words[Unseen] += unseen

	var due, right int
	for _, skill := range Skills {
		ss := SkillSummary{Skill: skill, Counts: map[State]int{}}
		for _, s := range States {
			ss.Counts[s] = 0
		}
		ss.Counts[Unseen] += unseen
		evidenced := [5]int{}
		known := 0
		for _, id := range ids {
			w, ok := words[id]
			if !ok {
				continue
			}
			sk := w.skill(skill)
			ss.Counts[sk.State]++
			if sk.State == Known || sk.State == Solid {
				known++
			}
			if sk.hasEvidence() && w.Band >= 0 {
				evidenced[w.Band]++
			}
		}
		ss.EstimatedKnown = estimateKnown(placed, havePlacement, evidenced, known)
		for _, r := range retention {
			if r.Skill == skill {
				ss.DueReviews = r.Due
				ss.Retention = ratio(r.Right, r.Due)
				due += r.Due
				right += r.Right
			}
		}
		out.Skills = append(out.Skills, ss)
	}
	for _, id := range ids {
		if w, ok := words[id]; ok {
			out.Words[w.State]++
		}
	}
	out.DueReviews = due
	out.Retention = ratio(right, due)
	return out, nil
}

// ConfusionPairs returns the learner's confusions over the last window, grouped by pair,
// most frequent first.
func (m *Model) ConfusionPairs(ctx context.Context, userID int64, window time.Duration) ([]db.ConfusionPair, error) {
	if err := m.ready(userID); err != nil {
		return nil, err
	}
	return m.store.RecentConfusionPairs(ctx, userID, m.clock.Now().Add(-window))
}

func ratio(right, due int) *float64 {
	if due == 0 {
		return nil
	}
	r := float64(right) / float64(due)
	return &r
}

// bandRankWidth is how many ranks of the frequency list each placement band covers. Band 0
// has no width. internal/placement keeps the same widths privately, and a test checks they agree.
var bandRankWidth = [5]float64{0, 500, 500, 1000, 2000}

// estimateKnown is the estimated words known in one skill.
//
// For each band the placement run gave a rate of words rated Good or Easy. That rate stands
// for the band's words the learner has no evidence on in this skill: the rate times the band's
// width, less the words with evidence. Band 0 has no width, so its placement count (Good plus
// Easy) stands, less the band 0 words with evidence. Words with evidence are then counted by
// the model: every word known or solid in this skill adds one, banded or not.
//
//	estimate = sum over bands of rate * max(width - evidenced, 0) + known and solid words
//
// With no finished placement the placement part is 0.
func estimateKnown(placed []db.BandScore, have bool, evidenced [5]int, known int) float64 {
	total := float64(known)
	if !have {
		return total
	}
	for _, b := range placed {
		if b.Band < 0 || b.Band > 4 {
			continue
		}
		if b.Band == 0 {
			total += max(float64(b.Good+b.Easy)-float64(evidenced[0]), 0)
			continue
		}
		total += b.GoodPlusRate * max(bandRankWidth[b.Band]-float64(evidenced[b.Band]), 0)
	}
	return total
}

func (s SkillState) hasEvidence() bool { return s.Attempts > 0 || s.HasCard }

func (w WordState) skill(name string) SkillState {
	for _, s := range w.Skills {
		if s.Skill == name {
			return s
		}
	}
	return SkillState{Skill: name, State: Unseen}
}

func weak(s State) bool { return s == Struggling || s == Regressed }

func (m *Model) query(now time.Time, lexemeID int64) db.LearnerQuery {
	return db.LearnerQuery{
		LexemeID:       lexemeID,
		AgainWindow:    StruggleAgainWindow,
		AccuracyWindow: AccuracyWindow,
		RecentWindow:   KnownRecentRight,
		MissSince:      now.AddDate(0, 0, -MissWindowDays),
		ConfusionSince: now.AddDate(0, 0, -ConfusionWindowDays),
	}
}

func bandOf(bands map[int64]int, id int64) int {
	if b, ok := bands[id]; ok {
		return b
	}
	return -1
}

func unseenWord(id int64, band int) WordState {
	w := WordState{LexemeID: id, Band: band, State: Unseen, Skills: make([]SkillState, len(Skills))}
	for i, skill := range Skills {
		w.Skills[i] = SkillState{Skill: skill, State: Unseen}
	}
	return w
}

// evaluate turns the evidence into the state of every word that has any. A word with no
// evidence is not in the result.
func (m *Model) evaluate(ev db.LearnerEvidence, bands map[int64]int, now time.Time) map[int64]WordState {
	words := map[int64]*WordState{}
	word := func(id int64) *WordState {
		if w, ok := words[id]; ok {
			return w
		}
		w := unseenWord(id, bandOf(bands, id))
		words[id] = &w
		return &w
	}
	skillIx := func(skill string) int { return slices.Index(Skills, skill) }

	for id, e := range ev.Exposures {
		w := word(id)
		w.Exposures = e.Count
		w.LastSeen = laterOf(w.LastSeen, e.Last)
	}
	evidence := map[int64][]Evidence{}
	for _, ls := range ev.Skills {
		ix := skillIx(ls.Skill)
		if ix < 0 {
			continue
		}
		w := word(ls.LexemeID)
		if evidence[ls.LexemeID] == nil {
			evidence[ls.LexemeID] = make([]Evidence, len(Skills))
		}
		e := Evidence{
			Outcomes: ls.Outcomes, Window: ls.Window, WindowCorrect: ls.WindowCorrect,
			AgainOf: ls.AgainOf, Agains: ls.Agains, Recent: ls.Recent, RecentRight: ls.RecentRight,
			HasCard: ls.HasCard, Successes: ls.ReviewSuccesses, Stability: ls.Stability,
		}
		if ls.HasCard && !ls.LastReview.IsZero() {
			e.Retrievability = fsrs.Retrievability(ls.Stability, now.Sub(ls.LastReview).Hours()/24)
		}
		evidence[ls.LexemeID][ix] = e

		sk := &w.Skills[ix]
		sk.Attempts, sk.Window = ls.Outcomes, ls.Window
		if ls.Window > 0 {
			sk.Accuracy = float64(ls.WindowCorrect) / float64(ls.Window)
		}
		sk.AgainRecent = ls.Agains
		sk.LastSeen = laterOf(ls.LastOutcome, ls.LastReviewLog)
		sk.HasCard, sk.Stability, sk.Retrievability, sk.Successes = ls.HasCard, ls.Stability, e.Retrievability, ls.ReviewSuccesses
		sk.WasLearned = WasLearned(e)

		w.Attempts += ls.Outcomes
		w.LastSeen = laterOf(w.LastSeen, sk.LastSeen)
		w.Review.Reviews += ls.Reviews
		w.Review.Lapses += ls.Lapses
		w.Review.Agains += ls.ReviewAgains
		w.Review.Misses7d += ls.RecentAgains
		w.Review.LastReview = laterOf(w.Review.LastReview, ls.LastReviewLog)
	}

	missSince := now.AddDate(0, 0, -MissWindowDays)
	keys := map[int64]map[string]bool{}
	for _, c := range ev.Confusions {
		w, ok := words[c.LexemeID]
		if !ok {
			w = word(c.LexemeID)
		}
		w.Confusions = append(w.Confusions, Confusion{Kind: c.Kind, Key: c.Key, At: c.At})
		if !c.At.Before(missSince) {
			if keys[c.LexemeID] == nil {
				keys[c.LexemeID] = map[string]bool{}
			}
			keys[c.LexemeID][c.Key] = true
		}
	}

	for id, w := range words {
		es := evidence[id]
		var evidenced []State
		windowRight, windowTotal := 0, 0
		for i := range Skills {
			var e Evidence
			if es != nil {
				e = es[i]
			}
			e.Exposed = w.Exposures > 0
			sk := &w.Skills[i]
			sk.State = StateOf(e)
			if e.HasEvidence() {
				evidenced = append(evidenced, sk.State)
				windowRight += e.WindowCorrect
				windowTotal += e.Window
			}
		}
		switch {
		case len(evidenced) > 0:
			w.State = Overall(evidenced...)
		case w.Exposures > 0:
			w.State = Seen
		default:
			w.State = Unseen
		}
		if windowTotal > 0 {
			w.Accuracy = float64(windowRight) / float64(windowTotal)
		}
		// Stability and recall chance come from the weakest skill that has a card.
		best := -1
		for i := range w.Skills {
			sk := w.Skills[i]
			if !sk.HasCard || !sk.hasEvidence() {
				continue
			}
			if best < 0 || sk.State.rank() < w.Skills[best].State.rank() {
				best = i
			}
		}
		if best >= 0 {
			w.Stability, w.Retrievability = w.Skills[best].Stability, w.Skills[best].Retrievability
		}
		if w.Review.Reviews > 0 {
			w.Review.AgainRate = float64(w.Review.Agains) / float64(w.Review.Reviews)
		}
		if ks := keys[id]; len(ks) > 0 {
			for k := range ks {
				w.Review.ConfusionKeys = append(w.Review.ConfusionKeys, k)
			}
			slices.Sort(w.Review.ConfusionKeys)
		}
	}

	// Band context: words of a band that are struggling now and were seen this week.
	slipSince := now.AddDate(0, 0, -BandSlipDays)
	strugglers := map[int]int{}
	for _, w := range words {
		if w.Band >= 0 && weak(w.State) && !w.LastSeen.Before(slipSince) {
			strugglers[w.Band]++
		}
	}
	out := make(map[int64]WordState, len(words))
	for id, w := range words {
		if w.Band >= 0 {
			w.BandStrugglers = strugglers[w.Band]
		}
		w.Cause = Diagnose(*w, w.Confusions)
		out[id] = *w
	}
	return out
}

func laterOf(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
