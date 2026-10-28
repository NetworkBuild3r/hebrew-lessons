package views

import (
	"context"
	"html"
	"io"
	"strconv"

	"github.com/a-h/templ"
)

// WordPageView is GET /words/{id} (INIT-023/SPEC-007, SPEC-008).
type WordPageView struct {
	Page Page
	// Kind is verb, noun, adjective, or plain.
	Kind string
	Word WordHead
	// Skills is the learner's state per skill, under the header.
	Skills []WordSkillState
	// Verb is set when Kind is verb.
	Verb *VerbPageView
	// Noun is set when Kind is noun.
	Noun *NounPageView
	// Adjective is set when Kind is adjective.
	Adjective *AdjectivePageView
	// Lessons is "In your lessons", at most 8.
	Lessons []WordLessonLine
}

// WordHead is the lemma at the top of the page.
type WordHead struct {
	He       string
	Translit string
	Gloss    string
	Audio    string
	AudioID  string
	POS      string
	Binyan   string
	Root     string
	RootHref string
	// GenderChip is "masculine", "feminine", or "both" for a noun (SPEC-008).
	GenderChip string
	// IrregularPlural adds the "irregular plural" chip when the lexeme marks one.
	IrregularPlural bool
}

// NounPageView holds the noun forms and possessive tables (INIT-023/SPEC-008).
type NounPageView struct {
	// FormRows is the word / with הַ / construct grid. Empty rows are omitted.
	FormRows []NounFormRow
	// Possessive is omitted when empty.
	Possessive []NounPossessiveRow
}

// NounFormRow is one Singular|Plural line in the forms block.
type NounFormRow struct {
	Label string // "", "with הַ", or "… of"
	Sg    VerbFormCell
	Pl    VerbFormCell
}

// NounPossessiveRow is one owner line with singular and plural thing-owned cells.
type NounPossessiveRow struct {
	En        string
	HePronoun string
	Sg        VerbFormCell
	Pl        VerbFormCell
}

// AdjectivePageView is the four agreement cells plus optional after-noun lines (SPEC-008).
type AdjectivePageView struct {
	Agreement []VerbFormCell // m.sg, f.sg, m.pl, f.pl
	Examples  []AdjExampleLine
}

// AdjExampleLine is the adjective after a noun of one gender, from a unit line.
type AdjExampleLine struct {
	HeHTML string
	En     string
	Gender string // "masculine" or "feminine"
}

// WordSkillState is one skill chip under the header.
type WordSkillState struct {
	Skill string
	State string
}

// VerbPageView holds the conjugation tables.
type VerbPageView struct {
	Present    []VerbFormCell // m.sg, f.sg, m.pl, f.pl when any present form exists
	Past       []VerbPersonRow
	Future     []VerbPersonRow
	Imperative []VerbFormCell
	Infinitive []VerbFormCell
}

// VerbFormCell is one playable form cell.
type VerbFormCell struct {
	He       string
	Translit string
	Audio    string
	AudioID  string
	Known    bool
	Label    string
}

// VerbPersonRow is one past/future person line.
type VerbPersonRow struct {
	En        string
	HePronoun string
	Cell      VerbFormCell
}

// WordLessonLine is one activity line that uses a form of the word.
type WordLessonLine struct {
	HeHTML string // Hebrew with the matched form already marked (safe HTML)
	En     string
	Href   string
	Title  string
}

// WordPage is the /words/{id} page. Written by hand so templ generate is not required.
func WordPage(v WordPageView) templ.Component {
	return wordPageComp{v: v}
}

type wordPageComp struct{ v WordPageView }

func (c wordPageComp) Render(ctx context.Context, w io.Writer) error {
	return Layout(c.v.Page).Render(templ.WithChildren(ctx, wordPageBody{v: c.v}), w)
}

type wordPageBody struct{ v WordPageView }

func (b wordPageBody) Render(ctx context.Context, w io.Writer) error {
	v := b.v
	write := func(s string) error {
		_, err := io.WriteString(w, s)
		return err
	}
	if err := write(`<link rel="stylesheet" href="` + html.EscapeString(string(Asset("/static/css/word.css"))) + `"/>`); err != nil {
		return err
	}
	if err := write(`<article class="word-page">`); err != nil {
		return err
	}
	if err := renderWordHeader(ctx, w, v); err != nil {
		return err
	}
	if len(v.Skills) > 0 {
		if err := write(`<ul class="word-page__skills" aria-label="Your state">`); err != nil {
			return err
		}
		for _, sk := range v.Skills {
			if err := write(`<li><span class="word-page__skill-name" lang="en">` + html.EscapeString(sk.Skill) +
				`</span> <span class="word-page__skill-state" lang="en">` + html.EscapeString(sk.State) + `</span></li>`); err != nil {
				return err
			}
		}
		if err := write(`</ul>`); err != nil {
			return err
		}
	}
	if v.Kind == "verb" && v.Verb != nil {
		if err := renderVerbTables(ctx, w, v.Verb); err != nil {
			return err
		}
	}
	if v.Kind == "noun" && v.Noun != nil {
		if err := renderNounTables(ctx, w, v.Noun); err != nil {
			return err
		}
	}
	if v.Kind == "adjective" && v.Adjective != nil {
		if err := renderAdjectiveTables(ctx, w, v.Adjective); err != nil {
			return err
		}
	}
	if len(v.Lessons) > 0 {
		if err := write(`<section class="word-page__lessons" aria-labelledby="word-lessons-h"><h2 id="word-lessons-h" lang="en">In your lessons</h2><ul>`); err != nil {
			return err
		}
		for _, line := range v.Lessons {
			if err := write(`<li><p class="word-page__lesson-he" lang="he" dir="rtl">` + line.HeHTML + `</p>`); err != nil {
				return err
			}
			if line.En != "" {
				if err := write(`<p class="word-page__lesson-en" lang="en">` + html.EscapeString(line.En) + `</p>`); err != nil {
					return err
				}
			}
			if err := write(`<a href="` + html.EscapeString(line.Href) + `" lang="en">` + html.EscapeString(line.Title) + `</a></li>`); err != nil {
				return err
			}
		}
		if err := write(`</ul></section>`); err != nil {
			return err
		}
	}
	if err := write(`</article><script src="` + html.EscapeString(string(Asset("/static/js/clips.js"))) + `" defer></script>`); err != nil {
		return err
	}
	return nil
}

func renderWordHeader(ctx context.Context, w io.Writer, v WordPageView) error {
	h := v.Word
	write := func(s string) error {
		_, err := io.WriteString(w, s)
		return err
	}
	if err := write(`<header class="word-page__head"><div class="word-page__lemma"><h1 class="word-page__he" lang="he" dir="rtl"><bdi>` +
		html.EscapeString(h.He) + `</bdi></h1>`); err != nil {
		return err
	}
	if h.Audio != "" {
		if err := clip(h.AudioID, h.Audio, "Play "+h.He, false).Render(ctx, w); err != nil {
			return err
		}
	}
	if err := write(`</div>`); err != nil {
		return err
	}
	if h.Translit != "" {
		if err := write(`<p class="word-page__tr" lang="he-Latn" dir="ltr">` + html.EscapeString(h.Translit) + `</p>`); err != nil {
			return err
		}
	}
	if h.Gloss != "" {
		if err := write(`<p class="word-page__gl" lang="en" dir="ltr">` + html.EscapeString(h.Gloss) + `</p>`); err != nil {
			return err
		}
	}
	if err := write(`<div class="word-page__chips">`); err != nil {
		return err
	}
	if h.POS != "" {
		if err := write(`<span class="c-chip" lang="en">` + html.EscapeString(h.POS) + `</span>`); err != nil {
			return err
		}
	}
	if h.Binyan != "" {
		if err := write(`<span class="c-chip" lang="en">` + html.EscapeString(h.Binyan) + `</span>`); err != nil {
			return err
		}
	}
	if h.Root != "" {
		if h.RootHref != "" {
			if err := write(`<a class="c-chip word-page__root" href="` + html.EscapeString(h.RootHref) +
				`"><bdi lang="he" dir="rtl">` + html.EscapeString(h.Root) + `</bdi></a>`); err != nil {
				return err
			}
		} else {
			if err := write(`<span class="c-chip"><bdi lang="he" dir="rtl">` + html.EscapeString(h.Root) + `</bdi></span>`); err != nil {
				return err
			}
		}
	}
	if h.GenderChip != "" {
		if err := write(`<span class="c-chip" lang="en">` + html.EscapeString(h.GenderChip) + `</span>`); err != nil {
			return err
		}
	}
	if h.IrregularPlural {
		if err := write(`<span class="c-chip" lang="en">irregular plural</span>`); err != nil {
			return err
		}
	}
	return write(`</div></header>`)
}

func renderNounTables(ctx context.Context, w io.Writer, noun *NounPageView) error {
	write := func(s string) error {
		_, err := io.WriteString(w, s)
		return err
	}
	if len(noun.FormRows) > 0 {
		if err := write(`<section class="word-page__tense" aria-labelledby="noun-forms-h"><h2 id="noun-forms-h" lang="en">Forms</h2><div class="word-page__scroll"><table class="word-page__noun-forms"><thead><tr><th scope="col" lang="en"></th><th scope="col" lang="en">Singular</th><th scope="col" lang="en">Plural</th></tr></thead><tbody>`); err != nil {
			return err
		}
		for i, row := range noun.FormRows {
			if err := write(`<tr>`); err != nil {
				return err
			}
			if row.Label != "" {
				if err := write(`<th scope="row" lang="en">` + html.EscapeString(row.Label) + `</th>`); err != nil {
					return err
				}
			} else {
				if err := write(`<th scope="row" lang="en"></th>`); err != nil {
					return err
				}
			}
			if err := write(`<td>`); err != nil {
				return err
			}
			if err := renderFormCell(ctx, w, row.Sg, "noun", i, 0); err != nil {
				return err
			}
			if err := write(`</td><td>`); err != nil {
				return err
			}
			if err := renderFormCell(ctx, w, row.Pl, "noun", i, 1); err != nil {
				return err
			}
			if err := write(`</td></tr>`); err != nil {
				return err
			}
		}
		if err := write(`</tbody></table></div></section>`); err != nil {
			return err
		}
	}
	if len(noun.Possessive) == 0 {
		return nil
	}
	if err := write(`<section class="word-page__tense" aria-labelledby="noun-poss-h"><h2 id="noun-poss-h" lang="en">Possessive</h2><div class="word-page__scroll"><table class="word-page__noun-poss"><thead><tr><th scope="col" lang="en"></th><th scope="col" lang="en">Singular</th><th scope="col" lang="en">Plural</th></tr></thead><tbody>`); err != nil {
		return err
	}
	for i, row := range noun.Possessive {
		if err := write(`<tr><th scope="row" lang="en">` + html.EscapeString(row.En)); err != nil {
			return err
		}
		if row.HePronoun != "" {
			if err := write(` <bdi lang="he" dir="rtl">` + html.EscapeString(row.HePronoun) + `</bdi>`); err != nil {
				return err
			}
		}
		if err := write(`</th><td>`); err != nil {
			return err
		}
		if err := renderFormCell(ctx, w, row.Sg, "noun-poss", i, 0); err != nil {
			return err
		}
		if err := write(`</td><td>`); err != nil {
			return err
		}
		if err := renderFormCell(ctx, w, row.Pl, "noun-poss", i, 1); err != nil {
			return err
		}
		if err := write(`</td></tr>`); err != nil {
			return err
		}
	}
	return write(`</tbody></table></div></section>`)
}

func renderAdjectiveTables(ctx context.Context, w io.Writer, adj *AdjectivePageView) error {
	write := func(s string) error {
		_, err := io.WriteString(w, s)
		return err
	}
	if len(adj.Agreement) > 0 {
		if err := write(`<section class="word-page__tense" aria-labelledby="adj-agree-h"><h2 id="adj-agree-h" lang="en">Agreement</h2><div class="word-page__scroll"><table class="word-page__present"><thead><tr>`); err != nil {
			return err
		}
		for _, label := range []string{"Singular M", "Singular F", "Plural M", "Plural F"} {
			if err := write(`<th scope="col" lang="en">` + label + `</th>`); err != nil {
				return err
			}
		}
		if err := write(`</tr></thead><tbody><tr>`); err != nil {
			return err
		}
		for i, cell := range adj.Agreement {
			if err := write(`<td>`); err != nil {
				return err
			}
			if err := renderFormCell(ctx, w, cell, "adj", 0, i); err != nil {
				return err
			}
			if err := write(`</td>`); err != nil {
				return err
			}
		}
		if err := write(`</tr></tbody></table></div></section>`); err != nil {
			return err
		}
	}
	if len(adj.Examples) == 0 {
		return nil
	}
	if err := write(`<section class="word-page__adj-examples" aria-labelledby="adj-ex-h"><h2 id="adj-ex-h" lang="en">After a noun</h2><ul>`); err != nil {
		return err
	}
	for _, ex := range adj.Examples {
		if err := write(`<li>`); err != nil {
			return err
		}
		if ex.Gender != "" {
			if err := write(`<span class="word-page__adj-gender" lang="en">` + html.EscapeString(ex.Gender) + `</span> `); err != nil {
				return err
			}
		}
		if err := write(`<p class="word-page__lesson-he" lang="he" dir="rtl">` + ex.HeHTML + `</p>`); err != nil {
			return err
		}
		if ex.En != "" {
			if err := write(`<p class="word-page__lesson-en" lang="en">` + html.EscapeString(ex.En) + `</p>`); err != nil {
				return err
			}
		}
		if err := write(`</li>`); err != nil {
			return err
		}
	}
	return write(`</ul></section>`)
}

func renderVerbTables(ctx context.Context, w io.Writer, verb *VerbPageView) error {
	write := func(s string) error {
		_, err := io.WriteString(w, s)
		return err
	}
	if len(verb.Present) > 0 {
		if err := write(`<section class="word-page__tense" aria-labelledby="tense-present"><h2 id="tense-present" lang="en">Present tense · <bdi lang="he" dir="rtl">הוֹוֶה</bdi></h2><div class="word-page__scroll"><table class="word-page__present"><thead><tr>`); err != nil {
			return err
		}
		for _, label := range []string{"Singular M", "Singular F", "Plural M", "Plural F"} {
			if err := write(`<th scope="col" lang="en">` + label + `</th>`); err != nil {
				return err
			}
		}
		if err := write(`</tr></thead><tbody><tr>`); err != nil {
			return err
		}
		for i, cell := range verb.Present {
			if err := write(`<td>`); err != nil {
				return err
			}
			if err := renderFormCell(ctx, w, cell, "present", 0, i); err != nil {
				return err
			}
			if err := write(`</td>`); err != nil {
				return err
			}
		}
		if err := write(`</tr></tbody></table></div></section>`); err != nil {
			return err
		}
	}
	if err := renderPersonTense(ctx, w, "past", "Past tense · ", "עָבָר", verb.Past); err != nil {
		return err
	}
	if err := renderPersonTense(ctx, w, "future", "Future tense · ", "עָתִיד", verb.Future); err != nil {
		return err
	}
	if len(verb.Imperative) > 0 {
		if err := write(`<section class="word-page__tense" aria-labelledby="tense-imperative"><h2 id="tense-imperative" lang="en">Imperative</h2><div class="word-page__scroll"><ul class="word-page__forms">`); err != nil {
			return err
		}
		for i, cell := range verb.Imperative {
			if err := write(`<li>`); err != nil {
				return err
			}
			if err := renderFormCell(ctx, w, cell, "imperative", i, 0); err != nil {
				return err
			}
			if err := write(`</li>`); err != nil {
				return err
			}
		}
		if err := write(`</ul></div></section>`); err != nil {
			return err
		}
	}
	if len(verb.Infinitive) > 0 {
		if err := write(`<section class="word-page__tense" aria-labelledby="tense-infinitive"><h2 id="tense-infinitive" lang="en">Infinitive</h2><div class="word-page__scroll"><ul class="word-page__forms">`); err != nil {
			return err
		}
		for i, cell := range verb.Infinitive {
			if err := write(`<li>`); err != nil {
				return err
			}
			if err := renderFormCell(ctx, w, cell, "infinitive", i, 0); err != nil {
				return err
			}
			if err := write(`</li>`); err != nil {
				return err
			}
		}
		if err := write(`</ul></div></section>`); err != nil {
			return err
		}
	}
	return nil
}

func renderPersonTense(ctx context.Context, w io.Writer, id, enPrefix, heLabel string, rows []VerbPersonRow) error {
	if len(rows) == 0 {
		return nil
	}
	write := func(s string) error {
		_, err := io.WriteString(w, s)
		return err
	}
	if err := write(`<section class="word-page__tense" aria-labelledby="tense-` + id + `"><h2 id="tense-` + id + `" lang="en">` +
		enPrefix + `<bdi lang="he" dir="rtl">` + heLabel + `</bdi></h2><div class="word-page__scroll"><table class="word-page__persons"><tbody>`); err != nil {
		return err
	}
	for i, row := range rows {
		if err := write(`<tr><th scope="row" lang="en">` + html.EscapeString(row.En)); err != nil {
			return err
		}
		if row.HePronoun != "" {
			if err := write(` <bdi lang="he" dir="rtl">` + html.EscapeString(row.HePronoun) + `</bdi>`); err != nil {
				return err
			}
		}
		if err := write(`</th><td>`); err != nil {
			return err
		}
		if err := renderFormCell(ctx, w, row.Cell, id, i, 0); err != nil {
			return err
		}
		if err := write(`</td></tr>`); err != nil {
			return err
		}
	}
	return write(`</tbody></table></div></section>`)
}

func renderFormCell(ctx context.Context, w io.Writer, cell VerbFormCell, tense string, row, col int) error {
	write := func(s string) error {
		_, err := io.WriteString(w, s)
		return err
	}
	known := "false"
	if cell.Known {
		known = "true"
	}
	attrs := ` class="word-page__cell" data-known="` + known + `"`
	if cell.Known {
		attrs += ` aria-label="you know this form"`
	}
	if err := write(`<div` + attrs + `>`); err != nil {
		return err
	}
	if cell.He != "" {
		if err := write(`<bdi class="word-page__form-he" lang="he" dir="rtl">` + html.EscapeString(cell.He) + `</bdi>`); err != nil {
			return err
		}
	}
	if cell.Translit != "" {
		if err := write(`<span class="word-page__form-tr" lang="he-Latn" dir="ltr">` + html.EscapeString(cell.Translit) + `</span>`); err != nil {
			return err
		}
	}
	if cell.Known {
		if err := write(`<span class="word-page__known" lang="en">you know this form</span>`); err != nil {
			return err
		}
	}
	if cell.Audio != "" {
		id := "verb-" + tense + "-" + strconv.Itoa(row) + "-" + strconv.Itoa(col)
		if cell.AudioID != "" {
			id = cell.AudioID
		}
		if err := clip(id, cell.Audio, "Play "+cell.He, false).Render(ctx, w); err != nil {
			return err
		}
	}
	return write(`</div>`)
}
