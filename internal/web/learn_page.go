package web

import (
	"net/http"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/richtext"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func (s *server) serveLearn(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, _, err := s.pageFor(r, lang, csrf, "learn")
	if err != nil {
		serverError(w)
		return
	}
	page.Title = "Learn · " + views.AppName
	groups, err := s.learnGroups(r)
	if err != nil {
		serverError(w)
		return
	}
	render(w, r, views.LearnIndex(views.LearnIndexView{Page: page, Groups: groups}))
}

func (s *server) serveLearnOne(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	slug := r.PathValue("slug")
	row, nodes, ok := s.publishedExplainer(r, slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	page, user, err := s.pageFor(r, lang, csrf, "learn")
	if err != nil {
		serverError(w)
		return
	}
	page.Title = row.Title + " · " + views.AppName
	render(w, r, views.LearnPage(views.LearnPageView{
		Page: page, Title: row.Title, Nodes: nodes, Used: s.usedIn(r, slug, user), Often: s.often(r, slug),
	}))
}

func (s *server) often(r *http.Request, slug string) []views.LearnOften {
	if s == nil || s.store == nil {
		return nil
	}
	rows, err := s.store.CommonMistakesFor(r.Context(), slug, true)
	if err != nil {
		return nil
	}
	out := make([]views.LearnOften, 0, len(rows))
	for _, row := range rows {
		out = append(out, views.LearnOften{Wrong: row.Wrong, Right: row.Right, Explain: row.Explain})
	}
	return out
}

func (s *server) serveLearnSheet(w http.ResponseWriter, r *http.Request, _, _ string) {
	row, nodes, ok := s.publishedExplainer(r, r.PathValue("slug"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	render(w, r, views.LearnSheet(views.LearnSheetView{Title: row.Title, Nodes: nodes}))
}

func (s *server) publishedExplainer(r *http.Request, slug string) (db.Explainer, []richtext.Node, bool) {
	if s == nil || s.store == nil || slug == "" {
		return db.Explainer{}, nil, false
	}
	row, err := s.store.ExplainerFor(r.Context(), slug, true)
	if err != nil {
		return db.Explainer{}, nil, false
	}
	nodes, err := richtext.Parse(row.Body)
	if err != nil {
		return db.Explainer{}, nil, false
	}
	return row, richtext.Sanitize(nodes), true
}

// guideCovers are the card pictures from the Guides board. They decorate
// the list. A note does not have its own picture yet.
var guideCovers = []string{
	"/static/illustrations/cover-arches.svg",
	"/static/illustrations/cover-cactus.svg",
	"/static/illustrations/cover-pomegranate.svg",
	"/static/illustrations/cover-waves.svg",
	"/static/illustrations/cover-hills.svg",
	"/static/illustrations/cover-night.svg",
}

func (s *server) learnGroups(r *http.Request) ([]views.LearnGroup, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	published, err := s.store.PublishedExplainers(r.Context())
	if err != nil {
		return nil, err
	}
	unitRows, err := s.store.PatternUnits(r.Context())
	if err != nil {
		return nil, err
	}
	prereqs, err := s.store.Prereqs(r.Context())
	if err != nil {
		return nil, err
	}
	course := make([]units.CourseUnit, len(unitRows))
	for i, row := range unitRows {
		course[i] = units.CourseUnit{ID: row.ID, Position: row.Position, Scope: row.Scope, Title: row.Title, Pattern: row.Pattern}
	}
	bySlug := map[string]db.PatternInfo{}
	var slugs []string
	for _, p := range published {
		bySlug[p.Slug] = p
		slugs = append(slugs, p.Slug)
	}
	var groups []views.LearnGroup
	for _, slug := range units.PatternSequence(slugs, course, prereqs) {
		p := bySlug[slug]
		groups = append(groups, views.LearnGroup{
			Slug: p.Slug, Title: p.Title, Summary: p.Summary, Example: p.Example,
			Cover: guideCovers[len(groups)%len(guideCovers)],
		})
	}
	return groups, nil
}

func (s *server) usedIn(r *http.Request, slug string, user *auth.User) []views.LearnUsed {
	if s == nil || s.store == nil {
		return nil
	}
	scope := ""
	if user != nil {
		scope = db.LearnerScope(user.ID)
	}
	rows, err := s.store.LessonsForPattern(r.Context(), slug, scope)
	if err != nil {
		return nil
	}
	var out []views.LearnUsed
	for _, row := range rows {
		out = append(out, views.LearnUsed{
			Href:  "/lesson/" + strconv.FormatInt(row.ID, 10) + "/0",
			Title: row.Title,
		})
	}
	return out
}
