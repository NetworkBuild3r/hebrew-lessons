package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestPickPrefersTheEverydayWord(t *testing.T) {
	rows := buildRows(t, "אם\t1\nעם\t2\n", "")
	if rows[0][5] != "if" || rows[0][6] != "conjunction" {
		t.Fatalf("אם = %s %s, want if conjunction", rows[0][5], rows[0][6])
	}
	if rows[1][5] != "with" || rows[1][6] != "preposition" {
		t.Fatalf("עם = %s %s, want with preposition", rows[1][5], rows[1][6])
	}
}

func TestGlossDropsTheExtraWords(t *testing.T) {
	rows := buildRows(t, "ידע\t15\n", "")
	if rows[0][5] != "to know" {
		t.Fatalf("gloss = %q, want to know", rows[0][5])
	}
}

func TestDefectiveSenseIsSkipped(t *testing.T) {
	rows := buildRows(t, "כול\t14\n", "")
	if strings.HasPrefix(rows[0][5], "excessive") {
		t.Fatalf("gloss = %q", rows[0][5])
	}
	if rows[0][5] != "totality" {
		t.Fatalf("gloss = %q, want totality", rows[0][5])
	}
}

func TestPaalRootAndPielDraft(t *testing.T) {
	rows := buildRows(t, "סגר\t1\nפעל\t2\n", "")
	if rows[0][8] != "ס־ג־ר" || rows[0][9] != "paal" || rows[0][14] != "review.missing" {
		t.Fatalf("סגר root %s pattern %s check %s", rows[0][8], rows[0][9], rows[0][14])
	}
	if rows[1][14] != "entry.verb" {
		t.Fatalf("פעל check = %s, want entry.verb", rows[1][14])
	}
}

func TestNameIsNotAWord(t *testing.T) {
	rows := buildRows(t, "תנך\t1\n", "")
	if rows[0][14] != "entry.nodict" {
		t.Fatalf("check = %s, want entry.nodict", rows[0][14])
	}
}

func TestFixSplitPrefixAndDrop(t *testing.T) {
	dict := strings.Join([]string{
		`{"word":"הוא","pos":"pron","senses":[{"glosses":["he"]}],"head_templates":[{"name":"head","args":{"wv":"הוּא","tr":"hu"}}]}`,
		`{"word":"ה","pos":"article","senses":[{"glosses":["the"]}],"head_templates":[{"name":"head","args":{"wv":"הַ","tr":"ha"}}]}`,
		`{"word":"טוב","pos":"adv","senses":[{"glosses":["well"]}],"head_templates":[{"name":"head","args":{"wv":"טוֹב","tr":"tov"}}]}`,
		`{"word":"והיי","pos":"intj","senses":[{"glosses":["hey"]}],"head_templates":[{"name":"head","args":{"wv":"הֵי","tr":"hey"}}]}`,
		`{"word":"אב","pos":"noun","senses":[{"glosses":["father"]}],"head_templates":[{"name":"head","args":{"wv":"אָב","tr":"av","g":"m"}}]}`,
		`{"word":"אבא","pos":"noun","senses":[{"glosses":["dad"]}],"head_templates":[{"name":"head","args":{"wv":"אַבָּא","tr":"aba","g":"m"}}]}`,
	}, "\n")
	list := "הוא\t1\nה\t2\nטוב\t3\nוהיי\t4\nאב\t5\nאבא\t6\n"
	fixes := strings.Join([]string{
		"rank\tlemma\taction\tpointed\ttranslit\tgloss\tpos\tgender\troot\tbinyan\treason\treviewed_by\treviewed_on",
		"1\tהוא\tsplit\tאֲנִי\tani\tI\tpronoun\t\t\t\tthe parser folded the pronouns\t\t",
		"1\tהוא\tsplit\tהוּא\thu\the\tpronoun\t\t\t\tthe parser folded the pronouns\t\t",
		"2\tה\tprefix\tהַ\tha\tthe\tprefix\t\t\t\tthe article\t\t",
		"3\tטוב\tfix\tטוֹב\ttov\tgood\tadjective\t\t\t\teveryday sense\t\t",
		"4\tוהיי\tdrop\t\t\t\t\t\t\t\tparser artifact\t\t",
		"5\tאב\tkeep\t\t\t\t\t\t\t\tchecked\tagent:test\t2026-11-12",
		"6\tאבא\tfix\tאָב\tav\tfather\tnoun\tm\t\t\tsame spelling as אב\tagent:test\t2026-11-12",
	}, "\n")
	lemmas, err := ParseLemmas(list)
	if err != nil {
		t.Fatal(err)
	}
	fx, err := ParseFixes(fixes)
	if err != nil {
		t.Fatal(err)
	}
	body, err := Build(strings.NewReader(dict), lemmas, fx)
	if err != nil {
		t.Fatal(err)
	}
	rows := parseRows(t, body)
	if len(rows) != 7 {
		t.Fatalf("rows = %d, want 7 (split adds one)", len(rows))
	}
	if rows[0][3] != "אֲנִי" || rows[0][14] != "review.missing" {
		t.Fatalf("first split = %s %s", rows[0][3], rows[0][14])
	}
	if rows[1][3] != "הוּא" || rows[1][14] != "review.missing" {
		t.Fatalf("second split = %s %s", rows[1][3], rows[1][14])
	}
	if rows[2][11] != "prefix" || rows[2][6] != "prefix" || rows[2][5] != "the" {
		t.Fatalf("prefix = kind %s pos %s gloss %s", rows[2][11], rows[2][6], rows[2][5])
	}
	if rows[3][5] != "good" || rows[3][6] != "adjective" || rows[3][15] != "fix" {
		t.Fatalf("fix = %s %s %s", rows[3][5], rows[3][6], rows[3][15])
	}
	if rows[4][13] != "dropped" {
		t.Fatalf("drop status = %s", rows[4][13])
	}
	if rows[5][13] != "ready" || rows[5][16] != "agent:test" {
		t.Fatalf("keep = %s by %s", rows[5][13], rows[5][16])
	}
	if rows[6][14] != "course.duplicate" {
		t.Fatalf("same spelling check = %s, want course.duplicate", rows[6][14])
	}
}

func TestBuildBytesAreStable(t *testing.T) {
	lemmas, err := ParseLemmas("אם\t1\nעם\t2\n")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("testdata/dict.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	a, err := Build(f, lemmas, nil)
	if err != nil {
		t.Fatal(err)
	}
	f2, err := os.Open("testdata/dict.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()
	b, err := Build(f2, lemmas, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("two builds differ")
	}
}

func TestRealFixFileParses(t *testing.T) {
	b, err := os.ReadFile("../../internal/speech/course_fixes.tsv")
	if err != nil {
		t.Fatal(err)
	}
	fixes, err := ParseFixes(string(b))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) < 20 {
		t.Fatalf("fixes = %d", len(fixes))
	}
	for _, f := range fixes {
		if f.Action == "drop" || f.Gloss == "you (f.)" {
			if f.ReviewedBy != "" || f.ReviewedOn != "" {
				t.Fatalf("rank %d %s is signed", f.Rank, f.Gloss)
			}
			continue
		}
		if f.ReviewedBy != "" && f.ReviewedOn == "" {
			t.Fatalf("rank %d signed with no date", f.Rank)
		}
	}
}

func buildRows(t *testing.T, list, fixes string) [][]string {
	t.Helper()
	f, err := os.Open("testdata/dict.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	lemmas, err := ParseLemmas(list)
	if err != nil {
		t.Fatal(err)
	}
	var fx []Fix
	if fixes != "" {
		fx, err = ParseFixes(fixes)
		if err != nil {
			t.Fatal(err)
		}
	}
	body, err := Build(f, lemmas, fx)
	if err != nil {
		t.Fatal(err)
	}
	return parseRows(t, body)
}

func parseRows(t *testing.T, body []byte) [][]string {
	t.Helper()
	lines := splitLines(string(body))
	if len(lines) < 2 {
		t.Fatalf("course file has %d lines", len(lines))
	}
	var out [][]string
	for i, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 17 {
			t.Fatalf("line %d has %d fields", i+2, len(fields))
		}
		out = append(out, fields)
	}
	return out
}
