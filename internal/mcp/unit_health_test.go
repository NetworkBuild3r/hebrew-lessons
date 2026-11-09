package mcp

// INIT-037/SPEC-009: get_context blocks, read-only sweep allowlist, fixture sweep.

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

func TestUnitHealthAllowlistIsReadOnly(t *testing.T) {
	allowed := map[string]bool{}
	for _, name := range UnitHealthAllowlist {
		allowed[name] = true
		if strings.HasPrefix(name, "upsert_") {
			t.Fatalf("UnitHealthAllowlist includes write tool %q", name)
		}
	}
	for _, name := range unitHealthWriteTools {
		if allowed[name] {
			t.Fatalf("UnitHealthAllowlist includes write tool %q", name)
		}
	}
}

func TestGetContextOutlineBlocks(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	passDoc := helloDoc()
	pass, err := mem.PutUnit(context.Background(), nil, &passDoc, ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	mem.SetStatus(pass.UnitID, "published")

	clozeDoc := helloDoc()
	clozeDoc.Title = "One-word cloze"
	clozeDoc.Position = 1
	clozeDoc.Lessons = []units.Lesson{{
		Kind: units.KindLesson,
		Activities: []units.Activity{
			{Type: units.ActivityListenPick, Text: hello},
			{Type: units.ActivityCloze, Text: hello, Target: hello, Translation: "hello"},
		},
	}}
	cloze, err := mem.PutUnit(context.Background(), nil, &clozeDoc, ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	mem.SetStatus(cloze.UnitID, "published")

	session := connectMCP(t, Handler(Options{
		AuthToken: testToken, Course: mem, Lexicon: mem, Lookup: mem, Units: mem,
	}), testToken)

	id := int64(1)
	got := call[getContextOutput](t, session, "get_context", getContextInput{Position: 0, LearnerID: &id})
	if len(got.Outline) != 2 {
		t.Fatalf("outline %+v", got.Outline)
	}
	byID := map[int64]UnitOutline{}
	for _, row := range got.Outline {
		byID[row.ID] = row
	}
	if byID[pass.UnitID].Blocks != 0 {
		t.Fatalf("passing unit blocks=%d want 0; row=%+v", byID[pass.UnitID].Blocks, byID[pass.UnitID])
	}
	if byID[cloze.UnitID].Blocks < 1 {
		t.Fatalf("one-word cloze blocks=%d want >= 1", byID[cloze.UnitID].Blocks)
	}
}

func TestGetContextBlocksHideOtherLearnerUnits(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	doc := helloDoc()
	shared, err := mem.PutUnit(context.Background(), nil, &doc, ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	mem.SetStatus(shared.UnitID, "published")

	ownDoc := helloDoc()
	ownDoc.Title = "Mine"
	own, err := mem.PutUnit(context.Background(), nil, &ownDoc, "learner:7")
	if err != nil {
		t.Fatal(err)
	}
	mem.SetStatus(own.UnitID, "published")

	otherDoc := helloDoc()
	otherDoc.Title = "Theirs"
	otherDoc.Lessons[0].Activities = []units.Activity{
		{Type: units.ActivityListenPick, Text: hello},
		{Type: units.ActivityCloze, Text: hello, Target: hello, Translation: "hello"},
	}
	other, err := mem.PutUnit(context.Background(), nil, &otherDoc, "learner:9")
	if err != nil {
		t.Fatal(err)
	}
	mem.SetStatus(other.UnitID, "published")

	h := Handler(Options{
		Validator: roleTokens{},
		Course:    mem,
		Lexicon:   mem,
		Lookup:    mem,
		Units:     mem,
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	res := postTool(t, srv.URL, "learner", "get_context", getContextInput{Position: 0}, nil)
	body := readBody(t, res)
	if strings.Contains(body, "Theirs") || strings.Contains(body, `"id":`+strconv.FormatInt(other.UnitID, 10)) {
		t.Fatalf("learner saw another learner's unit: %s", body)
	}
	session := connectMCP(t, h, "learner")
	out := call[getContextOutput](t, session, "get_context", getContextInput{Position: 0})
	seen := map[int64]bool{}
	for _, row := range out.Outline {
		seen[row.ID] = true
		if row.ID == other.UnitID || row.Scope == "learner:9" {
			t.Fatalf("learner outline leaked other unit %+v", row)
		}
	}
	if !seen[shared.UnitID] || !seen[own.UnitID] {
		t.Fatalf("want shared and own, got %+v", out.Outline)
	}
}

func TestSweepFixtureListsCopyAndOneWordClozeOnly(t *testing.T) {
	mem := healthFixtureStore(t)
	learnerID := int64(7)
	rows, err := SweepPublished(context.Background(), mem, mem, mem, &learnerID)
	if err != nil {
		t.Fatal(err)
	}
	bad := UnhealthyPublished(rows)
	if len(bad) != 2 {
		t.Fatalf("unhealthy=%d all=%+v", len(bad), annotateHealth(t, mem, rows))
	}
	titles := map[int64]string{}
	for _, row := range rows {
		snap, err := mem.LoadUnit(context.Background(), row.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		titles[row.ID] = snap.Doc.Title
	}
	got := map[string]HealthRow{}
	for _, row := range bad {
		got[titles[row.ID]] = row
	}
	if _, ok := got["Copied"]; !ok {
		t.Fatalf("missing Copied in %+v", got)
	}
	if _, ok := got["One-word cloze"]; !ok {
		t.Fatalf("missing One-word cloze in %+v", got)
	}
	for title := range got {
		if title != "Copied" && title != "One-word cloze" {
			t.Fatalf("unexpected unhealthy %q in %+v", title, got)
		}
	}
}

func annotateHealth(t *testing.T, mem *Memory, rows []HealthRow) []string {
	t.Helper()
	var out []string
	for _, row := range rows {
		snap, err := mem.LoadUnit(context.Background(), row.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, snap.Doc.Title+":blocks="+strconv.Itoa(row.Blocks)+":rules="+strings.Join(row.Rules, ","))
	}
	return out
}

func healthFixtureStore(t *testing.T) *Memory {
	t.Helper()
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	mem.AddLearner(7)
	mem.SetAudio(hello)

	// Base: passes ModeWrite. Story of the new word so a later copy can trip story_repeat.
	base := units.Unit{
		Title: "Base", Situation: "s", Concept: "c",
		Position: 0,
		NewWords: []string{hello},
		Lessons: []units.Lesson{{
			Kind: units.KindLesson,
			Activities: []units.Activity{
				{Type: units.ActivityStory, Text: hello},
				{Type: units.ActivityShadow, Text: hello},
				{Type: units.ActivityListenPick, Text: hello},
				{Type: units.ActivityRecall, Text: hello},
			},
		}},
	}
	savedBase, err := mem.PutUnit(context.Background(), nil, &base, "learner:7")
	if err != nil {
		t.Fatal(err)
	}
	mem.SetStatus(savedBase.UnitID, "published")

	// Copied: same activity sequence and same first story as Base.
	copied := base
	copied.Title = "Copied"
	copied.Position = 1
	savedCopy, err := mem.PutUnit(context.Background(), nil, &copied, "learner:7")
	if err != nil {
		t.Fatal(err)
	}
	mem.SetStatus(savedCopy.UnitID, "published")

	cloze := units.Unit{
		Title: "One-word cloze", Situation: "s", Concept: "c",
		Position: 2,
		NewWords: []string{hello},
		Lessons: []units.Lesson{{
			Kind: units.KindLesson,
			Activities: []units.Activity{
				{Type: units.ActivityListenPick, Text: hello},
				{Type: units.ActivityCloze, Text: hello, Target: hello, Translation: "hello"},
			},
		}},
	}
	savedCloze, err := mem.PutUnit(context.Background(), nil, &cloze, "learner:7")
	if err != nil {
		t.Fatal(err)
	}
	mem.SetStatus(savedCloze.UnitID, "published")

	clean := helloDoc()
	clean.Title = "Clean"
	clean.Position = 3
	savedClean, err := mem.PutUnit(context.Background(), nil, &clean, ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	mem.SetStatus(savedClean.UnitID, "published")

	return mem
}
