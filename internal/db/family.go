package db

import (
	"context"
	"fmt"
	"sort"
)

// RootLesson is one published lesson that uses a word of a root, in a
// scope the learner can open.
type RootLesson struct {
	UnitID   int64
	LessonIx int
	Title    string
}

// RootLessons returns published lessons that teach a word of root.
// learnerScope is learner:<id>. Shared lessons are included. Another
// learner's units are not.
func (s *Store) RootLessons(ctx context.Context, root, learnerScope string) ([]RootLesson, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT u.id, MIN(li.lesson_ix), u.title
		FROM lesson_items AS li
		JOIN lexemes AS l ON l.id = li.lexeme_id
		JOIN units AS u ON u.id = li.unit_id
		WHERE l.root = ? AND u.status = 'published' AND (u.scope = 'shared' OR u.scope = ?)
		GROUP BY u.id
		ORDER BY u.position, u.id`, root, learnerScope)
	if err != nil {
		return nil, fmt.Errorf("db: root lessons: %w", err)
	}
	defer rows.Close()
	var out []RootLesson
	for rows.Next() {
		var row RootLesson
		if err := rows.Scan(&row.UnitID, &row.LessonIx, &row.Title); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// LessonUses counts lesson_items rows per lexeme. Ids that never appear
// are absent.
func (s *Store) LessonUses(ctx context.Context, ids []int64) (map[int64]int, error) {
	out := map[int64]int{}
	if len(ids) == 0 {
		return out, nil
	}
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	q, args := inClause(ids)
	rows, err := conn.QueryContext(ctx, `SELECT lexeme_id, COUNT(*) FROM lesson_items WHERE lexeme_id IN (`+q+`) GROUP BY lexeme_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("db: lesson uses: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

func inClause(ids []int64) (string, []any) {
	parts := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		parts[i] = "?"
		args[i] = id
	}
	return joinComma(parts), args
}

func joinComma(parts []string) string {
	out := parts[0]
	for _, p := range parts[1:] {
		out += ", " + p
	}
	return out
}

// VersesForWords returns up to limit verses that contain any plain as a
// whole token. Isaiah comes first, then canonical book order. The read is
// the same full pass VersesContaining already uses. There is no token index.
func (s *Store) VersesForWords(ctx context.Context, plains []string, limit int) ([]Verse, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, fmt.Errorf("db: tanakh limit must be positive")
	}
	needles := map[string]bool{}
	for _, p := range plains {
		n, ok := singleToken(p)
		if ok {
			needles[n] = true
		}
	}
	if len(needles) == 0 {
		return []Verse{}, nil
	}
	rows, err := conn.QueryContext(ctx, `SELECT `+verseSelect+` FROM tanakh_verses`)
	if err != nil {
		return nil, fmt.Errorf("db: tanakh search: %w", err)
	}
	defer rows.Close()
	all, err := scanVerses(rows)
	if err != nil {
		return nil, err
	}
	matched := make([]Verse, 0)
	for _, v := range all {
		for _, tok := range plainTokens(v.HePlain) {
			if needles[tok] {
				matched = append(matched, v)
				break
			}
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		a, b := matched[i], matched[j]
		if ra, rb := familyRank(a.Book), familyRank(b.Book); ra != rb {
			return ra < rb
		}
		if a.Chapter != b.Chapter {
			return a.Chapter < b.Chapter
		}
		return a.Verse < b.Verse
	})
	if len(matched) > limit {
		matched = matched[:limit]
	}
	return matched, nil
}

// familyRank puts Isaiah ahead of the rest of the canon.
func familyRank(book string) int {
	if book == "isaiah" {
		return -1
	}
	if n, ok := bookOrder[book]; ok {
		return n
	}
	return len(bookOrder) + 1
}
