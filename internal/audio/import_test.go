package audio

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func openStoreAt(t *testing.T, dir string, at time.Time) *db.Store {
	t.Helper()
	s, err := db.Open(context.Background(), filepath.Join(dir, "hebrew.sqlite"), fixedClock{at})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func shaID(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func hasNiqqud(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) {
			return true
		}
	}
	return false
}

func TestImportDirectory(t *testing.T) {
	v4 := []byte("clip-v4-kam")
	v3 := []byte("clip-v3-kam")
	keep := []byte("clip-keep")
	nfdBody := []byte("clip-nfd")
	spaceBody := []byte("clip-space")
	lessonBody := []byte("clip-lesson")
	// Bet, dagesh, qamats. Canonical order is qamats then dagesh, so this
	// is NFD-incompatible until NormalizeText reorders the marks.
	nfdText := "\u05D1\u05BC\u05B8 "
	if norm.NFC.IsNormalString(strings.TrimSpace(nfdText)) {
		t.Fatal("nfd fixture is already NFC")
	}

	export := t.TempDir()
	files := map[string][]byte{
		"files/_audio/words/v4.mp3":            v4,
		"files/_audio/words/v3.mp3":            v3,
		"files/_audio/words/a-keep.mp3":        keep,
		"files/_audio/words/b-dup.mp3":         keep,
		"files/_audio/words/nfd.mp3":           nfdBody,
		"files/_audio/words/space.mp3":         spaceBody,
		"files/_audio/words/unmapped.mp3":      []byte("skip-me"),
		"files/demo/audio/story/line-1.mp3":    lessonBody,
		"files/demo/audio/sentences/empty.mp3": []byte("empty-text"),
		"files/demo/audio/_archive/line-9.mp3": []byte("archived"),
	}
	for rel, body := range files {
		path := filepath.Join(export, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	clipMap := map[string]any{
		"words": map[string]any{
			"v4.mp3":     map[string]string{"text": "קָם", "voiceId": "voice-a", "model": "eleven_v4"},
			"v3.mp3":     map[string]string{"text": "קָם", "voiceId": "voice-a", "model": "eleven_v3"},
			"a-keep.mp3": map[string]string{"text": "אֶחָד", "voiceId": "voice-a", "model": "eleven_v4"},
			"b-dup.mp3":  map[string]string{"text": "שְׁנַיִם", "voiceId": "voice-a", "model": "eleven_v3"},
			"nfd.mp3":    map[string]string{"text": nfdText, "voiceId": "voice-b", "model": "eleven_v3"},
			"space.mp3":  map[string]string{"text": "אַחַר  כָּךְ", "voiceId": "voice-a", "model": "eleven_v4"},
		},
		"lessons": []map[string]string{
			{"lesson": "demo", "clip": "audio/story/line-1.mp3", "text": "בַּבֹּקֶר \n"},
			{"lesson": "demo", "clip": "audio/sentences/empty.mp3", "text": " \n\t "},
			{"lesson": "demo", "clip": "audio/sentences/missing.mp3", "text": "אֵין"},
			{"lesson": "demo", "clip": "audio/_archive/line-9.mp3", "text": "אַרְכִיּוֹן"},
		},
		"unmapped_word_files": []string{"unmapped.mp3"},
	}
	raw, err := json.Marshal(clipMap)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(export, "clip-text-map.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	data := t.TempDir()
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store := openStoreAt(t, data, when)

	dry, err := Import(ctx, store, data, export, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if dry.Imported != 6 || dry.Duplicate != 1 || dry.SkippedUnmapped != 2 || dry.SkippedArchive != 1 || dry.MissingFile != 1 {
		t.Fatalf("dry-run counts: %+v", dry)
	}
	if dry.ByModel["eleven_v4"] != 3 || dry.ByModel["eleven_v3"] != 3 || dry.ByModel["legacy"] != 1 {
		t.Fatalf("dry-run models: %+v", dry.ByModel)
	}
	if mp3Count(t, filepath.Join(data, "audio")) != 0 {
		t.Fatal("dry-run wrote an mp3")
	}
	if has, err := store.HasAudio(ctx, shaID(v4)); err != nil || has {
		t.Fatalf("dry-run stored a row: has=%v err=%v", has, err)
	}
	assertReportLists(t, dry)

	got, err := Import(ctx, store, data, export, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Imported != 6 || got.Duplicate != 1 || got.SkippedUnmapped != 2 || got.SkippedArchive != 1 || got.MissingFile != 1 {
		t.Fatalf("import counts: %+v", got)
	}
	if mp3Count(t, filepath.Join(data, "audio")) != 6 {
		t.Fatalf("mp3 files = %d, want 6", mp3Count(t, filepath.Join(data, "audio")))
	}
	if body, err := os.ReadFile(filepath.Join(data, "audio", shaID(v4)+".mp3")); err != nil || !bytes.Equal(body, v4) {
		t.Fatalf("v4 file = %q err=%v", body, err)
	}
	if _, err := os.Stat(filepath.Join(data, "audio", shaID(keep)+".mp3")); err != nil {
		t.Fatal(err)
	}

	kam, ok, err := store.AudioByText(ctx, "קָם")
	if err != nil || !ok || kam.ID != shaID(v4) || kam.Model != "eleven_v4" || kam.Voice != "voice-a" {
		t.Fatalf("kam lookup = %+v ok=%v err=%v", kam, ok, err)
	}
	if has, err := store.HasAudio(ctx, shaID(v3)); err != nil || !has {
		t.Fatalf("v3 row kept: has=%v err=%v", has, err)
	}
	kept, ok, err := store.AudioByText(ctx, "אֶחָד")
	if err != nil || !ok || kept.ID != shaID(keep) || kept.Model != "eleven_v4" {
		t.Fatalf("kept duplicate text = %+v ok=%v err=%v", kept, ok, err)
	}
	if _, ok, err := store.AudioByText(ctx, "שְׁנַיִם"); err != nil || ok {
		t.Fatalf("losing duplicate text was stored: ok=%v err=%v", ok, err)
	}
	wantNFC := norm.NFC.String(strings.TrimSpace(nfdText))
	nfdGot, ok, err := store.AudioByText(ctx, wantNFC)
	if err != nil || !ok {
		t.Fatalf("nfd lookup ok=%v err=%v", ok, err)
	}
	if nfdGot.TextNFC != wantNFC || !norm.NFC.IsNormalString(nfdGot.TextNFC) || nfdGot.TextNFC == strings.TrimSpace(nfdText) {
		t.Fatalf("stored nfd text %q", nfdGot.TextNFC)
	}
	if !hasNiqqud(nfdGot.TextNFC) || nfdGot.Voice != "voice-b" || nfdGot.Model != "eleven_v3" {
		t.Fatalf("nfd asset = %+v", nfdGot)
	}
	spaceGot, ok, err := store.AudioByText(ctx, "אַחַר כָּךְ")
	if err != nil || !ok || spaceGot.TextNFC != "אַחַר כָּךְ" {
		t.Fatalf("collapsed space = %+v ok=%v err=%v", spaceGot, ok, err)
	}
	lesson, ok, err := store.AudioByText(ctx, "בַּבֹּקֶר")
	if err != nil || !ok || lesson.Voice != "" || lesson.Model != "legacy" || lesson.Bytes != int64(len(lessonBody)) {
		t.Fatalf("lesson asset = %+v ok=%v err=%v", lesson, ok, err)
	}
	if _, ok, err := store.AudioByText(ctx, "אַרְכִיּוֹן"); err != nil || ok {
		t.Fatalf("archive clip was stored: ok=%v err=%v", ok, err)
	}
	assertReportLists(t, got)

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	later := openStoreAt(t, data, when.Add(24*time.Hour))
	again, err := Import(ctx, later, data, export, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if again.Imported != 0 || again.Duplicate != 7 || again.SkippedUnmapped != 2 || again.SkippedArchive != 1 || again.MissingFile != 1 {
		t.Fatalf("re-run counts: %+v", again)
	}
	if mp3Count(t, filepath.Join(data, "audio")) != 6 {
		t.Fatal("re-run added an mp3")
	}
	kam, ok, err = later.AudioByText(ctx, "קָם")
	if err != nil || !ok || !kam.CreatedAt.Equal(when) {
		t.Fatalf("re-run changed the row: %+v ok=%v err=%v", kam, ok, err)
	}

	v4Path := filepath.Join(data, "audio", shaID(v4)+".mp3")
	if err := os.Remove(v4Path); err != nil {
		t.Fatal(err)
	}
	repaired, err := Import(ctx, later, data, export, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if repaired.Imported != 0 || repaired.Duplicate != 7 {
		t.Fatalf("repair counts: %+v", repaired)
	}
	if body, err := os.ReadFile(v4Path); err != nil || !bytes.Equal(body, v4) {
		t.Fatalf("repaired file %q err=%v", body, err)
	}
	kam, ok, err = later.AudioByText(ctx, "קָם")
	if err != nil || !ok || !kam.CreatedAt.Equal(when) || kam.ID != shaID(v4) {
		t.Fatalf("repair changed the row: %+v ok=%v err=%v", kam, ok, err)
	}
}

func assertReportLists(t *testing.T, rep Report) {
	t.Helper()
	body, err := os.ReadFile(rep.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Unmapped []string `json:"unmapped"`
		Missing  []string `json:"missing"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	if strings.Join(parsed.Unmapped, ",") != strings.Join(rep.Unmapped, ",") {
		t.Fatalf("report unmapped = %v, summary %v", parsed.Unmapped, rep.Unmapped)
	}
	wantUnmapped := []string{
		"files/_audio/words/unmapped.mp3",
		"files/demo/audio/sentences/empty.mp3",
	}
	if strings.Join(parsed.Unmapped, ",") != strings.Join(wantUnmapped, ",") {
		t.Fatalf("unmapped = %v", parsed.Unmapped)
	}
	if len(parsed.Missing) != 1 || parsed.Missing[0] != "files/demo/audio/sentences/missing.mp3" {
		t.Fatalf("missing = %v", parsed.Missing)
	}
	for _, path := range append(append([]string{}, parsed.Unmapped...), parsed.Missing...) {
		if strings.Contains(path, "/_archive/") {
			t.Fatalf("archive path listed in the report: %s", path)
		}
	}
}

func TestImportArchiveAndStdin(t *testing.T) {
	export := t.TempDir()
	word := []byte("word-bytes")
	lesson := []byte("lesson-bytes")
	for rel, body := range map[string][]byte{
		"files/_audio/words/w.mp3":          word,
		"files/demo/audio/story/line-1.mp3": lesson,
		"files/_audio/words/skip.mp3":       []byte("nope"),
		"files/demo/audio/_archive/old.mp3": []byte("old"),
	} {
		path := filepath.Join(export, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw := []byte(`{
		"words": {"w.mp3": {"text": "שָׁלוֹם", "voiceId": "voice-z", "model": "eleven_v4"}},
		"lessons": [{"lesson": "demo", "clip": "audio/story/line-1.mp3", "text": "בֹּקֶר טוֹב"}],
		"unmapped_word_files": ["skip.mp3"]
	}`)
	if err := os.WriteFile(filepath.Join(export, "clip-text-map.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	tgz := packExport(t, export, "bundle")

	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 8, 0, 0, 0, time.UTC))
	ctx := context.Background()
	got, err := Import(ctx, store, data, tgz, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Imported != 2 || got.Duplicate != 0 || got.SkippedUnmapped != 1 || got.SkippedArchive != 1 || got.MissingFile != 0 {
		t.Fatalf("tarball counts: %+v", got)
	}
	asset, ok, err := store.AudioByText(ctx, "שָׁלוֹם")
	if err != nil || !ok || asset.Voice != "voice-z" || asset.Model != "eleven_v4" || asset.ID != shaID(word) {
		t.Fatalf("word from tarball = %+v ok=%v err=%v", asset, ok, err)
	}
	asset, ok, err = store.AudioByText(ctx, "בֹּקֶר טוֹב")
	if err != nil || !ok || asset.Model != "legacy" || asset.Voice != "" || asset.ID != shaID(lesson) {
		t.Fatalf("lesson from tarball = %+v ok=%v err=%v", asset, ok, err)
	}

	data2 := t.TempDir()
	store2 := openStoreAt(t, data2, time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC))
	packed, err := os.ReadFile(tgz)
	if err != nil {
		t.Fatal(err)
	}
	got, err = Import(ctx, store2, data2, "", Options{Archive: bytes.NewReader(packed)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Imported != 2 || got.SkippedArchive != 1 || got.SkippedUnmapped != 1 {
		t.Fatalf("stdin counts: %+v", got)
	}
	matches, err := filepath.Glob(filepath.Join(data2, "audio", ".import-stdin-*.tgz"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("spooled archive left behind: %v", matches)
	}
}

func TestImportRejectsEscapingTarPath(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	body := []byte(`{"words":{},"lessons":[],"unmapped_word_files":[]}`)
	if err := tw.WriteHeader(&tar.Header{Name: "clip-text-map.json", Mode: 0o644, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	evil := []byte("nope")
	if err := tw.WriteHeader(&tar.Header{Name: "../evil.mp3", Mode: 0o644, Size: int64(len(evil))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(evil); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 8, 0, 0, 0, time.UTC))
	_, err := Import(context.Background(), store, data, "", Options{Archive: &buf})
	if err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("escaping path error = %v", err)
	}
}

func TestAudioByTextPreference(t *testing.T) {
	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC))
	ctx := context.Background()
	older := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	text := "קָם"
	rows := []db.Asset{
		{ID: strings.Repeat("a", 64), TextNFC: text, Voice: "v", Model: "legacy", Bytes: 1, CreatedAt: newer},
		{ID: strings.Repeat("b", 64), TextNFC: text, Voice: "v", Model: "eleven_v3", Bytes: 2, CreatedAt: older},
		{ID: strings.Repeat("c", 64), TextNFC: text, Voice: "v", Model: "eleven_v4", Bytes: 3, CreatedAt: older},
		{ID: strings.Repeat("d", 64), TextNFC: text, Voice: "v", Model: "other", Bytes: 4, CreatedAt: newer.Add(time.Hour)},
	}
	for _, row := range rows {
		inserted, err := store.InsertAudio(ctx, row)
		if err != nil || !inserted {
			t.Fatalf("insert %+v: inserted=%v err=%v", row, inserted, err)
		}
	}
	got, ok, err := store.AudioByText(ctx, text)
	if err != nil || !ok || got.Model != "eleven_v4" || got.ID != strings.Repeat("c", 64) {
		t.Fatalf("preference = %+v ok=%v err=%v", got, ok, err)
	}

	again, err := store.InsertAudio(ctx, rows[2])
	if err != nil || again {
		t.Fatalf("duplicate insert inserted=%v err=%v", again, err)
	}

	oldID := strings.Repeat("1", 64)
	newID := strings.Repeat("2", 64)
	for _, row := range []db.Asset{
		{ID: oldID, TextNFC: "שָׁלוֹם", Voice: "", Model: "legacy", Bytes: 1, CreatedAt: older},
		{ID: newID, TextNFC: "שָׁלוֹם", Voice: "", Model: "legacy", Bytes: 1, CreatedAt: newer},
	} {
		if _, err := store.InsertAudio(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err = store.AudioByText(ctx, "שָׁלוֹם")
	if err != nil || !ok || got.ID != newID {
		t.Fatalf("newer legacy = %+v ok=%v err=%v", got, ok, err)
	}

	low := strings.Repeat("0", 64)
	high := strings.Repeat("f", 64)
	for _, id := range []string{high, low} {
		if _, err := store.InsertAudio(ctx, db.Asset{
			ID: id, TextNFC: "כֵּן", Voice: "v", Model: "eleven_v3", Bytes: 1, CreatedAt: older,
		}); err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err = store.AudioByText(ctx, "כֵּן")
	if err != nil || !ok || got.ID != low {
		t.Fatalf("tie break = %+v ok=%v err=%v", got, ok, err)
	}
	if _, ok, err := store.AudioByText(ctx, "אֵין"); err != nil || ok {
		t.Fatalf("missing text ok=%v err=%v", ok, err)
	}
}

func packExport(t *testing.T, dir, prefix string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "audio.tgz")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	zw := gzip.NewWriter(f)
	tw := tar.NewWriter(zw)
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name := prefix + "/" + filepath.ToSlash(rel)
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body))}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = tw.Write(body)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return out
}

func mp3Count(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".mp3") {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}
