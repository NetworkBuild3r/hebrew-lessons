package plans

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/ucd"
)

// Every Unicode 15 to 16 category change that the validators can see is checked
// through exactKey, strippedKey, and tokenize. The strings come from Node.
func TestUnicodeDeltaKeys(t *testing.T) {
	if ucd.UnicodeVersion != "16.0" {
		t.Fatalf("tables are Unicode %s", ucd.UnicodeVersion)
	}
	f, err := os.Open(filepath.Join("..", "..", "testdata", "golden", "ucd", "deltas.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	const word = "שלום"
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "# unicode ") && strings.TrimPrefix(line, "# unicode ") != ucd.UnicodeVersion {
				t.Fatalf("delta file says %s, tables are %s", strings.TrimPrefix(line, "# unicode "), ucd.UnicodeVersion)
			}
			continue
		}
		col := strings.Split(line, "\t")
		cp, err := strconv.ParseUint(col[0], 16, 32)
		if err != nil {
			t.Fatalf("cp %q", col[0])
		}
		ch := string(rune(cp))
		input := word + ch
		var exact, stripped string
		var tokens []string
		switch col[1] {
		case "k":
			exact, stripped, tokens = input, input, []string{input}
		case "e":
			exact, stripped, tokens = input, input, []string{word}
		case "x":
			if len(col) != 5 {
				t.Fatalf("exception %q", line)
			}
			exact = jsonString(t, col[2])
			stripped = jsonString(t, col[3])
			if err := json.Unmarshal([]byte(col[4]), &tokens); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("flag %q", col[1])
		}
		if got := exactKey(input); got != exact {
			t.Errorf("U+%04X exactKey %q, node %q", cp, got, exact)
		}
		if got := strippedKey(input); got != stripped {
			t.Errorf("U+%04X strippedKey %q, node %q", cp, got, stripped)
		}
		gotTok := tokenize(input)
		if strings.Join(gotTok, "\x00") != strings.Join(tokens, "\x00") {
			t.Errorf("U+%04X tokens %q, node %q", cp, gotTok, tokens)
		}
		n++
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if n < 1000 {
		t.Fatalf("only %d unicode deltas", n)
	}
}

func jsonString(t *testing.T, quoted string) string {
	t.Helper()
	var s string
	if err := json.Unmarshal([]byte(quoted), &s); err != nil {
		t.Fatal(err)
	}
	return s
}
