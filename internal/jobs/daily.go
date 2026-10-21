package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/clock"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
)

const (
	// MaxCatchUpDays is the most days CatchUp fills for one learner.
	MaxCatchUpDays = 14

	// BandSlipPoints is how many points of known share a band must lose, in BandSlipDays,
	// to count as a band slip.
	BandSlipPoints = 15
	// BandSlipDays is how far back a band's known share is compared.
	BandSlipDays = 7

	// LockTTL is how long a lock holder may be silent before another process takes the lock.
	LockTTL = 30 * time.Minute

	// UnknownCause is the cause of a regression when no diagnosis rule fits.
	UnknownCause = "unexplained"

	// bandHistoryDays is how many days of band shares are kept.
	bandHistoryDays = 14

	lockKey   = "daily.lock"
	dayLayout = "2006-01-02"
)

// ErrLocked means another process holds the daily job lock.
var ErrLocked = errors.New("jobs: the daily job is running elsewhere")

// ErrFutureDay means the day has not started yet in the server time zone.
var ErrFutureDay = errors.New("jobs: day is in the future")

// Result counts what a run did.
type Result struct {
	// Day is the day of a Run. It is empty for CatchUp, which fills several.
	Day string
	// Learners is how many learners were looked at.
	Learners int
	// Written is how many learner-days got snapshot rows.
	Written int
	// Skipped is how many learner-days already had rows.
	Skipped int
	// Events is how many word regression events were written.
	Events int
	// BandSlips is how many band_slip events were written.
	BandSlips int
}

func (r *Result) add(o Result) {
	r.Written += o.Written
	r.Skipped += o.Skipped
	r.Events += o.Events
	r.BandSlips += o.BandSlips
}

// Daily is the daily snapshot job.
type Daily struct {
	store *db.Store
	model *learner.Model
	clock clock.Clock
	loc   *time.Location
	owner string
	log   *slog.Logger
}

// Option changes how NewDaily builds a Daily.
type Option func(*options)

type options struct {
	owner   string
	log     *slog.Logger
	learner []learner.Option
}

// WithOwner names this process in the lock. The default is the host name and process id.
func WithOwner(owner string) Option { return func(o *options) { o.owner = owner } }

// WithLogger sets where the job logs. The default is slog.Default.
func WithLogger(l *slog.Logger) Option { return func(o *options) { o.log = l } }

// WithLearnerOptions passes options to the learner model, for example where bands come from.
func WithLearnerOptions(opts ...learner.Option) Option {
	return func(o *options) { o.learner = append(o.learner, opts...) }
}

// NewDaily builds the job. clk is the process clock and also the clock of the learner model,
// so a snapshot and the state it holds agree on "now". loc is the server time zone and
// decides where a day starts. A nil loc is UTC.
func NewDaily(store *db.Store, clk clock.Clock, loc *time.Location, opts ...Option) *Daily {
	o := options{}
	for _, f := range opts {
		f(&o)
	}
	if clk == nil && store != nil {
		clk = store.Clock()
	}
	if loc == nil {
		loc = time.UTC
	}
	if o.owner == "" {
		host, _ := os.Hostname()
		o.owner = host + "-" + strconv.Itoa(os.Getpid())
	}
	if o.log == nil {
		o.log = slog.Default()
	}
	return &Daily{
		store: store,
		model: learner.New(store, clk, o.learner...),
		clock: clk,
		loc:   loc,
		owner: o.owner,
		log:   o.log,
	}
}

// Today is the current day in the server time zone, as YYYY-MM-DD.
func (d *Daily) Today() string { return d.clock.Now().In(d.loc).Format(dayLayout) }

// Yesterday is the day before Today.
func (d *Daily) Yesterday() string { return addDays(d.Today(), -1) }

// ParseDay checks a YYYY-MM-DD day and returns it unchanged.
func ParseDay(s string) (string, error) {
	t, err := time.Parse(dayLayout, s)
	if err != nil || t.Format(dayLayout) != s {
		return "", fmt.Errorf("jobs: day %q is not a date as YYYY-MM-DD", s)
	}
	return s, nil
}

func addDays(day string, n int) string {
	t, err := time.Parse(dayLayout, day)
	if err != nil {
		return day
	}
	return t.AddDate(0, 0, n).Format(dayLayout)
}

// Run writes the snapshot of one day for every learner. A learner that already has rows for
// the day is skipped, so running a day twice changes nothing. A day after today is refused.
// A learner whose write fails does not stop the others: the error is returned at the end,
// joined, and the next run retries only the learners that have no rows. Run returns
// ErrLocked when another process holds the lock.
func (d *Daily) Run(ctx context.Context, day string) (Result, error) {
	if _, err := ParseDay(day); err != nil {
		return Result{}, err
	}
	if day > d.Today() {
		return Result{Day: day}, fmt.Errorf("%w: %s", ErrFutureDay, day)
	}
	res := Result{Day: day}
	err := d.locked(ctx, func() error {
		ids, err := d.store.UserIDs(ctx)
		if err != nil {
			return err
		}
		res.Learners = len(ids)
		var errs []error
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			part, err := d.learnerDays(ctx, id, []string{day})
			res.add(part)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				errs = append(errs, fmt.Errorf("learner %d: %w", id, err))
			}
		}
		return errors.Join(errs...)
	})
	return res, err
}

// CatchUp fills the days each learner is missing, from the day after their newest snapshot
// through yesterday, at most MaxCatchUpDays of the newest. A learner with no snapshot gets
// yesterday only. It is what the scheduler runs at start and at 04:30, so a restart after
// idle days fills every one of them. It returns ErrLocked when another process holds the lock.
func (d *Daily) CatchUp(ctx context.Context) (Result, error) {
	var res Result
	err := d.locked(ctx, func() error {
		yesterday := d.Yesterday()
		ids, err := d.store.UserIDs(ctx)
		if err != nil {
			return err
		}
		last, err := d.store.LastSnapshotDays(ctx)
		if err != nil {
			return err
		}
		res.Learners = len(ids)
		var errs []error
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			days := missedDays(last[id], yesterday)
			if len(days) == 0 {
				continue
			}
			part, err := d.learnerDays(ctx, id, days)
			res.add(part)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				errs = append(errs, fmt.Errorf("learner %d: %w", id, err))
			}
		}
		return errors.Join(errs...)
	})
	return res, err
}

// missedDays lists the days after last up to and including through, oldest first, keeping
// the newest MaxCatchUpDays. With no last day only through is returned.
func missedDays(last, through string) []string {
	if last == "" {
		return []string{through}
	}
	var days []string
	for day := addDays(last, 1); day <= through; day = addDays(day, 1) {
		days = append(days, day)
	}
	if len(days) > MaxCatchUpDays {
		days = days[len(days)-MaxCatchUpDays:]
	}
	return days
}

// locked runs fn holding the daily lock. The lock is released with a fresh context, so a
// cancelled run still lets go of it.
func (d *Daily) locked(ctx context.Context, fn func() error) error {
	ok, err := d.store.TryLockJob(ctx, lockKey, d.owner, d.clock.Now(), LockTTL)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLocked
	}
	defer func() {
		rel, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := d.store.UnlockJob(rel, lockKey, d.owner); err != nil {
			d.log.Warn("daily job: release lock", "err", err)
		}
	}()
	return fn()
}

// heartbeat refreshes the lock so a long run is not taken over.
func (d *Daily) heartbeat(ctx context.Context) {
	if ok, err := d.store.TryLockJob(ctx, lockKey, d.owner, d.clock.Now(), LockTTL); err != nil || !ok {
		d.log.Warn("daily job: refresh lock", "held", ok, "err", err)
	}
}

// learnerDays writes the given days for one learner, oldest first. The learner is evaluated
// once for all of them, and only when some day has no rows yet.
func (d *Daily) learnerDays(ctx context.Context, userID int64, days []string) (Result, error) {
	var res Result
	var todo []string
	for _, day := range days {
		has, err := d.store.HasSnapshot(ctx, userID, day)
		if err != nil {
			return res, err
		}
		if has {
			res.Skipped++
			continue
		}
		todo = append(todo, day)
	}
	if len(todo) == 0 {
		return res, nil
	}
	d.heartbeat(ctx)
	snap, err := d.model.Snapshot(ctx, userID)
	if err != nil {
		return res, err
	}
	for _, day := range todo {
		part, err := d.writeDay(ctx, userID, day, snap)
		res.add(part)
		if err != nil {
			return res, fmt.Errorf("day %s: %w", day, err)
		}
	}
	return res, nil
}

// wordsState is the daily.words.<user> row: each word's four skill states as of Day.
type wordsState struct {
	Day string `json:"day"`
	// Words maps a lexeme id to one letter per skill, in the order of learner.Skills.
	Words map[string]string `json:"w"`
}

// bandDay is the band shares of one day as words in the band and known words in it.
type bandDay [5][2]int

// bandsState is the daily.bands.<user> row: band shares by day.
type bandsState struct {
	Days map[string]bandDay `json:"days"`
}

var stateCodes = map[learner.State]byte{
	learner.Unseen: 'u', learner.Seen: 's', learner.Learning: 'l', learner.Struggling: 't',
	learner.Known: 'k', learner.Solid: 'o', learner.Regressed: 'r',
}

func knownCode(c byte) bool { return c == 'k' || c == 'o' }
func weakCode(c byte) bool  { return c == 't' || c == 'r' }

var codeStates = func() map[byte]learner.State {
	m := map[byte]learner.State{}
	for s, c := range stateCodes {
		m[c] = s
	}
	return m
}()

func wordsKey(userID int64) string { return "daily.words." + strconv.FormatInt(userID, 10) }
func bandsKey(userID int64) string { return "daily.bands." + strconv.FormatInt(userID, 10) }

func encodeWord(w learner.WordState) string {
	b := make([]byte, len(learner.Skills))
	for i, skill := range learner.Skills {
		b[i] = 'u'
		for _, sk := range w.Skills {
			if sk.Skill == skill {
				b[i] = stateCodes[sk.State]
				break
			}
		}
	}
	return string(b)
}

// writeDay writes one learner-day in one transaction.
func (d *Daily) writeDay(ctx context.Context, userID int64, day string, snap learner.Snapshot) (Result, error) {
	var res Result
	rows := snapshotRows(snap.Summary)
	err := d.store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		res = Result{}
		// Another process may have written the day since the check outside the transaction.
		has, err := tx.HasSnapshot(ctx, userID, day)
		if err != nil {
			return err
		}
		if has {
			res.Skipped++
			return nil
		}
		now := d.clock.Now()

		events, err := d.wordEvents(ctx, tx, userID, day, snap, now)
		if err != nil {
			return err
		}
		slips, err := d.bandSlips(ctx, tx, userID, day, snap, now)
		if err != nil {
			return err
		}
		if err := tx.InsertSnapshot(ctx, userID, day, rows); err != nil {
			return err
		}
		for _, e := range events {
			if err := tx.InsertRegression(ctx, e); err != nil {
				return err
			}
		}
		for _, e := range slips {
			if err := tx.InsertRegression(ctx, e); err != nil {
				return err
			}
		}
		res.Written, res.Events, res.BandSlips = 1, len(events), len(slips)
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return res, nil
}

// snapshotRows is one row per skill and state, so a learner always has 28 rows for a day.
func snapshotRows(s learner.Summary) []db.DailySnapshotRow {
	rows := make([]db.DailySnapshotRow, 0, len(learner.Skills)*len(learner.States))
	for _, skill := range learner.Skills {
		var ss learner.SkillSummary
		for _, c := range s.Skills {
			if c.Skill == skill {
				ss = c
			}
		}
		for _, state := range learner.States {
			rows = append(rows, db.DailySnapshotRow{
				Skill:     skill,
				State:     string(state),
				Count:     ss.Counts[state],
				EstKnown:  ss.EstimatedKnown,
				Retention: ss.Retention,
			})
		}
	}
	return rows
}

// wordEvents compares the learner's words with their states in the previous snapshot, and
// stores the new states. It returns the events to write. A day that is not newer than the
// stored states (a back-filled day) has nothing to compare, and leaves the stored states alone.
func (d *Daily) wordEvents(ctx context.Context, tx *db.DailyTx, userID int64, day string, snap learner.Snapshot, now time.Time) ([]db.RegressionEvent, error) {
	raw, ok, err := tx.JobState(ctx, wordsKey(userID))
	if err != nil {
		return nil, err
	}
	var prev wordsState
	if ok {
		if err := json.Unmarshal([]byte(raw), &prev); err != nil {
			// A row nobody can read is replaced. No events are made from it.
			d.log.Warn("daily job: unreadable word states, starting over", "user", userID, "err", err)
			prev = wordsState{}
		}
	}
	if prev.Day != "" && prev.Day >= day {
		return nil, nil
	}

	cur := wordsState{Day: day, Words: make(map[string]string, len(snap.Words))}
	ids := make([]int64, 0, len(snap.Words))
	for id := range snap.Words {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var events []db.RegressionEvent
	for _, id := range ids {
		w := snap.Words[id]
		key := strconv.FormatInt(id, 10)
		code := encodeWord(w)
		cur.Words[key] = code
		before, had := prev.Words[key]
		if !had || len(before) != len(code) {
			continue
		}
		for i, skill := range learner.Skills {
			if !knownCode(before[i]) || !weakCode(code[i]) {
				continue
			}
			cause := string(w.Cause)
			if cause == "" {
				cause = UnknownCause
			}
			events = append(events, db.RegressionEvent{
				UserID: userID, Kind: db.EventWord, LexemeID: id, Skill: skill,
				FromState: string(codeStates[before[i]]), ToState: string(codeStates[code[i]]),
				Day: day, Cause: cause,
			})
		}
	}
	out, err := json.Marshal(cur)
	if err != nil {
		return nil, fmt.Errorf("daily job: encode word states: %w", err)
	}
	if err := tx.SetJobState(ctx, wordsKey(userID), string(out), now); err != nil {
		return nil, err
	}
	return events, nil
}

// bandSlips records the band shares of the day and returns a band_slip event for each band
// whose known share fell BandSlipPoints or more since BandSlipDays earlier. A band that has
// a band_slip event inside the last BandSlipDays does not get another.
func (d *Daily) bandSlips(ctx context.Context, tx *db.DailyTx, userID int64, day string, snap learner.Snapshot, now time.Time) ([]db.RegressionEvent, error) {
	raw, ok, err := tx.JobState(ctx, bandsKey(userID))
	if err != nil {
		return nil, err
	}
	hist := bandsState{Days: map[string]bandDay{}}
	if ok {
		if err := json.Unmarshal([]byte(raw), &hist); err != nil || hist.Days == nil {
			d.log.Warn("daily job: unreadable band shares, starting over", "user", userID, "err", err)
			hist = bandsState{Days: map[string]bandDay{}}
		}
	}

	var cur bandDay
	for i, b := range snap.Bands {
		cur[i] = [2]int{b.Words, b.Known}
	}

	var events []db.RegressionEvent
	then := addDays(day, -BandSlipDays)
	if old, ok := hist.Days[then]; ok {
		for band := range cur {
			if !slipped(old[band], cur[band]) {
				continue
			}
			dup, err := tx.BandSlipSince(ctx, userID, band, then)
			if err != nil {
				return nil, err
			}
			if dup {
				continue
			}
			events = append(events, db.RegressionEvent{
				UserID: userID, Kind: db.EventBandSlip, Day: day,
				Cause: fmt.Sprintf("band_slip band=%d known share %s to %s since %s",
					band, percent(old[band]), percent(cur[band]), then),
			})
		}
	}

	hist.Days[day] = cur
	newest := day
	for k := range hist.Days {
		if k > newest {
			newest = k
		}
	}
	cut := addDays(newest, -bandHistoryDays)
	for k := range hist.Days {
		if k < cut {
			delete(hist.Days, k)
		}
	}
	out, err := json.Marshal(hist)
	if err != nil {
		return nil, fmt.Errorf("daily job: encode band shares: %w", err)
	}
	if err := tx.SetJobState(ctx, bandsKey(userID), string(out), now); err != nil {
		return nil, err
	}
	return events, nil
}

// slipped reports whether a band's known share fell BandSlipPoints or more. A band is
// {words, known}. The comparison is in whole numbers: known/words falls by at least
// 15/100 when 100*(oldKnown*newWords - newKnown*oldWords) >= 15*oldWords*newWords.
func slipped(old, cur [2]int) bool {
	if old[0] == 0 || cur[0] == 0 {
		return false
	}
	lhs := 100 * (old[1]*cur[0] - cur[1]*old[0])
	return lhs >= BandSlipPoints*old[0]*cur[0]
}

func percent(b [2]int) string {
	if b[0] == 0 {
		return "0%"
	}
	return strconv.Itoa(int(float64(b[1])/float64(b[0])*100+0.5)) + "%"
}
