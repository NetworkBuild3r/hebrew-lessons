package mcpstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/jobs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcp"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/sessionlimits"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
)

var errDigestZone = errors.New("mcpstore: study digest requires a zone location")

// INIT-026/SPEC-009: StudyDigest builds get_study_digest from the same store the tools share.

const (
	digestStruggleCap    = 10
	digestCorrectionCap  = 20
	digestTaughtCap      = 30
	digestCoachNoteCap   = 5
	digestRetentionDays  = 7
	digestMinutesDays    = 7
	digestTaughtDays     = 14
	digestCorrectionDays = 30
	digestRetentionHigh  = 0.90
)

var _ mcp.DigestReader = (*Store)(nil)

// StudyDigest implements mcp.DigestReader. Every count is computed here; the tool does not recompute.
func (s *Store) StudyDigest(ctx context.Context, id int64, now time.Time, z studyday.Zone) (mcp.StudyDigest, error) {
	if err := s.ready(ctx); err != nil {
		return mcp.StudyDigest{}, err
	}
	if z.Loc == nil {
		return mcp.StudyDigest{}, errDigestZone
	}
	today := studyday.Today(now, z)
	cutoff := studyday.DueCutoff(now, z)
	// Retention and minutes cover the last 7 learner days inclusive (today back 6).
	day7ago := civilAdd(today, -(digestRetentionDays - 1))
	// Skills also compare known+solid on the snapshot from exactly 7 days before today.
	ago7 := civilAdd(today, -7)
	day14ago := civilAdd(today, -(digestTaughtDays - 1))
	day30ago := civilAdd(today, -(digestCorrectionDays - 1))

	out := mcp.StudyDigest{
		Zone: z.Name, ZoneSource: z.Source, Today: today, NextCutoff: cutoff.UTC(),
		AsOf: now.UTC(), Skills: []mcp.DigestSkill{}, Struggle: []mcp.DigestStruggleWord{},
		Corrections: []mcp.DigestCorrection{}, Taught: []mcp.DigestTaughtWord{},
		CoachNotes: []mcp.CoachNote{},
		ClosedLoop: mcp.DigestClosedLoop{Since: day30ago},
		Retention: mcp.DigestRetention{
			BandLow: sessionlimits.TargetSuccess, BandHigh: digestRetentionHigh,
			Skills: []mcp.DigestRetentionSkill{},
		},
	}

	live, err := learner.New(s.db, nil).Summary(ctx, id)
	if err != nil {
		return out, err
	}
	out.KnownAnySkill = live.KnownAnySkill

	snaps, err := s.db.SnapshotRows(ctx, id, ago7, today)
	if err != nil {
		return out, err
	}
	days := snapshotDays(snaps)
	out.Skills = digestSkills(days, ago7, live)
	out.Retention.Skills = digestRetention(days, day7ago, today)

	fromMin, toMin, err := civilWindow(day7ago, today, z)
	if err != nil {
		return out, err
	}
	attempts, err := s.db.AttemptTimes(ctx, id, fromMin, toMin)
	if err != nil {
		return out, err
	}
	out.MinutesInApp = jobs.ActiveMinutes(attempts)

	struggle, err := s.digestStruggle(ctx, id)
	if err != nil {
		return out, err
	}
	out.Struggle = struggle

	corrections, closed, err := s.digestCorrections(ctx, id, day30ago, today, z)
	if err != nil {
		return out, err
	}
	out.Corrections, out.ClosedLoop = corrections, closed

	taught, err := s.digestTaught(ctx, id, day14ago, today, z)
	if err != nil {
		return out, err
	}
	out.Taught = taught

	dueNow, err := s.db.ListDueCards(ctx, id, now)
	if err != nil {
		return out, err
	}
	dueBy, err := s.db.ListDueCards(ctx, id, cutoff)
	if err != nil {
		return out, err
	}
	out.Due = mcp.DigestDue{Now: len(dueNow), ByCutoff: len(dueBy)}

	notes, err := s.RecentCoachNotes(ctx, id, digestCoachNoteCap)
	if err != nil {
		return out, err
	}
	out.CoachNotes = notes

	last, err := s.newestWeeklyReport(ctx, id)
	if err != nil {
		return out, err
	}
	out.LastWeek = last
	return out, nil
}

// digestSkills fills live knownSolidNow / strugglingNow from the model and
// knownSolid7dAgo from the snapshot on ago7 (ADR-0027 D4 / INIT-032/SPEC-005).
func digestSkills(days []mcp.SnapshotDay, ago7 string, live learner.Summary) []mcp.DigestSkill {
	var agoDay *mcp.SnapshotDay
	for i := range days {
		if days[i].Day == ago7 {
			agoDay = &days[i]
		}
	}
	out := make([]mcp.DigestSkill, 0, len(learner.Skills))
	for _, sk := range live.Skills {
		row := mcp.DigestSkill{
			Skill:         sk.Skill,
			KnownSolidNow: sk.Counts[learner.Known] + sk.Counts[learner.Solid],
			StrugglingNow: sk.Counts[learner.Struggling],
			AgoDay:        ago7,
		}
		if agoDay != nil {
			for _, snap := range agoDay.Skills {
				if snap.Skill == sk.Skill {
					row.KnownSolid7dAgo = knownOf(snap)
				}
			}
		}
		out = append(out, row)
	}
	return out
}

func digestRetention(days []mcp.SnapshotDay, from, to string) []mcp.DigestRetentionSkill {
	out := make([]mcp.DigestRetentionSkill, 0, len(learner.Skills))
	for _, skill := range learner.Skills {
		row := mcp.DigestRetentionSkill{Skill: skill, Days: []mcp.DigestRetentionDay{}, BandStatus: "none"}
		var latest *float64
		for d := from; d <= to; d = civilAdd(d, 1) {
			var value *float64
			for _, day := range days {
				if day.Day != d {
					continue
				}
				for _, sk := range day.Skills {
					if sk.Skill == skill && sk.Retention != nil {
						v := *sk.Retention
						value = &v
						latest = &v
					}
				}
			}
			if value == nil {
				continue // omit empty days to keep the digest under the 16 KB cap
			}
			row.Days = append(row.Days, mcp.DigestRetentionDay{
				Day: d, Value: value, Window: mcp.RetentionWindow30d, AsOf: d,
			})
		}
		if latest != nil {
			switch {
			case *latest < sessionlimits.TargetSuccess:
				row.BandStatus = "below"
			case *latest > digestRetentionHigh:
				row.BandStatus = "above"
			default:
				row.BandStatus = "in_band"
			}
		}
		out = append(out, row)
	}
	return out
}

func (s *Store) digestStruggle(ctx context.Context, id int64) ([]mcp.DigestStruggleWord, error) {
	words, _, err := learner.New(s.db, nil).List(ctx, id, learner.Filter{
		State: learner.Struggling, Limit: digestStruggleCap,
	})
	if err != nil {
		return nil, err
	}
	out := make([]mcp.DigestStruggleWord, 0, len(words))
	for _, w := range words {
		out = append(out, mcp.DigestStruggleWord{
			Text: w.Text, Gloss: w.Gloss, Cause: string(w.Cause),
			ConfusedWith: confusedWith(w.Confusions), LastSeen: timePtr(w.LastSeen),
		})
	}
	return out, nil
}

func (s *Store) digestCorrections(ctx context.Context, id int64, from, to string, z studyday.Zone) ([]mcp.DigestCorrection, mcp.DigestClosedLoop, error) {
	closed := mcp.DigestClosedLoop{Since: from}
	conn, err := s.conn(ctx)
	if err != nil {
		return nil, closed, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT wrong_he, right_he, COALESCE(note_en, ''), times, COALESCE(last_on, ''), COALESCE(lexeme_id, 0),
			COALESCE((SELECT text_nfc FROM lexemes WHERE id = learner_mistakes.lexeme_id), '')
		FROM learner_mistakes
		WHERE learner_id = ? AND last_on IS NOT NULL AND last_on >= ? AND last_on <= ?
		ORDER BY last_on DESC, times DESC, id DESC
		LIMIT ?`, id, from, to, digestCorrectionCap)
	if err != nil {
		return nil, closed, err
	}
	type row struct {
		c        mcp.DigestCorrection
		lexemeID int64
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.c.Wrong, &r.c.Right, &r.c.Note, &r.c.Times, &r.c.On, &r.lexemeID, &r.c.Word); err != nil {
			rows.Close()
			return nil, closed, err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, closed, err
	}
	rows.Close()

	out := make([]mcp.DigestCorrection, 0, len(list))
	for _, r := range list {
		c := r.c
		c.Status = mcp.DigestCorrectionStatus{Kind: "open"}
		if r.lexemeID > 0 && c.On != "" {
			n, err := s.rightSinceCount(ctx, id, r.lexemeID, c.On, z)
			if err != nil {
				return nil, closed, err
			}
			if n > 0 {
				c.Status = mcp.DigestCorrectionStatus{Kind: "right_since", Count: n}
				closed.RightSince++
			}
		}
		out = append(out, c)
	}
	closed.Total = len(out)
	return out, closed, nil
}

// rightSinceCount is correct recall/spell attempts after the end of last_on in z.
// A right answer earlier on that same learner day does not count.
func (s *Store) rightSinceCount(ctx context.Context, userID, lexemeID int64, lastOn string, z studyday.Zone) (int, error) {
	end, err := endOfCivilDay(lastOn, z)
	if err != nil {
		return 0, err
	}
	conn, err := s.conn(ctx)
	if err != nil {
		return 0, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT created_at FROM attempts
		WHERE user_id = ? AND lexeme_id = ? AND correct = 1 AND skill IN ('recall', 'spell')
		  AND created_at >= ?
		ORDER BY created_at, id`,
		userID, lexemeID, db.FormatTime(end.Add(-24*time.Hour)))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var at string
		if err := rows.Scan(&at); err != nil {
			return 0, err
		}
		t, err := db.ParseTime(at)
		if err != nil {
			return 0, err
		}
		if !t.Before(end) {
			n++
		}
	}
	return n, rows.Err()
}

func (s *Store) digestTaught(ctx context.Context, id int64, from, to string, z studyday.Zone) ([]mcp.DigestTaughtWord, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT t.lexeme_id, l.text_nfc, t.taught_on
		FROM taught_words AS t
		JOIN lexemes AS l ON l.id = t.lexeme_id
		WHERE t.learner_id = ? AND t.taught_on >= ? AND t.taught_on <= ?
		ORDER BY t.taught_on DESC, t.id DESC
		LIMIT ?`, id, from, to, digestTaughtCap)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct {
		lexeme int64
		text   string
		on     string
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.lexeme, &r.text, &r.on); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]mcp.DigestTaughtWord, 0, len(list))
	for _, r := range list {
		cards, err := s.taughtCards(ctx, id, r.lexeme, r.on, z)
		if err != nil {
			return nil, err
		}
		out = append(out, mcp.DigestTaughtWord{Text: r.text, On: r.on, Cards: cards})
	}
	return out, nil
}

func (s *Store) taughtCards(ctx context.Context, userID, lexemeID int64, on string, z studyday.Zone) ([]mcp.DigestTaughtCard, error) {
	start, err := startOfCivilDay(on, z)
	if err != nil {
		return nil, err
	}
	conn, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := conn.QueryContext(ctx, `
		SELECT c.skill, c.state,
			(SELECT COUNT(*) FROM review_log AS r WHERE r.card_id = c.id AND r.reviewed_at >= ? AND r.implied = 0)
		FROM cards AS c
		WHERE c.user_id = ? AND c.lexeme_id = ?
		ORDER BY c.skill`, db.FormatTime(start), userID, lexemeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []mcp.DigestTaughtCard{}
	for rows.Next() {
		var c mcp.DigestTaughtCard
		if err := rows.Scan(&c.Skill, &c.State, &c.Reviews); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) newestWeeklyReport(ctx context.Context, id int64) (*mcp.ReportView, error) {
	conn, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	var r mcp.ReportView
	var at, payload string
	err = conn.QueryRowContext(ctx, `
		SELECT week_start, summary, created_at, payload_json FROM weekly_reports
		WHERE user_id = ?
		ORDER BY week_start DESC LIMIT 1`, id).Scan(&r.WeekStart, &r.Summary, &at, &payload)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if r.CreatedAt, err = db.ParseTime(at); err != nil {
		return nil, err
	}
	r.Payload = decodeReportPayload(payload)
	labelWeeklyRetention(r.Payload)
	return &r, nil
}

func civilAdd(day string, delta int) string {
	t, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return day
	}
	return t.AddDate(0, 0, delta).Format(time.DateOnly)
}

func startOfCivilDay(day string, z studyday.Zone) (time.Time, error) {
	t, err := time.ParseInLocation(time.DateOnly, day, z.Loc)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func endOfCivilDay(day string, z studyday.Zone) (time.Time, error) {
	t, err := startOfCivilDay(day, z)
	if err != nil {
		return time.Time{}, err
	}
	return t.AddDate(0, 0, 1), nil
}

// civilWindow is [start of from, start of the day after to) in z.
func civilWindow(from, to string, z studyday.Zone) (time.Time, time.Time, error) {
	start, err := startOfCivilDay(from, z)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := endOfCivilDay(to, z)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, end, nil
}
