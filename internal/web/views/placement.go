package views

import (
	"fmt"
	"strconv"
)

// Placement modes select the region rendered at /placement.
const (
	PlaceModeIntro  = "intro"
	PlaceModeResume = "resume"
	PlaceModeFront  = "front"
	PlaceModeBack   = "back"
	PlaceModeBreak  = "break"
	PlaceModeResult = "result"
)

// PlacementCopy is the placement chrome for one interface language.
// Hebrew words on a card travel with the lexeme, each in a word block.
// Deck chrome in Hebrew is unpointed masculine, the immersion layer.
type PlacementCopy struct {
	Lang        string
	Before      string
	Kicker      string
	TakeTitle   string
	TakeHint    string
	TodayResume string
	Heading     string
	Why         string
	BulletSit   string
	BulletFlip  string
	BulletScore string
	Start       string
	Skip        string
	Welcome     string
	ResumeTitle string
	KeepGoing   string
	FinishNow   string
	Direction   string
	ShowMeaning string
	Hint        string
	Flip        string
	Space       string
	DontKnow    string
	Listen      string
	Slow        string
	HowWell     string
	Again       string
	Hard        string
	Good        string
	Easy        string
	AgainSub    string
	HardSub     string
	GoodSub     string
	EasySub     string
	Stop        string
	ToastLead   string
	BreakKicker string
	BreakTitle  string
	BreakBody   string
	DoneToday   string
	SoFar       string
	Samples     string
	AllDone     string
	Results     string
	Added       string
	StartOver   string
	Back        string
	KnowThese   string
	StartHere   string
	StillAhead  string
	Covered     string
	NoteLeft    string
	NoteChange  string
}

// PlacementFor returns the English chrome unless lang is he.
func PlacementFor(lang string) PlacementCopy {
	if lang == "he" {
		return hebrewPlacement()
	}
	return englishPlacement()
}

func englishPlacement() PlacementCopy {
	return PlacementCopy{
		Lang:        "en",
		Kicker:      "Optional",
		Before:      "Before you start",
		TakeTitle:   "Take the placement check",
		TakeHint:    "A short check of words you know. You can leave and come back. Today stays open.",
		TodayResume: "Resume placement",
		Heading:     "Let’s see what you already know",
		Why:         "So you don’t spend time on words you already know.",
		BulletSit:   "Short sittings of 20 cards, about 3 minutes each. Up to about 100 cards, spread over a few days if you like.",
		BulletFlip:  "For each word: try to recall it, flip the card, and rate how well you knew it.",
		BulletScore: "There’s no score. Each rating already counts as your first review of that word, and you can stop at any time.",
		Start:       "Let’s start",
		Skip:        "Skip and start from the beginning",
		Welcome:     "Welcome back",
		ResumeTitle: "Pick up where you left off",
		KeepGoing:   "Keep going",
		FinishNow:   "Finish now and see where I start",
		Direction:   "Hebrew → English",
		ShowMeaning: "Show the meaning",
		Hint:        "Do you know what it means? Try to recall it, then flip the card.",
		Flip:        "Flip",
		Space:       "Space",
		DontKnow:    "I don’t know this",
		Listen:      "Listen",
		Slow:        "0.75×",
		HowWell:     "How well did you know it?",
		Again:       "Again",
		Hard:        "Hard",
		Good:        "Good",
		Easy:        "Easy",
		AgainSub:    "Didn’t know",
		HardSub:     "With effort",
		GoodSub:     "Knew it",
		EasySub:     "Instantly",
		Stop:        "Stop here",
		ToastLead:   "Added to your practice",
		BreakTitle:  "Nice work. Call it a day?",
		BreakBody:   "Everything you rated is in your practice now, and it’ll come back a few at a time.",
		DoneToday:   "Done for today",
		SoFar:       "So far",
		Samples:     "Some words from this sitting",
		AllDone:     "All done",
		Results:     "Here’s where you are",
		StartOver:   "Start from the beginning instead",
		Back:        "Back to Today",
		KnowThese:   "You know these",
		StartHere:   "You start here",
		StillAhead:  "Still ahead",
		Covered:     "Covered",
		NoteChange:  "You can change this any time on Progress.",
	}
}

func hebrewPlacement() PlacementCopy {
	return PlacementCopy{
		Lang:        "he",
		Before:      "לפני שמתחילים",
		Kicker:      "רשות",
		TakeTitle:   "בְּדִיקַת רָמָה",
		TakeHint:    "בְּדִיקָה קְצָרָה שֶׁל מִלִּים מוּכָּרוֹת. אֶפְשָׁר לַעֲזֹב וְלַחֲזֹר. הַיּוֹם נִשְׁאַר פָּתוּחַ.",
		TodayResume: "חֲזָרָה לַבְּדִיקָה",
		Heading:     "בוא נבדוק מה אתה כבר יודע",
		Why:         "כדי שלא תבזבז זמן על מילים שאתה כבר מכיר.",
		BulletSit:   "סבבים קצרים של 20 כרטיסים, בערך 3 דקות כל אחד. עד כ־100 כרטיסים, אפשר לאורך כמה ימים.",
		BulletFlip:  "לכל מילה: נסה להיזכר, הפוך את הכרטיס, ודרג כמה טוב ידעת.",
		BulletScore: "אין ציון. כל דירוג הוא כבר החזרה הראשונה שלך על המילה, ואפשר לעצור בכל רגע.",
		Start:       "בוא נתחיל",
		Skip:        "לדלג ולהתחיל מההתחלה",
		Welcome:     "ברוך שובך",
		ResumeTitle: "נמשיך מאיפה שעצרת",
		KeepGoing:   "להמשיך",
		FinishNow:   "לסיים עכשיו ולראות מאיפה מתחילים",
		Direction:   "עברית ← אנגלית",
		ShowMeaning: "להציג את הפירוש",
		Hint:        "אתה יודע מה זה אומר? נסה להיזכר, ואז הפוך את הכרטיס.",
		Flip:        "להפוך",
		Space:       "רווח",
		DontKnow:    "אני לא מכיר את המילה",
		Listen:      "השמעה",
		Slow:        "0.75×",
		HowWell:     "כמה טוב ידעת?",
		Again:       "שוב",
		Hard:        "קשה",
		Good:        "טוב",
		Easy:        "קל",
		AgainSub:    "לא ידעתי",
		HardSub:     "במאמץ",
		GoodSub:     "ידעתי",
		EasySub:     "מיד",
		Stop:        "לעצור כאן",
		ToastLead:   "נוסף לתרגול שלך",
		BreakTitle:  "עבודה יפה. זה מספיק להיום?",
		BreakBody:   "כל מה שדירגת נוסף לתרגול שלך ויחזור בהדרגה, כמה בכל יום.",
		DoneToday:   "סיימתי להיום",
		SoFar:       "עד עכשיו",
		Samples:     "כמה מילים מהסבב",
		AllDone:     "סיימת",
		Results:     "הנה איפה אתה עומד",
		StartOver:   "בעצם, להתחיל מההתחלה",
		Back:        "חזרה להיום",
		KnowThese:   "מוכר לך",
		StartHere:   "כאן מתחילים",
		StillAhead:  "עוד לפנינו",
		Covered:     "עברנו על זה",
		NoteChange:  "אפשר לשנות את זה בכל זמן בעמוד ההתקדמות.",
	}
}

// ResumeLine is the welcome-back sentence. done is sittings already finished.
func (c PlacementCopy) ResumeLine(done int) string {
	if c.Lang == "he" {
		switch {
		case done <= 0:
			return "הסבב הבא: 20 כרטיסים, בערך 3 דקות."
		case done == 1:
			return "עשית כבר סבב אחד. הסבב הבא: 20 כרטיסים, בערך 3 דקות."
		default:
			return fmt.Sprintf("עשית כבר %d סבבים. הסבב הבא: 20 כרטיסים, בערך 3 דקות.", done)
		}
	}
	switch {
	case done <= 0:
		return "You’re partway through a sitting. Next up: 20 cards, about 3 minutes."
	case done == 1:
		return "You’ve done one sitting so far. Next up: 20 cards, about 3 minutes."
	default:
		return fmt.Sprintf("You’ve done %d sittings so far. Next up: 20 cards, about 3 minutes.", done)
	}
}

// SittingKicker is "Sitting N done".
func (c PlacementCopy) SittingKicker(n int) string {
	if c.Lang == "he" {
		return fmt.Sprintf("סבב %d הושלם", n)
	}
	return fmt.Sprintf("Sitting %d done", n)
}

// CoveredLine names how many words this sitting rated.
func (c PlacementCopy) CoveredLine(n int) string {
	if c.Lang == "he" {
		return fmt.Sprintf("עברנו על %d מילים.", n)
	}
	return fmt.Sprintf("We covered %d words.", n)
}

// LeftNote is the "sittings left" line. left is the cap minus sittings done.
func (c PlacementCopy) LeftNote(left int) string {
	if left < 0 {
		left = 0
	}
	if c.Lang == "he" {
		return fmt.Sprintf("נמשיך בדיוק מכאן בפעם הבאה. נשארו עד %d סבבים, ואולי פחות.", left)
	}
	return fmt.Sprintf("Next time we’ll pick up right here. Up to %d sittings left, maybe fewer.", left)
}

// BarLabel is the sitting progress name. card is the 1-based card in the sitting.
func (c PlacementCopy) BarLabel(sitting, card int) string {
	if c.Lang == "he" {
		return fmt.Sprintf("סבב %d: כרטיס %d מתוך 20", sitting, card)
	}
	return fmt.Sprintf("Sitting %d: card %d of 20", sitting, card)
}

// AddedLine is the result sentence. n is the count, outside the ladder.
func (c PlacementCopy) AddedLine(n int) string {
	if c.Lang == "he" {
		return fmt.Sprintf("%d מילים נוספו לתרגול שלך. הן יחזרו בהדרגה, כמה בכל יום.", n)
	}
	return fmt.Sprintf("%d words added to your practice. They’ll come back a few at a time.", n)
}

// UnitLine is "Unit N · Lesson 1".
func (c PlacementCopy) UnitLine(unit int) string {
	if unit < 1 {
		unit = 1
	}
	if c.Lang == "he" {
		return fmt.Sprintf("יחידה %d · שיעור 1", unit)
	}
	return fmt.Sprintf("Unit %d · Lesson 1", unit)
}

// StartUnit is the primary result button.
func (c PlacementCopy) StartUnit(unit int) string {
	if unit < 1 {
		unit = 1
	}
	if c.Lang == "he" {
		return fmt.Sprintf("להתחיל ביחידה %d", unit)
	}
	return fmt.Sprintf("Start Unit %d", unit)
}

// PlacementWord is one dealt lexeme. Text is pointed NFC Hebrew.
type PlacementWord struct {
	ID       string
	Text     string
	Translit string
	Gloss    string
	Gender   string
	AudioURL string
}

// PlacementView is one render of /placement, full page or fragment.
type PlacementView struct {
	Page
	Copy       PlacementCopy
	Mode       string
	RunIDText  string
	Word       PlacementWord
	Toast      PlacementWord
	HasToast   bool
	Samples    []PlacementWord
	Ladder     []BandRow
	Compact    bool
	Sitting    int
	CardIndex  int
	DoneInSit  int
	Fill       string
	FillPrev   string
	Overall    string
	Added      int
	AddedText  string
	Unit       int
	UnitLine   string
	StartLabel string
	ResumeLine string
	BreakKick  string
	Covered    string
	LeftNote   string
	CourseDone bool
}

func fmtInt(n int) string {
	if n < 0 {
		n = 0
	}
	return strconv.Itoa(n)
}

// Enter is the view-transition token for this face.
func (v PlacementView) Enter() string {
	if v.Mode == PlaceModeBack {
		return "flip"
	}
	return "stack"
}

// Fraction is the sitting label, "7/20".
func (v PlacementView) Fraction() string {
	n := v.CardIndex
	if n < 1 {
		n = 1
	}
	if n > 20 {
		n = 20
	}
	return fmt.Sprintf("%d/20", n)
}

// BarName is the progressbar accessible name.
func (v PlacementView) BarName() string {
	return v.Copy.BarLabel(v.Sitting, v.CardIndex)
}

// GenderChip is the short mark, the full word, and whether to show it.
// English uses m/f. Hebrew uses ז׳/נ׳. The full word is the tooltip and the
// accessible name.
func GenderChip(lang, gender string) (short, full string, ok bool) {
	switch gender {
	case AddressMasculine:
		if lang == "he" {
			return "ז׳", "זכר", true
		}
		return "m", "masculine", true
	case AddressFeminine:
		if lang == "he" {
			return "נ׳", "נקבה", true
		}
		return "f", "feminine", true
	default:
		return "", "", false
	}
}

// BandRow is one rung of the five-band ladder. State is solid, here, ahead, or seen.
type BandRow struct {
	Title string
	Units string
	State string
	Label string
}

type bandInfo struct {
	titleEN, unitsEN, titleHE, unitsHE string
	unit                               int
}

var courseBands = [5]bandInfo{
	{"First words", "Unit 1", "מילים ראשונות", "יחידה 1", 1},
	{"Home and kitchen", "Units 2–3", "בבית ובמטבח", "יחידות 2–3", 2},
	{"Out in town", "Units 4–5", "בעיר", "יחידות 4–5", 4},
	{"Daily life and work", "Units 6–7", "יום־יום ועבודה", "יחידות 6–7", 6},
	{"Stories and opinions", "Units 8–10", "סיפורים ודעות", "יחידות 8–10", 8},
}

// UnitForBand maps a finished-band index to the first unit of the next band.
// level is the highest completed band, or −1. The returned unit is where
// the learner starts. All five bands completed stays on unit 8.
func UnitForBand(level int) (unit int, courseDone bool) {
	if level >= 4 {
		return courseBands[4].unit, true
	}
	start := 0
	if level >= 0 {
		start = level + 1
	}
	return courseBands[start].unit, false
}

// ResultLadder is the verdict ladder. No scores. Highest band first.
func ResultLadder(copy PlacementCopy, level int) []BandRow {
	state := [5]string{}
	for i := range state {
		state[i] = "ahead"
	}
	if level >= 4 {
		for i := range state {
			state[i] = "solid"
		}
	} else {
		start := 0
		if level >= 0 {
			start = level + 1
		}
		for i := 0; i < start; i++ {
			state[i] = "solid"
		}
		state[start] = "here"
	}
	return ladderRows(copy, state[:])
}

// ProgressLadder is the "so far" ladder. seen marks bands with any rating.
func ProgressLadder(copy PlacementCopy, seen [5]bool) []BandRow {
	state := [5]string{}
	for i := range state {
		if seen[i] {
			state[i] = "seen"
		} else {
			state[i] = "ahead"
		}
	}
	return ladderRows(copy, state[:])
}

func ladderRows(copy PlacementCopy, state []string) []BandRow {
	rows := make([]BandRow, 0, 5)
	for i := 4; i >= 0; i-- {
		info := courseBands[i]
		title, units := info.titleEN, info.unitsEN
		if copy.Lang == "he" {
			title, units = info.titleHE, info.unitsHE
		}
		rows = append(rows, BandRow{
			Title: title,
			Units: units,
			State: state[i],
			Label: ladderLabel(copy, state[i]),
		})
	}
	return rows
}

func ladderLabel(copy PlacementCopy, state string) string {
	switch state {
	case "solid":
		return copy.KnowThese
	case "here":
		return copy.StartHere
	case "seen":
		return copy.Covered
	default:
		return copy.StillAhead
	}
}
