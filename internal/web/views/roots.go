package views

// RootRow is one root that has a page.
type RootRow struct {
	Href       string
	Root       string
	Gloss      string
	WordCount  string
	VerseCount string
}

// RootsIndexView is the roots index.
type RootsIndexView struct {
	Page  Page
	Roots []RootRow
}

// RootWord is one word of a family, shown as a card.
type RootWord struct {
	Word
}

// RootLessonLink is a lesson that uses the root.
type RootLessonLink struct {
	Href  string
	Title string
}

// RootVerseView is one verse. Spans mark the family's words.
type RootVerseView struct {
	Ref   string
	En    string
	Spans []RootSpan
}

// RootSpan is a piece of a verse.
type RootSpan struct {
	Text string
	Mark bool
}

// RootFormView is one verb form of the family.
type RootFormView struct {
	Label string
	He    string
	Known bool
}

// RootPageView is one root.
type RootPageView struct {
	Page      Page
	Root      string
	Gloss     string
	Words     []Word
	Forms     []RootFormView
	Lessons   []RootLessonLink
	Verses    []RootVerseView
	VerseHead string
}
