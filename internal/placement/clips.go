package placement

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/audio"
)

// ManifestSchema names the layout of audio-manifest.json.
const ManifestSchema = "placement-audio-manifest/v1"

// Clip kinds in the manifest.
const (
	ClipWord   = "word"
	ClipLetter = "letter"
)

//go:embed letters.json
var lettersJSON []byte

// manifestJSON is audio-manifest.json compiled into the binary. The image is
// distroless and has no repository checkout, so `hebrew audio-ensure` reads the
// list of clips from here (INIT-015/SPEC-026).
//
//go:embed audio-manifest.json
var manifestJSON []byte

// EmbeddedManifest is the committed clip manifest, as compiled into this binary.
func EmbeddedManifest() (ClipManifest, error) {
	return ParseManifest(manifestJSON)
}

// Letter is one Part 0 item: the learner hears SyllableText and picks
// LetterNFC (a letter with its vowel) from LetterNFC and Distractors.
// SyllableText is the exact pointed text the clip is generated from.
// Sound is the Latin spelling of the syllable, for people reading the file.
type Letter struct {
	ID           string   `json:"id"`
	LetterNFC    string   `json:"letter_nfc"`
	SyllableText string   `json:"syllable_text"`
	Sound        string   `json:"sound"`
	Distractors  []string `json:"distractors"`
}

// Clip is one audio clip the deck needs. Text is the audio_assets.text_nfc
// key: audio.NormalizeText of the pointed text.
type Clip struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Text string `json:"text_nfc"`
}

// ClipManifest is the committed list of every clip the deck needs.
// It is derived from items.json and letters.json by BuildManifest, and
// a test fails when the committed file is out of date.
type ClipManifest struct {
	Schema string `json:"schema"`
	Count  int    `json:"count"`
	Clips  []Clip `json:"clips"`
}

// ParseLetters decodes letters.json and rejects unknown fields.
func ParseLetters(body []byte) ([]Letter, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var out []Letter
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("placement: letters: %w", err)
	}
	return out, nil
}

// Letters is the embedded Part 0 letter items, in file order.
func Letters() ([]Letter, error) {
	return ParseLetters(lettersJSON)
}

func parseItems(body []byte) ([]fixtureItem, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var items []fixtureItem
	if err := dec.Decode(&items); err != nil {
		return nil, fmt.Errorf("placement: items: %w", err)
	}
	return items, nil
}

// requiredClips lists one clip per word, in items.json order, then one per
// letter item, in letters.json order.
func requiredClips(items []fixtureItem, letters []Letter) []Clip {
	out := make([]Clip, 0, len(items)+len(letters))
	for _, it := range items {
		out = append(out, Clip{Kind: ClipWord, ID: it.ID, Text: audio.NormalizeText(it.TextNFC)})
	}
	for _, l := range letters {
		out = append(out, Clip{Kind: ClipLetter, ID: l.ID, Text: audio.NormalizeText(l.SyllableText)})
	}
	return out
}

// BuildManifest derives the clip manifest from the bytes of items.json and
// letters.json.
func BuildManifest(items, letters []byte) (ClipManifest, error) {
	it, err := parseItems(items)
	if err != nil {
		return ClipManifest{}, err
	}
	lt, err := ParseLetters(letters)
	if err != nil {
		return ClipManifest{}, err
	}
	clips := requiredClips(it, lt)
	return ClipManifest{Schema: ManifestSchema, Count: len(clips), Clips: clips}, nil
}

// Marshal is the committed form: indented JSON, Hebrew unescaped, one
// trailing newline.
func (m ClipManifest) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ParseManifest decodes audio-manifest.json and rejects unknown fields.
func ParseManifest(body []byte) (ClipManifest, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var m ClipManifest
	if err := dec.Decode(&m); err != nil {
		return ClipManifest{}, fmt.Errorf("placement: manifest: %w", err)
	}
	return m, nil
}

// MissingAudio returns the clips whose text is not a key in available,
// in manifest order. available holds audio_assets.text_nfc values.
func MissingAudio(clips []Clip, available map[string]struct{}) []Clip {
	var missing []Clip
	for _, c := range clips {
		if _, ok := available[c.Text]; !ok {
			missing = append(missing, c)
		}
	}
	return missing
}

// AvailableSet builds the lookup MissingAudio takes from clip texts.
// Each text is normalized the way audio_assets stores it.
func AvailableSet(texts []string) map[string]struct{} {
	set := make(map[string]struct{}, len(texts))
	for _, t := range texts {
		set[audio.NormalizeText(t)] = struct{}{}
	}
	return set
}
