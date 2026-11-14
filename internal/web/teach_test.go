package web

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
)

// A word the learner has never met is shown before it is asked. This is the fix
// for "why am I being quizzed on a word I have never seen": the first screen of
// that visit carries the word, its transliteration, its meaning, and a clip, and
// asks nothing. Continue builds the question for the same visit.

// newWordCard stores one word with a gloss and a due card, and returns both ids.
func newWordCard(t *testing.T, store *db.Store, userID int64, text, gloss, translit string, now time.Time) (lexemeID, cardID int64) {
	t.Helper()
	sp := func(s string) *string { return &s }
	saved, _, err := store.UpsertLexemes(context.Background(), []lexicon.Input{
		{TextNFC: text, Gloss: gloss, Translit: sp(translit), POS: sp("noun")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 {
		t.Fatalf("upsert %s = %d rows", text, len(saved))
	}
	cardID = addCard(t, store, userID, saved[0].ID, fsrs.SkillRead, now.Add(-time.Minute), 0.5, 0, now.Add(-48*time.Hour))
	return saved[0].ID, cardID
}

// TestTeachScreenComesBeforeTheFirstQuestion drives one never-seen word through
// the review and requires the teach screen, then the question.
func TestTeachScreenComesBeforeTheFirstQuestion(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-teach", now)
	newWordCard(t, store, l.user.ID, "דַּעַת", "knowledge", "da'at", now)

	page := getLearner(t, h, l, "/review")
	if !strings.Contains(page, `data-activity="teach"`) {
		t.Fatalf("the first screen of a new word is not the teach screen:\n%s", page)
	}
	if !strings.Contains(page, "knowledge") {
		t.Fatal("the teach screen does not give the meaning")
	}
	if !strings.Contains(page, "דַּעַת") {
		t.Fatal("the teach screen does not show the word")
	}
	// No question and no grade on the teach screen.
	for _, bad := range []string{`name="token"`, `name="rating"`, "What does this word mean?"} {
		if strings.Contains(page, bad) {
			t.Fatalf("the teach screen asks something (%s):\n%s", bad, page)
		}
	}
	// Nothing was graded before Continue.
	if n := scalar(t, shellPaths[store], `SELECT COUNT(*) FROM review_log`); n != 0 {
		t.Fatalf("the teach screen wrote %d review rows", n)
	}

	// Continue builds the question for the same visit.
	rec := actPost(t, h, l, "/_hx/review/teach", teachForm(page))
	if rec.Code != http.StatusOK {
		t.Fatalf("teach = %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, `data-activity="teach"`) {
		t.Fatal("the teach screen came back after Continue")
	}
	// A checked question when the lexicon can fill one, a flashcard otherwise: a
	// lone word has no distractor glosses. Either way the teach screen is gone and
	// the learner is being asked something.
	asking := strings.Contains(body, `name="token"`) || strings.Contains(body, "What does this word mean?")
	if !asking {
		t.Fatalf("Continue did not build a question:\n%s", body)
	}
}

// TestTeachIsRecordedAsAnExposure requires the exposure on Continue, so the
// learner model knows the word has been shown.
func TestTeachIsRecordedAsAnExposure(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "b@example.com", "sub-expose", now)
	word, _ := newWordCard(t, store, l.user.ID, "שָׁלוֹם", "hello", "shalom", now)

	page := getLearner(t, h, l, "/review")
	if rec := actPost(t, h, l, "/_hx/review/teach", teachForm(page)); rec.Code != http.StatusOK {
		t.Fatalf("teach = %d", rec.Code)
	}
	if n := scalar(t, shellPaths[store], `SELECT COUNT(*) FROM exposures WHERE user_id = ? AND lexeme_id = ?`, l.user.ID, word); n != 1 {
		t.Fatalf("exposures = %d, want the teach screen recorded as one", n)
	}
}

// TestTeachPostRefusesAStaleCard keeps the same card guard as every review post.
func TestTeachPostRefusesAStaleCard(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "learner1@example.com", "sub-stale", now)
	newWordCard(t, store, l.user.ID, "בַּיִת", "house", "bayit", now)

	page := getLearner(t, h, l, "/review")
	form := teachForm(page)
	if rec := actPostAs(t, h, l.cookie, "", "/_hx/review/teach", form); rec.Code != http.StatusForbidden {
		t.Fatalf("no csrf = %d", rec.Code)
	}
	form = teachForm(page)
	form.Set("card_id", "999999")
	if rec := actPost(t, h, l, "/_hx/review/teach", form); rec.Code != http.StatusBadRequest {
		t.Fatalf("stale card = %d", rec.Code)
	}
}

// teachForm is the form the teach screen posts. The csrf is a hidden input, and
// the learner fixture also knows it, so actPost sets it either way.
func teachForm(page string) url.Values {
	form := url.Values{}
	form.Set("card_id", regexp.MustCompile(`data-card="(\d+)"`).FindStringSubmatch(page)[1])
	return form
}
