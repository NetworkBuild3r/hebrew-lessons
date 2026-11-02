package web

import (
	"net/http"
)

// drill is POST /_hx/review/drill: a timed two-option pick for a letter pair.
// CSRF and Origin are the checks on /_hx/ in front of this handler.
// INIT-027/SPEC-006, ADR-0023 D1.
func (s *server) drill(w http.ResponseWriter, r *http.Request) {
	cardID, ok := reviewCardID(w, r)
	if !ok {
		return
	}
	key, user, ok := s.sessionKey(r)
	if !ok || user == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	pick := r.PostForm.Get("pick")
	lang := reviewLang(r)

	s.mu.Lock()
	q := s.queues[key]
	parts := pairParts(q)
	if q == nil || q.userID != user.ID || q.sess == nil || q.sess.UserID() != user.ID ||
		q.done || !q.has || q.cur.CardID != cardID || q.item != nil || q.drillFB != nil ||
		parts == nil || q.asked == "" || q.servedAt.IsZero() || !q.sameDay(s.now()) {
		s.mu.Unlock()
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	dontKnow := pick == "dont_know"
	if !dontKnow && !askedInPair(parts, pick) {
		s.mu.Unlock()
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	correct := !dontKnow && pick == q.asked
	latency := s.now().Sub(q.servedAt)
	out, err := q.sess.DrillAnswered(r.Context(), cardID, correct, latency)
	if err != nil {
		s.mu.Unlock()
		status := reviewStatus(err)
		http.Error(w, http.StatusText(status), status)
		return
	}
	q.answered++
	if correct {
		q.correct++
	}
	_, q.total, _ = q.sess.Place()
	q.drillFB = &drillState{correct: correct, cleared: out.Cleared}
	vm, err := s.cardVM(r.Context(), q)
	s.mu.Unlock()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	s.finishReview(w, r, lang, vm)
}
