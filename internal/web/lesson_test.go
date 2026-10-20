package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"

	_ "modernc.org/sqlite"
)

func TestLessonPlayer(t *testing.T) {
	const (
		hello   = "שָׁלוֹם"
		morning = "בֹּקֶר"
	)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hebrew.sqlite")
	audioDir := filepath.Join(dir, "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store, err := db.Open(ctx, dbPath, clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	brian, err := store.CreateUser(ctx, "brian@example.com", "sub-brian", auth.RoleLearner)
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.CreateUser(ctx, "other@example.com", "sub-other", auth.RoleLearner)
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

	// A role=new row gates the card until THAT lesson is finished, including
	// a draft or another learner's unit. These two units recycle the words
	// so finishing the shared lesson is what lets the cards into the queue.
	draftID := putLesson(t, store, nil, units.Unit{
		Title: "Draft secret", Position: 0,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: hello},
		}}},
	}, "shared")
	otherID := putLesson(t, store, nil, units.Unit{
		Title: "Other morning", Position: 1,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: morning},
		}}},
	}, "learner:"+itoa(other.ID))
	sharedID := putLesson(t, store, nil, units.Unit{
		Title: "Shared hello", Situation: "Saying hello.", Concept: "Greeting", Position: 2,
		NewWords: []string{hello, morning},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: hello + " " + morning},
			{Type: units.ActivityRecall, Text: hello},
		}}},
	}, "shared")
	laterID := putLesson(t, store, nil, units.Unit{
		Title: "Brian later", Position: 3,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: "OK"},
		}}},
	}, "learner:"+itoa(brian.ID))
	publish(t, store, otherID, sharedID, laterID)

	phraseID := strings.Repeat("ab", 32)
	wordID := strings.Repeat("cd", 32)
	if _, err := store.InsertAudio(ctx, db.Asset{
		ID: phraseID, TextNFC: units.Normalize(hello + " " + morning), Voice: "voice", Model: "eleven_v4", Bytes: 4,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertAudio(ctx, db.Asset{
		ID: wordID, TextNFC: hello, Voice: "voice", Model: "eleven_v4", Bytes: 4,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(audioDir, wordID+".mp3"), []byte("word"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `
		INSERT INTO cards (user_id, lexeme_id, skill, state, due, stability, difficulty, reps, lapses)
		VALUES (?, ?, 'read', 'new', ?, 0, 0, 0, 0)`,
		brian.ID, ids[hello], db.FormatTime(now),
	); err != nil {
		t.Fatal(err)
	}
	if due, err := store.ListDueCards(ctx, brian.ID, now); err != nil || len(due) != 0 {
		t.Fatalf("cards before the lesson = %d, %v", len(due), err)
	}

	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		CookieSecure:  false,
		Clock:         clk,
		Store:         store,
		AllowedEmails: []string{"brian@example.com", "other@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store, AudioDir: audioDir})
	brianCookie := mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour))
	otherCookie := mintSession(t, store, other.ID, "other-csrf", now.Add(time.Hour))

	today := getAuthed(t, h, brianCookie, "/")
	link := "/lesson/" + itoa(sharedID) + "/0"
	if !strings.Contains(today, `href="`+link+`"`) {
		t.Fatalf("Today does not link to the shared lesson:\n%s", today)
	}
	for _, gloss := range []string{"(shalom, hello)", "(boker, morning)"} {
		if !strings.Contains(today, gloss) {
			t.Fatalf("chip gloss %s missing", gloss)
		}
	}
	if !strings.Contains(today, "Saying hello.") || !strings.Contains(today, "ז׳") {
		t.Fatal("situation or gender chip missing")
	}
	for _, hidden := range []string{"Draft secret", "Other morning", "Brian later", "כִּסֵּא", "הַבֹּקֶר שֶׁלִּי"} {
		if strings.Contains(today, hidden) {
			t.Fatalf("Today shows %s", hidden)
		}
	}
	if !strings.Contains(today, `data-due="0"`) || !strings.Contains(today, `class="btn btn-primary"`) || !strings.Contains(today, link) {
		t.Fatal("a gated card stays off the due count, and the next lesson stays on Today")
	}
	if strings.Contains(today, ">12<") {
		t.Fatal("placeholder due count is back")
	}
	if scalar(t, dbPath, `SELECT COUNT(*) FROM lesson_runs`) != 0 {
		t.Fatal("opening Today started a lesson run")
	}
	otherToday := getAuthed(t, h, otherCookie, "/")
	if !strings.Contains(otherToday, "Other morning") || strings.Contains(otherToday, "Shared hello") {
		t.Fatal("other learner should see their own unit first")
	}

	for _, path := range []string{
		"/lesson/" + itoa(draftID) + "/0",
		"/lesson/" + itoa(otherID) + "/0",
		"/lesson/" + itoa(laterID) + "/0",
		"/lesson/" + itoa(sharedID) + "/1",
		"/lesson/0/0",
	} {
		if code := codeAuthed(t, h, brianCookie, path); code != http.StatusNotFound {
			t.Fatalf("GET %s = %d, want 404", path, code)
		}
		posted := postLesson(t, h, brianCookie, path, "1", "continue", true, "brian-csrf")
		if posted.Code != http.StatusNotFound {
			t.Fatalf("POST %s = %d, want 404\n%s", path, posted.Code, posted.Body.String())
		}
	}

	body := getAuthed(t, h, brianCookie, "/lesson")
	runID := runValue(t, body)
	if !strings.Contains(body, "wb__gl") || !strings.Contains(body, "wb__tr") || !strings.Contains(body, hello) {
		t.Fatal("story step is missing the word block")
	}
	if !strings.Contains(body, "/audio/"+phraseID+".mp3") || !strings.Contains(body, "/audio/"+wordID+".mp3") {
		t.Fatal("story step is missing its clips")
	}
	for _, want := range []string{
		`class="orb orb--hero"`, `data-src="/audio/` + phraseID + `.mp3"`, `aria-label="Play the line"`,
		`aria-label="Play ` + hello + `"`, `/static/js/clips.js`, `preload="none"`,
		`<audio id="line-clip"`, ` controls`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("story clip missing %s", want)
		}
	}
	if strings.Contains(body, "clip__label") || strings.Contains(body, "onclick=") || strings.Contains(body, "hx-on") {
		t.Fatal("story clip uses an inline handler")
	}
	if strings.Contains(body, `class="c-orb"`) {
		t.Fatal("the lesson clip reused the dialogue orb")
	}
	if strings.Count(body, "/static/js/clips.js") != 1 {
		t.Fatalf("clips.js included %d times", strings.Count(body, "/static/js/clips.js"))
	}
	if scalar(t, dbPath, `SELECT COUNT(*) FROM lesson_runs WHERE user_id = ? AND finished_at IS NULL`, brian.ID) != 1 {
		t.Fatal("opening the lesson did not write one open run")
	}
	again := getAuthed(t, h, brianCookie, link)
	if runValue(t, again) != runID {
		t.Fatal("refresh started a second run")
	}

	denied := postLesson(t, h, brianCookie, link, runID, "continue", false, "")
	if denied.Code != http.StatusForbidden {
		t.Fatalf("post without origin = %d", denied.Code)
	}
	if scalar(t, dbPath, `SELECT activity_ix FROM lesson_runs WHERE id = ?`, runID) != 0 {
		t.Fatal("rejected post advanced the run")
	}

	story := postLesson(t, h, brianCookie, link, runID, "continue", true, "brian-csrf")
	if story.Code != http.StatusOK {
		t.Fatalf("continue = %d %s", story.Code, story.Body.String())
	}
	recall := story.Body.String()
	if strings.Contains(recall, "<html") || strings.Contains(recall, "wb__gl") || strings.Contains(recall, "wb__tr") || strings.Contains(recall, "shalom") {
		t.Fatalf("recall prompt shows the answer:\n%s", recall)
	}
	if strings.Contains(recall, "data-target") || strings.Contains(recall, "This word") {
		t.Fatalf("a single-word recall marks a target:\n%s", recall)
	}
	if !strings.Contains(recall, hello) || !strings.Contains(recall, "hello") || !strings.Contains(recall, "What does this Hebrew mean?") {
		t.Fatalf("recall prompt missing: %s", recall)
	}
	if strings.Contains(recall, "What does “") {
		t.Fatalf("a single-word recall names a word in the question:\n%s", recall)
	}
	if strings.Contains(recall, `value="`+hello+`"`) || strings.Contains(recall, `value="`+morning+`"`) {
		t.Fatalf("recall choice value leaks Hebrew:\n%s", recall)
	}
	if !strings.Contains(recall, `value="`+itoa(ids[hello])+`"`) {
		t.Fatalf("recall choice value should be the lexeme id:\n%s", recall)
	}

	wrong := postLesson(t, h, brianCookie, link, runID, itoa(ids[morning]), true, "brian-csrf")
	revealed := wrong.Body.String()
	if wrong.Code != http.StatusOK || !strings.Contains(revealed, "Not quite") || !strings.Contains(revealed, "shalom") || !strings.Contains(revealed, "wb__gl") {
		t.Fatalf("wrong answer did not reveal the word: %d %s", wrong.Code, revealed)
	}
	done := postLesson(t, h, brianCookie, link, runID, "continue", true, "brian-csrf")
	finished := done.Body.String()
	if done.Code != http.StatusOK || !strings.Contains(finished, "Lesson complete.") || !strings.Contains(finished, "Score 50%") {
		t.Fatalf("finish = %d %s", done.Code, finished)
	}
	var score float64
	if err := sqlScalar(t, dbPath, `SELECT score FROM lesson_runs WHERE id = ? AND finished_at IS NOT NULL`, []any{runID}, &score); err != nil {
		t.Fatal(err)
	}
	if score != 0.5 {
		t.Fatalf("score = %v", score)
	}
	if scalar(t, dbPath, `SELECT COUNT(*) FROM cards WHERE user_id = ? AND lexeme_id = ? AND skill = 'read'`, brian.ID, ids[hello]) != 1 {
		t.Fatal("finishing duplicated the read card")
	}
	due, err := store.ListDueCards(ctx, brian.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 4 {
		t.Fatalf("due cards = %d, want 4", len(due))
	}
	gotLex := map[int64]int{}
	for _, card := range due {
		gotLex[card.LexemeID]++
	}
	if gotLex[ids[hello]] != 2 || gotLex[ids[morning]] != 2 {
		t.Fatalf("due lexemes = %v", gotLex)
	}
	if otherDue, err := store.ListDueCards(ctx, other.ID, now); err != nil || len(otherDue) != 0 {
		t.Fatalf("other learner due = %d, %v", len(otherDue), err)
	}

	summary := getAuthed(t, h, brianCookie, link)
	if !strings.Contains(summary, "Lesson complete.") {
		t.Fatal("finished lesson did not show the summary")
	}
	if scalar(t, dbPath, `SELECT COUNT(*) FROM lesson_runs WHERE user_id = ? AND unit_id = ?`, brian.ID, sharedID) != 1 {
		t.Fatal("reopening a finished lesson wrote another run")
	}
	next := getAuthed(t, h, brianCookie, "/lesson")
	if !strings.Contains(next, "Brian later") || strings.Contains(next, "Other morning") {
		t.Fatal("the next lesson should be the learner's own published unit")
	}
	if runValue(t, next) == runID {
		t.Fatal("the next lesson reused the finished run")
	}

	foreign := postLesson(t, h, otherCookie, link, runID, "continue", true, "other-csrf")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("other learner post = %d", foreign.Code)
	}

	audio := httptest.NewRecorder()
	h.ServeHTTP(audio, httptest.NewRequest(http.MethodGet, "/audio/"+wordID+".mp3", nil))
	if audio.Code != http.StatusOK || audio.Header().Get("Content-Type") != "audio/mpeg" || audio.Body.String() != "word" {
		t.Fatalf("audio = %d %q %q", audio.Code, audio.Header().Get("Content-Type"), audio.Body.String())
	}
	missing := httptest.NewRecorder()
	h.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/audio/"+phraseID+".mp3", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing file = %d", missing.Code)
	}
	bad := httptest.NewRecorder()
	h.ServeHTTP(bad, httptest.NewRequest(http.MethodGet, "/audio/not-a-hash.mp3", nil))
	if bad.Code != http.StatusNotFound {
		t.Fatalf("bad audio name = %d", bad.Code)
	}
}

func TestLessonPromptDoesNotLeak(t *testing.T) {
	const (
		hello   = "שָׁלוֹם"
		morning = "בֹּקֶר"
	)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	dir := t.TempDir()
	store, err := db.Open(ctx, filepath.Join(dir, "hebrew.sqlite"), fixedClock(now))
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
	line := hello + " " + morning
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "Greetings", Situation: "Saying hello.", Position: 1,
		NewWords: []string{hello, morning},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityListenPick, Text: line},
			{Type: units.ActivityProduce, Text: hello},
			{Type: units.ActivityCloze, Text: line},
		}}},
	}, "shared")
	publish(t, store, unitID)

	phraseID := strings.Repeat("ab", 32)
	helloID := strings.Repeat("cd", 32)
	morningID := strings.Repeat("ef", 32)
	for _, asset := range []db.Asset{
		{ID: phraseID, TextNFC: units.Normalize(line), Voice: "voice", Model: "eleven_v4", Bytes: 4},
		{ID: helloID, TextNFC: hello, Voice: "voice", Model: "eleven_v4", Bytes: 4},
		{ID: morningID, TextNFC: morning, Voice: "voice", Model: "eleven_v4", Bytes: 4},
	} {
		if _, err := store.InsertAudio(ctx, asset); err != nil {
			t.Fatal(err)
		}
	}

	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         fixedClock(now),
		Store:         store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	cookie := mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour))
	path := "/lesson/" + itoa(unitID) + "/0"

	listen := activityHTML(t, getAuthed(t, h, cookie, path))
	if containsHebrew(listen) || strings.Contains(listen, "shalom") || strings.Contains(listen, "boker") {
		t.Fatalf("listen_pick prompt leaks Hebrew or translit:\n%s", listen)
	}
	if containsHebrew(attrValues(listen)) {
		t.Fatalf("listen_pick attributes leak Hebrew:\n%s", attrValues(listen))
	}
	for _, gloss := range []string{">hello<", ">morning<"} {
		if !strings.Contains(listen, gloss) {
			t.Fatalf("listen_pick missing gloss %s:\n%s", gloss, listen)
		}
	}
	if !strings.Contains(listen, `value="`+itoa(ids[hello])+`"`) || !strings.Contains(listen, `value="`+itoa(ids[morning])+`"`) {
		t.Fatalf("listen_pick choices should be lexeme ids:\n%s", listen)
	}
	if strings.Contains(listen, `value="`+hello+`"`) || strings.Contains(listen, `value="`+morning+`"`) {
		t.Fatal("listen_pick value is pointed Hebrew")
	}
	if !strings.Contains(listen, `/audio/`+phraseID+`.mp3`) || !strings.Contains(listen, `aria-label="Play"`) {
		t.Fatalf("listen_pick missing the phrase clip:\n%s", listen)
	}
	if strings.Contains(listen, `aria-label="Play the line"`) || strings.Contains(listen, "Play "+hello) {
		t.Fatalf("listen_pick names the hidden Hebrew:\n%s", listen)
	}
	if !strings.Contains(listen, `class="orb orb--hero"`) || !strings.Contains(listen, `preload="none"`) || !strings.Contains(listen, " controls") {
		t.Fatalf("listen_pick clip is not a native audio plus orb:\n%s", listen)
	}
	if strings.Contains(listen, helloID) || strings.Contains(listen, morningID) {
		t.Fatal("listen_pick prompt includes a word clip")
	}

	runID := runValue(t, listen)
	revealed := activityHTML(t, postLesson(t, h, cookie, path, runID, itoa(ids[hello]), true, "brian-csrf").Body.String())
	if !strings.Contains(revealed, hello) || !strings.Contains(revealed, morning) || !strings.Contains(revealed, "shalom") || !strings.Contains(revealed, "boker") || !strings.Contains(revealed, "wb__gl") {
		t.Fatalf("listen_pick answer did not reveal the words:\n%s", revealed)
	}

	produce := activityHTML(t, postLesson(t, h, cookie, path, runID, "continue", true, "brian-csrf").Body.String())
	if strings.Contains(produce, "<audio") || strings.Contains(produce, "/audio/") || strings.Contains(produce, "shalom") {
		t.Fatalf("produce prompt leaks translit or audio:\n%s", produce)
	}
	if containsHebrew(attrValues(produce)) {
		t.Fatalf("produce attributes leak Hebrew:\n%s", attrValues(produce))
	}
	if !strings.Contains(produce, hello) || !strings.Contains(produce, `value="`+itoa(ids[hello])+`"`) {
		t.Fatalf("produce choices should show pointed Hebrew and post the lexeme id:\n%s", produce)
	}
	if strings.Contains(produce, `value="`+hello+`"`) {
		t.Fatal("produce value is pointed Hebrew")
	}

	shown := activityHTML(t, postLesson(t, h, cookie, path, runID, itoa(ids[hello]), true, "brian-csrf").Body.String())
	if !strings.Contains(shown, hello) || !strings.Contains(shown, "shalom") || !strings.Contains(shown, "wb__gl") || !strings.Contains(shown, `/audio/`+helloID+`.mp3`) {
		t.Fatalf("produce answer did not reveal the word and clip:\n%s", shown)
	}

	cloze := activityHTML(t, postLesson(t, h, cookie, path, runID, "continue", true, "brian-csrf").Body.String())
	sentence, choices, ok := strings.Cut(cloze, `class="choices"`)
	if !ok {
		t.Fatalf("cloze missing choices:\n%s", cloze)
	}
	if strings.Contains(sentence, hello) || strings.Contains(sentence, "shalom") || strings.Contains(sentence, ">hello<") || strings.Contains(sentence, "wb__gl") {
		t.Fatalf("cloze sentence shows the gap or a gloss:\n%s", sentence)
	}
	if !strings.Contains(sentence, morning) || !strings.Contains(sentence, "boker") || !strings.Contains(sentence, "___") {
		t.Fatalf("cloze should show the other word with translit:\n%s", sentence)
	}
	if strings.Contains(cloze, `/audio/`+phraseID+`.mp3`) || strings.Contains(cloze, helloID) {
		t.Fatalf("cloze ask plays the gapped clip:\n%s", cloze)
	}
	if !strings.Contains(cloze, `/audio/`+morningID+`.mp3`) {
		t.Fatalf("cloze hid the other word's clip:\n%s", cloze)
	}
	if strings.Contains(attrValues(cloze), hello) || strings.Contains(attrValues(cloze), "shalom") || strings.Contains(attrValues(cloze), "hello") {
		t.Fatalf("cloze attributes leak the gap:\n%s", attrValues(cloze))
	}
	if !strings.Contains(cloze, `aria-label="Play `+morning+`"`) {
		t.Fatalf("the other word's play button is not named:\n%s", cloze)
	}
	if !strings.Contains(choices, hello) || !strings.Contains(choices, "shalom") {
		t.Fatalf("cloze choices should show pointed Hebrew and translit:\n%s", choices)
	}

	filled := activityHTML(t, postLesson(t, h, cookie, path, runID, itoa(ids[hello]), true, "brian-csrf").Body.String())
	if strings.Contains(filled, "___") || !strings.Contains(filled, hello) || !strings.Contains(filled, morning) {
		t.Fatalf("cloze answer did not fill the gap:\n%s", filled)
	}
	if !strings.Contains(filled, "shalom") || !strings.Contains(filled, "boker") || !strings.Contains(filled, ">hello<") || !strings.Contains(filled, ">morning<") {
		t.Fatalf("cloze answer should show translit and gloss:\n%s", filled)
	}
	if !strings.Contains(filled, `/audio/`+phraseID+`.mp3`) {
		t.Fatalf("cloze answer should play the sentence:\n%s", filled)
	}
}

// A recall of several words still asks for one word: the first new word in
// the line. The question names that word, the other words are shown with
// their sound and meaning, and the buttons are word glosses of words that
// are not already on screen. A following recall of that word alone keeps
// the single-word question.
func TestMultiTokenRecallNamesTheTarget(t *testing.T) {
	const (
		ani = "אֲנִי"
		gar = "גָּר"
		po  = "פֹּה"
	)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock(now))
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
		{TextNFC: ani, Gloss: "I", Translit: sp("ani"), POS: sp("pron")},
		{TextNFC: gar, Gloss: "lives", Translit: sp("gar"), POS: sp("verb")},
		{TextNFC: po, Gloss: "here", Translit: sp("po"), POS: sp("adv")},
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]int64{}
	for _, row := range saved {
		ids[row.TextNFC] = row.ID
	}
	line := ani + " " + gar + " " + po
	unitID := putLesson(t, store, nil, units.Unit{
		Title: "At home", Situation: "Saying where you live.", Position: 1,
		NewWords: []string{gar},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityRecall, Text: line},
			{Type: units.ActivityRecall, Text: gar},
		}}},
	}, "shared")
	publish(t, store, unitID)

	phraseID := strings.Repeat("ab", 32)
	aniID := strings.Repeat("cd", 32)
	garID := strings.Repeat("ef", 32)
	poID := strings.Repeat("a1", 32)
	for _, asset := range []db.Asset{
		{ID: phraseID, TextNFC: units.Normalize(line), Voice: "voice", Model: "eleven_v4", Bytes: 4},
		{ID: aniID, TextNFC: ani, Voice: "voice", Model: "eleven_v4", Bytes: 4},
		{ID: garID, TextNFC: gar, Voice: "voice", Model: "eleven_v4", Bytes: 4},
		{ID: poID, TextNFC: po, Voice: "voice", Model: "eleven_v4", Bytes: 4},
	} {
		if _, err := store.InsertAudio(ctx, asset); err != nil {
			t.Fatal(err)
		}
	}

	svc, err := auth.New(auth.Config{
		PublicOrigin: "https://learn.example", Clock: fixedClock(now), Store: store,
		AllowedEmails: []string{"brian@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewOptions(nil, svc, Options{Store: store})
	cookie := mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour))
	path := "/lesson/" + itoa(unitID) + "/0"

	body := getAuthed(t, h, cookie, path)
	ask := activityHTML(t, body)
	head, choices, ok := strings.Cut(ask, `class="choices"`)
	if !ok {
		t.Fatalf("recall missing choices:\n%s", ask)
	}
	// English runs stay in their own bdi, the way every other prompt is isolated.
	// The word itself is the spec's <bdi lang=he dir=rtl>.
	prompt := `<bdi lang="en" dir="ltr">What does </bdi><bdi class="recall-prompt" lang="he" dir="rtl">` + gar + `</bdi><bdi lang="en" dir="ltr"> mean?</bdi>`
	if !strings.Contains(head, prompt) || strings.Contains(ask, "What does this Hebrew mean?") {
		t.Fatalf("prompt = %q, want the target word named:\n%s", prompt, head)
	}
	if strings.Count(head, `class="target"`) != 1 || strings.Contains(head, "data-target") || strings.Contains(head, "This word") {
		t.Fatalf("the target word is not the one mark:\n%s", head)
	}
	if !strings.Contains(head, `<mark class="target"><bdi class="wb__he" lang="he" dir="rtl">`+gar+`</bdi></mark>`) {
		t.Fatalf("the mark does not wrap only the target word:\n%s", head)
	}
	if !strings.Contains(head, `class="word-blocks word-blocks--recall" dir="rtl"`) {
		t.Fatalf("the line is not a centred recall row:\n%s", head)
	}
	if !strings.Contains(head, `class="target-slot"`) || !strings.Contains(head, ">?</span>") {
		t.Fatalf("the target is missing its dashed slot:\n%s", head)
	}
	for _, help := range []string{">ani<", ">I<", ">po<", ">here<", ani, po} {
		if !strings.Contains(head, help) {
			t.Errorf("help word missing %s", help)
		}
	}
	for _, hidden := range []string{">gar<", ">lives<"} {
		if strings.Contains(head, hidden) {
			t.Errorf("the tested word shows %s before the answer", hidden)
		}
	}
	if !strings.Contains(head, gar) {
		t.Fatal("the tested word's Hebrew is hidden")
	}
	if !strings.Contains(choices, ">lives<") {
		t.Errorf("choice missing the target gloss")
	}
	// The other words' glosses are on screen as help. Offering them as
	// buttons would give the answer away. This unit has no other words,
	// so the only button left is the target.
	for _, gloss := range []string{">I<", ">here<"} {
		if strings.Contains(choices, gloss) {
			t.Errorf("choice equals a visible helper gloss %s", gloss)
		}
	}
	if strings.Contains(choices, `c-answer__key" aria-hidden="true">1<`) || strings.Contains(choices, `c-answer__key" aria-hidden="true">2<`) {
		t.Fatalf("lesson choices still show key badges:\n%s", choices)
	}
	if strings.Contains(choices, "I live here") || strings.Contains(choices, `value="`+gar+`"`) || strings.Contains(choices, `value="`+line+`"`) {
		t.Fatalf("choices are not word glosses:\n%s", choices)
	}
	if !strings.Contains(choices, `value="`+itoa(ids[gar])+`"`) {
		t.Fatalf("the target choice is not the lexeme id:\n%s", choices)
	}
	for _, want := range []string{
		`aria-label="Play the line"`,
		`aria-label="Play ` + ani + `"`,
		`aria-label="Play ` + gar + `"`,
		`aria-label="Play ` + po + `"`,
		`id="line-clip"`,
		`id="word-clip-0"`,
		`id="word-clip-1"`,
		`id="word-clip-2"`,
		`class="orb orb--hero"`,
		`class="orb orb--word"`,
		`type="button"`,
		`preload="none"`,
		` controls`,
		`data-src="/audio/` + phraseID + `.mp3"`,
		`/static/js/clips.js`,
	} {
		if !strings.Contains(ask, want) {
			t.Errorf("clip missing %s", want)
		}
	}
	// The handler is an HTTP document, so this is the no-JS baseline: the
	// native audio is in the markup, and the script is not required to see it.
	if strings.Count(ask, "<audio ") != 4 || strings.Count(ask, `preload="none"`) != 4 {
		t.Fatalf("native audio is not present for every clip without JS:\n%s", ask)
	}
	assertOneClipControl(t, ask)
	if strings.Contains(ask, "onclick=") || strings.Contains(ask, "hx-on") || strings.Count(ask, "/static/js/clips.js") != 1 {
		t.Fatalf("clip markup:\n%s", ask)
	}
	if strings.Count(ask, `class="orb orb--word"`) != 3 || strings.Count(ask, `class="orb orb--hero"`) != 1 {
		t.Fatalf("orbs: hero %d word %d, want 1 and 3", strings.Count(ask, `class="orb orb--hero"`), strings.Count(ask, `class="orb orb--word"`))
	}

	runID := runValue(t, body)
	revealed := activityHTML(t, postLesson(t, h, cookie, path, runID, itoa(ids[gar]), true, "brian-csrf").Body.String())
	for _, shown := range []string{">ani<", ">gar<", ">po<", ">I<", ">lives<", ">here<", `class="target"`, `target-slot--filled`} {
		if !strings.Contains(revealed, shown) {
			t.Errorf("after the answer, missing %s", shown)
		}
	}

	alone := activityHTML(t, postLesson(t, h, cookie, path, runID, "continue", true, "brian-csrf").Body.String())
	if !strings.Contains(alone, "What does this Hebrew mean?") || strings.Contains(alone, "What does <bdi") || strings.Contains(alone, "data-target") || strings.Contains(alone, `class="target"`) || strings.Contains(alone, ">gar<") {
		t.Fatalf("single-word recall changed:\n%s", alone)
	}
	if !strings.Contains(alone, gar) || !strings.Contains(alone, ">lives<") || !strings.Contains(alone, `aria-label="Play `+gar+`"`) {
		t.Fatalf("single-word recall missing the word, its gloss, or its play button:\n%s", alone)
	}
	assertOneClipControl(t, alone)
	if strings.Contains(alone, ani) || strings.Contains(alone, po) {
		t.Fatalf("single-word recall shows the rest of the line:\n%s", alone)
	}
}

// A multi-word recall and a cloze both show the line's other words as help.
// Those glosses, and a different lexeme with the same gloss, are not choices.
// The four buttons are the target plus same-part-of-speech words from this
// unit and the earlier learned set. Another part of speech is left out.
func TestChoicesSkipVisibleHelperGlosses(t *testing.T) {
	const (
		ani    = "אֲנִי"
		anokhi = "אָנֹכִי"
		gar    = "גָּר"
		po     = "פֹּה"
		kotev  = "כּוֹתֵב"
		nimtsa = "נִמְצָא"
		okhel  = "אוֹכֵל"
		shoteh = "שׁוֹתֶה"
		mayim  = "מַיִם"
	)
	for _, typ := range []string{units.ActivityRecall, units.ActivityCloze} {
		t.Run(typ, func(t *testing.T) {
			now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
			ctx := context.Background()
			store, err := db.Open(ctx, filepath.Join(t.TempDir(), "hebrew.sqlite"), fixedClock(now))
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
				{TextNFC: ani, Gloss: "I", Translit: sp("ani"), POS: sp("pron")},
				{TextNFC: anokhi, Gloss: "I", Translit: sp("anokhi"), POS: sp("pron")},
				{TextNFC: gar, Gloss: "lives", Translit: sp("gar"), POS: sp("verb")},
				{TextNFC: po, Gloss: "here", Translit: sp("po"), POS: sp("adv")},
				{TextNFC: kotev, Gloss: "writes", Translit: sp("kotev"), POS: sp("verb")},
				{TextNFC: nimtsa, Gloss: "here", Translit: sp("nimtsa"), POS: sp("verb")},
				{TextNFC: okhel, Gloss: "eats", Translit: sp("okhel"), POS: sp("verb")},
				{TextNFC: shoteh, Gloss: "drinks", Translit: sp("shoteh"), POS: sp("verb")},
				{TextNFC: mayim, Gloss: "water", Translit: sp("mayim"), POS: sp("noun")},
			})
			if err != nil {
				t.Fatal(err)
			}
			ids := map[string]int64{}
			for _, row := range saved {
				ids[row.TextNFC] = row.ID
			}
			glossByID := map[string]string{}
			for text, id := range ids {
				switch text {
				case ani, anokhi:
					glossByID[itoa(id)] = "I"
				case gar:
					glossByID[itoa(id)] = "lives"
				case po:
					glossByID[itoa(id)] = "here"
				case kotev:
					glossByID[itoa(id)] = "writes"
				case nimtsa:
					glossByID[itoa(id)] = "here"
				case okhel:
					glossByID[itoa(id)] = "eats"
				case shoteh:
					glossByID[itoa(id)] = "drinks"
				case mayim:
					glossByID[itoa(id)] = "water"
				}
			}
			line := ani + " " + gar + " " + po
			earlierID := putLesson(t, store, nil, units.Unit{
				Title: "Earlier", Position: 0, NewWords: []string{okhel, shoteh},
				Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
					{Type: units.ActivityRecall, Text: okhel + " " + shoteh},
				}}},
			}, "shared")
			// anokhi shares the helper gloss "I" and is a different lexeme.
			// nimtsa is the target's part of speech but glosses "here", which
			// is on screen. kotev is the only other verb in this unit, so two
			// buttons come from the earlier learned set. mayim is another
			// part of speech and must not fill a button.
			unitID := putLesson(t, store, nil, units.Unit{
				Title: "At home", Situation: "Saying where you live.", Position: 1,
				NewWords: []string{gar},
				Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
					{Type: typ, Text: line},
					{Type: units.ActivityRecall, Text: anokhi},
					{Type: units.ActivityRecall, Text: kotev},
					{Type: units.ActivityRecall, Text: nimtsa},
					{Type: units.ActivityRecall, Text: mayim},
				}}},
			}, "shared")
			publish(t, store, earlierID, unitID)
			when := db.FormatTime(now)
			if _, err := store.ExecContext(ctx, `
				INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at, finished_at, score)
				VALUES (?, ?, 0, ?, ?, 1)`, brian.ID, earlierID, when, when); err != nil {
				t.Fatal(err)
			}

			svc, err := auth.New(auth.Config{
				PublicOrigin: "https://learn.example", Clock: fixedClock(now), Store: store,
				AllowedEmails: []string{"brian@example.com"},
			})
			if err != nil {
				t.Fatal(err)
			}
			h := NewOptions(nil, svc, Options{Store: store})
			cookie := mintSession(t, store, brian.ID, "brian-csrf", now.Add(time.Hour))
			body := getAuthed(t, h, cookie, "/lesson/"+itoa(unitID)+"/0")
			ask := activityHTML(t, body)
			head, choiceHTML, ok := strings.Cut(ask, `class="choices"`)
			if !ok {
				t.Fatalf("missing choices:\n%s", ask)
			}
			for _, help := range []string{ani, po} {
				if !strings.Contains(head, help) {
					t.Errorf("helper word %s is not on screen", help)
				}
			}
			if typ == units.ActivityRecall {
				for _, gloss := range []string{">I<", ">here<"} {
					if !strings.Contains(head, gloss) {
						t.Errorf("helper gloss %s is not visible", gloss)
					}
				}
			}
			helperGloss := map[string]bool{"i": true, "here": true}
			for _, gloss := range []string{">I<", ">here<"} {
				if strings.Contains(choiceHTML, gloss) {
					t.Errorf("choice equals a visible helper gloss %s\n%s", gloss, choiceHTML)
				}
			}
			got := choiceValues(t, ask)
			if len(got) != lessonWideChoices {
				t.Fatalf("buttons = %d, want %d: %v\n%s", len(got), lessonWideChoices, got, choiceHTML)
			}
			for _, id := range got {
				gloss := glossByID[id]
				if helperGloss[glossKey(gloss)] {
					t.Errorf("choice %s gloss %q equals a visible helper gloss", id, gloss)
				}
				if id == itoa(ids[ani]) || id == itoa(ids[po]) || id == itoa(ids[anokhi]) || id == itoa(ids[nimtsa]) || id == itoa(ids[mayim]) {
					t.Errorf("choice is a line word, the same gloss, or another part of speech: %s %s", id, gloss)
				}
			}
			for _, text := range []string{gar, kotev, okhel, shoteh} {
				if !slicesContains(got, itoa(ids[text])) {
					t.Errorf("missing same-POS choice %s (%s): %v", text, glossByID[itoa(ids[text])], got)
				}
			}
		})
	}
}

// assertOneClipControl checks the no-JS document. Each lesson clip renders
// exactly one native audio[controls] and exactly one button.orb. The audio
// is not hidden, so the bar is the play control. The orb stays in the markup
// and CSS keeps it display:none until html.js.
func assertOneClipControl(t *testing.T, html string) {
	t.Helper()
	clips := regexp.MustCompile(`(?s)<div class="clip(?: clip--hero)?">.*?</div>`).FindAllString(html, -1)
	if len(clips) == 0 {
		t.Fatalf("no lesson clips:\n%s", html)
	}
	audioTag := regexp.MustCompile(`<audio\b[^>]*>`)
	buttonTag := regexp.MustCompile(`<button\b[^>]*>`)
	hiddenAttr := regexp.MustCompile(`(?:\s)hidden(?:[\s=/>]|$)`)
	for _, block := range clips {
		audios := audioTag.FindAllString(block, -1)
		buttons := buttonTag.FindAllString(block, -1)
		if len(audios) != 1 || len(buttons) != 1 {
			t.Fatalf("clip has %d audio and %d buttons, want one of each:\n%s", len(audios), len(buttons), block)
		}
		if !strings.Contains(audios[0], "controls") || hiddenAttr.MatchString(audios[0]) {
			t.Fatalf("no-JS bar is not the single play control:\n%s", audios[0])
		}
		if !strings.Contains(buttons[0], `class="orb`) || hiddenAttr.MatchString(buttons[0]) {
			t.Fatalf("clip button is not an unhidden orb:\n%s", buttons[0])
		}
	}
}

// assertClipFallbackCSS checks the two modes. With html.js the native bar is
// display:none (not opacity or off-screen). Without it the bar stays shown
// and button.orb is display:none until html.js reveals it.
func assertClipFallbackCSS(t *testing.T, css string) {
	t.Helper()
	bar := cssRuleBody(t, css, ".clip > audio")
	if !regexp.MustCompile(`(?i)display\s*:\s*block\b`).MatchString(bar) || regexp.MustCompile(`(?i)display\s*:\s*none\b`).MatchString(bar) {
		t.Fatalf("no-JS native bar is not shown:\n%s", bar)
	}
	hiddenBar := cssRuleBody(t, css, "html.js .clip > audio")
	if !regexp.MustCompile(`(?i)display\s*:\s*none\b`).MatchString(hiddenBar) {
		t.Fatalf("with JS the native bar is not display:none:\n%s", hiddenBar)
	}
	for _, bad := range []string{"opacity", "position", "clip-path", "visibility", "inset", "translate", "1px", "z-index"} {
		if strings.Contains(strings.ToLower(hiddenBar), bad) {
			t.Errorf("with JS the native bar is hidden by %s:\n%s", bad, hiddenBar)
		}
	}
	orb := cssRuleBody(t, css, ".orb")
	if !regexp.MustCompile(`(?i)display\s*:\s*none\b`).MatchString(orb) {
		t.Fatalf("the orb is focusable without JS:\n%s", orb)
	}
	shown := cssRuleBody(t, css, "html.js .orb")
	if regexp.MustCompile(`(?i)display\s*:\s*none\b`).MatchString(shown) || !regexp.MustCompile(`(?i)display\s*:\s*grid\b`).MatchString(shown) {
		t.Fatalf("html.js does not show the orb:\n%s", shown)
	}
}

// assertRecallLineCSS checks the review of the recall grid. Text rows grow,
// the orb row grows to the native bar, five words wrap because only four
// tracks fit on a phone, and the wide gap is the 16px space token.
func assertRecallLineCSS(t *testing.T, css string) {
	t.Helper()
	grid := cssRuleBody(t, css, ".word-blocks--recall")
	for _, need := range []string{
		"repeat(4, fit-content(7rem))",
		"minmax(1lh, auto)",
		"minmax(var(--size-tap), auto)",
		"justify-content: center",
	} {
		if !strings.Contains(grid, need) {
			t.Errorf("recall grid missing %s:\n%s", need, grid)
		}
	}
	if strings.Contains(grid, "grid-auto-flow: column") {
		t.Fatal("the recall line still flows in columns, so a long line cannot wrap")
	}
	if !strings.Contains(css, "repeat(8, fit-content(7rem))") {
		t.Fatal("a wide viewport still wraps a five-word line")
	}
	wide := css[strings.Index(css, "@media (min-width: 1440px)"):]
	wide = wide[:strings.Index(wide, "}")+1]
	if !strings.Contains(wide, "column-gap: var(--space-4)") {
		t.Fatalf("1440 column gap is not --space-4:\n%s", wide)
	}
	hover := cssRuleBody(t, css, "html.js .orb--word:hover::before")
	if !strings.Contains(hover, "var(--surface-raised)") || !strings.Contains(hover, "border-color: var(--text)") {
		t.Fatalf("word orb hover has no colour change:\n%s", hover)
	}
	bars := cssRuleBody(t, css, ".word-blocks--recall .clip > audio")
	if strings.Contains(bars, "16rem") {
		t.Fatalf("recall bars still use the 16rem width:\n%s", bars)
	}
}

// cssRuleBody returns the declaration block of the rule whose selector is
// exactly selector. A longer selector that merely contains it is skipped.
func cssRuleBody(t *testing.T, css, selector string) string {
	t.Helper()
	needle := selector + " {"
	from := 0
	for from < len(css) {
		i := strings.Index(css[from:], needle)
		if i < 0 {
			break
		}
		i += from
		if i == 0 || cssRuleBoundary(css[i-1]) {
			start := i + len(needle)
			end := strings.Index(css[start:], "}")
			if end < 0 {
				t.Fatalf("rule %s is unclosed", selector)
			}
			return css[start : start+end]
		}
		from = i + len(needle)
	}
	t.Fatalf("missing rule %s", selector)
	return ""
}

func cssRuleBoundary(b byte) bool {
	return b == '\n' || b == '\r' || b == '{' || b == '}' || b == ',' || b == '/'
}

func slicesContains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestClipScript(t *testing.T) {
	b := readStatic(t, "static/js/clips.js")
	if strings.TrimSpace(b) == "" {
		t.Fatal("clips.js is empty")
	}
	if body := get(t, New(nil, nil), "/static/js/clips.js"); body != b {
		t.Fatal("served clips.js does not match the file")
	}
	for _, bad := range []string{"eval(", "new Function", "innerHTML", "document.write", "onclick", "requestAnimationFrame", ".animate(", "hx-on"} {
		if strings.Contains(b, bad) {
			t.Errorf("clips.js uses %s", bad)
		}
	}
	for _, need := range []string{
		`classList.add("js")`, `querySelectorAll("audio")`, ".pause()", `closest(".orb")`, "150",
		`Audio unavailable, tap to retry`, `aria-pressed`, `aria-busy`,
		"AbortError", "NotAllowedError", "audio.load", `setAttribute("hidden"`,
		`addEventListener("click"`, `addEventListener("htmx:afterSwap"`, `addEventListener("htmx:afterSettle"`,
	} {
		if !strings.Contains(b, need) {
			t.Errorf("clips.js missing %s", need)
		}
	}
	dlg := readStatic(t, "static/js/dialogue.js")
	if !strings.Contains(dlg, `querySelectorAll("audio")`) {
		t.Fatal("playing a dialogue line does not stop other audio")
	}
	css := readStatic(t, "static/css/base.css")
	assertClipFallbackCSS(t, css)
	assertRecallLineCSS(t, css)
	if !strings.Contains(css, ".orb--hero") || !strings.Contains(css, "var(--size-cta)") || !strings.Contains(css, "var(--size-tap)") || !strings.Contains(css, "inline-size: 28px") {
		t.Fatal("hero and word orb sizes are missing")
	}
	if strings.Contains(css, ".clip__label") || strings.Contains(css, "var(--primary)") {
		t.Fatal("the lesson orb uses a removed token")
	}
	if hero := css[strings.Index(css, ".orb--hero"):]; strings.Contains(hero[:strings.Index(hero, ".orb--word")], "var(--action-bg)") || strings.Contains(hero[:strings.Index(hero, ".orb--word")], "var(--lamp)") {
		t.Fatal("the lesson orb is gold")
	}
	if !strings.Contains(css, `outline: 2px solid var(--focus-ring)`) || !strings.Contains(css, ".word-blocks--recall") || !strings.Contains(css, "justify-content: center") || !strings.Contains(css, ".recall-prompt") || !strings.Contains(css, "padding-block: .1em") || !strings.Contains(css, ".choices .c-answer__key") {
		t.Fatal("recall line, focus ring, or choice badges drifted")
	}
	motion := readStatic(t, "static/css/motion.css")
	for _, need := range []string{"orb-spin", "orb-ring", "slot-fill", `prefers-reduced-motion: reduce`, ".orb__glyph--dots", `background: var(--focus-ring)`, `color: var(--on-ink)`, `outline: 3px solid var(--focus-ring)`} {
		if !strings.Contains(motion, need) {
			t.Errorf("motion.css missing %s", need)
		}
	}
	if strings.Contains(motion, `outline: 2px solid var(--focus-ring)`) {
		t.Fatal("reduced-motion playing still uses the focus outline")
	}
	orb := readStatic(t, "static/css/components.css")
	if !strings.Contains(orb, ".c-orb") || !strings.Contains(orb, "var(--size-cta)") {
		t.Fatal("the dialogue orb was restyled")
	}
}

// TestClipPlayAfterSwap runs clips.js under Node. Tapping a second orb while
// the first play() is still starting rejects that promise with AbortError or
// NotAllowedError, and the first orb must stay out of the error state. A retry
// from error calls load() before play(). An htmx:afterSwap hides audio that
// arrived in the swap. Click handling is delegated, so the new orb plays too.
func TestClipPlayAfterSwap(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal("node is required to run clips.js")
	}
	script := filepath.Join(t.TempDir(), "clips-check.js")
	if err := os.WriteFile(script, []byte(clipPlayCheckJS), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", script, filepath.Join("static", "js", "clips.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("clips.js behaviour: %v\n%s", err, out)
	}
}

const clipPlayCheckJS = `
const fs = require("fs");
const vm = require("vm");
const src = fs.readFileSync(process.argv[2], "utf8");
const fails = [];
function check(cond, msg) { if (!cond) fails.push(msg); }

function makeEl(tag) {
  const el = {
    tagName: String(tag).toUpperCase(),
    attrs: {},
    kids: [],
    parent: null,
    isConnected: true,
    className: "",
    tabIndex: 0,
    paused: true,
    currentTime: 0,
    ended: false,
    loads: 0,
    order: [],
    plays: [],
  };
  el.classList = { add(c) { el.className = (el.className + " " + c).trim(); } };
  el.setAttribute = (k, v) => { el.attrs[k] = String(v); };
  el.getAttribute = (k) => (Object.prototype.hasOwnProperty.call(el.attrs, k) ? el.attrs[k] : null);
  el.hasAttribute = (k) => Object.prototype.hasOwnProperty.call(el.attrs, k);
  el.removeAttribute = (k) => { delete el.attrs[k]; };
  el.append = (child) => { child.parent = el; el.kids.push(child); };
  el.closest = (sel) => { let n = el; while (n) { if (matches(n, sel)) return n; n = n.parent; } return null; };
  el.querySelector = (sel) => query(el, sel)[0] || null;
  el.pause = () => { el.paused = true; };
  el.load = () => { el.loads++; el.order.push("load"); };
  el.play = () => {
    el.order.push("play");
    const box = {};
    const p = { then(ok, bad) { box.ok = ok; box.bad = bad; return p; } };
    el.plays.push(box);
    el.paused = false;
    return p;
  };
  return el;
}
function hasClass(el, name) { return (el.className || "").split(/\s+/).indexOf(name) >= 0; }
function matches(el, sel) {
  if (sel === "audio") return el.tagName === "AUDIO";
  if (sel === ".orb") return hasClass(el, "orb");
  if (sel === ".clip") return hasClass(el, "clip");
  return false;
}
function query(root, sel) {
  const out = [];
  (function rec(n) {
    if (sel === ".clip > audio" && hasClass(n, "clip")) {
      n.kids.forEach((k) => { if (k.tagName === "AUDIO") out.push(k); });
    } else if (n !== root && matches(n, sel)) out.push(n);
    n.kids.forEach(rec);
  })(root);
  return out;
}

const listeners = [];
const doc = makeEl("#document");
doc.documentElement = makeEl("html");
doc.append(doc.documentElement);
doc.addEventListener = (name, fn) => { listeners.push({ name, fn }); };
doc.querySelectorAll = (sel) => query(doc, sel);
function fire(name, target) {
  listeners.filter((l) => l.name === name).forEach((l) => l.fn({ target, preventDefault() {} }));
}

const windowMock = {
  document: doc,
  setTimeout(fn) { return 1; },
  clearTimeout() {},
};
windowMock.window = windowMock;
vm.runInNewContext(src, { window: windowMock });
check(doc.documentElement.className.indexOf("js") >= 0, "html did not gain class js");

function addClip() {
  const wrap = makeEl("div");
  wrap.className = "clip";
  const audio = makeEl("audio");
  const orb = makeEl("button");
  orb.className = "orb";
  orb.setAttribute("data-src", "/a.mp3");
  orb.setAttribute("data-state", "idle");
  orb.setAttribute("data-label", "Play word");
  wrap.append(audio);
  wrap.append(orb);
  doc.append(wrap);
  return { audio, orb };
}
const a = addClip();
const b = addClip();
fire("click", a.orb);
check(a.audio.plays.length === 1, "first tap did not play");
fire("click", b.orb);
check(a.audio.paused === true, "starting A was not paused");
a.audio.plays[0].bad({ name: "AbortError" });
check(a.orb.getAttribute("data-state") !== "error", "AbortError put A into error: " + a.orb.getAttribute("data-state"));
const c = addClip();
fire("click", c.orb);
c.audio.plays[0].bad({ name: "NotAllowedError" });
check(c.orb.getAttribute("data-state") !== "error", "NotAllowedError put the orb into error");
const d = addClip();
fire("click", d.orb);
d.audio.plays[0].bad({ name: "NotSupportedError" });
check(d.orb.getAttribute("data-state") === "error", "a real play failure did not reach error");

a.orb.setAttribute("data-state", "error");
const before = a.audio.order.length;
fire("click", a.orb);
const tail = a.audio.order.slice(before);
check(tail[0] === "load" && tail[1] === "play", "retry did not load before play: " + tail.join(","));

const swapped = addClip();
fire("htmx:afterSwap", doc);
check(swapped.audio.getAttribute("hidden") !== null, "afterSwap did not set hidden on the new bar");
check(swapped.audio.getAttribute("aria-hidden") === "true", "afterSwap did not hide the new bar");
check(swapped.audio.tabIndex === -1, "afterSwap left the new bar tabbable");
const playsBefore = swapped.audio.plays.length;
fire("click", swapped.orb);
check(swapped.audio.plays.length === playsBefore + 1, "a swapped orb did not play through the document listener");

vm.runInNewContext(src, { window: windowMock });
const plays = b.audio.plays.length;
fire("click", b.orb);
check(b.audio.plays.length === plays + 1, "loading the script again doubled the click listener");

if (fails.length) {
  console.error(fails.join("\n"));
  process.exit(1);
}
`

func TestLessonPostRequiresCSRF(t *testing.T) {
	h := newMux(nil, CSRFFunc(func(*http.Request) string { return "tok" }))
	req := httptest.NewRequest(http.MethodPost, "/lesson/1/0", strings.NewReader(""))
	req.Host = "learn.example"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("post without a token = %d", rec.Code)
	}
}

func putLesson(t *testing.T, store *db.Store, id *int64, doc units.Unit, scope string) int64 {
	t.Helper()
	res, err := store.PutUnit(context.Background(), id, doc, scope, "test")
	if err != nil {
		t.Fatal(err)
	}
	return res.UnitID
}

func publish(t *testing.T, store *db.Store, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		if _, err := store.ExecContext(context.Background(), `UPDATE units SET status = 'published' WHERE id = ?`, id); err != nil {
			t.Fatal(err)
		}
	}
}

func mintSession(t *testing.T, store *db.Store, userID int64, csrf string, exp time.Time) *http.Cookie {
	t.Helper()
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(buf)
	if err := store.InsertSession(context.Background(), hex.EncodeToString(sum[:]), userID, csrf, exp); err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: "hebrew_session", Value: base64.RawURLEncoding.EncodeToString(buf)}
}

func getAuthed(t *testing.T, h http.Handler, cookie *http.Cookie, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "learn.example"
	req.AddCookie(cookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d\n%s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func codeAuthed(t *testing.T, h http.Handler, cookie *http.Cookie, path string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "learn.example"
	req.AddCookie(cookie)
	h.ServeHTTP(rec, req)
	return rec.Code
}

func postLesson(t *testing.T, h http.Handler, cookie *http.Cookie, path, runID, answer string, origin bool, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{}
	form.Set("run", runID)
	form.Set("answer", answer)
	if csrf != "" {
		form.Set("_csrf", csrf)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Host = "learn.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.AddCookie(cookie)
	if origin {
		req.Header.Set("Origin", "https://learn.example")
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func activityHTML(t *testing.T, body string) string {
	t.Helper()
	const marker = `id="activity"`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("activity missing:\n%s", body)
	}
	start := strings.LastIndex(body[:i], "<div")
	if start < 0 {
		t.Fatalf("activity div missing:\n%s", body)
	}
	depth := 0
	for j := start; j < len(body); j++ {
		switch {
		case strings.HasPrefix(body[j:], "<div"):
			depth++
		case strings.HasPrefix(body[j:], "</div>"):
			depth--
			if depth == 0 {
				return body[start : j+len("</div>")]
			}
		}
	}
	t.Fatalf("activity div is unclosed:\n%s", body[start:])
	return ""
}

func attrValues(html string) string {
	var b strings.Builder
	for _, m := range regexp.MustCompile(`=\s*"([^"]*)"`).FindAllStringSubmatch(html, -1) {
		b.WriteString(m[1])
		b.WriteByte('\n')
	}
	return b.String()
}

func containsHebrew(s string) bool {
	for _, r := range s {
		if r >= 0x0590 && r <= 0x05FF {
			return true
		}
	}
	return false
}

func runValue(t *testing.T, body string) string {
	t.Helper()
	m := regexp.MustCompile(`name="run" value="([0-9]+)"`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("run id missing:\n%s", body)
	}
	return m[1]
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}

func scalar(t *testing.T, path, query string, args ...any) int {
	t.Helper()
	var n int
	if err := sqlScalar(t, path, query, args, &n); err != nil {
		t.Fatal(err)
	}
	return n
}

func sqlScalar(t *testing.T, path, query string, args []any, dest any) error {
	t.Helper()
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return err
	}
	return conn.QueryRow(query, args...).Scan(dest)
}
