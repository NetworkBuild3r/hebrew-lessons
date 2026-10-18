package ucd

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestUnicodeVersionMatchesGolden(t *testing.T) {
	meta := readMeta(t)
	if UnicodeVersion != meta.Unicode {
		t.Fatalf("ucd.UnicodeVersion is %s, Node golden meta says %s", UnicodeVersion, meta.Unicode)
	}
	if meta.Unicode != "16.0" {
		t.Fatalf("golden unicode %s, Node 22.14 reports 16.0", meta.Unicode)
	}
}

func TestPropsMatchNode(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "..", "testdata", "golden", "ucd", "props.rle"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	bits := map[string]uint32{}
	var runs int
	var covered uint32
	next := uint32(0)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			fields := strings.Fields(line)
			if len(fields) == 4 && fields[1] == "bit" {
				n, err := strconv.ParseUint(fields[3], 10, 32)
				if err != nil {
					t.Fatalf("bit %q", line)
				}
				bits[fields[2]] = uint32(n)
			}
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			t.Fatalf("row %q", line)
		}
		lo, hi := parseRun(t, parts[0])
		mask64, err := strconv.ParseUint(parts[1], 16, 32)
		if err != nil {
			t.Fatal(err)
		}
		mask := uint32(mask64)
		if lo != next {
			t.Fatalf("gap before %s (have U+%X)", parts[0], next)
		}
		fails := 0
		for cp := lo; cp <= hi; cp++ {
			r := rune(cp)
			for name, bit := range bits {
				got := Is(name, r)
				want := mask&bit != 0
				if got != want {
					t.Errorf("U+%04X %s got %v want %v", cp, name, got, want)
					fails++
					if fails > 20 {
						t.Fatal("too many property mismatches")
					}
				}
			}
			if IsEdgeMark(r) != (IsPunct(r) || IsSymbol(r) || IsFormat(r)) {
				t.Errorf("U+%04X edge mark disagrees with P|S|Cf", cp)
			}
		}
		next = hi + 1
		covered += hi - lo + 1
		runs++
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if next != 0x110000 {
		t.Fatalf("props cover through U+%X", next-1)
	}
	if covered != 0x110000 {
		t.Fatalf("covered %d code points", covered)
	}
	for _, name := range []string{"L", "M", "N", "P", "S", "Cf", "Z", "C", "js", "ws"} {
		if bits[name] == 0 {
			t.Errorf("golden has no %s bit", name)
		}
	}
	if runs < 10 {
		t.Fatalf("props.rle has %d runs", runs)
	}
}

type goldenMeta struct {
	Unicode string `json:"unicode"`
	Node    string `json:"node"`
}

func readMeta(t *testing.T) goldenMeta {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "ucd", "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	var meta goldenMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	return meta
}

func parseRun(t *testing.T, s string) (uint32, uint32) {
	t.Helper()
	a, b, ok := strings.Cut(s, "..")
	lo := parseHex32(t, a)
	if !ok {
		return lo, lo
	}
	return lo, parseHex32(t, b)
}

func parseHex32(t *testing.T, s string) uint32 {
	t.Helper()
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		t.Fatalf("hex %q: %v", s, err)
	}
	return uint32(v)
}
