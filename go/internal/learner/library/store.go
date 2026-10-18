// Package library is the learner's word library.
//
// List matches the read side of server/learner/library.js list and
// GET /api/me/library: the same rows, fields, filters, and JavaScript string
// order. The cursor is not Node's base64url(word). It is the keyset cursor
// documented on encodeCursor.
//
// SetSuspended matches PATCH /api/me/library/:key. That route does not write
// audit_log, and neither does SetSuspended.
//
// EditWord is not implemented. user_words.gloss_en is a personal note (shown
// as note when a lexicon row exists, and as gloss_en only when it does not).
// No per-user column stores a transliteration. lexicon_entries.translit is
// shared. Node's add() reads key and gloss_en and ignores translit.
package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	listDefault = 50
	listMax     = 200
	keyMax      = 60
	glossMax    = 100
	queryMax    = 80
	rawMax      = 500
)

var (
	markup       = regexp.MustCompile(`[<>]`)
	control      = regexp.MustCompile("[\u0000-\u001F\u007F]")
	hebrewLetter = regexp.MustCompile("[\u05D0-\u05EA]")
	errCursor    = &OpError{Code: "invalid_cursor", Status: 400}
)

// OpError is a library rejection with the same code Node returns.
type OpError struct {
	Code   string
	Status int
}

func (e *OpError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code
}

// Direction is one review direction on a library row.
type Direction struct {
	Stage          string   `json:"stage"`
	StrengthBucket *string  `json:"strength_bucket"`
	Retrievability *float64 `json:"retrievability"`
	Lapses         int      `json:"lapses"`
	DueInDays      *int     `json:"due_in_days"`
}

// Item is one library row, in the GET /api/me/library word shape.
type Item struct {
	Word           string     `json:"word"`
	GlossEN        string     `json:"gloss_en"`
	Note           *string    `json:"note"`
	Translit       string     `json:"translit"`
	POS            *string    `json:"pos"`
	Root           *string    `json:"root"`
	Binyan         *string    `json:"binyan"`
	RootClass      *string    `json:"root_class"`
	HeEN           *Direction `json:"he_en"`
	EnHE           *Direction `json:"en_he"`
	StrengthBucket *string    `json:"strength_bucket"`
	Weak           *string    `json:"weak"`
	Suspended      bool       `json:"suspended"`
	Sources        []string   `json:"sources"`
	Audio          string     `json:"audio"`
}

// Filter selects rows. Empty strings are unset.
// Q is the raw `q` parameter: List applies exactKey and lowercases it,
// then matches the word, gloss, note, and transliteration.
type Filter struct {
	Q         string
	Status    string
	Weak      string
	POS       string
	Binyan    string
	Tense     string
	Source    string
	Suspended *bool
}

// Store reads and updates one learner's library on hebrew.sqlite.
// DB may be read-only for List. SetSuspended needs a writable connection.
type Store struct {
	DB         *sql.DB
	LessonsDir string
	// Scheduler is "ladder" or "fsrs". Empty reads SRS_SCHEDULER, and an
	// unknown or empty value is ladder, matching server/learner/model.js.
	Scheduler string
	// Now is the clock for strength and due_in_days. Nil means time.Now.
	Now func() time.Time
}

// ParseQuery reads GET /api/me/library parameters the way
// server/learner/library.js parseListQuery does, except the cursor.
// Pass q["cursor"] to List. That value is this package's keyset cursor,
// not Node's base64url(word) cursor.
func ParseQuery(q map[string]string) (Filter, int, error) {
	if q == nil {
		q = map[string]string{}
	}
	filter := Filter{
		Q:      q["q"],
		Status: q["status"],
		POS:    q["pos"],
		Binyan: q["binyan"],
		Tense:  q["tense"],
		Source: q["source"],
	}
	switch q["weak"] {
	case "", "weak", "very_weak":
		filter.Weak = q["weak"]
	case "1", "true":
		filter.Weak = "any"
	case "0", "false":
		filter.Weak = "none"
	default:
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	if raw := q["suspended"]; raw != "" {
		switch raw {
		case "1", "true":
			filter.Suspended = boolPtr(true)
		case "0", "false":
			filter.Suspended = boolPtr(false)
		default:
			return Filter{}, 0, fail("invalid_filter", 400)
		}
	}
	limit := listDefault
	if raw := q["limit"]; raw != "" {
		parsed, ok := parsePositiveInt(strings.TrimSpace(raw))
		if !ok {
			return Filter{}, 0, fail("invalid_filter", 400)
		}
		if parsed > listMax {
			parsed = listMax
		}
		limit = parsed
	}
	if _, _, err := normalize(filter, limit); err != nil {
		return Filter{}, 0, err
	}
	return filter, limit, nil
}

// List returns one page of the learner's library.
//
// limit below 1 uses the Node default of 50. limit above 200 is capped at
// 200. cursor is empty to start, or the nextCursor from the previous page.
// The concatenation of pages, in order, equals one unpaged call, and a
// cursor stays on the same word when rows are inserted concurrently.
func (s *Store) List(ctx context.Context, userID int64, filter Filter, cursor string, limit int) ([]Item, string, error) {
	if ctx == nil {
		return nil, "", errors.New("library: nil context")
	}
	if s == nil || s.DB == nil {
		return nil, "", errors.New("library: nil database")
	}
	if userID <= 0 {
		return nil, "", fail("bad_request", 400)
	}
	normalized, limit, err := normalize(filter, limit)
	if err != nil {
		return nil, "", err
	}
	cur, err := decodeCursor(cursor)
	if err != nil {
		return nil, "", err
	}

	states, err := s.wordStates(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	byWord := map[string]wordState{}
	for _, state := range states {
		byWord[state.Word] = state
	}
	personal, err := s.personalByWord(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	words := unionWords(byWord, personal)
	lexByKey, err := s.lexiconFor(ctx, words)
	if err != nil {
		return nil, "", err
	}
	vocab := loadVocab(s.LessonsDir)
	formCache := map[string][]lexRow{}

	page := []Item{}
	overflow := false
	for _, word := range words {
		var state *wordState
		if found, ok := byWord[word]; ok {
			state = &found
		}
		item := buildItem(word, state, personal[word], lexByKey[word], lexByKey, vocab)
		ok, err := s.matches(ctx, item, normalized, cur, formCache, lexByKey)
		if err != nil {
			return nil, "", err
		}
		if !ok {
			continue
		}
		if len(page) == limit {
			overflow = true
			break
		}
		page = append(page, item)
	}
	next := ""
	if overflow && len(page) > 0 {
		next, err = encodeCursor(page[len(page)-1].Word)
		if err != nil {
			return nil, "", err
		}
	}
	return page, next, nil
}

// SetSuspended sets suspended on every review direction of one library word.
// The word must already have a review row. A personal-library word with no
// review row is not_found, matching server/learner/library.js setSuspended.
// The normalized word is returned. audit_log is not written.
func (s *Store) SetSuspended(ctx context.Context, userID int64, key string, suspended bool) (string, error) {
	if ctx == nil {
		return "", errors.New("library: nil context")
	}
	if s == nil || s.DB == nil {
		return "", errors.New("library: nil database")
	}
	if userID <= 0 {
		return "", fail("bad_request", 400)
	}
	checked, err := checkKey(key)
	if err != nil {
		return "", err
	}
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return "", fmt.Errorf("library: begin: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return "", fmt.Errorf("library: begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()

	inReview, err := reviewExists(ctx, conn, userID, checked)
	if err != nil {
		return "", err
	}
	if !inReview {
		// Node also checks user_words and still returns not_found when the
		// word is only in the personal library. The status is the same.
		return "", fail("not_found", 404)
	}
	flag := 0
	if suspended {
		flag = 1
	}
	if _, err := conn.ExecContext(ctx,
		`UPDATE review_items SET suspended = ? WHERE user_id = ? AND word = ?`,
		flag, userID, checked,
	); err != nil {
		return "", fmt.Errorf("library: set suspended: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return "", fmt.Errorf("library: commit: %w", err)
	}
	committed = true
	return checked, nil
}

func fail(code string, status int) *OpError {
	return &OpError{Code: code, Status: status}
}

func boolPtr(v bool) *bool { return &v }

func parsePositiveInt(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	n := 0
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
		if n > listMax*10 {
			return 0, false
		}
	}
	if n < 1 {
		return 0, false
	}
	return n, true
}

func normalize(filter Filter, limit int) (Filter, int, error) {
	if limit <= 0 {
		limit = listDefault
	}
	if limit > listMax {
		limit = listMax
	}
	if filter.Q != "" {
		if utf16Len(filter.Q) > rawMax {
			return Filter{}, 0, fail("invalid_filter", 400)
		}
		term := strings.ToLower(exactKey(filter.Q))
		if codePoints(term) > queryMax || markup.MatchString(term) || control.MatchString(term) {
			return Filter{}, 0, fail("invalid_filter", 400)
		}
		filter.Q = term
	}
	switch filter.Status {
	case "", "new", "learning", "longTerm":
	default:
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	switch filter.Weak {
	case "", "any", "none", "weak", "very_weak":
	default:
		// URL values 1/true/0/false are mapped by ParseQuery before this.
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	if filter.POS != "" && !validPOS(filter.POS) {
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	if filter.Binyan != "" && !validBinyan(filter.Binyan) {
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	if filter.Tense != "" && !validTense(filter.Tense) {
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	switch filter.Source {
	case "", "self", "ai", "lesson":
	default:
		return Filter{}, 0, fail("invalid_filter", 400)
	}
	return filter, limit, nil
}

func validPOS(value string) bool {
	switch value {
	case "noun", "verb", "adjective", "adverb", "preposition",
		"pronoun", "conjunction", "particle", "numeral", "other":
		return true
	default:
		return false
	}
}

func validBinyan(value string) bool {
	switch value {
	case "paal", "nifal", "piel", "pual", "hifil", "hufal", "hitpael":
		return true
	default:
		return false
	}
}

func validTense(value string) bool {
	switch value {
	case "infinitive", "present", "past", "future", "imperative":
		return true
	default:
		return false
	}
}

func checkKey(raw string) (string, error) {
	if utf16Len(raw) > rawMax {
		return "", fail("invalid_key", 400)
	}
	key := exactKey(raw)
	if key == "" || codePoints(key) > keyMax {
		return "", fail("invalid_key", 400)
	}
	if markup.MatchString(key) || control.MatchString(key) || !hebrewLetter.MatchString(key) {
		return "", fail("invalid_key", 400)
	}
	return key, nil
}

func (s *Store) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Store) scheduler() string {
	raw := ""
	if s != nil {
		raw = s.Scheduler
	}
	if strings.TrimSpace(raw) == "" {
		raw = os.Getenv("SRS_SCHEDULER")
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "fsrs":
		return "fsrs"
	default:
		return "ladder"
	}
}

func (s *Store) wordStates(ctx context.Context, userID int64) ([]wordState, error) {
	const query = `
		SELECT word, direction, due, step, lapses, last_result, last_at, sources,
		       stability, difficulty, reps, last_grade, suspended,
		       EXISTS (
		           SELECT 1 FROM practice_events AS e
		           WHERE e.user_id = review_items.user_id
		             AND e.word = review_items.word
		             AND e.direction = review_items.direction
		             AND e.result = 'pass'
		             AND e.kind = 'word'
		       )
		FROM review_items
		WHERE user_id = ?`
	rows, err := s.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("library: review rows: %w", err)
	}
	defer rows.Close()
	groups := map[string][]reviewRow{}
	order := []string{}
	for rows.Next() {
		var row reviewRow
		var due, lastResult, lastAt, sources sql.NullString
		var stability, difficulty sql.NullFloat64
		var lastGrade sql.NullInt64
		if err := rows.Scan(
			&row.Word, &row.Direction, &due, &row.Step, &row.Lapses,
			&lastResult, &lastAt, &sources, &stability, &difficulty,
			&row.Reps, &lastGrade, &row.Suspended, &row.PassedEvent,
		); err != nil {
			return nil, fmt.Errorf("library: review rows: %w", err)
		}
		row.Due = due.String
		row.LastResult, row.LastResultOK = lastResult.String, lastResult.Valid
		row.LastAt, row.LastAtOK = lastAt.String, lastAt.Valid && lastAt.String != ""
		row.Sources = sources.String
		row.Stability, row.StabilityOK = stability.Float64, stability.Valid
		row.Difficulty, row.DifficultyOK = difficulty.Float64, difficulty.Valid
		row.LastGrade, row.LastGradeOK = lastGrade.Int64, lastGrade.Valid
		if _, ok := groups[row.Word]; !ok {
			order = append(order, row.Word)
		}
		groups[row.Word] = append(groups[row.Word], row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("library: review rows: %w", err)
	}
	sortJS(order)
	nowMs := s.now().UnixMilli()
	scheduler := s.scheduler()
	out := make([]wordState, 0, len(order))
	for _, word := range order {
		out = append(out, wordFromRows(groups[word], scheduler, nowMs))
	}
	return out, nil
}

type personalRow struct {
	Word    string
	Source  string
	GlossEN sql.NullString
}

func (s *Store) personalByWord(ctx context.Context, userID int64) (map[string]personalRow, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT word, source, gloss_en FROM user_words WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("library: user words: %w", err)
	}
	defer rows.Close()
	out := map[string]personalRow{}
	for rows.Next() {
		var row personalRow
		if err := rows.Scan(&row.Word, &row.Source, &row.GlossEN); err != nil {
			return nil, fmt.Errorf("library: user words: %w", err)
		}
		out[row.Word] = row
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("library: user words: %w", err)
	}
	return out, nil
}

func unionWords(states map[string]wordState, personal map[string]personalRow) []string {
	seen := map[string]bool{}
	words := []string{}
	for word := range states {
		if !seen[word] {
			seen[word] = true
			words = append(words, word)
		}
	}
	for word := range personal {
		if !seen[word] {
			seen[word] = true
			words = append(words, word)
		}
	}
	sortJS(words)
	return words
}

type lexRow struct {
	Key       string
	Kind      string
	LemmaKey  sql.NullString
	GlossEN   sql.NullString
	POS       sql.NullString
	Root      sql.NullString
	Binyan    sql.NullString
	RootClass sql.NullString
	Tense     sql.NullString
	Translit  sql.NullString
}

func (s *Store) lexiconFor(ctx context.Context, words []string) (map[string]*lexRow, error) {
	byKey := map[string]*lexRow{}
	if err := s.loadLexicon(ctx, words, byKey); err != nil {
		return nil, err
	}
	missing := []string{}
	seen := map[string]bool{}
	for _, row := range byKey {
		if row.LemmaKey.Valid && row.LemmaKey.String != "" && byKey[row.LemmaKey.String] == nil && !seen[row.LemmaKey.String] {
			seen[row.LemmaKey.String] = true
			missing = append(missing, row.LemmaKey.String)
		}
	}
	if err := s.loadLexicon(ctx, missing, byKey); err != nil {
		return nil, err
	}
	return byKey, nil
}

func (s *Store) loadLexicon(ctx context.Context, keys []string, into map[string]*lexRow) error {
	const chunk = 200
	for start := 0; start < len(keys); start += chunk {
		end := start + chunk
		if end > len(keys) {
			end = len(keys)
		}
		slice := keys[start:end]
		if len(slice) == 0 {
			continue
		}
		marks := strings.TrimRight(strings.Repeat("?,", len(slice)), ",")
		args := make([]any, len(slice))
		for i, key := range slice {
			args[i] = key
		}
		query := `SELECT key, kind, lemma_key, gloss_en, pos, root, binyan, root_class, tense, translit
			FROM lexicon_entries WHERE key IN (` + marks + `)`
		rows, err := s.DB.QueryContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("library: lexicon: %w", err)
		}
		for rows.Next() {
			row := &lexRow{}
			if err := rows.Scan(
				&row.Key, &row.Kind, &row.LemmaKey, &row.GlossEN, &row.POS,
				&row.Root, &row.Binyan, &row.RootClass, &row.Tense, &row.Translit,
			); err != nil {
				rows.Close()
				return fmt.Errorf("library: lexicon: %w", err)
			}
			into[row.Key] = row
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("library: lexicon: %w", err)
		}
		rows.Close()
	}
	return nil
}

func (s *Store) formsOf(ctx context.Context, lemmaKey string, cache map[string][]lexRow) ([]lexRow, error) {
	if forms, ok := cache[lemmaKey]; ok {
		return forms, nil
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT key, kind, lemma_key, gloss_en, pos, root, binyan, root_class, tense, translit
		 FROM lexicon_entries WHERE lemma_key = ? ORDER BY key`, lemmaKey)
	if err != nil {
		return nil, fmt.Errorf("library: forms: %w", err)
	}
	defer rows.Close()
	forms := []lexRow{}
	for rows.Next() {
		var row lexRow
		if err := rows.Scan(
			&row.Key, &row.Kind, &row.LemmaKey, &row.GlossEN, &row.POS,
			&row.Root, &row.Binyan, &row.RootClass, &row.Tense, &row.Translit,
		); err != nil {
			return nil, fmt.Errorf("library: forms: %w", err)
		}
		if row.Kind == "form" {
			forms = append(forms, row)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("library: forms: %w", err)
	}
	cache[lemmaKey] = forms
	return forms, nil
}

func buildItem(word string, state *wordState, personal personalRow, entry *lexRow, lexByKey map[string]*lexRow, vocab map[string]vocabEntry) Item {
	vocabEntry, hasVocab := vocab[word]
	lemma := lemmaOf(entry, lexByKey)
	tags := tagsOf(entry, lemma)
	noteText := ""
	if personal.Word != "" && personal.GlossEN.Valid && personal.GlossEN.String != "" {
		noteText = personal.GlossEN.String
	}
	gloss := ""
	var shownNote *string
	if entry != nil {
		if entry.GlossEN.Valid {
			gloss = entry.GlossEN.String
		}
		if noteText != "" {
			shownNote = &noteText
		}
	} else if noteText != "" {
		gloss = noteText
	} else if hasVocab && vocabEntry.Meaning != "" {
		gloss = vocabEntry.Meaning
	}
	translit := firstText(nullText(entry, func(row *lexRow) sql.NullString { return row.Translit }), nullText(lemma, func(row *lexRow) sql.NullString { return row.Translit }))
	if translit == "" && hasVocab {
		translit = vocabEntry.Sound
	}
	sources := []string{}
	seen := map[string]bool{}
	if state != nil {
		for _, source := range state.Sources {
			if source == "" || seen[source] {
				continue
			}
			seen[source] = true
			sources = append(sources, source)
		}
	}
	if personal.Word != "" && personal.Source != "" && !seen[personal.Source] {
		sources = append(sources, personal.Source)
	}
	var he, en *Direction
	var strength *string
	weak := ""
	suspended := false
	if state != nil {
		he = state.HeEN
		en = state.EnHE
		var heBucket, enBucket *string
		if he != nil {
			heBucket = he.StrengthBucket
		}
		if en != nil {
			enBucket = en.StrengthBucket
		}
		strength = worseBucket(heBucket, enBucket)
		weak = state.Weak
		suspended = state.Suspended
	}
	audio := ""
	if hasVocab {
		audio = vocabEntry.Audio
	}
	return Item{
		Word:           word,
		GlossEN:        gloss,
		Note:           shownNote,
		Translit:       translit,
		POS:            tags.pos,
		Root:           tags.root,
		Binyan:         tags.binyan,
		RootClass:      tags.rootClass,
		HeEN:           he,
		EnHE:           en,
		StrengthBucket: strength,
		Weak:           strPtr(weak),
		Suspended:      suspended,
		Sources:        sources,
		Audio:          audio,
	}
}

type tagSet struct {
	pos, root, binyan, rootClass, tense *string
}

func tagsOf(entry, lemma *lexRow) tagSet {
	return tagSet{
		pos:       firstNull(field(entry, func(row *lexRow) sql.NullString { return row.POS }), field(lemma, func(row *lexRow) sql.NullString { return row.POS })),
		root:      firstNull(field(entry, func(row *lexRow) sql.NullString { return row.Root }), field(lemma, func(row *lexRow) sql.NullString { return row.Root })),
		binyan:    firstNull(field(entry, func(row *lexRow) sql.NullString { return row.Binyan }), field(lemma, func(row *lexRow) sql.NullString { return row.Binyan })),
		rootClass: firstNull(field(entry, func(row *lexRow) sql.NullString { return row.RootClass }), field(lemma, func(row *lexRow) sql.NullString { return row.RootClass })),
		tense:     firstNull(field(entry, func(row *lexRow) sql.NullString { return row.Tense })),
	}
}

func lemmaOf(entry *lexRow, lexByKey map[string]*lexRow) *lexRow {
	if entry == nil {
		return nil
	}
	if entry.Kind == "lemma" {
		return entry
	}
	if entry.Kind == "form" && entry.LemmaKey.Valid && entry.LemmaKey.String != "" {
		return lexByKey[entry.LemmaKey.String]
	}
	return nil
}

func field(row *lexRow, get func(*lexRow) sql.NullString) sql.NullString {
	if row == nil {
		return sql.NullString{}
	}
	return get(row)
}

func nullText(row *lexRow, get func(*lexRow) sql.NullString) string {
	value := field(row, get)
	if !value.Valid {
		return ""
	}
	return value.String
}

func firstText(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNull(values ...sql.NullString) *string {
	for _, value := range values {
		if value.Valid && value.String != "" {
			s := value.String
			return &s
		}
	}
	return nil
}

func strPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (s *Store) matches(ctx context.Context, row Item, filter Filter, cursor cursorBody, formCache map[string][]lexRow, lexByKey map[string]*lexRow) (bool, error) {
	if !rowAfter(row.Word, cursor) {
		return false, nil
	}
	if filter.Q != "" {
		note := ""
		if row.Note != nil {
			note = *row.Note
		}
		hay := strings.ToLower(row.Word + "\n" + row.GlossEN + "\n" + note + "\n" + row.Translit)
		if !strings.Contains(hay, filter.Q) {
			return false, nil
		}
	}
	if filter.Status != "" {
		stages := []string{}
		if row.HeEN != nil {
			stages = append(stages, row.HeEN.Stage)
		}
		if row.EnHE != nil {
			stages = append(stages, row.EnHE.Stage)
		}
		if !containsString(stages, filter.Status) {
			return false, nil
		}
	}
	switch filter.Weak {
	case "any":
		if row.Weak == nil || (*row.Weak != "weak" && *row.Weak != "very_weak") {
			return false, nil
		}
	case "none":
		if row.Weak != nil {
			return false, nil
		}
	case "weak":
		if row.Weak == nil || *row.Weak != "weak" {
			return false, nil
		}
	case "very_weak":
		if row.Weak == nil || *row.Weak != "very_weak" {
			return false, nil
		}
	}
	if filter.POS != "" && (row.POS == nil || *row.POS != filter.POS) {
		return false, nil
	}
	if filter.Binyan != "" && (row.Binyan == nil || *row.Binyan != filter.Binyan) {
		return false, nil
	}
	if filter.Tense != "" {
		ok, err := s.rowHasTense(ctx, row, filter.Tense, formCache, lexByKey)
		if err != nil || !ok {
			return false, err
		}
	}
	if filter.Source == "lesson" {
		lesson := false
		for _, source := range row.Sources {
			if source != "self" && source != "ai" && source != "lesson" {
				lesson = true
				break
			}
		}
		if !lesson {
			return false, nil
		}
	} else if filter.Source != "" && !containsString(row.Sources, filter.Source) {
		return false, nil
	}
	if filter.Suspended != nil {
		if *filter.Suspended && !row.Suspended {
			return false, nil
		}
		if !*filter.Suspended && row.Suspended {
			return false, nil
		}
	}
	return true, nil
}

func (s *Store) rowHasTense(ctx context.Context, row Item, tense string, formCache map[string][]lexRow, lexByKey map[string]*lexRow) (bool, error) {
	entry := lexByKey[row.Word]
	if entry != nil && entry.Tense.Valid && entry.Tense.String == tense {
		return true, nil
	}
	lemma := lemmaOf(entry, lexByKey)
	lemmaKey := ""
	if lemma != nil {
		lemmaKey = lemma.Key
	} else if entry != nil && entry.Kind == "lemma" {
		lemmaKey = entry.Key
	}
	if lemmaKey == "" {
		return false, nil
	}
	forms, err := s.formsOf(ctx, lemmaKey, formCache)
	if err != nil {
		return false, err
	}
	for _, form := range forms {
		if form.Tense.Valid && form.Tense.String == tense {
			return true, nil
		}
	}
	return false, nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func reviewExists(ctx context.Context, conn *sql.Conn, userID int64, word string) (bool, error) {
	for _, direction := range []string{"he_en", "en_he"} {
		var one int
		err := conn.QueryRowContext(ctx,
			`SELECT 1 FROM review_items WHERE user_id = ? AND word = ? AND direction = ?`,
			userID, word, direction,
		).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("library: find review: %w", err)
		}
		return true, nil
	}
	return false, nil
}
