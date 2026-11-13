package web

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/jobs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/speech"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func (s *server) courseView(ctx context.Context, userID int64) (views.CoursePage, error) {
	var v views.CoursePage
	course, err := s.store.CourseLexemes(ctx)
	if err != nil {
		return v, err
	}
	have, err := s.store.LearnerCardLexemeIDs(ctx, userID)
	if err != nil {
		return v, err
	}
	file, err := speech.Course()
	if err != nil {
		return v, err
	}
	byOrder := map[int]speech.CourseWord{}
	for _, w := range file {
		byOrder[w.Order] = w
	}
	met := map[int64]db.CourseLexeme{}
	for _, row := range course {
		if _, ok := have[row.LexemeID]; ok {
			met[row.LexemeID] = row
			v.Met++
			if row.Order > v.Place {
				v.Place = row.Order
			}
		}
	}
	known, err := s.courseKnown(ctx, userID)
	if err != nil {
		return v, err
	}
	readyByLemma := map[string]int{}
	knownByLemma := map[string]int{}
	for _, row := range course {
		word, ok := byOrder[row.Order]
		if !ok || word.Status != "ready" {
			continue
		}
		readyByLemma[word.Lemma]++
		if known[row.LexemeID] {
			knownByLemma[word.Lemma]++
		}
	}
	lemmas := map[string]bool{}
	for lemma, n := range readyByLemma {
		if n > 0 && knownByLemma[lemma] == n {
			lemmas[lemma] = true
		}
	}
	share, err := speech.ShareOf(lemmas)
	if err != nil {
		return v, err
	}
	v.Share = strconv.Itoa(int(math.Round(share*100))) + "%"

	for _, row := range course {
		if _, ok := have[row.LexemeID]; ok {
			continue
		}
		if len(v.Next) == 8 {
			break
		}
		face, found, err := s.store.LexemeByID(ctx, row.LexemeID)
		if err != nil {
			return v, err
		}
		next := views.CourseNext{English: row.Lemma}
		if found {
			next.Pointed = face.TextNFC
			if face.Gloss != "" {
				next.English = face.Gloss
			}
		}
		if e, ok, err := speech.Lookup(row.Lemma); err != nil {
			return v, err
		} else if ok {
			next.Times = e.TimesLabel()
		}
		v.Next = append(v.Next, next)
	}

	if s.study != nil {
		n, reason, err := s.study.CourseWaiting(ctx, userID)
		if err != nil {
			return v, err
		}
		v.Waiting, v.Reason = n, reason
	}

	slipping, err := s.courseSlipping(ctx, userID, met)
	if err != nil {
		return v, err
	}
	v.Slipping = slipping

	if err := s.courseWeek(ctx, userID, met, &v); err != nil {
		return v, err
	}
	steps, err := speech.Map()
	if err != nil {
		return v, err
	}
	for _, step := range steps {
		v.Climb = append(v.Climb, views.SpeechRung{
			Words: strconv.Itoa(step.Words),
			Share: strconv.Itoa(int(math.Round(step.Share*100))) + "%",
		})
	}
	return v, nil
}

func (s *server) courseKnown(ctx context.Context, userID int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	for _, skill := range []string{"read", "recall"} {
		for _, state := range []learner.State{learner.Known, learner.Solid} {
			rows, err := s.listStates(ctx, userID, learner.Filter{Skill: skill, State: state})
			if err != nil {
				return nil, err
			}
			for _, w := range rows {
				out[w.LexemeID] = true
			}
		}
	}
	return out, nil
}

func (s *server) courseSlipping(ctx context.Context, userID int64, met map[int64]db.CourseLexeme) ([]views.CourseSlip, error) {
	var rows []learner.WordState
	for _, state := range []learner.State{learner.Struggling, learner.Regressed} {
		got, err := s.listStates(ctx, userID, learner.Filter{State: state})
		if err != nil {
			return nil, err
		}
		rows = append(rows, got...)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].LastSeen.After(rows[j].LastSeen) })
	z, err := s.learnerZone(ctx, userID)
	if err != nil {
		return nil, err
	}
	today := studyday.Today(s.now(), z)
	events, err := s.store.RegressionRows(ctx, userID, addDaysStr(today, -7), today, 50)
	if err != nil {
		return nil, err
	}
	cause := map[int64]string{}
	for _, e := range events {
		if _, ok := cause[e.LexemeID]; !ok {
			cause[e.LexemeID] = e.Cause
		}
	}
	var out []views.CourseSlip
	seen := map[int64]bool{}
	for _, w := range rows {
		if _, ok := met[w.LexemeID]; !ok || seen[w.LexemeID] {
			continue
		}
		seen[w.LexemeID] = true
		out = append(out, views.CourseSlip{
			Pointed: w.Text, English: w.Gloss, Cause: views.CauseLabel(cause[w.LexemeID]),
		})
		if len(out) == 10 {
			break
		}
	}
	return out, nil
}

func (s *server) listStates(ctx context.Context, userID int64, f learner.Filter) ([]learner.WordState, error) {
	var out []learner.WordState
	for {
		page, cur, err := s.model().List(ctx, userID, f)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)
		if cur == "" {
			return out, nil
		}
		f.After = cur
	}
}

func (s *server) courseWeek(ctx context.Context, userID int64, met map[int64]db.CourseLexeme, v *views.CoursePage) error {
	z, err := s.learnerZone(ctx, userID)
	if err != nil {
		return err
	}
	today := studyday.Today(s.now(), z)
	fromDay := addDaysStr(today, -7)
	from, err := time.ParseInLocation("2006-01-02", fromDay, z.Loc)
	if err != nil {
		return err
	}
	to := s.now()
	attempts, err := s.store.AttemptTimes(ctx, userID, from, to)
	if err != nil {
		return err
	}
	v.Minutes = jobs.ActiveMinutes(attempts)
	days := map[string]bool{}
	for _, a := range attempts {
		days[a.At.In(z.Loc).Format("2006-01-02")] = true
	}
	v.Days = len(days)
	v.MetWeek, err = s.store.CourseCardsCreated(ctx, userID, from, to)
	if err != nil {
		return err
	}
	events, err := s.store.RegressionRows(ctx, userID, fromDay, today, 100)
	if err != nil {
		return err
	}
	slipped := map[int64]bool{}
	for _, e := range events {
		if _, ok := met[e.LexemeID]; ok {
			slipped[e.LexemeID] = true
		}
	}
	v.Slipped = len(slipped)

	reports, err := s.store.WeeklyReportRows(ctx, userID, "1970-01-01", today, 1)
	if err != nil {
		return err
	}
	if len(reports) == 0 || reports[0].Payload == "" {
		return nil
	}
	var payload jobs.WeeklyPayload
	if err := json.Unmarshal([]byte(reports[0].Payload), &payload); err != nil {
		return nil
	}
	if payload.Course == nil {
		return nil
	}
	last := &views.CourseLastWeek{
		WeekOf: payload.WeekStart, Met: payload.Course.MetThisWeek,
		Days: payload.Course.StudyDays, Minutes: payload.Minutes,
	}
	for _, g := range payload.Course.Gained {
		last.Gained = append(last.Gained, g.Text)
	}
	for _, l := range payload.Course.Lost {
		last.Lost = append(last.Lost, views.CourseLost{Word: l.Text, Cause: views.CauseLabel(l.Cause)})
	}
	v.Last = last
	return nil
}
