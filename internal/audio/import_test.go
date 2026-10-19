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
	aliasA := []byte("alias-a-bytes")
	aliasB := []byte("alias-b-bytes-longer")
	emptyBody := []byte("empty-text")
	// Bet, dagesh, qamats. Canonical order is qamats then dagesh, so this
	// is not NFC until NormalizeText reorders the marks.
	nfdText := "\u05D1\u05BC\u05B8 "
	if norm.NFC.IsNormalString(strings.TrimSpace(nfdText)) {
		t.Fatal("nfd fixture is already NFC")
	}
	const (
		voice = "voice-a"
		alias = "אוֹתוֹ מִשְׁפָּט"
	)

	export := t.TempDir()
	lessons := []any{
		lessonObj("demo", "audio/story/line-1.mp3", "בַּבֹּקֶר \n", voice, "eleven_v4", lessonBody, "", ""),
		lessonObj("demo", "audio/sentences/empty.mp3", " \n\t ", voice, "eleven_v4", emptyBody, "", ""),
		lessonObj("demo", "audio/sentences/missing.mp3", "אֵין", voice, "eleven_v4", nil, "", shaID([]byte("missing"))),
		lessonObj("demo", "audio/story/line-2.mp3", alias, voice, "eleven_v4", aliasA, "", ""),
		lessonObj("demo", "audio/story/line-3.mp3", alias, voice, "eleven_v4", aliasB, "", ""),
	}
	words := []any{
		wordObj("קָם", voice, "eleven_v4", v4, ""),
		wordObj("קָם", voice, "eleven_v3", v3, ""),
		wordObj("אֶחָד", voice, "eleven_v4", keep, ""),
		wordObj("שְׁנַיִם", voice, "eleven_v3", keep, ""),
		wordObj(nfdText, "voice-b", "eleven_v3", nfdBody, ""),
		wordObj("אַחַר  כָּךְ", voice, "eleven_v4", spaceBody, ""),
	}
	unmapped := []any{
		map[string]string{"reason": "word-not-in-manifest", "path": "_audio/words/unmapped.mp3", "sha256": shaID([]byte("skip-me"))},
		map[string]string{"reason": "archive", "path": "demo/audio/_archive/line-9.mp3", "sha256": shaID([]byte("archived"))},
	}
	writeFiles(t, export, map[string][]byte{
		words[0].(map[string]any)["path"].(string): v4,
		words[1].(map[string]any)["path"].(string): v3,
		words[2].(map[string]any)["path"].(string): keep,
		words[3].(map[string]any)["path"].(string): keep,
		words[4].(map[string]any)["path"].(string): nfdBody,
		words[5].(map[string]any)["path"].(string): spaceBody,
		"demo/audio/story/line-1.mp3":              lessonBody,
		"demo/audio/sentences/empty.mp3":           emptyBody,
		"demo/audio/story/line-2.mp3":              aliasA,
		"demo/audio/story/line-3.mp3":              aliasB,
		"_audio/words/unmapped.mp3":                []byte("skip-me"),
		"demo/audio/_archive/line-9.mp3":           []byte("archived"),
	})
	writeMap(t, export, lessons, words, unmapped)

	data := t.TempDir()
	when := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store := openStoreAt(t, data, when)

	dry, err := Import(ctx, store, data, export, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if dry.Imported != 8 || dry.Duplicate != 0 || dry.Unmapped != 3 || dry.HashMismatch != 0 || dry.TextMismatch != 0 || dry.Conflict != 1 || dry.Alias != 1 || dry.MissingFile != 1 {
		t.Fatalf("dry-run counts: %+v", dry)
	}
	if dry.ByModel["eleven_v4"] != 6 || dry.ByModel["eleven_v3"] != 2 {
		t.Fatalf("dry-run models: %+v", dry.ByModel)
	}
	if mp3Count(t, filepath.Join(data, "audio")) != 0 {
		t.Fatal("dry-run wrote an mp3")
	}
	if has, err := store.HasAudio(ctx, shaID(v4)); err != nil || has {
		t.Fatalf("dry-run stored a row: has=%v err=%v", has, err)
	}
	assertConflict(t, dry, "אֶחָד", "שְׁנַיִם")

	got, err := Import(ctx, store, data, export, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Imported != 8 || got.Duplicate != 0 || got.Unmapped != 3 || got.Conflict != 1 || got.Alias != 1 || got.MissingFile != 1 {
		t.Fatalf("import counts: %+v", got)
	}
	if mp3Count(t, filepath.Join(data, "audio")) != 8 {
		t.Fatalf("mp3 files = %d, want 8", mp3Count(t, filepath.Join(data, "audio")))
	}
	if body, err := os.ReadFile(filepath.Join(data, "audio", shaID(v4)+".mp3")); err != nil || !bytes.Equal(body, v4) {
		t.Fatalf("v4 file = %q err=%v", body, err)
	}

	kam, ok, err := store.AudioByText(ctx, "קָם")
	if err != nil || !ok || kam.ID != shaID(v4) || kam.Model != "eleven_v4" || kam.Voice != voice {
		t.Fatalf("kam lookup = %+v ok=%v err=%v", kam, ok, err)
	}
	if has, err := store.HasAudio(ctx, shaID(v3)); err != nil || !has {
		t.Fatalf("v3 row kept: has=%v err=%v", has, err)
	}
	assertConflict(t, got, "אֶחָד", "שְׁנַיִם")
	storedConflict := 0
	for _, text := range []string{"אֶחָד", "שְׁנַיִם"} {
		asset, ok, err := store.AudioByText(ctx, text)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			storedConflict++
			if asset.ID != shaID(keep) {
				t.Fatalf("conflict row = %+v", asset)
			}
		}
	}
	if storedConflict != 1 {
		t.Fatalf("conflict stored %d texts, want the first path's text only", storedConflict)
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
	if err != nil || !ok || lesson.Voice != voice || lesson.Model != "eleven_v4" || lesson.Bytes != int64(len(lessonBody)) {
		t.Fatalf("lesson asset = %+v ok=%v err=%v", lesson, ok, err)
	}
	if _, ok, err := store.AudioByText(ctx, "אַרְכִיּוֹן"); err != nil || ok {
		t.Fatalf("archive clip was stored: ok=%v err=%v", ok, err)
	}

	low, high := shaID(aliasA), shaID(aliasB)
	if high < low {
		low, high = high, low
	}
	chosen, ok, err := store.AudioByText(ctx, alias)
	if err != nil || !ok || chosen.ID != low {
		t.Fatalf("alias index = %+v ok=%v err=%v, want %s", chosen, ok, err, low)
	}
	if has, err := store.HasAudio(ctx, high); err != nil || !has {
		t.Fatalf("alias asset stored: has=%v err=%v", has, err)
	}
	manifest := readManifest(t, got.ManifestPath)
	aliasPaths := 0
	for _, clip := range manifest {
		if clip.Text == alias && clip.Alias {
			aliasPaths++
			if clip.ID != high {
				t.Fatalf("alias entry = %+v, want id %s", clip, high)
			}
		}
	}
	if aliasPaths != 1 {
		t.Fatalf("alias manifest paths = %d", aliasPaths)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	later := openStoreAt(t, data, when.Add(24*time.Hour))
	again, err := Import(ctx, later, data, export, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if again.Imported != 0 || again.Duplicate != 8 || again.Conflict != 1 || again.Alias != 1 || again.Unmapped != 3 || again.MissingFile != 1 {
		t.Fatalf("re-run counts: %+v", again)
	}
	if mp3Count(t, filepath.Join(data, "audio")) != 8 {
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
	if repaired.Imported != 0 || repaired.Duplicate != 8 {
		t.Fatalf("repair counts: %+v", repaired)
	}
	if body, err := os.ReadFile(v4Path); err != nil || !bytes.Equal(body, v4) {
		t.Fatalf("repaired file %q err=%v", body, err)
	}

	if err := os.WriteFile(v4Path, []byte("corrupt-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	rewritten, err := Import(ctx, later, data, export, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rewritten.Duplicate != 8 || rewritten.Imported != 0 {
		t.Fatalf("rehash counts: %+v", rewritten)
	}
	if body, err := os.ReadFile(v4Path); err != nil || !bytes.Equal(body, v4) {
		t.Fatalf("rehashed file %q err=%v", body, err)
	}
	kam, ok, err = later.AudioByText(ctx, "קָם")
	if err != nil || !ok || !kam.CreatedAt.Equal(when) || kam.ID != shaID(v4) {
		t.Fatalf("rehash changed the row: %+v ok=%v err=%v", kam, ok, err)
	}
}

func TestImportStoresClipWhenSHA256IsAbsent(t *testing.T) {
	body := []byte("no-map-sha")
	word := wordObj("קָם", "voice-a", "eleven_v4", body, "")
	delete(word, "sha256")
	export := t.TempDir()
	writeFiles(t, export, map[string][]byte{word["path"].(string): body})
	writeMap(t, export, nil, []any{word}, nil)
	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC))
	got, err := Import(context.Background(), store, data, export, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Imported != 1 || got.HashMismatch != 0 || got.TextMismatch != 0 {
		t.Fatalf("counts: %+v", got)
	}
	asset, ok, err := store.AudioByText(context.Background(), "קָם")
	if err != nil || !ok || asset.ID != shaID(body) {
		t.Fatalf("stored = %+v ok=%v err=%v", asset, ok, err)
	}
}

func TestImportMismatchesAreNotStored(t *testing.T) {
	body := []byte("good-bytes")
	badBody := []byte("other-bytes")
	const voice = "voice-a"
	export := t.TempDir()
	textBad := lessonObj("demo", "audio/story/line-1.mp3", "שָׁלוֹם", voice, "eleven_v4", body, "deadbeef", "")
	hashBad := lessonObj("demo", "audio/story/line-2.mp3", "שָׁלוֹם", voice, "eleven_v4", badBody, "", shaID(body))
	wordBad := wordObj("קָם", voice, "eleven_v4", body, "not-the-cache-key.mp3")
	writeFiles(t, export, map[string][]byte{
		textBad["path"].(string): body,
		hashBad["path"].(string): badBody,
		wordBad["path"].(string): body,
	})
	writeMap(t, export, []any{textBad, hashBad}, []any{wordBad}, nil)

	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC))
	got, err := Import(context.Background(), store, data, export, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Imported != 0 || got.TextMismatch != 2 || got.HashMismatch != 1 || got.Conflict != 0 || got.MissingFile != 0 {
		t.Fatalf("mismatch counts: %+v text=%+v hash=%+v", got, got.TextMismatches, got.HashMismatches)
	}
	if mp3Count(t, filepath.Join(data, "audio")) != 0 {
		t.Fatal("mismatch stored an mp3")
	}
	if _, ok, err := store.AudioByText(context.Background(), "שָׁלוֹם"); err != nil || ok {
		t.Fatalf("mismatched text stored ok=%v err=%v", ok, err)
	}
	raw, err := os.ReadFile(got.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte("textMismatch")) || !bytes.Contains(raw, []byte("hashMismatch")) {
		t.Fatalf("report missing mismatch lists: %s", raw)
	}
}

func TestImportArchiveAndStdin(t *testing.T) {
	export := t.TempDir()
	word := []byte("word-bytes")
	lesson := []byte("lesson-bytes")
	wordClip := wordObj("שָׁלוֹם", "voice-z", "eleven_v4", word, "")
	lessonClip := lessonObj("demo", "audio/story/line-1.mp3", "בֹּקֶר טוֹב", "voice-z", "eleven_v4", lesson, "", "")
	writeFiles(t, export, map[string][]byte{
		wordClip["path"].(string):     word,
		lessonClip["path"].(string):   lesson,
		"_audio/words/skip.mp3":       []byte("nope"),
		"demo/audio/_archive/old.mp3": []byte("old"),
	})
	writeMap(t, export, []any{lessonClip}, []any{wordClip}, []any{
		map[string]string{"reason": "word-not-in-manifest", "path": "_audio/words/skip.mp3"},
		map[string]string{"reason": "archive", "path": "demo/audio/_archive/old.mp3"},
	})
	tgz := packExport(t, export, "bundle")

	data := t.TempDir()
	store := openStoreAt(t, data, time.Date(2026, 10, 18, 8, 0, 0, 0, time.UTC))
	ctx := context.Background()
	got, err := Import(ctx, store, data, tgz, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Imported != 2 || got.Duplicate != 0 || got.Unmapped != 2 || got.MissingFile != 0 || got.Conflict != 0 {
		t.Fatalf("tarball counts: %+v", got)
	}
	asset, ok, err := store.AudioByText(ctx, "שָׁלוֹם")
	if err != nil || !ok || asset.Voice != "voice-z" || asset.Model != "eleven_v4" || asset.ID != shaID(word) {
		t.Fatalf("word from tarball = %+v ok=%v err=%v", asset, ok, err)
	}
	asset, ok, err = store.AudioByText(ctx, "בֹּקֶר טוֹב")
	if err != nil || !ok || asset.Model != "eleven_v4" || asset.Voice != "voice-z" || asset.ID != shaID(lesson) {
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
	if got.Imported != 2 || got.Unmapped != 2 {
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
	_, err := importRawTar(t, []tarFile{
		{name: "clip-text-map.json", body: v2Map(nil, nil, nil)},
		{name: "../evil.mp3", body: []byte("nope")},
	})
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

func lessonObj(lesson, clip, text, voice, model string, body []byte, textHash, sha string) map[string]any {
	if textHash == "" {
		textHash = contentHash(voice, model, text)
	}
	if sha == "" && body != nil {
		sha = shaID(body)
	}
	return map[string]any{
		"source":   "lesson",
		"lesson":   lesson,
		"clip":     clip,
		"path":     lesson + "/" + clip,
		"text":     text,
		"voiceId":  voice,
		"model":    model,
		"textHash": textHash,
		"sha256":   sha,
		"verified": true,
	}
}

func wordObj(text, voice, model string, body []byte, file string) map[string]any {
	if file == "" {
		file = wordFileName(voice, model, text)
	}
	sha := ""
	if body != nil {
		sha = shaID(body)
	}
	return map[string]any{
		"source":   "word",
		"file":     file,
		"path":     "_audio/words/" + file,
		"text":     text,
		"voiceId":  voice,
		"model":    model,
		"sha256":   sha,
		"verified": true,
	}
}

func writeMap(t *testing.T, dir string, lessons, words, unmapped []any) {
	t.Helper()
	if lessons == nil {
		lessons = []any{}
	}
	if words == nil {
		words = []any{}
	}
	if unmapped == nil {
		unmapped = []any{}
	}
	raw, err := json.Marshal(map[string]any{
		"schema":   clipMapSchema,
		"lessons":  lessons,
		"words":    words,
		"unmapped": unmapped,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "clip-text-map.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func v2Map(lessons, words, unmapped []any) []byte {
	if lessons == nil {
		lessons = []any{}
	}
	if words == nil {
		words = []any{}
	}
	if unmapped == nil {
		unmapped = []any{}
	}
	raw, err := json.Marshal(map[string]any{
		"schema":   clipMapSchema,
		"lessons":  lessons,
		"words":    words,
		"unmapped": unmapped,
	})
	if err != nil {
		panic(err)
	}
	return raw
}

func writeFiles(t *testing.T, dir string, files map[string][]byte) {
	t.Helper()
	for rel, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func assertConflict(t *testing.T, rep Report, texts ...string) {
	t.Helper()
	if len(rep.Conflicts) != 1 || len(rep.Conflicts[0].Clips) != 2 {
		t.Fatalf("conflicts = %+v", rep.Conflicts)
	}
	got := map[string]bool{}
	for _, clip := range rep.Conflicts[0].Clips {
		got[clip.Text] = true
		if clip.Path == "" {
			t.Fatalf("conflict clip missing path: %+v", clip)
		}
	}
	for _, text := range texts {
		if !got[text] {
			t.Fatalf("conflict texts = %+v, missing %s", rep.Conflicts, text)
		}
	}
	body, err := os.ReadFile(rep.ReportPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range texts {
		if !bytes.Contains(body, []byte(text)) {
			t.Fatalf("report does not list %s: %s", text, body)
		}
	}
}

func readManifest(t *testing.T, path string) []ManifestEntry {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Clips []ManifestEntry `json:"clips"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatal(err)
	}
	return parsed.Clips
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
