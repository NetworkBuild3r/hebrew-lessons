package web

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Sentence listen_pick words. Glosses are single words so they cannot be
// mistaken for an author-written line.
const (
	slAni   = "אֲנִי"
	slGar   = "גָּר"
	slPo    = "פֹּה"
	slBayit = "בַּיִת"
	slMayim = "מַיִם"

	slAnswer   = "I live here"
	slEvening  = "Good evening"
	slHouse    = "This is a house"
	slCold     = "The water is cold"
	slTomorrow = "See you tomorrow"
	slEvening2 = "good evening"
	slAnswer2  = "I live here!"
	slFifth    = "A fifth line stays off the buttons"
)

// slWant is the answer plus the four distractors that survive folding.
// The fixture lists a folded duplicate of the answer, a folded duplicate of
// Good evening, and a fifth distinct line, in that order, so dropping the
// fold drops one of these into the four slots.
var slWant = []string{slAnswer, slEvening, slHouse, slCold, slTomorrow}

func TestSentenceListenCreditsNothingAndExposesOnce(t *testing.T) {
	env := openSentenceLesson(t, sentenceActivities(false), &listenRec{})
	body := getAuthed(t, env.h, env.cookie, env.path)
	rec := postLesson(t, env.h, env.cookie, env.path, runValue(t, body), slAnswer, true, env.csrf)
	if rec.Code != http.StatusOK {
		t.Fatalf("answer = %d\n%s", rec.Code, rec.Body.String())
	}
	again := postLesson(t, env.h, env.cookie, env.path, runValue(t, body), slAnswer, true, env.csrf)
	if again.Code != http.StatusOK {
		t.Fatalf("repeat = %d", again.Code)
	}
	if len(env.rec.attempts) != 0 {
		t.Fatalf("exposure-only listen_pick wrote %d attempts", len(env.rec.attempts))
	}
	if len(env.rec.exposures) != 1 {
		t.Fatalf("exposures = %d, want 1", len(env.rec.exposures))
	}
	got := env.rec.exposures[0]
	if got.where != evidence.ContextLessonListen || got.unitID != env.unitID || got.userID != env.userID {
		t.Fatalf("exposure = %+v", got)
	}
	wantIDs := []int64{env.ids[slAni], env.ids[slGar], env.ids[slPo]}
	if !slices.Equal(got.ids, wantIDs) {
		t.Fatalf("exposure lexemes = %v, want %v", got.ids, wantIDs)
	}
	for _, table := range []string{"attempts", "review_log", "cards"} {
		if n := scalar(t, env.dbPath, `SELECT COUNT(*) FROM `+table); n != 0 {
			t.Fatalf("%s = %d, want 0", table, n)
		}
	}
}

func TestSentenceListenChoicesAreTranslations(t *testing.T) {
	env := openSentenceLesson(t, sentenceActivities(false), nil)
	labels := sentenceLabels(t, getAuthed(t, env.h, env.cookie, env.path))
	if len(labels) < 5 {
		t.Fatalf("choices = %q, want at least 5", labels)
	}
	if !sameStrings(labels, slWant) {
		t.Fatalf("choices = %q, want %q", labels, slWant)
	}
	glosses := []string{"I", "lives", "here", "house", "water"}
	for _, label := range labels {
		if slices.Contains(glosses, label) {
			t.Fatalf("choice %q is a lexicon gloss", label)
		}
	}
	seen := map[string]int{}
	for _, label := range labels {
		seen[foldTranslation(label)]++
	}
	for key, n := range seen {
		if n != 1 {
			t.Fatalf("folded %q appears %d times in %q", key, n, labels)
		}
	}
	for _, banned := range []string{slAnswer2, slEvening2, slFifth} {
		if slices.Contains(labels, banned) {
			t.Fatalf("choice %q should have been folded away or capped", banned)
		}
	}
}

func TestSentenceListenOrderIsStablePerRun(t *testing.T) {
	env := openSentenceLesson(t, sentenceActivities(false), nil)
	var first []string
	for i := 0; i < 8; i++ {
		labels := sentenceLabels(t, getAuthed(t, env.h, env.cookie, env.path))
		if !sameStrings(labels, slWant) {
			t.Fatalf("render %d choices = %q, want the sentence set %q", i, labels, slWant)
		}
		if i == 0 {
			first = labels
			continue
		}
		if !slices.Equal(labels, first) {
			t.Fatalf("render %d order %q, first render %q", i, labels, first)
		}
	}

	orders := map[string]int{strings.Join(first, "|"): 1}
	emails := make([]string, 0, 25)
	emails = append(emails, "brian@example.com")
	for i := 0; i < 24; i++ {
		emails = append(emails, "listen"+itoa(int64(i))+"@example.com")
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: fixedClock(env.now), Store: env.store,
		AllowedEmails: emails,
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: env.store})
	for i := 0; i < 24; i++ {
		email := emails[i+1]
		user, err := env.store.CreateUser(context.Background(), email, "sub-"+email, auth.RoleLearner)
		if err != nil {
			t.Fatal(err)
		}
		cookie := mintSession(t, env.store, user.ID, "csrf-"+email, env.now.Add(time.Hour))
		labels := sentenceLabels(t, getAuthed(t, h, cookie, env.path))
		if !sameStrings(labels, slWant) {
			t.Fatalf("session %d choices = %q", i, labels)
		}
		orders[strings.Join(labels, "|")]++
	}
	if len(orders) < 2 {
		t.Fatalf("button order did not change across sessions: %v", orders)
	}
}

func TestSentenceListenGradesTranslationOnBothPaths(t *testing.T) {
	for _, hx := range []bool{true, false} {
		name := "htmx"
		if !hx {
			name = "no-js"
		}
		t.Run(name+" correct", func(t *testing.T) {
			env := openSentenceLesson(t, sentenceActivities(false), &listenRec{})
			body := getAuthed(t, env.h, env.cookie, env.path)
			rec := postSentence(t, env, runValue(t, body), slAnswer, hx)
			assertGrade(t, rec, hx, "feedback--right")
		})
		t.Run(name+" distractor", func(t *testing.T) {
			env := openSentenceLesson(t, sentenceActivities(false), &listenRec{})
			body := getAuthed(t, env.h, env.cookie, env.path)
			rec := postSentence(t, env, runValue(t, body), slHouse, hx)
			assertGrade(t, rec, hx, "feedback--wrong")
		})
	}
}

func TestSentenceListenDoesNotRevealTheAnswer(t *testing.T) {
	env := openSentenceLesson(t, sentenceActivities(false), nil)
	body := getAuthed(t, env.h, env.cookie, env.path)
	if strings.Contains(body, `data-state="correct"`) || strings.Contains(body, `data-state="wrong"`) || strings.Contains(body, ">Correct<") {
		t.Fatal("the ask marks a choice before the answer")
	}
	labels := sentenceLabels(t, body)
	if !slices.Contains(labels, slAnswer) {
		t.Fatalf("the translation is not one of the choices: %q", labels)
	}
	i := strings.Index(body, `class="choices"`)
	if i < 0 {
		t.Fatalf("choices missing:\n%s", body)
	}
	j := strings.Index(body[i:], "</div>")
	if j < 0 {
		t.Fatal("choices div is unclosed")
	}
	rest := body[:i] + body[i+j:]
	if strings.Contains(rest, slAnswer) {
		t.Fatal("the translation appears outside the choices")
	}
	if containsHebrew(choiceRegion(body)) {
		t.Fatal("sentence choices show Hebrew before the answer")
	}
}

func TestSingleWordListenWithoutTargetCreditsThatWord(t *testing.T) {
	const hello = "שָׁלוֹם"
	env := openWordListen(t, units.Activity{Type: units.ActivityListenPick, Text: hello})
	body := getAuthed(t, env.h, env.cookie, env.path)
	rec := postLesson(t, env.h, env.cookie, env.path, runValue(t, body), itoa(env.ids[hello]), true, env.csrf)
	if rec.Code != http.StatusOK {
		t.Fatalf("answer = %d\n%s", rec.Code, rec.Body.String())
	}
	assertListenCredit(t, env, hello)
}

func TestSingleWordListenWithTargetCreditsThatWord(t *testing.T) {
	const hello = "שָׁלוֹם"
	env := openWordListen(t, units.Activity{Type: units.ActivityListenPick, Text: hello, Target: hello})
	body := getAuthed(t, env.h, env.cookie, env.path)
	rec := postLesson(t, env.h, env.cookie, env.path, runValue(t, body), itoa(env.ids[hello]), true, env.csrf)
	if rec.Code != http.StatusOK {
		t.Fatalf("answer = %d\n%s", rec.Code, rec.Body.String())
	}
	assertListenCredit(t, env, hello)
}

func TestSentenceListenTargetCreditsOnlyThatWord(t *testing.T) {
	env := openSentenceLesson(t, sentenceActivities(true), nil)
	body := getAuthed(t, env.h, env.cookie, env.path)
	labels := sentenceLabels(t, body)
	if !sameStrings(labels, slWant) {
		t.Fatalf("a targeted sentence used glosses: %q", labels)
	}
	rec := postLesson(t, env.h, env.cookie, env.path, runValue(t, body), slAnswer, true, env.csrf)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "feedback--right") {
		t.Fatalf("targeted sentence answer = %d\n%s", rec.Code, rec.Body.String())
	}
	rows, err := env.store.ListAttempts(context.Background(), env.userID, env.ids[slPo])
	if err != nil || len(rows) != 1 || !rows[0].Correct || rows[0].Activity != units.ActivityListenPick || rows[0].Skill != fsrs.SkillListen {
		t.Fatalf("target attempts = %+v %v", rows, err)
	}
	for _, word := range []string{slAni, slGar} {
		rows, err = env.store.ListAttempts(context.Background(), env.userID, env.ids[word])
		if err != nil || len(rows) != 0 {
			t.Fatalf("%s attempts = %+v %v", word, rows, err)
		}
	}
	if n := scalar(t, env.dbPath, `SELECT COUNT(*) FROM exposures`); n != 0 {
		t.Fatalf("a credited sentence logged %d exposures", n)
	}
}

type sentenceEnv struct {
	h      http.Handler
	store  *db.Store
	dbPath string
	cookie *http.Cookie
	csrf   string
	path   string
	now    time.Time
	userID int64
	unitID int64
	ids    map[string]int64
	rec    *listenRec
}

type listenExposure struct {
	userID int64
	ids    []int64
	where  string
	unitID int64
}

type listenRec struct {
	attempts  []evidence.Attempt
	exposures []listenExposure
}

func (r *listenRec) Attempt(_ context.Context, a evidence.Attempt) (fsrs.Rating, error) {
	r.attempts = append(r.attempts, a)
	return fsrs.Again, nil
}

func (r *listenRec) Expose(_ context.Context, learnerID int64, lexemeIDs []int64, where string, unitID int64) error {
	r.exposures = append(r.exposures, listenExposure{
		userID: learnerID,
		ids:    append([]int64(nil), lexemeIDs...),
		where:  where,
		unitID: unitID,
	})
	return nil
}

func openSentenceLesson(t *testing.T, activities []units.Activity, rec *listenRec) sentenceEnv {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "hebrew.sqlite")
	store, err := db.Open(ctx, dbPath, fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	brian, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	ids := upsertSentenceWords(t, store)
	line := slAni + " " + slGar + " " + slPo
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "At home", Situation: "Saying where you live.", Position: 1,
		NewWords: []string{slAni, slGar, slPo, slBayit, slMayim},
		Lessons:  []units.Lesson{{Kind: units.KindLesson, Activities: activities}},
	}, "shared")
	publish(t, store, unitID)
	if activities[0].Text != line && activities[0].Type == units.ActivityListenPick && strings.Contains(activities[0].Text, " ") {
		t.Fatalf("fixture text = %q", activities[0].Text)
	}
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: fixedClock(now), Store: store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var hook lessonRecorder
	if rec != nil {
		hook = rec
	}
	h := NewOptions(nil, svc, Options{Store: store, lessonRec: hook})
	return sentenceEnv{
		h: h, store: store, dbPath: dbPath,
		cookie: mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour)),
		csrf:   "brian-csrf",
		path:   "/lesson/" + itoa(unitID) + "/0",
		now:    now, userID: brian.ID, unitID: unitID, ids: ids, rec: rec,
	}
}

// sentenceActivities is one sentence listen_pick plus the other English lines
// of the unit. withTarget sets the listen_pick target to the last word.
func sentenceActivities(withTarget bool) []units.Activity {
	line := slAni + " " + slGar + " " + slPo
	listen := units.Activity{Type: units.ActivityListenPick, Text: line, Translation: slAnswer}
	if withTarget {
		listen.Target = slPo
	}
	return []units.Activity{
		listen,
		{Type: units.ActivityRecall, Text: slBayit, Target: slBayit, Translation: slEvening},
		{Type: units.ActivityRecall, Text: slMayim, Target: slMayim, Translation: slEvening2},
		{Type: units.ActivityStory, Text: line, Translation: slAnswer2},
		{Type: units.ActivityProduce, Text: slBayit, Target: slBayit, Translation: slHouse},
		{Type: units.ActivityCloze, Text: line, Target: slAni, Translation: slCold},
		{Type: units.ActivityDialogue, Lines: []units.DialogueLine{
			{Speaker: "a", Text: slBayit, Translation: slTomorrow},
			{Speaker: "b", Text: slMayim, Translation: slFifth},
		}},
	}
}

func openWordListen(t *testing.T, listen units.Activity) sentenceEnv {
	t.Helper()
	const (
		hello   = "שָׁלוֹם"
		morning = "בֹּקֶר"
	)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "hebrew.sqlite")
	store, err := db.Open(ctx, dbPath, fixedClock(now))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	brian, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(ctx, []lexicon.Input{
		{TextNFC: hello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: morning, Gloss: "morning", Translit: sp("boker"), Gender: sp("m"), POS: sp("noun")},
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for _, row := range saved {
		ids[row.TextNFC] = row.ID
	}
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Greetings", Situation: "Saying hello.", Position: 1,
		NewWords: []string{hello, morning},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			listen,
			{Type: units.ActivityRecall, Text: morning, Target: morning},
		}}},
	}, "shared")
	publish(t, store, unitID)
	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: fixedClock(now), Store: store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	return sentenceEnv{
		h: h, store: store, dbPath: dbPath,
		cookie: mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour)),
		csrf:   "brian-csrf", path: "/lesson/" + itoa(unitID) + "/0",
		now: now, userID: brian.ID, unitID: unitID, ids: ids,
	}
}

func upsertSentenceWords(t *testing.T, store *db.Store) map[string]int64 {
	t.Helper()
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(context.Background(), []lexicon.Input{
		{TextNFC: slAni, Gloss: "I", Translit: sp("ani"), POS: sp("pron")},
		{TextNFC: slGar, Gloss: "lives", Translit: sp("gar"), POS: sp("verb")},
		{TextNFC: slPo, Gloss: "here", Translit: sp("po"), POS: sp("adv")},
		{TextNFC: slBayit, Gloss: "house", Translit: sp("bayit"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: slMayim, Gloss: "water", Translit: sp("mayim"), Gender: sp("m"), POS: sp("noun")},
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for _, row := range saved {
		ids[row.TextNFC] = row.ID
	}
	return ids
}

func assertListenCredit(t *testing.T, env sentenceEnv, word string) {
	t.Helper()
	rows, err := env.store.ListAttempts(context.Background(), env.userID, env.ids[word])
	if err != nil || len(rows) != 1 || !rows[0].Correct || rows[0].Activity != units.ActivityListenPick || rows[0].LexemeID != env.ids[word] {
		t.Fatalf("attempts = %+v %v", rows, err)
	}
	for _, table := range []string{"review_log", "cards", "exposures"} {
		if n := scalar(t, env.dbPath, `SELECT COUNT(*) FROM `+table); n != 0 {
			t.Fatalf("%s = %d, want 0", table, n)
		}
	}
}

func postSentence(t *testing.T, env sentenceEnv, runID, answer string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("run", runID)
	form.Set("answer", answer)
	form.Set("_csrf", env.csrf)
	req := httptest.NewRequest(http.MethodPost, env.path, strings.NewReader(form.Encode()))
	req.Host = "learn.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("X-CSRF-Token", env.csrf)
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	req.AddCookie(env.cookie)
	rec := httptest.NewRecorder()
	env.h.ServeHTTP(rec, req)
	return rec
}

func assertGrade(t *testing.T, rec *httptest.ResponseRecorder, hx bool, class string) {
	t.Helper()
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, class) {
		t.Fatalf("grade = %d, want %s\n%s", rec.Code, class, body)
	}
	if hx && strings.Contains(body, "<html") {
		t.Fatal("htmx answer returned a full page")
	}
	if !hx && !strings.Contains(body, "<html") {
		t.Fatal("no-js answer did not return the full page")
	}
}

var sentenceButton = regexp.MustCompile(`(?s)<button\b([^>]*\bname="answer"[^>]*)>(.*?)</button>`)

func sentenceLabels(t *testing.T, body string) []string {
	t.Helper()
	var labels []string
	for _, m := range sentenceButton.FindAllStringSubmatch(choiceRegion(body), -1) {
		value := html.UnescapeString(attrFromTag(m[1], "value"))
		label := html.UnescapeString(strings.TrimSpace(stripTags(labelSpan(m[2]))))
		if value == "continue" {
			continue
		}
		if value != label {
			t.Fatalf("button value %q, label %q", value, label)
		}
		labels = append(labels, label)
	}
	if len(labels) == 0 {
		t.Fatalf("no sentence choices:\n%s", choiceRegion(body))
	}
	return labels
}

func labelSpan(inner string) string {
	const open = `class="c-answer__label">`
	i := strings.Index(inner, open)
	if i < 0 {
		return inner
	}
	rest := inner[i+len(open):]
	j := strings.Index(rest, "</span>")
	if j < 0 {
		return rest
	}
	return rest[:j]
}

func attrFromTag(tag, name string) string {
	m := regexp.MustCompile(name + `="([^"]*)"`).FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	return m[1]
}

func stripTags(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '<' {
			j := strings.IndexByte(s[i:], '>')
			if j < 0 {
				break
			}
			i += j
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	a := append([]string(nil), got...)
	b := append([]string(nil), want...)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
