package jsstr

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestGoldenProps(t *testing.T) {
	word := "שָׁלוֹם"
	head, tail := splitFirst(word)
	wordCP, word16 := phraseLengths(t, "word")
	next := rune(0)
	astral := 0
	scanLines(t, "props.jsonl", func(cols []string) {
		if len(cols) != 5 {
			t.Fatalf("props row %q", cols)
		}
		start, end := parseSpan(t, cols[0])
		utf16n, err := strconv.Atoi(cols[1])
		if err != nil {
			t.Fatalf("%s utf16: %v", cols[0], err)
		}
		flags := cols[2]
		if flags != "-" && flags != "t" && flags != "s" && flags != "ts" {
			t.Fatalf("%s flags %q", cols[0], flags)
		}
		removes := strings.Contains(flags, "t")
		space := strings.Contains(flags, "s")
		if (cols[3] != "" || cols[4] != "") && start != end {
			t.Fatalf("%s records a case or NFC change across a range", cols[0])
		}
		var lower, nfc string
		if cols[3] != "" {
			lower = unquote(t, cols[0], cols[3])
		}
		if cols[4] != "" {
			nfc = unquote(t, cols[0], cols[4])
		}
		if start > 0xFFFF {
			astral++
		} else {
			if start != next {
				t.Fatalf("gap or overlap before %s (want U+%04X)", cols[0], next)
			}
			next = end + 1
		}
		for cp := start; cp <= end; cp++ {
			ch := fromCP(cp)
			if CodePoints(ch) != 1 {
				t.Errorf("U+%04X code points %d", cp, CodePoints(ch))
			}
			if UTF16Len(ch) != utf16n {
				t.Errorf("U+%04X utf16 %d, golden %d", cp, UTF16Len(ch), utf16n)
			}
			wantLower := ch
			if lower != "" {
				wantLower = lower
			}
			if got := ToLower(ch); got != wantLower {
				t.Errorf("U+%04X toLower %q, golden %q", cp, got, wantLower)
			}
			wantNFC := ch
			if nfc != "" {
				wantNFC = nfc
			}
			if got := NFC(ch); got != wantNFC {
				t.Errorf("U+%04X NFC %q, golden %q", cp, got, wantNFC)
			}
			pair := ch + ch
			between := "א" + ch + "ב"
			if space {
				if CollapseSpace(pair) != " " || CollapseSpace(between) != "א ב" {
					t.Errorf("U+%04X \\s collapse pair %q between %q", cp, CollapseSpace(pair), CollapseSpace(between))
				}
			} else if CollapseSpace(pair) != pair || CollapseSpace(between) != between {
				t.Errorf("U+%04X non-space collapsed pair %q between %q", cp, CollapseSpace(pair), CollapseSpace(between))
			}
			lead := ch + word
			trail := word + ch
			interior := head + ch + tail
			repeated := ch + ch + word + ch + ch
			checkTrim := func(name, in, kept string) {
				t.Helper()
				want := in
				if removes {
					want = kept
				}
				if got := Trim(in); got != want {
					t.Errorf("U+%04X %s trim %q, want %q", cp, name, got, want)
				}
			}
			checkTrim("lead", lead, word)
			checkTrim("trail", trail, word)
			checkTrim("interior", interior, interior)
			checkTrim("repeated", repeated, word)
			if CodePoints(lead) != 1+wordCP || UTF16Len(lead) != utf16n+word16 {
				t.Errorf("U+%04X lead lengths cp %d utf16 %d", cp, CodePoints(lead), UTF16Len(lead))
			}
			if CodePoints(repeated) != 4+wordCP || UTF16Len(repeated) != 4*utf16n+word16 {
				t.Errorf("U+%04X repeated lengths cp %d utf16 %d", cp, CodePoints(repeated), UTF16Len(repeated))
			}
		}
	})
	if next != 0x10000 {
		t.Fatalf("BMP table ended at U+%04X", next)
	}
	if astral != 8 {
		t.Fatalf("astral sample has %d rows, want 8", astral)
	}
}

func TestGoldenPhrases(t *testing.T) {
	n := 0
	scanLines(t, "phrases.jsonl", func(cols []string) {
		if len(cols) != 8 {
			t.Fatalf("phrase %q", cols)
		}
		name := cols[0]
		in := unquote(t, name, cols[1])
		cp, err1 := strconv.Atoi(cols[2])
		u16, err2 := strconv.Atoi(cols[3])
		if err1 != nil || err2 != nil {
			t.Fatalf("%s lengths", name)
		}
		if CodePoints(in) != cp || UTF16Len(in) != u16 {
			t.Errorf("%s code points %d utf16 %d, golden %d %d", name, CodePoints(in), UTF16Len(in), cp, u16)
		}
		if got, want := ToLower(in), unquote(t, name, cols[4]); got != want {
			t.Errorf("%s toLower %q, golden %q", name, got, want)
		}
		if got, want := NFC(in), unquote(t, name, cols[5]); got != want {
			t.Errorf("%s NFC %q, golden %q", name, got, want)
		}
		if got, want := Trim(in), unquote(t, name, cols[6]); got != want {
			t.Errorf("%s trim %q, golden %q", name, got, want)
		}
		if got, want := CollapseSpace(in), unquote(t, name, cols[7]); got != want {
			t.Errorf("%s collapse %q, golden %q", name, got, want)
		}
		n++
	})
	if n < 13 {
		t.Fatalf("phrases has %d rows", n)
	}
}

func TestGoldenNamedTrim(t *testing.T) {
	want := []string{
		"U+FEFF", "U+0085", "U+180E", "U+200B", "U+2028", "U+2029", "U+3000", "U+00A0",
	}
	seen := map[string]int{}
	scanLines(t, "named-trim.jsonl", func(cols []string) {
		if len(cols) != 3 {
			t.Fatalf("named %q", cols)
		}
		name := cols[0]
		in, out := unquote(t, name, cols[1]), unquote(t, name, cols[2])
		if got := Trim(in); got != out {
			t.Errorf("%s\ntrim got  %q\nwant %q", name, got, out)
		}
		for _, id := range want {
			if strings.HasPrefix(name, id+"-") {
				seen[id]++
			}
		}
	})
	for _, id := range want {
		// only, lead, trail, interior, repeated
		if seen[id] != 5 {
			t.Errorf("%s has %d named rows, want 5", id, seen[id])
		}
	}
}

// strings.TrimSpace removes U+0085 and keeps U+FEFF. String.prototype.trim does
// the opposite. The expected sides come from the Node golden, not from Go.
func TestTrimSpaceDiffersOnFEFFAndNEL(t *testing.T) {
	rows := map[string][2]string{}
	scanLines(t, "named-trim.jsonl", func(cols []string) {
		if cols[0] == "U+FEFF-only" || cols[0] == "U+0085-only" {
			rows[cols[0]] = [2]string{unquote(t, cols[0], cols[1]), unquote(t, cols[0], cols[2])}
		}
	})
	fef, ok := rows["U+FEFF-only"]
	if !ok {
		t.Fatal("missing U+FEFF-only")
	}
	if Trim(fef[0]) != fef[1] {
		t.Fatalf("U+FEFF trim = %q, golden %q", Trim(fef[0]), fef[1])
	}
	if strings.TrimSpace(fef[0]) == Trim(fef[0]) {
		t.Fatal("strings.TrimSpace matched String.prototype.trim on U+FEFF")
	}
	nel, ok := rows["U+0085-only"]
	if !ok {
		t.Fatal("missing U+0085-only")
	}
	if Trim(nel[0]) != nel[1] {
		t.Fatalf("U+0085 trim = %q, golden %q", Trim(nel[0]), nel[1])
	}
	if strings.TrimSpace(nel[0]) == Trim(nel[0]) {
		t.Fatal("strings.TrimSpace matched String.prototype.trim on U+0085")
	}
}

func phraseLengths(t *testing.T, name string) (int, int) {
	t.Helper()
	var cp, u16 int
	found := false
	scanLines(t, "phrases.jsonl", func(cols []string) {
		if cols[0] != name {
			return
		}
		var err1, err2 error
		cp, err1 = strconv.Atoi(cols[2])
		u16, err2 = strconv.Atoi(cols[3])
		if err1 != nil || err2 != nil {
			t.Fatalf("%s lengths", name)
		}
		found = true
	})
	if !found {
		t.Fatalf("missing phrase %s", name)
	}
	return cp, u16
}

func splitFirst(s string) (head, tail string) {
	_, size := Next(s, 0)
	return s[:size], s[size:]
}

func fromCP(cp rune) string {
	var b strings.Builder
	appendRune(&b, cp)
	return b.String()
}

func parseSpan(t *testing.T, s string) (rune, rune) {
	t.Helper()
	a, b, ok := strings.Cut(s, "-")
	start := parseHex(t, a)
	if !ok {
		return start, start
	}
	return start, parseHex(t, b)
}

func parseHex(t *testing.T, s string) rune {
	t.Helper()
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		t.Fatalf("hex %q: %v", s, err)
	}
	return rune(v)
}

func scanLines(t *testing.T, name string, fn func(cols []string)) {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "testdata", "golden", "jsstr", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fn(strings.Split(line, "\t"))
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
}

func unquote(t *testing.T, name, quoted string) string {
	t.Helper()
	s, err := unquoteJSON(quoted)
	if err != nil {
		t.Fatalf("%s unquote %q: %v", name, quoted, err)
	}
	return s
}

// unquoteJSON accepts a JSON string, including lone surrogates (\uD800), which
// strconv.Unquote rejects. The result uses WTF-8 for those surrogates.
func unquoteJSON(quoted string) (string, error) {
	if len(quoted) < 2 || quoted[0] != '"' || quoted[len(quoted)-1] != '"' {
		return "", strconv.ErrSyntax
	}
	var b strings.Builder
	s := quoted[1 : len(quoted)-1]
	for i := 0; i < len(s); {
		if s[i] != '\\' {
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 1 {
				return "", strconv.ErrSyntax
			}
			b.WriteString(s[i : i+size])
			i += size
			continue
		}
		if i+1 >= len(s) {
			return "", strconv.ErrSyntax
		}
		switch s[i+1] {
		case '"', '\\', '/':
			b.WriteByte(s[i+1])
			i += 2
		case 'b':
			b.WriteByte('\b')
			i += 2
		case 'f':
			b.WriteByte('\f')
			i += 2
		case 'n':
			b.WriteByte('\n')
			i += 2
		case 'r':
			b.WriteByte('\r')
			i += 2
		case 't':
			b.WriteByte('\t')
			i += 2
		case 'u':
			if i+6 > len(s) {
				return "", strconv.ErrSyntax
			}
			v, err := strconv.ParseUint(s[i+2:i+6], 16, 16)
			if err != nil {
				return "", err
			}
			appendRune(&b, rune(v))
			i += 6
		default:
			return "", strconv.ErrSyntax
		}
	}
	return b.String(), nil
}
