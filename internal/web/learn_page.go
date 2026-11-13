package web

import (
	"net/http"
	"strconv"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/auth"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/richtext"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/speech"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// serveBinyanIndex is GET /learn/verbs: the seven binyanim, and a way into each verb.
// The numbers and examples come from the signed course list. INIT-040.
func (s *server) serveBinyanIndex(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, _, err := s.pageFor(r, lang, csrf, "learn")
	if err != nil {
		serverError(w)
		return
	}
	page.Title = "Verbs · " + views.AppName
	list, err := speech.BinyanList()
	if err != nil {
		serverError(w)
		return
	}
	verbs, err := speech.VerbFormList(60)
	if err != nil {
		serverError(w)
		return
	}
	counts, err := speech.BinyanCounts()
	if err != nil {
		serverError(w)
		return
	}
	v := views.VerbsIndex{Page: page}
	total := 0
	for _, b := range list {
		v.Binyanim = append(v.Binyanim, views.BinyanCard{
			Key: b.Key, Name: b.Name, Translit: b.Translit, English: b.English,
			Shape: b.Shape, Count: b.Count, Example: b.Example, Gloss: b.ExampleGloss,
			Root: b.ExampleRoot, Href: "/learn/verbs/" + b.Key,
		})
	}
	for key, n := range counts {
		_ = key
		total += n
	}
	v.Total = total
	for _, verb := range verbs {
		v.Verbs = append(v.Verbs, views.VerbRow{
			Href:   "/learn/verbs/word/" + verb.Word.Lemma,
			Text:   verb.Word.PointedOrLemma(),
			Gloss:  verb.Word.Gloss,
			Root:   verb.Root,
			Binyan: verb.Binyan.Translit,
			Tenses: joinComma(verb.Tenses),
		})
	}
	v.Note = "These forms come from your own course list, so each verb shows only the tenses that list actually holds."
	render(w, r, views.VerbsPageView(v))
}

// serveVerb is GET /learn/verbs/{key}: one verb in full.
func (s *server) serveVerb(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, _, err := s.pageFor(r, lang, csrf, "learn")
	if err != nil {
		serverError(w)
		return
	}
	lemma := r.PathValue("lemma")
	verb, ok, err := speech.VerbByLemma(lemma)
	if err != nil {
		serverError(w)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	page.Title = verb.Word.PointedOrLemma() + " · " + views.AppName
	v := views.VerbPage{
		Page: page, Text: verb.Word.PointedOrLemma(), Gloss: verb.Word.Gloss,
		Root: verb.Root, Forms: verb.Forms, Tenses: verb.Tenses,
		Past:    endingRows(speech.PastEndings),
		Present: endingRows(speech.PresentEndings),
		Future:  endingRows(speech.FuturePrefixes),
		Note:    "These spellings are the ones your course list holds for this verb, without the vowel points it stores them without.",
	}
	v.Binyan = views.BinyanCard{
		Key: verb.Binyan.Key, Name: verb.Binyan.Name, Translit: verb.Binyan.Translit,
		English: verb.Binyan.English, Shape: verb.Binyan.Shape,
	}
	render(w, r, views.ConjugationView(v))
}

// serveCustoms is GET /learn/customs: the ways of the Jews that Isaiah leans on.
// Reference only, never graded (ADR-0035 D5). INIT-040.
func (s *server) serveCustoms(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, _, err := s.pageFor(r, lang, csrf, "learn")
	if err != nil {
		serverError(w)
		return
	}
	page.Title = "The ways of the Jews · " + views.AppName
	render(w, r, views.CustomsPageView(views.CustomsPage{Page: page, Groups: views.CustomsByGroup()}))
}

// endingRows turns the rule tables into rows the template renders.
func endingRows(rows []speech.Ending) []views.EndingRow {
	out := make([]views.EndingRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, views.EndingRow{
			Pronoun: row.Person.Hebrew, Translit: row.Person.Translit,
			English: row.Person.English, Suffix: row.Suffix, Note: row.Note,
		})
	}
	return out
}

// joinComma joins a short list in plain English.
func joinComma(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	}
	out := items[0]
	for _, item := range items[1 : len(items)-1] {
		out += ", " + item
	}
	return out + " and " + items[len(items)-1]
}

// serveAlefbet is GET /learn/alefbet: the letter table as a page.
func (s *server) serveAlefbet(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, _, err := s.pageFor(r, lang, csrf, "learn")
	if err != nil {
		serverError(w)
		return
	}
	page.Title = "The alphabet · " + views.AppName
	render(w, r, views.AlefbetPageView(views.AlefbetPage{Page: page, Forms: views.Alefbet}))
}

// serveVowels is GET /learn/vowels: the vowel marks as a page.
func (s *server) serveVowels(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, _, err := s.pageFor(r, lang, csrf, "learn")
	if err != nil {
		serverError(w)
		return
	}
	page.Title = "The vowels · " + views.AppName
	render(w, r, views.VowelsPageView(views.VowelsPage{Page: page, Marks: views.VowelMarks}))
}

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
	groups = append(groups,
		views.LearnGroup{
			Slug: "alefbet", Title: "The alphabet", Href: "/learn/alefbet",
			Summary: "All twenty-two letters, their names, and the sounds they make.",
			Example: "א", Cover: "/static/illustrations/cover-arches.svg",
		},
		views.LearnGroup{
			Slug: "vowels", Title: "The vowels", Href: "/learn/vowels",
			Summary: "The marks under and above the letters, and what each one sounds like.",
			Example: "אַ", Cover: "/static/illustrations/cover-waves.svg",
		},
		views.LearnGroup{
			Slug: "verbs", Title: "Verbs", Href: "/learn/verbs",
			Summary: "Three root letters, seven patterns, and the endings that tell you who did it.",
			Example: "כָּתַב", Cover: "/static/illustrations/cover-pomegranate.svg",
		},
		views.LearnGroup{
			Slug: "customs", Title: "The ways of the Jews", Href: "/learn/customs",
			Summary: "The days, the Temple, and the words Isaiah assumes you already know.",
			Example: "שַׁבָּת", Cover: "/static/illustrations/cover-night.svg",
		},
	)
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
