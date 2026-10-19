package audio

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

const (
	importReportName   = "import-unmapped.json"
	importManifestName = "import-manifest.json"
)

// Caps keep a hostile or truncated archive from filling the disk.
// Tests lower them.
var (
	maxClipBytes  int64 = 32 << 20
	maxSpoolBytes int64 = 512 << 20
	maxMapBytes   int64 = 16 << 20
)

// failAtomicWrite, when set, makes writeAtomic fail after it creates the
// temp file so tests can see that the temp file is removed.
var failAtomicWrite bool

// Options controls one import.
type Options struct {
	// DryRun counts the work and writes the report and manifest. It does not
	// write mp3s or rows.
	DryRun bool
	// Archive, when set, is a gzip-compressed tar of the export. The stream
	// is not seekable. exportDir is ignored. The archive is not extracted.
	// A temporary copy of the compressed bytes is removed before Import
	// returns, including when the process is signaled.
	Archive io.Reader
}

// Import reads a v2 or v3 export directory, or a .tgz of that directory,
// and fills audio_assets plus <dataDir>/audio/<sha256>.mp3.
//
// The export root holds clip-text-map.json (schema hebrew-clip-text-map/v3,
// or v2). Clip paths in the map are relative to that root: lesson directories
// and _audio/ sit next to the map, with no files/ prefix. A single archive
// prefix, such as bundle/clip-text-map.json, is stripped.
//
// Every mapped clip is checked before it is stored. When the map entry has
// a sha256, the mp3 bytes must hash to it (hash-mismatch otherwise). An
// entry with no sha256 is still stored under the hash of its bytes.
// v3 requires hashText on every lesson and word. The digest is sha256 of
// voiceId, model, and hashText.trim() joined by NUL when hashText is
// present, and of text.trim() when it is absent. A word's file name must be
// the first 24 hex characters of that digest plus ".mp3" (text-mismatch
// otherwise). A lesson is checked against <lesson>/audio/manifest.json
// ({"version":1,"clips":{"audio/sentences/vocab-1.mp3":"<64-hex>",...}},
// keys relative to the lesson directory). The digest and the map's textHash
// must both equal manifest.clips[key]. A missing manifest, a missing key,
// or an unparseable manifest is a text-mismatch. When hashText is present,
// NormalizeText(hashText) must also equal text (text-mismatch, reason
// nfc-differs, otherwise), and text is what is stored. When hashText is
// absent, the stored text is NormalizeText(text).
//
// Unmapped entries are listed and not stored. The same bytes with two
// different texts are a conflict: both paths and texts are reported, and
// the second text is not written over the first. Distinct sha256 values are
// all stored. The text index keeps one asset per (text, voice, model), the
// lowest id, which is what AudioByText returns when created_at ties. The
// other ids are aliases in the summary and the import manifest.
func Import(ctx context.Context, store *db.Store, dataDir, exportDir string, opts Options) (Report, error) {
	if store == nil {
		return Report{}, errors.New("audio: store is nil")
	}
	if dataDir == "" {
		return Report{}, errors.New("audio: data dir is empty")
	}
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	audioDir := filepath.Join(dataDir, "audio")
	if err := os.MkdirAll(audioDir, 0o755); err != nil {
		return Report{}, fmt.Errorf("audio: create %s: %w", audioDir, err)
	}
	if opts.Archive != nil {
		spooled, err := spoolArchive(audioDir, opts.Archive)
		if err != nil {
			return Report{}, err
		}
		stop := unlinkOnSignal(spooled)
		defer stop()
		return importArchive(ctx, store, dataDir, spooled, opts)
	}
	if exportDir == "" {
		return Report{}, errors.New("audio: export path is empty")
	}
	info, err := os.Lstat(exportDir)
	if err != nil {
		return Report{}, fmt.Errorf("audio: export %s: %w", exportDir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Report{}, fmt.Errorf("audio: export %s is a symlink", exportDir)
	}
	if !info.IsDir() {
		return importArchive(ctx, store, dataDir, exportDir, opts)
	}
	return importDir(ctx, store, dataDir, exportDir, opts)
}

func spoolArchive(dir string, r io.Reader) (string, error) {
	f, err := os.CreateTemp(dir, ".import-stdin-*.tgz")
	if err != nil {
		return "", fmt.Errorf("audio: spool archive: %w", err)
	}
	name := f.Name()
	n, copyErr := io.Copy(f, io.LimitReader(r, maxSpoolBytes+1))
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(name)
		return "", fmt.Errorf("audio: spool archive: %w", copyErr)
	}
	if n > maxSpoolBytes {
		os.Remove(name)
		return "", fmt.Errorf("audio: archive exceeds %d bytes", maxSpoolBytes)
	}
	if closeErr != nil {
		os.Remove(name)
		return "", fmt.Errorf("audio: spool archive: %w", closeErr)
	}
	return name, nil
}

// unlinkOnSignal removes path if this process receives SIGINT or SIGTERM,
// and the returned function always removes it. SIGKILL cannot be caught.
func unlinkOnSignal(path string) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	var once sync.Once
	go func() {
		select {
		case <-ch:
			os.Remove(path)
		case <-done:
		}
		signal.Stop(ch)
	}()
	return func() {
		once.Do(func() { close(done) })
		signal.Stop(ch)
		os.Remove(path)
	}
}

type planned struct {
	rel   string
	text  string
	voice string
	model string
	sha   string
}

// refusedText is a clip whose legacy text hash did not match.
// sha is the map's byte hash, checked when the file is present.
type refusedText struct {
	item Mismatch
	sha  string
}

type seenSHA struct {
	rel   string
	text  string
	voice string
	model string
}

type builder struct {
	dry       bool
	store     *db.Store
	audioDir  string
	createdAt time.Time

	mapped   map[string]planned
	textBad  map[string]refusedText
	unmapped map[string]string
	seenPath map[string]struct{}
	seenSHA  map[string]seenSHA
	byModel  map[string]int

	imported  int
	duplicate int
	conflict  int

	extraUnmapped  []UnmappedItem
	hashMismatches []Mismatch
	conflicts      []Conflict
	manifest       []ManifestEntry
}

func plan(m clipMap, lessons map[string]lessonManifest) (*builder, error) {
	b := &builder{
		mapped:   map[string]planned{},
		textBad:  map[string]refusedText{},
		unmapped: map[string]string{},
		seenPath: map[string]struct{}{},
		seenSHA:  map[string]seenSHA{},
		byModel:  map[string]int{},
	}
	for _, item := range m.Unmapped {
		rel, err := cleanRel(item.Path)
		if err != nil {
			return nil, err
		}
		if _, dup := b.unmapped[rel]; dup {
			return nil, fmt.Errorf("audio: repeated path %s", rel)
		}
		reason := item.Reason
		if reason == "" {
			reason = "unmapped"
		}
		b.unmapped[rel] = reason
	}
	clips := append(append([]mappedClip{}, m.Words...), m.Lessons...)
	for _, clip := range clips {
		rel, err := cleanRel(clip.Path)
		if err != nil {
			return nil, err
		}
		if _, dup := b.mapped[rel]; dup {
			return nil, fmt.Errorf("audio: repeated path %s", rel)
		}
		if _, dup := b.textBad[rel]; dup {
			return nil, fmt.Errorf("audio: repeated path %s", rel)
		}
		if _, skip := b.unmapped[rel]; skip {
			return nil, fmt.Errorf("audio: path %s is both mapped and unmapped", rel)
		}
		voice := strings.TrimSpace(clip.VoiceID)
		model := strings.TrimSpace(clip.Model)
		fileSHA := strings.ToLower(strings.TrimSpace(clip.SHA256))
		hashInput := clip.Text
		if strings.TrimSpace(clip.HashText) != "" {
			hashInput = clip.HashText
		}
		got := contentHash(voice, model, hashInput)
		switch clip.Source {
		case "lesson":
			if !lessonTextMatches(b, rel, clip, got, fileSHA, lessons) {
				continue
			}
		case "word":
			want := wordFileName(voice, model, hashInput)
			if path.Base(rel) != want {
				b.textBad[rel] = refusedText{item: Mismatch{Path: rel, Want: want, Got: path.Base(rel)}, sha: fileSHA}
				continue
			}
		default:
			return nil, fmt.Errorf("audio: clip %s source %q", rel, clip.Source)
		}
		text := NormalizeText(clip.Text)
		if strings.TrimSpace(clip.HashText) != "" {
			if NormalizeText(clip.HashText) != clip.Text {
				b.textBad[rel] = refusedText{item: Mismatch{
					Path:   rel,
					Want:   NormalizeText(clip.HashText),
					Got:    clip.Text,
					Reason: "nfc-differs",
				}, sha: fileSHA}
				continue
			}
			text = clip.Text
		}
		if text == "" {
			b.unmapped[rel] = "empty-text"
			continue
		}
		b.mapped[rel] = planned{
			rel:   rel,
			text:  text,
			voice: voice,
			model: model,
			sha:   strings.ToLower(strings.TrimSpace(clip.SHA256)),
		}
	}
	return b, nil
}

func cleanRel(rel string) (string, error) {
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" || path.IsAbs(rel) || strings.Contains(rel, `\`) {
		return "", fmt.Errorf("audio: path %q is not a relative clip path", rel)
	}
	cleaned := path.Clean(rel)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("audio: path %q escapes the export", rel)
	}
	return cleaned, nil
}

func importDir(ctx context.Context, store *db.Store, dataDir, exportDir string, opts Options) (Report, error) {
	m, err := loadMapFile(filepath.Join(exportDir, "clip-text-map.json"))
	if err != nil {
		return Report{}, err
	}
	lessons, err := loadDirManifests(exportDir, m.Lessons)
	if err != nil {
		return Report{}, err
	}
	b, err := newBuilder(store, dataDir, opts, m, lessons)
	if err != nil {
		return Report{}, err
	}
	rels, err := listMP3(exportDir)
	if err != nil {
		return Report{}, err
	}
	for _, rel := range rels {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		if _, dup := b.seenPath[rel]; dup {
			return Report{}, fmt.Errorf("audio: repeated path %s", rel)
		}
		f, err := os.Open(filepath.Join(exportDir, filepath.FromSlash(rel)))
		if err != nil {
			return Report{}, fmt.Errorf("audio: read %s: %w", rel, err)
		}
		err = b.onFile(ctx, rel, f)
		closeErr := f.Close()
		if err != nil {
			return Report{}, err
		}
		if closeErr != nil {
			return Report{}, fmt.Errorf("audio: read %s: %w", rel, closeErr)
		}
	}
	return b.finish()
}

func listMP3(exportDir string) ([]string, error) {
	var rels []string
	err := filepath.WalkDir(exportDir, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if p == exportDir {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("audio: symlink %s", p)
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("audio: non-regular file %s", p)
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".mp3") {
			return nil
		}
		rel, err := filepath.Rel(exportDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		cleaned, err := cleanRel(rel)
		if err != nil {
			return err
		}
		rels = append(rels, cleaned)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("audio: walk export: %w", err)
	}
	sort.Strings(rels)
	return rels, nil
}

func importArchive(ctx context.Context, store *db.Store, dataDir, tarPath string, opts Options) (Report, error) {
	m, root, bodies, err := mapFromArchive(tarPath)
	if err != nil {
		return Report{}, err
	}
	lessons, err := manifestsForLessons(m.Lessons, bodies)
	if err != nil {
		return Report{}, err
	}
	b, err := newBuilder(store, dataDir, opts, m, lessons)
	if err != nil {
		return Report{}, err
	}
	err = withGzipTar(tarPath, func(tr *tar.Reader) error {
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			header, err := tr.Next()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("audio: read archive: %w", err)
			}
			if err := rejectSpecial(header); err != nil {
				return err
			}
			if header.Typeflag == tar.TypeDir {
				if err := discardEntry(tr); err != nil {
					return err
				}
				continue
			}
			if !tarRegular(header) {
				if err := discardEntry(tr); err != nil {
					return err
				}
				continue
			}
			name, err := cleanTarPath(header.Name)
			if err != nil {
				return err
			}
			rel, ok := stripRoot(name, root)
			if !ok || !strings.EqualFold(path.Ext(rel), ".mp3") {
				if err := discardEntry(tr); err != nil {
					return err
				}
				continue
			}
			if _, dup := b.seenPath[rel]; dup {
				return fmt.Errorf("audio: repeated path %s", rel)
			}
			if err := b.onFile(ctx, rel, tr); err != nil {
				return err
			}
		}
	})
	if err != nil {
		return Report{}, err
	}
	return b.finish()
}

func rejectSpecial(header *tar.Header) error {
	switch header.Typeflag {
	case tar.TypeSymlink:
		return fmt.Errorf("audio: symlink %s", header.Name)
	case tar.TypeLink:
		return fmt.Errorf("audio: hardlink %s", header.Name)
	case tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
		return fmt.Errorf("audio: non-regular file %s", header.Name)
	default:
		return nil
	}
}

func newBuilder(store *db.Store, dataDir string, opts Options, m clipMap, lessons map[string]lessonManifest) (*builder, error) {
	b, err := plan(m, lessons)
	if err != nil {
		return nil, err
	}
	if store.Clock() == nil {
		return nil, errors.New("audio: store clock is nil")
	}
	b.dry = opts.DryRun
	b.store = store
	b.audioDir = filepath.Join(dataDir, "audio")
	b.createdAt = store.Clock().Now()
	return b, nil
}

func (b *builder) onFile(ctx context.Context, rel string, r io.Reader) error {
	b.seenPath[rel] = struct{}{}
	if reason, skip := b.unmapped[rel]; skip {
		if err := discardLimited(r); err != nil {
			return fmt.Errorf("audio: read %s: %w", rel, err)
		}
		_ = reason
		return nil
	}
	if bad, skip := b.textBad[rel]; skip {
		body, err := readLimited(r)
		if err != nil {
			return fmt.Errorf("audio: read %s: %w", rel, err)
		}
		sum := sha256.Sum256(body)
		id := hex.EncodeToString(sum[:])
		if bad.sha != "" && id != bad.sha {
			b.hashMismatches = append(b.hashMismatches, Mismatch{Path: rel, Want: bad.sha, Got: id})
		}
		return nil
	}
	clip, ok := b.mapped[rel]
	if !ok {
		if err := discardLimited(r); err != nil {
			return fmt.Errorf("audio: read %s: %w", rel, err)
		}
		if strings.Contains(rel, "/_archive/") {
			b.extraUnmapped = append(b.extraUnmapped, UnmappedItem{Path: rel, Reason: "archive"})
			return nil
		}
		b.extraUnmapped = append(b.extraUnmapped, UnmappedItem{Path: rel, Reason: "not-in-map"})
		return nil
	}
	body, err := readLimited(r)
	if err != nil {
		return fmt.Errorf("audio: read %s: %w", rel, err)
	}
	sum := sha256.Sum256(body)
	id := hex.EncodeToString(sum[:])
	if clip.sha != "" && id != clip.sha {
		b.hashMismatches = append(b.hashMismatches, Mismatch{Path: rel, Want: clip.sha, Got: id})
		return nil
	}
	return b.accept(ctx, clip, id, body)
}

func (b *builder) accept(ctx context.Context, clip planned, id string, body []byte) error {
	if prev, ok := b.seenSHA[id]; ok {
		if prev.text != clip.text {
			b.addConflict(id, prev, clip)
			return nil
		}
		b.byModel[clip.model]++
		b.duplicate++
		b.manifest = append(b.manifest, manifestEntry(clip, id, false))
		return nil
	}
	b.seenSHA[id] = seenSHA{rel: clip.rel, text: clip.text, voice: clip.voice, model: clip.model}
	b.byModel[clip.model]++
	b.manifest = append(b.manifest, manifestEntry(clip, id, false))
	if b.dry {
		has, err := b.store.HasAudio(ctx, id)
		if err != nil {
			return err
		}
		if has {
			b.duplicate++
			return nil
		}
		b.imported++
		return nil
	}
	if err := writeAudioFile(b.audioDir, id, body); err != nil {
		return err
	}
	inserted, err := b.store.InsertAudio(ctx, db.Asset{
		ID:        id,
		TextNFC:   clip.text,
		Voice:     clip.voice,
		Model:     clip.model,
		Bytes:     int64(len(body)),
		CreatedAt: b.createdAt,
	})
	if err != nil {
		return err
	}
	if inserted {
		b.imported++
		return nil
	}
	b.duplicate++
	return nil
}

func manifestEntry(clip planned, id string, conflict bool) ManifestEntry {
	return ManifestEntry{
		Path:     clip.rel,
		ID:       id,
		Text:     clip.text,
		Voice:    clip.voice,
		Model:    clip.model,
		Conflict: conflict,
	}
}

func (b *builder) addConflict(id string, prev seenSHA, clip planned) {
	b.conflict++
	for i := range b.conflicts {
		if b.conflicts[i].SHA256 != id {
			continue
		}
		b.conflicts[i].Clips = append(b.conflicts[i].Clips, ConflictClip{Path: clip.rel, Text: clip.text})
		b.manifest = append(b.manifest, manifestEntry(clip, id, true))
		return
	}
	b.conflicts = append(b.conflicts, Conflict{
		SHA256: id,
		Clips: []ConflictClip{
			{Path: prev.rel, Text: prev.text},
			{Path: clip.rel, Text: clip.text},
		},
	})
	b.manifest = append(b.manifest, manifestEntry(clip, id, true))
}

func (b *builder) finish() (Report, error) {
	missing := make([]string, 0)
	for rel := range b.mapped {
		if _, ok := b.seenPath[rel]; !ok {
			missing = append(missing, rel)
		}
	}
	sort.Strings(missing)
	sort.Slice(b.hashMismatches, func(i, j int) bool {
		return b.hashMismatches[i].Path < b.hashMismatches[j].Path
	})
	textMismatch := make([]Mismatch, 0, len(b.textBad))
	for _, item := range b.textBad {
		textMismatch = append(textMismatch, item.item)
	}
	sort.Slice(textMismatch, func(i, j int) bool {
		return textMismatch[i].Path < textMismatch[j].Path
	})
	unmapped := make([]UnmappedItem, 0, len(b.unmapped)+len(b.extraUnmapped))
	for rel, reason := range b.unmapped {
		unmapped = append(unmapped, UnmappedItem{Path: rel, Reason: reason})
	}
	unmapped = append(unmapped, b.extraUnmapped...)
	sort.Slice(unmapped, func(i, j int) bool {
		if unmapped[i].Path == unmapped[j].Path {
			return unmapped[i].Reason < unmapped[j].Reason
		}
		return unmapped[i].Path < unmapped[j].Path
	})
	sort.Slice(b.conflicts, func(i, j int) bool {
		return b.conflicts[i].SHA256 < b.conflicts[j].SHA256
	})
	markAliases(b.manifest)
	alias := countAliases(b.manifest)

	reportPath := filepath.Join(b.audioDir, importReportName)
	manifestPath := filepath.Join(b.audioDir, importManifestName)
	rep := Report{
		Imported:       b.imported,
		Duplicate:      b.duplicate,
		Unmapped:       len(unmapped),
		HashMismatch:   len(b.hashMismatches),
		TextMismatch:   len(textMismatch),
		Conflict:       b.conflict,
		Alias:          alias,
		MissingFile:    len(missing),
		ByModel:        b.byModel,
		UnmappedItems:  unmapped,
		Missing:        missing,
		HashMismatches: b.hashMismatches,
		TextMismatches: textMismatch,
		Conflicts:      b.conflicts,
		ReportPath:     reportPath,
		ManifestPath:   manifestPath,
	}
	if rep.ByModel == nil {
		rep.ByModel = map[string]int{}
	}
	if rep.HashMismatches == nil {
		rep.HashMismatches = []Mismatch{}
	}
	if rep.TextMismatches == nil {
		rep.TextMismatches = []Mismatch{}
	}
	if rep.Conflicts == nil {
		rep.Conflicts = []Conflict{}
	}
	if err := writeUnmappedReport(reportPath, rep); err != nil {
		return Report{}, err
	}
	if err := writeManifest(manifestPath, b.manifest); err != nil {
		return Report{}, err
	}
	return rep, nil
}

func markAliases(entries []ManifestEntry) {
	type key struct {
		text  string
		voice string
		model string
	}
	best := map[key]string{}
	for _, e := range entries {
		if e.Conflict {
			continue
		}
		k := key{e.Text, e.Voice, e.Model}
		id, ok := best[k]
		if !ok || e.ID < id {
			best[k] = e.ID
		}
	}
	for i := range entries {
		if entries[i].Conflict {
			continue
		}
		k := key{entries[i].Text, entries[i].Voice, entries[i].Model}
		entries[i].Alias = entries[i].ID != best[k]
	}
}

func countAliases(entries []ManifestEntry) int {
	type key struct {
		text  string
		voice string
		model string
	}
	ids := map[key]map[string]struct{}{}
	for _, e := range entries {
		if e.Conflict || !e.Alias {
			continue
		}
		k := key{e.Text, e.Voice, e.Model}
		if ids[k] == nil {
			ids[k] = map[string]struct{}{}
		}
		ids[k][e.ID] = struct{}{}
	}
	n := 0
	for _, set := range ids {
		n += len(set)
	}
	return n
}

func writeUnmappedReport(path string, rep Report) error {
	unmapped := rep.UnmappedItems
	if unmapped == nil {
		unmapped = []UnmappedItem{}
	}
	missing := rep.Missing
	if missing == nil {
		missing = []string{}
	}
	hashMismatch := rep.HashMismatches
	if hashMismatch == nil {
		hashMismatch = []Mismatch{}
	}
	textMismatch := rep.TextMismatches
	if textMismatch == nil {
		textMismatch = []Mismatch{}
	}
	conflicts := rep.Conflicts
	if conflicts == nil {
		conflicts = []Conflict{}
	}
	body, err := json.MarshalIndent(struct {
		Unmapped     []UnmappedItem `json:"unmapped"`
		Missing      []string       `json:"missing"`
		HashMismatch []Mismatch     `json:"hashMismatch"`
		TextMismatch []Mismatch     `json:"textMismatch"`
		Conflicts    []Conflict     `json:"conflicts"`
	}{
		Unmapped:     unmapped,
		Missing:      missing,
		HashMismatch: hashMismatch,
		TextMismatch: textMismatch,
		Conflicts:    conflicts,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("audio: write report: %w", err)
	}
	body = append(body, '\n')
	return writeAtomic(path, body)
}

func writeManifest(path string, entries []ManifestEntry) error {
	if entries == nil {
		entries = []ManifestEntry{}
	}
	sorted := append([]ManifestEntry(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Path == sorted[j].Path {
			return sorted[i].Text < sorted[j].Text
		}
		return sorted[i].Path < sorted[j].Path
	})
	body, err := json.MarshalIndent(struct {
		Clips []ManifestEntry `json:"clips"`
	}{Clips: sorted}, "", "  ")
	if err != nil {
		return fmt.Errorf("audio: write manifest: %w", err)
	}
	body = append(body, '\n')
	return writeAtomic(path, body)
}

func readLimited(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxClipBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxClipBytes {
		return nil, fmt.Errorf("entry exceeds %d bytes", maxClipBytes)
	}
	return body, nil
}

func discardLimited(r io.Reader) error {
	n, err := io.Copy(io.Discard, io.LimitReader(r, maxClipBytes+1))
	if err != nil {
		return err
	}
	if n > maxClipBytes {
		return fmt.Errorf("entry exceeds %d bytes", maxClipBytes)
	}
	return nil
}

func discardEntry(r io.Reader) error {
	if _, err := io.Copy(io.Discard, r); err != nil {
		return fmt.Errorf("audio: read archive: %w", err)
	}
	return nil
}

func writeAudioFile(dir, id string, data []byte) error {
	final := filepath.Join(dir, id+".mp3")
	got, err := hashFile(final)
	if err == nil && got == id {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("audio: hash %s: %w", final, err)
	}
	if err := writeAtomic(final, data); err != nil {
		return err
	}
	if err := os.Chmod(final, 0o644); err != nil {
		return fmt.Errorf("audio: chmod %s: %w", final, err)
	}
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// writeAtomic replaces path by renaming a temp file in the same directory.
// The destination is left untouched when the write fails, and the temp file
// is removed. The directory is fsynced after the rename.
func writeAtomic(dest string, data []byte) error {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("audio: create %s: %w", dir, err)
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(dest)+".*")
	if err != nil {
		return fmt.Errorf("audio: write %s: %w", dest, err)
	}
	tmp := f.Name()
	ok := false
	defer func() {
		if !ok {
			f.Close()
			os.Remove(tmp)
		}
	}()
	if failAtomicWrite {
		return fmt.Errorf("audio: write %s: injected failure", dest)
	}
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("audio: write %s: %w", dest, err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("audio: write %s: %w", dest, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("audio: write %s: %w", dest, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("audio: rename %s: %w", dest, err)
	}
	ok = true
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("audio: sync %s: %w", dir, err)
	}
	return nil
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	syncErr := d.Sync()
	closeErr := d.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func loadMapFile(path string) (clipMap, error) {
	f, err := os.Open(path)
	if err != nil {
		return clipMap{}, fmt.Errorf("audio: read clip map: %w", err)
	}
	defer f.Close()
	body, err := readLimitedN(f, maxMapBytes)
	if err != nil {
		return clipMap{}, fmt.Errorf("audio: read clip map: %w", err)
	}
	return parseClipMap(body)
}

func readLimitedN(r io.Reader, max int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > max {
		return nil, fmt.Errorf("exceeds %d bytes", max)
	}
	return body, nil
}

func mapFromArchive(tarPath string) (clipMap, string, map[string][]byte, error) {
	var (
		raw      []byte
		root     string
		found    int
		manifest = map[string][]byte{}
	)
	err := withGzipTar(tarPath, func(tr *tar.Reader) error {
		for {
			header, err := tr.Next()
			if errors.Is(err, io.EOF) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("audio: read archive: %w", err)
			}
			name, err := cleanTarPath(header.Name)
			if err != nil {
				return err
			}
			if name == "." {
				if err := discardEntry(tr); err != nil {
					return err
				}
				continue
			}
			if tarRegular(header) && (name == "clip-text-map.json" || strings.HasSuffix(name, "/clip-text-map.json")) {
				found++
				if found == 1 {
					body, err := readLimitedN(tr, maxMapBytes)
					if err != nil {
						return fmt.Errorf("audio: read clip map: %w", err)
					}
					raw = body
					root = strings.TrimSuffix(name, "clip-text-map.json")
					continue
				}
			}
			if tarRegular(header) && strings.HasSuffix(name, "/audio/manifest.json") {
				if _, dup := manifest[name]; dup {
					return fmt.Errorf("audio: repeated path %s", name)
				}
				body, err := readLimitedN(tr, maxMapBytes)
				if err != nil {
					return fmt.Errorf("audio: read %s: %w", name, err)
				}
				manifest[name] = body
				continue
			}
			if err := discardEntry(tr); err != nil {
				return err
			}
		}
	})
	if err != nil {
		return clipMap{}, "", nil, err
	}
	if found == 0 {
		return clipMap{}, "", nil, errors.New("audio: clip-text-map.json not found in archive")
	}
	if found > 1 {
		return clipMap{}, "", nil, errors.New("audio: archive contains more than one clip-text-map.json")
	}
	m, err := parseClipMap(raw)
	if err != nil {
		return clipMap{}, "", nil, err
	}
	byLesson, err := manifestBodiesByLesson(root, manifest)
	if err != nil {
		return clipMap{}, "", nil, err
	}
	return m, root, byLesson, nil
}

func tarRegular(header *tar.Header) bool {
	// A zero type flag is a regular file in the oldest tar format.
	return header.Typeflag == tar.TypeReg || header.Typeflag == 0
}

func cleanTarPath(name string) (string, error) {
	name = strings.TrimPrefix(name, "./")
	if name == "" || name == "." {
		return ".", nil
	}
	if path.IsAbs(name) {
		return "", fmt.Errorf("audio: absolute tar path %q", name)
	}
	cleaned := path.Clean(name)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("audio: tar path %q escapes the archive", name)
	}
	return cleaned, nil
}

func stripRoot(name, root string) (string, bool) {
	if root == "" {
		return name, true
	}
	if !strings.HasPrefix(name, root) {
		return "", false
	}
	rel := strings.TrimPrefix(name, root)
	if rel == "" {
		return "", false
	}
	return rel, true
}

func withGzipTar(path string, fn func(*tar.Reader) error) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("audio: open archive: %w", err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("audio: open archive: %w", err)
	}
	err = fn(tar.NewReader(zr))
	closeErr := zr.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("audio: open archive: %w", closeErr)
	}
	return nil
}
