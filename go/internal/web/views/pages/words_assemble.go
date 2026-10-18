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

// SelectItem opens the detail for an item that the current filter hid.
func SelectItem(v *WordsView, item library.Item) {
	if v == nil {
		return
	}
	detail := detailFrom(item, v.URLWithout(QueryWord))
	v.Selected = &detail
	v.SelectedKey = item.Word
	v.Word = item.Word
}

func rowFrom(item library.Item, selected string, v WordsView) WordRow {
	stage := ""
	if item.HeEN != nil {
		stage = item.HeEN.Stage
	}
	other := ""
	if item.EnHE != nil {
		other = item.EnHE.Stage
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
	strength := textOf(item.StrengthBucket)
	row := WordRow{
		Word:          item.Word,
		Gloss:         item.GlossEN,
		Translit:      item.Translit,
		Binyan:        textOf(item.Binyan),
		Stage:         stage,
		Audio:         item.Audio,
		Suspended:     item.Suspended,
		Pressed:       selected != "" && item.Word == selected,
		SelectURL:     v.URLWith(QueryWord, item.Word),
		ChipStatus:    status,
		ChipHe:        he,
		ChipEn:        en,
		Strength:      strength,
		StrengthWidth: strengthWidth(strength),
	}
	she, sen := strengthLabel(strength)
	if she != "" {
		row.StrengthLabel = she + " · " + sen
	}
	weak := textOf(item.Weak)
	if weak == "weak" || weak == "very_weak" {
		row.Weak = weak
		row.WeakHe, row.WeakEn = weakLabel(weak)
	}
	return row
}

func detailFrom(item library.Item, closeURL string) WordDetail {
	pos := textOf(item.POS)
	root := textOf(item.Root)
	rootClass := textOf(item.RootClass)
	binyan := textOf(item.Binyan)
	posHe, posEn := posLabel(pos)
	d := WordDetail{
		Word:       item.Word,
		Gloss:      item.GlossEN,
		Translit:   item.Translit,
		Audio:      item.Audio,
		POS:        pos,
		POSHe:      posHe,
		POSEn:      posEn,
		Root:       root,
		RootClass:  rootClass,
		RootColour: rootClassColour(rootClass),
		Binyan:     binyan,
		BinyanHe:   binyanHe(binyan),
		HeEn:       directionView("he_en", "עברית ← אנגלית", "Hebrew → English", item.HeEN),
		EnHe:       directionView("en_he", "אנגלית ← עברית", "English → Hebrew", item.EnHE),
		Suspended:  item.Suspended,
		CloseURL:   closeURL,
	}
	for _, source := range item.Sources {
		d.Sources = append(d.Sources, sourceLink(source))
	}
	return d
}

func textOf(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func directionView(dir, nameHe, nameEn string, d *library.Direction) DirectionView {
	if d == nil || d.Stage == "" {
		return DirectionView{}
	}
	he, en := stageLabel(d.Stage)
	strength := textOf(d.StrengthBucket)
	she, sen := strengthLabel(strength)
	label := ""
	if she != "" {
		label = she + " · " + sen
	}
	return DirectionView{
		Dir: dir, Stage: d.Stage, Status: stageStatus(d.Stage),
		LabelHe: he, LabelEn: en, NameHe: nameHe, NameEn: nameEn,
		Strength: strength, StrengthLabel: label, StrengthWidth: strengthWidth(strength),
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
