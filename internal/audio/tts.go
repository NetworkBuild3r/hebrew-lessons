// Package audio stores mp3 clips and synthesizes the ones that are missing.
//
// A clip's file is <data>/audio/<sha256>.mp3. That hex sha256 of the bytes
// is audio_assets.id. Callers pass the exact pointed NFC text so one clip
// serves every lesson that uses the word.
package audio

import (
	"context"
	"errors"
)

// TTS synthesizes one mp3 for an exact pointed NFC string.
// voice and model are provider ids.
type TTS interface {
	Synthesize(ctx context.Context, textNFC, voice, model string) ([]byte, error)
}

// ElevenLabs is the production TTS client.
// Synthesize is stubbed until the audio store is wired to the provider.
type ElevenLabs struct {
	APIKey string
}

// NewElevenLabs returns a client that will use apiKey.
// An empty key does not call the provider; Synthesize reports that it is unset.
func NewElevenLabs(apiKey string) *ElevenLabs {
	return &ElevenLabs{APIKey: apiKey}
}

// Synthesize will call the ElevenLabs text-to-speech API.
// It is not implemented in this skeleton.
func (e *ElevenLabs) Synthesize(ctx context.Context, textNFC, voice, model string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if e == nil || e.APIKey == "" {
		return nil, errors.New("audio: elevenlabs api key is not set")
	}
	if textNFC == "" || voice == "" || model == "" {
		return nil, errors.New("audio: text, voice, and model are required")
	}
	return nil, errors.New("audio: elevenlabs synthesize is not implemented")
}
