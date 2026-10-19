package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// maxStudyQueues is the most review queues kept in memory.
// The oldest is dropped when a new one would pass this.
const maxStudyQueues = 128

// server holds the learner's database and the in-memory study queues.
// Queues are keyed by the auth session id hash. A restart drops them.
// An auth session can slide for 30 days. A study queue does not: it is
// rebuilt when it is Done, and when the store clock's UTC date is no
// longer the date it was built.
type server struct {
	store    *db.Store
	audioDir string
	study    *session.Service
	deck     placementDeck
	runs     placementRuns

	mu     sync.Mutex
	queues map[string]*studyQueue
}

// studyQueue is one auth session's review, including the counts the card view shows.
type studyQueue struct {
	userID   int64
	sess     *session.Session
	cur      session.CardRef
	has      bool
	index    int
	total    int
	retry    bool
	answered int
	correct  int
	done     bool
	revealed bool
	builtAt  time.Time
}

func newServer(store *db.Store) *server {
	s := &server{
		store:  store,
		queues: map[string]*studyQueue{},
	}
	if store != nil {
		s.study = session.New(store, store.Clock(), 1)
	}
	return s
}

func (s *server) loadHome(r *http.Request) (HomeVM, string, string, error) {
	user, ok := auth.CurrentUser(r.Context())
	if !ok || s == nil || s.store == nil {
		return HomeVM{}, views.AddressMasculine, "", nil
	}
	vm, situation, err := s.learnerHome(r.Context(), user.ID, user.Email)
	if err != nil {
		return HomeVM{}, "", "", err
	}
	gender, err := s.addressGender(r.Context(), user.ID)
	if err != nil {
		return HomeVM{}, "", "", err
	}
	return vm, gender, situation, nil
}

func (s *server) learnerHome(ctx context.Context, userID int64, name string) (HomeVM, string, error) {
	vm := HomeVM{Learner: LearnerVM{ID: userID, Name: name}}
	if s == nil || s.store == nil || s.store.Clock() == nil {
		return vm, "", fmt.Errorf("web: store clock is not configured")
	}
	now := s.store.Clock().Now()
	due, err := s.store.ListDueCards(ctx, userID, now)
	if err != nil {
		return HomeVM{}, "", err
	}
	vm.DueCount = len(due)

	known, err := s.store.KnownLexemeIDs(ctx, userID)
	if err != nil {
		return HomeVM{}, "", err
	}
	vm.KnownWords = len(known)

	next, situation, _, _, err := s.unitProgress(ctx, userID)
	if err != nil {
		return HomeVM{}, "", err
	}
	vm.NextUnit = next

	vm.Placement, err = s.placementVM(ctx, userID)
	if err != nil {
		return HomeVM{}, "", err
	}
	return vm, situation, nil
}

func (s *server) progressVM(ctx context.Context, userID int64) (ProgressVM, error) {
	var vm ProgressVM
	if s == nil || s.store == nil || s.store.Clock() == nil {
		return vm, fmt.Errorf("web: store clock is not configured")
	}
	now := s.store.Clock().Now()
	due, err := s.store.ListDueCards(ctx, userID, now)
	if err != nil {
		return ProgressVM{}, err
	}
	vm.DueCount = len(due)
	known, err := s.store.KnownLexemeIDs(ctx, userID)
	if err != nil {
		return ProgressVM{}, err
	}
	vm.KnownWords = len(known)
	_, _, completed, total, err := s.unitProgress(ctx, userID)
	if err != nil {
		return ProgressVM{}, err
	}
	vm.UnitsCompleted = completed
	vm.UnitsTotal = total
	vm.Placement, err = s.placementVM(ctx, userID)
	if err != nil {
		return ProgressVM{}, err
	}
	return vm, nil
}

// unitProgress returns the next published shared unit this learner has not
// finished, that unit's situation line, and the published shared counts.
func (s *server) unitProgress(ctx context.Context, userID int64) (*UnitRefVM, string, int, int, error) {
	units, err := s.store.UnitsInScope(ctx, userID)
	if err != nil {
		return nil, "", 0, 0, err
	}
	done, err := s.store.CompletedSharedUnits(ctx, userID)
	if err != nil {
		return nil, "", 0, 0, err
	}
	finished := map[int64]bool{}
	for _, unit := range done {
		finished[unit.UnitID] = true
	}
	var next *UnitRefVM
	situation := ""
	completed, total := 0, 0
	for _, unit := range units {
		if unit.Scope != "shared" || unit.Status != "published" {
			continue
		}
		total++
		if finished[unit.ID] {
			completed++
			continue
		}
		if next == nil {
			next = &UnitRefVM{ID: unit.ID, Position: unit.Position, Title: unit.Title}
			situation = unit.Situation
		}
	}
	return next, situation, completed, total, nil
}

func (s *server) placementVM(ctx context.Context, userID int64) (*PlacementVM, error) {
	latest, ok, err := s.store.LatestPlacement(ctx, userID)
	if err != nil || !ok {
		return nil, err
	}
	_, active, err := s.store.OpenPlacementRun(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &PlacementVM{
		Active:              active,
		Finished:            latest.Finished,
		CardsRated:          latest.CardsRated,
		Level:               latest.Level,
		EstimatedKnownWords: latest.EstimatedKnownWords,
		TakenAt:             latest.TakenAt,
	}, nil
}

func (s *server) addressGender(ctx context.Context, userID int64) (string, error) {
	if s == nil || s.store == nil || userID == 0 {
		return views.AddressMasculine, nil
	}
	gender, err := s.store.AddressGender(ctx, userID)
	if err != nil {
		return "", err
	}
	if gender == views.AddressFeminine {
		return views.AddressFeminine, nil
	}
	return views.AddressMasculine, nil
}

func homeToView(lang, csrf, gender, situation string, vm HomeVM) views.Home {
	ui := views.UIFor(lang, gender)
	ui.Situation = situation
	home := views.Home{
		UI:        ui,
		CSRF:      csrf,
		Due:       vm.DueCount,
		DueText:   strconv.Itoa(vm.DueCount),
		KnownText: strconv.Itoa(vm.KnownWords),
		Words:     []views.Chip{},
	}
	fillPlacement(&home, vm.Placement)
	return home
}

// fillPlacement copies the learner's placement onto the Today card.
// A nil value leaves the card off the page only when the caller then sets
// an inactive shell. Active and finished runs also fill the data attributes.
func fillPlacement(home *views.Home, p *PlacementVM) {
	if home == nil || p == nil {
		return
	}
	home.Placement = &views.PlacementVM{
		Active:              p.Active,
		Finished:            p.Finished,
		CardsRated:          p.CardsRated,
		Level:               p.Level,
		EstimatedKnownWords: p.EstimatedKnownWords,
		TakenAt:             p.TakenAt,
	}
	state := placementState(p)
	if state == "" {
		return
	}
	home.PlacementState = state
	home.PlacementWords = strconv.Itoa(p.EstimatedKnownWords)
	home.PlacementLevel = strconv.Itoa(p.Level)
	home.PlacementRated = strconv.Itoa(p.CardsRated)
	if !p.TakenAt.IsZero() {
		home.PlacementAt = db.FormatTime(p.TakenAt)
	}
}

func progressToView(lang, csrf, gender string, vm ProgressVM) views.ProgressPage {
	ui := views.UIFor(lang, gender)
	page := views.ProgressPage{
		Page:      views.Page{UI: ui, Title: ui.DocTitle("progress"), CSRF: csrf, Current: "progress"},
		KnownText: strconv.Itoa(vm.KnownWords),
		UnitsText: strconv.Itoa(vm.UnitsCompleted) + "/" + strconv.Itoa(vm.UnitsTotal),
		DueText:   strconv.Itoa(vm.DueCount),
	}
	if vm.Placement != nil {
		page.Placement = placementState(vm.Placement)
		page.PlacementWords = strconv.Itoa(vm.Placement.EstimatedKnownWords)
		page.PlacementLevel = strconv.Itoa(vm.Placement.Level)
		page.PlacementRated = strconv.Itoa(vm.Placement.CardsRated)
		page.PlacementAt = db.FormatTime(vm.Placement.TakenAt)
	}
	return page
}

func placementState(p *PlacementVM) string {
	if p == nil {
		return ""
	}
	if p.Active {
		return "active"
	}
	if p.Finished {
		return "finished"
	}
	return ""
}

func (s *server) sessionKey(r *http.Request) (string, *auth.User, bool) {
	user, ok := auth.CurrentUser(r.Context())
	if !ok || user == nil {
		return "", nil, false
	}
	sess, ok := auth.CurrentSession(r.Context())
	if !ok || sess.IDHash == "" || sess.UserID != user.ID {
		return "", user, false
	}
	return sess.IDHash, user, true
}

// queueLocked returns the study queue for this auth session.
// A Done queue, or one built on another UTC day, is rebuilt.
// The caller holds s.mu.
func (s *server) queueLocked(ctx context.Context, key string, userID int64) (*studyQueue, error) {
	now := s.now()
	if q := s.queues[key]; q != nil && q.userID == userID && !q.done && sameStudyDay(q.builtAt, now) {
		return q, nil
	}
	if s.study == nil {
		q := &studyQueue{userID: userID, done: true, builtAt: now}
		s.rememberLocked(key, q, now)
		return q, nil
	}
	built, err := s.study.BuildSession(ctx, userID)
	if err != nil {
		return nil, err
	}
	q := &studyQueue{userID: userID, sess: built, builtAt: now}
	s.advance(q)
	s.rememberLocked(key, q, now)
	return q, nil
}

// rememberLocked stores q and keeps the map within maxStudyQueues.
// Queues from another UTC day are dropped first. The caller holds s.mu.
func (s *server) rememberLocked(key string, q *studyQueue, now time.Time) {
	s.evictLocked(now)
	delete(s.queues, key)
	for len(s.queues) >= maxStudyQueues {
		s.dropOldestLocked()
	}
	s.queues[key] = q
}

// evictLocked drops queues built on another UTC day, then the oldest
// until the map is within maxStudyQueues. The caller holds s.mu.
func (s *server) evictLocked(now time.Time) {
	for key, q := range s.queues {
		if q == nil || !sameStudyDay(q.builtAt, now) {
			delete(s.queues, key)
		}
	}
	for len(s.queues) > maxStudyQueues {
		s.dropOldestLocked()
	}
}

func (s *server) dropOldestLocked() {
	oldestKey := ""
	var oldest time.Time
	for key, q := range s.queues {
		if q == nil {
			delete(s.queues, key)
			return
		}
		if oldestKey == "" || q.builtAt.Before(oldest) {
			oldestKey = key
			oldest = q.builtAt
		}
	}
	if oldestKey != "" {
		delete(s.queues, oldestKey)
	}
}

func (s *server) now() time.Time {
	if s != nil && s.store != nil && s.store.Clock() != nil {
		return s.store.Clock().Now()
	}
	return time.Now().UTC()
}

// sameStudyDay reports whether built and now fall on the same UTC date.
// The study queue is rebuilt when the store clock crosses that date.
func sameStudyDay(built, now time.Time) bool {
	if built.IsZero() {
		return false
	}
	by, bm, bd := built.UTC().Date()
	ny, nm, nd := now.UTC().Date()
	return by == ny && bm == nm && bd == nd
}

func (s *server) advance(q *studyQueue) {
	if q != nil {
		q.revealed = false
	}
	if q == nil || q.sess == nil {
		if q != nil {
			q.done = true
			q.has = false
		}
		return
	}
	card, ok := q.sess.Next()
	if !ok {
		_, total, _ := q.sess.Place()
		q.done = true
		q.has = false
		q.total = total
		q.index = total
		q.retry = false
		return
	}
	q.cur = card
	q.has = true
	q.done = false
	q.index, q.total, q.retry = q.sess.Place()
}

func (s *server) cardVM(ctx context.Context, q *studyQueue) (ReviewCardVM, error) {
	vm := ReviewCardVM{Done: true}
	if q == nil {
		return vm, nil
	}
	vm.Index = q.index
	vm.Total = q.total
	vm.Retry = q.retry
	vm.Answered = q.answered
	vm.Correct = q.correct
	vm.Done = q.done || !q.has
	vm.Revealed = q.revealed
	if vm.Done {
		return vm, nil
	}
	ref := q.cur
	vm.CardID = ref.CardID
	vm.Kind = ref.Kind
	if ref.Kind == session.KindLetter || ref.Kind == session.KindPattern {
		vm.Pointed = ref.Key
		vm.Unpointed = units.Plain(ref.Key)
		return vm, nil
	}
	if s.store == nil || ref.LexemeID == 0 {
		return vm, fmt.Errorf("web: card %d has no lexeme", ref.CardID)
	}
	face, ok, err := s.store.LexemeByID(ctx, ref.LexemeID)
	if err != nil {
		return ReviewCardVM{}, err
	}
	if !ok {
		return ReviewCardVM{}, fmt.Errorf("web: lexeme %d: not found", ref.LexemeID)
	}
	if face.AudioID != "" {
		vm.AudioURL = "/audio/" + face.AudioID + ".mp3"
	}
	vm.Pointed = face.TextNFC
	vm.Unpointed = units.Plain(face.TextNFC)
	vm.Translit = face.Translit
	vm.Back = CardBackVM{Translit: face.Translit, Gender: face.Gender, Gloss: face.Gloss}
	show, err := s.showUnpointed(ctx, ref.CardID, ref.Skill)
	if err != nil {
		return ReviewCardVM{}, err
	}
	vm.ShowUnpointed = show
	if ref.Skill == fsrs.SkillRecall || ref.Kind == fsrs.SkillRecall {
		vm.Kind = fsrs.SkillRecall
		return vm, nil
	}
	vm.Kind = fsrs.SkillRead
	return vm, nil
}

// showUnpointed uses fsrs.ShowUnpointed. A recall card, a drill, and a card
// below 21 days of stability stay pointed.
func (s *server) showUnpointed(ctx context.Context, cardID int64, skill string) (bool, error) {
	if s == nil || s.store == nil || cardID <= 0 || skill != fsrs.SkillRead {
		return false, nil
	}
	row, err := s.store.GetCard(ctx, cardID)
	if err != nil {
		return false, err
	}
	logs, err := s.store.ReviewLogs(ctx, cardID)
	if err != nil {
		return false, err
	}
	ratings := make([]fsrs.Rating, len(logs))
	for i, log := range logs {
		ratings[i] = fsrs.Rating(log.Rating)
	}
	return fsrs.ShowUnpointed(fsrs.Card{
		Skill:     row.Skill,
		Stability: row.Stability,
		Ratings:   ratings,
	}), nil
}

func reviewView(lang, csrf, lede string, vm ReviewCardVM) views.ReviewCard {
	card := views.ReviewCard{
		CSRF:          csrf,
		Lang:          lang,
		Lede:          lede,
		CardID:        vm.CardID,
		Kind:          vm.Kind,
		Pointed:       vm.Pointed,
		Unpointed:     vm.Unpointed,
		ShowUnpointed: vm.ShowUnpointed,
		AudioURL:      vm.AudioURL,
		Gloss:         vm.Back.Gloss,
		Translit:      vm.Translit,
		Gender:        vm.Back.Gender,
		Index:         vm.Index,
		Total:         vm.Total,
		Retry:         vm.Retry,
		Done:          vm.Done,
		Revealed:      vm.Revealed,
		Answered:      vm.Answered,
		Correct:       vm.Correct,
	}
	if vm.Revealed {
		return card
	}
	// Anything that reveals the answer side is post-reveal only.
	// read: the Hebrew is the prompt, so pointed text, translit, and audio stay.
	// The English gloss and the gender chip wait.
	// recall: the English gloss is the prompt. The Hebrew, its transliteration,
	// its audio, and the gender chip all identify the answer, so they wait.
	card.Gender = ""
	if vm.Kind == fsrs.SkillRecall || vm.Kind == "recall" {
		card.Pointed = ""
		card.Unpointed = ""
		card.Translit = ""
		card.AudioURL = ""
		return card
	}
	card.Gloss = ""
	return card
}

func reviewStatus(err error) int {
	if errors.Is(err, session.ErrNotServed) || errors.Is(err, session.ErrNotRevealed) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
