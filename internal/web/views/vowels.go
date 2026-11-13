package views

import "github.com/NetworkBuild3r/hebrew-lessons/internal/units"

// Vowel is one vowel mark and how it sounds. Example is a word the app already
// knows that carries the mark.
type Vowel struct {
	Mark         string // The mark on its own, to show beside the name.
	Name         string // Its name, pointed.
	Translit     string // The name in Latin letters.
	Sound        string // What it sounds like, in plain English.
	Example      string // A pointed word that carries the mark.
	ExampleGloss string
}

// VowelMarks are the marks the app shows on a new word, in the order Hebrew
// teaching usually takes them.
var VowelMarks = []Vowel{
	{Mark: "\u05B7", Name: "פַּתָּח", Translit: "pataḥ", Sound: "a as in father", Example: units.Normalize("רַק"), ExampleGloss: "only"},
	{Mark: "\u05B8", Name: "קָמָץ", Translit: "qamats", Sound: "a as in father, and o in some words", Example: units.Normalize("הָיָה"), ExampleGloss: "to be"},
	{Mark: "\u05B5", Name: "צֵירֵי", Translit: "tsere", Sound: "e as in bed", Example: units.Normalize("הֵם"), ExampleGloss: "they"},
	{Mark: "\u05B6", Name: "סֶגּוֹל", Translit: "segol", Sound: "e as in bed", Example: units.Normalize("אֶת"), ExampleGloss: "object marker"},
	{Mark: "\u05B4", Name: "חִירִיק", Translit: "ḥiriq", Sound: "i as in machine", Example: units.Normalize("אִם"), ExampleGloss: "if"},
	{Mark: "\u05B9", Name: "חוֹלָם", Translit: "ḥolam", Sound: "o as in more", Example: units.Normalize("לֹא"), ExampleGloss: "not"},
	{Mark: "\u05BB", Name: "קֻבּוּץ", Translit: "qubuts", Sound: "u as in rule", Example: units.Normalize("שֻׁלְחָן"), ExampleGloss: "table"},
	{Mark: "\u05B0", Name: "שְׁוָא", Translit: "sheva", Sound: "a very short e, or nothing", Example: units.Normalize("כְּמוֹ"), ExampleGloss: "like"},
	{Mark: "\u05BC", Name: "דָּגֵשׁ", Translit: "dagesh", Sound: "doubles the letter, or hardens ב, כ, and פ", Example: units.Normalize("אַתֶּם"), ExampleGloss: "you"},
}
