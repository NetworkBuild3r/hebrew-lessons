package jobs

import (
	"context"
	"database/sql"
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
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
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

	// RuleVersion is the learner-model stamp on daily_snapshots and daily_totals written
	// by the daily job and by hebrew history rebuild (ADR-0028 D3 / INIT-033/SPEC-004).
	RuleVersion = 2

	// DailyLockKey is the job_state key for the daily job and history rebuild lock.
	DailyLockKey = "daily.lock"

	// bandHistoryDays is how many days of band shares are kept.
	bandHistoryDays = 14

	dayLayout = "2006-01-02"
)

// ErrLocked means another process holds the daily job lock.
var ErrLocked = errors.New("jobs: the daily job is running elsewhere")

// ErrFutureDay means the day has not started yet for every learner.
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
	// NotYet is how many learners were skipped because the day has not ended in their zone.
	NotYet int
	// Events is how many word regression events were written.
	Events int
	// BandSlips is how many band_slip events were written.
	BandSlips int
	// ZoneLearner and ZoneApp count learners by studyday.Zone.Source. INIT-026/SPEC-004.
	ZoneLearner int
	ZoneApp     int
}

func (r *Result) add(o Result) {
	r.Written += o.Written
	r.Skipped += o.Skipped
	r.NotYet += o.NotYet
	r.Events += o.Events
	r.BandSlips += o.BandSlips
}

// Daily is the daily snapshot job.
type Daily struct {
	store    *db.Store
	model    *learner.Model
	clock    clock.Clock
	resolver *studyday.Resolver
	app      studyday.Zone // app fallback zone the resolver was built with
	owner    string
	log      *slog.Logger
}

// Option changes how NewDaily builds a Daily.
type Option func(*options)

type options struct {
	owner   string
	log     *slog.Logger
	learner []learner.Option
	appLoc  *time.Location
}

// WithOwner names this process in the lock. The default is the host name and process id.
func WithOwner(owner string) Option { return func(o *options) { o.owner = owner } }

// WithLogger sets where the job logs. The default is slog.Default.
func WithLogger(l *slog.Logger) Option { return func(o *options) { o.log = l } }

// WithLearnerOptions passes options to the learner model, for example where bands come from.
func WithLearnerOptions(opts ...learner.Option) Option {
	return func(o *options) { o.learner = append(o.learner, opts...) }
}

// WithAppZone records the app zone the resolver was built with. Required so Today and the
// weekly DueWeek default share that zone when every learner has a personal zone.
// INIT-026/SPEC-004.
func WithAppZone(loc *time.Location) Option {
	return func(o *options) { o.appLoc = loc }
}

// NewDaily builds the job. clk is the process clock and also the clock of the learner model,
// so a snapshot and the state it holds agree on "now". resolver decides each learner's day.
// A nil resolver fails to build. Pass WithAppZone with the same location given to
// studyday.NewResolver. INIT-026/SPEC-004.
func NewDaily(store *db.Store, clk clock.Clock, resolver *studyday.Resolver, opts ...Option) *Daily {
	if resolver == nil {
		panic("jobs: NewDaily requires a studyday.Resolver")
	}
	o := options{}
	for _, f := range opts {
		f(&o)
	}
	if o.appLoc == nil {
		panic("jobs: NewDaily requires WithAppZone")
	}
	if clk == nil && store != nil {
		clk = store.Clock()
	}
	if o.owner == "" {
		host, _ := os.Hostname()
		o.owner = host + "-" + strconv.Itoa(os.Getpid())
	}
	if o.log == nil {
		o.log = slog.Default()
	}
	return &Daily{
		store:    store,
		model:    learner.New(store, clk, o.learner...),
		clock:    clk,
		resolver: resolver,
		app:      studyday.Zone{Name: o.appLoc.String(), Loc: o.appLoc, Source: studyday.SourceApp},
		owner:    o.owner,
		log:      o.log,
	}
}

// Today is the current day in the app zone, as YYYY-MM-DD. Learner-day work uses each
// learner's own zone via the resolver. INIT-026/SPEC-004.
func (d *Daily) Today() string {
	return studyday.Today(d.clock.Now(), d.app)
}

// Yesterday is the day before Today in the app zone.
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
// the day is skipped, so running a day twice changes nothing. A learner for whom the day has
// not ended yet (midnight after it has not come in their zone) is skipped and counted in
// NotYet, so an unfinished day is never frozen. ErrFutureDay is returned only when the day
// has not ended for every learner.
// A learner whose write fails does not stop the others: the error is returned at the end,
// joined, and the next run retries only the learners that have no rows. Run returns
// ErrLocked when another process holds the lock. INIT-026/SPEC-004.
func (d *Daily) Run(ctx context.Context, day string) (Result, error) {
	if _, err := ParseDay(day); err != nil {
		return Result{}, err
	}
	res := Result{Day: day}
	err := d.locked(ctx, func() error {
		if err := d.replayListenOnce(ctx); err != nil {
			return err
		}
		ids, err := d.store.UserIDs(ctx)
		if err != nil {
			return err
		}
		zones, err := d.resolver.All(ctx)
		if err != nil {
			return err
		}
		res.Learners = len(ids)
		res.ZoneLearner, res.ZoneApp = countZoneSources(zones)
		var errs []error
		now := d.clock.Now()
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			z := zones[id]
			if end, err := EndOfDay(day, z); err != nil {
				return err
			} else if end.After(now) {
				res.NotYet++
				continue
			}
			part, err := d.learnerDays(ctx, id, []string{day}, z)
			res.add(part)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				errs = append(errs, fmt.Errorf("learner %d: %w", id, err))
			}
		}
		if res.Learners > 0 && res.NotYet == res.Learners {
			return fmt.Errorf("%w: %s", ErrFutureDay, day)
		}
		if res.Learners == 0 && day >= studyday.Today(now, d.app) {
			return fmt.Errorf("%w: %s", ErrFutureDay, day)
		}
		return errors.Join(errs...)
	})
	return res, err
}

// CatchUp fills the days each learner is missing, from the day after their newest snapshot
// through yesterday in that learner's zone, at most MaxCatchUpDays of the newest. A learner
// with no snapshot gets yesterday only. It is what the scheduler runs at start and on each
// hourly sweep, so a restart after idle days fills every one of them. It returns ErrLocked
// when another process holds the lock. INIT-026/SPEC-004.
func (d *Daily) CatchUp(ctx context.Context) (Result, error) {
	var res Result
	err := d.locked(ctx, func() error {
		if err := d.replayListenOnce(ctx); err != nil {
			return err
		}
		ids, err := d.store.UserIDs(ctx)
		if err != nil {
			return err
		}
		zones, err := d.resolver.All(ctx)
		if err != nil {
			return err
		}
		last, err := d.store.LastSnapshotDays(ctx)
		if err != nil {
			return err
		}
		res.Learners = len(ids)
		res.ZoneLearner, res.ZoneApp = countZoneSources(zones)
		var errs []error
		now := d.clock.Now()
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return err
			}
			z := zones[id]
			yesterday := addDays(studyday.Today(now, z), -1)
			days := missedDays(last[id], yesterday)
			if len(days) == 0 {
				continue
			}
			part, err := d.learnerDays(ctx, id, days, z)
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

func countZoneSources(zones map[int64]studyday.Zone) (learnerN, appN int) {
	for _, z := range zones {
		switch z.Source {
		case studyday.SourceLearner:
			learnerN++
		default:
			appN++
		}
	}
	return learnerN, appN
}

// replayListenOnce runs the one-time listen-card replay from INIT-023/SPEC-003.
// It logs a dry-run count, writes cards for past listen answers that have none,
// and records completion under evidence.ListenReplayKey so a second run is a no-op.
func (d *Daily) replayListenOnce(ctx context.Context) error {
	_, done, err := d.store.JobState(ctx, evidence.ListenReplayKey)
	if err != nil {
		return err
	}
	if done {
		return nil
	}
	dry, err := evidence.ReplayListenCards(ctx, d.store, nil, true)
	if err != nil {
		return err
	}
	d.log.Info("listen replay dry-run", "targets", dry.Targets, "cards", dry.Written)
	res, err := evidence.ReplayListenCards(ctx, d.store, nil, false)
	if err != nil {
		return err
	}
	d.log.Info("listen replay done", "targets", res.Targets, "written", res.Written)
	return d.store.InDailyTx(ctx, func(tx *db.DailyTx) error {
		return tx.SetJobState(ctx, evidence.ListenReplayKey, strconv.Itoa(res.Written), d.clock.Now())
	})
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
	ok, err := d.store.TryLockJob(ctx, DailyLockKey, d.owner, d.clock.Now(), LockTTL)
	if err != nil {
		return err
	}
	if !ok {
		return ErrLocked
	}
	defer func() {
		rel, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := d.store.UnlockJob(rel, DailyLockKey, d.owner); err != nil {
			d.log.Warn("daily job: release lock", "err", err)
		}
	}()
	return fn()
}

// heartbeat refreshes the lock so a long run is not taken over.
func (d *Daily) heartbeat(ctx context.Context) {
	if ok, err := d.store.TryLockJob(ctx, DailyLockKey, d.owner, d.clock.Now(), LockTTL); err != nil || !ok {
		d.log.Warn("daily job: refresh lock", "held", ok, "err", err)
	}
}

// EndOfDay is the instant day ends in z: midnight that starts the next calendar
// day, the same boundary studyday.Today uses. The 4:00 review cutoff is not a day
// boundary. day must be YYYY-MM-DD. DST-safe via time.Date in z.Loc.
// INIT-033/SPEC-004 / ADR-0028 D3.
func EndOfDay(day string, z studyday.Zone) (time.Time, error) {
	if z.Loc == nil {
		return time.Time{}, errors.New("jobs: EndOfDay requires a zone location")
	}
	t, err := time.ParseInLocation(dayLayout, day, z.Loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("jobs: end of day %q: %w", day, err)
	}
	y, m, d := t.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, z.Loc), nil
}

// DailyLockHolder reports whether a live process holds the daily lock (within LockTTL)
// and names that holder. A missing or stale lock is not held. INIT-033/SPEC-004.
func DailyLockHolder(ctx context.Context, store *db.Store, now time.Time) (holder string, held bool, err error) {
	conn, err := store.DB()
	if err != nil {
		return "", false, err
	}
	var updated string
	err = conn.QueryRowContext(ctx, `SELECT value, updated_at FROM job_state WHERE key = ?`, DailyLockKey).Scan(&holder, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("jobs: daily lock: %w", err)
	}
	at, perr := db.ParseTime(updated)
	if perr == nil && now.Sub(at) >= LockTTL {
		return holder, false, nil
	}
	return holder, true, nil
}

// learnerDays writes the given days for one learner, oldest first. Each missing day is
// evaluated as of EndOfDay in the learner's zone under the current rules (ADR-0028 D3).
func (d *Daily) learnerDays(ctx context.Context, userID int64, days []string, z studyday.Zone) (Result, error) {
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
	for _, day := range todo {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		d.heartbeat(ctx)
		at, err := EndOfDay(day, z)
		if err != nil {
			return res, err
		}
		// A day that has not ended is not history yet. Writing it now would freeze a
		// partial row that HasSnapshot then keeps.
		if at.After(d.clock.Now()) {
			res.Skipped++
			continue
		}
		snap, err := d.model.SnapshotAt(ctx, userID, at)
		if err != nil {
			return res, err
		}
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
		if err := tx.InsertTotals(ctx, userID, day, snap.Summary.KnownAnySkill, RuleVersion); err != nil {
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
// Rows are stamped with RuleVersion (ADR-0028 D3).
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
				Skill:       skill,
				State:       string(state),
				Count:       ss.Counts[state],
				EstKnown:    ss.EstimatedKnown,
				Retention:   ss.Retention,
				RuleVersion: RuleVersion,
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
