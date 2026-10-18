package library

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// Demo is an in-memory library used under the dev flag and in tests.
// It does not open the database. Replace the import with
// go/internal/learner/library when that package is on this branch.
type Demo struct{}

// cursorBody matches learner/library: base64url JSON
// {"v":1,"ord":["<word>"],"key":"<word>"}. The page treats the string as opaque.
type cursorBody struct {
	V   int      `json:"v"`
	Ord []string `json:"ord"`
	Key string   `json:"key"`
}

const cursorVersion = 1

// List filters and pages demoItems. userID is ignored: one shared shelf until
// sessions exist. limit follows the library rules (default 50, max 200).
func (Demo) List(_ context.Context, _ int64, filter Filter, cursor string, limit int) ([]Item, string, error) {
	normalized, limit, err := normalize(filter, limit)
	if err != nil {
		return nil, "", err
	}
	after, err := decodeCursor(cursor)
	if err != nil {
		return nil, "", err
	}
	sorted := append([]Item(nil), demoItems...)
	sortItems(sorted)
	var matched []Item
	for _, item := range sorted {
		if !matches(item, normalized, after) {
			continue
		}
		matched = append(matched, item)
	}
	if len(matched) > limit {
		page := matched[:limit]
		next, err := encodeCursor(page[len(page)-1].Word)
		if err != nil {
			return nil, "", err
		}
		return page, next, nil
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
		note := ""
		if item.Note != nil {
			note = *item.Note
		}
		hay := strings.ToLower(item.Word + "\n" + item.GlossEN + "\n" + note + "\n" + item.Translit)
		if !strings.Contains(hay, f.Q) {
			return false
		}
	}
	if f.Status != "" {
		stages := map[string]bool{}
		if item.HeEN != nil {
			stages[item.HeEN.Stage] = true
		}
		if item.EnHE != nil {
			stages[item.EnHE.Stage] = true
		}
		if !stages[f.Status] {
			return false
		}
	}
	weak := ""
	if item.Weak != nil {
		weak = *item.Weak
	}
	switch f.Weak {
	case "any":
		if weak != "weak" && weak != "very_weak" {
			return false
		}
	case "none":
		if weak != "" {
			return false
		}
	case "weak":
		if weak != "weak" {
			return false
		}
	case "very_weak":
		if weak != "very_weak" {
			return false
		}
	}
	if f.POS != "" && (item.POS == nil || *item.POS != f.POS) {
		return false
	}
	if f.Binyan != "" && (item.Binyan == nil || *item.Binyan != f.Binyan) {
		return false
	}
	if f.Tense != "" && !hasTense(item.Word, f.Tense) {
		return false
	}
	if f.Source == "lesson" {
		if !hasLessonSource(item.Sources) {
			return false
		}
	} else if f.Source != "" && !contains(item.Sources, f.Source) {
		return false
	}
	if f.Suspended != nil {
		if *f.Suspended != item.Suspended {
			return false
		}
	}
	return true
}

func hasTense(word, tense string) bool {
	for _, got := range demoTenses[word] {
		if got == tense {
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

func encodeCursor(word string) (string, error) {
	raw, err := json.Marshal(cursorBody{V: cursorVersion, Ord: []string{word}, Key: word})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeCursor(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if len(s) > 512 || !utf8.ValidString(s) {
		return "", fail("invalid_cursor", 400)
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", fail("invalid_cursor", 400)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var body cursorBody
	if err := dec.Decode(&body); err != nil {
		return "", fail("invalid_cursor", 400)
	}
	if dec.More() {
		return "", fail("invalid_cursor", 400)
	}
	if body.V != cursorVersion || body.Key == "" || len(body.Ord) != 1 || body.Ord[0] != body.Key {
		return "", fail("invalid_cursor", 400)
	}
	if utf8.RuneCountInString(body.Key) > 60 {
		return "", fail("invalid_cursor", 400)
	}
	again, err := json.Marshal(body)
	if err != nil {
		return "", fail("invalid_cursor", 400)
	}
	if base64.RawURLEncoding.EncodeToString(again) != s {
		return "", fail("invalid_cursor", 400)
	}
	return body.Key, nil
}

func dir(stage, bucket string) *Direction {
	return &Direction{Stage: stage, StrengthBucket: strPtr(bucket)}
}

func strPtr(s string) *string { return &s }

// demoTenses stands in for the lexicon forms List checks. Item has no tense field.
var demoTenses = map[string][]string{
	"הִסְבִּיר":     {"past", "present"},
	"כָּתַב":        {"past"},
	"לְהִתְחַדֵּשׁ": {"infinitive"},
}

// demoItems is a small shelf for the dev server and screenshots.
// Word strings are stored pointed, exactly as a learner key would be.
// שָׁלוֹם is first in the slice and last in code-unit order, so a List that
// forgets to sort does not accidentally match Node.
var demoItems = []Item{
	{
		Word: "שָׁלוֹם", GlossEN: "peace", Translit: "shalom", POS: strPtr("noun"),
		HeEN: dir("longTerm", "solid"), EnHE: dir("longTerm", "solid"),
		StrengthBucket: strPtr("solid"), Sources: []string{"greetings"},
	},
	{
		Word: "אֹכֶל", GlossEN: "food", Translit: "ochel", POS: strPtr("noun"),
		HeEN: dir("longTerm", "solid"), EnHE: dir("longTerm", "solid"),
		StrengthBucket: strPtr("solid"), Sources: []string{"pantry"},
	},
	{
		Word: "בְּבַקָּשָׁה", GlossEN: "please", Translit: "bevakasha", POS: strPtr("particle"),
		HeEN: dir("learning", "steady"), EnHE: dir("learning", "steady"),
		StrengthBucket: strPtr("steady"), Sources: []string{"self"},
	},
	{
		Word: "בַּיִת", GlossEN: "house", Translit: "bayit", POS: strPtr("noun"),
		HeEN: dir("learning", "steady"), EnHE: dir("new", "shaky"),
		StrengthBucket: strPtr("shaky"), Sources: []string{"home"},
	},
	{
		Word: "הִסְבִּיר", GlossEN: "explained", Translit: "hisbir", POS: strPtr("verb"),
		Root: strPtr("ס.ב.ר"), Binyan: strPtr("hifil"), RootClass: strPtr("regular"),
		HeEN: dir("learning", "shaky"), EnHE: dir("new", "fragile"),
		StrengthBucket: strPtr("fragile"), Weak: strPtr("weak"), Sources: []string{"self"},
	},
	{
		Word: "הִתְלַבֵּשׁ", GlossEN: "got dressed", Translit: "hitlabesh", POS: strPtr("verb"),
		Root: strPtr("ל.ב.ש"), Binyan: strPtr("hitpael"), RootClass: strPtr("regular"),
		HeEN: dir("learning", "steady"), EnHE: dir("learning", "steady"),
		StrengthBucket: strPtr("steady"), Sources: []string{"ai"},
	},
	{
		Word: "חָבֵר", GlossEN: "friend", Translit: "chaver", POS: strPtr("noun"),
		HeEN: dir("learning", "shaky"), EnHE: dir("learning", "shaky"),
		StrengthBucket: strPtr("shaky"), Suspended: true, Sources: []string{"self"},
	},
	{
		Word: "כָּתַב", GlossEN: "wrote", Translit: "katav", POS: strPtr("verb"),
		Root: strPtr("כ.ת.ב"), Binyan: strPtr("paal"), RootClass: strPtr("regular"),
		HeEN: dir("new", "fragile"), EnHE: dir("new", "fragile"),
		StrengthBucket: strPtr("fragile"), Weak: strPtr("very_weak"), Sources: []string{"letters"},
	},
	{
		Word: "לְהִתְחַדֵּשׁ", GlossEN: "to renew", Translit: "le hitchadesh", POS: strPtr("verb"),
		Root: strPtr("ח.ד.ש"), Binyan: strPtr("hitpael"), RootClass: strPtr("regular"),
		HeEN: dir("new", "fragile"), EnHE: dir("learning", "shaky"),
		StrengthBucket: strPtr("fragile"), Sources: []string{"self"},
	},
	{
		Word: "סֵפֶר", GlossEN: "book", Translit: "sefer", POS: strPtr("noun"),
		HeEN: dir("learning", "steady"), EnHE: dir("longTerm", "solid"),
		StrengthBucket: strPtr("steady"), Note: strPtr("from the library lesson"), Sources: []string{"reading"},
	},
}
