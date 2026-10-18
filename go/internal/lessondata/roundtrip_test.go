package lessondata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// unicodeCaseIDs are the Node goldens added for astral text, surrogates, and Hebrew.
var unicodeCaseIDs = []string{
	"uni-emoji-value",
	"uni-emoji-key",
	"uni-emoji-zwj",
	"uni-emoji-skin",
	"uni-emoji-flag",
	"uni-emoji-keycap",
	"uni-astral",
	"uni-astral-esc",
	"uni-surrogate-reversed",
	"uni-surrogate-lone",
	"uni-hebrew-nikud",
	"uni-hebrew-teamim",
	"uni-hebrew-maqaf",
	"uni-hebrew-presentation",
	"uni-hebrew-key",
	"uni-rtl-emoji",
}

func TestRoundTripByteIdentity(t *testing.T) {
	root := repoRoot(t)
	base := goldenRoot(t)
	seen := map[string]bool{}
	for _, line := range readJSONL(t, "quirks.jsonl") {
		var row quirkRow
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatal(err)
		}
		seen[row.ID] = true
		t.Run("quirks/"+row.ID, func(t *testing.T) {
			assertByteIdentity(t, row.Label, row.Input, row.Written, row.Emit, row.Assign)
		})
	}
	for _, id := range unicodeCaseIDs {
		if !seen[id] {
			t.Errorf("missing unicode golden %s", id)
		}
	}
	for _, line := range readJSONL(t, "worksheets.jsonl") {
		var row worksheetRow
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatal(err)
		}
		t.Run(row.Repo, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(row.Repo)))
			if err != nil {
				t.Fatal(err)
			}
			path, err := writtenFile(base, row.Written)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			assertByteIdentity(t, row.Label, string(input), string(want), row.Emit, row.Assign)
		})
	}
}

// TestConstructedUnicode stringifies Values built in Go, not Values that came
// out of Parse. The expected bytes are the Node goldens.
func TestConstructedUnicode(t *testing.T) {
	grin := codes(0x1F600)
	shalom := codes(0x05E9, 0x05DC, 0x05D5, 0x05DD)
	family := codes(0x1F468, 0x200D, 0x1F469, 0x200D, 0x1F467, 0x200D, 0x1F466)
	flagIL := codes(0x1F1EE, 0x1F1F1)
	astral := codes(0x10000, 0x1D11E, 0x20000, 0x10FFFF)
	cases := []struct {
		id  string
		doc Document
	}{
		{"uni-emoji-value", global("X", strValue(grin+" "+shalom))},
		{"uni-emoji-key", global("X", objValue(&Obj{
			Keys: []string{grin, family},
			Vals: []Value{
				objValue(&Obj{Keys: []string{"word"}, Vals: []Value{strValue(shalom)}}),
				objValue(&Obj{Keys: []string{"flag"}, Vals: []Value{strValue(flagIL)}}),
			},
		}))},
		{"uni-emoji-zwj", global("X", strValue(codes(
			0x1F468, 0x200D, 0x1F469, 0x200D, 0x1F467, 0x200D, 0x1F466,
			0x1F468, 0x200D, 0x1F4BB,
			0x1F3F3, 0xFE0F, 0x200D, 0x1F308,
		)))},
		{"uni-emoji-skin", global("X", strValue(codes(0x1F44B, 0x1F3FB, 0x1F44D, 0x1F3FD, 0x1F468, 0x1F3FF)))},
		{"uni-emoji-flag", global("X", strValue(codes(0x1F1EE, 0x1F1F1, 0x1F1FA, 0x1F1F8, 0x1F1EF, 0x1F1F5)))},
		{"uni-emoji-keycap", global("X", strValue(codes(0x31, 0xFE0F, 0x20E3, 0x23, 0xFE0F, 0x20E3, 0x2A, 0xFE0F, 0x20E3)))},
		{"uni-astral", global("X", strValue(astral))},
		{"uni-astral-esc", global("X", strValue(astral))},
		{"uni-surrogate-reversed", global("X", strValue(unitsToString([]uint16{0xDE00, 0xD83D, 0xDFFF, 0xD800})))},
		{"uni-surrogate-lone", global("X", strValue(unitsToString([]uint16{0xD800, 0xDEAD, 0xDFFF})))},
		{"uni-hebrew-nikud", global("X", strValue(hebrewNikud()))},
		{"uni-hebrew-teamim", global("X", strValue(hebrewTeamim()))},
		{"uni-hebrew-maqaf", global("X", strValue(hebrewMaqaf()))},
		{"uni-hebrew-presentation", global("X", strValue(hebrewPresentation()))},
		{"uni-hebrew-key", global("X", objValue(&Obj{
			Keys: []string{hebrewTeamim()},
			Vals: []Value{objValue(&Obj{Keys: []string{"ok"}, Vals: []Value{boolValue(true)}})},
		}))},
		{"uni-rtl-emoji", global("X", strValue(hebrewNikud()+" "+codes(
			0x1F468, 0x200D, 0x1F469, 0x200D, 0x1F467, 0x200D, 0x1F466,
			0x1F468, 0x200D, 0x1F4BB,
			0x1F3F3, 0xFE0F, 0x200D, 0x1F308,
		)+" "+hebrewTeamim()+" "+codes(0x1F1EE, 0x1F1F1, 0x1F1FA, 0x1F1F8, 0x1F1EF, 0x1F1F5)+" "+codes(0x31, 0xFE0F, 0x20E3, 0x23, 0xFE0F, 0x20E3, 0x2A, 0xFE0F, 0x20E3)+" "+hebrewMaqaf()+" "+hebrewPresentation()))},
	}
	if len(cases) != len(unicodeCaseIDs) {
		t.Fatalf("constructed %d cases, unicode list %d", len(cases), len(unicodeCaseIDs))
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			row := quirkByID(t, tc.id)
			if tc.doc.Write() != row.Written {
				t.Fatalf("written\n%s", diffAt(tc.doc.Write(), row.Written))
			}
			if tc.doc.Emit() != row.Emit {
				t.Fatalf("emit\n%s", diffAt(tc.doc.Emit(), row.Emit))
			}
			if tc.doc.Assign() != row.Assign {
				t.Fatalf("assign\n%s", diffAt(tc.doc.Assign(), row.Assign))
			}
			if strings.ContainsRune(tc.doc.Write(), '\uFFFD') {
				t.Fatalf("go-constructed write contains U+FFFD: %q", tc.doc.Write())
			}
			again, err := Parse(tc.doc.Write(), row.Label)
			if err != nil {
				t.Fatal(err)
			}
			if again.Write() != tc.doc.Write() {
				t.Fatalf("second pass\n%s", diffAt(again.Write(), tc.doc.Write()))
			}
			if !sameDocument(tc.doc, again) {
				t.Fatal("parse(write(v)) != v")
			}
		})
	}
}

func TestEmojiGoStringIsNotReplaced(t *testing.T) {
	got := WriteIndented("X", Value{Kind: String, Str: "😀 שלום"})
	if strings.ContainsRune(got, '\uFFFD') || !strings.Contains(got, "😀") {
		t.Fatalf("plain Go string was corrupted: %q", got)
	}
	got = WriteIndented("X", Value{Kind: Object, Obj: &Obj{
		Keys: []string{"😀"},
		Vals: []Value{strValue("שלום")},
	}})
	if strings.ContainsRune(got, '\uFFFD') || !strings.Contains(got, "😀") {
		t.Fatalf("plain Go key was corrupted: %q", got)
	}
}

func FuzzRoundTrip(f *testing.F) {
	root := repoRoot(f)
	for _, line := range readJSONL(f, "quirks.jsonl") {
		var row quirkRow
		if err := json.Unmarshal(line, &row); err != nil {
			f.Fatal(err)
		}
		f.Add(row.Input)
	}
	for _, line := range readJSONL(f, "worksheets.jsonl") {
		var row worksheetRow
		if err := json.Unmarshal(line, &row); err != nil {
			f.Fatal(err)
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(row.Repo)))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(body))
	}
	f.Fuzz(func(t *testing.T, src string) {
		doc, err := Parse(src, "fuzz")
		if err != nil {
			return
		}
		written := doc.Write()
		if utf8.ValidString(src) && !strings.ContainsRune(src, '\uFFFD') && strings.ContainsRune(written, '\uFFFD') {
			t.Fatalf("write introduced U+FFFD\n%s", written)
		}
		again, err := Parse(written, "fuzz")
		if err != nil {
			t.Fatalf("rewrite: %v\n%s", err, written)
		}
		if again.Write() != written {
			t.Fatalf("second pass changed output\n%s", diffAt(again.Write(), written))
		}
		if !sameDocument(doc, again) {
			t.Fatal("parse(write(v)) != v")
		}
	})
}

func assertByteIdentity(t *testing.T, label, input, written, emit, assign string) {
	t.Helper()
	doc, err := Parse(input, label)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Write() != written {
		t.Fatalf("written\n%s", diffAt(doc.Write(), written))
	}
	if doc.Emit() != emit {
		t.Fatalf("emit\n%s", diffAt(doc.Emit(), emit))
	}
	if doc.Assign() != assign {
		t.Fatalf("assign\n%s", diffAt(doc.Assign(), assign))
	}
	again, err := Parse(doc.Write(), label)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if again.Write() != written {
		t.Fatalf("second pass\n%s", diffAt(again.Write(), written))
	}
	if !sameDocument(doc, again) {
		t.Fatal("parse(write(v)) != v")
	}
	built := goConstructed(doc)
	if built.Write() != written || built.Emit() != emit || built.Assign() != assign {
		t.Fatalf("go-constructed writers diverged\n%s", diffAt(built.Write(), written))
	}
	if utf8.ValidString(input) && !strings.ContainsRune(input, '\uFFFD') && strings.ContainsRune(written, '\uFFFD') {
		t.Fatal("write introduced U+FFFD")
	}
}

func quirkByID(t *testing.T, id string) quirkRow {
	t.Helper()
	for _, line := range readJSONL(t, "quirks.jsonl") {
		var row quirkRow
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatal(err)
		}
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("missing quirk %s", id)
	return quirkRow{}
}

func global(name string, v Value) Document {
	return Document{Names: []string{name}, Values: []Value{v}}
}

func codes(cps ...rune) string { return string(cps) }

func hebrewNikud() string {
	return codes(
		0x05D1, 0x05BC, 0x05B0, 0x05E8, 0x05B5, 0x05D0, 0x05E9, 0x05B4, 0x05C1, 0x05D9, 0x05EA, ' ',
		0x05D1, 0x05BC, 0x05B8, 0x05E8, 0x05B8, 0x05D0, ' ',
		0x05D0, 0x05B1, 0x05DC, 0x05B9, 0x05D4, 0x05B4, 0x05D9, 0x05DD,
	)
}

func hebrewTeamim() string {
	var b strings.Builder
	for c := rune(0x0591); c <= 0x05AF; c++ {
		b.WriteRune(0x05D1)
		b.WriteRune(c)
	}
	return b.String()
}

func hebrewMaqaf() string {
	return codes(
		0x05D5, 0x05B7, 0x05D9, 0x05B9, 0x05D0, 0x05DE, 0x05B6, 0x05E8, 0x05BE,
		0x05D0, 0x05B1, 0x05DC, 0x05B9, 0x05D4, 0x05B4, 0x05D9, 0x05DD, 0x05C3,
	)
}

func hebrewPresentation() string {
	var b strings.Builder
	for c := rune(0xFB1D); c <= 0xFB4F; c++ {
		b.WriteRune(c)
	}
	return b.String()
}

func goConstructed(doc Document) Document {
	out := Document{Names: append([]string(nil), doc.Names...)}
	for _, v := range doc.Values {
		out.Values = append(out.Values, goConstructedValue(v))
	}
	return out
}

func goConstructedValue(v Value) Value {
	switch v.Kind {
	case String:
		return strValue(unitsToString(decodeUnits(v.Str)))
	case Array:
		arr := make([]Value, len(v.Arr))
		for i, el := range v.Arr {
			arr[i] = goConstructedValue(el)
		}
		return arrValue(arr)
	case Object:
		keys, vals := objEntries(v)
		o := &Obj{}
		for i, k := range keys {
			o.set(unitsToString(decodeUnits(k)), goConstructedValue(vals[i]))
		}
		return objValue(o)
	default:
		return v
	}
}

func objEntries(v Value) ([]string, []Value) {
	if v.Obj == nil {
		return nil, nil
	}
	return v.Obj.Keys, v.Obj.Vals
}

func sameDocument(a, b Document) bool {
	if len(a.Names) != len(b.Names) {
		return false
	}
	for i := range a.Names {
		if a.Names[i] != b.Names[i] || !sameValue(a.Values[i], b.Values[i]) {
			return false
		}
	}
	return true
}

func sameValue(a, b Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case Null:
		return true
	case Bool:
		return a.Bool == b.Bool
	case Number:
		return formatNumber(a.Num) == formatNumber(b.Num)
	case String:
		return unitsToString(decodeUnits(a.Str)) == unitsToString(decodeUnits(b.Str))
	case Array:
		if len(a.Arr) != len(b.Arr) {
			return false
		}
		for i := range a.Arr {
			if !sameValue(a.Arr[i], b.Arr[i]) {
				return false
			}
		}
		return true
	case Object:
		ak, av := objEntries(a)
		bk, bv := objEntries(b)
		if len(ak) != len(bk) {
			return false
		}
		used := make([]bool, len(bk))
		for i, k := range ak {
			canon := unitsToString(decodeUnits(k))
			found := false
			for j, k2 := range bk {
				if used[j] || unitsToString(decodeUnits(k2)) != canon {
					continue
				}
				if !sameValue(av[i], bv[j]) {
					return false
				}
				used[j] = true
				found = true
				break
			}
			if !found {
				return false
			}
		}
		return true
	default:
		return false
	}
}
