package mcp

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// INIT-023/SPEC-006

type nounFormIn struct {
	LemmaID      int64  `json:"lemmaId" jsonschema:"Lexicon id of the dictionary form. The lemma must already exist."`
	Kind         string `json:"kind" jsonschema:"singular, plural, definite, construct, possessive, or agreement."`
	Number       string `json:"number,omitempty" jsonschema:"sg, pl, or empty. For possessive, the thing owned; for construct and agreement, the form's number."`
	Person       string `json:"person,omitempty" jsonschema:"1, 2, 3, or empty. Possessive owner person."`
	Gender       string `json:"gender,omitempty" jsonschema:"m, f, or empty. Possessive owner gender, or agreement gender."`
	OwnerNumber  string `json:"ownerNumber,omitempty" jsonschema:"sg, pl, or empty. Possessive owner number."`
	FormHe       string `json:"formHe" jsonschema:"The pointed form, at most 200 characters."`
	Translit     string `json:"translit,omitempty" jsonschema:"Optional transliteration, at most 200 characters."`
	FormLexemeID int64  `json:"formLexemeId,omitempty" jsonschema:"Optional lexicon id of this form."`
}

type upsertNounFormsInput struct {
	Forms []nounFormIn `json:"forms" jsonschema:"At most 50. If any lemma is missing, the whole batch writes nothing and the reply names that lemma."`
}

func registerNounFormTools(server *mcp.Server, svc *service) {
	falseHint := false
	write := &mcp.ToolAnnotations{
		ReadOnlyHint: false, IdempotentHint: true, DestructiveHint: &falseHint, OpenWorldHint: &falseHint,
	}
	mcp.AddTool(server, &mcp.Tool{
		Name: "upsert_noun_forms",
		Description: "Write noun and adjective forms. Admin only. At most 50. lemmaId must already be in the lexicon. " +
			"If any lemma is missing, the whole batch writes nothing and the reply names that lemma. " +
			"kind is singular, plural, definite, construct, possessive, or agreement. " +
			"A possessive example: for סֵפֶר, סִפְרִי is person 1, ownerNumber sg, number sg (my book). " +
			"An agreement example: for רַע, רָעָה is number sg, gender f (bad, feminine). " +
			"A repeat of the same lemma, kind, number, person, gender, and ownerNumber replaces the form.",
		Annotations: write,
	}, svc.upsertNounForms)
}

func (s *service) upsertNounForms(ctx context.Context, _ *mcp.CallToolRequest, in upsertNounFormsInput) (*mcp.CallToolResult, writtenOutput, error) {
	var out writtenOutput
	if _, err := s.gate(ctx, "upsert_noun_forms"); err != nil {
		return nil, out, err
	}
	store, err := s.mistakeDB()
	if err != nil {
		return nil, out, err
	}
	if len(in.Forms) == 0 || len(in.Forms) > maxMistakeBatch {
		return nil, out, errors.New("Send 1 to 50 noun forms.")
	}
	rows := make([]db.NounFormInput, len(in.Forms))
	for i, row := range in.Forms {
		if utf8.RuneCountInString(row.FormHe) > 200 || utf8.RuneCountInString(row.Translit) > 200 {
			return nil, out, errors.New("A form is at most 200 characters, so nothing was written.")
		}
		rows[i] = db.NounFormInput{
			LemmaID: row.LemmaID, Kind: row.Kind, Number: row.Number, Person: row.Person,
			Gender: row.Gender, OwnerNumber: row.OwnerNumber, FormHe: row.FormHe,
			Translit: row.Translit, FormLexemeID: row.FormLexemeID,
		}
	}
	if err := store.UpsertNounForms(ctx, rows); err != nil {
		var missing db.MissingLemmaError
		if errors.As(err, &missing) {
			return nil, out, fmt.Errorf("Lemma %d is not in the lexicon, so nothing was written.", missing.ID)
		}
		return nil, out, storeErr("The noun forms could not be saved.", err)
	}
	out.Written = len(rows)
	return nil, out, nil
}
