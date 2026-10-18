// Package seed builds the contract database through HTTP and MCP only.
// The same sequence has to work against the Go server later, so it does not
// import Node or open the database except to snapshot the file the server wrote.
package seed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/httpx"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/contract/internal/sqlite"
)

const (
	// AdminUsername is the bootstrap admin stored in the seeded fixture.
	AdminUsername = "contract-admin"
	// AdminPassword is the contract-only password for AdminUsername.
	// It is not a production secret. SignIn uses it; session rows are not.
	AdminPassword = "contract-admin-pass"
	// LearnerUsername is the account with AI planning on.
	LearnerUsername = "learner"
	// LearnerPassword is the contract-only password for LearnerUsername.
	LearnerPassword = "learner-pass-1"
	// LearnerOffUsername is active and has AI planning left off.
	LearnerOffUsername = "learner-off"
	// LearnerOffPassword is the contract-only password for LearnerOffUsername.
	LearnerOffPassword = "learner-off-pass-1"
	// PlanID is the published plan built from the five-lesson-types fixture.
	PlanID = "five-lesson-types-sample"
	// DraftPlanID is created and never published, so diff_plan can see no_snapshot.
	DraftPlanID = "draft-plan"
)

// State is the seeded ids the read-only cases call with.
type State struct {
	PlanID      string
	DraftPlanID string
	Revision    int
	StoryPath   string
	BlockPath   string
}

// Current is set by a successful Run.
var Current State

// Options points Run at the server run.sh booted.
type Options struct {
	WebURL     string
	MCPURL     string
	Token      string
	AdminUser  string
	AdminPass  string
	DataDir    string
	ModuleRoot string
	UpdateDB   bool
}

// UseFixture points Current at the committed database. It does not call HTTP
// or MCP and does not create a session. dataDir is the server's DATA_DIR,
// which already holds a copy of the fixture.
func UseFixture(dataDir, moduleRoot string) error {
	if moduleRoot == "" {
		root, err := moduleRootFromCaller()
		if err != nil {
			return err
		}
		moduleRoot = root
	}
	plan, err := loadJSON(filepath.Join(moduleRoot, "fixtures", "plans", "five-lesson-types.plan.json"))
	if err != nil {
		return err
	}
	units, _ := plan["units"].([]any)
	if len(units) != 1 {
		return fmt.Errorf("plan units %d", len(units))
	}
	unit, _ := units[0].(map[string]any)
	unitID, _ := unit["id"].(string)
	lessons, _ := unit["lessons"].([]any)
	story, block := pathsFromLessons(unitID, lessons)
	if story == "" || block == "" {
		return fmt.Errorf("fixture plan is missing a story or block path")
	}
	rev, err := sqlite.PlanRevision(filepath.Join(dataDir, "hebrew.sqlite"), PlanID)
	if err != nil {
		return fmt.Errorf("fixture plan revision: %w", err)
	}
	if rev < 1 {
		return fmt.Errorf("fixture plan revision %d", rev)
	}
	Current = State{
		PlanID:      PlanID,
		DraftPlanID: DraftPlanID,
		Revision:    rev,
		StoryPath:   story,
		BlockPath:   block,
	}
	fmt.Fprintf(os.Stderr, "seed: using fixture plan %s revision %d\n", PlanID, rev)
	return nil
}

// SignIn posts /api/session with a new cookie jar, then checks GET /api/me.
// Each call starts empty. It does not read the sessions table.
func SignIn(ctx context.Context, webURL, username, password string) (cookie string, err error) {
	web := httpx.New()
	web.Jar = map[string]string{}
	s := &session{
		web:    web,
		origin: strings.TrimRight(webURL, "/"),
		ctx:    ctx,
	}
	if err := s.login(username, password); err != nil {
		return "", err
	}
	cookie = web.Jar["hebrew_session"]
	if cookie == "" {
		cookie = web.Jar["__Host-hebrew_session"]
	}
	if cookie == "" {
		return "", fmt.Errorf("login %s did not set a session cookie", username)
	}
	res, err := web.Do(ctx, "GET", s.origin+"/api/me", map[string]string{
		"Accept": "application/json",
	}, nil)
	if err != nil {
		return "", err
	}
	if res.Status != 200 {
		return "", fmt.Errorf("me %s status %d: %s", username, res.Status, truncate(res.Body))
	}
	var me struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(res.Body, &me); err != nil {
		return "", err
	}
	if me.Username != username {
		return "", fmt.Errorf("me username %q, want %s", me.Username, username)
	}
	return cookie, nil
}

// Run logs in as the bootstrap admin, creates the two learners, sets lesson
// visibility, writes lexicon rows, adds library words, and publishes one plan.
// upsert_lesson is not called: the worksheet files are already the fixture,
// and a write would generate audio unless audio is false. No provider key is set.
func Run(ctx context.Context, opt Options) error {
	if opt.UpdateDB && (opt.AdminUser != AdminUsername || opt.AdminPass != AdminPassword) {
		return fmt.Errorf("refusing to snapshot: fixture admin must be %s with the documented contract password", AdminUsername)
	}
	if opt.ModuleRoot == "" {
		root, err := moduleRootFromCaller()
		if err != nil {
			return err
		}
		opt.ModuleRoot = root
	}
	plan, err := loadJSON(filepath.Join(opt.ModuleRoot, "fixtures", "plans", "five-lesson-types.plan.json"))
	if err != nil {
		return err
	}
	lexiconDoc, err := loadJSON(filepath.Join(opt.ModuleRoot, "fixtures", "plans", "lexicon-ledaber.json"))
	if err != nil {
		return err
	}
	web := httpx.New()
	web.Jar = map[string]string{}
	s := &session{
		web:    web,
		origin: strings.TrimRight(opt.WebURL, "/"),
		mcp:    mcp.New(opt.MCPURL, opt.Token),
		ctx:    ctx,
	}
	if err := s.login(opt.AdminUser, opt.AdminPass); err != nil {
		return err
	}
	learnerID, err := s.createUser(LearnerUsername, LearnerPassword, "Contract Learner")
	if err != nil {
		return err
	}
	if err := s.setAIPlanning(learnerID, true); err != nil {
		return err
	}
	if _, err := s.createUser(LearnerOffUsername, LearnerOffPassword, "Planning Off"); err != nil {
		return err
	}
	if err := s.toolOK("set_lesson_visibility", map[string]any{
		"slug":       "my-morning",
		"visibility": "everyone",
	}); err != nil {
		return err
	}
	if err := s.toolOK("set_lesson_visibility", map[string]any{
		"slug":       "shai-date-indirect-speech",
		"visibility": map[string]any{"users": []any{LearnerUsername}},
	}); err != nil {
		return err
	}
	// Lexicon rows go in before publish. The verb-workshop lesson does not
	// publish while those keys are missing. expectedRev 0 is for keys the
	// boot seed did not already insert.
	entries, _ := lexiconDoc["entries"].([]any)
	if err := s.toolOK("upsert_lexicon_entries", map[string]any{
		"entries":     entries,
		"expectedRev": 0,
	}); err != nil {
		return err
	}
	if err := s.toolOK("add_library_words", map[string]any{
		"username": LearnerUsername,
		"words":    []any{"לדבר", "מדבר"},
	}); err != nil {
		return err
	}
	revision, story, block, err := s.buildPlan(plan)
	if err != nil {
		return err
	}
	if err := s.toolOK("create_plan", map[string]any{
		"id":    DraftPlanID,
		"title": "Unpublished draft",
	}); err != nil {
		return err
	}
	Current = State{
		PlanID:      PlanID,
		DraftPlanID: DraftPlanID,
		Revision:    revision,
		StoryPath:   story,
		BlockPath:   block,
	}
	if opt.UpdateDB {
		src := filepath.Join(opt.DataDir, "hebrew.sqlite")
		dest := filepath.Join(opt.ModuleRoot, "fixtures", "db", "seeded.sqlite")
		if err := sqlite.Snapshot(src, dest); err != nil {
			return fmt.Errorf("snapshot seeded db: %w", err)
		}
		fmt.Fprintf(os.Stderr, "seed: wrote %s\n", dest)
	}
	fmt.Fprintf(os.Stderr, "seed: plan %s revision %d\n", PlanID, revision)
	return nil
}

type session struct {
	web    *httpx.Client
	origin string
	mcp    *mcp.Client
	ctx    context.Context
	nextID int
}

func (s *session) login(user, pass string) error {
	status, _, err := s.sendJSON("POST", "/api/session", map[string]any{
		"username": user,
		"password": pass,
	})
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("login status %d", status)
	}
	return nil
}

func (s *session) createUser(username, password, display string) (int, error) {
	status, body, err := s.sendJSON("POST", "/api/admin/users", map[string]any{
		"username":    username,
		"password":    password,
		"role":        "user",
		"displayName": display,
	})
	if err != nil {
		return 0, err
	}
	if status != 201 && status != 200 {
		return 0, fmt.Errorf("create user %s status %d", username, status)
	}
	obj, _ := body.(map[string]any)
	id, ok := asInt(obj["id"])
	if !ok || id < 1 {
		return 0, fmt.Errorf("create user %s id %#v", username, obj["id"])
	}
	return id, nil
}

func (s *session) setAIPlanning(id int, enabled bool) error {
	status, _, err := s.sendJSON("PATCH",
		fmt.Sprintf("/api/admin/users/%d/ai-planning", id),
		map[string]any{"enabled": enabled},
	)
	if err != nil {
		return err
	}
	if status != 200 {
		return fmt.Errorf("ai-planning status %d", status)
	}
	return nil
}

func (s *session) sendJSON(method, path string, payload any) (int, any, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}
	res, err := s.web.Do(s.ctx, method, s.origin+path, map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
		"Origin":       s.origin,
	}, raw)
	if err != nil {
		return 0, nil, err
	}
	if path == "/api/admin/users" || strings.Contains(path, "ai-planning") {
		if res.Status >= 400 {
			return res.Status, nil, fmt.Errorf("%s: %s", path, truncate(res.Body))
		}
	}
	if res.Status == 403 || res.Status >= 500 {
		return res.Status, nil, fmt.Errorf("%s: %s", path, truncate(res.Body))
	}
	if len(bytes.TrimSpace(res.Body)) == 0 {
		return res.Status, nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(res.Body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return res.Status, nil, fmt.Errorf("%s: %s", path, truncate(res.Body))
	}
	if res.Status >= 400 {
		return res.Status, v, fmt.Errorf("%s: %s", path, truncate(res.Body))
	}
	return res.Status, v, nil
}

func (s *session) toolOK(name string, args map[string]any) error {
	_, err := s.tool(name, args)
	return err
}

func (s *session) tool(name string, args map[string]any) (mcp.ToolBody, error) {
	s.nextID++
	res, err := s.mcp.CallTool(s.ctx, s.nextID, name, args)
	if err != nil {
		return mcp.ToolBody{}, fmt.Errorf("%s: %w", name, err)
	}
	if res.Status != 200 {
		return mcp.ToolBody{}, fmt.Errorf("%s: HTTP %d %s", name, res.Status, truncate(res.Body))
	}
	body, ok := mcp.DecodeTool(res)
	if !ok {
		return mcp.ToolBody{}, fmt.Errorf("%s: %s", name, truncate(res.Body))
	}
	if body.IsError {
		return body, fmt.Errorf("%s: %s", name, truncate([]byte(body.Text)))
	}
	return body, nil
}

func (s *session) buildPlan(plan map[string]any) (revision int, storyPath, blockPath string, err error) {
	units, _ := plan["units"].([]any)
	if len(units) != 1 {
		return 0, "", "", fmt.Errorf("plan units %d", len(units))
	}
	unit, _ := units[0].(map[string]any)
	unitID, _ := unit["id"].(string)
	lessons, _ := unit["lessons"].([]any)
	if unitID == "" || len(lessons) == 0 {
		return 0, "", "", fmt.Errorf("plan has no lessons")
	}
	first, _ := lessons[0].(map[string]any)
	lessonID, _ := first["id"].(string)
	title, _ := plan["title"].(string)
	recipe, _ := first["recipe"].(string)
	storyPath, blockPath = pathsFromLessons(unitID, lessons)
	body, err := s.tool("create_plan", map[string]any{
		"id":       PlanID,
		"title":    title,
		"recipe":   recipe,
		"unitId":   unitID,
		"lessonId": lessonID,
	})
	if err != nil {
		return 0, "", "", err
	}
	rev, err := revisionOf(body)
	if err != nil {
		return 0, "", "", fmt.Errorf("create_plan: %w", err)
	}
	lessonPath := "units/" + unitID + "/lessons/" + lessonID
	blocks, _ := first["blocks"].([]any)
	for i := len(blocks) - 1; i >= 0; i-- {
		body, err = s.tool("insert_block", map[string]any{
			"id":               PlanID,
			"path":             lessonPath,
			"index":            0,
			"block":            blocks[i],
			"expectedRevision": rev,
		})
		if err != nil {
			return 0, "", "", err
		}
		rev, err = revisionOf(body)
		if err != nil {
			return 0, "", "", err
		}
	}
	body, err = s.tool("remove_block", map[string]any{
		"id":               PlanID,
		"path":             lessonPath + "/blocks/placeholder",
		"expectedRevision": rev,
	})
	if err != nil {
		return 0, "", "", err
	}
	rev, err = revisionOf(body)
	if err != nil {
		return 0, "", "", err
	}
	body, err = s.tool("set_field", map[string]any{
		"id":               PlanID,
		"path":             lessonPath,
		"field":            "visibility",
		"value":            "everyone",
		"expectedRevision": rev,
	})
	if err != nil {
		return 0, "", "", err
	}
	rev, err = revisionOf(body)
	if err != nil {
		return 0, "", "", err
	}
	for _, raw := range lessons[1:] {
		lesson, _ := raw.(map[string]any)
		id, _ := lesson["id"].(string)
		body, err = s.tool("insert_lesson", map[string]any{
			"id":               PlanID,
			"path":             "units/" + unitID,
			"lesson":           lesson,
			"expectedRevision": rev,
		})
		if err != nil {
			return 0, "", "", err
		}
		rev, err = revisionOf(body)
		if err != nil {
			return 0, "", "", err
		}
		body, err = s.tool("set_field", map[string]any{
			"id":               PlanID,
			"path":             "units/" + unitID + "/lessons/" + id,
			"field":            "visibility",
			"value":            "everyone",
			"expectedRevision": rev,
		})
		if err != nil {
			return 0, "", "", err
		}
		rev, err = revisionOf(body)
		if err != nil {
			return 0, "", "", err
		}
	}
	body, err = s.tool("publish_plan", map[string]any{
		"id":               PlanID,
		"expectedRevision": rev,
	})
	if err != nil {
		return 0, "", "", err
	}
	rev, err = revisionOf(body)
	if err != nil {
		return 0, "", "", err
	}
	obj, _ := body.Content.(map[string]any)
	if obj["status"] != "published" {
		return 0, "", "", fmt.Errorf("publish status %#v", obj["status"])
	}
	return rev, storyPath, blockPath, nil
}

func pathsFromLessons(unitID string, lessons []any) (story, block string) {
	for _, raw := range lessons {
		lesson, _ := raw.(map[string]any)
		lessonID, _ := lesson["id"].(string)
		blocks, _ := lesson["blocks"].([]any)
		for _, rawBlock := range blocks {
			item, _ := rawBlock.(map[string]any)
			id, _ := item["id"].(string)
			path := "units/" + unitID + "/lessons/" + lessonID + "/blocks/" + id
			if block == "" && item["type"] != "graded-story" {
				block = path
			}
			if story == "" && item["type"] == "graded-story" {
				story = path
			}
		}
	}
	return story, block
}

func revisionOf(body mcp.ToolBody) (int, error) {
	obj, ok := body.Content.(map[string]any)
	if !ok {
		return 0, fmt.Errorf("tool result is not an object: %s", truncate([]byte(body.Text)))
	}
	rev, ok := asInt(obj["revision"])
	if !ok {
		return 0, fmt.Errorf("revision %#v", obj["revision"])
	}
	return rev, nil
}

func loadJSON(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: not an object", path)
	}
	return obj, nil
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	case float64:
		return int(n), true
	case int:
		return n, true
	default:
		return 0, false
	}
}

func truncate(b []byte) string {
	s := string(b)
	if len(s) > 2000 {
		return s[:2000] + "…"
	}
	return s
}

func moduleRootFromCaller() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}
