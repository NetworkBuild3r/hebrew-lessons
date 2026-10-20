package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// TestReviewAnkiReveal is the Anki contract for every self-graded review card.
// The front has the prompt and Show answer, and not the answer or the grades.
// The reveal, both as an htmx fragment and as a no-JS full page, has the answer,
// the grade buttons, and the clip. A grade before that reveal is rejected and
// writes no review_log row.
func TestReviewAnkiReveal(t *testing.T) {
	for _, kind := range []string{"read", "recall", "listen", "spell", "letter", "pattern"} {
		t.Run(kind, func(t *testing.T) {
			testAnkiKind(t, kind)
		})
	}
}

// TestReviewPromptFollowsTheKey locks the one-line questions. A pair is not a
// single letter, and a two-form agreement key is not a single form.
func TestReviewPromptFollowsTheKey(t *testing.T) {
	if views.LetterPrompt("ג") != views.PromptLetter || views.LetterPrompt("ג/פ") != views.PromptLetterPair {
		t.Fatalf("letter prompts = %q %q", views.LetterPrompt("ג"), views.LetterPrompt("ג/פ"))
	}
	if views.PatternPrompt("הוא") != views.PromptPatternOne || views.PatternPrompt("הוא/היא") != views.PromptPattern {
		t.Fatalf("pattern prompts = %q %q", views.PatternPrompt("הוא"), views.PatternPrompt("הוא/היא"))
	}
	if views.LetterName("ג") != "גימל" || views.LetterName("פ") != "פא" || views.LetterName("X") != "" {
		t.Fatalf("letter names ג=%q פ=%q", views.LetterName("ג"), views.LetterName("פ"))
	}
}

type ankiSpec struct {
	prompt  string
	front   []string // must be on the front
	banned  []string // answer text that must not be on the front
	answers []string // must be on the back
	clipOn  bool     // read and listen play the prompt clip before the reveal
}

func testAnkiKind(t *testing.T, kind string) {
	t.Helper()
	store, h, now := openApp(t)
	spec, clipID, word := seedAnkiLexeme(t, store, now, kind)
	// openApp only admits a@, b@, and en@. Each subtest has its own store.
	hx := addLearner(t, store, "a@example.com", "sub-hx-"+kind, now)
	plain := addLearner(t, store, "en@example.com", "sub-plain-"+kind, now)
	for _, learner := range []learnerFix{hx, plain} {
		if kind == "letter" || kind == "pattern" {
			seedDrill(t, store, learner.user.ID, kind, word.exampleID)
			continue
		}
		addCard(t, store, learner.user.ID, word.lexemeID, word.skill, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
	}

	front := getLearner(t, h, hx, "/review")
	assertAnkiFront(t, front, spec)
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

	shown := postReveal(t, h, hx, cardID, "https://learn.example")
	if shown.Code != http.StatusOK {
		t.Fatalf("reveal = %d\n%s", shown.Code, shown.Body.String())
	}
	assertAnkiBack(t, shown.Body.String(), spec, clipID, false)

	plainFront := getLearner(t, h, plain, "/review")
	assertAnkiFront(t, plainFront, spec)
	plainID := mustCardID(t, plainFront)
	got := postRevealRaw(t, h, plain.cookie, plain.csrf, plainID, "https://learn.example", false)
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
		spec.prompt = views.PromptLetterPair
		spec.front = []string{"ג/פ", spec.prompt}
		spec.banned = []string{views.LetterName("ג"), views.LetterName("פ"), "rain-shower", "geshem-ex", "/audio/"}
		spec.answers = []string{views.LetterName("ג"), views.LetterName("פ"), "rain-shower", "geshem-ex", clipText}
		word.exampleID = addLexeme(t, store, clipText, "rain-shower", "geshem-ex", "m", "")
	case "pattern":
		clipText = "הוא"
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

func seedDrill(t *testing.T, store *db.Store, userID int64, kind string, exampleID int64) {
	t.Helper()
	rev := fsrs.New(store, store.Clock())
	ctx := context.Background()
	var missKind, key string
	var lexemeID int64
	switch kind {
	case "letter":
		missKind, key, lexemeID = fsrs.MissLetterPair, "ג/פ", exampleID
		if lexemeID == 0 {
			t.Fatal("example word was not stored")
		}
	case "pattern":
		missKind, key = fsrs.MissAgreement, "הוא/היא"
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
	if !strings.Contains(body, "Show answer") || !strings.Contains(body, "autofocus") || !strings.Contains(body, `action="/_hx/review/reveal"`) {
		t.Fatal("front has no autofocused Show answer")
	}
	if strings.Contains(body, `name="rating"`) || strings.Contains(body, "Forgot") || strings.Contains(body, "Got it") {
		t.Fatal("grade buttons rendered on the front")
	}
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
	for _, want := range spec.answers {
		if !strings.Contains(body, want) {
			t.Fatalf("back missing %q\n%s", want, body)
		}
	}
	assertReviewClip(t, body, clipID, fullPage)
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
