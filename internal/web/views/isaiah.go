package views

import "github.com/NetworkBuild3r/hebrew-lessons/internal/richtext"

// IsaiahChapter is one chapter that has a checked verse.
type IsaiahChapter struct {
	Label string
	Href  string
}

// IsaiahWord is one tappable word. RootHref is set only when the root has a page.
type IsaiahWord struct {
	Text     string
	Gloss    string
	Root     string
	RootHref string
}

// IsaiahLayer is one checked section.
type IsaiahLayer struct {
	Title string
	Nodes []richtext.Node
}

// IsaiahVerse is one verse in the reader.
type IsaiahVerse struct {
	Heading  string
	He       string
	En       string
	Audio    string
	Words    []IsaiahWord
	Layers   []IsaiahLayer
	PrevHref string
	NextHref string
}

// IsaiahView is the chapter list or one verse.
type IsaiahView struct {
	Page     Page
	Chapters []IsaiahChapter
	Verse    *IsaiahVerse
}

// AdminIsaiahNote is one layer on the check page, draft or checked.
type AdminIsaiahNote struct {
	Layer  string
	Body   string
	Status string
}

// AdminIsaiahGloss is one word on the check page.
type AdminIsaiahGloss struct {
	Word   string
	Gloss  string
	Status string
}

// AdminIsaiahView is the page where a person checks a verse.
type AdminIsaiahView struct {
	Page     Page
	Heading  string
	He       string
	Notes    []AdminIsaiahNote
	Glosses  []AdminIsaiahGloss
	CanCheck bool
}
