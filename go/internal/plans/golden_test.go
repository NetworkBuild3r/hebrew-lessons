package plans

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// expandRepeats restores runs that gen.js stored as @@REPEAT:hex:count@@.
var repeatMarker = regexp.MustCompile(`@@REPEAT:([0-9a-fA-F]+):(\d+)@@`)

func expandRepeats(s string) string {
	return repeatMarker.ReplaceAllStringFunc(s, func(m string) string {
		parts := repeatMarker.FindStringSubmatch(m)
		cp, err1 := strconv.ParseInt(parts[1], 16, 32)
		n, err2 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil {
			return m
		}
		return strings.Repeat(string(rune(cp)), n)
	})
}

func TestGoldenValidate(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "golden")
	covRaw, err := os.ReadFile(filepath.Join(root, "coverage.json"))
	if err != nil {
		t.Fatal(err)
	}
	cov, err := Parse(string(covRaw))
	if err != nil {
		t.Fatal(err)
	}
	wantN, ok := cov.Get("cases")
	if !ok || wantN.K != KindNum {
		t.Fatal("coverage.json has no cases count")
	}
	f, err := os.Open(filepath.Join(root, "validate.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	n := 0
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		n++
		doc, err := Parse(line)
		if err != nil {
			t.Fatalf("line %d: %v", n, err)
		}
		id, _ := doc.StrKey("id")
		t.Run(id, func(t *testing.T) {
			rawIn, ok := doc.StrKey("input")
			if !ok {
				t.Fatal("no input")
			}
			input := expandRepeats(rawIn)
			optVal, ok := doc.Get("options")
			if !ok {
				t.Fatal("no options")
			}
			opt := optionsFrom(optVal)
			mode, _ := optVal.StrKey("inputMode")
			var plan any = input
			if mode == "object" {
				parsed, err := Parse(input)
				if err != nil {
					t.Fatal(err)
				}
				plan = parsed
			}
			got := ValidatePlan(plan, opt).JSON()
			wantVal, ok := doc.Get("want")
			if !ok {
				t.Fatal("no want")
			}
			want := Stringify(wantVal)
			if got != want {
				t.Fatalf("validatePlan\n%s", snippet(got, want))
			}
			parsed, err := Parse(input)
			var hebrew []Finding
			if err == nil && parsed.IsObject() {
				hebrew = CheckHebrew(parsed, opt)
			} else {
				hebrew = []Finding{}
			}
			gotH := Stringify(findingsValue(hebrew))
			heb, ok := doc.Get("hebrew")
			if !ok {
				t.Fatal("no hebrew")
			}
			wantH := Stringify(heb)
			if gotH != wantH {
				t.Fatalf("checkHebrew\n%s", snippet(gotH, wantH))
			}
		})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if n != int(wantN.N) {
		t.Fatalf("validate.jsonl has %d cases, coverage.json says %d", n, int(wantN.N))
	}
}

func TestGoldenJSON(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "json.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Parse(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	for i, item := range doc.A {
		input, _ := item.StrKey("input")
		canonical, _ := item.StrKey("canonical")
		parsed, err := Parse(input)
		if err != nil {
			t.Errorf("case %d parse %q: %v", i, input, err)
			continue
		}
		if got := Stringify(parsed); got != canonical {
			t.Errorf("case %d\ninput %s\ngot   %s\nwant  %s", i, input, got, canonical)
		}
	}
}

func snippet(got, want string) string {
	n := len(got)
	if len(want) < n {
		n = len(want)
	}
	i := 0
	for i < n && got[i] == want[i] {
		i++
	}
	lo := i - 120
	if lo < 0 {
		lo = 0
	}
	hi := i + 160
	ghi, whi := hi, hi
	if ghi > len(got) {
		ghi = len(got)
	}
	if whi > len(want) {
		whi = len(want)
	}
	return fmt.Sprintf("at byte %d\ngot:  %s\nwant: %s", i, got[lo:ghi], want[lo:whi])
}

type memLex struct {
	rows  []Value
	list  bool
	throw bool
}

func (m *memLex) Find(key string) (Value, bool) {
	for _, row := range m.rows {
		k, ok := row.Get("key")
		if ok && k.K == KindStr && exactKey(k.S) == key {
			return row, true
		}
	}
	return Value{}, false
}

func (m *memLex) ListByStrippedKeys(keys []string) ([]Value, error) {
	if m.throw {
		return nil, fmt.Errorf("no such table")
	}
	set := map[string]struct{}{}
	for _, key := range keys {
		set[strippedKey(key)] = struct{}{}
	}
	var out []Value
	for _, row := range m.rows {
		k, ok := row.Get("key")
		if !ok || k.K != KindStr {
			continue
		}
		if _, yes := set[strippedKey(k.S)]; yes {
			out = append(out, row)
		}
	}
	if out == nil {
		out = []Value{}
	}
	return out, nil
}

type findOnly struct{ rows []Value }

func (m findOnly) Find(key string) (Value, bool) {
	return (&memLex{rows: m.rows}).Find(key)
}

func optionsFrom(v Value) *Options {
	opt := &Options{}
	if lex, ok := v.Get("lexicon"); ok && lex.IsObject() {
		rows, _ := lex.Get("rows")
		list := listOf(rows)
		spec := &memLex{rows: list}
		if flag, ok := lex.Get("listByStrippedKeys"); ok {
			if flag.K == KindBool && flag.B {
				spec.list = true
			}
			if flag.K == KindStr && flag.S == "throw" {
				spec.list = true
				spec.throw = true
			}
		}
		if spec.list {
			opt.Lexicon = spec
		} else if flag, ok := lex.Get("find"); ok && flag.K == KindBool && flag.B {
			opt.Lexicon = findOnly{rows: list}
		}
	}
	if snap, ok := v.Get("snapshot"); ok && snap.K != KindNull {
		if snap.K == KindStr {
			opt.SnapshotMode = SnapRaw
			opt.SnapshotRaw = snap.S
		} else {
			opt.SnapshotMode = SnapValue
			opt.Snapshot = snap
		}
	}
	return opt
}
