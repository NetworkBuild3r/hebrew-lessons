package mcp

import (
	"context"
	"errors"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/lexicon"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type lexemeInput struct {
	TextNFC  string  `json:"text_nfc" jsonschema:"Pointed Hebrew. Stored with the audio importer's normalization: format characters removed, Unicode NFC, trimmed, and spaces collapsed."`
	Bare     *string `json:"bare,omitempty" jsonschema:"Unpointed spelling. Omit it and the consonant skeleton of text_nfc is stored. Shin and sin dots stay."`
	GlossEN  string  `json:"gloss_en" jsonschema:"Short English gloss."`
	Translit *string `json:"translit,omitempty" jsonschema:"Latin transliteration. Omit it to leave the field empty."`
	POS      string  `json:"pos,omitempty" jsonschema:"Part of speech, such as noun, verb, pron, or adv."`
	Gender   *string `json:"gender,omitempty" jsonschema:"m, f, or both. Omit it when the word has no gender."`
	Plural   *string `json:"plural,omitempty" jsonschema:"Pointed plural. Omit it, or send null, when the entry has none."`
	Verified *bool   `json:"verified,omitempty" jsonschema:"True once a person has confirmed the entry. Omit it to leave an existing flag unchanged. A new row is unverified when this is omitted."`
}

type upsertLexemesInput struct {
	Entries []lexemeInput `json:"entries" jsonschema:"Rows to insert or update, in one transaction. One invalid row rejects the batch and writes nothing."`
}

type upsertLexemesOutput struct {
	Entries      []LexemeResult `json:"entries"`
	RebuiltUnits []int64        `json:"rebuiltUnits"`
}

func (s *service) upsertLexemes(ctx context.Context, _ *mcp.CallToolRequest, in upsertLexemesInput) (*mcp.CallToolResult, upsertLexemesOutput, error) {
	var out upsertLexemesOutput
	if _, err := s.gate(ctx, "upsert_lexemes"); err != nil {
		return nil, out, err
	}
	if s.lexemes == nil {
		return nil, out, errors.New("The lexicon is not connected, so nothing was written.")
	}
	saved, rebuilt, err := s.lexemes.UpsertLexemes(ctx, lexemeInputs(in.Entries))
	if err != nil {
		var bad *lexicon.Invalid
		if errors.As(err, &bad) {
			return nil, out, bad
		}
		return nil, out, storeErr("The lexicon could not be saved.", err)
	}
	if saved == nil {
		saved = []LexemeResult{}
	}
	if rebuilt == nil {
		rebuilt = []int64{}
	}
	out.Entries = saved
	out.RebuiltUnits = rebuilt
	return nil, out, nil
}

func lexemeInputs(in []lexemeInput) []lexicon.Input {
	out := make([]lexicon.Input, len(in))
	for i, row := range in {
		out[i] = lexicon.Input{
			TextNFC:  row.TextNFC,
			Bare:     row.Bare,
			Gloss:    row.GlossEN,
			Translit: row.Translit,
			POS:      row.POS,
			Gender:   row.Gender,
			Plural:   row.Plural,
			Verified: row.Verified,
		}
	}
	return out
}

func registerUpsertLexemes(server *mcp.Server, svc *service) {
	falseHint := false
	mcp.AddTool(server, &mcp.Tool{
		Name:        "upsert_lexemes",
		Description: "Insert or update lexicon entries. Admin only, including the static token: a learner call is a tool error, forbidden: admin only. entries is one transaction. text_nfc is stored with the audio importer's normalization, and bare is the unpointed form when you omit it. The unique key is text_nfc. Sending the same row again reports unchanged and does not write it. One invalid entry rejects the batch and writes nothing; the error names that entry with a JSON pointer. After a successful batch, lesson_items are rebuilt for every unit whose document references an entry by its pointed text or its bare form. rebuiltUnits lists those unit ids. This does not add a unit revision and does not publish.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &falseHint,
		},
	}, svc.upsertLexemes)
}
