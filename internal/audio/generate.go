package audio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
)

const (
	// MaxTexts is how many clips one Generate call will accept.
	// Unit 1 is 20 texts. A larger batch waits for another call so one
	// request cannot hold the provider open without a bound.
	MaxTexts = 40

	// StatusGenerated means this call stored a new clip.
	StatusGenerated = "generated"
	// StatusSkipped means a clip for this text was already stored.
	// Synthesize was not called.
	StatusSkipped = "skipped"
	// StatusFailed means this text was not stored. Earlier texts in the
	// same call stay stored.
	StatusFailed = "failed"
)

// Result is one text from Generate, in request order.
// Text is the caller's string. TextNFC is the audio_assets key, the same
// string units.Normalize produces and validate passes to HasAudio.
// AudioID is set when a clip is stored. Detail is set when Status is failed.
type Result struct {
	Text    string
	TextNFC string
	Status  string
	AudioID string
	Detail  string
}

// Generator synthesizes missing clips and stores them.
// A text that already has a row in audio_assets is skipped.
// TTS nil fails those texts at call time. It does not stop the process
// from starting, and it does not skip the lookup of clips already stored.
type Generator struct {
	TTS   TTS
	Store *db.Store
	// Dir is the directory of <sha256>.mp3 files, <DataDir>/audio.
	Dir   string
	Voice string
	Model string
}

// Generate stores one clip per text. texts that normalize to the same key
// share one clip: the first stores it, and a later copy is skipped.
// A provider or database failure is that item's StatusFailed. Items that
// already succeeded stay stored. There is no batch transaction.
func (g *Generator) Generate(ctx context.Context, texts []string) ([]Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if g == nil || g.Store == nil {
		return nil, errors.New("audio: generator is not configured")
	}
	if g.Dir == "" {
		return nil, errors.New("audio: audio directory is empty")
	}
	if len(texts) == 0 {
		return nil, errors.New("audio: at least one text is required")
	}
	if len(texts) > MaxTexts {
		return nil, fmt.Errorf("audio: at most %d texts", MaxTexts)
	}
	voice := firstNonEmpty(g.Voice, DefaultVoiceID)
	model := firstNonEmpty(g.Model, DefaultModel)
	out := make([]Result, 0, len(texts))
	for _, text := range texts {
		if err := ctx.Err(); err != nil {
			out = append(out, failRemaining(texts[len(out):], err.Error())...)
			return out, nil
		}
		out = append(out, g.one(ctx, text, voice, model))
	}
	return out, nil
}

func (g *Generator) one(ctx context.Context, text, voice, model string) Result {
	key := NormalizeText(text)
	item := Result{Text: text, TextNFC: key}
	if key == "" {
		item.Status = StatusFailed
		item.Detail = "text is empty"
		return item
	}
	existing, ok, err := g.Store.AudioByText(ctx, key)
	if err != nil {
		slog.Error("audio lookup", "err", err)
		item.Status = StatusFailed
		item.Detail = "The clip could not be read."
		return item
	}
	if ok {
		item.Status = StatusSkipped
		item.AudioID = existing.ID
		return item
	}
	if g.TTS == nil {
		item.Status = StatusFailed
		item.Detail = "audio: elevenlabs api key is not set"
		return item
	}
	body, err := g.TTS.Synthesize(ctx, key, voice, model)
	if err != nil {
		item.Status = StatusFailed
		item.Detail = err.Error()
		return item
	}
	if len(body) == 0 {
		item.Status = StatusFailed
		item.Detail = "audio: elevenlabs returned no audio"
		return item
	}
	sum := sha256.Sum256(body)
	id := hex.EncodeToString(sum[:])
	if err := writeAudioFile(g.Dir, id, body); err != nil {
		slog.Error("audio write", "err", err)
		item.Status = StatusFailed
		item.Detail = "The clip could not be stored."
		return item
	}
	inserted, err := g.Store.InsertAudio(ctx, db.Asset{
		ID:      id,
		TextNFC: key,
		Voice:   voice,
		Model:   model,
		Bytes:   int64(len(body)),
	})
	if err != nil {
		slog.Error("audio insert", "err", err)
		item.Status = StatusFailed
		item.Detail = "The clip could not be stored."
		return item
	}
	if !inserted {
		stored, found, err := g.Store.AudioByText(ctx, key)
		if err != nil {
			slog.Error("audio lookup after insert", "err", err)
			item.Status = StatusFailed
			item.Detail = "The clip could not be stored."
			return item
		}
		if !found || stored.ID == "" {
			item.Status = StatusFailed
			item.Detail = "The clip bytes are already stored for different text."
			return item
		}
		id = stored.ID
	}
	item.Status = StatusGenerated
	item.AudioID = id
	return item
}

func failRemaining(texts []string, detail string) []Result {
	out := make([]Result, 0, len(texts))
	for _, text := range texts {
		out = append(out, Result{
			Text:    text,
			TextNFC: NormalizeText(text),
			Status:  StatusFailed,
			Detail:  detail,
		})
	}
	return out
}
