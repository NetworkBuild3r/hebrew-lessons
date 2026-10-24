package views

func knownAttr(known bool) string {
	if known {
		return "true"
	}
	return "false"
}

// StuffWord is one word the learner has met.
type StuffWord struct {
	Text  string
	Gloss string
	State string
	Root  string
	Href  string
}

// WordsView is the word bank.
type WordsView struct {
	Page           Page
	State          string
	AllHref        string
	LearningHref   string
	KnownHref      string
	StrugglingHref string
	Words          []StuffWord
	Empty          string
}

// VerbCell is one form in a verb table.
type VerbCell struct {
	Label    string
	He       string
	Translit string
	Known    bool
}

// VerbTable is one verb the learner knows a form of.
type VerbTable struct {
	Lemma string
	Rows  []VerbCell
}

// VerbsView is the verb tables.
type VerbsView struct {
	Page  Page
	Verbs []VerbTable
	Empty string
}

// MistakeRow is one personal mistake.
type MistakeRow struct {
	Wrong   string
	Right   string
	Note    string
	Times   string
	HowHref string
	How     string
}

// MistakesView is the learner's own mistakes.
type MistakesView struct {
	Page     Page
	Mistakes []MistakeRow
	Empty    string
}

