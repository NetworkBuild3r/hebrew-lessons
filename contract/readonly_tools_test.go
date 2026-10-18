package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/golden"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/normalize"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/seed"
)

// callCase is one tools/call against the seeded server.
// kind ok is a successful result. kind domain must be an isError tool result
// (not a JSON-RPC error) whose text is frozen after masking. kind validation
// is recorded as category plus field, with the message replaced by <validation>.
type callCase struct {
	tool string
	name string
	kind string
	args map[string]any
}

func TestReadOnlyTools(t *testing.T) {
	if os.Getenv("CONTRACT_ADMIN_USER") != "contract-admin" {
		t.Fatal("read-only goldens are pinned to CONTRACT_ADMIN_USER=contract-admin")
	}
	if name := os.Getenv("ADMIN_DISPLAY_NAME"); name != "" && name != "Contract Admin" {
		t.Fatalf("read-only goldens are pinned to ADMIN_DISPLAY_NAME=Contract Admin, got %q", name)
	}
	if seed.Current.PlanID == "" || seed.Current.Revision < 1 || seed.Current.StoryPath == "" || seed.Current.BlockPath == "" {
		t.Fatalf("seed state %#v", seed.Current)
	}
	for _, tc := range readOnlyCases() {
		tc := tc
		t.Run(tc.tool+"/"+tc.name, func(t *testing.T) {
			recordCall(t, tc)
		})
	}
}

func readOnlyCases() []callCase {
	plan := seed.Current.PlanID
	draft := seed.Current.DraftPlanID
	rev := seed.Current.Revision
	story := seed.Current.StoryPath
	block := seed.Current.BlockPath
	return []callCase{
		{tool: "list_lessons", name: "happy", kind: "ok"},
		{tool: "get_lesson", name: "happy", kind: "ok", args: map[string]any{"slug": "my-morning"}},
		{tool: "get_lesson", name: "bad_slug", kind: "domain", args: map[string]any{"slug": "Not A Slug"}},
		{tool: "get_lesson", name: "missing", kind: "domain", args: map[string]any{"slug": "no-such-lesson"}},
		{tool: "get_lesson", name: "missing_slug", kind: "validation"},
		{tool: "list_words", name: "happy", kind: "ok"},
		{tool: "list_words", name: "one_lesson", kind: "ok", args: map[string]any{"slug": "my-morning"}},
		{tool: "list_words", name: "unknown_slug", kind: "ok", args: map[string]any{"slug": "no-such-lesson"}},
		{tool: "list_activity_types", name: "happy", kind: "ok"},
		{tool: "list_users", name: "happy", kind: "ok"},
		{tool: "read_part", name: "plan", kind: "ok", args: map[string]any{"id": plan, "path": ""}},
		{tool: "read_part", name: "block", kind: "ok", args: map[string]any{"id": plan, "path": block}},
		{tool: "read_part", name: "unknown_id", kind: "domain", args: map[string]any{"id": "missing-plan", "path": ""}},
		{tool: "read_part", name: "missing_path", kind: "domain", args: map[string]any{"id": plan, "path": "units/does-not-exist"}},
		{tool: "suggest_glosses", name: "story", kind: "ok", args: map[string]any{"id": plan, "path": story}},
		{tool: "suggest_glosses", name: "not_a_story", kind: "domain", args: map[string]any{"id": plan, "path": block}},
		{tool: "suggest_glosses", name: "unknown_id", kind: "domain", args: map[string]any{"id": "missing-plan", "path": story}},
		// plan_image refuses with missing_key when OPENAI_API_KEY is unset.
		// The dry-run success shape is not reachable without that key.
		{tool: "plan_image", name: "missing_key", kind: "domain", args: map[string]any{"prompt": "a red apple on a table", "count": 2}},
		{tool: "plan_image", name: "missing_prompt", kind: "validation", args: map[string]any{"count": 1}},
		{tool: "list_plans", name: "happy", kind: "ok"},
		{tool: "list_plans", name: "limit_zero", kind: "validation", args: map[string]any{"limit": 0}},
		{tool: "list_plan_revisions", name: "happy", kind: "ok", args: map[string]any{"id": plan}},
		{tool: "list_plan_revisions", name: "unknown_id", kind: "domain", args: map[string]any{"id": "missing-plan"}},
		{tool: "get_plan_revision", name: "happy", kind: "ok", args: map[string]any{"id": plan, "revision": rev}},
		{tool: "get_plan_revision", name: "missing_revision", kind: "domain", args: map[string]any{"id": plan, "revision": 999999999}},
		{tool: "get_plan_revision", name: "unknown_id", kind: "domain", args: map[string]any{"id": "missing-plan", "revision": 1}},
		{tool: "diff_plan", name: "published_to_working", kind: "ok", args: map[string]any{"id": plan, "from": "published", "to": "working"}},
		{tool: "diff_plan", name: "unknown_id", kind: "domain", args: map[string]any{"id": "missing-plan", "from": "published", "to": "working"}},
		{tool: "diff_plan", name: "no_snapshot", kind: "domain", args: map[string]any{"id": draft, "from": "published", "to": "working"}},
		{tool: "diff_plan", name: "missing_from", kind: "validation", args: map[string]any{"id": plan, "to": "working"}},
		{tool: "list_plan_issues", name: "happy", kind: "ok", args: map[string]any{"id": plan}},
		{tool: "list_plan_issues", name: "unknown_id", kind: "domain", args: map[string]any{"id": "missing-plan"}},
		{tool: "get_learner_profile", name: "happy", kind: "ok", args: map[string]any{"username": seed.LearnerUsername}},
		{tool: "get_learner_profile", name: "planning_off", kind: "domain", args: map[string]any{"username": seed.LearnerOffUsername}},
		{tool: "get_learner_profile", name: "unknown_user", kind: "domain", args: map[string]any{"username": "nobody"}},
		{tool: "get_learner_profile", name: "missing_username", kind: "validation"},
		{tool: "check_text_coverage", name: "happy", kind: "ok", args: map[string]any{"username": seed.LearnerUsername, "text": "אני מדבר עברית"}},
		{tool: "check_text_coverage", name: "planning_off", kind: "domain", args: map[string]any{"username": seed.LearnerOffUsername, "text": "שלום"}},
		{tool: "check_text_coverage", name: "unknown_user", kind: "domain", args: map[string]any{"username": "nobody", "text": "שלום"}},
		{tool: "check_text_coverage", name: "invalid_text", kind: "domain", args: map[string]any{"username": seed.LearnerUsername, "text": strings.Repeat("א", 4001)}},
		{tool: "list_grammar_concepts", name: "happy", kind: "ok"},
		{tool: "list_lexicon", name: "search", kind: "ok", args: map[string]any{"q": "speak", "limit": 5}},
		{tool: "list_lexicon", name: "invalid_pos", kind: "domain", args: map[string]any{"pos": "not-a-pos"}},
		{tool: "list_lexicon", name: "cursor", kind: "ok", args: map[string]any{"cursor": "לדבר", "limit": 5}},
		{tool: "get_lexicon_entry", name: "happy", kind: "ok", args: map[string]any{"key": "לדבר"}},
		{tool: "get_lexicon_entry", name: "missing", kind: "domain", args: map[string]any{"key": "no-such-lexicon-key"}},
	}
}

func recordCall(t *testing.T, tc callCase) {
	t.Helper()
	res, err := mcpClient(t).CallTool(testCtx(t), 1, tc.tool, tc.args)
	if err != nil {
		t.Fatal(err)
	}
	assertSSE(t, res)
	rel := filepath.Join("golden", "mcp", "calls", tc.tool, tc.name+".json")
	msg := res.Message()
	if tc.kind == "validation" {
		rec := normalize.Classify(res.Status, "", msg)
		if rec.Category != normalize.CategoryValidation {
			t.Fatalf("category %s message %q body %s", rec.Category, rec.Message, truncateBody(res.Body))
		}
		compareValidation(t, rel, rec)
		return
	}
	if msg != nil {
		if _, bad := msg["error"]; bad && msg["result"] == nil {
			t.Fatalf("%s %s returned a JSON-RPC error: %s", tc.tool, tc.name, truncateBody(res.Body))
		}
	}
	body, ok := mcp.DecodeTool(res)
	if !ok {
		t.Fatalf("no tool result: %s", truncateBody(res.Body))
	}
	switch tc.kind {
	case "ok":
		if body.IsError {
			t.Fatalf("isError: %s", truncateBody([]byte(body.Text)))
		}
	case "domain":
		if !body.IsError {
			t.Fatalf("expected isError: %s", truncateBody([]byte(body.Text)))
		}
		rec := normalize.Classify(res.Status, "", msg)
		if rec.Category != normalize.CategoryDomain {
			t.Fatalf("category %s message %q", rec.Category, rec.Message)
		}
	default:
		t.Fatalf("kind %s", tc.kind)
	}
	golden.Compare(t, rel, map[string]any{
		"isError": body.IsError,
		"content": maskContent(body.Content),
	})
}

func maskContent(v any) any {
	if s, ok := v.(string); ok {
		return normalize.MaskString(s)
	}
	return normalize.Mask(v)
}

func compareValidation(t *testing.T, rel string, rec normalize.Record) {
	t.Helper()
	full := map[string]any{
		"category":      rec.Category,
		"field":         rec.Field,
		"message":       rec.Message,
		"transportForm": rec.TransportForm,
	}
	if golden.Update() {
		golden.Compare(t, rel, full)
		return
	}
	path := filepath.Join(golden.Root(t), rel)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (set UPDATE_GOLDEN=1 to create it)", rel, err)
	}
	var want map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	delete(want, "transportForm")
	got := map[string]any{
		"category": rec.Category,
		"field":    rec.Field,
		"message":  rec.Message,
	}
	gotBody, err := golden.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	wantBody, err := golden.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBody) != string(wantBody) {
		t.Fatalf("golden mismatch %s\n got %s\nwant %s", rel, gotBody, wantBody)
	}
}

func truncateBody(b []byte) string {
	s := string(b)
	if len(s) > 1500 {
		return s[:1500] + "…"
	}
	return s
}
