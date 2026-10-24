package web

import (
	"net/http"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func (s *server) serveAdminIsaiah(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	if s == nil || s.store == nil {
		http.NotFound(w, r)
		return
	}
	user, ok := auth.CurrentUser(r.Context())
	if !ok || user.Role != auth.RoleAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	chapter, verse, ok := isaiahRef(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	row, found, err := s.store.TanakhVerse(r.Context(), "isaiah", chapter, verse)
	if err != nil {
		serverError(w)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		if err := s.checkIsaiahVerse(r, user.ID, chapter, verse); err != nil {
			serverError(w)
			return
		}
		http.Redirect(w, r, "/admin/isaiah/"+strconv.Itoa(chapter)+"/"+strconv.Itoa(verse), http.StatusSeeOther)
		return
	}
	notes, err := s.store.VerseNotes(r.Context(), "isaiah", chapter, verse, false)
	if err != nil {
		serverError(w)
		return
	}
	glosses, err := s.store.VerseGlosses(r.Context(), "isaiah", chapter, verse, false)
	if err != nil {
		serverError(w)
		return
	}
	page, _, err := s.pageFor(r, lang, csrf, "isaiah")
	if err != nil {
		serverError(w)
		return
	}
	view := views.AdminIsaiahView{
		Page:    page,
		Heading: "Check Isaiah " + strconv.Itoa(chapter) + ":" + strconv.Itoa(verse),
		He:      row.He,
	}
	page.Title = view.Heading + " · " + views.AppName
	view.Page = page
	for _, note := range notes {
		view.Notes = append(view.Notes, views.AdminIsaiahNote{Layer: note.Layer, Body: note.Body, Status: note.Status})
		if note.Status != "checked" {
			view.CanCheck = true
		}
	}
	for _, gloss := range glosses {
		view.Glosses = append(view.Glosses, views.AdminIsaiahGloss{Word: gloss.WordHe, Gloss: gloss.GlossEn, Status: gloss.Status})
		if gloss.Status != "checked" {
			view.CanCheck = true
		}
	}
	render(w, r, views.AdminIsaiahPage(view))
}

func (s *server) checkIsaiahVerse(r *http.Request, checkerID int64, chapter, verse int) error {
	notes, err := s.store.VerseNotes(r.Context(), "isaiah", chapter, verse, false)
	if err != nil {
		return err
	}
	for _, note := range notes {
		if note.Status == "checked" {
			continue
		}
		if err := s.store.CheckVerseNote(r.Context(), "isaiah", chapter, verse, note.Layer, checkerID); err != nil {
			return err
		}
	}
	glosses, err := s.store.VerseGlosses(r.Context(), "isaiah", chapter, verse, false)
	if err != nil {
		return err
	}
	for _, gloss := range glosses {
		if gloss.Status == "checked" {
			continue
		}
		if err := s.store.CheckVerseGloss(r.Context(), "isaiah", chapter, verse, gloss.Position); err != nil {
			return err
		}
	}
	return nil
}
