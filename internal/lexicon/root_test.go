package lexicon

import (
	"strings"
	"testing"
)

func TestPrepareRootAndBinyan(t *testing.T) {
	tr := "shama"
	cases := []struct {
		name   string
		root   string
		binyan string
		want   string
		bad    string
	}{
		{name: "hyphen and shin dot", root: "שׁ-מ-ע", binyan: "paal", want: "שׁ־מ־ע"},
		{name: "maqaf already", root: "כ־ת־ב", binyan: "nifal", want: "כ־ת־ב"},
		{name: "spaces and niqqud dropped", root: "קָ ט ל", binyan: "piel", want: "ק־ט־ל"},
		{name: "binyan case folded", root: "ש-מ-ר", binyan: "Hifil", want: "ש־מ־ר"},
		{name: "two letters", root: "ב-ן", binyan: "pual", want: "ב־ן"},
		{name: "one letter", root: "ב", bad: "/entries/0/root"},
		{name: "five letters", root: "א-ב-ג-ד-ה", bad: "/entries/0/root"},
		{name: "latin letter", root: "abc", bad: "/entries/0/root"},
		{name: "unknown binyan", root: "ש-מ-ע", binyan: "qal", bad: "/entries/0/binyan"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, bad := Prepare([]Input{{
				TextNFC: "שָׁמַע", Gloss: "he heard", Translit: &tr, Root: &tc.root, Binyan: &tc.binyan,
			}})
			if tc.bad != "" {
				if rows != nil || bad == nil || !strings.Contains(bad.Error(), tc.bad) {
					t.Fatalf("rows %v err %v", rows, bad)
				}
				return
			}
			if bad != nil || len(rows) != 1 || rows[0].Root == nil || *rows[0].Root != tc.want {
				t.Fatalf("rows %+v err %v", rows, bad)
			}
		})
	}

	rows, bad := Prepare([]Input{{TextNFC: "שָׁמַע", Gloss: "he heard", Translit: &tr}})
	if bad != nil || rows[0].Root != nil || rows[0].Binyan != nil {
		t.Fatalf("omitted should stay nil: %+v %v", rows, bad)
	}
	empty := ""
	rows, bad = Prepare([]Input{{TextNFC: "שָׁמַע", Gloss: "he heard", Translit: &tr, Root: &empty, Binyan: &empty}})
	if bad != nil || rows[0].Root == nil || *rows[0].Root != "" || *rows[0].Binyan != "" {
		t.Fatalf("empty should clear: %+v %v", rows, bad)
	}
}
