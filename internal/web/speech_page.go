package web

import (
	"context"
	"net/http"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func (s *server) serveSpeech(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, user, err := s.pageFor(r, lang, csrf, "today")
	if err != nil {
		serverError(w)
		return
	}
	page.Title = "Common words · " + views.AppName
	view := views.CoursePage{Page: page}
	if user != nil && s.store != nil {
		filled, err := s.courseView(r.Context(), user.ID)
		if err != nil {
			serverError(w)
			return
		}
		filled.Page = page
		view = filled
	}
	render(w, r, views.Speech(view))
}

// postCourseReview replaces the queue with this learner's slipping course words.
// The ids are read from the learner's record. A posted id is ignored.
func (s *server) postCourseReview(w http.ResponseWriter, r *http.Request, lang, _ string) {
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
	ids, err := s.slipLexemeIDs(r.Context(), user.ID)
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

func (s *server) slipLexemeIDs(ctx context.Context, userID int64) ([]int64, error) {
	course, err := s.store.CourseLexemes(ctx)
	if err != nil {
		return nil, err
	}
	have, err := s.store.LearnerCardLexemeIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	met := map[int64]bool{}
	for _, row := range course {
		if _, ok := have[row.LexemeID]; ok {
			met[row.LexemeID] = true
		}
	}
	var ids []int64
	seen := map[int64]bool{}
	for _, state := range []learner.State{learner.Struggling, learner.Regressed} {
		rows, err := s.listStates(ctx, userID, learner.Filter{State: state})
		if err != nil {
			return nil, err
		}
		for _, w := range rows {
			if met[w.LexemeID] && !seen[w.LexemeID] {
				seen[w.LexemeID] = true
				ids = append(ids, w.LexemeID)
			}
		}
	}
	return ids, nil
}
