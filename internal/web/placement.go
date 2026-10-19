package web

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
	"golang.org/x/text/unicode/norm"
)

// placementDeck is the slice of placement.Service the writes call.
// The service does not read the session. The handler does.
type placementDeck interface {
	Start(ctx context.Context, userID int64) (int64, error)
	Next(ctx context.Context, runID int64) (placement.NextResult, error)
	ContinueSitting(ctx context.Context, userID int64) error
	Rate(ctx context.Context, runID int64, itemID string, rating int) error
	Finish(ctx context.Context, runID int64) (placement.Result, error)
}

// placementRuns is the store read for ownership and the Today card.
// placement.Service does not return the run's owner. The handler loads
// the row and requires UserID to be the session user.
type placementRuns interface {
	GetPlacementRun(ctx context.Context, id int64) (db.PlacementRun, error)
	OpenPlacementRun(ctx context.Context, userID int64) (db.PlacementRun, bool, error)
	LatestPlacement(ctx context.Context, userID int64) (db.Placement, bool, error)
	ListPlacementAnswers(ctx context.Context, runID int64) ([]db.PlacementAnswer, error)
}

var (
	_ placementDeck = (*placement.Service)(nil)
	_ placementRuns = (*db.Store)(nil)
)

type placementHTTP struct {
	deck   placementDeck
	runs   placementRuns
	tokens CSRFToken
	secure bool
}

func mountPlacement(mux *http.ServeMux, tokens CSRFToken, gate, secure bool, deck placementDeck, runs placementRuns) {
	h := &placementHTTP{deck: deck, runs: runs, tokens: tokens, secure: secure}
	mux.HandleFunc("GET /placement", gated(gate, secure, h.page))
	// Writes only. A GET under /_hx/ is not registered (ADR-0016).
	mux.HandleFunc("POST /_hx/placement/start", h.start)
	mux.HandleFunc("POST /_hx/placement/flip", h.flip)
	mux.HandleFunc("POST /_hx/placement/rate", h.rate)
	mux.HandleFunc("POST /_hx/placement/continue", h.cont)
	mux.HandleFunc("POST /_hx/placement/finish", h.finish)
	// No service method sets the start unit back to 1, or skips the deck.
	mux.HandleFunc("POST /_hx/placement/skip", h.unavailable)
	mux.HandleFunc("POST /_hx/placement/start-over", h.unavailable)
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

func (h *placementHTTP) page(w http.ResponseWriter, r *http.Request) {
	if h.runs == nil {
		render(w, r, views.Placement(h.view(w, r, views.PlaceModeIntro, 0, views.PlacementWord{})))
		return
	}
	user, ok := auth.CurrentUser(r.Context())
	if !ok || user == nil {
		render(w, r, views.Placement(h.view(w, r, views.PlaceModeIntro, 0, views.PlacementWord{})))
		return
	}
	open, found, err := h.runs.OpenPlacementRun(r.Context(), user.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError)
		return
	}
	if found {
		v, err := h.enrich(r.Context(), h.view(w, r, views.PlaceModeResume, open.ID, views.PlacementWord{}))
		if err != nil {
			fail(w, http.StatusInternalServerError)
			return
		}
		render(w, r, views.Placement(v))
		return
	}
	latest, found, err := h.runs.LatestPlacement(r.Context(), user.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError)
		return
	}
	if found && latest.Finished {
		v := applyResult(h.view(w, r, views.PlaceModeResult, 0, views.PlacementWord{}), latest.Level, bandAdded(latest.BandScores))
		render(w, r, views.Placement(v))
		return
	}
	render(w, r, views.Placement(h.view(w, r, views.PlaceModeIntro, 0, views.PlacementWord{})))
}

func (h *placementHTTP) start(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	if !h.ready(w) || !readForm(w, r) {
		return
	}
	if r.PostForm.Get("run_id") != "" {
		id, ok := h.postedRun(w, r)
		if !ok {
			return
		}
		run, ok := h.owns(w, r, user.ID, id)
		if !ok {
			return
		}
		if !run.FinishedAt.IsZero() {
			fail(w, http.StatusConflict)
			return
		}
	}
	runID, err := h.deck.Start(r.Context(), user.ID)
	if err != nil {
		failPlacement(w, err)
		return
	}
	if _, ok := h.owns(w, r, user.ID, runID); !ok {
		return
	}
	h.step(w, r, runID, views.PlacementWord{}, false, "region")
}

func (h *placementHTTP) flip(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	if !h.ready(w) || !readForm(w, r) {
		return
	}
	runID, ok := h.postedRun(w, r)
	if !ok {
		return
	}
	if _, ok := h.owns(w, r, user.ID, runID); !ok {
		return
	}
	res, err := h.deck.Next(r.Context(), runID)
	if err != nil {
		failPlacement(w, err)
		return
	}
	if res.Card == nil {
		h.stepResult(w, r, runID, res, views.PlacementWord{}, false, "card")
		return
	}
	itemID := r.PostForm.Get("item_id")
	if itemID != "" && itemID != res.Card.ID {
		fail(w, http.StatusConflict)
		return
	}
	v, err := h.enrich(r.Context(), h.view(w, r, views.PlaceModeBack, runID, wordFrom(*res.Card)))
	if err != nil {
		fail(w, http.StatusInternalServerError)
		return
	}
	h.write(w, r, v, "card")
}

func (h *placementHTTP) rate(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	if !h.ready(w) || !readForm(w, r) {
		return
	}
	runID, ok := h.postedRun(w, r)
	if !ok {
		return
	}
	if _, ok := h.owns(w, r, user.ID, runID); !ok {
		return
	}
	itemID := r.PostForm.Get("item_id")
	rating, err := strconv.Atoi(r.PostForm.Get("rating"))
	if itemID == "" || err != nil || rating < placement.DontKnow || rating > placement.Easy {
		fail(w, http.StatusBadRequest)
		return
	}
	var toast views.PlacementWord
	idk := rating == placement.DontKnow
	if idk {
		if card, _, ok := placement.Lookup(itemID); ok {
			toast = wordFrom(card)
		}
	}
	if err := h.deck.Rate(r.Context(), runID, itemID, rating); err != nil {
		failPlacement(w, err)
		return
	}
	h.step(w, r, runID, toast, idk, "card")
}

func (h *placementHTTP) cont(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	if !h.ready(w) || !readForm(w, r) {
		return
	}
	runID, ok := h.postedRun(w, r)
	if !ok {
		return
	}
	run, ok := h.owns(w, r, user.ID, runID)
	if !ok {
		return
	}
	if !run.FinishedAt.IsZero() {
		fail(w, http.StatusConflict)
		return
	}
	if err := h.deck.ContinueSitting(r.Context(), user.ID); err != nil {
		failPlacement(w, err)
		return
	}
	h.step(w, r, runID, views.PlacementWord{}, false, "region")
}

func (h *placementHTTP) unavailable(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.user(w, r); !ok {
		return
	}
	if !readForm(w, r) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, "placement cannot change the start unit yet", http.StatusNotImplemented)
}

func (h *placementHTTP) finish(w http.ResponseWriter, r *http.Request) {
	user, ok := h.user(w, r)
	if !ok {
		return
	}
	if !h.ready(w) || !readForm(w, r) {
		return
	}
	runID, ok := h.postedRun(w, r)
	if !ok {
		return
	}
	if _, ok := h.owns(w, r, user.ID, runID); !ok {
		return
	}
	result, err := h.deck.Finish(r.Context(), runID)
	if err != nil {
		failPlacement(w, err)
		return
	}
	if result.UserID != user.ID {
		fail(w, http.StatusForbidden)
		return
	}
	v := applyResult(h.view(w, r, views.PlaceModeResult, runID, views.PlacementWord{}), result.Level, addedFromBands(result.Bands))
	h.write(w, r, v, "region")
}

// step renders the next card, the sitting break, or the scored result.
// Next may finish the run. Finish then only reads that stored result.
// swap is "card" when the form targets .rcard, and "region" otherwise.
func (h *placementHTTP) step(w http.ResponseWriter, r *http.Request, runID int64, toast views.PlacementWord, hasToast bool, swap string) {
	res, err := h.deck.Next(r.Context(), runID)
	if err != nil {
		failPlacement(w, err)
		return
	}
	h.stepResult(w, r, runID, res, toast, hasToast, swap)
}

func (h *placementHTTP) stepResult(w http.ResponseWriter, r *http.Request, runID int64, res placement.NextResult, toast views.PlacementWord, hasToast bool, swap string) {
	if res.Done {
		result, err := h.deck.Finish(r.Context(), runID)
		if err != nil {
			failPlacement(w, err)
			return
		}
		v := applyResult(h.view(w, r, views.PlaceModeResult, runID, views.PlacementWord{}), result.Level, addedFromBands(result.Bands))
		if swap == "card" && r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Retarget", "#placement")
			w.Header().Set("HX-Reswap", "outerHTML transition:true")
		}
		h.write(w, r, v, "region")
		return
	}
	if res.Break {
		v, err := h.enrich(r.Context(), h.view(w, r, views.PlaceModeBreak, runID, views.PlacementWord{}))
		if err != nil {
			fail(w, http.StatusInternalServerError)
			return
		}
		if swap == "card" && r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Retarget", "#placement")
			w.Header().Set("HX-Reswap", "outerHTML transition:true")
		}
		h.write(w, r, v, "region")
		return
	}
	if res.Card == nil {
		fail(w, http.StatusInternalServerError)
		return
	}
	v, err := h.enrich(r.Context(), h.view(w, r, views.PlaceModeFront, runID, wordFrom(*res.Card)))
	if err != nil {
		fail(w, http.StatusInternalServerError)
		return
	}
	v.Toast = toast
	v.HasToast = hasToast && toast.Text != ""
	h.write(w, r, v, swap)
}

func (h *placementHTTP) write(w http.ResponseWriter, r *http.Request, v views.PlacementView, swap string) {
	if r.Header.Get("HX-Request") == "true" {
		if swap == "card" && (v.Mode == views.PlaceModeFront || v.Mode == views.PlaceModeBack) {
			render(w, r, views.PlacementCardSwap(v))
			return
		}
		render(w, r, views.PlacementRegion(v))
		return
	}
	render(w, r, views.Placement(v))
}

func (h *placementHTTP) view(w http.ResponseWriter, r *http.Request, mode string, runID int64, word views.PlacementWord) views.PlacementView {
	lang := chooseLang(w, r, h.secure)
	copy := views.PlacementFor(lang)
	page := views.NewPage(lang, csrfValue(h.tokens, r), "placement")
	title := copy.Heading
	switch mode {
	case views.PlaceModeResult:
		title = copy.Results
	case views.PlaceModeResume:
		title = copy.Welcome
	case views.PlaceModeBreak:
		title = copy.BreakTitle
	}
	page.Title = title + " · " + views.AppName
	idText := ""
	if runID > 0 {
		idText = strconv.FormatInt(runID, 10)
	}
	v := views.PlacementView{
		Page:      page,
		Copy:      copy,
		Mode:      mode,
		RunIDText: idText,
		Word:      word,
		Sitting:   1,
		CardIndex: 1,
		Fill:      "0",
		FillPrev:  "0",
		Overall:   "0",
	}
	if mode == views.PlaceModeResume || mode == views.PlaceModeBreak {
		v.Compact = true
		v.Ladder = views.ProgressLadder(copy, [5]bool{})
		v.ResumeLine = copy.ResumeLine(0)
	}
	if mode == views.PlaceModeResult {
		v = applyResult(v, -1, 0)
	}
	return v
}

func (h *placementHTTP) enrich(ctx context.Context, v views.PlacementView) (views.PlacementView, error) {
	if h.runs == nil || v.RunIDText == "" {
		return v, nil
	}
	runID, err := strconv.ParseInt(v.RunIDText, 10, 64)
	if err != nil {
		return v, err
	}
	run, err := h.runs.GetPlacementRun(ctx, runID)
	if err != nil {
		return v, err
	}
	rows, err := h.runs.ListPlacementAnswers(ctx, runID)
	if err != nil {
		return v, err
	}
	sitting := run.SittingNo
	if sitting < 1 {
		sitting = 1
	}
	var rated, total int
	var seen [5]bool
	var samples []views.PlacementWord
	for _, row := range rows {
		if !row.Rating.Valid {
			continue
		}
		total++
		card, band, ok := placement.Lookup(row.ItemID)
		if ok && band >= 0 && band < len(seen) {
			seen[band] = true
		}
		if row.SittingNo != sitting {
			continue
		}
		rated++
		if ok {
			samples = append(samples, wordFrom(card))
		}
	}
	if len(samples) > 6 {
		samples = samples[len(samples)-6:]
	}
	done := sitting - 1
	if rated >= placement.SittingSize {
		done = sitting
	}
	if done < 0 {
		done = 0
	}
	left := placement.CardCap/placement.SittingSize - done
	index := rated + 1
	if v.Mode == views.PlaceModeBreak {
		index = rated
	}
	if index < 1 {
		index = 1
	}
	prev := rated - 1
	if prev < 0 {
		prev = 0
	}
	v.Sitting = sitting
	v.DoneInSit = rated
	v.CardIndex = index
	v.Samples = samples
	v.Fill = ratio(rated, placement.SittingSize)
	v.FillPrev = ratio(prev, placement.SittingSize)
	v.Overall = ratio(total, placement.CardCap)
	v.ResumeLine = v.Copy.ResumeLine(done)
	v.BreakKick = v.Copy.SittingKicker(sitting)
	v.Covered = v.Copy.CoveredLine(rated)
	v.LeftNote = v.Copy.LeftNote(left)
	if v.Mode == views.PlaceModeResume || v.Mode == views.PlaceModeBreak {
		v.Compact = true
		v.Ladder = views.ProgressLadder(v.Copy, seen)
	}
	return v, nil
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

func (h *placementHTTP) postedRun(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.PostForm.Get("run_id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		fail(w, http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// owns loads the run and requires it to belong to the session user.
// A missing id is 404. Another learner's run is 403.
func (h *placementHTTP) owns(w http.ResponseWriter, r *http.Request, userID, runID int64) (db.PlacementRun, bool) {
	run, err := h.runs.GetPlacementRun(r.Context(), runID)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, http.StatusNotFound)
		return db.PlacementRun{}, false
	}
	if err != nil {
		fail(w, http.StatusInternalServerError)
		return db.PlacementRun{}, false
	}
	if run.UserID != userID {
		fail(w, http.StatusForbidden)
		return db.PlacementRun{}, false
	}
	return run, true
}

func wordFrom(c placement.Card) views.PlacementWord {
	return views.PlacementWord{
		ID:       c.ID,
		Text:     norm.NFC.String(c.TextNFC),
		Translit: c.Translit,
		Gloss:    c.GlossEN,
		Gender:   c.Gender,
		AudioURL: audioPath(c.AudioURL),
	}
}

func audioPath(raw string) string {
	if raw == "" || !strings.HasPrefix(raw, "/audio/") {
		return ""
	}
	if strings.Contains(raw, "..") || strings.Contains(raw, "://") || strings.ContainsAny(raw, "\\?#") {
		return ""
	}
	return raw
}

func applyResult(v views.PlacementView, level, added int) views.PlacementView {
	unit, done := views.UnitForBand(level)
	v.Ladder = views.ResultLadder(v.Copy, level)
	v.Compact = false
	v.Unit = unit
	v.CourseDone = done
	v.UnitLine = v.Copy.UnitLine(unit)
	v.StartLabel = v.Copy.StartUnit(unit)
	v.Added = added
	v.AddedText = v.Copy.AddedLine(added)
	return v
}

func addedFromBands(bands []placement.BandScore) int {
	n := 0
	for _, band := range bands {
		n += band.Again + band.Hard + band.DontKnow
	}
	return n
}

func bandAdded(bands []db.BandScore) int {
	n := 0
	for _, band := range bands {
		n += band.Again + band.Hard + band.DontKnow
	}
	return n
}

func ratio(n, d int) string {
	if d <= 0 || n <= 0 {
		return "0.000"
	}
	if n > d {
		n = d
	}
	return strconv.FormatFloat(float64(n)/float64(d), 'f', 3, 64)
}

func readForm(w http.ResponseWriter, r *http.Request) bool {
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

// failPlacement maps service errors onto status codes. The service has
// no exported sentinels, so the match is on the text it already returns.
func failPlacement(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
		fail(w, http.StatusNotFound)
		return
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "outside 0..4"), strings.Contains(msg, "item id is empty"):
		fail(w, http.StatusBadRequest)
	case strings.Contains(msg, "already finished"),
		strings.Contains(msg, "not the open card"),
		strings.Contains(msg, "not full"),
		strings.Contains(msg, "no open run"),
		strings.Contains(msg, "already has a card"):
		fail(w, http.StatusConflict)
	default:
		fail(w, http.StatusInternalServerError)
	}
}
