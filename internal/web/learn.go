package web

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
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
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// maxStudyQueues is the most review queues kept in memory.
// The oldest is dropped when a new one would pass this.
const maxStudyQueues = 128

// server holds the learner's database and the in-memory study queues.
// Queues are keyed by the auth session id hash. A restart drops them.
// An auth session can slide for 30 days. A study queue does not: it is
// rebuilt when it is Done, and when the store clock passes the study-day
// rollover (4:00 in the learner's zone).
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
	zones    *studyday.Resolver
	speech   *audio.Generator

	mu     sync.Mutex
	queues map[string]*studyQueue
	// fixedReviewSeed keeps the service seed for every sitting. Tests that
	// inject a Service set it so the queue stays deterministic. Production
	// draws a seed once per sitting.
	fixedReviewSeed bool
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
	// rollover is studyday.DueCutoff when the queue was built. Answers and the
	// queue itself last until then (INIT-027/SPEC-006, ADR-0023 D6).
	rollover time.Time
	// seed is the mix seed for this sitting. It is drawn once, when the queue
	// is built, and kept until the queue is Done or the study day ends, so a
	// reload serves the same order.
	seed int64
	// asked is the letter a two-letter letter step asks. It is pinned when the
	// step is served (and kept after a pick), so a study-day rollover cannot
	// switch the letter mid-step.
	asked string
	// servedAt is when the current two-letter letter step was served. Drill
	// latency is now.Sub(servedAt).
	servedAt time.Time
	// item is the checked activity the current card is asked through. It is nil for a
	// flashcard and a drill. fb is set from the checked answer until Continue.
	// drillFB is set from a letter pick until Continue.
	item    *activity.Item
	fb      *answerState
	drillFB *drillState
	// tally is a copy of Session.Tally when the sitting finished, so a reload
	// of the done screen shows the same rows.
	tally    session.Tally
	hasTally bool
}

// sameDay reports whether now is still before this queue's study-day rollover.
func (q *studyQueue) sameDay(now time.Time) bool {
	return q != nil && !q.rollover.IsZero() && now.Before(q.rollover)
}

// drillState is a letter-pick answer kept until Continue so a reload of /review
// shows the same feedback.
type drillState struct {
	correct bool
	cleared bool
}

func newServer(store *db.Store) *server {
	s := &server{
		store:  store,
		queues: map[string]*studyQueue{},
	}
	if store != nil {
		// 1 is the band-slip tie-break (BuildSessionFor) and the seed an
		// injected service replaces. GET /review does not use it: queueLocked
		// draws a seed for the sitting and passes that to BuildSessionWithSeed.
		s.study = session.New(store, store.Clock(), 1)
		s.useActivityKey(nil)
		appLoc, err := studyday.AppZone("")
		if err == nil {
			if zones, zerr := studyday.NewResolver(store, appLoc); zerr == nil {
				s.zones = zones
			}
		}
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

	next, situation, _, _, _, _, err := s.unitProgress(ctx, userID)
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
// finished, that unit's situation line, and the finished counts.
// UnitsInScope already leaves another learner's units out. Totals then keep
// published shared units and this learner's own published units. Unpublished
// units and units they reported are left out. A unit is finished only when it
// has at least one lesson and every one of those lessons is finished. A
// finished run whose index is past the current lesson count does not count.
// The lesson counts are the lessons inside the units that count. The next
// unit stays the first unfinished shared unit.
func (s *server) unitProgress(ctx context.Context, userID int64) (*UnitRefVM, string, int, int, int, int, error) {
	units, err := s.store.UnitsInScope(ctx, userID)
	if err != nil {
		return nil, "", 0, 0, 0, 0, err
	}
	finishes, err := s.store.FinishedLessons(ctx, userID)
	if err != nil {
		return nil, "", 0, 0, 0, 0, err
	}
	// A unit the learner reported is hidden for them: it is not next, and it does
	// not count in their totals.
	reported, err := s.store.HiddenUnitIDs(ctx, userID)
	if err != nil {
		return nil, "", 0, 0, 0, 0, err
	}
	hidden := map[int64]bool{}
	for _, id := range reported {
		hidden[id] = true
	}
	var next *UnitRefVM
	situation := ""
	unitsDone, unitsTotal, lessonsDone, lessonsTotal := 0, 0, 0, 0
	for _, unit := range units {
		if hidden[unit.ID] || unit.Status != "published" {
			continue
		}
		n := db.LessonCount(unit.Doc)
		unitsTotal++
		lessonsTotal += n
		got := 0
		for ix := 0; ix < n; ix++ {
			if _, ok := finishes[unit.ID][ix]; ok {
				got++
			}
		}
		lessonsDone += got
		whole := n > 0 && got == n
		if whole {
			unitsDone++
		}
		if unit.Scope == "shared" && !whole && next == nil {
			next = &UnitRefVM{ID: unit.ID, Position: unit.Position, Title: unit.Title}
			situation = unit.Situation
		}
	}
	return next, situation, unitsDone, unitsTotal, lessonsDone, lessonsTotal, nil
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
// A queue past its study-day rollover is rebuilt. A finished sitting is kept
// when the rebuild is also finished, so its count stays on the done screen.
// The caller holds s.mu.
func (s *server) queueLocked(ctx context.Context, key string, userID int64) (*studyQueue, error) {
	now := s.now()
	prev := s.queues[key]
	if prev != nil && prev.userID == userID && !prev.done && prev.sameDay(now) {
		return prev, nil
	}
	// No learner database: Review stays on the empty done state. That page is
	// not a civil day, so it does not invent a zone.
	if s.study == nil {
		q := &studyQueue{userID: userID, done: true, builtAt: now}
		s.rememberLocked(key, q, now)
		return q, nil
	}
	z, err := s.learnerZone(ctx, userID)
	if err != nil {
		return nil, err
	}
	rollover := studyday.DueCutoff(now, z)
	seed, err := s.reviewSeedLocked()
	if err != nil {
		return nil, err
	}
	built, err := s.study.BuildSessionWithSeed(ctx, userID, seed)
	if err != nil {
		return nil, err
	}
	// The session was built for userID. The caller is the signed-in learner, and
	// session/doc.go asks the handler to check that before it serves the queue.
	if built.UserID() != userID {
		return nil, fmt.Errorf("web: session belongs to learner %d, not %d", built.UserID(), userID)
	}
	q := &studyQueue{userID: userID, sess: built, builtAt: now, seed: seed, rollover: rollover}
	if err := s.advance(ctx, q); err != nil {
		return nil, err
	}
	// A finished sitting stays on the next read when nothing new is due, so the
	// done screen can still name how many cards that sitting reviewed.
	if q.done && prev != nil && prev.userID == userID && prev.done && prev.answered > 0 && prev.sameDay(now) {
		return prev, nil
	}
	s.rememberLocked(key, q, now)
	return q, nil
}

// rememberLocked stores q and keeps the map within maxStudyQueues.
// Queues past their study-day rollover are dropped first. The caller holds s.mu.
func (s *server) rememberLocked(key string, q *studyQueue, now time.Time) {
	s.evictLocked(now)
	delete(s.queues, key)
	for len(s.queues) >= maxStudyQueues {
		s.dropOldestLocked()
	}
	s.queues[key] = q
}

// evictLocked drops queues past their study-day rollover, then the oldest
// until the map is within maxStudyQueues. The caller holds s.mu.
func (s *server) evictLocked(now time.Time) {
	for key, q := range s.queues {
		if q == nil || !q.sameDay(now) {
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

// reviewSeedLocked is the mix seed for a new sitting. The caller holds s.mu.
// A fixed seed is the one the service was built with, so tests that inject
// a Service keep a deterministic queue. Otherwise the seed is drawn once
// and stored on the queue.
func (s *server) reviewSeedLocked() (int64, error) {
	if s.fixedReviewSeed && s.study != nil {
		return s.study.Seed(), nil
	}
	return drawReviewSeed()
}

// drawReviewSeed reads 8 bytes from crypto/rand. The value is the sitting
// seed. Zero is a valid seed. The queue stores it, so later requests in
// the sitting do not draw again.
func drawReviewSeed() (int64, error) {
	var buf [8]byte
	if _, err := cryptorand.Read(buf[:]); err != nil {
		return 0, fmt.Errorf("web: review seed: %w", err)
	}
	return int64(binary.LittleEndian.Uint64(buf[:])), nil
}

func (s *server) now() time.Time {
	if s != nil && s.store != nil && s.store.Clock() != nil {
		return s.store.Clock().Now()
	}
	return time.Now().UTC()
}

// advance serves the next card. For a word card it records the exposure of a new
// word, and for a card the mix chose a checked activity for, it builds the item the
// screen asks. When the lexicon cannot support the activity after all, the visit falls
// back to a flashcard, which the session grades through the card's own skill.
// A two-letter letter step pins asked and servedAt here for the timed pick.
func (s *server) advance(ctx context.Context, q *studyQueue) error {
	if q != nil {
		q.revealed = false
		q.asked = ""
		q.servedAt = time.Time{}
		q.item = nil
		q.fb = nil
		q.drillFB = nil
	}
	if q == nil || q.sess == nil {
		if q != nil {
			q.done = true
			q.has = false
			q.snapshotTally()
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
		q.snapshotTally()
		return nil
	}
	q.cur = card
	q.has = true
	q.done = false
	q.index, q.total, q.retry = q.sess.Place()
	if err := s.prepareItem(ctx, q); err != nil {
		return err
	}
	if parts := pairParts(q); parts != nil {
		z, err := s.learnerZone(ctx, q.userID)
		if err != nil {
			return err
		}
		q.asked = pickPart(q.cur.CardID, studyday.DueCutoff(s.now(), z), parts)
		q.servedAt = s.now()
	}
	return nil
}

// snapshotTally stores the sitting's counters on the queue for the done screen.
func (q *studyQueue) snapshotTally() {
	if q == nil || q.sess == nil {
		return
	}
	q.tally = q.sess.Tally()
	q.hasTally = true
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
		if q.hasTally {
			vm.MovedUp = q.tally.MovedUp
			vm.PairsCleared = q.tally.PairsCleared
			vm.Slow = q.tally.Slow
			vm.NewMet = q.tally.NewMet
		}
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
		parts := splitDrillKey(ref.Key)
		letterPair := ref.Kind == session.KindLetter && len(parts) >= 2
		if q.asked != "" {
			vm.Asked = q.asked
		}
		switch {
		case letterPair && q.drillFB != nil:
			// Pick feedback: show the full answer (sides, cues, example).
			vm.Drill = q.drillFB
			vm.Pointed = q.asked
			vm.Unpointed = units.Plain(q.asked)
			vm.Revealed = true
		case letterPair:
			// Timed pick: blank the example (or letter clip/sound), two buttons.
			vm.Pick = true
			vm.PickLetters = parts
			vm.Asked = q.asked
			if vm.ExampleText != "" {
				vm.BlankedExample = blankAskedLetter(vm.ExampleText, q.asked)
				vm.Pointed = ""
				vm.Unpointed = ""
			} else {
				// Prompt is the asked letter's clip or sound; the glyph stays off.
				// Drop any other letter's clip fillDrill may have stored.
				vm.Pointed = ""
				vm.Unpointed = ""
				vm.AudioURL = ""
				if url, err := s.clipURL(ctx, q.asked, ""); err != nil {
					return ReviewCardVM{}, err
				} else if url != "" {
					vm.AudioURL = url
				}
			}
		case q.asked != "":
			// Single-letter reveal path: keep the posted letter.
			vm.Pointed = q.asked
			vm.Unpointed = units.Plain(q.asked)
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
		z, err := s.learnerZone(ctx, userID)
		if err != nil {
			return err
		}
		vm.Asked = pickPart(vm.CardID, studyday.DueCutoff(s.now(), z), parts)
		stored := make([]string, 0, len(parts))
		sides := make([]views.DrillSide, 0, len(parts))
		for _, part := range parts {
			sound, err := s.letterSound(ctx, part)
			if err != nil {
				return err
			}
			stored = append(stored, sound)
			side := views.DrillSide{Hebrew: part, Sound: sound}
			if e, ok := views.LookupAlefbet(part); ok {
				side.Name = e.Name
				side.NameTranslit = e.Translit
				side.Sound = e.Sound
			} else {
				side.Name = views.LetterName(part)
			}
			sides = append(sides, side)
		}
		vm.Sides = sides
		if len(parts) == 1 {
			if e, ok := views.LookupAlefbet(parts[0]); ok {
				vm.LooksLike = views.FirstAlefbetCue(e.Cue)
			}
		}
		vm.TellApart = views.AlefbetTellApart(parts)
		vm.PairNote = views.AlefbetPairNote(parts)
		vm.DageshNote = views.LetterDageshNote(parts, stored)
		vm.DageshHard = views.DageshHardNote(parts)
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
	// A two-letter pick that has an example word uses that word's clip only.
	// A missing example clip must not fall back to a pair letter, or the
	// asked letter's sound is on the page before the pick.
	pairWithExample := vm.Kind == session.KindLetter && len(parts) >= 2 && vm.ExampleText != ""
	if vm.AudioURL == "" && vm.Kind == session.KindLetter && !pairWithExample {
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
	// A single-letter front shows that letter. Pair picks set Pointed in cardVM.
	if vm.Kind == session.KindLetter && vm.Asked != "" && len(parts) < 2 {
		vm.Pointed = vm.Asked
		vm.Unpointed = units.Plain(vm.Asked)
	}
	return nil
}

// blankAskedLetter replaces the first occurrence of asked in text with a cloze blank.
func blankAskedLetter(text, asked string) string {
	if text == "" || asked == "" {
		return text
	}
	if i := strings.Index(text, asked); i >= 0 {
		return text[:i] + "___" + text[i+len(asked):]
	}
	plainAsked := units.Plain(asked)
	if plainAsked != "" && plainAsked != asked {
		if i := strings.Index(text, plainAsked); i >= 0 {
			return text[:i] + "___" + text[i+len(plainAsked):]
		}
	}
	return text
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

// learnerZone is the signed-in learner's civil-day zone (or the app zone).
// INIT-026/SPEC-006.
func (s *server) learnerZone(ctx context.Context, userID int64) (studyday.Zone, error) {
	if s == nil || s.zones == nil {
		return studyday.Zone{}, fmt.Errorf("web: study zones not configured")
	}
	return s.zones.For(ctx, userID)
}

// pickPart chooses one part of a pair. The seed is the card id plus the
// learner's study-day cutoff from studyday.DueCutoff. The same study day
// asks the same letter. The next study day can ask the other one.
// One part is returned as itself. A reveal does not call this again: the
// posted letter is what the back shows.
func pickPart(cardID int64, cutoff time.Time, parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	src := rand.NewPCG(uint64(cardID), uint64(cutoff.Unix()))
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
		MovedUp:       vm.MovedUp,
		PairsCleared:  vm.PairsCleared,
		Slow:          vm.Slow,
		NewMet:        vm.NewMet,
	}
	if vm.Item != nil {
		card.Act = activityView(lang, csrf, vm)
		return card
	}
	if vm.Pick {
		card.Pick = true
		card.PickLetters = vm.PickLetters
		card.BlankedExample = vm.BlankedExample
		card.ExampleGloss = vm.ExampleGloss
		card.AudioURL = vm.AudioURL
		if vm.ExampleText == "" {
			// No example: surface the asked letter's English sound as the prompt,
			// never its name or the tell-apart cue.
			for _, side := range vm.Sides {
				if side.Hebrew == vm.Asked && side.Sound != "" {
					card.PickSound = side.Sound
					break
				}
			}
		}
		return card
	}
	card.Sides = vm.Sides
	card.ExampleText = vm.ExampleText
	card.ExampleTranslit = vm.ExampleTranslit
	card.ExampleGloss = vm.ExampleGloss
	card.LooksLike = vm.LooksLike
	card.TellApart = vm.TellApart
	card.PairNote = vm.PairNote
	card.DageshNote = vm.DageshNote
	card.DageshHard = vm.DageshHard
	if vm.Drill != nil {
		card.AfterDrill = true
		card.DrillCorrect = vm.Drill.correct
		card.DrillCleared = vm.Drill.cleared
		card.Revealed = true
		return card
	}
	// The mode's hold decides what the front may carry. A revealed card passes through.
	return views.ConcealFront(card)
}

func reviewStatus(err error) int {
	if errors.Is(err, session.ErrNotServed) || errors.Is(err, session.ErrNotRevealed) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}
