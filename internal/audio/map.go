package audio

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const clipMapSchema = "hebrew-clip-text-map/v2"

// clipMapDocument is the clip-text-map.json shape this importer knows.
// Hebrew Dev is regenerating the bundle and will send the schema; change
// this struct and parseClipMap, and leave the rest of the importer alone.
// The file is read from the bundle root. Testdata copies are fixtures.
type clipMapDocument struct {
	Schema   string            `json:"schema"`
	Lessons  []clipMapRecord   `json:"lessons"`
	Words    []clipMapRecord   `json:"words"`
	Unmapped []clipMapUnmapped `json:"unmapped"`
}

// clipMapRecord is one mapped lesson or word. SHA256 is optional: when it
// is set, the mp3 bytes must hash to it.
type clipMapRecord struct {
	Source   string `json:"source"`
	Lesson   string `json:"lesson"`
	Clip     string `json:"clip"`
	File     string `json:"file"`
	Path     string `json:"path"`
	Text     string `json:"text"`
	VoiceID  string `json:"voiceId"`
	Model    string `json:"model"`
	TextHash string `json:"textHash"`
	SHA256   string `json:"sha256,omitempty"`
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
	Source   string
	Lesson   string
	Clip     string
	File     string
	Path     string
	Text     string
	VoiceID  string
	Model    string
	TextHash string
	SHA256   string
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
	if doc.Schema != clipMapSchema {
		return clipMap{}, fmt.Errorf("audio: clip map schema %q, want %s", doc.Schema, clipMapSchema)
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
	return out, nil
}
