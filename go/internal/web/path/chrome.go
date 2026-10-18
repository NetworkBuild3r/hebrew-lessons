package path

// Face is display copy the learner Path does not carry: pointed Hebrew,
// worksheet hrefs, strength, and checkpoint size. It is fixture chrome
// until the plan snapshot is the source for those fields.
type Face struct {
	He         string
	Known      int
	Strength   int
	Fading     bool
	Checkpoint bool
	Total      int
	Href       string
}

type unitFace struct {
	TitleHe    string
	Concept    string
	ConceptHe  string
	GuideHref  string
	TutorHe    string
	DateLabel  string
	DateHe     string
	SentenceEn string
	SentenceHe string
	Phrase     string
	PhraseEn   string
	TutorHref  string
}

// fixtureCourseUnits is the kicker denominator for the demo plan, which
// shows four units of a twelve-unit course.
const fixtureCourseUnits = 12

func lessonFace(id string) Face {
	return lessonFaces[id]
}

func unitChrome(id string) unitFace {
	return unitFaces[id]
}

var lessonFaces = map[string]Face{
	"hello":                    {He: "שָׁלוֹם", Known: 4, Strength: 4, Href: "/worksheets/hello/"},
	"what-i-have":              {He: "יֵשׁ לִי", Known: 8, Strength: 1, Fading: true, Href: "/worksheets/what-i-have/"},
	"where-is-it":              {He: "אֵיפֹה זֶה", Known: 10, Strength: 3, Href: "/worksheets/where-is-it/"},
	"morning":                  {He: "בֹּקֶר", Known: 6, Strength: 4, Href: "/worksheets/morning/"},
	"the-street":               {He: "הָרְחוֹב", Known: 10, Strength: 3, Href: "/worksheets/the-street/"},
	"out-the-door":             {He: "יוֹצְאִים", Known: 12, Strength: 3, Href: "/worksheets/out-the-door/"},
	"at-the-shop":              {He: "בַּחֲנוּת", Known: 12, Href: "/worksheets/at-the-shop/"},
	"asking":                   {He: "לִשְׁאוֹל", Known: 12, Href: "/worksheets/asking/"},
	"out-and-about-checkpoint": {He: "מִבְחָן", Checkpoint: true, Total: 20, Href: "/worksheets/out-and-about-checkpoint/"},
	"coffee":                   {He: "קָפֶה", Href: "/worksheets/coffee/"},
	"the-bill":                 {He: "הַחֶשְׁבּוֹן", Href: "/worksheets/the-bill/"},
	"cafe-checkpoint":          {He: "מִבְחָן", Checkpoint: true, Total: 20, Href: "/worksheets/cafe-checkpoint/"},
	"class-with-aaron":         {He: "שִׁעוּר עִם אַהֲרֹן", Href: "/worksheets/class-with-aaron/"},
}

var unitFaces = map[string]unitFace{
	"unit-1": {TitleHe: "מִסְתּוֹבְבִים"},
	"unit-2": {
		TitleHe: "בַּחוּץ", Concept: "Infinitive", ConceptHe: "שֵׁם הַפֹּעַל",
		GuideHref: "/guides/",
	},
	"unit-3": {TitleHe: "בְּבֵית קָפֶה"},
	"unit-tutor": {
		TitleHe: "שִׁעוּר עִם אַהֲרֹן", TutorHe: "שִׁעוּר עִם אַהֲרֹן",
		DateLabel: "Tue", DateHe: "יום ג׳",
		SentenceEn: "Tuesday's class practices the infinitive.",
		SentenceHe: "השיעור ביום שלישי מתרגל את שם הפועל.",
		Phrase:     "בֹּקֶר טוֹב", PhraseEn: "good morning",
		TutorHref: "/lessons/#unit-unit-tutor",
	},
}

func demoDueWords() []DueWord {
	return []DueWord{
		{He: "שָׁלוֹם", En: "hello", Strength: 2, Speak: "שָׁלוֹם"},
		{He: "בְּבַקָּשָׁה", En: "please", Strength: 1, Speak: "בְּבַקָּשָׁה"},
		{He: "תּוֹדָה", En: "thank you", Strength: 3, Speak: "תּוֹדָה"},
	}
}
