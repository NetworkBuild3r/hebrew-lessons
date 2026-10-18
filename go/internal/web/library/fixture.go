package library

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// Demo is an in-memory library used under the dev flag and in tests.
// It does not open the database. Replace it with learner/library when that
// package lands; the handler only depends on Lister.
type Demo struct{}

// cursorBody is the fixture's opaque cursor: base64url JSON of the ordering
// tuple plus the word key. Node currently sorts by the word alone, so the
// tuple is that one key. The page treats the whole string as opaque.
type cursorBody struct {
	Ord  []string `json:"ord"`
	Word string   `json:"word"`
}

// List filters and pages demoItems. userID is ignored: one shared shelf until
// sessions exist. limit follows the library rules (default 50, max 200).
func (Demo) List(_ context.Context, _ int64, filter Filter, cursor string, limit int) ([]Item, string, error) {
	after, err := decodeCursor(cursor)
	if err != nil {
		return nil, "", err
	}
	if limit < 1 {
		limit = PageSize
	}
	if limit > MaxPage {
		limit = MaxPage
	}
	sorted := append([]Item(nil), demoItems...)
	sortItems(sorted)
	var matched []Item
	for _, item := range sorted {
		if !matches(item, filter, after) {
			continue
		}
		matched = append(matched, item)
	}
	if len(matched) > limit {
		page := matched[:limit]
		return page, encodeCursor(page[len(page)-1].Word), nil
	}
	return matched, "", nil
}

func sortItems(items []Item) {
	// Same order as Array.prototype.sort with (a < b ? -1 : a > b ? 1 : 0)
	// on the word key. Hebrew is in the BMP, so UTF-16 code units and Go's
	// byte-wise string compare agree.
	for i := 1; i < len(items); i++ {
		j := i
		for j > 0 && items[j].Word < items[j-1].Word {
			items[j], items[j-1] = items[j-1], items[j]
			j--
		}
	}
}

func matches(item Item, f Filter, after string) bool {
	if after != "" && item.Word <= after {
		return false
	}
	if f.Q != "" {
		term := strings.ToLower(collapse(f.Q))
		hay := strings.ToLower(strings.Join([]string{item.Word, item.GlossEN, item.Note, item.Translit}, "\n"))
		if !strings.Contains(hay, term) {
			return false
		}
	}
	if f.Status != "" {
		stages := map[string]bool{}
		if item.HeEn != nil {
			stages[item.HeEn.Stage] = true
		}
		if item.EnHe != nil {
			stages[item.EnHe.Stage] = true
		}
		if !stages[f.Status] {
			return false
		}
	}
	switch f.Weak {
	case "1":
		if item.Weak != "weak" && item.Weak != "very_weak" {
			return false
		}
	case "weak":
		if item.Weak != "weak" {
			return false
		}
	case "very_weak":
		if item.Weak != "very_weak" {
			return false
		}
	}
	if f.POS != "" && item.POS != f.POS {
		return false
	}
	if f.Binyan != "" && item.Binyan != f.Binyan {
		return false
	}
	if f.Tense != "" && !hasTense(item, f.Tense) {
		return false
	}
	if f.Source == "lesson" {
		if !hasLessonSource(item.Sources) {
			return false
		}
	} else if f.Source != "" && !contains(item.Sources, f.Source) {
		return false
	}
	if f.Suspended == "1" && !item.Suspended {
		return false
	}
	if f.Suspended == "0" && item.Suspended {
		return false
	}
	return true
}

func hasTense(item Item, tense string) bool {
	if item.Tense == tense {
		return true
	}
	for _, form := range item.Paradigm {
		if form.Tense == tense {
			return true
		}
	}
	return false
}

func hasLessonSource(sources []string) bool {
	for _, source := range sources {
		if source != "self" && source != "ai" && source != "lesson" {
			return true
		}
	}
	return false
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func encodeCursor(word string) string {
	body, _ := json.Marshal(cursorBody{Ord: []string{word}, Word: word})
	return base64.RawURLEncoding.EncodeToString(body)
}

func decodeCursor(cursor string) (string, error) {
	if cursor == "" {
		return "", nil
	}
	if len(cursor) > 512 || !utf8.ValidString(cursor) {
		return "", ErrInvalidCursor
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", ErrInvalidCursor
	}
	var body cursorBody
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", ErrInvalidCursor
	}
	if body.Word == "" || len(body.Ord) == 0 || body.Ord[0] != body.Word {
		return "", ErrInvalidCursor
	}
	if utf8.RuneCountInString(body.Word) > 60 {
		return "", ErrInvalidCursor
	}
	return body.Word, nil
}

func dir(stage, bucket string) *Direction {
	return &Direction{Stage: stage, StrengthBucket: bucket}
}

// demoItems is a small shelf for the dev server and screenshots.
// Word strings are stored pointed, exactly as a learner key would be.
// שָׁלוֹם is first in the slice and last in code-unit order, so a List that
// forgets to sort does not accidentally match Node.
var demoItems = []Item{
	{
		Word: "שָׁלוֹם", GlossEN: "peace", Translit: "shalom", POS: "noun",
		HeEn: dir("longTerm", "solid"), EnHe: dir("longTerm", "solid"),
		StrengthBucket: "solid", Sources: []string{"greetings"},
	},
	{
		Word: "אֹכֶל", GlossEN: "food", Translit: "ochel", POS: "noun",
		HeEn: dir("longTerm", "solid"), EnHe: dir("longTerm", "solid"),
		StrengthBucket: "solid", Sources: []string{"pantry"},
	},
	{
		Word: "בְּבַקָּשָׁה", GlossEN: "please", Translit: "bevakasha", POS: "particle",
		HeEn: dir("learning", "steady"), EnHe: dir("learning", "steady"),
		StrengthBucket: "steady", Sources: []string{"self"},
	},
	{
		Word: "בַּיִת", GlossEN: "house", Translit: "bayit", POS: "noun",
		HeEn: dir("learning", "steady"), EnHe: dir("new", "shaky"),
		StrengthBucket: "shaky", Sources: []string{"home"},
	},
	{
		Word: "הִסְבִּיר", GlossEN: "explained", Translit: "hisbir", POS: "verb",
		Root: "ס.ב.ר", Binyan: "hifil", RootClass: "regular",
		HeEn: dir("learning", "shaky"), EnHe: dir("new", "fragile"),
		StrengthBucket: "fragile", Weak: "weak", Sources: []string{"self"},
		Paradigm: []Form{
			{Tense: "past", Cell: "1s", He: "הִסְבַּרְתִּי", Gloss: "I explained"},
			{Tense: "present", Cell: "ms", He: "מַסְבִּיר", Gloss: "explains"},
		},
	},
	{
		Word: "הִתְלַבֵּשׁ", GlossEN: "got dressed", Translit: "hitlabesh", POS: "verb",
		Root: "ל.ב.ש", Binyan: "hitpael", RootClass: "regular",
		HeEn: dir("learning", "steady"), EnHe: dir("learning", "steady"),
		StrengthBucket: "steady", Sources: []string{"ai"},
	},
	{
		Word: "חָבֵר", GlossEN: "friend", Translit: "chaver", POS: "noun",
		HeEn: dir("learning", "shaky"), EnHe: dir("learning", "shaky"),
		StrengthBucket: "shaky", Suspended: true, Sources: []string{"self"},
	},
	{
		Word: "כָּתַב", GlossEN: "wrote", Translit: "katav", POS: "verb",
		Root: "כ.ת.ב", Binyan: "paal", RootClass: "regular", Tense: "past",
		HeEn: dir("new", "fragile"), EnHe: dir("new", "fragile"),
		StrengthBucket: "fragile", Weak: "very_weak", Sources: []string{"letters"},
	},
	{
		Word: "לְהִתְחַדֵּשׁ", GlossEN: "to renew", Translit: "le hitchadesh", POS: "verb",
		Root: "ח.ד.ש", Binyan: "hitpael", RootClass: "regular", Tense: "infinitive",
		HeEn: dir("new", "fragile"), EnHe: dir("learning", "shaky"),
		StrengthBucket: "fragile", Sources: []string{"self"},
	},
	{
		Word: "סֵפֶר", GlossEN: "book", Translit: "sefer", POS: "noun",
		HeEn: dir("learning", "steady"), EnHe: dir("longTerm", "solid"),
		StrengthBucket: "steady", Note: "from the library lesson", Sources: []string{"reading"},
	},
}
