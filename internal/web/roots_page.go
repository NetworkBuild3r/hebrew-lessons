package web

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/roots"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func (s *server) serveRoots(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, _, err := s.pageFor(r, lang, csrf, "roots")
	if err != nil {
		serverError(w)
		return
	}
	page.Title = "Roots · " + views.AppName
	var rows []views.RootRow
	if s != nil && s.store != nil {
		list, err := roots.List(r.Context(), s.store)
		if err != nil {
			serverError(w)
			return
		}
		for _, row := range list {
			rows = append(rows, views.RootRow{
				Href:      page.Href("/roots/" + row.Root),
				Root:      row.Root,
				Gloss:     row.CoreGloss,
				WordCount: fmt.Sprintf("%d words", row.Words),
			})
		}
	}
	render(w, r, views.RootsIndex(views.RootsIndexView{Page: page, Roots: rows}))
}

func (s *server) serveRoot(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	root := r.PathValue("root")
	if s == nil || s.store == nil || !roots.ValidPath(root) {
		http.NotFound(w, r)
		return
	}
	ok, err := roots.Visible(r.Context(), s.store, root)
	if err != nil {
		serverError(w)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	page, user, err := s.pageFor(r, lang, csrf, "roots")
	if err != nil {
		serverError(w)
		return
	}
	var userID int64
	if user != nil {
		userID = user.ID
	}
	fam, err := roots.Family(r.Context(), s.store, s.model(), root, userID)
	if err != nil {
		serverError(w)
		return
	}
	page.Title = root + " · " + views.AppName
	view := views.RootPageView{
		Page: page, Root: fam.Root, Gloss: fam.CoreGloss, VerseHead: "In Isaiah",
	}
	for _, form := range fam.Forms {
		label := form.Tense
		for _, part := range []string{form.Person, form.Number, form.Gender} {
			if part != "" {
				label += " " + part
			}
		}
		view.Forms = append(view.Forms, views.RootFormView{Label: label, He: form.He, Known: form.Known})
	}
	for _, word := range fam.Words {
		view.Words = append(view.Words, views.Word{
			Text: word.Text, Translit: word.Translit, Gloss: word.Gloss, POS: word.POS,
			ShowText: true, ShowTranslit: word.Translit != "", ShowGloss: true,
			Flip: true, State: word.State,
		})
	}
	for _, les := range fam.Lessons {
		view.Lessons = append(view.Lessons, views.RootLessonLink{
			Href:  "/lesson/" + strconv.FormatInt(les.UnitID, 10) + "/" + strconv.Itoa(les.LessonIx),
			Title: les.Title,
		})
	}
	isaiah := false
	for _, v := range fam.Verses {
		if v.Book == "isaiah" {
			isaiah = true
		}
		spans := make([]views.RootSpan, len(v.Spans))
		for i, sp := range v.Spans {
			spans[i] = views.RootSpan{Text: sp.Text, Mark: sp.Mark}
		}
		view.Verses = append(view.Verses, views.RootVerseView{
			Ref: v.Book + " " + strconv.Itoa(v.Chapter) + ":" + strconv.Itoa(v.Verse),
			En:  v.En, Spans: spans,
		})
	}
	if !isaiah && len(view.Verses) > 0 {
		view.VerseHead = "In the Tanakh"
	}
	render(w, r, views.RootPage(view))
}
