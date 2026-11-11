package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/choices"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// Words shaped like unit 1 lesson 2. The feminine "you" shares a content
// word with "you (m.sg.)". "to live" shares one with "live, reside".
const (
	upAta   = "אַתָּה"
	upAt    = "אַתְּ"
	upGar   = "גָּר"
	upGara  = "גָּרָה"
	upPo    = "פֹּה"
	upMayim = "מַיִם"
	upEifo  = "אֵיפֹה"
	upOved  = "עוֹבֵד"
	upKotev = "כּוֹתֵב"
	upOkhel = "אוֹכֵל"
)

func upLine() string { return upAta + " " + upGar + " " + upPo + "?" }

func upGlosses() map[string]string {
	return map[string]string{
		upAta: "you (m.sg.)", upAt: "you (f.sg.)",
		upGar: "live, reside (I/he lives)", upGara: "to live",
		upPo: "here", upMayim: "water", upEifo: "where",
		upOved: "work(s)", upKotev: "writes", upOkhel: "eats",
	}
}

func upPOS(text string) string {
	switch text {
	case upAta, upAt:
		return "pron"
	case upGar, upGara, upOved, upKotev, upOkhel:
		return "verb"
	case upMayim:
		return "noun"
	default:
		return "adv"
	}
}

func upPool() []db.LexemeView {
	gloss := upGlosses()
	out := make([]db.LexemeView, 0, len(gloss))
	var id int64
	for _, text := range []string{upAta, upAt, upGar, upGara, upPo, upMayim, upEifo, upOved, upKotev, upOkhel} {
		id++
		out = append(out, db.LexemeView{ID: id, Text: text, Gloss: gloss[text], POS: upPOS(text)})
	}
	return out
}

// TestGlossOverlapDropsWrongButtons is the button filter on the graded asks
// that remain. A wrong button is dropped when its gloss overlaps the target,
// not only when the gloss is identical. INIT-038/SPEC-008.
func TestGlossOverlapDropsWrongButtons(t *testing.T) {
	pool := upPool()
	you, live := pool[0], pool[2]
	byID := map[string]db.LexemeView{}
	for _, lex := range pool {
		byID[itoa(lex.ID)] = lex
	}
	recall := askChoices(units.ActivityRecall, pool, nil, you, choices.ChoiceSeed{RunID: 1, Step: 0, TargetID: you.ID}, choiceExclude{})
	if len(recall) < lessonMinChoices {
		t.Fatalf("recall buttons = %d, want at least %d: %+v", len(recall), lessonMinChoices, recall)
	}
	assertNoChoiceOverlap(t, "recall", recall, byID, you)

	produce := lessonChoices(pool, nil, live, choices.ChoiceSeed{RunID: 1, Step: 5, TargetID: live.ID}, true, false, lessonChoiceButtons, choiceExclude{})
	if len(produce) < lessonMinChoices {
		t.Fatalf("produce buttons = %d, want at least %d: %+v", len(produce), lessonMinChoices, produce)
	}
	assertNoChoiceOverlap(t, "produce", produce, byID, live)

	wide := wideChoices(pool, nil, you, choices.ChoiceSeed{RunID: 2, Step: 0, TargetID: you.ID}, false, false, choiceExclude{})
	assertNoChoiceOverlap(t, "wide", wide, byID, you)
}

func assertNoChoiceOverlap(t *testing.T, name string, got []views.Choice, byID map[string]db.LexemeView, target db.LexemeView) {
	t.Helper()
	for _, choice := range got {
		lex, ok := byID[choice.Value]
		if !ok || lex.ID == target.ID {
			continue
		}
		if activity.GlossesOverlap(lex.Gloss, target.Gloss) {
			t.Errorf("%s button %s gloss %q overlaps target %q", name, choice.Value, lex.Gloss, target.Gloss)
		}
	}
}

// TestUnprovableStepsFiftySeeds walks one fixture lesson, once per seed.
// A cloze and a multi-word listen_pick write no attempt. Building the stored
// sentence does: the key is that sentence. On the graded asks, no wrong
// button's gloss overlaps the target. INIT-038/SPEC-008.
func TestUnprovableStepsFiftySeeds(t *testing.T) {
	const seeds = 50
	emails := make([]string, seeds)
	for i := range emails {
		emails[i] = fmt.Sprintf("unprovable%d@example.com", i)
	}
	fx := openTaughtMultiPlayer(t, emails)
	gloss := upGlosses()
	var inputs []lexicon.Input
	for text, g := range gloss {
		pos := upPOS(text)
		tr := "tr-" + pos
		inputs = append(inputs, lexicon.Input{TextNFC: text, Gloss: g, Translit: &tr, POS: &pos})
	}
	ids := upsertTaughtLexemes(t, fx, inputs)
	glossByID := map[int64]string{}
	for text, id := range ids {
		glossByID[id] = gloss[text]
	}
	line := upLine()
	acts := []units.Activity{
		{Type: units.ActivityRecall, Text: upAta},
		{Type: units.ActivityCloze, Text: line, Target: upGar, Translation: "Do you live here?"},
		{Type: units.ActivityOrder, Text: line, Translation: "Do you live here?"},
		{Type: units.ActivityListenPick, Text: line},
		{Type: units.ActivityListenPick, Text: upEifo},
		// Write is typed now, so it has no buttons to count. lesson_typed_test.go covers it.
	}
	words := []string{upAta, upAt, upGar, upGara, upPo, upMayim, upEifo, upOved, upKotev, upOkhel}
	unitID := putLesson(t, fx.store, nil, units.Unit{
		Title: "You live here", Position: 0, NewWords: words,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: acts}},
	}, "shared")
	publish(t, fx.store, unitID)
	lesson, err := fx.store.CourseLesson(fx.ctx, fx.userID, unitID, 0)
	if err != nil {
		t.Fatal(err)
	}
	path := "/lesson/" + itoa(unitID) + "/0"
	pl := player{store: fx.store}

	for seed := 0; seed < seeds; seed++ {
		user, err := fx.store.UserByEmail(fx.ctx, emails[seed])
		if err != nil {
			t.Fatal(err)
		}
		cookie := mintSession(t, fx.store, user.ID, lessonCSRF, fx.now.Add(time.Hour))
		body := getAuthed(t, fx.h, cookie, path)
		attempts := 0
		for step, act := range acts {
			lex, err := fx.store.LexemesByText(fx.ctx, actTokens(act))
			if err != nil {
				t.Fatal(err)
			}
			runID := mustRunID(t, body)
			run, err := fx.store.PlayerRunFor(fx.ctx, user.ID, runID)
			if err != nil {
				t.Fatal(err)
			}
			target, err := pl.targetOf(fx.ctx, lesson, act)
			if err != nil {
				t.Fatal(err)
			}
			n, err := pl.askChoiceCount(fx.ctx, user.ID, lesson, run, act, target)
			if err != nil {
				t.Fatal(err)
			}
			if act.Type == units.ActivityOrder {
				if !strings.Contains(body, `data-order-chip`) {
					t.Fatalf("seed %d order step did not ask for the sentence", seed)
				}
				served := servedOrderChips(t, body)
				rec := postOrderPositions(t, fx.h, cookie, path, itoa(runID), positionsForTokens(served, units.Tokenize(act.Text)))
				if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "feedback--right") {
					t.Fatalf("seed %d order build = %d", seed, rec.Code)
				}
				attempts++
				next := postLesson(t, fx.h, cookie, path, itoa(runID), "continue", true, lessonCSRF)
				if next.Code != http.StatusOK {
					t.Fatalf("seed %d continue order = %d", seed, next.Code)
				}
				body = next.Body.String()
				continue
			}
			got := choiceValuesAllowEmpty(t, body)
			if len(got) != n {
				t.Fatalf("seed %d step %d %s: render %d buttons, post count %d", seed, step, act.Type, len(got), n)
			}
			if unprovableAsk(act, lex) {
				if n != 0 {
					t.Fatalf("seed %d step %d %s is still an ask (%d buttons)", seed, step, act.Type, n)
				}
				if target.ID != 0 {
					rec := postLesson(t, fx.h, cookie, path, itoa(runID), itoa(target.ID), true, lessonCSRF)
					if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "feedback--") {
						t.Fatalf("seed %d step %d graded a reveal", seed, step)
					}
				}
				if gotN := countRows(t, fx.store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, user.ID); gotN != attempts {
					t.Fatalf("seed %d step %d %s wrote an attempt (%d → %d)", seed, step, act.Type, attempts, gotN)
				}
				next := postLesson(t, fx.h, cookie, path, itoa(runID), "continue", true, lessonCSRF)
				if next.Code != http.StatusOK {
					t.Fatalf("seed %d continue %s = %d", seed, act.Type, next.Code)
				}
				body = next.Body.String()
				continue
			}
			if n < lessonMinChoices {
				t.Fatalf("seed %d step %d %s has %d buttons, want a graded ask", seed, step, act.Type, n)
			}
			for _, idStr := range got {
				id, err := strconv.ParseInt(idStr, 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				if id == target.ID {
					continue
				}
				if activity.GlossesOverlap(glossByID[id], glossByID[target.ID]) {
					t.Fatalf("seed %d step %d wrong button %d gloss %q overlaps %q", seed, step, id, glossByID[id], glossByID[target.ID])
				}
			}
			ans := postLesson(t, fx.h, cookie, path, itoa(runID), itoa(target.ID), true, lessonCSRF)
			if ans.Code != http.StatusOK || !strings.Contains(ans.Body.String(), "feedback--right") {
				t.Fatalf("seed %d step %d grade: %d\n%s", seed, step, ans.Code, ans.Body.String())
			}
			attempts++
			if gotN := countRows(t, fx.store, `SELECT COUNT(*) FROM attempts WHERE user_id = ?`, user.ID); gotN != attempts {
				t.Fatalf("seed %d step %d attempts = %d, want %d", seed, step, gotN, attempts)
			}
			next := postLesson(t, fx.h, cookie, path, itoa(mustRunID(t, ans.Body.String())), "continue", true, lessonCSRF)
			if next.Code != http.StatusOK {
				t.Fatalf("seed %d continue after %s = %d", seed, act.Type, next.Code)
			}
			body = next.Body.String()
		}
		if n := countRows(t, fx.store, `SELECT COUNT(*) FROM attempts WHERE user_id = ? AND activity = 'cloze'`, user.ID); n != 0 {
			t.Fatalf("seed %d cloze attempts = %d", seed, n)
		}
		if n := countRows(t, fx.store, `SELECT COUNT(*) FROM attempts WHERE user_id = ? AND activity = 'listen_pick' AND prompt LIKE '%' || ? || '%'`, user.ID, upGar); n != 0 {
			t.Fatalf("seed %d multi-word listen_pick attempts = %d", seed, n)
		}
	}
}

var orderChipRE = regexp.MustCompile(`(?s)data-order-i="(\d+)"[^>]*>.*?<bdi[^>]*>([^<]+)</bdi>`)

func servedOrderChips(t *testing.T, body string) []string {
	t.Helper()
	matches := orderChipRE.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		t.Fatalf("no order chips:\n%s", body)
	}
	out := make([]string, len(matches))
	for _, m := range matches {
		i, err := strconv.Atoi(m[1])
		if err != nil || i < 0 || i >= len(out) {
			t.Fatalf("chip index %q", m[1])
		}
		out[i] = strings.TrimSpace(m[2])
	}
	return out
}

func postOrderPositions(t *testing.T, h http.Handler, cookie *http.Cookie, path, runID string, positions []int) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("run", runID)
	form.Set("answer", "check")
	form.Set("_csrf", lessonCSRF)
	for _, p := range positions {
		form.Add("order", strconv.Itoa(p))
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Host = "learn.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("X-CSRF-Token", lessonCSRF)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
