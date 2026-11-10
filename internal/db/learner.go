package db

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner/rule"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

// This file is the query layer of internal/learner (INIT-015/SPEC-013). It reads
// attempts, exposures, cards, review_log, and confusion_misses and returns counts.
// The state rules are in internal/learner. Nothing here decides a state.
//
// The pool is one connection, so every query here finishes and closes its rows
// before the next one starts. Each query reads one learner by an indexed column:
// attempts_user_lexeme_skill, cards (user_id, lexeme_id, skill), review_log_card,
// exposures_user_lexeme, and confusion_misses_learner_key_at. There is one query
// per kind of evidence for the whole learner, never one per word.

// LearnerQuery says how LearnerEvidence summarizes the outcomes of a word and skill.
// An outcome is one checked answer, newest first. The windows count outcomes from the newest.
type LearnerQuery struct {
	LexemeID       int64     // 0 reads every word. Otherwise one word.
	AgainWindow    int       // Outcomes in the "Again in the last N" count.
	AccuracyWindow int       // Outcomes in the accuracy count.
	RecentWindow   int       // Outcomes in the "last N right" count.
	MissSince      time.Time // Again ratings at or after this time count in RecentAgains.
	ConfusionSince time.Time // Confusion rows at or after this time are returned.
	// AsOf, when non-zero, keeps only attempts with created_at <= AsOf and review_log
	// rows with reviewed_at <= AsOf. A zero AsOf leaves the live query text unchanged.
	AsOf time.Time
	// IncludeVoided counts answers an active void covers. The zero value skips
	// them. A void with undone_at set always counts. INIT-038/SPEC-002.
	IncludeVoided bool
}

// LearnerSkill is what one learner has for one word in one skill.
//
// The outcomes are the learner's attempts for the word and skill, newest first, and
// an Again is an attempt that was not correct. A word and skill with no attempts but
// a card falls back to that card's review_log, where rating 1 is an Again. After v3
// every answer writes an attempt, so the fallback only serves reviews from before.
// Outcomes is all of them. Window, WindowCorrect and the other pairs count the newest few.
type LearnerSkill struct {
	LexemeID int64
	Skill    string

	Outcomes      int
	Window        int // Outcomes among the newest AccuracyWindow.
	WindowCorrect int
	AgainOf       int // Outcomes among the newest AgainWindow.
	Agains        int // Of those, the ones that were not right.
	Recent        int // Outcomes among the newest RecentWindow.
	RecentRight   int
	// LatestRight is true when the newest checked outcome was right.
	// Meaningful when Outcomes > 0. A voided answer is not an outcome.
	LatestRight bool
	LastOutcome time.Time // Zero when there are no outcomes.

	HasCard         bool
	CardState       string
	Stability       float64
	LastReview      time.Time // Card's last_review. Zero when never reviewed.
	Reps            int
	Lapses          int
	Reviews         int // review_log rows of the card.
	ReviewAgains    int
	ReviewSuccesses int // Checked Hard/Good/Easy rows (not peeked, not say; spell implied counts).
	// SuccessDays is how many distinct study days carry a checked success.
	// Day = studyDay of reviewed_at in the learner's stored zone (4:00 rollover).
	// INIT-036/SPEC-002; ADR-0031 D1.
	SuccessDays int
	// FirstSuccessDay and LastSuccessDay are the earliest and latest of those
	// study days, YYYY-MM-DD. Both are empty when SuccessDays is 0.
	// INIT-038/SPEC-002.
	FirstSuccessDay string
	LastSuccessDay  string
	// Taught is true when the word counts as taught (ADR-0033 D5).
	// TaughtDay is that study day, YYYY-MM-DD.
	Taught        bool
	TaughtDay     string
	RecentAgains  int       // Again rows at or after MissSince.
	LastReviewLog time.Time // Newest review_log row. Zero when none.
}

// LearnerExposure is how often a word was shown with no question asked.
type LearnerExposure struct {
	Count int
	Last  time.Time
}

// LearnerConfusion is one confusion_misses row tied to a word.
type LearnerConfusion struct {
	LexemeID int64
	Kind     string
	Key      string
	At       time.Time
}

// LearnerEvidence is everything the model reads for one learner.
type LearnerEvidence struct {
	Skills     []LearnerSkill
	Exposures  map[int64]LearnerExposure
	Confusions []LearnerConfusion
	// WaitingForSecondDay is how many words pass every part of the old same-day
	// known rule but have only one checked-success study day (ADR-0031 D5).
	// INIT-036/SPEC-002.
	WaitingForSecondDay int
	// Taught maps a taught lexeme to its taught study day (ADR-0033 D5).
	// A word absent here is not counted on the ladder.
	Taught map[int64]string
}

// CardReview is one review_log row of a card, in replay order.
type CardReview struct {
	Rating        int
	ReviewedAt    time.Time
	ElapsedDays   int
	ScheduledDays int
	Implied       bool
}

// CardHistory is one card with its review_log rows up to an AsOf bound, plus the
// stored schedule columns used for live ReplayMismatch without a second GetCard.
// CreatedAt is zero when cards.created_at is NULL.
type CardHistory struct {
	CardID     int64
	Skill      string
	LexemeID   int64
	State      string
	Due        time.Time
	Stability  float64
	Difficulty float64
	Reps       int
	Lapses     int
	LastReview time.Time
	CreatedAt  time.Time
	Reviews    []CardReview
}

// StoredCard returns the schedule fields as stored on the cards row.
func (h CardHistory) StoredCard() Card {
	return Card{
		ID: h.CardID, LexemeID: h.LexemeID, Skill: h.Skill,
		State: h.State, Due: h.Due, Stability: h.Stability, Difficulty: h.Difficulty,
		Reps: h.Reps, Lapses: h.Lapses, LastReview: h.LastReview, CreatedAt: h.CreatedAt,
	}
}

// LearnerEvidence reads the learner's evidence. It never returns another learner's rows.
func (s *Store) LearnerEvidence(ctx context.Context, userID int64, q LearnerQuery) (LearnerEvidence, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return LearnerEvidence{}, err
	}
	if userID <= 0 {
		return LearnerEvidence{}, errors.New("db: learner evidence needs a learner")
	}
	if q.AgainWindow <= 0 || q.AccuracyWindow <= 0 || q.RecentWindow <= 0 {
		return LearnerEvidence{}, errors.New("db: learner evidence needs positive windows")
	}
	out := LearnerEvidence{Exposures: map[int64]LearnerExposure{}, Taught: map[int64]string{}}
	byKey := map[skillKey]*LearnerSkill{}
	var order []skillKey
	get := func(lexeme int64, skill string) *LearnerSkill {
		k := skillKey{lexeme, skill}
		if p, ok := byKey[k]; ok {
			return p
		}
		p := &LearnerSkill{LexemeID: lexeme, Skill: skill}
		byKey[k] = p
		order = append(order, k)
		return p
	}

	outcomes := func(query string, args []any) error {
		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("db: learner outcomes: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var lexeme int64
			var skill, last string
			var r LearnerSkill
			var latest int
			if err := rows.Scan(&lexeme, &skill, &r.Outcomes, &r.Window, &r.WindowCorrect,
				&r.AgainOf, &r.Agains, &r.Recent, &r.RecentRight, &latest, &last); err != nil {
				return fmt.Errorf("db: learner outcomes: %w", err)
			}
			p := get(lexeme, skill)
			p.Outcomes, p.Window, p.WindowCorrect = r.Outcomes, r.Window, r.WindowCorrect
			p.AgainOf, p.Agains, p.Recent, p.RecentRight = r.AgainOf, r.Agains, r.Recent, r.RecentRight
			p.LatestRight = latest > 0
			if p.LastOutcome, err = ParseTime(last); err != nil {
				return fmt.Errorf("db: learner outcomes: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("db: learner outcomes: %w", err)
		}
		return nil
	}

	lexemeArg := func(args []any) []any {
		if q.LexemeID != 0 {
			return append(args, q.LexemeID)
		}
		return args
	}
	attemptWhere, cardWhere := "", ""
	if q.LexemeID != 0 {
		attemptWhere, cardWhere = " AND lexeme_id = ?", " AND c.lexeme_id = ?"
	}
	// AsOf bounds are appended only when set, so the live (zero AsOf) SQL stays unchanged.
	attemptAsOf, reviewAsOf, attemptExistsAsOf := "", "", ""
	var asOfBound string
	if !q.AsOf.IsZero() {
		asOfBound = FormatTime(q.AsOf)
		attemptAsOf = " AND created_at <= ?"
		reviewAsOf = " AND rl.reviewed_at <= ?"
		attemptExistsAsOf = " AND a.created_at <= ?"
	}
	withAsOf := func(args []any, n int) []any {
		if asOfBound == "" {
			return args
		}
		for i := 0; i < n; i++ {
			args = append(args, asOfBound)
		}
		return args
	}
	aggregate := fmt.Sprintf(`
		SELECT lexeme_id, skill, COUNT(*),
			SUM(rn <= %[1]d), SUM(rn <= %[1]d AND ok = 1),
			SUM(rn <= %[2]d), SUM(rn <= %[2]d AND ok = 0),
			SUM(rn <= %[3]d), SUM(rn <= %[3]d AND ok = 1),
			SUM(rn = 1 AND ok = 1),
			MAX(at)
		FROM r
		GROUP BY lexeme_id, skill`, q.AccuracyWindow, q.AgainWindow, q.RecentWindow)

	// Attempts, newest first inside each word and skill. The query walks
	// attempts_user_lexeme_skill_id backwards, so rows arrive grouped by word and skill and
	// newest first with no sort, and the counts are made in one pass here. A window
	// function would make SQLite sort every row, which is the slowest part at 50,000 rows.
	// The ORDER BY includes id. The index has to include id too, or SQLite sorts.
	// Say self-checks and flashcard peeks are skipped: they never count as outcomes
	// toward the newest-three rule (INIT-036/SPEC-002; ADR-0031 D2).
	if err := func() error {
		rows, err := conn.QueryContext(ctx, `
			SELECT lexeme_id, skill, correct, created_at, activity
			FROM attempts INDEXED BY attempts_user_lexeme_skill_id
			WHERE user_id = ?`+attemptWhere+attemptAsOf+skipVoid(q.IncludeVoided, "attempt_id", "attempts.id")+`
			ORDER BY lexeme_id DESC, skill DESC, created_at DESC, id DESC`,
			withAsOf(lexemeArg([]any{userID}), 1)...)
		if err != nil {
			return fmt.Errorf("db: learner attempts: %w", err)
		}
		defer rows.Close()
		var (
			cur      *LearnerSkill
			curLex   int64
			curSkill []byte
			rank     int
		)
		for rows.Next() {
			var lexeme int64
			var skill, at, activity sql.RawBytes
			var correct int
			if err := rows.Scan(&lexeme, &skill, &correct, &at, &activity); err != nil {
				return fmt.Errorf("db: learner attempts: %w", err)
			}
			act := string(activity)
			if act == "say" || act == "flashcard" {
				continue
			}
			if cur == nil || lexeme != curLex || !bytes.Equal(skill, curSkill) {
				cur = get(lexeme, string(skill))
				curLex, curSkill, rank = lexeme, append(curSkill[:0], skill...), 0
				if cur.LastOutcome, err = ParseTime(string(at)); err != nil {
					return fmt.Errorf("db: learner attempts: %w", err)
				}
			}
			rank++
			cur.Outcomes++
			if rank == 1 {
				cur.LatestRight = correct == 1
			}
			if rank <= q.AccuracyWindow {
				cur.Window++
				cur.WindowCorrect += correct
			}
			if rank <= q.AgainWindow {
				cur.AgainOf++
				cur.Agains += 1 - correct
			}
			if rank <= q.RecentWindow {
				cur.Recent++
				cur.RecentRight += correct
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("db: learner attempts: %w", err)
		}
		return nil
	}(); err != nil {
		return LearnerEvidence{}, err
	}
	// The review_log of a card that has no checked attempts at all.
	// Peeked and say self-check rows are left out of the newest-three (ADR-0031 D2).
	if err := outcomes(`
		WITH r AS (
			SELECT c.lexeme_id AS lexeme_id, c.skill AS skill, rl.rating > 1 AS ok, rl.reviewed_at AS at,
				ROW_NUMBER() OVER (PARTITION BY c.id ORDER BY rl.reviewed_at DESC, rl.id DESC) AS rn
			FROM cards AS c
			JOIN review_log AS rl ON rl.card_id = c.id
			WHERE c.user_id = ?`+cardWhere+`
			  AND rl.implied = 0 AND rl.peeked = 0`+reviewAsOf+skipVoid(q.IncludeVoided, "review_log_id", "rl.id")+`
			  AND NOT EXISTS (
				SELECT 1 FROM attempts AS a
				WHERE a.user_id = c.user_id AND a.lexeme_id = c.lexeme_id AND a.skill = c.skill
				  AND a.activity NOT IN ('say', 'flashcard')`+attemptExistsAsOf+skipVoid(q.IncludeVoided, "attempt_id", "a.id")+`
			  )
		)`+aggregate, withAsOf(lexemeArg([]any{userID}), 2)); err != nil {
		return LearnerEvidence{}, err
	}

	// Cards, with their review_log totals. When AsOf is set, omit cards that did
	// not exist yet (created_at after AsOf, or NULL created_at with no review by then).
	if err := func() error {
		cardArgs := []any{FormatTime(q.MissSince)}
		if !q.AsOf.IsZero() {
			cardArgs = append(cardArgs, FormatTime(q.AsOf))
		}
		cardArgs = append(cardArgs, userID)
		cardExist := ""
		if !q.AsOf.IsZero() {
			cardExist = `
			  AND (
			    (c.created_at IS NOT NULL AND c.created_at <= ?)
			    OR EXISTS (
			      SELECT 1 FROM review_log AS r2
			      WHERE r2.card_id = c.id AND r2.reviewed_at <= ?
			    )
			  )`
			cardArgs = append(cardArgs, FormatTime(q.AsOf), FormatTime(q.AsOf))
		}
		// ReviewSuccesses counts checked Hard+ only: peeked and say self-checks
		// (always peeked) are out; implied Good+ still counts on spell cards
		// (ADR-0030; ADR-0031 D2). INIT-036/SPEC-002.
		rows, err := conn.QueryContext(ctx, `
			SELECT c.lexeme_id, c.skill, c.state, c.stability, c.last_review, c.reps, c.lapses,
				COUNT(CASE WHEN rl.implied = 0 THEN rl.id END),
				COALESCE(SUM(rl.implied = 0 AND rl.rating = 1), 0),
				COALESCE(SUM(rl.rating > 1 AND rl.peeked = 0 AND (rl.implied = 0 OR c.skill = 'spell')), 0),
				COALESCE(SUM(rl.implied = 0 AND rl.rating = 1 AND rl.reviewed_at >= ?), 0),
				MAX(CASE WHEN rl.implied = 0 THEN rl.reviewed_at END)
			FROM cards AS c
			LEFT JOIN review_log AS rl ON rl.card_id = c.id`+reviewAsOf+skipVoid(q.IncludeVoided, "review_log_id", "rl.id")+`
			WHERE c.user_id = ?`+cardExist+cardWhere+`
			GROUP BY c.id`, lexemeArg(cardArgs)...)
		if err != nil {
			return fmt.Errorf("db: learner cards: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var lexeme int64
			var skill string
			var c LearnerSkill
			var last, lastLog sql.NullString
			if err := rows.Scan(&lexeme, &skill, &c.CardState, &c.Stability, &last, &c.Reps, &c.Lapses,
				&c.Reviews, &c.ReviewAgains, &c.ReviewSuccesses, &c.RecentAgains, &lastLog); err != nil {
				return fmt.Errorf("db: learner cards: %w", err)
			}
			p := get(lexeme, skill)
			p.HasCard, p.CardState, p.Stability = true, c.CardState, c.Stability
			p.Reps, p.Lapses = c.Reps, c.Lapses
			p.Reviews, p.ReviewAgains, p.ReviewSuccesses, p.RecentAgains = c.Reviews, c.ReviewAgains, c.ReviewSuccesses, c.RecentAgains
			if last.Valid {
				if p.LastReview, err = ParseTime(last.String); err != nil {
					return fmt.Errorf("db: learner cards: %w", err)
				}
			}
			if lastLog.Valid {
				if p.LastReviewLog, err = ParseTime(lastLog.String); err != nil {
					return fmt.Errorf("db: learner cards: %w", err)
				}
			}
		}
		return rows.Err()
	}(); err != nil {
		return LearnerEvidence{}, err
	}

	// Distinct study days that carry a checked success, in the learner's zone,
	// and the first and last of those days. INIT-036/SPEC-002; INIT-038/SPEC-002.
	zone, err := s.learnerStudyZone(ctx, userID)
	if err != nil {
		return LearnerEvidence{}, err
	}
	if err := func() error {
		rows, err := conn.QueryContext(ctx, `
			SELECT c.lexeme_id, c.skill, rl.reviewed_at
			FROM cards AS c
			JOIN review_log AS rl ON rl.card_id = c.id
			WHERE c.user_id = ?`+cardWhere+reviewAsOf+skipVoid(q.IncludeVoided, "review_log_id", "rl.id")+`
			  AND rl.rating > 1 AND rl.peeked = 0
			  AND (rl.implied = 0 OR c.skill = 'spell')`,
			withAsOf(lexemeArg([]any{userID}), 1)...)
		if err != nil {
			return fmt.Errorf("db: learner success days: %w", err)
		}
		defer rows.Close()
		spans := map[skillKey]*successSpan{}
		for rows.Next() {
			var lexeme int64
			var skill, atStr string
			if err := rows.Scan(&lexeme, &skill, &atStr); err != nil {
				return fmt.Errorf("db: learner success days: %w", err)
			}
			at, err := ParseTime(atStr)
			if err != nil {
				return fmt.Errorf("db: learner success days: %w", err)
			}
			k := skillKey{lexeme, skill}
			sp := spans[k]
			if sp == nil {
				sp = &successSpan{days: map[string]struct{}{}}
				spans[k] = sp
			}
			day := studyDay(at, zone)
			sp.days[day] = struct{}{}
			if sp.first == "" || day < sp.first {
				sp.first = day
			}
			if day > sp.last {
				sp.last = day
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("db: learner success days: %w", err)
		}
		for k, sp := range spans {
			p := get(k.lexeme, k.skill)
			p.SuccessDays = len(sp.days)
			p.FirstSuccessDay = sp.first
			p.LastSuccessDay = sp.last
		}
		return nil
	}(); err != nil {
		return LearnerEvidence{}, err
	}

	// Exposures. AsOf bounds created_at the same way as attempts when set.
	if err := func() error {
		where := ""
		if q.LexemeID != 0 {
			where = " AND lexeme_id = ?"
		}
		exposureAsOf := ""
		if !q.AsOf.IsZero() {
			exposureAsOf = " AND created_at <= ?"
		}
		rows, err := conn.QueryContext(ctx, `
			SELECT lexeme_id, COUNT(*), MAX(created_at)
			FROM exposures
			WHERE user_id = ?`+where+exposureAsOf+`
			GROUP BY lexeme_id`, withAsOf(lexemeArg([]any{userID}), 1)...)
		if err != nil {
			return fmt.Errorf("db: learner exposures: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var lexeme int64
			var e LearnerExposure
			var last string
			if err := rows.Scan(&lexeme, &e.Count, &last); err != nil {
				return fmt.Errorf("db: learner exposures: %w", err)
			}
			if e.Last, err = ParseTime(last); err != nil {
				return fmt.Errorf("db: learner exposures: %w", err)
			}
			out.Exposures[lexeme] = e
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("db: learner exposures: %w", err)
		}
		return nil
	}(); err != nil {
		return LearnerEvidence{}, err
	}

	// Confusions inside the window.
	if err := func() error {
		where := ""
		if q.LexemeID != 0 {
			where = " AND lexeme_id = ?"
		}
		rows, err := conn.QueryContext(ctx, `
			SELECT lexeme_id, kind, key, at
			FROM confusion_misses
			WHERE learner_id = ? AND lexeme_id IS NOT NULL AND at >= ?`+where+skipVoid(q.IncludeVoided, "confusion_id", "confusion_misses.id")+`
			ORDER BY at, id`, lexemeArg([]any{userID, FormatTime(q.ConfusionSince)})...)
		if err != nil {
			return fmt.Errorf("db: learner confusions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var c LearnerConfusion
			var at string
			if err := rows.Scan(&c.LexemeID, &c.Kind, &c.Key, &at); err != nil {
				return fmt.Errorf("db: learner confusions: %w", err)
			}
			if c.At, err = ParseTime(at); err != nil {
				return fmt.Errorf("db: learner confusions: %w", err)
			}
			out.Confusions = append(out.Confusions, c)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("db: learner confusions: %w", err)
		}
		return nil
	}(); err != nil {
		return LearnerEvidence{}, err
	}

	taught, err := s.markTaught(ctx, conn, userID, q, zone, byKey)
	if err != nil {
		return LearnerEvidence{}, err
	}
	out.Taught = taught

	out.Skills = make([]LearnerSkill, 0, len(order))
	for _, k := range order {
		out.Skills = append(out.Skills, *byKey[k])
	}
	now := s.Clock().Now()
	if !q.AsOf.IsZero() {
		now = q.AsOf
	}
	out.WaitingForSecondDay = waitingForSecondDay(out.Skills, now)
	return out, nil
}

type skillKey struct {
	lexeme int64
	skill  string
}

// successSpan is the checked-success study days of one word and skill.
type successSpan struct {
	days        map[string]struct{}
	first, last string
}

// studyRolloverHour is when the learner's study day rolls, local time (ADR-0022).
const studyRolloverHour = 4

// studyDay is the learner's study day YYYY-MM-DD for at in z.
// The study day rolls at 04:00 local (ADR-0022); before then it is the previous
// calendar day. INIT-036/SPEC-002.
func studyDay(at time.Time, z studyday.Zone) string {
	local := at.In(z.Loc)
	y, m, d := local.Date()
	rollover := time.Date(y, m, d, studyRolloverHour, 0, 0, 0, z.Loc)
	if local.Before(rollover) {
		prev := time.Date(y, m, d-1, 12, 0, 0, 0, z.Loc)
		return prev.Format(time.DateOnly)
	}
	return local.Format(time.DateOnly)
}

// learnerStudyZone is the learner's stored zone, or the app default when unset.
func (s *Store) learnerStudyZone(ctx context.Context, userID int64) (studyday.Zone, error) {
	name, ok, err := s.UserTimeZone(ctx, userID)
	if err != nil {
		return studyday.Zone{}, err
	}
	if !ok {
		loc, err := studyday.AppZone("")
		if err != nil {
			return studyday.Zone{}, err
		}
		return studyday.Zone{Name: loc.String(), Loc: loc, Source: studyday.SourceApp}, nil
	}
	loc, err := studyday.ParseZone(name)
	if err != nil {
		return studyday.Zone{}, fmt.Errorf("db: learner %d zone: %w", userID, err)
	}
	return studyday.Zone{Name: name, Loc: loc, Source: studyday.SourceLearner}, nil
}

// skillOldKnown is the same-day half of rule version 4. The thresholds live in
// internal/learner/rule; this calls that rule (INIT-038/SPEC-005).
func skillOldKnown(sk LearnerSkill, now time.Time) bool {
	return rule.SameDayKnown(skillInput(sk, now))
}

// skillStrictKnown is rule version 4, including the second study day.
func skillStrictKnown(sk LearnerSkill, now time.Time) bool {
	return rule.Rule4(skillInput(sk, now))
}

func skillInput(sk LearnerSkill, now time.Time) rule.Input {
	in := rule.Input{
		Outcomes: sk.Outcomes, Recent: sk.Recent, RecentRight: sk.RecentRight,
		Successes: sk.ReviewSuccesses, SuccessDays: sk.SuccessDays, HasCard: sk.HasCard,
		LatestRight: sk.LatestRight, Taught: sk.Taught, Stability: sk.Stability,
		First: sk.FirstSuccessDay, Last: sk.LastSuccessDay, TaughtDay: sk.TaughtDay,
	}
	if sk.HasCard && !sk.LastReview.IsZero() {
		in.ElapsedDays = now.Sub(sk.LastReview).Hours() / 24
	}
	in.Retrievability = rule.Chance(in.Stability, in.ElapsedDays)
	return in
}

func recalledOrProved(sk LearnerSkill, now time.Time) bool {
	return rule.RecalledOrProved(skillInput(sk, now))
}

// waitingForSecondDay counts words that pass the old known rule on some skill
// with only one success day, and are not strictly known on any skill.
func waitingForSecondDay(skills []LearnerSkill, now time.Time) int {
	type flags struct{ waiting, known bool }
	byLex := map[int64]*flags{}
	for _, sk := range skills {
		f := byLex[sk.LexemeID]
		if f == nil {
			f = &flags{}
			byLex[sk.LexemeID] = f
		}
		if skillStrictKnown(sk, now) {
			f.known = true
			continue
		}
		if skillOldKnown(sk, now) && sk.SuccessDays == 1 {
			f.waiting = true
		}
	}
	n := 0
	for _, f := range byLex {
		if f.waiting && !f.known {
			n++
		}
	}
	return n
}

// LexemeText is the part of a lexeme the learner model shows.
type LexemeText struct {
	ID       int64
	TextNFC  string
	Gloss    string
	Translit string
}

// LexemeIDs returns every lexeme id, ascending.
func (s *Store) LexemeIDs(ctx context.Context) ([]int64, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `SELECT id FROM lexemes ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("db: lexeme ids: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("db: lexeme ids: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: lexeme ids: %w", err)
	}
	return out, nil
}

// LexemeTexts returns the text and gloss of the given lexemes, keyed by id.
// An id that does not exist is missing from the map.
func (s *Store) LexemeTexts(ctx context.Context, ids []int64) (map[int64]LexemeText, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	out := make(map[int64]LexemeText, len(ids))
	// SQLite allows 32766 variables. A page is at most a few hundred, so one query is enough,
	// and chunking keeps a bigger caller safe.
	const chunk = 500
	for start := 0; start < len(ids); start += chunk {
		end := min(start+chunk, len(ids))
		part := ids[start:end]
		marks := strings.TrimSuffix(strings.Repeat("?,", len(part)), ",")
		args := make([]any, len(part))
		for i, id := range part {
			args[i] = id
		}
		rows, err := conn.QueryContext(ctx, `SELECT id, text_nfc, gloss_en, translit FROM lexemes WHERE id IN (`+marks+`)`, args...)
		if err != nil {
			return nil, fmt.Errorf("db: lexeme texts: %w", err)
		}
		for rows.Next() {
			var l LexemeText
			var translit sql.NullString
			if err := rows.Scan(&l.ID, &l.TextNFC, &l.Gloss, &translit); err != nil {
				rows.Close()
				return nil, fmt.Errorf("db: lexeme texts: %w", err)
			}
			l.Translit = translit.String
			out[l.ID] = l
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("db: lexeme texts: %w", err)
		}
	}
	return out, nil
}

// RecallLearnedAt returns, for one learner, the time of the earliest recall review of each
// word that has one. It is the closest record of when the word moved into known.
// An active void on the review_log row leaves that review out. INIT-038/SPEC-002.
func (s *Store) RecallLearnedAt(ctx context.Context, userID int64) (map[int64]time.Time, error) {
	return s.recallLearnedAt(ctx, userID, EvidenceOpts{})
}

// RecallLearnedAtOpts is RecallLearnedAt with EvidenceOpts.
func (s *Store) RecallLearnedAtOpts(ctx context.Context, userID int64, opts EvidenceOpts) (map[int64]time.Time, error) {
	return s.recallLearnedAt(ctx, userID, opts)
}

func (s *Store) recallLearnedAt(ctx context.Context, userID int64, opts EvidenceOpts) (map[int64]time.Time, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT c.lexeme_id, MIN(r.reviewed_at)
		FROM cards AS c JOIN review_log AS r ON r.card_id = c.id`+skipVoid(opts.IncludeVoided, "review_log_id", "r.id")+`
		WHERE c.user_id = ? AND c.skill = 'recall'
		GROUP BY c.lexeme_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("db: recall learned at: %w", err)
	}
	defer rows.Close()
	out := map[int64]time.Time{}
	for rows.Next() {
		var id int64
		var at string
		if err := rows.Scan(&id, &at); err != nil {
			return nil, fmt.Errorf("db: recall learned at: %w", err)
		}
		when, err := ParseTime(at)
		if err != nil {
			return nil, fmt.Errorf("db: recall learned at: %w", err)
		}
		out[id] = when
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: recall learned at: %w", err)
	}
	return out, nil
}

// RecentChecked counts the newest n attempts of one learner: how many there are, and how many
// were right. n under 1 reads nothing. An active void leaves that attempt out. INIT-038/SPEC-002.
func (s *Store) RecentChecked(ctx context.Context, userID int64, n int) (correct, total int, err error) {
	return s.recentChecked(ctx, userID, n, EvidenceOpts{})
}

// RecentCheckedOpts is RecentChecked with EvidenceOpts.
func (s *Store) RecentCheckedOpts(ctx context.Context, userID int64, n int, opts EvidenceOpts) (correct, total int, err error) {
	return s.recentChecked(ctx, userID, n, opts)
}

func (s *Store) recentChecked(ctx context.Context, userID int64, n int, opts EvidenceOpts) (correct, total int, err error) {
	if n < 1 {
		return 0, 0, nil
	}
	conn, err := s.connOrErr()
	if err != nil {
		return 0, 0, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT correct FROM attempts WHERE user_id = ?`+skipVoid(opts.IncludeVoided, "attempt_id", "attempts.id")+`
		ORDER BY id DESC LIMIT ?`, userID, n)
	if err != nil {
		return 0, 0, fmt.Errorf("db: recent checked: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c int
		if err := rows.Scan(&c); err != nil {
			return 0, 0, fmt.Errorf("db: recent checked: %w", err)
		}
		total++
		correct += c
	}
	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("db: recent checked: %w", err)
	}
	return correct, total, nil
}

// PlacementItemLexeme says which placement item a lexeme was dealt as.
type PlacementItemLexeme struct {
	ItemID   string
	LexemeID int64
}

// PlacementItemLexemes returns each distinct pair of placement item and lexeme that any run,
// v2 or v3, has answered. The pairs say which deck item a word is, which is how a word gets a band. The
// result carries no learner, so it is the same for every caller.
func (s *Store) PlacementItemLexemes(ctx context.Context) ([]PlacementItemLexeme, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT item_id, lexeme_id FROM placement_answers WHERE lexeme_id IS NOT NULL
		UNION
		SELECT item_id, lexeme_id FROM placement_items WHERE lexeme_id IS NOT NULL AND part <> 'letters'`)
	if err != nil {
		return nil, fmt.Errorf("db: placement items: %w", err)
	}
	defer rows.Close()
	var out []PlacementItemLexeme
	for rows.Next() {
		var p PlacementItemLexeme
		if err := rows.Scan(&p.ItemID, &p.LexemeID); err != nil {
			return nil, fmt.Errorf("db: placement items: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: placement items: %w", err)
	}
	return out, nil
}

// PlacementBands returns the band tallies of the learner's newest run, when that run is
// finished. ok is false when the learner has no finished newest run.
func (s *Store) PlacementBands(ctx context.Context, userID int64) (bands []BandScore, ok bool, err error) {
	run, err := s.LatestRun(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if run.FinishedAt.IsZero() || run.Result == "" {
		return nil, false, nil
	}
	var body placementResultJSON
	if err := json.Unmarshal([]byte(run.Result), &body); err != nil {
		return nil, false, fmt.Errorf("db: placement bands for user %d: %w", userID, err)
	}
	return body.Bands, true, nil
}

// CardHistories returns each of the learner's cards (LEFT JOIN review_log) with
// stored schedule columns and review rows in reviewed_at, id order.
// When asOf is non-zero, only review_log rows with reviewed_at <= asOf are joined,
// and cards that did not exist yet are omitted: created_at > asOf, or created_at
// IS NULL with no review at or before asOf. A zero asOf returns every card and
// its full history. An active void leaves that review_log row out of the history.
// INIT-038/SPEC-002.
func (s *Store) CardHistories(ctx context.Context, userID int64, asOf time.Time) ([]CardHistory, error) {
	return s.cardHistories(ctx, userID, asOf, EvidenceOpts{})
}

// CardHistoriesOpts is CardHistories with EvidenceOpts.
func (s *Store) CardHistoriesOpts(ctx context.Context, userID int64, asOf time.Time, opts EvidenceOpts) ([]CardHistory, error) {
	return s.cardHistories(ctx, userID, asOf, opts)
}

func (s *Store) cardHistories(ctx context.Context, userID int64, asOf time.Time, opts EvidenceOpts) ([]CardHistory, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, errors.New("db: card histories needs a learner")
	}
	query := `
		SELECT c.id, c.skill, c.lexeme_id, c.state, c.due, c.stability, c.difficulty, c.reps, c.lapses,
			c.last_review, c.created_at,
			rl.rating, rl.reviewed_at, rl.elapsed_days, rl.scheduled_days, rl.implied
		FROM cards AS c
		LEFT JOIN review_log AS rl ON rl.card_id = c.id` + skipVoid(opts.IncludeVoided, "review_log_id", "rl.id")
	args := []any{}
	if !asOf.IsZero() {
		query += ` AND rl.reviewed_at <= ?`
		args = append(args, FormatTime(asOf))
	}
	query += `
		WHERE c.user_id = ?`
	args = append(args, userID)
	if !asOf.IsZero() {
		query += `
		  AND (
		    (c.created_at IS NOT NULL AND c.created_at <= ?)
		    OR EXISTS (
		      SELECT 1 FROM review_log AS r2
		      WHERE r2.card_id = c.id AND r2.reviewed_at <= ?
		    )
		  )`
		bound := FormatTime(asOf)
		args = append(args, bound, bound)
	}
	query += `
		ORDER BY c.id, rl.reviewed_at, rl.id`
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("db: card histories: %w", err)
	}
	defer rows.Close()
	var out []CardHistory
	var cur *CardHistory
	for rows.Next() {
		var cardID, lexeme int64
		var skill, state, due string
		var last, created sql.NullString
		var stability, difficulty float64
		var reps, lapses int
		var rating, elapsed, scheduled, implied sql.NullInt64
		var reviewedAt sql.NullString
		if err := rows.Scan(&cardID, &skill, &lexeme, &state, &due, &stability, &difficulty, &reps, &lapses,
			&last, &created, &rating, &reviewedAt, &elapsed, &scheduled, &implied); err != nil {
			return nil, fmt.Errorf("db: card histories: %w", err)
		}
		if cur == nil || cur.CardID != cardID {
			h := CardHistory{
				CardID: cardID, Skill: skill, LexemeID: lexeme,
				State: state, Stability: stability, Difficulty: difficulty, Reps: reps, Lapses: lapses,
			}
			h.Due, err = ParseTime(due)
			if err != nil {
				return nil, fmt.Errorf("db: card histories: %w", err)
			}
			if last.Valid {
				if h.LastReview, err = ParseTime(last.String); err != nil {
					return nil, fmt.Errorf("db: card histories: %w", err)
				}
			}
			if created.Valid {
				if h.CreatedAt, err = ParseTime(created.String); err != nil {
					return nil, fmt.Errorf("db: card histories: %w", err)
				}
			}
			out = append(out, h)
			cur = &out[len(out)-1]
		}
		if !rating.Valid || !reviewedAt.Valid {
			continue
		}
		reviewed, err := ParseTime(reviewedAt.String)
		if err != nil {
			return nil, fmt.Errorf("db: card histories: %w", err)
		}
		cur.Reviews = append(cur.Reviews, CardReview{
			Rating: int(rating.Int64), ReviewedAt: reviewed,
			ElapsedDays: int(elapsed.Int64), ScheduledDays: int(scheduled.Int64), Implied: implied.Int64 == 1,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: card histories: %w", err)
	}
	return out, nil
}

// RetentionRow is the due reviews of one skill. Due is how many were reviewed in the
// window after at least a day away from the card, and Right is how many of those were
// not rated Again.
type RetentionRow struct {
	Skill string
	Due   int
	Right int
}

// DueRetention counts the learner's due reviews since a time, per skill. A due review is
// a review_log row with elapsed_days of 1 or more: the card was reviewed after waiting,
// not again in the same sitting. When asOf is non-zero, only rows with reviewed_at <= asOf
// are counted. A zero asOf leaves the unbound query unchanged. An active void leaves
// that review out. INIT-038/SPEC-002.
func (s *Store) DueRetention(ctx context.Context, userID int64, since, asOf time.Time) ([]RetentionRow, error) {
	return s.dueRetention(ctx, userID, since, asOf, EvidenceOpts{})
}

// DueRetentionOpts is DueRetention with EvidenceOpts.
func (s *Store) DueRetentionOpts(ctx context.Context, userID int64, since, asOf time.Time, opts EvidenceOpts) ([]RetentionRow, error) {
	return s.dueRetention(ctx, userID, since, asOf, opts)
}

func (s *Store) dueRetention(ctx context.Context, userID int64, since, asOf time.Time, opts EvidenceOpts) ([]RetentionRow, error) {
	conn, err := s.connOrErr()
	if err != nil {
		return nil, err
	}
	query := `
		SELECT c.skill, COUNT(*), COALESCE(SUM(rl.rating > 1), 0)
		FROM review_log AS rl
		JOIN cards AS c ON c.id = rl.card_id
		WHERE c.user_id = ? AND rl.reviewed_at >= ? AND rl.elapsed_days >= 1 AND rl.implied = 0` + skipVoid(opts.IncludeVoided, "review_log_id", "rl.id")
	args := []any{userID, FormatTime(since)}
	if !asOf.IsZero() {
		query += ` AND rl.reviewed_at <= ?`
		args = append(args, FormatTime(asOf))
	}
	query += `
		GROUP BY c.skill`
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("db: due retention: %w", err)
	}
	defer rows.Close()
	var out []RetentionRow
	for rows.Next() {
		var r RetentionRow
		if err := rows.Scan(&r.Skill, &r.Due, &r.Right); err != nil {
			return nil, fmt.Errorf("db: due retention: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: due retention: %w", err)
	}
	return out, nil
}

// RecentConfusionPairs returns the learner's confusions since a time, grouped by kind and
// key, most frequent first. It is the confusion list of StruggleStats with the window given.
// An active void on the confusion row leaves it out. INIT-038/SPEC-002.
func (s *Store) RecentConfusionPairs(ctx context.Context, userID int64, since time.Time) ([]ConfusionPair, error) {
	return s.confusionPairs(ctx, userID, FormatTime(since), false)
}

// RecentConfusionPairsOpts is RecentConfusionPairs with EvidenceOpts.
func (s *Store) RecentConfusionPairsOpts(ctx context.Context, userID int64, since time.Time, opts EvidenceOpts) ([]ConfusionPair, error) {
	return s.confusionPairs(ctx, userID, FormatTime(since), opts.IncludeVoided)
}
