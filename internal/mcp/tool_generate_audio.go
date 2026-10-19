package mcp

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GenerateAudioRequest is what generate_audio asks the store to synthesize.
// Texts are the caller's strings. UnitID, when set, adds that unit's Hebrew
// activity texts. The adapter resolves UnitID. This package does not.
type GenerateAudioRequest struct {
	Texts  []string
	UnitID *int64
}

// AudioClip is one text after generate_audio.
// Status is generated, skipped, or failed. AudioID is set when a clip
// is stored. Error is set when Status is failed.
type AudioClip struct {
	Text    string
	TextNFC string
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

type generateAudioInput struct {
	Texts  []string `json:"texts,omitempty" jsonschema:"Pointed Hebrew to speak, in order. A text that already has a clip is skipped. At most 40 texts per call, including activity texts added for unitId."`
	UnitID *int64   `json:"unitId,omitempty" jsonschema:"Id from put_unit. Hebrew activity texts in that unit are added to texts. Omit it to generate only texts."`
}

type generateAudioItem struct {
	Text    string `json:"text"`
	TextNFC string `json:"textNfc"`
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
	if len(in.Texts) == 0 && in.UnitID == nil {
		return nil, out, errors.New("Pass texts, or a unitId.")
	}
	if in.UnitID != nil && *in.UnitID <= 0 {
		return nil, out, errors.New("unitId must be a positive id from put_unit.")
	}
	clips, err := s.audio.GenerateAudio(ctx, GenerateAudioRequest{Texts: in.Texts, UnitID: in.UnitID})
	if err != nil {
		return nil, out, err
	}
	out.Results = make([]generateAudioItem, 0, len(clips))
	for _, clip := range clips {
		out.Results = append(out.Results, generateAudioItem{
			Text:    clip.Text,
			TextNFC: clip.TextNFC,
			Status:  clip.Status,
			AudioID: clip.AudioID,
			Error:   clip.Error,
		})
	}
	return nil, out, nil
}

func registerGenerateAudio(server *mcp.Server, svc *service) {
	falseHint := false
	trueHint := true
	mcp.AddTool(server, &mcp.Tool{
		Name: "generate_audio",
		Description: "Store a spoken clip for pointed Hebrew. Admin only, including the static token: a learner call is a tool error, forbidden: admin only. " +
			"Pass texts, or unitId to add that unit's Hebrew activity texts. A text that already has a clip is skipped and the provider is not called. " +
			"Each result is generated, skipped, or failed, and includes audioId when a clip is stored. One failure does not remove clips that succeeded. At most 40 texts per call. " +
			"The clip is stored under the normalized pointed text validate_unit looks up.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &trueHint,
		},
	}, svc.generateAudio)
}
