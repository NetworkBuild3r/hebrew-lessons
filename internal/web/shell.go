package web

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/mcpstore"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/roots"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/speech"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
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
	// SPEC-011 does not offer that lesson on Today. The field stays so the
	// lesson card can still be chosen if a later switch turns lessons back on.
	HasLesson bool
	// PlacementOn and CheckInOn are the config switches (ADR-0033 D12). Off,
	// those actions are not offered.
	PlacementOn bool
	CheckInOn   bool
	// Askable is true when a session built from cards that can be asked today
	// has at least one card. Its length is Session.Minutes.
	Askable bool
	// CourseWaiting is how many new common words the next sitting would add.
	CourseWaiting int
}

// ChooseTodayAction is the one next action Today offers. With the placement
// switch on, an open or never-started placement comes first, as it does today.
// With the check-in switch on, a due check-in comes next. Otherwise a session
// of cards that can be asked today is the action. Nothing due is ActionNone:
// one line, and no empty session. Lessons are not a Today action
// (INIT-038/SPEC-011).
func ChooseTodayAction(in TodayInputs) views.TodayAction {
	switch {
	case in.PlacementOn && in.PlacementActive:
		return views.ActionResumePlacement
	case in.PlacementOn && in.PlacementNever:
		return views.ActionStartPlacement
	case in.CheckInOn && in.CheckinDue:
		return views.ActionCheckin
	case in.Askable || in.CourseWaiting > 0:
		return views.ActionSession
	}
	return views.ActionNone
}

// nextTodayLesson is the unfinished lesson Today offers. An untouched learner
// unit with ModeWrite blocks > 0 (SPEC-009's WriteModeBlocks count, including
// path shape checks) is skipped; a started unit with blocks is still offered
// so the learner can finish it. Shared units are never skipped for blocks.
// Reported units stay hidden as before. When every forward unit is skipped,
// ok is false and Today falls through to review. INIT-037/SPEC-010.
func (s *server) nextTodayLesson(ctx context.Context, userID int64) (db.CourseLesson, bool, error) {
	if s == nil || s.store == nil {
		return db.CourseLesson{}, false, nil
	}
	rows, err := s.store.UnitsInScope(ctx, userID)
	if err != nil {
		return db.CourseLesson{}, false, err
	}
	hidden, err := s.store.HiddenUnitIDs(ctx, userID)
	if err != nil {
		return db.CourseLesson{}, false, err
	}
	hiddenSet := map[int64]bool{}
	for _, id := range hidden {
		hiddenSet[id] = true
	}
	finished, err := s.store.FinishedLessons(ctx, userID)
	if err != nil {
		return db.CourseLesson{}, false, err
	}
	lk := mcpstore.New(s.store).WithVoices(s.voices)
	var earlierStories []string
	var prevTypes []string
	for _, row := range rows {
		if row.Status != "published" || hiddenSet[row.ID] {
			continue
		}
		snap, err := s.store.LoadUnit(ctx, row.ID, nil)
		if err != nil {
			return db.CourseLesson{}, false, err
		}
		if snap.Status != "published" {
			continue
		}
		doc := snap.Doc
		blocks, _ := units.WriteModeBlocks(ctx, &doc, lk, earlierStories, prevTypes)
		done := finished[row.ID]
		unfinishedIx := -1
		for ix := range doc.Lessons {
			if _, ok := done[ix]; ok {
				continue
			}
			unfinishedIx = ix
			break
		}
		if unfinishedIx >= 0 {
			skip, err := s.skipBlockedUntouchedLearnerUnit(ctx, userID, row.Scope, row.ID, blocks)
			if err != nil {
				return db.CourseLesson{}, false, err
			}
			if !skip {
				lesson, err := s.store.CourseLesson(ctx, userID, row.ID, unfinishedIx)
				if err != nil {
					return db.CourseLesson{}, false, err
				}
				return lesson, true, nil
			}
		}
		if story := units.FirstStoryText(&doc); story != "" {
			earlierStories = append(earlierStories, story)
		}
		prevTypes = units.ActivityTypes(&doc)
	}
	return db.CourseLesson{}, false, nil
}

// skipBlockedUntouchedLearnerUnit is true when Today must not start this unit:
// learner-scoped, never opened, and ModeWrite blocks > 0. INIT-037/SPEC-010.
func (s *server) skipBlockedUntouchedLearnerUnit(ctx context.Context, userID int64, scope string, unitID int64, blocks int) (bool, error) {
	if blocks <= 0 || !strings.HasPrefix(scope, "learner:") {
		return false, nil
	}
	untouched, err := s.store.IsUntouched(ctx, userID, unitID)
	if err != nil {
		return false, err
	}
	return untouched, nil
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
	mux.HandleFunc("GET /learn/alefbet", page(s.serveAlefbet))
	mux.HandleFunc("GET /learn/vowels", page(s.serveVowels))
	mux.HandleFunc("GET /learn/verbs", page(s.serveBinyanIndex))
	mux.HandleFunc("GET /learn/verbs/word/{lemma}", page(s.serveVerb))
	mux.HandleFunc("GET /learn/customs", page(s.serveCustoms))
	mux.HandleFunc("GET /learn/{slug}", page(s.serveLearnOne))
	mux.HandleFunc("GET /_hx/learn/{slug}", page(s.serveLearnSheet))
	mux.HandleFunc("GET /roots", page(s.serveRoots))
	mux.HandleFunc("GET /roots/{root}", page(s.serveRoot))
	mux.HandleFunc("GET /tutor", page(s.serveTutor))
	mux.HandleFunc("POST /_hx/tutor/{id}/study", page(s.postTutorStudy))
	mux.HandleFunc("GET /lessons", page(s.serveLessons))
	mux.HandleFunc("GET /lessons/{number}", page(s.serveCourseLesson))
	mux.HandleFunc("POST /_hx/lessons/{number}/study", page(s.postCourseLesson))
	mux.HandleFunc("GET /me", page(s.serveMe))
	mux.HandleFunc("GET /me/speaking", page(s.serveSpeaking))
	mux.HandleFunc("GET /speech", page(s.serveSpeech))
	mux.HandleFunc("POST /speech", page(s.serveSpeech))
	mux.HandleFunc("GET /me/words", page(s.serveWords))
	mux.HandleFunc("GET /me/verbs", page(s.serveVerbs))
	mux.HandleFunc("GET /me/mistakes", page(s.serveMistakes))
	mux.HandleFunc("GET /words/{id}", page(s.serveWord))
	mux.HandleFunc("GET /isaiah", page(s.serveIsaiah))
	mux.HandleFunc("GET /isaiah/{chapter}/{verse}", page(s.serveIsaiahVerse))
	mux.HandleFunc("GET /admin/isaiah/{chapter}/{verse}", page(s.serveAdminIsaiah))
	mux.HandleFunc("POST /admin/isaiah/{chapter}/{verse}", page(s.serveAdminIsaiah))
	mux.HandleFunc("GET /lessons/{id}/report", page(s.serveReportForm))
	mux.HandleFunc("POST /lessons/{id}/report", page(s.postReport))
	mux.HandleFunc("POST /_hx/review/band", page(s.postBandReview))
	mux.HandleFunc("POST /_hx/review/course", page(s.postCourseReview))
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
		out = append(out, views.SectionLink{Href: ui.Href("/learn"), Label: "Learn", Icon: "tiles", ID: "learn"})
	}
	if s.showRoots {
		showRoots, err := roots.Any(ctx, s.store)
		if err != nil {
			return nil, err
		}
		if showRoots {
			out = append(out, views.SectionLink{Href: ui.Href("/roots"), Label: "Roots", Icon: "branch", ID: "roots"})
		}
	}
	if s.showIsaiah {
		n, err := s.store.CheckedVerseCount(ctx)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			out = append(out, views.SectionLink{Href: ui.Href("/isaiah"), Label: "Isaiah", Icon: "flag", ID: "isaiah"})
		}
	}
	out = append(out, views.SectionLink{Href: ui.Href("/tutor"), Label: "Tutor", Icon: "chat", ID: "tutor"})
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
		rows, summary, err := s.courseLessonRows(r.Context(), user.ID)
		if err != nil {
			serverError(w)
			return
		}
		v.Rows, v.Summary = rows, summary
	}
	render(w, r, views.Lessons(v))
}

// courseLessonRows is the Lessons list: numbered lessons of five common words in
// frequency order, with the first one not yet studied marked next.
func (s *server) courseLessonRows(ctx context.Context, userID int64) ([]views.LessonRow, string, error) {
	lessons, err := speech.CourseLessons()
	if err != nil {
		return nil, "", err
	}
	if len(lessons) == 0 {
		return nil, "", nil
	}
	course, err := s.store.CourseLexemes(ctx)
	if err != nil {
		return nil, "", err
	}
	have, err := s.store.CourseCardLexemes(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	byOrder := map[int]db.CourseLexeme{}
	for _, row := range course {
		byOrder[row.Order] = row
	}
	// A lesson whose words are not all in the database cannot be studied, so it is
	// not listed. Numbering is over the lessons that are listed.
	var kept []speech.CourseLesson
	for _, lesson := range lessons {
		complete := true
		for _, w := range lesson.Words {
			if _, ok := byOrder[w.Order]; !ok {
				complete = false
				break
			}
		}
		if complete {
			kept = append(kept, lesson)
		}
	}
	rows := make([]views.LessonRow, 0, len(kept))
	next := 0
	for i, lesson := range kept {
		row := views.LessonRow{
			Number: i + 1,
			Title:  "Lesson " + strconv.Itoa(i+1),
			Href:   "/lessons/" + strconv.Itoa(i+1),
			State:  "later",
		}
		var words, glosses []string
		for _, w := range lesson.Words {
			words = append(words, w.Pointed)
			glosses = append(glosses, w.Gloss)
			row.Lessons++
			stored := byOrder[w.Order]
			if have[stored.LexemeID] {
				row.Done++
			}
		}
		row.Words = strings.Join(words, " · ")
		row.Gloss = strings.Join(glosses, " · ")
		if row.Lessons > 0 && row.Done >= row.Lessons {
			row.State = "done"
		}
		rows = append(rows, row)
	}
	for i := range rows {
		if rows[i].State != "done" {
			rows[i].State = "next"
			next = rows[i].Number
			break
		}
	}
	if len(rows) == 0 {
		return nil, "", nil
	}
	summary := "All " + strconv.Itoa(len(rows)) + " lessons done."
	if next > 0 {
		summary = "Lesson " + strconv.Itoa(next) + " of " + strconv.Itoa(len(rows))
	}
	return rows, summary, nil
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
	// One batch of this learner's wrong answers in the unit. Right answers,
	// other units, and anyone else's answers stay. INIT-038/SPEC-009.
	if _, err := evidence.New(s.store, nil, nil).VoidUnitWrong(r.Context(), user.ID, id, db.LearnerScope(user.ID)); err != nil {
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
	z, err := s.learnerZone(ctx, userID)
	if err != nil {
		return err
	}
	q := &studyQueue{userID: userID, sess: built, builtAt: now, rollover: studyday.DueCutoff(now, z)}
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
	if s == nil || !s.showCheckIn {
		notFound(w, r)
		return
	}
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
// so one broken read does not take away the learner's next action. A failure building
// the session is the caller's error: Today must not claim nothing is due when it
// could not read the queue.
func (s *server) fillToday(r *http.Request, page *views.Home) error {
	t := &page.Today
	t.Reported = r.URL.Query().Get("reported") == "1"
	in := TodayInputs{
		Due:         page.Due,
		HasLesson:   page.Lesson,
		PlacementOn: s != nil && s.showPlacement,
		CheckInOn:   s != nil && s.showCheckIn,
	}
	user, ok := auth.CurrentUser(r.Context())
	signedIn := ok && user != nil
	// A nil Placement is a learner with no run. The switch has to be on before
	// that becomes the action. A request with no learner has no run to offer.
	if in.PlacementOn {
		in.PlacementNever = signedIn && page.Placement == nil
		in.PlacementActive = page.Placement != nil && page.Placement.Active
	}
	in.LessonInProgress = page.Lesson && t.Step > 0

	if signedIn && s != nil && s.store != nil {
		ctx := r.Context()
		if in.CheckInOn && s.recheck != nil && !in.PlacementNever && !in.PlacementActive {
			s.fillCheckin(ctx, user.ID, t, &in)
		}
		if s.study != nil {
			n, _, err := s.study.CourseWaiting(ctx, user.ID)
			if err != nil {
				slog.Warn("today: course", "err", err)
			} else {
				in.CourseWaiting = n
				t.CourseNew = n
			}
		}
		if page.Due > 0 && s.study != nil {
			minutes, askable, err := s.askableMinutes(ctx, user.ID)
			if err != nil {
				return err
			}
			if askable {
				t.SessionMinutes = minutes
				in.Askable = true
			}
		}
		s.fillModelCards(ctx, user.ID, t)
		if _, line, err := s.canSayView(ctx, user.ID); err != nil {
			slog.Warn("today: can say", "err", err)
		} else {
			t.CanSayLine = line
		}
	}
	t.Action = ChooseTodayAction(in)
	return nil
}

// askableMinutes builds the session SPEC-006 already sizes, from cards that
// can be asked today, and returns Session.Minutes. ok is false when that
// session has no card, so Today does not offer an empty sitting.
// BuildSessionFor does not introduce new cards; the lexeme ids are the due set.
func (s *server) askableMinutes(ctx context.Context, userID int64) (minutes int, ok bool, err error) {
	due, err := s.store.ListDueCards(ctx, userID, s.now())
	if err != nil || len(due) == 0 {
		return 0, false, err
	}
	seen := map[int64]struct{}{}
	ids := make([]int64, 0, len(due))
	for _, card := range due {
		if _, dup := seen[card.LexemeID]; dup {
			continue
		}
		seen[card.LexemeID] = struct{}{}
		ids = append(ids, card.LexemeID)
	}
	built, err := s.study.BuildSessionFor(ctx, userID, ids)
	if err != nil {
		return 0, false, err
	}
	_, total, _ := built.Place()
	if total == 0 {
		return 0, false, nil
	}
	return built.Minutes(), true, nil
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
	z, err := s.learnerZone(ctx, userID)
	if err != nil {
		slog.Warn("today: learner zone", "err", err)
		return
	}
	since := addDaysStr(studyday.Today(s.now(), z), -(slipShowDays - 1))
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
