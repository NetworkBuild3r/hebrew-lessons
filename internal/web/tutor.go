package web

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/speech"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// serveTutor is GET /tutor: this learner's classes, marked against the course.
func (s *server) serveTutor(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, user, err := s.pageFor(r, lang, csrf, "tutor")
	if err != nil {
		serverError(w)
		return
	}
	page.Title = "Your tutor's classes · " + views.AppName
	v := views.TutorPage{Page: page, Nothing: r.URL.Query().Get("nothing") == "1"}
	if user != nil && s.store != nil {
		classes, err := s.tutorClasses(r.Context(), user.ID)
		if err != nil {
			serverError(w)
			return
		}
		v.Classes = classes
	}
	render(w, r, views.Tutor(v))
}

// tutorClasses reads the learner's sheets and marks each word against the course.
func (s *server) tutorClasses(ctx context.Context, userID int64) ([]views.TutorClass, error) {
	sheets, err := s.store.ClassSheets(ctx, userID)
	if err != nil {
		return nil, err
	}
	course, err := s.store.CourseLexemes(ctx)
	if err != nil {
		return nil, err
	}
	byLemma := map[string][]db.CourseLexeme{}
	for _, row := range course {
		byLemma[speech.LemmaKey(row.Lemma)] = append(byLemma[speech.LemmaKey(row.Lemma)], row)
	}
	out := make([]views.TutorClass, 0, len(sheets))
	for _, sh := range sheets {
		c := views.TutorClass{ID: sh.ID, ClassOn: sh.ClassOn, Title: sh.Title, Status: sh.Status}
		seen := map[string]bool{}
		for _, line := range sh.Lines {
			c.Lines = append(c.Lines, views.TutorLine{Hebrew: line.Tutor, English: line.Gloss, Held: line.Held})
			if line.Held || line.Tutor == "" {
				continue
			}
			for _, tok := range units.Tokenize(line.Tutor) {
				key := speech.LemmaKey(tok)
				if key == "" || seen[key] {
					continue
				}
				seen[key] = true
				w := views.TutorWord{Pointed: tok, Where: "other"}
				if rows := byLemma[key]; len(rows) > 0 {
					w.Where = "course"
					w.English = rows[0].Lemma
					c.InCourse++
				} else if line.Gloss != "" {
					w.Where = "new"
					w.English = line.Gloss
					c.NewWords++
				}
				c.Words = append(c.Words, w)
			}
		}
		c.CanStudy = c.NewWords > 0 || c.InCourse > 0
		if c.CanStudy && sh.Status != "passed" {
			c.CanStudy = false
		}
		out = append(out, c)
	}
	return out, nil
}

// storedSpellings is every stored lexeme's consonant spelling, with every id that
// has it. Two ids under one spelling means the spelling is ambiguous.
func (s *server) storedSpellings(ctx context.Context) (map[string][]int64, error) {
	return s.store.LexemeIDsBySpelling(ctx)
}

// postTutorStudy is POST /_hx/tutor/{id}/study. It studies one class's words,
// adding the ones the lexicon does not have yet. A word whose spelling is shared
// by two stored rows is skipped, because choosing one would be a guess.
func (s *server) postTutorStudy(w http.ResponseWriter, r *http.Request, lang, _ string) {
	back := "/tutor"
	if lang == "he" {
		back = "/tutor?lang=he"
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
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	sheets, err := s.store.ClassSheets(r.Context(), user.ID)
	if err != nil {
		serverError(w)
		return
	}
	var sheet db.ClassSheet
	found := false
	for _, sh := range sheets {
		if sh.ID == id {
			sheet, found = sh, true
			break
		}
	}
	if !found {
		notFound(w, r)
		return
	}
	ids, err := s.tutorStudyIDs(r.Context(), sheet)
	if err != nil {
		serverError(w)
		return
	}
	if len(ids) == 0 {
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, back+"?nothing=1", http.StatusSeeOther)
		return
	}
	built, err := s.study.BuildLesson(r.Context(), user.ID, ids)
	if err != nil {
		serverError(w)
		return
	}
	if err := s.replaceQueue(r.Context(), key, user.ID, built); err != nil {
		serverError(w)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	review := "/review"
	if lang == "he" {
		review = "/review?lang=he"
	}
	http.Redirect(w, r, review, http.StatusSeeOther)
}

// tutorStudyIDs resolves a class's words to lexeme ids, creating the ones that
// are not stored yet.
func (s *server) tutorStudyIDs(ctx context.Context, sheet db.ClassSheet) ([]int64, error) {
	stored, err := s.storedSpellings(ctx)
	if err != nil {
		return nil, err
	}
	var ids []int64
	seen := map[int64]bool{}
	for _, line := range sheet.Lines {
		if line.Held || line.Tutor == "" {
			continue
		}
		for _, tok := range units.Tokenize(line.Tutor) {
			if tok == "" {
				continue
			}
			text := units.Normalize(tok)
			spelling := units.StripNiqqud(text)
			if list := stored[spelling]; len(list) > 0 {
				if len(list) == 1 && !seen[list[0]] {
					seen[list[0]] = true
					ids = append(ids, list[0])
				}
				continue
			}
			gloss := strings.TrimSpace(line.Gloss)
			if gloss == "" {
				continue
			}
			pos := "noun"
			translit := "?"
			in := lexicon.Input{TextNFC: text, Gloss: gloss, Translit: &translit, POS: &pos}
			created, _, err := s.store.UpsertLexemes(ctx, []lexicon.Input{in})
			if err != nil {
				continue
			}
			for _, row := range created {
				if row.ID > 0 && !seen[row.ID] {
					seen[row.ID] = true
					ids = append(ids, row.ID)
				}
			}
		}
	}
	return ids, nil
}
