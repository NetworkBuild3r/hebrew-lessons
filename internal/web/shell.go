package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/roots"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// The app shell (INIT-015/SPEC-017): what Today offers, the Lessons list, Me, the report
// path, and the two routes Today's buttons post to.

// PlacementEngine is the part of *placement.Engine the shell uses. It is defined here so
// this package does not depend on how the placement screens are wired: app.go passes the
// engine as Options.Placement, and a nil value turns the check-in off.
//
//   - RecheckDue says whether the 3-minute check-in is due (28 days after the last
//     placement or check-in).
//   - Latest is the learner's newest finished result, which says which skill is lower.
//   - NewRecheck opens the check-in run. The placement screen resumes an open run.
type PlacementEngine interface {
	RecheckDue(ctx context.Context, userID int64) (placement.RecheckStatus, error)
	Latest(ctx context.Context, userID int64) (placement.RunResult, bool, error)
	NewRecheck(ctx context.Context, userID int64, skill string) (int64, error)
}

var _ PlacementEngine = (*placement.Engine)(nil)

const (
	// slipShowDays is how many days a band-slip alert stays on Today after the daily job
	// writes it: the day of the event and the six after.
	slipShowDays = 7
	// watchMax is how many struggling words "Words to watch" lists.
	watchMax = 5
	// reportNoteMax is the longest report note. The database refuses a longer one too.
	reportNoteMax = db.MaxReportNoteRunes
)

// TodayInputs is everything ChooseTodayAction reads. Each field is a fact about the
// learner right now, so the choice is a pure function and a table test covers it.
type TodayInputs struct {
	// PlacementNever is true when the learner has never started a placement.
	PlacementNever bool
	// PlacementActive is true when a placement (or a check-in) is open.
	PlacementActive bool
	// LessonInProgress is true when the learner is partway through a lesson.
	LessonInProgress bool
	// CheckinDue is true when Engine.RecheckDue says so.
	CheckinDue bool
	// Due is how many reviews are due.
	Due int
	// HasLesson is true when a published lesson in the learner's scope is waiting.
	HasLesson bool
}

// ChooseTodayAction is the one next action Today offers, in this order: resume a
// placement, start a placement, continue a lesson, take the check-in, open the next
// lesson, then do the due reviews. A learner with none of these has ActionNone and no
// gold button. Reviews stay on the page beside a lesson, so a finished lesson can
// continue. The check-in sits below a lesson in progress, so it never takes a learner
// out of one.
func ChooseTodayAction(in TodayInputs) views.TodayAction {
	switch {
	case in.PlacementActive:
		return views.ActionResumePlacement
	case in.PlacementNever:
		return views.ActionStartPlacement
	case in.LessonInProgress:
		return views.ActionContinueLesson
	case in.CheckinDue:
		return views.ActionCheckin
	case in.HasLesson:
		return views.ActionNextLesson
	case in.Due > 0:
		return views.ActionReviews
	}
	return views.ActionNone
}

// mountShell registers the shell's routes. All of them need a signed-in learner, and every
// POST goes through the CSRF check in front of the mux.
func (s *server) mountShell(mux *http.ServeMux, tokens CSRFToken, gate, secure bool) {
	page := func(fn func(http.ResponseWriter, *http.Request, string, string)) http.HandlerFunc {
		return gated(gate, secure, func(w http.ResponseWriter, r *http.Request) {
			fn(w, r, chooseLang(w, r, secure), csrfValue(tokens, r))
		})
	}
	mux.HandleFunc("GET /learn", page(s.serveLearn))
	mux.HandleFunc("GET /learn/{slug}", page(s.serveLearnOne))
	mux.HandleFunc("GET /_hx/learn/{slug}", page(s.serveLearnSheet))
	mux.HandleFunc("GET /roots", page(s.serveRoots))
	mux.HandleFunc("GET /roots/{root}", page(s.serveRoot))
	mux.HandleFunc("GET /lessons", page(s.serveLessons))
	mux.HandleFunc("GET /me", page(s.serveMe))
	mux.HandleFunc("GET /me/words", page(s.serveWords))
	mux.HandleFunc("GET /me/verbs", page(s.serveVerbs))
	mux.HandleFunc("GET /me/mistakes", page(s.serveMistakes))
	mux.HandleFunc("GET /isaiah", page(s.serveIsaiah))
	mux.HandleFunc("GET /isaiah/{chapter}/{verse}", page(s.serveIsaiahVerse))
	mux.HandleFunc("GET /admin/isaiah/{chapter}/{verse}", page(s.serveAdminIsaiah))
	mux.HandleFunc("POST /admin/isaiah/{chapter}/{verse}", page(s.serveAdminIsaiah))
	mux.HandleFunc("GET /lessons/{id}/report", page(s.serveReportForm))
	mux.HandleFunc("POST /lessons/{id}/report", page(s.postReport))
	mux.HandleFunc("POST /_hx/review/band", page(s.postBandReview))
	mux.HandleFunc("POST /_hx/placement/recheck", page(s.postRecheck))
}

// pageFor is the shell page for the signed-in learner, so Hebrew copy addresses them as
// they asked. A signed-out request (the ungated test router) gets the masculine default.
func (s *server) pageFor(r *http.Request, lang, csrf, current string) (views.Page, *auth.User, error) {
	gender := views.AddressMasculine
	user, ok := auth.CurrentUser(r.Context())
	if !ok {
		user = nil
	}
	if user != nil {
		g, err := s.addressGender(r.Context(), user.ID)
		if err != nil {
			return views.Page{}, nil, err
		}
		gender = g
	}
	ui := views.UIFor(lang, gender)
	page := views.Page{UI: ui, Title: ui.DocTitle(current), CSRF: csrf, Current: current}
	links, err := s.sectionLinks(r.Context(), ui)
	if err != nil {
		return views.Page{}, nil, err
	}
	page.Sections = links
	return page, user, nil
}

// withSections adds Learn, Roots, and Isaiah when those pages exist.
func (s *server) withSections(r *http.Request, page views.Page) (views.Page, error) {
	links, err := s.sectionLinks(r.Context(), page.UI)
	if err != nil {
		return page, err
	}
	page.Sections = links
	return page, nil
}

func (s *server) sectionLinks(ctx context.Context, ui views.UI) ([]views.SectionLink, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	var out []views.SectionLink
	published, err := s.store.PublishedExplainers(ctx)
	if err != nil {
		return nil, err
	}
	if len(published) > 0 {
		out = append(out, views.SectionLink{Href: ui.Href("/learn"), Label: "Learn", Icon: "book", ID: "learn"})
	}
	showRoots, err := roots.Any(ctx, s.store)
	if err != nil {
		return nil, err
	}
	if showRoots {
		out = append(out, views.SectionLink{Href: ui.Href("/roots"), Label: "Roots", Icon: "layers", ID: "roots"})
	}
	n, err := s.store.CheckedVerseCount(ctx)
	if err != nil {
		return nil, err
	}
	if n > 0 {
		out = append(out, views.SectionLink{Href: ui.Href("/isaiah"), Label: "Isaiah", Icon: "flag", ID: "isaiah"})
	}
	return out, nil
}

func (s *server) serveMe(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, user, err := s.pageFor(r, lang, csrf, "me")
	if err != nil {
		serverError(w)
		return
	}
	v := views.MeView{Page: page}
	if user != nil {
		v.Name = user.Email
	}
	render(w, r, views.Me(v))
}

func (s *server) serveLessons(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, user, err := s.pageFor(r, lang, csrf, "lessons")
	if err != nil {
		serverError(w)
		return
	}
	v := views.LessonsView{Page: page}
	if user != nil && s != nil && s.store != nil {
		rows, err := s.lessonRows(r.Context(), user.ID)
		if err != nil {
			serverError(w)
			return
		}
		v.Rows = rows
	}
	render(w, r, views.Lessons(v))
}

// lessonRows is the Lessons list: the learner's published units in order, with a unit they
// reported left out. The first unit with a lesson still to do is "next"; the units before
// it are done and the units after it are later.
func (s *server) lessonRows(ctx context.Context, userID int64) ([]views.LessonRow, error) {
	list, err := s.store.LearnerUnitProgress(ctx, userID)
	if err != nil {
		return nil, err
	}
	next, hasNext, err := s.store.NextCourseLesson(ctx, userID)
	if err != nil {
		return nil, err
	}
	rows := make([]views.LessonRow, 0, len(list))
	for _, u := range list {
		gloss, err := titleGloss(ctx, s.store, u.Title)
		if err != nil {
			return nil, err
		}
		row := views.LessonRow{
			Title: u.Title, Gloss: gloss, Mine: u.Scope != "shared",
			Lessons: u.Lessons, Done: u.Finished, State: "later",
		}
		switch {
		case u.Finished >= u.Lessons:
			row.State = "done"
		case hasNext && u.UnitID == next.UnitID:
			row.State = "next"
			row.Href = lessonPath(next.UnitID, next.LessonIx)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// reportTarget reads the unit id of a report route. A unit the learner cannot see (not
// published, another learner's, or one they already reported) is 404 for the form.
func (s *server) reportTarget(w http.ResponseWriter, r *http.Request) (int64, *auth.User, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	user, ok := auth.CurrentUser(r.Context())
	if err != nil || id <= 0 || !ok || user == nil || s == nil || s.store == nil {
		notFound(w, r)
		return 0, nil, false
	}
	return id, user, true
}

func (s *server) serveReportForm(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	id, user, ok := s.reportTarget(w, r)
	if !ok {
		return
	}
	title, found, err := s.visibleUnitTitle(r.Context(), user.ID, id)
	if err != nil {
		serverError(w)
		return
	}
	if !found {
		notFound(w, r)
		return
	}
	page, _, err := s.pageFor(r, lang, csrf, "lesson")
	if err != nil {
		serverError(w)
		return
	}
	render(w, r, views.ReportPage(views.ReportView{Page: page, UnitID: id, Title: title}))
}

// visibleUnitTitle finds a unit among those the learner can open.
func (s *server) visibleUnitTitle(ctx context.Context, userID, unitID int64) (string, bool, error) {
	list, err := s.store.LearnerUnitProgress(ctx, userID)
	if err != nil {
		return "", false, err
	}
	for _, u := range list {
		if u.UnitID == unitID {
			return u.Title, true, nil
		}
	}
	return "", false, nil
}

// postReport is POST /lessons/{id}/report. It needs a session and the CSRF token (the
// middleware in front of the mux checks both). It writes a unit_reports row and hides the
// unit for this learner through db.ReportUnit, then sends the learner to Today, where the
// lesson is already gone. The note is capped at 500 characters.
func (s *server) postReport(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	id, user, ok := s.reportTarget(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	reason, note := r.PostForm.Get("reason"), r.PostForm.Get("note")
	refuse := func(msg string) {
		page, _, err := s.pageFor(r, lang, csrf, "lesson")
		if err != nil {
			serverError(w)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		render(w, r, views.ReportPage(views.ReportView{Page: page, UnitID: id, Reason: reason, Note: note, Error: msg}))
	}
	switch {
	case !views.ValidReason(reason):
		refuse("Pick what went wrong.")
		return
	case len([]rune(note)) > reportNoteMax:
		refuse("That note is too long. Keep it under " + strconv.Itoa(reportNoteMax) + " characters.")
		return
	}
	_, err := s.store.ReportUnit(r.Context(), db.Report{UserID: user.ID, UnitID: id, Reason: reason, Note: note})
	switch {
	case errors.Is(err, db.ErrUnitNotInScope), errors.Is(err, db.ErrNoLearner):
		notFound(w, r)
		return
	case errors.Is(err, db.ErrBadReport):
		refuse("That report could not be saved. Check the note and try again.")
		return
	case err != nil:
		serverError(w)
		return
	}
	// The unit is hidden in ReportUnit's own transaction, so Today, at the address below,
	// no longer offers it.
	w.Header().Set("Cache-Control", "no-store")
	loc := "/?reported=1"
	if lang == "he" {
		loc = "/?lang=he&reported=1"
	}
	http.Redirect(w, r, loc, http.StatusSeeOther)
}

// postBandReview is POST /_hx/review/band. It replaces the learner's review queue with a
// review set of the words of one band that are struggling or regressed, and sends them to
// Review. When the band has no such words, or none of them has a card to review, the
// queue is left alone: the learner reaches the ordinary review, not an empty one.
func (s *server) postBandReview(w http.ResponseWriter, r *http.Request, lang, _ string) {
	back := "/review"
	if lang == "he" {
		back = "/review?lang=he"
	}
	key, user, ok := s.sessionKey(r)
	if !ok || user == nil || s == nil || s.store == nil || s.study == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	band, err := strconv.Atoi(r.PostForm.Get("band"))
	if err != nil || band < 0 || band > 4 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	ids, err := s.bandWordIDs(r.Context(), user.ID, band)
	if err != nil {
		serverError(w)
		return
	}
	if len(ids) > 0 {
		built, err := s.study.BuildSessionFor(r.Context(), user.ID, ids)
		if err != nil {
			serverError(w)
			return
		}
		if _, total, _ := built.Place(); total > 0 {
			if err := s.replaceQueue(r.Context(), key, user.ID, built); err != nil {
				serverError(w)
				return
			}
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// model is the learner model for this store. bands is a test seam; nil reads the placement deck.
func (s *server) model() *learner.Model {
	if s.bands != nil {
		return learner.New(s.store, s.store.Clock(), learner.WithBands(s.bands))
	}
	return learner.New(s.store, s.store.Clock())
}

// replaceQueue puts a built session in place of the learner's queue for this browser session.
func (s *server) replaceQueue(ctx context.Context, key string, userID int64, built *session.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	q := &studyQueue{userID: userID, sess: built, builtAt: now}
	if err := s.advance(ctx, q); err != nil {
		return err
	}
	s.rememberLocked(key, q, now)
	return nil
}

// bandWordIDs lists the words of one band that are struggling or regressed for the learner.
func (s *server) bandWordIDs(ctx context.Context, userID int64, band int) ([]int64, error) {
	model := s.model()
	var ids []int64
	for _, st := range []learner.State{learner.Struggling, learner.Regressed} {
		words, _, err := model.List(ctx, userID, learner.Filter{State: st, Band: &band, Limit: learner.MaxPageSize})
		if err != nil {
			return nil, err
		}
		for _, w := range words {
			ids = append(ids, w.LexemeID)
		}
	}
	return ids, nil
}

// postRecheck is POST /_hx/placement/recheck: open the check-in and go to the placement
// screen, which resumes an open run. Which skill it asks is chosen here, not by the form.
func (s *server) postRecheck(w http.ResponseWriter, r *http.Request, lang, _ string) {
	to := "/placement"
	if lang == "he" {
		to = "/placement?lang=he"
	}
	user, ok := auth.CurrentUser(r.Context())
	if !ok || user == nil || s == nil || s.recheck == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	skill, ok, err := s.checkinSkill(r.Context(), user.ID)
	if err != nil {
		serverError(w)
		return
	}
	if ok {
		_, err = s.recheck.NewRecheck(r.Context(), user.ID, skill)
		switch {
		case err == nil:
		case errors.Is(err, placement.ErrNotReady):
			// A run is already open. The placement screen resumes it.
		case errors.Is(err, placement.ErrNoPlacement):
			to = "/"
		default:
			serverError(w)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// checkinSkill is the skill a check-in asks: the lower of the two the placement tested,
// listening when they are level. ok is false when the learner has no finished placement.
func (s *server) checkinSkill(ctx context.Context, userID int64) (string, bool, error) {
	res, ok, err := s.recheck.Latest(ctx, userID)
	if err != nil || !ok {
		return "", false, err
	}
	l, rd := res.Listening, res.Reading
	switch {
	case l.Tested && rd.Tested && rd.Level < l.Level:
		return placement.SkillRead, true, nil
	case l.Tested:
		return placement.SkillListen, true, nil
	case rd.Tested:
		return placement.SkillRead, true, nil
	}
	return "", false, nil
}

// fillToday decides Today's one action and fills the cards beside it. A failure in a
// supporting read (the check-in, the learner model) is logged and that card is left off,
// so one broken read does not take away the learner's next action. A failure in the
// reads the action stands on (the lesson and the due count) is the caller's error.
func (s *server) fillToday(r *http.Request, page *views.Home) {
	t := &page.Today
	t.Reported = r.URL.Query().Get("reported") == "1"
	in := TodayInputs{
		Due:       page.Due,
		HasLesson: page.Lesson,
	}
	user, ok := auth.CurrentUser(r.Context())
	signedIn := ok && user != nil
	// A nil Placement is a learner with no run. The shell without a placement store sets an
	// empty value, which offers nothing, and a request with no learner has no run to offer.
	in.PlacementNever = signedIn && page.Placement == nil
	in.PlacementActive = page.Placement != nil && page.Placement.Active
	in.LessonInProgress = page.Lesson && t.Step > 0
	t.DailyLine = views.DailyLineFor(page.Due, session.SessionCap)

	if signedIn && s != nil && s.store != nil {
		ctx := r.Context()
		if s.recheck != nil && !in.PlacementNever && !in.PlacementActive {
			s.fillCheckin(ctx, user.ID, t, &in)
		}
		s.fillModelCards(ctx, user.ID, t)
		if _, line, err := s.canSayView(ctx, user.ID); err != nil {
			slog.Warn("today: can say", "err", err)
		} else {
			t.CanSayLine = line
		}
	}
	t.Action = ChooseTodayAction(in)
}

// fillCheckin asks the engine whether the check-in is due and which skill it would ask.
func (s *server) fillCheckin(ctx context.Context, userID int64, t *views.TodayView, in *TodayInputs) {
	st, err := s.recheck.RecheckDue(ctx, userID)
	if err != nil {
		slog.Warn("today: check-in due", "err", err)
		return
	}
	if !st.Due {
		return
	}
	skill, ok, err := s.checkinSkill(ctx, userID)
	if err != nil {
		slog.Warn("today: check-in skill", "err", err)
		return
	}
	if !ok {
		return
	}
	in.CheckinDue = true
	t.CheckinSkill = skill
	t.CheckinLate = st.Late
	t.CheckinWeeks = int(s.now().Sub(st.LastAt).Hours() / 24 / 7)
}

// fillModelCards fills "Words to watch" and the band-slip alerts from the learner model.
func (s *server) fillModelCards(ctx context.Context, userID int64, t *views.TodayView) {
	model := s.model()
	words, _, err := model.List(ctx, userID, learner.Filter{State: learner.Struggling, Limit: learner.MaxPageSize})
	if err != nil {
		slog.Warn("today: words to watch", "err", err)
	} else {
		t.Watch = watchWords(words, watchMax)
	}
	since := s.now().UTC().AddDate(0, 0, -(slipShowDays - 1)).Format("2006-01-02")
	slips, err := s.store.RecentBandSlips(ctx, userID, since)
	if err != nil {
		slog.Warn("today: band slips", "err", err)
		return
	}
	seen := map[int]bool{}
	for _, sl := range slips {
		if seen[sl.Band] {
			continue
		}
		seen[sl.Band] = true
		ids, err := s.bandWordIDs(ctx, userID, sl.Band)
		if err != nil {
			slog.Warn("today: band words", "err", err)
			continue
		}
		t.Slips = append(t.Slips, views.SlipAlert{Level: sl.Band + 1, Band: sl.Band, Words: len(ids), Day: sl.Day})
	}
}

// watchWords picks up to n struggling words, worst first: the lowest accuracy in the
// failing skill, then the most recent Agains, then the word seen last. Each carries the
// skill that fails, which is the weakest skill of the word with evidence.
func watchWords(words []learner.WordState, n int) []views.WatchWord {
	type scored struct {
		w     views.WatchWord
		acc   float64
		again int
		seen  time.Time
	}
	var all []scored
	for _, w := range words {
		sk, ok := failingSkill(w)
		if !ok {
			continue
		}
		all = append(all, scored{
			w:   views.WatchWord{Text: w.Text, Gloss: w.Gloss, Skill: sk.Skill},
			acc: sk.Accuracy, again: sk.AgainRecent, seen: w.LastSeen,
		})
	}
	sort.SliceStable(all, func(i, j int) bool {
		a, b := all[i], all[j]
		switch {
		case a.acc != b.acc:
			return a.acc < b.acc
		case a.again != b.again:
			return a.again > b.again
		}
		return a.seen.After(b.seen)
	})
	out := make([]views.WatchWord, 0, min(n, len(all)))
	for _, s := range all[:min(n, len(all))] {
		out = append(out, s.w)
	}
	return out
}

// failingSkill is the skill of a word that is struggling (or regressed): the weakest, then
// the one with the lowest accuracy, then the first in the order of learner.Skills.
func failingSkill(w learner.WordState) (learner.SkillState, bool) {
	var best learner.SkillState
	found := false
	for _, sk := range w.Skills {
		if sk.State != learner.Struggling && sk.State != learner.Regressed {
			continue
		}
		if !found || weaker(sk, best) {
			best, found = sk, true
		}
	}
	return best, found
}

func weaker(a, b learner.SkillState) bool {
	if a.State != b.State {
		return a.State == learner.Struggling
	}
	return a.Accuracy < b.Accuracy
}
