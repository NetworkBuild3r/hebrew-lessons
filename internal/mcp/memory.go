package mcp

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
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
	placement      map[int64]storedPlacement
	struggle       map[int64]storedStruggle
	completed      map[int64][]UnitCompletion
	completedWords map[int64][]string
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
}

type storedLexeme struct {
	id int64
	lx Lexeme
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
// learner has completed. LearnerKnownLexemes unions them with the
// placement Good and Easy words.
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
	parts = append(parts, m.completedWords[userID]...)
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
		m.units = append(m.units, memUnit{
			id:        m.nextID,
			position:  doc.Position,
			title:     doc.Title,
			status:    "draft",
			scope:     scope,
			rev:       1,
			canonical: raw,
			doc:       stored,
			actor:     actor,
		})
		return PutResult{UnitID: m.nextID, Revision: 1}, nil
	}
	u := m.byID(*id)
	if u == nil {
		return PutResult{}, ErrUnitNotFound
	}
	if u.position == doc.Position && u.scope == scope && bytes.Equal(u.canonical, raw) {
		return PutResult{UnitID: u.id, Revision: u.rev, Unchanged: true}, nil
	}
	u.rev++
	u.position = doc.Position
	u.title = doc.Title
	u.status = "draft"
	u.scope = scope
	u.canonical = append([]byte(nil), raw...)
	u.doc = stored
	u.actor = actor
	return PutResult{UnitID: u.id, Revision: u.rev}, nil
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
	out := make([]LexemeResult, 0, len(rows))
	for _, row := range rows {
		lx := Lexeme{
			TextNFC:  row.TextNFC,
			Bare:     row.Bare,
			Gloss:    row.Gloss,
			POS:      row.POS,
			Gender:   row.Gender,
			Plural:   row.Plural,
			Translit: row.Translit,
		}
		found := -1
		for i, stored := range m.lexemes {
			if units.Normalize(stored.lx.TextNFC) == row.TextNFC {
				found = i
				break
			}
		}
		if found < 0 {
			m.nextLex++
			m.lexemes = append(m.lexemes, storedLexeme{id: m.nextLex, lx: lx})
			out = append(out, LexemeResult{TextNFC: row.TextNFC, ID: m.nextLex, Status: lexicon.StatusCreated})
			continue
		}
		cur := m.lexemes[found]
		if memoryUnchanged(cur.lx, row) {
			out = append(out, LexemeResult{TextNFC: row.TextNFC, ID: cur.id, Status: lexicon.StatusUnchanged})
			continue
		}
		m.lexemes[found].lx = lx
		out = append(out, LexemeResult{TextNFC: row.TextNFC, ID: cur.id, Status: lexicon.StatusUpdated})
	}
	return out, []int64{}, nil
}

func memoryUnchanged(cur Lexeme, row lexicon.Row) bool {
	return cur.TextNFC == row.TextNFC && cur.Bare == row.Bare && cur.Gloss == row.Gloss && cur.POS == row.POS && cur.Gender == row.Gender && cur.Plural == row.Plural && cur.Translit == row.Translit
}

// KnownLexemes implements units.Lookup. Words introduced by a saved shared
// unit whose position is lower than beforePosition are known.
func (m *Memory) KnownLexemes(ctx context.Context, beforePosition int) (map[string]bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]bool{}
	for _, u := range m.units {
		if u.scope != ScopeShared || u.position >= beforePosition {
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
