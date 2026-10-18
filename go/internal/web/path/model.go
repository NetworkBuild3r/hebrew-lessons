// Package path turns a learner Path into the snapshot the templates render.
// The learner types live in go/internal/web/learner/path until PR #37 merges.
package path

// Snapshot is the rendered path. Review stops are visual nodes with Kind
// "review". They are not plan lessons.
type Snapshot struct {
	PlanID        string
	UnitCount     int
	Units         []Unit
	Due           int
	DueKnown      bool
	DueWords      []DueWord
	WarmupDue     int
	ReviewStopDue int
	HeroKickerEn  string
	HeroKickerHe  string
	HeroTitleHe   string
	HeroSpeak     string
	HeroMetaEn    string
	HeroMetaHe    string
	ContinueHref  string
	Strip         []Node
	StripEn       string
	StripHe       string
	Tutor         *TutorCard
	Rail          Rail
	DueEn         string
	DueHe         string
}

// Rail is the sidebar goal. The label is "Unit N". The ring and the
// words line share Learned and Total.
type Rail struct {
	UnitID  string
	UnitN   int
	Learned int
	Total   int
	WordsEn string
	WordsHe string
}

// Unit is one situation on the path.
type Unit struct {
	ID          string
	Order       int
	Title       string
	TitleHe     string
	Concept     string
	ConceptHe   string
	NewLearned  int
	NewTotal    int
	Current     bool
	NotYet      bool
	LessonIndex int
	LessonCount int
	GuideHref   string
	KickerEn    string
	KickerHe    string
	BannerEn    string
	BannerHe    string
	Nodes       []Node
}

// Node is one circle on the path.
type Node struct {
	Slug           string
	Title          string
	TitleHe        string
	Kind           string
	State          string
	Gate           string
	UnitID         string
	NewCount       int
	KnownCount     int
	Strength       int
	Fading         bool
	ClassDate      string
	ClassDateHe    string
	ClassDateISO   string
	Score          *int
	PassNeeded     int
	Total          int
	Href           string
	CheckpointHref string
	LessonNumber   int
	Due            int
	Label          string
	SubEn          string
	SubHe          string
	HXVals         string
}

// DueWord is one sample on the Today due card.
type DueWord struct {
	He       string
	En       string
	Strength int
	Speak    string
}

// TutorCard is the class waiting on the path. Nil hides the card.
type TutorCard struct {
	UnitID     string
	DateLabel  string
	DateHe     string
	DateISO    string
	SentenceEn string
	SentenceHe string
	Phrase     string
	PhraseEn   string
	Href       string
	Clip       string
}
