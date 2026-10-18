package path

import (
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/learner/config"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()
	t.Setenv(config.EnvCheckpointPassMark, "")
	t.Setenv(config.EnvReviewStopDueCount, "")
	t.Setenv(config.EnvReviewStopGapLessons, "")
	t.Setenv(config.EnvLessonWarmupDue, "")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func les(id string, words ...string) lessonDoc {
	return lessonDoc{ID: id, Title: id, NewWords: words, Visibility: "everyone"}
}

func TestAssembleFrontierAndStop(t *testing.T) {
	cfg := testConfig(t)
	doc := planDoc{
		ID:    "course",
		Title: "Course",
		Units: []unitDoc{{
			ID: "u",
			Lessons: []lessonDoc{
				les("a"), les("b"), les("c"), les("d"), les("e"), les("f"),
			},
		}},
	}
	finished := map[string]bool{"a": true}
	got := assemble(doc, finished, cfg.ReviewStopDueCount, cfg)
	wantStates := []State{StateDone, StateCurrent, StateNext, StateNext, StateNext, StateNext}
	if len(got.Units) != 1 || len(got.Units[0].Lessons) != len(wantStates) {
		t.Fatalf("lessons %+v", got.Units)
	}
	for i, lesson := range got.Units[0].Lessons {
		if lesson.State != wantStates[i] {
			t.Fatalf("lesson %s state %s", lesson.ID, lesson.State)
		}
	}
	if got.Units[0].State != StateCurrent {
		t.Fatalf("unit %s", got.Units[0].State)
	}
	var nodes []string
	for _, node := range got.Nodes {
		if node.Kind == KindReviewStop {
			nodes = append(nodes, "stop:"+node.AfterLessonID)
			if node.State != StateReviewStop || node.LessonID != "" {
				t.Fatalf("stop node %+v", node)
			}
			continue
		}
		nodes = append(nodes, string(node.State)+":"+node.LessonID)
	}
	wantNodes := []string{"done:a", "current:b", "stop:b", "next:c", "next:d", "next:e", "stop:e", "next:f"}
	if stringsJoin(nodes) != stringsJoin(wantNodes) {
		t.Fatalf("nodes %v", nodes)
	}
	if got.WarmupLimit != cfg.LessonWarmupDue || got.ReviewStopGap != cfg.ReviewStopGapLessons {
		t.Fatalf("config not copied: %+v", got)
	}

	below := assemble(doc, finished, cfg.ReviewStopDueCount-1, cfg)
	for _, node := range below.Nodes {
		if node.Kind == KindReviewStop {
			t.Fatal("stop inserted below the threshold")
		}
	}
}

func TestAssembleUsesConfigGap(t *testing.T) {
	t.Setenv(config.EnvReviewStopDueCount, "1")
	t.Setenv(config.EnvReviewStopGapLessons, "1")
	t.Setenv(config.EnvCheckpointPassMark, "")
	t.Setenv(config.EnvLessonWarmupDue, "")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	doc := planDoc{ID: "c", Units: []unitDoc{{ID: "u", Lessons: []lessonDoc{
		les("a"), les("b"), les("c"), les("d"),
	}}}}
	got := assemble(doc, nil, 1, cfg)
	var stops []string
	for _, node := range got.Nodes {
		if node.Kind == KindReviewStop {
			stops = append(stops, node.AfterLessonID)
		}
	}
	// gap 1: stop after current a, skip b, stop after c, skip d.
	if stringsJoin(stops) != "a,c" {
		t.Fatalf("stops %v", stops)
	}
}

func TestAssembleSoftGateAndTestOut(t *testing.T) {
	cfg := testConfig(t)
	intro := les("intro", "א")
	check := les("check")
	check.Checkpoint = true
	next := les("next", "ב")
	doc := planDoc{ID: "c", Units: []unitDoc{
		{ID: "first", Lessons: []lessonDoc{intro, check}},
		{ID: "second", Prerequisites: []string{"first"}, Lessons: []lessonDoc{next}},
	}}
	blocked := assemble(doc, nil, 0, cfg)
	if blocked.Units[0].Passed || blocked.Units[0].State != StateCurrent {
		t.Fatalf("first %+v", blocked.Units[0])
	}
	if !blocked.Units[1].NotYet || blocked.Units[1].State != StateNotYet {
		t.Fatalf("second %+v", blocked.Units[1])
	}
	if blocked.Units[1].Lessons[0].State != StateNotYet {
		t.Fatalf("gated lesson %s", blocked.Units[1].Lessons[0].State)
	}

	// Checkpoint finished, intro not. The unit is passed. The second unit opens.
	opened := assemble(doc, map[string]bool{"check": true}, 0, cfg)
	if !opened.Units[0].Passed || opened.Units[0].State != StateDone {
		t.Fatalf("passed unit %+v", opened.Units[0])
	}
	if opened.Units[0].Lessons[0].State != StateNext || opened.Units[0].Lessons[0].Done {
		t.Fatalf("unfinished lesson in a passed unit %+v", opened.Units[0].Lessons[0])
	}
	if opened.Units[1].NotYet || opened.Units[1].Lessons[0].State != StateCurrent {
		t.Fatalf("second after test out %+v", opened.Units[1])
	}
	if opened.Units[0].NewWordsIntroduced != 1 || opened.Units[0].NewWordsDone != 0 {
		t.Fatalf("words x=%d y=%d", opened.Units[0].NewWordsDone, opened.Units[0].NewWordsIntroduced)
	}
	doneWords := assemble(doc, map[string]bool{"intro": true, "check": true}, 0, cfg)
	if doneWords.Units[0].NewWordsDone != 1 {
		t.Fatalf("x=%d", doneWords.Units[0].NewWordsDone)
	}
}

func TestAssembleNoCheckpointNeedsEveryLesson(t *testing.T) {
	cfg := testConfig(t)
	doc := planDoc{ID: "c", Units: []unitDoc{{ID: "u", Lessons: []lessonDoc{les("a"), les("b")}}}}
	if assemble(doc, map[string]bool{"a": true}, 0, cfg).Units[0].Passed {
		t.Fatal("passed with a lesson unfinished and no checkpoint")
	}
	got := assemble(doc, map[string]bool{"a": true, "b": true}, 99, cfg)
	if !got.Units[0].Passed || got.Units[0].State != StateDone {
		t.Fatalf("%+v", got.Units[0])
	}
	for _, node := range got.Nodes {
		if node.Kind == KindReviewStop {
			t.Fatal("stop with no current lesson")
		}
	}
}

func TestAssembleOrderAndWords(t *testing.T) {
	cfg := testConfig(t)
	doc := planDoc{ID: "c", Units: []unitDoc{
		{ID: "b", Index: 0, HasOrder: true, Order: 2, Lessons: []lessonDoc{les("b1")}},
		{ID: "a", Index: 1, HasOrder: true, Order: 1, Lessons: []lessonDoc{les("a1", "  א  ", "א"), les("a2", "ב")}},
		{ID: "c", Index: 2, Lessons: []lessonDoc{les("c1")}},
	}}
	// c has no order. Any unit sets order, so c sorts as 0 and comes first.
	got := assemble(doc, map[string]bool{"a1": true}, 0, cfg)
	var ids []string
	for _, unit := range got.Units {
		ids = append(ids, unit.ID)
	}
	if stringsJoin(ids) != "c,a,b" {
		t.Fatalf("order %v", ids)
	}
	if got.Units[1].NewWordsIntroduced != 2 || got.Units[1].NewWordsDone != 1 {
		t.Fatalf("words %+v", got.Units[1])
	}

	plain := planDoc{ID: "c", Units: []unitDoc{
		{ID: "b", Index: 0, Lessons: []lessonDoc{les("b1")}},
		{ID: "a", Index: 1, Lessons: []lessonDoc{les("a1")}},
	}}
	array := assemble(plain, nil, 0, cfg)
	if array.Units[0].ID != "b" || array.Units[1].ID != "a" {
		t.Fatalf("array order %s %s", array.Units[0].ID, array.Units[1].ID)
	}
}

func TestAssembleIntroducesWordsAndLegacySlug(t *testing.T) {
	cfg := testConfig(t)
	lesson := les("morning", "שלום")
	lesson.LegacyKey = "my-morning"
	unit := unitDoc{
		ID:            "u",
		HasIntroduces: true,
		Introduces:    []string{"שלום", "שלום", "בוקר"},
		Lessons:       []lessonDoc{lesson},
	}
	got := assemble(planDoc{ID: "c", Units: []unitDoc{unit}}, map[string]bool{"my-morning": true}, 0, cfg)
	if !got.Units[0].Lessons[0].Done || got.Units[0].NewWordsIntroduced != 2 || got.Units[0].NewWordsDone != 1 {
		t.Fatalf("%+v words %d/%d", got.Units[0].Lessons[0], got.Units[0].NewWordsDone, got.Units[0].NewWordsIntroduced)
	}

	emptyIntro := unit
	emptyIntro.Introduces = nil
	emptyIntro.HasIntroduces = true
	emptyIntro.Lessons = []lessonDoc{les("x", "שלום")}
	y := assemble(planDoc{ID: "c", Units: []unitDoc{emptyIntro}}, map[string]bool{"x": true}, 0, cfg)
	if y.Units[0].NewWordsIntroduced != 0 || y.Units[0].NewWordsDone != 0 {
		t.Fatalf("empty introduces_words fell back to newWords: %+v", y.Units[0])
	}
}

func TestParseSnapshotCheckpointAndOrder(t *testing.T) {
	text := `{
	  "status":"published","audience":"personal","assigneeId":"path-learner","title":"T",
	  "units":[
	    {"id":"b","title":"B","theme":"t","order":2,"lessons":[
	      {"id":"b1","title":"B1","visibility":"everyone","kind":"checkpoint","newWords":[],"recycles":[],"blocks":[{"type":"fluency-pass"}]}
	    ]},
	    {"id":"a","title":"A","theme":"t","order":1,"prerequisites":["missing"],"introduces_words":["א"],"source":"tutor","tutor_class_date":"2026-10-01","lessons":[
	      {"id":"a1","title":"A1","visibility":"everyone","legacyStorageKey":"my-morning","newWords":["  א  "],"recycles":[{"word":"  ב  ","fromLessonId":"old"}],"blocks":[{"type":"spaced-review-checkpoint"}]}
	    ]}
	  ]
	}`
	doc, err := parseSnapshot("course", text)
	if err != nil {
		t.Fatal(err)
	}
	got := assemble(doc, map[string]bool{"my-morning": true}, 0, testConfig(t))
	if got.Units[0].ID != "a" || got.Units[1].ID != "b" {
		t.Fatalf("ids %s %s", got.Units[0].ID, got.Units[1].ID)
	}
	if got.Units[0].Source != "tutor" || got.Units[0].TutorClassDate != "2026-10-01" {
		t.Fatalf("tutor %+v", got.Units[0])
	}
	if !got.Units[0].Lessons[0].Done || !got.Units[0].Passed {
		t.Fatalf("checkpoint via block not passed: %+v", got.Units[0])
	}
	if got.Units[0].Lessons[0].Recycles[0].Word != "ב" {
		t.Fatalf("recycle %v", got.Units[0].Lessons[0].Recycles)
	}
	if got.Units[1].Passed {
		t.Fatalf("kind checkpoint unfinished should not pass: %+v", got.Units[1])
	}
	// b1 has kind checkpoint and is not finished, so unit b is not passed.
	if got.Units[1].Lessons[0].State != StateNotYet && got.Units[1].NotYet {
		t.Fatal("unexpected")
	}
	if !got.Units[0].NotYet {
		t.Fatal("missing prerequisite should soft-gate unit a")
	}
	if _, err := parseSnapshot("course", `{"units":[{"id":"a","order":1.5,"lessons":[]}]}`); err == nil {
		t.Fatal("fractional order accepted")
	}
}

func stringsJoin(items []string) string {
	out := ""
	for i, item := range items {
		if i > 0 {
			out += ","
		}
		out += item
	}
	return out
}
