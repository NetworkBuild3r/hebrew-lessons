package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// verseStore is the Isaiah half of the database. Checking is not on this
// interface: a person checks a verse on the admin page, not through a tool.
type verseStore interface {
	UpsertVerseNotes(ctx context.Context, book string, chapter, verse int, notes []db.VerseNoteInput) error
	UpsertVerseGlosses(ctx context.Context, book string, chapter, verse int, glosses []db.VerseGlossInput) error
	VerseNotes(ctx context.Context, book string, chapter, verse int, checkedOnly bool) ([]db.VerseNote, error)
	VerseGlosses(ctx context.Context, book string, chapter, verse int, checkedOnly bool) ([]db.VerseGloss, error)
	TanakhVerse(ctx context.Context, book string, chapter, verse int) (db.TanakhVerse, bool, error)
}

func (s *service) verseDB() (verseStore, error) {
	if s.verses == nil {
		return nil, errors.New("Verse storage is not connected, so nothing was written.")
	}
	return s.verses, nil
}

type verseNoteIn struct {
	Layer  string `json:"layer" jsonschema:"gloss, roots, manner, or bridge."`
	Body   string `json:"body" jsonschema:"Restricted markdown, at most 8000 bytes. Raw HTML is refused."`
	Status string `json:"status,omitempty" jsonschema:"Ignored. This tool always stores a draft."`
}

type verseGlossIn struct {
	Position int    `json:"position" jsonschema:"Word position in the verse, starting at 0."`
	WordHe   string `json:"wordHe" jsonschema:"The pointed word."`
	GlossEn  string `json:"glossEn" jsonschema:"English gloss, at most 200 characters."`
	Root     string `json:"root,omitempty" jsonschema:"Optional consonantal root."`
	LexemeID int64  `json:"lexemeId,omitempty" jsonschema:"Optional lexicon id."`
	Status   string `json:"status,omitempty" jsonschema:"Ignored. This tool always stores a draft."`
}

type upsertVerseInput struct {
	Chapter int            `json:"chapter" jsonschema:"Isaiah chapter, from 1 to 66."`
	Verse   int            `json:"verse" jsonschema:"Verse number, from 1 to 200. The verse must already be in the Tanakh."`
	Notes   []verseNoteIn  `json:"notes,omitempty" jsonschema:"Layers for this verse. Each is stored as a draft."`
	Glosses []verseGlossIn `json:"glosses,omitempty" jsonschema:"Word glosses for this verse. Each is stored as a draft."`
}

type upsertVerseOutput struct {
	Notes   int `json:"notes"`
	Glosses int `json:"glosses"`
}

type getVerseInput struct {
	Chapter int `json:"chapter" jsonschema:"Isaiah chapter, from 1 to 66."`
	Verse   int `json:"verse" jsonschema:"Verse number, from 1 to 200."`
}

type verseNoteOut struct {
	Layer  string `json:"layer"`
	Body   string `json:"body"`
	Status string `json:"status"`
}

type verseGlossOut struct {
	Position int    `json:"position"`
	WordHe   string `json:"wordHe"`
	GlossEn  string `json:"glossEn"`
	Root     string `json:"root,omitempty"`
	Status   string `json:"status"`
}

type getVerseOutput struct {
	Book    string          `json:"book"`
	Chapter int             `json:"chapter"`
	Verse   int             `json:"verse"`
	He      string          `json:"he"`
	En      string          `json:"en"`
	Notes   []verseNoteOut  `json:"notes"`
	Glosses []verseGlossOut `json:"glosses"`
}

func registerVerseTools(server *mcp.Server, svc *service) {
	falseHint := false
	write := &mcp.ToolAnnotations{
		ReadOnlyHint: false, IdempotentHint: true, DestructiveHint: &falseHint, OpenWorldHint: &falseHint,
	}
	read := &mcp.ToolAnnotations{
		ReadOnlyHint: true, IdempotentHint: true, DestructiveHint: &falseHint, OpenWorldHint: &falseHint,
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "upsert_verse_notes",
		Description: "Draft notes and word glosses for one Isaiah verse. Admin only. Every row is stored as draft, including when status says checked. A checked note that is written again returns to draft. Bodies use the restricted markdown of an explainer. Raw HTML is refused and nothing is written. The verse must already be in the Tanakh. Checking is not a tool. A person checks the verse on the admin page.",
		Annotations: write,
	}, svc.upsertVerseNotes)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_verse",
		Description: "Read one Isaiah verse: the pointed text, the English, the notes, and the word glosses. A learner sees checked rows only. An admin, including the static token, also sees drafts. There is no tool that marks a note checked.",
		Annotations: read,
	}, svc.getVerse)
}

func (s *service) upsertVerseNotes(ctx context.Context, _ *mcp.CallToolRequest, in upsertVerseInput) (*mcp.CallToolResult, upsertVerseOutput, error) {
	var out upsertVerseOutput
	if _, err := s.gate(ctx, "upsert_verse_notes"); err != nil {
		return nil, out, err
	}
	if err := isaiahRef(in.Chapter, in.Verse); err != nil {
		return nil, out, err
	}
	if len(in.Notes) == 0 && len(in.Glosses) == 0 {
		return nil, out, errors.New("Send a note or a word gloss. Nothing was written.")
	}
	store, err := s.verseDB()
	if err != nil {
		return nil, out, err
	}
	caller := principalFrom(ctx)
	if len(in.Notes) > 0 {
		notes := make([]db.VerseNoteInput, len(in.Notes))
		for i, note := range in.Notes {
			notes[i] = db.VerseNoteInput{Layer: note.Layer, Body: note.Body, DraftedBy: caller.UserID}
		}
		if err := store.UpsertVerseNotes(ctx, "isaiah", in.Chapter, in.Verse, notes); err != nil {
			return nil, out, verseWriteErr(err)
		}
		out.Notes = len(notes)
	}
	if len(in.Glosses) > 0 {
		glosses := make([]db.VerseGlossInput, len(in.Glosses))
		for i, g := range in.Glosses {
			glosses[i] = db.VerseGlossInput{
				Position: g.Position, WordHe: g.WordHe, GlossEn: g.GlossEn, Root: g.Root, LexemeID: g.LexemeID,
			}
		}
		if err := store.UpsertVerseGlosses(ctx, "isaiah", in.Chapter, in.Verse, glosses); err != nil {
			return nil, out, verseWriteErr(err)
		}
		out.Glosses = len(glosses)
	}
	return nil, out, nil
}

func (s *service) getVerse(ctx context.Context, _ *mcp.CallToolRequest, in getVerseInput) (*mcp.CallToolResult, getVerseOutput, error) {
	var out getVerseOutput
	caller, err := s.gate(ctx, "get_verse")
	if err != nil {
		return nil, out, err
	}
	if err := isaiahRef(in.Chapter, in.Verse); err != nil {
		return nil, out, err
	}
	store, err := s.verseDB()
	if err != nil {
		return nil, out, err
	}
	row, ok, err := store.TanakhVerse(ctx, "isaiah", in.Chapter, in.Verse)
	if err != nil {
		return nil, out, storeErr("The verse could not be read.", err)
	}
	if !ok {
		return nil, out, errors.New("That verse is not in the Tanakh.")
	}
	checkedOnly := caller.Role != RoleAdmin
	notes, err := store.VerseNotes(ctx, "isaiah", in.Chapter, in.Verse, checkedOnly)
	if err != nil {
		return nil, out, storeErr("The notes could not be read.", err)
	}
	glosses, err := store.VerseGlosses(ctx, "isaiah", in.Chapter, in.Verse, checkedOnly)
	if err != nil {
		return nil, out, storeErr("The glosses could not be read.", err)
	}
	out = getVerseOutput{Book: row.Book, Chapter: row.Chapter, Verse: row.Verse, He: row.He, En: row.En}
	out.Notes = make([]verseNoteOut, len(notes))
	for i, note := range notes {
		out.Notes[i] = verseNoteOut{Layer: note.Layer, Body: note.Body, Status: note.Status}
	}
	out.Glosses = make([]verseGlossOut, len(glosses))
	for i, g := range glosses {
		out.Glosses[i] = verseGlossOut{Position: g.Position, WordHe: g.WordHe, GlossEn: g.GlossEn, Root: g.Root, Status: g.Status}
	}
	return nil, out, nil
}

func isaiahRef(chapter, verse int) error {
	if chapter < 1 || chapter > 66 || verse < 1 || verse > 200 {
		return errors.New("Isaiah chapters are 1 to 66 and verses are 1 to 200.")
	}
	return nil
}

func verseWriteErr(err error) error {
	var missing db.MissingVerseError
	if errors.As(err, &missing) {
		return errors.New("That verse is not in the Tanakh, so nothing was written.")
	}
	msg := err.Error()
	if strings.Contains(msg, "db: verse notes:") || strings.Contains(msg, "db: verse glosses:") ||
		strings.Contains(msg, "db: begin") || strings.Contains(msg, "db: commit") || strings.Contains(msg, "db: tanakh") {
		return storeErr("The verse notes could not be saved.", fmt.Errorf("%w", err))
	}
	return errors.New(strings.TrimPrefix(msg, "db: "))
}
