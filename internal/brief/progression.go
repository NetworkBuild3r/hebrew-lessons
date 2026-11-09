package brief

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// earlierUnit is one unit strictly before the brief's position, with its pattern and doc.
// INIT-030/SPEC-003.
type earlierUnit struct {
	Position  int
	Title     string
	Situation string
	Pattern   string
	Doc       units.Unit
}

// resolvePosition is the brief's position. A nil want means one past the highest unit in
// the learner's scope (a new unit). INIT-030/SPEC-003.
func (b *Builder) resolvePosition(ctx context.Context, userID int64, want *int) (int, error) {
	if want != nil {
		return *want, nil
	}
	rows, err := b.store.UnitsInScope(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("brief: units in scope: %w", err)
	}
	next := 0
	for _, u := range rows {
		if u.Position+1 > next {
			next = u.Position + 1
		}
	}
	return next, nil
}

// untouchedConsolidationQueued reports whether the learner already has a learner-scoped
// unit with no new words that they have not started. INIT-037/SPEC-007; ADR-0032 D4.
func (b *Builder) untouchedConsolidationQueued(ctx context.Context, userID int64) (bool, error) {
	rows, err := b.store.UnitsInScope(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("brief: units in scope: %w", err)
	}
	scope := db.LearnerScope(userID)
	for _, row := range rows {
		if row.Scope != scope {
			continue
		}
		var doc units.Unit
		if row.Doc != "" {
			if err := json.Unmarshal([]byte(row.Doc), &doc); err != nil {
				return false, fmt.Errorf("brief: unit %d doc: %w", row.ID, err)
			}
		}
		if len(doc.NewWords) > 0 {
			continue
		}
		ok, err := b.store.IsUntouched(ctx, userID, row.ID)
		if err != nil {
			return false, fmt.Errorf("brief: is untouched: %w", err)
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// earlierUnits are the learner-scoped units strictly before position, oldest first.
// INIT-030/SPEC-003.
func (b *Builder) earlierUnits(ctx context.Context, userID int64, before int) ([]earlierUnit, error) {
	rows, err := b.store.UnitsInScope(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("brief: units in scope: %w", err)
	}
	byID := map[int64]string{}
	meta, err := b.store.PatternUnits(ctx)
	if err != nil {
		return nil, fmt.Errorf("brief: pattern units: %w", err)
	}
	for _, u := range meta {
		byID[u.ID] = u.Pattern
	}
	var out []earlierUnit
	for _, row := range rows {
		if row.Position >= before {
			continue
		}
		var doc units.Unit
		if row.Doc != "" {
			if err := json.Unmarshal([]byte(row.Doc), &doc); err != nil {
				return nil, fmt.Errorf("brief: unit %d doc: %w", row.ID, err)
			}
		}
		out = append(out, earlierUnit{
			Position:  row.Position,
			Title:     row.Title,
			Situation: row.Situation,
			Pattern:   byID[row.ID],
			Doc:       doc,
		})
	}
	slices.SortFunc(out, func(a, c earlierUnit) int {
		if a.Position != c.Position {
			return a.Position - c.Position
		}
		return strings.Compare(a.Title, c.Title)
	})
	return out, nil
}

func previousUnitViews(earlier []earlierUnit) []PreviousUnit {
	if len(earlier) == 0 {
		return nil
	}
	start := 0
	if len(earlier) > 2 {
		start = len(earlier) - 2
	}
	out := make([]PreviousUnit, 0, len(earlier)-start)
	for _, u := range earlier[start:] {
		out = append(out, PreviousUnit{
			Title:     u.Title,
			Situation: u.Situation,
			Pattern:   u.Pattern,
			NewWords:  slices.Clone(u.Doc.NewWords),
			Texts:     storyDialogueTexts(u.Doc),
		})
	}
	return out
}

func storyDialogueTexts(doc units.Unit) []string {
	var out []string
	for _, lesson := range doc.Lessons {
		for _, a := range lesson.Activities {
			switch a.Type {
			case units.ActivityStory:
				if t := strings.TrimSpace(a.Text); t != "" {
					out = append(out, t)
				}
			case units.ActivityDialogue:
				for _, line := range a.Lines {
					if t := strings.TrimSpace(line.Text); t != "" {
						out = append(out, t)
					}
				}
			}
		}
	}
	return out
}

// reusePatterns are the two most recently taught pattern slugs before this position,
// each with its catalog title. INIT-030/SPEC-003.
func (b *Builder) reusePatterns(ctx context.Context, earlier []earlierUnit) ([]ReusePattern, error) {
	titles, err := b.patternTitles(ctx)
	if err != nil {
		return nil, err
	}
	var out []ReusePattern
	for i := len(earlier) - 1; i >= 0 && len(out) < 2; i-- {
		slug := earlier[i].Pattern
		if slug == "" {
			continue
		}
		if slices.ContainsFunc(out, func(p ReusePattern) bool { return p.Slug == slug }) {
			continue
		}
		out = append(out, ReusePattern{Slug: slug, Title: titles[slug]})
	}
	return out, nil
}

// nextPattern is the first catalog pattern this learner's earlier units have not taught
// and whose prerequisites those units have all taught. INIT-030/SPEC-003.
func (b *Builder) nextPattern(ctx context.Context, earlier []earlierUnit) (string, error) {
	catalog, err := b.store.Patterns(ctx)
	if err != nil {
		return "", fmt.Errorf("brief: patterns: %w", err)
	}
	taught := map[string]bool{}
	for _, u := range earlier {
		if u.Pattern != "" {
			taught[u.Pattern] = true
		}
	}
	for _, p := range catalog {
		if taught[p.Slug] {
			continue
		}
		ok := true
		for _, req := range p.Prereqs {
			if !taught[req] {
				ok = false
				break
			}
		}
		if ok {
			return p.Slug, nil
		}
	}
	return "", nil
}

func (b *Builder) patternTitles(ctx context.Context) (map[string]string, error) {
	catalog, err := b.store.Patterns(ctx)
	if err != nil {
		return nil, fmt.Errorf("brief: patterns: %w", err)
	}
	out := make(map[string]string, len(catalog))
	for _, p := range catalog {
		out[p.Slug] = p.Title
	}
	return out, nil
}

// reviewAccuracy is the Good/Easy share of the newest PaceReviewWindow non-implied reviews.
// INIT-030/SPEC-003.
func (b *Builder) reviewAccuracy(ctx context.Context, userID int64, now time.Time) (float64, int, error) {
	ratings, err := b.store.RecentReviewRatings(ctx, userID, now, PaceReviewWindow)
	if err != nil {
		return 0, 0, fmt.Errorf("brief: review accuracy: %w", err)
	}
	if len(ratings) == 0 {
		return 0, 0, nil
	}
	good := 0
	for _, r := range ratings {
		if r == int(fsrs.Good) || r == int(fsrs.Easy) {
			good++
		}
	}
	return float64(good) / float64(len(ratings)), len(ratings), nil
}

// fixMistakes reads personal mistakes, skips a pair whose right form is in any of the
// previous three units' final checkpoints, retires a pair answered right three times since
// its date, and returns at most MaxFixMistakes, most repeated first. When none remain, the
// brief carries no fixMistakes. INIT-030/SPEC-003; INIT-037/SPEC-007; ADR-0032 D4.
func (b *Builder) fixMistakes(ctx context.Context, userID int64, earlier []earlierUnit) ([]FixMistake, error) {
	rows, err := b.store.MistakesFor(ctx, userID, fixMistakeScan)
	if err != nil {
		return nil, fmt.Errorf("brief: mistakes: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	skip := skipRightForms(earlier)
	out := make([]FixMistake, 0, MaxFixMistakes)
	for _, row := range rows {
		if skip[units.Normalize(row.Right)] {
			continue
		}
		retired, err := b.mistakeRetired(ctx, userID, row)
		if err != nil {
			return nil, err
		}
		if retired {
			continue
		}
		out = append(out, FixMistake{Wrong: row.Wrong, Right: row.Right, Note: row.Note, Pattern: row.Pattern})
		if len(out) == MaxFixMistakes {
			break
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// skipRightForms are right forms that appear in the final checkpoint of any of the three
// units immediately before the brief's position. INIT-037/SPEC-007; ADR-0032 D4.
func skipRightForms(earlier []earlierUnit) map[string]bool {
	n := len(earlier)
	if n == 0 {
		return nil
	}
	start := n - 3
	if start < 0 {
		start = 0
	}
	out := map[string]bool{}
	for _, u := range earlier[start:] {
		for form := range finalCheckpointForms(u.Doc) {
			out[form] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func finalCheckpointForms(doc units.Unit) map[string]bool {
	var last *units.Lesson
	for i := range doc.Lessons {
		if doc.Lessons[i].Kind == units.KindCheckpoint {
			last = &doc.Lessons[i]
		}
	}
	if last == nil {
		return nil
	}
	out := map[string]bool{}
	add := func(s string) {
		if n := units.Normalize(s); n != "" {
			out[n] = true
		}
	}
	for _, a := range last.Activities {
		add(a.Text)
		add(a.Target)
		for _, line := range a.Lines {
			add(line.Text)
		}
	}
	return out
}

func (b *Builder) mistakeRetired(ctx context.Context, userID int64, row db.LearnerMistake) (bool, error) {
	lexemeID := row.LexemeID
	if lexemeID <= 0 {
		id, _, _, _, ok, err := b.store.LexemeByText(ctx, row.Right)
		if err != nil {
			return false, fmt.Errorf("brief: mistake lexeme: %w", err)
		}
		if !ok {
			return false, nil
		}
		lexemeID = id
	}
	since, err := db.ParseTime(row.FirstAt)
	if err != nil {
		return false, fmt.Errorf("brief: mistake date: %w", err)
	}
	n, err := b.rightAnswersSince(ctx, userID, lexemeID, since)
	if err != nil {
		return false, err
	}
	return n >= FixMistakeRetireRights, nil
}

func (b *Builder) rightAnswersSince(ctx context.Context, userID, lexemeID int64, since time.Time) (int, error) {
	conn, err := b.store.DB()
	if err != nil {
		return 0, fmt.Errorf("brief: %w", err)
	}
	var n int
	err = conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM attempts
		WHERE user_id = ? AND lexeme_id = ? AND correct = 1
		  AND skill IN ('recall', 'spell') AND created_at >= ?`,
		userID, lexemeID, db.FormatTime(since)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("brief: right answers: %w", err)
	}
	return n, nil
}
