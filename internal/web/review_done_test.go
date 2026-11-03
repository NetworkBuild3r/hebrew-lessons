package web

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// progressOnly is the bare "7 / 8" the done screen used to show by itself.
var progressOnly = regexp.MustCompile(`>\d+ / \d+<`)

// TestReviewDoneNamesTheSitting checks the finished review. The count is how
// many grades this test recorded, not a number read back from the page and
// parsed. A wrong grade still counts as reviewed. Zero is the empty queue.
func TestReviewDoneNamesTheSitting(t *testing.T) {
	t.Run("two cards, full page and htmx", func(t *testing.T) {
		store, h, now := openApp(t)
		learner := addLearner(t, store, "a@example.com", "sub-a", now)
		reviewed := now.Add(-48 * time.Hour)
		firstLex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
		secondLex := addLexeme(t, store, "מַיִם", "water", "mayim", "m", "")
		first := addCard(t, store, learner.user.ID, firstLex, fsrs.SkillRead, now.Add(-2*time.Hour), 1, int(fsrs.Good), reviewed)
		second := addCard(t, store, learner.user.ID, secondLex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), reviewed)
		byHe := map[string]string{"שָׁלוֹם": "hello", "מַיִם": "water"}
		page := getLearner(t, h, learner, "/review")
		if mustCardID(t, page) != first && mustCardID(t, page) != second {
			t.Fatalf("first card %d not in the sitting: %s", first, cardAttrs(page))
		}
		// A wrong answer still counts as reviewed. It is not asked again in this sitting.
		openedGloss := glossOnPage(t, page, byHe)
		midFB := answerChecked(t, h, learner, page, wrongPickIndex(t, page, openedGloss))
		mid := continueChecked(t, h, learner, midFB.Body.String())
		if strings.Contains(mid, `data-done="true"`) {
			t.Fatalf("first answer ended the sitting: %s", cardAttrs(mid))
		}
		grades := 1
		lastGloss := glossOnPage(t, mid, byHe)
		lastFB := answerChecked(t, h, learner, mid, rightPickIndex(t, mid, lastGloss))
		done := continueChecked(t, h, learner, lastFB.Body.String())
		grades++
		want := reviewedSentence(grades)
		assertReviewDone(t, done, want, "/")
		if strings.Contains(done, "<html") {
			t.Fatal("the htmx done response is a full page")
		}

		// Nothing else is due, so the next full page is still this sitting.
		if err := store.SetCardSuspended(context.Background(), first, true); err != nil {
			t.Fatal(err)
		}
		if err := store.SetCardSuspended(context.Background(), second, true); err != nil {
			t.Fatal(err)
		}
		full := getLearner(t, h, learner, "/review")
		assertReviewDone(t, full, want, "/")
		if !strings.Contains(full, "<html") {
			t.Fatal("GET /review after the sitting is not a full page")
		}
	})

	t.Run("one card", func(t *testing.T) {
		store, h, now := openApp(t)
		learner := addLearner(t, store, "a@example.com", "sub-a", now)
		lex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
		seedFillerLexemes(t, store, 1)
		addCard(t, store, learner.user.ID, lex, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
		page := getLearner(t, h, learner, "/review")
		fb := answerChecked(t, h, learner, page, rightPickIndex(t, page, "hello"))
		done := continueChecked(t, h, learner, fb.Body.String())
		assertReviewDone(t, done, reviewedSentence(1), "/")
	})

	t.Run("zero cards", func(t *testing.T) {
		store, h, now := openApp(t)
		learner := addLearner(t, store, "a@example.com", "sub-a", now)
		body := getLearner(t, h, learner, "/review")
		assertReviewDone(t, body, reviewedSentence(0), "/")
		if !strings.Contains(body, "<html") {
			t.Fatal("the empty review is not a full page")
		}
	})

	t.Run("hebrew today link", func(t *testing.T) {
		store, h, now := openApp(t)
		learner := addLearner(t, store, "a@example.com", "sub-a", now)
		body := getLearner(t, h, learner, "/review?lang=he")
		assertReviewDone(t, body, reviewedSentence(0), "/?lang=he")
	})
}

// reviewedSentence is the summary this test expects. It is not the handler.
// TestFinishedSittingStaysOnItsStudyDay pins the keep guard. A sitting past
// its study-day rollover is not kept. Another learner's finished queue,
// even under the same key, is not this learner's sitting.
func TestFinishedSittingStaysOnItsStudyDay(t *testing.T) {
	store, _, now := openApp(t)
	learner := addLearner(t, store, "a@example.com", "sub-a", now)
	other := addLearner(t, store, "b@example.com", "sub-b", now)
	s := newServer(store)
	appLoc, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatal(err)
	}
	zones, err := studyday.NewResolver(store, appLoc)
	if err != nil {
		t.Fatal(err)
	}
	s.zones = zones
	// Rollover already passed relative to now → rebuild, drop the old count.
	expired := &studyQueue{
		userID: learner.user.ID, done: true, answered: 4, builtAt: now.Add(-48 * time.Hour),
		rollover: now.Add(-time.Hour),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.queues["day"] = expired
	kept, err := s.queueLocked(context.Background(), "day", learner.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if kept.answered != 0 || kept.userID != learner.user.ID {
		t.Fatalf("expired sitting stayed: user %d answered %d", kept.userID, kept.answered)
	}

	s.queues["other"] = &studyQueue{
		userID: other.user.ID, done: true, answered: 4, builtAt: now,
		rollover: now.Add(time.Hour),
	}
	fresh, err := s.queueLocked(context.Background(), "other", learner.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.answered != 0 || fresh.userID != learner.user.ID {
		t.Fatalf("another learner's sitting was kept: user %d answered %d", fresh.userID, fresh.answered)
	}
}

func reviewedSentence(n int) string {
	if n == 1 {
		return "You reviewed 1 card."
	}
	return "You reviewed " + itoa(int64(n)) + " cards."
}

func assertReviewDone(t *testing.T, body, summary, href string) {
	t.Helper()
	if !strings.Contains(body, "Done for now") && !strings.Contains(body, "זהו להיום") {
		t.Fatalf("done screen has no heading:\n%s", body)
	}
	if !strings.Contains(body, summary) {
		t.Fatalf("done screen summary, want %q:\n%s", summary, body)
	}
	link := doneTodayLink(t, body)
	if link != href {
		t.Fatalf("Today link href = %q, want %q", link, href)
	}
	if progressOnly.MatchString(body) {
		t.Fatalf("done screen still shows a progress fraction:\n%s", body)
	}
}

func doneTodayLink(t *testing.T, body string) string {
	t.Helper()
	marker := "Back to Today"
	i := strings.Index(body, marker)
	if i < 0 {
		marker = "חזרה להיום"
		i = strings.Index(body, marker)
	}
	if i < 0 {
		t.Fatalf("no Back to Today link:\n%s", body)
	}
	start := strings.LastIndex(body[:i], "<a ")
	if start < 0 {
		t.Fatalf("Back to Today is not a link:\n%s", body)
	}
	tag := body[start:i]
	const key = `href="`
	h := strings.Index(tag, key)
	if h < 0 {
		t.Fatalf("Today link has no href: %s", tag)
	}
	rest := tag[h+len(key):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("Today link href is open: %s", tag)
	}
	if !strings.Contains(tag, `btn-primary`) {
		t.Fatalf("Today link is not the primary button: %s", tag)
	}
	return rest[:end]
}
