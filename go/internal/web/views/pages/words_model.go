package pages

import (
	"net/url"
	"strconv"
	"strings"
)

// Page keys approved by Backend Dev. They are not FILTER_KEYS.
// word selects a row for the no-JS detail. cursor is the opaque List cursor
// for the no-JS "Next 50" link. fragment is a presentation hint for the
// load-more partial and is never written into the canonical URL.
const (
	QueryWord     = "word"
	QueryCursor   = "cursor"
	QueryFragment = "fragment"
	FragmentRows  = "rows"
)

// FilterKeys is the words-page.js FILTER_KEYS order. Canonical URLs use it.
var FilterKeys = []string{"q", "status", "weak", "pos", "binyan", "tense", "source", "suspended"}

// Filters is the validated FILTER_KEYS set. Empty means unset.
// Weak is "1", "weak", or "very_weak". Suspended is "0" or "1".
type Filters struct {
	Q, Status, Weak, POS, Binyan, Tense, Source, Suspended string
}

// IsEmpty reports that no library filter is set. word and cursor are not filters.
func (f Filters) IsEmpty() bool {
	return f.Q == "" && f.Status == "" && f.Weak == "" && f.POS == "" &&
		f.Binyan == "" && f.Tense == "" && f.Source == "" && f.Suspended == ""
}

// MoreCount is how many "More filters" controls are set.
func (f Filters) MoreCount() int {
	n := 0
	for _, v := range []string{f.POS, f.Binyan, f.Tense, f.Source, f.Suspended} {
		if v != "" {
			n++
		}
	}
	return n
}

// MoreOpen is true when a more-filter is active, so the panel is open on load.
func (f Filters) MoreOpen() bool { return f.MoreCount() > 0 }

// With returns a copy that sets one FILTER_KEYS field. Unknown keys are ignored.
func (f Filters) With(key, value string) Filters {
	return f.with(key, value)
}

func (f Filters) with(key, value string) Filters {
	switch key {
	case "q":
		f.Q = value
	case "status":
		f.Status = value
	case "weak":
		f.Weak = value
	case "pos":
		f.POS = value
	case "binyan":
		f.Binyan = value
	case "tense":
		f.Tense = value
	case "source":
		f.Source = value
	case "suspended":
		f.Suspended = value
	}
	return f
}

func (f Filters) get(key string) string {
	switch key {
	case "q":
		return f.Q
	case "status":
		return f.Status
	case "weak":
		return f.Weak
	case "pos":
		return f.POS
	case "binyan":
		return f.Binyan
	case "tense":
		return f.Tense
	case "source":
		return f.Source
	case "suspended":
		return f.Suspended
	default:
		return ""
	}
}

// CanonicalQuery is the query string in FILTER_KEYS order, then word, then cursor.
// Empty values are omitted. weak=0 is omitted. The result has no leading '?'.
func CanonicalQuery(f Filters, word, cursor string) string {
	var parts []string
	add := func(key, value string) {
		if value == "" {
			return
		}
		parts = append(parts, url.QueryEscape(key)+"="+url.QueryEscape(value))
	}
	for _, key := range FilterKeys {
		add(key, f.get(key))
	}
	add(QueryWord, word)
	add(QueryCursor, cursor)
	return strings.Join(parts, "&")
}

// CanonicalPath is /words/ plus the canonical query.
func CanonicalPath(f Filters, word, cursor string) string {
	q := CanonicalQuery(f, word, cursor)
	if q == "" {
		return "/words/"
	}
	return "/words/?" + q
}

// WordsView is the read-only words screen.
type WordsView struct {
	State       string
	ErrorKind   string
	Rows        []WordRow
	Selected    *WordDetail
	SelectedKey string
	NextCursor  string
	ReviewHref  string
	ReviewDeck  string
	Filters     Filters
	Active      []ActiveFilter
	Word        string
	Cursor      string
	SyncFilters bool
	Shown       int
}

// URL is the canonical page URL for this view.
func (v WordsView) URL() string {
	return CanonicalPath(v.Filters, v.Word, v.Cursor)
}

// URLWith sets one page or filter key. word drops cursor, cursor drops word,
// so a row open does not page and Load more does not keep a stale selection.
func (v WordsView) URLWith(key, value string) string {
	f, word, cursor := v.Filters, v.Word, v.Cursor
	switch key {
	case QueryWord:
		word = value
		cursor = ""
	case QueryCursor:
		cursor = value
		word = ""
	default:
		f = f.with(key, value)
	}
	return CanonicalPath(f, word, cursor)
}

// URLWithout drops a key. Dropping a filter also drops word and cursor.
func (v WordsView) URLWithout(key string) string {
	if key == QueryWord || key == QueryCursor {
		return v.URLWith(key, "")
	}
	return CanonicalPath(v.Filters.with(key, ""), "", "")
}

// ActiveFilter is one removable chip.
type ActiveFilter struct {
	Key, LabelHe, LabelEn, HrefWithout string
}

// WordRow is one list row. SelectURL is the no-JS and HTMX detail URL.
type WordRow struct {
	Word, Gloss, Translit, Binyan, Stage, Audio string
	Suspended                                   bool
	Pressed                                     bool
	SelectURL                                   string
	ChipStatus, ChipHe, ChipEn                  string
	Weak, WeakHe, WeakEn                        string
	Strength, StrengthLabel, StrengthWidth      string
}

// WordDetail is the lookup card.
type WordDetail struct {
	Word, Gloss, Translit, Audio string
	POS, POSHe, POSEn            string
	Root, RootClass, RootColour  string
	Binyan, BinyanHe             string
	HeEn, EnHe                   DirectionView
	Paradigm                     []ParadigmCell
	Sources                      []SourceLink
	CloseURL                     string
	Suspended                    bool
	CanSuspend                   bool
	Blocked                      bool
	FocusAction                  bool
	PostURL                      string
	CSRF                         string
	CSRFHeader                   string
	SuspendValue                 string
	SuspendHe, SuspendEn         string
}

// DirectionView is one direction line on the card.
type DirectionView struct {
	Dir, Stage, Status, LabelHe, LabelEn   string
	NameHe, NameEn                         string
	Strength, StrengthLabel, StrengthWidth string
	Show                                   bool
}

// ParadigmCell is one form line.
type ParadigmCell struct {
	Tense, Cell, He, Gloss string
	Checked                bool
}

// SourceLink is a lesson source, or a personal source with no href.
type SourceLink struct {
	TitleHe, TitleEn, Href string
	Lesson                 bool
	LessonSlug             string
}

// SummaryEn is the extra filter note in the no-results sentence.
func (f Filters) SummaryEn() string {
	return summary(f, false)
}

// SummaryHe is the Hebrew no-results note.
func (f Filters) SummaryHe() string {
	return summary(f, true)
}

func summary(f Filters, hebrew bool) string {
	var bits []string
	add := func(he, en string) {
		if hebrew {
			bits = append(bits, he)
		} else {
			bits = append(bits, en)
		}
	}
	if f.Status != "" {
		add(stageLabel(f.Status))
	}
	if f.Weak == "very_weak" {
		add("חלשה מאוד", "very weak")
	} else if f.Weak != "" {
		add("חלשות", "weak")
	}
	if f.POS != "" {
		add(posLabel(f.POS))
	}
	if f.Binyan != "" {
		bits = append(bits, binyanHe(f.Binyan))
	}
	if f.Tense != "" {
		add(tenseLabel(f.Tense))
	}
	if f.Source != "" {
		add(sourceLabel(f.Source))
	}
	if f.Suspended == "1" {
		add("מושהות", "paused")
	} else if f.Suspended == "0" {
		add("פעילות", "active")
	}
	if len(bits) == 0 {
		return ""
	}
	return " · " + strings.Join(bits, ", ")
}

// ShownHe and ShownEn are the status line. There is no total, so it is "N shown".
func ShownHe(n int) string { return strconv.Itoa(n) + " מוצגות" }
func ShownEn(n int) string { return strconv.Itoa(n) + " shown" }
