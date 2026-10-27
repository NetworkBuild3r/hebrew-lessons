package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// INIT-015/SPEC-017: the app shell. Today's one next action (ac-1), the band-slip alert
// (ac-1b), the dock and the rail (ac-2: the render half here, the real-browser half in
// shell_browser_test.go), the report path (ac-3), and Words to watch (ac-4).

// ---- fixtures ---------------------------------------------------------------

// fakeEngine is the placement engine as Today sees it.
type fakeEngine struct {
	status    placement.RecheckStatus
	statusErr error
	latest    placement.RunResult
	hasLatest bool
	newErr    error
	asked     []string
}

func (f *fakeEngine) RecheckDue(context.Context, int64) (placement.RecheckStatus, error) {
	return f.status, f.statusErr
}

func (f *fakeEngine) Latest(context.Context, int64) (placement.RunResult, bool, error) {
	return f.latest, f.hasLatest, nil
}

func (f *fakeEngine) NewRecheck(_ context.Context, _ int64, skill string) (int64, error) {
	f.asked = append(f.asked, skill)
	return 1, f.newErr
}

// dueEngine is an engine whose check-in is due, with listening ahead of reading.
func dueEngine() *fakeEngine {
	return &fakeEngine{
		status: placement.RecheckStatus{HasPlacement: true, Due: true, LastAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)},
		latest: placement.RunResult{
			Listening: placement.SkillResult{Skill: placement.SkillListen, Tested: true, Level: 3},
			Reading:   placement.SkillResult{Skill: placement.SkillRead, Tested: true, Level: 2},
		},
		hasLatest: true,
	}
}

// staticBands says which placement band a word is in, so a test can build a band.
type staticBands map[int64]int

func (b staticBands) LexemeBands(context.Context) (map[int64]int, error) {
	out := map[int64]int{}
	for k, v := range b {
		out[k] = v
	}
	return out, nil
}

// shellEmails is who may sign in: a and b, and learner1 to learner64 for the browser tests.
func shellEmails() []string {
	out := []string{"a@example.com", "b@example.com"}
	for i := 1; i <= 64; i++ {
		out = append(out, fmt.Sprintf("learner%d@example.com", i))
	}
	return out
}

// shellPaths is where each test store lives, so a test can read it with a second connection.
var shellPaths = map[*db.Store]string{}

// openShell is the app with a closed letters gate, so reviews are flashcards (as openAt),
// plus the placement engine and the band seam the shell reads.
func openShell(t *testing.T, clk clock.Clock, eng PlacementEngine, bands learner.BandSource) (*db.Store, http.Handler) {
	t.Helper()
	return openShellAt(t, clk, "https://learn.example", eng, bands)
}

// openShellAt is openShell for an origin: a browser test posts from its own server address.
func openShellAt(t *testing.T, clk clock.Clock, origin string, eng PlacementEngine, bands learner.BandSource) (*db.Store, http.Handler) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hebrew.sqlite")
	store, err := db.Open(context.Background(), path, clk)
	if err != nil {
		t.Fatal(err)
	}
	shellPaths[store] = path
	t.Cleanup(func() { _ = store.Close() })
	svc, err := auth.New(auth.Config{
		PublicOrigin:  origin,
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
	return store, NewOptions(nil, svc, Options{Store: store, Runs: store, study: study, Placement: eng, bands: bands})
}

// shellUnit publishes a shared or learner unit of ungraded story steps.
func shellUnit(t *testing.T, store *db.Store, title string, position int, scope string, steps int) int64 {
	t.Helper()
	acts := make([]units.Activity, steps)
	for i := range acts {
		acts[i] = units.Activity{Type: units.ActivityStory, Text: "OK"}
	}
	id := putLesson(t, store, nil, units.Unit{
		Title: title, Position: position,
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: acts}},
	}, scope)
	publish(t, store, id)
	return id
}

// finishPlacement gives the learner a finished placement run.
func finishPlacement(t *testing.T, store *db.Store, userID int64, at time.Time) {
	t.Helper()
	ctx := context.Background()
	run, err := store.InsertPlacementRun(ctx, userID, at, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishPlacement(ctx, run, at, 2, "cap_cards", `{"estimated_known_words":40,"bands":[]}`); err != nil {
		t.Fatal(err)
	}
}

// strugglingWord gives a word right answers first and then wrong ones, so the wrong ones are
// the newest. The learner model reads the newest outcomes.
func strugglingWord(t *testing.T, store *db.Store, clk clock.Clock, userID, lexemeID int64, skill string, wrong, right int) {
	t.Helper()
	rec := evidence.New(store, fsrs.New(store, clk), clk)
	for i := 0; i < wrong+right; i++ {
		_, err := rec.Attempt(context.Background(), evidence.Attempt{
			UserID: userID, LexemeID: lexemeID, Skill: skill, Activity: "listen_pick",
			Source: evidence.SourceLesson, Prompt: "p", Response: "r", Correct: i < right, SkipSchedule: true,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

var (
	goldButton = regexp.MustCompile(`class="btn btn-primary"`)
	todayAct   = regexp.MustCompile(`<article class="today" data-action="([a-z_]+)"`)
)

func goldCount(body string) int { return len(goldButton.FindAllString(body, -1)) }

func todayAction(t *testing.T, body string) string {
	t.Helper()
	m := todayAct.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("Today has no data-action:\n%s", articleAttrs(body))
	}
	return m[1]
}

func formPost(t *testing.T, h http.Handler, l learnerFix, path string, form url.Values, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Host = "learn.example"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://learn.example")
	if l.cookie != nil {
		req.AddCookie(l.cookie)
	}
	if mutate != nil {
		mutate(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func codeFor(t *testing.T, h http.Handler, l learnerFix, path string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(l.cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

// ---- ac-1: one gold action --------------------------------------------------

func TestChooseTodayAction(t *testing.T) {
	for _, c := range []struct {
		name string
		in   TodayInputs
		want views.TodayAction
	}{
		{"open placement resumes", TodayInputs{PlacementActive: true, LessonInProgress: true, CheckinDue: true, Due: 9, HasLesson: true}, views.ActionResumePlacement},
		{"no placement starts one", TodayInputs{PlacementNever: true, LessonInProgress: true, CheckinDue: true, Due: 9, HasLesson: true}, views.ActionStartPlacement},
		{"a lesson in progress continues", TodayInputs{LessonInProgress: true, Due: 9, HasLesson: true}, views.ActionContinueLesson},
		{"the check-in never outranks a lesson in progress", TodayInputs{LessonInProgress: true, CheckinDue: true, Due: 9, HasLesson: true}, views.ActionContinueLesson},
		{"the check-in outranks reviews", TodayInputs{CheckinDue: true, Due: 9, HasLesson: true}, views.ActionCheckin},
		{"the next lesson outranks reviews", TodayInputs{Due: 1, HasLesson: true}, views.ActionNextLesson},
		{"the next lesson", TodayInputs{HasLesson: true}, views.ActionNextLesson},
		{"nothing to do", TodayInputs{}, views.ActionNone},
	} {
		if got := ChooseTodayAction(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// Every action but none is drawn with exactly one gold button, and it is the right one.
func TestTodayStatesRenderOneGoldAction(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	ctx := context.Background()

	type state struct {
		name   string
		action string
		eng    PlacementEngine
		setup  func(store *db.Store, l learnerFix)
		target string // the href, or the form action, of the gold button
	}
	var lessonLink string
	states := []state{
		{
			name: "resume placement", action: "resume_placement", target: `href="/placement"`,
			setup: func(store *db.Store, l learnerFix) {
				if _, err := store.InsertPlacementRun(ctx, l.user.ID, now, 1); err != nil {
					t.Fatal(err)
				}
			},
		},
		{name: "start placement", action: "start_placement", target: `href="/placement"`, setup: func(*db.Store, learnerFix) {}},
		{
			name: "continue a lesson", action: "continue_lesson",
			setup: func(store *db.Store, l learnerFix) {
				finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
				id := shellUnit(t, store, "Mine", 1, "shared", 3)
				lessonLink = "/lesson/" + itoa(id) + "/0"
				run, err := store.StartPlayerRun(ctx, l.user.ID, id, 0)
				if err != nil {
					t.Fatal(err)
				}
				if _, _, err := store.ContinuePlayerRun(ctx, l.user.ID, run.ID, 0, 3); err != nil {
					t.Fatal(err)
				}
				addCard(t, store, l.user.ID, addLexeme(t, store, helloHe, "hello", "shalom", "m", ""), fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
			},
		},
		{
			name: "take the check-in", action: "checkin", eng: dueEngine(), target: `action="/_hx/placement/recheck"`,
			setup: func(store *db.Store, l learnerFix) {
				finishPlacement(t, store, l.user.ID, now.Add(-40*24*time.Hour))
				addCard(t, store, l.user.ID, addLexeme(t, store, helloHe, "hello", "shalom", "m", ""), fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
				shellUnit(t, store, "Next", 1, "shared", 2)
			},
		},
		{
			name: "do the due reviews", action: "reviews", target: `href="/review"`,
			setup: func(store *db.Store, l learnerFix) {
				finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
				addCard(t, store, l.user.ID, addLexeme(t, store, helloHe, "hello", "shalom", "m", ""), fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
			},
		},
		{
			name: "open the next lesson", action: "next_lesson",
			setup: func(store *db.Store, l learnerFix) {
				finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
				addCard(t, store, l.user.ID, addLexeme(t, store, helloHe, "hello", "shalom", "m", ""), fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
				id := shellUnit(t, store, "Next", 1, "shared", 2)
				lessonLink = "/lesson/" + itoa(id) + "/0"
			},
		},
	}
	for i, s := range states {
		t.Run(s.name, func(t *testing.T) {
			lessonLink = ""
			store, h := openShell(t, clk, s.eng, nil)
			l := addLearner(t, store, "a@example.com", fmt.Sprintf("sub-%d", i), now)
			s.setup(store, l)
			body := getLearner(t, h, l, "/")
			if got := todayAction(t, body); got != s.action {
				t.Fatalf("action = %q, want %q", got, s.action)
			}
			if n := goldCount(body); n != 1 {
				t.Fatalf("%d gold actions, want exactly 1", n)
			}
			target := s.target
			if target == "" {
				target = `href="` + lessonLink + `"`
			}
			// The gold button is the one that goes where the action says.
			gold := regexp.MustCompile(`(?s)<(?:a|button)\b[^>]*class="btn btn-primary"[^>]*>|<form\b[^>]*>(?:\s*<input[^>]*>)*\s*<button type="submit" class="btn btn-primary">`).FindString(body)
			if !strings.Contains(gold, target) {
				t.Fatalf("the gold button does not go to %s:\n%s", target, gold)
			}
		})
	}

	t.Run("nothing to do has no gold", func(t *testing.T) {
		store, h := openShell(t, clk, nil, nil)
		l := addLearner(t, store, "a@example.com", "sub-idle", now)
		finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
		body := getLearner(t, h, l, "/")
		if todayAction(t, body) != "none" || goldCount(body) != 0 {
			t.Fatalf("idle Today: action %q, %d gold", todayAction(t, body), goldCount(body))
		}
		// An empty state names what to do next.
		if !strings.Contains(body, "Until then, look over what you know.") || !strings.Contains(body, `href="/progress"`) {
			t.Fatal("the idle Today does not say what to do next")
		}
	})
}

// A lesson in progress outranks a due check-in and a due review, end to end.
func TestTodayCheckinNeverOutranksALessonInProgress(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, h := openShell(t, fixedClock(now), dueEngine(), nil)
	l := addLearner(t, store, "a@example.com", "sub-x", now)
	finishPlacement(t, store, l.user.ID, now.Add(-40*24*time.Hour))
	addCard(t, store, l.user.ID, addLexeme(t, store, helloHe, "hello", "shalom", "m", ""), fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
	id := shellUnit(t, store, "Mine", 1, "shared", 3)

	before := getLearner(t, h, l, "/")
	if todayAction(t, before) != "checkin" {
		t.Fatalf("before the lesson: %q, want checkin", todayAction(t, before))
	}
	run, err := store.StartPlayerRun(ctx, l.user.ID, id, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ContinuePlayerRun(ctx, l.user.ID, run.ID, 0, 3); err != nil {
		t.Fatal(err)
	}
	after := getLearner(t, h, l, "/")
	if todayAction(t, after) != "continue_lesson" || !strings.Contains(after, "Step 2 of 3") {
		t.Fatalf("with a lesson in progress: %q\n%s", todayAction(t, after), articleAttrs(after))
	}
	// Opening the lesson without answering is not progress: the check-in is still first.
	other := addLearner(t, store, "b@example.com", "sub-y", now)
	finishPlacement(t, store, other.user.ID, now.Add(-40*24*time.Hour))
	if _, err := store.StartPlayerRun(ctx, other.user.ID, id, 0); err != nil {
		t.Fatal(err)
	}
	if got := todayAction(t, getLearner(t, h, other, "/")); got != "checkin" {
		t.Fatalf("an opened but untouched lesson counts as progress: %q", got)
	}
}

func TestDailyLine(t *testing.T) {
	for _, c := range []struct {
		due  int
		want string
	}{
		{0, ""},
		{1, "1 review due · about 1 minute"},
		{12, "12 reviews due · about 6 minutes"},
		{13, "13 reviews due · about 7 minutes"},
		{50, "50 reviews due · about 10 minutes"}, // a visit asks twenty at most
	} {
		if got := views.DailyLineFor(c.due, session.SessionCap); got != c.want {
			t.Errorf("DailyLineFor(%d) = %q, want %q", c.due, got, c.want)
		}
	}
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	for i := 0; i < 12; i++ {
		w := actWords[i%len(actWords)]
		id := addLexeme(t, store, fmt.Sprintf("%s%d", w.he, i), w.gloss, w.translit, "m", "")
		addCard(t, store, l.user.ID, id, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
	}
	body := getLearner(t, h, l, "/")
	if !strings.Contains(body, "12 reviews due · about 6 minutes") {
		t.Fatal("the daily line is not on Today")
	}
	if regexp.MustCompile(`(?i)streak|xp\b|\bpoints\b`).MatchString(body) {
		t.Fatal("a streak or score is on Today")
	}
}

// ---- ac-1b: a band slip shows for seven days -------------------------------

func slipEvent(t *testing.T, store *db.Store, userID int64, day string, band int) {
	t.Helper()
	err := store.InDailyTx(context.Background(), func(tx *db.DailyTx) error {
		return tx.InsertRegression(context.Background(), db.RegressionEvent{
			UserID: userID, Kind: db.EventBandSlip, Day: day,
			Cause: fmt.Sprintf("band_slip band=%d known share 0.80 to 0.60 since 2026-10-01", band),
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBandSlipShowsForSevenDays(t *testing.T) {
	start := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := &stepClock{t: start}
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", start)
	finishPlacement(t, store, l.user.ID, start.Add(-time.Hour))
	if strings.Contains(getLearner(t, h, l, "/"), "today-slip") {
		t.Fatal("an alert with no event")
	}
	slipEvent(t, store, l.user.ID, "2026-10-18", 2)

	// The sign-in session lives an hour. Keep it alive as the days pass.
	renew := func() {
		if _, err := store.ExecContext(context.Background(), `UPDATE sessions SET expires_at = ?`, db.FormatTime(clk.Now().Add(time.Hour))); err != nil {
			t.Fatal(err)
		}
	}
	for day := 0; day <= 6; day++ {
		renew()
		body := getLearner(t, h, l, "/")
		if !strings.Contains(body, `class="today-slip"`) || !strings.Contains(body, "Some level 3 words are slipping") {
			t.Fatalf("day %d after the event: no alert", day)
		}
		clk.Advance(24 * time.Hour)
	}
	renew()
	if body := getLearner(t, h, l, "/"); strings.Contains(body, "today-slip") {
		t.Fatal("day 7 after the event: the alert is still there")
	}
}

// ---- ac-4: Words to watch, and the review set of a slipped band ------------

func TestWordsToWatchNamesTheFailingSkill(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	words := seedLexicon(t, store, now, 8, false, true)

	// Six struggle, in different skills and to different depths. Two do not. The order is by
	// accuracy in the failing skill, and it is not the order of the wrong answers: the word with
	// the most wrong answers (words[0]) is in the middle, so a sort by wrong answers would show.
	strugglingWord(t, store, clk, l.user.ID, words[0].id, fsrs.SkillListen, 8, 12) // 8 wrong, 60% right
	strugglingWord(t, store, clk, l.user.ID, words[1].id, fsrs.SkillRead, 5, 0)    // nothing right: worst
	strugglingWord(t, store, clk, l.user.ID, words[2].id, fsrs.SkillRecall, 4, 2)
	strugglingWord(t, store, clk, l.user.ID, words[3].id, fsrs.SkillListen, 5, 5)
	strugglingWord(t, store, clk, l.user.ID, words[4].id, fsrs.SkillSpell, 3, 7)
	strugglingWord(t, store, clk, l.user.ID, words[5].id, fsrs.SkillRead, 3, 9) // the sixth: left off
	strugglingWord(t, store, clk, l.user.ID, words[6].id, fsrs.SkillRead, 1, 9) // not struggling
	body := getLearner(t, h, l, "/")

	card := regexp.MustCompile(`(?s)<section class="c-card today-watch".*?</section>`).FindString(body)
	if card == "" {
		t.Fatalf("no Words to watch card:\n%s", body)
	}
	rows := regexp.MustCompile(`(?s)<li class="watch__item" data-skill="([a-z]+)">(.*?)</li>`).FindAllStringSubmatch(card, -1)
	if len(rows) != 5 {
		t.Fatalf("%d rows, want 5", len(rows))
	}
	want := []struct{ he, skill, label string }{
		{words[1].he, "read", "Reading"},
		{words[2].he, "recall", "Recall"},
		{words[3].he, "listen", "Listening"},
		{words[0].he, "listen", "Listening"},
		{words[4].he, "spell", "Spelling"},
	}
	for i, w := range want {
		if rows[i][1] != w.skill || !strings.Contains(rows[i][2], w.he) || !strings.Contains(rows[i][2], ">"+w.label+"<") {
			t.Errorf("row %d: want %s %s %s, got %s", i, w.he, w.skill, w.label, rows[i][0])
		}
	}
	if strings.Contains(card, words[5].he) || strings.Contains(card, words[6].he) || strings.Contains(card, words[7].he) {
		t.Error("the card lists a word that is not in the top five struggling")
	}
	if !strings.Contains(card, `lang="he" dir="rtl"`) {
		t.Error("Hebrew on the card is not marked lang=he dir=rtl")
	}
}

func TestBandSlipLinksToAReviewSetOfThatBand(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	words := map[int64]int{}
	store, h := openShell(t, clk, nil, bandsOf(words))
	l := addLearner(t, store, "a@example.com", "sub-a", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	w := seedLexicon(t, store, now, 6, false, true)
	for i, band := range []int{0, 0, 1, 0} { // w[3] is band 0 but fine
		words[w[i].id] = band
	}
	words[w[3].id] = 0
	for i := range w[:4] {
		addCard(t, store, l.user.ID, w[i].id, fsrs.SkillRead, now.Add(-time.Hour), 1, int(fsrs.Good), now.Add(-48*time.Hour))
	}
	strugglingWord(t, store, clk, l.user.ID, w[0].id, fsrs.SkillRead, 5, 0)
	strugglingWord(t, store, clk, l.user.ID, w[1].id, fsrs.SkillListen, 5, 0)
	strugglingWord(t, store, clk, l.user.ID, w[2].id, fsrs.SkillRead, 5, 0) // band 1: not in the set
	strugglingWord(t, store, clk, l.user.ID, w[3].id, fsrs.SkillRead, 0, 8) // band 0, fine: not in the set
	slipEvent(t, store, l.user.ID, "2026-10-18", 0)

	body := getLearner(t, h, l, "/")
	slip := regexp.MustCompile(`(?s)<section class="today-slip".*?</section>`).FindString(body)
	if !strings.Contains(slip, "Some level 1 words are slipping") || !strings.Contains(slip, "2 words you knew need another look.") {
		t.Fatalf("alert text:\n%s", slip)
	}
	if !strings.Contains(slip, `action="/_hx/review/band"`) || !strings.Contains(slip, `name="band" value="0"`) || !strings.Contains(slip, `name="_csrf"`) {
		t.Fatalf("the alert does not post to the band review set:\n%s", slip)
	}

	// Without a CSRF token the post is refused.
	if rec := formPost(t, h, l, "/_hx/review/band", url.Values{"band": {"0"}}, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("no csrf = %d", rec.Code)
	}
	if rec := formPost(t, h, l, "/_hx/review/band", url.Values{"band": {"9"}, "_csrf": {l.csrf}}, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("band 9 = %d", rec.Code)
	}
	rec := formPost(t, h, l, "/_hx/review/band", url.Values{"band": {"0"}, "_csrf": {l.csrf}}, nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/review" {
		t.Fatalf("band review = %d %q", rec.Code, rec.Header().Get("Location"))
	}

	// The queue now holds exactly the two struggling band-0 words.
	got := map[int64]bool{}
	page := getLearner(t, h, l, "/review")
	for i := 0; i < 10 && !strings.Contains(cardAttrs(page), `data-done="true"`); i++ {
		card := mustInt(t, attr(t, cardAttrs(page), "data-card"))
		got[card] = true
		rec := gradeRevealed(t, h, l, card, "3", "https://learn.example")
		if rec.Code != http.StatusOK {
			t.Fatalf("grade = %d %s", rec.Code, rec.Body.String())
		}
		page = rec.Body.String()
	}
	cards := map[int64]bool{}
	for _, c := range []lexWord{w[0], w[1]} {
		cards[int64(scalar(t, shellPaths[store], `SELECT id FROM cards WHERE user_id = ? AND lexeme_id = ?`, l.user.ID, c.id))] = true
	}
	if len(got) != len(cards) {
		t.Fatalf("review set = %v, want %v", got, cards)
	}
	for id := range cards {
		if !got[id] {
			t.Fatalf("review set = %v, want %v", got, cards)
		}
	}
}

// bandsOf reads a map the test fills in after the app is built.
func bandsOf(m map[int64]int) learner.BandSource { return staticBands(m) }

// ---- ac-3: a report hides the unit -----------------------------------------

func TestReportHidesTheUnitInTheSameResponse(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	a := addLearner(t, store, "a@example.com", "sub-a", now)
	b := addLearner(t, store, "b@example.com", "sub-b", now)
	for _, l := range []learnerFix{a, b} {
		finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	}
	reported := shellUnit(t, store, "Reported unit", 1, "shared", 2)
	kept := shellUnit(t, store, "Kept unit", 2, "shared", 2)
	mine := shellUnit(t, store, "Mine only", 3, "learner:"+itoa(a.user.ID), 2)
	theirs := shellUnit(t, store, "Theirs only", 4, "learner:"+itoa(b.user.ID), 2)

	if today := getLearner(t, h, a, "/"); todayAction(t, today) != "next_lesson" || !strings.Contains(today, "/lesson/"+itoa(reported)+"/0") {
		t.Fatal("before the report the first unit is next")
	}
	if !strings.Contains(getLearner(t, h, a, "/lessons"), "Reported unit") {
		t.Fatal("before the report the unit is on Lessons")
	}
	// Two shared units plus this learner's unit. The other learner's unit is not in the count.
	if !strings.Contains(getLearner(t, h, a, "/progress"), ">0/3</bdi>") {
		t.Fatal("before the report Progress counts both shared units and this learner's unit")
	}

	path := "/lessons/" + itoa(reported) + "/report"
	form := func(extra url.Values) url.Values {
		v := url.Values{"reason": {"wrong_hebrew"}, "note": {"the second line"}, "_csrf": {a.csrf}}
		for k, vs := range extra {
			v[k] = vs
		}
		return v
	}

	// Refused: no token, a bad reason, a long note, someone else's unit, a signed-out post.
	if rec := formPost(t, h, a, path, url.Values{"reason": {"wrong_hebrew"}}, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("no csrf = %d", rec.Code)
	}
	if rec := formPost(t, h, a, path, form(url.Values{"reason": {"because"}}), nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown reason = %d", rec.Code)
	}
	long := strings.Repeat("x", 501)
	rec := formPost(t, h, a, path, form(url.Values{"note": {long}}), nil)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "too long") {
		t.Fatalf("501-character note = %d", rec.Code)
	}
	if rec := formPost(t, h, a, "/lessons/"+itoa(theirs)+"/report", form(nil), nil); rec.Code != http.StatusNotFound {
		t.Fatalf("another learner's unit = %d", rec.Code)
	}
	if rec := formPost(t, h, a, "/lessons/999999/report", form(nil), nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing unit = %d", rec.Code)
	}
	if rec := formPost(t, h, learnerFix{}, path, form(nil), nil); rec.Code != http.StatusForbidden {
		t.Fatalf("signed-out post = %d, want the CSRF refusal", rec.Code)
	}
	if n := countReports(t, store); n != 0 {
		t.Fatalf("refused posts wrote %d reports", n)
	}

	// A 500-character note is fine.
	ok := formPost(t, h, a, path, form(url.Values{"note": {strings.Repeat("y", 500)}}), nil)
	if ok.Code != http.StatusSeeOther || ok.Header().Get("Location") != "/?reported=1" {
		t.Fatalf("report = %d %q", ok.Code, ok.Header().Get("Location"))
	}
	if n := countReports(t, store); n != 1 {
		t.Fatalf("%d reports, want 1", n)
	}

	// The response sends the learner to Today. The unit is gone from there and from Lessons.
	today := getLearner(t, h, a, ok.Header().Get("Location"))
	if strings.Contains(today, "Reported unit") || strings.Contains(today, "/lesson/"+itoa(reported)+"/0") {
		t.Fatal("the reported unit is still on Today")
	}
	if !strings.Contains(today, "That lesson is hidden for you.") || !strings.Contains(today, "/lesson/"+itoa(kept)+"/0") {
		t.Fatalf("Today after the report:\n%s", articleAttrs(today))
	}
	lessons := getLearner(t, h, a, "/lessons")
	if strings.Contains(lessons, "Reported unit") || !strings.Contains(lessons, "Kept unit") || !strings.Contains(lessons, "Mine only") {
		t.Fatal("Lessons after the report")
	}
	// The reported unit drops out. The kept shared unit and this learner's unit remain.
	if !strings.Contains(getLearner(t, h, a, "/progress"), ">0/2</bdi>") {
		t.Fatal("Progress still counts the reported unit")
	}
	for _, p := range []string{"/lesson/" + itoa(reported) + "/0", "/lessons/" + itoa(reported) + "/report"} {
		if code := codeFor(t, h, a, p); code != http.StatusNotFound {
			t.Fatalf("GET %s = %d after the report, want 404", p, code)
		}
	}
	if body := getLearner(t, h, a, "/lesson"); strings.Contains(body, "Reported unit") {
		t.Fatal("the lesson player still opens the reported unit")
	}
	// It is hidden for this learner only.
	if !strings.Contains(getLearner(t, h, b, "/"), "/lesson/"+itoa(reported)+"/0") || !strings.Contains(getLearner(t, h, b, "/lessons"), "Reported unit") {
		t.Fatal("a report hid the unit for another learner")
	}
	if audit := scalar(t, shellPaths[store], `SELECT COUNT(*) FROM publish_audit WHERE action = 'report_hide' AND unit_id = ?`, reported); audit != 1 {
		t.Fatalf("report_hide audit rows = %d", audit)
	}

	// Reporting a learner's own unit takes it down for them.
	own := formPost(t, h, a, "/lessons/"+itoa(mine)+"/report", form(url.Values{"reason": {"too_hard"}, "note": {""}}), nil)
	if own.Code != http.StatusSeeOther {
		t.Fatalf("own unit report = %d", own.Code)
	}
	if strings.Contains(getLearner(t, h, a, "/lessons"), "Mine only") {
		t.Fatal("a reported own unit is still listed")
	}

	// Hebrew shell keeps its language through the redirect.
	if rec := formPost(t, h, a, "/lessons/"+itoa(kept)+"/report?lang=he", form(nil), nil); rec.Header().Get("Location") != "/?lang=he&reported=1" {
		t.Fatalf("hebrew redirect %q", rec.Header().Get("Location"))
	}
}

func countReports(t *testing.T, store *db.Store) int {
	t.Helper()
	return scalar(t, shellPaths[store], `SELECT COUNT(*) FROM unit_reports`)
}

func TestReportFormPageAndSheet(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", now)
	id := shellUnit(t, store, "A unit", 1, "shared", 2)
	page := getLearner(t, h, l, "/lessons/"+itoa(id)+"/report")
	for _, want := range []string{
		`action="/lessons/` + itoa(id) + `/report"`, `name="_csrf" value="` + l.csrf + `"`,
		`value="wrong_hebrew"`, `value="wrong_translation"`, `value="broken_audio"`, `value="too_hard"`, `value="other"`,
		`maxlength="500"`, "Wrong Hebrew", "Wrong translation", "Broken audio", "Too hard",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("report page lacks %s", want)
		}
	}
	// A refused post keeps what was typed and says why.
	rec := formPost(t, h, l, "/lessons/"+itoa(id)+"/report", url.Values{"reason": {"other"}, "note": {strings.Repeat("z", 501)}, "_csrf": {l.csrf}}, nil)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `role="alert"`) || !strings.Contains(rec.Body.String(), `value="other" required checked`) {
		t.Fatalf("refused post: %d\n%s", rec.Code, rec.Body.String())
	}
}

// ---- ac-2 (render half): five destinations, full labels --------------------

func TestDockHasFiveDestinationsWithFullLabels(t *testing.T) {
	for _, lang := range []string{"en", "he"} {
		body := get(t, New(nil, nil), "/?lang="+lang)
		nav := regexp.MustCompile(`(?s)<nav class="dock".*?</nav>`).FindString(body)
		links := regexp.MustCompile(`(?s)<a [^>]*class="dock__link"[^>]*>.*?</a>`).FindAllString(nav, -1)
		if len(links) != 5 {
			t.Fatalf("%s: %d dock links, want 5", lang, len(links))
		}
		ui := views.UIFor(lang, "m")
		for i, label := range []string{ui.Today, ui.Lessons, ui.Review, ui.Progress, ui.Me} {
			if !strings.Contains(links[i], ">"+label+"<") {
				t.Errorf("%s: link %d lacks the full label %q:\n%s", lang, i, label, links[i])
			}
			if !strings.Contains(links[i], `<svg class="icon" aria-hidden="true"`) {
				t.Errorf("%s: link %d has no icon", lang, i)
			}
		}
		for i, path := range []string{"/", "/lessons", "/review", "/progress", "/me"} {
			want := path
			if lang == "he" {
				want += "?lang=he"
			}
			if !strings.Contains(links[i], `href="`+want+`"`) {
				t.Errorf("%s: link %d does not go to %s", lang, i, want)
			}
		}
	}
	css := get(t, New(nil, nil), "/static/css/shell.css")
	if !strings.Contains(css, "white-space: nowrap") || !strings.Contains(css, "@media (max-width: 359px)") {
		t.Error("shell.css does not keep dock labels on one line at 320px")
	}
}

func TestHeaderHoldsOnlyTheWordmark(t *testing.T) {
	body := get(t, New(nil, nil), "/")
	header := regexp.MustCompile(`(?s)<header class="top">.*?</header>`).FindString(body)
	if header == "" || strings.Contains(header, "<button") || strings.Contains(header, "<form") || strings.Contains(header, "Log out") {
		t.Fatalf("the phone header holds more than the wordmark:\n%s", header)
	}
	if !strings.Contains(header, `class="brand"`) {
		t.Fatal("the wordmark is missing")
	}
}

// ---- Me, Lessons ------------------------------------------------------------

func TestMePage(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", now)
	body := getLearner(t, h, l, "/me")
	for _, mode := range []string{"light", "dark", "auto"} {
		if !strings.Contains(body, `data-theme-choice="`+mode+`" aria-pressed="false"`) {
			t.Errorf("no %s choice", mode)
		}
	}
	for _, want := range []string{`href="/me?lang=en"`, `href="/me?lang=he"`, "a@example.com", `action="/auth/logout"`, `data-logout="/signin"`, `name="_csrf" value="` + l.csrf + `"`, `<span class="me__avatar" aria-hidden="true">A</span>`} {
		if !strings.Contains(body, want) {
			t.Errorf("Me lacks %s", want)
		}
	}
	he := getLearner(t, h, l, "/me?lang=he")
	ui := views.UIFor("he", "m")
	if !strings.Contains(he, ">"+ui.Me+"<") || !strings.Contains(he, `<html lang="he" dir="rtl"`) {
		t.Error("the Hebrew shell does not label Me")
	}
	if n := assertRunDir(t, he, "en", "ltr"); n == 0 {
		t.Error("the Hebrew Me page has no isolated English runs")
	}
	// The script writes data-theme from the choice, and Auto clears it.
	js := get(t, h, "/static/js/shell.js")
	if !strings.Contains(js, `removeAttribute("data-theme")`) || !strings.Contains(js, `setAttribute("data-theme",v)`) {
		t.Error("shell.js does not write data-theme from the choice")
	}
}

func TestLessonsList(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", now)

	empty := getLearner(t, h, l, "/lessons")
	if !strings.Contains(empty, "No lesson published yet") || !strings.Contains(empty, "Ask your tutor") || !strings.Contains(empty, `href="/progress"`) {
		t.Fatal("the empty Lessons list does not say what to do next")
	}

	done := shellUnit(t, store, "Done one", 1, "shared", 1)
	next := shellUnit(t, store, "Next one", 2, "shared", 2)
	shellUnit(t, store, "Later one", 3, "shared", 1)
	shellUnit(t, store, "Made one", 4, "learner:"+itoa(l.user.ID), 1)
	shellUnit(t, store, "Hers", 5, "learner:999", 1)
	finished := now
	if _, err := testharness.InsertLessonRun(context.Background(), store, l.user.ID, done, 0, now.Add(-time.Hour), &finished); err != nil {
		t.Fatal(err)
	}
	body := getLearner(t, h, l, "/lessons")
	rows := regexp.MustCompile(`(?s)<li class="lesson-row" data-state="([a-z]+)">(.*?)</li>`).FindAllStringSubmatch(body, -1)
	if len(rows) != 4 {
		t.Fatalf("%d rows, want 4:\n%s", len(rows), body)
	}
	for i, want := range []struct{ state, title string }{{"done", "Done one"}, {"next", "Next one"}, {"later", "Later one"}, {"later", "Made one"}} {
		if rows[i][1] != want.state || !strings.Contains(rows[i][2], want.title) {
			t.Errorf("row %d = %s %q, want %s %s", i, rows[i][1], rows[i][2], want.state, want.title)
		}
	}
	if !strings.Contains(rows[1][2], `href="/lesson/`+itoa(next)+`/0"`) || strings.Contains(rows[2][2], "<a ") || strings.Contains(rows[0][2], "<a ") {
		t.Error("only the next lesson opens")
	}
	if !strings.Contains(rows[3][2], "Made for you") || strings.Contains(body, "Hers") {
		t.Error("learner units: own is marked, another's is hidden")
	}
}

// ---- the lesson frame -------------------------------------------------------

func TestLessonFrame(t *testing.T) {
	const (
		hello   = "שָׁלוֹם"
		morning = "בֹּקֶר"
	)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", now)
	sp := func(s string) *string { return &s }
	const evening = "עֶרֶב"
	_, _, err := store.UpsertLexemes(context.Background(), []lexicon.Input{
		{TextNFC: hello, Gloss: "hello", Translit: sp("shalom"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: morning, Gloss: "morning", Translit: sp("boker"), Gender: sp("m"), POS: sp("noun")},
		{TextNFC: evening, Gloss: "evening", Translit: sp("erev"), Gender: sp("m"), POS: sp("noun")},
	})
	if err != nil {
		t.Fatal(err)
	}
	id := putLesson(t, store, nil, units.Unit{
		Title: "Greetings", Position: 1, NewWords: []string{hello, morning},
		Lessons: []units.Lesson{{Kind: units.KindLesson, Activities: []units.Activity{
			{Type: units.ActivityStory, Text: hello + " " + morning + " " + evening},
			{Type: units.ActivityRecall, Text: hello},
			{Type: units.ActivityListenPick, Text: morning},
		}}},
	}, "shared")
	publish(t, store, id)
	link := "/lesson/" + itoa(id) + "/0"

	body := getLearner(t, h, l, link)
	for _, want := range []string{
		`<progress class="c-progress" value="0" max="3" aria-label="Lesson progress">`,
		`data-step="1" data-total="3"`, `<h1 class="lf-title" lang="en" dir="ltr">Read the story</h1>`, "Step 1 of 3",
		`href="/lessons/` + itoa(id) + `/report" hx-boost="false" data-sheet="report-sheet"`, `aria-label="Report a problem"`,
		`<dialog id="report-sheet"`, `maxlength="500"`, `aria-label="Back to Today"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("first step lacks %s", want)
		}
	}
	steps := regexp.MustCompile(`<li class="lf-step" data-state="([a-z]+)"( aria-current="step")?>`).FindAllStringSubmatch(body, -1)
	if len(steps) != 3 || steps[0][1] != "current" || steps[0][2] == "" || steps[1][1] != "todo" || steps[2][1] != "todo" {
		t.Fatalf("outline = %v", steps)
	}
	if !strings.Contains(body, "Read the story") || !strings.Contains(body, "Recall the meaning") || !strings.Contains(body, "Listen and pick") {
		t.Error("the outline does not name the steps")
	}
	if strings.Contains(body, "listen_pick") || strings.Contains(body, "ActivityRecall") {
		t.Error("an internal activity name leaks into the frame")
	}
	if strings.Count(body, "<h1") != 1 {
		t.Errorf("%d h1 on the lesson page, want 1", strings.Count(body, "<h1"))
	}

	// A step swap brings the whole frame back, so the bar and the outline move.
	run := runValue(t, body)
	step2 := postLesson(t, h, l.cookie, link, run, "continue", true, l.csrf).Body.String()
	for _, want := range []string{`value="1" max="3"`, `data-step="2" data-total="3"`, "Step 2 of 3", "Recall the meaning", `<article id="lesson-frame"`, `hx-target="#lesson-frame"`, `hx-select="unset"`, `data-no-transition`} {
		if !strings.Contains(step2, want) {
			t.Errorf("second step lacks %s", want)
		}
	}
	steps = regexp.MustCompile(`<li class="lf-step" data-state="([a-z]+)"( aria-current="step")?>`).FindAllStringSubmatch(step2, -1)
	if len(steps) != 3 || steps[0][1] != "done" || steps[1][1] != "current" || steps[2][1] != "todo" {
		t.Fatalf("outline after one step = %v", steps)
	}
	if strings.Contains(step2, "<html") {
		t.Error("a swap returned a whole page")
	}

	// Every answer is a c-answer submit button with a key, and nothing else is an answer.
	buttons := regexp.MustCompile(`<button [^>]*class="c-answer"[^>]*>`).FindAllString(step2, -1)
	if len(buttons) != lessonMinChoices {
		t.Fatalf("%d c-answer buttons, want %d:\n%s", len(buttons), lessonMinChoices, step2)
	}
	for _, b := range buttons {
		if !strings.Contains(b, `type="submit"`) || !strings.Contains(b, `name="answer"`) || !strings.Contains(b, `value="`) {
			t.Errorf("answer button: %s", b)
		}
	}
	if strings.Contains(step2, `class="choice"`) {
		t.Error("an answer still uses the old .choice class")
	}
	if strings.Contains(step2, `c-answer__key" aria-hidden="true">1<`) || strings.Contains(step2, `c-answer__key" aria-hidden="true">2<`) {
		t.Error("lesson choices show key badges, and the lesson has no shortcut legend")
	}

	// Answering shows the result in a sheet with the one gold button, and the bar fills.
	helloID := int64(scalar(t, shellPaths[store], `SELECT id FROM lexemes WHERE text_nfc = ?`, hello))
	right := postLesson(t, h, l.cookie, link, run, itoa(helloID), true, l.csrf).Body.String()
	if !strings.Contains(right, `value="2" max="3"`) || !strings.Contains(right, `data-step="2" data-total="3"`) {
		t.Errorf("the bar did not fill after the answer:\n%s", right)
	}
	if !strings.Contains(right, `class="lf-result"`) || !strings.Contains(right, `class="c-sheet"`) || !strings.Contains(right, "feedback feedback--right") {
		t.Errorf("no result sheet after an answer:\n%s", right)
	}
	if strings.Contains(right, `class="c-answer"`) {
		t.Error("the answers are still there after the answer")
	}
	// The report sheet is a closed dialog until the flag opens it, so it is not on screen.
	visible := regexp.MustCompile(`(?s)<dialog.*?</dialog>`).ReplaceAllString(right, "")
	if goldCount(visible) != 1 {
		t.Errorf("%d gold buttons in the lesson frame after an answer, want 1", goldCount(visible))
	}
}

func TestFrameForCounts(t *testing.T) {
	lesson := db.CourseLesson{UnitID: 7, Activities: []units.Activity{{Type: "story"}, {Type: "recall"}, {Type: "cloze"}, {Type: "mystery"}}}
	page := views.NewPage("en", "tok", "lesson")
	for _, c := range []struct {
		name        string
		run         db.PlayerRun
		step, value int
		kind        string
		done        bool
	}{
		{"first step", db.PlayerRun{ActivityIx: 0, Phase: "ask"}, 1, 0, "Read the story", false},
		{"answer showing", db.PlayerRun{ActivityIx: 1, Phase: "reveal"}, 2, 2, "Recall the meaning", false},
		{"third, asking", db.PlayerRun{ActivityIx: 2, Phase: "ask"}, 3, 2, "Fill the gap", false},
		{"an unknown type is Practice", db.PlayerRun{ActivityIx: 3, Phase: "ask"}, 4, 3, "Practice", false},
		{"finished", db.PlayerRun{ActivityIx: 4, Finished: true}, 4, 4, "All done", true},
	} {
		f := frameFor(page, lesson, c.run, false)
		if f.Step != c.step || f.Value != c.value || f.Kind != c.kind || f.Done != c.done || f.Total != 4 {
			t.Errorf("%s: %+v", c.name, f)
		}
	}
	empty := frameFor(page, db.CourseLesson{UnitID: 7}, db.PlayerRun{}, false)
	if empty.Total != 1 || empty.Kind != "Practice" {
		t.Errorf("a lesson with no steps: %+v", empty)
	}
}

// ---- the check-in route -----------------------------------------------------

func TestCheckinRoute(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	eng := dueEngine()
	store, h := openShell(t, fixedClock(now), eng, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", now)
	finishPlacement(t, store, l.user.ID, now.Add(-40*24*time.Hour))

	post := func(extra url.Values) *httptest.ResponseRecorder {
		v := url.Values{"_csrf": {l.csrf}}
		for k, vs := range extra {
			v[k] = vs
		}
		return formPost(t, h, l, "/_hx/placement/recheck", v, nil)
	}
	if rec := formPost(t, h, l, "/_hx/placement/recheck", url.Values{}, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("no csrf = %d", rec.Code)
	}
	if len(eng.asked) != 0 {
		t.Fatal("a refused post opened a check-in")
	}
	rec := post(url.Values{"skill": {"listen"}}) // the form's skill is ignored: the server chooses
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/placement" {
		t.Fatalf("check-in = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if len(eng.asked) != 1 || eng.asked[0] != placement.SkillRead {
		t.Fatalf("asked %v, want the lower skill, read", eng.asked)
	}

	// Level skills ask listening.
	eng.latest.Reading.Level = 3
	post(nil)
	if eng.asked[1] != placement.SkillListen {
		t.Fatalf("level skills asked %q, want listen", eng.asked[1])
	}
	// A run already open is resumed by the placement screen.
	eng.newErr = placement.ErrNotReady
	if rec := post(nil); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/placement" {
		t.Fatalf("open run = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	eng.newErr = placement.ErrNoPlacement
	if rec := post(nil); rec.Header().Get("Location") != "/" {
		t.Fatalf("no placement goes to %q", rec.Header().Get("Location"))
	}
	eng.newErr = errors.New("boom")
	if rec := post(nil); rec.Code != http.StatusInternalServerError {
		t.Fatalf("engine error = %d", rec.Code)
	}

	// With no engine the check-in is off, and Today does not offer it.
	store2, h2 := openShell(t, fixedClock(now), nil, nil)
	l2 := addLearner(t, store2, "a@example.com", "sub-b", now)
	finishPlacement(t, store2, l2.user.ID, now.Add(-40*24*time.Hour))
	if rec := formPost(t, h2, l2, "/_hx/placement/recheck", url.Values{"_csrf": {l2.csrf}}, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("no engine = %d", rec.Code)
	}
	if strings.Contains(getLearner(t, h2, l2, "/"), "check-in") {
		t.Fatal("Today offers a check-in with no engine")
	}
}

func TestCheckinOnTodayIsNotDueUntilTheEngineSaysSo(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	eng := dueEngine()
	eng.status.Due = false
	store, h := openShell(t, fixedClock(now), eng, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", now)
	finishPlacement(t, store, l.user.ID, now.Add(-time.Hour))
	if got := todayAction(t, getLearner(t, h, l, "/")); got != "none" {
		t.Fatalf("not due: %q", got)
	}
	eng.status.Due = true
	eng.status.Late = true
	eng.status.LastAt = now.Add(-45 * 24 * time.Hour)
	body := getLearner(t, h, l, "/")
	if todayAction(t, body) != "checkin" || !strings.Contains(body, "Check-in · overdue") || !strings.Contains(body, "6 weeks since your last check") || !strings.Contains(body, "Three minutes of reading") {
		t.Fatalf("due check-in:\n%s", body)
	}
	// An engine that fails leaves Today working, with the check-in off.
	eng.statusErr = errors.New("down")
	if got := todayAction(t, getLearner(t, h, l, "/")); got != "none" {
		t.Fatalf("a failing engine: %q", got)
	}
}

// ---- hebrew -----------------------------------------------------------------

func TestTodayHebrewShellKeepsDirections(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	store, h := openShell(t, fixedClock(now), nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", now)
	body := getLearner(t, h, l, "/?lang=he")
	if !strings.Contains(body, `<html lang="he" dir="rtl"`) || !strings.Contains(body, `href="/placement?lang=he"`) {
		t.Fatal("the Hebrew shell lost its language")
	}
	if n := assertRunDir(t, body, "en", "ltr"); n == 0 {
		t.Fatal("no isolated English runs on the Hebrew Today")
	}
	if !strings.Contains(body, "בְּדִיקַת רָמָה") {
		t.Fatal("the placement action has no Hebrew title")
	}
}
