package audio

import (
	"errors"
	"regexp"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
)

// Voices maps the two speakers of a dialogue to ElevenLabs voice ids (INIT-015/SPEC-026).
//
// Speaker a is the female voice: ELEVENLABS_VOICE_ID, which is also the default
// voice of narration and of every clip made without a speaker. Speaker b is the
// male voice: ELEVENLABS_VOICE_ID_MALE. A speaker never takes the other's voice.
// If the male id is unset, speaker b has no voice at all and ForSpeaker says so.
// Nothing here accepts a voice id from a client: a request names a speaker, and
// this table, read from the environment, names the voice.
type Voices struct {
	// Female is ELEVENLABS_VOICE_ID. Empty means DefaultVoiceID.
	Female string
	// Male is ELEVENLABS_VOICE_ID_MALE. Empty means speaker b is not available.
	Male string
}

// Setting names, for messages. They name the setting and never its value.
const (
	EnvVoiceFemale = "ELEVENLABS_VOICE_ID"
	EnvVoiceMale   = "ELEVENLABS_VOICE_ID_MALE"
)

var (
	// ErrMaleVoiceUnset is what a speaker b clip gets when ELEVENLABS_VOICE_ID_MALE
	// is empty. generate_audio shows it to the caller.
	ErrMaleVoiceUnset = errors.New("speaker b needs the male voice, and " + EnvVoiceMale + " is not set; set it to a voice id and call again. No clip was made in another voice.")
	// ErrVoicesSame is the message when both speakers would share one voice.
	ErrVoicesSame = errors.New(EnvVoiceMale + " is the same voice as " + EnvVoiceFemale + "; a dialogue needs two different voices")
	// ErrBadSpeaker is the message for a speaker that is not a or b.
	ErrBadSpeaker = errors.New("speaker must be a or b")
	// errBadVoiceID names a configured voice id that cannot be a path segment.
	errBadVoiceID = errors.New("a configured voice id has characters a voice id cannot have")
)

// voiceIDPattern is what an ElevenLabs voice id looks like: letters, digits, hyphen,
// and underscore. It cannot hold a slash, a dot, a percent sign, or a space, so
// it is safe as one URL path segment. ElevenLabs.endpoint joins it into a path.
var voiceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidVoiceID reports whether id is safe to use as the voice segment of a provider URL.
func ValidVoiceID(id string) bool { return voiceIDPattern.MatchString(id) }

// Default is the voice of narration and of every clip made without a speaker.
func (v Voices) Default() string { return firstNonEmpty(strings.TrimSpace(v.Female), DefaultVoiceID) }

// ForSpeaker returns the voice id of speaker a or b. It returns ErrBadSpeaker for
// anything else, ErrMaleVoiceUnset for b when no male voice is configured, and
// ErrVoicesSame when both would be one voice. The error never holds a voice id.
func (v Voices) ForSpeaker(speaker string) (string, error) {
	var id string
	switch speaker {
	case units.SpeakerA:
		id = v.Default()
	case units.SpeakerB:
		id = strings.TrimSpace(v.Male)
		if id == "" {
			return "", ErrMaleVoiceUnset
		}
		if id == v.Default() {
			return "", ErrVoicesSame
		}
	default:
		return "", ErrBadSpeaker
	}
	if !ValidVoiceID(id) {
		return "", errBadVoiceID
	}
	return id, nil
}
