package mcp

// Tests of the learner tools (INIT-015/SPEC-021) against the in-memory store: who may call
// each tool, what each returns, and how each checks its input. The tests that need the real
// database, the brief, and the audit rows are in e2e_brief_test.go and
// learner_tools_sql_test.go.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	learnerA = int64(7)
	learnerB = int64(9)
)

// triTokens has two learners, an OAuth admin, and the static token.
type triTokens struct{}

func (triTokens) Validate(_ context.Context, token string, _ *http.Request) (Principal, error) {
	switch token {
	case "learner-a":
		return Principal{Actor: "7", UserID: learnerA, Role: RoleLearner, Source: "oauth"}, nil
	case "learner-b":
		return Principal{Actor: "9", UserID: learnerB, Role: RoleLearner, Source: "oauth"}, nil
	case "admin":
		return Principal{Actor: "1", UserID: 1, Role: RoleAdmin, Source: "oauth"}, nil
	case "static":
		return Principal{Actor: defaultActor, Role: RoleAdmin, Source: "token"}, nil
	default:
		return Principal{}, ErrUnauthorized
	}
}

func mw(text, gloss, state string) ModelWord {
	return ModelWord{Text: text, Gloss: gloss, State: state, Skills: []ModelSkill{
		{Skill: "listen", State: state}, {Skill: "read", State: "unseen"},
	}}
}

type ltEnv struct {
	t        *testing.T
	mem      *Memory
	srv      *httptest.Server
	sessions map[string]*mcp.ClientSession
}

// newLT is a server with learners 7 and 9 and an admin. Learner 7 has the word hello and
// learner 9 has the word coffee, so a read of the wrong learner shows in the text.
func newLT(t *testing.T) *ltEnv {
	t.Helper()
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	mem.AddLexeme(Lexeme{TextNFC: coffee, Bare: "קפה", Gloss: "coffee", POS: "noun"})
	mem.AddLearner(learnerA)
	mem.AddLearner(learnerB)
	mem.SetLearnerModel(learnerA, []ModelWord{mw(hello, "hello", "known")}, &ModelSummary{Words: map[string]int{"known": 1}})
	mem.SetLearnerModel(learnerB, []ModelWord{mw(coffee, "coffee", "struggling")}, &ModelSummary{Words: map[string]int{"struggling": 1}})
	mem.SetProgress(learnerA, Progress{
		Snapshots:   []SnapshotDay{{Day: "2026-10-10", Skills: []SkillDay{{Skill: "listen", Counts: map[string]int{"known": 1}}}}},
		Regressions: []RegressionView{{Day: "2026-10-11", Kind: "word", Text: hello, Skill: "listen", FromState: "known", ToState: "struggling", Cause: "decay"}},
	})
	mem.SetProgress(learnerB, Progress{
		Snapshots:   []SnapshotDay{{Day: "2026-10-10", Skills: []SkillDay{{Skill: "listen", Counts: map[string]int{"known": 5}}}}},
		Regressions: []RegressionView{{Day: "2026-10-11", Kind: "word", Text: coffee, Skill: "read", FromState: "solid", ToState: "regressed", Cause: "BSECRET"}},
	})
	mem.SetBriefTemplate(learnerA, LessonBrief{Tier: BriefTier{Name: "beginner"}, NewWords: []BriefNewWord{{BriefWord: BriefWord{Text: hello, Gloss: "hello"}, MinUses: 3}}})
	mem.SetBriefTemplate(learnerB, LessonBrief{Tier: BriefTier{Name: "beginner"}, NewWords: []BriefNewWord{{BriefWord: BriefWord{Text: coffee, Gloss: "coffee"}, MinUses: 3}}})
	srv := httptest.NewServer(Handler(Options{Validator: triTokens{}, Course: mem, Lexicon: mem, Lookup: mem, Units: mem}))
	t.Cleanup(srv.Close)
	return &ltEnv{t: t, mem: mem, srv: srv}
}

func (e *ltEnv) session(token string) *mcp.ClientSession {
	e.t.Helper()
	if s, ok := e.sessions[token]; ok {
		return s
	}
	if e.sessions == nil {
		e.sessions = map[string]*mcp.ClientSession{}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "lt", Version: "0"}, nil)
	s, err := client.Connect(e.t.Context(), &mcp.StreamableClientTransport{
		Endpoint: e.srv.URL, DisableStandaloneSSE: true, MaxRetries: -1,
		HTTPClient: &http.Client{Transport: bearerTrip{token: token}},
	}, nil)
	if err != nil {
		e.t.Fatalf("connect %s: %v", token, err)
	}
	e.t.Cleanup(func() { _ = s.Close() })
	e.sessions[token] = s
	return s
}

// raw returns the whole structured result as JSON, or the error text.
func (e *ltEnv) raw(token, tool string, args any) (text string, isErr bool) {
	e.t.Helper()
	res, err := e.session(token).CallTool(e.t.Context(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		e.t.Fatalf("%s protocol error: %v", tool, err)
	}
	if res.IsError {
		return resultText(res), true
	}
	b, err := json.Marshal(res.StructuredContent)
	if err != nil {
		e.t.Fatal(err)
	}
	return string(b), false
}

func (e *ltEnv) ok(token, tool string, args any) string {
	e.t.Helper()
	text, isErr := e.raw(token, tool, args)
	if isErr {
		e.t.Fatalf("%s as %s: tool error %s", tool, token, text)
	}
	return text
}

func (e *ltEnv) fail(token, tool string, args any) string {
	e.t.Helper()
	text, isErr := e.raw(token, tool, args)
	if !isErr {
		e.t.Fatalf("%s as %s: want a tool error, got %s", tool, token, text)
	}
	return text
}

func i64(n int64) *int64 { return &n }

// seedAuthz publishes a unit in learner 7's scope and gives learner 7 a brief, and returns
// the unit id. The authz table needs both to call every learner tool with real arguments.
func (e *ltEnv) seedAuthz() int64 {
	e.t.Helper()
	doc := helloDoc()
	res, err := e.mem.PutUnit(e.t.Context(), nil, &doc, "learner:7")
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.mem.PublishUnit(e.t.Context(), res.UnitID, nil); err != nil {
		e.t.Fatal(err)
	}
	e.mem.SetBriefFacts(learnerA, "BRIEF7", units.BriefFacts{})
	return res.UnitID
}

// ---- get_learner_model -------------------------------------------------------------

func TestLearnerModelReadsOwnWordsOnly(t *testing.T) {
	e := newLT(t)
	own := e.ok("learner-a", "get_learner_model", map[string]any{})
	if !strings.Contains(own, hello) || strings.Contains(own, coffee) {
		t.Fatalf("learner a, no id: %s", own)
	}
	self := e.ok("learner-a", "get_learner_model", map[string]any{"learnerId": 7})
	if !strings.Contains(self, hello) || strings.Contains(self, coffee) {
		t.Fatalf("learner a, own id: %s", self)
	}
	// Another learner's id is refused, and nothing of theirs is in the reply.
	msg := e.fail("learner-a", "get_learner_model", map[string]any{"learnerId": 9})
	if msg != msgOtherLearner || strings.Contains(msg, coffee) {
		t.Fatalf("learner a asking for 9: %q", msg)
	}
	// 9.0 is 9, and 7.0 is 7.
	if m := e.fail("learner-a", "get_learner_model", map[string]any{"learnerId": 9.0}); m != msgOtherLearner {
		t.Fatalf("9.0: %q", m)
	}
	if o := e.ok("learner-a", "get_learner_model", map[string]any{"learnerId": 7.0}); !strings.Contains(o, hello) {
		t.Fatalf("7.0: %s", o)
	}
	// An admin names the learner.
	adm := e.ok("admin", "get_learner_model", map[string]any{"learnerId": 9})
	if !strings.Contains(adm, coffee) || strings.Contains(adm, hello) {
		t.Fatalf("admin for 9: %s", adm)
	}
	if m := e.fail("admin", "get_learner_model", map[string]any{}); m != msgLearnerRequired {
		t.Fatalf("admin with no id: %q", m)
	}
	if m := e.fail("static", "get_learner_model", map[string]any{"learnerId": 0}); m != msgLearnerBad {
		t.Fatalf("id 0: %q", m)
	}
	if m := e.fail("static", "get_learner_model", map[string]any{"learnerId": -3}); m != msgLearnerBad {
		t.Fatalf("id -3: %q", m)
	}
	if m := e.fail("static", "get_learner_model", map[string]any{"learnerId": 12345}); m != msgNoSuchLearner {
		t.Fatalf("no such learner: %q", m)
	}
}

// ac-2: pages are right, and a limit over 200 is cut to 200.
func TestLearnerModelPagesAndClampsTheLimit(t *testing.T) {
	e := newLT(t)
	var words []ModelWord
	for i := range 450 {
		words = append(words, mw(fmt.Sprintf("מ%03d", i), "g", "learning"))
	}
	e.mem.SetLearnerModel(learnerA, words, nil)

	type page struct {
		Words      []ModelWord `json:"words"`
		NextCursor *string     `json:"nextCursor"`
		Limit      int         `json:"limit"`
	}
	read := func(args map[string]any) page {
		var p page
		if err := json.Unmarshal([]byte(e.ok("learner-a", "get_learner_model", args)), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}

	big := read(map[string]any{"limit": 5000})
	if len(big.Words) != 200 || big.Limit != 200 || big.NextCursor == nil {
		t.Fatalf("limit 5000: %d words, limit %d, cursor %v", len(big.Words), big.Limit, big.NextCursor)
	}
	if d := read(map[string]any{}); len(d.Words) != 50 || d.Limit != 50 {
		t.Fatalf("default: %d words, limit %d", len(d.Words), d.Limit)
	}
	if n := read(map[string]any{"limit": -4}); len(n.Words) != 50 {
		t.Fatalf("negative limit: %d words", len(n.Words))
	}
	if one := read(map[string]any{"limit": 200}); len(one.Words) != 200 || one.Limit != 200 {
		t.Fatalf("limit 200: %d", len(one.Words))
	}

	// Follow the cursor: 200, 200, 50, with every word once and in order.
	var seen []string
	var sizes []int
	args := map[string]any{"limit": 5000}
	for range 5 {
		p := read(args)
		sizes = append(sizes, len(p.Words))
		for _, w := range p.Words {
			seen = append(seen, w.Text)
		}
		if p.NextCursor == nil {
			break
		}
		args = map[string]any{"limit": 5000, "cursor": *p.NextCursor}
	}
	if !slices.Equal(sizes, []int{200, 200, 50}) {
		t.Fatalf("page sizes %v", sizes)
	}
	if len(seen) != 450 || !slices.IsSorted(seen) || len(slices.Compact(slices.Clone(seen))) != 450 {
		t.Fatalf("pages skipped or repeated a word: %d seen", len(seen))
	}
}

func TestLearnerModelFiltersAndRefusesBadInput(t *testing.T) {
	e := newLT(t)
	b0, b2 := 0, 2
	words := []ModelWord{mw("a", "", "known"), mw("b", "", "struggling"), mw("c", "", "known")}
	words[0].Band, words[2].Band = &b0, &b2
	e.mem.SetLearnerModel(learnerA, words, nil)

	count := func(args map[string]any) int {
		var p struct{ Words []ModelWord }
		if err := json.Unmarshal([]byte(e.ok("learner-a", "get_learner_model", args)), &p); err != nil {
			t.Fatal(err)
		}
		return len(p.Words)
	}
	if n := count(map[string]any{"state": "known"}); n != 2 {
		t.Fatalf("state known: %d", n)
	}
	if n := count(map[string]any{"state": "known", "band": 2}); n != 1 {
		t.Fatalf("state known band 2: %d", n)
	}
	if n := count(map[string]any{"state": "known", "skill": "listen"}); n != 2 {
		t.Fatalf("known in listen: %d", n)
	}
	if n := count(map[string]any{"state": "known", "skill": "read"}); n != 0 {
		t.Fatalf("known in read: %d", n)
	}
	for name, args := range map[string]map[string]any{
		"state":        {"state": "expert"},
		"state case":   {"state": "Known"},
		"skill":        {"skill": "speak"},
		"band high":    {"band": 5},
		"band low":     {"band": -1},
		"cursor":       {"cursor": "not-a-cursor"},
		"cursor long":  {"cursor": strings.Repeat("A", maxCursorLen+1)},
		"state inject": {"state": "known' OR '1'='1"},
	} {
		msg := e.fail("learner-a", "get_learner_model", args)
		if strings.Contains(strings.ToLower(msg), "sql") || strings.Contains(msg, "OR '1'") {
			t.Fatalf("%s: the error repeats the input or names the store: %q", name, msg)
		}
	}
}

// The role middleware reads the exact key learnerId. A body that spells it another way, or
// repeats it, must not reach another learner's data by getting past that read.
func TestLearnerIDSpellingsDoNotCrossLearners(t *testing.T) {
	e := newLT(t)
	for _, tool := range []string{"get_learner_model", "get_progress", "unpublish_unit", "validate_unit"} {
		for _, args := range []string{
			`{"learnerId":9}`,
			`{"LearnerId":9}`,
			`{"LEARNERID":9}`,
			`{"learnerid":9}`,
			`{"learner_id":9}`,
			`{"learnerId":7,"learnerId":9}`,
			`{"learnerId":9,"learnerId":7}`,
			`{"learnerId":"9"}`,
			`{"learnerId":9.5}`,
			`{"learnerId":9e0}`,
			`{"learnerId":null,"LearnerId":9}`,
			`{"learnerId":[9]}`,
		} {
			if tool == "unpublish_unit" || tool == "validate_unit" {
				args = strings.TrimSuffix(args, "}") + `,"unitId":1,"position":0,"briefId":"B"}`
			}
			body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `","arguments":` + args + `}}`
			text := readBody(t, postRaw(t, e.srv.URL, "learner-a", body))
			if strings.Contains(text, coffee) || strings.Contains(text, "BSECRET") {
				t.Errorf("%s with %s returned learner 9's data: %s", tool, args, text)
			}
		}
	}
}

// ---- get_progress ------------------------------------------------------------------

func TestProgressRange(t *testing.T) {
	now := time.Date(2026, 10, 19, 15, 30, 0, 0, time.UTC)
	ok := []struct{ from, to, wantFrom, wantTo string }{
		{"", "", "2026-09-20", "2026-10-19"},
		{"2026-10-01", "", "2026-10-01", "2026-10-19"},
		{"", "2026-06-30", "2026-06-01", "2026-06-30"},
		{"2025-10-18", "2026-10-19", "2025-10-18", "2026-10-19"}, // 366 days
		{"2026-10-19", "2026-10-19", "2026-10-19", "2026-10-19"},
	}
	for _, c := range ok {
		from, to, err := progressRange(c.from, c.to, now)
		if err != nil || from != c.wantFrom || to != c.wantTo {
			t.Errorf("range(%q,%q) = %q %q %v, want %q %q", c.from, c.to, from, to, err, c.wantFrom, c.wantTo)
		}
	}
	bad := []struct{ from, to string }{
		{"2025-10-17", "2026-10-19"}, // 367 days
		{"2000-01-01", "2026-10-19"},
		{"2026-10-20", "2026-10-19"}, // from after to
		{"2026-1-5", ""},
		{"2026-02-30", ""},
		{"20261019", ""},
		{"x", ""},
		{"", "2026-13-01"},
		{"2026-10-19T00:00:00Z", ""},
		{"2026-10-01'; DROP TABLE users;--", ""},
	}
	for _, c := range bad {
		if from, to, err := progressRange(c.from, c.to, now); err == nil {
			t.Errorf("range(%q,%q) = %q %q, want an error", c.from, c.to, from, to)
		}
	}
}

func TestProgressReadsOwnHistoryOnly(t *testing.T) {
	e := newLT(t)
	args := map[string]any{"from": "2026-10-01", "to": "2026-10-19"}
	own := e.ok("learner-a", "get_progress", args)
	if !strings.Contains(own, "decay") || strings.Contains(own, "BSECRET") || strings.Contains(own, coffee) {
		t.Fatalf("learner a: %s", own)
	}
	other := map[string]any{"from": "2026-10-01", "to": "2026-10-19", "learnerId": 9}
	if msg := e.fail("learner-a", "get_progress", other); msg != msgOtherLearner {
		t.Fatalf("learner a asking for 9: %q", msg)
	}
	adm := e.ok("admin", "get_progress", other)
	if !strings.Contains(adm, "BSECRET") || strings.Contains(adm, "decay") {
		t.Fatalf("admin for 9: %s", adm)
	}
	// The range cuts the days. The regression of 10-11 is outside 10-12 to 10-19.
	late := e.ok("learner-a", "get_progress", map[string]any{"from": "2026-10-12", "to": "2026-10-19"})
	if strings.Contains(late, "decay") {
		t.Fatalf("the range did not cut: %s", late)
	}
	// An empty learner gets empty lists, not null and not an error.
	empty := e.ok("learner-b", "get_progress", map[string]any{"from": "2026-01-01", "to": "2026-01-02"})
	for _, key := range []string{`"snapshots":[]`, `"regressions":[]`, `"placements":[]`, `"weeklyReports":[]`, `"changes":[]`} {
		if !strings.Contains(empty, key) {
			t.Fatalf("empty progress lacks %s: %s", key, empty)
		}
	}
	for _, args := range []map[string]any{
		{"from": "2025-10-17", "to": "2026-10-19"},
		{"from": "2026-10-20", "to": "2026-10-19"},
		{"from": "nope"},
	} {
		e.fail("learner-a", "get_progress", args)
	}
}

// ---- get_lesson_brief --------------------------------------------------------------

func TestLessonBrief(t *testing.T) {
	e := newLT(t)
	first := e.ok("admin", "get_lesson_brief", map[string]any{"learnerId": 7})
	second := e.ok("admin", "get_lesson_brief", map[string]any{"learnerId": 7, "theme": "cafe"})
	var a, b LessonBrief
	if err := json.Unmarshal([]byte(first), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(second), &b); err != nil {
		t.Fatal(err)
	}
	if a.BriefID == "" || a.BriefID == b.BriefID || len(a.NewWords) != 1 || a.NewWords[0].Text != hello {
		t.Fatalf("briefs %+v %+v", a, b)
	}
	if strings.Contains(first, coffee) {
		t.Fatalf("learner 7's brief holds learner 9's word: %s", first)
	}
	// Lists are lists, not null.
	for _, key := range []string{`"struggleWords":[]`, `"comfortableWords":[]`, `"coachNotes":[]`, `"notes":[]`, `"activities":[]`} {
		if !strings.Contains(first, key) {
			t.Fatalf("brief lacks %s: %s", key, first)
		}
	}
	// A brief names no learner.
	for _, banned := range []string{"learnerId", "userId", "user_id", "email"} {
		if strings.Contains(first, banned) {
			t.Fatalf("brief holds %s: %s", banned, first)
		}
	}
	for name, theme := range map[string]string{
		"too long": strings.Repeat("ת", maxThemeRunes+1),
		"control":  "cafe\x00",
		"newline":  "a\nb",
		"override": "a\u202Eb",
	} {
		if msg := e.fail("admin", "get_lesson_brief", map[string]any{"learnerId": 7, "theme": theme}); msg != ErrBadTheme.Error() {
			t.Fatalf("%s: %q", name, msg)
		}
	}
	// 200 Hebrew letters is 200 characters, not 400 bytes.
	e.ok("admin", "get_lesson_brief", map[string]any{"learnerId": 7, "theme": strings.Repeat("ת", maxThemeRunes)})
}

// ---- add_coach_note ----------------------------------------------------------------

func TestCoachNote(t *testing.T) {
	e := newLT(t)
	var out coachNoteOutput
	if err := json.Unmarshal([]byte(e.ok("admin", "add_coach_note", map[string]any{"learnerId": 7, "body": "  ח and כ keep coming up in dictation  "})), &out); err != nil {
		t.Fatal(err)
	}
	if out.NoteID == 0 || out.CreatedAt.IsZero() {
		t.Fatalf("result %+v", out)
	}
	notes, err := e.mem.RecentCoachNotes(t.Context(), learnerA, 10)
	if err != nil || len(notes) != 1 || notes[0].Body != "ח and כ keep coming up in dictation" || notes[0].Author != "1" || notes[0].ID != out.NoteID {
		t.Fatalf("stored %+v %v", notes, err)
	}
	other, _ := e.mem.RecentCoachNotes(t.Context(), learnerB, 10)
	if len(other) != 0 {
		t.Fatalf("the note landed on another learner: %+v", other)
	}
	// The static token is stored under its own name.
	e.ok("static", "add_coach_note", map[string]any{"learnerId": 7, "body": "from the token"})
	notes, _ = e.mem.RecentCoachNotes(t.Context(), learnerA, 10)
	if notes[0].Author != defaultActor {
		t.Fatalf("author %q", notes[0].Author)
	}

	for name, body := range map[string]string{
		"empty":      "",
		"spaces":     "   \n\t ",
		"too long":   strings.Repeat("a", maxCoachNoteRunes+1),
		"long heb":   strings.Repeat("ת", maxCoachNoteRunes+1),
		"nul":        "a\x00b",
		"escape":     "a\x1bb",
		"override":   "a\u202Eb",
		"isolate":    "a\u2066b",
		"bom":        "\uFEFFa",
		"c1 control": "a\u0085b",
	} {
		before, _ := e.mem.RecentCoachNotes(t.Context(), learnerA, 100)
		e.fail("admin", "add_coach_note", map[string]any{"learnerId": 7, "body": body})
		after, _ := e.mem.RecentCoachNotes(t.Context(), learnerA, 100)
		if len(after) != len(before) {
			t.Fatalf("%s: a refused note was stored", name)
		}
	}
	// The limit is characters. 500 Hebrew letters is 1000 bytes and is allowed. A line break is allowed.
	e.ok("admin", "add_coach_note", map[string]any{"learnerId": 7, "body": strings.Repeat("ת", maxCoachNoteRunes)})
	e.ok("admin", "add_coach_note", map[string]any{"learnerId": 7, "body": "line one\nline two"})
	if m := e.fail("admin", "add_coach_note", map[string]any{"body": "x"}); m != msgLearnerRequired {
		t.Fatalf("no learnerId: %q", m)
	}
	if m := e.fail("admin", "add_coach_note", map[string]any{"learnerId": 4242, "body": "x"}); m != msgNoSuchLearner {
		t.Fatalf("no such learner: %q", m)
	}
}

// ---- get_context -------------------------------------------------------------------

func TestContextAddsTheLastFiveNotesAndTheModel(t *testing.T) {
	e := newLT(t)
	for i := range 7 {
		e.ok("admin", "add_coach_note", map[string]any{"learnerId": 7, "body": fmt.Sprintf("note %d", i)})
	}
	e.ok("admin", "add_coach_note", map[string]any{"learnerId": 9, "body": "BNOTE"})

	own := call[getContextOutput](t, e.session("learner-a"), "get_context", getContextInput{})
	if len(own.CoachNotes) != 5 || own.CoachNotes[0].Body != "note 6" || own.CoachNotes[4].Body != "note 2" {
		t.Fatalf("notes %+v", own.CoachNotes)
	}
	if own.Model == nil || own.Model.Words["known"] != 1 {
		t.Fatalf("model %+v", own.Model)
	}
	raw := callRaw(t, e.session("learner-a"), "get_context", getContextInput{})
	if strings.Contains(raw, "BNOTE") {
		t.Fatalf("learner a's context holds learner b's note: %s", raw)
	}
	adm := call[getContextOutput](t, e.session("admin"), "get_context", getContextInput{LearnerID: i64(9)})
	if len(adm.CoachNotes) != 1 || adm.CoachNotes[0].Body != "BNOTE" || adm.Model.Words["struggling"] != 1 {
		t.Fatalf("admin for 9: %+v", adm)
	}
	shared := call[getContextOutput](t, e.session("admin"), "get_context", getContextInput{})
	if shared.CoachNotes == nil || len(shared.CoachNotes) != 0 || shared.Model != nil {
		t.Fatalf("no learnerId must give no notes and no model: %+v", shared)
	}
	if raw := callRaw(t, e.session("admin"), "get_context", getContextInput{}); !strings.Contains(raw, `"coachNotes":[]`) || !strings.Contains(raw, `"model":null`) {
		t.Fatalf("shape %s", raw)
	}
}

// ---- unpublish_unit ----------------------------------------------------------------

func TestUnpublishThroughTheMemoryStore(t *testing.T) {
	e := newLT(t)
	ctx := t.Context()
	mk := func(scope string) int64 {
		doc := helloDoc()
		res, err := e.mem.PutUnit(ctx, nil, &doc, scope)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.mem.PublishUnit(ctx, res.UnitID, nil); err != nil {
			t.Fatal(err)
		}
		return res.UnitID
	}
	mine, theirs, shared := mk("learner:7"), mk("learner:9"), mk(ScopeShared)

	// The three refusals have one text, so a reply does not say which case it was.
	m1 := e.fail("learner-a", "unpublish_unit", map[string]any{"unitId": theirs})
	m2 := e.fail("learner-a", "unpublish_unit", map[string]any{"unitId": shared})
	m3 := e.fail("learner-a", "unpublish_unit", map[string]any{"unitId": 99999})
	if m1 != msgNoUnitInScope || m2 != m1 || m3 != m1 {
		t.Fatalf("refusals differ: %q %q %q", m1, m2, m3)
	}
	for _, id := range []int64{theirs, shared} {
		if e.mem.Status(id) != "published" {
			t.Fatalf("unit %d was changed by a refused call", id)
		}
	}
	if len(e.mem.Audits()) != 0 {
		t.Fatalf("a refusal wrote an audit row")
	}
	if msg := e.fail("learner-a", "unpublish_unit", map[string]any{"unitId": theirs, "learnerId": 9}); msg != msgOtherLearner {
		t.Fatalf("learner naming 9: %q", msg)
	}
	if msg := e.fail("learner-a", "unpublish_unit", map[string]any{"unitId": 0}); !strings.Contains(msg, "positive") {
		t.Fatalf("unitId 0: %q", msg)
	}

	out := e.ok("learner-a", "unpublish_unit", map[string]any{"unitId": mine})
	if !strings.Contains(out, `"status":"draft"`) || !strings.Contains(out, `"unchanged":false`) || e.mem.Status(mine) != "draft" {
		t.Fatalf("unpublish %s status %s", out, e.mem.Status(mine))
	}
	if a := e.mem.Audits(); len(a) != 1 || a[0].UnitID != mine || a[0].Actor != "7" || a[0].Action != "unpublish" || a[0].Scope != "learner:7" {
		t.Fatalf("audit %+v", a)
	}
	again := e.ok("learner-a", "unpublish_unit", map[string]any{"unitId": mine})
	if !strings.Contains(again, `"unchanged":true`) || len(e.mem.Audits()) != 1 {
		t.Fatalf("repeat %s audits %d", again, len(e.mem.Audits()))
	}

	// An admin must name the learner, and the unit must be in that learner's scope.
	if m := e.fail("admin", "unpublish_unit", map[string]any{"unitId": theirs}); m != msgLearnerRequired {
		t.Fatalf("admin with no learner: %q", m)
	}
	if m := e.fail("admin", "unpublish_unit", map[string]any{"unitId": theirs, "learnerId": 7}); m != msgNoUnitInScope {
		t.Fatalf("admin with the wrong learner: %q", m)
	}
	if m := e.fail("admin", "unpublish_unit", map[string]any{"unitId": shared, "learnerId": 7}); m != msgNoUnitInScope {
		t.Fatalf("admin on a shared unit: %q", m)
	}
	e.ok("admin", "unpublish_unit", map[string]any{"unitId": theirs, "learnerId": 9})
	if e.mem.Status(theirs) != "draft" || e.mem.Status(shared) != "published" {
		t.Fatalf("statuses %s %s", e.mem.Status(theirs), e.mem.Status(shared))
	}
}

// ---- validate_unit with briefId ----------------------------------------------------

func TestValidateWithBriefIsScopedToTheLearner(t *testing.T) {
	e := newLT(t)
	// Learner 7's brief wants a new word the unit does not hold.
	e.mem.SetBriefFacts(learnerA, "BRIEF7", units.BriefFacts{
		Tier: "beginner", MinKnownShare: 0.6, MaxTextTokens: 20,
		NewWords:       []units.BriefNewWord{{LexemeID: 1, Text: coffee, MinUses: 3}},
		BriefLexemeIDs: map[int64]bool{1: true}, KnownLexemeIDs: map[int64]bool{},
	})
	args := func(extra map[string]any) map[string]any {
		m := map[string]any{"position": 0, "doc": helloDoc(), "briefId": "BRIEF7"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	rules := func(text string) []string {
		var out struct{ Issues []units.Issue }
		if err := json.Unmarshal([]byte(text), &out); err != nil {
			t.Fatal(err)
		}
		var r []string
		for _, is := range out.Issues {
			if strings.HasPrefix(is.Rule, "brief.") {
				r = append(r, is.Rule+" "+is.Pointer)
			}
		}
		return r
	}

	own := rules(e.ok("learner-a", "validate_unit", args(nil)))
	if len(own) == 0 || slices.Contains(own, units.RuleBriefNotFound+" /briefId") {
		t.Fatalf("learner a with its own brief: %v", own)
	}
	if adm := rules(e.ok("admin", "validate_unit", args(map[string]any{"learnerId": 7}))); !slices.Equal(adm, own) {
		t.Fatalf("admin for 7 %v, want %v", adm, own)
	}
	// Learner 9 holds the id of 7's brief and gets the same answer as for an id that does not exist.
	stolen := rules(e.ok("learner-b", "validate_unit", args(nil)))
	missing := rules(e.ok("learner-b", "validate_unit", args(map[string]any{"briefId": "NOSUCHBRIEF"})))
	if !slices.Equal(stolen, missing) || !slices.Contains(stolen, units.RuleBriefNotFound+" /briefId") {
		t.Fatalf("stolen %v, missing %v", stolen, missing)
	}
	if text, _ := e.raw("learner-b", "validate_unit", args(nil)); strings.Contains(text, coffee) {
		t.Fatalf("learner 9 saw learner 7's brief words: %s", text)
	}
	// Naming learner 7 does not help learner 9.
	if m := e.fail("learner-b", "validate_unit", args(map[string]any{"learnerId": 7})); m != msgOtherLearner {
		t.Fatalf("learner 9 naming 7: %q", m)
	}
	// An admin who does not name the learner gets an error, not a guess.
	if m := e.fail("admin", "validate_unit", args(nil)); m != msgLearnerRequired {
		t.Fatalf("admin with no learner: %q", m)
	}
	// An admin naming 9 cannot use 7's brief either.
	if got := rules(e.ok("admin", "validate_unit", args(map[string]any{"learnerId": 9}))); !slices.Contains(got, units.RuleBriefNotFound+" /briefId") {
		t.Fatalf("admin for 9 on 7's brief: %v", got)
	}
	// Without briefId the call is the v2 dry run, and learnerId is not needed.
	plain := e.ok("admin", "validate_unit", map[string]any{"position": 0, "doc": helloDoc()})
	if len(rules(plain)) != 0 {
		t.Fatalf("a plain dry run ran brief rules: %s", plain)
	}
}

// ---- cross-cutting -----------------------------------------------------------------

// No output of a learner tool holds a field that names a learner, an account, or a secret.
func TestNoToolOutputHoldsIdentityOrSecrets(t *testing.T) {
	e := newLT(t)
	e.ok("admin", "add_coach_note", map[string]any{"learnerId": 7, "body": "n"})
	banned := []string{"email", "token", "secret", "oauth", "googlesub", "google_sub", "password", "userid", "user_id", "learnerid", "sub"}
	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				if slices.Contains(banned, strings.ToLower(k)) {
					t.Errorf("%s.%s is a field that must not be in a tool output", path, k)
				}
				walk(path+"."+k, child)
			}
		case []any:
			for _, child := range x {
				walk(path+"[]", child)
			}
		}
	}
	for _, c := range []struct {
		tool string
		args map[string]any
	}{
		{"get_learner_model", map[string]any{"learnerId": 7}},
		{"get_progress", map[string]any{"learnerId": 7, "from": "2026-10-01", "to": "2026-10-19"}},
		{"get_lesson_brief", map[string]any{"learnerId": 7}},
		{"add_coach_note", map[string]any{"learnerId": 7, "body": "x"}},
		{"get_context", map[string]any{"position": 0, "learnerId": 7}},
	} {
		var v any
		if err := json.Unmarshal([]byte(e.ok("admin", c.tool, c.args)), &v); err != nil {
			t.Fatal(err)
		}
		walk(c.tool, v)
	}
}

// failingData returns an error that names the database, as a real store error might.
type failingData struct{ *Memory }

var errInternal = errors.New(`sqlite: no such table: lesson_briefs at /var/data/hebrew.sqlite (db: store is closed)`)

func (failingData) LearnerModel(context.Context, int64, ModelQuery) (ModelPage, error) {
	return ModelPage{}, errInternal
}
func (failingData) LearnerModelSummary(context.Context, int64) (ModelSummary, error) {
	return ModelSummary{}, errInternal
}
func (failingData) Progress(context.Context, int64, string, string) (Progress, error) {
	return Progress{}, errInternal
}
func (failingData) BuildLessonBrief(context.Context, int64, string) (LessonBrief, error) {
	return LessonBrief{}, errInternal
}
func (failingData) AddCoachNote(context.Context, int64, string, string) (CoachNote, error) {
	return CoachNote{}, errInternal
}
func (failingData) RecentCoachNotes(context.Context, int64, int) ([]CoachNote, error) {
	return nil, errInternal
}
func (failingData) UnpublishLearner(context.Context, Principal, int64) (UnpublishResult, error) {
	return UnpublishResult{}, errInternal
}
func (failingData) BriefFacts(context.Context, int64, string) (units.BriefFacts, error) {
	return units.BriefFacts{}, errInternal
}

func TestStoreErrorsDoNotLeakInternals(t *testing.T) {
	mem := NewMemory()
	mem.AddLexeme(helloLexeme())
	mem.AddLearner(learnerA)
	store := failingData{mem}
	srv := httptest.NewServer(Handler(Options{Validator: triTokens{}, Course: store, Lexicon: mem, Lookup: mem, Units: store}))
	t.Cleanup(srv.Close)
	e := &ltEnv{t: t, mem: mem, srv: srv}
	for _, c := range []struct {
		tool string
		args map[string]any
	}{
		{"get_learner_model", map[string]any{"learnerId": 7}},
		{"get_progress", map[string]any{"learnerId": 7}},
		{"get_lesson_brief", map[string]any{"learnerId": 7}},
		{"add_coach_note", map[string]any{"learnerId": 7, "body": "x"}},
		{"unpublish_unit", map[string]any{"learnerId": 7, "unitId": 1}},
		{"validate_unit", map[string]any{"learnerId": 7, "briefId": "B", "position": 0, "doc": helloDoc()}},
		{"get_context", map[string]any{"learnerId": 7, "position": 0}},
	} {
		msg := e.fail("admin", c.tool, c.args)
		for _, leak := range []string{"sqlite", "lesson_briefs", "/var/data", "store is closed", "db:"} {
			if strings.Contains(msg, leak) {
				t.Errorf("%s leaked %q: %s", c.tool, leak, msg)
			}
		}
	}
}

// aud-1: 60 calls a minute for each caller, and a caller does not spend another's.
func TestRateLimitIsPerCaller(t *testing.T) {
	e := newLT(t)
	args := map[string]any{"learnerId": 7}
	for i := range learnerToolsPerMinute {
		if text, isErr := e.raw("admin", "get_learner_model", args); isErr {
			t.Fatalf("call %d refused: %s", i+1, text)
		}
	}
	if msg := e.fail("admin", "get_learner_model", args); msg != msgRateLimited {
		t.Fatalf("call 61: %q", msg)
	}
	// The limit is shared by the learner tools, not one per tool.
	if msg := e.fail("admin", "get_progress", args); msg != msgRateLimited {
		t.Fatalf("another tool: %q", msg)
	}
	// Another caller still has its calls.
	e.ok("learner-a", "get_learner_model", map[string]any{})
	e.ok("static", "get_learner_model", args)
	// A denied call does not use up the limit of the caller that made it.
	for range 80 {
		e.fail("learner-b", "get_lesson_brief", map[string]any{"learnerId": 9})
	}
	e.ok("learner-b", "get_learner_model", map[string]any{})
}

func TestLimiterWindow(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	l := newLimiter(func() time.Time { return now }, 3, time.Minute)
	for i := range 3 {
		if !l.allow("k") {
			t.Fatalf("call %d refused", i+1)
		}
	}
	if l.allow("k") {
		t.Fatal("call 4 was allowed")
	}
	if !l.allow("other") {
		t.Fatal("another key was refused")
	}
	now = now.Add(59 * time.Second)
	if l.allow("k") {
		t.Fatal("allowed inside the window")
	}
	now = now.Add(2 * time.Second)
	if !l.allow("k") {
		t.Fatal("refused after the window")
	}
	// The table does not grow without bound.
	for i := range maxRateKeys + 100 {
		now = now.Add(time.Second)
		l.allow(fmt.Sprintf("key-%d", i))
	}
	l.mu.Lock()
	n := len(l.buckets)
	l.mu.Unlock()
	if n > maxRateKeys+1 {
		t.Fatalf("%d keys", n)
	}
}

// The second check, below the role middleware: learnerFor itself refuses.
func TestLearnerForRules(t *testing.T) {
	mem := NewMemory()
	mem.AddLearner(7)
	s := &service{learner: mem}
	ctx := t.Context()
	for _, c := range []struct {
		name string
		p    Principal
		req  *int64
		want int64
		err  string
	}{
		{"learner, no id", Principal{Role: RoleLearner, UserID: 7}, nil, 7, ""},
		{"learner, own id", Principal{Role: RoleLearner, UserID: 7}, i64(7), 7, ""},
		{"learner, other id", Principal{Role: RoleLearner, UserID: 7}, i64(9), 0, msgOtherLearner},
		{"learner, id 0", Principal{Role: RoleLearner, UserID: 7}, i64(0), 0, msgOtherLearner},
		{"learner with no account", Principal{Role: RoleLearner}, nil, 0, msgAdminOnly},
		{"admin, no id", Principal{Role: RoleAdmin, UserID: 1}, nil, 0, msgLearnerRequired},
		{"admin, id 7", Principal{Role: RoleAdmin, UserID: 1}, i64(7), 7, ""},
		{"admin, itself is not a default", Principal{Role: RoleAdmin, UserID: 1}, nil, 0, msgLearnerRequired},
		{"admin, id 0", Principal{Role: RoleAdmin}, i64(0), 0, msgLearnerBad},
		{"admin, unknown", Principal{Role: RoleAdmin}, i64(8), 0, msgNoSuchLearner},
		{"no role", Principal{UserID: 7}, i64(7), 0, msgAdminOnly},
		{"odd role", Principal{Role: "owner", UserID: 7}, i64(7), 0, msgAdminOnly},
	} {
		got, err := s.learnerFor(ctx, c.p, c.req)
		switch {
		case c.err == "" && (err != nil || got != c.want):
			t.Errorf("%s: %d %v, want %d", c.name, got, err, c.want)
		case c.err != "" && (err == nil || err.Error() != c.err || got != 0):
			t.Errorf("%s: %d %v, want error %q", c.name, got, err, c.err)
		}
	}
}

// A tool whose input has learnerId must be role-checked on it: a learner tool must be in
// learnerOwnDataTools so the middleware refuses another id before the handler runs, and
// anything else must be admin-only. A new tool that forgets this fails here.
func TestEveryLearnerIDToolIsGuarded(t *testing.T) {
	e := newLT(t)
	tools, err := e.session("admin").ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, tool := range tools.Tools {
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), `"learnerId"`) {
			if learnerOwnDataTools[tool.Name] {
				t.Errorf("%s is in learnerOwnDataTools but takes no learnerId", tool.Name)
			}
			continue
		}
		seen++
		if learnerTools[tool.Name] && !learnerOwnDataTools[tool.Name] {
			t.Errorf("%s takes learnerId and a learner may call it, but the middleware does not check the id", tool.Name)
		}
	}
	if seen < 8 {
		t.Fatalf("only %d tools take learnerId", seen)
	}
	// The write tools that feed the next prompt are not learner tools.
	for _, name := range []string{"get_lesson_brief", "add_coach_note", "put_unit", "publish_unit", "generate_audio", "upsert_lexemes"} {
		if learnerTools[name] {
			t.Errorf("%s must be admin-only", name)
		}
	}
}

// ac-5: resources/list includes hebrew://authoring, and it reads.
func TestAuthoringResource(t *testing.T) {
	e := newLT(t)
	s := e.session("learner-a")
	res, err := s.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var found *mcp.Resource
	for _, r := range res.Resources {
		if r.URI == resourceAuthoring {
			found = r
		}
	}
	if found == nil || found.URI != "hebrew://authoring" || found.MIMEType != "text/markdown" {
		t.Fatalf("resources %+v", res.Resources)
	}
	text := readResource(t, s, resourceAuthoring)
	for _, want := range []string{"get_lesson_brief", "validate_unit", "generate_audio", "put_unit", "add_coach_note", "briefId"} {
		if !strings.Contains(text, want) {
			t.Errorf("the authoring stub does not mention %s", want)
		}
	}
	for _, banned := range []string{"bearer", "secret", "password", "@", "MCP_AUTH"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(banned)) {
			t.Errorf("the authoring text mentions %q", banned)
		}
	}
}

// Each new tool says who may call it, and the limit, the way publish_unit does.
func TestNewToolDescriptions(t *testing.T) {
	e := newLT(t)
	tools, err := e.session("admin").ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	adminOnly := map[string]bool{"get_lesson_brief": true, "add_coach_note": true}
	for _, name := range []string{"get_learner_model", "get_progress", "get_lesson_brief", "add_coach_note", "unpublish_unit"} {
		d := tools.Tools[indexOf(tools, name)].Description
		if adminOnly[name] != strings.Contains(d, "Admin only") {
			t.Errorf("%s: description and policy disagree about admin only: %s", name, d)
		}
		if !strings.Contains(d, "learnerId") || !strings.Contains(d, "does not exist") && name != "unpublish_unit" {
			t.Errorf("%s: description does not say how learnerId works", name)
		}
		if name != "unpublish_unit" && !strings.Contains(d, "60 calls a minute") {
			t.Errorf("%s: description does not state the limit", name)
		}
		if strings.Count(d, ".") < 4 {
			t.Errorf("%s: description is too thin: %s", name, d)
		}
	}
	for _, name := range []string{"get_learner_model", "get_progress"} {
		if a := tools.Tools[indexOf(tools, name)].Annotations; a == nil || !a.ReadOnlyHint {
			t.Errorf("%s must be read-only", name)
		}
	}
}
