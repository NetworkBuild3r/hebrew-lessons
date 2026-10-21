package web

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
)

// Real-store checks for the welcome gate. The handler calls IsOnboarded
// and does not combine onboarded_at, finished runs, and lesson attempts.

func openStoreApp(t *testing.T, realGate bool) (*db.Store, http.Handler) {
	t.Helper()
	clk := fixedClock(welcomeNow)
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	svc, err := auth.New(auth.Config{
		PublicOrigin:  "https://learn.example",
		Clock:         clk,
		Store:         store,
		AllowedEmails: shellEmails(),
	})
	if err != nil {
		t.Fatal(err)
	}
	study := session.New(store, clk, 1, session.WithGate(func(context.Context, int64) (activity.Gate, error) {
		return activity.Gate{LettersPassed: false}, nil
	}))
	opts := Options{Store: store, Runs: store, study: study}
	if realGate {
		opts.Onboard = storeOnboard{store}
	}
	return store, NewOptions(nil, svc, opts)
}

func TestStoreNewLearnerIsSentToWelcome(t *testing.T) {
	store, h := openStoreApp(t, true)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	for _, path := range welcomeLandings() {
		assertWelcome(t, welcomeGet(t, h, l, path, false), path)
	}
}

func TestStoreAbandonedPlacementStillGates(t *testing.T) {
	store, h := openStoreApp(t, true)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	ctx := context.Background()
	if _, err := store.StartPlacementV3(ctx, l.user.ID, welcomeNow, 1, "initial", "", 0); err != nil {
		t.Fatal(err)
	}
	user, err := store.UserByEmail(ctx, l.user.Email)
	if err != nil || user.OnboardedAt != nil {
		t.Fatalf("abandoned run set onboarded_at %v %v", user.OnboardedAt, err)
	}
	assertWelcome(t, welcomeGet(t, h, l, "/", false), "/")
}

func TestStorePlacementAttemptStillGates(t *testing.T) {
	store, h := openStoreApp(t, true)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	lex := addLexeme(t, store, "שָׁלוֹם", "hello", "shalom", "m", "")
	insertAttemptRow(t, store, l.user.ID, lex, "placement")
	assertWelcome(t, welcomeGet(t, h, l, "/", false), "/")
}

func TestStoreLessonRunSkipsWelcome(t *testing.T) {
	store, h := openStoreApp(t, true)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	unit := shellUnit(t, store, "Basics", 1, "shared", 1)
	if _, err := store.ExecContext(context.Background(),
		`INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at) VALUES (?, ?, 0, ?)`,
		l.user.ID, unit, db.FormatTime(welcomeNow)); err != nil {
		t.Fatal(err)
	}
	user, err := store.UserByEmail(context.Background(), l.user.Email)
	if err != nil || user.OnboardedAt != nil {
		t.Fatalf("lesson run set onboarded_at before the gate %v %v", user.OnboardedAt, err)
	}
	home := welcomeGet(t, h, l, "/", false)
	assertNotWelcome(t, home, "/")
	if home.Code != http.StatusOK {
		t.Fatalf("Today = %d", home.Code)
	}
}

func TestStoreLessonAttemptSkipsWelcome(t *testing.T) {
	store, h := openStoreApp(t, true)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	lex := addLexeme(t, store, "בַּיִת", "house", "bayit", "m", "")
	insertAttemptRow(t, store, l.user.ID, lex, "lesson")
	home := welcomeGet(t, h, l, "/", false)
	assertNotWelcome(t, home, "/")
	if home.Code != http.StatusOK {
		t.Fatalf("Today = %d", home.Code)
	}
}

func TestStoreOtherLearnersLessonDoesNotOpenTheGate(t *testing.T) {
	store, h := openStoreApp(t, true)
	a := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	b := addLearner(t, store, "b@example.com", "sub-b", welcomeNow)
	unit := shellUnit(t, store, "Basics", 1, "shared", 1)
	if _, err := store.ExecContext(context.Background(),
		`INSERT INTO lesson_runs (user_id, unit_id, lesson_ix, started_at) VALUES (?, ?, 0, ?)`,
		a.user.ID, unit, db.FormatTime(welcomeNow)); err != nil {
		t.Fatal(err)
	}
	assertNotWelcome(t, welcomeGet(t, h, a, "/", false), "/")
	assertWelcome(t, welcomeGet(t, h, b, "/", false), "/")
}

func TestStoreHistoryShowsTheWordsKnownColumn(t *testing.T) {
	store, h := openStoreApp(t, true)
	a := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	b := addLearner(t, store, "b@example.com", "sub-b", welcomeNow)
	shellUnit(t, store, "At home", 4, "shared", 1)
	ctx := context.Background()
	id, err := store.StartPlacementV3(ctx, a.user.ID, welcomeNow.AddDate(0, 0, -1), 1, "initial", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishPlacement(ctx, id, welcomeNow.AddDate(0, 0, -1), 3, "exhausted", `{"estimated_known_words":999}`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecContext(ctx, `UPDATE placement_runs SET start_unit = 4, words_known = 81 WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}

	card := section(t, welcomeGet(t, h, a, "/progress", false).Body.String(), "c-card pplace", "")
	for _, want := range []string{"81 words", "Unit 4: At home", ">Unit 4</bdi>", "Placed Oct 17"} {
		if !strings.Contains(card, want) {
			t.Errorf("history missing %q:\n%s", want, card)
		}
	}
	if strings.Contains(card, "999") {
		t.Errorf("the card used the JSON estimate:\n%s", card)
	}

	other := welcomeGet(t, h, b, "/progress", false)
	assertWelcome(t, other, "/progress")
	if strings.Contains(other.Body.String(), "81") || strings.Contains(other.Body.String(), "At home") {
		t.Fatalf("the redirect leaks this history:\n%s", other.Body.String())
	}
	basics := formPost(t, h, b, "/onboarding/basics", url.Values{"_csrf": {b.csrf}}, nil)
	if basics.Code != http.StatusSeeOther {
		t.Fatalf("other basics = %d", basics.Code)
	}
	page := welcomeGet(t, h, b, "/progress", false).Body.String()
	if strings.Contains(page, "81 words") || strings.Contains(page, "At home") {
		t.Fatalf("the other learner sees this history:\n%s", page)
	}
	if !strings.Contains(page, "No results yet.") {
		t.Fatalf("the other learner has no empty history:\n%s", page)
	}
}

func TestStoreSetOnboardedKeepsTheFirstTime(t *testing.T) {
	store, h := openStoreApp(t, true)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	rec := formPost(t, h, l, "/onboarding/basics", url.Values{"_csrf": {l.csrf}}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("basics = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	for _, path := range welcomeLandings() {
		got := welcomeGet(t, h, l, path, false)
		assertNotWelcome(t, got, path)
		if path == "/" && got.Code != http.StatusOK {
			t.Fatalf("Today after basics = %d", got.Code)
		}
	}
	again := welcomeGet(t, h, l, "/welcome", false)
	if again.Code != http.StatusSeeOther || again.Header().Get("Location") != "/" {
		t.Errorf("GET /welcome after basics = %d %q", again.Code, again.Header().Get("Location"))
	}
	ctx := context.Background()
	user, err := store.UserByEmail(ctx, l.user.Email)
	if err != nil || user.OnboardedAt == nil || !user.OnboardedAt.Equal(welcomeNow) {
		t.Fatalf("onboarded_at = %v %v", user.OnboardedAt, err)
	}
	later := welcomeNow.AddDate(1, 0, 0)
	if err := store.SetOnboarded(ctx, l.user.ID, later); err != nil {
		t.Fatal(err)
	}
	user, err = store.UserByEmail(ctx, l.user.Email)
	if err != nil || user.OnboardedAt == nil || !user.OnboardedAt.Equal(welcomeNow) {
		t.Fatalf("second write moved onboarded_at to %v %v", user.OnboardedAt, err)
	}
}

func TestStoreFinishedPlacementDoesNotRedirect(t *testing.T) {
	store, h := openStoreApp(t, true)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	ctx := context.Background()
	finished := welcomeNow.Add(time.Hour)
	id, err := store.StartPlacementV3(ctx, l.user.ID, welcomeNow, 1, "initial", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishPlacement(ctx, id, finished, 2, "cap_cards", `{"estimated_known_words":9}`); err != nil {
		t.Fatal(err)
	}
	home := welcomeGet(t, h, l, "/", false)
	assertNotWelcome(t, home, "/")
	if home.Code != http.StatusOK {
		t.Fatalf("Today after a finish = %d", home.Code)
	}
	// The finished run is enough on its own. Clearing the column must not
	// send the learner back to /welcome.
	if _, err := store.ExecContext(ctx, `UPDATE users SET onboarded_at = NULL WHERE id = ?`, l.user.ID); err != nil {
		t.Fatal(err)
	}
	user, err := store.UserByEmail(ctx, l.user.Email)
	if err != nil || user.OnboardedAt != nil {
		t.Fatalf("column clear left onboarded_at %v %v", user.OnboardedAt, err)
	}
	again := welcomeGet(t, h, l, "/", false)
	assertNotWelcome(t, again, "/")
	if again.Code != http.StatusOK {
		t.Fatalf("Today after clearing onboarded_at = %d %s", again.Code, again.Header().Get("Location"))
	}
}

func TestStoreFinishPlacementSetsOnboardedAt(t *testing.T) {
	store, _ := openStoreApp(t, true)
	l := addLearner(t, store, "a@example.com", "sub-a", welcomeNow)
	ctx := context.Background()
	finished := welcomeNow.Add(time.Hour)
	id, err := store.StartPlacementV3(ctx, l.user.ID, welcomeNow, 1, "initial", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishPlacement(ctx, id, finished, 2, "cap_cards", `{"estimated_known_words":9}`); err != nil {
		t.Fatal(err)
	}
	user, err := store.UserByEmail(ctx, l.user.Email)
	if err != nil || user.OnboardedAt == nil || !user.OnboardedAt.Equal(finished) {
		t.Fatalf("onboarded_at = %v %v, want %v", user.OnboardedAt, err, finished)
	}
}

func insertAttemptRow(t *testing.T, store *db.Store, userID, lexemeID int64, source string) {
	t.Helper()
	if _, err := store.ExecContext(context.Background(), `INSERT INTO attempts (
		user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at
	) VALUES (?, ?, 'read', 'pick', ?, 'p', 'r', 1, 0, 0, 10, ?)`,
		userID, lexemeID, source, db.FormatTime(welcomeNow)); err != nil {
		t.Fatal(err)
	}
}
