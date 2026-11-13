// The seven binyanim, and the forms a verb really takes.
//
// Everything a page says about a particular verb is read from
// internal/speech/course.tsv: its root, its pattern, its meaning, and the forms
// the dictionary lists for it. Nothing here guesses a form. A rule is stated as
// a rule (a suffix table), and a verb shows the forms the signed list holds for
// it. INIT-040.
package speech

import (
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Binyan is one of the seven Hebrew verb patterns.
type Binyan struct {
	// Key is the slug used in a URL and in the course list, such as "paal".
	Key string
	// Name is the Hebrew name, pointed.
	Name string
	// Translit is that name in Latin letters.
	Translit string
	// English is the plain description of what the pattern does.
	English string
	// Shape is how the pattern changes the three root letters.
	Shape string
	// Example is one ready course verb in this pattern.
	Example string
	// ExampleGloss is that verb's meaning.
	ExampleGloss string
	// ExampleRoot is that verb's root, pointed with maqaf.
	ExampleRoot string
	// Count is how many ready course verbs use it.
	Count int
}

// Binyanim is the seven patterns in the order Hebrew teaching takes them.
var Binyanim = []Binyan{
	{
		Key: "paal", Name: "פָּעַל", Translit: "pa'al",
		English: "Simple action",
		Shape:   "The plain root, with no extra letter. This is the pattern to learn first.",
	},
	{
		Key: "nifal", Name: "נִפְעַל", Translit: "nif'al",
		English: "Passive, or doing it to yourself",
		Shape:   "A נ goes on the front of the root.",
	},
	{
		Key: "piel", Name: "פִּעֵל", Translit: "pi'el",
		English: "Stronger, or making someone else do it",
		Shape:   "The middle root letter doubles. לָמַד is he learned; לִמֵּד is he taught.",
	},
	{
		Key: "pual", Name: "פֻּעַל", Translit: "pu'al",
		English: "Passive of pi'el",
		Shape:   "The same doubled middle letter, with a u sound.",
	},
	{
		Key: "hitpael", Name: "הִתְפַּעֵל", Translit: "hitpa'el",
		English: "Doing it to yourself",
		Shape:   "הִת goes on the front of the root.",
	},
	{
		Key: "hifil", Name: "הִפְעִיל", Translit: "hif'il",
		English: "Making it happen",
		Shape:   "הִ goes on the front, and the vowels follow the hif'il pattern.",
	},
	{
		Key: "hufal", Name: "הֻפְעַל", Translit: "huf'al",
		English: "Passive of hif'il",
		Shape:   "The hif'il shape with a u sound.",
	},
}

var binyanBy = func() map[string]Binyan {
	m := make(map[string]Binyan, len(Binyanim))
	for _, b := range Binyanim {
		m[b.Key] = b
	}
	return m
}()

// LookupBinyan returns one pattern by its slug.
func LookupBinyan(key string) (Binyan, bool) {
	b, ok := binyanBy[key]
	return b, ok
}

// BinyanList is Binyanim with count and one real example filled in from the
// course. A pattern with no ready verb still appears, with a zero count, so the
// page explains all seven rather than hiding a gap.
func BinyanList() ([]Binyan, error) {
	rows, err := CourseReady()
	if err != nil {
		return nil, err
	}
	out := make([]Binyan, len(Binyanim))
	copy(out, Binyanim)
	index := make(map[string]int, len(out))
	for i, b := range out {
		index[b.Key] = i
	}
	for _, w := range rows {
		if w.POS != "verb" || w.Binyan == "" {
			continue
		}
		i, ok := index[w.Binyan]
		if !ok {
			continue
		}
		out[i].Count++
		if out[i].Example == "" {
			out[i].Example = w.PointedOrLemma()
			out[i].ExampleGloss = w.Gloss
			out[i].ExampleRoot = w.Root
		}
	}
	return out, nil
}

// Person is one row of the suffix table: who the verb is about.
type Person struct {
	// Key is the slug: "ani", "ata", "at", "hu", "hi", "anachnu", "atem", "aten", "hem", "hen".
	Key string
	// Hebrew is the pronoun, pointed.
	Hebrew string
	// Translit is the pronoun in Latin letters.
	Translit string
	// English is who that is.
	English string
}

// People are the pronouns a Hebrew verb table uses, in the order a table reads.
var People = []Person{
	{Key: "ani", Hebrew: "אֲנִי", Translit: "ani", English: "I"},
	{Key: "ata", Hebrew: "אַתָּה", Translit: "ata", English: "you (m.)"},
	{Key: "at", Hebrew: "אַתְּ", Translit: "at", English: "you (f.)"},
	{Key: "hu", Hebrew: "הוּא", Translit: "hu", English: "he"},
	{Key: "hi", Hebrew: "הִיא", Translit: "hi", English: "she"},
	{Key: "anachnu", Hebrew: "אֲנַחְנוּ", Translit: "anachnu", English: "we"},
	{Key: "atem", Hebrew: "אַתֶּם", Translit: "atem", English: "you (m. pl.)"},
	{Key: "aten", Hebrew: "אַתֶּן", Translit: "aten", English: "you (f. pl.)"},
	{Key: "hem", Hebrew: "הֵם", Translit: "hem", English: "they"},
}

// Ending is one row of a rule: a pronoun and the ending that goes with it.
type Ending struct {
	Person Person
	// Suffix is the letters added to the stem, pointed where points are usual.
	Suffix string
	// Note explains a trap on this row, such as two pronouns sharing one form.
	Note string
}

// PastEndings is how a Hebrew past-tense verb changes with its subject. These
// are the endings on the plain stem; the stem itself is the "he" form.
var PastEndings = []Ending{
	{Person: People[0], Suffix: "־תִּי"},
	{Person: People[1], Suffix: "־תָּ"},
	{Person: People[2], Suffix: "־תְּ"},
	{Person: People[3], Suffix: "the plain stem, with no ending"},
	{Person: People[4], Suffix: "־ָה"},
	{Person: People[5], Suffix: "־נוּ"},
	{Person: People[6], Suffix: "־תֶּם"},
	{Person: People[7], Suffix: "־תֶּן"},
	{Person: People[8], Suffix: "־וּ", Note: "One form serves both they (m.) and they (f.)."},
}

// PresentEndings is how a Hebrew present-tense verb changes. Hebrew present
// verbs behave like adjectives, so there are four forms, not six.
var PresentEndings = []Ending{
	{Person: Person{Key: "ms", Hebrew: "הוּא", Translit: "hu", English: "he"}, Suffix: "the plain form"},
	{Person: Person{Key: "fs", Hebrew: "הִיא", Translit: "hi", English: "she"}, Suffix: "־ָה or ־ֶת"},
	{Person: Person{Key: "mp", Hebrew: "הֵם", Translit: "hem", English: "they (m.)"}, Suffix: "־ִים"},
	{Person: Person{Key: "fp", Hebrew: "הֵן", Translit: "hen", English: "they (f.)"}, Suffix: "־וֹת"},
}

// FuturePrefixes is how a Hebrew future-tense verb starts. The pronoun is built
// into the front of the word, which is why Hebrew often leaves the pronoun out.
var FuturePrefixes = []Ending{
	{Person: People[0], Suffix: "א־"},
	{Person: People[1], Suffix: "ת־"},
	{Person: People[2], Suffix: "ת־ ... ־י", Note: "You (f.) adds a yod at the end as well."},
	{Person: People[3], Suffix: "י־"},
	{Person: People[4], Suffix: "ת־", Note: "She uses the same word as he, except in careful writing."},
	{Person: People[5], Suffix: "נ־"},
	{Person: People[6], Suffix: "ת־ ... ־ו"},
	{Person: People[7], Suffix: "ת־ ... ־נָה"},
	{Person: People[8], Suffix: "י־ ... ־ו"},
}

// VerbForms is one verb and the forms the course data holds for it.
type VerbForms struct {
	Word   CourseWord
	Binyan Binyan
	// Root is the three root letters, pointed with maqaf, such as "כ־ת־ב".
	Root string
	// Forms are the extra spellings the dictionary lists, in the data's order,
	// each with its niqqud dropped as the list stores them.
	Forms []string
	// Tenses names the tenses this verb's forms cover, read from the endings in
	// the data. Empty means the data cannot say.
	Tenses []string
}

// Plain returns the consonant spelling, which is how the forms are stored.
func PlainForm(s string) string { return units.Plain(s) }

// VerbFormList lists ready verbs whose data carries forms to show, in course
// order, at most limit. Zero means no limit.
func VerbFormList(limit int) ([]VerbForms, error) {
	rows, err := CourseReady()
	if err != nil {
		return nil, err
	}
	var out []VerbForms
	for _, w := range rows {
		if w.POS != "verb" || w.Root == "" || w.Binyan == "" || len(w.Forms) < 4 {
			continue
		}
		binyan, ok := LookupBinyan(w.Binyan)
		if !ok {
			continue
		}
		out = append(out, VerbForms{
			Word: w, Binyan: binyan, Root: w.Root,
			Forms: dedupeForms(w.Forms), Tenses: tensesIn(w.Forms),
		})
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out, nil
}

// VerbByLemma finds one ready verb by its unpointed lemma.
func VerbByLemma(lemma string) (VerbForms, bool, error) {
	rows, err := CourseReady()
	if err != nil {
		return VerbForms{}, false, err
	}
	want := strings.TrimSpace(lemma)
	for _, w := range rows {
		if w.POS != "verb" || w.Lemma != want || len(w.Forms) < 4 {
			continue
		}
		binyan, ok := LookupBinyan(w.Binyan)
		if !ok {
			continue
		}
		return VerbForms{Word: w, Binyan: binyan, Root: w.Root, Forms: dedupeForms(w.Forms), Tenses: tensesIn(w.Forms)}, true, nil
	}
	return VerbForms{}, false, nil
}

// dedupeForms drops an empty form and a repeat, keeping the data's order.
func dedupeForms(forms []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range forms {
		f = strings.TrimSpace(f)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// tensesIn reads which tenses a verb's forms cover, from the endings the list
// actually holds. It never claims a tense the data does not show.
func tensesIn(forms []string) []string {
	var past, present, future bool
	for _, f := range forms {
		switch {
		case strings.HasSuffix(f, "תי") || strings.HasSuffix(f, "נו") || strings.HasSuffix(f, "תם") || strings.HasSuffix(f, "תן"):
			past = true
		case strings.HasSuffix(f, "ים") || strings.HasSuffix(f, "ות"):
			present = true
		}
		switch {
		case strings.HasPrefix(f, "א") || strings.HasPrefix(f, "נ") || strings.HasPrefix(f, "י") || strings.HasPrefix(f, "ת"):
			future = true
		}
	}
	var out []string
	if past {
		out = append(out, "Past")
	}
	if present {
		out = append(out, "Present")
	}
	if future {
		out = append(out, "Future")
	}
	return out
}

// PointedOrLemma is the pointed spelling, falling back to the lemma.
func (w CourseWord) PointedOrLemma() string {
	if w.Pointed != "" {
		return w.Pointed
	}
	return w.Lemma
}

// RootLetters splits a root such as "כ־ת־ב" into its letters. A root with no
// maqaf is returned as one part.
func RootLetters(root string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	return strings.Split(root, "־")
}

// BinyanCounts is the number of ready verbs in each pattern, keyed by slug.
func BinyanCounts() (map[string]int, error) {
	rows, err := CourseReady()
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, w := range rows {
		if w.POS == "verb" && w.Binyan != "" {
			out[w.Binyan]++
		}
	}
	return out, nil
}
