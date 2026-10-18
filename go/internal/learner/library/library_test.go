package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/go/internal/db"
	_ "modernc.org/sqlite"
)

func TestMain(m *testing.M) {
	time.Local = time.UTC
	os.Exit(m.Run())
}

func TestExactKeyAndOrder(t *testing.T) {
	if got := exactKey(" \uFEFF  \u05e9\u05dc\u05d5\u05dd \t "); got != "\u05e9\u05dc\u05d5\u05dd" {
		t.Fatalf("exactKey %q", got)
	}
	if !strings.Contains(exactKey("a\u0085b"), "\u0085") {
		t.Fatal("U+0085 must be kept; it is not JavaScript whitespace")
	}
	// Shin + qamats + shin-dot + mem + patah + etnachta + ayin.
	pointed := exactKey(" \uFEFF \u05E9\u05B8\u05C1\u05DE\u05B7\u0597\u05E2 \t ")
	for _, r := range []rune{'\u05B8', '\u05C1', '\u05B7', '\u0597'} {
		if !strings.ContainsRune(pointed, r) {
			t.Fatalf("nikud or cantillation dropped from %q", pointed)
		}
	}
	if utf16Len("\U0001F600") != 2 || utf16Len("\U0001D11E") != 2 {
		t.Fatal("supplementary characters must be two UTF-16 units")
	}
	if utf16Len(pointed) != utf8.RuneCountInString(pointed) {
		t.Fatal("pointed Hebrew is in the BMP")
	}
	emoji := "\u05E9\U0001F600"
	astral := "\u05E9\U0001D11E"
	high := "\u05E9\uF8FF"
	if !jsLess(astral, emoji) || !jsLess(emoji, high) {
		t.Fatal("UTF-16 order is astral, emoji, then U+F8FF")
	}
	if !(high < astral && astral < emoji) {
		t.Fatal("UTF-8 byte order no longer differs from UTF-16; the golden would not catch a sort regression")
	}
}

func TestCursorRoundTrip(t *testing.T) {
	word := "\u05e9\u05dc\u05d5\u05dd"
	encoded, err := encodeCursor(word)
	if err != nil {
		t.Fatal(err)
	}
	body, err := decodeCursor(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if body.Key != word || len(body.Ord) != 1 || body.Ord[0] != word || body.V != 1 {
		t.Fatalf("body %+v", body)
	}
	if _, err := decodeCursor("not-json"); err == nil {
		t.Fatal("garbage cursor was accepted")
	}
	if _, err := decodeCursor(""); err != nil {
		t.Fatal(err)
	}
}

func TestLibraryMatchesNode(t *testing.T) {
	dataDir := t.TempDir()
	lessons := t.TempDir()
	outPath := filepath.Join(dataDir, "library.jsonl")
	cmd := exec.Command("node", filepath.Join("testdata", "golden.js"), dataDir, lessons, outPath)
	cmd.Env = nodeEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node golden: %v\n%s", err, output)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}

	var meta goldenMeta
	var lists []listCase
	var suspends []suspendCase
	var added addCase
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var kind struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal([]byte(line), &kind); err != nil {
			t.Fatal(err)
		}
		switch kind.Kind {
		case "meta":
			if err := json.Unmarshal([]byte(line), &meta); err != nil {
				t.Fatal(err)
			}
		case "list":
			var item listCase
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				t.Fatal(err)
			}
			lists = append(lists, item)
		case "suspend":
			var item suspendCase
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				t.Fatal(err)
			}
			suspends = append(suspends, item)
		case "add":
			if err := json.Unmarshal([]byte(line), &added); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("kind %s", kind.Kind)
		}
	}

	now, err := time.Parse(time.RFC3339Nano, meta.Now)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.OpenReadOnly(meta.DB)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	store := &Store{DB: conn, LessonsDir: meta.Lessons, Now: func() time.Time { return now }}

	var full []Item
	for _, item := range lists {
		t.Run(item.ID, func(t *testing.T) {
			store.Scheduler = item.Scheduler
			filter, limit, err := ParseQuery(item.Query)
			if !item.OK {
				var op *OpError
				if !errors.As(err, &op) || op.Code != item.Error {
					t.Fatalf("parse err %v want %s", err, item.Error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, _, err := store.List(context.Background(), item.UserID, filter, "", limit)
			if err != nil {
				t.Fatal(err)
			}
			if !itemsEqual(got, item.Words) {
				gb, _ := json.Marshal(got)
				wb, _ := json.Marshal(item.Words)
				t.Fatalf("got %s\nwant %s", gb, wb)
			}
			if item.ID == "all" {
				full = got
			}
		})
	}

	if len(full) < 4 {
		t.Fatalf("full list too small: %d", len(full))
	}
	assertUnicodeWords(t, full)
	store.Scheduler = "ladder"
	var paged []Item
	cursor := ""
	for {
		page, next, err := store.List(context.Background(), meta.Learner, Filter{}, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		paged = append(paged, page...)
		if next == "" {
			break
		}
		if next == cursor {
			t.Fatal("cursor did not advance")
		}
		cursor = next
	}
	if !itemsEqual(paged, full) {
		t.Fatalf("paged list != full scan")
	}

	assertSchema(t, conn, meta.Schema)
	if !added.OK || added.GlossEN != "new" {
		t.Fatalf("add proof %+v", added)
	}
	for _, column := range added.Columns {
		if column == "translit" {
			t.Fatal("user_words stored translit; EditWord can be implemented")
		}
	}

	base := copyDB(t, meta.DB)
	writer := openRW(t, base)
	defer writer.Close()
	mut := &Store{DB: writer, LessonsDir: meta.Lessons, Now: func() time.Time { return now }, Scheduler: "ladder"}
	for _, item := range suspends {
		t.Run("suspend-"+item.ID, func(t *testing.T) {
			word, err := mut.SetSuspended(context.Background(), item.UserID, item.Key, item.Suspended)
			if item.OK {
				if err != nil {
					t.Fatal(err)
				}
				if word != item.Word {
					t.Fatalf("word %q want %q", word, item.Word)
				}
			} else {
				var op *OpError
				if !errors.As(err, &op) || op.Code != item.Error || op.Status != item.Status {
					t.Fatalf("err %v want %s %d", err, item.Error, item.Status)
				}
			}
			if item.Audit != item.AuditBefore {
				t.Fatalf("node audit changed %d -> %d", item.AuditBefore, item.Audit)
			}
			got := readFlags(t, writer, meta.Learner)
			if !reflect.DeepEqual(got, item.Flags) {
				t.Fatalf("flags got %#v want %#v", got, item.Flags)
			}
			var audit int
			if err := writer.QueryRowContext(context.Background(), `SELECT count(*) FROM audit_log`).Scan(&audit); err != nil {
				t.Fatal(err)
			}
			if audit != item.AuditBefore {
				t.Fatalf("go audit %d want %d", audit, item.AuditBefore)
			}
		})
	}

	t.Run("cursor stable under inserts", func(t *testing.T) {
		fresh := openRW(t, copyDB(t, meta.DB))
		defer fresh.Close()
		stable := &Store{DB: fresh, LessonsDir: meta.Lessons, Now: func() time.Time { return now }, Scheduler: "ladder"}
		page, cursor, err := stable.List(context.Background(), meta.Learner, Filter{}, "", 2)
		if err != nil || len(page) != 2 || cursor == "" {
			t.Fatalf("page %v cursor %q err %v", page, cursor, err)
		}
		before := "\u05d0"
		after := page[1].Word + "\u05d0"
		insertWord(t, fresh, meta.Learner, before, "before")
		insertWord(t, fresh, meta.Learner, after, "after")
		rest, _, err := stable.List(context.Background(), meta.Learner, Filter{}, cursor, 50)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range rest {
			if item.Word == page[0].Word || item.Word == page[1].Word || item.Word == before {
				t.Fatalf("continuation included %s", item.Word)
			}
		}
		foundAfter := false
		for _, item := range rest {
			if item.Word == after {
				foundAfter = true
			}
		}
		if !foundAfter {
			t.Fatal("insert after the cursor was missing from the continuation")
		}
		rescanned, _, err := stable.List(context.Background(), meta.Learner, Filter{}, "", 200)
		if err != nil {
			t.Fatal(err)
		}
		var words []string
		for _, item := range rescanned {
			words = append(words, item.Word)
		}
		if !jsLess(before, page[0].Word) {
			t.Fatalf("%s should sort before %s", before, page[0].Word)
		}
		if !containsWord(words, before) || !containsWord(words, after) {
			t.Fatalf("rescan %v", words)
		}
	})
}

func assertUnicodeWords(t *testing.T, items []Item) {
	t.Helper()
	var sawPointed, sawEmoji, sawAstral, sawHigh bool
	for _, item := range items {
		word := item.Word
		if strings.ContainsRune(word, '\u0597') && strings.ContainsRune(word, '\u05B8') && strings.ContainsRune(word, '\u05C1') {
			sawPointed = true
		}
		if strings.ContainsRune(word, '\U0001F600') {
			sawEmoji = true
		}
		if strings.ContainsRune(word, '\U0001D11E') {
			sawAstral = true
		}
		if strings.ContainsRune(word, '\uF8FF') {
			sawHigh = true
		}
	}
	if !sawPointed || !sawEmoji || !sawAstral || !sawHigh {
		t.Fatalf("golden list missing unicode rows pointed=%v emoji=%v astral=%v high=%v", sawPointed, sawEmoji, sawAstral, sawHigh)
	}
}

func assertSchema(t *testing.T, conn *sql.DB, schema map[string][]string) {
	t.Helper()
	userWords := schema["user_words"]
	if !containsWord(userWords, "gloss_en") || containsWord(userWords, "translit") || containsWord(userWords, "gloss_override") {
		t.Fatalf("user_words columns %v", userWords)
	}
	review := schema["review_items"]
	if containsWord(review, "gloss_en") || containsWord(review, "translit") {
		t.Fatalf("review_items columns %v", review)
	}
	lex := schema["lexicon_entries"]
	if !containsWord(lex, "translit") || !containsWord(lex, "gloss_en") || containsWord(lex, "user_id") {
		t.Fatalf("lexicon_entries columns %v", lex)
	}
}

type goldenMeta struct {
	DB      string              `json:"db"`
	Lessons string              `json:"lessons"`
	Now     string              `json:"now"`
	Learner int64               `json:"learner"`
	Other   int64               `json:"other"`
	Schema  map[string][]string `json:"schema"`
}

type listCase struct {
	ID        string            `json:"id"`
	UserID    int64             `json:"userId"`
	Scheduler string            `json:"scheduler"`
	Query     map[string]string `json:"query"`
	OK        bool              `json:"ok"`
	Error     string            `json:"error"`
	Words     []Item            `json:"words"`
}

type suspendCase struct {
	ID          string `json:"id"`
	UserID      int64  `json:"userId"`
	Key         string `json:"key"`
	Suspended   bool   `json:"suspended"`
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
	Status      int    `json:"status"`
	Word        string `json:"word"`
	Audit       int    `json:"audit"`
	AuditBefore int    `json:"auditBefore"`
	Flags       []flag `json:"flags"`
}

type flag struct {
	Word      string `json:"word"`
	Direction string `json:"direction"`
	Suspended int    `json:"suspended"`
}

type addCase struct {
	OK      bool     `json:"ok"`
	GlossEN string   `json:"gloss_en"`
	Columns []string `json:"columns"`
}

func itemsEqual(got, want []Item) bool {
	gb, err1 := json.Marshal(got)
	wb, err2 := json.Marshal(want)
	if err1 != nil || err2 != nil {
		return false
	}
	var g, w any
	if json.Unmarshal(gb, &g) != nil || json.Unmarshal(wb, &w) != nil {
		return false
	}
	return deepEqual(g, w)
}

func deepEqual(a, b any) bool {
	switch av := a.(type) {
	case float64:
		bv, ok := b.(float64)
		if !ok {
			return false
		}
		diff := math.Abs(av - bv)
		if diff <= 1e-9 {
			return true
		}
		scale := math.Max(math.Abs(av), math.Abs(bv))
		return scale > 0 && diff/scale <= 1e-9
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !deepEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for key, value := range av {
			if !deepEqual(value, bv[key]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

func readFlags(t *testing.T, conn *sql.DB, userID int64) []flag {
	t.Helper()
	rows, err := conn.QueryContext(context.Background(),
		`SELECT word, direction, suspended FROM review_items WHERE user_id = ? ORDER BY word, direction`, userID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []flag
	for rows.Next() {
		var item flag
		if err := rows.Scan(&item.Word, &item.Direction, &item.Suspended); err != nil {
			t.Fatal(err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func insertWord(t *testing.T, conn *sql.DB, userID int64, word, gloss string) {
	t.Helper()
	_, err := conn.ExecContext(context.Background(),
		`INSERT INTO user_words (user_id, word, source, added_at, gloss_en) VALUES (?, ?, 'self', ?, ?)`,
		userID, word, "2026-01-15T12:00:00.000Z", gloss)
	if err != nil {
		t.Fatal(err)
	}
}

func copyDB(t *testing.T, src string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), "hebrew.sqlite")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

func openRW(t *testing.T, path string) *sql.DB {
	t.Helper()
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	q := url.Values{}
	q.Set("_busy_timeout", "5000")
	q.Set("_foreign_keys", "on")
	q.Set("_journal_mode", "WAL")
	q.Set("_txlock", "immediate")
	u.RawQuery = q.Encode()
	conn, err := sql.Open("sqlite", u.String())
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	if err := conn.Ping(); err != nil {
		t.Fatal(err)
	}
	return conn
}

func containsWord(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func nodeEnv() []string {
	out := make([]string, 0, len(os.Environ())+1)
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "TZ=") || strings.HasPrefix(value, "SRS_SCHEDULER=") {
			continue
		}
		out = append(out, value)
	}
	return append(out, "TZ=UTC", "SRS_SCHEDULER=ladder")
}
