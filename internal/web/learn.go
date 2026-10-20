package web

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/fsrs"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/placement"
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
	voices   audio.Voices
	study    *session.Service
	engine   *activity.Engine
	deck     placementDeck
	runs     placementRuns
	recheck  PlacementEngine
	bands    learner.BandSource

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
	// item is the checked activity the current card is asked through. It is nil for a
	// flashcard and a drill. fb is set from the checked answer until Continue.
	item *activity.Item
	fb   *answerState
}

func newServer(store *db.Store) *server {
	s := &server{
		store:  store,
		queues: map[string]*studyQueue{},
	}
	if store != nil {
		s.study = session.New(store, store.Clock(), 1)
		s.useActivityKey(nil)
	}
	return s
}

// useActivityKey builds the activity engine. key signs the item tokens; nil gives a
// random key for the life of the process (activity.KeyFromSecret("")).
func (s *server) useActivityKey(key []byte) {
	if s == nil || s.store == nil {
		return
	}
	clk := s.store.Clock()
	opts := []activity.Option{}
	if len(key) > 0 {
		opts = append(opts, activity.WithKey(key))
	}
	s.engine = activity.New(s.store, clk, evidence.New(s.store, fsrs.New(s.store, clk), clk), opts...)
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
	// A shared unit the learner reported is hidden for them: it is not next, and it does
	// not count in their totals.
	reported, err := s.store.HiddenUnitIDs(ctx, userID)
	if err != nil {
		return nil, "", 0, 0, err
	}
	hidden := map[int64]bool{}
	for _, id := range reported {
		hidden[id] = true
	}
	var next *UnitRefVM
	situation := ""
	completed, total := 0, 0
	for _, unit := range units {
		if unit.Scope != "shared" || unit.Status != "published" || hidden[unit.ID] {
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
	// The session was built for userID. The caller is the signed-in learner, and
	// session/doc.go asks the handler to check that before it serves the queue.
	if built.UserID() != userID {
		return nil, fmt.Errorf("web: session belongs to learner %d, not %d", built.UserID(), userID)
	}
	q := &studyQueue{userID: userID, sess: built, builtAt: now}
	if err := s.advance(ctx, q); err != nil {
		return nil, err
	}
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

// advance serves the next card. For a word card it records the exposure of a new
// word, and for a card the mix chose a checked activity for, it builds the item the
// screen asks. When the lexicon cannot support the activity after all, the visit falls
// back to a flashcard, which the session grades through the card's own skill.
func (s *server) advance(ctx context.Context, q *studyQueue) error {
	if q != nil {
		q.revealed = false
		q.item = nil
		q.fb = nil
	}
	if q == nil || q.sess == nil {
		if q != nil {
			q.done = true
			q.has = false
		}
		return nil
	}
	card, ok := q.sess.Next()
	if !ok {
		_, total, _ := q.sess.Place()
		q.done = true
		q.has = false
		q.total = total
		q.index = total
		q.retry = false
		return nil
	}
	q.cur = card
	q.has = true
	q.done = false
	q.index, q.total, q.retry = q.sess.Place()
	return s.prepareItem(ctx, q)
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
	if q.item != nil {
		// A checked activity carries its own prompt. Nothing about the word goes on
		// the card: the answer is not in the page until Check has run.
		vm.Item = q.item
		vm.Answer = q.fb
		return vm, nil
	}
	// A mode that is not a lexeme loads from its key. Letter and pattern are
	// those modes today. A later one is the same path once its entry says so.
	if mode, ok := views.LookupReviewMode(ref.Kind); ok && !mode.Lexeme {
		vm.Pointed = ref.Key
		vm.Unpointed = units.Plain(ref.Key)
		if err := s.fillDrill(ctx, q.userID, &vm); err != nil {
			return ReviewCardVM{}, err
		}
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
	// Lessons play the clip AudioByText finds. lexemes.audio_id stays empty:
	// import and generate write audio_assets, and nothing copies that id onto
	// the lexeme. A review that only read audio_id rendered no <audio> and no orb.
	audioURL, err := s.clipURL(ctx, face.TextNFC, face.AudioID)
	if err != nil {
		return ReviewCardVM{}, err
	}
	vm.AudioURL = audioURL
	vm.Pointed = face.TextNFC
	vm.Unpointed = units.Plain(face.TextNFC)
	vm.Translit = face.Translit
	vm.Back = CardBackVM{Translit: face.Translit, Gender: face.Gender, Gloss: face.Gloss}
	show, err := s.showUnpointed(ctx, ref.CardID, ref.Skill)
	if err != nil {
		return ReviewCardVM{}, err
	}
	vm.ShowUnpointed = show
	// The four study skills keep the fallback below, including a recall skill
	// forcing the recall face. Any other registered mode keeps the kind the
	// queue put on the card, so adding the mode entry is enough.
	if _, ok := views.LookupReviewMode(ref.Kind); ok && !studySkillKind(ref.Kind) {
		vm.Kind = ref.Kind
		return vm, nil
	}
	switch ref.Kind {
	case fsrs.SkillRead, fsrs.SkillRecall, fsrs.SkillListen, fsrs.SkillSpell:
		vm.Kind = ref.Kind
	default:
		if ref.Skill == fsrs.SkillRecall {
			vm.Kind = fsrs.SkillRecall
		} else {
			vm.Kind = fsrs.SkillRead
		}
	}
	if ref.Skill == fsrs.SkillRecall {
		vm.Kind = fsrs.SkillRecall
	}
	return vm, nil
}

func studySkillKind(kind string) bool {
	switch kind {
	case fsrs.SkillRead, fsrs.SkillRecall, fsrs.SkillListen, fsrs.SkillSpell:
		return true
	default:
		return false
	}
}

// fillDrill loads a card that is not a lexeme. Letter and pattern are the
// modes that need it. A later mode whose card is a lexeme does not come
// through here. A mode that is not a lexeme adds a branch in this switch.
// The reveal route and the grade guard do not grow a branch for it.
// The key stays the prompt. Names, sounds, glosses, the example word, and
// its clip are answer fields. ConcealFront keeps them off the page until
// the reveal.
func (s *server) fillDrill(ctx context.Context, userID int64, vm *ReviewCardVM) error {
	key := vm.Pointed
	parts := splitDrillKey(key)
	switch vm.Kind {
	case session.KindLetter:
		vm.Asked = pickPart(vm.CardID, parts)
		sides := make([]views.DrillSide, 0, len(parts))
		for _, part := range parts {
			sound, err := s.letterSound(ctx, part)
			if err != nil {
				return err
			}
			sides = append(sides, views.DrillSide{
				Hebrew: part,
				Name:   views.LetterName(part),
				Sound:  sound,
			})
		}
		vm.Sides = sides
		vm.DageshNote = views.DageshNote(sides)
		ex, ok, err := s.confusionExample(ctx, userID, fsrs.MissLetterPair, key)
		if err != nil {
			return err
		}
		if ok {
			vm.ExampleText = ex.TextNFC
			vm.ExampleTranslit = ex.Translit
			vm.ExampleGloss = ex.Gloss
			url, err := s.clipURL(ctx, ex.TextNFC, ex.AudioID)
			if err != nil {
				return err
			}
			vm.AudioURL = url
		}
	case session.KindPattern:
		var found map[string]db.LexemeView
		if s != nil && s.store != nil {
			var err error
			found, err = s.store.LexemesByText(ctx, parts)
			if err != nil {
				return err
			}
		}
		sides := make([]views.DrillSide, 0, len(parts))
		for _, part := range parts {
			side := views.DrillSide{Hebrew: part}
			if row, ok := found[part]; ok {
				side.Translit = row.Translit
				side.Gloss = row.Gloss
			}
			sides = append(sides, side)
			if vm.AudioURL == "" {
				url, err := s.clipURL(ctx, part, "")
				if err != nil {
					return err
				}
				vm.AudioURL = url
			}
		}
		vm.Sides = sides
	}
	if vm.AudioURL == "" && vm.Kind == session.KindLetter {
		for _, part := range parts {
			url, err := s.clipURL(ctx, part, "")
			if err != nil {
				return err
			}
			if url != "" {
				vm.AudioURL = url
				break
			}
		}
	}
	// The front shows the asked letter. The pair key was the lookup above.
	if vm.Kind == session.KindLetter && vm.Asked != "" {
		vm.Pointed = vm.Asked
		vm.Unpointed = units.Plain(vm.Asked)
	}
	return nil
}

// letterSound is a Latin reading stored for this exact letter.
// Placement syllables match only when the text is the letter itself.
// A lexicon row whose text is the letter contributes its transliteration.
// A bare letter with neither has no sound, and the caller does not invent one.
func (s *server) letterSound(ctx context.Context, letter string) (string, error) {
	letters, err := placement.Letters()
	if err != nil {
		return "", err
	}
	for _, l := range letters {
		if (l.LetterNFC == letter || l.SyllableText == letter) && l.Sound != "" {
			return l.Sound, nil
		}
	}
	if s == nil || s.store == nil || letter == "" {
		return "", nil
	}
	found, err := s.store.LexemesByText(ctx, []string{letter})
	if err != nil {
		return "", err
	}
	if row, ok := found[letter]; ok {
		return row.Translit, nil
	}
	return "", nil
}

func (s *server) confusionExample(ctx context.Context, userID int64, kind, key string) (db.LexemeRef, bool, error) {
	if s == nil || s.store == nil || userID == 0 || key == "" {
		return db.LexemeRef{}, false, nil
	}
	id, ok, err := s.store.LatestConfusionLexeme(ctx, userID, kind, key)
	if err != nil || !ok {
		return db.LexemeRef{}, false, err
	}
	return s.store.LexemeByID(ctx, id)
}

// pickPart chooses one part of a pair. The seed is the card id, so a refresh
// and the no-JS page show the same letter. One part is returned as itself.
func pickPart(cardID int64, parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	src := rand.NewPCG(uint64(cardID), 0x72657665616c)
	return parts[rand.New(src).Uint64N(uint64(len(parts)))]
}

func splitDrillKey(key string) []string {
	if key == "" {
		return nil
	}
	parts := strings.Split(key, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return []string{key}
	}
	return out
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

// clipURL is the clip the lesson player would use for this pointed text.
// AudioByText prefers the newest eleven_v4, then eleven_v3, then legacy.
// audioID is the lexeme column, used only when no asset is stored for the text,
// so a fixture that sets audio_id and does not insert audio_assets still plays.
func (s *server) clipURL(ctx context.Context, text, audioID string) (string, error) {
	if s != nil && s.store != nil {
		key := units.Normalize(text)
		if key != "" {
			asset, ok, err := s.store.AudioByText(ctx, key)
			if err != nil {
				return "", err
			}
			if ok && asset.ID != "" {
				return "/audio/" + asset.ID + ".mp3", nil
			}
		}
	}
	if audioID != "" {
		return "/audio/" + audioID + ".mp3", nil
	}
	return "", nil
}

// attachReviewClips fills a checked activity's answer clips from audio_assets
// when the lexicon column is empty. The prompt of a reading item stays without
// a clip: the Hebrew is the question, and the sound belongs on the answer.
func (s *server) attachReviewClips(ctx context.Context, card *views.ReviewCard) error {
	if card == nil || card.Act == nil || card.Act.Feedback == nil {
		return nil
	}
	fb := card.Act.Feedback
	if fb.AudioURL == "" {
		url, err := s.clipURL(ctx, fb.Hebrew, "")
		if err != nil {
			return err
		}
		fb.AudioURL = url
	}
	for i := range fb.Pairs {
		if fb.Pairs[i].AudioURL != "" || fb.Pairs[i].Hebrew == "" {
			continue
		}
		url, err := s.clipURL(ctx, fb.Pairs[i].Hebrew, "")
		if err != nil {
			return err
		}
		fb.Pairs[i].AudioURL = url
	}
	return nil
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
	if vm.Item != nil {
		card.Act = activityView(lang, csrf, vm)
		return card
	}
	card.Sides = vm.Sides
	card.ExampleText = vm.ExampleText
	card.ExampleTranslit = vm.ExampleTranslit
	card.ExampleGloss = vm.ExampleGloss
	card.TellApart = vm.TellApart
	card.DageshNote = vm.DageshNote
	// The mode's hold decides what the front may carry. A revealed card passes through.
	return views.ConcealFront(card)
}

func reviewStatus(err error) int {
	if errors.Is(err, session.ErrNotServed) || errors.Is(err, session.ErrNotRevealed) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
