package textkey

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

func TestExactKeyMatchesNode(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "golden", "exact-key.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.Valid(raw) {
		t.Fatal("golden file is not valid UTF-8")
	}
	var space []int
	var node, unicode string
	keys := map[string]struct{ in, out string }{}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var row struct {
			Kind    string `json:"kind"`
			Node    string `json:"node"`
			Unicode string `json:"unicode"`
			CPs     []int  `json:"cps"`
			ID      string `json:"id"`
			In      string `json:"in"`
			Out     string `json:"out"`
		}
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatalf("line %s: %v", line, err)
		}
		switch row.Kind {
		case "space":
			if row.Node != "v22.14.0" || row.Unicode != "16.0" {
				t.Fatalf("golden node %s unicode %s", row.Node, row.Unicode)
			}
			node, unicode = row.Node, row.Unicode
			space = row.CPs
		case "key":
			keys[row.ID] = struct{ in, out string }{row.In, row.Out}
		default:
			t.Fatalf("kind %s", row.Kind)
		}
	}
	if node != "v22.14.0" || unicode != "16.0" || len(space) == 0 || len(keys) == 0 {
		t.Fatal("golden is missing the space set or the keys")
	}
	gotSpace := map[rune]bool{}
	for _, cp := range space {
		r := rune(cp)
		gotSpace[r] = true
		if !jsSpace(r) {
			t.Fatalf("U+%04X is \\s in the golden and not in the hardcoded set", cp)
		}
	}
	for cp := rune(0); cp <= 0x10FFFF; cp++ {
		if cp >= 0xD800 && cp <= 0xDFFF {
			continue
		}
		if jsSpace(cp) && !gotSpace[cp] {
			t.Fatalf("U+%04X is hardcoded and is not \\s in the golden", cp)
		}
	}

	for id, item := range keys {
		if got := ExactKey(item.in); got != item.out {
			t.Fatalf("%s:\n go     %q\n golden %q", id, got, item.out)
		}
		if !utf8.ValidString(item.out) {
			t.Fatalf("%s golden output is not utf-8", id)
		}
	}
	reorder := keys["reorder"]
	ordered := keys["ordered"]
	if reorder.out != ordered.out {
		t.Fatalf("nikud order did not meet: %q vs %q", reorder.out, ordered.out)
	}
	if reorder.out == reorder.in {
		t.Fatal("NFC left the swapped nikud order unchanged")
	}
	if keys["u1c89"].out != "a\u1C89b" || keys["u1fae9"].out != "a\U0001FAE9b" {
		t.Fatalf("astral or cyrillic was treated as space: %#v", keys)
	}
	if keys["nel-inside"].out != "a\u0085b" || keys["fef"].out != "a" {
		t.Fatalf("FEFF or NEL: fef %q nel %q", keys["fef"].out, keys["nel-inside"].out)
	}
	if keys["case"].out != "Shalom" {
		t.Fatalf("lowercased: %q", keys["case"].out)
	}
	if ExactKey("\xff") != "" {
		t.Fatal("invalid utf-8 was kept")
	}
}
