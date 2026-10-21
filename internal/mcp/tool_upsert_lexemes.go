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
	Translit *string `json:"translit,omitempty" jsonschema:"Required for a new lexeme. A missing or empty value rejects the batch. Omit it on an update to keep the stored value. Send an empty string on an update to clear it. Lowercase ASCII: kh for ח and for soft כ or ך, ts for צ or ץ, an apostrophe (') for a glottal stop. Hyphenate prefixes and give each its contextual vowel, e.g. ve-/u-, be-/bi-, mi-/me-, la-/ba- (with the article); these examples are illustrative, not a fixed list."`
	POS      *string `json:"pos,omitempty" jsonschema:"Part of speech, such as noun, verb, pron, or adv. Omit it on an update to keep the stored value. Send an empty string to clear it."`
	Gender   *string `json:"gender,omitempty" jsonschema:"m, f, or both. Omit it on an update to keep the stored value. Send an empty string to clear it. A new row has no gender when this is omitted."`
	Plural   *string `json:"plural,omitempty" jsonschema:"Pointed Hebrew plural. Omit it on an update to keep the stored value. Send an empty string to clear it. A new row has no plural when this is omitted."`
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
		Description: "Insert or update lexicon entries. Admin only, including the static token: a learner call is a tool error, forbidden: admin only. entries is one transaction of at most 200 rows. text_nfc is stored with the audio importer's normalization, and bare is the unpointed form when you omit it. text_nfc and a set plural must be Hebrew. The unique key is text_nfc. Sending the same row again reports unchanged and does not write it. A new lexeme requires translit. Omitting it, or sending it empty, rejects the batch. On an update, omit pos, gender, plural, translit, or verified to keep the stored value. Send a value, including an empty string for the text fields, to replace it. Write translit in lowercase ASCII: kh for ח and for soft כ or ך, ts for צ or ץ, an apostrophe (') for a glottal stop. Hyphenate prefixes and give each its contextual vowel, e.g. ve-/u-, be-/bi-, mi-/me-, la-/ba- (with the article); these examples are illustrative, not a fixed list. One invalid entry rejects the batch and writes nothing; the error starts with a sentence and names that entry with a JSON pointer. After a successful batch, lesson_items are rebuilt for every unit whose document references an entry by its pointed text or its bare form. rebuiltUnits lists those unit ids. A rebuild failure rolls the batch back. This does not add a unit revision and does not publish.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  true,
			DestructiveHint: &falseHint,
			OpenWorldHint:   &falseHint,
		},
	}, svc.upsertLexemes)
}
