package mcp_test

// End-to-end tests of learner auto-publish (INIT-015/SPEC-020). They run the real
// handler over the real store on a real SQLite file: put_unit with publish true,
// the brief checks, the brief claim, and the audit table. A black-box package, so
// it can import internal/mcpstore, which imports internal/mcp.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/brief"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcpstore"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

var apNow = time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)

type apClock struct{}

func (apClock) Now() time.Time { return apNow }

// The words of the fixture. Three new words, one struggle word.
var (
	apShalom = units.Normalize("\u05E9\u05B8\u05C1\u05DC\u05D5\u05B9\u05DD")
	apBoker  = units.Normalize("\u05D1\u05BC\u05B9\u05E7\u05B6\u05E8")
	apToda   = units.Normalize("\u05EA\u05BC\u05D5\u05B9\u05D3\u05B8\u05D4")
	apMayim  = units.Normalize("\u05DE\u05B7\u05D9\u05B4\u05DD")
)

type apEnv struct {
	t       *testing.T
	store   *db.Store
	adapter *mcpstore.Store
	session *sdkmcp.ClientSession
	a, b    int64
	ids     map[string]int64
}

type apPut struct {
	UnitID        int64         `json:"unitId"`
	Revision      int           `json:"revision"`
	Unchanged     bool          `json:"unchanged"`
	Published     bool          `json:"published"`
	PublishReason string        `json:"publishReason"`
	Issues        []units.Issue `json:"issues"`
}

func newAP(t *testing.T) *apEnv {
	t.Helper()
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), apClock{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	e := &apEnv{t: t, store: store, adapter: mcpstore.New(store), ids: map[string]int64{}}
	if e.a, err = store.InsertUser(ctx, "a@example.com", "sub-a", "learner"); err != nil {
		t.Fatal(err)
	}
	if e.b, err = store.InsertUser(ctx, "b@example.com", "sub-b", "learner"); err != nil {
		t.Fatal(err)
	}
	for _, w := range []struct{ text, gloss, gender string }{
		{apShalom, "hello", "m"}, {apBoker, "morning", "m"}, {apToda, "thanks", "f"}, {apMayim, "water", "m"},
	} {
		e.exec(`INSERT INTO lexemes (text_nfc, bare, gloss_en, pos, gender) VALUES (?, ?, ?, 'noun', ?)`,
			w.text, units.StripNiqqud(w.text), w.gloss, w.gender)
		e.ids[w.text] = int64(e.count(`SELECT id FROM lexemes WHERE text_nfc = ?`, w.text))
	}
	e.session = e.connect(mcp.Options{AuthToken: "secret", Course: e.adapter, Lexicon: e.adapter, Lexemes: e.adapter, Lookup: e.adapter, Units: e.adapter}, "secret")
	return e
}

func (e *apEnv) connect(opts mcp.Options, token string) *sdkmcp.ClientSession {
	e.t.Helper()
	srv := httptest.NewServer(mcp.Handler(opts))
	e.t.Cleanup(srv.Close)
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "author-test", Version: "0"}, nil)
	session, err := client.Connect(e.t.Context(), &sdkmcp.StreamableClientTransport{
		Endpoint:             srv.URL,
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
		HTTPClient:           &http.Client{Transport: apBearer{token: token}},
	}, nil)
	if err != nil {
		e.t.Fatalf("connect: %v", err)
	}
	e.t.Cleanup(func() { _ = session.Close() })
	return session
}

type apBearer struct{ token string }

func (b apBearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

func (e *apEnv) exec(query string, args ...any) {
	e.t.Helper()
	conn, err := e.store.DB()
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := conn.ExecContext(e.t.Context(), query, args...); err != nil {
		e.t.Fatalf("exec %s: %v", query, err)
	}
}

func (e *apEnv) count(query string, args ...any) int {
	e.t.Helper()
	conn, err := e.store.DB()
	if err != nil {
		e.t.Fatal(err)
	}
	var n int
	if err := conn.QueryRowContext(e.t.Context(), query, args...).Scan(&n); err != nil {
		e.t.Fatalf("count %s: %v", query, err)
	}
	return n
}

func (e *apEnv) text(query string, args ...any) string {
	e.t.Helper()
	conn, err := e.store.DB()
	if err != nil {
		e.t.Fatal(err)
	}
	var s string
	if err := conn.QueryRowContext(e.t.Context(), query, args...).Scan(&s); err != nil {
		e.t.Fatalf("text %s: %v", query, err)
	}
	return s
}

func (e *apEnv) status(id int64) string {
	return e.text(`SELECT status FROM units WHERE id = ?`, id)
}

func (e *apEnv) audits() int { return e.count(`SELECT COUNT(*) FROM publish_audit`) }

func (e *apEnv) briefUsed(id string) int {
	return e.count(`SELECT COALESCE(used_by_unit_id, 0) FROM lesson_briefs WHERE id = ?`, id)
}

// brief stores a brief for the learner: the three new words and the struggle word.
// The tier is beginner: 60% known, at most 20 tokens per passage.
func (e *apEnv) brief(id string, user int64, created time.Time) {
	e.t.Helper()
	word := func(text, gloss string) brief.Word {
		return brief.Word{LexemeID: e.ids[text], Text: text, Gloss: gloss}
	}
	plan := brief.Plan{
		V: brief.Version, Theme: "cafe", Tier: brief.TierFor(10),
		NewWords: []brief.NewWord{
			{Word: word(apShalom, "hello"), MinUses: 3}, {Word: word(apBoker, "morning"), MinUses: 3}, {Word: word(apToda, "thanks"), MinUses: 3},
		},
		StruggleWords: []brief.StruggleWord{{
			Word: word(apMayim, "water"), State: "struggling", Skill: "listen", RequiredCount: 3, Activities: []string{"listen_pick"},
		}},
		ComfortableWords: []brief.Word{}, Activities: []string{}, Notes: []string{}, CoachNotes: []brief.CoachNote{},
		Targets: brief.Targets{Coverage: brief.CoverageTarget, NewWordMinUses: brief.NewWordMinUses},
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		e.t.Fatal(err)
	}
	e.exec(`INSERT INTO lesson_briefs (id, user_id, theme, payload_json, created_at) VALUES (?, ?, 'cafe', ?, ?)`,
		id, user, string(raw), db.FormatTime(created))
}

func (e *apEnv) goodBrief(id string, user int64) { e.brief(id, user, apNow.Add(-time.Hour)) }

// audio stores a clip for every Hebrew text of the unit, so audio.missing and
// brief.audio_all have nothing to say.
func (e *apEnv) audio(doc units.Unit) {
	e.t.Helper()
	texts := map[string]bool{}
	for _, w := range doc.NewWords {
		texts[units.Normalize(w)] = true
	}
	for _, les := range doc.Lessons {
		for _, act := range les.Activities {
			texts[units.Normalize(act.Text)] = true
		}
	}
	for text := range texts {
		sum := sha256.Sum256([]byte(text))
		e.exec(`INSERT OR IGNORE INTO audio_assets (id, text_nfc, voice, model, bytes, created_at) VALUES (?, ?, 'v', 'm', 1, ?)`,
			hex.EncodeToString(sum[:]), text, db.FormatTime(apNow))
	}
}

// apDoc is a unit that passes every rule against the brief of e.brief. Lesson 0
// shows each new word (shadow) before it is checked, then checks it, and reads a
// story. A checkpoint recycles the new words and plays the struggle word three times.
func apDoc(title string) units.Unit {
	act := func(typ, text string) units.Activity {
		a := units.Activity{Type: typ, Text: text}
		if typ == units.ActivityListenPick {
			fields := strings.Fields(text)
			if len(fields) > 1 {
				a.Target = fields[0]
				a.Translation = "a short line"
			}
		}
		return a
	}
	doc := units.Unit{
		Title: title, Situation: "A cafe", Concept: "Greetings",
		NewWords: []string{apShalom, apBoker, apToda},
		Lessons: []units.Lesson{
			{Kind: units.KindLesson, Activities: []units.Activity{
				act(units.ActivityShadow, apShalom), act(units.ActivityShadow, apBoker), act(units.ActivityShadow, apToda),
				act(units.ActivityRecall, apShalom), act(units.ActivityRecall, apBoker), act(units.ActivityRecall, apToda),
				act(units.ActivityListenPick, apShalom+" "+apBoker),
				act(units.ActivityStory, apShalom+" "+apBoker+" "+apToda),
			}},
			{Kind: units.KindCheckpoint, Activities: []units.Activity{
				act(units.ActivityRecall, apShalom), act(units.ActivityRecall, apBoker), act(units.ActivityRecall, apToda),
				act(units.ActivityListenPick, apMayim), act(units.ActivityListenPick, apMayim), act(units.ActivityListenPick, apMayim),
			}},
		},
	}
	return withSentenceTranslations(doc)
}

// withSentenceTranslations fills a short English line on every multi-word
// listen_pick, then other activities, until a unit that has one has four
// translated lines. The autopublish fixtures are about coverage, not sentences.
func withSentenceTranslations(doc units.Unit) units.Unit {
	n := 0
	sentence := false
	for i := range doc.Lessons {
		for j := range doc.Lessons[i].Activities {
			a := &doc.Lessons[i].Activities[j]
			if strings.TrimSpace(a.Translation) != "" {
				n++
			}
			if a.Type == units.ActivityListenPick && len(strings.Fields(a.Text)) > 1 {
				sentence = true
				if strings.TrimSpace(a.Translation) == "" {
					a.Translation = "a short line"
					n++
				}
			}
		}
	}
	if !sentence {
		return doc
	}
	for i := range doc.Lessons {
		for j := range doc.Lessons[i].Activities {
			if n >= 4 {
				return doc
			}
			a := &doc.Lessons[i].Activities[j]
			if strings.TrimSpace(a.Translation) != "" {
				continue
			}
			a.Translation = "another line"
			n++
		}
	}
	return doc
}

func (e *apEnv) put(scope string, unitID *int64, doc units.Unit, briefID string, publish bool) apPut {
	e.t.Helper()
	args := map[string]any{"scope": scope, "position": 0, "doc": doc, "publish": publish}
	if unitID != nil {
		args["unitId"] = *unitID
	}
	if briefID != "" {
		args["briefId"] = briefID
	}
	return apCall(e.t, e.session, args)
}

func apCall(t testing.TB, session *sdkmcp.ClientSession, args map[string]any) apPut {
	t.Helper()
	res, err := session.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: "put_unit", Arguments: args})
	if err != nil {
		t.Fatalf("put_unit: %v", err)
	}
	if res.IsError {
		t.Fatalf("put_unit: tool error %s", apText(res))
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var out apPut
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

func apText(res *sdkmcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*sdkmcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func apHas(issues []units.Issue, rule, pointer string) bool {
	for _, is := range issues {
		if is.Rule == rule && (pointer == "" || is.Pointer == pointer) {
			return true
		}
	}
	return false
}

func apBlocks(issues []units.Issue) []string {
	var out []string
	for _, is := range issues {
		if is.Severity == units.SeverityBlock {
			out = append(out, is.Rule+" "+is.Pointer)
		}
	}
	return out
}

const (
	// A brief id is 16 random bytes in unpadded base64url, so the last character has its low four bits clear: A, Q, g, or w.
	apBriefA = "AAAAAAAAAAAAAAAAAAAAAA"
	apBriefB = "BBBBBBBBBBBBBBBBBBBBBQ"
)

// TestAutoPublishPassingUnit is ac-2: a unit that passes every rule is published by
// put_unit with publish true, and exactly one audit row is written.
func TestAutoPublishPassingUnit(t *testing.T) {
	e := newAP(t)
	doc := apDoc("Cafe")
	e.audio(doc)
	e.goodBrief(apBriefA, e.a)

	out := e.put(db.LearnerScope(e.a), nil, doc, apBriefA, true)
	if !out.Published || out.Unchanged || out.PublishReason != "" || e.status(out.UnitID) != "published" {
		t.Fatalf("not published: %+v (blocks %v) status %s", out, apBlocks(out.Issues), e.status(out.UnitID))
	}
	if len(apBlocks(out.Issues)) != 0 {
		t.Fatalf("block issues on a published unit: %v", apBlocks(out.Issues))
	}
	if !apHas(out.Issues, "recycle.count", "/newWords/0") {
		t.Fatalf("the warnings are missing from the result: %+v", out.Issues)
	}
	if n := e.audits(); n != 1 {
		t.Fatalf("audit rows = %d, want 1", n)
	}
	var issuesJSON string
	if n := e.count(`SELECT COUNT(*) FROM publish_audit WHERE unit_id = ? AND revision = ? AND scope = ? AND actor = 'mcp' AND action = 'auto_publish' AND brief_id = ?`,
		out.UnitID, out.Revision, db.LearnerScope(e.a), apBriefA); n != 1 {
		t.Fatalf("audit row does not name the actor, unit, revision, scope, and brief")
	}
	issuesJSON = e.text(`SELECT issues_json FROM publish_audit`)
	var audited []units.Issue
	if err := json.Unmarshal([]byte(issuesJSON), &audited); err != nil || !apHas(audited, "recycle.count", "") {
		t.Fatalf("audit issues %s: %v (warnings must be recorded)", issuesJSON, err)
	}
	if got := e.briefUsed(apBriefA); int64(got) != out.UnitID {
		t.Fatalf("brief used by %d, want %d", got, out.UnitID)
	}

	// The same save again is an exact repeat: still published, nothing new written.
	again := e.put(db.LearnerScope(e.a), &out.UnitID, doc, apBriefA, true)
	if !again.Published || !again.Unchanged || again.Revision != out.Revision || e.audits() != 1 {
		t.Fatalf("repeat %+v audits %d", again, e.audits())
	}
}

// TestAutoPublishNoBriefIDStaysDraft is ac-3.
func TestAutoPublishNoBriefIDStaysDraft(t *testing.T) {
	e := newAP(t)
	doc := apDoc("Cafe")
	e.audio(doc)
	e.goodBrief(apBriefA, e.a)
	out := e.put(db.LearnerScope(e.a), nil, doc, "", true)
	if out.Published || e.status(out.UnitID) != "draft" {
		t.Fatalf("published with no briefId: %+v", out)
	}
	if !strings.Contains(out.PublishReason, "no briefId") || !strings.Contains(out.PublishReason, "draft") {
		t.Fatalf("reason %q", out.PublishReason)
	}
	if e.audits() != 0 || e.briefUsed(apBriefA) != 0 {
		t.Fatal("a draft wrote an audit row or spent the brief")
	}
	// Blank and whitespace ids are no id.
	if out := e.put(db.LearnerScope(e.a), &out.UnitID, doc, "   ", true); out.Published {
		t.Fatalf("whitespace briefId published: %+v", out)
	}
	// Publish false saves a draft and does not look at the brief.
	if out := e.put(db.LearnerScope(e.a), nil, apDoc("Quiet"), apBriefA, false); out.Published || out.PublishReason != "" || e.briefUsed(apBriefA) != 0 {
		t.Fatalf("publish false: %+v", out)
	}
	// Shared scope never auto-publishes, with or without a brief.
	if out := e.put("shared", nil, apDoc("Shared"), apBriefA, true); out.Published || e.status(out.UnitID) != "draft" {
		t.Fatalf("shared: %+v", out)
	}
}

// TestAutoPublishBriefReuse is ac-5: a used brief id is a block issue.
func TestAutoPublishBriefReuse(t *testing.T) {
	e := newAP(t)
	doc := apDoc("Cafe")
	e.audio(doc)
	e.goodBrief(apBriefA, e.a)
	first := e.put(db.LearnerScope(e.a), nil, doc, apBriefA, true)
	if !first.Published {
		t.Fatalf("first: %+v", first)
	}
	second := apDoc("Cafe again")
	e.audio(second)
	out := e.put(db.LearnerScope(e.a), nil, second, apBriefA, true)
	if out.Published || e.status(out.UnitID) != "draft" {
		t.Fatalf("a used brief published a second unit: %+v", out)
	}
	if !apHas(out.Issues, units.RuleBriefUsed, "/briefId") {
		t.Fatalf("no brief.used issue: %+v", out.Issues)
	}
	for _, is := range out.Issues {
		if is.Rule == units.RuleBriefUsed && is.Severity != units.SeverityBlock {
			t.Fatalf("brief.used is %s", is.Severity)
		}
	}
	if e.audits() != 1 || int64(e.briefUsed(apBriefA)) != first.UnitID {
		t.Fatalf("audits %d, brief used by %d", e.audits(), e.briefUsed(apBriefA))
	}
	// Editing the published unit and publishing it again with its own, used brief is
	// a second use, not a repeat: the revision is new, so it is a block issue too.
	edited := apDoc("Cafe edited")
	e.audio(edited)
	if out := e.put(db.LearnerScope(e.a), &first.UnitID, edited, apBriefA, true); out.Published || !apHas(out.Issues, units.RuleBriefUsed, "/briefId") {
		t.Fatalf("re-publish of an edit with a used brief: %+v", out)
	}
}

// TestAutoPublishBriefMissingExpiredOrForeign: a brief that cannot be used is a
// block issue, never an error, and it is never spent.
func TestAutoPublishBriefMissingExpiredOrForeign(t *testing.T) {
	e := newAP(t)
	doc := apDoc("Cafe")
	e.audio(doc)
	e.goodBrief(apBriefB, e.b)
	e.brief("EEEEEEEEEEEEEEEEEEEEEQ", e.a, apNow.Add(-brief.TTL-time.Minute))
	for _, c := range []struct {
		name, id, rule string
	}{
		{"another learner's brief", apBriefB, units.RuleBriefNotFound},
		{"an unknown brief", "CCCCCCCCCCCCCCCCCCCCCQ", units.RuleBriefNotFound},
		{"a malformed id", "x", units.RuleBriefNotFound},
		{"an id with markup", "<script>alert(1)</script>", units.RuleBriefNotFound},
		{"an expired brief", "EEEEEEEEEEEEEEEEEEEEEQ", units.RuleBriefExpired},
	} {
		out := e.put(db.LearnerScope(e.a), nil, doc, c.id, true)
		if out.Published || e.status(out.UnitID) != "draft" || !apHas(out.Issues, c.rule, "/briefId") {
			t.Fatalf("%s: %+v", c.name, out)
		}
		if !strings.Contains(out.PublishReason, "block issue") {
			t.Fatalf("%s: reason %q", c.name, out.PublishReason)
		}
	}
	if e.audits() != 0 || e.briefUsed(apBriefB) != 0 || e.briefUsed("EEEEEEEEEEEEEEEEEEEEEQ") != 0 {
		t.Fatal("a refused brief was spent or audited")
	}
	// learner:0 has no briefs at all.
	if out := e.put(db.LearnerScope(0), nil, doc, apBriefA, true); out.Published {
		t.Fatalf("learner:0 published: %+v", out)
	}
}

// TestAutoPublishBriefRulesBlock: a unit that breaks a brief rule stays a draft, does
// not spend the brief, and publishes once it is fixed.
func TestAutoPublishBriefRulesBlock(t *testing.T) {
	e := newAP(t)
	e.goodBrief(apBriefA, e.a)
	bad := apDoc("Cafe")
	bad.Lessons[0].Activities = bad.Lessons[0].Activities[1:] // the first word loses its shadow
	e.audio(bad)
	out := e.put(db.LearnerScope(e.a), nil, bad, apBriefA, true)
	if out.Published || !apHas(out.Issues, units.RuleBriefIntro, "/lessons/0/activities/2/text") {
		// shadow B and shadow C remain; the first checked step of shalom is recall at index 2.
		t.Fatalf("intro rule: %+v", out)
	}
	if e.briefUsed(apBriefA) != 0 || e.audits() != 0 {
		t.Fatal("a blocked publish spent the brief")
	}
	good := apDoc("Cafe")
	e.audio(good)
	fixed := e.put(db.LearnerScope(e.a), &out.UnitID, good, apBriefA, true)
	if !fixed.Published || fixed.UnitID != out.UnitID || fixed.Revision != 2 {
		t.Fatalf("fixed: %+v (blocks %v)", fixed, apBlocks(fixed.Issues))
	}
}

// TestAutoPublishMarkupInEveryField: markup in any text field blocks the publish.
func TestAutoPublishMarkupInEveryField(t *testing.T) {
	for _, c := range []struct {
		name    string
		mutate  func(*units.Unit)
		pointer string
	}{
		{"title", func(u *units.Unit) { u.Title = "Cafe <b>morning</b>" }, "/title"},
		{"situation", func(u *units.Unit) { u.Situation = "javascript:alert(1)" }, "/situation"},
		{"concept", func(u *units.Unit) { u.Concept = "a > b" }, "/concept"},
		{"new word", func(u *units.Unit) { u.NewWords[1] = u.NewWords[1] + "<i>" }, "/newWords/1"},
		{"story text", func(u *units.Unit) { u.Lessons[0].Activities[7].Text += " <img src=x onerror=alert(1)>" }, "/lessons/0/activities/7/text"},
		{"checkpoint recall", func(u *units.Unit) { u.Lessons[1].Activities[0].Text = "JavaScript:" + u.Lessons[1].Activities[0].Text }, "/lessons/1/activities/0/text"},
		{"lesson kind", func(u *units.Unit) { u.Lessons[1].Kind = "checkpoint<script>" }, "/lessons/1/kind"},
		{"activity type", func(u *units.Unit) { u.Lessons[1].Activities[3].Type = "listen_pick>" }, "/lessons/1/activities/3/type"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := newAP(t)
			e.goodBrief(apBriefA, e.a)
			doc := apDoc("Cafe")
			c.mutate(&doc)
			e.audio(doc)
			out := e.put(db.LearnerScope(e.a), nil, doc, apBriefA, true)
			if out.Published || e.status(out.UnitID) != "draft" {
				t.Fatalf("markup was published: %+v", out)
			}
			if !apHas(out.Issues, units.RuleUnitNoMarkup, c.pointer) {
				t.Fatalf("no unit.no_markup at %s: %+v", c.pointer, out.Issues)
			}
			if e.briefUsed(apBriefA) != 0 || e.audits() != 0 {
				t.Fatal("markup spent the brief or wrote an audit row")
			}
		})
	}
}

type apValidator struct{ p mcp.Principal }

func (v apValidator) Validate(context.Context, string, *http.Request) (mcp.Principal, error) {
	return v.p, nil
}

// TestAutoPublishLearnerCannotCallPutUnit: a learner session cannot publish, for
// themselves or for anyone else. put_unit is admin only, so the scope and the brief
// of a learner publish are only ever set by an admin client.
func TestAutoPublishLearnerCannotCallPutUnit(t *testing.T) {
	e := newAP(t)
	doc := apDoc("Cafe")
	e.audio(doc)
	e.goodBrief(apBriefA, e.a)
	learner := e.connect(mcp.Options{
		Course: e.adapter, Lexicon: e.adapter, Lookup: e.adapter, Units: e.adapter,
		Validator: apValidator{p: mcp.Principal{Actor: "7", UserID: e.a, Role: mcp.RoleLearner, Source: "oauth"}},
	}, "any")
	for _, scope := range []string{db.LearnerScope(e.a), db.LearnerScope(e.b)} {
		res, err := learner.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: "put_unit", Arguments: map[string]any{
			"scope": scope, "position": 0, "doc": doc, "publish": true, "briefId": apBriefA,
		}})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError || !strings.Contains(apText(res), "forbidden: admin only") {
			t.Fatalf("scope %s: %v %s", scope, res.IsError, apText(res))
		}
	}
	if e.count(`SELECT COUNT(*) FROM units`) != 0 || e.audits() != 0 || e.briefUsed(apBriefA) != 0 {
		t.Fatal("a learner session wrote something")
	}
}

// TestAutoPublishSameBriefConcurrently runs put_unit publish true for many units with
// one brief at once, through the handler. Exactly one is published.
func TestAutoPublishSameBriefConcurrently(t *testing.T) {
	e := newAP(t)
	e.goodBrief(apBriefA, e.a)
	const n = 12
	docs := make([]units.Unit, n)
	for i := range docs {
		docs[i] = apDoc(fmt.Sprintf("Cafe %d", i))
		e.audio(docs[i])
	}
	outs := make([]apPut, n)
	var ready, wg sync.WaitGroup
	start := make(chan struct{})
	ready.Add(n)
	wg.Add(n)
	for i := range docs {
		go func() {
			defer wg.Done()
			ready.Done()
			<-start
			outs[i] = apCall(t, e.session, map[string]any{
				"scope": db.LearnerScope(e.a), "position": i, "doc": docs[i], "publish": true, "briefId": apBriefA,
			})
		}()
	}
	ready.Wait()
	close(start)
	wg.Wait()

	published, blockedByUse := 0, 0
	var winner int64
	for _, out := range outs {
		if out.Published {
			published++
			winner = out.UnitID
			continue
		}
		if apHas(out.Issues, units.RuleBriefUsed, "/briefId") {
			blockedByUse++
		}
		if e.status(out.UnitID) != "draft" {
			t.Fatalf("loser %d status %s", out.UnitID, e.status(out.UnitID))
		}
	}
	if published != 1 || blockedByUse != n-1 {
		t.Fatalf("published %d, blocked by brief.used %d, want 1 and %d", published, blockedByUse, n-1)
	}
	if e.audits() != 1 || int64(e.briefUsed(apBriefA)) != winner || e.count(`SELECT COUNT(*) FROM units WHERE status = 'published'`) != 1 {
		t.Fatalf("audits %d, used by %d, winner %d", e.audits(), e.briefUsed(apBriefA), winner)
	}
}

// know makes a word known to a learner in listen: three right answers, and a review
// card with three Good ratings, which is what learner.Model reads as known.
func (e *apEnv) know(user int64, text, gloss string) {
	e.t.Helper()
	e.exec(`INSERT INTO lexemes (text_nfc, bare, gloss_en, pos, gender) VALUES (?, ?, ?, 'noun', 'm')`, text, units.StripNiqqud(text), gloss)
	id := int64(e.count(`SELECT id FROM lexemes WHERE text_nfc = ?`, text))
	for i := range 3 {
		at := apNow.Add(-2 * time.Hour).Add(-time.Duration(2-i) * time.Minute)
		e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, 'listen', 'test', 'review', '', '', 1, 0, 0, 0, ?)`, user, id, db.FormatTime(at))
	}
	last := apNow.AddDate(0, 0, -1)
	e.exec(`INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses, last_review, suspended)
		VALUES (?, ?, 'listen', ?, ?, 10, 5, 3, 0, ?, 0)`, user, id, fsrs.StateReview, db.FormatTime(apNow), db.FormatTime(last))
	card := e.count(`SELECT id FROM cards WHERE user_id = ? AND lexeme_id = ?`, user, id)
	for i := range 3 {
		e.exec(`INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days) VALUES (?, 3, ?, 0, 1)`,
			card, db.FormatTime(last.Add(-time.Duration(2-i)*24*time.Hour)))
	}
	e.ids[text] = id
}

// TestAutoPublishCountsTheLearnersKnownWords: a passage made of words the learner
// knows, and that the brief does not list, passes brief.coverage only for the learner
// who knows them. The check reads the learner's model, not the earlier shared units.
func TestAutoPublishCountsTheLearnersKnownWords(t *testing.T) {
	e := newAP(t)
	lechem := units.Normalize("\u05DC\u05B6\u05D7\u05B6\u05DD")
	e.know(e.b, lechem, "bread")
	e.goodBrief(apBriefA, e.a)
	e.goodBrief(apBriefB, e.b)
	doc := apDoc("Cafe")
	doc.Lessons[1].Activities = append(doc.Lessons[1].Activities,
		units.Activity{Type: units.ActivityListenPick, Text: lechem + " " + lechem + " " + lechem + " " + lechem, Target: lechem, Translation: "bread"})
	doc = withSentenceTranslations(doc)
	e.audio(doc)

	// Learner A does not know the word: the passage is all unknown, and the word has a
	// gloss in the lexicon, so coverage is the one block.
	blocked := e.put(db.LearnerScope(e.a), nil, doc, apBriefA, true)
	if blocked.Published || !apHas(blocked.Issues, units.RuleBriefCoverage, "/lessons/1/activities/6") {
		t.Fatalf("learner A: %+v", blocked)
	}
	if got := apBlocks(blocked.Issues); len(got) != 1 {
		t.Fatalf("blocks for learner A: %v", got)
	}
	// Learner B knows it, so the same document publishes.
	ok := e.put(db.LearnerScope(e.b), nil, doc, apBriefB, true)
	if !ok.Published {
		t.Fatalf("learner B: %+v (blocks %v)", ok, apBlocks(ok.Issues))
	}
}

// TestAutoPublishCannotBypassBriefViaPublishUnitTool: publish_unit runs the v2 rules only, so it must
// not be a way to put a learner's unit on Today without the brief checks.
func TestAutoPublishCannotBypassBriefViaPublishUnitTool(t *testing.T) {
	e := newAP(t)
	doc := apDoc("Cafe")
	e.audio(doc)
	draft := e.put(db.LearnerScope(e.a), nil, doc, "", true)
	if draft.Published {
		t.Fatalf("setup: %+v", draft)
	}
	res, err := e.session.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: "publish_unit", Arguments: map[string]any{"unitId": draft.UnitID}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || !strings.Contains(apText(res), "put_unit") || e.status(draft.UnitID) != "draft" || e.audits() != 0 {
		t.Fatalf("publish_unit on a learner unit: error %v %q status %s", res.IsError, apText(res), e.status(draft.UnitID))
	}
}
