package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// INIT-015/SPEC-026: the second voice setting.
func TestConfigReadsTheMaleVoice(t *testing.T) {
	t.Setenv("DATA_DIR", "/data")
	t.Setenv("ELEVENLABS_VOICE_ID", "")
	t.Setenv("ELEVENLABS_VOICE_ID_MALE", "")
	cfg, err := configFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ElevenLabsVoiceMale != "" {
		t.Fatalf("the male voice has a default: %q", cfg.ElevenLabsVoiceMale)
	}
	if v := cfg.Voices(); v.Male != "" || v.Default() == "" {
		t.Fatalf("voices %+v", v)
	}

	t.Setenv("ELEVENLABS_VOICE_ID", " female-x ")
	t.Setenv("ELEVENLABS_VOICE_ID_MALE", "\tmale-y\n")
	cfg, err = configFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ElevenLabsVoice != "female-x" || cfg.ElevenLabsVoiceMale != "male-y" {
		t.Fatalf("voices read as %q and %q", cfg.ElevenLabsVoice, cfg.ElevenLabsVoiceMale)
	}
	if v := cfg.Voices(); v.Female != "female-x" || v.Male != "male-y" {
		t.Fatalf("Voices() = %+v", v)
	}
	// The existing setting is the female voice and is not renamed or moved.
	t.Setenv("ELEVENLABS_VOICE_ID", "")
	t.Setenv("ELEVENLABS_VOICE_ID_MALE", "male-y")
	cfg, _ = configFromEnv()
	if cfg.ElevenLabsVoice == "" || cfg.ElevenLabsVoice == "male-y" {
		t.Fatalf("the female voice with only the male one set = %q", cfg.ElevenLabsVoice)
	}
}

// Neither voice id is written to a log by the optional-config warnings.
func TestVoiceIDsAreNotLogged(t *testing.T) {
	t.Setenv("DATA_DIR", "/data")
	t.Setenv("ELEVENLABS_VOICE_ID", "female-secret-id")
	t.Setenv("ELEVENLABS_VOICE_ID_MALE", "male-secret-id")
	cfg, err := configFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	warnOptionalConfig(cfg)
	if strings.Contains(buf.String(), "secret-id") {
		t.Fatalf("a voice id was logged: %s", buf.String())
	}
}
