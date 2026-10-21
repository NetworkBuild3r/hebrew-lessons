package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/activity"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/session"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// The checked activities of the review session (INIT-015/SPEC-016).
//
// The session mix chooses an activity for each card (session.CardRef.Activity). When a
// card is served, prepareItem builds the item with activity.Engine.Build. The item
// carries no answer, so nothing about the word is in the page until Check has run. The
// learner picks, pairs or types, and POST /_hx/review/answer checks the response with
// the item token and shows the feedback sheet. POST /_hx/review/next is Continue. A
// flashcard is still graded with /_hx/review/reveal and /_hx/review/grade.
//
// The learner is the signed-in one, as in the other review handlers: the queue is found
// by the auth session id and its userID is compared with the signed-in user, and Check
// refuses a token issued to another learner.

// maxAnswerBody bounds an answer post. A typed answer is at most
// activity.MaxResponseBytes, and a match item posts five small numbers.
const maxAnswerBody = 8 << 10

// answerState is a checked answer, kept until the learner continues so a reload of
// /review shows the same feedback.
type answerState struct {
	item activity.Item
	resp activity.Response
	// text is what the learner typed, trimmed. It reaches the page only through templ.
	text string
	res  activity.Result
}

// prepareItem records the exposure of a new word and builds the item the visit is asked
// through. A visit whose activity the lexicon cannot support after all stays a
// flashcard. The caller holds s.mu.
func (s *server) prepareItem(ctx context.Context, q *studyQueue) error {
	ref := q.cur
	if s.engine == nil || ref.LexemeID <= 0 {
		return nil
	}
	// A mode that is not a lexeme stays a flashcard. Letter and pattern are
	// those modes. A later one is too, once its entry says Lexeme is false.
	if mode, ok := views.LookupReviewMode(ref.Kind); ok && !mode.Lexeme {
		return nil
	}
	// A visit that comes back after a miss has shown the word already.
	if ref.Intro && !q.retry {
		if err := s.engine.Expose(ctx, q.userID, []int64{ref.LexemeID}, 0); err != nil {
			return err
		}
	}
	if ref.Activity == "" || ref.Activity == activity.Flashcard {
		return nil
	}
	item, ok, err := s.buildItem(ctx, q.userID, ref)
	if err != nil {
		return err
	}
	if ok {
		q.item = &item
	}
	return nil
}

// buildItem builds the item for a card. ok is false when the lexicon cannot support the
// activity, which is not an error: the visit becomes a flashcard.
func (s *server) buildItem(ctx context.Context, userID int64, ref session.CardRef) (activity.Item, bool, error) {
	item, err := s.engine.Build(ctx, userID, ref.LexemeID, ref.Activity, activity.WithSource(evidence.SourceReview))
	switch {
	case err == nil:
		return item, true, nil
	case errors.Is(err, activity.ErrNoGloss), errors.Is(err, activity.ErrNoAudio),
		errors.Is(err, activity.ErrNotEnoughWords), errors.Is(err, activity.ErrUnknownWord):
		return activity.Item{}, false, nil
	default:
		return activity.Item{}, false, err
	}
}

// answer checks the response to the current item and returns the card with feedback.
// A post for any card but the one served, with any token but the item's, or after the
// item was answered is 400. A response that does not fit the item is 400 and leaves the
// item pending. An item that expired, or that the engine no longer holds, is built
// again and the learner sees the new question.
func (s *server) answer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAnswerBody)
	cardID, ok := reviewCardID(w, r)
	if !ok {
		return
	}
	key, user, ok := s.sessionKey(r)
	if !ok || user == nil || s.engine == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	token := r.PostForm.Get("token")
	lang := reviewLang(r)

	s.mu.Lock()
	q := s.queues[key]
	if q == nil || q.userID != user.ID || q.sess == nil || q.sess.UserID() != user.ID || q.done || !q.has ||
		q.cur.CardID != cardID || q.item == nil || q.fb != nil || token == "" || token != q.item.Token ||
		!sameStudyDay(q.builtAt, s.now()) {
		s.mu.Unlock()
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	resp, err := responseFrom(r.PostForm, *q.item)
	if err != nil {
		s.mu.Unlock()
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	res, err := s.engine.Check(r.Context(), user.ID, token, resp)
	switch {
	case err == nil:
	case errors.Is(err, activity.ErrTokenExpired), errors.Is(err, activity.ErrTokenUnknown):
		item, built, berr := s.buildItem(r.Context(), user.ID, q.cur)
		if berr != nil || !built {
			s.mu.Unlock()
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		q.item = &item
		vm, verr := s.cardVM(r.Context(), q)
		s.mu.Unlock()
		if verr != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		s.finishReview(w, r, lang, vm)
		return
	case errors.Is(err, activity.ErrBadResponse), errors.Is(err, activity.ErrTokenInvalid), errors.Is(err, activity.ErrTokenUser):
		s.mu.Unlock()
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	default:
		s.mu.Unlock()
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := q.sess.Answered(cardID, res.Correct); err != nil {
		s.mu.Unlock()
		status := reviewStatus(err)
		http.Error(w, http.StatusText(status), status)
		return
	}
	q.answered++
	if res.Correct {
		q.correct++
	}
	// A miss puts the visit back once, so the session is one longer.
	_, q.total, _ = q.sess.Place()
	q.fb = &answerState{item: *q.item, resp: resp, text: strings.TrimSpace(resp.Text), res: res}
	vm, err := s.cardVM(r.Context(), q)
	s.mu.Unlock()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	s.finishReview(w, r, lang, vm)
}

// next is Continue: it serves the card after a checked answer.
func (s *server) next(w http.ResponseWriter, r *http.Request) {
	cardID, ok := reviewCardID(w, r)
	if !ok {
		return
	}
	key, user, ok := s.sessionKey(r)
	if !ok || user == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	lang := reviewLang(r)
	s.mu.Lock()
	q := s.queues[key]
	if q == nil || q.userID != user.ID || q.sess == nil || q.done || !q.has || q.cur.CardID != cardID ||
		q.item == nil || q.fb == nil || !sameStudyDay(q.builtAt, s.now()) {
		s.mu.Unlock()
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if err := s.advance(r.Context(), q); err != nil {
		s.mu.Unlock()
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	vm, err := s.cardVM(r.Context(), q)
	s.mu.Unlock()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	s.finishReview(w, r, lang, vm)
}

var errBadAnswer = errors.New("web: the response does not fit the item")

// responseFrom reads the learner's response from the form. The engine checks it again
// against the item (bounds, a permutation for a match, a length for typed text).
func responseFrom(form url.Values, item activity.Item) (activity.Response, error) {
	resp := activity.Response{Hinted: form.Get("hinted") == "1"}
	if form.Get("dont_know") == "1" {
		resp.DontKnow = true
		return resp, nil
	}
	switch item.Kind {
	case activity.ListenPick, activity.ReadPick:
		n, err := strconv.Atoi(form.Get("choice"))
		if err != nil {
			return resp, errBadAnswer
		}
		resp.Choice = n
	case activity.WordMatch, activity.AudioMatch:
		for i := range item.Left {
			n, err := strconv.Atoi(form.Get("pair_" + strconv.Itoa(i)))
			if err != nil {
				return resp, errBadAnswer
			}
			resp.Pairs = append(resp.Pairs, n)
		}
	case activity.TypeIt, activity.Dictation:
		resp.Text = form.Get("text")
		if strings.TrimSpace(resp.Text) == "" {
			return resp, errBadAnswer
		}
	default:
		return resp, errBadAnswer
	}
	return resp, nil
}

// audioURL is where a clip is served. An empty id has no clip.
func audioURL(id string) string {
	if id == "" {
		return ""
	}
	return "/audio/" + id + ".mp3"
}

func leadFor(k activity.Kind) string {
	switch k {
	case activity.ListenPick:
		return "Listen. What does it mean?"
	case activity.ReadPick:
		return "What does it mean?"
	case activity.WordMatch:
		return "Match each word to its meaning."
	case activity.AudioMatch:
		return "Match each clip to its meaning."
	case activity.TypeIt:
		return "Type it in Hebrew."
	case activity.Dictation:
		return "Type what you hear."
	default:
		return ""
	}
}

// activityView turns a served item, and its answer when there is one, into the screen.
func activityView(lang, csrf string, vm ReviewCardVM) *views.ActivityView {
	it := *vm.Item
	a := &views.ActivityView{
		Kind: string(it.Kind), Phase: views.ActAsk, CSRF: csrf, Lang: lang,
		CardID: vm.CardID, Token: it.Token, Index: vm.Index, Total: vm.Total, Lead: leadFor(it.Kind),
	}
	st := vm.Answer
	if st != nil {
		a.Phase = views.ActFeedback
		a.Feedback = feedbackView(it, st)
	}
	switch it.Kind {
	case activity.ListenPick, activity.ReadPick:
		if it.Kind == activity.ListenPick {
			a.AudioURL = audioURL(it.Prompt.AudioID)
		} else {
			a.Hebrew = it.Prompt.Hebrew
		}
		right := -1
		if st != nil {
			for i, o := range it.Options {
				if o == st.res.Answer {
					right = i
				}
			}
		}
		for i, o := range it.Options {
			state := views.AnswerDefault
			switch {
			case st == nil:
			case i == right:
				state = views.AnswerCorrect
			case !st.resp.DontKnow && i == st.resp.Choice:
				state = views.AnswerWrong
			}
			a.Options = append(a.Options, views.OptionView{Text: o, State: state})
		}
	case activity.TypeIt:
		a.Gloss = it.Prompt.Gloss
	case activity.Dictation:
		a.AudioURL = audioURL(it.Prompt.AudioID)
	case activity.WordMatch, activity.AudioMatch:
		matchTiles(a, it, st)
	}
	return a
}

// matchTiles fills the two columns. After an answer each pair is marked, and both of its
// tiles carry the pair number.
func matchTiles(a *views.ActivityView, it activity.Item, st *answerState) {
	for i, c := range it.Left {
		t := views.TileView{Index: i, Key: strconv.Itoa(i + 1), State: views.AnswerDefault}
		if c.Hebrew != "" {
			t.Text, t.Hebrew = c.Hebrew, true
			t.Name = fmt.Sprintf("%d. Hebrew word: %s", i+1, c.Hebrew)
			a.HebrewCol = true
		} else {
			t.AudioURL = audioURL(c.AudioID)
			t.Name = fmt.Sprintf("%d. Clip %d: plays the word", i+1, i+1)
		}
		a.Left = append(a.Left, t)
	}
	for j, g := range it.Right {
		key := string(views.MatchKeys[j%len(views.MatchKeys)])
		a.Right = append(a.Right, views.TileView{
			Text: g, Index: j, Key: key, State: views.AnswerDefault,
			Name: fmt.Sprintf("%s. Meaning: %s", key, g),
		})
	}
	if st == nil {
		return
	}
	for _, p := range st.res.Pairs {
		if p.Left < 0 || p.Left >= len(a.Left) {
			continue
		}
		state, word := views.AnswerWrong, "wrong"
		if p.Correct {
			state, word = views.AnswerCorrect, "right"
		}
		num := strconv.Itoa(p.Left + 1)
		a.Left[p.Left].State, a.Left[p.Left].Pair = state, num
		a.Left[p.Left].Name += ", " + word
		if p.Chosen >= 0 && p.Chosen < len(a.Right) {
			a.Right[p.Chosen].State, a.Right[p.Chosen].Pair = state, num
			a.Right[p.Chosen].Name += fmt.Sprintf(", paired with %d, %s", p.Left+1, word)
		}
	}
}

// feedbackView builds the sheet from a checked answer: the right response, its pointed
// form and clip, what the learner mixed up, and for typing the wrong letters.
func feedbackView(it activity.Item, st *answerState) *views.FeedbackView {
	res := st.res
	f := &views.FeedbackView{
		Correct: res.Correct, NearMiss: res.NearMiss, DontKnow: st.resp.DontKnow,
		Answer: res.Answer, Hebrew: res.Text, Gloss: res.Gloss, AudioURL: audioURL(res.AudioID),
	}
	multi := it.Kind == activity.WordMatch || it.Kind == activity.AudioMatch
	switch {
	case st.resp.DontKnow:
		f.Title = "Here is the answer"
	case res.NearMiss:
		f.Title = "Almost. One letter off."
	case res.Correct && multi:
		f.Title = "Every pair is right"
	case res.Correct:
		f.Title = "Correct"
	case multi:
		right := 0
		for _, p := range res.Pairs {
			if p.Correct {
				right++
			}
		}
		f.Title = fmt.Sprintf("%d of %d pairs right", right, len(res.Pairs))
	default:
		f.Title = "Not quite"
	}

	switch {
	case multi:
		for _, p := range res.Pairs {
			if !p.Correct {
				f.Pairs = append(f.Pairs, views.PairView{Hebrew: p.Text, Gloss: p.Gloss, AudioURL: audioURL(p.AudioID)})
			}
		}
		// Each wrong pair was logged in order, so the nth confusion is the nth wrong pair.
		k := 0
		for _, p := range res.Pairs {
			if p.Correct || p.Chosen < 0 {
				continue
			}
			if k < len(res.Confused) && res.Confused[k].With != "" && p.Chosen < len(it.Right) {
				f.Confusions = append(f.Confusions, views.ConfusionView{Lead: "You picked", Picked: it.Right[p.Chosen], Thats: res.Confused[k].With})
			}
			k++
		}
	case it.Kind == activity.TypeIt || it.Kind == activity.Dictation:
		if !st.resp.DontKnow && (!res.Correct || res.NearMiss) {
			f.TypedLabel = "You typed"
			f.Typed = typedSegments(res.Text, st.text)
		}
		for _, c := range res.Confused {
			if c.With != "" {
				f.Confusions = append(f.Confusions, views.ConfusionView{Lead: "You typed", Picked: st.text, PickedHebrew: true, Thats: c.With})
			}
		}
	default:
		if !st.resp.DontKnow && st.resp.Choice >= 0 && st.resp.Choice < len(it.Options) {
			for _, c := range res.Confused {
				if c.With != "" {
					f.Confusions = append(f.Confusions, views.ConfusionView{Lead: "You picked", Picked: it.Options[st.resp.Choice], Thats: c.With})
				}
			}
		}
	}
	return f
}

// typedSegments is what the learner typed, in runs, with the letters that are not in the
// right answer marked. A letter the learner left out is a Missing run. Align compares
// without niqqud, as grading does, so the typed text is shown without it.
func typedSegments(expected, typed string) []views.SegView {
	_, edits := activity.Align(expected, typed)
	var segs []views.SegView
	add := func(text string, wrong, missing bool) {
		if n := len(segs); n > 0 && segs[n-1].Wrong == wrong && segs[n-1].Missing == missing && !missing {
			segs[n-1].Text += text
			return
		}
		segs = append(segs, views.SegView{Text: text, Wrong: wrong, Missing: missing})
	}
	for _, e := range edits {
		switch e.Op {
		case activity.Match:
			add(string(e.Typed), false, false)
		case activity.Sub, activity.Ins:
			add(string(e.Typed), true, false)
		case activity.Del:
			add("", false, true)
		}
	}
	return segs
}
