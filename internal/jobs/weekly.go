package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lessons"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/sessionlimits"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// The weekly report (INIT-015/SPEC-023). It writes one weekly_reports row for each learner and
// each finished week. The package doc says what a week holds and what it cannot know.

const (
	// WeeklyDay is the weekday a week starts on.
	WeeklyDay = time.Monday

	// WeeklyHour and WeeklyMinute remain for NextWeekly tests; the scheduler is an hourly
	// sweep and no longer wakes at Monday 04:00. INIT-026/SPEC-004.
	WeeklyHour   = 4
	WeeklyMinute = 0

	// IdleGap is the longest pause between two answers that still counts as studying. A longer
	// pause starts a new sitting, and a sitting's first answer counts for the time it took.
	IdleGap = 3 * time.Minute

	// payloadVersion is the shape of weekly_reports.payload_json.
	payloadVersion = 1

	weeklyBaselineKey = "weekly.words."
)

// ErrWeekOpen means the week has not finished yet for every learner.
var ErrWeekOpen = errors.New("jobs: the week is not over")

// ErrNotMonday means the week does not start on a Monday.
var ErrNotMonday = errors.New("jobs: a week starts on a Monday")

// WeeklyResult counts what a weekly run did. It holds counts only, never a learner's words.
type WeeklyResult struct {
	// Week is the first day of the week. CatchUp may leave it empty when learners differ.
	Week string
	// Learners is how many learners were looked at.
	Learners int
	// Written is how many learners got a report.
	Written int
	// Skipped is how many learners already had a report for the week.
	Skipped int
	// NotYet is how many learners were skipped because their week is still open.
	NotYet int
	// Gained, Lost, and Cleared are the words named in the reports written by this run.
	Gained, Lost, Cleared int
	// BandSlips is the band slips in the reports written by this run.
	BandSlips int
	// ZoneLearner and ZoneApp count learners by studyday.Zone.Source. INIT-026/SPEC-004.
	ZoneLearner int
	ZoneApp     int
}

func (r *WeeklyResult) add(o WeeklyResult) {
	r.Written += o.Written
	r.Skipped += o.Skipped
	r.NotYet += o.NotYet
	r.Gained += o.Gained
	r.Lost += o.Lost
	r.Cleared += o.Cleared
	r.BandSlips += o.BandSlips
}

// Weekly is the weekly report job. It shares the daily job's store, clock, time zone, log,
// and lock, so the two never run at once and a fake clock moves both.
type Weekly struct {
	d *Daily
}

// NewWeekly builds the job on a daily job.
func NewWeekly(d *Daily) *Weekly { return &Weekly{d: d} }

// ParseWeek checks a week start: a real day, written YYYY-MM-DD, that is a Monday.
func ParseWeek(s string) (string, error) {
	t, err := time.Parse(dayLayout, s)
	if err != nil || t.Format(dayLayout) != s {
		return "", fmt.Errorf("jobs: week %q is not a date as YYYY-MM-DD", s)
	}
	if t.Weekday() != WeeklyDay {
		return "", fmt.Errorf("%w: %s is a %s", ErrNotMonday, s, t.Weekday())
	}
	return s, nil
}

// dueWeekIn is the last full Monday–Sunday in z that has ended: the Monday of the week before
// the learner's current week. Written on the first sweep after that following Monday starts.
// INIT-026/SPEC-004.
func dueWeekIn(now time.Time, z studyday.Zone) string {
	local := now.In(z.Loc)
	offset := int((local.Weekday() + 6) % 7) // days since Monday
	thisMonday := time.Date(local.Year(), local.Month(), local.Day()-offset, 0, 0, 0, 0, z.Loc)
	return thisMonday.AddDate(0, 0, -7).Format(dayLayout)
}

// DueWeek is the last finished week in the app zone. Prefer CatchUp, which uses each
// learner's own zone. INIT-026/SPEC-004.
func (w *Weekly) DueWeek() string {
	return dueWeekIn(w.d.clock.Now(), w.d.app)
}

// NextWeekly is the first Monday 04:00 after now in loc. Kept for tests; the scheduler is an
// hourly sweep and does not call it. INIT-026/SPEC-004.
func NextWeekly(now time.Time, loc *time.Location) time.Time {
	local := now.In(loc)
	days := (int(WeeklyDay) - int(local.Weekday()) + 7) % 7
	next := time.Date(local.Year(), local.Month(), local.Day()+days, WeeklyHour, WeeklyMinute, 0, 0, loc)
	if !next.After(now) {
		next = time.Date(local.Year(), local.Month(), local.Day()+days+7, WeeklyHour, WeeklyMinute, 0, 0, loc)
	}
	return next
}

// CatchUp writes each learner's last finished week in their own zone when they have none. It
// is what the scheduler runs at each hourly sweep after the daily catch-up. Only that week is
// written: an older week cannot be recomputed (the package doc says why). INIT-026/SPEC-004.
func (w *Weekly) CatchUp(ctx context.Context) (WeeklyResult, error) {
	var res WeeklyResult
	err := w.d.locked(ctx, func() error {
		ids, err := w.d.store.UserIDs(ctx)
		if err != nil {
			return err
		}
		zones, err := w.d.resolver.All(ctx)
		if err != nil {
			return err
		}
		res.Learners = len(ids)
		res.ZoneLearner, res.ZoneApp = countZoneSources(zones)
		var errs []error
		now := w.d.clock.Now()
		weeks := map[string]int{}
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			z := zones[id]
			week := dueWeekIn(now, z)
			weeks[week]++
			part, err := w.learnerWeek(ctx, id, week, z)
			res.add(part)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				errs = append(errs, fmt.Errorf("learner %d: %w", id, err))
			}
		}
		if len(weeks) == 1 {
			for week := range weeks {
				res.Week = week
			}
		}
		return errors.Join(errs...)
	})
	return res, err
}

// Run writes the report of one finished week for every learner. weekStart is the Monday. A
// learner that already has a report for the week is skipped, so running a week twice changes
// nothing. A learner for whom the week is still open is skipped and counted in NotYet.
// ErrWeekOpen is returned only when the week is open for every learner. A learner whose write
// fails does not stop the others: the error is returned at the end, joined, and the next run
// retries only the learners that have no report. Run returns ErrLocked when another process
// holds the lock. INIT-026/SPEC-004.
func (w *Weekly) Run(ctx context.Context, weekStart string) (WeeklyResult, error) {
	res := WeeklyResult{Week: weekStart}
	if _, err := ParseWeek(weekStart); err != nil {
		return res, err
	}
	err := w.d.locked(ctx, func() error {
		ids, err := w.d.store.UserIDs(ctx)
		if err != nil {
			return err
		}
		zones, err := w.d.resolver.All(ctx)
		if err != nil {
			return err
		}
		res.Learners = len(ids)
		res.ZoneLearner, res.ZoneApp = countZoneSources(zones)
		var errs []error
		now := w.d.clock.Now()
		end := addDays(weekStart, 6)
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			z := zones[id]
			today := studyday.Today(now, z)
			if end >= today {
				res.NotYet++
				continue
			}
			part, err := w.learnerWeek(ctx, id, weekStart, z)
			res.add(part)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				errs = append(errs, fmt.Errorf("learner %d: %w", id, err))
			}
		}
		if res.Learners > 0 && res.NotYet == res.Learners {
			return fmt.Errorf("%w: %s runs to %s", ErrWeekOpen, weekStart, end)
		}
		if res.Learners == 0 && end >= studyday.Today(now, w.d.app) {
			return fmt.Errorf("%w: %s runs to %s", ErrWeekOpen, weekStart, end)
		}
		return errors.Join(errs...)
	})
	return res, err
}

// WordSkill is one word in one skill, with the states it moved between.
type WordSkill struct {
	LexemeID int64  `json:"lexemeId"`
	Text     string `json:"text"`
	Skill    string `json:"skill"`
	From     string `json:"from"`
	To       string `json:"to"`
}

// LostWord is a word that fell out of known during the week, from a regression event of the
// daily job. Cause is the diagnosis the job stored with the event. Recovered is true when the
// word is known again in that skill in the states the report compares.
type LostWord struct {
	WordSkill
	Day       string `json:"day"`
	Cause     string `json:"cause"`
	Recovered bool   `json:"recovered"`
}

// BandSlipEvent is a placement band whose known share fell during the week.
type BandSlipEvent struct {
	Day    string `json:"day"`
	Band   int    `json:"band"`
	Detail string `json:"detail"`
}

// RetentionStat is the retention of the newest snapshot of the week against the target. Actual
// is null when the week has no snapshot with enough data.
type RetentionStat struct {
	Target float64  `json:"target"`
	Actual *float64 `json:"actual"`
	Gap    *float64 `json:"gap"`
	Day    string   `json:"day,omitempty"`
}

// WeeklyPayload is weekly_reports.payload_json.
type WeeklyPayload struct {
	Version int `json:"version"`
	// WeekStart is the Monday, and WeekEnd the Sunday, both days in Zone.
	WeekStart string `json:"weekStart"`
	WeekEnd   string `json:"weekEnd"`
	Zone      string `json:"zone"`
	// StateDay is the day of the word states the report ends at. It is empty when the daily
	// job has not stored any for the learner.
	StateDay string `json:"stateDay"`
	// BaselineWeek is the week of the earlier states that gains and cleared words are measured
	// from. It is empty on a first report, and then Gained and Cleared are empty because there is
	// nothing to measure from, and BaselineKnown is false.
	BaselineWeek  string          `json:"baselineWeek"`
	BaselineKnown bool            `json:"baselineKnown"`
	Gained        []WordSkill     `json:"gained"`
	Lost          []LostWord      `json:"lost"`
	Cleared       []WordSkill     `json:"cleared"`
	Retention     RetentionStat   `json:"retention"`
	Minutes       int             `json:"minutesStudied"`
	BandSlips     []BandSlipEvent `json:"bandSlips"`
	// Course is the common-word course for this week. It is absent when the learner
	// has no course word with a card.
	Course *CourseWeek `json:"course,omitempty"`
	// Readout is the five-line lesson note. It says too early while counts are small.
	Readout []string `json:"readout,omitempty"`
}

// CourseWeek is the common-word course in one weekly report.
type CourseWeek struct {
	Place       int         `json:"place"`
	Met         int         `json:"met"`
	MetThisWeek int         `json:"metThisWeek"`
	Gained      []WordSkill `json:"gained"`
	Lost        []LostWord  `json:"lost"`
	StudyDays   int         `json:"studyDays"`
}

// weeklyBaseline is the job_state row weekly.words.<user>: each word's four skill states at the
// end of the newest week reported, in the same letters as daily.words.<user>.
type weeklyBaseline struct {
	Week  string            `json:"week"`
	Day   string            `json:"day"`
	Words map[string]string `json:"w"`
}

func baselineKey(userID int64) string { return weeklyBaselineKey + strconv.FormatInt(userID, 10) }

// weekWindow is the instants a week covers in the server time zone: from the start of Monday
// to the start of the next Monday. Built from calendar days, so a week with a clock change is
// 167 or 169 hours long and still holds exactly its seven days.
func weekWindow(weekStart string, loc *time.Location) (from, to time.Time) {
	t, _ := time.Parse(dayLayout, weekStart)
	from = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
	to = time.Date(t.Year(), t.Month(), t.Day()+7, 0, 0, 0, 0, loc)
	return from, to
}

// learnerWeek writes one learner's report. The reads are done first, because the store has one
// connection and a transaction waits for no other call. The transaction checks again that the
// week has no report, writes it, and moves the baseline.
func (w *Weekly) learnerWeek(ctx context.Context, userID int64, weekStart string, z studyday.Zone) (WeeklyResult, error) {
	var res WeeklyResult
	has, err := w.d.store.HasWeeklyReport(ctx, userID, weekStart)
	if err != nil {
		return res, err
	}
	if has {
		res.Skipped++
		return res, nil
	}
	w.d.heartbeat(ctx)
	in, err := w.read(ctx, userID, weekStart, z)
	if err != nil {
		return res, err
	}
	payload := buildWeekly(in)
	body, err := json.Marshal(payload)
	if err != nil {
		return res, fmt.Errorf("weekly job: encode payload: %w", err)
	}
	report := db.WeeklyReport{
		UserID: userID, WeekStart: weekStart, PayloadJSON: string(body),
		Summary: summarize(payload), CreatedAt: w.d.clock.Now(),
	}
	var newBase []byte
	// The baseline only moves forward. A report for a week older than the stored baseline
	// (a back-fill) leaves it alone, and so does a learner with no stored word states.
	if in.current != nil && (in.baseWeek == "" || in.baseWeek < weekStart) {
		newBase, err = json.Marshal(weeklyBaseline{Week: weekStart, Day: in.current.Day, Words: in.current.Words})
		if err != nil {
			return res, fmt.Errorf("weekly job: encode baseline: %w", err)
		}
	}
	err = w.d.store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		res = WeeklyResult{}
		dup, err := tx.HasWeeklyReport(ctx, userID, weekStart)
		if err != nil {
			return err
		}
		if dup {
			res.Skipped++
			return nil
		}
		if err := tx.InsertWeeklyReport(ctx, report); err != nil {
			return err
		}
		if newBase != nil {
			if err := tx.SetJobState(ctx, baselineKey(userID), string(newBase), w.d.clock.Now()); err != nil {
				return err
			}
		}
		res.Written = 1
		res.Gained, res.Lost, res.Cleared = distinct(payload.Gained), distinctLost(payload.Lost), distinct(payload.Cleared)
		res.BandSlips = len(payload.BandSlips)
		return nil
	})
	if err != nil {
		return WeeklyResult{}, err
	}
	return res, nil
}

// weekInput is everything a report is made from. buildWeekly is a pure function of it.
type weekInput struct {
	weekStart string
	zoneName  string
	loc       *time.Location
	snapshots []db.SnapshotRow
	events    []db.WeekEvent
	attempts  []db.AttemptTime
	current   *wordsState
	// base is the baseline to measure from, or nil when there is none that can be used.
	base *weeklyBaseline
	// baseWeek is the week of the stored baseline, usable or not. It is empty when none is stored.
	baseWeek string
	texts    map[int64]string
	course   map[int64]int
	cards    map[int64]time.Time
	lesson   lessons.Week
}

// read gathers the inputs of one learner and week in that learner's zone. INIT-026/SPEC-004.
func (w *Weekly) read(ctx context.Context, userID int64, weekStart string, z studyday.Zone) (weekInput, error) {
	in := weekInput{weekStart: weekStart, zoneName: z.Name, loc: z.Loc}
	weekEnd := addDays(weekStart, 6)
	var err error
	if in.snapshots, err = w.d.store.SnapshotRows(ctx, userID, weekStart, weekEnd); err != nil {
		return in, err
	}
	if in.events, err = w.d.store.WeekEvents(ctx, userID, weekStart, weekEnd); err != nil {
		return in, err
	}
	from, to := weekWindow(weekStart, z.Loc)
	if in.attempts, err = w.d.store.AttemptTimes(ctx, userID, from, to); err != nil {
		return in, err
	}
	if in.current, err = w.readWords(ctx, userID); err != nil {
		return in, err
	}
	if in.base, err = w.readBaseline(ctx, userID); err != nil {
		return in, err
	}
	if in.base != nil {
		in.baseWeek = in.base.Week
	}
	if in.base != nil && in.base.Week >= weekStart {
		// A report for an older week than the baseline. The baseline is newer than the week, so
		// it cannot be the week's start. There is nothing to measure from.
		in.base = nil
	}
	if in.base != nil && in.current != nil {
		ids := changedIDs(in.base, in.current)
		found, err := w.d.store.LexemeTexts(ctx, ids)
		if err != nil {
			return in, err
		}
		in.texts = make(map[int64]string, len(found))
		for id, l := range found {
			in.texts[id] = l.TextNFC
		}
	}
	courseRows, err := w.d.store.CourseLexemes(ctx)
	if err != nil {
		return in, err
	}
	in.course = map[int64]int{}
	for _, row := range courseRows {
		in.course[row.LexemeID] = row.Order
	}
	in.cards, err = w.d.store.CourseCardFirstCreated(ctx, userID)
	if err != nil {
		return in, err
	}
	counts, err := w.d.store.LessonWeekCounts(ctx, userID, from, to)
	if err != nil {
		return in, err
	}
	in.lesson = lessons.Week{
		Offered: counts.Started, Started: counts.Started, Finished: counts.Finished,
		Flags: counts.Flags, Answers: counts.Answers, Right: counts.Right, Voids: counts.Voids,
		LaterDayRight: counts.LaterRight, LaterDayN: counts.LaterN,
	}
	return in, nil
}

func (w *Weekly) readWords(ctx context.Context, userID int64) (*wordsState, error) {
	raw, ok, err := w.d.store.JobState(ctx, wordsKey(userID))
	if err != nil || !ok {
		return nil, err
	}
	var s wordsState
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		w.d.log.Warn("weekly job: unreadable word states, reporting without them")
		return nil, nil
	}
	return &s, nil
}

func (w *Weekly) readBaseline(ctx context.Context, userID int64) (*weeklyBaseline, error) {
	raw, ok, err := w.d.store.JobState(ctx, baselineKey(userID))
	if err != nil || !ok {
		return nil, err
	}
	var b weeklyBaseline
	if err := json.Unmarshal([]byte(raw), &b); err != nil || b.Week == "" {
		w.d.log.Warn("weekly job: unreadable baseline, starting over")
		return nil, nil
	}
	return &b, nil
}

// changedIDs lists the lexeme ids whose states differ between the two maps, which is a
// superset of the gained and cleared words. Only those need their text read.
func changedIDs(base *weeklyBaseline, cur *wordsState) []int64 {
	var ids []int64
	for key, code := range cur.Words {
		if base.Words[key] == code {
			continue
		}
		if id, err := strconv.ParseInt(key, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// buildWeekly makes the payload. It reads no clock and no database.
func buildWeekly(in weekInput) WeeklyPayload {
	zone := in.zoneName
	if zone == "" && in.loc != nil {
		zone = in.loc.String()
	}
	p := WeeklyPayload{
		Version: payloadVersion, WeekStart: in.weekStart, WeekEnd: addDays(in.weekStart, 6), Zone: zone,
		Gained: []WordSkill{}, Lost: []LostWord{}, Cleared: []WordSkill{}, BandSlips: []BandSlipEvent{},
		Retention: retentionOf(in.snapshots),
		Minutes:   ActiveMinutes(in.attempts),
	}
	if in.current != nil {
		p.StateDay = in.current.Day
	}
	if in.base != nil && in.current != nil {
		p.BaselineWeek, p.BaselineKnown = in.base.Week, true
		p.Gained, p.Cleared = movedWords(in.base, in.current, in.texts)
	}
	for _, e := range in.events {
		switch e.Kind {
		case db.EventWord:
			l := LostWord{
				WordSkill: WordSkill{LexemeID: e.LexemeID, Text: e.Text, Skill: e.Skill, From: e.FromState, To: e.ToState},
				Day:       e.Day, Cause: e.Cause,
			}
			if in.current != nil {
				l.Recovered = knownNow(in.current, e.LexemeID, e.Skill)
			}
			p.Lost = append(p.Lost, l)
		case db.EventBandSlip:
			band := -1
			if _, err := fmt.Sscanf(e.Cause, "band_slip band=%d ", &band); err != nil {
				band = -1
			}
			p.BandSlips = append(p.BandSlips, BandSlipEvent{Day: e.Day, Band: band, Detail: e.Cause})
		}
	}
	if len(in.cards) > 0 {
		cw := &CourseWeek{Gained: []WordSkill{}, Lost: []LostWord{}}
		loc := in.loc
		if loc == nil {
			loc = time.UTC
		}
		from, to := weekWindow(in.weekStart, loc)
		for id := range in.cards {
			cw.Met++
			if order := in.course[id]; order > cw.Place {
				cw.Place = order
			}
			if at := in.cards[id]; !at.Before(from) && at.Before(to) {
				cw.MetThisWeek++
			}
		}
		for _, g := range p.Gained {
			if _, ok := in.course[g.LexemeID]; ok {
				cw.Gained = append(cw.Gained, g)
			}
		}
		for _, l := range p.Lost {
			if _, ok := in.course[l.LexemeID]; ok {
				cw.Lost = append(cw.Lost, l)
			}
		}
		cw.StudyDays = studyDays(in.attempts, in.loc)
		p.Course = cw
	}
	note := in.lesson
	for id := range in.course {
		if _, ok := in.cards[id]; !ok {
			note.Pool++
		}
	}
	note.HeavyDays = heavyDays(in.attempts, in.loc)
	p.Readout = lessons.Readout(note)
	return p
}

// heavyDayMinutes is the review cap in ADR-0034 D5. A day over it is a heavy day.
const heavyDayMinutes = 15

func heavyDays(attempts []db.AttemptTime, loc *time.Location) int {
	by := map[string][]db.AttemptTime{}
	for _, a := range attempts {
		at := a.At
		if loc != nil {
			at = at.In(loc)
		}
		day := at.Format("2006-01-02")
		by[day] = append(by[day], a)
	}
	n := 0
	for _, day := range by {
		if ActiveMinutes(day) > heavyDayMinutes {
			n++
		}
	}
	return n
}

func studyDays(attempts []db.AttemptTime, loc *time.Location) int {
	seen := map[string]bool{}
	for _, a := range attempts {
		at := a.At
		if loc != nil {
			at = at.In(loc)
		}
		seen[at.Format("2006-01-02")] = true
	}
	return len(seen)
}

func knownNow(cur *wordsState, lexeme int64, skill string) bool {
	code, ok := cur.Words[strconv.FormatInt(lexeme, 10)]
	if !ok {
		return false
	}
	i := slices.Index(learner.Skills, skill)
	return i >= 0 && i < len(code) && knownCode(code[i])
}

// movedWords compares two word maps. A word gained is not known in a skill at the baseline,
// with no entry meaning not known, and known or solid now. A struggle word cleared was
// struggling in a skill at the baseline and is neither struggling nor regressed now. Both are
// listed by word and then skill order.
func movedWords(base *weeklyBaseline, cur *wordsState, texts map[int64]string) (gained, cleared []WordSkill) {
	gained, cleared = []WordSkill{}, []WordSkill{}
	type pair struct {
		id   int64
		code string
	}
	var words []pair
	for key, code := range cur.Words {
		if id, err := strconv.ParseInt(key, 10, 64); err == nil {
			words = append(words, pair{id, code})
		}
	}
	sort.Slice(words, func(i, j int) bool { return words[i].id < words[j].id })
	for _, w := range words {
		before := base.Words[strconv.FormatInt(w.id, 10)]
		for i, skill := range learner.Skills {
			if i >= len(w.code) {
				break
			}
			was := byte('u')
			if i < len(before) {
				was = before[i]
			}
			now := w.code[i]
			ws := WordSkill{LexemeID: w.id, Text: texts[w.id], Skill: skill, From: string(codeStates[was]), To: string(codeStates[now])}
			// Both can hold: a struggle word that is known now is a gain and a cleared word.
			if !knownCode(was) && knownCode(now) {
				gained = append(gained, ws)
			}
			if was == 't' && !weakCode(now) {
				cleared = append(cleared, ws)
			}
		}
	}
	return gained, cleared
}

// retentionOf is the mean of the skills' retention on the newest day of the week that has any,
// against the fixed target. The snapshot has one retention per skill, repeated on each state
// row, and no due counts, so the skills weigh the same. This is the figure get_progress gives.
func retentionOf(rows []db.SnapshotRow) RetentionStat {
	r := RetentionStat{Target: sessionlimits.TargetSuccess}
	perDay := map[string]map[string]float64{}
	for _, row := range rows {
		if row.Retention == nil {
			continue
		}
		if perDay[row.Day] == nil {
			perDay[row.Day] = map[string]float64{}
		}
		perDay[row.Day][row.Skill] = *row.Retention
	}
	if len(perDay) == 0 {
		return r
	}
	newest := ""
	for d := range perDay {
		if d > newest {
			newest = d
		}
	}
	var sum float64
	for _, v := range perDay[newest] {
		sum += v
	}
	actual := sum / float64(len(perDay[newest]))
	gap := actual - r.Target
	r.Actual, r.Gap, r.Day = &actual, &gap, newest
	return r
}

// ActiveMinutes is the time spent answering, in whole minutes. Two answers closer than IdleGap
// count for the time between them. An answer after a longer pause, and the first one, count for
// the time the learner took to give it, at most IdleGap. Reading a story, or any time with no
// answer, is not counted, and neither are flashcard grades that wrote no attempt.
// The weekly job and get_study_digest share this rule. INIT-026/SPEC-009.
func ActiveMinutes(attempts []db.AttemptTime) int {
	var total time.Duration
	for i, a := range attempts {
		if i > 0 {
			if gap := a.At.Sub(attempts[i-1].At); gap <= IdleGap {
				total += gap
				continue
			}
		}
		total += min(time.Duration(a.LatencyMS)*time.Millisecond, IdleGap)
	}
	return int((total + 30*time.Second) / time.Minute)
}

func distinct(ws []WordSkill) int {
	seen := map[int64]bool{}
	for _, w := range ws {
		seen[w.LexemeID] = true
	}
	return len(seen)
}

func distinctLost(ls []LostWord) int {
	seen := map[int64]bool{}
	for _, l := range ls {
		seen[l.LexemeID] = true
	}
	return len(seen)
}

// causePhrases say a cause in plain words, after "mostly from".
var causePhrases = map[string]string{
	string(learner.CauseDecoding):   "reading the letters",
	string(learner.CauseProduction): "recalling the word",
	string(learner.CauseSpelling):   "spelling",
	string(learner.CauseConfusion):  "mixing it up with another word",
	string(learner.CauseBandSlip):   "a whole group of words slipping",
	UnknownCause:                    "reasons the app could not name",
}

// topCause is the cause behind the most lost words. Causes the app named win over the one it
// could not, and a tie goes to the cause that sorts first. It is empty when nothing was lost.
func topCause(lost []LostWord) string {
	words := map[string]map[int64]bool{}
	for _, l := range lost {
		if words[l.Cause] == nil {
			words[l.Cause] = map[int64]bool{}
		}
		words[l.Cause][l.LexemeID] = true
	}
	best, bestN := "", 0
	for _, named := range []bool{true, false} {
		keys := make([]string, 0, len(words))
		for c := range words {
			if (c != UnknownCause) == named {
				keys = append(keys, c)
			}
		}
		sort.Strings(keys)
		for _, c := range keys {
			if n := len(words[c]); n > bestN {
				best, bestN = c, n
			}
		}
		if best != "" {
			break
		}
	}
	return best
}

func count(n int, one, many string) string {
	switch n {
	case 0:
		return "no " + many
	case 1:
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// summarize is the three sentences of the report, from templates. It calls no model. Each
// sentence is short and has one full stop, at its end, so a reader (and a test) can count them.
func summarize(p WeeklyPayload) string {
	lostN := distinctLost(p.Lost)
	lost := count(lostN, "word", "words")
	because := ""
	if c := topCause(p.Lost); c != "" {
		phrase, ok := causePhrases[c]
		if !ok {
			phrase = "reasons the app could not name"
		}
		because = ", mostly from " + phrase
	}
	var first string
	if p.BaselineKnown {
		first = fmt.Sprintf("You gained %s and lost %s%s.", count(distinct(p.Gained), "word", "words"), lost, because)
	} else {
		first = fmt.Sprintf("You lost %s%s, and this first report does not count gains.", lost, because)
	}

	retention := "retention was not measured"
	if p.Retention.Actual != nil {
		actual := int(*p.Retention.Actual*100 + 0.5)
		target := int(p.Retention.Target*100 + 0.5)
		switch gap := actual - target; {
		case gap < 0:
			retention = fmt.Sprintf("retention was %d%%, %d points under the %d%% target", actual, -gap, target)
		case gap > 0:
			retention = fmt.Sprintf("retention was %d%%, %d points over the %d%% target", actual, gap, target)
		default:
			retention = fmt.Sprintf("retention was %d%%, right on the target", actual)
		}
	}
	var second string
	if p.BaselineKnown {
		cleared := count(distinct(p.Cleared), "struggle word cleared", "struggle words cleared")
		second = strings.ToUpper(cleared[:1]) + cleared[1:] + ", and " + retention + "."
	} else {
		second = strings.ToUpper(retention[:1]) + retention[1:] + "."
	}

	minutes := count(p.Minutes, "minute", "minutes")
	if p.Minutes == 0 {
		minutes = "0 minutes"
	}
	slips := fmt.Sprintf("%s slipped", count(len(p.BandSlips), "band", "bands"))
	third := fmt.Sprintf("You studied for %s, and %s.", minutes, slips)
	return first + " " + second + " " + third
}
