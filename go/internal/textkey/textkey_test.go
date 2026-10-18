package textkey

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExactKeyMatchesNode(t *testing.T) {
	cmd := exec.Command("node", filepath.Join("testdata", "exact-key-oracle.js"))
	cmd.Env = append(os.Environ(), "DATA_DIR="+t.TempDir())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("oracle: %v\n%s", err, stderr.String())
	}
	var space []int
	keys := map[string]string{}
	for _, line := range bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var row struct {
			Kind    string `json:"kind"`
			Node    string `json:"node"`
			Unicode string `json:"unicode"`
			CPs     []int  `json:"cps"`
			ID      string `json:"id"`
			Out     string `json:"out"`
		}
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatalf("line %s: %v", line, err)
		}
		switch row.Kind {
		case "space":
			if !strings.HasPrefix(row.Node, "v22.14.") || row.Unicode != "16.0" {
				t.Fatalf("node %s unicode %s", row.Node, row.Unicode)
			}
			space = row.CPs
		case "key":
			keys[row.ID] = row.Out
		default:
			t.Fatalf("kind %s", row.Kind)
		}
	}
	if len(space) == 0 || len(keys) == 0 {
		t.Fatal("oracle returned no rows")
	}
	gotSpace := map[rune]bool{}
	for _, cp := range space {
		r := rune(cp)
		gotSpace[r] = true
		if !jsSpace(r) {
			t.Fatalf("U+%04X is \\s in Node and not in the hardcoded set", cp)
		}
	}
	for cp := rune(0); cp <= 0x10FFFF; cp++ {
		if cp >= 0xD800 && cp <= 0xDFFF {
			continue
		}
		if jsSpace(cp) && !gotSpace[cp] {
			t.Fatalf("U+%04X is hardcoded and is not \\s in Node", cp)
		}
	}

	for id, want := range keys {
		in := exactInput(t, id)
		if got := ExactKey(in); got != want {
			t.Fatalf("%s:\n go   %q\n node %q", id, got, want)
		}
		if !utf8.ValidString(want) {
			t.Fatalf("%s node output is not utf-8", id)
		}
	}
	if keys["reorder"] != keys["ordered"] {
		t.Fatalf("nikud order did not meet: %q vs %q", keys["reorder"], keys["ordered"])
	}
	if keys["reorder"] == exactInput(t, "reorder") {
		t.Fatal("NFC left the swapped nikud order unchanged")
	}
	if keys["u1c89"] != "a\u1C89b" || keys["u1fae9"] != "a\U0001FAE9b" {
		t.Fatalf("astral or cyrillic was treated as space: %#v", keys)
	}
	if keys["nel-inside"] != "a\u0085b" || keys["fef"] != "a" {
		t.Fatalf("FEFF or NEL: fef %q nel %q", keys["fef"], keys["nel-inside"])
	}
	if keys["case"] != "Shalom" {
		t.Fatalf("lowercased: %q", keys["case"])
	}
	if ExactKey("\xff") != "" {
		t.Fatal("invalid utf-8 was kept")
	}
}

func exactInput(t *testing.T, id string) string {
	t.Helper()
	switch id {
	case "reorder":
		return "ש\u05C1\u05B8"
	case "ordered":
		return "ש\u05B8\u05C1"
	case "double-space":
		return "a  b"
	case "em-space":
		return "a\u2003b"
	case "nbsp":
		return "a\u00A0b"
	case "fef":
		return "\uFEFFa\uFEFF"
	case "nel-inside":
		return "a\u0085b"
	case "nel-edge":
		return "\u0085a\u0085"
	case "u1c89":
		return "a\u1C89b"
	case "u1fae9":
		return "a\U0001FAE9b"
	case "case":
		return "Shalom"
	case "edges":
		return " \t\n a \u1680 "
	case "hebrew-gap":
		return "שָׁלוֹם  בַּיִת"
	case "only-space":
		return " \u3000\uFEFF "
	case "empty":
		return ""
	case "mongolian":
		return "a\u180Eb"
	default:
		t.Fatalf("unknown id %s", id)
		return ""
	}
}
