package mcp

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/NetworkBuild3r/hebrew-lessons/internal/evidence"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/studyday"
	"github.com/NetworkBuild3r/hebrew-lessons/internal/units"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// INIT-026/SPEC-008; ADR-0022 D2: outside teaching as taught words only.

const (
	maxTaughtWordsBatch = 50
	privacyWriteSuffix  = " Send extracted words and corrections only; never send transcript text, recordings, or other people's names."
)

const taughtWordsDescription = "Record words taught to one learner outside the app (tutor session, worksheet). " +
	"A learner account writes its own, and omitting learnerId uses its own id. Another id is forbidden. " +
	"An admin, including the static token, must pass learnerId. " +
	"words is 1 to 50 items, each with lexemeId or he (pointed Hebrew). he is looked up in the lexicon; " +
	"an unknown string is listed in unknown and nothing is written for it. Lexemes are never created here. " +
	"on is the learner's civil day YYYY-MM-DD; omit it for today in the learner's zone. " +
	"A day after today, or more than 30 days before today, is refused and nothing is written. " +
	"Each known word gets new read and recall cards (and listen when a clip exists) due at the next 4:00 in the learner's zone. " +
	"Existing cards are never graded or moved. The reply has created and hadCard per known word, unknown, due (RFC3339), and zone." +
	privacyWriteSuffix

const removeTaughtDescription = "Undo record_taught_words for named lexemes on one civil day. " +
	"A learner account undoes its own, and omitting learnerId uses its own id. Another id is forbidden. " +
	"An admin, including the static token, must pass learnerId. " +
	"on is required (YYYY-MM-DD). lexemeIds is 1 to 50. " +
	"Deletes the taught_words row and only cards that row created that still have no review. " +
	"Cards that were reviewed are listed in kept." +
	privacyWriteSuffix

// taughtStore is the Course half that schedules taught words through evidence.Recorder.
type taughtStore interface {
	Taught(ctx context.Context, learnerID int64, lexemeIDs []int64, on string, z studyday.Zone) (evidence.TaughtResult, error)
	RemoveTaught(ctx context.Context, learnerID int64, on string, lexemeIDs []int64) (evidence.RemoveTaughtResult, error)
	ResolveTaughtWord(ctx context.Context, lexemeID int64, he string) (id int64, ok bool, err error)
}

func (s *service) taughtDB() (taughtStore, error) {
	if t, ok := s.course.(taughtStore); ok {
		return t, nil
	}
	return nil, errors.New("Taught-word storage is not connected, so nothing was written.")
}

type taughtWordIn struct {
	LexemeID int64  `json:"lexemeId,omitempty" jsonschema:"Lexicon id when known. Prefer he when the tutor extracted pointed Hebrew."`
	He       string `json:"he,omitempty" jsonschema:"Pointed Hebrew. Looked up; unknown strings come back in unknown."`
}

type recordTaughtInput struct {
	LearnerID *int64         `json:"learnerId,omitempty" jsonschema:"Required for an admin, including the static token. A learner account may omit it and may only use its own id."`
	On        string         `json:"on,omitempty" jsonschema:"Learner's civil day YYYY-MM-DD. Defaults to today in the learner's zone."`
	Words     []taughtWordIn `json:"words" jsonschema:"1 to 50 words, each with lexemeId or he."`
	SheetRef  string         `json:"sheetRef,omitempty" jsonschema:"File reference of the passed class sheet these words came from. Omit it for a word that is not from a class sheet. An unknown reference writes nothing."`
}

type taughtWordOut struct {
	LexemeID int64    `json:"lexemeId"`
	Created  []string `json:"created"`
	HadCard  []string `json:"hadCard"`
	Already  bool     `json:"already,omitempty"`
}

type recordTaughtOutput struct {
	Words   []taughtWordOut `json:"words"`
	Unknown []string        `json:"unknown"`
	Due     time.Time       `json:"due"`
	Zone    string          `json:"zone"`
}

type removeTaughtInput struct {
	LearnerID *int64  `json:"learnerId,omitempty" jsonschema:"Required for an admin, including the static token. A learner account may omit it and may only use its own id."`
	On        string  `json:"on" jsonschema:"Learner's civil day YYYY-MM-DD when the words were taught."`
	LexemeIDs []int64 `json:"lexemeIds" jsonschema:"1 to 50 lexicon ids."`
}

type removeTaughtWordOut struct {
	LexemeID int64    `json:"lexemeId"`
	Removed  []string `json:"removed"`
	Kept     []string `json:"kept"`
}

type removeTaughtOutput struct {
	Words []removeTaughtWordOut `json:"words"`
}

func registerTaughtTools(server *mcp.Server, svc *service) {
	falseHint := false
	write := &mcp.ToolAnnotations{
		ReadOnlyHint: false, IdempotentHint: true, DestructiveHint: &falseHint, OpenWorldHint: &falseHint,
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "record_taught_words",
		Description: taughtWordsDescription,
		Annotations: write,
	}, svc.recordTaughtWords)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "remove_taught_words",
		Description: removeTaughtDescription,
		Annotations: write,
	}, svc.removeTaughtWords)
}

func (s *service) recordTaughtWords(ctx context.Context, _ *mcp.CallToolRequest, in recordTaughtInput) (*mcp.CallToolResult, recordTaughtOutput, error) {
	var out recordTaughtOutput
	_, id, _, err := s.learnerTool(ctx, "record_taught_words", in.LearnerID)
	if err != nil {
		return nil, out, err
	}
	store, err := s.taughtDB()
	if err != nil {
		return nil, out, err
	}
	if len(in.Words) == 0 || len(in.Words) > maxTaughtWordsBatch {
		return nil, out, errors.New("Send 1 to 50 words.")
	}
	z, err := s.learnerZone(ctx, id)
	if err != nil {
		return nil, out, err
	}
	on, err := resolveOnDay(strings.TrimSpace(in.On), s.now(), z)
	if err != nil {
		return nil, out, err
	}
	unknown := []string{}
	ids := make([]int64, 0, len(in.Words))
	seen := map[int64]struct{}{}
	for _, w := range in.Words {
		lexID, ok, err := store.ResolveTaughtWord(ctx, w.LexemeID, w.He)
		if err != nil {
			return nil, out, storeErr("The lexicon could not be read.", err)
		}
		if !ok {
			label := units.Normalize(w.He)
			if label == "" && w.LexemeID > 0 {
				label = "lexemeId:" + strconv.FormatInt(w.LexemeID, 10)
			}
			if label == "" {
				label = "unknown"
			}
			unknown = append(unknown, label)
			continue
		}
		if _, dup := seen[lexID]; dup {
			continue
		}
		seen[lexID] = struct{}{}
		ids = append(ids, lexID)
	}
	out.Unknown = unknown
	out.Zone = z.Name
	out.Due = studyday.DueCutoff(s.now(), z)
	out.Words = []taughtWordOut{}
	if len(ids) == 0 {
		return nil, out, nil
	}
	var sheetID int64
	if ref := strings.TrimSpace(in.SheetRef); ref != "" {
		attacher, ok := s.course.(sheetAttacher)
		if !ok {
			return nil, out, errors.New("Class sheets are not connected, so nothing was written.")
		}
		sheetID, err = attacher.RequirePassedSheet(ctx, id, ref)
		if err != nil {
			return nil, out, errors.New("sheetRef is not a passed class sheet, so nothing was written.")
		}
	}
	res, err := store.Taught(ctx, id, ids, on, z)
	if err != nil {
		return nil, out, mapTaughtErr(err)
	}
	out.Words = make([]taughtWordOut, len(res.Words))
	for i, w := range res.Words {
		created, had := w.Created, w.HadCard
		if created == nil {
			created = []string{}
		}
		if had == nil {
			had = []string{}
		}
		out.Words[i] = taughtWordOut{LexemeID: w.LexemeID, Created: created, HadCard: had, Already: w.Already}
		out.Due = w.Due
	}
	if sheetID > 0 {
		if err := s.course.(sheetAttacher).AttachTaughtSheet(ctx, id, ids, on, sheetID); err != nil {
			return nil, out, storeErr("The words were recorded, but the class sheet could not be attached.", err)
		}
	}
	return nil, out, nil
}

func (s *service) removeTaughtWords(ctx context.Context, _ *mcp.CallToolRequest, in removeTaughtInput) (*mcp.CallToolResult, removeTaughtOutput, error) {
	var out removeTaughtOutput
	_, id, _, err := s.learnerTool(ctx, "remove_taught_words", in.LearnerID)
	if err != nil {
		return nil, out, err
	}
	store, err := s.taughtDB()
	if err != nil {
		return nil, out, err
	}
	on := strings.TrimSpace(in.On)
	if on == "" {
		return nil, out, errors.New("on is required and must be YYYY-MM-DD.")
	}
	if _, err := time.Parse(time.DateOnly, on); err != nil {
		return nil, out, errors.New("on must be YYYY-MM-DD.")
	}
	if len(in.LexemeIDs) == 0 || len(in.LexemeIDs) > maxTaughtWordsBatch {
		return nil, out, errors.New("Send 1 to 50 lexeme ids.")
	}
	res, err := store.RemoveTaught(ctx, id, on, in.LexemeIDs)
	if err != nil {
		return nil, out, mapTaughtErr(err)
	}
	out.Words = make([]removeTaughtWordOut, len(res.Words))
	for i, w := range res.Words {
		removed, kept := w.Removed, w.Kept
		if removed == nil {
			removed = []string{}
		}
		if kept == nil {
			kept = []string{}
		}
		out.Words[i] = removeTaughtWordOut{LexemeID: w.LexemeID, Removed: removed, Kept: kept}
	}
	return nil, out, nil
}

func mapTaughtErr(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "after today"):
		return errors.New(strings.TrimPrefix(msg, "evidence: "))
	case strings.Contains(msg, "more than"):
		return errors.New(strings.TrimPrefix(msg, "evidence: "))
	case strings.Contains(msg, "YYYY-MM-DD"):
		return errors.New("on must be YYYY-MM-DD.")
	case strings.Contains(msg, "1 to 50"):
		return errors.New("Send 1 to 50 words.")
	default:
		return storeErr("The taught words could not be saved.", err)
	}
}
