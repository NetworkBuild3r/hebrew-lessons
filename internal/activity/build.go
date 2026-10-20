package activity

import (
	"context"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
)

// BandSource says which placement band a word is in.
type BandSource = learner.BandSource

// PlacementBands is the default BandSource: the band of the placement item a word was dealt
// as, the same rule internal/learner uses. A word that was never placed has no band.
func PlacementBands(store *db.Store) BandSource { return placementBands{store: store} }

type placementBands struct{ store *db.Store }

func (p placementBands) LexemeBands(ctx context.Context) (map[int64]int, error) {
	if p.store == nil {
		return nil, nil
	}
	pairs, err := p.store.PlacementItemLexemes(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int, len(pairs))
	for _, pair := range pairs {
		_, band, ok := placement.Lookup(pair.ItemID)
		if !ok {
			continue
		}
		if prev, seen := out[pair.LexemeID]; !seen || band < prev {
			out[pair.LexemeID] = band
		}
	}
	return out, nil
}

// Engine builds items and checks answers. It is safe for use by several goroutines.
type Engine struct {
	store *db.Store
	rec   *evidence.Recorder
	clock clock.Clock
	key   []byte
	bands BandSource
	ttl   time.Duration

	rngMu sync.Mutex
	rng   *rand.Rand

	pending *pendingSet
}

// Option changes how New builds an Engine.
type Option func(*Engine)

// WithKey sets the item token key. KeyFromSecret derives it from HEBREW_SESSION_SECRET.
func WithKey(key []byte) Option { return func(e *Engine) { e.key = append([]byte(nil), key...) } }

// WithSeed makes the option order and the distractor choice repeatable. Tokens stay random.
func WithSeed(seed int64) Option {
	return func(e *Engine) { e.rng = rand.New(rand.NewPCG(uint64(seed), 0x5bd1e995)) }
}

// WithBands sets where bands come from. The default is the placement deck.
func WithBands(b BandSource) Option { return func(e *Engine) { e.bands = b } }

// WithTTL sets how long an item can be answered. The default is TokenTTL.
func WithTTL(d time.Duration) Option { return func(e *Engine) { e.ttl = d } }

// New builds an engine. A nil clk uses the clock the store was opened with, and a nil rec is
// a recorder on that clock. Without WithKey the token key is random for this process.
func New(store *db.Store, clk clock.Clock, rec *evidence.Recorder, opts ...Option) *Engine {
	if clk == nil && store != nil {
		clk = store.Clock()
	}
	if rec == nil && store != nil {
		rec = evidence.New(store, nil, clk)
	}
	e := &Engine{
		store:   store,
		rec:     rec,
		clock:   clk,
		bands:   PlacementBands(store),
		ttl:     TokenTTL,
		rng:     rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
		pending: newPendingSet(),
	}
	for _, o := range opts {
		o(e)
	}
	if len(e.key) == 0 {
		// KeyFromSecret("") only fails when the system has no random source.
		e.key, _ = KeyFromSecret("")
	}
	return e
}

func (e *Engine) ready() error {
	if e == nil || e.store == nil || e.rec == nil || e.clock == nil || len(e.key) == 0 {
		return ErrNotConfigured
	}
	return nil
}

// BuildOption changes one Build.
type BuildOption func(*buildConfig)

type buildConfig struct {
	source string
	unit   int64
}

// WithSource sets where the question comes from: evidence.SourceReview, SourcePractice,
// SourcePlacement or SourceLesson. The default is practice. A placement item gives no
// feedback and only a right answer seeds a card. A lesson item stores the attempt and
// leaves every card alone.
func WithSource(source string) BuildOption { return func(c *buildConfig) { c.source = source } }

// WithUnit sets the unit a lesson item belongs to.
func WithUnit(unitID int64) BuildOption { return func(c *buildConfig) { c.unit = unitID } }

// built is an item with the answer the server keeps for it. Tests read the answer.
type built struct {
	item Item
	p    *pending
}

// Build makes one question about a word. The item carries an opaque token and no answer:
// the engine keeps the answer and Check finds it by the token. The token binds the learner,
// the word, the activity and a 30-minute expiry, and it is single use.
//
// A match item is built around the word: its other pairs are fair distractors of the word.
// Flashcard has no item (ErrNotChecked): session.Record grades it. Build refuses with
// ErrNoGloss, ErrNoAudio or ErrNotEnoughWords when the lexicon cannot support the activity
// for this word, and the caller then falls back to a flashcard.
func (e *Engine) Build(ctx context.Context, userID, lexemeID int64, kind Kind, opts ...BuildOption) (Item, error) {
	b, err := e.build(ctx, userID, lexemeID, kind, opts...)
	if err != nil {
		return Item{}, err
	}
	return b.item, nil
}

func (e *Engine) build(ctx context.Context, userID, lexemeID int64, kind Kind, opts ...BuildOption) (built, error) {
	if err := e.ready(); err != nil {
		return built{}, err
	}
	cfg := buildConfig{source: evidence.SourcePractice}
	for _, o := range opts {
		o(&cfg)
	}
	switch cfg.source {
	case evidence.SourcePlacement, evidence.SourceReview, evidence.SourceLesson, evidence.SourcePractice:
	default:
		return built{}, ErrBadSource
	}
	spec, ok := SpecOf(kind)
	if !ok {
		return built{}, ErrUnknownActivity
	}
	if !spec.Checked {
		return built{}, ErrNotChecked
	}
	if userID <= 0 || lexemeID <= 0 {
		return built{}, ErrUnknownWord
	}

	words, err := e.loadWords(ctx, userID)
	if err != nil {
		return built{}, err
	}
	var anchor Word
	found := false
	for _, w := range words {
		if w.ID == lexemeID {
			anchor, found = w, true
			break
		}
	}
	if !found {
		return built{}, ErrUnknownWord
	}
	if needsGloss(kind) && strings.TrimSpace(anchor.Gloss) == "" {
		return built{}, ErrNoGloss
	}
	if needsAudio(kind) && anchor.AudioID == "" {
		return built{}, ErrNoAudio
	}

	now := e.clock.Now()
	nonce, err := newNonce()
	if err != nil {
		return built{}, err
	}
	p := &pending{
		user: userID, lexeme: lexemeID, kind: kind, skill: spec.Skill,
		source: cfg.source, unit: cfg.unit, builtAt: now, expires: now.Add(e.ttl),
	}
	item := Item{Kind: kind, Skill: spec.Skill, ExpiresAt: p.expires, Typed: spec.Typed}

	switch kind {
	case ListenPick, ReadPick:
		prefer, err := e.partners(ctx, userID, anchor, now)
		if err != nil {
			return built{}, err
		}
		dis, err := e.pick(anchor, words, OptionCount-1, Need{}, prefer, false)
		if err != nil {
			return built{}, err
		}
		shown := e.shuffled(append([]Word{anchor}, dis...))
		p.words = shown
		for i, w := range shown {
			item.Options = append(item.Options, w.Gloss)
			if w.ID == anchor.ID {
				p.correct = i
			}
		}
		if kind == ListenPick {
			item.Prompt = Prompt{AudioID: anchor.AudioID}
		} else {
			item.Prompt = Prompt{Hebrew: anchor.Text}
		}
	case TypeIt:
		p.expected = anchor.Text
		p.words = []Word{anchor}
		item.Prompt = Prompt{Gloss: anchor.Gloss}
	case Dictation:
		p.expected = anchor.Text
		p.words = []Word{anchor}
		item.Prompt = Prompt{AudioID: anchor.AudioID}
	case WordMatch, AudioMatch:
		n, need := WordMatchPairs, Need{}
		if kind == AudioMatch {
			n, need = AudioMatchClips, Need{Audio: true}
		}
		prefer, err := e.partners(ctx, userID, anchor, now)
		if err != nil {
			return built{}, err
		}
		others, err := e.pick(anchor, words, n-1, need, prefer, true)
		if err != nil {
			return built{}, err
		}
		left := e.shuffled(append([]Word{anchor}, others...))
		right := e.shuffled(append([]Word(nil), left...))
		p.words, p.rightWords = left, right
		p.rightOf = make([]int, len(left))
		for i, l := range left {
			for j, r := range right {
				if r.ID == l.ID {
					p.rightOf[i] = j
				}
			}
			if kind == AudioMatch {
				item.Left = append(item.Left, Cue{AudioID: l.AudioID})
			} else {
				item.Left = append(item.Left, Cue{Hebrew: l.Text})
			}
		}
		for _, r := range right {
			item.Right = append(item.Right, r.Gloss)
		}
	}

	claimsOf := claims{ID: nonce, User: userID, Lexeme: lexemeID, Kind: kind, Exp: p.expires.Unix()}
	item.Token, err = signClaims(e.key, claimsOf)
	if err != nil {
		return built{}, err
	}
	e.pending.add(nonce, p, now)
	return built{item: item, p: p}, nil
}

func needsAudio(k Kind) bool { return k == ListenPick || k == AudioMatch || k == Dictation }

func needsGloss(k Kind) bool {
	return k == ListenPick || k == ReadPick || k == WordMatch || k == AudioMatch || k == TypeIt
}

// loadWords reads the lexicon as Words with their placement bands.
func (e *Engine) loadWords(ctx context.Context, userID int64) ([]Word, error) {
	pool, err := e.store.ActivityPool(ctx, userID)
	if err != nil {
		return nil, err
	}
	bands, err := e.bands.LexemeBands(ctx)
	if err != nil {
		return nil, err
	}
	words := make([]Word, len(pool))
	for i, w := range pool {
		band := -1
		if b, ok := bands[w.ID]; ok {
			band = b
		}
		words[i] = Word{ID: w.ID, Text: w.TextNFC, Gloss: w.Gloss, POS: w.POS, AudioID: w.AudioID, Band: band, Seen: w.Seen}
	}
	return words, nil
}

func (e *Engine) pick(answer Word, pool []Word, n int, need Need, prefer func(Word) bool, seenFirst bool) ([]Word, error) {
	e.rngMu.Lock()
	defer e.rngMu.Unlock()
	return Pick(answer, pool, n, need, prefer, seenFirst, e.rng)
}

func (e *Engine) shuffled(ws []Word) []Word {
	e.rngMu.Lock()
	defer e.rngMu.Unlock()
	e.rng.Shuffle(len(ws), func(i, j int) { ws[i], ws[j] = ws[j], ws[i] })
	return ws
}

// partners reads the learner's recent confusions about the word and returns a test for the
// words they were taken for: the other half of a word-pair confusion, and any word that is
// one letter off in a confused letter pair. It is nil when there are none.
func (e *Engine) partners(ctx context.Context, userID int64, anchor Word, now time.Time) (func(Word) bool, error) {
	since := now.AddDate(0, 0, -learner.ConfusionWindowDays)
	rows, err := e.store.WordConfusions(ctx, userID, anchor.ID, since)
	if err != nil {
		return nil, err
	}
	return partnerTest(anchor, rows), nil
}

func partnerTest(anchor Word, rows []db.WordConfusion) func(Word) bool {
	texts := map[string]bool{}
	pairs := map[string]bool{}
	for _, r := range rows {
		switch r.Kind {
		case fsrs.MissOther:
			a, b, _ := strings.Cut(r.Key, "/")
			switch anchor.Text {
			case a:
				texts[b] = true
			case b:
				texts[a] = true
			}
		case fsrs.MissLetterPair:
			pairs[r.Key] = true
		}
	}
	if len(texts) == 0 && len(pairs) == 0 {
		return nil
	}
	return func(w Word) bool {
		if texts[w.Text] {
			return true
		}
		if len(pairs) == 0 {
			return false
		}
		key, ok := oneLetterPair(anchor.Text, w.Text)
		return ok && (pairs[key] || pairs[foldedKey(key)])
	}
}

// foldedKey is a letter-pair key with final letters written as ordinary ones, which is how
// internal/evidence keys a pair it finds between two picked words.
func foldedKey(key string) string {
	a, b, _ := strings.Cut(key, "/")
	ra, rb := []rune(a), []rune(b)
	if len(ra) != 1 || len(rb) != 1 {
		return key
	}
	return pairKey(ordinary(ra[0]), ordinary(rb[0]))
}

// ordinary maps a final letter (kaf, mem, nun, pe, tsadi) to its ordinary form.
func ordinary(r rune) rune {
	switch r {
	case '\u05DA':
		return '\u05DB'
	case '\u05DD':
		return '\u05DE'
	case '\u05DF':
		return '\u05E0'
	case '\u05E3':
		return '\u05E4'
	case '\u05E5':
		return '\u05E6'
	default:
		return r
	}
}
