package mcp_test

// Tests of the learner tools on the real store (INIT-015/SPEC-021): the same handler and the
// same SQLite file the app uses, with two learners, so a read that crosses learners shows.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// sqlTokens is the static token (an admin with no learner id) and one OAuth-style session
// for each of the two learners. The actor of a session is its user id, as the app sets it.
type sqlTokens struct{ a, b int64 }

func (s sqlTokens) Validate(_ context.Context, token string, _ *http.Request) (mcp.Principal, error) {
	switch token {
	case "static":
		return mcp.Principal{Actor: "mcp", Role: mcp.RoleAdmin, Source: "token"}, nil
	case "learner-a":
		return mcp.Principal{Actor: strconv.FormatInt(s.a, 10), UserID: s.a, Role: mcp.RoleLearner, Source: "oauth"}, nil
	case "learner-b":
		return mcp.Principal{Actor: strconv.FormatInt(s.b, 10), UserID: s.b, Role: mcp.RoleLearner, Source: "oauth"}, nil
	}
	return mcp.Principal{}, mcp.ErrUnauthorized
}

// as opens a session for the token, over the real store. The audio generator may be nil.
func (e *apEnv) as(token string, gen mcp.AudioGenerator) *sdkmcp.ClientSession {
	e.t.Helper()
	return e.connect(mcp.Options{
		Validator: sqlTokens{a: e.a, b: e.b},
		Course:    e.adapter, Lexicon: e.adapter, Lexemes: e.adapter, Lookup: e.adapter, Units: e.adapter,
		Audio: gen,
	}, token)
}

// toolJSON calls a tool and returns the structured result as JSON, or the error text.
func toolJSON(t testing.TB, s *sdkmcp.ClientSession, tool string, args any) (text string, isErr bool) {
	t.Helper()
	res, err := s.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatalf("%s protocol error: %v", tool, err)
	}
	if res.IsError {
		return apText(res), true
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), false
}

func mustOK(t testing.TB, s *sdkmcp.ClientSession, tool string, args any) string {
	t.Helper()
	text, isErr := toolJSON(t, s, tool, args)
	if isErr {
		t.Fatalf("%s: tool error %s", tool, text)
	}
	return text
}

func mustFail(t testing.TB, s *sdkmcp.ClientSession, tool string, args any) string {
	t.Helper()
	text, isErr := toolJSON(t, s, tool, args)
	if !isErr {
		t.Fatalf("%s: want a tool error, got %s", tool, text)
	}
	return text
}

func decodeInto[T any](t testing.TB, raw string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return v
}

// attempts adds answers for a learner, oldest first, R right and W wrong, a minute apart,
// ending an hour before apNow.
func (e *apEnv) attempts(user int64, text, skill, history string) {
	e.t.Helper()
	for i := range len(history) {
		at := apNow.Add(-time.Hour).Add(-time.Duration(len(history)-1-i) * time.Minute)
		correct := 0
		if history[i] == 'R' {
			correct = 1
		}
		e.exec(`INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', ?, 0, 0, 0, ?)`, user, e.ids[text], skill, correct, db.FormatTime(at))
	}
}

// ---- get_learner_model on SQLite ----------------------------------------------------

// ac-2 on the real model: 450 more words, a limit of 5000, and the cursor to the end.
func TestLearnerModelPagesOverSQLite(t *testing.T) {
	e := newAP(t)
	for i := range 450 {
		e.exec(`INSERT INTO lexemes (text_nfc, bare, gloss_en, pos) VALUES (?, ?, 'g', 'noun')`, fmt.Sprintf("מ%03d", i), fmt.Sprintf("מ%03d", i))
	}
	total := e.count(`SELECT COUNT(*) FROM lexemes`)
	admin := e.as("static", nil)

	type page struct {
		Words []struct {
			Text string `json:"text"`
		} `json:"words"`
		NextCursor *string `json:"nextCursor"`
		Limit      int     `json:"limit"`
	}
	var sizes []int
	seen := map[string]bool{}
	args := map[string]any{"learnerId": e.a, "limit": 5000}
	for range 6 {
		p := decodeInto[page](t, mustOK(t, admin, "get_learner_model", args))
		if p.Limit != 200 {
			t.Fatalf("limit reported %d", p.Limit)
		}
		sizes = append(sizes, len(p.Words))
		for _, w := range p.Words {
			if seen[w.Text] {
				t.Fatalf("%s came twice", w.Text)
			}
			seen[w.Text] = true
		}
		if p.NextCursor == nil {
			break
		}
		args = map[string]any{"learnerId": e.a, "limit": 5000, "cursor": *p.NextCursor}
	}
	if len(seen) != total || sizes[0] != 200 || sizes[1] != 200 || sizes[2] != total-400 {
		t.Fatalf("pages %v, %d of %d words", sizes, len(seen), total)
	}
}

// A learner's words come from that learner's evidence only.
func TestLearnerModelIsScopedOverSQLite(t *testing.T) {
	e := newAP(t)
	e.attempts(e.a, apMayim, "listen", "RRWWWWWW") // struggling for A
	e.attempts(e.b, apShalom, "listen", "RRWWWWWW")
	e.attempts(e.b, apBoker, "listen", "RRWWWWWW") // B has two struggling words, so the summaries differ
	a, b := e.as("learner-a", nil), e.as("learner-b", nil)

	type word struct {
		Text     string `json:"text"`
		State    string `json:"state"`
		Attempts int    `json:"attempts"`
		LastSeen string `json:"lastSeen"`
		Skills   []struct {
			Skill string `json:"skill"`
			State string `json:"state"`
		} `json:"skills"`
	}
	read := func(s *sdkmcp.ClientSession, args map[string]any) []word {
		return decodeInto[struct{ Words []word }](t, mustOK(t, s, "get_learner_model", args)).Words
	}
	ownA := read(a, map[string]any{"state": "struggling"})
	ownB := read(b, map[string]any{"state": "struggling"})
	if len(ownA) != 1 || ownA[0].Text != apMayim || ownA[0].Attempts != 8 || ownA[0].LastSeen == "" || len(ownA[0].Skills) != 4 {
		t.Fatalf("learner a: %+v", ownA)
	}
	if len(ownB) != 2 || ownB[0].Text != apShalom || ownB[1].Text != apBoker {
		t.Fatalf("learner b: %+v", ownB)
	}
	for _, w := range ownA {
		if w.Text == apShalom || w.Text == apBoker {
			t.Fatalf("learner a holds learner b's word %s", w.Text)
		}
	}
	if s := mustFail(t, a, "get_learner_model", map[string]any{"learnerId": e.b}); s != "learnerId must be your own id." || strings.Contains(s, apShalom) {
		t.Fatalf("a naming b: %q", s)
	}
	// The summary in get_context is the same learner's.
	ctxA := mustOK(t, a, "get_context", map[string]any{"position": 0})
	if m := decodeInto[struct {
		Model struct{ Words map[string]int } `json:"model"`
	}](t, ctxA).Model; m.Words["struggling"] != 1 {
		t.Fatalf("a's summary %s", ctxA)
	}
}

// ---- add_coach_note on SQLite --------------------------------------------------------

func TestCoachNotesOverSQLite(t *testing.T) {
	e := newAP(t)
	admin := e.as("static", nil)
	a, b := e.as("learner-a", nil), e.as("learner-b", nil)

	out := decodeInto[struct {
		NoteID    int64     `json:"noteId"`
		CreatedAt time.Time `json:"createdAt"`
	}](t, mustOK(t, admin, "add_coach_note", map[string]any{"learnerId": e.a, "body": "ח and כ keep coming up"}))
	if out.NoteID == 0 || !out.CreatedAt.Equal(apNow) {
		t.Fatalf("result %+v", out)
	}
	// The row is the audit of the note: who wrote it, for whom, and when.
	if n := e.count(`SELECT COUNT(*) FROM coach_notes WHERE id = ? AND user_id = ? AND author = 'mcp' AND body = 'ח and כ keep coming up' AND created_at = ?`,
		out.NoteID, e.a, db.FormatTime(apNow)); n != 1 {
		t.Fatalf("no row names the author, learner, body, and time")
	}
	if n := e.count(`SELECT COUNT(*) FROM coach_notes WHERE user_id = ?`, e.b); n != 0 {
		t.Fatalf("learner b has %d notes", n)
	}
	mustOK(t, admin, "add_coach_note", map[string]any{"learnerId": e.b, "body": "BNOTE"})

	type notes struct {
		CoachNotes []struct {
			ID     int64  `json:"id"`
			Author string `json:"author"`
			Body   string `json:"body"`
		} `json:"coachNotes"`
	}
	ctxA := decodeInto[notes](t, mustOK(t, a, "get_context", map[string]any{"position": 0}))
	ctxB := decodeInto[notes](t, mustOK(t, b, "get_context", map[string]any{"position": 0}))
	if len(ctxA.CoachNotes) != 1 || ctxA.CoachNotes[0].Body != "ח and כ keep coming up" || ctxA.CoachNotes[0].ID != out.NoteID {
		t.Fatalf("a: %+v", ctxA)
	}
	if len(ctxB.CoachNotes) != 1 || ctxB.CoachNotes[0].Body != "BNOTE" {
		t.Fatalf("b: %+v", ctxB)
	}
	// A learner cannot write a note, not even for itself.
	for _, id := range []int64{e.a, e.b} {
		if s := mustFail(t, a, "add_coach_note", map[string]any{"learnerId": id, "body": "x"}); s != "forbidden: admin only" {
			t.Fatalf("learner a writing for %d: %q", id, s)
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM coach_notes`); n != 2 {
		t.Fatalf("%d notes", n)
	}
	// A refused note leaves no row, and a note of 500 Hebrew letters is stored whole.
	mustFail(t, admin, "add_coach_note", map[string]any{"learnerId": e.a, "body": strings.Repeat("ת", 501)})
	mustOK(t, admin, "add_coach_note", map[string]any{"learnerId": e.a, "body": strings.Repeat("ת", 500)})
	if n := e.count(`SELECT COUNT(*) FROM coach_notes WHERE user_id = ?`, e.a); n != 2 {
		t.Fatalf("a has %d notes", n)
	}
	// Notes come back newest first, at most 5.
	for i := range 6 {
		mustOK(t, admin, "add_coach_note", map[string]any{"learnerId": e.a, "body": fmt.Sprintf("later %d", i)})
	}
	got := decodeInto[notes](t, mustOK(t, a, "get_context", map[string]any{"position": 0}))
	if len(got.CoachNotes) != 5 || got.CoachNotes[0].Body != "later 5" {
		t.Fatalf("a after 9 notes: %+v", got)
	}
}

// The limit of notes in a day holds in both stores, and is per learner.
func TestCoachNoteDailyCap(t *testing.T) {
	e := newAP(t)
	ctx := t.Context()
	for i := range db.MaxCoachNotesPerDay {
		if _, err := e.adapter.AddCoachNote(ctx, e.a, "mcp", fmt.Sprintf("n%d", i)); err != nil {
			t.Fatalf("note %d: %v", i, err)
		}
	}
	if _, err := e.adapter.AddCoachNote(ctx, e.a, "mcp", "one too many"); err != mcp.ErrTooManyNotes {
		t.Fatalf("note 101: %v", err)
	}
	if n := e.count(`SELECT COUNT(*) FROM coach_notes WHERE user_id = ?`, e.a); n != db.MaxCoachNotesPerDay {
		t.Fatalf("%d notes", n)
	}
	if _, err := e.adapter.AddCoachNote(ctx, e.b, "mcp", "fine"); err != nil {
		t.Fatalf("learner b: %v", err)
	}
	mem := mcp.NewMemory()
	for i := range db.MaxCoachNotesPerDay {
		if _, err := mem.AddCoachNote(ctx, 1, "mcp", fmt.Sprintf("n%d", i)); err != nil {
			t.Fatalf("memory note %d: %v", i, err)
		}
	}
	if _, err := mem.AddCoachNote(ctx, 1, "mcp", "one too many"); err != mcp.ErrTooManyNotes {
		t.Fatalf("memory note 101: %v", err)
	}
	// The tool turns it into a fixed sentence.
	admin := e.as("static", nil)
	if s := mustFail(t, admin, "add_coach_note", map[string]any{"learnerId": e.a, "body": "x"}); s != mcp.ErrTooManyNotes.Error() {
		t.Fatalf("tool text %q", s)
	}
}

// ---- get_progress on SQLite ----------------------------------------------------------

func (e *apEnv) snapshot(user int64, day, skill, state string, count int, est float64, retention any) {
	e.exec(`INSERT INTO daily_snapshots (user_id, day, skill, state, count, est_known, retention) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		user, day, skill, state, count, est, retention)
}

func TestProgressOverSQLite(t *testing.T) {
	e := newAP(t)
	// Learner A: a snapshot on two days, two events, a report, and a placement.
	e.snapshot(e.a, "2026-10-10", "listen", "known", 3, 10, 0.9)
	e.snapshot(e.a, "2026-10-10", "listen", "struggling", 1, 10, 0.9)
	e.snapshot(e.a, "2026-10-17", "listen", "known", 5, 12, 0.8)
	e.snapshot(e.a, "2026-10-17", "listen", "struggling", 0, 12, 0.8)
	e.snapshot(e.a, "2026-10-17", "read", "known", 1, 2, nil)
	e.exec(`INSERT INTO regression_events (user_id, lexeme_id, skill, from_state, to_state, day, cause, kind) VALUES (?, ?, 'listen', 'known', 'struggling', '2026-10-15', 'decay', 'word')`, e.a, e.ids[apMayim])
	e.exec(`INSERT INTO regression_events (user_id, day, cause, kind) VALUES (?, '2026-10-16', 'band_slip band=1 dropped 25', 'band_slip')`, e.a)
	e.exec(`INSERT INTO weekly_reports (user_id, week_start, payload_json, summary, created_at) VALUES (?, '2026-10-12', '{"secret":"payload"}', 'A summary', ?)`, e.a, db.FormatTime(apNow))
	run, err := e.store.InsertPlacementRun(t.Context(), e.a, apNow.Add(-48*time.Hour), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.FinishPlacement(t.Context(), run, apNow.Add(-47*time.Hour), 2, "band_stop", `{"bands":[],"estimated_known_words":120}`); err != nil {
		t.Fatal(err)
	}
	e.exec(`INSERT INTO placement_answers (run_id, item_id, rating, shown_at, answered_at) VALUES (?, 'rated', 3, ?, ?)`, run, db.FormatTime(apNow), db.FormatTime(apNow))
	// Learner B has data that must never show in A's.
	e.snapshot(e.b, "2026-10-17", "listen", "known", 99, 987654, 0.1)
	e.exec(`INSERT INTO regression_events (user_id, lexeme_id, skill, from_state, to_state, day, cause, kind) VALUES (?, ?, 'read', 'solid', 'regressed', '2026-10-15', 'BSECRET', 'word')`, e.b, e.ids[apShalom])
	e.exec(`INSERT INTO weekly_reports (user_id, week_start, payload_json, summary, created_at) VALUES (?, '2026-10-12', '{}', 'BSUMMARY', ?)`, e.b, db.FormatTime(apNow))
	runB, err := e.store.InsertPlacementRun(t.Context(), e.b, apNow.Add(-48*time.Hour), 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.FinishPlacement(t.Context(), runB, apNow.Add(-47*time.Hour), 4, "band_stop", `{"bands":[],"estimated_known_words":424242}`); err != nil {
		t.Fatal(err)
	}
	e.exec(`INSERT INTO placement_answers (run_id, item_id, rating, shown_at, answered_at) VALUES (?, 'rated', 3, ?, ?)`, runB, db.FormatTime(apNow), db.FormatTime(apNow))

	type progress struct {
		From      string `json:"from"`
		To        string `json:"to"`
		Snapshots []struct {
			Day    string `json:"day"`
			Skills []struct {
				Skill     string   `json:"skill"`
				Retention *float64 `json:"retention"`
			} `json:"skills"`
		} `json:"snapshots"`
		Changes []struct {
			Skill  string `json:"skill"`
			Start  int    `json:"knownAtStart"`
			End    int    `json:"knownAtEnd"`
			Gained int    `json:"gained"`
			Lost   int    `json:"lost"`
		} `json:"changes"`
		Regressions []struct {
			Day, Kind, Text, Cause string
		} `json:"regressions"`
		Retention struct {
			Target float64  `json:"target"`
			Actual *float64 `json:"actual"`
			Day    string   `json:"day"`
			Gap    *float64 `json:"gap"`
		} `json:"retention"`
		Placements []struct {
			Kind     string `json:"kind"`
			Level    int    `json:"level"`
			Estimate int    `json:"estimatedKnownWords"`
		} `json:"placements"`
		WeeklyReports []struct {
			WeekStart string `json:"weekStart"`
			Summary   string `json:"summary"`
		} `json:"weeklyReports"`
	}
	admin, a, b := e.as("static", nil), e.as("learner-a", nil), e.as("learner-b", nil)
	rawA := mustOK(t, a, "get_progress", map[string]any{"from": "2026-10-01", "to": "2026-10-19"})
	for _, leak := range []string{"BSECRET", "BSUMMARY", "987654", "424242", `"payload"`, "secret"} {
		if strings.Contains(rawA, leak) {
			t.Fatalf("learner a's progress holds %q: %s", leak, rawA)
		}
	}
	p := decodeInto[progress](t, rawA)
	if len(p.Snapshots) != 2 || p.Snapshots[0].Day != "2026-10-10" || p.Snapshots[1].Day != "2026-10-17" || len(p.Snapshots[1].Skills) != 2 {
		t.Fatalf("snapshots %+v", p.Snapshots)
	}
	if len(p.Changes) != 1 || p.Changes[0].Skill != "listen" || p.Changes[0].Start != 3 || p.Changes[0].End != 5 || p.Changes[0].Gained != 2 || p.Changes[0].Lost != 0 {
		t.Fatalf("changes %+v", p.Changes)
	}
	if len(p.Regressions) != 2 || p.Regressions[0].Kind != "band_slip" || p.Regressions[0].Text != "" || p.Regressions[1].Text != apMayim {
		t.Fatalf("regressions %+v", p.Regressions)
	}
	if p.Retention.Target != 0.85 || p.Retention.Actual == nil || *p.Retention.Actual != 0.8 || p.Retention.Day != "2026-10-17" || p.Retention.Gap == nil || *p.Retention.Gap > -0.049 || *p.Retention.Gap < -0.051 {
		t.Fatalf("retention %+v", p.Retention)
	}
	if len(p.Placements) != 1 || p.Placements[0].Estimate != 120 || p.Placements[0].Level != 2 || p.Placements[0].Kind != "v2" {
		t.Fatalf("placements %+v", p.Placements)
	}
	if len(p.WeeklyReports) != 1 || p.WeeklyReports[0].Summary != "A summary" {
		t.Fatalf("reports %+v", p.WeeklyReports)
	}

	// The range cuts: 10-16 to 10-19 has the band slip and the 10-17 snapshot, and no report.
	cut := decodeInto[progress](t, mustOK(t, admin, "get_progress", map[string]any{"learnerId": e.a, "from": "2026-10-16", "to": "2026-10-19"}))
	if len(cut.Snapshots) != 1 || len(cut.Regressions) != 1 || len(cut.WeeklyReports) != 0 || cut.Changes[0].Gained != 0 {
		t.Fatalf("cut %+v", cut)
	}
	// B reads B's.
	rawB := mustOK(t, b, "get_progress", map[string]any{"from": "2026-10-01", "to": "2026-10-19"})
	if !strings.Contains(rawB, "BSECRET") || !strings.Contains(rawB, "BSUMMARY") || strings.Contains(rawB, "decay") || strings.Contains(rawB, "A summary") {
		t.Fatalf("learner b's progress: %s", rawB)
	}
	// And cannot name A.
	if s := mustFail(t, b, "get_progress", map[string]any{"learnerId": e.a}); strings.Contains(s, "decay") || s != "learnerId must be your own id." {
		t.Fatalf("b naming a: %q", s)
	}
	// A learner with no history gets empty lists.
	e2 := mustOK(t, admin, "get_progress", map[string]any{"learnerId": e.b, "from": "2025-01-01", "to": "2025-01-31"})
	if !strings.Contains(e2, `"snapshots":[]`) || !strings.Contains(e2, `"actual":null`) {
		t.Fatalf("empty %s", e2)
	}
}

// ---- brief scoping on SQLite ---------------------------------------------------------

// A brief is built from one learner's evidence and is read back only by that learner.
func TestBriefIsScopedOverSQLite(t *testing.T) {
	e := newAP(t)
	e.attempts(e.a, apMayim, "listen", "RRWWWWWW")
	e.attempts(e.b, apBoker, "listen", "RRWWWWWW") // B struggles with boker. A has never seen it.
	admin, a, b := e.as("static", nil), e.as("learner-a", nil), e.as("learner-b", nil)

	rawA := mustOK(t, admin, "get_lesson_brief", map[string]any{"learnerId": e.a})
	brief := decodeInto[mcp.LessonBrief](t, rawA)
	if len(brief.StruggleWords) != 1 || brief.StruggleWords[0].Text != apMayim || brief.BriefID == "" {
		t.Fatalf("a's brief %s", rawA)
	}
	for _, s := range brief.StruggleWords {
		if s.Text == apBoker {
			t.Fatal("B's struggle word is in A's struggle words")
		}
	}
	for _, banned := range []string{`"learnerId"`, `"userId"`, `"user_id"`, "example.com", "sub-a", "sub-b"} {
		if strings.Contains(rawA, banned) {
			t.Fatalf("brief holds %s: %s", banned, rawA)
		}
	}

	doc := apDoc("Cafe")
	rules := func(s *sdkmcp.ClientSession, id string) []string {
		var out struct{ Issues []units.Issue }
		in := map[string]any{"position": 0, "doc": doc, "briefId": id}
		if err := json.Unmarshal([]byte(mustOK(t, s, "validate_unit", in)), &out); err != nil {
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
	own := rules(a, brief.BriefID)
	for _, r := range own {
		if r == "brief.not_found /briefId" {
			t.Fatalf("a's own brief was not found")
		}
	}
	stolen := rules(b, brief.BriefID)
	missing := rules(b, "AAAAAAAAAAAAAAAAAAAAAA")
	malformed := rules(b, "not a brief id")
	if len(stolen) == 0 || fmt.Sprint(stolen) != fmt.Sprint(missing) || fmt.Sprint(stolen) != fmt.Sprint(malformed) {
		t.Fatalf("b with a's brief: %v; missing id: %v; malformed id: %v", stolen, missing, malformed)
	}
	found := false
	for _, r := range stolen {
		found = found || r == "brief.not_found /briefId"
	}
	if !found {
		t.Fatalf("no brief.not_found: %v", stolen)
	}
	// Publishing into B's scope with A's brief does nothing.
	e.audio(doc)
	out := e.put(db.LearnerScope(e.b), nil, doc, brief.BriefID, true)
	if out.Published || e.status(out.UnitID) != "draft" || e.audits() != 0 || e.briefUsed(brief.BriefID) != 0 {
		t.Fatalf("A's brief published a unit for B: %+v", out)
	}
}

// ---- unpublish_unit on SQLite (ac-4) -------------------------------------------------

func TestUnpublishOverSQLite(t *testing.T) {
	e := newAP(t)
	doc := apDoc("Cafe")
	e.audio(doc)
	e.goodBrief(apBriefA, e.a)
	e.goodBrief(apBriefB, e.b)
	mine := e.put(db.LearnerScope(e.a), nil, doc, apBriefA, true)
	theirsDoc := apDoc("Theirs")
	e.audio(theirsDoc)
	theirs := e.put(db.LearnerScope(e.b), nil, theirsDoc, apBriefB, true)
	if !mine.Published || !theirs.Published || e.audits() != 2 {
		t.Fatalf("setup: %+v %+v", mine, theirs)
	}
	shared, err := e.adapter.PutUnit(t.Context(), nil, &units.Unit{Title: "Shared", Situation: "s", Concept: "c"}, mcp.ScopeShared)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.adapter.PublishUnit(t.Context(), shared.UnitID, nil); err != nil {
		t.Fatal(err)
	}
	today := func(user int64) map[int64]string {
		rows, err := e.store.UnitsInScope(t.Context(), user)
		if err != nil {
			t.Fatal(err)
		}
		out := map[int64]string{}
		for _, r := range rows {
			if r.Status == "published" {
				out[r.ID] = r.Scope
			}
		}
		return out
	}
	if _, ok := today(e.a)[mine.UnitID]; !ok {
		t.Fatal("setup: the unit is not on A's Today")
	}
	a, b, admin := e.as("learner-a", nil), e.as("learner-b", nil), e.as("static", nil)

	// B cannot unpublish A's unit, a shared unit, or a missing one, and the three answers are the same.
	m1 := mustFail(t, b, "unpublish_unit", map[string]any{"unitId": mine.UnitID})
	m2 := mustFail(t, b, "unpublish_unit", map[string]any{"unitId": shared.UnitID})
	m3 := mustFail(t, b, "unpublish_unit", map[string]any{"unitId": 99999})
	if m1 != m2 || m2 != m3 || !strings.Contains(m1, "No learner-scoped unit") {
		t.Fatalf("refusals %q %q %q", m1, m2, m3)
	}
	if s := mustFail(t, b, "unpublish_unit", map[string]any{"unitId": mine.UnitID, "learnerId": e.a}); s != "learnerId must be your own id." {
		t.Fatalf("b naming a: %q", s)
	}
	if _, ok := today(e.a)[mine.UnitID]; !ok || e.audits() != 2 {
		t.Fatalf("a refused call changed a unit: audits %d", e.audits())
	}
	// An admin naming the wrong learner is refused too.
	if s := mustFail(t, admin, "unpublish_unit", map[string]any{"unitId": mine.UnitID, "learnerId": e.b}); s != m1 {
		t.Fatalf("admin for the wrong learner: %q", s)
	}
	if s := mustFail(t, admin, "unpublish_unit", map[string]any{"unitId": mine.UnitID}); !strings.Contains(s, "learnerId is required") {
		t.Fatalf("admin with no learner: %q", s)
	}
	if _, ok := today(e.a)[mine.UnitID]; !ok {
		t.Fatal("a refused admin call changed the unit")
	}

	// ac-4: A takes its own auto-published unit off Today, and an audit row says so.
	out := decodeInto[struct {
		UnitID    int64  `json:"unitId"`
		Revision  int    `json:"revision"`
		Status    string `json:"status"`
		Unchanged bool   `json:"unchanged"`
	}](t, mustOK(t, a, "unpublish_unit", map[string]any{"unitId": mine.UnitID}))
	if out.UnitID != mine.UnitID || out.Status != "draft" || out.Unchanged || out.Revision != mine.Revision {
		t.Fatalf("result %+v", out)
	}
	if _, ok := today(e.a)[mine.UnitID]; ok {
		t.Fatal("the unit is still on A's Today")
	}
	if e.status(mine.UnitID) != "draft" {
		t.Fatalf("status %s", e.status(mine.UnitID))
	}
	if n := e.count(`SELECT COUNT(*) FROM publish_audit WHERE unit_id = ? AND action = 'unpublish' AND actor = ? AND scope = ? AND revision = ?`,
		mine.UnitID, strconv.FormatInt(e.a, 10), db.LearnerScope(e.a), mine.Revision); n != 1 {
		t.Fatalf("no unpublish audit row names the unit, actor, scope, and revision")
	}
	if e.audits() != 3 {
		t.Fatalf("audit rows %d", e.audits())
	}
	// The other learner's unit and the shared unit are still on Today.
	if _, ok := today(e.b)[theirs.UnitID]; !ok {
		t.Fatal("B's unit was taken off")
	}
	if _, ok := today(e.b)[shared.UnitID]; !ok {
		t.Fatal("the shared unit was taken off")
	}
	// A repeat changes nothing and writes nothing.
	again := decodeInto[struct{ Unchanged bool }](t, mustOK(t, a, "unpublish_unit", map[string]any{"unitId": mine.UnitID}))
	if !again.Unchanged || e.audits() != 3 {
		t.Fatalf("repeat %+v audits %d", again, e.audits())
	}
	// The static token, naming the learner, can do the same for B's unit.
	mustOK(t, admin, "unpublish_unit", map[string]any{"unitId": theirs.UnitID, "learnerId": e.b})
	if _, ok := today(e.b)[theirs.UnitID]; ok || e.audits() != 4 {
		t.Fatalf("admin unpublish: audits %d", e.audits())
	}
	if n := e.count(`SELECT COUNT(*) FROM publish_audit WHERE unit_id = ? AND action = 'unpublish' AND actor = 'mcp'`, theirs.UnitID); n != 1 {
		t.Fatal("the admin's unpublish is not logged under its name")
	}
}
