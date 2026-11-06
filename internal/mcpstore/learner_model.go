package mcpstore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/brief"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/sessionlimits"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// The learner tools' store (INIT-015/SPEC-021). The app and the AI read the same functions:
// learner.Model for words, states, and the summary, brief.Builder for the lesson brief, and
// db for history and notes. This file only converts their types to the tool shapes.
//
// Every method takes the learner id its caller resolved from the principal. Every query
// below is keyed on it.

var (
	_ mcp.LearnerData          = (*Store)(nil)
	_ mcp.BriefReader          = (*Store)(nil)
	_ mcp.UnitUnpublisher      = (*Store)(nil)
	_ mcp.PlacementPartsReader = (*Store)(nil)
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
	out.ConfusedWith = confusedWith(w.Confusions)
	for _, sk := range w.Skills {
		out.Skills = append(out.Skills, mcp.ModelSkill{
			Skill: sk.Skill, State: string(sk.State), Attempts: sk.Attempts, Accuracy: sk.Accuracy,
			LastSeen: timePtr(sk.LastSeen), Stability: sk.Stability,
		})
	}
	return out
}

// confusedWith is the dominant confusion pair when DominantPair says there is one.
// DominantPair returns the key and count only; the kind is recovered from the same window.
func confusedWith(confusions []learner.Confusion) *mcp.ConfusedWith {
	key, count, ok := learner.DominantPair(confusions)
	if !ok {
		return nil
	}
	type pair struct{ kind, key string }
	counts := map[pair]int{}
	for _, c := range confusions {
		counts[pair{c.Kind, c.Key}]++
	}
	for p, n := range counts {
		if p.key == key && n == count {
			return &mcp.ConfusedWith{Kind: p.kind, Key: key, Count: count}
		}
	}
	return &mcp.ConfusedWith{Key: key, Count: count}
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
	now := s.now()
	sum, err := learner.New(s.db, nil).Summary(ctx, id)
	if err != nil {
		return mcp.ModelSummary{}, err
	}
	asOfDay, err := s.learnerCivilDay(ctx, id, now)
	if err != nil {
		return mcp.ModelSummary{}, err
	}
	out := mcp.ModelSummary{
		Words: map[string]int{}, Skills: make([]mcp.ModelSkillSummary, 0, len(sum.Skills)),
		KnownAnySkill: sum.KnownAnySkill, DueReviews: sum.DueReviews, AsOf: now.UTC(),
		Retention: labelRetention(sum.Retention, asOfDay),
	}
	for st, n := range sum.Words {
		out.Words[string(st)] = n
	}
	for _, sk := range sum.Skills {
		counts := map[string]int{}
		for st, n := range sk.Counts {
			counts[string(st)] = n
		}
		row := mcp.ModelSkillSummary{
			Skill: sk.Skill, Counts: counts, EstimatedKnown: sk.EstimatedKnown,
			EstimateSource: sk.EstimateSource, DueReviews: sk.DueReviews,
			Retention: labelRetention(sk.Retention, asOfDay),
		}
		if sk.EstimateSource == learner.EstimateSourcePlacement && !sk.PlacementRunAt.IsZero() {
			at := sk.PlacementRunAt.UTC()
			row.PlacementRunAt = &at
		}
		out.Skills = append(out.Skills, row)
	}
	return out, nil
}

// learnerCivilDay is the learner's today in their effective zone (learner or app).
// It never substitutes UTC for a missing zone (ADR-0022 / ADR-0027).
func (s *Store) learnerCivilDay(ctx context.Context, id int64, now time.Time) (string, error) {
	app, err := studyday.AppZone(os.Getenv("TZ"))
	if err != nil {
		return "", err
	}
	zones, err := studyday.NewResolver(s, app)
	if err != nil {
		return "", err
	}
	z, err := zones.For(ctx, id)
	if err != nil {
		return "", err
	}
	return studyday.Today(now, z), nil
}

func labelRetention(share *float64, asOfDay string) *mcp.RetentionLabeled {
	if share == nil {
		return nil
	}
	v := *share
	return &mcp.RetentionLabeled{Share: &v, Window: mcp.RetentionWindow30d, AsOf: asOfDay}
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
	if len(days) > 0 {
		out.SnapshotsThrough = days[len(days)-1].Day
	}

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

	reports, err := s.weeklyReports(ctx, id, from, to, progressReportCap)
	if err != nil {
		return mcp.Progress{}, err
	}
	out.WeeklyReports = reports
	return out, nil
}

// LatestPlacementParts implements mcp.PlacementPartsReader for get_context.placement.
func (s *Store) LatestPlacementParts(ctx context.Context, userID int64) (listening, reading *mcp.PlacementPartView, err error) {
	if err := s.ready(ctx); err != nil {
		return nil, nil, err
	}
	runs, err := s.db.PlacementHistory(ctx, userID, 1)
	if err != nil {
		return nil, nil, err
	}
	if len(runs) == 0 {
		return nil, nil, nil
	}
	v := placementView(runs[0])
	return v.ListeningPart, v.ReadingPart, nil
}

// weeklyReports reads weekly_reports including payload_json. A row whose JSON does
// not decode leaves Payload nil and is not an error (INIT-026/SPEC-007).
func (s *Store) weeklyReports(ctx context.Context, id int64, from, to string, limit int) ([]mcp.ReportView, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT week_start, summary, created_at, payload_json FROM weekly_reports
		WHERE user_id = ? AND week_start >= ? AND week_start <= ?
		ORDER BY week_start DESC LIMIT ?`, id, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []mcp.ReportView{}
	for rows.Next() {
		var r mcp.ReportView
		var at, payload string
		if err := rows.Scan(&r.WeekStart, &r.Summary, &at, &payload); err != nil {
			return nil, err
		}
		if r.CreatedAt, err = db.ParseTime(at); err != nil {
			return nil, err
		}
		r.Payload = decodeReportPayload(payload)
		labelWeeklyRetention(r.Payload)
		out = append(out, r)
	}
	return out, rows.Err()
}

func decodeReportPayload(raw string) *mcp.ReportPayload {
	var p mcp.ReportPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil
	}
	return &p
}

// labelWeeklyRetention sets window week and asOf on a weekly payload (ADR-0027 D5).
func labelWeeklyRetention(p *mcp.ReportPayload) {
	if p == nil {
		return
	}
	p.Retention.Window = mcp.RetentionWindowWeek
	if p.Retention.AsOf == "" {
		p.Retention.AsOf = p.Retention.Day
	}
}

// UserTimeZone implements mcp.LearnerData and studyday.ZoneStore.
func (s *Store) UserTimeZone(ctx context.Context, id int64) (string, bool, error) {
	if err := s.ready(ctx); err != nil {
		return "", false, err
	}
	return s.db.UserTimeZone(ctx, id)
}

// SetUserTimeZone implements mcp.LearnerData.
func (s *Store) SetUserTimeZone(ctx context.Context, id int64, zone string) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	return s.db.SetUserTimeZone(ctx, id, zone)
}

// UserTimeZones implements studyday.ZoneStore.
func (s *Store) UserTimeZones(ctx context.Context) (map[int64]string, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.db.UserTimeZones(ctx)
}

// UserIDs implements studyday.ZoneStore.
func (s *Store) UserIDs(ctx context.Context) ([]int64, error) {
	if err := s.ready(ctx); err != nil {
		return nil, err
	}
	return s.db.UserIDs(ctx)
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
	v := mcp.RetentionView{Target: sessionlimits.TargetSuccess, Window: mcp.RetentionWindow30d}
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
		v.Actual, v.Gap, v.Day, v.AsOf = &actual, &gap, days[i].Day, days[i].Day
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
		Level    int  `json:"level"`
		Estimate int  `json:"estimate"`
	} `json:"listening"`
	Reading *struct {
		Tested   bool `json:"tested"`
		Level    int  `json:"level"`
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
		if res.Listening != nil {
			v.ListeningPart = &mcp.PlacementPartView{
				Tested: res.Listening.Tested, Level: res.Listening.Level, Estimate: res.Listening.Estimate,
			}
			if res.Listening.Tested {
				n := res.Listening.Estimate
				v.Listening = &n
			}
		}
		if res.Reading != nil {
			v.ReadingPart = &mcp.PlacementPartView{
				Tested: res.Reading.Tested, Level: res.Reading.Level, Estimate: res.Reading.Estimate,
			}
			if res.Reading.Tested {
				n := res.Reading.Estimate
				v.Reading = &n
			}
		}
	}
	return v
}

// BuildLessonBrief implements mcp.LearnerData with brief.Builder.Build, which stores the
// brief under the learner id and returns it without that id. Refuses when the learner
// already has three or more untouched learner-scoped units. INIT-030/SPEC-005.
func (s *Store) BuildLessonBrief(ctx context.Context, id int64, theme string) (mcp.LessonBrief, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.LessonBrief{}, err
	}
	if err := s.refuseNewBriefBacklog(ctx, id); err != nil {
		return mcp.LessonBrief{}, err
	}
	app, err := studyday.AppZone(os.Getenv("TZ"))
	if err != nil {
		return mcp.LessonBrief{}, err
	}
	zones, err := studyday.NewResolver(s, app)
	if err != nil {
		return mcp.LessonBrief{}, err
	}
	model := learner.New(s.db, nil)
	br, err := brief.New(s.db, model, nil, brief.WithResolver(zones)).Build(ctx, id, theme)
	if errors.Is(err, brief.ErrTheme) {
		return mcp.LessonBrief{}, mcp.ErrBadTheme
	}
	if err != nil {
		return mcp.LessonBrief{}, err
	}
	return lessonBrief(br), nil
}

// BuildRevisionBrief implements mcp.LearnerData: a revision brief for one untouched
// learner-scoped unit, built as of that unit's position. INIT-030/SPEC-005.
func (s *Store) BuildRevisionBrief(ctx context.Context, id int64, theme string, unitID int64) (mcp.LessonBrief, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.LessonBrief{}, err
	}
	snap, err := s.db.LoadUnit(ctx, unitID, nil)
	if errors.Is(err, db.ErrUnitNotFound) {
		return mcp.LessonBrief{}, mcp.ErrReviseOtherLearner
	}
	if err != nil {
		return mcp.LessonBrief{}, err
	}
	switch {
	case snap.Scope == mcp.ScopeShared:
		return mcp.LessonBrief{}, mcp.ErrReviseShared
	case snap.Scope != db.LearnerScope(id):
		return mcp.LessonBrief{}, mcp.ErrReviseOtherLearner
	}
	ok, err := s.db.IsUntouched(ctx, id, unitID)
	if err != nil {
		return mcp.LessonBrief{}, err
	}
	if !ok {
		progress, err := s.db.UnitProgress(ctx, id)
		if err != nil {
			return mcp.LessonBrief{}, err
		}
		for _, p := range progress {
			if p.UnitID != unitID {
				continue
			}
			if p.State == db.UnitFinished {
				return mcp.LessonBrief{}, mcp.ErrReviseFinished
			}
			return mcp.LessonBrief{}, mcp.ErrReviseStartedUnit
		}
		return mcp.LessonBrief{}, mcp.ErrReviseStartedUnit
	}
	app, err := studyday.AppZone(os.Getenv("TZ"))
	if err != nil {
		return mcp.LessonBrief{}, err
	}
	zones, err := studyday.NewResolver(s, app)
	if err != nil {
		return mcp.LessonBrief{}, err
	}
	model := learner.New(s.db, nil)
	br, err := brief.New(s.db, model, nil, brief.WithResolver(zones)).BuildAt(ctx, id, theme, snap.Position, unitID)
	if errors.Is(err, brief.ErrTheme) {
		return mcp.LessonBrief{}, mcp.ErrBadTheme
	}
	if err != nil {
		return mcp.LessonBrief{}, err
	}
	return lessonBrief(br), nil
}

// refuseNewBriefBacklog refuses a new brief when three or more untouched learner-scoped
// units already sit on Today. INIT-030/SPEC-005; ADR-0026 D8.
func (s *Store) refuseNewBriefBacklog(ctx context.Context, learnerID int64) error {
	units, err := s.db.UnitsInScope(ctx, learnerID)
	if err != nil {
		return err
	}
	progress, err := s.db.UnitProgress(ctx, learnerID)
	if err != nil {
		return err
	}
	byID := make(map[int64]db.UnitProgress, len(progress))
	for _, p := range progress {
		byID[p.UnitID] = p
	}
	scope := db.LearnerScope(learnerID)
	var earliest int64
	var earliestPos int
	count := 0
	for _, u := range units {
		if u.Scope != scope {
			continue
		}
		p, ok := byID[u.ID]
		if !ok || p.State != db.UnitUntouched {
			continue
		}
		count++
		if earliest == 0 || u.Position < earliestPos || (u.Position == earliestPos && u.ID < earliest) {
			earliest = u.ID
			earliestPos = u.Position
		}
	}
	if count >= 3 {
		return mcp.ErrUntouchedBacklog{UnitID: earliest}
	}
	return nil
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
		out.CoachNotes = append(out.CoachNotes, mcp.CoachNote{Author: n.Author, AuthorRole: n.AuthorRole, Body: n.Body, CreatedAt: n.At})
	}
	for _, m := range br.FixMistakes {
		out.FixMistakes = append(out.FixMistakes, mcp.BriefFixMistake{Wrong: m.Wrong, Right: m.Right, Note: m.Note, Pattern: m.Pattern})
	}
	return out
}

// AddCoachNote implements mcp.LearnerData.
func (s *Store) AddCoachNote(ctx context.Context, id int64, author, authorRole, body string) (mcp.CoachNote, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.CoachNote{}, err
	}
	at := s.now()
	noteID, err := s.db.AddCoachNote(ctx, id, author, authorRole, body, at)
	if errors.Is(err, db.ErrTooManyCoachNotes) {
		return mcp.CoachNote{}, mcp.ErrTooManyNotes
	}
	if err != nil {
		return mcp.CoachNote{}, err
	}
	return mcp.CoachNote{ID: noteID, Author: author, AuthorRole: authorRole, Body: body, CreatedAt: at.UTC()}, nil
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
		out = append(out, mcp.CoachNote{ID: r.ID, Author: r.Author, AuthorRole: r.AuthorRole, Body: r.Body, CreatedAt: r.CreatedAt})
	}
	return out, nil
}
