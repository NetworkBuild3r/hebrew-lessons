package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AudioLine is one text with the speaker who says it. Speaker is "" for the
// default voice (narration, a single word), or a (female voice) or b (male voice).
type AudioLine struct {
	Text    string
	Speaker string
}

// GenerateAudioRequest is what generate_audio asks the store to synthesize.
// Texts are the caller's strings, in the default voice. Lines are texts with a
// speaker. UnitID, when set, adds that unit's Hebrew activity texts, and a
// dialogue's lines with their speakers. The adapter resolves UnitID. This
// package does not. LemmaID is resolved in this package to each form's text
// before the request is passed to the adapter (INIT-023/SPEC-006).
type GenerateAudioRequest struct {
	Texts   []string
	Lines   []AudioLine
	UnitID  *int64
	LemmaID *int64
}

// AudioClip is one text after generate_audio.
// Status is generated, skipped, or failed. AudioID is set when a clip
// is stored. Error is set when Status is failed. Speaker is "" for a text
// in the default voice.
type AudioClip struct {
	Text    string
	TextNFC string
	Speaker string
	Status  string
	AudioID string
	Error   string
}

// AudioGenerator stores clips for generate_audio.
// An implementation lives outside this package so internal/mcp does not
// import the database. A nil Audio on Options makes the tool return isError.
type AudioGenerator interface {
	GenerateAudio(ctx context.Context, req GenerateAudioRequest) ([]AudioClip, error)
}

type generateAudioLine struct {
	Text    string `json:"text" jsonschema:"Pointed Hebrew this speaker says."`
	Speaker string `json:"speaker,omitempty" jsonschema:"a is the female voice and b is the male voice. Omit it for the default voice. Anything else is refused. A voice id is never accepted."`
}

type generateAudioInput struct {
	Texts   []string            `json:"texts,omitempty" jsonschema:"Pointed Hebrew to speak in the default (female) voice, in order. A text that already has a clip is skipped. At most 40 texts and lines per call, including those added for unitId or lemmaId."`
	Lines   []generateAudioLine `json:"lines,omitempty" jsonschema:"Pointed Hebrew with a speaker, for a dialogue. The same text can be asked for as speaker a and as speaker b, and each makes its own clip. A line is skipped only when it has a clip in that speaker's voice."`
	UnitID  *int64              `json:"unitId,omitempty" jsonschema:"Id from put_unit. Hebrew activity texts in that unit are added, and each dialogue line is added with its speaker. Omit it to generate only texts, lines, or lemmaId forms."`
	LemmaID *int64              `json:"lemmaId,omitempty" jsonschema:"Lexicon id of a word. Every stored verb or noun form of that lemma is added as a text in the default voice. A form that already has a clip is skipped. Pair with texts or lines, or pass alone."`
}

type generateAudioItem struct {
	Text    string `json:"text"`
	TextNFC string `json:"textNfc"`
	Speaker string `json:"speaker,omitempty"`
	Status  string `json:"status"`
	AudioID string `json:"audioId,omitempty"`
	Error   string `json:"error,omitempty"`
}

type generateAudioOutput struct {
	Results []generateAudioItem `json:"results"`
}

func (s *service) generateAudio(ctx context.Context, _ *mcp.CallToolRequest, in generateAudioInput) (*mcp.CallToolResult, generateAudioOutput, error) {
	var out generateAudioOutput
	if _, err := s.gate(ctx, "generate_audio"); err != nil {
		return nil, out, err
	}
	if s.audio == nil {
		return nil, out, errors.New("Audio generation is not connected.")
	}
	if len(in.Texts) == 0 && len(in.Lines) == 0 && in.UnitID == nil && in.LemmaID == nil {
		return nil, out, errors.New("Pass texts, lines, a unitId, or a lemmaId.")
	}
	if in.UnitID != nil && *in.UnitID <= 0 {
		return nil, out, errors.New("unitId must be a positive id from put_unit.")
	}
	if in.LemmaID != nil && *in.LemmaID <= 0 {
		return nil, out, errors.New("lemmaId must be a positive lexicon id.")
	}
	texts := append([]string(nil), in.Texts...)
	if in.LemmaID != nil {
		formTexts, err := s.lemmaFormTexts(ctx, *in.LemmaID)
		if err != nil {
			return nil, out, err
		}
		texts = append(texts, formTexts...)
	}
	if len(texts) == 0 && len(in.Lines) == 0 && in.UnitID == nil {
		return nil, out, errors.New("Pass texts, lines, a unitId, or a lemmaId whose forms have Hebrew text.")
	}
	lines := make([]AudioLine, 0, len(in.Lines))
	for i, l := range in.Lines {
		// The speaker picks the voice, so it is checked here, before anything is
		// stored or sent to the provider. There is no field for a voice id.
		if l.Speaker != "" && !units.ValidSpeaker(l.Speaker) {
			return nil, out, fmt.Errorf("lines[%d].speaker must be a or b, or left out.", i)
		}
		lines = append(lines, AudioLine{Text: l.Text, Speaker: l.Speaker})
	}
	// Lemma forms are expanded into Texts so a fake generator never needs a
	// network call. LemmaID stays on the request for adapters that want it.
	clips, err := s.audio.GenerateAudio(ctx, GenerateAudioRequest{
		Texts: texts, Lines: lines, UnitID: in.UnitID, LemmaID: in.LemmaID,
	})
	if err != nil {
		return nil, out, err
	}
	out.Results = make([]generateAudioItem, 0, len(clips))
	for _, clip := range clips {
		out.Results = append(out.Results, generateAudioItem{
			Text:    clip.Text,
			TextNFC: clip.TextNFC,
			Speaker: clip.Speaker,
			Status:  clip.Status,
			AudioID: clip.AudioID,
			Error:   clip.Error,
		})
	}
	return nil, out, nil
}

// lemmaFormTexts is every stored form of the lemma, verb then noun, for
// generate_audio. Forms that already have a clip are still listed; the
// generator skips them. INIT-023/SPEC-006
func (s *service) lemmaFormTexts(ctx context.Context, lemmaID int64) ([]string, error) {
	store, err := s.mistakeDB()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	take := func(he string) {
		if he == "" || seen[he] {
			return
		}
		seen[he] = true
		out = append(out, he)
	}
	verbs, err := store.VerbFormsFor(ctx, lemmaID)
	if err != nil {
		return nil, storeErr("The verb forms could not be read.", err)
	}
	for _, row := range verbs {
		take(row.FormHe)
	}
	nouns, err := store.NounFormsFor(ctx, lemmaID)
	if err != nil {
		return nil, storeErr("The noun forms could not be read.", err)
	}
	for _, row := range nouns {
		take(row.FormHe)
	}
	return out, nil
}

func registerGenerateAudio(server *mcp.Server, svc *service) {
	falseHint := false
	trueHint := true
	mcp.AddTool(server, &mcp.Tool{
		Name: "generate_audio",
		Description: "Store a spoken clip for pointed Hebrew. Admin only, including the static token: a learner call is a tool error, forbidden: admin only. " +
			"Pass texts (default female voice), lines ({text, speaker}, speaker a = female voice, b = male voice), unitId to add that unit's Hebrew activity texts and each dialogue line with its speaker, or lemmaId to add every stored form of that word. " +
			"A text with no speaker is skipped when it already has a clip, and a line with a speaker is skipped only when it has a clip in that speaker's voice, so the same Hebrew can have one clip per voice. The provider is not called for a skipped clip. " +
			"Each result is generated, skipped, or failed, and includes audioId when a clip is stored. One failure does not remove clips that succeeded. " +
			"If the male voice is not configured, each speaker b line fails with an error naming ELEVENLABS_VOICE_ID_MALE and no clip is made in the other voice. " +
			"At most 40 texts and lines per call, and a call stops after about 22 seconds: lines it did not reach come back failed with 'not attempted', and calling again continues because stored clips are skipped. Ten clips in a call is the safe size. " +
			"The clip is stored under the normalized pointed text validate_unit looks up.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &trueHint,
		},
	}, svc.generateAudio)
}
