package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
	"github.com/a-h/templ"
)

// TestReviewAnkiReveal is the Anki contract for every self-graded review card.
// The front has the prompt and Show answer, and not the answer or the grades.
// The reveal, both as an htmx fragment and as a no-JS full page, has the answer,
// the grade buttons, and the clip. A grade before that reveal is rejected and
// writes no review_log row.
func TestReviewAnkiReveal(t *testing.T) {
	for _, kind := range []string{"read", "recall", "listen", "spell", "letter", "pair", "pattern"} {
		t.Run(kind, func(t *testing.T) {
			testAnkiKind(t, kind)
		})
	}
}

// TestReviewPromptFollowsTheKey locks the one-line questions. A pair is not a
// single letter, and a two-form agreement key is not a single form.
func TestReviewPromptFollowsTheKey(t *testing.T) {
	if views.LetterPrompt("ג") != views.PromptLetter || views.LetterPrompt("ג/פ") != views.PromptLetter {
		t.Fatalf("letter prompts = %q %q", views.LetterPrompt("ג"), views.LetterPrompt("ג/פ"))
	}
	if views.PatternPrompt("הוא/היא") != views.PromptPattern || views.PatternPrompt("he/she") != views.PromptPatternSay {
		t.Fatalf("pattern prompts = %q %q", views.PatternPrompt("הוא/היא"), views.PatternPrompt("he/she"))
	}
	dot, bare := "פ\u05BC", "פ"
	if got := views.DageshNote([]views.DrillSide{{Hebrew: dot, Sound: "p"}, {Hebrew: bare, Sound: "f"}}); got != "With the dot: p. Without: f." {
		t.Fatalf("dagesh note = %q", got)
	}
	if got := views.DageshNote([]views.DrillSide{{Hebrew: dot, Sound: ""}, {Hebrew: bare, Sound: "f"}}); got != "" {
		t.Fatalf("missing sound was filled in: %q", got)
	}
	if got := views.DageshNote([]views.DrillSide{{Hebrew: "ג", Sound: "g"}, {Hebrew: "פ", Sound: "p"}}); got != "" {
		t.Fatalf("unrelated pair produced a note: %q", got)
	}
	if views.LetterName("ג") != "גימל" || views.LetterName("פ") != "פא" || views.LetterName("X") != "" {
		t.Fatalf("letter names ג=%q פ=%q", views.LetterName("ג"), views.LetterName("פ"))
	}
}

// TestReviewModesShareOneRevealPath locks the plug-in. Each current kind is
// one table entry with a prompt, a front, and a back. ConcealFront is the
// hold for that entry. Match and picture match are not entries. The reveal
// route and the peeked grade guard are not per kind.
func TestReviewModesShareOneRevealPath(t *testing.T) {
	seen := map[string]int{}
	for _, mode := range views.ReviewModes {
		seen[mode.Kind]++
		if mode.Prompt == nil || mode.Front == nil || mode.Back == nil {
			t.Errorf("%s: a mode needs a prompt, a front, and a back", mode.Kind)
		}
		if mode.Prompt != nil && mode.Prompt("שלום") == "" {
			t.Errorf("%s: prompt is empty", mode.Kind)
		}
	}
	for _, kind := range []string{"read", "recall", "listen", "spell", "letter", "pattern"} {
		if seen[kind] != 1 {
			t.Errorf("%s registered %d times", kind, seen[kind])
		}
	}
	for _, kind := range []string{"match", "picture_match"} {
		if seen[kind] != 0 {
			t.Errorf("%s is registered; it is not a mode yet", kind)
		}
		if _, ok := views.LookupReviewMode(kind); ok {
			t.Errorf("lookup found %s", kind)
		}
	}
	for _, kind := range []string{"read", "recall", "listen", "spell"} {
		mode, ok := views.LookupReviewMode(kind)
		if !ok || !mode.Lexeme {
			t.Errorf("%s should load as a lexeme card", kind)
		}
	}
	for _, kind := range []string{"letter", "pattern"} {
		mode, ok := views.LookupReviewMode(kind)
		if !ok || mode.Lexeme {
			t.Errorf("%s should load from its key", kind)
		}
	}
	if views.ReviewModeFor("letter").Prompt("ג/פ") != views.PromptLetter {
		t.Fatal("letter mode prompt drifted from PromptLetter")
	}
	if views.ReviewModeFor("pattern").Prompt("הוא/היא") != views.PromptPattern || views.ReviewModeFor("pattern").Prompt("he/she") != views.PromptPatternSay {
		t.Fatal("pattern mode prompt drifted")
	}

	recall := views.ReviewCard{
		Kind: "recall", Pointed: "שלום", Unpointed: "שלום",
		Translit: "shalom", AudioURL: "/audio/x.mp3", Gloss: "hello", Gender: "m",
	}
	hidden := views.ConcealFront(recall)
	if hidden.Pointed != "" || hidden.Unpointed != "" || hidden.Translit != "" || hidden.AudioURL != "" || hidden.Gender != "" {
		t.Fatalf("recall front kept the answer: %+v", hidden)
	}
	if hidden.Gloss != "hello" {
		t.Fatalf("recall front gloss = %q", hidden.Gloss)
	}
	recall.Revealed = true
	if shown := views.ConcealFront(recall); shown.Pointed != "שלום" || shown.AudioURL != "/audio/x.mp3" || shown.Gloss != "hello" {
		t.Fatalf("revealed recall was concealed: %+v", shown)
	}

	read := views.ReviewCard{
		Kind: "read", Pointed: "שלום", Unpointed: "שלום",
		Translit: "shalom", AudioURL: "/audio/x.mp3", Gloss: "hello", Gender: "m",
	}
	readFront := views.ConcealFront(read)
	if readFront.Pointed != "שלום" || readFront.Translit != "shalom" || readFront.AudioURL != "/audio/x.mp3" {
		t.Fatalf("read front lost the prompt: %+v", readFront)
	}
	if readFront.Gloss != "" || readFront.Gender != "" {
		t.Fatalf("read front kept the gloss: %+v", readFront)
	}

	listen := views.ReviewCard{
		Kind: "listen", Pointed: "מַיִם", Unpointed: "מים",
		Translit: "mayim", AudioURL: "/audio/x.mp3", Gloss: "water", Gender: "m",
	}
	listenFront := views.ConcealFront(listen)
	if listenFront.Pointed != "מַיִם" || listenFront.Unpointed != "מים" || listenFront.Translit != "mayim" || listenFront.AudioURL != "/audio/x.mp3" {
		t.Fatalf("listen front lost the prompt: %+v", listenFront)
	}
	if listenFront.Gloss != "" || listenFront.Gender != "" {
		t.Fatalf("listen front kept the gloss: %+v", listenFront)
	}
	listen.Revealed = true
	if shown := views.ConcealFront(listen); shown.Gloss != "water" || shown.AudioURL != "/audio/x.mp3" {
		t.Fatalf("revealed listen was concealed: %+v", shown)
	}

	pattern := views.ReviewCard{
		Kind: "pattern", Pointed: "הוא/היא", Translit: "hu", AudioURL: "/audio/x.mp3", Gloss: "he", Gender: "m",
		Sides:       []views.DrillSide{{Hebrew: "הוא", Translit: "hu", Gloss: "he"}},
		ExampleText: "הוא", LooksLike: "looks", TellApart: "cue", PairNote: "note",
		DageshNote: "dot", DageshHard: "hard",
	}
	patternFront := views.ConcealFront(pattern)
	if patternFront.Pointed != "הוא/היא" {
		t.Fatalf("pattern front pointed = %q", patternFront.Pointed)
	}
	if patternFront.Translit != "" || patternFront.AudioURL != "" || patternFront.Gloss != "" || patternFront.Gender != "" ||
		len(patternFront.Sides) != 0 || patternFront.ExampleText != "" || patternFront.LooksLike != "" || patternFront.TellApart != "" || patternFront.PairNote != "" || patternFront.DageshNote != "" || patternFront.DageshHard != "" {
		t.Fatalf("pattern front kept the answer: %+v", patternFront)
	}
	pattern.Revealed = true
	if shown := views.ConcealFront(pattern); len(shown.Sides) != 1 || shown.Gloss != "he" || shown.AudioURL != "/audio/x.mp3" {
		t.Fatalf("revealed pattern was concealed: %+v", shown)
	}

	letter := views.ReviewCard{
		Kind: "letter", Pointed: "ג", Translit: "g", AudioURL: "/audio/x.mp3", Gloss: "gimel", Gender: "m",
		Sides:       []views.DrillSide{{Hebrew: "ג", Name: "גימל", Sound: "ga"}, {Hebrew: "פ", Name: "פא"}},
		ExampleText: "גשם", ExampleGloss: "rain", LooksLike: "looks", TellApart: "cue", PairNote: "final note",
		DageshNote: "With the dot: p. Without: f.",
		DageshHard: "At the start of a word, ב, כ, and פ are almost always hard (b, k, p).",
	}
	letterFront := views.ConcealFront(letter)
	if letterFront.Pointed != "ג" {
		t.Fatalf("letter front pointed = %q", letterFront.Pointed)
	}
	if letterFront.Gloss != "" || letterFront.Translit != "" || letterFront.AudioURL != "" || letterFront.Gender != "" ||
		len(letterFront.Sides) != 0 || letterFront.ExampleText != "" || letterFront.LooksLike != "" || letterFront.TellApart != "" || letterFront.PairNote != "" || letterFront.DageshNote != "" || letterFront.DageshHard != "" {
		t.Fatalf("letter front kept the answer: %+v", letterFront)
	}

	frontHTML := renderMode(t, views.ReviewModeFor("letter").Front(letter))
	if strings.Contains(frontHTML, "גימל") || strings.Contains(frontHTML, "פא") || strings.Contains(frontHTML, `name="rating"`) || strings.Contains(frontHTML, "act__answer") || strings.Contains(frontHTML, "Tell them apart") {
		t.Fatalf("letter front face rendered the answer: %s", frontHTML)
	}
	if !strings.Contains(frontHTML, "ג") {
		t.Fatalf("letter front face dropped the letter: %s", frontHTML)
	}
	backHTML := renderMode(t, views.ReviewModeFor("letter").Back(letter))
	if !strings.Contains(backHTML, "גימל") || !strings.Contains(backHTML, ">Answer</h2>") || !strings.Contains(backHTML, "גשם") || !strings.Contains(backHTML, "<b>Tell them apart:</b>") || !strings.Contains(backHTML, "cue") || !strings.Contains(backHTML, "final note") || !strings.Contains(backHTML, "With the dot: p. Without: f.") || !strings.Contains(backHTML, "almost always hard") {
		t.Fatalf("letter back face dropped the answer: %s", backHTML)
	}
	if strings.Contains(backHTML, `name="rating"`) {
		t.Fatalf("letter back face owns the grade buttons: %s", backHTML)
	}

	unknown := views.ConcealFront(views.ReviewCard{Kind: "picture_match", Pointed: "שלום", Gloss: "hello", AudioURL: "/audio/x.mp3"})
	if unknown.Pointed != "שלום" || unknown.AudioURL != "/audio/x.mp3" || unknown.Gloss != "" {
		t.Fatalf("unknown kind should use the read hold: %+v", unknown)
	}
}

func renderMode(t *testing.T, c templ.Component) string {
	t.Helper()
	var b strings.Builder
	if err := c.Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// The alef-bet names the letter back has to show. These are the NFC forms.
const (
	gimelName = "\u05d2\u05b4\u05bc\u05d9\u05de\u05b6\u05dc" // גִּימֶל
	feName    = "\u05e4\u05b5\u05d0"                         // פֵא
)

type ankiSpec struct {
	prompt  string
	front   []string // must be on the front
	banned  []string // answer text that must not be on the front
	answers []string // must be on the back
	clipOn  bool     // read and listen play the prompt clip before the reveal
	pair    []string // a letter pair: the front shows exactly one of these
	key     string
}

func testAnkiKind(t *testing.T, kind string) {
	t.Helper()
	store, h, now := openApp(t)
	spec, clipID, word := seedAnkiLexeme(t, store, now, kind)
	// openApp only admits a@, b@, and en@. Each subtest has its own store.
	hx := addLearner(t, store, "a@example.com", "sub-hx-"+kind, now)
	plain := addLearner(t, store, "en@example.com", "sub-plain-"+kind, now)
	for _, learner := range []learnerFix{hx, plain} {
		if kind == "letter" || kind == "pair" || kind == "pattern" {
			seedDrill(t, store, learner.user.ID, kind, spec.key, word.exampleID)
			continue
		}
		addCard(t, store, learner.user.ID, word.lexemeID, word.skill, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
	}

	front := getLearner(t, h, hx, "/review")
	assertAnkiFront(t, front, spec)
	if again := getLearner(t, h, hx, "/review"); attr(t, again, "data-pointed") != attr(t, front, "data-pointed") {
		t.Fatalf("refresh changed the prompt from %s to %s", attr(t, front, "data-pointed"), attr(t, again, "data-pointed"))
	}
	cardID := mustCardID(t, front)
	logsBefore := reviewLogCount(t, store)

	hidden := postGrade(t, h, hx, cardID, "3", "https://learn.example")
	if hidden.Code != http.StatusBadRequest {
		t.Fatalf("unrevealed grade = %d, want 400", hidden.Code)
	}
	if n := reviewLogCount(t, store); n != logsBefore {
		t.Fatalf("unrevealed grade wrote a review log, count %d want %d", n, logsBefore)
	}
	still := getLearner(t, h, hx, "/review")
	if attr(t, still, "data-card") != attr(t, front, "data-card") || strings.Contains(still, `data-revealed="true"`) || strings.Contains(still, `name="rating"`) {
		t.Fatalf("unrevealed grade moved the card: %s", cardAttrs(still))
	}

	shown := getReveal(t, h, hx, cardID)
	if shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d\n%s", shown.Code, shown.Body.String())
	}
	assertAnkiBack(t, shown.Body.String(), spec, clipID, false)

	plainFront := getLearner(t, h, plain, "/review")
	assertAnkiFront(t, plainFront, spec)
	plainID := mustCardID(t, plainFront)
	got := getRevealRaw(t, h, plain.cookie, plainID, false)
	if got.Code != http.StatusSeeOther || got.Header().Get("Location") != "/review" {
		t.Fatalf("plain reveal = %d location %q", got.Code, got.Header().Get("Location"))
	}
	page := getLearner(t, h, plain, "/review")
	assertAnkiBack(t, page, spec, clipID, true)
	if !strings.Contains(page, "<!doctype html>") {
		t.Fatal("plain reveal did not render the full page")
	}
	refresh := getLearner(t, h, plain, "/review")
	assertAnkiBack(t, refresh, spec, clipID, true)
}

type ankiWord struct {
	lexemeID  int64
	exampleID int64
	skill     string
}

// TestReviewDageshPairUsesStoredSounds puts a pe/fe pair on the back only when
// both letters have a stored reading. The front stays one letter and does not
// carry the contrast line.
func TestReviewDageshPairUsesStoredSounds(t *testing.T) {
	store, h, now := openApp(t)
	dot := "פ\u05BC"
	addLexeme(t, store, dot, "pe-sound", "p", "m", "")
	addLexeme(t, store, "פ", "fe-sound", "f", "m", "")
	learner := addLearner(t, store, "a@example.com", "sub-dagesh", now)
	seedDrill(t, store, learner.user.ID, "pair", dot+"/פ", 0)
	front := getLearner(t, h, learner, "/review")
	if strings.Contains(front, "With the dot") || strings.Contains(front, "pe-sound") || strings.Contains(front, "fe-sound") || strings.Contains(front, dot+"/פ") {
		t.Fatalf("dagesh answer leaked onto the front\n%s", front)
	}
	hasDot := strings.Contains(front, dot)
	hasBare := strings.Contains(front, "פ") && !hasDot
	if hasDot == hasBare {
		t.Fatalf("front should show exactly one of the dagesh pair\n%s", front)
	}
	shown := getReveal(t, h, learner, mustCardID(t, front))
	body := shown.Body.String()
	if shown.Code != http.StatusOK || !strings.Contains(body, "With the dot: p. Without: f.") || !strings.Contains(body, dot) || !strings.Contains(body, "פ") {
		t.Fatalf("dagesh back = %d\n%s", shown.Code, body)
	}
}

func seedAnkiLexeme(t *testing.T, store *db.Store, now time.Time, kind string) (ankiSpec, string, ankiWord) {
	t.Helper()
	spec := ankiSpec{clipOn: kind == "read" || kind == "listen"}
	var word ankiWord
	var clipText string
	switch kind {
	case "read":
		clipText = "שָׁלוֹם"
		spec.prompt = views.PromptRead
		spec.front = []string{clipText, spec.prompt, "read-tr"}
		spec.banned = []string{"read-meaning"}
		spec.answers = []string{"read-meaning", "read-tr", clipText}
		word.lexemeID = addLexeme(t, store, clipText, "read-meaning", "read-tr", "m", "")
		word.skill = fsrs.SkillRead
	case "recall":
		clipText = "כִּסֵּא"
		spec.prompt = views.PromptRecall
		spec.front = []string{"recall-meaning", spec.prompt}
		spec.banned = []string{clipText, "recall-tr", "/audio/"}
		spec.answers = []string{clipText, "recall-tr", "recall-meaning"}
		word.lexemeID = addLexeme(t, store, clipText, "recall-meaning", "recall-tr", "m", "")
		word.skill = fsrs.SkillRecall
	case "listen":
		clipText = "מַיִם"
		spec.prompt = views.PromptListen
		spec.front = []string{clipText, spec.prompt, "listen-tr"}
		spec.banned = []string{"listen-meaning"}
		spec.answers = []string{"listen-meaning", "listen-tr", clipText}
		word.lexemeID = addLexeme(t, store, clipText, "listen-meaning", "listen-tr", "m", "")
		word.skill = fsrs.SkillListen
	case "spell":
		clipText = "לֶחֶם"
		spec.prompt = views.PromptSpell
		spec.front = []string{"spell-meaning", spec.prompt}
		spec.banned = []string{clipText, "spell-tr", "/audio/"}
		spec.answers = []string{clipText, "spell-tr", "spell-meaning"}
		word.lexemeID = addLexeme(t, store, clipText, "spell-meaning", "spell-tr", "m", "")
		word.skill = fsrs.SkillSpell
	case "letter":
		clipText = "גֶּשֶׁם"
		spec.key = "ג"
		spec.prompt = views.PromptLetter
		spec.front = []string{"ג", spec.prompt}
		spec.banned = []string{views.LetterName("ג"), gimelName, "gimel", "g as in go", "rain-shower", "geshem-ex", "/audio/", "ג/פ"}
		spec.answers = []string{gimelName, "gimel", "g as in go", "rain-shower", "geshem-ex", clipText}
		word.exampleID = addLexeme(t, store, clipText, "rain-shower", "geshem-ex", "m", "")
	case "pair":
		clipText = "גֶּשֶׁם"
		spec.key = "ג/פ"
		spec.pair = []string{"ג", "פ"}
		spec.prompt = views.PromptLetter
		spec.front = []string{spec.prompt}
		spec.banned = []string{views.LetterName("ג"), views.LetterName("פ"), gimelName, feName, "gimel", "g as in go", "f as in fun", "rain-shower", "geshem-ex", "/audio/", "ג/פ", "Tell them apart"}
		spec.answers = []string{gimelName, "gimel", "g as in go", feName, "fe", "f as in fun", "ג", "פ", "rain-shower", "geshem-ex", clipText}
		word.exampleID = addLexeme(t, store, clipText, "rain-shower", "geshem-ex", "m", "")
	case "pattern":
		clipText = "הוא"
		spec.key = "הוא/היא"
		spec.prompt = views.PromptPattern
		spec.front = []string{"הוא/היא", spec.prompt}
		spec.banned = []string{"masc-pronoun", "fem-pronoun", "hu-form", "hi-form", "/audio/"}
		spec.answers = []string{"masc-pronoun", "fem-pronoun", "hu-form", "hi-form", "הוא", "היא"}
		addLexeme(t, store, "הוא", "masc-pronoun", "hu-form", "m", "")
		addLexeme(t, store, "היא", "fem-pronoun", "hi-form", "f", "")
	default:
		t.Fatalf("unknown kind %s", kind)
	}
	return spec, seedAnkiClip(t, store, clipText, now), word
}

func seedDrill(t *testing.T, store *db.Store, userID int64, kind, key string, exampleID int64) {
	t.Helper()
	rev := fsrs.New(store, store.Clock())
	ctx := context.Background()
	var missKind string
	var lexemeID int64
	switch kind {
	case "letter", "pair":
		missKind, lexemeID = fsrs.MissLetterPair, exampleID
		if key == "" {
			t.Fatal("letter key was not stored")
		}
	case "pattern":
		missKind = fsrs.MissAgreement
		if key == "" {
			key = "הוא/היא"
		}
	default:
		return
	}
	for i := 0; i < session.MinMissesForDrill; i++ {
		if err := rev.RecordMiss(ctx, userID, missKind, key, lexemeID); err != nil {
			t.Fatal(err)
		}
	}
}

func seedAnkiClip(t *testing.T, store *db.Store, text string, now time.Time) string {
	t.Helper()
	sum := sha256.Sum256([]byte("review-anki:" + text))
	id := hex.EncodeToString(sum[:])
	if _, err := store.InsertAudio(context.Background(), db.Asset{
		ID: id, TextNFC: units.Normalize(text), Voice: "voice", Model: "eleven_v4", Bytes: 4, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func assertAnkiFront(t *testing.T, body string, spec ankiSpec) {
	t.Helper()
	for _, want := range spec.front {
		if !strings.Contains(body, want) {
			t.Fatalf("front missing %q\n%s", want, body)
		}
	}
	if !strings.Contains(body, "Show answer") || !strings.Contains(body, "autofocus") || !strings.Contains(body, `method="get"`) || !strings.Contains(body, `hx-get="/_hx/review/reveal"`) {
		t.Fatal("front has no autofocused GET Show answer")
	}
	if strings.Contains(body, "<details") {
		t.Fatal("show answer used details")
	}
	if len(spec.pair) == 2 {
		a, b := spec.pair[0], spec.pair[1]
		hasA, hasB := strings.Contains(body, a), strings.Contains(body, b)
		if hasA == hasB {
			t.Fatalf("front should show exactly one of %s/%s\n%s", a, b, body)
		}
	}
	if strings.Contains(body, `name="rating"`) || strings.Contains(body, "Forgot") || strings.Contains(body, "Got it") {
		t.Fatal("grade buttons rendered on the front")
	}
	assertNoAnswerHeading(t, body)
	for _, banned := range spec.banned {
		if banned != "" && strings.Contains(body, banned) {
			t.Fatalf("front contains answer %q", banned)
		}
	}
	if !spec.clipOn && strings.Contains(body, "/audio/") {
		t.Fatal("front played the answer clip")
	}
	if spec.clipOn && !strings.Contains(body, "/audio/") {
		t.Fatal("prompt clip missing")
	}
}

func assertAnkiBack(t *testing.T, body string, spec ankiSpec, clipID string, fullPage bool) {
	t.Helper()
	if !strings.Contains(body, `name="rating"`) || !strings.Contains(body, "Forgot") || !strings.Contains(body, "Easy") || !strings.Contains(body, `data-revealed="true"`) {
		t.Fatalf("back is missing the grades\n%s", body)
	}
	if !strings.Contains(body, `id="act-answer" tabindex="-1" autofocus`) || !strings.Contains(body, ">Answer</h2>") {
		t.Fatal("back has no autofocused answer heading")
	}
	for _, want := range spec.answers {
		if !strings.Contains(body, want) {
			t.Fatalf("back missing %q\n%s", want, body)
		}
	}
	assertReviewClip(t, body, clipID, fullPage)
}

func assertNoAnswerHeading(t *testing.T, body string) {
	t.Helper()
	if strings.Contains(body, `id="act-answer"`) || strings.Contains(body, "act__answer-h") {
		t.Fatalf("front rendered the answer heading\n%s", body)
	}
}

func getReveal(t *testing.T, h http.Handler, learner learnerFix, cardID int64) *httptest.ResponseRecorder {
	t.Helper()
	return getRevealRaw(t, h, learner.cookie, cardID, true)
}

func getRevealRaw(t *testing.T, h http.Handler, cookie *http.Cookie, cardID int64, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/_hx/review/reveal?card_id="+strconv.FormatInt(cardID, 10), nil)
	req.Host = "learn.example"
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func mustCardID(t *testing.T, body string) int64 {
	t.Helper()
	id, err := strconv.ParseInt(attr(t, body, "data-card"), 10, 64)
	if err != nil {
		t.Fatalf("card id: %v\n%s", err, cardAttrs(body))
	}
	return id
}

func reviewLogCount(t *testing.T, store *db.Store) int {
	t.Helper()
	n, err := store.CountReviewLogs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestReviewFrontHasNoAnswerHeading checks every mode's front on the full
// page and on the htmx fragment the next card swaps in. The answer heading
// belongs on the back.
func TestReviewFrontHasNoAnswerHeading(t *testing.T) {
	for _, kind := range []string{"read", "recall", "listen", "spell", "letter", "pair", "pattern"} {
		t.Run(kind, func(t *testing.T) {
			store, h, now := openApp(t)
			spec, _, word := seedAnkiLexeme(t, store, now, kind)
			hx := addLearner(t, store, "a@example.com", "sub-front-"+kind, now)
			if kind == "letter" || kind == "pair" || kind == "pattern" {
				seedDrill(t, store, hx.user.ID, kind, spec.key, word.exampleID)
			} else {
				addCard(t, store, hx.user.ID, word.lexemeID, word.skill, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
				extra := addLexeme(t, store, "סֵפֶר", kind+"-extra", kind+"-tr", "m", "")
				addCard(t, store, hx.user.ID, extra, word.skill, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
			}
			page := getLearner(t, h, hx, "/review")
			if !strings.Contains(page, "<!doctype html>") {
				t.Fatal("full page front was not a page")
			}
			assertNoAnswerHeading(t, page)
			id := mustCardID(t, page)
			if shown := getReveal(t, h, hx, id); shown.Code != http.StatusOK {
				t.Fatalf("reveal = %d\n%s", shown.Code, shown.Body.String())
			}
			next := postGrade(t, h, hx, id, "3", "https://learn.example")
			if next.Code != http.StatusOK {
				t.Fatalf("next front = %d\n%s", next.Code, next.Body.String())
			}
			body := next.Body.String()
			if strings.Contains(body, "<!doctype html>") {
				t.Fatal("htmx front rendered a full page")
			}
			assertNoAnswerHeading(t, body)
			if !strings.Contains(body, "Show answer") || strings.Contains(body, `name="rating"`) {
				t.Fatalf("htmx response was not the next front\n%s", body)
			}
		})
	}
}

// TestReviewPairAskFollowsTheDay keeps one letter across refreshes on the
// same study day, and asks both letters of the pair across days.
func TestReviewPairAskFollowsTheDay(t *testing.T) {
	start := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h, clk := openStepped(t, start)
	learner := addLearner(t, store, "a@example.com", "sub-pair-day", start.Add(400*24*time.Hour))
	seedDrill(t, store, learner.user.ID, "pair", "ג/פ", 0)

	first := ""
	for i := 0; i < 50; i++ {
		asked := attr(t, getLearner(t, h, learner, "/review"), "data-pointed")
		if asked != "ג" && asked != "פ" {
			t.Fatalf("refresh %d asked %q", i, asked)
		}
		if i == 0 {
			first = asked
			continue
		}
		if asked != first {
			t.Fatalf("refresh %d changed the letter from %s to %s", i, first, asked)
		}
	}
	seen := map[string]bool{first: true}
	for day := 1; day <= 40 && len(seen) < 2; day++ {
		clk.Advance(24 * time.Hour)
		seedDrill(t, store, learner.user.ID, "pair", "ג/פ", 0)
		asked := attr(t, getLearner(t, h, learner, "/review"), "data-pointed")
		if asked != "ג" && asked != "פ" {
			t.Fatalf("day %d asked %q", day, asked)
		}
		seen[asked] = true
	}
	if !seen["ג"] || !seen["פ"] {
		t.Fatalf("pair asked %v across days, want both letters", seen)
	}
}

// TestReviewRevealRejectsCrossSite rejects a GET reveal that the browser
// marks cross-site, or whose Origin host is not the request host. A request
// with neither header still reveals, and a rejected GET does not.
func TestReviewRevealRejectsCrossSite(t *testing.T) {
	store, h, now := openApp(t)
	_, _, word := seedAnkiLexeme(t, store, now, "letter")
	a := addLearner(t, store, "a@example.com", "sub-cross-a", now)
	b := addLearner(t, store, "b@example.com", "sub-cross-b", now)
	en := addLearner(t, store, "en@example.com", "sub-cross-en", now)
	for _, learner := range []learnerFix{a, b, en} {
		seedDrill(t, store, learner.user.ID, "letter", "ג", word.exampleID)
	}
	front := getLearner(t, h, a, "/review")
	id := mustCardID(t, front)
	rejects := []struct {
		host, origin, site string
	}{
		{"learn.example", "https://learn.example", "cross-site"},
		{"learn.example", "", "same-site"},
		{"learn.example", "https://evil.example", ""},
		{"learn.example", "null", ""},
	}
	for _, tc := range rejects {
		rec := revealGet(t, h, a.cookie, id, tc.host, tc.origin, tc.site, true)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("reveal host %q origin %q site %q = %d, want 403\n%s", tc.host, tc.origin, tc.site, rec.Code, rec.Body.String())
		}
	}
	page := getLearner(t, h, a, "/review")
	if strings.Contains(page, `data-revealed="true"`) || strings.Contains(page, `name="rating"`) {
		t.Fatal("a rejected reveal marked the card revealed")
	}
	allowed := revealGet(t, h, a.cookie, id, "", "", "", true)
	if allowed.Code != http.StatusOK || !strings.Contains(allowed.Body.String(), `data-revealed="true"`) {
		t.Fatalf("reveal with no origin headers = %d\n%s", allowed.Code, allowed.Body.String())
	}

	bFront := getLearner(t, h, b, "/review")
	same := revealGet(t, h, b.cookie, mustCardID(t, bFront), "learn.example", "", "same-origin", true)
	if same.Code != http.StatusOK || !strings.Contains(same.Body.String(), `data-revealed="true"`) {
		t.Fatalf("same-origin reveal = %d\n%s", same.Code, same.Body.String())
	}
	enFront := getLearner(t, h, en, "/review")
	match := revealGet(t, h, en.cookie, mustCardID(t, enFront), "learn.example", "https://learn.example", "", true)
	if match.Code != http.StatusOK || !strings.Contains(match.Body.String(), `data-revealed="true"`) {
		t.Fatalf("matching origin reveal = %d\n%s", match.Code, match.Body.String())
	}
}

func revealGet(t *testing.T, h http.Handler, cookie *http.Cookie, cardID int64, host, origin, site string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/_hx/review/reveal?card_id="+strconv.FormatInt(cardID, 10), nil)
	if host != "" {
		req.Host = host
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if site != "" {
		req.Header.Set("Sec-Fetch-Site", site)
	}
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestReviewGradeIgnoresQueryCardID grades the card id in the POST body.
// A different card_id on the query string does not choose the card.
func TestReviewGradeIgnoresQueryCardID(t *testing.T) {
	store, h, now := openApp(t)
	_, _, word := seedAnkiLexeme(t, store, now, "read")
	learner := addLearner(t, store, "a@example.com", "sub-grade-query", now)
	addCard(t, store, learner.user.ID, word.lexemeID, word.skill, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
	front := getLearner(t, h, learner, "/review")
	id := mustCardID(t, front)
	if shown := getReveal(t, h, learner, id); shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d\n%s", shown.Code, shown.Body.String())
	}
	before := reviewLogCount(t, store)
	other := id + 999

	// card_id is only on the query string. The body has the grade and the
	// token. PostForm does not see the query value, so this is not the card.
	form := url.Values{}
	form.Set("rating", "3")
	form.Set("_csrf", learner.csrf)
	req := httptest.NewRequest(http.MethodPost, "/_hx/review/grade?card_id="+strconv.FormatInt(id, 10), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("HX-Request", "true")
	req.AddCookie(learner.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("grade used the query card_id: %d\n%s", rec.Code, rec.Body.String())
	}
	if n := reviewLogCount(t, store); n != before {
		t.Fatalf("query card_id wrote a review log, count %d want %d", n, before)
	}

	req = gradeRequest(t, learner.cookie, learner.csrf, id, "3", "https://learn.example", true)
	req.URL.RawQuery = "card_id=" + strconv.FormatInt(other, 10)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("grade ignored the body card_id: %d\n%s", rec.Code, rec.Body.String())
	}
	if n := reviewLogCount(t, store); n != before+1 {
		t.Fatalf("body card_id wrote %d new review logs, want 1", n-before)
	}
}
