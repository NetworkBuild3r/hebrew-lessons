package brief

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

// Version is the version of the payload shape, the "v" field of Plan.
const Version = 1

// The hard limits. The word counts are north-star section 7.1 and SPEC-019.
const (
	// MinNewWords and MaxNewWords bound the new words of a brief. MaxNewWords is the unit
	// budget (units.MaxNewWordsPerUnit, which a test pins). This package does not import
	// internal/units, because the brief checks of SPEC-020 will live there.
	MinNewWords = 5
	MaxNewWords = 8

	// BeginnerKnownBelow is the vocabulary size under which a lesson takes fewer new words,
	// so a short passage can stay mostly known (north-star 7.3). BeginnerMinNewWords and
	// BeginnerMaxNewWords are those fewer.
	BeginnerKnownBelow  = 100
	BeginnerMinNewWords = 3
	BeginnerMaxNewWords = 4

	// DevelopingKnownBelow is the vocabulary size from which the passage length is free.
	DevelopingKnownBelow = 300

	// MinStruggleWords is how many struggle words the brief aims for, and MaxStruggleWords
	// is the most it holds. A learner with fewer struggling words gets fewer.
	MinStruggleWords = 3
	MaxStruggleWords = 5

	// MinStruggleUses is the fewest times a struggle word must appear in the lesson. A
	// struggling word needs one more than a regressed one.
	MinStruggleUses       = 3
	StrugglingExtraUses   = 1
	RegressedRequiredUses = MinStruggleUses

	// MaxComfortableWords is the most known or solid words a brief lists.
	MaxComfortableWords = 150

	// NewWordMinUses is how many times each new word must be used in the lesson.
	NewWordMinUses = 3

	// CoverageTarget is the known-word coverage the lesson aims for (SPEC-019). The staged
	// target for a small vocabulary is in Tier.
	CoverageTarget = 0.95

	// MinSkillEvidence is the fewest words with evidence a skill needs before it can be
	// the skill focus.
	MinSkillEvidence = 3

	// DefaultSkillFocus is the focus when no unlocked skill has enough evidence. Listening
	// is open from the start (north-star 4.1).
	DefaultSkillFocus = fsrs.SkillListen

	// MaxCoachNotes is how many coach notes a brief carries, and CoachNoteDays how far back
	// they count.
	MaxCoachNotes = 5
	CoachNoteDays = 30

	// TTL is how long a brief can be read and used.
	TTL = 7 * 24 * time.Hour

	// MaxPayloadBytes is the largest payload_json.
	MaxPayloadBytes = 64 << 10

	// MaxThemeRunes is the longest theme, in characters, as lesson_briefs.theme allows.
	MaxThemeRunes = 200

	// DefaultTheme is stored when the caller gives none.
	DefaultTheme = "general"

	// MaxGlossRunes cuts a long gloss. A word whose text is longer than MaxTextRunes is not
	// a word to teach, and is left out.
	MaxGlossRunes = 100
	MaxTextRunes  = 80

	// maxScanWords bounds how many words one list read pages through.
	maxScanWords = 5000
	// maxPoolPerBand bounds the unseen candidates read for one band.
	maxPoolPerBand = 1000
)

// The activity types of SPEC-015 that a brief names. flashcard is never named: a checked
// activity is used wherever one exists (north-star 6).
const (
	ActivityListenPick = "listen_pick"
	ActivityReadPick   = "read_pick"
	ActivityWordMatch  = "word_match"
	ActivityAudioMatch = "audio_match"
	ActivityTypeIt     = "type_it"
	ActivityDictation  = "dictation"
)

// skillActivities is the activity types that train each skill, in the order a brief lists them.
var skillActivities = map[string][]string{
	fsrs.SkillListen: {ActivityListenPick, ActivityAudioMatch},
	fsrs.SkillRead:   {ActivityReadPick, ActivityWordMatch},
	fsrs.SkillRecall: {ActivityTypeIt},
	fsrs.SkillSpell:  {ActivityDictation},
}

// Tier names for Tier.Name.
const (
	TierBeginner    = "beginner"
	TierDeveloping  = "developing"
	TierEstablished = "established"
)

// Errors. ErrNotFound covers an id that is malformed, unknown, or another learner's.
var (
	ErrNotFound = errors.New("brief: not found")
	ErrExpired  = errors.New("brief: expired")
	ErrUsed     = errors.New("brief: already used")
	ErrTheme    = errors.New("brief: bad theme")
	ErrTooLarge = errors.New("brief: payload too large")
)

// Word is a lexeme in a brief.
type Word struct {
	LexemeID int64  `json:"lexeme_id"`
	Text     string `json:"text"`  // Pointed NFC.
	Gloss    string `json:"gloss"` // Plain text. Escape it on output.
}

// NewWord is a word to teach.
type NewWord struct {
	Word
	// Band is the placement band, 0 to 4, or -1 when the word has none.
	Band       int  `json:"band"`
	MinUses    int  `json:"min_uses"`
	ThemeMatch bool `json:"theme_match"`
}

// StruggleWord is a word to bring back.
type StruggleWord struct {
	Word
	State string `json:"state"` // struggling or regressed.
	// Skill is the skill that is failing: the weak skill with the lowest accuracy.
	Skill string `json:"skill"`
	// Cause is the model's diagnosis (north-star 4.5), or empty when no pattern fits.
	Cause string `json:"cause"`
	// Score is StruggleScore. The words are listed from the highest.
	Score float64 `json:"score"`
	// RequiredCount is how many times the lesson must use the word. It is 3 or more.
	RequiredCount int      `json:"required_count"`
	Activities    []string `json:"activities"`
}

// Tier is the staged coverage rule for the learner's vocabulary size (north-star 7.3).
type Tier struct {
	Name          string `json:"name"`
	KnownWords    int    `json:"known_words"`
	MaxTextTokens int    `json:"max_text_tokens"` // 0: no limit beyond the unit budget.
	// MinKnownShare is the share of passage tokens that must be known at this tier. The
	// Targets coverage is the full-strength figure, and this is the staged one.
	MinKnownShare float64 `json:"min_known_share"`
}

// Targets are what the finished lesson is measured against.
type Targets struct {
	Coverage       float64 `json:"coverage"`
	NewWordMinUses int     `json:"new_word_min_uses"`
}

// CoachNote is a note a client left for the learner.
type CoachNote struct {
	Author string    `json:"author"` // Plain text. Escape it on output.
	Body   string    `json:"body"`   // Plain text. Escape it on output.
	At     time.Time `json:"at"`
}

// Plan is the versioned payload of a brief: lesson_briefs.payload_json.
type Plan struct {
	V                int            `json:"v"`
	Theme            string         `json:"theme"`
	Tier             Tier           `json:"tier"`
	NewWords         []NewWord      `json:"new_words"`
	StruggleWords    []StruggleWord `json:"struggle_words"`
	ComfortableWords []Word         `json:"comfortable_words"`
	// FocusCause is the cause most struggle words share, or empty.
	FocusCause string `json:"focus_cause"`
	// SkillFocus is the weakest skill overall. It gets the most activities.
	SkillFocus string `json:"skill_focus"`
	// Activities are the activity types that train the skill focus and each struggle
	// word's failing skill, without repeats.
	Activities []string    `json:"activities"`
	Targets    Targets     `json:"targets"`
	Notes      []string    `json:"notes"`
	CoachNotes []CoachNote `json:"coach_notes"`
}

// Brief is a stored Plan. The id, times, and use are the table's columns.
type Brief struct {
	ID        string    `json:"id"`
	UserID    int64     `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	// UsedByUnitID is the unit built from this brief, or nil while it is unused.
	UsedByUnitID *int64 `json:"used_by_unit_id,omitempty"`
	Plan
}

// TagSource gives lexemes their tags, for theme matching. The lexicon has no tag column
// today, so the default has none and the theme matches on the gloss alone.
type TagSource interface {
	// Tags returns the tags of each lexeme it knows. A lexeme with none is left out.
	Tags(ctx context.Context, lexemeIDs []int64) (map[int64][]string, error)
}

// Option changes how New builds a Builder.
type Option func(*Builder)

// WithSeed mixes a seed into the selection order. The default is 0.
func WithSeed(seed uint64) Option { return func(b *Builder) { b.seed = seed } }

// WithRand sets the source of brief ids. The default is crypto/rand. Tests use it to make
// the random source fail.
func WithRand(r io.Reader) Option { return func(b *Builder) { b.rand = r } }

// WithTags sets where lexeme tags come from.
func WithTags(t TagSource) Option { return func(b *Builder) { b.tags = t } }

// WithUnlockedSkills sets which skills the learner has unlocked. A locked skill is never the
// skill focus (north-star 4.1: the letters check opens read and spell). The default is every
// skill, because no code records the letters check yet.
func WithUnlockedSkills(f func(ctx context.Context, userID int64) ([]string, error)) Option {
	return func(b *Builder) { b.unlocked = f }
}

// Builder builds briefs and reads them back. It holds no learner data and is safe for use by
// several goroutines.
type Builder struct {
	store *db.Store
	model *learner.Model
	clock clock.Clock

	seed     uint64
	rand     io.Reader
	tags     TagSource
	unlocked func(ctx context.Context, userID int64) ([]string, error)

	maxPayload int
}

// New builds a Builder. A nil clk uses the clock the store was opened with.
func New(store *db.Store, model *learner.Model, clk clock.Clock, opts ...Option) *Builder {
	if clk == nil && store != nil {
		clk = store.Clock()
	}
	b := &Builder{store: store, model: model, clock: clk, maxPayload: MaxPayloadBytes}
	for _, o := range opts {
		o(b)
	}
	return b
}

func (b *Builder) ready() error {
	if b == nil || b.store == nil || b.model == nil {
		return errors.New("brief: builder is not configured")
	}
	if b.clock == nil {
		return errors.New("brief: clock is not configured")
	}
	return nil
}

// TierFor is the tier of a learner who knows this many words (known or solid).
func TierFor(known int) Tier {
	switch {
	case known < BeginnerKnownBelow:
		return Tier{Name: TierBeginner, KnownWords: known, MaxTextTokens: 20, MinKnownShare: 0.60}
	case known < DevelopingKnownBelow:
		return Tier{Name: TierDeveloping, KnownWords: known, MaxTextTokens: 60, MinKnownShare: 0.90}
	default:
		return Tier{Name: TierEstablished, KnownWords: known, MaxTextTokens: 0, MinKnownShare: 0.95}
	}
}

// NewWordCount is how many new words a lesson takes. It starts at the top of the tier's range
// and drops by one for each struggle word beyond MinStruggleWords, so a heavy review load
// leaves room, and never goes under the bottom of the range.
func NewWordCount(t Tier, struggleWords int) int {
	lo, hi := MinNewWords, MaxNewWords
	if t.Name == TierBeginner {
		lo, hi = BeginnerMinNewWords, BeginnerMaxNewWords
	}
	return max(lo, hi-max(0, struggleWords-MinStruggleWords))
}

// StruggleScore says how badly a word is going. Higher is worse.
//
//	score = 100 * (1 - accuracy) + 10 * misses in 7 days + 5 * confusion keys in 7 days
//	        + 25 when struggling (a regressed word is the lesser case)
//
// A word with no attempts (only a card from before v3) counts as 50% accurate.
func StruggleScore(w learner.WordState) float64 {
	acc := w.Accuracy
	if w.Attempts == 0 {
		acc = 0.5
	}
	score := 100*(1-acc) + 10*float64(w.Review.Misses7d) + 5*float64(len(w.Review.ConfusionKeys))
	if w.State == learner.Struggling {
		score += 25
	}
	return score
}

// FailingSkill is the skill a struggling or regressed word is failing: among its weak
// skills, the one with the lowest accuracy, the first in learner.Skills on a tie.
func FailingSkill(w learner.WordState) string {
	best, bestAcc := "", 2.0
	for _, s := range w.Skills {
		if s.State != learner.Struggling && s.State != learner.Regressed {
			continue
		}
		if s.Accuracy < bestAcc {
			best, bestAcc = s.Skill, s.Accuracy
		}
	}
	if best == "" {
		return fsrs.SkillRecall
	}
	return best
}

// Build makes a brief for the learner and stores it. theme may be empty. It never reads
// another learner's evidence, and the learner id is the caller's, taken from the principal.
func (b *Builder) Build(ctx context.Context, userID int64, theme string) (Brief, error) {
	if err := b.ready(); err != nil {
		return Brief{}, err
	}
	if userID <= 0 {
		return Brief{}, errors.New("brief: no learner")
	}
	given := strings.TrimSpace(theme) != ""
	theme, err := cleanTheme(theme)
	if err != nil {
		return Brief{}, err
	}
	now := b.clock.Now().UTC()
	day := now.Format(time.DateOnly)
	var keywords []string
	if given {
		keywords = themeKeywords(theme)
	}

	sum, err := b.model.Summary(ctx, userID)
	if err != nil {
		return Brief{}, fmt.Errorf("brief: learner summary: %w", err)
	}
	tier := TierFor(sum.Words[learner.Known] + sum.Words[learner.Solid])

	skills, err := b.unlockedSkills(ctx, userID)
	if err != nil {
		return Brief{}, err
	}
	plan := Plan{V: Version, Theme: theme, Tier: tier, Notes: []string{}, Activities: []string{},
		Targets: Targets{Coverage: CoverageTarget, NewWordMinUses: NewWordMinUses}}
	plan.SkillFocus = skillFocus(sum, skills)

	struggle, err := b.struggleWords(ctx, userID)
	if err != nil {
		return Brief{}, err
	}
	plan.StruggleWords = struggle
	plan.FocusCause = focusCause(struggle)

	want := NewWordCount(tier, len(struggle))
	newWords, notes, err := b.newWords(ctx, userID, day, want, keywords)
	if err != nil {
		return Brief{}, err
	}
	plan.NewWords = newWords
	plan.Notes = append(plan.Notes, notes...)

	comfortable, err := b.comfortableWords(ctx, userID, keywords)
	if err != nil {
		return Brief{}, err
	}
	plan.ComfortableWords = comfortable

	plan.Activities = activitiesFor(plan.SkillFocus, struggle)
	plan.CoachNotes, err = b.coachNotes(ctx, userID, now)
	if err != nil {
		return Brief{}, err
	}

	payload, err := b.fit(&plan)
	if err != nil {
		return Brief{}, err
	}
	id, err := b.newID()
	if err != nil {
		return Brief{}, err
	}
	if err := b.insert(ctx, id, userID, theme, payload, now); err != nil {
		return Brief{}, err
	}
	return Brief{ID: id, UserID: userID, CreatedAt: now, ExpiresAt: now.Add(TTL), Plan: plan}, nil
}

// fit marshals the plan, dropping comfortable words from the end until it is no larger than
// the payload limit.
func (b *Builder) fit(plan *Plan) ([]byte, error) {
	for {
		payload, err := json.Marshal(plan)
		if err != nil {
			return nil, fmt.Errorf("brief: encode: %w", err)
		}
		if len(payload) <= b.maxPayload {
			return payload, nil
		}
		if len(plan.ComfortableWords) == 0 {
			return nil, ErrTooLarge
		}
		// Halve the excess quickly, then one at a time near the limit.
		drop := max(1, len(plan.ComfortableWords)/8)
		plan.ComfortableWords = plan.ComfortableWords[:len(plan.ComfortableWords)-drop]
		if !slices.Contains(plan.Notes, trimNote) {
			plan.Notes = append(plan.Notes, trimNote)
		}
	}
}

const trimNote = "The list of comfortable words was cut to keep the brief within its size limit."

func cleanTheme(theme string) (string, error) {
	theme = strings.TrimSpace(theme)
	if theme == "" {
		return DefaultTheme, nil
	}
	if !utf8.ValidString(theme) {
		return "", fmt.Errorf("%w: not valid UTF-8", ErrTheme)
	}
	if utf8.RuneCountInString(theme) > MaxThemeRunes {
		return "", fmt.Errorf("%w: longer than %d characters", ErrTheme, MaxThemeRunes)
	}
	for _, r := range theme {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("%w: contains a control character", ErrTheme)
		}
	}
	return theme, nil
}

func (b *Builder) unlockedSkills(ctx context.Context, userID int64) ([]string, error) {
	if b.unlocked == nil {
		return learner.Skills, nil
	}
	skills, err := b.unlocked(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("brief: unlocked skills: %w", err)
	}
	return skills, nil
}

// skillFocus is the weakest unlocked skill that has evidence on at least MinSkillEvidence
// words. A skill's weakness is the share of its evidenced words that are struggling or
// regressed, with a learning word counting half. The first skill in learner.Skills wins a
// tie. With no skill to judge, the focus is DefaultSkillFocus.
func skillFocus(sum learner.Summary, unlocked []string) string {
	focus, worst := "", -1.0
	for _, ss := range sum.Skills {
		if !slices.Contains(unlocked, ss.Skill) {
			continue
		}
		evidenced := ss.Counts[learner.Learning] + ss.Counts[learner.Struggling] + ss.Counts[learner.Regressed] +
			ss.Counts[learner.Known] + ss.Counts[learner.Solid]
		if evidenced < MinSkillEvidence {
			continue
		}
		weak := float64(ss.Counts[learner.Struggling]+ss.Counts[learner.Regressed]) + 0.5*float64(ss.Counts[learner.Learning])
		if w := weak / float64(evidenced); w > worst {
			focus, worst = ss.Skill, w
		}
	}
	if focus == "" {
		if len(unlocked) > 0 && !slices.Contains(unlocked, DefaultSkillFocus) {
			return unlocked[0]
		}
		return DefaultSkillFocus
	}
	return focus
}

func (b *Builder) struggleWords(ctx context.Context, userID int64) ([]StruggleWord, error) {
	var words []learner.WordState
	for _, state := range []learner.State{learner.Struggling, learner.Regressed} {
		got, err := b.listAll(ctx, userID, learner.Filter{State: state}, maxScanWords)
		if err != nil {
			return nil, err
		}
		words = append(words, got...)
	}
	type scored struct {
		w     learner.WordState
		score float64
	}
	var ranked []scored
	for _, w := range words {
		if _, ok := briefWord(w); !ok {
			continue
		}
		ranked = append(ranked, scored{w, StruggleScore(w)})
	}
	slices.SortFunc(ranked, func(a, c scored) int {
		switch {
		case a.score > c.score:
			return -1
		case a.score < c.score:
			return 1
		}
		return cmpInt64(a.w.LexemeID, c.w.LexemeID)
	})
	if len(ranked) > MaxStruggleWords {
		ranked = ranked[:MaxStruggleWords]
	}
	out := make([]StruggleWord, 0, len(ranked))
	for _, r := range ranked {
		word, _ := briefWord(r.w)
		skill := FailingSkill(r.w)
		uses := RegressedRequiredUses
		if r.w.State == learner.Struggling {
			uses = MinStruggleUses + StrugglingExtraUses
		}
		out = append(out, StruggleWord{
			Word: word, State: string(r.w.State), Skill: skill, Cause: string(r.w.Cause),
			Score: r.score, RequiredCount: uses, Activities: slices.Clone(skillActivities[skill]),
		})
	}
	return out, nil
}

// focusCause is the cause most struggle words share. A tie goes to the cause of the
// highest-scored word among those tied, and words with no cause do not vote.
func focusCause(words []StruggleWord) string {
	counts := map[string]int{}
	best, bestN := "", 0
	for _, w := range words {
		if w.Cause == "" {
			continue
		}
		counts[w.Cause]++
		if counts[w.Cause] > bestN {
			best, bestN = w.Cause, counts[w.Cause]
		}
	}
	return best
}

func activitiesFor(focus string, struggle []StruggleWord) []string {
	out := []string{}
	add := func(types []string) {
		for _, t := range types {
			if !slices.Contains(out, t) {
				out = append(out, t)
			}
		}
	}
	add(skillActivities[focus])
	for _, w := range struggle {
		add(skillActivities[w.Skill])
	}
	return out
}

// themeSet is the words of a pool that fit the theme, by lexeme id. A word fits when a tag
// matches a theme keyword, and failing that when a gloss keyword does (SPEC-019 aud-1).
func (b *Builder) themeSet(ctx context.Context, keywords []string, pool []learner.WordState) (map[int64]bool, error) {
	out := map[int64]bool{}
	if len(keywords) == 0 || len(pool) == 0 {
		return out, nil
	}
	var tags map[int64][]string
	if b.tags != nil {
		ids := make([]int64, len(pool))
		for i, w := range pool {
			ids[i] = w.LexemeID
		}
		var err error
		if tags, err = b.tags.Tags(ctx, ids); err != nil {
			return nil, fmt.Errorf("brief: lexeme tags: %w", err)
		}
	}
	for _, w := range pool {
		if matchesAny(keywords, tags[w.LexemeID]...) || matchesAny(keywords, w.Gloss) {
			out[w.LexemeID] = true
		}
	}
	return out, nil
}

// themeStop are words too common to say what a theme is about.
var themeStop = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "about": true, "from": true,
	"into": true, "your": true, "our": true, "some": true, "lesson": true, "words": true,
}

// themeKeywords splits a theme into lower-case keywords of 3 or more characters, without
// stop words and with a plural "s" removed.
func themeKeywords(theme string) []string {
	var out []string
	for _, f := range tokens(theme) {
		if utf8.RuneCountInString(f) < 3 || themeStop[f] {
			continue
		}
		if s := stem(f); !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func tokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

func stem(w string) string {
	if len(w) > 3 && strings.HasSuffix(w, "s") && !strings.HasSuffix(w, "ss") {
		return w[:len(w)-1]
	}
	return w
}

// matchesAny reports whether any word of any text, stemmed, is one of the keywords.
func matchesAny(keywords []string, texts ...string) bool {
	for _, t := range texts {
		for _, tok := range tokens(t) {
			if slices.Contains(keywords, stem(tok)) {
				return true
			}
		}
	}
	return false
}

// bands are tried in order: the placement bands from the most common word, then the words that
// were never placed.
var bandOrder = []int{0, 1, 2, 3, 4, -1}

// newWords picks the words to teach: the unseen words of the lowest band that has any, themed
// words first and each group in seeded order. A band with too few words is topped up from the
// next band. It returns the words and any notes for the brief.
func (b *Builder) newWords(ctx context.Context, userID int64, day string, want int, keywords []string) ([]NewWord, []string, error) {
	var picked []NewWord
	var notes []string
	themedPicked, themedAvail := 0, 0
	for _, band := range bandOrder {
		if len(picked) >= want {
			break
		}
		pool, err := b.unseenInBand(ctx, userID, band)
		if err != nil {
			return nil, nil, err
		}
		usable := pool[:0:0]
		for _, w := range pool {
			// An unseen word cannot be struggling. The check keeps that true if the model changes.
			if _, ok := briefWord(w); ok && w.State == learner.Unseen {
				usable = append(usable, w)
			}
		}
		themed, err := b.themeSet(ctx, keywords, usable)
		if err != nil {
			return nil, nil, err
		}
		themedAvail += len(themed)
		b.order(usable, userID, day, func(w learner.WordState) bool { return themed[w.LexemeID] })
		for _, w := range usable {
			if len(picked) == want {
				break
			}
			word, _ := briefWord(w)
			picked = append(picked, NewWord{Word: word, Band: w.Band, MinUses: NewWordMinUses, ThemeMatch: themed[w.LexemeID]})
			if themed[w.LexemeID] {
				themedPicked++
			}
		}
	}
	if len(keywords) > 0 {
		switch {
		case themedAvail == 0:
			notes = append(notes, "No word matched the theme, so the theme was ignored when choosing new words.")
		case themedPicked < len(picked):
			notes = append(notes, fmt.Sprintf("Only %d of %d new words match the theme.", themedPicked, len(picked)))
		}
	}
	if len(picked) < want {
		notes = append(notes, fmt.Sprintf("Only %d unseen words were available, fewer than the %d planned.", len(picked), want))
	}
	return picked, notes, nil
}

// order sorts words with the preferred ones first, each group by the seeded rank, then id.
func (b *Builder) order(words []learner.WordState, userID int64, day string, preferred func(learner.WordState) bool) {
	slices.SortFunc(words, func(x, y learner.WordState) int {
		if px, py := preferred(x), preferred(y); px != py {
			if px {
				return -1
			}
			return 1
		}
		rx, ry := b.rank(userID, day, x.LexemeID), b.rank(userID, day, y.LexemeID)
		switch {
		case rx < ry:
			return -1
		case rx > ry:
			return 1
		}
		return cmpInt64(x.LexemeID, y.LexemeID)
	})
}

// unseenInBand lists the unseen words of one band, at most maxPoolPerBand, by lexeme id.
// Band -1 is the words with no band, which the model can only list among every band.
func (b *Builder) unseenInBand(ctx context.Context, userID int64, band int) ([]learner.WordState, error) {
	if band >= 0 {
		return b.listAll(ctx, userID, learner.Filter{State: learner.Unseen, Band: &band}, maxPoolPerBand)
	}
	var out []learner.WordState
	f := learner.Filter{State: learner.Unseen, Limit: learner.MaxPageSize}
	for scanned := 0; scanned < maxScanWords && len(out) < maxPoolPerBand; {
		page, next, err := b.model.List(ctx, userID, f)
		if err != nil {
			return nil, fmt.Errorf("brief: learner words: %w", err)
		}
		scanned += len(page)
		for _, w := range page {
			if w.Band < 0 && len(out) < maxPoolPerBand {
				out = append(out, w)
			}
		}
		if next == "" {
			break
		}
		f.After = next
	}
	return out, nil
}

// comfortableWords picks up to MaxComfortableWords known or solid words, themed ones first,
// then the most recently seen, then by id.
func (b *Builder) comfortableWords(ctx context.Context, userID int64, keywords []string) ([]Word, error) {
	var pool []learner.WordState
	for _, state := range []learner.State{learner.Known, learner.Solid} {
		got, err := b.listAll(ctx, userID, learner.Filter{State: state}, maxScanWords)
		if err != nil {
			return nil, err
		}
		pool = append(pool, got...)
	}
	themed, err := b.themeSet(ctx, keywords, pool)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(pool, func(x, y learner.WordState) int {
		if tx, ty := themed[x.LexemeID], themed[y.LexemeID]; tx != ty {
			if tx {
				return -1
			}
			return 1
		}
		if !x.LastSeen.Equal(y.LastSeen) {
			if x.LastSeen.After(y.LastSeen) {
				return -1
			}
			return 1
		}
		return cmpInt64(x.LexemeID, y.LexemeID)
	})
	out := []Word{}
	for _, w := range pool {
		if len(out) == MaxComfortableWords {
			break
		}
		if word, ok := briefWord(w); ok {
			out = append(out, word)
		}
	}
	return out, nil
}

func (b *Builder) listAll(ctx context.Context, userID int64, f learner.Filter, limit int) ([]learner.WordState, error) {
	f.Limit = learner.MaxPageSize
	var out []learner.WordState
	for len(out) < limit {
		page, next, err := b.model.List(ctx, userID, f)
		if err != nil {
			return nil, fmt.Errorf("brief: learner words: %w", err)
		}
		out = append(out, page...)
		if next == "" {
			break
		}
		f.After = next
	}
	return out, nil
}

func briefWord(w learner.WordState) (Word, bool) {
	n := utf8.RuneCountInString(w.Text)
	if n == 0 || n > MaxTextRunes {
		return Word{}, false
	}
	gloss := w.Gloss
	if utf8.RuneCountInString(gloss) > MaxGlossRunes {
		gloss = string([]rune(gloss)[:MaxGlossRunes])
	}
	return Word{LexemeID: w.LexemeID, Text: w.Text, Gloss: gloss}, true
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// rank orders candidates by day and seed with no random draw.
func (b *Builder) rank(userID int64, day string, lexemeID int64) uint64 {
	h := sha256.New()
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], b.seed)
	h.Write(buf[:])
	binary.BigEndian.PutUint64(buf[:], uint64(userID))
	h.Write(buf[:])
	h.Write([]byte(day))
	binary.BigEndian.PutUint64(buf[:], uint64(lexemeID))
	h.Write(buf[:])
	return binary.BigEndian.Uint64(h.Sum(nil)[:8])
}
