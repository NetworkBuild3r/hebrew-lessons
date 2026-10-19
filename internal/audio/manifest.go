package audio

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// errUnparseable marks a lesson manifest that exists but is not the
// version-1 clips object. Callers record a text-mismatch per clip.
var errUnparseable = errors.New("unparseable lesson manifest")

// lessonManifest is one lesson's audio/manifest.json. bad is empty when
// clips is usable. "missing" and "unparseable" refuse every clip in the lesson.
type lessonManifest struct {
	clips map[string]string
	bad   string
}

func lessonTextMatches(b *builder, rel string, clip mappedClip, got, fileSHA string, lessons map[string]lessonManifest) bool {
	man, ok := lessons[clip.Lesson]
	if !ok || man.bad == "missing" {
		b.textBad[rel] = refusedText{item: Mismatch{Path: rel, Reason: "manifest-missing"}, sha: fileSHA}
		return false
	}
	if man.bad != "" {
		b.textBad[rel] = refusedText{item: Mismatch{Path: rel, Reason: "manifest-unparseable"}, sha: fileSHA}
		return false
	}
	key := lessonClipKey(clip.Lesson, rel)
	want, ok := man.clips[key]
	if key == "" || !ok {
		b.textBad[rel] = refusedText{item: Mismatch{Path: rel, Reason: "missing-key"}, sha: fileSHA}
		return false
	}
	mapHash := strings.ToLower(strings.TrimSpace(clip.TextHash))
	if got != want || mapHash != want {
		shown := got
		if got == want {
			shown = mapHash
		}
		b.textBad[rel] = refusedText{item: Mismatch{Path: rel, Want: want, Got: shown}, sha: fileSHA}
		return false
	}
	return true
}

// lessonClipKey is the manifest key: the clip path relative to the lesson directory.
func lessonClipKey(lesson, rel string) string {
	prefix := lesson + "/"
	if lesson == "" || !strings.HasPrefix(rel, prefix) {
		return ""
	}
	return strings.TrimPrefix(rel, prefix)
}

func validLessonID(lesson string) bool {
	if lesson == "" || lesson == "." || lesson == ".." {
		return false
	}
	return !strings.ContainsAny(lesson, `/\`)
}

func loadDirManifests(exportDir string, lessons []mappedClip) (map[string]lessonManifest, error) {
	out := map[string]lessonManifest{}
	for _, clip := range lessons {
		if _, done := out[clip.Lesson]; done {
			continue
		}
		if !validLessonID(clip.Lesson) {
			return nil, fmt.Errorf("audio: lesson %q is not a directory name", clip.Lesson)
		}
		body, err := readManifestFile(filepath.Join(exportDir, clip.Lesson, "audio", "manifest.json"))
		if err := noteManifest(out, clip.Lesson, body, err); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func manifestsForLessons(lessons []mappedClip, bodies map[string][]byte) (map[string]lessonManifest, error) {
	out := map[string]lessonManifest{}
	for _, clip := range lessons {
		if _, done := out[clip.Lesson]; done {
			continue
		}
		if !validLessonID(clip.Lesson) {
			return nil, fmt.Errorf("audio: lesson %q is not a directory name", clip.Lesson)
		}
		body, ok := bodies[clip.Lesson]
		if !ok {
			out[clip.Lesson] = lessonManifest{bad: "missing"}
			continue
		}
		if err := noteManifest(out, clip.Lesson, body, nil); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func noteManifest(out map[string]lessonManifest, lesson string, body []byte, readErr error) error {
	if readErr != nil {
		if errors.Is(readErr, os.ErrNotExist) {
			out[lesson] = lessonManifest{bad: "missing"}
			return nil
		}
		if errors.Is(readErr, errUnparseable) {
			out[lesson] = lessonManifest{bad: "unparseable"}
			return nil
		}
		return readErr
	}
	clips, err := parseLessonManifest(body)
	if err != nil {
		out[lesson] = lessonManifest{bad: "unparseable"}
		return nil
	}
	out[lesson] = lessonManifest{clips: clips}
	return nil
}

func readManifestFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("audio: symlink %s", path)
	}
	if !info.Mode().IsRegular() {
		return nil, errUnparseable
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	body, err := readLimitedN(f, maxMapBytes)
	if err != nil {
		return nil, fmt.Errorf("audio: read %s: %w", path, err)
	}
	return body, nil
}

func manifestBodiesByLesson(root string, bodies map[string][]byte) (map[string][]byte, error) {
	out := map[string][]byte{}
	for name, body := range bodies {
		rel, ok := stripRoot(name, root)
		if !ok {
			continue
		}
		const suffix = "/audio/manifest.json"
		if !strings.HasSuffix(rel, suffix) {
			continue
		}
		lesson := strings.TrimSuffix(rel, suffix)
		if !validLessonID(lesson) {
			continue
		}
		if _, dup := out[lesson]; dup {
			return nil, fmt.Errorf("audio: repeated path %s", rel)
		}
		out[lesson] = body
	}
	return out, nil
}

func parseLessonManifest(body []byte) (map[string]string, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var doc struct {
		Version int               `json:"version"`
		Clips   map[string]string `json:"clips"`
	}
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, errors.New("trailing json")
	}
	if doc.Version != 1 || doc.Clips == nil {
		return nil, errors.New("manifest shape")
	}
	out := make(map[string]string, len(doc.Clips))
	for key, hash := range doc.Clips {
		cleaned, err := cleanRel(key)
		if err != nil {
			return nil, err
		}
		if _, dup := out[cleaned]; dup {
			return nil, fmt.Errorf("duplicate key %s", cleaned)
		}
		out[cleaned] = strings.ToLower(strings.TrimSpace(hash))
	}
	return out, nil
}
