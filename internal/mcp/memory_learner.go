package mcp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
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
	zones      map[int64]string
	words      map[int64][]ModelWord
	summary    map[int64]ModelSummary
	standing   map[int64]StandingView
	progress   map[int64]Progress
	template   map[int64]LessonBrief
	briefOwner map[string]int64
	facts      map[string]units.BriefFacts
	notes      []memNote
	nextNote   int64
	nextBrief  int
	audits     []MemoryAudit
	// practiceResults seeds LaterDayResults for get_context tests.
	// INIT-029/SPEC-008.
	practiceResults map[int64][]evidence.PracticeResult
	// unitProgress seeds UnitProgress for get_context outline rows. INIT-030/SPEC-005.
	unitProgress map[int64][]UnitProgressRow
	// reviseOK is the set of unit ids BuildRevisionBrief accepts as untouched.
	// INIT-030/SPEC-005.
	reviseOK map[int64]bool
	// reviseRefuse maps unit id to the error BuildRevisionBrief returns.
	reviseRefuse map[int64]error
	// untouchedLearnerUnits are learner-scoped untouched unit ids in position order,
	// used by the new-brief backlog cap. INIT-030/SPEC-005.
	untouchedLearnerUnits map[int64][]int64
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
		if m.learn.standing == nil {
			m.learn.standing = map[int64]StandingView{}
		}
		if _, ok := m.learn.standing[id]; !ok {
			m.learn.standing[id] = StandingView{KnownAnySkill: summary.KnownAnySkill, Directions: []DirectionStandingView{}}
		}
	}
}

// SetLearnerStanding stores the standing get_learner_model returns for a learner.
func (m *Memory) SetLearnerStanding(id int64, standing StandingView) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.learn.standing == nil {
		m.learn.standing = map[int64]StandingView{}
	}
	m.learn.standing[id] = standing
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
	if s.Standing == nil {
		if st, ok := m.learn.standing[id]; ok {
			s.Standing = compactStandingFromView(st)
		}
	}
	return s, nil
}

// LearnerStanding implements LearnerData.
func (m *Memory) LearnerStanding(ctx context.Context, id int64) (StandingView, error) {
	if err := ctx.Err(); err != nil {
		return StandingView{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.learn.standing[id]; ok {
		return st, nil
	}
	sum := m.learn.summary[id]
	return StandingView{KnownAnySkill: sum.KnownAnySkill, Directions: []DirectionStandingView{}}, nil
}

func compactStandingFromView(s StandingView) *CompactStanding {
	out := &CompactStanding{Directions: make([]CompactDirection, 0, len(s.Directions))}
	for _, d := range s.Directions {
		row := CompactDirection{
			Skill: d.Skill, Direction: d.Direction, Verdict: d.Verdict, Reason: d.Reason,
			Help: make([]CompactHelp, 0, CompactStandingHelpCap),
		}
		for i, n := range d.Help {
			if i >= CompactStandingHelpCap {
				break
			}
			row.Help = append(row.Help, CompactHelp{LexemeID: n.LexemeID, Cause: n.Cause})
		}
		out.Directions = append(out.Directions, row)
	}
	return out
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

// UserTimeZone implements LearnerData and studyday.ZoneStore.
func (m *Memory) UserTimeZone(ctx context.Context, id int64) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	zone, ok := m.learn.zones[id]
	return zone, ok && zone != "", nil
}

// SetUserTimeZone implements LearnerData.
func (m *Memory) SetUserTimeZone(ctx context.Context, id int64, zone string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if zone == "" {
		return errors.New("time zone is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.learn.learners == nil || !m.learn.learners[id] {
		return errors.New("no such learner")
	}
	if m.learn.zones == nil {
		m.learn.zones = map[int64]string{}
	}
	m.learn.zones[id] = zone
	return nil
}

// UserTimeZones implements studyday.ZoneStore.
func (m *Memory) UserTimeZones(ctx context.Context) (map[int64]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[int64]string{}
	for id, zone := range m.learn.zones {
		if zone != "" {
			out[id] = zone
		}
	}
	return out, nil
}

// UserIDs implements studyday.ZoneStore.
func (m *Memory) UserIDs(ctx context.Context) ([]int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]int64, 0, len(m.learn.learners))
	for id := range m.learn.learners {
		out = append(out, id)
	}
	slices.Sort(out)
	return out, nil
}

// SetPracticeResults seeds LaterDayResults for one learner. INIT-029/SPEC-008.
func (m *Memory) SetPracticeResults(learnerID int64, rows []evidence.PracticeResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.learn.practiceResults == nil {
		m.learn.practiceResults = map[int64][]evidence.PracticeResult{}
	}
	m.learn.practiceResults[learnerID] = append([]evidence.PracticeResult(nil), rows...)
}

// LaterDayResults implements laterDayResultsReader for get_context tests.
// INIT-029/SPEC-008.
func (m *Memory) LaterDayResults(ctx context.Context, learnerID int64, _ time.Time, _ studyday.Zone) ([]evidence.PracticeResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	rows := m.learn.practiceResults[learnerID]
	if rows == nil {
		return []evidence.PracticeResult{}, nil
	}
	return append([]evidence.PracticeResult(nil), rows...), nil
}

// SetUnitProgress seeds UnitProgress for get_context. INIT-030/SPEC-005.
func (m *Memory) SetUnitProgress(learnerID int64, rows []UnitProgressRow) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.learn.unitProgress == nil {
		m.learn.unitProgress = map[int64][]UnitProgressRow{}
	}
	m.learn.unitProgress[learnerID] = slices.Clone(rows)
}

// UnitProgress implements unitProgressReader. INIT-030/SPEC-005.
func (m *Memory) UnitProgress(ctx context.Context, userID int64) ([]UnitProgressRow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.learn.unitProgress[userID]), nil
}

// SetUntouchedLearnerUnits seeds the backlog-cap list for BuildLessonBrief. INIT-030/SPEC-005.
func (m *Memory) SetUntouchedLearnerUnits(learnerID int64, unitIDs []int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.learn.untouchedLearnerUnits == nil {
		m.learn.untouchedLearnerUnits = map[int64][]int64{}
	}
	m.learn.untouchedLearnerUnits[learnerID] = slices.Clone(unitIDs)
}

// SetReviseUnitOK marks a unit as an allowed revision target. INIT-030/SPEC-005.
func (m *Memory) SetReviseUnitOK(unitID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.learn.reviseOK == nil {
		m.learn.reviseOK = map[int64]bool{}
	}
	m.learn.reviseOK[unitID] = true
}

// SetReviseUnitRefuse makes BuildRevisionBrief return err for unitID. INIT-030/SPEC-005.
func (m *Memory) SetReviseUnitRefuse(unitID int64, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.learn.reviseRefuse == nil {
		m.learn.reviseRefuse = map[int64]error{}
	}
	m.learn.reviseRefuse[unitID] = err
}

// BuildLessonBrief implements LearnerData. The brief is the learner's template with a new id.
func (m *Memory) BuildLessonBrief(ctx context.Context, id int64, theme string) (LessonBrief, error) {
	if err := ctx.Err(); err != nil {
		return LessonBrief{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if ids := m.learn.untouchedLearnerUnits[id]; len(ids) >= 3 {
		return LessonBrief{}, ErrUntouchedBacklog{UnitID: ids[0]}
	}
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

// BuildRevisionBrief implements LearnerData for in-memory tests. INIT-030/SPEC-005.
func (m *Memory) BuildRevisionBrief(ctx context.Context, id int64, theme string, unitID int64) (LessonBrief, error) {
	if err := ctx.Err(); err != nil {
		return LessonBrief{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err, ok := m.learn.reviseRefuse[unitID]; ok {
		return LessonBrief{}, err
	}
	if !m.learn.reviseOK[unitID] {
		return LessonBrief{}, ErrReviseOtherLearner
	}
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

// AddCoachNote implements LearnerData. It refuses a note when that authorRole already has
// its daily cap from the last 24 hours.
func (m *Memory) AddCoachNote(ctx context.Context, id int64, author, authorRole, body string) (CoachNote, error) {
	if err := ctx.Err(); err != nil {
		return CoachNote{}, err
	}
	cap := maxCoachNotesPerDay
	if authorRole == "learner" {
		cap = maxLearnerCoachNotesPerDay
	}
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()
	recent := 0
	for _, n := range m.learn.notes {
		if n.user == id && n.note.AuthorRole == authorRole && n.note.CreatedAt.After(now.Add(-24*time.Hour)) {
			recent++
		}
	}
	if recent >= cap {
		return CoachNote{}, ErrTooManyNotes
	}
	m.learn.nextNote++
	note := CoachNote{ID: m.learn.nextNote, Author: author, AuthorRole: authorRole, Body: body, CreatedAt: now}
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
