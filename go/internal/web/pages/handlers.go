package pages

import (
	"errors"
	"net/http"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/views/components"
	guideview "github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/views/pages"
	"github.com/a-h/templ"
)

// Handler serves the guides shelf and one guide.
// The reader is anonymous until sessions land, so personal and selected
// guides stay hidden.
type Handler struct {
	src Source
}

// NewHandler returns a handler that reads guide data through src.
func NewHandler(src Source) *Handler {
	return &Handler{src: src}
}

// Register mounts the existing guide URLs. Patterns are exact: /guides/
// does not swallow later routes under that prefix.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /guides", h.List)
	mux.HandleFunc("GET /guides/{$}", h.List)
	mux.HandleFunc("GET /guides/index.html", h.List)
	mux.HandleFunc("GET /shared/plan", h.Detail)
	mux.HandleFunc("GET /shared/plan/{$}", h.Detail)
	mux.HandleFunc("GET /shared/plan/index.html", h.Detail)
	mux.HandleFunc("POST /_hx/ui", h.SetUI)
}

// List is GET /guides/.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := Guides(r.Context(), h.src, nil)
	if err != nil {
		list = guideview.GuideList{Failed: true}
	}
	writeHTML(w, r, http.StatusOK, guideview.GuidesPage(list), guideview.GuidesFragment(list))
}

// Detail is GET /shared/plan/?id=&lesson= for a published guide.
func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	guide, err := OpenGuide(r.Context(), h.src, nil, q.Get("id"), q.Get("lesson"))
	switch {
	case err == nil:
		writeHTML(w, r, http.StatusOK, guideview.GuidePage(guide), guideview.GuideFragment(guide))
	case errors.Is(err, ErrMissingID):
		writeHTML(w, r, http.StatusNotFound, guideview.MissingPlanPage(), components.MissingPlan())
	case errors.Is(err, ErrNotFound):
		writeHTML(w, r, http.StatusNotFound, guideview.NotFoundPage(), components.PlanNotFound())
	default:
		list := guideview.GuideList{Failed: true}
		writeHTML(w, r, http.StatusOK, guideview.GuidesPage(list), guideview.GuidesFragment(list))
	}
}

func writeHTML(w http.ResponseWriter, r *http.Request, status int, full, fragment templ.Component) {
	r = attachUI(w, r)
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Cache-Control", "no-store")
	h.Set("Vary", "HX-Request")
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	comp := full
	if r.Header.Get("HX-Request") == "true" {
		comp = fragment
	}
	_ = comp.Render(r.Context(), w)
}
