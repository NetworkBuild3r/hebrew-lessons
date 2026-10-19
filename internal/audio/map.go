package audio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	clipMapSchemaV2 = "hebrew-clip-text-map/v2"
	clipMapSchemaV3 = "hebrew-clip-text-map/v3"
	clipMapSchemaV4 = "hebrew-clip-text-map/v4"
)

// clipMapDocument is the clip-text-map.json shape this importer knows.
// v3 is v2 plus a required hashText on every lesson and word. v4 adds
// the bundle path of the clip's old manifest, and recovered words whose
// manifest is null. v2 still accepts a missing hashText. The file is
// read from the bundle root. Testdata copies are fixtures.
type clipMapDocument struct {
	Schema   string            `json:"schema"`
	Lessons  []clipMapRecord   `json:"lessons"`
	Words    []clipMapRecord   `json:"words"`
	Unmapped []clipMapUnmapped `json:"unmapped"`
}

// clipMapRecord is one mapped lesson or word. SHA256 is optional: when it
// is set, the mp3 bytes must hash to it. HashText, when set, is the string
// the legacy digest was computed from. v3 requires it. Manifest is the
// bundle path of the clip's old manifest (v4). Recovered names how a v4
// word with a null manifest was found. Both are pointers so JSON null
// stays distinct from an empty string.
type clipMapRecord struct {
	Source    string  `json:"source"`
	Lesson    string  `json:"lesson"`
	Clip      string  `json:"clip"`
	File      string  `json:"file"`
	Path      string  `json:"path"`
	Text      string  `json:"text"`
	HashText  string  `json:"hashText,omitempty"`
	VoiceID   string  `json:"voiceId"`
	Model     string  `json:"model"`
	TextHash  string  `json:"textHash"`
	SHA256    string  `json:"sha256,omitempty"`
	Manifest  *string `json:"manifest,omitempty"`
	Recovered *string `json:"recovered,omitempty"`
}

type clipMapUnmapped struct {
	Reason string `json:"reason"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
}

// clipMap is the parsed map the importer plans from.
type clipMap struct {
	Schema   string
	Lessons  []mappedClip
	Words    []mappedClip
	Unmapped []unmappedClip
}

type mappedClip struct {
	Source    string
	Lesson    string
	Clip      string
	File      string
	Path      string
	Text      string
	HashText  string
	VoiceID   string
	Model     string
	TextHash  string
	SHA256    string
	Manifest  *string
	Recovered *string
}

type unmappedClip struct {
	Reason string
	Path   string
	SHA256 string
}

// parseClipMap decodes a bundle's clip-text-map.json.
func parseClipMap(body []byte) (clipMap, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var doc clipMapDocument
	if err := dec.Decode(&doc); err != nil {
		return clipMap{}, fmt.Errorf("audio: parse clip map: %w", err)
	}
	switch doc.Schema {
	case clipMapSchemaV2, clipMapSchemaV3, clipMapSchemaV4:
	default:
		return clipMap{}, fmt.Errorf("audio: clip map schema %q, want %s, %s, or %s", doc.Schema, clipMapSchemaV2, clipMapSchemaV3, clipMapSchemaV4)
	}
	out := clipMap{
		Schema:   doc.Schema,
		Lessons:  make([]mappedClip, len(doc.Lessons)),
		Words:    make([]mappedClip, len(doc.Words)),
		Unmapped: make([]unmappedClip, len(doc.Unmapped)),
	}
	for i, rec := range doc.Lessons {
		out.Lessons[i] = mappedClip(rec)
	}
	for i, rec := range doc.Words {
		out.Words[i] = mappedClip(rec)
	}
	for i, rec := range doc.Unmapped {
		out.Unmapped[i] = unmappedClip(rec)
	}
	if doc.Schema == clipMapSchemaV3 {
		if err := requireHashText(out.Lessons); err != nil {
			return clipMap{}, err
		}
		if err := requireHashText(out.Words); err != nil {
			return clipMap{}, err
		}
	}
	return out, nil
}

func requireHashText(clips []mappedClip) error {
	for _, clip := range clips {
		if strings.TrimSpace(clip.HashText) != "" {
			continue
		}
		rel := clip.Path
		if rel == "" {
			rel = clip.File
		}
		return fmt.Errorf("audio: clip %s: v3 hashText is required", rel)
	}
	return nil
}
