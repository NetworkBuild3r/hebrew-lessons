package mcp

// INIT-030/SPEC-005: unit progress on get_context, revision briefs, backlog cap,
// and smoke-token isolation (Memory-backed).

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// TestGetContextProgressWithLearnerId covers outline progress with and without
// learnerId. INIT-030/SPEC-005 AC: Outline rows carry progress with learnerId, and omit it without.
func TestGetContextProgressWithLearnerId(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	doc := units.Unit{Title: "One", Situation: "s", Concept: "c", Lessons: []units.Lesson{{}}}
	saved, err := mem.PutUnit(context.Background(), nil, &doc, ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	mem.SetUnitProgress(7, []UnitProgressRow{{
		UnitID: saved.UnitID, State: "started", LessonsFinished: 0, LessonsTotal: 1, LastFinishedAt: at,
	}})

	session := connectMCP(t, Handler(Options{
		AuthToken: testToken, Course: mem, Lexicon: mem, Lookup: mem, Units: mem,
	}), testToken)

	shared := call[getContextOutput](t, session, "get_context", getContextInput{Position: 0})
	if len(shared.Outline) != 1 || shared.Outline[0].Progress != nil {
		t.Fatalf("without learnerId progress must be omitted: %+v", shared.Outline)
	}

	id := int64(7)
	with := call[getContextOutput](t, session, "get_context", getContextInput{Position: 0, LearnerID: &id})
	if len(with.Outline) != 1 || with.Outline[0].Progress == nil {
		t.Fatalf("with learnerId want progress: %+v", with.Outline)
	}
	p := with.Outline[0].Progress
	if p.State != "started" || p.LessonsFinished != 0 || p.LessonsTotal != 1 || p.LastFinishedAt == nil || !p.LastFinishedAt.Equal(at) {
		t.Fatalf("progress %+v", p)
	}
}

// TestLessonBriefRevisionRefusals covers started, finished, shared, and another
// learner's units. INIT-030/SPEC-005 AC: revision brief refusals.
func TestLessonBriefRevisionRefusals(t *testing.T) {
	mem := NewMemory()
	mem.AddLearner(7)
	mem.SetBriefTemplate(7, LessonBrief{Theme: "cafe"})
	mem.SetReviseUnitOK(10)
	mem.SetReviseUnitRefuse(11, ErrReviseStartedUnit)
	mem.SetReviseUnitRefuse(12, ErrReviseFinished)
	mem.SetReviseUnitRefuse(13, ErrReviseShared)
	mem.SetReviseUnitRefuse(14, ErrReviseOtherLearner)

	session := connectMCP(t, Handler(Options{
		AuthToken: testToken, Course: mem, Lexicon: mem, Lookup: mem, Units: mem,
	}), testToken)

	id := int64(7)
	okUnit := int64(10)
	got := call[LessonBrief](t, session, "get_lesson_brief", map[string]any{"learnerId": id, "unitId": okUnit})
	if got.BriefID == "" {
		t.Fatalf("untouched revision brief empty: %+v", got)
	}

	for _, c := range []struct {
		unit int64
		want string
	}{
		{11, "started"},
		{12, "finished"},
		{13, "shared"},
		{14, "another learner"},
		{99, "another learner"},
	} {
		raw := callErr(t, session, "get_lesson_brief", map[string]any{"learnerId": id, "unitId": c.unit})
		if !strings.Contains(strings.ToLower(raw), c.want) {
			t.Fatalf("unit %d: want %q in %s", c.unit, c.want, raw)
		}
	}
}

// TestLessonBriefUntouchedBacklog covers the three-unit cap. INIT-030/SPEC-005 AC:
// Three untouched units refuse a new brief and name the earliest. Two allow it.
func TestLessonBriefUntouchedBacklog(t *testing.T) {
	mem := NewMemory()
	mem.AddLearner(7)
	mem.SetBriefTemplate(7, LessonBrief{Theme: "cafe"})
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken, Course: mem, Lexicon: mem, Lookup: mem, Units: mem,
	}), testToken)
	id := int64(7)

	mem.SetUntouchedLearnerUnits(7, []int64{101, 102})
	if got := call[LessonBrief](t, session, "get_lesson_brief", map[string]any{"learnerId": id}); got.BriefID == "" {
		t.Fatal("two untouched units must still allow a new brief")
	}

	mem.SetUntouchedLearnerUnits(7, []int64{201, 202, 203})
	raw := callErr(t, session, "get_lesson_brief", map[string]any{"learnerId": id})
	if !strings.Contains(raw, "201") || !strings.Contains(strings.ToLower(raw), "untouched") {
		t.Fatalf("three untouched must name earliest 201: %s", raw)
	}
}

// TestSmokeProgressOwnOnlyAndNoRevisionPath covers smoke-token progress isolation
// and that the revision path stays admin-only. INIT-030/SPEC-005 AC: smoke token.
func TestSmokeProgressOwnOnlyAndNoRevisionPath(t *testing.T) {
	mem, srv := readOnlyHandler(t, false)
	ownUnit, err := mem.PutUnit(context.Background(), nil, &units.Unit{Title: "Own", Situation: "s", Concept: "c"}, ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	mem.SetUnitProgress(readOnlyLearner, []UnitProgressRow{{
		UnitID: ownUnit.UnitID, State: "untouched", LessonsFinished: 0, LessonsTotal: 1,
	}})
	mem.SetUnitProgress(9, []UnitProgressRow{{
		UnitID: 999, State: "finished", LessonsFinished: 2, LessonsTotal: 2,
	}})

	self := readOnlyLearner
	got := postTool(t, srv.URL, readOnlySmokeToken, "get_context", getContextInput{Position: 0, LearnerID: &self}, nil)
	body := readBody(t, got)
	if got.StatusCode != 200 || strings.Contains(body, `"isError":true`) {
		t.Fatalf("self get_context: %s", body)
	}
	if !strings.Contains(body, `"progress"`) || !strings.Contains(body, `"untouched"`) {
		t.Fatalf("self missing own progress: %s", body)
	}
	if strings.Contains(body, `"unitId":999`) || (strings.Contains(body, `"finished"`) && strings.Contains(body, `"lessonsFinished":2`)) {
		t.Fatalf("self saw another learner's progress: %s", body)
	}

	other := int64(9)
	denied := postTool(t, srv.URL, readOnlySmokeToken, "get_context", getContextInput{Position: 0, LearnerID: &other}, nil)
	assertDenied(t, denied, msgOtherLearner)

	rev := postTool(t, srv.URL, readOnlySmokeToken, "get_lesson_brief", map[string]any{
		"learnerId": readOnlyLearner, "unitId": ownUnit.UnitID,
	}, nil)
	assertDenied(t, rev, msgAdminOnly)
}
