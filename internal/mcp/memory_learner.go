package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// The in-memory side of the learner tools (INIT-015/SPEC-021). Memory implements
// LearnerData, BriefReader, and UnitUnpublisher, so the tests of the tools need no SQLite.
// It does not implement AutoPublishLearner, so it is not a LearnerUnits and a learner
// unit saved to it stays a draft, as before.
//
// The test seeds what the learner model would say: SetLearnerModel, SetProgress,
// SetBriefTemplate, and SetBriefFacts. Memory applies the same filters and the same scoping
// as the database store: nothing is returned for a learner id other than the one asked for.

// MemoryAudit is one row Memory wrote for an unpublish.
type MemoryAudit struct {
	UnitID   int64
	Revision int
	Scope    string
	Actor    string
	Action   string
}

type memLearn struct {
	learners   map[int64]bool
	words      map[int64][]ModelWord
	summary    map[int64]ModelSummary
	progress   map[int64]Progress
	template   map[int64]LessonBrief
	briefOwner map[string]int64
	facts      map[string]units.BriefFacts
	notes      []memNote
	nextNote   int64
	nextBrief  int
	audits     []MemoryAudit
}

type memNote struct {
	user int64
	note CoachNote
}

var (
	_ LearnerData     = (*Memory)(nil)
	_ BriefReader     = (*Memory)(nil)
	_ UnitUnpublisher = (*Memory)(nil)
)

// AddLearner makes a learner id exist for LearnerExists.
func (m *Memory) AddLearner(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.learn.learners == nil {
		m.learn.learners = map[int64]bool{}
	}
	m.learn.learners[id] = true
}

// SetLearnerModel stores the words a learner's model returns, in the order given, and
// optionally a summary. The words are paged in that order.
func (m *Memory) SetLearnerModel(id int64, words []ModelWord, summary *ModelSummary) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.learn.words == nil {
		m.learn.words = map[int64][]ModelWord{}
		m.learn.summary = map[int64]ModelSummary{}
	}
	m.learn.words[id] = slices.Clone(words)
	if summary != nil {
		m.learn.summary[id] = *summary
	}
}

// SetProgress stores a learner's progress. Progress reads the days inside the range.
func (m *Memory) SetProgress(id int64, p Progress) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.learn.progress == nil {
		m.learn.progress = map[int64]Progress{}
	}
	m.learn.progress[id] = p
}

// SetBriefTemplate stores what BuildLessonBrief returns for a learner. The id is new each time.
func (m *Memory) SetBriefTemplate(id int64, b LessonBrief) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.learn.template == nil {
		m.learn.template = map[int64]LessonBrief{}
	}
	m.learn.template[id] = b
}

// SetBriefFacts stores the facts of a brief for a learner. Any other learner gets
// ErrBriefNotFound for the id, as the database store does.
func (m *Memory) SetBriefFacts(learnerID int64, briefID string, f units.BriefFacts) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.learn.briefOwner == nil {
		m.learn.briefOwner = map[string]int64{}
		m.learn.facts = map[string]units.BriefFacts{}
	}
	m.learn.briefOwner[briefID] = learnerID
	m.learn.facts[briefID] = f
}

// Audits returns the audit rows Memory wrote.
func (m *Memory) Audits() []MemoryAudit {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.learn.audits)
}

// LearnerExists implements LearnerData.
func (m *Memory) LearnerExists(ctx context.Context, id int64) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.learn.learners[id], nil
}

const memCursorPrefix = "m1:"

// LearnerModel implements LearnerData. The state filter is the overall state, or the state
// of the skill when a skill is set.
func (m *Memory) LearnerModel(ctx context.Context, id int64, q ModelQuery) (ModelPage, error) {
	if err := ctx.Err(); err != nil {
		return ModelPage{}, err
	}
	start := 0
	if q.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(q.Cursor)
		rest, ok := strings.CutPrefix(string(raw), memCursorPrefix)
		if err != nil || !ok {
			return ModelPage{}, ErrBadModelCursor
		}
		n, err := strconv.Atoi(rest)
		if err != nil || n < 0 {
			return ModelPage{}, ErrBadModelCursor
		}
		start = n
	}
	limit := q.Limit
	if limit <= 0 {
		limit = learnerModelDefaultLimit
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var page ModelPage
	for i := start; i < len(m.learn.words[id]); i++ {
		w := m.learn.words[id][i]
		if !memModelMatches(w, q) {
			continue
		}
		if len(page.Words) == limit {
			page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(memCursorPrefix + strconv.Itoa(i)))
			break
		}
		page.Words = append(page.Words, w)
	}
	return page, nil
}

func memModelMatches(w ModelWord, q ModelQuery) bool {
	if q.Band != nil && (w.Band == nil || *w.Band != *q.Band) {
		return false
	}
	if q.State == "" {
		return true
	}
	if q.Skill == "" {
		return w.State == q.State
	}
	for _, s := range w.Skills {
		if s.Skill == q.Skill {
			return s.State == q.State
		}
	}
	return q.State == "unseen"
}

// LearnerModelSummary implements LearnerData.
func (m *Memory) LearnerModelSummary(ctx context.Context, id int64) (ModelSummary, error) {
	if err := ctx.Err(); err != nil {
		return ModelSummary{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.learn.summary[id]
	if s.Words == nil {
		s.Words = map[string]int{}
	}
	if s.Skills == nil {
		s.Skills = []ModelSkillSummary{}
	}
	return s, nil
}

// Progress implements LearnerData. It keeps the snapshot days, regression days, and report
// weeks that fall in the range.
func (m *Memory) Progress(ctx context.Context, id int64, from, to string) (Progress, error) {
	if err := ctx.Err(); err != nil {
		return Progress{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.learn.progress[id]
	out := Progress{Changes: slices.Clone(p.Changes), Retention: p.Retention, Placements: slices.Clone(p.Placements)}
	for _, d := range p.Snapshots {
		if d.Day >= from && d.Day <= to {
			out.Snapshots = append(out.Snapshots, d)
		}
	}
	for _, r := range p.Regressions {
		if r.Day >= from && r.Day <= to {
			out.Regressions = append(out.Regressions, r)
		}
	}
	for _, r := range p.WeeklyReports {
		if r.WeekStart >= from && r.WeekStart <= to {
			out.WeeklyReports = append(out.WeeklyReports, r)
		}
	}
	return out, nil
}

// BuildLessonBrief implements LearnerData. The brief is the learner's template with a new id.
func (m *Memory) BuildLessonBrief(ctx context.Context, id int64, theme string) (LessonBrief, error) {
	if err := ctx.Err(); err != nil {
		return LessonBrief{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	b := m.learn.template[id]
	m.learn.nextBrief++
	b.BriefID = fmt.Sprintf("M%021d", m.learn.nextBrief)
	b.Theme = theme
	if b.Theme == "" {
		b.Theme = "general"
	}
	if m.learn.briefOwner == nil {
		m.learn.briefOwner = map[string]int64{}
		m.learn.facts = map[string]units.BriefFacts{}
	}
	m.learn.briefOwner[b.BriefID] = id
	m.learn.facts[b.BriefID] = units.BriefFacts{}
	return b, nil
}

// BriefFacts implements BriefReader. A brief of another learner is not found.
func (m *Memory) BriefFacts(ctx context.Context, learnerID int64, briefID string) (units.BriefFacts, error) {
	if err := ctx.Err(); err != nil {
		return units.BriefFacts{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	owner, ok := m.learn.briefOwner[briefID]
	if !ok || owner != learnerID {
		return units.BriefFacts{}, ErrBriefNotFound
	}
	return m.learn.facts[briefID], nil
}

// AddCoachNote implements LearnerData. It refuses a note when the learner already has
// maxCoachNotesPerDay from the last 24 hours.
func (m *Memory) AddCoachNote(ctx context.Context, id int64, author, body string) (CoachNote, error) {
	if err := ctx.Err(); err != nil {
		return CoachNote{}, err
	}
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()
	recent := 0
	for _, n := range m.learn.notes {
		if n.user == id && n.note.CreatedAt.After(now.Add(-24*time.Hour)) {
			recent++
		}
	}
	if recent >= maxCoachNotesPerDay {
		return CoachNote{}, ErrTooManyNotes
	}
	m.learn.nextNote++
	note := CoachNote{ID: m.learn.nextNote, Author: author, Body: body, CreatedAt: now}
	m.learn.notes = append(m.learn.notes, memNote{user: id, note: note})
	return note, nil
}

// RecentCoachNotes implements LearnerData.
func (m *Memory) RecentCoachNotes(ctx context.Context, id int64, n int) ([]CoachNote, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []CoachNote{}
	for i := len(m.learn.notes) - 1; i >= 0 && len(out) < n; i-- {
		if m.learn.notes[i].user == id {
			out = append(out, m.learn.notes[i].note)
		}
	}
	return out, nil
}

// UnpublishLearner implements UnitUnpublisher with the rules of the database store: the
// scope is learner:<p.UserID>, a unit in any other scope is ErrUnitNotInScope, and a unit
// that is not published is unchanged and writes nothing.
func (m *Memory) UnpublishLearner(ctx context.Context, p Principal, unitID int64) (UnpublishResult, error) {
	if err := ctx.Err(); err != nil {
		return UnpublishResult{}, err
	}
	if p.UserID <= 0 {
		return UnpublishResult{}, ErrNoLearner
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.byID(unitID)
	scope := fmt.Sprintf("learner:%d", p.UserID)
	if u == nil || u.scope != scope {
		return UnpublishResult{}, ErrUnitNotInScope
	}
	if u.status != "published" {
		return UnpublishResult{UnitID: u.id, Revision: u.rev, Unchanged: true}, nil
	}
	u.status = "draft"
	actor := strings.TrimSpace(p.Actor)
	if actor == "" {
		actor = defaultActor
	}
	m.learn.audits = append(m.learn.audits, MemoryAudit{UnitID: u.id, Revision: u.rev, Scope: scope, Actor: actor, Action: "unpublish"})
	return UnpublishResult{UnitID: u.id, Revision: u.rev}, nil
}
