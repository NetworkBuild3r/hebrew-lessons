package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// server holds the learner's database and the in-memory study queues.
// Queues are keyed by the auth session id hash. A restart drops them.
type server struct {
	store *db.Store
	study *session.Service

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
	open, active, err := s.store.OpenPlacementRun(ctx, userID)
	if err != nil {
		return nil, err
	}
	_ = open
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
	if vm.NextUnit != nil {
		home.Lesson = true
		home.UnitTitle = vm.NextUnit.Title
	}
	if vm.Placement != nil {
		home.Placement = placementState(vm.Placement)
		home.PlacementWords = strconv.Itoa(vm.Placement.EstimatedKnownWords)
		home.PlacementLevel = strconv.Itoa(vm.Placement.Level)
		home.PlacementRated = strconv.Itoa(vm.Placement.CardsRated)
		home.PlacementAt = db.FormatTime(vm.Placement.TakenAt)
	}
	return home
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
// The caller holds s.mu.
func (s *server) queueLocked(ctx context.Context, key string, userID int64) (*studyQueue, error) {
	if q := s.queues[key]; q != nil && q.userID == userID {
		return q, nil
	}
	if s.study == nil {
		q := &studyQueue{userID: userID, done: true}
		s.queues[key] = q
		return q, nil
	}
	built, err := s.study.BuildSession(ctx, userID)
	if err != nil {
		return nil, err
	}
	q := &studyQueue{userID: userID, sess: built}
	s.advance(q)
	s.queues[key] = q
	return q, nil
}

func (s *server) advance(q *studyQueue) {
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
	show, err := s.showUnpointed(ctx, ref.CardID, ref.Dir)
	if err != nil {
		return ReviewCardVM{}, err
	}
	vm.ShowUnpointed = show
	if ref.Dir == fsrs.DirEnHe || ref.Kind == fsrs.DirEnHe {
		vm.Kind = fsrs.DirEnHe
		return vm, nil
	}
	vm.Kind = fsrs.DirHeEn
	return vm, nil
}

// showUnpointed uses fsrs.ShowUnpointed. An en_he card, a drill, and a card
// below 21 days of stability stay pointed.
func (s *server) showUnpointed(ctx context.Context, cardID int64, dir string) (bool, error) {
	if s == nil || s.store == nil || cardID <= 0 || dir != fsrs.DirHeEn {
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
		Dir:       row.Dir,
		Stability: row.Stability,
		Ratings:   ratings,
	}), nil
}

func reviewView(lang, csrf, lede string, vm ReviewCardVM) views.ReviewCard {
	return views.ReviewCard{
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
		Answered:      vm.Answered,
		Correct:       vm.Correct,
	}
}
