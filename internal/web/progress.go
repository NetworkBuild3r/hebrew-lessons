package web

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/sessionlimits"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// The Progress page (INIT-015/SPEC-018). It reads what the AI reads through get_progress and
// get_learner_model: learner.Model for the states now, and the db queries of internal/db/progress.go
// for the daily snapshots, the regression events, the placement runs, and the weekly reports.
// Every query is keyed on the signed-in learner, and nothing here takes a learner id from the request.
// The one choice a request makes is the skill, ?skill=listen|read|recall|spell.

const (
	dayLayout = "2006-01-02"

	// progressWindowDays is how many days of snapshots the line charts read.
	progressWindowDays = 90
	// weekDays is the span of "This week": the newest snapshot day and the seven before it.
	weekDays = 7
	// backWindowDays is how far back a slip counts for "Back on track".
	backWindowDays = 28
	// eventCap is the most regression events one page reads.
	eventCap = 200
	// slippedMax, backMax, and placementMax cap the lists on the page.
	slippedMax   = 8
	backMax      = 6
	placementMax = 6

	// retentionBandHigh is the top of the retention target band. The bottom is the session target,
	// sessionlimits.TargetSuccess (85%). The band is 85 to 90%.
	retentionBandHigh = 0.90
)

// retentionBandLow is the bottom of the target band.
const retentionBandLow = sessionlimits.TargetSuccess

// skillTabLabels are the four tab names, in the order of learner.Skills.
var skillTabLabels = map[string]string{"listen": "Listen", "read": "Read", "recall": "Recall", "spell": "Spell"}

// chooseSkill is the skill of a request: one of learner.Skills, else the first.
func chooseSkill(q string) string {
	if slices.Contains(learner.Skills, q) {
		return q
	}
	return learner.Skills[0]
}

func (s *server) serveProgress(w http.ResponseWriter, r *http.Request, secure bool, csrf string) {
	lang := chooseLang(w, r, secure)
	user, ok := auth.CurrentUser(r.Context())
	gender := views.AddressMasculine
	if ok && user != nil && s != nil && s.store != nil {
		g, err := s.addressGender(r.Context(), user.ID)
		if err != nil {
			serverError(w)
			return
		}
		gender = g
	}
	ui := views.UIFor(lang, gender)
	skill := chooseSkill(r.URL.Query().Get("skill"))
	v := views.ProgressView{
		Page:      views.Page{UI: ui, Title: ui.DocTitle("progress"), CSRF: csrf, Current: "progress"},
		Empty:     true,
		Skill:     skill,
		SkillName: views.SkillLabel(skill),
	}
	for _, sk := range learner.Skills {
		v.Tabs = append(v.Tabs, views.SkillTab{Skill: sk, Label: skillTabLabels[sk], Current: sk == skill})
	}
	if ok && user != nil && s != nil && s.store != nil && s.store.Clock() != nil {
		if err := s.fillProgress(r.Context(), user.ID, &v); err != nil {
			serverError(w)
			return
		}
	}
	render(w, r, views.Progress(v))
}

// snapDay is the snapshot rows of one day, by skill.
type snapDay struct {
	Day    string
	Skills map[string]snapSkill
}

// snapSkill is one skill on one day. est_known and retention repeat on every state row of the
// skill, so any row gives them.
type snapSkill struct {
	Counts    map[string]int
	EstKnown  float64
	Retention *float64
}

// groupDays groups snapshot rows (ordered by day, skill, state) into days, oldest first.
func groupDays(rows []db.SnapshotRow) []snapDay {
	var out []snapDay
	for _, r := range rows {
		if len(out) == 0 || out[len(out)-1].Day != r.Day {
			out = append(out, snapDay{Day: r.Day, Skills: map[string]snapSkill{}})
		}
		d := out[len(out)-1]
		sk, ok := d.Skills[r.Skill]
		if !ok {
			sk = snapSkill{Counts: map[string]int{}}
		}
		sk.Counts[r.State] = r.Count
		sk.EstKnown = math.Max(sk.EstKnown, r.EstKnown)
		if r.Retention != nil {
			sk.Retention = r.Retention
		}
		d.Skills[r.Skill] = sk
	}
	return out
}

// knownOf is the words a skill knew that day: known and solid.
func knownOf(sk snapSkill) int {
	return sk.Counts[string(learner.Known)] + sk.Counts[string(learner.Solid)]
}

// weakOf is the words struggling or regressed that day.
func weakOf(sk snapSkill) int {
	return sk.Counts[string(learner.Struggling)] + sk.Counts[string(learner.Regressed)]
}

// weekChanges is the move in words known per skill between the first and the last day of days,
// in the order of learner.Skills. It is get_progress's net change (mcpstore.skillChanges): the
// same two snapshot days, the same known-plus-solid count. A skill missing on either day is left out.
func weekChanges(days []snapDay) []views.WeekSkill {
	if len(days) < 2 {
		return nil
	}
	first, last := days[0], days[len(days)-1]
	var out []views.WeekSkill
	for _, skill := range learner.Skills {
		a, okA := first.Skills[skill]
		b, okB := last.Skills[skill]
		if !okA || !okB {
			continue
		}
		start, end := knownOf(a), knownOf(b)
		out = append(out, views.WeekSkill{
			Skill: skill, Label: views.SkillLabel(skill),
			Gained: max(end-start, 0), Lost: max(start-end, 0),
			StruggleStart: weakOf(a), StruggleEnd: weakOf(b),
		})
	}
	return out
}

func addDaysStr(day string, n int) string {
	t, err := time.Parse(dayLayout, day)
	if err != nil {
		return day
	}
	return t.AddDate(0, 0, n).Format(dayLayout)
}

// fillProgress reads everything the page shows for one learner. A failed read is the caller's
// error: a chart that quietly leaves out a number would be a wrong picture.
func (s *server) fillProgress(ctx context.Context, userID int64, v *views.ProgressView) error {
	now := s.now().UTC()
	today := now.Format(dayLayout)

	_, _, done, total, lessonsDone, lessonsTotal, err := s.unitProgress(ctx, userID)
	if err != nil {
		return err
	}
	if total > 0 {
		v.UnitsText = strconv.Itoa(done) + "/" + strconv.Itoa(total)
	}
	if lessonsTotal > 0 {
		v.LessonsText = strconv.Itoa(lessonsDone) + "/" + strconv.Itoa(lessonsTotal)
	}

	// The job writes yesterday, so 90 snapshot days end the day before today and begin 90 days back.
	rows, err := s.store.SnapshotRows(ctx, userID, addDaysStr(today, -progressWindowDays), today)
	if err != nil {
		return err
	}
	days := groupDays(rows)

	sum, err := s.model().Summary(ctx, userID)
	if err != nil {
		return err
	}

	// A day later than today (a clock set back) still counts as the newest snapshot.
	ref := today
	if len(days) > 0 && days[len(days)-1].Day > ref {
		ref = days[len(days)-1].Day
	}
	reports, err := s.store.WeeklyReportRows(ctx, userID, "1970-01-01", ref, 1)
	if err != nil {
		return err
	}
	if len(reports) > 0 {
		v.Weekly = &views.WeeklyCard{WeekOf: "Week of " + shortDay(reports[0].WeekStart), Summary: reports[0].Summary}
	}

	seen := 0
	for st, n := range sum.Words {
		if st != learner.Unseen {
			seen += n
		}
	}
	v.Empty = len(days) == 0 && seen == 0 && v.Weekly == nil
	if v.Empty {
		return nil
	}

	s.fillSkillPanel(v, sum, days)
	if err := s.fillWeek(ctx, userID, v, days, ref); err != nil {
		return err
	}
	return s.fillPlacementHistory(ctx, userID, v, now)
}

// fillSkillPanel fills the bar from the learner model now, and the two line charts from the
// snapshots.
func (s *server) fillSkillPanel(v *views.ProgressView, sum learner.Summary, days []snapDay) {
	skill := v.Skill
	var ss learner.SkillSummary
	for _, c := range sum.Skills {
		if c.Skill == skill {
			ss = c
		}
	}
	v.Known = fmt.Sprintf("%.0f", ss.EstimatedKnown)

	total := 0
	for _, st := range learner.States {
		total += ss.Counts[st]
	}
	v.Total = total
	segs := make([]barSeg, 0, len(learner.States))
	var desc []string
	table := views.ChartTable{
		Caption: "Words by state for " + v.SkillName + ", now",
		Head:    []string{"State", "Words", "Share of all words"},
	}
	for _, st := range learner.States {
		n := ss.Counts[st]
		share := "0%"
		if total > 0 {
			share = percent(float64(n) / float64(total))
		}
		v.Legend = append(v.Legend, views.LegendItem{State: string(st), Label: views.StateLabel(string(st)), Count: n, Share: share})
		segs = append(segs, barSeg{State: string(st), Count: n})
		desc = append(desc, strconv.Itoa(n)+" "+strings.ToLower(views.StateLabel(string(st))))
		table.Rows = append(table.Rows, []string{views.StateLabel(string(st)), strconv.Itoa(n), share})
	}
	v.BarTable = table
	v.BarSVG = stackedBarSVG("pb", "Words by state for "+v.SkillName+": "+strings.Join(desc, ", "), segs)

	v.Est = estChart(skill, v.SkillName, days)
	v.Retention = retentionChart(skill, v.SkillName, days, ss.Retention)
}

// estChart is the estimated words known over the snapshot days of one skill.
func estChart(skill, name string, days []snapDay) views.ChartView {
	c := views.ChartView{ID: "est", Title: "Estimated words known"}
	var pts []linePoint
	for _, d := range days {
		if sk, ok := d.Skills[skill]; ok {
			pts = append(pts, linePoint{Day: d.Day, Value: sk.EstKnown})
		}
	}
	if len(pts) == 0 {
		c.Hint = "Practice a few days and your chart starts here."
		return c
	}
	lo, hi := pts[0].Value, pts[0].Value
	for _, p := range pts {
		lo, hi = math.Min(lo, p.Value), math.Max(hi, p.Value)
	}
	minY, maxY, ticks := niceCount(lo, hi)
	c.SVG = lineChartSVG(lineSpec{
		ID: "est", Label: "Estimated words known, " + name, Points: pts,
		Min: minY, Max: maxY, Ticks: ticks,
	})
	first, last := pts[0], pts[len(pts)-1]
	c.Note = "From the daily snapshots, " + shortDay(first.Day) + " to " + shortDay(last.Day) + "."
	switch {
	case len(pts) == 1:
		c.Summary = fmt.Sprintf("%.0f words on %s. A line needs a second day.", first.Value, shortDay(first.Day))
	default:
		d := int(math.Round(last.Value)) - int(math.Round(first.Value))
		move := "no change"
		switch {
		case d > 0:
			move = "up " + views.CountText(d, "word", "words")
		case d < 0:
			move = "down " + views.CountText(-d, "word", "words")
		}
		c.Summary = fmt.Sprintf("%.0f to %.0f · %s", first.Value, last.Value, move)
	}
	c.Table = views.ChartTable{Caption: "Estimated words known, " + name + ", by day", Head: []string{"Day", "Estimated words known"}}
	for _, p := range pts {
		c.Table.Rows = append(c.Table.Rows, []string{shortDay(p.Day), fmt.Sprintf("%.0f", p.Value)})
	}
	return c
}

// retentionChart is the share of due reviews remembered, over the snapshot days, with the
// target band. live is the model's retention over the last 30 days, or nil.
func retentionChart(skill, name string, days []snapDay, live *float64) views.ChartView {
	c := views.ChartView{ID: "ret", Title: "Retention"}
	band := "The target is " + percent(retentionBandLow) + " to " + percent(retentionBandHigh) + "."
	if live != nil {
		c.Note = "Last 30 days: " + percent(*live) + ". " + band
	} else {
		c.Note = "Share of due reviews you remembered. " + band
	}
	var pts []linePoint
	for _, d := range days {
		if sk, ok := d.Skills[skill]; ok && sk.Retention != nil {
			pts = append(pts, linePoint{Day: d.Day, Value: *sk.Retention})
		}
	}
	if len(pts) == 0 {
		c.Hint = "Retention starts once you have due reviews to answer."
		return c
	}
	lo := pts[0].Value
	for _, p := range pts {
		lo = math.Min(lo, p.Value)
	}
	minY, maxY, ticks := niceShare(lo)
	b := [2]float64{retentionBandLow, retentionBandHigh}
	c.SVG = lineChartSVG(lineSpec{
		ID: "ret", Label: "Retention, " + name + ", against the " + percent(retentionBandLow) + " to " + percent(retentionBandHigh) + " target", Points: pts,
		Min: minY, Max: maxY, Ticks: ticks, Format: percent, Band: &b,
	})
	c.Key = []string{"Shaded band between dashed lines: the " + percent(retentionBandLow) + " to " + percent(retentionBandHigh) + " target"}
	last := pts[len(pts)-1]
	where := "inside the target"
	switch {
	case last.Value < retentionBandLow:
		where = "below the target"
	case last.Value > retentionBandHigh:
		where = "above the target"
	}
	c.Summary = "Newest snapshot " + percent(last.Value) + " on " + shortDay(last.Day) + ", " + where + "."
	c.Table = views.ChartTable{Caption: "Retention, " + name + ", by day", Head: []string{"Day", "Retention"}}
	for _, p := range pts {
		c.Table.Rows = append(c.Table.Rows, []string{shortDay(p.Day), percent(p.Value)})
	}
	return c
}

// fillWeek fills "This week": gained and lost per skill from the week's snapshots, the words that
// slipped (with cause), the bands that slipped, and the words that came back.
func (s *server) fillWeek(ctx context.Context, userID int64, v *views.ProgressView, days []snapDay, ref string) error {
	from := addDaysStr(ref, -weekDays)
	var week []snapDay
	for _, d := range days {
		if d.Day >= from && d.Day <= ref {
			week = append(week, d)
		}
	}
	if len(week) >= 2 {
		v.Week.Compared = true
		v.Week.From, v.Week.To = week[0].Day, week[len(week)-1].Day
		v.Week.Skills = weekChanges(week)
	}

	to := ref
	if today := s.now().UTC().Format(dayLayout); today > to {
		to = today
	}
	events, err := s.store.RegressionRows(ctx, userID, addDaysStr(ref, -backWindowDays), to, eventCap)
	if err != nil {
		return err
	}
	bands := map[int]bool{}
	type pair struct {
		lexeme int64
		skill  string
	}
	backSeen := map[pair]bool{}
	var back []db.RegressionRow
	for _, e := range events { // newest first
		inWeek := e.Day >= from
		if e.Kind == db.EventBandSlip {
			if b, ok := bandOfCause(e.Cause); ok && inWeek && !bands[b] {
				bands[b] = true
				v.Week.Bands = append(v.Week.Bands, b+1)
			}
			continue
		}
		if inWeek && len(v.Week.Slipped) < slippedMax {
			w, err := s.weekWord(ctx, e)
			if err != nil {
				return err
			}
			v.Week.Slipped = append(v.Week.Slipped, w)
		}
		p := pair{e.LexemeID, e.Skill}
		if !backSeen[p] {
			backSeen[p] = true
			back = append(back, e)
		}
	}
	// A word is back when its newest event is behind it and the skill is no longer weak.
	model := s.model()
	checked := 0
	for _, e := range back {
		if len(v.Week.Back) >= backMax || checked >= 3*backMax {
			break
		}
		checked++
		w, err := model.Word(ctx, userID, e.LexemeID)
		if err != nil {
			return err
		}
		still := false
		for _, sk := range w.Skills {
			if sk.Skill == e.Skill && (sk.State == learner.Struggling || sk.State == learner.Regressed) {
				still = true
			}
		}
		if still {
			continue
		}
		ww, err := s.weekWord(ctx, e)
		if err != nil {
			return err
		}
		v.Week.Back = append(v.Week.Back, ww)
	}
	slices.Sort(v.Week.Bands)
	return nil
}

// bandOfCause reads the band out of a band_slip cause, "band_slip band=2 known share ...".
func bandOfCause(cause string) (int, bool) {
	_, rest, ok := strings.Cut(cause, "band=")
	if !ok {
		return 0, false
	}
	digits, _, _ := strings.Cut(rest, " ")
	n, err := strconv.Atoi(digits)
	if err != nil || n < 0 || n > 4 {
		return 0, false
	}
	return n, true
}

// weekWord is one regression event as a row: the word, its meaning, the skill, and the cause.
func (s *server) weekWord(ctx context.Context, e db.RegressionRow) (views.WeekWord, error) {
	w := views.WeekWord{
		Text: e.Text, Skill: e.Skill, SkillLabel: views.SkillLabel(e.Skill),
		Cause: e.Cause, CauseLabel: views.CauseLabel(e.Cause), Day: e.Day,
	}
	face, ok, err := s.store.LexemeByID(ctx, e.LexemeID)
	if err != nil {
		return views.WeekWord{}, err
	}
	if ok {
		w.Gloss = face.Gloss
	}
	return w, nil
}

// placementResult is the part of placement_runs.result this page reads. A v2 run has no
// listening or reading.
type placementResult struct {
	EstimatedKnownWords int `json:"estimated_known_words"`
	Listening           *struct {
		Tested   bool `json:"tested"`
		Level    int  `json:"level"`
		Estimate int  `json:"estimate"`
	} `json:"listening"`
	Reading *struct {
		Tested   bool `json:"tested"`
		Level    int  `json:"level"`
		Estimate int  `json:"estimate"`
	} `json:"reading"`
}

// fillPlacementHistory fills the placement history: each finished run or check-in, newest first,
// with the level it found in each skill it tested.
func (s *server) fillPlacementHistory(ctx context.Context, userID int64, v *views.ProgressView, now time.Time) error {
	runs, err := s.store.PlacementHistory(ctx, userID, placementMax)
	if err != nil {
		return err
	}
	for _, r := range runs {
		row := views.PlacementRow{Date: dateText(r.FinishedAt, now), Kind: "Placement"}
		switch r.Kind {
		case "initial":
			row.Kind = "First placement"
		case "recheck":
			row.Kind = "Check-in"
		}
		var res placementResult
		if r.Result != "" {
			_ = json.Unmarshal([]byte(r.Result), &res)
		}
		var parts []string
		skill := func(label string, tested bool, level, estimate int) {
			if !tested {
				return
			}
			p := label
			if level > 0 {
				p += " level " + strconv.Itoa(level)
			}
			parts = append(parts, p+", about "+views.CountText(estimate, "word", "words"))
		}
		if res.Listening != nil {
			skill("Listening", res.Listening.Tested, res.Listening.Level, res.Listening.Estimate)
		}
		if res.Reading != nil {
			skill("Reading", res.Reading.Tested, res.Reading.Level, res.Reading.Estimate)
		}
		switch {
		case r.Level < 0:
			row.Level = "Not finished"
		case r.Kind == "":
			row.Level = "Level " + strconv.Itoa(r.Level)
		}
		if res.EstimatedKnownWords > 0 && len(parts) == 0 {
			parts = append(parts, "about "+views.CountText(res.EstimatedKnownWords, "word", "words")+" known")
		}
		row.Detail = strings.Join(parts, " · ")
		v.Placement = append(v.Placement, row)
	}
	return nil
}

// dateText is "Oct 2", with the year when it is not this year.
func dateText(t, now time.Time) string {
	t = t.UTC()
	if t.Year() != now.Year() {
		return t.Format("Jan 2, 2006")
	}
	return t.Format("Jan 2")
}
