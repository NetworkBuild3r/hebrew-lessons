package mcp

import (
	"context"
	"errors"
	"unicode/utf8"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxMistakeBatch = 50

// mistakeStore is the mistakes and verb-form half of the database.
// A memory store does not implement it, and the tools then refuse to write.
type mistakeStore interface {
	RecordMistakes(ctx context.Context, learnerID int64, rows []db.MistakeInput) error
	MistakesFor(ctx context.Context, learnerID int64, limit int) ([]db.LearnerMistake, error)
	UpsertCommonMistakes(ctx context.Context, rows []db.CommonMistakeInput) error
	UpsertVerbForms(ctx context.Context, rows []db.VerbFormInput) error
}

func (s *service) mistakeDB() (mistakeStore, error) {
	if s.mistakes == nil {
		return nil, errors.New("Mistake storage is not connected, so nothing was written.")
	}
	return s.mistakes, nil
}

type mistakeIn struct {
	Wrong    string `json:"wrong" jsonschema:"What was said, pointed Hebrew, at most 200 characters."`
	Right    string `json:"right" jsonschema:"The right form, pointed Hebrew, at most 200 characters."`
	Note     string `json:"note,omitempty" jsonschema:"Optional English note, at most 300 characters."`
	Pattern  string `json:"pattern,omitempty" jsonschema:"Optional grammar-pattern slug."`
	LexemeID int64  `json:"lexemeId,omitempty" jsonschema:"Optional lexicon id of the word."`
}

type recordMistakesInput struct {
	LearnerID *int64      `json:"learnerId,omitempty" jsonschema:"Required for an admin, including the static token. A learner account may omit it and may only use its own id."`
	Mistakes  []mistakeIn `json:"mistakes" jsonschema:"At most 50. A repeat of the same wrong and right form increments how often it has come back."`
}

type recordMistakesOutput struct {
	Written int `json:"written"`
}

type commonMistakeIn struct {
	Pattern string `json:"pattern,omitempty" jsonschema:"Optional grammar-pattern slug. There is no learner on a shared mistake."`
	Wrong   string `json:"wrong" jsonschema:"The form learners often say, pointed Hebrew, at most 200 characters."`
	Right   string `json:"right" jsonschema:"The right form, pointed Hebrew, at most 200 characters."`
	Explain string `json:"explain,omitempty" jsonschema:"Optional English explanation, at most 300 characters."`
	Status  string `json:"status" jsonschema:"draft or published. Only published rows are shown."`
}

type upsertCommonInput struct {
	Mistakes []commonMistakeIn `json:"mistakes" jsonschema:"At most 50 shared mistakes. A repeat replaces the explanation and the status."`
}

type verbFormIn struct {
	LemmaID      int64  `json:"lemmaId" jsonschema:"Lexicon id of the dictionary form. The lemma must already exist."`
	Tense        string `json:"tense" jsonschema:"present, past, future, imperative, or infinitive."`
	Person       string `json:"person,omitempty" jsonschema:"1, 2, 3, or empty."`
	Number       string `json:"number,omitempty" jsonschema:"sg, pl, or empty."`
	Gender       string `json:"gender,omitempty" jsonschema:"m, f, or empty."`
	FormHe       string `json:"formHe" jsonschema:"The pointed form, at most 200 characters."`
	Translit     string `json:"translit,omitempty" jsonschema:"Optional transliteration, at most 200 characters."`
	FormLexemeID int64  `json:"formLexemeId,omitempty" jsonschema:"Optional lexicon id of this form."`
}

type upsertVerbFormsInput struct {
	Forms []verbFormIn `json:"forms" jsonschema:"At most 50. If any lemma is missing, the whole batch writes nothing."`
}

type writtenOutput struct {
	Written int `json:"written"`
}

func registerMistakeTools(server *mcp.Server, svc *service) {
	falseHint := false
	write := &mcp.ToolAnnotations{
		ReadOnlyHint: false, IdempotentHint: true, DestructiveHint: &falseHint, OpenWorldHint: &falseHint,
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "record_mistakes",
		Description: "Record personal mistakes for one learner. A learner account writes its own, and the learner id comes from the token. Naming another learner is refused. An admin, including the static token, must pass learnerId. At most 50. Each has wrong, right, and optional note, pattern, and lexemeId. A repeat of the same pair increments how often it returns. These rows are never copied into the shared list.",
		Annotations: write,
	}, svc.recordMistakes)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "upsert_common_mistakes",
		Description: "Write shared mistakes, the ones learners often say. Admin only. There is no learner on these rows, and a personal mistake is never copied here. At most 50. status is draft or published. Only a published row is shown on an explainer.",
		Annotations: write,
	}, svc.upsertCommonMistakes)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "upsert_verb_forms",
		Description: "Write verb forms. Admin only. At most 50. lemmaId must already be in the lexicon. If any lemma is missing, the whole batch writes nothing. A repeat of the same lemma, tense, person, number, and gender replaces the form.",
		Annotations: write,
	}, svc.upsertVerbForms)
}

func (s *service) recordMistakes(ctx context.Context, _ *mcp.CallToolRequest, in recordMistakesInput) (*mcp.CallToolResult, recordMistakesOutput, error) {
	var out recordMistakesOutput
	caller, id, _, err := s.learnerTool(ctx, "record_mistakes", in.LearnerID)
	if err != nil {
		return nil, out, err
	}
	store, err := s.mistakeDB()
	if err != nil {
		return nil, out, err
	}
	if len(in.Mistakes) == 0 || len(in.Mistakes) > maxMistakeBatch {
		return nil, out, errors.New("Send 1 to 50 mistakes.")
	}
	source := "tutor"
	if caller.Role == RoleLearner {
		source = "app"
	}
	rows := make([]db.MistakeInput, len(in.Mistakes))
	for i, row := range in.Mistakes {
		if utf8.RuneCountInString(row.Wrong) > 200 || utf8.RuneCountInString(row.Right) > 200 || utf8.RuneCountInString(row.Note) > 300 {
			return nil, out, errors.New("A form is at most 200 characters and a note is at most 300, so nothing was written.")
		}
		rows[i] = db.MistakeInput{
			Wrong: row.Wrong, Right: row.Right, Note: row.Note, Pattern: row.Pattern, LexemeID: row.LexemeID, Source: source,
		}
	}
	if err := store.RecordMistakes(ctx, id, rows); err != nil {
		return nil, out, storeErr("The mistakes could not be saved.", err)
	}
	out.Written = len(rows)
	return nil, out, nil
}

func (s *service) upsertCommonMistakes(ctx context.Context, _ *mcp.CallToolRequest, in upsertCommonInput) (*mcp.CallToolResult, writtenOutput, error) {
	var out writtenOutput
	if _, err := s.gate(ctx, "upsert_common_mistakes"); err != nil {
		return nil, out, err
	}
	store, err := s.mistakeDB()
	if err != nil {
		return nil, out, err
	}
	if len(in.Mistakes) == 0 || len(in.Mistakes) > maxMistakeBatch {
		return nil, out, errors.New("Send 1 to 50 mistakes.")
	}
	rows := make([]db.CommonMistakeInput, len(in.Mistakes))
	for i, row := range in.Mistakes {
		if utf8.RuneCountInString(row.Wrong) > 200 || utf8.RuneCountInString(row.Right) > 200 || utf8.RuneCountInString(row.Explain) > 300 {
			return nil, out, errors.New("A form is at most 200 characters and an explanation is at most 300, so nothing was written.")
		}
		rows[i] = db.CommonMistakeInput{Pattern: row.Pattern, Wrong: row.Wrong, Right: row.Right, Explain: row.Explain, Status: row.Status}
	}
	if err := store.UpsertCommonMistakes(ctx, rows); err != nil {
		return nil, out, storeErr("The shared mistakes could not be saved.", err)
	}
	out.Written = len(rows)
	return nil, out, nil
}

func (s *service) upsertVerbForms(ctx context.Context, _ *mcp.CallToolRequest, in upsertVerbFormsInput) (*mcp.CallToolResult, writtenOutput, error) {
	var out writtenOutput
	if _, err := s.gate(ctx, "upsert_verb_forms"); err != nil {
		return nil, out, err
	}
	store, err := s.mistakeDB()
	if err != nil {
		return nil, out, err
	}
	if len(in.Forms) == 0 || len(in.Forms) > maxMistakeBatch {
		return nil, out, errors.New("Send 1 to 50 verb forms.")
	}
	rows := make([]db.VerbFormInput, len(in.Forms))
	for i, row := range in.Forms {
		if utf8.RuneCountInString(row.FormHe) > 200 || utf8.RuneCountInString(row.Translit) > 200 {
			return nil, out, errors.New("A form is at most 200 characters, so nothing was written.")
		}
		rows[i] = db.VerbFormInput{
			LemmaID: row.LemmaID, Tense: row.Tense, Person: row.Person, Number: row.Number, Gender: row.Gender,
			FormHe: row.FormHe, Translit: row.Translit, FormLexemeID: row.FormLexemeID,
		}
	}
	if err := store.UpsertVerbForms(ctx, rows); err != nil {
		var missing db.MissingLemmaError
		if errors.As(err, &missing) {
			return nil, out, errors.New("A lemma is not in the lexicon, so nothing was written.")
		}
		return nil, out, storeErr("The verb forms could not be saved.", err)
	}
	out.Written = len(rows)
	return nil, out, nil
}
