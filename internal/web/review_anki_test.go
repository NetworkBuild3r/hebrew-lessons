package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/rand/v2"
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
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
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
	if strings.Contains(frontHTML, "גימל") || strings.Contains(frontHTML, "פא") || strings.Contains(frontHTML, `name="rating"`) || strings.Contains(frontHTML, "act__answer") || strings.Contains(frontHTML, "Tell them apart") || strings.Contains(frontHTML, "Asked") {
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
	if strings.Count(backHTML, `aria-current="true"`) != 1 || strings.Count(backHTML, ">Asked<") != 1 {
		t.Fatalf("pair back should mark the asked tile once\n%s", backHTML)
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
	if kind == "pair" {
		assertLetterPickFront(t, front, spec.pair)
		cardID := mustCardID(t, front)
		if rec := getReveal(t, h, hx, cardID); rec.Code != http.StatusBadRequest {
			t.Fatalf("pair reveal = %d, want 400", rec.Code)
		}
		if rec := postGrade(t, h, hx, cardID, "3", "https://learn.example"); rec.Code != http.StatusBadRequest {
			t.Fatalf("pair grade = %d, want 400", rec.Code)
		}
		fb := postDrill(t, h, hx, cardID, spec.pair[0])
		if fb.Code != http.StatusOK {
			t.Fatalf("pair pick = %d\n%s", fb.Code, fb.Body.String())
		}
		assertAnkiBack(t, fb.Body.String(), spec, clipID, false)
		plainFront := getLearner(t, h, plain, "/review")
		assertLetterPickFront(t, plainFront, spec.pair)
		plainID := mustCardID(t, plainFront)
		plainFB := postDrill(t, h, plain, plainID, spec.pair[1])
		if plainFB.Code != http.StatusOK {
			t.Fatalf("plain pair pick = %d\n%s", plainFB.Code, plainFB.Body.String())
		}
		assertAnkiBack(t, plainFB.Body.String(), spec, clipID, false)
		return
	}
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

	shown := getReveal(t, h, hx, cardID, attr(t, front, "data-pointed"))
	if shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d\n%s", shown.Code, shown.Body.String())
	}
	assertAnkiBack(t, shown.Body.String(), spec, clipID, false)

	plainFront := getLearner(t, h, plain, "/review")
	assertAnkiFront(t, plainFront, spec)
	plainID := mustCardID(t, plainFront)
	got := getRevealRaw(t, h, plain.cookie, plainID, attr(t, plainFront, "data-pointed"), false)
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
	assertLetterPickFront(t, front, []string{dot, "פ"})
	shown := postDrill(t, h, learner, mustCardID(t, front), "פ")
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
		spec.prompt = "Which letter is missing?"
		spec.front = []string{spec.prompt, "___"}
		spec.banned = []string{views.LetterName("ג"), views.LetterName("פ"), gimelName, feName, "gimel", "g as in go", "f as in fun", "rain-shower", "geshem-ex", "ג/פ", "Tell them apart"}
		spec.answers = []string{gimelName, "gimel", "g as in go", feName, "fe", "f as in fun", "ג", "פ", "rain-shower", "geshem-ex", clipText, "Tell them apart"}
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
	if len(spec.pair) == 2 {
		assertLetterPickFront(t, body, spec.pair)
	} else {
		if !strings.Contains(body, "Show answer") || !strings.Contains(body, "autofocus") || !strings.Contains(body, `method="get"`) || !strings.Contains(body, `hx-get="/_hx/review/reveal"`) {
			t.Fatal("front has no autofocused GET Show answer")
		}
		if strings.Contains(body, "<details") {
			t.Fatal("show answer used details")
		}
		if !spec.clipOn && strings.Contains(body, "/audio/") {
			t.Fatal("front played the answer clip")
		}
		if spec.clipOn && !strings.Contains(body, "/audio/") {
			t.Fatal("prompt clip missing")
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
}

func assertAnkiBack(t *testing.T, body string, spec ankiSpec, clipID string, fullPage bool) {
	t.Helper()
	if len(spec.pair) == 2 {
		if !strings.Contains(body, `data-activity="letter-pick"`) || !strings.Contains(body, `data-phase="feedback"`) {
			t.Fatalf("pair feedback missing pick phase\n%s", body)
		}
		if !strings.Contains(body, "Continue") || !strings.Contains(body, `action="/_hx/review/next"`) {
			t.Fatalf("pair feedback missing Continue\n%s", body)
		}
		for _, want := range spec.answers {
			if !strings.Contains(body, want) {
				t.Fatalf("pair feedback missing %q\n%s", want, body)
			}
		}
		return
	}
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

func getReveal(t *testing.T, h http.Handler, learner learnerFix, cardID int64, asked ...string) *httptest.ResponseRecorder {
	t.Helper()
	letter := ""
	if len(asked) > 0 {
		letter = asked[0]
	}
	return getRevealRaw(t, h, learner.cookie, cardID, letter, true)
}

func getRevealRaw(t *testing.T, h http.Handler, cookie *http.Cookie, cardID int64, asked string, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	path := "/_hx/review/reveal?card_id=" + strconv.FormatInt(cardID, 10)
	if asked != "" {
		path += "&asked=" + url.QueryEscape(asked)
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
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
			var body string
			if kind == "pair" {
				fb := postDrill(t, h, hx, id, "ג")
				if fb.Code != http.StatusOK {
					t.Fatalf("pair pick = %d\n%s", fb.Code, fb.Body.String())
				}
				next := actPost(t, h, hx, "/_hx/review/next", url.Values{
					"_csrf": {hx.csrf}, "card_id": {strconv.FormatInt(id, 10)},
				})
				if next.Code != http.StatusOK {
					t.Fatalf("pair continue = %d\n%s", next.Code, next.Body.String())
				}
				body = next.Body.String()
			} else {
				asked := ""
				if shown := getReveal(t, h, hx, id, asked); shown.Code != http.StatusOK {
					t.Fatalf("reveal = %d\n%s", shown.Code, shown.Body.String())
				}
				next := postGrade(t, h, hx, id, "3", "https://learn.example", asked)
				if next.Code != http.StatusOK {
					t.Fatalf("next front = %d\n%s", next.Code, next.Body.String())
				}
				body = next.Body.String()
			}
			if strings.Contains(body, "<!doctype html>") {
				t.Fatal("htmx front rendered a full page")
			}
			assertNoAnswerHeading(t, body)
			if kind == "pair" {
				if !strings.Contains(body, `data-activity="letter-pick"`) || strings.Contains(body, `name="rating"`) {
					t.Fatalf("htmx response was not the next pick front\n%s", body)
				}
			} else if !strings.Contains(body, "Show answer") || strings.Contains(body, `name="rating"`) {
				t.Fatalf("htmx response was not the next front\n%s", body)
			}
			// Read and listen clear the gloss before the HTTP render, so a
			// back rendered on the front stays invisible there. This render
			// still has the gloss and goes through the same face choice.
			if kind == "read" || kind == "listen" {
				gloss := kind + "-meaning"
				tr := kind + "-tr"
				card := views.ReviewCard{
					Kind: kind, Lang: "en",
					Pointed: "שלום", Unpointed: "שלום",
					Translit: tr, AudioURL: "/audio/x.mp3",
					Gloss: gloss, Gender: "m",
				}
				html := renderMode(t, views.ReviewBody(card))
				if strings.Contains(html, gloss) || strings.Contains(html, `class="act__answer"`) {
					t.Fatalf("%s front rendered the back\n%s", kind, html)
				}
				if !strings.Contains(html, tr) || !strings.Contains(html, "שלום") {
					t.Fatalf("%s front dropped the prompt\n%s", kind, html)
				}
			}
		})
	}
}

// denverZone is America/Denver for DueCutoff cases moved off the deleted
// internal/web/studyday.go helper. INIT-026/SPEC-006.
func denverZone(t *testing.T) studyday.Zone {
	t.Helper()
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	return studyday.Zone{Name: "America/Denver", Loc: loc, Source: studyday.SourceLearner}
}

func setLearnerZone(t *testing.T, store *db.Store, userID int64, zone string) {
	t.Helper()
	if err := store.SetUserTimeZone(context.Background(), userID, zone); err != nil {
		t.Fatal(err)
	}
}

// TestStudyDayCutoffIsTheNextFourDenver locks studyday.DueCutoff for Denver:
// the next 4:00 America/Denver, and 4:00 itself belongs to the new study day.
func TestStudyDayCutoffIsTheNextFourDenver(t *testing.T) {
	z := denverZone(t)
	at := func(y int, m time.Month, d, hh, mm int) time.Time {
		return time.Date(y, m, d, hh, mm, 0, 0, z.Loc)
	}
	for _, tc := range []struct{ now, want time.Time }{
		{at(2026, 10, 21, 3, 59), at(2026, 10, 21, 4, 0)},
		{at(2026, 10, 21, 4, 0), at(2026, 10, 22, 4, 0)},
		{at(2026, 10, 20, 22, 0), at(2026, 10, 21, 4, 0)},
		{at(2026, 10, 21, 3, 30), at(2026, 10, 21, 4, 0)},
	} {
		if got := studyday.DueCutoff(tc.now, z); !got.Equal(tc.want) {
			t.Fatalf("%s cutoff = %s, want %s", tc.now, got, tc.want)
		}
	}
	early, late := at(2026, 10, 20, 17, 0), at(2026, 10, 20, 23, 0)
	if !studyday.DueCutoff(early, z).Equal(studyday.DueCutoff(late, z)) {
		t.Fatal("17:00 and 23:00 Denver are one study day")
	}
	ey, em, ed := early.UTC().Date()
	ly, lm, ld := late.UTC().Date()
	if ey == ly && em == lm && ed == ld {
		t.Fatal("17:00 and 23:00 Denver were the same UTC date")
	}
}

// TestReviewPairAskFollowsTheStudyDay asks both letters across ten study
// days, and keeps one letter from 17:00 to 23:00 Denver. Those two times
// are one study day and two UTC dates, so a rebuilt queue still asks the
// same letter. The front is a pick: asked is computed from the study-day seed.
func TestReviewPairAskFollowsTheStudyDay(t *testing.T) {
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	z := denverZone(t)
	start := time.Date(2026, 10, 18, 12, 0, 0, 0, loc)
	store, h, clk := openStepped(t, start)
	learner := addLearner(t, store, "a@example.com", "sub-pair-day", start.Add(400*24*time.Hour))
	setLearnerZone(t, store, learner.user.ID, "America/Denver")
	parts := []string{"ג", "פ"}
	seedDrill(t, store, learner.user.ID, "pair", "ג/פ", 0)

	seen := map[string]bool{}
	var cardID int64
	var flipDay time.Time
	crossed := false
	for i := 0; i < 10; i++ {
		if i > 0 {
			seedDrill(t, store, learner.user.ID, "pair", "ג/פ", 0)
		}
		front := getLearner(t, h, learner, "/review")
		id := mustCardID(t, front)
		letter := pickPart(id, studyday.DueCutoff(clk.Now(), z), parts)
		if letter != "ג" && letter != "פ" {
			t.Fatalf("day %d asked %q", i, letter)
		}
		assertLetterPickFront(t, front, parts)
		if i == 0 {
			if id >= 0 {
				t.Fatalf("drill card id = %d", id)
			}
			cardID = id
			flipDay = utcFlipEvening(cardID, start, parts)
			if flipDay.IsZero() {
				t.Fatal("no evening where a UTC date seed would change the letter")
			}
		} else if id != cardID {
			t.Fatalf("day %d card = %d, want %d", i, id, cardID)
		}
		seen[letter] = true
		if sameLocalDate(clk.Now(), flipDay) {
			noon := letter
			clk.Advance(5 * time.Hour)
			at17 := pickPart(cardID, studyday.DueCutoff(clk.Now(), z), parts)
			getLearner(t, h, learner, "/review")
			clk.Advance(6 * time.Hour)
			at23 := pickPart(cardID, studyday.DueCutoff(clk.Now(), z), parts)
			getLearner(t, h, learner, "/review")
			early, late := clk.Now().Add(-6*time.Hour), clk.Now()
			ey, em, ed := early.UTC().Date()
			ly, lm, ld := late.UTC().Date()
			if ey == ly && em == lm && ed == ld {
				t.Fatal("17:00 and 23:00 Denver were the same UTC date")
			}
			if at17 != noon || at23 != noon {
				t.Fatalf("study day %s asked %s at noon, %s at 17:00, %s at 23:00", flipDay.Format("2006-01-02"), noon, at17, at23)
			}
			crossed = true
			clk.Advance(13 * time.Hour)
			continue
		}
		clk.Advance(24 * time.Hour)
	}
	if !seen["ג"] || !seen["פ"] {
		t.Fatalf("ten study days asked %v, want both letters", seen)
	}
	if !crossed {
		t.Fatal("the ten days never rebuilt the queue inside one study day")
	}
}

// TestStudyDayAnswerAfterRolloverRefused serves a letter pick before 4:00 and
// refuses the pick after the study-day rollover (queue rebuilt).
func TestStudyDayAnswerAfterRolloverRefused(t *testing.T) {
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 21, 3, 30, 0, 0, loc)
	store, h, clk := openStepped(t, start)
	learner := addLearner(t, store, "a@example.com", "sub-pair-roll", start.Add(400*24*time.Hour))
	setLearnerZone(t, store, learner.user.ID, "America/Denver")
	seedDrill(t, store, learner.user.ID, "pair", "ג/פ", 0)
	front := getLearner(t, h, learner, "/review")
	cardID := mustCardID(t, front)
	asked := pickPart(cardID, studyday.DueCutoff(clk.Now(), denverZone(t)), []string{"ג", "פ"})
	assertLetterPickFront(t, front, []string{"ג", "פ"})

	clk.Advance(31 * time.Minute) // past 4:00
	got := postDrill(t, h, learner, cardID, asked)
	if got.Code != http.StatusBadRequest {
		t.Fatalf("pick after rollover = %d, want 400\n%s", got.Code, got.Body.String())
	}
	page := getLearner(t, h, learner, "/review")
	if !strings.Contains(page, `data-activity="letter-pick"`) {
		t.Fatalf("after rollover the queue was not rebuilt to a pick: %s", cardAttrs(page))
	}
}

// TestReviewPairRevealAndGradeRefuse returns 400 for reveal and grade on a
// two-letter letter step. The pick path is the only way to answer.
func TestReviewPairRevealAndGradeRefuse(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-pair-refuse", now)
	seedDrill(t, store, learner.user.ID, "pair", "ג/פ", 0)
	front := getLearner(t, h, learner, "/review")
	cardID := mustCardID(t, front)
	assertLetterPickFront(t, front, []string{"ג", "פ"})
	if rec := getReveal(t, h, learner, cardID, "ג"); rec.Code != http.StatusBadRequest {
		t.Fatalf("reveal = %d, want 400", rec.Code)
	}
	if rec := postGrade(t, h, learner, cardID, "3", "https://learn.example", "ג"); rec.Code != http.StatusBadRequest {
		t.Fatalf("grade = %d, want 400", rec.Code)
	}
}

func assertLetterPickFront(t *testing.T, body string, parts []string) {
	t.Helper()
	if attr(t, body, "data-activity") != "letter-pick" {
		t.Fatalf("want letter-pick front: %s", cardAttrs(body))
	}
	if strings.Contains(body, "Tell them apart") || strings.Contains(body, "Asked") || strings.Contains(body, "גימל") || strings.Contains(body, "פא") {
		t.Fatalf("pick front leaked the answer:\n%s", body)
	}
	for _, p := range parts {
		if !strings.Contains(body, `name="pick" type="submit" value="`+p+`"`) && !strings.Contains(body, `value="`+p+`" name="pick"`) {
			// ButtonWith may order attributes differently.
			if !strings.Contains(body, `value="`+p+`"`) || !strings.Contains(body, `name="pick"`) {
				t.Fatalf("pick front missing button for %q:\n%s", p, body)
			}
		}
	}
	if !strings.Contains(body, `value="dont_know"`) {
		t.Fatalf("pick front missing Don't know:\n%s", body)
	}
	if attr(t, body, "data-pointed") != "" {
		t.Fatalf("pick front showed the asked letter in data-pointed")
	}
}

func postDrill(t *testing.T, h http.Handler, l learnerFix, cardID int64, pick string) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"_csrf": {l.csrf}, "card_id": {strconv.FormatInt(cardID, 10)}, "pick": {pick}}
	req := httptest.NewRequest(http.MethodPost, "/_hx/review/drill", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.Header.Set("Origin", "https://learn.example")
	req.Host = "learn.example"
	req.AddCookie(l.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func pairAnswerLine(letter string) string {
	switch letter {
	case "ג":
		return "gimel · g as in go"
	case "פ":
		return "fe · f as in fun"
	default:
		return ""
	}
}

func assertAskedAnswer(t *testing.T, body, asked string) {
	t.Helper()
	want := pairAnswerLine(asked)
	other := pairAnswerLine("ג")
	if asked == "ג" {
		other = pairAnswerLine("פ")
	}
	if want == "" || !strings.Contains(body, want) || strings.Contains(body, other) {
		t.Fatalf("answer does not match %s\n%s", asked, body)
	}
}

// utcFlipEvening is a 17:00 America/Denver where a UTC-date seed would ask
// the other letter than 23:00 the same evening, and the study-day seed would not.
func utcFlipEvening(cardID int64, from time.Time, parts []string) time.Time {
	loc := from.Location()
	z := studyday.Zone{Name: loc.String(), Loc: loc, Source: studyday.SourceLearner}
	y, m, d := from.Date()
	for i := 0; i < 40; i++ {
		early := time.Date(y, m, d+i, 17, 0, 0, 0, loc)
		late := time.Date(y, m, d+i, 23, 0, 0, 0, loc)
		if pickPart(cardID, studyday.DueCutoff(early, z), parts) != pickPart(cardID, studyday.DueCutoff(late, z), parts) {
			continue
		}
		if utcDatePick(cardID, early, parts) != utcDatePick(cardID, late, parts) {
			return early
		}
	}
	return time.Time{}
}

// utcDatePick is the letter a card-id plus UTC yyyymmdd seed would ask.
// The study-day tests use it to choose an evening where that seed flips.
func utcDatePick(cardID int64, now time.Time, parts []string) string {
	y, m, d := now.UTC().Date()
	lane := uint64(y*10000 + int(m)*100 + d)
	src := rand.NewPCG(uint64(cardID), lane)
	return parts[rand.New(src).Uint64N(uint64(len(parts)))]
}

func sameLocalDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd && a.Location().String() == b.Location().String()
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
		// Unparseable. Treating a parse error as success would read a nil URL.
		{"learn.example", "http://[::1", ""},
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

// TestReviewLetterClipWithoutExample is the letter clip when the drill has
// no confusion example. The clip is the letter's own asset: on the back,
// and not on the front.
func TestReviewLetterClipWithoutExample(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-letter-clip", now)
	clipID := seedAnkiClip(t, store, "ג", now)
	seedDrill(t, store, learner.user.ID, "letter", "ג", 0)
	front := getLearner(t, h, learner, "/review")
	clip := "/audio/" + clipID + ".mp3"
	if strings.Contains(front, "/audio/") {
		t.Fatalf("front played the letter clip\n%s", front)
	}
	shown := getReveal(t, h, learner, mustCardID(t, front))
	if shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d\n%s", shown.Code, shown.Body.String())
	}
	if !strings.Contains(shown.Body.String(), clip) {
		t.Fatalf("back missing letter clip %s\n%s", clip, shown.Body.String())
	}
}

// TestReviewLetterBackUsesTheNewestMiss pins the example to the newest
// confusion row. An older miss of a different word stays off the back.
func TestReviewLetterBackUsesTheNewestMiss(t *testing.T) {
	store, h, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-newest-miss", now)
	older := addLexeme(t, store, "גֶּשֶׁם", "older-rain", "geshem-old", "m", "")
	newer := addLexeme(t, store, "בַּיִת", "newer-house", "bayit-new", "m", "")
	ctx := context.Background()
	if err := store.InsertConfusionMiss(ctx, learner.user.ID, fsrs.MissLetterPair, "ג", older, now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertConfusionMiss(ctx, learner.user.ID, fsrs.MissLetterPair, "ג", newer, now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	front := getLearner(t, h, learner, "/review")
	shown := getReveal(t, h, learner, mustCardID(t, front))
	if shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d\n%s", shown.Code, shown.Body.String())
	}
	body := shown.Body.String()
	if !strings.Contains(body, "newer-house") || strings.Contains(body, "older-rain") {
		t.Fatalf("back did not use the newest miss\n%s", body)
	}
}

// TestReviewDrillIgnoresQueryPick takes pick from the POST body only.
func TestReviewDrillIgnoresQueryPick(t *testing.T) {
	store, h, now := openApp(t)
	_, _, word := seedAnkiLexeme(t, store, now, "pair")
	learner := addLearner(t, store, "a@example.com", "sub-asked-query", now)
	seedDrill(t, store, learner.user.ID, "pair", "ג/פ", word.exampleID)
	front := getLearner(t, h, learner, "/review")
	cardID := mustCardID(t, front)
	assertLetterPickFront(t, front, []string{"ג", "פ"})

	form := url.Values{}
	form.Set("card_id", strconv.FormatInt(cardID, 10))
	form.Set("_csrf", learner.csrf)
	// pick only on the query string — body has none.
	req := httptest.NewRequest(http.MethodPost, "/_hx/review/drill?pick="+url.QueryEscape("ג"), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://learn.example")
	req.Header.Set("HX-Request", "true")
	req.AddCookie(learner.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("drill read pick from the query: %d\n%s", rec.Code, rec.Body.String())
	}
	page := getLearner(t, h, learner, "/review")
	if mustCardID(t, page) != cardID || attr(t, page, "data-activity") != "letter-pick" {
		t.Fatalf("query pick moved the card: %s", cardAttrs(page))
	}
}

// TestUnknownKindUsesTheCardSkill is the default face for a kind that is not
// one of the four study skills. Recall skill uses the recall face. Any other
// skill uses the read face.
func TestUnknownKindUsesTheCardSkill(t *testing.T) {
	store, _, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-unknown-kind", now)
	lex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	cardID := addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
	s := newServer(store)
	cases := []struct{ skill, want string }{
		{fsrs.SkillRecall, fsrs.SkillRecall},
		{fsrs.SkillRead, fsrs.SkillRead},
		{fsrs.SkillListen, fsrs.SkillRead},
	}
	for _, tc := range cases {
		vm, err := s.cardVM(context.Background(), &studyQueue{
			has: true,
			cur: session.CardRef{CardID: cardID, LexemeID: lex, Skill: tc.skill, Kind: "picture_match"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if vm.Kind != tc.want {
			t.Fatalf("skill %s kind = %s, want %s", tc.skill, vm.Kind, tc.want)
		}
	}
}

// TestLetterSoundUsesPlacementThenLexicon prefers a placement reading over a
// lexicon transliteration of the same text. A text that is not a placement
// letter uses the lexicon transliteration.
func TestLetterSoundUsesPlacementThenLexicon(t *testing.T) {
	store, _, _ := openApp(t)
	s := newServer(store)
	ctx := context.Background()
	addLexeme(t, store, "דָ", "dalet-a", "not-da", "m", "")
	got, err := s.letterSound(ctx, "דָ")
	if err != nil || got != "da" {
		t.Fatalf("placement sound = %q err=%v", got, err)
	}
	addLexeme(t, store, "בַּיִת", "house", "lex-only", "m", "")
	got, err = s.letterSound(ctx, "בַּיִת")
	if err != nil || got != "lex-only" {
		t.Fatalf("lexicon sound = %q err=%v", got, err)
	}
	got, err = s.letterSound(ctx, "זֻ")
	if err != nil || got != "" {
		t.Fatalf("missing sound = %q err=%v", got, err)
	}
}
