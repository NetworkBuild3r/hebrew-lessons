package audio

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// legacyModel is stored for lesson clips. The export has no voice or model
// for those files.
const legacyModel = "legacy"

// NormalizeText is the audio_assets.text_nfc form: NFC, trimmed, internal
// whitespace collapsed to one space. Niqqud is kept. Callers that look up
// a clip pass this same string to AudioByText.
func NormalizeText(s string) string {
	return strings.Join(strings.Fields(norm.NFC.String(s)), " ")
}
