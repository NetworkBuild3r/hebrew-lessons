package audio

import "github.com/NetworkBuild3r/hebrew-lessons/internal/units"

// NormalizeText is the audio_assets.text_nfc form. It is units.Normalize:
// format characters (Cf) removed, NFC, trimmed, Unicode spaces collapsed
// to one ASCII space. Niqqud is kept. Callers that look up a clip pass
// this same string to AudioByText.
//
// The legacy content hash is not this string. cacheKey and textHash in
// origin/main server/audio.js hash voice, model, and String(text).trim()
// joined by NUL, with no Cf strip and no NFC.
func NormalizeText(s string) string {
	return units.Normalize(s)
}
