package views

import "testing"

func TestSplitGloss(t *testing.T) {
	cases := []struct{ in, meaning, note, lemma string }{
		{"choose(s) (m.sg. present of לִבְחוֹר)", "choose(s)", "m.sg. present", "לִבְחוֹר"},
		{"live, reside (I/he lives; m.sg. present of לָגוּר)", "live, reside", "I/he lives; m.sg. present", "לָגוּר"},
		{"work(s) (m.sg. present of לַעֲבוֹד)", "work(s)", "m.sg. present", "לַעֲבוֹד"},
		{"hello", "hello", "", ""},
		{"work(s)", "work(s)", "", ""},
		{"(m.sg. present of לַעֲבוֹד)", "(m.sg. present of לַעֲבוֹד)", "", ""},
		{"", "", "", ""},
	}
	for _, c := range cases {
		meaning, note, lemma := SplitGloss(c.in)
		if meaning != c.meaning || note != c.note || lemma != c.lemma {
			t.Errorf("SplitGloss(%q) = %q, %q, %q; want %q, %q, %q", c.in, meaning, note, lemma, c.meaning, c.note, c.lemma)
		}
	}
}

func TestCardGlossSeparatesAPhraseFromTheMeaning(t *testing.T) {
	meaning, phraseHe, phraseEn, note, lemma := CardGloss("to me, for me (לָמָּה לִי = why should I)")
	if meaning != "to me · for me" || phraseHe != "לָמָּה לִי…" || phraseEn != "Why should I…" || note != "" || lemma != "" {
		t.Fatalf("phrase gloss = %q %q %q %q %q", meaning, phraseHe, phraseEn, note, lemma)
	}
	meaning, phraseHe, phraseEn, note, lemma = CardGloss("to me, for me (lama li = why should I)")
	if meaning != "to me · for me" || phraseHe != "" || phraseEn != "Why should I…" || note != "lama li" {
		t.Fatalf("latin phrase = %q %q %q %q", meaning, phraseHe, phraseEn, note)
	}
	meaning, _, _, note, lemma = CardGloss("choose(s) (m.sg. present of לִבְחוֹר)")
	if meaning != "choose(s)" || note != "m.sg. present" || lemma != "לִבְחוֹר" {
		t.Fatalf("form gloss = %q %q %q", meaning, note, lemma)
	}
	if got, _, _, _, _ := CardGloss("why"); got != "why" {
		t.Fatalf("why = %q", got)
	}
}
