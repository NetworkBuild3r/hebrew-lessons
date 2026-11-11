package mcp

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/db"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/lessons"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Class tools for INIT-039. The import check and the story check are the
// lessons package. A skill that does not call these tools has not checked anything.

type classStore interface {
	SaveClassSheet(ctx context.Context, learnerID int64, in lessons.Import) (int64, bool, error)
	ClassSheets(ctx context.Context, learnerID int64) ([]db.ClassSheet, error)
	SaveStory(ctx context.Context, learnerID int64, lessonType, body, status, failures string) error
	LoadStory(ctx context.Context, learnerID int64, lessonType string) (db.Story, bool, error)
}

type sheetAttacher interface {
	RequirePassedSheet(ctx context.Context, learnerID int64, ref string) (int64, error)
	AttachTaughtSheet(ctx context.Context, learnerID int64, lexemeIDs []int64, on string, sheetID int64) error
}

type importSheetInput struct {
	LearnerID *int64 `json:"learnerId,omitempty" jsonschema:"Required for an admin. A learner account may omit it."`
	Ref       string `json:"ref" jsonschema:"File reference of the class sheet. The same reference with the same lines changes nothing."`
	ClassOn   string `json:"classOn" jsonschema:"Class date YYYY-MM-DD."`
	Title     string `json:"title,omitempty" jsonschema:"Title as exported."`
	Kind      string `json:"kind,omitempty" jsonschema:"class, or notes. Notes, a zip, and html are refused."`
	Body      string `json:"body" jsonschema:"Exported lines. tutor: Hebrew || English, or learner: Hebrew. One line each. Do not split a line on punctuation."`
	Modified  string `json:"modified,omitempty" jsonschema:"RFC3339 modified time of the export. The same time changes nothing."`
}

type importSheetOutput struct {
	Passed    bool     `json:"passed"`
	Unchanged bool     `json:"unchanged"`
	Note      string   `json:"note"`
	Held      int      `json:"held"`
	Matches   []string `json:"matches"`
}

type classQueueOutput struct {
	Ready  int            `json:"ready"`
	Held   map[string]int `json:"held"`
	Sheets int            `json:"sheets"`
	Depth  int            `json:"depth"`
}

type storyRequestOutput struct {
	LessonType string   `json:"lessonType"`
	Coverage   float64  `json:"coverage"`
	MaxRunes   int      `json:"maxRunes"`
	NewWords   []string `json:"newWords"`
	Known      []string `json:"known"`
}

type putStoryInput struct {
	LearnerID  *int64   `json:"learnerId,omitempty" jsonschema:"Required for an admin. A learner account may omit it."`
	LessonType string   `json:"lessonType,omitempty" jsonschema:"word or easy. Defaults to word."`
	Text       string   `json:"text" jsonschema:"The story. One clip of the whole line later. This text is never graded."`
	Known      []string `json:"known" jsonschema:"Words the learner may meet: taught words plus this lesson's new words."`
	MustUse    []string `json:"mustUse,omitempty" jsonschema:"Due words the story was told to include."`
}

type putStoryOutput struct {
	Passed   bool     `json:"passed"`
	Failures []string `json:"failures"`
}

func registerClassTools(server *mcp.Server, svc *service) {
	falseHint := false
	write := &mcp.ToolAnnotations{
		ReadOnlyHint: false, IdempotentHint: true, DestructiveHint: &falseHint, OpenWorldHint: &falseHint,
	}
	read := &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, DestructiveHint: &falseHint, OpenWorldHint: &falseHint}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "import_class_sheet",
		Description: "Load one dated class sheet. Study notes, a zip, and html are refused. Lines stay verbatim. A held line is kept and is not used. Importing the same lines again changes nothing. The reply is the per-sheet check.",
		Annotations: write,
	}, svc.importClassSheet)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_class_queue",
		Description: "Ready class sheets, held lines by reason, and how many sheets have passed. Pool depth is the count of passed tutor lines.",
		Annotations: read,
	}, svc.getClassQueue)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_story_request",
		Description: "What a story must satisfy for the next word lesson: coverage 90 percent, at most 400 characters, the known words, and the new words when a passed sheet is loaded. The story is not graded.",
		Annotations: read,
	}, svc.getStoryRequest)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "put_story",
		Description: "Check one story and store it. The reply lists every failed check in that same response. Pass the known words and any due word the story was told to use. A story is never a graded key. put_unit is not how a story is saved.",
		Annotations: write,
	}, svc.putStory)
}

func (s *service) importClassSheet(ctx context.Context, _ *mcp.CallToolRequest, in importSheetInput) (*mcp.CallToolResult, importSheetOutput, error) {
	var out importSheetOutput
	_, id, _, err := s.learnerTool(ctx, "import_class_sheet", in.LearnerID)
	if err != nil {
		return nil, out, err
	}
	var modified time.Time
	if strings.TrimSpace(in.Modified) != "" {
		modified, err = time.Parse(time.RFC3339, strings.TrimSpace(in.Modified))
		if err != nil {
			return nil, out, errors.New("modified must be RFC3339.")
		}
	}
	got, err := lessons.ImportSheet(lessons.Sheet{
		Ref: in.Ref, ClassOn: in.ClassOn, Title: in.Title, Kind: in.Kind, Modified: modified, Body: in.Body,
	}, time.Time{})
	if err != nil {
		return nil, out, err
	}
	out.Passed, out.Unchanged, out.Note = got.Passed, got.Unchanged, got.Note
	for _, line := range got.Lines {
		if line.Held {
			out.Held++
		}
		if line.Tutor != "" && line.Gloss != "" && !line.Held {
			out.Matches = append(out.Matches, line.Tutor)
		}
	}
	if out.Matches == nil {
		out.Matches = []string{}
	}
	if _, err := time.Parse(time.DateOnly, got.ClassOn); err != nil {
		return nil, out, nil
	}
	store, ok := s.course.(classStore)
	if !ok {
		return nil, out, errors.New("Class sheets are not connected, so nothing was stored.")
	}
	_, unchanged, err := store.SaveClassSheet(ctx, id, got)
	if err != nil {
		return nil, out, storeErr("The class sheet could not be stored.", err)
	}
	if unchanged {
		out.Unchanged = true
		out.Note = "this export was already imported"
	}
	return nil, out, nil
}

func (s *service) getClassQueue(ctx context.Context, _ *mcp.CallToolRequest, in struct {
	LearnerID *int64 `json:"learnerId,omitempty"`
}) (*mcp.CallToolResult, classQueueOutput, error) {
	var out classQueueOutput
	_, id, _, err := s.learnerTool(ctx, "get_class_queue", in.LearnerID)
	if err != nil {
		return nil, out, err
	}
	store, ok := s.course.(classStore)
	if !ok {
		return nil, out, errors.New("Class sheets are not connected.")
	}
	sheets, err := store.ClassSheets(ctx, id)
	if err != nil {
		return nil, out, storeErr("The class queue could not be read.", err)
	}
	out.Held = map[string]int{}
	out.Sheets = len(sheets)
	for _, sh := range sheets {
		if sh.Status == "passed" {
			out.Ready++
		}
		for _, line := range sh.Lines {
			if line.Held {
				reason := line.HeldReason
				if reason == "" {
					reason = "held"
				}
				out.Held[reason]++
				continue
			}
			if sh.Status == "passed" && line.Tutor != "" {
				out.Depth++
			}
		}
	}
	return nil, out, nil
}

func (s *service) getStoryRequest(ctx context.Context, _ *mcp.CallToolRequest, in struct {
	LearnerID *int64 `json:"learnerId,omitempty"`
}) (*mcp.CallToolResult, storyRequestOutput, error) {
	var out storyRequestOutput
	_, id, _, err := s.learnerTool(ctx, "get_story_request", in.LearnerID)
	if err != nil {
		return nil, out, err
	}
	out.LessonType = "word"
	out.Coverage = 0.90
	out.MaxRunes = 400
	out.NewWords = []string{}
	out.Known = []string{}
	store, ok := s.course.(classStore)
	if !ok {
		return nil, out, nil
	}
	sheets, err := store.ClassSheets(ctx, id)
	if err != nil {
		return nil, out, storeErr("The class queue could not be read.", err)
	}
	for _, sh := range sheets {
		if sh.Status != "passed" {
			continue
		}
		for _, line := range sh.Lines {
			if line.Tutor != "" && !line.Held {
				out.Known = append(out.Known, line.Tutor)
			}
		}
	}
	return nil, out, nil
}

func (s *service) putStory(ctx context.Context, _ *mcp.CallToolRequest, in putStoryInput) (*mcp.CallToolResult, putStoryOutput, error) {
	var out putStoryOutput
	_, id, _, err := s.learnerTool(ctx, "put_story", in.LearnerID)
	if err != nil {
		return nil, out, err
	}
	kind := strings.TrimSpace(in.LessonType)
	if kind == "" {
		kind = "word"
	}
	if kind != "word" && kind != "easy" {
		return nil, out, errors.New("lessonType must be word or easy.")
	}
	if strings.TrimSpace(in.Text) == "" {
		out.Failures = []string{"story.empty"}
		return nil, out, nil
	}
	store, ok := s.course.(classStore)
	if !ok {
		return nil, out, errors.New("Stories are not connected, so nothing was stored.")
	}
	known := map[string]bool{}
	for _, w := range in.Known {
		known[w] = true
	}
	var earlier []string
	if prev, ok, err := store.LoadStory(ctx, id, kind); err != nil {
		return nil, out, storeErr("The story could not be read.", err)
	} else if ok && prev.Body != strings.TrimSpace(in.Text) {
		earlier = []string{prev.Body}
	}
	fails := lessons.CheckStory(in.Text, known, in.MustUse, earlier)
	out.Failures = make([]string, len(fails))
	for i, f := range fails {
		out.Failures[i] = f.ID
		if f.Word != "" {
			out.Failures[i] += ":" + f.Word
		}
	}
	if out.Failures == nil {
		out.Failures = []string{}
	}
	out.Passed = len(fails) == 0
	status := "failed"
	if out.Passed {
		status = "passed"
	}
	joined := strings.Join(out.Failures, " ")
	if err := store.SaveStory(ctx, id, kind, strings.TrimSpace(in.Text), status, joined); err != nil {
		return nil, out, storeErr("The story could not be stored.", err)
	}
	return nil, out, nil
}
