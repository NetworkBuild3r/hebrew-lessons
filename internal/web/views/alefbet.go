// verified by Hebrew Tutor 2026-10-20
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
	{Letter: "א", Name: "אָלֶף", Translit: "alef", Sound: "silent (carries a vowel)", Short: "", Form: true},
	{Letter: "ב", Name: "וֵית", Translit: "vet", Sound: "v as in van", Short: "v", Form: true},
	{Letter: "ג", Name: "גִּימֶל", Translit: "gimel", Sound: "g as in go", Short: "", Form: true},
	{Letter: "ד", Name: "דָּלֶת", Translit: "dalet", Sound: "d as in dog", Short: "", Form: true},
	{Letter: "ה", Name: "הֵא", Translit: "he", Sound: "h as in hat", Short: "", Form: true},
	{Letter: "ו", Name: "וָו", Translit: "vav", Sound: "v as in van; also the vowel letter for o (וֹ) and u (וּ)", Short: "", Form: true},
	{Letter: "ז", Name: "זַיִן", Translit: "zayin", Sound: "z as in zoo", Short: "", Form: true},
	{Letter: "ח", Name: "חֵית", Translit: "khet", Sound: "kh as in Bach", Short: "", Form: true},
	{Letter: "ט", Name: "טֵית", Translit: "tet", Sound: "t as in top", Short: "", Form: true},
	{Letter: "י", Name: "יוֹד", Translit: "yod", Sound: "y as in yes; also the vowel letter for i (and sometimes e)", Short: "", Form: true},
	{Letter: "כ", Name: "כַף", Translit: "khaf", Sound: "kh as in Bach", Short: "kh", Form: true},
	{Letter: "ל", Name: "לָמֶד", Translit: "lamed", Sound: "l as in let", Short: "", Form: true},
	{Letter: "מ", Name: "מֵם", Translit: "mem", Sound: "m as in mom", Short: "", Form: true},
	{Letter: "נ", Name: "נוּן", Translit: "nun", Sound: "n as in no", Short: "", Form: true},
	{Letter: "ס", Name: "סָמֶךְ", Translit: "samekh", Sound: "s as in sun", Short: "", Form: true},
	{Letter: "ע", Name: "עַיִן", Translit: "ayin", Sound: "silent (carries a vowel)", Short: "", Form: true},
	{Letter: "פ", Name: "פֵא", Translit: "fe", Sound: "f as in fun", Short: "f", Form: true},
	{Letter: "צ", Name: tsadiName, Translit: "tsadi", Sound: "ts as in cats", Short: "", Form: true},
	{Letter: "ק", Name: "קוֹף", Translit: "kof", Sound: "k as in kite", Short: "", Form: true},
	{Letter: "ר", Name: "רֵישׁ", Translit: "resh", Sound: "guttural r, as in the French rue", Short: "", Form: true},
	{Letter: "ש", Name: "שִׁין", Translit: "shin", Sound: "sh as in ship (s when the dot is on the left: שׂ)", Short: "", Form: true},
	{Letter: "ת", Name: "תָּו", Translit: "tav", Sound: "t as in top", Short: "", Form: true},
	{Letter: "ך", Name: "כַף סוֹפִית", Translit: "khaf sofit", Sound: "kh as in Bach", Short: "", Form: true},
	{Letter: "ם", Name: "מֵם סוֹפִית", Translit: "mem sofit", Sound: "m as in mom", Short: "", Form: true},
	{Letter: "ן", Name: "נוּן סוֹפִית", Translit: "nun sofit", Sound: "n as in no", Short: "", Form: true},
	{Letter: "ף", Name: "פֵא סוֹפִית", Translit: "fe sofit", Sound: "f as in fun", Short: "", Form: true},
	{Letter: "ץ", Name: tsadiSofitName, Translit: "tsadi sofit", Sound: "ts as in cats", Short: "", Form: true},
	{Letter: "בּ", Name: "בֵּית", Translit: "bet", Sound: "b as in boy", Short: "b", Form: false},
	{Letter: "כּ", Name: "כַּף", Translit: "kaf", Sound: "k as in kite", Short: "k", Form: false},
	{Letter: "פּ", Name: "פֵּא", Translit: "pe", Sound: "p as in pet", Short: "p", Form: false},
	{Letter: "שׁ", Name: "שִׁין", Translit: "shin", Sound: "sh as in ship", Short: "", Form: false},
	{Letter: "שׂ", Name: "שִׂין", Translit: "sin", Sound: "s as in sun", Short: "", Form: false},
}

type alefbetPair struct {
	letters [2]string
	cue     string
	note    string
}

const (
	// tsadiName is צָדִי: qamats, dalet, hiriq (U+05B4), yod. The hiriq sits
	// under the dalet. tsadiSofitName is that name plus סוֹפִית, so the final
	// form cannot drop the mark on its own.
	tsadiName      = "\u05E6\u05B8\u05D3\u05B4\u05D9"
	tsadiSofitName = tsadiName + " \u05E1\u05D5\u05B9\u05E4\u05B4\u05D9\u05EA"

	cueBetKaf    = "ב (vet) has a little tail at the bottom right; כ (khaf) is round."
	cueDaletResh = "ד (dalet) has a sharp corner at the top right, where the roof sticks out past the leg; ר (resh) is rounded at the top."
	cueHeHet     = "ה (he) is open at the upper left; ח (khet) is closed."
	cueHetTav    = "ת (tav) has a little foot kicking out at the bottom left; ח (khet) has none."
	cueVavZayin  = "ו (vav) is a straight line; ז (zayin) has a bar across the top."
	cueVavNun    = "ו (vav) stays on the line; ן (nun sofit) drops below the line."
	cueTetMem    = "ט (tet) is round at the bottom and open at the top, with its right arm curling inward; מ (mem) has a flat bottom with a small gap at the bottom left."
	cueSamekhMem = "ס (samekh) is round; ם (mem sofit) is square."
	cueGimelNun  = "ג (gimel) stands on one leg, with a small foot kicking out to the left at the bottom; נ (nun) is narrow, with a flat base running left along the line."
	cueTsadiAyin = "צ (tsadi) has a small yod-shaped arm on the upper right and sits on a flat base; ע (ayin) is two arms that meet low on the left, with a tail sweeping down-left and no flat base."
	cuePeKaf     = "פ (fe) has a little curl hanging inside from the top; כ (khaf) is a plain open curve."
	cueFeKafSof  = "ך (khaf sofit) is a roof with a plain straight leg dropping below the line; ף (fe sofit) has the same drop but with a little curl hanging inside at the top."
	cueShinSin   = "שׁ (shin) has the dot on the upper right; שׂ (sin) has the dot on the upper left."
	cueMemSof    = "מ (mem) is open at the bottom left; ם (mem sofit) is closed."
	cueNunSof    = "נ (nun) sits on the line; ן (nun sofit) drops below the line."
	cueKafSof    = "כ (khaf) sits on the line; ך (khaf sofit) drops below the line."
	cuePeSof     = "פ (fe) sits on the line; ף (fe sofit) drops below the line."
	cueTsadiSof  = "צ (tsadi) sits on the line; ץ (tsadi sofit) drops below the line."
	// cueGimelPe is the dotted pair ג/פּ. cueGimelFe is the undotted pair ג/פ.
	// The shape of each sentence is its own; a dagesh rewrite does not turn one into the other.
	cueGimelPe  = "ג (gimel) stands on one leg, with a small foot kicking out to the left at the bottom. פּ (pe) curls back into itself, with a dot inside. Without the dot, פ is f, as in fun."
	cueGimelFe  = "ג (gimel) stands on one leg, with a small foot kicking out to the left at the bottom. פ (fe) curls back into itself, with a little curl hanging inside from the top. With a dot inside (פּ), it's p, as in pet."
	noteGimelPe = "At the end of a word it's ף, almost always f."
)

var alefbetPairs = []alefbetPair{
	{letters: [2]string{"ב", "כ"}, cue: cueBetKaf},
	{letters: [2]string{"בּ", "כ"}, cue: cueBetKaf},
	{letters: [2]string{"ב", "כּ"}, cue: cueBetKaf},
	{letters: [2]string{"בּ", "כּ"}, cue: cueBetKaf},
	{letters: [2]string{"ד", "ר"}, cue: cueDaletResh},
	{letters: [2]string{"ה", "ח"}, cue: cueHeHet},
	{letters: [2]string{"ח", "ת"}, cue: cueHetTav},
	{letters: [2]string{"ו", "ז"}, cue: cueVavZayin},
	{letters: [2]string{"ו", "ן"}, cue: cueVavNun},
	{letters: [2]string{"ט", "מ"}, cue: cueTetMem},
	{letters: [2]string{"ס", "ם"}, cue: cueSamekhMem},
	{letters: [2]string{"ג", "נ"}, cue: cueGimelNun},
	{letters: [2]string{"צ", "ע"}, cue: cueTsadiAyin},
	{letters: [2]string{"פ", "כ"}, cue: cuePeKaf},
	{letters: [2]string{"פּ", "כ"}, cue: cuePeKaf},
	{letters: [2]string{"פ", "כּ"}, cue: cuePeKaf},
	{letters: [2]string{"פּ", "כּ"}, cue: cuePeKaf},
	{letters: [2]string{"ף", "ך"}, cue: cueFeKafSof},
	{letters: [2]string{"שׁ", "שׂ"}, cue: cueShinSin},
	{letters: [2]string{"מ", "ם"}, cue: cueMemSof},
	{letters: [2]string{"נ", "ן"}, cue: cueNunSof},
	{letters: [2]string{"כ", "ך"}, cue: cueKafSof},
	{letters: [2]string{"כּ", "ך"}, cue: cueKafSof},
	{letters: [2]string{"פ", "ף"}, cue: cuePeSof},
	{letters: [2]string{"פּ", "ף"}, cue: cuePeSof},
	{letters: [2]string{"צ", "ץ"}, cue: cueTsadiSof},
	{letters: [2]string{"ג", "פ"}, cue: cueGimelFe, note: noteGimelPe},
	{letters: [2]string{"ג", "פּ"}, cue: cueGimelPe, note: noteGimelPe},
}

var alefbetBy map[string]AlefbetEntry

func init() {
	for i := range alefbetPairs {
		alefbetPairs[i].cue = dottedCue(alefbetPairs[i])
	}
	alefbetBy = make(map[string]AlefbetEntry, len(Alefbet))
	for i := range Alefbet {
		Alefbet[i].Cue = alefbetCues(Alefbet[i].Letter)
		alefbetBy[norm.NFC.String(Alefbet[i].Letter)] = Alefbet[i]
	}
}

// dottedCue keeps the signed-off shape and rewrites a label when that side
// of the pair is the dotted letter. בּ is bet, כּ is kaf, פּ is pe.
// Undotted sides stay vet, khaf, and fe. ג/פ is not produced this way.
func dottedCue(pair alefbetPair) string {
	cue := pair.cue
	for _, letter := range pair.letters {
		if !strings.ContainsRune(letter, '\u05BC') {
			continue
		}
		switch stripDagesh(norm.NFC.String(letter)) {
		case "ב":
			cue = strings.ReplaceAll(cue, "ב (vet)", "ב\u05BC (bet)")
		case "כ":
			cue = strings.ReplaceAll(cue, "כ (khaf)", "כ\u05BC (kaf)")
		case "פ":
			cue = strings.ReplaceAll(cue, "פ (fe)", "פ\u05BC (pe)")
		}
	}
	return cue
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
	if pair, ok := matchAlefbetPair(parts); ok {
		return pair.cue
	}
	return ""
}

// AlefbetPairNote is the secondary line under a pair cue.
// ג/פ and ג/פּ use it for the final pe. Every other pair is empty.
func AlefbetPairNote(parts []string) string {
	if pair, ok := matchAlefbetPair(parts); ok {
		return pair.note
	}
	return ""
}

func matchAlefbetPair(parts []string) (alefbetPair, bool) {
	if len(parts) != 2 {
		return alefbetPair{}, false
	}
	a, b := norm.NFC.String(parts[0]), norm.NFC.String(parts[1])
	for _, pair := range alefbetPairs {
		x, y := norm.NFC.String(pair.letters[0]), norm.NFC.String(pair.letters[1])
		if (a == x && b == y) || (a == y && b == x) {
			return pair, true
		}
	}
	return alefbetPair{}, false
}

// FirstAlefbetCue is the first tell-apart line joined onto a letter.
// Later lines are other pairs. A single-letter card shows only this one.
func FirstAlefbetCue(cue string) string {
	head, _, _ := strings.Cut(cue, " / ")
	return head
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

// DageshHardNote is the word-initial line for ב, כ, and פ.
// A single letter shows it with or without the dot. A pair shows it when
// the two sides are that letter with and without the dot.
func DageshHardNote(parts []string) string {
	const note = "At the start of a word, ב, כ, and פ are almost always hard (b, k, p)."
	if len(parts) == 1 {
		if begadkefat(parts[0]) {
			return note
		}
		return ""
	}
	if len(parts) != 2 || !begadkefat(parts[0]) || !begadkefat(parts[1]) {
		return ""
	}
	a := stripDagesh(norm.NFC.String(parts[0]))
	b := stripDagesh(norm.NFC.String(parts[1]))
	if a != b {
		return ""
	}
	aDot := strings.ContainsRune(parts[0], '\u05BC')
	bDot := strings.ContainsRune(parts[1], '\u05BC')
	if aDot == bDot {
		return ""
	}
	return note
}

func begadkefat(part string) bool {
	switch stripDagesh(norm.NFC.String(part)) {
	case "ב", "כ", "פ":
		return true
	default:
		return false
	}
}
