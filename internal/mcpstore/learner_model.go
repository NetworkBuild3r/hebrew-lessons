package mcpstore

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/brief"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/sessionlimits"
)

// The learner tools' store (INIT-015/SPEC-021). The app and the AI read the same functions:
// learner.Model for words, states, and the summary, brief.Builder for the lesson brief, and
// db for history and notes. This file only converts their types to the tool shapes.
//
// Every method takes the learner id its caller resolved from the principal. Every query
// below is keyed on it.

var (
	_ mcp.LearnerData     = (*Store)(nil)
	_ mcp.BriefReader     = (*Store)(nil)
	_ mcp.UnitUnpublisher = (*Store)(nil)
)

const (
	// progressRegressionCap is the most regression events get_progress returns.
	progressRegressionCap = 200
	// progressPlacementCap is the most placement runs get_progress returns.
	progressPlacementCap = 20
	// progressReportCap is the most weekly reports get_progress returns.
	progressReportCap = 60
)

// LearnerExists implements mcp.LearnerData.
func (s *Store) LearnerExists(ctx context.Context, id int64) (bool, error) {
	if err := s.ready(ctx); err != nil {
		return false, err
	}
	return s.db.UserExists(ctx, id)
}

// LearnerModel implements mcp.LearnerData with learner.Model.List.
func (s *Store) LearnerModel(ctx context.Context, id int64, q mcp.ModelQuery) (mcp.ModelPage, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.ModelPage{}, err
	}
	words, next, err := learner.New(s.db, nil).List(ctx, id, learner.Filter{
		State: learner.State(q.State), Skill: q.Skill, Band: q.Band, Limit: q.Limit, After: learner.Cursor(q.Cursor),
	})
	if errors.Is(err, learner.ErrBadCursor) {
		return mcp.ModelPage{}, mcp.ErrBadModelCursor
	}
	if err != nil {
		return mcp.ModelPage{}, err
	}
	page := mcp.ModelPage{Words: make([]mcp.ModelWord, 0, len(words)), NextCursor: string(next)}
	for _, w := range words {
		page.Words = append(page.Words, modelWord(w))
	}
	return page, nil
}

func modelWord(w learner.WordState) mcp.ModelWord {
	out := mcp.ModelWord{
		Text: w.Text, Gloss: w.Gloss, State: string(w.State), Cause: string(w.Cause),
		Attempts: w.Attempts, Accuracy: w.Accuracy, LastSeen: timePtr(w.LastSeen), Stability: w.Stability,
		Skills: make([]mcp.ModelSkill, 0, len(w.Skills)),
	}
	if w.Band >= 0 {
		b := w.Band
		out.Band = &b
	}
	for _, sk := range w.Skills {
		out.Skills = append(out.Skills, mcp.ModelSkill{
			Skill: sk.Skill, State: string(sk.State), Attempts: sk.Attempts, Accuracy: sk.Accuracy,
			LastSeen: timePtr(sk.LastSeen), Stability: sk.Stability,
		})
	}
	return out
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

// LearnerModelSummary implements mcp.LearnerData with learner.Model.Summary.
func (s *Store) LearnerModelSummary(ctx context.Context, id int64) (mcp.ModelSummary, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.ModelSummary{}, err
	}
	sum, err := learner.New(s.db, nil).Summary(ctx, id)
	if err != nil {
		return mcp.ModelSummary{}, err
	}
	out := mcp.ModelSummary{
		Words: map[string]int{}, Skills: make([]mcp.ModelSkillSummary, 0, len(sum.Skills)),
		Retention: sum.Retention, DueReviews: sum.DueReviews,
	}
	for st, n := range sum.Words {
		out.Words[string(st)] = n
	}
	for _, sk := range sum.Skills {
		counts := map[string]int{}
		for st, n := range sk.Counts {
			counts[string(st)] = n
		}
		out.Skills = append(out.Skills, mcp.ModelSkillSummary{
			Skill: sk.Skill, Counts: counts, EstimatedKnown: sk.EstimatedKnown,
			Retention: sk.Retention, DueReviews: sk.DueReviews,
		})
	}
	return out, nil
}

// Progress implements mcp.LearnerData. The snapshots are the daily job's rows, so the call
// does not evaluate the model.
func (s *Store) Progress(ctx context.Context, id int64, from, to string) (mcp.Progress, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.Progress{}, err
	}
	rows, err := s.db.SnapshotRows(ctx, id, from, to)
	if err != nil {
		return mcp.Progress{}, err
	}
	days := snapshotDays(rows)
	out := mcp.Progress{Snapshots: days, Changes: skillChanges(days), Retention: retentionOf(days)}

	events, err := s.db.RegressionRows(ctx, id, from, to, progressRegressionCap)
	if err != nil {
		return mcp.Progress{}, err
	}
	out.Regressions = make([]mcp.RegressionView, 0, len(events))
	for _, e := range events {
		out.Regressions = append(out.Regressions, mcp.RegressionView{
			Day: e.Day, Kind: e.Kind, Text: e.Text, Skill: e.Skill, FromState: e.FromState, ToState: e.ToState, Cause: e.Cause,
		})
	}

	runs, err := s.db.PlacementHistory(ctx, id, progressPlacementCap)
	if err != nil {
		return mcp.Progress{}, err
	}
	out.Placements = make([]mcp.PlacementView, 0, len(runs))
	for _, r := range runs {
		out.Placements = append(out.Placements, placementView(r))
	}

	reports, err := s.db.WeeklyReportRows(ctx, id, from, to, progressReportCap)
	if err != nil {
		return mcp.Progress{}, err
	}
	out.WeeklyReports = make([]mcp.ReportView, 0, len(reports))
	for _, r := range reports {
		out.WeeklyReports = append(out.WeeklyReports, mcp.ReportView{WeekStart: r.WeekStart, Summary: r.Summary, CreatedAt: r.CreatedAt})
	}
	return out, nil
}

// snapshotDays groups snapshot rows (ordered by day, skill, state) into days, with the
// skills of each day in learner.Skills order. est_known and retention repeat on every state
// row of a skill, so any row of the skill gives them.
func snapshotDays(rows []db.SnapshotRow) []mcp.SnapshotDay {
	out := []mcp.SnapshotDay{}
	for _, r := range rows {
		if len(out) == 0 || out[len(out)-1].Day != r.Day {
			out = append(out, mcp.SnapshotDay{Day: r.Day, Skills: []mcp.SkillDay{}})
		}
		day := &out[len(out)-1]
		var sk *mcp.SkillDay
		for i := range day.Skills {
			if day.Skills[i].Skill == r.Skill {
				sk = &day.Skills[i]
			}
		}
		if sk == nil {
			day.Skills = append(day.Skills, mcp.SkillDay{Skill: r.Skill, Counts: map[string]int{}})
			sk = &day.Skills[len(day.Skills)-1]
		}
		sk.Counts[r.State] = r.Count
		sk.EstimatedKnown = max(sk.EstimatedKnown, r.EstKnown)
		if r.Retention != nil {
			sk.Retention = r.Retention
		}
	}
	for i := range out {
		slices.SortStableFunc(out[i].Skills, func(a, b mcp.SkillDay) int {
			return slices.Index(learner.Skills, a.Skill) - slices.Index(learner.Skills, b.Skill)
		})
	}
	return out
}

// knownOf is the words a skill knew that day: known and solid.
func knownOf(sk mcp.SkillDay) int {
	return sk.Counts[string(learner.Known)] + sk.Counts[string(learner.Solid)]
}

// skillChanges is, for each skill, the known words at the first and the last snapshot day
// of the range, and the net move between them. A range with one snapshot day has no move.
func skillChanges(days []mcp.SnapshotDay) []mcp.SkillChange {
	out := []mcp.SkillChange{}
	if len(days) == 0 {
		return out
	}
	first, last := days[0], days[len(days)-1]
	for _, skill := range learner.Skills {
		var a, b *mcp.SkillDay
		for i := range first.Skills {
			if first.Skills[i].Skill == skill {
				a = &first.Skills[i]
			}
		}
		for i := range last.Skills {
			if last.Skills[i].Skill == skill {
				b = &last.Skills[i]
			}
		}
		if a == nil || b == nil {
			continue
		}
		c := mcp.SkillChange{Skill: skill, KnownAtStart: knownOf(*a), KnownAtEnd: knownOf(*b)}
		c.Gained = max(c.KnownAtEnd-c.KnownAtStart, 0)
		c.Lost = max(c.KnownAtStart-c.KnownAtEnd, 0)
		out = append(out, c)
	}
	return out
}

// retentionOf is the mean of the skills' retention on the newest snapshot day that has any,
// against the target. The snapshot has no due counts, so the skills are weighted equally.
func retentionOf(days []mcp.SnapshotDay) mcp.RetentionView {
	v := mcp.RetentionView{Target: sessionlimits.TargetSuccess}
	for i := len(days) - 1; i >= 0; i-- {
		var sum float64
		n := 0
		for _, sk := range days[i].Skills {
			if sk.Retention != nil {
				sum += *sk.Retention
				n++
			}
		}
		if n == 0 {
			continue
		}
		actual := sum / float64(n)
		gap := actual - v.Target
		v.Actual, v.Gap, v.Day = &actual, &gap, days[i].Day
		break
	}
	return v
}

// placementResult is the part of placement_runs.result this tool reads. A v2 run has no
// listening or reading.
type placementResult struct {
	EstimatedKnownWords int `json:"estimated_known_words"`
	Listening           *struct {
		Tested   bool `json:"tested"`
		Estimate int  `json:"estimate"`
	} `json:"listening"`
	Reading *struct {
		Tested   bool `json:"tested"`
		Estimate int  `json:"estimate"`
	} `json:"reading"`
}

func placementView(r db.PlacementHistoryRow) mcp.PlacementView {
	v := mcp.PlacementView{Kind: r.Kind, FinishedAt: r.FinishedAt, Level: r.Level}
	if v.Kind == "" {
		v.Kind = "v2"
	}
	var res placementResult
	if r.Result != "" && json.Unmarshal([]byte(r.Result), &res) == nil {
		v.EstimatedKW = res.EstimatedKnownWords
		if res.Listening != nil && res.Listening.Tested {
			n := res.Listening.Estimate
			v.Listening = &n
		}
		if res.Reading != nil && res.Reading.Tested {
			n := res.Reading.Estimate
			v.Reading = &n
		}
	}
	return v
}

// BuildLessonBrief implements mcp.LearnerData with brief.Builder.Build, which stores the
// brief under the learner id and returns it without that id.
func (s *Store) BuildLessonBrief(ctx context.Context, id int64, theme string) (mcp.LessonBrief, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.LessonBrief{}, err
	}
	model := learner.New(s.db, nil)
	br, err := brief.New(s.db, model, nil).Build(ctx, id, theme)
	if errors.Is(err, brief.ErrTheme) {
		return mcp.LessonBrief{}, mcp.ErrBadTheme
	}
	if err != nil {
		return mcp.LessonBrief{}, err
	}
	return lessonBrief(br), nil
}

func lessonBrief(br brief.Brief) mcp.LessonBrief {
	word := func(w brief.Word) mcp.BriefWord { return mcp.BriefWord{Text: w.Text, Gloss: w.Gloss} }
	bandOf := func(b int) *int {
		if b < 0 {
			return nil
		}
		return &b
	}
	out := mcp.LessonBrief{
		BriefID: br.ID, Theme: br.Theme, CreatedAt: br.CreatedAt, ExpiresAt: br.ExpiresAt,
		Tier: mcp.BriefTier{
			Name: br.Tier.Name, KnownWords: br.Tier.KnownWords, MaxTextTokens: br.Tier.MaxTextTokens, MinKnownShare: br.Tier.MinKnownShare,
		},
		FocusCause: br.FocusCause, SkillFocus: br.SkillFocus,
		Activities: slices.Clone(br.Activities), Notes: slices.Clone(br.Notes),
		Targets: mcp.BriefTargets{Coverage: br.Targets.Coverage, NewWordMinUses: br.Targets.NewWordMinUses},
	}
	for _, w := range br.NewWords {
		out.NewWords = append(out.NewWords, mcp.BriefNewWord{
			BriefWord: word(w.Word), Band: bandOf(w.Band), MinUses: w.MinUses, ThemeMatch: w.ThemeMatch,
		})
	}
	for _, w := range br.StruggleWords {
		out.StruggleWords = append(out.StruggleWords, mcp.BriefStruggleWord{
			BriefWord: word(w.Word), State: w.State, Skill: w.Skill, Cause: w.Cause,
			RequiredCount: w.RequiredCount, Activities: slices.Clone(w.Activities),
		})
	}
	for _, w := range br.ComfortableWords {
		out.ComfortableWords = append(out.ComfortableWords, word(w))
	}
	for _, n := range br.CoachNotes {
		out.CoachNotes = append(out.CoachNotes, mcp.CoachNote{Author: n.Author, Body: n.Body, CreatedAt: n.At})
	}
	for _, m := range br.FixMistakes {
		out.FixMistakes = append(out.FixMistakes, mcp.BriefFixMistake{Wrong: m.Wrong, Right: m.Right, Note: m.Note, Pattern: m.Pattern})
	}
	return out
}

// AddCoachNote implements mcp.LearnerData.
func (s *Store) AddCoachNote(ctx context.Context, id int64, author, body string) (mcp.CoachNote, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.CoachNote{}, err
	}
	at := s.now()
	noteID, err := s.db.AddCoachNote(ctx, id, author, body, at)
	if errors.Is(err, db.ErrTooManyCoachNotes) {
		return mcp.CoachNote{}, mcp.ErrTooManyNotes
	}
	if err != nil {
		return mcp.CoachNote{}, err
	}
	return mcp.CoachNote{ID: noteID, Author: author, Body: body, CreatedAt: at.UTC()}, nil
}

// RecentCoachNotes implements mcp.LearnerData.
func (s *Store) RecentCoachNotes(ctx context.Context, id int64, n int) ([]mcp.CoachNote, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.RecentCoachNotes(ctx, id, n)
	if err != nil {
		return nil, err
	}
	out := make([]mcp.CoachNote, 0, len(rows))
	for _, r := range rows {
		out = append(out, mcp.CoachNote{ID: r.ID, Author: r.Author, Body: r.Body, CreatedAt: r.CreatedAt})
	}
	return out, nil
}
