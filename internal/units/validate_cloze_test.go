package units

import (
	"testing"
)

func clozeUnit(acts ...Activity) *Unit {
	return &Unit{Lessons: []Lesson{{Kind: KindLesson, Activities: acts}}}
}

func rulesIn(issues []Issue, id string) []Issue {
	var out []Issue
	for _, is := range issues {
		if is.Rule == id {
			out = append(out, is)
		}
	}
	return out
}

func TestClozeNeedsContextAndTranslation(t *testing.T) {
	const (
		ani = "אֲנִי"
		gar = "גָּר"
		et  = "אֶת"
	)
	good := clozeUnit(Activity{Type: ActivityCloze, Text: ani + " " + gar + ".", Target: gar, Translation: "I live."})
	if got := clozeIssues(good, ModeWrite); len(got) != 0 {
		t.Fatalf("a two-word cloze with its English: %+v", got)
	}

	bare := clozeUnit(Activity{Type: ActivityCloze, Text: et, Target: et, Translation: "specific-object marker"})
	got := rulesIn(clozeIssues(bare, ModeWrite), RuleUnitClozeContext)
	if len(got) != 1 || got[0].Severity != SeverityBlock || got[0].Pointer != "/lessons/0/activities/0/text" {
		t.Fatalf("a one-word cloze: %+v", got)
	}

	silent := clozeUnit(Activity{Type: ActivityCloze, Text: ani + " " + gar + ".", Target: gar})
	got = rulesIn(clozeIssues(silent, ModeWrite), RuleUnitClozeTranslation)
	if len(got) != 1 || got[0].Severity != SeverityBlock || got[0].Pointer != "/lessons/0/activities/0/translation" {
		t.Fatalf("a cloze with no English: %+v", got)
	}

	for _, is := range clozeIssues(clozeUnit(Activity{Type: ActivityCloze, Text: et}), ModePublished) {
		if is.Severity != SeverityWarn {
			t.Fatalf("the current published revision blocks: %+v", is)
		}
	}

	other := clozeUnit(Activity{Type: ActivityProduce, Text: et})
	if got := clozeIssues(other, ModeWrite); len(got) != 0 {
		t.Fatalf("a one-word produce is not a cloze: %+v", got)
	}
}

func TestLessonRepeatIsFlagged(t *testing.T) {
	acts := []Activity{
		{Type: ActivityShadow, Text: "גָּר"},
		{Type: ActivityCloze, Text: "אֲנִי גָּר.", Target: "גָּר", Translation: "I live."},
	}
	doc := &Unit{Lessons: []Lesson{
		{Kind: KindLesson, Activities: acts},
		{Kind: KindLesson, Activities: append([]Activity(nil), acts...)},
		{Kind: KindLesson, Activities: acts[:1]},
		{Kind: KindCheckpoint, Activities: acts},
	}}
	got := lessonRepeatIssues(doc, ModeWrite)
	if len(got) != 1 || got[0].Rule != RuleUnitLessonRepeat || got[0].Severity != SeverityBlock || got[0].Pointer != "/lessons/1" {
		t.Fatalf("repeat: %+v", got)
	}
	if got := lessonRepeatIssues(doc, ModePublished); len(got) != 1 || got[0].Severity != SeverityWarn {
		t.Fatalf("published repeat: %+v", got)
	}
}

func TestClozeRulesAreCataloged(t *testing.T) {
	cat := map[string]Rule{}
	for _, r := range Rules {
		cat[r.ID] = r
	}
	for _, id := range []string{RuleUnitClozeContext, RuleUnitClozeTranslation, RuleUnitLessonRepeat} {
		if r, ok := cat[id]; !ok || r.Severity != SeverityBlock || r.Description == "" {
			t.Fatalf("catalog row for %s: %+v", id, r)
		}
	}
}
