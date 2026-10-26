// Package roots builds the page for one Hebrew root: the words that share
// it, the lessons that teach them, and up to ten verses, Isaiah first.
package roots

import (
	"context"
	"sort"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// VerseCap is how many verses a root page shows.
const VerseCap = 10

// Word is one lexicon entry of the family, with the learner's state.
type Word struct {
	ID       int64
	Text     string
	Translit string
	Gloss    string
	POS      string
	Binyan   string
	State    string
}

// Lesson is a published lesson the learner can open that uses the root.
type Lesson struct {
	UnitID   int64
	LessonIx int
	Title    string
}

// Verse is one tanakh verse. Spans mark the family's words.
type Verse struct {
	Book    string
	Chapter int
	Verse   int
	En      string
	Spans   []Span
}

// Span is a piece of the pointed verse. Mark is true when it is one of
// the family's words.
type Span struct {
	Text string
	Mark bool
}

// Form is one cell of a verb that belongs to the family.
type Form struct {
	Tense    string
	Person   string
	Number   string
	Gender   string
	He       string
	Translit string
	Known    bool
}

// Page is everything the root page shows. Forms is empty until verb forms
// are stored for a word of the family.
type Page struct {
	Root      string
	CoreGloss string
	Words     []Word
	Forms     []Form
	Lessons   []Lesson
	Verses    []Verse
}

// Summary is one row of the roots index.
type Summary struct {
	Root      string
	Words     int
	Verses    int
	CoreGloss string
}

// Store is the database the family reads.
type Store interface {
	LexemesByRoot(ctx context.Context, root string) ([]db.LexemeView, error)
	RootCounts(ctx context.Context) (map[string]int, error)
	RootLessons(ctx context.Context, root, learnerScope string) ([]db.RootLesson, error)
	LessonUses(ctx context.Context, ids []int64) (map[int64]int, error)
	VersesForWords(ctx context.Context, plains []string, limit int) ([]db.Verse, error)
	// VerseGroupCounts is one pass over the Tanakh for every group.
	// The count for a group stops at limit.
	VerseGroupCounts(ctx context.Context, groups map[string][]string, limit int) (map[string]int, error)
}

// formReader is optional. A store with verb forms fills Page.Forms.
type formReader interface {
	VerbFormsFor(ctx context.Context, lemmaID int64) ([]db.VerbForm, error)
}

// Family loads one root for one learner. Forms come from the verb-form
// table when the store has one. A missing model leaves every state unseen.
func Family(ctx context.Context, store Store, model *learner.Model, root string, userID int64) (Page, error) {
	words, err := store.LexemesByRoot(ctx, root)
	if err != nil {
		return Page{}, err
	}
	ids := make([]int64, len(words))
	plains := make([]string, len(words))
	for i, w := range words {
		ids[i] = w.ID
		plains[i] = units.Plain(w.Text)
	}
	uses, err := store.LessonUses(ctx, ids)
	if err != nil {
		return Page{}, err
	}
	scope := ""
	if userID > 0 {
		scope = db.LearnerScope(userID)
	}
	lessons, err := store.RootLessons(ctx, root, scope)
	if err != nil {
		return Page{}, err
	}
	verses, err := store.VersesForWords(ctx, plains, VerseCap)
	if err != nil {
		return Page{}, err
	}
	plainSet := map[string]bool{}
	for _, p := range plains {
		if p != "" {
			plainSet[p] = true
		}
	}
	out := Page{Root: root, Forms: []Form{}, Words: make([]Word, len(words))}
	met := map[int64]bool{}
	bestUses := -1
	for i, w := range words {
		state := string(learner.Unseen)
		if model != nil && userID > 0 {
			ws, err := model.Word(ctx, userID, w.ID)
			if err == nil && ws.State != "" {
				state = string(ws.State)
				if ws.State == learner.Learning || ws.State == learner.Known || ws.State == learner.Solid {
					met[w.ID] = true
				}
			}
		}
		out.Words[i] = Word{
			ID: w.ID, Text: w.Text, Translit: w.Translit, Gloss: w.Gloss,
			POS: w.POS, Binyan: w.Binyan, State: state,
		}
		if n := uses[w.ID]; n > bestUses {
			bestUses = n
			out.CoreGloss = w.Gloss
		}
	}
	if out.CoreGloss == "" && len(words) > 0 {
		out.CoreGloss = words[0].Gloss
	}
	if reader, ok := store.(formReader); ok {
		for _, w := range out.Words {
			rows, err := reader.VerbFormsFor(ctx, w.ID)
			if err != nil {
				return Page{}, err
			}
			for _, row := range rows {
				out.Forms = append(out.Forms, Form{
					Tense: row.Tense, Person: row.Person, Number: row.Number, Gender: row.Gender,
					He: row.FormHe, Translit: row.Translit, Known: row.FormLexemeID > 0 && met[row.FormLexemeID],
				})
			}
		}
	}
	for _, les := range lessons {
		out.Lessons = append(out.Lessons, Lesson{UnitID: les.UnitID, LessonIx: les.LessonIx, Title: les.Title})
	}
	if out.Lessons == nil {
		out.Lessons = []Lesson{}
	}
	for _, v := range verses {
		out.Verses = append(out.Verses, Verse{
			Book: v.Book, Chapter: v.Chapter, Verse: v.Verse, En: v.En,
			Spans: markVerse(v.He, plainSet),
		})
	}
	if out.Verses == nil {
		out.Verses = []Verse{}
	}
	return out, nil
}

func markVerse(he string, plains map[string]bool) []Span {
	toks := units.Tokenize(he)
	if len(toks) == 0 {
		if he == "" {
			return nil
		}
		return []Span{{Text: he}}
	}
	var spans []Span
	rest := he
	for _, tok := range toks {
		i := indexOf(rest, tok)
		if i < 0 {
			continue
		}
		if i > 0 {
			spans = append(spans, Span{Text: rest[:i]})
		}
		spans = append(spans, Span{Text: tok, Mark: plains[units.Plain(tok)]})
		rest = rest[i+len(tok):]
	}
	if rest != "" {
		spans = append(spans, Span{Text: rest})
	}
	return spans
}

func indexOf(s, part string) int {
	if part == "" {
		return 0
	}
	for i := 0; i+len(part) <= len(s); i++ {
		if s[i:i+len(part)] == part {
			return i
		}
	}
	return -1
}

// List returns roots that have a page: at least two words, or one word and
// at least one verse. Verse totals come from one Tanakh pass.
func List(ctx context.Context, store Store) ([]Summary, error) {
	counts, err := store.RootCounts(ctx)
	if err != nil {
		return nil, err
	}
	words := map[string][]db.LexemeView{}
	groups := map[string][]string{}
	for root, n := range counts {
		if n < 1 {
			continue
		}
		list, err := store.LexemesByRoot(ctx, root)
		if err != nil {
			return nil, err
		}
		words[root] = list
		groups[root] = plainsOf(list)
	}
	hits, err := store.VerseGroupCounts(ctx, groups, VerseCap)
	if err != nil {
		return nil, err
	}
	var out []Summary
	for root, n := range counts {
		verses := hits[root]
		if n < 2 && verses < 1 {
			continue
		}
		gloss, err := coreGloss(ctx, store, words[root])
		if err != nil {
			return nil, err
		}
		out = append(out, Summary{Root: root, Words: n, Verses: verses, CoreGloss: gloss})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Root < out[j].Root })
	return out, nil
}

// Any reports whether any root has a page. A family of two or more words
// answers yes before the Tanakh is read. The nav uses this. The index is List.
func Any(ctx context.Context, store Store) (bool, error) {
	counts, err := store.RootCounts(ctx)
	if err != nil {
		return false, err
	}
	singles := map[string][]string{}
	for root, n := range counts {
		if n >= 2 {
			return true, nil
		}
		if n != 1 {
			continue
		}
		words, err := store.LexemesByRoot(ctx, root)
		if err != nil {
			return false, err
		}
		singles[root] = plainsOf(words)
	}
	if len(singles) == 0 {
		return false, nil
	}
	hits, err := store.VerseGroupCounts(ctx, singles, 1)
	if err != nil {
		return false, err
	}
	for _, n := range hits {
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}

func plainsOf(words []db.LexemeView) []string {
	out := make([]string, len(words))
	for i, w := range words {
		out[i] = units.Plain(w.Text)
	}
	return out
}

func coreGloss(ctx context.Context, store Store, words []db.LexemeView) (string, error) {
	if len(words) == 0 {
		return "", nil
	}
	ids := make([]int64, len(words))
	for i, w := range words {
		ids[i] = w.ID
	}
	uses, err := store.LessonUses(ctx, ids)
	if err != nil {
		return "", err
	}
	best, gloss := -1, words[0].Gloss
	for _, w := range words {
		if uses[w.ID] > best {
			best = uses[w.ID]
			gloss = w.Gloss
		}
	}
	return gloss, nil
}

func verseCount(ctx context.Context, store Store, root string) (int, error) {
	words, err := store.LexemesByRoot(ctx, root)
	if err != nil {
		return 0, err
	}
	hits, err := store.VerseGroupCounts(ctx, map[string][]string{root: plainsOf(words)}, VerseCap)
	if err != nil {
		return 0, err
	}
	return hits[root], nil
}

// Visible reports whether a root page exists.
func Visible(ctx context.Context, store Store, root string) (bool, error) {
	if !ValidPath(root) {
		return false, nil
	}
	counts, err := store.RootCounts(ctx)
	if err != nil {
		return false, err
	}
	n := counts[root]
	if n >= 2 {
		return true, nil
	}
	if n == 1 {
		v, err := verseCount(ctx, store, root)
		return v >= 1, err
	}
	return false, nil
}

// ValidPath is a root as stored: Hebrew letters, shin and sin dots, and maqaf.
func ValidPath(root string) bool {
	if root == "" || utf8.RuneCountInString(root) > 24 {
		return false
	}
	for _, r := range root {
		switch {
		case r == 0x05BE || r == 0x05C1 || r == 0x05C2:
		case r >= 0x05D0 && r <= 0x05EA:
		default:
			return false
		}
	}
	return true
}
