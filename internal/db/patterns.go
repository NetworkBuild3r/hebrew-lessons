package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrPatternCycle means a prerequisite write would loop and was rolled back.
var ErrPatternCycle = errors.New("db: pattern prerequisites cycle")

// PatternInput is one grammar pattern and the patterns that must come first.
// Prereqs replaces the stored edges for this slug.
type PatternInput struct {
	Slug    string
	Title   string
	Summary string
	Example string
	Prereqs []string
}

// PatternInfo is a stored pattern plus whether its explainer is published.
type PatternInfo struct {
	Slug               string
	Title              string
	Summary            string
	Example            string
	Prereqs            []string
	ExplainerPublished bool
}

// Explainer is the note for one pattern.
type Explainer struct {
	Slug     string
	Title    string
	Body     string
	Status   string
	Revision int
}

// PatternUnit is a unit that may carry a pattern, in no particular order.
type PatternUnit struct {
	ID       int64
	Position int
	Scope    string
	Title    string
	Pattern  string
	Status   string
}

// UpsertPatterns writes the batch in one transaction. A cycle, a self-edge,
// an unknown prerequisite, or a bad slug writes nothing.
func (s *Store) UpsertPatterns(ctx context.Context, in []PatternInput) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if len(in) == 0 {
		return errors.New("db: no patterns")
	}
	if len(in) > 100 {
		return errors.New("db: at most 100 patterns")
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin patterns: %w", err)
	}
	defer tx.Rollback()
	if err := upsertPatternsTx(ctx, tx, in, s.clock.Now()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit patterns: %w", err)
	}
	return nil
}

func upsertPatternsTx(ctx context.Context, tx *sql.Tx, in []PatternInput, now time.Time) error {
	known, edges, err := loadPatternGraph(ctx, tx)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, p := range in {
		if !validPatternSlug(p.Slug) {
			return fmt.Errorf("db: pattern slug %q", p.Slug)
		}
		if seen[p.Slug] {
			return fmt.Errorf("db: pattern %s is repeated", p.Slug)
		}
		seen[p.Slug] = true
		if err := validPatternText(p); err != nil {
			return err
		}
		known[p.Slug] = true
		edges[p.Slug] = append([]string(nil), p.Prereqs...)
	}
	for slug, reqs := range edges {
		for _, req := range reqs {
			if req == slug {
				return fmt.Errorf("db: pattern %s requires itself", slug)
			}
			if !known[req] {
				return fmt.Errorf("db: pattern %s requires unknown %s", slug, req)
			}
		}
	}
	if patternCycle(known, edges) {
		return ErrPatternCycle
	}
	stamp := FormatTime(now)
	for _, p := range in {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO grammar_patterns (slug, title_en, summary_en, example_he, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(slug) DO UPDATE SET
				title_en = excluded.title_en,
				summary_en = excluded.summary_en,
				example_he = excluded.example_he,
				updated_at = excluded.updated_at`,
			p.Slug, p.Title, p.Summary, p.Example, stamp, stamp); err != nil {
			return fmt.Errorf("db: upsert pattern %s: %w", p.Slug, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM pattern_prereqs WHERE pattern_slug = ?`, p.Slug); err != nil {
			return fmt.Errorf("db: clear prereqs %s: %w", p.Slug, err)
		}
		for _, req := range p.Prereqs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO pattern_prereqs (pattern_slug, requires_slug) VALUES (?, ?)`,
				p.Slug, req); err != nil {
				return fmt.Errorf("db: prereq %s: %w", p.Slug, err)
			}
		}
	}
	return nil
}

func validPatternSlug(slug string) bool {
	if len(slug) < 1 || len(slug) > 60 {
		return false
	}
	for _, r := range slug {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func validPatternText(p PatternInput) error {
	if n := len(p.Title); n < 1 || n > 200 {
		return fmt.Errorf("db: pattern %s title length", p.Slug)
	}
	if n := len(p.Summary); n < 1 || n > 300 {
		return fmt.Errorf("db: pattern %s summary length", p.Slug)
	}
	if len(p.Example) > 200 {
		return fmt.Errorf("db: pattern %s example length", p.Slug)
	}
	return nil
}

func loadPatternGraph(ctx context.Context, tx *sql.Tx) (map[string]bool, map[string][]string, error) {
	known := map[string]bool{}
	rows, err := tx.QueryContext(ctx, `SELECT slug FROM grammar_patterns`)
	if err != nil {
		return nil, nil, fmt.Errorf("db: patterns: %w", err)
	}
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			rows.Close()
			return nil, nil, err
		}
		known[slug] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()
	edges := map[string][]string{}
	req, err := tx.QueryContext(ctx, `SELECT pattern_slug, requires_slug FROM pattern_prereqs`)
	if err != nil {
		return nil, nil, fmt.Errorf("db: prereqs: %w", err)
	}
	defer req.Close()
	for req.Next() {
		var slug, need string
		if err := req.Scan(&slug, &need); err != nil {
			return nil, nil, err
		}
		edges[slug] = append(edges[slug], need)
	}
	return known, edges, req.Err()
}

func patternCycle(known map[string]bool, edges map[string][]string) bool {
	color := map[string]int{}
	var walk func(string) bool
	walk = func(n string) bool {
		switch color[n] {
		case 1:
			return true
		case 2:
			return false
		}
		color[n] = 1
		for _, m := range edges[n] {
			if walk(m) {
				return true
			}
		}
		color[n] = 2
		return false
	}
	for n := range known {
		if walk(n) {
			return true
		}
	}
	return false
}

// Patterns lists every pattern. Order is by slug. Callers that want path
// order sort the result.
func (s *Store) Patterns(ctx context.Context) ([]PatternInfo, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT slug, title_en, summary_en, example_he,
			EXISTS(SELECT 1 FROM explainers e WHERE e.pattern_slug = grammar_patterns.slug AND e.status = 'published')
		FROM grammar_patterns
		ORDER BY slug`)
	if err != nil {
		return nil, fmt.Errorf("db: patterns: %w", err)
	}
	defer rows.Close()
	var out []PatternInfo
	for rows.Next() {
		var p PatternInfo
		var published int
		if err := rows.Scan(&p.Slug, &p.Title, &p.Summary, &p.Example, &published); err != nil {
			return nil, err
		}
		p.ExplainerPublished = published == 1
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	reqs, err := s.Prereqs(ctx)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Prereqs = append([]string(nil), reqs[out[i].Slug]...)
		if out[i].Prereqs == nil {
			out[i].Prereqs = []string{}
		}
	}
	return out, nil
}

// Prereqs maps each pattern to the slugs it requires. A pattern with none
// is absent.
func (s *Store) Prereqs(ctx context.Context) (map[string][]string, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT pattern_slug, requires_slug FROM pattern_prereqs ORDER BY pattern_slug, requires_slug`)
	if err != nil {
		return nil, fmt.Errorf("db: prereqs: %w", err)
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var slug, need string
		if err := rows.Scan(&slug, &need); err != nil {
			return nil, err
		}
		out[slug] = append(out[slug], need)
	}
	return out, rows.Err()
}

// HasPattern reports whether the slug is stored.
func (s *Store) HasPattern(ctx context.Context, slug string) (bool, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return false, err
	}
	var n int
	err = conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM grammar_patterns WHERE slug = ?`, slug).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("db: pattern %s: %w", slug, err)
	}
	return n == 1, nil
}

// PatternUnits reads every unit, with an empty pattern when none is set.
func (s *Store) PatternUnits(ctx context.Context) ([]PatternUnit, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, position, scope, title, status, COALESCE(pattern_slug, '')
		FROM units
		ORDER BY position, id`)
	if err != nil {
		return nil, fmt.Errorf("db: pattern units: %w", err)
	}
	defer rows.Close()
	var out []PatternUnit
	for rows.Next() {
		var u PatternUnit
		if err := rows.Scan(&u.ID, &u.Position, &u.Scope, &u.Title, &u.Status, &u.Pattern); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpsertExplainer writes one explainer. The body is stored as given; the
// caller has already parsed it. Revision starts at 1 and increments.
func (s *Store) UpsertExplainer(ctx context.Context, row Explainer) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if row.Status != "draft" && row.Status != "published" {
		return fmt.Errorf("db: explainer status %q", row.Status)
	}
	if len(row.Title) < 1 || len(row.Title) > 200 || len(row.Body) < 1 || len(row.Body) > 20000 {
		return errors.New("db: explainer text length")
	}
	stamp := FormatTime(s.clock.Now())
	res, err := conn.ExecContext(ctx, `
		INSERT INTO explainers (pattern_slug, title_en, body, status, revision, updated_at)
		VALUES (?, ?, ?, ?, 1, ?)
		ON CONFLICT(pattern_slug) DO UPDATE SET
			title_en = excluded.title_en,
			body = excluded.body,
			status = excluded.status,
			revision = explainers.revision + 1,
			updated_at = excluded.updated_at`,
		row.Slug, row.Title, row.Body, row.Status, stamp)
	if err != nil {
		return fmt.Errorf("db: explainer %s: %w", row.Slug, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("db: explainer %s: %w", row.Slug, ErrNotFound)
	}
	return nil
}

// ExplainerFor reads one explainer. publishedOnly hides a draft.
func (s *Store) ExplainerFor(ctx context.Context, slug string, publishedOnly bool) (Explainer, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return Explainer{}, err
	}
	q := `SELECT pattern_slug, title_en, body, status, revision FROM explainers WHERE pattern_slug = ?`
	if publishedOnly {
		q += ` AND status = 'published'`
	}
	var row Explainer
	err = conn.QueryRowContext(ctx, q, slug).Scan(&row.Slug, &row.Title, &row.Body, &row.Status, &row.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		return Explainer{}, ErrNotFound
	}
	if err != nil {
		return Explainer{}, fmt.Errorf("db: explainer %s: %w", slug, err)
	}
	return row, nil
}

// ExplainerCount is how many explainer rows exist.
func (s *Store) ExplainerCount(ctx context.Context) (int, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return 0, err
	}
	var n int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM explainers`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// SetUnitMeta sets the pattern and worksheet columns that are non-nil.
// An empty pattern clears it. An empty worksheet type clears it.
func (s *Store) SetUnitMeta(ctx context.Context, id int64, pattern, worksheet *string) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if pattern == nil && worksheet == nil {
		return nil
	}
	var sets []string
	var args []any
	if pattern != nil {
		sets = append(sets, "pattern_slug = ?")
		if strings.TrimSpace(*pattern) == "" {
			args = append(args, nil)
		} else {
			args = append(args, *pattern)
		}
	}
	if worksheet != nil {
		sets = append(sets, "worksheet_type = ?")
		args = append(args, *worksheet)
	}
	args = append(args, id)
	res, err := conn.ExecContext(ctx, `UPDATE units SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("db: unit meta: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUnitNotFound
	}
	return nil
}

// AuditContent appends one publish_audit row for a pattern or explainer write.
// unit_id stays null. revision is 1 because the column requires it.
func (s *Store) AuditContent(ctx context.Context, actor, action, scope string) error {
	conn, err := s.connOrErr()
	if err != nil {
		return err
	}
	if actor == "" {
		actor = "mcp"
	}
	if scope == "" {
		scope = "shared"
	}
	_, err = conn.ExecContext(ctx, `
		INSERT INTO publish_audit (revision, scope, actor, action, issues_json, created_at)
		VALUES (1, ?, ?, ?, '[]', ?)`,
		scope, actor, action, FormatTime(s.clock.Now()))
	if err != nil {
		return fmt.Errorf("db: audit %s: %w", action, err)
	}
	return nil
}

// PublishedExplainers returns published explainers joined to their pattern.
func (s *Store) PublishedExplainers(ctx context.Context) ([]PatternInfo, error) {
	all, err := s.Patterns(ctx)
	if err != nil {
		return nil, err
	}
	var out []PatternInfo
	for _, p := range all {
		if p.ExplainerPublished {
			out = append(out, p)
		}
	}
	return out, nil
}

// LessonsForPattern lists published units that teach slug and that this
// learner may open: shared, or learner:<id>.
func (s *Store) LessonsForPattern(ctx context.Context, slug, learnerScope string) ([]PatternUnit, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT id, position, scope, title, status, pattern_slug
		FROM units
		WHERE pattern_slug = ? AND status = 'published' AND (scope = 'shared' OR scope = ?)
		ORDER BY position, id`, slug, learnerScope)
	if err != nil {
		return nil, fmt.Errorf("db: lessons for pattern: %w", err)
	}
	defer rows.Close()
	var out []PatternUnit
	for rows.Next() {
		var u PatternUnit
		if err := rows.Scan(&u.ID, &u.Position, &u.Scope, &u.Title, &u.Status, &u.Pattern); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
