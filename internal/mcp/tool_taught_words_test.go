package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// taughtAP is newAP with Denver as the learner zone and a fixed MCP clock (apNow).
func taughtAP(t *testing.T) *apEnv {
	t.Helper()
	e := newAP(t)
	app, err := studyday.AppZone("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := studyday.NewResolver(e.adapter, app)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetUserTimeZone(context.Background(), e.a, "America/Denver"); err != nil {
		t.Fatal(err)
	}
	e.taughtOpts = mcp.Options{
		Validator: sqlTokens{a: e.a, b: e.b},
		Course:    e.adapter, Lexicon: e.adapter, Lexemes: e.adapter, Lookup: e.adapter, Units: e.adapter,
		StudyDay: resolver,
		Now:      func() time.Time { return apNow },
	}
	return e
}

func (e *apEnv) asTaught(token string) *sdkmcp.ClientSession {
	e.t.Helper()
	opts := e.taughtOpts
	if opts.Course == nil {
		opts = mcp.Options{
			Validator: sqlTokens{a: e.a, b: e.b},
			Course:    e.adapter, Lexicon: e.adapter, Lexemes: e.adapter, Lookup: e.adapter, Units: e.adapter,
			Now: func() time.Time { return apNow },
		}
	}
	return e.connect(opts, token)
}

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := studyday.ParseZone(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// ac-1: omit on, two known he + one unknown → new cards at next 4:00 Denver; unknown listed.
func TestTaughtWordsCreatesCardsAndListsUnknown(t *testing.T) {
	e := taughtAP(t)
	learner := e.asTaught("learner-a")
	shalom, mayim := e.ids[apShalom], e.ids[apMayim]
	raw := mustOK(t, learner, "record_taught_words", map[string]any{
		"words": []any{
			map[string]any{"he": apShalom},
			map[string]any{"he": apMayim},
			map[string]any{"he": "זזזזז"},
		},
	})
	var out struct {
		Words []struct {
			LexemeID int64    `json:"lexemeId"`
			Created  []string `json:"created"`
			HadCard  []string `json:"hadCard"`
		} `json:"words"`
		Unknown []string  `json:"unknown"`
		Due     time.Time `json:"due"`
		Zone    string    `json:"zone"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	if out.Zone != "America/Denver" {
		t.Fatalf("zone = %q", out.Zone)
	}
	wantDue := studyday.DueCutoff(apNow, studyday.Zone{Name: "America/Denver", Loc: mustLoc(t, "America/Denver")})
	if !out.Due.Equal(wantDue) {
		t.Fatalf("due = %v, want %v", out.Due, wantDue)
	}
	if len(out.Unknown) != 1 || units.Normalize(out.Unknown[0]) != units.Normalize("זזזזז") {
		t.Fatalf("unknown = %#v", out.Unknown)
	}
	if len(out.Words) != 2 {
		t.Fatalf("words = %#v", out.Words)
	}
	gotIDs := map[int64][]string{}
	for _, w := range out.Words {
		gotIDs[w.LexemeID] = w.Created
		if len(w.HadCard) != 0 {
			t.Fatalf("hadCard = %#v", w)
		}
	}
	for _, id := range []int64{shalom, mayim} {
		if strings.Join(gotIDs[id], ",") != "read,recall" {
			t.Fatalf("lexeme %d created = %v", id, gotIDs[id])
		}
		for _, skill := range []string{"read", "recall"} {
			if n := e.count(`SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = ? AND state = 'new'`, e.a, id, skill); n != 1 {
				t.Fatalf("card %d/%s count = %d", id, skill, n)
			}
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM taught_words WHERE learner_id = ?`, e.a); n != 2 {
		t.Fatalf("taught_words = %d", n)
	}
}

// ac-2: other learnerId refused; readonly refused on all four tools.
func TestTaughtAndCoachWriteSecurity(t *testing.T) {
	e := taughtAP(t)
	learner := e.asTaught("learner-a")
	other := e.b
	for _, tool := range []string{"record_taught_words", "remove_taught_words", "record_mistakes", "add_coach_note"} {
		args := map[string]any{"learnerId": other}
		switch tool {
		case "record_taught_words":
			args["words"] = []any{map[string]any{"he": apShalom}}
		case "remove_taught_words":
			args["on"] = "2026-10-19"
			args["lexemeIds"] = []any{e.ids[apShalom]}
		case "record_mistakes":
			args["mistakes"] = []any{map[string]any{"wrong": "א", "right": "ב"}}
		case "add_coach_note":
			args["body"] = "x"
		}
		if msg := mustFail(t, learner, tool, args); msg != "learnerId must be your own id." {
			t.Fatalf("%s other id: %q", tool, msg)
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM taught_words`); n != 0 {
		t.Fatalf("taught wrote on refuse: %d", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM learner_mistakes`); n != 0 {
		t.Fatalf("mistakes wrote on refuse: %d", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM coach_notes`); n != 0 {
		t.Fatalf("notes wrote on refuse: %d", n)
	}

	ro := e.connect(mcp.Options{
		AuthToken:         "secret",
		ReadOnlyToken:     "readonly",
		ReadOnlyLearnerID: e.a,
		Course:            e.adapter, Lexicon: e.adapter, Lexemes: e.adapter, Lookup: e.adapter, Units: e.adapter,
		StudyDay: e.taughtOpts.StudyDay,
		Now:      e.taughtOpts.Now,
	}, "readonly")
	for _, tool := range []string{"record_taught_words", "remove_taught_words", "record_mistakes", "add_coach_note"} {
		args := map[string]any{}
		switch tool {
		case "record_taught_words":
			args["words"] = []any{map[string]any{"he": apShalom}}
		case "remove_taught_words":
			args["on"] = "2026-10-19"
			args["lexemeIds"] = []any{e.ids[apShalom]}
		case "record_mistakes":
			args["mistakes"] = []any{map[string]any{"wrong": "א", "right": "ב"}}
		case "add_coach_note":
			args["body"] = "x"
		}
		if msg := mustFail(t, ro, tool, args); msg != "forbidden: admin only" {
			t.Fatalf("readonly %s: %q", tool, msg)
		}
	}
}

// ac-3: 51 words, on tomorrow, on 31 days back → tool errors, nothing written.
func TestTaughtWordsRejectsBadBatchAndOn(t *testing.T) {
	e := taughtAP(t)
	learner := e.asTaught("learner-a")
	words := make([]any, 51)
	for i := range words {
		words[i] = map[string]any{"he": apShalom}
	}
	mustFail(t, learner, "record_taught_words", map[string]any{"words": words})
	z := studyday.Zone{Name: "America/Denver", Loc: mustLoc(t, "America/Denver")}
	today := studyday.Today(apNow, z)
	todayT, _ := time.ParseInLocation(time.DateOnly, today, z.Loc)
	tomorrow := todayT.AddDate(0, 0, 1).Format(time.DateOnly)
	old := todayT.AddDate(0, 0, -31).Format(time.DateOnly)
	for _, on := range []string{tomorrow, old} {
		mustFail(t, learner, "record_taught_words", map[string]any{
			"on": on, "words": []any{map[string]any{"he": apShalom}},
		})
	}
	if n := e.count(`SELECT COUNT(*) FROM taught_words`); n != 0 {
		t.Fatalf("taught_words = %d after refuses", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM cards WHERE user_id = ?`, e.a); n != 0 {
		t.Fatalf("cards = %d after refuses", n)
	}
}

// ac-4: remove_taught_words keeps a reviewed card and removes the rest.
func TestTaughtWordsRemoveKeepsReviewed(t *testing.T) {
	e := taughtAP(t)
	learner := e.asTaught("learner-a")
	lex := e.ids[apShalom]
	on := studyday.Today(apNow, studyday.Zone{Name: "America/Denver", Loc: mustLoc(t, "America/Denver")})
	mustOK(t, learner, "record_taught_words", map[string]any{
		"on": on, "words": []any{map[string]any{"lexemeId": lex}},
	})
	ctx := context.Background()
	cardID, ok, err := e.store.CardID(ctx, e.a, lex, "recall")
	if err != nil || !ok {
		t.Fatalf("recall card: %v ok=%v", err, ok)
	}
	card, err := e.store.GetCard(ctx, cardID)
	if err != nil {
		t.Fatal(err)
	}
	card.State = "review"
	card.Reps = 1
	card.Stability = 1.2
	card.Difficulty = 5
	card.LastReview = apNow
	card.Due = apNow.Add(24 * time.Hour)
	if err := e.store.ApplyReview(ctx, card, db.ReviewLog{
		CardID: card.ID, Rating: 3, ReviewedAt: apNow, ScheduledDays: 1,
	}); err != nil {
		t.Fatal(err)
	}

	raw := mustOK(t, learner, "remove_taught_words", map[string]any{
		"on": on, "lexemeIds": []any{lex},
	})
	var out struct {
		Words []struct {
			LexemeID int64    `json:"lexemeId"`
			Removed  []string `json:"removed"`
			Kept     []string `json:"kept"`
		} `json:"words"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Words) != 1 {
		t.Fatalf("words = %#v", out.Words)
	}
	w := out.Words[0]
	if strings.Join(w.Kept, ",") != "recall" {
		t.Fatalf("kept = %v, want recall", w.Kept)
	}
	if strings.Join(w.Removed, ",") != "read" {
		t.Fatalf("removed = %v, want read", w.Removed)
	}
	if n := e.count(`SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = 'recall'`, e.a, lex); n != 1 {
		t.Fatalf("reviewed recall card deleted")
	}
	if n := e.count(`SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = 'read'`, e.a, lex); n != 0 {
		t.Fatalf("read card still present")
	}
	if n := e.count(`SELECT COUNT(*) FROM taught_words WHERE learner_id = ? AND lexeme_id = ?`, e.a, lex); n != 0 {
		t.Fatalf("taught row remains")
	}
}

// ac-5: record_mistakes with on stores last_on; without on uses today.
func TestMistakeOnDefaultsAndStores(t *testing.T) {
	e := taughtAP(t)
	learner := e.asTaught("learner-a")
	mustOK(t, learner, "record_mistakes", map[string]any{
		"on": "2026-10-18",
		"mistakes": []any{
			map[string]any{"wrong": "אֲנִי הוֹלֵךְ", "right": "אֲנִי הוֹלֶכֶת"},
		},
	})
	if got := e.text(`SELECT last_on FROM learner_mistakes WHERE learner_id = ?`, e.a); got != "2026-10-18" {
		t.Fatalf("last_on = %q", got)
	}
	mustOK(t, learner, "record_mistakes", map[string]any{
		"mistakes": []any{
			map[string]any{"wrong": "שָׁלוֹם", "right": "בֹּקֶר"},
		},
	})
	today := studyday.Today(apNow, studyday.Zone{Name: "America/Denver", Loc: mustLoc(t, "America/Denver")})
	if got := e.text(`SELECT last_on FROM learner_mistakes WHERE learner_id = ? AND wrong_he = ?`, e.a, units.Normalize("שָׁלוֹם")); got != today {
		t.Fatalf("default last_on = %q, want %s", got, today)
	}
}

// ac-6 + ac-7: learner note role, cap 20, brief quote + isolation, script stored raw / JSON-escaped.
func TestCoachNoteLearnerRoleCapAndBrief(t *testing.T) {
	e := taughtAP(t)
	learner := e.asTaught("learner-a")
	other := e.asTaught("learner-b")
	admin := e.asTaught("static")

	for i := 0; i < db.MaxLearnerCoachNotesPerDay-1; i++ {
		mustOK(t, learner, "add_coach_note", map[string]any{"body": fmt.Sprintf("n%d", i)})
	}
	body := `<script>alert(1)</script>`
	mustOK(t, learner, "add_coach_note", map[string]any{"body": body})
	if role := e.text(`SELECT author_role FROM coach_notes WHERE user_id = ? AND body = ?`, e.a, body); role != "learner" {
		t.Fatalf("role = %q", role)
	}
	stored := e.text(`SELECT body FROM coach_notes WHERE user_id = ? AND body = ?`, e.a, body)
	if stored != body {
		t.Fatalf("stored body = %q", stored)
	}
	if msg := mustFail(t, learner, "add_coach_note", map[string]any{"body": "one too many"}); msg != mcp.ErrTooManyNotes.Error() {
		t.Fatalf("21st note: %q", msg)
	}

	briefRaw := mustOK(t, admin, "get_lesson_brief", map[string]any{"learnerId": e.a})
	if !strings.Contains(briefRaw, `"authorRole":"learner"`) {
		t.Fatalf("brief missing authorRole: %s", briefRaw)
	}
	if !strings.Contains(briefRaw, `The learner wrote:`) {
		t.Fatalf("brief missing quoted learner line: %s", briefRaw)
	}
	if strings.Contains(briefRaw, `<script>`) {
		t.Fatalf("brief JSON did not escape script: %s", briefRaw)
	}
	if !strings.Contains(briefRaw, "alert(1)") {
		t.Fatalf("brief lost the note body: %s", briefRaw)
	}

	briefB := mustOK(t, admin, "get_lesson_brief", map[string]any{"learnerId": e.b})
	if strings.Contains(briefB, body) || strings.Contains(briefB, "alert(1)") || strings.Contains(briefB, "The learner wrote:") {
		t.Fatalf("learner B brief holds A's note: %s", briefB)
	}
	_ = other
}
