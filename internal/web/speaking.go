package web

import (
	"net/http"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lessons"
)

// serveSpeaking is GET /me/speaking. It is the copyable list of this learner's
// class words and the tutor's lines, for a speaking session outside the app.
// Another learner's sheets are not included. INIT-039/SPEC-014.
func (s *server) serveSpeaking(w http.ResponseWriter, r *http.Request, _, _ string) {
	if s == nil || s.store == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	user, ok := auth.CurrentUser(r.Context())
	if !ok || user == nil {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	sheets, err := s.store.ClassSheets(r.Context(), user.ID)
	if err != nil {
		serverError(w)
		return
	}
	var classOn string
	var words, lines []string
	for _, sh := range sheets {
		if sh.Status != "passed" {
			continue
		}
		if classOn == "" {
			classOn = sh.ClassOn
		}
		for _, line := range sh.Lines {
			if line.Held || line.Tutor == "" {
				continue
			}
			lines = append(lines, line.Tutor)
			if line.Gloss != "" {
				words = append(words, line.Tutor+" — "+line.Gloss)
			}
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(lessons.SpeakingPrep(classOn, words, lines)))
}
