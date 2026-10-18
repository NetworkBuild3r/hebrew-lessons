package pages

// Option is one select or chip value. He is pointed where the label is Hebrew.
type Option struct {
	V, He, En string
}

var posOptions = []Option{
	{"noun", "שם עצם", "noun"},
	{"verb", "פועל", "verb"},
	{"adjective", "שם תואר", "adjective"},
	{"adverb", "תואר הפועל", "adverb"},
	{"preposition", "מילת יחס", "preposition"},
	{"pronoun", "כינוי", "pronoun"},
	{"conjunction", "מילת חיבור", "conjunction"},
	{"particle", "מילית", "particle"},
	{"numeral", "מספר", "numeral"},
	{"other", "אחר", "other"},
}

var binyanOptions = []Option{
	{"paal", "פָּעַל", "paal"},
	{"nifal", "נִפְעַל", "nifal"},
	{"piel", "פִּעֵל", "piel"},
	{"pual", "פֻּעַל", "pual"},
	{"hifil", "הִפְעִיל", "hifil"},
	{"hufal", "הֻפְעַל", "hufal"},
	{"hitpael", "הִתְפַּעֵל", "hitpael"},
}

var tenseOptions = []Option{
	{"infinitive", "שם פועל", "infinitive"},
	{"present", "זמן הווה", "present"},
	{"past", "זמן עבר", "past"},
	{"future", "זמן עתיד", "future"},
	{"imperative", "ציווי", "imperative"},
}

var statusOptions = []Option{
	{"", "הכל", "All"},
	{"new", "חדשות", "New"},
	{"learning", "בלמידה", "Learning"},
	{"longTerm", "לטווח ארוך", "Long-term"},
}

// Words status labels differ from the shared chip (In progress / Done / Locked).
var wordStages = map[string][2]string{
	"new":      {"חדשה", "New"},
	"learning": {"בלמידה", "Learning"},
	"longTerm": {"לטווח ארוך", "Long-term"},
}

var wordStageStatus = map[string]string{
	"new":      "new",
	"learning": "progress",
	"longTerm": "done",
}

var stageRank = map[string]int{"new": 0, "learning": 1, "longTerm": 2}

// Strength buckets match server/learner/model.js. pct is the legacy bar width.
var strengthMeta = map[string]struct {
	pct int
	he  string
	en  string
}{
	"fragile": {25, "שביר", "Fragile"},
	"shaky":   {50, "רועד", "Shaky"},
	"steady":  {75, "יציב", "Steady"},
	"solid":   {100, "איתן", "Solid"},
}

var rootColour = map[string]string{
	"regular":   "green",
	"lamed-he":  "yellow",
	"peh-yod":   "red",
	"peh-nun":   "red",
	"ayin-vav":  "red",
	"irregular": "red",
}

// KnownPOS, KnownBinyan, and KnownTense report whether a filter value is in the set.
func KnownPOS(value string) bool    { return known(posOptions, value) }
func KnownBinyan(value string) bool { return known(binyanOptions, value) }
func KnownTense(value string) bool  { return known(tenseOptions, value) }

func known(options []Option, value string) bool {
	if value == "" {
		return true
	}
	for _, o := range options {
		if o.V == value {
			return true
		}
	}
	return false
}

func posLabel(v string) (string, string)   { return pair(posOptions, v) }
func binyanHe(v string) string             { he, _ := pair(binyanOptions, v); return he }
func binyanEn(v string) string             { _, en := pair(binyanOptions, v); return en }
func tenseLabel(v string) (string, string) { return pair(tenseOptions, v) }

func pair(options []Option, value string) (string, string) {
	for _, o := range options {
		if o.V == value {
			return o.He, o.En
		}
	}
	return value, value
}

func stageLabel(stage string) (string, string) {
	if row, ok := wordStages[stage]; ok {
		return row[0], row[1]
	}
	return "חדשה", "New"
}

func stageStatus(stage string) string {
	if s, ok := wordStageStatus[stage]; ok {
		return s
	}
	return "new"
}

func lowerStage(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if stageRank[a] <= stageRank[b] {
		return a
	}
	return b
}

func strengthLabel(bucket string) (string, string) {
	if m, ok := strengthMeta[bucket]; ok {
		return m.he, m.en
	}
	return "", ""
}

func strengthWidth(bucket string) string {
	m, ok := strengthMeta[bucket]
	if !ok {
		return ""
	}
	return strconvPct(m.pct)
}

func strconvPct(n int) string {
	return itoa(n) + "%"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [4]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func sourceLabel(v string) (string, string) {
	switch v {
	case "lesson":
		return "שיעורים", "Lessons"
	case "self":
		return "הוספתי", "Added by me"
	case "ai":
		return "AI", "AI"
	default:
		return v, v
	}
}

func weakLabel(v string) (string, string) {
	if v == "very_weak" {
		return "חלשה מאוד", "Very weak"
	}
	return "חלשה", "Weak"
}

func rootClassColour(v string) string {
	return rootColour[v]
}
