package pages

// GuideCard is one published guide on the shelf.
type GuideCard struct {
	PlanID   string
	LessonID string
	Title    string
	Lang     string
	Dir      string
	Href     string
	Topic    string
	Cover    string
	Letter   string
	// HeTitle is a second Hebrew line. The catalog has one title, so this
	// stays empty until a plan stores both.
	HeTitle string
	// Status is set only when progress is known. Empty omits the chip.
	Status string
}

// TopicGroup is guides that share a unit title, in first-seen order.
type TopicGroup struct {
	Topic string
	Slug  string
	Lang  string
	Dir   string
	Cards []GuideCard
}

// GuideList is the shelf. Empty Topics with Failed false is the empty state.
// Hero is the first guide when at least three are visible, and that guide is
// not also in Topics.
type GuideList struct {
	Topics []TopicGroup
	Hero   *GuideCard
	Count  int
	Failed bool
}

// Example is one pointed Hebrew line in an explainer.
type Example struct {
	ID       string
	He       string
	En       string
	Translit string
	Root     string
	// Clip is a shared-word clip name when the plan stores one. Explainers
	// in the 1.1 schema have no audio path, so this stays empty and the
	// button still carries the Hebrew on data-say.
	Clip string
}

// Callout is a traffic-light note. Tone is green, yellow, red, info, or empty.
type Callout struct {
	ID    string
	Tone  string
	Title string
	Text  string
}

// Table is a plain explainer table. Cells are already clipped.
type Table struct {
	Columns []string
	Rows    [][]string
}

// Section is one explainer section.
type Section struct {
	ID         string
	Heading    string
	Paragraphs []string
	Examples   []Example
	Table      *Table
	Callouts   []Callout
}

// CompareSide is one column of a compare block. Stem and Ending are split
// on Hebrew grapheme clusters so nikud stays on its letter.
type CompareSide struct {
	Tag      string
	He       string
	Stem     string
	Ending   string
	Translit string
	Gloss    string
	Clip     string
}

// Compare is a side-by-side block. Nil when the lesson has none.
type Compare struct {
	Title string
	Note  string
	Sides []CompareSide
}

// Verse is a verse-reading block. Nil when the lesson has none.
type Verse struct {
	Ref  string
	Text string
	En   string
	Clip string
}

// Block is one explainer inside a guide lesson.
type Block struct {
	ID       string
	Title    string
	Sections []Section
	Compare  *Compare
	Verse    *Verse
}

// GuideDetail is one published guide lesson.
type GuideDetail struct {
	PlanID    string
	LessonID  string
	Title     string
	Lang      string
	Dir       string
	Topic     string
	TopicLang string
	TopicDir  string
	Blocks    []Block
}

// ToneWord is the bilingual label for a callout tone.
type ToneWord struct {
	En string
	He string
}

// ToneWords matches explainer.js. An unknown tone uses info.
var ToneWords = map[string]ToneWord{
	"green":  {En: "Go", He: "קדימה"},
	"yellow": {En: "Check", He: "בדקו"},
	"red":    {En: "Stop", He: "עצרו"},
	"info":   {En: "Note", He: "שימו לב"},
}

// ToneOf returns the tone when it is one of the four words, otherwise empty.
func ToneOf(tone string) string {
	if _, ok := ToneWords[tone]; ok {
		return tone
	}
	return ""
}

func toneAttr(tone string) string {
	_, attr := ToneLabel(tone)
	return attr
}

func toneEn(tone string) string {
	word, _ := ToneLabel(tone)
	return word.En
}

func toneHe(tone string) string {
	word, _ := ToneLabel(tone)
	return word.He
}

// ToneLabel returns the word pair and the data-tone value.
func ToneLabel(tone string) (word ToneWord, attr string) {
	name := ToneOf(tone)
	word, ok := ToneWords[name]
	if !ok {
		word = ToneWords["info"]
	}
	attr = name
	if attr == "" {
		attr = "info"
	}
	return word, attr
}
