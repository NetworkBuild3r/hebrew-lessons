package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestLedgerExactExtraAndMissing(t *testing.T) {
	deps := wiredDeps(t, identityRoundTrip)

	t.Run("versions 1-11", func(t *testing.T) {
		fx := buildFixture(t, "apply", "clean", oneLesson())
		code, stdout, stderr := invoke(t, fx.root, []string{"--json"}, &deps)
		if code != 0 {
			t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
		}
		rep := decodeReport(t, stdout)
		if !rep.OK {
			t.Fatalf("ok false: %s", stdout)
		}
		for _, name := range []string{"ledger", "lexicon", "lessons"} {
			if got := mustCheck(t, rep, name).Status; got != statusPass {
				t.Fatalf("%s status %s\n%s", name, got, stdout)
			}
		}
		if !strings.Contains(stderr, "preflight: PASS") {
			t.Fatalf("stderr missing PASS:\n%s", stderr)
		}
	})

	t.Run("extra version 12", func(t *testing.T) {
		fx := buildFixture(t, "extra", "", oneLesson())
		code, stdout, stderr := invoke(t, fx.root, []string{"--json"}, &deps)
		if code != 1 {
			t.Fatalf("exit %d\n%s\n%s", code, stdout, stderr)
		}
		rep := decodeReport(t, stdout)
		if rep.OK {
			t.Fatal("ok on extra version")
		}
		ledger := mustCheck(t, rep, "ledger")
		if ledger.Status != statusFail || !strings.Contains(ledger.Summary, "unexpected version 12") {
			t.Fatalf("ledger: %+v", ledger)
		}
		if mustCheck(t, rep, "lexicon").Status != statusPass {
			t.Fatal(stdout)
		}
		if mustCheck(t, rep, "lessons").Status != statusPass {
			t.Fatal(stdout)
		}
	})

	t.Run("missing version 11", func(t *testing.T) {
		fx := buildFixture(t, "apply", "drop11", oneLesson())
		code, stdout, stderr := invoke(t, fx.root, []string{"--json"}, &deps)
		if code != 1 {
			t.Fatalf("exit %d\n%s\n%s", code, stdout, stderr)
		}
		ledger := mustCheck(t, decodeReport(t, stdout), "ledger")
		if ledger.Status != statusFail || !strings.Contains(ledger.Summary, "missing version 11") {
			t.Fatalf("ledger: %+v", ledger)
		}
	})
}

func TestLexiconMismatchesAndCollisions(t *testing.T) {
	fx := buildFixture(t, "apply", "bad", oneLesson())
	deps := wiredDeps(t, identityRoundTrip)
	code, stdout, stderr := invoke(t, fx.root, []string{"--json"}, &deps)
	if code != 1 {
		t.Fatalf("exit %d\n%s\n%s", code, stdout, stderr)
	}
	rep := decodeReport(t, stdout)
	if rep.OK {
		t.Fatal("ok with bad keys")
	}
	if mustCheck(t, rep, "ledger").Status != statusPass {
		t.Fatal(stdout)
	}
	raw := mustCheck(t, rep, "lexicon")
	if raw.Status != statusFail {
		t.Fatalf("lexicon %s: %s", raw.Status, raw.Summary)
	}
	var detail lexiconDetail
	if err := json.Unmarshal(raw.Detail, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Table != "lexicon_entries" {
		t.Fatalf("table %s", detail.Table)
	}
	if strings.Join(detail.Columns, ",") != "key,lemma_key" {
		t.Fatalf("columns %v", detail.Columns)
	}
	if detail.Rows != len(fx.rows) || detail.Rows != 7 {
		t.Fatalf("rows %d seed %d", detail.Rows, len(fx.rows))
	}
	if detail.Mismatches != 5 || detail.Collisions != 1 {
		t.Fatalf("mismatches=%d collisions=%d\n%s", detail.Mismatches, detail.Collisions, stdout)
	}
	counts := map[string]int{}
	for _, class := range detail.Classes {
		counts[class.Class] = class.Count
	}
	want := map[string]int{
		classInternal: 1,
		classEdge:     1,
		classNonASCII: 2,
		classNFC:      1,
		classOther:    0,
	}
	for class, n := range want {
		if counts[class] != n {
			t.Fatalf("class %q count %d want %d\n%s", class, counts[class], n, stdout)
		}
	}
	byKey := map[string]lexiconExample{}
	for _, ex := range detail.Examples {
		byKey[ex.Column+" "+ex.KeyCodePoints] = ex
		if ex.RowID <= 0 {
			t.Fatalf("missing rowid: %+v", ex)
		}
		if ex.ExactKeyCodePoints != codePoints(mustNode(t, ex.Key)) {
			t.Fatalf("exact code points for %s: got %s", ex.KeyCodePoints, ex.ExactKeyCodePoints)
		}
		seedID := rowID(t, fx.rows, ex.Key)
		if ex.RowID != seedID {
			t.Fatalf("rowid for %s: got %d want %d", ex.KeyCodePoints, ex.RowID, seedID)
		}
	}
	double := "U+05D0 U+0020 U+0020 U+05D1"
	if byKey["key "+double].Class != classInternal {
		t.Fatalf("double space: %+v", byKey["key "+double])
	}
	nbsp := "U+00A0 U+05DC U+05D7 U+05DD"
	if byKey["key "+nbsp].Class != classNonASCII {
		t.Fatalf("leading NBSP: %+v", byKey["key "+nbsp])
	}
	nfc := "U+05E9 U+05C1 U+05B8"
	if byKey["key "+nfc].Class != classNFC {
		t.Fatalf("nfc: %+v", byKey["key "+nfc])
	}
	if byKey["key U+0020 U+05D0 U+05D1"].Class != classEdge {
		t.Fatalf("leading space: %+v", byKey["key U+0020 U+05D0 U+05D1"])
	}
	if byKey["lemma_key U+00A0 U+05D0"].Class != classNonASCII {
		t.Fatalf("lemma_key: %+v", byKey["lemma_key U+00A0 U+05D0"])
	}
	if _, ok := byKey["key U+05E9 U+05DC U+05D5 U+05DD"]; ok {
		t.Fatal("clean key reported as a mismatch")
	}
	if len(detail.CollisionGroups) != 1 || len(detail.CollisionGroups[0].Rows) != 2 {
		t.Fatalf("groups: %+v", detail.CollisionGroups)
	}
	gotExact := detail.CollisionGroups[0].ExactKeyCodePoints
	if gotExact != "U+05D0 U+0020 U+05D1" {
		t.Fatalf("collision exact %s", gotExact)
	}
	var sawDouble, sawSingle bool
	for _, row := range detail.CollisionGroups[0].Rows {
		if row.RowID <= 0 || row.Column != "key" {
			t.Fatalf("collision row: %+v", row)
		}
		switch row.KeyCodePoints {
		case double:
			sawDouble = true
		case "U+05D0 U+0020 U+05D1":
			sawSingle = true
		default:
			t.Fatalf("unexpected collision member %s", row.KeyCodePoints)
		}
	}
	if !sawDouble || !sawSingle {
		t.Fatal("collision group missing a stored key")
	}

	code, human, _ := invoke(t, fx.root, nil, &deps)
	if code != 1 {
		t.Fatalf("human exit %d\n%s", code, human)
	}
	for _, piece := range []string{
		"preflight: FAIL (lexicon)",
		"internal multi-whitespace: 1",
		"leading/trailing whitespace: 1",
		"non-ASCII whitespace: 2",
		"NFC-only: 1",
		"other: 0",
		"collisions: 1",
		"U+00A0",
		"rowid=",
	} {
		if !strings.Contains(human, piece) {
			t.Fatalf("human report missing %q\n%s", piece, human)
		}
	}
}

func TestLessonDoesNotRoundTrip(t *testing.T) {
	body := "window.LESSON = {\"ok\":true};\n"
	fx := buildFixture(t, "apply", "clean", map[string]string{
		"bad/worksheet.js": body,
	})
	deps := wiredDeps(t, func(path string, src []byte) ([]byte, error) {
		if strings.Contains(filepath.ToSlash(path), "/bad/") {
			return []byte("window.LESSON = {\"ok\":false};\n"), nil
		}
		return append([]byte(nil), src...), nil
	})
	code, stdout, stderr := invoke(t, fx.root, []string{"--json"}, &deps)
	if code != 1 {
		t.Fatalf("exit %d\n%s\n%s", code, stdout, stderr)
	}
	raw := mustCheck(t, decodeReport(t, stdout), "lessons")
	if raw.Status != statusFail {
		t.Fatalf("lessons %s %s", raw.Status, raw.Summary)
	}
	var detail lessonDetail
	if err := json.Unmarshal(raw.Detail, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Files != 1 || detail.Mismatches != 1 || len(detail.Examples) != 1 {
		t.Fatalf("detail: %+v", detail)
	}
	ex := detail.Examples[0]
	if ex.Path != "bad/worksheet.js" || !strings.Contains(ex.Reason, "round-trip bytes differ") {
		t.Fatalf("example: %+v", ex)
	}
	stored, err := os.ReadFile(filepath.Join(fx.root, "worksheets", "bad", "worksheet.js"))
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != body {
		t.Fatalf("lesson file changed: %q", stored)
	}
}

func TestPendingDependenciesFailClosed(t *testing.T) {
	fx := buildFixture(t, "apply", "clean", oneLesson())
	code, stdout, stderr := invoke(t, fx.root, []string{"--json"}, nil)
	if code != 1 {
		t.Fatalf("exit %d\n%s\n%s", code, stdout, stderr)
	}
	rep := decodeReport(t, stdout)
	if rep.OK {
		t.Fatal("pending checks exited ok")
	}
	if mustCheck(t, rep, "ledger").Status != statusPass {
		t.Fatal(stdout)
	}
	lex := mustCheck(t, rep, "lexicon")
	les := mustCheck(t, rep, "lessons")
	if lex.Status != statusPending || les.Status != statusPending {
		t.Fatalf("lexicon %s lessons %s", lex.Status, les.Status)
	}
	if !strings.Contains(lex.Summary, "PR #37") || !strings.Contains(les.Summary, "PR #25") {
		t.Fatalf("summaries:\n%s\n%s", lex.Summary, les.Summary)
	}
	if !strings.Contains(stderr, "preflight: FAIL (lexicon, lessons)") {
		t.Fatalf("stderr:\n%s", stderr)
	}
}

func TestUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--help"}, &out, &errb); code != 0 {
		t.Fatalf("help exit %d", code)
	}
	if !strings.Contains(out.String(), "DATA_DIR") {
		t.Fatalf("help: %s", out.String())
	}
	out.Reset()
	errb.Reset()
	if code := run([]string{"--write"}, &out, &errb); code != 2 {
		t.Fatalf("usage exit %d\n%s", code, errb.String())
	}
}

func TestClassifyAndExampleLimit(t *testing.T) {
	exact := nodeExactKey(t)
	rows := []lexiconRow{
		{id: 1, key: "א  ב"},
		{id: 2, key: " אב"},
		{id: 3, key: "\u00A0לחם"},
		{id: 4, key: "\u05E9\u05C1\u05B8"},
		{id: 5, key: "א\tב"},
		{id: 6, key: "שלום"},
	}
	detail, err := classifyLexicon(rows, exact)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, class := range detail.Classes {
		counts[class.Class] = class.Count
	}
	if counts[classInternal] != 1 || counts[classEdge] != 1 || counts[classNonASCII] != 1 ||
		counts[classNFC] != 1 || counts[classOther] != 1 {
		t.Fatalf("counts %+v", counts)
	}
	if detail.Mismatches != 5 {
		t.Fatalf("mismatches %d", detail.Mismatches)
	}
	clean, err := exact("שלום")
	if err != nil {
		t.Fatal(err)
	}
	if clean != "שלום" {
		t.Fatalf("clean exact %q", clean)
	}

	wide := make([]lexiconExample, exampleLimit+5)
	shown, cut := clip(wide, exampleLimit)
	if !cut || len(shown) != exampleLimit {
		t.Fatalf("clip %d cut %v", len(shown), cut)
	}
}

func oneLesson() map[string]string {
	return map[string]string{
		"aleph/worksheet.js": "window.LESSON = {\"ok\":true};\n",
	}
}

func identityRoundTrip(_ string, src []byte) ([]byte, error) {
	return append([]byte(nil), src...), nil
}

type seedRow struct {
	RowID int64   `json:"rowid"`
	Key   string  `json:"key"`
	Lemma *string `json:"lemma_key"`
}

type fixture struct {
	root string
	rows []seedRow
}

func buildFixture(t *testing.T, migrateMode, seedMode string, lessons map[string]string) fixture {
	t.Helper()
	root := t.TempDir()
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, body := range lessons {
		path := filepath.Join(root, "worksheets", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	script := filepath.Join("..", "..", "internal", "db", "testdata", "apply-migrations.js")
	cmd := exec.Command("node", script, migrateMode, data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("apply %s: %v\n%s", migrateMode, err, out)
	}
	var rows []seedRow
	if seedMode != "" {
		seedPath := filepath.Join(root, "seed.js")
		if err := os.WriteFile(seedPath, []byte(seedJS), 0o644); err != nil {
			t.Fatal(err)
		}
		seed := exec.Command("node", seedPath, filepath.Join(data, "hebrew.sqlite"), seedMode)
		var seedErr bytes.Buffer
		seed.Stderr = &seedErr
		out, err := seed.Output()
		if err != nil {
			t.Fatalf("seed %s: %v\n%s\n%s", seedMode, err, out, seedErr.String())
		}
		if err := json.Unmarshal(out, &rows); err != nil {
			t.Fatalf("seed json: %v\n%s", err, out)
		}
	}
	return fixture{root: root, rows: rows}
}

func invoke(t *testing.T, root string, args []string, deps *Deps) (int, string, string) {
	t.Helper()
	t.Setenv("HEBREW_ROOT", root)
	t.Setenv("DATA_DIR", filepath.Join(root, "data"))
	t.Setenv("LESSONS_DIR", filepath.Join(root, "worksheets"))
	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("TRUSTED_PROXIES", "")
	t.Setenv("COOKIE_SECURE", "")
	before := snapshotTree(t, root)
	var stdout, stderr bytes.Buffer
	var code int
	if deps == nil {
		code = run(args, &stdout, &stderr)
	} else {
		code = runWith(args, &stdout, &stderr, *deps)
	}
	after := snapshotTree(t, root)
	assertUnchanged(t, before, after)
	return code, stdout.String(), stderr.String()
}

func snapshotTree(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[rel] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out[filepath.Join("data", "hebrew.sqlite")]; !ok {
		t.Fatal("snapshot missing hebrew.sqlite")
	}
	return out
}

func assertUnchanged(t *testing.T, before, after map[string][]byte) {
	t.Helper()
	for rel, prev := range before {
		got, ok := after[rel]
		if !ok {
			t.Errorf("removed %s", rel)
			continue
		}
		if !bytes.Equal(prev, got) {
			t.Errorf("changed %s (%d bytes -> %d)", rel, len(prev), len(got))
		}
	}
	for rel := range after {
		if _, ok := before[rel]; ok {
			continue
		}
		base := filepath.Base(rel)
		if base == "hebrew.sqlite-shm" || base == "hebrew.sqlite-wal" {
			continue
		}
		t.Errorf("created %s", rel)
	}
}

type reportJSON struct {
	OK     bool        `json:"ok"`
	Checks []checkJSON `json:"checks"`
}

type checkJSON struct {
	Name    string          `json:"name"`
	Status  string          `json:"status"`
	Summary string          `json:"summary"`
	Detail  json.RawMessage `json:"detail"`
}

func decodeReport(t *testing.T, stdout string) reportJSON {
	t.Helper()
	var rep reportJSON
	if err := json.Unmarshal([]byte(stdout), &rep); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	return rep
}

func mustCheck(t *testing.T, rep reportJSON, name string) checkJSON {
	t.Helper()
	for _, check := range rep.Checks {
		if check.Name == name {
			return check
		}
	}
	t.Fatalf("no check %s", name)
	return checkJSON{}
}

func rowID(t *testing.T, rows []seedRow, key string) int64 {
	t.Helper()
	for _, row := range rows {
		if row.Key == key {
			return row.RowID
		}
		if row.Lemma != nil && *row.Lemma == key {
			return row.RowID
		}
	}
	t.Fatalf("seed has no key %q", key)
	return 0
}

func wiredDeps(t *testing.T, roundTrip func(string, []byte) ([]byte, error)) Deps {
	t.Helper()
	return Deps{ExactKey: nodeExactKey(t), RoundTrip: roundTrip}
}

func mustNode(t *testing.T, key string) string {
	t.Helper()
	got, err := nodeExactKey(t)(key)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func nodeExactKey(t *testing.T) func(string) (string, error) {
	t.Helper()
	vocab, err := filepath.Abs(filepath.Join("..", "..", "..", "server", "vocab-index.js"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(vocab); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	cache := map[string]string{}
	return func(s string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if v, ok := cache[s]; ok {
			return v, nil
		}
		cmd := exec.Command("node", "-e", nodeExactKeyJS, vocab)
		cmd.Stdin = strings.NewReader(s)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("node exactKey: %w\n%s", err, stderr.String())
		}
		cache[s] = string(out)
		return cache[s], nil
	}
}

const nodeExactKeyJS = `
const {exactKey} = require(process.argv[1]);
const chunks = [];
process.stdin.on('data', (c) => chunks.push(c));
process.stdin.on('end', () => {
  process.stdout.write(exactKey(Buffer.concat(chunks).toString('utf8')));
});
`

const seedJS = `
'use strict';
const {DatabaseSync} = require('node:sqlite');
const file = process.argv[2];
const mode = process.argv[3];
const db = new DatabaseSync(file);
if (mode === 'drop11') {
  db.prepare('DELETE FROM schema_migrations WHERE version = 11').run();
}
if (mode === 'clean' || mode === 'bad') {
  const ins = db.prepare(
    'INSERT INTO lexicon_entries (' +
    'key, kind, lemma_key, gloss_en, pos, root, binyan, root_class, ' +
    'tense, cell, translit, accept_json, verified, verified_by, ' +
    'source, rev, updated_at) VALUES (' +
    '?, ?, ?, NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, ' +
    "'[]', 0, NULL, 'admin', 0, '2026-10-18T00:00:00Z')"
  );
  const add = (key, kind, lemma) => ins.run(key, kind, lemma);
  add('\u05E9\u05DC\u05D5\u05DD', 'lemma', null);
  if (mode === 'bad') {
    add('\u05D0  \u05D1', 'phrase', null);
    add('\u05D0 \u05D1', 'phrase', null);
    add('\u00A0\u05DC\u05D7\u05DD', 'lemma', null);
    add('\u05E9\u05C1\u05B8', 'lemma', null);
    add('\u05D2\u05DE\u05E8', 'form', '\u00A0\u05D0');
    add(' \u05D0\u05D1', 'phrase', null);
  }
}
const rows = db.prepare(
  'SELECT rowid AS rowid, key, lemma_key FROM lexicon_entries ORDER BY rowid'
).all();
db.close();
process.stdout.write(JSON.stringify(rows));
`
