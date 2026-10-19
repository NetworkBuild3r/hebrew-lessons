package audio

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/unicode/norm"
)

func TestClipMapFixture(t *testing.T) {
	m := loadFixtureMap(t)
	if m.Schema != clipMapSchema {
		t.Fatalf("schema %q", m.Schema)
	}
	if len(m.Lessons) != 5 || len(m.Words) != 2 || len(m.Unmapped) != 2 {
		t.Fatalf("lessons=%d words=%d unmapped=%d", len(m.Lessons), len(m.Words), len(m.Unmapped))
	}
	var v3, v4 int
	for _, word := range m.Words {
		switch word.Model {
		case "eleven_v4":
			v4++
		case "eleven_v3":
			v3++
		default:
			t.Errorf("word %s model %q", word.Path, word.Model)
		}
		if path.Base(word.Path) != wordFileName(word.VoiceID, word.Model, word.Text) {
			t.Errorf("word %s is not cacheKey(voice, model, text.trim())", word.Path)
		}
		assertNormalized(t, word.Text)
	}
	if v3 != 1 || v4 != 1 {
		t.Fatalf("word models v3=%d v4=%d", v3, v4)
	}
	var corrected mappedClip
	foundCorrected := false
	for _, lesson := range m.Lessons {
		if lesson.Source != "lesson" {
			t.Errorf("lesson source %q", lesson.Source)
		}
		if contentHash(lesson.VoiceID, lesson.Model, lesson.Text) != strings.ToLower(lesson.TextHash) {
			t.Errorf("lesson %s text hash mismatch", lesson.Path)
		}
		if strings.Contains(lesson.Path, "/_archive/") {
			t.Errorf("archive path in the lesson map: %s", lesson.Path)
		}
		assertNormalized(t, lesson.Text)
		if lesson.Lesson == "date-night-maadif-infinitive" && lesson.Clip == "audio/story/line-3.mp3" {
			corrected = lesson
			foundCorrected = true
		}
	}
	if !foundCorrected {
		t.Fatal("fixture is missing the corrected direct-speech clip")
	}
	const full = "הִיא שׁוֹאֶלֶת: \"מָה אַתָּה מַעֲדִיף לַעֲשׂוֹת הָעֶרֶב?\""
	if corrected.Text != full {
		t.Fatalf("corrected text %q", corrected.Text)
	}
	reasons := map[string]string{}
	for _, item := range m.Unmapped {
		reasons[item.Path] = item.Reason
	}
	if reasons["_audio/words/1828c011c5e8e563ce974431.mp3"] != "word-not-in-manifest" {
		t.Fatalf("unmapped words: %+v", reasons)
	}
	if reasons["anal-sex-vocab/audio/_archive/friend-1.94ba0ad93fdd.mp3"] != "archive" {
		t.Fatalf("unmapped archive: %+v", reasons)
	}
}

func TestImportRealMapDryRun(t *testing.T) {
	// The importer reads clip-text-map.json from the bundle root. The
	// testdata file is only the fixture copied into that bundle.
	src := t.TempDir()
	raw, err := os.ReadFile("testdata/clip-text-map.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "clip-text-map.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	bundle := packExport(t, src, "bundle")
	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC))
	rep, err := Import(context.Background(), store, data, bundle, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Imported != 0 || rep.Duplicate != 0 || rep.Unmapped != 2 || rep.HashMismatch != 0 || rep.TextMismatch != 0 || rep.Conflict != 0 || rep.Alias != 0 || rep.MissingFile != 7 {
		t.Fatalf("real map dry-run: %+v", rep)
	}
	if len(rep.ByModel) != 0 {
		t.Fatalf("models counted without files: %+v", rep.ByModel)
	}
	if !containsAll(rep.Missing,
		"my-morning/audio/story/line-1.mp3",
		"_audio/words/ee2f7d856aaa3bd643275012.mp3",
	) {
		t.Fatalf("missing = %v", rep.Missing)
	}
	for _, rel := range rep.Missing {
		if strings.HasPrefix(rel, "files/") || strings.Contains(rel, "/_archive/") {
			t.Fatalf("missing path %s", rel)
		}
	}
	var sawWord, sawArchive bool
	for _, item := range rep.UnmappedItems {
		switch item.Reason {
		case "word-not-in-manifest":
			sawWord = true
		case "archive":
			sawArchive = strings.Contains(item.Path, "/_archive/")
		}
	}
	if !sawWord || !sawArchive {
		t.Fatalf("unmapped = %+v", rep.UnmappedItems)
	}
	if mp3Count(t, filepath.Join(data, "audio")) != 0 {
		t.Fatal("dry-run wrote an mp3")
	}
}

func TestPartTextEveryWorksheet(t *testing.T) {
	m := loadFixtureMap(t)
	sheets := loadWorksheets(t)
	if len(sheets) < 2 {
		t.Fatalf("worksheets = %d, want my-morning and a direct/reported lesson", len(sheets))
	}
	checked := 0
	for id, sheet := range sheets {
		stories := storyClips(m, id)
		if len(stories) != len(sheet.Reading.Paragraphs) || len(stories) != len(sheet.Reading.Audio) {
			t.Fatalf("%s stories=%d paragraphs=%d audio=%d", id, len(stories), len(sheet.Reading.Paragraphs), len(sheet.Reading.Audio))
		}
		for i, story := range stories {
			line, err := joinParagraph(sheet.Reading.Paragraphs[i])
			if err != nil {
				t.Fatal(err)
			}
			if story.Clip != sheet.Reading.Audio[i] || story.Text != line {
				t.Fatalf("%s story %d: map %s %q, worksheet %s %q", id, i, story.Clip, story.Text, sheet.Reading.Audio[i], line)
			}
			checked++
		}
	}
	if checked < 5 {
		t.Fatalf("checked %d story lines", checked)
	}
}

func TestMyMorningStoryOrder(t *testing.T) {
	m := loadFixtureMap(t)
	stories := storyClips(m, "my-morning")
	if stories[0].Clip != "audio/story/line-1.mp3" || stories[0].Text != "בַּבֹּקֶר אֲנִי קָם." {
		t.Fatalf("first pair: %s %q", stories[0].Clip, stories[0].Text)
	}
	last := stories[len(stories)-1]
	if last.Clip != "audio/story/line-4.mp3" || last.Text != "אַחַר כָּךְ אֲנִי יוֹצֵא לָעֲבוֹדָה." {
		t.Fatalf("last pair: %s %q", last.Clip, last.Text)
	}
}

func TestPartTextReportedDirectStrong(t *testing.T) {
	// origin/main shared/player/utils.js HebrewUtils.partText, and the
	// recursive form in the v2 map generator: reported || direct || strong ||
	// word, then the same function on that value.
	cases := []struct {
		raw  string
		want string
	}{
		{`["הִיא שׁוֹאֶלֶת: ",{"direct":"\"מָה אַתָּה מַעֲדִיף לַעֲשׂוֹת הָעֶרֶב?\""}]`, "הִיא שׁוֹאֶלֶת: \"מָה אַתָּה מַעֲדִיף לַעֲשׂוֹת הָעֶרֶב?\""},
		{`{"reported":[{"strong":"שַׁי "},"אָמַר לִי ."]}`, "שַׁי אָמַר לִי ."},
		{`{"reported":"","direct":"שָׁלוֹם"}`, "שָׁלוֹם"},
		{`{"word":"קָם"}`, "קָם"},
		{`"בַּבֹּקֶר"`, "בַּבֹּקֶר"},
		{`{"strong":["אֲנִי ","קָם"]}`, "אֲנִי קָם"},
	}
	for _, tc := range cases {
		got, err := joinParagraph(json.RawMessage(tc.raw))
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("partText(%s) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestStoryOrdinalSortsNumerically(t *testing.T) {
	clips := []string{"audio/story/line-10.mp3", "audio/story/line-2.mp3", "audio/story/line-1.mp3"}
	sort.Slice(clips, func(i, j int) bool {
		return storyOrdinal(clips[i]) < storyOrdinal(clips[j])
	})
	want := []string{"audio/story/line-1.mp3", "audio/story/line-2.mp3", "audio/story/line-10.mp3"}
	if strings.Join(clips, ",") != strings.Join(want, ",") {
		t.Fatalf("sorted %v", clips)
	}
}

func TestNormalizeTextStripsFormatCharacters(t *testing.T) {
	got := NormalizeText("קָם\u200e")
	if got != "קָם" {
		t.Fatalf("NormalizeText kept a format character: %q", got)
	}
	if NormalizeText("\ufeffבַּ  בֹּקֶר") != "בַּ בֹּקֶר" {
		t.Fatalf("bom/spaces: %q", NormalizeText("\ufeffבַּ  בֹּקֶר"))
	}
	if !norm.NFC.IsNormalString(got) {
		t.Fatal("result is not NFC")
	}
}

func storyClips(m clipMap, lesson string) []mappedClip {
	var stories []mappedClip
	for _, clip := range m.Lessons {
		if clip.Lesson == lesson && strings.Contains(clip.Clip, "/story/") {
			stories = append(stories, clip)
		}
	}
	sort.Slice(stories, func(i, j int) bool {
		return storyOrdinal(stories[i].Clip) < storyOrdinal(stories[j].Clip)
	})
	return stories
}

func storyOrdinal(clip string) int {
	base := strings.TrimSuffix(path.Base(clip), path.Ext(clip))
	i := strings.LastIndex(base, "-")
	if i < 0 {
		return 0
	}
	n, err := strconv.Atoi(base[i+1:])
	if err != nil {
		return 0
	}
	return n
}

func loadFixtureMap(t *testing.T) clipMap {
	t.Helper()
	m, err := loadMapFile("testdata/clip-text-map.json")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func assertNormalized(t *testing.T, text string) {
	t.Helper()
	got := NormalizeText(text)
	if got == "" {
		t.Fatalf("text %q normalized to empty", text)
	}
	if !norm.NFC.IsNormalString(got) {
		t.Fatalf("normalized text is not NFC: %q", got)
	}
	if got != text {
		t.Fatalf("fixture text is not normalized: %q -> %q", text, got)
	}
}

type worksheet struct {
	ID      string `json:"id"`
	Reading struct {
		Paragraphs []json.RawMessage `json:"paragraphs"`
		Audio      []string          `json:"audio"`
	} `json:"reading"`
}

func loadWorksheets(t *testing.T) map[string]worksheet {
	t.Helper()
	matches, err := filepath.Glob("testdata/*-worksheet.js")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]worksheet{}
	for _, path := range matches {
		sheet := loadWorksheet(t, path)
		if _, dup := out[sheet.ID]; dup {
			t.Fatalf("duplicate worksheet %s", sheet.ID)
		}
		out[sheet.ID] = sheet
	}
	return out
}

func loadWorksheet(t *testing.T, path string) worksheet {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.TrimSpace(string(body))
	const prefix = "window.WORKSHEET ="
	if !strings.HasPrefix(s, prefix) {
		t.Fatalf("worksheet %s does not start with %s", path, prefix)
	}
	s = strings.TrimSpace(strings.TrimPrefix(s, prefix))
	s = strings.TrimSuffix(s, ";")
	var sheet worksheet
	if err := json.Unmarshal([]byte(s), &sheet); err != nil {
		t.Fatal(err)
	}
	return sheet
}

// joinParagraph is storyPlain's per-paragraph step: partText, then trim.
// partText is HebrewUtils.partText from origin/main shared/player/utils.js
// (reported || direct || strong || word). Nested objects and arrays are
// visited again, which is the generator's recursive form of that function.
func joinParagraph(raw json.RawMessage) (string, error) {
	s, err := partText(raw)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

func partText(raw json.RawMessage) (string, error) {
	raw = bytesTrim(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	switch raw[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", err
		}
		return s, nil
	case '[':
		var parts []json.RawMessage
		if err := json.Unmarshal(raw, &parts); err != nil {
			return "", err
		}
		var b strings.Builder
		for _, part := range parts {
			s, err := partText(part)
			if err != nil {
				return "", err
			}
			b.WriteString(s)
		}
		return b.String(), nil
	case '{':
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			return "", err
		}
		for _, key := range []string{"reported", "direct", "strong", "word"} {
			v, ok := obj[key]
			if !ok || jsFalsy(v) {
				continue
			}
			return partText(v)
		}
		return "", nil
	default:
		return "", nil
	}
}

func jsFalsy(raw json.RawMessage) bool {
	raw = bytesTrim(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return true
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return true
	}
	switch t := v.(type) {
	case nil:
		return true
	case bool:
		return !t
	case string:
		return t == ""
	case float64:
		return t == 0
	default:
		return false
	}
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func containsAll(list []string, want ...string) bool {
	have := map[string]bool{}
	for _, item := range list {
		have[item] = true
	}
	for _, item := range want {
		if !have[item] {
			return false
		}
	}
	return true
}
