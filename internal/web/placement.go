package web

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// The placement screens (INIT-015/SPEC-012). The v3 engine deals the test; this file serves it. The
// engine takes the learner's id from the session on every call, and treats a run that is not theirs as
// not found, so this file never looks at whose run it is.
//
// D1: nothing on the way through says whether an answer was right. The reply to an answer is the
// "picked" screen, built from the item that was open and the option that was tapped, and nothing else.
// The engine's Step carries no correctness, and this file reads none.

// placementDeck is the part of placement.Engine the screens call.
type placementDeck interface {
	Start(ctx context.Context, userID int64) (int64, error)
	Next(ctx context.Context, userID, runID int64) (placement.Step, error)
	Answer(ctx context.Context, userID, runID int64, itemID string, in placement.AnswerIn) (placement.Step, error)
	StartTyping(ctx context.Context, userID, runID int64) (placement.Step, error)
	Replace(ctx context.Context, userID, runID int64, itemID string) (placement.Step, error)
	Finish(ctx context.Context, userID, runID int64) (placement.RunResult, error)
	Result(ctx context.Context, userID, runID int64) (placement.RunResult, error)
	Latest(ctx context.Context, userID int64) (placement.RunResult, bool, error)
}

// placementRuns is the store read for the Today card, the resume screen, and the progress count.
type placementRuns interface {
	OpenPlacementRun(ctx context.Context, userID int64) (db.PlacementRun, bool, error)
	LatestPlacement(ctx context.Context, userID int64) (db.Placement, bool, error)
	ListPlacementItems(ctx context.Context, runID int64) ([]db.PlacementItem, error)
}

var (
	_ placementDeck = (*placement.Engine)(nil)
	_ placementRuns = (*db.Store)(nil)
)

// maxTyped is the longest typed answer the form accepts, in letters. The longest deck word is far shorter.
const maxTyped = 64

type placementHTTP struct {
	deck   placementDeck
	runs   placementRuns
	tokens CSRFToken
	secure bool
}

// mountPlacement registers the placement routes. Reads are page URLs. Writes are POSTs under /_hx/ and
// are CSRF checked by the middleware in front of the mux (ADR-0016).
func mountPlacement(mux *http.ServeMux, tokens CSRFToken, gate, secure bool, deck placementDeck, runs placementRuns) {
	h := &placementHTTP{deck: deck, runs: runs, tokens: tokens, secure: secure}
	mux.HandleFunc("GET /placement", gated(gate, secure, h.page))
	mux.HandleFunc("GET /placement/review", gated(gate, secure, h.review))
	mux.HandleFunc("POST /_hx/placement/start", h.start)
	mux.HandleFunc("POST /_hx/placement/answer", h.answer)
	mux.HandleFunc("POST /_hx/placement/next", h.next)
	mux.HandleFunc("POST /_hx/placement/typing", h.typing)
	mux.HandleFunc("POST /_hx/placement/finish", h.finish)
	mux.HandleFunc("POST /_hx/placement/replace", h.replace)
}

// todayPlacement is the Today card.
// No store returns a non-nil inactive value so the placeholder shell stays
// clear. No row returns nil: the learner has never taken the check.
// An open run is Active. A finished run is Finished and the card stays off.
func todayPlacement(r *http.Request, runs placementRuns) (*views.PlacementVM, error) {
	if runs == nil {
		return &views.PlacementVM{}, nil
	}
	user, ok := auth.CurrentUser(r.Context())
	if !ok || user == nil {
		return &views.PlacementVM{}, nil
	}
	row, found, err := runs.LatestPlacement(r.Context(), user.ID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return &views.PlacementVM{
		Active:              !row.Finished,
		Finished:            row.Finished,
		CardsRated:          row.CardsRated,
		Level:               row.Level,
		EstimatedKnownWords: row.EstimatedKnownWords,
		TakenAt:             row.TakenAt,
	}, nil
}

// page is GET /placement. It reads and deals nothing. A learner with answers in an open run lands on the
// resume screen, one with a finished run on the results, and anyone else on the intro.
func (h *placementHTTP) page(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.CurrentUser(r.Context())
	if h.deck == nil || h.runs == nil || !ok || user == nil {
		h.writePage(w, r, h.base(w, r, views.PlaceIntro, 0))
		return
	}
	open, found, err := h.runs.OpenPlacementRun(r.Context(), user.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError)
		return
	}
	if found {
		items, err := h.runs.ListPlacementItems(r.Context(), open.ID)
		if err != nil {
			fail(w, http.StatusInternalServerError)
			return
		}
		if placementAnswered(items) > 0 {
			h.writePage(w, r, h.base(w, r, views.PlaceResume, open.ID))
			return
		}
		h.writePage(w, r, h.base(w, r, views.PlaceIntro, 0))
		return
	}
	res, found, err := h.deck.Latest(r.Context(), user.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError)
		return
	}
	if found {
		h.writePage(w, r, h.resultsView(w, r, res))
		return
	}
	h.writePage(w, r, h.base(w, r, views.PlaceIntro, 0))
}

// review is GET /placement/review: every miss of one finished run, with its right answer. The run is the
// one in ?run=, or the learner's latest. The engine refuses another learner's run as not found.
func (h *placementHTTP) review(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.CurrentUser(r.Context())
	if h.deck == nil || !ok || user == nil {
		http.Redirect(w, r, views.NewPage(chooseLang(w, r, h.secure), "", "placement").Href("/placement"), http.StatusSeeOther)
		return
	}
	var res placement.RunResult
	if raw := r.URL.Query().Get("run"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			fail(w, http.StatusBadRequest)
			return
		}
		if res, err = h.deck.Result(r.Context(), user.ID, id); err != nil {
			if errors.Is(err, placement.ErrNotReady) {
				http.Redirect(w, r, "/placement", http.StatusSeeOther)
				return
			}
			failPlacement(w, err)
			return
		}
	} else {
		var found bool
		var err error
		if res, found, err = h.deck.Latest(r.Context(), user.ID); err != nil {
			fail(w, http.StatusInternalServerError)
			return
		} else if !found {
			http.Redirect(w, r, "/placement", http.StatusSeeOther)
			return
		}
	}
	v := h.base(w, r, views.PlaceReview, res.RunID)
	v.Review = placementReview(res)
	v.Page.Title = "Review · " + views.AppName
	h.writePage(w, r, v)
}

// start is POST /_hx/placement/start. It opens the learner's run, or returns the one they left, and
// shows its next screen.
func (h *placementHTTP) start(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok || !h.ready(w) || !readForm(w, r) {
		return
	}
	runID, err := h.deck.Start(r.Context(), user.ID)
	if err != nil {
		failPlacement(w, err)
		return
	}
	step, err := h.deck.Next(r.Context(), user.ID, runID)
	if err != nil {
		failPlacement(w, err)
		return
	}
	h.renderStep(w, r, user.ID, runID, step, stepOpts{})
}

// next is POST /_hx/placement/next: the screen after the picked one, after a break, and on resume. It
// deals nothing the learner has not been dealt: the engine returns the open item again.
func (h *placementHTTP) next(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok || !h.ready(w) || !readForm(w, r) {
		return
	}
	runID, ok := postedRun(w, r)
	if !ok {
		return
	}
	step, err := h.deck.Next(r.Context(), user.ID, runID)
	if err != nil {
		failPlacement(w, err)
		return
	}
	h.renderStep(w, r, user.ID, runID, step, stepOpts{cont: r.PostForm.Get("cont") == "1"})
}

// answer is POST /_hx/placement/answer. The reply to a pick is the picked screen: the chosen option is
// Selected, the others dim, and it says the answer is saved. It does not depend on the outcome. A post
// for an item that is not the open one (a double tap, an old tab) shows where the run is instead.
func (h *placementHTTP) answer(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok || !h.ready(w) || !readForm(w, r) {
		return
	}
	runID, ok := postedRun(w, r)
	if !ok {
		return
	}
	token := r.PostForm.Get("item")
	typed := strings.TrimSpace(r.PostForm.Get("typed"))
	if utf8.RuneCountInString(typed) > maxTyped {
		fail(w, http.StatusBadRequest)
		return
	}
	in := placement.AnswerIn{Option: r.PostForm.Get("option"), Typed: typed, DontKnow: r.PostForm.Get("dont_know") == "1"}

	cur, err := h.deck.Next(r.Context(), user.ID, runID)
	if err != nil {
		failPlacement(w, err)
		return
	}
	if cur.Item == nil || cur.Item.ID != token {
		h.renderStep(w, r, user.ID, runID, cur, stepOpts{})
		return
	}
	if cur.Item.Kind == placement.KindType && in.Typed == "" && !in.DontKnow {
		// A typing answer with nothing typed is not an answer: show the same item.
		h.renderStep(w, r, user.ID, runID, cur, stepOpts{})
		return
	}
	step, err := h.deck.Answer(r.Context(), user.ID, runID, token, in)
	switch {
	case errors.Is(err, placement.ErrWrongItem):
		// Another request answered first. Show where the run is.
		if cur, err = h.deck.Next(r.Context(), user.ID, runID); err != nil {
			failPlacement(w, err)
			return
		}
		h.renderStep(w, r, user.ID, runID, cur, stepOpts{})
		return
	case err != nil:
		failPlacement(w, err)
		return
	}
	if cur.Item.Kind == placement.KindType {
		h.renderStep(w, r, user.ID, runID, step, stepOpts{})
		return
	}
	v, ok := h.itemView(w, r, runID, *cur.Item, stepOpts{})
	if !ok {
		return
	}
	v.Phase = views.PlacePicked
	for i := range v.Options {
		if v.Options[i].ID == in.Option && !in.DontKnow {
			v.Options[i].State = views.AnswerSelected
		} else {
			v.Options[i].Dim = true
		}
	}
	v.AudioURL = ""
	h.write(w, r, v)
}

// typing is POST /_hx/placement/typing: the learner takes the optional typing part.
func (h *placementHTTP) typing(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok || !h.ready(w) || !readForm(w, r) {
		return
	}
	runID, ok := postedRun(w, r)
	if !ok {
		return
	}
	step, err := h.deck.StartTyping(r.Context(), user.ID, runID)
	if err != nil {
		failPlacement(w, err)
		return
	}
	h.renderStep(w, r, user.ID, runID, step, stepOpts{})
}

// finish is POST /_hx/placement/finish: skip the typing part, or stop it, and see the results.
func (h *placementHTTP) finish(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok || !h.ready(w) || !readForm(w, r) {
		return
	}
	runID, ok := postedRun(w, r)
	if !ok {
		return
	}
	res, err := h.deck.Finish(r.Context(), user.ID, runID)
	if err != nil {
		failPlacement(w, err)
		return
	}
	h.write(w, r, h.resultsView(w, r, res))
}

// replace is POST /_hx/placement/replace: the clip of a listening item would not play, so the engine
// deals another word in its place. When it cannot, the same item comes back and says its clip is broken.
func (h *placementHTTP) replace(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok || !h.ready(w) || !readForm(w, r) {
		return
	}
	runID, ok := postedRun(w, r)
	if !ok {
		return
	}
	tries, err := strconv.Atoi(r.PostForm.Get("tries"))
	if err != nil || tries < 1 {
		tries = 1
	}
	if tries > views.PlaceMaxReplaceTries {
		tries = views.PlaceMaxReplaceTries
	}
	step, err := h.deck.Replace(r.Context(), user.ID, runID, r.PostForm.Get("item"))
	switch {
	case err == nil:
		h.renderStep(w, r, user.ID, runID, step, stepOpts{cont: true, tries: tries})
	case errors.Is(err, placement.ErrNoAudio), errors.Is(err, placement.ErrNotReady), errors.Is(err, placement.ErrWrongItem):
		cur, nerr := h.deck.Next(r.Context(), user.ID, runID)
		if nerr != nil {
			failPlacement(w, nerr)
			return
		}
		h.renderStep(w, r, user.ID, runID, cur, stepOpts{cont: true, tries: views.PlaceMaxReplaceTries, audioFailed: !errors.Is(err, placement.ErrWrongItem)})
	default:
		failPlacement(w, err)
	}
}

// stepOpts says how to render a step. cont skips the break before reading: the learner already took it.
// tries and audioFailed are for a screen that came from a replacement.
type stepOpts struct {
	cont        bool
	tries       int
	audioFailed bool
}

// renderStep shows what the engine says comes next: an item, the break before reading, the offer of the
// typing part, or the results.
func (h *placementHTTP) renderStep(w http.ResponseWriter, r *http.Request, userID, runID int64, step placement.Step, o stepOpts) {
	switch {
	case step.Done && step.Result != nil:
		h.write(w, r, h.resultsView(w, r, *step.Result))
	case step.TypingOffer:
		v := h.base(w, r, views.PlaceOffer, runID)
		v.Part = 2
		h.write(w, r, v)
	case step.Item != nil:
		it := *step.Item
		if it.NewPart && it.Part == placement.PartReading && !o.cont {
			v := h.base(w, r, views.PlaceBreak, runID)
			v.Part = 1
			h.write(w, r, v)
			return
		}
		if v, ok := h.itemView(w, r, runID, it, o); ok {
			h.write(w, r, v)
		}
	default:
		fail(w, http.StatusInternalServerError)
	}
}

// itemView builds the screen of one item, with its place in its part.
func (h *placementHTTP) itemView(w http.ResponseWriter, r *http.Request, runID int64, it placement.Item, o stepOpts) (views.PlacementView, bool) {
	items, err := h.runs.ListPlacementItems(r.Context(), runID)
	if err != nil {
		fail(w, http.StatusInternalServerError)
		return views.PlacementView{}, false
	}
	v := h.base(w, r, placementScreen(it.Part), runID)
	v.Phase = views.PlaceAsk
	v.Token = it.ID
	v.AudioURL = it.AudioURL
	v.Prompt = it.Prompt
	v.Tries = o.tries
	v.AudioFailed = o.audioFailed
	v.Index, v.Total = placementProgress(items, it)
	for i, opt := range it.Options {
		v.Options = append(v.Options, views.PlaceOptionView{
			ID:     opt.ID,
			Text:   opt.Text,
			Hebrew: it.Part == placement.PartLetters,
			Key:    views.PlaceKey(i),
			State:  views.AnswerDefault,
		})
	}
	return v, true
}

// base is a view with its page chrome and run id set.
func (h *placementHTTP) base(w http.ResponseWriter, r *http.Request, screen string, runID int64) views.PlacementView {
	lang := chooseLang(w, r, h.secure)
	page := views.NewPage(lang, csrfValue(h.tokens, r), "placement")
	page.Title = views.PlacementFor("en").Heading + " · " + views.AppName
	v := views.PlacementView{Page: page, Screen: screen}
	if runID > 0 {
		v.RunID = strconv.FormatInt(runID, 10)
	}
	return v
}

// writePage sends a whole page. A GET is always a page, even when htmx boosted it: the body selects #app
// out of the response, and a bare fragment would swap in nothing.
func (h *placementHTTP) writePage(w http.ResponseWriter, r *http.Request, v views.PlacementView) {
	render(w, r, views.Placement(v))
}

// write sends a screen: the fragment for an htmx write, the whole page otherwise.
func (h *placementHTTP) write(w http.ResponseWriter, r *http.Request, v views.PlacementView) {
	if r.Header.Get("HX-Request") == "true" {
		render(w, r, views.PlacementRegion(v))
		return
	}
	render(w, r, views.Placement(v))
}

func (h *placementHTTP) user(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	u, ok := auth.CurrentUser(r.Context())
	if ok && u != nil {
		return *u, true
	}
	w.Header().Set("Cache-Control", "no-store")
	loc := signInPath(w, r, h.secure)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", loc)
		w.WriteHeader(http.StatusUnauthorized)
		return auth.User{}, false
	}
	http.Redirect(w, r, loc, http.StatusSeeOther)
	return auth.User{}, false
}

func (h *placementHTTP) ready(w http.ResponseWriter) bool {
	if h.deck == nil || h.runs == nil {
		fail(w, http.StatusServiceUnavailable)
		return false
	}
	return true
}

func postedRun(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PostForm.Get("run_id"), 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func readForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		fail(w, http.StatusBadRequest)
		return false
	}
	return true
}

func fail(w http.ResponseWriter, status int) {
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, http.StatusText(status), status)
}

// failPlacement maps an engine error to a status. None of the messages names an answer.
func failPlacement(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, placement.ErrNotYourRun):
		fail(w, http.StatusNotFound)
	case errors.Is(err, placement.ErrBadAnswer):
		fail(w, http.StatusBadRequest)
	case errors.Is(err, placement.ErrWrongItem), errors.Is(err, placement.ErrFinished), errors.Is(err, placement.ErrNotReady):
		fail(w, http.StatusConflict)
	case errors.Is(err, placement.ErrNoAudio), errors.Is(err, placement.ErrDeckShort):
		fail(w, http.StatusServiceUnavailable)
	default:
		fail(w, http.StatusInternalServerError)
	}
}
