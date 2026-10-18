package pages

import (
	"net/url"
	"regexp"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/web/library"
)

var lessonSlug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Assemble turns one List page into the screen. state is ready, empty, or error.
// The word key is copied through unchanged.
func Assemble(f Filters, word, cursor, state, errKind string, items []library.Item, next string) WordsView {
	v := WordsView{
		State:       state,
		ErrorKind:   errKind,
		Filters:     f,
		Word:        word,
		Cursor:      cursor,
		SelectedKey: word,
		NextCursor:  next,
		Shown:       len(items),
	}
	href, deck := reviewLink(f)
	v.ReviewHref = href
	v.ReviewDeck = deck
	for _, item := range items {
		row := rowFrom(item, word, v)
		v.Rows = append(v.Rows, row)
		if word != "" && item.Word == word {
			detail := detailFrom(item, v.URLWithout(QueryWord))
			v.Selected = &detail
		}
	}
	v.Active = activeFilters(v)
	return v
}

func rowFrom(item library.Item, selected string, v WordsView) WordRow {
	stage := ""
	if item.HeEn != nil {
		stage = item.HeEn.Stage
	}
	other := ""
	if item.EnHe != nil {
		other = item.EnHe.Stage
	}
	status, he, en := "new", "חדשה", "New"
	if item.Suspended {
		status, he, en = "locked", "מושהית", "Paused"
	} else {
		low := lowerStage(stage, other)
		if low == "" {
			low = "new"
		}
		status = stageStatus(low)
		he, en = stageLabel(low)
	}
	row := WordRow{
		Word:          item.Word,
		Gloss:         item.GlossEN,
		Translit:      item.Translit,
		Binyan:        item.Binyan,
		Stage:         stage,
		Audio:         item.Audio,
		Suspended:     item.Suspended,
		Pressed:       selected != "" && item.Word == selected,
		SelectURL:     v.URLWith(QueryWord, item.Word),
		ChipStatus:    status,
		ChipHe:        he,
		ChipEn:        en,
		Strength:      item.StrengthBucket,
		StrengthWidth: strengthWidth(item.StrengthBucket),
	}
	she, sen := strengthLabel(item.StrengthBucket)
	if she != "" {
		row.StrengthLabel = she + " · " + sen
	}
	if item.Weak == "weak" || item.Weak == "very_weak" {
		row.Weak = item.Weak
		row.WeakHe, row.WeakEn = weakLabel(item.Weak)
	}
	return row
}

func detailFrom(item library.Item, closeURL string) WordDetail {
	posHe, posEn := posLabel(item.POS)
	d := WordDetail{
		Word:       item.Word,
		Gloss:      item.GlossEN,
		Translit:   item.Translit,
		Audio:      item.Audio,
		POS:        item.POS,
		POSHe:      posHe,
		POSEn:      posEn,
		Root:       item.Root,
		RootClass:  item.RootClass,
		RootColour: rootClassColour(item.RootClass),
		Binyan:     item.Binyan,
		BinyanHe:   binyanHe(item.Binyan),
		HeEn:       directionView("he_en", "עברית ← אנגלית", "Hebrew → English", item.HeEn),
		EnHe:       directionView("en_he", "אנגלית ← עברית", "English → Hebrew", item.EnHe),
		CloseURL:   closeURL,
	}
	for _, form := range item.Paradigm {
		d.Paradigm = append(d.Paradigm, ParadigmCell{
			Tense: form.Tense, Cell: form.Cell, He: form.He, Gloss: form.Gloss, Checked: form.Checked,
		})
	}
	for _, source := range item.Sources {
		d.Sources = append(d.Sources, sourceLink(source))
	}
	return d
}

func directionView(dir, nameHe, nameEn string, d *library.Direction) DirectionView {
	if d == nil || d.Stage == "" {
		return DirectionView{}
	}
	he, en := stageLabel(d.Stage)
	she, sen := strengthLabel(d.StrengthBucket)
	label := ""
	if she != "" {
		label = she + " · " + sen
	}
	return DirectionView{
		Dir: dir, Stage: d.Stage, Status: stageStatus(d.Stage),
		LabelHe: he, LabelEn: en, NameHe: nameHe, NameEn: nameEn,
		Strength: d.StrengthBucket, StrengthLabel: label, StrengthWidth: strengthWidth(d.StrengthBucket),
		Show: true,
	}
}

func sourceLink(source string) SourceLink {
	switch source {
	case "self":
		return SourceLink{TitleHe: "הוספתי", TitleEn: "Added by me"}
	case "ai":
		return SourceLink{TitleHe: "בינה", TitleEn: "AI"}
	default:
		if lessonSlug.MatchString(source) {
			return SourceLink{
				TitleHe: source, TitleEn: source, Lesson: true, LessonSlug: source,
				Href: "/review/?deck=" + url.QueryEscape("lesson:"+source),
			}
		}
		return SourceLink{TitleHe: source, TitleEn: source}
	}
}

func activeFilters(v WordsView) []ActiveFilter {
	type spec struct {
		key, he, en string
	}
	var specs []spec
	f := v.Filters
	if f.Q != "" {
		specs = append(specs, spec{"q", f.Q, f.Q})
	}
	if f.Status != "" {
		he, en := stageLabel(f.Status)
		specs = append(specs, spec{"status", he, en})
	}
	if f.Weak != "" {
		he, en := weakLabel(f.Weak)
		if f.Weak == "1" {
			he, en = "חלשות", "Weak"
		}
		specs = append(specs, spec{"weak", he, en})
	}
	if f.POS != "" {
		he, en := posLabel(f.POS)
		specs = append(specs, spec{"pos", he, en})
	}
	if f.Binyan != "" {
		specs = append(specs, spec{"binyan", binyanHe(f.Binyan), f.Binyan})
	}
	if f.Tense != "" {
		he, en := tenseLabel(f.Tense)
		specs = append(specs, spec{"tense", he, en})
	}
	if f.Source != "" {
		he, en := sourceLabel(f.Source)
		specs = append(specs, spec{"source", he, en})
	}
	if f.Suspended == "1" {
		specs = append(specs, spec{"suspended", "מושהות בלבד", "Paused only"})
	} else if f.Suspended == "0" {
		specs = append(specs, spec{"suspended", "פעילות", "Active only"})
	}
	out := make([]ActiveFilter, 0, len(specs))
	for _, s := range specs {
		out = append(out, ActiveFilter{
			Key: s.key, LabelHe: s.he, LabelEn: s.en, HrefWithout: v.URLWithout(s.key),
		})
	}
	return out
}

func reviewLink(f Filters) (href, deck string) {
	switch {
	case f.Binyan != "" && known(binyanOptions, f.Binyan):
		deck = "concept:binyan:" + f.Binyan
	case f.Tense != "" && known(tenseOptions, f.Tense):
		deck = "concept:tense:" + f.Tense
	case f.Weak == "1" || f.Weak == "weak" || f.Weak == "very_weak":
		deck = "weak"
	default:
		return "/review/", "due"
	}
	return "/review/?deck=" + url.QueryEscape(deck), deck
}
