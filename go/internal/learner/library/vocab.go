package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/unicode/norm"
)

// vocabEntry is the slice of a lesson vocab item that library rows show.
type vocabEntry struct {
	Meaning string
	Sound   string
	Audio   string
}

// loadVocab merges lesson vocab the way server/vocab-index.js buildVocabIndex
// does for the fields the library reads: meaning, sound, and audio.
// A shared clip in LESSONS_DIR/_audio/words/catalog.js wins over the lesson
// item. Malformed lessons are skipped. Directory names are sorted with
// JavaScript string order so the first meaning for a repeated key is stable;
// Node's readdir order is not. Golden fixtures use one lesson.
func loadVocab(lessonsDir string) map[string]vocabEntry {
	out := map[string]vocabEntry{}
	if lessonsDir == "" {
		return out
	}
	audio := loadWordAudio(filepath.Join(lessonsDir, "_audio", "words", "catalog.js"))
	entries, err := os.ReadDir(lessonsDir)
	if err != nil {
		return out
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "" || strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		names = append(names, name)
	}
	sortJS(names)
	for _, slug := range names {
		items := loadLessonItems(filepath.Join(lessonsDir, slug, "worksheet.js"))
		for _, item := range items {
			key := exactKey(item.Word)
			if key == "" {
				continue
			}
			shared := catalogAudio(audio, key, item.Word)
			lessonAudio := item.Audio
			audioPath := shared
			if audioPath == "" {
				audioPath = lessonAudio
			}
			prev, ok := out[key]
			if !ok {
				out[key] = vocabEntry{Meaning: item.Meaning, Sound: item.Sound, Audio: audioPath}
				continue
			}
			if prev.Meaning == "" {
				prev.Meaning = item.Meaning
			}
			if prev.Sound == "" {
				prev.Sound = item.Sound
			}
			if shared != "" {
				prev.Audio = shared
			} else if prev.Audio == "" && lessonAudio != "" {
				prev.Audio = lessonAudio
			}
			out[key] = prev
		}
	}
	return out
}

type lessonItem struct {
	Word    string `json:"word"`
	Meaning string `json:"meaning"`
	Sound   string `json:"sound"`
	Audio   string `json:"audio"`
}

func loadLessonItems(path string) []lessonItem {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	payload, ok := jsonAfterAssignment(string(raw), "window.WORKSHEET")
	if !ok {
		return nil
	}
	var sheet struct {
		Vocab struct {
			Items []lessonItem `json:"items"`
		} `json:"vocab"`
	}
	if err := json.Unmarshal(payload, &sheet); err != nil {
		return nil
	}
	return sheet.Vocab.Items
}

func loadWordAudio(path string) map[string]string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	const marker = "Object.assign({}, window.HEBREW_WORD_AUDIO || {}, "
	src := string(raw)
	i := strings.Index(src, marker)
	if i < 0 {
		return nil
	}
	rest := strings.TrimSpace(src[i+len(marker):])
	rest = strings.TrimSuffix(strings.TrimSpace(rest), ";")
	rest = strings.TrimSpace(rest)
	if strings.HasSuffix(rest, ")") {
		rest = strings.TrimSpace(rest[:len(rest)-1])
	}
	var audio map[string]string
	if err := json.Unmarshal([]byte(rest), &audio); err != nil {
		return nil
	}
	return audio
}

func catalogAudio(audio map[string]string, key, word string) string {
	if audio == nil {
		return ""
	}
	if key != "" {
		if path := audio[key]; path != "" {
			return path
		}
	}
	raw := jsTrim(word)
	if raw != "" {
		if path := audio[raw]; path != "" {
			return path
		}
	}
	nfc := norm.NFC.String(raw)
	if nfc != "" && nfc != raw {
		if path := audio[nfc]; path != "" {
			return path
		}
	}
	return ""
}

// jsonAfterAssignment returns the JSON value after `name =`.
// Lesson files are `window.WORKSHEET = { ... };`.
func jsonAfterAssignment(src, name string) ([]byte, bool) {
	needle := name + " ="
	i := strings.Index(src, needle)
	if i < 0 {
		needle = name + "="
		i = strings.Index(src, needle)
		if i < 0 {
			return nil, false
		}
	}
	rest := strings.TrimSpace(src[i+len(needle):])
	if rest == "" || (rest[0] != '{' && rest[0] != '[') {
		return nil, false
	}
	end := len(rest)
	if semi := strings.LastIndex(rest, ";"); semi >= 0 {
		end = semi
	}
	body := strings.TrimSpace(rest[:end])
	if body == "" {
		return nil, false
	}
	return []byte(body), true
}

func sortJS(values []string) {
	for i := 1; i < len(values); i++ {
		value := values[i]
		j := i
		for j > 0 && jsLess(value, values[j-1]) {
			values[j] = values[j-1]
			j--
		}
		values[j] = value
	}
}
