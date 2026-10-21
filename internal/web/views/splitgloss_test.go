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
