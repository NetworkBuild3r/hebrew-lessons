// Package audio stores mp3 clips and synthesizes the ones that are missing.
//
// A clip's file is <data>/audio/<sha256>.mp3. That hex sha256 of the bytes
// is audio_assets.id. Callers pass the exact pointed NFC text so one clip
// serves every lesson that uses the word.
//
// Import loads clips the previous app already synthesized, so a later lookup
// can play those bytes instead of calling ElevenLabs again.
package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultVoiceID is the ElevenLabs voice the legacy clips used.
	DefaultVoiceID = "6WRlPmU1fqPwYbSxeP16"
	// DefaultModel is the ElevenLabs model the legacy clips used.
	DefaultModel = "eleven_v3"

	elevenLabsHost    = "https://api.elevenlabs.io"
	mp3OutputFormat   = "mp3_44100_128"
	synthesizeTimeout = 45 * time.Second
	maxTTSBytes       = 8 << 20
	maxTTSErrorBytes  = 2048
)

// TTS synthesizes one mp3 for an exact pointed NFC string.
// voice and model are provider ids. An empty voice or model means the
// client's default, which is the legacy voice and model.
type TTS interface {
	Synthesize(ctx context.Context, textNFC, voice, model string) ([]byte, error)
}

// ElevenLabs is the production TTS client.
// An empty APIKey does not fail construction. Synthesize reports it.
type ElevenLabs struct {
	APIKey string
	// Voice is used when Synthesize is called with an empty voice.
	// Empty means DefaultVoiceID.
	Voice string
	// Model is used when Synthesize is called with an empty model.
	// Empty means DefaultModel.
	Model string
	// BaseURL overrides the provider host. Tests point it at httptest.
	BaseURL string
	// HTTP overrides the client. Nil uses a client with a deadline.
	HTTP *http.Client
}

// NewElevenLabs returns a client that will use apiKey.
// An empty key does not call the provider; Synthesize reports that it is unset.
// Voice and model start at the legacy defaults. Callers may replace them.
func NewElevenLabs(apiKey string) *ElevenLabs {
	return &ElevenLabs{
		APIKey: apiKey,
		Voice:  DefaultVoiceID,
		Model:  DefaultModel,
	}
}

// Synthesize calls POST /v1/text-to-speech/{voice_id} and returns mp3 bytes.
// The API key is sent only in the xi-api-key header. Errors never include it.
func (e *ElevenLabs) Synthesize(ctx context.Context, textNFC, voice, model string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if e == nil || strings.TrimSpace(e.APIKey) == "" {
		return nil, errors.New("audio: elevenlabs api key is not set")
	}
	textNFC = strings.TrimSpace(textNFC)
	if textNFC == "" {
		return nil, errors.New("audio: text is required")
	}
	voice = firstNonEmpty(strings.TrimSpace(voice), strings.TrimSpace(e.Voice), DefaultVoiceID)
	model = firstNonEmpty(strings.TrimSpace(model), strings.TrimSpace(e.Model), DefaultModel)

	payload, err := json.Marshal(struct {
		Text         string `json:"text"`
		ModelID      string `json:"model_id"`
		LanguageCode string `json:"language_code"`
	}{
		Text:         textNFC,
		ModelID:      model,
		LanguageCode: "he",
	})
	if err != nil {
		return nil, errors.New("audio: elevenlabs request could not be encoded")
	}

	endpoint, err := e.endpoint(voice)
	if err != nil {
		return nil, e.clean(err)
	}
	ctx, cancel := context.WithTimeout(ctx, synthesizeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, e.clean(err)
	}
	req.Header.Set("xi-api-key", e.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	client := e.HTTP
	if client == nil {
		client = &http.Client{Timeout: synthesizeTimeout + 5*time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, e.clean(err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		detail := e.errorDetail(res.Body)
		if detail == "" {
			return nil, fmt.Errorf("audio: elevenlabs HTTP %d", res.StatusCode)
		}
		return nil, errors.New(e.redact(fmt.Sprintf("audio: elevenlabs HTTP %d: %s", res.StatusCode, detail)))
	}
	body, err := readTTS(res.Body)
	if err != nil {
		return nil, e.clean(fmt.Errorf("audio: elevenlabs response: %w", err))
	}
	if len(body) == 0 {
		return nil, errors.New("audio: elevenlabs returned no audio")
	}
	return body, nil
}

func (e *ElevenLabs) endpoint(voice string) (string, error) {
	base := strings.TrimSpace(e.BaseURL)
	if base == "" {
		base = elevenLabsHost
	}
	raw, err := url.JoinPath(base, "v1", "text-to-speech", voice)
	if err != nil {
		return "", errors.New("audio: elevenlabs voice id is not a valid path")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("audio: elevenlabs voice id is not a valid path")
	}
	q := u.Query()
	q.Set("output_format", mp3OutputFormat)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (e *ElevenLabs) errorDetail(r io.Reader) string {
	body, err := io.ReadAll(io.LimitReader(r, maxTTSErrorBytes))
	if err != nil || len(body) == 0 {
		return ""
	}
	detail := elevenMessage(body)
	detail = strings.TrimSpace(e.redact(detail))
	if len(detail) > 300 {
		detail = detail[:300]
	}
	return detail
}

func elevenMessage(body []byte) string {
	var parsed struct {
		Detail  json.RawMessage `json:"detail"`
		Message string          `json:"message"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return string(body)
	}
	if msg := strings.TrimSpace(parsed.Message); msg != "" {
		return msg
	}
	if len(bytes.TrimSpace(parsed.Detail)) == 0 {
		return string(body)
	}
	var asString string
	if json.Unmarshal(parsed.Detail, &asString) == nil && strings.TrimSpace(asString) != "" {
		return asString
	}
	var obj struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	if json.Unmarshal(parsed.Detail, &obj) == nil {
		if obj.Code == "paid_plan_required" {
			return "this voice needs a paid ElevenLabs plan; set ELEVENLABS_VOICE_ID to a voice the plan can use"
		}
		if strings.TrimSpace(obj.Message) != "" {
			return obj.Message
		}
	}
	return string(parsed.Detail)
}

func readTTS(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxTTSBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxTTSBytes {
		return nil, errors.New("response is too large")
	}
	return body, nil
}

func (e *ElevenLabs) clean(err error) error {
	if err == nil {
		return nil
	}
	if e == nil || e.APIKey == "" || !strings.Contains(err.Error(), e.APIKey) {
		return err
	}
	return errors.New(e.redact(err.Error()))
}

func (e *ElevenLabs) redact(msg string) string {
	if e == nil || e.APIKey == "" {
		return msg
	}
	return strings.ReplaceAll(msg, e.APIKey, "[redacted]")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
