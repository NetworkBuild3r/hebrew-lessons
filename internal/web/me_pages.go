package web

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/roots"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

func (s *server) serveWords(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, user, err := s.pageFor(r, lang, csrf, "me")
	if err != nil {
		serverError(w)
		return
	}
	state := r.URL.Query().Get("state")
	switch state {
	case "", "learning", "known", "struggling":
	default:
		http.Error(w, "unknown state", http.StatusBadRequest)
		return
	}
	page.Title = "Words · " + views.AppName
	view := views.WordsView{
		Page: page, State: state, Empty: "Words you have met appear here.",
		AllHref: stateHref(page, "/me/words", ""), LearningHref: stateHref(page, "/me/words", "learning"),
		KnownHref: stateHref(page, "/me/words", "known"), StrugglingHref: stateHref(page, "/me/words", "struggling"),
	}
	if user != nil && s != nil && s.store != nil {
		words, err := s.metWords(r.Context(), user.ID, state)
		if err != nil {
			serverError(w)
			return
		}
		visible, err := s.visibleRoots(r.Context())
		if err != nil {
			serverError(w)
			return
		}
		texts := make([]string, len(words))
		for i, word := range words {
			texts[i] = word.Text
		}
		lex, err := s.store.LexemesByText(r.Context(), texts)
		if err != nil {
			serverError(w)
			return
		}
		for _, word := range words {
			row := views.StuffWord{Text: word.Text, Gloss: word.Gloss, State: string(word.State)}
			if lx, ok := lex[word.Text]; ok && lx.Root != "" && visible[lx.Root] {
				row.Root = lx.Root
				row.Href = page.Href("/roots/" + lx.Root)
			}
			view.Words = append(view.Words, row)
		}
	}
	render(w, r, views.WordsPage(view))
}

func (s *server) serveVerbs(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, user, err := s.pageFor(r, lang, csrf, "me")
	if err != nil {
		serverError(w)
		return
	}
	page.Title = "Verbs · " + views.AppName
	view := views.VerbsView{Page: page, Empty: "Verb tables appear here once you know a form."}
	if user != nil && s != nil && s.store != nil {
		tables, err := s.verbTables(r.Context(), user.ID)
		if err != nil {
			serverError(w)
			return
		}
		view.Verbs = tables
	}
	render(w, r, views.VerbsPage(view))
}

func (s *server) serveMistakes(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	page, user, err := s.pageFor(r, lang, csrf, "me")
	if err != nil {
		serverError(w)
		return
	}
	page.Title = "Mistakes · " + views.AppName
	view := views.MistakesView{Page: page, Empty: "Mistakes you keep making appear here."}
	if user != nil && s != nil && s.store != nil {
		rows, err := s.store.MistakesFor(r.Context(), user.ID, 50)
		if err != nil {
			serverError(w)
			return
		}
		for _, row := range rows {
			item := views.MistakeRow{
				Wrong: row.Wrong, Right: row.Right, Note: row.Note,
				Times: strconv.Itoa(row.Times),
			}
			if row.Pattern != "" {
				if expl, err := s.store.ExplainerFor(r.Context(), row.Pattern, true); err == nil && expl.Status == "published" {
					item.HowHref = page.Href("/learn/" + row.Pattern)
					item.How = expl.Title
				}
			}
			view.Mistakes = append(view.Mistakes, item)
		}
	}
	render(w, r, views.MistakesPage(view))
}

func stateHref(page views.Page, path, state string) string {
	if state != "" {
		path += "?state=" + url.QueryEscape(state)
	}
	if page.Lang != "he" {
		return path
	}
	if strings.Contains(path, "?") {
		return path + "&lang=he"
	}
	return path + "?lang=he"
}

func (s *server) metWords(ctx context.Context, userID int64, filter string) ([]learner.WordState, error) {
	states := []learner.State{learner.Seen, learner.Learning, learner.Struggling, learner.Regressed, learner.Known, learner.Solid}
	switch filter {
	case "learning":
		states = []learner.State{learner.Learning}
	case "struggling":
		states = []learner.State{learner.Struggling}
	case "known":
		states = []learner.State{learner.Known, learner.Solid}
	}
	var out []learner.WordState
	model := s.model()
	for _, st := range states {
		cursor := learner.Cursor("")
		for pages := 0; pages < 20; pages++ {
			page, next, err := model.List(ctx, userID, learner.Filter{State: st, Limit: learner.MaxPageSize, After: cursor})
			if err != nil {
				return nil, err
			}
			out = append(out, page...)
			if next == "" {
				break
			}
			cursor = next
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Text < out[j].Text })
	return out, nil
}

func (s *server) visibleRoots(ctx context.Context) (map[string]bool, error) {
	list, err := roots.List(ctx, s.store)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, row := range list {
		out[row.Root] = true
	}
	return out, nil
}

func (s *server) verbTables(ctx context.Context, userID int64) ([]views.VerbTable, error) {
	met, err := s.metWords(ctx, userID, "known")
	if err != nil {
		return nil, err
	}
	learning, err := s.metWords(ctx, userID, "learning")
	if err != nil {
		return nil, err
	}
	met = append(met, learning...)
	ids := map[int64]bool{}
	for _, word := range met {
		ids[word.LexemeID] = true
	}
	forms, err := s.store.VerbFormsAll(ctx)
	if err != nil {
		return nil, err
	}
	byLemma := map[int64][]db.VerbForm{}
	var order []int64
	for _, form := range forms {
		if _, ok := byLemma[form.LemmaID]; !ok {
			order = append(order, form.LemmaID)
		}
		byLemma[form.LemmaID] = append(byLemma[form.LemmaID], form)
	}
	var lemmaIDs []int64
	for _, id := range order {
		show := ids[id]
		for _, form := range byLemma[id] {
			if ids[form.FormLexemeID] {
				show = true
			}
		}
		if show {
			lemmaIDs = append(lemmaIDs, id)
		}
	}
	texts, err := s.store.LexemeTexts(ctx, lemmaIDs)
	if err != nil {
		return nil, err
	}
	var tables []views.VerbTable
	for _, id := range lemmaIDs {
		table := views.VerbTable{Lemma: texts[id].TextNFC}
		for _, form := range byLemma[id] {
			table.Rows = append(table.Rows, views.VerbCell{
				Label: formLabel(form), He: form.FormHe, Translit: form.Translit,
				Known: form.FormLexemeID > 0 && ids[form.FormLexemeID],
			})
		}
		tables = append(tables, table)
	}
	return tables, nil
}

func formLabel(form db.VerbForm) string {
	parts := []string{form.Tense}
	for _, p := range []string{form.Person, form.Number, form.Gender} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, " ")
}
