package units

import (
	"strings"
	"testing"
)

func TestPatternOrderNamesTheLaterUnit(t *testing.T) {
	course := []CourseUnit{
		{ID: 1, Position: 8, Scope: "shared", Title: "Present", Pattern: "present-tense"},
		{ID: 2, Position: 2, Scope: "shared", Title: "Past", Pattern: "past-tense-ti"},
	}
	prereqs := map[string][]string{"past-tense-ti": {"present-tense"}}
	got := PatternOrderIssues("shared", 2, "past-tense-ti", course, prereqs)
	if len(got) != 1 || got[0].Severity != SeverityBlock || got[0].Rule != RulePatternOrder {
		t.Fatalf("%#v", got)
	}
	if !strings.Contains(got[0].Msg, "Present") {
		t.Fatalf("message %q", got[0].Msg)
	}
	warn := PatternOrderIssues("learner:4", 2, "past-tense-ti", course, prereqs)
	if len(warn) != 1 || warn[0].Severity != SeverityWarn {
		t.Fatalf("learner %#v", warn)
	}
	ok := PatternOrderIssues("shared", 9, "past-tense-ti", course, prereqs)
	if len(ok) != 0 {
		t.Fatalf("later unit should pass, got %#v", ok)
	}
}

func TestStablePathKeepsClassOrder(t *testing.T) {
	course := []CourseUnit{
		{ID: 1, Position: 1, Scope: "shared", Title: "A", Pattern: "a"},
		{ID: 2, Position: 2, Scope: "shared", Title: "B", Pattern: "b"},
		{ID: 3, Position: 3, Scope: "shared", Title: "C", Pattern: ""},
	}
	// b requires a, and a is already first, so the order stays.
	got := StablePath(course, map[string][]string{"b": {"a"}})
	want := []int64{1, 2, 3}
	if len(got) != len(want) {
		t.Fatal(got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v", got)
		}
	}
	// a requires b, so b moves before a, and c stays after both because nothing pulls it.
	swapped := StablePath(course, map[string][]string{"a": {"b"}})
	if swapped[0] != 2 || swapped[1] != 1 || swapped[2] != 3 {
		t.Fatalf("swapped %v", swapped)
	}
}

func TestWorksheetShape(t *testing.T) {
	doc := &Unit{Lessons: []Lesson{{Activities: []Activity{
		{Type: ActivityStory, Text: "a"},
		{Type: ActivityListenPick, Text: "b"},
	}}}}
	if got := WorksheetIssues(SheetStory, doc); len(got) != 1 {
		t.Fatalf("short story %#v", got)
	}
	doc.Lessons[0].Activities = append(doc.Lessons[0].Activities,
		Activity{Type: ActivityListenPick, Text: "c"},
		Activity{Type: ActivityListenPick, Text: "d"})
	if got := WorksheetIssues(SheetStory, doc); len(got) != 0 {
		t.Fatalf("complete story %#v", got)
	}
	if got := WorksheetIssues("nope", doc); len(got) != 1 {
		t.Fatalf("unknown %#v", got)
	}
}
