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
	if len(m.Words) != 104 {
		t.Fatalf("words = %d, want 104", len(m.Words))
	}
	if len(m.Lessons) != 397 {
		t.Fatalf("lessons = %d, want 397", len(m.Lessons))
	}
	if len(m.UnmappedWordFiles) != 23 {
		t.Fatalf("unmapped = %d, want 23", len(m.UnmappedWordFiles))
	}
	var v3, v4 int
	notNFC := 0
	for name, word := range m.Words {
		switch word.Model {
		case "eleven_v4":
			v4++
		case "eleven_v3":
			v3++
		default:
			t.Errorf("word %s model %q", name, word.Model)
		}
		if !norm.NFC.IsNormalString(word.Text) {
			notNFC++
		}
		assertNormalized(t, word.Text)
	}
	for _, lesson := range m.Lessons {
		if strings.Contains(lesson.Clip, "/_archive/") || strings.Contains(lesson.Lesson, "/_archive/") {
			t.Errorf("archive path in the lesson map: %s/%s", lesson.Lesson, lesson.Clip)
		}
		if !norm.NFC.IsNormalString(lesson.Text) {
			notNFC++
		}
		assertNormalized(t, lesson.Text)
	}
	if v4 != 96 || v3 != 8 {
		t.Fatalf("models eleven_v4=%d eleven_v3=%d, want 96 and 8", v4, v3)
	}
	if notNFC == 0 {
		t.Fatal("fixture has no NFD text, so normalization is not exercised")
	}
	unmapped := map[string]bool{}
	for _, name := range m.UnmappedWordFiles {
		if _, dup := unmapped[name]; dup {
			t.Fatalf("duplicate unmapped name %s", name)
		}
		unmapped[name] = true
		if _, mapped := m.Words[name]; mapped {
			t.Fatalf("unmapped word %s is also mapped", name)
		}
	}
}

func TestImportRealMapDryRun(t *testing.T) {
	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC))
	rep, err := Import(context.Background(), store, data, "testdata", Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Imported != 0 || rep.Duplicate != 0 || rep.SkippedUnmapped != 23 || rep.SkippedArchive != 0 || rep.MissingFile != 501 {
		t.Fatalf("real map dry-run: %+v", rep)
	}
	if len(rep.ByModel) != 0 {
		t.Fatalf("models counted without files: %+v", rep.ByModel)
	}
	if len(rep.Missing) != 501 || len(rep.Unmapped) != 23 {
		t.Fatalf("lists missing=%d unmapped=%d", len(rep.Missing), len(rep.Unmapped))
	}
	for _, rel := range rep.Missing {
		if !strings.HasPrefix(rel, "files/") || strings.Contains(rel, "/_archive/") {
			t.Fatalf("missing path %s", rel)
		}
	}
	for _, rel := range rep.Unmapped {
		if !strings.HasPrefix(rel, "files/_audio/words/") {
			t.Fatalf("unmapped path %s", rel)
		}
	}
	if !containsAll(rep.Missing,
		"files/my-morning/audio/story/line-1.mp3",
		"files/_audio/words/ee2f7d856aaa3bd643275012.mp3",
	) {
		t.Fatal("expected my-morning and word paths in the missing list")
	}
	if mp3Count(t, filepath.Join(data, "audio")) != 0 {
		t.Fatal("dry-run wrote an mp3")
	}
}

func TestMyMorningStoryOrder(t *testing.T) {
	m := loadFixtureMap(t)
	var stories []lessonClip
	for _, lesson := range m.Lessons {
		if lesson.Lesson != "my-morning" || !strings.Contains(lesson.Clip, "/story/") {
			continue
		}
		stories = append(stories, lesson)
	}
	sort.Slice(stories, func(i, j int) bool {
		return storyOrdinal(stories[i].Clip) < storyOrdinal(stories[j].Clip)
	})
	sheet := loadWorksheet(t, "testdata/my-morning-worksheet.js")
	if sheet.ID != "my-morning" {
		t.Fatalf("worksheet id %q", sheet.ID)
	}
	if len(stories) != len(sheet.Reading.Paragraphs) || len(stories) != len(sheet.Reading.Audio) {
		t.Fatalf("stories=%d paragraphs=%d audio=%d", len(stories), len(sheet.Reading.Paragraphs), len(sheet.Reading.Audio))
	}
	lines := make([]string, len(sheet.Reading.Paragraphs))
	for i, raw := range sheet.Reading.Paragraphs {
		line, err := joinParagraph(raw)
		if err != nil {
			t.Fatal(err)
		}
		lines[i] = line
	}
	for i, story := range stories {
		if story.Clip != sheet.Reading.Audio[i] || story.Text != lines[i] {
			t.Fatalf("story %d: map %s %q, worksheet %s %q", i, story.Clip, story.Text, sheet.Reading.Audio[i], lines[i])
		}
	}
	if stories[0].Clip != "audio/story/line-1.mp3" || stories[0].Text != "בַּבֹּקֶר אֲנִי קָם." {
		t.Fatalf("first pair: %s %q", stories[0].Clip, stories[0].Text)
	}
	last := stories[len(stories)-1]
	if last.Clip != "audio/story/line-4.mp3" || last.Text != "אַחַר כָּךְ אֲנִי יוֹצֵא לָעֲבוֹדָה." {
		t.Fatalf("last pair: %s %q", last.Clip, last.Text)
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
	if got != norm.NFC.String(strings.Join(strings.Fields(text), " ")) {
		t.Fatalf("NormalizeText(%q) = %q", text, got)
	}
}

type worksheet struct {
	ID      string `json:"id"`
	Reading struct {
		Paragraphs []json.RawMessage `json:"paragraphs"`
		Audio      []string          `json:"audio"`
	} `json:"reading"`
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

func joinParagraph(raw json.RawMessage) (string, error) {
	var parts []json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, part := range parts {
		part = bytesTrim(part)
		if len(part) == 0 {
			continue
		}
		if part[0] == '"' {
			var s string
			if err := json.Unmarshal(part, &s); err != nil {
				return "", err
			}
			b.WriteString(s)
			continue
		}
		var obj struct {
			Word string `json:"word"`
		}
		if err := json.Unmarshal(part, &obj); err != nil {
			return "", err
		}
		b.WriteString(obj.Word)
	}
	return b.String(), nil
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
