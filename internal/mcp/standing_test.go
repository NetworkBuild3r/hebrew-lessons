package mcp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

// INIT-033/SPEC-008: standing on get_learner_model / get_context, direction labels,
// digest history, size budget, and description thresholds.

func TestLearnerModelStandingEqualsSeed(t *testing.T) {
	e := newLT(t)
	acc := 0.72
	want := StandingView{
		AsOf:          time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC),
		KnownAnySkill: 7,
		Directions: []DirectionStandingView{{
			Skill: "recall", Direction: learner.DirectionLabel("recall"),
			Known: 1, Answers14: 68, Right14: 49, Accuracy14: &acc,
			Verdict: "needs_help", Reason: "72% right over 68 answers in 14 days",
			Help: []WordNoteView{{
				LexemeID: 5, Hebrew: "שָׁלוֹם", English: "hello",
				Misses14: 4, Answers14: 6, Cause: "production",
			}},
			Strong:  []WordNoteView{},
			Askable: true,
		}},
	}
	e.mem.SetLearnerStanding(learnerA, want)
	e.mem.SetLearnerModel(learnerA, []ModelWord{mw(hello, "hello", "known")}, &ModelSummary{
		Words: map[string]int{"known": 1}, KnownAnySkill: 7,
	})

	raw := e.ok("learner-a", "get_learner_model", map[string]any{})
	var out learnerModelOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	if out.Summary.KnownAnySkill != want.KnownAnySkill || out.Standing.KnownAnySkill != want.KnownAnySkill {
		t.Fatalf("summary=%d standing=%d want %d", out.Summary.KnownAnySkill, out.Standing.KnownAnySkill, want.KnownAnySkill)
	}
	if len(out.Standing.Directions) != 1 {
		t.Fatalf("directions %+v", out.Standing.Directions)
	}
	got := out.Standing.Directions[0]
	if got.Skill != want.Directions[0].Skill || got.Direction != want.Directions[0].Direction ||
		got.Verdict != want.Directions[0].Verdict || got.Reason != want.Directions[0].Reason {
		t.Fatalf("got %+v want %+v", got, want.Directions[0])
	}
	if len(got.Help) != 1 || got.Help[0].LexemeID != 5 || got.Help[0].Cause != "production" ||
		got.Help[0].Hebrew != "שָׁלוֹם" || got.Help[0].Misses14 != 4 {
		t.Fatalf("help %+v", got.Help)
	}
}

func TestLearnerModelDescriptionMentionsThresholdsNotRouting(t *testing.T) {
	e := newLT(t)
	tools, err := e.session("admin").ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var desc string
	for _, tool := range tools.Tools {
		if tool.Name == "get_learner_model" {
			desc = tool.Description
		}
	}
	if desc == "" {
		t.Fatal("get_learner_model missing")
	}
	for _, needle := range []string{"75%", "85%", "needs_help", "strong", "14"} {
		if !strings.Contains(desc, needle) {
			t.Errorf("description missing %q", needle)
		}
	}
	for _, banned := range []string{"if the user says", "when they ask", "match the message", "route to"} {
		if strings.Contains(strings.ToLower(desc), banned) {
			t.Errorf("description routes from user text: %q", banned)
		}
	}
}

func TestContextStandingBudgetAndNoSecrets(t *testing.T) {
	e := newLT(t)
	acc := 0.9
	standing := StandingView{
		KnownAnySkill: 4,
		Directions: []DirectionStandingView{
			{Skill: "read", Direction: learner.DirectionLabel("read"), Verdict: "strong", Reason: "ok",
				Help: []WordNoteView{
					{LexemeID: 1, Cause: "decoding"},
					{LexemeID: 2, Cause: "production"},
					{LexemeID: 3, Cause: "spelling"},
					{LexemeID: 4, Cause: "confusion"},
				}, Askable: true, Accuracy14: &acc, Answers14: 40, Right14: 36},
			{Skill: "listen", Direction: learner.DirectionLabel("listen"), Verdict: "not_askable",
				Reason: "No audio yet — practiced in lessons only", Askable: false},
		},
	}
	e.mem.SetLearnerStanding(learnerA, standing)
	e.mem.SetLearnerModel(learnerA, []ModelWord{mw(hello, "hello", "known")}, &ModelSummary{
		Words: map[string]int{"known": 1}, KnownAnySkill: 4,
		Standing: compactStandingFromView(standing),
	})

	raw := e.ok("learner-a", "get_context", map[string]any{"position": 0})
	if len(raw) > contextJSONBudget {
		t.Fatalf("get_context JSON is %d bytes, want <= %d", len(raw), contextJSONBudget)
	}
	lower := strings.ToLower(raw)
	for _, leak := range []string{"@example", "mailto:", `"response"`, "typed response"} {
		if strings.Contains(lower, leak) {
			t.Fatalf("get_context leaked %q", leak)
		}
	}
	var out getContextOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	if out.Model == nil || out.Model.Standing == nil {
		t.Fatal("model.standing missing")
	}
	var read *CompactDirection
	for i := range out.Model.Standing.Directions {
		if out.Model.Standing.Directions[i].Skill == "read" {
			read = &out.Model.Standing.Directions[i]
		}
	}
	if read == nil || read.Verdict != "strong" || len(read.Help) != CompactStandingHelpCap {
		t.Fatalf("compact read %+v", read)
	}
}

func TestProgressAndDigestCarryDirection(t *testing.T) {
	e := newLT(t)
	e.mem.SetProgress(learnerA, Progress{
		Snapshots: []SnapshotDay{{
			Day: "2026-10-10",
			Skills: []SkillDay{{
				Skill: "read", Direction: learner.DirectionLabel("read"),
				Counts: map[string]int{"known": 2},
			}},
		}},
	})
	raw := e.ok("learner-a", "get_progress", map[string]any{})
	if !strings.Contains(raw, learner.DirectionLabel("read")) {
		t.Fatalf("get_progress missing direction label:\n%s", raw)
	}
}
