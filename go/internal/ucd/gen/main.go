// Command gen writes go/internal/ucd/tables.go from the Unicode Character
// Database. It refuses to run unless Node reports the Unicode version this
// package is pinned to.
//
//	node -e 'process.stdout.write(process.versions.unicode)'
//	go run ./internal/ucd/gen
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	// wantNode is process.versions.unicode for Node 22.14.
	wantNode = "16.0"
	// ucdDir is the Unicode.org directory for that version.
	ucdDir = "16.0.0"
	// previousDir is the Unicode version in Go 1.26's unicode package.
	previousDir = "15.0.0"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	node, err := nodeUnicode()
	if err != nil {
		return err
	}
	if node != wantNode {
		return fmt.Errorf("node process.versions.unicode is %s, generator is pinned to %s", node, wantNode)
	}
	cur, err := load(ucdDir)
	if err != nil {
		return err
	}
	prev, err := load(previousDir)
	if err != nil {
		return err
	}
	js, err := nodeJSSpace()
	if err != nil {
		return err
	}
	if err := checkJSSpace(cur.white, js); err != nil {
		return err
	}
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	src, err := emit(cur)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "ucd", "tables.go"), src, 0o644); err != nil {
		return err
	}
	cps := deltaCPs(prev, cur)
	var b strings.Builder
	fmt.Fprintf(&b, "# code points whose L/M/N/P/S/Cf membership changed from Unicode %s to %s\n", previousDir, ucdDir)
	for _, sp := range cps {
		if sp.lo == sp.hi {
			fmt.Fprintf(&b, "%04X\n", sp.lo)
		} else {
			fmt.Fprintf(&b, "%04X..%04X\n", sp.lo, sp.hi)
		}
	}
	return os.WriteFile(filepath.Join(root, "internal", "ucd", "delta-cps.txt"), []byte(b.String()), 0o644)
}

type ucd struct {
	names []string
	gc    []catSpan
	white []span
	lower []lowerDelta
}

type span struct{ lo, hi uint32 }

type catSpan struct {
	lo, hi uint32
	gc     byte
}

type lowerDelta struct{ from, to rune }

type namedRange struct {
	lo, hi uint32
	name   string
}

func load(dir string) (*ucd, error) {
	gcText, err := fetch(dir, "extracted/DerivedGeneralCategory.txt")
	if err != nil {
		return nil, err
	}
	propText, err := fetch(dir, "PropList.txt")
	if err != nil {
		return nil, err
	}
	coreText, err := fetch(dir, "DerivedCoreProperties.txt")
	if err != nil {
		return nil, err
	}
	if !strings.Contains(coreText, "Derived Property") && !strings.Contains(coreText, ";") {
		return nil, fmt.Errorf("%s DerivedCoreProperties.txt is empty", dir)
	}
	dataText, err := fetch(dir, "UnicodeData.txt")
	if err != nil {
		return nil, err
	}
	specialText, err := fetch(dir, "SpecialCasing.txt")
	if err != nil {
		return nil, err
	}
	gcRanges := parseNamed(gcText)
	names, gc, err := categorySpans(gcRanges)
	if err != nil {
		return nil, err
	}
	var white []span
	for _, r := range parseNamed(propText) {
		if r.name == "White_Space" {
			white = append(white, span{r.lo, r.hi})
		}
	}
	white = coalesce(white)
	lower := lowerDeltas(dataText, specialText)
	return &ucd{names: names, gc: gc, white: white, lower: lower}, nil
}

func fetch(dir, name string) (string, error) {
	url := "https://www.unicode.org/Public/" + dir + "/ucd/" + name
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "hebrew-lessons-ucd")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("GET %s: %s", url, res.Status)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if len(body) == 0 {
		return "", fmt.Errorf("%s is empty", url)
	}
	return string(body), nil
}

func parseNamed(text string) []namedRange {
	var out []namedRange
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ";", 2)
		if len(parts) != 2 {
			continue
		}
		lo, hi, ok := parseSpan(strings.TrimSpace(parts[0]))
		if !ok {
			continue
		}
		name := strings.TrimSpace(parts[1])
		if i := strings.IndexByte(name, ';'); i >= 0 {
			name = strings.TrimSpace(name[:i])
		}
		name = strings.Fields(name)[0]
		out = append(out, namedRange{lo, hi, name})
	}
	return out
}

func parseSpan(s string) (uint32, uint32, bool) {
	a, b, ok := strings.Cut(s, "..")
	lo, err1 := strconv.ParseUint(a, 16, 32)
	if err1 != nil {
		return 0, 0, false
	}
	if !ok {
		return uint32(lo), uint32(lo), true
	}
	hi, err2 := strconv.ParseUint(b, 16, 32)
	if err2 != nil {
		return 0, 0, false
	}
	return uint32(lo), uint32(hi), true
}

func categorySpans(ranges []namedRange) ([]string, []catSpan, error) {
	index := map[string]byte{}
	var order []string
	var spans []catSpan
	for _, r := range ranges {
		id, ok := index[r.name]
		if !ok {
			if len(order) > 255 {
				return nil, nil, fmt.Errorf("too many general categories")
			}
			id = byte(len(order))
			index[r.name] = id
			order = append(order, r.name)
		}
		spans = append(spans, catSpan{r.lo, r.hi, id})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].lo < spans[j].lo })
	var merged []catSpan
	for _, sp := range spans {
		if len(merged) > 0 {
			last := &merged[len(merged)-1]
			if last.gc == sp.gc && last.hi+1 == sp.lo {
				last.hi = sp.hi
				continue
			}
		}
		merged = append(merged, sp)
	}
	return order, merged, nil
}

func coalesce(in []span) []span {
	sort.Slice(in, func(i, j int) bool { return in[i].lo < in[j].lo })
	var out []span
	for _, sp := range in {
		if len(out) > 0 && out[len(out)-1].hi+1 == sp.lo {
			out[len(out)-1].hi = sp.hi
			continue
		}
		out = append(out, sp)
	}
	return out
}

func lowerDeltas(data, special string) []lowerDelta {
	uncond := map[rune]bool{}
	for _, line := range strings.Split(special, "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if line == "" {
			continue
		}
		f := strings.Split(line, ";")
		if len(f) < 5 {
			continue
		}
		if strings.TrimSpace(f[4]) != "" {
			continue
		}
		cp, err := strconv.ParseUint(strings.TrimSpace(f[0]), 16, 32)
		if err != nil {
			continue
		}
		uncond[rune(cp)] = true
	}
	fold := cases.Lower(language.Und)
	var out []lowerDelta
	for _, line := range strings.Split(data, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, ";")
		if len(f) < 14 || f[13] == "" {
			continue
		}
		cp, err1 := strconv.ParseUint(f[0], 16, 32)
		lo, err2 := strconv.ParseUint(f[13], 16, 32)
		if err1 != nil || err2 != nil || cp > 0x10FFFF || lo > 0x10FFFF {
			continue
		}
		if cp >= 0xD800 && cp <= 0xDFFF {
			continue
		}
		from := rune(cp)
		if uncond[from] {
			continue
		}
		got := fold.String(string(from))
		want := string(rune(lo))
		if got == want {
			continue
		}
		if len([]rune(got)) != 1 || len([]rune(want)) != 1 {
			continue
		}
		out = append(out, lowerDelta{from, rune(lo)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].from < out[j].from })
	return out
}

func checkJSSpace(white []span, js []span) error {
	want := map[uint32]bool{}
	for _, sp := range white {
		for cp := sp.lo; cp <= sp.hi; cp++ {
			if cp != 0x0085 {
				want[cp] = true
			}
		}
	}
	want[0xFEFF] = true
	got := map[uint32]bool{}
	for _, sp := range js {
		for cp := sp.lo; cp <= sp.hi; cp++ {
			got[cp] = true
		}
	}
	if len(got) != len(want) {
		return fmt.Errorf("JS \\s has %d code points, White_Space minus U+0085 plus U+FEFF has %d", len(got), len(want))
	}
	for cp := range want {
		if !got[cp] {
			return fmt.Errorf("U+%04X is in the JS space formula but Node \\s does not match it", cp)
		}
	}
	return nil
}

func nodeUnicode() (string, error) {
	out, err := exec.Command("node", "-e", "process.stdout.write(process.versions.unicode)").Output()
	if err != nil {
		return "", fmt.Errorf("node unicode version: %w", err)
	}
	return string(out), nil
}

func nodeJSSpace() ([]span, error) {
	script := `
const spans = [];
let start = -1;
let prev = -1;
for (let cp = 0; cp <= 0x10FFFF; cp++) {
  if (cp >= 0xD800 && cp <= 0xDFFF) continue;
  const yes = /^\s$/u.test(String.fromCodePoint(cp));
  if (!yes) continue;
  if (start < 0) { start = prev = cp; continue; }
  if (cp === prev + 1) { prev = cp; continue; }
  spans.push(start.toString(16) + '..' + prev.toString(16));
  start = prev = cp;
}
if (start >= 0) spans.push(start.toString(16) + '..' + prev.toString(16));
process.stdout.write(spans.join('\n'));
`
	out, err := exec.Command("node", "-e", script).Output()
	if err != nil {
		return nil, fmt.Errorf("node \\s scan: %w", err)
	}
	var spans []span
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		lo, hi, ok := parseSpan(line)
		if !ok {
			return nil, fmt.Errorf("bad node space span %q", line)
		}
		spans = append(spans, span{lo, hi})
	}
	return spans, nil
}

func deltaCPs(prev, cur *ucd) []span {
	class := []func(string) bool{
		func(n string) bool { return n != "" && n[0] == 'L' },
		func(n string) bool { return n != "" && n[0] == 'M' },
		func(n string) bool { return n != "" && n[0] == 'N' },
		func(n string) bool { return n != "" && n[0] == 'P' },
		func(n string) bool { return n != "" && n[0] == 'S' },
		func(n string) bool { return n == "Cf" },
	}
	var raw []span
	var run span
	open := false
	for cp := uint32(0); cp <= 0x10FFFF; cp++ {
		a, b := catAt(prev, cp), catAt(cur, cp)
		changed := false
		for _, prop := range class {
			if prop(a) != prop(b) {
				changed = true
				break
			}
		}
		if !changed {
			continue
		}
		if open && run.hi+1 == cp {
			run.hi = cp
			continue
		}
		if open {
			raw = append(raw, run)
		}
		run = span{cp, cp}
		open = true
	}
	if open {
		raw = append(raw, run)
	}
	return raw
}

func catAt(u *ucd, cp uint32) string {
	lo, hi := 0, len(u.gc)
	for lo < hi {
		mid := (lo + hi) / 2
		if u.gc[mid].hi < cp {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(u.gc) && u.gc[lo].lo <= cp && cp <= u.gc[lo].hi {
		id := u.gc[lo].gc
		if int(id) < len(u.names) {
			return u.names[id]
		}
	}
	return "Cn"
}

func emit(u *ucd) ([]byte, error) {
	js := jsSpans(u.white)
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by go run ./internal/ucd/gen. DO NOT EDIT.\n")
	fmt.Fprintf(&b, "package ucd\n\n")
	fmt.Fprintf(&b, "// UnicodeVersion is the Unicode version of these tables.\n")
	fmt.Fprintf(&b, "// It matches Node's process.versions.unicode, not the UCD directory name.\n")
	fmt.Fprintf(&b, "const UnicodeVersion = %q\n\n", wantNode)
	fmt.Fprintf(&b, "var catName = []string{")
	for i, name := range u.names {
		if i%8 == 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%q, ", name)
	}
	fmt.Fprintf(&b, "\n}\n\n")
	fmt.Fprintf(&b, "type catSpan struct {\n\tlo, hi uint32\n\tgc     byte\n}\n\n")
	fmt.Fprintf(&b, "var categories = []catSpan{\n")
	for _, sp := range u.gc {
		fmt.Fprintf(&b, "\t{0x%X, 0x%X, %d},\n", sp.lo, sp.hi, sp.gc)
	}
	fmt.Fprintf(&b, "}\n\n")
	fmt.Fprintf(&b, "type span struct{ lo, hi uint32 }\n\n")
	writeSpans(&b, "whiteSpace", u.white)
	writeSpans(&b, "jsSpace", js)
	fmt.Fprintf(&b, "func LowerOverride(r rune) (rune, bool) {\n\tswitch r {\n")
	for _, d := range u.lower {
		fmt.Fprintf(&b, "\tcase 0x%X:\n\t\treturn 0x%X, true\n", d.from, d.to)
	}
	fmt.Fprintf(&b, "\tdefault:\n\t\treturn 0, false\n\t}\n}\n\n")
	fmt.Fprintf(&b, "func category(r rune) string {\n")
	fmt.Fprintf(&b, "\tif r < 0 || r > 0x10FFFF {\n\t\treturn \"Cn\"\n\t}\n")
	fmt.Fprintf(&b, "\tcp := uint32(r)\n")
	fmt.Fprintf(&b, "\tlo, hi := 0, len(categories)\n")
	fmt.Fprintf(&b, "\tfor lo < hi {\n")
	fmt.Fprintf(&b, "\t\tmid := (lo + hi) / 2\n")
	fmt.Fprintf(&b, "\t\tif categories[mid].hi < cp {\n\t\t\tlo = mid + 1\n\t\t} else {\n\t\t\thi = mid\n\t\t}\n")
	fmt.Fprintf(&b, "\t}\n")
	fmt.Fprintf(&b, "\tif lo < len(categories) && categories[lo].lo <= cp && cp <= categories[lo].hi {\n")
	fmt.Fprintf(&b, "\t\treturn catName[categories[lo].gc]\n\t}\n")
	fmt.Fprintf(&b, "\treturn \"Cn\"\n}\n\n")
	fmt.Fprintf(&b, "func inSpans(spans []span, r rune) bool {\n")
	fmt.Fprintf(&b, "\tif r < 0 || r > 0x10FFFF {\n\t\treturn false\n\t}\n")
	fmt.Fprintf(&b, "\tcp := uint32(r)\n")
	fmt.Fprintf(&b, "\tlo, hi := 0, len(spans)\n")
	fmt.Fprintf(&b, "\tfor lo < hi {\n")
	fmt.Fprintf(&b, "\t\tmid := (lo + hi) / 2\n")
	fmt.Fprintf(&b, "\t\tif spans[mid].hi < cp {\n\t\t\tlo = mid + 1\n\t\t} else {\n\t\t\thi = mid\n\t\t}\n")
	fmt.Fprintf(&b, "\t}\n")
	fmt.Fprintf(&b, "\treturn lo < len(spans) && spans[lo].lo <= cp && cp <= spans[lo].hi\n}\n\n")
	fmt.Fprintf(&b, "// WhiteSpace is the Unicode White_Space property.\n")
	fmt.Fprintf(&b, "func WhiteSpace(r rune) bool { return inSpans(whiteSpace, r) }\n\n")
	fmt.Fprintf(&b, "// JSSpace is the set String.prototype.trim removes and /\\s/ matches\n")
	fmt.Fprintf(&b, "// in Node. It is White_Space without U+0085, plus U+FEFF.\n")
	fmt.Fprintf(&b, "func JSSpace(r rune) bool { return inSpans(jsSpace, r) }\n")
	return format.Source(b.Bytes())
}

func writeSpans(b *bytes.Buffer, name string, spans []span) {
	fmt.Fprintf(b, "var %s = []span{\n", name)
	for _, sp := range spans {
		fmt.Fprintf(b, "\t{0x%X, 0x%X},\n", sp.lo, sp.hi)
	}
	fmt.Fprintf(b, "}\n\n")
}

func jsSpans(white []span) []span {
	var out []span
	for _, sp := range white {
		lo, hi := sp.lo, sp.hi
		if hi < 0x85 || lo > 0x85 {
			out = append(out, sp)
			continue
		}
		if lo < 0x85 {
			out = append(out, span{lo, 0x84})
		}
		if hi > 0x85 {
			out = append(out, span{0x86, hi})
		}
	}
	out = append(out, span{0xFEFF, 0xFEFF})
	return coalesce(out)
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}
