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

func TestGoldenTrim(t *testing.T) {
	n := 0
	scanJSONL(t, "trim.jsonl", func(name string, cols []string) {
		in, out := unquote(t, name, cols[0]), unquote(t, name, cols[1])
		if got := Trim(in); got != out {
			t.Errorf("%s\ntrim got  %q\nwant %q", name, got, out)
		}
		n++
	})
	// U+0000..U+FFFF in four positions, plus eight astral characters in four
	// positions, plus the multi-character phrases.
	if n < 0x10000*4 {
		t.Fatalf("trim golden has %d rows, want at least %d", n, 0x10000*4)
	}
}

func TestGoldenNamedTrim(t *testing.T) {
	want := []string{
		"U+FEFF", "U+0085", "U+180E", "U+200B", "U+2028", "U+2029", "U+3000", "U+00A0",
	}
	seen := map[string]int{}
	scanJSONL(t, "named-trim.jsonl", func(name string, cols []string) {
		in, out := unquote(t, name, cols[0]), unquote(t, name, cols[1])
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
	scanJSONL(t, "named-trim.jsonl", func(name string, cols []string) {
		if name == "U+FEFF-only" || name == "U+0085-only" {
			rows[name] = [2]string{unquote(t, name, cols[0]), unquote(t, name, cols[1])}
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

func TestGoldenLengths(t *testing.T) {
	n := 0
	scanJSONL(t, "lengths.jsonl", func(name string, cols []string) {
		in := unquote(t, name, cols[0])
		cp, err1 := strconv.Atoi(cols[1])
		u16, err2 := strconv.Atoi(cols[2])
		if err1 != nil || err2 != nil {
			t.Fatalf("%s lengths: %q %q", name, cols[1], cols[2])
		}
		if CodePoints(in) != cp || UTF16Len(in) != u16 {
			t.Errorf("%s code points %d utf16 %d, golden %d %d", name, CodePoints(in), UTF16Len(in), cp, u16)
		}
		n++
	})
	if n < 0x10000 {
		t.Fatalf("lengths golden has %d rows", n)
	}
}

func TestGoldenLower(t *testing.T) {
	n := 0
	scanJSONL(t, "lower.jsonl", func(name string, cols []string) {
		in, out := unquote(t, name, cols[0]), unquote(t, name, cols[1])
		if got := ToLower(in); got != out {
			t.Errorf("%s\ntoLower got  %q\nwant %q", name, got, out)
		}
		n++
	})
	if n < 0x10000 {
		t.Fatalf("lower golden has %d rows", n)
	}
}

func TestGoldenNFC(t *testing.T) {
	n := 0
	scanJSONL(t, "nfc.jsonl", func(name string, cols []string) {
		in, out := unquote(t, name, cols[0]), unquote(t, name, cols[1])
		if got := NFC(in); got != out {
			t.Errorf("%s\nNFC got  %q\nwant %q", name, got, out)
		}
		n++
	})
	if n < 0x10000 {
		t.Fatalf("nfc golden has %d rows", n)
	}
}

func TestGoldenCollapse(t *testing.T) {
	n := 0
	scanJSONL(t, "collapse.jsonl", func(name string, cols []string) {
		in, out := unquote(t, name, cols[0]), unquote(t, name, cols[1])
		if got := CollapseSpace(in); got != out {
			t.Errorf("%s\ncollapse got  %q\nwant %q", name, got, out)
		}
		n++
	})
	// Two rows per BMP character, plus astral and phrases.
	if n < 0x10000*2 {
		t.Fatalf("collapse golden has %d rows", n)
	}
}

func scanJSONL(t *testing.T, name string, fn func(name string, cols []string)) {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "testdata", "golden", "jsstr", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 3 {
			t.Fatalf("%s:%d: %q", name, lineNo, line)
		}
		fn(cols[0], cols[1:])
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
