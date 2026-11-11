package web

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db/testharness"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/jobs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcpstore"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// seedLearnerDaySnapshots writes one snapshot row per skill per day from first
// through last (inclusive). INIT-026/SPEC-006.
func seedLearnerDaySnapshots(t *testing.T, store *db.Store, userID int64, first, last string) {
	t.Helper()
	start, err := time.Parse(dayLayout, first)
	if err != nil {
		t.Fatal(err)
	}
	end, err := time.Parse(dayLayout, last)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		day := d.Format(dayLayout)
		var rows []db.DailySnapshotRow
		knownAny := 0
		for i, skill := range learner.Skills {
			ret := 0.8
			for _, st := range learner.States {
				n := 1
				if st == learner.Known {
					n = 10 + i
				}
				rows = append(rows, db.DailySnapshotRow{
					Skill: skill, State: string(st), Count: n,
					EstKnown: 100 + float64(i), Retention: &ret, RuleVersion: jobs.RuleVersion,
				})
			}
			if 10+i+1 > knownAny { // known + solid(1)
				knownAny = 10 + i + 1
			}
		}
		if err := store.InDailyTx(ctx, func(tx *db.DailyTx) error {
			if err := tx.InsertSnapshot(ctx, userID, day, rows); err != nil {
				return err
			}
			return tx.InsertTotals(ctx, userID, day, knownAny, jobs.RuleVersion)
		}); err != nil {
			t.Fatal(err)
		}
	}
}

// Tests for the Progress page (INIT-015/SPEC-018). The fixture has 90 days of snapshots whose
// numbers are known to the test, a learner model with real evidence, and the events, report,
// and placements around them, so a number on the page is checked against the number that made it.

var (
	progNow  = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	progLast = "2026-10-17" // the newest snapshot day: the night before progNow
)

// progEst and progRet are the estimated words known and the retention the fixture writes for the
// i-th of 90 days (0 is the oldest) of a skill. The numbers are fractional on purpose.
func progEst(i int, skill string) float64 {
	return 100 + 2*float64(skillIndex(skill)) + 0.4*float64(i)
}

func progRet(i int, skill string) float64 {
	return 0.78 + 0.0012*float64(i) + 0.01*float64(skillIndex(skill))
}

func skillIndex(skill string) int {
	for i, s := range learner.Skills {
		if s == skill {
			return i
		}
	}
	return -1
}

// progKnown is the words known (known plus solid) the fixture writes on day i. Spelling loses
// words over the last days, so "This week" has a loss to show.
func progKnown(i int, skill string) (known, solid int) {
	base := 20 + 3*skillIndex(skill)
	if skill == "spell" && i >= 85 {
		return base + 85/3 - (i - 84), 5
	}
	return base + i/3, 5
}

// progDay is the day of the i-th snapshot of 90, ending at progLast.
func progDay(i int) string {
	end, _ := time.Parse(dayLayout, progLast)
	return end.AddDate(0, 0, -(89 - i)).Format(dayLayout)
}

func seedProgressSnapshots(t *testing.T, store *db.Store, userID int64, days int, est func(int, string) float64) {
	t.Helper()
	ctx := context.Background()
	for i := 90 - days; i < 90; i++ {
		var rows []db.DailySnapshotRow
		knownAny := 0
		for _, skill := range learner.Skills {
			known, solid := progKnown(i, skill)
			if known+solid > knownAny {
				knownAny = known + solid
			}
			counts := map[string]int{"unseen": 300 - i, "seen": 40, "struggling": 6, "regressed": 2, "learning": 30 + i%5, "known": known, "solid": solid}
			ret := progRet(i, skill)
			for _, st := range learner.States {
				rows = append(rows, db.DailySnapshotRow{
					Skill: skill, State: string(st), Count: counts[string(st)],
					EstKnown: est(i, skill), Retention: &ret, RuleVersion: jobs.RuleVersion,
				})
			}
		}
		day := progDay(i)
		if err := store.InDailyTx(ctx, func(tx *db.DailyTx) error {
			if err := tx.InsertSnapshot(ctx, userID, day, rows); err != nil {
				return err
			}
			return tx.InsertTotals(ctx, userID, day, knownAny, jobs.RuleVersion)
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func progEvent(t *testing.T, store *db.Store, e db.RegressionEvent) {
	t.Helper()
	ctx := context.Background()
	if err := store.InDailyTx(ctx, func(tx *db.DailyTx) error { return tx.InsertRegression(ctx, e) }); err != nil {
		t.Fatal(err)
	}
}

// progressApp opens the app with one learner who has the 90-day fixture, real evidence, events, a
// weekly report, and two placement runs.
func progressApp(t *testing.T) (*db.Store, learnerFix, []lexWord, clock.Clock, func(path string) string) {
	t.Helper()
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l, words := seedProgressLearner(t, store, clk, "a@example.com", "sub-a")
	get := func(path string) string { return getLearner(t, h, l, path) }
	return store, l, words, clk, get
}

// seedProgressLearner gives one learner the whole fixture. A second learner, created first, has
// other numbers and a slip of its own, so the page under test is never the one with id 1 and a
// leak from another learner would show.
func seedProgressLearner(t *testing.T, store *db.Store, clk clock.Clock, email, sub string) (learnerFix, []lexWord) {
	t.Helper()
	other := addLearner(t, store, "other-"+email, "other-"+sub, progNow)
	l := addLearner(t, store, email, sub, progNow)
	words := seedLexicon(t, store, progNow, 8, false, true)
	seedProgressSnapshots(t, store, l.user.ID, 90, progEst)

	// Evidence: words[0] struggles in listening now, the rest have a little history.
	strugglingWord(t, store, clk, l.user.ID, words[0].id, "listen", 5, 0)
	strugglingWord(t, store, clk, l.user.ID, words[3].id, "read", 1, 4)
	strugglingWord(t, store, clk, l.user.ID, words[4].id, "recall", 0, 3)

	// Events: words[0] slipped in listening this week (and still struggles), words[2] slipped in
	// listening two weeks ago and has no evidence now, so it is back; band 1 slipped yesterday.
	progEvent(t, store, db.RegressionEvent{UserID: l.user.ID, Kind: db.EventWord, LexemeID: words[0].id, Skill: "listen", FromState: "known", ToState: "struggling", Day: "2026-10-16", Cause: "confusion"})
	progEvent(t, store, db.RegressionEvent{UserID: l.user.ID, Kind: db.EventWord, LexemeID: words[1].id, Skill: "read", FromState: "solid", ToState: "regressed", Day: "2026-10-15", Cause: "unexplained"})
	progEvent(t, store, db.RegressionEvent{UserID: l.user.ID, Kind: db.EventWord, LexemeID: words[2].id, Skill: "listen", FromState: "known", ToState: "struggling", Day: "2026-10-02", Cause: "decoding"})
	slipEvent(t, store, l.user.ID, "2026-10-17", 1)
	// Someone else's event must never show.
	progEvent(t, store, db.RegressionEvent{UserID: other.user.ID, Kind: db.EventWord, LexemeID: words[5].id, Skill: "recall", FromState: "known", ToState: "struggling", Day: "2026-10-16", Cause: "spelling"})
	seedProgressSnapshots(t, store, other.user.ID, 90, func(i int, skill string) float64 { return 777 + float64(i) })

	if _, err := store.ExecContext(context.Background(), `
		INSERT INTO weekly_reports (user_id, week_start, payload_json, summary, created_at) VALUES (?, '2026-10-05', '{}', 'Older report.', ?), (?, '2026-10-12', '{}', 'You gained 14 words & lost <3>.', ?)`,
		l.user.ID, db.FormatTime(progNow), l.user.ID, db.FormatTime(progNow)); err != nil {
		t.Fatal(err)
	}
	// Placement: an initial v3 run and a check-in, and a v2 run.
	ctx := context.Background()
	first, err := store.StartPlacementV3(ctx, l.user.ID, progNow.AddDate(0, 0, -44), 1, "initial", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishPlacement(ctx, first, progNow.AddDate(0, 0, -44), 3, "exhausted", `{"estimated_known_words":142,"listening":{"tested":true,"level":3,"estimate":142},"reading":{"tested":true,"level":2,"estimate":90}}`); err != nil {
		t.Fatal(err)
	}
	check, err := store.StartPlacementV3(ctx, l.user.ID, progNow.AddDate(0, 0, -16), 2, "recheck", "listen", 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishPlacement(ctx, check, progNow.AddDate(0, 0, -16), 3, "exhausted", `{"estimated_known_words":164,"listening":{"tested":true,"level":3,"estimate":164},"reading":{"tested":false,"level":0,"estimate":0}}`); err != nil {
		t.Fatal(err)
	}
	return l, words
}

// section returns the card with the marker, from its opening tag to its closing tag.
func section(t *testing.T, body, class, attr string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<section class="` + regexp.QuoteMeta(class) + `"[^>]*` + regexp.QuoteMeta(attr) + `[^>]*>.*?</section>`)
	s := re.FindString(body)
	if s == "" {
		t.Fatalf("no <section class=%q %s> in the page", class, attr)
	}
	return s
}

var tableRow = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
var tableCell = regexp.MustCompile(`(?s)<(?:th|td)[^>]*><bdi[^>]*>(.*?)</bdi></(?:th|td)>`)

// tableRows returns the body rows of the table in html, as their cell texts.
func tableRows(html string) [][]string {
	body := html
	if i := strings.Index(html, "<tbody>"); i >= 0 {
		body = html[i:]
	}
	var out [][]string
	for _, tr := range tableRow.FindAllStringSubmatch(body, -1) {
		var cells []string
		for _, c := range tableCell.FindAllStringSubmatch(tr[1], -1) {
			cells = append(cells, c[1])
		}
		out = append(out, cells)
	}
	return out
}

func summaryOf(t *testing.T, store *db.Store, clk clock.Clock, userID int64) learner.Summary {
	t.Helper()
	sum, err := learner.New(store, clk).Summary(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	return sum
}

func skillSummary(sum learner.Summary, skill string) learner.SkillSummary {
	for _, s := range sum.Skills {
		if s.Skill == skill {
			return s
		}
	}
	return learner.SkillSummary{}
}

// ---- ac-1: each tab shows its three charts, and the numbers are the model's and the snapshots' ----

func TestProgressTabsShowThreeChartsWithTheModelsAndSnapshotsNumbers(t *testing.T) {
	store, l, _, clk, get := progressApp(t)
	sum := summaryOf(t, store, clk, l.user.ID)
	headline := regexp.MustCompile(`class="count"><bdi lang="en" dir="ltr">([0-9]+)</bdi>`)

	for si, skill := range learner.Skills {
		t.Run(skill, func(t *testing.T) {
			body := get("/progress?skill=" + skill)
			if got := strings.Count(body, `<svg class="pc pc--bar"`); got != 1 {
				t.Fatalf("%d state bars, want 1", got)
			}
			if got := strings.Count(body, `<svg class="pc pc--line"`); got != 2 {
				t.Fatalf("%d line charts, want 2 (estimated words, retention)", got)
			}
			if !strings.Contains(body, `data-skill="`+skill+`"`) {
				t.Fatal("the page does not say which skill it shows")
			}
			ss := skillSummary(sum, skill)

			// The headline and every legend count are learner.Summary's.
			if m := headline.FindStringSubmatch(body); m == nil || m[1] != fmt.Sprintf("%.0f", ss.EstimatedKnown) {
				t.Errorf("headline %v, want %.0f (Summary.EstimatedKnown)", m, ss.EstimatedKnown)
			}
			know := section(t, body, "c-card pknow", "")
			for _, st := range learner.States {
				want := strconv.Itoa(ss.Counts[st])
				re := regexp.MustCompile(`data-state="` + string(st) + `"><span class="pc__sw"[^>]*></span> <span class="pc__legend-label"><bdi[^>]*>[^<]*</bdi></span> <span class="pc__legend-n"><bdi[^>]*>([0-9]+)</bdi>`)
				if m := re.FindStringSubmatch(know); m == nil || m[1] != want {
					t.Errorf("legend %s = %v, want %s", st, m, want)
				}
			}
			// The bar's own description lists the same counts, in the order of the states.
			var parts []string
			for _, st := range learner.States {
				parts = append(parts, strconv.Itoa(ss.Counts[st])+" "+strings.ToLower(stateWord(st)))
			}
			if !strings.Contains(know, "Words by state for "+skillName(skill)+": "+strings.Join(parts, ", ")) {
				t.Errorf("the bar's description does not carry Summary's counts: %v", parts)
			}

			// The two line charts are the snapshots: 90 days, each one's number, in order.
			est := tableRows(section(t, body, "c-card pchart", `data-chart="est"`))
			ret := tableRows(section(t, body, "c-card pchart", `data-chart="ret"`))
			// 90 snapshot days plus today's live point (ADR-0028 D2).
			if len(est) != 91 {
				t.Fatalf("%d known rows, want 91 (90 history + today)", len(est))
			}
			if len(ret) != 90 && len(ret) != 91 {
				t.Fatalf("%d retention rows, want 90 or 91", len(ret))
			}
			for i := 0; i < 90; i++ {
				if want := shortDay(progDay(i)); est[i][0] != want {
					t.Fatalf("est row %d is day %q, want %q", i, est[i][0], want)
				}
				if want := shortDay(progDay(i)); ret[i][0] != want {
					t.Fatalf("ret row %d is day %q, want %q", i, ret[i][0], want)
				}
				k, solid := progKnown(i, skill)
				if want := strconv.Itoa(k + solid); est[i][1] != want {
					t.Fatalf("known on %s = %s, want %s", progDay(i), est[i][1], want)
				}
				if want := percent(progRet(i, skill)); ret[i][1] != want {
					t.Fatalf("retention on %s = %s, want %s", progDay(i), ret[i][1], want)
				}
			}
			if est[90][0] != shortDay("2026-10-18") {
				t.Fatalf("last est day %q, want today's Oct 18", est[90][0])
			}
			live := ss.Counts[learner.Known] + ss.Counts[learner.Solid]
			if est[90][1] != strconv.Itoa(live) {
				t.Fatalf("today's known = %s, want %d", est[90][1], live)
			}
			if !strings.Contains(section(t, body, "c-card pchart", `data-chart="est"`), "Through today") {
				t.Error("skill chart missing Through today")
			}
			_ = si
		})
	}
}

func skillName(skill string) string {
	switch skill {
	case "listen":
		return "Listening"
	case "read":
		return "Reading"
	case "recall":
		return "Recall"
	}
	return "Spelling"
}

func stateWord(st learner.State) string {
	switch st {
	case learner.Unseen:
		return "Not seen yet"
	case learner.Regressed:
		return "Slipping"
	}
	return strings.ToUpper(string(st)[:1]) + string(st)[1:]
}

func TestProgressUnknownSkillShowsTheFirstTab(t *testing.T) {
	_, _, _, _, get := progressApp(t)
	// No ?skill= opens the overview (INIT-023/SPEC-004). An unknown skill still falls back to listen.
	for _, q := range []string{"/progress", "/progress?skill="} {
		body := get(q)
		if !strings.Contains(body, `data-skill=""`) || !strings.Contains(body, `data-cando="1"`) {
			t.Errorf("%s did not open the overview", q)
		}
		if strings.Contains(body, `aria-pressed="true"`) {
			t.Errorf("%s pressed a skill tab on the overview", q)
		}
	}
	for _, q := range []string{"/progress?skill=bogus", "/progress?skill=LISTEN"} {
		if body := get(q); !strings.Contains(body, `data-skill="listen"`) {
			t.Errorf("%s did not fall back to listening", q)
		}
	}
	// The tabs are four c-answer buttons in a GET form; the chosen one is pressed.
	body := get("/progress?skill=recall")
	form := regexp.MustCompile(`(?s)<form class="ptabs".*?</form>`).FindString(body)
	if !strings.Contains(form, `method="get"`) || strings.Count(form, `class="c-answer"`) != 4 || strings.Count(form, `type="submit"`) != 4 {
		t.Fatalf("the skill choice is not four c-answer submit buttons in a GET form:\n%s", form)
	}
	if strings.Count(form, `aria-pressed="true"`) != 1 || !regexp.MustCompile(`value="recall" class="c-answer" data-state="selected" aria-pressed="true"`).MatchString(form) {
		t.Errorf("exactly the chosen tab should be pressed:\n%s", form)
	}
}

// ---- ac-2: every chart has a table with the same values ----------------------------------

func TestEveryChartHasATableWithTheSameValues(t *testing.T) {
	store, l, _, clk, get := progressApp(t)
	sum := summaryOf(t, store, clk, l.user.ID)
	pathPoints := regexp.MustCompile(`<path class="pc__line[^"]*"[^>]*d="([^"]*)"`)
	label := regexp.MustCompile(`<svg class="pc pc--line"[^>]*aria-label="([^"]*)"`)

	for _, skill := range learner.Skills {
		body := get("/progress?skill=" + skill)

		// Every svg is in a card that also holds exactly one table, in a <details>.
		for _, c := range []struct{ class, attr string }{
			{"c-card pknow", ""}, {"c-card pchart", `data-chart="est"`}, {"c-card pchart", `data-chart="ret"`},
		} {
			card := section(t, body, c.class, c.attr)
			if strings.Count(card, `<svg class="pc `) != 1 {
				t.Fatalf("%s %s: want one chart", skill, c.attr)
			}
			if strings.Count(card, `<details class="pc-table">`) != 1 || strings.Count(card, `<table>`) != 1 {
				t.Fatalf("%s %s: the chart has no table fallback in a <details>", skill, c.attr)
			}
			if !strings.Contains(card, `<summary>`) || !strings.Contains(card, `<caption>`) || !strings.Contains(card, `scope="col"`) || !strings.Contains(card, `scope="row"`) {
				t.Errorf("%s %s: the table has no summary, caption, or header scopes", skill, c.attr)
			}
			// A table that scrolls is a labelled region the keyboard can reach.
			if !strings.Contains(card, `role="region" tabindex="0" aria-label="`) {
				t.Errorf("%s %s: the table's scroll region is not keyboard reachable", skill, c.attr)
			}
		}

		// The bar's table is Summary's counts and shares, a row per state.
		ss := skillSummary(sum, skill)
		total := 0
		for _, st := range learner.States {
			total += ss.Counts[st]
		}
		rows := tableRows(section(t, body, "c-card pknow", ""))
		if len(rows) != len(learner.States) {
			t.Fatalf("%s: %d bar rows, want %d", skill, len(rows), len(learner.States))
		}
		for i, st := range learner.States {
			share := percent(float64(ss.Counts[st]) / float64(total))
			if rows[i][1] != strconv.Itoa(ss.Counts[st]) || rows[i][2] != share {
				t.Errorf("%s bar row %s = %v, want %d and %s", skill, st, rows[i], ss.Counts[st], share)
			}
		}

		// A line chart has a path and a table; the accessible label carries the ends.
		for _, id := range []string{"est", "ret"} {
			card := section(t, body, "c-card pchart", `data-chart="`+id+`"`)
			rows := tableRows(card)
			if pathPoints.FindStringSubmatch(card) == nil && len(rows) > 1 {
				t.Fatalf("%s %s: no line for %d table rows", skill, id, len(rows))
			}
			m := label.FindStringSubmatch(card)
			if len(rows) > 0 {
				if m == nil || !strings.Contains(m[1], rows[0][1]+" on ") || !strings.Contains(m[1], rows[len(rows)-1][1]+" on ") {
					t.Errorf("%s %s: the chart's label %v does not carry the table's first (%s) and last (%s) values", skill, id, m, rows[0][1], rows[len(rows)-1][1])
				}
			}
		}
	}
}

// ---- ac-3: chart colours come from tokens; green and red are not in the state bars ---------

func TestChartColoursComeOnlyFromTokens(t *testing.T) {
	_, _, _, _, get := progressApp(t)
	css := readStatic(t, "static/css/progress.css")

	// The markup names no colour: no literal, no inline style, and no fill or stroke attribute
	// except the hatch pattern's own reference.
	literal := regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b|\b(?:rgba?|hsla?|oklch|oklab)\s*\(|\bstyle=`)
	attr := regexp.MustCompile(`\b(?:fill|stroke)="([^"]*)"`)
	for _, skill := range learner.Skills {
		body := get("/progress?skill=" + skill)
		for _, svg := range regexp.MustCompile(`(?s)<svg class="pc .*?</svg>`).FindAllString(body, -1) {
			if m := literal.FindString(svg); m != "" {
				t.Errorf("%s: a chart names a colour or a style: %q", skill, m)
			}
			for _, a := range attr.FindAllStringSubmatch(svg, -1) {
				if !strings.HasPrefix(a[1], "url(#pb-h") {
					t.Errorf("%s: a chart sets %s directly", skill, a[0])
				}
			}
		}
	}

	// The stylesheet passes the colour lint (no literal, no green or red outside feedback) ...
	if res := lintColors("progress.css", css, nil); len(res.Violations) != 0 {
		t.Errorf("progress.css:\n%s", strings.Join(res.Violations, "\n"))
	}
	// ... and the chart classes do not name green or red at all.
	chartRules := 0
	for _, r := range parseCSSRules(css) {
		if !strings.Contains(r.Selector, ".pc") && !strings.Contains(r.Selector, ".pbar") {
			continue
		}
		chartRules++
		if regexp.MustCompile(`--(?:go|stop)`).MatchString(r.Body) {
			t.Errorf("%s uses green or red (%s)", r.Selector, strings.TrimSpace(r.Body))
		}
		// Every colour value is a role from tokens.css or a mix of roles.
		for _, m := range regexp.MustCompile(`(?:^|;)\s*(?:fill|stroke|background|background-color|border|color)\s*:\s*([^;]+)`).FindAllStringSubmatch(r.Body, -1) {
			v := strings.TrimSpace(m[1])
			if v != "none" && !strings.Contains(v, "var(--") && !strings.HasPrefix(v, "repeating-linear-gradient") && !strings.HasPrefix(v, "1px solid") && !strings.HasPrefix(v, "transparent") {
				t.Errorf("%s: %q is not a token", r.Selector, v)
			}
		}
	}
	if chartRules < 20 {
		t.Fatalf("only %d chart rules found; the test is not looking at the chart CSS", chartRules)
	}

	// Each state has its own fill, so the bar tells seven states apart, and solid is the lamp.
	fills := map[string]string{}
	for _, r := range parseCSSRules(css) {
		m := regexp.MustCompile(`^\.pc__seg\[data-state=([a-z]+)\]$`).FindStringSubmatch(r.Selector)
		if m == nil {
			continue
		}
		f := regexp.MustCompile(`fill\s*:\s*([^;]+)`).FindStringSubmatch(r.Body)
		if f == nil {
			t.Fatalf("state %s has no fill", m[1])
		}
		fills[m[1]] = strings.TrimSpace(f[1])
	}
	seen := map[string]string{}
	for _, st := range learner.States {
		f, ok := fills[string(st)]
		if !ok {
			t.Errorf("no .pc__seg rule for %s", st)
			continue
		}
		if other, dup := seen[f]; dup {
			t.Errorf("states %s and %s share the fill %s", st, other, f)
		}
		seen[f] = string(st)
	}
	if fills["solid"] != "var(--lamp)" {
		t.Errorf("solid is %q, want the lamp", fills["solid"])
	}
	// The struggling state carries a hatch in the markup (the fixture has a struggling word), and
	// both hatch patterns are defined. The slipping hatch is covered in chart_test.go.
	body := get("/progress?skill=listen")
	bar := regexp.MustCompile(`(?s)<svg class="pc pc--bar".*?</svg>`).FindString(body)
	if !strings.Contains(bar, `<pattern id="pb-ha"`) || !strings.Contains(bar, `<pattern id="pb-hb"`) ||
		!regexp.MustCompile(`data-state="struggling"[^>]*/><rect class="pc__seg-hatch"[^>]*fill="url\(#pb-ha\)"`).MatchString(bar) {
		t.Errorf("struggling has no hatch:\n%s", bar)
	}
}

// ---- ac-4: no snapshots is an empty state, with no broken chart ---------------------------

var brokenChart = regexp.MustCompile(`NaN|Inf\b|d=""|="undefined"|<svg class="pc [^>]*viewBox="0 0 0`)

func TestNoSnapshotsShowsTheEmptyStateAndNoChart(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", progNow)
	body := getLearner(t, h, l, "/progress?detail=1")
	for _, want := range []string{`data-empty="true"`, "Practice a few days and your chart starts here", `class="quiet-card progress-empty"`, `href="/"`, "empty-progress.svg"} {
		if !strings.Contains(body, want) {
			t.Errorf("the empty page has no %q", want)
		}
	}
	for _, bad := range []string{`<svg class="pc `, `data-chart=`, `class="ptabs"`, "pweek", "pplace"} {
		if strings.Contains(body, bad) {
			t.Errorf("the empty page still has %q", bad)
		}
	}
	if m := brokenChart.FindString(body); m != "" {
		t.Errorf("broken chart text %q", m)
	}
}

func TestEvidenceWithoutSnapshotsShowsTheBarAndAHintInPlaceOfEachLine(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", progNow)
	words := seedLexicon(t, store, progNow, 4, false, true)
	strugglingWord(t, store, clk, l.user.ID, words[0].id, "listen", 4, 1)
	body := getLearner(t, h, l, "/progress?skill=listen")
	if !strings.Contains(body, `data-empty="false"`) || strings.Count(body, `<svg class="pc pc--bar"`) != 1 {
		t.Fatal("a learner with evidence should see the state bar")
	}
	if strings.Contains(body, `<svg class="pc pc--line"`) || strings.Count(body, "Practice a few days and your chart starts here.") != 1 {
		t.Error("with no snapshots the estimate chart should be its hint, and there is no line")
	}
	if strings.Contains(body, "pc__line") {
		t.Error("a line without snapshots")
	}
	if !strings.Contains(body, "Retention starts once you have due reviews to answer.") {
		t.Error("the retention card does not say what to do")
	}
	if m := brokenChart.FindString(body); m != "" {
		t.Errorf("broken chart text %q", m)
	}
}

func TestOneSnapshotPlusTodayDrawsALine(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", progNow)
	seedLexicon(t, store, progNow, 3, false, true)
	seedProgressSnapshots(t, store, l.user.ID, 1, progEst)
	body := getLearner(t, h, l, "/progress?skill=listen")
	est := section(t, body, "c-card pchart", `data-chart="est"`)
	// One snapshot day plus today's live point → a two-point line (ADR-0028 D2).
	if !strings.Contains(est, `<circle`) || !strings.Contains(est, "pc__line") {
		t.Errorf("snapshot + today should draw a line:\n%s", est)
	}
	if rows := tableRows(est); len(rows) != 2 {
		t.Errorf("%d rows, want 2 (yesterday + today)", len(rows))
	}
	if !strings.Contains(est, "Through today") {
		t.Error("chart missing Through today")
	}
	if m := brokenChart.FindString(body); m != "" {
		t.Errorf("broken chart text %q", m)
	}
}

// ---- the weekly report, This week, and the placement history ------------------------------

func TestWeeklyReportCardShowsTheNewestRowEscaped(t *testing.T) {
	_, _, _, _, get := progressApp(t)
	body := get("/progress?detail=1")
	card := section(t, body, "c-card pweekly", "")
	if !strings.Contains(card, "Week of Oct 12") || !strings.Contains(card, "You gained 14 words &amp; lost &lt;3&gt;.") {
		t.Errorf("weekly card:\n%s", card)
	}
	if strings.Contains(body, "Older report.") {
		t.Error("an older report is on the page")
	}
	if strings.Index(body, "pweekly") > strings.Index(body, `class="ptabs"`) {
		t.Error("the report is not above the skill tabs")
	}
}

func TestWeeklyReportCardIsAbsentWithoutARow(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", progNow)
	seedLexicon(t, store, progNow, 3, false, true)
	seedProgressSnapshots(t, store, l.user.ID, 90, progEst)
	if strings.Contains(getLearner(t, h, l, "/progress?detail=1"), "pweekly") {
		t.Error("a report card with no report")
	}
}

func TestThisWeekAgreesWithGetProgress(t *testing.T) {
	store, l, words, clk, get := progressApp(t)
	body := get("/progress?detail=1")
	week := section(t, body, "c-card pweek", "")

	if !strings.Contains(week, "to today") {
		t.Fatalf("This week should end on today:\n%s", week)
	}
	sum := summaryOf(t, store, clk, l.user.ID)
	// Week table ends on today's live known+solid (ADR-0028 D2).
	for _, ss := range sum.Skills {
		re := regexp.MustCompile(`(?s)<tr data-skill="` + ss.Skill + `">.*?</tr>`)
		row := tableRows(re.FindString(week))
		if len(row) != 1 {
			t.Fatalf("no row for %s", ss.Skill)
		}
	}

	from := addDaysStr(progLast, -weekDays)
	want, err := mcpstore.New(store).Progress(context.Background(), l.user.ID, from, progLast)
	if err != nil {
		t.Fatal(err)
	}
	// The slips are get_progress's regression events, the words with their cause.
	slipped := regexp.MustCompile(`(?s)<ul class="pweek__list" data-list="slipped">.*?</ul>`).FindString(week)
	wantWords := 0
	for _, e := range want.Regressions {
		if e.Kind != "word" {
			continue
		}
		wantWords++
		if !strings.Contains(slipped, e.Text) || !strings.Contains(slipped, `data-skill="`+e.Skill+`"`) {
			t.Errorf("the slip of %s is not listed", e.Text)
		}
	}
	if got := strings.Count(slipped, `class="pweek__he"`); got != wantWords || wantWords != 2 {
		t.Errorf("%d slipped words listed, get_progress has %d (want 2)", got, wantWords)
	}
	for _, s := range []string{"mixed up with another word", "no clear cause yet", "Some level 2 words are slipping"} {
		if !strings.Contains(slipped, s) {
			t.Errorf("slipped list lacks %q:\n%s", s, slipped)
		}
	}
	if strings.Contains(slipped, words[2].he) {
		t.Error("a slip from two weeks ago is in this week's list")
	}
	if strings.Contains(slipped, words[5].he) {
		t.Error("another learner's slip is in the list")
	}

	back := regexp.MustCompile(`(?s)<ul class="pweek__list" data-list="back">.*?</ul>`).FindString(week)
	if !strings.Contains(back, words[2].he) || strings.Contains(back, words[0].he) {
		t.Errorf("back on track should hold %s and not %s:\n%s", words[2].he, words[0].he, back)
	}
	if !strings.Contains(back, "hears it, cannot read it yet") {
		t.Error("a recovered word does not carry its cause")
	}
}

func progSigned(n int, sign string) string {
	if n <= 0 {
		return "0"
	}
	return sign + strconv.Itoa(n)
}

func TestPlacementHistoryListsEachRunWithItsLevels(t *testing.T) {
	_, _, _, _, get := progressApp(t)
	body := get("/progress?detail=1")
	card := section(t, body, "c-card pplace", "")
	items := regexp.MustCompile(`(?s)<li class="pplace__item">.*?</li>`).FindAllString(card, -1)
	if len(items) != 2 {
		t.Fatalf("%d placement rows, want 2:\n%s", len(items), card)
	}
	check, first := items[0], items[1] // newest first
	for _, w := range []string{"Sep 4", "Oct 2", "Check-in", "First placement"} {
		if !strings.Contains(card, w) {
			t.Errorf("no %q", w)
		}
	}
	if !strings.Contains(check, "Oct 2") || !strings.Contains(check, "Check-in") || !strings.Contains(check, "Listening level 3, about 164 words") || !strings.Contains(check, "Reading not tested") {
		t.Errorf("check-in row:\n%s", check)
	}
	if strings.Contains(check, "Reading level") {
		t.Errorf("check-in should not claim a reading level:\n%s", check)
	}
	if !strings.Contains(first, "Sep 4") || !strings.Contains(first, "First placement") || !strings.Contains(first, "Listening level 3, about 142 words") || !strings.Contains(first, "Reading level 2, about 90 words") {
		t.Errorf("first placement row:\n%s", first)
	}
	if got := placementTitle(t, check); got != "Oct 2 · Check-in" {
		t.Errorf("check-in title %q", got)
	}
	if got := placementTitle(t, first); got != "Sep 4 · First placement" {
		t.Errorf("first placement title %q", got)
	}
}

func TestPlacementHistoryEmptyStateSaysWhatToDo(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", progNow)
	seedLexicon(t, store, progNow, 3, false, true)
	seedProgressSnapshots(t, store, l.user.ID, 90, progEst)
	card := section(t, getLearner(t, h, l, "/progress?detail=1"), "c-card pplace", "")
	if !strings.Contains(card, "No placement yet. Take the placement check from Today") || !strings.Contains(card, `href="/"`) {
		t.Errorf("placement empty state:\n%s", card)
	}
}

// ---- security: only the signed-in learner's data ------------------------------------------

func TestProgressShowsOnlyTheSignedInLearnersData(t *testing.T) {
	store, l, words, _, get := progressApp(t)
	// The other learner (id 1) has estimates from 777 up, and a slip of words[5].
	_ = store
	otherID := l.user.ID - 1 // progressApp creates the other learner just before this one
	if otherID <= 0 {
		t.Fatal("fixture: the other learner is not the one before")
	}
	for _, q := range []string{
		"/progress",
		"/progress?skill=read",
		fmt.Sprintf("/progress?user_id=%d&learner=%d&user=%d&id=%d", otherID, otherID, otherID, otherID),
	} {
		body := get(q)
		for i := 0; i < 90; i++ {
			if strings.Contains(body, fmt.Sprintf(">%d<", 777+i)) {
				t.Fatalf("%s shows the other learner's estimate %d", q, 777+i)
			}
		}
		if strings.Contains(body, words[5].he) {
			t.Fatalf("%s shows the other learner's slip", q)
		}
		if !strings.Contains(q, "skill=") {
			if !strings.Contains(body, `data-cando="1"`) {
				t.Fatalf("%s is not the can-do screen", q)
			}
			continue
		}
		rows := tableRows(section(t, body, "c-card pchart", `data-chart="est"`))
		if len(rows) < 1 {
			t.Fatalf("%s: no chart rows", q)
		}
		// Other learner's seeded estimates were 777+; none of those may appear.
		for _, r := range rows {
			if len(r) > 1 {
				n, _ := strconv.Atoi(r[1])
				if n >= 777 && n < 777+90 {
					t.Fatalf("%s shows the other learner's estimate %d", q, n)
				}
			}
		}
	}
}

// ---- the Hebrew shell and the empty shell --------------------------------------------------

func TestProgressInTheHebrewShellKeepsEachRunInItsOwnLanguage(t *testing.T) {
	_, _, words, _, get := progressApp(t)
	body := get("/progress?lang=he")
	if !strings.Contains(body, `<html lang="he" dir="rtl"`) {
		t.Fatal("the Hebrew shell is not rtl")
	}
	if !strings.Contains(body, `<bdi lang="he" dir="rtl">אַתָּה`) {
		t.Error("the Hebrew can-do sentence is not marked Hebrew")
	}
	if !strings.Contains(body, "אַתָּה") {
		t.Error("the Hebrew can-do sentence is missing")
	}
	// The charts stay left to right on a skill page. The overview has none.
	skillBody := get("/progress?skill=listen&lang=he")
	week := section(t, skillBody, "c-card pweek", "")
	if !strings.Contains(week, `<bdi lang="he" dir="rtl">`+words[0].he+`</bdi>`) {
		t.Error("a Hebrew word is not marked lang=he dir=rtl")
	}
	if strings.Count(skillBody, `class="pbar" dir="ltr"`) != 1 || strings.Count(skillBody, `class="pchart__plot" dir="ltr"`) != 2 {
		t.Error("a chart is not dir=ltr")
	}
	if strings.Count(body, `class="pchart__plot"`) != 0 {
		t.Error("the overview still has a chart")
	}
	if !strings.Contains(skillBody, `<input type="hidden" name="lang" value="he">`) {
		t.Error("the skill form drops the Hebrew shell")
	}
	english := get("/progress?lang=en")
	if n := assertRunDir(t, english, "he", "rtl"); n == 0 {
		t.Error("the English shell has no Hebrew run to check")
	}
}

// ---- the page keeps what the old one had: the lessons finished ------------------------------

func TestProgressNamesTheLessonsFinished(t *testing.T) {
	store, l, _, _, get := progressApp(t)
	u1 := shellUnit(t, store, "One", 1, "shared", 2)
	shellUnit(t, store, "Two", 2, "shared", 2)
	finished := progNow
	if _, err := testharness.InsertLessonRun(context.Background(), store, l.user.ID, u1, 0, progNow.Add(-time.Hour), &finished); err != nil {
		t.Fatal(err)
	}
	body := get("/progress")
	if strings.Contains(body, "1/2") || strings.Contains(body, "Units finished") || strings.Contains(body, "Lessons finished") {
		t.Error("progress showed a finished count")
	}
	if !strings.Contains(body, `data-cando="1"`) || !strings.Contains(body, `progress__next`) {
		t.Error("the can-do sentence or the next action is missing")
	}
}

// progressUnit is one unit in the finished-count fixture. lessons and finished
// are the lesson rows this test inserts. count is whether that unit belongs
// in the signed-in learner's totals. The expected fractions are summed from
// these rows, not from the handler.
type progressUnit struct {
	lessons  int
	finished int
	count    bool
}

func TestProgressCountsLearnerUnitsAndWholeUnits(t *testing.T) {
	store, learner1, _, _, get := progressApp(t)
	learner2 := addLearner(t, store, "b@example.com", "sub-b", progNow)
	scope1 := "learner:" + itoa(learner1.user.ID)
	scope2 := "learner:" + itoa(learner2.user.ID)
	rows := []progressUnit{
		{lessons: 2, finished: 2, count: true},  // shared, every lesson finished
		{lessons: 3, finished: 1, count: true},  // shared, a lesson still open
		{lessons: 2, finished: 2, count: true},  // this learner's unit, finished
		{lessons: 0, finished: 0, count: true},  // published shared, no lessons: counted, not finished
		{lessons: 1, finished: 1, count: false}, // the other learner's unit
		{lessons: 1, finished: 1, count: false}, // unpublished
	}
	scopes := []string{"shared", "shared", scope1, "shared", scope2, "shared"}
	publishRow := []bool{true, true, true, true, true, false}
	for i, row := range rows {
		lessons := make([]units.Lesson, row.lessons)
		for n := range lessons {
			lessons[n] = units.Lesson{Kind: units.KindLesson, Activities: []units.Activity{{Type: units.ActivityStory, Text: "OK"}}}
		}
		id := putLesson(t, store, nil, units.Unit{
			Title: "U", Position: 10 + i, Lessons: lessons,
		}, scopes[i])
		if publishRow[i] {
			publish(t, store, id)
		}
		for n := 0; n < row.finished; n++ {
			at := progNow.Add(-time.Duration(n+1) * time.Hour)
			if _, err := testharness.InsertLessonRun(context.Background(), store, learner1.user.ID, id, n, at, &at); err != nil {
				t.Fatal(err)
			}
		}
		// A run at index == lesson count is a lesson the unit no longer has.
		// It must not raise the finished-lesson count or finish the unit.
		if i == 0 {
			at := progNow
			if _, err := testharness.InsertLessonRun(context.Background(), store, learner1.user.ID, id, row.lessons, at, &at); err != nil {
				t.Fatal(err)
			}
		}
	}
	unitsDone, unitsTotal, lessonsDone, lessonsTotal := 0, 0, 0, 0
	for _, row := range rows {
		if !row.count {
			continue
		}
		unitsTotal++
		lessonsTotal += row.lessons
		lessonsDone += row.finished
		if row.lessons > 0 && row.finished == row.lessons {
			unitsDone++
		}
	}
	body := get("/progress")
	unitsText := strconv.Itoa(unitsDone) + "/" + strconv.Itoa(unitsTotal)
	lessonsText := strconv.Itoa(lessonsDone) + "/" + strconv.Itoa(lessonsTotal)
	if unitsText == lessonsText {
		t.Fatalf("fixture units %s and lessons %s match, so a swapped label would pass", unitsText, lessonsText)
	}
	if strings.Contains(body, "Units finished") || strings.Contains(body, "Lessons finished") || strings.Contains(body, ">"+unitsText+"<") {
		t.Fatalf("progress showed a count, want the can-do sentence:\n%s", body)
	}
	if !strings.Contains(body, `data-cando="1"`) || !strings.Contains(body, `progress__next`) {
		t.Fatalf("missing the sentence or the next action:\n%s", body)
	}
	_ = lessonsText
}

func TestProgressHidesLessonsWhenNoneExist(t *testing.T) {
	store, _, _, _, get := progressApp(t)
	id := putLesson(t, store, nil, units.Unit{
		Title: "Empty", Position: 1, Lessons: nil,
	}, "shared")
	publish(t, store, id)
	body := get("/progress")
	if strings.Contains(body, "Lessons finished") || strings.Contains(body, "Units finished") || strings.Contains(body, "0/1") {
		t.Fatalf("progress showed a count:\n%s", body)
	}
	if !strings.Contains(body, `data-cando="1"`) {
		t.Fatal("the can-do sentence is missing")
	}
}

func TestPlacementUnfinishedLevelSaysNotFinished(t *testing.T) {
	store, l, _, _, get := progressApp(t)
	// Dates and levels are the fixture. Oct 19 is the unfinished run.
	unfinishedAt := time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)
	levelAt := time.Date(2026, 9, 3, 15, 0, 0, 0, time.UTC)
	zeroAt := time.Date(2026, 8, 2, 15, 0, 0, 0, time.UTC)
	const normalLevel = 4
	insertPlacementLevel(t, store, l.user.ID, unfinishedAt, -1)
	insertPlacementLevel(t, store, l.user.ID, levelAt, normalLevel)
	insertPlacementLevel(t, store, l.user.ID, zeroAt, 0)
	card := section(t, get("/progress?detail=1"), "c-card pplace", "")
	items := regexp.MustCompile(`(?s)<li class="pplace__item">.*?</li>`).FindAllString(card, -1)
	row := func(date string) string {
		t.Helper()
		for _, item := range items {
			if strings.Contains(item, date) {
				return item
			}
		}
		t.Fatalf("no placement row %s:\n%s", date, card)
		return ""
	}
	unfinished := row("Oct 19")
	if got := placementTitle(t, unfinished); got != "Oct 19 · Placement · Not finished" {
		t.Fatalf("unfinished placement title %q", got)
	}
	normal := row("Sep 3")
	if got := placementTitle(t, normal); got != "Sep 3 · Placement · Level "+strconv.Itoa(normalLevel) {
		t.Fatalf("finished placement title %q", got)
	}
	zero := row("Aug 2")
	if got := placementTitle(t, zero); got != "Aug 2 · Placement · Level 0" {
		t.Fatalf("level 0 placement title %q", got)
	}
}

// placementTitle is the visible title of one history row, with the tags removed.
func placementTitle(t *testing.T, item string) string {
	t.Helper()
	const open = `<p class="pplace__title">`
	i := strings.Index(item, open)
	if i < 0 {
		t.Fatalf("no placement title:\n%s", item)
	}
	rest := item[i+len(open):]
	j := strings.Index(rest, "</p>")
	if j < 0 {
		t.Fatalf("placement title is not closed:\n%s", item)
	}
	inner := rest[:j]
	var b strings.Builder
	for {
		start := strings.IndexByte(inner, '<')
		if start < 0 {
			b.WriteString(inner)
			break
		}
		b.WriteString(inner[:start])
		end := strings.IndexByte(inner[start:], '>')
		if end < 0 {
			t.Fatalf("unclosed tag in placement title:\n%s", item)
		}
		inner = inner[start+end+1:]
	}
	return b.String()
}

func insertPlacementLevel(t *testing.T, store *db.Store, userID int64, at time.Time, level int) {
	t.Helper()
	run, err := store.InsertPlacementRun(context.Background(), userID, at, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.FinishPlacement(context.Background(), run, at, level, "exhausted", `{"bands":[]}`); err != nil {
		t.Fatal(err)
	}
}

// ---- INIT-023/SPEC-004: Progress opens on every skill at once --------------------

// overviewLexemes adds n pointed words the overview fixture can hang evidence on.
func overviewLexemes(t *testing.T, store *db.Store, n int) []int64 {
	t.Helper()
	ids := make([]int64, n)
	for i := range ids {
		ids[i] = addLexeme(t, store, fmt.Sprintf("מִלָּה%d", i+1), fmt.Sprintf("word-%d", i+1), fmt.Sprintf("mila%d", i+1), "m", "")
	}
	return ids
}

// seedKnownSkill makes a word known in one skill: three right answers, a card
// reviewed yesterday with three Good reviews, and enough stability to stay known.
func seedKnownSkill(t *testing.T, store *db.Store, clk clock.Clock, userID, lexemeID int64, skill string) {
	t.Helper()
	now := clk.Now()
	rec := evidence.New(store, fsrs.New(store, clk), clk)
	for i := 0; i < 3; i++ {
		_, err := rec.Attempt(context.Background(), evidence.Attempt{
			UserID: userID, LexemeID: lexemeID, Skill: skill, Activity: "test",
			Source: evidence.SourceReview, Prompt: "p", Response: "r", Correct: true, SkipSchedule: true,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	last := now.AddDate(0, 0, -1)
	id, err := store.InsertCard(context.Background(), db.Card{
		UserID: userID, LexemeID: lexemeID, Skill: skill, State: fsrs.StateReview,
		Due: now, Stability: 10, Difficulty: 5, Reps: 3, LastReview: last,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		at := last.Add(-time.Duration(2-i) * 24 * time.Hour)
		insertReview(t, store, id, int(fsrs.Good), at)
	}
}

// seedListenAnswerNoCard stores a listening answer without making a card, so the
// skill stays at seen (answers with no card).
func seedListenAnswerNoCard(t *testing.T, store *db.Store, clk clock.Clock, userID, lexemeID int64) {
	t.Helper()
	rec := evidence.New(store, fsrs.New(store, clk), clk)
	_, err := rec.Attempt(context.Background(), evidence.Attempt{
		UserID: userID, LexemeID: lexemeID, Skill: "listen", Activity: "listen_pick",
		Source: evidence.SourceLesson, Prompt: "p", Response: "r", Correct: true, SkipSchedule: true,
	})
	if err != nil {
		t.Fatal(err)
	}
}

// overviewApp is the learner-1 shaped fixture for SPEC-004: Read 11 known, Recall 2,
// Listen 0 with 29 answered words and no listen cards, Spell 0.
func overviewApp(t *testing.T) (*db.Store, learnerFix, clock.Clock, func(path string) string) {
	t.Helper()
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-a", progNow)
	ids := overviewLexemes(t, store, 29)
	for _, id := range ids {
		seedListenAnswerNoCard(t, store, clk, l.user.ID, id)
	}
	for i := 0; i < 11; i++ {
		seedKnownSkill(t, store, clk, l.user.ID, ids[i], "read")
	}
	for i := 0; i < 2; i++ {
		seedKnownSkill(t, store, clk, l.user.ID, ids[i], "recall")
	}
	seedProgressSnapshots(t, store, l.user.ID, 90, progEst)
	get := func(path string) string { return getLearner(t, h, l, path) }
	return store, l, clk, get
}

func overviewRow(t *testing.T, body, skill string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)<li class="pknow__skill" data-skill="` + regexp.QuoteMeta(skill) + `"[^>]*>.*?</li>`)
	row := re.FindString(body)
	if row == "" {
		t.Fatalf("no overview row for %s", skill)
	}
	return row
}

func overviewKnown(t *testing.T, row string) int {
	t.Helper()
	re := regexp.MustCompile(`data-count="known"><bdi[^>]*>([0-9]+)</bdi>`)
	m := re.FindStringSubmatch(row)
	if m == nil {
		t.Fatalf("no known count in row:\n%s", row)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func rungCount(t *testing.T, row, rung string) int {
	t.Helper()
	re := regexp.MustCompile(`data-rung="` + rung + `"[^>]*>[\s\S]*?<bdi[^>]*>([0-9]+)% \(([0-9]+)\)</bdi>`)
	m := re.FindStringSubmatch(row)
	if m == nil {
		t.Fatalf("no %s count in row:\n%s", rung, row)
	}
	n, err := strconv.Atoi(m[2])
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func rungSum(t *testing.T, row string) int {
	t.Helper()
	n := 0
	for _, rung := range []string{"proved", "recalled_later", "slipping", "not_yet_asked"} {
		n += rungCount(t, row, rung)
	}
	return n
}

// The overview leads with the ladder. Each row's four counts sum to taught.
func TestProgressOverviewShowsKnownPerSkill(t *testing.T) {
	store, l, clk, get := overviewApp(t)
	sum := summaryOf(t, store, clk, l.user.ID)
	body := get("/progress")
	if !strings.Contains(body, `data-skill=""`) || !strings.Contains(body, `data-cando="1"`) {
		t.Fatal(" /progress did not open the overview")
	}
	if strings.Contains(body, "%") || strings.Contains(body, "pknow__skills") || strings.Contains(body, "Units finished") {
		t.Fatal("the overview still shows a count or a chart")
	}
	listen := skillSummary(sum, "listen").Ladder.Proved > 0
	recall := skillSummary(sum, "recall").Ladder.Proved > 0
	read := skillSummary(sum, "read").Ladder.Proved > 0
	spell := skillSummary(sum, "spell").Ladder.Proved > 0
	en, _ := views.CanDoSentence(listen, recall, read, spell)
	if !strings.Contains(body, en) {
		t.Fatalf("sentence %q missing:\n%s", en, body)
	}
}

// Listen with no audio still shows the ladder counts, and says there is no audio.
func TestProgressOverviewListenNoCardExplanation(t *testing.T) {
	_, _, _, get := overviewApp(t)
	body := get("/progress")
	if strings.Contains(body, "No audio yet") || strings.Contains(body, "0% (0)") || strings.Contains(body, "understand words by ear") {
		t.Fatalf("the overview claimed listening or showed a count:\n%s", body)
	}
	if !strings.Contains(body, `data-cando="1"`) {
		t.Fatal("the can-do sentence is missing")
	}
}

// ac-3: /progress?skill=read still renders the Read tab as today.
func TestProgressSkillTabStillShowsSingleSkill(t *testing.T) {
	store, l, clk, get := overviewApp(t)
	sum := summaryOf(t, store, clk, l.user.ID)
	ss := skillSummary(sum, "read")
	body := get("/progress?skill=read")
	if !strings.Contains(body, `data-skill="read"`) {
		t.Fatal("skill=read did not open the Read tab")
	}
	if strings.Contains(body, `class="pknow__skills"`) {
		t.Fatal("skill=read still shows the overview")
	}
	if !strings.Contains(body, "What you know · Reading") {
		t.Fatal("the Read tab heading is missing")
	}
	if got := strings.Count(body, `<svg class="pc pc--bar"`); got != 1 {
		t.Fatalf("%d state bars, want 1", got)
	}
	headline := regexp.MustCompile(`class="count"><bdi lang="en" dir="ltr">([0-9]+)</bdi>`)
	if m := headline.FindStringSubmatch(body); m == nil || m[1] != strconv.Itoa(ss.Ladder.Proved) {
		t.Errorf("headline %v, want proved %d", m, ss.Ladder.Proved)
	}
	if strings.Contains(body, "words estimated known") || strings.Contains(body, "Placement estimate:") || strings.Contains(body, "From placement") {
		t.Fatal("the skill tab still shows a placement estimate")
	}
	form := regexp.MustCompile(`(?s)<form class="ptabs".*?</form>`).FindString(body)
	if !regexp.MustCompile(`value="read" class="c-answer" data-state="selected" aria-pressed="true"`).MatchString(form) {
		t.Errorf("Read tab is not pressed:\n%s", form)
	}
}

// ac-4: overview numbers match get_learner_model (Summary) counts for the same fixture.
func TestProgressOverviewMatchesLearnerModel(t *testing.T) {
	store, l, clk, get := overviewApp(t)
	sum := summaryOf(t, store, clk, l.user.ID)
	body := get("/progress")
	if strings.Contains(body, "Words you know") || strings.Contains(body, "%") || strings.Contains(body, "pchart") {
		t.Error("overview still leads with a count or a chart")
	}
	en, _ := views.CanDoSentence(
		skillSummary(sum, "listen").Ladder.Proved > 0,
		skillSummary(sum, "recall").Ladder.Proved > 0,
		skillSummary(sum, "read").Ladder.Proved > 0,
		skillSummary(sum, "spell").Ladder.Proved > 0,
	)
	if !strings.Contains(body, en) || !strings.Contains(body, `progress__next`) {
		t.Fatalf("sentence %q missing:\n%s", en, body)
	}
}

// TestProgressDenverLearnerDayEndsOct31 is ac-1 (INIT-026/SPEC-006): at
// 2026-11-01T05:30:00Z a Denver learner's Progress chart and This week end on
// 2026-10-31.
func TestProgressDenverLearnerDayEndsOct31(t *testing.T) {
	now := time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC)
	store, h := openShellWithAppZone(t, fixedClock(now), "America/Chicago")
	l := addLearner(t, store, "a@example.com", "sub-denver-progress", now)
	setLearnerZone(t, store, l.user.ID, "America/Denver")
	seedLearnerDaySnapshots(t, store, l.user.ID, "2026-10-25", "2026-11-01")

	body := getLearner(t, h, l, "/progress?skill=listen")
	est := tableRows(section(t, body, "c-card pchart", `data-chart="est"`))
	if len(est) == 0 || est[len(est)-1][0] != "Oct 31" {
		t.Fatalf("chart ends on %v, want Oct 31", est)
	}
	week := section(t, body, "c-card pweek", "")
	if !strings.Contains(week, "to today") {
		t.Fatalf("This week does not end on today:\n%s", week)
	}
	if !strings.Contains(section(t, body, "c-card pchart", `data-chart="est"`), "Through today") {
		t.Fatal("chart missing Through today")
	}
}

// TestProgressNoZoneUsesAppChicago is ac-2 (INIT-026/SPEC-006): a learner with
// no zone uses the app zone. With Chicago, 2026-11-01T05:30:00Z is 2026-11-01.
func TestProgressNoZoneUsesAppChicago(t *testing.T) {
	now := time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC)
	store, h := openShellWithAppZone(t, fixedClock(now), "America/Chicago")
	l := addLearner(t, store, "a@example.com", "sub-app-progress", now)
	seedLearnerDaySnapshots(t, store, l.user.ID, "2026-10-25", "2026-11-01")

	body := getLearner(t, h, l, "/progress?skill=listen")
	est := tableRows(section(t, body, "c-card pchart", `data-chart="est"`))
	if len(est) == 0 || est[len(est)-1][0] != "Nov 1" {
		t.Fatalf("chart ends on %v, want Nov 1 (Chicago app zone)", est)
	}
	week := section(t, body, "c-card pweek", "")
	if !strings.Contains(week, "to today") {
		t.Fatalf("This week does not end on today:\n%s", week)
	}
}

// ---- INIT-032/SPEC-006: Home and Progress share KnownAnySkill ------------------

// claimProgressPlacement deals and answers one placement item (web test helper).
func claimProgressPlacement(t *testing.T, store *db.Store, runID int64, seq int, part, skill string, band int, itemID, outcome string, lexemeID int64, at time.Time) {
	t.Helper()
	ctx := context.Background()
	it := db.PlacementItem{
		RunID: runID, Seq: seq, Part: part, Skill: skill, Band: band, ItemID: itemID,
		Options: `{"options":[]}`, ShownAt: at,
	}
	if err := store.InsertPlacementItem(ctx, it); err != nil {
		t.Fatal(err)
	}
	ok, err := store.ClaimPlacementItem(ctx, runID, seq, outcome, "", lexemeID, at, 0)
	if err != nil || !ok {
		t.Fatalf("claim %s: ok=%v err=%v", itemID, ok, err)
	}
}

// TestProgressKnownAnySkillHeadlineAboveZero is AC2: solid in reading and learning
// in recall still makes a headline above 0.
func TestProgressKnownAnySkillHeadlineAboveZero(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-any-skill", progNow)
	id := addLexeme(t, store, "כֶּלֶב", "dog", "kelev", "m", "")
	seedKnownSkill(t, store, clk, l.user.ID, id, "read") // solid-capable known/solid
	// Learning in recall: card younger than three successes.
	last := progNow.AddDate(0, 0, -1)
	cardID, err := store.InsertCard(context.Background(), db.Card{
		UserID: l.user.ID, LexemeID: id, Skill: "recall", State: fsrs.StateReview,
		Due: progNow, Stability: 2, Difficulty: 5, Reps: 1, LastReview: last,
	})
	if err != nil {
		t.Fatal(err)
	}
	insertReview(t, store, cardID, int(fsrs.Good), last)

	sum := summaryOf(t, store, clk, l.user.ID)
	if sum.KnownAnySkill == 0 {
		t.Fatal("KnownAnySkill is 0 for solid/known read + learning recall")
	}
	body := getLearner(t, h, l, "/progress?detail=1")
	recall := overviewRow(t, body, "recall")
	if got := rungCount(t, recall, "proved"); got != skillSummary(sum, "recall").Ladder.Proved {
		t.Fatalf("recall proved = %d, ladder %d", got, skillSummary(sum, "recall").Ladder.Proved)
	}
	if strings.Contains(body, "Words you know") {
		t.Fatal("Progress still leads with Words you know")
	}
}

// TestProgressListeningOnlyPlacementMarksEvidenceSkills is AC3: after a
// listening-only placement, read/recall/spell say Not tested by placement.
func TestProgressListeningOnlyPlacementMarksEvidenceSkills(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-listen-only", progNow)
	ctx := context.Background()
	lex, err := store.UpsertPlacementLexeme(ctx, "שָׁלוֹם", "hello", "noun", "shalom", "")
	if err != nil {
		t.Fatal(err)
	}
	runID, err := store.StartPlacementV3(ctx, l.user.ID, progNow.Add(-time.Hour), 1, db.RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	claimProgressPlacement(t, store, runID, 1, db.PartListening, "listen", 1, "w-1", "right", lex, progNow.Add(-time.Hour))
	claimProgressPlacement(t, store, runID, 2, db.PartListening, "listen", 1, "w-2", "wrong", lex, progNow.Add(-time.Hour))
	result := `{"estimated_known_words":50,"listening":{"tested":true,"level":2,"estimate":50},"reading":{"tested":false,"level":0,"estimate":0}}`
	if err := store.FinishPlacement(ctx, runID, progNow.Add(-30*time.Minute), 2, "exhausted", result); err != nil {
		t.Fatal(err)
	}
	seedProgressSnapshots(t, store, l.user.ID, 2, progEst)

	body := getLearner(t, h, l, "/progress?detail=1")
	know := section(t, body, "c-card pknow", "")
	if strings.Contains(know, "Placement estimate:") || strings.Contains(know, "From placement") {
		t.Fatalf("headline still shows a placement estimate:\n%s", know)
	}
	place := section(t, body, "c-card pplace", "")
	if !strings.Contains(place, "Listening level 2, about 50 words") || !strings.Contains(place, "Reading not tested") {
		t.Errorf("placement history:\n%s", place)
	}
}

// TestProgressSpellNotPracticedThenLearningAfterTypeIt is AC4: empty spelling
// starts building; one correct typed answer shows learning 1.
func TestProgressSpellNotPracticedThenLearningAfterTypeIt(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-spell-practice", progNow)
	lex := addLexeme(t, store, "כֶּלֶב", "dog", "kelev", "m", "")
	seedProgressSnapshots(t, store, l.user.ID, 2, progEst)

	before := getLearner(t, h, l, "/progress?detail=1")
	spell := overviewRow(t, before, "spell")
	if strings.Contains(spell, "data-verdict=") {
		t.Fatalf("spell row still shows a verdict:\n%s", spell)
	}
	if rungSum(t, spell) != skillSummary(summaryOf(t, store, clk, l.user.ID), "spell").Ladder.Taught {
		t.Fatalf("spell rungs do not sum to taught:\n%s", spell)
	}

	ctx := context.Background()
	_, err := store.InsertCard(ctx, db.Card{
		UserID: l.user.ID, LexemeID: lex, Skill: fsrs.SkillRecall, State: fsrs.StateReview,
		Due: progNow.Add(-time.Hour), Stability: 2, Difficulty: 5, Reps: 2,
		LastReview: progNow.Add(-48 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := evidence.New(store, fsrs.New(store, clk), clk)
	_, err = rec.Attempt(ctx, evidence.Attempt{
		UserID: l.user.ID, LexemeID: lex, Skill: fsrs.SkillRecall, Activity: "type_it",
		Source: evidence.SourceReview, Prompt: "dog", Response: "כֶּלֶב", Correct: true,
		LettersPassed: true, Latency: 3 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	after := getLearner(t, h, l, "/progress?detail=1")
	row := overviewRow(t, after, "spell")
	got := skillSummary(summaryOf(t, store, clk, l.user.ID), "spell").Ladder
	if rungCount(t, row, "proved") != got.Proved || rungSum(t, row) != got.Taught {
		t.Fatalf("spell ladder on the page does not match the model %+v:\n%s", got, row)
	}
}

// TestProgressChartsSayThroughToday is AC5 / ADR-0028 D2.
func TestProgressChartsSayThroughToday(t *testing.T) {
	_, _, _, _, get := progressApp(t)
	body := get("/progress?skill=read")
	est := section(t, body, "c-card pchart", `data-chart="est"`)
	ret := section(t, body, "c-card pchart", `data-chart="ret"`)
	for name, card := range map[string]string{"estimate": est, "retention": ret} {
		if !strings.Contains(card, "Through today") {
			t.Errorf("%s chart missing Through today:\n%s", name, card)
		}
		if !strings.Contains(card, `class="pchart__note pchart__through"`) {
			t.Errorf("%s chart missing through note class", name)
		}
		if regexp.MustCompile(`Through (Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)`).MatchString(card) {
			t.Errorf("%s chart still names a day before today:\n%s", name, card)
		}
	}
}

// ---- INIT-033/SPEC-007: where you stand ------------------------------------

// standingLearner1App is a learner-1-shaped fixture: recall needs help, read is strong,
// listen has no audio, and placement dates differ per skill (ADR-0028 D5, D7).
func standingLearner1App(t *testing.T) (*db.Store, learnerFix, clock.Clock, func(path string) string, []lexWord) {
	t.Helper()
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-standing", progNow)
	words := seedLexicon(t, store, progNow, 8, false, true)
	// Read strong: many known words + enough right answers.
	for i := 0; i < 8; i++ {
		seedKnownSkill(t, store, clk, l.user.ID, words[i].id, "read")
	}
	seedAttemptWindowWeb(t, store, clk, l.user.ID, words, "read", 189, 33)
	// Recall needs help: 50 of 68 right, plus struggling word.
	strugglingWord(t, store, clk, l.user.ID, words[0].id, "recall", 5, 1)
	seedAttemptWindowWeb(t, store, clk, l.user.ID, words, "recall", 50, 18)
	// Confusions on the top miss so the help cause is concrete.
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		at := progNow.Add(-time.Duration(i+1) * time.Hour)
		if _, err := store.ExecContext(ctx, `INSERT INTO confusion_misses (learner_id, kind, key, lexeme_id, at) VALUES (?, 'letter_pair', 'ב/כ', ?, ?)`,
			l.user.ID, words[0].id, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}
	seedProgressSnapshots(t, store, l.user.ID, 10, progEst)
	// Two placement runs: older reading, newer listening-only — D7 per-skill date.
	lex, err := store.UpsertPlacementLexeme(ctx, "שָׁלוֹם", "hello", "noun", "shalom", "")
	if err != nil {
		t.Fatal(err)
	}
	readAt := progNow.AddDate(0, 0, -20)
	listenAt := progNow.AddDate(0, 0, -2)
	readRun, err := store.StartPlacementV3(ctx, l.user.ID, readAt, 1, db.RunInitial, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	claimProgressPlacement(t, store, readRun, 1, db.PartReading, "read", 2, "r-1", "right", lex, readAt)
	claimProgressPlacement(t, store, readRun, 2, db.PartReading, "read", 2, "r-2", "wrong", lex, readAt)
	if err := store.FinishPlacement(ctx, readRun, readAt, 2, "exhausted",
		`{"estimated_known_words":90,"bands":[{"band":2,"rated":2,"good":1,"again":1,"good_plus_rate":0.5}],"listening":{"tested":false,"level":0,"estimate":0},"reading":{"tested":true,"level":2,"estimate":90}}`); err != nil {
		t.Fatal(err)
	}
	listenRun, err := store.StartPlacementV3(ctx, l.user.ID, listenAt, 2, "recheck", "listen", 2)
	if err != nil {
		t.Fatal(err)
	}
	claimProgressPlacement(t, store, listenRun, 1, db.PartListening, "listen", 1, "l-1", "right", lex, listenAt)
	if err := store.FinishPlacement(ctx, listenRun, listenAt, 3, "exhausted",
		`{"estimated_known_words":164,"bands":[{"band":1,"rated":1,"good":1,"good_plus_rate":1}],"listening":{"tested":true,"level":3,"estimate":164},"reading":{"tested":false,"level":0,"estimate":0}}`); err != nil {
		t.Fatal(err)
	}
	get := func(path string) string { return getLearner(t, h, l, path) }
	return store, l, clk, get, words
}

func seedAttemptWindowWeb(t *testing.T, store *db.Store, clk clock.Clock, userID int64, words []lexWord, skill string, right, wrong int) {
	t.Helper()
	ctx := context.Background()
	total := right + wrong
	for i := 0; i < total; i++ {
		w := words[i%len(words)]
		at := clk.Now().Add(-time.Duration(i+1) * time.Hour)
		ok := i < right
		if _, err := store.ExecContext(ctx, `
			INSERT INTO attempts (
				user_id, lexeme_id, skill, activity, source, prompt, response,
				correct, dont_know, hint_used, latency_ms, created_at
			) VALUES (?, ?, ?, 'test', 'review', '', '', ?, 0, 0, 0, ?)`,
			userID, w.id, skill, boolInt(ok), db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func TestProgressKnownAnyLastPointEqualsHeadline(t *testing.T) {
	store, l, clk, get, _ := standingLearner1App(t)
	sum := summaryOf(t, store, clk, l.user.ID)
	body := get("/progress?detail=1")
	want := strconv.Itoa(skillSummary(sum, "recall").Ladder.Proved)
	recall := overviewRow(t, body, "recall")
	if got := rungCount(t, recall, "proved"); strconv.Itoa(got) != want {
		t.Fatalf("proved %d, want %s", got, want)
	}
	est := tableRows(section(t, body, "c-card pchart", `data-chart="est"`))
	if len(est) == 0 || est[len(est)-1][1] != want {
		t.Fatalf("chart last point %v, want headline %s", est[len(est)-1], want)
	}
	if !strings.Contains(section(t, body, "c-card pchart", `data-chart="est"`), "Through today") {
		t.Fatal("missing Through today")
	}
}

func TestProgressDirectionNamesAndNoAudio(t *testing.T) {
	_, _, _, get, _ := standingLearner1App(t)
	body := get("/progress?detail=1")
	for _, name := range []string{
		"Understand by ear", "Hebrew from English", "Reading", "Spelling",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("missing ladder name %q", name)
		}
	}
	if strings.Contains(body, "Say it from English") {
		t.Fatal("the old pronunciation name is on the page")
	}
	listen := overviewRow(t, body, "listen")
	if !strings.Contains(listen, "No audio yet") {
		t.Fatalf("listen missing No audio yet:\n%s", listen)
	}
}

func TestProgressHelpAndStrongSections(t *testing.T) {
	_, _, _, get, words := standingLearner1App(t)
	body := get("/progress?detail=1")
	help := section(t, body, "c-card pstand", `data-stand="help"`)
	if !strings.Contains(help, "Where you need help") || !strings.Contains(help, "English → Hebrew") {
		t.Fatalf("help section:\n%s", help)
	}
	if !strings.Contains(help, words[0].he) {
		t.Fatalf("help missing top missed word %s:\n%s", words[0].he, help)
	}
	if !strings.Contains(help, "swaps") && !strings.Contains(help, "can read it") && !strings.Contains(help, "mixed up") {
		t.Fatalf("help missing a plain cause:\n%s", help)
	}
	if !strings.Contains(help, "practice these") || !strings.Contains(help, `skill=recall`) {
		t.Fatalf("help missing practice link:\n%s", help)
	}
	strong := section(t, body, "c-card pstand", `data-stand="strong"`)
	if !strings.Contains(strong, "Where you") || !strings.Contains(strong, "strong") || !strings.Contains(strong, "Reading (Hebrew → English)") {
		t.Fatalf("strong section:\n%s", strong)
	}
}

func TestProgressReadingPlacementUsesReadingRunDate(t *testing.T) {
	_, _, _, get, _ := standingLearner1App(t)
	body := get("/progress?detail=1")
	read := overviewRow(t, body, "read")
	if strings.Contains(read, "Placement estimate:") {
		t.Fatalf("reading row still shows a placement estimate:\n%s", read)
	}
	place := section(t, body, "c-card pplace", "")
	// Reading run finished ~20 days before progNow (Oct 18) → Sep 28.
	if !strings.Contains(place, "Sep 28") {
		t.Fatalf("placement history should use the reading run date (Sep 28):\n%s", place)
	}
}

func TestProgressOldRuleDaysAreDashed(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-old-rule", progNow)
	seedLexicon(t, store, progNow, 3, false, true)
	ctx := context.Background()
	ret := 0.8
	day := "2026-10-16"
	var rows []db.DailySnapshotRow
	for _, skill := range learner.Skills {
		for _, st := range learner.States {
			rows = append(rows, db.DailySnapshotRow{
				Skill: skill, State: string(st), Count: 2,
				EstKnown: 10, Retention: &ret, RuleVersion: 1,
			})
		}
	}
	if err := store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		if err := tx.InsertSnapshot(ctx, l.user.ID, day, rows); err != nil {
			return err
		}
		return tx.InsertTotals(ctx, l.user.ID, day, 4, 1)
	}); err != nil {
		t.Fatal(err)
	}
	body := getLearner(t, h, l, "/progress?detail=1")
	est := section(t, body, "c-card pchart", `data-chart="est"`)
	if strings.Contains(est, "old rule") || strings.Contains(est, "pc__line--old") || strings.Contains(est, "Oct 16") {
		t.Fatalf("a rule_version=1 day is still on the chart:\n%s", est)
	}
	if !strings.Contains(est, "Through today") {
		t.Fatal("missing Through today")
	}

	// Days stamped 2 and 3 (literal) are below current and must also render dashed.
	// Assert each day's label so a leftover rule_version=1 dash cannot mask a miss.
	for _, old := range []struct {
		day string
		rv  int
		lbl string
	}{
		{"2026-10-17", 2, "Oct 17 (old rule)"},
		// Before today (progNow is 2026-10-18): a same-day total would be replaced by the live point.
		{"2026-10-15", 3, "Oct 15 (old rule)"}, // INIT-036/SPEC-003: v3 still renders under rule 4
	} {
		var rowsOld []db.DailySnapshotRow
		for _, skill := range learner.Skills {
			for _, st := range learner.States {
				rowsOld = append(rowsOld, db.DailySnapshotRow{
					Skill: skill, State: string(st), Count: 2,
					EstKnown: 11, Retention: &ret, RuleVersion: old.rv,
				})
			}
		}
		if err := store.InDailyTx(ctx, func(tx *db.DailyTx) error {
			if err := tx.InsertSnapshot(ctx, l.user.ID, old.day, rowsOld); err != nil {
				return err
			}
			return tx.InsertTotals(ctx, l.user.ID, old.day, 5, old.rv)
		}); err != nil {
			t.Fatal(err)
		}
		body = getLearner(t, h, l, "/progress?detail=1")
		est = section(t, body, "c-card pchart", `data-chart="est"`)
		if strings.Contains(est, old.lbl) || strings.Contains(est, "old rule") || strings.Contains(est, "pc__line--old") {
			t.Fatalf("rule_version=%d day is on the chart:\n%s", old.rv, est)
		}
	}
}

func TestProgressPracticeLinkListsThatDirectionsStrugglingWords(t *testing.T) {
	_, _, _, get, words := standingLearner1App(t)
	recall := get("/me/words?state=struggling&skill=recall")
	if !strings.Contains(recall, words[0].he) || !strings.Contains(recall, "English → Hebrew") {
		t.Fatalf("recall struggling list should name the direction and list %s:\n%s", words[0].he, recall)
	}
	if read := get("/me/words?state=struggling&skill=read"); strings.Contains(read, words[0].he) {
		t.Fatalf("%s is known in reading; the reading struggling list should leave it out:\n%s", words[0].he, read)
	}
}

func TestProgressHelpReadsInHebrew(t *testing.T) {
	_, _, _, get, _ := standingLearner1App(t)
	help := section(t, get("/progress?detail=1&lang=he"), "c-card pstand", `data-stand="help"`)
	for _, want := range []string{"מַחְלִיף בֵּין", "שְׁגִיאוֹת מִתּוֹךְ"} {
		if !strings.Contains(help, want) {
			t.Fatalf("Hebrew help section missing %q:\n%s", want, help)
		}
	}
	for _, english := range []string{"swaps", "missed "} {
		if strings.Contains(help, english) {
			t.Fatalf("Hebrew help section still says %q:\n%s", english, help)
		}
	}
}

// seedWaitingSkill makes a word pass the old same-day known rule on one study day
// only, so Summary.WaitingForSecondDay counts it (INIT-036/SPEC-006).
func seedWaitingSkill(t *testing.T, store *db.Store, clk clock.Clock, userID, lexemeID int64, skill string) {
	t.Helper()
	now := clk.Now()
	day1 := now.AddDate(0, 0, -1)
	ctx := context.Background()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		at := day1.Add(time.Duration(i) * time.Minute)
		if _, err := conn.ExecContext(ctx, `
			INSERT INTO attempts (user_id, lexeme_id, skill, activity, source, prompt, response, correct, dont_know, hint_used, latency_ms, created_at)
			VALUES (?, ?, ?, 'test', 'review', '', '', 1, 0, 0, 0, ?)`,
			userID, lexemeID, skill, db.FormatTime(at)); err != nil {
			t.Fatal(err)
		}
	}
	cardID, err := store.InsertCard(ctx, db.Card{
		UserID: userID, LexemeID: lexemeID, Skill: skill, State: fsrs.StateReview,
		Due: now, Stability: 10, Difficulty: 5, Reps: 3, LastReview: day1,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		insertReview(t, store, cardID, int(fsrs.Good), day1.Add(time.Duration(i)*time.Minute))
	}
}

// TestProgressWaitingShowsBesideKnown is INIT-036/SPEC-006: Progress shows
// "Words you know" and "learned today, check tomorrow" as separate figures.
func TestProgressWaitingShowsBesideKnown(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-waiting-progress", progNow)
	ids := overviewLexemes(t, store, 3)
	seedKnownSkill(t, store, clk, l.user.ID, ids[0], fsrs.SkillRead)
	seedWaitingSkill(t, store, clk, l.user.ID, ids[1], fsrs.SkillRead)
	seedProgressSnapshots(t, store, l.user.ID, 2, progEst)

	sum, err := learner.New(store, clk).Summary(context.Background(), l.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.WaitingForSecondDay != 1 {
		t.Fatalf("WaitingForSecondDay = %d, want 1", sum.WaitingForSecondDay)
	}
	if sum.KnownAnySkill < 1 {
		t.Fatalf("KnownAnySkill = %d, want ≥1", sum.KnownAnySkill)
	}

	body := getLearner(t, h, l, "/progress?detail=1")
	know := section(t, body, "c-card pknow", "")
	if strings.Contains(know, "Words you know") || strings.Contains(know, views.LabelLearnedTodayCheckTomorrow) || strings.Contains(know, views.LabelKnownExplain) {
		t.Fatalf("the old headline is still on Progress:\n%s", know)
	}
	if !strings.Contains(know, "Hebrew from English") || !strings.Contains(know, "Understand by ear") {
		t.Fatalf("ladder rows missing:\n%s", know)
	}
	home := getLearner(t, h, l, "/")
	if !strings.Contains(home, views.LabelLearnedTodayCheckTomorrow) {
		t.Fatal("Today should still show the waiting figure")
	}
}

// TestProgressWaitingAbsentWhenNoneWait hides the second count when zero.
func TestProgressWaitingAbsentWhenNoneWait(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-waiting-zero", progNow)
	ids := overviewLexemes(t, store, 2)
	seedKnownSkill(t, store, clk, l.user.ID, ids[0], fsrs.SkillRead)
	seedProgressSnapshots(t, store, l.user.ID, 2, progEst)

	sum, err := learner.New(store, clk).Summary(context.Background(), l.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.WaitingForSecondDay != 0 {
		t.Fatalf("WaitingForSecondDay = %d, want 0", sum.WaitingForSecondDay)
	}

	body := getLearner(t, h, l, "/progress?detail=1")
	know := section(t, body, "c-card pknow", "")
	if strings.Contains(know, views.LabelLearnedTodayCheckTomorrow) || strings.Contains(know, views.LabelKnownExplain) || strings.Contains(know, "Words you know") {
		t.Fatalf("old headline copy should be absent:\n%s", know)
	}
	if !strings.Contains(know, "Hebrew from English") {
		t.Fatalf("ladder headline missing:\n%s", know)
	}
	home := getLearner(t, h, l, "/")
	if strings.Contains(home, views.LabelLearnedTodayCheckTomorrow) {
		t.Fatalf("Today should omit waiting when none wait:\n%s", home)
	}
}

// TestProgressRuleChangeMarkerOnlyOnMixedVersion is INIT-036/SPEC-006: the
// old-rule chart marker appears only when the range mixes rule versions.
func TestProgressRuleChangeMarkerOnlyOnMixedVersion(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-rule-marker", progNow)
	seedLexicon(t, store, progNow, 3, false, true)
	ctx := context.Background()
	ret := 0.8

	// Current-rule-only history: no change marker.
	dayCur := "2026-10-16"
	var rowsCur []db.DailySnapshotRow
	for _, skill := range learner.Skills {
		for _, st := range learner.States {
			rowsCur = append(rowsCur, db.DailySnapshotRow{
				Skill: skill, State: string(st), Count: 2,
				EstKnown: 10, Retention: &ret, RuleVersion: jobs.RuleVersion,
			})
		}
	}
	if err := store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		if err := tx.InsertSnapshot(ctx, l.user.ID, dayCur, rowsCur); err != nil {
			return err
		}
		return tx.InsertTotals(ctx, l.user.ID, dayCur, 4, jobs.RuleVersion)
	}); err != nil {
		t.Fatal(err)
	}
	est := section(t, getLearner(t, h, l, "/progress?detail=1"), "c-card pchart", `data-chart="est"`)
	if strings.Contains(est, `class="pchart__note pchart__old"`) || strings.Contains(est, "Dashed: old rule") || strings.Contains(est, "old rule") {
		t.Fatalf("the chart must use one rule version:\n%s", est)
	}

	// Mix in a rule-version-3 day: marker must appear.
	dayOld := "2026-10-15"
	var rowsOld []db.DailySnapshotRow
	for _, skill := range learner.Skills {
		for _, st := range learner.States {
			rowsOld = append(rowsOld, db.DailySnapshotRow{
				Skill: skill, State: string(st), Count: 2,
				EstKnown: 9, Retention: &ret, RuleVersion: 3,
			})
		}
	}
	if err := store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		if err := tx.InsertSnapshot(ctx, l.user.ID, dayOld, rowsOld); err != nil {
			return err
		}
		return tx.InsertTotals(ctx, l.user.ID, dayOld, 3, 3)
	}); err != nil {
		t.Fatal(err)
	}
	est = section(t, getLearner(t, h, l, "/progress?detail=1"), "c-card pchart", `data-chart="est"`)
	if strings.Contains(est, `class="pchart__note pchart__old"`) || strings.Contains(est, "Dashed: old rule") || strings.Contains(est, "Oct 15") {
		t.Fatalf("a mixed-version day is on the chart:\n%s", est)
	}
}

// TestProgressTodayAndDigestShareWaitingCounts is INIT-036/SPEC-006: Progress,
// Today, and get_study_digest share KnownAnySkill and WaitingForSecondDay.
func TestProgressTodayAndDigestShareWaitingCounts(t *testing.T) {
	clk := fixedClock(progNow)
	store, h := openShell(t, clk, nil, nil)
	l := addLearner(t, store, "a@example.com", "sub-waiting-agree", progNow)
	setLearnerZone(t, store, l.user.ID, "America/Denver")
	ids := overviewLexemes(t, store, 4)
	seedKnownSkill(t, store, clk, l.user.ID, ids[0], fsrs.SkillRead)
	seedKnownSkill(t, store, clk, l.user.ID, ids[1], fsrs.SkillRead)
	seedWaitingSkill(t, store, clk, l.user.ID, ids[2], fsrs.SkillRead)
	seedProgressSnapshots(t, store, l.user.ID, 2, progEst)

	sum, err := learner.New(store, clk).Summary(context.Background(), l.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.WaitingForSecondDay != 1 || sum.KnownAnySkill < 2 {
		t.Fatalf("summary known=%d waiting=%d", sum.KnownAnySkill, sum.WaitingForSecondDay)
	}
	wantProved := strconv.Itoa(skillSummary(sum, "recall").Ladder.Proved)
	wantWait := strconv.Itoa(sum.WaitingForSecondDay)

	progress := getLearner(t, h, l, "/progress?detail=1")
	know := section(t, progress, "c-card pknow", "")
	if strings.Contains(know, "Words you know") || strings.Contains(know, views.LabelLearnedTodayCheckTomorrow) {
		t.Fatalf("Progress still shows the old headline:\n%s", know)
	}
	if got := rungCount(t, overviewRow(t, progress, "recall"), "proved"); strconv.Itoa(got) != wantProved {
		t.Fatalf("Progress proved = %d, want %s", got, wantProved)
	}

	home := getLearner(t, h, l, "/")
	reFigure := regexp.MustCompile(`(?s)aria-label="([^"]+)"[^>]*>\s*<span class="count"><bdi[^>]*>([0-9]+)</bdi>`)
	homeFigs := map[string]string{}
	for _, m := range reFigure.FindAllStringSubmatch(home, -1) {
		homeFigs[m[1]] = m[2]
	}
	if homeFigs["Words you know"] != wantProved || homeFigs[views.LabelLearnedTodayCheckTomorrow] != wantWait {
		t.Fatalf("Today figures = %v, want known=%s waiting=%s", homeFigs, wantProved, wantWait)
	}

	z := denverZone(t)
	digest, err := mcpstore.New(store).StudyDigest(context.Background(), l.user.ID, progNow, z)
	if err != nil {
		t.Fatal(err)
	}
	if digest.KnownAnySkill != sum.KnownAnySkill || digest.WaitingForSecondDay != sum.WaitingForSecondDay {
		t.Fatalf("digest known=%d waiting=%d, want known=%d waiting=%d",
			digest.KnownAnySkill, digest.WaitingForSecondDay, sum.KnownAnySkill, sum.WaitingForSecondDay)
	}
}
