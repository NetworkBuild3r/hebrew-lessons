// verified by Hebrew Tutor YYYY-MM-DD: pending
//
// Alefbet is reference data for the twenty-two Hebrew letters, the five
// final forms, and the dagesh and shin/sin variants. It is not lesson
// content. Example words stay on the confusion lookup. Pronunciation is
// modern Israeli. The name transliteration uses kh for ח and soft כ,
// ts for צ, and ' for a glottal stop.
package views

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// AlefbetEntry is one letter form.
// Name is the letter name with niqqud. Translit is that name in Latin.
// Sound is the plain-English reading. Short is the consonant used in a
// dagesh sentence such as "With the dot: p. Without: f." Cue is the
// tell-apart line for a confusable pair this letter belongs to, when
// there is one standard visual difference. Form marks the twenty-two
// letters and the five final forms.
type AlefbetEntry struct {
	Letter   string
	Name     string
	Translit string
	Sound    string
	Short    string
	Cue      string
	Form     bool
}

// Alefbet is every letter form the review card can name.
var Alefbet = []AlefbetEntry{
	{Letter: "א", Name: "אַלֶף", Translit: "alef", Sound: "silent (carries a vowel)", Short: "", Form: true},
	{Letter: "ב", Name: "בֵית", Translit: "vet", Sound: "v as in van", Short: "v", Form: true},
	{Letter: "ג", Name: "גִּימֶל", Translit: "gimel", Sound: "g as in go", Short: "", Form: true},
	{Letter: "ד", Name: "דָּלֶת", Translit: "dalet", Sound: "d as in dog", Short: "", Form: true},
	{Letter: "ה", Name: "הֵא", Translit: "he", Sound: "h as in hat", Short: "", Form: true},
	{Letter: "ו", Name: "וָו", Translit: "vav", Sound: "v as in van", Short: "", Form: true},
	{Letter: "ז", Name: "זַיִן", Translit: "zayin", Sound: "z as in zoo", Short: "", Form: true},
	{Letter: "ח", Name: "חֵית", Translit: "khet", Sound: "kh as in Bach", Short: "", Form: true},
	{Letter: "ט", Name: "טֵית", Translit: "tet", Sound: "t as in top", Short: "", Form: true},
	{Letter: "י", Name: "יוֹד", Translit: "yod", Sound: "y as in yes", Short: "", Form: true},
	{Letter: "כ", Name: "כַף", Translit: "khaf", Sound: "kh as in Bach", Short: "kh", Form: true},
	{Letter: "ל", Name: "לָמֶד", Translit: "lamed", Sound: "l as in let", Short: "", Form: true},
	{Letter: "מ", Name: "מֵם", Translit: "mem", Sound: "m as in mom", Short: "", Form: true},
	{Letter: "נ", Name: "נוּן", Translit: "nun", Sound: "n as in no", Short: "", Form: true},
	{Letter: "ס", Name: "סָמֶךְ", Translit: "samekh", Sound: "s as in sun", Short: "", Form: true},
	{Letter: "ע", Name: "עַיִן", Translit: "'ayin", Sound: "silent (carries a vowel)", Short: "", Form: true},
	{Letter: "פ", Name: "פֵא", Translit: "fe", Sound: "f as in fun", Short: "f", Form: true},
	{Letter: "צ", Name: "צַדִּי", Translit: "tsadi", Sound: "ts as in cats", Short: "", Form: true},
	{Letter: "ק", Name: "קוֹף", Translit: "kuf", Sound: "k as in kite", Short: "", Form: true},
	{Letter: "ר", Name: "רֵיש", Translit: "resh", Sound: "guttural r, as in the French rue", Short: "", Form: true},
	{Letter: "ש", Name: "שִׁין", Translit: "shin", Sound: "sh as in ship", Short: "", Form: true},
	{Letter: "ת", Name: "תָּו", Translit: "tav", Sound: "t as in top", Short: "", Form: true},
	{Letter: "ך", Name: "כַף סוֹפִית", Translit: "khaf sofit", Sound: "kh as in Bach", Short: "", Form: true},
	{Letter: "ם", Name: "מֵם סוֹפִית", Translit: "mem sofit", Sound: "m as in mom", Short: "", Form: true},
	{Letter: "ן", Name: "נוּן סוֹפִית", Translit: "nun sofit", Sound: "n as in no", Short: "", Form: true},
	{Letter: "ף", Name: "פֵא סוֹפִית", Translit: "fe sofit", Sound: "f as in fun", Short: "", Form: true},
	{Letter: "ץ", Name: "צַדִּי סוֹפִית", Translit: "tsadi sofit", Sound: "ts as in cats", Short: "", Form: true},
	{Letter: "בּ", Name: "בֵּית", Translit: "bet", Sound: "b as in boy", Short: "b", Form: false},
	{Letter: "כּ", Name: "כַּף", Translit: "kaf", Sound: "k as in kite", Short: "k", Form: false},
	{Letter: "פּ", Name: "פֵּא", Translit: "pe", Sound: "p as in pet", Short: "p", Form: false},
	{Letter: "שׁ", Name: "שִׁין", Translit: "shin", Sound: "sh as in ship", Short: "", Form: false},
	{Letter: "שׂ", Name: "שִׂין", Translit: "sin", Sound: "s as in sun", Short: "", Form: false},
}

type alefbetPair struct {
	letters [2]string
	cue     string
}

var alefbetPairs = []alefbetPair{
	{letters: [2]string{"ב", "כ"}, cue: "ב has a little tail at the bottom right; כ is round."},
	{letters: [2]string{"בּ", "כ"}, cue: "ב has a little tail at the bottom right; כ is round."},
	{letters: [2]string{"ב", "כּ"}, cue: "ב has a little tail at the bottom right; כ is round."},
	{letters: [2]string{"בּ", "כּ"}, cue: "ב has a little tail at the bottom right; כ is round."},
	{letters: [2]string{"ד", "ר"}, cue: "ד has a sharp corner and a foot; ר is round on top."},
	{letters: [2]string{"ה", "ח"}, cue: "ה is open at the upper left; ח is closed."},
	{letters: [2]string{"ו", "ז"}, cue: "ו is a straight line; ז has a bar across the top."},
	{letters: [2]string{"ו", "ן"}, cue: "ו stays on the line; ן drops below the line."},
	{letters: [2]string{"ט", "מ"}, cue: "ט is closed, with a foot inside; מ is open at the bottom."},
	{letters: [2]string{"ס", "ם"}, cue: "ס is round; ם is square."},
	{letters: [2]string{"ג", "נ"}, cue: "ג bends in the middle and kicks left; נ has a straight leg and a flat foot."},
	{letters: [2]string{"צ", "ע"}, cue: "צ has a yod-like tail on the right; ע is two strokes that meet."},
	{letters: [2]string{"פ", "כ"}, cue: "פ has a tail inside the curve; כ is a smooth curve."},
	{letters: [2]string{"פּ", "כ"}, cue: "פ has a tail inside the curve; כ is a smooth curve."},
	{letters: [2]string{"פ", "כּ"}, cue: "פ has a tail inside the curve; כ is a smooth curve."},
	{letters: [2]string{"פּ", "כּ"}, cue: "פ has a tail inside the curve; כ is a smooth curve."},
	{letters: [2]string{"ף", "ך"}, cue: "ף drops straight below the line; ך curves as it drops."},
	{letters: [2]string{"שׁ", "שׂ"}, cue: "שׁ has the dot on the upper right; שׂ has the dot on the upper left."},
	{letters: [2]string{"מ", "ם"}, cue: "מ is open at the bottom left; ם is closed."},
	{letters: [2]string{"נ", "ן"}, cue: "נ sits on the line; ן drops below the line."},
	{letters: [2]string{"כ", "ך"}, cue: "כ sits on the line; ך drops below the line."},
	{letters: [2]string{"כּ", "ך"}, cue: "כ sits on the line; ך drops below the line."},
	{letters: [2]string{"פ", "ף"}, cue: "פ sits on the line; ף drops below the line."},
	{letters: [2]string{"פּ", "ף"}, cue: "פ sits on the line; ף drops below the line."},
	{letters: [2]string{"צ", "ץ"}, cue: "צ sits on the line; ץ drops below the line."},
}

var alefbetBy map[string]AlefbetEntry

func init() {
	alefbetBy = make(map[string]AlefbetEntry, len(Alefbet))
	for i := range Alefbet {
		Alefbet[i].Cue = alefbetCues(Alefbet[i].Letter)
		alefbetBy[norm.NFC.String(Alefbet[i].Letter)] = Alefbet[i]
	}
}

// AlefbetForms is the twenty-two letters and the five final forms.
func AlefbetForms() []AlefbetEntry {
	out := make([]AlefbetEntry, 0, 27)
	for _, e := range Alefbet {
		if e.Form {
			out = append(out, e)
		}
	}
	return out
}

// LookupAlefbet returns the entry for this exact letter, after NFC.
func LookupAlefbet(letter string) (AlefbetEntry, bool) {
	e, ok := alefbetBy[norm.NFC.String(letter)]
	return e, ok
}

// AlefbetTellApart is the cue when parts are one confusable pair.
// A single letter, or a pair with no standard visual difference, is empty.
func AlefbetTellApart(parts []string) string {
	if len(parts) != 2 {
		return ""
	}
	a, b := norm.NFC.String(parts[0]), norm.NFC.String(parts[1])
	for _, pair := range alefbetPairs {
		x, y := norm.NFC.String(pair.letters[0]), norm.NFC.String(pair.letters[1])
		if (a == x && b == y) || (a == y && b == x) {
			return pair.cue
		}
	}
	return ""
}

func alefbetCues(letter string) string {
	key := norm.NFC.String(letter)
	var cues []string
	seen := map[string]bool{}
	for _, pair := range alefbetPairs {
		x, y := norm.NFC.String(pair.letters[0]), norm.NFC.String(pair.letters[1])
		if (key != x && key != y) || seen[pair.cue] {
			continue
		}
		seen[pair.cue] = true
		cues = append(cues, pair.cue)
	}
	return strings.Join(cues, " / ")
}

// LetterDageshNote is the with-dot sentence for a letter pair.
// A stored confusion sound wins. A missing one comes from the table's
// short consonant, so פּ/פ is "With the dot: p. Without: f." even when
// the lexicon has no reading.
func LetterDageshNote(parts, stored []string) string {
	sides := make([]DrillSide, len(parts))
	for i, part := range parts {
		sides[i].Hebrew = part
		sound := ""
		if i < len(stored) {
			sound = stored[i]
		}
		if sound == "" {
			if e, ok := LookupAlefbet(part); ok {
				sound = e.Short
			}
		}
		sides[i].Sound = sound
	}
	return DageshNote(sides)
}
