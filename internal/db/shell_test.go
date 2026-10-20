package db

import (
	"errors"
	"fmt"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// The reads behind the app shell (INIT-015/SPEC-017): a unit the learner reported is not
// offered, a band slip is read back with its band, and a lesson is in progress only after
// an answer.

// shellUnit saves a published unit with the given number of one-step lessons.
func (e *lpEnv) shellUnit(scope, title string, position, lessons int) int64 {
	e.t.Helper()
	doc := units.Unit{Title: title, Position: position}
	for i := 0; i < lessons; i++ {
		doc.Lessons = append(doc.Lessons, units.Lesson{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: "OK"}, {Type: units.ActivityStory, Text: "OK"},
		}})
	}
	res, err := e.s.PutUnit(e.ctx, nil, doc, scope, "test")
	if err != nil {
		e.t.Fatal(err)
	}
	e.exec(`UPDATE units SET status = 'published' WHERE id = ?`, res.UnitID)
	return res.UnitID
}

func TestReportedUnitIsNotOfferedByTheLessonReads(t *testing.T) {
	e := newLP(t)
	first := e.shellUnit("shared", "First", 1, 1)
	second := e.shellUnit("shared", "Second", 2, 1)
	mine := e.shellUnit(LearnerScope(e.a), "Mine", 3, 1)

	next, ok, err := e.s.NextCourseLesson(e.ctx, e.a)
	if err != nil || !ok || next.UnitID != first {
		t.Fatalf("before the report: %v %v %v", next.UnitID, ok, err)
	}
	if _, err := e.s.ReportUnit(e.ctx, Report{UserID: e.a, UnitID: first, Reason: "wrong_hebrew"}); err != nil {
		t.Fatal(err)
	}
	next, ok, err = e.s.NextCourseLesson(e.ctx, e.a)
	if err != nil || !ok || next.UnitID != second {
		t.Fatalf("after the report the next lesson is unit %d, want %d (%v %v)", next.UnitID, second, ok, err)
	}
	if _, err := e.s.CourseLesson(e.ctx, e.a, first, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CourseLesson of a reported unit: %v, want ErrNotFound", err)
	}
	if got, err := e.s.CourseLesson(e.ctx, e.a, second, 0); err != nil || got.UnitID != second {
		t.Fatalf("CourseLesson of another unit: %v %v", got.UnitID, err)
	}
	// Another learner is not affected.
	if next, ok, err := e.s.NextCourseLesson(e.ctx, e.b); err != nil || !ok || next.UnitID != first {
		t.Fatalf("learner b: %v %v %v", next.UnitID, ok, err)
	}
	if _, err := e.s.CourseLesson(e.ctx, e.b, first, 0); err != nil {
		t.Fatalf("learner b cannot open the unit: %v", err)
	}
	// Reporting everything leaves nothing to offer: ok is false, not an error.
	for _, id := range []int64{second, mine} {
		if _, err := e.s.ReportUnit(e.ctx, Report{UserID: e.a, UnitID: id, Reason: "other"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok, err := e.s.NextCourseLesson(e.ctx, e.a); err != nil || ok {
		t.Fatalf("all reported: ok=%v err=%v", ok, err)
	}
}

func TestLearnerUnitProgress(t *testing.T) {
	e := newLP(t)
	done := e.shellUnit("shared", "Done", 1, 2)
	partway := e.shellUnit("shared", "Partway", 2, 3)
	reported := e.shellUnit("shared", "Reported", 3, 1)
	own := e.shellUnit(LearnerScope(e.a), "Own", 4, 1)
	e.shellUnit(LearnerScope(e.b), "Theirs", 5, 1)
	draft, err := e.s.PutUnit(e.ctx, nil, units.Unit{Title: "Draft", Position: 6, Lessons: []units.Lesson{{Kind: units.KindLesson}}}, "shared", "test")
	if err != nil || draft.UnitID == 0 {
		t.Fatal(err)
	}
	empty := e.shellUnit("shared", "Empty", 7, 0)
	_ = empty

	finish := func(unit int64, ix int) {
		e.exec(`INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score) VALUES (?, ?, ?, ?, ?, 1)`,
			e.a, unit, ix, FormatTime(lpNow), FormatTime(lpNow))
	}
	finish(done, 0)
	finish(done, 1)
	finish(partway, 0)
	if _, err := e.s.ReportUnit(e.ctx, Report{UserID: e.a, UnitID: reported, Reason: "other"}); err != nil {
		t.Fatal(err)
	}

	got, err := e.s.LearnerUnitProgress(e.ctx, e.a)
	if err != nil {
		t.Fatal(err)
	}
	want := []UnitProgress{
		{UnitID: done, Position: 1, Title: "Done", Scope: "shared", Lessons: 2, Finished: 2},
		{UnitID: partway, Position: 2, Title: "Partway", Scope: "shared", Lessons: 3, Finished: 1},
		{UnitID: own, Position: 4, Title: "Own", Scope: LearnerScope(e.a), Lessons: 1, Finished: 0},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if _, err := e.s.LearnerUnitProgress(e.ctx, 0); !errors.Is(err, ErrNoLearner) {
		t.Fatalf("no learner: %v", err)
	}
}

func TestRecentBandSlips(t *testing.T) {
	e := newLP(t)
	slip := func(user int64, day, cause string) {
		e.t.Helper()
		e.exec(`INSERT INTO regression_events (user_id, kind, day, cause) VALUES (?, 'band_slip', ?, ?)`, user, day, cause)
	}
	word := func(user int64, day string) {
		e.t.Helper()
		e.exec(`INSERT INTO lexemes (id, text_nfc) VALUES (1, 'x') ON CONFLICT DO NOTHING`)
		e.exec(`INSERT INTO regression_events (user_id, kind, lexeme_id, skill, from_state, to_state, day, cause) VALUES (?, 'word', 1, 'read', 'known', 'struggling', ?, 'c')`, user, day)
	}
	slip(e.a, "2026-10-10", "band_slip band=1 known share 0.80 to 0.60 since 2026-10-03")
	slip(e.a, "2026-10-14", "band_slip band=3 known share 0.70 to 0.50 since 2026-10-07")
	slip(e.a, "2026-10-14", "band_slip band=0 known share 0.90 to 0.70 since 2026-10-07")
	slip(e.a, "2026-10-15", "band_slip no band here")
	slip(e.b, "2026-10-14", "band_slip band=2 known share 0.70 to 0.50 since 2026-10-07")
	word(e.a, "2026-10-14")

	got, err := e.s.RecentBandSlips(e.ctx, e.a, "2026-10-12")
	if err != nil {
		t.Fatal(err)
	}
	// Newest day first, then the later row first. The row with no band, the other learner's
	// row, the word row and the row before the cut are not here.
	want := []BandSlip{{Band: 0, Day: "2026-10-14"}, {Band: 3, Day: "2026-10-14"}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if all, _ := e.s.RecentBandSlips(e.ctx, e.a, "2026-10-10"); len(all) != 3 {
		t.Fatalf("since the 10th: %v", all)
	}
	if none, err := e.s.RecentBandSlips(e.ctx, e.a, "2026-10-16"); err != nil || len(none) != 0 {
		t.Fatalf("since the 16th: %v %v", none, err)
	}
	if _, err := e.s.RecentBandSlips(e.ctx, 0, "2026-10-01"); !errors.Is(err, ErrNoLearner) {
		t.Fatalf("no learner: %v", err)
	}
}

func TestSlipBand(t *testing.T) {
	for cause, want := range map[string]int{
		"band_slip band=0 known share 0.8 to 0.6": 0,
		"band_slip band=4 x":                      4,
		"band_slip band=12":                       12,
	} {
		if got, ok := slipBand(cause); !ok || got != want {
			t.Errorf("slipBand(%q) = %d %v", cause, got, ok)
		}
	}
	for _, cause := range []string{"", "band_slip", "band_slip band= x", "band_slip band=-1 x", "slip band=1 x", "band_slip band=a x"} {
		if _, ok := slipBand(cause); ok {
			t.Errorf("slipBand(%q) read a band", cause)
		}
	}
}

func TestInProgressLessonRun(t *testing.T) {
	e := newLP(t)
	unit := e.shellUnit("shared", "U", 1, 1)

	run, err := e.s.StartPlayerRun(e.ctx, e.a, unit, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := e.s.InProgressLessonRun(e.ctx, e.a, unit, 0); err != nil || ok {
		t.Fatalf("an opened lesson is in progress: %v %v", ok, err)
	}
	if _, _, err := e.s.ContinuePlayerRun(e.ctx, e.a, run.ID, 0, 2); err != nil {
		t.Fatal(err)
	}
	got, ok, err := e.s.InProgressLessonRun(e.ctx, e.a, unit, 0)
	if err != nil || !ok || got.ActivityIx != 1 {
		t.Fatalf("after one step: %+v %v %v", got, ok, err)
	}
	if _, ok, _ := e.s.InProgressLessonRun(e.ctx, e.b, unit, 0); ok {
		t.Fatal("another learner's progress")
	}
	if _, ok, _ := e.s.InProgressLessonRun(e.ctx, e.a, unit, 1); ok {
		t.Fatal("another lesson's progress")
	}
	e.exec(`UPDATE lesson_runs SET finished_at = ?, score = 1 WHERE id = ?`, FormatTime(lpNow), run.ID)
	if _, ok, _ := e.s.InProgressLessonRun(e.ctx, e.a, unit, 0); ok {
		t.Fatal("a finished lesson is in progress")
	}

	// An answer shown, on the first step, is progress too.
	run2, err := e.s.StartPlayerRun(e.ctx, e.b, unit, 0)
	if err != nil {
		t.Fatal(err)
	}
	e.exec(`UPDATE lesson_runs SET phase = 'reveal' WHERE id = ?`, run2.ID)
	if _, ok, err := e.s.InProgressLessonRun(e.ctx, e.b, unit, 0); err != nil || !ok {
		t.Fatalf("a revealed first step: %v %v", ok, err)
	}
}
