package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"html"
	"net/http"
	"strconv"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/learner"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/roots"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/web/views"
)

// serveWord is GET /words/{id} for a signed-in learner (INIT-023/SPEC-007, SPEC-008).
func (s *server) serveWord(w http.ResponseWriter, r *http.Request, lang, csrf string) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 || s == nil || s.store == nil {
		notFound(w, r)
		return
	}
	page, user, err := s.pageFor(r, lang, csrf, "words")
	if err != nil {
		serverError(w)
		return
	}
	face, ok, err := s.store.LexemeByID(r.Context(), id)
	if err != nil {
		serverError(w)
		return
	}
	if !ok {
		notFound(w, r)
		return
	}
	lex, err := s.store.LexemesByText(r.Context(), []string{face.TextNFC})
	if err != nil {
		serverError(w)
		return
	}
	lx := lex[face.TextNFC]
	page.Title = face.TextNFC + " · " + views.AppName
	view := views.WordPageView{
		Page: page,
		Kind: wordKind(lx.POS),
		Word: views.WordHead{
			He: face.TextNFC, Translit: face.Translit, Gloss: face.Gloss,
			POS: lx.POS, Binyan: lx.Binyan, Root: lx.Root,
		},
	}
	audioID := face.AudioID
	if audioID == "" {
		if asset, ok, err := s.store.AudioByText(r.Context(), face.TextNFC); err != nil {
			serverError(w)
			return
		} else if ok {
			audioID = asset.ID
		}
	}
	if audioID != "" {
		view.Word.Audio = audioURL(audioID)
		view.Word.AudioID = "word-lemma"
	}
	if lx.Root != "" {
		vis, err := roots.Visible(r.Context(), s.store, lx.Root)
		if err != nil {
			serverError(w)
			return
		}
		if vis {
			view.Word.RootHref = page.Href("/roots/" + lx.Root)
		}
	}
	var userID int64
	if user != nil {
		userID = user.ID
		ws, err := s.model().Word(r.Context(), userID, id)
		if err == nil {
			for _, sk := range ws.Skills {
				view.Skills = append(view.Skills, views.WordSkillState{Skill: sk.Skill, State: string(sk.State)})
			}
		}
	}
	switch view.Kind {
	case "verb":
		verb, forms, err := s.verbPage(r.Context(), userID, id)
		if err != nil {
			serverError(w)
			return
		}
		view.Verb = verb
		if userID > 0 {
			lessons, err := s.wordLessonLines(r.Context(), userID, forms)
			if err != nil {
				serverError(w)
				return
			}
			view.Lessons = lessons
		}
	case "noun":
		noun, forms, irreg, err := s.nounPage(r.Context(), userID, id, face)
		if err != nil {
			serverError(w)
			return
		}
		view.Noun = noun
		view.Word.GenderChip = genderChip(face.Gender)
		view.Word.IrregularPlural = irreg
		if userID > 0 {
			lessons, err := s.wordLessonLines(r.Context(), userID, forms)
			if err != nil {
				serverError(w)
				return
			}
			view.Lessons = lessons
		}
	case "adjective":
		adj, forms, err := s.adjectivePage(r.Context(), userID, id, face.TextNFC)
		if err != nil {
			serverError(w)
			return
		}
		view.Adjective = adj
		if userID > 0 {
			lessons, err := s.wordLessonLines(r.Context(), userID, forms)
			if err != nil {
				serverError(w)
				return
			}
			view.Lessons = lessons
		}
	default:
		if userID > 0 {
			lessons, err := s.wordLessonLines(r.Context(), userID, []string{face.TextNFC})
			if err != nil {
				serverError(w)
				return
			}
			view.Lessons = lessons
		}
	}
	render(w, r, views.WordPage(view))
}

func genderChip(gender string) string {
	switch strings.ToLower(strings.TrimSpace(gender)) {
	case "m":
		return "masculine"
	case "f":
		return "feminine"
	case "both":
		return "both"
	default:
		return ""
	}
}

func wordKind(pos string) string {
	switch strings.ToLower(strings.TrimSpace(pos)) {
	case "verb":
		return "verb"
	case "noun":
		return "noun"
	case "adjective", "adj":
		return "adjective"
	default:
		return "plain"
	}
}

// lexemePlural reads lexemes.plural and irregular_plural (not on LexemeView).
func (s *server) lexemePlural(ctx context.Context, id int64) (plural string, irregular bool, err error) {
	conn, err := s.store.DB()
	if err != nil {
		return "", false, err
	}
	var p sql.NullString
	var irr int
	err = conn.QueryRowContext(ctx, `SELECT plural, COALESCE(irregular_plural, 0) FROM lexemes WHERE id = ?`, id).Scan(&p, &irr)
	if err != nil {
		return "", false, err
	}
	return p.String, irr != 0, nil
}

func (s *server) nounPage(ctx context.Context, userID, lemmaID int64, face db.LexemeRef) (*views.NounPageView, []string, bool, error) {
	rows, err := s.store.NounFormsFor(ctx, lemmaID)
	if err != nil {
		return nil, nil, false, err
	}
	pluralText, irregFlag, err := s.lexemePlural(ctx, lemmaID)
	if err != nil {
		return nil, nil, false, err
	}
	irregular := irregFlag || pluralText != ""

	known := map[int64]bool{}
	if userID > 0 {
		model := s.model()
		seen := map[int64]bool{}
		for _, row := range rows {
			if row.FormLexemeID <= 0 || seen[row.FormLexemeID] {
				continue
			}
			seen[row.FormLexemeID] = true
			ws, err := model.Word(ctx, userID, row.FormLexemeID)
			if err != nil {
				continue
			}
			known[row.FormLexemeID] = formKnown(ws)
		}
	}

	byKind := map[string][]db.NounForm{}
	var forms []string
	addForm := func(he string) {
		if he == "" {
			return
		}
		for _, f := range forms {
			if f == he {
				return
			}
		}
		forms = append(forms, he)
	}
	addForm(face.TextNFC)
	for _, row := range rows {
		addForm(row.FormHe)
		byKind[row.Kind] = append(byKind[row.Kind], row)
	}
	if pluralText != "" {
		addForm(pluralText)
	}

	out := &views.NounPageView{}
	sgCell := views.VerbFormCell{He: face.TextNFC, Translit: face.Translit}
	if face.AudioID != "" {
		sgCell.Audio = audioURL(face.AudioID)
		sgCell.AudioID = "noun-0-0"
	} else if asset, ok, err := s.store.AudioByText(ctx, face.TextNFC); err != nil {
		return nil, nil, false, err
	} else if ok {
		sgCell.Audio = audioURL(asset.ID)
		sgCell.AudioID = "noun-0-0"
	}
	if len(byKind["singular"]) > 0 {
		sgCell = nounCell(byKind["singular"][0], known, 0, 0)
	}

	plCell := views.VerbFormCell{}
	if len(byKind["plural"]) > 0 {
		plCell = nounCell(byKind["plural"][0], known, 0, 1)
	} else if pluralText != "" {
		plCell = views.VerbFormCell{He: pluralText}
		if asset, ok, err := s.store.AudioByText(ctx, pluralText); err != nil {
			return nil, nil, false, err
		} else if ok {
			plCell.Audio = audioURL(asset.ID)
			plCell.AudioID = "noun-0-1"
		}
	}

	hasWordRow := sgCell.He != "" || plCell.He != ""
	if hasWordRow {
		out.FormRows = append(out.FormRows, views.NounFormRow{Sg: sgCell, Pl: plCell})
	}

	defSg, defPl := splitNumbered(byKind["definite"])
	if defSg.FormHe != "" || defPl.FormHe != "" {
		row := views.NounFormRow{Label: "with הַ"}
		if defSg.FormHe != "" {
			row.Sg = nounCell(defSg, known, 1, 0)
		}
		if defPl.FormHe != "" {
			row.Pl = nounCell(defPl, known, 1, 1)
		}
		out.FormRows = append(out.FormRows, row)
	}

	conSg, conPl := splitNumbered(byKind["construct"])
	if conSg.FormHe != "" || conPl.FormHe != "" {
		row := views.NounFormRow{Label: "… of"}
		if conSg.FormHe != "" {
			row.Sg = nounCell(conSg, known, 2, 0)
		}
		if conPl.FormHe != "" {
			row.Pl = nounCell(conPl, known, 2, 1)
		}
		out.FormRows = append(out.FormRows, row)
	}

	poss := byKind["possessive"]
	if len(poss) > 0 {
		out.Possessive = possessiveRows(poss, known)
	}

	// Omit empty forms block (no singular/plural/definite/construct content beyond blank).
	if len(out.FormRows) == 1 && out.FormRows[0].Label == "" && out.FormRows[0].Pl.He == "" &&
		out.FormRows[0].Sg.He == face.TextNFC && len(rows) == 0 && pluralText == "" {
		out.FormRows = nil
	}
	return out, forms, irregular, nil
}

func splitNumbered(rows []db.NounForm) (sg, pl db.NounForm) {
	for _, row := range rows {
		switch row.Number {
		case "pl":
			if pl.FormHe == "" {
				pl = row
			}
		default: // "" or "sg"
			if sg.FormHe == "" {
				sg = row
			}
		}
	}
	return sg, pl
}

func nounCell(row db.NounForm, known map[int64]bool, r, c int) views.VerbFormCell {
	cell := views.VerbFormCell{
		He: row.FormHe, Translit: row.Translit,
		Known: row.FormLexemeID > 0 && known[row.FormLexemeID],
	}
	if row.AudioID != "" {
		cell.Audio = audioURL(row.AudioID)
		cell.AudioID = "noun-" + strconv.Itoa(r) + "-" + strconv.Itoa(c)
	}
	return cell
}

// possessiveOwnerOrder is the fixed possessive row order (SPEC-008).
var possessiveOwnerOrder = []struct {
	Person, Gender, OwnerNumber string
	En, He                      string
}{
	{"1", "", "sg", "my", "אֲנִי"},
	{"2", "m", "sg", "your m.", "אַתָּה"},
	{"2", "f", "sg", "your f.", "אַתְּ"},
	{"3", "m", "sg", "his", "הוּא"},
	{"3", "f", "sg", "her", "הִיא"},
	{"1", "", "pl", "our", "אֲנַחְנוּ"},
	{"2", "m", "pl", "your m. pl.", "אַתֶּם"},
	{"2", "f", "pl", "your f. pl.", "אַתֶּן"},
	{"3", "m", "pl", "their m.", "הֵם"},
	{"3", "f", "pl", "their f.", "הֵן"},
}

func possessiveRows(forms []db.NounForm, known map[int64]bool) []views.NounPossessiveRow {
	by := map[string]db.NounForm{}
	for _, f := range forms {
		by[f.Person+"|"+f.Gender+"|"+f.OwnerNumber+"|"+f.Number] = f
	}
	var out []views.NounPossessiveRow
	for i, slot := range possessiveOwnerOrder {
		sg, okSg := by[slot.Person+"|"+slot.Gender+"|"+slot.OwnerNumber+"|sg"]
		pl, okPl := by[slot.Person+"|"+slot.Gender+"|"+slot.OwnerNumber+"|pl"]
		// Also accept empty number as singular thing owned.
		if !okSg {
			if alt, ok := by[slot.Person+"|"+slot.Gender+"|"+slot.OwnerNumber+"|"]; ok {
				sg, okSg = alt, true
			}
		}
		if !okSg && !okPl {
			continue
		}
		row := views.NounPossessiveRow{En: slot.En, HePronoun: slot.He}
		if okSg {
			row.Sg = nounCell(sg, known, 10+i, 0)
		}
		if okPl {
			row.Pl = nounCell(pl, known, 10+i, 1)
		}
		out = append(out, row)
	}
	if len(out) > 0 {
		return out
	}
	// Fall back to stored order when keys did not match the fixed slots.
	grouped := map[string]*views.NounPossessiveRow{}
	var order []string
	for i, f := range forms {
		key := f.Person + "|" + f.Gender + "|" + f.OwnerNumber
		slot, ok := grouped[key]
		if !ok {
			en, he := possessiveOwnerLabel(f.Person, f.Gender, f.OwnerNumber)
			nr := &views.NounPossessiveRow{En: en, HePronoun: he}
			grouped[key] = nr
			order = append(order, key)
			slot = nr
		}
		cell := nounCell(f, known, 10+i, 0)
		if f.Number == "pl" {
			cell = nounCell(f, known, 10+i, 1)
			slot.Pl = cell
		} else {
			slot.Sg = cell
		}
	}
	for _, key := range order {
		out = append(out, *grouped[key])
	}
	return out
}

func possessiveOwnerLabel(person, gender, ownerNumber string) (en, he string) {
	for _, slot := range possessiveOwnerOrder {
		if slot.Person == person && slot.Gender == gender && slot.OwnerNumber == ownerNumber {
			return slot.En, slot.He
		}
	}
	return "possessive", ""
}

func (s *server) adjectivePage(ctx context.Context, userID, lemmaID int64, lemmaHe string) (*views.AdjectivePageView, []string, error) {
	rows, err := s.store.NounFormsFor(ctx, lemmaID)
	if err != nil {
		return nil, nil, err
	}
	known := map[int64]bool{}
	if userID > 0 {
		model := s.model()
		seen := map[int64]bool{}
		for _, row := range rows {
			if row.FormLexemeID <= 0 || seen[row.FormLexemeID] {
				continue
			}
			seen[row.FormLexemeID] = true
			ws, err := model.Word(ctx, userID, row.FormLexemeID)
			if err != nil {
				continue
			}
			known[row.FormLexemeID] = formKnown(ws)
		}
	}
	out := &views.AdjectivePageView{}
	var forms []string
	addForm := func(he string) {
		if he == "" {
			return
		}
		for _, f := range forms {
			if f == he {
				return
			}
		}
		forms = append(forms, he)
	}
	addForm(lemmaHe)
	agree := map[string]db.NounForm{}
	for _, row := range rows {
		addForm(row.FormHe)
		if row.Kind == "agreement" {
			agree[row.Number+"|"+row.Gender] = row
		}
	}
	if len(agree) > 0 {
		for i, key := range []string{"sg|m", "sg|f", "pl|m", "pl|f"} {
			row, ok := agree[key]
			if !ok {
				out.Agreement = append(out.Agreement, views.VerbFormCell{})
				continue
			}
			cell := nounCell(row, known, 0, i)
			if cell.AudioID != "" {
				cell.AudioID = "adj-0-" + strconv.Itoa(i)
			}
			out.Agreement = append(out.Agreement, cell)
		}
	}
	if userID > 0 {
		examples, err := s.adjectiveExamples(ctx, userID, agree)
		if err != nil {
			return nil, nil, err
		}
		out.Examples = examples
	}
	return out, forms, nil
}

func (s *server) adjectiveExamples(ctx context.Context, userID int64, agree map[string]db.NounForm) ([]views.AdjExampleLine, error) {
	want := []struct {
		key, gender string
	}{
		{"sg|m", "masculine"},
		{"sg|f", "feminine"},
	}
	var forms []string
	formGender := map[string]string{}
	for _, w := range want {
		if row, ok := agree[w.key]; ok {
			forms = append(forms, row.FormHe)
			formGender[row.FormHe] = w.gender
		}
	}
	if len(forms) == 0 {
		return nil, nil
	}
	lines, err := s.wordLessonLines(ctx, userID, forms)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []views.AdjExampleLine
	for _, line := range lines {
		plain := unmarkFormHTML(line.HeHTML)
		match := matchedForm(plain, forms)
		g := formGender[match]
		if g == "" || seen[g] {
			continue
		}
		// Prefer a line where a token precedes the adjective (after a noun).
		if !adjectiveAfterToken(plain, match) {
			continue
		}
		seen[g] = true
		out = append(out, views.AdjExampleLine{HeHTML: line.HeHTML, En: line.En, Gender: g})
		if len(seen) == 2 {
			break
		}
	}
	return out, nil
}

func unmarkFormHTML(heHTML string) string {
	s := heHTML
	s = strings.ReplaceAll(s, `<mark><bdi lang="he" dir="rtl">`, "")
	s = strings.ReplaceAll(s, `</bdi></mark>`, "")
	s = html.UnescapeString(s)
	return s
}

func adjectiveAfterToken(text, adj string) bool {
	i := strings.Index(text, adj)
	if i <= 0 {
		return false
	}
	before := strings.TrimSpace(text[:i])
	return before != ""
}

func (s *server) verbPage(ctx context.Context, userID, lemmaID int64) (*views.VerbPageView, []string, error) {
	rows, err := s.store.VerbFormsFor(ctx, lemmaID)
	if err != nil {
		return nil, nil, err
	}
	known := map[int64]bool{}
	if userID > 0 {
		model := s.model()
		seen := map[int64]bool{}
		for _, row := range rows {
			if row.FormLexemeID <= 0 || seen[row.FormLexemeID] {
				continue
			}
			seen[row.FormLexemeID] = true
			ws, err := model.Word(ctx, userID, row.FormLexemeID)
			if err != nil {
				continue
			}
			known[row.FormLexemeID] = formKnown(ws)
		}
	}
	out := &views.VerbPageView{}
	var forms []string
	addForm := func(he string) {
		if he == "" {
			return
		}
		for _, f := range forms {
			if f == he {
				return
			}
		}
		forms = append(forms, he)
	}
	present := map[string]db.VerbForm{}
	var past, future []db.VerbForm
	for _, row := range rows {
		addForm(row.FormHe)
		switch row.Tense {
		case "present":
			present[row.Number+"|"+row.Gender] = row
		case "past":
			past = append(past, row)
		case "future":
			future = append(future, row)
		case "imperative":
			out.Imperative = append(out.Imperative, verbCell(row, known, "imperative", len(out.Imperative), 0))
		case "infinitive":
			out.Infinitive = append(out.Infinitive, verbCell(row, known, "infinitive", len(out.Infinitive), 0))
		}
	}
	if len(present) > 0 {
		for i, key := range []string{"sg|m", "sg|f", "pl|m", "pl|f"} {
			row, ok := present[key]
			if !ok {
				out.Present = append(out.Present, views.VerbFormCell{})
				continue
			}
			out.Present = append(out.Present, verbCell(row, known, "present", 0, i))
		}
	}
	out.Past = personRows(past, known, "past")
	out.Future = personRows(future, known, "future")
	return out, forms, nil
}

func formKnown(ws learner.WordState) bool {
	for _, sk := range ws.Skills {
		if sk.State == learner.Known || sk.State == learner.Solid {
			return true
		}
	}
	return ws.State == learner.Known || ws.State == learner.Solid
}

func verbCell(row db.VerbForm, known map[int64]bool, tense string, r, c int) views.VerbFormCell {
	cell := views.VerbFormCell{
		He: row.FormHe, Translit: row.Translit,
		Known: row.FormLexemeID > 0 && known[row.FormLexemeID],
	}
	if row.AudioID != "" {
		cell.Audio = audioURL(row.AudioID)
		cell.AudioID = "verb-" + tense + "-" + strconv.Itoa(r) + "-" + strconv.Itoa(c)
	}
	return cell
}

// personOrder is the fixed past/future row order.
var personOrder = []struct {
	Person, Number, Gender string
	En, He                 string
}{
	{"1", "sg", "", "I", "אֲנִי"},
	{"2", "sg", "m", "you m.", "אַתָּה"},
	{"2", "sg", "f", "you f.", "אַתְּ"},
	{"3", "sg", "m", "he", "הוּא"},
	{"3", "sg", "f", "she", "הִיא"},
	{"1", "pl", "", "we", "אֲנַחְנוּ"},
	{"2", "pl", "m", "you m. pl.", "אַתֶּם"},
	{"2", "pl", "f", "you f. pl.", "אַתֶּן"},
	{"3", "pl", "m", "they m.", "הֵם"},
	{"3", "pl", "f", "they f.", "הֵן"},
}

func personRows(forms []db.VerbForm, known map[int64]bool, tense string) []views.VerbPersonRow {
	if len(forms) == 0 {
		return nil
	}
	by := map[string]db.VerbForm{}
	for _, f := range forms {
		by[f.Person+"|"+f.Number+"|"+f.Gender] = f
	}
	var out []views.VerbPersonRow
	for i, slot := range personOrder {
		row, ok := by[slot.Person+"|"+slot.Number+"|"+slot.Gender]
		if !ok {
			// Also accept empty gender for 1st person when stored that way.
			if slot.Gender == "" {
				continue
			}
			continue
		}
		out = append(out, views.VerbPersonRow{
			En: slot.En, HePronoun: slot.He,
			Cell: verbCell(row, known, tense, i, 0),
		})
	}
	// If ordering matched nothing (odd person tags), fall back to stored order.
	if len(out) == 0 {
		for i, f := range forms {
			out = append(out, views.VerbPersonRow{
				En: formLabel(f), Cell: verbCell(f, known, tense, i, 0),
			})
		}
	}
	return out
}

const wordLessonCap = 8

func (s *server) wordLessonLines(ctx context.Context, userID int64, forms []string) ([]views.WordLessonLine, error) {
	if len(forms) == 0 {
		return nil, nil
	}
	rows, err := s.store.UnitsInScope(ctx, userID)
	if err != nil {
		return nil, err
	}
	hidden, err := s.store.HiddenUnitIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	gone := map[int64]bool{}
	for _, id := range hidden {
		gone[id] = true
	}
	var out []views.WordLessonLine
	for _, row := range rows {
		if row.Status != "published" || gone[row.ID] {
			continue
		}
		var doc units.Unit
		if err := json.Unmarshal([]byte(row.Doc), &doc); err != nil {
			continue
		}
		for li, les := range doc.Lessons {
			for _, act := range les.Activities {
				for _, line := range activityLines(act) {
					match := matchedForm(line.he, forms)
					if match == "" {
						continue
					}
					out = append(out, views.WordLessonLine{
						HeHTML: markForm(line.he, match),
						En:     line.en,
						Href:   lessonPath(row.ID, li),
						Title:  row.Title,
					})
					if len(out) >= wordLessonCap {
						return out, nil
					}
				}
			}
		}
	}
	return out, nil
}

type actLine struct{ he, en string }

func activityLines(act units.Activity) []actLine {
	if act.Type == units.ActivityDialogue {
		out := make([]actLine, 0, len(act.Lines))
		for _, l := range act.Lines {
			if strings.TrimSpace(l.Text) == "" {
				continue
			}
			out = append(out, actLine{he: l.Text, en: l.Translation})
		}
		return out
	}
	if strings.TrimSpace(act.Text) == "" {
		return nil
	}
	return []actLine{{he: act.Text, en: act.Translation}}
}

func matchedForm(text string, forms []string) string {
	// Prefer the longest match so a longer conjugation wins over a stem.
	best := ""
	for _, f := range forms {
		if f == "" {
			continue
		}
		if strings.Contains(text, f) && len([]rune(f)) >= len([]rune(best)) {
			best = f
		}
	}
	return best
}

func markForm(text, form string) string {
	if form == "" || !strings.Contains(text, form) {
		return html.EscapeString(text)
	}
	i := strings.Index(text, form)
	return html.EscapeString(text[:i]) +
		`<mark><bdi lang="he" dir="rtl">` + html.EscapeString(form) + `</bdi></mark>` +
		html.EscapeString(text[i+len(form):])
}
