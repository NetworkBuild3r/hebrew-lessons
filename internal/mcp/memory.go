package mcp

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"golang.org/x/text/unicode/norm"
)

// Memory is the in-memory CourseReader, LexiconReader,
// UnitWriter, and units.Lookup used by tests. It does not derive
// lesson_items and it does not write SQL. The process does not mount it.
type Memory struct {
	mu             sync.Mutex
	nextID         int64
	nextLex        int64
	units          []memUnit
	lexemes        []storedLexeme
	audio          map[string]bool
	speakerAudio   map[string]bool
	placement      map[int64]storedPlacement
	struggle       map[int64]storedStruggle
	completed      map[int64][]UnitCompletion
	completedWords map[int64][]string
	cards          []storedCard
	reviews        []storedReview
	lessons        []storedLesson
	nextCard       int64
	learn          memLearn
}

type storedPlacement struct {
	row      Placement
	known    []string
	learning []string
}

type storedStruggle struct {
	lexemes    []StruggleLexeme
	confusions []StruggleConfusion
}

type memRev struct {
	rev       int
	canonical []byte
}

type memUnit struct {
	id        int64
	position  int
	title     string
	status    string
	scope     string
	rev       int
	canonical []byte
	doc       units.Unit
	actor     string
	revisions []memRev
}

type storedLexeme struct {
	id       int64
	lx       Lexeme
	verified bool
}

// NewMemory returns an empty store.
func NewMemory() *Memory {
	return &Memory{audio: map[string]bool{}}
}

// AddLexeme inserts a dictionary row. AudioPresent also marks that exact
// text as having a clip for units.Lookup.
func (m *Memory) AddLexeme(lx Lexeme) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextLex++
	m.lexemes = append(m.lexemes, storedLexeme{id: m.nextLex, lx: lx})
	if lx.AudioPresent && lx.TextNFC != "" {
		m.audio[lx.TextNFC] = true
	}
}

// SetAudio marks an exact text as having a clip. Sentences that are not
// themselves lexicon entries use this.
func (m *Memory) SetAudio(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.audio == nil {
		m.audio = map[string]bool{}
	}
	m.audio[text] = true
}

// SetSpeakerAudio marks an exact text as having a clip in the voice of speaker
// a or b. It does not mark the text for HasAudio, which is any voice.
func (m *Memory) SetSpeakerAudio(text, speaker string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.speakerAudio == nil {
		m.speakerAudio = map[string]bool{}
	}
	m.speakerAudio[speaker+"\x00"+text] = true
}

// HasAudioForSpeaker implements units.SpeakerLookup.
func (m *Memory) HasAudioForSpeaker(ctx context.Context, textNFC, speaker string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.speakerAudio[speaker+"\x00"+textNFC], nil
}

// Actor returns the actor stored on the latest revision.
// The empty string means the id is unknown.
func (m *Memory) Actor(id int64) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byID(id)
	if u == nil {
		return ""
	}
	return u.actor
}

// SetPlacement stores one placement row for userID.
// known is the Good and Easy words. learning is the Again, Hard, and
// don't-know words. Both are pointed NFC and are sorted before they
// are stored.
func (m *Memory) SetPlacement(userID int64, row Placement, known, learning []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.placement == nil {
		m.placement = map[int64]storedPlacement{}
	}
	row.BandScores = append([]BandScore(nil), row.BandScores...)
	m.placement[userID] = storedPlacement{
		row:      row,
		known:    sortedCopy(known),
		learning: sortedCopy(learning),
	}
}

// ListUnits returns every saved unit, ordered by position then id.
// get_context reads UnitsInScope. Tests use this full list.
func (m *Memory) ListUnits(ctx context.Context) ([]UnitOutline, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]UnitOutline, len(m.units))
	for i, u := range m.units {
		out[i] = UnitOutline{ID: u.id, Position: u.position, Title: u.title, Status: u.status, Scope: u.scope}
	}
	slices.SortFunc(out, func(a, b UnitOutline) int {
		if c := cmp.Compare(a.Position, b.Position); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out, nil
}

// UnitsInScope implements CourseReader.
// A nil learnerID is shared units only.
func (m *Memory) UnitsInScope(ctx context.Context, learnerID *int64) ([]UnitOutline, error) {
	rows, err := m.ListUnits(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]UnitOutline, 0, len(rows))
	for _, row := range rows {
		if row.Scope == ScopeShared || (learnerID != nil && row.Scope == fmt.Sprintf("learner:%d", *learnerID)) {
			out = append(out, row)
		}
	}
	return out, nil
}

// LatestPlacement implements CourseReader.
// ok is false when userID has no placement row.
func (m *Memory) LatestPlacement(ctx context.Context, userID int64) (Placement, bool, error) {
	if err := ctx.Err(); err != nil {
		return Placement{}, false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.placement[userID]
	if !ok {
		return Placement{}, false, nil
	}
	row := stored.row
	row.BandScores = append([]BandScore(nil), row.BandScores...)
	return row, true, nil
}

// PlacementKnownLexemes implements CourseReader.
// The text is the Good and Easy words, pointed NFC, sorted.
// A learner with no run returns an empty list.
func (m *Memory) PlacementKnownLexemes(ctx context.Context, userID int64) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.placement[userID]
	if !ok || len(stored.known) == 0 {
		return []string{}, nil
	}
	return append([]string(nil), stored.known...), nil
}

// PlacementLearningLexemes implements CourseReader.
// The text is the Again, Hard, and don't-know words, pointed NFC, sorted.
// A learner with no row returns an empty list.
func (m *Memory) PlacementLearningLexemes(ctx context.Context, userID int64) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.placement[userID]
	if !ok || len(stored.learning) == 0 {
		return []string{}, nil
	}
	return append([]string(nil), stored.learning...), nil
}

// SetStruggle stores the raw struggle rows for userID.
// StruggleStats sorts the lexemes by text and ranks Top.
func (m *Memory) SetStruggle(userID int64, lexemes []StruggleLexeme, confusions []StruggleConfusion) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.struggle == nil {
		m.struggle = map[int64]storedStruggle{}
	}
	m.struggle[userID] = storedStruggle{
		lexemes:    append([]StruggleLexeme(nil), lexemes...),
		confusions: append([]StruggleConfusion(nil), confusions...),
	}
}

// StruggleStats implements CourseReader.
// Lexemes are sorted by pointed text. Top is the topN of those rows by
// misses in 7 days, then lapses, then Again rate. Confusions are the
// rows stored for this learner. The store's confusion list is 7 days.
func (m *Memory) StruggleStats(ctx context.Context, userID int64, topN int) (Struggle, error) {
	if err := ctx.Err(); err != nil {
		return Struggle{}, err
	}
	if topN <= 0 {
		topN = defaultStruggleTop
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	stored := m.struggle[userID]
	rows := make([]StruggleLexeme, len(stored.lexemes))
	for i, row := range stored.lexemes {
		rows[i] = cloneStruggleLexeme(row)
	}
	slices.SortFunc(rows, func(a, b StruggleLexeme) int {
		return cmp.Compare(a.TextNFC, b.TextNFC)
	})
	ranked := append([]StruggleLexeme(nil), rows...)
	slices.SortStableFunc(ranked, func(a, b StruggleLexeme) int {
		if c := cmp.Compare(b.Misses7d, a.Misses7d); c != 0 {
			return c
		}
		if c := cmp.Compare(b.Lapses, a.Lapses); c != 0 {
			return c
		}
		return cmp.Compare(b.AgainRate, a.AgainRate)
	})
	if topN < len(ranked) {
		ranked = ranked[:topN]
	}
	top := make([]string, len(ranked))
	for i, row := range ranked {
		top[i] = row.TextNFC
	}
	confusions := append([]StruggleConfusion(nil), stored.confusions...)
	if confusions == nil {
		confusions = []StruggleConfusion{}
	}
	if rows == nil {
		rows = []StruggleLexeme{}
	}
	return Struggle{Lexemes: rows, Confusions: confusions, Top: top}, nil
}

func cloneStruggleLexeme(row StruggleLexeme) StruggleLexeme {
	row.ConfusionKeys = append([]string(nil), row.ConfusionKeys...)
	if row.ConfusionKeys == nil {
		row.ConfusionKeys = []string{}
	}
	return row
}

// SetCompletedUnitLexemes stores pointed NFC words from units this
// learner has completed. Those words are not known. Known is a Good
// or Easy review, including a placement rating of Good or Easy.
func (m *Memory) SetCompletedUnitLexemes(userID int64, words []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.completedWords == nil {
		m.completedWords = map[int64][]string{}
	}
	m.completedWords[userID] = append([]string(nil), words...)
}

// LearnerKnownLexemes implements CourseReader.
func (m *Memory) LearnerKnownLexemes(ctx context.Context, userID int64) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var parts []string
	if stored, ok := m.placement[userID]; ok {
		parts = append(parts, stored.known...)
	}
	words := sortedCopy(parts)
	if words == nil {
		words = []string{}
	}
	return words, nil
}

// SetCompletedSharedUnits stores one learner's finished shared units.
func (m *Memory) SetCompletedSharedUnits(userID int64, rows []UnitCompletion) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.completed == nil {
		m.completed = map[int64][]UnitCompletion{}
	}
	m.completed[userID] = append([]UnitCompletion(nil), rows...)
}

// CompletedSharedUnits implements CourseReader.
func (m *Memory) CompletedSharedUnits(ctx context.Context, userID int64) ([]UnitCompletion, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rows := append([]UnitCompletion(nil), m.completed[userID]...)
	slices.SortFunc(rows, func(a, b UnitCompletion) int {
		if c := cmp.Compare(a.Position, b.Position); c != 0 {
			return c
		}
		return cmp.Compare(a.UnitID, b.UnitID)
	})
	if rows == nil {
		rows = []UnitCompletion{}
	}
	return rows, nil
}

func sortedCopy(in []string) []string {
	words := append([]string(nil), in...)
	slices.Sort(words)
	return slices.Compact(words)
}

// SearchLexemes implements LexiconReader.
func (m *Memory) SearchLexemes(ctx context.Context, query string, limit int) ([]Lexeme, []Lexeme, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	pointed, bare, glossQ := foldQuery(query)
	m.mu.Lock()
	lexemes := append([]storedLexeme(nil), m.lexemes...)
	m.mu.Unlock()

	var hits, suggestions []Lexeme
	for _, row := range lexemes {
		lx := row.lx
		stored, storedBare, _ := foldQuery(lx.TextNFC)
		if stored == pointed || (bare != "" && (lx.Bare == bare || storedBare == bare)) || glossEquals(lx.Gloss, glossQ) {
			hits = append(hits, lx)
			continue
		}
		if bareSuggest(lx.Bare, bare) || bareSuggest(storedBare, bare) || glossContains(lx.Gloss, glossQ) {
			suggestions = append(suggestions, lx)
		}
	}
	slices.SortFunc(hits, func(a, b Lexeme) int { return cmp.Compare(a.TextNFC, b.TextNFC) })
	slices.SortFunc(suggestions, func(a, b Lexeme) int { return cmp.Compare(a.TextNFC, b.TextNFC) })
	if len(hits) > 0 {
		suggestions = nil
	}
	return clip(hits, limit), clip(suggestions, limit), nil
}

// PutUnit implements UnitWriter.
// A scope-only change appends a revision. Actor comes from ctx.
func (m *Memory) PutUnit(ctx context.Context, id *int64, doc *units.Unit, scope string) (PutResult, error) {
	if err := ctx.Err(); err != nil {
		return PutResult{}, err
	}
	if doc == nil {
		return PutResult{}, errMissingDoc
	}
	if doc.Position < 0 {
		return PutResult{}, errBadPosition
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return PutResult{}, err
	}
	stored, err := cloneUnit(raw)
	if err != nil {
		return PutResult{}, err
	}
	actor := actorFrom(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	if id == nil {
		m.nextID++
		u := memUnit{
			id:        m.nextID,
			position:  doc.Position,
			title:     doc.Title,
			status:    "draft",
			scope:     scope,
			rev:       1,
			canonical: raw,
			doc:       stored,
			actor:     actor,
		}
		u.remember()
		m.units = append(m.units, u)
		return PutResult{UnitID: m.nextID, Revision: 1}, nil
	}
	u := m.byID(*id)
	if u == nil {
		return PutResult{}, ErrUnitNotFound
	}
	if u.position == doc.Position && u.scope == scope && bytes.Equal(u.canonical, raw) {
		return PutResult{UnitID: u.id, Revision: u.rev, Unchanged: true}, nil
	}
	u.rev = u.maxRev() + 1
	u.position = doc.Position
	u.title = doc.Title
	u.status = "draft"
	u.scope = scope
	u.canonical = append([]byte(nil), raw...)
	u.doc = stored
	u.actor = actor
	u.remember()
	return PutResult{UnitID: u.id, Revision: u.rev}, nil
}

func (u *memUnit) remember() {
	u.revisions = append(u.revisions, memRev{
		rev:       u.rev,
		canonical: append([]byte(nil), u.canonical...),
	})
}

func (u *memUnit) maxRev() int {
	max := u.rev
	for _, rev := range u.revisions {
		if rev.rev > max {
			max = rev.rev
		}
	}
	return max
}

func (u *memUnit) revision(n int) (memRev, bool) {
	for _, rev := range u.revisions {
		if rev.rev == n {
			return rev, true
		}
	}
	return memRev{}, false
}

// LoadUnit implements UnitWriter. A nil revision is the current row.
func (m *Memory) LoadUnit(ctx context.Context, id int64, revision *int) (UnitView, error) {
	if err := ctx.Err(); err != nil {
		return UnitView{}, err
	}
	if revision != nil && *revision < 1 {
		return UnitView{}, errors.New("revision must be a positive revision number")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byID(id)
	if u == nil {
		return UnitView{}, ErrUnitNotFound
	}
	raw := u.canonical
	rev := u.rev
	if revision != nil {
		stored, ok := u.revision(*revision)
		if !ok {
			return UnitView{}, ErrRevisionNotFound
		}
		raw = stored.canonical
		rev = stored.rev
	}
	doc, err := cloneUnit(raw)
	if err != nil {
		return UnitView{}, err
	}
	doc.Position = u.position
	return UnitView{
		UnitID:          u.id,
		Revision:        rev,
		CurrentRevision: u.rev,
		Position:        u.position,
		Scope:           u.scope,
		Status:          u.status,
		Doc:             doc,
	}, nil
}

// PublishUnit implements UnitWriter. It restores a stored revision onto
// the current row and does not append a revision. A nil revision publishes
// the current row. The same published revision writes nothing.
func (m *Memory) PublishUnit(ctx context.Context, id int64, revision *int) (PutResult, error) {
	if err := ctx.Err(); err != nil {
		return PutResult{}, err
	}
	if revision != nil && *revision < 1 {
		return PutResult{}, errors.New("revision must be a positive revision number")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byID(id)
	if u == nil {
		return PutResult{}, ErrUnitNotFound
	}
	raw := u.canonical
	rev := u.rev
	if revision != nil {
		stored, ok := u.revision(*revision)
		if !ok {
			return PutResult{}, ErrRevisionNotFound
		}
		raw = stored.canonical
		rev = stored.rev
	}
	if u.status == "published" && u.rev == rev && bytes.Equal(u.canonical, raw) {
		return PutResult{UnitID: u.id, Revision: rev, Unchanged: true}, nil
	}
	doc, err := cloneUnit(raw)
	if err != nil {
		return PutResult{}, err
	}
	doc.Position = u.position
	u.rev = rev
	u.title = doc.Title
	u.status = "published"
	u.canonical = append([]byte(nil), raw...)
	u.doc = doc
	return PutResult{UnitID: u.id, Revision: rev}, nil
}

// SetStatus sets the stored status. Tests use it to mark a unit published.
// PutUnit leaves that status alone when the document, position, and scope
// are unchanged, and sets it back to draft when they change.
func (m *Memory) SetStatus(id int64, status string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u := m.byID(id); u != nil {
		u.status = status
	}
}

// Status returns the stored status. An unknown id returns an empty string.
func (m *Memory) Status(id int64) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u := m.byID(id); u != nil {
		return u.status
	}
	return ""
}

func (m *Memory) byID(id int64) *memUnit {
	for i := range m.units {
		if m.units[i].id == id {
			return &m.units[i]
		}
	}
	return nil
}

// UpsertLexemes implements LexiconWriter. It does not rebuild lesson_items.
// The database writer does. An invalid batch returns *lexicon.Invalid and
// leaves the stored rows as they were.
func (m *Memory) UpsertLexemes(ctx context.Context, entries []lexicon.Input) ([]LexemeResult, []int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	rows, bad := lexicon.Prepare(entries)
	if bad != nil {
		return nil, nil, bad
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	foundAt := make([]int, len(rows))
	var missing []lexicon.Issue
	for i, row := range rows {
		foundAt[i] = -1
		for j, stored := range m.lexemes {
			if units.Normalize(stored.lx.TextNFC) == row.TextNFC {
				foundAt[i] = j
				break
			}
		}
		if foundAt[i] < 0 && lexicon.TranslitBlank(row) {
			missing = append(missing, lexicon.NewTranslitIssue(i))
		}
	}
	if len(missing) > 0 {
		return nil, nil, &lexicon.Invalid{Issues: missing}
	}
	out := make([]LexemeResult, 0, len(rows))
	for i, row := range rows {
		found := foundAt[i]
		if found < 0 {
			m.nextLex++
			m.lexemes = append(m.lexemes, storedLexeme{
				id:       m.nextLex,
				lx:       memoryLexeme(Lexeme{}, row),
				verified: row.Verified != nil && *row.Verified,
			})
			out = append(out, LexemeResult{TextNFC: row.TextNFC, ID: m.nextLex, Status: lexicon.StatusCreated})
			continue
		}
		cur := m.lexemes[found]
		next := memoryLexeme(cur.lx, row)
		verified := cur.verified
		if row.Verified != nil {
			verified = *row.Verified
		}
		// Forms is a slice (INIT-023/SPEC-006) and is not part of upsert_lexemes.
		if sameMemoryLexeme(cur.lx, next) && cur.verified == verified {
			out = append(out, LexemeResult{TextNFC: row.TextNFC, ID: cur.id, Status: lexicon.StatusUnchanged})
			continue
		}
		m.lexemes[found].lx = next
		m.lexemes[found].verified = verified
		out = append(out, LexemeResult{TextNFC: row.TextNFC, ID: cur.id, Status: lexicon.StatusUpdated})
	}
	return out, []int64{}, nil
}

func sameMemoryLexeme(a, b Lexeme) bool {
	return a.TextNFC == b.TextNFC && a.Bare == b.Bare && a.Gloss == b.Gloss && a.POS == b.POS &&
		a.Gender == b.Gender && a.Plural == b.Plural && a.Translit == b.Translit &&
		a.Root == b.Root && a.Binyan == b.Binyan && a.AudioPresent == b.AudioPresent
}

// memoryLexeme keeps omitted pos, gender, plural, translit, root, and binyan.
// A set pointer replaces the stored value, including an empty string.
func memoryLexeme(cur Lexeme, row lexicon.Row) Lexeme {
	out := cur
	out.TextNFC = row.TextNFC
	out.Bare = row.Bare
	out.Gloss = row.Gloss
	if row.POS != nil {
		out.POS = *row.POS
	}
	if row.Gender != nil {
		out.Gender = *row.Gender
	}
	if row.Plural != nil {
		out.Plural = *row.Plural
	}
	if row.Translit != nil {
		out.Translit = *row.Translit
	}
	if row.Root != nil {
		out.Root = *row.Root
	}
	if row.Binyan != nil {
		out.Binyan = *row.Binyan
	}
	return out
}

// KnownLexemes implements units.Lookup. Words introduced by a published
// shared unit whose position is lower than beforePosition are known.
// A draft contributes nothing.
func (m *Memory) KnownLexemes(ctx context.Context, beforePosition int) (map[string]bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]bool{}
	for _, u := range m.units {
		if u.scope != ScopeShared || u.status != "published" || u.position >= beforePosition {
			continue
		}
		for _, w := range u.doc.NewWords {
			if strings.TrimSpace(w) != "" {
				out[w] = true
			}
		}
	}
	return out, nil
}

// HasAudio implements units.Lookup.
func (m *Memory) HasAudio(ctx context.Context, textNFC string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.audio[textNFC], nil
}

// LexemeByText implements units.Lookup. The match is the exact text_nfc key.
func (m *Memory) LexemeByText(ctx context.Context, textNFC string) (int64, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, row := range m.lexemes {
		if row.lx.TextNFC == textNFC {
			return row.id, true, nil
		}
	}
	return 0, false, nil
}

// Facts implements units.Lookup.
func (m *Memory) Facts(ctx context.Context, textNFC string) (units.LexemeFacts, bool, error) {
	if err := ctx.Err(); err != nil {
		return units.LexemeFacts{}, false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, row := range m.lexemes {
		if row.lx.TextNFC != textNFC {
			continue
		}
		return units.LexemeFacts{
			POS:             row.lx.POS,
			Gender:          row.lx.Gender,
			Plural:          row.lx.Plural,
			IrregularPlural: row.lx.Plural != "",
			Bare:            row.lx.Bare,
			Gloss:           row.lx.Gloss,
		}, true, nil
	}
	return units.LexemeFacts{}, false, nil
}

func cloneUnit(raw []byte) (units.Unit, error) {
	var doc units.Unit
	if err := json.Unmarshal(raw, &doc); err != nil {
		return units.Unit{}, err
	}
	return doc, nil
}

func clip(in []Lexeme, n int) []Lexeme {
	if len(in) == 0 {
		return []Lexeme{}
	}
	if n > len(in) {
		n = len(in)
	}
	return append([]Lexeme(nil), in[:n]...)
}

// foldQuery prepares a search string. Marks are stripped the same way the
// validator builds a consonant skeleton, so an unpointed query can hit a
// pointed entry. This is dictionary search, not validation.
func foldQuery(s string) (pointed, bare, gloss string) {
	s = norm.NFC.String(strings.Join(strings.Fields(s), " "))
	return s, stripMarks(s), strings.ToLower(s)
}

func stripMarks(s string) string {
	return strings.Map(func(r rune) rune {
		if isSearchMark(r) {
			return -1
		}
		return r
	}, s)
}

func isSearchMark(r rune) bool {
	switch {
	case r >= 0x0591 && r <= 0x05AF:
		return true
	case r >= 0x05B0 && r <= 0x05BD:
		return true
	case r == 0x05BF:
		return true
	case r == 0x05C4 || r == 0x05C5 || r == 0x05C7:
		return true
	default:
		return false
	}
}

func glossEquals(gloss, query string) bool {
	if query == "" || strings.TrimSpace(gloss) == "" {
		return false
	}
	g := strings.ToLower(strings.TrimSpace(gloss))
	if g == query {
		return true
	}
	for part := range strings.SplitSeq(g, ";") {
		if strings.TrimSpace(part) == query {
			return true
		}
	}
	return false
}

func glossContains(gloss, query string) bool {
	if len([]rune(query)) < 3 {
		return false
	}
	return strings.Contains(strings.ToLower(gloss), query)
}

func bareSuggest(bare, query string) bool {
	if bare == "" || query == "" || bare == query {
		return false
	}
	if len([]rune(query)) < 2 {
		return false
	}
	return strings.HasPrefix(bare, query)
}
