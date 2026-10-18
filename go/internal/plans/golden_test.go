package plans

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGoldenValidate(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "golden", "validate")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no golden cases")
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(root, name)
			input, err := os.ReadFile(filepath.Join(dir, "input.txt"))
			if err != nil {
				t.Fatal(err)
			}
			optText, err := os.ReadFile(filepath.Join(dir, "options.json"))
			if err != nil {
				t.Fatal(err)
			}
			optVal, err := Parse(string(optText))
			if err != nil {
				t.Fatal(err)
			}
			opt := optionsFrom(optVal)
			mode, _ := optVal.StrKey("inputMode")
			var doc any = string(input)
			if mode == "object" {
				parsed, err := Parse(string(input))
				if err != nil {
					t.Fatal(err)
				}
				doc = parsed
			}
			got := ValidatePlan(doc, opt).JSON()
			wantRaw, err := os.ReadFile(filepath.Join(dir, "want.json"))
			if err != nil {
				t.Fatal(err)
			}
			want, err := restringify(string(wantRaw))
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("validatePlan\n%s", snippet(got, want))
			}
			hebrewWant, err := os.ReadFile(filepath.Join(dir, "hebrew.json"))
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := Parse(string(input))
			var hebrew []Finding
			if err == nil && parsed.IsObject() {
				hebrew = CheckHebrew(parsed, opt)
			} else {
				hebrew = []Finding{}
			}
			gotH := Stringify(findingsValue(hebrew))
			wantH, err := restringify(string(hebrewWant))
			if err != nil {
				t.Fatal(err)
			}
			if gotH != wantH {
				t.Fatalf("checkHebrew\n%s", snippet(gotH, wantH))
			}
		})
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

func restringify(raw string) (string, error) {
	v, err := Parse(raw)
	if err != nil {
		return "", err
	}
	return Stringify(v), nil
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
