package views

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
)

// The Progress page (INIT-015/SPEC-018). web/progress.go fills these from the learner model,
// the daily snapshots, the regression events, the placement runs, and the weekly reports, and
// progress.templ draws them. Nothing here reads a database.

// SkillTab is one of the four skill choices.
type SkillTab struct {
	Skill   string
	Label   string
	Current bool
}

// LegendItem is one state of the bar: its name, its count, and its share of the lexicon, as text.
type LegendItem struct {
	State string // unseen, seen, struggling, regressed, learning, known, solid
	Label string
	Count int
	Share string // "12%"
}

// ChartTable is the table that carries the same values as a chart.
type ChartTable struct {
	Caption string
	Head    []string
	Rows    [][]string
}

// ChartView is one chart with its words. SVG is markup built in web/chart.go: every dynamic
// value in it is escaped there, and it names no colour. When SVG is empty the chart has nothing
// to draw, and Hint says what to do. ThroughToday is set when the series ends on today's live
// point (ADR-0028 D2); Through is a leftover day label only when ThroughToday is false.
type ChartView struct {
	ID           string
	Title        string
	Note         string
	Through      string
	ThroughToday bool
	OldRuleNote  bool // any point still below the current rule version
	SVG          string
	Table        ChartTable
	Summary      string
	Hint         string
	// Key is the legend under a chart: a line of text per mark.
	Key []string
}

// WeekSkill is one skill's row in "This week". Gained and Lost are the net move in words known
// (known plus solid) between the first and the last snapshot of the week. Struggling is how many
// words were struggling or regressed at the start and at the end.
type WeekSkill struct {
	Skill         string
	Label         string
	Gained        int
	Lost          int
	StruggleStart int
	StruggleEnd   int
}

// WeekWord is a word that slipped, or came back, with the skill and the likely cause.
type WeekWord struct {
	Text       string // pointed Hebrew
	Gloss      string
	Skill      string
	SkillLabel string
	Cause      string // the code: confusion, decoding, ...
	CauseLabel string
	Day        string
}

// WeekView is the "This week" card.
type WeekView struct {
	// Compared is true when the week has two snapshot days to compare. From and To are those days.
	Compared bool
	From, To string
	Skills   []WeekSkill
	// Slipped are words that went from known or solid to struggling or regressed this week.
	Slipped []WeekWord
	// Bands are the levels that slipped this week, as the learner sees them (band plus one).
	Bands []int
	// Back are words that slipped in the last four weeks and are not struggling now.
	Back []WeekWord
}

// PlacementRow is one finished placement run or check-in.
type PlacementRow struct {
	Date   string
	Kind   string // First placement, Check-in, Placement
	Level  string // "Level 3" or ""
	Detail string // "about 164 words · Listening level 3, about 164 words · ..."
}

// WeeklyCard is the newest weekly report.
type WeeklyCard struct {
	WeekOf  string
	Summary string
}

// SkillOverview is one skill's row on the Progress overview (no ?skill=).
// Known is known plus solid (legacy). Prefer DirectionRow for INIT-033.
// NoCardHint replaces the counts bar when the skill has answers and no cards yet
// (INIT-023/SPEC-004). NoPracticeHint replaces the bar when spelling has no evidence
// (INIT-032/SPEC-006). EstimateSource is "evidence" or "placement"; PlacementDay is
// set when the estimate is from placement.
type SkillOverview struct {
	Skill          string
	Label          string
	Known          int
	Learning       int
	Seen           int
	NoCardHint     bool
	NoPracticeHint bool
	EstimateSource string
	PlacementDay   string
}

// DirectionRow is one direction on the Progress overview grid (ADR-0028 D4, D5, D7).
type DirectionRow struct {
	Skill         string
	Label         string // DirectionLabel
	Known, Solid  int
	Learning      int
	Struggling    int
	Due           int
	AccuracyText  string // "72% · 68 answers", or ""
	AccuracyHE    string
	Verdict       string // needs_help | strong | building | not_askable
	VerdictLabel  string // "Needs help", …
	Reason        string
	PlacementLine string // "Placement estimate: about 504 (listening, Nov 6)"
	PlacementHE   string
	NoAudio       bool
	PracticeHref  string // struggling-words practice filtered by skill
	Href          string // skill tab
}

// StandingWord is one help or strong word on the overview.
type StandingWord struct {
	Hebrew, English string
	MissText        string // "missed 4 of 6"
	MissHE          string
	Cause           string // plain words
	CauseHE         string
	Href            string // /words/{id}
}

// StandingBlock is one direction under "Where you need help" or "Where you're strong".
type StandingBlock struct {
	Skill        string
	Label        string
	Reason       string
	Words        []StandingWord
	PracticeHref string // help blocks only
}

// ProgressView is the whole page.
type ProgressView struct {
	Page
	// Empty is true when the learner has no snapshot and has seen no word: nothing to chart.
	Empty bool
	// UnitsText is "2/3" units finished, or "" when none are published.
	UnitsText string
	// LessonsText is "5/7" lessons finished inside those units, or "" when there are none.
	LessonsText string
	Weekly      *WeeklyCard
	Tabs        []SkillTab
	Skill       string
	SkillName   string
	// Overview is the legacy per-skill counts (kept for skill-tab empty paths). Prefer Directions.
	Overview []SkillOverview
	// Directions is the overview grid: one row per skill with Standing (ADR-0028).
	Directions []DirectionRow
	// Help and Strong are the overview sections keyed by verdict (ADR-0028 D5).
	Help   []StandingBlock
	Strong []StandingBlock
	// Known is the estimated words known in the chosen skill now, as text.
	// On the overview it is Standing.KnownAnySkill (ADR-0027 D1 / ADR-0028 D1).
	Known string
	// EstimateSource and PlacementDay label the skill-tab estimate (INIT-032/SPEC-006).
	EstimateSource string
	PlacementDay   string
	Legend         []LegendItem
	BarSVG         string
	BarTable       ChartTable
	Total          int
	Est            ChartView
	Retention      ChartView
	Week           WeekView
	Placement      []PlacementRow
	// CanSay is what the learner can produce from memory. Nil when there is no signed-in learner.
	CanSay *CanSayPanel
}

// CanSayWord is one word in the "you can now say" list. Audio is the clip src, empty without one.
type CanSayWord struct {
	He    string
	Gloss string
	Audio string
}

// CanSayPanel is the Progress section. Empty is true before any word is known.
type CanSayPanel struct {
	Empty     bool
	TotalText string
	AddedText string
	Items     []CanSayWord
	Next      []CanSayWord
}

// SkillTabHref is /progress?skill=…, keeping the Hebrew shell when needed.
// Page.Href only appends ?lang=he, so a path that already has a query must not use it.
func (v ProgressView) SkillTabHref(skill string) string {
	path := "/progress?skill=" + skill
	if v.Lang == "he" {
		return path + "&lang=he"
	}
	return path
}

// SignedGain is "+9" for a gain, and "0" for none.
func SignedGain(n int) string {
	if n <= 0 {
		return "0"
	}
	return "+" + strconv.Itoa(n)
}

// SignedLoss is "−2" for a loss (with a real minus sign), and "0" for none.
func SignedLoss(n int) string {
	if n <= 0 {
		return "0"
	}
	return "\u2212" + strconv.Itoa(n)
}

// shortDayOf is "Oct 18" for 2026-10-18, or the text unchanged when it is not a day.
func shortDayOf(day string) string {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return day
	}
	return t.Format("Jan 2")
}

// CauseLabel is the plain words for a regression cause, or for a word's diagnosed cause.
func CauseLabel(cause string) string {
	switch {
	case cause == "confusion":
		return "mixed up with another word"
	case cause == "decoding":
		return "hears it, cannot read it yet"
	case cause == "production":
		return "reads it, cannot recall it yet"
	case cause == "spelling":
		return "recalls it, cannot spell it yet"
	case cause == "band_slip", strings.HasPrefix(cause, "band_slip"):
		return "its whole level is slipping"
	case cause == "unexplained", cause == "":
		return "no clear cause yet"
	}
	return "no clear cause yet"
}

// HelpCauseText is the Progress help-list cause (ADR-0028 D5 / SPEC-007): letter
// swaps and mixed-up words first, then the Diagnose cause in plain words.
func HelpCauseText(cause, pair, confusedWith string) string {
	if confusedWith != "" {
		return "mixed up with " + confusedWith
	}
	if a, b, ok := strings.Cut(pair, "/"); ok {
		ar, br := []rune(a), []rune(b)
		if len(ar) == 1 && len(br) == 1 {
			return "swaps " + a + " and " + b
		}
	}
	switch cause {
	case "production":
		return "can read it, can't produce it yet"
	case "decoding":
		return "hears it, can't read it yet"
	case "spelling":
		return "can recall it, can't spell it yet"
	case "confusion":
		return "mixed up with another word"
	case "band_slip":
		return "its whole level is slipping"
	case "", "unexplained":
		return "no clear cause yet"
	}
	return CauseLabel(cause)
}

// HelpCauseTextHE is HelpCauseText in Hebrew, for the Hebrew shell.
func HelpCauseTextHE(cause, pair, confusedWith string) string {
	if confusedWith != "" {
		return "מִתְבַּלְבֵּל עִם " + confusedWith
	}
	if a, b, ok := strings.Cut(pair, "/"); ok {
		if len([]rune(a)) == 1 && len([]rune(b)) == 1 {
			return "מַחְלִיף בֵּין " + a + " לְ־" + b
		}
	}
	switch cause {
	case "production":
		return "קוֹרֵא אוֹתָהּ, עֲדַיִן לֹא שׁוֹלֵף אוֹתָהּ"
	case "decoding":
		return "שׁוֹמֵעַ אוֹתָהּ, עֲדַיִן לֹא קוֹרֵא אוֹתָהּ"
	case "spelling":
		return "זוֹכֵר אוֹתָהּ, עֲדַיִן לֹא מְאַיֵּת אוֹתָהּ"
	case "confusion":
		return "מִתְבַּלְבֵּל עִם מִלָּה אַחֶרֶת"
	case "band_slip":
		return "כָּל הָרָמָה שֶׁלָּהּ יוֹרֶדֶת"
	}
	if strings.HasPrefix(cause, "band_slip") {
		return "כָּל הָרָמָה שֶׁלָּהּ יוֹרֶדֶת"
	}
	return "עֲדַיִן אֵין סִבָּה בְּרוּרָה"
}

// AnswersTextHE is "72% · 68 answers" in Hebrew.
func AnswersTextHE(pct string, answers int) string {
	return fmt.Sprintf("%s · %d תְּשׁוּבוֹת", pct, answers)
}

// MissTextHE is "missed 4 of 6" in Hebrew.
func MissTextHE(misses, answers int) string {
	return fmt.Sprintf("%d שְׁגִיאוֹת מִתּוֹךְ %d", misses, answers)
}

// PlacementLineHE is "Placement estimate: about 504 (reading, Sep 28)" in Hebrew.
func PlacementLineHE(estimate float64, direction, day string) string {
	return fmt.Sprintf("הַעֲרָכָה מִבְּדִיקַת רָמָה: בְּעֵרֶךְ %.0f (%s, %s)", estimate, direction, day)
}

// VerdictLabel is the chip text for a Standing verdict.
func VerdictLabel(verdict string) string {
	switch verdict {
	case "needs_help":
		return "Needs help"
	case "strong":
		return "Strong"
	case "building":
		return "Building"
	case "not_askable":
		return "No audio yet"
	}
	return verdict
}

// StateLabel is the learner-facing name of a word state.
func StateLabel(state string) string {
	switch state {
	case "unseen":
		return "Not seen yet"
	case "seen":
		return "Seen"
	case "struggling":
		return "Struggling"
	case "regressed":
		return "Slipping"
	case "learning":
		return "Learning"
	case "known":
		return "Known"
	case "solid":
		return "Solid"
	}
	return state
}

// Progress string table (INIT-032/SPEC-006, INIT-033/SPEC-007). English is the key; Hebrew fills when present.
var progressHE = map[string]string{
	"Words you know":           "מִלִּים שֶׁיְּדוּעוֹת",
	"Words you know over time": "מִלִּים שֶׁיְּדוּעוֹת לְאֹרֶךְ הַזְּמַן",
	"Not tested by placement":  "לֹא נִבְדַּק בִּבְדִיקַת רָמָה",
	"From placement on":        "מִבְּדִיקַת רָמָה ב־",
	"Not practiced yet: type the Hebrew in review to start": "עֲדַיִן לֹא תֻּרְגַּל: הַקְלֵד אֶת הָעִבְרִית בַּחֲזָרָה כְּדֵי לְהַתְחִיל",
	"Reading not tested":  "קְרִיאָה לֹא נִבְדְּקָה",
	"Through":             "עַד",
	"Through today":       "עַד הַיּוֹם",
	"old rule":            "כְּלָל יָשָׁן",
	"Where you need help": "אֵיפֹה צָרִיךְ עֶזְרָה",
	"Where you're strong": "אֵיפֹה אַתָּה חָזָק",
	"Needs help":          "צָרִיךְ עֶזְרָה",
	"Strong":              "חָזָק",
	"Building":            "בִּבְנִיָּה",
	"No audio yet":        "עֲדַיִן אֵין שֵׁמַע",
	"No audio yet — practiced in lessons only": "עֲדַיִן אֵין שֵׁמַע — תֻּרְגַּל רַק בַּשִּׁעוּרִים",
	"practice these":             "תַּרְגֵּל אֵלֶּה",
	"known":                      "יְדוּעוֹת",
	"solid":                      "יַצִּיבוֹת",
	"learning":                   "בְּלִימּוּד",
	"struggling":                 "מִתְקַשּׁוֹת",
	"due":                        "לַחֲזָרָה",
	"Reading (Hebrew → English)": "קְרִיאָה (עִבְרִית ← אַנְגְּלִית)",
	"English → Hebrew":           "אַנְגְּלִית ← עִבְרִית",
	"Spelling":                   "אִיּוּת",
	"Listening":                  "הַקְשָׁבָה",
	"Dashed: old rule":           "מְקֻוְקָו: כְּלָל יָשָׁן",
}

// ProgressHE is the Hebrew for a Progress string, or the English when there is none.
func ProgressHE(en string) string {
	if he := progressHE[en]; he != "" {
		return he
	}
	return en
}

// ProgressT is UI copy for the Progress and Home number labels. Missing Hebrew falls back to English.
func ProgressT(lang, en string) templ.Component {
	return T(lang, progressHE[en], en)
}
