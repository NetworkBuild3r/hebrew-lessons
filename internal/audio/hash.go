package audio

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// contentHash is the legacy text hash: sha256 of voice, model, and the
// trimmed text, joined by NUL. origin/main server/audio.js textHash is the
// full digest. cacheKey is the first 24 hex characters and is the word
// file name without ".mp3".
func contentHash(voice, model, text string) string {
	sum := sha256.Sum256([]byte(voice + "\x00" + model + "\x00" + strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:])
}

func wordFileName(voice, model, text string) string {
	return contentHash(voice, model, text)[:24] + ".mp3"
}
