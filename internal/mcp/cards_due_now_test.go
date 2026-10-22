package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/sessionlimits"
)

// TestGetContextContractAddsDueFields locks the additive get_context contract.
// cardsDueNow and mixTarget.minNewPerSession are new. The fields that were
// already there keep their names and JSON types, including model.dueReviews,
// placement.cardsRated, and placement.stopReason.
func TestGetContextContractAddsDueFields(t *testing.T) {
	mem := NewMemory()
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken, Course: mem, Lexicon: mem, Lookup: mem, Units: mem,
	}), testToken)
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	tool := tools.Tools[indexOf(tools, "get_context")]
	for _, phrase := range []string{
		"model.dueReviews counts past reviews that waited at least a day, in the last 30 days",
		"It is not the number of cards due now",
		"cardsDueNow",
		"an empty stopReason can appear on older v3 placement rows",
		"minNewPerSession",
	} {
		if !strings.Contains(tool.Description, phrase) {
			t.Fatalf("description missing %q", phrase)
		}
	}

	outRaw, err := json.Marshal(tool.OutputSchema)
	if err != nil {
		t.Fatal(err)
	}
	root := schemaObject(t, outRaw)
	wantRequired := []string{
		"outline", "knownLexemes", "budgets", "placement", "struggles",
		"completedSharedUnits", "cards", "cardsTruncated", "coachNotes",
		"model", "recentAccuracy", "mixTarget", "cardsDueNow",
	}
	if !sameStrings(root.required, wantRequired) {
		t.Fatalf("required %v", root.required)
	}
	for _, name := range []string{"outline", "knownLexemes", "budgets", "placement", "struggles", "completedSharedUnits", "cards", "cardsTruncated", "cardsHint", "coachNotes", "model", "recentAccuracy", "mixTarget"} {
		if _, ok := root.props[name]; !ok {
			t.Fatalf("missing old field %s", name)
		}
	}

	due := schemaObject(t, root.props["cardsDueNow"])
	if schemaType(t, root.props["cardsDueNow"]) != `["null","object"]` {
		t.Fatalf("cardsDueNow type %s", schemaType(t, root.props["cardsDueNow"]))
	}
	for _, name := range []string{"new", "learning", "review", "relearning", "total"} {
		if schemaType(t, due.props[name]) != `"integer"` {
			t.Fatalf("cardsDueNow.%s type %s", name, schemaType(t, due.props[name]))
		}
	}
	if !sameStrings(due.required, []string{"new", "learning", "review", "relearning", "total"}) {
		t.Fatalf("cardsDueNow required %v", due.required)
	}

	mix := schemaObject(t, root.props["mixTarget"])
	oldMix := map[string]string{
		"maxNewWordsPerUnit":   `"integer"`,
		"minNewWordsPerLesson": `"integer"`,
		"maxNewWordsPerLesson": `"integer"`,
		"targetSuccess":        `"number"`,
		"maxNewPerSession":     `"integer"`,
		"maxNewShare":          `"number"`,
	}
	for name, want := range oldMix {
		if schemaType(t, mix.props[name]) != want {
			t.Fatalf("mixTarget.%s type %s, want %s", name, schemaType(t, mix.props[name]), want)
		}
	}
	if schemaType(t, mix.props["minNewPerSession"]) != `"integer"` {
		t.Fatalf("minNewPerSession type %s", schemaType(t, mix.props["minNewPerSession"]))
	}

	model := schemaObject(t, root.props["model"])
	if _, sneaked := model.props["cardsDueNow"]; sneaked {
		t.Fatal("cardsDueNow is a field of model")
	}
	if schemaType(t, model.props["dueReviews"]) != `"integer"` {
		t.Fatalf("dueReviews type %s", schemaType(t, model.props["dueReviews"]))
	}
	if schemaType(t, model.props["retention"]) != `["null","number"]` {
		t.Fatalf("retention type %s", schemaType(t, model.props["retention"]))
	}
	if _, ok := model.props["words"]; !ok {
		t.Fatal("model.words missing")
	}
	if _, ok := model.props["skills"]; !ok {
		t.Fatal("model.skills missing")
	}
	skill := schemaObject(t, schemaObject(t, model.props["skills"]).items)
	if schemaType(t, skill.props["dueReviews"]) != `"integer"` {
		t.Fatalf("skill dueReviews type %s", schemaType(t, skill.props["dueReviews"]))
	}

	place := schemaObject(t, root.props["placement"])
	if schemaType(t, place.props["cardsRated"]) != `"integer"` {
		t.Fatalf("cardsRated type %s", schemaType(t, place.props["cardsRated"]))
	}
	if schemaType(t, place.props["stopReason"]) != `"string"` {
		t.Fatalf("stopReason type %s", schemaType(t, place.props["stopReason"]))
	}
	for _, name := range []string{"taken", "takenAt", "finished", "sittings", "inProgress", "level", "cardsRated", "stopReason", "bands", "estimatedKnownWords", "suggestedStartPosition", "knownLexemes", "learningLexemes"} {
		if _, ok := place.props[name]; !ok {
			t.Fatalf("placement missing %s", name)
		}
	}

	inRaw, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	in := schemaObject(t, inRaw)
	if !sameStrings(keysOf(in.props), []string{"learnerId", "position", "topN"}) {
		t.Fatalf("input fields %v", keysOf(in.props))
	}

	sharedRaw := callRaw(t, session, "get_context", getContextInput{Position: 0})
	if !strings.Contains(sharedRaw, `"cardsDueNow":null`) || !strings.Contains(sharedRaw, `"model":null`) || !strings.Contains(sharedRaw, `"recentAccuracy":null`) {
		t.Fatalf("omitted learner %s", sharedRaw)
	}
	shared := call[getContextOutput](t, session, "get_context", getContextInput{Position: 0})
	if shared.CardsDueNow != nil || shared.Model != nil || shared.MixTarget.MinNewPerSession != sessionlimits.MinNewPerSession || shared.MixTarget.MaxNewPerSession != sessionlimits.MaxNewPerSession {
		t.Fatalf("shared mix %+v due %+v", shared.MixTarget, shared.CardsDueNow)
	}

	id := int64(7)
	with := call[getContextOutput](t, session, "get_context", getContextInput{LearnerID: &id})
	if with.CardsDueNow == nil || with.Model == nil || with.Model.DueReviews != 0 || with.CardsDueNow.Total != 0 {
		t.Fatalf("learner due %+v model %+v", with.CardsDueNow, with.Model)
	}
	if with.MixTarget != currentMixTarget() {
		t.Fatalf("mix %+v", with.MixTarget)
	}
}

func TestCardsDueNowFromStatesCountsEachState(t *testing.T) {
	got := CardsDueNowFromStates([]string{"new", "learning", "review", "relearning", "other", ""})
	want := CardsDueNow{New: 1, Learning: 1, Review: 1, Relearning: 1, Total: 6}
	if got != want {
		t.Fatalf("counts %+v", got)
	}
	if empty := CardsDueNowFromStates(nil); empty != (CardsDueNow{}) {
		t.Fatalf("empty %+v", empty)
	}
}

func TestCountsTowardDueNowLegsAndBoundary(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		user   int64
		due    time.Time
		userID int64
		want   bool
	}{
		{name: "other user at now", user: 2, due: now, userID: 1, want: false},
		{name: "same user one nanosecond before", user: 1, due: now.Add(-time.Nanosecond), userID: 1, want: true},
		{name: "same user at now", user: 1, due: now, userID: 1, want: true},
		{name: "same user one nanosecond after", user: 1, due: now.Add(time.Nanosecond), userID: 1, want: false},
		{name: "other user one nanosecond before", user: 2, due: now.Add(-time.Nanosecond), userID: 1, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := countsTowardDueNow(tc.user, tc.due, tc.userID, now); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestMemoryCardsDueNowAppliesPredicate(t *testing.T) {
	mem := NewMemory()
	now := time.Now().UTC()
	mem.AddCard(1, LearnerCard{State: "new", Due: now.Add(-time.Hour)})
	mem.AddCard(1, LearnerCard{State: "learning", Due: now.Add(-time.Second)})
	mem.AddCard(1, LearnerCard{State: "review", Due: now.Add(time.Hour)})
	mem.AddCard(1, LearnerCard{State: "relearning", Due: now})
	mem.AddCard(1, LearnerCard{State: "other", Due: now.Add(-time.Minute)})
	mem.AddCard(2, LearnerCard{State: "new", Due: now.Add(-time.Hour)})

	got, err := mem.CardsDueNow(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	want := CardsDueNow{New: 1, Learning: 1, Review: 0, Relearning: 1, Total: 4}
	if got != want {
		t.Fatalf("due %+v", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := mem.CardsDueNow(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled %v", err)
	}
}

type dueErrorCourse struct {
	*Memory
}

func (dueErrorCourse) CardsDueNow(context.Context, int64) (CardsDueNow, error) {
	return CardsDueNow{}, errors.New("due failed")
}

func TestGetContextReportsCardsDueNowStoreError(t *testing.T) {
	mem := NewMemory()
	session := connectMCP(t, Handler(Options{
		AuthToken: testToken, Course: dueErrorCourse{Memory: mem}, Lexicon: mem, Lookup: mem, Units: mem,
	}), testToken)
	id := int64(7)
	msg := callErr(t, session, "get_context", getContextInput{LearnerID: &id})
	if !strings.Contains(msg, "The cards due now could not be read.") {
		t.Fatalf("error %s", msg)
	}
}

func TestMinNewPerSessionMatchesSessionLimits(t *testing.T) {
	got := currentMixTarget()
	if got.MinNewPerSession != sessionlimits.MinNewPerSession {
		t.Fatalf("minNewPerSession %d sessionlimits %d", got.MinNewPerSession, sessionlimits.MinNewPerSession)
	}
}

type schemaNode struct {
	props    map[string]json.RawMessage
	required []string
	items    json.RawMessage
}

func schemaObject(t *testing.T, raw json.RawMessage) schemaNode {
	t.Helper()
	var node struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
		Items      json.RawMessage            `json:"items"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatal(err)
	}
	return schemaNode{props: node.Properties, required: node.Required, items: node.Items}
}

func schemaType(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	if len(raw) == 0 {
		t.Fatal("missing schema node")
	}
	var node struct {
		Type json.RawMessage `json:"type"`
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		t.Fatal(err)
	}
	return string(node.Type)
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]int{}
	for _, s := range got {
		seen[s]++
	}
	for _, s := range want {
		seen[s]--
		if seen[s] < 0 {
			return false
		}
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

func keysOf(props map[string]json.RawMessage) []string {
	out := make([]string, 0, len(props))
	for name := range props {
		out = append(out, name)
	}
	return out
}
