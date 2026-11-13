package views

// VerbsIndex is the page that teaches the seven binyanim.
type VerbsIndex struct {
	Page
	// Binyanim is the seven patterns, in teaching order, with real counts.
	Binyanim []BinyanCard
	// Verbs is a short list of ready verbs to open, in course order.
	Verbs []VerbRow
	// Total is how many ready verbs the course holds.
	Total int
	// Note explains where the words come from, so the page never overclaims.
	Note string
}

// BinyanCard is one pattern on the index.
type BinyanCard struct {
	Key      string
	Name     string
	Translit string
	English  string
	Shape    string
	Count    int
	Example  string
	Gloss    string
	Root     string
	// Href opens the pattern's own page.
	Href string
}

// VerbRow is one verb on the index, linking to its own page.
type VerbRow struct {
	Href   string
	Text   string
	Gloss  string
	Root   string
	Binyan string
	// Tenses names what its data can show, such as "Past, Present".
	Tenses string
}

// VerbPage is one verb taught in full.
type VerbPage struct {
	Page
	Text string
	// Gloss is the meaning as the course has it, such as "to write".
	Gloss  string
	Root   string
	Binyan BinyanCard
	// Forms are the spellings the signed course data holds for this verb.
	Forms []string
	// Tenses names the tenses those forms cover.
	Tenses []string
	// Past, Present and Future are the rule tables, shown so the learner can
	// read any verb, not only this one.
	Past    []EndingRow
	Present []EndingRow
	Future  []EndingRow
	// Note says plainly that the forms come from the course list.
	Note string
}

// EndingRow is one line of a rule table.
type EndingRow struct {
	Pronoun  string
	Translit string
	English  string
	Suffix   string
	Note     string
}

// joinComma joins a short list in plain English: "Past, Present and Future".
func joinComma(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	}
	out := ""
	for i, item := range items {
		switch {
		case i == 0:
			out = item
		case i == len(items)-1:
			out += " and " + item
		default:
			out += ", " + item
		}
	}
	return out
}
