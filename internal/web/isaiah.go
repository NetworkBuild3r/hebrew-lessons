package web

import (
	"net/http"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/richtext"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/roots"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func (s *server) serveIsaiah(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	if s == nil || s.store == nil || !s.isaiahOpen(r) {
		http.NotFound(w, r)
		return
	}
	chapters, err := s.store.IsaiahChapters(r.Context())
	if err != nil {
		serverError(w)
		return
	}
	page, _, err := s.pageFor(r, lang, csrf, "isaiah")
	if err != nil {
		serverError(w)
		return
	}
	page.Title = "Isaiah · " + views.AppName
	view := views.IsaiahView{Page: page}
	for _, chapter := range chapters {
		ref, ok, err := s.store.IsaiahCheckedVerse(r.Context(), chapter)
		if err != nil {
			serverError(w)
			return
		}
		if !ok {
			continue
		}
		view.Chapters = append(view.Chapters, views.IsaiahChapter{
			Label: "Chapter " + strconv.Itoa(chapter),
			Href:  page.Href("/isaiah/" + strconv.Itoa(ref.Chapter) + "/" + strconv.Itoa(ref.Verse)),
		})
	}
	render(w, r, views.IsaiahPage(view))
}

func (s *server) serveIsaiahVerse(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	if s == nil || s.store == nil || !s.isaiahOpen(r) {
		http.NotFound(w, r)
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
	page, _, err := s.pageFor(r, lang, csrf, "isaiah")
	if err != nil {
		serverError(w)
		return
	}
	view, err := s.isaiahVerse(r, page, row)
	if err != nil {
		serverError(w)
		return
	}
	page.Title = view.Heading + " · " + views.AppName
	render(w, r, views.IsaiahPage(views.IsaiahView{Page: page, Verse: view}))
}

func (s *server) isaiahOpen(r *http.Request) bool {
	n, err := s.store.CheckedVerseCount(r.Context())
	return err == nil && n > 0
}

func (s *server) isaiahVerse(r *http.Request, page views.Page, row db.TanakhVerse) (*views.IsaiahVerse, error) {
	ctx := r.Context()
	notes, err := s.store.VerseNotes(ctx, "isaiah", row.Chapter, row.Verse, true)
	if err != nil {
		return nil, err
	}
	glosses, err := s.store.VerseGlosses(ctx, "isaiah", row.Chapter, row.Verse, true)
	if err != nil {
		return nil, err
	}
	audio, err := s.clipURL(ctx, row.He, "")
	if err != nil {
		return nil, err
	}
	out := &views.IsaiahVerse{
		Heading: "Isaiah " + strconv.Itoa(row.Chapter) + ":" + strconv.Itoa(row.Verse),
		He:      row.He,
		En:      row.En,
		Audio:   audio,
	}
	for _, g := range glosses {
		word := views.IsaiahWord{Text: g.WordHe, Gloss: g.GlossEn, Root: g.Root}
		if g.Root != "" {
			visible, err := roots.Visible(ctx, s.store, g.Root)
			if err != nil {
				return nil, err
			}
			if visible {
				word.RootHref = page.Href("/roots/" + g.Root)
			}
		}
		out.Words = append(out.Words, word)
	}
	for _, note := range notes {
		title := isaiahLayerTitle(note.Layer)
		if title == "" {
			continue
		}
		nodes, err := richtext.Parse(note.Body)
		if err != nil {
			continue
		}
		out.Layers = append(out.Layers, views.IsaiahLayer{Title: title, Nodes: nodes})
	}
	prev, next, hasPrev, hasNext, err := s.store.IsaiahNeighbors(ctx, row.Chapter, row.Verse)
	if err != nil {
		return nil, err
	}
	if hasPrev {
		out.PrevHref = page.Href("/isaiah/" + strconv.Itoa(prev.Chapter) + "/" + strconv.Itoa(prev.Verse))
	}
	if hasNext {
		out.NextHref = page.Href("/isaiah/" + strconv.Itoa(next.Chapter) + "/" + strconv.Itoa(next.Verse))
	}
	return out, nil
}

func isaiahLayerTitle(layer string) string {
	switch layer {
	case db.LayerGloss:
		return "Gloss"
	case db.LayerRoots:
		return "Roots"
	case db.LayerManner:
		return "The manner of the Jews"
	case db.LayerBridge:
		return "Bridge to modern Hebrew"
	default:
		return ""
	}
}

func isaiahRef(r *http.Request) (int, int, bool) {
	chapter, err1 := strconv.Atoi(r.PathValue("chapter"))
	verse, err2 := strconv.Atoi(r.PathValue("verse"))
	if err1 != nil || err2 != nil || chapter < 1 || chapter > 66 || verse < 1 || verse > 200 {
		return 0, 0, false
	}
	return chapter, verse, true
}
