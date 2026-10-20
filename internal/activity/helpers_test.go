package activity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time          { return f.t }
func (f *fakeClock) Advance(d time.Duration) { f.t = f.t.Add(d) }

// fixedBands is a BandSource that gives every listed lexeme a band.
type fixedBands map[int64]int

func (b fixedBands) LexemeBands(context.Context) (map[int64]int, error) {
	out := make(map[int64]int, len(b))
	for k, v := range b {
		out[k] = v
	}
	return out, nil
}

func openStore(t *testing.T) (*db.Store, *fakeClock) {
	t.Helper()
	clk := &fakeClock{t: time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)}
	store, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "hebrew.sqlite"), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, clk
}

func openEngine(t *testing.T, opts ...Option) (*Engine, *db.Store, *fakeClock) {
	t.Helper()
	store, clk := openStore(t)
	opts = append([]Option{WithSeed(1)}, opts...)
	return New(store, clk, nil, opts...), store, clk
}

func addUser(t *testing.T, store *db.Store, name string) int64 {
	t.Helper()
	id, err := store.InsertUser(context.Background(), name+"@example.com", "sub-"+name, "learner")
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// audioFor makes a clip row for a word and returns its id.
func audioFor(t *testing.T, store *db.Store, text string) string {
	t.Helper()
	sum := sha256.Sum256([]byte("clip:" + text))
	id := hex.EncodeToString(sum[:])
	_, err := store.InsertAudio(context.Background(), db.Asset{
		ID: id, TextNFC: text, Voice: "v", Model: "m", Bytes: 10, CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func addWord(t *testing.T, store *db.Store, text, gloss, pos string, audio bool) int64 {
	t.Helper()
	var audioID any
	if audio {
		audioID = audioFor(t, store, text)
	}
	res, err := store.ExecContext(context.Background(),
		`INSERT INTO lexemes (text_nfc, gloss_en, pos, audio_id) VALUES (?, ?, ?, ?)`, text, gloss, pos, audioID)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// fakeEnglish makes the n-th distinct made-up gloss. Two of them never share a word.
func fakeEnglish(n int) string {
	const cons, vowels = "bdfgklmnprtvz", "aeiou"
	var sb strings.Builder
	n++
	for n > 0 {
		sb.WriteByte(cons[n%len(cons)])
		n /= len(cons)
		sb.WriteByte(vowels[n%len(vowels)])
		n /= len(vowels)
	}
	return sb.String() + "o"
}

// fakeHebrew makes the n-th distinct string of Hebrew letters, from the 22 letters.
func fakeHebrew(n int) string {
	letters := []rune("אבגדהוזחטיכלמנסעפצקרשת")
	out := []rune{}
	n++
	for n > 0 {
		out = append(out, letters[n%len(letters)])
		n /= len(letters)
	}
	return string(out) + "ה"
}

type lexicon struct {
	ids   []int64
	words map[int64]Word
}

// fillLexicon adds n words. Word i has pos noun when i is even and verb when odd, a clip, and
// a gloss no other word shares. Band, when given, assigns bands by index.
func fillLexicon(t *testing.T, store *db.Store, n int, audio func(i int) bool) lexicon {
	t.Helper()
	lex := lexicon{words: map[int64]Word{}}
	for i := 0; i < n; i++ {
		pos := "noun"
		if i%2 == 1 {
			pos = "verb"
		}
		text, gloss := fakeHebrew(i), fakeEnglish(i)
		hasAudio := audio == nil || audio(i)
		id := addWord(t, store, text, gloss, pos, hasAudio)
		lex.ids = append(lex.ids, id)
		w := Word{ID: id, Text: text, Gloss: gloss, POS: pos, Band: -1}
		if hasAudio {
			w.AudioID = audioFor(t, store, text)
		}
		lex.words[id] = w
	}
	return lex
}

func countRows(t *testing.T, store *db.Store, query string, args ...any) int {
	t.Helper()
	conn, err := store.DB()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func mustf(t *testing.T, err error, format string, args ...any) {
	t.Helper()
	if err != nil {
		t.Fatalf(format+": %v", append(args, err)...)
	}
}

var _ = fmt.Sprintf
