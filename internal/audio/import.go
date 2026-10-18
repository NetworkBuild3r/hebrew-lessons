package audio

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

const importReportName = "import-unmapped.json"

// Options controls one import.
type Options struct {
	// DryRun counts the work and writes the unmapped report. It does not
	// write mp3s or rows.
	DryRun bool
	// Archive, when set, is a gzip-compressed tar of the export. The stream
	// is not seekable. exportDir is ignored. The archive is not extracted;
	// a temporary copy of the compressed bytes is removed before Import
	// returns.
	Archive io.Reader
}

// Import reads an export directory, or a .tgz of that directory, and fills
// audio_assets plus <dataDir>/audio/<sha256>.mp3.
//
// The export root holds clip-text-map.json and files/. Word clips live at
// files/_audio/words/<file>.mp3. Lesson clips live at files/<lesson>/<clip>.
// unmapped_word_files and any path containing /_archive/ are skipped. A
// mapped entry whose text is empty after normalization is unmapped. A mapped
// file that is not on disk is a missing-file warning, not an error.
//
// The id is the hex sha256 of the bytes. The same bytes under two names are
// one row: the first file kept, in path order for a directory and in archive
// order for a .tgz. A second run inserts nothing new.
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
		defer os.Remove(spooled)
		return importArchive(ctx, store, dataDir, spooled, opts)
	}
	if exportDir == "" {
		return Report{}, errors.New("audio: export path is empty")
	}
	info, err := os.Stat(exportDir)
	if err != nil {
		return Report{}, fmt.Errorf("audio: export %s: %w", exportDir, err)
	}
	if info.IsDir() {
		return importDir(ctx, store, dataDir, exportDir, opts)
	}
	return importArchive(ctx, store, dataDir, exportDir, opts)
}

func spoolArchive(dir string, r io.Reader) (string, error) {
	f, err := os.CreateTemp(dir, ".import-stdin-*.tgz")
	if err != nil {
		return "", fmt.Errorf("audio: spool archive: %w", err)
	}
	name := f.Name()
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(name)
		return "", fmt.Errorf("audio: spool archive: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(name)
		return "", fmt.Errorf("audio: spool archive: %w", closeErr)
	}
	return name, nil
}

type clipMap struct {
	Words             map[string]wordClip `json:"words"`
	Lessons           []lessonClip        `json:"lessons"`
	UnmappedWordFiles []string            `json:"unmapped_word_files"`
}

type wordClip struct {
	Text    string `json:"text"`
	VoiceID string `json:"voiceId"`
	Model   string `json:"model"`
}

type lessonClip struct {
	Lesson string `json:"lesson"`
	Clip   string `json:"clip"`
	Text   string `json:"text"`
}

type planned struct {
	rel   string
	text  string
	voice string
	model string
}

type builder struct {
	dry         bool
	store       *db.Store
	audioDir    string
	createdAt   time.Time
	mapped      map[string]planned
	preUnmapped map[string]struct{}
	extra       []string
	seen        map[string]struct{}
	shaSeen     map[string]bool
	byModel     map[string]int

	imported        int
	duplicate       int
	skippedUnmapped int
	skippedArchive  int
}

func plan(m clipMap) (*builder, error) {
	b := &builder{
		mapped:      map[string]planned{},
		preUnmapped: map[string]struct{}{},
		seen:        map[string]struct{}{},
		shaSeen:     map[string]bool{},
		byModel:     map[string]int{},
	}
	for _, name := range m.UnmappedWordFiles {
		rel, err := wordRel(name)
		if err != nil {
			return nil, err
		}
		if relHasArchive(rel) {
			continue
		}
		b.preUnmapped[rel] = struct{}{}
	}
	names := make([]string, 0, len(m.Words))
	for name := range m.Words {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		word := m.Words[name]
		rel, err := wordRel(name)
		if err != nil {
			return nil, err
		}
		if _, skip := b.preUnmapped[rel]; skip || relHasArchive(rel) {
			continue
		}
		text := NormalizeText(word.Text)
		if text == "" {
			b.preUnmapped[rel] = struct{}{}
			continue
		}
		b.mapped[rel] = planned{
			rel:   rel,
			text:  text,
			voice: strings.TrimSpace(word.VoiceID),
			model: strings.TrimSpace(word.Model),
		}
	}
	for _, lesson := range m.Lessons {
		rel, err := lessonRel(lesson.Lesson, lesson.Clip)
		if err != nil {
			return nil, err
		}
		if relHasArchive(rel) {
			continue
		}
		if _, exists := b.mapped[rel]; exists {
			return nil, fmt.Errorf("audio: duplicate clip %s", rel)
		}
		text := NormalizeText(lesson.Text)
		if text == "" {
			b.preUnmapped[rel] = struct{}{}
			continue
		}
		b.mapped[rel] = planned{rel: rel, text: text, voice: "", model: legacyModel}
	}
	b.skippedUnmapped = len(b.preUnmapped)
	return b, nil
}

func wordRel(name string) (string, error) {
	if name == "" || path.Base(name) != name || strings.Contains(name, `\`) {
		return "", fmt.Errorf("audio: word file %q is not a single file name", name)
	}
	return path.Join("files", "_audio", "words", name), nil
}

func lessonRel(lesson, clip string) (string, error) {
	if strings.TrimSpace(lesson) == "" || strings.TrimSpace(clip) == "" {
		return "", errors.New("audio: lesson clip is missing a path")
	}
	rel := path.Join("files", lesson, clip)
	if rel == "files" || !strings.HasPrefix(rel, "files/") || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("audio: lesson path %s/%s escapes the export", lesson, clip)
	}
	return rel, nil
}

func relHasArchive(rel string) bool {
	return strings.Contains(rel, "/_archive/")
}

func importDir(ctx context.Context, store *db.Store, dataDir, exportDir string, opts Options) (Report, error) {
	m, err := loadMapFile(filepath.Join(exportDir, "clip-text-map.json"))
	if err != nil {
		return Report{}, err
	}
	b, err := newBuilder(store, dataDir, opts, m)
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
		f, err := os.Open(filepath.Join(exportDir, filepath.FromSlash(rel)))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
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
	filesDir := filepath.Join(exportDir, "files")
	info, err := os.Stat(filesDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("audio: read %s: %w", filesDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("audio: %s is not a directory", filesDir)
	}
	var rels []string
	err = filepath.WalkDir(filesDir, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".mp3") {
			return nil
		}
		rel, err := filepath.Rel(exportDir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ".." || strings.HasPrefix(rel, "../") {
			return fmt.Errorf("audio: path %s escapes the export", rel)
		}
		rels = append(rels, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("audio: walk export: %w", err)
	}
	sort.Strings(rels)
	return rels, nil
}

func importArchive(ctx context.Context, store *db.Store, dataDir, tarPath string, opts Options) (Report, error) {
	m, root, err := mapFromArchive(tarPath)
	if err != nil {
		return Report{}, err
	}
	b, err := newBuilder(store, dataDir, opts, m)
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
			if !tarRegular(header) {
				if _, err := io.Copy(io.Discard, tr); err != nil {
					return fmt.Errorf("audio: read archive: %w", err)
				}
				continue
			}
			name, err := cleanTarPath(header.Name)
			if err != nil {
				return err
			}
			rel, ok := stripRoot(name, root)
			if !ok || !strings.EqualFold(path.Ext(rel), ".mp3") {
				if _, err := io.Copy(io.Discard, tr); err != nil {
					return fmt.Errorf("audio: read archive: %w", err)
				}
				continue
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

func newBuilder(store *db.Store, dataDir string, opts Options, m clipMap) (*builder, error) {
	b, err := plan(m)
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
	if relHasArchive(rel) {
		b.skippedArchive++
		_, err := io.Copy(io.Discard, r)
		if err != nil {
			return fmt.Errorf("audio: read %s: %w", rel, err)
		}
		return nil
	}
	if _, skip := b.preUnmapped[rel]; skip {
		_, err := io.Copy(io.Discard, r)
		if err != nil {
			return fmt.Errorf("audio: read %s: %w", rel, err)
		}
		return nil
	}
	clip, ok := b.mapped[rel]
	if !ok {
		b.skippedUnmapped++
		b.extra = append(b.extra, rel)
		_, err := io.Copy(io.Discard, r)
		if err != nil {
			return fmt.Errorf("audio: read %s: %w", rel, err)
		}
		return nil
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("audio: read %s: %w", rel, err)
	}
	return b.accept(ctx, clip, body)
}

func (b *builder) accept(ctx context.Context, clip planned, body []byte) error {
	sum := sha256.Sum256(body)
	id := hex.EncodeToString(sum[:])
	b.byModel[clip.model]++
	b.seen[clip.rel] = struct{}{}
	if b.shaSeen[id] {
		b.duplicate++
		return nil
	}
	if b.dry {
		has, err := b.store.HasAudio(ctx, id)
		if err != nil {
			return err
		}
		b.shaSeen[id] = true
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
	b.shaSeen[id] = true
	if inserted {
		b.imported++
		return nil
	}
	b.duplicate++
	return nil
}

func (b *builder) finish() (Report, error) {
	missing := make([]string, 0)
	for rel := range b.mapped {
		if _, ok := b.seen[rel]; !ok {
			missing = append(missing, rel)
		}
	}
	sort.Strings(missing)
	unmapped := make([]string, 0, len(b.preUnmapped)+len(b.extra))
	for rel := range b.preUnmapped {
		unmapped = append(unmapped, rel)
	}
	unmapped = append(unmapped, b.extra...)
	sort.Strings(unmapped)
	reportPath := filepath.Join(b.audioDir, importReportName)
	rep := Report{
		Imported:        b.imported,
		Duplicate:       b.duplicate,
		SkippedUnmapped: b.skippedUnmapped,
		SkippedArchive:  b.skippedArchive,
		MissingFile:     len(missing),
		ByModel:         b.byModel,
		Unmapped:        unmapped,
		Missing:         missing,
		ReportPath:      reportPath,
	}
	if rep.ByModel == nil {
		rep.ByModel = map[string]int{}
	}
	if err := writeUnmappedReport(reportPath, unmapped, missing); err != nil {
		return Report{}, err
	}
	return rep, nil
}

func writeUnmappedReport(path string, unmapped, missing []string) error {
	if unmapped == nil {
		unmapped = []string{}
	}
	if missing == nil {
		missing = []string{}
	}
	body, err := json.MarshalIndent(struct {
		Unmapped []string `json:"unmapped"`
		Missing  []string `json:"missing"`
	}{Unmapped: unmapped, Missing: missing}, "", "  ")
	if err != nil {
		return fmt.Errorf("audio: write report: %w", err)
	}
	body = append(body, '\n')
	return writeAtomic(path, body)
}

func writeAudioFile(dir, id string, data []byte) error {
	final := filepath.Join(dir, id+".mp3")
	if _, err := os.Stat(final); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("audio: stat %s: %w", final, err)
	}
	if err := writeAtomic(final, data); err != nil {
		return err
	}
	if err := os.Chmod(final, 0o644); err != nil {
		return fmt.Errorf("audio: chmod %s: %w", final, err)
	}
	return nil
}

// writeAtomic replaces path by renaming a temp file in the same directory.
// The destination is left untouched when the write fails.
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
	return nil
}

func loadMapFile(path string) (clipMap, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return clipMap{}, fmt.Errorf("audio: read clip map: %w", err)
	}
	return decodeMap(body)
}

func decodeMap(body []byte) (clipMap, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var m clipMap
	if err := dec.Decode(&m); err != nil {
		return clipMap{}, fmt.Errorf("audio: parse clip map: %w", err)
	}
	if m.Words == nil {
		m.Words = map[string]wordClip{}
	}
	return m, nil
}

func mapFromArchive(tarPath string) (clipMap, string, error) {
	var (
		raw   []byte
		root  string
		found int
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
			if tarRegular(header) && (name == "clip-text-map.json" || strings.HasSuffix(name, "/clip-text-map.json")) {
				found++
				if found == 1 {
					body, err := io.ReadAll(tr)
					if err != nil {
						return fmt.Errorf("audio: read clip map: %w", err)
					}
					raw = body
					root = strings.TrimSuffix(name, "clip-text-map.json")
					continue
				}
			}
			if _, err := io.Copy(io.Discard, tr); err != nil {
				return fmt.Errorf("audio: read archive: %w", err)
			}
		}
	})
	if err != nil {
		return clipMap{}, "", err
	}
	if found == 0 {
		return clipMap{}, "", errors.New("audio: clip-text-map.json not found in archive")
	}
	if found > 1 {
		return clipMap{}, "", errors.New("audio: archive contains more than one clip-text-map.json")
	}
	m, err := decodeMap(raw)
	if err != nil {
		return clipMap{}, "", err
	}
	return m, root, nil
}

func tarRegular(header *tar.Header) bool {
	// A zero type flag is a regular file in the oldest tar format.
	return header.Typeflag == tar.TypeReg || header.Typeflag == 0
}

func cleanTarPath(name string) (string, error) {
	name = strings.TrimPrefix(name, "./")
	if name == "" || name == "." {
		return "", errors.New("audio: empty tar name")
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
