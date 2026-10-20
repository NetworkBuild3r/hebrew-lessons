package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// These tests are the store half of INIT-015/SPEC-020: auto publish with a brief,
// unpublish, and the learner's report.

var lpNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

const (
	lpBriefA = "AAAAAAAAAAAAAAAAAAAAAA"
	lpBriefB = "BBBBBBBBBBBBBBBBBBBBBB"
)

type lpEnv struct {
	t   *testing.T
	s   *Store
	a   int64
	b   int64
	ctx context.Context
}

func newLP(t *testing.T) *lpEnv {
	t.Helper()
	s := openStore(t, fixedClock{lpNow})
	ctx := context.Background()
	a, err := s.InsertUser(ctx, "a@example.com", "sub-a", "learner")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.InsertUser(ctx, "b@example.com", "sub-b", "learner")
	if err != nil {
		t.Fatal(err)
	}
	return &lpEnv{t: t, s: s, a: a, b: b, ctx: ctx}
}

func (e *lpEnv) exec(query string, args ...any) {
	e.t.Helper()
	conn, err := e.s.DB()
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := conn.ExecContext(e.ctx, query, args...); err != nil {
		e.t.Fatalf("exec %s: %v", query, err)
	}
}

func (e *lpEnv) count(query string, args ...any) int {
	e.t.Helper()
	conn, err := e.s.DB()
	if err != nil {
		e.t.Fatal(err)
	}
	var n int
	if err := conn.QueryRowContext(e.ctx, query, args...).Scan(&n); err != nil {
		e.t.Fatalf("count %s: %v", query, err)
	}
	return n
}

func (e *lpEnv) text(query string, args ...any) string {
	e.t.Helper()
	conn, err := e.s.DB()
	if err != nil {
		e.t.Fatal(err)
	}
	var v string
	if err := conn.QueryRowContext(e.ctx, query, args...).Scan(&v); err != nil {
		e.t.Fatalf("text %s: %v", query, err)
	}
	return v
}

func (e *lpEnv) brief(id string, user int64, created time.Time) {
	e.t.Helper()
	e.exec(`INSERT INTO lesson_briefs (id, user_id, theme, payload_json, created_at) VALUES (?, ?, 'general', '{"v":1}', ?)`,
		id, user, FormatTime(created))
}

// unit saves a draft with a distinct title and returns its id.
func (e *lpEnv) unit(scope, title string) int64 {
	e.t.Helper()
	res, err := e.s.PutUnit(e.ctx, nil, units.Unit{Title: title, Situation: "s", Concept: "c"}, scope, "admin")
	if err != nil {
		e.t.Fatal(err)
	}
	return res.UnitID
}

func (e *lpEnv) status(id int64) string {
	e.t.Helper()
	return e.text(`SELECT status FROM units WHERE id = ?`, id)
}

func (e *lpEnv) publish(id int64) {
	e.t.Helper()
	if _, err := e.s.PublishUnit(e.ctx, id, nil); err != nil {
		e.t.Fatal(err)
	}
}

func (e *lpEnv) auto(unit, learner int64, brief string) (PutResult, error) {
	return e.s.AutoPublishLearnerUnit(e.ctx, AutoPublish{
		UnitID: unit, Revision: 1, LearnerID: learner, BriefID: brief,
		BriefNotBefore: lpNow.Add(-7 * 24 * time.Hour), Actor: "claude",
		IssuesJSON: `[{"rule":"recycle.count","severity":"warn","pointer":"/newWords/0","msg":"m"}]`,
	})
}

func (e *lpEnv) audits() int { return e.count(`SELECT COUNT(*) FROM publish_audit`) }

func TestAutoPublishLearnerUnitWritesOneAuditRow(t *testing.T) {
	e := newLP(t)
	e.brief(lpBriefA, e.a, lpNow.Add(-time.Hour))
	id := e.unit(LearnerScope(e.a), "mine")
	res, err := e.auto(id, e.a, lpBriefA)
	if err != nil || res.UnitID != id || res.Revision != 1 || res.Unchanged {
		t.Fatalf("publish %+v %v", res, err)
	}
	if e.status(id) != "published" {
		t.Fatalf("status %s", e.status(id))
	}
	if n := e.audits(); n != 1 {
		t.Fatalf("audit rows %d", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM publish_audit WHERE unit_id = ? AND revision = 1 AND scope = ? AND actor = 'claude' AND action = 'auto_publish' AND brief_id = ? AND created_at = ? AND issues_json LIKE '%recycle.count%'`,
		id, LearnerScope(e.a), lpBriefA, FormatTime(lpNow)); n != 1 {
		t.Fatalf("audit row has the wrong columns")
	}
	if got := e.count(`SELECT used_by_unit_id FROM lesson_briefs WHERE id = ?`, lpBriefA); int64(got) != id {
		t.Fatalf("used_by_unit_id = %d, want %d", got, id)
	}
}

func TestAutoPublishLearnerUnitRefusals(t *testing.T) {
	e := newLP(t)
	e.brief(lpBriefA, e.a, lpNow.Add(-time.Hour))
	e.brief(lpBriefB, e.b, lpNow.Add(-time.Hour))
	mine := e.unit(LearnerScope(e.a), "mine")
	theirs := e.unit(LearnerScope(e.b), "theirs")
	shared := e.unit("shared", "shared")

	// A unit that is not the learner's is refused, and the brief is not spent.
	for _, id := range []int64{theirs, shared} {
		if _, err := e.auto(id, e.a, lpBriefA); !errors.Is(err, ErrUnitNotInScope) {
			t.Fatalf("unit %d: %v", id, err)
		}
	}
	// Another learner's brief is not found, and is not spent.
	if _, err := e.auto(mine, e.a, lpBriefB); !errors.Is(err, ErrBriefNotFound) {
		t.Fatalf("other learner's brief: %v", err)
	}
	if _, err := e.auto(mine, e.a, "CCCCCCCCCCCCCCCCCCCCCC"); !errors.Is(err, ErrBriefNotFound) {
		t.Fatalf("unknown brief: %v", err)
	}
	if _, err := e.auto(mine, 0, lpBriefA); !errors.Is(err, ErrNoLearner) {
		t.Fatalf("no learner: %v", err)
	}
	// A missing revision rolls the brief claim back.
	if _, err := e.s.AutoPublishLearnerUnit(e.ctx, AutoPublish{UnitID: mine, Revision: 9, LearnerID: e.a, BriefID: lpBriefA,
		BriefNotBefore: lpNow.Add(-time.Hour * 24), Actor: "claude"}); !errors.Is(err, ErrRevisionNotFound) {
		t.Fatalf("missing revision: %v", err)
	}
	if n := e.count(`SELECT COUNT(*) FROM lesson_briefs WHERE used_by_unit_id IS NOT NULL`); n != 0 {
		t.Fatalf("%d briefs were spent by refused publishes", n)
	}
	if e.audits() != 0 || e.status(mine) != "draft" || e.status(theirs) != "draft" || e.status(shared) != "draft" {
		t.Fatalf("a refused publish left a trace: audits %d", e.audits())
	}
	// An old brief is expired.
	e.brief("DDDDDDDDDDDDDDDDDDDDDD", e.a, lpNow.Add(-8*24*time.Hour))
	if _, err := e.auto(mine, e.a, "DDDDDDDDDDDDDDDDDDDDDD"); !errors.Is(err, ErrBriefExpired) {
		t.Fatalf("expired brief: %v", err)
	}
	// The same brief works once, then it is used.
	if _, err := e.auto(mine, e.a, lpBriefA); err != nil {
		t.Fatal(err)
	}
	other := e.unit(LearnerScope(e.a), "second")
	if _, err := e.auto(other, e.a, lpBriefA); !errors.Is(err, ErrBriefUsed) {
		t.Fatalf("second use: %v", err)
	}
	if e.status(other) != "draft" || e.audits() != 1 {
		t.Fatalf("a used brief published a second unit or wrote a second row")
	}
	if _, err := e.s.AutoPublishLearnerUnit(e.ctx, AutoPublish{UnitID: mine, Revision: 1, LearnerID: e.a, BriefID: "", Actor: "x"}); !errors.Is(err, ErrBriefNotFound) {
		t.Fatalf("empty brief id: %v", err)
	}
	if _, err := e.s.AutoPublishLearnerUnit(e.ctx, AutoPublish{UnitID: mine, Revision: 1, LearnerID: e.a, BriefID: lpBriefA, Actor: " "}); err == nil {
		t.Fatal("empty actor accepted")
	}
	if _, err := e.s.AutoPublishLearnerUnit(e.ctx, AutoPublish{UnitID: mine, Revision: 1, LearnerID: e.a, BriefID: lpBriefA, Actor: "x", IssuesJSON: "{nope"}); err == nil {
		t.Fatal("issues that are not JSON accepted")
	}
}

// TestAutoPublishLearnerUnitConcurrent publishes many units with one brief at the
// same time. Exactly one may win, and the losers leave nothing behind: not a
// published unit, not an audit row.
func TestAutoPublishLearnerUnitConcurrent(t *testing.T) {
	e := newLP(t)
	e.brief(lpBriefA, e.a, lpNow.Add(-time.Hour))
	const n = 24
	ids := make([]int64, n)
	for i := range ids {
		ids[i] = e.unit(LearnerScope(e.a), fmt.Sprintf("unit %d", i))
	}
	errs := make([]error, n)
	var ready, wg sync.WaitGroup
	start := make(chan struct{})
	ready.Add(n)
	wg.Add(n)
	for i := range ids {
		go func() {
			defer wg.Done()
			ready.Done()
			<-start
			_, errs[i] = e.auto(ids[i], e.a, lpBriefA)
		}()
	}
	ready.Wait()
	close(start)
	wg.Wait()

	winners, used := 0, 0
	var winner int64
	for i, err := range errs {
		switch {
		case err == nil:
			winners++
			winner = ids[i]
		case errors.Is(err, ErrBriefUsed):
			used++
		default:
			t.Fatalf("unit %d: unexpected error %v", ids[i], err)
		}
	}
	if winners != 1 || used != n-1 {
		t.Fatalf("winners %d, brief used %d, want 1 and %d", winners, used, n-1)
	}
	if got := e.count(`SELECT COUNT(*) FROM units WHERE status = 'published'`); got != 1 {
		t.Fatalf("%d units published", got)
	}
	if e.status(winner) != "published" || e.audits() != 1 {
		t.Fatalf("winner %d status %s audits %d", winner, e.status(winner), e.audits())
	}
	if got := e.count(`SELECT used_by_unit_id FROM lesson_briefs WHERE id = ?`, lpBriefA); int64(got) != winner {
		t.Fatalf("brief used by %d, want %d", got, winner)
	}
}

func TestUnpublishLearnerUnit(t *testing.T) {
	e := newLP(t)
	mine := e.unit(LearnerScope(e.a), "mine")
	theirs := e.unit(LearnerScope(e.b), "theirs")
	shared := e.unit("shared", "shared")
	zero := e.unit(LearnerScope(0), "zero")
	for _, id := range []int64{mine, theirs, shared, zero} {
		e.publish(id)
	}

	// ac-4: another learner's unit, a shared unit, a missing unit, and a unit of
	// learner 0 are all refused, and nothing is written.
	for _, c := range []struct {
		name    string
		learner int64
		unit    int64
		want    error
	}{
		{"another learner's unit", e.a, theirs, ErrUnitNotInScope},
		{"a shared unit", e.a, shared, ErrUnitNotInScope},
		{"a missing unit", e.a, 9999, ErrUnitNotInScope},
		{"no learner", 0, zero, ErrNoLearner},
		{"negative learner", -1, mine, ErrNoLearner},
	} {
		if _, err := e.s.UnpublishLearnerUnit(e.ctx, c.learner, c.unit, "learner"); !errors.Is(err, c.want) {
			t.Fatalf("%s: %v", c.name, err)
		}
	}
	if e.audits() != 0 {
		t.Fatalf("a refused unpublish wrote %d audit rows", e.audits())
	}
	for _, id := range []int64{mine, theirs, shared, zero} {
		if e.status(id) != "published" {
			t.Fatalf("unit %d status %s after refusals", id, e.status(id))
		}
	}

	res, err := e.s.UnpublishLearnerUnit(e.ctx, e.a, mine, "learner-7")
	if err != nil || res.UnitID != mine || res.Unchanged || res.Revision != 1 {
		t.Fatalf("unpublish %+v %v", res, err)
	}
	if e.status(mine) != "draft" {
		t.Fatalf("status %s", e.status(mine))
	}
	if n := e.count(`SELECT COUNT(*) FROM publish_audit WHERE unit_id = ? AND action = 'unpublish' AND actor = 'learner-7' AND scope = ? AND revision = 1 AND brief_id IS NULL AND issues_json = '[]'`,
		mine, LearnerScope(e.a)); n != 1 || e.audits() != 1 {
		t.Fatalf("audit rows: matching %d, all %d", n, e.audits())
	}
	// A repeat changes nothing and writes nothing.
	again, err := e.s.UnpublishLearnerUnit(e.ctx, e.a, mine, "learner-7")
	if err != nil || !again.Unchanged || e.audits() != 1 {
		t.Fatalf("repeat %+v %v audits %d", again, err, e.audits())
	}
	if _, err := e.s.UnpublishLearnerUnit(e.ctx, e.a, mine, ""); err == nil {
		t.Fatal("empty actor accepted")
	}
}

func TestReportUnit(t *testing.T) {
	e := newLP(t)
	mine := e.unit(LearnerScope(e.a), "mine")
	theirs := e.unit(LearnerScope(e.b), "theirs")
	shared := e.unit("shared", "shared")
	draftShared := e.unit("shared", "draft shared")
	for _, id := range []int64{mine, theirs, shared} {
		e.publish(id)
	}

	for _, c := range []struct {
		name string
		unit int64
	}{{"another learner's unit", theirs}, {"a shared draft", draftShared}, {"a missing unit", 9999}} {
		if _, err := e.s.ReportUnit(e.ctx, Report{UserID: e.a, UnitID: c.unit, Reason: "wrong"}); !errors.Is(err, ErrUnitNotInScope) {
			t.Fatalf("%s: %v", c.name, err)
		}
	}
	for _, r := range []Report{
		{UserID: e.a, UnitID: mine, Reason: " "},
		{UserID: e.a, UnitID: mine, Reason: strings.Repeat("x", MaxReportReasonRunes+1)},
		{UserID: e.a, UnitID: mine, Reason: "ok", Note: strings.Repeat("n", MaxReportNoteRunes+1)},
		{UserID: e.a, UnitID: mine, Reason: "bad\x00"},
	} {
		if _, err := e.s.ReportUnit(e.ctx, r); !errors.Is(err, ErrBadReport) {
			t.Fatalf("report %+v: %v", r, err)
		}
	}
	if _, err := e.s.ReportUnit(e.ctx, Report{UnitID: mine, Reason: "x"}); !errors.Is(err, ErrNoLearner) {
		t.Fatalf("no learner: %v", err)
	}
	if e.audits() != 0 || e.count(`SELECT COUNT(*) FROM unit_reports`) != 0 || e.status(mine) != "published" {
		t.Fatal("a refused report left a trace")
	}

	// Own unit: unpublished, a report_hide audit row, and a report row.
	res, err := e.s.ReportUnit(e.ctx, Report{UserID: e.a, UnitID: mine, Reason: "wrong word", Note: "line 2"})
	if err != nil || !res.Hidden || res.ReportID == 0 {
		t.Fatalf("own report %+v %v", res, err)
	}
	if e.status(mine) != "draft" {
		t.Fatalf("own unit status %s", e.status(mine))
	}
	if n := e.count(`SELECT COUNT(*) FROM publish_audit WHERE unit_id = ? AND action = 'report_hide' AND actor = ? AND scope = ?`,
		mine, LearnerScope(e.a), LearnerScope(e.a)); n != 1 {
		t.Fatalf("own audit rows %d", n)
	}

	// Shared unit: it stays published for everyone, and is hidden for the reporter only.
	res, err = e.s.ReportUnit(e.ctx, Report{UserID: e.a, UnitID: shared, Reason: "wrong"})
	if err != nil || !res.Hidden {
		t.Fatalf("shared report %+v %v", res, err)
	}
	if e.status(shared) != "published" {
		t.Fatalf("shared unit status %s: a report must not unpublish it for everyone", e.status(shared))
	}
	hiddenA, err := e.s.HiddenUnitIDs(e.ctx, e.a)
	if err != nil || len(hiddenA) != 2 || hiddenA[0] != mine || hiddenA[1] != shared {
		t.Fatalf("hidden for the reporter %v %v", hiddenA, err)
	}
	if hiddenB, err := e.s.HiddenUnitIDs(e.ctx, e.b); err != nil || len(hiddenB) != 0 {
		t.Fatalf("hidden for another learner %v %v", hiddenB, err)
	}
	if e.audits() != 2 {
		t.Fatalf("audit rows %d, want 2", e.audits())
	}
	// A repeat report is recorded, and is not a new hide.
	res, err = e.s.ReportUnit(e.ctx, Report{UserID: e.a, UnitID: shared, Reason: "still wrong"})
	if err != nil || res.Hidden || e.audits() != 2 || e.count(`SELECT COUNT(*) FROM unit_reports WHERE unit_id = ?`, shared) != 2 {
		t.Fatalf("repeat %+v %v audits %d", res, err, e.audits())
	}
	if _, err := e.s.HiddenUnitIDs(e.ctx, 0); !errors.Is(err, ErrNoLearner) {
		t.Fatalf("hidden for no learner: %v", err)
	}
}
