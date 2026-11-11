package web

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/jobs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// SPEC-010: the Progress headline is the proved ladder, each count lists those
// words, the comparison panel lasts 14 days, and Today uses the same proved count.
func TestProgressLadderHeadlineAndComparison(t *testing.T) {
	now := time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC)
	clk := fixedClock(now)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-ladder", now)
	setLearnerZone(t, store, l.user.ID, "America/Chicago")
	ctx := context.Background()

	dog := addLexeme(t, store, "כֶּלֶב", "dog", "kelev", "m", "")
	cat := addLexeme(t, store, "חָתוּל", "cat", "chatul", "m", "")
	horse := addLexeme(t, store, "סוּס", "horse", "sus", "m", "")
	cow := addLexeme(t, store, "פָּרָה", "cow", "para", "f", "")
	fish := addLexeme(t, store, "דָּג", "fish", "dag", "m", "")
	bird := addLexeme(t, store, "עוֹף", "bird", "of", "m", "")
	table := addLexeme(t, store, "שֻׁלְחָן", "table", "shulchan", "m", "")

	at := func(y, m, d, hour int) time.Time {
		return time.Date(y, time.Month(m), d, hour, 0, 0, 0, time.UTC)
	}
	ladderCard(t, store, l.user.ID, dog, "recall", at(2026, 11, 1, 18), 0.2, []int{3, 3, 3}, []time.Time{
		at(2026, 11, 1, 18), at(2026, 11, 4, 18), at(2026, 11, 8, 18),
	})
	ladderCard(t, store, l.user.ID, cat, "recall", at(2026, 11, 1, 18), 10, []int{3}, []time.Time{at(2026, 11, 2, 18)})
	ladderCard(t, store, l.user.ID, horse, "recall", at(2026, 11, 1, 18), 10, []int{3, 3, 3, 1}, []time.Time{
		at(2026, 11, 1, 18), at(2026, 11, 4, 18), at(2026, 11, 8, 18), at(2026, 11, 9, 18),
	})
	ladderCard(t, store, l.user.ID, cow, "recall", at(2026, 11, 1, 18), 10, nil, nil)
	ladderCard(t, store, l.user.ID, bird, "listen", at(2026, 11, 1, 18), 10, nil, nil)
	// Placement-only: not taught, so it is in none of the four counts.
	placed := ladderCard(t, store, l.user.ID, fish, "recall", at(2026, 11, 1, 18), 10, []int{3}, []time.Time{at(2026, 11, 1, 18)})
	insertLadderAttempt(t, store, l.user.ID, fish, "recall", true, at(2026, 11, 1, 18), "placement")
	_ = placed

	// Rule 4 still counts this word when the voided answer is included, and
	// drops it once the void applies. The span is too short to be proved.
	ruleCard := ladderCard(t, store, l.user.ID, table, "recall", at(2026, 11, 8, 18), 40, []int{3, 3, 3}, []time.Time{
		at(2026, 11, 8, 18), at(2026, 11, 9, 18), at(2026, 11, 10, 18),
	})
	voidAttempt := insertLadderAttempt(t, store, l.user.ID, table, "recall", true, at(2026, 11, 8, 18), "review")
	insertLadderAttempt(t, store, l.user.ID, table, "recall", true, at(2026, 11, 9, 18), "review")
	insertLadderAttempt(t, store, l.user.ID, table, "recall", true, at(2026, 11, 10, 18), "review")
	_ = ruleCard
	if _, err := store.InsertAnswerVoids(ctx, "ladder-compare", now, []db.AnswerVoidWrite{{
		AttemptID: voidAttempt, LearnerID: l.user.ID, Reason: db.VoidReasonAlsoRight, Actor: "test",
	}}); err != nil {
		t.Fatal(err)
	}

	// Two saved totals under two rule versions. The chart must not draw either.
	ret := 0.9
	for _, row := range []struct {
		day string
		n   int
		rv  int
	}{
		{"2026-11-08", 77, 4},
		{"2026-11-09", 66, jobs.RuleVersion},
	} {
		var snaps []db.DailySnapshotRow
		for _, skill := range learner.Skills {
			for _, st := range learner.States {
				snaps = append(snaps, db.DailySnapshotRow{
					Skill: skill, State: string(st), Count: 1, EstKnown: 10, Retention: &ret, RuleVersion: row.rv,
				})
			}
		}
		if err := store.InDailyTx(ctx, func(tx *db.DailyTx) error {
			if err := tx.InsertSnapshot(ctx, l.user.ID, row.day, snaps); err != nil {
				return err
			}
			return tx.InsertTotals(ctx, l.user.ID, row.day, row.n, row.rv)
		}); err != nil {
			t.Fatal(err)
		}
	}

	model := learner.New(store, clk)
	sum, err := model.Summary(ctx, l.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	recall := skillSummary(sum, "recall").Ladder
	listen := skillSummary(sum, "listen").Ladder
	if recall.Proved+recall.RecalledLater+recall.Slipping+recall.NotYetAsked != recall.Taught {
		t.Fatalf("model recall rungs = %+v", recall)
	}
	// A word taught in any skill is on every skill's ladder. The listen card
	// puts the bird on recall as not yet asked, and the recall cards do the
	// same for listen. The placement-only fish is in neither.
	if recall.Taught != 6 || recall.Proved != 1 || recall.NotYetAsked != 2 || listen.Taught != 6 || listen.NotYetAsked != 6 {
		t.Fatalf("recall %+v listen %+v", recall, listen)
	}
	withVoids, err := model.Rule4Known(ctx, l.user.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	afterVoid, err := model.Rule4Known(ctx, l.user.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if withVoids <= afterVoid {
		t.Fatalf("rule 4 with voids %d, after %d", withVoids, afterVoid)
	}

	body := getLearner(t, h, l, "/progress")
	for _, gone := range []string{
		"Words you know", "Say it from English", "Placement estimate:", "data-verdict=",
		"You gained", views.LabelLearnedTodayCheckTomorrow, views.LabelKnownExplain,
		views.PhraseAppTaught, "%", "What changed",
	} {
		if strings.Contains(body, gone) {
			t.Errorf("progress still has %q", gone)
		}
	}
	if !strings.Contains(body, "say words from English") || !strings.Contains(body, `class="btn btn-primary progress__next"`) {
		t.Fatalf("missing the can-do sentence or the next action:\n%s", body)
	}
	if recall.Proved < 1 {
		t.Fatal("the fixture has nothing proved to say")
	}

	expect := map[string][]string{
		"proved":         {"כֶּלֶב"},
		"recalled_later": {"חָתוּל", "שֻׁלְחָן"},
		"slipping":       {"סוּס"},
		"not_yet_asked":  {"פָּרָה", "עוֹף"},
	}
	absent := []string{"דָּג"}
	for rung, want := range expect {
		list := getLearner(t, h, l, "/me/words?skill=recall&ladder="+rung)
		if !strings.Contains(list, "Hebrew from English") || !strings.Contains(list, `data-ladder="`+rung+`"`) {
			t.Fatalf("%s list is not that rung:\n%s", rung, list)
		}
		for _, he := range want {
			if !strings.Contains(list, he) {
				t.Errorf("%s list missing %s", rung, he)
			}
		}
		for _, he := range absent {
			if strings.Contains(list, he) {
				t.Errorf("%s list includes %s", rung, he)
			}
		}
		for other, others := range expect {
			if other == rung {
				continue
			}
			for _, he := range others {
				if strings.Contains(list, he) {
					t.Errorf("%s list includes %s from %s", rung, he, other)
				}
			}
		}
	}

	if strings.Contains(body, `data-panel="what-changed"`) || strings.Contains(body, views.LabelLadderRule) {
		t.Fatal("the comparison numbers are on the progress screen")
	}
	_ = withVoids
	_ = afterVoid

	skillChart := section(t, getLearner(t, h, l, "/progress?skill=recall"), "c-card pchart", `data-chart="est"`)
	if strings.Contains(skillChart, "Nov 8") || strings.Contains(skillChart, "old rule") {
		t.Fatalf("skill chart drew the other rule version:\n%s", skillChart)
	}
	if !strings.Contains(skillChart, "Nov 9") {
		t.Fatalf("skill chart dropped the current rule version:\n%s", skillChart)
	}

	home := getLearner(t, h, l, "/")
	if !strings.Contains(home, `data-known="`+strconv.Itoa(recall.Proved)+`"`) {
		t.Fatalf("Today data-known: %s, want %d", articleAttrs(home), recall.Proved)
	}

	if _, err := store.ExecContext(ctx, `
		INSERT INTO taught_words (learner_id, lexeme_id, taught_on, cards_created, created_at)
		VALUES (?, ?, '2026-11-01', '', ?)`, l.user.ID, cow, db.FormatTime(now)); err != nil {
		t.Fatal(err)
	}
	afterTutor := getLearner(t, h, l, "/progress")
	if strings.Contains(afterTutor, views.PhraseAppTaught) || strings.Contains(afterTutor, views.PhraseTutorTaught) {
		t.Fatal("progress showed a denominator")
	}
	if !strings.Contains(afterTutor, "say words from English") {
		t.Fatal("the sentence changed when a tutor word was recorded")
	}
}

func TestLadderCompareIsGoneAfter14Days(t *testing.T) {
	show := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	gone := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		at   time.Time
		want bool
	}{
		{"ships", show, true},
		{"last morning", time.Date(2026, 11, 22, 23, 0, 0, 0, time.UTC), true},
		{"day 15", gone, false},
		{"before", progNow, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ladderCompareOpen(tc.at); got != tc.want {
				t.Fatalf("open at %s = %v, want %v", tc.at, got, tc.want)
			}
		})
	}

	clk := fixedClock(gone)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-ladder-gone", gone)
	id := addLexeme(t, store, "כֶּלֶב", "dog", "kelev", "m", "")
	ladderCard(t, store, l.user.ID, id, "recall", gone.Add(-time.Hour), 10, nil, nil)
	body := getLearner(t, h, l, "/progress")
	if strings.Contains(body, `data-panel="what-changed"`) || strings.Contains(body, views.LabelWhatChanged) {
		t.Fatal("the comparison panel is still on the page after 14 days")
	}
	if !strings.Contains(body, `data-cando="1"`) || !strings.Contains(body, `class="btn btn-primary progress__next"`) {
		t.Fatal("the can-do sentence is gone with the panel")
	}
}

func TestProgressLadderCSSWrapsOnAPhone(t *testing.T) {
	css := readProgressCSS(t)
	// One row per count on a phone, label left and share right, two columns from 640px.
	if !strings.Contains(css, ".pknow__rungs { display: grid; grid-template-columns: minmax(0, 1fr);") {
		t.Fatal("ladder counts do not stack one to a row on a phone")
	}
	if !strings.Contains(css, ".pknow__rungs { grid-template-columns: repeat(2, minmax(0, 1fr));") {
		t.Fatal("ladder counts do not use two columns on a wide screen")
	}
	if !strings.Contains(css, ".pchanged__nums {\n  display: grid;\n  grid-template-columns: 1fr;") {
		t.Fatal("the comparison panel does not stack on a narrow screen")
	}
}

func ladderCard(t *testing.T, store *db.Store, userID, lexeme int64, skill string, created time.Time, stability float64, ratings []int, at []time.Time) int64 {
	t.Helper()
	last := created
	if len(at) > 0 {
		last = at[len(at)-1]
	}
	id, err := store.InsertCard(context.Background(), db.Card{
		UserID: userID, LexemeID: lexeme, Skill: skill, State: fsrs.StateReview,
		Due: created, Stability: stability, Difficulty: 5, Reps: len(ratings),
		LastReview: last, CreatedAt: created,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, rating := range ratings {
		if _, err := store.ExecContext(context.Background(), `
			INSERT INTO review_log (card_id, rating, reviewed_at, elapsed_days, scheduled_days, peeked)
			VALUES (?, ?, ?, 1, 1, 0)`, id, rating, db.FormatTime(at[i])); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func insertLadderAttempt(t *testing.T, store *db.Store, userID, lexeme int64, skill string, right bool, at time.Time, source string) int64 {
	t.Helper()
	correct := 0
	if right {
		correct = 1
	}
	res, err := store.ExecContext(context.Background(), `
		INSERT INTO attempts (
			user_id, lexeme_id, skill, activity, source, prompt, response,
			correct, dont_know, hint_used, latency_ms, created_at
		) VALUES (?, ?, ?, 'test', ?, '', '', ?, 0, 0, 0, ?)`,
		userID, lexeme, skill, source, correct, db.FormatTime(at))
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func hrefOf(t *testing.T, row, rung string) string {
	t.Helper()
	re := regexp.MustCompile(`data-rung="` + rung + `"[^>]*href="([^"]+)"`)
	m := re.FindStringSubmatch(row)
	if m == nil {
		t.Fatalf("no href for %s in\n%s", rung, row)
	}
	return strings.ReplaceAll(m[1], "&amp;", "&")
}

func panelNumber(t *testing.T, panel, key string) int {
	t.Helper()
	re := regexp.MustCompile(`data-count="` + key + `"[^>]*>\s*<bdi[^>]*>([0-9]+)</bdi>`)
	m := re.FindStringSubmatch(panel)
	if m == nil {
		t.Fatalf("no %s number in\n%s", key, panel)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func readProgressCSS(t *testing.T) string {
	t.Helper()
	b, err := Assets.ReadFile("static/css/progress.css")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
